package theme

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// PillIconButton is a square icon-only pill-shaped button.
// Use for transport controls and other toolbar-style icon actions.
type PillIconButton struct {
	widget.DisableableWidget
	Icon     fyne.Resource
	OnTapped func()
	Active   bool    // true = highlighted (e.g. PiP active, mute engaged)
	IconSize float32 // button width/height (square); default 36
	hovered  bool
}

func NewPillIconButton(icon fyne.Resource, onTapped func()) *PillIconButton {
	p := &PillIconButton{
		Icon:     icon,
		OnTapped: onTapped,
		IconSize: 36,
	}
	p.ExtendBaseWidget(p)
	return p
}

func (p *PillIconButton) SetIcon(resource fyne.Resource) {
	p.Icon = resource
	p.Refresh()
}

func (p *PillIconButton) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(nil)
	bg.CornerRadius = 8
	bg.StrokeWidth = 1.5
	icon := canvas.NewImageFromResource(p.Icon)
	return &pillIconRenderer{btn: p, bg: bg, icon: icon}
}

func (p *PillIconButton) MouseIn(*desktop.MouseEvent) {
	if p.Disabled() {
		return
	}
	p.hovered = true
	p.Refresh()
}

func (p *PillIconButton) MouseOut() {
	p.hovered = false
	p.Refresh()
}

func (p *PillIconButton) MouseMoved(*desktop.MouseEvent) {}

func (p *PillIconButton) Tapped(*fyne.PointEvent) {
	if p.Disabled() {
		return
	}
	if p.OnTapped != nil {
		p.OnTapped()
	}
}

type pillIconRenderer struct {
	btn  *PillIconButton
	bg   *canvas.Rectangle
	icon *canvas.Image
}

func (r *pillIconRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	pad := float32(6)
	iconSize := size.Width - pad*2
	if iconSize < 8 {
		iconSize = 8
	}
	r.icon.Resize(fyne.NewSize(iconSize, iconSize))
	r.icon.Move(fyne.NewPos(pad, pad))
}

func (r *pillIconRenderer) MinSize() fyne.Size {
	s := r.btn.IconSize
	if s <= 0 {
		s = 36
	}
	return fyne.NewSize(s, s)
}

func (r *pillIconRenderer) Refresh() {
	p := r.btn
	r.icon.Resource = p.Icon
	switch {
	case p.Disabled():
		r.bg.FillColor = BgLight
		r.bg.StrokeColor = TextMuted
	case p.Active:
		r.bg.FillColor = BgCard
		r.bg.StrokeColor = Green
	case p.hovered:
		r.bg.FillColor = BgLight
		r.bg.StrokeColor = TextMuted
	default:
		r.bg.FillColor = BgLight
		r.bg.StrokeColor = color.Transparent
	}
	r.bg.Refresh()
	r.icon.Refresh()
}

func (r *pillIconRenderer) Destroy() {}

func (r *pillIconRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.icon}
}
