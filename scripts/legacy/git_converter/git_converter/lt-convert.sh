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

# Hardware benchmarking function
run_hardware_benchmark() {
    echo "Detecting your hardware..." >&2
    
    # Detect GPU type
    gpu_type="none"
    gpu_name="Unknown"
    
    # Try NVIDIA detection
    if command -v nvidia-smi >/dev/null 2>&1 && nvidia-smi >/dev/null 2>&1; then
        gpu_type="nvidia"
        gpu_name=$(nvidia-smi --query-gpu=name --format=csv,noheader,nounits | head -1)
        echo "  ✓ NVIDIA GPU detected: $gpu_name" >&2
    # Try AMD detection
    elif command -v lspci >/dev/null 2>&1 && lspci 2>/dev/null | grep -iq "amd\|radeon\|advanced micro devices"; then
        gpu_type="amd"
        gpu_name=$(lspci 2>/dev/null | grep -i "amd\|radeon" | head -1 | cut -d':' -f3 | xargs)
        echo "  ✓ AMD GPU detected: $gpu_name" >&2
    elif command -v lshw >/dev/null 2>&1 && lshw -c display 2>/dev/null | grep -iq "amd\|radeon"; then
        gpu_type="amd"
        gpu_name=$(lshw -c display 2>/dev/null | grep -i "product" | head -1 | cut -d':' -f2 | xargs)
        echo "  ✓ AMD GPU detected: $gpu_name" >&2
    elif [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" ]] && command -v wmic >/dev/null 2>&1 && wmic path win32_VideoController get name 2>/dev/null | grep -iq "amd\|radeon"; then
        gpu_type="amd"
        gpu_name=$(wmic path win32_VideoController get name 2>/dev/null | grep -i "amd\|radeon" | head -1 | xargs)
        echo "  ✓ AMD GPU detected: $gpu_name" >&2
    # Try Intel detection
    elif command -v lspci >/dev/null 2>&1 && lspci 2>/dev/null | grep -iq "intel.*vga\|intel.*display"; then
        gpu_type="intel"
        gpu_name=$(lspci 2>/dev/null | grep -i "intel.*vga\|intel.*display" | head -1 | cut -d':' -f3 | xargs)
        echo "  ✓ Intel GPU detected: $gpu_name" >&2
    elif command -v lshw >/dev/null 2>&1 && lshw -c display 2>/dev/null | grep -iq "intel"; then
        gpu_type="intel"
        gpu_name=$(lshw -c display 2>/dev/null | grep -i "product" | head -1 | cut -d':' -f2 | xargs)
        echo "  ✓ Intel GPU detected: $gpu_name" >&2
    elif [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" ]] && command -v wmic >/dev/null 2>&1 && wmic path win32_VideoController get name 2>/dev/null | grep -iq "intel"; then
        gpu_type="intel"
        gpu_name=$(wmic path win32_VideoController get name 2>/dev/null | grep -i "intel" | head -1 | xargs)
        echo "  ✓ Intel GPU detected: $gpu_name" >&2
    else
        echo "  ⚠ No GPU detected, will use CPU encoding" >&2
        gpu_name="CPU"
    fi
    
    # Test encoder availability and speed
    local best_encoder=""
    local best_time=999999
    local benchmark_score=0
    
    # Test candidates based on GPU type
    local candidates=()
    case $gpu_type in
        nvidia) candidates=("hevc_nvenc" "av1_nvenc" "libsvtav1" "libx265") ;;
        amd)    candidates=("hevc_amf" "av1_amf" "libsvtav1" "libx265") ;;
        intel)  candidates=("hevc_qsv" "av1_qsv" "libsvtav1" "libx265") ;;
        *)      candidates=("libsvtav1" "libx265") ;;
    esac
    
    echo "  Testing encoders..." >&2
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
                if $timeout_cmd ffmpeg -hide_banner -loglevel error -y -f lavfi -i "testsrc=duration=2:size=640x480:rate=30" \
                    -c:v "$enc" -f null - >/dev/null 2>&1; then
                    end_time=$(date +%s)
                    test_time=$((end_time - start_time))
                    # Calculate benchmark score (higher is better)
                    benchmark_score=$((1000 / test_time))
                    echo "      $enc: ${test_time}s (Score: $benchmark_score)" >&2
                    if [[ $test_time -lt $best_time ]]; then
                        best_time=$test_time
                        best_encoder=$enc
                        benchmark_score=$((1000 / best_time))
                    fi
                else
                    echo "      $enc: FAILED" >&2
                fi
            else
                # Windows fallback - run without timeout but limit test duration
                ffmpeg -hide_banner -loglevel error -y -f lavfi -i "testsrc=duration=2:size=640x480:rate=30" \
                    -c:v "$enc" -f null - >/dev/null 2>&1 &
                ffmpeg_pid=$!
                sleep 8  # Wait max 8 seconds
                if kill -0 $ffmpeg_pid 2>/dev/null; then
                    kill $ffmpeg_pid 2>/dev/null
                    wait $ffmpeg_pid 2>/dev/null
                    echo "      $enc: TIMEOUT" >&2
                else
                    wait $ffmpeg_pid
                    end_time=$(date +%s)
                    test_time=$((end_time - start_time))
                    if [[ $test_time -gt 0 ]]; then
                        benchmark_score=$((1000 / test_time))
                    else
                        benchmark_score=1
                    fi
                    echo "      $enc: ${test_time}s (Score: $benchmark_score)" >&2
                    if [[ $test_time -lt $best_time ]]; then
                        best_time=$test_time
                        best_encoder=$enc
                        benchmark_score=$((1000 / best_time))
                    fi
                fi
            fi
        else
            echo "      $enc: NOT AVAILABLE" >&2
        fi
    done
    
    # Display results with ASCII thumbs up
    echo
    echo "  Your $gpu_name was detected" >&2
    echo "  The best encoder for you is $best_encoder" >&2
    echo "  Your benchmark score was $benchmark_score" >&2
    echo
    echo "     ( ͡° ͜ʖ ͡°)" >&2
    echo
    
    # Cache results
    cat > "$CACHE_FILE" << EOF
