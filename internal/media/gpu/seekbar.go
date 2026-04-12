//go:build native_media

package gpu

import (
	"fmt"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	thumbnailInterval = 10.0
	thumbnailWidth    = 160
	thumbnailHeight   = 90
	maxThumbnails     = 100
)

type ThumbnailCache struct {
	mu         sync.RWMutex
	thumbnails map[int64]string
	paths      []int64
}

func NewThumbnailCache() *ThumbnailCache {
	return &ThumbnailCache{
		thumbnails: make(map[int64]string),
		paths:      make([]int64, 0, maxThumbnails),
	}
}

func (c *ThumbnailCache) Add(timestamp int64, path string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.paths) >= maxThumbnails {
		oldest := c.paths[0]
		delete(c.thumbnails, oldest)
		c.paths = c.paths[1:]
	}

	c.thumbnails[timestamp] = path
	c.paths = append(c.paths, timestamp)
}

func (c *ThumbnailCache) Get(timestamp int64) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	path, ok := c.thumbnails[timestamp]
	return path, ok
}

func (c *ThumbnailCache) GetNearest(timestamp int64) (int64, string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var nearest int64
	var nearestPath string
	minDiff := int64(^uint64(0) >> 1)

	for ts, path := range c.thumbnails {
		diff := ts - timestamp
		if diff < 0 {
			diff = -diff
		}
		if diff < minDiff {
			minDiff = diff
			nearest = ts
			nearestPath = path
		}
	}

	return nearest, nearestPath, nearestPath != ""
}

func (c *ThumbnailCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.thumbnails = make(map[int64]string)
	c.paths = c.paths[:0]
}

type SeekBar struct {
	widget.BaseWidget

	slider       *widget.Slider
	preview      *canvas.Image
	previewLabel *canvas.Text
	previewBox   *fyne.Container

	duration   float64
	hoverTime  float64
	isHovering bool
	thumbnails *ThumbnailCache

	onSeek  func(float64)
	onHover func(float64)
}

func NewSeekBar() *SeekBar {
	s := &SeekBar{
		thumbnails: NewThumbnailCache(),
		hoverTime:  -1,
	}
	s.ExtendBaseWidget(s)

	s.slider = widget.NewSlider(0, 100)
	s.slider.OnChanged = s.onSliderChange

	s.preview = canvas.NewImageFromResource(nil)
	s.preview.FillMode = canvas.ImageFillContain
	s.preview.SetMinSize(fyne.NewSize(float32(thumbnailWidth), float32(thumbnailHeight)))
	s.preview.Hide()

	s.previewLabel = canvas.NewText("", theme.ForegroundColor())
	s.previewLabel.TextSize = 10
	s.previewLabel.Alignment = fyne.TextAlignCenter

	s.previewBox = container.NewVBox(
		s.preview,
		s.previewLabel,
	)
	s.previewBox.Hide()

	return s
}

func (s *SeekBar) SetDuration(d float64) {
	s.duration = d
}

func (s *SeekBar) SetValue(v float64) {
	s.slider.SetValue(v)
}

func (s *SeekBar) OnSeek(cb func(float64)) {
	s.onSeek = cb
}

func (s *SeekBar) OnHover(cb func(float64)) {
	s.onHover = cb
}

func (s *SeekBar) AddThumbnail(timestamp float64, path string) {
	s.thumbnails.Add(int64(timestamp), path)
}

func (s *SeekBar) SetThumbnails(cache *ThumbnailCache) {
	s.thumbnails = cache
}

func (s *SeekBar) CreateRenderer() fyne.WidgetRenderer {
	return &seekBarRenderer{SeekBar: s}
}

func (s *SeekBar) onSliderChange(val float64) {
	if s.duration > 0 && s.onSeek != nil {
		target := (val / 100.0) * s.duration
		s.onSeek(target)
	}
}

func (s *SeekBar) MouseMoved(ev *desktop.MouseEvent) {
	size := s.slider.Size()
	if size.Width <= 0 {
		return
	}

	pos := ev.Position.X / size.Width
	if pos < 0 {
		pos = 0
	} else if pos > 1 {
		pos = 1
	}

	s.hoverTime = float64(pos) * s.duration

	if s.onHover != nil {
		s.onHover(s.hoverTime)
	}

	s.updatePreview()
}

func (s *SeekBar) MouseIn(ev *desktop.MouseEvent) {
	s.isHovering = true
	s.previewBox.Show()
	s.Refresh()
}

func (s *SeekBar) MouseOut() {
	s.isHovering = false
	s.previewBox.Hide()
	s.Refresh()
}

func (s *SeekBar) updatePreview() {
	if s.hoverTime < 0 {
		return
	}

	_, path, ok := s.thumbnails.GetNearest(int64(s.hoverTime))
	if ok && path != "" {
		s.preview.Resource = nil
		s.preview.File = path
		s.preview.Refresh()
	}

	s.previewLabel.Text = formatSeekTime(s.hoverTime)
	s.previewLabel.Refresh()
}

