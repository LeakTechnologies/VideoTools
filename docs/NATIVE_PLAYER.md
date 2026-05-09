# Native Media Player

The native player is a CGo/FFmpeg-based video engine gated behind the `native_media` build tag.
Without the tag the app compiles normally — all player types have stub implementations so the
rest of the codebase sees no difference.

---

## Architecture

Three layers, each owned by one package:

```
internal/media   Engine          — CGo/FFmpeg demux + decode + audio output (oto v3)
internal/media   VideoPlayer     — Fyne widget: renders frames, built-in controls overlay
internal/ui      InlineVideoPlayer — THE API LAYER every module talks to
```

### Engine (`internal/media/engine.go`)

Pure CGo wrapper around libavformat + libavcodec + libswscale + libswresample.

- Demuxer loop runs in its own goroutine (`Engine.Start()`)
- Video frames decoded to RGBA via swscale, returned from `NextFrame()`
- Audio decoded + resampled to 48 kHz stereo 16-bit via swresample, played via oto v3
- Hardware decode: VAAPI (Linux), D3D11VA (Windows), QSV (Intel)
  auto-detected at load time via `DetectHWDevice()`
- Seek flushes packet queues and codec buffers, resets the master clock
- `GetChapters()`, `GetAudioTracks()`, `GetSubtitleTracks()` — per-file metadata
- `StartThumbnailExtraction(callback)` — background goroutine, calls back with 160×90 thumbnails

### VideoPlayer widget (`internal/media/view.go`)

Fyne widget backed by `canvas.Raster`. The raster's `draw(w, h int) image.Image`
callback is invoked on Fyne's render goroutine at every repaint (vsync-aligned
because Fyne calls `SwapBuffers` with `SwapInterval=1`).

- `SetFrame(*image.RGBA)` — atomically stores the new frame via
  `atomic.Pointer[image.RGBA]` and calls `widget.Refresh()`. Goroutine-safe;
  no `DoFromGoroutine` round-trip required.
- `draw()` — reads the atomic frame pointer, writes into a **pre-allocated
  `*image.RGBA` buffer** (one allocation per widget-resize, not per frame),
  letterboxes/pillarboxes with black, then scales via `scaleNearest`.
- `scaleNearest` — nearest-neighbour downscale operating directly on `src.Pix`
  / `dst.Pix` byte slices; avoids the `image.Image` interface dispatch and
  `RGBA() uint32` conversion that the generic path would incur per pixel.
- `SetDuration / SetCurrentTime` — drives the built-in seek bar
- `SetInPoint / SetOutPoint` — draws trim markers on the seek bar
- `AddThumbnailFrame` — populates the hover-scrub thumbnail cache
- `SetChapters` — renders chapter markers on the seek bar
- `DisableBuiltinControls()` — hides the built-in overlay; use when the module
  provides its own control row (Convert does this; Trim does not)
- `SetError / ClearError / SetLoading` — error and loading states

### InlineVideoPlayer (`internal/ui/inline_player.go`)

The only type modules should reference. Owns the engine and widget together.

```
Load(path)          — open file, decode first frame, start thumbnail extraction,
                      set chapters; replaces any previously open file cleanly
Play()              — start playback goroutine (playbackLoop)
Pause()             — pause engine and stop playback goroutine
Seek(seconds)       — keyframe-accurate seek + first frame decode
ScrubTo(seconds)    — smooth scrub via SmoothScrubbing (for slider drag)
StepFrame(±1)       — single frame step; updates widget current time
SetSpeed(rate)      — playback speed multiplier

Duration()          — total duration in seconds (valid after Load)
FrameRate()         — frames per second (valid after Load)
CurrentTime()       — current playback position in seconds

SetOnProgress(fn)   — called from playback goroutine on each decoded frame;
                      fn receives current time in seconds; must be goroutine-safe
SetOnEnd(fn)        — called on main goroutine when playback reaches EOF cleanly

SetVolume(0–100)    — output volume
SetMuted(bool)
SelectAudioTrack(i)
SelectSubtitleTrack(i) / DisableSubtitles()
GetAudioTracks() / GetSubtitleTracks() / GetChapters()

Widget()            — returns the *media.VideoPlayer Fyne widget for embedding in layout
Close()             — release engine and audio resources
```

A non-native stub (`internal/ui/inline_player_stub.go`) implements the same interface as
no-ops, so importing packages compile on all build targets.

---

## Build Requirements

```
go build -tags native_media .
```

**Linux:** `pkg-config` for `libavcodec libavformat libswscale libavutil libswresample`

**Windows:** FFmpeg at `C:/ffmpeg`
- Headers: `C:/ffmpeg/include`
- Libs: `C:/ffmpeg/lib` (`avcodec avformat avutil swscale swresample avfilter`)
- Requires CGo via MSYS2/MinGW

Without the tag, `go build .` produces a fully functional app with the static-frame
preview fallback (ffprobe + ffmpeg thumbnails) instead of live playback.

---

## Module Integration Pattern

Every module that shows video follows this pattern — no exceptions.

### 1. Register a singleton in `native_media.go`

