# Persistent Video Context Design

## Overview
Videos loaded in any module remain in memory, allowing users to seamlessly work across multiple modules without reloading. This enables workflows like: load once → convert → generate thumbnails → apply filters → inspect metadata.

## User Experience

### Video Lifecycle
1. **Load**: User selects a video in any module (Convert, Filter, etc.)
2. **Persist**: Video remains loaded when switching between modules
3. **Clear**: Video is cleared either:
   - **Manual**: User clicks "Clear Video" button
   - **Auto** (optional): After successful task completion when leaving a module
   - **Replace**: Loading a new video replaces the current one

### UI Components

#### Persistent Video Info Bar
Display at top of application when video is loaded:
```
┌─────────────────────────────────────────────────────────────┐
│ 📹 example.mp4 | 1920×1080 | 10:23 | H.264 | [Clear] [↻]  │
└─────────────────────────────────────────────────────────────┘
```

Shows:
- Filename (clickable to show full path)
- Resolution
- Duration
- Codec
- Clear button (unload video)
- Reload button (refresh metadata)

#### Module Video Controls

Each module shows one of two states:

**When No Video Loaded:**
```
┌────────────────────────────────┐
│  [Select Video File]           │
│  or                            │
│  [Select from Recent ▼]        │
└────────────────────────────────┘
```

**When Video Loaded:**
```
┌────────────────────────────────┐
│ ✓ Using: example.mp4           │
│ [Use Different Video] [Clear]  │
└────────────────────────────────┘
```

### Workflow Examples

#### Multi-Operation Workflow
```
1. User opens Convert module
2. Loads "vacation.mp4"
3. Converts to H.265 → saves "vacation-h265.mp4"
4. Switches to Thumb module (vacation.mp4 still loaded)
5. Generates thumbnail grid → saves "vacation-grid.png"
6. Switches to Filter module (vacation.mp4 still loaded)
7. Applies color correction → saves "vacation-color.mp4"
8. Manually clicks "Clear" when done
```

#### Quick Comparison Workflow
```
1. Load video in Convert module
2. Test conversion with different settings:
   - H.264 CRF 23
   - H.265 CRF 28
   - VP9 CRF 30
3. Compare outputs in Inspect module
4. Video stays loaded for entire comparison session
```

## Technical Implementation

### State Management

#### Current appState Structure
```go
type appState struct {
    source       *videoSource    // Shared across all modules
    convert      convertConfig
    player       *player.Player
    // ... other module states
}
```

The `source` field is already global to the app state, so it persists across module switches.

#### Video Source Structure
```go
type videoSource struct {
    Path          string
    DisplayName   string
    Format        string
    Width         int
    Height        int
    Duration      float64
    VideoCodec    string
    AudioCodec    string
    Bitrate       int
    FrameRate     float64
    PreviewFrames []string
    // ... other metadata
}
```

### Module Integration

#### Loading Video in Any Module
```go
func loadVideoInModule(state *appState) {
    // Open file dialog
    file := openFileDialog()

    // Parse video metadata (ffprobe)
    source := parseVideoMetadata(file)

    // Set in global state
    state.source = source

    // Refresh UI to show video info bar
    state.showVideoInfoBar()

    // Update current module with loaded video
    state.refreshCurrentModule()
}
```

#### Checking for Loaded Video
```go
func buildModuleView(state *appState) fyne.CanvasObject {
    if state.source != nil {
        // Video already loaded
        return buildModuleWithVideo(state, state.source)
    } else {
        // No video loaded
        return buildModuleVideoSelector(state)
    }
}
```

#### Clearing Video
```go
func (s *appState) clearVideo() {
    // Stop any playback
    s.stopPlayer()

    // Clear source
    s.source = nil

    // Clean up preview frames
    if s.currentFrame != "" {
        os.RemoveAll(filepath.Dir(s.currentFrame))
    }

    // Reset module states (optional)
    s.resetModuleDefaults()

    // Refresh UI
    s.hideVideoInfoBar()
    s.refreshCurrentModule()
}
```

