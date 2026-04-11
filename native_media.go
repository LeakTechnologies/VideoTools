//go:build native_media

package main

import (
	"fyne.io/fyne/v2"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
)

var convertInlinePlayer *ui.InlineVideoPlayer
var trimInlinePlayer *ui.InlineVideoPlayer
var inspectInlinePlayer *ui.InlineVideoPlayer
var subtitleInlinePlayer *ui.InlineVideoPlayer

func init() {
	convertInlinePlayer = ui.NewInlineVideoPlayer()
	trimInlinePlayer = ui.NewInlineVideoPlayer()
	inspectInlinePlayer = ui.NewInlineVideoPlayer()
	subtitleInlinePlayer = ui.NewInlineVideoPlayer()
}

func HasNativeMediaPlayer() bool {
	return true
}

func GetConvertPlayer() *ui.InlineVideoPlayer {
	return convertInlinePlayer
}

func GetTrimPlayer() *ui.InlineVideoPlayer {
	return trimInlinePlayer
}

func GetInspectPlayer() *ui.InlineVideoPlayer {
	return inspectInlinePlayer
}

func GetSubtitlePlayer() *ui.InlineVideoPlayer {
	return subtitleInlinePlayer
}

func (s *appState) loadVideoNative(path string) {
	defer func() {
		if r := recover(); r != nil {
			logging.Error(logging.CatPlayer, "panic in loadVideoNative: %v", r)
		}
	}()
	if err := convertInlinePlayer.Load(path); err != nil {
		logging.Error(logging.CatPlayer, "loadVideoNative failed: path=%s err=%v", path, err)
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			ui.ShowToast(s.window, "Native player could not open this file.", ui.ToastWarning)
		}, false)
	}
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

func (s *appState) setVolumeNative(vol float64) {
	convertInlinePlayer.SetVolume(vol)
}

func (s *appState) setMutedNative(muted bool) {
	convertInlinePlayer.SetMuted(muted)
}

func (s *appState) selectSubtitleTrackNative(idx int) {
	if idx < 0 {
		convertInlinePlayer.DisableSubtitles()
		return
	}
	if err := convertInlinePlayer.SelectSubtitleTrack(idx); err != nil {
		logging.Error(logging.CatPlayer, "SelectSubtitleTrack(%d): %v", idx, err)
	}
}

func (s *appState) closeNativePlayer() {
	convertInlinePlayer.Close()
}

func BuildConvertPlayerPane(size fyne.Size) (fyne.CanvasObject, *ui.InlineVideoPlayer) {
	return ui.BuildInlinePlayerPane(size)
}
