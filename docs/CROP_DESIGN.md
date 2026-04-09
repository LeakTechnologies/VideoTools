# Frame Cropping Design

## Overview

Frame cropping removes unwanted borders (black bars, letterboxing, pillarboxing) from video. This is essential when converting between aspect ratios or working with source material that has hardcoded bars.

## Problem

- Users need to remove black bars from videos
- Auto-detection of crop boundaries is needed
- Manual adjustment should be available for fine-tuning
- Crop settings must be saved and reused

## Design Vision

### Three-step Crop Workflow

1. **Auto-detect** — Run crop detection to find black bars
2. **Preview** — Show detected crop region overlaid on video
3. **Adjust** — Allow manual tweaking if auto-detect is wrong

---

## UI Layout

### Crop Panel in Convert Module

```
┌─────────────────────────────────────────┐
│ Crop Settings                           │
├─────────────────────────────────────────┤
│ Mode: [○ Auto  ○ Manual  ○ Off]         │
├─────────────────────────────────────────┤
│ [Detect Crop] button                    │
├─────────────────────────────────────────┤
│ Detected: Top: 0  Bottom: 0             │
│            Left: 0  Right: 0            │
├─────────────────────────────────────────┤
│ Manual Adjust:                          │
│ Top:    [====●========] 0               │
│ Bottom: [==========●=] 0               │
│ Left:   [==========●=] 0               │
│ Right:  [==========●=] 0               │
├─────────────────────────────────────────┤
│ Output Size: 1920x1080 (16:9)           │
│ Cropped:  1920x1040 (black bars: 40px)  │
├─────────────────────────────────────────┤
│ [Preview Crop] button                   │
└─────────────────────────────────────────┘
```

### Visual Crop Selector (Alternative)

```
┌─────────────────────────────────────────┐
│                                        │
│   ┌─────────────────────────────┐     │
│   │ ═══════════════════════════ │ ← Top crop handle
│ L │║                           ║│R   │
│   │║     VIDEO CONTENT AREA   ║│   │
│   │║                           ║│   │ ← Draggable crop region
│   │╚═══════════════════════════╝│ ← Bottom crop handle
│                                        │
│   T←──────── width ────────→B        │
└─────────────────────────────────────────┘

L = Left crop (adjustable)
R = Right crop (adjustable)
T = Top crop (adjustable)
B = Bottom crop (adjustable)
```

### Aspect Ratio Overlay

When the output aspect ratio differs from the source, display a **white outline** over the video preview showing the target output dimensions. This helps users understand what the final output will look like.

```
┌─────────────────────────────────────────┐
│  Source Video: 1920x1080 (16:9)        │
│  Output:      1920x800  (2.40:1)        │
│                                        │
│  ┌─────────────────────────────────┐   │
│  │▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓│ ← Black bars (will be cropped)
│  ├─────────────────────────────────┤   │
│  │                                 │   │
│  │   ┌───────────────────────┐     │   │
│  │   │   WHITE OUTLINE      │     │ ← Output aspect ratio
│  │   │   ════════════════════│     │   (2.40:1)
│  │   │                       │     │
│  │   │   VIDEO CONTENT      │     │
│  │   │                       │     │
│  │   └───────────────────────┘     │
│  │                                 │   │
│  ├─────────────────────────────────┤   │
│  │▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓│ ← Black bars (will be cropped)
│  └─────────────────────────────────┘   │
│                                        │
│  Crop: Top=140, Bottom=140, Left=0, Right=0 │
└─────────────────────────────────────────┘

Key:
- ▓▓▓ = Black bars (source letterboxing)
- ─── = White outline (output aspect ratio boundary)
- ║║  = Active video region after crop
```

### Draggable Crop Region

Users can click and drag the white outline to position the crop region:
- **Drag center** — Move entire crop region
- **Drag edge** — Resize that edge
- **Double-click** — Reset to center

### Crop Region Interaction

| Action | Behavior |
|--------|----------|
| Click + Drag center | Move entire crop region |
| Click + Drag edge | Resize from that edge |
| Scroll wheel | Nudge crop position |
| Double-click | Reset to center |
| Right-click | Show crop preset menu |

### Aspect Ratio Presets

Common aspect ratios for quick selection:

| Ratio | Decimal | Common Name | Use Case |
|-------|---------|-------------|----------|
| 21:9 | 2.33:1 | Ultra-wide | Gaming monitors, film |
| 32:9 | 3.56:1 | Super-wide | Triple-monitor setups |
| 48:9 | 5.33:1 | Triple-wide | Professional displays |
| 16:9 | 1.78:1 | HDTV | YouTube, streaming |
| 2.39:1 | 2.39:1 | Anamorphic | CinemaScope |
| 2.35:1 | 2.35:1 | Cinemascope | Classic widescreen |
| 4:3 | 1.33:1 | SDTV | Legacy TV |
| 1:1 | 1.00:1 | Square | Instagram |
| 9:16 | 0.56:1 | Vertical | TikTok, short-form |
```

---

## FFmpeg Implementation

### Crop Detection

Use `cropdetect` filter to auto-detect black bars:

```bash
ffmpeg -i input.mp4 -vf "cropdetect=limit=10:round=2" -f null -
```

The filter outputs detection info to stderr, parse for `crop=` values.

### Crop Application

Apply crop in FFmpeg:

```go
func buildCropFilter(cfg cropConfig) string {
    if !cfg.Enabled {
        return ""
    }

    // crop=w:h:x:y
    return fmt.Sprintf("crop=%d:%d:%d:%d",
        cfg.Width,
        cfg.Height,
        cfg.Left,
        cfg.Top)
}
```

### Config Structure

```go
type cropConfig struct {
    Enabled   bool   // true to enable crop
    Mode      string // "auto", "manual", "off"
    AutoTop   int    // detected top crop
    AutoBottom int   // detected bottom crop
    AutoLeft  int    // detected left crop
    AutoRight int    // detected right crop
    ManualTop   int  // manual override
    ManualBottom int  // manual override
    ManualLeft  int   // manual override
    ManualRight int   // manual override
}
```

---

## Implementation Details

### Step 1: Auto-detect

When user clicks "Detect Crop":
1. Run FFmpeg with `cropdetect` filter
2. Parse output for crop values (take mode/median)
3. Display detected values in UI
4. Auto-apply if user confirms

### Step 2: Manual Adjustment

Sliders for each edge:
- Range: 0 to (video dimension / 2)
- Step: 2 pixels (DVD-aligned)
- Live preview updates as sliders move

### Step 3: Output Calculation

Show calculated output size:
```
Input:  1920x1080 (16:9)
Crop:   Top=40, Bottom=40, Left=0, Right=0
Output: 1920x1000 (16:9 → 1.91:1)
```

### Step 4: Preview

Show cropped frame in player before encoding.

---

## Edge Cases

| Case | Handling |
|------|----------|
| No black bars detected | Show message "No crop needed" |
| Uneven bars detected | Use maximum of top/bottom or left/right |
| Already cropped video | Warn "Video may already be cropped" |
| Mismatched aspect ratios | Allow but warn about distortion |

---

## Files to Modify

### Convert Module
- `main.go` — Add crop panel to Convert view
- `internal/convert/types.go` — Add crop config fields

### UI Components
- `internal/ui/components.go` — Crop control widgets
- `internal/app/modules/convert/view.go` — Build crop tab

### FFmpeg Integration
- `internal/ffmpeg/filters.go` — cropdetect wrapper
- `internal/convert/executor.go` — Apply crop in pipeline

### i18n Strings
- `internal/i18n/strings.go` — Add crop labels

---

## Testing Checklist

- [ ] Auto-detect finds black bars on letterboxed video
- [ ] Auto-detect finds pillarboxed video
- [ ] Manual sliders adjust crop correctly
- [ ] Output dimensions update in real-time
- [ ] Crop preview shows correct region
- [ ] Crop disabled (off) produces uncropped output
- [ ] Queue preserves crop settings

---

## Future Enhancements

1. **Aspect ratio snap** — Snap to common ratios (16:9, 4:3, 2.35:1)
2. **Region selector** — Click-drag to select crop region visually
3. **Batch crop** — Apply same crop to multiple files
4. **Crop profiles** — Save/load crop presets

---

## Reference

- Reference: FFmpeg crop filter documentation

---

*Last updated: 2026-04-09*
*Status: Ready for implementation*