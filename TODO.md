# VideoTools TODO (v0.1.1-dev27+ plan)

This file tracks upcoming features, improvements, and known issues.

## Maintenance

- [ ] **Git converter retirement**
  - Preserve presets, then deprecate `scripts/legacy/git_converter`.
- [ ] **Forgejo Windows runner validation**
  - Confirm the Windows packaging workflow completes without context canceled after the UTF-8 `GITHUB_OUTPUT` fix.
- [ ] **Forgejo Windows package validation**
  - Confirm Windows zip contains only `VideoTools.exe` and `README.md`.
- [ ] **Documentation naming hygiene**
  - Review new docs for personal names; use user report/dev report labels only.
- [X] **Installer dependency parity**
  - Ensure pip is installed on Linux/Windows and skip Go/pip when already present.
- [X] **Windows installer parse fix**
- [X] **Windows GCC PATH verification** - Align MSYS2 GCC detection with PATH checks.
  - Normalize PowerShell here-strings to prevent parser errors.
- [X] **Go auto-install in install.sh**
  - Skip prompting for Go and install automatically when missing.
- [X] **Windows install workflow split**
  - Route Windows to the dedicated installer to avoid mixed-shell prompts.
- [X] **Windows installer entrypoint**
  - Provide a PowerShell entrypoint and direct Windows users to it from install.sh.
- [X] **Windows DVDStyler mirror fallback**
  - Added Leak Technologies mirror URL and EXE fallback for DVD authoring tools.
- [X] **Windows script output alignment**
  - Match Linux-style headers and show build metadata in build.ps1 output.
- [X] **Windows build toolchain repair**
  - Refresh PATH and auto-repair missing MSYS2 GCC during builds when possible.
- [X] **Forgejo dev packaging**
  - Add Forgejo Actions workflow for Windows/Linux dev packages and artifacts.
- [X] **Forgejo dev release upload**
  - Upload dev artifacts to a Forgejo release when a token is provided.
- [ ] **Windows signing**
  - Provide production signing cert and wire it into Forgejo secrets for signed releases.
- [ ] **Forgejo runner labels**
  - Ensure runners are online with labels: ubuntu (Linux) and windows.
- [X] **Whisper model mirror fallback**
  - Prefer Leak Technologies mirror for whisper.cpp small model downloads.
- [X] **Git Bash handoff**
  - Keep Windows installs in the same Git Bash terminal using `winpty` when available.
- [X] **Windows root entrypoints**
- [X] **Linux script path fix**
  - Correct Linux build/install/run paths after scripts reorg.
  - Provide `install.bat` and `install.ps1` for PowerShell-first installs.
- [X] **Windows scripts entrypoints**
  - Provide `scripts/install.ps1` and `scripts/install.bat` to avoid Git Bash pop-ups.
- [X] **Windows setup launcher alignment**
  - Route `scripts/_internal/setup-windows.bat` through `scripts/install.bat`.
- [X] **Agent workflow rules**
  - Add `AGENTS.md` to enforce staging, commits, and documentation updates.
- [X] **Player fullscreen toggle**
  - Add fullscreen toggle to the Player module controls.
- [X] **Player EOS handling + metadata access**
  - Stop playback cleanly on EOS and expose duration/FPS from GStreamer.
- [X] **Main menu title cleanup**
  - Header shows "VideoTools" only; platform suffix moved to footer version label.
- [X] **Main menu palette refresh**
  - Restore a diverse, eye-friendly rainbow palette while keeping Convert constant.
- [X] **Main menu readability**
  - Increase tile label size and adjust low-contrast colors.
- [X] **Main menu contrast tuning**
  - Refine Audio, Rip, and Settings colors for legibility.
- [X] **Main menu layout cleanup**
  - Remove the scroll container so the main menu scales without scroll bars.
- [X] **Player silhouette placeholder**
  - Keep a stable player footprint before media loads.
