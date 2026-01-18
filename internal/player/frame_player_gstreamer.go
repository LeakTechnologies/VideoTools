//go:build gstreamer

package player

import "fmt"

func newFramePlayer(config Config) (framePlayer, error) {
	if DisableGStreamer {
		return nil, fmt.Errorf("gstreamer disabled by settings")
	}
	return NewGStreamerPlayer(config)
}
