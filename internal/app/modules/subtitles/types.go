package subtitles

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

type SubtitleCue struct {
	Start float64
	End   float64
	Text  string
}

type SubtitleStreamInfo struct {
	Index    int
	Codec    string
	Language string
	Title    string
	Default  bool
	Forced   bool
	IsText   bool
	IsImage  bool
}

type ViewCallbacks interface {
	Window() fyne.Window
	ShowMainMenu()
	ShowQueue()
	ShowModule(id string)
	StatsBar() fyne.CanvasObject

	StopPreview()
	MaximizeWindow()
	SetContent(obj fyne.CanvasObject)
	UpdateQueueButtonLabel()

	QueueBtn() *widget.Button
	SetQueueBtn(btn *widget.Button)

	PersistSubtitlesConfig()
	ApplySubtitlesConfig(cfg SubtitleState)

	SetSubtitleStatus(msg string)
	SetSubtitleStatusAsync(msg string)
	LoadSubtitleFile(path string) error
	ApplySubtitleTimeOffset(offsetSeconds float64)
	GenerateSubtitlesFromSpeech()
	ApplySubtitlesToVideo()
	ClearCompletedJobs()
	Clipboard() fyne.Clipboard

	DetectWhisperBackend() string
	DetectWhisperModel() string
}

type SubtitleState struct {
	VideoPath   string
	FilePath    string
	Cues        []SubtitleCue
	ModelPath   string
	BackendPath string
	Status      string
	StatusLabel *widget.Label

	OutputMode  string
	BurnOutput  string
	BurnEnabled bool
	CuesRefresh func()

	TimeOffset  float64
	RipStreams  []SubtitleStreamInfo
	RipIndex    int
	RipMode     string
	RipOutput   string
	OCRLanguage string
	OCROutput   string
}

type Options struct {
	Window    fyne.Window
	StatsBar  fyne.CanvasObject
	OnBack    func()
	BuildView func(cb ViewCallbacks) fyne.CanvasObject
}
