param(
  [Parameter(Mandatory = $true)][string]$InstallRoot,
  [switch]$ListBackups,
  [switch]$RestoreLatest,
  [string]$RestoreBackup = "",
  [switch]$Recreate,
  [string]$ConfirmRecovery = "",
  [ValidateRange(1, 300)][int]$ProcessStopTimeoutSeconds = 60
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

Import-Module (Join-Path $PSScriptRoot "windows-maintenance.psm1") -Force
$InstallRoot = [System.IO.Path]::GetFullPath($InstallRoot)
$InstallRoot = Assert-PalPanelReleaseRoot -InstallRoot $InstallRoot -AllowUninstalled
$MaintenanceRoot = Get-PalPanelMaintenanceRoot -InstallRoot $InstallRoot
$BackupRoot = Join-Path $MaintenanceRoot "config-backups"
Assert-PalPanelReleaseManagedPath -InstallRoot $InstallRoot -TargetPath $BackupRoot | Out-Null

function Get-PalPanelConfigBackups {
  if (-not (Test-Path -LiteralPath $BackupRoot -PathType Container)) {
    return @()
  }
  Assert-PalPanelMaintenanceNoReparseTree -Path $BackupRoot
  return @(
    Get-ChildItem -LiteralPath $BackupRoot -Directory -Force |
      Sort-Object LastWriteTimeUtc -Descending |
      ForEach-Object {
        $config = Join-Path $_.FullName "palpanel.env"
        if (Test-Path -LiteralPath $config -PathType Leaf) {
          [pscustomobject]@{
            Name = $_.Name
            Path = $config
            CreatedAt = $_.LastWriteTimeUtc.ToString("o")
            IsRecovery = $_.Name -match '-recovery-'
            Kind = if ($_.Name -match '-recovery-') { "replaced-config" } else { "restorable" }
          }
        }
      }
  )
}

function Save-PalPanelCurrentConfig {
  $current = Join-Path $InstallRoot "config\palpanel.env"
  if (-not (Test-Path -LiteralPath $current -PathType Leaf)) {
    return $null
  }
  Assert-PalPanelReleaseManagedPath -InstallRoot $InstallRoot -TargetPath $current | Out-Null
  $destination = Join-Path $BackupRoot ((Get-Date).ToUniversalTime().ToString("yyyyMMddTHHmmssfffZ") + "-recovery-" + [guid]::NewGuid().ToString("N"))
  Assert-PalPanelReleaseManagedPath -InstallRoot $InstallRoot -TargetPath $destination | Out-Null
  New-Item -ItemType Directory -Force -Path $destination | Out-Null
  Copy-Item -LiteralPath $current -Destination (Join-Path $destination "palpanel.env") -Force
  return $destination
}

function Restore-PalPanelConfig {
  param([Parameter(Mandatory = $true)][string]$Source)

  $target = Join-Path $InstallRoot "config\palpanel.env"
  Assert-PalPanelReleaseManagedPath -InstallRoot $InstallRoot -TargetPath $target | Out-Null
  Assert-PalPanelMaintenanceNoReparseTree -Path $Source
  $snapshot = Save-PalPanelCurrentConfig
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $target) | Out-Null
  Copy-Item -LiteralPath $Source -Destination $target -Force
  Write-Host "Restored configuration from $Source"
  if ($null -ne $snapshot) {
    Write-Host "The replaced configuration was retained at $snapshot"
  }
}

