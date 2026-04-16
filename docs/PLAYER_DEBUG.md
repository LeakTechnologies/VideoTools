# Player Debug Status

Rolling checklist of known issues, fixes applied, and remaining work for the native media player.

---

## Fixed (dev43 cycle)

- [x] **SW decode SIGSEGV after ~5 frames** `(dev43-297aa24a)` — `GrabFrame` and `NextFrame` called `ensureSwsCtx(e.videoCodecCtx.pix_fmt)`. For many codecs (H.264, AV1, HEVC) `videoCodecCtx.pix_fmt` is `AV_PIX_FMT_NONE` until the first SPS is parsed; `sws_getContext` returned nil, `sws_scale(nil, …)` produced an unrecoverable C SIGSEGV that Go's `recover()` cannot catch. Fixed: use `C.enum_AVPixelFormat(e.frame.format)` — the actual decoded frame format — in both call sites. `NextFrame`'s SW decode path was also missing the `ensureSwsCtx` call entirely.
- [x] **Close()/demuxerLoop use-after-free** `(dev43-297aa24a)` — `Engine.Close()` freed `formatCtx` and `videoCodecCtx` immediately after `close(e.stop)`, while `demuxerLoop` may still be blocked inside `av_read_frame`. Added `sync.WaitGroup demuxerWg`; `demuxerLoop` calls `Done()` on exit; `Close()` waits before freeing any FFmpeg context.
- [x] **NextFrame/Close codec race** `(dev43-297aa24a)` — `Close()` now acquires `videoCodecMu` before freeing `videoCodecCtx`, ensuring any in-flight `NextFrame` decode cycle completes first.
- [x] **seekLoop goroutine leak** `(dev43-297aa24a)` — `seekCh` was never closed so `seekLoop` leaked on every `Close()`. `seekCh` ownership moved to `Load()`: closed and reallocated per file. `Close()` closes the channel. `OnSeek` callback nil-guards under the player mutex.

## Fixed (dev42 cycle)

- [x] **D3D11VA crash — get_format enum mismatch** `(dev42-a45578db)` — `get_format` callback now accepts `AV_PIX_FMT_D3D11VA_VLD` (enum mismatch with `AV_PIX_FMT_D3D11`)
- [x] **AV_NOPTS_VALUE frame crash** `(dev42-5c218da6)` — Skip frames with invalid PTS (`AV_NOPTS_VALUE` or negative) and zero dimensions in both `GrabFrame` and `NextFrame`
- [x] **GrabFrame deadlock** `(dev42-ec7da409)` — Skip path tried to re-lock `videoCodecMu` when already held; removed the redundant `Lock()` call
- [x] **A/V clock double-speed** `(dev42-5c218da6)` — `NextFrame` was multiplying PTS by speed (`pts * e.speed`), but `MasterClock.GetTime()` already accounts for speed. Removed the double-application; wired `clock.SetSpeed()` into `Engine.SetSpeed()`
- [x] **Pause spin-loop** `(dev42-5c218da6)` — `NextFrame` busy-looped at 100% CPU when paused; added 50ms sleep
- [x] **Engine.Close() double-close panic** `(dev42-5c218da6)` — Set `running=false` before closing `stop` channel to prevent double-close panic
- [x] **Video drop routing** `(dev42-5c218da6)` — Inner `Droppable` widgets called `loadMultipleVideos` directly. Now all route through `handleDrop` which respects the active module
- [x] **GStreamer removal** `(dev42-5c218da6)` — Deleted all GStreamer code (~2000 lines). Native media engine is the only player on both platforms.
- [x] **HW frame buffer race** `(dev42-1c1a5bef)` — `retrieveHWFrame()` was sharing `e.rgbaFrame`/`e.rgbaBuffer` with `toRGBA()`. Now uses dedicated `hwRgbaFrame`/`hwRgbaBuffer`.
- [x] **HW frame transfer mutex** `(dev42-a802e192)` — `videoCodecMu` now held during HW→SW transfer and RGBA conversion; eliminates concurrent `AVCodecContext` access.
- [x] **Lazy swsCtx creation** `(dev42-1ecbd0a6)` — `swsCtx` deferred until first frame so HW decode path (where `videoCodecCtx.pix_fmt` is NONE at open time) doesn't crash on `sws_getContext`.
- [x] **HW decode disabled by default** `(dev42-5cf98918)` — D3D11VA crashes in `avcodec_send_packet` are C-level access violations that `recover()` cannot catch. `hwDecodeEnabled` defaults to `false`; opt-in via Settings.

