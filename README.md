# VideoTools - Professional Video Processing Suite

## What is VideoTools?

VideoTools is a professional-grade video processing application with a modern GUI. It specializes in creating **DVD-compliant videos** for authoring and distribution.

## Key Features

### DVD-NTSC & DVD-PAL Output
- **Professional MPEG-2 encoding** (720×480 @ 29.97fps for NTSC, 720×576 @ 25fps for PAL)
- **AC-3 Dolby Digital audio** (192 kbps, 48 kHz)
- **DVDStyler compatible** (no re-encoding warnings)
- **PS2 compatible** (PS2-safe bitrate limits)
- **Region-free format** (works worldwide)

### Batch Processing
- Queue multiple videos
- Pause/resume jobs
- Real-time progress tracking
- Job history and persistence

### Smart Features
- Automatic framerate conversion (23.976p, 24p, 30p, 60p, VFR → 29.97fps)
- Automatic audio resampling (any rate → 48 kHz)
- Aspect ratio preservation with intelligent handling
- Comprehensive validation with helpful warnings

## Quick Start

### Installation (One Command)

```bash
bash scripts/install.sh
```

The installer will build, install, and set up everything automatically with a guided wizard!

**After installation:**
```bash
source ~/.bashrc    # (or ~/.zshrc for zsh)
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

## How to Create a Professional DVD

1. **Start VideoTools** → `VideoTools`
2. **Load a video** → Drag & drop into Convert module
3. **Select format** → Choose "DVD-NTSC (MPEG-2)" or "DVD-PAL (MPEG-2)"
4. **Choose aspect** → Select 4:3 or 16:9
5. **Name output** → Enter filename (without .mpg)
6. **Queue** → Click "Add to Queue"
7. **Encode** → Click "View Queue" → "Start Queue"
8. **Export** → Use the .mpg file in DVDStyler

Output is professional quality, ready for:
- DVDStyler authoring (no re-encoding needed)
- DVD menu creation
- Burning to disc
- PS2 playback

## Documentation

**Getting Started:**
- **INSTALLATION.md** - Comprehensive installation guide (read this first!)

**For Users:**
- **BUILD_AND_RUN.md** - How to build and run VideoTools
- **DVD_USER_GUIDE.md** - Complete guide to DVD encoding

**For Developers:**
- **DVD_IMPLEMENTATION_SUMMARY.md** - Technical specifications
- **INTEGRATION_GUIDE.md** - System architecture and integration
- **QUEUE_SYSTEM_GUIDE.md** - Queue system reference

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
source scripts/alias.sh

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

## Professional Use Cases

- Home video archival to physical media
- Professional DVD authoring workflows
- Multi-region video distribution
- Content preservation on optical media
- PS2 compatible video creation

## Professional Quality Specifications

### DVD-NTSC
- **Resolution:** 720 × 480 pixels
- **Framerate:** 29.97 fps (NTSC standard)
- **Video:** MPEG-2 codec, 6000 kbps
- **Audio:** AC-3 stereo, 192 kbps, 48 kHz
- **Regions:** USA, Canada, Japan, Australia

### DVD-PAL
- **Resolution:** 720 × 576 pixels
- **Framerate:** 25.00 fps (PAL standard)
- **Video:** MPEG-2 codec, 8000 kbps
- **Audio:** AC-3 stereo, 192 kbps, 48 kHz
- **Regions:** Europe, Africa, Asia, Australia

## Getting Help

1. Read **BUILD_AND_RUN.md** for setup issues
2. Read **DVD_USER_GUIDE.md** for how-to questions
3. Check **videotools.log** for error details
4. Review documentation in project root
