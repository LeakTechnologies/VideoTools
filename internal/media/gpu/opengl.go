//go:build native_media

package gpu

import (
	"fmt"
	"image"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	vtheme "git.leaktechnologies.dev/leak_technologies/VideoTools/internal/theme"
)

const (
	controlBarHeight = 48
	controlAlpha     = 0xCC
)

var (
	controlBarBG = color.RGBA{R: 0x19, G: 0x1F, B: 0x35, A: controlAlpha}
	vtGreen      = color.RGBA{R: 0x4C, G: 0xE8, B: 0x70, A: 0xFF}
	sliderFill   = color.RGBA{R: 0x4C, G: 0xE8, B: 0x70, A: 0xFF}
)

type VideoRendererGL struct {
	*VideoRenderer

	window fyne.Window
	ppi    float64
}

func NewVideoRendererGL(window fyne.Window) *VideoRendererGL {
	v := &VideoRendererGL{
		VideoRenderer: NewVideoRenderer(),
		window:        window,
	}
	return v
}

func (v *VideoRendererGL) CreateRenderer() fyne.WidgetRenderer {
	return &videoRendererGLRenderer{VideoRendererGL: v}
}

type videoRendererGLRenderer struct {
	*VideoRendererGL
}

func (r *videoRendererGLRenderer) Objects() []fyne.CanvasObject {
	return nil
}

func (r *videoRendererGLRenderer) Layout(fyne.Size) {
}

func (r *videoRendererGLRenderer) MinSize() fyne.Size {
	return fyne.NewSize(320, 180)
}

func (r *videoRendererGLRenderer) Refresh() {
}

func (r *videoRendererGLRenderer) Destroy() {
}

func (v *VideoRendererGL) MakeCurrent() error {
	return nil
}

func (v *VideoRendererGL) SwapBuffers() error {
	return nil
}

func (v *VideoRendererGL) IsAvailable() bool {
	return false
}

func (v *VideoRendererGL) Name() string {
	return "OpenGL"
}

func InitGL() error {
	return nil
}

type GPUTexture struct {
	id       uint32
	width    int
	height   int
	pbo      uint32
	pboIndex int
}

func NewGPUTexture(id uint32, width, height int) *GPUTexture {
	return &GPUTexture{
		id:     id,
		width:  width,
		height: height,
	}
}

func (t *GPUTexture) Upload(img *image.RGBA) error {
	if img == nil {
		return fmt.Errorf("nil image")
	}
	return nil
}

func (t *GPUTexture) UploadBGRA(data []byte, width, height int) error {
	if len(data) == 0 {
		return fmt.Errorf("empty data")
	}
	return nil
}

func (t *GPUTexture) Width() int {
	return t.width
}

func (t *GPUTexture) Height() int {
	return t.height
}

func (t *GPUTexture) Delete() {
	if t.id > 0 {
	}
}

type VideoPlayerGPU struct {
	BaseWidget fyne.Window
	renderer   Renderer
	texture    Texture
	frame      *image.RGBA
	onFrame    func(*image.RGBA)

	playBtn    *widget.Button
	slider     *vtheme.Slider
	timeLabel  *canvas.Text
	durLabel   *canvas.Text
	volumeBtn  *widget.Button
	controlBar *canvas.Rectangle
	controls   *fyne.Container
	backing    *fyne.Container

	isPlaying    bool
	currentTime  float64
	duration     float64
	volume       float64
	showControls bool
	suppressSeek bool // true while SetCurrentTime updates the slider programmatically

	onPlay         func()
	onPause        func()
	onSeek         func(float64)
	onVolumeChange func(float64)

	shortcutsEnabled bool
}

func NewVideoPlayerGPU() *VideoPlayerGPU {
	v := &VideoPlayerGPU{
		showControls:     true,
		currentTime:      0,
		duration:         0,
		volume:           1.0,
		isPlaying:        false,
		shortcutsEnabled: true,
	}
	v.buildUI()
	return v
}

