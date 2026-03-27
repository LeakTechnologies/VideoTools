//go:build !native_media

package ui

import (
	"fyne.io/fyne/v2"
)

type InlineVideoPlayer struct{}

func NewInlineVideoPlayer() *InlineVideoPlayer {
	return &InlineVideoPlayer{}
}

func (v *InlineVideoPlayer) Widget() fyne.CanvasObject {
	return nil
}

func (v *InlineVideoPlayer) Load(path string) error {
	return nil
}

func (v *InlineVideoPlayer) Play()                    {}
func (v *InlineVideoPlayer) Pause()                   {}
func (v *InlineVideoPlayer) Seek(target float64)      {}
func (v *InlineVideoPlayer) SetSpeed(speed float64)   {}
func (v *InlineVideoPlayer) StepFrame(dir int)        {}
func (v *InlineVideoPlayer) Close()                   {}
func (v *InlineVideoPlayer) SeekToChapter(idx int)    {}
func (v *InlineVideoPlayer) ChapterAt(t float64) int  { return -1 }

func BuildInlinePlayerPane(size fyne.Size) (fyne.CanvasObject, *InlineVideoPlayer) {
	return nil, nil
}
