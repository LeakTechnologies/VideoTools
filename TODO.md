# VideoTools TODO (v0.1.0-dev19+ plan)

This file tracks upcoming features, improvements, and known issues.

## Current Focus: dev19 - Convert Module Cleanup & Polish

### In Progress
- [ ] **AI Frame Interpolation Support**
  - RIFE (Real-Time Intermediate Flow Estimation) - https://github.com/hzwer/ECCV2022-RIFE
  - FILM (Frame Interpolation for Large Motion) - https://github.com/google-research/frame-interpolation
  - DAIN (Depth-Aware Video Frame Interpolation) - https://github.com/baowenbo/DAIN
  - CAIN (Channel Attention Is All You Need) - https://github.com/myungsub/CAIN
  - Python-based models, need Go bindings or CLI wrappers
  - Model download/management system
  - UI controls for model selection

- [ ] **Color Space Preservation**
  - Fix color space preservation in upscale module
  - Ensure all conversions preserve color metadata (color_space, color_primaries, color_trc, color_range)
  - Test with HDR content

## Priority Features for dev20+

### Quality & Polish Improvements
- [ ] **UI/UX refinements**
  - Improve error message clarity and detail
  - Add progress indicators for long operations (striped bars landed; continue refining status cues)
  - Enhance drag-and-drop feedback
  - Add keyboard shortcuts for common actions

- [ ] **Performance optimizations**
  - Optimize preview frame generation
  - Reduce memory usage for large files
  - Improve queue processing efficiency
  - Add parallel processing options

- [ ] **Advanced Convert features**
  - Implement 2-pass encoding UI
  - Add custom FFmpeg arguments field
  - Create encoding preset save/load system
  - Add file size estimator

### Module Development
- [ ] **Merge module implementation**
  - Design UI layout for file joining
  - Implement drag-and-drop reordering
  - Add format conversion for mixed sources
  - Create preview functionality

- [ ] **Trim module implementation**
  - Timeline-based editing interface
  - Frame-accurate seeking
  - Multiple range selection
  - Smart copy mode detection

- [ ] **Filters module implementation**
  - Color correction controls
  - Enhancement filters (sharpen, denoise)
  - Creative effects (grayscale, vignette)
  - Real-time preview system

### Quality & Compression Improvements
- [x] **Automatic black bar detection and cropping** (v0.1.0-dev13 - COMPLETED)
  - Implement ffmpeg cropdetect analysis pass
  - Auto-apply detected crop values
  - 15-30% file size reduction with zero quality loss
  - Add manual crop override option

- [x] **Frame rate conversion UI** (v0.1.0-dev13 - COMPLETED)
  - Dropdown: Source, 23.976, 24, 25, 29.97, 30, 50, 59.94, 60 fps
  - Auto-suggest 60→30fps conversion with size estimate
  - Show file size impact (40-50% reduction for 60→30)

- [x] **HEVC/H.265 encoder preset options** (v0.1.0-dev13 - COMPLETED)
  - Preset dropdown: ultrafast, superfast, veryfast, faster, fast, medium, slow, slower, veryslow
  - Show time/quality trade-off estimates
  - Recommend "slow" for best quality/size balance

- [ ] **Advanced filters module**
  - Denoising: hqdn3d (fast), nlmeans (slow, high quality)
  - Sharpening: unsharp filter with strength slider
  - Deblocking: remove compression artifacts
  - All with strength sliders and preview

### Encoding Features
- [ ] **2-pass encoding for precise bitrate targeting**
  - UI for target file size
  - Auto-calculate bitrate from duration + size
  - Progress tracking for both passes

- [ ] **SVT-AV1 codec support**
  - Faster than H.265, smaller files
  - Add compatibility warnings for iOS
  - Preset selection (0-13)

### UI & Workflow
- [ ] **Add UI controls for dev12 backend features**
  - H.264 profile/level dropdowns
  - Deinterlace method selector (yadif/bwdif)
  - Audio normalization checkbox
  - Auto-crop toggle

- [ ] **Encoding presets system**
  - "iPhone Compatible" preset (main/4.0, stereo, 48kHz, auto-crop)
  - "Maximum Compression" preset (H.265, slower, CRF 24, 10-bit, auto-crop)
  - "Fast Encode" preset (medium, hardware encoding)
  - Save custom presets

