# VideoTools Changelog

## v0.1.1-dev29 (March 2026)

### Build/CI
- **Workflow parsing** - Fixed Windows dev-packages YAML parsing for bundled dependency notes.
- **Go module imports** - Fixed module import paths in refactored files so vendor mode builds resolve internal packages.
- **Main menu compile fix** - Removed duplicate package declaration in main menu module file.
- **Convert compile fix** - Removed a duplicated aspect/scale block and restored custom-aspect declarations before use.
- **Windows compile fix** - Removed stale `go-qrcode` import from `main.go`.
- **Windows packaging fallback** - Added Tesseract language-data download fallback for `eng/fra/iku` in bundled builds.
- **GStreamer packaging policy** - Bundled builds treat GStreamer as optional and continue when it is unavailable.
- **Whisper packaging resilience** - Bundled workflows now try multiple whisper model sources and continue if download fails.
- **Linux bundled zip fix** - Added `zip` to Linux CI build dependencies for bundled artifact creation.
- **Dev packaging policy** - Dev channel builds now skip bundled package generation to keep nightly/pre-release runs stable.
- **Bundled artifact retirement** - Dev-packages workflow now publishes standard VT packages only (no bundled artifacts).
- **Main menu layout fix** - Module tiles now use wrapping bounds to prevent over-wide window expansion.
- **Release asset cleanup fix** - Publish workflow now reliably removes old assets before uploading new artifacts.
- **Publish endpoint fix** - Corrected Forgejo asset delete endpoint to avoid 404 failures during release publish.

## v0.1.1-dev28 (February 2026)

### Windows
- **First-run FFmpeg bootstrap** - Added an in-app Windows prompt that installs FFmpeg/FFprobe into `%LOCALAPPDATA%\VideoTools\bin` when missing.
- **Module lock recovery** - Main menu now offers a direct fix path on clean Windows machines instead of leaving users in a settings-only workflow.
- **App-local FFmpeg discovery** - Platform detection now checks `%LOCALAPPDATA%\VideoTools\bin` before PATH/common locations, so bootstrap installs persist across launches.

### Cross-Platform
- **Settings dependency ordering** - Dependencies in Settings are now listed with core requirements first, then alphabetically.
- **Settings FFmpeg actions** - FFmpeg install actions are available in Settings on supported platforms (Windows app-local install; Linux package-manager actions).
- **Convert UI text safety** - Replaced mojibake-prone Unicode/emoji labels in the Convert workflow with ASCII-safe text labels.
- **About page cleanup** - Removed the Bitcoin address from the About/Support dialog.
- **Snippet AV1 fallback** - Snippet generation now falls back when `libsvtav1` is unavailable.
- **Adaptive scrolling** - Settings and other long panels use adaptive scroll speed for smoother navigation across screen sizes.
- **Settings tab scrolling** - Each Settings tab now scrolls independently so the header stays in view.
- **Master settings** - Preferences now surface global hardware acceleration with auto-detect plus module visibility toggles.
- **Aspect ratio handling** - Source aspect now honors display aspect ratio metadata and adds a 17:9 target option.
- **Aspect ratio UI** - The Source aspect option now shows the detected source aspect ratio.
- **Aspect ratio logging** - Conversion logs now include source/target aspect details and ignore stale auto-crop values when auto-crop is disabled.
- **Custom aspect input** - Added a Custom... option for cinema/ultrawide ratios without cluttering the dropdown.
- **Aspect/scale alignment** - Aspect conversion now uses target resolution to avoid odd output sizes.
- **Window resize stability** - Module switches no longer auto-resize the window.
- **UI text cleanup** - Removed garbled characters from UI labels and prompts.
- **Conversion stability** - Conversion workers now catch internal panics and surface a failure dialog instead of closing the UI.
- **Conversion recovery notice** - The app now records active conversions and shows a notice on next launch if one was running.
- **Main menu refactor** - Main menu builder and refresh helpers moved into a dedicated module file.
- **About/Support refactor** - About/Support dialog moved into a dedicated module file.
- **Dependencies dialog refactor** - Missing dependencies dialog moved into a dedicated module file.
- **Queue view refactor** - Queue view builders and refresh logic moved into a dedicated module file.
- **Dev packages workflow** - Fixed YAML parsing in bundled deps note generation.
- **Module imports** - Fixed main menu and queue module imports to match module path.
- **Main menu module** - Fixed duplicate package declaration causing Windows builds to fail.
- **Subtitle ripping** - Subtitles can be extracted from embedded tracks (lossless or OCR/SRT/ASS for text) and re-embedded without sync drift.
- **Subtitle OCR** - Image-based DVD/BD subtitle tracks can be OCR'd with Tesseract into SRT or ASS.
- **Subtitle OCR cleanup** - OCR output is normalized and consecutive duplicate cues are merged for cleaner timing.
- **Language options** - Preferences now focus on Canadian English, Canadian French, and Inuktitut.
- **Bundled packages** - Added bundled builds with FFmpeg, Tesseract, and GStreamer for Windows and Linux.
- **Bundled whisper model** - Bundled packages now include the whisper.cpp small model and enforce required dependency payloads.

