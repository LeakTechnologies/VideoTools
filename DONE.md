# VideoTools - Completed Features

This file tracks completed features, fixes, and milestones.

## Version 0.1.0-dev12 (2025-12-02)

### Features
- ✅ **Automatic hardware encoder detection and selection**
  - Prioritizes NVIDIA NVENC > Intel QSV > VA-API > OpenH264
  - Falls back to software encoders (libx264/libx265) if no hardware acceleration available
  - Automatically uses best available encoder without user configuration
  - Significant performance improvement on systems with GPU encoding support

- ✅ **iPhone/mobile device compatibility settings**
  - H.264 profile selection (baseline, main, high)
  - H.264 level selection (3.0, 3.1, 4.0, 4.1, 5.0, 5.1)
  - Defaults to main profile, level 4.0 for maximum compatibility
  - Ensures videos play on iPhone 4 and newer devices

- ✅ **Advanced deinterlacing with dual methods**
  - Added bwdif (Bob Weaver) deinterlacing - higher quality than yadif
  - Kept yadif for faster processing when speed is priority
  - Auto-detect interlaced content based on field_order metadata
  - Deinterlace modes: Auto (detect and apply), Force, Off
  - Defaults to bwdif for best quality

- ✅ **Audio normalization for compatibility**
  - Force stereo (2 channels) output
  - Force 48kHz sample rate
  - Ensures consistent playback across all devices
  - Optional toggle for maximum compatibility mode

- ✅ **10-bit encoding for better compression**
  - Changed default pixel format from yuv420p to yuv420p10le
  - Provides 10-20% file size reduction at same visual quality
  - Better handling of color gradients and banding
  - Automatic for all H.264/H.265 conversions

- ✅ **Browser desync fix**
  - Added `-fflags +genpts` to regenerate timestamps
  - Added `-r` flag to enforce constant frame rate (CFR)
  - Fixes "desync after multiple plays" issue in Chromium browsers (Chrome, Edge, Vivaldi)
  - Eliminates gradual audio drift when scrubbing/seeking

- ✅ **Extended resolution support**
  - Added 8K (4320p) resolution option
  - Supports: 720p, 1080p, 1440p, 4K (2160p), 8K (4320p)
  - Prepared for future VR and ultra-high-resolution content

- ✅ **Black bar cropping infrastructure**
  - Added AutoCrop configuration option
  - Cropdetect filter support for future auto-detection
  - Foundation for 15-30% file size reduction in dev13

### Technical Improvements
- ✅ All new settings propagate to both direct convert and queue processing
- ✅ Backward compatible with legacy InverseTelecine setting
- ✅ Comprehensive logging for all encoding decisions
- ✅ Settings persist across video loads

### Bug Fixes
- ✅ Fixed VFR (Variable Frame Rate) handling that caused desync
- ✅ Prevented timestamp drift in long videos
- ✅ Improved browser playback compatibility

## Version 0.1.0-dev11 (2025-11-30)

### Features
- ✅ Added persistent conversion stats bar visible on all screens
  - Real-time progress updates for running jobs
  - Displays pending/completed/failed job counts
  - Clickable to open queue view
  - Shows job title and progress percentage
- ✅ Added multi-video navigation with Prev/Next buttons
  - Load multiple videos for batch queue setup
  - Switch between loaded videos to review settings before queuing
  - Shows "Video X of Y" counter
- ✅ Added installation script with animated loading spinner
  - Braille character animations
  - Shows current task during build and install
  - Interactive path selection (system-wide or user-local)
- ✅ Added error dialogs with "Copy Error" button
  - One-click error message copying for debugging
  - Applied to all major error scenarios
  - Better user experience when reporting issues

### Improvements
- ✅ Align direct convert and queue behavior
  - Show active direct convert inline in queue with live progress
  - Preserve queue scroll position during updates
  - Back button from queue returns to originating module
  - Queue badge includes active direct conversions
  - Allow adding to queue while a convert is running
- ✅ DVD-compliant outputs
  - Enforce MPEG-2 video + AC-3 audio, yuv420p
  - Apply NTSC/PAL targets with correct fps/resolution
  - Disable cover art for DVD targets to avoid mux errors
  - Unified settings for direct and queued jobs
- ✅ Updated queue tile to show active/total jobs instead of completed/total
  - Shows pending + running jobs out of total
  - More intuitive status at a glance
- ✅ Fixed critical deadlock in queue callback system
  - Callbacks now run in goroutines to prevent blocking
  - Prevents app freezing when adding jobs to queue
