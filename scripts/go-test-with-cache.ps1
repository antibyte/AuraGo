[CmdletBinding()]
param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]] $Packages
)

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$cacheDir = Join-Path $repoRoot "disposable\gocache"
New-Item -ItemType Directory -Force $cacheDir | Out-Null

if (-not $env:GOCACHE) {
    $env:GOCACHE = $cacheDir
}

if (-not $env:LOCALAPPDATA) {
    $localAppData = Join-Path $repoRoot "disposable\localappdata"
    New-Item -ItemType Directory -Force $localAppData | Out-Null
    $env:LOCALAPPDATA = $localAppData
}

if (-not $Packages -or $Packages.Count -eq 0) {
    $Packages = @("./...")
}

& go test @Packages
exit $LASTEXITCODE
