@echo off
setlocal
REM VideoTools helper: Motion smoothing to 60fps using minterpolate; keeps audio/subs/metadata
REM Usage: smooth-60fps-vt.bat "input.ext" "output.mkv"

if "%~1"=="" (
  echo Usage: %~nx0 "input.ext" "output.mkv"
  exit /b 1
)

set INPUT=%~1
set OUTPUT=%~2
if "%OUTPUT%"=="" (
  set OUTPUT=%~dpn1-smooth60.mkv
)

ffmpeg -y -hide_banner -loglevel error ^
  -i "%INPUT%" ^
  -map 0 ^
  -vf "minterpolate=fps=60" ^
  -c:v libx265 -preset slow -b:v 3000k -pix_fmt yuv420p ^
  -c:a copy -c:s copy -c:d copy ^
  "%OUTPUT%"

if %ERRORLEVEL% equ 0 (
  echo Done: "%OUTPUT%"
) else (
  echo Encode failed. Check above ffmpeg output.
)
endlocal
