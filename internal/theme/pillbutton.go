package theme

import (
	"fmt"
	"image/color"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

var startupDebug = os.Getenv("VT_STARTUP_DEBUG") != ""

// PillButton renders a pill-shaped button matching the roadmap's visual style:
// dark background, coloured border, centred text, hover lightens border, active inverts.
type PillButton struct {
	widget.DisableableWidget
	Label     string
	BorderCol color.Color
	OnTapped  func()
	hovered   bool
	Active    bool
}

func NewPillButton(label string, borderCol color.Color, onTapped func()) *PillButton {
	p := &PillButton{
		Label:     label,
		BorderCol: borderCol,
		OnTapped:  onTapped,
	}
	p.ExtendBaseWidget(p)
	return p
}

func (p *PillButton) CreateRenderer() fyne.WidgetRenderer {
	if startupDebug {
		fmt.Fprintf(os.Stderr, "[vt-debug] PillButton.CreateRenderer label=%q\n", p.Label)
	}
	bg := canvas.NewRectangle(BgLight)
	bg.CornerRadius = 12
	bg.StrokeWidth = 1.5
	bg.StrokeColor = p.BorderCol
	txt := canvas.NewText(p.Label, TextOnDark)
	txt.Alignment = fyne.TextAlignCenter
	txt.TextStyle = fyne.TextStyle{Bold: true}
	return &pillButtonRenderer{pill: p, bg: bg, txt: txt}
}

func (p *PillButton) MouseIn(*desktop.MouseEvent) {
	if p.Disabled() {
		return
	}
	p.hovered = true
	p.Refresh()
}

func (p *PillButton) MouseOut() {
	p.hovered = false
	p.Refresh()
}

func (p *PillButton) MouseMoved(*desktop.MouseEvent) {}

func (p *PillButton) SetText(label string) {
	p.Label = label
	p.Refresh()
}

func (p *PillButton) Tapped(*fyne.PointEvent) {
	if p.Disabled() {
		return
	}
	if p.OnTapped != nil {
		p.OnTapped()
	}
}

type pillButtonRenderer struct {
	pill *PillButton
	bg   *canvas.Rectangle
	txt  *canvas.Text
}

func (r *pillButtonRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.txt.Resize(size)
}

func (r *pillButtonRenderer) MinSize() fyne.Size {
	return r.txt.MinSize().Add(fyne.NewSize(24, 12))
}

func (r *pillButtonRenderer) Refresh() {
	p := r.pill
	switch {
	case p.Disabled():
		r.bg.FillColor = BgLight
		r.bg.StrokeColor = TextMuted
		r.txt.Color = TextMuted
	case p.Active:
		r.bg.FillColor = p.BorderCol
		r.bg.StrokeColor = p.BorderCol
		r.txt.Color = BgDark
	case p.hovered:
		r.bg.FillColor = BgLight
		r.bg.StrokeColor = TextMuted
		r.txt.Color = TextOnDark
	default:
		r.bg.FillColor = BgLight
		r.bg.StrokeColor = p.BorderCol
		r.txt.Color = TextOnDark
	}
	r.txt.Text = p.Label
	r.bg.Refresh()
	r.txt.Refresh()
}

func (r *pillButtonRenderer) Destroy() {}

func (r *pillButtonRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.txt}
}
