# Project Status

This document provides a high-level overview of the implementation status of the VideoTools project. It is intended to give users and developers a clear, at-a-glance understanding of what is complete, what is in progress, and what is planned.

## High-Level Summary

VideoTools is a modular application for video processing. While many features are planned, the current implementation is focused on a few core modules. The documentation often describes planned features, so please refer to this document for the ground truth.

## 🚨 Critical Known Issues

- **Player performance and sync issues**
  - Video/audio playback uses separate processes and can drift.
  - Seeking and frame pacing are not yet frame-accurate.
  - See `PLAYER_PERFORMANCE_ISSUES.md` for details and plan.

## Module Implementation Status

### Core Modules

| Module  | Status                      | Notes                                                                  |
| :------ | :-------------------------- | :--------------------------------------------------------------------- |
| Player  | 🟡 **Implemented (Issues)**   | Core playback works but has known performance and sync issues.         |
| Convert | ✅ **Implemented**            | Fully implemented with DVD encoding and validation.                    |
| Merge   | ✅ **Implemented**            | Clip merge with format presets and queue support.                      |
| Trim    | 🔄 **Planned**                | Planned for a future release (depends on player work).                 |
| Filters | ✅ **Implemented**            | FFmpeg filter chains and stylistic presets.                            |
| Upscale | 🟡 **Partial**              | Traditional scaling + Real-ESRGAN (ncnn) integration.                  |
| Audio   | ✅ **Implemented**            | Audio extraction with presets and batch mode.                          |
| Thumb   | ✅ **Implemented**            | Thumbnail/contact sheet generation.                                    |
| Inspect | ✅ **Implemented**            | Metadata viewing with interlace detection; advanced features planned.  |
| Compare | ✅ **Implemented**            | Side-by-side playback comparison.                                      |
| Rip     | ✅ **Implemented**            | Ripping from `VIDEO_TS` folders and ISO images is implemented.         |
| Author  | ✅ **Implemented**            | DVD authoring with menu templates.                                     |
| Queue   | ✅ **Implemented**            | Job queue management with history.                                     |
| Settings| ✅ **Implemented**            | Preferences for language and hardware acceleration.                    |
| Blu-ray | 🔄 **Planned**                | Comprehensive planning is complete. Implementation is for a future release. |

### Suggested Modules (All Planned)

The following modules have been suggested and are planned for future development, but are not yet implemented:

*   Subtitle Management
*   Advanced Stream Management
*   GIF Creation
*   Cropping Tools
*   Screenshot Capture
