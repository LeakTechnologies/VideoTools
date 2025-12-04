@echo off
REM VideoTools Windows Setup Launcher
REM This batch file launches the PowerShell setup script

echo ================================================================
echo   VideoTools Windows Setup
echo ================================================================
echo.

REM Check if PowerShell is available
where powershell >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo ERROR: PowerShell is not found on this system.
    echo Please install PowerShell or manually download FFmpeg from:
    echo https://github.com/BtbN/FFmpeg-Builds/releases
    echo.
    pause
    exit /b 1
)

echo Starting setup...
echo.

REM Run the PowerShell script with portable installation by default
powershell -ExecutionPolicy Bypass -File "%~dp0scripts\setup-windows.ps1" -Portable

echo.
pause
