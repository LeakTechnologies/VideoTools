# VideoTools Modules

This document describes all the modules in VideoTools and their purpose. Each module is designed to handle specific FFmpeg operations with a user-friendly interface.

## Core Modules

### Player ✅ CRITICAL FOUNDATION
Embedded playback used across modules for preview and inspection.

**Current Status:** Implemented with known performance issues (see `PLAYER_PERFORMANCE_ISSUES.md`).

### Convert ✅ IMPLEMENTED
Convert is the primary module for video transcoding and format conversion. This handles:
- ✅ Codec conversion (H.264, H.265/HEVC, VP9, AV1, etc.)
- ✅ Container format changes (MP4, MKV, WebM, MOV, etc.)
- ✅ Quality presets (CRF-based and bitrate-based encoding)
- ✅ Resolution changes and aspect ratio handling (letterbox, pillarbox, crop, stretch)
- ✅ Deinterlacing and inverse telecine for legacy footage
- ✅ Hardware acceleration support (NVENC, QSV, VAAPI)
- ✅ DVD-NTSC/PAL encoding with professional compliance
- ✅ Auto-resolution setting for DVD formats
- ⏳ Two-pass encoding for optimal quality/size balance *(planned)*

**FFmpeg Features:** Video/audio encoding, filtering, format conversion

**Current Status:** Fully implemented with DVD encoding support, auto-resolution, and professional validation system.

### Merge ✅ IMPLEMENTED
Merge joins multiple video clips into a single output file. Features include:
- ✅ Concatenate multiple clips into a single output
- ✅ Format presets for copy or transcode output
- ✅ Output folder + filename control
- ✅ Optional chapter naming
- ✅ DVD region/aspect settings for DVD output
- ✅ Frame rate selection with optional motion interpolation
- ✅ Queue-based execution with progress/logs

**FFmpeg Features:** Concat demuxer/filter, stream mapping

**Current Status:** Implemented with queue support.

### Trim 🔄 PLANNED (Lossless-Cut Inspired)
Trim provides frame-accurate cutting with lossless-first philosophy (inspired by Lossless-Cut). Features include:

#### Core Lossless-Cut Features
- ⏳ **Lossless-First Approach** - Stream copy when possible, smart re-encode fallback
- ⏳ **Keyframe-Snapping Timeline** - Visual keyframe markers with smart snapping
- ⏳ **Frame-Accurate Navigation** - Reuse VT_Player's keyframe detection system
- ⏳ **Smart Export System** - Automatic method selection (lossless/re-encode/hybrid)
- ⏳ **Multi-Segment Trimming** - Multiple cuts from single source with auto-chapters

#### UI/UX Features
- ⏳ **Timeline Interface** - Zoomable timeline with keyframe visibility (reuse VT_Player)
- ⏳ **Visual Markers** - Blue (in), Red (out), Green (current position)
- ⏳ **Keyboard Shortcuts** - I (in), O (out), X (clear), ←→ (frames), ↑↓ (keyframes)
- ⏳ **Preview System** - Instant segment preview with loop option
- ⏳ **Quality Indicators** - Real-time feedback on export method and quality

#### Technical Implementation
- ⏳ **Stream Analysis** - Detect lossless trim possibility automatically
- ⏳ **Smart Export Logic** - Choose optimal method based on content and markers
- ⏳ **Format Conversion** - Handle format changes during trim operations
- ⏳ **Quality Validation** - Verify output integrity and quality preservation
- ⏳ **Error Recovery** - Smart suggestions when export fails

**FFmpeg Features:** Seeking, segment muxer, stream copying, smart re-encoding
**Integration:** Reuses VT_Player's keyframe detector and timeline widget
**Current Status:** Planned; depends on player stability work.
**Inspiration:** Lossless-Cut's lossless-first philosophy with modern enhancements

### Filters ✅ IMPLEMENTED
Filters module provides video and audio processing effects:
- ✅ **Color Correction:** Brightness, contrast, saturation, hue, color balance
- ✅ **Image Enhancement:** Sharpen, blur, denoise
- ✅ **Video Effects:** Stylized presets and analog emulation
- ✅ **Correction:** Crop, rotate, flip, and aspect controls
- ✅ **Creative:** Speed and frame-rate adjustments

**FFmpeg Features:** Video/audio filter graphs, complex filters

**Current Status:** Implemented with FFmpeg filter chains.

### Upscale 🔄 PARTIAL
Upscale increases video resolution using advanced scaling algorithms:
- ✅ **AI-based:** Real-ESRGAN (ncnn backend) with presets and model selection
- ✅ **Traditional:** Lanczos, Bicubic, Spline, Bilinear
- ✅ **Target resolutions:** Match Source, 2x/4x relative, 720p, 1080p, 1440p, 4K, 8K
- ✅ Frame extraction → AI upscale → reassemble pipeline
- ✅ Filters and frame-rate conversion can be applied before AI upscaling
- ⏳ Noise reduction and artifact mitigation beyond Real-ESRGAN
- ⏳ Batch processing for multiple files (via queue)
- ✅ Quality presets balancing speed vs. output quality (AI presets)

