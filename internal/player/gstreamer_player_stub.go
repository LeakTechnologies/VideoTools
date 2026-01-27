//go:build !gstreamer

package player

import (
	"errors"
	"image"
	"time"
)

// Reuse types from vtplayer.go to avoid redeclaration conflicts.
type busEvent struct {
	Type  int
	Info  string
	State PlayerState
}

// GStreamerPlayer is a stub used when the gstreamer build tag is not enabled.
type GStreamerPlayer struct{}

// NewGStreamerPlayer returns an error because GStreamer is not available in this build.
func NewGStreamerPlayer(config Config) (*GStreamerPlayer, error) {
	return nil, errors.New("gstreamer not available; build with -tags gstreamer")
}

func (p *GStreamerPlayer) Load(path string, offset time.Duration) error {
	return errors.New("gstreamer not available")
}
func (p *GStreamerPlayer) Play() error  { return errors.New("gstreamer not available") }
func (p *GStreamerPlayer) Pause() error { return errors.New("gstreamer not available") }
func (p *GStreamerPlayer) SeekToTime(offset time.Duration) error {
	return errors.New("gstreamer not available")
}
func (p *GStreamerPlayer) SeekToFrame(frame int64) error {
	return errors.New("gstreamer not available")
}
func (p *GStreamerPlayer) GetCurrentTime() time.Duration { return 0 }
func (p *GStreamerPlayer) GetFrameImage() (*image.RGBA, error) {
	return nil, errors.New("gstreamer not available")
}
func (p *GStreamerPlayer) GetDuration() time.Duration { return 0 }
func (p *GStreamerPlayer) GetFrameRate() float64     { return 0 }
func (p *GStreamerPlayer) SetVolume(level float64) error {
	return errors.New("gstreamer not available")
}
func (p *GStreamerPlayer) SetWindow(x, y, w, h int) {}
func (p *GStreamerPlayer) SetFullScreen(fullscreen bool) error {
	return errors.New("gstreamer not available")
}
func (p *GStreamerPlayer) Stop() error             { return nil }
func (p *GStreamerPlayer) Close()                  {}
func (p *GStreamerPlayer) Events() <-chan busEvent { return nil }
func (p *GStreamerPlayer) State() PlayerState      { return StateStopped }
