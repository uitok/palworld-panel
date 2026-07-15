param(
  [string]$RuntimeRoot = "",
  [string]$TestRoot = "",
  [switch]$KeepArtifacts,
  [switch]$SkipGameDownload,
  [switch]$SkipLiveServer,
  [switch]$SkipPalDefender,
  [string]$Proxy = "",
  [string]$TranslationBaseURL = "",
  [string]$TranslationAPIKeyEnv = "",
  [ValidateRange(1, 1440)]
  [int]$TimeoutMinutes = 180
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$WindowsHost = $env:OS -eq "Windows_NT"
if (Get-Variable -Name IsWindows -ErrorAction SilentlyContinue) {
  $WindowsHost = [bool]$IsWindows
}
if (-not $WindowsHost) {
  throw "scripts/e2e-windows.ps1 must run on Windows"
}

$ScriptDir = $PSScriptRoot
Import-Module (Join-Path $ScriptDir "windows-e2e-common.psm1") -Force
$RepositoryRoot = Get-PalPanelRepositoryRoot

if ([string]::IsNullOrWhiteSpace($RuntimeRoot)) {
  $RuntimeRoot = Join-Path $RepositoryRoot "dev-runtime\windows"
} else {
  $RuntimeRoot = Resolve-PalPanelPath -Path $RuntimeRoot -BasePath $RepositoryRoot
}
$RuntimeRoot = Assert-PalPanelManagedPath -RepositoryRoot $RepositoryRoot -TargetPath $RuntimeRoot -AllowManagedRoot
$RuntimeRoot = Initialize-PalPanelWindowsLayout -RepositoryRoot $RepositoryRoot -RuntimeRoot $RuntimeRoot

$RunID = "e2e-{0}-{1}-{2}" -f (Get-Date).ToUniversalTime().ToString("yyyyMMddTHHmmssfffZ"), $PID, ([guid]::NewGuid().ToString("N").Substring(0, 8))
if ([string]::IsNullOrWhiteSpace($TestRoot)) {
  $TestRoot = Join-Path $RuntimeRoot "e2e\run-$RunID"
} else {
  $TestRoot = Resolve-PalPanelPath -Path $TestRoot -BasePath $RuntimeRoot
}
$TestRoot = Assert-PalPanelManagedPath -RepositoryRoot $RepositoryRoot -TargetPath $TestRoot
if (-not (Test-PalPanelPathWithin -Root $RuntimeRoot -Target $TestRoot)) {
  throw "test root must stay under the selected runtime root: $TestRoot"
}

$TestMarker = Join-Path $TestRoot ".palpanel-e2e-root.json"
if (Test-Path -LiteralPath $TestRoot) {
  $existing = @(Get-ChildItem -LiteralPath $TestRoot -Force)
  if ($existing.Count -gt 0 -and -not (Test-Path -LiteralPath $TestMarker -PathType Leaf)) {
    throw "refusing to reuse an unmarked non-empty test root: $TestRoot"
  }
}
New-Item -ItemType Directory -Force -Path $TestRoot | Out-Null

$ArtifactRoot = Join-Path $RuntimeRoot "artifacts\$RunID"
$CommandLogRoot = Join-Path $ArtifactRoot "commands"
$RuntimeTemp = Join-Path $RuntimeRoot "temp\$RunID"
foreach ($path in @($ArtifactRoot, $CommandLogRoot, $RuntimeTemp)) {
  Assert-PalPanelManagedPath -RepositoryRoot $RepositoryRoot -TargetPath $path | Out-Null
  New-Item -ItemType Directory -Force -Path $path | Out-Null
}

$StructuredLog = Join-Path $ArtifactRoot "events.jsonl"
$SummaryPath = Join-Path $ArtifactRoot "summary.json"
$PathsPath = Join-Path $ArtifactRoot "paths.json"
$RunStarted = Get-Date
$Deadline = [DateTime]::UtcNow.AddMinutes($TimeoutMinutes)
$script:StageResults = [System.Collections.Generic.List[object]]::new()
$script:SensitiveValues = [System.Collections.Generic.List[string]]::new()
$script:CurrentStage = "initialization"
$script:CommandSequence = 0
$script:Backend = $null
$script:BaseURL = ""
$script:LiveServerStarted = $false
$script:PalDefenderInstalled = $false
$script:RunSucceeded = $false
$script:RunFailure = $null

function Protect-E2EText {
  param([AllowNull()][string]$Text)
  if ($null -eq $Text) { return "" }
  $protected = $Text
  foreach ($secret in $script:SensitiveValues) {
    if (-not [string]::IsNullOrEmpty($secret)) {
      $protected = $protected.Replace($secret, "[REDACTED]")
    }
  }
  return $protected
}

function Write-E2EJsonFile {
  param([Parameter(Mandatory = $true)]$Value, [Parameter(Mandatory = $true)][string]$Path)
  $json = $Value | ConvertTo-Json -Depth 20
  [System.IO.File]::WriteAllText($Path, $json + [Environment]::NewLine, [System.Text.UTF8Encoding]::new($false))
}

function Write-E2EEvent {
  param(
    [Parameter(Mandatory = $true)][string]$Phase,
    [Parameter(Mandatory = $true)][string]$Status,
    [Parameter(Mandatory = $true)][string]$Message,
    [hashtable]$Data = @{}
  )
  $safeMessage = Protect-E2EText $Message
  $event = [ordered]@{
    timestamp = (Get-Date).ToUniversalTime().ToString("o")
    run_id = $RunID
    phase = $Phase
    status = $Status
    message = $safeMessage
    data = $Data
  }
  Add-Content -LiteralPath $StructuredLog -Value ($event | ConvertTo-Json -Depth 10 -Compress) -Encoding UTF8
  Write-Host ("[{0}] {1}: {2}" -f $Phase, $Status, $safeMessage)
}

function Get-E2ERemainingSeconds {
  $remaining = [int]($Deadline - [DateTime]::UtcNow).TotalSeconds
  if ($remaining -le 0) {
    throw "E2E deadline exceeded after $TimeoutMinutes minutes"
  }
  return $remaining
}

function Invoke-E2EStage {
  param(
    [Parameter(Mandatory = $true)][string]$Name,
    [Parameter(Mandatory = $true)][string]$Classification,
    [Parameter(Mandatory = $true)][scriptblock]$Action
  )
  $script:CurrentStage = $Name
  $started = Get-Date
  Write-E2EEvent -Phase $Name -Status "started" -Message "$Classification stage started" -Data @{ classification = $Classification }
  try {
    & $Action
    $result = [ordered]@{
      name = $Name
      classification = $Classification
      status = "passed"
      duration_seconds = [Math]::Round(((Get-Date) - $started).TotalSeconds, 3)
    }
    $script:StageResults.Add([pscustomobject]$result)
    Write-E2EEvent -Phase $Name -Status "passed" -Message "$Classification stage passed"
  } catch {
    $message = Protect-E2EText $_.Exception.Message
    $result = [ordered]@{
      name = $Name
      classification = $Classification
      status = "failed"
      duration_seconds = [Math]::Round(((Get-Date) - $started).TotalSeconds, 3)
      error = $message
    }
    $script:StageResults.Add([pscustomobject]$result)
    Write-E2EEvent -Phase $Name -Status "failed" -Message $message
    throw
  }
}

function Add-E2ESkippedStage {
  param([string]$Name, [string]$Classification, [string]$Reason)
  $script:StageResults.Add([pscustomobject]@{
      name = $Name
      classification = $Classification
      status = "skipped"
      duration_seconds = 0
      reason = $Reason
    })
  Write-E2EEvent -Phase $Name -Status "skipped" -Message $Reason -Data @{ classification = $Classification }
}

function Get-E2ECommandEnvironment {
  $environment = @{
    TEMP = $RuntimeTemp
    TMP = $RuntimeTemp
    NO_PROXY = "127.0.0.1,localhost"
  }
  if (-not [string]::IsNullOrWhiteSpace($Proxy)) {
    $environment["HTTP_PROXY"] = $Proxy
    $environment["HTTPS_PROXY"] = $Proxy
    $environment["ALL_PROXY"] = $Proxy
  }
  return $environment
}

function Invoke-E2EExternal {
  param(
    [Parameter(Mandatory = $true)][string]$FilePath,
    [string[]]$Arguments = @(),
    [Parameter(Mandatory = $true)][string]$WorkingDirectory,
    [Parameter(Mandatory = $true)][string]$Activity
  )
  $script:CommandSequence++
  $safeName = ($Activity -replace '[^A-Za-z0-9._-]', '-')
  $prefix = "{0:D2}-{1}" -f $script:CommandSequence, $safeName
  $stdout = Join-Path $CommandLogRoot "$prefix.stdout.log"
  $stderr = Join-Path $CommandLogRoot "$prefix.stderr.log"
  return Invoke-PalPanelExternal `
    -FilePath $FilePath `
    -Arguments $Arguments `
    -WorkingDirectory $WorkingDirectory `
    -TimeoutSeconds (Get-E2ERemainingSeconds) `
    -StdoutPath $stdout `
    -StderrPath $stderr `
    -Environment (Get-E2ECommandEnvironment) `
    -Activity $Activity
}

function Get-AvailableTCPPort {
  $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
  try {
    $listener.Start()
    return ([System.Net.IPEndPoint]$listener.LocalEndpoint).Port
  } finally {
    $listener.Stop()
  }
}

function Invoke-E2EApi {
  param(
    [Parameter(Mandatory = $true)][ValidateSet("GET", "POST", "PUT", "DELETE")][string]$Method,
    [Parameter(Mandatory = $true)][string]$Path,
    $Body = $null
  )
  if ([string]::IsNullOrWhiteSpace($script:BaseURL)) {
    throw "E2E backend is not running"
  }
  $parameters = @{
    Method = $Method
    Uri = $script:BaseURL + $Path
    TimeoutSec = [Math]::Min(30, (Get-E2ERemainingSeconds))
    ErrorAction = "Stop"
  }
  if ($null -ne $Body) {
    $parameters["ContentType"] = "application/json"
    $parameters["Body"] = ($Body | ConvertTo-Json -Depth 12 -Compress)
  }
  try {
    $response = Invoke-RestMethod @parameters
  } catch {
    $detail = $_.Exception.Message
    if ($_.ErrorDetails -and $_.ErrorDetails.Message) {
      try {
        $decoded = $_.ErrorDetails.Message | ConvertFrom-Json
        if ($decoded.error.message) {
          $detail = $decoded.error.message
        }
      } catch { }
    }
    throw (Protect-E2EText "$Method $Path failed: $detail")
  }
  if ($response.PSObject.Properties.Name -contains "ok" -and -not $response.ok) {
    throw "$Method $Path returned an unsuccessful response"
  }
  if ($response.PSObject.Properties.Name -contains "data") {
    return $response.data
  }
  return $response
}

function Wait-E2EJob {
  param([Parameter(Mandatory = $true)][string]$ID, [Parameter(Mandatory = $true)][string]$Label)
  $last = ""
  while ($true) {
    $job = Invoke-E2EApi -Method GET -Path "/api/jobs/$ID"
    $state = "$($job.status):$($job.progress):$($job.message)"
    if ($state -ne $last) {
      Write-E2EEvent -Phase $script:CurrentStage -Status "progress" -Message "$Label`: $($job.progress)% $($job.message)" -Data @{
        job_id = $ID
        job_status = [string]$job.status
        progress = [int]$job.progress
      }
      $last = $state
    }
    switch ([string]$job.status) {
      "completed" { return $job }
      "success" { return $job }
      "failed" {
        $errorText = if ($job.error) { [string]$job.error } else { [string]$job.message }
        throw "$Label failed: $(Protect-E2EText $errorText)"
      }
    }
    Get-E2ERemainingSeconds | Out-Null
    Start-Sleep -Seconds 2
  }
}

function Start-E2EBackend {
  param([Parameter(Mandatory = $true)][string]$PackageDirectory)
  $server = Join-Path $PackageDirectory "palpanel-server.exe"
  if (-not (Test-Path -LiteralPath $server -PathType Leaf)) {
    throw "packaged backend is missing: $server"
  }
  $config = Join-Path $RuntimeRoot "config\palpanel.env"
  Invoke-E2EExternal -FilePath $server -WorkingDirectory $PackageDirectory -Activity "initialize-e2e-config" -Arguments @(
    "--runtime-root", $RuntimeRoot, "--config", $config, "--init-config"
  ) | Out-Null

  $port = Get-AvailableTCPPort
  $stdoutPath = Join-Path $ArtifactRoot "palpanel-server.stdout.log"
  $stderrPath = Join-Path $ArtifactRoot "palpanel-server.stderr.log"
  $info = [System.Diagnostics.ProcessStartInfo]::new()
  $info.FileName = $server
  $info.WorkingDirectory = $PackageDirectory
  $info.UseShellExecute = $false
  $info.CreateNoWindow = $true
  $info.RedirectStandardOutput = $true
  $info.RedirectStandardError = $true
  Set-PalPanelProcessArguments -StartInfo $info -Arguments @("--runtime-root", $RuntimeRoot, "--config", $config)
  $backendEnvironment = Get-E2ECommandEnvironment
  $backendEnvironment["PALPANEL_RUNTIME_ROOT"] = $RuntimeRoot
  $backendEnvironment["PALPANEL_LISTEN_ADDR"] = "127.0.0.1:$port"
  $backendEnvironment["PALPANEL_REQUIRE_AUTH"] = "false"
  $backendEnvironment["PALPANEL_SAVE_INDEXER_ENABLED"] = "false"
  Set-PalPanelProcessEnvironment -StartInfo $info -Environment $backendEnvironment

  $process = [System.Diagnostics.Process]::new()
  $process.StartInfo = $info
  if (-not $process.Start()) {
    throw "failed to start packaged palpanel-server.exe"
  }
  $stdoutTask = $process.StandardOutput.ReadToEndAsync()
  $stderrTask = $process.StandardError.ReadToEndAsync()
  $script:Backend = [pscustomobject]@{
    Process = $process
    StdoutTask = $stdoutTask
    StderrTask = $stderrTask
    StdoutPath = $stdoutPath
    StderrPath = $stderrPath
    PID = $process.Id
  }
  $script:BaseURL = "http://127.0.0.1:$port"

  $healthDeadline = [DateTime]::UtcNow.AddSeconds([Math]::Min(60, (Get-E2ERemainingSeconds)))
  while ([DateTime]::UtcNow -lt $healthDeadline) {
    if ($process.HasExited) {
      throw "palpanel-server.exe exited before health check with code $($process.ExitCode)"
    }
    try {
      $health = Invoke-RestMethod -Method GET -Uri "$($script:BaseURL)/api/health" -TimeoutSec 2
      if ($health.data.status -eq "ok") {
        Write-E2EEvent -Phase $script:CurrentStage -Status "progress" -Message "backend health check passed" -Data @{ pid = $process.Id; url = $script:BaseURL }
        return
      }
    } catch { }
    Start-Sleep -Milliseconds 500
  }
  throw "palpanel-server.exe did not become healthy within 60 seconds"
}

function Stop-E2EBackend {
  if ($null -eq $script:Backend) { return }
  $backend = $script:Backend
  try {
    if (-not $backend.Process.HasExited) {
      try {
        $script:CommandSequence++
        $cleanupPrefix = "{0:D2}-stop-e2e-backend-tree" -f $script:CommandSequence
        Invoke-PalPanelExternal `
          -FilePath "$env:SystemRoot\System32\taskkill.exe" `
          -WorkingDirectory $RepositoryRoot `
          -Arguments @("/PID", [string]$backend.PID, "/T", "/F") `
          -TimeoutSeconds 30 `
          -StdoutPath (Join-Path $CommandLogRoot "$cleanupPrefix.stdout.log") `
          -StderrPath (Join-Path $CommandLogRoot "$cleanupPrefix.stderr.log") `
          -Environment (Get-E2ECommandEnvironment) `
          -Activity "stop-e2e-backend-tree" | Out-Null
      } catch {
        if (-not $backend.Process.HasExited) {
          throw
        }
      }
      $backend.Process.WaitForExit(10000) | Out-Null
    }
    $stdout = $backend.StdoutTask.Result
    $stderr = $backend.StderrTask.Result
    $utf8 = [System.Text.UTF8Encoding]::new($false)
    [System.IO.File]::WriteAllText($backend.StdoutPath, $stdout, $utf8)
    [System.IO.File]::WriteAllText($backend.StderrPath, $stderr, $utf8)
  } finally {
    $backend.Process.Dispose()
    $script:Backend = $null
    $script:BaseURL = ""
  }
}

function Get-PalServerProcesses {
  $gameRoot = [System.IO.Path]::GetFullPath((Join-Path $RuntimeRoot "palworld"))
  return @(Get-CimInstance Win32_Process | Where-Object {
      $_.ExecutablePath -and (Test-PalPanelPathWithin -Root $gameRoot -Target $_.ExecutablePath)
    } | Select-Object ProcessId, Name, ExecutablePath, CommandLine)
}

function Wait-LiveServerEvidence {
  param(
    [int[]]$PreviousPIDs = @(),
    [string]$EvidenceFileName = "live-server-evidence.json"
  )
  $waitUntil = [DateTime]::UtcNow.AddSeconds([Math]::Min(300, (Get-E2ERemainingSeconds)))
  $logPath = Join-Path $RuntimeRoot "data\logs\palserver.log"
  while ([DateTime]::UtcNow -lt $waitUntil) {
    $status = Invoke-E2EApi -Method GET -Path "/api/server/status"
    $processes = @(Get-PalServerProcesses)
    $newProcesses = @($processes | Where-Object { $PreviousPIDs -notcontains [int]$_.ProcessId })
    $ports = @($status.ports.PSObject.Properties | ForEach-Object { [int]$_.Value } | Where-Object { $_ -gt 0 })
    $processIDs = @($processes | ForEach-Object { [int]$_.ProcessId })
    $endpoint = $null
    if ($processIDs.Count -gt 0 -and $ports.Count -gt 0) {
      try {
        $endpoint = Get-NetUDPEndpoint -ErrorAction Stop | Where-Object {
          $processIDs -contains [int]$_.OwningProcess -and $ports -contains [int]$_.LocalPort
        } | Select-Object -First 1
      } catch { }
    }
    $logReady = (Test-Path -LiteralPath $logPath -PathType Leaf) -and ((Get-Item -LiteralPath $logPath).Length -gt 0)
    if ($status.container.status -eq "running" -and $newProcesses.Count -gt 0 -and $null -ne $endpoint -and $logReady) {
      $evidence = [ordered]@{
        checked_at = (Get-Date).ToUniversalTime().ToString("o")
        status = $status
        processes = $processes
        udp_endpoint = $endpoint
        log_path = $logPath
        log_bytes = (Get-Item -LiteralPath $logPath).Length
      }
      Write-E2EJsonFile -Value $evidence -Path (Join-Path $ArtifactRoot $EvidenceFileName)
      return $evidence
    }
    Start-Sleep -Seconds 2
  }
  throw "Palworld server did not produce process, UDP endpoint, and log evidence within 300 seconds"
}

function Wait-LiveServerStopped {
  $waitUntil = [DateTime]::UtcNow.AddSeconds([Math]::Min(60, (Get-E2ERemainingSeconds)))
  while ([DateTime]::UtcNow -lt $waitUntil) {
    $status = Invoke-E2EApi -Method GET -Path "/api/server/status"
    $processes = @(Get-PalServerProcesses)
    if ($status.container.status -ne "running" -and $processes.Count -eq 0) {
      return
    }
    Start-Sleep -Seconds 1
  }
  throw "Palworld process tree remained after the stop request"
}

function Assert-E2EInstallPaths {
  $required = @(
    (Join-Path $RuntimeRoot "steamcmd\steamcmd.exe"),
    (Join-Path $RuntimeRoot "palworld\PalServer.exe"),
    (Join-Path $RuntimeRoot "palworld\Pal\Binaries\Win64\PalServer-Win64-Shipping-Cmd.exe"),
    (Join-Path $RuntimeRoot "palworld\steamapps\appmanifest_2394010.acf")
  )
  foreach ($path in $required) {
    Assert-PalPanelManagedPath -RepositoryRoot $RepositoryRoot -TargetPath $path | Out-Null
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
      throw "installed game evidence is missing: $path"
    }
    if ((Get-Item -LiteralPath $path).Length -eq 0) {
      throw "installed game evidence is empty: $path"
    }
  }
}