cached_gpu="$gpu_name"
cached_encoder="$best_encoder"
cached_score="$benchmark_score"
cached_gpu_type="$gpu_type"
EOF
    
    optimal_encoder="$best_encoder"
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

# Check for cached hardware results
CACHE_FILE="$SCRIPT_DIR/.hardware_cache"
SETTINGS_FILE="$SCRIPT_DIR/.user_settings"

if [[ -f "$CACHE_FILE" ]]; then
    echo
    echo "╔═══════════════════════════════════════════════════════════════╗"
    echo "║                    Hardware Benchmarking                         ║"
    echo "╚═══════════════════════════════════════════════════════════════╝"
    echo
    echo "   1) Convert (Skip Benchmark) - Use cached settings"
    echo "   2) Run Hardware Benchmarking"
    echo
    
    while true; do
        read -p "   Enter 1–2 → " bench_choice
        if [[ -n "$bench_choice" && "$bench_choice" =~ ^[1-2]$ ]]; then
            break
        else
            echo "   Invalid input. Please enter 1 or 2."
        fi
    done
    
    if [[ "$bench_choice" == "1" ]]; then
        # Load cached results
        source "$CACHE_FILE"
        echo "✅ Using cached hardware settings"
        echo "   Your $cached_gpu was detected"
        echo "   The best encoder for you is $cached_encoder"
        echo "   Your benchmark score was $cached_score"
        echo
        optimal_encoder="$cached_encoder"
    else
        echo -e "\n🔍 Running hardware benchmarking..."
        run_hardware_benchmark
    fi
else
    echo -e "\n🔍 First-time setup - running hardware benchmarking..."
    run_hardware_benchmark
fi

echo
echo "Press space to continue..."
read -n 1 -s
clear

# Get user settings
echo
echo "╔═══════════════════════════════════════════════════════════════╗"
echo "║                    Choose Encoder/GPU                         ║"
echo "╚═══════════════════════════════════════════════════════════════╝"
echo
echo "   1) Auto-detect optimal encoder (recommended)"
echo "   2) NVIDIA GPU encoding"
echo "   3) AMD GPU encoding"
echo "   4) CPU encoding"
echo

while true; do
    read -p "   Enter 1–4 → " enc_choice
    if [[ -n "$enc_choice" && "$enc_choice" =~ ^[1-4]$ ]]; then
        break
    else
        echo "   Invalid input. Please enter a number between 1 and 4."
    fi
