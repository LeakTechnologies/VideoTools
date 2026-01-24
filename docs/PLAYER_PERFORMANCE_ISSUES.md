# Player Module Performance Issues & Fixes

## Current Problems Causing Stuttering

### 1. **Separate Video & Audio Processes (No Sync)**
**Location:** `main.go:9144` (runVideo) and `main.go:9233` (runAudio)

**Problem:**
- Video and audio run in completely separate FFmpeg processes
- No synchronization mechanism between them
- They will inevitably drift apart, causing A/V desync and stuttering

**Current Implementation:**
```go
func (p *playSession) startLocked(offset float64) {
    p.runVideo(offset)  // Separate process
    p.runAudio(offset)  // Separate process
}
```

**Why It Stutters:**
- If video frame processing takes too long → audio continues → desync
- If audio buffer underruns → video continues → desync
- No feedback loop to keep them in sync

---

### 2. **Audio Buffer Too Small**
**Location:** `main.go:8960` (audio context) and `main.go:9274` (chunk size)

**Problem:**
```go
// Audio context with tiny buffer (42ms at 48kHz)
audioCtxGlobal.ctx, audioCtxGlobal.err = oto.NewContext(sampleRate, channels, bytesPerSample, 2048)

// Tiny read chunks (21ms of audio)
chunk := make([]byte, 4096)
```

**Why It Stutters:**
- 21ms chunks mean we need to read 47 times per second
- Any delay > 21ms causes audio dropout/stuttering
- 2048 sample buffer gives only 42ms protection against underruns
- Modern systems need 100-200ms buffers for smooth playback

---

### 3. **Volume Processing in Hot Path**
**Location:** `main.go:9294-9318`

**Problem:**
```go
// Processes volume on EVERY audio chunk read
for i := 0; i+1 < n; i += 2 {
    sample := int16(binary.LittleEndian.Uint16(tmp[i:]))
    amp := int(float64(sample) * gain)
    // ... clamping ...
    binary.LittleEndian.PutUint16(tmp[i:], uint16(int16(amp)))
}
```

**Why It Stutters:**
- CPU-intensive per-sample processing
- Happens 47 times/second with tiny chunks
- Blocks the audio read loop
- Should use FFmpeg's volume filter or hardware mixing

---

### 4. **Video Frame Pacing Issues**
**Location:** `main.go:9200-9203`

**Problem:**
```go
if delay := time.Until(nextFrameAt); delay > 0 {
    time.Sleep(delay)
}
nextFrameAt = nextFrameAt.Add(frameDur)
```

**Why It Stutters:**
- `time.Sleep()` is not precise (can wake up late)
- Cumulative drift: if one frame is late, all future frames shift
- No correction mechanism if we fall behind
- UI thread delays from `DoFromGoroutine` can cause frame drops

---

### 5. **UI Thread Blocking**
**Location:** `main.go:9207-9215`

**Problem:**
```go
// Every frame waits for UI thread to be available
fyne.CurrentApp().Driver().DoFromGoroutine(func() {
    p.img.Image = frame
    p.img.Refresh()
}, false)
```

**Why It Stutters:**
- If UI thread is busy, frame updates queue up
- Can cause video to appear choppy even if FFmpeg is delivering smoothly
- No frame dropping mechanism if UI can't keep up

---

### 6. **Frame Allocation on Every Frame**
**Location:** `main.go:9205-9206`

**Problem:**
```go
// Allocates new frame buffer 24-60 times per second
frame := image.NewRGBA(image.Rect(0, 0, p.targetW, p.targetH))
utils.CopyRGBToRGBA(frame.Pix, buf)
```

**Why It Stutters:**
- Memory allocation on every frame causes GC pressure
- Extra copy operation adds latency
- Could reuse buffers or use ring buffer

---

## Recommended Fixes (Priority Order)

### Priority 1: Increase Audio Buffers (Quick Fix)

**Change `main.go:8960`:**
```go
// OLD: 2048 samples = 42ms
audioCtxGlobal.ctx, audioCtxGlobal.err = oto.NewContext(sampleRate, channels, bytesPerSample, 2048)

// NEW: 8192 samples = 170ms (more buffer = smoother playback)
audioCtxGlobal.ctx, audioCtxGlobal.err = oto.NewContext(sampleRate, channels, bytesPerSample, 8192)
```

**Change `main.go:9274`:**
```go
// OLD: 4096 bytes = 21ms
chunk := make([]byte, 4096)

// NEW: 16384 bytes = 85ms per chunk
chunk := make([]byte, 16384)
```

**Expected Result:** Audio stuttering should improve significantly

---

### Priority 2: Use FFmpeg for Volume Control

**Change `main.go:9238-9247`:**
```go
// Add volume filter to FFmpeg command instead of processing in Go
volumeFilter := ""
if p.muted || p.volume <= 0 {
    volumeFilter = "-af volume=0"
} else if math.Abs(p.volume - 100) > 0.1 {
    volumeFilter = fmt.Sprintf("-af volume=%.2f", p.volume/100.0)
}

cmd := exec.Command(platformConfig.FFmpegPath,
    "-hide_banner", "-loglevel", "error",
    "-ss", fmt.Sprintf("%.3f", offset),
    "-i", p.path,
    "-vn",
    "-ac", fmt.Sprintf("%d", channels),
    "-ar", fmt.Sprintf("%d", sampleRate),
    volumeFilter,  // Let FFmpeg handle volume
    "-f", "s16le",
    "-",
)
```