- [ ] **File size estimator**
  - Show estimated output size before encoding
  - Based on source duration, target bitrate/CRF
  - Update in real-time as settings change

### VR & Advanced Features
- [ ] **VR video support infrastructure**
  - Detect VR metadata tags
  - Side-by-side and over-under format detection
  - Preserve VR metadata in output
  - Add VR-specific presets

- [ ] **Batch folder import**
  - Select folder, auto-add all videos to queue
  - Filter by extension
  - Apply same settings to all files
  - Progress indicator for folder scanning

## Windows Compatibility (COMPLETED in dev14)

### Build System
- [x] **Cross-compilation setup** ✅ COMPLETED
  - Configure CGO for Windows cross-compilation
  - Set up MinGW-w64 toolchain
  - Test Fyne compilation on Windows
  - Create Windows build script equivalent to build.sh

- [x] **Platform detection system** ✅ COMPLETED
  - Bundle ffmpeg.exe with Windows builds
  - Include all required DLLs (OpenGL, etc.)
  - Create installer with dependencies
  - Add ffmpeg to PATH or bundle in application directory

### Platform-Specific Code
- [x] **Path handling** ✅ COMPLETED
  - Replace Unix path separators with filepath.Separator
  - Handle Windows drive letters (C:\, D:\, etc.)
  - Support UNC paths (\\server\share\)
  - Test with spaces and special characters in paths

- [x] **Platform detection system** ✅ COMPLETED
  - Ensure Fyne file dialogs work on Windows
  - Test drag-and-drop on Windows Explorer
  - Handle Windows file associations
  - Add "Open with VideoTools" context menu option

- [x] **Process management** ✅ COMPLETED
  - Test ffmpeg process spawning on Windows
  - Handle Windows process termination (no SIGTERM)
  - Support Windows-style console output
  - Test background process handling

### Hardware Detection
- [x] **Windows GPU detection** ✅ COMPLETED
  - Detect NVIDIA GPUs (NVENC) on Windows
  - Detect Intel integrated graphics (QSV)
  - Detect AMD GPUs (AMF)
  - Auto-select best available encoder

- [x] **Windows-specific encoders** ✅ COMPLETED
  - Add Windows Media Foundation encoders
  - Test NVENC on Windows (h264_nvenc, hevc_nvenc)
  - Test Intel QSV on Windows
  - Add fallback to software encoding

### Testing & Distribution
- [x] **Windows testing** ⏳ CORE IMPLEMENTATION COMPLETE
  - Test on Windows 10 *(requires Windows environment)*
  - Test on Windows 11 *(requires Windows environment)*
  - Test with different GPU vendors *(requires Windows environment)*
  - Test on systems without GPU *(requires Windows environment)*

- [ ] **Installation** *(planned for dev15)*
  - Create Windows installer (MSI or NSIS)
  - Add to Windows Start Menu
  - Create desktop shortcut option
  - Auto-update mechanism

- [x] **Documentation** ✅ COMPLETED
  - Windows installation guide
  - Windows-specific troubleshooting
  - GPU driver requirements
  - Antivirus whitelist instructions

### Nice-to-Have
- [ ] Windows Store submission
- [ ] Portable/USB-stick version
- [ ] Windows taskbar progress integration
- [ ] File thumbnail generation for Windows Explorer
- [ ] Windows notification system integration

## Critical Issues / Polishing
- [ ] Queue polish: ensure scroll/refresh stability with 10+ jobs and long runs
- [ ] Direct+queue parity: verify label/progress/order are correct when mixing modes
- [ ] Conversion error surfacing: include stderr snippet in dialog for faster debug
- [ ] DVD author helper (optional): one-click VIDEO_TS/ISO from DVD .mpg
- [ ] Build reliability: document cgo/GL deps and avoid accidental cache wipes

## Core Features

### Persistent Video Context
- [ ] Implement video info bar UI component
- [ ] Add "Clear Video" button globally accessible
- [ ] Update all modules to check for `state.source`
- [ ] Add "Use Different Video" option in modules
- [ ] Implement auto-clear preferences
- [ ] Add recent files tracking and dropdown menu
- [ ] Test video persistence across module switches