```go
var myModulePlayer *ui.InlineVideoPlayer

func init() {
    myModulePlayer = ui.NewInlineVideoPlayer()
}

func GetMyModulePlayer() *ui.InlineVideoPlayer {
    return myModulePlayer
}
```

Add a stub in `native_media_stub.go`:

```go
func GetMyModulePlayer() *ui.InlineVideoPlayer {
    return ui.NewInlineVideoPlayer() // returns the no-op stub
}
```

### 2. Accept the player in Options

Both `view.go` (native_media) and `stub.go` (!native_media) must have the same struct:

```go
type Options struct {
    Player         *ui.InlineVideoPlayer
    Window         fyne.Window
    // ... other fields
}
```

### 3. Wire it from `main.go`

```go
mymodule.BuildView(mymodule.Options{
    Player: GetMyModulePlayer(),
    // ...
})
```

### 4. Use only the API inside the module

```go
ts := &myState{
    player: opts.Player.Widget().(*media.VideoPlayer), // display widget
    api:    opts.Player,                               // control handle
}

// Load
opts.Player.Load(path)

// Track progress and implement module-specific stop logic
opts.Player.SetOnProgress(func(t float64) {
    fyne.CurrentApp().Driver().DoFromGoroutine(func() {
        ts.currentTime = t
        ts.seekBar.SetValue(t)
    }, false)
})

opts.Player.SetOnEnd(func() {
    // reset UI
})
```

### What NOT to do

- Do not call `media.NewEngine()` inside a module
- Do not write a per-module playback goroutine
- Do not call `media.NewVideoPlayer()` — get the widget from `opts.Player.Widget()`
- Do not put module-specific stop logic (preview region, out-point stop) inside
  `InlineVideoPlayer` — keep it in the module via `SetOnProgress`

---

## Current Modules

| Module  | Singleton          | Notes |
|---------|--------------------|-------|
| Convert | `GetConvertPlayer()` | Custom control row; `DisableBuiltinControls()` called; cover frame / save frame buttons |
| Trim    | `GetTrimPlayer()`    | Built-in control overlay; in/out point markers; preview region via `SetOnProgress` |

---

## Frame Delivery Pipeline

```
demuxerLoop ──► videoQueue (packets) ──► videoDecodeLoop ──► frameQueue (8 decoded frames)
                                                                      │
                                                               playbackLoop
                                                                      │
                                                               NextFrame() — A/V sync
                                                                      │
                                                            WaitVsync() — DwmFlush / 60Hz timer
                                                                      │
                                                        SetFrame() — atomic.Pointer store
                                                                      │
                                                     Fyne render goroutine calls draw()
                                                                      │
                                                        scaleNearest() → pre-alloc buffer
                                                                      │
                                                              GPU texture upload
```

### Goroutine roles

| Goroutine | Owns |
|-----------|------|
| `demuxerLoop` | `AVFormatContext` reads via `formatMu`; pushes packets to queues |
| `videoDecodeLoop` | codec send/receive under `videoCodecMu`; pushes RGBA frames to `frameQueue` |
| `audioDecodeLoop` | audio codec + swresample; drives `oto` player buffer |
| `playbackLoop` | pulls frames, does A/V sync, calls `WaitVsync`, updates widget |
| Fyne render | calls `draw()` at vsync; reads `atomic.Pointer` frame |
| `seekHandler` | processes `seekQueue` from `SmoothScrubbing` |

### Why `WaitVsync`

At 50 fps on a 60 Hz display, 15 video frames must be spread across 18 vsync
slots per 300 ms. Without vsync alignment, frame swap can land anywhere within a
16.7 ms vsync window, producing irregular display durations (0–33 ms per frame)
perceived as judder. `WaitVsync()` calls `DwmFlush()` on Windows (blocks until
the DWM compose cycle) so every swap lands right after a vsync edge. The result
is the mathematically unavoidable 2-1-1-1-1 cadence (one 33 ms slot, four 16 ms
slots per 100 ms) rather than irregular timing. Linux/macOS fall back to a
60 Hz-aligned `time.Sleep`.

### A/V synchronisation

`MasterClock` is driven by the audio output timestamp (set via `AudioPlayer.Read`
→ `SetTime`). `NextFrame()` calls `clock.WaitForPTS(pts)` which sleeps until the
master clock reaches the frame's presentation timestamp, then `clock.SyncVideo`
drops frames that arrive more than `MaxDriftThreshold` (300 ms) late.

### Key design choices

| Choice | Reason |
|--------|--------|
| `atomic.Pointer[image.RGBA]` for `VideoPlayer.source` | `SetFrame` is called from the playback goroutine; `draw()` reads on Fyne's render goroutine — atomics eliminate both the data race and the `DoFromGoroutine(true)` round-trip |
| Pre-allocated `drawBuf` in `VideoPlayer` | Avoids ~8 MB allocation per frame (~400 MB/s GC pressure at 50 fps); reallocated only on widget resize |
| `scaleNearest` on raw `Pix` bytes | Avoids `image.Image` interface dispatch + `RGBA() uint32` conversion per pixel; direct 4-byte copy per output pixel |
| `DoFromGoroutine(false)` for time/callbacks | `SetCurrentTime` mutates Fyne widgets (must run on main goroutine) but is non-critical; async keeps the playback goroutine unblocked |
| `frameQueue` buffered to 8 frames | ~160 ms headroom at 50 fps; absorbs single-threaded H.264 I-frame decode spikes (~150 ms) without stalling display |

