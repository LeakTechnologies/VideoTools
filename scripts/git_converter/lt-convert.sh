#!/bin/bash
# ===================================================================
#  LT Converter v3.0 — Professional Edition (December 2025)
#  Author: LeakTechnologies
#  Fixed: Converted folder always created in script directory
#  Fixed: Filter chaining, input validation, bitrate handling
# ===================================================================

# Force window size in Git Bash on Windows
if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" ]]; then
    printf '\e[8;45;100t'
fi

# Get the directory where the script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Always create Converted folder in the script's directory
OUT="$SCRIPT_DIR/Converted"
mkdir -p "$OUT"

# Detect optimal encoder at startup
optimal_encoder=$(detect_hardware)

# Hardware detection and encoder selection
detect_hardware() {
    echo "Detecting hardware and optimal encoder..."
    
    # Detect GPU type
    gpu_type="none"
    if command -v nvidia-smi >/dev/null 2>&1 && nvidia-smi >/dev/null 2>&1; then
        gpu_type="nvidia"
        echo "  ✓ NVIDIA GPU detected"
    elif lspci 2>/dev/null | grep -iq "amd\|radeon"; then
        gpu_type="amd"
        echo "  ✓ AMD GPU detected"
    elif lspci 2>/dev/null | grep -iq "intel.*vga\|intel.*display"; then
        gpu_type="intel"
        echo "  ✓ Intel GPU detected"
    else
        echo "  ⚠ No GPU detected, will use CPU encoding"
    fi
    
    # Test encoder availability and speed
    local best_encoder=""
    local best_time=999999
    
    # Test candidates based on GPU type
    local candidates=()
    case $gpu_type in
        nvidia) candidates=("hevc_nvenc" "av1_nvenc" "libsvtav1" "libx265") ;;
        amd)    candidates=("hevc_amf" "av1_amf" "libsvtav1" "libx265") ;;
        intel)  candidates=("hevc_qsv" "av1_qsv" "libsvtav1" "libx265") ;;
        *)      candidates=("libsvtav1" "libx265") ;;
    esac
    
    # Quick benchmark each encoder
    for enc in "${candidates[@]}"; do
        if ffmpeg -hide_banner -loglevel error -encoders | grep -q "$enc"; then
            echo "  Testing $enc..."
            start_time=$(date +%s.%N)
            if ffmpeg -hide_banner -loglevel error -y -f lavfi -i "testsrc=duration=2:size=320x240:rate=1" \
                -c:v "$enc" -f null - >/dev/null 2>&1; then
                end_time=$(date +%s.%N)
                test_time=$(echo "$end_time - $start_time" | bc -l 2>/dev/null || echo "1")
                echo "    $enc: ${test_time}s"
                if (( $(echo "$test_time < $best_time" | bc -l 2>/dev/null || echo "0") )); then
                    best_time=$test_time
                    best_encoder=$enc
                fi
            fi
        fi
    done
    
    if [[ -n "$best_encoder" ]]; then
        echo "  ✓ Selected: $best_encoder (fastest encoder)"
        echo "$best_encoder"
        return 0
    else
        echo "  ⚠ No working encoder found, defaulting to libx265"
        echo "libx265"
        return 0
    fi
}

clear
cat << "EOF"

╔═══════════════════════════════════════════════════════════════════════════════╗
║                                                                               ║
║                     LT Converter v3.0 (December 2025)                        ║
║                              by LeakTechnologies                              ║
║                                                                               ║
║      High-quality batch conversion with hardware acceleration                  ║
║      • AV1 & H.265 support                                                    ║
║      • Smart upscaling (Source / 720p / 1080p / 1440p / 4K / 2X / 4X)         ║
║      • Optional 60 fps smooth motion                                          ║
║      • Color correction for Topaz AI videos                                  ║
║      • Clean 8-bit encoding (no green lines)                                  ║
║      • AAC stereo audio                                                       ║
║      • MKV or MP4 output                                                      ║
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

while true; do
    read -p "   Enter 1–7 → " res
    if [[ -n "$res" && "$res" =~ ^[1-7]$ ]]; then
        break
    else
        echo "   Invalid input. Please enter a number between 1 and 7."
    fi
done

