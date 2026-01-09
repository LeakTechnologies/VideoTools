# Phase 2: GStreamer Integration Plan

## Current State Analysis

### Two Player Systems Found:

1. **Player Module** (`main.go:6609`)
   - Uses: `player.Controller` interface
   - Implementation: `ffplayController` (uses external ffplay window)
   - File: `internal/player/controller_linux.go`
   - Problem: External window, not embedded in Fyne UI

2. **Convert Preview** (`main.go:11132`)
   - Uses: `playSession` struct
   - Implementation: `UnifiedPlayerAdapter` (broken FFmpeg pipes)
   - File: Defined in `main.go`
   - Problem: Uses the UnifiedPlayer we're deleting

## Integration Strategy

### Option A: Unified Approach (RECOMMENDED)
Replace both systems with a **single GStreamer-based player**:

```
GStreamerPlayer (internal/player/gstreamer_player.go)
    ↓
    ├──> Player Module (embedded playback)
    └──> Convert Preview (embedded preview)
```

**Benefits:**
- Single code path
- Easier to maintain
- Both use same solid GStreamer backend

### Option B: Hybrid Approach
Keep Controller interface, but make it use GStreamer internally:

```
Controller interface
    ↓
GStreamerController (wraps GStreamerPlayer)
    ↓
GStreamerPlayer
```

**Benefits:**
- Minimal changes to main.go
- Controller interface stays the same

**We'll use Option A** - cleaner, simpler.

---

## Implementation Steps

### Step 1: Create GStreamer-Based Controller
File: `internal/player/controller_gstreamer.go`

```go
//go:build gstreamer

package player

import (
    "fmt"
    "time"
)

func newController() Controller {
    return &gstreamerController{
        player: NewGStreamerPlayer(Config{
            PreviewMode: false,
            WindowWidth: 640,
            WindowHeight: 360,
        }),
    }
}

type gstreamerController struct {
    player *GStreamerPlayer
}

func (c *gstreamerController) Load(path string, offset float64) error {
    return c.player.Load(path, time.Duration(offset*float64(time.Second)))
}

func (c *gstreamerController) SetWindow(x, y, w, h int) {
    c.player.SetWindow(x, y, w, h)
}

func (c *gstreamerController) Play() error {
    return c.player.Play()
}

func (c *gstreamerController) Pause() error {
    return c.player.Pause()
}

func (c *gstreamerController) Seek(offset float64) error {
    return c.player.SeekToTime(time.Duration(offset * float64(time.Second)))
}

func (c *gstreamerController) SetVolume(level float64) error {
    // Controller uses 0-100, GStreamer uses 0.0-1.0
    return c.player.SetVolume(level / 100.0)
}

func (c *gstreamerController) FullScreen() error {
    return c.player.SetFullScreen(true)
}

func (c *gstreamerController) Stop() error {
    return c.player.Stop()
}

func (c *gstreamerController) Close() {
    c.player.Close()
}
```

### Step 2: Update playSession to Use GStreamer
File: `main.go` (around line 11132)

**BEFORE:**
```go
type playSession struct {
    // ...
    unifiedAdapter *player.UnifiedPlayerAdapter
}

func newPlaySession(...) *playSession {
    unifiedAdapter := player.NewUnifiedPlayerAdapter(...)
    return &playSession{
        unifiedAdapter: unifiedAdapter,
        // ...
    }
}
```

**AFTER:**
```go
type playSession struct {
    // ...
    gstPlayer *player.GStreamerPlayer
}

func newPlaySession(...) *playSession {
    gstPlayer, err := player.NewGStreamerPlayer(player.Config{
        PreviewMode:   true,
        WindowWidth:   targetW,
        WindowHeight:  targetH,
        Volume:        1.0,
    })
    if err != nil {
        // Handle error
    }

    return &playSession{
        gstPlayer: gstPlayer,
        // ...
    }
}
```

### Step 3: Update playSession Methods
Replace all `unifiedAdapter` calls with `gstPlayer`:

```go
func (p *playSession) Play() {
    p.mu.Lock()
    defer p.mu.Unlock()

    if p.gstPlayer != nil {
        p.gstPlayer.Play()
    }
    p.paused = false
}

func (p *playSession) Pause() {
    p.mu.Lock()
    defer p.mu.Unlock()

    if p.gstPlayer != nil {
        p.gstPlayer.Pause()
    }
    p.paused = true
}

func (p *playSession) Seek(offset float64) {
    p.mu.Lock()
    defer p.mu.Unlock()

    if p.gstPlayer != nil {
        p.gstPlayer.SeekToTime(time.Duration(offset * float64(time.Second)))
    }
    p.current = offset
    // ...
}

func (p *playSession) Stop() {
    p.mu.Lock()
    defer p.mu.Unlock()

    if p.gstPlayer != nil {
        p.gstPlayer.Stop()
    }
    p.stopLocked()
}
```

### Step 4: Connect GStreamer Frames to Fyne UI

The key challenge: GStreamer produces RGBA frames, Fyne needs to display them.

**In playSession:**
```go
// Start frame display loop
go func() {
    ticker := time.NewTicker(time.Second / time.Duration(fps))
    defer ticker.Stop()

    for {
        select {
        case <-p.stop:
            return
        case <-ticker.C:
            if p.gstPlayer != nil {
                frame, err := p.gstPlayer.GetFrameImage()
                if err == nil && frame != nil {
                    fyne.CurrentApp().Driver().DoFromGoroutine(func() {
                        p.img.Image = frame
                        p.img.Refresh()
                    }, false)
                }
            }
        }
    }
}()
```

---

## Module Integration Points

### Modules Using Player:

| Module | Usage | Status | Notes |
|--------|-------|--------|-------|
| **Player** | Main playback | ✅ Ready | Uses Controller interface |
| **Convert** | Preview pane | ✅ Ready | Uses playSession |
| **Trim** | Not implemented | ⏳ Waiting | Blocked by player |
| **Filters** | Not implemented | ⏳ Waiting | Blocked by player |

### After GStreamer Integration:
- ✅ Player module: Works with GStreamerController
- ✅ Convert preview: Works with GStreamerPlayer directly
- ✅ Trim module: Can be implemented (player stable)
- ✅ Filters module: Can be implemented (player stable)

---

## Build Order

1. Install GStreamer (user runs command)
2. Create `controller_gstreamer.go`
3. Update `playSession` in `main.go`
4. Build with `./scripts/build.sh`
5. Test Player module
6. Test Convert preview
7. Verify no crashes

---

## Testing Checklist

### Player Module Tests:
- [ ] Load video file
- [ ] Play button works
- [ ] Pause button works
- [ ] Seek bar works
- [ ] Volume control works
- [ ] Frame stepping works (if implemented)

### Convert Preview Tests:
- [ ] Load video in Convert module
- [ ] Preview pane shows video
- [ ] Playback works in preview
- [ ] Seek works in preview
- [ ] Preview updates when converting

---

## Rollback If Needed

If GStreamer integration has issues:

```bash
# Revert controller
git checkout HEAD -- internal/player/controller_gstreamer.go

# Revert playSession changes
git checkout HEAD -- main.go

# Rebuild without GStreamer
GOFLAGS="" ./scripts/build.sh
```

---

## Success Criteria

Phase 2 is complete when:
- ✅ GStreamer installed on system
- ✅ VideoTools builds with `-tags gstreamer`
- ✅ Player module loads and plays videos
- ✅ Convert preview shows video frames
- ✅ No crashes during basic playback
- ✅ Both systems use GStreamerPlayer backend

**Estimated Time**: 1-2 hours (mostly testing)
