param(
  [ValidateSet("All", "FreshCheckout", "Provenance", "Reparse", "ForbiddenContent")]
  [string]$Case = "All",
  [switch]$KeepArtifacts
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$RepositoryRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$CacheRoot = [System.IO.Path]::GetFullPath((Join-Path $RepositoryRoot ".cache\task-5a\source-verifier-tests"))
$RunRoot = Join-Path $CacheRoot ([guid]::NewGuid().ToString("N"))
$Utf8NoBom = [System.Text.UTF8Encoding]::new($false)
$PowerShellExe = Join-Path $PSHOME "powershell.exe"

function Assert-PathWithin {
  param(
    [Parameter(Mandatory = $true)][string]$Root,
    [Parameter(Mandatory = $true)][string]$Path,
    [switch]$AllowRoot
  )

  $resolvedRoot = [System.IO.Path]::GetFullPath($Root).TrimEnd([System.IO.Path]::DirectorySeparatorChar)
  $resolvedPath = [System.IO.Path]::GetFullPath($Path).TrimEnd([System.IO.Path]::DirectorySeparatorChar)
  if ($AllowRoot -and [string]::Equals($resolvedRoot, $resolvedPath, [System.StringComparison]::OrdinalIgnoreCase)) {
    return $resolvedPath
  }
  $prefix = $resolvedRoot + [System.IO.Path]::DirectorySeparatorChar
  if (-not $resolvedPath.StartsWith($prefix, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "test path escaped cache root: $resolvedPath"
  }
  return $resolvedPath
}

function Copy-VerifierFixture {
  param([Parameter(Mandatory = $true)][string]$Destination)

  $destinationRoot = Assert-PathWithin -Root $RunRoot -Path $Destination
  New-Item -ItemType Directory -Path (Join-Path $destinationRoot "scripts") -Force | Out-Null
  New-Item -ItemType Directory -Path (Join-Path $destinationRoot "third_party") -Force | Out-Null
  Copy-Item -LiteralPath (Join-Path $RepositoryRoot "scripts\verify-uid-remapper-sources.ps1") -Destination (Join-Path $destinationRoot "scripts")
  Copy-Item -LiteralPath (Join-Path $RepositoryRoot "third_party\palworld-save-pal.provenance.json") -Destination (Join-Path $destinationRoot "third_party")
  Copy-Item -LiteralPath (Join-Path $RepositoryRoot "third_party\uesave.provenance.json") -Destination (Join-Path $destinationRoot "third_party")
  Copy-Item -LiteralPath (Join-Path $RepositoryRoot "third_party\palworld-save-pal") -Destination (Join-Path $destinationRoot "third_party") -Recurse
  Copy-Item -LiteralPath (Join-Path $RepositoryRoot "third_party\uesave") -Destination (Join-Path $destinationRoot "third_party") -Recurse
  $attributes = Join-Path $RepositoryRoot "third_party\.gitattributes"
  if (Test-Path -LiteralPath $attributes -PathType Leaf) {
    Copy-Item -LiteralPath $attributes -Destination (Join-Path $destinationRoot "third_party")
  }
  return $destinationRoot
}

function Invoke-FixtureVerifier {
  param([Parameter(Mandatory = $true)][string]$FixtureRoot)

  $verifier = Join-Path $FixtureRoot "scripts\verify-uid-remapper-sources.ps1"
  $previousErrorAction = $ErrorActionPreference
  try {
    $ErrorActionPreference = "Continue"
    $output = (& $PowerShellExe -NoProfile -ExecutionPolicy Bypass -File $verifier 2>&1 | Out-String).Trim()
  } finally {
    $ErrorActionPreference = $previousErrorAction
  }
  [pscustomobject]@{
    ExitCode = $LASTEXITCODE
    Output = $output
  }
}

function Assert-VerifierRejected {
  param(
    [Parameter(Mandatory = $true)]$Result,
    [Parameter(Mandatory = $true)][string]$ExpectedPattern,
    [Parameter(Mandatory = $true)][string]$Scenario
  )

  if ($Result.ExitCode -eq 0) {
    throw "$Scenario was accepted unexpectedly"
  }
  if ($Result.Output -notmatch $ExpectedPattern) {
    throw "$Scenario failed for the wrong reason: $($Result.Output)"
  }
  Write-Host "$Scenario rejected as expected"
}

function Test-FreshCheckout {
  $staging = Copy-VerifierFixture -Destination (Join-Path $RunRoot "fresh-staging")
  & git -C $staging init --quiet
  if ($LASTEXITCODE -ne 0) { throw "git init failed" }
  & git -C $staging config user.name "Task 5A verifier test"
  & git -C $staging config user.email "task-5a@example.invalid"
  & git -C $staging config core.autocrlf false
  # Vendored tracked fixtures may match their upstream .gitignore rules.
  & git -C $staging add --all --force
  if ($LASTEXITCODE -ne 0) { throw "git add failed" }
  & git -C $staging commit --quiet -m "source verifier fixture"
  if ($LASTEXITCODE -ne 0) { throw "git commit failed" }

  $checkout = Assert-PathWithin -Root $RunRoot -Path (Join-Path $RunRoot "fresh-checkout")
  & git -c core.autocrlf=true clone --quiet --no-local $staging $checkout
  if ($LASTEXITCODE -ne 0) { throw "fresh clone failed" }
  & git -C $checkout config core.autocrlf true
  & git -C $checkout reset --hard --quiet HEAD
  if ($LASTEXITCODE -ne 0) { throw "fresh checkout reset failed" }

  $attributePaths = @(
    "third_party/palworld-save-pal/README.md",
    "third_party/uesave/LICENSE",
    "third_party/palworld-save-pal.provenance.json",
    "third_party/uesave.provenance.json"
  )
  $attributeOutput = @(& git -C $checkout check-attr text filter ident -- $attributePaths)
  if ($LASTEXITCODE -ne 0 -or $attributeOutput.Count -ne ($attributePaths.Count * 3)) {
    throw "could not inspect fresh-checkout Git attributes"
  }
  foreach ($line in $attributeOutput) {
    if ($line -notmatch ': (text|filter|ident): unset$') {
      throw "fresh checkout permits a Git content transformation: $line"
    }
  }

  $result = Invoke-FixtureVerifier -FixtureRoot $checkout
  if ($result.ExitCode -ne 0) {
    throw "core.autocrlf=true fresh checkout failed verification: $($result.Output)"
  }
  Write-Host "core.autocrlf=true fresh checkout verified"
}

function Test-ProvenanceTamper {
  $fixture = Copy-VerifierFixture -Destination (Join-Path $RunRoot "provenance")
  $path = Join-Path $fixture "third_party\palworld-save-pal.provenance.json"
  $content = [System.IO.File]::ReadAllText($path, $Utf8NoBom)
  $changed = $content.Replace('"format_version": 1', '"format_version": 2')
  if ($changed -ceq $content) { throw "could not mutate unchecked provenance field" }
  [System.IO.File]::WriteAllText($path, $changed, $Utf8NoBom)
  $result = Invoke-FixtureVerifier -FixtureRoot $fixture
  Assert-VerifierRejected -Result $result -ExpectedPattern 'provenance file SHA-256 does not match' -Scenario "previously unchecked provenance field"
}

function Test-ReparseRejection {
  $fixture = Copy-VerifierFixture -Destination (Join-Path $RunRoot "reparse")
  $target = Assert-PathWithin -Root $RunRoot -Path (Join-Path $RunRoot "reparse-target")
  New-Item -ItemType Directory -Path $target | Out-Null
  $lockedPath = Join-Path $target "must-not-be-read.bin"
  [System.IO.File]::WriteAllBytes($lockedPath, [byte[]](0x4d, 0x5a, 0x00, 0x00))
  $stream = [System.IO.File]::Open($lockedPath, [System.IO.FileMode]::Open, [System.IO.FileAccess]::ReadWrite, [System.IO.FileShare]::None)
  $junction = Join-Path $fixture "third_party\palworld-save-pal\reparse-probe"
  try {
    New-Item -ItemType Junction -Path $junction -Target $target | Out-Null
    $result = Invoke-FixtureVerifier -FixtureRoot $fixture
    Assert-VerifierRejected -Result $result -ExpectedPattern 'snapshot path contains reparse point' -Scenario "snapshot directory reparse point"
  } finally {
    $stream.Dispose()
    if (Test-Path -LiteralPath $junction) {
      [System.IO.Directory]::Delete($junction)
    }
  }

  $thirdParty = Join-Path $fixture "third_party"
  $chainTarget = Assert-PathWithin -Root $RunRoot -Path (Join-Path $RunRoot "reparse-chain-target")
  Move-Item -LiteralPath $thirdParty -Destination $chainTarget
  $lockedProvenance = Join-Path $chainTarget "palworld-save-pal.provenance.json"
  $stream = [System.IO.File]::Open($lockedProvenance, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, [System.IO.FileShare]::None)
  try {
    New-Item -ItemType Junction -Path $thirdParty -Target $chainTarget | Out-Null
    $result = Invoke-FixtureVerifier -FixtureRoot $fixture
    Assert-VerifierRejected -Result $result -ExpectedPattern 'path contains reparse point' -Scenario "provenance path-chain reparse point"
  } finally {
    $stream.Dispose()
    if (Test-Path -LiteralPath $thirdParty) {
      [System.IO.Directory]::Delete($thirdParty)
    }
  }
}

function Test-ForbiddenContent {
  $fixture = Copy-VerifierFixture -Destination (Join-Path $RunRoot "forbidden")
  $snapshot = Join-Path $fixture "third_party\palworld-save-pal"

  $sensitive = Join-Path $snapshot "player_uid_mapping.json"
  try {
    [System.IO.File]::WriteAllText($sensitive, "{}", $Utf8NoBom)
    $result = Invoke-FixtureVerifier -FixtureRoot $fixture
    Assert-VerifierRejected -Result $result -ExpectedPattern 'contains forbidden real-data filename' -Scenario "sensitive mapping filename"
  } finally {
    if (Test-Path -LiteralPath $sensitive) { [System.IO.File]::Delete($sensitive) }
  }

  $signature = Join-Path $snapshot "signature-probe.txt"
  try {
    [System.IO.File]::WriteAllBytes($signature, [byte[]](0x4d, 0x5a, 0x00, 0x00))
    $result = Invoke-FixtureVerifier -FixtureRoot $fixture
    Assert-VerifierRejected -Result $result -ExpectedPattern 'contains forbidden PE/MZ executable signature' -Scenario "PE signature with benign extension"
  } finally {
    if (Test-Path -LiteralPath $signature) { [System.IO.File]::Delete($signature) }
  }

  try {
    [System.IO.File]::WriteAllBytes($signature, [byte[]](0x7f, 0x45, 0x4c, 0x46))
    $result = Invoke-FixtureVerifier -FixtureRoot $fixture
    Assert-VerifierRejected -Result $result -ExpectedPattern 'contains forbidden ELF executable signature' -Scenario "ELF signature with benign extension"
  } finally {
    if (Test-Path -LiteralPath $signature) { [System.IO.File]::Delete($signature) }
  }
}

$RunRoot = Assert-PathWithin -Root $CacheRoot -Path $RunRoot
New-Item -ItemType Directory -Path $RunRoot -Force | Out-Null
try {
  if ($Case -in @("All", "FreshCheckout")) { Test-FreshCheckout }
  if ($Case -in @("All", "Provenance")) { Test-ProvenanceTamper }
  if ($Case -in @("All", "Reparse")) { Test-ReparseRejection }
  if ($Case -in @("All", "ForbiddenContent")) { Test-ForbiddenContent }
  Write-Host "UID remapper source verifier tests passed: $Case"
} finally {
  if ($KeepArtifacts) {
    Write-Warning "Test artifacts retained at $RunRoot"
  } elseif (Test-Path -LiteralPath $RunRoot) {
    $validated = Assert-PathWithin -Root $CacheRoot -Path $RunRoot
    Remove-Item -LiteralPath $validated -Recurse -Force
  }
}
