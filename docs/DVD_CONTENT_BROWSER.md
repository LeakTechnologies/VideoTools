# DVD Content Browser — Rip Module Redesign

## Overview

Replace the current rip module's left-side InlineVideoPlayer pane with a rich content
browser that shows DVD titles as a scrollable list with animated cycling-still thumbnails,
plus a static menu preview with per-button toggles for selective remuxing.

## Current State

The rip module (`internal/app/modules/rip/view.go`) uses a 65/35 HSplit:
- **Left**: InlineVideoPlayer + title nav dropdown
- **Right**: Source, Format/Enrichment (checkboxes), Output, Status panels

The enrichment panel already has per-title checkboxes for multi-title discs, but they
are plain text labels with no visual preview.

## Architecture

### New Files

| File | Purpose |
|---|---|
| `internal/app/modules/rip/content_list.go` | `ContentBrowser` widget — scrollable title cards with cycling thumbnails |
| `internal/app/modules/rip/menu_preview.go` | `MenuPreview` widget — static menu frame render with toggle overlay |

### Modified Files

| File | Changes |
|---|---|
| `internal/app/modules/rip/view.go` | Replace playerPane with ContentBrowser + compact player; remove old title nav |
| `internal/app/modules/rip/types.go` | Add `MenuButton` type for menu toggle state |

### Dependency Chain

```
scan.go (ScanDisc) → types.go (DiscScanResult)
                    → content_list.go (ContentBrowser)
                    → menu_preview.go (MenuPreview)
                    → view.go (BuildView wiring)
```

---

## 1. ContentBrowser Widget (`content_list.go`)

### Struct

```go
type ContentBrowser struct {
    container     *fyne.Container
    titleList     *widget.List
    thumbCache    map[int][]*canvas.Image  // title number → cycling frames
    thumbTimers   map[int]*time.Timer      // per-title cycle timers
    selectedTitles map[int]bool
    scanResult    *DiscScanResult
    onSelect      func(titleNum int, selected bool)
    onPreview     func(titleNum int)        // click to preview in compact player

    mu sync.Mutex
}
```

### Layout

```
┌──────────────────────────────────────────┐
│  CONTENT BROWSER              [Select All]│  ← header bar (teal tint)
├──────────────────────────────────────────┤
│ ┌────┬─────────────────────────────────┐ │
│ │ 🖼  │ T01  1h 42m  ★ MAIN FEATURE   │ │  ← title card
│ │    │ 24 chapters · EN AC3 5.1       │ │
│ │    │ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ [✓]       │ │  ← selection toggle
│ └────┴─────────────────────────────────┘ │
│ ┌────┬─────────────────────────────────┐ │
│ │ 🖼  │ T02  3m 12s                    │ │  ← extra title
│ │    │ 1 chapter · EN AC3 2.0         │ │
│ │    │ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ [✓]       │ │
│ └────┴─────────────────────────────────┘ │
│ ┌────┬─────────────────────────────────┐ │
│ │ 🖼  │ T03  0m 08s                    │ │  ← menu/short title
│ │    │ 1 chapter                       │ │
│ │    │ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ [✓]       │ │
│ └────┴─────────────────────────────────┘ │
└──────────────────────────────────────────┘
```

### Cycling Thumbnail Extraction

When the scan result is loaded, for each title:

1. **Extract 5 keyframes** via ffmpeg:
   ```
   ffmpeg -ss <0.10*duration> -i <disc_path> -title <N> -frames:v 1 -vf scale=160:-1 -q:v 5 -f image2 -
   ffmpeg -ss <0.25*duration> -i <disc_path> -title <N> -frames:v 1 -vf scale=160:-1 -q:v 5 -f image2 -
   ffmpeg -ss <0.50*duration> -i <disc_path> -title <N> -frames:v 1 -vf scale=160:-1 -q:v 5 -f image2 -
   ffmpeg -ss <0.75*duration> -i <disc_path> -title <N> -frames:v 1 -vf scale=160:-1 -q:v 5 -f image2 -
   ffmpeg -ss <0.90*duration> -i <disc_path> -title <N> -frames:v 1 -vf scale=160:-1 -q:v 5 -f image2 -
   ```
   Use `-f dvdvideo` when available (SupportsDVDVideo()), fallback to generic avformat.

