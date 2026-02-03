@echo off
setlocal
chcp 65001 >nul
title VideoTools Windows Dependency Installer

echo ========================================================
echo   VideoTools Windows Installation
echo   Delegating to PowerShell for full dependency setup
echo ========================================================
echo.

powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0..\\..\\windows\\support\\install-deps-windows.ps1"
set EXIT_CODE=%errorlevel%

if not %EXIT_CODE%==0 (
  echo.
  echo Dependency installer failed with exit code %EXIT_CODE%.
  pause
  exit /b %EXIT_CODE%
)

echo.
echo Done. Restart your terminal to refresh PATH.
pause
exit /b 0
