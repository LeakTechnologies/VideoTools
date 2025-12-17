#!/bin/bash
# ===================================================================
#  Filters Module - GIT Converter v2.7
#  Handles scaling, color correction, and FPS
# ===================================================================

# Detect GPU type for accelerated scaling
detect_gpu_type() {
    if command -v nvidia-smi >/dev/null 2>&1 && nvidia-smi >/dev/null 2>&1; then
        echo "nvidia"
    elif command -v lspci >/dev/null 2>&1 && lspci 2>/dev/null | grep -iq "amd\|radeon"; then
        echo "amd"
    elif command -v lspci >/dev/null 2>&1 && lspci 2>/dev/null | grep -iq "intel.*vga\|intel.*display"; then
        echo "intel"
    elif [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" ]] && command -v wmic >/dev/null 2>&1; then
        if wmic path win32_VideoController get name 2>/dev/null | grep -iq "nvidia"; then
            echo "nvidia"
        elif wmic path win32_VideoController get name 2>/dev/null | grep -iq "amd\|radeon"; then
            echo "amd"
        elif wmic path win32_VideoController get name 2>/dev/null | grep -iq "intel"; then
            echo "intel"
        else
            echo "none"
        fi
    else
        echo "none"
    fi
}

get_modular_operations() {
    clear
    cat << "EOF"
╔═══════════════════════════════════════════════════════════════╗
║                    Choose Operations                           ║
╚═══════════════════════════════════════════════════════════════╝

   1) Resolution scaling only
   2) FPS conversion only  
   3) Color correction only
   4) Resolution + FPS
   5) Resolution + Color
   6) FPS + Color
   7) All operations (traditional mode)
   8) Stream copy (no processing)
EOF
    echo

    while true; do
        read -p "   Enter 1–8 → " op_choice
        if [[ -n "$op_choice" && "$op_choice" =~ ^[1-8]$ ]]; then
            break
        else
            echo "   Invalid input. Please enter a number between 1 and 8."
        fi
    done

    # Set operation flags
    case $op_choice in
        1) 
            DO_RESOLUTION=true; DO_FPS=false; DO_COLOR=false
            echo "✅ Resolution scaling only"
            ;;
        2) 
            DO_RESOLUTION=false; DO_FPS=true; DO_COLOR=false
            echo "✅ FPS conversion only"
            ;;
        3) 
            DO_RESOLUTION=false; DO_FPS=false; DO_COLOR=true
            echo "✅ Color correction only"
            ;;
        4) 
            DO_RESOLUTION=true; DO_FPS=true; DO_COLOR=false
            echo "✅ Resolution + FPS"
            ;;
        5) 
            DO_RESOLUTION=true; DO_FPS=false; DO_COLOR=true
            echo "✅ Resolution + Color"
            ;;
        6) 
            DO_RESOLUTION=false; DO_FPS=true; DO_COLOR=true
            echo "✅ FPS + Color"
            ;;
        7) 
            DO_RESOLUTION=true; DO_FPS=true; DO_COLOR=true
            echo "✅ All operations (traditional mode)"
            ;;
        8) 
            DO_RESOLUTION=false; DO_FPS=false; DO_COLOR=false
            echo "✅ Stream copy (no processing)"
            ;;
    esac
}

get_resolution_settings() {
    # Skip if resolution not selected in modular mode
    if [[ "$DO_RESOLUTION" == "false" ]]; then
        scale=""
        res_name="Source"
        return
    fi

    # Auto-detect black bars if source resolution selected
    detect_black_bars() {
        echo "🔍 Detecting black bars for optimal cropping..."
        local input_file="$1"
        
        # Use ffmpeg cropdetect to analyze black bars
        local crop_result=$(ffmpeg -ss 30 -i "$input_file" -t 5 -vf "cropdetect=24:16:0" -f null - 2>&1 | grep "crop=" | tail -1)
        
        if [[ -n "$crop_result" ]]; then
            echo "✅ Black bars detected: $crop_result"
            echo "   (Use advanced options to apply cropping)"
        else
            echo "✅ No significant black bars detected"
        fi
    }

    clear
    cat << "EOF"
╔═══════════════════════════════════════════════════════════════╗
║                    Choose Resolution                         ║
╚═══════════════════════════════════════════════════════════════╝

   1) Source file resolution (no upscale)
   2) 720p  (1280×720)
   3) 1080p (1920×1080)
   4) 1440p (2560×1440)
   5) 4K    (3840×2160)
   6) 2X Upscale
   7) 4X Upscale
