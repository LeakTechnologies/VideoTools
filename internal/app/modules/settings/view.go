package settings

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

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

	// KeyHandler, when non-nil, is assigned a closure the settings key-nav
	// widget forwards navigation keys to. It returns true when the key was a
	// navigation key the host handled.
	KeyHandler *func(fyne.KeyName) bool

	// KeyCatcher, when non-nil, is assigned the focusable key-nav widget so the
	// host can focus it while Settings is on screen. Unmodified navigation keys
	// (PageUp/PageDown/Home/End) never reach Canvas.AddShortcut in Fyne's GLFW
	// driver, so the keys are intercepted through this focused widget's
	// TypedKey instead.
	KeyCatcher *fyne.Focusable
}

func BuildView(opts Options) fyne.CanvasObject {
	t := i18n.T()

	backBtn := ui.MakePillButton("< "+strings.ToUpper(t.ModuleSettings), ui.BorderDim, opts.OnBack)
	settingsColor := utils.MustHex(ModuleColor)

	var keyNav *settingsKeyNav
	if opts.KeyHandler != nil {
		keyNav = newSettingsKeyNav(func(key fyne.KeyName) {
			if h := *opts.KeyHandler; h != nil {
				h(key)
			}
		})
		if opts.KeyCatcher != nil {
			*opts.KeyCatcher = keyNav
		}
	}

	var keyNavObj fyne.CanvasObject
	if keyNav != nil {
		keyNavObj = keyNav
	} else {
		keyNavObj = layout.NewSpacer()
	}
	topBar := ui.TintedBar(settingsColor, container.NewHBox(backBtn, layout.NewSpacer(), keyNavObj))

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

var (
	_ fyne.Focusable  = (*settingsKeyNav)(nil)
	_ desktop.Keyable = (*settingsKeyNav)(nil)
)

// settingsKeyNav is an invisible, focusable widget that intercepts navigation
// keys (PageUp/PageDown/Home/End) while Settings is on screen. Fyne's GLFW
// driver only synthesizes desktop.CustomShortcut for modified keys, so plain
// keys must be caught through a focused widget's TypedKey (which also receives
// key repeats) or the canvas-level OnKeyDown fallback.
type settingsKeyNav struct {
	widget.BaseWidget
	onKey func(fyne.KeyName)
}

type settingsKeyNavRenderer struct {
	rect *canvas.Rectangle
}

func (r *settingsKeyNavRenderer) Layout(_ fyne.Size) {}

func (r *settingsKeyNavRenderer) MinSize() fyne.Size { return fyne.NewSize(1, 1) }

func (r *settingsKeyNavRenderer) Refresh() {}

func (r *settingsKeyNavRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.rect} }

func (r *settingsKeyNavRenderer) Destroy() {}

func newSettingsKeyNav(onKey func(fyne.KeyName)) *settingsKeyNav {
	k := &settingsKeyNav{onKey: onKey}
	k.ExtendBaseWidget(k)
	return k
}

func (k *settingsKeyNav) CreateRenderer() fyne.WidgetRenderer {
	rect := canvas.NewRectangle(color.Transparent)
	rect.Resize(fyne.NewSize(1, 1))
	return &settingsKeyNavRenderer{rect: rect}
}

func (k *settingsKeyNav) MinSize() fyne.Size { return fyne.NewSize(1, 1) }

func (k *settingsKeyNav) FocusGained() {}

func (k *settingsKeyNav) FocusLost() {}

func (k *settingsKeyNav) TypedRune(rune) {}

func (k *settingsKeyNav) KeyDown(*fyne.KeyEvent) {}

func (k *settingsKeyNav) KeyUp(*fyne.KeyEvent) {}

// TypedKey fires on both the initial press and repeats once this widget is the
// focused object, so holding a navigation key pages repeatedly.
func (k *settingsKeyNav) TypedKey(ev *fyne.KeyEvent) {
	if k.onKey != nil {
		k.onKey(ev.Name)
	}
}
