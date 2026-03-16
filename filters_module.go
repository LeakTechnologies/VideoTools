package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"

	"git.leaktechnologies.dev/stu/VideoTools/internal/app/modules/filters"
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
	opts := filters.Options{
		Window:               state.window,
		ModuleColor:          moduleColor("filters"),
		FilterBrightness:     state.filterBrightness,
		FilterContrast:       state.filterContrast,
		FilterSaturation:     state.filterSaturation,
		FilterSharpness:      state.filterSharpness,
		FilterDenoise:        state.filterDenoise,
		FilterGrayscale:      state.filterGrayscale,
		FilterFlipH:          state.filterFlipH,
		FilterFlipV:          state.filterFlipV,
		FilterRotation:       state.filterRotation,
		FilterStylisticMode:  state.filterStylisticMode,
		FilterScanlines:      state.filterScanlines,
		FilterChromaNoise:    state.filterChromaNoise,
		FilterColorBleeding:  state.filterColorBleeding,
		FilterTapeNoise:      state.filterTapeNoise,
		FilterTrackingError:  state.filterTrackingError,
		FilterDropout:        state.filterDropout,
		FilterInterlacing:    state.filterInterlacing,
		FilterInterpEnabled:  state.filterInterpEnabled,
		FilterInterpPreset:   state.filterInterpPreset,
		FilterInterpFPS:      state.filterInterpFPS,
		FiltersFile:          state.filtersFile,
		FilterActiveChain:    state.filterActiveChain,
		OnShowMainMenu:       func() { state.showMainMenu() },
		OnShowQueue:          func() { state.showQueue() },
		OnShowUpscaleView:    func() { state.showUpscaleView() },
		OnShowFiltersView:    func() { state.showFiltersView() },
		OnClearCompletedJobs: func() { state.clearCompletedJobs() },
		OnGetStatsBar:        func() fyne.CanvasObject { return state.statsBar },
		OnLoadFile: func(path string) {
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
		},
		OnSendToUpscale: func() {
			if state.filtersFile != nil {
				state.upscaleFile = state.filtersFile
				state.upscaleFilterChain = append([]string{}, state.filterActiveChain...)
			}
		},
		OnApplyFilters:     func() {},
		OnPersistConfig:    func() {},
		OnSetBrightness:    func(f float64) { state.filterBrightness = f },
		OnSetContrast:      func(f float64) { state.filterContrast = f },
		OnSetSaturation:    func(f float64) { state.filterSaturation = f },
		OnSetSharpness:     func(f float64) { state.filterSharpness = f },
		OnSetDenoise:       func(f float64) { state.filterDenoise = f },
		OnSetGrayscale:     func(b bool) { state.filterGrayscale = b },
		OnSetFlipH:         func(b bool) { state.filterFlipH = b },
		OnSetFlipV:         func(b bool) { state.filterFlipV = b },
		OnSetRotation:      func(i int) { state.filterRotation = i },
		OnSetStylisticMode: func(s string) { state.filterStylisticMode = s },
		OnSetScanlines:     func(b bool) { state.filterScanlines = b },
		OnSetChromaNoise:   func(f float64) { state.filterChromaNoise = f },
		OnSetColorBleeding: func(b bool) { state.filterColorBleeding = b },
		OnSetTapeNoise:     func(f float64) { state.filterTapeNoise = f },
		OnSetTrackingError: func(f float64) { state.filterTrackingError = f },
		OnSetDropout:       func(f float64) { state.filterDropout = f },
		OnSetInterlacing:   func(s string) { state.filterInterlacing = s },
		OnSetInterpEnabled: func(b bool) { state.filterInterpEnabled = b },
		OnSetInterpPreset:  func(s string) { state.filterInterpPreset = s },
		OnSetInterpFPS:     func(s string) { state.filterInterpFPS = s },
		OnBuildFilterChain: func() []string { return buildStylisticFilterChain(state) },
	}
	return filters.BuildView(opts)
}
