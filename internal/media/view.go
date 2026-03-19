//go:build native_media

package media

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

const (
	dividerWidth     = 4
	vtGreen          = 0x4CE870
	hoverPadding     = 8
	controlBarHeight = 48
	controlAlpha     = 0xCC
)

var (
	dividerColor      = color.RGBA{R: 0x4C, G: 0xE8, B: 0x70, A: 0xFF}
	dividerHoverColor = color.RGBA{R: 0x7F, G: 0xFF, B: 0xA0, A: 0xFF}
	controlBarBG      = color.RGBA{R: 0x19, G: 0x1F, B: 0x35, A: controlAlpha}
	sliderFill        = color.RGBA{R: 0x4C, G: 0xE8, B: 0x70, A: 0xFF}
	sliderBackground  = color.RGBA{R: 0x40, G: 0x40, B: 0x50, A: 0x80}
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

type VideoPlayer struct {
	widget.BaseWidget
	source *image.RGBA

	playBtn    *widget.Button
	slider     *widget.Slider
	timeLabel  *canvas.Text
	durLabel   *canvas.Text
	volumeBtn  *widget.Button
	controls   *fyne.Container
	controlBar *canvas.Rectangle

	isPlaying   bool
	currentTime float64
	duration    float64
	volume      float64

	onPlay         func()
	onPause        func()
	onSeek         func(float64)
	onVolumeChange func(float64)

	showControls bool
	mouseInView  bool
}

func NewVideoPlayer() *VideoPlayer {
	v := &VideoPlayer{
		showControls: true,
		currentTime:  0,
		duration:     0,
		volume:       1.0,
		isPlaying:    false,
	}
	v.ExtendBaseWidget(v)
	v.buildControls()
	return v
}

func (v *VideoPlayer) buildControls() {
	v.playBtn = widget.NewButton("▶", v.togglePlay)
	v.playBtn.Importance = widget.LowImportance
	v.playBtn.Resize(fyne.NewSize(36, 36))

	v.slider = widget.NewSlider(0, 100)
	v.slider.OnChanged = func(pos float64) {
		if v.duration > 0 {
			target := (pos / 100.0) * v.duration
			v.currentTime = target
			if v.onSeek != nil {
				v.onSeek(target)
			}
		}
	}

	v.timeLabel = canvas.NewText("00:00:00", color.White)
	v.timeLabel.TextSize = 12

	v.durLabel = canvas.NewText("00:00:00", color.White)
	v.durLabel.TextSize = 12

	v.volumeBtn = widget.NewButton("🔊", v.toggleMute)
	v.volumeBtn.Importance = widget.LowImportance
	v.volumeBtn.Resize(fyne.NewSize(36, 36))

	controlRow := container.NewHBox(
		v.playBtn,
		widget.NewLabel(""),
		v.timeLabel,
		v.slider,
		v.durLabel,
		v.volumeBtn,
	)

	v.controlBar = canvas.NewRectangle(controlBarBG)
	v.controlBar.CornerRadius = 0

	v.controls = container.NewStack(
		canvas.NewRectangle(color.Transparent),
		container.NewPadded(container.NewBorder(nil, nil, nil, nil, controlRow)),
	)

	_ = layout.NewBorderLayout(v.controls, nil, nil, nil)
}

func (v *VideoPlayer) CreateRenderer() fyne.WidgetRenderer {
	return &videoPlayerRenderer{VideoPlayer: v}
}

func (v *VideoPlayer) SetFrame(img *image.RGBA) {
	v.source = img
	v.Refresh()
}

func (v *VideoPlayer) SetDuration(d float64) {
	v.duration = d
	v.updateTimeLabels()
}

func (v *VideoPlayer) SetCurrentTime(t float64) {
	v.currentTime = t
	v.updateTimeLabels()
	if v.duration > 0 {
		v.slider.SetValue((t / v.duration) * 100)
	}
}

func (v *VideoPlayer) SetPlaying(playing bool) {
	v.isPlaying = playing
	if v.playBtn != nil {
		if playing {
			v.playBtn.SetText("⏸")
		} else {
			v.playBtn.SetText("▶")
		}
	}
}

func (v *VideoPlayer) SetVolume(vol float64) {
	v.volume = vol
	if v.volumeBtn != nil {
		if vol <= 0 {
			v.volumeBtn.SetText("🔇")
		} else if vol < 0.5 {
			v.volumeBtn.SetText("🔉")
		} else {
			v.volumeBtn.SetText("🔊")
		}
	}
}

func (v *VideoPlayer) togglePlay() {
	if v.isPlaying {
		if v.onPause != nil {
			v.onPause()
		}
	} else {
		if v.onPlay != nil {
			v.onPlay()
		}
	}
}

func (v *VideoPlayer) toggleMute() {
	if v.volume > 0 {
		v.SetVolume(0)
	} else {
		v.SetVolume(1.0)
	}
	if v.onVolumeChange != nil {
		v.onVolumeChange(v.volume)
	}
}

func (v *VideoPlayer) updateTimeLabels() {
	if v.timeLabel != nil {
		v.timeLabel.Text = formatVideoTime(v.currentTime)
	}
	if v.durLabel != nil {
		v.durLabel.Text = formatVideoTime(v.duration)
	}
}

func (v *VideoPlayer) OnPlay(cb func()) {
	v.onPlay = cb
}

func (v *VideoPlayer) OnPause(cb func()) {
	v.onPause = cb
}

func (v *VideoPlayer) OnSeek(cb func(float64)) {
	v.onSeek = cb
}

func (v *VideoPlayer) OnVolumeChange(cb func(float64)) {
	v.onVolumeChange = cb
}

func (v *VideoPlayer) MouseIn(ev *desktop.MouseEvent) {
	v.mouseInView = true
	v.showControls = true
	v.Refresh()
}

func (v *VideoPlayer) MouseOut() {
	v.mouseInView = false
	v.showControls = false
	v.Refresh()
}

func (v *VideoPlayer) Tapped(ev *fyne.PointEvent) {
	v.togglePlay()
}

func formatVideoTime(seconds float64) string {
	t := time.Duration(seconds * float64(time.Second))
	h := int(t.Hours())
	m := int(t.Minutes()) % 60
	s := int(t.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

type videoPlayerRenderer struct {
	*VideoPlayer
	raster *canvas.Raster
}

func (r *videoPlayerRenderer) Objects() []fyne.CanvasObject {
	if r.raster == nil {
		r.raster = canvas.NewRaster(r.VideoPlayer.draw)
	}
	return []fyne.CanvasObject{r.raster, r.VideoPlayer.controlBar, r.VideoPlayer.controls}
}

func (r *videoPlayerRenderer) Layout(size fyne.Size) {
	r.raster.Resize(size)

	barHeight := float32(controlBarHeight)
	if !r.showControls {
		barHeight = 0
	}

	r.VideoPlayer.controlBar.Resize(fyne.NewSize(size.Width, barHeight))
	r.VideoPlayer.controlBar.Move(fyne.NewPos(0, size.Height-barHeight))

	r.VideoPlayer.controls.Resize(fyne.NewSize(size.Width, barHeight))
	r.VideoPlayer.controls.Move(fyne.NewPos(0, size.Height-barHeight))

	if r.showControls {
		r.VideoPlayer.controlBar.Show()
		r.VideoPlayer.controls.Show()
	} else {
		r.VideoPlayer.controlBar.Hide()
		r.VideoPlayer.controls.Hide()
	}
}

func (r *videoPlayerRenderer) Refresh() {
	r.raster.Refresh()
}

func (r *videoPlayerRenderer) Destroy() {
}

func (r *videoPlayerRenderer) MinSize() fyne.Size {
	return fyne.NewSize(320, 180)
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

	availableH := h
	if v.showControls {
		availableH = h - controlBarHeight
	}

	scaleX := float64(w) / float64(srcW)
	scaleY := float64(availableH) / float64(srcH)
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}

	newW := int(float64(srcW) * scale)
	newH := int(float64(srcH) * scale)

	offsetX := (w - newW) / 2
	offsetY := (availableH - newH) / 2

	img := image.NewRGBA(image.Rect(0, 0, w, availableH))
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