## v0.1.1-dev27 (February 2026)

### Maintenance
- **.gitignore updates** - Excluded Windows build artifacts and agent working directory.
- **Forgejo Windows outputs** - Emit `GITHUB_OUTPUT` as UTF-8 (no BOM) with PowerShell-compatible append to prevent host-runner post-step failures.
- **Forgejo Windows GUI build** - Package the Windows exe with `-H windowsgui` to avoid console windows.
- **Forgejo Windows package contents** - Limit zip contents to `VideoTools.exe` and `README.md`.
- **Forgejo Windows artifact layout** - Write zip directly to `dist/windows/` and drop build metadata files.
- **Forgejo Windows artifact upload** - Upload only the zip file, excluding diagnostics and folders.
- **Forgejo dev release workflow** - Build Linux/Windows artifacts in the same workflow run before publishing releases.
- **Forgejo release assets** - Upload only Linux AppImage/zip and Windows zip (skip build metadata files).
- **Forgejo workflow cleanup** - Remove redundant Windows packaging workflow.
- **Forgejo workflow cleanup** - Remove redundant test trigger workflow.
- **Forgejo release assets** - Delete existing assets with the same name before upload to avoid duplicates.
- **Forgejo release assets** - Purge existing assets before upload to avoid duplicates.
- **Forgejo release assets** - Fail publish step on unauthorized asset deletion/upload.
- **Forgejo mirror** - Use built-in push mirror settings for Codeberg.
- **Forgejo dev release notes** - Use a nightly build release note body.
- **Forgejo workflow cleanup** - Remove redundant Linux packaging workflow.

## v0.1.1-dev26 (January 2026)

### Infrastructure
- **Mirror hosting** - Created lt_mirror repository on git.leaktechnologies.dev for downloads when source sites block bots (GStreamer, DVDStyler, Whisper, FFmpeg)
- **Forgejo CI/CD** - Self-hosted runner setup, CI workflows, artifact versioning, optional EXE signing

### Windows Build System
- **Installer** - Switched from Scoop to Chocolatey, added MSYS2, dependency checking with early exit, progress bars, verification
- **Build scripts** - Console popup suppression, icon embedding, windowsgui flag, Go module caching, Unicode fixes

### Documentation
- Added Forgejo runner and Windows service setup docs

## v0.1.0-dev24 (January 2026)

### 🎨 Main Menu Palette
- **Rainbow+ palette refresh** - restored diverse, eye-friendly module colors with improved readability
- **Convert color preserved** - Convert remains the visual anchor across the UI
- **Larger tile labels** - main menu button text is larger for accessibility
- **Contrast tuning** - audio/rip/settings colors adjusted for clarity
- **Scrollbar removed** - main menu now scales without a scroll bar
- **Module silhouette** - player area keeps a steady footprint before media loads
- **Bespoke hues** - each module now has its own distinct color identity
- **Locked state clarity** - disabled modules keep their hue with subdued brightness
- **Locked hue visibility** - reduced stripe opacity and raised label brightness

## v0.1.0-dev23 (January 2026)

### 🎉 UI Cleanup
- **Colored select refinement** - one-click open, left accent bar, rounded corners, larger labels
- **Unified input styling** - settings panel backgrounds match dropdown tone
- **Convert panel polish** - Auto-crop and Interlacing actions match panel styling

