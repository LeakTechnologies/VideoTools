//go:build gstreamer

package player

func newFramePlayer(config Config) (framePlayer, error) {
	if gstPlayer, err := NewGStreamerPlayer(config); err == nil {
		return gstPlayer, nil
	}
	return NewUnifiedPlayer(config), nil
}
