package main

import (
	"fmt"
	"image/color"
	"os/exec"
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
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

// AI Helper Functions (smaller, manageable functions)

// detectAIUpscaleBackend returns the available Real-ESRGAN backend ("ncnn", "python", or "").
func detectAIUpscaleBackend() string {
	if _, err := exec.LookPath("realesrgan-ncnn-vulkan"); err == nil {
		return "ncnn"
	}

	cmd := utils.HideWindowExec("python3", "-c", "import realesrgan")
	if err := cmd.Run(); err == nil {
		return "python"
	}

	cmd = utils.HideWindowExec("python", "-c", "import realesrgan")
	if err := cmd.Run(); err == nil {
		return "python"
	}

	return ""
}

// checkAIFaceEnhanceAvailable verifies whether face enhancement tooling is available.
func checkAIFaceEnhanceAvailable(backend string) bool {
	if backend != "python" {
		return false
	}
	cmd := exec.Command("python3", "-c", "import realesrgan, gfpgan")
	utils.ApplyNoWindow(cmd)
	if err := cmd.Run(); err == nil {
		return true
	}
	cmd = exec.Command("python", "-c", "import realesrgan, gfpgan")
	utils.ApplyNoWindow(cmd)
	return cmd.Run() == nil
}

func aiUpscaleModelOptions() []string {
	return []string{
		"General (RealESRGAN_x4plus)",
		"Anime/Illustration (RealESRGAN_x4plus_anime_6B)",
		"Anime Video (realesr-animevideov3)",
		"General Tiny (realesr-general-x4v3)",
		"2x General (RealESRGAN_x2plus)",
		"Clean Restore (realesrnet-x4plus)",
	}
}

func aiUpscaleModelID(label string) string {
	switch label {
	case "Anime/Illustration (RealESRGAN_x4plus_anime_6B)":
		return "realesrgan-x4plus-anime"
	case "Anime Video (realesr-animevideov3)":
		return "realesr-animevideov3"
	case "General Tiny (realesr-general-x4v3)":
		return "realesr-general-x4v3"
	case "2x General (RealESRGAN_x2plus)":
		return "realesrgan-x2plus"
	case "Clean Restore (realesrnet-x4plus)":
		return "realesrnet-x4plus"
	default:
		return "realesrgan-x4plus"
	}
}

func aiUpscaleModelLabel(modelID string) string {
	switch modelID {
	case "realesrgan-x4plus-anime":
		return "Anime/Illustration (RealESRGAN_x4plus_anime_6B)"
	case "realesr-animevideov3":
		return "Anime Video (realesr-animevideov3)"
	case "realesr-general-x4v3":
		return "General Tiny (realesr-general-x4v3)"
	case "realesrgan-x2plus":
		return "2x General (RealESRGAN_x2plus)"
	case "realesrnet-x4plus":
		return "Clean Restore (realesrnet-x4plus)"
	case "realesrgan-x4plus":
		return "General (RealESRGAN_x4plus)"
	default:
		return ""
	}
}

// parseResolutionPreset parses resolution preset strings and returns target dimensions and whether to preserve aspect.
// Special presets like "Match Source" and relative (2X/4X) use source dimensions to preserve AR.
func parseResolutionPreset(preset string, srcW, srcH int) (width, height int, preserveAspect bool, err error) {
	// Default: preserve aspect
	preserveAspect = true

	// Sanitize source
	if srcW < 1 || srcH < 1 {
		srcW, srcH = 1920, 1080 // fallback to avoid zero division
	}

	switch preset {
	case "", "Match Source":
		return srcW, srcH, true, nil
	case "2X (relative)":
		return srcW * 2, srcH * 2, true, nil
	case "4X (relative)":
		return srcW * 4, srcH * 4, true, nil
	}

	presetMap := map[string][2]int{
		"720p (1280x720)":   {1280, 720},
		"1080p (1920x1080)": {1920, 1080},
		"1440p (2560x1440)": {2560, 1440},
		"4K (3840x2160)":    {3840, 2160},
		"8K (7680x4320)":    {7680, 4320},
		"720p":              {1280, 720},
		"1080p":             {1920, 1080},
		"1440p":             {2560, 1440},
		"4K":                {3840, 2160},
		"8K":                {7680, 4320},
	}

	if dims, ok := presetMap[preset]; ok {
		// Keep aspect by default: use target height and let FFmpeg derive width
		return dims[0], dims[1], true, nil
	}

	return 0, 0, true, fmt.Errorf("unknown resolution preset: %s", preset)
}

// buildUpscaleFilter builds FFmpeg scale filter string with selected method
func buildUpscaleFilter(targetWidth, targetHeight int, method string, preserveAspect bool) string {
	// Ensure even dimensions for encoders
	makeEven := func(v int) int {
		if v%2 != 0 {
			return v + 1
		}
		return v
	}

	h := makeEven(targetHeight)
	w := targetWidth
	if preserveAspect || w <= 0 {
		w = -2 // FFmpeg will derive width from height while preserving AR
	}
	return fmt.Sprintf("scale=%d:%d:flags=%s", w, h, method)
}

// sanitizeForPath creates a simple slug for filenames from user-visible labels
func sanitizeForPath(label string) string {
	r := strings.NewReplacer(
		" ", "",
		"(", "",
		")", "",
		"×", "x",
		"/", "-",
		"\\", "-",
		":", "-",
		",", "",
		".", "",
		"_", "",
		"'", "",
		"\"", "",
		"`", "",
		"!", "",
		"?", "",
		"&", "and",
	)
	return strings.ToLower(r.Replace(label))
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

	// Back button
	backBtn := widget.NewButton("< UPSCALE", func() {
		state.showMainMenu()
	})
	backBtn.Importance = widget.LowImportance

	// Queue button
	queueBtn := widget.NewButton("View Queue", func() {
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
	if len(state.filterActiveChain) > 0 {
		state.upscaleFilterChain = append([]string{}, state.filterActiveChain...)
	}

	// File label
	fileLabel := widget.NewLabel("No file loaded")
	fileLabel.TextStyle = fyne.TextStyle{Bold: true}

	var videoContainer fyne.CanvasObject
	var sourceResLabel *widget.Label
	if state.upscaleFile != nil {
		fileLabel.SetText(fmt.Sprintf("File: %s", filepath.Base(state.upscaleFile.Path)))
		sourceResLabel = widget.NewLabel(fmt.Sprintf("Source: %dx%d", state.upscaleFile.Width, state.upscaleFile.Height))
		sourceResLabel.TextStyle = fyne.TextStyle{Italic: true}
		videoContainer = buildVideoPane(state, fyne.NewSize(480, 270), state.upscaleFile, nil)
	} else {
		sourceResLabel = widget.NewLabel("Source: N/A")
		sourceResLabel.TextStyle = fyne.TextStyle{Italic: true}
		videoContainer = container.NewCenter(widget.NewLabel("No video loaded"))
	}

	// Load button
	loadBtn := widget.NewButton("Load Video", func() {
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
	filtersNavBtn := widget.NewButton(" Adjust Filters", func() {
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
		body := container.NewVBox(
			widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewSeparator(),
			content,
		)
		layers := ui.NoisyBackgroundObjects(bg)
		layers = append(layers, container.NewPadded(body))
		return container.NewMax(layers...)
	}

	// Scaling (method + blur)
	methodLabel := widget.NewLabel(fmt.Sprintf("Method: %s", state.upscaleMethod))
	methodSelect := widget.NewSelect([]string{
		"lanczos",  // Sharp, best general purpose
		"bicubic",  // Smooth
		"spline",   // Balanced
		"bilinear", // Fast, lower quality
	}, func(s string) {
		state.upscaleMethod = s
		methodLabel.SetText(fmt.Sprintf("Method: %s", s))
	})
	methodSelect.SetSelected(state.upscaleMethod)

	methodInfo := widget.NewLabel("Lanczos: Sharp, best quality\nBicubic: Smooth\nSpline: Balanced\nBilinear: Fast")
	methodInfo.TextStyle = fyne.TextStyle{Italic: true}
	methodInfo.Wrapping = fyne.TextWrapWord

	blurLabel := widget.NewLabel(fmt.Sprintf("Blur Strength: %.2f", state.upscaleBlurSigma))
	blurSlider := widget.NewSlider(0.0, 8.0)
	blurSlider.Step = 0.1
	blurSlider.Value = state.upscaleBlurSigma
	blurSlider.OnChanged = func(v float64) {
		state.upscaleBlurSigma = v
		blurLabel.SetText(fmt.Sprintf("Blur Strength: %.2f", v))
	}

	blurCheck := widget.NewCheck("Enable Blur", func(checked bool) {
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
	resLabel := widget.NewLabel(fmt.Sprintf("Target: %s", state.upscaleTargetRes))
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
		resLabel.SetText(fmt.Sprintf("Target: %s", s))
	})
	resSelect.SetSelected(state.upscaleTargetRes)

	resolutionSection := buildUpscaleBox("Target Resolution", container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel("Resolution:"),
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

	encodingSection := buildUpscaleBox("Video Encoding", container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel("Encoder Preset:"),
			encoderPresetSelect,
		),
		container.NewGridWithColumns(2,
			widget.NewLabel("Quality Preset:"),
			qualitySelect,
		),
		container.NewGridWithColumns(2,
			widget.NewLabel("Bitrate Mode:"),
			bitrateModeSelect,
		),
		widget.NewLabel("CRF controls quality; bitrate modes target size."),
	))

	// Frame Rate
	frameRateLabel := widget.NewLabel(fmt.Sprintf("Frame Rate: %s", state.upscaleFrameRate))
	frameRateSelect := widget.NewSelect([]string{"Source", "23.976", "24", "25", "29.97", "30", "50", "59.94", "60"}, func(s string) {
		state.upscaleFrameRate = s
		frameRateLabel.SetText(fmt.Sprintf("Frame Rate: %s", s))
	})
	frameRateSelect.SetSelected(state.upscaleFrameRate)

	motionInterpCheck := widget.NewCheck("Use Motion Interpolation (slower, smoother)", func(checked bool) {
		state.upscaleMotionInterpolation = checked
	})
	motionInterpCheck.SetChecked(state.upscaleMotionInterpolation)

	frameRateSection := buildUpscaleBox("Frame Rate", container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel("Target FPS:"),
			frameRateSelect,
		),
		frameRateLabel,
		motionInterpCheck,
		widget.NewLabel("Motion interpolation creates smooth in-between frames"),
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

		updateDenoiseAvailability := func(model string) {
			if aiDenoiseSlider == nil || denoiseHint == nil {
				return
			}
			if model == "realesr-general-x4v3" {
				aiDenoiseSlider.Enable()
				denoiseHint.SetText("Denoise available on General Tiny model")
			} else {
				aiDenoiseSlider.Disable()
				denoiseHint.SetText("Denoise only supported on General Tiny model")
			}
		}

		aiEnabledCheck := widget.NewCheck("Use AI Upscaling", func(checked bool) {
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

		aiAdjustLabel := widget.NewLabel(fmt.Sprintf("Adjustment: %.2fx", state.upscaleAIOutputAdjust))
		aiAdjustSlider := widget.NewSlider(0.5, 2.0)
		aiAdjustSlider.Value = state.upscaleAIOutputAdjust
		aiAdjustSlider.Step = 0.05
		aiAdjustSlider.OnChanged = func(v float64) {
			state.upscaleAIOutputAdjust = v
			aiAdjustLabel.SetText(fmt.Sprintf("Adjustment: %.2fx", v))
		}

		aiDenoiseLabel := widget.NewLabel(fmt.Sprintf("Denoise: %.2f", state.upscaleAIDenoise))
		aiDenoiseSlider = widget.NewSlider(0.0, 1.0)
		aiDenoiseSlider.Value = state.upscaleAIDenoise
		aiDenoiseSlider.Step = 0.05
		aiDenoiseSlider.OnChanged = func(v float64) {
			state.upscaleAIDenoise = v
			aiDenoiseLabel.SetText(fmt.Sprintf("Denoise: %.2f", v))
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

		aiFaceCheck := widget.NewCheck("Face Enhancement (requires Python/GFPGAN)", func(checked bool) {
			state.upscaleAIFaceEnhance = checked
		})
		aiFaceAvailable := checkAIFaceEnhanceAvailable(state.upscaleAIBackend)
		if !aiFaceAvailable {
			aiFaceCheck.Disable()
		}
		aiFaceCheck.SetChecked(state.upscaleAIFaceEnhance && aiFaceAvailable)

		aiTTACheck = widget.NewCheck("Enable TTA (slower, higher quality)", func(checked bool) {
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
			widget.NewLabel("Real-ESRGAN detected - enhanced quality available"),
			aiEnabledCheck,
			container.NewGridWithColumns(2,
				widget.NewLabel("AI Model:"),
				aiModelSelect,
			),
			container.NewGridWithColumns(2,
				widget.NewLabel("Processing Preset:"),
				aiPresetSelect,
			),
			container.NewGridWithColumns(2,
				widget.NewLabel("Upscale Factor:"),
				aiScaleSelect,
			),
			container.NewVBox(aiAdjustLabel, aiAdjustSlider),
			container.NewVBox(aiDenoiseLabel, aiDenoiseSlider, denoiseHint),
			container.NewGridWithColumns(2,
				widget.NewLabel("Tile Size:"),
				aiTileSelect,
			),
			container.NewGridWithColumns(2,
				widget.NewLabel("Output Frames:"),
				aiOutputFormatSelect,
			),
			aiFaceCheck,
			aiTTACheck,
			widget.NewSeparator(),
			widget.NewLabel("Advanced (ncnn backend)"),
			container.NewGridWithColumns(2,
				widget.NewLabel("GPU:"),
				aiGPUSelect,
			),
			container.NewGridWithColumns(2,
				widget.NewLabel("Threads (Load/Proc/Save):"),
				container.NewGridWithColumns(3, aiThreadsLoad, aiThreadsProc, aiThreadsSave),
			),
			widget.NewLabel("Note: AI upscaling is slower but produces higher quality results"),
		)
	} else {
		backendNote := "Real-ESRGAN not detected. Install for enhanced quality:"
		if state.upscaleAIBackend == "python" {
			backendNote = "Python Real-ESRGAN detected, but the ncnn backend is required for now."
		}
		aiContent = container.NewVBox(
			widget.NewLabel(backendNote),
			widget.NewLabel("https://github.com/xinntao/Real-ESRGAN"),
			widget.NewLabel("Traditional scaling methods will be used."),
		)
	}

	aiSection := container.NewVBox(
		widget.NewLabelWithStyle("AI Upscaling", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		aiContent,
	)

	traditionalSection := buildUpscaleBox("Scaling", container.NewVBox(
		widget.NewLabel("Classic upscaling methods - always available"),
		container.NewGridWithColumns(2,
			widget.NewLabel("Scaling Algorithm:"),
			methodSelect,
		),
		methodLabel,
		widget.NewSeparator(),
		methodInfo,
		widget.NewSeparator(),
		widget.NewLabel("Optional blur"),
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

	filterIntegrationSection := buildUpscaleBox("Filter Integration", container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel("Filter Integration:"),
			filterIntegrationSelect,
		),
		widget.NewLabel("Filters from the Filters module are applied before upscaling."),
	))

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
				"duration":               state.upscaleFile.Duration,
				"sourceFrameRate":        state.upscaleFile.FrameRate,
				"frameRate":              state.upscaleFrameRate,
				"useMotionInterpolation": state.upscaleMotionInterpolation,
			},
		}, nil
	}

	// Apply/Queue buttons
	applyBtn := widget.NewButton("UPSCALE NOW", func() {
		job, err := createUpscaleJob()
		if err != nil {
			dialog.ShowError(err, state.window)
			return
		}

		state.jobQueue.Add(job)
		if !state.jobQueue.IsRunning() {
			state.jobQueue.Start()
		}
		dialog.ShowInformation("Upscale Started",
			fmt.Sprintf("Upscaling to %s.\nCheck the queue for progress.", state.upscaleTargetRes),
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

	addQueueBtn := widget.NewButton("Add to Queue", func() {
		job, err := createUpscaleJob()
		if err != nil {
			dialog.ShowError(err, state.window)
			return
		}

		state.jobQueue.Add(job)
		dialog.ShowInformation("Added to Queue",
			fmt.Sprintf("Upscale job added.\nTarget: %s, Method: %s", state.upscaleTargetRes, state.upscaleMethod),
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
		buildUpscaleBox("Video", container.NewVBox(
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