### 🧩 About / Support
- **Mockup-aligned layout** - title row, VT + LT logos on the right, Logs Folder action
- **Support placeholder** - “Support coming soon” until donation details are available

### 🐛 Fixes
- **Audio module crash** - guarded initial quality selection to avoid nil entry panic

## v0.1.0-dev22 (January 2026)

### 🎉 Major Features

#### Automatic GPU Detection for Hardware Encoding
- **Auto-detect GPU vendor** (NVIDIA/AMD/Intel) via system info detection
- **Automatic hardware encoder selection** when hardware acceleration set to "auto"
- **Resolves to appropriate encoder**: nvenc for NVIDIA, amf for AMD, qsv for Intel
- **Fallback to software encoding** if no compatible GPU detected
- **Cross-platform detection**: nvidia-smi, lspci, wmic, system_profiler

#### SVT-AV1 Encoding Performance
- **Proper AV1 codec support** with hardware (av1_nvenc, av1_qsv, av1_amf) and software (libsvtav1) encoders
- **SVT-AV1 speed preset mapping** (0-13 scale) for encoder performance tuning
- **Prevents 80+ hour encodes** by applying appropriate speed presets
- **ultrafast preset** → ~10-15 hours instead of 80+ hours for typical 1080p encodes
- **CRF quality control** for AV1 encoding

#### UI/UX Improvements
- **Fluid UI splitter** - removed rigid minimum size constraints for smoother resizing
- **Format selector widget** - proper dropdown for container format selection
- **Semantic color system** - ColoredSelect ONLY for format/codec navigation (not rainbow everywhere)
- **Format colors**: MKV=teal, MP4=blue, MOV=indigo
- **Codec colors**: AV1=emerald, H.265=lime, H.264=sky, AAC=purple, Opus=violet

### 🔧 Technical Improvements

#### Hardware Encoding
- **GPUVendor() method** in sysinfo package for GPU vendor identification
- **Automatic encoder resolution** based on detected hardware
- **Better hardware encoder fallback** logic

#### Platform Support
- **Windows FFmpeg popup suppression** - proper build tags on exec_windows.go/exec_unix.go
- **Platform-specific command creation** with CREATE_NO_WINDOW flag on Windows
- **Fixed process creation attributes** for silent FFmpeg execution on Windows

#### Code Quality
- **Queue system type consistency** - standardized JobType constants (JobTypeFilter)
- **Fixed forward declarations** for updateDVDOptions and buildCommandPreview
- **Removed incomplete formatBackground** section with TODO for future implementation
- **Git remote correction** - restored git.leaktechnologies.dev repository URL

### 🐛 Bug Fixes

#### Encoding
- **Fixed AV1 forced H.264 conversion** - restored proper AV1 encoding support
- **Added missing preset mapping** for libsvtav1 encoder
- **Proper CRF handling** for AV1 codec

#### UI
- **Fixed dropdown reversion** - removed rainbow colors from non-codec dropdowns
- **Fixed splitter stiffness** - metadata and labeled panels now resize fluidly
- **Fixed formatContainer** missing widget definition

#### Build
- **Resolved all compilation errors** from previous session
- **Fixed syntax errors** in formatBackground section
- **Fixed JobType constant naming** (JobTypeFilter vs JobTypeFilters)
- **Moved WIP files** out of build path (execute_edit_job.go.wip)

#### Dependencies
- **Upscale module accessibility** - changed from requiring realesrgan to optional
- **FFmpeg-only scaling** now works without AI upscaler dependencies

### 📝 Coordination & Planning

#### Agent Coordination
- **Updated WORKING_ON.md** with coordination request for opencode
- **Analyzed uncommitted job editing feature** (edit.go, command_editor.go)
- **Documented integration gaps** and presented 3 options for dev23
- **Removed Gemini from active agent rotation**

### 🚧 Work in Progress (Deferred to Dev23)

#### Job Editing Feature (opencode)
- **Core logic complete** - edit.go (363 lines), command_editor.go (352 lines)
- **Compiles successfully** but missing integration
- **Needs**: main.go hookups, UI buttons, end-to-end testing
- **Status**: Held for proper integration in dev23

### 🔄 Breaking Changes

None - this is a bug-fix and enhancement release.

### ⚠️ Known Issues

