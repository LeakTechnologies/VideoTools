@echo off
setlocal enabledelayedexpansion
chcp 65001 >nul
title VideoTools Helper — MKV Encode Questionnaire

REM ------------------------------------------------------------
REM Converts videos to MKV with selectable codec/bitrate, while
REM preserving all streams (audio/subtitle/attachments) and
REM copying color tags + pixel format to avoid color shifts.
REM ------------------------------------------------------------

where ffmpeg >nul 2>&1 && where ffprobe >nul 2>&1
if errorlevel 1 (
    echo ffmpeg or ffprobe not found in PATH. Install them, then rerun.
    pause
    exit /b 1
)

echo.
set /p INPUT_PATH="Drag a file or folder here, then press Enter: "
if not defined INPUT_PATH (
    echo No input provided. Exiting.
    exit /b 1
)
set "INPUT_PATH=%INPUT_PATH:"=%"

if exist "%INPUT_PATH%" (
    if exist "%INPUT_PATH%\" (
        set "MODE=folder"
        set "INPUT_DIR=%INPUT_PATH%"
    ) else (
        set "MODE=file"
        set "INPUT_FILE=%INPUT_PATH%"
        set "INPUT_DIR=%~dpINPUT_PATH%"
    )
) else (
    echo Path not found: %INPUT_PATH%
    exit /b 1
)

set /p OUTPUT_DIR="Output folder (Enter for default 'Converted' next to inputs): "
if not defined OUTPUT_DIR (
    if "%MODE%"=="file" (
        set "OUTPUT_DIR=%~dpINPUT_FILE%Converted"
    ) else (
        set "OUTPUT_DIR=%INPUT_DIR%Converted"
    )
)
if not exist "%OUTPUT_DIR%" md "%OUTPUT_DIR%"

echo.
echo ========================================================
echo Select video codec:
echo   1 = AV1 (libaom-av1)
echo   2 = H.265/HEVC (libx265)
echo   3 = Copy video (remux only)
echo ========================================================
choice /c 123 /n /m "Choose 1-3: "
if errorlevel 3 (
    set "VCODEC=copy"
    set "VCODEC_NAME=Copy"
) else if errorlevel 2 (
    set "VCODEC=libx265"
    set "VCODEC_NAME=H.265"
) else (
    set "VCODEC=libaom-av1"
    set "VCODEC_NAME=AV1"
)

set "LOSSLESS=0"
if "%VCODEC%"=="libx265" (
    choice /c YN /n /m "Use H.265 lossless (CRF 0, lossless=1)? (Y/N): "
    if not errorlevel 2 set "LOSSLESS=1"
) else if "%VCODEC%"=="libaom-av1" (
    choice /c YN /n /m "Use AV1 lossless (CRF 0, -b:v 0)? (Y/N): "
    if not errorlevel 2 set "LOSSLESS=1"
)

set "BITRATE="
if "%VCODEC%"=="copy" (
    set "MODE_TEXT=Remux (no re-encode)"
) else if "%LOSSLESS%"=="1" (
    set "MODE_TEXT=Lossless"
) else (
    echo.
    echo Enter target video bitrate (examples: 1400k, 2000k, 8M):
    set /p BITRATE="Bitrate: "
    if not defined BITRATE (
        echo No bitrate entered, defaulting to 2000k.
        set "BITRATE=2000k"
    )
    set "MODE_TEXT=Bitrate %BITRATE%"
)

echo.
echo ========================================================
echo   Input : %INPUT_PATH%
echo   Output: %OUTPUT_DIR%
echo   Codec : %VCODEC_NAME% (%MODE_TEXT%)
echo ========================================================
echo.
choice /c YN /n /m "Proceed? (Y/N): "
if errorlevel 2 exit /b 0

REM Build file list
set "LIST_FILE=%temp%\\vt_list.txt"
if exist "%LIST_FILE%" del "%LIST_FILE%"

if "%MODE%"=="file" (
    echo "%INPUT_FILE%">"%LIST_FILE%"
) else (
    for %%f in ("%INPUT_DIR%\*.mkv" "%INPUT_DIR%\*.mp4" "%INPUT_DIR%\*.mov" "%INPUT_DIR%\*.avi" "%INPUT_DIR%\*.mpg" "%INPUT_DIR%\*.mpeg" "%INPUT_DIR%\*.ts" "%INPUT_DIR%\*.m2ts" "%INPUT_DIR%\*.wmv") do (
        if exist "%%~f" echo "%%~f">>"%LIST_FILE%"
    )
)