2. **Store as `[]fyne.Resource`** per title (PNG bytes from ffmpeg stdout → StaticResource).

3. **Cycle with a shared `time.Ticker`** (1.5s interval):
   - Each visible title card's `canvas.Image.Resource` is updated.
   - Cards off-screen (scrolled out of view) skip updates to save CPU.
   - Ticker is created once when ContentBrowser is built, stopped on disc clear.

4. **Fade transition**: Simple alpha blend between frames using `canvas.Image`.
   Since Fyne doesn't support per-image alpha, use a crossfade by overlapping
   two `canvas.Image` objects with `container.NewMax` and toggling visibility
   on a 200ms timer sub-tick. Alternatively, use the simpler approach of just
   swapping the resource (no crossfade) — the cycling stills already feel animated.

### Title Card Structure

Each card is a `fyne.CanvasObject` built from:

```go
func (cb *ContentBrowser) buildTitleCard(dt DiscTitle, isMain bool) fyne.CanvasObject {
    // Thumbnail: 80×60 canvas.Image with cycling resource
    // Right side: VBox with title label, metadata line, chapter/audio info
    // Far right: widget.Check bound to selectedTitles[dt.Number]
    // Background: clickable navy rectangle with grid color border
}
```

- **Thumbnail**: `canvas.Image` 80×60, `ImageFillContain`, dark background rectangle behind it
- **Title label**: Bold monospace "T01  1h 42m  ★ MAIN"
- **Info line**: "24 chapters · EN AC3 5.1, FR AC3 2.0"
- **Selection toggle**: `widget.Check` with i18n label, wired to `vs.selectedTitles`
- **Click handler**: Tapping the card body calls `onPreview(dt.Number)` → loads title into compact player

### Methods

```go
func NewContentBrowser() *ContentBrowser
func (cb *ContentBrowser) SetScanResult(result *DiscScanResult, sourcePath string)
func (cb *ContentBrowser) GetContainer() fyne.CanvasObject
func (cb *ContentBrowser) SetOnSelect(fn func(int, bool))
func (cb *ContentBrowser) SetOnPreview(fn func(int))
func (cb *ContentBrowser) SetSelected(titles map[int]bool)
func (cb *ContentBrowser) Stop()  // stops ticker, cancels pending ffmpeg goroutines
```

### Thumbnail Extraction Goroutine

```go
func (cb *ContentBrowser) extractThumbnails(dt DiscTitle, sourcePath string) {
    dur := dt.Duration
    if dur <= 0 { dur = 120.0 } // fallback 2 min
    positions := []float64{0.10, 0.25, 0.50, 0.75, 0.90}

    var frames []fyne.Resource
    for _, pos := range positions {
        t := dur * pos
        data, err := extractTitleFrame(sourcePath, dt.Number, t)
        if err != nil { continue }
        frames = append(frames, fyne.NewStaticResource(
            fmt.Sprintf("thumb_t%d_%.0f.png", dt.Number, pos), data))
    }
    if len(frames) == 0 { return }

    cb.mu.Lock()
    cb.thumbCache[dt.Number] = // canvas.Image slice
    cb.mu.Unlock()
}
```

`extractTitleFrame` uses `ffmpeg` with `-f dvdvideo -title <N> -ss <time>` when
available, or generic avformat otherwise. Output goes to a temp PNG file, read back,
then deleted.

---

## 2. MenuPreview Widget (`menu_preview.go`)

### Overview

When a disc is loaded and contains a menu VOB (VTS_XX_0.VOB), capture the first
frame and display it with toggle overlays beside detected menu buttons.

### Struct

```go
type MenuPreview struct {
    container    *fyne.Container
    menuImage    *canvas.Image
    menuFrame    []byte           // raw PNG of first menu frame
    buttons      []MenuButton     // parsed from VOB NAV packets or heuristic
    toggles      []*widget.Check  // one per button
    visible      bool
}

type MenuButton struct {
    Label  string
    X, Y   int       // position on menu frame
    Width  int
    Target string    // title number or command
    Enabled bool     // toggle state
}
```

### Frame Capture

