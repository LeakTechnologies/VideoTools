package player

import (
	"context"
	"fmt"
	"image"
	"os/exec"
	"sync"
	"time"
)

// VLCController implements VTPlayer using VLC via command-line interface
type VLCController struct {
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	// VLC process
	cmd *exec.Cmd

	// State tracking
	currentPath  string
	currentTime  time.Duration
	currentFrame int64
	duration     time.Duration
	frameRate    float64
	state        PlayerState
	volume       float64
	speed        float64
	muted        bool
	fullscreen   bool
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

	// Process monitoring
	processDone chan struct{}
}

// NewVLCController creates a new VLC-based player
func NewVLCController(config *Config) (*VLCController, error) {
	if config == nil {
		config = &Config{
			Backend:       BackendVLC,
			Volume:        100.0,
			HardwareAccel: true,
			LogLevel:      LogInfo,
		}
	}

	// Check if VLC is available
	if _, err := exec.LookPath("vlc"); err != nil {
		return nil, fmt.Errorf("VLC not found: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	ctrl := &VLCController{
		ctx:         ctx,
		cancel:      cancel,
		state:       StateStopped,
		volume:      config.Volume,
		speed:       1.0,
		config:      config,
		frameRate:   30.0, // Default
		processDone: make(chan struct{}),
	}

	return ctrl, nil
}

// Load loads a video file at specified offset
func (v *VLCController) Load(path string, offset time.Duration) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.setState(StateLoading)

	// Clean up any existing process
	v.stopLocked()

	// Build VLC command
	args := []string{
		"--quiet",
		"--no-video-title-show",
		"--no-stats",
		"--no-disable-screensaver",
		"--play-and-exit", // Exit when done
	}

	// Hardware acceleration
	if v.config.HardwareAccel {
		args = append(args, "--hw-dec=auto")
	}

	// Volume
	args = append(args, fmt.Sprintf("--volume=%.0f", v.volume))

	// Initial seek offset
	if offset > 0 {
		args = append(args, fmt.Sprintf("--start-time=%.3f", float64(offset)/float64(time.Second)))
	}

	// Add the file
	args = append(args, path)

	// Start VLC process
	v.cmd = exec.CommandContext(v.ctx, "vlc", args...)

	// Start the process
	if err := v.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start VLC: %w", err)
	}

	v.currentPath = path

	// Start monitoring
	go v.monitorProcess()
	go v.monitorPosition()

	v.setState(StatePaused)

	// Auto-play if configured
	if v.config.AutoPlay {
		return v.Play()
	}

	return nil
}

// Play starts playback
func (v *VLCController) Play() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.state == StateError || v.currentPath == "" {
		return fmt.Errorf("cannot play: no valid file loaded")
	}

	if v.cmd == nil {
		return fmt.Errorf("VLC process not running")
	}

	// For VLC CLI, playing starts automatically when the file is loaded
	v.setState(StatePlaying)
	return nil
}

// Pause pauses playback
func (v *VLCController) Pause() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.state != StatePlaying {
		return nil
	}

	// VLC CLI doesn't support runtime pause well through command line
	// This would need VLC RC interface for proper control
	v.setState(StatePaused)
	return nil
}

// Stop stops playback and resets position
func (v *VLCController) Stop() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.stopLocked()
	v.currentTime = 0
	v.currentFrame = 0
	v.setState(StateStopped)
	return nil
}

// Close cleans up resources
func (v *VLCController) Close() {
	v.cancel()
	v.mu.Lock()
	defer v.mu.Unlock()
	v.stopLocked()
	v.setState(StateStopped)
}

// stopLocked stops VLC process (must be called with mutex held)
func (v *VLCController) stopLocked() {
	if v.cmd != nil && v.cmd.Process != nil {
		v.cmd.Process.Kill()
		v.cmd.Wait()
	}
	v.cmd = nil
}

// SeekToTime seeks to a specific time with frame accuracy
func (v *VLCController) SeekToTime(offset time.Duration) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.currentPath == "" {
		return fmt.Errorf("no file loaded")
	}

	// VLC CLI doesn't support runtime seeking well
	// This would need VLC RC interface for proper control
	// For now, reload with seek offset
	v.stopLocked()

	args := []string{
		"--quiet",
		"--no-video-title-show",
		"--no-stats",
		"--no-disable-screensaver",
		"--play-and-exit",
	}

	if v.config.HardwareAccel {
		args = append(args, "--hw-dec=auto")
	}

	args = append(args, fmt.Sprintf("--volume=%.0f", v.volume))
	args = append(args, fmt.Sprintf("--start-time=%.3f", float64(offset)/float64(time.Second)))
	args = append(args, v.currentPath)

	v.cmd = exec.CommandContext(v.ctx, "vlc", args...)

	if err := v.cmd.Start(); err != nil {
		return fmt.Errorf("seek failed: %w", err)
	}

	go v.monitorProcess()
	go v.monitorPosition()

	v.currentTime = offset
	if v.frameRate > 0 {
		v.currentFrame = int64(float64(offset) * v.frameRate / float64(time.Second))
	}

	return nil
}

