param(
  [Parameter(Mandatory = $true)]
  [string]$PackageDir,
  [Parameter(Mandatory = $true)]
  [string]$Version
)

$ErrorActionPreference = "Stop"
$Package = (Resolve-Path $PackageDir).Path

(& "$Package\palpanel-server.exe" --version | Out-String) | Select-String ([regex]::Escape($Version)) | Out-Null
(& "$Package\sav-cli.exe" --version | Out-String) | Select-String ([regex]::Escape($Version)) | Out-Null
(& "$Package\PalPanel.exe" --version | Out-String) | Select-String ([regex]::Escape($Version)) | Out-Null

$process = Start-Process -FilePath "$Package\PalPanel.exe" -ArgumentList "--no-browser", "--no-prompt", "--exit-after-health" -PassThru -Wait
if ($process.ExitCode -ne 0) {
  throw "Launcher smoke failed with exit code $($process.ExitCode)"
}
if (-not (Test-Path "$Package\config\palpanel.env")) {
  throw "Launcher did not initialize config"
}
Start-Sleep -Seconds 2
$prefix = $Package.ToLowerInvariant()
$remaining = Get-CimInstance Win32_Process | Where-Object {
  $_.ExecutablePath -and $_.ExecutablePath.ToLowerInvariant().StartsWith($prefix)
}
if ($remaining) {
  throw "Launcher left child processes running: $($remaining.Name -join ', ')"
}
Write-Host "Windows Launcher and process smoke test passed"
