package settings

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"image/color"

	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
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

	backBtn := widget.NewButton("< "+strings.ToUpper(t.ModuleSettings), opts.OnBack)
	backBtn.Importance = widget.LowImportance

	settingsColor := utils.MustHex(ModuleColor)
	topBar := ui.TintedBar(settingsColor, container.NewHBox(backBtn, layout.NewSpacer()))

	var bottomBar fyne.CanvasObject
	if opts.StatsBar != nil {
		bottomBar = container.NewHBox(layout.NewSpacer(), opts.StatsBar)
	} else {
		bottomBar = container.NewHBox(layout.NewSpacer())
	}

	tabs := container.NewAppTabs(
		container.NewTabItem(t.SettingsTabPreferences, ui.NewFastVScroll(container.NewPadded(opts.BuildPreferencesTab()))),
		container.NewTabItem(t.SettingsTabDependencies, ui.NewFastVScroll(container.NewPadded(opts.BuildDependenciesTab()))),
		container.NewTabItem(t.SettingsTabBenchmark, ui.NewFastVScroll(container.NewPadded(opts.BuildBenchmarkTab()))),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	return container.NewBorder(topBar, bottomBar, nil, nil, tabs)
}

func ModuleColorValue() color.Color {
	return utils.MustHex(ModuleColor)
}
