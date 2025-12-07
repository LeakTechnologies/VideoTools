@echo off
setlocal
chcp 65001 >nul
title VideoTools Windows Dependency Installer

echo ========================================================
echo   VideoTools Windows Dependency Installer (.bat)
echo   Installs Go, MinGW (GCC), Git, and FFmpeg
echo ========================================================
echo.

REM Prefer Chocolatey if available; otherwise fall back to winget.
where choco >nul 2>&1
if %errorlevel%==0 (
    echo Using Chocolatey...
    call :install_choco
    goto :verify
)

where winget >nul 2>&1
if %errorlevel%==0 (
    echo Chocolatey not found; using winget...
    call :install_winget
    goto :verify
)

echo Neither Chocolatey nor winget found.
echo Please install Chocolatey (recommended): https://chocolatey.org/install
echo Then re-run this script.
pause
exit /b 1

:install_choco
echo.
echo Installing dependencies via Chocolatey...
choco install -y golang mingw git ffmpeg
goto :eof

:install_winget
echo.
echo Installing dependencies via winget...
REM Winget package IDs can vary; these are common defaults.
winget install -e --id GoLang.Go
winget install -e --id Git.Git
winget install -e --id GnuWin32.Mingw
winget install -e --id Gyan.FFmpeg
goto :eof

:verify
echo.
echo ========================================================
echo   Verifying installs
echo ========================================================
where go >nul 2>&1 && go version
where gcc >nul 2>&1 && gcc --version | findstr /R /C:"gcc"
where git >nul 2>&1 && git --version
where ffmpeg >nul 2>&1 && ffmpeg -version | head -n 1

echo.
echo Done. If any tool is missing, ensure its bin folder is in PATH
echo (restart terminal after installation).
pause
exit /b 0
