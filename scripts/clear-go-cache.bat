@echo off
setlocal
echo ============================================================
echo   VideoTools Go Cache Cleaner (Windows)
echo ============================================================
echo.

where go >nul 2>&1
if %ERRORLEVEL% neq 0 (
  echo Go is not installed or not in PATH. Skipping go clean.
) else (
  echo Running: go clean -cache -modcache -testcache
  go clean -cache -modcache -testcache
)

set CACHE_DIR=%LOCALAPPDATA%\go-build
if exist "%CACHE_DIR%" (
  echo Removing build cache dir: "%CACHE_DIR%"
  rmdir /s /q "%CACHE_DIR%"
) else (
  echo No cache directory found at "%CACHE_DIR%" (nothing to remove).
)

echo.
echo Done. Re-run scripts\build.bat to rebuild VideoTools.
endlocal
