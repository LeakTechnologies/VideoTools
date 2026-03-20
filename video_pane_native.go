//go:build native_media

package main

import (
	"fyne.io/fyne/v2"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
)

func (s *appState) setupNativeMediaPlayer(videoPane fyne.CanvasObject, player *ui.InlineVideoPlayer) {
	if player == nil || player.Widget() == nil {
		return
	}

	p := player.Widget()

	p.OnPlay(func() {
		player.Play()
	})

	p.OnPause(func() {
		player.Pause()
	})

	p.OnSeek(func(target float64) {
		player.Seek(target)
	})

	p.OnSpeedChange(func(speed float64) {
		player.SetSpeed(speed)
	})
}
