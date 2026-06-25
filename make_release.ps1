# SECURITY: This script handles sensitive data. Do not echo secrets.
# make_release.ps1 — Build all release artifacts and publish to GitHub Releases
# Requires: PowerShell 5.1+, Go 1.26.4+, GitHub CLI (gh), tar

param(
    [string]$Version
)

$ErrorActionPreference = 'Stop'

# ── Banner ────────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "  +--------------------------------------------+" -ForegroundColor Cyan
Write-Host "  |  AuraGo Release Builder                     |" -ForegroundColor Cyan
Write-Host "  |  Builds + uploads all release artifacts   |" -ForegroundColor Cyan
Write-Host "  +--------------------------------------------+" -ForegroundColor Cyan
Write-Host ""

# ── Check prerequisites ──────────────────────────────────────────────────
Write-Host "[0/5] Checking prerequisites..." -ForegroundColor Yellow

# Go
try {
    $minGoVersion = [version]"1.26.4"
    $rawGoVersion = ((go version) -split '\s+')[2]
    $goVersionText = $rawGoVersion -replace '^go', ''
    $goVersion = [version]$goVersionText
    if ($goVersion -lt $minGoVersion) {
        throw "Go $minGoVersion or newer is required, found $rawGoVersion"
    }
    Write-Host "    Go: $rawGoVersion" -ForegroundColor Green
} catch {
    Write-Host "[ERROR] $_" -ForegroundColor Red
    Write-Host "        Install Go 1.26.4+ from https://go.dev/dl/" -ForegroundColor Yellow
    exit 1
}

# GitHub CLI
try {
    $ghVersion = (gh --version | Select-Object -First 1)
    Write-Host "    GitHub CLI: $ghVersion" -ForegroundColor Green
} catch {
    Write-Host "[ERROR] GitHub CLI (gh) not found. Install from https://cli.github.com" -ForegroundColor Red
    Write-Host "         Then run: gh auth login" -ForegroundColor Yellow
    exit 1
}

# tar
try {
    $null = tar --version 2>$null
    Write-Host "    tar: OK" -ForegroundColor Green
} catch {
    Write-Host "[ERROR] tar not found. Requires Windows 10 Build 17063 or later." -ForegroundColor Red
    exit 1
}

Write-Host ""

# ── Version tag ──────────────────────────────────────────────────────────
if ([string]::IsNullOrEmpty($Version)) {
    $dateStr = Get-Date -Format "yyyy.MM.dd"
    $Version = "v$dateStr"
    Write-Host "  Release tag [$Version]: " -NoNewline -ForegroundColor Cyan
    $input = Read-Host
    if (-not [string]::IsNullOrEmpty($input)) {
        $Version = $input
    }
}

Write-Host "  Release: $Version" -ForegroundColor Cyan
Write-Host ""

# ── Prepare output dirs ──────────────────────────────────────────────────
$scriptDir = $PSScriptRoot
if ([string]::IsNullOrEmpty($scriptDir)) {
    $scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
}
Set-Location $scriptDir

if (Test-Path "deploy") {
    Remove-Item -Recurse -Force "deploy"
}
New-Item -ItemType Directory -Force -Path "deploy" | Out-Null
if (-not (Test-Path "bin")) {
    New-Item -ItemType Directory -Force -Path "bin" | Out-Null
}

# ── Step 1: Pack resources.dat ───────────────────────────────────────────
Write-Host "[1/5] Packing resources.dat ..." -ForegroundColor Yellow

$tmpStage = Join-Path $env:TEMP "aurago-release-$([guid]::NewGuid().ToString().Substring(0,8))"
$stagingDirs = @(
    "agent_workspace\skills",
    "agent_workspace\tools",
    "agent_workspace\workdir\attachments",
    "assets\media_samples",
    "assets\mission_samples",
    "assets\cheatsheet_samples",
    "assets\skill_samples",
    "data\vectordb",
    "log",
    "prompts"
)

foreach ($dir in $stagingDirs) {
    $path = Join-Path $tmpStage $dir
    $null = New-Item -ItemType Directory -Force -Path $path
}

