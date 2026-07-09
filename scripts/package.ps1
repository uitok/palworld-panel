param(
  [string]$Version = "",
  [string]$Targets = "linux-amd64,windows-amd64",
  [switch]$SkipTests,
  [switch]$Clean
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = Split-Path -Parent $ScriptDir
$PackagesDir = Join-Path $RootDir "dist\packages"
$StagingDir = Join-Path $PackagesDir "staging"

function Invoke-External {
  param(
    [string]$FilePath,
    [string[]]$Arguments,
    [string]$WorkingDirectory
  )
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

function Get-PackageVersion {
  if ($Version) {
    return $Version
  }
  $gitVersion = & git -C $RootDir describe --tags --always --dirty 2>$null
  if ($LASTEXITCODE -eq 0 -and $gitVersion) {
    return $gitVersion.Trim()
  }
  return (Get-Date).ToUniversalTime().ToString("yyyyMMddHHmmss")
}

function Copy-CommonFiles {
  param([string]$PackageDir)

  New-Item -ItemType Directory -Force -Path (Join-Path $PackageDir "bin") | Out-Null
  New-Item -ItemType Directory -Force -Path (Join-Path $PackageDir "config") | Out-Null
  New-Item -ItemType Directory -Force -Path (Join-Path $PackageDir "scripts") | Out-Null
  New-Item -ItemType Directory -Force -Path (Join-Path $PackageDir "frontend") | Out-Null
  New-Item -ItemType Directory -Force -Path (Join-Path $PackageDir "backend\deployments") | Out-Null

  Copy-Item -Recurse -Force -LiteralPath (Join-Path $RootDir "frontend\dist") -Destination (Join-Path $PackageDir "frontend\dist")
  Copy-Item -Recurse -Force -LiteralPath (Join-Path $RootDir "backend\deployments\wine-runner") -Destination (Join-Path $PackageDir "backend\deployments\wine-runner")
  Copy-Item -Force -LiteralPath (Join-Path $RootDir "scripts\palpanel.env.example") -Destination (Join-Path $PackageDir "config\palpanel.env.example")
  Copy-Item -Force -LiteralPath (Join-Path $RootDir "scripts\package-README.md") -Destination (Join-Path $PackageDir "README.md")
  Copy-Item -Force -LiteralPath (Join-Path $RootDir "scripts\start.sh") -Destination (Join-Path $PackageDir "scripts\start.sh")
  Copy-Item -Force -LiteralPath (Join-Path $RootDir "scripts\start.ps1") -Destination (Join-Path $PackageDir "scripts\start.ps1")
}

function Write-Checksums {
  param([string]$PackageDir)

  $basePath = (Resolve-Path -LiteralPath $PackageDir).Path
  $baseUri = [Uri]($basePath.TrimEnd([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar)
  $lines = Get-ChildItem -LiteralPath $PackageDir -Recurse -File |
    Where-Object { $_.Name -ne "checksums.txt" } |
    Sort-Object FullName |
    ForEach-Object {
      $relative = [Uri]::UnescapeDataString($baseUri.MakeRelativeUri([Uri]$_.FullName).ToString())
      $hash = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
      "$hash  ./$relative"
    }
  Set-Content -LiteralPath (Join-Path $PackageDir "checksums.txt") -Value $lines -Encoding ASCII
}

function Build-Target {
  param(
    [string]$Target,
    [string]$PackageVersion
  )

  $parts = $Target.Split("-", 2)
  if ($parts.Length -ne 2) {
    throw "Invalid target: $Target"
  }
  $goos = $parts[0]
  $goarch = $parts[1]
  $exeExt = if ($goos -eq "windows") { ".exe" } else { "" }
  $packageName = "palpanel_${PackageVersion}_${goos}_${goarch}"
  $packageDir = Join-Path $StagingDir $packageName

  if (Test-Path $packageDir) {
    Remove-Item -Recurse -Force $packageDir
  }
  Copy-CommonFiles -PackageDir $packageDir

  Write-Host "[palpanel] Building backend for $Target"
  $oldGoos = $env:GOOS
  $oldGoarch = $env:GOARCH
  $oldCgo = $env:CGO_ENABLED
  try {
    $env:GOOS = $goos
    $env:GOARCH = $goarch
    $env:CGO_ENABLED = "0"
    Invoke-External -FilePath "go" -Arguments @("build", "-trimpath", "-o", (Join-Path $packageDir "bin\palpanel$exeExt"), "./cmd/palpanel") -WorkingDirectory (Join-Path $RootDir "backend")
    Write-Host "[palpanel] Building sav-cli for $Target"
    Invoke-External -FilePath "go" -Arguments @("build", "-trimpath", "-o", (Join-Path $packageDir "bin\sav-cli$exeExt"), "./cmd/sav_cli") -WorkingDirectory (Join-Path $RootDir "sav-cli")
  } finally {
    $env:GOOS = $oldGoos
    $env:GOARCH = $oldGoarch
    $env:CGO_ENABLED = $oldCgo
  }

  Write-Checksums -PackageDir $packageDir

  if ($goos -eq "windows") {
    $archive = Join-Path $PackagesDir "$packageName.zip"
    if (Test-Path $archive) {
      Remove-Item -Force $archive
    }
    Compress-Archive -Path $packageDir -DestinationPath $archive -Force
  } else {
    $archive = Join-Path $PackagesDir "$packageName.tar.gz"
    if (Test-Path $archive) {
      Remove-Item -Force $archive
    }
    Invoke-External -FilePath "tar" -Arguments @("-czf", $archive, "-C", $StagingDir, $packageName) -WorkingDirectory $RootDir
  }

  Write-Host "[palpanel] Wrote $archive"
}

if ($Clean -and (Test-Path $PackagesDir)) {
  Remove-Item -Recurse -Force $PackagesDir
}
New-Item -ItemType Directory -Force -Path $PackagesDir | Out-Null
New-Item -ItemType Directory -Force -Path $StagingDir | Out-Null

$packageVersion = (Get-PackageVersion) -replace "[/ ]", "-"

if (-not $SkipTests) {
  Write-Host "[palpanel] Running backend tests"
  Invoke-External -FilePath "go" -Arguments @("test", "./...") -WorkingDirectory (Join-Path $RootDir "backend")
  Write-Host "[palpanel] Running sav-cli tests"
  Invoke-External -FilePath "go" -Arguments @("test", "./...") -WorkingDirectory (Join-Path $RootDir "sav-cli")
  Write-Host "[palpanel] Installing frontend dependencies"
  Invoke-External -FilePath "npm" -Arguments @("ci") -WorkingDirectory (Join-Path $RootDir "frontend")
  Write-Host "[palpanel] Running frontend check"
  Invoke-External -FilePath "npm" -Arguments @("run", "check") -WorkingDirectory (Join-Path $RootDir "frontend")
} else {
  Write-Host "[palpanel] Skipping tests and checks"
  Write-Host "[palpanel] Installing frontend dependencies"
  Invoke-External -FilePath "npm" -Arguments @("ci") -WorkingDirectory (Join-Path $RootDir "frontend")
  Write-Host "[palpanel] Building frontend"
  Invoke-External -FilePath "npm" -Arguments @("run", "build") -WorkingDirectory (Join-Path $RootDir "frontend")
}

$Targets.Split(",") |
  ForEach-Object { $_.Trim() } |
  Where-Object { $_ } |
  ForEach-Object { Build-Target -Target $_ -PackageVersion $packageVersion }

if (Test-Path $StagingDir) {
  Remove-Item -Recurse -Force $StagingDir
}
