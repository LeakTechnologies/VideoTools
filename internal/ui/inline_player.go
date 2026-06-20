//go:build native_media

package ui

import (
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/media"
	mediafilters "git.leaktechnologies.dev/leak_technologies/VideoTools/internal/media/filters"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/media/state"
)

type InlineVideoPlayer struct {
	mu          sync.Mutex // serialises all engine access (Load, Play, Pause, Seek, Step)
	player      *media.VideoPlayer
	engine      *media.Engine
	scrubber    *media.SmoothScrubbing
	playing     bool
	currentPath string          // path of the most recently loaded file; used for EOF→reload
	onProgress  func(float64)   // called from playbackLoop with current time in seconds
	onEnd       func()          // called on clean end-of-stream; NOT called on error
	onFrame     func(*image.RGBA) // called on every rendered frame (playback + scrub)
	onLoad      func(LoadEvent)  // fired on main goroutine at each load milestone
	seekCh      chan float64     // capacity-1 channel; seekLoop drains it serially
	peer        *InlineVideoPlayer // optional follower driven by play/pause/seek
	resumeState *state.ResumeState  // persisted playback position (optional)
	lastSave    time.Time           // throttle for resume auto-save

	// Frame timing diagnostics (P1-8)
	frameTimingVisible  bool
	frameTimingCount    int
	frameTimingLastPTS  float64
	frameTimingLastTime time.Time

	// Playlist for sequential playback.  playlist holds paths queued after
	// the current item; playlistIdx is the index of the next item to load.
	// Both are protected by mu.
	playlist    []string
	playlistIdx int
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

// Enqueue appends path to the playlist.  When the current item reaches
// end-of-stream, the player automatically loads and plays the next queued
// item.  Calling Enqueue while nothing is loaded makes path the first item;
// the caller must still call Load+Play explicitly for the first item.
func (v *InlineVideoPlayer) Enqueue(path string) {
	v.mu.Lock()
	v.playlist = append(v.playlist, path)
	v.mu.Unlock()
}

// ClearPlaylist empties the queued items.  The currently playing item is
// not affected.
func (v *InlineVideoPlayer) ClearPlaylist() {
	v.mu.Lock()
	v.playlist = v.playlist[:0]
	v.playlistIdx = 0
	v.mu.Unlock()
}

// PlaylistLen returns the number of items currently queued (not including
// the currently playing item).
func (v *InlineVideoPlayer) PlaylistLen() int {
	v.mu.Lock()
	n := len(v.playlist) - v.playlistIdx
	v.mu.Unlock()
	if n < 0 {
		return 0
	}
	return n
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

// SetPeer designates peer as a follower: every Play, Pause, and Seek on this
// player is mirrored to peer. The peer's built-in controls are disabled so
// only the primary player's transport bar drives both.
func (v *InlineVideoPlayer) SetPeer(peer *InlineVideoPlayer) {
	v.mu.Lock()
	v.peer = peer
	v.mu.Unlock()
	if peer != nil {
		peer.Widget().DisableBuiltinControls()
	}
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

// SetFrameTimingOverlayVisible enables or disables the per-frame timing
// diagnostics overlay on the player widget.
func (v *InlineVideoPlayer) SetFrameTimingOverlayVisible(visible bool) {
	v.mu.Lock()
	v.frameTimingVisible = visible
	if !visible {
		v.frameTimingCount = 0
		v.frameTimingLastTime = time.Time{}
		v.frameTimingLastPTS = 0
	}
	player := v.player
	v.mu.Unlock()
	if player != nil {
		player.SetFrameTimingVisible(visible)
	}
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
	p.OnSubtitles(func(enabled bool) {
		if enabled {
			tracks := v.GetSubtitleTracks()
			if len(tracks) > 0 {
				v.SelectSubtitleTrack(tracks[0].Index)
			}
		} else {
			v.DisableSubtitles()
		}
	})
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

func (v *InlineVideoPlayer) SetIdleAspectRatio(ratio float64) {
	v.player.SetIdleAspectRatio(ratio)
}

func (v *InlineVideoPlayer) SetDeinterlaceEnabled(enabled bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.engine != nil {
		v.engine.SetDeinterlaceEnabled(enabled)
	}
}

func (v *InlineVideoPlayer) SetSeekAccuracy(acc media.SeekAccuracy) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.engine != nil {
		v.engine.SetSeekAccuracy(acc)
	}
}

func (v *InlineVideoPlayer) SetGrowingFile(growing bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.engine != nil {
		v.engine.SetGrowingFile(growing)
	}
}

func (v *InlineVideoPlayer) SetABLoopEnabled(enabled bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.engine != nil {
		v.engine.SetABLoopEnabled(enabled)
	}
}

func (v *InlineVideoPlayer) SetLoopPoints(a, b float64) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.engine != nil {
		v.engine.SetLoopPoints(a, b)
	}
}

// SetAudioDelay sets the A/V offset in seconds on the live engine.
// Positive = delay video (compensates for early-arriving audio).
// Negative = advance video (compensates for late-arriving audio).
func (v *InlineVideoPlayer) SetAudioDelay(d float64) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.engine != nil {
		v.engine.SetAudioDelay(d)
	}
}

// SetResumeState attaches a persisted playback-position store. When set, the
// player automatically saves position during playback and restores on load.
// Pass nil to disable.
func (v *InlineVideoPlayer) SetResumeState(s *state.ResumeState) {
	v.mu.Lock()
	v.resumeState = s
	v.lastSave = time.Time{}
	v.mu.Unlock()
}

func (v *InlineVideoPlayer) Load(path string) (err error) {
	return v.loadViaOpen(path, true, func(eng *media.Engine) error { return eng.OpenAuto(path) })
}

// LoadDVD opens a DVD disc (ISO file or VIDEO_TS parent directory) in the
// player using FFmpeg's dvdvideo demuxer (libdvdnav/libdvdread). title=0
// selects the longest title automatically; title>0 selects by DVD title number.
func (v *InlineVideoPlayer) LoadDVD(devicePath string, title int) (err error) {
	return v.loadViaOpen(devicePath, true, func(eng *media.Engine) error { return eng.OpenDVD(devicePath, title) })
}

// LoadURL opens a network stream or URL. opts may be nil to use sensible
// defaults (60s timeout, reconnect on error). Supported schemes: http,
// https, hls, dash, rtsp, rtmp, mms, tcp, udp.
func (v *InlineVideoPlayer) LoadURL(url string, opts map[string]string) (err error) {
	return v.loadViaOpen(url, true, func(eng *media.Engine) error { return eng.OpenURL(url, opts) })
}

// loadViaOpen is the shared implementation of Load, LoadDVD and LoadURL.
// resetPlaylist clears the pending playlist queue (set true on all user-facing
// load calls; false when the playlist auto-advance reuses this path internally).
func (v *InlineVideoPlayer) loadViaOpen(displayPath string, resetPlaylist bool, openFn func(*media.Engine) error) (err error) {
	logging.Info(logging.CatPlayer, "Load: called for %s", displayPath)
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

	if resetPlaylist {
		v.playlist = v.playlist[:0]
		v.playlistIdx = 0
	}

	// Snapshot and clear the previous engine/scrubber before swapping in new
	// ones. Clearing under the lock prevents concurrent callers (seekLoop,
	// Seek, playbackLoop) from using the old engine after we've released it.
	v.playing = false
	v.currentPath = displayPath
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
	eng.SetSeekAccuracy(media.DefaultSeekAccuracy())
	eng.SetAudioDelay(media.DefaultAudioDelay())
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

	logging.Info(logging.CatPlayer, "InlineVideoPlayer: opening %s", displayPath)
	if err := openFn(eng); err != nil {
		logging.Error(logging.CatPlayer, "InlineVideoPlayer: failed to open %s: %v", displayPath, err)
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

	// Auto-restore saved playback position (P1-2).
	// Falls through to the SMPTE-bars placeholder if no saved position exists.
	if v.resumeState != nil {
		if saved, ok := v.resumeState.GetPosition(displayPath); ok && v.resumeState.ShouldResume(saved) {
			logging.Info(logging.CatPlayer, "InlineVideoPlayer: resuming at %.2fs for %s", saved.Position, displayPath)
			eng.Seek(saved.Position)
			if img, err := eng.NextFrame(); err == nil {
				firstFrame = img
			}
		}
	}

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
		v.player.SetFrameRate(eng.GetFrameRate())
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
	peer := v.peer

	// Guard: if a playbackLoop is already running, just ensure the engine is
	// unpaused and sync the widget icon. Do NOT seek or spawn a new goroutine —
	// stacking concurrent playbackLoops disrupts the audio/video pipeline.
	if v.playing {
		v.mu.Unlock()
		if eng.IsPaused() {
			eng.Resume()
		}
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			v.player.SetPlaying(true)
		}, false)
		if peer != nil {
			go peer.Play()
		}
		return
	}

	v.playing = true
	logging.Info(logging.CatPlayer, "InlineVideoPlayer.Play: calling Start()")

	if eng.IsRunning() {
		// Resuming from pause: seek back to the current clock position to
		// resync every pipeline stage (demuxer, packet queues, codecs) back
		// to the pause point. Without this the first audio chunk after resume
		// is ~1.47 s ahead, the clock jumps, and all video frames in that
		// gap are dropped.
		currentTime := eng.CurrentTime()
		logging.Info(logging.CatPlayer, "InlineVideoPlayer.Play: resuming at %.3f, seeking to resync pipeline", currentTime)
		if err := eng.Seek(currentTime); err != nil {
			logging.Warning(logging.CatPlayer, "InlineVideoPlayer.Play: seek-on-resume failed: %v", err)
		}
		v.mu.Unlock()

		if peer != nil {
			t := currentTime
			go func() {
				peer.Seek(t)
				peer.Play()
			}()
		}
		// Gate Resume on the first decoded frame so audio and video always
		// start together. videoDecodeLoop is already running; when it sees
		// paused=true and an empty frameQueue it decodes one frame, so
		// WaitForFrame normally returns within a single decode interval
		// (~10-30 ms for SW H.264). Check v.playing before unpausing in
		// case the user cancelled by pressing Pause during the wait.
		go func() {
			eng.WaitForFrame(500 * time.Millisecond)
			v.mu.Lock()
			stillPlaying := v.playing
			v.mu.Unlock()
			if !stillPlaying {
				return
			}
			logging.Info(logging.CatPlayer, "InlineVideoPlayer.Play: calling Resume()")
			eng.Resume()
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				v.player.SetPlaying(true)
			}, false)
			v.playbackLoop()
		}()
	} else {
		// Initial play: drain audio pre-buffered during thumbnail extraction.
		// audioDecodeLoop runs during Load() and pre-buffers audio while
		// thumbnails are being generated. Without draining, the first audio
		// chunk consumed by Read() has pts ~5 s, jumping the clock and
		// dropping all initial video frames.
		// Resume() is called immediately because it is what starts
		// videoDecodeLoop (deferred from Start to avoid racing with GrabFrame).
		eng.DrainAudio()
		eng.Start()
		logging.Info(logging.CatPlayer, "InlineVideoPlayer.Play: calling Resume()")
		eng.Resume()
		t := eng.CurrentTime()
		v.mu.Unlock()
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			v.player.SetPlaying(true)
		}, false)
		if peer != nil {
			go func() {
				peer.Seek(t)
				peer.Play()
			}()
		}
		go v.playbackLoop()
	}
}