# Copy skills (excluding credentials)
if (Test-Path "agent_workspace\skills") {
    Copy-Item -Path "agent_workspace\skills" -Destination (Join-Path $tmpStage "agent_workspace\skills") -Recurse -Force
    # Strip credential files
    $creds = @("client_secret.json", "client_secrets.json", "token.json")
    foreach ($f in $creds) {
        $path = Join-Path $tmpStage "agent_workspace\skills\$f"
        if (Test-Path $path) {
            Remove-Item -Force $path
        }
    }
}

# Copy prompts
if (Test-Path "prompts") {
    Copy-Item -Path "prompts" -Destination (Join-Path $tmpStage "prompts") -Recurse -Force
}

# Copy agent_workspace/tools
if (Test-Path "agent_workspace\tools") {
    Copy-Item -Path "agent_workspace\tools" -Destination (Join-Path $tmpStage "agent_workspace\tools") -Recurse -Force
}

# Copy agent_workspace/workdir/attachments
if (Test-Path "agent_workspace\workdir\attachments") {
    Copy-Item -Path "agent_workspace\workdir\attachments" -Destination (Join-Path $tmpStage "agent_workspace\workdir\attachments") -Recurse -Force
}

# Copy bundled sample media
if (Test-Path "assets\media_samples") {
    Copy-Item -Path "assets\media_samples" -Destination (Join-Path $tmpStage "assets\media_samples") -Recurse -Force
}
if (Test-Path "assets\mission_samples") {
    Copy-Item -Path "assets\mission_samples" -Destination (Join-Path $tmpStage "assets\mission_samples") -Recurse -Force
}
if (Test-Path "assets\cheatsheet_samples") {
    Copy-Item -Path "assets\cheatsheet_samples" -Destination (Join-Path $tmpStage "assets\cheatsheet_samples") -Recurse -Force
}
if (Test-Path "assets\skill_samples") {
    Copy-Item -Path "assets\skill_samples" -Destination (Join-Path $tmpStage "assets\skill_samples") -Recurse -Force
}

# Copy data/vectordb and log (empty dirs already created)

# Strip sensitive values from config template
$configContent = Get-Content "config_template.yaml" -Raw
$configContent = $configContent -replace 'api_key: "sk-[^"]*"', 'api_key: ""'
$configContent = $configContent -replace 'bot_token: "[^"]*"', 'bot_token: ""'
$configContent = $configContent -replace 'access_token: "[^"]*"', 'access_token: ""'
Set-Content -Path (Join-Path $tmpStage "config.yaml") -Value $configContent

# Create tar.gz
$resourcesOut = Join-Path $scriptDir "deploy\resources.dat"
tar -czf $resourcesOut -C $tmpStage .
Remove-Item -Recurse -Force $tmpStage

Write-Host "    -> deploy\resources.dat" -ForegroundColor Green

# ── Step 2: Compile all binaries ─────────────────────────────────────────
Write-Host ""
Write-Host "[2/5] Compiling binaries (cross-compilation for all platforms)..." -ForegroundColor Yellow
Write-Host ""

$env:CGO_ENABLED = "0"

# Helper function for building
function Build-Binary {
    param(
        [string]$Output,
        [string]$Target
    )
    $fullOutput = Join-Path $scriptDir $Output
    Write-Host "    -> $Output" -ForegroundColor Green
    go build -trimpath -ldflags="-s -w" -o $fullOutput $Target
    if ($LASTEXITCODE -ne 0) {
        throw "Build failed for $Output"
    }
}

# ── Linux amd64 ──
Write-Host "  Linux amd64..." -ForegroundColor Cyan
$env:GOOS = "linux"
$env:GOARCH = "amd64"
Build-Binary -Output "bin\aurago_linux" -Target "./cmd/aurago/"
Build-Binary -Output "bin\lifeboat_linux" -Target "./cmd/lifeboat/"
Build-Binary -Output "bin\config-merger_linux" -Target "./cmd/config-merger/"
Build-Binary -Output "bin\aurago-remote_linux" -Target "./cmd/remote/"
# ── Linux arm64 ──
Write-Host "  Linux arm64..." -ForegroundColor Cyan
$env:GOOS = "linux"
$env:GOARCH = "arm64"
Build-Binary -Output "bin\aurago_linux_arm64" -Target "./cmd/aurago/"
Build-Binary -Output "bin\lifeboat_linux_arm64" -Target "./cmd/lifeboat/"
Build-Binary -Output "bin\config-merger_linux_arm64" -Target "./cmd/config-merger/"
Build-Binary -Output "bin\aurago-remote_linux_arm64" -Target "./cmd/remote/"