---

## Known Issues (dev44 scope)

### P0 — Process-killing

- [ ] **D3D11VA still crashes when enabled** `(dev42-5cf98918, first audited dev43-186fa244)` — When the user opts in to HW decode via Settings, D3D11VA decode crashes in `avcodec_send_packet` with a C-level access violation. `recover()` cannot catch it. Long-term fix requires wrapping FFmpeg calls in a C SEH (`__try`/`__except`) bridge (`safe_bridge.c`). Until then HW decode stays opt-in with a warning.

### P1 — Playback correctness

- [ ] **HW-fallback SW path uses wrong pixel format** `(dev43-186fa244)` — When HW decode fails (`retrieveHWFrame` errors) and the frame has no `hw_frames_ctx` (SW fallback possible), both `GrabFrame` and `NextFrame` call `ensureSwsCtx(e.videoCodecCtx.pix_fmt)` instead of `ensureSwsCtx(C.enum_AVPixelFormat(e.frame.format))`. This is the same `AV_PIX_FMT_NONE` crash fixed for the pure SW path in dev43 but missed in the HW-fallback branches. Since HW decode is off by default this won't bite most users, but will crash if D3D11VA is enabled and degrades mid-stream.
- [ ] **predecodeFrom shares formatCtx read position with demuxerLoop** `(dev43-186fa244)` — Both `predecodeFrom` (scrubber) and `demuxerLoop` call `av_read_frame` on the same `formatCtx`, serialised per-call via `formatMu`. The per-call lock is correct, but both sides advance the read pointer in an interleaved fashion: the scrubber consumes packets the demuxer would have queued, and vice versa. This does not crash but can produce corrupted/stuck playback after a seek. Fix: give `SmoothScrubbing` its own independent `AVFormatContext` (same pattern as `StartThumbnailExtraction`), or pause the demuxer before scrub reads.
- [ ] **Speed changes do not affect audio tempo** `(dev43-186fa244)` — `AudioPlayer.Read()` does not resample audio pitch/tempo when speed != 1.0. Video frames are dropped/waited to match speed, but audio plays at 1× regardless. Requires `swr_convert` with a rate ratio or a time-stretch filter.
- [ ] **Audio queue not flushed before seek** `(dev43-186fa244)` — `Seek()` flushes the video queue and calls `AudioPlayer.FlushCodec()` but stale packets may remain in `audioQueue` between the codec flush and the demuxer seek. These play as a brief audio glitch at the seek point.

### P2 — Quality / UX

- [ ] **`SmoothScrubbing.predecodeAhead` never triggered** `(dev43-186fa244)` — `predecodeLoop` reads from `s.decodeQueue` but nothing in the codebase writes to `s.decodeQueue`. `predecodeAhead` is dead code. Either wire it (trigger on `NextFrame` milestones) or remove it to reduce confusion.
- [ ] **`retrieveHWFrame` creates a new `sws_getContext` per frame** `(dev43-186fa244)` — HW→SW conversion path allocates and frees an `sws` context on every decoded frame. This is correct for correctness (format may vary) but expensive at high frame rates. Cache the context keyed on `(format, width, height)`.
- [ ] **`Duration()` reads `formatCtx` without a lock** `(dev43-186fa244)` — A nil `formatCtx` check exists but the window between the check and the read is not protected. If `Close()` runs concurrently, this could dereference a freed pointer. Low-frequency; guard with `e.mu`.
- [ ] **`DegradeToSoftware()` races with active decode** `(dev43-186fa244)` — Frees `hwDeviceCtx`/`hwFramesCtx` without holding `videoCodecMu`. Only called from paths not currently wired up so not a live crash risk, but should acquire `videoCodecMu` before touching HW contexts.