**Remove volume processing loop (lines 9294-9318):**
```go
// Simply write chunks directly
localPlayer.Write(chunk[:n])
```

**Expected Result:** Reduced CPU usage, smoother audio

---

### Priority 3: Use Single FFmpeg Process with A/V Sync

**Conceptual Change:**
Instead of separate video/audio processes, use ONE FFmpeg process that:
1. Outputs video frames to one pipe
2. Outputs audio to another pipe (or use `-f matroska` with demuxing)
3. Maintains sync internally

**Pseudocode:**
```go
cmd := exec.Command(platformConfig.FFmpegPath,
    "-ss", fmt.Sprintf("%.3f", offset),
    "-i", p.path,
    // Video stream
    "-map", "0:v:0",
    "-f", "rawvideo",
    "-pix_fmt", "rgb24",
    "-r", fmt.Sprintf("%.3f", p.fps),
    "pipe:4",  // Video to fd 4
    // Audio stream
    "-map", "0:a:0",
    "-ac", "2",
    "-ar", "48000",
    "-f", "s16le",
    "pipe:5",  // Audio to fd 5
)
```

**Expected Result:** Perfect A/V sync, no drift

---

### Priority 4: Frame Buffer Reuse

**Change `main.go:9205-9206`:**
```go
// Reuse frame buffers instead of allocating every frame
type framePool struct {
    pool sync.Pool
}

func (p *framePool) get(w, h int) *image.RGBA {
    if img := p.pool.Get(); img != nil {
        return img.(*image.RGBA)
    }
    return image.NewRGBA(image.Rect(0, 0, w, h))
}

func (p *framePool) put(img *image.RGBA) {
    // Clear pixel data
    for i := range img.Pix {
        img.Pix[i] = 0
    }
    p.pool.Put(img)
}

// In video loop:
frame := framePool.get(p.targetW, p.targetH)
utils.CopyRGBToRGBA(frame.Pix, buf)
// ... use frame ...
// Note: can't return to pool if UI is still using it
```

**Expected Result:** Reduced GC pressure, smoother frame delivery

---

### Priority 5: Adaptive Frame Timing

**Change `main.go:9200-9203`:**
```go
// Track actual vs expected time to detect drift
now := time.Now()
behind := now.Sub(nextFrameAt)

if behind < 0 {
    // We're ahead, sleep until next frame
    time.Sleep(-behind)
} else if behind > frameDur*2 {
    // We're way behind (>2 frames), skip this frame
    logging.Debug(logging.CatFFMPEG, "dropping frame, %.0fms behind", behind.Seconds()*1000)
    nextFrameAt = now
    continue
} else {
    // We're slightly behind, catchup gradually
    nextFrameAt = now.Add(frameDur / 2)
}

nextFrameAt = nextFrameAt.Add(frameDur)
```

**Expected Result:** Better handling of temporary slowdowns, adaptive recovery

---

## Testing Checklist

After each fix, test:

- [ ] 24fps video plays smoothly
- [ ] 30fps video plays smoothly
- [ ] 60fps video plays smoothly
- [ ] Audio doesn't stutter
- [ ] A/V sync maintained over 30+ seconds
- [ ] Seeking doesn't cause prolonged stuttering
- [ ] CPU usage is reasonable (<20% for playback)
- [ ] Works on both Linux and Windows
- [ ] Works with various codecs (H.264, H.265, VP9)
- [ ] Volume control works smoothly
- [ ] Pause/resume doesn't cause issues

---

## Performance Monitoring

Add instrumentation to measure:

```go
// Video frame timing
frameDeliveryTime := time.Since(frameReadStart)
if frameDeliveryTime > frameDur*1.5 {
    logging.Debug(logging.CatFFMPEG, "slow frame delivery: %.1fms (target: %.1fms)",
        frameDeliveryTime.Seconds()*1000,
        frameDur.Seconds()*1000)
}

// Audio buffer health
if audioBufferFillLevel < 0.3 {
    logging.Debug(logging.CatFFMPEG, "audio buffer low: %.0f%%", audioBufferFillLevel*100)
}
```

---

## Alternative: Use External Player Library

If these tweaks don't achieve smooth playback, consider:

1. **mpv library** (libmpv) - Industry standard, perfect A/V sync
2. **FFmpeg's ffplay** code - Reference implementation
3. **VLC libvlc** - Proven playback engine

These handle all the complex synchronization automatically.

---

## Summary

**Root Causes:**
1. Separate video/audio processes with no sync
2. Tiny audio buffers causing underruns
3. CPU waste on per-sample volume processing
4. Frame timing drift with no correction
5. UI thread blocking frame updates

**Quick Wins (30 min):**
- Increase audio buffers (Priority 1)
- Move volume to FFmpeg (Priority 2)

**Proper Fix (2-4 hours):**
- Single FFmpeg process with A/V muxing (Priority 3)
- Frame buffer pooling (Priority 4)
- Adaptive timing (Priority 5)

**Expected Final Result:**
- Smooth playback at all frame rates
- Rock-solid A/V sync
- Low CPU usage
- No stuttering or dropouts
