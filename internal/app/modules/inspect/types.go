//go:build native_media

package inspect

import (
	"fyne.io/fyne/v2"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
)

type InspectState struct {
	VideoPath                 string
	InspectFile               any
	InspectInterlaceResult    any
	InspectInterlaceAnalyzing bool
}

type ViewCallbacks interface {
	Window() fyne.Window
	ShowMainMenu()
	ShowQueue()
	ShowInspectView()
	ClearCompletedJobs()
	StatsBar() fyne.CanvasObject
	OpenLogViewer(title string, path string, isTemp bool)

	LoadFile(path string)
	ClearFile()

	GetFormat() string
	GetVideoCodec() string
	GetWidth() int
	GetHeight() int
	GetAspectRatio() string
	GetFrameRate() float64
	GetBitrate() int64
	GetPixelFormat() string
	GetColorSpace() string
	GetColorRange() string
	GetFieldOrder() string
	GetGOPSize() int
	GetAudioCodec() string
	GetAudioBitrate() int64
	GetAudioRate() int
	GetChannels() int
	GetDuration() string
	GetSampleAspect() string
	GetHasChapters() bool
	GetHasMetadata() bool
	GetTitle() string
	GetPreviewFrame() string
	GetFilePath() string

	Player() *ui.InlineVideoPlayer

	Clipboard() fyne.Clipboard
}
