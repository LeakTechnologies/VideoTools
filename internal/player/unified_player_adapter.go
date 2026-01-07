package player

import (
	"image"
	"image/color"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

// UnifiedPlayerAdapter wraps UnifiedPlayer to provide playSession interface compatibility
// This allows seamless replacement of the dual-process player with UnifiedPlayer
type UnifiedPlayerAdapter struct {
	// Core UnifiedPlayer
	player *UnifiedPlayer

	// Interface compatibility fields (from playSession)
	path      string
	fps       float64
	width     int
	height    int
	targetW   int
	targetH   int
	volume    float64
	muted     bool
	paused    bool
	current   float64
	stop      chan struct{}
	done      chan struct{}
	prog      func(float64)
	frameFunc func(int) // Callback for frame number updates
	img       *canvas.Image
	mu        sync.Mutex
	frameN    int
	duration  float64 // Total duration in seconds
	startTime time.Time

	// Adapter-specific state
	lastUpdateTime time.Time
	updateTicker   *time.Ticker
}

// NewUnifiedPlayerAdapter creates a new adapter that wraps UnifiedPlayer
func NewUnifiedPlayerAdapter(path string, width, height int, fps, duration float64, targetW, targetH int, prog func(float64), frameFunc func(int), img *canvas.Image) *UnifiedPlayerAdapter {
	adapter := &UnifiedPlayerAdapter{
		path:      path,
		fps:       fps,
		width:     width,
		height:    height,
		targetW:   targetW,
		targetH:   targetH,
		volume:    100.0,
		muted:     false,
		paused:    true,
		current:   0.0,
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
		prog:      prog,
		frameFunc: frameFunc,
		img:       img,
		duration:  duration,
		startTime: time.Now(),
	}

	// Create UnifiedPlayer with proper configuration
	config := Config{
		Backend:       BackendAuto, // Use auto for UnifiedPlayer
		WindowX:       0,
		WindowY:       0,
		WindowWidth:   targetW,
		WindowHeight:  targetH,
		Volume:        1.0, // Full volume
		Muted:         false,
		AutoPlay:      false,
		HardwareAccel: false,
		PreviewMode:   true,
		AudioOutput:   "auto",
		VideoOutput:   "rgb24",
		CacheEnabled:  true,
		CacheSize:     64 * 1024 * 1024, // 64MB
		LogLevel:      3,                // Debug
	}

	adapter.player = NewUnifiedPlayer(config)

	// Set up callbacks for progress and frame updates
	adapter.player.SetTimeCallback(func(d time.Duration) {
		seconds := d.Seconds()
		adapter.current = seconds
		if adapter.prog != nil {
			adapter.prog(seconds)
		}
	})

	adapter.player.SetFrameCallback(func(frame int64) {
		adapter.frameN = int(frame)
		if adapter.frameFunc != nil {
			adapter.frameFunc(int(frame))
		}
	})

	return adapter
}

// Play starts or resumes playback
func (p *UnifiedPlayerAdapter) Play() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.player == nil {
		return
	}

	if p.paused {
		// Load video if not already loaded
		if p.current == 0 {
			err := p.player.Load(p.path, 0)
			if err != nil {
				return
			}
		}

		// Start playback in UnifiedPlayer
		if err := p.player.Play(); err != nil {
			return
		}

		p.paused = false
		p.startTime = time.Now().Add(-time.Duration(p.current * float64(time.Second)))
		p.startUpdateLoop()
		p.startFrameDisplayLoop()
	}
}

// Pause pauses playback
func (p *UnifiedPlayerAdapter) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.player != nil {
		p.player.Pause()
	}
	p.paused = true
	p.stopUpdateLoop()
}

// Seek seeks to the specified time offset
func (p *UnifiedPlayerAdapter) Seek(offset float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if offset < 0 {
		offset = 0
	}
	if offset > p.duration {
		offset = p.duration
	}

	paused := p.paused
	p.current = offset
	p.frameN = int(offset * p.fps)

	// Seek in UnifiedPlayer
	if p.player != nil {
		err := p.player.SeekToTime(time.Duration(offset * float64(time.Second)))
		if err != nil {
			return
		}
	}

	p.paused = paused
	if p.prog != nil {
		p.prog(p.current)
	}
	if p.frameFunc != nil {
		p.frameFunc(p.frameN)
	}
}

