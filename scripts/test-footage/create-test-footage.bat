@echo off
REM create-test-footage.bat - Create test footage in all common formats
REM
REM Usage: create-test-footage.bat <source_video> [duration_seconds]
REM
REM Creates 10-second snippets from source video in H.264, H.265, VP9,
REM MPEG4, VP8, and AV1 formats for player testing.

setlocal enabledelayedexpansion

set "SOURCE=%~1"
set "DURATION=%~2"
if "%DURATION%"=="" set "DURATION=10"

set "OUTPUT_DIR=test-footage-%date:~-4%%date:~3,2%%date:~0,2%-%time:~0,2%%time:~3,2%%time:~6,2%"
set "OUTPUT_DIR=%OUTPUT_DIR: =0%"
set "OUTPUT_DIR=%OUTPUT_DIR:_=%"

if "%SOURCE%"=="" (
    echo Usage: %~nx0 ^<source_video^> [duration_seconds]
    echo   Creates test footage in multiple formats
    exit /b 1
)

if not exist "%SOURCE%" (
    echo Error: Source file not found: %SOURCE%
    exit /b 1
)

mkdir "%OUTPUT_DIR%" 2>nul

for %%F in ("%SOURCE%") do set "BASENAME=%%~nF"

echo Creating test footage in %OUTPUT_DIR%/
echo Source: %SOURCE%
echo Duration: %DURATION%s
echo.

REM H.264 in MP4 container
echo [1/8] Creating H.264/MP4...
ffmpeg -y -i "%SOURCE%" -t %DURATION% ^
    -c:v libx264 -preset fast -crf 23 ^
    -c:a aac -b:a 128k ^
    "%OUTPUT_DIR%\%BASENAME%_h264.mp4" 2>nul

REM H.265 in MP4 container
echo [2/8] Creating H.265/MP4...
ffmpeg -y -i "%SOURCE%" -t %DURATION% ^
    -c:v libx265 -preset fast -crf 28 ^
    -c:a aac -b:a 128k ^
    "%OUTPUT_DIR%\%BASENAME%_h265.mp4" 2>nul

REM VP9 in WebM container
echo [3/8] Creating VP9/WebM...
ffmpeg -y -i "%SOURCE%" -t %DURATION% ^
    -c:v libvpx-vp9 -crf 30 -b:v 0 ^
    -c:a libopus -b:a 128k ^
    "%OUTPUT_DIR%\%BASENAME%_vp9.webm" 2>nul

REM MPEG4 in AVI container
echo [4/8] Creating MPEG4/AVI...
ffmpeg -y -i "%SOURCE%" -t %DURATION% ^
    -c:v mpeg4 -q:v 5 ^
    -c:a libmp3lame -b:a 128k ^
    "%OUTPUT_DIR%\%BASENAME%_mpeg4.avi" 2>nul

REM VP8 in WebM container
echo [5/8] Creating VP8/WebM...
ffmpeg -y -i "%SOURCE%" -t %DURATION% ^
    -c:v libvpx -crf 30 -b:v 0 ^
    -c:a libopus -b:a 128k ^
    "%OUTPUT_DIR%\%BASENAME%_vp8.webm" 2>nul

REM AV1 in MKV container
echo [6/8] Creating AV1/MKV...
ffmpeg -y -i "%SOURCE%" -t %DURATION% ^
    -c:v libaom-av1 -crf 35 -cpu-used 4 ^
    -c:a libopus -b:a 128k ^
    "%OUTPUT_DIR%\%BASENAME%_av1.mkv" 2>nul

REM H.264 in MOV container
echo [7/8] Creating H.264/MOV...
ffmpeg -y -i "%SOURCE%" -t %DURATION% ^
    -c:v libx264 -preset fast -crf 23 ^
    -c:a aac -b:a 128k ^
    "%OUTPUT_DIR%\%BASENAME%_h264.mov" 2>nul

REM ProRes 422 for high quality reference
echo [8/8] Creating ProRes422/MOV...
ffmpeg -y -i "%SOURCE%" -t %DURATION% ^
    -c:v prores_ks -profile:v 2 ^
    -c:a pcm_s16le ^
    "%OUTPUT_DIR%\%BASENAME%_prores422.mov" 2>nul

echo.
echo Done! Files in %OUTPUT_DIR%/
echo.
dir /b "%OUTPUT_DIR%\" | find /c /v ""
echo files created.
dir /o-n "%OUTPUT_DIR%\"