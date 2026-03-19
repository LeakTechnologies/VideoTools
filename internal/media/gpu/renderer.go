//go:build native_media

package gpu

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type Texture interface {
	Upload(img *image.RGBA) error
	UploadBGRA(data []byte, width, height int) error
	Width() int
	Height() int
	Delete()
}

type Renderer interface {
	CreateTexture(w, h int) (Texture, error)
	MakeCurrent() error
	SwapBuffers() error
	Delete()
	IsAvailable() bool
	Name() string
}

var (
	ErrNoRenderer     = errors.New("no GPU renderer available")
	ErrTextureCreate  = errors.New("failed to create texture")
	ErrContextLost    = errors.New("GPU context lost")
	ErrNotImplemented = errors.New("method not implemented")
)

type VideoRenderer struct {
	renderer       Renderer
	canvas         *GPUCanvas
	controlBar     *canvas.Rectangle
	controls       *fyne.Container
	playBtn        *widget.Button
	slider         *widget.Slider
	timeLabel      *canvas.Text
	durLabel       *canvas.Text
	volumeBtn      *widget.Button
	loadingSpinner *widget.ProgressBarInfinite

	videoTex     Texture
	frame        *image.RGBA
	isPlaying    bool
	currentTime  float64
	duration     float64
	volume       float64
	showControls bool
	mouseInView  bool

	onPlay         func()
	onPause        func()
	onSeek         func(float64)
	onVolumeChange func(float64)

	backing *fyne.Container
}

func NewVideoRenderer() *VideoRenderer {
	v := &VideoRenderer{
		showControls: true,
		currentTime:  0,
		duration:     0,
		volume:       1.0,
		isPlaying:    false,
	}
	v.canvas = NewGPUCanvas(v.onFrame)
	v.buildUI()
	return v
}

func (v *VideoRenderer) buildUI() {
	v.playBtn = widget.NewButton("▶", v.togglePlay)
	v.playBtn.Importance = widget.LowImportance

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

	v.controlBar = canvas.NewRectangle(color.RGBA{R: 0x19, G: 0x1F, B: 0x35, A: 0xCC})

	controlRow := container.NewHBox(
		v.playBtn,
		layout.NewSpacer(),
		v.timeLabel,
		widget.NewLabel(" / "),
		v.durLabel,
		layout.NewSpacer(),
		v.volumeBtn,
	)

	v.controls = container.NewStack(
		canvas.NewRectangle(color.Transparent),
		container.NewPadded(controlRow),
	)

	v.loadingSpinner = widget.NewProgressBarInfinite()
	v.loadingSpinner.Hide()

	videoContent := container.NewStack(
		v.canvas,
		canvas.NewRectangle(color.Black),
		v.controlBar,
		v.controls,
	)

	v.backing = container.NewMax(videoContent)
}

func (v *VideoRenderer) SetFrame(img *image.RGBA) {
	v.frame = img
	v.canvas.RequestRender(img)
}

func (v *VideoRenderer) SetDuration(d float64) {
	v.duration = d
	v.updateTimeLabels()
}

func (v *VideoRenderer) SetCurrentTime(t float64) {
	v.currentTime = t
	v.updateTimeLabels()
	if v.duration > 0 {
		v.slider.SetValue((t / v.duration) * 100)
	}
}

func (v *VideoRenderer) SetPlaying(playing bool) {
	v.isPlaying = playing
	if v.playBtn != nil {
		if playing {
			v.playBtn.SetText("⏸")
		} else {
			v.playBtn.SetText("▶")
		}
	}
}

func (v *VideoRenderer) SetVolume(vol float64) {
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

func (v *VideoRenderer) SetLoading(loading bool) {
	if loading {
		v.loadingSpinner.Show()
	} else {
		v.loadingSpinner.Hide()
	}
}

func (v *VideoRenderer) togglePlay() {
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

func (v *VideoRenderer) toggleMute() {
	if v.volume > 0 {
		v.SetVolume(0)
	} else {
		v.SetVolume(1.0)
	}
	if v.onVolumeChange != nil {
		v.onVolumeChange(v.volume)
	}
}

func (v *VideoRenderer) updateTimeLabels() {
	if v.timeLabel != nil {
		v.timeLabel.Text = formatTime(v.currentTime)
	}
	if v.durLabel != nil {
		v.durLabel.Text = formatTime(v.duration)
	}
}

func (v *VideoRenderer) OnPlay(cb func()) {
	v.onPlay = cb
}

func (v *VideoRenderer) OnPause(cb func()) {
	v.onPause = cb
}

func (v *VideoRenderer) OnSeek(cb func(float64)) {
	v.onSeek = cb
}

func (v *VideoRenderer) OnVolumeChange(cb func(float64)) {
	v.onVolumeChange = cb
}

func (v *VideoRenderer) NativeCanvas() fyne.CanvasObject {
	return v.backing
}

func (v *VideoRenderer) onFrame(img *image.RGBA) {
	if img == nil || img.Rect.Size().X == 0 {
		return
	}

	w := img.Rect.Size().X
	h := img.Rect.Size().Y

	if v.videoTex == nil || v.videoTex.Width() != w || v.videoTex.Height() != h {
		if v.videoTex != nil {
			v.videoTex.Delete()
		}
		v.videoTex, _ = v.canvas.CreateTexture(w, h)
	}

	if v.videoTex != nil {
		v.videoTex.Upload(img)
	}
}

func (v *VideoRenderer) Texture() Texture {
	return v.videoTex
}

func (v *VideoRenderer) Destroy() {
	if v.videoTex != nil {
		v.videoTex.Delete()
		v.videoTex = nil
	}
}

func (v *VideoRenderer) MouseIn() {
	v.mouseInView = true
	v.showControls = true
}

func (v *VideoRenderer) MouseOut() {
	v.mouseInView = false
	v.showControls = false
}

func (v *VideoRenderer) ControlsVisible() bool {
	return v.showControls
}

func formatTime(seconds float64) string {
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	s := int(seconds) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

type GPUCanvas struct {
	widget.BaseWidget
	onFrame func(*image.RGBA)
	frame   *image.RGBA
	tex     Texture
}

func NewGPUCanvas(onFrame func(*image.RGBA)) *GPUCanvas {
	return &GPUCanvas{
		onFrame: onFrame,
	}
}

func (g *GPUCanvas) CreateRenderer() fyne.WidgetRenderer {
	return &gpuCanvasRenderer{GPUCanvas: g}
}

func (g *GPUCanvas) RequestRender(img *image.RGBA) {
	g.frame = img
	g.Refresh()
}

func (g *GPUCanvas) CreateTexture(w, h int) (Texture, error) {
	return nil, ErrNotImplemented
}

func (g *GPUCanvas) SetTexture(tex Texture) {
	g.tex = tex
}

type gpuCanvasRenderer struct {
	*GPUCanvas
}

func (r *gpuCanvasRenderer) Objects() []fyne.CanvasObject {
	return nil
}

func (r *gpuCanvasRenderer) Layout(fyne.Size) {
}

func (r *gpuCanvasRenderer) MinSize() fyne.Size {
	return fyne.NewSize(320, 180)
}

func (r *gpuCanvasRenderer) Refresh() {
}

func (r *gpuCanvasRenderer) Destroy() {
}

var _ unsafe.Pointer = unsafe.Pointer(nil)
