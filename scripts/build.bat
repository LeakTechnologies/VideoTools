@echo off
setlocal enabledelayedexpansion

REM ----------------------------
REM Delegate to PowerShell build script
REM ----------------------------
set "PS1_PATH=%~dp0build.ps1"
if not exist "%PS1_PATH%" (
    echo [ERROR] build.ps1 not found at %PS1_PATH%
    exit /b 1
)

net session >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [INFO] Elevation required for Windows build tools.
    echo        Approve the UAC prompt to continue.
    powershell -NoProfile -Command "Start-Process -FilePath 'powershell.exe' -ArgumentList '-NoProfile -NoExit -ExecutionPolicy Bypass -File \"%PS1_PATH%\" %*' -Verb RunAs"
    exit /b 0
)

powershell -NoProfile -ExecutionPolicy Bypass -File "%PS1_PATH%" %*
exit /b %ERRORLEVEL%