done

case $enc_choice in
    1) 
        echo "✅ Auto-detected: $optimal_encoder"
        ;;
    2) 
        # Auto-select best NVIDIA encoder
        if ffmpeg -hide_banner -loglevel error -encoders | grep -q "hevc_nvenc"; then
            optimal_encoder="hevc_nvenc"
            echo "✅ Selected: NVIDIA HEVC NVENC"
        elif ffmpeg -hide_banner -loglevel error -encoders | grep -q "av1_nvenc"; then
            optimal_encoder="av1_nvenc"
            echo "✅ Selected: NVIDIA AV1 NVENC"
        else
            echo "❌ No NVIDIA encoders available, using auto-detected"
        fi
        ;;
    3) 
        # Auto-select best AMD encoder
        if ffmpeg -hide_banner -loglevel error -encoders | grep -q "hevc_amf"; then
            optimal_encoder="hevc_amf"
            echo "✅ Selected: AMD HEVC AMF"
        elif ffmpeg -hide_banner -loglevel error -encoders | grep -q "av1_amf"; then
            optimal_encoder="av1_amf"
            echo "✅ Selected: AMD AV1 AMF"
        else
            echo "❌ No AMD encoders available, using auto-detected"
        fi
        ;;
    4) 
        # Auto-select best CPU encoder
        if ffmpeg -hide_banner -loglevel error -encoders | grep -q "libsvtav1"; then
            optimal_encoder="libsvtav1"
            echo "✅ Selected: CPU SVT-AV1"
        else
            optimal_encoder="libx265"
            echo "✅ Selected: CPU x265 HEVC"
        fi
        ;;
    esac

# Settings persistence functions
save_user_settings() {
    cat > "$SETTINGS_FILE" << EOF
# User settings cache - GIT Converter v2.7
cached_encoder="$optimal_encoder"
cached_resolution="$res_name"
cached_fps_choice="$fps_choice"
cached_quality_choice="$b"
cached_color_choice="$color_opt"
cached_modular_choice="$op_choice"
cached_scale="$scale"
cached_scale_flags="$scale_flags"
cached_quality_params="$quality_params"
cached_quality_name="$quality_name"
cached_color_filter="$color_filter"
cached_color_suf="$color_suf"
cached_fps_filter="$fps_filter"
cached_suf="$suf"
EOF
}

load_user_settings() {
    if [[ -f "$SETTINGS_FILE" ]]; then
        echo
        echo "╔═══════════════════════════════════════════════════════════════╗"
        echo "║                    Previous Settings Found                      ║"
        echo "╚═══════════════════════════════════════════════════════════════╝"
        echo
        echo "   1) Use previous settings"
        echo "   2) Configure new settings"
        echo
        
        while true; do
            read -p "   Enter 1–2 → " settings_choice
            if [[ -n "$settings_choice" && "$settings_choice" =~ ^[1-2]$ ]]; then
                break
            else
                echo "   Invalid input. Please enter 1 or 2."
            fi
        done
        
        if [[ "$settings_choice" == "1" ]]; then
            source "$SETTINGS_FILE"
            echo "✅ Using previous settings"
            echo "   Encoder: $cached_encoder"
            echo "   Resolution: $cached_resolution"
            echo "   Quality: $cached_quality_name"
            echo "   Color: ${cached_color_suf:-None}"
            echo "   FPS: ${cached_suf:-Original}"
            echo
            optimal_encoder="$cached_encoder"
            res_name="$cached_resolution"
            scale="$cached_scale"
            scale_flags="$cached_scale_flags"
            fps_choice="$cached_fps_choice"
            fps_filter="$cached_fps_filter"
            suf="$cached_suf"
            b="$cached_quality_choice"
            quality_params="$cached_quality_params"
            quality_name="$cached_quality_name"
            color_opt="$cached_color_choice"
            color_filter="$cached_color_filter"
            color_suf="$cached_color_suf"
            return 0
        fi
    fi
    return 1
}

setup_codec_and_container
if ! load_user_settings; then
    get_modular_operations
    get_resolution_settings
    get_fps_settings
    get_quality_settings "$optimal_encoder"
    get_color_correction
    
    # Save settings for next time
    save_user_settings
fi

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