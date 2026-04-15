# Player Debug Status

Rolling checklist of known issues, fixes applied, and remaining work for the native media player.

## Fixed (dev42 cycle)

- [x] **D3D11VA crash** — `get_format` callback now accepts `AV_PIX_FMT_D3D11VA_VLD` (enum mismatch with `AV_PIX_FMT_D3D11`)
- [x] **AV_NOPTS_VALUE frame crash** — Skip frames with invalid PTS (`AV_NOPTS_VALUE` or negative) and zero dimensions in both `GrabFrame` and `NextFrame`
- [x] **GrabFrame deadlock** — Skip path tried to re-lock `videoCodecMu` when already held; removed the redundant `Lock()` call
- [x] **A/V clock double-speed** — `NextFrame` was multiplying PTS by speed (`pts * e.speed`), but `MasterClock.GetTime()` already accounts for speed. Removed the double-application; wired `clock.SetSpeed()` into `Engine.SetSpeed()`
- [x] **Pause spin-loop** — `NextFrame` busy-looped at 100% CPU when paused; added 50ms sleep
- [x] **Engine.Close() race** — Set `running=false` before closing `stop` channel to prevent double-close panic
- [x] **Video drop routing** — Inner `Droppable` widgets called `loadMultipleVideos` directly (switches to Convert). Now all route through `handleDrop` which respects the active module
- [x] **GStreamer removal** — Deleted all GStreamer code (~2000 lines). Native media engine is the only player on both platforms. Build scripts and CI updated.
- [x] **HW frame buffer race** — `retrieveHWFrame()` was sharing `e.rgbaFrame`/`e.rgbaBuffer` with `toRGBA()`, creating a potential race when both code paths run. Now uses dedicated `hwRgbaFrame`/`hwRgbaBuffer`

## Known Issues

- [ ] **D3D11VA hard crash after several frames** — H.264 files with D3D11VA decode crash silently (segfault) after decoding 5+ frames. Last log: `NextFrame #5: returning frame pts=0.167 hw_frames_ctx=true`. The crash happens in CGo, likely in `av_hwframe_transfer_data` or `sws_scale` during HW→SW conversion. The dedicated buffer fix may resolve this; needs testing.
- [ ] **Scrub goroutine races** — `SmoothScrubbing.predecodeFrom`/`predecodeAhead` use `e.videoCodecCtx` under `videoCodecMu`, but `NextFrame` unlocks `videoCodecMu` before doing HW frame transfer. If scrub starts during that window, both decode into the same codec context. Needs: either hold `videoCodecMu` through HW transfer, or pause scrub during playback.
- [ ] **SW decode crash after 5 frames** — AV1 and H.264 software decode also crash after ~5 frames. Log shows `NextFrame #5: returning frame pts=0.167 hw_frames_ctx=false` (SW, not HW). Added panic recovery to NextFrame and predecodeAhead.

## Future Features

- **Dynamic test patterns** — Fully dynamically generated test patterns for monitor calibration (SMPTE, grayscale, color bars, etc.)
- **Test module** — Replace Player module with comprehensive test card generator featuring:
  - SMPTE color bars (various resolutions)
  - BBC Test Card F and other historical test cards
  - Grayscale/pluge patterns
  - Geometric patterns (circles, gradients, etc.)
  - Audio test tones

## Current Status

- ✅ SMPTE bars display correctly when no video loaded
- ✅ First frame displays (GrabFrame with 4s timeout works)
- ✅ Frames #1-5 decode and display successfully
- ⚠️ Crash on frame #5-6 in NextFrame/predecodeAhead (CGo segfault)
- [ ] **Speed changes don't affect audio** — `AudioPlayer.Read()` doesn't resample audio tempo/pitch when speed changes
- [ ] **`GrabFrame` timeout too short** — Default 8s may not be enough for some files; should be configurable

## Architecture Notes

### Playback Flow

```
Load() → engine.Open() → engine.Start() → engine.Seek(0) → engine.GrabFrame(8s) → engine.Pause()
Play()  → engine.Resume() → playbackLoop() → engine.NextFrame() (loop)
                                              ↓
                                     avcodec_send_packet
                                     avcodec_receive_frame
                                     (skip AV_NOPTS_VALUE frames)
                                     clock.WaitForPTS(pts) or clock.SetTime(pts)
                                     clock.SyncVideo(pts) → delay
                                     retrieveHWFrame() or toRGBA()
                                     return img → SetFrame(img)
```

### Key Mutexes

| Mutex | Protects | Held During |
|-------|----------|-------------|
| `e.mu` | `running`, `paused`, `speed`, etc. | State checks |
| `e.videoCodecMu` | `videoCodecCtx` send/receive | `avcodec_send_packet` + `avcodec_receive_frame` in NextFrame, GrabFrame, Scrub |
| `e.formatMu` | `formatCtx` read/seek | `av_read_frame` in demuxerLoop, `avformat_seek_file` in Seek |

### HW Decode Path

1. `avcodec_receive_frame` produces frame with `hw_frames_ctx != nil`
2. `retrieveHWFrame()` calls `av_hwframe_transfer_data` to download GPU surface to CPU
3. Creates per-frame `sws_getContext` for format conversion (NV12 → RGBA)
4. `sws_scale` into dedicated `hwRgbaFrame`/`hwRgbaBuffer` (separate from SW buffers)
5. Copies result into `image.RGBA`

### Clock System

- `MasterClock` manages PTS timing with speed multiplier
- `WaitForPTS(target)` blocks until master clock reaches target
- `SyncVideo(pts)` returns delay or -1 (frame late, drop it)
- Audio clock sync: `AudioPlayer.Read()` sets `clock.SetTime(pts)` on each audio frame

### Videos Tested

| Video | Format | Codec | HW | Result |
|-------|--------|-------|----|--------|
| Herzog.avi | AVI | MPEG4 | No (SW) | Loads, plays 2 frames, then crashes (older builds); now plays |
| Horny Sports.avi | AVI | MPEG4 | No (SW) | Loads, plays ~2 frames, freezes (older); now works |
| Meng vs Sting.mp4 | MP4 | H.264 | Yes (D3D11VA) | Loads, plays 5 frames, hard crash (CGo segfault) |
| ECW Terry Funk vs Cactus Jack.mp4 | MOV | h264 | No | Crash after frame 5 |
| 2 Minutes.mp4 | MP4 | AV1 | No | Crash after frame 5 |

### Relevant Files

- `internal/media/engine.go` — FFmpeg engine, frame decoding, HW transfer, clock
- `internal/media/clock.go` — MasterClock with speed/PTS sync
- `internal/media/audio.go` — AudioPlayer with oto v3 backend
- `internal/media/scrub.go` — SmoothScrubbing predecode goroutines
- `internal/ui/inline_player.go` — Player API, load/play/pause lifecycle
- `native_media.go` — Platform-specific player initialization