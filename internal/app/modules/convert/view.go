package convert

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/LeakTechnologies/VideoTools/internal/i18n"
	"github.com/LeakTechnologies/VideoTools/internal/logging"
)

type Options struct {
	Window       fyne.Window
	VideoMinSize fyne.Size
	MetaMinSize  fyne.Size

	OnStopPreview    func()
	OnMaximizeWindow func()
	OnSetContent     func(obj fyne.CanvasObject)
	OnPersistConfig  func()
	OnAddToQueue     func()
	OnAddAllToQueue  func()
	OnConvert        func()
	OnViewLog        func(title, log string, busy bool)
	OnCaptureCover   func() (string, error)
	OnLoadVideo      func(path string)
	OnDroppedFiles   func(paths []string)

	ConvertConfig     ConvertConfigOptions
	LoadedVideos      []string
	QueueTotal        int
	QueueCompleted    int
	IsBusy            bool
	Status            string
	JobQueueRunning   bool
	CurrentJobRunning func() (id, title string, exists bool)
}

type ConvertConfigOptions struct {
	OutputBase             string
	OutputDir              string
	SelectedFormat         string
	SelectedFormatExt      string
	Quality                string
	Mode                   string
	UseAutoNaming          bool
	AutoNameTemplate       string
	AppendSuffix           bool
	PreserveChapters       bool
	VideoCodec             string
	EncoderPreset          string
	CRF                    string
	BitrateMode            string
	BitratePreset          string
	VideoBitrate           string
	TargetFileSize         string
	TargetResolution       string
	FrameRate              string
	UseMotionInterpolation bool
	PixelFormat            string
	HardwareAccel          string
	TwoPass                bool
	H264Profile            string
	H264Level              string
	Deinterlace            string
	DeinterlaceMethod      string
	AutoCrop               bool
	CropWidth              string
	CropHeight             string
	CropX                  string
	CropY                  string
	FlipHorizontal         bool
	FlipVertical           bool
	Rotation               string
	AudioCodec             string
	AudioBitrate           string
	AudioChannels          string
	AudioSampleRate        string
	NormalizeAudio         bool
	InverseTelecine        bool
	CoverArtPath           string
	OutputAspect           string
	AspectUserSet          bool
	ForceAspect            bool
	AspectHandling         string
}

type VideoSourceInfo struct {
	Path              string
	DisplayName       string
	Width             int
	Height            int
	Duration          float64
	FrameRate         float64
	Format            string
	Bitrate           int
	VideoCodec        string
	AudioCodec        string
	AudioBitrate      int
	AudioRate         int
	AudioChannels     string
	FieldOrder        string
	ColorSpace        string
	ColorRange        string
	SampleAspectRatio string
	GOPSize           int
	HasChapters       bool
	HasMetadata       bool
	PreviewFrames     []string
}

func BuildView(opts Options, src *VideoSourceInfo) fyne.CanvasObject {
	logging.Debug(logging.CatUI, "convert module: BuildView called")

	t := i18n.T()
	title := widget.NewLabelWithStyle(t.ModuleConvert, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	content := container.NewVBox(
		title,
		widget.NewLabel("Module placeholder - UI building logic to move here"),
	)

	return container.NewPadded(content)
}

type ConvertState struct {
	LastModule     string
	Active         string
	Source         *VideoSourceInfo
	OutputBase     string
	CoverArtPath   string
	AspectHandling string
	OutputAspect   string
	AspectUserSet  bool
}

type ConvertCallbacks struct {
	OnStopPreview    func()
	OnMaximizeWindow func()
	OnSetContent     func(obj fyne.CanvasObject)
	OnPersistConfig  func()
	OnBuildView      func(src *VideoSourceInfo) fyne.CanvasObject
}

func ShowView(lastModule, active string, file *VideoSourceInfo, state *ConvertState, callbacks ConvertCallbacks) {
	callbacks.OnStopPreview()
	_ = active

	if file != nil {
		state.Source = file
	}

	if state.Source == nil {
		state.OutputBase = "converted"
		state.CoverArtPath = ""
		state.AspectHandling = "Auto"
	}

	if !state.AspectUserSet || state.OutputAspect == "" {
		state.OutputAspect = "Source"
		state.AspectUserSet = false
	}

	callbacks.OnMaximizeWindow()
	callbacks.OnSetContent(callbacks.OnBuildView(state.Source))
}
