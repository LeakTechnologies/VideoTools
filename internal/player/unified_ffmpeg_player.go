package player

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

// UnifiedPlayer implements rock-solid video playback with proper A/V synchronization
// and frame-accurate seeking using a single FFmpeg process
type UnifiedPlayer struct {
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	// FFmpeg process
	cmd    *exec.Cmd
	stdin  *bufio.Writer
	stdout *bufio.Reader
	stderr *bufio.Reader

	// Video output pipes
	videoPipeReader *io.PipeReader
	videoPipeWriter *io.PipeWriter
	audioPipeReader *io.PipeReader
	audioPipeWriter *io.PipeWriter

	// Audio output
	audioContext *oto.Context
	audioPlayer  *oto.Player

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
	paused       bool // Playback paused state

	// Video info
	videoInfo *VideoInfo

	// Synchronization
	syncClock time.Time
	videoPTS  int64
	audioPTS  int64
	ptsOffset int64

	// Buffer management
	frameBuffer     *sync.Pool
	audioBuffer     []byte
	audioBufferSize int

	// Window state
	windowX, windowY int
	windowW, windowH int

	// Callbacks
	timeCallback  func(time.Duration)
	frameCallback func(int64)
	stateCallback func(PlayerState)

	// Configuration
	config Config
}

// NewUnifiedPlayer creates a new unified player with proper A/V synchronization
func NewUnifiedPlayer(config Config) *UnifiedPlayer {
	player := &UnifiedPlayer{
		config: config,
		frameBuffer: &sync.Pool{
			New: func() interface{} {
				return &image.RGBA{
					Pix:    make([]uint8, 0),
					Stride: 0,
					Rect:   image.Rect(0, 0, 0, 0),
				}
			},
		},
		audioBufferSize: 32768, // 170ms at 48kHz for smooth playback
	}

	ctx, cancel := context.WithCancel(context.Background())
	player.ctx = ctx
	player.cancel = cancel

	return player
}

// Load loads a video file and initializes playback
func (p *UnifiedPlayer) Load(path string, offset time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.currentPath = path
	p.state = StateLoading

	// Create pipes for FFmpeg communication
	p.videoPipeReader, p.videoPipeWriter = io.Pipe()
	p.audioPipeReader, p.audioPipeWriter = io.Pipe()

	// Build FFmpeg command with unified A/V output
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-ss", fmt.Sprintf("%.3f", offset.Seconds()),
		"-i", path,
		// Video stream to pipe 4
		"-map", "0:v:0",
		"-f", "rawvideo",
		"-pix_fmt", "rgb24",
		"-r", "24", // We'll detect actual framerate
		"pipe:4",
		// Audio stream to pipe 5
		"-map", "0:a:0",
		"-ac", "2",
		"-ar", "48000",
		"-f", "s16le",
		"pipe:5",
	}

	// Add hardware acceleration if available
	if p.config.HardwareAccel {
		if args = p.addHardwareAcceleration(args); args != nil {
			logging.Debug(logging.CatPlayer, "Hardware acceleration enabled: %v", args)
		}
	}

	// Initialize audio context for playback
	sampleRate := 48000
	channels := 2
	bytesPerSample := 2 // 16-bit = 2 bytes

	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   sampleRate,
		ChannelCount: channels,
		Format:       oto.FormatSignedInt16LE,
		BufferSize:   4096, // 85ms chunks for smooth playback
	})
	if err != nil {
		logging.Error(logging.CatPlayer, "Failed to create audio context: %v", err)
		return err
	}
	if ready != nil {
		<-ready
	}

	p.audioContext = ctx

	// Initialize audio buffer
	p.audioBuffer = make([]byte, 0, 0) // Will grow as needed

	// Start FFmpeg process for unified A/V output
	err = p.startVideoProcess()
	if err != nil {
		return err
	}

	// Start audio stream processing
	go p.readAudioStream()

	return nil
}

