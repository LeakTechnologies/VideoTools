package filters

import (
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
	"image/color"
)

var gridColor = utils.MustHex("#2A3A52")
var navyBlue = utils.MustHex("#191F35")

type Options struct {
	Window      fyne.Window
	ModuleColor color.Color

	FilterBrightness    float64
	FilterContrast      float64
	FilterSaturation    float64
	FilterSharpness     float64
	FilterDenoise       float64
	FilterGrayscale     bool
	FilterFlipH         bool
	FilterFlipV         bool
	FilterRotation      int
	FilterStylisticMode string
	FilterScanlines     bool
	FilterChromaNoise   float64
	FilterColorBleeding bool
	FilterTapeNoise     float64
	FilterTrackingError float64
	FilterDropout       float64
	FilterInterlacing   string
	FilterInterpEnabled bool
	FilterInterpPreset  string
	FilterInterpFPS     string
	FiltersFile         any
	FiltersFilePath     string
	FilterActiveChain   []string

	OnShowMainMenu       func()
	OnShowQueue          func()
	OnShowUpscaleView    func()
	OnShowFiltersView    func()
	OnClearCompletedJobs func()
	OnGetStatsBar        func() fyne.CanvasObject

	OnLoadFile      func(path string)
	OnUpdateFile    func(file any)
	OnSendToUpscale func()
	OnApplyFilters  func()
	OnFilterNow     func()

	OnPersistConfig func()

	OnSetBrightness    func(f float64)
	OnSetContrast      func(f float64)
	OnSetSaturation    func(f float64)
	OnSetSharpness     func(f float64)
	OnSetDenoise       func(f float64)
	OnSetGrayscale     func(b bool)
	OnSetFlipH         func(b bool)
	OnSetFlipV         func(b bool)
	OnSetRotation      func(i int)
	OnSetStylisticMode func(s string)
	OnSetScanlines     func(b bool)
	OnSetChromaNoise   func(f float64)
	OnSetColorBleeding func(b bool)
	OnSetTapeNoise     func(f float64)
	OnSetTrackingError func(f float64)
	OnSetDropout       func(f float64)
	OnSetInterlacing   func(s string)
	OnSetInterpEnabled func(b bool)
	OnSetInterpPreset  func(s string)
	OnSetInterpFPS     func(s string)

	OnBuildFilterChain func() []string
	OnAddToQueue       func()
	OnDroppedFiles     func(paths []fyne.URI)

	OnProbeVideo           func(path string) (interface{}, error)
	OnBuildVideoPane       func(state interface{}, size fyne.Size, src interface{}, overlay fyne.CanvasObject) fyne.CanvasObject
	OnHasNativeMediaPlayer func() bool
	OnLoadVideoNative      func(path string)
}

