package ui

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

var (
	// GridColor is the color used for grid lines and borders
	GridColor color.Color
	// TextColor is the main text color
	TextColor color.Color
)

// SetColors sets the UI colors
func SetColors(grid, text color.Color) {
	GridColor = grid
	TextColor = text
}

// MonoTheme ensures all text uses a monospace font
type MonoTheme struct{}

func (m *MonoTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(name, variant)
}

func (m *MonoTheme) Font(style fyne.TextStyle) fyne.Resource {
	style.Monospace = true
	return theme.DefaultTheme().Font(style)
}

func (m *MonoTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m *MonoTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}

// ModuleTile is a clickable tile widget for module selection
type ModuleTile struct {
	widget.BaseWidget
	label     string
	color     color.Color
	enabled   bool
	onTapped  func()
	onDropped func([]fyne.URI)
}

// NewModuleTile creates a new module tile
func NewModuleTile(label string, col color.Color, enabled bool, tapped func(), dropped func([]fyne.URI)) *ModuleTile {
	m := &ModuleTile{
		label:     strings.ToUpper(label),
		color:     col,
		enabled:   enabled,
		onTapped:  tapped,
		onDropped: dropped,
	}
	m.ExtendBaseWidget(m)
	return m
}

// DraggedOver implements desktop.Droppable interface
func (m *ModuleTile) DraggedOver(pos fyne.Position) {
	logging.Debug(logging.CatUI, "DraggedOver tile=%s enabled=%v pos=%v", m.label, m.enabled, pos)
}

// Dropped implements desktop.Droppable interface
func (m *ModuleTile) Dropped(pos fyne.Position, items []fyne.URI) {
	logging.Debug(logging.CatUI, "Dropped on tile=%s enabled=%v items=%v", m.label, m.enabled, items)
	if m.enabled && m.onDropped != nil {
		logging.Debug(logging.CatUI, "Calling onDropped callback for %s", m.label)
		m.onDropped(items)
	} else {
		logging.Debug(logging.CatUI, "Drop ignored: enabled=%v hasCallback=%v", m.enabled, m.onDropped != nil)
	}
}

func (m *ModuleTile) CreateRenderer() fyne.WidgetRenderer {
	tileColor := m.color
	labelColor := TextColor

	// Dim disabled tiles
	if !m.enabled {
		// Reduce opacity by mixing with dark background
		if c, ok := m.color.(color.NRGBA); ok {
			tileColor = color.NRGBA{R: c.R / 3, G: c.G / 3, B: c.B / 3, A: c.A}
		}
		if c, ok := TextColor.(color.NRGBA); ok {
			labelColor = color.NRGBA{R: c.R / 2, G: c.G / 2, B: c.B / 2, A: c.A}
		}
	}

	bg := canvas.NewRectangle(tileColor)
	bg.CornerRadius = 8
	bg.StrokeColor = GridColor
	bg.StrokeWidth = 1

	txt := canvas.NewText(m.label, labelColor)
	txt.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	txt.Alignment = fyne.TextAlignCenter
	txt.TextSize = 20

	return &moduleTileRenderer{
		tile:  m,
		bg:    bg,
		label: txt,
	}
}

func (m *ModuleTile) Tapped(*fyne.PointEvent) {
	if m.enabled && m.onTapped != nil {
		m.onTapped()
	}
}

type moduleTileRenderer struct {
	tile  *ModuleTile
	bg    *canvas.Rectangle
	label *canvas.Text
}

func (r *moduleTileRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	// Center the label by positioning it in the middle
	labelSize := r.label.MinSize()
	r.label.Resize(labelSize)
	x := (size.Width - labelSize.Width) / 2
	y := (size.Height - labelSize.Height) / 2
	r.label.Move(fyne.NewPos(x, y))
}

func (r *moduleTileRenderer) MinSize() fyne.Size {
	return fyne.NewSize(220, 110)
}

func (r *moduleTileRenderer) Refresh() {
	r.bg.FillColor = r.tile.color
	r.bg.Refresh()
	r.label.Text = r.tile.label
	r.label.Refresh()
}

func (r *moduleTileRenderer) Destroy() {}

func (r *moduleTileRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.label}
}

// TintedBar creates a colored bar container
func TintedBar(col color.Color, body fyne.CanvasObject) fyne.CanvasObject {
	rect := canvas.NewRectangle(col)
	rect.SetMinSize(fyne.NewSize(0, 48))
	padded := container.NewPadded(body)
	return container.NewMax(rect, padded)
}
