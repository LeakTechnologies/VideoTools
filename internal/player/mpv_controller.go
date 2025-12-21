package player

import (
	"bufio"
	"context"
	"fmt"
	"image"
	"os/exec"
	"sync"
	"time"
)

// MPVController implements VTPlayer using MPV via command-line interface
type MPVController struct {
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	// MPV process
	cmd    *exec.Cmd
	stdin  *bufio.Writer
	stdout *bufio.Reader
	stderr *bufio.Reader

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

// NewMPVController creates a new MPV-based player
func NewMPVController(config *Config) (*MPVController, error) {
	if config == nil {
		config = &Config{
			Backend:       BackendMPV,
			Volume:        100.0,
			HardwareAccel: true,
			LogLevel:      LogInfo,
		}
	}

	// Check if MPV is available
	if _, err := exec.LookPath("mpv"); err != nil {
		return nil, fmt.Errorf("MPV not found: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	ctrl := &MPVController{
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

// Load loads a video file at the specified offset
func (m *MPVController) Load(path string, offset time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.setState(StateLoading)

	// Clean up any existing process
	m.stopLocked()

	// Build MPV command
	args := []string{
		"--no-terminal",
		"--force-window=no",
		"--keep-open=yes",
		"--hr-seek=yes",
		"--hr-seek-framedrop=no",
		"--video-sync=display-resample",
	}

	// Hardware acceleration
	if m.config.HardwareAccel {
		args = append(args, "--hwdec=auto")
	}

	// Volume
	args = append(args, fmt.Sprintf("--volume=%.0f", m.volume))

	// Window geometry
	if m.windowW > 0 && m.windowH > 0 {
		args = append(args, fmt.Sprintf("--geometry=%dx%d+%d+%d", m.windowW, m.windowH, m.windowX, m.windowY))
	}

	// Initial seek offset
	if offset > 0 {
		args = append(args, fmt.Sprintf("--start=%.3f", float64(offset)/float64(time.Second)))
	}

	// Input control
	args = append(args, "--input-ipc-server=/tmp/mpvsocket") // For future IPC control

	// Add the file
	args = append(args, path)

	// Start MPV process
	m.cmd = exec.CommandContext(m.ctx, "mpv", args...)

	// Setup pipes
	stdin, err := m.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := m.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := m.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	m.stdin = bufio.NewWriter(stdin)
	m.stdout = bufio.NewReader(stdout)
	m.stderr = bufio.NewReader(stderr)

	// Start the process
	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MPV: %w", err)
	}

	m.currentPath = path

	// Start monitoring
	go m.monitorProcess()
	go m.monitorOutput()

	m.setState(StatePaused)

	// Auto-play if configured
	if m.config.AutoPlay {
		return m.Play()
	}

	return nil
}

// Play starts playback
func (m *MPVController) Play() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == StateError || m.currentPath == "" {
		return fmt.Errorf("cannot play: no valid file loaded")
	}

	if m.cmd == nil || m.stdin == nil {
		return fmt.Errorf("MPV process not running")
	}

	// Send play command
	if _, err := m.stdin.WriteString("set pause no\n"); err != nil {
		return fmt.Errorf("failed to send play command: %w", err)
	}
	if err := m.stdin.Flush(); err != nil {
		return fmt.Errorf("failed to flush stdin: %w", err)
	}

	m.setState(StatePlaying)
	return nil
}

// Pause pauses playback
func (m *MPVController) Pause() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != StatePlaying {
		return nil
	}

	if m.cmd == nil || m.stdin == nil {
		return fmt.Errorf("MPV process not running")
	}

	// Send pause command
	if _, err := m.stdin.WriteString("set pause yes\n"); err != nil {
		return fmt.Errorf("failed to send pause command: %w", err)
	}
	if err := m.stdin.Flush(); err != nil {
		return fmt.Errorf("failed to flush stdin: %w", err)
	}

	m.setState(StatePaused)
	return nil
}

// Stop stops playback and resets position
func (m *MPVController) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopLocked()
	m.currentTime = 0
	m.currentFrame = 0
	m.setState(StateStopped)
	return nil
}

