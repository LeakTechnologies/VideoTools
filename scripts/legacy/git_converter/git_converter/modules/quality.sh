#!/bin/bash
# ===================================================================
#  Quality Settings Module - GIT Converter v2.7
#  Handles quality modes and encoding parameters
# ===================================================================

get_quality_settings() {
    local encoder="$1"
    
    clear
cat << "EOF"
╔═════════════════════════════════════════════════════════════╗
║                    Choose Quality Mode                       ║
╚═════════════════════════════════════════════════════════════╝

   1) Source quality (bypass mode)
   2) High Quality (CRF 18) - Recommended
   3) Good Quality (CRF 20) - Balanced
   4) DVD-NTSC Professional (MPEG-2)
   5) DVD-PAL Professional (MPEG-2)
   6) Custom bitrate
EOF
    echo

    while true; do
        read -p "   Enter 1–6 → " b
        if [[ -n "$b" && "$b" =~ ^[1-6]$ ]]; then
            break
        else
            echo "   Invalid input. Please enter a number between 1 and 6."
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
                quality_params="-crf 20 -preset 6"
                quality_name="AV1 CRF 20"
            else
                quality_params="-crf 20 -quality 23"
                quality_name="HEVC CRF 20"
            fi
            ;;
        4) 
            # DVD-NTSC Professional (MPEG-2)
            quality_params="-c:v mpeg2video -b:v 6000k -g 15 -bf 2 -sc_threshold 1000000000"
            quality_name="DVD-NTSC Professional"
            echo "✅ DVD-NTSC: 720×480 @ 29.97fps, MPEG-2, 6000kbps"
            ;;
        5) 
            # DVD-PAL Professional (MPEG-2)
            quality_params="-c:v mpeg2video -b:v 8000k -g 12 -bf 2 -sc_threshold 1000000000"
            quality_name="DVD-PAL Professional"
            echo "✅ DVD-PAL: 720×576 @ 25fps, MPEG-2, 8000kbps"
            ;;
        6) 
            echo
            echo "Enter bitrate (e.g., 5000k, 8000k):"
            read -p "→ " custom_bitrate
            # Clean input - remove control characters but allow numbers and 'k'
            custom_bitrate=$(echo "$custom_bitrate" | tr -cd '0123456789k')
            # Remove any trailing/leading spaces
            custom_bitrate=$(echo "$custom_bitrate" | xargs)
            
            if [[ -n "$custom_bitrate" ]]; then
                # Validate format (number followed by 'k')
                if [[ "$custom_bitrate" =~ ^[0-9]+k$ ]]; then
                    quality_params="-b:v $custom_bitrate"
                    quality_name="Custom $custom_bitrate"
                else
                    echo "Invalid format. Use format like: 3000k"
                    quality_params="-b:v 3000k"
                    quality_name="Custom 3000k"
                fi
            else
                echo "No input provided, using default 3000k"
                quality_params="-b:v 3000k"
                quality_name="Custom 3000k"
            fi
            ;;
    esac
}