# ── macOS amd64 ──
Write-Host "  macOS amd64..." -ForegroundColor Cyan
$env:GOOS = "darwin"
$env:GOARCH = "amd64"
Build-Binary -Output "deploy\aurago_darwin_amd64" -Target "./cmd/aurago/"
Build-Binary -Output "deploy\aurago-remote_darwin_amd64" -Target "./cmd/remote/"

# ── macOS arm64 ──
Write-Host "  macOS arm64..." -ForegroundColor Cyan
$env:GOOS = "darwin"
$env:GOARCH = "arm64"
Build-Binary -Output "deploy\aurago_darwin_arm64" -Target "./cmd/aurago/"
Build-Binary -Output "deploy\aurago-remote_darwin_arm64" -Target "./cmd/remote/"

# ── Windows amd64 ──
Write-Host "  Windows amd64..." -ForegroundColor Cyan
$env:GOOS = "windows"
$env:GOARCH = "amd64"
Build-Binary -Output "deploy\aurago_windows_amd64.exe" -Target "./cmd/aurago/"
Build-Binary -Output "deploy\aurago-remote_windows_amd64.exe" -Target "./cmd/remote/"

# ── Windows arm64 ──
Write-Host "  Windows arm64..." -ForegroundColor Cyan
$env:GOOS = "windows"
$env:GOARCH = "arm64"
Build-Binary -Output "deploy\aurago_windows_arm64.exe" -Target "./cmd/aurago/"
Build-Binary -Output "deploy\aurago-remote_windows_arm64.exe" -Target "./cmd/remote/"

# Copy install.sh
Copy-Item "install.sh" "deploy\install.sh" -Force
Copy-Item "update.sh" "deploy\update.sh" -Force
Write-Host "    -> deploy\install.sh" -ForegroundColor Green
Write-Host "    -> deploy\update.sh" -ForegroundColor Green

Write-Host "  Generating SHA256SUMS..." -ForegroundColor Cyan
function Get-Sha256Hex {
    param([string]$Path)
    $sha = [System.Security.Cryptography.SHA256]::Create()
    $stream = [IO.File]::OpenRead($Path)
    try {
        $bytes = $sha.ComputeHash($stream)
        return -join ($bytes | ForEach-Object { $_.ToString("x2") })
    } finally {
        $stream.Dispose()
        $sha.Dispose()
    }
}

$checksumFiles = @(
    "deploy\resources.dat",
    "deploy\install.sh",
    "deploy\update.sh",
    "bin\aurago_linux",
    "bin\aurago_linux_arm64",
    "bin\lifeboat_linux",
    "bin\lifeboat_linux_arm64",
    "bin\config-merger_linux",
    "bin\config-merger_linux_arm64",
    "bin\aurago-remote_linux",
    "bin\aurago-remote_linux_arm64",
    "deploy\aurago_darwin_amd64",
    "deploy\aurago_darwin_arm64",
    "deploy\aurago-remote_darwin_amd64",
    "deploy\aurago-remote_darwin_arm64",
    "deploy\aurago_windows_amd64.exe",
    "deploy\aurago_windows_arm64.exe",
    "deploy\aurago-remote_windows_amd64.exe",
    "deploy\aurago-remote_windows_arm64.exe"
)
$checksumLines = foreach ($file in $checksumFiles) {
    $fullPath = Join-Path $scriptDir $file
    if (Test-Path $fullPath) {
        $hash = Get-Sha256Hex $fullPath
        "{0}  {1}" -f $hash, [IO.Path]::GetFileName($file)
    }
}
Set-Content -Path "deploy\SHA256SUMS" -Value $checksumLines
Write-Host "    -> deploy\SHA256SUMS" -ForegroundColor Green

# Reset env
$env:GOOS = "windows"
$env:GOARCH = "amd64"