EOF
    echo

    while true; do
        read -p "   Enter 1–7 → " res
        if [[ -n "$res" && "$res" =~ ^[1-7]$ ]]; then
            break
        else
            echo "   Invalid input. Please enter a number between 1 and 7."
        fi
    done

    case $res in
        1) scale=""                     ; res_name="Source"   ;;
        2) scale="1280:720"             ; res_name="720p"     ;;
        3) scale="1920:1080"            ; res_name="1080p"    ;;
        4) scale="2560:1440"            ; res_name="1440p"    ;;
        5) scale="3840:2160"            ; res_name="4K"       ;;
        6) scale="iw*2:ih*2"            ; res_name="2X"       ;;
        7) scale="iw*4:ih*4"            ; res_name="4X"       ;;
    esac

    # Scaling algorithm choice (only if upscaling)
    if [[ -n "$scale" ]]; then
        # Detect GPU for accelerated options
        gpu_type=$(detect_gpu_type)
        
        clear
        cat << "EOF"
╔═══════════════════════════════════════════════════════════════╗
║                    Choose Scaling Algorithm                   ║
╚═══════════════════════════════════════════════════════════════╝

   1) Bicubic (fast, good quality)
   2) Lanczos (best quality, slower)
   3) Bilinear (fastest, basic quality)
EOF
        
        # Add GPU accelerated options if available
        if [[ "$gpu_type" == "nvidia" ]]; then
            echo "   4) NVIDIA GPU Accelerated (fastest, good quality)"
        elif [[ "$gpu_type" == "amd" ]]; then
            echo "   4) AMD GPU Accelerated (fastest, good quality)"
        elif [[ "$gpu_type" == "intel" ]]; then
            echo "   4) Intel GPU Accelerated (fastest, good quality)"
        fi
        echo

        # Set max option based on GPU availability
        if [[ "$gpu_type" != "none" ]]; then
            max_opt=4
        else
            max_opt=3
        fi

        while true; do
            read -p "   Enter 1–$max_opt → " scale_opt
            if [[ -n "$scale_opt" && "$scale_opt" =~ ^[1-$max_opt]$ ]]; then
                break
            else
                echo "   Invalid input. Please enter a number between 1 and $max_opt."
            fi
        done

        case $scale_opt in
            1) scale_flags="bicubic" ;;
            2) scale_flags="lanczos" ;;
            3) scale_flags="bilinear" ;;
            4) 
                # GPU accelerated scaling
                case $gpu_type in
                    nvidia) 
                        scale_flags="bicubic:sws_flags=neighbor+accurate_rnd"
                        echo "✅ Using NVIDIA GPU accelerated scaling"
                        ;;
                    amd) 
                        scale_flags="bicubic:sws_flags=neighbor+accurate_rnd"
                        echo "✅ Using AMD GPU accelerated scaling"
                        ;;
                    intel) 
                        scale_flags="bicubic:sws_flags=neighbor+accurate_rnd"
                        echo "✅ Using Intel GPU accelerated scaling"
                        ;;
                esac
                ;;
        esac
    else
        scale_flags="lanczos"
    fi
}

get_fps_settings() {
    # Skip if FPS not selected in modular mode
    if [[ "$DO_FPS" == "false" ]]; then
        fps_filter=""
        suf=""
        return
    fi

    clear
    cat << "EOF"
╔═══════════════════════════════════════════════════════════════╗
║                    Choose FPS Mode                           ║
╚═══════════════════════════════════════════════════════════════╝

   1) Original FPS
   2) 60 FPS
