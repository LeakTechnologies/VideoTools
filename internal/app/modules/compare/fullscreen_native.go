//go:build native_media

package compare

import (
	"image"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/media"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

// BuildFullscreenView renders a side-by-side fullscreen comparison using
// the native FFmpeg media engine for frame-accurate playback.
func BuildFullscreenView(opts Options) fyne.CanvasObject {
	compareColor := utils.MustHex("#9C27B0")
	t := i18n.T()

	file1 := toVideoSource(opts.CompareFile1)
	file2 := toVideoSource(opts.CompareFile2)

	backBtn := widget.NewButton(t.CompareBackToView, func() {
		if opts.OnShowCompareFullscreen != nil {
			opts.OnShowCompareFullscreen()
		}
	})
	backBtn.Importance = widget.LowImportance
	topBar := ui.TintedBar(compareColor, container.NewHBox(backBtn, layout.NewSpacer()))

	splitView := media.NewSplitView()

	var engine1, engine2 *media.Engine
	if file1 != nil {
		engine1 = media.NewEngine()
		_ = engine1.Open(file1.Path)
		engine1.Start()
	}
	if file2 != nil {
		engine2 = media.NewEngine()
		_ = engine2.Open(file2.Path)
		engine2.Start()
	}

	go func() {
		for {
			var frame1, frame2 *image.RGBA
			if engine1 != nil {
				frame1, _ = engine1.NextFrame()
			}
			if engine2 != nil {
				frame2, _ = engine2.NextFrame()
			}
			splitView.SetFrames(frame1, frame2)
			time.Sleep(16 * time.Millisecond)
		}
	}()

	statsBar := opts.OnGetStatsBar()
	var bottomBar fyne.CanvasObject
	if opts.OnGetCompareFooter != nil {
		bottomBar = opts.OnGetCompareFooter(layout.NewSpacer())
	} else {
		bottomBar = container.NewVBox(statsBar, layout.NewSpacer())
	}

	return container.NewBorder(topBar, bottomBar, nil, nil, splitView)
}
