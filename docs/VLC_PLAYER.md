# VLC Player Backend

Replace the custom FFmpeg demux/decode/sync engine with libVLC for video playback.
The FFmpeg engine stays as a long-term plan; libVLC is battle-tested for seek/resume/A-V sync.

---

## Why

The custom FFmpeg engine (`internal/media/engine.go` + `playback.go`) has had 10+ crash-fix
cycles across dev43–dev55. The architecture — 6 goroutines, 4 mutexes, 3 packet queues,
a frame queue, and a CGo SEH bridge — is fundamentally too complex to stabilise for
user-facing playback. Every seek can land on a keyframe that stalls the decode pipeline,
and every resume must flush stale state across multiple subsystems without a race.

libVLC has solved these problems for 20+ years. Its internal player handles demux, decode,
A-V sync, seek, resume, subtitle rendering, and track selection — all in a single
well-tested library. Wrapping it gives us a stable player at ~300 lines of CGo instead of
~1500.

---

## Architecture

### Layer separation (unchanged)

```
internal/media   Engine (FFmpeg) OR VLCEngine (libVLC) — playback backend
internal/media   VideoPlayer — Fyne widget: renders RGBA frames, seek bar, controls overlay
internal/ui      InlineVideoPlayer — THE API LAYER every module talks to
```

Modules never see the backend. `InlineVideoPlayer` delegates to whichever Engine is active.

### Backend selection

A `PlaybackEngine` interface sits between `InlineVideoPlayer` and the concrete engines:

```go
// internal/media/engine_interface.go
type PlaybackEngine interface {
    Load(path string) error
    Play()
    Pause()
    Stop()
    Seek(seconds float64, accuracy SeekAccuracy) error
    Duration() float64
    CurrentTime() float64
    FrameRate() float64
    Playing() bool

    SetOnFrame(fn func(*image.RGBA))     // frame callback (RGBA decoded frame)
    SetOnProgress(fn func(float64))       // progress callback (seconds)
    SetOnEOF(func())                      // end-of-stream callback
    SetVolume(v int)                      // 0–100
    SetMuted(bool)
    SetSpeed(rate float64)

    AudioTrackCount() int
    AudioTrackDescriptors() []TrackDescriptor
    SelectAudioTrack(id int)
    SubtitleTrackCount() int
    SubtitleTrackDescriptors() []TrackDescriptor
    SelectSubtitleTrack(id int)
    DisableSubtitles()
    ChapterCount() int
    ChapterIndex() int
    SelectChapter(idx int)

    GrabFrame(at float64) (*image.RGBA, error)  // scrub: decode frame at position
    ThumbnailExtract(path string, w, h int, cb func(*image.NRGBA))  // background thumbnail
    Close()
}
```

- `FFmpegBackend` wraps existing `*Engine` (current path, unchanged behaviour)
- `VLCBackend` wraps new libVLC code (simpler, stable seek/resume)

`InlineVideoPlayer` stores `engine PlaybackEngine` instead of `engine *Engine`.
The `VideoPlayer` widget is shared — both backends feed RGBA frames via `SetFrame()`.

### Build-tag gating

- `//go:build native_media && vlc` — VLC backend files
- `//go:build native_media && !vlc` — FFmpeg engine files (current)
- Default: `!vlc` = FFmpeg engine (no change for CI/release until VLC is validated)
- When VLC is validated: flip default or remove FFmpeg player entirely

### Config field

```go
// internal/appcfg/config.go
type ConvertConfig struct {
    // ... existing fields ...
    UseVLC bool `json:"useVlc"` // false = FFmpeg engine (default); true = libVLC
}
```

A settings toggle lets the user choose the backend at runtime. This lets us ship VLC
as opt-in while keeping FFmpeg as fallback during validation.

---

## CGo Wrapper

`adrg/libvlc-go/v3` does NOT expose `libvlc_video_set_callbacks` — it only renders to
native windows (HWND/X11/NSView). We need our own thin CGo wrapper for frame callbacks.

### Files

| File | Purpose |
|------|---------|
| `internal/media/vlc_glue.h` | C declarations: extern Go callbacks, static bridge functions |
| `internal/media/vlc_engine.go` | Go struct, CGo init, `libvlc_new`, media load, play/pause/seek |
| `internal/media/vlc_video.go` | Video callback setup, RGBA buffer management, frame delivery |
| `internal/media/vlc_events.go` | Event handling: EOF, error, position changed |

