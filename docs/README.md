# VideoTools Documentation

VideoTools is a professional-grade video processing suite with a modern GUI, currently on v0.1.0-dev18. It specializes in creating DVD-compliant videos for authoring and distribution.

## Documentation Structure

### Core Modules (Implementation Status)

#### ✅ Fully Implemented
- [Convert](convert/) - Video transcoding and format conversion with DVD presets
- [Inspect](inspect/) - Metadata viewing and editing
- [Queue System](../QUEUE_SYSTEM_GUIDE.md) - Batch processing with job management

#### 🔄 Partially Implemented  
- [Merge](merge/) - Join multiple video clips *(planned)*
- [Trim](trim/) - Cut and split videos *(planned)*
- [Filters](filters/) - Video and audio effects *(planned)*
- [Upscale](upscale/) - Resolution enhancement *(AI + traditional now wired)*
- [Audio](audio/) - Audio track operations *(planned)*
- [Thumb](thumb/) - Thumbnail generation *(planned)*
- [Rip](rip/) - DVD/Blu-ray extraction *(planned)*

### Additional Modules (Proposed)
- [Subtitle](subtitle/) - Subtitle management *(planned)*
- [Streams](streams/) - Multi-stream handling *(planned)*
- [GIF](gif/) - Animated GIF creation *(planned)*
- [Crop](crop/) - Video cropping tools *(planned)*
- [Screenshots](screenshots/) - Frame extraction *(planned)*

## Implementation Documents
- [DVD Implementation Summary](../DVD_IMPLEMENTATION_SUMMARY.md) - Complete DVD encoding system
- [Windows Compatibility](WINDOWS_COMPATIBILITY.md) - Cross-platform support
- [Queue System Guide](../QUEUE_SYSTEM_GUIDE.md) - Batch processing system
- [Module Overview](MODULES.md) - Complete module feature list
- [Persistent Video Context](PERSISTENT_VIDEO_CONTEXT.md) - Cross-module video state management
- [Custom Video Player](VIDEO_PLAYER.md) - Embedded playback implementation

## Development Documentation
- [Integration Guide](../INTEGRATION_GUIDE.md) - System architecture and integration
- [Build and Run Guide](../BUILD_AND_RUN.md) - Build instructions and workflows
- [FFmpeg Integration](ffmpeg/) - FFmpeg command building and execution *(coming soon)*
- [Contributing](CONTRIBUTING.md) - Contribution guidelines *(coming soon)*

## User Guides
- [Installation Guide](../INSTALLATION.md) - Comprehensive installation instructions
- [DVD User Guide](../DVD_USER_GUIDE.md) - DVD encoding workflow
- [Quick Start](../README.md#quick-start) - Installation and first steps
- [Workflows](workflows/) - Common multi-module workflows *(coming soon)*
- [Keyboard Shortcuts](shortcuts.md) - Keyboard shortcuts reference *(coming soon)*

## Quick Links
- [Module Feature Matrix](MODULES.md#module-coverage-summary)
- [Latest Updates](../LATEST_UPDATES.md) - Recent development changes
- [Windows Implementation](DEV14_WINDOWS_IMPLEMENTATION.md) - dev14 Windows support
- [Modern Video Processing Strategy](HANDBRAKE_REPLACEMENT.md) - Next-generation video tools approach
- [VT_Player Integration](../VT_Player/README.md) - Frame-accurate playback system
