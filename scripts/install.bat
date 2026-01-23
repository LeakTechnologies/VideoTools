@echo off
setlocal
chcp 65001 >nul
title VideoTools Windows Installation

echo ========================================================
echo   VideoTools Windows Installation
echo ========================================================
echo.

powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0install.ps1"
exit /b %errorlevel%
