package enhancement

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/utils"
)

var gridColor = utils.MustHex("#2A3A52")

func BuildView() fyne.CanvasObject {
	t := i18n.T()
	viewTitle := widget.NewLabelWithStyle(t.ModuleEnhancement, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	content := container.NewVBox(
		viewTitle,
		widget.NewSeparator(),
		widget.NewLabel("AI-powered video enhancement is coming soon!"),
		widget.NewLabel("Features planned:"),
		widget.NewLabel(" Real-ESRGAN Super-Resolution"),
		widget.NewLabel(" BasicVSR Video Enhancement"),
		widget.NewLabel(" Content-Aware Processing"),
		widget.NewLabel(" Real-time Preview"),
		widget.NewSeparator(),
		widget.NewLabel("This will use the unified FFmpeg player foundation"),
		widget.NewLabel("for frame-accurate enhancement processing."),
	)

	outer := canvas.NewRectangle(utils.MustHex("#191F35"))
	outer.CornerRadius = 8
	outer.StrokeColor = gridColor
	outer.StrokeWidth = 1

	view := container.NewBorder(
		viewTitle,
		nil, nil, nil,
		content,
	)

	_ = outer // background reserved for future use
	return view
}
