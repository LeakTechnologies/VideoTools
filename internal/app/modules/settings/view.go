package settings

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"

	"github.com/LeakTechnologies/VideoTools/internal/i18n"
	"github.com/LeakTechnologies/VideoTools/internal/ui"
	"github.com/LeakTechnologies/VideoTools/internal/utils"
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

	// ActiveScroll, when non-nil, is assigned a closure resolving the scroll
	// container of the currently visible tab. The settings host uses it to
	// drive keyboard navigation (PageUp/PageDown/Home/End).
	ActiveScroll *func() *ui.FastVScroll
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

	prefScroll := ui.NewFastVScroll(container.NewPadded(opts.BuildPreferencesTab()))
	depScroll := ui.NewFastVScroll(container.NewPadded(opts.BuildDependenciesTab()))
	benchScroll := ui.NewFastVScroll(container.NewPadded(opts.BuildBenchmarkTab()))
	scrolls := []*ui.FastVScroll{prefScroll, depScroll, benchScroll}
	activeScroll := 0
	tabs := container.NewAppTabs(
		container.NewTabItem(t.SettingsTabPreferences, prefScroll),
		container.NewTabItem(t.SettingsTabDependencies, depScroll),
		container.NewTabItem(t.SettingsTabBenchmark, benchScroll),
	)
	tabs.SetTabLocation(container.TabLocationTop)
	tabs.OnSelected = func(item *container.TabItem) {
		for i, sc := range scrolls {
			if item.Content == sc {
				activeScroll = i
				return
			}
		}
	}
	if opts.ActiveScroll != nil {
		*opts.ActiveScroll = func() *ui.FastVScroll {
			return scrolls[activeScroll]
		}
	}

	return container.NewBorder(topBar, bottomBar, nil, nil,
		container.New(&centeredPanel{maxWidth: settingsPanelMaxWidth}, tabs))
}

func ModuleColorValue() color.Color {
	return utils.MustHex(ModuleColor)
}