- **Windows dropdown UI differences** - investigating appearance differences on Windows vs Linux (deferred to dev23)
- **Benchmark system** needs improvements (deferred to dev23)

### 📊 Development Stats

**Commits This Release**: 3 main commits
- feat: add automatic GPU detection for hardware encoding
- fix: resolve build errors and complete dev22 fixes
- docs: update WORKING_ON coordination file

**Files Modified**: 8 files
- FyneApp.toml (version bump)
- main.go (GPU detection, AV1 presets, UI fixes)
- internal/sysinfo/sysinfo.go (GPUVendor method)
- internal/queue/queue.go (JobType fixes)
- internal/utils/exec_windows.go (build tags)
- internal/utils/exec_unix.go (build tags)
- settings_module.go (Upscale dependencies)
- WORKING_ON.md (coordination)

---

## v0.1.0-dev14 (December 2025)

### 🎉 Major Features

#### Windows Compatibility Implementation
- **Cross-platform build system** with MinGW-w64 support
- **Platform detection system** (`platform.go`) for OS-specific configuration
- **FFmpeg path abstraction** supporting bundled and system installations
- **Hardware encoder detection** for Windows (NVENC, QSV, AMF)
- **Windows-specific process handling** and path validation
- **Cross-compilation script** (`scripts/windows/build-windows.sh`)

#### Professional Installation System
- **One-command installer** (`scripts/linux/install.sh`) with guided wizard
- **Automatic shell detection** (bash/zsh) and configuration
- **System-wide vs user-local installation** options
- **Convenience aliases** (`VideoTools`, `VideoToolsRebuild`, `VideoToolsClean`)
- **Comprehensive installation guide** (`INSTALLATION.md`)

#### DVD Auto-Resolution Enhancement
- **Automatic resolution setting** when selecting DVD formats
- **NTSC/PAL auto-configuration** (720×480 @ 29.97fps, 720×576 @ 25fps)
- **Simplified user workflow** - one click instead of three
- **Standards compliance** ensured automatically

#### Queue System Improvements
- **Enhanced thread-safety** with improved mutex locking
- **New queue control methods**: `PauseAll()`, `ResumeAll()`, `MoveUp()`, `MoveDown()`
- **Better job reordering** with up/down arrow controls
- **Improved status tracking** for running/paused/completed jobs
- **Batch operations** for queue management

### 🔧 Technical Improvements

#### Code Organization
- **Platform abstraction layer** for cross-platform compatibility
- **FFmpeg path variables** in internal packages
- **Improved error handling** for Windows-specific scenarios
- **Better process termination** handling across platforms

#### Build System
- **Cross-compilation support** from Linux to Windows
- **Optimized build flags** for Windows GUI applications
- **Dependency management** for cross-platform builds
- **Distribution packaging** for Windows releases

#### Documentation
- **Windows compatibility guide** (`WINDOWS_COMPATIBILITY.md`)
- **Implementation documentation** (`DEV14_WINDOWS_IMPLEMENTATION.md`)
- **Updated installation instructions** with platform-specific notes
- **Enhanced troubleshooting guides** for Windows users

### 🐛 Bug Fixes

#### Queue System
- **Fixed thread-safety issues** in queue operations
- **Resolved callback deadlocks** with goroutine execution
- **Improved error handling** for job state transitions
- **Better memory management** for long-running queues

#### Platform Compatibility
- **Fixed path separator handling** for cross-platform file operations
- **Resolved drive letter issues** on Windows systems
- **Improved UNC path support** for network locations
- **Better temp directory handling** across platforms

### 📚 Documentation Updates

#### New Documentation
- `INSTALLATION.md` - Comprehensive installation guide (360 lines)
- `WINDOWS_COMPATIBILITY.md` - Windows support planning (609 lines)
- `DEV14_WINDOWS_IMPLEMENTATION.md` - Implementation summary (325 lines)

#### Updated Documentation
- `README.md` - Updated Quick Start for install.sh
- `BUILD_AND_RUN.md` - Added Windows build instructions
- `docs/README.md` - Updated module implementation status
- `TODO.md` - Reorganized for dev15 planning

### 🔄 Breaking Changes

#### Build Process
- **New build requirement**: MinGW-w64 for Windows cross-compilation
- **Updated build scripts** with platform detection
- **Changed FFmpeg path handling** in internal packages

