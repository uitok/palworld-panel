param(
  [Parameter(Mandatory = $true)]
  [string]$PackageDir,
  [Parameter(Mandatory = $true)]
  [string]$Version,
  [string]$RuntimeRoot = ""
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
$ScriptDir = $PSScriptRoot
Import-Module (Join-Path $ScriptDir "windows-e2e-common.psm1") -Force
$RepositoryRoot = Get-PalPanelRepositoryRoot
$Package = Resolve-PalPanelPath -Path $PackageDir -BasePath $RepositoryRoot
if (-not (Test-Path -LiteralPath $Package -PathType Container)) {
  throw "Windows package directory is missing: $Package"
}
if ([string]::IsNullOrWhiteSpace($RuntimeRoot)) {
  $RuntimeRoot = Join-Path $RepositoryRoot "dev-runtime\windows\smoke\run-$([guid]::NewGuid().ToString('N'))"
} else {
  $RuntimeRoot = Resolve-PalPanelPath -Path $RuntimeRoot -BasePath $RepositoryRoot
}
$RuntimeRoot = Initialize-PalPanelWindowsLayout -RepositoryRoot $RepositoryRoot -RuntimeRoot $RuntimeRoot
$SmokeID = "smoke-$([guid]::NewGuid().ToString('N'))"
$ArtifactDir = Join-Path $RuntimeRoot "artifacts\$SmokeID"
New-Item -ItemType Directory -Force -Path $ArtifactDir | Out-Null
$environment = @{
  TEMP = (Join-Path $RuntimeRoot "temp")
  TMP = (Join-Path $RuntimeRoot "temp")
  PALPANEL_RUNTIME_ROOT = $RuntimeRoot
  NO_PROXY = "127.0.0.1,localhost"
}

function Invoke-SmokeExecutable {
  param([string]$Name, [string[]]$Arguments)
  $stdout = Join-Path $ArtifactDir "$Name.stdout.log"
  $stderr = Join-Path $ArtifactDir "$Name.stderr.log"
  Invoke-PalPanelExternal `
    -FilePath (Join-Path $Package $Name) `
    -Arguments $Arguments `
    -WorkingDirectory $Package `
    -TimeoutSeconds 120 `
    -StdoutPath $stdout `
    -StderrPath $stderr `
    -Environment $environment `
    -Activity "$Name smoke" | Out-Null
  $content = Get-Content -LiteralPath $stdout -Raw
  return $content
}

foreach ($executable in @("palpanel-server.exe", "sav-cli.exe", "PalPanel.exe")) {
  $output = Invoke-SmokeExecutable -Name $executable -Arguments @("--version")
  if ($output -notmatch [regex]::Escape($Version)) {
    throw "$executable --version did not contain $Version"
  }
}

Invoke-SmokeExecutable -Name "PalPanel.exe" -Arguments @(
  "--no-browser", "--no-prompt", "--exit-after-health", "--runtime-root", $RuntimeRoot
) | Out-Null

$runtimeConfig = Join-Path $RuntimeRoot "config\palpanel.env"
if (-not (Test-Path -LiteralPath $runtimeConfig -PathType Leaf)) {
  throw "Launcher did not initialize runtime config: $runtimeConfig"
}
if (Test-Path -LiteralPath (Join-Path $Package "config\palpanel.env")) {
  throw "Launcher polluted the release package instead of the selected runtime root"
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
Write-Host "Runtime root: $RuntimeRoot"
Write-Host "Smoke artifacts: $ArtifactDir"
