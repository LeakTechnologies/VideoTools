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
    
    for f in "${video_files[@]}"; do
        [[ -f "$f" ]] || continue

        # Extract basename for output filename
        basename_f=$(basename "$f")
        out="$OUT/${basename_f%.*}${suf}${color_suf}__cv.$ext"
        [[ -f "$out" ]] && { echo "SKIP $f (already exists)"; continue; }

        encode_video "$f" "$out" "$encoder" "$quality_params" "$filter_chain"
        echo
    done
}