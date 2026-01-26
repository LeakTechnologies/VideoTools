# VideoTools - Video Processing Suite

## What is VideoTools?

VideoTools is a desktop video processing application built on FFmpeg.
It provides a graphical interface for converting, inspecting, and preparing video.

It includes tools for video conversion, batch processing, media inspection, merging, filtering, audio extraction, thumbnail generation, ripping, DVD-compliant output, and upscaling where supported.

## Project Status

**This project is under active development, and many documented features are not yet implemented.**

For a clear, up-to-date overview of what is complete, in progress, and planned, please see our **[Project Status Page](docs/PROJECT_STATUS.md)**. This document provides the most accurate reflection of the project's current state.

## Builds

- **Daily (dev):** https://git.leaktechnologies.dev/Leak_Technologies/VideoTools
- **Stable (public):** https://github.com/LeakTechnologes/VideoTools


## Capabilities

- Video conversion via FFmpeg
- Queue-based batch processing
- Media inspection and analysis
- Merge, filters, and audio extraction
- Thumbnail generation
- Compare playback
- DVD authoring, encoding, and ripping tools
- Settings for language and hardware acceleration
- Optional AI-assisted upscaling (where supported)

## Codecs and Frame Rates

**Preset output formats:**
- MP4: H.264, H.265, AV1
- MOV: H.264, H.265, ProRes
- MKV: Remux (copy), H.265, AV1
- WebM: VP9, AV1
- DVD: NTSC/PAL (MPEG-2)

**Frame rate targets:**
- Source, 23.976, 24, 25, 29.97, 30, 50, 59.94, 60
- Optional motion interpolation for frame-rate changes
- DVD presets lock to NTSC (29.97) or PAL (25) frame rates

## Quick Start

### Installation (One Command)

```bash
bash scripts/install.sh
```

The installer will build, install, and set up shell aliases.

**After installation:**
```bash
source ~/.bashrc    # (or ~/.zshrc, or ~/.config/fish/config.fish)
VideoTools
```

### Alternative: Developer Setup

If you already have the repo cloned (dev workflow):

```bash
cd /path/to/VideoTools
bash scripts/build.sh
bash scripts/run.sh
```

For detailed installation options, troubleshooting, and platform-specific notes, see **INSTALLATION.md**.
For upcoming work and priorities, see **docs/ROADMAP.md**.

## DVD Workflow (Optional)

1. **Start VideoTools** → `VideoTools`
2. **Load a video** → Drag & drop into Convert module
3. **Select format** → Choose "DVD-NTSC (MPEG-2)" or "DVD-PAL (MPEG-2)"
4. **Choose aspect** → Select 4:3 or 16:9
5. **Name output** → Enter filename (without .mpg)
6. **Queue** → Click "Add to Queue"
7. **Encode** → Click "View Queue" → "Start Queue"
8. **Export** → Use the .mpg file in DVDStyler

## Documentation

**Getting Started:**
- **PROJECT_STATUS.md** - Current implementation status
- **INSTALLATION.md** - Comprehensive installation guide (read this first!)
- **docs/README.md** - Documentation index

**For Users:**
- **BUILD_AND_RUN.md** - How to build and run VideoTools
- **DVD_USER_GUIDE.md** - Complete guide to DVD encoding

**For Developers:**
- **DVD_IMPLEMENTATION_SUMMARY.md** - Technical specifications
- **INTEGRATION_GUIDE.md** - System architecture and integration
- **QUEUE_SYSTEM_GUIDE.md** - Queue system reference
- **localization-policy.md** - Localization strategy and implementation guide

## Requirements

- **Go 1.21+** (for building)
- **FFmpeg** (for video encoding)
- **X11 or Wayland display server** (for GUI)

## System Architecture

VideoTools has a modular architecture:
- `internal/convert/` - DVD and video encoding
- `internal/queue/` - Job queue system
- `internal/ui/` - User interface components
- `internal/player/` - Media playback
- `scripts/` - Build and run automation

## Commands

### Build & Run
```bash
# One-time setup
source scripts/alias.sh    # bash
# source scripts/alias.zsh  # zsh
# source scripts/alias.fish # fish

# Run the application
VideoTools

# Force rebuild
VideoToolsRebuild

# Clean build artifacts
VideoToolsClean
```

### Legacy (Direct commands)
```bash
# Build
go build -o VideoTools .

# Run
./VideoTools

# Run with debug logging
VIDEOTOOLS_DEBUG=1 ./VideoTools

# View logs
go run . logs
```

## Troubleshooting

- See **BUILD_AND_RUN.md** for detailed troubleshooting
- Check **videotools.log** for detailed error messages
- Use `VIDEOTOOLS_DEBUG=1` for verbose logging

## Getting Help

1. Read **BUILD_AND_RUN.md** for setup issues
2. Read **docs/README.md** for module and guide links
3. Read **DVD_USER_GUIDE.md** for DVD-specific workflows
4. Check **videotools.log** for error details
5. Review documentation in project root