### Auto-Clear Options

Add user preference for auto-clear behavior:

```go
type Preferences struct {
    AutoClearVideo string // "never", "on_success", "on_module_switch"
}
```

**Options:**
- `never`: Only clear when user clicks "Clear" button
- `on_success`: Clear after successful operation when switching modules
- `on_module_switch`: Always clear when switching modules

### Video Info Bar Implementation

```go
func (s *appState) buildVideoInfoBar() fyne.CanvasObject {
    if s.source == nil {
        return container.NewMax() // Empty container
    }

    // File info
    filename := widget.NewLabel(s.source.DisplayName)
    filename.TextStyle = fyne.TextStyle{Bold: true}

    // Video specs
    specs := fmt.Sprintf("%dx%d | %s | %s",
        s.source.Width,
        s.source.Height,
        formatDuration(s.source.Duration),
        s.source.VideoCodec)
    specsLabel := widget.NewLabel(specs)

    // Clear button
    clearBtn := widget.NewButton("Clear", func() {
        s.clearVideo()
    })

    // Reload button (refresh metadata)
    reloadBtn := widget.NewButton("↻", func() {
        s.reloadVideoMetadata()
    })

    // Icon
    icon := widget.NewIcon(theme.MediaVideoIcon())

    return container.NewBorder(nil, nil,
        container.NewHBox(icon, filename),
        container.NewHBox(reloadBtn, clearBtn),
        specsLabel,
    )
}
```

### Recent Files Integration

Enhance with recent files list for quick access:

```go
func (s *appState) buildRecentFilesMenu() *fyne.Menu {
    items := []*fyne.MenuItem{}

    for _, path := range s.getRecentFiles() {
        path := path // Capture for closure
        items = append(items, fyne.NewMenuItem(
            filepath.Base(path),
            func() { s.loadVideoFromPath(path) },
        ))
    }

    return fyne.NewMenu("Recent Files", items...)
}
```

## Benefits

### User Benefits
- **Efficiency**: Load once, use everywhere
- **Workflow**: Natural multi-step processing
- **Speed**: No repeated file selection/parsing
- **Context**: Video stays "in focus" during work session

### Technical Benefits
- **Performance**: Single metadata parse per video load
- **Memory**: Shared video info across modules
- **Simplicity**: Consistent state management
- **Flexibility**: Easy to add new modules that leverage loaded video

## Migration Path

### Phase 1: Add Video Info Bar
- Implement persistent video info bar at top of window
- Show when `state.source != nil`
- Add "Clear" button

### Phase 2: Update Module Loading
- Check for `state.source` in each module's build function
- Show "Using: [filename]" when video is already loaded
- Add "Use Different Video" option

### Phase 3: Add Preferences
- Add auto-clear settings
- Implement auto-clear logic on module switch
- Add auto-clear on success option

### Phase 4: Recent Files
- Implement recent files tracking
- Add recent files dropdown in video selectors
- Persist recent files list

## Future Enhancements

### Multi-Video Support
For advanced users who want to work with multiple videos:
- Video tabs or dropdown selector
- "Pin" videos to keep multiple in memory
- Quick switch between loaded videos

### Batch Processing
Extend to batch operations on loaded video:
- Queue multiple operations
- Execute as single FFmpeg pass when possible
- Show operation queue in video info bar

### Workspace/Project Files
Save entire session state:
- Currently loaded video(s)
- Module settings
- Queued operations
- Allow resuming work sessions

## Implementation Checklist

- [ ] Design and implement video info bar component
- [ ] Add `clearVideo()` method to appState
- [ ] Update all module build functions to check for `state.source`
- [ ] Add "Use Different Video" buttons to modules
- [ ] Implement auto-clear preferences
- [ ] Add recent files tracking and menu
- [ ] Update Convert module (already partially implemented)
- [ ] Update other modules (Merge, Trim, Filters, etc.)
- [ ] Add keyboard shortcuts (Ctrl+W to clear video, etc.)
- [ ] Write user documentation
- [ ] Add tooltips explaining persistent video behavior