---

## Current Status

| Check | State |
|-------|-------|
| SMPTE bars display when no video loaded | ✅ |
| First frame displays after Load() | ✅ |
| SW decode playback (H.264, AV1, MPEG4) | ⚠️ Fixed in dev43 — needs smoke test |
| HW decode (D3D11VA) opt-in | ⚠️ Crashes on some files when enabled; opt-in with warning. SW fallback path has same pix_fmt bug (dev43-186fa244) |
| Seek / scrub | ⚠️ Works but shares formatCtx read position with demuxer (P1) |
| Audio sync | ✅ Correct at 1× speed |
| Speed change | ⚠️ Video timing correct; audio does not pitch-shift |
| Close() / Load() lifecycle | ✅ WaitGroup + mutex gate added in dev43 |
| seekLoop goroutine lifecycle | ✅ Fixed in dev43 |

---

## Architecture Notes

### Load / Play Lifecycle

```
Load()
  v.mu.Lock()
  scrubber.Stop() + engine.Close()   ← waits for demuxerWg; gates videoCodecMu
  close(seekCh); seekCh = make(...); go seekLoop()
  engine = NewEngine()
  engine.Open(path)                  ← opens formatCtx, codecs, audioPlayer
  engine.Start()                     ← demuxerWg.Add(1); go demuxerLoop()
  engine.GrabFrame(4s)               ← TryGet from videoQueue; decode into e.frame
                                        ensureSwsCtx(frame.format); toRGBA()
  engine.Pause()
  scrubber = NewSmoothScrubbing(engine); scrubber.Start()
  engine.StartThumbnailExtraction()  ← independent AVFormatContext; safe
  v.mu.Unlock()

Play()
  engine.Start()     ← no-op (already running)
  engine.Resume()    ← clock.SetPaused(false)
  go playbackLoop()

playbackLoop()        ← per-iteration: snapshot eng/playing under v.mu, then drop lock
  engine.NextFrame()  ← videoQueue.Get() → videoCodecMu → avcodec_send/receive
                         ensureSwsCtx(frame.format) → toRGBA()
                         clock.WaitForPTS(pts) or clock.SetTime(pts)
                         clock.SyncVideo(pts) → drop if late
  SetFrame(img)

Close() / Load()-into-existing-player
  v.playing = false
  scrubber.Stop()         ← close(s.stop); frees s.frame, s.swsCtx
  engine.Close()
    close(e.stop)
    videoQueue.Close()    ← unblocks demuxerLoop queue.Put(); unblocks NextFrame queue.Get()
    audioQueue.Close()
    subtitleQueue.Close()
    demuxerWg.Wait()      ← waits for demuxerLoop to fully exit
    videoCodecMu.Lock()   ← waits for any in-flight NextFrame decode
    sws_freeContext / avcodec_free_context / av_frame_free
    videoCodecMu.Unlock()
    avcodec_free_context (audio, subtitle)
    avformat_close_input  ← safe: demuxer has exited
  close(seekCh)           ← drains seekLoop goroutine
```

### Key Mutexes

