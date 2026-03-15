package audio

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

type Options struct {
	Window fyne.Window

	// State fields
	BatchMode            bool
	FileInfoLabel        *widget.Label
	TrackListContainer   *fyne.Container
	BatchListContainer   *fyne.Container
	BatchFiles           []string
	LeftPanel            *fyne.Container
	SingleContent        *fyne.Container
	BatchContent         *fyne.Container
	OutputFormat         string
	Quality              string
	Bitrate              string
	BitrateEntry         *widget.Entry
	Normalize            bool
	NormOptionsContainer *fyne.Container
	OutputDir            string
	StatusLabel          *widget.Label
	ProgressBar          *widget.ProgressBar
	NormTargetLUFS       float64
	NormTruePeak         float64
	Config               AudioConfig
	SourceFile           string
	TrackInfo            []TrackInfo
	IsBusy               bool

	// Callbacks
	OnShowMainMenu             func()
	OnRefreshView              func()
	OnUpdateBatchFilesList     func()
	OnUpdateBitrateVisibility  func()
	OnUpdateBitrateFromQuality func()
	OnUpdateNormVisibility     func()
	OnPersistConfig            func()
	OnLoadFile                 func(path string)
	OnAddBatchFile             func(path string)
	OnClearBatchFiles          func()
	OnStartExtraction          func(queue bool)
	OnShowQueue                func()
	OnClearCompletedJobs       func()
	OnBrowseOutputDir          func() string
}

type AudioConfig struct {
	OutputFormat   string
	Quality        string
	Bitrate        string
	Normalize      bool
	NormTargetLUFS float64
	NormTruePeak   float64
	OutputDir      string
	BatchMode      bool
	BatchFiles     []string
}

type TrackInfo struct {
	Index      int
	Codec      string
	Channels   int
	SampleRate int
	Bitrate    int
	Language   string
	Title      string
	Default    bool
}

func BuildView(opts Options) fyne.CanvasObject {
	// Implementation will go here
	// This is a placeholder for the refactoring
	return nil
}
