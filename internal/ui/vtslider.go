package ui

import (
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
	vtheme "git.leaktechnologies.dev/leak_technologies/VideoTools/internal/theme"
)

const (
	sliderTrackH    = float32(2)
	sliderThumbD    = float32(14)
	sliderMinHeight = float32(26)
)

// Slider is a styled slider: thin coloured track + hollow circle thumb.
// Generic enough to be used in any Fyne project.
type Slider struct {
	widget.BaseWidget

	Min   float64
	Max   float64
	Value float64
	Step  float64

	TrackColor color.Color // nil → vtheme.Green

	OnChanged     func(float64)
	OnChangeEnded func(float64)

	disabled bool
}

// MakeSlider constructs a Slider with the given range.
func MakeSlider(min, max float64) *Slider {
	s := &Slider{Min: min, Max: max}
	s.ExtendBaseWidget(s)
	return s
}

func (s *Slider) Enable() {
	s.disabled = false
	s.Refresh()
}

func (s *Slider) Disable() {
	s.disabled = true
	s.Refresh()
}

func (s *Slider) Disabled() bool {
	return s.disabled
}

func (s *Slider) SetValue(v float64) {
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

func (s *Slider) Tapped(e *fyne.PointEvent) {
	if s.disabled {
		return
	}
	s.setFromX(e.Position.X)
	if s.OnChangeEnded != nil {
		s.OnChangeEnded(s.Value)
	}
}

func (s *Slider) Dragged(e *fyne.DragEvent) {
	if s.disabled {
		return
	}
	s.setFromX(e.Position.X)
}

func (s *Slider) DragEnd() {
	if s.OnChangeEnded != nil {
		s.OnChangeEnded(s.Value)
	}
}

func (s *Slider) setFromX(x float32) {
	w := s.Size().Width
	if w <= 0 {
		return
	}
	ratio := math.Max(0, math.Min(1, float64(x)/float64(w)))
	s.SetValue(s.Min + ratio*(s.Max-s.Min))
}

func (s *Slider) trackColor() color.Color {
	if s.TrackColor != nil {
		return s.TrackColor
	}
	return vtheme.Green
}

func (s *Slider) CreateRenderer() fyne.WidgetRenderer {
	tc := s.trackColor()
	track := canvas.NewRectangle(tc)
	thumb := canvas.NewCircle(color.Transparent)
	thumb.StrokeColor = tc
	thumb.StrokeWidth = 1.5
	return &sliderRenderer{s: s, track: track, thumb: thumb}
}

type sliderRenderer struct {
	s     *Slider
	track *canvas.Rectangle
	thumb *canvas.Circle
}

func (r *sliderRenderer) MinSize() fyne.Size {
	return fyne.NewSize(40, sliderMinHeight)
}

func (r *sliderRenderer) Layout(size fyne.Size) {
	mid := size.Height / 2
	r.track.Move(fyne.NewPos(0, mid-sliderTrackH/2))
	r.track.Resize(fyne.NewSize(size.Width, sliderTrackH))

	ratio := float32(0)
	if r.s.Max > r.s.Min {
		ratio = float32((r.s.Value - r.s.Min) / (r.s.Max - r.s.Min))
	}
	tx := ratio*size.Width - sliderThumbD/2
	r.thumb.Move(fyne.NewPos(tx, mid-sliderThumbD/2))
	r.thumb.Resize(fyne.NewSize(sliderThumbD, sliderThumbD))
}

func (r *sliderRenderer) Refresh() {
	tc := r.s.trackColor()
	if r.s.disabled {
		tc = vtheme.BorderDim
	}
	r.track.FillColor = tc
	r.thumb.StrokeColor = tc
	r.track.Refresh()
	r.thumb.Refresh()
	r.Layout(r.s.Size())
}

func (r *sliderRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.track, r.thumb}
}

func (r *sliderRenderer) Destroy() {}

// ProgressBar renders a thin styled progress bar (track + filled portion).
// Use instead of widget.ProgressBar in module views.
// Not for use in the queue module where the stock widget is intentional.
type ProgressBar struct {
	widget.BaseWidget
	Min   float64
	Max   float64
	Value float64

	TrackColor color.Color // filled portion; nil → vtheme.Green
}

// MakeProgressBar constructs a ProgressBar with the default [0, 1] range.
func MakeProgressBar() *ProgressBar {
	p := &ProgressBar{Min: 0, Max: 1}
	p.ExtendBaseWidget(p)
	return p
}

func (p *ProgressBar) SetValue(v float64) {
	p.Value = math.Max(p.Min, math.Min(p.Max, v))
	p.Refresh()
}

func (p *ProgressBar) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(vtheme.Border)
	fill := canvas.NewRectangle(vtheme.Green)
	return &progressBarRenderer{p: p, bg: bg, fill: fill}
}

type progressBarRenderer struct {
	p    *ProgressBar
	bg   *canvas.Rectangle
	fill *canvas.Rectangle
}

func (r *progressBarRenderer) MinSize() fyne.Size {
	return fyne.NewSize(40, 6)
}

func (r *progressBarRenderer) Layout(size fyne.Size) {
	r.bg.Move(fyne.NewPos(0, 0))
	r.bg.Resize(size)

	ratio := float32(0)
	if r.p.Max > r.p.Min {
		ratio = float32((r.p.Value - r.p.Min) / (r.p.Max - r.p.Min))
	}
	r.fill.Move(fyne.NewPos(0, 0))
	r.fill.Resize(fyne.NewSize(ratio*size.Width, size.Height))
}

func (r *progressBarRenderer) Refresh() {
	tc := r.p.TrackColor
	if tc == nil {
		tc = vtheme.Green
	}
	r.fill.FillColor = tc
	r.bg.FillColor = vtheme.Border
	r.bg.Refresh()
	r.fill.Refresh()
	r.Layout(r.p.Size())
}

func (r *progressBarRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.fill}
}

func (r *progressBarRenderer) Destroy() {}
