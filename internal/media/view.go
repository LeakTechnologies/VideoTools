//go:build native_media

package media

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

const (
	dividerWidth         = 4
	vtGreen              = 0x4CE870
	hoverPadding         = 8
	controlBarHeight     = 48
	controlBarHeightMini = 32
	controlAlpha         = 0xCC
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

	playBtn        *widget.Button
	slider         *widget.Slider
	timeLabel      *canvas.Text
	durLabel       *canvas.Text
	volumeBtn      *widget.Button
	speedBtn       *widget.Button
	prevChapterBtn *widget.Button
	nextChapterBtn *widget.Button
	fullscreenBtn  *widget.Button
	pipBtn         *widget.Button
	subtitleBtn    *widget.Button
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
	isFullscreen     bool
	isPiP            bool
	subtitlesEnabled bool
	hasError         bool
	errorMessage     string
	currentTime      float64
	duration         float64
	volume           float64
	speed            float64
	frameRate        float64

	displayFrame  *image.RGBA
	displayWidth  int
	displayHeight int
	frameSeq      uint64
	lastFrameSeq  uint64

	thumbnailCache map[int64]*image.RGBA
	thumbnailMu    sync.RWMutex

	chapters     []Chapter
	chapterMark  []*canvas.Circle
	markerCanvas *canvas.Raster

	currentChapter int

	onPlay          func()
	onPause         func()
	onSeek          func(float64)
	onHover         func(float64)
	onVolumeChange  func(float64)
	onSpeedChange   func(float64)
	onFrameRate     func(float64)
	onPrevChapter   func()
	onNextChapter   func()
	onChapterSelect func(int)
	onFullscreen    func(bool)
	onPiP           func()
	onSubtitles     func(bool)

	subtitleBgAlpha int

	showControls bool
	mouseInView  bool
	minimal      bool

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
		v.isSeeking = true
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

	v.speedBtn = widget.NewButton("1x", v.toggleSpeed)
	v.speedBtn.Importance = widget.LowImportance
	v.speedBtn.Resize(fyne.NewSize(36, 24))

	v.prevChapterBtn = widget.NewButton("⏮", v.prevChapter)
	v.prevChapterBtn.Importance = widget.LowImportance
	v.prevChapterBtn.Resize(fyne.NewSize(36, 24))
	v.prevChapterBtn.Hide()

	v.nextChapterBtn = widget.NewButton("⏭", v.nextChapter)
	v.nextChapterBtn.Importance = widget.LowImportance
	v.nextChapterBtn.Resize(fyne.NewSize(36, 24))
	v.nextChapterBtn.Hide()

	v.loadingSpinner = widget.NewProgressBarInfinite()
	v.loadingSpinner.Hide()

	v.bufferingLabel = widget.NewLabel("Buffering...")
	v.bufferingLabel.TextStyle = fyne.TextStyle{Bold: true}
	v.bufferingLabel.Hide()

	v.errorIndicator = canvas.NewCircle(color.RGBA{R: 0xFF, G: 0x44, B: 0x44, A: 0xFF})
	v.errorIndicator.Hide()

	v.errorLabel = widget.NewLabel("")
	v.errorLabel.TextStyle = fyne.TextStyle{Bold: true}
	v.errorLabel.Hide()

	v.fullscreenBtn = widget.NewButton("⛶", v.toggleFullscreen)
	v.fullscreenBtn.Importance = widget.LowImportance
	v.fullscreenBtn.Resize(fyne.NewSize(36, 24))

	v.pipBtn = widget.NewButton("📺", v.togglePiP)
	v.pipBtn.Importance = widget.LowImportance
	v.pipBtn.Resize(fyne.NewSize(36, 24))

	v.subtitleBtn = widget.NewButton("CC", v.toggleSubtitles)
	v.subtitleBtn.Importance = widget.LowImportance
	v.subtitleBtn.Resize(fyne.NewSize(36, 24))

	v.markerCanvas = canvas.NewRaster(v.drawChapterMarkers)
	seekStack := container.NewStack(v.slider, v.markerCanvas)

	if v.minimal {
		controlRow := container.NewHBox(
			v.playBtn,
			layout.NewSpacer(),
			seekStack,
			layout.NewSpacer(),
		)
		v.controlBar = canvas.NewRectangle(controlBarBG)
		v.controlBar.CornerRadius = 0
		v.controls = container.NewStack(
			canvas.NewRectangle(color.Transparent),
			container.NewPadded(container.NewBorder(nil, nil, nil, nil, controlRow)),
		)
	} else {
		controlRow := container.NewHBox(
			v.playBtn,
			widget.NewLabel(""),
			v.prevChapterBtn,
			v.nextChapterBtn,
			widget.NewLabel(""),
			v.timeLabel,
			seekStack,
			v.durLabel,
			layout.NewSpacer(),
			v.speedBtn,
			v.volumeBtn,
			v.subtitleBtn,
			v.pipBtn,
			v.fullscreenBtn,
		)
		v.controlBar = canvas.NewRectangle(controlBarBG)
		v.controlBar.CornerRadius = 0
		v.controls = container.NewStack(
			canvas.NewRectangle(color.Transparent),
			container.NewPadded(container.NewBorder(nil, nil, nil, nil, controlRow)),
		)
		_ = layout.NewBorderLayout(v.controls, nil, nil, nil)
	}
}