case $res in
  1) scale=""                     ; res_name="Source"   ;;
  2) scale="1280:720"             ; res_name="720p"     ;;
  3) scale="1920:1080"            ; res_name="1080p"    ;;
  4) scale="2560:1440"            ; res_name="1440p"    ;;
  5) scale="3840:2160"            ; res_name="4K"       ;;
  6) scale="iw*2:ih*2"            ; res_name="2X"       ;;
  7) scale="iw*4:ih*4"            ; res_name="4X"       ;;
esac

# — Scaling Algorithm (only if upscaling) —
if [[ -n "$scale" ]]; then
    clear
    cat << "EOF"
╔═══════════════════════════════════════════════════════════════╗
║                    Choose Scaling Algorithm                   ║
╚═══════════════════════════════════════════════════════════════╝

   1) Bicubic (fast, good quality)
   2) Lanczos (best quality, slower)
   3) Bilinear (fastest, basic quality)
EOF
    echo

    while true; do
        read -p "   Enter 1–3 → " scale_opt
        if [[ -n "$scale_opt" && "$scale_opt" =~ ^[1-3]$ ]]; then
            break
        else
            echo "   Invalid input. Please enter a number between 1 and 3."
        fi
    done

    case $scale_opt in
        1) scale_flags="bicubic" ;;
        2) scale_flags="lanczos" ;;
        3) scale_flags="bilinear" ;;
    esac
else
    scale_flags="lanczos"  # Default for consistency
fi

# — Encoding Options —
clear
cat << "EOF"
╔═════════════════════════════════════════════════════════════╗
║                    Encoding Options                           ║
╚═══════════════════════════════════════════════════════════════╝

   Auto-detected encoder: $optimal_encoder

   1) Original FPS
   2) 60 FPS
EOF
echo

while true; do
    read -p "   Enter 1–2 → " c
    if [[ -n "$c" && "$c" =~ ^[1-2]$ ]]; then
        break
    else
        echo "   Invalid input. Please enter 1 or 2."
    fi
done

# Set container and FPS
ext="mkv"
fps_filter=""
suf=""
if [[ "$c" == "2" ]]; then
    fps_filter="fps=60"
    suf="_60fps"
fi

# Use auto-detected encoder
codec="$optimal_encoder"

# — Color Correction Option —
clear
cat << "EOF"
╔═══════════════════════════════════════════════════════════════╗
║                    Color Correction Option                    ║
╚═══════════════════════════════════════════════════════════════╝

   1) No color correction
   2) Restore pink skin tones (Topaz AI fix)
   3) Warm color boost
   4) Cool color boost
   5) 2000s DVD Restore
   6) 90s Quality Restore
   7) VHS Quality Restore
   8) Anime Preservation (clean lines & colors)
EOF
echo

while true; do
    read -p "   Enter 1–8 → " color_opt
    if [[ -n "$color_opt" && "$color_opt" =~ ^[1-8]$ ]]; then
        break
    else
        echo "   Invalid input. Please enter a number between 1 and 8."
    fi
done

case $color_opt in
  1) color_filter=""; color_suf="" ;;
  2) color_filter="eq=contrast=1.05:brightness=0.02:saturation=1.1:hue=-0.02"; color_suf="_colorfix" ;;
  3) color_filter="eq=contrast=1.03:brightness=0.01:saturation=1.15:hue=-0.01"; color_suf="_warm" ;;
  4) color_filter="eq=contrast=1.03:brightness=0.01:saturation=0.95:hue=0.02"; color_suf="_cool" ;;
  5) color_filter="eq=contrast=1.08:brightness=0.03:saturation=1.2:hue=0:gamma=0.95,unsharp=4:4:0.8:4:4:0.0"; color_suf="_dvdrestore" ;;
  6) color_filter="eq=contrast=1.12:brightness=0.05:saturation=1.3:hue=0:gamma=0.92,unsharp=5:5:1.0:5:5:0.0,hqdn3d=3:2:2:3"; color_suf="_90srestore" ;;
  7) color_filter="eq=contrast=1.15:brightness=0.08:saturation=1.4:hue=0:gamma=0.90,unsharp=5:5:1.0:5:5:0.0,hqdn3d=3:2:2:3"; color_suf="_vhsrestore" ;;
  8) color_filter="eq=contrast=1.02:brightness=0:saturation=1.05:hue=0:gamma=1.0,unsharp=3:3:0.5:3:3:0.0,gradfun=2.5:2.5"; color_suf="_anime" ;;
esac

