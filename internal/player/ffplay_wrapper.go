package player

import (
	"context"
	"fmt"
	"image"
	"sync"
	"time"
)

// FFplayWrapper wraps the existing ffplay controller to implement VTPlayer interface
type FFplayWrapper struct {
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc

	// Original ffplay controller
	ffplay Controller

	// Enhanced state tracking
	currentTime  time.Duration
	currentFrame int64
	duration     time.Duration
	frameRate    float64
	volume       float64
	speed        float64
	previewMode  bool

	// Window state
	windowX, windowY int
	windowW, windowH int

	// Video info
	videoInfo *VideoInfo

	// Callbacks
	timeCallback  func(time.Duration)
	frameCallback func(int64)
	stateCallback func(PlayerState)

	// Configuration
	config *Config

	// State monitoring
	monitorActive  bool
	lastUpdateTime time.Time
	currentPath    string
	state          PlayerState
}

// NewFFplayWrapper creates a new wrapper around the existing FFplay controller
func NewFFplayWrapper(config *Config) (*FFplayWrapper, error) {
	if config == nil {
		config = &Config{
			Backend: BackendFFplay,
			Volume:  100.0,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create the original ffplay controller
	ffplay := New()

	wrapper := &FFplayWrapper{
		ctx:       ctx,
		cancel:    cancel,
		ffplay:    ffplay,
		volume:    config.Volume,
		speed:     1.0,
		config:    config,
		frameRate: 30.0, // Default, will be updated when file loads
	}

	// Start monitoring for position updates
	go wrapper.monitorPosition()

	return wrapper, nil
}

// Load loads a video file at the specified offset
func (f *FFplayWrapper) Load(path string, offset time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.setState(StateLoading)

	// Set window properties before loading
	if f.windowW > 0 && f.windowH > 0 {
		f.ffplay.SetWindow(f.windowX, f.windowY, f.windowW, f.windowH)
	}

	// Load using the original controller
	if err := f.ffplay.Load(path, float64(offset)/float64(time.Second)); err != nil {
		f.setState(StateError)
		return fmt.Errorf("failed to load file: %w", err)
	}

	f.currentPath = path
	f.currentTime = offset
	f.currentFrame = int64(float64(offset) * f.frameRate / float64(time.Second))

	// Initialize video info (limited capabilities with ffplay)
	f.videoInfo = &VideoInfo{
		Duration:  time.Hour * 24, // Placeholder, will be updated if we can detect
		FrameRate: f.frameRate,
		Width:     0, // Will be updated if detectable
		Height:    0, // Will be updated if detectable
	}

	f.setState(StatePaused)

	// Auto-play if configured
	if f.config.AutoPlay {
		return f.Play()
	}

	return nil
}

// Play starts playback
func (f *FFplayWrapper) Play() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.ffplay.Play(); err != nil {
		return fmt.Errorf("failed to start playback: %w", err)
	}

	f.setState(StatePlaying)
	return nil
}

// Pause pauses playback
func (f *FFplayWrapper) Pause() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.ffplay.Pause(); err != nil {
		return fmt.Errorf("failed to pause playback: %w", err)
	}

	f.setState(StatePaused)
	return nil
}

// Stop stops playback and resets position
func (f *FFplayWrapper) Stop() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.ffplay.Stop(); err != nil {
		return fmt.Errorf("failed to stop playback: %w", err)
	}

	f.currentTime = 0
	f.currentFrame = 0
	f.setState(StateStopped)
	return nil
}

// Close cleans up resources
func (f *FFplayWrapper) Close() {
	f.cancel()
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.ffplay != nil {
		f.ffplay.Close()
	}

	f.setState(StateStopped)
}

// SeekToTime seeks to a specific time with frame accuracy
func (f *FFplayWrapper) SeekToTime(offset time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.ffplay.Seek(float64(offset) / float64(time.Second)); err != nil {
		return fmt.Errorf("seek failed: %w", err)
	}

	f.currentTime = offset
	f.currentFrame = int64(float64(offset) * f.frameRate / float64(time.Second))

	return nil
}

// SeekToFrame seeks to a specific frame number
func (f *FFplayWrapper) SeekToFrame(frame int64) error {
	if f.frameRate <= 0 {
		return fmt.Errorf("invalid frame rate")
	}

	offset := time.Duration(float64(frame) * float64(time.Second) / f.frameRate)
	return f.SeekToTime(offset)
}

// GetCurrentTime returns the current playback time
func (f *FFplayWrapper) GetCurrentTime() time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.currentTime
}

// GetCurrentFrame returns the current frame number
func (f *FFplayWrapper) GetCurrentFrame() int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.currentFrame
}

// GetFrameRate returns the video frame rate
func (f *FFplayWrapper) GetFrameRate() float64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.frameRate
}

// GetDuration returns the total video duration
func (f *FFplayWrapper) GetDuration() time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.duration
}