func (v *VideoPlayer) CreateRenderer() fyne.WidgetRenderer {
	return &videoPlayerRenderer{VideoPlayer: v}
}

func (v *VideoPlayer) SetFrame(img *image.RGBA) {
	v.source = img
	if img == nil {
		return
	}

	srcW := img.Bounds().Dx()
	srcH := img.Bounds().Dy()
	if srcW == 0 || srcH == 0 {
		return
	}

	size := v.Size()
	w := int(size.Width)
	h := int(size.Height)
	if w <= 0 || h <= 0 {
		v.Refresh()
		return
	}

	availableH := h
	if v.showControls {
		availableH = h - controlBarHeight
		if availableH < 0 {
			availableH = h
		}
	}

	newW := w
	newH := availableH
	scaleX := float64(w) / float64(srcW)
	scaleY := float64(availableH) / float64(srcH)
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}
	newW = int(float64(srcW) * scale)
	newH = int(float64(srcH) * scale)
	offsetX := (w - newW) / 2
	offsetY := (availableH - newH) / 2

	displayImg := image.NewRGBA(image.Rect(0, 0, w, availableH))
	draw.Draw(displayImg, displayImg.Bounds(), image.Black, image.Point{}, draw.Src)
	v.scaleNearest(img, displayImg, srcW, srcH, newW, newH, offsetX, offsetY)

	if v.raster != nil && v.currentWidth == w && v.currentHeight == availableH {
		v.raster.UpdatePixels(w, availableH, displayImg.Pix)
	} else {
		v.raster = canvas.NewRaster(func(rw, rh int) image.Image {
			return displayImg
		})
		v.currentWidth = w
		v.currentHeight = availableH
		v.Refresh()
	}
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

func (v *VideoPlayer) toggleSpeed() {
	speeds := []float64{0.25, 0.5, 0.75, 1.0, 1.25, 1.5, 2.0}
	found := -1
	for i, s := range speeds {
		if s == v.speed {
			found = i
			break
		}
	}

	nextIdx := (found + 1) % len(speeds)
	v.SetSpeed(speeds[nextIdx])
}

func (v *VideoPlayer) SetLoading(loading bool) {
	v.isLoading = loading
	if v.loadingSpinner != nil {
		if loading {
			v.loadingSpinner.Show()
		} else {
			v.loadingSpinner.Hide()
		}
	}
	v.Refresh()
}

func (v *VideoPlayer) SetBuffering(buffering bool) {
	v.isBuffering = buffering
	if v.bufferingLabel != nil {
		if buffering {
			v.bufferingLabel.Show()
		} else {
			v.bufferingLabel.Hide()
		}
	}
	v.Refresh()
}

func (v *VideoPlayer) IsBuffering() bool {
	return v.isBuffering
}

func (v *VideoPlayer) CurrentFrame() *image.RGBA {
	return v.source
}

func (v *VideoPlayer) CurrentTime() float64 {
	return v.currentTime
}