// StepFrame moves forward or backward by a specific number of frames
func (p *UnifiedPlayerAdapter) StepFrame(delta int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.fps <= 0 {
		return
	}

	// Calculate current frame from time position
	currentFrame := int(p.current * p.fps)
	targetFrame := currentFrame + delta

	// Clamp to valid range
	if targetFrame < 0 {
		targetFrame = 0
	}
	maxFrame := int(p.duration * p.fps)
	if targetFrame > maxFrame {
		targetFrame = maxFrame
	}

	// Convert to time offset
	offset := float64(targetFrame) / p.fps

	// Seek to the new position
	if p.player != nil {
		err := p.player.SeekToFrame(int64(targetFrame))
		if err != nil {
			return
		}
	}

	p.current = offset
	p.frameN = targetFrame
	p.paused = true // Auto-pause when frame stepping

	if p.prog != nil {
		p.prog(p.current)
	}
	if p.frameFunc != nil {
		p.frameFunc(p.frameN)
	}
}

// GetCurrentFrame returns the current frame number
func (p *UnifiedPlayerAdapter) GetCurrentFrame() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.frameN
}

// SetVolume sets the audio volume (0-100)
func (p *UnifiedPlayerAdapter) SetVolume(v float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.volume = v
	if p.player != nil {
		// Convert 0-100 to 0.0-1.0 range
		volumeLevel := v / 100.0
		err := p.player.SetVolume(volumeLevel)
		if err != nil {
			return
		}
	}
}

// Stop stops playback and cleans up resources
func (p *UnifiedPlayerAdapter) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stopUpdateLoop()

	if p.player != nil {
		p.player.Close()
		p.player = nil
	}

	// Close channels to signal completion
	select {
	case <-p.stop:
	default:
		close(p.stop)
	}
}

// startUpdateLoop starts the update loop for progress tracking
func (p *UnifiedPlayerAdapter) startUpdateLoop() {
	if p.updateTicker != nil {
		return // Already running
	}

	// Update progress based on frame rate (30fps updates)
	interval := time.Second / 30
	p.updateTicker = time.NewTicker(interval)

	go func() {
		defer p.updateTicker.Stop()

		for {
			select {
			case <-p.stop:
				return
			case <-p.updateTicker.C:
				p.mu.Lock()
				if !p.paused && p.player != nil {
					// Drive timeline locally to avoid fighting the frame reader.
					elapsed := time.Since(p.startTime).Seconds()
					if elapsed < 0 {
						elapsed = 0
					}
					if p.duration > 0 && elapsed > p.duration {
						elapsed = p.duration
					}
					p.current = elapsed
					p.frameN = int(p.current * p.fps)

					// Update UI callbacks
					if p.prog != nil {
						p.prog(p.current)
					}
					if p.frameFunc != nil {
						p.frameFunc(p.frameN)
					}
				}
				p.mu.Unlock()
			}
		}
	}()
}

// stopUpdateLoop stops the update loop
func (p *UnifiedPlayerAdapter) stopUpdateLoop() {
	if p.updateTicker != nil {
		p.updateTicker.Stop()
		p.updateTicker = nil
	}
}

// startFrameDisplayLoop starts the loop that reads frames and displays them
func (p *UnifiedPlayerAdapter) startFrameDisplayLoop() {
	if p.player == nil || p.img == nil {
		return
	}

	go func() {
		// Display at frame rate
		frameDuration := time.Second / time.Duration(p.fps)
		ticker := time.NewTicker(frameDuration)
		defer ticker.Stop()

		for {
			select {
			case <-p.stop:
				return
			case <-ticker.C:
				p.mu.Lock()
					if !p.paused && p.player != nil {
						// Get frame from UnifiedPlayer
						frame, err := p.player.GetFrameImage()
						if err == nil && frame != nil {
							fyne.CurrentApp().Driver().DoFromGoroutine(func() {
								p.img.Image = frame
								p.img.Refresh()
							}, false)
						}
					}
				p.mu.Unlock()
			}
		}
	}()
}

// GetVideoFrame returns the current video frame for display
func (p *UnifiedPlayerAdapter) GetVideoFrame() *image.RGBA {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.player == nil {
		return nil
	}

	// Get real frame from UnifiedPlayer
	frame, err := p.player.GetFrameImage()
	if err != nil || frame == nil {
		// Return black frame on error
		rect := image.Rect(0, 0, p.targetW, p.targetH)
		blackFrame := image.NewRGBA(rect)
		for y := 0; y < p.targetH; y++ {
			for x := 0; x < p.targetW; x++ {
				blackFrame.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
			}
		}
		return blackFrame
	}

	return frame
}

// IsPlaying returns whether playback is active
func (p *UnifiedPlayerAdapter) IsPlaying() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return !p.paused
}

// GetDuration returns the total duration in seconds
func (p *UnifiedPlayerAdapter) GetDuration() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.duration
}

// Close closes the adapter and cleans up resources
func (p *UnifiedPlayerAdapter) Close() {
	p.Stop()
}
