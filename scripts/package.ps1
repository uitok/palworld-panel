param(
  [string]$Version = "v0.0.0-dev",
  [switch]$SkipTests,
  [switch]$Clean,
  [string]$MingwGcc = "C:\msys64\mingw64\bin\gcc.exe"
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = Split-Path -Parent $ScriptDir
$PackagesDir = Join-Path $RootDir "dist\packages"
$PackageName = "palpanel_${Version}_windows_amd64"
$PackageDir = Join-Path $PackagesDir $PackageName
$Archive = Join-Path $PackagesDir "$PackageName.zip"
$WebUIEmbedDir = Join-Path $RootDir "backend\internal\webui\embedded"
$Commit = (& git -C $RootDir rev-parse HEAD).Trim()
$BuildTime = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")

function Invoke-External {
  param([string]$FilePath, [string[]]$Arguments, [string]$WorkingDirectory)
  Push-Location $WorkingDirectory
  try {
    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
      throw "$FilePath $($Arguments -join ' ') failed with exit code $LASTEXITCODE"
    }
  } finally {
    Pop-Location
  }
}

function Clear-WebUIStage {
  if (-not (Test-Path $WebUIEmbedDir)) { return }
  Get-ChildItem -LiteralPath $WebUIEmbedDir -Force |
    Where-Object { $_.Name -ne ".keep" } |
    Remove-Item -Recurse -Force
}

if ($Clean -and (Test-Path $PackageDir)) {
  Remove-Item -Recurse -Force $PackageDir
}
if (Test-Path $Archive) {
  Remove-Item -Force $Archive
}
New-Item -ItemType Directory -Force -Path $PackageDir | Out-Null

if (-not $SkipTests) {
  Invoke-External "go" @("test", "-p=1", "./...") (Join-Path $RootDir "backend")
  if (-not (Test-Path $MingwGcc)) {
    throw "MinGW gcc not found: $MingwGcc"
  }
  $oldCgo = $env:CGO_ENABLED
  $oldCc = $env:CC
  try {
    $env:CGO_ENABLED = "1"
    $env:CC = $MingwGcc
    Invoke-External "go" @("test", "-p=1", "./...") (Join-Path $RootDir "sav-cli")
  } finally {
    $env:CGO_ENABLED = $oldCgo
    $env:CC = $oldCc
  }
  Invoke-External "npm.cmd" @("ci") (Join-Path $RootDir "frontend")
  Invoke-External "npm.cmd" @("run", "check") (Join-Path $RootDir "frontend")
} else {
  Invoke-External "npm.cmd" @("ci") (Join-Path $RootDir "frontend")
  Invoke-External "npm.cmd" @("run", "build") (Join-Path $RootDir "frontend")
}

New-Item -ItemType Directory -Force -Path (Join-Path $PackageDir "backend\deployments") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $PackageDir "config") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $PackageDir "licenses") | Out-Null
Copy-Item -Recurse -Force (Join-Path $RootDir "backend\deployments\wine-runner") (Join-Path $PackageDir "backend\deployments\wine-runner")
Copy-Item -Force (Join-Path $RootDir "scripts\palpanel.env.example") (Join-Path $PackageDir "config\palpanel.env.example")
Copy-Item -Force (Join-Path $RootDir "scripts\package-README-windows.md") (Join-Path $PackageDir "README.md")
Copy-Item -Force (Join-Path $RootDir "LICENSE") (Join-Path $PackageDir "LICENSE")
Copy-Item -Force (Join-Path $RootDir "THIRD_PARTY_LICENSES.txt") (Join-Path $PackageDir "THIRD_PARTY_LICENSES.txt")
Copy-Item -Force (Join-Path $RootDir "sav-cli\LICENSE") (Join-Path $PackageDir "licenses\sav-cli-LICENSE.txt")
Copy-Item -Force (Join-Path $RootDir "backend\internal\pallocalize\LICENSE.apache-2.0") (Join-Path $PackageDir "licenses\pallocalize-Apache-2.0.txt")
Copy-Item -Force (Join-Path $RootDir "backend\internal\paldefender\assets\LICENSE.txt") (Join-Path $PackageDir "licenses\PalDefender-MIT.txt")

$backendLdflags = "-s -w -X palpanel/internal/buildinfo.Version=$Version -X palpanel/internal/buildinfo.Commit=$Commit -X palpanel/internal/buildinfo.BuildTime=$BuildTime"
$savLdflags = "-s -w -X palpanel/sav-cli/internal/buildinfo.Version=$Version -X palpanel/sav-cli/internal/buildinfo.Commit=$Commit -X palpanel/sav-cli/internal/buildinfo.BuildTime=$BuildTime"
$oldGoos = $env:GOOS
$oldGoarch = $env:GOARCH
$oldCgo = $env:CGO_ENABLED
$oldCc = $env:CC
Clear-WebUIStage
New-Item -ItemType Directory -Force -Path $WebUIEmbedDir | Out-Null
Copy-Item -Recurse -Force (Join-Path $RootDir "frontend\dist\*") $WebUIEmbedDir
try {
  $env:GOOS = "windows"
  $env:GOARCH = "amd64"
  $env:CGO_ENABLED = "0"
  Invoke-External "go" @("build", "-tags", "embed_webui", "-trimpath", "-ldflags", $backendLdflags, "-o", (Join-Path $PackageDir "palpanel-server.exe"), "./cmd/palpanel") (Join-Path $RootDir "backend")
  Invoke-External "go" @("build", "-trimpath", "-ldflags", "$backendLdflags -H windowsgui", "-o", (Join-Path $PackageDir "PalPanel.exe"), "./cmd/palpanel-launcher") (Join-Path $RootDir "backend")

  if (-not (Test-Path $MingwGcc)) {
    throw "MinGW gcc not found: $MingwGcc"
  }
  $env:CGO_ENABLED = "1"
  $env:CC = $MingwGcc
  Invoke-External "go" @("build", "-trimpath", "-ldflags", $savLdflags, "-o", (Join-Path $PackageDir "sav-cli.exe"), "./cmd/sav_cli") (Join-Path $RootDir "sav-cli")
} finally {
  $env:GOOS = $oldGoos
  $env:GOARCH = $oldGoarch
  $env:CGO_ENABLED = $oldCgo
  $env:CC = $oldCc
  Clear-WebUIStage
}

$checksumLines = Get-ChildItem -LiteralPath $PackageDir -Recurse -File |
  Where-Object { $_.Name -ne "checksums.txt" } |
  Sort-Object FullName |
  ForEach-Object {
    $relative = $_.FullName.Substring($PackageDir.Length).TrimStart("\") -replace "\\", "/"
    $hash = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
    "$hash  ./$relative"
  }
Set-Content -LiteralPath (Join-Path $PackageDir "checksums.txt") -Value $checksumLines -Encoding ASCII
Compress-Archive -Path $PackageDir -DestinationPath $Archive -Force
Write-Host "[palpanel] Wrote unsigned Windows release package $Archive"
