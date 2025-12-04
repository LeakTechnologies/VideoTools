@echo off
setlocal enabledelayedexpansion

echo ================================================================
echo   VideoTools Windows Build Script
echo ================================================================
echo.

REM ----------------------------
REM Detect Go
REM ----------------------------
where go >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo ❌ ERROR: Go is not installed or not in PATH.
    echo Download Go from: https://go.dev/dl/
    exit /b 1
)

echo 📦 Go version:
go version
echo.

REM ----------------------------
REM Move to project root
REM ----------------------------
pushd "%~dp0\.."

REM ----------------------------
REM Clean previous build
REM ----------------------------
echo 🧹 Cleaning previous Windows build...
if exist VideoTools.exe del /f VideoTools.exe
echo ✓ Cache cleaned
echo.

REM ----------------------------
REM Download go dependencies
REM ----------------------------
echo ⬇️  Downloading dependencies...
go mod download
if %ERRORLEVEL% neq 0 (
    echo ❌ Failed to download dependencies.
    exit /b 1
)
echo ✓ Dependencies downloaded
echo.

REM ----------------------------
REM Build VideoTools (Windows GUI mode)
REM Equivalent to:
REM go build -ldflags="-H windowsgui -s -w" -o VideoTools.exe .
REM ----------------------------
echo 🔨 Building VideoTools.exe...

go build ^
    -ldflags="-H windowsgui -s -w" ^
    -o VideoTools.exe ^
    .

if %ERRORLEVEL% neq 0 (
    echo ❌ Build failed!
    popd
    exit /b 1
)

echo ✓ Build successful!
echo.

REM ----------------------------
REM Show file size
REM ----------------------------
for %%A in (VideoTools.exe) do set FILESIZE=%%~zA
echo Output: VideoTools.exe  (Size: !FILESIZE! bytes)
echo.

REM ----------------------------
REM Offer to run FFmpeg setup
REM ----------------------------
if exist "%~dp0setup-windows.ps1" (
    echo Would you like to download FFmpeg now? (Y/N):
    set /p choice=

    if /I "!choice!"=="Y" (
        powershell -ExecutionPolicy Bypass -File "%~dp0setup-windows.ps1" -Portable
    ) else (
        echo Skipping FFmpeg setup. You can run setup-windows.ps1 later.
    )
)

popd
exit /b 0