### C glue pattern

```c
// vlc_glue.h
#include <vlc/vlc.h>

// Go export functions (defined in vlc_video.go)
extern void* goVLCLockCB(void* opaque, void** planes);
extern void  goVLCUnlockCB(void* opaque, void* picture, void* const* planes);
extern void  goVLCDisplayCB(void* opaque, void* picture);

// Static bridge (CGo cannot pass Go function pointers directly to C)
static void vlcSetCallbacks(libvlc_media_player_t* mp, void* userdata) {
    libvlc_video_set_callbacks(mp,
        goVLCLockCB, goVLCUnlockCB, goVLCDisplayCB, userdata);
}
```

### Frame delivery

1. `VLCEngine.Load(path)` — creates `libvlc_media_new_path` → `libvlc_media_player_new_from_media`
2. Calls `libvlc_video_set_format(mp, "RGBA", width, height, width*4)` with initial dimensions
3. Calls `vlcSetCallbacks(mp, ctx)` via CGo static bridge
4. On `libvlc_media_player_play`, libVLC decodes frames and calls our callbacks

Per-frame flow:
```
lock_cb(opaque, planes)     — set planes[0] to our RGBA buffer; return picture handle
  [libVLC decodes into buffer]
unlock_cb(opaque, picture)  — frame decoded; copy to *image.RGBA
display_cb(opaque, picture) — trigger SetFrame() on VideoPlayer widget
```

### Buffer management

- Single pre-allocated `[]byte` buffer sized to max video dimensions
- `libvlc_video_set_format` is called on first frame when dimensions are known
- If dimensions change mid-stream, reallocate and call format again
- `unsafe.Pointer` arithmetic to copy from C buffer to Go `image.RGBA.Pix`

---

## libVLC API Surface Used

| Function | Purpose |
|----------|---------|
| `libvlc_new(...)` | Create instance with `--no-xlib` (headless), `--verbose=0` |
| `libvlc_release(...)` | Cleanup |
| `libvlc_media_new_path(...)` | Load file |
| `libvlc_media_player_new_from_media(...)` | Create player |
| `libvlc_media_player_play/pause/stop(...)` | Control |
| `libvlc_media_player_set_time/time/length(...)` | Seek + info |
| `libvlc_media_player_set_position/position(...)` | Relative seek |
| `libvlc_media_player_is_playing/is_seekable(...)` | State queries |
| `libvlc_media_player_set_volume/volume(...)` | Audio volume |
| `libvlc_media_player_set_mute/mute(...)` | Mute |
| `libvlc_media_player_video_set_track(...)` | Track selection |
| `libvlc_media_player_audio_set_track(...)` | Track selection |
| `libvlc_media_player_video_set_spu(...)` | Subtitle track |
| `libvlc_video_set_callbacks(...)` | Frame callbacks |
| `libvlc_video_set_format(...)` | RGBA pixel format |
| `libvlc_media_player_event_manager(...)` | Events |
| `libvlc_event_attach/detach(...)` | Subscribe to events |

---

## Phased Implementation

### Phase 1: Basic playback (no trim, no scrub)

**Goal:** Convert module can load a file, play/pause/seek, and show frames. No in/out
markers, no scrubbing, no thumbnails.

