@echo off
REM make_deploy.bat — Push AuraGo source code to GitHub (server compiles via update.sh)
setlocal enabledelayedexpansion
cd /d "%~dp0"

echo ━━━ AuraGo Push-Only Deploy ━━━
echo.

REM ── Push code to GitHub ──────────────────────────────────────────────
echo [1/1] Pushing code changes to GitHub ...
git add .
git diff-index --quiet HEAD 2>nul && (
    echo     Nothing to commit.
) || (
    git commit -m "build: code updates [skip actions]"
    git push origin main
    echo     Code pushed to GitHub successfully.
)

echo.
echo ━━━ Done ━━━
endlocal
