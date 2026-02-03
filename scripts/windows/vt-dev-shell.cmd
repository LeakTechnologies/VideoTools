@echo off
setlocal

set "PROJECT_ROOT=%~dp0..\.."
for %%I in ("%PROJECT_ROOT%") do set "PROJECT_ROOT=%%~fI"

set "MSYS2_ROOT=%PROJECT_ROOT%\Tools\msys64"
if not "%VT_MSYS2_ROOT%"=="" (
    set "MSYS2_ROOT=%VT_MSYS2_ROOT%"
)

set "MSYS2_FLAVOR=ucrt64"
if not "%VT_MSYS2_FLAVOR%"=="" (
    set "MSYS2_FLAVOR=%VT_MSYS2_FLAVOR%"
)

set "PATH=%MSYS2_ROOT%\%MSYS2_FLAVOR%\bin;%MSYS2_ROOT%\usr\bin;%PATH%"
echo VideoTools dev shell ready.
echo MSYS2 root: %MSYS2_ROOT%
echo MSYS2 flavor: %MSYS2_FLAVOR%
echo.
cmd /k
