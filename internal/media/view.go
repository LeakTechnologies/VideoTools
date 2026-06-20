//go:build native_media

package media

import (
	"image"
	"image/color"
	"image/draw"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/smpte"
	vtheme "git.leaktechnologies.dev/leak_technologies/VideoTools/internal/theme"
)

const (
	vtGreen = 0x4CE870
)

var _ fyne.Focusable = (*VideoPlayer)(nil)

type VideoPlayer struct {
	widget.BaseWidget
	source atomic.Pointer[image.RGBA]

	drawBuf  *image.RGBA
	drawBufW int
	drawBufH int

	playBtn        *vtheme.PillIconButton
	slider         *vtheme.Slider
	timeLabel      *canvas.Text
	durLabel       *canvas.Text
	volumeBtn      *vtheme.PillIconButton
	volumeSlider   *vtheme.Slider
	speedBtn       *vtheme.PillButton
	prevChapterBtn *vtheme.PillIconButton
	nextChapterBtn *vtheme.PillIconButton
	subtitleBtn    *vtheme.PillButton
	loadingSpinner *widget.ProgressBarInfinite
	bufferingLabel *widget.Label
	errorLabel     *widget.Label
	errorIndicator *canvas.Circle
	controls       *fyne.Container
	controlBar     *canvas.Rectangle

	isPlaying        bool
	isLoading        bool
	isBuffering      bool
	isSeeking        bool
	subtitlesEnabled bool
	hasError         bool
	errorMessage     string
	currentTime      float64
	duration         float64
	volume           float64
	speed            float64
	frameRate        float64

	thumbnailCache map[int64]*image.RGBA
	thumbnailMu    sync.RWMutex

	chapters     []Chapter
	markerCanvas *canvas.Raster

	currentChapter int

	inPoint  float64
	outPoint float64

	onPlay          func()
	onPause         func()
	onSeek          func(float64)
	onVolumeChange  func(float64)
	onSpeedChange   func(float64)
	onPrevChapter   func()
	onNextChapter   func()
	onSubtitles     func(bool)
	onTapEmpty      func()
	idleText        string
	idleAspectRatio float64

	subtitleBgAlpha int

	showControls          bool
	mouseInView           bool
	minimal               bool
	builtinControlsLocked bool
	suppressSeek          bool
	controlHideTimer      *time.Timer

	osdBg    *canvas.Rectangle
	osdText  *canvas.Text
	osdTimer *time.Timer

	frameTimingBg   *canvas.Rectangle
	frameTimingText *canvas.Text
	showFrameTiming bool

	raster        *canvas.Raster
	currentWidth  int
	currentHeight int
}

func NewVideoPlayer() *VideoPlayer {
	v := &VideoPlayer{
		showControls: true,
		currentTime:  0,
		duration:     0,
		volume:       1.0,
		speed:        1.0,
		isPlaying:    false,
		isLoading:    false,
		chapters:     make([]Chapter, 0),
		idleText:     "DRAG TO LOAD VIDEO",
	}
	v.ExtendBaseWidget(v)
	v.buildControls()
	return v
}

func NewInlineVideoPlayer() *VideoPlayer {
	v := &VideoPlayer{
		showControls: true,
		minimal:      true,
		currentTime:  0,
		duration:     0,
		volume:       1.0,
		speed:        1.0,
		isPlaying:    false,
		isLoading:    false,
		chapters:     make([]Chapter, 0),
		idleText:     "DRAG TO LOAD VIDEO",
	}
	v.ExtendBaseWidget(v)
	v.buildControls()
	return v
}

func (v *VideoPlayer) CreateRenderer() fyne.WidgetRenderer {
	return &videoPlayerRenderer{VideoPlayer: v}
}

func (v *VideoPlayer) SetFrame(img *image.RGBA) {
	logging.Debug(logging.CatPlayer, "SetFrame called: img=%v", img != nil)
	v.source.Store(img)
	if img != nil && (img.Bounds().Dx() == 0 || img.Bounds().Dy() == 0) {
		return
	}
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
		v.suppressSeek = true
		v.slider.SetValue((t / v.duration) * 100)
		v.suppressSeek = false
	}
}

func (v *VideoPlayer) IsPlaying() bool {
	return v.isPlaying
}

func (v *VideoPlayer) CurrentFrame() *image.RGBA {
	return v.source.Load()
}

