param(
  [Parameter(Mandatory = $true)][string]$InstallRoot,
  [switch]$PurgeData,
  [string]$ConfirmPurge = "",
  [ValidateRange(1, 300)][int]$ProcessStopTimeoutSeconds = 60
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

Import-Module (Join-Path $PSScriptRoot "windows-maintenance.psm1") -Force
$InstallRoot = [System.IO.Path]::GetFullPath($InstallRoot)
$InstallRoot = Assert-PalPanelReleaseRoot -InstallRoot $InstallRoot -AllowUninstalled

if ($PurgeData -and $ConfirmPurge -cne "PURGE_PALPANEL_MANAGED_DATA") {
  throw "complete cleanup requires -PurgeData -ConfirmPurge PURGE_PALPANEL_MANAGED_DATA"
}

Write-PalPanelMaintenanceIdentity -InstallRoot $InstallRoot
$payloadTargets = @(
  Get-PalPanelPayloadItemRoots | ForEach-Object { Join-Path $InstallRoot $_.RelativePath }
)
$purgeTargets = @()
if ($PurgeData) {
  $purgeTargets = @("config", "data", ".palpanel-maintenance") | ForEach-Object { Join-Path $InstallRoot $_ }
}
foreach ($target in @($payloadTargets) + @($purgeTargets)) {
  if (Test-Path -LiteralPath $target) {
    Assert-PalPanelReleaseManagedPath -InstallRoot $InstallRoot -TargetPath $target | Out-Null
    Assert-PalPanelMaintenanceNoReparseTree -Path $target
  }
}

$stopped = @(Stop-PalPanelManagedProcesses -InstallRoot $InstallRoot -TimeoutSeconds $ProcessStopTimeoutSeconds)
if (@(Get-PalPanelManagedProcesses -InstallRoot $InstallRoot).Count -gt 0) {
  throw "refusing to uninstall while a managed PalPanel process is still running"
}

foreach ($target in $payloadTargets) {
  if (Test-Path -LiteralPath $target) {
    Remove-PalPanelReleaseManagedPath -InstallRoot $InstallRoot -TargetPath $target
  }
}

$maintenance = Get-PalPanelMaintenanceRoot -InstallRoot $InstallRoot
if ($PurgeData) {
  foreach ($target in $purgeTargets) {
    if (Test-Path -LiteralPath $target) {
      Remove-PalPanelReleaseManagedPath -InstallRoot $InstallRoot -TargetPath $target
    }
  }
  Write-Host "PalPanel program files and managed runtime data were removed. The installation root itself was retained: $InstallRoot"
} else {
  $record = [ordered]@{
    schema_version = 1
    uninstalled_at = (Get-Date).ToUniversalTime().ToString("o")
    install_root = $InstallRoot
    stopped_process_ids = @($stopped)
    preserved_paths = @(
      (Join-Path $InstallRoot "config"),
      (Join-Path $InstallRoot "data"),
      $maintenance
    )
  }
  Write-PalPanelMaintenanceJson -Value $record -Path (Join-Path $maintenance "uninstall.json")
  Write-Host "PalPanel program files were removed. Configuration, database, game files, saves, Mods, UE4SS, PalDefender, backups, and maintenance recovery snapshots were preserved."
  Write-Host "To delete only PalPanel-managed runtime data as well, rerun from a retained copy of this script with -PurgeData -ConfirmPurge PURGE_PALPANEL_MANAGED_DATA."
}
