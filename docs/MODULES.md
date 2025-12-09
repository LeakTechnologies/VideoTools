# VideoTools Modules

This document describes all the modules in VideoTools and their purpose. Each module is designed to handle specific FFmpeg operations with a user-friendly interface.

## Core Modules

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

### Merge 🔄 PLANNED
Merge joins multiple video clips into a single output file. Features include:
- ⏳ Concatenate clips with different formats, codecs, or resolutions
- ⏳ Automatic transcoding to unified output format
- ⏳ Re-encoding or stream copying (when formats match)
- ⏳ Maintains or normalizes audio levels across clips
- ⏳ Handles mixed framerates and aspect ratios
- ⏳ Optional transition effects between clips

**FFmpeg Features:** Concat demuxer/filter, stream mapping

**Current Status:** Planned for dev15, UI design phase.

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
**Current Status:** Planning complete, implementation ready for dev15
**Inspiration:** Lossless-Cut's lossless-first philosophy with modern enhancements

### Filters 🔄 PLANNED
Filters module provides video and audio processing effects:
- ⏳ **Color Correction:** Brightness, contrast, saturation, hue, color balance
- ⏳ **Image Enhancement:** Sharpen, blur, denoise, deband
- ⏳ **Video Effects:** Grayscale, sepia, vignette, fade in/out
- ⏳ **Audio Effects:** Normalize, equalize, noise reduction, tempo change
- ⏳ **Correction:** Stabilization, deshake, lens distortion
- ⏳ **Creative:** Speed adjustment, reverse playback, rotation/flip
- ⏳ **Overlay:** Watermarks, logos, text, timecode burn-in

**FFmpeg Features:** Video/audio filter graphs, complex filters

**Current Status:** Planned for dev15, basic filter system design.

### Upscale 🔄 PLANNED
Upscale increases video resolution using advanced scaling algorithms:
- ⏳ **AI-based:** Waifu2x, Real-ESRGAN (via external integration)
- ⏳ **Traditional:** Lanczos, Bicubic, Spline, Super-resolution
- ⏳ **Target resolutions:** 720p, 1080p, 1440p, 4K, custom
- ⏳ Noise reduction and artifact mitigation during upscaling
- ⏳ Batch processing for multiple files
- ⏳ Quality presets balancing speed vs. output quality

**FFmpeg Features:** Scale filter, super-resolution filters

**Current Status:** Planned for dev16, AI integration research phase.

### Audio 🔄 PLANNED
Audio module handles all audio track operations:
- ⏳ Extract audio tracks to separate files (MP3, AAC, FLAC, WAV, OGG)
- ⏳ Replace or add audio tracks to video
- ⏳ Audio format conversion and codec changes
- ⏳ Multi-track management (select, reorder, remove tracks)
- ⏳ Volume normalization and adjustment
- ⏳ Audio delay/sync correction
- ⏳ Stereo/mono/surround channel mapping
- ⏳ Sample rate and bitrate conversion

**FFmpeg Features:** Audio stream mapping, audio encoding, audio filters

**Current Status:** Planned for dev15, basic audio operations design.

### Thumb 🔄 PLANNED
Thumbnail and preview generation module:
- ⏳ Generate single or grid thumbnails from video
- ⏳ Contact sheet creation with customizable layouts
- ⏳ Extract frames at specific timestamps or intervals
- ⏳ Animated thumbnails (short preview clips)
- ⏳ Smart scene detection for representative frames
- ⏳ Batch thumbnail generation
- ⏳ Custom resolution and quality settings

**FFmpeg Features:** Frame extraction, select filter, tile filter

**Current Status:** Planned for dev15, thumbnail system design.

### Inspect ✅ PARTIALLY IMPLEMENTED
Comprehensive metadata viewer and editor:
- ✅ **Technical Details:** Codec, resolution, framerate, bitrate, pixel format
- ✅ **Stream Information:** All video/audio/subtitle streams with full details
- ✅ **Container Metadata:** Title, artist, album, year, genre, cover art
- ⏳ **Advanced Info:** Color space, HDR metadata, field order, GOP structure
- ⏳ **Chapter Viewer:** Display and edit chapter markers
- ⏳ **Subtitle Info:** List all subtitle tracks and languages
- ⏳ **MediaInfo Integration:** Extended technical analysis
- ⏳ Edit and update metadata fields

**FFmpeg Features:** ffprobe, metadata filters

**Current Status:** Basic metadata viewing implemented, advanced features planned.

### Rip 🔄 PLANNED
Extract and convert content from optical media and disc images:
- ⏳ Rip directly from DVD/Blu-ray drives to video files
- ⏳ Extract from ISO, IMG, and other disc image formats
- ⏳ Title and chapter selection
- ⏳ Preserve or transcode during extraction
- ⏳ Handle copy protection (via libdvdcss/libaacs when available)
- ⏳ Subtitle and audio track selection
- ⏳ Batch ripping of multiple titles
- ⏳ Output to lossless or compressed formats

**FFmpeg Features:** DVD/Blu-ray input, concat, stream copying

**Current Status:** Planned for dev16, requires legal research and library integration.

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

**Current Status:** Comprehensive planning complete, implementation planned for dev15+. See TODO.md for detailed specifications.

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

### ✅ Currently Implemented
- ✅ **Transcoding and format conversion** - Full DVD encoding system
- ✅ **Metadata viewing and editing** - Basic implementation
- ✅ **Queue system** - Batch processing with job management
- ✅ **Cross-platform support** - Linux, macOS, Windows (dev14)

### 🔄 In Development/Planned
- 🔄 **Concatenation and merging** - Planned for dev15
- 🔄 **Trimming and splitting** - Planned for dev15
- 🔄 **Video/audio filtering and effects** - Planned for dev15
- 🔄 **Scaling and upscaling** - Planned for dev16
- 🔄 **Audio extraction and manipulation** - Planned for dev15
- 🔄 **Thumbnail generation** - Planned for dev15
- 🔄 **Optical media ripping** - Planned for dev16
- 🔄 **Blu-ray authoring** - Comprehensive planning complete
- 🔄 **Subtitle handling** - Planned for dev15
- 🔄 **Stream management** - Planned for dev15
- 🔄 **GIF creation** - Planned for dev16
- 🔄 **Cropping** - Planned for dev15
- 🔄 **Screenshot capture** - Planned for dev16

### 📊 Implementation Progress
- **Core Modules:** 1/8 fully implemented (Convert)
- **Additional Modules:** 0/7 implemented
- **Overall Progress:** ~12% complete
- **Next Major Release:** dev15 (Merge, Trim, Filters modules)
- **Future Focus:** Blu-ray professional authoring system 