func (v *VideoPlayer) CurrentTime() float64 {
	return v.currentTime
}

func (v *VideoPlayer) IsSeeking() bool {
	return v.isSeeking
}

func (v *VideoPlayer) FinishSeeking() {
	v.isSeeking = false
	v.Refresh()
}

func (v *VideoPlayer) drawMarkers(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	if v.duration <= 0 {
		return img
	}

	margin := h / 4
	if margin < 2 {
		margin = 2
	}

	hasTrimMarkers := v.outPoint > v.inPoint

	if hasTrimMarkers {
		inX := int(v.inPoint / v.duration * float64(w))
		outX := int(v.outPoint / v.duration * float64(w))
		if inX < 0 {
			inX = 0
		}
		if outX > w {
			outX = w
		}
		regionColor := color.RGBA{R: 0x4C, G: 0xE8, B: 0x70, A: 0x30}
		for x := inX; x < outX; x++ {
			for y := margin; y < h-margin; y++ {
				img.SetRGBA(x, y, regionColor)
			}
		}
	}

	if hasTrimMarkers {
		inX := int(v.inPoint / v.duration * float64(w))
		inMarkerColor := color.RGBA{R: 0xFF, G: 0xA5, B: 0x00, A: 0xFF}
		inMarkerW := 3
		for dx := 0; dx < inMarkerW; dx++ {
			px := inX + dx
			if px < 0 || px >= w {
				continue
			}
			for py := 0; py < h; py++ {
				img.SetRGBA(px, py, inMarkerColor)
			}
		}
	}

	if hasTrimMarkers {
		outX := int(v.outPoint / v.duration * float64(w))
		outMarkerColor := color.RGBA{R: 0xFF, G: 0x45, B: 0x00, A: 0xFF}
		outMarkerW := 3
		for dx := 0; dx < outMarkerW; dx++ {
			px := outX + dx
			if px < 0 || px >= w {
				continue
			}
			for py := 0; py < h; py++ {
				img.SetRGBA(px, py, outMarkerColor)
			}
		}
	}

	if len(v.chapters) > 1 {
		tick := color.RGBA{R: 0x4C, G: 0xE8, B: 0x70, A: 0xCC}
		tickW := 2
		for _, ch := range v.chapters[1:] {
			if ch.StartTime <= 0 {
				continue
			}
			x := int(ch.StartTime / v.duration * float64(w))
			for dx := 0; dx < tickW; dx++ {
				px := x + dx
				if px < 0 || px >= w {
					continue
				}
				for py := margin; py < h-margin; py++ {
					img.SetRGBA(px, py, tick)
				}
			}
		}
	}

	return img
}

func (v *VideoPlayer) SetInPoint(t float64) {
	v.inPoint = t
	v.refreshMarkers()
}

func (v *VideoPlayer) SetOutPoint(t float64) {
	v.outPoint = t
	v.refreshMarkers()
}

func (v *VideoPlayer) GetInPoint() float64 {
	return v.inPoint
}

func (v *VideoPlayer) GetOutPoint() float64 {
	return v.outPoint
}

func (v *VideoPlayer) ClearTrimMarkers() {
	v.inPoint = 0
	v.outPoint = 0
	v.refreshMarkers()
}

func (v *VideoPlayer) refreshMarkers() {
	if v.markerCanvas != nil {
		v.markerCanvas.Refresh()
	}
}

func (v *VideoPlayer) SetIdleText(text string) {
	v.idleText = text
}

func (v *VideoPlayer) SetIdleAspectRatio(ratio float64) {
	if ratio <= 0 {
		ratio = 4.0 / 3.0
	}
	v.idleAspectRatio = ratio
}

func (v *VideoPlayer) IdleAspectRatio() float64 {
	if v.idleAspectRatio <= 0 {
		return 4.0 / 3.0
	}
	return v.idleAspectRatio
}

type videoPlayerRenderer struct {
	*VideoPlayer
}

func (r *videoPlayerRenderer) Objects() []fyne.CanvasObject {
	if r.VideoPlayer.raster == nil {
		r.VideoPlayer.raster = canvas.NewRaster(r.VideoPlayer.draw)
	}
	return []fyne.CanvasObject{
		r.VideoPlayer.raster,
		r.VideoPlayer.controlBar,
		r.VideoPlayer.controls,
		r.VideoPlayer.osdBg,
		r.VideoPlayer.osdText,
		r.VideoPlayer.frameTimingBg,
		r.VideoPlayer.frameTimingText,
		r.VideoPlayer.loadingSpinner,
		r.VideoPlayer.bufferingLabel,
		r.VideoPlayer.errorIndicator,
		r.VideoPlayer.errorLabel,
	}
}