#### Configuration
- **Platform-specific configuration** now required
- **New environment variables** for FFmpeg paths
- **Updated hardware encoder detection** system

### 🚀 Performance Improvements

#### Build Performance
- **Faster incremental builds** with better dependency management
- **Optimized cross-compilation** with proper toolchain usage
- **Reduced binary size** with improved build flags

#### Runtime Performance
- **Better process management** on Windows
- **Improved queue performance** with optimized locking
- **Enhanced memory usage** for large file operations

### 🎯 Platform Support

#### Windows (New)
- ✅ Windows 10 support
- ✅ Windows 11 support  
- ✅ Cross-compilation from Linux
- ✅ Hardware acceleration (NVENC, QSV, AMF)
- ✅ Windows-specific file handling

#### Linux (Enhanced)
- ✅ Improved hardware encoder detection
- ✅ Better Wayland support
- ✅ Enhanced process management

#### Linux (Enhanced)
- ✅ Continued support with native builds
- ✅ Hardware acceleration (VAAPI, NVENC, QSV)
- ✅ Cross-platform compatibility

### 📊 Statistics

#### Code Changes
- **New files**: 3 (platform.go, build-windows.sh, install.sh)
- **Updated files**: 15+ across codebase
- **Documentation**: 1,300+ lines added/updated
- **Platform support**: 2 platforms (Linux, Windows)

#### Features
- **New major features**: 4 (Windows support, installer, auto-resolution, queue improvements)
- **Enhanced features**: 6 (build system, documentation, queue, DVD encoding)
- **Bug fixes**: 8+ across queue, platform, and build systems

### 🔮 Next Steps (dev15 Planning)

#### Immediate Priorities
- Windows environment testing and validation
- NSIS installer creation for Windows
- Performance optimization for large files
- UI/UX refinements and polish

#### Module Development
- Merge module implementation
- Trim module with timeline interface
- Filters module with real-time preview
- Advanced Convert features (2-pass, presets)

#### Platform Enhancements
- Native Windows builds
- Linux AppImage bundle creation
- Linux package distribution (.deb, .rpm)
- Auto-update mechanism

---

## v0.1.0-dev13 (November 2025)

### 🎉 Major Features

#### DVD Encoding System
- **Complete DVD-NTSC implementation** with professional specifications
- **Multi-region support** (NTSC, PAL, SECAM) with region-free output
- **Comprehensive validation system** with actionable warnings
- **FFmpeg command generation** for DVD-compliant output
- **Professional compatibility** (DVDStyler, PS2, standalone players)

#### Code Modularization
- **Extracted 1,500+ lines** from main.go into organized packages
- **New package structure**: `internal/convert/`, `internal/app/`
- **Type-safe APIs** with exported functions and structs
- **Independent testing capability** for modular components
- **Professional code organization** following Go best practices

#### Queue System Integration
- **Production-ready queue system** with 24 public methods
- **Thread-safe operations** with proper synchronization
- **Job persistence** with JSON serialization
- **Real-time progress tracking** and status management
- **Batch processing capabilities** with priority handling

### 📚 Documentation

#### New Comprehensive Guides
- `DVD_IMPLEMENTATION_SUMMARY.md` (432 lines) - Complete DVD system reference
- `QUEUE_SYSTEM_GUIDE.md` (540 lines) - Full queue system documentation  
- `INTEGRATION_GUIDE.md` (546 lines) - Step-by-step integration instructions
- `COMPLETION_SUMMARY.md` (548 lines) - Project completion overview

#### Updated Documentation
- `README.md` - Updated with DVD features and installation
- `MODULES.md` - Enhanced module descriptions and coverage
- `TODO.md` - Reorganized for dev14 planning

### 📚 Documentation Updates

#### New Documentation Added
- Enhanced `TODO.md` with Lossless-Cut inspired trim module specifications
- Updated `MODULES.md` with detailed trim module implementation plan
- Enhanced `docs/README.md` with VT_Player integration links

#### Documentation Enhancements
- **Trim Module Specifications** - Detailed Lossless-Cut inspired design
- **VT_Player Integration Notes** - Cross-project component reuse
- **Implementation Roadmap** - Clear development phases and priorities

---

*For detailed technical information, see the individual implementation documents in the `docs/` directory.*



