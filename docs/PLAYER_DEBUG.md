# Native Media Player Debug Notes

## Issues Identified

### 1. GStreamer Fallback (RESOLVED)
- **Problem**: Linux builds were falling back to GStreamer player instead of native FFmpeg engine
- **Root Cause**: CI build script used `GSTREAMER_TAG` which added `-tags gstreamer` but NOT `-tags native_media`
- **Fix**: Removed all GStreamer code from the codebase (commit `078fc846`)

### 2. SMPTE Bars Not Showing (RESOLVED)
- **Problem**: Player module showed old placeholder UI instead of SMPTE bars when empty
- **Root Cause**: Player view returned `nil` from `OnBuildVideoPane` when no video loaded
- **Fix**: Always call `buildVideoPane` with nil src to get SMPTE bars (commit `9f006ded`, `c77a1124`)

### 3. Hard Crash When Loading Video (ONGOING)
- **Problem**: App crashes hard (SIGSEGV) when loading video in native player
- **Symptoms**: 
  - Log shows "GrabFrame: sending packet to video codec"
  - Log shows "GrabFrame: packet sent OK, calling avcodec_receive_frame"
  - Then crash - no further log output
- **Attempted Fixes**:
  1. Added panic recovery to `GrabFrame` - didn't catch (CGo crash)
  2. Added panic recovery to `toRGBA` - didn't catch
  3. Added panic recovery to `Load` - didn't catch
  4. Skipped initial `GrabFrame` call - still crashes
  5. Added debug log after load completes - log never appears

### 4. Linux CI Build Failing (RESOLVED)
- **Problem**: Linux build failed with `cannot find -ldrm`
- **Root Cause**: FFmpeg configured with VAAPI/VDPAU hardware acceleration requiring libdrm
- **Fix**: Added `libdrm-dev` to Linux build dependencies in CI

### 5. Linux Update Fails (RESOLVED)
- **Problem**: Update failed with "invalid cross-device link" 
- **Root Cause**: Temp file in /tmp, binary in ~/Videos - rename fails across devices
- **Fix**: Use helper script that copies instead of renames (commit `b0b06684`)

## Current State (dev42)

- SMPTE bars display correctly when no video loaded
- Native media engine initializes correctly
- FFmpeg DLLs download and load
- Video file probes successfully (h264, aac, av1 tested)
- Codecs open (video and audio)
- Audio player creates and plays
- Crash happens AFTER all initialization, BEFORE our debug log appears

## Suspected Issue

The crash appears to be in FFmpeg's C code during:
- Frame decoding (avcodec_receive_frame)
- OR demuxer loop 
- OR smooth scrubbing initialization

The crash is NOT caught by Go panic recovery - it's a C-level SIGSEGV.

## Videos Tested

| Video | Format | Codec | Result |
|-------|--------|-------|--------|
| ECW Terry Funk vs Cactus Jack.mp4 | MOV | h264 | Crash |
| 2 Minutes.mp4 | MP4 | AV1 | Crash |

## Next Steps

1. Try loading video WITHOUT starting engine (just open file, don't Start/Seek)
2. Try loading video without audio
3. Run under GDB to get C stack trace
4. Check if specific FFmpeg build (BtbN) has known issues

## Relevant Files

- `internal/media/engine.go` - FFmpeg engine, frame decoding
- `internal/ui/inline_player.go` - Player API, load sequence  
- `native_media.go` - Platform-specific player initialization
- `.forgejo/workflows/dev-packages.yml` - CI build configuration