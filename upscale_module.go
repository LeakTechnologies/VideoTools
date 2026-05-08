package main

import (
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"git.leaktechnologies.dev/stu/VideoTools/internal/app/modules/upscale"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"image/color"
)

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
	content := upscale.BuildView(s.upscaleOptions())
	s.setContent(ui.NewDroppable(content, func(items []fyne.URI) {
		s.handleDrop(fyne.NewPos(0, 0), items)
	}))
}

func (s *appState) loadUpscaleVideo(path string) {
	defer func() {
		if r := recover(); r != nil {
			logging.Error(logging.CatPlayer, "panic in loadUpscaleVideo: %v", r)
		}
	}()
	if err := GetUpscalePlayer().Load(path); err != nil {
		logging.Error(logging.CatPlayer, "loadUpscaleVideo failed: path=%s err=%v", path, err)
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			ui.ShowToast(s.window, "Native player could not open this file.", ui.ToastWarning)
		}, false)
		return
	}
	// Load preview player and apply current filter pipeline for live before/after feedback.
	go s.loadUpscalePreviewVideo(path)
}

// mainToUpscaleVideoSource converts a main-package videoSource to the upscale package's
// exported VideoSource type, which is the type expected by upscale.BuildView callbacks.
func mainToUpscaleVideoSource(v *videoSource) *upscale.VideoSource {
	if v == nil {
		return nil
	}
	return &upscale.VideoSource{
		Path:              v.Path,
		Format:            v.Format,
		VideoCodec:        v.VideoCodec,
		Width:             v.Width,
		Height:            v.Height,
		FrameRate:         v.FrameRate,
		Bitrate:           v.Bitrate,
		PixelFormat:       v.PixelFormat,
		ColorSpace:        v.ColorSpace,
		ColorRange:        v.ColorRange,
		FieldOrder:        v.FieldOrder,
		GOPSize:           v.GOPSize,
		AudioCodec:        v.AudioCodec,
		AudioBitrate:      v.AudioBitrate,
		AudioRate:         v.AudioRate,
		Channels:          v.Channels,
		SampleAspectRatio: v.SampleAspectRatio,
		HasChapters:       v.HasChapters,
		HasMetadata:       v.HasMetadata,
		Duration:          v.Duration,
	}
}

// upscaleToMainVideoSource converts an upscale.VideoSource back to the main-package videoSource.
func upscaleToMainVideoSource(v *upscale.VideoSource) *videoSource {
	if v == nil {
		return nil
	}
	return &videoSource{
		Path:              v.Path,
		DisplayName:       filepath.Base(v.Path),
		Format:            v.Format,
		VideoCodec:        v.VideoCodec,
		Width:             v.Width,
		Height:            v.Height,
		FrameRate:         v.FrameRate,
		Bitrate:           v.Bitrate,
		PixelFormat:       v.PixelFormat,
		ColorSpace:        v.ColorSpace,
		ColorRange:        v.ColorRange,
		FieldOrder:        v.FieldOrder,
		GOPSize:           v.GOPSize,
		AudioCodec:        v.AudioCodec,
		AudioBitrate:      v.AudioBitrate,
		AudioRate:         v.AudioRate,
		Channels:          v.Channels,
		SampleAspectRatio: v.SampleAspectRatio,
		HasChapters:       v.HasChapters,
		HasMetadata:       v.HasMetadata,
		Duration:          v.Duration,
	}
}