```go
func (mp *MenuPreview) captureMenuFrame(videoTSPath string, vtsNum int) {
    menuVOB := filepath.Join(videoTSPath,
        fmt.Sprintf("VTS_%02d_0.VOB", vtsNum))
    // Extract first frame:
    // ffmpeg -i <menuVOB> -frames:v 1 -vf scale=320:-1 -q:v 3 -f image2 <tmpfile>.png
    // Read PNG bytes → menuFrame
}
```

### Button Detection (Heuristic)

Since full NAV packet parsing is complex, use a heuristic approach:

1. Extract the first frame (already done for display).
2. Look for the VIDEO_TS.VOB first frame (main menu) as an alternative.
3. Present buttons as a simple list beside the image — the user knows their
   DVD's menu structure.

For the initial implementation, display the menu frame image and a "Select titles
to include" list beside it (matching the existing title checkboxes but visually
integrated). The menu frame serves as visual context; the toggles remain the
per-title checkboxes from the enrichment panel.

### Future Enhancement

When the `dvd-menu-play` roadmap card lands (Native DVD Menu VM), the menu preview
can become interactive with NAV packet button highlighting and cursor position
tracking. For now, static frame + toggles.

### Layout

```
┌────────────────────────────────────────────┐
│  DISC MENU                    [Preserve ✓] │
├────────────────────────────────────────────┤
│ ┌──────────────────┐  ┌──────────────────┐ │
│ │                  │  │ [✓] Title 1      │ │
│ │   Menu Frame     │  │ [✓] Title 2      │ │
│ │   (static PNG)   │  │ [ ] Title 3      │ │
│ │                  │  │ [✓] Title 4      │ │
│ └──────────────────┘  └──────────────────┘ │
└────────────────────────────────────────────┘
```

---

## 3. View Modifications (`view.go`)

### Layout Change

**Before** (current):
```
┌─────────────────────────────────────────────────┐
│ [topBar]                                        │
├──────────────────────┬──────────────────────────┤
│                      │ [Source]                 │
│  InlineVideoPlayer   │ [Format + Enrichment]    │
│  + title nav         │ [Output]                 │
│                      │ [Status]                 │
├──────────────────────┴──────────────────────────┤
│ [bottomBar: Add to Queue | Open in Player | Rip]│
├─────────────────────────────────────────────────┤
│ [LOG]                                           │
└─────────────────────────────────────────────────┘
```

**After** (new):
```
┌─────────────────────────────────────────────────┐
│ [topBar]                                        │
├──────────────────────┬──────────────────────────┤
│                      │ [Source]                 │
│  Compact Player      │ [Format]                 │
│  (30% of left pane)  │                          │
│──────────────────────│ [Content Selection]      │
│                      │  (moved from Enrichment) │
│  ContentBrowser      │                          │
│  (70% of left pane)  │ [Output]                 │
│                      │ [Status]                 │
├──────────────────────┴──────────────────────────┤
│ [bottomBar: Add to Queue | Open in Player | Rip]│
├─────────────────────────────────────────────────┤
│ [LOG]                                           │
└─────────────────────────────────────────────────┘
```

### Specific Changes to `view.go`

1. **Replace playerPane construction** (lines 311-382):
   - Remove old title nav dropdown (prevTitleBtn, nextTitleBtn, titleNavSelect)
   - Create compact player (smaller InlineVideoPlayer)
   - Create ContentBrowser below it
   - Wire ContentBrowser.onPreview → load title in compact player
   - Wire ContentBrowser.onSelect → update vs.selectedTitles

2. **Remove title checkboxes from rebuildEnrich** (lines 819-843):
   - The title checkboxes move to ContentBrowser
   - rebuildEnrich keeps chapter/audio/subtitle/region controls only

3. **Add MenuPreview section**:
   - When scan result has multiple VTS sets with menu VOBs, show MenuPreview
   - Place it between Format and Output boxes
   - Wire "Preserve menus" toggle to MenuPreview visibility

4. **Update mainSplit offset**:
   - Change from 0.65 to 0.55 (content browser is denser than video player)

5. **Add select-all / deselect-all** in ContentBrowser header

