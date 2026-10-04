//go:build native_media

package ui

import "github.com/LeakTechnologies/VideoTools/internal/media"

// newPlaybackEngine returns the FFmpeg engine, the only playback backend. It is
// kept behind a factory so a future alternative backend has a single seam; the
// libVLC backend that used to live behind a `vlc` build tag was removed
// unbuilt and unverified — see docs/VLC_PLAYER.md for the retained design.
func newPlaybackEngine() media.PlaybackEngine {
	return media.NewEngine()
}