func (s *appState) upscaleOptions() upscale.Options {
	return upscale.Options{
		Window:      s.window,
		ModuleColor: moduleColor("upscale"),

		UpscaleFile: mainToUpscaleVideoSource(s.upscaleFile),
		QueueBtn:    s.queueBtn,

		OnShowMainMenu:           s.showMainMenu,
		OnRefreshView:            s.showUpscaleView,
		OnShowQueue:              s.showQueue,
		OnShowFiltersView:        s.showFiltersView,
		OnUpdateQueueButtonLabel: s.updateQueueButtonLabel,
		OnGetStatsBar: func() *ui.ConversionStatsBar {
			return s.statsBar
		},
		OnGetModuleFooter: func(col color.Color, actions fyne.CanvasObject, stats *ui.ConversionStatsBar) fyne.CanvasObject {
			return moduleFooter(col, actions, stats)
		},
		OnGetGridColor: func() color.Color {
			return gridColor
		},
		OnProbeVideo: func(path string) (interface{}, error) {
			result, err := probeVideo(path)
			if err != nil {
				return nil, err
			}
			return mainToUpscaleVideoSource(result), nil
		},
		OnBuildVideoPane: func(state interface{}, size fyne.Size, src interface{}, overlay fyne.CanvasObject) fyne.CanvasObject {
			if vs, ok := src.(*upscale.VideoSource); ok {
				return buildVideoPane(s, size, upscaleToMainVideoSource(vs), nil)
			}
			return buildVideoPane(s, size, nil, nil)
		},
		OnHasNativeMediaPlayer: HasNativeMediaPlayer,
		OnLoadVideoNative:      s.loadUpscaleVideo,
		OnGetFilterActiveChain: func() []string {
			return s.filterActiveChain
		},
		BuildOriginalPlayerPane: func() fyne.CanvasObject {
			if !HasNativeMediaPlayer() {
				return nil
			}
			w := GetUpscalePlayer().Widget()
			if w == nil {
				return nil
			}
			return ui.BuildPlayerContainer(w, fyne.NewSize(0, 0))
		},
		BuildPreviewPlayerPane: func() fyne.CanvasObject {
			if !HasNativeMediaPlayer() {
				return nil
			}
			w := GetUpscalePreviewPlayer().Widget()
			if w == nil {
				return nil
			}
			return ui.BuildPlayerContainer(w, fyne.NewSize(0, 0))
		},
		OnFilterChanged: func() { s.applyUpscalePreview() },
		OnDualPlayerSeek: func(seconds float64) {
			s.renderDualPlayerPreview(seconds, 5*time.Second)
		},
		OnDualPlayerRender: s.renderDualPlayerPreview,

		UpscaleMethod:              func() string { return s.upscaleMethod },
		UpscaleTargetRes:           func() string { return s.upscaleTargetRes },
		UpscaleCustomWidth:         func() int { return s.upscaleCustomWidth },
		UpscaleCustomHeight:        func() int { return s.upscaleCustomHeight },
		UpscaleQualityPreset:       func() string { return s.upscaleQualityPreset },
		UpscaleAIEnabled:           func() bool { return s.upscaleAIEnabled },
		UpscaleAIModel:             func() string { return s.upscaleAIModel },
		UpscaleAIAvailable:         func() bool { return s.upscaleAIAvailable },
		UpscaleAIBackend:           func() string { return s.upscaleAIBackend },
		UpscaleAIPreset:            func() string { return s.upscaleAIPreset },
		UpscaleAIScale:             func() float64 { return s.upscaleAIScale },
		UpscaleAIScaleUseTarget:    func() bool { return s.upscaleAIScaleUseTarget },
		UpscaleAIOutputAdjust:      func() float64 { return s.upscaleAIOutputAdjust },
		UpscaleAIFaceEnhance:       func() bool { return s.upscaleAIFaceEnhance },
		UpscaleAIDenoise:           func() float64 { return s.upscaleAIDenoise },
		UpscaleAITile:              func() int { return s.upscaleAITile },
		UpscaleAIGPU:               func() int { return s.upscaleAIGPU },
		UpscaleAIGPUAuto:           func() bool { return s.upscaleAIGPUAuto },
		UpscaleAIThreadsLoad:       func() int { return s.upscaleAIThreadsLoad },
		UpscaleAIThreadsProc:       func() int { return s.upscaleAIThreadsProc },
		UpscaleAIThreadsSave:       func() int { return s.upscaleAIThreadsSave },
		UpscaleAITTA:               func() bool { return s.upscaleAITTA },
		UpscaleAIOutputFormat:      func() string { return s.upscaleAIOutputFormat },
		UpscaleApplyFilters:        func() bool { return s.upscaleApplyFilters },
		UpscaleFilterChain:         func() []string { return s.upscaleFilterChain },
		UpscaleFrameRate:           func() string { return s.upscaleFrameRate },
		UpscaleMotionInterpolation: func() bool { return s.upscaleMotionInterpolation },
		UpscaleBlurEnabled:         func() bool { return s.upscaleBlurEnabled },
		UpscaleBlurSigma:           func() float64 { return s.upscaleBlurSigma },
		UpscaleEncoderPreset:       func() string { return s.upscaleEncoderPreset },
		UpscaleVideoCodec:          func() string { return s.upscaleVideoCodec },
		UpscaleBitrateMode:         func() string { return s.upscaleBitrateMode },
		UpscaleBitratePreset:       func() string { return s.upscaleBitratePreset },
		UpscaleManualBitrate:       func() string { return s.upscaleManualBitrate },
		UpscaleRIFEBackend:         func() string { return s.upscaleRIFEBackend },
		UpscaleRIFEAvailable:       func() bool { return s.upscaleRIFEAvailable },
		UpscaleRIFEEnabled:         func() bool { return s.upscaleRIFEEnabled },
		UpscaleRIFEMultiplier:      func() int { return s.upscaleRIFEMultiplier },
		UpscaleRIFEModel:           func() string { return s.upscaleRIFEModel },
		UpscaleRealCUGANAvailable:  func() bool { return s.upscaleRealCUGANAvailable },
		UpscaleHardwareAccel:       func() string { return s.upscaleHardwareAccel },
		UpscaleOutputContainer:     func() string { return s.upscaleOutputContainer },
		UpscaleManualCRF:           func() int { return s.upscaleManualCRF },
		UpscalePixelFormat:         func() string { return s.upscalePixelFormat },
		UpscaleSrcColorSpace:       func() string { return s.upscaleSrcColorSpace },
		UpscaleColorDepth:          func() string { return s.upscaleColorDepth },
		UpscaleSkinTone:            func() string { return s.upscaleSkinTone },

		SetUpscaleFile: func(f interface{}) {
			if vs, ok := f.(*upscale.VideoSource); ok {
				s.upscaleFile = upscaleToMainVideoSource(vs)
			}
		},
		SetUpscaleMethod:              func(v string) { s.upscaleMethod = v },
		SetUpscaleTargetRes:           func(v string) { s.upscaleTargetRes = v },
		SetUpscaleCustomWidth:         func(v int) { s.upscaleCustomWidth = v },
		SetUpscaleCustomHeight:        func(v int) { s.upscaleCustomHeight = v },
		SetUpscaleQualityPreset:       func(v string) { s.upscaleQualityPreset = v },
		SetUpscaleAIEnabled:           func(v bool) { s.upscaleAIEnabled = v },
		SetUpscaleAIModel:             func(v string) { s.upscaleAIModel = v },
		SetUpscaleAIAvailable:         func(v bool) { s.upscaleAIAvailable = v },
		SetUpscaleAIBackend:           func(v string) { s.upscaleAIBackend = v },
		SetUpscaleAIPreset:            func(v string) { s.upscaleAIPreset = v },
		SetUpscaleAIScale:             func(v float64) { s.upscaleAIScale = v },
		SetUpscaleAIScaleUseTarget:    func(v bool) { s.upscaleAIScaleUseTarget = v },
		SetUpscaleAIOutputAdjust:      func(v float64) { s.upscaleAIOutputAdjust = v },
		SetUpscaleAIFaceEnhance:       func(v bool) { s.upscaleAIFaceEnhance = v },
		SetUpscaleAIDenoise:           func(v float64) { s.upscaleAIDenoise = v },
		SetUpscaleAITile:              func(v int) { s.upscaleAITile = v },
		SetUpscaleAIGPU:               func(v int) { s.upscaleAIGPU = v },
		SetUpscaleAIGPUAuto:           func(v bool) { s.upscaleAIGPUAuto = v },
		SetUpscaleAIThreadsLoad:       func(v int) { s.upscaleAIThreadsLoad = v },
		SetUpscaleAIThreadsProc:       func(v int) { s.upscaleAIThreadsProc = v },
		SetUpscaleAIThreadsSave:       func(v int) { s.upscaleAIThreadsSave = v },
		SetUpscaleAITTA:               func(v bool) { s.upscaleAITTA = v },
		SetUpscaleAIOutputFormat:      func(v string) { s.upscaleAIOutputFormat = v },
		SetUpscaleApplyFilters:        func(v bool) { s.upscaleApplyFilters = v },
		SetUpscaleFilterChain:         func(chain []string) { s.upscaleFilterChain = chain },
		SetUpscaleFrameRate:           func(v string) { s.upscaleFrameRate = v },
		SetUpscaleMotionInterpolation: func(v bool) { s.upscaleMotionInterpolation = v },
		SetUpscaleBlurEnabled:         func(v bool) { s.upscaleBlurEnabled = v },
		SetUpscaleBlurSigma:           func(v float64) { s.upscaleBlurSigma = v },
		SetUpscaleEncoderPreset:       func(v string) { s.upscaleEncoderPreset = v },
		SetUpscaleVideoCodec:          func(v string) { s.upscaleVideoCodec = v },
		SetUpscaleBitrateMode:         func(v string) { s.upscaleBitrateMode = v },
		SetUpscaleBitratePreset:       func(v string) { s.upscaleBitratePreset = v },
		SetUpscaleManualBitrate:       func(v string) { s.upscaleManualBitrate = v },
		SetUpscaleRIFEBackend:         func(v string) { s.upscaleRIFEBackend = v },
		SetUpscaleRIFEAvailable:       func(v bool) { s.upscaleRIFEAvailable = v },
		SetUpscaleRIFEEnabled:         func(v bool) { s.upscaleRIFEEnabled = v },
		SetUpscaleRIFEMultiplier:      func(v int) { s.upscaleRIFEMultiplier = v },
		SetUpscaleRIFEModel:           func(v string) { s.upscaleRIFEModel = v },
		SetUpscaleRealCUGANAvailable:  func(v bool) { s.upscaleRealCUGANAvailable = v },
		SetUpscaleHardwareAccel:       func(v string) { s.upscaleHardwareAccel = v },
		SetUpscaleOutputContainer:     func(v string) { s.upscaleOutputContainer = v },
		SetUpscaleManualCRF:           func(v int) { s.upscaleManualCRF = v },
		SetUpscalePixelFormat:         func(v string) { s.upscalePixelFormat = v },
		SetUpscaleSrcColorSpace:       func(v string) { s.upscaleSrcColorSpace = v },
		SetUpscaleColorDepth:          func(v string) { s.upscaleColorDepth = v },
		SetUpscaleSkinTone:            func(v string) { s.upscaleSkinTone = v },

		FiltersFile: func() interface{} { return s.filtersFile },
		SetFiltersFile: func(f interface{}) {
			if vs, ok := f.(*upscale.VideoSource); ok {
				s.filtersFile = upscaleToMainVideoSource(vs)
			}
		},

		// Integrated filter controls
		FilterBrightness: func() float64 { return s.filterBrightness },
		FilterContrast:   func() float64 { return s.filterContrast },
		FilterSaturation: func() float64 { return s.filterSaturation },
		FilterSharpness:  func() float64 { return s.filterSharpness },
		FilterDenoise:    func() float64 { return s.filterDenoise },
		FilterGrayscale:  func() bool { return s.filterGrayscale },
		FilterFlipH:      func() bool { return s.filterFlipH },
		FilterFlipV:      func() bool { return s.filterFlipV },
		FilterRotation:   func() int { return s.filterRotation },

		SetFilterBrightness: func(f float64) { s.filterBrightness = f },
		SetFilterContrast:   func(f float64) { s.filterContrast = f },
		SetFilterSaturation: func(f float64) { s.filterSaturation = f },
		SetFilterSharpness:  func(f float64) { s.filterSharpness = f },
		SetFilterDenoise:    func(f float64) { s.filterDenoise = f },
		SetFilterGrayscale:  func(b bool) { s.filterGrayscale = b },
		SetFilterFlipH:      func(b bool) { s.filterFlipH = b },
		SetFilterFlipV:      func(b bool) { s.filterFlipV = b },
		SetFilterRotation:   func(i int) { s.filterRotation = i },

		JobQueue: func() *queue.Queue { return s.jobQueue },
		AddJob:   func(job *queue.Job) { s.jobQueue.Add(job) },
	}
}
