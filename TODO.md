# VideoTools TODO (v0.1.0-dev13 plan)

This file tracks upcoming features, improvements, and known issues.

## Priority Features for dev13 (Based on Jake's research)

### Quality & Compression Improvements
- [ ] **Automatic black bar detection and cropping** (HIGHEST PRIORITY)
  - Implement ffmpeg cropdetect analysis pass
  - Auto-apply detected crop values
  - 15-30% file size reduction with zero quality loss
  - Add manual crop override option

- [ ] **Frame rate conversion UI**
  - Dropdown: Source, 24, 25, 29.97, 30, 50, 59.94, 60 fps
  - Auto-suggest 60→30fps conversion with size estimate
  - Show file size impact (40-45% reduction for 60→30)

- [ ] **HEVC/H.265 preset options**
  - Add preset dropdown: ultrafast, superfast, veryfast, faster, fast, medium, slow, slower, veryslow
  - Show time/quality trade-off estimates
  - Default to "slow" for best quality/size balance

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

### Merge Module (Not Started)
- [ ] Design UI layout
- [ ] Implement file list/order management
- [ ] Add drag-and-drop reordering
- [ ] Preview transitions
- [ ] Handle mixed formats/resolutions
- [ ] Audio normalization across clips
- [ ] Transition effects (optional)
- [ ] Chapter markers at join points

### Trim Module (Not Started)
- [ ] Design UI with timeline
- [ ] Implement frame-accurate seeking
- [ ] Visual timeline with preview thumbnails
- [ ] Multiple trim ranges selection
- [ ] Chapter-based splitting
- [ ] Smart copy mode (no re-encode)
- [ ] Batch trim operations
- [ ] Keyboard shortcuts for marking in/out points

### Filters Module (Not Started)
- [ ] Design filter selection UI
- [ ] Implement color correction filters
  - [ ] Brightness/Contrast
  - [ ] Saturation/Hue
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

### Thumb Module (Not Started)
- [ ] Design thumbnail generation UI
- [ ] Single thumbnail extraction
- [ ] Grid/contact sheet generation
- [ ] Customizable layouts
- [ ] Scene detection
- [ ] Animated thumbnails
- [ ] Batch processing
- [ ] Template system

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
- [ ] Installation instructions (Windows, macOS, Linux)
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
- [ ] Create macOS app bundle (.dmg)
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
- No Windows/macOS builds tested yet
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
