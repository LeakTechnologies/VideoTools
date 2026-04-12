//go:build native_media

package ui

import (
	"errors"
	"fmt"
	"image"
	"io"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/media"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

type InlineVideoPlayer struct {
	mu         sync.Mutex // serialises all engine access (Load, Play, Pause, Seek, Step)
	player     *media.VideoPlayer
	engine     *media.Engine
	scrubber   *media.SmoothScrubbing
	playing    bool
	onProgress func(float64) // called from playbackLoop with current time in seconds
	onEnd      func()        // called on clean end-of-stream; NOT called on error
	seekCh     chan float64  // capacity-1 channel; seekLoop drains it serially
}

// SetOnProgress registers a callback that is called from the playback goroutine
// with the current playback time (in seconds) on each decoded frame.
// The callback must be safe to call from a goroutine.
func (v *InlineVideoPlayer) SetOnProgress(fn func(float64)) {
	v.mu.Lock()
	v.onProgress = fn
	v.mu.Unlock()
}

// SetOnEnd registers a callback that fires when playback reaches end-of-stream.
// It is dispatched on the main Fyne goroutine and is safe to update UI from.
func (v *InlineVideoPlayer) SetOnEnd(fn func()) {
	v.mu.Lock()
	v.onEnd = fn
	v.mu.Unlock()
}

func NewInlineVideoPlayer() *InlineVideoPlayer {
	v := &InlineVideoPlayer{
		player: media.NewInlineVideoPlayer(),
		seekCh: make(chan float64, 1),
	}
	go v.seekLoop()
	// Wire the widget's built-in controls to this player by default.
	// Modules that need custom logic (e.g. Trim) can overwrite via OnPlay/OnPause/OnSeek.
	p := v.player
	p.OnPlay(func() { v.Play() })
	p.OnPause(func() { v.Pause() })
	// OnSeek sends to the debounce channel — rapid slider drags drop intermediate
	// positions and the seekLoop runs each accepted seek off the event goroutine.
	p.OnSeek(func(target float64) {
		select {
		case v.seekCh <- target:
		default: // a seek is already queued; drop this one
		}
	})
	p.OnSpeedChange(func(speed float64) { v.SetSpeed(speed) })
	return v
}

// seekLoop drains seekCh and executes seeks serially on a dedicated goroutine,
// keeping engine.Seek + NextFrame off the main event goroutine.
func (v *InlineVideoPlayer) seekLoop() {
	for target := range v.seekCh {
		v.Seek(target)
	}
}

func (v *InlineVideoPlayer) Widget() *media.VideoPlayer {
	return v.player
}

func (v *InlineVideoPlayer) SetOnTapEmpty(fn func()) {
	v.player.SetOnTapEmpty(fn)
}

func (v *InlineVideoPlayer) SetIdleText(text string) {
	v.player.SetIdleText(text)
}

func (v *InlineVideoPlayer) Load(path string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.player.ClearError()
	v.player.SetLoading(true)

	// Stop any in-progress playback before swapping the engine.
	v.playing = false
	if v.scrubber != nil {
		v.scrubber.Stop()
		v.scrubber = nil
	}
	if v.engine != nil {
		v.engine.Close()
		v.engine = nil
	}

	v.engine = media.NewEngine()
	v.engine.SetSeekAccuracy(media.SeekAccuracyKeyframe)
	v.engine.SetDropFrames(true)
	if hw := media.DetectHWDevice(); hw != media.HWDeviceNone {
		v.engine.SetHWDevice(hw)
		logging.Info(logging.CatPlayer, "InlineVideoPlayer: HW decode active (%v)", hw)
	}

	logging.Info(logging.CatPlayer, "InlineVideoPlayer: opening %s", path)
	if err := v.engine.Open(path); err != nil {
		logging.Error(logging.CatPlayer, "InlineVideoPlayer: failed to open %s: %v", path, err)
		v.player.SetError(err.Error())
		v.player.SetLoading(false)
		return err
	}

	logging.Info(logging.CatPlayer, "InlineVideoPlayer: file opened successfully")

	v.engine.InitFrameCache(30)

	if chapters := v.engine.GetChapters(); len(chapters) > 0 {
		v.player.SetChapters(chapters)
	}

	duration := v.engine.Duration()
	v.player.SetDuration(duration)

	// Start the demuxer briefly to decode the first frame for preview, then pause.
	// Seek(0) resets the master clock to t=0 after the demuxer goroutine starts.
	// Without this, the clock may advance >100ms before the first packet is decoded,
	// causing SyncVideo to classify PTS≈0 as "late" and drop the frame — leaving
	// the preview area black.
	v.engine.Start()
	_ = v.engine.Seek(0)
	if img, err := v.engine.NextFrame(); err == nil {
		// Use blocking dispatch so the frame is committed before the caller
		// rebuilds the view around this widget.
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			v.player.SetFrame(img)
		}, true)
	} else {
		logging.Warning(logging.CatPlayer, "InlineVideoPlayer: first-frame fetch failed: %v", err)
	}
	v.engine.Pause()

	v.scrubber = media.NewSmoothScrubbing(v.engine)
	v.scrubber.SetOnFrame(func(img *image.RGBA) {
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			v.player.SetFrame(img)
		}, false)
	})
	v.scrubber.Start()

	v.engine.StartThumbnailExtraction(func(t float64, img *image.RGBA) {
		v.player.AddThumbnailFrame(t, img)
	})

	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		v.player.SetLoading(false)
		v.player.Refresh()
	}, false)
	return nil
}