- [X] **Main menu palette tuning**
  - Adjust audio/compare/subtitles colors for clearer separation.
- [X] **Main menu vibrancy pass**
  - Remove monochrome tiles outside Settings.
- [X] **Main menu bespoke hues**
  - Assign unique hue families to each module for maximum legibility.
- [X] **Locked tile hue preservation**
  - Keep disabled modules colored while subdued.
- [X] **Locked hue visibility**
  - Reduce stripe opacity and raise label brightness.
- [ ] **Main package layout cleanup**
  - Move root `package main` files into `cmd/videotools` when the build is stable.
- [ ] **Windows packaging prep**
  - [x] Draft MSIX + WinGet layout under `packaging/windows/`.
  - [x] Add GitHub Actions workflow to build MSIX and upload release artifacts.
  - [ ] Wire signing step (SignTool) once certificate is available.
  - [ ] Keep Windows installer aligned with MSIX/WinGet production path.

- [ ] **Git converter integration**
  - Move git_converter workflow into the main VT UI and retire legacy scripts.
- [ ] **Windows installer validation**
  - Test `scripts\_internal\install-deps-windows.ps1` MSYS2 flow and GStreamer MSI install on Windows 10/11.
  - Re-test GStreamer MSI download and local MSI override after variable fix.
  - Confirm mirror fallback works when the primary download returns HTML.
  - Verify winget fallback works when MSI downloads fail.
  - Confirm winget-first flow succeeds on clean Windows VM.
  - Verify DVDStyler winget fallback sets dvdauthor/mkisofs on PATH.
  - Validate MSI-first flow installs GStreamer and DVDStyler without winget.
  - Validate that GStreamer devel MSI is skipped unless build tools are selected.
  - Verify Whisper model prompt/download and mirror override on Windows.

## Documentation: Fix Structural Errors

**Priority:** High

- [X] **Audit All Docs for Broken Links:**
  - Systematically check all 46 `.md` files for internal links that point to non-existent files or sections.
  - Create placeholder stubs for missing documents that are essential (e.g., `CONTRIBUTING.md`) or remove the links if they are not.
  - This ensures a professional and navigable documentation experience.

## Critical Priority: dev24

### AUTHOR MODULE: CONTENT TYPES + GALLERIES + CHAPTER THUMBS

- [ ] **Content classification (Feature/Extra/Gallery)**
  - Feature: supports chapters + chapter menus
  - Extra: separate DVD titles; no chapters
  - Gallery: still-image slideshow title under Extras
  - Extras require subtype (behind_the_scenes, deleted_scenes, featurettes, interviews, trailers, commentary, other)
- [ ] **Cross-platform DVD authoring parity**
  - Ensure Windows and Linux use the same dvdauthor XML + ISO tool flags
  - Treat DVDStyler as a CLI tool bundle only (no GUI authoring dependency)
- [ ] **Chapter screenshot generation (Feature only)**
  - Auto-generate one still per chapter (default 2s offset)
  - Fallback to first valid frame on failure
  - Allow per-chapter override image
- [ ] **Menu structure rules**
  - Main: Play Feature, Chapters (if any), Extras (if extras/galleries)
  - Extras menu groups by subtype; galleries listed separately
- [ ] **UI layout guardrails**
  - Separate Feature / Extras / Galleries sections
  - Chapters disabled when content type is not Feature
- [ ] **Schema + config updates**
  - Add content_type per video, gallery assets list, chapter thumb config
  - Persist extras subtype and gallery behavior (auto-advance, loop)

### VIDEO PLAYER IMPLEMENTATION

**CRITICAL BLOCKER:** All advanced features (enhancement, trim, advanced filters) depend on stable player foundation.

#### Current Player Issues (from docs/PLAYER_PERFORMANCE_ISSUES.md):

