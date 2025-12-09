#!/usr/bin/env bash
# VideoTools helper: H.265/HEVC lossless (CRF 0) re-encode; keeps audio/subs/metadata
# Usage: ./convert-hevc-lossless-vt.sh "input.ext" ["output.mkv"]
set -e

if [ -z "$1" ]; then
  echo "Usage: $0 input.ext [output.mkv]"
  exit 1
fi

in="$1"
out="${2:-${1%.*}-hevc-lossless.mkv}"

ffmpeg -y -hide_banner -loglevel error \
  -i "$in" \
  -map 0 \
  -c:v libx265 -preset slow -crf 0 -x265-params lossless=1 -pix_fmt yuv420p \
  -c:a copy -c:s copy -c:d copy \
  "$out"

echo "Done: $out"