func (v *InlineVideoPlayer) Play() {
	v.mu.Lock()
	eng := v.engine
	if eng == nil {
		v.mu.Unlock()
		return
	}
	v.playing = true
	eng.Start()
	eng.Resume()
	v.mu.Unlock()
	go v.playbackLoop()
}

func (v *InlineVideoPlayer) Pause() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.engine == nil {
		return
	}
	v.playing = false
	v.engine.Pause()
}

func (v *InlineVideoPlayer) Seek(target float64) {
	v.mu.Lock()
	eng := v.engine
	v.mu.Unlock()
	if eng == nil {
		return
	}
	eng.Seek(target)
	img, err := eng.NextFrame()
	// SetFrame/SetCurrentTime write directly to raster state — dispatch to main thread.
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		if err == nil {
			v.player.SetFrame(img)
		}
		v.player.SetCurrentTime(target)
	}, false)
}

func (v *InlineVideoPlayer) GetChapters() []media.Chapter {
	return v.player.GetChapters()
}

// SeekToChapter seeks to the start of the chapter at idx (0-based).
func (v *InlineVideoPlayer) SeekToChapter(idx int) {
	chapters := v.player.GetChapters()
	if idx < 0 || idx >= len(chapters) {
		return
	}
	v.Seek(chapters[idx].StartTime)
}

// ChapterAt returns the index of the chapter that contains t, or -1 if none.
func (v *InlineVideoPlayer) ChapterAt(t float64) int {
	chapters := v.player.GetChapters()
	idx := -1
	for i, ch := range chapters {
		if t >= ch.StartTime {
			idx = i
		}
	}
	return idx
}

func (v *InlineVideoPlayer) SetSpeed(speed float64) {
	v.mu.Lock()
	eng := v.engine
	v.mu.Unlock()
	if eng == nil {
		return
	}
	eng.SetSpeed(speed)
}

func (v *InlineVideoPlayer) StepFrame(dir int) {
	v.mu.Lock()
	eng := v.engine
	if eng == nil {
		v.mu.Unlock()
		return
	}
	v.playing = false
	eng.Pause()
	v.mu.Unlock()
	if img, err := eng.Step(dir); err == nil {
		v.player.SetFrame(img)
		v.player.SetCurrentTime(eng.CurrentTime())
	}
}