func (v *VideoPlayer) SetError(message string) {
	v.hasError = true
	v.errorMessage = message
	if v.errorLabel != nil {
		v.errorLabel.SetText(message)
		v.errorLabel.Show()
	}
	if v.errorIndicator != nil {
		v.errorIndicator.Show()
	}
	if v.loadingSpinner != nil {
		v.loadingSpinner.Hide()
	}
	v.Refresh()
}

func (v *VideoPlayer) ClearError() {
	v.hasError = false
	v.errorMessage = ""
	if v.errorLabel != nil {
		v.errorLabel.Hide()
	}
	if v.errorIndicator != nil {
		v.errorIndicator.Hide()
	}
	v.Refresh()
}

func (v *VideoPlayer) HasError() bool {
	return v.hasError
}

func (v *VideoPlayer) IsSeeking() bool {
	return v.isSeeking
}

func (v *VideoPlayer) FinishSeeking() {
	v.isSeeking = false
	v.Refresh()
}

func (v *VideoPlayer) SetSpeed(speed float64) {
	v.speed = speed
	if v.speedBtn != nil {
		if speed == 1.0 {
			v.speedBtn.SetText("1x")
		} else if speed < 1.0 {
			v.speedBtn.SetText(fmt.Sprintf("%.2gx", speed))
		} else {
			v.speedBtn.SetText(fmt.Sprintf("%.1gx", speed))
		}
	}
	if v.onSpeedChange != nil {
		v.onSpeedChange(speed)
	}
}

func (v *VideoPlayer) GetSpeed() float64 {
	return v.speed
}

func (v *VideoPlayer) SetFrameRate(fps float64) {
	v.frameRate = fps
}

func (v *VideoPlayer) GetFrameRate() float64 {
	if v.frameRate > 0 {
		return v.frameRate
	}
	return 30.0
}

func (v *VideoPlayer) OnFrameRate(cb func(float64)) {
	v.onFrameRate = cb
}

func (v *VideoPlayer) SetChapters(chapters []Chapter) {
	v.chapters = chapters
	v.currentChapter = 0
	v.updateChapterVisibility()
	v.updateChapterMarkers()
}

func (v *VideoPlayer) prevChapter() {
	if len(v.chapters) == 0 {
		return
	}

	currentIdx := -1
	for i, ch := range v.chapters {
		if v.currentTime >= ch.StartTime && (i == len(v.chapters)-1 || v.currentTime < v.chapters[i+1].StartTime) {
			currentIdx = i
			break
		}
	}

	targetIdx := currentIdx - 1
	if targetIdx < 0 {
		targetIdx = 0
	}

	if v.onPrevChapter != nil {
		v.onPrevChapter()
	} else if v.onSeek != nil {
		v.onSeek(v.chapters[targetIdx].StartTime)
	}
	v.currentChapter = targetIdx
	v.updateChapterVisibility()
}

func (v *VideoPlayer) nextChapter() {
	if len(v.chapters) == 0 {
		return
	}

	currentIdx := -1
	for i, ch := range v.chapters {
		if v.currentTime >= ch.StartTime && (i == len(v.chapters)-1 || v.currentTime < v.chapters[i+1].StartTime) {
			currentIdx = i
			break
		}
	}

	targetIdx := currentIdx + 1
	if targetIdx >= len(v.chapters) {
		targetIdx = len(v.chapters) - 1
	}

	if v.onNextChapter != nil {
		v.onNextChapter()
	} else if v.onSeek != nil {
		v.onSeek(v.chapters[targetIdx].StartTime)
	}
	v.currentChapter = targetIdx
	v.updateChapterVisibility()
}

func (v *VideoPlayer) updateChapterVisibility() {
	hasChapters := len(v.chapters) > 1

	if v.prevChapterBtn != nil {
		if hasChapters {
			v.prevChapterBtn.Show()
		} else {
			v.prevChapterBtn.Hide()
		}
	}

	if v.nextChapterBtn != nil {
		if hasChapters {
			v.nextChapterBtn.Show()
		} else {
			v.nextChapterBtn.Hide()
		}
	}
}

func (v *VideoPlayer) updateChapterMarkers() {
	if v.markerCanvas != nil {
		v.markerCanvas.Refresh()
	}
}

