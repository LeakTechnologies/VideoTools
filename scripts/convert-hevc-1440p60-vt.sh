#!/usr/bin/env bash
# VideoTools helper: HEVC 1440p @60fps upscale (balanced bitrate, keeps audio/subs/metadata)
# Usage: ./convert-hevc-1440p60-vt.sh "input.ext" ["output.mkv"]
set -e

if [ -z "$1" ]; then
  echo "Usage: $0 input.ext [output.mkv]"
  exit 1
fi

in="$1"
out="${2:-${1%.*}-hevc-1440p60.mkv}"

ffmpeg -y -hide_banner -loglevel error \
  -i "$in" \
  -map 0 \
  -vf "scale=-2:1440,fps=60" \
  -c:v libx265 -preset slow -b:v 4000k -pix_fmt yuv420p \
  -c:a copy -c:s copy -c:d copy \
  "$out"

echo "Done: $out"