func (r *videoPlayerRenderer) Layout(size fyne.Size) {
	if r.VideoPlayer.raster != nil {
		r.VideoPlayer.raster.Resize(size)
	}

	barHeight := float32(controlBarHeight)
	if r.minimal {
		barHeight = float32(controlBarHeightMini)
	}

	if r.showControls {
		r.VideoPlayer.controlBar.Resize(fyne.NewSize(size.Width, barHeight))
		r.VideoPlayer.controlBar.Move(fyne.NewPos(0, size.Height-barHeight))
		r.VideoPlayer.controls.Resize(fyne.NewSize(size.Width, barHeight))
		r.VideoPlayer.controls.Move(fyne.NewPos(0, size.Height-barHeight))
		r.VideoPlayer.controlBar.Show()
		r.VideoPlayer.controls.Show()
	} else {
		r.VideoPlayer.controlBar.Hide()
		r.VideoPlayer.controls.Hide()
	}

	const osdW, osdH = float32(160), float32(64)
	osdX := (size.Width - osdW) / 2
	osdY := float32(24)
	r.VideoPlayer.osdBg.Resize(fyne.NewSize(osdW, osdH))
	r.VideoPlayer.osdBg.Move(fyne.NewPos(osdX, osdY))
	r.VideoPlayer.osdText.Resize(fyne.NewSize(osdW, osdH))
	r.VideoPlayer.osdText.Move(fyne.NewPos(osdX, osdY+6))

	if r.VideoPlayer.showFrameTiming {
		ftText := r.VideoPlayer.frameTimingText
		ftBg := r.VideoPlayer.frameTimingBg
		textSize := fyne.MeasureText(ftText.Text, ftText.TextSize, ftText.TextStyle)
		bgW := textSize.Width + 16
		bgH := textSize.Height + 12
		bgX := size.Width - bgW - 8
		bgY := float32(8)
		ftBg.Resize(fyne.NewSize(bgW, bgH))
		ftBg.Move(fyne.NewPos(bgX, bgY))
		ftBg.Show()
		ftText.Resize(fyne.NewSize(bgW, bgH))
		ftText.Move(fyne.NewPos(bgX+6, bgY+4))
		ftText.Show()
	} else {
		r.VideoPlayer.frameTimingBg.Hide()
		r.VideoPlayer.frameTimingText.Hide()
	}

	if r.VideoPlayer.isLoading {
		spinnerW := float32(200)
		spinnerH := float32(6)
		r.VideoPlayer.loadingSpinner.Resize(fyne.NewSize(spinnerW, spinnerH))
		r.VideoPlayer.loadingSpinner.Move(fyne.NewPos((size.Width-spinnerW)/2, (size.Height-spinnerH)/2))
		r.VideoPlayer.loadingSpinner.Show()
	} else {
		r.VideoPlayer.loadingSpinner.Hide()
	}

	if r.VideoPlayer.isBuffering {
		bufW := float32(160)
		bufH := float32(32)
		r.VideoPlayer.bufferingLabel.Resize(fyne.NewSize(bufW, bufH))
		r.VideoPlayer.bufferingLabel.Move(fyne.NewPos((size.Width-bufW)/2, (size.Height-bufH)/2))
		r.VideoPlayer.bufferingLabel.Show()
	} else {
		r.VideoPlayer.bufferingLabel.Hide()
	}

	if r.VideoPlayer.hasError {
		indicatorSize := float32(16)
		labelW := size.Width - float32(40)
		if labelW < float32(100) {
			labelW = float32(100)
		}
		labelH := float32(28)
		errX := (size.Width - labelW) / float32(2)
		errY := (size.Height-labelH)/float32(2) + indicatorSize
		r.VideoPlayer.errorIndicator.Resize(fyne.NewSize(indicatorSize, indicatorSize))
		r.VideoPlayer.errorIndicator.Move(fyne.NewPos(errX, errY-indicatorSize-float32(4)))
		r.VideoPlayer.errorIndicator.Show()
		r.VideoPlayer.errorLabel.Resize(fyne.NewSize(labelW, labelH))
		r.VideoPlayer.errorLabel.Move(fyne.NewPos(errX, errY))
		r.VideoPlayer.errorLabel.Show()
	} else {
		r.VideoPlayer.errorIndicator.Hide()
		r.VideoPlayer.errorLabel.Hide()
	}
}

