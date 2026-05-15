package theme

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// NewTitleLabel creates a page/module title (Monospace, Bold, 24pt).
// Use for top-level view titles like "Convert", "Audio", "Settings".
func NewTitleLabel(text string, col color.Color) *canvas.Text {
	t := canvas.NewText(text, col)
	t.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	t.TextSize = 24
	return t
}

// NewSectionLabel creates a bold section heading.
// Use for grouping controls within a view (e.g. "Output Settings", "Source").
func NewSectionLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Bold: true}
	return l
}

// NewWrappingLabel creates body text with word wrapping enabled.
// Use for instructions, descriptions, and multi-line status messages.
func NewWrappingLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.Wrapping = fyne.TextWrapWord
	return l
}

// NewHintLabel creates italic hint/secondary text.
// Use for explanations, context help, and non-critical information.
func NewHintLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Italic: true}
	return l
}

// NewMonoLabel creates monospace technical text.
// Use for file hashes, paths, FFmpeg output, and other machine-readable data.
func NewMonoLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Monospace: true}
	return l
}
