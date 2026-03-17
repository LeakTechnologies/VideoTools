//go:build native_media

package media

import (
	"image"
	"image/draw"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
)

// SplitView is a custom Fyne widget for side-by-side video comparison.
type SplitView struct {
	container.Max
	leftImg  *canvas.Image
	rightImg *canvas.Image
	divider  float32 // 0.0 to 1.0
	
	// Data
	leftSource  *image.RGBA
	rightSource *image.RGBA
}

// NewSplitView creates a new split-view renderer.
func NewSplitView() *SplitView {
	s := &SplitView{
		divider: 0.5,
	}
	s.leftImg = canvas.NewImageFromImage(nil)
	s.rightImg = canvas.NewImageFromImage(nil)
	
	// Create a raster-based renderer for high performance
	raster := canvas.NewRaster(s.draw)
	s.Objects = []fyne.CanvasObject{raster}
	
	return s
}

// SetFrames updates the left and right frame data.
func (s *SplitView) SetFrames(left, right *image.RGBA) {
	s.leftSource = left
	s.rightSource = right
	s.Refresh()
}

// SetDivider sets the split position (0.0 to 1.0).
func (s *SplitView) SetDivider(pos float32) {
	if pos < 0 { pos = 0 }
	if pos > 1 { pos = 1 }
	s.divider = pos
	s.Refresh()
}

func (s *SplitView) draw(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	if s.leftSource == nil || s.rightSource == nil {
		return img
	}

	splitX := int(float32(w) * s.divider)

	// Draw left part
	leftRect := image.Rect(0, 0, splitX, h)
	draw.Draw(img, leftRect, s.leftSource, image.Point{}, draw.Src)

	// Draw right part
	rightRect := image.Rect(splitX, 0, w, h)
	draw.Draw(img, rightRect, s.rightSource, image.Point{X: splitX}, draw.Src)

	// Draw divider line (VT Green)
	divColor := image.NewUniform(image.Config{}.ColorModel.Convert(image.Point{X: 0x4C, Y: 0xE8})) // Simplified
	// TODO: Use exact VT Green #4CE870
	
	return img
}

// Mouse movement handling for the divider
func (s *SplitView) MouseMoved(ev *desktop.MouseEvent) {
	// If dragging, update divider...
}

// VideoPlayer is a custom Fyne widget for single video playback.
type VideoPlayer struct {
	container.Max
	source *image.RGBA
	raster *canvas.Raster
}

// NewVideoPlayer creates a new video player renderer.
func NewVideoPlayer() *VideoPlayer {
	v := &VideoPlayer{}
	v.raster = canvas.NewRaster(v.draw)
	v.Objects = []fyne.CanvasObject{v.raster}
	return v
}

// SetFrame updates the current frame data.
func (v *VideoPlayer) SetFrame(img *image.RGBA) {
	v.source = img
	v.Refresh()
}

func (v *VideoPlayer) draw(w, h int) image.Image {
	if v.source == nil {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		return img
	}
	return v.source
}