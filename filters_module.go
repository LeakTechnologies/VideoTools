package main

import (
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
		FiltersFile: state.filtersFile,
		FiltersFilePath: func() string {
			if state.filtersFile != nil {
				return state.filtersFile.Path
			}
			return ""
		}(),
		FilterActiveChain: state.filterActiveChain,
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
		OnProbeVideo: func(path string) (interface{}, error) {
			result, err := probeVideo(path)
			if err != nil {
				return nil, err
			}
			return result, nil
		},
		OnBuildVideoPane: func(st interface{}, size fyne.Size, src interface{}, overlay fyne.CanvasObject) fyne.CanvasObject {
			if vs, ok := src.(*videoSource); ok {
				return buildVideoPane(state, size, vs, nil)
			}
			return buildVideoPane(state, size, nil, nil)
		},
		OnHasNativeMediaPlayer: HasNativeMediaPlayer,
		OnLoadVideoNative:      state.loadVideoNative,
		OnBuildFilterChain: func() []string {
			return filters.BuildStylisticFilterChain(filters.FilterChainParams{
				StylisticMode: state.filterStylisticMode,
				Scanlines:     state.filterScanlines,
				ChromaNoise:   state.filterChromaNoise,
				ColorBleeding: state.filterColorBleeding,
				TapeNoise:     state.filterTapeNoise,
				TrackingError: state.filterTrackingError,
				Dropout:       state.filterDropout,
				Interlacing:   state.filterInterlacing,
			})
		},
	}
	return filters.BuildView(opts)
}
