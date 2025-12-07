@echo off
setlocal enabledelayedexpansion
chcp 65001 >nul
title AV1 / H265 Converter — Bitrate Menu

REM Simple ffmpeg/ffprobe check
where ffmpeg >nul 2>&1 && where ffprobe >nul 2>&1
if errorlevel 1 (
    echo ffmpeg/ffprobe not found in PATH. Install via winget/choco/scoop or run setup-windows.
    pause
    exit /b 1
)

set "SRC=%~dp0"
set "OUT=%SRC%Converted"
if not exist "%OUT%" md "%OUT%"

cls
echo.
echo ========================================================
echo   Choose codec:
echo   1 = AV1  (av1_amf hardware)
echo   2 = H265 (hevc_amf hardware)
echo ========================================================
choice /c 12 /n /m "Press 1 or 2: "

set "codec=av1_amf"
set "codec_name=AV1"
if errorlevel 2 (
    set "codec=hevc_amf"
    set "codec_name=H265"
)

set "lossless=0"
if "%codec%"=="hevc_amf" (
    echo.
    echo Optional: H.265 lossless uses CPU libx265 and ignores bitrate/CRF.
    choice /c YN /n /m "Use H.265 lossless (libx265)? (Y/N): "
    if not errorlevel 2 (
        set "lossless=1"
        set "codec=libx265"
        set "codec_name=H265 lossless (CPU)"
    )
)

set "BITRATE="
if "%lossless%"=="0" (
    echo.
    echo Select target bitrate for %codec_name%:
    if "%codec%"=="av1_amf" (
        echo   1 = 1200k  (Grok 1080p sweet spot)
        echo   2 = 1400k  (safe default)
        echo   3 = 1800k  (extra headroom)
        choice /c 123C /n /m "Pick 1-3 or C for custom: "
        if errorlevel 4 (
            set /p BITRATE="Enter bitrate (e.g. 1600k or 8M): "
        ) else if errorlevel 3 (
            set "BITRATE=1800k"
        ) else if errorlevel 2 (
            set "BITRATE=1400k"
        ) else (
            set "BITRATE=1200k"
        )
    ) else (
        echo   1 = 1800k  (lean 1080p H.265)
        echo   2 = 2000k  (balanced default)
        echo   3 = 2400k  (noisy sources)
        choice /c 123C /n /m "Pick 1-3 or C for custom: "
        if errorlevel 4 (
            set /p BITRATE="Enter bitrate (e.g. 2200k or 10M): "
        ) else if errorlevel 3 (
            set "BITRATE=2400k"
        ) else if errorlevel 2 (
            set "BITRATE=2000k"
        ) else (
            set "BITRATE=1800k"
        )
    )
)

echo.
echo Using %codec_name% output to "%OUT%"
if "%lossless%"=="0" (
    echo Target bitrate: %BITRATE%
) else (
    echo Mode: lossless (libx265 -x265-params lossless=1)
)
echo.

set "found=0"

for %%f in ("%SRC%*.mkv" "%SRC%*.mp4" "%SRC%*.mov" "%SRC%*.avi" "%SRC%*.wmv" "%SRC%*.mpg" "%SRC%*.mpeg" "%SRC%*.ts" "%SRC%*.m2ts") do (
    if exist "%%f" (
        set /a found+=1
        if exist "%OUT%\%%~nf__cv.mkv" (
            echo [SKIP] "%%~nxf"
        ) else (
            echo Encoding: "%%~nxf"
            for /f %%h in ('ffprobe -v error -select_streams v^:0 -show_entries stream^=height -of csv^=p^=0 "%%f" 2^>nul') do set h=%%h

            if "%lossless%"=="1" (
                if !h! LSS 1080 (
                    ffmpeg -i "%%f" -vf scale=1920:1080:flags=lanczos -c:v libx265 -preset medium -x265-params lossless=1 -c:a copy "%OUT%\%%~nf__cv.mkv"
                ) else (
                    ffmpeg -i "%%f" -c:v libx265 -preset medium -x265-params lossless=1 -c:a copy "%OUT%\%%~nf__cv.mkv"
                )
            ) else (
                if !h! LSS 1080 (
                    ffmpeg -i "%%f" -vf scale=1920:1080:flags=lanczos -c:v %codec% -b:v %BITRATE% -maxrate %BITRATE% -bufsize 3600k -c:a copy "%OUT%\%%~nf__cv.mkv"
                ) else (
                    ffmpeg -i "%%f" -c:v %codec% -b:v %BITRATE% -maxrate %BITRATE% -bufsize 3600k -c:a copy "%OUT%\%%~nf__cv.mkv"
                )
            )
            echo   DONE: "%%~nf__cv.mkv"
        )
        echo.
    )
)

if %found%==0 echo No files found.

echo.
echo ========================================================
echo All finished!
echo ========================================================
pause
