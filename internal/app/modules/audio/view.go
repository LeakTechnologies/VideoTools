package audio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
	"image/color"
)

var gridColor = utils.MustHex("#2A3A52")
var navyBlue = utils.MustHex("#191F35")

type Options struct {
	Window      fyne.Window
	ModuleColor color.Color

	Player *ui.InlineVideoPlayer

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
	OutputPreviewLabel   *widget.Label // Shows expected output filename

	// Callbacks
	OnShowMainMenu             func()
	OnRefreshView              func()
	OnUpdateBatchFilesList     func()
	OnUpdateBitrateVisibility  func()
	OnUpdateBitrateFromQuality func()
	OnUpdateOutputPreview      func() // Update output filename preview
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
	Duration   float64 // Track duration in seconds
	Language   string
	Title      string
	Default    bool
}

func BuildView(opts Options) fyne.CanvasObject {
	defer logging.RecoverPanic()
	logging.Info(logging.CatModule, "audio.BuildView: entering, Player=%v, ModuleColor=%v", opts.Player != nil, opts.ModuleColor)

	t := i18n.T()

	backBtn := widget.NewButton("< "+strings.ToUpper(t.ModuleAudio), func() {
		if opts.OnShowMainMenu != nil {
			opts.OnShowMainMenu()
		}
	})
	backBtn.Importance = widget.LowImportance

	audioColor := opts.ModuleColor
	if audioColor == nil {
		audioColor = utils.MustHex("#9A7500")
	}
	topBar := ui.TintedBar(audioColor, container.NewHBox(backBtn, layout.NewSpacer()))

	// Video preview pane
	var videoContainer fyne.CanvasObject
	if opts.Player != nil {
		logging.Info(logging.CatModule, "audio.BuildView: calling opts.Player.Widget()")
		videoContainer = opts.Player.Widget()
		logging.Info(logging.CatModule, "audio.BuildView: Widget() returned %v", videoContainer != nil)
	} else {
		logging.Info(logging.CatModule, "audio.BuildView: Player is nil, using SMPTE")
		videoContainer = buildAudioSMPTE()
	}

	// Audio options panel (combines left and right panels)
	logging.Info(logging.CatModule, "audio.BuildView: building left panel")
	leftPanel := buildAudioLeftPanel(opts)
	logging.Info(logging.CatModule, "audio.BuildView: building right panel")
	rightPanel := buildAudioRightPanel(opts)
	audioOptionsPanel := container.NewVBox(leftPanel, rightPanel)
	audioScroll := ui.NewFastVScroll(audioOptionsPanel)

	logging.Info(logging.CatModule, "audio.BuildView: creating HSplit")
	mainSplit := container.NewHSplit(videoContainer, audioScroll)
	mainSplit.SetOffset(0.5)

	extractBtn := widget.NewButton(t.AudioExtractNow, func() {
		if opts.OnStartExtraction != nil {
			opts.OnStartExtraction(false)
		}
	})
	extractBtn.Importance = widget.HighImportance

	queueBtn := widget.NewButton(t.AudioAddToQueue, func() {
		if opts.OnStartExtraction != nil {
			opts.OnStartExtraction(true)
		}
	})

	actionBar := container.NewHBox(
		layout.NewSpacer(),
		extractBtn,
		queueBtn,
	)

	var bottomBar fyne.CanvasObject
	if opts.OnGetStatsBar != nil {
		statsBar := opts.OnGetStatsBar()
		if statsBar != nil {
			audioColor := opts.ModuleColor
			if audioColor == nil {
				audioColor = utils.MustHex("#9A7500")
			}
			bg := canvas.NewRectangle(audioColor)
			bg.SetMinSize(fyne.NewSize(0, 44))
			tinted := container.NewMax(bg, container.NewPadded(actionBar))
			bottomBar = container.NewVBox(statsBar, tinted)
		} else {
			bottomBar = actionBar
		}
	} else {
		bottomBar = actionBar
	}

	return container.NewBorder(topBar, bottomBar, nil, nil, mainSplit)
}

