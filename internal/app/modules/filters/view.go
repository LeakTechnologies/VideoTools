package filters

import (
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/ui"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/utils"
	"image/color"
)

var gridColor = utils.MustHex("#2A3A52")
var navyBlue = utils.MustHex("#191F35")
var mediumBlue = utils.MustHex("#13182B")

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

	HardwareAccel func() string
	SetHardwareAccel func(s string)

	OnShowMainMenu       func()
	OnShowQueue          func()
	OnShowUpscaleView    func()
	OnShowFiltersView    func()
	OnClearCompletedJobs func()
	OnGetStatsBar        func() *ui.ConversionStatsBar

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

	// Dual before/after player panes. When both return non-nil the view shows
	// a split original|filtered layout instead of the single-pane fallback.
	BuildOriginalPlayerPane func() fyne.CanvasObject
	BuildPreviewPlayerPane  func() fyne.CanvasObject
	// OnFilterChanged is called after every filter parameter change so the host
	// can rebuild and apply the filter pipeline to the preview player.
	OnFilterChanged func()
	OnGetModuleFooter func(color.Color, fyne.CanvasObject, *ui.ConversionStatsBar) fyne.CanvasObject
	BuildMetadataPane func(onToggle func(bool)) fyne.CanvasObject
}

func BuildView(opts Options) fyne.CanvasObject {
	t := i18n.T()
	filtersColor := opts.ModuleColor
	if filtersColor == nil {
		filtersColor = utils.MustHex("#005F5F")
	}

	backBtn := ui.MakePillButton("< "+strings.ToUpper(t.ModuleFilters), ui.BorderDim, func() {
		if opts.OnShowMainMenu != nil {
			opts.OnShowMainMenu()
		}
	})

	loadBtn := ui.MakePillButton(t.ActionLoadVideo, ui.BorderDim, nil)

	queueBtn := ui.MakePillButton(t.ActionViewQueue, ui.BorderDim, func() {
		if opts.OnShowQueue != nil {
			opts.OnShowQueue()
		}
	})

	clearCompletedBtn := ui.MakePillButton("⌫", ui.BorderDim, func() {
		if opts.OnClearCompletedJobs != nil {
			opts.OnClearCompletedJobs()
		}
	})

	topBar := ui.TintedBar(filtersColor, container.NewHBox(backBtn, loadBtn, layout.NewSpacer(), clearCompletedBtn, queueBtn))

	buildFilterChain := func() {
		if opts.OnBuildFilterChain != nil {
			opts.OnBuildFilterChain()
		}
		if opts.OnFilterChanged != nil {
			opts.OnFilterChanged()
		}
	}

	if opts.FiltersFile != nil && opts.OnHasNativeMediaPlayer != nil && opts.OnHasNativeMediaPlayer() && opts.FiltersFilePath != "" {
		go opts.OnLoadVideoNative(opts.FiltersFilePath)
	}

	// Build player area — dual before/after panes when available, single pane otherwise.
	var videoArea fyne.CanvasObject
	var origPane, prevPane fyne.CanvasObject
	if opts.BuildOriginalPlayerPane != nil {
		origPane = opts.BuildOriginalPlayerPane()
	}
	if opts.BuildPreviewPlayerPane != nil {
		prevPane = opts.BuildPreviewPlayerPane()
	}
	if origPane != nil && prevPane != nil {
		labelStyle := fyne.TextStyle{Bold: true}
		origLabel := widget.NewLabelWithStyle("ORIGINAL", fyne.TextAlignCenter, labelStyle)
		filtLabel := widget.NewLabelWithStyle("FILTERED", fyne.TextAlignCenter, labelStyle)
		origCol := container.NewBorder(origLabel, nil, nil, nil, origPane)
		filtCol := container.NewBorder(filtLabel, nil, nil, nil, prevPane)
		videoArea = container.NewGridWithColumns(2, origCol, filtCol)
	} else {
		var videoContainer fyne.CanvasObject
		if opts.FiltersFile != nil {
			videoContainer = opts.OnBuildVideoPane(nil, fyne.NewSize(480, 270), opts.FiltersFile, nil)
	} else {
		videoContainer = opts.OnBuildVideoPane(nil, fyne.NewSize(480, 270), nil, nil)
	}
	videoArea = videoContainer
}

	loadBtn.OnTapped = func() {
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
	}

	upscaleNavBtn := ui.MakePillButton("Send to Upscale →", filtersColor, func() {
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
		headerBg := canvas.NewRectangle(filtersColor)
		headerBg.CornerRadius = 10
		headerBg.SetMinSize(fyne.NewSize(0, 34))
		headerTitle := canvas.NewText(strings.ToUpper(title), color.White)
		headerTitle.TextStyle = fyne.TextStyle{Bold: true}
		headerTitle.TextSize = 12
		header := container.NewMax(
			headerBg,
			container.NewPadded(container.NewHBox(headerTitle, layout.NewSpacer())),
		)
		body := container.NewBorder(header, nil, nil, nil, container.NewPadded(content))
		layers := ui.NoisyBackgroundObjects(bg)
		layers = append(layers, body)
		return container.NewMax(layers...)
	}

	brightnessSlider := ui.MakeSlider(-1.0, 1.0)
	brightnessSlider.SetValue(opts.FilterBrightness)
	brightnessSlider.OnChanged = func(f float64) {
		if opts.OnSetBrightness != nil {
			opts.OnSetBrightness(f)
		}
		buildFilterChain()
	}

	contrastSlider := ui.MakeSlider(0.0, 3.0)
	contrastSlider.SetValue(opts.FilterContrast)
	contrastSlider.OnChanged = func(f float64) {
		if opts.OnSetContrast != nil {
			opts.OnSetContrast(f)
		}
		buildFilterChain()
	}

	saturationSlider := ui.MakeSlider(0.0, 3.0)
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

	sharpnessSlider := ui.MakeSlider(0.0, 5.0)
	sharpnessSlider.SetValue(opts.FilterSharpness)
	sharpnessSlider.OnChanged = func(f float64) {
		if opts.OnSetSharpness != nil {
			opts.OnSetSharpness(f)
		}
		buildFilterChain()
	}

	denoiseSlider := ui.MakeSlider(0.0, 10.0)
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

	chromaNoiseSlider := ui.MakeSlider(0.0, 1.0)
	chromaNoiseSlider.SetValue(opts.FilterChromaNoise)
	chromaNoiseSlider.OnChanged = func(f float64) {
		if opts.OnSetChromaNoise != nil {
			opts.OnSetChromaNoise(f)
		}
		buildFilterChain()
	}

	tapeNoiseSlider := ui.MakeSlider(0.0, 1.0)
	tapeNoiseSlider.SetValue(opts.FilterTapeNoise)
	tapeNoiseSlider.OnChanged = func(f float64) {
		if opts.OnSetTapeNoise != nil {
			opts.OnSetTapeNoise(f)
		}
		buildFilterChain()
	}

	trackingErrorSlider := ui.MakeSlider(0.0, 1.0)
	trackingErrorSlider.SetValue(opts.FilterTrackingError)
	trackingErrorSlider.OnChanged = func(f float64) {
		if opts.OnSetTrackingError != nil {
			opts.OnSetTrackingError(f)
		}
		buildFilterChain()
	}

	dropoutSlider := ui.MakeSlider(0.0, 1.0)
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

	// Hardware acceleration
	hwAccelOptions := []string{"auto", "none"}
	if runtime.GOOS == "windows" {
		hwAccelOptions = append(hwAccelOptions, "nvenc", "qsv", "amf")
	} else {
		hwAccelOptions = append(hwAccelOptions, "nvenc", "qsv", "vaapi", "amf")
	}
	hwAccelSelect := widget.NewSelect(hwAccelOptions, func(value string) {
		if opts.SetHardwareAccel != nil {
			opts.SetHardwareAccel(value)
		}
	})
	if opts.HardwareAccel != nil {
		hwAccel := opts.HardwareAccel()
		if hwAccel == "" {
			hwAccel = "auto"
		}
		hwAccelSelect.SetSelected(hwAccel)
	} else {
		hwAccelSelect.SetSelected("auto")
	}

	hwAccelSection := buildFilterBox("Hardware Acceleration", container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel("Hardware Accel:"),
			hwAccelSelect,
		),
	))

	applyBtn := ui.MakePillButton("Apply Filters", filtersColor, func() {
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

	filterNowBtn := ui.MakePillButton("Filter Now", filtersColor, func() {
		if opts.FiltersFile == nil {
			dialog.ShowInformation("No Video", "Please load a video first.", opts.Window)
			return
		}
		buildFilterChain()
		if opts.OnFilterNow != nil {
			opts.OnFilterNow()
		}
	})

	addQueueBtn := ui.MakePillButton("Add to Queue", filtersColor, func() {
		if opts.FiltersFile == nil {
			dialog.ShowInformation("No Video", "Please load a video first.", opts.Window)
			return
		}
		buildFilterChain()
		if opts.OnAddToQueue != nil {
			opts.OnAddToQueue()
		}
	})

	var leftSplit *container.Split

	var metaPane fyne.CanvasObject
	if opts.BuildMetadataPane != nil {
		metaPane = opts.BuildMetadataPane(func(open bool) {
			if open {
				leftSplit.SetOffset(0.65)
			} else {
				leftSplit.SetOffset(0.97)
			}
		})
	} else {
		outer := canvas.NewRectangle(navyBlue)
		outer.CornerRadius = 8
		outer.StrokeColor = gridColor
		outer.StrokeWidth = 1
		hdr, _ := ui.BuildCollapsibleHeader("Source Metadata", filtersColor, func(open bool) {
			if open {
				leftSplit.SetOffset(0.65)
			} else {
				leftSplit.SetOffset(0.97)
			}
		})
		body := container.NewBorder(hdr, nil, nil, nil,
			container.NewPadded(widget.NewLabel("Load a video to inspect its technical details.")))
		layers := ui.NoisyBackgroundObjects(outer)
		layers = append(layers, body)
		metaPane = container.NewMax(layers...)
	}

	metaScroll := ui.NewFastVScroll(metaPane)
	leftSplit = container.NewVSplit(container.NewPadded(videoArea), metaScroll)
	leftSplit.SetOffset(0.65)

	settingsPanel := container.NewVBox(
		colorSection,
		enhanceSection,
		transformSection,
		interpSection,
		hwAccelSection,
		creativeSection,
		stylisticSection,
	)

	settingsScroll := ui.NewFastVScroll(settingsPanel)

	leftMin := canvas.NewRectangle(color.Transparent)
	leftMin.SetMinSize(fyne.NewSize(680, 0))
	leftWrapped := container.NewMax(leftMin, leftSplit)

	rightMin := canvas.NewRectangle(color.Transparent)
	rightMin.SetMinSize(fyne.NewSize(400, 0))
	rightWrapped := container.NewMax(rightMin, settingsScroll)

	split := container.NewHSplit(leftWrapped, rightWrapped)
	split.Offset = 0.65

	content := container.NewMax(
		append(ui.NoisyBackgroundObjects(canvas.NewRectangle(mediumBlue)), container.NewPadded(split))...,
	)

	actionBar := container.NewHBox(layout.NewSpacer(), upscaleNavBtn, applyBtn, filterNowBtn, addQueueBtn)
	bottomBar := opts.OnGetModuleFooter(filtersColor, actionBar, opts.OnGetStatsBar())

	return container.NewBorder(topBar, bottomBar, nil, nil, content)
}
