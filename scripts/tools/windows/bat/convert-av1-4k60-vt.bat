@echo off
setlocal
REM VideoTools helper: AV1 4K @60fps upscale (balanced bitrate, keeps audio/subs/metadata)
REM Usage: convert-av1-4k60-vt.bat "input.ext" "output.mkv"

if "%~1"=="" (
  echo Usage: %~nx0 "input.ext" "output.mkv"
  exit /b 1
)

set INPUT=%~1
set OUTPUT=%~2
if "%OUTPUT%"=="" (
  set OUTPUT=%~dpn1-av1-4k60.mkv
)

ffmpeg -y -hide_banner -loglevel error ^
  -i "%INPUT%" ^
  -map 0 ^
  -vf "scale=-2:2160,fps=60" ^
  -c:v libaom-av1 -b:v 5200k -cpu-used 4 -row-mt 1 -pix_fmt yuv420p ^
  -c:a copy -c:s copy -c:d copy ^
  "%OUTPUT%"

if %ERRORLEVEL% equ 0 (
  echo Done: "%OUTPUT%"
) else (
  echo Encode failed. Check above ffmpeg output.
)
endlocal
