# Rip Module

Extract and convert content from DVD folder structures and disc images.

## Overview

The Rip module focuses on offline extraction from VIDEO_TS folders or DVD ISO images. It is designed to be fast and lossless by default, with optional H.264 transcodes when you want smaller files. All processing happens locally.

## Current Capabilities (dev20+)

### Supported Sources
- VIDEO_TS folders
- ISO images (requires `xorriso` or `bsdtar` to extract)

### Output Modes
- Lossless DVD -> MKV (stream copy, default)
- H.264 MKV (transcode)
- H.264 MP4 (transcode)

### Behavior Notes
- Uses a queue job with progress and logs.
- No online lookups or network calls.
- ISO extraction is performed to a temporary working folder before FFmpeg runs.
- Default output naming is based on the source name.

## Not Yet Implemented
- Direct ripping from physical drives (DVD/Blu-ray)
- Multi-title selection from ISO contents
- Auto metadata lookup
- Subtitle/audio track selection UI

## Usage

1. Open the Rip module.
2. Drag a VIDEO_TS folder or an ISO into the drop area.
3. Choose the output mode (lossless MKV or H.264 MKV/MP4).
4. Start the rip job and monitor the log/progress.

## Dependencies

- `ffmpeg`
- `xorriso` or `bsdtar` for ISO extraction

## Example FFmpeg Flow (conceptual)

- VIDEO_TS: concatenate VOBs then stream copy to MKV.
- ISO: extract VIDEO_TS from the ISO, then follow the same flow.

