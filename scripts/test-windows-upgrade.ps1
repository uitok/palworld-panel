param(
  [string]$RuntimeRoot = "",
  [string]$TestRoot = "",
  [switch]$KeepArtifacts
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$WindowsHost = $env:OS -eq "Windows_NT"
if (Get-Variable -Name IsWindows -ErrorAction SilentlyContinue) {
  $WindowsHost = [bool]$IsWindows
}
if (-not $WindowsHost) {
  throw "scripts/test-windows-upgrade.ps1 must run on Windows"
}

$ScriptDir = $PSScriptRoot
Import-Module (Join-Path $ScriptDir "windows-e2e-common.psm1") -Force
$RepositoryRoot = Get-PalPanelRepositoryRoot
if ([string]::IsNullOrWhiteSpace($RuntimeRoot)) {
  $RuntimeRoot = Join-Path $RepositoryRoot "dev-runtime\windows"
} else {
  $RuntimeRoot = Resolve-PalPanelPath -Path $RuntimeRoot -BasePath $RepositoryRoot
}
$RuntimeRoot = Initialize-PalPanelWindowsLayout -RepositoryRoot $RepositoryRoot -RuntimeRoot $RuntimeRoot
if ([string]::IsNullOrWhiteSpace($TestRoot)) {
  $TestRoot = Join-Path $RuntimeRoot (Join-Path "e2e" ("up-" + [guid]::NewGuid().ToString("N").Substring(0, 8)))
} else {
  $TestRoot = Resolve-PalPanelPath -Path $TestRoot -BasePath $RuntimeRoot
}
$TestRoot = Assert-PalPanelManagedPath -RepositoryRoot $RepositoryRoot -TargetPath $TestRoot
if (-not (Test-PalPanelPathWithin -Root $RuntimeRoot -Target $TestRoot)) {
  throw "test root must remain under the managed Windows runtime root: $TestRoot"
}
if (Test-Path -LiteralPath $TestRoot) {
  throw "upgrade contract test root already exists: $TestRoot"
}

function Write-FakePE {
  param([Parameter(Mandatory = $true)][string]$Path, [Parameter(Mandatory = $true)][string]$Marker)

  $bytes = New-Object byte[] 128
  $bytes[0] = 0x4d
  $bytes[1] = 0x5a
  [System.BitConverter]::GetBytes([int]0x40).CopyTo($bytes, 0x3c)
  $bytes[0x40] = 0x50
  $bytes[0x41] = 0x45
  $bytes[0x42] = 0
  $bytes[0x43] = 0
  [System.Text.Encoding]::ASCII.GetBytes($Marker).CopyTo($bytes, 80)
  $parent = Split-Path -Parent $Path
  New-Item -ItemType Directory -Force -Path $parent | Out-Null
  [System.IO.File]::WriteAllBytes($Path, $bytes)
}

function Write-TextFile {
  param([Parameter(Mandatory = $true)][string]$Path, [Parameter(Mandatory = $true)][string]$Text)

  $parent = Split-Path -Parent $Path
  New-Item -ItemType Directory -Force -Path $parent | Out-Null
  [System.IO.File]::WriteAllText($Path, $Text, [System.Text.UTF8Encoding]::new($false))
}

function New-FakeWindowsPackage {
  param(
    [Parameter(Mandatory = $true)][string]$Parent,
    [Parameter(Mandatory = $true)][string]$Version,
    [Parameter(Mandatory = $true)][string]$Marker,
    [string]$LauncherPath = ""
  )

  $packageName = "palpanel_${Version}_windows_amd64"
  $package = Join-Path $Parent $packageName
  New-Item -ItemType Directory -Force -Path $package | Out-Null
  foreach ($executable in @("PalPanel.exe", "palpanel-server.exe", "sav-cli.exe", "palcalc-bridge.exe")) {
    Write-FakePE -Path (Join-Path $package $executable) -Marker "$Marker-$executable"
  }
  Write-TextFile -Path (Join-Path $package "README.md") -Text "fixture package $Marker"
  Write-TextFile -Path (Join-Path $package "LICENSE") -Text "fixture license $Marker"
  Write-TextFile -Path (Join-Path $package "THIRD_PARTY_LICENSES.txt") -Text "fixture third-party license $Marker"
  Write-TextFile -Path (Join-Path $package "config\palpanel.env.example") -Text "PALPANEL_LISTEN_ADDR=127.0.0.1:8080`n"
  Write-TextFile -Path (Join-Path $package "backend\deployments\wine-runner\fixture.txt") -Text "backend $Marker"
  Write-TextFile -Path (Join-Path $package "licenses\sav-cli-LICENSE.txt") -Text "sav-cli $Marker"
  Write-TextFile -Path (Join-Path $package "licenses\pallocalize-Apache-2.0.txt") -Text "pallocalize $Marker"
  Write-TextFile -Path (Join-Path $package "licenses\PalDefender-MIT.txt") -Text "paldefender $Marker"
  Write-TextFile -Path (Join-Path $package "licenses\PalCalc-MIT.txt") -Text "palcalc $Marker"
  New-Item -ItemType Directory -Force -Path (Join-Path $package "maintenance") | Out-Null
  foreach ($scriptName in @(
    "windows-maintenance.psm1",
    "upgrade-windows.ps1",
    "uninstall-windows.ps1",
    "recover-windows-config.ps1"
  )) {
    Copy-Item -LiteralPath (Join-Path $ScriptDir $scriptName) -Destination (Join-Path $package (Join-Path "maintenance" $scriptName)) -Force
  }
  if (-not [string]::IsNullOrWhiteSpace($LauncherPath)) {
    Copy-Item -LiteralPath $LauncherPath -Destination (Join-Path $package "PalPanel.exe") -Force
  }
  $checksumLines = Get-ChildItem -LiteralPath $package -File -Recurse |
    Sort-Object FullName |
    ForEach-Object {
      $relative = $_.FullName.Substring($package.Length).TrimStart('\') -replace "\\", "/"
      $hash = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
      "$hash  ./$relative"
    }
  [System.IO.File]::WriteAllLines((Join-Path $package "checksums.txt"), [string[]]$checksumLines, [System.Text.UTF8Encoding]::new($false))
  $archive = Join-Path $Parent "$packageName.zip"
  Compress-Archive -Path $package -DestinationPath $archive -Force
  return [pscustomobject]@{ Package = $package; Archive = $archive }
}

function New-MutatingLauncher {
  param([Parameter(Mandatory = $true)][string]$Path)

  $parent = Split-Path -Parent $Path
  New-Item -ItemType Directory -Force -Path $parent | Out-Null
  $source = @'
using System;
using System.IO;

public static class PalPanelMutationFixture
{
    public static int Main(string[] args)
    {
        string root = AppDomain.CurrentDomain.BaseDirectory;
        string config = Path.Combine(root, "config", "palpanel.env");
        string marker = Path.Combine(root, "data", "candidate-launcher-ran.txt");
        Directory.CreateDirectory(Path.GetDirectoryName(config));
        Directory.CreateDirectory(Path.GetDirectoryName(marker));
        File.WriteAllText(config, "candidate-mutated-config");
        File.WriteAllText(marker, "candidate-launcher-ran");
        return 42;
    }
}
'@
  $frameworkRoots = @(
    (Join-Path $env:WINDIR "Microsoft.NET\Framework64\v4.0.30319\csc.exe"),
    (Join-Path $env:WINDIR "Microsoft.NET\Framework\v4.0.30319\csc.exe")
  )
  $compiler = $frameworkRoots | Where-Object { Test-Path -LiteralPath $_ -PathType Leaf } | Select-Object -First 1
  if (-not $compiler) {
    throw "C# compiler is unavailable"
  }

  $sourcePath = [System.IO.Path]::ChangeExtension($Path, ".cs")
  [System.IO.File]::WriteAllText($sourcePath, $source, [System.Text.UTF8Encoding]::new($false))
  try {
    & $compiler /nologo /target:exe "/out:$Path" $sourcePath
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $Path -PathType Leaf)) {
      throw "failed to compile mutating launcher"
    }
  } finally {
    Remove-Item -LiteralPath $sourcePath -Force -ErrorAction SilentlyContinue
  }
}

