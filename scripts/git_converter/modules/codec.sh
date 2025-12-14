#!/bin/bash
# ===================================================================
#  Codec and Container Selection Module - GIT Converter v2.7
#  User choice for codec and container type
# ===================================================================

get_codec_and_container_settings() {
    local encoder="$1"
    
    clear
    cat << "EOF"
╔═════════════════════════════════════════════════════════════╗
║                    Choose Codec & Container                 ║
╚═════════════════════════════════════════════════════════╝

   1) AV1 Encoding
      - MKV container (recommended)
      - MP4 container
   
   2) HEVC Encoding
      - MKV container (recommended)
      - MP4 container
EOF
    echo

    while true; do
        read -p "   Enter 1–2 → " codec_choice
        if [[ -n "$codec_choice" && "$codec_choice" =~ ^[1-2]$ ]]; then
            break
        else
            echo "   Invalid input. Please enter 1 or 2."
        fi
    done

    # Get container preference
    if [[ "$codec_choice" =~ ^[1-2]$ ]]; then
        echo
        echo "Choose container for $( [[ "$codec_choice" == "1" ]] && echo "AV1" || echo "HEVC"):"
        echo "   1) MKV (recommended)"
        echo "   2) MP4"
        echo
        
        while true; do
            read -p "   Enter 1–2 → " container_choice
            if [[ -n "$container_choice" && "$container_choice" =~ ^[1-2]$ ]]; then
                break
            else
                echo "   Invalid input. Please enter 1 or 2."
            fi
        done
        
        case $container_choice in
            1) ext="mkv" ;;
            2) ext="mp4" ;;
        esac
    else
        ext="mkv"  # Default fallback
    fi
    
    # Set encoder based on choice
    case $codec_choice in
        1) 
            # AV1 encoding - use detected encoder if AV1-capable
            if [[ "$encoder" == *"av1"* ]]; then
                final_encoder="$encoder"
            else
                # Fallback to SVT-AV1 if detected encoder doesn't support AV1
                final_encoder="libsvtav1"
            fi
            ;;
        2) 
            # HEVC encoding - use detected encoder
            final_encoder="$encoder"
            ;;
        *)
            final_encoder="$encoder"  # Fallback
            ;;
    esac
    
    # Export variables for main script
    echo "$final_encoder"
    echo "$ext"
}