func formatSeekTime(seconds float64) string {
	t := time.Duration(seconds * float64(time.Second))
	m := int(t.Minutes())
	s := int(t.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

type seekBarRenderer struct {
	*SeekBar
}

func (r *seekBarRenderer) Objects() []fyne.CanvasObject {
	objects := []fyne.CanvasObject{r.SeekBar.slider}

	if r.SeekBar.isHovering {
		objects = append(objects, r.SeekBar.previewBox)
	}

	return objects
}

func (r *seekBarRenderer) Layout(size fyne.Size) {
	r.SeekBar.slider.Resize(size)

	if r.SeekBar.isHovering {
		previewSize := fyne.NewSize(float32(thumbnailWidth), float32(thumbnailHeight+20))
		r.SeekBar.previewBox.Resize(previewSize)

		sliderPos := r.SeekBar.slider.Position()
		previewX := sliderPos.X - float32(thumbnailWidth)/2
		if previewX < 0 {
			previewX = 0
		}
		if previewX+float32(thumbnailWidth) > size.Width {
			previewX = size.Width - float32(thumbnailWidth)
		}

		r.SeekBar.previewBox.Move(fyne.NewPos(previewX, -float32(thumbnailHeight+25)))
	}
}

func (r *seekBarRenderer) MinSize() fyne.Size {
	return fyne.NewSize(200, 30)
}

func (r *seekBarRenderer) Refresh() {
	r.SeekBar.slider.Refresh()
}

func (r *seekBarRenderer) Destroy() {
}

type SeekBarWithThumbnails struct {
	*SeekBar
	thumbnailer ThumbnailGenerator
}

type ThumbnailGenerator interface {
	Generate(path string, timestamps []float64) error
}

func NewSeekBarWithThumbnails(gen ThumbnailGenerator) *SeekBarWithThumbnails {
	return &SeekBarWithThumbnails{
		SeekBar:     NewSeekBar(),
		thumbnailer: gen,
	}
}

func (s *SeekBarWithThumbnails) LoadThumbnails(videoPath string) error {
	if s.thumbnailer == nil {
		return nil
	}

	timestamps := make([]float64, 0, maxThumbnails)
	duration := s.duration
	for t := 0.0; t < duration; t += thumbnailInterval {
		timestamps = append(timestamps, t)
	}

	return s.thumbnailer.Generate(videoPath, timestamps)
}

type VolumeControl struct {
	widget.BaseWidget

	slider     *widget.Slider
	icon       *widget.Button
	mute       bool
	volume     float64
	prevVolume float64

	onChange func(float64)
}

func NewVolumeControl() *VolumeControl {
	v := &VolumeControl{
		volume: 1.0,
	}
	v.ExtendBaseWidget(v)

	v.slider = widget.NewSlider(0, 1)
	v.slider.SetValue(1.0)
	v.slider.OnChanged = v.onSliderChange

	th := fyne.CurrentApp().Settings().Theme()
	v.icon = widget.NewButtonWithIcon("", th.Icon(theme.IconNameVolumeUp), v.onIconTap)
	v.icon.Importance = widget.LowImportance

	return v
}

func (v *VolumeControl) SetVolume(vol float64) {
	v.volume = vol
	v.slider.SetValue(vol)
	v.updateIcon()
}

func (v *VolumeControl) Volume() float64 {
	return v.volume
}

func (v *VolumeControl) SetMuted(muted bool) {
	v.mute = muted
	v.updateIcon()
}

func (v *VolumeControl) OnChange(cb func(float64)) {
	v.onChange = cb
}

func (v *VolumeControl) CreateRenderer() fyne.WidgetRenderer {
	return &volumeControlRenderer{VolumeControl: v}
}

func (v *VolumeControl) onSliderChange(val float64) {
	v.volume = val
	v.mute = val == 0
	v.updateIcon()
	if v.onChange != nil {
		v.onChange(val)
	}
}

func (v *VolumeControl) onIconTap() {
	if v.volume > 0 {
		v.prevVolume = v.volume
		v.SetVolume(0)
		if v.onChange != nil {
			v.onChange(0)
		}
	} else {
		v.SetVolume(v.prevVolume)
		if v.onChange != nil {
			v.onChange(v.prevVolume)
		}
	}
}

func (v *VolumeControl) updateIcon() {
	th := fyne.CurrentApp().Settings().Theme()
	if v.mute || v.volume == 0 {
		v.icon.SetIcon(th.Icon(theme.IconNameVolumeMute))
	} else if v.volume < 0.5 {
		v.icon.SetIcon(th.Icon(theme.IconNameVolumeDown))
	} else {
		v.icon.SetIcon(th.Icon(theme.IconNameVolumeUp))
	}
}

type volumeControlRenderer struct {
	*VolumeControl
}

func (r *volumeControlRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.VolumeControl.icon, r.VolumeControl.slider}
}

func (r *volumeControlRenderer) Layout(size fyne.Size) {
	iconSize := float32(36)
	sliderWidth := size.Width - iconSize - 10

	r.VolumeControl.icon.Resize(fyne.NewSize(iconSize, iconSize))
	r.VolumeControl.icon.Move(fyne.NewPos(0, (size.Height-iconSize)/2))

	r.VolumeControl.slider.Resize(fyne.NewSize(sliderWidth, size.Height))
	r.VolumeControl.slider.Move(fyne.NewPos(iconSize+10, 0))
}

func (r *volumeControlRenderer) MinSize() fyne.Size {
	return fyne.NewSize(120, 30)
}

func (r *volumeControlRenderer) Refresh() {
	r.VolumeControl.slider.Refresh()
}

func (r *volumeControlRenderer) Destroy() {
}
