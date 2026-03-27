package upscale

import (
	"fmt"
	"image/color"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

var gridColor = utils.MustHex("#2A3A52")
var navyBlue = utils.MustHex("#191F35")
var mediumBlue = utils.MustHex("#13182B")

func BuildView(opts Options) fyne.CanvasObject {
	upscaleColor := opts.ModuleColor
	if upscaleColor == nil {
		upscaleColor = utils.MustHex(ModuleColor)
	}
	t := i18n.T()

	if opts.UpscaleMethod() == "" {
		opts.SetUpscaleMethod("lanczos")
	}
	if opts.UpscaleTargetRes() == "" {
		opts.SetUpscaleTargetRes("Match Source")
	}
	if opts.UpscaleAIModel() == "" {
		opts.SetUpscaleAIModel("realesrgan-x4plus")
	}
	if opts.UpscaleFrameRate() == "" {
		opts.SetUpscaleFrameRate("Source")
	}
	if opts.UpscaleQualityPreset() == "" {
		opts.SetUpscaleQualityPreset("Near-lossless (CRF 16)")
	}
	if opts.UpscaleEncoderPreset() == "" {
		opts.SetUpscaleEncoderPreset("slow")
	}
	if opts.UpscaleVideoCodec() == "" {
		opts.SetUpscaleVideoCodec("H.264")
	}
	if opts.UpscaleBitrateMode() == "" {
		opts.SetUpscaleBitrateMode("CRF")
	}
	if opts.UpscaleBitratePreset() == "" {
		opts.SetUpscaleBitratePreset("2.5 Mbps - Medium")
	}
	if opts.UpscaleManualBitrate() == "" {
		opts.SetUpscaleManualBitrate("2500k")
	}
	if opts.UpscaleAIPreset() == "" {
		opts.SetUpscaleAIPreset("Balanced")
		opts.SetUpscaleAIScale(4.0)
		opts.SetUpscaleAIScaleUseTarget(true)
		opts.SetUpscaleAIOutputAdjust(1.0)
		opts.SetUpscaleAIDenoise(0.5)
		opts.SetUpscaleAITile(512)
		opts.SetUpscaleAIOutputFormat("png")
		opts.SetUpscaleAIGPUAuto(true)
		opts.SetUpscaleAIThreadsLoad(1)
		opts.SetUpscaleAIThreadsProc(2)
		opts.SetUpscaleAIThreadsSave(2)
	}
	if opts.UpscaleBlurSigma() <= 0 {
		opts.SetUpscaleBlurSigma(1.5)
	}

	if opts.UpscaleAIBackend() == "" {
		opts.SetUpscaleAIBackend(DetectAIUpscaleBackend())
		opts.SetUpscaleAIAvailable(opts.UpscaleAIBackend() == "ncnn")
	}
	if opts.UpscaleRIFEBackend() == "" {
		opts.SetUpscaleRIFEBackend(DetectRIFEBackend())
		opts.SetUpscaleRIFEAvailable(opts.UpscaleRIFEBackend() == "ncnn")
	}
	if opts.UpscaleRIFEMultiplier() == 0 {
		opts.SetUpscaleRIFEMultiplier(2)
	}
	if opts.UpscaleRIFEModel() == "" {
		opts.SetUpscaleRIFEModel(RIFEModelOptions()[0])
	}
	if len(opts.OnGetFilterActiveChain()) > 0 {
		opts.SetUpscaleFilterChain(append([]string{}, opts.OnGetFilterActiveChain()...))
	}

	loadBtn := widget.NewButton(t.ActionLoadVideo, nil)
	loadBtn.Importance = widget.LowImportance

	backBtn := widget.NewButton("< "+strings.ToUpper(t.ModuleUpscale), func() {
		opts.OnShowMainMenu()
	})
	backBtn.Importance = widget.LowImportance

	queueBtn := widget.NewButton(t.ActionViewQueue, func() {
		opts.OnShowQueue()
	})
	opts.QueueBtn = queueBtn
	opts.OnUpdateQueueButtonLabel()

	topBar := ui.TintedBar(upscaleColor, container.NewHBox(backBtn, loadBtn, layout.NewSpacer(), queueBtn))

	fileLabel := widget.NewLabel(t.LabelNoFile)
	fileLabel.TextStyle = fyne.TextStyle{Bold: true}

	src := toVideoSource(opts.UpscaleFile)
	var videoContainer fyne.CanvasObject
	var sourceResLabel *widget.Label
	if src != nil {
		fileLabel.SetText(fmt.Sprintf(t.LabelFileFmt, filepath.Base(src.Path)))
		sourceResLabel = widget.NewLabel(fmt.Sprintf(t.UpscaleSourceFmt, src.Width, src.Height))
		sourceResLabel.TextStyle = fyne.TextStyle{Italic: true}
		videoContainer = opts.OnBuildVideoPane(nil, fyne.NewSize(480, 270), src, nil)
		if opts.OnHasNativeMediaPlayer() {
			go opts.OnLoadVideoNative(src.Path)
		}
	} else {
		sourceResLabel = widget.NewLabel(t.UpscaleSourceNA)
		sourceResLabel.TextStyle = fyne.TextStyle{Italic: true}
		videoContainer = opts.OnBuildVideoPane(nil, fyne.NewSize(480, 270), nil, nil)
	}

	loadBtn.OnTapped = func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()

			path := reader.URI().Path()
			go func() {
				probeSrc, err := opts.OnProbeVideo(path)
				if err != nil {
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						dialog.ShowError(err, opts.Window)
					}, false)
					return
				}

				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					opts.SetUpscaleFile(probeSrc)
					opts.OnRefreshView()
				}, false)
			}()
		}, opts.Window)
	}

	filtersNavBtn := widget.NewButton(t.UpscaleAdjustFilters, func() {
		if src != nil {
			opts.SetFiltersFile(src)
		}
		opts.OnShowFiltersView()
	})

	buildUpscaleBox := func(title string, content fyne.CanvasObject) fyne.CanvasObject {
		bg := canvas.NewRectangle(navyBlue)
		bg.CornerRadius = 10
		bg.StrokeColor = gridColor
		bg.StrokeWidth = 1
		header := container.NewVBox(
			widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewSeparator(),
		)
		body := container.NewBorder(header, nil, nil, nil, content)
		layers := ui.NoisyBackgroundObjects(bg)
		layers = append(layers, container.NewPadded(body))
		return container.NewMax(layers...)
	}

	methodLabel := widget.NewLabel(fmt.Sprintf(t.UpscaleMethodFmt, opts.UpscaleMethod()))
	methodSelect := widget.NewSelect([]string{
		"lanczos",
		"bicubic",
		"spline",
		"bilinear",
	}, func(s string) {
		opts.SetUpscaleMethod(s)
		methodLabel.SetText(fmt.Sprintf(t.UpscaleMethodFmt, s))
	})
	methodSelect.SetSelected(opts.UpscaleMethod())

	methodInfo := widget.NewLabel("Lanczos: Sharp, best quality\nBicubic: Smooth\nSpline: Balanced\nBilinear: Fast")
	methodInfo.TextStyle = fyne.TextStyle{Italic: true}
	methodInfo.Wrapping = fyne.TextWrapWord

	resLabel := widget.NewLabel(fmt.Sprintf(t.UpscaleTargetFmt, opts.UpscaleTargetRes()))
	resSelect := widget.NewSelect([]string{
		"Match Source",
		"2X (relative)",
		"4X (relative)",
		"720p (1280x720)",
		"1080p (1920x1080)",
		"1440p (2560x1440)",
		"4K (3840x2160)",
		"8K (7680x4320)",
		"Custom",
	}, func(s string) {
		opts.SetUpscaleTargetRes(s)
		resLabel.SetText(fmt.Sprintf(t.UpscaleTargetFmt, s))
	})
	resSelect.SetSelected(opts.UpscaleTargetRes())

	resolutionSection := buildUpscaleBox(t.UpscaleTargetResBox, container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel(t.UpscaleResLabel),
			resSelect,
		),
		resLabel,
		sourceResLabel,
	))

	encoderPresetSelect := widget.NewSelect([]string{
		"ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow", "slower", "veryslow",
	}, func(s string) {
		opts.SetUpscaleEncoderPreset(s)
	})
	encoderPresetSelect.SetSelected(opts.UpscaleEncoderPreset())

	videoCodecOptions := []string{"H.264", "H.265", "VP9", "AV1", "Copy"}
	videoCodecColorMap := ui.BuildVideoCodecColorMap(videoCodecOptions)
	videoCodecSelect := ui.NewColoredSelect(videoCodecOptions, videoCodecColorMap, func(value string) {
		opts.SetUpscaleVideoCodec(value)
	}, opts.Window)
	videoCodecSelect.SetSelected(opts.UpscaleVideoCodec())

	containerOptions := []string{"mp4", "mkv", "mov", "webm"}
	containerColorMap := ui.BuildFormatColorMap(containerOptions)
	containerSelect := ui.NewColoredSelect(containerOptions, containerColorMap, func(s string) {
		opts.SetUpscaleOutputContainer(s)
	}, opts.Window)
	if opts.UpscaleOutputContainer() == "" {
		opts.SetUpscaleOutputContainer("mp4")
	}
	containerSelect.SetSelected(opts.UpscaleOutputContainer())

	hwAccelSelect := widget.NewSelect([]string{"auto", "none", "nvenc", "vaapi", "qsv", "amf"}, func(s string) {
		opts.SetUpscaleHardwareAccel(s)
	})
	if opts.UpscaleHardwareAccel() == "" {
		opts.SetUpscaleHardwareAccel("auto")
	}
	hwAccelSelect.SetSelected(opts.UpscaleHardwareAccel())

	if opts.UpscaleManualCRF() == 0 {
		opts.SetUpscaleManualCRF(16)
	}
	crfValueLabel := widget.NewLabel(fmt.Sprintf("%d", opts.UpscaleManualCRF()))
	crfSlider := widget.NewSlider(0, 51)
	crfSlider.Step = 1
	crfSlider.Value = float64(opts.UpscaleManualCRF())
	crfSlider.OnChanged = func(v float64) {
		opts.SetUpscaleManualCRF(int(v))
		crfValueLabel.SetText(fmt.Sprintf("%d", int(v)))
	}
	crfHint := widget.NewLabel(t.UpscaleCRFHint)
	crfHint.TextStyle = fyne.TextStyle{Italic: true}
	crfHint.Wrapping = fyne.TextWrapWord
	crfSection := container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel(t.UpscaleManualCRFLabel),
			container.NewBorder(nil, nil, nil, crfValueLabel, crfSlider),
		),
		crfHint,
	)

	bitrateEntry := widget.NewEntry()
	bitrateEntry.SetPlaceHolder("e.g. 8000k, 20M")
	if opts.UpscaleManualBitrate() != "" {
		bitrateEntry.SetText(opts.UpscaleManualBitrate())
	}
	bitrateEntry.OnChanged = func(s string) {
		opts.SetUpscaleManualBitrate(s)
	}
	bitrateHint := widget.NewLabel(t.UpscaleBitrateHint)
	bitrateSection := container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel(t.UpscaleBitrateValueLabel),
			bitrateEntry,
		),
		bitrateHint,
	)

	updateBitrateModeUI := func(mode string) {
		if mode == "CRF" {
			crfSection.Show()
			bitrateSection.Hide()
		} else {
			crfSection.Hide()
			bitrateSection.Show()
		}
	}
	updateBitrateModeUI(opts.UpscaleBitrateMode())

	bitrateModeSelect := widget.NewSelect([]string{
		"CRF (Constant Rate Factor)",
		"CBR (Constant Bitrate)",
		"VBR (Variable Bitrate)",
	}, func(s string) {
		switch {
		case strings.HasPrefix(s, "CRF"):
			opts.SetUpscaleBitrateMode("CRF")
		case strings.HasPrefix(s, "CBR"):
			opts.SetUpscaleBitrateMode("CBR")
		case strings.HasPrefix(s, "VBR"):
			opts.SetUpscaleBitrateMode("VBR")
		default:
			opts.SetUpscaleBitrateMode(s)
		}
		updateBitrateModeUI(opts.UpscaleBitrateMode())
	})
	switch opts.UpscaleBitrateMode() {
	case "CBR":
		bitrateModeSelect.SetSelected("CBR (Constant Bitrate)")
	case "VBR":
		bitrateModeSelect.SetSelected("VBR (Variable Bitrate)")
	default:
		bitrateModeSelect.SetSelected("CRF (Constant Rate Factor)")
	}

	pixelFormatOptions := []string{"yuv420p", "yuv444p", "yuv420p10le"}
	pixelFormatColorMap := ui.BuildPixelFormatColorMap(pixelFormatOptions)
	pixelFormatSelect := ui.NewColoredSelect(pixelFormatOptions, pixelFormatColorMap, func(s string) {
		opts.SetUpscalePixelFormat(s)
	}, opts.Window)
	if opts.UpscalePixelFormat() == "" {
		opts.SetUpscalePixelFormat("yuv420p")
	}
	pixelFormatSelect.SetSelected(opts.UpscalePixelFormat())

	encodingSection := buildUpscaleBox(t.UpscaleEncodingBox, container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel(t.UpscaleVideoCodecLabel),
			videoCodecSelect,
		),
		container.NewGridWithColumns(2,
			widget.NewLabel(t.UpscaleContainerLabel),
			containerSelect,
		),
		container.NewGridWithColumns(2,
			widget.NewLabel(t.UpscaleHardwareAccelLabel),
			hwAccelSelect,
		),
		container.NewGridWithColumns(2,
			widget.NewLabel(t.UpscaleEncoderLabel),
			encoderPresetSelect,
		),
		container.NewGridWithColumns(2,
			widget.NewLabel(t.UpscaleBitrateLabel),
			bitrateModeSelect,
		),
		crfSection,
		bitrateSection,
		container.NewGridWithColumns(2,
			widget.NewLabel(t.UpscalePixelFormatLabel),
			pixelFormatSelect,
		),
	))

	srcColourSelect := widget.NewSelect([]string{"auto", "bt601", "bt709", "bt2020"}, func(s string) {
		opts.SetUpscaleSrcColorSpace(s)
	})
	if opts.UpscaleSrcColorSpace() == "" {
		opts.SetUpscaleSrcColorSpace("auto")
	}
	srcColourSelect.SetSelected(opts.UpscaleSrcColorSpace())

	colorDepthSelect := widget.NewSelect([]string{"8bit", "16bit"}, func(s string) {
		opts.SetUpscaleColorDepth(s)
	})
	if opts.UpscaleColorDepth() == "" {
		opts.SetUpscaleColorDepth("8bit")
	}
	colorDepthSelect.SetSelected(opts.UpscaleColorDepth())

	skinToneSelect := widget.NewSelect([]string{"off", "subtle", "strong"}, func(s string) {
		opts.SetUpscaleSkinTone(s)
	})
	if opts.UpscaleSkinTone() == "" {
		opts.SetUpscaleSkinTone("off")
	}
	skinToneSelect.SetSelected(opts.UpscaleSkinTone())

	colourHint := widget.NewLabel(t.UpscaleColourHint)
	colourHint.TextStyle = fyne.TextStyle{Italic: true}
	colourHint.Wrapping = fyne.TextWrapWord

	colourAccuracySection := buildUpscaleBox(t.UpscaleColourBox, container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel(t.UpscaleSrcColourLabel),
			srcColourSelect,
		),
		container.NewGridWithColumns(2,
			widget.NewLabel(t.UpscaleDepthLabel),
			colorDepthSelect,
		),
		container.NewGridWithColumns(2,
			widget.NewLabel(t.UpscaleSkinToneLabel),
			skinToneSelect,
		),
		colourHint,
	))

	frameRateLabel := widget.NewLabel(fmt.Sprintf(t.UpscaleFrameRateFmt, opts.UpscaleFrameRate()))
	frameRateSelect := widget.NewSelect([]string{"Source", "23.976", "24", "25", "29.97", "30", "50", "59.94", "60"}, func(s string) {
		opts.SetUpscaleFrameRate(s)
		frameRateLabel.SetText(fmt.Sprintf(t.UpscaleFrameRateFmt, s))
	})
	frameRateSelect.SetSelected(opts.UpscaleFrameRate())

	motionInterpCheck := widget.NewCheck(t.UpscaleMotionInterp, func(checked bool) {
		opts.SetUpscaleMotionInterpolation(checked)
	})
	motionInterpCheck.SetChecked(opts.UpscaleMotionInterpolation())

	frameRateSection := buildUpscaleBox(t.UpscaleFrameRateBox, container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel(t.UpscaleTargetFPSLabel),
			frameRateSelect,
		),
		frameRateLabel,
		motionInterpCheck,
		widget.NewLabel(t.UpscaleMotionHint),
	))

	aiModelOptions := ModelOptions()
	aiModelLabel := ModelLabelFromID(opts.UpscaleAIModel())
	if aiModelLabel == "" && len(aiModelOptions) > 0 {
		aiModelLabel = aiModelOptions[0]
	}

	var aiContent fyne.CanvasObject
	if opts.UpscaleAIAvailable() {
		var aiTileSelect *widget.Select
		var aiTTACheck *widget.Check
		var aiDenoiseSlider *widget.Slider
		var denoiseHint *widget.Label

		applyAIPreset := func(preset string) {
			opts.SetUpscaleAIPreset(preset)
			switch preset {
			case "Ultra Fast":
				opts.SetUpscaleAITile(800)
				opts.SetUpscaleAITTA(false)
			case "Fast":
				opts.SetUpscaleAITile(800)
				opts.SetUpscaleAITTA(false)
			case "Balanced":
				opts.SetUpscaleAITile(512)
				opts.SetUpscaleAITTA(false)
			case "High Quality":
				opts.SetUpscaleAITile(256)
				opts.SetUpscaleAITTA(false)
			case "Maximum Quality":
				opts.SetUpscaleAITile(0)
				opts.SetUpscaleAITTA(true)
			}
			if aiTileSelect != nil {
				switch opts.UpscaleAITile() {
				case 256:
					aiTileSelect.SetSelected("256")
				case 512:
					aiTileSelect.SetSelected("512")
				case 800:
					aiTileSelect.SetSelected("800")
				default:
					aiTileSelect.SetSelected("Auto")
				}
			}
			if aiTTACheck != nil {
				aiTTACheck.SetChecked(opts.UpscaleAITTA())
			}
		}

		denoiseAvailStr := t.UpscaleDenoiseAvail
		denoiseUnavailStr := t.UpscaleDenoiseUnavail

		updateDenoiseAvailability := func(model string) {
			if aiDenoiseSlider == nil || denoiseHint == nil {
				return
			}
			if model == "realesr-general-x4v3" {
				aiDenoiseSlider.Enable()
				denoiseHint.SetText(denoiseAvailStr)
			} else {
				aiDenoiseSlider.Disable()
				denoiseHint.SetText(denoiseUnavailStr)
			}
		}

		aiEnabledCheck := widget.NewCheck(t.UpscaleAIEnabled, func(checked bool) {
			opts.SetUpscaleAIEnabled(checked)
			if checked {
				colourAccuracySection.Show()
			} else {
				colourAccuracySection.Hide()
			}
		})
		aiEnabledCheck.SetChecked(opts.UpscaleAIEnabled())
		if !opts.UpscaleAIEnabled() {
			colourAccuracySection.Hide()
		}

		aiModelSelect := widget.NewSelect(aiModelOptions, func(s string) {
			opts.SetUpscaleAIModel(ModelIDFromLabel(s))
			aiModelLabel = s
			updateDenoiseAvailability(opts.UpscaleAIModel())
		})
		if aiModelLabel != "" {
			aiModelSelect.SetSelected(aiModelLabel)
		}

		aiPresetSelect := widget.NewSelect([]string{"Ultra Fast", "Fast", "Balanced", "High Quality", "Maximum Quality"}, func(s string) {
			applyAIPreset(s)
		})
		aiPresetSelect.SetSelected(opts.UpscaleAIPreset())

		aiScaleSelect := widget.NewSelect([]string{"Match Target", "1x", "2x", "3x", "4x", "8x"}, func(s string) {
			if s == "Match Target" {
				opts.SetUpscaleAIScaleUseTarget(true)
				return
			}
			opts.SetUpscaleAIScaleUseTarget(false)
			switch s {
			case "1x":
				opts.SetUpscaleAIScale(1)
			case "2x":
				opts.SetUpscaleAIScale(2)
			case "3x":
				opts.SetUpscaleAIScale(3)
			case "4x":
				opts.SetUpscaleAIScale(4)
			case "8x":
				opts.SetUpscaleAIScale(8)
			}
		})
		if opts.UpscaleAIScaleUseTarget() {
			aiScaleSelect.SetSelected("Match Target")
		} else {
			aiScaleSelect.SetSelected(fmt.Sprintf("%.0fx", opts.UpscaleAIScale()))
		}

		aiAdjustLabel := widget.NewLabel(fmt.Sprintf(t.UpscaleAdjustFmt, opts.UpscaleAIOutputAdjust()))
		aiAdjustSlider := widget.NewSlider(0.5, 2.0)
		aiAdjustSlider.Value = opts.UpscaleAIOutputAdjust()
		aiAdjustSlider.Step = 0.05
		aiAdjustSlider.OnChanged = func(v float64) {
			opts.SetUpscaleAIOutputAdjust(v)
			aiAdjustLabel.SetText(fmt.Sprintf(t.UpscaleAdjustFmt, v))
		}

		aiDenoiseLabel := widget.NewLabel(fmt.Sprintf(t.UpscaleDenoiseFmt, opts.UpscaleAIDenoise()))
		aiDenoiseSlider = widget.NewSlider(0.0, 1.0)
		aiDenoiseSlider.Value = opts.UpscaleAIDenoise()
		aiDenoiseSlider.Step = 0.05
		aiDenoiseSlider.OnChanged = func(v float64) {
			opts.SetUpscaleAIDenoise(v)
			aiDenoiseLabel.SetText(fmt.Sprintf(t.UpscaleDenoiseFmt, v))
		}

		aiTileSelect = widget.NewSelect([]string{"Auto", "256", "512", "800"}, func(s string) {
			switch s {
			case "Auto":
				opts.SetUpscaleAITile(0)
			case "256":
				opts.SetUpscaleAITile(256)
			case "512":
				opts.SetUpscaleAITile(512)
			case "800":
				opts.SetUpscaleAITile(800)
			}
		})
		switch opts.UpscaleAITile() {
		case 256:
			aiTileSelect.SetSelected("256")
		case 512:
			aiTileSelect.SetSelected("512")
		case 800:
			aiTileSelect.SetSelected("800")
		default:
			aiTileSelect.SetSelected("Auto")
		}

		aiOutputFormatSelect := widget.NewSelect([]string{"PNG", "JPG", "WEBP"}, func(s string) {
			opts.SetUpscaleAIOutputFormat(strings.ToLower(s))
		})
		switch strings.ToLower(opts.UpscaleAIOutputFormat()) {
		case "jpg", "jpeg":
			aiOutputFormatSelect.SetSelected("JPG")
		case "webp":
			aiOutputFormatSelect.SetSelected("WEBP")
		default:
			aiOutputFormatSelect.SetSelected("PNG")
		}

		aiFaceCheck := widget.NewCheck(t.UpscaleFaceEnhance, func(checked bool) {
			opts.SetUpscaleAIFaceEnhance(checked)
		})
		aiFaceAvailable := CheckAIFaceEnhanceAvailable(opts.UpscaleAIBackend())
		if !aiFaceAvailable {
			aiFaceCheck.Disable()
		}
		aiFaceCheck.SetChecked(opts.UpscaleAIFaceEnhance() && aiFaceAvailable)

		aiTTACheck = widget.NewCheck(t.UpscaleTTACheck, func(checked bool) {
			opts.SetUpscaleAITTA(checked)
		})
		aiTTACheck.SetChecked(opts.UpscaleAITTA())

		aiGPUSelect := widget.NewSelect([]string{"Auto", "0", "1", "2"}, func(s string) {
			if s == "Auto" {
				opts.SetUpscaleAIGPUAuto(true)
				return
			}
			opts.SetUpscaleAIGPUAuto(false)
			if gpu, err := strconv.Atoi(s); err == nil {
				opts.SetUpscaleAIGPU(gpu)
			}
		})
		if opts.UpscaleAIGPUAuto() {
			aiGPUSelect.SetSelected("Auto")
		} else {
			aiGPUSelect.SetSelected(strconv.Itoa(opts.UpscaleAIGPU()))
		}

		threadOptions := []string{"1", "2", "3", "4"}
		aiThreadsLoad := widget.NewSelect(threadOptions, func(s string) {
			if v, err := strconv.Atoi(s); err == nil {
				opts.SetUpscaleAIThreadsLoad(v)
			}
		})
		aiThreadsLoad.SetSelected(strconv.Itoa(opts.UpscaleAIThreadsLoad()))

		aiThreadsProc := widget.NewSelect(threadOptions, func(s string) {
			if v, err := strconv.Atoi(s); err == nil {
				opts.SetUpscaleAIThreadsProc(v)
			}
		})
		aiThreadsProc.SetSelected(strconv.Itoa(opts.UpscaleAIThreadsProc()))

		aiThreadsSave := widget.NewSelect(threadOptions, func(s string) {
			if v, err := strconv.Atoi(s); err == nil {
				opts.SetUpscaleAIThreadsSave(v)
			}
		})
		aiThreadsSave.SetSelected(strconv.Itoa(opts.UpscaleAIThreadsSave()))

		denoiseHint = widget.NewLabel("")
		denoiseHint.TextStyle = fyne.TextStyle{Italic: true}
		updateDenoiseAvailability(opts.UpscaleAIModel())

		aiContent = container.NewVBox(
			widget.NewLabel(t.UpscaleAIDetected),
			aiEnabledCheck,
			container.NewGridWithColumns(2,
				widget.NewLabel(t.UpscaleAIModelLabel),
				aiModelSelect,
			),
			container.NewGridWithColumns(2,
				widget.NewLabel(t.UpscaleAIPresetLabel),
				aiPresetSelect,
			),
			container.NewGridWithColumns(2,
				widget.NewLabel(t.UpscaleAIScaleLabel),
				aiScaleSelect,
			),
			container.NewVBox(aiAdjustLabel, aiAdjustSlider),
			container.NewVBox(aiDenoiseLabel, aiDenoiseSlider, denoiseHint),
			container.NewGridWithColumns(2,
				widget.NewLabel(t.UpscaleAITileLabel),
				aiTileSelect,
			),
			container.NewGridWithColumns(2,
				widget.NewLabel(t.UpscaleAIOutputLabel),
				aiOutputFormatSelect,
			),
			aiFaceCheck,
			aiTTACheck,
			widget.NewSeparator(),
			widget.NewLabel(t.UpscaleAIAdvanced),
			container.NewGridWithColumns(2,
				widget.NewLabel(t.UpscaleGPULabel),
				aiGPUSelect,
			),
			container.NewGridWithColumns(2,
				widget.NewLabel(t.UpscaleThreadsLabel),
				container.NewGridWithColumns(3, aiThreadsLoad, aiThreadsProc, aiThreadsSave),
			),
			widget.NewLabel(t.UpscaleAINote),
		)
	} else {
		backendNote := t.UpscaleAINotDetected
		if opts.UpscaleAIBackend() == "python" {
			backendNote = t.UpscaleAIPython
		}
		aiContent = container.NewVBox(
			widget.NewLabel(backendNote),
			widget.NewLabel("https://github.com/xinntao/Real-ESRGAN"),
			widget.NewLabel(t.UpscaleAIFallback),
		)
	}

	aiSection := container.NewVBox(
		widget.NewLabelWithStyle(t.UpscaleAIBox, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		aiContent,
	)

	traditionalSection := buildUpscaleBox(t.UpscaleScalingBox, container.NewVBox(
		widget.NewLabel(t.UpscaleClassicDesc),
		container.NewGridWithColumns(2,
			widget.NewLabel(t.UpscaleScalingLabel),
			methodSelect,
		),
		methodLabel,
		widget.NewSeparator(),
		methodInfo,
		widget.NewSeparator(),
		aiSection,
	))

	filterApplyCheck := widget.NewCheck(t.UpscaleFilterIntLabel, func(checked bool) {
		opts.SetUpscaleApplyFilters(checked)
	})
	filterApplyCheck.SetChecked(opts.UpscaleApplyFilters())

	filterIntHint := widget.NewLabel(t.UpscaleFilterIntHint)
	filterIntHint.TextStyle = fyne.TextStyle{Italic: true}
	filterIntHint.Wrapping = fyne.TextWrapWord

	filterIntegrationSection := buildUpscaleBox(t.UpscaleFilterIntBox, container.NewVBox(
		filterApplyCheck,
		filterIntHint,
		filtersNavBtn,
	))

	var rifeSection fyne.CanvasObject
	if opts.UpscaleRIFEAvailable() {
		var updateEstFPS func()
		var estFPSLabel *widget.Label

		rifeMultiplierSelect := widget.NewSelect([]string{"2×", "4×", "8×"}, func(s string) {
			switch s {
			case "4×":
				opts.SetUpscaleRIFEMultiplier(4)
			case "8×":
				opts.SetUpscaleRIFEMultiplier(8)
			default:
				opts.SetUpscaleRIFEMultiplier(2)
			}
			if updateEstFPS != nil {
				updateEstFPS()
			}
		})
		switch opts.UpscaleRIFEMultiplier() {
		case 4:
			rifeMultiplierSelect.SetSelected("4×")
		case 8:
			rifeMultiplierSelect.SetSelected("8×")
		default:
			rifeMultiplierSelect.SetSelected("2×")
		}

		rifeModelSelect := widget.NewSelect(RIFEModelOptions(), func(s string) {
			opts.SetUpscaleRIFEModel(s)
		})
		rifeModelSelect.SetSelected(opts.UpscaleRIFEModel())

		rifeEnabledCheck := widget.NewCheck(t.RIFEEnabled, func(checked bool) {
			opts.SetUpscaleRIFEEnabled(checked)
		})
		rifeEnabledCheck.SetChecked(opts.UpscaleRIFEEnabled())

		estFPSLabel = widget.NewLabel("")
		estFPSLabel.TextStyle = fyne.TextStyle{Italic: true}
		updateEstFPS = func() {
			if src != nil && src.FrameRate > 0 {
				estFPSLabel.SetText(fmt.Sprintf(t.RIFEEstFPSFmt, src.FrameRate*float64(opts.UpscaleRIFEMultiplier())))
			} else {
				estFPSLabel.SetText("")
			}
		}
		updateEstFPS()

		rifeSection = buildUpscaleBox(t.RIFEBoxTitle, container.NewVBox(
			widget.NewLabel(t.RIFEDetected),
			rifeEnabledCheck,
			container.NewGridWithColumns(2,
				widget.NewLabel(t.RIFEMultiplierLabel),
				rifeMultiplierSelect,
			),
			container.NewGridWithColumns(2,
				widget.NewLabel(t.RIFEModelLabel),
				rifeModelSelect,
			),
			estFPSLabel,
			widget.NewLabel(t.RIFENote),
		))
	} else {
		rifeLink, _ := url.Parse("https://github.com/nihui/rife-ncnn-vulkan")
		rifeSection = buildUpscaleBox(t.RIFEBoxTitle, container.NewVBox(
			widget.NewLabel(t.RIFENotDetected),
			widget.NewHyperlink("nihui/rife-ncnn-vulkan", rifeLink),
			widget.NewLabel(t.RIFEInstallHint),
		))
	}

	createUpscaleJob := func() (*queue.Job, error) {
		if src == nil {
			return nil, fmt.Errorf("no video loaded")
		}

		targetWidth, targetHeight, preserveAspect, err := ParseResolutionPreset(opts.UpscaleTargetRes(), src.Width, src.Height)
		if err != nil {
			return nil, fmt.Errorf("invalid resolution: %w", err)
		}

		videoDir := filepath.Dir(src.Path)
		videoBaseName := strings.TrimSuffix(filepath.Base(src.Path), filepath.Ext(src.Path))
		slug := SanitizeForPath(opts.UpscaleTargetRes())
		if slug == "" {
			slug = "source"
		}
		containerExt := opts.UpscaleOutputContainer()
		if containerExt == "" {
			containerExt = "mkv"
		}
		outputPath := filepath.Join(videoDir, fmt.Sprintf("%s_upscaled_%s_%s.%s",
			videoBaseName, slug, opts.UpscaleMethod(), containerExt))

		description := fmt.Sprintf("Upscale to %s using %s", opts.UpscaleTargetRes(), opts.UpscaleMethod())
		if opts.UpscaleAIEnabled() && opts.UpscaleAIAvailable() {
			description += fmt.Sprintf(" + AI (%s)", opts.UpscaleAIModel())
		}
		if opts.UpscaleRIFEEnabled() && opts.UpscaleRIFEAvailable() {
			description += fmt.Sprintf(" + RIFE %dx", opts.UpscaleRIFEMultiplier())
		}

		desc := fmt.Sprintf("%s  %s", description, filepath.Base(outputPath))

		return &queue.Job{
			Type:        queue.JobTypeUpscale,
			Title:       "Upscale: " + filepath.Base(src.Path),
			Description: desc,
			OutputFile:  outputPath,
			Config: map[string]interface{}{
				"inputPath":              src.Path,
				"outputPath":             outputPath,
				"method":                 opts.UpscaleMethod(),
				"encoderPreset":          opts.UpscaleEncoderPreset(),
				"bitrateMode":            opts.UpscaleBitrateMode(),
				"bitratePreset":          opts.UpscaleBitratePreset(),
				"manualBitrate":          opts.UpscaleManualBitrate(),
				"targetWidth":            float64(targetWidth),
				"targetHeight":           float64(targetHeight),
				"targetPreset":           opts.UpscaleTargetRes(),
				"sourceWidth":            float64(src.Width),
				"sourceHeight":           float64(src.Height),
				"preserveAR":             preserveAspect,
				"useAI":                  opts.UpscaleAIEnabled() && opts.UpscaleAIAvailable(),
				"aiModel":                opts.UpscaleAIModel(),
				"qualityPreset":          opts.UpscaleQualityPreset(),
				"aiBackend":              opts.UpscaleAIBackend(),
				"aiPreset":               opts.UpscaleAIPreset(),
				"aiScale":                opts.UpscaleAIScale(),
				"aiScaleUseTarget":       opts.UpscaleAIScaleUseTarget(),
				"aiOutputAdjust":         opts.UpscaleAIOutputAdjust(),
				"aiFaceEnhance":          opts.UpscaleAIFaceEnhance(),
				"aiDenoise":              opts.UpscaleAIDenoise(),
				"aiTile":                 float64(opts.UpscaleAITile()),
				"aiGPU":                  float64(opts.UpscaleAIGPU()),
				"aiGPUAuto":              opts.UpscaleAIGPUAuto(),
				"aiThreadsLoad":          float64(opts.UpscaleAIThreadsLoad()),
				"aiThreadsProc":          float64(opts.UpscaleAIThreadsProc()),
				"aiThreadsSave":          float64(opts.UpscaleAIThreadsSave()),
				"aiTTA":                  opts.UpscaleAITTA(),
				"aiOutputFormat":         opts.UpscaleAIOutputFormat(),
				"applyFilters":           opts.UpscaleApplyFilters(),
				"filterChain":            opts.UpscaleFilterChain(),
				"blurEnabled":            opts.UpscaleBlurEnabled(),
				"blurSigma":              opts.UpscaleBlurSigma(),
				"videoCodec":             opts.UpscaleVideoCodec(),
				"duration":               src.Duration,
				"sourceFrameRate":        src.FrameRate,
				"frameRate":              opts.UpscaleFrameRate(),
				"useMotionInterpolation": opts.UpscaleMotionInterpolation(),
				"useRIFE":                opts.UpscaleRIFEEnabled() && opts.UpscaleRIFEAvailable(),
				"rifeModel":              opts.UpscaleRIFEModel(),
				"rifeMultiplier":         float64(opts.UpscaleRIFEMultiplier()),
				"hardwareAccel":          opts.UpscaleHardwareAccel(),
				"outputContainer":        opts.UpscaleOutputContainer(),
				"manualCRF":              float64(opts.UpscaleManualCRF()),
				"pixelFormat":            opts.UpscalePixelFormat(),
				"srcColorSpace":          opts.UpscaleSrcColorSpace(),
				"colorDepth":             opts.UpscaleColorDepth(),
				"skinTone":               opts.UpscaleSkinTone(),
			},
		}, nil
	}

	applyBtn := widget.NewButton(t.UpscaleNow, func() {
		job, err := createUpscaleJob()
		if err != nil {
			dialog.ShowError(err, opts.Window)
			return
		}

		opts.AddJob(job)
		if !opts.JobQueue().IsRunning() {
			opts.JobQueue().Start()
		}
		dialog.ShowInformation(t.UpscaleStartedTitle,
			fmt.Sprintf(t.UpscaleStartedFmt, opts.UpscaleTargetRes()),
			opts.Window)
	})
	applyBtn.Importance = widget.HighImportance
	if src == nil {
		applyBtn.Disable()
	}

	if c := opts.Window.Canvas(); c != nil {
		triggerUpscale := func() {
			if !applyBtn.Disabled() && applyBtn.OnTapped != nil {
				applyBtn.OnTapped()
			}
		}
		c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyReturn, Modifier: fyne.KeyModifierControl}, func(fyne.Shortcut) { triggerUpscale() })
		c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyEnter, Modifier: fyne.KeyModifierControl}, func(fyne.Shortcut) { triggerUpscale() })
	}

	addQueueBtn := widget.NewButton(t.ActionAddToQueue, func() {
		job, err := createUpscaleJob()
		if err != nil {
			dialog.ShowError(err, opts.Window)
			return
		}

		opts.AddJob(job)
		dialog.ShowInformation(t.UpscaleAddedTitle,
			fmt.Sprintf(t.UpscaleAddedFmt, opts.UpscaleTargetRes(), opts.UpscaleMethod()),
			opts.Window)
	})
	addQueueBtn.Importance = widget.MediumImportance
	if src == nil {
		addQueueBtn.Disable()
	}

	spacing := func() fyne.CanvasObject {
		spacer := canvas.NewRectangle(color.Transparent)
		spacer.SetMinSize(fyne.NewSize(0, 10))
		return spacer
	}

	metaPanel := buildMetadataPanel(opts, src, fyne.NewSize(0, 200))

	videoBoxContent := container.NewBorder(fileLabel, nil, nil, nil, videoContainer)
	videoBox := buildUpscaleBox(t.UpscaleVideoBox, videoBoxContent)
	metaScroll := ui.NewFastVScroll(metaPanel)
	leftSplit := container.NewVSplit(videoBox, metaScroll)
	leftSplit.SetOffset(0.55)
	leftPanel := leftSplit

	settingsPanel := container.NewVBox(
		traditionalSection,
		spacing(),
		resolutionSection,
		spacing(),
		encodingSection,
		spacing(),
		colourAccuracySection,
		spacing(),
		frameRateSection,
		spacing(),
		rifeSection,
		spacing(),
		filterIntegrationSection,
	)

	settingsScroll := ui.NewFastVScroll(settingsPanel)

	leftMin := canvas.NewRectangle(color.Transparent)
	leftMin.SetMinSize(fyne.NewSize(560, 0))
	leftWrapped := container.NewMax(leftMin, leftPanel)

	rightMin := canvas.NewRectangle(color.Transparent)
	rightMin.SetMinSize(fyne.NewSize(400, 0))
	rightWrapped := container.NewMax(rightMin, settingsScroll)

	split := container.NewHSplit(leftWrapped, rightWrapped)
	split.Offset = 0.58
	mainContent := split

	content := container.NewMax(
		append(ui.NoisyBackgroundObjects(canvas.NewRectangle(mediumBlue)), container.NewPadded(mainContent))...,
	)

	actionBar := container.NewHBox(layout.NewSpacer(), applyBtn, addQueueBtn)
	bottomBar := opts.OnGetModuleFooter(upscaleColor, actionBar, opts.OnGetStatsBar())

	return container.NewBorder(topBar, bottomBar, nil, nil, content)
}

