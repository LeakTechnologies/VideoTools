# Project Status

This document provides a high-level overview of the implementation status of the VideoTools project. It is intended to give users and developers a clear, at-a-glance understanding of what is complete, what is in progress, and what is planned.

## High-Level Summary

VideoTools is a modular application for video processing. While many features are planned, the current implementation is focused on a few core modules. The documentation often describes planned features, so please refer to this document for the ground truth.

## 🚨 Critical Known Issues

*   **Player Module:** The core player has fundamental A/V synchronization and frame-accurate seeking issues. This blocks the development of several planned features that depend on it (e.g., Trim, Filters). A major rework of the player is a critical priority.

## Module Implementation Status

### Core Modules

| Module  | Status                      | Notes                                                                  |
| :------ | :-------------------------- | :--------------------------------------------------------------------- |
| Player  | 🟡 **Partial / Buggy**      | Core playback works, but critical bugs block further development.      |
| Convert | ✅ **Implemented**            | Fully implemented with DVD encoding and professional validation.       |
| Merge   | 🔄 **Planned**                | Planned for a future release.                                          |
| Trim    | 🔄 **Planned**                | Planned. Depends on Player module fixes.                               |
| Filters | 🔄 **Planned**                | Planned. Depends on Player module fixes.                               |
| Upscale | 🟡 **Partial**              | AI-based upscaling (Real-ESRGAN) is integrated.                        |
| Audio   | 🔄 **Planned**                | Planned for a future release.                                          |
| Thumb   | 🔄 **Planned**                | Planned for a future release.                                          |
| Inspect | 🟡 **Partial**              | Basic metadata viewing is implemented. Advanced features are planned.  |
| Rip     | ✅ **Implemented**            | Ripping from `VIDEO_TS` folders and ISO images is implemented.         |
| Blu-ray | 🔄 **Planned**                | Comprehensive planning is complete. Implementation is for a future release. |

### Suggested Modules (All Planned)

The following modules have been suggested and are planned for future development, but are not yet implemented:

*   Subtitle Management
*   Advanced Stream Management
*   GIF Creation
*   Cropping Tools
*   Screenshot Capture
