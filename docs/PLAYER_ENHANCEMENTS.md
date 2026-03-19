# Video Player Enhancement Roadmap

This document outlines the planned enhancements to the VideoTools media engine to create a YouTube-quality playback experience.

## Overview

The goal is to leverage FFmpeg's full capabilities for professional-grade video playback with GPU acceleration, smooth seeking, and responsive controls.

---

## Priority 1: GPU Rendering Pipeline

### 1.1 Multi-API GPU Rendering

Support both OpenGL and Direct3D 11 for cross-vendor GPU acceleration:

| API | Vendor | Platform | Status |
|-----|--------|----------|--------|
| OpenGL 4.6+ | NVIDIA, AMD, Intel | Linux, macOS | Planned |
| Direct3D 11 | NVIDIA, AMD, Intel | Windows | Planned |
| Vulkan | All major | Cross-platform | Future |

**Implementation Strategy:**
- Abstract renderer interface (`Renderer` interface in `internal/media/gpu/`)
- Platform-specific implementations: `opengl.go`, `d3d11.go`
- Automatic detection of best available API
- Fallback to software rendering if GPU unavailable

### 1.2 Frame Upload Pipeline

```
FFmpeg Decode (HW) → GPU Frame → Direct Texture Upload → Display
                           ↓
                    Minimize CPU copies
```

**Current State:** CPU pixel copying via `canvas.Raster`
**Target State:** Zero-copy GPU texture upload

### 1.3 Hardware Frame Reading

Use FFmpeg's `AVBufferRef` for hardware-backed frames:
- `av_hwframe_transfer_data()` for GPU→CPU when needed
- Direct GPU texture access when possible
- Configurable for different hardware contexts (VAAPI, D3D11VA, QSV)

---

## Priority 2: Keyboard Shortcuts

### 2.1 Core Shortcuts

| Key | Action |
|-----|--------|
| `Space` | Play/Pause |
| `←` / `→` | Seek ±5 seconds |
| `Shift+←` / `Shift+→` | Seek ±1 frame |
| `F` | Toggle fullscreen |
| `M` | Toggle mute |
| `0-9` | Jump to 0%-90% |
| `Home` / `End` | Jump to start/end |
| `↑` / `↓` | Volume ±10% |

### 2.2 Advanced Shortcuts

| Key | Action |
|-----|--------|
| `<` / `>` | Playback speed: 0.25x → 2x |
| `P` | Toggle picture-in-picture |
| `C` | Toggle subtitles |
| `I` | Show file info |

---

## Priority 3: Smooth Scrubbing

### 3.1 Thumbnail Preview System

When hovering over the seek bar, show mini-thumbnails:
```
┌─────────────────────────────────────────┐
│  [thumb1] [thumb2] [thumb3] [thumb4]...  │
│           ↑ cursor                      │
│     ┌─────────┐                         │
│     │ preview │  <- 16:9 thumbnail      │
│     └─────────┘                         │
└─────────────────────────────────────────┘
```

**Implementation:**
- Extract keyframe thumbnails during file open (async)
- Store in circular buffer (~100 thumbnails max)
- Show nearest keyframe thumbnail on slider hover
- Pre-decode frames during seek for instant preview

### 3.2 Seek Strategy

```
1. User requests seek to timestamp T
2. Find nearest keyframe before T (fast)
3. Seek to keyframe position
4. Decode forward until T (few frames)
5. Display frame immediately
6. Continue decoding ahead for smooth playback
```

**FFmpeg APIs:**
- `av_seek_frame()` with `AVSEEK_FLAG_BACKWARD`
- `avformat_seek_file()` for precise seeking
- Async decode queue for background pre-loading

---

## Priority 4: Buffering & Pre-loading

### 4.1 Frame Queue Architecture

```
┌──────────────────────────────────────────────────────┐
│                    Decoded Frames                     │
│   ┌────┐ ┌────┐ ┌────┐ ┌────┐ ┌────┐ ┌────┐ ┌────┐  │
│   │ F1 │ │ F2 │ │ F3 │ │ F4 │ │ F5 │ │ F6 │ │ F7 │  │
│   └────┘ └────┘ └────┘ └────┘ └────┘ └────┘ └────┘  │
└──────────────────────────────────────────────────────┘
         ↑                                     ↑
      Display                              Decode
        Gap                              Buffering
```

