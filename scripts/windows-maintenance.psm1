Set-StrictMode -Version Latest

$script:PalPanelPayloadFiles = @(
  "PalPanel.exe",
  "palpanel-server.exe",
  "sav-cli.exe",
  "palcalc-bridge.exe",
  "README.md",
  "LICENSE",
  "THIRD_PARTY_LICENSES.txt",
  "checksums.txt",
  "config\palpanel.env.example"
)
$script:PalPanelPayloadDirectories = @("backend", "licenses", "maintenance")
$script:PalPanelRequiredPackageFiles = @(
  "PalPanel.exe",
  "palpanel-server.exe",
  "sav-cli.exe",
  "palcalc-bridge.exe",
  "README.md",
  "LICENSE",
  "THIRD_PARTY_LICENSES.txt",
  "checksums.txt",
  "config\palpanel.env.example",
  "licenses\sav-cli-LICENSE.txt",
  "licenses\pallocalize-Apache-2.0.txt",
  "licenses\PalDefender-MIT.txt",
  "licenses\PalCalc-MIT.txt",
  "maintenance\windows-maintenance.psm1",
  "maintenance\upgrade-windows.ps1",
  "maintenance\uninstall-windows.ps1",
  "maintenance\recover-windows-config.ps1"
)

function Resolve-PalPanelMaintenancePath {
  param(
    [Parameter(Mandatory = $true)][string]$Path,
    [Parameter(Mandatory = $true)][string]$BasePath
  )

  if ([string]::IsNullOrWhiteSpace($Path)) {
    throw "path must not be empty"
  }
  $base = [System.IO.Path]::GetFullPath($BasePath)
  $candidate = $Path.Trim()
  if (-not [System.IO.Path]::IsPathRooted($candidate)) {
    $candidate = Join-Path $base $candidate
  }
  return [System.IO.Path]::GetFullPath($candidate)
}

function ConvertFrom-PalPanelEnvironmentValue {
  param([AllowEmptyString()][string]$RawValue)

  $raw = $RawValue.Trim()
  if ($raw.Length -eq 0) {
    return ""
  }
  if ($raw.IndexOf([char]0) -ge 0) {
    throw "configuration value contains NUL"
  }
  if ($raw[0] -eq [char]39) {
    if ($raw.Length -lt 2 -or $raw[$raw.Length - 1] -ne [char]39) {
      throw "unterminated single-quoted configuration value"
    }
    return $raw.Substring(1, $raw.Length - 2)
  }
  if ($raw[0] -ne [char]34) {
    if ($raw.IndexOf([char]34) -ge 0 -or $raw.IndexOf([char]39) -ge 0) {
      throw "quotes must surround the entire configuration value"
    }
    return $raw
  }
  if ($raw.Length -lt 2 -or $raw[$raw.Length - 1] -ne [char]34) {
    throw "unterminated double-quoted configuration value"
  }

  $inner = $raw.Substring(1, $raw.Length - 2)
  $builder = [System.Text.StringBuilder]::new()
  for ($index = 0; $index -lt $inner.Length; $index++) {
    $character = $inner[$index]
    if ($character -ne [char]92) {
      [void]$builder.Append($character)
      continue
    }
    $index++
    if ($index -ge $inner.Length) {
      throw "unterminated escape in double-quoted configuration value"
    }
    $escape = $inner[$index]
    $simpleCode = -1
    $digits = 0
    switch -CaseSensitive ([string]$escape) {
      'a' { $simpleCode = 7 }
      'b' { $simpleCode = 8 }
      'f' { $simpleCode = 12 }
      'n' { $simpleCode = 10 }
      'r' { $simpleCode = 13 }
      't' { $simpleCode = 9 }
      'v' { $simpleCode = 11 }
      '"' { $simpleCode = 34 }
      '\' { $simpleCode = 92 }
      'x' { $digits = 2 }
      'u' { $digits = 4 }
      'U' { $digits = 8 }
    }
    if ($simpleCode -ge 0) {
      [void]$builder.Append([char]$simpleCode)
      continue
    }
    if ($digits -eq 0) {
      if ($escape -ge [char]48 -and $escape -le [char]55) {
        if (($index + 2) -ge $inner.Length) {
          throw "short octal escape in double-quoted configuration value"
        }
        $octal = ([string]$escape) + [string]$inner[$index + 1] + [string]$inner[$index + 2]
        if ($octal -notmatch '^[0-7]{3}$') {
          throw "invalid octal escape in double-quoted configuration value"
        }
        [void]$builder.Append([char][Convert]::ToInt32($octal, 8))
        $index += 2
        continue
      }
      throw "invalid escape in double-quoted configuration value: \$escape"
    }
    if (($index + $digits) -ge $inner.Length) {
      throw "short Unicode escape in double-quoted configuration value"
    }
    $hex = $inner.Substring($index + 1, $digits)
    if ($hex -notmatch "^[0-9a-fA-F]{$digits}$") {
      throw "invalid Unicode escape in double-quoted configuration value"
    }
    $codePoint = [Convert]::ToInt32($hex, 16)
    if ($digits -eq 2) {
      [void]$builder.Append([char]$codePoint)
    } else {
      if ($codePoint -gt 0x10ffff -or ($codePoint -ge 0xd800 -and $codePoint -le 0xdfff)) {
        throw "invalid Unicode code point in double-quoted configuration value"
      }
      [void]$builder.Append([char]::ConvertFromUtf32($codePoint))
    }
    $index += $digits
  }
  return $builder.ToString()
}