// Close cleans up resources
func (m *MPVController) Close() {
	m.cancel()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked()
	m.setState(StateStopped)
}

// stopLocked stops the MPV process (must be called with mutex held)
func (m *MPVController) stopLocked() {
	if m.cmd != nil && m.cmd.Process != nil {
		m.cmd.Process.Kill()
		m.cmd.Wait()
	}
	m.cmd = nil
	m.stdin = nil
	m.stdout = nil
	m.stderr = nil
}

// SeekToTime seeks to a specific time with frame accuracy
func (m *MPVController) SeekToTime(offset time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentPath == "" {
		return fmt.Errorf("no file loaded")
	}

	if m.cmd == nil || m.stdin == nil {
		return fmt.Errorf("MPV process not running")
	}

	// Clamp to valid range
	if offset < 0 {
		offset = 0
	}

	// Send seek command
	seekSeconds := float64(offset) / float64(time.Second)
	cmd := fmt.Sprintf("seek %.3f absolute+exact\n", seekSeconds)

	if _, err := m.stdin.WriteString(cmd); err != nil {
		return fmt.Errorf("seek failed: %w", err)
	}
	if err := m.stdin.Flush(); err != nil {
		return fmt.Errorf("seek flush failed: %w", err)
	}

	m.currentTime = offset
	if m.frameRate > 0 {
		m.currentFrame = int64(float64(offset) * m.frameRate / float64(time.Second))
	}

	return nil
}

// SeekToFrame seeks to a specific frame number
func (m *MPVController) SeekToFrame(frame int64) error {
	if m.frameRate <= 0 {
		return fmt.Errorf("invalid frame rate")
	}

	offset := time.Duration(float64(frame) * float64(time.Second) / m.frameRate)
	return m.SeekToTime(offset)
}

// GetCurrentTime returns the current playback time
func (m *MPVController) GetCurrentTime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentTime
}

// GetCurrentFrame returns the current frame number
func (m *MPVController) GetCurrentFrame() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentFrame
}

// GetFrameRate returns the video frame rate
func (m *MPVController) GetFrameRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.frameRate
}

// GetDuration returns the total video duration
func (m *MPVController) GetDuration() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.duration
}

// GetVideoInfo returns video metadata
func (m *MPVController) GetVideoInfo() *VideoInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.videoInfo == nil {
		return &VideoInfo{}
	}
	info := *m.videoInfo
	return &info
}

// ExtractFrame extracts a frame at the specified time
func (m *MPVController) ExtractFrame(offset time.Duration) (image.Image, error) {
	// For now, we'll use ffmpeg for frame extraction
	// This would be a separate implementation
	return nil, fmt.Errorf("frame extraction not implemented for MPV backend yet")
}

// ExtractCurrentFrame extracts the currently displayed frame
func (m *MPVController) ExtractCurrentFrame() (image.Image, error) {
	return m.ExtractFrame(m.currentTime)
}

// SetWindow sets the window position and size
func (m *MPVController) SetWindow(x, y, w, h int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.windowX, m.windowY, m.windowW, m.windowH = x, y, w, h

	// If MPV is running, we could send geometry command
	if m.cmd != nil && m.stdin != nil {
		cmd := fmt.Sprintf("set geometry %dx%d+%d+%d\n", w, h, x, y)
		m.stdin.WriteString(cmd)
		m.stdin.Flush()
	}
}

// SetFullScreen toggles fullscreen mode
func (m *MPVController) SetFullScreen(fullscreen bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.fullscreen == fullscreen {
		return
	}

	m.fullscreen = fullscreen
	if m.cmd != nil && m.stdin != nil {
		cmd := fmt.Sprintf("set fullscreen %v\n", fullscreen)
		m.stdin.WriteString(cmd)
		m.stdin.Flush()
	}
}

