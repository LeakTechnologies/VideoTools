//go:build native_media

package main

import (
	"fyne.io/fyne/v2"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
)

var convertInlinePlayer *ui.InlineVideoPlayer

func init() {
	convertInlinePlayer = ui.NewInlineVideoPlayer()
}

func HasNativeMediaPlayer() bool {
	return true
}

func GetConvertPlayer() *ui.InlineVideoPlayer {
	return convertInlinePlayer
}

func (s *appState) loadVideoNative(path string) {
	convertInlinePlayer.Load(path)
}

func (s *appState) playNative() {
	convertInlinePlayer.Play()
}

func (s *appState) pauseNative() {
	convertInlinePlayer.Pause()
}

func (s *appState) seekNative(target float64) {
	convertInlinePlayer.Seek(target)
}

func (s *appState) stepFrameNative(dir int) {
	convertInlinePlayer.StepFrame(dir)
}

func (s *appState) scrubNative(target float64) {
	convertInlinePlayer.ScrubTo(target)
}

func (s *appState) selectAudioTrackNative(idx int) {
	if err := convertInlinePlayer.SelectAudioTrack(idx); err != nil {
		logging.Error(logging.CatPlayer, "SelectAudioTrack(%d): %v", idx, err)
	}
}

func (s *appState) closeNativePlayer() {
	convertInlinePlayer.Close()
}

func BuildConvertPlayerPane(size fyne.Size) (fyne.CanvasObject, *ui.InlineVideoPlayer) {
	return ui.BuildInlinePlayerPane(size)
}