function Test-PalPanelMaintenancePathWithin {
  param(
    [Parameter(Mandatory = $true)][string]$Root,
    [Parameter(Mandatory = $true)][string]$Target
  )

  $rootFull = [System.IO.Path]::GetFullPath($Root).TrimEnd('\', '/')
  $targetFull = [System.IO.Path]::GetFullPath($Target).TrimEnd('\', '/')
  if ([string]::Equals($rootFull, $targetFull, [System.StringComparison]::OrdinalIgnoreCase)) {
    return $true
  }
  return $targetFull.StartsWith($rootFull + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)
}

function Test-PalPanelMaintenanceVolumeRoot {
  param([Parameter(Mandatory = $true)][string]$Path)

  $full = [System.IO.Path]::GetFullPath($Path)
  $volume = [System.IO.Path]::GetPathRoot($full)
  if ([string]::IsNullOrWhiteSpace($volume)) {
    return $false
  }
  return [string]::Equals($full.TrimEnd('\', '/'), $volume.TrimEnd('\', '/'), [System.StringComparison]::OrdinalIgnoreCase)
}

function Assert-PalPanelMaintenanceNoReparsePath {
  param([Parameter(Mandatory = $true)][string]$Path)

  $current = [System.IO.Path]::GetFullPath($Path)
  while (-not (Test-Path -LiteralPath $current)) {
    $parent = Split-Path -Parent $current
    if ([string]::IsNullOrWhiteSpace($parent) -or [string]::Equals($parent, $current, [System.StringComparison]::OrdinalIgnoreCase)) {
      return
    }
    $current = $parent
  }
  while ($true) {
    $item = Get-Item -LiteralPath $current -Force
    if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
      throw "managed path contains a reparse point: $current"
    }
    $parent = Split-Path -Parent $current
    if ([string]::IsNullOrWhiteSpace($parent) -or [string]::Equals($parent, $current, [System.StringComparison]::OrdinalIgnoreCase)) {
      return
    }
    $current = $parent
  }
}

function Assert-PalPanelMaintenanceNoReparseTree {
  param([Parameter(Mandatory = $true)][string]$Path)

  Assert-PalPanelMaintenanceNoReparsePath -Path $Path
  if (-not (Test-Path -LiteralPath $Path)) {
    return
  }
  $pending = [System.Collections.Generic.Stack[string]]::new()
  $pending.Push([System.IO.Path]::GetFullPath($Path))
  while ($pending.Count -gt 0) {
    $current = $pending.Pop()
    foreach ($entry in Get-ChildItem -LiteralPath $current -Force) {
      if (($entry.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "refusing to operate on a directory tree containing a reparse point: $($entry.FullName)"
      }
      if ($entry.PSIsContainer) {
        $pending.Push($entry.FullName)
      }
    }
  }
}

function Get-PalPanelMaintenanceRoot {
  param([Parameter(Mandatory = $true)][string]$InstallRoot)

  $root = Assert-PalPanelReleaseRoot -InstallRoot $InstallRoot -AllowUninstalled
  $maintenance = Join-Path $root ".palpanel-maintenance"
  Assert-PalPanelMaintenanceNoReparsePath -Path $maintenance
  return $maintenance
}

function Assert-PalPanelReleaseRoot {
  param(
    [Parameter(Mandatory = $true)][string]$InstallRoot,
    [switch]$AllowUninstalled
  )

  $root = [System.IO.Path]::GetFullPath($InstallRoot)
  if (Test-PalPanelMaintenanceVolumeRoot -Path $root) {
    throw "installation root must not be a volume root: $root"
  }
  if (-not (Test-Path -LiteralPath $root -PathType Container)) {
    throw "installation root does not exist: $root"
  }
  Assert-PalPanelMaintenanceNoReparsePath -Path $root
  foreach ($sourceMarker in @(".git", "backend\go.mod", "frontend\package.json", "sav-cli\go.mod")) {
    if (Test-Path -LiteralPath (Join-Path $root $sourceMarker)) {
      throw "refusing to operate on a source repository: $root"
    }
  }

  $packageFiles = @("PalPanel.exe", "palpanel-server.exe", "sav-cli.exe", "checksums.txt")
  $looksInstalled = @($packageFiles | Where-Object { Test-Path -LiteralPath (Join-Path $root $_) -PathType Leaf }).Count -eq $packageFiles.Count
  $identity = Join-Path $root ".palpanel-maintenance\install.json"
  if (-not $looksInstalled -and -not (Test-Path -LiteralPath $identity -PathType Leaf)) {
    if ($AllowUninstalled) {
      throw "installation identity is missing: expected PalPanel package files or $identity"
    }
    throw "PalPanel package files are missing from installation root: $root"
  }
  return $root
}

function Assert-PalPanelReleaseManagedPath {
  param(
    [Parameter(Mandatory = $true)][string]$InstallRoot,
    [Parameter(Mandatory = $true)][string]$TargetPath
  )

  $root = Assert-PalPanelReleaseRoot -InstallRoot $InstallRoot -AllowUninstalled
  $target = Resolve-PalPanelMaintenancePath -Path $TargetPath -BasePath $root
  if ([string]::Equals($root, $target, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "refusing to operate on the installation root itself: $target"
  }
  if (-not (Test-PalPanelMaintenancePathWithin -Root $root -Target $target)) {
    throw "managed path escapes installation root: $target"
  }
  Assert-PalPanelMaintenanceNoReparsePath -Path $target
  return $target
}

function Remove-PalPanelReleaseManagedPath {
  param(
    [Parameter(Mandatory = $true)][string]$InstallRoot,
    [Parameter(Mandatory = $true)][string]$TargetPath
  )

  $target = Assert-PalPanelReleaseManagedPath -InstallRoot $InstallRoot -TargetPath $TargetPath
  if (-not (Test-Path -LiteralPath $target)) {
    return
  }
  Assert-PalPanelMaintenanceNoReparseTree -Path $target
  Remove-Item -LiteralPath $target -Recurse -Force
}

function Get-PalPanelPayloadItemRoots {
  $items = [System.Collections.Generic.List[object]]::new()
  foreach ($relative in $script:PalPanelPayloadFiles) {
    $items.Add([pscustomobject]@{ RelativePath = $relative; IsDirectory = $false })
  }
  foreach ($relative in $script:PalPanelPayloadDirectories) {
    $items.Add([pscustomobject]@{ RelativePath = $relative; IsDirectory = $true })
  }
  return $items
}

function ConvertTo-PalPanelPackageRelativePath {
  param([Parameter(Mandatory = $true)][string]$Path)

  $normalized = $Path.Replace('\', '/').Trim('/')
  if ([string]::IsNullOrWhiteSpace($normalized)) {
    throw "package path must not be empty"
  }
  if ($normalized.StartsWith('/') -or $normalized.Contains(':')) {
    throw "package path must be relative: $Path"
  }
  $components = $normalized.Split('/')
  foreach ($component in $components) {
    if ([string]::IsNullOrWhiteSpace($component) -or $component -eq "." -or $component -eq "..") {
      throw "package path contains an unsafe component: $Path"
    }
    if ($component.EndsWith('.') -or $component.EndsWith(' ') -or $component.IndexOfAny([char[]]'<>"|?*') -ge 0) {
      throw "package path contains a Windows-unsafe component: $Path"
    }
    foreach ($character in $component.ToCharArray()) {
      if ([int]$character -lt 0x20 -or [int]$character -eq 0x7f) {
        throw "package path contains a control character: $Path"
      }
    }
    $deviceBase = $component.Split('.')[0].ToUpperInvariant()
    if ($deviceBase -in @("CON", "PRN", "AUX", "NUL", "CLOCK$") -or $deviceBase -match '^(COM|LPT)[1-9]$') {
      throw "package path uses a reserved Windows device name: $Path"
    }
  }
  return ($components -join '/')
}

function Test-PalPanelAllowedPayloadPath {
  param([Parameter(Mandatory = $true)][string]$RelativePath)

  $normalized = ConvertTo-PalPanelPackageRelativePath -Path $RelativePath
  $windowsPath = $normalized.Replace('/', '\')
  if ([string]::Equals($windowsPath, "config", [System.StringComparison]::OrdinalIgnoreCase)) {
    return $true
  }
  foreach ($file in $script:PalPanelPayloadFiles) {
    if ([string]::Equals($windowsPath, $file, [System.StringComparison]::OrdinalIgnoreCase)) {
      return $true
    }
  }
  foreach ($directory in $script:PalPanelPayloadDirectories) {
    if ([string]::Equals($windowsPath, $directory, [System.StringComparison]::OrdinalIgnoreCase)) {
      return $true
    }
    $prefix = $directory.Replace('\', '/') + "/"
    if ($normalized.StartsWith($prefix, [System.StringComparison]::OrdinalIgnoreCase)) {
      return $true
    }
  }
  return $false
}

function Test-PalPanelPEFile {
  param([Parameter(Mandatory = $true)][string]$Path)

  $stream = [System.IO.File]::Open($Path, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, [System.IO.FileShare]::Read)
  try {
    if ($stream.Length -lt 68) {
      throw "PE file is too short: $Path"
    }
    $reader = [System.IO.BinaryReader]::new($stream)
    if ($reader.ReadByte() -ne 0x4d -or $reader.ReadByte() -ne 0x5a) {
      throw "PE file is missing the MZ header: $Path"
    }
    $stream.Position = 0x3c
    $offset = $reader.ReadInt32()
    if ($offset -lt 0 -or ($offset + 4) -gt $stream.Length) {
      throw "PE file has an invalid header offset: $Path"
    }
    $stream.Position = $offset
    if ($reader.ReadByte() -ne 0x50 -or $reader.ReadByte() -ne 0x45 -or $reader.ReadByte() -ne 0 -or $reader.ReadByte() -ne 0) {
      throw "PE file is missing the PE signature: $Path"
    }
  } finally {
    $stream.Dispose()
  }
}

function Test-PalPanelPayloadChecksums {
  param(
    [Parameter(Mandatory = $true)][string]$PackageRoot,
    [switch]$RequirePackageFiles
  )

  $root = [System.IO.Path]::GetFullPath($PackageRoot)
  if (-not (Test-Path -LiteralPath $root -PathType Container)) {
    throw "package root is missing: $root"
  }
  Assert-PalPanelMaintenanceNoReparseTree -Path $root
  $checksumPath = Join-Path $root "checksums.txt"
  if (-not (Test-Path -LiteralPath $checksumPath -PathType Leaf)) {
    throw "package checksum manifest is missing: $checksumPath"
  }
  if ($RequirePackageFiles) {
    foreach ($relative in $script:PalPanelRequiredPackageFiles) {
      if (-not (Test-Path -LiteralPath (Join-Path $root $relative) -PathType Leaf)) {
        throw "package is missing required file: $relative"
      }
    }
  }
  foreach ($executable in @("PalPanel.exe", "palpanel-server.exe", "sav-cli.exe")) {
    Test-PalPanelPEFile -Path (Join-Path $root $executable)
  }

  $seen = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)
  $verified = 0
  foreach ($line in Get-Content -LiteralPath $checksumPath) {
    if ($line -notmatch '^([0-9a-fA-F]{64})  \./(.+)$') {
      throw "invalid checksum manifest line: $line"
    }
    $expected = $Matches[1].ToLowerInvariant()
    $relative = ConvertTo-PalPanelPackageRelativePath -Path $Matches[2]
    if (-not (Test-PalPanelAllowedPayloadPath -RelativePath $relative)) {
      throw "checksum manifest references an unmanaged package path: $relative"
    }
    if (-not $seen.Add($relative)) {
      throw "checksum manifest contains a duplicate path: $relative"
    }
    $path = Join-Path $root $relative.Replace('/', '\')
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
      throw "checksummed package file is missing: $relative"
    }
    $actual = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected) {
      throw "checksum mismatch for package file: $relative"
    }
    $verified++
  }
  $files = @(Get-ChildItem -LiteralPath $root -File -Recurse)
  $payloadFiles = @($files | Where-Object { $_.FullName -ne $checksumPath })
  if ($verified -ne $payloadFiles.Count) {
    throw "checksum manifest covers $verified files, package contains $($payloadFiles.Count) files"
  }
}

function Test-PalPanelInstalledPayload {
  param([Parameter(Mandatory = $true)][string]$InstallRoot)

  $root = Assert-PalPanelReleaseRoot -InstallRoot $InstallRoot -AllowUninstalled
  $checksumPath = Join-Path $root "checksums.txt"
  if (-not (Test-Path -LiteralPath $checksumPath -PathType Leaf)) {
    throw "installed checksum manifest is missing: $checksumPath"
  }
  foreach ($relative in $script:PalPanelRequiredPackageFiles) {
    if (-not (Test-Path -LiteralPath (Join-Path $root $relative) -PathType Leaf)) {
      throw "installed package is missing required file: $relative"
    }
  }
  foreach ($executable in @("PalPanel.exe", "palpanel-server.exe", "sav-cli.exe")) {
    Test-PalPanelPEFile -Path (Join-Path $root $executable)
  }
  $seen = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)
  foreach ($line in Get-Content -LiteralPath $checksumPath) {
    if ($line -notmatch '^([0-9a-fA-F]{64})  \./(.+)$') {
      throw "invalid installed checksum manifest line: $line"
    }
    $expected = $Matches[1].ToLowerInvariant()
    $relative = ConvertTo-PalPanelPackageRelativePath -Path $Matches[2]
    if (-not (Test-PalPanelAllowedPayloadPath -RelativePath $relative)) {
      throw "installed checksum manifest references an unmanaged package path: $relative"
    }
    if (-not $seen.Add($relative)) {
      throw "installed checksum manifest contains a duplicate path: $relative"
    }
    $path = Join-Path $root $relative.Replace('/', '\')
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
      throw "checksummed installed file is missing: $relative"
    }
    $actual = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected) {
      throw "checksum mismatch for installed file: $relative"
    }
  }
}

function Expand-PalPanelUpgradeArchive {
  param(
    [Parameter(Mandatory = $true)][string]$Archive,
    [Parameter(Mandatory = $true)][string]$Destination,
    [string]$ExpectedSHA256 = ""
  )

  Add-Type -AssemblyName System.IO.Compression.FileSystem
  $archivePath = [System.IO.Path]::GetFullPath($Archive)
  if (-not (Test-Path -LiteralPath $archivePath -PathType Leaf)) {
    throw "upgrade archive is missing: $archivePath"
  }
  if (-not [string]::IsNullOrWhiteSpace($ExpectedSHA256)) {
    if ($ExpectedSHA256 -notmatch '^[0-9a-fA-F]{64}$') {
      throw "ExpectedSHA256 must contain 64 hexadecimal characters"
    }
    $actualArchiveHash = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash
    if (-not [string]::Equals($actualArchiveHash, $ExpectedSHA256, [System.StringComparison]::OrdinalIgnoreCase)) {
      throw "upgrade archive SHA-256 does not match ExpectedSHA256"
    }
  }

  $destinationPath = [System.IO.Path]::GetFullPath($Destination)
  if (Test-Path -LiteralPath $destinationPath) {
    throw "upgrade staging destination already exists: $destinationPath"
  }
  $parent = Split-Path -Parent $destinationPath
  if ([string]::IsNullOrWhiteSpace($parent) -or -not (Test-Path -LiteralPath $parent -PathType Container)) {
    throw "upgrade staging parent is missing: $parent"
  }
  Assert-PalPanelMaintenanceNoReparsePath -Path $parent

  $zip = [System.IO.Compression.ZipFile]::OpenRead($archivePath)
  $packageName = ""
  try {
    if ($zip.Entries.Count -gt 200000) {
      throw "upgrade archive contains too many entries"
    }
    [int64]$declaredBytes = 0
    $seen = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)
    foreach ($entry in $zip.Entries) {
      $isDirectory = $entry.FullName.EndsWith('/') -or $entry.FullName.EndsWith('\')
      $normalized = $entry.FullName.Replace('\', '/').TrimEnd('/')
      if ([string]::IsNullOrWhiteSpace($normalized)) {
        continue
      }
      $normalized = ConvertTo-PalPanelPackageRelativePath -Path $normalized
      $components = $normalized.Split('/')
      if ($components.Count -lt 1) {
        throw "upgrade archive entry is outside its package root: $($entry.FullName)"
      }
      if ([string]::IsNullOrWhiteSpace($packageName)) {
        $packageName = $components[0]
        if ($packageName -notmatch '^palpanel_[A-Za-z0-9][A-Za-z0-9._+-]*_windows_amd64$') {
          throw "upgrade archive package root is invalid: $packageName"
        }
      } elseif (-not [string]::Equals($packageName, $components[0], [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "upgrade archive contains more than one package root"
      }
      if ($components.Count -eq 1) {
        if (-not $isDirectory) {
          throw "upgrade archive contains a file at its package root: $($entry.FullName)"
        }
        continue
      }
      $relative = ($components[1..($components.Count - 1)] -join '/')
      if (-not (Test-PalPanelAllowedPayloadPath -RelativePath $relative)) {
        throw "upgrade archive contains an unmanaged payload path: $relative"
      }
      if ($isDirectory -and $relative -notmatch '^(config|backend|licenses|maintenance)(/.*)?$') {
        throw "upgrade archive contains an unmanaged directory: $relative"
      }
      if (-not $seen.Add($normalized)) {
        throw "upgrade archive contains a duplicate case-insensitive path: $($entry.FullName)"
      }
      if ($entry.Length -gt 512MB -or $declaredBytes -gt (4GB - $entry.Length)) {
        throw "upgrade archive exceeds the extracted size limit"
      }
      $declaredBytes += $entry.Length
      $unixType = (($entry.ExternalAttributes -shr 16) -band 0xF000)
      if ($unixType -eq 0xA000) {
        throw "upgrade archive contains a symbolic link: $($entry.FullName)"
      }
    }
    if ([string]::IsNullOrWhiteSpace($packageName)) {
      throw "upgrade archive is empty"
    }
  } finally {
    $zip.Dispose()
  }

  Expand-Archive -LiteralPath $archivePath -DestinationPath $destinationPath
  $packageRoot = Join-Path $destinationPath $packageName
  Test-PalPanelPayloadChecksums -PackageRoot $packageRoot -RequirePackageFiles
  return $packageRoot
}

function Get-PalPanelManagedProcesses {
  param([Parameter(Mandatory = $true)][string]$InstallRoot)

  $root = Assert-PalPanelReleaseRoot -InstallRoot $InstallRoot -AllowUninstalled
  $staticPaths = @(
    (Join-Path $root "PalPanel.exe"),
    (Join-Path $root "palpanel-server.exe"),
    (Join-Path $root "sav-cli.exe")
  ) | ForEach-Object { [System.IO.Path]::GetFullPath($_) }
  $serverRoots = @(
    (Join-Path $root "data\server"),
    (Join-Path $root "palworld")
  ) | Where-Object { Test-Path -LiteralPath $_ -PathType Container }
  foreach ($path in $serverRoots) {
    Assert-PalPanelMaintenanceNoReparsePath -Path $path
  }
  try {
    $processes = @(Get-CimInstance -ClassName Win32_Process -Property ProcessId, ParentProcessId, ExecutablePath, Name)
  } catch {
    throw "could not enumerate Windows processes: $($_.Exception.Message)"
  }
  $owned = [System.Collections.Generic.List[object]]::new()
  foreach ($process in $processes) {
    if ([string]::IsNullOrWhiteSpace($process.ExecutablePath)) {
      continue
    }
    try {
      $path = [System.IO.Path]::GetFullPath($process.ExecutablePath)
    } catch {
      continue
    }
    $matchesStatic = $false
    foreach ($staticPath in $staticPaths) {
      if ([string]::Equals($path, $staticPath, [System.StringComparison]::OrdinalIgnoreCase)) {
        $matchesStatic = $true
        break
      }
    }
    $matchesServer = $false
    foreach ($serverRoot in $serverRoots) {
      if (Test-PalPanelMaintenancePathWithin -Root $serverRoot -Target $path) {
        $matchesServer = $true
        break
      }
    }
    if ($matchesStatic -or $matchesServer) {
      $owned.Add([pscustomobject]@{
        ProcessId = [int]$process.ProcessId
        ParentProcessId = [int]$process.ParentProcessId
        ExecutablePath = $path
        Name = [string]$process.Name
      })
    }
  }
  return $owned
}

function Stop-PalPanelManagedProcesses {
  param(
    [Parameter(Mandatory = $true)][string]$InstallRoot,
    [ValidateRange(1, 300)][int]$TimeoutSeconds = 60
  )

  $direct = @(Get-PalPanelManagedProcesses -InstallRoot $InstallRoot)
  if ($direct.Count -eq 0) {
    return @()
  }
  $all = @()
  try {
    $all = @(Get-CimInstance -ClassName Win32_Process -Property ProcessId, ParentProcessId, ExecutablePath, Name)
  } catch {
    throw "could not enumerate Windows process tree: $($_.Exception.Message)"
  }
  $owned = [System.Collections.Generic.Dictionary[int, object]]::new()
  foreach ($process in $direct) {
    $owned[$process.ProcessId] = $process
  }
  $changed = $true
  while ($changed) {
    $changed = $false
    foreach ($process in $all) {
      $pid = [int]$process.ProcessId
      $parent = [int]$process.ParentProcessId
      if (-not $owned.ContainsKey($pid) -and $owned.ContainsKey($parent)) {
        $owned[$pid] = [pscustomobject]@{
          ProcessId = $pid
          ParentProcessId = $parent
          ExecutablePath = [string]$process.ExecutablePath
          Name = [string]$process.Name
        }
        $changed = $true
      }
    }
  }
  $depths = @{}
  function Get-PalPanelProcessDepth([int]$ProcessId) {
    if ($depths.ContainsKey($ProcessId)) { return [int]$depths[$ProcessId] }
    $depth = 0
    $current = $ProcessId
    while ($owned.ContainsKey($current)) {
      $parent = [int]$owned[$current].ParentProcessId
      if (-not $owned.ContainsKey($parent)) { break }
      $depth++
      $current = $parent
      if ($depth -gt 128) { break }
    }
    $depths[$ProcessId] = $depth
    return $depth
  }
  $stopped = [System.Collections.Generic.List[int]]::new()
  foreach ($process in @($owned.Values | Sort-Object @{ Expression = { Get-PalPanelProcessDepth $_.ProcessId }; Descending = $true })) {
    try {
      Stop-Process -Id $process.ProcessId -Force -ErrorAction Stop
      $stopped.Add($process.ProcessId)
    } catch {
      if (Get-Process -Id $process.ProcessId -ErrorAction SilentlyContinue) {
        throw "failed to stop managed process $($process.ProcessId) ($($process.Name)): $($_.Exception.Message)"
      }
    }
  }
  $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
  do {
    $remaining = @($owned.Keys | Where-Object { Get-Process -Id $_ -ErrorAction SilentlyContinue })
    if ($remaining.Count -eq 0) { break }
    Start-Sleep -Milliseconds 200
  } while ([DateTime]::UtcNow -lt $deadline)
  $remaining = @($owned.Keys | Where-Object { Get-Process -Id $_ -ErrorAction SilentlyContinue })
  if ($remaining.Count -gt 0) {
    throw "managed PalPanel processes remained after stop: $($remaining -join ', ')"
  }
  return $stopped
}

function Write-PalPanelMaintenanceJson {
  param(
    [Parameter(Mandatory = $true)]$Value,
    [Parameter(Mandatory = $true)][string]$Path
  )

  $parent = Split-Path -Parent $Path
  if ($parent) {
    New-Item -ItemType Directory -Force -Path $parent | Out-Null
  }
  $json = $Value | ConvertTo-Json -Depth 32
  [System.IO.File]::WriteAllText($Path, $json + [Environment]::NewLine, [System.Text.UTF8Encoding]::new($false))
}

function Read-PalPanelMaintenanceJson {
  param([Parameter(Mandatory = $true)][string]$Path)

  if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
    throw "maintenance state file is missing: $Path"
  }
  try {
    return Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json
  } catch {
    throw "maintenance state file is invalid JSON: $Path"
  }
}

function Write-PalPanelMaintenanceIdentity {
  param([Parameter(Mandatory = $true)][string]$InstallRoot)

  $root = Assert-PalPanelReleaseRoot -InstallRoot $InstallRoot -AllowUninstalled
  $maintenance = Join-Path $root ".palpanel-maintenance"
  Assert-PalPanelReleaseManagedPath -InstallRoot $root -TargetPath $maintenance | Out-Null
  $identity = [ordered]@{
    schema_version = 1
    install_root = $root
    created_at = (Get-Date).ToUniversalTime().ToString("o")
  }
  Write-PalPanelMaintenanceJson -Value $identity -Path (Join-Path $maintenance "install.json")
}

Export-ModuleMember -Function @(
  "Resolve-PalPanelMaintenancePath",
  "ConvertFrom-PalPanelEnvironmentValue",
  "Test-PalPanelMaintenancePathWithin",
  "Assert-PalPanelReleaseRoot",
  "Assert-PalPanelReleaseManagedPath",
  "Assert-PalPanelMaintenanceNoReparsePath",
  "Assert-PalPanelMaintenanceNoReparseTree",
  "Remove-PalPanelReleaseManagedPath",
  "Get-PalPanelMaintenanceRoot",
  "Get-PalPanelPayloadItemRoots",
  "Test-PalPanelPayloadChecksums",
  "Test-PalPanelInstalledPayload",
  "Expand-PalPanelUpgradeArchive",
  "Get-PalPanelManagedProcesses",
  "Stop-PalPanelManagedProcesses",
  "Write-PalPanelMaintenanceJson",
  "Read-PalPanelMaintenanceJson",
  "Write-PalPanelMaintenanceIdentity"
)
