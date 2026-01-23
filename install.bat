@echo off
setlocal
chcp 65001 >nul
title VideoTools Windows Installation

echo ========================================================
echo   VideoTools Windows Installation
echo ========================================================
echo.

call "%~dp0scripts\setup-windows.bat"
exit /b %errorlevel%
