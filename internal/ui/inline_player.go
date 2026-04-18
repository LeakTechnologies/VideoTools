//go:build native_media

package ui

import (
	"errors"
	"fmt"
	"image"
	"io"
	"sync"
	"time"

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
	onProgress func(float64)    // called from playbackLoop with current time in seconds
	onEnd      func()           // called on clean end-of-stream; NOT called on error
	onFrame    func(*image.RGBA) // called on every rendered frame (playback + scrub)
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

// SetOnFrame registers a callback that receives every rendered frame (both
// during playback and scrubbing). Called on the main Fyne goroutine.
// Pass nil to clear. Use this to mirror frames to a secondary surface.
func (v *InlineVideoPlayer) SetOnFrame(fn func(*image.RGBA)) {
	v.mu.Lock()
	v.onFrame = fn
	v.mu.Unlock()
}

func NewInlineVideoPlayer() *InlineVideoPlayer {
	v := &InlineVideoPlayer{
		player: media.NewInlineVideoPlayer(),
		// seekCh starts nil; Load() allocates it and starts seekLoop each time
		// a file is opened. This avoids a leaked goroutine when a player is
		// constructed but never loaded, and makes Load() the single owner.
	}
	// Wire the widget's built-in controls to this player by default.
	// Modules that need custom logic (e.g. Trim) can overwrite via OnPlay/OnPause/OnSeek.
	p := v.player
	p.OnPlay(func() { v.Play() })
	p.OnPause(func() { v.Pause() })
	// OnSeek sends to the debounce channel — rapid slider drags drop intermediate
	// positions and the seekLoop runs each accepted seek off the event goroutine.
	p.OnSeek(func(target float64) {
		v.mu.Lock()
		ch := v.seekCh
		v.mu.Unlock()
		if ch == nil {
			return
		}
		select {
		case ch <- target:
		default: // a seek is already queued; drop this one
		}
	})
	p.OnSpeedChange(func(speed float64) { v.SetSpeed(speed) })
	p.OnVolumeChange(func(vol float64) { v.SetVolume(vol * 100) })
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

func (v *InlineVideoPlayer) Load(path string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			logging.Error(logging.CatPlayer, "Load panic: %v", r)
			err = fmt.Errorf("Load panic: %v", r)
		}
	}()

	v.mu.Lock()

	v.player.ClearError()
	v.player.SetLoading(true)

	// Snapshot and clear the previous engine/scrubber before swapping in new
	// ones. Clearing under the lock prevents concurrent callers (seekLoop,
	// Seek, playbackLoop) from using the old engine after we've released it.
	v.playing = false
	oldScrubber := v.scrubber
	oldEngine := v.engine
	oldSeekCh := v.seekCh
	v.scrubber = nil
	v.engine = nil
	v.seekCh = nil

	// Fresh seek channel and loop for the new file.
	v.seekCh = make(chan float64, 1)
	go v.seekLoop()

	v.mu.Unlock()

	// Close the old seekCh synchronously so seekLoop exits and can't forward
	// stale seeks to the new engine we're about to create.
	if oldSeekCh != nil {
		close(oldSeekCh)
	}

	// Tear down old resources off the caller's goroutine (scrubber.Stop and
	// engine.Close both block waiting for goroutines and FFmpeg calls).
	go func() {
		if oldScrubber != nil {
			oldScrubber.Stop()
		}
		if oldEngine != nil {
			oldEngine.Close()
		}
	}()

	// Re-acquire the lock for the rest of Load (engine open, scrubber setup).
	v.mu.Lock()
	defer v.mu.Unlock()

	v.engine = media.NewEngine()
	v.engine.SetSeekAccuracy(media.SeekAccuracyKeyframe)
	v.engine.SetDropFrames(true)
	if hw := media.DetectHWDevice(); hw != media.HWDeviceNone {
		v.engine.SetHWDevice(hw)
		logging.Info(logging.CatPlayer, "InlineVideoPlayer: HW decode active (%v)", hw)
	} else {
		logging.Info(logging.CatPlayer, "InlineVideoPlayer: using SW decode (HW decode %v)", func() string {
			if media.HWDecodeEnabled() {
				return "enabled but not available"
			}
			return "disabled in Settings"
		}())
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

	// Start the demuxer so packets begin flowing, then seek to the start.
	// Get the first frame for immediate display.
	v.engine.Start()
	logging.Info(logging.CatPlayer, "Load: calling GrabFrame")
	if img, err := v.engine.GrabFrame(4 * time.Second); err == nil {
		logging.Info(logging.CatPlayer, "Got initial frame: %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			v.player.SetFrame(img)
		}, true)
	} else {
		logging.Warning(logging.CatPlayer, "Initial frame fetch failed: %v", err)
	}
	logging.Info(logging.CatPlayer, "Load: GrabFrame completed, seeking to 0 to reset clock")
	// Seek resets the master clock to 0 via clock.ResetTime(0). Without this,
	// the clock has been ticking since Start() and is already ahead of pts=0
	// by however long GrabFrame took, causing the first video frames to be
	// dropped as late when the user presses Play.
	v.engine.Seek(0)
	v.engine.Pause()

	logging.Info(logging.CatPlayer, "InlineVideoPlayer: load completed, engine ready")

	v.scrubber = media.NewSmoothScrubbing(v.engine)
	v.scrubber.SetOnFrame(func(img *image.RGBA) {
		logging.Info(logging.CatPlayer, "scrubber OnFrame callback: img=%v", img != nil)
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			v.player.SetFrame(img)
			v.mu.Lock()
			fn := v.onFrame
			v.mu.Unlock()
			if fn != nil {
				fn(img)
			}
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
	v.mu.Lock()
	eng := v.engine
	scrubber := v.scrubber
	v.mu.Unlock()
	if eng == nil || scrubber == nil {
		return
	}
	scrubber.RequestSeek(target)
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
	// Snapshot resources and clear all fields atomically under the lock so that
	// any concurrent caller (seekLoop, playbackLoop, Seek, Load) immediately
	// sees a nil engine and stops. The actual teardown — which blocks while
	// waiting for goroutines and FFmpeg calls to finish — runs on a background
	// goroutine so the UI thread is never frozen when the user presses Back.
	v.mu.Lock()
	v.playing = false
	scrubber := v.scrubber
	engine := v.engine
	seekCh := v.seekCh
	v.scrubber = nil
	v.engine = nil
	v.seekCh = nil
	v.mu.Unlock()

	// Close seekCh synchronously so seekLoop exits before Load() might create
	// a new one. seekLoop ranges over the old channel; closing it unblocks it.
	if seekCh != nil {
		close(seekCh)
	}

	// Heavy teardown (wg.Wait, demuxerWg.Wait, FFmpeg free) runs off-thread.
	go func() {
		if scrubber != nil {
			scrubber.Stop() // waits for predecodeFrom goroutines to exit
		}
		if engine != nil {
			engine.Close() // waits for demuxer, then frees FFmpeg contexts
		}
	}()
}

func (v *InlineVideoPlayer) playbackLoop() {
	defer logging.RecoverPanic()

	for {
		// Snapshot engine pointer under lock; if Load replaced it, stop this loop.
		v.mu.Lock()
		eng := v.engine
		playing := v.playing
		onProg := v.onProgress
		onFrm := v.onFrame
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
		// Synchronous dispatch (true) ensures at most one frame update is
		// pending on the main goroutine at a time. Async dispatch (false)
		// lets the queue grow unbounded, making button clicks feel frozen
		// until the backlog drains. v.mu is NOT held here, so Pause/Seek
		// callbacks on the main goroutine can acquire it without deadlock.
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			v.player.SetFrame(img)
			v.player.SetCurrentTime(t)
			if onFrm != nil {
				onFrm(img)
			}
		}, true)
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
