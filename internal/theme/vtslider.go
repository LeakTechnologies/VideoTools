package theme

import (
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

const (
	VTSliderTrackH    = float32(2)
	VTSliderThumbD    = float32(14)
	VTSliderMinHeight = float32(26)
)

// VTSlider is a styled slider: thin coloured track + hollow circle thumb.
// API-compatible with the fields of widget.Slider that are used across VT.
type VTSlider struct {
	widget.BaseWidget

	Min   float64
	Max   float64
	Value float64
	Step  float64

	TrackColor color.Color // nil → theme.Green

	OnChanged     func(float64)
	OnChangeEnded func(float64)

	disabled bool
}

func NewVTSlider(min, max float64) *VTSlider {
	s := &VTSlider{Min: min, Max: max}
	s.ExtendBaseWidget(s)
	return s
}

func (s *VTSlider) Enable() {
	s.disabled = false
	s.Refresh()
}

func (s *VTSlider) Disable() {
	s.disabled = true
	s.Refresh()
}

func (s *VTSlider) Disabled() bool {
	return s.disabled
}

func (s *VTSlider) SetValue(v float64) {
	v = math.Max(s.Min, math.Min(s.Max, v))
	if s.Step > 0 {
		steps := math.Round((v - s.Min) / s.Step)
		v = s.Min + steps*s.Step
		v = math.Max(s.Min, math.Min(s.Max, v))
	}
	if s.Value == v {
		return
	}
	s.Value = v
	s.Refresh()
	if s.OnChanged != nil {
		s.OnChanged(v)
	}
}

func (s *VTSlider) Tapped(e *fyne.PointEvent) {
	if s.disabled {
		return
	}
	s.setFromX(e.Position.X)
	if s.OnChangeEnded != nil {
		s.OnChangeEnded(s.Value)
	}
}

func (s *VTSlider) Dragged(e *fyne.DragEvent) {
	if s.disabled {
		return
	}
	s.setFromX(e.Position.X)
}

func (s *VTSlider) DragEnd() {
	if s.OnChangeEnded != nil {
		s.OnChangeEnded(s.Value)
	}
}

func (s *VTSlider) setFromX(x float32) {
	w := s.Size().Width
	if w <= 0 {
		return
	}
	ratio := math.Max(0, math.Min(1, float64(x)/float64(w)))
	s.SetValue(s.Min + ratio*(s.Max-s.Min))
}

func (s *VTSlider) vtTrackColor() color.Color {
	if s.TrackColor != nil {
		return s.TrackColor
	}
	return Green
}

func (s *VTSlider) CreateRenderer() fyne.WidgetRenderer {
	tc := s.vtTrackColor()
	track := canvas.NewRectangle(tc)
	thumb := canvas.NewCircle(color.Transparent)
	thumb.StrokeColor = tc
	thumb.StrokeWidth = 1.5
	return &vtSliderRenderer{s: s, track: track, thumb: thumb}
}

// vtSliderRenderer -------------------------------------------------------

type vtSliderRenderer struct {
	s     *VTSlider
	track *canvas.Rectangle
	thumb *canvas.Circle
}

func (r *vtSliderRenderer) MinSize() fyne.Size {
	return fyne.NewSize(40, VTSliderMinHeight)
}

func (r *vtSliderRenderer) Layout(size fyne.Size) {
	mid := size.Height / 2
	r.track.Move(fyne.NewPos(0, mid-VTSliderTrackH/2))
	r.track.Resize(fyne.NewSize(size.Width, VTSliderTrackH))

	ratio := float32(0)
	if r.s.Max > r.s.Min {
		ratio = float32((r.s.Value - r.s.Min) / (r.s.Max - r.s.Min))
	}
	tx := ratio*size.Width - VTSliderThumbD/2
	r.thumb.Move(fyne.NewPos(tx, mid-VTSliderThumbD/2))
	r.thumb.Resize(fyne.NewSize(VTSliderThumbD, VTSliderThumbD))
}

func (r *vtSliderRenderer) Refresh() {
	tc := r.s.vtTrackColor()
	if r.s.disabled {
		tc = BorderDim
	}
	r.track.FillColor = tc
	r.thumb.StrokeColor = tc
	r.track.Refresh()
	r.thumb.Refresh()
	r.Layout(r.s.Size())
}

func (r *vtSliderRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.track, r.thumb}
}

func (r *vtSliderRenderer) Destroy() {}

// VTProgressBar ----------------------------------------------------------

// VTProgressBar renders a thin styled progress bar (track + filled portion).
// Use instead of widget.ProgressBar in module views.
// Not for use in the queue module where the stock widget is intentional.
type VTProgressBar struct {
	widget.BaseWidget
	Min   float64
	Max   float64
	Value float64

	TrackColor color.Color // filled portion; nil → theme.Green
}

func NewVTProgressBar() *VTProgressBar {
	p := &VTProgressBar{Min: 0, Max: 1}
	p.ExtendBaseWidget(p)
	return p
}

func (p *VTProgressBar) SetValue(v float64) {
	p.Value = math.Max(p.Min, math.Min(p.Max, v))
	p.Refresh()
}

func (p *VTProgressBar) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(Border)
	fill := canvas.NewRectangle(Green)
	return &vtProgressRenderer{p: p, bg: bg, fill: fill}
}

type vtProgressRenderer struct {
	p    *VTProgressBar
	bg   *canvas.Rectangle
	fill *canvas.Rectangle
}

func (r *vtProgressRenderer) MinSize() fyne.Size {
	return fyne.NewSize(40, 6)
}

func (r *vtProgressRenderer) Layout(size fyne.Size) {
	r.bg.Move(fyne.NewPos(0, 0))
	r.bg.Resize(size)

	ratio := float32(0)
	if r.p.Max > r.p.Min {
		ratio = float32((r.p.Value - r.p.Min) / (r.p.Max - r.p.Min))
	}
	r.fill.Move(fyne.NewPos(0, 0))
	r.fill.Resize(fyne.NewSize(ratio*size.Width, size.Height))
}

func (r *vtProgressRenderer) Refresh() {
	tc := r.p.TrackColor
	if tc == nil {
		tc = Green
	}
	r.fill.FillColor = tc
	r.bg.FillColor = Border
	r.bg.Refresh()
	r.fill.Refresh()
	r.Layout(r.p.Size())
}

func (r *vtProgressRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.fill}
}

func (r *vtProgressRenderer) Destroy() {}
