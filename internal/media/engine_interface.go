//go:build native_media

package media

import (
	"image"
	"time"

	"github.com/LeakTechnologies/VideoTools/internal/media/filters"
)

// PlaybackEngine is the interface that both the FFmpeg and VLC backends implement.
// InlineVideoPlayer stores this instead of *Engine so the backend is swappable.
// The interface contains only the methods InlineVideoPlayer actually calls.
type PlaybackEngine interface {
	// Lifecycle
	Close()
	Start()

	// Open
	OpenAuto(path string) error
	OpenDVD(devicePath string, title int) error
	OpenURL(url string, opts map[string]string) error

	// Playback control
	Pause()
	Resume()
	IsPaused() bool
	IsRunning() bool
	Seek(seconds float64) error
	NextFrame() (*image.RGBA, error)
	Step(frames int) (*image.RGBA, error)
	GrabFrame(timeout time.Duration) (*image.RGBA, error)
	ResetAfterGrab()
	WaitForFrame(timeout time.Duration) bool

	// Audio
	SetVolume(vol float32)
	SetMuted(muted bool)
	DrainAudio()
	FlushAudioCodec()

	// Speed / timing
	SetSpeed(speed float64)
	CurrentTime() float64
	Duration() float64
	GetFrameRate() float64

	// Seeking / accuracy
	SetSeekAccuracy(acc SeekAccuracy)
	SetAudioDelay(d float64)

	// Deinterlace / growing file / AB loop
	SetDeinterlaceEnabled(enabled bool)
	SetGrowingFile(growing bool)
	IsGrowingFile() bool
	SetABLoopEnabled(enabled bool)
	SetLoopPoints(a, b float64)

	// Frame cache
	InitFrameCache(maxSize int)

	// Chapters
	GetChapters() []Chapter

	// Titles (multi-title discs like DVDs)
	GetTitles() []Title
	SelectTitle(index int) error
	GetCurrentTitle() int

	// Tracks
	GetAudioTracks() []StreamInfo
	SelectAudioTrack(trackIndex int) error
	GetSubtitleTracks() []StreamInfo
	SelectSubtitleTrack(trackIndex int) error
	DisableSubtitles()

	// Filters
	SetFilterPipeline(pipeline *filters.FilterPipeline)

	// HW decode
	SetHWDevice(hw HWDeviceType)

	// PTS diagnostics
	GetLastVideoPTS() float64
	GetLastAudioPTS() float64

	// Drop frames (decode-loop optimization)
	SetDropFrames(enabled bool)

	// Thumbnail extraction
	StartThumbnailExtraction(onFrame func(time float64, img *image.RGBA))

	// Scrubber factory — returns a SmoothScrubbing for FFmpeg, nil for VLC.
	NewScrubber() Scrubber
}

// Scrubber provides smooth scrubbing during slider drag.
// FFmpegBackend returns a SmoothScrubbing; VLCBackend returns nil
// (VLC handles seeking internally via libvlc_media_player_set_position).
type Scrubber interface {
	Start()
	Stop()
	RequestSeek(target float64)
	SetOnFrame(cb func(*image.RGBA))
}