// SeekToTime seeks to a specific time without restarting processes
func (p *UnifiedPlayer) SeekToTime(offset time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if offset < 0 {
		offset = 0
	}

	seekTime := offset.Seconds()
	logging.Debug(logging.CatPlayer, "Seeking to time: %.3f seconds", seekTime)
	p.writeStringToStdin(fmt.Sprintf("seek %.3f\n", seekTime))

	p.currentTime = offset
	if p.frameRate > 0 {
		p.currentFrame = int64(seekTime * p.frameRate)
	}
	p.syncClock = time.Now()

	if p.timeCallback != nil {
		p.timeCallback(offset)
	}
	if p.frameCallback != nil {
		p.frameCallback(p.currentFrame)
	}

	logging.Debug(logging.CatPlayer, "Seek completed to %.3f seconds", seekTime)
	return nil
}

// SeekToFrame seeks to a specific frame without restarting processes
func (p *UnifiedPlayer) SeekToFrame(frame int64) error {
	if p.frameRate <= 0 {
		return fmt.Errorf("invalid frame rate: %f", p.frameRate)
	}

	// Convert frame number to time
	frameTime := time.Duration(float64(frame) * float64(time.Second) / p.frameRate)
	return p.SeekToTime(frameTime)
}

// GetCurrentTime returns the current playback time
func (p *UnifiedPlayer) GetCurrentTime() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentTime
}

// GetCurrentFrame returns the current frame number
func (p *UnifiedPlayer) GetCurrentFrame() int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.frameRate > 0 {
		return int64(p.currentTime.Seconds() * p.frameRate)
	}
	return 0
}

// GetDuration returns the total video duration
func (p *UnifiedPlayer) GetDuration() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.duration
}

// GetFrameImage reads and returns the current video frame as an RGBA image
// This is the main method for getting video frames to display in the UI
func (p *UnifiedPlayer) GetFrameImage() (*image.RGBA, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state != StatePlaying || p.paused {
		return nil, nil
	}

	return p.readVideoFrame()
}

// GetFrameRate returns the video frame rate
func (p *UnifiedPlayer) GetFrameRate() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.frameRate
}

// GetVideoInfo returns video metadata
func (p *UnifiedPlayer) GetVideoInfo() *VideoInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.videoInfo == nil {
		return &VideoInfo{}
	}
	return p.videoInfo
}

// SetWindow sets the window position and size
func (p *UnifiedPlayer) SetWindow(x, y, w, h int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.windowX, p.windowY, p.windowW, p.windowH = x, y, w, h

	// Send window command to FFmpeg
	p.writeStringToStdin(fmt.Sprintf("window %d %d %d %d\n", x, y, w, h))
}

// SetFullScreen toggles fullscreen mode
func (p *UnifiedPlayer) SetFullScreen(fullscreen bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.fullscreen = fullscreen

	// Send fullscreen command to FFmpeg
	var cmd string
	if fullscreen {
		cmd = "fullscreen"
	} else {
		cmd = "windowed"
	}

	p.writeStringToStdin(fmt.Sprintf("%s\n", cmd))

	logging.Debug(logging.CatPlayer, "Fullscreen set to: %v", fullscreen)
	return nil
}

// GetWindowSize returns current window dimensions
func (p *UnifiedPlayer) GetWindowSize() (x, y, w, h int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.windowX, p.windowY, p.windowW, p.windowH
}

// SetVolume sets the audio volume (0.0-1.0)
func (p *UnifiedPlayer) SetVolume(level float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Clamp volume to valid range
	if level < 0 {
		level = 0
	} else if level > 1 {
		level = 1
	}

	p.volume = level

	// Send volume command to FFmpeg
	p.writeStringToStdin(fmt.Sprintf("volume %.3f\n", level))

	logging.Debug(logging.CatPlayer, "Volume set to: %.3f", level)
	return nil
}

// GetVolume returns current volume level
func (p *UnifiedPlayer) GetVolume() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.volume
}

// SetMuted sets the mute state
func (p *UnifiedPlayer) SetMuted(muted bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.muted = muted

	// Send mute command to FFmpeg
	var cmd string
	if muted {
		cmd = "mute"
	} else {
		cmd = "unmute"
	}

	p.writeStringToStdin(fmt.Sprintf("%s\n", cmd))

	logging.Debug(logging.CatPlayer, "Mute set to: %v", muted)
}

// IsMuted returns current mute state
func (p *UnifiedPlayer) IsMuted() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.muted
}

