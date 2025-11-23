# VideoTools Modules

This document describes all the modules in VideoTools and their purpose. Each module is designed to handle specific FFmpeg operations with a user-friendly interface.

## Core Modules

### Convert
Convert is the primary module for video transcoding and format conversion. This handles:
- Codec conversion (H.264, H.265/HEVC, VP9, AV1, etc.)
- Container format changes (MP4, MKV, WebM, MOV, etc.)
- Quality presets (CRF-based and bitrate-based encoding)
- Resolution changes and aspect ratio handling (letterbox, pillarbox, crop, stretch)
- Deinterlacing and inverse telecine for legacy footage
- Hardware acceleration support (NVENC, QSV, VAAPI)
- Two-pass encoding for optimal quality/size balance

**FFmpeg Features:** Video/audio encoding, filtering, format conversion

### Merge
Merge joins multiple video clips into a single output file. Features include:
- Concatenate clips with different formats, codecs, or resolutions
- Automatic transcoding to unified output format
- Re-encoding or stream copying (when formats match)
- Maintains or normalizes audio levels across clips
- Handles mixed framerates and aspect ratios
- Optional transition effects between clips

**FFmpeg Features:** Concat demuxer/filter, stream mapping

### Trim
Trim provides timeline editing capabilities for cutting and splitting video. Features include:
- Precise frame-accurate cutting with timestamp or frame number input
- Split single video into multiple segments
- Extract specific scenes or time ranges
- Chapter-based splitting (soft split without re-encoding)
- Batch trim operations for multiple cuts in one pass
- Smart copy mode (no re-encode when possible)

**FFmpeg Features:** Seeking, segment muxer, chapter metadata

### Filters
Filters module provides video and audio processing effects:
- **Color Correction:** Brightness, contrast, saturation, hue, color balance
- **Image Enhancement:** Sharpen, blur, denoise, deband
- **Video Effects:** Grayscale, sepia, vignette, fade in/out
- **Audio Effects:** Normalize, equalize, noise reduction, tempo change
- **Correction:** Stabilization, deshake, lens distortion
- **Creative:** Speed adjustment, reverse playback, rotation/flip
- **Overlay:** Watermarks, logos, text, timecode burn-in

**FFmpeg Features:** Video/audio filter graphs, complex filters

### Upscale
Upscale increases video resolution using advanced scaling algorithms:
- **AI-based:** Waifu2x, Real-ESRGAN (via external integration)
- **Traditional:** Lanczos, Bicubic, Spline, Super-resolution
- **Target resolutions:** 720p, 1080p, 1440p, 4K, custom
- Noise reduction and artifact mitigation during upscaling
- Batch processing for multiple files
- Quality presets balancing speed vs. output quality

**FFmpeg Features:** Scale filter, super-resolution filters

### Audio
Audio module handles all audio track operations:
- Extract audio tracks to separate files (MP3, AAC, FLAC, WAV, OGG)
- Replace or add audio tracks to video
- Audio format conversion and codec changes
- Multi-track management (select, reorder, remove tracks)
- Volume normalization and adjustment
- Audio delay/sync correction
- Stereo/mono/surround channel mapping
- Sample rate and bitrate conversion

**FFmpeg Features:** Audio stream mapping, audio encoding, audio filters

### Thumb
Thumbnail and preview generation module:
- Generate single or grid thumbnails from video
- Contact sheet creation with customizable layouts
- Extract frames at specific timestamps or intervals
- Animated thumbnails (short preview clips)
- Smart scene detection for representative frames
- Batch thumbnail generation
- Custom resolution and quality settings

**FFmpeg Features:** Frame extraction, select filter, tile filter

### Inspect
Comprehensive metadata viewer and editor:
- **Technical Details:** Codec, resolution, framerate, bitrate, pixel format
- **Stream Information:** All video/audio/subtitle streams with full details
- **Container Metadata:** Title, artist, album, year, genre, cover art
- **Advanced Info:** Color space, HDR metadata, field order, GOP structure
- **Chapter Viewer:** Display and edit chapter markers
- **Subtitle Info:** List all subtitle tracks and languages
- **MediaInfo Integration:** Extended technical analysis
- Edit and update metadata fields

**FFmpeg Features:** ffprobe, metadata filters

### Rip (formerly "Remux")
Extract and convert content from optical media and disc images:
- Rip directly from DVD/Blu-ray drives to video files
- Extract from ISO, IMG, and other disc image formats
- Title and chapter selection
- Preserve or transcode during extraction
- Handle copy protection (via libdvdcss/libaacs when available)
- Subtitle and audio track selection
- Batch ripping of multiple titles
- Output to lossless or compressed formats

**FFmpeg Features:** DVD/Blu-ray input, concat, stream copying

## Additional Suggested Modules

### Subtitle
Dedicated subtitle handling module:
- Extract subtitle tracks (SRT, ASS, SSA, VTT)
- Add or replace subtitle files
- Burn (hardcode) subtitles into video
- Convert between subtitle formats
- Adjust subtitle timing/sync
- Multi-language subtitle management

**FFmpeg Features:** Subtitle filters, subtitle codec support

### Streams
Advanced stream management for complex files:
- View all streams (video/audio/subtitle/data) in detail
- Select which streams to keep or remove
- Reorder stream priority/default flags
- Map streams to different output files
- Handle multiple video angles or audio tracks
- Copy or transcode individual streams

**FFmpeg Features:** Stream mapping, stream selection

### GIF
Create animated GIFs from videos:
- Convert video segments to GIF format
- Optimize file size with palette generation
- Frame rate and resolution control
- Loop settings and duration limits
- Dithering options for better quality
- Preview before final export

**FFmpeg Features:** Palettegen, paletteuse filters

### Crop
Precise cropping and aspect ratio tools:
- Visual crop selection with preview
- Auto-detect black bars
- Aspect ratio presets
- Maintain aspect ratio or free-form crop
- Batch crop with saved presets

**FFmpeg Features:** Crop filter, cropdetect

### Screenshots
Extract still images from video:
- Single frame extraction at specific time
- Burst capture (multiple frames)
- Scene-based capture
- Format options (PNG, JPEG, BMP, TIFF)
- Resolution and quality control

**FFmpeg Features:** Frame extraction, image encoding

## Module Coverage Summary

This module set covers all major FFmpeg capabilities:
- ✅ Transcoding and format conversion
- ✅ Concatenation and merging
- ✅ Trimming and splitting
- ✅ Video/audio filtering and effects
- ✅ Scaling and upscaling
- ✅ Audio extraction and manipulation
- ✅ Thumbnail generation
- ✅ Metadata viewing and editing
- ✅ Optical media ripping
- ✅ Subtitle handling
- ✅ Stream management
- ✅ GIF creation
- ✅ Cropping
- ✅ Screenshot capture 