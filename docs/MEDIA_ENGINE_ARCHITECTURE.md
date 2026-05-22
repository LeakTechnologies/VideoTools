# Media Engine Architecture

## Overview

The Media Engine is a CGo/FFmpeg-based playback system split across three layers,
each owned by one package and gated behind the `native_media` build tag:

```
internal/media   Engine            — CGo/FFmpeg demux + decode + audio (oto v3)
internal/media   VideoPlayer       — Fyne widget: renders frames, built-in controls
internal/ui      InlineVideoPlayer — THE API LAYER every module talks to
```

Without `native_media`, stub implementations produce a fully functional app with
static-frame preview (ffprobe + ffmpeg thumbnails) instead of live playback.

---

## The 10-Player Problem (Current Architectural Debt)

**root cause:** `native_media.go` allocates 10 separate `InlineVideoPlayer` instances:

| Player var | Used by | Notes |
|---|---|---|
| `convertInlinePlayer` | Convert module | Actually used via GetConvertPlayer() |
| `convertPreviewPlayer` | *(none)* | Allocated but never used |
| `trimInlinePlayer` | Trim module | Used via GetTrimPlayer() |
| `inspectInlinePlayer` | Inspect module | Used via GetInspectPlayer() |
| `subtitleInlinePlayer` | *(none)* | Allocated but never used |
| `upscaleInlinePlayer` | Upscale module | Used internally |
| `audioInlinePlayer` | *(none)* | Allocated but never used |
| `filtersInlinePlayer` | Filters module | Used internally |
| `filtersPreviewPlayer` | Filters preview | Used as peer of filtersInlinePlayer |
| `upscalePreviewPlayer` | Upscale preview | Used as peer of upscaleInlinePlayer |

### Why this is a problem

1. **Inconsistent UX** — Convert, Trim, and Inspect each have their own player
   state. Switching modules loses the playing video. A single shared player would
   preserve state across modules.

2. **Duplicate configuration** — All 10 players must be updated on preference
   changes (aspect ratio, font). The `applyPlayerDefaultAspect` function already
   iterates all 10 — a code smell.

3. **Wasted resources** — 4 players are allocated but never referenced. Each
   `InlineVideoPlayer` holds a `VideoPlayer` widget (~1 MB draw buffer) and
   an `Engine` (format context, codecs, frame pool).

4. **Amateur architecture** — Per-module getters (`GetConvertPlayer()`,
   `GetTrimPlayer()`, `GetInspectPlayer()`) make the player look like it belongs
   to the module rather than being a first-class platform service.

### The Fix: Consolidation to 2 Players

Replace all 10 singletons with 2:

- `GetPrimaryPlayer()` — a single shared player for all single-playback modules
  (Convert, Trim, Inspect). When the user switches modules, playback continues
  uninterrupted.
- `GetPreviewPlayer()` — a second player for before/after comparison in Filters
  and Upscale modules. Wired as `SetPeer(GetPreviewPlayer())` so it mirrors the
  primary's transport.

**Exceptions:** Compare module creates engines directly for side-by-side
comparison (two independent video streams). This is a documented Approved
Exception.

### Migration Plan

1. Replace `convertInlinePlayer`, `trimInlinePlayer`, `inspectInlinePlayer`
   with `primaryInlinePlayer`.
2. Replace `filtersPreviewPlayer`, `upscalePreviewPlayer` with `previewPlayer`.
3. Remove `convertPreviewPlayer`, `subtitleInlinePlayer`, `audioInlinePlayer`
   (unused).
4. Update all getters: `GetConvertPlayer` → `GetPrimaryPlayer`,
   `GetTrimPlayer` → `GetPrimaryPlayer`, `GetInspectPlayer` → `GetPrimaryPlayer`.
5. Remove unused getter stubs from `native_media_stub.go`.

---

## Three-Layer Stack Detail

### Engine (`internal/media/engine.go` + 6 subsystem files)

The Engine was split from a 3245-line monolith into 7 files:

| File | Responsibility | Size |
|---|---|---|
| `engine.go` | Core Engine struct, Open/Close, stream selection, chapter/track queries | 1117 lines |
| `playback.go` | Seek, NextFrame, videoDecodeLoop, audioDecodeLoop, demuxerLoop, Start/Resume/Pause | 909 lines |
| `hwdecode.go` | HW device detection (VAAPI/D3D11VA/QSV), HW→SW frame transfer | 246 lines |
| `errors.go` | Error wrapper types, sentinel errors | 42 lines |
| `framepool.go` | Pre-allocated image.RGBA pool, DecodedFrame struct, frame queue | 105 lines |
| `subtitle_engine.go` | Subtitle decode, ASS overlay rendering | 233 lines |
| `buffer.go` | Packet queue (bounded channel), seek-flush marker | 170 lines |