func (v *VideoPlayer) scaleNearest(src image.Image, dst *image.RGBA, srcW, srcH, dstW, dstH, offsetX, offsetY int) {
	if dstW == 0 || dstH == 0 {
		return
	}

	scaleX := float64(srcW) / float64(dstW)
	scaleY := float64(srcH) / float64(dstH)

	bounds := dst.Bounds()
	pix := dst.Pix
	stride := dst.Stride

	for y := 0; y < dstH; y++ {
		srcY := int(float64(y) * scaleY)
		if srcY >= srcH {
			srcY = srcH - 1
		}
		dstY := y + offsetY
		if dstY < bounds.Min.Y || dstY >= bounds.Max.Y {
			continue
		}
		rowStart := (dstY - bounds.Min.Y) * stride

		for x := 0; x < dstW; x++ {
			srcX := int(float64(x) * scaleX)
			if srcX >= srcW {
				srcX = srcW - 1
			}
			dstX := x + offsetX
			if dstX < bounds.Min.X || dstX >= bounds.Max.X {
				continue
			}

			r, g, b, a := src.At(srcX, srcY).RGBA()
			pixOffset := rowStart + (dstX-bounds.Min.X)*4
			pix[pixOffset] = byte(r >> 8)
			pix[pixOffset+1] = byte(g >> 8)
			pix[pixOffset+2] = byte(b >> 8)
			pix[pixOffset+3] = byte(a >> 8)
		}
	}
}