func BuildView(opts Options) fyne.CanvasObject {
	t := i18n.T()
	filtersColor := opts.ModuleColor
	if filtersColor == nil {
		filtersColor = utils.MustHex("#005F5F")
	}

	backBtn := widget.NewButton("< "+strings.ToUpper(t.ModuleFilters), func() {
		if opts.OnShowMainMenu != nil {
			opts.OnShowMainMenu()
		}
	})
	backBtn.Importance = widget.LowImportance

	queueBtn := widget.NewButton("View Queue", func() {
		if opts.OnShowQueue != nil {
			opts.OnShowQueue()
		}
	})

	clearCompletedBtn := widget.NewButton("⌫", func() {
		if opts.OnClearCompletedJobs != nil {
			opts.OnClearCompletedJobs()
		}
	})
	clearCompletedBtn.Importance = widget.LowImportance

	topBar := ui.TintedBar(filtersColor, container.NewHBox(backBtn, layout.NewSpacer(), clearCompletedBtn, queueBtn))

	instructions := widget.NewLabel(t.FiltersInstructions)
	instructions.Wrapping = fyne.TextWrapWord
	instructions.Alignment = fyne.TextAlignCenter

	buildFilterChain := func() {
		if opts.OnBuildFilterChain != nil {
			opts.OnBuildFilterChain()
		}
	}

	fileLabel := widget.NewLabel(t.LabelNoFile)
	fileLabel.TextStyle = fyne.TextStyle{Bold: true}

	var videoContainer fyne.CanvasObject
	if opts.FiltersFile != nil {
		if opts.FiltersFilePath != "" {
			fileLabel.SetText(fmt.Sprintf(t.LabelFileFmt, filepath.Base(opts.FiltersFilePath)))
		} else {
			fileLabel.SetText(fmt.Sprintf(t.LabelFileFmt, "video loaded"))
		}
		videoContainer = opts.OnBuildVideoPane(nil, fyne.NewSize(480, 270), opts.FiltersFile, nil)
		if opts.OnHasNativeMediaPlayer != nil && opts.OnHasNativeMediaPlayer() && opts.FiltersFilePath != "" {
			go opts.OnLoadVideoNative(opts.FiltersFilePath)
		}
	} else {
		videoContainer = opts.OnBuildVideoPane(nil, fyne.NewSize(480, 270), nil, nil)
	}

	loadBtn := widget.NewButton("Load Video", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			path := reader.URI().Path()
			if opts.OnLoadFile != nil {
				opts.OnLoadFile(path)
			}
		}, opts.Window)
	})
	loadBtn.Importance = widget.HighImportance

	upscaleNavBtn := widget.NewButton("Send to Upscale →", func() {
		if opts.OnSendToUpscale != nil {
			opts.OnSendToUpscale()
		}
		if opts.OnShowUpscaleView != nil {
			opts.OnShowUpscaleView()
		}
	})

	buildFilterBox := func(title string, content fyne.CanvasObject) fyne.CanvasObject {
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

	brightnessSlider := widget.NewSlider(-1.0, 1.0)
	brightnessSlider.SetValue(opts.FilterBrightness)
	brightnessSlider.OnChanged = func(f float64) {
		if opts.OnSetBrightness != nil {
			opts.OnSetBrightness(f)
		}
		buildFilterChain()
	}

	contrastSlider := widget.NewSlider(0.0, 3.0)
	contrastSlider.SetValue(opts.FilterContrast)
	contrastSlider.OnChanged = func(f float64) {
		if opts.OnSetContrast != nil {
			opts.OnSetContrast(f)
		}
		buildFilterChain()
	}

	saturationSlider := widget.NewSlider(0.0, 3.0)
	saturationSlider.SetValue(opts.FilterSaturation)
	saturationSlider.OnChanged = func(f float64) {
		if opts.OnSetSaturation != nil {
			opts.OnSetSaturation(f)
		}
		buildFilterChain()
	}

	colorSection := buildFilterBox("Color Correction", container.NewVBox(
		widget.NewLabel(t.FiltersSectionBrightness),
		container.NewGridWithColumns(2,
			widget.NewLabel(t.FiltersBrightness),
			brightnessSlider,
			widget.NewLabel(t.FiltersContrast),
			contrastSlider,
			widget.NewLabel(t.FiltersSaturation),
			saturationSlider,
		),
	))

	sharpnessSlider := widget.NewSlider(0.0, 5.0)
	sharpnessSlider.SetValue(opts.FilterSharpness)
	sharpnessSlider.OnChanged = func(f float64) {
		if opts.OnSetSharpness != nil {
			opts.OnSetSharpness(f)
		}
		buildFilterChain()
	}

	denoiseSlider := widget.NewSlider(0.0, 10.0)
	denoiseSlider.SetValue(opts.FilterDenoise)
	denoiseSlider.OnChanged = func(f float64) {
		if opts.OnSetDenoise != nil {
			opts.OnSetDenoise(f)
		}
		buildFilterChain()
	}

	enhanceSection := buildFilterBox("Enhancement", container.NewVBox(
		widget.NewLabel(t.FiltersSectionSharpen),
		container.NewGridWithColumns(2,
			widget.NewLabel(t.FiltersSharpness),
			sharpnessSlider,
			widget.NewLabel(t.FiltersDenoise),
			denoiseSlider,
		),
	))

	rotationSelect := widget.NewSelect([]string{"0°", "90°", "180°", "270°"}, func(s string) {
		var rot int
		switch s {
		case "90°":
			rot = 90
		case "180°":
			rot = 180
		case "270°":
			rot = 270
		}
		if opts.OnSetRotation != nil {
			opts.OnSetRotation(rot)
		}
		buildFilterChain()
	})

	var rotationStr string
	switch opts.FilterRotation {
	case 90:
		rotationStr = "90°"
	case 180:
		rotationStr = "180°"
	case 270:
		rotationStr = "270°"
	default:
		rotationStr = "0°"
	}
	rotationSelect.SetSelected(rotationStr)

	flipHCheck := widget.NewCheck("", func(b bool) {
		if opts.OnSetFlipH != nil {
			opts.OnSetFlipH(b)
		}
		buildFilterChain()
	})
	flipHCheck.SetChecked(opts.FilterFlipH)

	flipVCheck := widget.NewCheck("", func(b bool) {
		if opts.OnSetFlipV != nil {
			opts.OnSetFlipV(b)
		}
		buildFilterChain()
	})
	flipVCheck.SetChecked(opts.FilterFlipV)

	transformSection := buildFilterBox("Transform", container.NewVBox(
		widget.NewLabel(t.FiltersSectionRotate),
		container.NewGridWithColumns(2,
			widget.NewLabel(t.FiltersRotation),
			rotationSelect,
			widget.NewLabel(t.FiltersFlipHorizontal),
			flipHCheck,
			widget.NewLabel(t.FiltersFlipVertical),
			flipVCheck,
		),
	))

	grayscaleCheck := widget.NewCheck("Grayscale", func(b bool) {
		if opts.OnSetGrayscale != nil {
			opts.OnSetGrayscale(b)
		}
		buildFilterChain()
	})
	grayscaleCheck.SetChecked(opts.FilterGrayscale)

	creativeSection := buildFilterBox("Creative Effects", container.NewVBox(
		widget.NewLabel(t.FiltersSectionArtistic),
		grayscaleCheck,
	))

	stylisticModeSelect := widget.NewSelect([]string{"None", "8mm Film", "16mm Film", "B&W Film", "Silent Film", "70s", "80s", "90s", "VHS", "Webcam"}, func(s string) {
		if opts.OnSetStylisticMode != nil {
			opts.OnSetStylisticMode(s)
		}
		buildFilterChain()
	})
	stylisticModeSelect.SetSelected(opts.FilterStylisticMode)

	scanlinesCheck := widget.NewCheck("CRT Scanlines", func(b bool) {
		if opts.OnSetScanlines != nil {
			opts.OnSetScanlines(b)
		}
		buildFilterChain()
	})
	scanlinesCheck.SetChecked(opts.FilterScanlines)

	chromaNoiseSlider := widget.NewSlider(0.0, 1.0)
	chromaNoiseSlider.SetValue(opts.FilterChromaNoise)
	chromaNoiseSlider.OnChanged = func(f float64) {
		if opts.OnSetChromaNoise != nil {
			opts.OnSetChromaNoise(f)
		}
		buildFilterChain()
	}

	tapeNoiseSlider := widget.NewSlider(0.0, 1.0)
	tapeNoiseSlider.SetValue(opts.FilterTapeNoise)
	tapeNoiseSlider.OnChanged = func(f float64) {
		if opts.OnSetTapeNoise != nil {
			opts.OnSetTapeNoise(f)
		}
		buildFilterChain()
	}

	trackingErrorSlider := widget.NewSlider(0.0, 1.0)
	trackingErrorSlider.SetValue(opts.FilterTrackingError)
	trackingErrorSlider.OnChanged = func(f float64) {
		if opts.OnSetTrackingError != nil {
			opts.OnSetTrackingError(f)
		}
		buildFilterChain()
	}

	dropoutSlider := widget.NewSlider(0.0, 1.0)
	dropoutSlider.SetValue(opts.FilterDropout)
	dropoutSlider.OnChanged = func(f float64) {
		if opts.OnSetDropout != nil {
			opts.OnSetDropout(f)
		}
		buildFilterChain()
	}

	colorBleedingCheck := widget.NewCheck("Color Bleeding", func(b bool) {
		if opts.OnSetColorBleeding != nil {
			opts.OnSetColorBleeding(b)
		}
		buildFilterChain()
	})
	colorBleedingCheck.SetChecked(opts.FilterColorBleeding)

	interlacingSelect := widget.NewSelect([]string{"Off", "Telecine (2:3)", "Inverse Telecine", "Force Progressive"}, func(s string) {
		if opts.OnSetInterlacing != nil {
			opts.OnSetInterlacing(s)
		}
		buildFilterChain()
	})
	interlacingSelect.SetSelected(opts.FilterInterlacing)

	stylisticSection := buildFilterBox("Stylistic Effects", container.NewVBox(
		widget.NewLabel(t.FiltersSectionEra),
		container.NewGridWithColumns(2,
			widget.NewLabel(t.FiltersEraMode),
			stylisticModeSelect,
			widget.NewLabel(t.FiltersInterlacing),
			interlacingSelect,
		),
		scanlinesCheck,
		widget.NewSeparator(),
		container.NewGridWithColumns(2,
			widget.NewLabel(t.FiltersChromaNoise),
			chromaNoiseSlider,
			widget.NewLabel(t.FiltersTapeNoise),
			tapeNoiseSlider,
			widget.NewLabel(t.FiltersTrackingError),
			trackingErrorSlider,
			widget.NewLabel(t.FiltersTapeDropout),
			dropoutSlider,
		),
		colorBleedingCheck,
	))

	interpEnabledCheck := widget.NewCheck("Enable Frame Interpolation", func(checked bool) {
		if opts.OnSetInterpEnabled != nil {
			opts.OnSetInterpEnabled(checked)
		}
		buildFilterChain()
	})
	interpEnabledCheck.SetChecked(opts.FilterInterpEnabled)

	interpPresetSelect := widget.NewSelect([]string{"Ultra Fast", "Fast", "Balanced", "High Quality", "Maximum Quality"}, func(val string) {
		if opts.OnSetInterpPreset != nil {
			opts.OnSetInterpPreset(val)
		}
		buildFilterChain()
	})
	interpPresetSelect.SetSelected(opts.FilterInterpPreset)

	interpFPSSelect := widget.NewSelect([]string{"24", "30", "50", "59.94", "60"}, func(val string) {
		if opts.OnSetInterpFPS != nil {
			opts.OnSetInterpFPS(val)
		}
		buildFilterChain()
	})
	interpFPSSelect.SetSelected(opts.FilterInterpFPS)

	interpHint := widget.NewLabel(t.FiltersInterpHint)
	interpHint.TextStyle = fyne.TextStyle{Italic: true}
	interpHint.Wrapping = fyne.TextWrapWord

	interpSection := buildFilterBox("Frame Interpolation (Minterpolate)", container.NewVBox(
		widget.NewLabel(t.FiltersSectionMotion),
		interpEnabledCheck,
		container.NewGridWithColumns(2,
			widget.NewLabel(t.FiltersPreset),
			interpPresetSelect,
			widget.NewLabel(t.FiltersTargetFPS),
			interpFPSSelect,
		),
		interpHint,
	))
	buildFilterChain()

	applyBtn := widget.NewButton("Apply Filters", func() {
		if opts.FiltersFile == nil {
			dialog.ShowInformation("No Video", "Please load a video first.", opts.Window)
			return
		}
		buildFilterChain()
		if opts.OnApplyFilters != nil {
			opts.OnApplyFilters()
		}
		dialog.ShowInformation("Filters", "Filters are now configured and will be applied when sent to Upscale.", opts.Window)
	})
	applyBtn.Importance = widget.HighImportance

	leftPanel := container.NewVBox(
		instructions,
		widget.NewSeparator(),
		fileLabel,
		loadBtn,
		upscaleNavBtn,
	)

	settingsPanel := container.NewVBox(
		colorSection,
		enhanceSection,
		transformSection,
		interpSection,
		creativeSection,
		stylisticSection,
	)

	settingsScroll := ui.NewFastVScroll(settingsPanel)

	mainContent := container.New(&fixedHSplitLayout{ratio: 0.6},
		container.NewBorder(leftPanel, nil, nil, nil, videoContainer),
		settingsScroll,
	)

	content := container.NewPadded(mainContent)

	statsBar := opts.OnGetStatsBar()

	bottomBar := container.NewVBox(
		container.NewHBox(layout.NewSpacer(), applyBtn, widget.NewButton("Filter Now", func() {
			if opts.FiltersFile == nil {
				dialog.ShowInformation("No Video", "Please load a video first.", opts.Window)
				return
			}
			buildFilterChain()
			if opts.OnFilterNow != nil {
				opts.OnFilterNow()
			}
		}), widget.NewButton("Add to Queue", func() {
			if opts.FiltersFile == nil {
				dialog.ShowInformation("No Video", "Please load a video first.", opts.Window)
				return
			}
			buildFilterChain()
			if opts.OnAddToQueue != nil {
				opts.OnAddToQueue()
			}
		})),
		statsBar,
	)
	return container.NewBorder(topBar, bottomBar, nil, nil, content)
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