func buildAudioBox(title string, content fyne.CanvasObject) fyne.CanvasObject {
	// Use same styling as Convert module
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

func buildAudioLeftPanel(opts Options) fyne.CanvasObject {
	t := i18n.T()

	dropLabel := widget.NewLabel(t.AudioInstructions)
	dropLabel.Alignment = fyne.TextAlignCenter
	dropLabel.Wrapping = fyne.TextWrapWord

	dropZone := ui.NewDroppable(dropLabel, func(items []fyne.URI) {
		if opts.OnDroppedFiles != nil {
			opts.OnDroppedFiles(items)
		}
	})

	dropContainer := container.NewPadded(dropZone)

	browseBtn := widget.NewButton(t.AudioBrowseForVideo, func() {
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

	fileInfoLabel := widget.NewLabel(t.LabelNoFile)
	fileInfoLabel.Alignment = fyne.TextAlignCenter

	trackListContainer := container.NewVBox()

	selectAllBtn := widget.NewButton(t.AudioSelectAll, nil)
	deselectAllBtn := widget.NewButton(t.AudioDeselectAll, nil)

	trackControls := container.NewHBox(selectAllBtn, deselectAllBtn)

	batchListContainer := container.NewVBox()

	batchModeCheck := widget.NewCheck(t.AudioBatchMode, func(checked bool) {
		opts.BatchMode = checked
		if opts.OnRefreshView != nil {
			opts.OnRefreshView()
		}
	})
	batchModeCheck.SetChecked(opts.BatchMode)

	batchContent := container.NewVBox(
		batchModeCheck,
		container.NewHBox(
			widget.NewButton(t.AudioAddFiles, func() {
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
			widget.NewButton(t.AudioClearFiles, func() {
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
	t := i18n.T()

	formatLabel := widget.NewLabel(t.AudioOutputFormat)
	formatLabel.TextStyle = fyne.TextStyle{Bold: true}

	formatRadio := widget.NewRadioGroup([]string{"MP3", "AAC", "FLAC", "WAV"}, func(value string) {
		opts.OutputFormat = value
		if opts.OnUpdateBitrateVisibility != nil {
			opts.OnUpdateBitrateVisibility()
		}
		if opts.OnUpdateOutputPreview != nil {
			opts.OnUpdateOutputPreview()
		}
		if opts.OnPersistConfig != nil {
			opts.OnPersistConfig()
		}
	})
	formatRadio.Horizontal = true

	outputPreviewLabel := widget.NewLabel("")
	outputPreviewLabel.TextStyle = fyne.TextStyle{Italic: true}
	outputPreviewLabel.Wrapping = fyne.TextWrapWord
	opts.OutputPreviewLabel = outputPreviewLabel

	qualityLabel := widget.NewLabel(t.AudioQualityPreset)
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

	bitrateLabel := widget.NewLabel(t.AudioBitrate)
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

	normalizeCheck := widget.NewCheck(t.AudioNormalization, func(checked bool) {
		opts.Normalize = checked
		if opts.OnUpdateNormVisibility != nil {
			opts.OnUpdateNormVisibility()
		}
		if opts.OnPersistConfig != nil {
			opts.OnPersistConfig()
		}
	})
	normalizeCheck.SetChecked(opts.Normalize)

	lufsLabel := widget.NewLabel(fmt.Sprintf("%s %.1f", t.AudioTargetLUFS, opts.NormTargetLUFS))
	lufsSlider := widget.NewSlider(-30, -10)
	lufsSlider.SetValue(opts.NormTargetLUFS)
	lufsSlider.Step = 0.5
	lufsSlider.OnChanged = func(value float64) {
		opts.NormTargetLUFS = value
		lufsLabel.SetText(fmt.Sprintf("%s %.1f", t.AudioTargetLUFS, value))
		if opts.OnPersistConfig != nil {
			opts.OnPersistConfig()
		}
	}

	peakLabel := widget.NewLabel(fmt.Sprintf("%s %.1f dB", t.AudioTruePeak, opts.NormTruePeak))
	peakSlider := widget.NewSlider(-3, 0)
	peakSlider.SetValue(opts.NormTruePeak)
	peakSlider.Step = 0.1
	peakSlider.OnChanged = func(value float64) {
		opts.NormTruePeak = value
		peakLabel.SetText(fmt.Sprintf("%s %.1f dB", t.AudioTruePeak, value))
		if opts.OnPersistConfig != nil {
			opts.OnPersistConfig()
		}
	}

	normOptions := container.NewVBox(lufsLabel, lufsSlider, peakLabel, peakSlider)

	outputDirLabel := widget.NewLabel(t.AudioOutputDirectory)
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

	statusLabel := widget.NewLabel(t.StatusReady)
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
		buildAudioBox(t.AudioFormat, container.NewVBox(formatLabel, formatRadio)),
		buildAudioBox(t.AudioQuality, container.NewVBox(qualityLabel, qualitySelect)),
		buildAudioBox(t.AudioBitrate, container.NewVBox(bitrateLabel, bitrateEntry)),
		buildAudioBox(t.AudioNormSection, container.NewVBox(normalizeCheck, normOptions)),
		buildAudioBox(t.AudioOutput, container.NewVBox(outputDirLabel, outputDirRow, widget.NewLabel("Output preview:"), outputPreviewLabel, statusLabel, progressBar)),
	)

	scrollable := ui.NewFastVScroll(rightContent)
	return scrollable
}

// buildAudioSMPTE creates a SMPTE color bars widget for the audio module idle state
func buildAudioSMPTE() fyne.CanvasObject {
	t := i18n.T()
	label := widget.NewLabel(t.LabelDropVideoToLoad)
	label.Alignment = fyne.TextAlignCenter
	label.TextStyle = fyne.TextStyle{Bold: true}
	return container.NewCenter(label)
}
