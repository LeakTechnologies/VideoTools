package audio

import (
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

var gridColor = utils.MustHex("#2A3A52")
var navyBlue = utils.MustHex("#191F35")

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
	OnDroppedFiles             func(paths []fyne.URI)
	OnGetStatsBar              func() fyne.CanvasObject
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
	backBtn := widget.NewButton("< AUDIO", func() {
		if opts.OnShowMainMenu != nil {
			opts.OnShowMainMenu()
		}
	})
	backBtn.Importance = widget.LowImportance

	topBar := ui.TintedBar(utils.MustHex("#FF8F00"), container.NewHBox(backBtn, layout.NewSpacer()))

	leftPanel := buildAudioLeftPanel(opts)
	rightPanel := buildAudioRightPanel(opts)

	mainSplit := container.New(&fixedHSplitLayout{ratio: 0.5}, leftPanel, rightPanel)

	extractBtn := widget.NewButton("Extract Now", func() {
		if opts.OnStartExtraction != nil {
			opts.OnStartExtraction(false)
		}
	})
	extractBtn.Importance = widget.HighImportance

	queueBtn := widget.NewButton("Add to Queue", func() {
		if opts.OnStartExtraction != nil {
			opts.OnStartExtraction(true)
		}
	})

	actionBar := container.NewHBox(
		layout.NewSpacer(),
		extractBtn,
		queueBtn,
	)

	statsBar := widget.NewLabel("")
	if opts.OnGetStatsBar != nil {
		statsBar = opts.OnGetStatsBar().(*widget.Label)
		if statsBar == nil {
			statsBar = widget.NewLabel("")
		}
	}

	bottomBar := ui.ModuleFooter(utils.MustHex("#FF8F00"), actionBar, statsBar)

	return container.NewBorder(topBar, bottomBar, nil, nil, mainSplit)
}

func buildAudioLeftPanel(opts Options) fyne.CanvasObject {
	dropLabel := widget.NewLabel("Drop video file here or click to browse")
	dropLabel.Alignment = fyne.TextAlignCenter

	dropZone := ui.NewDroppable(dropLabel, func(items []fyne.URI) {
		if opts.OnDroppedFiles != nil {
			opts.OnDroppedFiles(items)
		}
	})

	dropContainer := container.NewPadded(dropZone)

	browseBtn := widget.NewButton("Browse for Video", func() {
		dialog.ShowFileOpen(func(uc fyne.URIReadCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			defer uc.Close()
			if opts.OnLoadFile != nil {
				opts.OnLoadFile(uc.URI().Path())
			}
		}, opts.Window)
	})

	fileInfoLabel := widget.NewLabel("No file loaded")
	fileInfoLabel.Alignment = fyne.TextAlignCenter

	trackListContainer := container.NewVBox()

	selectAllBtn := widget.NewButton("Select All", nil)
	deselectAllBtn := widget.NewButton("Deselect All", nil)

	trackControls := container.NewHBox(selectAllBtn, deselectAllBtn)

	batchListContainer := container.NewVBox()

	batchModeCheck := widget.NewCheck("Batch Mode (process multiple files)", func(checked bool) {
		opts.BatchMode = checked
		if opts.OnRefreshView != nil {
			opts.OnRefreshView()
		}
	})
	batchModeCheck.SetChecked(opts.BatchMode)

	batchContent := container.NewVBox(
		batchModeCheck,
		container.NewHBox(
			widget.NewButton("Add Files", func() {
				dialog.ShowFileOpen(func(uc fyne.URIReadCloser, err error) {
					if err != nil || uc == nil {
						return
					}
					defer uc.Close()
					if opts.OnAddBatchFile != nil {
						opts.OnAddBatchFile(uc.URI().Path())
					}
				}, opts.Window)
			}),
			widget.NewButton("Clear", func() {
				if opts.OnClearBatchFiles != nil {
					opts.OnClearBatchFiles()
				}
			}),
		),
		container.NewVScroll(batchListContainer),
	)

	singleContent := container.NewVBox(
		browseBtn,
		fileInfoLabel,
		trackControls,
		container.NewVScroll(trackListContainer),
	)

	leftContent := container.NewMax()
	if opts.BatchMode {
		leftContent.Objects = []fyne.CanvasObject{batchContent}
	} else {
		leftContent.Objects = []fyne.CanvasObject{singleContent}
	}

	return container.NewPadded(container.NewVBox(dropContainer, leftContent))
}

