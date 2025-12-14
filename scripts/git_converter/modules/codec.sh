#!/bin/bash

# Codec and container selection module
# Sets output codec and container format

select_codec() {
    echo -e "\n🎬 Select output codec:"
    echo "1) AV1 (best compression, newer)"
    echo "2) HEVC (good compatibility, mature)"
    
    while true; do
        read -p "Enter choice [1-2]: " codec_choice
        case $codec_choice in
            1) 
                OUTPUT_CODEC="av1"
                ENCODER="libsvtav1"
                echo "✅ Selected AV1 codec"
                break
                ;;
            2) 
                OUTPUT_CODEC="hevc"
                ENCODER="libx265"
                echo "✅ Selected HEVC codec"
                break
                ;;
            *) 
                echo "❌ Invalid choice. Please enter 1 or 2."
                ;;
        esac
    done
}

select_container() {
    echo -e "\n📦 Select output container:"
    echo "1) MKV (flexible, supports all features)"
    echo "2) MP4 (better device compatibility)"
    
    while true; do
        read -p "Enter choice [1-2]: " container_choice
        case $container_choice in
            1) 
                OUTPUT_CONTAINER="mkv"
                echo "✅ Selected MKV container"
                break
                ;;
            2) 
                OUTPUT_CONTAINER="mp4"
                echo "✅ Selected MP4 container"
                break
                ;;
            *) 
                echo "❌ Invalid choice. Please enter 1 or 2."
                ;;
        esac
    done
}

setup_codec_and_container() {
    select_codec
    select_container
    
    # Set output filename
    local base_name="${INPUT_FILE%.*}"
    OUTPUT_FILE="${base_name}_converted.${OUTPUT_CONTAINER}"
    
    echo -e "\n🎯 Codec & Container Configuration:"
    echo "Codec: $OUTPUT_CODEC ($ENCODER)"
    echo "Container: $OUTPUT_CONTAINER"
    echo "Output file: $OUTPUT_FILE"
}