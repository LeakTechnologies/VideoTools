package upscale

import (
	"fmt"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

var gridColor = utils.MustHex("#2A3A52")
var navyBlue = utils.MustHex("#191F35")
var mediumBlue = utils.MustHex("#13182B")

type Options struct {
	Window fyne.Window

	State interface{}

	QueueBtn   *widget.Button
	StatsBar   fyne.CanvasObject

	OnShowMainMenu       func()
	OnShowQueue         func()
	OnShowFiltersView   func()
	OnShowUpscaleView   func()
	OnUpdateQueueButtonLabel func()
	OnProbeVideo        func(path string) (interface{}, error)
	OnBuildVideoPane    func(state interface{}, size fyne.Size, src interface{}, onSeek func(float64)) fyne.CanvasObject
	OnGetUpscaleFooter  func(content fyne.CanvasObject) fyne.CanvasObject
}

func BuildView(opts Options) fyne.CanvasObject {
	upscaleColor := utils.MustHex("#E91E63")
	t := i18n.T()

	backBtn := widget.NewButton("< "+t.ModuleUpscale, func() {
		if opts.OnShowMainMenu != nil {
			opts.OnShowMainMenu()
		}
	})
	backBtn.Importance = widget.LowImportance

	queueBtn := widget.NewButton(t.ActionViewQueue, func() {
		if opts.OnShowQueue != nil {
			opts.OnShowQueue()
		}
	})
	if opts.QueueBtn != nil {
		opts.QueueBtn = queueBtn
	}
	if opts.OnUpdateQueueButtonLabel != nil {
		opts.OnUpdateQueueButtonLabel()
	}

	topBar := ui.TintedBar(upscaleColor, container.NewHBox(backBtn, layout.NewSpacer(), queueBtn))

	getState := func() *upscaleState {
		if opts.State == nil {
			return nil
		}
		if s, ok := opts.State.(*upscaleState); ok {
			return s
		}
		return nil
	}

	state := getState()

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

	fileLabel := widget.NewLabel(t.LabelNoFile)
	fileLabel.TextStyle = fyne.TextStyle{Bold: true}

	var videoContainer fyne.CanvasObject
	if state != nil && state.upscaleFile != nil {
		if vs, ok := state.upscaleFile.(interface{ Path string; Width int; Height int }); ok {
			fileLabel.SetText(fmt.Sprintf(t.LabelFileFmt, filepath.Base(vs.Path)))
		}
		if opts.OnBuildVideoPane != nil {
			videoContainer = opts.OnBuildVideoPane(nil, fyne.NewSize(480, 270), state.upscaleFile, nil)
		}
	} else {
		videoContainer = container.NewCenter(widget.NewLabel(t.LabelNoVideoLoaded))
	}

	loadBtn := widget.NewButton(t.ActionLoadVideo, func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()

			path := reader.URI().Path()
			go func() {
				var src interface{}
				var probeErr error
				if opts.OnProbeVideo != nil {
					src, probeErr = opts.OnProbeVideo(path)
				}
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					if probeErr != nil {
						dialog.ShowError(probeErr, opts.Window)
						return
					}
					if state != nil {
						state.upscaleFile = src
					}
					if opts.OnShowUpscaleView != nil {
						opts.OnShowUpscaleView()
					}
				}, false)
			}()
		}, opts.Window)
	})
	loadBtn.Importance = widget.HighImportance

	filtersNavBtn := widget.NewButton(t.UpscaleAdjustFilters, func() {
		if opts.OnShowFiltersView != nil {
			opts.OnShowFiltersView()
		}
	})

	content := container.NewPadded(container.NewVBox(
		container.NewHBox(loadBtn, filtersNavBtn),
		container.NewHBox(fileLabel, videoContainer),
	))

	statsBar := opts.StatsBar
	var bottomBar fyne.CanvasObject
	if opts.OnGetUpscaleFooter != nil {
		bottomBar = opts.OnGetUpscaleFooter(layout.NewSpacer())
	} else {
		bottomBar = container.NewVBox(statsBar, layout.NewSpacer())
	}

	return container.NewBorder(topBar, bottomBar, nil, nil, content)
}

type upscaleState struct {
	upscaleFile            interface{}
	upscaleMethod          string
	upscaleTargetRes       string
	upscaleAIModel         string
	upscaleFrameRate       string
	upscaleQualityPreset   string
	upscaleEncoderPreset   string
	upscaleVideoCodec      string
	upscaleBitrateMode     string
	upscaleBitratePreset   string
	upscaleManualBitrate   string
	upscaleAIPreset        string
	upscaleAIScale         float64
	upscaleAIScaleUseTarget bool
	upscaleAIOutputAdjust  float64
	upscaleAIDenoise       float64
	upscaleAITile          int
	upscaleAIOutputFormat  string
	upscaleAIGPUAuto       bool
	upscaleAIThreadsLoad   int
	upscaleAIThreadsProc   int
	upscaleAIThreadsSave   int
	upscaleBlurSigma       float64
	upscaleBlurEnabled     bool
	upscaleAIBackend       string
	upscaleAIAvailable     bool
	upscaleRIFEBackend     string
	upscaleRIFEAvailable   bool
	upscaleRIFEMultiplier  int
	upscaleRIFEModel       string
	upscaleFilterChain     []string
	filterActiveChain     []string
	upscaleMotionInterpolation bool
	upscaleAIEnabled       bool
	upscaleAITTA           bool
}
