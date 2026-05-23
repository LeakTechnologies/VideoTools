//go:build !native_media

package ui

import (
	"image"

	"fyne.io/fyne/v2"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/media"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/media/state"
)

type InlineVideoPlayer struct{}

func NewInlineVideoPlayer() *InlineVideoPlayer { return &InlineVideoPlayer{} }

func (v *InlineVideoPlayer) Widget() fyne.CanvasObject { return nil }

func (v *InlineVideoPlayer) Load(path string) error                     { return nil }
func (v *InlineVideoPlayer) LoadDVD(path string, title int) error        { return nil }
func (v *InlineVideoPlayer) LoadURL(url string, opts map[string]string) error { return nil }

func (v *InlineVideoPlayer) Play()                                {}
func (v *InlineVideoPlayer) Pause()                               {}
func (v *InlineVideoPlayer) Seek(target float64)                  {}
func (v *InlineVideoPlayer) ScrubTo(target float64)               {}
func (v *InlineVideoPlayer) SetSpeed(speed float64)               {}
func (v *InlineVideoPlayer) StepFrame(dir int)                    {}
func (v *InlineVideoPlayer) Close()                               {}
func (v *InlineVideoPlayer) SeekToChapter(idx int)                {}
func (v *InlineVideoPlayer) ChapterAt(t float64) int              { return -1 }
func (v *InlineVideoPlayer) Duration() float64                    { return 0 }
func (v *InlineVideoPlayer) FrameRate() float64                   { return 0 }
func (v *InlineVideoPlayer) CurrentTime() float64                 { return 0 }
func (v *InlineVideoPlayer) GetClockTime() float64                { return -1 }
func (v *InlineVideoPlayer) GetChapters() []media.Chapter         { return nil }
func (v *InlineVideoPlayer) GetAudioTracks() []media.StreamInfo   { return nil }
func (v *InlineVideoPlayer) GetSubtitleTracks() []media.StreamInfo { return nil }
func (v *InlineVideoPlayer) SetOnProgress(fn func(float64))       {}
func (v *InlineVideoPlayer) SetOnEnd(fn func())                   {}
func (v *InlineVideoPlayer) SetOnFrame(fn func(*image.RGBA))      {}
func (v *InlineVideoPlayer) SetOnLoad(fn func(LoadEvent))         {}
func (v *InlineVideoPlayer) SetVolume(vol float64)                {}
func (v *InlineVideoPlayer) SetMuted(muted bool)                  {}
func (v *InlineVideoPlayer) SelectAudioTrack(idx int) error       { return nil }
func (v *InlineVideoPlayer) SelectSubtitleTrack(idx int) error    { return nil }
func (v *InlineVideoPlayer) DisableSubtitles()                    {}
func (v *InlineVideoPlayer) SetOnTapEmpty(fn func())              {}
func (v *InlineVideoPlayer) SetIdleText(text string)              {}
func (v *InlineVideoPlayer) SetIdleAspectRatio(ratio float64)     {}
func (v *InlineVideoPlayer) SetDeinterlaceEnabled(enabled bool)   {}
func (v *InlineVideoPlayer) SetGrowingFile(growing bool)         {}
func (v *InlineVideoPlayer) SetABLoopEnabled(enabled bool)       {}
func (v *InlineVideoPlayer) SetLoopPoints(a, b float64)          {}
func (v *InlineVideoPlayer) SetResumeState(s *state.ResumeState)  {}
func (v *InlineVideoPlayer) RefreshCurrentFrame()                 {}
func (v *InlineVideoPlayer) SetFrameTimingOverlayVisible(bool)    {}

func BuildInlinePlayerPane(size fyne.Size) (fyne.CanvasObject, *InlineVideoPlayer) {
	return nil, nil
}
