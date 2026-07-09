$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$PackageDir = Split-Path -Parent $ScriptDir
$EnvFile = Join-Path $PackageDir "config\palpanel.env"
$ExampleFile = Join-Path $PackageDir "config\palpanel.env.example"

function New-PanelToken {
  $bytes = New-Object byte[] 32
  $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
  try {
    $rng.GetBytes($bytes)
  } finally {
    $rng.Dispose()
  }
  -join ($bytes | ForEach-Object { $_.ToString("x2") })
}

function Set-EnvFileValue {
  param(
    [string]$Path,
    [string]$Name,
    [string]$Value
  )
  $line = "$Name=$Value"
  if (Test-Path $Path) {
    $content = Get-Content -LiteralPath $Path
    $updated = $false
    $next = foreach ($item in $content) {
      if ($item -match "^$([regex]::Escape($Name))=") {
        $updated = $true
        $line
      } else {
        $item
      }
    }
    if (-not $updated) {
      $next += $line
    }
    Set-Content -LiteralPath $Path -Value $next -Encoding ASCII
  } else {
    Set-Content -LiteralPath $Path -Value $line -Encoding ASCII
  }
}

if (-not (Test-Path $EnvFile)) {
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $EnvFile) | Out-Null
  if (Test-Path $ExampleFile) {
    Copy-Item -LiteralPath $ExampleFile -Destination $EnvFile
  } else {
    New-Item -ItemType File -Force -Path $EnvFile | Out-Null
  }
  $token = New-PanelToken
  Set-EnvFileValue -Path $EnvFile -Name "PANEL_TOKEN" -Value $token
  Write-Host "[palpanel] Created config\palpanel.env"
  Write-Host "[palpanel] PANEL_TOKEN=$token"
}

Get-Content -LiteralPath $EnvFile | ForEach-Object {
  $line = $_.Trim()
  if ($line -and -not $line.StartsWith("#")) {
    $parts = $line.Split("=", 2)
    if ($parts.Length -eq 2) {
      [Environment]::SetEnvironmentVariable($parts[0].Trim(), $parts[1].Trim(), "Process")
    }
  }
}

if (-not $env:PALPANEL_FRONTEND_DIST) {
  $env:PALPANEL_FRONTEND_DIST = Join-Path $PackageDir "frontend\dist"
}
if (-not $env:PALPANEL_BACKEND_DIR) {
  $env:PALPANEL_BACKEND_DIR = Join-Path $PackageDir "backend"
}
if (-not $env:PALPANEL_DATA_DIR) {
  $env:PALPANEL_DATA_DIR = Join-Path $PackageDir "data"
}
if (-not $env:PALPANEL_RUNNER_DIR) {
  $env:PALPANEL_RUNNER_DIR = Join-Path $PackageDir "backend\deployments\wine-runner"
}

New-Item -ItemType Directory -Force -Path $env:PALPANEL_DATA_DIR | Out-Null

$listen = if ($env:PALPANEL_LISTEN_ADDR) { $env:PALPANEL_LISTEN_ADDR } else { ":8080" }
$displayPort = "8080"
if ($listen -match ":(\d+)$") {
  $displayPort = $Matches[1]
}

Write-Host "[palpanel] Frontend: http://127.0.0.1:$displayPort/dashboard"
if ($env:PANEL_TOKEN) {
  Write-Host "[palpanel] Token: $env:PANEL_TOKEN"
}

& (Join-Path $PackageDir "bin\palpanel.exe")