---

## Playback Loop Internals

`InlineVideoPlayer.Play()` launches `playbackLoop()` in a goroutine:

```
for each frame:
    snapshot engine + playing flag + callbacks under mutex
    if not playing → return
    img, err := eng.NextFrame()        ← blocks on A/V clock (WaitForPTS)
    if err == io.EOF → notify main thread, reload, return
    if err != nil   → notify main thread, return
    media.WaitVsync()                  ← align to display vsync boundary
    v.player.SetFrame(img)             ← atomic store + widget.Refresh()
    DoFromGoroutine(false, func() {    ← async; does not block playback goroutine
        v.player.SetCurrentTime(t)
        onFrm(img)                     ← frame-mirror callback
    })
    onProgress(t)                      ← progress callback (any goroutine)
```

`Pause()` sets `v.playing = false`; the loop exits on next iteration.
`Load()` replaces the engine under the mutex; any running loop exits because
its engine snapshot no longer matches.

### Known timing constraint: 50 fps on 60 Hz

`WaitVsync` makes the judder pattern regular but cannot eliminate it — 15 frames
cannot divide evenly into 18 vsync slots. Eliminating the judder entirely would
require either:
- **Frame interpolation** — generate a synthetic frame for the extra slot (complex,
  latency cost).
- **Direct display output** — bypass Fyne and present via a D3D11 swap chain or
  OpenGL surface with precise vsync control (significant work; tracked below).

Both are future work. The current implementation matches the cadence VLC produces
without a hardware overlay.

---

## Scrubbing

Timeline drag goes through `ScrubTo` → `media.SmoothScrubbing.RequestSeek`.
`SmoothScrubbing` debounces rapid seek requests and decodes the nearest keyframe,
calling back with the frame image. This keeps the seek bar responsive without
flooding the decoder.

---

## Resume Position

Resume state is module-managed, not player-managed. Use `SetOnProgress` with a
time throttle to write positions periodically:

```go
var lastSave time.Time
opts.Player.SetOnProgress(func(t float64) {
    if time.Since(lastSave) >= 5*time.Second {
        lastSave = time.Now()
        resumeState.SavePosition(path, t, duration)
    }
})
```

---

## Planned Improvements

### 1. Bilinear scaling

`scaleNearest` is fast but produces aliasing on non-integer scale factors
(visible on fine horizontal lines / text in the source video). The next step is
to add a `scaleBilinear` path that uses the four surrounding source pixels for
each destination pixel. Selection logic:

- Use bilinear when `scale < 1.0` (downscaling: avoids moiré/aliasing).
- Use nearest when `scale >= 1.0` (upscaling: nearest is sharper for pixel art
  / screen recordings; bilinear would add unnecessary blur).
- Add a widget-level `SetScaleMode(Nearest|Bilinear|Auto)` so modules can
  override if needed.

Implementation note: both `src` and `dst` are `*image.RGBA` so the inner loop
can still work on raw `Pix` bytes; bilinear just needs four source pixel reads
and a weighted average (integer arithmetic, no floats inside the inner loop).

### 2. Frame timing metrics

To diagnose A/V sync drift, judder, and dropped frames the engine should
accumulate per-frame timing data and expose it for display/logging.

Proposed additions to `Engine`:

```go
type FrameTimingStats struct {
    FrameNum      int64
    PTS           float64   // frame presentation timestamp
    ClockAtDecode float64   // master clock when NextFrame returned
    DisplayedAt   time.Time // wall time when SetFrame was called
    Dropped       bool
}

// Ring buffer of the last N frames (e.g. 300 = 6s at 50fps)
func (e *Engine) FrameTimingHistory() []FrameTimingStats
func (e *Engine) ResetFrameTimingHistory()
```

From these, derived metrics:
- **A/V drift** = `ClockAtDecode - PTS` (positive = video ahead of audio)
- **Display jitter** = stddev of `DisplayedAt[i+1] - DisplayedAt[i]`
- **Drop rate** = `dropped / total`

The stats can be exposed in the Inspect module or via a dev overlay toggled by
a keyboard shortcut. This data is the foundation for auto-tuning
`MaxDriftThreshold` and for diagnosing per-file sync anomalies (like the
`pts_delay=2719ms` VLC had to compensate for on the wrestling file).

### 3. Direct display output (long term)

Bypassing Fyne's texture upload for the video rect would let us:
- Present frames directly via a D3D11 swap chain on Windows (zero-copy from
  HW decode to screen).
- Use OpenGL PBO (pixel buffer objects) for async CPU→GPU upload, hiding the
  transfer latency behind the decode of the next frame.

This requires embedding a platform-native window surface inside the Fyne canvas
object, which Fyne supports via `driver.NativeWindow`. The implementation is
significant but would enable true hardware-accelerated playback with no CPU copy.
