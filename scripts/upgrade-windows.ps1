param(
  [string]$Archive = "",
  [Parameter(Mandatory = $true)][string]$InstallRoot,
  [string]$ExpectedSHA256 = "",
  [switch]$SkipStartupValidation,
  [switch]$KeepRollback,
  [switch]$RecoverOnly,
  [ValidateRange(1, 300)][int]$ProcessStopTimeoutSeconds = 60,
  [ValidateRange(5, 600)][int]$StartupTimeoutSeconds = 90
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

Import-Module (Join-Path $PSScriptRoot "windows-maintenance.psm1") -Force
$InstallRoot = [System.IO.Path]::GetFullPath($InstallRoot)
$InstallRoot = Assert-PalPanelReleaseRoot -InstallRoot $InstallRoot
$MaintenanceRoot = Get-PalPanelMaintenanceRoot -InstallRoot $InstallRoot
$TransactionParent = Join-Path $MaintenanceRoot "tx"
Assert-PalPanelReleaseManagedPath -InstallRoot $InstallRoot -TargetPath $TransactionParent | Out-Null
New-Item -ItemType Directory -Force -Path $TransactionParent | Out-Null

function Set-PalPanelTransactionProperty {
  param(
    [Parameter(Mandatory = $true)]$Transaction,
    [Parameter(Mandatory = $true)][string]$Name,
    $Value
  )

  if ($Transaction -is [System.Collections.IDictionary]) {
    $Transaction[$Name] = $Value
  } else {
    $Transaction | Add-Member -MemberType NoteProperty -Name $Name -Value $Value -Force
  }
}

function Get-RelativeInstallPath {
  param([Parameter(Mandatory = $true)][string]$Path)

  $full = [System.IO.Path]::GetFullPath($Path)
  if (-not (Test-PalPanelMaintenancePathWithin -Root $InstallRoot -Target $full)) {
    throw "path escapes installation root: $full"
  }
  return $full.Substring($InstallRoot.TrimEnd('\', '/').Length).TrimStart('\', '/')
}

function Copy-PalPanelUpgradeItem {
  param(
    [Parameter(Mandatory = $true)][string]$Source,
    [Parameter(Mandatory = $true)][string]$Destination
  )

  Assert-PalPanelMaintenanceNoReparseTree -Path $Source
  $parent = Split-Path -Parent $Destination
  if ($parent) {
    New-Item -ItemType Directory -Force -Path $parent | Out-Null
  }
  Copy-Item -LiteralPath $Source -Destination $Destination -Recurse -Force
}

function Get-PalPanelConfiguredDatabasePaths {
  $paths = [System.Collections.Generic.List[string]]::new()
  $external = [System.Collections.Generic.List[string]]::new()
  foreach ($candidate in @(
    (Join-Path $InstallRoot "data\palpanel.db"),
    (Join-Path $InstallRoot "data\database\palpanel.db")
  )) {
    $paths.Add([System.IO.Path]::GetFullPath($candidate))
  }

  $config = Join-Path $InstallRoot "config\palpanel.env"
  if (Test-Path -LiteralPath $config -PathType Leaf) {
    foreach ($line in Get-Content -LiteralPath $config) {
      if ($line -match '^\s*PALPANEL_DB_PATH\s*=\s*(.*)$') {
        $rawValue = $Matches[1]
        try {
          $value = ConvertFrom-PalPanelEnvironmentValue -RawValue $rawValue
        } catch {
          $external.Add($rawValue.Trim())
          continue
        }
        if (-not [string]::IsNullOrWhiteSpace($value)) {
          try {
            $candidate = Resolve-PalPanelMaintenancePath -Path $value -BasePath $InstallRoot
            if (Test-PalPanelMaintenancePathWithin -Root (Join-Path $InstallRoot "data") -Target $candidate) {
              $paths.Add($candidate)
            } else {
              $external.Add($candidate)
            }
          } catch {
            $external.Add($value)
          }
        }
      }
    }
  }
  $unique = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)
  $result = [System.Collections.Generic.List[string]]::new()
  foreach ($path in $paths) {
    if ($unique.Add($path)) {
      $result.Add($path)
    }
  }
  return [pscustomobject]@{ Paths = @($result); External = @($external) }
}

function Assert-PalPanelStartupPathsManaged {
  $config = Join-Path $InstallRoot "config\palpanel.env"
  if (-not (Test-Path -LiteralPath $config -PathType Leaf)) {
    return
  }
  $managedKeys = @(
    "PALPANEL_DATA_DIR",
    "PALPANEL_SERVER_DIR",
    "PALPANEL_WINE_PREFIX_DIR",
    "PALPANEL_TOOLS_DIR",
    "PALPANEL_STEAMCMD_DIR",
    "PALPANEL_UE4SS_DIR",
    "PALPANEL_UPLOADS_DIR",
    "PALPANEL_BACKUPS_DIR",
    "PALPANEL_LOGS_DIR",
    "PALPANEL_DB_PATH",
    "PALPANEL_SAVE_INDEX_CACHE_DIR"
  )
  foreach ($line in Get-Content -LiteralPath $config) {
    if ($line -notmatch '^\s*([A-Z][A-Z0-9_]*)\s*=\s*(.*)$') {
      continue
    }
    $name = $Matches[1]
    try {
      $value = ConvertFrom-PalPanelEnvironmentValue -RawValue $Matches[2]
    } catch {
      throw "configured path $name cannot be parsed: $($_.Exception.Message)"
    }
    if ($name -eq "PALPANEL_RUNTIME_ROOT" -and -not [string]::IsNullOrWhiteSpace($value)) {
      throw "startup validation will not use a PALPANEL_RUNTIME_ROOT from the release config; use -SkipStartupValidation for this custom layout"
    }
    if ($name -notin $managedKeys -or [string]::IsNullOrWhiteSpace($value)) {
      continue
    }
    try {
      $resolved = Resolve-PalPanelMaintenancePath -Path $value -BasePath $InstallRoot
    } catch {
      throw "configured path $name cannot be safely resolved; use -SkipStartupValidation and validate the custom layout manually"
    }
    if (-not (Test-PalPanelMaintenancePathWithin -Root (Join-Path $InstallRoot "data") -Target $resolved)) {
      throw "configured path $name is outside the managed release data directory; use -SkipStartupValidation and migrate that custom path manually"
    }
  }
}

function Save-PalPanelDatabaseSnapshot {
  param(
    [Parameter(Mandatory = $true)]$Transaction,
    [Parameter(Mandatory = $true)][string]$TransactionRoot
  )

  $database = Get-PalPanelConfiguredDatabasePaths
  if ($database.External.Count -gt 0 -and -not $SkipStartupValidation) {
    throw "configured database path is outside the managed data directory; use -SkipStartupValidation or move it under $InstallRoot\data"
  }
  $entries = [System.Collections.Generic.List[object]]::new()
  foreach ($databasePath in $database.Paths) {
    foreach ($path in @($databasePath, "$databasePath-wal", "$databasePath-shm")) {
      $relative = Get-RelativeInstallPath -Path $path
      $entry = [ordered]@{ relative_path = $relative; existed = (Test-Path -LiteralPath $path -PathType Leaf) }
      if ($entry.existed) {
        Assert-PalPanelReleaseManagedPath -InstallRoot $InstallRoot -TargetPath $path | Out-Null
        $backup = Join-Path $TransactionRoot (Join-Path "database" $relative)
        Copy-PalPanelUpgradeItem -Source $path -Destination $backup
      }
      $entries.Add([pscustomobject]$entry)
    }
  }
  $Transaction.database_entries = @($entries)
  $Transaction.external_database_paths = @($database.External)
}

function Save-PalPanelConfigSnapshot {
  param(
    [Parameter(Mandatory = $true)]$Transaction,
    [Parameter(Mandatory = $true)][string]$TransactionRoot
  )

  $config = Join-Path $InstallRoot "config\palpanel.env"
  $Transaction.config_existed = Test-Path -LiteralPath $config -PathType Leaf
  if ($Transaction.config_existed) {
    Assert-PalPanelReleaseManagedPath -InstallRoot $InstallRoot -TargetPath $config | Out-Null
    Copy-PalPanelUpgradeItem -Source $config -Destination (Join-Path $TransactionRoot "config\palpanel.env")
  }
}

function Save-PalPanelPayloadSnapshot {
  param(
    [Parameter(Mandatory = $true)]$Transaction,
    [Parameter(Mandatory = $true)][string]$TransactionRoot
  )

  $entries = [System.Collections.Generic.List[object]]::new()
  foreach ($item in Get-PalPanelPayloadItemRoots) {
    $target = Join-Path $InstallRoot $item.RelativePath
    $exists = Test-Path -LiteralPath $target
    $entry = [ordered]@{
      relative_path = $item.RelativePath
      is_directory = [bool]$item.IsDirectory
      existed = [bool]$exists
    }
    if ($exists) {
      Assert-PalPanelReleaseManagedPath -InstallRoot $InstallRoot -TargetPath $target | Out-Null
      Assert-PalPanelMaintenanceNoReparseTree -Path $target
      Copy-PalPanelUpgradeItem -Source $target -Destination (Join-Path $TransactionRoot (Join-Path "payload" $item.RelativePath))
    }
    $entries.Add([pscustomobject]$entry)
  }
  $Transaction.payload_items = @($entries)
}

function Restore-PalPanelUpgradeTransaction {
  param(
    [Parameter(Mandatory = $true)][string]$TransactionRoot,
    [Parameter(Mandatory = $true)]$Transaction,
    [string]$Reason = "recovery"
  )

  if ($null -eq $Transaction.payload_items -or @($Transaction.payload_items).Count -eq 0) {
    throw "transaction cannot be rolled back because its payload snapshot is missing: $TransactionRoot"
  }
  $configTarget = Join-Path $InstallRoot "config\palpanel.env"
  $configBackup = Join-Path $TransactionRoot "config\palpanel.env"
  if ([bool]$Transaction.config_existed) {
    if (-not (Test-Path -LiteralPath $configBackup -PathType Leaf)) {
      throw "transaction configuration backup is missing: $configBackup"
    }
    Assert-PalPanelMaintenanceNoReparseTree -Path $configBackup
  }
  foreach ($entry in @($Transaction.payload_items)) {
    $target = Join-Path $InstallRoot ([string]$entry.relative_path)
    if (Test-Path -LiteralPath $target) {
      Remove-PalPanelReleaseManagedPath -InstallRoot $InstallRoot -TargetPath $target
    }
    if ([bool]$entry.existed) {
      $backup = Join-Path $TransactionRoot (Join-Path "payload" ([string]$entry.relative_path))
      if (-not (Test-Path -LiteralPath $backup)) {
        throw "transaction payload backup is missing: $backup"
      }
      Copy-PalPanelUpgradeItem -Source $backup -Destination $target
    }
  }
  if (Test-Path -LiteralPath $configTarget) {
    Remove-PalPanelReleaseManagedPath -InstallRoot $InstallRoot -TargetPath $configTarget
  }
  if ([bool]$Transaction.config_existed) {
    Copy-PalPanelUpgradeItem -Source $configBackup -Destination $configTarget
  }
  foreach ($entry in @($Transaction.database_entries)) {
    $target = Join-Path $InstallRoot ([string]$entry.relative_path)
    if (Test-Path -LiteralPath $target) {
      Remove-PalPanelReleaseManagedPath -InstallRoot $InstallRoot -TargetPath $target
    }
    if ([bool]$entry.existed) {
      $backup = Join-Path $TransactionRoot (Join-Path "database" ([string]$entry.relative_path))
      if (-not (Test-Path -LiteralPath $backup -PathType Leaf)) {
        throw "transaction database backup is missing: $backup"
      }
      Copy-PalPanelUpgradeItem -Source $backup -Destination $target
    }
  }
  Set-PalPanelTransactionProperty -Transaction $Transaction -Name "status" -Value "rolled_back"
  Set-PalPanelTransactionProperty -Transaction $Transaction -Name "rollback_reason" -Value $Reason
  Set-PalPanelTransactionProperty -Transaction $Transaction -Name "finished_at" -Value (Get-Date).ToUniversalTime().ToString("o")
  Write-PalPanelMaintenanceJson -Value $Transaction -Path (Join-Path $TransactionRoot "state.json")
}

function Recover-PalPanelIncompleteTransactions {
  $recovered = [System.Collections.Generic.List[string]]::new()
  foreach ($directory in @(Get-ChildItem -LiteralPath $TransactionParent -Directory -Force)) {
    $statePath = Join-Path $directory.FullName "state.json"
    if (-not (Test-Path -LiteralPath $statePath -PathType Leaf)) {
      continue
    }
    $transaction = Read-PalPanelMaintenanceJson -Path $statePath
    if ([string]$transaction.install_root -ne $InstallRoot) {
      throw "transaction belongs to a different installation root: $($directory.FullName)"
    }
    if ([string]$transaction.status -in @("applying", "validating", "rollback_failed")) {
      Restore-PalPanelUpgradeTransaction -TransactionRoot $directory.FullName -Transaction $transaction -Reason "automatic recovery of incomplete upgrade"
      $recovered.Add($directory.Name)
    }
  }
  return @($recovered)
}

function Test-PalPanelUpgradeFreeSpace {
  param([Parameter(Mandatory = $true)][string]$ArchivePath)

  try {
    $drive = [System.IO.DriveInfo]::new([System.IO.Path]::GetPathRoot($InstallRoot))
    $required = (Get-Item -LiteralPath $ArchivePath).Length * 3 + 128MB
    foreach ($item in Get-PalPanelPayloadItemRoots) {
      $path = Join-Path $InstallRoot $item.RelativePath
      if (Test-Path -LiteralPath $path) {
        $required += (@(Get-ChildItem -LiteralPath $path -File -Recurse -ErrorAction Stop | Measure-Object -Property Length -Sum).Sum)
      }
    }
    if ($drive.AvailableFreeSpace -lt $required) {
      throw "insufficient free disk space for a transactional upgrade: need at least $required bytes, available $($drive.AvailableFreeSpace) bytes"
    }
  } catch [System.Management.Automation.RuntimeException] {
    throw
  } catch {
    Write-Warning "could not determine free disk space; continuing with transactional staging: $($_.Exception.Message)"
  }
}

function Invoke-PalPanelStartupValidation {
  param([Parameter(Mandatory = $true)][string]$TransactionRoot)

  $stdout = Join-Path $TransactionRoot "startup.stdout.log"
  $stderr = Join-Path $TransactionRoot "startup.stderr.log"
  $launcher = Join-Path $InstallRoot "PalPanel.exe"
  $pathEnvironmentNames = @(
    "PALPANEL_RUNTIME_ROOT",
    "PALPANEL_DATA_DIR",
    "PALPANEL_SERVER_DIR",
    "PALPANEL_WINE_PREFIX_DIR",
    "PALPANEL_TOOLS_DIR",
    "PALPANEL_STEAMCMD_DIR",
    "PALPANEL_UE4SS_DIR",
    "PALPANEL_UPLOADS_DIR",
    "PALPANEL_BACKUPS_DIR",
    "PALPANEL_LOGS_DIR",
    "PALPANEL_DB_PATH",
    "PALPANEL_SAVE_INDEX_CACHE_DIR"
  )
  $previousEnvironment = @{}
  $process = $null
  try {
    foreach ($name in $pathEnvironmentNames) {
      $previousEnvironment[$name] = [System.Environment]::GetEnvironmentVariable($name, [System.EnvironmentVariableTarget]::Process)
      [System.Environment]::SetEnvironmentVariable($name, $null, [System.EnvironmentVariableTarget]::Process)
    }
    $process = Start-Process -FilePath $launcher -ArgumentList @("--no-browser", "--no-prompt", "--exit-after-health") -WorkingDirectory $InstallRoot -PassThru -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    if (-not $process.WaitForExit($StartupTimeoutSeconds * 1000)) {
      Stop-PalPanelManagedProcesses -InstallRoot $InstallRoot -TimeoutSeconds $ProcessStopTimeoutSeconds | Out-Null
      throw "upgraded Launcher did not complete startup validation within $StartupTimeoutSeconds seconds; logs retained in $TransactionRoot"
    }
    # WaitForExit(timeout) can report completion before redirected output has
    # drained and before Windows PowerShell refreshes ExitCode. Finish the
    # wait and refresh explicitly so a successful launcher is not rolled back
    # with an empty exit code.
    $process.WaitForExit()
    $process.Refresh()
    $exitCode = $process.ExitCode
    if ($exitCode -ne 0) {
      throw "upgraded Launcher failed startup validation with exit code $exitCode; logs retained in $TransactionRoot"
    }
  } finally {
    if ($null -ne $process) {
      $process.Dispose()
    }
    foreach ($name in $pathEnvironmentNames) {
      [System.Environment]::SetEnvironmentVariable($name, $previousEnvironment[$name], [System.EnvironmentVariableTarget]::Process)
    }
  }
}

function Preserve-PalPanelConfigRecoverySnapshot {
  param([Parameter(Mandatory = $true)][string]$TransactionRoot)

  $source = Join-Path $TransactionRoot "config\palpanel.env"
  if (-not (Test-Path -LiteralPath $source -PathType Leaf)) {
    return
  }
  $destination = Join-Path $MaintenanceRoot (Join-Path "config-backups" ((Get-Date).ToUniversalTime().ToString("yyyyMMddTHHmmssZ") + "-" + [guid]::NewGuid().ToString("N").Substring(0, 8)))
  Assert-PalPanelReleaseManagedPath -InstallRoot $InstallRoot -TargetPath $destination | Out-Null
  Copy-PalPanelUpgradeItem -Source $source -Destination (Join-Path $destination "palpanel.env")
}

if ($RecoverOnly) {
  Stop-PalPanelManagedProcesses -InstallRoot $InstallRoot -TimeoutSeconds $ProcessStopTimeoutSeconds | Out-Null
  $recovered = @(Recover-PalPanelIncompleteTransactions)
  if ($recovered.Count -gt 0) {
    Write-Host "Recovered incomplete upgrade transaction(s): $($recovered -join ', ')"
  }
  Write-Host "PalPanel upgrade recovery completed for $InstallRoot"
  exit 0
}
if ([string]::IsNullOrWhiteSpace($Archive)) {
  throw "-Archive is required unless -RecoverOnly is specified"
}
if (-not $SkipStartupValidation) {
  Assert-PalPanelStartupPathsManaged
}

$Archive = [System.IO.Path]::GetFullPath($Archive)
Test-PalPanelUpgradeFreeSpace -ArchivePath $Archive
Write-PalPanelMaintenanceIdentity -InstallRoot $InstallRoot
$TransactionID = "u-$PID-$([guid]::NewGuid().ToString('N').Substring(0, 8))"
$TransactionRoot = Join-Path $TransactionParent $TransactionID
Assert-PalPanelReleaseManagedPath -InstallRoot $InstallRoot -TargetPath $TransactionRoot | Out-Null
New-Item -ItemType Directory -Force -Path $TransactionRoot | Out-Null
$Stage = Join-Path $TransactionRoot "stage"
$Transaction = [ordered]@{
  schema_version = 1
  id = $TransactionID
  install_root = $InstallRoot
  archive = $Archive
  status = "prepared"
  created_at = (Get-Date).ToUniversalTime().ToString("o")
  payload_items = @()
  database_entries = @()
  config_existed = $false
  external_database_paths = @()
  failure = ""
  rollback_reason = ""
  finished_at = ""
}
Write-PalPanelMaintenanceJson -Value $Transaction -Path (Join-Path $TransactionRoot "state.json")

$succeeded = $false
$payloadApplied = $false
try {
  $PackageRoot = Expand-PalPanelUpgradeArchive -Archive $Archive -Destination $Stage -ExpectedSHA256 $ExpectedSHA256
  # Candidate validation is deliberately complete before stopping a live
  # installation. From this point onward failures may require downtime.
  Stop-PalPanelManagedProcesses -InstallRoot $InstallRoot -TimeoutSeconds $ProcessStopTimeoutSeconds | Out-Null
  $recovered = @(Recover-PalPanelIncompleteTransactions)
  if ($recovered.Count -gt 0) {
    Write-Host "Recovered incomplete upgrade transaction(s): $($recovered -join ', ')"
  }
  Save-PalPanelPayloadSnapshot -Transaction $Transaction -TransactionRoot $TransactionRoot
  Save-PalPanelConfigSnapshot -Transaction $Transaction -TransactionRoot $TransactionRoot
  Save-PalPanelDatabaseSnapshot -Transaction $Transaction -TransactionRoot $TransactionRoot
  $Transaction.status = "ready"
  Write-PalPanelMaintenanceJson -Value $Transaction -Path (Join-Path $TransactionRoot "state.json")

  $Transaction.status = "applying"
  Write-PalPanelMaintenanceJson -Value $Transaction -Path (Join-Path $TransactionRoot "state.json")
  $payloadApplied = $true
  foreach ($item in Get-PalPanelPayloadItemRoots) {
    $source = Join-Path $PackageRoot $item.RelativePath
    if (-not (Test-Path -LiteralPath $source)) {
      throw "staged package payload is missing: $($item.RelativePath)"
    }
    $target = Join-Path $InstallRoot $item.RelativePath
    if (Test-Path -LiteralPath $target) {
      Remove-PalPanelReleaseManagedPath -InstallRoot $InstallRoot -TargetPath $target
    }
    Copy-PalPanelUpgradeItem -Source $source -Destination $target
  }
  Test-PalPanelInstalledPayload -InstallRoot $InstallRoot

  if (-not $SkipStartupValidation) {
    $Transaction.status = "validating"
    Write-PalPanelMaintenanceJson -Value $Transaction -Path (Join-Path $TransactionRoot "state.json")
    Invoke-PalPanelStartupValidation -TransactionRoot $TransactionRoot
    Stop-PalPanelManagedProcesses -InstallRoot $InstallRoot -TimeoutSeconds $ProcessStopTimeoutSeconds | Out-Null
  }

  Preserve-PalPanelConfigRecoverySnapshot -TransactionRoot $TransactionRoot
  $Transaction.status = "completed"
  $Transaction.finished_at = (Get-Date).ToUniversalTime().ToString("o")
  Write-PalPanelMaintenanceJson -Value $Transaction -Path (Join-Path $TransactionRoot "state.json")
  $succeeded = $true
  Write-Host "PalPanel upgrade completed. Configuration, database, game files, saves, Mods, UE4SS, and PalDefender were not overlaid."
  Write-Host "Installation root: $InstallRoot"
} catch {
  $failure = $_
  $requiresRollback = $payloadApplied
  if ($requiresRollback -and @($Transaction.payload_items).Count -gt 0) {
    try {
      Stop-PalPanelManagedProcesses -InstallRoot $InstallRoot -TimeoutSeconds $ProcessStopTimeoutSeconds | Out-Null
      $Transaction.status = "rollback_failed"
      Set-PalPanelTransactionProperty -Transaction $Transaction -Name "failure" -Value $failure.Exception.Message
      Write-PalPanelMaintenanceJson -Value $Transaction -Path (Join-Path $TransactionRoot "state.json")
      Restore-PalPanelUpgradeTransaction -TransactionRoot $TransactionRoot -Transaction $Transaction -Reason "upgrade failure: $($failure.Exception.Message)"
    } catch {
      throw "upgrade failed: $($failure.Exception.Message). Automatic rollback also failed: $($_.Exception.Message). Retained transaction: $TransactionRoot"
    }
    throw "upgrade failed and was rolled back: $($failure.Exception.Message). Retained transaction: $TransactionRoot"
  }
  $Transaction.status = "failed_before_apply"
  Set-PalPanelTransactionProperty -Transaction $Transaction -Name "failure" -Value $failure.Exception.Message
  Set-PalPanelTransactionProperty -Transaction $Transaction -Name "finished_at" -Value (Get-Date).ToUniversalTime().ToString("o")
  Write-PalPanelMaintenanceJson -Value $Transaction -Path (Join-Path $TransactionRoot "state.json")
  throw "upgrade failed before the installed payload was changed: $($failure.Exception.Message). Retained transaction: $TransactionRoot"
} finally {
  if ($succeeded -and -not $KeepRollback) {
    Remove-PalPanelReleaseManagedPath -InstallRoot $InstallRoot -TargetPath $TransactionRoot
  }
}
