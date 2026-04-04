# VR360 Video Processing

## Overview

The VR360 module provides support for 360°/VR video processing, including converting 360° content to standard flat video through viewport extraction. This enables users to turn immersive 360° footage into watchable video without VR headsets.

## Use Cases

1. **Viewport Extraction (Primary)** — Convert a 360° video to flat video by extracting a fixed view
2. **Stabilization** — Reduce shakiness in handheld 360° footage
3. **Format Conversion** — Convert between equirectangular, cubemap, and fisheye formats

## Technical Background

### Projection Formats

**Equirectangular (e)**
- Standard 360° format: 2:1 aspect ratio
- Every pixel represents equal angular area
- Used by most 360 cameras (Insta360, GoPro Max, Ricoh Theta)

**Cubemap (c3x2, c6x1, c1x6)**
- Cube projection with 6 faces
- Less distortion at edges
- Used in some VR platforms

**Fisheye**
- Single or dual-fisheye from 360 cameras
- Must be stitched to equirectangular

### FFmpeg Filter: v360

The `v360` filter (available since FFmpeg 4.3) handles all projection conversions:

```bash
# Equirectangular to flat (viewport extraction)
v360=input=e:output=flat:yaw=0:pitch=0:fov=90

# Flat to equirectangular
v360=input=flat:output=e:yaw=0:pitch=0

# Equirectangular to cubemap
v360=input=e:output=c3x2

# Fisheye to equirectangular
v360=input=dfisheye:output=e:ih_fov=180:iv_fov=180
```

### Detection

360° video can be detected by:
- Filename patterns: `_360`, `360_`, `vr`, `equirect`
- Metadata: `major_brand` = `qt` with `vr` atom
- Stream dimensions: 2:1 aspect ratio (commonly 4K, 5.2K, 8K)

## Feature: Viewport Extraction

### Description

Extract a rectangular "window" from a 360° equirectangular video and output as a standard flat video. The viewer sees a fixed perspective instead of the full 360° sphere.

### Parameters

| Parameter | Range | Description |
|-----------|-------|-------------|
| yaw | -180° to +180° | Horizontal rotation (left/right) |
| pitch | -90° to +90° | Vertical rotation (up/down) |
| fov | 30° to 180° | Field of view (zoom level) |

### User Workflow

1. **Load 360 Video**
   - User selects an MP4 or MKV with 360° content
   - System detects format and displays 360 badge in UI

2. **Set Viewport**
   - Interactive preview shows the 360 video with a view cone overlay
   - Yaw slider: drag left/right to pan horizontally
   - Pitch slider: drag up/down to tilt vertically
   - FOV slider: adjust zoom level (narrower = more zoomed in)

3. **Preview**
   - Real-time preview in video player with current viewport settings
   - Play/pause scrubbing to check key moments

4. **Export**
   - Output as standard H.264/H.265 MP4
   - Resolution options: match source, 1080p, 720p
   - Aspect ratio: 16:9 or custom

### FFmpeg Pipeline

```go
// Simple viewport extraction
filterChain := []string{
    "v360=input=e:output=flat:yaw=45:pitch=0:fov=90",
}

// With scaling and encoding
ffmpeg -i input360.mp4 \
    -vf "v360=input=e:output=flat:yaw=${yaw}:pitch=${pitch}:fov=${fov},scale=1920:1080" \
    -c:v libx264 -crf 23 \
    output.mp4
```

### Output Examples

| Source | Settings | Output |
|--------|----------|--------|
| 5.2K equirectangular | yaw=0, pitch=0, fov=90 | 1080p flat (front view) |
| 5.2K equirectangular | yaw=45, pitch=-15, fov=60 | 1080p flat (angled down-right) |
| 4K equirectangular | yaw=0, pitch=0, fov=120 | 1080p flat (wide angle) |

## Feature: Stabilization

### Description

Reduce shakiness in handheld 360° video. This is more complex than standard video stabilization because the entire spherical surface moves.

### Challenges

- Traditional `vidstab` works on flat frames only
- Must stabilize the spherical projection, not the 2D frame
- Gyroscope metadata (from Insta360, Ricoh Theta Z1) can be used instead of visual analysis

### Approaches

