//go:build !native_media

package main

import (
	"time"

	"fyne.io/fyne/v2"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/ui"
)

func HasNativeMediaPlayer() bool {
	return false
}

func hwDecodeEnabled() bool   { return false }
func setHWDecodeEnabled(bool) {}

func initNativeMediaAssets(_ *appState) {}

func (s *appState) loadVideoNative(path string)       {}
func (s *appState) playNative()                       {}
func (s *appState) pauseNative()                      {}
func (s *appState) seekNative(target float64)         {}
func (s *appState) stepFrameNative(dir int)           {}
func (s *appState) scrubNative(target float64)        {}
func (s *appState) renderDualPlayerPreview(seconds float64, duration time.Duration) {}
func (s *appState) selectAudioTrackNative(idx int)    {}
func (s *appState) setVolumeNative(vol float64)       {}
func (s *appState) setMutedNative(muted bool)         {}
func (s *appState) selectSubtitleTrackNative(idx int) {}
func (s *appState) closeNativePlayer()                {}
func BuildConvertPlayerPane(size fyne.Size) (fyne.CanvasObject, interface{}) {
	return nil, nil
}

func buildVideoPaneNative(_ *appState, _ fyne.Size, _ *videoSource, _ func(string)) fyne.CanvasObject {
	return nil
}

func (s *appState) showVideoLoadDialog() {}

func GetTrimPlayer() *ui.InlineVideoPlayer {
	return ui.NewInlineVideoPlayer()
}

func GetInspectPlayer() *ui.InlineVideoPlayer {
	return ui.NewInlineVideoPlayer()
}

func GetConvertPlayer() *ui.InlineVideoPlayer {
	return ui.NewInlineVideoPlayer()
}

func GetConvertPreviewPlayer() *ui.InlineVideoPlayer {
	return ui.NewInlineVideoPlayer()
}

func GetSubtitlePlayer() *ui.InlineVideoPlayer {
	return ui.NewInlineVideoPlayer()
}

func GetUpscalePlayer() *ui.InlineVideoPlayer {
	return ui.NewInlineVideoPlayer()
}

func GetAudioPlayer() *ui.InlineVideoPlayer {
	return ui.NewInlineVideoPlayer()
}

func GetFiltersPlayer() *ui.InlineVideoPlayer        { return ui.NewInlineVideoPlayer() }
func GetFiltersPreviewPlayer() *ui.InlineVideoPlayer { return ui.NewInlineVideoPlayer() }
func GetUpscalePreviewPlayer() *ui.InlineVideoPlayer { return ui.NewInlineVideoPlayer() }

func (s *appState) loadFiltersVideo(_ string)           {}
func (s *appState) applyFiltersPreview()                {}
func (s *appState) loadUpscalePreviewVideo(_ string)    {}
func (s *appState) applyUpscalePreview()                {}