func (v *VideoPlayerGPU) buildUI() {
	th := fyne.CurrentApp().Settings().Theme()
	v.playBtn = widget.NewButtonWithIcon("", th.Icon(theme.IconNameMediaPlay), v.togglePlay)
	v.playBtn.Importance = widget.LowImportance

	v.slider = vtheme.MakeSlider(0, 100)
	v.slider.OnChanged = func(pos float64) {
		if v.suppressSeek {
			return
		}
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

	v.volumeBtn = widget.NewButtonWithIcon("", th.Icon(theme.IconNameVolumeUp), v.toggleMute)
	v.volumeBtn.Importance = widget.LowImportance

	v.controlBar = canvas.NewRectangle(controlBarBG)

	timeContainer := container.NewHBox(
		v.timeLabel,
		widget.NewLabel("/"),
		v.durLabel,
	)

	controlRow := container.NewHBox(
		v.playBtn,
		layout.NewSpacer(),
		v.slider,
		layout.NewSpacer(),
		timeContainer,
		layout.NewSpacer(),
		v.volumeBtn,
	)

	v.controls = container.NewStack(
		canvas.NewRectangle(color.Transparent),
		container.NewPadded(controlRow),
	)

	v.backing = container.NewMax(
		canvas.NewRectangle(color.Black),
		v.controlBar,
		v.controls,
	)
}

func (v *VideoPlayerGPU) CreateRenderer() fyne.WidgetRenderer {
	return &videoPlayerGPURenderer{VideoPlayerGPU: v}
}

func (v *VideoPlayerGPU) SetFrame(img *image.RGBA) {
	v.frame = img
	if v.onFrame != nil {
		v.onFrame(img)
	}
}

func (v *VideoPlayerGPU) SetDuration(d float64) {
	v.duration = d
	v.updateTimeLabels()
}

func (v *VideoPlayerGPU) SetCurrentTime(t float64) {
	v.currentTime = t
	v.updateTimeLabels()
	if v.duration > 0 {
		v.suppressSeek = true
		v.slider.SetValue((t / v.duration) * 100)
		v.suppressSeek = false
	}
}

func (v *VideoPlayerGPU) SetPlaying(playing bool) {
	v.isPlaying = playing
	if v.playBtn != nil {
		th := fyne.CurrentApp().Settings().Theme()
		if playing {
			v.playBtn.SetIcon(th.Icon(theme.IconNameMediaPause))
		} else {
			v.playBtn.SetIcon(th.Icon(theme.IconNameMediaPlay))
		}
	}
}

func (v *VideoPlayerGPU) SetVolume(vol float64) {
	v.volume = vol
	if v.volumeBtn != nil {
		th := fyne.CurrentApp().Settings().Theme()
		if vol <= 0 {
			v.volumeBtn.SetIcon(th.Icon(theme.IconNameVolumeMute))
		} else if vol < 0.5 {
			v.volumeBtn.SetIcon(th.Icon(theme.IconNameVolumeDown))
		} else {
			v.volumeBtn.SetIcon(th.Icon(theme.IconNameVolumeUp))
		}
	}
}

func (v *VideoPlayerGPU) SetLoading(loading bool) {
}

func (v *VideoPlayerGPU) togglePlay() {
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

func (v *VideoPlayerGPU) toggleMute() {
	if v.volume > 0 {
		v.SetVolume(0)
	} else {
		v.SetVolume(1.0)
	}
	if v.onVolumeChange != nil {
		v.onVolumeChange(v.volume)
	}
}

func (v *VideoPlayerGPU) updateTimeLabels() {
	if v.timeLabel != nil {
		v.timeLabel.Text = formatTimeGPU(v.currentTime)
	}
	if v.durLabel != nil {
		v.durLabel.Text = formatTimeGPU(v.duration)
	}
}

func (v *VideoPlayerGPU) OnPlay(cb func()) {
	v.onPlay = cb
}

func (v *VideoPlayerGPU) OnPause(cb func()) {
	v.onPause = cb
}

func (v *VideoPlayerGPU) OnSeek(cb func(float64)) {
	v.onSeek = cb
}

func (v *VideoPlayerGPU) OnVolumeChange(cb func(float64)) {
	v.onVolumeChange = cb
}

func (v *VideoPlayerGPU) NativeCanvas() fyne.CanvasObject {
	return v.backing
}

func (v *VideoPlayerGPU) MouseIn() {
	v.showControls = true
}

func (v *VideoPlayerGPU) MouseOut() {
	v.showControls = false
}

func (v *VideoPlayerGPU) ControlsVisible() bool {
	return v.showControls
}

func (v *VideoPlayerGPU) EnableShortcuts(enabled bool) {
	v.shortcutsEnabled = enabled
}

func (v *VideoPlayerGPU) HandleKey(key string) bool {
	if !v.shortcutsEnabled {
		return false
	}

	switch key {
	case "Space":
		v.togglePlay()
		return true
	case "Left":
		if v.onSeek != nil {
			target := v.currentTime - 5
			if target < 0 {
				target = 0
			}
			v.onSeek(target)
		}
		return true
	case "Right":
		if v.onSeek != nil {
			target := v.currentTime + 5
			if target > v.duration {
				target = v.duration
			}
			v.onSeek(target)
		}
		return true
	case "Up":
		v.SetVolume(v.volume + 0.1)
		if v.onVolumeChange != nil {
			v.onVolumeChange(v.volume)
		}
		return true
	case "Down":
		v.SetVolume(v.volume - 0.1)
		if v.onVolumeChange != nil {
			v.onVolumeChange(v.volume)
		}
		return true
	case "M":
		v.toggleMute()
		return true
	case "F":
		return true
	}

	return false
}

func formatTimeGPU(seconds float64) string {
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	s := int(seconds) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

type videoPlayerGPURenderer struct {
	*VideoPlayerGPU
}

func (r *videoPlayerGPURenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.VideoPlayerGPU.backing}
}

func (r *videoPlayerGPURenderer) Layout(size fyne.Size) {
	r.VideoPlayerGPU.backing.Resize(size)

	barHeight := float32(controlBarHeight)
	if !r.showControls {
		barHeight = 0
	}

	r.VideoPlayerGPU.controlBar.Resize(fyne.NewSize(size.Width, barHeight))
	r.VideoPlayerGPU.controlBar.Move(fyne.NewPos(0, size.Height-barHeight))

	r.VideoPlayerGPU.controls.Resize(fyne.NewSize(size.Width, barHeight))
	r.VideoPlayerGPU.controls.Move(fyne.NewPos(0, size.Height-barHeight))
}

func (r *videoPlayerGPURenderer) MinSize() fyne.Size {
	return fyne.NewSize(320, 180)
}

func (r *videoPlayerGPURenderer) Refresh() {
}

func (r *videoPlayerGPURenderer) Destroy() {
}
