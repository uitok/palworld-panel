param(
  [Parameter(Mandatory = $true)]
  [string]$Archive,
  [string]$ObjdumpPath = ""
)

$ErrorActionPreference = "Stop"
$Archive = (Resolve-Path $Archive).Path
$PackageName = [System.IO.Path]::GetFileNameWithoutExtension($Archive)
$Temp = Join-Path ([System.IO.Path]::GetTempPath()) "palpanel-verify-$([guid]::NewGuid().ToString('N'))"

try {
  Expand-Archive -LiteralPath $Archive -DestinationPath $Temp
  $Package = Join-Path $Temp $PackageName
  if (-not (Test-Path -LiteralPath $Package -PathType Container)) {
    throw "archive package root is missing: $PackageName"
  }

  $required = @(
    "PalPanel.exe",
    "palpanel-server.exe",
    "sav-cli.exe",
    "frontend\dist\index.html",
    "config\palpanel.env.example",
    "LICENSE",
    "THIRD_PARTY_LICENSES.txt",
    "licenses\sav-cli-LICENSE.txt",
    "licenses\pallocalize-Apache-2.0.txt",
    "checksums.txt"
  )
  foreach ($relative in $required) {
    if (-not (Test-Path -LiteralPath (Join-Path $Package $relative) -PathType Leaf)) {
      throw "Windows package is missing $relative"
    }
  }

  foreach ($runtimePath in @("data", "logs", "run", "config\palpanel.env")) {
    if (Test-Path -LiteralPath (Join-Path $Package $runtimePath)) {
      throw "Windows package contains runtime path $runtimePath"
    }
  }

  $checksumPath = Join-Path $Package "checksums.txt"
  $checksumLines = Get-Content -LiteralPath $checksumPath
  $verified = 0
  foreach ($line in $checksumLines) {
    if ($line -notmatch '^([0-9a-fA-F]{64})  \./(.+)$') {
      throw "invalid checksum line: $line"
    }
    $expected = $Matches[1].ToLowerInvariant()
    $relative = $Matches[2].Replace('/', [System.IO.Path]::DirectorySeparatorChar)
    $path = Join-Path $Package $relative
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
      throw "checksummed file is missing: $relative"
    }
    $actual = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected) {
      throw "checksum mismatch: $relative"
    }
    $verified++
  }
  $packagedFiles = @(Get-ChildItem -LiteralPath $Package -Recurse -File | Where-Object { $_.Name -ne "checksums.txt" })
  if ($verified -ne $packagedFiles.Count) {
    throw "checksums.txt covers $verified files, package contains $($packagedFiles.Count) files"
  }

  if ($ObjdumpPath) {
    if (-not (Test-Path -LiteralPath $ObjdumpPath -PathType Leaf)) {
      throw "objdump is missing: $ObjdumpPath"
    }
    $imports = (& $ObjdumpPath -p (Join-Path $Package "sav-cli.exe") | Out-String)
    if ($LASTEXITCODE -ne 0) {
      throw "objdump failed with exit code $LASTEXITCODE"
    }
    if ($imports -match '(?i)libgcc_s[^\s]*\.dll|libstdc\+\+-6\.dll|libwinpthread-1\.dll') {
      throw "sav-cli.exe depends on a MinGW runtime DLL"
    }
  }

  Write-Host "Windows package content verification passed"
} finally {
  Remove-Item -LiteralPath $Temp -Recurse -Force -ErrorAction SilentlyContinue
}