Write-E2EJsonFile -Value ([ordered]@{
    run_id = $RunID
    repository_root = $RepositoryRoot
    runtime_root = $RuntimeRoot
    test_root = $TestRoot
    created_at = (Get-Date).ToUniversalTime().ToString("o")
  }) -Path $TestMarker
Write-E2EEvent -Phase "initialization" -Status "passed" -Message "isolated Windows E2E roots initialized" -Data @{
  repository_root = $RepositoryRoot
  runtime_root = $RuntimeRoot
  test_root = $TestRoot
  artifact_root = $ArtifactRoot
}

$Version = if (-not [string]::IsNullOrWhiteSpace($env:CI_PACKAGE_VERSION)) { $env:CI_PACKAGE_VERSION } else { "v0.0.0-windows-e2e" }
$PackageName = "palpanel_${Version}_windows_amd64"
$PackageRoot = Join-Path $RuntimeRoot "package"
$PackageDir = Join-Path $PackageRoot $PackageName
$Archive = Join-Path $PackageRoot "$PackageName.zip"
$PowerShellExe = [System.Diagnostics.Process]::GetCurrentProcess().MainModule.FileName

try {
  Invoke-E2EStage -Name "parameter-validation" -Classification "windows-e2e" -Action {
    if ($Version -notmatch '^[A-Za-z0-9][A-Za-z0-9._+-]*$') {
      throw "unsafe package version: $Version"
    }
    if (-not [string]::IsNullOrWhiteSpace($Proxy)) {
      $proxyURI = $null
      if (-not [uri]::TryCreate($Proxy, [UriKind]::Absolute, [ref]$proxyURI) -or
          $proxyURI.Scheme -notin @("http", "https", "socks5") -or
          -not [string]::IsNullOrEmpty($proxyURI.UserInfo)) {
        throw "Proxy must be an absolute http, https, or socks5 URL without embedded credentials"
      }
    }
    if (-not [string]::IsNullOrWhiteSpace($TranslationBaseURL)) {
      $translationURI = $null
      if (-not [uri]::TryCreate($TranslationBaseURL, [UriKind]::Absolute, [ref]$translationURI) -or -not [string]::IsNullOrEmpty($translationURI.UserInfo)) {
        throw "TranslationBaseURL must be an absolute URL without embedded credentials"
      }
      if ([string]::IsNullOrWhiteSpace($TranslationAPIKeyEnv)) {
        throw "TranslationAPIKeyEnv is required when TranslationBaseURL is set"
      }
      if ($TranslationAPIKeyEnv -notmatch '^[A-Za-z_][A-Za-z0-9_]*$') {
        throw "TranslationAPIKeyEnv is not a valid environment variable name"
      }
      $translationKey = [Environment]::GetEnvironmentVariable($TranslationAPIKeyEnv)
      if ([string]::IsNullOrWhiteSpace($translationKey)) {
        throw "translation API key environment variable is empty: $TranslationAPIKeyEnv"
      }
      $script:SensitiveValues.Add($translationKey)
    }
  }

  Invoke-E2EStage -Name "path-contract" -Classification "destructive-test-root-only" -Action {
    Invoke-E2EExternal -FilePath $PowerShellExe -WorkingDirectory $RepositoryRoot -Activity "windows-path-contract" -Arguments @(
      "-NoLogo", "-NoProfile", "-NonInteractive", "-File", (Join-Path $ScriptDir "test-windows-paths.ps1"),
      "-RuntimeRoot", $RuntimeRoot, "-TestRoot", $TestRoot
    ) | Out-Null
  }

  Invoke-E2EStage -Name "package" -Classification "windows-smoke" -Action {
    if (-not (Test-Path -LiteralPath $PackageDir -PathType Container) -or -not (Test-Path -LiteralPath $Archive -PathType Leaf)) {
      $arguments = @(
        "-NoLogo", "-NoProfile", "-NonInteractive", "-File", (Join-Path $ScriptDir "package.ps1"),
        "-Version", $Version, "-RuntimeRoot", $RuntimeRoot, "-Clean"
      )
      if (-not [string]::IsNullOrWhiteSpace($env:PALPANEL_MINGW_GCC)) {
        $arguments += @("-MingwGcc", $env:PALPANEL_MINGW_GCC)
      }
      Invoke-E2EExternal -FilePath $PowerShellExe -WorkingDirectory $RepositoryRoot -Activity "windows-package" -Arguments $arguments | Out-Null
    } else {
      Write-E2EEvent -Phase "package" -Status "progress" -Message "reusing existing package for $Version"
    }
    if (-not (Test-Path -LiteralPath $PackageDir -PathType Container) -or -not (Test-Path -LiteralPath $Archive -PathType Leaf)) {
      throw "Windows package output is incomplete"
    }
  }

  Invoke-E2EStage -Name "verify-package" -Classification "windows-smoke" -Action {
    $arguments = @(
      "-NoLogo", "-NoProfile", "-NonInteractive", "-File", (Join-Path $ScriptDir "verify-windows-package.ps1"),
      "-Archive", $Archive, "-RuntimeRoot", $RuntimeRoot
    )
    $objdump = ""
    foreach ($candidate in @(
        $env:PALPANEL_OBJDUMP_PATH,
        $(if ($env:RUNNER_TEMP) { Join-Path $env:RUNNER_TEMP "msys64\mingw64\bin\objdump.exe" }),
        "C:\msys64\mingw64\bin\objdump.exe"
      )) {
      if ($candidate) {
        $resolvedCandidate = Resolve-PalPanelPath -Path $candidate -BasePath $RepositoryRoot
        if (Test-Path -LiteralPath $resolvedCandidate -PathType Leaf) {
          $objdump = $resolvedCandidate
          break
        }
      }
    }
    if ($objdump) { $arguments += @("-ObjdumpPath", $objdump) }
    if ($KeepArtifacts) { $arguments += "-KeepArtifacts" }
    Invoke-E2EExternal -FilePath $PowerShellExe -WorkingDirectory $RepositoryRoot -Activity "verify-windows-package" -Arguments $arguments | Out-Null
  }

  Invoke-E2EStage -Name "launcher-smoke" -Classification "windows-smoke" -Action {
    Invoke-E2EExternal -FilePath $PowerShellExe -WorkingDirectory $RepositoryRoot -Activity "launcher-process-smoke" -Arguments @(
      "-NoLogo", "-NoProfile", "-NonInteractive", "-File", (Join-Path $ScriptDir "smoke-windows.ps1"),
      "-PackageDir", $PackageDir, "-Version", $Version, "-RuntimeRoot", $TestRoot
    ) | Out-Null
  }

  $needsBackend = (-not $SkipGameDownload) -or (-not $SkipLiveServer) -or (-not $SkipPalDefender) -or (-not [string]::IsNullOrWhiteSpace($TranslationBaseURL))
  if ($needsBackend) {
    Invoke-E2EStage -Name "backend-harness" -Classification "windows-e2e" -Action {
      Start-E2EBackend -PackageDirectory $PackageDir
      Invoke-E2EApi -Method PUT -Path "/api/server/runtime" -Body @{ mode = "windows_steamcmd" } | Out-Null
    }
  } else {
    Add-E2ESkippedStage -Name "backend-harness" -Classification "windows-e2e" -Reason "all live and translation stages were explicitly skipped"
  }

  if (-not [string]::IsNullOrWhiteSpace($TranslationBaseURL)) {
    Invoke-E2EStage -Name "translation" -Classification "integration" -Action {
      $key = [Environment]::GetEnvironmentVariable($TranslationAPIKeyEnv)
      $update = @{ base_url = $TranslationBaseURL.TrimEnd('/'); model = "palpanel-e2e"; api_key = $key }
      $config = Invoke-E2EApi -Method PUT -Path "/api/ai/translation/config" -Body $update
      if (-not $config.configured -or -not $config.api_key_present) {
        throw "translation configuration was not persisted"
      }
      $test = Invoke-E2EApi -Method POST -Path "/api/ai/translation/test" -Body @{}
      if (-not $test.ok) { throw "translation provider connection test failed" }
      Write-E2EJsonFile -Value $test -Path (Join-Path $ArtifactRoot "translation-test.json")
    }
  } else {
    Add-E2ESkippedStage -Name "translation" -Classification "integration" -Reason "TranslationBaseURL was not provided; backend mock/unit coverage remains in the normal test suite"
  }

  if (-not $SkipGameDownload) {
    Invoke-E2EStage -Name "game-install" -Classification "windows-live" -Action {
      $job = Invoke-E2EApi -Method POST -Path "/api/server/bootstrap" -Body @{}
      Wait-E2EJob -ID ([string]$job.id) -Label "Palworld bootstrap" | Out-Null
      Assert-E2EInstallPaths
      $versionInfo = Invoke-E2EApi -Method GET -Path "/api/server/version"
      Write-E2EJsonFile -Value $versionInfo -Path (Join-Path $ArtifactRoot "game-version.json")
    }
  } else {
    Add-E2ESkippedStage -Name "game-install" -Classification "windows-live" -Reason "SkipGameDownload was specified; no SteamCMD or game network download was attempted"
  }

  if (-not $SkipPalDefender) {
    Invoke-E2EStage -Name "paldefender" -Classification "windows-live" -Action {
      Assert-E2EInstallPaths
      $job = Invoke-E2EApi -Method POST -Path "/api/security/paldefender/install" -Body @{}
      Wait-E2EJob -ID ([string]$job.id) -Label "PalDefender and UE4SS installation" | Out-Null
      $status = Invoke-E2EApi -Method GET -Path "/api/security/paldefender/status"
      Write-E2EJsonFile -Value $status -Path (Join-Path $ArtifactRoot "paldefender-status.json")
      if (-not $status.installed) { throw "PalDefender status is not installed after the install job" }
      if (-not $status.ue4ss.installed -or $status.ue4ss.state -ne "installed") {
        throw "UE4SS dependency status is not installed after the PalDefender install job"
      }
      $script:PalDefenderInstalled = $true
    }
  } else {
    Add-E2ESkippedStage -Name "paldefender" -Classification "windows-live" -Reason "SkipPalDefender was specified; no PalDefender or UE4SS network installation was attempted"
  }

  if (-not $SkipLiveServer) {
    Invoke-E2EStage -Name "live-server" -Classification "windows-live" -Action {
      Assert-E2EInstallPaths
      Invoke-E2EApi -Method POST -Path "/api/server/initialize-config" -Body @{} | Out-Null
      Invoke-E2EApi -Method POST -Path "/api/server/start" -Body @{} | Out-Null
      $script:LiveServerStarted = $true
      $firstEvidence = Wait-LiveServerEvidence
      $firstPIDs = @($firstEvidence.processes | ForEach-Object { [int]$_.ProcessId })

      Invoke-E2EApi -Method POST -Path "/api/server/restart" -Body @{} | Out-Null
      $restartEvidence = Wait-LiveServerEvidence -PreviousPIDs $firstPIDs -EvidenceFileName "live-server-restart-evidence.json"
      foreach ($oldPID in $firstPIDs) {
        if (Get-Process -Id $oldPID -ErrorAction SilentlyContinue) {
          throw "restart left the previous Palworld process running: $oldPID"
        }
      }

      Invoke-E2EApi -Method POST -Path "/api/server/stop" -Body @{} | Out-Null
      Wait-LiveServerStopped
      $script:LiveServerStarted = $false

      if ($script:PalDefenderInstalled) {
        $dependencyStatus = Invoke-E2EApi -Method GET -Path "/api/security/paldefender/status"
        $loadEvidence = [ordered]@{
          ue4ss_observed = [bool]$dependencyStatus.ue4ss.load_verified
          ue4ss_evidence = $dependencyStatus.ue4ss.load_evidence
          paldefender_observed = [bool]$dependencyStatus.load_verified
          paldefender_evidence = $dependencyStatus.load_evidence
          ue4ss_log_path = Join-Path $RuntimeRoot "palworld\Pal\Binaries\Win64\UE4SS.log"
          paldefender_log_dir = Join-Path $RuntimeRoot "palworld\Pal\Binaries\Win64\PalDefender\Logs"
        }
        Write-E2EJsonFile -Value $loadEvidence -Path (Join-Path $ArtifactRoot "dependency-load-evidence.json")
        if (-not $loadEvidence.ue4ss_observed -or -not $loadEvidence.paldefender_observed) {
          Write-E2EEvent -Phase "live-server" -Status "warning" -Message "UE4SS or PalDefender load evidence was not observed in the component startup logs; installation status is retained for manual review"
        }
      }
    }
  } else {
    Add-E2ESkippedStage -Name "live-server" -Classification "windows-live" -Reason "SkipLiveServer was specified; no Palworld server process was started"
  }

  Invoke-E2EStage -Name "path-audit" -Classification "windows-e2e" -Action {
    $paths = [ordered]@{
      repository_root = $RepositoryRoot
      runtime_root = $RuntimeRoot
      test_root = $TestRoot
      artifacts = $ArtifactRoot
      steamcmd = Join-Path $RuntimeRoot "steamcmd"
      palworld_server = Join-Path $RuntimeRoot "palworld"
      config = Join-Path $RuntimeRoot "config"
      database = Join-Path $RuntimeRoot "data\database"
      logs = Join-Path $RuntimeRoot "data\logs"
      saves = Join-Path $RuntimeRoot "data\saves"
      mods = Join-Path $RuntimeRoot "mods"
      ue4ss = Join-Path $RuntimeRoot "ue4ss"
      paldefender = Join-Path $RuntimeRoot "paldefender"
      package = $PackageRoot
      temp = Join-Path $RuntimeRoot "temp"
    }
    foreach ($entry in $paths.GetEnumerator()) {
      if ($entry.Key -eq "repository_root") { continue }
      Assert-PalPanelManagedPath -RepositoryRoot $RepositoryRoot -TargetPath ([string]$entry.Value) -AllowManagedRoot | Out-Null
    }
    Write-E2EJsonFile -Value $paths -Path $PathsPath
    Write-Host "Repository root: $RepositoryRoot"
    Write-Host "Windows runtime root: $RuntimeRoot"
    Write-Host "SteamCMD: $(Join-Path $RuntimeRoot 'steamcmd')"
    Write-Host "Palworld server: $(Join-Path $RuntimeRoot 'palworld')"
    Write-Host "Artifacts: $ArtifactRoot"
  }

  $script:RunSucceeded = $true
} catch {
  $script:RunFailure = $_
  Write-E2EEvent -Phase $script:CurrentStage -Status "run-failed" -Message $_.Exception.Message
} finally {
  if ($script:LiveServerStarted -and -not [string]::IsNullOrWhiteSpace($script:BaseURL)) {
    try {
      Invoke-E2EApi -Method POST -Path "/api/server/stop" -Body @{} | Out-Null
      Wait-LiveServerStopped
      $script:LiveServerStarted = $false
    } catch {
      Write-E2EEvent -Phase "cleanup" -Status "warning" -Message "failed to stop Palworld through the API: $($_.Exception.Message)"
    }
  }
  try {
    Stop-E2EBackend
  } catch {
    Write-E2EEvent -Phase "cleanup" -Status "warning" -Message "failed to stop E2E backend tree cleanly: $($_.Exception.Message)"
    if ($null -eq $script:RunFailure) { $script:RunFailure = $_ }
    $script:RunSucceeded = $false
  }

  if ($script:RunSucceeded -and -not $KeepArtifacts) {
    try {
      Remove-PalPanelManagedDirectory -RepositoryRoot $RepositoryRoot -RuntimeRoot $RuntimeRoot -TargetPath $TestRoot
      Remove-PalPanelManagedDirectory -RepositoryRoot $RepositoryRoot -RuntimeRoot $RuntimeRoot -TargetPath $RuntimeTemp
      Write-E2EEvent -Phase "cleanup" -Status "passed" -Message "temporary test root cleaned; SteamCMD, game files, package, and reports were retained"
    } catch {
      Write-E2EEvent -Phase "cleanup" -Status "warning" -Message "temporary cleanup failed: $($_.Exception.Message)"
      if ($null -eq $script:RunFailure) { $script:RunFailure = $_ }
      $script:RunSucceeded = $false
    }
  } else {
    Write-E2EEvent -Phase "cleanup" -Status "retained" -Message "test root and runtime temp were retained for diagnosis"
  }

  $summary = [ordered]@{
    schema_version = 1
    run_id = $RunID
    outcome = if ($script:RunSucceeded) { "passed" } else { "failed" }
    started_at = $RunStarted.ToUniversalTime().ToString("o")
    finished_at = (Get-Date).ToUniversalTime().ToString("o")
    duration_seconds = [Math]::Round(((Get-Date) - $RunStarted).TotalSeconds, 3)
    repository_root = $RepositoryRoot
    runtime_root = $RuntimeRoot
    test_root = $TestRoot
    artifact_root = $ArtifactRoot
    package_archive = $Archive
    flags = [ordered]@{
      keep_artifacts = [bool]$KeepArtifacts
      skip_game_download = [bool]$SkipGameDownload
      skip_live_server = [bool]$SkipLiveServer
      skip_paldefender = [bool]$SkipPalDefender
      proxy_configured = -not [string]::IsNullOrWhiteSpace($Proxy)
      translation_configured = -not [string]::IsNullOrWhiteSpace($TranslationBaseURL)
      timeout_minutes = $TimeoutMinutes
    }
    phases = $script:StageResults
    error = if ($null -ne $script:RunFailure) { Protect-E2EText $script:RunFailure.Exception.Message } else { "" }
  }
  Write-E2EJsonFile -Value $summary -Path $SummaryPath
}

if (-not $script:RunSucceeded) {
  throw "Windows E2E failed; artifacts retained at $ArtifactRoot`: $(Protect-E2EText $script:RunFailure.Exception.Message)"
}

Write-Host "Windows E2E passed"
Write-Host "Summary: $SummaryPath"
Write-Host "Artifacts: $ArtifactRoot"