// GetWindowSize returns the current window geometry
func (m *MPVController) GetWindowSize() (x, y, w, h int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.windowX, m.windowY, m.windowW, m.windowH
}

// SetVolume sets the audio volume (0-100)
func (m *MPVController) SetVolume(level float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if level < 0 {
		level = 0
	} else if level > 100 {
		level = 100
	}

	m.volume = level
	if m.cmd != nil && m.stdin != nil {
		cmd := fmt.Sprintf("set volume %.0f\n", level)
		if _, err := m.stdin.WriteString(cmd); err != nil {
			return fmt.Errorf("failed to set volume: %w", err)
		}
		if err := m.stdin.Flush(); err != nil {
			return fmt.Errorf("failed to flush volume command: %w", err)
		}
	}
	return nil
}

// GetVolume returns the current volume level
func (m *MPVController) GetVolume() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.volume
}

// SetMuted sets the mute state
func (m *MPVController) SetMuted(muted bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.muted == muted {
		return
	}

	m.muted = muted
	if m.cmd != nil && m.stdin != nil {
		cmd := fmt.Sprintf("set mute %v\n", muted)
		m.stdin.WriteString(cmd)
		m.stdin.Flush()
	}
}

// IsMuted returns the current mute state
func (m *MPVController) IsMuted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.muted
}

// SetSpeed sets the playback speed
func (m *MPVController) SetSpeed(speed float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if speed <= 0 {
		speed = 0.1
	} else if speed > 10 {
		speed = 10
	}

	m.speed = speed
	if m.cmd != nil && m.stdin != nil {
		cmd := fmt.Sprintf("set speed %.2f\n", speed)
		if _, err := m.stdin.WriteString(cmd); err != nil {
			return fmt.Errorf("failed to set speed: %w", err)
		}
		if err := m.stdin.Flush(); err != nil {
			return fmt.Errorf("failed to flush speed command: %w", err)
		}
	}
	return nil
}

// GetSpeed returns the current playback speed
func (m *MPVController) GetSpeed() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.speed
}

// SetTimeCallback sets the time position callback
func (m *MPVController) SetTimeCallback(callback func(time.Duration)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.timeCallback = callback
}

// SetFrameCallback sets the frame position callback
func (m *MPVController) SetFrameCallback(callback func(int64)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.frameCallback = callback
}

// SetStateCallback sets the player state callback
func (m *MPVController) SetStateCallback(callback func(PlayerState)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateCallback = callback
}

// EnablePreviewMode enables or disables preview mode
func (m *MPVController) EnablePreviewMode(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.previewMode = enabled
}

// IsPreviewMode returns whether preview mode is enabled
func (m *MPVController) IsPreviewMode() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.previewMode
}

// Helper methods

func (m *MPVController) setState(state PlayerState) {
	if m.state != state {
		m.state = state
		if m.stateCallback != nil {
			go m.stateCallback(state)
		}
	}
}

func (m *MPVController) monitorProcess() {
	if m.cmd != nil {
		m.cmd.Wait()
	}
	select {
	case m.processDone <- struct{}{}:
	case <-m.ctx.Done():
	}
}

func (m *MPVController) monitorOutput() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.processDone:
			return
		case <-ticker.C:
			m.updatePosition()
		}
	}
}

func (m *MPVController) updatePosition() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != StatePlaying || m.cmd == nil || m.stdin == nil {
		return
	}

	// Simple time estimation since we can't easily get position from command-line MPV
	// In a real implementation, we'd use IPC or parse output
	m.currentTime += 50 * time.Millisecond // Rough estimate
	if m.frameRate > 0 {
		m.currentFrame = int64(float64(m.currentTime) * m.frameRate / float64(time.Second))
	}

	// Trigger callbacks
	if m.timeCallback != nil {
		go m.timeCallback(m.currentTime)
	}
	if m.frameCallback != nil {
		go m.frameCallback(m.currentFrame)
	}

	// Check if we've exceeded estimated duration
	if m.duration > 0 && m.currentTime >= m.duration {
		m.setState(StateStopped)
	}
}
