package main

import (
	"fmt"
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

func (s *appState) showFiltersView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "filters"
	s.maximizeWindow()
	s.setContent(buildFiltersView(s))
}

// buildStylisticFilterChain creates FFmpeg filter chains for decade-based stylistic effects
func buildStylisticFilterChain(state *appState) []string {
	var chain []string

	switch state.filterStylisticMode {
	case "8mm Film":
		// 8mm/Super 8 film characteristics (1960s-1980s home movies)
		// - Very fine grain structure
		// - Slight color shifts toward warm/cyan
		// - Film gate weave and frame instability
		// - Lower resolution and softer details
		chain = append(chain, "eq=contrast=1.0:saturation=0.9:brightness=0.02") // Slightly desaturated, natural contrast
		chain = append(chain, "unsharp=6:6:0.2:6:6:0.2")                        // Very soft, film-like
		chain = append(chain, "scale=iw*0.8:ih*0.8:flags=lanczos")              // Lower resolution
		chain = append(chain, "fftnorm=nor=0.08:Links=0")                       // Subtle film grain

		if state.filterTapeNoise > 0 {
			// Film grain with proper frequency
			grain := fmt.Sprintf("fftnorm=nor=%.2f:Links=0", state.filterTapeNoise*0.1)
			chain = append(chain, grain)
		}

		// Subtle frame weave (film movement in gate)
		if state.filterTrackingError > 0 {
			weave := fmt.Sprintf("crop='iw-mod(iw*%f/200,1)':'ih-mod(ih*%f/200,1)':%f:%f",
				state.filterTrackingError, state.filterTrackingError*0.5,
				state.filterTrackingError*2, state.filterTrackingError)
			chain = append(chain, weave)
		}

	case "16mm Film":
		// 16mm film characteristics (professional/educational films 1930s-1990s)
		// - Higher resolution than 8mm but still grainy
		// - More accurate color response
		// - Film scratches and dust (age-dependent)
		// - Stable but still organic movement
		chain = append(chain, "eq=contrast=1.05:saturation=1.0:brightness=0.0") // Natural contrast
		chain = append(chain, "unsharp=5:5:0.4:5:5:0.4")                        // Slightly sharper than 8mm
		chain = append(chain, "scale=iw*0.9:ih*0.9:flags=lanczos")              // Moderate resolution
		chain = append(chain, "fftnorm=nor=0.06:Links=0")                       // Fine grain

		if state.filterTapeNoise > 0 {
			grain := fmt.Sprintf("fftnorm=nor=%.2f:Links=0", state.filterTapeNoise*0.08)
			chain = append(chain, grain)
		}

		if state.filterDropout > 0 {
			// Occasional film scratches
			scratches := int(state.filterDropout * 5) // Max 5 scratches
			if scratches > 0 {
				chain = append(chain, "geq=lum=lum:cb=cb:cr=cr,boxblur=1:1:cr=0:ar=1")
			}
		}

	case "B&W Film":
		// Black and white film characteristics (various eras)
		// - Rich tonal range with silver halide characteristics
		// - Film grain in luminance only
		// - High contrast potential
		// - No color bleeding, but potential for halation
		chain = append(chain, "colorchannelmixer=.299:.587:.114:0:.299:.587:.114:0:.299:.587:.114") // True B&W conversion
		chain = append(chain, "eq=contrast=1.1:brightness=-0.02")                                   // Higher contrast for B&W
		chain = append(chain, "unsharp=4:4:0.3:4:4:0.3")                                            // Moderate sharpness
		chain = append(chain, "fftnorm=nor=0.05:Links=0")                                           // Film grain

		// Add subtle halation effect (bright edge bleed)
		if state.filterColorBleeding {
			chain = append(chain, "unsharp=7:7:0.8:7:7:0.8") // Glow effect for highlights
		}

	case "Silent Film":
		// 1920s silent film characteristics
		// - Very low frame rate (16-22 fps)
		// - Sepia or B&W toning
		// - Film grain with age-related deterioration
		// - Frame jitter and instability
		chain = append(chain, "framerate=18")                                                       // Classic silent film speed
		chain = append(chain, "colorchannelmixer=.393:.769:.189:0:.393:.769:.189:0:.393:.769:.189") // Sepia tone
		chain = append(chain, "eq=contrast=1.15:brightness=0.05")                                   // High contrast, slightly bright
		chain = append(chain, "unsharp=8:8:0.1:8:8:0.1")                                            // Very soft, aged film look
		chain = append(chain, "fftnorm=nor=0.12:Links=0")                                           // Heavy grain

		// Pronounced frame instability
		if state.filterTrackingError > 0 {
			jitter := fmt.Sprintf("crop='iw-mod(iw*%f/100,2)':'ih-mod(ih*%f/100,2)':%f:%f",
				state.filterTrackingError*3, state.filterTrackingError*1.5,
				state.filterTrackingError*5, state.filterTrackingError*2)
			chain = append(chain, jitter)
		}

	case "70s":
		// 1970s film/video characteristics
		// - Lower resolution, softer images
		// - Warmer color temperature, faded colors
		// - Film grain (if film) or early video noise
		// - Slight color shifts common in analog processing
		chain = append(chain, "eq=contrast=0.95:saturation=0.85:brightness=0.05") // Slightly washed out
		chain = append(chain, "unsharp=5:5:0.3:5:5:0.3")                          // Soften
		chain = append(chain, "fftnorm=nor=0.15:Links=0")                         // Subtle noise
		if state.filterChromaNoise > 0 {
			noise := fmt.Sprintf("fftnorm=nor=%.2f:Links=0", state.filterChromaNoise*0.2)
			chain = append(chain, noise)
		}

	case "80s":
		// 1980s video characteristics
		// - Early home video camcorders (VHS, Betamax)
		// - More pronounced color bleeding
		// - Noticeable video noise and artifacts
		// - Stronger contrast, vibrant colors
		chain = append(chain, "eq=contrast=1.1:saturation=1.2:brightness=0.02") // Enhanced contrast/saturation
		chain = append(chain, "unsharp=3:3:0.4:3:3:0.4")                        // Moderate sharpening (80s video look)
		chain = append(chain, "fftnorm=nor=0.2:Links=0")                        // Moderate noise

		if state.filterColorBleeding {
			// Simulate chroma bleeding common in 80s video
			chain = append(chain, "format=yuv420p,scale=iw+2:ih+2:flags=neighbor,crop=iw:ih")
		}

		if state.filterChromaNoise > 0 {
			noise := fmt.Sprintf("fftnorm=nor=%.2f:Links=0", state.filterChromaNoise*0.3)
			chain = append(chain, noise)
		}

	case "90s":
		// 1990s video characteristics
		// - Improved VHS quality, early digital video
		// - Less color bleeding but still present
		// - Better resolution but still analog artifacts
		// - More stable but with tape noise
		chain = append(chain, "eq=contrast=1.05:saturation=1.1:brightness=0.0") // Slight enhancement
		chain = append(chain, "unsharp=3:3:0.5:3:3:0.5")                        // Light sharpening
		chain = append(chain, "fftnorm=nor=0.1:Links=0")                        // Light noise

		if state.filterTapeNoise > 0 {
			// Magnetic tape noise simulation
			noise := fmt.Sprintf("fftnorm=nor=%.2f:Links=0", state.filterTapeNoise*0.15)
			chain = append(chain, noise)
		}

	case "VHS":
		// General VHS characteristics across decades
		// - Resolution: ~240-320 lines horizontal
		// - Chroma subsampling issues
		// - Tracking errors and dropouts
		// - Scanline artifacts
		chain = append(chain, "eq=contrast=1.08:saturation=1.15:brightness=0.03") // VHS color boost
		chain = append(chain, "unsharp=4:4:0.4:4:4:0.4")                          // VHS softness
		chain = append(chain, "fftnorm=nor=0.18:Links=0")                         // VHS noise floor

		if state.filterColorBleeding {
			// Classic VHS chroma bleeding
			chain = append(chain, "format=yuv420p,scale=iw+4:ih+4:flags=neighbor,crop=iw:ih")
		}

		if state.filterTrackingError > 0 {
			// Simulate tracking errors (slight image shifts/stutters)
			errorLevel := state.filterTrackingError * 2.0
			wobble := fmt.Sprintf("crop='iw-mod(iw*%f/100,2)':'ih-mod(ih*%f/100,2)':%f:%f",
				errorLevel, errorLevel/2, errorLevel/2, errorLevel/4)
			chain = append(chain, wobble)
		}

		if state.filterDropout > 0 {
			// Tape dropout effect (random horizontal lines)
			dropoutLevel := int(state.filterDropout * 20) // 0-20 dropouts max
			if dropoutLevel > 0 {
				chain = append(chain, fmt.Sprintf("geq=lum=lum:cb=cb:cr=cr,sendcmd=f=%d:'drawbox w=iw h=2 y=%f:color=black@1:t=fill',drawbox w=iw h=2 y=%f:color=black@1:t=fill'",
					dropoutLevel, 100.0, 200.0))
			}
		}

	case "Webcam":
		// Early 2000s webcam characteristics
		// - Low resolution (320x240, 640x480)
		// - High compression artifacts
		// - Poor low-light performance
		// - Frame rate issues
		chain = append(chain, "eq=contrast=1.15:saturation=0.9:brightness=-0.05") // Webcam contrast boost, desaturation
		chain = append(chain, "scale=640:480:flags=neighbor")                     // Typical low resolution
		chain = append(chain, "unsharp=2:2:0.8:2:2:0.8")                          // Over-sharpened (common in webcams)
		chain = append(chain, "fftnorm=nor=0.25:Links=0")                         // High compression noise

		if state.filterChromaNoise > 0 {
			// Webcam compression artifacts
			noise := fmt.Sprintf("fftnorm=nor=%.2f:Links=0", state.filterChromaNoise*0.4)
			chain = append(chain, noise)
		}
	}

	// Add scanlines if enabled (across all modes)
	if state.filterScanlines {
		// CRT scanline simulation
		scanlineFilter := "format=yuv420p,scale=ih*2/3:ih:flags=neighbor,setsar=1,scale=ih*3/2:ih"
		chain = append(chain, scanlineFilter)
	}

	// Add interlacing if specified
	switch state.filterInterlacing {
	case "Interlaced":
		// Add interlacing artifacts
		chain = append(chain, "interlace=scan=tff:lowpass=1")
	case "Progressive":
		// Ensure progressive output
		chain = append(chain, "yadif=0:-1:0")
	}

	return chain
}

