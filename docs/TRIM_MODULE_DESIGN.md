# Trim Module Design

## Overview
The Trim module allows users to cut portions of video files using visual keyframe markers. Users can set In/Out points on the timeline and preview the trimmed segment before processing.

## Core Features

### 1. Visual Timeline Editing
- Load video with VT_Player
- Set **In Point** (start of keep region) - Press `I` or click button
- Set **Out Point** (end of keep region) - Press `O` or click button
- Visual markers on timeline showing trim region
- Scrub through video to find exact frames

### 2. Keyframe Controls
```
[In Point] ←────────────────→ [Out Point]
   0:10              Keep Region              2:45
   ═══════════════════════════════════════════
```

### 3. Frame-Accurate Navigation
- `←` / `→` - Step backward/forward one frame
- `Shift+←` / `Shift+→` - Jump 1 second
- `I` - Set In Point at current position
- `O` - Set Out Point at current position
- `Space` - Play/Pause
- `C` - Clear all keyframes

### 4. Multiple Trim Modes

#### Mode 1: Keep Region (Default)
Keep video between In and Out points, discard rest.
```
Input:  [─────IN════════OUT─────]
Output:       [════════]
```

#### Mode 2: Cut Region
Remove video between In and Out points, keep rest.
```
Input:  [─────IN════════OUT─────]
Output: [─────]        [─────]
```

#### Mode 3: Multiple Segments (Advanced)
Define multiple keep/cut regions using segment list.

## UI Layout

```
┌─────────────────────────────────────────────┐
│ < TRIM                                       │ ← Cyan header bar
├─────────────────────────────────────────────┤
│                                              │
│  ┌───────────────────────────────────────┐  │
│  │        Video Player (VT_Player)       │  │
│  │                                       │  │
│  │  [Timeline with In/Out markers]      │  │
│  │  ────I═══════════════O────────       │  │
│  │                                       │  │
│  │  [Play] [Pause] [In] [Out] [Clear]   │  │
│  └───────────────────────────────────────┘  │
│                                              │
│  Trim Mode: ○ Keep Region  ○ Cut Region     │
│                                              │
│  In Point:  00:01:23.456  [Set In]  [Clear] │
│  Out Point: 00:04:56.789  [Set Out] [Clear] │
│  Duration:  00:03:33.333                     │
│                                              │
│  Output Settings:                            │
│  ┌─────────────────────────────────────┐    │
│  │ Format: [Same as source ▼]          │    │
│  │ Re-encode: [ ] Smart copy (fast)    │    │
│  │ Quality: [Source quality]            │    │
│  └─────────────────────────────────────┘    │
│                                              │
│  [Preview Trimmed] [Add to Queue]           │
│                                              │
└─────────────────────────────────────────────┘
                                          ← Cyan footer bar
```

## VT_Player API Requirements

### Required Methods
```go
// Keyframe management
player.SetInPoint(position time.Duration)
player.SetOutPoint(position time.Duration)
player.GetInPoint() time.Duration
player.GetOutPoint() time.Duration
player.ClearKeyframes()

// Frame-accurate navigation
player.StepForward()  // Advance one frame
player.StepBackward() // Go back one frame
player.GetCurrentTime() time.Duration
player.GetFrameRate() float64

// Visual feedback
player.ShowMarkers(in, out time.Duration) // Draw on timeline
```

### Required Events
```go
// Keyboard shortcuts
- OnKeyPress('I') -> Set In Point
- OnKeyPress('O') -> Set Out Point
- OnKeyPress('→') -> Step Forward
- OnKeyPress('←') -> Step Backward
- OnKeyPress('Space') -> Play/Pause
- OnKeyPress('C') -> Clear Keyframes
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

### Smart Copy (Fast)
- Use `-c copy` when no re-encoding needed
- Only works at keyframe boundaries
- Show warning if In/Out not at keyframes

## Workflow

1. **Load Video** - Drag video onto Trim tile or use Load button
2. **Navigate** - Scrub or use keyboard to find start point
3. **Set In** - Press `I` or click "Set In" button
4. **Find End** - Navigate to end of region to keep
5. **Set Out** - Press `O` or click "Set Out" button
6. **Preview** - Click "Preview Trimmed" to see result
7. **Queue** - Click "Add to Queue" to process

## Technical Notes

### Precision Considerations
- Frame-accurate requires seeking to exact frame boundaries
- Display timestamps with millisecond precision (HH:MM:SS.mmm)
- VT_Player must handle fractional frame positions
- Consider GOP (Group of Pictures) boundaries for smart copy

### Performance
- Preview shouldn't require full re-encode
- Show preview using VT_Player with constrained timeline
- Cache preview segments for quick playback testing

## Future Enhancements
- Multiple trim regions in single operation
- Batch trim multiple files with same In/Out offsets
- Save trim presets (e.g., "Remove first 30s and last 10s")
- Visual waveform for audio-based trimming
- Chapter-aware trimming (trim to chapter boundaries)

## Module Color
**Cyan** - #44DDFF (already defined in modulesList)