### Convert Module Completion (dev12 focus)
- [ ] Add hardware acceleration UI controls (NVENC, QSV, VAAPI)
- [ ] Implement two-pass encoding mode
- [ ] Add bitrate-based encoding option (not just CRF)
- [ ] Implement custom FFmpeg arguments field
- [ ] Add preset save/load functionality
- [x] Add batch conversion queue (v0.1.0-dev11)
- [x] Multi-video loading and navigation (v0.1.0-dev11)
- [ ] Estimated file size calculator
- [ ] Preview/comparison mode
- [ ] Audio-only output option
- [ ] Add more codec options (AV1, VP9)

### Blu-ray Encoding System (dev15+ priority)

#### Blu-ray Standards Implementation
- [ ] **Blu-ray Disc Specifications**
  - **Resolution Support**: 1920×1080 (Full HD), 1280×720 (HD), 3840×2160 (4K UHD)
  - **Frame Rates**: 23.976, 24, 25, 29.97, 50, 59.94 fps
  - **Video Codecs**: H.264/AVC, H.265/HEVC, VP9 (optional)
  - **Audio Codecs**: LPCM, Dolby Digital (AC-3), Dolby Digital Plus (E-AC-3), DTS, DTS-HD
  - **Container**: MPEG-2 Transport Stream (.m2ts) with Blu-ray compatibility

#### Multi-Region Blu-ray Support
- [ ] **Region A** (Americas, East Asia, Southeast Asia)
  - NTSC-based standards (23.976, 29.97, 59.94 fps)
  - Primary audio: English, Spanish, French, Portuguese
  - Subtitle support for major languages

- [ ] **Region B** (Europe, Africa, Middle East, Australia, New Zealand)
  - PAL/SECAM-based standards (25, 50 fps)
  - Primary audio: English, French, German, Italian, Spanish
  - Extensive subtitle support for European languages

- [ ] **Region C** (Central Asia, South Asia, East Asia)
  - Mixed standards support
  - Primary audio: Mandarin, Cantonese, Korean, Japanese, Hindi
  - Complex subtitle requirements (CJK character sets)

#### Professional Blu-ray Features
- [ ] **Advanced Video Encoding**
  - **H.264 High Profile Level 4.1/5.1** for 1080p content
  - **H.265 Main 10 Profile** for HDR content
  - **Variable Bitrate (VBR)** encoding with peak bitrate management
  - **GOP structure optimization** for Blu-ray compatibility
  - **Color space support**: Rec. 601, Rec. 709, Rec. 2020
  - **HDR metadata**: HDR10, Dolby Vision (optional)

- [ ] **Professional Audio System**
  - **LPCM (Linear PCM)**: Uncompressed audio for maximum quality
  - **Dolby Digital Plus (E-AC-3)**: Enhanced compression with surround support
  - **DTS-HD Master Audio**: Lossless audio compression
  - **Multi-channel support**: 5.1, 7.1, and object-based audio
  - **Sample rates**: 48 kHz, 96 kHz, 192 kHz
  - **Bit depth**: 16-bit, 24-bit, 32-bit

#### Blu-ray Validation System
- [ ] **Comprehensive Validation**
  - **Bitrate compliance checking** (max 40 Mbps for video, 48 Mbps total)
  - **Resolution and framerate validation** per Blu-ray spec
  - **Audio codec and channel validation**
  - **Subtitle format and encoding validation**
  - **Container format compliance checking**
  - **HDR metadata validation** for HDR content

- [ ] **Quality Assurance**
  - **Professional authoring compatibility** (Adobe Encore, Scenarist)
  - **Standalone Blu-ray player compatibility**
  - **PlayStation 3/4/5 compatibility testing**
  - **Xbox One/Series X compatibility testing**
  - **PC software player compatibility** (PowerDVD, VLC, MPC-HC)

#### Technical Implementation
- [ ] **Blu-ray Package Structure**
  - `internal/convert/bluray.go` - Blu-ray encoding logic
  - `internal/convert/bluray_regions.go` - Regional Blu-ray standards
  - `internal/convert/bluray_validation.go` - Compliance checking
  - `internal/app/bluray_adapter.go` - Integration layer

- [ ] **FFmpeg Command Generation**
  - **H.264/AVC encoding parameters** for Blu-ray compliance
  - **H.265/HEVC encoding parameters** for UHD Blu-ray
  - **Audio encoding pipelines** for all supported formats
  - **Transport stream muxing** with proper Blu-ray parameters
  - **Subtitle and metadata integration**

