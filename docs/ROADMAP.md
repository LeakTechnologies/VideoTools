# VideoTools Roadmap

This roadmap is intentionally lightweight. It captures the next few high-priority goals without locking the project into a rigid plan.

## How We Use This

- The roadmap is a short list, not a full backlog.
- Items can move between buckets as priorities change.
- We update this at the start of each dev cycle.

## Current State

- dev21 focused on stylistic filters and enhancement module planning.
- Filters module now includes decade-based authentic effects (8mm, 16mm, B&W Film, Silent Film, VHS, Webcam).
- Player stability identified as critical blocker for enhancement development.
- dev23 delivered UI cleanup (dropdown styling, settings panel polish, about/support layout).

## Now (dev24 focus)

- **Rock-solid video player implementation** - CRITICAL PRIORITY
  - Fix fundamental A/V synchronization issues
  - Implement frame-accurate seeking without restarts
  - Add hardware acceleration (CUDA/VA-API/VideoToolbox)
  - Integrate chapter detection from Author module
  - Build foundation for frame extraction and keyframing
  - Eliminate seeking glitches and desync issues

- **Enhancement module foundation** - DEPENDS ON PLAYER
  - Unified Filters + Upscale workflow
  - Content-type aware processing (general/anime/film)
  - Add blur control alongside sharpen/denoise
  - AI model management system (extensible for future models)
  - Multi-pass processing pipeline
  - Before/after preview system
  - Real-time enhancement feedback

- **Upscale workflow parity**
  - Replace Upscale output quality with Convert-style Bitrate Mode controls
  - Ensure FFmpeg-based upscale jobs report progress in queue
- **Authoring structure upgrade**
  - Feature/Extras/Gallery content types with subtype grouping
  - Chapter thumbnails auto-generated for Feature only
  - Galleries authored as still-image slideshows under Extras
  - Cross-platform DVD authoring parity (same dvdauthor XML + ISO flags)

## Next (dev25+)

- **Enhancement module completion** - DEPENDS ON PLAYER
  - Open-source AI model integration (BasicVSR, RIFE, RealCUGan)
  - Model registry system for easy addition of new models
  - Content-aware model selection
  - Advanced restoration (SVFR, SeedVR2, diffusion-based)
  - Quality-aware enhancement strategies

- **Trim module with timeline interface** - DEPENDS ON PLAYER
  - Frame-accurate trimming and cutting
  - Manual chapter support with keyframing
  - Visual timeline with chapter markers
  - Preview-based trimming with exact frame selection
  - Import chapter detection from Author module

- **Professional workflow integration**
  - Seamless module communication (Player ↔ Enhancement ↔ Trim)
  - Batch enhancement processing through queue
  - Cross-platform frame extraction
  - Hardware-accelerated enhancement pipeline

## Localization Implementation (Parallel Track)

- **Localization Infrastructure Foundation**
  - Core localization system with go-i18n integration
  - Language pack structure and key architecture
  - Canadian English (en-CA) as authoritative source
  - Fyne integration limits (system dialogs only)

- **Module-by-Module Localization**
  - Common UI elements (menus, dialogs, status messages)
  - Convert module localization (technical terminology)
  - Queue system localization (job management, progress)
  - Error messages and status indicators

- **Indigenous Language Support**
  - Inuktitut dual-script support (syllabics + romanized)
  - Cree syllabics integration
  - Human-review workflow and cultural validation
  - Community contribution guidelines

- **Quality Assurance Framework**
  - Pseudo-language testing (long strings, RTL simulation)
  - Automated translation completeness checking
  - UI layout expansion validation
  - Script rendering and font support testing

## Later

- **Advanced AI features**
  - AI-powered scene detection
  - Intelligent upscaling model selection
  - Temporal consistency algorithms
  - Custom model training framework
  - Cloud processing options

- **Module expansion**
  - Audio enhancement and restoration
  - Subtitle processing and burning
  - Multi-track management
  - Advanced metadata editing

- **Global Language Expansion**
  - European languages (German, Spanish, Italian)
  - Asian languages (Japanese, Chinese, Korean)
  - Right-to-left language support
  - Cultural adaptation for regional markets

## Versioning Note

We keep continuous dev numbering. After v0.1.1 release, the next dev tag becomes v0.1.1-dev22 (or whatever the next number is).

## Technical Debt and Architecture

### Player Module Critical Issues Identified

The current video player has fundamental architectural problems preventing stable playback:

1. **Separate A/V Processes** - No synchronization, guaranteed drift
2. **Command-Line Interface Limitations** - VLC/MPV controllers use basic CLI, not proper IPC
3. **Frame-Accurate Seeking** - Seeking restarts processes with full re-decoding
4. **No Frame Extraction** - Critical for enhancement and chapter functionality
5. **Poor Buffer Management** - Small audio buffers cause stuttering
6. **No Hardware Acceleration** - Software decoding causes high CPU usage

### Proposed Go-Based Solution

**Unified FFmpeg Player Architecture:**
- Single FFmpeg process with multiplexed A/V output
- Proper PTS-based synchronization with drift correction
- Frame buffer pooling and memory management
- Hardware acceleration through FFmpeg's native support
- Frame extraction via pipe without restarts

**Key Implementation Strategies:**
- Ring buffers for audio/video to eliminate stuttering
- Master clock reference for A/V sync
- Adaptive frame timing with drift correction
- Zero-copy frame operations where possible
- Hardware backend detection and utilization

This player enhancement is the foundation requirement for all advanced features including enhancement module and all other features that depend on reliable video playback.