for /f "usebackq delims=" %%f in ("%LIST_FILE%") do (
    set "IN=%%~f"
    set "BASE=%%~nf"
    set "OUT=%OUTPUT_DIR%\%%~nf__enc.mkv"

    echo --------------------------------------------------------
    echo Source: !IN!
    echo Output: !OUT!

    call :probe_video "!IN!" PIX_FMT COLOR_PRIM COLOR_TRC COLOR_SPACE COLOR_RANGE

    set "PIX_ARG="
    if defined PIX_FMT set "PIX_ARG=-pix_fmt !PIX_FMT!"

    set "COLOR_ARGS="
    if defined COLOR_PRIM set "COLOR_ARGS=!COLOR_ARGS! -color_primaries !COLOR_PRIM!"
    if defined COLOR_TRC  set "COLOR_ARGS=!COLOR_ARGS! -color_trc !COLOR_TRC!"
    if defined COLOR_SPACE set "COLOR_ARGS=!COLOR_ARGS! -colorspace !COLOR_SPACE!"
    if defined COLOR_RANGE set "COLOR_ARGS=!COLOR_ARGS! -color_range !COLOR_RANGE!"

    if "%VCODEC%"=="copy" (
        ffmpeg -y -i "!IN!" -map 0 -c copy -map_metadata 0 -map_chapters 0 !PIX_ARG! !COLOR_ARGS! "!OUT!"
    ) else if "%LOSSLESS%"=="1" (
        if "%VCODEC%"=="libx265" (
            ffmpeg -y -i "!IN!" -map 0 -c:v libx265 -crf 0 -preset medium -x265-params lossless=1 !PIX_ARG! !COLOR_ARGS! -c:a copy -c:s copy -c:d copy -map_metadata 0 -map_chapters 0 "!OUT!"
        ) else (
            ffmpeg -y -i "!IN!" -map 0 -c:v libaom-av1 -crf 0 -b:v 0 -cpu-used 4 -row-mt 1 !PIX_ARG! !COLOR_ARGS! -c:a copy -c:s copy -c:d copy -map_metadata 0 -map_chapters 0 "!OUT!"
        )
    ) else (
        ffmpeg -y -i "!IN!" -map 0 -c:v %VCODEC% -b:v %BITRATE% -maxrate %BITRATE% -bufsize %BITRATE% !PIX_ARG! !COLOR_ARGS! -c:a copy -c:s copy -c:d copy -map_metadata 0 -map_chapters 0 "!OUT!"
    )
    echo DONE: !OUT!
)

if exist "%LIST_FILE%" del "%LIST_FILE%"
echo.
echo All jobs finished.
pause
exit /b 0

:probe_video
REM Usage: call :probe_video "file" PIX_VAR PRIM_VAR TRC_VAR SPACE_VAR RANGE_VAR
set "P_FILE=%~1"
set "%2="
set "%3="
set "%4="
set "%5="
set "%6="

for /f "usebackq delims=" %%i in (`ffprobe -v error -select_streams v^:0 -show_entries stream^=pix_fmt -of csv^=p^=0 "%P_FILE%" 2^>nul`) do set "%2=%%i"
for /f "usebackq delims=" %%i in (`ffprobe -v error -select_streams v^:0 -show_entries stream^=color_primaries -of csv^=p^=0 "%P_FILE%" 2^>nul`) do set "%3=%%i"
for /f "usebackq delims=" %%i in (`ffprobe -v error -select_streams v^:0 -show_entries stream^=color_transfer -of csv^=p^=0 "%P_FILE%" 2^>nul`) do set "%4=%%i"
for /f "usebackq delims=" %%i in (`ffprobe -v error -select_streams v^:0 -show_entries stream^=color_space -of csv^=p^=0 "%P_FILE%" 2^>nul`) do set "%5=%%i"
for /f "usebackq delims=" %%i in (`ffprobe -v error -select_streams v^:0 -show_entries stream^=color_range -of csv^=p^=0 "%P_FILE%" 2^>nul`) do set "%6=%%i"
goto :eof
