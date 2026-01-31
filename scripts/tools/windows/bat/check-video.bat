@echo off
setlocal enabledelayedexpansion
chcp 65001 >nul
title VideoTools Quick Check

if "%~1"=="" (
    echo Drag a video file onto this .bat
    pause
    exit /b 1
)

where ffprobe >nul 2>&1 && where ffmpeg >nul 2>&1
if errorlevel 1 (
    echo ffmpeg/ffprobe not found in PATH. Install via winget/choco or run setup-windows.
    pause
    exit /b 1
)

cls
echo === VideoTools Quick Check ===
echo File: "%~1"
echo.

ffprobe -v error -hide_banner -i "%~1" ^
  -show_entries format=format_name,duration,size,bit_rate ^
  -show_entries stream=codec_name,codec_type,width,height,avg_frame_rate,channels,sample_rate ^
  -select_streams v:0 -select_streams a:0 ^
  -of default=noprint_wrappers=1:nokey=1

echo.
echo Checking interlacing (~first 600 frames)...
set "idetLine="
for /f "usebackq tokens=*" %%L in (`ffmpeg -v error -hide_banner -i "%~1" -vf idet -frames:v 600 -an -sn -f null NUL 2^>^&1 ^| findstr /i "Multi frame detection"`) do set "idetLine=%%L"

if not defined idetLine (
    echo (No idet summary found)
    echo.
    echo Done.
    pause
    exit /b 0
)

rem Example: Multi frame detection: TFF: 0 BFF: 0 Progressive: 898 Undetermined: 0
for /f "tokens=5,7,9,11 delims=: " %%a in ("!idetLine!") do (
    set "TFF=%%a"
    set "BFF=%%b"
    set "PROG=%%c"
    set "UNDET=%%d"
)

set /a TOTAL=!TFF!+!BFF!+!PROG!+!UNDET!
if !TOTAL! NEQ 0 (
    set /a PTFF=(!TFF!*100)/!TOTAL!
    set /a PBFF=(!BFF!*100)/!TOTAL!
    set /a PPROG=(!PROG!*100)/!TOTAL!
    set /a PUN=(!UNDET!*100)/!TOTAL!
)

echo !idetLine!
if !TOTAL! GTR 0 (
    echo TFF: !TFF! (^~!PTFF!%%^) ^| BFF: !BFF! (^~!PBFF!%%^) ^| Progressive: !PROG! (^~!PPROG!%%^) ^| Undetermined: !UNDET! (^~!PUN!%%^)
)

echo.
echo Done.
pause