// drawChapterMarkers renders thin tick marks over the seek slider at each
// chapter boundary. The image background is transparent so the slider
// beneath remains visible and interactive.
func (v *VideoPlayer) drawChapterMarkers(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	if v.duration <= 0 || len(v.chapters) <= 1 {
		return img
	}

	tick := color.RGBA{R: 0x4C, G: 0xE8, B: 0x70, A: 0xCC}
	tickW := 2
	// Leave a small vertical margin so the tick sits on the track
	margin := h / 4
	if margin < 2 {
		margin = 2
	}

	// Skip index 0 — that's just the start of the video.
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
	return img
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

func (v *VideoPlayer) OnSpeedChange(cb func(float64)) {
	v.onSpeedChange = cb
}

func (v *VideoPlayer) OnPrevChapter(cb func()) {
	v.onPrevChapter = cb
}

func (v *VideoPlayer) OnNextChapter(cb func()) {
	v.onNextChapter = cb
}

func (v *VideoPlayer) OnChapterSelect(cb func(int)) {
	v.onChapterSelect = cb
}

func (v *VideoPlayer) GetCurrentChapter() int {
	return v.currentChapter
}

func (v *VideoPlayer) GetChapterCount() int {
	return len(v.chapters)
}

func (v *VideoPlayer) toggleFullscreen() {
	v.isFullscreen = !v.isFullscreen
	if v.fullscreenBtn != nil {
		if v.isFullscreen {
			v.fullscreenBtn.SetText("❎")
		} else {
			v.fullscreenBtn.SetText("⛶")
		}
	}
	if v.onFullscreen != nil {
		v.onFullscreen(v.isFullscreen)
	}
}

func (v *VideoPlayer) SetFullscreen(fullscreen bool) {
	if v.isFullscreen == fullscreen {
		return
	}
	v.toggleFullscreen()
}

func (v *VideoPlayer) IsFullscreen() bool {
	return v.isFullscreen
}

func (v *VideoPlayer) OnFullscreen(cb func(bool)) {
	v.onFullscreen = cb
}

func (v *VideoPlayer) OnPiP(cb func()) {
	v.onPiP = cb
}

func (v *VideoPlayer) togglePiP() {
	v.isPiP = !v.isPiP
	if v.pipBtn != nil {
		if v.isPiP {
			v.pipBtn.Importance = widget.HighImportance
		} else {
			v.pipBtn.Importance = widget.LowImportance
		}
	}
	if v.onPiP != nil {
		v.onPiP()
	}
	logging.Info(logging.CatPlayer, "PiP toggled: %v", v.isPiP)
}

func (v *VideoPlayer) IsPiP() bool {
	return v.isPiP
}

func (v *VideoPlayer) OnSubtitles(cb func(bool)) {
	v.onSubtitles = cb
}

func (v *VideoPlayer) toggleSubtitles() {
	v.subtitlesEnabled = !v.subtitlesEnabled
	if v.subtitleBtn != nil {
		if v.subtitlesEnabled {
			v.subtitleBtn.Text = "CC"
			v.subtitleBtn.Importance = widget.MediumImportance
		} else {
			v.subtitleBtn.Text = "CC"
			v.subtitleBtn.Importance = widget.LowImportance
		}
	}
	if v.onSubtitles != nil {
		v.onSubtitles(v.subtitlesEnabled)
	}
	logging.Info(logging.CatPlayer, "Subtitles toggled: %v", v.subtitlesEnabled)
}

func (v *VideoPlayer) IsSubtitlesEnabled() bool {
	return v.subtitlesEnabled
}

func (v *VideoPlayer) SetSubtitlesEnabled(enabled bool) {
	v.subtitlesEnabled = enabled
	if v.subtitleBtn != nil {
		if enabled {
			v.subtitleBtn.Importance = widget.MediumImportance
		} else {
			v.subtitleBtn.Importance = widget.LowImportance
		}
	}
}

func (v *VideoPlayer) OnHover(cb func(float64)) {
	v.onHover = cb
}

func (v *VideoPlayer) GetHoverFrame(time float64) *image.RGBA {
	v.thumbnailMu.RLock()
	defer v.thumbnailMu.RUnlock()

	pts := int64(time * 1000)
	if frame, ok := v.thumbnailCache[pts]; ok {
		return frame
	}

	var nearestFrame *image.RGBA
	minDiff := int64(^uint64(0) >> 1)

	for cachedPts, frame := range v.thumbnailCache {
		diff := cachedPts - pts
		if diff < 0 {
			diff = -diff
		}
		if diff < minDiff {
			minDiff = diff
			nearestFrame = frame
		}
	}

	return nearestFrame
}

func (v *VideoPlayer) AddThumbnailFrame(time float64, frame *image.RGBA) {
	if frame == nil {
		return
	}
	v.thumbnailMu.Lock()
	defer v.thumbnailMu.Unlock()

	pts := int64(time * 1000)
	if v.thumbnailCache == nil {
		v.thumbnailCache = make(map[int64]*image.RGBA)
	}

	if len(v.thumbnailCache) >= 50 {
		var oldest int64
		for k := range v.thumbnailCache {
			if oldest == 0 || k < oldest {
				oldest = k
			}
		}
		delete(v.thumbnailCache, oldest)
	}

	v.thumbnailCache[pts] = frame
}

func (v *VideoPlayer) ClearThumbnailCache() {
	v.thumbnailMu.Lock()
	defer v.thumbnailMu.Unlock()
	v.thumbnailCache = make(map[int64]*image.RGBA)
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
	if v.hasError && v.errorMessage != "" {
		logging.Info(logging.CatPlayer, "VideoPlayer error: %s", v.errorMessage)
		return
	}
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
}

func (r *videoPlayerRenderer) Objects() []fyne.CanvasObject {
	if r.VideoPlayer.raster == nil {
		r.VideoPlayer.raster = canvas.NewRaster(r.VideoPlayer.draw)
	}
	return []fyne.CanvasObject{r.VideoPlayer.raster, r.VideoPlayer.controlBar, r.VideoPlayer.controls}
}

func (r *videoPlayerRenderer) Layout(size fyne.Size) {
	if r.VideoPlayer.raster != nil {
		r.VideoPlayer.raster.Resize(size)
	}

	barHeight := float32(controlBarHeight)
	if r.minimal {
		barHeight = float32(controlBarHeightMini)
	}
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
	if r.VideoPlayer.raster != nil {
		r.VideoPlayer.raster.Refresh()
	}
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

	newW := w
	newH := availableH

	scaleX := float64(w) / float64(srcW)
	scaleY := float64(availableH) / float64(srcH)
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}

	newW = int(float64(srcW) * scale)
	newH = int(float64(srcH) * scale)

	offsetX := (w - newW) / 2
	offsetY := (availableH - newH) / 2

	img := image.NewRGBA(image.Rect(0, 0, w, availableH))
	draw.Draw(img, img.Bounds(), image.Black, image.Point{}, draw.Src)

	v.scaleNearest(src, img, srcW, srcH, newW, newH, offsetX, offsetY)

	return img
}