| Mutex | Protects | Held During |
|-------|----------|-------------|
| `e.mu` | `running`, `paused`, `loading`, `speed`, chapters, framePool | State checks, Close lifecycle |
| `e.videoCodecMu` | `videoCodecCtx` send/receive; `e.frame`; `e.swsCtx`/`e.rgbaFrame` | `avcodec_send/receive` in NextFrame, GrabFrame, SmoothScrubbing |
| `e.formatMu` | `formatCtx` read/seek | `av_read_frame` in demuxerLoop + SmoothScrubbing, `avformat_seek_file` in Seek |
| `e.demuxerWg` | demuxerLoop goroutine lifecycle | `Close()` waits before freeing FFmpeg contexts |
| `v.mu` (InlineVideoPlayer) | `engine`, `scrubber`, `playing`, `seekCh`, callbacks | Load, Play, Pause, Close snapshots |
| `s.convertMu` (SmoothScrubbing) | `s.swsCtx`, `s.rgbaFrame`, `s.rgbaBuffer` | Lazy init + convertFrameToRGBA |

### HW Decode Path (opt-in, default off)

1. `avcodec_receive_frame` produces frame with `hw_frames_ctx != nil`
2. `retrieveHWFrame()` calls `av_hwframe_transfer_data` to download GPU surface to CPU
3. Creates per-frame `sws_getContext` for format conversion (NV12 → RGBA) — **expensive; see P2 above**
4. `sws_scale` into dedicated `hwRgbaFrame`/`hwRgbaBuffer` (separate from SW buffers)
5. Copies result into `image.RGBA`

### SW Decode Path (default)

1. `avcodec_receive_frame` fills `e.frame` (held under `videoCodecMu`)
2. `ensureSwsCtx(C.enum_AVPixelFormat(e.frame.format))` — lazy create/update based on **actual frame format** (not `videoCodecCtx.pix_fmt`)
3. `toRGBA()` — `sws_scale` into `e.rgbaFrame`; copies result into pooled `image.RGBA`
4. Lock released; `clock.WaitForPTS` / `clock.SyncVideo` happen outside the codec lock

### Clock System

- `MasterClock` manages PTS timing with speed multiplier
- `WaitForPTS(target)` blocks until master clock reaches target
- `SyncVideo(pts)` returns delay or -1 (frame late, drop)
- Audio drives the clock: `AudioPlayer.Read()` calls `clock.SetTime(pts)` on each audio frame
- Video-only files: `NextFrame` calls `clock.SetTime(pts)` directly

---

## Videos Tested

| Video | Container | Codec | HW | dev42 | dev43 |
|-------|-----------|-------|----|-------|-------|
| Herzog.avi | AVI | MPEG4 | SW | ✅ Plays | needs re-test |
| Horny Sports.avi | AVI | MPEG4 | SW | ✅ Plays | needs re-test |
| Meng vs Sting.mp4 | MP4 | H.264 | D3D11VA | ❌ crash frame 5 | HW off by default; SW path needs re-test |
| ECW Terry Funk vs Cactus Jack.mp4 | MOV | H.264 | SW | ❌ crash frame 5 | ✅ fix applied — needs re-test |
| 2 Minutes.mp4 | MP4 | AV1 | SW | ❌ crash frame 5 | ✅ fix applied — needs re-test |

---

## Relevant Files

| File | Role |
|------|------|
| `internal/media/engine.go` | FFmpeg engine, Open/Start/Close lifecycle, GrabFrame, NextFrame, Seek, HW/SW decode |
| `internal/media/clock.go` | MasterClock — PTS sync, speed, WaitForPTS, SyncVideo |
| `internal/media/audio.go` | AudioPlayer — oto v3 backend, codec decode, clock driver |
| `internal/media/scrub.go` | SmoothScrubbing — seek pre-decode with private AVFrame/swsCtx |
| `internal/media/queue.go` | PacketQueue — cond-var queue with Put/Get/TryGet/Flush/Close |
| `internal/ui/inline_player.go` | InlineVideoPlayer API — Load/Play/Pause/Seek/Close, playbackLoop, seekLoop |
| `native_media.go` | Player singleton getters, HasNativeMediaPlayer |
| `docs/NATIVE_PLAYER.md` | Architecture reference — read before touching player code |
