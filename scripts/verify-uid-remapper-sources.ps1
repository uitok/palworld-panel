param()

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$RepositoryRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$Utf8NoBom = [System.Text.UTF8Encoding]::new($false)

$Sources = @(
  [pscustomobject]@{
    Name = "palworld-save-pal"
    Snapshot = "third_party/palworld-save-pal"
    Provenance = "third_party/palworld-save-pal.provenance.json"
    ProvenanceSha256 = "8ee55bc28b4155ca0ac55a8c16c4f3c63988c00c58463c93697da60533497017"
    RepositoryUrl = "https://github.com/oMaN-Rod/palworld-save-pal"
    Commit = "0d99b04acba369ec88550d122794b9917bbf820e"
    GitTreeOid = "e8647b41d14fd354629316bf833dca47e0ad880d"
    GitTreeManifestSha256 = "21f425f9fbdd37eddd212c2b340de08a6d0d93027d2977781ff9d874483bb721"
    ArchiveSha256 = "d22bef8d516da0b98ba7d56c3be5eb9f49968130f58d9b83b11fc0e37427f4f8"
    SnapshotManifestSha256 = "4e0106be04887c4a5e69a5b79156b1545b49f7dfbc4c9460d1a4b2192e730144"
    SnapshotFileCount = 3351
    SnapshotSizeBytes = 70287397
    RequiredFiles = @("Cargo.lock", "Cargo.toml", "README.md")
    LicenseFile = $null
    LicenseMaterialSha256 = "7bf66dd3e22ac43f047be3fea34664ade3a836598ccf33618dcbc2e3e6400c2c"
    LicenseReadmePattern = "MIT License \(do whatever you want with it\)\."
    ExpectedLicense = "README.md License section: MIT License (do whatever you want with it). No standalone LICENSE file exists at this commit."
    StandaloneLicense = $false
    ModeExceptions = @("100755 scripts/build-docker.sh")
  },
  [pscustomobject]@{
    Name = "uesave"
    Snapshot = "third_party/uesave"
    Provenance = "third_party/uesave.provenance.json"
    ProvenanceSha256 = "2574859bd85b8cc968c70388381e07396f02b41eb30d8c8039117a7fb2e13f62"
    RepositoryUrl = "https://github.com/trumank/uesave"
    Commit = "a5271781df0ed021d72e5ad6eab1c59d5199451c"
    GitTreeOid = "d8fc034a4ef57ce75608f5fa4aa2ada8f02e741d"
    GitTreeManifestSha256 = "285a17b371edb2308d135560dc0680e54bbe9b9cec5be43160739276275f4863"
    ArchiveSha256 = "78f1293202411e42c2da1620e2a6628e291b20555e92a79cba87907f79c4615f"
    SnapshotManifestSha256 = "0e8afe6ab9e90ceb00673f7356eced2f41a6be117f16e068c4ac2b9427ac8044"
    SnapshotFileCount = 71
    SnapshotSizeBytes = 1461565
    RequiredFiles = @("Cargo.lock", "Cargo.toml", "README.md", "LICENSE")
    LicenseFile = "LICENSE"
    LicenseMaterialSha256 = "deeabbe360012f52f145850eece6571b69b79f3b6a900872bf1176859ad0f64b"
    LicenseReadmePattern = $null
    ExpectedLicense = "MIT; complete upstream text is preserved in LICENSE."
    StandaloneLicense = $true
    ModeExceptions = @("120000 uesave/README.md -> ../README.md", "120000 uesave_cli/README.md -> ../README.md")
  }
)

$ForbiddenExtensions = @(
  ".exe", ".dll", ".com", ".scr", ".sys", ".msi", ".msp", ".elf", ".so", ".dylib",
  ".a", ".lib", ".o", ".obj", ".pdb", ".wasm", ".zip", ".7z"
)
$ForbiddenNames = @(
  "palpanel-save-migration-toolkit-20260722.zip",
  "20260720T035404.522541500Z-manual.zip"
)
$ForbiddenNamePatterns = @(
  '^(player[-_]?uid[-_]?mappings?|uid[-_]?mappings?)(\..+)?$',
  '^(production[-_]?(migration[-_]?)?report|migration[-_]?report|server[-_]?save)(\..+)?$'
)
$ForbiddenPathComponents = @(".git", "target", "node_modules", "dist", "build", "__pycache__")

