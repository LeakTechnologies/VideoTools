//go:build !native_media

package main

import (
	"time"

	"fyne.io/fyne/v2"
	"github.com/LeakTechnologies/VideoTools/internal/ui"
)

func HasNativeMediaPlayer() bool {
	return false
}

func autoDeinterlaceEnabled() bool { return false }
func setAutoDeinterlace(bool)      {}
func hwDecodeEnabled() bool        { return false }
func setHWDecodeEnabled(bool)      {}
func setHWCodecDenyList(string)    {}
func setPlayerSeekAccuracy(string) {}
func setPlayerAVOffset(int)       {}
func applyPlayerDefaultAspect(string) {}

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

func GetPrimaryPlayer() *ui.InlineVideoPlayer {
	return ui.NewInlineVideoPlayer()
}

func GetPreviewPlayer() *ui.InlineVideoPlayer {
	return ui.NewInlineVideoPlayer()
}

func GetTrimPlayer() *ui.InlineVideoPlayer {
	return GetPrimaryPlayer()
}

func GetInspectPlayer() *ui.InlineVideoPlayer {
	return GetPrimaryPlayer()
}

func GetConvertPlayer() *ui.InlineVideoPlayer {
	return GetPrimaryPlayer()
}

func GetConvertPreviewPlayer() *ui.InlineVideoPlayer {
	return GetPreviewPlayer()
}

func GetSubtitlePlayer() *ui.InlineVideoPlayer {
	return GetPrimaryPlayer()
}

func GetUpscalePlayer() *ui.InlineVideoPlayer {
	return GetPrimaryPlayer()
}

func GetAudioPlayer() *ui.InlineVideoPlayer {
	return GetPrimaryPlayer()
}

func GetFiltersPlayer() *ui.InlineVideoPlayer        { return GetPrimaryPlayer() }
func GetFiltersPreviewPlayer() *ui.InlineVideoPlayer { return GetPreviewPlayer() }
func GetUpscalePreviewPlayer() *ui.InlineVideoPlayer { return GetPreviewPlayer() }

func (s *appState) loadFiltersVideo(_ string)           {}
func (s *appState) applyFiltersPreview()                {}
func (s *appState) loadUpscalePreviewVideo(_ string)    {}
func (s *appState) applyUpscalePreview()                {}