Write-Host ""

# ── Step 3: Commit & push code ───────────────────────────────────────────
Write-Host "[3/5] Pushing code to GitHub..." -ForegroundColor Yellow

git add .
$status = git diff-index --quiet HEAD 2>&1
$hasChanges = $LASTEXITCODE -ne 0
if ($hasChanges) {
    git commit -m "build: release $Version [skip actions]"
    git push origin main
    Write-Host "    Code pushed." -ForegroundColor Green
}
if (-not $hasChanges) {
    Write-Host "    Nothing to commit - working tree clean." -ForegroundColor Gray
}

Write-Host ""

# ── Step 4: Create GitHub Release and upload assets ──────────────────────
Write-Host "[4/5] Creating GitHub Release $Version ..." -ForegroundColor Yellow
Write-Host ""

# Collect existing assets
$assets = @()
$assetPaths = @(
    "deploy\resources.dat",
    "deploy\SHA256SUMS",
    "deploy\SHA256SUMS.sig",
    "deploy\SHA256SUMS.pem",
    "bin\aurago_linux",
    "bin\aurago_linux_arm64",
    "bin\lifeboat_linux",
    "bin\lifeboat_linux_arm64",
    "bin\config-merger_linux",
    "bin\config-merger_linux_arm64",
    "bin\aurago-remote_linux",
    "bin\aurago-remote_linux_arm64",
    "deploy\aurago_darwin_amd64",
    "deploy\aurago_darwin_arm64",
    "deploy\aurago-remote_darwin_amd64",
    "deploy\aurago-remote_darwin_arm64",
    "deploy\aurago_windows_amd64.exe",
    "deploy\aurago_windows_arm64.exe",
    "deploy\aurago-remote_windows_amd64.exe",
    "deploy\aurago-remote_windows_arm64.exe",
    "deploy\install.sh",
    "deploy\update.sh"
)

foreach ($path in $assetPaths) {
    $fullPath = Join-Path $scriptDir $path
    if (Test-Path $fullPath) {
        $assets += $fullPath
    }
}

$notes = @'
## AuraGo {VERSION}

### Installation

**One-liner (no Go required):**
```bash
curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh | bash
```

**Update existing install:**
```bash
./update.sh
```

### Included binaries
- Linux amd64 / arm64 (main, lifeboat, config-merger, aurago-remote)
- macOS amd64 / arm64 (Apple Silicon)
- Windows x64 / arm64
'@
$notes = $notes -replace '\{VERSION\}', $Version

try {
    $ghArgs = @("release", "create", $Version, $assets, "--title", "AuraGo $Version", "--notes", $notes)
    gh @ghArgs
    Write-Host "    Release created successfully!" -ForegroundColor Green
} catch {
    Write-Host ""
    Write-Host "[ERROR] gh release create failed. Check:" -ForegroundColor Red
    Write-Host "         - gh auth status (must be logged in)" -ForegroundColor Yellow
    Write-Host "         - Tag $Version may already exist (run: gh release delete $Version)" -ForegroundColor Yellow
    exit 1
}

Write-Host ""

# ── Step 5: Cleanup old releases ─────────────────────────────────────────
Write-Host "[5/5] Cleaning up old releases (keeping latest 3)..." -ForegroundColor Yellow

try {
    $releases = gh release list --limit 20 --json tagName | ConvertFrom-Json
    $toDelete = $releases | Select-Object -Skip 3
    foreach ($rel in $toDelete) {
        Write-Host "    Deleting old release: $($rel.tagName)" -ForegroundColor Gray
        gh release delete $rel.tagName --yes --cleanup-tag 2>$null
    }
} catch {
    Write-Host "    Could not cleanup old releases (non-fatal)" -ForegroundColor Gray
}

Write-Host ""
Write-Host "Verifying release..." -ForegroundColor Yellow
$releaseInfo = gh release view $Version --json tagName,url | ConvertFrom-Json
Write-Host "  Tag: $($releaseInfo.tagName) | $($releaseInfo.url)" -ForegroundColor Green
Write-Host ""
Write-Host "  --- Release $Version published successfully ---" -ForegroundColor Cyan