#### User Interface Integration
- [ ] **Blu-ray Format Selection**
  - **Blu-ray 1080p (H.264)** - Standard Full HD
  - **Blu-ray 1080p (H.265)** - High efficiency
  - **Blu-ray 4K (H.265)** - Ultra HD
  - **Blu-ray 720p (H.264)** - HD option
  - **Region selection** (A/B/C) with auto-detection

- [ ] **Advanced Options Panel**
  - **Video codec selection** (H.264, H.265)
  - **Audio codec selection** (LPCM, AC-3, E-AC-3, DTS-HD)
  - **Quality presets** (Standard, High, Cinema, Archive)
  - **HDR options** (SDR, HDR10, Dolby Vision)
  - **Multi-language audio and subtitle tracks**

#### Compatibility Targets
- [ ] **Professional Authoring Software**
  - Adobe Encore CC compatibility
  - Sony Scenarist compatibility
  - DVDLogic EasyBD compatibility
  - MultiAVCHD compatibility

- [ ] **Hardware Player Compatibility**
  - Sony PlayStation 3/4/5
  - Microsoft Xbox One/Series X|S
  - Standalone Blu-ray players (all major brands)
  - 4K Ultra HD Blu-ray players
  - Portable Blu-ray players

- [ ] **Software Player Compatibility**
  - CyberLink PowerDVD
  - ArcSoft TotalMedia Theatre
  - VLC Media Player
  - MPC-HC/MPC-BE
  - Windows Media Player (with codecs)

#### File Structure and Output
- [ ] **Output Formats**
  - **Single M2TS files** for direct burning
  - **BDMV folder structure** for full Blu-ray authoring
  - **ISO image creation** for disc burning
  - **AVCHD compatibility** for DVD media

- [ ] **Metadata and Navigation**
  - **Chapter marker support**
  - **Menu structure preparation**
  - **Subtitle track management**
  - **Audio stream organization**
  - **Thumbnail generation** for menu systems

#### Development Phases
- [ ] **Phase 1: Basic Blu-ray Support**
  - H.264 1080p encoding
  - AC-3 audio support
  - Basic validation system
  - Region A implementation

- [ ] **Phase 2: Advanced Features**
  - H.265/HEVC support
  - Multi-region implementation
  - LPCM and DTS-HD audio
  - Advanced validation

- [ ] **Phase 3: Professional Features**
  - 4K UHD support
  - HDR content handling
  - Professional authoring compatibility
  - Advanced audio options

#### Integration with Existing Systems
- [ ] **Queue System Integration**
  - Blu-ray job types in queue
  - Progress tracking for long encodes
  - Batch Blu-ray processing
  - Error handling and recovery

- [ ] **Convert Module Integration**
  - Blu-ray presets in format selector
  - Auto-resolution for Blu-ray standards
  - Quality tier system
  - Validation warnings before encoding

#### Documentation and Testing
- [ ] **Documentation Requirements**
  - `BLURAY_IMPLEMENTATION_SUMMARY.md` - Technical specifications
  - `BLURAY_USER_GUIDE.md` - User workflow documentation
  - `BLURAY_COMPATIBILITY.md` - Hardware/software compatibility
  - Updated `MODULES.md` with Blu-ray features

- [ ] **Testing Requirements**
  - **Compatibility testing** with major Blu-ray authoring software
  - **Hardware player testing** across different brands
  - **Quality validation** with professional tools
  - **Performance benchmarking** for encoding times
  - **Cross-platform testing** (Windows, Linux)

### Merge Module (Not Started)
- [ ] Design UI layout
- [ ] Implement file list/order management
- [ ] Add drag-and-drop reordering
- [ ] Preview transitions
- [ ] Handle mixed formats/resolutions
- [ ] Audio normalization across clips
- [ ] Transition effects (optional)
- [ ] Chapter markers at join points

### Trim Module (Lossless-Cut Inspired) 🔄 PLANNED
Trim provides frame-accurate cutting with lossless-first philosophy (inspired by Lossless-Cut):

#### Core Features
- [ ] **Lossless-First Approach** - Stream copy when possible, smart re-encode fallback
- [ ] **Keyframe-Snapping Timeline** - Visual keyframe markers with smart snapping
- [ ] **Frame-Accurate Navigation** - Reuse VT_Player's keyframe detection system
- [ ] **Smart Export System** - Automatic method selection (lossless/re-encode/hybrid)
- [ ] **Multi-Segment Trimming** - Multiple cuts from single source with auto-chapters

