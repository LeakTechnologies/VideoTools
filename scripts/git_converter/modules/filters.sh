#!/bin/bash
# ===================================================================
#  Filters Module - GIT Converter v2.7
#  Handles scaling, color correction, and FPS
# ===================================================================

get_resolution_settings() {
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
        clear
        cat << "EOF"
╔═══════════════════════════════════════════════════════════════╗
║                    Choose Scaling Algorithm                   ║
╚═══════════════════════════════════════════════════════════════╝

   1) Bicubic (fast, good quality)
   2) Lanczos (best quality, slower)
   3) Bilinear (fastest, basic quality)
EOF
        echo

        while true; do
            read -p "   Enter 1–3 → " scale_opt
            if [[ -n "$scale_opt" && "$scale_opt" =~ ^[1-3]$ ]]; then
                break
            else
                echo "   Invalid input. Please enter a number between 1 and 3."
            fi
        done

        case $scale_opt in
            1) scale_flags="bicubic" ;;
            2) scale_flags="lanczos" ;;
            3) scale_flags="bilinear" ;;
        esac
    else
        scale_flags="lanczos"
    fi
}

get_fps_settings() {
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
    clear
    cat << "EOF"
╔═══════════════════════════════════════════════════════════════╗
║                    Color Correction Option                    ║
╚═══════════════════════════════════════════════════════════════╝

   1) No color correction
   2) Restore pink skin tones (Topaz AI fix)
   3) Warm color boost
   4) Cool color boost
   5) 2000s DVD Restore
   6) 90s Quality Restore
   7) VHS Quality Restore
   8) Anime Preservation (clean lines & colors)
EOF
    echo

    while true; do
        read -p "   Enter 1–8 → " color_opt
        if [[ -n "$color_opt" && "$color_opt" =~ ^[1-8]$ ]]; then
            break
        else
            echo "   Invalid input. Please enter a number between 1 and 8."
        fi
    done

    case $color_opt in
        1) color_filter=""; color_suf="" ;;
        2) color_filter="eq=contrast=1.05:brightness=0.02:saturation=1.1,hue=h=-0.02"; color_suf="_colorfix" ;;
        3) color_filter="eq=contrast=1.03:brightness=0.01:saturation=1.15,hue=h=-0.01"; color_suf="_warm" ;;
        4) color_filter="eq=contrast=1.03:brightness=0.01:saturation=0.95,hue=h=0.02"; color_suf="_cool" ;;
        5) color_filter="eq=contrast=1.08:brightness=0.03:saturation=1.2:gamma=0.95,unsharp=5:5:0.8:5:5:0.0"; color_suf="_dvdrestore" ;;
        6) color_filter="eq=contrast=1.12:brightness=0.05:saturation=1.3:gamma=0.92,unsharp=5:5:1.0:5:5:0.0,hqdn3d=3:2:2:3"; color_suf="_90srestore" ;;
        7) color_filter="eq=contrast=1.15:brightness=0.08:saturation=1.4:gamma=0.90,unsharp=5:5:1.0:5:5:0.0,hqdn3d=3:2:2:3"; color_suf="_vhsrestore" ;;
        8) color_filter="eq=contrast=1.02:brightness=0:saturation=1.05:gamma=1.0,unsharp=3:3:0.5:3:3:0.0,gradfun=2.5:2.5"; color_suf="_anime" ;;
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