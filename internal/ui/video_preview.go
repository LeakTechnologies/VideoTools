package ui

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

type VideoPreview struct {
	container        *fyne.Container
	videoPath        string
	duration         float64
	currentTime      float64
	previewImg       *canvas.Image
	loadingIndicator *widget.Label
	icon             *widget.Icon
	mu               sync.RWMutex
	onSeek           func(time float64)
}

func NewVideoPreview() *VideoPreview {
	v := &VideoPreview{
		currentTime: 0,
	}

	darkBg := canvas.NewRectangle(theme.BackgroundColor())

	v.loadingIndicator = widget.NewLabel("Loading preview...")
	v.loadingIndicator.Alignment = fyne.TextAlignCenter

	v.icon = widget.NewIcon(theme.MediaVideoIcon())
	v.icon.Hide()

	v.previewImg = canvas.NewImageFromImage(nil)
	v.previewImg.FillMode = canvas.ImageFillContain
	v.previewImg.Hide()

	v.container = container.NewMax(
		darkBg,
		container.NewCenter(v.loadingIndicator),
		container.NewCenter(v.icon),
		v.previewImg,
	)

	return v
}

func (v *VideoPreview) SetVideo(path string, duration float64) {
	v.mu.Lock()
	v.videoPath = path
	v.duration = duration
	v.currentTime = 0
	v.mu.Unlock()

	v.showLoading(true)
	v.UpdatePreview(0)
}

func (v *VideoPreview) showLoading(show bool) {
	if show {
		v.loadingIndicator.Show()
		v.icon.Hide()
		v.previewImg.Hide()
	} else {
		v.loadingIndicator.Hide()
	}
}

func (v *VideoPreview) SeekTo(time float64) {
	v.mu.Lock()
	v.currentTime = time
	v.mu.Unlock()

	if v.onSeek != nil {
		v.onSeek(time)
	}
}

func (v *VideoPreview) SetOnSeek(callback func(time float64)) {
	v.onSeek = callback
}

func (v *VideoPreview) UpdatePreview(time float64) {
	v.mu.RLock()
	path := v.videoPath
	v.mu.RUnlock()

	if path == "" {
		return
	}

	v.showLoading(true)

	go func() {
		frameData, err := extractPreviewFrame(path, time)
		if err != nil {
			logging.Debug(logging.CatPlayer, "failed to extract preview frame: %v", err)
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				v.icon.Show()
				v.loadingIndicator.Hide()
			}, false)
			return
		}

		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			v.mu.Lock()
			v.previewImg.Resource = fyne.NewStaticResource("preview.png", frameData)
			v.currentTime = time
			v.mu.Unlock()

			v.previewImg.Show()
			v.icon.Hide()
			v.loadingIndicator.Hide()
		}, false)
	}()
}

func (v *VideoPreview) GetContainer() fyne.CanvasObject {
	return v.container
}

func (v *VideoPreview) GetVideoPath() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.videoPath
}

func (v *VideoPreview) GetDuration() float64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.duration
}

func extractPreviewFrame(path string, timestamp float64) ([]byte, error) {
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, "vt_preview_"+randomString(8)+".png")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := []string{
		"-ss", formatTimestamp(timestamp),
		"-i", path,
		"-frames:v", "1",
		"-vf", "scale=640:-2",
		"-q:v", "3",
		"-y",
		tmpFile,
	}

	cmd := exec.CommandContext(ctx, utils.GetFFmpegPath(), args...)
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	defer os.Remove(tmpFile)

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func formatTimestamp(seconds float64) string {
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	s := int(seconds) % 60
	ms := int((seconds - float64(int(seconds))) * 1000)
	return time.Date(0, 0, 0, h, m, s, ms*1000000, time.UTC).Format("15:04:05.000")
}

var randLetters = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
var randMutex sync.Mutex

func randomString(n int) string {
	b := make([]byte, n)
	randMutex.Lock()
	defer randMutex.Unlock()
	for i := range b {
		b[i] = randLetters[time.Now().UnixNano()%int64(len(randLetters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}

func NewStaticResourceFromPNG(name string, pngData []byte) fyne.Resource {
	return fyne.NewStaticResource(name, pngData)
}

func EncodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	err := encoder.Encode(&buf, img)
	return buf.Bytes(), err
}