# — Quality Selection —
clear
cat << "EOF"
╔═══════════════════════════════════════════════════════════════╗
║                    Choose Quality Mode                       ║
╚═══════════════════════════════════════════════════════════════╝

   1) High Quality AV1 (CRF 18) - Best compression
   2) High Quality HEVC (CRF 18) - Fast encoding
   3) Near-Lossless AV1 (CRF 16) - Maximum quality
   4) Near-Lossless HEVC (CRF 16) - Quality/Speed balance
   5) Custom bitrate
EOF
echo

while true; do
    read -p "   Enter 1–5 → " b
    if [[ -n "$b" && "$b" =~ ^[1-5]$ ]]; then
        break
    else
        echo "   Invalid input. Please enter a number between 1 and 5."
    fi
done

# Set quality parameters based on auto-detected encoder
case $b in
  1) 
      if [[ "$codec" == *"av1"* ]]; then
          quality_params="-crf 18 -preset 6"
          quality_name="AV1 CRF 18"
      else
          quality_params="-crf 18 -quality 23"
          quality_name="HEVC CRF 18"
      fi
      ;;
  2) 
      if [[ "$codec" == *"av1"* ]]; then
          quality_params="-crf 16 -preset 4"
          quality_name="AV1 CRF 16"
      else
          quality_params="-crf 16 -quality 28"
          quality_name="HEVC CRF 16"
      fi
      ;;
  3) 
      echo
      echo "Enter bitrate (e.g., 5000k, 8000k):"
      read -p "→ " custom_bitrate
      if [[ -n "$custom_bitrate" ]]; then
          quality_params="-b:v $custom_bitrate"
          quality_name="Custom $custom_bitrate"
      else
          quality_params="-crf 18 -quality 23"
          quality_name="HEVC CRF 18"
      fi
      ;;
esac

# Force 8-bit input with performance optimization
bitdepth_filter="-pix_fmt yuv420p -movflags +faststart"

echo
echo "Encoding → $codec | $res_name | $ext $suf${color_suf} @ $quality_name"
echo

# Process video files - handle drag-and-drop vs double-click
if [[ $# -gt 0 ]]; then
    # Files were dragged onto the script
    video_files=("$@")
    echo "Processing dragged files: ${#video_files[@]} file(s)"
else
    # Double-clicked - process all video files in current directory
    shopt -s nullglob
    video_files=(*.mp4 *.mkv *.mov *.avi *.wmv *.ts *.m2ts)
    shopt -u nullglob
    
    if [[ ${#video_files[@]} -eq 0 ]]; then
        echo "No video files found in current directory."
        read -p "Press Enter to exit"
        exit 0
    fi
    echo "Processing all video files in current directory: ${#video_files[@]} file(s)"
fi

for f in "${video_files[@]}"; do
    [[ -f "$f" ]] || continue

    # Extract basename for output filename (handles both relative and absolute paths)
    basename_f=$(basename "$f")
    out="$OUT/${basename_f%.*}${suf}${color_suf}__cv.$ext"
    [[ -f "$out" ]] && { echo "SKIP $f (already exists)"; continue; }

    # Build filter chain
    filter_chain=""
    if [[ -n "$scale" ]]; then
        filter_chain="scale=${scale}:flags=${scale_flags}"
    fi
    
    if [[ -n "$fps_filter" ]]; then
        if [[ -n "$filter_chain" ]]; then
            filter_chain="${filter_chain},${fps_filter}"
        else
            filter_chain="$fps_filter"
        fi
    fi
    
    if [[ -n "$color_filter" ]]; then
        if [[ -n "$filter_chain" ]]; then
            filter_chain="${filter_chain},${color_filter}"
        else
            filter_chain="$color_filter"
        fi
    fi

    echo "Processing: $f → $(basename "$out")"
    
    # Build optimized ffmpeg command
    if [[ -n "$filter_chain" ]]; then
        ffmpeg -y -i "$f" -pix_fmt yuv420p -vf "$filter_chain" \
               -c:v "$codec" $quality_params -c:a aac -b:a 192k -ac 2 "$out"
    else
        ffmpeg -y -i "$f" -pix_fmt yuv420p \
               -c:v "$codec" $quality_params -c:a aac -b:a 192k -ac 2 "$out"
    fi

    if [[ $? -eq 0 ]]; then
        echo "DONE → $(basename "$out")"
    else
        echo "ERROR → Failed to process $f"
    fi
    echo
done

echo "========================================================"
echo "All finished — files in '$OUT'"
echo "========================================================"
read -p "Press Enter to exit"