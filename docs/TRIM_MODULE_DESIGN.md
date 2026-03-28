# Trim Module Design

## Overview
The Trim module allows users to cut portions of video files using visual keyframe markers. Users can set In/Out points on the timeline and preview the trimmed segment before processing.

## Core Features

### 1. Visual Timeline Editing ✅
- [x] Load video with native VideoPlayer
- [x] Set **In Point** (start of keep region) - Press `I` or click button
- [x] Set **Out Point** (end of keep region) - Press `O` or click button
- [x] Visual markers on timeline showing trim region (orange/red brackets)
- [x] Scrub through video to find exact frames

### 2. Keyframe Controls ✅
```
[In Point] ←────────────────→ [Out Point]
   0:10              Keep Region              2:45
   ═══════════════════════════════════════════
   ^Orange                ^Red-Orange
```

### 3. Frame-Accurate Navigation ✅
- `←` / `→` - Step backward/forward one frame
- `Shift+←` / `Shift+→` - Jump 1 second
- `I` - Set In Point at current position
- `O` - Set Out Point at current position
- `Space` - Play/Pause
- `C` - Clear all keyframes
- `P` - Preview trimmed region

### 4. Multiple Trim Modes ✅

#### Mode 1: Keep Region (Default) ✅
Keep video between In and Out points, discard rest.
```
Input:  [─────IN════════OUT─────]
Output:       [════════]
```

#### Mode 2: Cut Region ✅
Remove video between In and Out points, keep rest.
```
Input:  [─────IN════════OUT─────]
Output: [─────]        [─────]
```

#### Mode 3: Multiple Segments (Advanced) ⬜
Define multiple keep/cut regions using segment list.

## UI Layout ✅

```
┌─────────────────────────────────────────────┐
│ < TRIM                                       │ ← Cyan header bar
├─────────────────────────────────────────────┤
│                                              │
│  ┌───────────────────────────────────────┐  │
│  │        Video Player (native)          │  │
│  │                                       │  │
│  │  [Timeline with In/Out markers]      │  │
│  │  ────I═══════════════O────────       │  │
│  │  ════════════════════════════════     │  │
│  │  (green shaded region between marks) │  │
│  │                                       │  │
│  │  [◄] [Set In] [Set Out] [►] [▶P]   │  │
│  └───────────────────────────────────────┘  │
│                                              │
│  Trim Mode: ○ Keep Region  ○ Cut Region     │
│  Output:   ○ Smart Copy  ○ Re-encode        │
│                                              │
│  In Point:  00:01:23.456                    │
│  Out Point: 00:04:56.789                    │
│  Duration:  00:03:33.333                     │
│                                              │
│  [Clear Points]        [Add to Queue]       │
│                                              │
└─────────────────────────────────────────────┘
```

## VideoPlayer API (Implemented)

### Trim Marker Methods
```go
player.SetInPoint(t float64)    // Set In marker position
player.SetOutPoint(t float64)   // Set Out marker position
player.GetInPoint() float64     // Get In marker position
player.GetOutPoint() float64    // Get Out marker position
player.ClearTrimMarkers()       // Clear all trim markers
```

### Keyboard Shortcuts (Implemented)
```go
KeyI     -> Set In Point
KeyO     -> Set Out Point
KeyC     -> Clear Keyframes
KeyP     -> Preview Trimmed Region
KeyLeft  -> Step back 1 frame
KeyRight -> Step forward 1 frame
Space    -> Play/Pause
```

## FFmpeg Integration

### Keep Region Mode
```bash
ffmpeg -i input.mp4 -ss 00:01:23.456 -to 00:04:56.789 -c copy output.mp4
```

### Cut Region Mode (Complex filter)
```bash
ffmpeg -i input.mp4 \
  -filter_complex "[0:v]split[v1][v2]; \
                   [v1]trim=start=0:end=83.456[v1t]; \
                   [v2]trim=start=296.789[v2t]; \
                   [v1t][v2t]concat=n=2:v=1:a=0[outv]" \
  -map [outv] output.mp4
```

### Smart Copy (Fast) ✅
- Use `-c copy` when no re-encoding needed
- Only works at keyframe boundaries
- [x] Show warning if In/Out not at keyframes (confirmation dialog)

## Workflow ✅

1. [x] **Load Video** - Drag video or use Load button
2. [x] **Navigate** - Scrub or use keyboard to find start point
3. [x] **Set In** - Press `I` or click "Set In" button
4. [x] **Find End** - Navigate to end of region to keep
5. [x] **Set Out** - Press `O` or click "Set Out" button
6. [x] **Preview** - Press `P` or click "Preview" button
7. [x] **Queue** - Click "Add to Queue" to process

