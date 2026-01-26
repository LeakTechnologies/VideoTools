# Convert Module

The Convert module is the primary tool for video transcoding and format conversion in VideoTools.

## Overview

Convert handles all aspects of changing video codec, container format, quality, resolution, and aspect ratio. It's designed to be the most frequently used module for everyday video conversion tasks.

## Features

### Codec Support
- **H.264 (AVC)** - Universal compatibility, excellent quality/size balance
- **H.265 (HEVC)** - Better compression than H.264, smaller files
- **VP9** - Open-source, efficient for web delivery
- **AV1** - Next-gen codec, best compression (slower encoding)
- **Legacy codecs** - MPEG-4, MPEG-2, etc.

### Container Formats
- **MP4** - Universal playback support
- **MKV** - Feature-rich, supports multiple tracks
- **WebM** - Web-optimized format
- **MOV** - Apple/professional workflows
- **AVI** - Legacy format support

### Quality Presets

#### CRF (Constant Rate Factor)
Quality-based encoding for predictable visual results:
- **High Quality** - CRF 18 (near-lossless, large files)
- **Standard** - CRF 23 (recommended default)
- **Efficient** - CRF 28 (good quality, smaller files)
- **Compressed** - CRF 32 (streaming/preview)
- **Custom** - User-defined CRF value

#### Bitrate-Based
For specific file size targets:
- **High** - 8-12 Mbps (1080p) / 20-30 Mbps (4K)
- **Medium** - 4-6 Mbps (1080p) / 10-15 Mbps (4K)
- **Low** - 2-3 Mbps (1080p) / 5-8 Mbps (4K)
- **Custom** - User-defined bitrate

### Resolution & Aspect Ratio

#### Resolution Presets
- **Source** - Keep original resolution
- **4K** - 3840×2160
- **1440p** - 2560×1440
- **1080p** - 1920×1080
- **720p** - 1280×720
- **480p** - 854×480
- **Custom** - User-defined dimensions

#### Aspect Ratio Handling
- **Source** - Preserve original aspect ratio (default as of v0.1.0-dev7)
- **16:9** - Standard widescreen
- **4:3** - Classic TV/monitor ratio
- **1:1** - Square (social media)
- **9:16** - Vertical/mobile video
- **21:9** - Ultra-widescreen
- **Custom** - User-defined ratio

#### Aspect Ratio Methods
- **Auto** - Smart handling based on source/target
- **Letterbox** - Add black bars top/bottom
- **Pillarbox** - Add black bars left/right
- **Blur Fill** - Blur background instead of black bars
- **Crop** - Cut edges to fill frame
- **Stretch** - Distort to fill (not recommended)

### Deinterlacing

#### Inverse Telecine
For content converted from film (24fps → 30fps):
- Automatically detects 3:2 pulldown
- Recovers original progressive frames
- Default: Enabled with smooth blending

#### Deinterlace Modes
- **Auto** - Detect and deinterlace if needed
- **Yadif** - High-quality deinterlacer
- **Bwdif** - Motion-adaptive deinterlacing
- **W3fdif** - Weston 3-field deinterlacing
- **Off** - No deinterlacing

### Hardware Acceleration

When available, use GPU encoding for faster processing:
- **NVENC** - NVIDIA GPUs (RTX, GTX, Quadro)
- **QSV** - Intel Quick Sync Video
- **VAAPI** - Intel/AMD (Linux)
- **AMF** - AMD GPUs

### Advanced Options

#### Encoding Modes
- **Simple** - One-pass encoding (fast)
- **Two-Pass** - Optimal quality for target bitrate (slower)

#### Audio Options
- Codec selection (AAC, MP3, Opus, Vorbis, FLAC)
- Bitrate control
- Sample rate conversion
- Channel mapping (stereo, mono, 5.1, etc.)

#### Metadata
- Copy or strip metadata
- Add custom title, artist, album, etc.
- Embed cover art

