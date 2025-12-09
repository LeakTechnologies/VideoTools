#!/usr/bin/env bash
# VideoTools helper: Motion smoothing to 60fps using minterpolate; keeps audio/subs/metadata
# Usage: ./smooth-60fps-vt.sh "input.ext" ["output.mkv"]
set -e

if [ -z "$1" ]; then
  echo "Usage: $0 input.ext [output.mkv]"
  exit 1
fi

in="$1"
out="${2:-${1%.*}-smooth60.mkv}"

ffmpeg -y -hide_banner -loglevel error \
  -i "$in" \
  -map 0 \
  -vf "minterpolate=fps=60" \
  -c:v libx265 -preset slow -b:v 3000k -pix_fmt yuv420p \
  -c:a copy -c:s copy -c:d copy \
  "$out"

echo "Done: $out"
