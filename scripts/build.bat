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
REM Check for GCC (required for CGO)
REM ----------------------------
where gcc >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo ⚠️  WARNING: GCC not found. CGO requires a C compiler.
    echo.
    echo VideoTools requires MinGW-w64 to build on Windows.
    echo.
    echo Would you like to install MinGW-w64 automatically? (Y/N):
    set /p install_gcc=

    if /I "!install_gcc!"=="Y" (
        echo.
        echo 📥 Installing MinGW-w64 via winget...
        echo This may take a few minutes...
        winget install -e --id=MSYS2.MSYS2

        if !ERRORLEVEL! equ 0 (
            echo ✓ MSYS2 installed successfully!
            echo.
            echo 📦 Installing GCC toolchain...
            C:\msys64\usr\bin\bash.exe -lc "pacman -S --noconfirm mingw-w64-x86_64-gcc"

            if !ERRORLEVEL! equ 0 (
                echo ✓ GCC installed successfully!
                echo.
                echo 🔧 Adding MinGW to PATH for this session...
                set "PATH=C:\msys64\mingw64\bin;!PATH!"

                echo ✓ Setup complete! Continuing with build...
                echo.
            ) else (
                echo ❌ Failed to install GCC. Please install manually.
                echo Visit: https://www.msys2.org/
                exit /b 1
            )
        ) else (
            echo ❌ Failed to install MSYS2. Please install manually.
            echo Visit: https://www.msys2.org/
            exit /b 1
        )
    ) else (
        echo.
        echo ❌ GCC is required to build VideoTools on Windows.
        echo.
        echo Please install MinGW-w64:
        echo   1. Install MSYS2 from https://www.msys2.org/
        echo   2. Run: pacman -S mingw-w64-x86_64-gcc
        echo   3. Add C:\msys64\mingw64\bin to your PATH
        echo.
        echo Or install via winget:
        echo   winget install MSYS2.MSYS2
        echo   C:\msys64\usr\bin\bash.exe -lc "pacman -S --noconfirm mingw-w64-x86_64-gcc"
        exit /b 1
    )
) else (
    echo ✓ GCC found:
    gcc --version | findstr /C:"gcc"
    echo.
)

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
REM Note: CGO is required for Fyne/OpenGL on Windows
REM ----------------------------
echo 🔨 Building VideoTools.exe...

REM Enable CGO for Windows build (required for Fyne)
set CGO_ENABLED=1

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
