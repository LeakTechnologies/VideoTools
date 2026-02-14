@echo off
REM VideoTools Uninstaller for Windows
REM Preserves shared dependencies by default
setlocal

echo.
echo =============================================================
echo   VideoTools Uninstaller for Windows
echo =============================================================
echo.
echo This uninstaller removes VideoTools while preserving:
echo   - FFmpeg (NEVER removed - system dependency managed by user)
echo   - GStreamer (media framework - used by other apps)
echo   - Go (programming language used by other tools)
echo.
echo Use -RemoveAll to remove ALL VideoTools-managed components
echo Use -Force to skip confirmation prompts
echo.
echo Use -RemoveAll to remove ALL components
echo Use -RemoveFFmpeg to remove bundled FFmpeg binaries
echo Use -Force to skip confirmation prompts
echo.

powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0uninstall.ps1" %*
set exitcode=%errorlevel%

if not %exitcode%==0 (
  echo.
  echo [ERROR] Uninstall failed with exit code %exitcode%.
  echo Press any key to close...
  pause >nul
) else (
  echo.
  echo [SUCCESS] VideoTools uninstall completed.
  echo Press any key to close...
  pause >nul
)

exit /b %exitcode%