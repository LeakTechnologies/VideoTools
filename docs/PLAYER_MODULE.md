# VideoTools Player Module

## Overview

The Player module provides rock-solid video playback with frame-accurate capabilities, serving as the foundation for advanced features like enhancement, trimming, and chapter management.

**Current UI:** Includes a fullscreen toggle in the player controls.

## Architecture Philosophy

**Player stability is critical blocker** for all advanced features. The current implementation follows VideoTools' core principles:
- **Internal Implementation**: No external player dependencies
- **Go-based**: Native integration with existing codebase
- **Cross-platform**: Consistent behavior across Linux, Windows, macOS
- **Frame-accurate**: Precise seeking and frame extraction
- **A/V Sync**: Perfect synchronization without drift
- **Extensible**: Clean interfaces for module integration

## Critical Issues Identified (Legacy Implementation)

### 1. Separate A/V Processes - A/V Desync Inevitable
**Problem**: Video and audio run in completely separate FFmpeg processes with no synchronization.

**Location**: `main.go:10184-10185`
```go
func (p *playSession) startLocked(offset float64) {
    p.runVideo(offset)  // Separate process
    p.runAudio(offset)  // Separate process
}
```

**Symptoms**: 
- Gradual A/V drift over time
- Stuttering when one process slows down
- No way to correct sync when drift occurs

### 2. Command-Line Interface Limitations
**Problem**: MPV/VLC controllers use basic CLI without proper IPC or frame extraction.

**Location**: `internal/player/mpv_controller.go`, `vlc_controller.go`
- No real-time position feedback
- No frame extraction capability
- Process restart required for control changes

### 3. Frame-Accurate Seeking Problems
**Problem**: Seeking restarts entire FFmpeg processes instead of precise seeking.

**Location**: `main.go:10018-10028`
```go
func (p *playSession) Seek(offset float64) {
    p.stopLocked()     // Kill processes
    p.startLocked(p.current)  // Restart from new position
}
```

**Symptoms**:
- 100-500ms gap during seek operations
- No keyframe awareness
- Cannot extract exact frames

### 4. Performance Issues
**Problems**: 
- Frame allocation every frame causes GC pressure
- Small audio buffers cause underruns
- Volume processing in hot path wastes CPU

## Unified Player Architecture (Solution)

### Core Design Principles

1. **Single FFmpeg Process**
   - Multiplexed A/V output to maintain perfect sync
   - Master clock reference for timing
   - PTS-based synchronization with drift correction

2. **Frame-Accurate Operations**
   - Seeking to exact frames without restarts
   - Keyframe extraction for previews
   - Frame buffer pooling to reduce GC pressure

3. **Hardware Acceleration**
   - CUDA/VA-API/VideoToolbox integration
   - Fallback to software decoding
   - Cross-platform hardware detection

4. **Module Integration**
   - Clean interfaces for other modules
   - Frame extraction APIs for enhancement
   - Chapter detection integration from Author module

## Implementation Strategy

### Phase 1: Foundation (Week 1-2)

#### 1.1 Unified FFmpeg Process
```go
type UnifiedPlayer struct {
    cmd           *exec.Cmd
    videoPipe     io.Reader
    audioPipe     io.Reader
    frameBuffer   *RingBuffer
    audioBuffer   *RingBuffer
    syncClock     time.Time
    ptsOffset     int64
    
    // Video properties
    frameRate     float64
    frameCount     int64
    duration      time.Duration
}

// Single FFmpeg with A/V sync
func (p *UnifiedPlayer) load(path string) error {
    cmd := exec.Command("ffmpeg",
        "-i", path,
        // Video stream
        "-map", "0:v:0", "-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:4",
        // Audio stream  
        "-map", "0:a:0", "-f", "s16le", "-ar", "48000", "pipe:5",
        "-")
    
    // Maintain sync internally
}
```

#### 1.2 Hardware Acceleration
```go
type HardwareBackend struct {
    Name     string  // "cuda", "vaapi", "videotoolbox"
    Available bool
    Device   int
    Memory   int64
}

func detectHardwareSupport() []HardwareBackend {
    var backends []HardwareBackend
    
    // NVIDIA CUDA
    if checkNVML() {
        backends = append(backends, HardwareBackend{
            Name: "cuda", Available: true})
    }
    
    // Intel VA-API
    if runtime.GOOS == "linux" && checkVA-API() {
        backends = append(backends, HardwareBackend{
            Name: "vaapi", Available: true})
    }
    
    // Apple VideoToolbox
    if runtime.GOOS == "darwin" && checkVideoToolbox() {
        backends = append(backends, HardwareBackend{
            Name: "videotoolbox", Available: true})
    }
    
    return backends
}
```