func buildAudioRightPanel(opts Options) fyne.CanvasObject {
	formatLabel := widget.NewLabel("Output Format:")
	formatLabel.TextStyle = fyne.TextStyle{Bold: true}

	formatRadio := widget.NewRadioGroup([]string{"MP3", "AAC", "FLAC", "WAV"}, func(value string) {
		opts.OutputFormat = value
		if opts.OnUpdateBitrateVisibility != nil {
			opts.OnUpdateBitrateVisibility()
		}
		if opts.OnPersistConfig != nil {
			opts.OnPersistConfig()
		}
	})
	formatRadio.Horizontal = true

	qualityLabel := widget.NewLabel("Quality Preset:")
	qualityLabel.TextStyle = fyne.TextStyle{Bold: true}

	qualitySelect := widget.NewSelect([]string{"Low", "Medium", "High", "Lossless"}, func(value string) {
		opts.Quality = value
		if opts.OnUpdateBitrateFromQuality != nil {
			opts.OnUpdateBitrateFromQuality()
		}
		if opts.OnPersistConfig != nil {
			opts.OnPersistConfig()
		}
	})

	bitrateLabel := widget.NewLabel("Bitrate:")
	bitrateEntry := widget.NewEntry()
	bitrateEntry.SetText(opts.Bitrate)
	bitrateEntry.OnChanged = func(value string) {
		opts.Bitrate = value
		if opts.OnPersistConfig != nil {
			opts.OnPersistConfig()
		}
	}

	qualitySelect.SetSelected(opts.Quality)
	formatRadio.SetSelected(opts.OutputFormat)

	normalizeCheck := widget.NewCheck("Apply EBU R128 Normalization", func(checked bool) {
		opts.Normalize = checked
		if opts.OnUpdateNormVisibility != nil {
			opts.OnUpdateNormVisibility()
		}
		if opts.OnPersistConfig != nil {
			opts.OnPersistConfig()
		}
	})
	normalizeCheck.SetChecked(opts.Normalize)

	lufsLabel := widget.NewLabel(fmt.Sprintf("Target LUFS: %.1f", opts.NormTargetLUFS))
	lufsSlider := widget.NewSlider(-30, -10)
	lufsSlider.SetValue(opts.NormTargetLUFS)
	lufsSlider.Step = 0.5
	lufsSlider.OnChanged = func(value float64) {
		opts.NormTargetLUFS = value
		lufsLabel.SetText(fmt.Sprintf("Target LUFS: %.1f", value))
		if opts.OnPersistConfig != nil {
			opts.OnPersistConfig()
		}
	}

	peakLabel := widget.NewLabel(fmt.Sprintf("True Peak: %.1f dB", opts.NormTruePeak))
	peakSlider := widget.NewSlider(-3, 0)
	peakSlider.SetValue(opts.NormTruePeak)
	peakSlider.Step = 0.1
	peakSlider.OnChanged = func(value float64) {
		opts.NormTruePeak = value
		peakLabel.SetText(fmt.Sprintf("True Peak: %.1f dB", value))
		if opts.OnPersistConfig != nil {
			opts.OnPersistConfig()
		}
	}

	normOptions := container.NewVBox(lufsLabel, lufsSlider, peakLabel, peakSlider)

	outputDirLabel := widget.NewLabel("Output Directory:")
	outputDirLabel.TextStyle = fyne.TextStyle{Bold: true}

	outputDirEntry := widget.NewEntry()
	if opts.OutputDir == "" {
		home, _ := os.UserHomeDir()
		opts.OutputDir = filepath.Join(home, "Music", "VideoTools", "AudioExtract")
	}
	outputDirEntry.SetText(opts.OutputDir)
	outputDirEntry.OnChanged = func(value string) {
		opts.OutputDir = value
		if opts.OnPersistConfig != nil {
			opts.OnPersistConfig()
		}
	}

	outputDirBrowseBtn := widget.NewButton("Browse", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			opts.OutputDir = uri.Path()
			outputDirEntry.SetText(uri.Path())
			if opts.OnPersistConfig != nil {
				opts.OnPersistConfig()
			}
		}, opts.Window)
	})

	outputDirRow := container.NewBorder(nil, nil, nil, outputDirBrowseBtn, outputDirEntry)

	statusLabel := widget.NewLabel("Ready")
	progressBar := widget.NewProgressBar()
	progressBar.Hide()

	buildAudioBox := func(title string, content fyne.CanvasObject) fyne.CanvasObject {
		bg := canvas.NewRectangle(navyBlue)
		bg.CornerRadius = 10
		bg.StrokeColor = gridColor
		bg.StrokeWidth = 1
		body := container.NewVBox(
			widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewSeparator(),
			content,
		)
		layers := ui.NoisyBackgroundObjects(bg)
		layers = append(layers, container.NewPadded(body))
		return container.NewMax(layers...)
	}

	rightContent := container.NewVBox(
		buildAudioBox("Format", container.NewVBox(formatLabel, formatRadio)),
		buildAudioBox("Quality", container.NewVBox(qualityLabel, qualitySelect)),
		buildAudioBox("Bitrate", container.NewVBox(bitrateLabel, bitrateEntry)),
		buildAudioBox("Normalization", container.NewVBox(normalizeCheck, normOptions)),
		buildAudioBox("Output", container.NewVBox(outputDirLabel, outputDirRow, statusLabel, progressBar)),
	)

	scrollable := ui.NewFastVScroll(rightContent)
	return scrollable
}

type fixedHSplitLayout struct {
	ratio float32
}

func (f *fixedHSplitLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 2 {
		return
	}
	width := float32(size.Width)
	leftWidth := float32(int(width * f.ratio))
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(fyne.NewSize(leftWidth, size.Height))
	objects[1].Move(fyne.NewPos(leftWidth, 0))
	objects[1].Resize(fyne.NewSize(size.Width-leftWidth, size.Height))
}

func (f *fixedHSplitLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) < 2 {
		return fyne.NewSize(0, 0)
	}
	min1 := objects[0].MinSize()
	min2 := objects[1].MinSize()
	return fyne.NewSize(min1.Width+min2.Width, max(min1.Height, min2.Height))
}
