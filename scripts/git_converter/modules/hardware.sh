#!/bin/bash
# ===================================================================
#  Hardware Detection Module - GIT Converter v2.7
#  Optimized GPU and encoder detection
# ===================================================================

detect_hardware() {
    echo "Detecting hardware and optimal encoder..." >&2
    
    # Detect GPU type
    gpu_type="none"
    if command -v nvidia-smi >/dev/null 2>&1 && nvidia-smi >/dev/null 2>&1; then
        gpu_type="nvidia"
        echo "  ✓ NVIDIA GPU detected" >&2
    elif command -v lspci >/dev/null 2>&1 && lspci 2>/dev/null | grep -iq "amd\|radeon\|advanced micro devices"; then
        gpu_type="amd"
        echo "  ✓ AMD GPU detected" >&2
    elif command -v lspci >/dev/null 2>&1 && lspci 2>/dev/null | grep -iq "intel.*vga\|intel.*display"; then
        gpu_type="intel"
        echo "  ✓ Intel GPU detected" >&2
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
            start_time=$(date +%s.%N)
            if ffmpeg -hide_banner -loglevel error -y -f lavfi -i "testsrc=duration=2:size=320x240:rate=1" \
                -c:v "$enc" -f null - >/dev/null 2>&1; then
                end_time=$(date +%s.%N)
                test_time=$(echo "$end_time - $start_time" | bc -l 2>/dev/null || echo "1")
                echo "    $enc: ${test_time}s" >&2
                if (( $(echo "$test_time < $best_time" | bc -l 2>/dev/null || echo "0") )); then
                    best_time=$test_time
                    best_encoder=$enc
                fi
            fi
        fi
    done
    
    if [[ -n "$best_encoder" ]]; then
        echo "  ✓ Selected: $best_encoder (fastest encoder)" >&2
        echo "" >&2
        echo "Hardware Detection Summary:" >&2
        echo "  GPU Type: $gpu_type" >&2
        echo "  Available Encoders Tested: ${candidates[*]}" >&2
        echo "  Optimal Encoder: $best_encoder" >&2
        echo "  Benchmark Time: ${best_time}s" >&2
        echo "" >&2
        echo "Press space to continue..." >&2
        read -n 1 -s
        echo "$best_encoder"
        return 0
    else
        echo "  ⚠ No working encoder found, defaulting to libx265" >&2
        echo "" >&2
        echo "Hardware Detection Summary:" >&2
        echo "  GPU Type: $gpu_type" >&2
        echo "  Available Encoders Tested: ${candidates[*]}" >&2
        echo "  Fallback Encoder: libx265" >&2
        echo "" >&2
        echo "Press space to continue..." >&2
        read -n 1 -s
        echo "libx265"
        return 0
    fi
}