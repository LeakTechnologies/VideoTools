@echo off
REM Uses MSYS2 toolchain for build dependencies
setlocal
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0install.ps1" %*
exit /b %errorlevel%

