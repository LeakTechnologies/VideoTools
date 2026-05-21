package settings

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/ui"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/utils"
)

const ModuleColor = "#607D8B"

type Options struct {
	Window   fyne.Window
	StatsBar fyne.CanvasObject
	OnBack   func()

	BuildPreferencesTab  func() fyne.CanvasObject
	BuildDependenciesTab func() fyne.CanvasObject
	BuildBenchmarkTab    func() fyne.CanvasObject
}

func BuildView(opts Options) fyne.CanvasObject {
	t := i18n.T()

	backBtn := ui.MakePillButton("< "+strings.ToUpper(t.ModuleSettings), ui.BorderDim, opts.OnBack)
	settingsColor := utils.MustHex(ModuleColor)
	topBar := ui.TintedBar(settingsColor, container.NewHBox(backBtn, layout.NewSpacer()))

	var bottomBar fyne.CanvasObject
	if opts.StatsBar != nil {
		bottomBar = container.NewHBox(layout.NewSpacer(), opts.StatsBar)
	} else {
		bottomBar = container.NewHBox(layout.NewSpacer())
	}

	page := container.NewVBox(
		opts.BuildPreferencesTab(),
		widget.NewSeparator(),
		opts.BuildDependenciesTab(),
		widget.NewSeparator(),
		opts.BuildBenchmarkTab(),
	)
	return container.NewBorder(topBar, bottomBar, nil, nil,
		ui.NewFastVScroll(container.NewPadded(page)))
}

func ModuleColorValue() color.Color {
	return utils.MustHex(ModuleColor)
}