func (v *InlineVideoPlayer) Duration() float64 {
	v.mu.Lock()
	eng := v.engine
	v.mu.Unlock()
	if eng == nil {
		return 0
	}
	return eng.Duration()
}

func (v *InlineVideoPlayer) FrameRate() float64 {
	v.mu.Lock()
	eng := v.engine
	v.mu.Unlock()
	if eng == nil {
		return 0
	}
	return eng.GetFrameRate()
}

func (v *InlineVideoPlayer) CurrentTime() float64 {
	return v.player.CurrentTime()
}

func (v *InlineVideoPlayer) ScrubTo(target float64) {
	if v.engine == nil || v.scrubber == nil {
		return
	}
	v.scrubber.RequestSeek(target)
	v.player.SetCurrentTime(target)
}

func (v *InlineVideoPlayer) GetAudioTracks() []media.StreamInfo {
	if v.engine == nil {
		return nil
	}
	return v.engine.GetAudioTracks()
}

func (v *InlineVideoPlayer) SelectAudioTrack(idx int) error {
	if v.engine == nil {
		return fmt.Errorf("no media loaded")
	}
	return v.engine.SelectAudioTrack(idx)
}

func (v *InlineVideoPlayer) SetVolume(vol float64) {
	if v.engine == nil {
		return
	}
	v.engine.SetVolume(float32(vol / 100.0))
}

func (v *InlineVideoPlayer) SetMuted(muted bool) {
	if v.engine == nil {
		return
	}
	v.engine.SetMuted(muted)
}

func (v *InlineVideoPlayer) GetSubtitleTracks() []media.StreamInfo {
	if v.engine == nil {
		return nil
	}
	return v.engine.GetSubtitleTracks()
}

func (v *InlineVideoPlayer) SelectSubtitleTrack(idx int) error {
	if v.engine == nil {
		return fmt.Errorf("no media loaded")
	}
	return v.engine.SelectSubtitleTrack(idx)
}

func (v *InlineVideoPlayer) DisableSubtitles() {
	if v.engine == nil {
		return
	}
	v.engine.DisableSubtitles()
}

func (v *InlineVideoPlayer) Close() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.playing = false
	if v.scrubber != nil {
		v.scrubber.Stop()
		v.scrubber = nil
	}
	if v.engine != nil {
		v.engine.Close()
		v.engine = nil
	}
}

func (v *InlineVideoPlayer) playbackLoop() {
	defer logging.RecoverPanic()
	defer logging.LogAllGoroutines()

	for {
		// Snapshot engine pointer under lock; if Load replaced it, stop this loop.
		v.mu.Lock()
		eng := v.engine
		playing := v.playing
		onProg := v.onProgress
		v.mu.Unlock()

		if !playing || eng == nil {
			return
		}

		img, err := eng.NextFrame()
		if err != nil {
			if errors.Is(err, io.EOF) {
				// Clean end of stream — reset state and notify the UI.
				v.mu.Lock()
				v.playing = false
				endFn := v.onEnd
				v.mu.Unlock()
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					v.player.SetPlaying(false)
					if endFn != nil {
						endFn()
					}
				}, false)
			} else {
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					v.player.SetError("Playback stopped: " + err.Error())
					v.player.SetPlaying(false)
				}, false)
			}
			return
		}
		t := eng.CurrentTime()
		// SetFrame/SetCurrentTime write directly to raster state and must run
		// on the main goroutine to avoid racing the renderer.
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			v.player.SetFrame(img)
			v.player.SetCurrentTime(t)
		}, false)
		if onProg != nil {
			onProg(t)
		}
	}
}

func BuildInlinePlayerPane(size fyne.Size) (fyne.CanvasObject, *InlineVideoPlayer) {
	player := NewInlineVideoPlayer()

	bg := canvas.NewRectangle(utils.MustHex("#0F1529"))
	bg.SetMinSize(size)

	container := container.NewMax(bg, player.Widget())

	return container, player
}