func toVideoSource(v interface{}) *VideoSource {
	if v == nil {
		return nil
	}
	if vs, ok := v.(*VideoSource); ok {
		return vs
	}
	return nil
}

// VideoSource holds the probed metadata for a video file loaded into the upscale module.
type VideoSource struct {
	Path              string
	Format            string
	VideoCodec        string
	Width             int
	Height            int
	FrameRate         float64
	Bitrate           int
	PixelFormat       string
	ColorSpace        string
	ColorRange        string
	FieldOrder        string
	GOPSize           int
	AudioCodec        string
	AudioBitrate      int
	AudioRate         int
	Channels          int
	SampleAspectRatio string
	HasChapters       bool
	HasMetadata       bool
	Duration          float64
}

func buildMetadataPanel(opts Options, src *VideoSource, size fyne.Size) fyne.CanvasObject {
	outer := canvas.NewRectangle(navyBlue)
	outer.CornerRadius = 8
	outer.StrokeColor = gridColor
	outer.StrokeWidth = 1

	header := widget.NewLabelWithStyle("Source Metadata", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	if src == nil {
		body := container.NewVBox(
			header,
			widget.NewSeparator(),
			widget.NewLabel("Load a video to inspect its technical details."),
		)
		layers := ui.NoisyBackgroundObjects(outer)
		layers = append(layers, container.NewPadded(body))
		return container.NewMax(layers...)
	}

	valueBg := utils.MustHex("#2B334A")
	valueBorder := utils.MustHex("#3A4360")

	makeValuePill := func(text string) fyne.CanvasObject {
		bg := canvas.NewRectangle(valueBg)
		bg.CornerRadius = 6
		bg.StrokeColor = valueBorder
		bg.StrokeWidth = 1
		lbl := widget.NewLabel(text)
		lbl.TextStyle = fyne.TextStyle{Monospace: true}
		lbl.Wrapping = fyne.TextTruncate
		return container.NewMax(bg, container.NewPadded(lbl))
	}
	makeRow := func(key string, value fyne.CanvasObject) fyne.CanvasObject {
		keyLbl := widget.NewLabel(key + ":")
		keyLbl.TextStyle = fyne.TextStyle{Bold: true}
		return container.NewBorder(nil, nil, keyLbl, nil, value)
	}

	bitrate := "--"
	if src.Bitrate > 0 {
		bitrate = fmt.Sprintf("%d kbps", src.Bitrate/1000)
	}
	audioBitrate := "--"
	if src.AudioBitrate > 0 {
		audioBitrate = fmt.Sprintf("%d kbps", src.AudioBitrate/1000)
	}
	interlacing := "Progressive"
	if src.FieldOrder != "" && src.FieldOrder != "progressive" && src.FieldOrder != "unknown" {
		interlacing = "Interlaced (" + src.FieldOrder + ")"
	}
	colorRange := src.ColorRange
	if colorRange == "tv" {
		colorRange = "Limited (TV)"
	} else if colorRange == "pc" || colorRange == "jpeg" {
		colorRange = "Full (PC)"
	}
	chapters := "No"
	if src.HasChapters {
		chapters = "Yes"
	}

	// Duration display
	durSec := int(src.Duration)
	durStr := fmt.Sprintf("%d:%02d:%02d", durSec/3600, (durSec%3600)/60, durSec%60)
	if durSec == 0 {
		durStr = "--"
	}
	// Aspect ratio display
	aspectStr := "--"
	if src.Width > 0 && src.Height > 0 {
		gcdVal := func(a, b int) int {
			for b != 0 {
				a, b = b, a%b
			}
			return a
		}(src.Width, src.Height)
		aspectStr = fmt.Sprintf("%d:%d (%.2f:1)", src.Width/gcdVal, src.Height/gcdVal, float64(src.Width)/float64(src.Height))
	}

	col1 := container.NewVBox(
		makeRow("Title", makeValuePill(strings.TrimSuffix(filepath.Base(src.Path), filepath.Ext(src.Path)))),
		makeRow("Resolution", makeValuePill(fmt.Sprintf("%dx%d", src.Width, src.Height))),
		makeRow("Aspect Ratio", makeValuePill(aspectStr)),
		makeRow("Frame Rate", makeValuePill(fmt.Sprintf("%.2f fps", src.FrameRate))),
		makeRow("Duration", makeValuePill(durStr)),
		makeRow("Interlacing", makeValuePill(interlacing)),
		makeRow("Color Space", makeValuePill(utils.FirstNonEmpty(src.ColorSpace, "--"))),
		makeRow("Color Range", makeValuePill(utils.FirstNonEmpty(colorRange, "--"))),
	)
	col2 := container.NewVBox(
		makeRow("Video Codec", makeValuePill(utils.FirstNonEmpty(src.VideoCodec, "Unknown"))),
		makeRow("Video Bitrate", makeValuePill(bitrate)),
		makeRow("Pixel Format", makeValuePill(utils.FirstNonEmpty(src.PixelFormat, "--"))),
		makeRow("Audio Codec", makeValuePill(utils.FirstNonEmpty(src.AudioCodec, "Unknown"))),
		makeRow("Audio Bitrate", makeValuePill(audioBitrate)),
		makeRow("Audio Rate", makeValuePill(fmt.Sprintf("%d Hz", src.AudioRate))),
		makeRow("Chapters", makeValuePill(chapters)),
	)

	grid := container.NewGridWithColumns(2, col1, col2)
	body := container.NewVBox(header, widget.NewSeparator(), grid)
	layers := ui.NoisyBackgroundObjects(outer)
	layers = append(layers, container.NewPadded(body))
	return container.NewMax(layers...)
}
