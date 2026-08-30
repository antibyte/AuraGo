#Requires -Version 5.1

<#
.SYNOPSIS
Runs a command in a disposable detached AuraGo worktree.

.DESCRIPTION
Creates the worktree below the Windows temporary directory and removes it in a
finally block. Git metadata is pruned even when the command fails. Use this for
clean build and release validation instead of ad-hoc clones or worktrees.

.EXAMPLE
.\scripts\invoke-clean-worktree.ps1 -Command go -ArgumentList @('test', './...')
#>

[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateNotNullOrEmpty()]
    [string]$Command,

    [string[]]$ArgumentList = @(),

    [ValidateNotNullOrEmpty()]
    [string]$Revision = 'HEAD',

    [ValidatePattern('^[A-Za-z0-9._-]+$')]
    [string]$Purpose = 'build'
)

$ErrorActionPreference = 'Stop'
$repoRoot = (& git -C $PSScriptRoot rev-parse --show-toplevel 2>$null)
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($repoRoot)) {
    throw 'Could not resolve the AuraGo repository root.'
}
$repoRoot = [System.IO.Path]::GetFullPath($repoRoot.Trim())

$tempRoot = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath()).TrimEnd('\') + '\'
$worktreeName = 'aurago-{0}-{1}' -f $Purpose, [guid]::NewGuid().ToString('N').Substring(0, 10)
$worktreePath = Join-Path $tempRoot $worktreeName
$worktreePath = [System.IO.Path]::GetFullPath($worktreePath)
if (-not $worktreePath.StartsWith($tempRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Refusing unsafe temporary worktree path: $worktreePath"
}

$worktreeCreated = $false
$locationPushed = $false
try {
    & git -C $repoRoot worktree add --detach -- $worktreePath $Revision
    if ($LASTEXITCODE -ne 0) {
        throw "Could not create temporary worktree for revision $Revision."
    }
    $worktreeCreated = $true

    Push-Location -LiteralPath $worktreePath
    $locationPushed = $true
    & $Command @ArgumentList
    $commandExitCode = $LASTEXITCODE
    if ($null -eq $commandExitCode) {
        $commandExitCode = if ($?) { 0 } else { 1 }
    }
    if ($commandExitCode -ne 0) {
        throw "Command failed with exit code ${commandExitCode}: $Command"
    }
} finally {
    if ($locationPushed) {
        Pop-Location
    }

    if ($worktreeCreated -and (Test-Path -LiteralPath $worktreePath)) {
        Get-ChildItem -LiteralPath $worktreePath -File -Force -Recurse -ErrorAction SilentlyContinue |
            ForEach-Object { $_.IsReadOnly = $false }
        & git -C $repoRoot worktree remove --force -- $worktreePath
        if ($LASTEXITCODE -ne 0) {
            Write-Error "Failed to remove temporary worktree: $worktreePath"
        }
    }

    & git -C $repoRoot worktree prune
}
