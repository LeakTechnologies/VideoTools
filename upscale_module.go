package main

import (
	"fmt"
	"image/color"
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

	"git.leaktechnologies.dev/stu/VideoTools/internal/app/modules/upscale"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

// AI Helper Functions - delegates to internal package

func detectAIUpscaleBackend() string {
	return upscale.DetectAIUpscaleBackend()
}

func detectRIFEBackend() string {
	return upscale.DetectRIFEBackend()
}

func rifeModelOptions() []string {
	return upscale.RIFEModelOptions()
}

func checkAIFaceEnhanceAvailable(backend string) bool {
	return upscale.CheckAIFaceEnhanceAvailable(backend)
}

func aiUpscaleModelOptions() []string {
	return upscale.ModelOptions()
}

func aiUpscaleModelID(label string) string {
	return upscale.ModelIDFromLabel(label)
}

func aiUpscaleModelLabel(modelID string) string {
	return upscale.ModelLabelFromID(modelID)
}

func parseResolutionPreset(preset string, srcW, srcH int) (width, height int, preserveAspect bool, err error) {
	return upscale.ParseResolutionPreset(preset, srcW, srcH)
}

func buildUpscaleFilter(targetWidth, targetHeight int, method string, preserveAspect bool) string {
	return upscale.BuildUpscaleFilter(targetWidth, targetHeight, method, preserveAspect)
}

func sanitizeForPath(label string) string {
	return upscale.SanitizeForPath(label)
}

func (s *appState) showUpscaleView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "upscale"
	s.maximizeWindow()
	s.setContent(buildUpscaleView(s))
}

