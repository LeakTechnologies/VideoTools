package ui

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
)

var consoleDark = color.NRGBA{R: 0x0a, G: 0x0d, B: 0x18, A: 0xff}

// NewConsoleBox builds a styled terminal-output box:
//   - dark background with a rounded, coloured border
//   - small pill at the top-left showing label + a clipboard copy icon
//   - content embedded in the body (typically a scrollable label)
//
// getText is called when the copy icon is pressed; pass nil to omit it.
// headerExtra items are appended to the header row to the right of the pill
// (e.g. a "view full log" button).
func NewConsoleBox(
	label string,
	accentColor color.Color,
	content fyne.CanvasObject,
	getText func() string,
	window fyne.Window,
	headerExtra ...fyne.CanvasObject,
) fyne.CanvasObject {
	textColor := getContrastColor(accentColor)

	pillBg := canvas.NewRectangle(accentColor)
	pillBg.CornerRadius = 5

	labelTxt := canvas.NewText(label, textColor)
	labelTxt.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	labelTxt.TextSize = 11

	pillParts := []fyne.CanvasObject{container.NewPadded(labelTxt)}
	if getText != nil && window != nil {
		copyBtn := MakePillIconButton(theme.ContentCopyIcon(), func() {
			text := getText()
			if strings.TrimSpace(text) == "" {
				return
			}
			window.Clipboard().SetContent(text)
		})
		pillParts = append(pillParts, copyBtn)
	}
	pill := container.NewMax(pillBg, container.NewHBox(pillParts...))

	headerItems := []fyne.CanvasObject{pill, layout.NewSpacer()}
	headerItems = append(headerItems, headerExtra...)
	header := container.NewHBox(headerItems...)

	borderRect := canvas.NewRectangle(consoleDark)
	borderRect.StrokeColor = accentColor
	borderRect.StrokeWidth = 2
	borderRect.CornerRadius = 8

	inner := container.NewBorder(header, nil, nil, nil,
		container.NewPadded(content),
	)
	return container.NewMax(borderRect, container.NewPadded(inner))
}