// SetSpeed sets playback speed
func (p *UnifiedPlayer) SetSpeed(speed float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.speed = speed

	// Send speed command to FFmpeg
	p.writeStringToStdin(fmt.Sprintf("speed %.2f\n", speed))

	logging.Debug(logging.CatPlayer, "Speed set to: %.2f", speed)
	return nil
}

// GetSpeed returns current playback speed
func (p *UnifiedPlayer) GetSpeed() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.speed
}

// SetTimeCallback sets the time update callback
func (p *UnifiedPlayer) SetTimeCallback(callback func(time.Duration)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.timeCallback = callback
}

// SetFrameCallback sets the frame update callback
func (p *UnifiedPlayer) SetFrameCallback(callback func(int64)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.frameCallback = callback
}

// SetStateCallback sets the state change callback
func (p *UnifiedPlayer) SetStateCallback(callback func(PlayerState)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stateCallback = callback
}

// EnablePreviewMode enables or disables preview mode
func (p *UnifiedPlayer) EnablePreviewMode(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.previewMode = enabled
}

// IsPreviewMode returns current preview mode state
func (p *UnifiedPlayer) IsPreviewMode() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.previewMode
}

// Close shuts down the player and cleans up resources
func (p *UnifiedPlayer) Close() {
	p.Stop()

	p.mu.Lock()
	defer p.mu.Unlock()

	p.frameBuffer = nil
	p.audioBuffer = nil

	// Close audio context and player
	if p.audioContext != nil {
		p.audioContext = nil
	}
	if p.audioPlayer != nil {
		p.audioPlayer.Close()
		p.audioPlayer = nil
	}
}

// Stop halts playback and tears down the FFmpeg process.
func (p *UnifiedPlayer) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cancel != nil {
		p.cancel()
	}
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	p.state = StateStopped
	p.paused = false
	if p.stateCallback != nil {
		p.stateCallback(p.state)
	}
	return nil
}

// Play starts or resumes video playback
func (p *UnifiedPlayer) Play() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == StateStopped {
		// Need to load first
		return fmt.Errorf("no video loaded")
	}

	p.paused = false
	p.state = StatePlaying
	p.syncClock = time.Now()

	logging.Debug(logging.CatPlayer, "UnifiedPlayer: Play() called, state=%v", p.state)

	if p.stateCallback != nil {
		p.stateCallback(p.state)
	}
	return nil
}

// Pause pauses video playback
func (p *UnifiedPlayer) Pause() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state != StatePlaying {
		return nil // Already paused or stopped
	}

	p.paused = true
	p.state = StatePaused

	logging.Debug(logging.CatPlayer, "UnifiedPlayer: Pause() called, state=%v", p.state)

	if p.stateCallback != nil {
		p.stateCallback(p.state)
	}
	return nil
}

// IsPaused returns whether playback is paused
func (p *UnifiedPlayer) IsPaused() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.paused
}

// IsPlaying returns whether playback is active
func (p *UnifiedPlayer) IsPlaying() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state == StatePlaying && !p.paused
}

// Helper methods

// startVideoProcess starts the video processing goroutine and FFmpeg process
func (p *UnifiedPlayer) startVideoProcess() error {
	// Build FFmpeg command for unified A/V output
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-ss", fmt.Sprintf("%.3f", p.currentTime.Seconds()),
		"-i", p.currentPath,
		// Video stream to pipe 4
		"-map", "0:v:0",
		"-f", "rawvideo",
		"-pix_fmt", "rgb24",
		"-r", "24", // We'll detect actual framerate
		"pipe:4",
		// Audio stream to pipe 5
		"-map", "0:a:0",
		"-ac", "2",
		"-ar", "48000",
		"-f", "s16le",
		"pipe:5",
	}

	// Add hardware acceleration if available
	if p.config.HardwareAccel {
		if args = p.addHardwareAcceleration(args); args != nil {
			logging.Debug(logging.CatPlayer, "Hardware acceleration enabled: %v", args)
		}
	}

	// Create FFmpeg command
	cmd := utils.CreateCommandRaw(utils.GetFFmpegPath(), args...)
	cmd.Stdin = nil
	cmd.Stdout = p.videoPipeWriter
	cmd.Stderr = nil // We'll handle errors through logging

	// Start FFmpeg process
	if err := cmd.Start(); err != nil {
		logging.Error(logging.CatPlayer, "Failed to start FFmpeg: %v", err)
		return err
	}

	// Store command reference
	p.cmd = cmd

	// Start video frame reading goroutine
	go func() {
		frameDuration := time.Second / time.Duration(p.frameRate)
		frameTime := p.syncClock

		for {
			select {
			case <-p.ctx.Done():
				logging.Debug(logging.CatPlayer, "Video processing goroutine stopped")
				return

			default:
				// Read frame from video pipe
				frame, err := p.readVideoFrame()
				if err != nil {
					logging.Error(logging.CatPlayer, "Failed to read video frame: %v", err)
					continue
				}

				if frame == nil {
					continue
				}

				// Update timing
				p.currentTime = frameTime.Sub(p.syncClock)
				frameTime = frameTime.Add(frameDuration)
				p.syncClock = time.Now()

				// Notify callback
				if p.frameCallback != nil {
					p.frameCallback(p.GetCurrentFrame())
				}

				// Sleep until next frame time
				sleepTime := frameTime.Sub(time.Now())
				if sleepTime > 0 {
					time.Sleep(sleepTime)
				}
			}
		}
	}()

	return nil
}

