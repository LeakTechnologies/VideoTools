package player

import (
	"image"
	"time"
)

// VTPlayer defines the enhanced player interface with frame-accurate capabilities
type VTPlayer interface {
	// Core playback control
	Load(path string, offset time.Duration) error
	Play() error
	Pause() error
	Stop() error
	Close()

	// Frame-accurate seeking
	SeekToTime(offset time.Duration) error
	SeekToFrame(frame int64) error
	GetCurrentTime() time.Duration
	GetCurrentFrame() int64
	GetFrameRate() float64

	// Video properties
	GetDuration() time.Duration
	GetVideoInfo() *VideoInfo

	// Frame extraction for previews
	ExtractFrame(offset time.Duration) (image.Image, error)
	ExtractCurrentFrame() (image.Image, error)

	// Window and display control
	SetWindow(x, y, w, h int)
	SetFullScreen(fullscreen bool)
	GetWindowSize() (x, y, w, h int)

	// Audio control
	SetVolume(level float64) error
	GetVolume() float64
	SetMuted(muted bool)
	IsMuted() bool

	// Playback speed control
	SetSpeed(speed float64) error
	GetSpeed() float64

	// Events and callbacks
	SetTimeCallback(callback func(time.Duration))
	SetFrameCallback(callback func(int64))
	SetStateCallback(callback func(PlayerState))

	// Preview system support
	EnablePreviewMode(enabled bool)
	IsPreviewMode() bool
}

// VideoInfo contains metadata about the loaded video
type VideoInfo struct {
	Width      int
	Height     int
	Duration   time.Duration
	FrameRate  float64
	BitRate    int64
	Codec      string
	Format     string
	FrameCount int64
}

// PlayerState represents the current playback state
type PlayerState int

const (
    StateStopped PlayerState = iota
    StatePlaying
    StatePaused
    StateLoading
    StateError
)

const (
    StateIdle PlayerState = iota + 100
    StateSeeking
    StateStepping
    StateEOS
)

// BackendType represents the player backend being used
type BackendType int

const (
	BackendMPV BackendType = iota
	BackendVLC
	BackendFFplay
	BackendAuto
)

// Config holds player configuration
type Config struct {
	Backend       BackendType
	WindowX       int
	WindowY       int
	WindowWidth   int
	WindowHeight  int
	Volume        float64
	Muted         bool
	AutoPlay      bool
	HardwareAccel bool
	PreviewMode   bool
	AudioOutput   string
	VideoOutput   string
	CacheEnabled  bool
	CacheSize     int64
	LogLevel      LogLevel
}

// LogLevel for debugging
type LogLevel int

const (
	LogError LogLevel = iota
	LogWarning
	LogInfo
	LogDebug
)


