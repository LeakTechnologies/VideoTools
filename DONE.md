# VideoTools - Completed Features

This file tracks completed features, fixes, and milestones.

## Version 0.1.0-dev19 (2025-12-18 to 2025-12-20) - Convert Module Cleanup & UX Polish

### Features (2025-12-20 Session)
- ✅ **History Sidebar - In Progress Tab**
  - Added "In Progress" tab to history sidebar
  - Shows running and pending jobs without opening queue
  - Animated striped progress bars per module color
  - Real-time progress updates (0-100%)
  - No delete button on active jobs (only completed/failed)
  - Dynamic status text ("Running..." or "Pending")

- ✅ **Benchmark System Overhaul**
  - **Hardware Detection Module** (`internal/sysinfo/sysinfo.go`)
    - Cross-platform CPU detection (model, cores, clock speed)
    - GPU detection with driver version (NVIDIA via nvidia-smi)
    - RAM detection with human-readable formatting
    - Linux, Windows, macOS support
  - **Hardware Info Display**
    - Shown immediately in benchmark progress view (before tests run)
    - Displayed in benchmark results view
    - Saved with each benchmark run for history
  - **Settings Persistence**
    - Hardware acceleration settings saved with benchmarks
    - Settings persist between sessions via config file
    - GPU automatically detected and used
  - **UI Polish**
    - "Run Benchmark" button highlighted (HighImportance) on first run
    - Returns to normal styling after initial benchmark
    - Guides new users to run initial benchmark

