# VideoTools Feature Extensions

## Scope Note

VideoTools is a **converter and enhancement tool** built on FFmpeg. It is NOT a non-linear video editor. The following features are selected to enhance the core conversion workflow without turning the application into a full editing suite.

The existing Trim and Merge modules already handle basic editing. This document covers features that extend those capabilities in practical ways.

---

## Features That Fit Within Scope

### 1. Enhanced Trim Module

**Current:** Basic in/out point selection
**Extension:** Add transitions between trimmed segments

When trimming a single source into multiple output segments, allow transitions between them:
- Cross-dissolve between segments
- Fade to black between segments
- This is useful for creating scene compilations

---

### 2. Batch Operations (Already in Queue)

**Current:** Add files one by one
**Extension:** Add folder to queue, add disc titles

This is already covered in `docs/BATCH_QUEUE_DESIGN.md`

---

### 3. Custom Presets

**Current:** Manual configuration each time
**Extension:** Save/load preset configurations

This is already covered in `docs/CUSTOM_PRESETS_DESIGN.md`

---

### 4. Platform Export Presets

One-click settings for common output scenarios:

### Platform Export Presets (Web/Social)

| Preset | Output | Use Case |
|--------|--------|----------|
| YouTube | H.264, 1080p, 8Mbps | Upload to YouTube |
| Vimeo | H.264, 1080p, 12Mbps | Upload to Vimeo |
| Twitter/X | H.264, 720p, 4Mbps | Social media |
| Instagram | H.264, 1080p, 4Mbps, 9:16 | Social media |
| TikTok | H.264, 1080p, 4Mbps, 9:16 | Short-form video |
| Discord | H.264, 720p, 3Mbps | Discord attachment |
| Email | H.264, 480p, 1.5Mbps | Small file email |
| Archive | ProRes 422 | Maximum quality |

### Hardware Platform Presets (Device Compatibility)

Offer codec options for each platform — H.264 (safe/compatible), H.265 (higher quality), and AV1 (best compression, newer devices only):

| Preset | H.264 | H.265/HEVC | AV1 |
|--------|-------|------------|-----|
| iPhone/iPad | High L4.0 | Main Profile | — |
| Android Phone | High L4.1 | Main Profile | Main (2023+) |
| Chromecast Ultra | High L4.0 | Main Profile | Main (2022+) |
| Chromecast Max | High L4.0 | Main Profile | Main (2024+) |
| Fire TV | High L4.0 | Main Profile | — |
| Smart TV (2017+) | High L4.1 | Main Profile | — |
| PlayStation 4 | High L4.1 | — | — |
| PlayStation 5 | High L4.1 | Main Profile | — |
| Xbox One | High L4.1 | — | — |
| Xbox Series X | High L4.1 | Main Profile | — |
| Nintendo Switch | Main L4.1 | — | — |

### Codec Selection UI

When user selects a device preset, show a codec toggle:

```
Device Preset: [Chromecast ▼]

Codec: (•) H.264  ( ) H.265  ( ) AV1
       └─ Safe   └─ Higher   └─ Best
               Quality      Compression

[?] H.265: Better quality, smaller files. Supported on most 2017+ devices.
[?] AV1: Best compression, newest format. Requires 2022+ devices.
```

The user can choose their preferred codec tier. Default to H.264 for maximum compatibility.

---

### 5. Output Naming Templates

Flexible output filename based on source and metadata:

| Token | Description | Example |
|-------|-------------|---------|
| `{source}` | Original filename | video |
| `{ext}` | Output extension | .mp4 |
| `{date}` | Current date | 2026-04-09 |
| `{time}` | Current time | 143022 |
| `{n}` | Sequential number | 001 |
| `{width}` | Output width | 1920 |
| `{height}` | Output height | 1080 |
| `{codec}` | Video codec | h264 |
| `{quality}` | Quality setting | crf20 |

Example template: `{source}_{date}_{width}x{height}_{codec}.{ext}`

Result: `video_2026-04-09_1920x1080_h264.mp4`

---

### 6. Watch Folder (Auto-Process)

Monitor a folder and automatically process new files:

```
Watch Folder Settings:
├── Folder: [~/Downloads/Videos]
├── File types: [.mp4, .mkv, .mov]
├── Preset: [YouTube Upload]
├── Output to: [~/Videos/Converted]
└── On complete: [Move to folder] [Delete original]
```

This enables unattended batch processing.

---

### 7. Simple Watermark/Overlay

Add text or image overlay without full editing:

| Type | Description |
|------|-------------|
| Text watermark | Text at position with opacity |
| Image overlay | Logo in corner |
| Timecode | Burn in timestamp |
| Filename | Show source name |

```
Watermark Settings:
├── Type: [○ Text ○ Image ○ None]
├── Text: [My Watermark]
├── Position: [Bottom-Right ▼]
├── Opacity: [────●────] 50%
├── Size: [────●────] 20%
└── [ ] Show timecode
```

---

### 8. Audio Ducking (Background Music)

Automatically lower music when speech is detected:

```
Audio Ducking:
├── [ ] Enable ducking
├── Reduce by: [────●────] 50%
├── Threshold: -30dB
├── Attack: [100ms]
├── Hold: [500ms]
└── Release: [300ms]
```

This uses FFmpeg's `afftdn` filter to detect speech and lower background audio.

---

### 9. Simple Color Adjustment

Basic color correction (not full color grading):

| Adjustment | Range | FFmpeg |
|------------|-------|--------|
| Brightness | -100 to +100 | `eq=brightness=0.1` |
| Contrast | -100 to +100 | `eq=contrast=1.2` |
| Saturation | -100 to +100 | `eq=saturation=1.5` |
| Gamma | 0.1 to 10 | `eq=gamma=1.2` |

**Skin Tone Preservation:**
When applying color adjustments, preserve natural skin tones. This prevents the "orange/washed out" look common in some AI models where skin tones become too warm or lose pink undertones.

Implementation approach:
- Use color matrix filters that preserve hue
- Avoid over-saturation in flesh-tone ranges
- Option to apply adjustments to "non-skin" areas only

This is especially important when combined with AI upscale — some upscaling models tend to wash out pink tones in light skin, making skin tones appear more orange/tanned. The color adjustment should allow corrections that restore natural skin tones.

```
Color Adjustment:
├── Brightness: [────●────] 0
├── Contrast:   [────●────] 0
├── Saturation: [────●────] +10
├── Gamma:      [────●────] 1.0
├── [ ] Preserve skin tones
└── [ ] Apply to background only
```

This is simpler than color wheels — just a few sliders.

---

## Features That Are Out of Scope

The following are explicitly NOT part of VideoTools:
- Multi-track timeline editing
- Razor/split clips on timeline
- Transitions between arbitrary clips
- Keyframe animation
- Motion graphics
- Green screen/chroma key
- Multi-camera editing
- Video overlays with complex animation

These require a full NLE application.

---

## Summary: What Fits

| Feature | Module | Status |
|--------|--------|--------|
| Video Filters | Convert/Filters | Design done |
| Frame Cropping | Convert | Design done (white outline, draggable) |
| Batch Queue | Queue | Design done |
| Custom Presets | Convert | Design done |
| Platform Export Presets | Convert | New |
| Output Naming Templates | Convert | New |
| Watch Folder | Queue | New |
| Watermark/Overlay | Convert/Filters | New |
| Color Adjustment | Convert/Filters | New (with skin tone preservation) |

---

*Last updated: 2026-04-09*
*Status: Aligned with converter scope*