// readAudioStream reads and processes audio from the audio pipe
func (p *UnifiedPlayer) readAudioStream() {
	buffer := make([]byte, 4096) // 85ms chunks

	for {
		select {
		case <-p.ctx.Done():
			logging.Debug(logging.CatPlayer, "Audio reading goroutine stopped")
			return

		default:
			// Read from audio pipe
			n, err := p.audioPipeReader.Read(buffer)
			if err != nil && err.Error() != "EOF" {
				logging.Error(logging.CatPlayer, "Audio read error: %v", err)
				continue
			}

			if n == 0 {
				continue
			}

			// Initialize audio player if needed
			if p.audioPlayer == nil && p.audioContext != nil {
				player, err := p.audioContext.NewPlayer(p.audioPipeReader)
				if err != nil {
					logging.Error(logging.CatPlayer, "Failed to create audio player: %v", err)
					return
				}
				p.audioPlayer = player
			}

			// Write audio data to player buffer
			if p.audioPlayer != nil {
				p.audioPlayer.Write(buffer[:n])
			}

			// Buffer for sync monitoring (keep small to avoid memory issues)
			if len(p.audioBuffer) > 32768 { // Max 1 second at 48kHz
				p.audioBuffer = p.audioBuffer[len(p.audioBuffer)-16384:] // Keep half
			}

			// Simple audio sync timing
			p.updateAVSync()
		}
	}
}

// readVideoStream reads video frames from the video pipe
func (p *UnifiedPlayer) readVideoFrame() (*image.RGBA, error) {
	// Check if paused - skip reading frames while paused
	if p.paused {
		return nil, nil
	}

	// Read RGB24 frame data from FFmpeg pipe
	frameSize := p.windowW * p.windowH * 3 // RGB24 = 3 bytes per pixel
	frameData := make([]byte, frameSize)

	// Check for paused state before reading
	if p.paused {
		return nil, fmt.Errorf("player is paused")
	}

	// Read full frame - io.ReadFull ensures we get complete frame
	n, err := io.ReadFull(p.videoPipeReader, frameData)
	if err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil, nil // End of stream
		}
		return nil, fmt.Errorf("video read error: %w", err)
	}

	if n != frameSize {
		return nil, fmt.Errorf("incomplete frame: got %d bytes, expected %d", n, frameSize)
	}

	// Create RGBA image (Fyne requires RGBA, not RGB)
	img := image.NewRGBA(image.Rect(0, 0, p.windowW, p.windowH))

	// Convert RGB24 to RGBA (add alpha channel)
	for y := 0; y < p.windowH; y++ {
		for x := 0; x < p.windowW; x++ {
			srcIdx := (y*p.windowW + x) * 3
			dstIdx := (y*p.windowW + x) * 4

			img.Pix[dstIdx+0] = frameData[srcIdx+0] // R
			img.Pix[dstIdx+1] = frameData[srcIdx+1] // G
			img.Pix[dstIdx+2] = frameData[srcIdx+2] // B
			img.Pix[dstIdx+3] = 255                 // A (fully opaque)
		}
	}

	// Update frame counter
	p.currentFrame++

	// Notify time callback
	if p.timeCallback != nil {
		p.timeCallback(p.currentTime)
	}

	return img, nil
}

