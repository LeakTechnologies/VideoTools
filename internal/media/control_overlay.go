//go:build native_media

package media

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/LeakTechnologies/VideoTools/internal/logging"
	vtheme "github.com/LeakTechnologies/VideoTools/internal/theme"
)

const (
	controlBarHeight     = 48
	controlBarHeightMini = 44
	controlAlpha         = 0xCC
)

var (
	controlBarBG     = color.RGBA{R: 0x0A, G: 0x0E, B: 0x1A, A: 0xD0}
	sliderFill        = color.RGBA{R: 0x4C, G: 0xE8, B: 0x70, A: 0xFF}
	sliderBackground  = color.RGBA{R: 0x40, G: 0x40, B: 0x50, A: 0x80}
)

func (v *VideoPlayer) buildControls() {
	th := fyne.CurrentApp().Settings().Theme()
	v.playBtn = vtheme.MakePillIconButton(th.Icon(theme.IconNameMediaPlay), v.togglePlay)

	var lastSliderPos float64
	v.slider = vtheme.MakeSlider(0, 100)
	v.slider.OnChanged = func(pos float64) {
		if v.suppressSeek {
			return
		}
		v.isSeeking = true
		if v.duration > 0 {
			target := (pos / 100.0) * v.duration
			v.currentTime = target
			if v.onSeek != nil {
				v.onSeek(target)
			}
			icon := fmt.Sprintf("⏩ %s", formatVideoTime(target))
			if pos < lastSliderPos {
				icon = fmt.Sprintf("⏪ %s", formatVideoTime(target))
			}
			v.showOSD(icon)
		}
		lastSliderPos = pos
	}

	v.timeLabel = canvas.NewText("00:00:00", color.White)
	v.timeLabel.TextSize = 12

	v.durLabel = canvas.NewText("00:00:00", color.White)
	v.durLabel.TextSize = 12

	v.volumeBtn = vtheme.MakePillIconButton(th.Icon(theme.IconNameVolumeUp), v.toggleMute)

	v.volumeSlider = vtheme.MakeSlider(0, 100)
	v.volumeSlider.Value = v.volume * 100
	v.volumeSlider.Resize(fyne.NewSize(150, 40))
	v.volumeSlider.OnChanged = func(pos float64) {
		v.SetVolume(pos / 100.0)
		if v.onVolumeChange != nil {
			v.onVolumeChange(v.volume)
		}
	}

	v.speedBtn = vtheme.MakePillButton("1x", vtheme.TextMuted, v.toggleSpeed)

	v.prevChapterBtn = vtheme.MakePillIconButton(th.Icon(theme.IconNameMediaSkipPrevious), v.prevChapter)
	v.prevChapterBtn.Hide()

	v.nextChapterBtn = vtheme.MakePillIconButton(th.Icon(theme.IconNameMediaSkipNext), v.nextChapter)
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

	v.subtitleBtn = vtheme.MakePillButton("CC", vtheme.TextMuted, v.toggleSubtitles)

	v.markerCanvas = canvas.NewRaster(v.drawMarkers)
	seekStack := container.NewStack(v.slider, v.markerCanvas)

	if v.minimal {
		controlRow := container.NewHBox(
			v.playBtn,
			v.timeLabel,
			seekStack,
			v.durLabel,
			v.speedBtn,
			v.volumeBtn,
			v.volumeSlider,
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
			v.volumeSlider,
			v.subtitleBtn,
		)
		v.controlBar = canvas.NewRectangle(controlBarBG)
		v.controlBar.CornerRadius = 0
		v.controls = container.NewStack(
			canvas.NewRectangle(color.Transparent),
			container.NewPadded(container.NewBorder(nil, nil, nil, nil, controlRow)),
		)
		_ = layout.NewBorderLayout(v.controls, nil, nil, nil)
	}

	v.osdBg = canvas.NewRectangle(color.NRGBA{R: 0, G: 0, B: 0, A: 185})
	v.osdBg.CornerRadius = 8
	v.osdText = canvas.NewText("▶", color.NRGBA{R: 0, G: 230, B: 80, A: 255})
	v.osdText.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	v.osdText.TextSize = 42
	v.osdText.Alignment = fyne.TextAlignCenter
	v.osdBg.Hide()
	v.osdText.Hide()

	v.frameTimingBg = canvas.NewRectangle(color.NRGBA{R: 0, G: 0, B: 0, A: 160})
	v.frameTimingBg.CornerRadius = 4
	v.frameTimingText = canvas.NewText("", color.NRGBA{G: 220, A: 255})
	v.frameTimingText.TextSize = 11
	v.frameTimingText.TextStyle = fyne.TextStyle{Monospace: true}
	v.frameTimingBg.Hide()
	v.frameTimingText.Hide()
}

