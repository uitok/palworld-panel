$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$PackageDir = Split-Path -Parent $ScriptDir
$Launcher = Join-Path $PackageDir "PalPanel.exe"
if (-not (Test-Path $Launcher)) {
  $Launcher = Join-Path $ScriptDir "PalPanel.exe"
}
if (-not (Test-Path $Launcher)) {
  throw "PalPanel.exe was not found"
}
& $Launcher @args
exit $LASTEXITCODE