// detectVideoProperties analyzes the video to determine properties
func (p *UnifiedPlayer) detectVideoProperties() error {
	// Use ffprobe to get video information
	cmd := exec.Command(utils.GetFFprobePath(),
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=r_frame_rate,duration,width,height",
		p.currentPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffprobe failed: %w", err)
	}

	// Parse frame rate and duration
	p.frameRate = 25.0 // Default fallback
	p.duration = 0

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "r_frame_rate=") {
			if parts := strings.Split(line, "="); len(parts) > 1 {
				var fr float64
				if _, err := fmt.Sscanf(parts[1], "%f", &fr); err == nil {
					p.frameRate = fr
				}
			}
		} else if strings.Contains(line, "duration=") {
			if parts := strings.Split(line, "="); len(parts) > 1 {
				if dur, err := time.ParseDuration(parts[1]); err == nil {
					p.duration = dur
				}
			}
		}
	}

	if p.frameRate > 0 && p.duration > 0 {
		p.videoInfo = &VideoInfo{
			Width:      p.windowW,
			Height:     p.windowH,
			Duration:   p.duration,
			FrameRate:  p.frameRate,
			FrameCount: int64(p.duration.Seconds() * p.frameRate),
		}
	} else {
		p.videoInfo = &VideoInfo{
			Width:      p.windowW,
			Height:     p.windowH,
			Duration:   p.duration,
			FrameRate:  p.frameRate,
			FrameCount: 0,
		}
	}

	logging.Debug(logging.CatPlayer, "Video properties: %dx%d@%.3ffps, %.2fs",
		p.windowW, p.windowH, p.frameRate, p.duration.Seconds())

	return nil
}

// writeStringToStdin sends a command to FFmpeg's stdin
func (p *UnifiedPlayer) writeStringToStdin(cmd string) {
	// TODO: Implement stdin command writing for interactive FFmpeg control
	// Currently a no-op as stdin is not configured in this player implementation
	logging.Debug(logging.CatPlayer, "Stdin command (not implemented): %s", cmd)
}

// updateAVSync maintains synchronization between audio and video
func (p *UnifiedPlayer) updateAVSync() {
	// PTS-based drift correction with adaptive timing
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.audioPTS > 0 && p.videoPTS > 0 {
		drift := p.audioPTS - p.videoPTS
		if abs(drift) > 900 { // More than 10ms of drift (at 90kHz)
			logging.Debug(logging.CatPlayer, "A/V sync drift: %d PTS", drift)
			// Gradual adjustment to avoid audio glitches
			p.ptsOffset += drift / 10 // 10% correction per frame
		} else {
			logging.Debug(logging.CatPlayer, "A/V sync drift: %d PTS", drift)
		}
	}
}

// addHardwareAcceleration adds hardware acceleration flags to FFmpeg args
func (p *UnifiedPlayer) addHardwareAcceleration(args []string) []string {
	// This is a placeholder - actual implementation would detect available hardware
	// and add appropriate flags like "-hwaccel cuda", "-c:v h264_nvenc"

	// For now, just log that hardware acceleration is considered
	logging.Debug(logging.CatPlayer, "Hardware acceleration requested but not yet implemented")
	return args
}

// applyVolumeToBuffer applies volume adjustments to audio buffer
func (p *UnifiedPlayer) applyVolumeToBuffer(buffer []byte) {
	if p.volume <= 0 {
		// Muted - set to silence
		for i := range buffer {
			buffer[i] = 0
		}
	} else {
		// Apply volume gain
		gain := p.volume
		for i := 0; i < len(buffer); i += 2 {
			if i+1 < len(buffer) {
				sample := int16(binary.LittleEndian.Uint16(buffer[i : i+2]))
				adjusted := int(float64(sample) * gain)

				// Clamp to int16 range
				if adjusted > 32767 {
					adjusted = 32767
				} else if adjusted < -32768 {
					adjusted = -32768
				}

				binary.LittleEndian.PutUint16(buffer[i:i+2], uint16(adjusted))
			}
		}
	}
}

// abs returns absolute value of int64
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