func (v *VideoPlayer) SetPlaying(playing bool) {
	v.isPlaying = playing
	if v.playBtn != nil {
		th := fyne.CurrentApp().Settings().Theme()
		if playing {
			v.playBtn.SetIcon(th.Icon(theme.IconNameMediaPause))
		} else {
			v.playBtn.SetIcon(th.Icon(theme.IconNameMediaPlay))
		}
		v.playBtn.Refresh()
	}
}

func (v *VideoPlayer) SetVolume(vol float64) {
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
	if v.volumeSlider != nil {
		v.volumeSlider.Value = vol * 100
	}
}

func (v *VideoPlayer) showOSD(icon string) {
	if v.osdText == nil || v.osdBg == nil {
		return
	}
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		v.osdText.Text = icon
		v.osdText.Refresh()
		v.osdBg.Show()
		v.osdText.Show()
		v.Refresh()
	}, false)

	if v.osdTimer != nil {
		v.osdTimer.Stop()
	}
	v.osdTimer = time.AfterFunc(1500*time.Millisecond, func() {
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			v.osdBg.Hide()
			v.osdText.Hide()
			v.Refresh()
		}, false)
	})
}

func (v *VideoPlayer) togglePlay() {
	if v.isPlaying {
		v.showOSD("⏸")
		if v.onPause != nil {
			v.onPause()
		}
	} else {
		v.showOSD("▶")
		if v.onPlay != nil {
			v.onPlay()
		}
	}
}

func (v *VideoPlayer) toggleMute() {
	v.muted = !v.muted
	if v.muted {
		v.SetVolume(0)
		if v.volumeSlider != nil {
			v.volumeSlider.Value = 0
		}
		v.showOSD("▷ ╳")
	} else {
		v.SetVolume(1.0)
		if v.volumeSlider != nil {
			v.volumeSlider.Value = 100
		}
		v.showOSD("♪")
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
	next := speeds[nextIdx]
	v.SetSpeed(next)
	icon := fmt.Sprintf("%.2g×", next)
	if next == 1.0 {
		icon = "▶  1×"
	} else if next < 1.0 {
		icon = fmt.Sprintf("◀  %.2g×", next)
	} else {
		icon = fmt.Sprintf("▶▶ %.2g×", next)
	}
	v.showOSD(icon)
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

func (v *VideoPlayer) GetChapters() []Chapter {
	return v.chapters
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

func (v *VideoPlayer) GetCurrentChapter() int {
	return v.currentChapter
}

func (v *VideoPlayer) GetChapterCount() int {
	return len(v.chapters)
}

func (v *VideoPlayer) toggleSubtitles() {
	v.subtitlesEnabled = !v.subtitlesEnabled
	if v.subtitleBtn != nil {
		v.subtitleBtn.Active = v.subtitlesEnabled
		v.subtitleBtn.Refresh()
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
		v.subtitleBtn.Active = enabled
		v.subtitleBtn.Refresh()
	}
}

func (v *VideoPlayer) OnSubtitles(cb func(bool)) {
	v.onSubtitles = cb
}

func (v *VideoPlayer) DisableBuiltinControls() {
	v.builtinControlsLocked = true
	v.showControls = false
	v.Refresh()
}

func (v *VideoPlayer) MouseIn(ev *desktop.MouseEvent) {
	v.mouseInView = true
	if !v.builtinControlsLocked {
		v.showControls = true
		v.resetControlHideTimer()
	}
	v.Refresh()
}

func (v *VideoPlayer) MouseMoved(ev *desktop.MouseEvent) {
	if v.builtinControlsLocked {
		return
	}
	if !v.showControls {
		v.showControls = true
		v.Refresh()
	}
	v.resetControlHideTimer()
}

func (v *VideoPlayer) MouseOut() {
	v.mouseInView = false
	if v.controlHideTimer != nil {
		v.controlHideTimer.Stop()
	}
	v.showControls = false
	v.Refresh()
}

func (v *VideoPlayer) resetControlHideTimer() {
	if v.controlHideTimer != nil {
		v.controlHideTimer.Stop()
	}
	v.controlHideTimer = time.AfterFunc(3*time.Second, func() {
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			if v.mouseInView {
				v.showControls = false
				v.Refresh()
			}
		}, false)
	})
}

func (v *VideoPlayer) SetFrameTimingVisible(visible bool) {
	v.showFrameTiming = visible
	if !visible {
		v.frameTimingBg.Hide()
		v.frameTimingText.Hide()
	}
	v.Refresh()
}

func (v *VideoPlayer) SetFrameTimingText(text string) {
	v.frameTimingText.Text = text
	v.Refresh()
}

func formatVideoTime(seconds float64) string {
	t := time.Duration(seconds * float64(time.Second))
	h := int(t.Hours())
	m := int(t.Minutes()) % 60
	s := int(t.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}