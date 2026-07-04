//go:build native_media

package media

import (
	"image"
	"image/color"
	"image/draw"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/LeakTechnologies/VideoTools/internal/smpte"
)

const (
	dividerWidth  = 4
	hoverPadding  = 8
)

var (
	dividerColor      = color.RGBA{R: 0x4C, G: 0xE8, B: 0x70, A: 0xFF}
	dividerHoverColor = color.RGBA{R: 0x7F, G: 0xFF, B: 0xA0, A: 0xFF}
)

type SplitView struct {
	widget.BaseWidget
	leftImg       *canvas.Image
	rightImg      *canvas.Image
	divider       float32
	isDragging    bool
	isHovering    bool
	leftSource    *image.RGBA
	rightSource   *image.RGBA
	onDividerMove func(float32)
	leftIdleText  string
	rightIdleText string
}

func NewSplitView() *SplitView {
	s := &SplitView{
		divider:       0.5,
		leftIdleText:  "DRAG TO LOAD VIDEO",
		rightIdleText: "NO SOURCE",
	}
	s.leftImg = canvas.NewImageFromImage(nil)
	s.rightImg = canvas.NewImageFromImage(nil)
	s.ExtendBaseWidget(s)
	return s
}

func (s *SplitView) SetIdleText(left, right string) {
	s.leftIdleText = left
	s.rightIdleText = right
}

func (s *SplitView) CreateRenderer() fyne.WidgetRenderer {
	return &splitViewRenderer{SplitView: s}
}

type splitViewRenderer struct {
	*SplitView
	raster *canvas.Raster
}

func (r *splitViewRenderer) Objects() []fyne.CanvasObject {
	if r.raster == nil {
		r.raster = canvas.NewRaster(r.SplitView.draw)
	}
	return []fyne.CanvasObject{r.raster}
}

func (r *splitViewRenderer) MinSize() fyne.Size {
	return fyne.NewSize(640, 480)
}

func (r *splitViewRenderer) Layout(size fyne.Size) {
	r.raster.Resize(size)
}

func (r *splitViewRenderer) Refresh() {
	r.raster.Refresh()
}

func (r *splitViewRenderer) Destroy() {
}

func (s *SplitView) SetFrames(left, right *image.RGBA) {
	s.leftSource = left
	s.rightSource = right
	s.Refresh()
}

func (s *SplitView) SetDivider(pos float32) {
	if pos < 0 {
		pos = 0
	}
	if pos > 1 {
		pos = 1
	}
	s.divider = pos
	s.Refresh()
}

func (s *SplitView) SetOnDividerMove(cb func(float32)) {
	s.onDividerMove = cb
}

func (s *SplitView) MouseMoved(ev *desktop.MouseEvent) {
	size := s.Size()
	if size.Width <= 0 {
		return
	}

	splitX := float32(size.Width) * s.divider
	hoverStart := splitX - hoverPadding
	hoverEnd := splitX + dividerWidth + hoverPadding

	isHovering := ev.Position.X >= hoverStart && ev.Position.X <= hoverEnd

	if isHovering != s.isHovering {
		s.isHovering = isHovering
		s.Refresh()
	}
}

func (s *SplitView) MouseIn(ev *desktop.MouseEvent) {
}

func (s *SplitView) MouseOut() {
	s.isDragging = false
}

func (s *SplitView) Dragged(ev *fyne.DragEvent) {
	if !s.isDragging {
		s.isDragging = true
	}
	size := s.Size()
	if size.Width > 0 {
		pos := ev.Position.X / size.Width
		s.SetDivider(pos)
		if s.onDividerMove != nil {
			s.onDividerMove(pos)
		}
	}
}

func (s *SplitView) DragEnd() {
	s.isDragging = false
}

func (s *SplitView) draw(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	splitX := int(float32(w) * s.divider)

	if s.leftSource != nil {
		leftRect := image.Rect(0, 0, splitX, h)
		draw.Draw(img, leftRect, s.leftSource, image.Point{}, draw.Src)
	} else {
		bars := smpte.DrawBars(splitX, h, s.leftIdleText)
		draw.Draw(img, image.Rect(0, 0, splitX, h), bars, image.Point{}, draw.Src)
	}

	rightX := splitX + dividerWidth
	rightW := w - rightX
	if rightW > 0 {
		if s.rightSource != nil {
			rightRect := image.Rect(rightX, 0, w, h)
			srcX := 0
			if s.leftSource != nil {
				srcX = splitX
			}
			draw.Draw(img, rightRect, s.rightSource, image.Point{X: srcX}, draw.Src)
		} else {
			bars := smpte.DrawBars(rightW, h, s.rightIdleText)
			draw.Draw(img, image.Rect(rightX, 0, w, h), bars, image.Point{}, draw.Src)
		}
	}

	drawColor := dividerColor
	if s.isHovering || s.isDragging {
		drawColor = dividerHoverColor
	}

	for x := splitX; x < splitX+dividerWidth && x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, drawColor)
		}
	}

	return img
}