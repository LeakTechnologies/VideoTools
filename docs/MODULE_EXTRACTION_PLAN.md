# Module Extraction Plan

This document outlines the strategy for extracting `package main` UI modules into `internal/app/modules/`.

## Why Extract Modules?

1. **Reduced root package size** - `main.go` is the entry point, not a dumping ground for UI code
2. **Clear boundaries** - Each module is self-contained with well-defined interfaces
3. **Testability** - Internal packages can be unit tested without the full Fyne UI
4. **Maintainability** - Developers can work on modules independently

## Extraction Pattern

### Phase 1: Structure
1. Create `internal/app/modules/{name}/` directory
2. Create `types.go` with:
   - `ModuleColor` constant
   - `Options` struct for `BuildView`
   - Shared types (if any)
3. Create `view.go` with `BuildView` function

### Phase 2: Logic Extraction
4. Identify UI builder functions (`show*View`, `build*View`)
5. Create callback interfaces for `appState` dependencies
6. Implement adapters in root `*_module.go`

### Phase 3: Cleanup
7. Remove old functions from root module
8. Keep only thin shims that delegate to internal package
9. Update imports and verify build

## Extraction Status

### Completed Extractions

| Module | Root File | Lines Removed | Internal Package |
|--------|-----------|--------------|------------------|
| About | `about_module.go` | ~300 | `internal/app/modules/about/` |
| Deps | `deps_dialog_module.go` | ~50 | `internal/app/modules/deps/` |
| Main Menu | `mainmenu_module.go` | ~400 | `internal/app/modules/mainmenu/` |
| Audio | `audio_module.go` | ~500 | `internal/app/modules/audio/` |
| Filters | `filters_module.go` | ~600 | `internal/app/modules/filters/` |
| Inspect | `inspect_module.go` | ~800 | `internal/app/modules/inspect/` |
| Thumbnail | `thumbnail_module.go` | ~400 | `internal/app/modules/thumbnail/` |
| Player | `player_module.go` | ~300 | `internal/app/modules/player/` |
| Enhancement | `enhancement_module.go` | ~200 | `internal/app/modules/enhancement/` |
| Compare | `compare_module.go` | ~500 | `internal/app/modules/compare/` |
| Trim | `trim_module.go` | ~200 | `internal/app/modules/trim/` |
| Queue | `queue_module.go` | ~100 | `internal/ui/queueview.go` |
| **Settings** | `settings_module.go` | ~600 | `internal/app/modules/settings/` |

### Pending Extractions (Priority Order)

#### 1. Subtitle Extraction (HIGH)
- **Root file**: `subtitles_module.go`
- **Lines**: ~1784 total
- **View code**: ~1200 lines in `showSubtitlesView` + `buildSubtitlesView`
- **appState dependencies**: 100+ references
- **Dependencies**: Whisper (python), OCR, FFmpeg subtitle filters
- **Complexity**: HIGH - many state interactions

#### 2. Upscale Extraction (MEDIUM)
- **Root file**: `upscale_module.go`
- **Lines**: ~993 total
- **View code**: ~600 lines
- **appState dependencies**: 244 references (HIGH coupling)
- **Dependencies**: Real-ESRGAN ncnn, RIFE ncnn, FFmpeg
- **Complexity**: MEDIUM-HIGH - complex AI pipeline wiring

#### 3. Merge Extraction (LOWER)
- **Root file**: `merge_module.go`
- **Lines**: ~1200 total
- **View code**: ~400 lines
- **appState dependencies**: ~80 references
- **Dependencies**: FFmpeg concat
- **Complexity**: MEDIUM - simpler than subtitles

#### 4. Snippet Extraction (LOWER)
- **Root file**: (embedded in main.go)
- **Lines**: ~800 lines
- **View code**: ~300 lines
- **appState dependencies**: ~50 references
- **Dependencies**: FFmpeg
- **Complexity**: MEDIUM - similar to merge

### Not Recommended for Extraction

| File | Reason |
|------|--------|
| `main.go` | Entry point, contains app initialization and entry functions |
| `convert_module.go` | ~3500 lines, heavily coupled - defer until other modules extracted |
| `author_module.go` | ~3000+ lines, complex DVD authoring - complex extraction |
| `rip_module.go` | ~700 lines, paired with author extraction |
| `fonts_embed.go` | `go:embed` constraint - must stay at root |
| `icons_embed.go` | `go:embed` constraint - must stay at root |
| `logo_embed.go` | `go:embed` constraint - must stay at root |

## Callback Pattern

For modules with heavy `appState` coupling, use the adapter pattern:

```go
// In internal/app/modules/{name}/types.go
type ViewCallbacks interface {
    Window() fyne.Window
    ShowMainMenu()
    ShowQueue()
    StatsBar() fyne.CanvasObject
    // ... other methods
}

// In root *module.go
type viewAdapter struct {
    s *appState
}

func (a *viewAdapter) Window() fyne.Window {
    return a.s.window
}

func (a *viewAdapter) ShowMainMenu() {
    a.s.showMainMenu()
}

// ... implement all interface methods

func (s *appState) showXxxView() {
    s.setContent(xxx.BuildView(&viewAdapter{s: s}))
}
```

## Verification Checklist

After each extraction:
- [ ] Build passes (`go build`)
- [ ] Module loads without errors
- [ ] All buttons/menus trigger correct actions
- [ ] Module switching works correctly
- [ ] State persists between visits
- [ ] No memory leaks (module-specific state cleaned up)

## Files That Must Stay at Root

Due to `go:embed` constraints:
- `main.go` - Application entry point
- `fonts_embed.go` - Embedded fonts
- `icons_embed.go` - Embedded icons
- `logo_embed.go` - Embedded logos

Platform shims (acceptable at root):
- `update_windows.go`
- `update_linux.go`
- `platform.go`

## Benefits of Completion

When all UI modules are extracted:
1. `main.go` becomes a thin orchestrator (~500 lines)
2. Each module is independently testable
3. Clear boundaries enable parallel development
4. Code navigation becomes easier
