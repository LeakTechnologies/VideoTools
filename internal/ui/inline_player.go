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
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/media"
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
	onLoad     func(LoadEvent)   // fired on main goroutine at each load milestone
	seekCh     chan float64      // capacity-1 channel; seekLoop drains it serially
}

// LoadPhase identifies a milestone in the video load pipeline.
type LoadPhase int

const (
	LoadPhaseStarted    LoadPhase = iota // Load() entered; engine not yet open
	LoadPhaseOpen                        // avformat_open_input + find_stream_info done
	LoadPhaseFirstFrame                  // first video frame decoded and ready to display
	LoadPhaseReady                       // player UI updated; video is fully usable
	LoadPhaseFailed                      // load aborted with an error
)

func (p LoadPhase) String() string {
	switch p {
	case LoadPhaseStarted:
		return "Starting"
	case LoadPhaseOpen:
		return "Engine open"
	case LoadPhaseFirstFrame:
		return "First frame"
	case LoadPhaseReady:
		return "Ready"
	case LoadPhaseFailed:
		return "Failed"
	}
	return "Unknown"
}

// LoadEvent is delivered to the onLoad callback at each load milestone.
type LoadEvent struct {
	Phase LoadPhase
	At    time.Time
	Err   error // non-nil only when Phase == LoadPhaseFailed
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

// SetOnLoad registers a callback that is fired on the main Fyne goroutine at
// each milestone during Load() — Started, Open, FirstFrame, Ready, Failed.
// Use this to drive diagnostic displays without polling.
func (v *InlineVideoPlayer) SetOnLoad(fn func(LoadEvent)) {
	v.mu.Lock()
	v.onLoad = fn
	v.mu.Unlock()
}

// fireLoad dispatches a LoadEvent to the onLoad callback on the main goroutine.
// It is safe to call from any goroutine.
func (v *InlineVideoPlayer) fireLoad(evt LoadEvent) {
	v.mu.Lock()
	fn := v.onLoad
	v.mu.Unlock()
	if fn == nil {
		return
	}
	fyne.CurrentApp().Driver().DoFromGoroutine(func() { fn(evt) }, false)
}

func NewInlineVideoPlayer() *InlineVideoPlayer {
	v := &InlineVideoPlayer{
		player: media.NewInlineVideoPlayer(),
		// seekCh starts nil; Load() allocates it and starts seekLoop each time
		// a file is opened. This avoids a leaked goroutine when a player is
		// constructed but never loaded, and makes Load() the single owner.
	}
	v.player.SetIdleText("DRAG TO LOAD VIDEO")
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
	logging.Info(logging.CatPlayer, "Load: called for %s", path)
	defer func() {
		if r := recover(); r != nil {
			logging.Error(logging.CatPlayer, "Load panic: %v", r)
			err = fmt.Errorf("Load panic: %v", r)
		}
	}()

	v.fireLoad(LoadEvent{Phase: LoadPhaseStarted, At: time.Now()})

	// Widget mutations (ClearError, SetLoading) touch Fyne widget state and must
	// run on the main goroutine. Dispatching async lets Load() continue
	// immediately without waiting for a round-trip through the event loop.
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		v.player.ClearError()
		v.player.SetLoading(true)
	}, false)

	v.mu.Lock()

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

	// All heavy work (engine open, GrabFrame) runs without v.mu so the main
	// goroutine is never blocked acquiring v.mu while Load is inside FFmpeg.
	// v.mu is held only briefly at the end to swap in the live engine/scrubber.
	eng := media.NewEngine()
	eng.SetSeekAccuracy(media.SeekAccuracyKeyframe)
	eng.SetDropFrames(true)
	if hw := media.DetectHWDevice(); hw != media.HWDeviceNone {
		eng.SetHWDevice(hw)
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
	if err := eng.Open(path); err != nil {
		logging.Error(logging.CatPlayer, "InlineVideoPlayer: failed to open %s: %v", path, err)
		v.fireLoad(LoadEvent{Phase: LoadPhaseFailed, At: time.Now(), Err: err})
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			v.player.SetError(err.Error())
			v.player.SetLoading(false)
		}, false)
		return err
	}

	logging.Info(logging.CatPlayer, "InlineVideoPlayer: file opened successfully")
	v.fireLoad(LoadEvent{Phase: LoadPhaseOpen, At: time.Now()})
	eng.InitFrameCache(30)

	chapters := eng.GetChapters()
	duration := eng.Duration()

	// Start demuxer and grab first frame for immediate display.
	eng.Start()
	logging.Info(logging.CatPlayer, "Load: calling GrabFrame")
	var firstFrame *image.RGBA
	if img, err := eng.GrabFrame(4 * time.Second); err == nil {
		logging.Info(logging.CatPlayer, "Load: GrabFrame success %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
		firstFrame = img
		v.fireLoad(LoadEvent{Phase: LoadPhaseFirstFrame, At: time.Now()})
	} else {
		logging.Error(logging.CatPlayer, "Load: GrabFrame failed (player shows SMPTE bars): %v", err)
	}
	logging.Info(logging.CatPlayer, "Load: GrabFrame completed, resetting to start")
	// ResetAfterGrab repositions the format to 0, drains B-frame buffered
	// codec frames, and resets the clock. Seek(0) was not used here because
	// it self-deadlocked (Seek holds e.mu; seekFlushBefore then tried to
	// re-acquire e.mu which is not re-entrant).
	eng.ResetAfterGrab()
	eng.Pause()

	logging.Info(logging.CatPlayer, "InlineVideoPlayer: load completed, engine ready")

	scrubber := media.NewSmoothScrubbing(eng)
	scrubber.SetOnFrame(func(img *image.RGBA) {
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

	// Briefly lock to publish the live engine and scrubber.
	v.mu.Lock()
	v.engine = eng
	v.scrubber = scrubber
	v.mu.Unlock()

	scrubber.Start()

	eng.StartThumbnailExtraction(func(t float64, img *image.RGBA) {
		v.player.AddThumbnailFrame(t, img)
	})

	readyAt := time.Now()
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		if len(chapters) > 0 {
			v.player.SetChapters(chapters)
			v.player.OnPrevChapter(func() { v.prevChapter() })
			v.player.OnNextChapter(func() { v.nextChapter() })
		}
		v.player.SetDuration(duration)
		if firstFrame != nil {
			v.player.SetFrame(firstFrame)
		}
		v.player.SetLoading(false)
		v.player.Refresh()
		v.mu.Lock()
		fn := v.onLoad
		v.mu.Unlock()
		if fn != nil {
			fn(LoadEvent{Phase: LoadPhaseReady, At: readyAt})
		}
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

	logging.Info(logging.CatPlayer, "InlineVideoPlayer.Play: calling Start()")

	if eng.IsRunning() {
		// Resuming from pause: audioDecodeLoop was pcmCh-capacity (~1.47s) ahead
		// of the actual playback position when paused. Those packets are consumed
		// from audioQueue and cannot be un-consumed. A mini-seek to the current
		// clock position flushes all queues and codecs, repositioning every
		// pipeline stage back to the pause point. Without this, the first audio
		// chunk after resume is ~1.47s ahead, the clock jumps, and all video
		// frames in that gap are dropped.
		currentTime := eng.CurrentTime()
		logging.Info(logging.CatPlayer, "InlineVideoPlayer.Play: resuming at %.3f, seeking to resync pipeline", currentTime)
		if err := eng.Seek(currentTime); err != nil {
			logging.Warning(logging.CatPlayer, "InlineVideoPlayer.Play: seek-on-resume failed: %v", err)
		}
	} else {
		// Initial play: drain audio pre-buffered during thumbnail extraction.
		// audioDecodeLoop runs during Load() and pre-buffers audio while thumbnails
		// are being generated. Without draining here, the first audio chunk consumed
		// by Read() would have pts ~5s, jumping the clock and dropping initial video frames.
		eng.DrainAudio()
		eng.Start()
	}

	logging.Info(logging.CatPlayer, "InlineVideoPlayer.Play: calling Resume()")
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
	for i, ch := range chapters {
		if t >= ch.StartTime && (i == len(chapters)-1 || t < chapters[i+1].StartTime) {
			return i
		}
	}
	return -1
}

// prevChapter seeks to the previous chapter
func (v *InlineVideoPlayer) prevChapter() {
	chapters := v.player.GetChapters()
	if len(chapters) == 0 {
		return
	}
	current := v.player.GetCurrentChapter()
	if current <= 0 {
		return
	}
	v.Seek(chapters[current-1].StartTime)
}

// nextChapter seeks to the next chapter
func (v *InlineVideoPlayer) nextChapter() {
	chapters := v.player.GetChapters()
	if len(chapters) == 0 {
		return
	}
current := v.player.GetCurrentChapter()
	if current >= len(chapters)-1 {
		return
	}
	v.Seek(chapters[current+1].StartTime)
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

func (v *InlineVideoPlayer) GetClockTime() float64 {
	v.mu.Lock()
	eng := v.engine
	v.mu.Unlock()
	if eng == nil {
		return -1
	}
	return eng.CurrentTime()
}

func (v *InlineVideoPlayer) GetLastVideoPTS() float64 {
	v.mu.Lock()
	eng := v.engine
	v.mu.Unlock()
	if eng == nil {
		return -1
	}
	return eng.GetLastVideoPTS()
}

func (v *InlineVideoPlayer) GetLastAudioPTS() float64 {
	v.mu.Lock()
	eng := v.engine
	v.mu.Unlock()
	if eng == nil {
		return -1
	}
	return eng.GetLastAudioPTS()
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

	// Reset the widget to idle (SMPTE bars) so the last video frame isn't
	// left on screen after the user clears the video.
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		v.player.SetFrame(nil)
		v.player.SetDuration(0)
		v.player.SetCurrentTime(0)
		v.player.SetChapters(nil)
		v.player.ClearThumbnailCache()
		v.player.Refresh()
	}, false)

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
		t := eng.CurrentTime()
		if err != nil {
			logging.Info(logging.CatPlayer, "playbackLoop: NextFrame returned err=%v", err)
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
	return BuildPlayerContainer(player.Widget(), size), player
}
