# Inspect Module

The Inspect module provides comprehensive metadata viewing and technical analysis of video files.

## Overview

Inspect is a read-only module designed to display detailed information about video files that doesn't fit in the compact metadata panel shown in other modules. It's useful for technical analysis, troubleshooting, and understanding video file characteristics.

## Features

### Technical Details
- **Video Codec** - H.264, H.265, VP9, etc.
- **Container Format** - MP4, MKV, AVI, etc.
- **Resolution** - Width × Height in pixels
- **Frame Rate** - Exact fps (23.976, 29.97, 30, 60, etc.)
- **Aspect Ratio** - Display aspect ratio (DAR) and pixel aspect ratio (PAR)
- **Bitrate** - Overall, video, and audio bitrates
- **Duration** - Precise timestamp
- **File Size** - Human-readable format
- **Pixel Format** - yuv420p, yuv444p, rgb24, etc.
- **Color Space** - BT.709, BT.601, BT.2020, etc.
- **Color Range** - Limited (TV) or Full (PC)
- **Bit Depth** - 8-bit, 10-bit, 12-bit

### Stream Information

#### Video Streams
For each video stream:
- Stream index and type
- Codec name and profile
- Resolution and aspect ratio
- Frame rate and time base
- Bitrate
- GOP structure (keyframe interval)
- Encoding library/settings

#### Audio Streams
For each audio stream:
- Stream index and type
- Codec name
- Sample rate (44.1kHz, 48kHz, etc.)
- Bit depth (16-bit, 24-bit, etc.)
- Channels (stereo, 5.1, 7.1, etc.)
- Bitrate
- Language tag

#### Subtitle Streams
For each subtitle stream:
- Stream index and type
- Subtitle format (SRT, ASS, PGS, etc.)
- Language tag
- Default/forced flags

### Container Metadata

#### Common Tags
- **Title** - Media title
- **Artist/Author** - Creator
- **Album** - Collection name
- **Year** - Release year
- **Genre** - Content category
- **Comment** - Description
- **Track Number** - Position in album
- **Cover Art** - Embedded image

#### Technical Metadata
- **Creation Time** - When file was created
- **Encoder** - Software used to create file
- **Handler Name** - Video/audio handler
- **Timecode** - Start timecode for professional footage

### Chapter Information
- Chapter count
- Chapter titles
- Start/end timestamps for each chapter
- Chapter thumbnail (if available)

### Advanced Analysis

#### HDR Metadata
For HDR content:
- **Color Primaries** - BT.2020, DCI-P3
- **Transfer Characteristics** - PQ (ST.2084), HLG
- **Mastering Display** - Peak luminance, color gamut
- **Content Light Level** - MaxCLL, MaxFALL

#### Interlacing Detection
- Field order (progressive, top-field-first, bottom-field-first)
- Telecine flags
- Repeat field flags

#### Variable Frame Rate
- Detection of VFR content
- Frame rate range (min/max)
- Frame duplication patterns

### Cover Art Viewer
- Display embedded cover art
- Show resolution and format
- Extract to separate file option

### MediaInfo Integration
When available, show extended MediaInfo output:
- Writing library details
- Encoding settings reconstruction
- Format-specific technical data

## Usage Guide

### Basic Inspection

1. **Load Video**
   - Select video file or use already loaded video
   - Inspection loads automatically

2. **Review Information**
   - Browse through categorized sections
   - Copy technical details to clipboard
   - Export full report

### Viewing Streams

Navigate to "Streams" tab to see all tracks:
- Identify default streams
- Check language tags
- Verify codec compatibility

### Checking Metadata

Open "Metadata" tab to view/copy tags:
- Useful for organizing media libraries
- Verify embedded information
- Check for privacy concerns (GPS, camera info)

### Chapter Navigation

If video has chapters:
- View chapter list with timestamps
- Preview chapter thumbnails
- Use for planning trim operations

## Export Options

### Text Report
Export all information as plain text file:
```
VideoTools Inspection Report
File: example.mp4
Date: 2025-11-23

== GENERAL ==
Format: QuickTime / MOV
Duration: 00:10:23.456
File Size: 512.3 MB
...
```

### JSON Export
Structured data for programmatic use:
```json
{
  "format": "mov,mp4,m4a,3gp,3g2,mj2",
  "duration": 623.456,
  "bitrate": 6892174,
  "streams": [...]
}
```

### Clipboard Copy
Quick copy of specific details:
- Right-click any field → Copy
- Copy entire section
- Copy full ffprobe output

## Integration with Other Modules

### Pre-Convert Analysis
Before converting, check:
- Source codec and quality
- HDR metadata (may need special handling)
- Audio tracks (which to keep?)
- Subtitle availability

### Post-Convert Verification
After conversion, compare:
- File size reduction
- Bitrate changes
- Metadata preservation
- Stream count/types

### Troubleshooting Aid
When something goes wrong:
- Verify source file integrity
- Check for unusual formats
- Identify problematic streams
- Get exact technical specs for support

## FFmpeg Integration

Inspect uses `ffprobe` for metadata extraction:

```bash
# Basic probe
ffprobe -v quiet -print_format json -show_format -show_streams input.mp4

# Include chapters
ffprobe -v quiet -print_format json -show_chapters input.mp4

# Frame-level analysis (for advanced detection)
ffprobe -v quiet -select_streams v:0 -show_frames input.mp4
```

## Tips & Best Practices

### Understanding Codecs
- **H.264 Baseline** - Basic compatibility (phones, old devices)
- **H.264 Main** - Standard use (most common)
- **H.264 High** - Better quality (Blu-ray, streaming)
- **H.265 Main** - Consumer HDR content
- **H.265 Main10** - 10-bit color depth

### Bitrate Interpretation
| Quality | 1080p Bitrate | 4K Bitrate |
|---------|---------------|------------|
| Low | 2-4 Mbps | 8-12 Mbps |
| Medium | 5-8 Mbps | 15-25 Mbps |
| High | 10-15 Mbps | 30-50 Mbps |
| Lossless | 50+ Mbps | 100+ Mbps |

### Frame Rate Notes
- **23.976** - Film transferred to video (NTSC)
- **24** - Film, cinema
- **25** - PAL standard
- **29.97** - NTSC standard
- **30** - Modern digital
- **50/60** - High frame rate, sports
- **120+** - Slow motion source

### Color Space
- **BT.601** - SD content (DVD, old TV)
- **BT.709** - HD content (Blu-ray, modern)
- **BT.2020** - UHD/HDR content

## See Also
- [Convert Module](../convert/) - Use inspection data to inform conversion settings
- [Filters Module](../filters/) - Understand color space before applying filters
- [Streams Module](../streams/) - Manage individual streams found in inspection