#### UI/UX Features
- [ ] **Timeline Interface** - Zoomable timeline with keyframe visibility (reuse VT_Player)
- [ ] **Visual Markers** - Blue (in), Red (out), Green (current position)
- [ ] **Keyboard Shortcuts** - I (in), O (out), X (clear), ←→ (frames), ↑↓ (keyframes)
- [ ] **Preview System** - Instant segment preview with loop option
- [ ] **Quality Indicators** - Real-time feedback on export method and quality

#### Technical Implementation
- [ ] **Stream Analysis** - Detect lossless trim possibility automatically
- [ ] **Smart Export Logic** - Choose optimal method based on content and markers
- [ ] **Format Conversion** - Handle format changes during trim operations
- [ ] **Quality Validation** - Verify output integrity and quality preservation
- [ ] **Error Recovery** - Smart suggestions when export fails

#### Integration Points
- [ ] **VT_Player Integration** - Reuse keyframe detector and timeline widget
- [ ] **Queue System** - Batch trim operations with progress tracking
- [ ] **Chapter System** - Auto-create chapters for each segment
- [ ] **Convert Module** - Seamless format conversion during trim

**FFmpeg Features:** Seeking, segment muxer, stream copying, smart re-encoding
**Current Status:** Planning complete, implementation ready for dev15
**Inspiration:** Lossless-Cut's lossless-first philosophy with modern enhancements

### Filters Module (Not Started)
- [ ] Design filter selection UI
- [ ] Implement color correction filters
  - [ ] Brightness/Contrast
  - [ ] Saturation/Hue
  - [ ] LUT support (1D/3D .cube load/apply) — primary home in Filters menu; optionally expose quick apply in Convert presets
  - [ ] Color balance
  - [ ] Curves/Levels
- [ ] Implement enhancement filters
  - [ ] Sharpen/Blur
  - [ ] Denoise
  - [ ] Deband
- [ ] Implement creative filters
  - [ ] Grayscale/Sepia
  - [ ] Vignette
  - [ ] Speed adjustment
  - [ ] Rotation/Flip
- [ ] Implement stabilization
- [ ] Add real-time preview
- [ ] Filter presets
- [ ] Custom filter chains

### Upscale Module (Not Started)
- [ ] Design UI for upscaling
- [ ] Implement traditional scaling (Lanczos, Bicubic)
- [ ] Integrate Waifu2x (if feasible)
- [ ] Integrate Real-ESRGAN (if feasible)
- [ ] Add resolution presets
- [ ] Quality vs. speed slider
- [ ] Before/after comparison
- [ ] Batch upscaling

### Audio Module (Not Started)
- [ ] Design audio extraction UI
- [ ] Implement audio track extraction
- [ ] Audio track replacement/addition
- [ ] Multi-track management
- [ ] Volume normalization
- [ ] Audio delay correction
- [ ] Format conversion
- [ ] Channel mapping
- [ ] Audio-only operations

### Thumb Module ✅ COMPLETED (v0.1.0-dev18)
- [x] Design thumbnail generation UI
- [x] Single thumbnail extraction
- [x] Grid/contact sheet generation
- [x] Customizable layouts (columns/rows 2-12)
- [x] Batch processing (job queue integration)
- [x] Contact sheet metadata headers
- [x] Preview window integration
- [x] Dual-mode settings (individual vs contact sheet)
- [x] Dynamic total count display
- [x] View results in-app
- [ ] Scene detection (future enhancement)
- [ ] Animated thumbnails (future enhancement)
- [ ] Template system (future enhancement)

### Inspect Module (Partial)
- [ ] Enhanced metadata display
- [ ] Stream information viewer
- [ ] Chapter viewer/editor
- [ ] Cover art viewer/extractor
- [ ] HDR metadata display
- [ ] Export reports (text/JSON)
- [ ] MediaInfo integration
- [ ] Comparison mode (before/after conversion)

### Rip Module (Not Started)
- [ ] Design disc ripping UI
- [ ] DVD drive detection and scanning
- [ ] Blu-ray drive support
- [ ] ISO file loading
- [ ] Title selection interface
- [ ] Track management (audio/subtitle)
- [ ] libdvdcss integration
- [ ] libaacs integration
- [ ] Batch ripping
- [ ] Metadata lookup integration

## Additional Modules

### Files Module (Proposed)
Built-in Video File Explorer/Manager for comprehensive file management without leaving VideoTools.

