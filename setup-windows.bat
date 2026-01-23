@echo off
REM VideoTools Windows Setup Launcher
REM This batch file launches the scripts installer entrypoint

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

REM Run the scripts entrypoint (keeps Windows workflow consistent)
call "%~dp0scripts\install.bat"

echo.
pause
