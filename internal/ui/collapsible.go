package ui

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// tappableBox wraps any canvas object and makes the entire area tappable.
// Used to turn decorative containers (header bars, tiles) into tap targets
// without requiring a visible button widget.
type tappableBox struct {
	widget.BaseWidget
	content fyne.CanvasObject
	onTap   func()
}

func newTappableBox(content fyne.CanvasObject, onTap func()) *tappableBox {
	tb := &tappableBox{content: content, onTap: onTap}
	tb.ExtendBaseWidget(tb)
	return tb
}

func (tb *tappableBox) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(tb.content)
}

func (tb *tappableBox) Tapped(*fyne.PointEvent) {
	if tb.onTap != nil {
		tb.onTap()
	}
}

// BuildCollapsibleHeader returns a full-width tappable section header bar that
// matches the buildConvertBox / buildRipBox visual language:
//   - module-coloured accent background, CornerRadius 10, height 34
//   - uppercase bold title preceded by ▼ (open) or ▶ (closed) arrow
//   - optional extra widgets appended to the right side of the header
//
// onToggle(open) is called whenever the header is tapped.
// The returned update(open) func lets external callers drive the arrow state
// (e.g. when the caller controls a VSplit offset directly).
//
// extraRight items appear between the title and the right edge, in order.
func BuildCollapsibleHeader(
	title string,
	accentColor color.Color,
	onToggle func(open bool),
	extraRight ...fyne.CanvasObject,
) (fyne.CanvasObject, func(open bool)) {
	hdrBg := canvas.NewRectangle(accentColor)
	hdrBg.CornerRadius = 10
	hdrBg.SetMinSize(fyne.NewSize(0, 34))

	textColor := getContrastColor(accentColor)
	arrow := canvas.NewText("▼  "+strings.ToUpper(title), textColor)
	arrow.TextStyle = fyne.TextStyle{Bold: true}
	arrow.TextSize = 12

	rowItems := []fyne.CanvasObject{arrow, layout.NewSpacer()}
	rowItems = append(rowItems, extraRight...)
	visual := container.NewMax(
		hdrBg,
		container.NewPadded(container.NewHBox(rowItems...)),
	)

	open := true
	update := func(o bool) {
		open = o
		if open {
			arrow.Text = "▼  " + strings.ToUpper(title)
		} else {
			arrow.Text = "▶  " + strings.ToUpper(title)
		}
		arrow.Refresh()
	}

	hdr := newTappableBox(visual, func() {
		open = !open
		update(open)
		if onToggle != nil {
			onToggle(open)
		}
	})

	return hdr, update
}