6. **Add `rebuildContentBrowser`** callback (parallel to rebuildEnrich):
   - Called when scan completes
   - Calls `contentBrowser.SetScanResult(result, sourcePath)`

---

## 4. Type Additions (`types.go`)

```go
// MenuButton describes one interactive button on a DVD menu screen.
type MenuButton struct {
    Label   string
    X, Y    int
    Width   int
    Height  int
    Target  string // title number or command identifier
    Enabled bool   // user toggle state
}
```

---

## 5. i18n Keys

Add to `internal/i18n/strings.go` + `en_ca.go`:

```go
RipContentBrowser  = "Content Browser"
RipSelectAll       = "Select All"
RipDeselectAll     = "Deselect All"
RipMenuPreview     = "Disc Menu"
RipPreserveMenus   = "Preserve menus"
RipMainFeature     = "★ Main Feature"
RipChapters        = "%d chapters"
RipAudioTracks     = "%d audio"
RipClickToPreview  = "Click to preview"
RipNoMenuFound     = "No menu VOB found"
```

---

## 6. Performance Considerations

- **Thumbnail extraction**: Run in background goroutines, one per title.
  Use a worker pool (max 3 concurrent ffmpeg processes) to avoid I/O saturation.
- **Cycling ticker**: Single `time.Ticker(1500ms)` shared across all visible cards.
  Only update `canvas.Image.Resource` for cards within the scroll viewport.
- **Menu frame capture**: Single ffmpeg call, runs once on disc load.
- **Cleanup**: `ContentBrowser.Stop()` cancels all pending goroutines and stops
  the ticker. Called from `clearISOBtn` handler and view teardown.

---

## 7. Testing Checklist

- [ ] Load a single-title DVD → shows one card with thumbnail, no checkboxes
- [ ] Load a multi-title DVD → shows all titles with checkboxes, thumbnails cycle
- [ ] Click a title card → loads that title in the compact player
- [ ] Toggle checkbox → updates vs.selectedTitles, reflected in "Add to Queue" jobs
- [ ] "Select All" / "Deselect All" buttons work
- [ ] Main feature marked with star, auto-selected by default
- [ ] Extra titles show correct metadata (duration, chapters, audio)
- [ ] Menu preview shows when menu VOB exists, hides when absent
- [ ] "Preserve menus" checkbox toggles menu preview visibility
- [ ] Clear disc → stops all timers, clears cache, resets UI
- [ ] ISO loading → thumbnails extracted via dvdvideo demuxer
- [ ] VIDEO_TS folder loading → thumbnails extracted via concat or dvdvideo
- [ ] No-scan path (direct drop without IFO) → graceful fallback (no thumbnails)
- [ ] CPU usage reasonable during thumbnail cycling (measure with 20+ titles)
- [ ] Player minimize toggle still works with new layout
- [ ] Log collapse/expand still works
- [ ] All existing enrichment options (chapters, audio, subs, region) still function

---

## 8. Files Changed (Summary)

| File | Action | Lines (est.) |
|---|---|---|
| `internal/app/modules/rip/content_list.go` | **NEW** | ~350 |
| `internal/app/modules/rip/menu_preview.go` | **NEW** | ~200 |
| `internal/app/modules/rip/view.go` | MODIFY | ~80 changes |
| `internal/app/modules/rip/types.go` | MODIFY | ~15 additions |
| `internal/i18n/strings.go` | MODIFY | ~12 additions |
| `internal/i18n/en_ca.go` | MODIFY | ~12 additions |
| `docs/roadmap.html` | MODIFY | 1 new card |

---

## 9. Roadmap Card

```javascript
{ id: 'content-browser', title: 'DVD Content Browser',
  desc: 'Replace player pane with scrollable title cards showing cycling-still thumbnails, metadata, and selection toggles. Static menu frame preview with per-button preserve toggles.',
  status: 'planned', cycle: 'dev56',
  files: ['internal/app/modules/rip/content_list.go', 'internal/app/modules/rip/menu_preview.go',
          'internal/app/modules/rip/view.go', 'internal/app/modules/rip/types.go'],
  deps: ['dvdvideo', 'rip-player'],
  docs: ['docs/DVD_CONTENT_BROWSER.md'] },
```
