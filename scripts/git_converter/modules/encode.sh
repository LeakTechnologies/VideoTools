#!/bin/bash
# ===================================================================
#  Encoding Module - GIT Converter v2.7
#  Core ffmpeg execution logic
# ===================================================================

encode_video() {
    local input_file="$1"
    local output_file="$2"
    local encoder="$3"
    local quality_params="$4"
    local filter_chain="$5"
    
    echo "Processing: $input_file → $(basename "$output_file")"
    
    # Stream copy mode: when user explicitly chooses no processing
    if [[ "$DO_RESOLUTION" == "false" && "$DO_FPS" == "false" && "$DO_COLOR" == "false" ]]; then
        echo "🚀 STREAM COPY: No processing selected"
        ffmpeg -y -i "$input_file" -c:v copy -c:a copy "$output_file"
        return $?
    fi
    
    # Fast bypass mode: stream copy when no processing needed
    if [[ -z "$filter_chain" && -z "$quality_params" ]]; then
        echo "🚀 FAST BYPASS: Stream copy (no re-encoding)"
        ffmpeg -y -i "$input_file" -c:v copy -c:a copy "$output_file"
        return $?
    fi
    
    # Build optimized ffmpeg command
    if [[ -n "$filter_chain" ]]; then
        ffmpeg -y -i "$input_file" -pix_fmt yuv420p -vf "$filter_chain" \
               -c:v "$encoder" $quality_params -c:a aac -b:a 192k -ac 2 "$output_file"
    else
        ffmpeg -y -i "$input_file" -pix_fmt yuv420p \
               -c:v "$encoder" $quality_params -c:a aac -b:a 192k -ac 2 "$output_file"
    fi
    
    if [[ $? -eq 0 ]]; then
        echo "DONE → $(basename "$output_file")"
        return 0
    else
        echo "ERROR → Failed to process $input_file"
        return 1
    fi
}

process_files() {
    local video_files=("$@")
    local encoder="$1"
    local quality_params="$2"
    local filter_chain="$3"
    local suf="$4"
    local color_suf="$5"
    local ext="$6"
    
    # Simple queue processing
    local total_files=${#video_files[@]}
    local current_file=0
    
    echo "📋 Queue: $total_files file(s) to process"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo
    
    for f in "${video_files[@]}"; do
        [[ -f "$f" ]] || continue
        
        ((current_file++))
        
        # Extract basename for output filename
        basename_f=$(basename "$f")
        out="$OUT/${basename_f%.*}${suf}${color_suf}__cv.$ext"
        
        echo "📁 [$current_file/$total_files] Processing: $basename_f"
        
        if [[ -f "$out" ]]; then
            echo "⚠️  SKIP - Output file already exists"
            echo
            continue
        fi

        encode_video "$f" "$out" "$encoder" "$quality_params" "$filter_chain"
        
        if [[ $? -eq 0 ]]; then
            echo "✅ [$current_file/$total_files] COMPLETED: $(basename "$out")"
        else
            echo "❌ [$current_file/$total_files] FAILED: $basename_f"
        fi
        
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo
    done
    
    echo "🎉 Queue processing complete!"
    echo "📁 All files saved to: $OUT"
}