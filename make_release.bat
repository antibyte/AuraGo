@echo off
REM make_release.bat -- Build all release artifacts and publish to GitHub Releases
REM
REM Usage:
REM   make_release.bat            -> prompts for version tag (default: v{YYYY.MM.DD})
REM   make_release.bat v1.2.3     -> uses given tag directly
REM
REM Prerequisites:
REM   - Go 1.26+         (https://go.dev)
REM   - GitHub CLI (gh)  (https://cli.github.com)  -- run `gh auth login` once
REM   - tar              (built-in on Windows 10 Build 17063+)
REM
REM Published release assets (consumed by install.sh / update.sh):
REM   resources.dat                   shared runtime resources (tar.gz)
REM   aurago_linux                    Linux amd64 main binary
REM   aurago_linux_arm64              Linux arm64 main binary
REM   lifeboat_linux                  Linux amd64 lifeboat binary
REM   lifeboat_linux_arm64            Linux arm64 lifeboat binary
REM   config-merger_linux             Linux amd64 config-merger binary
REM   config-merger_linux_arm64       Linux arm64 config-merger binary
REM   aurago-remote_linux             Linux amd64 remote agent
REM   aurago-remote_linux_arm64       Linux arm64 remote agent
REM   aurago_darwin_amd64             macOS x86_64 binary
REM   aurago_darwin_arm64             macOS Apple Silicon binary
REM   aurago-remote_darwin_amd64      macOS x86_64 remote agent
REM   aurago-remote_darwin_arm64      macOS Apple Silicon remote agent
REM   aurago_windows_amd64.exe        Windows x64 binary
REM   aurago_windows_arm64.exe        Windows ARM64 binary
REM   aurago-remote_windows_amd64.exe Windows x64 remote agent
REM   aurago-remote_windows_arm64.exe Windows ARM64 remote agent
REM   install.sh                      one-liner installer script

setlocal enabledelayedexpansion
cd /d "%~dp0"

echo.
echo  ?------------------------------------------?
echo  |  AuraGo Release Builder                  |
echo  |  Builds + uploads all release artifacts  |
echo  ?------------------------------------------?
echo.

REM -- Check prerequisites --------------------------------------------------
echo [0/5] Checking prerequisites...

where go >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Go not found in PATH.
    echo         Install from https://go.dev/dl/
    exit /b 1
)
for /f "tokens=3" %%v in ('go version') do echo     Go: %%v

where gh >nul 2>&1
if errorlevel 1 (
    echo [ERROR] GitHub CLI (gh) not found.
    echo         Install from https://cli.github.com
    echo         Then run:  gh auth login
    exit /b 1
)
for /f "tokens=*" %%v in ('gh --version 2^>nul ^| findstr /i "gh version"') do echo     %%v

where tar >nul 2>&1
if errorlevel 1 (
    echo [ERROR] tar not found. Requires Windows 10 Build 17063 or later.
    exit /b 1
)
echo     tar: OK
echo.

REM -- Version tag ----------------------------------------------------------
if not "%~1"=="" (
    set VERSION=%~1
) else (
    for /f "delims=" %%d in ('powershell -nologo -noprofile -command "Get-Date -Format 'yyyy.MM.dd'"') do set DATESTR=%%d
    set VERSION=v!DATESTR!
    set /p VERSION="  Release tag [!VERSION!]: "
    if "!VERSION!"=="" set VERSION=v!DATESTR!
)

echo   Release: !VERSION!
echo.

REM -- Prepare output dirs --------------------------------------------------
if exist deploy rmdir /s /q deploy
mkdir deploy
if not exist bin mkdir bin

REM -- Step 1: Pack resources.dat -------------------------------------------
echo [1/5] Packing resources.dat ...

set TMPSTAGE=%TEMP%\aurago-release-%RANDOM%
mkdir "%TMPSTAGE%\agent_workspace\skills"
mkdir "%TMPSTAGE%\agent_workspace\tools"
mkdir "%TMPSTAGE%\agent_workspace\workdir\attachments"
mkdir "%TMPSTAGE%\data\vectordb"
mkdir "%TMPSTAGE%\log"

if exist "prompts" (
    xcopy /e /i /q "prompts" "%TMPSTAGE%\prompts" >nul
)
if exist "agent_workspace\skills" (
    xcopy /e /i /q "agent_workspace\skills" "%TMPSTAGE%\agent_workspace\skills" >nul
)
REM Strip credential files that must never be deployed
del /f /q "%TMPSTAGE%\agent_workspace\skills\client_secret.json"  2>nul
del /f /q "%TMPSTAGE%\agent_workspace\skills\client_secrets.json" 2>nul
del /f /q "%TMPSTAGE%\agent_workspace\skills\token.json"           2>nul

REM Strip sensitive values from config template
powershell -nologo -noprofile -command ^
  "(Get-Content 'config_template.yaml') -replace 'api_key: \"sk-[^\"]*\"','api_key: \"\"' -replace 'bot_token: \"[^\"]*\"','bot_token: \"\"' -replace 'access_token: \"[^\"]*\"','access_token: \"\"' | Set-Content '%TMPSTAGE%\config.yaml'"

tar -czf "deploy\resources.dat" -C "%TMPSTAGE%" .
rmdir /s /q "%TMPSTAGE%"
echo     -> deploy\resources.dat

REM -- Step 2: Compile all binaries -----------------------------------------
echo [2/5] Compiling binaries (cross-compilation for all platforms)...
echo.
set CGO_ENABLED=0

REM -- Linux amd64 -- primary binaries go to bin/ (install.sh / update.sh needs them there)
echo   Linux amd64...
set GOOS=linux
set GOARCH=amd64
go build -trimpath -ldflags="-s -w" -o "bin\aurago_linux"           ./cmd/aurago/       || goto :build_error
go build -trimpath -ldflags="-s -w" -o "bin\lifeboat_linux"         ./cmd/lifeboat/     || goto :build_error
go build -trimpath -ldflags="-s -w" -o "bin\config-merger_linux"    ./cmd/config-merger/ || goto :build_error
go build -trimpath -ldflags="-s -w" -o "bin\aurago-remote_linux"    ./cmd/remote/       || goto :build_error
echo     -> bin\aurago_linux  bin\lifeboat_linux  bin\config-merger_linux  bin\aurago-remote_linux

REM -- Linux arm64
echo   Linux arm64...
set GOOS=linux
set GOARCH=arm64
go build -trimpath -ldflags="-s -w" -o "bin\aurago_linux_arm64"             ./cmd/aurago/        || goto :build_error
go build -trimpath -ldflags="-s -w" -o "bin\lifeboat_linux_arm64"           ./cmd/lifeboat/      || goto :build_error
go build -trimpath -ldflags="-s -w" -o "bin\config-merger_linux_arm64"      ./cmd/config-merger/ || goto :build_error
go build -trimpath -ldflags="-s -w" -o "bin\aurago-remote_linux_arm64"      ./cmd/remote/        || goto :build_error
echo     -> bin\aurago_linux_arm64  bin\lifeboat_linux_arm64  bin\config-merger_linux_arm64  bin\aurago-remote_linux_arm64

REM -- macOS amd64
echo   macOS amd64...
set GOOS=darwin
set GOARCH=amd64
go build -trimpath -ldflags="-s -w" -o "deploy\aurago_darwin_amd64"        ./cmd/aurago/ || goto :build_error
go build -trimpath -ldflags="-s -w" -o "deploy\aurago-remote_darwin_amd64" ./cmd/remote/ || goto :build_error
echo     -> deploy\aurago_darwin_amd64  deploy\aurago-remote_darwin_amd64

REM -- macOS arm64 (Apple Silicon)
echo   macOS arm64...
set GOOS=darwin
set GOARCH=arm64
go build -trimpath -ldflags="-s -w" -o "deploy\aurago_darwin_arm64"        ./cmd/aurago/ || goto :build_error
go build -trimpath -ldflags="-s -w" -o "deploy\aurago-remote_darwin_arm64" ./cmd/remote/ || goto :build_error
echo     -> deploy\aurago_darwin_arm64  deploy\aurago-remote_darwin_arm64

REM -- Windows amd64
echo   Windows amd64...
set GOOS=windows
set GOARCH=amd64
go build -trimpath -ldflags="-s -w" -o "deploy\aurago_windows_amd64.exe"        ./cmd/aurago/ || goto :build_error
go build -trimpath -ldflags="-s -w" -o "deploy\aurago-remote_windows_amd64.exe" ./cmd/remote/ || goto :build_error
echo     -> deploy\aurago_windows_amd64.exe  deploy\aurago-remote_windows_amd64.exe

REM -- Windows arm64
echo   Windows arm64...
set GOOS=windows
set GOARCH=arm64
go build -trimpath -ldflags="-s -w" -o "deploy\aurago_windows_arm64.exe"        ./cmd/aurago/ || goto :build_error
go build -trimpath -ldflags="-s -w" -o "deploy\aurago-remote_windows_arm64.exe" ./cmd/remote/ || goto :build_error
echo     -> deploy\aurago_windows_arm64.exe  deploy\aurago-remote_windows_arm64.exe

copy "install.sh" "deploy\install.sh" >nul
echo     -> deploy\install.sh

REM Reset env to native Windows so subsequent go commands in this session work normally
set GOOS=windows
set GOARCH=amd64
echo.

REM -- Step 3: Commit & push code -------------------------------------------
echo [3/5] Pushing code to GitHub...
git add .
git diff-index --quiet HEAD 2>nul
if errorlevel 1 (
    git commit -m "build: release !VERSION! [skip actions]"
    git push origin main
    echo     Code pushed.
) else (
    echo     Nothing to commit -- working tree clean.
)
echo.

REM -- Step 4: Create GitHub Release and upload assets ----------------------
echo [4/5] Creating GitHub Release !VERSION! ...
echo.

REM Collect asset paths (only files that actually exist)
set ASSETS=
for %%F in (
    "deploy\resources.dat"
    "bin\aurago_linux"
    "bin\aurago_linux_arm64"
    "bin\lifeboat_linux"
    "bin\lifeboat_linux_arm64"
    "bin\config-merger_linux"
    "bin\config-merger_linux_arm64"
    "bin\aurago-remote_linux"
    "bin\aurago-remote_linux_arm64"
    "deploy\aurago_darwin_amd64"
    "deploy\aurago_darwin_arm64"
    "deploy\aurago-remote_darwin_amd64"
    "deploy\aurago-remote_darwin_arm64"
    "deploy\aurago_windows_amd64.exe"
    "deploy\aurago_windows_arm64.exe"
    "deploy\aurago-remote_windows_amd64.exe"
    "deploy\aurago-remote_windows_arm64.exe"
    "deploy\install.sh"
) do (
    if exist %%F set ASSETS=!ASSETS! %%F
)

gh release create "!VERSION!" !ASSETS! ^
    --title "AuraGo !VERSION!" ^
    --notes "## AuraGo !VERSION!^

### Installation^

**One-liner (no Go required):**^
```bash^
curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh ^| bash^
```^

**Update existing install:**^
```bash^
./update.sh^
```^

### Included binaries^
- Linux amd64 / arm64 (main, lifeboat, config-merger, aurago-remote)^
- macOS amd64 / arm64 (Apple Silicon)^
- Windows x64 / arm64"

if errorlevel 1 (
    echo.
    echo [ERROR] gh release create failed. Check:
    echo         - `gh auth status`   (must be logged in^)
    echo         - Tag !VERSION! may already exist  (run: gh release delete !VERSION!^)
    exit /b 1
)

echo.
echo [5/5] Cleaning up old releases (keeping latest 3)...
REM List all releases sorted by creation date (newest first), skip first 3, delete the rest
for /f "skip=3 tokens=*" %%T in ('gh release list --limit 20 --json tagName --jq ".[].tagName" 2^>nul') do (
    echo     Deleting old release: %%T
    gh release delete "%%T" --yes --cleanup-tag 2>nul && echo     Deleted: %%T || echo     Could not delete: %%T
)

echo.
echo  Verifying release...
gh release view "!VERSION!" --json tagName,url --jq '"  Tag: " + .tagName + " | " + .url'
echo.
echo  === Release !VERSION! published successfully ===
goto :eof

:build_error
echo.
echo [ERROR] Build failed. Fix compilation errors above.
exit /b 1
