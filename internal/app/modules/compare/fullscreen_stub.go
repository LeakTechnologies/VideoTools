//go:build !native_media

package compare

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/LeakTechnologies/VideoTools/internal/i18n"
	"github.com/LeakTechnologies/VideoTools/internal/ui"
	"github.com/LeakTechnologies/VideoTools/internal/utils"
)

// BuildFullscreenView renders a side-by-side fullscreen comparison.
// In standard builds this uses the OnBuildVideoPane callback.
// Build with -tags native_media for the frame-accurate engine version.
func BuildFullscreenView(opts Options) fyne.CanvasObject {
	compareColor := utils.MustHex("#91931A")
	t := i18n.T()

	backBtn := ui.MakePillButton(t.CompareBackToView, ui.BorderDim, func() {
		if opts.OnShowCompareFullscreen != nil {
			opts.OnShowCompareFullscreen()
		}
	})
	topBar := ui.TintedBar(compareColor, container.NewHBox(backBtn, layout.NewSpacer()))

	var left, right fyne.CanvasObject
	if opts.OnBuildVideoPane != nil {
		if opts.CompareFile1 != nil {
			left = opts.OnBuildVideoPane(nil, fyne.NewSize(640, 360), opts.CompareFile1, nil)
		}
		if opts.CompareFile2 != nil {
			right = opts.OnBuildVideoPane(nil, fyne.NewSize(640, 360), opts.CompareFile2, nil)
		}
	}
	if left == nil {
		left = container.NewCenter(widget.NewLabel(""))
	}
	if right == nil {
		right = container.NewCenter(widget.NewLabel(""))
	}

	return container.NewBorder(topBar, nil, nil, nil,
		container.NewGridWithColumns(2, left, right))
}