#### Core Features
- [ ] **File Browser Interface**
  - Open folder selection with hierarchical tree view
  - Batch drag-and-drop support for multiple files
  - Recursive folder scanning with file filtering
  - Video file type detection and filtering
  - Recent folders quick access

- [ ] **Metadata Table/Grid View**
  - Sortable columns: Filename, Size, Duration, Codec, Resolution, FPS, Bitrate, Format
  - Fast metadata loading with caching
  - Column customization (show/hide, reorder)
  - Multi-select support for batch operations
  - Search/filter capabilities

- [ ] **Integration with Existing Modules**
  - Seamless Compare module integration for video comparison
  - Direct file loading into Convert module
  - Quick access to Inspect module for file properties
  - Return navigation flow after module actions
  - Maintain selection state across module switches

- [ ] **File Management Tools**
  - Delete with confirmation dialog ("Are you sure?")
  - Move/copy file operations
  - Rename functionality
  - File organization tools
  - Recycle bin safety (platform-specific)

- [ ] **Context Menu System**
  - Right-click context menu for all file operations
  - "Open in Player" - Launch VT_Player or internal player
  - "Open in External Player" - System default or configured external player
  - "File Properties" - Open in Inspect module
  - "Convert" - Pre-load file into Convert module
  - "Compare" - Add to Compare module
  - "Delete" - Confirmation prompt before deletion

- [ ] **UI/UX Enhancements**
  - Grid view and list view toggle
  - Thumbnail preview column (optional)
  - File size/duration statistics for selections
  - Batch operation progress indicators
  - Drag-and-drop to other modules

#### Technical Implementation
- [ ] **Efficient Metadata Caching**
  - Background metadata scanning
  - SQLite database for fast lookups
  - Incremental folder scanning
  - Smart cache invalidation

- [ ] **Cross-Platform File Operations**
  - Platform-specific delete (trash vs recycle bin)
  - External player detection and configuration
  - File association handling
  - Permission management

#### Integration Architecture
- [ ] **Module Interconnection**
  - Files → Compare: Select 2+ files for comparison
  - Files → Convert: Single-click pre-load into Convert
  - Files → Inspect: Double-click or context menu
  - Module → Files: "Return to Files" button in other modules
  - Persistent selection state across navigation

- [ ] **Color-Coded Module Navigation**
  - Each module has a signature color (already established)
  - Buttons/links to other modules use that module's color
  - Creates visual consistency and intuitive navigation
  - Example: "Compare" button in Files uses Compare module's color
  - Example: "Convert" button in Files uses Convert module's color

**Current Status:** Proposed for VideoTools workflow integration
**Priority:** High - Significantly improves user workflow and file management

### Subtitle Module (Proposed)
- [ ] Requirements analysis
- [ ] UI design
- [ ] Extract subtitle tracks
- [ ] Add/replace subtitles
- [ ] Burn subtitles into video
- [ ] Format conversion
- [ ] Timing adjustment
- [ ] Multi-language support

### Streams Module (Proposed)
- [ ] Requirements analysis
- [ ] UI design
- [ ] Stream viewer/inspector
- [ ] Stream selection/removal
- [ ] Stream reordering
- [ ] Map streams to outputs
- [ ] Default flag management

### GIF Module (Proposed)
- [ ] Requirements analysis
- [ ] UI design
- [ ] Video segment to GIF
- [ ] Palette optimization
- [ ] Frame rate control
- [ ] Loop settings
- [ ] Dithering options
- [ ] Preview before export

### Crop Module (Proposed)
- [ ] Requirements analysis
- [ ] UI design
- [ ] Visual crop selector
- [ ] Auto-detect black bars
- [ ] Aspect ratio presets
- [ ] Preview with crop overlay
- [ ] Batch crop with presets

### Screenshots Module (Proposed)
- [ ] Requirements analysis
- [ ] UI design
- [ ] Single frame extraction
- [ ] Burst capture
- [ ] Scene-based capture
- [ ] Format options
- [ ] Batch processing

## UI/UX Improvements

### General Interface
- [ ] Keyboard shortcuts system
- [x] Drag-and-drop file loading (v0.1.0-dev11)
- [x] Multiple file drag-and-drop with batch processing (v0.1.0-dev11)
- [ ] **Color-Coded Module Navigation System**
  - Apply module signature colors to all references/buttons pointing to that module
  - Creates visual consistency and intuitive navigation loop
  - Example: "Convert" button anywhere uses Convert module's color
  - Example: "Compare" link uses Compare module's color
  - Applies globally across all modules for unified experience