// buildUpscaleView creates the Upscale module UI
func buildUpscaleView(state *appState) fyne.CanvasObject {
	upscaleColor := moduleColor("upscale")
	t := i18n.T()

	// Back button
	backBtn := widget.NewButton("< "+t.ModuleUpscale, func() {
		state.showMainMenu()
	})
	backBtn.Importance = widget.LowImportance

	// Queue button
	queueBtn := widget.NewButton(t.ActionViewQueue, func() {
		state.showQueue()
	})
	state.queueBtn = queueBtn
	state.updateQueueButtonLabel()

	// Top bar with module color
	topBar := ui.TintedBar(upscaleColor, container.NewHBox(backBtn, layout.NewSpacer(), queueBtn))

	// Initialize state defaults
	if state.upscaleMethod == "" {
		state.upscaleMethod = "lanczos" // Best general-purpose traditional method
	}
	if state.upscaleTargetRes == "" {
		state.upscaleTargetRes = "Match Source"
	}
	if state.upscaleAIModel == "" {
		state.upscaleAIModel = "realesrgan-x4plus" // General purpose AI model
	}
	if state.upscaleFrameRate == "" {
		state.upscaleFrameRate = "Source"
	}
	if state.upscaleQualityPreset == "" {
		state.upscaleQualityPreset = "Near-lossless (CRF 16)"
	}
	if state.upscaleEncoderPreset == "" {
		state.upscaleEncoderPreset = "slow"
	}
	if state.upscaleVideoCodec == "" {
		state.upscaleVideoCodec = "H.264"
	}
	if state.upscaleBitrateMode == "" {
		state.upscaleBitrateMode = "CRF"
	}
	if state.upscaleBitratePreset == "" {
		state.upscaleBitratePreset = "2.5 Mbps - Medium"
	}
	if state.upscaleManualBitrate == "" {
		state.upscaleManualBitrate = "2500k"
	}
	if state.upscaleAIPreset == "" {
		state.upscaleAIPreset = "Balanced"
		state.upscaleAIScale = 4.0
		state.upscaleAIScaleUseTarget = true
		state.upscaleAIOutputAdjust = 1.0
		state.upscaleAIDenoise = 0.5
		state.upscaleAITile = 512
		state.upscaleAIOutputFormat = "png"
		state.upscaleAIGPUAuto = true
		state.upscaleAIThreadsLoad = 1
		state.upscaleAIThreadsProc = 2
		state.upscaleAIThreadsSave = 2
	}
	if state.upscaleBlurSigma <= 0 {
		state.upscaleBlurSigma = 1.5
	}

	// Check AI availability on first load
	if state.upscaleAIBackend == "" {
		state.upscaleAIBackend = detectAIUpscaleBackend()
		state.upscaleAIAvailable = state.upscaleAIBackend == "ncnn"
	}
	// Check RIFE availability on first load
	if state.upscaleRIFEBackend == "" {
		state.upscaleRIFEBackend = detectRIFEBackend()
		state.upscaleRIFEAvailable = state.upscaleRIFEBackend == "ncnn"
	}
	if state.upscaleRIFEMultiplier == 0 {
		state.upscaleRIFEMultiplier = 2
	}
	if state.upscaleRIFEModel == "" {
		state.upscaleRIFEModel = rifeModelOptions()[0]
	}
	if len(state.filterActiveChain) > 0 {
		state.upscaleFilterChain = append([]string{}, state.filterActiveChain...)
	}

	// File label
	fileLabel := widget.NewLabel(t.LabelNoFile)
	fileLabel.TextStyle = fyne.TextStyle{Bold: true}

	var videoContainer fyne.CanvasObject
	var sourceResLabel *widget.Label
	if state.upscaleFile != nil {
		fileLabel.SetText(fmt.Sprintf(t.LabelFileFmt, filepath.Base(state.upscaleFile.Path)))
		sourceResLabel = widget.NewLabel(fmt.Sprintf(t.UpscaleSourceFmt, state.upscaleFile.Width, state.upscaleFile.Height))
		sourceResLabel.TextStyle = fyne.TextStyle{Italic: true}
		videoContainer = buildVideoPane(state, fyne.NewSize(480, 270), state.upscaleFile, nil)
	} else {
		sourceResLabel = widget.NewLabel(t.UpscaleSourceNA)
		sourceResLabel.TextStyle = fyne.TextStyle{Italic: true}
		videoContainer = container.NewCenter(widget.NewLabel(t.LabelNoVideoLoaded))
	}

	// Load button
	loadBtn := widget.NewButton(t.ActionLoadVideo, func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()

			path := reader.URI().Path()
			go func() {
				src, err := probeVideo(path)
				if err != nil {
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						dialog.ShowError(err, state.window)
					}, false)
					return
				}

				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					state.upscaleFile = src
					state.showUpscaleView()
				}, false)
			}()
		}, state.window)
	})
	loadBtn.Importance = widget.HighImportance

	// Navigation to Filters module
	filtersNavBtn := widget.NewButton(t.UpscaleAdjustFilters, func() {
		if state.upscaleFile != nil {
			state.filtersFile = state.upscaleFile
		}
		state.showFiltersView()
	})

	mediumBlue := utils.MustHex("#13182B")
	navyBlue := utils.MustHex("#191F35")

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

	// Scaling (method + blur)
	methodLabel := widget.NewLabel(fmt.Sprintf(t.UpscaleMethodFmt, state.upscaleMethod))
	methodSelect := widget.NewSelect([]string{
		"lanczos",  // Sharp, best general purpose
		"bicubic",  // Smooth
		"spline",   // Balanced
		"bilinear", // Fast, lower quality
	}, func(s string) {
		state.upscaleMethod = s
		methodLabel.SetText(fmt.Sprintf(t.UpscaleMethodFmt, s))
	})
	methodSelect.SetSelected(state.upscaleMethod)

	methodInfo := widget.NewLabel("Lanczos: Sharp, best quality\nBicubic: Smooth\nSpline: Balanced\nBilinear: Fast")
	methodInfo.TextStyle = fyne.TextStyle{Italic: true}
	methodInfo.Wrapping = fyne.TextWrapWord

	blurLabel := widget.NewLabel(fmt.Sprintf(t.UpscaleBlurFmt, state.upscaleBlurSigma))
	blurSlider := widget.NewSlider(0.0, 8.0)
	blurSlider.Step = 0.1
	blurSlider.Value = state.upscaleBlurSigma
	blurSlider.OnChanged = func(v float64) {
		state.upscaleBlurSigma = v
		blurLabel.SetText(fmt.Sprintf(t.UpscaleBlurFmt, v))
	}

	blurCheck := widget.NewCheck(t.UpscaleEnableBlur, func(checked bool) {
		state.upscaleBlurEnabled = checked
		if checked {
			blurSlider.Enable()
		} else {
			blurSlider.Disable()
		}
	})
	blurCheck.SetChecked(state.upscaleBlurEnabled)
	if state.upscaleBlurEnabled {
		blurSlider.Enable()
	} else {
		blurSlider.Disable()
	}

	// Resolution
	resLabel := widget.NewLabel(fmt.Sprintf(t.UpscaleTargetFmt, state.upscaleTargetRes))
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
		state.upscaleTargetRes = s
		resLabel.SetText(fmt.Sprintf(t.UpscaleTargetFmt, s))
	})
	resSelect.SetSelected(state.upscaleTargetRes)

	resolutionSection := buildUpscaleBox(t.UpscaleTargetResBox, container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel(t.UpscaleResLabel),
			resSelect,
		),
		resLabel,
		sourceResLabel,
	))

	// Video Encoding
	qualitySelect := widget.NewSelect([]string{
		"Lossless (CRF 0)",
		"Near-lossless (CRF 16)",
		"High (CRF 18)",
	}, func(s string) {
		state.upscaleQualityPreset = s
	})
	qualitySelect.SetSelected(state.upscaleQualityPreset)

	encoderPresetSelect := widget.NewSelect([]string{
		"ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow", "slower", "veryslow",
	}, func(s string) {
		state.upscaleEncoderPreset = s
	})
	encoderPresetSelect.SetSelected(state.upscaleEncoderPreset)

	// Video Codec selection
	videoCodecOptions := []string{"H.264", "H.265", "VP9", "AV1", "Copy"}
	videoCodecSelect := widget.NewSelect(videoCodecOptions, func(value string) {
		state.upscaleVideoCodec = value
	})
	videoCodecSelect.SetSelected(state.upscaleVideoCodec)

	bitrateModeSelect := widget.NewSelect([]string{
		"CRF (Constant Rate Factor)",
		"CBR (Constant Bitrate)",
		"VBR (Variable Bitrate)",
	}, func(s string) {
		switch {
		case strings.HasPrefix(s, "CRF"):
			state.upscaleBitrateMode = "CRF"
		case strings.HasPrefix(s, "CBR"):
			state.upscaleBitrateMode = "CBR"
		case strings.HasPrefix(s, "VBR"):
			state.upscaleBitrateMode = "VBR"
		default:
			state.upscaleBitrateMode = s
		}
	})
	switch state.upscaleBitrateMode {
	case "CBR":
		bitrateModeSelect.SetSelected("CBR (Constant Bitrate)")
	case "VBR":
		bitrateModeSelect.SetSelected("VBR (Variable Bitrate)")
	default:
		bitrateModeSelect.SetSelected("CRF (Constant Rate Factor)")
	}

	encodingSection := buildUpscaleBox(t.UpscaleEncodingBox, container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel(t.UpscaleVideoCodecLabel),
			videoCodecSelect,
		),
		container.NewGridWithColumns(2,
			widget.NewLabel(t.UpscaleEncoderLabel),
			encoderPresetSelect,
		),
		container.NewGridWithColumns(2,
			widget.NewLabel(t.UpscaleQualityLabel),
			qualitySelect,
		),
		container.NewGridWithColumns(2,
			widget.NewLabel(t.UpscaleBitrateLabel),
			bitrateModeSelect,
		),
		widget.NewLabel(t.UpscaleBitrateHint),
	))

	// Frame Rate
	frameRateLabel := widget.NewLabel(fmt.Sprintf(t.UpscaleFrameRateFmt, state.upscaleFrameRate))
	frameRateSelect := widget.NewSelect([]string{"Source", "23.976", "24", "25", "29.97", "30", "50", "59.94", "60"}, func(s string) {
		state.upscaleFrameRate = s
		frameRateLabel.SetText(fmt.Sprintf(t.UpscaleFrameRateFmt, s))
	})
	frameRateSelect.SetSelected(state.upscaleFrameRate)

	motionInterpCheck := widget.NewCheck(t.UpscaleMotionInterp, func(checked bool) {
		state.upscaleMotionInterpolation = checked
	})
	motionInterpCheck.SetChecked(state.upscaleMotionInterpolation)

	frameRateSection := buildUpscaleBox(t.UpscaleFrameRateBox, container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel(t.UpscaleTargetFPSLabel),
			frameRateSelect,
		),
		frameRateLabel,
		motionInterpCheck,
		widget.NewLabel(t.UpscaleMotionHint),
	))

	aiModelOptions := aiUpscaleModelOptions()
	aiModelLabel := aiUpscaleModelLabel(state.upscaleAIModel)
	if aiModelLabel == "" && len(aiModelOptions) > 0 {
		aiModelLabel = aiModelOptions[0]
	}

	// AI Upscaling Section (nested under Scaling)
	var aiContent fyne.CanvasObject
	if state.upscaleAIAvailable {
		var aiTileSelect *widget.Select
		var aiTTACheck *widget.Check
		var aiDenoiseSlider *widget.Slider
		var denoiseHint *widget.Label

		applyAIPreset := func(preset string) {
			state.upscaleAIPreset = preset
			switch preset {
			case "Ultra Fast":
				state.upscaleAITile = 800
				state.upscaleAITTA = false
			case "Fast":
				state.upscaleAITile = 800
				state.upscaleAITTA = false
			case "Balanced":
				state.upscaleAITile = 512
				state.upscaleAITTA = false
			case "High Quality":
				state.upscaleAITile = 256
				state.upscaleAITTA = false
			case "Maximum Quality":
				state.upscaleAITile = 0
				state.upscaleAITTA = true
			}
			if aiTileSelect != nil {
				switch state.upscaleAITile {
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
				aiTTACheck.SetChecked(state.upscaleAITTA)
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
			state.upscaleAIEnabled = checked
		})
		aiEnabledCheck.SetChecked(state.upscaleAIEnabled)

		aiModelSelect := widget.NewSelect(aiModelOptions, func(s string) {
			state.upscaleAIModel = aiUpscaleModelID(s)
			aiModelLabel = s
			updateDenoiseAvailability(state.upscaleAIModel)
		})
		if aiModelLabel != "" {
			aiModelSelect.SetSelected(aiModelLabel)
		}

		aiPresetSelect := widget.NewSelect([]string{"Ultra Fast", "Fast", "Balanced", "High Quality", "Maximum Quality"}, func(s string) {
			applyAIPreset(s)
		})
		aiPresetSelect.SetSelected(state.upscaleAIPreset)

		aiScaleSelect := widget.NewSelect([]string{"Match Target", "1x", "2x", "3x", "4x", "8x"}, func(s string) {
			if s == "Match Target" {
				state.upscaleAIScaleUseTarget = true
				return
			}
			state.upscaleAIScaleUseTarget = false
			switch s {
			case "1x":
				state.upscaleAIScale = 1
			case "2x":
				state.upscaleAIScale = 2
			case "3x":
				state.upscaleAIScale = 3
			case "4x":
				state.upscaleAIScale = 4
			case "8x":
				state.upscaleAIScale = 8
			}
		})
		if state.upscaleAIScaleUseTarget {
			aiScaleSelect.SetSelected("Match Target")
		} else {
			aiScaleSelect.SetSelected(fmt.Sprintf("%.0fx", state.upscaleAIScale))
		}

		aiAdjustLabel := widget.NewLabel(fmt.Sprintf(t.UpscaleAdjustFmt, state.upscaleAIOutputAdjust))
		aiAdjustSlider := widget.NewSlider(0.5, 2.0)
		aiAdjustSlider.Value = state.upscaleAIOutputAdjust
		aiAdjustSlider.Step = 0.05
		aiAdjustSlider.OnChanged = func(v float64) {
			state.upscaleAIOutputAdjust = v
			aiAdjustLabel.SetText(fmt.Sprintf(t.UpscaleAdjustFmt, v))
		}

		aiDenoiseLabel := widget.NewLabel(fmt.Sprintf(t.UpscaleDenoiseFmt, state.upscaleAIDenoise))
		aiDenoiseSlider = widget.NewSlider(0.0, 1.0)
		aiDenoiseSlider.Value = state.upscaleAIDenoise
		aiDenoiseSlider.Step = 0.05
		aiDenoiseSlider.OnChanged = func(v float64) {
			state.upscaleAIDenoise = v
			aiDenoiseLabel.SetText(fmt.Sprintf(t.UpscaleDenoiseFmt, v))
		}

		aiTileSelect = widget.NewSelect([]string{"Auto", "256", "512", "800"}, func(s string) {
			switch s {
			case "Auto":
				state.upscaleAITile = 0
			case "256":
				state.upscaleAITile = 256
			case "512":
				state.upscaleAITile = 512
			case "800":
				state.upscaleAITile = 800
			}
		})
		switch state.upscaleAITile {
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
			state.upscaleAIOutputFormat = strings.ToLower(s)
		})
		switch strings.ToLower(state.upscaleAIOutputFormat) {
		case "jpg", "jpeg":
			aiOutputFormatSelect.SetSelected("JPG")
		case "webp":
			aiOutputFormatSelect.SetSelected("WEBP")
		default:
			aiOutputFormatSelect.SetSelected("PNG")
		}

		aiFaceCheck := widget.NewCheck(t.UpscaleFaceEnhance, func(checked bool) {
			state.upscaleAIFaceEnhance = checked
		})
		aiFaceAvailable := checkAIFaceEnhanceAvailable(state.upscaleAIBackend)
		if !aiFaceAvailable {
			aiFaceCheck.Disable()
		}
		aiFaceCheck.SetChecked(state.upscaleAIFaceEnhance && aiFaceAvailable)

		aiTTACheck = widget.NewCheck(t.UpscaleTTACheck, func(checked bool) {
			state.upscaleAITTA = checked
		})
		aiTTACheck.SetChecked(state.upscaleAITTA)

		aiGPUSelect := widget.NewSelect([]string{"Auto", "0", "1", "2"}, func(s string) {
			if s == "Auto" {
				state.upscaleAIGPUAuto = true
				return
			}
			state.upscaleAIGPUAuto = false
			if gpu, err := strconv.Atoi(s); err == nil {
				state.upscaleAIGPU = gpu
			}
		})
		if state.upscaleAIGPUAuto {
			aiGPUSelect.SetSelected("Auto")
		} else {
			aiGPUSelect.SetSelected(strconv.Itoa(state.upscaleAIGPU))
		}

		threadOptions := []string{"1", "2", "3", "4"}
		aiThreadsLoad := widget.NewSelect(threadOptions, func(s string) {
			if v, err := strconv.Atoi(s); err == nil {
				state.upscaleAIThreadsLoad = v
			}
		})
		aiThreadsLoad.SetSelected(strconv.Itoa(state.upscaleAIThreadsLoad))

		aiThreadsProc := widget.NewSelect(threadOptions, func(s string) {
			if v, err := strconv.Atoi(s); err == nil {
				state.upscaleAIThreadsProc = v
			}
		})
		aiThreadsProc.SetSelected(strconv.Itoa(state.upscaleAIThreadsProc))

		aiThreadsSave := widget.NewSelect(threadOptions, func(s string) {
			if v, err := strconv.Atoi(s); err == nil {
				state.upscaleAIThreadsSave = v
			}
		})
		aiThreadsSave.SetSelected(strconv.Itoa(state.upscaleAIThreadsSave))

		denoiseHint = widget.NewLabel("")
		denoiseHint.TextStyle = fyne.TextStyle{Italic: true}
		updateDenoiseAvailability(state.upscaleAIModel)

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
		if state.upscaleAIBackend == "python" {
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
		widget.NewLabel(t.UpscaleOptionalBlur),
		blurCheck,
		container.NewVBox(blurLabel, blurSlider),
		widget.NewSeparator(),
		aiSection,
	))

	// Filter Integration Section
	filterIntegrationSelect := widget.NewSelect([]string{
		"None",
		"Apply filters before upscaling",
	}, func(s string) {
		state.upscaleApplyFilters = s == "Apply filters before upscaling"
	})
	if state.upscaleApplyFilters {
		filterIntegrationSelect.SetSelected("Apply filters before upscaling")
	} else {
		filterIntegrationSelect.SetSelected("None")
	}

	filterIntegrationSection := buildUpscaleBox(t.UpscaleFilterIntBox, container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel(t.UpscaleFilterIntLabel),
			filterIntegrationSelect,
		),
		widget.NewLabel(t.UpscaleFilterIntHint),
	))

	// RIFE Frame Interpolation Section
	var rifeSection fyne.CanvasObject
	if state.upscaleRIFEAvailable {
		var updateEstFPS func()
		var estFPSLabel *widget.Label

		rifeMultiplierSelect := widget.NewSelect([]string{"2×", "4×"}, func(s string) {
			if s == "4×" {
				state.upscaleRIFEMultiplier = 4
			} else {
				state.upscaleRIFEMultiplier = 2
			}
			if updateEstFPS != nil {
				updateEstFPS()
			}
		})
		if state.upscaleRIFEMultiplier == 4 {
			rifeMultiplierSelect.SetSelected("4×")
		} else {
			rifeMultiplierSelect.SetSelected("2×")
		}

		rifeModelSelect := widget.NewSelect(rifeModelOptions(), func(s string) {
			state.upscaleRIFEModel = s
		})
		rifeModelSelect.SetSelected(state.upscaleRIFEModel)

		rifeEnabledCheck := widget.NewCheck(t.RIFEEnabled, func(checked bool) {
			state.upscaleRIFEEnabled = checked
		})
		rifeEnabledCheck.SetChecked(state.upscaleRIFEEnabled)

		estFPSLabel = widget.NewLabel("")
		estFPSLabel.TextStyle = fyne.TextStyle{Italic: true}
		updateEstFPS = func() {
			if state.upscaleFile != nil && state.upscaleFile.FrameRate > 0 {
				estFPSLabel.SetText(fmt.Sprintf(t.RIFEEstFPSFmt, state.upscaleFile.FrameRate*float64(state.upscaleRIFEMultiplier)))
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
		rifeSection = buildUpscaleBox(t.RIFEBoxTitle, container.NewVBox(
			widget.NewLabel(t.RIFENotDetected),
			widget.NewLabel("https://github.com/nihui/rife-ncnn-vulkan"),
		))
	}

	// Helper function to create upscale job
	createUpscaleJob := func() (*queue.Job, error) {
		if state.upscaleFile == nil {
			return nil, fmt.Errorf("no video loaded")
		}

		// Parse target resolution (preserve aspect by default)
		targetWidth, targetHeight, preserveAspect, err := parseResolutionPreset(state.upscaleTargetRes, state.upscaleFile.Width, state.upscaleFile.Height)
		if err != nil {
			return nil, fmt.Errorf("invalid resolution: %w", err)
		}

		// Build output path
		videoDir := filepath.Dir(state.upscaleFile.Path)
		videoBaseName := strings.TrimSuffix(filepath.Base(state.upscaleFile.Path), filepath.Ext(state.upscaleFile.Path))
		slug := sanitizeForPath(state.upscaleTargetRes)
		if slug == "" {
			slug = "source"
		}
		outputPath := filepath.Join(videoDir, fmt.Sprintf("%s_upscaled_%s_%s.mkv",
			videoBaseName, slug, state.upscaleMethod))

		// Build description
		description := fmt.Sprintf("Upscale to %s using %s", state.upscaleTargetRes, state.upscaleMethod)
		if state.upscaleAIEnabled && state.upscaleAIAvailable {
			description += fmt.Sprintf(" + AI (%s)", state.upscaleAIModel)
		}
		if state.upscaleRIFEEnabled && state.upscaleRIFEAvailable {
			description += fmt.Sprintf(" + RIFE %dx", state.upscaleRIFEMultiplier)
		}

		desc := fmt.Sprintf("%s  %s", description, filepath.Base(outputPath))

		return &queue.Job{
			Type:        queue.JobTypeUpscale,
			Title:       "Upscale: " + filepath.Base(state.upscaleFile.Path),
			Description: desc,
			OutputFile:  outputPath,
			Config: map[string]interface{}{
				"inputPath":              state.upscaleFile.Path,
				"outputPath":             outputPath,
				"method":                 state.upscaleMethod,
				"encoderPreset":          state.upscaleEncoderPreset,
				"bitrateMode":            state.upscaleBitrateMode,
				"bitratePreset":          state.upscaleBitratePreset,
				"manualBitrate":          state.upscaleManualBitrate,
				"targetWidth":            float64(targetWidth),
				"targetHeight":           float64(targetHeight),
				"targetPreset":           state.upscaleTargetRes,
				"sourceWidth":            float64(state.upscaleFile.Width),
				"sourceHeight":           float64(state.upscaleFile.Height),
				"preserveAR":             preserveAspect,
				"useAI":                  state.upscaleAIEnabled && state.upscaleAIAvailable,
				"aiModel":                state.upscaleAIModel,
				"qualityPreset":          state.upscaleQualityPreset,
				"aiBackend":              state.upscaleAIBackend,
				"aiPreset":               state.upscaleAIPreset,
				"aiScale":                state.upscaleAIScale,
				"aiScaleUseTarget":       state.upscaleAIScaleUseTarget,
				"aiOutputAdjust":         state.upscaleAIOutputAdjust,
				"aiFaceEnhance":          state.upscaleAIFaceEnhance,
				"aiDenoise":              state.upscaleAIDenoise,
				"aiTile":                 float64(state.upscaleAITile),
				"aiGPU":                  float64(state.upscaleAIGPU),
				"aiGPUAuto":              state.upscaleAIGPUAuto,
				"aiThreadsLoad":          float64(state.upscaleAIThreadsLoad),
				"aiThreadsProc":          float64(state.upscaleAIThreadsProc),
				"aiThreadsSave":          float64(state.upscaleAIThreadsSave),
				"aiTTA":                  state.upscaleAITTA,
				"aiOutputFormat":         state.upscaleAIOutputFormat,
				"applyFilters":           state.upscaleApplyFilters,
				"filterChain":            state.upscaleFilterChain,
				"blurEnabled":            state.upscaleBlurEnabled,
				"blurSigma":              state.upscaleBlurSigma,
				"videoCodec":             state.upscaleVideoCodec,
				"duration":               state.upscaleFile.Duration,
				"sourceFrameRate":        state.upscaleFile.FrameRate,
				"frameRate":              state.upscaleFrameRate,
				"useMotionInterpolation": state.upscaleMotionInterpolation,
				"useRIFE":                state.upscaleRIFEEnabled && state.upscaleRIFEAvailable,
				"rifeModel":              state.upscaleRIFEModel,
				"rifeMultiplier":         float64(state.upscaleRIFEMultiplier),
			},
		}, nil
	}

	// Apply/Queue buttons
	applyBtn := widget.NewButton(t.UpscaleNow, func() {
		job, err := createUpscaleJob()
		if err != nil {
			dialog.ShowError(err, state.window)
			return
		}

		state.jobQueue.Add(job)
		if !state.jobQueue.IsRunning() {
			state.jobQueue.Start()
		}
		dialog.ShowInformation(t.UpscaleStartedTitle,
			fmt.Sprintf(t.UpscaleStartedFmt, state.upscaleTargetRes),
			state.window)
	})
	applyBtn.Importance = widget.HighImportance

	// Keyboard shortcut: Ctrl+Enter -> Upscale Now
	if c := state.window.Canvas(); c != nil {
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
			dialog.ShowError(err, state.window)
			return
		}

		state.jobQueue.Add(job)
		dialog.ShowInformation(t.UpscaleAddedTitle,
			fmt.Sprintf(t.UpscaleAddedFmt, state.upscaleTargetRes, state.upscaleMethod),
			state.window)
	})
	addQueueBtn.Importance = widget.MediumImportance

	// Main content
	spacing := func() fyne.CanvasObject {
		spacer := canvas.NewRectangle(color.Transparent)
		spacer.SetMinSize(fyne.NewSize(0, 10))
		return spacer
	}

	metaPanel, _ := buildMetadataPanel(state, state.upscaleFile, fyne.NewSize(0, 200))

	leftPanel := container.NewVBox(
		buildUpscaleBox(t.UpscaleVideoBox, container.NewVBox(
			fileLabel,
			loadBtn,
			filtersNavBtn,
			videoContainer,
		)),
		spacing(),
		metaPanel,
	)

	settingsPanel := container.NewVBox(
		traditionalSection,
		spacing(),
		resolutionSection,
		spacing(),
		encodingSection,
		spacing(),
		frameRateSection,
		spacing(),
		rifeSection,
		spacing(),
		filterIntegrationSection,
	)

	settingsScroll := ui.NewFastVScroll(settingsPanel)
	// Adaptive height for small screens
	// Avoid rigid min sizes so window snapping works across modules.

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
	bottomBar := moduleFooter(upscaleColor, actionBar, state.statsBar)

	return container.NewBorder(topBar, bottomBar, nil, nil, content)
}
