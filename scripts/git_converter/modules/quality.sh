#!/bin/bash
# ===================================================================
#  Quality Settings Module - GIT Converter v2.7
#  Handles quality modes and encoding parameters
# ===================================================================

get_quality_settings() {
    local encoder="$1"
    
    clear
    cat << "EOF"
╔═══════════════════════════════════════════════════════════╗
║                    Choose Quality Mode                       ║
╚═════════════════════════════════════════════════════════════╝

   1) Source quality (no changes unless required)
   2) High Quality (CRF 18) - Recommended
   3) Near-Lossless (CRF 16) - Maximum quality
   4) Good Quality (CRF 20) - Balanced
   5) Custom bitrate (exact bitrate control)
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

    case $b in
        1) 
            quality_params=""
            quality_name="Source quality"
            ;;
        2) 
            if [[ "$encoder" == *"av1"* ]]; then
                quality_params="-crf 18 -preset 6"
                quality_name="AV1 CRF 18"
            else
                quality_params="-crf 18 -quality 23"
                quality_name="HEVC CRF 18"
            fi
            ;;
        3) 
            if [[ "$encoder" == *"av1"* ]]; then
                quality_params="-crf 16 -preset 4"
                quality_name="AV1 CRF 16"
            else
                quality_params="-crf 16 -quality 28"
                quality_name="HEVC CRF 16"
            fi
            ;;
        4) 
            if [[ "$encoder" == *"av1"* ]]; then
                quality_params="-crf 20 -preset 6"
                quality_name="AV1 CRF 20"
            else
                quality_params="-crf 20 -quality 23"
                quality_name="HEVC CRF 20"
            fi
            ;;
        5) 
            echo
            echo "Enter bitrate (e.g., 5000k, 8000k):"
            read -p "→ " custom_bitrate
            # Clean input - remove control characters and spaces
            custom_bitrate=$(echo "$custom_bitrate" | tr -cd '[:alnum:]k')
            if [[ -n "$custom_bitrate" && "$custom_bitrate" =~ ^[0-9]+k$ ]]; then
                quality_params="-b:v $custom_bitrate"
                quality_name="Custom $custom_bitrate"
            else
                quality_params="-crf 18 -quality 23"
                quality_name="HEVC CRF 18"
            fi
            ;;
    esac
}