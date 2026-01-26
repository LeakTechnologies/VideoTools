# Custom Video Player Implementation

## Overview

VideoTools features a custom-built media player for embedded video playback within the application. This was developed as a complex but necessary component to provide frame-accurate preview and playback capabilities integrated directly into the Fyne UI.

## Why Custom Implementation?

### Initial Approach: External ffplay
The project initially attempted to use `ffplay` (FFmpeg's built-in player) by embedding it in the application window. This approach had several challenges:

- **Window Management**: Embedding external player windows into Fyne's UI proved difficult
- **Control Integration**: Limited programmatic control over ffplay
- **Platform Differences**: X11 window embedding behaves differently across platforms
- **UI Consistency**: External player doesn't match application theming

### Final Solution: Custom FFmpeg-Based Player
A custom player was built using FFmpeg as a frame/audio source with manual rendering:

- **Full Control**: Complete programmatic control over playback
- **Native Integration**: Renders directly into Fyne canvas
- **Consistent UI**: Matches application look and feel
- **Frame Accuracy**: Precise seeking and frame-by-frame control

## Architecture

### Dual-Stream Design

The player uses **two separate FFmpeg processes** running simultaneously:

```
┌─────────────────────────────────────────────────────┐
│                   playSession                       │
├─────────────────────────────────────────────────────┤
│                                                     │
│  ┌──────────────┐           ┌──────────────┐        │
│  │ Video Stream │           │ Audio Stream │        │
│  │  (FFmpeg)    │           │  (FFmpeg)    │        │
│  └──────┬───────┘           └──────┬───────┘        │
│         │                         │                 │
│         │ RGB24 frames            │ s16le PCM       │
│         │ (raw video)             │ (raw audio)     │
│         ▼                         ▼                 │
│  ┌──────────────┐           ┌──────────────┐        │
│  │ Frame Pump   │           │ Audio Player │        │
│  │ (goroutine)  │           │ (SDL2/oto)   │        │
│  └──────┬───────┘           └──────────────┘        │
│         │                                           │
│         │ Update Fyne canvas.Image                  │
│         ▼                                           │
│  ┌──────────────┐                                   │
│  │ UI Display   │                                   │
│  └──────────────┘                                   │
└─────────────────────────────────────────────────────┘
```

### Component Breakdown

#### 1. Video Stream (`runVideo`)

**FFmpeg Command:**
```bash
ffmpeg -hide_banner -loglevel error \
  -ss <offset> \
  -i <video_file> \
  -vf scale=<targetW>:<targetH> \
  -f rawvideo \
  -pix_fmt rgb24 \
  -r <fps> \
  -
```

**Purpose:** Extract video frames as raw RGB data

**Process:**
1. Starts FFmpeg to decode video
2. Scales frames to target display resolution
3. Outputs RGB24 pixel data to stdout
4. Frames read by goroutine and displayed

**Frame Pacing:**
- Calculates frame duration from source FPS: `frameDuration = 1 / fps`
- Sleeps between frames to maintain proper playback speed
- Honors pause state by skipping frame updates

**Frame Pump Loop:**
```go
frameSize := targetW * targetH * 3  // RGB = 3 bytes per pixel
buf := make([]byte, frameSize)

for {
    // Read exactly one frame worth of data
    io.ReadFull(stdout, buf)

    // Respect pause state
    if paused {
        continue (wait for unpause)
    }

    // Pace to source FPS
    waitUntil(nextFrameTime)

    // Update canvas image
    updateImage(buf)

    // Schedule next frame
    nextFrameTime += frameDuration
}
```

#### 2. Audio Stream (`runAudio`)

**FFmpeg Command:**
```bash
ffmpeg -hide_banner -loglevel error \
  -ss <offset> \
  -i <video_file> \
  -vn \              # No video
  -ac 2 \            # Stereo
  -ar 48000 \        # 48kHz sample rate
  -f s16le \         # 16-bit signed little-endian
  -
```

**Purpose:** Extract audio as raw PCM data

**Audio Playback:**
- Uses SDL2/oto library for cross-platform audio output
- Fixed format: 48kHz, stereo (2 channels), 16-bit PCM
- Direct pipe from FFmpeg to audio device

**Volume Control:**
- Software gain adjustment before playback
- Real-time volume multiplication on PCM samples
- Mute by zeroing audio buffer
- Volume range: 0-100 (can amplify up to 200% in code)

**Volume Processing:**
```go
gain := volume / 100.0

for each 16-bit sample {
    sample := readInt16(audioData)
    amplified := int16(float64(sample) * gain)
    // Clamp to prevent distortion
    amplified = clamp(amplified, -32768, 32767)
    writeInt16(audioData, amplified)
}

audioPlayer.Write(audioData)
```

#### 3. Synchronization

**Shared State:**
- Both streams start from same offset timestamp
- `paused` flag affects both video and audio loops
- `current` position tracks playback time
- No explicit A/V sync mechanism (relies on OS scheduling)

**Synchronization Strategy:**
- Video paced by sleep timing between frames
- Audio paced by audio device buffer consumption
- Both start from same `-ss` offset
- Generally stays synchronized for short clips
- May drift on longer playback (known limitation)

### State Management

#### playSession Structure

```go
type playSession struct {
    mu sync.Mutex

    // File info
    path    string
    fps     float64
    width   int      // Original dimensions
    height  int
    targetW int      // Display dimensions
    targetH int

    // Playback state
    paused  bool
    current float64  // Current position (seconds)
    frameN  int      // Frame counter

    // Volume
    volume float64   // 0-100
    muted  bool

    // FFmpeg processes
    videoCmd *exec.Cmd
    audioCmd *exec.Cmd

    // Control channels
    stop    chan struct{}
    done    chan struct{}

    // UI callbacks
    prog    func(float64)  // Progress update callback
    img     *canvas.Image  // Fyne image to render to
}
```

## Implemented Features

### ✅ Play/Pause
- **Play**: Starts or resumes both video and audio streams
- **Pause**: Halts frame updates and audio output
- Preserves current position when paused
- No resource cleanup during pause (streams keep running)

### ✅ Seek
- Jump to any timestamp in the video
- **Implementation**: Stop both streams, restart at new position
- Preserves pause state across seeks
- Updates progress indicator immediately

**Known Issue:** Seeking restarts FFmpeg processes, causing brief interruption

### ✅ Volume Control
- Range: 0-100 (UI) / 0-200 (code max)
- Real-time volume adjustment without restarting audio
- Software mixing/gain control
- Automatic mute at volume 0
- No crackling/popping during adjustment

### ✅ Embedded Playback
- Renders directly into Fyne `canvas.Image`
- No external windows
- Respects Fyne layout system
- Scales to target dimensions

### ✅ Progress Tracking
- Reports current playback position
- Callback to update UI slider/display
- Accurate to ~frame duration

### ✅ Resource Management
- Properly kills FFmpeg processes on stop
- Cleans up goroutines
- No zombie processes
- Handles early termination gracefully

## Current Limitations

### ❌ No Fullscreen Support
- Controller interface includes `FullScreen()` method
- Currently returns "player unavailable" error
- Would require:
  - Dedicated fullscreen window
  - Escaping fullscreen (ESC key handling)
  - Preserving playback state during transition
  - Overlay controls in fullscreen mode

**Future Implementation:**
```go
func (s *appState) enterFullscreen() {
    // Create new fullscreen window
    fsWindow := fyne.CurrentApp().NewWindow("Playback")
    fsWindow.SetFullScreen(true)

    // Transfer playback to fullscreen canvas
    // Preserve playback position
    // Add overlay controls
}
```

### Limited Audio Format
- Fixed at 48kHz, stereo, 16-bit
- Doesn't adapt to source format
- Mono sources upconverted to stereo
- Other sample rates resampled

**Why:** Simplifies audio playback code, 48kHz/stereo is standard

### A/V Sync Drift
- No PTS (Presentation Timestamp) tracking
- Relies on OS thread scheduling
- May drift on long playback (>5 minutes)
- Seek resynchronizes

**Mitigation:** Primarily used for short previews, not long playback

### Seeking Performance
- Restarts FFmpeg processes
- Brief audio/video gap during seek
- Not instantaneous like native players
- ~100-500ms interruption

**Why:** Simpler than maintaining seekable streams

### No Speed Control
- Playback speed fixed at 1.0×
- No fast-forward/rewind
- No slow-motion

**Future:** Could adjust frame pacing and audio playback rate

### No Subtitle Support
- Video-only rendering
- Subtitles not displayed during playback
- Would require subtitle stream parsing and rendering

## Implementation Challenges Overcome

### 1. Frame Pacing
**Challenge:** How fast to pump frames to avoid flicker or lag?

**Solution:** Calculate exact frame duration from FPS:
```go
frameDuration := time.Duration(float64(time.Second) / fps)
nextFrameAt := time.Now()

for {
    // Process frame...

    // Wait until next frame time
    nextFrameAt = nextFrameAt.Add(frameDuration)
    sleepUntil(nextFrameAt)
}
```

### 2. Image Updates in Fyne
**Challenge:** Fyne's `canvas.Image` needs proper refresh

**Solution:**
```go
img.Resource = canvas.NewImageFromImage(frameImage)
img.Refresh()  // Trigger redraw
```

### 3. Pause State Handling
**Challenge:** Pause without destroying streams (avoid restart delay)

**Solution:** Keep streams running but:
- Skip frame updates in video loop
- Skip audio writes in audio loop
- Resume instantly by unsetting pause flag

### 4. Volume Adjustment
**Challenge:** Adjust volume without restarting audio stream

**Solution:** Apply gain to PCM samples in real-time:
```go
if !muted {
    sample *= (volume / 100.0)
    clamp(sample)
}
write(audioBuffer, sample)
```

### 5. Clean Shutdown
**Challenge:** Stop playback without leaving orphaned FFmpeg processes

**Solution:**
```go
func stopLocked() {
    close(stopChannel)  // Signal goroutines to exit

    if videoCmd != nil {
        videoCmd.Process.Kill()
        videoCmd.Wait()  // Clean up zombie
    }

    if audioCmd != nil {
        audioCmd.Process.Kill()
        audioCmd.Wait()
    }
}
```

### 6. Seeking While Paused
**Challenge:** Seek should work whether playing or paused

**Solution:**
```go
func Seek(offset float64) {
    wasPaused := paused

    stopStreams()
    startStreams(offset)

    if wasPaused {
        // Ensure pause state restored after restart
        time.AfterFunc(30*time.Millisecond, func() {
            paused = true
        })
    }
}
```

## Technical Details

### Video Frame Processing

**Frame Size Calculation:**
```
frameSize = width × height × 3 bytes (RGB24)
Example: 640×360 = 691,200 bytes per frame
```

**Reading Frames:**
```go
buf := make([]byte, targetW * targetH * 3)

for {
    // Read exactly one frame
    n, err := io.ReadFull(stdout, buf)

    if n == frameSize {
        // Convert to image.RGBA
        img := image.NewRGBA(image.Rect(0, 0, targetW, targetH))

        // Copy RGB24 → RGBA
        for i := 0; i < targetW * targetH; i++ {
            img.Pix[i*4+0] = buf[i*3+0]  // R
            img.Pix[i*4+1] = buf[i*3+1]  // G
            img.Pix[i*4+2] = buf[i*3+2]  // B
            img.Pix[i*4+3] = 255         // A (opaque)
        }

        updateCanvas(img)
    }
}
```

### Audio Processing

**Audio Format:**
- **Sample Rate**: 48,000 Hz
- **Channels**: 2 (stereo)
- **Bit Depth**: 16-bit signed integer
- **Byte Order**: Little-endian
- **Format**: s16le (signed 16-bit little-endian)

**Buffer Size:**
- 4096 bytes (2048 samples, 1024 per channel)
- ~21ms of audio at 48kHz stereo

**Volume Control Math:**
```go
// Read 16-bit sample (2 bytes)
sample := int16(binary.LittleEndian.Uint16(audioData[i:i+2]))

// Apply gain
amplified := int(float64(sample) * gain)

// Clamp to prevent overflow/distortion
if amplified > 32767 {
    amplified = 32767
} else if amplified < -32768 {
    amplified = -32768
}

// Write back
binary.LittleEndian.PutUint16(audioData[i:i+2], uint16(int16(amplified)))
```

### Performance Characteristics

**CPU Usage:**
- **Video Decoding**: ~5-15% per core (depends on codec)
- **Audio Decoding**: ~1-2% per core
- **Frame Rendering**: ~2-5% (image conversion + Fyne refresh)
- **Total**: ~10-25% CPU for 720p H.264 playback

**Memory Usage:**
- **Frame Buffers**: ~2-3 MB (multiple frames buffered)
- **Audio Buffers**: ~100 KB
- **FFmpeg Processes**: ~50-100 MB each
- **Total**: ~150-250 MB during playback

**Startup Time:**
- FFmpeg process spawn: ~50-100ms
- First frame decode: ~100-300ms
- Total time to first frame: ~150-400ms

## Integration with VideoTools

### Usage in Convert Module

The player is embedded in the metadata panel:

```go
// Create player surface
playerImg := canvas.NewImageFromImage(image.NewRGBA(...))
playerSurface := container.NewStack(playerImg)

// Create play session
session := newPlaySession(
    videoPath,
    sourceWidth, sourceHeight,
    fps,
    displayWidth, displayHeight,
    progressCallback,
    playerImg,
)

// Playback controls
playBtn := widget.NewButton("Play", func() {
    session.Play()
})

pauseBtn := widget.NewButton("Pause", func() {
    session.Pause()
})

seekSlider := widget.NewSlider(0, duration)
seekSlider.OnChanged = func(val float64) {
    session.Seek(val)
}
```

### Player Window Sizing

Aspect ratio preserved based on source video:

```go
targetW := 508  // Fixed width for UI layout
targetH := int(float64(targetW) * (float64(sourceH) / float64(sourceW)))

// E.g., 1920×1080 → 508×286
// E.g., 1280×720  → 508×286
// E.g., 720×480   → 508×339
```

## Alternative Player (ffplay-based)

The `internal/player` package contains a platform-specific `ffplay` wrapper:

### Controller Interface

```go
type Controller interface {
    Load(path string, offset float64) error
    SetWindow(x, y, w, h int)
    Play() error
    Pause() error
    Seek(offset float64) error
    SetVolume(level float64) error
    FullScreen() error
    Stop() error
    Close()
}
```

### Implementations

- **Stub** (`controller_stub.go`): Returns errors for all operations
- **Linux** (`controller_linux.go`): Uses X11 window embedding (partially implemented)
- **Windows**: Not implemented

**Status:** This approach was largely abandoned in favor of the custom `playSession` implementation due to window embedding complexity.

## Future Improvements

### High Priority
1. **Fullscreen Mode**
   - Dedicated fullscreen window
   - Overlay controls with auto-hide
   - ESC key to exit
   - Maintain playback position

2. **Better A/V Sync**
   - PTS (Presentation Timestamp) tracking
   - Adjust frame pacing based on audio clock
   - Detect and correct drift

3. **Smoother Seeking**
   - Keep streams alive during seek (use -ss on open pipe)
   - Reduce interruption time
   - Consider keyframe-aware seeking

### Medium Priority
4. **Speed Control**
   - Playback speed adjustment (0.5×, 1.5×, 2×)
   - Maintain pitch for audio (atempo filter)

5. **Subtitle Support**
   - Parse subtitle streams
   - Render text overlays
   - Subtitle track selection

6. **Format Adaptation**
   - Auto-detect audio channels/sample rate
   - Adapt audio pipeline to source format
   - Reduce resampling overhead

### Low Priority
7. **Performance Optimization**
   - GPU-accelerated decoding (hwaccel)
   - Frame buffer pooling
   - Reduce memory allocations

8. **Enhanced Controls**
   - Frame-by-frame stepping (← → keys)
   - Skip forward/backward (10s, 30s jumps)
   - A-B repeat loop
   - Playback markers

## See Also

- [Convert Module](convert/) - Uses player for video preview
- FFmpeg Integration *(planned)*
- Architecture *(planned)*

## Developer Notes

### Testing the Player

```go
// Minimal test setup
session := newPlaySession(
    "test.mp4",
    1920, 1080,  // Source dimensions
    29.97,       // FPS
    640, 360,    // Target dimensions
    func(pos float64) {
        fmt.Printf("Position: %.2fs\n", pos)
    },
    canvasImage,
)

session.Play()
time.Sleep(5 * time.Second)
session.Pause()
session.Seek(30.0)
session.Play()
```

### Debugging

Enable FFmpeg logging:
```go
debugLog(logCatFFMPEG, "message")
```

Set environment variable:
```bash
VIDEOTOOLS_DEBUG=1 ./VideoTools
```

### Common Issues

**Black screen:** FFmpeg failed to start or decode
- Check stderr output
- Verify file path is valid
- Test FFmpeg command manually

**No audio:** SDL2/oto initialization failed
- Check audio device availability
- Verify SDL2 libraries installed
- Test with different sample rate

**Choppy playback:** FPS mismatch or CPU overload
- Check calculated frameDuration
- Verify FPS detection
- Monitor CPU usage

---

*Last Updated: 2025-11-23*
