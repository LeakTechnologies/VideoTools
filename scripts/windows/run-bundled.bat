@echo off
setlocal
set "BASE_DIR=%~dp0"
set "DEPS_DIR=%BASE_DIR%deps"

if exist "%DEPS_DIR%\bin" set "PATH=%DEPS_DIR%\bin;%PATH%"
if exist "%DEPS_DIR%\tessdata" set "TESSDATA_PREFIX=%DEPS_DIR%\tessdata"
if exist "%DEPS_DIR%\lib\gstreamer-1.0" (
  set "GST_PLUGIN_PATH=%DEPS_DIR%\lib\gstreamer-1.0"
  set "GST_PLUGIN_SYSTEM_PATH_1_0=%DEPS_DIR%\lib\gstreamer-1.0"
)
if exist "%DEPS_DIR%\bin\gst-plugin-scanner.exe" set "GST_PLUGIN_SCANNER=%DEPS_DIR%\bin\gst-plugin-scanner.exe"
if exist "%DEPS_DIR%\lib\gio\modules" set "GIO_EXTRA_MODULES=%DEPS_DIR%\lib\gio\modules"

"%BASE_DIR%VideoTools.exe" %*
