#!/bin/bash
# ===================================================================
#  GIT Converter — FINAL — 1080p FORCED + 60 FPS FIXED
#  Author: LeakTechnologies
# ===================================================================

OUT="Converted"
mkdir -p "$OUT"

# Choose best available encoder with runtime check
pick_encoder() {
  # candidate list in order of preference
  local candidates=("$@")
  for enc in "${candidates[@]}"; do
    # Quick runtime probe: attempt tiny encode and discard output
    if ffmpeg -hide_banner -loglevel error \
      -f lavfi -i color=size=16x16:rate=1 -frames:v 1 \
      -c:v "$enc" -f null - >/dev/null 2>&1; then
      echo "$enc"
      return 0
    fi
  done
  return 1
}

clear
cat << "EOF"

╔═══════════════════════════════════════════════════════════════════════════════╗
║                                                                               ║
║                     GIT Converter (December 2025)                             ║
║                              by LeakTechnologies                              ║
║                                                                               ║
╚═══════════════════════════════════════════════════════════════════════════════╝

EOF

# — Resolution —
echo "   Choose your resolution:"
echo
echo "   1) Source file resolution (no upscale)"
echo "   2) 720p  (1280×720)"
echo "   3) 1080p (1920×1080)"
echo "   4) 1440p (2560×1440)"
echo "   5) 4K    (3840×2160)"
echo "   6) 2X Upscale"
echo "   7) 4X Upscale"
echo
read -p "   Enter 1–7 → " res

case $res in
  1) scale=""                     ; res_name="Source"   ;;
  2) scale="1280:720"             ; res_name="720p"     ;;
  3) scale="1920:1080"            ; res_name="1080p"    ;;
  4) scale="2560:1440"            ; res_name="1440p"    ;;
  5) scale="3840:2160"            ; res_name="4K"       ;;
  6) scale="iw*2:ih*2"            ; res_name="2X"       ;;
  7) scale="iw*4:ih*4"            ; res_name="4X"       ;;
  *) echo "Invalid — using 1080p"; scale="1920:1080"; res_name="1080p" ;;
esac

# — Codec + FPS + Container —
clear
cat << "EOF"
╔═══════════════════════════════════════════════════════════════╗
║                    Choose codec + FPS                         ║
╚═══════════════════════════════════════════════════════════════╝

   1) [MKV]  AV1_AMF    — Original FPS
   2) [MKV]  HEVC       — Original FPS
   3) [MKV]  AV1_AMF    — 60 FPS
   4) [MKV]  HEVC       — 60 FPS
   5) [MP4]  HEVC       — Original FPS
   6) [MP4]  HEVC       — 60 FPS
EOF
echo
read -p "   Enter 1–6 → " c

case $c in
  1|3) codec_pref=("av1_amf" "libaom-av1") ;;
  2|4|5|6) codec_pref=("hevc_amf" "hevc_nvenc" "h264_nvenc" "libx265") ;;
  *)   echo "Invalid — exiting"; sleep 3; exit ;;
esac

case $c in
  1|2|3|4) ext="mkv" ;;
  5|6)     ext="mp4" ;;
esac

case $c in
  3|4|6) fps_filter=",fps=60"; suf="_60fps" ;;
  *)     fps_filter="";        suf=""       ;;
esac

# Resolve encoder now, once
codec=$(pick_encoder "${codec_pref[@]}")
if [ -z "$codec" ]; then
  echo "No supported encoder found (tried: ${codec_pref[*]})."
  echo "Install/enable GPU drivers or fall back to CPU codecs."
  echo "Defaulting to libx265."
  codec="libx265"
fi

# — Bitrate Selection —
clear
cat << "EOF"
╔═══════════════════════════════════════════════════════════════╗
║                       Choose bitrate                          ║
╚═══════════════════════════════════════════════════════════════╝

   1) 1800 kbps
   2) 2000 kbps
   3) 2300 kbps
   4) 2600 kbps
   5) 2900 kbps
   6) 3200 kbps
   7) 3500 kbps
   8) 3800 kbps
   9) Source file bitrate
EOF
echo
read -p "   Enter 1–9 → " b

case $b in
  1) BITRATE="1800k" ;;
  2) BITRATE="2000k" ;;
  3) BITRATE="2300k" ;;
  4) BITRATE="2600k" ;;
  5) BITRATE="2900k" ;;
  6) BITRATE="3200k" ;;
  7) BITRATE="3500k" ;;
  8) BITRATE="3800k" ;;
  9) BITRATE="source" ;;
  *) BITRATE="2400k"; echo "Invalid → using default 2400k" ;;
esac

# Force 8-bit input
bitdepth_filter="-pix_fmt yuv420p"

echo
echo "Encoding → $codec | $res_name | $ext $suf @ ${BITRATE:-source} kbps"
echo

for f in *.mp4 *.mkv *.mov *.avi *.wmv *.ts *.m2ts; do
  [[ -f "$f" ]] || continue

  out="$OUT/${f%.*}${suf}__cv.$ext"
  [[ -f "$out" ]] && { echo "SKIP $f"; continue; }

  # Source bitrate if chosen
  if [ "$BITRATE" = "source" ]; then
    src_bitrate=$(ffprobe -v error -select_streams v:0 -show_entries stream=bit_rate -of csv=p=0 "$f" 2>/dev/null || echo 2400000)
    this_bitrate=$(( src_bitrate / 1000 ))k
  else
    this_bitrate="$BITRATE"
  fi

  # FINAL FIXED CONVERSION — 1080p FORCED + 60fps works
  if ffmpeg -y -i "$f" $bitdepth_filter -vf "scale=${scale}:flags=lanczos${fps_filter}" \
         -c:v "$codec" -b:v "$this_bitrate" -c:a aac -b:a 192k -ac 2 "$out"; then
    echo "DONE → $(basename "$out")"
  else
    echo "FAILED → $f (encoder: $codec). Check ffmpeg output above."
    rm -f "$out"
  fi
  echo
done

echo "========================================================"
echo "All finished — files in '$OUT'"
echo "========================================================"
read -p "Press Enter to exit"
