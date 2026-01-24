# VideoTools Documentation

VideoTools is a professional-grade video processing suite with a modern GUI. It specializes in creating DVD-compliant videos for authoring and distribution.

**For a high-level overview of what is currently implemented, in progress, or planned, please see the [Project Status Page](./PROJECT_STATUS.md).**

## Documentation Structure

### Core Modules (Implementation Status)

#### ✅ Implemented
- [Convert](convert/) - Video transcoding and format conversion with DVD presets.
- [Inspect](inspect/) - Basic metadata viewing.
- [Rip](rip/) - Extraction from `VIDEO_TS` folders and `.iso` images.
- [Queue System](../QUEUE_SYSTEM_GUIDE.md) - Batch processing with job management.

#### 🟡 Partially Implemented / Buggy
- **Player** - Core video playback is functional but has critical bugs blocking development.
- **Upscale** - AI-based upscaling (Real-ESRGAN) is integrated.

#### 🔄 Planned
- **Merge** - [PLANNED] Join multiple video clips.
- **Trim** - [PLANNED] Cut and split videos.
- **Filters** - [PLANNED] Video and audio effects.
- **Audio** - [PLANNED] Audio track operations.
- **Thumb** - [PLANNED] Thumbnail generation.

### Additional Modules (All Planned)
- **Subtitle** - [PLANNED] Subtitle management.
- **Streams** - [PLANNED] Multi-stream handling.
- **GIF** - [PLANNED] Animated GIF creation.
- **Crop** - [PLANNED] Video cropping tools.
- **Screenshots** - [PLANNED] Frame extraction.

## Implementation Documents
- [DVD Implementation Summary](./DVD_IMPLEMENTATION_SUMMARY.md) - Technical details of the DVD encoding system.
- [Windows Compatibility](WINDOWS_COMPATIBILITY.md) - Notes on cross-platform support.
- [Queue System Guide](../QUEUE_SYSTEM_GUIDE.md) - Deep dive into the batch processing system.
- [Module Overview](MODULES.md) - The complete feature list for all modules (implemented and planned).
- [Persistent Video Context](PERSISTENT_VIDEO_CONTEXT.md) - Design for cross-module video state management.
- [Custom Video Player](VIDEO_PLAYER.md) - Documentation for the embedded playback implementation.

## Development Documentation
- [Integration Guide](../INTEGRATION_GUIDE.md) - System architecture and integration plans.
- [Build and Run Guide](../BUILD_AND_RUN.md) - Instructions for setting up a development environment.
- **FFmpeg Integration** - [PLANNED] Documentation on FFmpeg command building.
- **Contributing** - [PLANNED] Contribution guidelines.

## User Guides
- [Installation Guide](../INSTALLATION.md) - Comprehensive installation instructions.
- [DVD User Guide](../DVD_USER_GUIDE.md) - A step-by-step guide to the DVD encoding workflow.
- [Quick Start](../README.md#quick-start) - The fastest way to get up and running.
- **Workflows** - [PLANNED] Guides for common multi-module tasks.
- **Keyboard Shortcuts** - [PLANNED] A reference for all keyboard shortcuts.

## Quick Links
- [Module Feature Matrix](MODULES.md#module-coverage-summary)
- [Latest Updates](../LATEST_UPDATES.md) - Recent development changes.
- [Windows Implementation Notes](DEV14_WINDOWS_IMPLEMENTATION.md)
- **VT_Player Integration** - [PLANNED] Frame-accurate playback system.
