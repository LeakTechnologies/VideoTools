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

   1) Fast Bitrate (200+ FPS) - Legacy speed
   2) Source quality (no changes unless required)
   3) High Quality (CRF 18) - Recommended
   4) Near-Lossless (CRF 16) - Maximum quality
   5) Good Quality (CRF 20) - Balanced
   6) Custom bitrate (exact bitrate control)
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
            # Fast bitrate mode - legacy style
            echo
            echo "Choose target bitrate:"
            echo "  1) 1800 kbps (~400 MB per 30 min)"
            echo "  2) 2000 kbps (~440 MB per 30 min)"
            echo "  3) 2300 kbps (~510 MB per 30 min)"
            echo "  4) 2600 kbps (~580 MB per 30 min)"
            echo "  5) 2900 kbps (~640 MB per 30 min)"
            echo "  6) 3200 kbps (~710 MB per 30 min)"
            echo "  7) 3500 kbps (~780 MB per 30 min)"
            echo
            
            while true; do
                read -p "Enter 1–7 → " bitrate_choice
                if [[ -n "$bitrate_choice" && "$bitrate_choice" =~ ^[1-7]$ ]]; then
                    break
                else
                    echo "Invalid input. Please enter a number between 1 and 7."
                fi
            done
            
            case $bitrate_choice in
                1) quality_params="-b:v 1800k"; quality_name="Fast 1800k" ;;
                2) quality_params="-b:v 2000k"; quality_name="Fast 2000k" ;;
                3) quality_params="-b:v 2300k"; quality_name="Fast 2300k" ;;
                4) quality_params="-b:v 2600k"; quality_name="Fast 2600k" ;;
                5) quality_params="-b:v 2900k"; quality_name="Fast 2900k" ;;
                6) quality_params="-b:v 3200k"; quality_name="Fast 3200k" ;;
                7) quality_params="-b:v 3500k"; quality_name="Fast 3500k" ;;
                *) quality_params="-b:v 2400k"; quality_name="Fast 2400k" ;;
            esac
            ;;
        2) 
            quality_params=""
            quality_name="Source quality"
            ;;
        3) 
            if [[ "$encoder" == *"av1"* ]]; then
                quality_params="-crf 18 -preset 6"
                quality_name="AV1 CRF 18"
            else
                quality_params="-crf 18 -quality 23"
                quality_name="HEVC CRF 18"
            fi
            ;;
        4) 
            if [[ "$encoder" == *"av1"* ]]; then
                quality_params="-crf 16 -preset 4"
                quality_name="AV1 CRF 16"
            else
                quality_params="-crf 16 -quality 28"
                quality_name="HEVC CRF 16"
            fi
            ;;
        5) 
            if [[ "$encoder" == *"av1"* ]]; then
                quality_params="-crf 20 -preset 6"
                quality_name="AV1 CRF 20"
            else
                quality_params="-crf 20 -quality 23"
                quality_name="HEVC CRF 20"
            fi
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