func (v *InlineVideoPlayer) Pause() {
	v.mu.Lock()
	if v.engine == nil {
		v.mu.Unlock()
		return
	}
	v.playing = false
	v.engine.Pause()
	peer := v.peer
	v.mu.Unlock()
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		v.player.SetPlaying(false)
	}, false)
	if peer != nil {
		go peer.Pause()
	}
}

func (v *InlineVideoPlayer) Seek(target float64) {
	v.mu.Lock()
	eng := v.engine
	peer := v.peer
	v.mu.Unlock()
	if eng == nil {
		return
	}
	logging.Info(logging.CatPlayer, "InlineVideoPlayer.Seek: target=%.2f", target)
	eng.Seek(target)
	img, err := eng.NextFrame()
	logging.Info(logging.CatPlayer, "InlineVideoPlayer.Seek: first frame after seek err=%v img=%v", err, img != nil)
	// SetFrame/SetCurrentTime write directly to raster state — dispatch to main thread.
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		if err == nil {
			v.player.SetFrame(img)
		}
		v.player.SetCurrentTime(target)
	}, false)
	if peer != nil {
		go peer.Seek(target)
	}
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

// SetFilterPipeline installs a video filter graph on the active engine.
// The pipeline takes effect on the next decoded frame; call RefreshCurrentFrame
// immediately after to force a re-decode at the current position.
func (v *InlineVideoPlayer) SetFilterPipeline(pipeline *mediafilters.FilterPipeline) {
	v.mu.Lock()
	eng := v.engine
	v.mu.Unlock()
	if eng != nil {
		eng.SetFilterPipeline(pipeline)
	}
}

