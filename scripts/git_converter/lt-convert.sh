#!/bin/bash
# ===================================================================
#  GIT Converter v2.7 — Professional Edition (December 2025)
#  Author: LeakTechnologies
#  Modular Architecture for optimal performance
# ===================================================================

# Force window size and font in Git Bash on Windows
if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" ]]; then
    printf '\e[8;100;80t'  # Window size 1000x800
    printf '\e[14]'           # Font size 14
fi

# Get directory where the script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Always create Converted folder in the script's directory
OUT="$SCRIPT_DIR/Converted"
mkdir -p "$OUT"

# Source modules
source "$(dirname "$0")/modules/hardware.sh"
source "$(dirname "$0")/modules/codec.sh"
source "$(dirname "$0")/modules/quality.sh"
source "$(dirname "$0")/modules/filters.sh"
source "$(dirname "$0")/modules/encode.sh"

# Auto-detect encoder function
auto_detect_encoder() {
    echo "Detecting hardware and optimal encoder..." >&2
    
    # Detect GPU type
    gpu_type="none"
    
    # Try NVIDIA detection
    if command -v nvidia-smi >/dev/null 2>&1 && nvidia-smi >/dev/null 2>&1; then
        gpu_type="nvidia"
        echo "  ✓ NVIDIA GPU detected" >&2
    # Try AMD detection with multiple methods
    elif command -v lspci >/dev/null 2>&1 && lspci 2>/dev/null | grep -iq "amd\|radeon\|advanced micro devices"; then
        gpu_type="amd"
        echo "  ✓ AMD GPU detected (via lspci)" >&2
    elif command -v wmic >/dev/null 2>&1 && wmic path win32_VideoController get name 2>/dev/null | grep -iq "amd\|radeon"; then
        gpu_type="amd"
        echo "  ✓ AMD GPU detected (via wmic)" >&2
    # Try Intel detection
    elif command -v lspci >/dev/null 2>&1 && lspci 2>/dev/null | grep -iq "intel.*vga\|intel.*display"; then
        gpu_type="intel"
        echo "  ✓ Intel GPU detected (via lspci)" >&2
    elif command -v wmic >/dev/null 2>&1 && wmic path win32_VideoController get name 2>/dev/null | grep -iq "intel"; then
        gpu_type="intel"
        echo "  ✓ Intel GPU detected (via wmic)" >&2
    else
        echo "  ⚠ No GPU detected, will use CPU encoding" >&2
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
            echo "  Testing $enc..." >&2
            start_time=$(date +%s)
            if timeout 10 ffmpeg -hide_banner -loglevel error -y -f lavfi -i "testsrc=duration=1:size=320x240:rate=1" \
                -c:v "$enc" -f null - >/dev/null 2>&1; then
                end_time=$(date +%s)
                test_time=$((end_time - start_time))
                echo "    $enc: ${test_time}s (SUCCESS)" >&2
                if [[ $test_time -lt $best_time ]]; then
                    best_time=$test_time
                    best_encoder=$enc
                fi
            else
                echo "    $enc: FAILED" >&2
            fi
        else
            echo "    $enc: NOT AVAILABLE" >&2
        fi
    done
    
    if [[ -n "$best_encoder" ]]; then
        echo "  ✓ Selected: $best_encoder (fastest encoder)" >&2
        echo "$best_encoder"
    else
        echo "  ⚠ No working encoder found, defaulting to libx265" >&2
        echo "libx265"
    fi
}

# Display header
clear
cat << "EOF"

╔═══════════════════════════════════════════════════════════════════════════════╗
║                                                                               ║
║                     GIT Converter v2.7 (December 2025)                        ║
║                              by LeakTechnologies                              ║
║                                                                               ║
║      High-quality batch conversion with hardware acceleration             ║
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

# Auto-detect encoder first
echo -e "\n🔍 Auto-detecting optimal encoder..."
optimal_encoder=$(auto_detect_encoder)
echo "✅ Selected: $optimal_encoder"

echo
echo "Press space to continue..."
read -n 1 -s

# Get user settings
echo
echo "╔═══════════════════════════════════════════════════════════════╗"
echo "║                    Choose Encoder/GPU                         ║"
echo "╚═══════════════════════════════════════════════════════════════╝"
echo
echo "   1) Keep auto-detected: $optimal_encoder"
echo "   2) NVIDIA HEVC NVENC"
echo "   3) NVIDIA AV1 NVENC"
echo "   4) AMD HEVC AMF"
echo "   5) AMD AV1 AMF"
echo "   6) Intel HEVC Quick Sync"
echo "   7) Intel AV1 Quick Sync"
echo "   8) CPU SVT-AV1"
echo "   9) CPU x265 HEVC"
echo "  10) Custom encoder selection"
echo

while true; do
    read -p "   Enter 1–10 → " enc_choice
    if [[ -n "$enc_choice" && "$enc_choice" =~ ^([1-9]|10)$ ]]; then
        break
    else
        echo "   Invalid input. Please enter a number between 1 and 10."
    fi
done

case $enc_choice in
    1) 
        echo "✅ Keeping: $optimal_encoder"
        ;;
    2) 
        optimal_encoder="hevc_nvenc"
        echo "✅ Selected: NVIDIA HEVC NVENC"
        ;;
    3) 
        optimal_encoder="av1_nvenc"
        echo "✅ Selected: NVIDIA AV1 NVENC"
        ;;
    4) 
        optimal_encoder="hevc_amf"
        echo "✅ Selected: AMD HEVC AMF"
        ;;
    5) 
        optimal_encoder="av1_amf"
        echo "✅ Selected: AMD AV1 AMF"
        ;;
    6) 
        optimal_encoder="hevc_qsv"
        echo "✅ Selected: Intel HEVC Quick Sync"
        ;;
    7) 
        optimal_encoder="av1_qsv"
        echo "✅ Selected: Intel AV1 Quick Sync"
        ;;
    8) 
        optimal_encoder="libsvtav1"
        echo "✅ Selected: CPU SVT-AV1"
        ;;
    9) 
        optimal_encoder="libx265"
        echo "✅ Selected: CPU x265 HEVC"
        ;;
    10) 
        echo -e "\n⚙️ Available Encoders:"
        ffmpeg -hide_banner -encoders | grep -E "(hevc|av1|h265)" | grep -v "V\|D" | awk '{print $2}' | nl
        
        while true; do
            read -p "Enter encoder name: " custom_enc
            if ffmpeg -hide_banner -loglevel error -encoders | grep -q "$custom_enc"; then
                optimal_encoder="$custom_enc"
                echo "✅ Selected: $custom_enc"
                break
            else
                echo "❌ Encoder '$custom_enc' not found. Please try again."
            fi
        done
        ;;
esac

setup_codec_and_container
get_resolution_settings
get_fps_settings
get_quality_settings "$optimal_encoder"
get_color_correction

# Build filter chain
build_filter_chain

# Set container from user selection
ext="$OUTPUT_CONTAINER"

# Display encoding summary
echo
echo "Encoding → $ENCODER | $res_name | $ext $suf${color_suf} @ $quality_name"
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

# Process all files
process_files "$ENCODER" "$quality_params" "$filter_chain" "$suf" "$color_suf" "$ext" "${video_files[@]}"

echo "========================================================"
echo "All finished — files in '$OUT'"
echo "========================================================"
read -p "Press Enter to exit"