EOF
    echo

    while true; do
        read -p "   Enter 1–2 → " fps_choice
        if [[ -n "$fps_choice" && "$fps_choice" =~ ^[1-2]$ ]]; then
            break
        else
            echo "   Invalid input. Please enter 1 or 2."
        fi
    done

    case $fps_choice in
        2) fps_filter="fps=60"; suf="_60fps" ;;
        *) fps_filter="";           suf=""       ;;
    esac
}

get_color_correction() {
    # Skip if color correction not selected in modular mode
    if [[ "$DO_COLOR" == "false" ]]; then
        color_filter=""
        color_suf=""
        return
    fi

    clear
    cat << "EOF"
╔═══════════════════════════════════════════════════════════════╗
║                    Color Correction Option                    ║
╚═══════════════════════════════════════════════════════════════╝

   1) No color correction
   2) Fix pink skin tones (Topaz AI)
   3) Warm enhancement
   4) Cool enhancement
   5) Advanced options (DVD/VHS/Anime)
EOF
    echo

    while true; do
        read -p "   Enter 1–5 → " color_opt
        if [[ -n "$color_opt" && "$color_opt" =~ ^[1-5]$ ]]; then
            break
        else
            echo "   Invalid input. Please enter a number between 1 and 5."
        fi
    done

    case $color_opt in
        1) color_filter=""; color_suf="" ;;
        2) color_filter="eq=contrast=1.05:brightness=0.02:saturation=1.1,hue=h=-0.02"; color_suf="_colorfix" ;;
        3) color_filter="eq=contrast=1.03:brightness=0.01:saturation=1.15,hue=h=-0.01"; color_suf="_warm" ;;
        4) color_filter="eq=contrast=1.03:brightness=0.01:saturation=0.95,hue=h=0.02"; color_suf="_cool" ;;
        5) get_advanced_color_correction ;;
    esac
}

get_advanced_color_correction() {
    clear
    cat << "EOF"
╔═══════════════════════════════════════════════════════════════╗
║                 Advanced Color Correction                       ║
╚═══════════════════════════════════════════════════════════════╝

   1) 2000s DVD Restore
   2) 90s Quality Restore  
   3) VHS Quality Restore
   4) Anime Preservation (clean lines & colors)
EOF
    echo

    while true; do
        read -p "   Enter 1–4 → " adv_color_opt
        if [[ -n "$adv_color_opt" && "$adv_color_opt" =~ ^[1-4]$ ]]; then
            break
        else
            echo "   Invalid input. Please enter a number between 1 and 4."
        fi
    done

    case $adv_color_opt in
        1) color_filter="eq=contrast=1.08:brightness=0.03:saturation=1.2:gamma=0.95,unsharp=5:5:0.8:5:5:0.0"; color_suf="_dvdrestore" ;;
        2) color_filter="eq=contrast=1.12:brightness=0.05:saturation=1.3:gamma=0.92,unsharp=5:5:1.0:5:5:0.0,hqdn3d=3:2:2:3"; color_suf="_90srestore" ;;
        3) color_filter="eq=contrast=1.15:brightness=0.08:saturation=1.4:gamma=0.90,unsharp=5:5:1.0:5:5:0.0,hqdn3d=3:2:2:3"; color_suf="_vhsrestore" ;;
        4) color_filter="eq=contrast=1.02:brightness=0:saturation=1.05:gamma=1.0,unsharp=3:3:0.5:3:3:0.0,gradfun=2.5:2.5"; color_suf="_anime" ;;
    esac
}

build_filter_chain() {
    filter_chain=""
    
    # Only build filter chain if actually needed
    if [[ -n "$scale" || -n "$fps_filter" || -n "$color_filter" ]]; then
        if [[ -n "$scale" ]]; then
            filter_chain="scale=${scale}:flags=${scale_flags}"
        fi
        
        if [[ -n "$fps_filter" ]]; then
            if [[ -n "$filter_chain" ]]; then
                filter_chain="${filter_chain},${fps_filter}"
            else
                filter_chain="$fps_filter"
            fi
        fi
        
        if [[ -n "$color_filter" ]]; then
            if [[ -n "$filter_chain" ]]; then
                filter_chain="${filter_chain},${color_filter}"
            else
                filter_chain="$color_filter"
            fi
        fi
    fi
}