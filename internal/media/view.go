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
)

const (
	dividerWidth = 4
	vtGreen      = 0x4CE870
)

var (
	dividerColor = color.RGBA{R: 0x4C, G: 0xE8, B: 0x70, A: 0xFF}
)

type SplitView struct {
	widget.BaseWidget
	leftImg       *canvas.Image
	rightImg      *canvas.Image
	divider       float32
	isDragging    bool
	leftSource    *image.RGBA
	rightSource   *image.RGBA
	onDividerMove func(float32)
}

func NewSplitView() *SplitView {
	s := &SplitView{
		divider: 0.5,
	}
	s.leftImg = canvas.NewImageFromImage(nil)
	s.rightImg = canvas.NewImageFromImage(nil)
	s.ExtendBaseWidget(s)
	return s
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

	if s.leftSource == nil && s.rightSource == nil {
		return img
	}

	splitX := int(float32(w) * s.divider)

	if s.leftSource != nil {
		leftRect := image.Rect(0, 0, splitX, h)
		draw.Draw(img, leftRect, s.leftSource, image.Point{}, draw.Src)
	}

	if s.rightSource != nil {
		rightSrcX := 0
		rightRect := image.Rect(splitX+dividerWidth, 0, w, h)
		if s.leftSource != nil {
			rightSrcX = splitX
		}
		draw.Draw(img, rightRect, s.rightSource, image.Point{X: rightSrcX}, draw.Src)
	}

	for x := splitX; x < splitX+dividerWidth && x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, dividerColor)
		}
	}

	return img
}

type VideoPlayer struct {
	widget.BaseWidget
	source *image.RGBA
}

func NewVideoPlayer() *VideoPlayer {
	v := &VideoPlayer{}
	v.ExtendBaseWidget(v)
	return v
}

func (v *VideoPlayer) CreateRenderer() fyne.WidgetRenderer {
	return &videoPlayerRenderer{VideoPlayer: v}
}

func (v *VideoPlayer) SetFrame(img *image.RGBA) {
	v.source = img
	v.Refresh()
}

type videoPlayerRenderer struct {
	*VideoPlayer
	raster *canvas.Raster
}

func (r *videoPlayerRenderer) Objects() []fyne.CanvasObject {
	if r.raster == nil {
		r.raster = canvas.NewRaster(r.VideoPlayer.draw)
	}
	return []fyne.CanvasObject{r.raster}
}

func (r *videoPlayerRenderer) Layout(size fyne.Size) {
	r.raster.Resize(size)
}

func (r *videoPlayerRenderer) Refresh() {
	r.raster.Refresh()
}

func (r *videoPlayerRenderer) Destroy() {
}

func (r *videoPlayerRenderer) MinSize() fyne.Size {
	return fyne.NewSize(320, 240)
}

func (v *VideoPlayer) draw(w, h int) image.Image {
	if v.source == nil {
		return image.NewRGBA(image.Rect(0, 0, w, h))
	}

	src := v.source
	srcW := src.Bounds().Dx()
	srcH := src.Bounds().Dy()

	if srcW == 0 || srcH == 0 {
		return image.NewRGBA(image.Rect(0, 0, w, h))
	}

	scaleX := float64(w) / float64(srcW)
	scaleY := float64(h) / float64(srcH)
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}

	newW := int(float64(srcW) * scale)
	newH := int(float64(srcH) * scale)

	offsetX := (w - newW) / 2
	offsetY := (h - newH) / 2

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.Black, image.Point{}, draw.Src)

	for y := 0; y < newH; y++ {
		for x := 0; x < newW; x++ {
			srcX := int(float64(x) / scale)
			srcY := int(float64(y) / scale)
			if srcX >= srcW {
				srcX = srcW - 1
			}
			if srcY >= srcH {
				srcY = srcH - 1
			}
			c := src.At(srcX, srcY)
			img.Set(x+offsetX, y+offsetY, c)
		}
	}

	return img
}
