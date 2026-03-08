@echo off
setlocal enabledelayedexpansion
set GOOS=windows
set GOARCH=amd64
echo Building AuraGo for Windows...

if not exist bin mkdir bin

go build -o aurago.exe ./cmd/aurago
if %ERRORLEVEL% neq 0 (
    echo Build of aurago failed!
    exit /b %ERRORLEVEL%
)

go build -o bin\lifeboat.exe ./cmd/lifeboat
if %ERRORLEVEL% neq 0 (
    echo Build of lifeboat failed!
    exit /b %ERRORLEVEL%
)

REM Auto-load master key from .env if not already set
if not defined AURAGO_MASTER_KEY (
    if exist .env (
        for /f "usebackq tokens=1,2 delims==" %%A in (".env") do (
            if "%%A"=="AURAGO_MASTER_KEY" set "AURAGO_MASTER_KEY=%%B"
        )
    )
)

if not defined AURAGO_MASTER_KEY (
    echo.
    echo  [33mWARN: AURAGO_MASTER_KEY is not set. [0m
    echo AuraGo needs a 64-character hex key to encrypt its vault.
    set /p CONFIRM="Generate and save a new key to .env automatically? [y/N]: "
    if /i "!CONFIRM:~0,1!"=="y" (
        for /f "tokens=*" %%F in ('python -c "import secrets; print(secrets.token_hex(32))" 2^>nul') do set "GEN_KEY=%%F"
        if not defined GEN_KEY (
            echo.
            echo  [31mERROR: Failed to generate key. Ensure Python is installed. [0m
            echo Manual setup:  python -c "import secrets; print(secrets.token_hex(32))"
            exit /b 1
        )
        echo AURAGO_MASTER_KEY=!GEN_KEY!>> .env
        set "AURAGO_MASTER_KEY=!GEN_KEY!"
        echo.
        echo  [32mOK: New key generated and saved to .env [0m
    ) else (
        echo.
        echo  [31mERROR: AURAGO_MASTER_KEY is required to start. [0m
        echo Set it via:  setx AURAGO_MASTER_KEY your_key_here
        exit /b 1
    )
)

echo Starting AuraGo in debug mode...
:loop
.\aurago.exe --debug --config config_debug.yaml
if %ERRORLEVEL% equ 42 (
    echo.
    echo [UI Restart] AuraGo wird neu gestartet...
    goto loop
)