func (r *videoPlayerRenderer) Refresh() {
	if r.VideoPlayer.raster != nil {
		r.VideoPlayer.raster.Refresh()
	}
}

func (r *videoPlayerRenderer) Destroy() {
}

func (r *videoPlayerRenderer) MinSize() fyne.Size {
	return fyne.NewSize(320, 180)
}

func (v *VideoPlayer) scaleNearest(src *image.RGBA, dst *image.RGBA, srcW, srcH, dstW, dstH, offsetX, offsetY int) {
	if dstW == 0 || dstH == 0 {
		return
	}

	scaleX := float64(srcW) / float64(dstW)
	scaleY := float64(srcH) / float64(dstH)

	dstBounds := dst.Bounds()
	dstPix := dst.Pix
	dstStride := dst.Stride
	srcPix := src.Pix
	srcStride := src.Stride

	for y := 0; y < dstH; y++ {
		srcY := int(float64(y) * scaleY)
		if srcY >= srcH {
			srcY = srcH - 1
		}
		dstY := y + offsetY
		if dstY < dstBounds.Min.Y || dstY >= dstBounds.Max.Y {
			continue
		}
		dstRowBase := (dstY - dstBounds.Min.Y) * dstStride
		srcRowBase := srcY * srcStride

		for x := 0; x < dstW; x++ {
			srcX := int(float64(x) * scaleX)
			if srcX >= srcW {
				srcX = srcW - 1
			}
			dstX := x + offsetX
			if dstX < dstBounds.Min.X || dstX >= dstBounds.Max.X {
				continue
			}
			dstOff := dstRowBase + (dstX-dstBounds.Min.X)*4
			srcOff := srcRowBase + srcX*4
			dstPix[dstOff] = srcPix[srcOff]
			dstPix[dstOff+1] = srcPix[srcOff+1]
			dstPix[dstOff+2] = srcPix[srcOff+2]
			dstPix[dstOff+3] = srcPix[srcOff+3]
		}
	}
}

func (v *VideoPlayer) draw(w, h int) image.Image {
	src := v.source.Load()
	if src == nil {
		availableH := h

		targetAspect := v.IdleAspectRatio()
		availableAspect := float64(w) / float64(availableH)

		var smpteW, smpteH int
		var offsetX, offsetY int

		if availableAspect > targetAspect {
			smpteH = availableH
			smpteW = int(float64(smpteH) * targetAspect)
			offsetX = (w - smpteW) / 2
			offsetY = 0
		} else {
			smpteW = w
			smpteH = int(float64(smpteW) / targetAspect)
			offsetX = 0
			offsetY = (availableH - smpteH) / 2
		}

		img := image.NewRGBA(image.Rect(0, 0, w, availableH))
		draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{0x0F, 0x15, 0x29, 0xFF}), image.Point{}, draw.Src)

		overlayText := v.idleText
		if v.isLoading {
			overlayText = "NOW LOADING"
		}
		smpteImg := smpte.DrawBars(smpteW, smpteH, overlayText)
		draw.Draw(img, image.Rect(offsetX, offsetY, offsetX+smpteW, offsetY+smpteH), smpteImg, image.Point{}, draw.Src)

		return img
	}

	srcW := src.Bounds().Dx()
	srcH := src.Bounds().Dy()

	availableH := h

	if v.drawBuf == nil || v.drawBufW != w || v.drawBufH != availableH {
		v.drawBuf = image.NewRGBA(image.Rect(0, 0, w, availableH))
		v.drawBufW = w
		v.drawBufH = availableH
	}

	if srcW == 0 || srcH == 0 {
		draw.Draw(v.drawBuf, v.drawBuf.Bounds(), image.Black, image.Point{}, draw.Src)
		return v.drawBuf
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

	draw.Draw(v.drawBuf, v.drawBuf.Bounds(), image.Black, image.Point{}, draw.Src)
	v.scaleNearest(src, v.drawBuf, srcW, srcH, newW, newH, offsetX, offsetY)

	return v.drawBuf
}