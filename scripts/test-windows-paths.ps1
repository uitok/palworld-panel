param(
  [string]$RuntimeRoot = "",
  [string]$TestRoot = ""
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$ScriptDir = $PSScriptRoot
Import-Module (Join-Path $ScriptDir "windows-e2e-common.psm1") -Force
$RepositoryRoot = Get-PalPanelRepositoryRoot

if ([string]::IsNullOrWhiteSpace($RuntimeRoot)) {
  $RuntimeRoot = Join-Path $RepositoryRoot "dev-runtime\windows"
} else {
  $RuntimeRoot = Resolve-PalPanelPath -Path $RuntimeRoot -BasePath $RepositoryRoot
}
$RuntimeRoot = Assert-PalPanelManagedPath -RepositoryRoot $RepositoryRoot -TargetPath $RuntimeRoot -AllowManagedRoot

if ([string]::IsNullOrWhiteSpace($TestRoot)) {
  $TestRoot = Join-Path $RuntimeRoot "e2e\path-contract"
} else {
  $TestRoot = Resolve-PalPanelPath -Path $TestRoot -BasePath $RuntimeRoot
}
$TestRoot = Assert-PalPanelManagedPath -RepositoryRoot $RepositoryRoot -TargetPath $TestRoot
if (-not (Test-PalPanelPathWithin -Root $RuntimeRoot -Target $TestRoot)) {
  throw "test root escaped runtime root: $TestRoot"
}

$originalLocation = Get-Location
try {
  Set-Location ([System.Environment]::SystemDirectory)
  $fromOtherWorkingDirectory = Get-PalPanelRepositoryRoot
  if (-not [string]::Equals($RepositoryRoot, $fromOtherWorkingDirectory, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "repository discovery depends on the current working directory"
  }
} finally {
  Set-Location $originalLocation
}

$expectedDefault = [System.IO.Path]::GetFullPath((Join-Path $RepositoryRoot "dev-runtime\windows"))
$resolvedDefault = Resolve-PalPanelPath -Path ".\dev-runtime\windows" -BasePath $RepositoryRoot
if (-not [string]::Equals($expectedDefault, $resolvedDefault, [System.StringComparison]::OrdinalIgnoreCase)) {
  throw "repository-relative runtime root resolution is inconsistent"
}

$rejected = @(
  $RepositoryRoot,
  [System.IO.Path]::GetPathRoot($RepositoryRoot),
  (Join-Path $RepositoryRoot "scripts"),
  (Join-Path $RuntimeRoot "..\escape")
)
foreach ($candidate in $rejected) {
  $didReject = $false
  try {
    Assert-PalPanelManagedPath -RepositoryRoot $RepositoryRoot -TargetPath $candidate | Out-Null
  } catch {
    $didReject = $true
  }
  if (-not $didReject) {
    throw "unsafe managed path was accepted: $candidate"
  }
}

$marker = Join-Path $TestRoot ".palpanel-path-test"
New-Item -ItemType Directory -Force -Path $TestRoot | Out-Null
Set-Content -LiteralPath $marker -Value "path contract" -Encoding UTF8
if (-not (Test-Path -LiteralPath $marker -PathType Leaf)) {
  throw "could not write inside the isolated test root"
}
Remove-Item -LiteralPath $marker -Force

$cleanupRoot = Join-Path $TestRoot "managed-cleanup-contract"
New-Item -ItemType Directory -Force -Path (Join-Path $cleanupRoot "nested") | Out-Null
Set-Content -LiteralPath (Join-Path $cleanupRoot "nested\evidence.txt") -Value "cleanup contract" -Encoding UTF8
Remove-PalPanelManagedDirectory -RepositoryRoot $RepositoryRoot -RuntimeRoot $RuntimeRoot -TargetPath $cleanupRoot
if (Test-Path -LiteralPath $cleanupRoot) {
  throw "managed cleanup did not remove its isolated child directory"
}

Write-Host "Windows path isolation contract passed"
Write-Host "Repository root: $RepositoryRoot"
Write-Host "Windows runtime root: $RuntimeRoot"
Write-Host "Test root: $TestRoot"
