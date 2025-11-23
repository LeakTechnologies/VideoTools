package ui

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
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
	label    string
	color    color.Color
	onTapped func()
}

// NewModuleTile creates a new module tile
func NewModuleTile(label string, col color.Color, tapped func()) *ModuleTile {
	m := &ModuleTile{
		label:    strings.ToUpper(label),
		color:    col,
		onTapped: tapped,
	}
	m.ExtendBaseWidget(m)
	return m
}

func (m *ModuleTile) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(m.color)
	bg.CornerRadius = 8
	bg.StrokeColor = GridColor
	bg.StrokeWidth = 1

	txt := canvas.NewText(m.label, TextColor)
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
	if m.onTapped != nil {
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
