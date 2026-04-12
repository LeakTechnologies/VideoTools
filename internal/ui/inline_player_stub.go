//go:build !native_media

package ui

import (
	"fyne.io/fyne/v2"
)

type InlineVideoPlayer struct{}

func NewInlineVideoPlayer() *InlineVideoPlayer { return &InlineVideoPlayer{} }

func (v *InlineVideoPlayer) Widget() fyne.CanvasObject { return nil }

func (v *InlineVideoPlayer) Load(path string) error { return nil }

func (v *InlineVideoPlayer) Play()                         {}
func (v *InlineVideoPlayer) Pause()                        {}
func (v *InlineVideoPlayer) Seek(target float64)           {}
func (v *InlineVideoPlayer) ScrubTo(target float64)        {}
func (v *InlineVideoPlayer) SetSpeed(speed float64)        {}
func (v *InlineVideoPlayer) StepFrame(dir int)             {}
func (v *InlineVideoPlayer) Close()                        {}
func (v *InlineVideoPlayer) SeekToChapter(idx int)         {}
func (v *InlineVideoPlayer) ChapterAt(t float64) int       { return -1 }
func (v *InlineVideoPlayer) Duration() float64             { return 0 }
func (v *InlineVideoPlayer) FrameRate() float64            { return 0 }
func (v *InlineVideoPlayer) CurrentTime() float64          { return 0 }
func (v *InlineVideoPlayer) SetOnProgress(fn func(float64)) {}
func (v *InlineVideoPlayer) SetOnEnd(fn func())             {}
func (v *InlineVideoPlayer) SetVolume(vol float64)          {}
func (v *InlineVideoPlayer) SetMuted(muted bool)            {}
func (v *InlineVideoPlayer) SelectAudioTrack(idx int) error { return nil }
func (v *InlineVideoPlayer) SelectSubtitleTrack(idx int) error { return nil }
func (v *InlineVideoPlayer) DisableSubtitles()                 {}
func (v *InlineVideoPlayer) SetOnTapEmpty(fn func())           {}
func (v *InlineVideoPlayer) SetIdleText(text string)           {}

func BuildInlinePlayerPane(size fyne.Size) (fyne.CanvasObject, *InlineVideoPlayer) {
	return nil, nil
}
