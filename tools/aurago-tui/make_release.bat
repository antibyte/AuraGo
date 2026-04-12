@echo off
setlocal EnableDelayedExpansion

echo ===========================================
echo  Building aurago-tui release binaries
echo ===========================================
echo.

REM Check for cargo
where cargo >nul 2>nul
if errorlevel 1 (
    echo ERROR: cargo not found in PATH.
    echo.
    echo Please install Rust from https://rustup.rs/
    echo On Windows you ALSO need one of the following:
    echo   - Visual Studio Build Tools with "Desktop development with C++"
    echo   - MSYS2 / MinGW-w64
    echo.
    echo Alternatively, download a pre-built binary from:
    echo   https://github.com/antibyte/AuraGo/releases/tag/aurago-tui-rolling
    exit /b 1
)

cd /d "%~dp0"

set TARGET=x86_64-pc-windows-msvc

echo Building for %TARGET% ...
cargo build --release --target %TARGET%
if errorlevel 1 (
    echo.
    echo BUILD FAILED.
    echo If the linker is missing, install Visual Studio Build Tools or MinGW,
    echo or use the pre-built binary from GitHub Releases.
    exit /b 1
)

set SRC=target\%TARGET%\release\aurago-tui.exe
set DEST=..\..\bin\aurago-tui-windows-amd64.exe

if not exist "..\..\bin" mkdir "..\..\bin"
copy /Y "%SRC%" "%DEST%" >nul

echo.
echo SUCCESS: Binary copied to %DEST%
echo.
endlocal
