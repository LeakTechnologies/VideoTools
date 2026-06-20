//go:build native_media

package inspect

import (
	"fyne.io/fyne/v2"
	"github.com/LeakTechnologies/VideoTools/internal/ui"
)

type InspectState struct {
	VideoPath                 string
	InspectFile               any
	InspectInterlaceResult    any
	InspectInterlaceAnalyzing bool
}

type Chapter struct {
	Index     int
	StartTime float64
	EndTime   float64
	Title     string
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
	GetColorTransfer() string
	GetColorPrimaries() string
	GetFieldOrder() string
	GetGOPSize() int
	GetAudioCodec() string
	GetAudioBitrate() int64
	GetAudioRate() int
	GetChannels() int
	GetDuration() string
	GetSampleAspect() string
	GetHasChapters() bool
	GetChapters() []Chapter
	GetEmbeddedCoverArt() string
	SaveMetadata(title, author, description string) error
	GetHasMetadata() bool
	GetTitle() string
	GetPreviewFrame() string
	GetFilePath() string

	Player() *ui.InlineVideoPlayer

	GetClockTime() float64
	GetLastVideoPTS() float64
	GetLastAudioPTS() float64

	Clipboard() fyne.Clipboard
	ModuleFooter(content fyne.CanvasObject) fyne.CanvasObject
}