1. **Separate A/V Processes** (lines 10184-10185 in main.go)
   - Video and audio run in completely separate FFmpeg processes
   - No synchronization mechanism between them
   - They will inevitably drift apart, causing A/V desync and stuttering
   - **FIX:** Implement unified FFmpeg process with multiplexed output

2. **Audio Buffer Too Small** (lines 8960, 9274 in main.go)
   - Currently 8192 samples = 170ms buffer
   - Modern systems need 100-200ms buffers for smooth playback
   - **FIX:** Increase to 16384-32768 samples (340-680ms)

3. **Volume Processing in Hot Path** (lines 9294-9318 in main.go)
   - Processes volume on EVERY audio sample in real-time
   - CPU-intensive and blocks audio read loop
   - **FIX:** Move volume processing to FFmpeg filters

4. **Video Frame Pacing Issues** (lines 9200-9203 in main.go)
   - time.Sleep() is not precise, cumulative timing errors
   - No correction mechanism if we fall behind
   - **FIX:** Implement adaptive timing with drift correction

5. **UI Thread Blocking** (lines 9207-9215 in main.go)
   - Frame updates queue up if UI thread is busy
   - No frame dropping mechanism
   - **FIX:** Implement proper frame buffer management

6. **No Frame-Accurate Seeking** (lines 10018-10028 in main.go)
   - Seeking kills and restarts both FFmpeg processes
   - 100-500ms gap during seek operations
   - No keyframe awareness
   - **FIX:** Implement frame-level seeking without process restart

#### Player Implementation Plan:

**Phase 1: Foundation (Week 1-2)**
- [ ] **Unified FFmpeg Architecture**
  - Single process with multiplexed A/V output using pipes
  - Master clock reference for synchronization
  - PTS-based drift correction mechanisms
  - Ring buffers for audio and video

- [ ] **Hardware Acceleration Integration**
  - Auto-detect available backends (CUDA, VA-API, VideoToolbox)
  - FFmpeg hardware acceleration through native flags
  - Fallback to software acceleration when hardware unavailable

- [ ] **Frame Extraction System**
  - Frame extraction without restarting playback
  - Keyframe detection and indexing
  - Frame buffer pooling to reduce GC pressure

**Phase 2: Core Features (Week 3-4)**
- [ ] **Frame-Accurate Seeking**
  - Seek to specific frames without restarts
  - Keyframe-aware seeking for performance
  - Frame extraction at seek points for preview

- [ ] **Chapter System Integration**
  - Port scene detection from Author module
  - Manual chapter support with keyframing
  - Chapter navigation (next/previous)
  - Chapter display in UI

- [ ] **Performance Optimization**
  - Adaptive frame timing with drift correction
  - Frame dropping when UI thread can't keep up
  - Memory pool management for frame buffers
  - CPU usage optimization

**Phase 3: Advanced Features (Week 5-6)**
- [ ] **Preview System**
  - Real-time frame extraction
  - Thumbnail generation from keyframes
  - Frame buffer caching for previews

- [ ] **Error Recovery**
  - Graceful failure handling
  - Resume capability after crashes
  - Smart fallback mechanisms

### ENHANCEMENT MODULE FOUNDATION

**DEPENDS ON PLAYER COMPLETION**

#### Current State:
- [X] Basic filters module with color correction, sharpening, transforms
- [X] Stylistic effects (8mm, 16mm, B&W Film, Silent Film, VHS, Webcam)
- [X] AI upscaling with Real-ESRGAN integration
- [X] Basic AI model management
- [ ] No content-aware processing
- [ ] No multi-pass enhancement pipeline
- [ ] No before/after preview system

#### Enhancement Module Plan:

**Phase 1: Architecture (Week 1-2 - POST PLAYER)**
- [ ] **Model Registry System**
  - Abstract AI model interface for easy extension
  - Dynamic model discovery and registration
  - Model requirements validation
  - Configuration management for different model types

- [ ] **Content Detection Pipeline**
  - Automatic content type detection (general/anime/film)
  - Quality assessment algorithms
  - Progressive vs interlaced detection
  - Artifact analysis (compression noise, film grain)

