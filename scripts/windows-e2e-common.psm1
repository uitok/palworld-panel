Set-StrictMode -Version Latest

function Get-PalPanelRepositoryRoot {
  $root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
  foreach ($relative in @(".git", "backend\go.mod", "frontend\package.json", "sav-cli\go.mod")) {
    if (-not (Test-Path -LiteralPath (Join-Path $root $relative))) {
      throw "repository marker is missing: $relative"
    }
  }
  return $root
}

function Resolve-PalPanelPath {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Path,
    [Parameter(Mandatory = $true)]
    [string]$BasePath
  )

  if ([string]::IsNullOrWhiteSpace($Path)) {
    throw "path must not be empty"
  }
  $candidate = $Path.Trim()
  if (-not [System.IO.Path]::IsPathRooted($candidate)) {
    $candidate = Join-Path $BasePath $candidate
  }
  return [System.IO.Path]::GetFullPath($candidate)
}

function Test-PalPanelPathWithin {
  param(
    [Parameter(Mandatory = $true)][string]$Root,
    [Parameter(Mandatory = $true)][string]$Target
  )

  $rootFull = [System.IO.Path]::GetFullPath($Root).TrimEnd('\', '/')
  $targetFull = [System.IO.Path]::GetFullPath($Target).TrimEnd('\', '/')
  if ([string]::Equals($rootFull, $targetFull, [System.StringComparison]::OrdinalIgnoreCase)) {
    return $true
  }
  return $targetFull.StartsWith(
    $rootFull + [System.IO.Path]::DirectorySeparatorChar,
    [System.StringComparison]::OrdinalIgnoreCase
  )
}

function Assert-NoPalPanelReparsePoint {
  param(
    [Parameter(Mandatory = $true)][string]$RepositoryRoot,
    [Parameter(Mandatory = $true)][string]$TargetPath
  )

  $repository = [System.IO.Path]::GetFullPath($RepositoryRoot).TrimEnd('\', '/')
  $current = [System.IO.Path]::GetFullPath($TargetPath).TrimEnd('\', '/')
  while (-not (Test-Path -LiteralPath $current)) {
    $parent = Split-Path -Parent $current
    if ([string]::IsNullOrEmpty($parent) -or $parent -eq $current) {
      return
    }
    $current = $parent
  }

  while (Test-PalPanelPathWithin -Root $repository -Target $current) {
    if ([string]::Equals($repository, $current, [System.StringComparison]::OrdinalIgnoreCase)) {
      return
    }
    $item = Get-Item -LiteralPath $current -Force
    if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
      throw "managed path contains a reparse point: $current"
    }
    $parent = Split-Path -Parent $current
    if ([string]::IsNullOrEmpty($parent) -or $parent -eq $current) {
      return
    }
    $current = $parent
  }
}

function Assert-PalPanelManagedPath {
  param(
    [Parameter(Mandatory = $true)][string]$RepositoryRoot,
    [Parameter(Mandatory = $true)][string]$TargetPath,
    [switch]$AllowManagedRoot
  )

  $repository = [System.IO.Path]::GetFullPath($RepositoryRoot).TrimEnd('\', '/')
  $target = [System.IO.Path]::GetFullPath($TargetPath).TrimEnd('\', '/')
  $managedRoot = [System.IO.Path]::GetFullPath((Join-Path $repository "dev-runtime\windows")).TrimEnd('\', '/')
  $volumeRoot = [System.IO.Path]::GetPathRoot($target).TrimEnd('\', '/')

  if ([string]::Equals($target, $repository, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "managed path must not equal the repository root: $target"
  }
  if ([string]::Equals($target, $volumeRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "managed path must not equal a volume root: $target"
  }
  if (-not (Test-PalPanelPathWithin -Root $managedRoot -Target $target)) {
    throw "managed path must stay under $managedRoot`: $target"
  }
  if (-not $AllowManagedRoot -and [string]::Equals($target, $managedRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "managed target must not equal the Windows runtime root: $target"
  }
  Assert-NoPalPanelReparsePoint -RepositoryRoot $repository -TargetPath $target
  return $target
}

function Initialize-PalPanelWindowsLayout {
  param(
    [Parameter(Mandatory = $true)][string]$RepositoryRoot,
    [Parameter(Mandatory = $true)][string]$RuntimeRoot
  )

  $runtime = Assert-PalPanelManagedPath -RepositoryRoot $RepositoryRoot -TargetPath $RuntimeRoot -AllowManagedRoot
  $directories = @(
    "config",
    "data",
    "data\database",
    "data\backups",
    "data\logs",
    "data\saves",
    "data\tasks",
    "steamcmd",
    "palworld",
    "mods",
    "mods\downloads",
    "mods\cache",
    "mods\staging",
    "mods\fixtures",
    "ue4ss",
    "paldefender",
    "package",
    "artifacts",
    "temp",
    "e2e"
  )
  New-Item -ItemType Directory -Force -Path $runtime | Out-Null
  foreach ($relative in $directories) {
    $path = Join-Path $runtime $relative
    Assert-PalPanelManagedPath -RepositoryRoot $RepositoryRoot -TargetPath $path | Out-Null
    New-Item -ItemType Directory -Force -Path $path | Out-Null
  }
  Assert-NoPalPanelReparsePoint -RepositoryRoot $RepositoryRoot -TargetPath $runtime
  return $runtime
}

function Remove-PalPanelManagedDirectory {
  param(
    [Parameter(Mandatory = $true)][string]$RepositoryRoot,
    [Parameter(Mandatory = $true)][string]$RuntimeRoot,
    [Parameter(Mandatory = $true)][string]$TargetPath
  )

  $runtime = Assert-PalPanelManagedPath -RepositoryRoot $RepositoryRoot -TargetPath $RuntimeRoot -AllowManagedRoot
  $target = Assert-PalPanelManagedPath -RepositoryRoot $RepositoryRoot -TargetPath $TargetPath
  if (-not (Test-PalPanelPathWithin -Root $runtime -Target $target)) {
    throw "refusing to remove a path outside the selected runtime root: $target"
  }
  if ([string]::Equals($runtime, $target, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "refusing to remove the selected runtime root: $target"
  }
  Assert-NoPalPanelReparsePoint -RepositoryRoot $RepositoryRoot -TargetPath $target
  if (Test-Path -LiteralPath $target) {
    $pending = [System.Collections.Generic.Stack[string]]::new()
    $pending.Push($target)
    while ($pending.Count -gt 0) {
      $directory = $pending.Pop()
      foreach ($item in Get-ChildItem -LiteralPath $directory -Force) {
        if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
          throw "refusing to remove a directory tree containing a reparse point: $($item.FullName)"
        }
        if ($item.PSIsContainer) {
          $pending.Push($item.FullName)
        }
      }
    }
    Assert-NoPalPanelReparsePoint -RepositoryRoot $RepositoryRoot -TargetPath $target
    Remove-Item -LiteralPath $target -Recurse -Force
  }
}

function ConvertTo-PalPanelCommandLineArgument {
  param([AllowEmptyString()][string]$Argument)

  if ($Argument.Length -eq 0) { return '""' }
  if ($Argument -notmatch '[\s"]') { return $Argument }

  $builder = [System.Text.StringBuilder]::new()
  [void]$builder.Append('"')
  $backslashes = 0
  foreach ($character in $Argument.ToCharArray()) {
    if ($character -eq '\') {
      $backslashes++
      continue
    }
    if ($character -eq '"') {
      for ($index = 0; $index -lt (2 * $backslashes + 1); $index++) {
        [void]$builder.Append('\')
      }
      [void]$builder.Append('"')
      $backslashes = 0
      continue
    }
    for ($index = 0; $index -lt $backslashes; $index++) {
      [void]$builder.Append('\')
    }
    $backslashes = 0
    [void]$builder.Append($character)
  }
  for ($index = 0; $index -lt (2 * $backslashes); $index++) {
    [void]$builder.Append('\')
  }
  [void]$builder.Append('"')
  return $builder.ToString()
}

function Set-PalPanelProcessArguments {
  param(
    [Parameter(Mandatory = $true)][System.Diagnostics.ProcessStartInfo]$StartInfo,
    [string[]]$Arguments = @()
  )

  if ($StartInfo.PSObject.Properties.Name -contains "ArgumentList") {
    foreach ($argument in $Arguments) {
      $StartInfo.ArgumentList.Add([string]$argument)
    }
    return
  }
  $StartInfo.Arguments = (($Arguments | ForEach-Object { ConvertTo-PalPanelCommandLineArgument ([string]$_) }) -join ' ')
}

function Set-PalPanelProcessEnvironment {
  param(
    [Parameter(Mandatory = $true)][System.Diagnostics.ProcessStartInfo]$StartInfo,
    [hashtable]$Environment = @{}
  )

  foreach ($name in $Environment.Keys) {
    if ($StartInfo.PSObject.Properties.Name -contains "Environment") {
      $StartInfo.Environment[[string]$name] = [string]$Environment[$name]
    } else {
      $StartInfo.EnvironmentVariables[[string]$name] = [string]$Environment[$name]
    }
  }
}

function Invoke-PalPanelExternal {
  param(
    [Parameter(Mandatory = $true)][string]$FilePath,
    [string[]]$Arguments = @(),
    [Parameter(Mandatory = $true)][string]$WorkingDirectory,
    [Parameter(Mandatory = $true)][int]$TimeoutSeconds,
    [Parameter(Mandatory = $true)][string]$StdoutPath,
    [Parameter(Mandatory = $true)][string]$StderrPath,
    [hashtable]$Environment = @{},
    [string]$Activity = "external command"
  )

  if ($TimeoutSeconds -le 0) {
    throw "external command timeout must be positive"
  }
  $working = Resolve-PalPanelPath -Path $WorkingDirectory -BasePath $WorkingDirectory
  if (-not (Test-Path -LiteralPath $working -PathType Container)) {
    throw "working directory does not exist: $working"
  }
  foreach ($path in @($StdoutPath, $StderrPath)) {
    $parent = Split-Path -Parent $path
    if ($parent) {
      New-Item -ItemType Directory -Force -Path $parent | Out-Null
    }
  }

  $start = Get-Date
  $process = [System.Diagnostics.Process]::new()
  $started = $false
  $stdoutTask = $null
  $stderrTask = $null
  $logsWritten = $false
  $info = [System.Diagnostics.ProcessStartInfo]::new()
  $info.FileName = $FilePath
  $info.WorkingDirectory = $working
  $info.UseShellExecute = $false
  $info.CreateNoWindow = $true
  $info.RedirectStandardOutput = $true
  $info.RedirectStandardError = $true
  Set-PalPanelProcessArguments -StartInfo $info -Arguments $Arguments
  Set-PalPanelProcessEnvironment -StartInfo $info -Environment $Environment
  $process.StartInfo = $info

  try {
    if (-not $process.Start()) {
      throw "failed to start external command: $FilePath"
    }
    $started = $true
    $stdoutTask = $process.StandardOutput.ReadToEndAsync()
    $stderrTask = $process.StandardError.ReadToEndAsync()
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    while (-not $process.WaitForExit(500)) {
      $remaining = [Math]::Max(0, [int]($deadline - [DateTime]::UtcNow).TotalSeconds)
      Write-Progress -Activity $Activity -Status "$remaining seconds remaining"
      if ([DateTime]::UtcNow -ge $deadline) {
        try {
          $process.Kill($true)
        } catch {
          try { $process.Kill() } catch { }
        }
        throw "$Activity timed out after $TimeoutSeconds seconds"
      }
    }
    Write-Progress -Activity $Activity -Completed
    $stdout = $stdoutTask.Result
    $stderr = $stderrTask.Result
    $utf8 = [System.Text.UTF8Encoding]::new($false)
    [System.IO.File]::WriteAllText($StdoutPath, $stdout, $utf8)
    [System.IO.File]::WriteAllText($StderrPath, $stderr, $utf8)
    $logsWritten = $true
    if (-not [string]::IsNullOrWhiteSpace($stdout)) {
      Write-Host $stdout.TrimEnd()
    }
    if (-not [string]::IsNullOrWhiteSpace($stderr)) {
      Write-Host $stderr.TrimEnd() -ForegroundColor Yellow
    }
    if ($process.ExitCode -ne 0) {
      throw "$Activity failed with exit code $($process.ExitCode); stderr: $StderrPath"
    }
    return [pscustomobject]@{
      ExitCode = $process.ExitCode
      DurationSeconds = [Math]::Round(((Get-Date) - $start).TotalSeconds, 3)
      StdoutPath = $StdoutPath
      StderrPath = $StderrPath
    }
  } finally {
    Write-Progress -Activity $Activity -Completed
    if ($started -and -not $process.HasExited) {
      try {
        $process.Kill($true)
      } catch {
        try { $process.Kill() } catch { }
      }
    }
    if ($started -and -not $logsWritten) {
      try {
        $process.WaitForExit(5000) | Out-Null
        $stdout = if ($null -ne $stdoutTask) { $stdoutTask.Result } else { "" }
        $stderr = if ($null -ne $stderrTask) { $stderrTask.Result } else { "" }
        $utf8 = [System.Text.UTF8Encoding]::new($false)
        [System.IO.File]::WriteAllText($StdoutPath, $stdout, $utf8)
        [System.IO.File]::WriteAllText($StderrPath, $stderr, $utf8)
      } catch { }
    }
    $process.Dispose()
  }
}

Export-ModuleMember -Function @(
  "Get-PalPanelRepositoryRoot",
  "Resolve-PalPanelPath",
  "Test-PalPanelPathWithin",
  "Assert-PalPanelManagedPath",
  "Initialize-PalPanelWindowsLayout",
  "Remove-PalPanelManagedDirectory",
  "Set-PalPanelProcessArguments",
  "Set-PalPanelProcessEnvironment",
  "Invoke-PalPanelExternal"
)
