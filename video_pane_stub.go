//go:build !native_media

package main

import (
	"fyne.io/fyne/v2"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
)

func (s *appState) setupNativeMediaPlayer(videoPane fyne.CanvasObject, player *ui.InlineVideoPlayer) {
}
