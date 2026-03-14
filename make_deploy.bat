@echo off
REM make_deploy.bat — Build aurago-remote client binaries + push AuraGo source to GitHub
setlocal enabledelayedexpansion
cd /d "%~dp0"

echo ━━━ AuraGo Deploy Builder ━━━
echo.

REM ── Check for Go ─────────────────────────────────────────────────────
where go >nul 2>&1
if errorlevel 1 (
    echo [WARN] Go not found in PATH - skipping binary compilation.
    echo        Run this script with Go installed to build client binaries.
    goto :push
)

REM ── Cross-compile aurago-remote for all client platforms ─────────────
echo [1/2] Cross-compiling aurago-remote client binaries...
if not exist deploy mkdir deploy

set TARGETS=linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

for %%T in (%TARGETS%) do (
    for /f "tokens=1,2 delims=/" %%A in ("%%T") do (
        set GOOS=%%A
        set GOARCH=%%B
        set EXT=
        if "%%A"=="windows" set EXT=.exe
        set OUT=deploy\aurago-remote_%%A_%%B!EXT!
        echo     ^-^> !OUT!
        set CGO_ENABLED=0
        go build -trimpath -ldflags="-s -w" -o "!OUT!" ./cmd/remote/
        if errorlevel 1 (
            echo     [WARN] Failed to build !OUT!
        )
    )
)

echo.

:push
REM ── Push code to GitHub ──────────────────────────────────────────────
echo [2/2] Pushing code changes to GitHub...
git add .
git diff-index --quiet HEAD 2>nul && (
    echo     Nothing to commit.
) || (
    git commit -m "build: deploy artifacts and code updates [skip actions]"
    git push origin main
    echo     Code pushed to GitHub successfully.
)

echo.
echo ━━━ Done ━━━
endlocal
