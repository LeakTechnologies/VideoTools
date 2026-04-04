# VideoTools Roadmap

This roadmap is intentionally lightweight. It captures the next few high-priority goals without locking the project into a rigid plan.

## How We Use This

- The roadmap is a short list, not a full backlog.
- Items can move between buckets as priorities change.
- We update this at the start of each dev cycle.

## Current State (dev40)

- Core modules fully implemented: Convert, Merge, Filters, Audio, Thumb, Inspect, Compare, Rip, Author, Queue, Settings, Subtitles.
- DVD authoring engine is native Go (no dvdauthor/xorriso). Menu system complete (M1–M7).
- Localization active for en-CA, fr-CA, and Inuktitut (syllabics + Latin).
- CI green on both Linux and Windows with from-source FFmpeg static builds.
- Burn module UI wired; burn logic (IMAPI2/SG_IO) pending.

## Now (dev40 focus)

- **Burn module** — Implement disc burn logic via IMAPI2 (Windows) / SG_IO (Linux).
  See `docs/BURN_MODULE_DESIGN.md`.

- **Update-install guard** — Block `applyUpdate` while a queue or conversion job is active.
  See `AGENTS.md` for spec.

- **Convert UI cleanup** — Layout consistency and label clarity pass on `buildConvertView`.
  Tracked as issue #5.

- **Player stabilization** — Single-process A/V sync, frame-accurate seeking.
  Blocker for Trim module and Enhancement pipeline.

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

## Localization

The localization system is implemented. See `docs/localization-policy.md` for the full policy.

- en-CA and fr-CA are maintained and complete.
- Inuktitut (syllabics + Latin romanization) is present as machine-generated placeholders pending human review.
- All user-facing strings use `i18n.T().KeyName` — no hardcoded display strings.
- `CompletionPercent` helper available for gap detection.

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

We keep a continuous global `dev` counter and do not reset it per public version.

Examples:
- `v0.1.4-dev55`
- `v0.1.6-dev72`

Public releases use the base version only (for example `v0.1.6`), while dev builds keep increasing `-devN` across cycles.

## Public Version Bump Policy

We do not bump public version by dev-count depth alone. We bump when release gates are satisfied.

Minimum gate to move from `v0.1.1-devN` to `v0.1.2`:
- Windows and Linux package workflows are green on the release candidate commit.
- Full module smoke test pass using `docs/TESTING_MODULE_CHECKLIST.md`.
- No known P0/P1 regressions in conversion stability, queue reliability, or subtitle sync handling.
- Changelog section is complete and matches release scope.
- Deferred items are documented in `TODO.md` with explicit carry-over to the next cycle.

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
