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

# Get directory where the script is located (cross-platform)
if [[ -n "${BASH_SOURCE[0]}" ]]; then
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
else
    SCRIPT_DIR="$(pwd)"
fi

# Always create Converted folder in the script's directory
OUT="$SCRIPT_DIR/Converted"
mkdir -p "$OUT"

# Source modules with cross-platform path handling
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/modules/hardware.sh"
source "$SCRIPT_DIR/modules/codec.sh"
source "$SCRIPT_DIR/modules/quality.sh"
source "$SCRIPT_DIR/modules/filters.sh"
source "$SCRIPT_DIR/modules/encode.sh"

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
            # Cross-platform timeout handling
            if command -v timeout >/dev/null 2>&1; then
                # Linux/macOS timeout
                timeout_cmd="timeout 10"
            elif command -v gtimeout >/dev/null 2>&1; then
                # macOS with GNU coreutils
                timeout_cmd="gtimeout 10"
            else
                # Windows - no timeout, but we'll background and kill
                timeout_cmd=""
            fi
            
            if [[ -n "$timeout_cmd" ]]; then
                if $timeout_cmd ffmpeg -hide_banner -loglevel error -y -f lavfi -i "testsrc=duration=1:size=320x240:rate=1" \
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
                # Windows fallback - run without timeout but limit test duration
                ffmpeg -hide_banner -loglevel error -y -f lavfi -i "testsrc=duration=1:size=320x240:rate=1" \
                    -c:v "$enc" -f null - >/dev/null 2>&1 &
                ffmpeg_pid=$!
                sleep 5  # Wait max 5 seconds
                if kill -0 $ffmpeg_pid 2>/dev/null; then
                    kill $ffmpeg_pid 2>/dev/null
                    wait $ffmpeg_pid 2>/dev/null
                    echo "    $enc: TIMEOUT" >&2
                else
                    wait $ffmpeg_pid
                    end_time=$(date +%s)
                    test_time=$((end_time - start_time))
                    echo "    $enc: ${test_time}s (SUCCESS)" >&2
                    if [[ $test_time -lt $best_time ]]; then
                        best_time=$test_time
                        best_encoder=$enc
                    fi
                fi
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
    # More comprehensive file detection for cross-platform compatibility
    video_files=()
    
    # Common video extensions
    extensions=("mp4" "mkv" "mov" "avi" "wmv" "ts" "m2ts" "flv" "webm" "m4v" "3gp" "mpg" "mpeg" "m4v")
    
    for ext in "${extensions[@]}"; do
        for file in *."$ext"; do
            if [[ -f "$file" ]]; then
                video_files+=("$file")
            fi
        done
    done
    
    if [[ ${#video_files[@]} -eq 0 ]]; then
        echo "No video files found in current directory."
        echo "Supported formats: ${extensions[*]}"
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