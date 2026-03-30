@echo off
REM make_release.bat -- Build all release artifacts and publish to GitHub Releases
REM
REM Usage:
REM   make_release.bat            -> prompts for version tag (default: v{YYYY.MM.DD})
REM   make_release.bat v1.2.3     -> uses given tag directly
REM
REM Prerequisites:
REM   - Go 1.26+  (https://go.dev)
REM   - gh CLI    (https://cli.github.com) -- run "gh auth login" once
REM   - tar       (built-in Windows 10 Build 17063+)

setlocal enabledelayedexpansion
cd /d "%~dp0"

echo.
echo  +--------------------------------------------+
echo  ^|  AuraGo Release Builder                   ^|
echo  ^|  Builds + uploads all release artifacts   ^|
echo  +--------------------------------------------+
echo.

REM -- [0/5] Check prerequisites
echo [0/5] Checking prerequisites...

where go >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Go not found in PATH. Install from https://go.dev/dl/
    exit /b 1
)
for /f "tokens=3" %%v in ('go version') do echo     Go: %%v

where gh >nul 2>&1
if errorlevel 1 (
    echo [ERROR] GitHub CLI not found. Install from https://cli.github.com
    echo         Then run: gh auth login
    exit /b 1
)
echo     GitHub CLI: OK

where tar >nul 2>&1
if errorlevel 1 (
    echo [ERROR] tar not found. Requires Windows 10 Build 17063 or later.
    exit /b 1
)
echo     tar: OK
echo.

REM -- Version tag
REM  Compute default date into temp file to avoid single-quote issues in for/f
powershell -nologo -noprofile -command "Get-Date -Format 'yyyy.MM.dd'" > "%TEMP%\aurago_date.txt"
set /p DEFDATE= < "%TEMP%\aurago_date.txt"
del "%TEMP%\aurago_date.txt" 2>nul

if not "%~1"=="" (
    set VERSION=%~1
) else (
    set VERSION=v!DEFDATE!
    set /p VERSION="  Release tag [v!DEFDATE!]: "
    if "!VERSION!"=="" set VERSION=v!DEFDATE!
)
echo   Release: !VERSION!
echo.

REM -- Prepare output dirs
if exist deploy rmdir /s /q deploy
mkdir deploy
if not exist bin mkdir bin

REM -- [1/5] Pack resources.dat
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
del /f /q "%TMPSTAGE%\agent_workspace\skills\client_secret.json"  2>nul
del /f /q "%TMPSTAGE%\agent_workspace\skills\client_secrets.json" 2>nul
del /f /q "%TMPSTAGE%\agent_workspace\skills\token.json"          2>nul

powershell -nologo -noprofile -command "(Get-Content 'config_template.yaml') -replace 'api_key: \"sk-[^\"]*\"','api_key: \"\"' -replace 'bot_token: \"[^\"]*\"','bot_token: \"\"' | Set-Content '%TMPSTAGE%\config.yaml'"

tar -czf "deploy\resources.dat" -C "%TMPSTAGE%" .
rmdir /s /q "%TMPSTAGE%"
echo     -> deploy\resources.dat

REM -- [2/5] Compile all binaries
echo.
echo [2/5] Compiling binaries (cross-compilation)...
echo.
set CGO_ENABLED=0

echo   Linux amd64...
set "GOOS=linux" & set "GOARCH=amd64"
go build -trimpath -ldflags="-s -w" -o "bin\aurago_linux"          ./cmd/aurago/        || goto :build_error
go build -trimpath -ldflags="-s -w" -o "bin\lifeboat_linux"        ./cmd/lifeboat/      || goto :build_error
go build -trimpath -ldflags="-s -w" -o "bin\config-merger_linux"   ./cmd/config-merger/ || goto :build_error
go build -trimpath -ldflags="-s -w" -o "bin\aurago-remote_linux"   ./cmd/remote/        || goto :build_error
go build -trimpath -ldflags="-s -w" -o "bin\agocli_linux"          ./cmd/agocli/        || goto :build_error
    copy /y "bin\agocli_linux" "agocli" >nul
echo     -> Linux amd64 OK

echo   Linux arm64...
set "GOOS=linux" & set "GOARCH=arm64"
go build -trimpath -ldflags="-s -w" -o "bin\aurago_linux_arm64"        ./cmd/aurago/        || goto :build_error
go build -trimpath -ldflags="-s -w" -o "bin\lifeboat_linux_arm64"      ./cmd/lifeboat/      || goto :build_error
go build -trimpath -ldflags="-s -w" -o "bin\config-merger_linux_arm64" ./cmd/config-merger/ || goto :build_error
go build -trimpath -ldflags="-s -w" -o "bin\aurago-remote_linux_arm64" ./cmd/remote/        || goto :build_error
go build -trimpath -ldflags="-s -w" -o "bin\agocli_linux_arm64"        ./cmd/agocli/        || goto :build_error
echo     -> Linux arm64 OK

echo   macOS amd64...
set "GOOS=darwin" & set "GOARCH=amd64"
go build -trimpath -ldflags="-s -w" -o "deploy\aurago_darwin_amd64"        ./cmd/aurago/ || goto :build_error
go build -trimpath -ldflags="-s -w" -o "deploy\aurago-remote_darwin_amd64" ./cmd/remote/ || goto :build_error
go build -trimpath -ldflags="-s -w" -o "deploy\agocli_darwin_amd64"        ./cmd/agocli/ || goto :build_error
echo     -> macOS amd64 OK

echo   macOS arm64...
set "GOOS=darwin" & set "GOARCH=arm64"
go build -trimpath -ldflags="-s -w" -o "deploy\aurago_darwin_arm64"        ./cmd/aurago/ || goto :build_error
go build -trimpath -ldflags="-s -w" -o "deploy\aurago-remote_darwin_arm64" ./cmd/remote/ || goto :build_error
go build -trimpath -ldflags="-s -w" -o "deploy\agocli_darwin_arm64"        ./cmd/agocli/ || goto :build_error
echo     -> macOS arm64 OK

echo   Windows amd64...
set "GOOS=windows" & set "GOARCH=amd64"
go build -trimpath -ldflags="-s -w" -o "deploy\aurago_windows_amd64.exe"        ./cmd/aurago/ || goto :build_error
go build -trimpath -ldflags="-s -w" -o "deploy\aurago-remote_windows_amd64.exe" ./cmd/remote/ || goto :build_error
go build -trimpath -ldflags="-s -w" -o "deploy\agocli_windows_amd64.exe"        ./cmd/agocli/ || goto :build_error
echo     -> Windows amd64 OK

echo   Windows arm64...
set "GOOS=windows" & set "GOARCH=arm64"
go build -trimpath -ldflags="-s -w" -o "deploy\aurago_windows_arm64.exe"        ./cmd/aurago/ || goto :build_error
go build -trimpath -ldflags="-s -w" -o "deploy\aurago-remote_windows_arm64.exe" ./cmd/remote/ || goto :build_error
go build -trimpath -ldflags="-s -w" -o "deploy\agocli_windows_arm64.exe"        ./cmd/agocli/ || goto :build_error
echo     -> Windows arm64 OK

copy "install.sh" "deploy\install.sh" >nul
echo     -> deploy\install.sh
set "GOOS=windows" & set "GOARCH=amd64"
echo.

REM -- [3/5] Commit and push
echo [3/5] Pushing code to GitHub...
git add .
git diff-index --quiet HEAD 2>nul
if errorlevel 1 (
    git commit -m "build: release !VERSION! [skip actions]"
    git push origin main
    echo     Code pushed.
) else (
    echo     Nothing to commit.
)
echo.

REM -- [4/5] Create GitHub Release
echo [4/5] Creating GitHub Release !VERSION! ...
echo.

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
    "bin\agocli_linux"
    "bin\agocli_linux_arm64"
    "agocli"
    "deploy\aurago_darwin_amd64"
    "deploy\aurago_darwin_arm64"
    "deploy\aurago-remote_darwin_amd64"
    "deploy\aurago-remote_darwin_arm64"
    "deploy\agocli_darwin_amd64"
    "deploy\agocli_darwin_arm64"
    "deploy\aurago_windows_amd64.exe"
    "deploy\aurago_windows_arm64.exe"
    "deploy\aurago-remote_windows_amd64.exe"
    "deploy\aurago-remote_windows_arm64.exe"
    "deploy\agocli_windows_amd64.exe"
    "deploy\agocli_windows_arm64.exe"
    "deploy\install.sh"
) do (
    if exist %%F set ASSETS=!ASSETS! %%F
)

gh release create "!VERSION!" !ASSETS! --title "AuraGo !VERSION!" --notes "## AuraGo !VERSION!"
if errorlevel 1 (
    echo [ERROR] gh release create failed.
    echo         Check: gh auth status  ^(must be logged in^)
    echo         Tag !VERSION! may already exist: gh release delete !VERSION!
    exit /b 1
)

REM -- [5/5] Cleanup old releases
echo.
echo [5/5] Cleaning up old releases (keeping latest 3)...
for /f "skip=3 tokens=*" %%T in ('gh release list --limit 20 --json tagName --jq ".[].tagName" 2^>nul') do (
    echo     Deleting: %%T
    gh release delete "%%T" --yes --cleanup-tag 2>nul
)

echo.
gh release view "!VERSION!" --json tagName,url --jq "\"  Tag: \" + .tagName + \" | \" + .url"
echo.
echo  --- Release !VERSION! published successfully ---
goto :eof

:build_error
echo.
echo [ERROR] Build failed. Fix compilation errors above.
set GOOS=windows & set GOARCH=amd64
exit /b 1