- ✅ Improved batch file handling with detailed error reporting
  - Shows which specific files failed to analyze
  - Continues processing valid files when some fail
  - Clear summary messages
- ✅ Fixed queue status display
  - Always shows progress percentage (even at 0%)
  - Clearer indication when job is running vs. pending
- ✅ Fixed queue deserialization for formatOption struct
  - Handles JSON map conversion properly
  - Prevents panic when reloading saved queue on startup

### Bug Fixes
- ✅ Fixed crash when dragging multiple files
  - Better error handling in batch processing
  - Graceful degradation for problematic files
- ✅ Fixed deadlock when queue callbacks tried to read stats
- ✅ Fixed formatOption deserialization from saved queue

## Version 0.1.0-dev7 (2025-11-23)

### Features
- ✅ Changed default aspect ratio from 16:9 to Source across all instances
  - Updated initial state default
  - Updated empty fallback default
  - Updated reset button behavior
  - Updated clear video behavior
  - Updated hint label text

### Documentation
- ✅ Created comprehensive MODULES.md with all planned modules
- ✅ Created PERSISTENT_VIDEO_CONTEXT.md design document
- ✅ Created VIDEO_PLAYER.md documenting custom player implementation
- ✅ Reorganized docs into module-specific folders
- ✅ Created detailed Convert module documentation
- ✅ Created detailed Inspect module documentation
- ✅ Created detailed Rip module documentation
- ✅ Created docs/README.md navigation hub
- ✅ Created TODO.md and DONE.md tracking files

## Version 0.1.0-dev6 and Earlier

### Core Application
- ✅ Fyne-based GUI framework
- ✅ Multi-module architecture with tile-based main menu
- ✅ Application icon and branding
- ✅ Debug logging system (VIDEOTOOLS_DEBUG environment variable)
- ✅ Cross-module state management
- ✅ Window initialization and sizing

### Convert Module (Partial Implementation)
- ✅ Basic video conversion functionality
- ✅ Format selection (MP4, MKV, WebM, MOV, AVI)
- ✅ Codec selection (H.264, H.265, VP9)
- ✅ Quality presets (CRF-based encoding)
- ✅ Output aspect ratio selection
  - Source, 16:9, 4:3, 1:1, 9:16, 21:9
- ✅ Aspect ratio handling methods
  - Auto, Letterbox, Pillarbox, Blur Fill
- ✅ Deinterlacing options
  - Inverse telecine with default smoothing
- ✅ Mode toggle (Simple/Advanced)
- ✅ Output filename customization
- ✅ Default output naming ("-convert" suffix)
- ✅ Status indicator during conversion
- ✅ Cancelable conversion process
- ✅ FFmpeg command construction
- ✅ Process management and execution

### Video Loading & Metadata
- ✅ File selection dialog
- ✅ FFprobe integration for metadata parsing
- ✅ Video source structure with comprehensive metadata
  - Path, format, resolution, duration
  - Video/audio codecs
  - Bitrate, framerate, pixel format
  - Field order detection
- ✅ Preview frame generation (24 frames)
- ✅ Temporary directory management for previews

### Media Player
- ✅ Embedded video playback using FFmpeg
- ✅ Audio playback with SDL2
- ✅ Frame-accurate rendering
- ✅ Playback controls (play/pause)
- ✅ Volume control
- ✅ Seek functionality with progress bar
- ✅ Player window sizing based on video aspect ratio
- ✅ Frame pump system for smooth playback
- ✅ Audio/video synchronization
- ✅ Stable seeking and embedded video rendering

### Metadata Display
- ✅ Metadata panel showing key video information
- ✅ Resolution display
- ✅ Duration formatting
- ✅ Codec information
- ✅ Aspect ratio display
- ✅ Field order indication

### Inspect Module (Basic)
- ✅ Video metadata viewing
- ✅ Technical details display
- ✅ Comprehensive information in Convert module metadata panel
- ✅ Cover art preview capability

### UI Components
- ✅ Main menu with 8 module tiles
  - Convert, Merge, Trim, Filters, Upscale, Audio, Thumb, Inspect
- ✅ Module color coding for visual identification
- ✅ Clear video control in metadata panel
- ✅ Reset button for Convert settings
- ✅ Status label for operation feedback
- ✅ Progress indication during operations

### Git & Version Control
- ✅ Git repository initialization
- ✅ .gitignore configuration
- ✅ Version tagging system (v0.1.0-dev1 through dev7)
- ✅ Commit message formatting
- ✅ Binary exclusion from repository
- ✅ Build cache exclusion

