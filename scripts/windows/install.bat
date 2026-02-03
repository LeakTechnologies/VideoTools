@echo off
REM Uses MSYS2 toolchain for build dependencies
setlocal
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0install.ps1" %*
set exitcode=%errorlevel%
if not %exitcode%==0 (
  echo [ERROR] Install failed with exit code %exitcode%.
  echo Press any key to close...
  pause >nul
)
exit /b %exitcode%