`engine.go` owns the `Engine` struct with all fields; each subsystem file
attaches methods to it. The split is purely organizational — no interface
boundaries between files. An interface-based refactor (`Player` interface)
is planned.

### VideoPlayer (`internal/media/view.go`)

Fyne widget backed by `canvas.Raster`. Key design:

- **`source atomic.Pointer[image.RGBA]`** — goroutine-safe frame store.
  `SetFrame()` is called from the playback goroutine (or seek goroutine).
  `draw()` reads on Fyne's render goroutine. Atomics eliminate the data race
  AND the `DoFromGoroutine` round-trip.
- **Pre-allocated `drawBuf`** — avoids ~8 MB allocation per frame.
  Reallocated on widget resize only.
- **`scaleNearest`** — nearest-neighbour downscale on raw `Pix` bytes.
  Avoids `image.Image` interface dispatch and `RGBA() uint32` per pixel.
- **SMPTE colour bars** — rendered when `source == nil` (no video loaded).
  Aspect ratio controlled by `idleAspectRatio` field (default 16:9).

### InlineVideoPlayer (`internal/ui/inline_player.go`)

The only type modules should reference. Owns the Engine and VideoPlayer together.

```
Load(path)          → Engine.Open + GrabFrame + thumbnail extraction
Play()              → Start + Resume + playbackLoop goroutine
Pause()             → Engine.Pause + stop playbackLoop
Seek(seconds)       → Engine.Seek + first-frame decode
ScrubTo(seconds)    → SmoothScrubbing.RequestSeek (debounced)
StepFrame(±1)       → single frame advance/backward
SetSpeed(rate)      → MasterClock.SetSpeed
Widget()            → returns *media.VideoPlayer for layout embedding
SetPeer(other)      → mirror play/pause/seek to another player
```

---

## Seek Architecture & The Fixed Accurate Fallback Bug

### Seek flow

```
InlineVideoPlayer.seek(target)
  └─► seekLoop goroutine (drains seekCh, debounces)
      └─► Engine.Seek(target)
          ├─► avformat_seek_file (keyframe: flags=0, nearest direction)
          ├─► peek 5 packets to check distance from target
          ├─► if diff > 2s: accurate fallback seek (BACKWARD|ACCURATE)
          ├─► flush video + audio packet queues
          ├─► avcodec_flush_buffers (destroys decoder reference state)
          ├─► clock.ResetTime(target)
          ├─► drain frameQueue
          └─► return
      └─► Engine.NextFrame() → first frame from new position
      └─► SetFrame(img) + SetCurrentTime(target)
```

### The bug (FIXED dev49)

The accurate fallback at `playback.go:179-181` used `AVSEEK_FLAG_ACCURATE` **without**
`AVSEEK_FLAG_BACKWARD`. This positioned the format context's read cursor at the
exact target PTS — typically **mid-GOP** (between keyframes). Then
`avcodec_flush_buffers()` destroyed all decoder reference state. The first packet
read from mid-GOP was a P/B-frame with no reference I-frame → **garbage output**
until the next I-frame arrived (typically 1-3 seconds of corruption).

**Fix:** Added `AVSEEK_FLAG_BACKWARD` so the accurate fallback always lands at
the keyframe immediately before the target. The decoder receives a valid I-frame
first, then decodes forward through the GOP to reach the target PTS.

### Seek accuracy modes

| Mode | Flags | Behavior |
|---|---|---|
| `SeekAccuracyKeyframe` (default) | `flags=0` (no BACKWARD) | FFmpeg picks nearest keyframe in either direction. Fast (<5ms). |
| `SeekAccuracyAccurate` | `BACKWARD|ACCURATE` | Seeks to keyframe before target, then decodes forward. Slower but exact. |
| Adaptive fallback | `BACKWARD|ACCURATE` (retry) | Auto-retry when keyframe seek lands >2s from target. Newly fixed. |

### Logging added (dev49)

- `Seek: flags=` — human-readable seek flags
- `Seek: accurate fallback with BACKWARD` — confirms fallback path
- `Seek: clock reset to %.2f (audio offset: -%.2fs)` — clock anchor target
- `Seek: drained %d stale frames from frameQueue` — queue drain count
- `videoDecodeLoop: seekGen changed %d→%d — first frame after seek` — decode
  resumption after seek, includes frame format/type/keyframe flags
- `InlineVideoPlayer.Seek: first frame after seek` — seek completion in the
  API layer

---

## Frame Pacing & Timing

### Current pacing (after dev49 changes)