**FFmpeg Features:** Scale filter, minterpolate, fps

**Current Status:** AI integration wired (ncnn). Python backend options are documented but not yet executed.

### Audio ✅ IMPLEMENTED
Audio module handles audio extraction from video files:
- ✅ Extract audio tracks to separate files (MP3, AAC, FLAC, WAV, OGG)
- ✅ Track selection and batch extraction
- ✅ Quality presets and bitrate selection
- ✅ Loudness normalization (LUFS/true peak targets)

**FFmpeg Features:** Audio stream mapping, audio encoding, audio filters

**Current Status:** Implemented with extraction and batch tools.

### Thumb ✅ IMPLEMENTED
Thumbnail and preview generation module:
- ✅ Generate single or grid thumbnails from video
- ✅ Contact sheet creation with customizable layouts
- ✅ Extract frames at specific timestamps or intervals
- ✅ Batch thumbnail generation
- ✅ Custom resolution and quality settings

**FFmpeg Features:** Frame extraction, select filter, tile filter

**Current Status:** Implemented with grid and contact sheet output.

### Inspect ✅ IMPLEMENTED
Comprehensive metadata viewer and editor:
- ✅ **Technical Details:** Codec, resolution, framerate, bitrate, pixel format
- ✅ **Stream Information:** All video/audio/subtitle streams with full details
- ✅ **Container Metadata:** Title, artist, album, year, genre, cover art
- ✅ **Interlacing Analysis:** Quick detection and recommendations
- ⏳ **Advanced Info:** Color space, HDR metadata, field order, GOP structure
- ⏳ **Chapter Viewer:** Display and edit chapter markers
- ⏳ **Subtitle Info:** List all subtitle tracks and languages
- ⏳ **MediaInfo Integration:** Extended technical analysis
- ⏳ Edit and update metadata fields

**FFmpeg Features:** ffprobe, metadata filters

**Current Status:** Implemented with advanced features planned.

### Compare ✅ IMPLEMENTED
Side-by-side comparison of two videos with synchronized playback controls.

**FFmpeg Features:** FFmpeg-backed decode pipeline

**Current Status:** Implemented.

### Rip ✅ IMPLEMENTED
Extract and convert content from optical media and disc images:
- ✅ Rip from VIDEO_TS folders
- ✅ Extract from ISO images (requires `xorriso` or `bsdtar`)
- ✅ Default lossless DVD → MKV (stream copy)
- ✅ Optional H.264 MKV/MP4 outputs
- ✅ Queue-based execution with logs and progress

**FFmpeg Features:** concat demuxer, stream copy, H.264 encoding

**Current Status:** Available in dev20+. Physical disc and multi-title selection are still planned.

### Author ✅ IMPLEMENTED
DVD authoring and menu generation:
- ✅ DVD output type and region selection (NTSC/PAL/AUTO)
- ✅ Menu templates with optional background image support
- ✅ Chapter handling and menu layout options
- ✅ Queue-based execution with logs and progress

**FFmpeg Features:** DVD-Video encoding, muxing, and asset generation

**Current Status:** Implemented for DVD output. Blu-ray is planned.

### Blu-ray 🔄 PLANNED
Professional Blu-ray Disc authoring and encoding system:
- ⏳ **Blu-ray Standards Support:** 1080p, 4K UHD, HDR content
- ⏳ **Multi-Region Encoding:** Region A/B/C with proper specifications
- ⏳ **Advanced Video Codecs:** H.264/AVC, H.265/HEVC with professional profiles
- ⏳ **Professional Audio:** LPCM, Dolby Digital Plus, DTS-HD Master Audio
- ⏳ **HDR Support:** HDR10, Dolby Vision metadata handling
- ⏳ **Authoring Compatibility:** Adobe Encore, Sony Scenarist integration
- ⏳ **Hardware Compatibility:** PS3/4/5, Xbox, standalone players
- ⏳ **Validation System:** Blu-ray specification compliance checking

**FFmpeg Features:** H.264/HEVC encoding, transport stream muxing, HDR metadata

**Current Status:** Comprehensive planning complete; implementation is planned. See TODO.md for detailed specifications.

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

**Current Status:** Core playback works but has known A/V sync and seeking issues. See `PLAYER_PERFORMANCE_ISSUES.md` for details.

### ✅ Currently Implemented
- ✅ **Video conversion and DVD encoding**
- ✅ **Merge, filters, and audio extraction**
- ✅ **Upscale (traditional + optional AI)**
- ✅ **Metadata inspection and compare**
- ✅ **Rip and DVD authoring**
- ✅ **Thumbnail generation**
- ✅ **Queue system and history**

### 🔄 Planned / In Progress
- 🔄 **Player stabilization** (single-process A/V sync, frame-accurate seeking)
- 🔄 **Trim module**
- 🔄 **Subtitles**
- 🔄 **Blu-ray authoring**
- 🔄 **Enhancement pipeline**