## Technical Notes

### Precision Considerations ✅
- [x] Frame-accurate requires seeking to exact frame boundaries
- [x] Display timestamps with millisecond precision (HH:MM:SS.mmm)
- [x] VideoPlayer handles fractional frame positions

### Performance ✅
- [x] Preview uses native playback (no re-encode)
- [x] Frame caching for smooth scrubbing

## Future Enhancements ⬜

- [x] Multiple trim regions in single operation (2026-03-28)
- [x] Split to clips vs embed chapters (2026-03-28)
- [ ] Batch trim multiple files with same In/Out offsets
- [ ] Save trim presets (e.g., "Remove first 30s and last 10s")
- [ ] Visual waveform for audio-based trimming
- [ ] Chapter-aware trimming (trim to chapter boundaries)

## Multi-Segment Trim (Implemented 2026-03-28)

### Overview
The timeline widget supports multiple trim segments. Users can either:
1. **Split to Clips** - Each segment becomes a separate output file
2. **Keep Intact + Chapters** - Full video with chapter markers at segment boundaries

### UI Components

```
┌─────────────────────────────────────────────────────┐
│ Timeline: [===SEGMENT 1===] [===SEGMENT 2===]      │
│        │IN1─────OUT1│  │IN2─────OUT2│              │
│ ───────██████████████  ███████████████───────────── │
│        ^                          ^                │
│    Green Handle              Green Handle          │
│    (draggable)              (draggable)            │
│                                                     │
│ [+ Add Segment]  [Clear All]                       │
│                                                     │
│ Output Mode: ○ Split to Clips  ● Keep + Chapters │
└─────────────────────────────────────────────────────┘
```

### Data Structure
```go
type TrimSegment struct {
    InPoint  float64 // Start time in seconds
    OutPoint float64 // End time in seconds
}

type TrimState struct {
    Segments   []TrimSegment    // Multiple trim regions
    OutputMode string           // "clips" or "chapters"
}
```

### Timeline Widget API
```go
// Create timeline with single segment (backward compatible)
timeline := ui.NewTrimTimeline(duration)

// Add new segment (for multi-segment mode)
timeline.AddSegment(inPoint, outPoint)

// Remove segment by index
timeline.RemoveSegment(index)

// Get all segments
segments := timeline.GetSegments()

// Set output mode
timeline.SetOutputMode("clips")   // Split to multiple files
timeline.SetOutputMode("chapters") // Single file with chapters
```

### Implementation Details

#### Timeline Widget (`internal/ui/components.go`)
- `TrimTimeline` widget with draggable handles
- Supports multiple segment pairs (in/out handles)
- Visual differentiation: green handles for in-points, red for out-points
- Position indicator (blue line) shows current playback position

#### Trim Module (`internal/app/modules/trim/view.go`)
- Extended `trimState` to hold `[]TrimSegment` instead of single in/out
- Segment list UI for add/remove operations
- Output mode toggle (clips vs chapters)

#### FFmpeg Output

**Split to Clips (multiple files):**
```bash
# Segment 1
ffmpeg -i input.mp4 -ss IN1 -to OUT1 -c copy output_part1.mp4
# Segment 2  
ffmpeg -i input.mp4 -ss IN2 -to OUT2 -c copy output_part2.mp4
```

**Keep Intact + Chapters (single file):**
```bash
# Extract segments, concat with chapter metadata
ffmpeg -i input.mp4 -ss IN1 -to OUT1 -c copy -metadata title="Chapter 1" part1.mp4
ffmpeg -i input.mp4 -ss IN2 -to OUT2 -c copy -metadata title="Chapter 2" part2.mp4
# Concat with chapter file
ffmpeg -f concat -safe 0 -i chapters.txt -c copy output_with_chapters.mp4
```

### Testing Checklist

- [ ] Add first segment - handles appear on timeline
- [ ] Add second segment - two handle pairs visible
- [ ] Drag in-point handle - updates segment start
- [ ] Drag out-point handle - updates segment end  
- [ ] Remove segment - timeline updates
- [ ] Select "Split to Clips" - creates multiple jobs
- [ ] Select "Keep + Chapters" - creates single job with chapter metadata
- [ ] Clear all - resets to single full-video segment
- [ ] Preview plays through all segments in order

## Module Color
**Cyan** - #44DDFF (already defined in modulesList)
