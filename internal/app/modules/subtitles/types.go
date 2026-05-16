package subtitles

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/ui"
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

	TimeOffset  float64
	RipStreams  []SubtitleStreamInfo
	RipIndex    int
	RipMode     string
	RipOutput   string
	OCRLanguage string
	OCROutput   string
}

type ViewCallbacks interface {
	Window() fyne.Window
	ShowMainMenu()
	ShowQueue()
	StatsBar() fyne.CanvasObject

	// Player for subtitle preview
	HasPlayer() bool
	PlayerWidget() fyne.CanvasObject
	SetPlayerOnTapEmpty(fn func())
	LoadVideoInPlayer(path string)
	SetProgressCallback(fn func(t float64))

	StopPreview()
	MaximizeWindow()
	SetContent(obj fyne.CanvasObject)
	UpdateQueueButtonLabel()

	QueueBtn() *ui.PillButton
	SetQueueBtn(btn *ui.PillButton)

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
	IsVideoFile(path string) bool
	IsSubtitleFile(path string) bool
	LoadConfig() (SubtitleState, error)
	SaveConfig(cfg SubtitleState) error

	VideoPath() string
	SetVideoPath(path string)
	FilePath() string
	SetFilePath(path string)
	Cues() []SubtitleCue
	SetCues(cues []SubtitleCue)
	UpdateCue(index int, cue SubtitleCue)
	RemoveCue(index int)
	ModelPath() string
	SetModelPath(path string)
	BackendPath() string
	SetBackendPath(path string)
	Status() string
	StatusLabel() *widget.Label
	SetStatusLabel(lbl *widget.Label)
	OutputMode() string
	SetOutputMode(mode string)
	BurnOutput() string
	SetBurnOutput(path string)
	TimeOffset() float64
	SetTimeOffset(offset float64)
	RipStreams() []SubtitleStreamInfo
	SetRipStreams(streams []SubtitleStreamInfo)
	RipIndex() int
	SetRipIndex(index int)
	RipMode() string
	SetRipMode(mode string)
	OCRLanguage() string
	SetOCRLanguage(lang string)
	OCROutput() string
	SetOCROutput(output string)
}