**Visual Stabilization (Pass 1-2)**
```bash
# Pass 1: Detect transforms
ffmpeg -i input.mp4 -vf vidstabdetect=shakiness=5:accuracy=15 -f null -

# Pass 2: Apply stabilization
ffmpeg -i input.mp4 -vf vidstabtransform=optalgo=gauss:maxshift=50:maxangle=0.1,v360=input=e:output=flat -c:v libx264 -crf 18 output.mp4
```

**Gyro-Based Stabilization**
Some cameras embed gyro data in the file. External tools can extract and apply this for smoother results than visual analysis.

### Implementation Notes

- Gyro-based is preferred for 360 (more accurate)
- Visual stabilization may leave artifacts at poles
- Consider adding a "stabilization strength" slider (0-100%)

## Feature: Format Conversion

### Equirectangular ↔ Cubemap

```bash
# e → c3x2 (6 faces as 3x2 grid)
ffmpeg -i input.mp4 -vf v360=input=e:output=c3x2 output.mp4

# c3x2 → e
ffmpeg -i input.mp4 -vf v360=input=c3x2:output=e output.mp4
```

### Fisheye De-warp

Dual-fisheye (Rylo, Insta360) to equirectangular:

```bash
ffmpeg -i input.mp4 -vf "v360=input=dfisheye:output=e:ih_fov=190:iv_fov=190:yaw=90" output.mp4
```

## UI Design

### VR360 Module Layout

```
┌─────────────────────────────────────────────────────────────────────┐
│ VR360 — Viewport Extraction                                         │
├─────────────────────────────────────────────────────────────────────┤
│ Source: /path/to/360video.mp4                        [Browse...]   │
├─────────────────────────────────────────────────────────────────────┤
│ ┌───────────────────────────────────────┐ ┌─────────────────────┐ │
│ │                                       │ │ Yaw:    [-180] ──[0]│ │
│ │         360 Preview                   │ │ Pitch:  [  -90] ──[0]│ │
│ │         (with view cone overlay)      │ │ FOV:    [  30] ──[90]│ │
│ │                                       │ │                      │ │
│ │                                       │ │ [Reset View]         │ │
│ │                                       │ └─────────────────────┘ │
│ └───────────────────────────────────────┘                          │
├─────────────────────────────────────────────────────────────────────┤
│ Output: 1080p @ 16:9                               [Export Flat]   │
└─────────────────────────────────────────────────────────────────────┘
```

### Viewport Controls

- **Yaw slider**: -180° to +180° (default: 0°)
- **Pitch slider**: -90° to +90° (default: 0°)
- **FOV slider**: 30° to 180° (default: 90°)

### Preview Mode

The preview should show:
1. The 360 video playing
2. A semi-transparent "cone" overlay showing the current viewport
3. Draggable overlay to quickly set yaw/pitch
4. Mouse wheel to adjust FOV

## Implementation Checklist

### Phase 1: Viewport Extraction (Priority)
- [ ] Detect 360° video format
- [ ] Add v360 filter to FFmpeg build
- [ ] Build UI with yaw/pitch/FOV sliders
- [ ] Implement live preview
- [ ] Export to flat MP4

### Phase 2: Stabilization
- [ ] Add vidstab to FFmpeg build (if not present)
- [ ] Implement gyro metadata detection
- [ ] Build stabilization pipeline
- [ ] Add strength slider

### Phase 3: Format Conversion
- [ ] Equirect ↔ Cubemap conversion
- [ ] Fisheye de-warp for supported cameras
- [ ] Batch processing for multiple files

## FFmpeg Build Requirements

The following filters must be enabled in the FFmpeg build:

- `v360` — 360° format conversion (required)
- `vidstab` — Video stabilization (Phase 2)
- `scale` — Resolution adjustment
- `crop` — Additional cropping if needed

Check with:
```bash
ffmpeg -filters | grep -E "v360|vidstab"
```

## Known Limitations

1. **Performance**: Real-time preview on high-resolution 360 video may require GPU acceleration
2. **Audio**: Spatial audio (ambisonics) will be lost in flat export; may need to downmix or add stereo fallback
3. **Stitching**: Multi-camera stitching is not supported; assumes pre-stitched equirectangular input
4. **Projection**: Only equirectangular input is fully supported; cubemap requires additional handling

## Future Enhancements

- **Animated viewport**: Export with viewport keyframes that move over time (Ken Burns for 360)
- **Multi-viewport**: Export multiple views in a single file (like multi-angle video)
- **VR preview**: Option to preview in VR mode before export
- **Gyro export**: Export gyro data alongside video for external VR players
