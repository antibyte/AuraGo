#Requires -Version 5.1
<#
.SYNOPSIS
    Quick installer for aurago-tui on Windows.
.DESCRIPTION
    Downloads the latest pre-built binary from GitHub Releases and places it in
    %LOCALAPPDATA%\aurago-tui (adding it to the user PATH if missing).
#>
$ErrorActionPreference = "Stop"

$repo = "antibyte/AuraGo"
$releaseTag = "aurago-tui-rolling"
$assetName = "aurago-tui-x86_64-pc-windows-msvc.exe"
$installDir = "$env:LOCALAPPDATA\aurago-tui"
$binaryPath = "$installDir\aurago-tui.exe"

Write-Host "==> Installing aurago-tui..." -ForegroundColor Cyan

# Create install directory
if (!(Test-Path $installDir)) {
    New-Item -ItemType Directory -Force -Path $installDir | Out-Null
}

# Download latest release asset
$url = "https://github.com/$repo/releases/download/$releaseTag/$assetName"
Write-Host "==> Downloading from $url" -ForegroundColor DarkGray

try {
    Invoke-WebRequest -Uri $url -OutFile $binaryPath -UseBasicParsing
} catch {
    Write-Host "ERROR: Failed to download binary." -ForegroundColor Red
    Write-Host "       Ensure a release exists at:" -ForegroundColor Red
    Write-Host "       https://github.com/$repo/releases/tag/$releaseTag" -ForegroundColor Red
    exit 1
}

# Add to PATH if missing
$path = [Environment]::GetEnvironmentVariable("Path", "User")
if ($path -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$path;$installDir", "User")
    Write-Host "==> Added $installDir to user PATH." -ForegroundColor Green
    Write-Host "    Restart your terminal to use 'aurago-tui'." -ForegroundColor Yellow
}

Write-Host "==> aurago-tui installed to $binaryPath" -ForegroundColor Green
Write-Host "    Run with: aurago-tui --url http://localhost:8080" -ForegroundColor Cyan
