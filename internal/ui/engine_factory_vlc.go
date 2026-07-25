//go:build native_media && vlc

package ui

import "github.com/LeakTechnologies/VideoTools/internal/media"

// newPlaybackEngine returns the appropriate engine based on the UseVLC setting.
// When the vlc build tag is active and UseVLC is true, creates a libVLC engine.
// Otherwise falls back to the FFmpeg engine.
func newPlaybackEngine() media.PlaybackEngine {
	if media.UseVLC() {
		eng := media.NewVLCEngine()
		if eng != nil {
			return eng
		}
		// VLC init failed — fall through to FFmpeg.
	}
	return media.NewEngine()
}