- [ ] **Unified Enhancement Workflow**
  - Combine Filters + Upscale into single module
  - Content-aware model selection logic
  - Multi-pass processing framework
  - Quality preservation controls

**Phase 2: Model Integration (Week 3-4)**
- [ ] **Open-Source AI Model Expansion**
  - BasicVSR integration (video-specific super-resolution)
  - RIFE models for frame interpolation
  - Real-CUGan for anime/cartoon enhancement
  - Model selection based on content type

- [ ] **Advanced Processing Features**
  - Sequential model application capabilities
  - Custom enhancement pipeline creation
  - Parameter fine-tuning for different models
  - Quality vs Speed presets

### TRIM MODULE ENHANCEMENT

**DEPENDS ON PLAYER COMPLETION**

#### Current State:
- [X] Basic planning completed
- [ ] No timeline interface
- [ ] No frame-accurate cutting
- [ ] No chapter integration from Author module

#### Trim Module Plan:

**Phase 1: Foundation (Week 1-2 - POST PLAYER)**
- [ ] **Timeline Interface**
  - Frame-accurate timeline visualization
  - Zoom capabilities for precise editing
  - Scrubbing with real-time preview
  - Time/frame dual display modes

- [ ] **Chapter Integration**
  - Import scene detection from Author module
  - Manual chapter marker creation
  - Chapter navigation controls
  - Visual chapter markers on timeline

- [ ] **Frame-Accurate Cutting**
  - Exact frame selection for in/out points
  - Preview before/after trim points
  - Multiple segment trimming support

**Phase 2: Advanced Features (Week 3-4)**
- [ ] **Smart Export System**
  - Lossless vs re-encode decision logic
  - Format preservation when possible
  - Quality-aware encoding settings
  - Batch trimming operations

### DOCUMENTATION UPDATES

- [X] **Create PLAYER_MODULE.md** - Comprehensive player architecture documentation
- [X] **Update MODULES.md** - Player and enhancement integration details
- [X] **Update ROADMAP.md** - Player-first development strategy
- [ ] **Create enhancement integration guide** - How modules work together
- [ ] **API documentation** - Player interface for module developers

## Future Enhancements (dev24+)

### AI Model Expansion
- [ ] **Diffusion-based models** - SeedVR2, SVFR integration
- [ ] **Advanced restoration** - Scratch repair, dust removal, color fading
- [ ] **Face enhancement** - GFPGAN integration for portrait content
- [ ] **Specialized models** - Content-specific models (sports, archival, etc.)

### Professional Features
- [ ] **Batch enhancement queue** - Process multiple videos with enhancement pipeline
- [ ] **Hardware optimization** - Multi-GPU support, memory management
- [ ] **Export system** - Professional format support (ProRes, DNxHD, etc.)
- [ ] **Plugin architecture** - Extensible system for community contributions

### Integration Improvements
- [ ] **Module communication** - Seamless data flow between modules
- [ ] **Unified settings** - Shared configuration across modules
- [ ] **Performance monitoring** - Resource usage tracking and optimization
- [ ] **Cross-platform testing** - Linux, Windows, macOS parity

## Technical Debt Addressed

### Player Architecture
- [X] Identified root causes of instability
- [X] Planned Go-based unified solution
- [X] Hardware acceleration strategy defined
- [X] Frame-accurate seeking approach designed

### Enhancement Strategy
- [X] Open-source model ecosystem researched
- [X] Scalable architecture designed
- [X] Content-aware processing planned
- [X] Future-proof model integration system

## Notes

- **Player stability is BLOCKER**: Cannot proceed with enhancement features until player is stable
- **Go implementation preferred**: Maintains single codebase, excellent testing ecosystem
- **Open-source focus**: No commercial dependencies, community-driven model ecosystem
- **Modular design**: Each enhancement system can be developed and tested independently