**Scope:**
- `PlaybackEngine` interface (extracted from InlineVideoPlayer's Engine usage)
- `FFmpegBackend` implementing the interface (wrap existing Engine)
- `VLCBackend` with basic libVLC CGo wrapper
- `InlineVideoPlayer` refactored to accept `PlaybackEngine`
- Build tag selects default backend
- CI builds both variants

**Files changed:**
- `internal/media/engine_interface.go` (new — interface)
- `internal/media/ffmpeg_backend.go` (new — wraps Engine)
- `internal/media/vlc_glue.h` (new)
- `internal/media/vlc_engine.go` (new)
- `internal/media/vlc_video.go` (new)
- `internal/ui/inline_player.go` (modified — uses interface)

### Phase 2: Full feature parity

**Goal:** Trim in/out markers, smooth scrubbing, thumbnails, track selection, chapters.

**Scope:**
- `GrabFrame` via `libvlc_media_player_set_time` + one-frame decode
- Thumbnail extraction via `libvlc_media_player_set_time` + snapshot
- Track/chapter selection via `libvlc_media_player_*_set_track`
- Speed control via `libvlc_media_player_set_rate`
- Events: EOF, error, position changed

### Phase 3: Validation + default flip

**Goal:** User validation of VLC backend. Flip default from FFmpeg to VLC.

**Scope:**
- Settings toggle for backend selection
- User testing
- Remove FFmpeg player code (long-term)

---

## Build Requirements

### Windows

- **libVLC SDK:** Download from https://get.videolan.org/vlc/ — extract headers + DLLs
- **Headers:** `C:/vlc/include/vlc/` — `vlc.h`, `vlc_media.h`, `vlc_media_player.h`
- **Libs:** `C:/vlc/lib/` — `libvlc.dll.a` (import lib) + `libvlc.dll` (runtime)
- **Runtime:** `libvlc.dll` + `libvlccore.dll` must be in PATH or next to the EXE
- **CGo flags:**
  ```
  #cgo windows LDFLAGS: -LC:/vlc/lib -llibvlc
  #cgo windows CFLAGS: -IC:/vlc/include
  ```

### Linux

- **Packages:** `libvlc-dev` (Ubuntu/Debian) or `vlc-devel` (Fedora)
- **Libs:** `pkg-config --libs libvlc`
- **Runtime:** `libvlc.so.5` (usually in system path)

### CI (GitHub Actions)

- Windows: Download VLC SDK as a build step (or cache it)
- Linux: `apt install libvlc-dev`
- Build with `-tags "native_media vlc"` for VLC variant

---

## Risk Assessment

| Risk | Mitigation |
|------|-----------|
| libVLC DLLs large (~50 MB) | Ship as separate sidecar ZIP (like FFmpeg) |
| HW accel disabled in callback mode | Acceptable for short clip preview; CPU decode is fast enough |
| Subtitle rendering requires CPU blending | Acceptable — subtitles are rendered into the RGBA buffer by libVLC |
| CGo callback complexity | ~300 lines vs FFmpeg's ~1500; well-documented C API |
| libVLC version differences | Pin to VLC 3.0.x (stable) via SDK download |
| `libvlc_media_player_set_time` may not be frame-accurate | For GrabFrame, use `libvlc_media_player_set_time` + wait for frame callback |
| No `ScrubTo` equivalent (continuous seek during drag) | Use `libvlc_media_player_set_position` for real-time position updates |

---

## Relationship to FFmpeg Player

The FFmpeg engine (`engine.go`, `playback.go`, `queue.go`, etc.) stays in the codebase.
Once VLC is validated and default, the FFmpeg files are:
1. Gated behind `//go:build !vlc` (or removed entirely)
2. Kept in `docs/NATIVE_PLAYER.md` as architecture reference
3. Eventually retired when VLC proves stable in production

The FFmpeg engine's design is documented in `NATIVE_PLAYER.md` — this is valuable
architecture knowledge that informs the VLC wrapper's interface design.

---

## Testing Checklist

- [ ] Load and play MP4 (H.264 + AAC)
- [ ] Load and play MKV (H.265 + AAC)
- [ ] Load and play AVI (MPEG-2 + MP3)
- [ ] Load and play MOV (ProRes + PCM)
- [ ] Seek to random positions (10+ seeks in rapid succession)
- [ ] Seek to start (position 0)
- [ ] Seek to near-end (position > 95%)
- [ ] Pause and resume (position must be exact)
- [ ] Resume after app restart (persisted position)
- [ ] Stop and reload same file
- [ ] Stop and load different file
- [ ] Volume control (0%, 50%, 100%)
- [ ] Mute/unmute
- [ ] Playback speed (0.5×, 1×, 2×)
- [ ] Audio track selection (multi-track file)
- [ ] Subtitle track selection (MKV with subs)
- [ ] Chapter navigation (MKV with chapters)
- [ ] EOF notification (onEnd callback fires)
- [ ] Error handling (corrupt file, missing codec)
- [ ] Thumbnail extraction (background goroutine)
- [ ] Smooth scrubbing (slider drag)
- [ ] Trim in/out markers (set + playback respects region)
- [ ] Convert module: cover frame capture
- [ ] Convert module: save frame to file
- [ ] Performance: 50 fps source on 60 Hz display (judder comparison with FFmpeg)
