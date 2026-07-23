[CmdletBinding()]
param(
  [string]$OutputPath = "",
  [string]$MingwGcc = "C:\msys64\mingw64\bin\gcc.exe",
  [string]$MingwGxx = "C:\msys64\mingw64\bin\g++.exe",
  [switch]$VerifyOnly
)

$ErrorActionPreference = "Stop"
$RootDir = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($OutputPath)) {
  $OutputPath = Join-Path $RootDir "output\dev\sav-cli.exe"
}
$OutputPath = [System.IO.Path]::GetFullPath($OutputPath)

if (-not $VerifyOnly) {
  if (-not (Test-Path -LiteralPath $MingwGcc -PathType Leaf)) {
    throw "MinGW gcc not found: $MingwGcc"
  }
  if (-not (Test-Path -LiteralPath $MingwGxx -PathType Leaf)) {
    throw "MinGW g++ not found: $MingwGxx"
  }

  $outputDir = Split-Path -Parent $OutputPath
  New-Item -ItemType Directory -Force -Path $outputDir | Out-Null

  $oldGoos = $env:GOOS
  $oldGoarch = $env:GOARCH
  $oldCgo = $env:CGO_ENABLED
  $oldCc = $env:CC
  $oldCxx = $env:CXX
  $oldPath = $env:PATH
  try {
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    $env:CGO_ENABLED = "1"
    $env:CC = $MingwGcc
    $env:CXX = $MingwGxx
    $env:PATH = (Split-Path -Parent $MingwGcc) + [System.IO.Path]::PathSeparator + $oldPath

    Push-Location (Join-Path $RootDir "sav-cli")
    try {
      & go build -trimpath -o $OutputPath ./cmd/sav_cli
      if ($LASTEXITCODE -ne 0) {
        throw "sav-cli development build failed with exit code $LASTEXITCODE"
      }
    } finally {
      Pop-Location
    }
  } finally {
    $env:GOOS = $oldGoos
    $env:GOARCH = $oldGoarch
    $env:CGO_ENABLED = $oldCgo
    $env:CC = $oldCc
    $env:CXX = $oldCxx
    $env:PATH = $oldPath
  }
}

if (-not (Test-Path -LiteralPath $OutputPath -PathType Leaf)) {
  throw "sav-cli executable not found: $OutputPath"
}

& $OutputPath verify-build --require-oodle
if ($LASTEXITCODE -ne 0) {
  if (-not $VerifyOnly) {
    Remove-Item -LiteralPath $OutputPath -Force -ErrorAction SilentlyContinue
  }
  throw "sav-cli build self-check failed: the executable must report oodle=true"
}

Write-Host "sav-cli development build verified: $OutputPath"
