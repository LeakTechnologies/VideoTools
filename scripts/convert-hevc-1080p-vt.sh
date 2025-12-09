#!/usr/bin/env bash
# VideoTools helper: H.265/HEVC 1080p balanced encode (keeps audio/subs/metadata)
# Usage: ./convert-hevc-1080p-vt.sh "input.ext" ["output.mkv"]
set -e

if [ -z "$1" ]; then
  echo "Usage: $0 input.ext [output.mkv]"
  exit 1
fi

in="$1"
out="${2:-${1%.*}-hevc-1080p.mkv}"

ffmpeg -y -hide_banner -loglevel error \
  -i "$in" \
  -map 0 \
  -c:v libx265 -preset slow -b:v 2000k -pix_fmt yuv420p \
  -c:a copy -c:s copy -c:d copy \
  "$out"

echo "Done: $out"
