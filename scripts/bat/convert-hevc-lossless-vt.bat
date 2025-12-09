@echo off
setlocal
REM VideoTools helper: H.265/HEVC lossless (CRF 0) re-encode; keeps audio/subs/metadata
REM Usage: convert-hevc-lossless-vt.bat "input.ext" "output.mkv"

if "%~1"=="" (
  echo Usage: %~nx0 "input.ext" "output.mkv"
  exit /b 1
)

set INPUT=%~1
set OUTPUT=%~2
if "%OUTPUT%"=="" (
  set OUTPUT=%~dpn1-hevc-lossless.mkv
)

ffmpeg -y -hide_banner -loglevel error ^
  -i "%INPUT%" ^
  -map 0 ^
  -c:v libx265 -preset slow -crf 0 -x265-params lossless=1 -pix_fmt yuv420p ^
  -c:a copy -c:s copy -c:d copy ^
  "%OUTPUT%"

if %ERRORLEVEL% equ 0 (
  echo Done: "%OUTPUT%"
) else (
  echo Encode failed. Check above ffmpeg output.
)
endlocal
