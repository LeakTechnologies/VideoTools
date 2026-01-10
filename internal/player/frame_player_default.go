//go:build !gstreamer

package player

import "errors"

func newFramePlayer(config Config) (framePlayer, error) {
	return nil, errors.New("GStreamer is required but not available - build with -tags gstreamer")
}