// RefreshCurrentFrame seeks to the current position, forcing the engine to
// re-decode the frame — useful after changing the filter pipeline on a paused player.
func (v *InlineVideoPlayer) RefreshCurrentFrame() {
	v.Seek(v.CurrentTime())
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

// growingFileWatcher polls path for size growth every 2s. When the file
// grows, it re-opens, seeks to lastPos, and resumes playback. Called as
// a goroutine from playbackLoop when growing-file mode is active on EOF.
func (v *InlineVideoPlayer) growingFileWatcher(path string, lastPos float64) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	fi, err := os.Stat(path)
	if err != nil {
		logging.Warning(logging.CatPlayer, "growingFileWatcher: stat failed: %v", err)
		return
	}
	lastSize := fi.Size()
	logging.Info(logging.CatPlayer, "growingFileWatcher: polling %s (size=%d)", path, lastSize)

	for range ticker.C {
		fi, err := os.Stat(path)
		if err != nil {
			logging.Warning(logging.CatPlayer, "growingFileWatcher: stat failed: %v", err)
			continue
		}
		if fi.Size() > lastSize && fi.Size() > 0 {
			logging.Info(logging.CatPlayer, "growingFileWatcher: file grew %d→%d, reloading", lastSize, fi.Size())
			if err := v.Load(path); err != nil {
				logging.Error(logging.CatPlayer, "growingFileWatcher: Load failed: %v", err)
				continue
			}
			if lastPos > 0 {
				v.Seek(lastPos)
			}
			v.Play()
			return
		}
		lastSize = fi.Size()
	}
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
		path := v.currentPath
		rs := v.resumeState
		v.mu.Unlock()

		if !playing || eng == nil {
			return
		}

		img, err := eng.NextFrame()
		t := eng.CurrentTime()

		if err != nil {
			logging.Info(logging.CatPlayer, "playbackLoop: NextFrame returned err=%v", err)
			if errors.Is(err, io.EOF) {
				// Clean end of stream.
				v.mu.Lock()
				v.playing = false
				endFn := v.onEnd
				reloadPath := v.currentPath
				isGrowing := eng.IsGrowingFile()
				v.mu.Unlock()

				// Mark completed in resume state (unless growing file).
				if !isGrowing && rs != nil && path != "" {
					rs.MarkCompleted(path)
				}

				// Growing file: don't fire end-of-stream yet — poll for growth.
				if isGrowing && reloadPath != "" {
					logging.Info(logging.CatPlayer, "playbackLoop: growing-file EOF, polling for growth")
					go v.growingFileWatcher(reloadPath, t)
					return
				}

				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					v.player.SetPlaying(false)
					if endFn != nil {
						endFn()
					}
				}, false)

				// Advance to next playlist item if one is queued.
				v.mu.Lock()
				var nextPath string
				if v.playlistIdx < len(v.playlist) {
					nextPath = v.playlist[v.playlistIdx]
					v.playlistIdx++
				}
				v.mu.Unlock()

				if nextPath != "" {
					go func() {
						if err := v.loadViaOpen(nextPath, false, func(eng *media.Engine) error { return eng.OpenAuto(nextPath) }); err != nil {
							logging.Error(logging.CatPlayer, "playbackLoop: playlist advance Load failed: %v", err)
							return
						}
						v.Play()
					}()
				} else if reloadPath != "" {
					// No next item — reload current file so the user can play again.
					go func() {
						if err := v.Load(reloadPath); err != nil {
							logging.Error(logging.CatPlayer, "playbackLoop: EOF reload failed: %v", err)
						}
					}()
				}
			} else {
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					v.player.SetError("Playback stopped: " + err.Error())
					v.player.SetPlaying(false)
				}, false)
			}
			return
		}

		// Frame timing diagnostics (P1-8): collect per-frame stats.
		if v.frameTimingVisible {
			v.frameTimingCount++
			now := time.Now()
			var dt string
			if !v.frameTimingLastTime.IsZero() {
				elapsed := now.Sub(v.frameTimingLastTime)
				dt = fmt.Sprintf("%.1fms", float64(elapsed)/float64(time.Millisecond))
			} else {
				dt = "-"
			}
			ptsDelta := t - v.frameTimingLastPTS
			v.frameTimingLastPTS = t
			v.frameTimingLastTime = now

			label := fmt.Sprintf("frm %d  pts %.3f  Δ%s  ptsΔ%+.3f",
				v.frameTimingCount, t, dt, ptsDelta)
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				v.player.SetFrameTimingText(label)
			}, false)
		}

		// Frame delivery: atomic store + goroutine-safe widget.Refresh().
		// No DoFromGoroutine round-trip needed — SetFrame is now lock-free.
		v.player.SetFrame(img)

		// Time and callback dispatch: async (non-blocking).  SetCurrentTime
		// must run on the main goroutine because it mutates Fyne widgets; false
		// keeps the playback goroutine from stalling if the UI is briefly busy.
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			v.player.SetCurrentTime(t)
			if onFrm != nil {
				onFrm(img)
			}
		}, false)
		if onProg != nil {
			onProg(t)
		}

		// Auto-save position for resume (throttled to every 5s).
		if rs != nil && t > 0 {
			v.mu.Lock()
			if time.Since(v.lastSave) >= 5*time.Second {
				v.lastSave = time.Now()
				dur := eng.Duration()
				v.mu.Unlock()
				rs.SavePosition(path, t, dur)
			} else {
				v.mu.Unlock()
			}
		}
	}
}

func BuildInlinePlayerPane(size fyne.Size) (fyne.CanvasObject, *InlineVideoPlayer) {
	player := NewInlineVideoPlayer()
	return BuildPlayerContainer(player.Widget(), size), player
}
