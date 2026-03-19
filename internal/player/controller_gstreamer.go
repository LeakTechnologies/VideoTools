//go:build gstreamer

package player

import (
	"fmt"
	"time"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

// DisableGStreamer forces fallback to the stub controller even when the gstreamer build tag is set.
var DisableGStreamer bool

// newController creates a GStreamer-based controller for embedded video playback
func newController() Controller {
	if DisableGStreamer {
		logging.Info(logging.CatPlayer, "GStreamer disabled by settings; using stub controller")
		return &stubController{}
	}

	config := Config{
		Backend:       BackendAuto,
		WindowWidth:   640,
		WindowHeight:  360,
		Volume:        1.0,
		Muted:         false,
		AutoPlay:      false,
		HardwareAccel: false,
		PreviewMode:   false, // Full playback mode
		AudioOutput:   "auto",
		VideoOutput:   "rgb24",
		CacheEnabled:  true,
		CacheSize:     64 * 1024 * 1024,
		LogLevel:      LogInfo,
	}

	player, err := NewGStreamerPlayer(config)
	if err != nil {
		logging.Warning(logging.CatPlayer, "GStreamer unavailable (deprecated): %v", err)
		return &stubController{}
	}

	logging.Info(logging.CatPlayer, "GStreamer controller initialized (GStreamer %s)", "1.26+")
	return &gstreamerController{
		player: player,
	}
}

// gstreamerController wraps GStreamerPlayer to implement the Controller interface
type gstreamerController struct {
	player *GStreamerPlayer
}

func (c *gstreamerController) Load(path string, offset float64) error {
	if c.player == nil {
		return fmt.Errorf("GStreamer player not initialized")
	}

	offsetDuration := time.Duration(offset * float64(time.Second))
	logging.Debug(logging.CatPlayer, "Loading video: path=%s offset=%.3fs", path, offset)

	return c.player.Load(path, offsetDuration)
}

func (c *gstreamerController) SetWindow(x, y, w, h int) {
	if c.player == nil {
		return
	}
	c.player.SetWindow(x, y, w, h)
}

func (c *gstreamerController) Play() error {
	if c.player == nil {
		return fmt.Errorf("GStreamer player not initialized")
	}
	return c.player.Play()
}

func (c *gstreamerController) Pause() error {
	if c.player == nil {
		return fmt.Errorf("GStreamer player not initialized")
	}
	return c.player.Pause()
}

func (c *gstreamerController) Seek(offset float64) error {
	if c.player == nil {
		return fmt.Errorf("GStreamer player not initialized")
	}

	offsetDuration := time.Duration(offset * float64(time.Second))
	return c.player.SeekToTime(offsetDuration)
}

func (c *gstreamerController) SetVolume(level float64) error {
	if c.player == nil {
		return fmt.Errorf("GStreamer player not initialized")
	}

	// Controller uses 0-100 scale, GStreamer uses 0.0-1.0
	normalizedLevel := level / 100.0
	return c.player.SetVolume(normalizedLevel)
}

func (c *gstreamerController) FullScreen() error {
	if c.player == nil {
		return fmt.Errorf("GStreamer player not initialized")
	}
	return c.player.SetFullScreen(true)
}

func (c *gstreamerController) Stop() error {
	if c.player == nil {
		return fmt.Errorf("GStreamer player not initialized")
	}
	return c.player.Stop()
}

func (c *gstreamerController) Close() {
	if c.player != nil {
		c.player.Close()
	}
}

// stubController provides a no-op implementation when GStreamer fails to initialize
type stubController struct{}

func (s *stubController) Load(path string, offset float64) error {
	return fmt.Errorf("GStreamer player not available")
}

func (s *stubController) SetWindow(x, y, w, h int) {}
func (s *stubController) Play() error              { return fmt.Errorf("GStreamer player not available") }
func (s *stubController) Pause() error             { return fmt.Errorf("GStreamer player not available") }
func (s *stubController) Seek(offset float64) error {
	return fmt.Errorf("GStreamer player not available")
}
func (s *stubController) SetVolume(level float64) error {
	return fmt.Errorf("GStreamer player not available")
}
func (s *stubController) FullScreen() error { return fmt.Errorf("GStreamer player not available") }
func (s *stubController) Stop() error       { return fmt.Errorf("GStreamer player not available") }
func (s *stubController) Close()            {}