function Recreate-PalPanelConfig {
  $config = Join-Path $InstallRoot "config\palpanel.env"
  $server = Join-Path $InstallRoot "palpanel-server.exe"
  if (-not (Test-Path -LiteralPath $server -PathType Leaf)) {
    throw "palpanel-server.exe is missing; restore a valid package before recreating configuration"
  }
  Assert-PalPanelReleaseManagedPath -InstallRoot $InstallRoot -TargetPath $config | Out-Null
  $snapshot = Save-PalPanelCurrentConfig
  $renamed = ""
  if (Test-Path -LiteralPath $config -PathType Leaf) {
    $renamed = "$config.corrupt-$(Get-Date -Format 'yyyyMMddTHHmmssfff')"
    Assert-PalPanelReleaseManagedPath -InstallRoot $InstallRoot -TargetPath $renamed | Out-Null
    Move-Item -LiteralPath $config -Destination $renamed -Force
  }
  $previousRuntimeRoot = $env:PALPANEL_RUNTIME_ROOT
  try {
    Remove-Item Env:PALPANEL_RUNTIME_ROOT -ErrorAction SilentlyContinue
    & $server "--config" $config "--init-config"
    if ($LASTEXITCODE -ne 0) {
      throw "palpanel-server.exe --init-config failed with exit code $LASTEXITCODE"
    }
    if (-not (Test-Path -LiteralPath $config -PathType Leaf) -or (Get-Item -LiteralPath $config).Length -eq 0) {
      throw "palpanel-server.exe did not create a usable configuration"
    }
  } catch {
    if (Test-Path -LiteralPath $config -PathType Leaf) {
      Remove-PalPanelReleaseManagedPath -InstallRoot $InstallRoot -TargetPath $config
    }
    if (-not [string]::IsNullOrWhiteSpace($renamed) -and (Test-Path -LiteralPath $renamed -PathType Leaf)) {
      Move-Item -LiteralPath $renamed -Destination $config -Force
    }
    throw
  } finally {
    if ($null -eq $previousRuntimeRoot) {
      Remove-Item Env:PALPANEL_RUNTIME_ROOT -ErrorAction SilentlyContinue
    } else {
      $env:PALPANEL_RUNTIME_ROOT = $previousRuntimeRoot
    }
  }
  Write-Host "Created a fresh PalPanel configuration: $config"
  if ($null -ne $snapshot) {
    Write-Host "The previous configuration was retained at $snapshot"
  }
  if (-not [string]::IsNullOrWhiteSpace($renamed)) {
    Write-Host "The unreadable configuration was also retained at $renamed"
  }
}

$actions = @(@($ListBackups, $RestoreLatest, (-not [string]::IsNullOrWhiteSpace($RestoreBackup)), $Recreate) | Where-Object { $_ }).Count
if ($actions -eq 0) {
  $ListBackups = $true
  $actions = 1
}
if ($actions -gt 1) {
  throw "choose only one configuration recovery action"
}

if ($ListBackups) {
  $backups = @(Get-PalPanelConfigBackups)
  if ($backups.Count -eq 0) {
    Write-Host "No PalPanel configuration snapshots are available. Use -Recreate with explicit confirmation to create a fresh configuration."
  } else {
    $backups | Format-Table Name, Kind, CreatedAt, Path -AutoSize
  }
  exit 0
}

Stop-PalPanelManagedProcesses -InstallRoot $InstallRoot -TimeoutSeconds $ProcessStopTimeoutSeconds | Out-Null
if ($RestoreLatest -or -not [string]::IsNullOrWhiteSpace($RestoreBackup)) {
  if ($ConfirmRecovery -cne "RESTORE_PALPANEL_CONFIG") {
    throw "restoring configuration requires -ConfirmRecovery RESTORE_PALPANEL_CONFIG"
  }
  $source = ""
  if ($RestoreLatest) {
    # Replaced-config snapshots may contain the malformed file that prompted
    # recovery. Keep them available for explicit selection, but never promote
    # them to the automatic "latest known restorable" choice.
    $backups = @(Get-PalPanelConfigBackups | Where-Object { -not $_.IsRecovery })
    if ($backups.Count -eq 0) {
      throw "no restorable configuration snapshot is available; replaced-config snapshots require -RestoreBackup"
    }
    $source = $backups[0].Path
  } else {
    $source = Resolve-PalPanelMaintenancePath -Path $RestoreBackup -BasePath $BackupRoot
    if (-not (Test-PalPanelMaintenancePathWithin -Root $BackupRoot -Target $source)) {
      throw "configuration restore source must remain under $BackupRoot"
    }
    if (-not (Test-Path -LiteralPath $source -PathType Leaf)) {
      throw "configuration restore source is missing: $source"
    }
  }
  Restore-PalPanelConfig -Source $source
  exit 0
}

if ($Recreate) {
  if ($ConfirmRecovery -cne "RECREATE_PALPANEL_CONFIG") {
    throw "recreating configuration requires -ConfirmRecovery RECREATE_PALPANEL_CONFIG"
  }
  Recreate-PalPanelConfig
}