function Assert-FileHashEqual {
  param([Parameter(Mandatory = $true)][string]$Path, [Parameter(Mandatory = $true)][string]$Expected)

  $actual = (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash
  if ($actual -ne $Expected) {
    throw "file hash changed unexpectedly: $Path"
  }
}

$Succeeded = $false
$InsideHelper = $null
$OutsideHelper = $null
try {
  New-Item -ItemType Directory -Force -Path $TestRoot | Out-Null
  $fixtureRoot = Join-Path $TestRoot "fixtures"
  $candidateA = New-FakeWindowsPackage -Parent (Join-Path $fixtureRoot "a") -Version "v0.0.1" -Marker "A"
  $candidateB = New-FakeWindowsPackage -Parent (Join-Path $fixtureRoot "b") -Version "v0.0.2" -Marker "B"
  $candidateC = New-FakeWindowsPackage -Parent (Join-Path $fixtureRoot "c") -Version "v0.0.3" -Marker "C"
  $mutatingLauncher = Join-Path $fixtureRoot "helpers\mutating-launcher.exe"
  New-MutatingLauncher -Path $mutatingLauncher
  $candidateD = New-FakeWindowsPackage -Parent (Join-Path $fixtureRoot "d") -Version "v0.0.4" -Marker "D" -LauncherPath $mutatingLauncher
  $install = Join-Path $TestRoot "release-install"
  New-Item -ItemType Directory -Force -Path $install | Out-Null
  foreach ($entry in Get-ChildItem -LiteralPath $candidateA.Package -Force) {
    Copy-Item -LiteralPath $entry.FullName -Destination $install -Recurse -Force
  }

  $PowerShellExecutable = (Get-Command powershell.exe -ErrorAction SilentlyContinue).Source
  if ([string]::IsNullOrWhiteSpace($PowerShellExecutable) -or -not (Test-Path -LiteralPath $PowerShellExecutable -PathType Leaf)) {
    throw "Windows PowerShell executable is missing"
  }
  Copy-Item -LiteralPath $PowerShellExecutable -Destination (Join-Path $install "palpanel-server.exe") -Force
  $outsideExecutable = Join-Path $TestRoot "outside\palpanel-server.exe"
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $outsideExecutable) | Out-Null
  Copy-Item -LiteralPath $PowerShellExecutable -Destination $outsideExecutable -Force
  $helperArguments = @("-NoProfile", "-Command", "Start-Sleep -Seconds 120")
  $InsideHelper = Start-Process -FilePath (Join-Path $install "palpanel-server.exe") -ArgumentList $helperArguments -WorkingDirectory $install -WindowStyle Hidden -PassThru
  $OutsideHelper = Start-Process -FilePath $outsideExecutable -ArgumentList $helperArguments -WorkingDirectory (Split-Path -Parent $outsideExecutable) -WindowStyle Hidden -PassThru
  Start-Sleep -Seconds 1
  if ($InsideHelper.HasExited -or $OutsideHelper.HasExited) {
    throw "process ownership fixtures exited before the upgrade"
  }

  $config = Join-Path $install "config\palpanel.env"
  Write-TextFile -Path $config -Text "PALPANEL_LISTEN_ADDR=127.0.0.1:18080`nPALPANEL_DB_PATH=data\palpanel.db`n"
  $database = Join-Path $install "data\palpanel.db"
  $save = Join-Path $install "data\server\Pal\Saved\SaveGames\world.sav"
  $mod = Join-Path $install "data\server\Pal\Content\Paks\Mods\user-mod.pak"
  $ue4ss = Join-Path $install "data\server\Pal\Binaries\Win64\Mods\mods.txt"
  $palDefender = Join-Path $install "data\server\Pal\Binaries\Win64\PalDefender\PalDefender.dll"
  Write-TextFile -Path $database -Text "database-before-upgrade"
  Write-TextFile -Path $save -Text "save-before-upgrade"
  Write-TextFile -Path $mod -Text "mod-before-upgrade"
  Write-TextFile -Path $ue4ss -Text "ue4ss-before-upgrade"
  Write-TextFile -Path $palDefender -Text "paldefender-before-upgrade"
  $preserved = @{}
  foreach ($path in @($config, $database, $save, $mod, $ue4ss, $palDefender)) {
    $preserved[$path] = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash
  }

  $invalidPackageRejected = $false
  try {
    & (Join-Path $ScriptDir "upgrade-windows.ps1") -InstallRoot $install -Archive $candidateB.Archive -ExpectedSHA256 ([string]::new([char]48, 64)) -SkipStartupValidation -KeepRollback
  } catch {
    $invalidPackageRejected = $true
  }
  $InsideHelper.Refresh()
  if (-not $invalidPackageRejected -or $InsideHelper.HasExited) {
    throw "invalid candidate preflight stopped a managed process"
  }

  & (Join-Path $ScriptDir "upgrade-windows.ps1") -InstallRoot $install -Archive $candidateB.Archive -SkipStartupValidation -KeepRollback
  $InsideHelper.Refresh()
  $OutsideHelper.Refresh()
  if (-not $InsideHelper.HasExited) {
    throw "upgrade did not stop the process owned by the installation"
  }
  if ($OutsideHelper.HasExited) {
    throw "upgrade killed a same-name process outside the installation"
  }
  Stop-Process -Id $OutsideHelper.Id -Force
  $OutsideHelper.WaitForExit(5000) | Out-Null
  foreach ($path in $preserved.Keys) {
    Assert-FileHashEqual -Path $path -Expected $preserved[$path]
  }
  $marker = [System.Text.Encoding]::ASCII.GetString([System.IO.File]::ReadAllBytes((Join-Path $install "PalPanel.exe")))
  if ($marker -notmatch "B-PalPanel.exe") {
    throw "successful overlay upgrade did not replace the packaged executable"
  }
  if (@(Get-ChildItem -LiteralPath (Join-Path $install ".palpanel-maintenance\config-backups") -Recurse -Filter "palpanel.env" -File).Count -eq 0) {
    throw "successful upgrade did not retain a configuration recovery snapshot"
  }

  Write-TextFile -Path $config -Text "malformed configuration"
  & (Join-Path $ScriptDir "recover-windows-config.ps1") -InstallRoot $install -RestoreLatest -ConfirmRecovery RESTORE_PALPANEL_CONFIG
  Assert-FileHashEqual -Path $config -Expected $preserved[$config]
  & (Join-Path $ScriptDir "recover-windows-config.ps1") -InstallRoot $install -RestoreLatest -ConfirmRecovery RESTORE_PALPANEL_CONFIG
  Assert-FileHashEqual -Path $config -Expected $preserved[$config]

  $externalRuntime = Join-Path $TestRoot "external-runtime"
  Write-TextFile -Path $config -Text "PALPANEL_LISTEN_ADDR=127.0.0.1:18080`nPALPANEL_DB_PATH=data\palpanel.db`nPALPANEL_SERVER_DIR=$externalRuntime`n"
  $externalRejected = $false
  try {
    & (Join-Path $ScriptDir "upgrade-windows.ps1") -InstallRoot $install -Archive $candidateC.Archive -KeepRollback
  } catch {
    $externalRejected = $true
  }
  if (-not $externalRejected -or (Test-Path -LiteralPath $externalRuntime)) {
    throw "upgrade startup validation accepted or created an external runtime path"
  }
  $markerAfterExternalRejection = [System.Text.Encoding]::ASCII.GetString([System.IO.File]::ReadAllBytes((Join-Path $install "PalPanel.exe")))
  if ($markerAfterExternalRejection -notmatch "B-PalPanel.exe") {
    throw "external path preflight changed the installed payload"
  }
  Write-TextFile -Path $config -Text "PALPANEL_LISTEN_ADDR=127.0.0.1:18080`nPALPANEL_DB_PATH=data\palpanel.db`n"
  Assert-FileHashEqual -Path $config -Expected $preserved[$config]

  $failedUpgrade = $false
  try {
    & (Join-Path $ScriptDir "upgrade-windows.ps1") -InstallRoot $install -Archive $candidateC.Archive -KeepRollback
  } catch {
    $failedUpgrade = $true
  }
  if (-not $failedUpgrade) {
    throw "fixture Launcher unexpectedly passed startup validation"
  }
  $markerAfterRollback = [System.Text.Encoding]::ASCII.GetString([System.IO.File]::ReadAllBytes((Join-Path $install "PalPanel.exe")))
  if ($markerAfterRollback -notmatch "B-PalPanel.exe") {
    throw "failed upgrade did not roll back the previous payload"
  }
  foreach ($path in $preserved.Keys) {
    Assert-FileHashEqual -Path $path -Expected $preserved[$path]
  }

  $candidateMarker = Join-Path $install "data\candidate-launcher-ran.txt"
  $quotedConfig = "PALPANEL_LISTEN_ADDR=127.0.0.1:18080`nPALPANEL_DB_PATH='data\palpanel.db'`nPALPANEL_SERVER_DIR=`"data/server`"`n"
  Write-TextFile -Path $config -Text $quotedConfig
  $quotedConfigHash = (Get-FileHash -LiteralPath $config -Algorithm SHA256).Hash
  $mutatingUpgradeFailed = $false
  try {
    & (Join-Path $ScriptDir "upgrade-windows.ps1") -InstallRoot $install -Archive $candidateD.Archive -KeepRollback
  } catch {
    $mutatingUpgradeFailed = $true
  }
  if (-not $mutatingUpgradeFailed -or -not (Test-Path -LiteralPath $candidateMarker -PathType Leaf)) {
    throw "quoted managed paths did not reach candidate startup validation"
  }
  Assert-FileHashEqual -Path $config -Expected $quotedConfigHash
  Remove-Item -LiteralPath $candidateMarker -Force

  Remove-Item -LiteralPath $config -Force
  $newConfigUpgradeFailed = $false
  try {
    & (Join-Path $ScriptDir "upgrade-windows.ps1") -InstallRoot $install -Archive $candidateD.Archive -KeepRollback
  } catch {
    $newConfigUpgradeFailed = $true
  }
  if (-not $newConfigUpgradeFailed -or -not (Test-Path -LiteralPath $candidateMarker -PathType Leaf)) {
    throw "missing-config rollback fixture did not run"
  }
  if (Test-Path -LiteralPath $config) {
    throw "failed upgrade retained a configuration created by the candidate Launcher"
  }
  Remove-Item -LiteralPath $candidateMarker -Force
  Write-TextFile -Path $config -Text "PALPANEL_LISTEN_ADDR=127.0.0.1:18080`nPALPANEL_DB_PATH=data\palpanel.db`n"
  Assert-FileHashEqual -Path $config -Expected $preserved[$config]

  $rejectedPurge = $false
  try {
    & (Join-Path $ScriptDir "uninstall-windows.ps1") -InstallRoot $install -PurgeData
  } catch {
    $rejectedPurge = $true
  }
  if (-not $rejectedPurge) {
    throw "full cleanup without explicit confirmation was accepted"
  }
  Assert-FileHashEqual -Path $database -Expected $preserved[$database]

  & (Join-Path $ScriptDir "uninstall-windows.ps1") -InstallRoot $install
  foreach ($path in @($config, $database, $save, $mod, $ue4ss, $palDefender)) {
    Assert-FileHashEqual -Path $path -Expected $preserved[$path]
  }
  if (Test-Path -LiteralPath (Join-Path $install "PalPanel.exe")) {
    throw "normal uninstall retained the Launcher binary"
  }
  if (-not (Test-Path -LiteralPath (Join-Path $install ".palpanel-maintenance\uninstall.json") -PathType Leaf)) {
    throw "normal uninstall did not retain an uninstall record"
  }

  $junctionTarget = Join-Path $TestRoot "junction-target"
  $junctionMarker = Join-Path $junctionTarget "must-survive.txt"
  Write-TextFile -Path $junctionMarker -Text "outside managed install"
  $junction = Join-Path $install "data\unsafe-junction"
  New-Item -ItemType Junction -Path $junction -Target $junctionTarget | Out-Null
  $reparseRejected = $false
  try {
    & (Join-Path $ScriptDir "uninstall-windows.ps1") -InstallRoot $install -PurgeData -ConfirmPurge PURGE_PALPANEL_MANAGED_DATA
  } catch {
    $reparseRejected = $true
  }
  if (-not $reparseRejected) {
    throw "complete cleanup traversed a runtime junction"
  }
  if (-not (Test-Path -LiteralPath $config -PathType Leaf) -or -not (Test-Path -LiteralPath $junctionMarker -PathType Leaf)) {
    throw "failed purge preflight changed configuration or junction target data"
  }
  [System.IO.Directory]::Delete($junction)

  & (Join-Path $ScriptDir "uninstall-windows.ps1") -InstallRoot $install -PurgeData -ConfirmPurge PURGE_PALPANEL_MANAGED_DATA
  foreach ($path in @("config", "data", ".palpanel-maintenance")) {
    if (Test-Path -LiteralPath (Join-Path $install $path)) {
      throw "complete cleanup retained managed runtime path: $path"
    }
  }
  if (-not (Test-Path -LiteralPath $install -PathType Container)) {
    throw "complete cleanup removed the installation root itself"
  }

  $sourceRootRejected = $false
  try {
    & (Join-Path $ScriptDir "uninstall-windows.ps1") -InstallRoot $RepositoryRoot
  } catch {
    $sourceRootRejected = $true
  }
  if (-not $sourceRootRejected) {
    throw "uninstall accepted the source repository root"
  }
  $Succeeded = $true
  Write-Host "Windows upgrade, uninstall, and recovery contract passed"
} finally {
  foreach ($process in @($InsideHelper, $OutsideHelper)) {
    if ($null -ne $process) {
      try {
        $process.Refresh()
        if (-not $process.HasExited) {
          Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
          $process.WaitForExit(5000) | Out-Null
        }
      } catch { }
      $process.Dispose()
    }
  }
  if ($Succeeded -and -not $KeepArtifacts -and (Test-Path -LiteralPath $TestRoot)) {
    Remove-PalPanelManagedDirectory -RepositoryRoot $RepositoryRoot -RuntimeRoot $RuntimeRoot -TargetPath $TestRoot
  } elseif (Test-Path -LiteralPath $TestRoot) {
    Write-Host "Upgrade contract artifacts retained: $TestRoot"
  }
}
