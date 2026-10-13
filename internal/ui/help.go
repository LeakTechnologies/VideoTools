package ui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// hoverHelp belongs to the control, so pointer exit can cancel delayed help.
// Unlike the old SetTooltip helper it does not require a separate window layer.
type hoverHelp struct {
	window     fyne.Window
	owner      fyne.CanvasObject
	text       string
	timer      *time.Timer
	popup      *widget.PopUp
	generation uint64
}

// Active controls are managed on Fyne's UI thread together with pointer events.
var activeHoverHelp = map[*hoverHelp]struct{}{}

// DismissTooltips retires pending and visible hover help before changing panes.
func DismissTooltips() {
	for help := range activeHoverHelp {
		help.leave()
	}
}

func (h *hoverHelp) enter() {
	h.leave()
	if !ShowTooltips || h.text == "" {
		return
	}
	generation := h.generation
	activeHoverHelp[h] = struct{}{}
	h.timer = time.AfterFunc(650*time.Millisecond, func() {
		fyne.Do(func() {
			if generation != h.generation || !ShowTooltips || !h.owner.Visible() {
				return
			}
			canvas := fyne.CurrentApp().Driver().CanvasForObject(h.owner)
			if h.window != nil {
				canvas = h.window.Canvas()
			}
			if canvas == nil {
				return
			}
			label := widget.NewLabel(h.text)
			label.Wrapping = fyne.TextWrapWord
			width := float32(340)
			if available := canvas.Size().Width - 16; available < width {
				width = available
			}
			if width < 1 {
				return
			}
			label.Resize(fyne.NewSize(width-16, label.MinSize().Height))
			height := label.MinSize().Height + 16
			maximumHeight := canvas.Size().Height - 16
			if maximumHeight < 1 {
				return
			}
			if height > maximumHeight {
				height = maximumHeight
			}
			content := container.NewVScroll(container.NewPadded(label))
			h.popup = widget.NewPopUp(content, canvas)
			h.popup.Resize(fyne.NewSize(width, height))
			position := fyne.CurrentApp().Driver().AbsolutePositionForObject(h.owner)
			position.Y += h.owner.Size().Height
			h.popup.ShowAtPosition(position)
		})
	})
}

func (h *hoverHelp) leave() {
	delete(activeHoverHelp, h)
	h.generation++
	if h.timer != nil {
		h.timer.Stop()
		h.timer = nil
	}
	if h.popup != nil {
		h.popup.Hide()
		h.popup = nil
	}
}

// TooltipEntry keeps the normal Entry focus/editing behavior and adds real hover help.
type TooltipEntry struct {
	widget.Entry
	help hoverHelp
}

func NewTooltipEntry(text string) *TooltipEntry {
	e := &TooltipEntry{}
	e.ExtendBaseWidget(e)
	e.help = hoverHelp{owner: e, text: text}
	return e
}

func (e *TooltipEntry) SetToolTip(text string)         { e.help.leave(); e.help.text = text }
func (e *TooltipEntry) MouseIn(*desktop.MouseEvent)    { e.help.enter() }
func (e *TooltipEntry) MouseMoved(*desktop.MouseEvent) {}
func (e *TooltipEntry) MouseOut()                      { e.help.leave() }
func (e *TooltipEntry) Hide()                          { e.help.leave(); e.Entry.Hide() }

// HelpButton exposes optional explanations on hover, click, or keyboard activation.
// Its accessible action remains available when hover tooltips are disabled.
type HelpButton struct {
	widget.Button
	help hoverHelp
}

func NewHelpButton(text string) *HelpButton {
	b := &HelpButton{}
	b.Icon = theme.InfoIcon()
	b.Importance = widget.LowImportance
	b.ExtendBaseWidget(b)
	b.help = hoverHelp{owner: b, text: text}
	b.OnTapped = func() {
		b.help.leave()
		canvas := fyne.CurrentApp().Driver().CanvasForObject(b)
		for _, window := range fyne.CurrentApp().Driver().AllWindows() {
			if window.Canvas() == canvas {
				dialog.ShowInformation("", b.help.text, window)
				break
			}
		}
	}
	return b
}

func (b *HelpButton) MouseIn(e *desktop.MouseEvent)    { b.Button.MouseIn(e); b.help.enter() }
func (b *HelpButton) MouseMoved(e *desktop.MouseEvent) { b.Button.MouseMoved(e) }
func (b *HelpButton) MouseOut()                        { b.help.leave(); b.Button.MouseOut() }
func (b *HelpButton) Hide()                            { b.help.leave(); b.Button.Hide() }
func (b *HelpButton) SetToolTip(text string)           { b.help.leave(); b.help.text = text }

// WithHelp reserves one compact help action beside a control instead of a prose row.
func WithHelp(control fyne.CanvasObject, text string) fyne.CanvasObject {
	return container.NewBorder(nil, nil, nil, NewHelpButton(text), control)
}

func (b *HelpButton) SetHelpText(text string) { b.SetToolTip(text) }

func (m *ModuleTile) MouseIn(*desktop.MouseEvent)    { m.help.enter() }
func (m *ModuleTile) MouseMoved(*desktop.MouseEvent) {}
func (m *ModuleTile) MouseOut()                      { m.help.leave() }
func (m *ModuleTile) Hide()                          { m.help.leave(); m.BaseWidget.Hide() }
func (m *ModuleTile) SetToolTip(text string)         { m.help.leave(); m.help.text = text }

func (c *ColoredSelect) MouseIn(*desktop.MouseEvent)    { c.help.enter() }
func (c *ColoredSelect) MouseMoved(*desktop.MouseEvent) {}
func (c *ColoredSelect) MouseOut()                      { c.help.leave() }
func (c *ColoredSelect) Hide()                          { c.help.leave(); c.BaseWidget.Hide() }
func (c *ColoredSelect) SetToolTip(text string)         { c.help.leave(); c.help.text = text }