### Build System
- ✅ Go modules setup
- ✅ Fyne dependencies integration
- ✅ FFmpeg/FFprobe external tool integration
- ✅ SDL2 integration for audio
- ✅ OpenGL bindings (go-gl) for video rendering
- ✅ Cross-platform file path handling

### Asset Management
- ✅ Application icon (VT_Icon.svg)
- ✅ Icon export to PNG format
- ✅ Icon embedding in application

### Logging & Debugging
- ✅ Category-based logging (SYS, UI, MODULE, etc.)
- ✅ Timestamp formatting
- ✅ Debug output toggle via environment variable
- ✅ Comprehensive debug messages throughout application
- ✅ Log file output (videotools.log)

### Error Handling
- ✅ FFmpeg execution error capture
- ✅ File selection cancellation handling
- ✅ Video parsing error messages
- ✅ Process cancellation cleanup

### Utility Functions
- ✅ Duration formatting (seconds to HH:MM:SS)
- ✅ Aspect ratio parsing and calculation
- ✅ File path manipulation
- ✅ Temporary directory creation and cleanup

## Technical Achievements

### Architecture
- ✅ Clean separation between UI and business logic
- ✅ Shared state management across modules
- ✅ Modular design allowing easy addition of new modules
- ✅ Event-driven UI updates

### FFmpeg Integration
- ✅ Dynamic FFmpeg command building
- ✅ Filter chain construction for complex operations
- ✅ Stream mapping for video/audio handling
- ✅ Process execution with proper cleanup
- ✅ Progress parsing from FFmpeg output (basic)

### Media Playback
- ✅ Custom media player implementation
- ✅ Frame extraction and display pipeline
- ✅ Audio decoding and playback
- ✅ Synchronization between audio and video
- ✅ Embedded playback within application window
- ✅ Checkpoint system for playback position

### UI/UX
- ✅ Responsive layout adapting to content
- ✅ Intuitive module selection
- ✅ Clear visual feedback during operations
- ✅ Logical grouping of related controls
- ✅ Helpful hint labels for user guidance

## Milestones

- **2025-11-23** - v0.1.0-dev7 released with Source aspect ratio default
- **2025-11-22** - Documentation reorganization and expansion
- **2025-11-21** - Last successful binary build (GCC compatibility)
- **Earlier** - v0.1.0-dev1 through dev6 with progressive feature additions
  - dev6: Aspect ratio controls and cancelable converts
  - dev5: Icon and basic UI improvements
  - dev4: Build cache management
  - dev3: Media player checkpoint
  - Earlier: Initial implementation and architecture

## Development Progress

### Lines of Code (Estimated)
- **main.go**: ~2,500 lines (comprehensive Convert module, UI, player)
- **Documentation**: ~1,500 lines across multiple files
- **Total**: ~4,000+ lines

### Modules Status
- **Convert**: 60% complete (core functionality working, advanced features pending)
- **Inspect**: 20% complete (basic metadata display, needs dedicated module)
- **Merge**: 0% (planned)
- **Trim**: 0% (planned)
- **Filters**: 0% (planned)
- **Upscale**: 0% (planned)
- **Audio**: 0% (planned)
- **Thumb**: 0% (planned)
- **Rip**: 0% (planned)

### Documentation Status
- **Module Documentation**: 30% complete
  - ✅ Convert: Complete
  - ✅ Inspect: Complete
  - ✅ Rip: Complete
  - ⏳ Others: Pending
- **Design Documents**: 50% complete
  - ✅ Persistent Video Context
  - ✅ Module Overview
  - ⏳ Architecture
  - ⏳ FFmpeg Integration
- **User Guides**: 0% complete

## Bug Fixes & Improvements

### Recent Fixes
- ✅ Fixed aspect ratio default from 16:9 to Source (dev7)
- ✅ Stabilized video seeking and embedded rendering
- ✅ Improved player window positioning
- ✅ Fixed clear video functionality
- ✅ Resolved build caching issues
- ✅ Removed binary from git repository

### Performance Improvements
- ✅ Optimized preview frame generation
- ✅ Efficient FFmpeg process management
- ✅ Proper cleanup of temporary files
- ✅ Responsive UI during long operations

## Acknowledgments

### Technologies Used
- **Fyne** - Cross-platform GUI framework
- **FFmpeg/FFprobe** - Video processing and analysis
- **SDL2** - Audio playback
- **OpenGL (go-gl)** - Video rendering
- **Go** - Primary programming language

### Community Resources
- FFmpeg documentation and community
- Fyne framework documentation
- Go community and standard library

---

*Last Updated: 2025-11-23*
