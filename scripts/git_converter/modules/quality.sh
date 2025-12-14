#!/bin/bash
# ===================================================================
#  Optimized Quality Settings Module - GIT Converter v2.7
#  Encoder-specific optimal parameters for maximum speed
# ===================================================================

get_quality_settings() {
    local encoder="$1"
    
    clear
    cat << "EOF"
╔═══════════════════════════════════════════════════════════════════╗
║                    Choose Quality Mode                       ║
╚═══════════════════════════════════════════════════════════════╝

   1) Near-Lossless (CRF 16) - Maximum quality
   2) High Quality (CRF 18) - Recommended
   3) Good Quality (CRF 20) - Balanced
   4) Custom bitrate
EOF
    echo

    while true; do
        read -p "   Enter 1–4 → " b
        if [[ -n "$b" && "$b" =~ ^[1-4]$ ]]; then
            break
        else
            echo "   Invalid input. Please enter a number between 1 and 4."
        fi
    done

    # Encoder-specific optimal settings
    case $encoder in
        *nvenc*)
            quality_params="-crf ${b:16:18:20:22} -preset fast -tune ll"
            quality_name="NVENC CRF ${b:16:18:20:22}"
            ;;
        *amf*)
            quality_params="-crf ${b:16:18:20:22} -quality 23 -rc 1"
            quality_name="AMF CRF ${b:16:18:20:22}"
            ;;
        *qsv*)
            quality_params="-crf ${b:16:18:20:22} -preset fast -global_quality 23"
            quality_name="QSV CRF ${b:16:18:20:22}"
            ;;
        libsvtav1)
            quality_params="-crf ${b:16:18:20:22} -preset 8 -tile-rows 2 -tile-columns 2"
            quality_name="SVT-AV1 CRF ${b:16:18:20:22}"
            ;;
        libx265)
            quality_params="-crf ${b:16:18:20:22} -preset fast -tune fastdecode"
            quality_name="x265 CRF ${b:16:18:20:22}"
            ;;
        *)
            quality_params="-crf ${b:16:18:20:22}"
            quality_name="CRF ${b:16:18:20:22}"
            ;;
    esac
    
    # Handle custom bitrate with exact control and override option
    if [[ "$b" == "4" ]]; then
        echo
        echo "Choose custom bitrate mode:"
        echo "   1) Target bitrate (exact match)"
        echo "   2) Target bitrate with optimal adjustment"
        echo
        
        while true; do
            read -p "   Enter 1–2 → " bitrate_mode
            if [[ -n "$bitrate_mode" && "$bitrate_mode" =~ ^[1-2]$ ]]; then
                break
            else
                echo "   Invalid input. Please enter 1 or 2."
            fi
        done
        
        if [[ "$bitrate_mode" == "1" ]]; then
            echo
            echo "Enter target bitrate (e.g., 3200k, 5000k, 8000k):"
            read -p "→ " custom_bitrate
            
            # Clean input - remove control characters and spaces
            custom_bitrate=$(echo "$custom_bitrate" | tr -cd '[:alnum:]k')
            
            # Validate exact bitrate format
            if [[ -n "$custom_bitrate" && "$custom_bitrate" =~ ^[0-9]+k$ ]]; then
                quality_params="-b:v $custom_bitrate"
                quality_name="Exact $custom_bitrate"
            else
                echo "   Invalid bitrate format. Use format like 3200k, 5000k"
                quality_params="-crf 18"
                quality_name="HEVC CRF 18"
            fi
        else
            echo
            echo "Enter target bitrate (e.g., 3200k, 5000k, 8000k):"
            read -p "→ " custom_bitrate
            
            # Clean input - remove control characters and spaces
            custom_bitrate=$(echo "$custom_bitrate" | tr -cd '[:alnum:]k')
            
            # Validate and adjust bitrate to nearest standard tier for optimal compression
            if [[ -n "$custom_bitrate" && "$custom_bitrate" =~ ^[0-9]+k$ ]]; then
                # Extract numeric value and round to nearest standard bitrate
                target_kbps=$(echo "$custom_bitrate" | sed 's/k//')
                
                # Standard bitrate tiers with optimal compression
                case $target_kbps in
                    [0-1499]) adjusted_bitrate="1500k" ;;
                    [1500-2499]) adjusted_bitrate="2000k" ;;
                    [2500-3499]) adjusted_bitrate="2500k" ;;
                    [3500-4499]) adjusted_bitrate="3000k" ;;
                    [4500-5499]) adjusted_bitrate="3500k" ;;
                    [5500-6499]) adjusted_bitrate="4000k" ;;
                    [6500-7499]) adjusted_bitrate="4500k" ;;
                    [7500-8499]) adjusted_bitrate="5000k" ;;
                    [8500-9499]) adjusted_bitrate="5500k" ;;
                    [9500-11499]) adjusted_bitrate="6000k" ;;
                    [11500-13499]) adjusted_bitrate="7000k" ;;
                    [13500-15499]) adjusted_bitrate="8000k" ;;
                    *) adjusted_bitrate="9000k" ;;
                esac
            
            echo "  Target: $target_kbps kbps → Adjusted to: $adjusted_bitrate"
            
            case $encoder in
                *nvenc*)
                    quality_params="-b:v $adjusted_bitrate -preset fast -tune ll"
                    quality_name="Custom NVENC $adjusted_bitrate"
                    ;;
                *amf*)
                    quality_params="-b:v $adjusted_bitrate -quality 23 -rc 1"
                    quality_name="Custom AMF $adjusted_bitrate"
                    ;;
                *qsv*)
                    quality_params="-b:v $adjusted_bitrate -preset fast -global_quality 23"
                    quality_name="Custom QSV $adjusted_bitrate"
                    ;;
                libsvtav1)
                    quality_params="-b:v $adjusted_bitrate -preset 8 -tile-rows 2 -tile-columns 2"
                    quality_name="Custom SVT-AV1 $adjusted_bitrate"
                    ;;
                libx265)
                    quality_params="-b:v $adjusted_bitrate -preset fast -tune fastdecode"
                    quality_name="Custom x265 $adjusted_bitrate"
                    ;;
                *)
                    quality_params="-b:v $adjusted_bitrate"
                    quality_name="Custom $adjusted_bitrate"
                    ;;
            esac
        else
            quality_params="-crf 18"
            quality_name="HEVC CRF 18"
        fi
    fi
}