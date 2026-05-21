package settings

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/ui"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/utils"
)

const ModuleColor = "#607D8B"

// settingsPanelMaxWidth caps the settings content column so label→control rows
// don't span the full window on wide displays.
const settingsPanelMaxWidth = float32(800)

// centeredPanel is a single-child layout that constrains the child to maxWidth
// and centres it horizontally. On screens narrower than maxWidth the child fills
// the available width, so small displays are unaffected.
type centeredPanel struct{ maxWidth float32 }

func (c *centeredPanel) MinSize(objs []fyne.CanvasObject) fyne.Size {
	if len(objs) == 0 {
		return fyne.NewSize(0, 0)
	}
	s := objs[0].MinSize()
	if s.Width > c.maxWidth {
		s.Width = c.maxWidth
	}
	return s
}

func (c *centeredPanel) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) == 0 {
		return
	}
	w := size.Width
	x := float32(0)
	if w > c.maxWidth {
		x = (w - c.maxWidth) / 2
		w = c.maxWidth
	}
	objs[0].Move(fyne.NewPos(x, 0))
	objs[0].Resize(fyne.NewSize(w, size.Height))
}

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

	tabs := container.NewAppTabs(
		container.NewTabItem(t.SettingsTabPreferences, ui.NewFastVScroll(container.NewPadded(opts.BuildPreferencesTab()))),
		container.NewTabItem(t.SettingsTabDependencies, ui.NewFastVScroll(container.NewPadded(opts.BuildDependenciesTab()))),
		container.NewTabItem(t.SettingsTabBenchmark, ui.NewFastVScroll(container.NewPadded(opts.BuildBenchmarkTab()))),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	return container.NewBorder(topBar, bottomBar, nil, nil,
		container.New(&centeredPanel{maxWidth: settingsPanelMaxWidth}, tabs))
}

func ModuleColorValue() color.Color {
	return utils.MustHex(ModuleColor)
}