**Configuration:**
| Parameter | Default | Min | Max |
|-----------|---------|-----|-----|
| Video buffer | 15 frames | 5 | 60 |
| Audio buffer | 2 seconds | 0.5s | 10s |
| Pre-decode | 30 frames | 10 | 120 |

### 4.2 Adaptive Frame Dropping

When behind schedule:
1. Drop B-frames first (least visible impact)
2. If still behind, drop P-frames
3. Only drop I-frames as last resort
4. Visual indicator when dropping frames

---

## Priority 5: Playback Speed Control

### 5.1 Speed Presets

| Speed | Use Case |
|-------|----------|
| 0.25x | Slow motion analysis |
| 0.5x | Detailed review |
| 0.75x | Slightly slow |
| 1.0x | Normal playback |
| 1.25x | Faster review |
| 1.5x | Quick scan |
| 2.0x | Fastest practical |

### 5.2 Audio Pitch Correction

For non-1x speeds, use FFmpeg audio filters:
```
atempo=0.5-2.0  (rubberband library for better quality)
```

---

## Priority 6: Video Filters (AVFilter)

Enable common processing via FFmpeg filters:

### 6.1 Deinterlacing
```
yadif=mode=1:parity=-1:field=auto
```

### 6.2 Scaling
```
scale=1920:1080:flags=lanczos
scale=-1:1080:flags=fast_bilinear
```

### 6.3 Color Correction
```
eq=brightness=0.06:saturation=1.1:contrast=1.0
```

### 6.4 Custom Filter Pipeline
User-configurable filter strings:
```
"unsharp=5:5:1.0:5:5:0.0"  // Sharpen
"curves=vintage"            // Vintage look
```

---

## Priority 7: Loading & Error States

### 7.1 Loading Indicators

| State | Indicator |
|-------|-----------|
| Opening file | Circular spinner |
| Seeking | Progress bar in seek bar |
| Buffering | Spinner + "Buffering..." |
| Error | Red indicator + message |

### 7.2 Error Recovery

**Transient Errors:**
- Network timeout → Retry 3x with exponential backoff
- Decode error → Skip corrupted frame, continue
- Hardware decode fail → Fallback to software

**Permanent Errors:**
- Unsupported codec → Show clear message + suggest codec
- Corrupt file → Offer to scan with ffprobe
- Missing hardware → Graceful software fallback

---

## Priority 8: Picture-in-Picture

### 8.1 Implementation

| Platform | API |
|----------|-----|
| Windows | `setWindowDisplayAffinity(WDA_EXCLUDEFROMCAPTURE)` |
| Linux | X11 `override_redirect` + composite |
| macOS | `NSWindow.styleMask` |

### 8.2 PiP Window Controls
- Drag to reposition
- Click to restore full player
- Right-click menu: Close PiP, Open in Player

---

## Priority 9: Resume Playback

Store playback state:
```json
{
  "path": "/videos/example.mp4",
  "position": 1234.5,
  "lastWatched": "2026-03-19T10:30:00Z",
  "duration": 3600.0,
  "completed": false
}
```

**UI Flow:**
1. On file open, check for saved position
2. If >5% remaining, show "Continue from X:XX?"
3. Options: Resume, Start Over, Cancel

---

## Priority 10: Subtitle Rendering

### 10.1 Styling Options
| Property | Default | Options |
|----------|---------|---------|
| Font | System default | Custom |
| Size | 85% of video | 50%-200% |
| Color | White | Any + outline |
| Background | Semi-transparent | None, 25%, 50%, 75% |
| Position | Bottom | Top, Custom |

### 10.2 Format Support
- SRT (SubRip)
- ASS/SSA (Advanced SubStation Alpha)
- VTT (WebVTT)
- Embedded PGS (Blu-ray)

---

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                        VideoPlayer UI                           │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │  Controls: Play/Pause, Seek, Volume, Speed, Fullscreen    │ │
│  └─────────────────────────────────────────────────────────────┘ │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │                    GPU Canvas                               │ │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐                   │ │
│  │  │ OpenGL  │  │ D3D11   │  │ Vulkan  │  ← Renderer Select │ │
│  │  └─────────┘  └─────────┘  └─────────┘                   │ │
│  └─────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Media Engine                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐        │
│  │ Audio Queue  │  │ Video Queue  │  │ Subtitle Q   │        │
│  └──────────────┘  └──────────────┘  └──────────────┘        │
│                              │                                  │
│  ┌──────────────────────────────────────────────────────────────┐│
│  │              Master Clock (Audio as reference)              ││
│  └──────────────────────────────────────────────────────────────┘│
│                              │                                  │
│  ┌──────────────────────────────────────────────────────────────┐│
│  │              FFmpeg Demuxer/Decoder                         ││
│  │  - av_seek_frame() for accurate seeking                     ││
│  │  - av_hwdevice_ctx_create() for HW acceleration            ││
│  │  - avcodec_send_packet() / avcodec_receive_frame()          ││
│  └──────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