// GetVideoInfo returns video metadata
func (f *FFplayWrapper) GetVideoInfo() *VideoInfo {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.videoInfo == nil {
		return &VideoInfo{}
	}
	info := *f.videoInfo
	return &info
}

// ExtractFrame extracts a frame at the specified time
func (f *FFplayWrapper) ExtractFrame(offset time.Duration) (image.Image, error) {
	// FFplay doesn't support frame extraction through its interface
	// This would require using ffmpeg directly for frame extraction
	return nil, fmt.Errorf("frame extraction not supported by FFplay backend")
}

// ExtractCurrentFrame extracts the currently displayed frame
func (f *FFplayWrapper) ExtractCurrentFrame() (image.Image, error) {
	return f.ExtractFrame(f.currentTime)
}

// SetWindow sets the window position and size
func (f *FFplayWrapper) SetWindow(x, y, w, h int) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.windowX, f.windowY, f.windowW, f.windowH = x, y, w, h
	f.ffplay.SetWindow(x, y, w, h)
}

// SetFullScreen toggles fullscreen mode
func (f *FFplayWrapper) SetFullScreen(fullscreen bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if fullscreen {
		f.ffplay.FullScreen()
	}
}

// GetWindowSize returns the current window geometry
func (f *FFplayWrapper) GetWindowSize() (x, y, w, h int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.windowX, f.windowY, f.windowW, f.windowH
}

// SetVolume sets the audio volume (0-100)
func (f *FFplayWrapper) SetVolume(level float64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if level < 0 {
		level = 0
	} else if level > 100 {
		level = 100
	}

	f.volume = level
	if err := f.ffplay.SetVolume(level); err != nil {
		return fmt.Errorf("failed to set volume: %w", err)
	}
	return nil
}

// GetVolume returns the current volume level
func (f *FFplayWrapper) GetVolume() float64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.volume
}

// SetMuted sets the mute state
func (f *FFplayWrapper) SetMuted(muted bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// FFplay doesn't have explicit mute control, set volume to 0 instead
	if muted {
		f.ffplay.SetVolume(0)
	} else {
		f.ffplay.SetVolume(f.volume)
	}
}

// IsMuted returns the current mute state
func (f *FFplayWrapper) IsMuted() bool {
	// Since FFplay doesn't have explicit mute, return false
	return false
}

// SetSpeed sets the playback speed
func (f *FFplayWrapper) SetSpeed(speed float64) error {
	// FFplay doesn't support speed changes through the controller interface
	return fmt.Errorf("speed control not supported by FFplay backend")
}

// GetSpeed returns the current playback speed
func (f *FFplayWrapper) GetSpeed() float64 {
	return f.speed
}

// SetTimeCallback sets the time position callback
func (f *FFplayWrapper) SetTimeCallback(callback func(time.Duration)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.timeCallback = callback
}

// SetFrameCallback sets the frame position callback
func (f *FFplayWrapper) SetFrameCallback(callback func(int64)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.frameCallback = callback
}

// SetStateCallback sets the player state callback
func (f *FFplayWrapper) SetStateCallback(callback func(PlayerState)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stateCallback = callback
}

// EnablePreviewMode enables or disables preview mode
func (f *FFplayWrapper) EnablePreviewMode(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.previewMode = enabled
}

// IsPreviewMode returns whether preview mode is enabled
func (f *FFplayWrapper) IsPreviewMode() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.previewMode
}

func (f *FFplayWrapper) setState(newState PlayerState) {
	if f.state != newState {
		f.state = newState
		if f.stateCallback != nil {
			go f.stateCallback(newState)
		}
	}
}

func (f *FFplayWrapper) monitorPosition() {
	ticker := time.NewTicker(100 * time.Millisecond) // 10Hz update rate
	defer ticker.Stop()

	for {
		select {
		case <-f.ctx.Done():
			return
		case <-ticker.C:
			f.updatePosition()
		}
	}
}

func (f *FFplayWrapper) updatePosition() {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.state != StatePlaying {
		return
	}

	// Simple time estimation since we can't get exact position from ffplay
	now := time.Now()
	elapsed := now.Sub(f.lastUpdateTime)
	if !f.lastUpdateTime.IsZero() {
		f.currentTime += time.Duration(float64(elapsed) * f.speed)
		if f.frameRate > 0 {
			f.currentFrame = int64(float64(f.currentTime) * f.frameRate / float64(time.Second))
		}

		// Trigger callbacks
		if f.timeCallback != nil {
			go f.timeCallback(f.currentTime)
		}
		if f.frameCallback != nil {
			go f.frameCallback(f.currentFrame)
		}
	}
	f.lastUpdateTime = now

	// Check if we've exceeded estimated duration
	if f.duration > 0 && f.currentTime >= f.duration {
		f.setState(StateStopped)
	}
}
