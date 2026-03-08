@echo off
REM make_deploy.bat — Build AuraGo deployment artifacts (Windows host)
REM Requires: Go toolchain in PATH
setlocal enabledelayedexpansion
cd /d "%~dp0"

set DEPLOY_DIR=deploy
set RESOURCES=resources.dat

echo ━━━ AuraGo Deployment Builder ━━━
echo.

REM ── Clean ──────────────────────────────────────────────────────────────
if exist "%DEPLOY_DIR%" rd /s /q "%DEPLOY_DIR%"
mkdir "%DEPLOY_DIR%"

REM ── Step 1: Pack resources.dat ─────────────────────────────────────────
echo [1/3] Packing resources.dat ...

set TMPRES=%TEMP%\aurago_res_%RANDOM%
mkdir "%TMPRES%\agent_workspace" 2>nul
REM System prompts are embedded in the binary — not shipped on disk.
REM Only the user-editable personalities/ placeholder goes into resources.dat;
REM identity.md is extracted from the binary on first start by EnsurePromptsDir.
mkdir "%TMPRES%\prompts\personalities" 2>nul
xcopy /E /I /Q agent_workspace\skills   "%TMPRES%\agent_workspace\skills"   >nul
REM Remove credential files that must never be deployed
del /Q "%TMPRES%\agent_workspace\skills\client_secret.json" 2>nul
del /Q "%TMPRES%\agent_workspace\skills\client_secrets.json" 2>nul
del /Q "%TMPRES%\agent_workspace\skills\token.json" 2>nul
mkdir "%TMPRES%\agent_workspace\tools" 2>nul
mkdir "%TMPRES%\agent_workspace\workdir\attachments" 2>nul
mkdir "%TMPRES%\data\vectordb" 2>nul
mkdir "%TMPRES%\log" 2>nul

REM Copy clean config template (no secrets, no providers — triggers Setup Wizard)
if exist config_template.yaml (
    copy /Y config_template.yaml "%TMPRES%\config.yaml" >nul
) else (
    echo     WARNING: config_template.yaml not found, using config.yaml
    copy /Y config.yaml "%TMPRES%\config.yaml" >nul
)

REM Include update.sh so binary installs self-update
copy /Y update.sh "%TMPRES%\update.sh" >nul 2>&1

REM Use tar (built into Windows 10+)
tar -czf "%DEPLOY_DIR%\%RESOURCES%" -C "%TMPRES%" .
echo     resources.dat created

rd /s /q "%TMPRES%"

REM ── Step 2: Cross-compile ──────────────────────────────────────────────
echo [2/3] Compiling binaries ...

for %%P in (linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64) do (
    for /f "tokens=1,2 delims=/" %%A in ("%%P") do (
        set CGO_ENABLED=0
        set GOOS=%%A
        set GOARCH=%%B

        set "BIN_TARGET="
        if "%%A"=="linux" if "%%B"=="amd64"  set "BIN_TARGET=amd64"
        if "%%A"=="linux" if "%%B"=="arm64"  set "BIN_TARGET=arm64"

        if "!BIN_TARGET!"=="amd64" (
            REM Standard Linux amd64: put binaries in bin\ for GitHub updates
            if not exist "bin" mkdir "bin"
            echo     bin\aurago_linux
            go build -trimpath -ldflags="-s -w" -o "bin\aurago_linux"          .\cmd\aurago\
            echo     bin\lifeboat_linux
            go build -trimpath -ldflags="-s -w" -o "bin\lifeboat_linux"        .\cmd\lifeboat\
            echo     bin\config-merger_linux
            go build -trimpath -ldflags="-s -w" -o "bin\config-merger_linux"   .\cmd\config-merger\
        ) else if "!BIN_TARGET!"=="arm64" (
            REM Linux arm64: put binaries in bin\ with _arm64 suffix
            if not exist "bin" mkdir "bin"
            echo     bin\aurago_linux_arm64
            go build -trimpath -ldflags="-s -w" -o "bin\aurago_linux_arm64"         .\cmd\aurago\
            echo     bin\lifeboat_linux_arm64
            go build -trimpath -ldflags="-s -w" -o "bin\lifeboat_linux_arm64"       .\cmd\lifeboat\
            echo     bin\config-merger_linux_arm64
            go build -trimpath -ldflags="-s -w" -o "bin\config-merger_linux_arm64"  .\cmd\config-merger\
        ) else (
            REM Other targets go to deploy\
            set "EXT="
            if "%%A"=="windows" set "EXT=.exe"
            set "OUT=%DEPLOY_DIR%\aurago_%%A_%%B!EXT!"
            echo     !OUT!
            go build -trimpath -ldflags="-s -w" -o "!OUT!" .\cmd\aurago\
        )
    )
)

REM ── Step 3: Copy install script ────────────────────────────────────────
echo [3/3] Copying install script ...
copy /Y install.sh "%DEPLOY_DIR%\install.sh" >nul 2>&1

echo.
echo ━━━ Done! Artifacts in %DEPLOY_DIR%\ ━━━
dir "%DEPLOY_DIR%"

REM ── Step 4: Commit code & upload binaries as GitHub Release ─────────────
echo.
echo [4/5] Committing code changes to GitHub ...
git add .
git diff-index --quiet HEAD || (
    git commit -m "build: code updates [skip actions]" >nul
    git push origin main
    echo     Code pushed to GitHub successfully.
)

echo.
echo [5/5] Creating GitHub Release with binaries ...

REM Build a date-based tag  (v2026.0308.HHMM)
set "TAG=v%date:~6,4%.%date:~3,2%%date:~0,2%.%time:~0,2%%time:~3,2%"
set "TAG=%TAG: =0%"

gh release create %TAG% ^
    --title "AuraGo %TAG%" ^
    --notes "Auto-built on %date% %time%" ^
    --latest ^
    "bin\aurago_linux" ^
    "bin\aurago_linux_arm64" ^
    "bin\lifeboat_linux" ^
    "bin\lifeboat_linux_arm64" ^
    "bin\config-merger_linux" ^
    "bin\config-merger_linux_arm64" ^
    "%DEPLOY_DIR%\%RESOURCES%"

if errorlevel 1 (
    echo     ERROR: Failed to create GitHub Release.
    echo     Make sure gh is authenticated: gh auth status
) else (
    echo     GitHub Release "%TAG%" created with all binaries.
)

REM ── Cleanup: keep only the 3 most recent releases ──────────────────────
echo.
echo [cleanup] Pruning old releases (keeping latest 3) ...
set "SKIP=0"
for /f "tokens=1" %%r in ('gh release list --limit 100 --json tagName --jq ".[].tagName"') do (
    set /a SKIP+=1
    if !SKIP! GTR 3 (
        echo     Deleting old release %%r ...
        gh release delete %%r --yes --cleanup-tag 2>nul
    )
)
echo     Done.
endlocal
