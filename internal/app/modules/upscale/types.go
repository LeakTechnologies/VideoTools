package upscale

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"image/color"
)

type Options struct {
	Window      fyne.Window
	ModuleColor color.Color

	UpscaleFile interface{}
	QueueBtn    *widget.Button

	OnShowMainMenu           func()
	OnRefreshView            func()
	OnShowQueue              func()
	OnShowFiltersView        func()
	OnUpdateQueueButtonLabel func()
	OnGetStatsBar            func() *ui.ConversionStatsBar
	OnGetModuleFooter        func(color color.Color, actions fyne.CanvasObject, stats *ui.ConversionStatsBar) fyne.CanvasObject
	OnGetGridColor           func() color.Color
	OnProbeVideo             func(path string) (interface{}, error)
	OnBuildVideoPane         func(state interface{}, size fyne.Size, src interface{}, overlay fyne.CanvasObject) fyne.CanvasObject
	OnHasNativeMediaPlayer   func() bool
	OnLoadVideoNative        func(path string)
	OnGetFilterActiveChain   func() []string

	// Dual player callbacks for split-view preview
	OnDualPlayerSeek   func(seconds float64) // triggered when source seekbar moves
	OnDualPlayerRender func(seconds float64, duration time.Duration) // render segment

	UpscaleMethod              func() string
	UpscaleTargetRes           func() string
	UpscaleCustomWidth         func() int
	UpscaleCustomHeight        func() int
	UpscaleQualityPreset       func() string
	UpscaleAIEnabled           func() bool
	UpscaleAIModel             func() string
	UpscaleAIAvailable         func() bool
	UpscaleAIBackend           func() string
	UpscaleAIPreset            func() string
	UpscaleAIScale             func() float64
	UpscaleAIScaleUseTarget    func() bool
	UpscaleAIOutputAdjust      func() float64
	UpscaleAIFaceEnhance       func() bool
	UpscaleAIDenoise           func() float64
	UpscaleAITile              func() int
	UpscaleAIGPU               func() int
	UpscaleAIGPUAuto           func() bool
	UpscaleAIThreadsLoad       func() int
	UpscaleAIThreadsProc       func() int
	UpscaleAIThreadsSave       func() int
	UpscaleAITTA               func() bool
	UpscaleAIOutputFormat      func() string
	UpscaleApplyFilters        func() bool
	UpscaleFilterChain         func() []string
	UpscaleFrameRate           func() string
	UpscaleMotionInterpolation func() bool
	UpscaleBlurEnabled         func() bool
	UpscaleBlurSigma           func() float64
	UpscaleEncoderPreset       func() string
	UpscaleVideoCodec          func() string
	UpscaleBitrateMode         func() string
	UpscaleBitratePreset       func() string
	UpscaleManualBitrate       func() string
	UpscaleRIFEBackend         func() string
	UpscaleRIFEAvailable       func() bool
	UpscaleRIFEEnabled         func() bool
	UpscaleRIFEMultiplier      func() int
	UpscaleRIFEModel           func() string
	UpscaleRealCUGANAvailable  func() bool
	UpscaleHardwareAccel       func() string
	UpscaleOutputContainer     func() string
	UpscaleManualCRF           func() int
	UpscalePixelFormat         func() string
	UpscaleSrcColorSpace       func() string
	UpscaleColorDepth          func() string
	UpscaleSkinTone            func() string

	SetUpscaleFile                func(f interface{})
	SetUpscaleMethod              func(s string)
	SetUpscaleTargetRes           func(s string)
	SetUpscaleCustomWidth         func(v int)
	SetUpscaleCustomHeight        func(v int)
	SetUpscaleQualityPreset       func(s string)
	SetUpscaleAIEnabled           func(v bool)
	SetUpscaleAIModel             func(s string)
	SetUpscaleAIAvailable         func(v bool)
	SetUpscaleAIBackend           func(s string)
	SetUpscaleAIPreset            func(s string)
	SetUpscaleAIScale             func(v float64)
	SetUpscaleAIScaleUseTarget    func(v bool)
	SetUpscaleAIOutputAdjust      func(v float64)
	SetUpscaleAIFaceEnhance       func(v bool)
	SetUpscaleAIDenoise           func(v float64)
	SetUpscaleAITile              func(v int)
	SetUpscaleAIGPU               func(v int)
	SetUpscaleAIGPUAuto           func(v bool)
	SetUpscaleAIThreadsLoad       func(v int)
	SetUpscaleAIThreadsProc       func(v int)
	SetUpscaleAIThreadsSave       func(v int)
	SetUpscaleAITTA               func(v bool)
	SetUpscaleAIOutputFormat      func(s string)
	SetUpscaleApplyFilters        func(v bool)
	SetUpscaleFilterChain         func(chain []string)
	SetUpscaleFrameRate           func(s string)
	SetUpscaleMotionInterpolation func(v bool)
	SetUpscaleBlurEnabled         func(v bool)
	SetUpscaleBlurSigma           func(v float64)
	SetUpscaleEncoderPreset       func(s string)
	SetUpscaleVideoCodec          func(s string)
	SetUpscaleBitrateMode         func(s string)
	SetUpscaleBitratePreset       func(s string)
	SetUpscaleManualBitrate       func(s string)
	SetUpscaleRIFEBackend         func(s string)
	SetUpscaleRIFEAvailable       func(v bool)
	SetUpscaleRIFEEnabled         func(v bool)
	SetUpscaleRIFEMultiplier      func(v int)
	SetUpscaleRIFEModel           func(s string)
	SetUpscaleRealCUGANAvailable  func(v bool)
	SetUpscaleHardwareAccel       func(s string)
	SetUpscaleOutputContainer     func(s string)
	SetUpscaleManualCRF           func(v int)
	SetUpscalePixelFormat         func(s string)
	SetUpscaleSrcColorSpace       func(s string)
	SetUpscaleColorDepth          func(s string)
	SetUpscaleSkinTone            func(s string)

	FiltersFile    func() interface{}
	SetFiltersFile func(f interface{})
	JobQueue       func() *queue.Queue
	AddJob         func(job *queue.Job)

	// Filter state getters for integrated filter controls
	FilterBrightness func() float64
	FilterContrast   func() float64
	FilterSaturation func() float64
	FilterSharpness  func() float64
	FilterDenoise    func() float64
	FilterGrayscale  func() bool
	FilterFlipH      func() bool
	FilterFlipV      func() bool
	FilterRotation   func() int

	SetFilterBrightness func(f float64)
	SetFilterContrast   func(f float64)
	SetFilterSaturation func(f float64)
	SetFilterSharpness  func(f float64)
	SetFilterDenoise    func(f float64)
	SetFilterGrayscale  func(b bool)
	SetFilterFlipH      func(b bool)
	SetFilterFlipV      func(b bool)
	SetFilterRotation   func(i int)
}

const ModuleColor = "#2B9C1C"
