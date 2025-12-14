#!/bin/bash
# ===================================================================
#  Hardware Detection Module - GIT Converter v2.7
#  Detects GPU and selects optimal encoder
# ===================================================================

select_encoder() {
    echo
    echo "╔═══════════════════════════════════════════════════════════════╗"
    echo "║                    Choose Encoder/GPU                         ║"
    echo "╚═══════════════════════════════════════════════════════════════╝"
    echo
    echo "   1) Auto-detect optimal encoder (recommended)"
    echo "   2) NVIDIA NVENC (HEVC/AV1)"
    echo "   3) AMD AMF (HEVC/AV1)"
    echo "   4) Intel Quick Sync (HEVC/AV1)"
    echo "   5) CPU encoding (SVT-AV1/x265)"
    echo "   6) Custom encoder selection"
    echo

    while true; do
        read -p "   Enter 1–6 → " enc_choice
        if [[ -n "$enc_choice" && "$enc_choice" =~ ^[1-6]$ ]]; then
            break
        else
            echo "   Invalid input. Please enter a number between 1 and 6."
        fi
    done

    case $enc_choice in
        1) 
            echo -e "\n🔍 Auto-detecting optimal encoder..."
            optimal_encoder=$(auto_detect_encoder)
            echo "✅ Selected: $optimal_encoder"
            ;;
        2) 
            optimal_encoder=$(select_nvidia_encoder)
            ;;
        3) 
            optimal_encoder=$(select_amd_encoder)
            ;;
        4) 
            optimal_encoder=$(select_intel_encoder)
            ;;
        5) 
            optimal_encoder=$(select_cpu_encoder)
            ;;
        6) 
            optimal_encoder=$(select_custom_encoder)
            ;;
    esac
    
    echo "$optimal_encoder"
}

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
    elif command -v lshw >/dev/null 2>&1 && lshw -c display 2>/dev/null | grep -iq "amd\|radeon"; then
        gpu_type="amd"
        echo "  ✓ AMD GPU detected (via lshw)" >&2
    elif [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" ]] && command -v wmic >/dev/null 2>&1 && wmic path win32_VideoController get name 2>/dev/null | grep -iq "amd\|radeon"; then
        gpu_type="amd"
        echo "  ✓ AMD GPU detected (via wmic)" >&2
    # Try Intel detection
    elif command -v lspci >/dev/null 2>&1 && lspci 2>/dev/null | grep -iq "intel.*vga\|intel.*display"; then
        gpu_type="intel"
        echo "  ✓ Intel GPU detected (via lspci)" >&2
    elif command -v lshw >/dev/null 2>&1 && lshw -c display 2>/dev/null | grep -iq "intel"; then
        gpu_type="intel"
        echo "  ✓ Intel GPU detected (via lshw)" >&2
    elif [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" ]] && command -v wmic >/dev/null 2>&1 && wmic path win32_VideoController get name 2>/dev/null | grep -iq "intel"; then
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
        echo "$best_encoder"
    else
        echo "  ⚠ No working encoder found, defaulting to libx265" >&2
        echo "libx265"
    fi
}

select_nvidia_encoder() {
    echo -e "\n🎮 NVIDIA Encoder Selection:"
    echo "1) HEVC NVENC (recommended)"
    echo "2) AV1 NVENC (newer, slower)"
    
    while true; do
        read -p "Enter choice [1-2]: " nvidia_choice
        case $nvidia_choice in
            1) echo "✅ Selected HEVC NVENC"; echo "hevc_nvenc"; return ;;
            2) echo "✅ Selected AV1 NVENC"; echo "av1_nvenc"; return ;;
            *) echo "❌ Invalid choice. Please enter 1 or 2." ;;
        esac
    done
}

select_amd_encoder() {
    echo -e "\n🎮 AMD Encoder Selection:"
    echo "1) HEVC AMF (recommended)"
    echo "2) AV1 AMF (newer, slower)"
    
    while true; do
        read -p "Enter choice [1-2]: " amd_choice
        case $amd_choice in
            1) echo "✅ Selected HEVC AMF"; echo "hevc_amf"; return ;;
            2) echo "✅ Selected AV1 AMF"; echo "av1_amf"; return ;;
            *) echo "❌ Invalid choice. Please enter 1 or 2." ;;
        esac
    done
}

select_intel_encoder() {
    echo -e "\n🎮 Intel Encoder Selection:"
    echo "1) HEVC Quick Sync (recommended)"
    echo "2) AV1 Quick Sync (newer, slower)"
    
    while true; do
        read -p "Enter choice [1-2]: " intel_choice
        case $intel_choice in
            1) echo "✅ Selected HEVC QSV"; echo "hevc_qsv"; return ;;
            2) echo "✅ Selected AV1 QSV"; echo "av1_qsv"; return ;;
            *) echo "❌ Invalid choice. Please enter 1 or 2." ;;
        esac
    done
}

select_cpu_encoder() {
    echo -e "\n💻 CPU Encoder Selection:"
    echo "1) SVT-AV1 (recommended, faster)"
    echo "2) x265 HEVC (mature, compatible)"
    
    while true; do
        read -p "Enter choice [1-2]: " cpu_choice
        case $cpu_choice in
            1) echo "✅ Selected SVT-AV1"; echo "libsvtav1"; return ;;
            2) echo "✅ Selected x265"; echo "libx265"; return ;;
            *) echo "❌ Invalid choice. Please enter 1 or 2." ;;
        esac
    done
}

select_custom_encoder() {
    echo -e "\n⚙️ Available Encoders:"
    ffmpeg -hide_banner -encoders | grep -E "(hevc|av1|h265)" | grep -v "V\|D" | awk '{print $2}' | nl
    
    while true; do
        read -p "Enter encoder name: " custom_enc
        if ffmpeg -hide_banner -loglevel error -encoders | grep -q "$custom_enc"; then
            echo "✅ Selected: $custom_enc"
            echo "$custom_enc"
            return
        else
            echo "❌ Encoder '$custom_enc' not found. Please try again."
        fi
    done
}

detect_hardware() {
    select_encoder
    SELECTED_ENCODER=$optimal_encoder
}