- [ ] Dark/light theme toggle
- [ ] Custom color schemes
- [ ] Window size/position persistence
- [ ] Multi-window support
- [ ] Responsive layout improvements

### Media Player
- [ ] Enhanced playback controls
- [ ] Frame-by-frame navigation
- [ ] Playback speed control
- [ ] A-B repeat loop
- [ ] Snapshot/screenshot button
- [ ] Audio waveform display
- [ ] Subtitle display during playback

### Queue/Batch System
- [x] Global job queue (v0.1.0-dev11)
- [x] Priority management (v0.1.0-dev11)
- [x] Pause/resume individual jobs (v0.1.0-dev11)
- [x] Queue persistence (v0.1.0-dev11)
- [x] Job history (v0.1.0-dev11)
- [x] Persistent status bar showing queue stats (v0.1.0-dev11)
- [ ] Parallel processing option
- [ ] Estimated completion time

### Settings/Preferences
- [ ] Settings dialog
- [ ] Default output directory
- [ ] FFmpeg path configuration
- [ ] Hardware acceleration preferences
- [ ] Auto-clear video behavior
- [ ] Preview quality settings
- [ ] Logging verbosity
- [ ] Update checking

## Performance & Optimization

- [ ] Optimize preview frame generation
- [ ] Cache metadata for recently opened files
- [ ] Implement progressive loading for large files
- [ ] Add GPU acceleration detection
- [ ] Optimize memory usage for long videos
- [ ] Background processing improvements
- [ ] FFmpeg process management enhancements

## Testing & Quality

- [ ] Unit tests for core functions
- [ ] Integration tests for FFmpeg commands
- [ ] UI automation tests
- [ ] Test suite for different video formats
- [ ] Regression tests
- [ ] Performance benchmarks
- [ ] Error handling improvements
- [ ] Logging system enhancements

## Documentation

### User Documentation
- [ ] Complete README.md for all modules
- [ ] Getting Started guide
- [ ] Installation instructions (Windows, Linux)
- [ ] Keyboard shortcuts reference
- [ ] Workflow examples
- [ ] FAQ section
- [ ] Troubleshooting guide
- [ ] Video tutorials (consider for future)

### Developer Documentation
- [ ] Architecture overview
- [ ] Code structure documentation
- [ ] FFmpeg integration guide
- [ ] Contributing guidelines
- [ ] Build instructions for all platforms
- [ ] Release process documentation
- [ ] API documentation (if applicable)

## Packaging & Distribution

- [ ] Create installers for Windows (.exe/.msi)
- [ ] Create Linux packages (.deb, .rpm, AppImage)
- [ ] Set up CI/CD pipeline
- [ ] Automatic builds for releases
- [ ] Code signing (Windows/macOS)
- [ ] Update mechanism
- [ ] Crash reporting system

## Future Considerations

- [ ] Plugin system for extending functionality
- [ ] Scripting/automation support
- [ ] Command-line interface mode
- [ ] Web-based remote control
- [ ] Cloud storage integration
- [ ] Collaborative features
- [ ] AI-powered scene detection
- [ ] AI-powered quality enhancement
- [ ] Streaming output support
- [ ] Live input support (webcam, capture card)

## Known Issues

- **Build hangs on GCC 15.2.1** - CGO compilation freezes during OpenGL binding compilation
- No Windows builds tested yet
- Preview frames not cleaned up on crash

## Fixed Issues (v0.1.0-dev11)

- ✅ Limited error messages for FFmpeg failures - Added "Copy Error" button to all error dialogs
- ✅ No progress indication during metadata parsing - Added persistent stats bar showing real-time progress
- ✅ Crash when dragging multiple files - Improved error handling with detailed reporting
- ✅ Queue callback deadlocks - Fixed by running callbacks in goroutines
- ✅ Queue deserialization panic - Fixed formatOption struct handling

## Research Needed

- [ ] Best practices for FFmpeg filter chain optimization
- [ ] GPU acceleration capabilities across platforms
- [ ] AI upscaling integration options
- [ ] Disc copy protection legal landscape
- [ ] Cross-platform video codecs support
- [ ] HDR/Dolby Vision handling
