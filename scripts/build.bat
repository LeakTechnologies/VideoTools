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
    echo [ERROR] Go is not installed or not in PATH.
    echo Download Go from: https://go.dev/dl/
    exit /b 1
)

echo [INFO] Go version:
go version
echo.

REM ----------------------------
REM Check for winget (required for auto-install)
REM ----------------------------
set WINGET_AVAILABLE=0
where winget >nul 2>&1
set WINGET_CHECK=!ERRORLEVEL!
if !WINGET_CHECK! equ 0 (
    set WINGET_AVAILABLE=1
    echo [OK] winget found ^(automatic installation available^)
) else (
    echo [WARN] winget not found ^(manual installation will be required^)
    echo        To enable automatic installation, update to Windows 10 1809+ or Windows 11
)
echo.

REM ----------------------------
REM Check for Git (recommended for development)
REM ----------------------------
where git >nul 2>&1
set GIT_CHECK=!ERRORLEVEL!
if !GIT_CHECK! equ 0 (
    echo [OK] Git found
    git --version
) else (
    echo [WARN] Git not found ^(recommended for development^)

    if !WINGET_AVAILABLE! equ 1 (
        echo.
        echo Would you like to install Git automatically? ^(Y/N^):
        set /p install_git=

        if /I "!install_git!"=="Y" (
            echo.
            echo [INFO] Installing Git via winget...
            winget install -e --id=Git.Git
            set GIT_INSTALL_RESULT=!ERRORLEVEL!

            if !GIT_INSTALL_RESULT! equ 0 (
                echo [OK] Git installed successfully!
                echo Please restart your terminal and run this script again.
                exit /b 0
            ) else (
                echo [ERROR] Failed to install Git automatically.
                echo Please install manually from: https://git-scm.com/
            )
        )
    ) else (
        echo Please install Git from: https://git-scm.com/
    )
)
echo.

REM ----------------------------
REM Check for GCC (required for CGO)
REM ----------------------------
where gcc >nul 2>&1
set GCC_CHECK=!ERRORLEVEL!
if !GCC_CHECK! neq 0 (
    echo [WARN] GCC not found. CGO requires a C compiler.
    echo.
    echo VideoTools requires MinGW-w64 to build on Windows.
    echo.

    if !WINGET_AVAILABLE! equ 1 (
        echo Would you like to install MinGW-w64 automatically? ^(Y/N^):
        set /p install_gcc=

        if /I "!install_gcc!"=="Y" (
            echo.
            echo [INFO] Installing MinGW-w64 via winget...
            echo This may take a few minutes...
            winget install -e --id=MSYS2.MSYS2
            set MSYS2_INSTALL_RESULT=!ERRORLEVEL!

        REM Check if MSYS2 is installed (either newly installed or already present)
        if exist "C:\msys64\usr\bin\bash.exe" (
            set MSYS2_INSTALL_RESULT=0
        )

        if !MSYS2_INSTALL_RESULT! equ 0 (
            echo [OK] MSYS2 installed successfully!
            echo.
            echo [INFO] Installing GCC toolchain...
            C:\msys64\usr\bin\bash.exe -lc "pacman -S --noconfirm mingw-w64-x86_64-gcc"
            set GCC_INSTALL_RESULT=!ERRORLEVEL!

            if !GCC_INSTALL_RESULT! equ 0 (
                echo [OK] GCC installed successfully!
                echo.
                echo [INFO] Adding MinGW to PATH for this session...
                set "PATH=C:\msys64\mingw64\bin;!PATH!"

                echo [OK] Setup complete! Continuing with build...
                echo.
            ) else (
                echo [ERROR] Failed to install GCC. Please install manually.
                echo Visit: https://www.msys2.org/
                exit /b 1
            )
        ) else (
            echo [ERROR] Failed to install MSYS2. Please install manually.
            echo Visit: https://www.msys2.org/
            exit /b 1
        )
        ) else (
            echo Skipping automatic installation.
        )
    ) else (
        echo winget is not available on this system.
    )

    REM Show manual installation instructions if we get here
    echo.
    echo [ERROR] GCC is required to build VideoTools on Windows.
    echo.
    echo Please install MinGW-w64 manually:
    echo   1. Install MSYS2 from https://www.msys2.org/
    echo   2. Run: pacman -S mingw-w64-x86_64-gcc
    echo   3. Add C:\msys64\mingw64\bin to your PATH
    echo   4. Restart your terminal and run this script again
    echo.
    exit /b 1
    )
) else (
    echo [OK] GCC found:
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
echo [INFO] Cleaning previous Windows build...
if exist VideoTools.exe del /f VideoTools.exe
echo [OK] Cache cleaned
echo.

REM ----------------------------
REM Download go dependencies
REM ----------------------------
echo [INFO] Downloading dependencies...
go mod download
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Failed to download dependencies.
    exit /b 1
)
echo [OK] Dependencies downloaded
echo.

REM ----------------------------
REM Embed Windows icon (if windres is available)
REM ----------------------------
where windres >nul 2>&1
if %ERRORLEVEL% equ 0 (
    windres scripts\videotools.rc -O coff -o videotools_windows_amd64.syso
) else (
    echo [WARN] windres not found; Windows icon will not be embedded in the EXE
)
echo.

REM ----------------------------
REM Build VideoTools (Windows GUI mode)
REM Note: CGO is required for Fyne/OpenGL on Windows
REM ----------------------------
echo [INFO] Building VideoTools.exe...

REM Enable CGO for Windows build (required for Fyne)
set CGO_ENABLED=1

REM Detect CPU cores for parallel compilation
for /f "tokens=2 delims==" %%I in ('wmic cpu get NumberOfLogicalProcessors /value ^| find "="') do set NUM_CORES=%%I
if not defined NUM_CORES set NUM_CORES=4
echo [INFO] Using %NUM_CORES% parallel build processes

REM Build with optimizations:
REM -p: Parallel build processes (use all CPU cores)
REM -trimpath: Remove absolute paths (faster builds, smaller binary)
REM -ldflags: Strip debug info (-s -w) and use Windows GUI mode (-H windowsgui)
go build ^
    -p %NUM_CORES% ^
    -trimpath ^
    -ldflags="-H windowsgui -s -w" ^
    -o VideoTools.exe ^
    .

if %ERRORLEVEL% neq 0 (
    echo [ERROR] Build failed!
    popd
    exit /b 1
)

echo [OK] Build successful!
echo.

REM ----------------------------
REM Show file size
REM ----------------------------
for %%A in (VideoTools.exe) do set FILESIZE=%%~zA
echo Output: VideoTools.exe  ^(Size: !FILESIZE! bytes^)
echo.

REM ----------------------------
REM Offer to run FFmpeg setup
REM ----------------------------
if exist "%~dp0setup-windows.ps1" (
    echo Would you like to download FFmpeg now? ^(Y/N^):
    set /p choice=

    if /I "!choice!"=="Y" (
        powershell -ExecutionPolicy Bypass -File "%~dp0setup-windows.ps1" -Portable
    ) else (
        echo Skipping FFmpeg setup. You can run setup-windows.ps1 later.
    )
)

popd
exit /b 0
