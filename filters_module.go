package main

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"

	"github.com/LeakTechnologies/VideoTools/internal/app/modules/filters"
	"github.com/LeakTechnologies/VideoTools/internal/logging"
	"github.com/LeakTechnologies/VideoTools/internal/queue"
	"github.com/LeakTechnologies/VideoTools/internal/ui"
	"github.com/LeakTechnologies/VideoTools/internal/utils"
)

func (s *appState) showFiltersView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "filters"
	s.maximizeWindow()
	s.setContent(ui.NewDroppable(buildFiltersView(s), func(items []fyne.URI) {
		s.handleDrop(fyne.NewPos(0, 0), items)
	}))
}

func buildFiltersView(state *appState) fyne.CanvasObject {
	opts := filters.Options{
		Window:              state.window,
		ModuleColor:         moduleColor("filters"),
		FilterBrightness:    state.filterBrightness,
		FilterContrast:      state.filterContrast,
		FilterSaturation:    state.filterSaturation,
		FilterSharpness:     state.filterSharpness,
		FilterDenoise:       state.filterDenoise,
		FilterGrayscale:     state.filterGrayscale,
		FilterFlipH:         state.filterFlipH,
		FilterFlipV:         state.filterFlipV,
		FilterRotation:      state.filterRotation,
		FilterStylisticMode: state.filterStylisticMode,
		FilterScanlines:     state.filterScanlines,
		FilterChromaNoise:   state.filterChromaNoise,
		FilterColorBleeding: state.filterColorBleeding,
		FilterTapeNoise:     state.filterTapeNoise,
		FilterTrackingError: state.filterTrackingError,
		FilterDropout:       state.filterDropout,
		FilterInterlacing:   state.filterInterlacing,
		FilterInterpEnabled: state.filterInterpEnabled,
		FilterInterpPreset:  state.filterInterpPreset,
		FilterInterpFPS:     state.filterInterpFPS,
		FiltersFile:         state.filtersFile,
		FiltersFilePath: func() string {
			if state.filtersFile != nil {
				return state.filtersFile.Path
			}
			return ""
		}(),
		FilterActiveChain:    state.filterActiveChain,
		HardwareAccel:        func() string { return state.convert.HardwareAccel },
		SetHardwareAccel:      func(s string) { state.convert.HardwareAccel = s },
		OnShowMainMenu:       func() { state.showMainMenu() },
		OnShowQueue:          func() { state.showQueue() },
		OnShowUpscaleView:    func() { state.showUpscaleView() },
		OnShowFiltersView:    func() { state.showFiltersView() },
		OnClearCompletedJobs: func() { state.clearCompletedJobs() },
		OnGetStatsBar:        func() *ui.ConversionStatsBar { return state.statsBar },
		OnGetModuleFooter: func(col color.Color, actions fyne.CanvasObject, stats *ui.ConversionStatsBar) fyne.CanvasObject {
			return moduleFooter(col, actions, stats)
		},
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
		OnAddToQueue: func() {
			if state.filtersFile == nil {
				return
			}
			path := ""
			if state.filtersFile != nil {
				path = state.filtersFile.Path
			}
			job := &queue.Job{
				Type:        queue.JobTypeFilter,
				Status:      queue.JobStatusPending,
				Title:       "Filter",
				InputFile:   path,
				OutputFile:  "",
				Config:      state.filterJobConfig(),
				Description: "Apply filters to video",
			}
			state.pipelineAdd(job)
		},
		OnApplyFilters: func() {},
		OnFilterNow: func() {
			if state.filtersFile == nil {
				return
			}
			path := state.filtersFile.Path
			dir := filepath.Dir(path)
			name := filepath.Base(path)
			ext := filepath.Ext(name)
			base := name[:len(name)-len(ext)]
			outputPath := filepath.Join(dir, fmt.Sprintf("%s_filtered%s", base, ext))

			cfg := state.filterJobConfig()
			cfg["outputPath"] = outputPath

			state.filterBusy = true
			state.filterActiveIn = path
			state.filterActiveOut = outputPath
			state.filterProgress = 0
			state.filterFPS = 0
			state.filterSpeed = 0
			state.filterETA = 0

			go func() {
				ctx, cancel := context.WithCancel(context.Background())
				state.filterCancel = cancel

				err := state.executeFilterJob(ctx, &queue.Job{
					Type:   queue.JobTypeFilter,
					Config: cfg,
				}, func(p float64) {
					state.filterProgress = p
					if state.statsBar != nil {
						state.applyFilterStatusToUI()
					}
				})

				state.filterBusy = false
				state.filterCancel = nil
				if state.statsBar != nil {
					state.applyFilterStatusToUI()
				}
				if err != nil {
					dialog.ShowError(fmt.Errorf("filter failed: %w", err), state.window)
				} else {
					dialog.ShowInformation("Filter Complete", fmt.Sprintf("Output saved to:\n%s", outputPath), state.window)
				}
			}()
		},
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
		OnLoadVideoNative:      state.loadFiltersVideo,
		BuildOriginalPlayerPane: func() fyne.CanvasObject {
			if !HasNativeMediaPlayer() {
				return nil
			}
			w := GetFiltersPlayer().Widget()
			if w == nil {
				return nil
			}
			return ui.BuildPlayerContainer(w, fyne.NewSize(0, 160))
		},
		BuildPreviewPlayerPane: func() fyne.CanvasObject {
			if !HasNativeMediaPlayer() {
				return nil
			}
			w := GetFiltersPreviewPlayer().Widget()
			if w == nil {
				return nil
			}
			return ui.BuildPlayerContainer(w, fyne.NewSize(0, 160))
		},
		BuildMetadataPane: func(onToggle func(bool)) fyne.CanvasObject {
			panel, _ := buildMetadataPanel(state, state.filtersFile, fyne.NewSize(0, 200), moduleColor("filters"), onToggle)
			return panel
		},
		OnFilterChanged: func() { state.applyFiltersPreview() },
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

func (s *appState) filterJobConfig() map[string]interface{} {
	cfg := make(map[string]interface{})

	// Input/output paths
	if s.filtersFile != nil {
		cfg["inputPath"] = s.filtersFile.Path
	}

	// Color correction
	cfg["brightness"] = s.filterBrightness
	cfg["contrast"] = s.filterContrast
	cfg["saturation"] = s.filterSaturation

	// Enhancement
	cfg["sharpness"] = s.filterSharpness
	cfg["denoise"] = s.filterDenoise
	cfg["grayscale"] = s.filterGrayscale

	// Transform
	cfg["flipH"] = s.filterFlipH
	cfg["flipV"] = s.filterFlipV
	cfg["rotation"] = s.filterRotation

	// Stylistic
	cfg["stylisticMode"] = s.filterStylisticMode
	cfg["scanlines"] = s.filterScanlines
	cfg["chromaNoise"] = s.filterChromaNoise
	cfg["colorBleeding"] = s.filterColorBleeding
	cfg["tapeNoise"] = s.filterTapeNoise
	cfg["trackingError"] = s.filterTrackingError
	cfg["dropout"] = s.filterDropout
	cfg["interlacing"] = s.filterInterlacing

	// Frame interpolation
	cfg["interpEnabled"] = s.filterInterpEnabled
	cfg["interpPreset"] = s.filterInterpPreset
	cfg["interpFPS"] = s.filterInterpFPS

	// Active filter chain
	if len(s.filterActiveChain) > 0 {
		cfg["filterChain"] = s.filterActiveChain
	}

	return cfg
}

func (s *appState) executeFilterJob(ctx context.Context, job *queue.Job, progress func(float64)) error {
	cfg := job.Config
	inputPath, _ := cfg["inputPath"].(string)
	if inputPath == "" {
		return fmt.Errorf("no input file")
	}

	outputPath, _ := cfg["outputPath"].(string)
	if outputPath == "" {
		// Generate output path from input path
		dir := filepath.Dir(inputPath)
		name := filepath.Base(inputPath)
		ext := filepath.Ext(name)
		base := name[:len(name)-len(ext)]
		outputPath = filepath.Join(dir, fmt.Sprintf("%s_filtered%s", base, ext))
	}

	// Ensure output directory exists
	if outputDir := filepath.Dir(outputPath); outputDir != "" {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// Build filter chain from job config
	var chain []string

	// Color correction (brightness/contrast/saturation use eq filter)
	brightness, _ := cfg["brightness"].(float64)
	contrast, _ := cfg["contrast"].(float64)
	saturation, _ := cfg["saturation"].(float64)
	if brightness != 0 || contrast != 1.0 || saturation != 1.0 {
		chain = append(chain, fmt.Sprintf("eq=brightness=%.2f:contrast=%.2f:saturation=%.2f", brightness, contrast, saturation))
	}

	// Sharpness
	sharpness, _ := cfg["sharpness"].(float64)
	if sharpness > 0 {
		chain = append(chain, fmt.Sprintf("unsharp=5:5:%.2f:5:5:0.0", sharpness/5))
	}

	// Denoise
	denoise, _ := cfg["denoise"].(float64)
	if denoise > 0 {
		chain = append(chain, fmt.Sprintf("hqdn3d=%.2f", denoise))
	}

	// Grayscale
	if grayscale, _ := cfg["grayscale"].(bool); grayscale {
		chain = append(chain, "hue=s=0")
	}

	// Flip
	if flipH, _ := cfg["flipH"].(bool); flipH {
		chain = append(chain, "hflip")
	}
	if flipV, _ := cfg["flipV"].(bool); flipV {
		chain = append(chain, "vflip")
	}

	// Rotation
	if rotation, _ := cfg["rotation"].(int); rotation > 0 {
		switch rotation {
		case 90:
			chain = append(chain, "transpose=1")
		case 180:
			chain = append(chain, "transpose=1,transpose=1")
		case 270:
			chain = append(chain, "transpose=2")
		}
	}

	// Stylistic filters
	stylisticMode, _ := cfg["stylisticMode"].(string)
	if stylisticMode != "" && stylisticMode != "None" {
		stylisticParams := filters.FilterChainParams{
			StylisticMode: stylisticMode,
		}
		if v, ok := cfg["scanlines"].(bool); ok {
			stylisticParams.Scanlines = v
		}
		if v, ok := cfg["chromaNoise"].(float64); ok {
			stylisticParams.ChromaNoise = v
		}
		if v, ok := cfg["colorBleeding"].(bool); ok {
			stylisticParams.ColorBleeding = v
		}
		if v, ok := cfg["tapeNoise"].(float64); ok {
			stylisticParams.TapeNoise = v
		}
		if v, ok := cfg["trackingError"].(float64); ok {
			stylisticParams.TrackingError = v
		}
		if v, ok := cfg["dropout"].(float64); ok {
			stylisticParams.Dropout = v
		}
		if v, ok := cfg["interlacing"].(string); ok {
			stylisticParams.Interlacing = v
		}
		stylisticChain := filters.BuildStylisticFilterChain(stylisticParams)
		chain = append(chain, stylisticChain...)
	}

	if len(chain) == 0 {
		return fmt.Errorf("no filters configured")
	}

	// Combine filter chain
	filterStr := strings.Join(chain, ",")

	logging.Info(logging.CatFilters, "Executing filter job: %s -> %s", inputPath, outputPath)
	logging.Debug(logging.CatFFMPEG, "Filter chain: %s", filterStr)

	// Build ffmpeg arguments
	args := []string{
		"-y", "-hide_banner", "-loglevel", "error",
		"-i", inputPath,
		"-vf", filterStr,
		"-c:a", "copy",
		"-progress", "pipe:1", "-nostats",
		outputPath,
	}

	// Probe source for duration
	src, err := probeVideo(inputPath)
	if err != nil {
		logging.Error(logging.CatFilters, "filter probe failed: input=%s err=%v", inputPath, err)
		return fmt.Errorf("failed to probe input: %w", err)
	}
	totalDur := src.Duration
	if totalDur <= 0 {
		totalDur = 1.0
	}

	ffmpeg := utils.GetFFmpegPath()
	if err := runFFmpegWithProgress(ctx, ffmpeg, args, totalDur, progress); err != nil {
		logging.Error(logging.CatFilters, "filter encode failed: input=%s output=%s err=%v", inputPath, outputPath, err)
		return fmt.Errorf("filter encode failed: %w", err)
	}

	return nil
}

func (s *appState) applyFilterStatusToUI() {
	if s.statsBar == nil {
		return
	}
	if s.filterBusy {
		eta := ""
		if s.filterETA > 0 {
			eta = s.filterETA.Round(time.Second).String()
		}
		elapsed := ""
		remaining := ""
		if s.filterProgress > 0 && s.filterProgress < 100 {
			// Calculate remaining time from progress
			remaining = fmt.Sprintf("Remaining: %s", s.filterETA.Round(time.Second))
		}
		s.statsBar.UpdateStatsWithDetails(1, 0, 0, 0, 0, s.filterProgress, s.filterFPS, s.filterSpeed, eta, elapsed, remaining, "Filter: "+filepath.Base(s.filterActiveIn))
	} else {
		s.statsBar.UpdateStats(0, 0, 0, 0, 0, 0, "")
	}
}
