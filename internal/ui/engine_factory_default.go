//go:build native_media && !vlc

package ui

import "github.com/LeakTechnologies/VideoTools/internal/media"

// newPlaybackEngine returns the FFmpeg engine. The vlc build tag is not set,
// so libVLC is not available.
func newPlaybackEngine() media.PlaybackEngine {
	return media.NewEngine()
}