function Resolve-RepositoryPath {
  param([Parameter(Mandatory = $true)][string]$RelativePath)

  if ([System.IO.Path]::IsPathRooted($RelativePath)) {
    throw "repository path must be relative: $RelativePath"
  }
  $normalized = $RelativePath.Replace('/', [System.IO.Path]::DirectorySeparatorChar)
  $resolved = [System.IO.Path]::GetFullPath((Join-Path $RepositoryRoot $normalized))
  $rootWithSeparator = $RepositoryRoot.TrimEnd([System.IO.Path]::DirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar
  if (-not $resolved.StartsWith($rootWithSeparator, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "repository path escaped root: $RelativePath"
  }
  return $resolved
}

function Get-PathWithoutReparsePoint {
  param(
    [Parameter(Mandatory = $true)][string]$Path,
    [Parameter(Mandatory = $true)][string]$MissingMessage
  )

  $resolved = [System.IO.Path]::GetFullPath($Path)
  $root = $RepositoryRoot.TrimEnd([System.IO.Path]::DirectorySeparatorChar)
  $rootItem = Get-Item -LiteralPath $root -Force
  if (($rootItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
    throw "repository path contains reparse point: $root"
  }
  if ([string]::Equals($resolved, $root, [System.StringComparison]::OrdinalIgnoreCase)) {
    return $rootItem
  }

  $rootPrefix = $root + [System.IO.Path]::DirectorySeparatorChar
  if (-not $resolved.StartsWith($rootPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "path escaped repository root: $resolved"
  }
  $relative = $resolved.Substring($rootPrefix.Length)
  $current = $root
  foreach ($component in $relative.Split([System.IO.Path]::DirectorySeparatorChar)) {
    $current = Join-Path $current $component
    try {
      $item = Get-Item -LiteralPath $current -Force -ErrorAction Stop
    } catch [System.Management.Automation.ItemNotFoundException] {
      throw $MissingMessage
    }
    if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
      throw "path contains reparse point: $current"
    }
  }
  return $item
}

function Get-SnapshotFilesSafely {
  param([Parameter(Mandatory = $true)][System.IO.DirectoryInfo]$SnapshotRoot)

  $root = $SnapshotRoot.FullName.TrimEnd([System.IO.Path]::DirectorySeparatorChar)
  $rootPrefix = $root + [System.IO.Path]::DirectorySeparatorChar
  $directories = [System.Collections.Generic.Stack[System.IO.DirectoryInfo]]::new()
  $directories.Push($SnapshotRoot)
  $files = [System.Collections.Generic.List[object]]::new()

  while ($directories.Count -gt 0) {
    $directory = $directories.Pop()
    foreach ($entry in $directory.EnumerateFileSystemInfos()) {
      if (($entry.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "snapshot path contains reparse point: $($entry.FullName)"
      }
      $fullPath = [System.IO.Path]::GetFullPath($entry.FullName)
      if (-not $fullPath.StartsWith($rootPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "snapshot entry escaped root: $fullPath"
      }
      if (($entry.Attributes -band [System.IO.FileAttributes]::Directory) -ne 0) {
        $directories.Push([System.IO.DirectoryInfo]$entry)
        continue
      }
      $relative = $fullPath.Substring($rootPrefix.Length).Replace('\', '/')
      $files.Add([pscustomobject]@{
        RelativePath = $relative
        FullPath = $fullPath
        Item = [System.IO.FileInfo]$entry
      })
    }
  }

  $ordered = @($files)
  [System.Array]::Sort($ordered, [System.Collections.Generic.Comparer[object]]::Create(
    [System.Comparison[object]]{ param($left, $right) [System.StringComparer]::Ordinal.Compare($left.RelativePath, $right.RelativePath) }
  ))
  return $ordered
}

function Assert-SnapshotFilesAllowed {
  param(
    [Parameter(Mandatory = $true)][string]$SourceName,
    [Parameter(Mandatory = $true)][object[]]$Files
  )

  foreach ($file in $Files) {
    $components = $file.RelativePath.Split('/')
    foreach ($component in $components) {
      if ($component.ToLowerInvariant() -in $ForbiddenPathComponents) {
        throw "$SourceName contains forbidden generated/cache path: $($file.RelativePath)"
      }
    }
    $leafName = $components[$components.Length - 1]
    if ($leafName -in $ForbiddenNames) {
      throw "$SourceName contains forbidden toolkit or real-data filename: $($file.RelativePath)"
    }
    foreach ($pattern in $ForbiddenNamePatterns) {
      if ($leafName -match $pattern) {
        throw "$SourceName contains forbidden real-data filename: $($file.RelativePath)"
      }
    }
    $extension = [System.IO.Path]::GetExtension($leafName).ToLowerInvariant()
    if ($extension -in $ForbiddenExtensions) {
      throw "$SourceName contains forbidden binary extension: $($file.RelativePath)"
    }
    $signature = Test-ExecutableSignature -Path $file.FullPath
    if ($signature) {
      throw "$SourceName contains forbidden $signature executable signature: $($file.RelativePath)"
    }
  }
}

function Get-SnapshotManifest {
  param([Parameter(Mandatory = $true)][object[]]$Files)

  $hashedFiles = @($Files | ForEach-Object {
    $file = $_
    [pscustomobject]@{
      RelativePath = $file.RelativePath
      Length = $file.Item.Length
      Hash = (Get-FileHash -LiteralPath $file.FullPath -Algorithm SHA256).Hash.ToLowerInvariant()
      FullPath = $file.FullPath
    }
  })

  $manifestText = [System.Text.StringBuilder]::new()
  foreach ($file in $hashedFiles) {
    [void]$manifestText.Append($file.Hash).Append('  ').Append($file.Length).Append('  ').Append($file.RelativePath).Append([char]10)
  }
  $bytes = $Utf8NoBom.GetBytes($manifestText.ToString())
  $sha256 = [System.Security.Cryptography.SHA256]::Create()
  try {
    $manifestHash = ([System.BitConverter]::ToString($sha256.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant()
  } finally {
    $sha256.Dispose()
  }
  [pscustomobject]@{
    Hash = $manifestHash
    FileCount = $hashedFiles.Count
    SizeBytes = [int64](($hashedFiles | Measure-Object -Property Length -Sum).Sum)
    Files = $hashedFiles
  }
}

function Test-ExecutableSignature {
  param([Parameter(Mandatory = $true)][string]$Path)

  $stream = [System.IO.File]::OpenRead($Path)
  try {
    $header = [byte[]]::new(4)
    $read = $stream.Read($header, 0, $header.Length)
    if ($read -ge 2 -and $header[0] -eq 0x4d -and $header[1] -eq 0x5a) {
      return "PE/MZ"
    }
    if ($read -eq 4 -and $header[0] -eq 0x7f -and $header[1] -eq 0x45 -and $header[2] -eq 0x4c -and $header[3] -eq 0x46) {
      return "ELF"
    }
    return $null
  } finally {
    $stream.Dispose()
  }
}

foreach ($source in $Sources) {
  $snapshotRoot = Resolve-RepositoryPath -RelativePath $source.Snapshot
  $snapshotRootItem = Get-PathWithoutReparsePoint -Path $snapshotRoot -MissingMessage "required snapshot is missing: $($source.Snapshot)"
  if (-not ($snapshotRootItem -is [System.IO.DirectoryInfo])) {
    throw "required snapshot is not a directory: $($source.Snapshot)"
  }

  $provenancePath = Resolve-RepositoryPath -RelativePath $source.Provenance
  $provenanceItem = Get-PathWithoutReparsePoint -Path $provenancePath -MissingMessage "required provenance is missing: $($source.Provenance)"
  if (-not ($provenanceItem -is [System.IO.FileInfo])) {
    throw "required provenance is not a file: $($source.Provenance)"
  }
  $provenanceHash = (Get-FileHash -LiteralPath $provenanceItem.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
  if ($provenanceHash -cne $source.ProvenanceSha256) {
    throw "$($source.Name) provenance file SHA-256 does not match the pinned contract"
  }
  $provenance = Get-Content -LiteralPath $provenanceItem.FullName -Raw -Encoding UTF8 | ConvertFrom-Json
  if ($provenance.repository_url -cne $source.RepositoryUrl) {
    throw "$($source.Name) repository URL does not match the pinned contract"
  }
  if ($provenance.commit -cne $source.Commit) {
    throw "$($source.Name) commit does not match the pinned contract"
  }
  if ($provenance.git_tree_oid -cne $source.GitTreeOid) {
    throw "$($source.Name) Git tree OID does not match the pinned contract"
  }
  if ($provenance.git_tree_manifest_sha256 -cne $source.GitTreeManifestSha256) {
    throw "$($source.Name) Git tree manifest SHA-256 does not match the pinned contract"
  }
  if ($provenance.archive_sha256 -cne $source.ArchiveSha256) {
    throw "$($source.Name) archive SHA-256 does not match the pinned contract"
  }
  if ($provenance.snapshot_manifest_sha256 -cne $source.SnapshotManifestSha256) {
    throw "$($source.Name) snapshot manifest SHA-256 does not match the pinned contract"
  }
  if ([bool]$provenance.standalone_license_file -ne $source.StandaloneLicense) {
    throw "$($source.Name) standalone license caveat changed unexpectedly"
  }
  if ($provenance.expected_license -cne $source.ExpectedLicense) {
    throw "$($source.Name) expected license statement changed unexpectedly"
  }
  if ([string]::IsNullOrWhiteSpace($provenance.retrieval_date) -or $provenance.retrieval_date -notmatch '^\d{4}-\d{2}-\d{2}$') {
    throw "$($source.Name) retrieval date is missing or invalid"
  }
  if (@($provenance.verification_commands).Count -eq 0) {
    throw "$($source.Name) verification commands are missing"
  }
  if (@($provenance.git_mode_exceptions).Count -ne $source.ModeExceptions.Count) {
    throw "$($source.Name) Git mode exception count changed unexpectedly"
  }
  for ($index = 0; $index -lt $source.ModeExceptions.Count; $index++) {
    if ($provenance.git_mode_exceptions[$index] -cne $source.ModeExceptions[$index]) {
      throw "$($source.Name) Git mode exception changed unexpectedly"
    }
  }

  $snapshotFiles = @(Get-SnapshotFilesSafely -SnapshotRoot $snapshotRootItem)
  Assert-SnapshotFilesAllowed -SourceName $source.Name -Files $snapshotFiles
  $snapshotPaths = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
  foreach ($file in $snapshotFiles) { [void]$snapshotPaths.Add($file.RelativePath) }

  foreach ($relative in $source.RequiredFiles) {
    if (-not $snapshotPaths.Contains($relative.Replace('\', '/'))) {
      throw "$($source.Name) required upstream file is missing: $relative"
    }
  }
  if ($source.LicenseFile) {
    $licensePath = Join-Path $snapshotRoot $source.LicenseFile
    $licenseHash = (Get-FileHash -LiteralPath $licensePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($licenseHash -cne $source.LicenseMaterialSha256) {
      throw "$($source.Name) upstream license hash mismatch"
    }
    if (-not (Select-String -LiteralPath $licensePath -Pattern 'MIT License' -Quiet)) {
      throw "$($source.Name) MIT license text is missing"
    }
  } else {
    $standaloneLicenseCandidates = @($snapshotFiles | Where-Object { $_.RelativePath -notmatch '/' -and $_.Item.Name -match '^(LICENSE|LICENCE)(\..*)?$' })
    if ($standaloneLicenseCandidates.Count -ne 0) {
      throw "$($source.Name) unexpectedly contains a standalone upstream license file"
    }
    if (-not (Select-String -LiteralPath (Join-Path $snapshotRoot "README.md") -Pattern $source.LicenseReadmePattern -Quiet)) {
      throw "$($source.Name) README license declaration is missing"
    }
    $readmeHash = (Get-FileHash -LiteralPath (Join-Path $snapshotRoot "README.md") -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($readmeHash -cne $source.LicenseMaterialSha256) {
      throw "$($source.Name) README license material hash mismatch"
    }
  }

  $manifest = Get-SnapshotManifest -Files $snapshotFiles
  if ($manifest.Hash -cne $provenance.snapshot_manifest_sha256) {
    throw "$($source.Name) snapshot manifest SHA-256 mismatch: expected $($provenance.snapshot_manifest_sha256), got $($manifest.Hash)"
  }
  if ($manifest.Hash -cne $source.SnapshotManifestSha256) {
    throw "$($source.Name) snapshot content does not match the verifier contract"
  }
  if ($manifest.FileCount -ne $source.SnapshotFileCount -or $manifest.SizeBytes -ne $source.SnapshotSizeBytes) {
    throw "$($source.Name) snapshot count or size does not match the verifier contract"
  }
  if ($manifest.FileCount -ne [int]$provenance.snapshot_file_count) {
    throw "$($source.Name) snapshot file count mismatch"
  }
  if ($manifest.SizeBytes -ne [int64]$provenance.snapshot_size_bytes) {
    throw "$($source.Name) snapshot size mismatch"
  }

  Write-Host "$($source.Name): $($manifest.FileCount) files, $($manifest.SizeBytes) bytes, manifest $($manifest.Hash)"
}

Write-Host "Pinned UID remapper source verification passed"