```
NextFrame():
  clock.WaitForPTS(pts)     ← blocks until clock reaches frame PTS
  clock.SyncVideo(pts)      ← if behind > MaxDriftThreshold, drop OR snap
  return img
```

Changes made in dev49:
- **No-audio path** (was `clock.SetTime(pts)`, now `WaitForPTS(pts)`):
  Previously the clock was snapped to PTS every frame, meaning decode speed
  determined frame rate — no actual pacing. Now `WaitForPTS` blocks for the
  correct PTS interval, giving real-time pacing even without audio.
- **WaitVsync removed** from `playbackLoop`: The `DwmFlush()` call after every
  `NextFrame` introduced 0-16.7ms per-frame jitter. Frame timing is now purely
  PTS-driven. (DWM on Windows 10+ handles composition regardless.)
- **Frame rate propagated**: `v.player.SetFrameRate(eng.GetFrameRate())` added
  in `loadViaOpen` ready callback.

### The two player consistency problem

Currently each module has its own `InlineVideoPlayer`. This means:
- Filters preview and main player are separate instances running independently.
- `SetPeer()` mirrors play/pause/seek but doesn't synchronize frame-level timing.
- After the consolidation to 2 players, the primary-preview pair will be the
  only peer relationship, and it will be consistent across all modules.

### Clock architecture

`MasterClock` is driven by:
1. **Audio path**: AudioPlayer.Read() → `SetTime(pts)` (monotonic ratchet)
2. **No-audio path**: clock advances via wall-time ticks seeded by
   `WaitForPTS(pts)`. No explicit wall-time correction goroutine.
3. **Seek**: `ResetTime(seconds)` anchors the clock to the target. Audio path
   subtracts `AudioBufferLatency` (default ~60ms) to account for the audio
   output buffer.

Drift safeguards:
- `SetTime` is a monotonic ratchet — ignores backward jumps.
- During frame queue stall, clock is paused via `SetPaused(true)`.
- If clock exceeds frame PTS by `MaxDriftThreshold` (300ms), clock snaps and
   frame is displayed rather than dropped.
- Audio underrun: `Read()` counts consecutive silent callbacks (logs warning at
  10 frames = ~230ms).

---

## Module Player Usage Patterns

Four distinct patterns exist:

### Pattern A: Single player (Convert, Trim, Inspect)

Module calls `opts.Player.Load(path)`, receives player through Options struct.
For Convert, the view also calls `GetConvertPlayer()` directly for some
operations (refresh, load-video). This needs to become `GetPrimaryPlayer()`.

### Pattern B: Primary + Preview (Filters, Upscale)

`native_media.go` calls `filtersInlinePlayer.SetPeer(filtersPreviewPlayer)` in
`init()`. The preview is muted and has built-in controls disabled. Both share
the same `Load(path)` flow.

### Pattern C: Direct engine creation (Compare)

Compare creates two `media.Engine()` instances directly — one per video stream.
This is an Approved Exception because InlineVideoPlayer doesn't support dual
simultaneous playback. No change planned.

### Pattern D: Disc debug (C utility)

`internal/media/disc_debug.{c,h,go}` provides C-level functions for probing
DVD filesystems (FindFirstFile/opendir). Not part of the playback pipeline.

---

## Known Issues

### Pre-existing (not caused by dev49 changes)

| Issue | Location | Notes |
|---|---|---|
| ASS subtitle format bugs | `subtitle_engine.go` | `formatASSTime`, `escapeASSText` — 2 failing tests; pre-existing |
| HW decode disabled by default | `engine.go` | D3D11VA/VAAPI detected but not enabled — safety policy |
| No wall-time clock correction | `clock.go` | Long audio underruns cause uncorrected drift |
| 50fps on 60Hz cadence | `playback.go` | Structural: 15 frames can't divide evenly into 18 vsync slots |

### Fixed in dev49

| Issue | Location | Fix |
|---|---|---|
| Accurate fallback mid-GOP corruption | `playback.go:179-181` | Added AVSEEK_FLAG_BACKWARD |
| No-audio frame blasting | `playback.go:646` | Changed SetTime(pts) → WaitForPTS(pts) |
| WaitVsync jitter (0-16ms per frame) | `inline_player.go` | Removed WaitVsync from playbackLoop |
| Frame rate not propagated | `inline_player.go` | Added SetFrameRate in loadViaOpen |
| Clock not seeded in no-audio path | `playback.go` | WaitForPTS first call seeds clock from pts |
| Engine.go monolith | `engine.go` | Split into 6 subsystem files |

### Remaining for dev49

| Issue | Status |
|---|---|
| 10-player singleton consolidation | **PENDING** — replace with GetPrimaryPlayer/GetPreviewPlayer |
| Performance-encode frames from VOB boundary artifacts | Blocked on user's FFmpeg not having dvdvideo demuxer |
| Chapter embedding in rip | Need to verify EmbedChapters flag propagation |