#### 1.3 Frame Buffer Management
```go
type FramePool struct {
    pool sync.Pool
    active int
    maxSize int
}

func (p *FramePool) get(w, h int) *image.RGBA {
    if img := p.pool.Get(); img != nil {
        atomic.AddInt32(&p.active, -1)
        return img.(*image.RGBA)
    }
    
    if atomic.LoadInt32(&p.active) >= p.maxSize {
        return image.NewRGBA(image.Rect(0, 0, w, h)) // Fallback
    }
    
    atomic.AddInt32(&p.active, 1)
    return image.NewRGBA(image.Rect(0, 0, w, h))
}
```

### Phase 2: Core Features (Week 3-4)

#### 2.1 Frame-Accurate Seeking
```go
// Frame extraction without restart
func (p *Player) SeekToFrame(frame int64) error {
    seekTime := time.Duration(frame) * time.Second / time.Duration(p.frameRate)
    
    // Extract single frame
    cmd := exec.Command("ffmpeg",
        "-ss", fmt.Sprintf("%.3f", seekTime.Seconds()),
        "-i", p.path,
        "-vframes", "1",
        "-f", "rawvideo",
        "-pix_fmt", "rgb24",
        "-")
    
    // Update display immediately
    frame, err := p.extractFrame(cmd)
    if err != nil {
        return err
    }
    
    return p.displayFrame(frame)
}
```

#### 2.2 Chapter System Integration
```go
// Port scene detection from Author module
func (p *Player) DetectScenes(threshold float64) ([]Chapter, error) {
    cmd := exec.Command("ffmpeg",
        "-i", p.path,
        "-vf", fmt.Sprintf("select='gt(scene=%.2f)',metadata=print:file", threshold),
        "-f", "null",
        "-")
    
    return parseSceneChanges(cmd.Stdout)
}

// Manual chapter support
func (p *Player) AddManualChapter(time time.Duration, title string) error {
    p.chapters = append(p.chapters, Chapter{
        StartTime: time,
        Title:     title,
        Type:      "manual",
    })
    p.updateChapterList()
}

// Chapter navigation
func (p *Player) GoToChapter(index int) error {
    if index < len(p.chapters) {
        return p.SeekToTime(p.chapters[index].StartTime)
    }
    return nil
}
```

#### 2.3 Performance Optimization
```go
type SyncManager struct {
    masterClock    time.Time
    videoPTS        int64
    audioPTS        int64
    driftOffset      int64
    correctionRate  float64
}

func (s *SyncManager) SyncFrame(frameTime time.Duration) error {
    now := time.Now()
    expected := s.masterClock.Add(frameTime)
    
    if now.Before(expected) {
        // We're ahead, wait precisely
        time.Sleep(expected.Sub(now))
    } else if behind := now.Sub(expected); behind > frameDur*2 {
        // We're way behind, skip this frame
        logging.Debug(logging.CatPlayer, "dropping frame, %.0fms behind", behind.Seconds()*1000)
        s.masterClock = now
        return fmt.Errorf("too far behind, skipping frame")
    } else {
        // We're slightly behind, catch up gradually
        s.masterClock = now.Add(frameDur / 2)
    }
    
    s.masterClock = expected
    return nil
}
```

### Phase 3: Advanced Features (Week 5-6)

#### 3.1 Preview System
```go
type PreviewManager struct {
    player    *UnifiedPlayer
    cache     map[int64]*image.RGBA  // Frame cache
    maxSize   int
}

func (p *PreviewManager) GetPreviewFrame(offset time.Duration) (*image.RGBA, error) {
    frameNum := int64(offset.Seconds() * p.player.FrameRate)
    
    if cached, exists := p.cache[frameNum]; exists {
        return cached, nil
    }
    
    // Extract frame if not cached
    frame, err := p.player.ExtractFrame(frameNum)
    if err != nil {
        return nil, err
    }
    
    // Cache for future use
    if len(p.cache) >= p.maxSize {
        p.clearOldestCache()
    }
    p.cache[frameNum] = frame
    
    return frame, nil
}
```

