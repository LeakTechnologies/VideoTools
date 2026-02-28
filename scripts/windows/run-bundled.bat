@echo off
setlocal
set "BASE_DIR=%~dp0"
set "DEPS_DIR=%BASE_DIR%deps"

if exist "%DEPS_DIR%\bin" set "PATH=%DEPS_DIR%\bin;%PATH%"
if exist "%DEPS_DIR%\tessdata" set "TESSDATA_PREFIX=%DEPS_DIR%\tessdata"

"%BASE_DIR%VideoTools.exe" %*