func buildFiltersView(state *appState) fyne.CanvasObject {
	filtersColor := moduleColor("filters")

	// Back button
	backBtn := widget.NewButton("< FILTERS", func() {
		state.showMainMenu()
	})
	backBtn.Importance = widget.LowImportance

	// Queue button
	queueBtn := widget.NewButton("View Queue", func() {
		state.showQueue()
	})
	state.queueBtn = queueBtn
	state.updateQueueButtonLabel()

	clearCompletedBtn := widget.NewButton("⌫", func() {
		state.clearCompletedJobs()
	})
	clearCompletedBtn.Importance = widget.LowImportance

	// Top bar with module color
	topBar := ui.TintedBar(filtersColor, container.NewHBox(backBtn, layout.NewSpacer(), clearCompletedBtn, queueBtn))

	// Instructions
	instructions := widget.NewLabel("Apply filters and color corrections to your video. Preview changes in real-time.")
	instructions.Wrapping = fyne.TextWrapWord
	instructions.Alignment = fyne.TextAlignCenter

	// Initialize state defaults
	if state.filterBrightness == 0 && state.filterContrast == 0 && state.filterSaturation == 0 {
		state.filterBrightness = 0.0 // -1.0 to 1.0
		state.filterContrast = 1.0   // 0.0 to 3.0
		state.filterSaturation = 1.0 // 0.0 to 3.0
		state.filterSharpness = 0.0  // 0.0 to 5.0
		state.filterDenoise = 0.0    // 0.0 to 10.0
	}
	if state.filterInterpPreset == "" {
		state.filterInterpPreset = "Balanced"
	}
	if state.filterInterpFPS == "" {
		state.filterInterpFPS = "60"
	}

	buildFilterChain := func() {
		var chain []string

		// Add basic color correction/enhancement first
		if state.filterBrightness != 0 || state.filterContrast != 1.0 || state.filterSaturation != 1.0 {
			eqFilter := fmt.Sprintf("eq=brightness=%.2f:contrast=%.2f:saturation=%.2f",
				state.filterBrightness, state.filterContrast, state.filterSaturation)
			chain = append(chain, eqFilter)
		}

		if state.filterSharpness != 0.5 {
			sharpenFilter := fmt.Sprintf("unsharp=5:5:%.1f:5:5:%.1f", state.filterSharpness, state.filterSharpness)
			chain = append(chain, sharpenFilter)
		}

		if state.filterDenoise != 0 {
			denoiseFilter := fmt.Sprintf("hqdn3d=%.1f:%.1f:%.1f:%.1f",
				state.filterDenoise, state.filterDenoise, state.filterDenoise, state.filterDenoise)
			chain = append(chain, denoiseFilter)
		}

		if state.filterGrayscale {
			chain = append(chain, "colorchannelmixer=.299:.587:.114:0:.299:.587:.114:0:.299:.587:.114")
		}

		// Add stylistic effects after basic corrections
		if state.filterStylisticMode != "None" && state.filterStylisticMode != "" {
			stylisticChain := buildStylisticFilterChain(state)
			chain = append(chain, stylisticChain...)
		}

		// Add geometric transforms
		if state.filterFlipH || state.filterFlipV {
			var transform string
			if state.filterFlipH && state.filterFlipV {
				transform = "hflip,vflip"
			} else if state.filterFlipH {
				transform = "hflip"
			} else {
				transform = "vflip"
			}
			chain = append(chain, transform)
		}

		if state.filterRotation != 0 {
			rotateFilter := fmt.Sprintf("rotate=%d*PI/180", state.filterRotation)
			chain = append(chain, rotateFilter)
		}

		// Add frame interpolation last
		if state.filterInterpEnabled {
			fps := state.filterInterpFPS
			if fps == "" {
				fps = "60"
			}
			var filter string
			switch state.filterInterpPreset {
			case "Ultra Fast":
				filter = fmt.Sprintf("minterpolate=fps=%s:mi_mode=blend", fps)
			case "Fast":
				filter = fmt.Sprintf("minterpolate=fps=%s:mi_mode=duplicate", fps)
			case "High Quality":
				filter = fmt.Sprintf("minterpolate=fps=%s:mi_mode=mci:mc_mode=aobmc:me_mode=bidir:vsbmc=1:search_param=32", fps)
			case "Maximum Quality":
				filter = fmt.Sprintf("minterpolate=fps=%s:mi_mode=mci:mc_mode=aobmc:me_mode=bidir:vsbmc=1:search_param=64", fps)
			default: // Balanced
				filter = fmt.Sprintf("minterpolate=fps=%s:mi_mode=mci:mc_mode=obmc:me_mode=bidir:me=epzs:search_param=16:vsbmc=0", fps)
			}
			chain = append(chain, filter)
		}

		state.filterActiveChain = chain
	}

	// File label
	fileLabel := widget.NewLabel("No file loaded")
	fileLabel.TextStyle = fyne.TextStyle{Bold: true}

	var videoContainer fyne.CanvasObject
	if state.filtersFile != nil {
		fileLabel.SetText(fmt.Sprintf("File: %s", filepath.Base(state.filtersFile.Path)))
		videoContainer = buildVideoPane(state, fyne.NewSize(480, 270), state.filtersFile, nil)
	} else {
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
					state.filtersFile = src
					state.showFiltersView()
				}, false)
			}()
		}, state.window)
	})
	loadBtn.Importance = widget.HighImportance

	// Navigation to Upscale module
	upscaleNavBtn := widget.NewButton("Send to Upscale →", func() {
		if state.filtersFile != nil {
			state.upscaleFile = state.filtersFile
			buildFilterChain()
			state.upscaleFilterChain = append([]string{}, state.filterActiveChain...)
		}
		state.showUpscaleView()
	})

	// Helper to build boxed sections matching Convert module style
	gridColor := utils.MustHex("#2A3A52")
	navyBlue := utils.MustHex("#191F35")

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

	// Color Correction Section
	brightnessSlider := widget.NewSlider(-1.0, 1.0)
	brightnessSlider.SetValue(state.filterBrightness)
	brightnessSlider.OnChanged = func(f float64) {
		state.filterBrightness = f
		buildFilterChain()
	}

	contrastSlider := widget.NewSlider(0.0, 3.0)
	contrastSlider.SetValue(state.filterContrast)
	contrastSlider.OnChanged = func(f float64) {
		state.filterContrast = f
		buildFilterChain()
	}

	saturationSlider := widget.NewSlider(0.0, 3.0)
	saturationSlider.SetValue(state.filterSaturation)
	saturationSlider.OnChanged = func(f float64) {
		state.filterSaturation = f
		buildFilterChain()
	}

	colorSection := buildFilterBox("Color Correction", container.NewVBox(
		widget.NewLabel("Adjust brightness, contrast, and saturation"),
		container.NewGridWithColumns(2,
			widget.NewLabel("Brightness:"),
			brightnessSlider,
			widget.NewLabel("Contrast:"),
			contrastSlider,
			widget.NewLabel("Saturation:"),
			saturationSlider,
		),
	))

	// Enhancement Section
	sharpnessSlider := widget.NewSlider(0.0, 5.0)
	sharpnessSlider.SetValue(state.filterSharpness)
	sharpnessSlider.OnChanged = func(f float64) {
		state.filterSharpness = f
		buildFilterChain()
	}

	denoiseSlider := widget.NewSlider(0.0, 10.0)
	denoiseSlider.SetValue(state.filterDenoise)
	denoiseSlider.OnChanged = func(f float64) {
		state.filterDenoise = f
		buildFilterChain()
	}

	enhanceSection := buildFilterBox("Enhancement", container.NewVBox(
		widget.NewLabel("Sharpen, blur, and denoise"),
		container.NewGridWithColumns(2,
			widget.NewLabel("Sharpness:"),
			sharpnessSlider,
			widget.NewLabel("Denoise:"),
			denoiseSlider,
		),
	))

	// Transform Section
	rotationSelect := widget.NewSelect([]string{"0°", "90°", "180°", "270°"}, func(s string) {
		switch s {
		case "90°":
			state.filterRotation = 90
		case "180°":
			state.filterRotation = 180
		case "270°":
			state.filterRotation = 270
		default:
			state.filterRotation = 0
		}
		buildFilterChain()
	})

	var rotationStr string
	switch state.filterRotation {
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
		state.filterFlipH = b
		buildFilterChain()
	})
	flipHCheck.SetChecked(state.filterFlipH)

	flipVCheck := widget.NewCheck("", func(b bool) {
		state.filterFlipV = b
		buildFilterChain()
	})
	flipVCheck.SetChecked(state.filterFlipV)

	transformSection := buildFilterBox("Transform", container.NewVBox(
		widget.NewLabel("Rotate and flip video"),
		container.NewGridWithColumns(2,
			widget.NewLabel("Rotation:"),
			rotationSelect,
			widget.NewLabel("Flip Horizontal:"),
			flipHCheck,
			widget.NewLabel("Flip Vertical:"),
			flipVCheck,
		),
	))

	// Creative Effects Section
	grayscaleCheck := widget.NewCheck("Grayscale", func(b bool) {
		state.filterGrayscale = b
		buildFilterChain()
	})
	grayscaleCheck.SetChecked(state.filterGrayscale)

	creativeSection := buildFilterBox("Creative Effects", container.NewVBox(
		widget.NewLabel("Apply artistic effects"),
		grayscaleCheck,
	))

	// Stylistic Effects Section
	stylisticModeSelect := widget.NewSelect([]string{"None", "8mm Film", "16mm Film", "B&W Film", "Silent Film", "70s", "80s", "90s", "VHS", "Webcam"}, func(s string) {
		state.filterStylisticMode = s
		buildFilterChain()
	})
	stylisticModeSelect.SetSelected(state.filterStylisticMode)

	scanlinesCheck := widget.NewCheck("CRT Scanlines", func(b bool) {
		state.filterScanlines = b
		buildFilterChain()
	})
	scanlinesCheck.SetChecked(state.filterScanlines)

	chromaNoiseSlider := widget.NewSlider(0.0, 1.0)
	chromaNoiseSlider.SetValue(state.filterChromaNoise)
	chromaNoiseSlider.OnChanged = func(f float64) {
		state.filterChromaNoise = f
		buildFilterChain()
	}

	colorBleedingCheck := widget.NewCheck("Color Bleeding", func(b bool) {
		state.filterColorBleeding = b
		buildFilterChain()
	})
	colorBleedingCheck.SetChecked(state.filterColorBleeding)

	tapeNoiseSlider := widget.NewSlider(0.0, 1.0)
	tapeNoiseSlider.SetValue(state.filterTapeNoise)
	tapeNoiseSlider.OnChanged = func(f float64) {
		state.filterTapeNoise = f
		buildFilterChain()
	}

	trackingErrorSlider := widget.NewSlider(0.0, 1.0)
	trackingErrorSlider.SetValue(state.filterTrackingError)
	trackingErrorSlider.OnChanged = func(f float64) {
		state.filterTrackingError = f
		buildFilterChain()
	}

	dropoutSlider := widget.NewSlider(0.0, 1.0)
	dropoutSlider.SetValue(state.filterDropout)
	dropoutSlider.OnChanged = func(f float64) {
		state.filterDropout = f
		buildFilterChain()
	}

	interlacingSelect := widget.NewSelect([]string{"None", "Progressive", "Interlaced"}, func(s string) {
		state.filterInterlacing = s
		buildFilterChain()
	})
	interlacingSelect.SetSelected(state.filterInterlacing)

	stylisticSection := buildFilterBox("Stylistic Effects", container.NewVBox(
		widget.NewLabel("Authentic decade-based video effects"),
		container.NewGridWithColumns(2,
			widget.NewLabel("Era Mode:"),
			stylisticModeSelect,
			widget.NewLabel("Interlacing:"),
			interlacingSelect,
		),
		scanlinesCheck,
		widget.NewSeparator(),
		container.NewGridWithColumns(2,
			widget.NewLabel("Chroma Noise:"),
			chromaNoiseSlider,
			widget.NewLabel("Tape Noise:"),
			tapeNoiseSlider,
			widget.NewLabel("Tracking Error:"),
			trackingErrorSlider,
			widget.NewLabel("Tape Dropout:"),
			dropoutSlider,
		),
		colorBleedingCheck,
	))

	// Frame Interpolation Section
	interpEnabledCheck := widget.NewCheck("Enable Frame Interpolation", func(checked bool) {
		state.filterInterpEnabled = checked
		buildFilterChain()
	})
	interpEnabledCheck.SetChecked(state.filterInterpEnabled)

	interpPresetSelect := widget.NewSelect([]string{"Ultra Fast", "Fast", "Balanced", "High Quality", "Maximum Quality"}, func(val string) {
		state.filterInterpPreset = val
		buildFilterChain()
	})
	interpPresetSelect.SetSelected(state.filterInterpPreset)

	interpFPSSelect := widget.NewSelect([]string{"24", "30", "50", "59.94", "60"}, func(val string) {
		state.filterInterpFPS = val
		buildFilterChain()
	})
	interpFPSSelect.SetSelected(state.filterInterpFPS)

	interpHint := widget.NewLabel("Balanced preset is recommended; higher presets are CPU-intensive.")
	interpHint.TextStyle = fyne.TextStyle{Italic: true}
	interpHint.Wrapping = fyne.TextWrapWord

	interpSection := widget.NewCard("Frame Interpolation (Minterpolate)", "", container.NewVBox(
		widget.NewLabel("Generate smoother motion by interpolating new frames"),
		interpEnabledCheck,
		container.NewGridWithColumns(2,
			widget.NewLabel("Preset:"),
			interpPresetSelect,
			widget.NewLabel("Target FPS:"),
			interpFPSSelect,
		),
		interpHint,
	))
	buildFilterChain()

	// Apply button
	applyBtn := widget.NewButton("Apply Filters", func() {
		if state.filtersFile == nil {
			dialog.ShowInformation("No Video", "Please load a video first.", state.window)
			return
		}
		buildFilterChain()
		dialog.ShowInformation("Filters", "Filters are now configured and will be applied when sent to Upscale.", state.window)
	})
	applyBtn.Importance = widget.HighImportance

	// Main content
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
	// Adaptive height for small screens - allow content to flow
	// settingsScroll.SetMinSize(fyne.NewSize(350, 400)) // Removed for flexible sizing

	mainContent := container.New(&fixedHSplitLayout{ratio: 0.6},
		container.NewVBox(leftPanel, container.NewCenter(videoContainer)),
		settingsScroll,
	)

	content := container.NewPadded(mainContent)

	bottomBar := moduleFooter(filtersColor, container.NewHBox(layout.NewSpacer(), applyBtn), state.statsBar)
	return container.NewBorder(topBar, bottomBar, nil, nil, content)
}
