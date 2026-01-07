//go:build !gstreamer

package player

func newFramePlayer(config Config) (framePlayer, error) {
	return NewUnifiedPlayer(config), nil
}
