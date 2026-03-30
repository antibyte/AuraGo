@echo off
REM make_deploy.bat — Push AuraGo source to GitHub (for development)
REM For full release builds, use make_release.bat instead.
setlocal enabledelayedexpansion
cd /d "%~dp0"

echo --- AuraGo Deploy (Dev Push) ---
echo.

REM ── Push code to GitHub ──────────────────────────────────────────────
echo Pushing code changes to GitHub...
git add .
git diff-index --quiet HEAD 2>nul && (
    echo     Nothing to commit.
) || (
    git commit -m "build: code updates [skip actions]"
    git push origin main
    echo     Code pushed to GitHub successfully.
)

echo.
echo --- Done ---
echo Note: For full release builds with binaries, run make_release.bat
endlocal
