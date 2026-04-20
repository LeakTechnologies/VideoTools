# Dual Player Architecture

VideoTools uses a dual-player pattern for modules that require comparing source to processed output (Upscale, Filters, Compare).

## Architecture

### Core Components

```
┌─────────────────────────────────────────────────────┐
│                 SplitView Widget                  │
│  ┌─────────────────┬─────────────────┐            │
│  │   Left Pane    │   Right Pane   │            │
│  │   (Source)    │   (Output)    │            │
│  └─────────────────┴─────────────────┘            │
└─────────────────────────────────────────────────────┘
```

Each pane contains its own `media.Engine` instance:
- **Left engine**: Direct source decode, no processing
- **Right engine**: Processed output (via filter chain or AI upscaling)

### Synchronization

The two engines must stay synchronized during playback:
- Same frame timing, seek position
- Shared play/pause state
- Clock master is typically the left (source) engine

### SplitView Widget

Located in `internal/media/view.go`:
```go
type SplitView struct {
    leftFrame  *image.RGBA
    rightFrame *image.RGBA
    dividerPos float32  // 0.0 - 1.0
}
```

Methods:
- `SetFrames(left, right *image.RGBA)` - Update both panes
- `SetDivider(pos float32)` - Move split position
- `SetOnDividerMove(cb func(float32))` - Drag handler

## Module Implementation

### Compare Module (Complete)

Already implements dual-player in `internal/app/modules/compare/`:
- Uses two independent `media.Engine` instances
- Each engine runs its own demux/decode loop
- `SplitView` for side-by-side display
- No real-time processing (just two different source files)

### Filters Module (Single Player)

Currently uses single-player preview. Dual-player implementation would require:
1. FFmpeg filter chain applied in real-time (CPU intensive)
2. Or: on-demand preview rendering

### Upscale Module (Single Player)

Currently uses single-player preview. Dual-player implementation would require:
1. AI model applied per-frame (not feasible real-time)
2. Or: periodic preview snapshots during conversion
3. Or: preview of first N seconds

## Real-Time Processing Challenges

### Filter Chain (FFmpeg)

Simple filters (brightness, contrast, saturation) can run in real-time:
```bash
ffmpeg -i input.mp4 -vf "eq=brightness=0.1" -f rawvideo -
```

Complex filters or filter chains may not achieve real-time on consumer hardware.

### AI Upscaling

AI models (Real-ESRGAN, Real-CUGAN, RIFE) are too slow for real-time:
- Single-frame processing: 100ms - 500ms per frame
- 24fps playback: 41ms max per frame
- GPU required, still not 1:1

## Preview Strategies

### Strategy 1: Real-Time (Simple Filters Only)

For lightweight filters, apply FFmpeg filter chain directly:
```go
engine2.SetFilterChain("eq=brightness=0.1:contrast=1.2")
```

Requirements:
- Filter chain must be computationally cheap
- Machine must have sufficient CPU

### Strategy 2: On-Demand Preview (Recommended)

When user scrubs the seek bar, render ~5 seconds of processed output:
1. User moves seek bar to position
2. System renders 5-second segment starting at that position
3. Processed output plays in right pane while user watches
4. Click "Refresh" to re-render with new settings

This is how Topaz handles it - render segment on seek, not real-time.

Implementation:
```go
// On seek/scrub action
func onSeek(position float64) {
    // Render 5-second segment at position
    go func() {
        outputPath := renderSegment(src.Path, position, 5*time.Second, filterChain)
        rightEngine.Close()
        rightEngine.Open(outputPath)
        rightEngine.Seek(0)
        rightEngine.Start()
    }()
}
```

### Strategy 3: Segment Preview

User selects "Preview 5 seconds":
1. Process first 5 seconds through filter/AI
2. Display segment in right pane
3. User can seek within segment

### Strategy 4: Post-Conversion Preview

After conversion completes:
1. Load output file in right pane
2. Compare side-by-side in fullscreen

## Implementation Pattern

```go
// Two engines → SplitView
leftEngine := media.NewEngine()
rightEngine := media.NewEngine()

// Left: direct source
leftEngine.Open(src.Path)
leftEngine.Start()

// Right: filtered output (simplest: no processing for now)
rightEngine.Open(src.Path)
rightEngine.Start()

// Sync: use left as master
go func() {
    for {
        leftFrame, _ := leftEngine.NextFrame()
        rightFrame, _ := rightEngine.NextFrame() // or processed
        splitView.SetFrames(leftFrame, rightFrame)
        time.Sleep(time.Duration(int(1000/frameRate)) * time.Millisecond)
    }
}()
```

## Known Limitations

1. **Dual-engine resource usage**: Two FFmpeg instances = ~2x CPU/memory
2. **No real-time AI**: AI upscaling cannot run at playback framerates
3. **Seek sync issues**: Seeking one side may cause frame mismatch
4. **Audio**: Only left (source) audio plays; right is filtered output

## Files to Modify

For Filters/Upscale dual-player:
- `internal/app/modules/filters/view.go` - Add second player
- `internal/app/modules/upscale/view.go` - Add second player  
- Add filter chain parameter to `media.Engine`
- Update SplitView to handle frame updates

## Reference Implementation

Compare module (`internal/app/modules/compare/fullscreen_native.go`) shows the pattern:
- Two `media.Engine` instances
- `media.NewSplitView()` for display
- Synchronized frame extraction in goroutine