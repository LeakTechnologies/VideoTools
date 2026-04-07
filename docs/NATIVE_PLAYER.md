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

Fyne widget that renders decoded frames using `canvas.Raster` + the vendored
`UpdatePixels` extension (efficient in-place pixel swap without texture recreation).

- `SetFrame(*image.RGBA)` — displays a frame; scales to widget size maintaining aspect ratio
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

## Playback Loop Internals

`InlineVideoPlayer.Play()` launches `playbackLoop()` in a goroutine:

```
for each frame:
    snapshot engine + playing flag + onProgress callback under mutex
    if not playing → return
    img, err := eng.NextFrame()
    if err == io.EOF → call SetOnEnd on main thread, return cleanly
    if err != nil   → call SetError on main thread, return
    widget.SetFrame(img)
    widget.SetCurrentTime(t)
    onProgress(t)             ← fires SetOnProgress callbacks
```

`Pause()` sets `v.playing = false`; the loop exits on next iteration.
`Load()` replaces the engine under the mutex; any running loop exits because
its engine snapshot no longer matches.

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
