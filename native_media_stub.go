//go:build !native_media

package main

import (
	"fyne.io/fyne/v2"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
)

func HasNativeMediaPlayer() bool {
	return false
}

func (s *appState) loadVideoNative(path string)    {}
func (s *appState) playNative()                    {}
func (s *appState) pauseNative()                   {}
func (s *appState) seekNative(target float64)      {}
func (s *appState) stepFrameNative(dir int)        {}
func (s *appState) scrubNative(target float64)        {}
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

func GetTrimPlayer() *ui.InlineVideoPlayer {
	return ui.NewInlineVideoPlayer()
}

func GetInspectPlayer() *ui.InlineVideoPlayer {
	return ui.NewInlineVideoPlayer()
}