- ✅ **Bitrate Preset Simplification**
  - Reduced from 13 confusing options to 6 clear presets
  - Removed resolution references (no more "1440p" confusion)
  - Codec-agnostic (presets don't change selected codec)
  - Quality-based naming: Low/Medium/Good/High/Very High Quality
  - Focused on common use cases (1.5-8 Mbps range)
  - Presets only set bitrate and switch to CBR mode
  - User codec choice (H.264, VP9, AV1, etc.) preserved

- ✅ **Quality Preset Codec Compatibility**
  - "Lossless" quality option only available for H.265 and AV1
  - Dynamic quality dropdown based on selected codec
  - Automatic fallback to "Near-Lossless" when switching to non-lossless codec
  - Lossless + Target Size bitrate mode now supported for H.265/AV1
  - Prevents invalid codec/quality combinations

- ✅ **App Icon Improvements**
  - Regenerated VT_Icon.ico with transparent background
  - Updated LoadAppIcon() to search PNG first (better Linux support)
  - Searches both current directory and executable directory
  - Added debug logging for icon loading troubleshooting

- ✅ **UI Scaling for 800x600 Windows** (2025-12-20 continuation)
  - Reduced module tile size from 220x110 to 160x80
  - Reduced title text size from 28 to 20
  - Reduced queue tile from 160x60 to 140x50
  - Reduced section padding from 14 to 8 pixels
  - Removed extra padding wrapper around tiles
  - Made main menu scrollable with fixed header
  - All UI elements fit within 800x600 default window

- ✅ **Header Layout Improvements** (2025-12-20 continuation)
  - Changed from HBox with spacer to border layout
  - Title on left, all controls grouped compactly on right
  - Shortened button labels for space efficiency
  - "☰ History" → "☰", "Run Benchmark" → "Benchmark", "View Results" → "Results"
  - Eliminates wasted horizontal space

- ✅ **Queue Clear Behavior Fix** (2025-12-20 continuation)
  - "Clear Completed" now always returns to main menu
  - "Clear All" now always returns to main menu
  - Prevents unwanted navigation to convert module after clearing queue
  - Consistent and predictable behavior

- ✅ **Threading Safety Fix** (2025-12-20 continuation)
  - Fixed Fyne threading errors in stats bar component
  - Removed Show()/Hide() calls from Layout() method
  - Layout() can be called from any thread during resize/redraw
  - Show/Hide logic remains only in Refresh() with proper DoFromGoroutine
  - Eliminates threading warnings during UI updates

### Features (2025-12-18 Session)
- ✅ **History Sidebar Enhancements**
  - Delete button ("×") on each history entry
  - Remove individual entries from history
  - Auto-save and refresh after deletion
  - Clean, unobtrusive button placement

- ✅ **Command Preview Improvements**
  - Show/Hide button state based on preview visibility
  - Disabled when no video source loaded
  - Displays actual file paths instead of placeholders
  - Real-time live updates as settings change
  - Collapsible to save screen space

- ✅ **Format Options Reorganization**
  - Grouped by codec family (H.264 → H.265 → AV1 → VP9 → ProRes → MPEG-2)
  - Added descriptive comments for each codec type
  - Improved dropdown readability and navigation
  - Easier to find and compare similar formats

- ✅ **Bitrate Mode Clarity**
  - Descriptive labels in dropdown:
    - CRF (Constant Rate Factor)
    - CBR (Constant Bitrate)
    - VBR (Variable Bitrate)
    - Target Size (Calculate from file size)
  - Immediate understanding without documentation
  - Preserves internal compatibility with short codes

- ✅ **Root Folder Cleanup**
  - Moved all documentation .md files to docs/ folder
  - Kept only README.md, TODO.md, DONE.md in root
  - Cleaner project structure
  - Better organization for contributors

### Bug Fixes
- ✅ **Critical Convert Module Crash Fixed**
  - Fixed nil pointer dereference when opening Convert module
  - Corrected widget initialization order
  - bitrateContainer now created after bitratePresetSelect initialized
  - Eliminated "invalid memory address" panic on startup

- ✅ **Log Viewer Crash Fixed**
  - Fixed "close of closed channel" panic
  - Duplicate close handlers removed
  - Proper dialog cleanup

- ✅ **Bitrate Control Improvements**
  - CBR: Set bufsize to 2x bitrate for better encoder handling
  - VBR: Increased maxrate cap from 1.5x to 2x target bitrate
  - VBR: Added bufsize at 4x target to enforce caps
  - Prevents runaway bitrates while maintaining quality peaks

### Technical Improvements
- ✅ **Widget Initialization Order**
  - Fixed container creation dependencies
  - All Select widgets initialized before container use
  - Proper nil checking in UI construction

- ✅ **Bidirectional Label Mapping**
  - Display labels map to internal storage codes
  - Config files remain compatible
  - Clean separation of UI and data layers

## Version 0.1.0-dev18 (2025-12-15)

### Features
- ✅ **Thumbnail Module Enhancements**
  - Enhanced metadata display with 3 lines of comprehensive technical data
  - Added 8px padding between thumbnails in contact sheets
  - Increased thumbnail width to 280px for analyzable screenshots (4x8 grid = ~1144x1416)
  - Audio bitrate display alongside audio codec (e.g., "AAC 192kbps")
  - Concise bitrate display (removed "Total:" prefix)
  - Video codec, audio codec, FPS, and overall bitrate shown in metadata
  - Navy blue background (#0B0F1A) for professional appearance

- ✅ **Player Module**
  - New Player button on main menu (Teal #44FFDD)
  - Access to VT_Player for video playback
  - Video loading and preview integration
  - Module handler for CLI support

- ✅ **Filters Module - UI Complete**
  - Color correction controls (brightness, contrast, saturation)
  - Enhancement tools (sharpness, denoise)
  - Transform operations (rotation, flip horizontal/vertical)
  - Creative effects (grayscale)
  - Navigation to Upscale module with video transfer
  - Full state management for filter settings

- ✅ **Upscale Module - Fully Functional**
  - Traditional FFmpeg scaling methods: Lanczos (sharp), Bicubic (smooth), Spline (balanced), Bilinear (fast)
  - Resolution presets: 720p, 1080p, 1440p, 4K, 8K
  - "UPSCALE NOW" button for immediate processing
  - "Add to Queue" button for batch processing
  - Job queue integration with real-time progress tracking
  - AI upscaling detection (Real-ESRGAN) with graceful fallback
  - High quality encoding (libx264, preset slow, CRF 18)
  - Navigation back to Filters module

- ✅ **Snippet System Overhaul - Dual Output Modes**
  - **"Snippet to Default Format" (Checkbox CHECKED - Default)**:
    - Stream copy mode preserves exact source format, codec, bitrate
    - Zero quality loss - bit-perfect copy of source
    - Outputs to source container (.wmv → .wmv, .avi → .avi, etc.)
    - Fast processing (no re-encoding)
    - Duration: Keyframe-level precision (may vary ±1-2s)
    - Perfect for merge testing without quality changes
  - **"Snippet to Output Format" (Checkbox UNCHECKED)**:
    - Uses configured conversion settings from Convert tab
    - Applies video codec (H.264, H.265, VP9, AV1, etc.)
    - Applies audio codec (AAC, Opus, MP3, FLAC, etc.)
    - Uses encoder preset and CRF quality settings
    - Outputs to selected format (.mp4, .mkv, .webm, etc.)
    - Frame-perfect duration control (exactly configured length)
    - Perfect preview of final conversion output

- ✅ **Configurable Snippet Length**
  - Adjustable snippet length (5-60 seconds, default: 20)
  - Slider control with real-time display
  - Snippets centered on video midpoint
  - Length persists across video loads

- ✅ **Batch Snippet Generation**
  - "Generate All Snippets" button for multiple loaded videos
  - Processes all videos with same configured length
  - Consistent timestamp for uniform naming
  - Efficient queue integration
  - Shows confirmation with count of jobs added

- ✅ **Smart Job Descriptions**
  - Displays snippet length and mode in job queue
  - "10s snippet centred on midpoint (source format)"
  - "20s snippet centred on midpoint (conversion settings)"

### Technical Improvements
- ✅ **Dual-Mode Snippet System Implementation**
  - Default Format mode: Stream copy for bit-perfect source preservation
  - Output Format mode: Full conversion using user's configured settings
  - Automatic container/codec matching based on mode selection
  - Integration with conversion config (video/audio codecs, presets, CRF)
  - Smart extension handling (source format vs. selected output format)
- ✅ **Queue/Status UI polish**
  - Animated striped progress bars per module color with faster motion for visibility
  - Footer refactor: consistent dark status strip + tinted action bar across modules
  - Status bar tap restored to open Job Queue; full-width clickable strip
- ✅ **Snippet progress reporting**
  - Live progress from ffmpeg `-progress` output; 0–100% updates in status bar and queue
  - Error/log capture preserved for snippet jobs

- ✅ **Metadata Enhancement System**
  - New `getDetailedVideoInfo()` function using FFprobe
  - Extracts video codec, audio codec, FPS, video bitrate, audio bitrate
  - Multiple ffprobe calls for comprehensive data
  - Graceful fallback to format-level bitrate if stream bitrate unavailable

- ✅ **Module Navigation Pattern**
  - Bidirectional navigation between Filters and Upscale
  - Video file transfer between modules
  - Filter chain transfer capability (foundation for future)

- ✅ **Resolution Parsing System**
  - `parseResolutionPreset()` function for preset strings
  - Maps "1080p (1920x1080)" format to width/height integers
  - Support for custom resolution input (foundation)

- ✅ **Upscale Filter Builder**
  - `buildUpscaleFilter()` constructs FFmpeg scale filters
  - Method-specific scaling: lanczos, bicubic, spline, bilinear
  - Filter chain combination support

### Bug Fixes
- ✅ Fixed snippet duration issues with dual-mode approach
  - Default Format: Uses stream copy (keyframe-level precision)
  - Output Format: Re-encodes for frame-perfect duration
- ✅ Fixed container/codec mismatch in snippet generation
  - Now properly matches container to codec (MP4 for h264, source format for stream copy)
- ✅ Fixed missing audio bitrate in thumbnail metadata
- ✅ Fixed contact sheet dimensions not accounting for padding
- ✅ Added missing `strings` import to thumbnail/generator.go
- ✅ Updated snippet UI labels for clarity (Default Format vs Output Format)

### Documentation
- ✅ Updated ai-speak.md with comprehensive dev18 documentation
- ✅ Created 24-item testing checklist for dev18
- ✅ Documented all implementation details and technical decisions

## Version 0.1.0-dev17 (2025-12-14)

### Features
- ✅ **Thumbnail Module - Complete Implementation**
  - Individual thumbnail generation with customizable count (3-50 thumbnails)
  - Contact sheet generation with metadata headers
  - Customizable grid layouts (2-12 columns, 2-12 rows)
  - Even timestamp distribution across video duration
  - JPEG output with configurable quality (default: 85)
  - Configurable thumbnail width (160-640px for individual, 200px for contact sheets)
  - Saves to `{video_directory}/{video_name}_thumbnails/` for easy access
  - DejaVu Sans Mono font matching app styling
  - App background color (#0B0F1A) for contact sheet padding
  - Dynamic total count display for grid layouts

- ✅ **Thumbnail UI Integration**
  - Video preview window (640x360) in thumbnail module
  - Mode-specific controls (contact sheet: columns/rows, individual: count/width)
  - Dual button system:
    - "GENERATE NOW" - Adds to queue and starts immediately
    - "Add to Queue" - Adds for batch processing
  - "View Results" button with in-app contact sheet viewer (900x700 dialog)
  - "View Queue" button for queue access from thumbnail module
  - Drag-and-drop support for video files (universal across app)
  - Real-time grid total calculation as columns/rows change

- ✅ **Job Queue Integration for Thumbnails**
  - Background thumbnail generation with progress tracking
  - Job queue support with live progress updates
  - Can queue multiple thumbnail jobs from different videos
  - Progress callback integration for thumbnail extraction
  - Proper context cancellation support

- ✅ **Snippet Tool Improvement**
  - Changed from re-encoding to stream copy (`-c copy`)
  - Instant 20-second snippet extraction with zero quality loss
  - No encoding overhead - extracts source streams directly
  - Removed 148 lines of unnecessary encoding logic

### Technical Improvements
- ✅ **Timestamp-based Frame Selection**
  - Fixed frame selection from FPS-dependent (`eq(n,frame_num)`) to timestamp-based (`gte(t,timestamp)`)
  - Ensures correct thumbnail count regardless of video frame rate
  - Works reliably with VFR (Variable Frame Rate) content
  - Uses `setpts=N/TB` for proper timestamp reset in contact sheets

- ✅ **FFmpeg Filter Optimization**
  - Tile filter for grid layouts: `tile=COLUMNSxROWS`
  - Select filter with timestamp-based frame extraction
  - Pad filter with hex color codes for app background matching
  - Drawtext filter with font specification and positioning
  - Scale filter maintaining aspect ratios

- ✅ **Module Architecture**
  - Added thumbnail state fields to appState (thumbFile, thumbCount, thumbWidth, thumbContactSheet, thumbColumns, thumbRows, thumbLastOutputPath)
  - Implemented `showThumbView()` for thumbnail module UI
  - Implemented `buildThumbView()` for split layout (preview 55%, settings 45%)
  - Implemented `executeThumbJob()` for job queue integration
  - Universal drag-and-drop handler for all modules

- ✅ **Error Handling**
  - Disabled timestamp overlay on individual thumbnails to avoid font availability issues
  - Graceful handling of missing output directories
  - Proper error dialogs with context-specific messages
  - Exit status 234 resolution (font-related errors)

### Bug Fixes
- ✅ Fixed incorrect thumbnail count in contact sheets (was generating 34 instead of 40 for 5x8 grid)
- ✅ Fixed frame selection FPS assumption (hardcoded 30fps removed)
- ✅ Fixed module visibility (added thumb module to enabled check)
- ✅ Fixed undefined function call (openFileManager → openFolder)
- ✅ Fixed dynamic total count not updating when changing grid dimensions
- ✅ Fixed font-related crash on systems without DejaVu Sans Mono

## Version 0.1.0-dev16 (2025-12-14)

### Features
- ✅ **Interlacing Detection Module - Complete Implementation**
  - Automatic interlacing analysis using FFmpeg idet filter
  - Field order detection (TFF - Top Field First, BFF - Bottom Field First)
  - Frame-by-frame analysis with classifications:
    - Progressive frames
    - Top Field First interlaced frames
    - Bottom Field First interlaced frames
    - Undetermined frames
  - Interlaced percentage calculation
  - Status determination: Progressive (<5%), Interlaced (>95%), Mixed Content (5-95%)
  - Confidence levels: High (<5% undetermined), Medium (5-15%), Low (>15%)
  - Quick analyze mode (500 frames) for fast detection
  - Full video analysis option for comprehensive results

- ✅ **Deinterlacing Recommendations**
  - Automatic deinterlacing recommendations based on analysis
  - Suggested filter selection (yadif for compatibility)
  - Human-readable recommendations
  - SuggestDeinterlace boolean flag for programmatic use

- ✅ **Preview Generation**
  - Deinterlace preview at specific timestamps
  - Side-by-side comparison (original vs deinterlaced)
  - Uses yadif filter for preview generation
  - Frame extraction with proper scaling

### Technical Improvements
- ✅ **Detector Implementation**
  - Created `/internal/interlace/detector.go` package
  - NewDetector() constructor accepting ffmpeg and ffprobe paths
  - Analyze() method with configurable sample frame count
  - QuickAnalyze() convenience method for 500-frame sampling
  - Regex-based parsing of idet filter output
  - Multi-frame detection statistics extraction

- ✅ **Detection Result Structure**
  - Comprehensive DetectionResult type with all metrics
  - String() method for formatted output
  - Percentage calculations for interlaced content
  - Field order determination logic
  - Confidence calculation based on undetermined ratio

- ✅ **FFmpeg Integration**
  - idet filter integration for interlacing detection
  - Proper stderr pipe handling for filter statistics
  - Context-aware command execution with cancellation support
  - Null output format for analysis-only operations

### Documentation
- ✅ Added interlacing detection to module list
- ✅ Documented detection algorithms and thresholds
- ✅ Explained field order types and their implications

## Version 0.1.0-dev13 (In Progress - 2025-12-03)

### Features
- ✅ **Automatic Black Bar Detection and Cropping**
  - Detects and removes black bars to reduce file size (15-30% typical reduction)
  - One-click "Detect Crop" button analyzes video using FFmpeg cropdetect
  - Samples 10 seconds from middle of video for stable detection
  - Shows estimated file size reduction percentage before applying
  - User confirmation dialog displays before/after dimensions
  - Manual crop override capability (width, height, X/Y offsets)
  - Applied before scaling for optimal results
  - Works in both direct convert and queue job execution
  - Proper handling for videos without black bars
  - 30-second timeout protection for detection process

- ✅ **Frame Rate Conversion UI with Size Estimates**
  - Comprehensive frame rate options: Source, 23.976, 24, 25, 29.97, 30, 50, 59.94, 60
  - Intelligent file size reduction estimates (40-50% for 60→30 fps)
  - Real-time hints showing "Converting X → Y fps: ~Z% smaller file"
  - Warning for upscaling attempts with judder notice
  - Automatic calculation based on source and target frame rates
  - Dynamic updates when video or frame rate changes
  - Supports both film (24 fps) and broadcast standards (25/29.97/30)
  - Uses FFmpeg fps filter for frame rate conversion

- ✅ **Encoder Preset Descriptions with Speed/Quality Trade-offs**
  - Detailed information for all 9 preset options
  - Speed comparisons relative to "slow" and "medium" baselines
  - File size impact percentages for each preset
  - Visual icons indicating speed categories (⚡⏩⚖️🎯🐌)
  - Recommends "slow" as best quality/size ratio
  - Dynamic hint updates when preset changes
  - Helps users make informed encoding time decisions
  - Ranges from ultrafast (~10x faster, ~30% larger) to veryslow (~5x slower, ~15-20% smaller)

- ✅ **Compare Module**
  - Side-by-side video comparison interface
  - Load two videos and compare detailed metadata
  - Displays format, resolution, codecs, bitrates, frame rate, pixel format
  - Shows color space, color range, GOP size, field order
  - Indicates presence of chapters and metadata
  - Accessible via GUI button (pink color) or CLI: `videotools compare <file1> <file2>`
  - Added formatBitrate() helper function for consistent bitrate display

- ✅ **Target File Size Encoding Mode**
  - New "Target Size" bitrate mode in convert module
  - Specify desired output file size (e.g., "25MB", "100MB", "8MB")
  - Automatically calculates required video bitrate based on:
    - Target file size
    - Video duration
    - Audio bitrate
    - Container overhead (3% reserved)
  - Implemented ParseFileSize() to parse size strings (KB, MB, GB)
  - Implemented CalculateBitrateForTargetSize() for bitrate calculation
  - Works in both GUI convert view and job queue execution
  - Minimum bitrate sanity check (100 kbps) to prevent invalid outputs

### Technical Improvements
- ✅ Added compare command to CLI help text
- ✅ Consistent "Target Size" naming throughout UI and code
- ✅ Added compareFile1 and compareFile2 to appState for video comparison
- ✅ Module button grid updated with compare button (pink/magenta color)

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
- ✅ Ranked benchmark results by score and added cancel confirmation
- ✅ Added estimated audio bitrate fallback when metadata is missing
- ✅ Made target file size input unit-selectable with numeric-only entry
- ✅ Prevented snippet runaway bitrates when using Match Source Format
- ✅ History sidebar refreshes when jobs complete (snippet entries now appear)
- ✅ Benchmark errors now show non-blocking notifications instead of OK popups
- ✅ Fixed stats bar updates to run on the UI thread to avoid Fyne warnings
- ✅ Defaulted Target Aspect Ratio back to Source unless user explicitly sets it
- ✅ Synced Target Aspect Ratio between Simple and Advanced menus
- ✅ Hide manual CRF input when Lossless quality is selected
- ✅ Upscale now recomputes target dimensions from the preset to ensure 2X/4X apply
- ✅ Added unit selector for manual video bitrate entry
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