---

## File Structure

```
internal/media/
├── engine.go           # Core playback engine
├── audio.go            # AudioPlayer with volume/mute
├── clock.go            # Master clock synchronization
├── queue.go            # Packet/frame queues
├── subtitle.go         # Subtitle extraction/rendering
├── view.go             # Basic VideoPlayer widget
├── media_test.go       # Tests
│
├── gpu/                # GPU rendering (NEW)
│   ├── renderer.go     # Abstract renderer interface
│   ├── texture.go      # Texture upload abstraction
│   ├── opengl.go       # OpenGL implementation
│   ├── d3d11.go        # Direct3D 11 implementation
│   ├── helpers.go      # Shared GPU utilities
│   └── shaders/        # GLSL/HLSL shaders
│       ├── vertex.glsl
│       ├── fragment.glsl
│       └── yuv2rgb.glsl
│
├── controls/           # Player controls (NEW)
│   ├── shortcuts.go    # Keyboard handler
│   ├── seekbar.go      # Seek bar with thumbnails
│   ├── volume.go       # Volume slider
│   └── speed.go        # Playback speed selector
│
├── filters/            # FFmpeg filter pipeline (NEW)
│   ├── pipeline.go     # Filter graph builder
│   ├── deinterlace.go  # Deinterlacing
│   ├── scale.go        # Scaling
│   └── color.go        # Color correction
│
└── state/              # Playback state (NEW)
    ├── resume.go       # Resume playback logic
    ├── history.go      # Watch history
    └── preferences.go  # User preferences
```

---

## Testing Strategy

### Unit Tests
- Queue operations
- Clock synchronization
- Seek calculations
- Filter parsing

### Integration Tests
- File open/close cycles
- Seek accuracy
- Buffer underrun handling
- HW decode fallback

### Platform Tests
- NVIDIA GPU (Windows/Linux)
- AMD GPU (Windows/Linux/macOS)
- Intel integrated (Windows/Linux)
- Software fallback

### Stress Tests
- 4K60fps playback
- High bitrate HEVC
- Network streams
- Large file seeking

---

## Performance Targets

| Metric | Target | Minimum |
|--------|--------|---------|
| Frame latency | <16ms | <33ms |
| Seek latency | <100ms | <500ms |
| Memory (1080p) | <200MB | <500MB |
| CPU idle | <5% | <15% |
| GPU usage | <30% | <60% |

---

## Implementation Phases

### Phase 1: GPU Rendering (Current)
- [x] Renderer interface
- [ ] OpenGL implementation
- [ ] D3D11 implementation
- [ ] Texture upload pipeline
- [ ] Shader-based YUV→RGB

### Phase 2: Controls & Shortcuts
- [ ] Keyboard handler
- [ ] Seek bar with preview
- [ ] Volume/speed controls
- [ ] Fullscreen toggle

### Phase 3: Smooth Seeking
- [ ] Thumbnail extraction
- [ ] Preview on hover
- [ ] Async decode queue
- [ ] Frame pre-loading

### Phase 4: Buffering & Polish
- [ ] Adaptive buffering
- [ ] Loading indicators
- [ ] Error recovery
- [ ] Performance tuning

---

## Dependencies

### FFmpeg Libraries (via CGO)
- `libavcodec` - Decoding
- `libavformat` - Demuxing
- `libavutil` - Utilities
- `libswscale` - Software scaling (fallback)
- `libswresample` - Audio resampling
- `libavfilter` - Video filters

### System Libraries
- OpenGL 4.6+ (Linux/Windows)
- Direct3D 11 (Windows)
- Vulkan SDK (Future)

### Go Packages
- `fyne.io/fyne/v2` - UI framework
- `github.com/go-gl/gl/v4.6-core/gl` - OpenGL bindings
- `github.com/Microsoft/go-d3d11` - D3D11 bindings

---

*Last Updated: 2026-03-19*
*Version: Planning v1.0*
