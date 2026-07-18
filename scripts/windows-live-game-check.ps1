param(
  [string]$RuntimeRoot = "",
  [string]$PackageDir = "",
  [string]$ArtifactRoot = "",
  [ValidateRange(5, 60)][int]$SafeStopWaitSeconds = 5,
  [ValidateRange(5, 30)][int]$ControlTimeoutMinutes = 15
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$RepositoryRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
if ([string]::IsNullOrWhiteSpace($RuntimeRoot)) {
  $RuntimeRoot = Join-Path $RepositoryRoot "dev-runtime\windows"
}
$RuntimeRoot = [IO.Path]::GetFullPath($RuntimeRoot)
$managedRoot = [IO.Path]::GetFullPath((Join-Path $RepositoryRoot "dev-runtime\windows"))
if (-not ($RuntimeRoot -eq $managedRoot -or $RuntimeRoot.StartsWith($managedRoot + '\', [StringComparison]::OrdinalIgnoreCase))) {
  throw "runtime root must stay under $managedRoot"
}
if ([string]::IsNullOrWhiteSpace($PackageDir)) {
  $PackageDir = Get-ChildItem (Join-Path $RuntimeRoot "package") -Directory -ErrorAction SilentlyContinue |
    Where-Object { Test-Path (Join-Path $_.FullName "palpanel-server.exe") -PathType Leaf } |
    Sort-Object LastWriteTime -Descending |
    Select-Object -First 1 -ExpandProperty FullName
  if ([string]::IsNullOrWhiteSpace($PackageDir)) {
    throw "no extracted Windows package containing palpanel-server.exe was found under $RuntimeRoot\package"
  }
}
$PackageDir = [IO.Path]::GetFullPath($PackageDir)
if (-not $PackageDir.StartsWith($RuntimeRoot + '\', [StringComparison]::OrdinalIgnoreCase)) {
  throw "package directory must stay under the runtime root"
}
if ([string]::IsNullOrWhiteSpace($ArtifactRoot)) {
  $runID = "live-game-{0}-{1}" -f (Get-Date).ToUniversalTime().ToString("yyyyMMddTHHmmssfffZ"), ([guid]::NewGuid().ToString("N").Substring(0, 8))
  $ArtifactRoot = Join-Path $RuntimeRoot "artifacts\$runID"
}
$ArtifactRoot = [IO.Path]::GetFullPath($ArtifactRoot)
if (-not $ArtifactRoot.StartsWith($RuntimeRoot + '\', [StringComparison]::OrdinalIgnoreCase)) {
  throw "artifact root must stay under the runtime root"
}
New-Item -ItemType Directory -Force -Path $ArtifactRoot | Out-Null
$Deadline = [DateTime]::UtcNow.AddMinutes($ControlTimeoutMinutes)
$Backend = $null
$BaseURL = ""
$LiveServerStarted = $false
$Events = [Collections.Generic.List[object]]::new()

function Write-JSON([object]$Value, [string]$Path) {
  [IO.File]::WriteAllText($Path, (($Value | ConvertTo-Json -Depth 20) + [Environment]::NewLine), [Text.UTF8Encoding]::new($false))
}

function Add-Event([string]$Stage, [string]$Status, [string]$Message) {
  $event = [pscustomobject]@{ timestamp = (Get-Date).ToUniversalTime().ToString("o"); stage = $Stage; status = $Status; message = $Message }
  $Events.Add($event)
  Add-Content -LiteralPath (Join-Path $ArtifactRoot "events.jsonl") -Value ($event | ConvertTo-Json -Compress) -Encoding UTF8
  Write-Host "[$Stage] $Status`: $Message"
}

function Assert-RemainingTime {
  if ([DateTime]::UtcNow -ge $Deadline) { throw "live-game check exceeded $ControlTimeoutMinutes minutes" }
}

function Get-FreeTCPPort {
  $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
  try { $listener.Start(); return ([Net.IPEndPoint]$listener.LocalEndpoint).Port } finally { $listener.Stop() }
}

function Get-PalSetting([string]$Name) {
  $path = Join-Path $RuntimeRoot "palworld\Pal\Saved\Config\WindowsServer\PalWorldSettings.ini"
  if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { return "" }
  $content = [IO.File]::ReadAllText($path)
  $pattern = '(?<![A-Za-z0-9_])' + [regex]::Escape($Name) + '=(?:"(?<quoted>(?:\\.|[^"])*)"|(?<plain>[^,\)\r\n]*))'
  $match = [regex]::Match($content, $pattern)
  if (-not $match.Success) { return "" }
  if ($match.Groups["quoted"].Success) { return $match.Groups["quoted"].Value }
  return $match.Groups["plain"].Value.Trim()
}

function Invoke-PanelAPI([string]$Method, [string]$Path, [object]$Body = $null) {
  $parameters = @{ Method = $Method; Uri = $BaseURL + $Path; TimeoutSec = 30; ErrorAction = "Stop" }
  if ($null -ne $Body) {
    $parameters.ContentType = "application/json"
    $parameters.Body = $Body | ConvertTo-Json -Depth 12 -Compress
  }
  $response = Invoke-RestMethod @parameters
  if ($response.PSObject.Properties.Name -contains "ok" -and -not $response.ok) { throw "$Method $Path returned ok=false" }
  if ($response.PSObject.Properties.Name -contains "data") { return $response.data }
  return $response
}

function Get-PalProcesses {
  $gameRoot = [IO.Path]::GetFullPath((Join-Path $RuntimeRoot "palworld")).TrimEnd('\')
  return @(Get-CimInstance Win32_Process | Where-Object {
      $_.ExecutablePath -and ([IO.Path]::GetFullPath($_.ExecutablePath) -eq $gameRoot -or [IO.Path]::GetFullPath($_.ExecutablePath).StartsWith($gameRoot + '\', [StringComparison]::OrdinalIgnoreCase))
    } | Select-Object ProcessId, ParentProcessId, Name, ExecutablePath, CommandLine)
}

function Start-Backend([string]$AdminPassword) {
  $server = Join-Path $PackageDir "palpanel-server.exe"
  if (-not (Test-Path -LiteralPath $server -PathType Leaf)) { throw "packaged backend missing: $server" }
  $port = Get-FreeTCPPort
  $config = Join-Path $RuntimeRoot "config\palpanel.env"
  $info = [Diagnostics.ProcessStartInfo]::new()
  $info.FileName = $server
  $info.Arguments = "--runtime-root `"$RuntimeRoot`" --config `"$config`""
  $info.WorkingDirectory = $PackageDir
  $info.UseShellExecute = $false
  $info.CreateNoWindow = $true
  $info.RedirectStandardOutput = $true
  $info.RedirectStandardError = $true
  $info.EnvironmentVariables["PALPANEL_RUNTIME_ROOT"] = $RuntimeRoot
  $info.EnvironmentVariables["PALPANEL_LISTEN_ADDR"] = "127.0.0.1:$port"
  $info.EnvironmentVariables["PALPANEL_REQUIRE_AUTH"] = "false"
  $info.EnvironmentVariables["PALPANEL_SAVE_INDEXER_ENABLED"] = "false"
  $info.EnvironmentVariables["PALWORLD_ADMIN_PASSWORD"] = $AdminPassword
  $process = [Diagnostics.Process]::new()
  $process.StartInfo = $info
  if (-not $process.Start()) { throw "failed to start packaged backend" }
  $Backend = [pscustomobject]@{ Process = $process; Stdout = $process.StandardOutput.ReadToEndAsync(); Stderr = $process.StandardError.ReadToEndAsync() }
  Set-Variable -Scope Script -Name Backend -Value $Backend
  Set-Variable -Scope Script -Name BaseURL -Value "http://127.0.0.1:$port"
  $healthDeadline = [DateTime]::UtcNow.AddSeconds(60)
  while ([DateTime]::UtcNow -lt $healthDeadline) {
    if ($process.HasExited) { throw "backend exited before health check with code $($process.ExitCode)" }
    try { if ((Invoke-RestMethod "$BaseURL/api/health" -TimeoutSec 2).data.status -eq "ok") { return } } catch { }
    Start-Sleep -Milliseconds 500
  }
  throw "backend health check timed out"
}

function Stop-Backend {
  if ($null -eq $Backend) { return }
  try {
    if (-not $Backend.Process.HasExited) {
      & "$env:SystemRoot\System32\taskkill.exe" /PID $Backend.Process.Id /T /F | Out-Null
      $Backend.Process.WaitForExit(10000) | Out-Null
    }
    [IO.File]::WriteAllText((Join-Path $ArtifactRoot "palpanel-server.stdout.log"), $Backend.Stdout.Result, [Text.UTF8Encoding]::new($false))
    [IO.File]::WriteAllText((Join-Path $ArtifactRoot "palpanel-server.stderr.log"), $Backend.Stderr.Result, [Text.UTF8Encoding]::new($false))
  } finally {
    $Backend.Process.Dispose()
    Set-Variable -Scope Script -Name Backend -Value $null
  }
}

function Wait-ServerEvidence([int[]]$PreviousPIDs = @()) {
  $until = [DateTime]::UtcNow.AddMinutes(5)
  $logPath = Join-Path $RuntimeRoot "data\logs\palserver.log"
  while ([DateTime]::UtcNow -lt $until) {
    Assert-RemainingTime
    $status = Invoke-PanelAPI GET "/api/server/status"
    $processes = @(Get-PalProcesses)
    $newProcesses = @($processes | Where-Object { $PreviousPIDs -notcontains [int]$_.ProcessId })
    $pids = @($processes | ForEach-Object { [int]$_.ProcessId })
    $ports = @($status.ports.PSObject.Properties | ForEach-Object { [int]$_.Value } | Where-Object { $_ -gt 0 })
    $udp = Get-NetUDPEndpoint -ErrorAction SilentlyContinue | Where-Object { $pids -contains [int]$_.OwningProcess -and $ports -contains [int]$_.LocalPort } | Select-Object -First 1
    if ($status.container.status -eq "running" -and $newProcesses.Count -gt 0 -and $null -ne $udp -and (Test-Path $logPath) -and (Get-Item $logPath).Length -gt 0) {
      return [pscustomobject]@{
        checked_at = (Get-Date).ToUniversalTime().ToString("o")
        status = $status
        processes = $processes
        udp_endpoint = [pscustomobject]@{ local_address = [string]$udp.LocalAddress; local_port = [int]$udp.LocalPort; owning_process = [int]$udp.OwningProcess }
        log_path = $logPath
        log_bytes = (Get-Item $logPath).Length
      }
    }
    Start-Sleep -Seconds 2
  }
  throw "PalServer process, UDP endpoint, and log evidence did not become ready"
}

function Invoke-PalREST([string]$Path, [string]$Password, [string]$Method = "GET", [object]$Body = $null) {
  $credential = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes("admin`:$Password"))
  $parameters = @{ Method = $Method; Uri = ("http://127.0.0.1:8212/v1/api/" + $Path.TrimStart('/')); Headers = @{ Authorization = "Basic $credential" }; TimeoutSec = 10 }
  if ($null -ne $Body) { $parameters.ContentType = "application/json"; $parameters.Body = $Body | ConvertTo-Json -Compress }
  return Invoke-RestMethod @parameters
}

function Read-RCONBytes([IO.Stream]$Stream, [int]$Count) {
  $buffer = [byte[]]::new($Count); $offset = 0
  while ($offset -lt $Count) { $read = $Stream.Read($buffer, $offset, $Count - $offset); if ($read -le 0) { throw "RCON connection closed" }; $offset += $read }
  Write-Output -NoEnumerate $buffer
}

function Read-RCONPacket([IO.Stream]$Stream) {
  $sizeBytes = [byte[]](Read-RCONBytes $Stream 4); $size = [BitConverter]::ToInt32($sizeBytes, 0)
  if ($size -lt 10 -or $size -gt 4MB) { throw "invalid RCON packet size $size" }
  $payload = [byte[]](Read-RCONBytes $Stream $size)
  if ($payload[$size - 2] -ne 0 -or $payload[$size - 1] -ne 0) { throw "invalid RCON packet terminator" }
  return [pscustomobject]@{ id = [BitConverter]::ToInt32($payload, 0); type = [BitConverter]::ToInt32($payload, 4); body = [Text.Encoding]::UTF8.GetString($payload, 8, $size - 10).Trim([char]0) }
}

function Write-RCONPacket([IO.Stream]$Stream, [int]$ID, [int]$Type, [string]$Body) {
  $bodyBytes = [Text.Encoding]::UTF8.GetBytes($Body); $size = $bodyBytes.Length + 10; $packet = [byte[]]::new($size + 4)
  [BitConverter]::GetBytes($size).CopyTo($packet, 0); [BitConverter]::GetBytes($ID).CopyTo($packet, 4); [BitConverter]::GetBytes($Type).CopyTo($packet, 8); $bodyBytes.CopyTo($packet, 12)
  $Stream.Write($packet, 0, $packet.Length); $Stream.Flush()
}

function Invoke-RCON([string]$Command, [string]$Password, [int]$Port) {
  $client = [Net.Sockets.TcpClient]::new()
  try {
    if (-not $client.ConnectAsync("127.0.0.1", $Port).Wait(5000)) { throw "RCON connect timeout" }
    $client.ReceiveTimeout = 1500; $client.SendTimeout = 5000; $stream = $client.GetStream()
    Write-RCONPacket $stream 1 3 $Password
    $authenticated = $false
    for ($i = 0; $i -lt 3; $i++) { $packet = Read-RCONPacket $stream; if ($packet.id -eq -1) { throw "RCON authentication failed" }; if ($packet.id -eq 1 -and $packet.type -eq 2) { $authenticated = $true; break } }
    if (-not $authenticated) { throw "RCON authentication reply missing" }
    Write-RCONPacket $stream 2 2 $Command
    $parts = [Collections.Generic.List[string]]::new()
    while ($true) {
      try { $packet = Read-RCONPacket $stream } catch { if ($parts.Count -gt 0 -and $_.Exception.Message -match 'timed out|did not properly respond') { break }; throw }
      if ($packet.id -eq 2 -and $packet.type -eq 0) { $parts.Add([string]$packet.body) }
    }
    if ($parts.Count -eq 0) { throw "RCON command returned no response: $Command" }
    return ($parts -join '').Trim()
  } finally { $client.Dispose() }
}

function Wait-ControlPlane([string]$Password, [int]$RCONPort) {
  $until = [DateTime]::UtcNow.AddMinutes(3); $lastError = ""
  while ([DateTime]::UtcNow -lt $until) {
    try {
      $info = Invoke-PalREST "info" $Password; $players = Invoke-PalREST "players" $Password; $rconInfo = Invoke-RCON "Info" $Password $RCONPort
      return [pscustomobject]@{ rest_info = $info; rest_players = $players; rcon_info = $rconInfo }
    } catch { $lastError = $_.Exception.Message }
    Start-Sleep -Seconds 2
  }
  throw "Palworld REST/RCON did not become ready: $lastError"
}

function Get-SaveSnapshot {
  $root = Join-Path $RuntimeRoot "palworld\Pal\Saved\SaveGames"
  if (-not (Test-Path $root -PathType Container)) { return @() }
  return @(Get-ChildItem $root -Recurse -File | ForEach-Object { [pscustomobject]@{ path = $_.FullName.Substring($root.Length).TrimStart('\').Replace('\', '/'); size_bytes = $_.Length; modified_at = $_.LastWriteTimeUtc.ToString("o"); modified_ticks = $_.LastWriteTimeUtc.Ticks } })
}

function Wait-SaveChange([object[]]$Before) {
  $old = @{}; foreach ($item in $Before) { $old[[string]$item.path] = $item }
  $until = [DateTime]::UtcNow.AddSeconds(60)
  while ([DateTime]::UtcNow -lt $until) {
    $after = @(Get-SaveSnapshot); $changed = @($after | Where-Object { $previous = $old[[string]$_.path]; $null -eq $previous -or $_.modified_ticks -gt $previous.modified_ticks -or $_.size_bytes -ne $previous.size_bytes })
    if (@($after | Where-Object path -like '*.sav').Count -gt 0 -and $changed.Count -gt 0) { return $after }
    Start-Sleep -Seconds 1
  }
  throw "RCON Save did not update a .sav file within 60 seconds"
}

function Wait-Job([string]$ID) {
  $lastState = ""
  while ($true) {
    $job = Invoke-PanelAPI GET "/api/jobs/$ID"
    $jobError = if ($job.PSObject.Properties.Name -contains "error") { [string]$job.error } else { "" }
    $state = "$($job.status):$($job.progress):$($job.message):$jobError"
    if ($state -ne $lastState) { Add-Event "safe-stop-job" "progress" $state; $lastState = $state }
    if ($job.status -in @("completed", "success")) { return $job }
    if ($job.status -eq "failed") { throw "job failed: $jobError $($job.message)" }
    Assert-RemainingTime; Start-Sleep -Seconds 1
  }
}

function Wait-Stopped([int]$TimeoutSeconds = 60) {
  $until = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
  while ([DateTime]::UtcNow -lt $until) {
    $status = Invoke-PanelAPI GET "/api/server/status"; if ($status.container.status -ne "running" -and @(Get-PalProcesses).Count -eq 0) { return $status }; Start-Sleep -Seconds 1
  }
  throw "PalServer process tree remained after safe stop"
}

$Outcome = "failed"; $Failure = ""; $StartedAt = (Get-Date).ToUniversalTime()
try {
  if (@(Get-PalProcesses).Count -ne 0) { throw "refusing to start while PalServer processes already exist" }
  foreach ($path in @("steamcmd\steamcmd.exe", "palworld\PalServer.exe", "palworld\Pal\Binaries\Win64\PalServer-Win64-Shipping-Cmd.exe", "palworld\steamapps\appmanifest_2394010.acf")) {
    $full = Join-Path $RuntimeRoot $path; if (-not (Test-Path $full -PathType Leaf) -or (Get-Item $full).Length -eq 0) { throw "installed game evidence missing: $full" }
  }
  $adminPassword = Get-PalSetting "AdminPassword"
  if ([string]::IsNullOrWhiteSpace($adminPassword)) { throw "AdminPassword is missing from PalWorldSettings.ini" }
  if ((Get-PalSetting "RESTAPIEnabled") -ne "True" -or (Get-PalSetting "RCONEnabled") -ne "True") { throw "RESTAPIEnabled and RCONEnabled must both be True" }
  $rconPort = [int](Get-PalSetting "RCONPort"); if ($rconPort -lt 1 -or $rconPort -gt 65535) { throw "invalid RCON port" }

  Add-Event "backend" "started" "starting packaged PalPanel backend with matching Palworld administrator credential"
  Start-Backend $adminPassword
  Invoke-PanelAPI PUT "/api/server/runtime" @{ mode = "windows_steamcmd" } | Out-Null
  Add-Event "backend" "passed" "backend health and Windows runtime selection passed"

  Add-Event "live-server" "started" "starting real Palworld Dedicated Server"
  Invoke-PanelAPI POST "/api/server/initialize-config" @{} | Out-Null
  Invoke-PanelAPI POST "/api/server/start" @{} | Out-Null
  $LiveServerStarted = $true
  $first = Wait-ServerEvidence
  Write-JSON $first (Join-Path $ArtifactRoot "start-evidence.json")
  Add-Event "live-server" "progress" "PalServer process, UDP endpoint, and panel log observed"

  $control = Wait-ControlPlane $adminPassword $rconPort
  $showPlayers = Invoke-RCON "ShowPlayers" $adminPassword $rconPort
  $before = @(Get-SaveSnapshot); Start-Sleep -Milliseconds 1100
  $saveOutput = Invoke-RCON "Save" $adminPassword $rconPort
  $after = @(Wait-SaveChange $before)
  Write-JSON ([pscustomobject]@{ checked_at = (Get-Date).ToUniversalTime().ToString("o"); official_rest = @{ info = $control.rest_info; players = $control.rest_players }; rcon = @{ port = $rconPort; info = $control.rcon_info; show_players = $showPlayers; save = $saveOutput }; saves = @{ root = Join-Path $RuntimeRoot "palworld\Pal\Saved\SaveGames"; before = $before; after = $after } }) (Join-Path $ArtifactRoot "control-plane-and-save-evidence.json")
  Add-Event "control-plane" "passed" "official REST info/players, RCON Info/ShowPlayers/Save, and .sav update passed"

  $oldPIDs = @($first.processes | ForEach-Object { [int]$_.ProcessId })
  Invoke-PanelAPI POST "/api/server/restart" @{} | Out-Null
  $restarted = Wait-ServerEvidence $oldPIDs
  foreach ($pidValue in $oldPIDs) { if (Get-Process -Id $pidValue -ErrorAction SilentlyContinue) { throw "restart left old PalServer PID $pidValue running" } }
  Write-JSON $restarted (Join-Path $ArtifactRoot "restart-evidence.json")
  Add-Event "restart" "passed" "managed restart produced a new PalServer process and UDP evidence"

  $directShutdownRequestedAt = (Get-Date).ToUniversalTime()
  $directShutdownResponse = Invoke-PalREST "shutdown" $adminPassword "POST" @{ waittime = 5; message = "PalPanel direct official REST shutdown check" }
  $directFinalStatus = Wait-Stopped 30
  Write-JSON ([pscustomobject]@{ requested_at = $directShutdownRequestedAt.ToString("o"); stopped_at = (Get-Date).ToUniversalTime().ToString("o"); response = $directShutdownResponse; final_status = $directFinalStatus; remaining_processes = @(Get-PalProcesses) }) (Join-Path $ArtifactRoot "official-rest-shutdown-evidence.json")
  $LiveServerStarted = $false
  Add-Event "official-rest-shutdown" "passed" "direct official REST shutdown exited the real PalServer process"

  Invoke-PanelAPI POST "/api/server/start" @{} | Out-Null
  $LiveServerStarted = $true
  $safeStopStart = Wait-ServerEvidence
  $safeStopControlPlane = Wait-ControlPlane $adminPassword $rconPort
  Write-JSON $safeStopStart (Join-Path $ArtifactRoot "safe-stop-start-evidence.json")
  Write-JSON ([pscustomobject]@{ checked_at = (Get-Date).ToUniversalTime().ToString("o"); official_rest = @{ info = $safeStopControlPlane.rest_info; players = $safeStopControlPlane.rest_players }; rcon_info = $safeStopControlPlane.rcon_info }) (Join-Path $ArtifactRoot "safe-stop-control-plane-evidence.json")
  Add-Event "safe-stop-control-plane" "passed" "REST and RCON became ready again before safe stop"

  $request = Invoke-PanelAPI POST "/api/server/safe-stop" @{ waittime = $SafeStopWaitSeconds; message = "PalPanel Windows live game check" }
  $job = Wait-Job ([string]$request.id)
  if ([string]$job.message -match "fallback") { throw "safe stop used managed fallback: $($job.message)" }
  $finalStatus = Wait-Stopped
  $LiveServerStarted = $false
  $audits = @(Invoke-PanelAPI GET "/api/audit-logs?limit=50")
  $audit = @($audits | Where-Object { $_.action -eq "POST /api/server/safe-stop" -and $_.status -eq "success" } | Select-Object -First 1)
  if ($audit.Count -eq 0) { throw "safe-stop audit entry missing" }
  Write-JSON ([pscustomobject]@{ checked_at = (Get-Date).ToUniversalTime().ToString("o"); job = $job; audit = $audit[0]; final_status = $finalStatus; remaining_processes = @(Get-PalProcesses) }) (Join-Path $ArtifactRoot "safe-stop-evidence.json")
  Add-Event "safe-stop" "passed" "official REST safe stop completed without managed fallback; process exit and audit verified"
  $Outcome = "passed"
} catch {
  $Failure = $_.Exception.Message
  Add-Event "run" "failed" $Failure
} finally {
  if ($LiveServerStarted -and -not [string]::IsNullOrWhiteSpace($BaseURL)) {
    try { Invoke-PanelAPI POST "/api/server/stop" @{} | Out-Null; Wait-Stopped | Out-Null } catch { Add-Event "cleanup" "warning" $_.Exception.Message }
  }
  try { Stop-Backend } catch { Add-Event "cleanup" "warning" $_.Exception.Message; if ($Outcome -eq "passed") { $Outcome = "failed"; $Failure = $_.Exception.Message } }
  Write-JSON ([pscustomobject]@{ outcome = $Outcome; started_at = $StartedAt.ToString("o"); finished_at = (Get-Date).ToUniversalTime().ToString("o"); runtime_root = $RuntimeRoot; package_dir = $PackageDir; artifact_root = $ArtifactRoot; phases = $Events; error = $Failure }) (Join-Path $ArtifactRoot "summary.json")
}

if ($Outcome -ne "passed") { throw "Windows live game check failed; artifacts: $ArtifactRoot; $Failure" }
Write-Host "Windows live game check passed"
Write-Host "Artifacts: $ArtifactRoot"