## Usage Guide

### Basic Conversion

1. **Load Video**
   - Click "Select Video" or use already loaded video
   - Preview appears with metadata

2. **Choose Format**
   - Select output container (MP4, MKV, etc.)
   - Auto-selects compatible codec

3. **Set Quality**
   - Choose preset or custom CRF/bitrate
   - Preview estimated file size

4. **Configure Output**
   - Set output filename/location
   - Choose aspect ratio and resolution

5. **Convert**
   - Click "Convert" button
   - Monitor progress bar
   - Cancel anytime if needed

### Common Workflows

#### Modern Efficient Encoding
```
Format: MP4
Codec: H.265
Quality: CRF 26
Resolution: Source
Aspect: Source
```
Result: Smaller file, good quality

#### Universal Compatibility
```
Format: MP4
Codec: H.264
Quality: CRF 23
Resolution: 1080p
Aspect: 16:9
```
Result: Plays anywhere

#### Web/Streaming Optimized
```
Format: WebM
Codec: VP9
Quality: Two-pass 4Mbps
Resolution: 1080p
Aspect: Source
```
Result: Efficient web delivery

#### DVD/Older Content
```
Format: MP4
Codec: H.264
Quality: CRF 20
Deinterlace: Yadif
Inverse Telecine: On
```
Result: Clean progressive video

## FFmpeg Integration

### Command Building

The Convert module builds FFmpeg commands based on user selections:

```bash
# Basic conversion
ffmpeg -i input.mp4 -c:v libx264 -crf 23 -c:a aac output.mp4

# With aspect ratio handling (letterbox)
ffmpeg -i input.mp4 -vf "scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2" -c:v libx264 -crf 23 output.mp4

# With deinterlacing
ffmpeg -i input.mp4 -vf "yadif=1,bwdif" -c:v libx264 -crf 23 output.mp4

# Two-pass encoding
ffmpeg -i input.mp4 -c:v libx264 -b:v 4M -pass 1 -f null /dev/null
ffmpeg -i input.mp4 -c:v libx264 -b:v 4M -pass 2 output.mp4
```

### Filter Chain Construction

Multiple filters are chained automatically:
```bash
-vf "yadif,scale=1920:1080,unsharp=5:5:1.0:5:5:0.0"
     ↑       ↑                  ↑
     deinterlace  resize        sharpen
```

## Tips & Best Practices

### Quality vs. File Size
- Start with CRF 23, adjust if needed
- Higher CRF = smaller file, lower quality
- H.265 ~30% smaller than H.264 at same quality
- AV1 ~40% smaller but much slower to encode

### Hardware Acceleration
- NVENC is 5-10× faster but slightly larger files
- Use for quick previews or when speed matters
- CPU encoding gives better quality/size ratio

### Aspect Ratio
- Use "Source" to preserve original (default)
- Use "Auto" for smart handling when changing resolution
- Avoid "Stretch" - distorts video badly

### Deinterlacing
- Only use if source is interlaced (1080i, 720i, DVD)
- Progressive sources (1080p, web videos) don't need it
- Inverse telecine recovers film sources

## Troubleshooting

### Conversion Failed
- Check FFmpeg output for errors
- Verify source file isn't corrupted
- Try different codec/format combination

### Quality Issues
- Increase quality setting (lower CRF)
- Check source quality - can't improve bad source
- Try two-pass encoding for better results

### Slow Encoding
- Enable hardware acceleration if available
- Lower resolution or use faster preset
- H.265/AV1 are slower than H.264

### Audio Out of Sync
- Check if source has variable frame rate
- Use audio delay correction if needed
- Try re-encoding audio track

## See Also
- [Filters Module](../MODULES.md) - Apply effects before converting
- [Inspect Module](../inspect/) - View detailed source information
- [Persistent Video Context](../PERSISTENT_VIDEO_CONTEXT.md) - Using video across modules