// SeekToFrame seeks to a specific frame number
func (v *VLCController) SeekToFrame(frame int64) error {
	if v.frameRate <= 0 {
		return fmt.Errorf("invalid frame rate")
	}

	offset := time.Duration(float64(frame) * float64(time.Second) / v.frameRate)
	return v.SeekToTime(offset)
}

// GetCurrentTime returns the current playback time
func (v *VLCController) GetCurrentTime() time.Duration {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.currentTime
}

// GetCurrentFrame returns the current frame number
func (v *VLCController) GetCurrentFrame() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.currentFrame
}

// GetFrameRate returns the video frame rate
func (v *VLCController) GetFrameRate() float64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.frameRate
}

// GetDuration returns the total video duration
func (v *VLCController) GetDuration() time.Duration {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.duration
}

// GetVideoInfo returns video metadata
func (v *VLCController) GetVideoInfo() *VideoInfo {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.videoInfo == nil {
		return &VideoInfo{}
	}
	info := *v.videoInfo
	return &info
}

// ExtractFrame extracts a frame at the specified time
func (v *VLCController) ExtractFrame(offset time.Duration) (image.Image, error) {
	// VLC CLI doesn't support frame extraction directly
	// This would need ffmpeg or VLC with special options
	return nil, fmt.Errorf("frame extraction not implemented for VLC backend yet")
}

// ExtractCurrentFrame extracts the currently displayed frame
func (v *VLCController) ExtractCurrentFrame() (image.Image, error) {
	return v.ExtractFrame(v.currentTime)
}

// SetWindow sets the window position and size
func (v *VLCController) SetWindow(x, y, w, h int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.windowX, v.windowY, v.windowW, v.windowH = x, y, w, h
	// VLC CLI doesn't support runtime window control well
}

// SetFullScreen toggles fullscreen mode
func (v *VLCController) SetFullScreen(fullscreen bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.fullscreen == fullscreen {
		return
	}

	v.fullscreen = fullscreen
	// VLC CLI doesn't support runtime fullscreen control well without RC interface
}

// GetWindowSize returns the current window geometry
func (v *VLCController) GetWindowSize() (x, y, w, h int) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.windowX, v.windowY, v.windowW, v.windowH
}

// SetVolume sets the audio volume (0-100)
func (v *VLCController) SetVolume(level float64) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if level < 0 {
		level = 0
	} else if level > 100 {
		level = 100
	}

	v.volume = level
	// VLC CLI doesn't support runtime volume control without RC interface
	return nil
}

// GetVolume returns the current volume level
func (v *VLCController) GetVolume() float64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.volume
}

// SetMuted sets the mute state
func (v *VLCController) SetMuted(muted bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.muted = muted
	// VLC CLI doesn't support runtime mute control without RC interface
}

// IsMuted returns the current mute state
func (v *VLCController) IsMuted() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.muted
}

// SetSpeed sets the playback speed
func (v *VLCController) SetSpeed(speed float64) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if speed <= 0 {
		speed = 0.1
	} else if speed > 10 {
		speed = 10
	}

	v.speed = speed
	// VLC CLI doesn't support runtime speed control without RC interface
	return nil
}

// GetSpeed returns the current playback speed
func (v *VLCController) GetSpeed() float64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.speed
}

// SetTimeCallback sets the time position callback
func (v *VLCController) SetTimeCallback(callback func(time.Duration)) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.timeCallback = callback
}

// SetFrameCallback sets the frame position callback
func (v *VLCController) SetFrameCallback(callback func(int64)) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.frameCallback = callback
}

// SetStateCallback sets the player state callback
func (v *VLCController) SetStateCallback(callback func(PlayerState)) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.stateCallback = callback
}

// EnablePreviewMode enables or disables preview mode
func (v *VLCController) EnablePreviewMode(enabled bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.previewMode = enabled
}

// IsPreviewMode returns whether preview mode is enabled
func (v *VLCController) IsPreviewMode() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.previewMode
}

// Helper methods

func (v *VLCController) setState(state PlayerState) {
	if v.state != state {
		v.state = state
		if v.stateCallback != nil {
			go v.stateCallback(state)
		}
	}
}

func (v *VLCController) monitorProcess() {
	if v.cmd != nil {
		v.cmd.Wait()
	}
	select {
	case v.processDone <- struct{}{}:
	case <-v.ctx.Done():
	}
}

func (v *VLCController) monitorPosition() {
	ticker := time.NewTicker(100 * time.Millisecond) // 10Hz update rate
	defer ticker.Stop()

	for {
		select {
		case <-v.ctx.Done():
			return
		case <-v.processDone:
			return
		case <-ticker.C:
			v.updatePosition()
		}
	}
}

func (v *VLCController) updatePosition() {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.state != StatePlaying || v.cmd == nil {
		return
	}

	// Simple time estimation since we can't easily get position from command-line VLC
	v.currentTime += 100 * time.Millisecond // Rough estimate
	if v.frameRate > 0 {
		v.currentFrame = int64(float64(v.currentTime) * v.frameRate / float64(time.Second))
	}

	// Trigger callbacks
	if v.timeCallback != nil {
		go v.timeCallback(v.currentTime)
	}
	if v.frameCallback != nil {
		go v.frameCallback(v.currentFrame)
	}

	// Check if we've exceeded estimated duration
	if v.duration > 0 && v.currentTime >= v.duration {
		v.setState(StateStopped)
	}
}