---

## Build & Verification

```
go build -tags native_media .
go vet ./internal/media/
go vet ./internal/ui/
go test ./internal/media/    # 29/31 pass (2 pre-existing ASS subtitle failures)
```

### Test failures (pre-existing)

```
--- FAIL: TestFormatASSTime
--- FAIL: TestEscapeASSText
```

Both are formatting/escaping edge cases in the ASS subtitle overlay renderer.
Not related to seek, pacing, or frame delivery.

---

## Player Consolidation Design

### Current (dev49)

```go
func GetConvertPlayer() *ui.InlineVideoPlayer   { return convertInlinePlayer }
func GetTrimPlayer() *ui.InlineVideoPlayer      { return trimInlinePlayer }
func GetInspectPlayer() *ui.InlineVideoPlayer   { return inspectInlinePlayer }
func GetFiltersPlayer() *ui.InlineVideoPlayer   { return filtersInlinePlayer }
func GetFiltersPreviewPlayer() *ui.InlineVideoPlayer { return filtersPreviewPlayer }
func GetUpscalePlayer() *ui.InlineVideoPlayer   { return upscaleInlinePlayer }
func GetUpscalePreviewPlayer() *ui.InlineVideoPlayer  { return upscalePreviewPlayer }
func GetSubtitlePlayer() *ui.InlineVideoPlayer  { return subtitleInlinePlayer }
func GetAudioPlayer() *ui.InlineVideoPlayer     { return audioInlinePlayer }
func GetConvertPreviewPlayer() *ui.InlineVideoPlayer  { return convertPreviewPlayer }
```

### Target

```go
var primaryInlinePlayer  *ui.InlineVideoPlayer  // all single-playback modules
var previewPlayer        *ui.InlineVideoPlayer  // Filters/Upscale before/after

func GetPrimaryPlayer() *ui.InlineVideoPlayer   { return primaryInlinePlayer }
func GetPreviewPlayer() *ui.InlineVideoPlayer   { return previewPlayer }

// Backward-compat aliases — remove after all callers migrated:
func GetConvertPlayer() *ui.InlineVideoPlayer   { return GetPrimaryPlayer() }
func GetTrimPlayer() *ui.InlineVideoPlayer      { return GetPrimaryPlayer() }
func GetInspectPlayer() *ui.InlineVideoPlayer   { return GetPrimaryPlayer() }
```

### Migration steps

1. Add `primaryInlinePlayer` and `previewPlayer` vars alongside existing vars.
2. Reassign: `primaryInlinePlayer = ui.NewInlineVideoPlayer()`,
   `previewPlayer = ui.NewInlineVideoPlayer()`.
3. Wire: `filtersInlinePlayer.SetPeer(previewPlayer)`,
   `upscaleInlinePlayer.SetPeer(previewPlayer)`.
4. Add `GetPrimaryPlayer()` and `GetPreviewPlayer()` getters.
5. Replace all `GetConvertPlayer()` calls with `GetPrimaryPlayer()`.
6. Replace `GetTrimPlayer()` with `GetPrimaryPlayer()`.
7. Replace `GetInspectPlayer()` with `GetPrimaryPlayer()`.
8. Replace `GetFiltersPreviewPlayer()` with `GetPreviewPlayer()`.
9. Replace `GetUpscalePreviewPlayer()` with `GetPreviewPlayer()`.
10. Remove unused `convertPreviewPlayer`, `subtitleInlinePlayer`, `audioInlinePlayer`.
11. Remove unused getter stubs from `native_media_stub.go`.
12. Update `applyPlayerDefaultAspect` to only iterate 2 players.
13. Build, vet, test.

---

## Future Work (Post-dev49)

- **Player interface extraction**: Extract formal `Player` interface from
  `InlineVideoPlayer` so modules depend on the interface, not the concrete type.
- **view.go component split**: Break 1438-line VideoPlayer widget into
  `control_overlay.go`, `keyboard_shortcuts.go`, `thumbnail_preview.go`.
- **HW decode default-on**: With VEH/SEH bridge now catching stack overflow and
  access violations, consider enabling HW decode by default.
- **Lock hierarchy formalization**: Add lockdep-style assertions to prevent
  reverse-order lock acquisition.
- **Frame timing metrics**: Accumulate per-frame PTS, clock, drop stats for
  diagnostic overlay (Inspect module or dev overlay).
- **Clock drift correction goroutine**: Background goroutine to detect and
  correct drift during long audio underruns.
- **Bilinear scaling**: Replace nearest-neighbour with bilinear for downscaled
  frames (reduces aliasing on fine horizontal lines).