#### 3.2 Error Recovery
```go
type ErrorRecovery struct {
    lastGoodFrame int64
    retryCount    int
    maxRetries    int
}

func (e *ErrorRecovery) HandlePlaybackError(err error) error {
    e.retryCount++
    
    if e.retryCount > e.maxRetries {
        return fmt.Errorf("max retries exceeded: %w", err)
    }
    
    // Implement recovery strategy
    if isDecodeError(err) {
        return e.attemptCodecFallback()
    }
    
    if isBufferError(err) {
        return e.increaseBufferSize()
    }
    
    return e.retryFromLastGoodFrame()
}
```

## Module Integration Points

### Enhancement Module
```go
type EnhancementPlayer interface {
    // Core playback
    GetCurrentFrame() int64
    ExtractFrame(frame int64) (*image.RGBA, error)
    ExtractKeyframes() ([]int64, error)
    
    // Chapter integration
    GetChapters() []Chapter
    AddManualChapter(time time.Duration, title string) error
    
    // Content analysis
    GetVideoInfo() *VideoInfo
    DetectContent() (ContentType, error)
}
```

### Trim Module
```go
type TrimPlayer interface {
    // Timeline interface
    GetTimeline() *TimelineWidget
    SetChapterMarkers([]Chapter) error
    
    // Frame-accurate operations
    TrimToFrames(start, end int64) error
    GetTrimPreview(start, end int64) (*image.RGBA, error)
    
    // Export integration
    ExportTrimmed(path string) error
}
```

### Author Module Integration
```go
// Scene detection integration
func (p *Player) ImportSceneChapters(chapters []Chapter) error {
    p.chapters = append(p.chapters, chapters...)
    return p.updateChapterList()
}
```

## Performance Monitoring

### Key Metrics
```go
type PlayerMetrics struct {
    FrameDeliveryTime    time.Duration  // Target: frameDur * 0.8
    AudioBufferHealth    float64        // Target: > 0.3 (30%)
    SyncDrift          time.Duration  // Target: < 10ms
    CPUMemoryUsage     float64        // Target: < 80%
    FrameDrops         int64          // Target: 0
    SeekTime           time.Duration  // Target: < 50ms
}

func (m *PlayerMetrics) Collect() {
    // Real-time performance tracking
    if frameDelivery := time.Since(frameReadStart); frameDelivery > frameDur*1.5 {
        logging.Warn(logging.CatPlayer, "slow frame delivery: %.1fms", frameDelivery.Seconds()*1000)
    }
    
    if audioBufferFillLevel := audioBuffer.Available() / audioBuffer.Capacity(); 
       audioBufferFillLevel < 0.3 {
        logging.Warn(logging.CatPlayer, "audio buffer low: %.0f%%", audioBufferFillLevel*100)
    }
}
```

## Testing Strategy

### Test Matrix
| Feature | Test Cases | Success Criteria |
|----------|-------------|-----------------|
| Playback | 24/30/60fps smooth | No stuttering, <5% frame drops |
| Seeking | Frame-accurate | <50ms seek time, exact frame |
| A/V Sync | 30+ seconds stable | <10ms drift, no correction needed |
| Chapters | Navigation works | Previous/Next jumps correctly |
| Hardware | Acceleration detected | GPU utilization when available |
| Memory | Stable long-term | No memory leaks, stable usage |
| Cross-platform | Consistent behavior | Linux/Windows/macOS parity |

### Stress Testing
- Long-duration playback (2+ hours)
- Rapid seeking operations (10+ seeks/minute)
- Multiple format support (H.264, H.265, VP9, AV1)
- Hardware acceleration stress testing
- Memory leak detection with runtime/pprof
- CPU usage profiling under different loads

## Implementation Timeline

**Week 1**: Core unified player architecture
**Week 2**: Frame-accurate seeking and chapter integration
**Week 3**: Hardware acceleration and performance optimization
**Week 4**: Preview system and error recovery
**Week 5**: Advanced features (multiple audio tracks, subtitle support)
**Week 6**: Cross-platform testing and optimization

This player implementation provides the rock-solid foundation needed for all advanced VideoTools features while maintaining cross-platform compatibility and Go-based architecture principles.
