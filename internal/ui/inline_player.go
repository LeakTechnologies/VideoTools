//go:build native_media

package ui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"github.com/LeakTechnologies/VideoTools/internal/logging"
	"github.com/LeakTechnologies/VideoTools/internal/media"
	mediafilters "github.com/LeakTechnologies/VideoTools/internal/media/filters"
	"github.com/LeakTechnologies/VideoTools/internal/media/state"
)

type InlineVideoPlayer struct {
	diagnosticsEnabled bool
	diagnostics        atomic.Pointer[media.PlayerDebugStats]
	sourceGeneration   uint64
	session            *playerSession
	newEngine          func() media.PlaybackEngine
	dispatch           func(func())
	mu                 sync.Mutex // serialises all engine access (Load, Play, Pause, Seek, Step)
	player             *media.VideoPlayer
	engine             media.PlaybackEngine
	scrubber           media.Scrubber
	playing            bool
	speed              float64            // authoritative speed; mirrors the engine, survives engine swaps
	audioIdx           int                // active audio track index, -1 when unknown/none
	subtitleIdx        int                // active subtitle track index, -1 when subtitles are off
	currentPath        string             // path of the most recently loaded file; used for EOF→reload
	onProgress         func(float64)      // called from playbackLoop with current time in seconds
	onEnd              func()             // called on clean end-of-stream; NOT called on error
	onFrame            func(*image.RGBA)  // called on every rendered frame (playback + scrub)
	onLoad             func(LoadEvent)    // fired on main goroutine at each load milestone
	peer               *InlineVideoPlayer // optional follower driven by play/pause/seek
	resumeState        *state.ResumeState // persisted playback position (optional)
	lastSave           time.Time          // throttle for resume auto-save

	// Authoritative playback-state change notifications. Modules derive their
	// displayed transport/speed/track state from these instead of keeping an
	// independent copy that can drift (Convert audit #20). Each fires only on an
	// actual change, and always on the main Fyne goroutine so subscribers may
	// touch widgets directly.
	onPlayState   func(bool)
	onSpeedChange func(float64)
	onAudioTrack  func(int)
	onSubtitleTrk func(int)

	// notifiedPlaying is the value last handed to onPlayState. Comparing against
	// it is what makes the notification fire on genuine changes only, without any
	// caller having to remember whether it was the one that changed the state.
	notifiedPlaying bool

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

// ---------------------------------------------------------------------------
// Authoritative playback state (Convert audit #20)
//
// These getters and notifications are the single source of truth for
// play/pause, speed and track selection. A module must derive what it displays
// from here rather than caching its own copy: a widget-construction-time
// assignment or a fixed default will disagree with the engine the moment
// anything changes the state through another path (the overlay pill, a peer
// player, a programmatic SetSpeed, end-of-stream, frame stepping).
// ---------------------------------------------------------------------------

// OnPlayStateChange registers a callback fired whenever playback state changes.
// It runs on the main Fyne goroutine, so subscribers may update widgets.
func (v *InlineVideoPlayer) OnPlayStateChange(cb func(bool)) {
	v.mu.Lock()
	v.onPlayState = cb
	v.mu.Unlock()
}

// OnSpeedChange registers a callback fired when the playback speed changes.
func (v *InlineVideoPlayer) OnSpeedChange(cb func(float64)) {
	v.mu.Lock()
	v.onSpeedChange = cb
	v.mu.Unlock()
}

// OnAudioTrackChange registers a callback fired when the active audio track
// changes. The index is -1 when no audio track is active.
func (v *InlineVideoPlayer) OnAudioTrackChange(cb func(int)) {
	v.mu.Lock()
	v.onAudioTrack = cb
	v.mu.Unlock()
}

// OnSubtitleTrackChange registers a callback fired when the active subtitle
// track changes. The index is -1 when subtitles are off.
func (v *InlineVideoPlayer) OnSubtitleTrackChange(cb func(int)) {
	v.mu.Lock()
	v.onSubtitleTrk = cb
	v.mu.Unlock()
}

// IsPlaying reports the authoritative playback state: true only while the
// playback loop is running. Read this instead of caching a local paused flag.
func (v *InlineVideoPlayer) IsPlaying() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.playing
}

// GetSpeed returns the authoritative playback speed (1.0 when unset).
func (v *InlineVideoPlayer) GetSpeed() float64 {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.speed
}

// CurrentAudioTrack returns the index of the active audio track, or -1 when
// none is known or active.
func (v *InlineVideoPlayer) CurrentAudioTrack() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.audioIdx
}

// CurrentSubtitleTrack returns the index of the active subtitle track, or -1
// when subtitles are off.
func (v *InlineVideoPlayer) CurrentSubtitleTrack() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.subtitleIdx
}

// fireOnMain marshals fn onto the Fyne main goroutine, because widget
// mutations are only legal there and every authoritative-state notification
// originates on the playback, seek, or engine goroutine.
//
// It is a no-op when no Fyne app is running: the app is not yet constructed
// during early boot, and tests construct players headless. Silently skipping is
// safe there because there is no widget tree to mutate.
func fireOnMain(fn func()) {
	if fn == nil {
		return
	}
	if app := fyne.CurrentApp(); app == nil || app.Driver() == nil {
		return
	}
	fyne.CurrentApp().Driver().DoFromGoroutine(fn, false)
}

// notifyPlayState fires onPlayState if the authoritative state differs from
// what subscribers were last told, then records what they were told.
//
// It deliberately does NOT assign v.playing. Callers assign the field under
// v.mu, release the lock, and then call this. That ordering matters: it means a
// late notification can never resurrect a state that has already moved on.
//
// The naive alternative — `defer v.publishPlayState(true)` in Play() — is a
// real race. Play() starts playbackLoop() before returning, so a short clip can
// reach EOF, set playing=false and publish it, and only then would the deferred
// call force playing back to true. The button would show "playing" over a
// finished video and every later Pause() would look like a no-op transition.
//
// v.mu must NOT be held.
func (v *InlineVideoPlayer) notifyPlayState() {
	v.mu.Lock()
	playing := v.playing
	if v.notifiedPlaying == playing {
		v.mu.Unlock()
		return
	}
	v.notifiedPlaying = playing
	cb := v.onPlayState
	source := v.session
	v.mu.Unlock()
	if cb != nil {
		fireOnMain(func() {
			v.mu.Lock()
			current := v.session == source && v.playing == playing && v.notifiedPlaying == playing
			v.mu.Unlock()
			if current {
				cb(playing)
			}
		})
	}
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
	fireOnMain(func() { fn(evt) })
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
		// Authoritative state defaults. The engine, the audio clock and the
		// master clock all start at 1.0 with no config round-trip, so a fresh
		// player is neither playing, nor speed-modified, nor on any track.
		speed:       1.0,
		audioIdx:    -1,
		subtitleIdx: -1,
	}
	v.player.SetIdleText("DRAG TO LOAD VIDEO")
	// Wire the widget's built-in controls to this player by default.
	// Modules that need custom logic (e.g. Trim) can overwrite via OnPlay/OnPause/OnSeek.
	p := v.player
	p.OnPlay(func() { v.Play() })
	p.OnPause(func() { v.Pause() })
	// Seek submits a source-owned transport run off the event goroutine.
	p.OnSeek(func(target float64) { v.Seek(target) })

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
	v.withEngine(func(e media.PlaybackEngine) { e.SetDeinterlaceEnabled(enabled) })
}

func (v *InlineVideoPlayer) SetSeekAccuracy(acc media.SeekAccuracy) {
	v.withEngine(func(e media.PlaybackEngine) { e.SetSeekAccuracy(acc) })
}

func (v *InlineVideoPlayer) SetGrowingFile(growing bool) {
	v.withEngine(func(e media.PlaybackEngine) { e.SetGrowingFile(growing) })
}

func (v *InlineVideoPlayer) SetABLoopEnabled(enabled bool) {
	v.withEngine(func(e media.PlaybackEngine) { e.SetABLoopEnabled(enabled) })
}

func (v *InlineVideoPlayer) SetLoopPoints(a, b float64) {
	v.withEngine(func(e media.PlaybackEngine) { e.SetLoopPoints(a, b) })
}

func (v *InlineVideoPlayer) SetAudioDelay(d float64) {
	v.withEngine(func(e media.PlaybackEngine) { e.SetAudioDelay(d) })
}

func (v *InlineVideoPlayer) SetResumeState(s *state.ResumeState) {
	v.mu.Lock()
	v.resumeState = s
	v.lastSave = time.Time{}
	v.mu.Unlock()
}

func (v *InlineVideoPlayer) Load(path string) (err error) {
	return v.loadViaOpen(path, true, func(eng media.PlaybackEngine) error { return eng.OpenAuto(path) })
}

// LoadDVD opens a DVD disc (ISO file or VIDEO_TS parent directory) in the
// player using FFmpeg's dvdvideo demuxer (libdvdnav/libdvdread). title=0
// selects the longest title automatically; title>0 selects by DVD title number.
func (v *InlineVideoPlayer) LoadDVD(devicePath string, title int) (err error) {
	return v.loadViaOpen(devicePath, true, func(eng media.PlaybackEngine) error { return eng.OpenDVD(devicePath, title) })
}

// LoadURL opens a network stream or URL. opts may be nil to use sensible
// defaults (60s timeout, reconnect on error). Supported schemes: http,
// https, hls, dash, rtsp, rtmp, mms, tcp, udp.
func (v *InlineVideoPlayer) LoadURL(url string, opts map[string]string) (err error) {
	return v.loadViaOpen(url, true, func(eng media.PlaybackEngine) error { return eng.OpenURL(url, opts) })
}

// loadViaOpen is the shared implementation of Load, LoadDVD and LoadURL.
// resetPlaylist clears the pending playlist queue (set true on all user-facing
// load calls; false when the playlist auto-advance reuses this path internally).
func (v *InlineVideoPlayer) loadViaOpen(path string, reset bool, open func(media.PlaybackEngine) error) error {
	_, err := v.loadSession(path, reset, open, nil)
	return err
}

func (v *InlineVideoPlayer) Play() {
	defer v.notifyPlayState()
	v.mu.Lock()
	s := v.session
	v.mu.Unlock()
	v.playSession(s)
}

func (v *InlineVideoPlayer) Pause() {
	v.mu.Lock()
	s, peer := v.session, v.peer
	v.mu.Unlock()
	r := v.beginRun(s, false)
	if r == nil {
		return
	}
	v.notifyPlayState()
	go func() {
		defer close(r.done)
		if !r.waitPrevious() {
			return
		}
		s.ops.Lock()
		if v.runCurrent(s, r) {
			s.engine.Pause()
		}
		s.ops.Unlock()
		v.dispatchRun(s, r, func() {
			if v.player != nil {
				v.player.SetPlaying(false)
			}
		})
	}()
	if peer != nil {
		peer.Pause()
	}
}

func (v *InlineVideoPlayer) FlushAudioCodec() {
	v.withEngine(func(e media.PlaybackEngine) { e.FlushAudioCodec() })
}

func (v *InlineVideoPlayer) Seek(target float64) {
	v.mu.Lock()
	s, playing, peer := v.session, v.playing, v.peer
	v.mu.Unlock()
	r := v.beginRun(s, playing)
	if r == nil {
		return
	}
	go func() {
		defer close(r.done)
		if !r.waitPrevious() {
			return
		}
		s.ops.Lock()
		if !v.runCurrent(s, r) {
			s.ops.Unlock()
			return
		}
		s.engine.Pause()
		err := s.engine.Seek(target)
		var img *image.RGBA
		if err == nil {
			img, err = s.engine.NextFrameContext(r.ctx)
		}
		pts := s.engine.GetLastVideoPTS()
		s.ops.Unlock()
		if !v.runCurrent(s, r) {
			return
		}
		if err != nil {
			v.stopRun(s, r, err)
			return
		}
		v.dispatchRun(s, r, func() {
			if v.player != nil {
				v.player.SetFrame(img)
				v.player.SetCurrentTime(pts)
			}
		})
		if playing {
			v.playFromRun(s, r)
		}
	}()
	if peer != nil {
		peer.Seek(target)
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

// SetSpeed sets the playback speed and publishes the new authoritative value.
func (v *InlineVideoPlayer) SetSpeed(speed float64) {
	v.mu.Lock()
	source := v.session
	ready := v.engine != nil
	changed := v.speed != speed
	v.speed = speed
	cb := v.onSpeedChange
	v.mu.Unlock()
	if source != nil && ready {
		source.ops.Lock()
		if v.sessionCurrent(source) {
			source.engine.SetSpeed(speed)
		}
		source.ops.Unlock()
	}
	if changed && cb != nil {
		if source != nil {
			v.dispatchSession(source, func() { cb(speed) })
		} else {
			v.dispatchIdle(func() { cb(speed) })
		}
	}
}

func (v *InlineVideoPlayer) StepFrame(dir int) {
	v.mu.Lock()
	s := v.session
	v.mu.Unlock()
	r := v.beginRun(s, false)
	if r == nil {
		return
	}
	v.notifyPlayState()
	go func() {
		defer close(r.done)
		if !r.waitPrevious() {
			return
		}
		s.ops.Lock()
		if !v.runCurrent(s, r) {
			s.ops.Unlock()
			return
		}
		s.engine.Pause()
		img, err := s.engine.StepContext(r.ctx, dir)
		pts := s.engine.GetLastVideoPTS()
		s.ops.Unlock()
		if err != nil {
			v.stopRun(s, r, err)
			return
		}
		v.dispatchRun(s, r, func() {
			if v.player != nil {
				v.player.SetFrame(img)
				v.player.SetCurrentTime(pts)
				v.player.SetPlaying(false)
			}
		})
	}()
}

func (v *InlineVideoPlayer) Duration() float64 {
	var result float64 = 0
	v.withEngine(func(e media.PlaybackEngine) { result = e.Duration() })
	return result
}

func (v *InlineVideoPlayer) FrameRate() float64 {
	var result float64 = 0
	v.withEngine(func(e media.PlaybackEngine) { result = e.GetFrameRate() })
	return result
}

func (v *InlineVideoPlayer) CurrentTime() float64 {
	return v.player.CurrentTime()
}

// SetFilterPipeline installs a video filter graph on the active engine.
// The pipeline takes effect on the next decoded frame; call RefreshCurrentFrame
// immediately after to force a re-decode at the current position.
func (v *InlineVideoPlayer) SetFilterPipeline(pipeline *mediafilters.FilterPipeline) {
	v.withEngine(func(e media.PlaybackEngine) { e.SetFilterPipeline(pipeline) })
}

func (v *InlineVideoPlayer) RefreshCurrentFrame() {
	v.Seek(v.CurrentTime())
}

func (v *InlineVideoPlayer) GetClockTime() float64 {
	var result float64 = -1
	v.withEngine(func(e media.PlaybackEngine) { result = e.CurrentTime() })
	return result
}

func (v *InlineVideoPlayer) GetLastVideoPTS() float64 {
	var result float64 = -1
	v.withEngine(func(e media.PlaybackEngine) { result = e.GetLastVideoPTS() })
	return result
}

func (v *InlineVideoPlayer) GetLastAudioPTS() float64 {
	var result float64 = -1
	v.withEngine(func(e media.PlaybackEngine) { result = e.GetLastAudioPTS() })
	return result
}

func (v *InlineVideoPlayer) ScrubTo(target float64) {
	v.mu.Lock()
	s := v.session
	v.mu.Unlock()
	if s == nil {
		return
	}
	go func() {
		s.ops.Lock()
		if v.sessionCurrent(s) && s.scrubber != nil {
			s.scrubber.RequestSeek(target)
		}
		s.ops.Unlock()
	}()
	v.dispatchSession(s, func() {
		if v.player != nil {
			v.player.SetCurrentTime(target)
		}
	})
}

func (v *InlineVideoPlayer) GetAudioTracks() []media.StreamInfo {
	var result []media.StreamInfo = nil
	v.withEngine(func(e media.PlaybackEngine) { result = e.GetAudioTracks() })
	return result
}

func (v *InlineVideoPlayer) SelectAudioTrack(idx int) error {
	var cb func(int)
	changed := false
	err := fmt.Errorf("no media loaded")
	source := v.withEngine(func(e media.PlaybackEngine) {
		err = e.SelectAudioTrack(idx)
		if err != nil {
			return
		}
		v.mu.Lock()
		if v.session != nil && v.session.engine == e && v.session.ctx.Err() == nil {
			changed = v.audioIdx != idx
			v.audioIdx = idx
			cb = v.onAudioTrack
		}
		v.mu.Unlock()
	})
	if err != nil {
		return err
	}
	if source != nil && changed && cb != nil {
		v.dispatchSession(source, func() { cb(idx) })
	}
	return nil
}

func (v *InlineVideoPlayer) SetVolume(vol float64) {
	v.withEngine(func(e media.PlaybackEngine) { e.SetVolume(float32(vol / 100)) })
}

func (v *InlineVideoPlayer) SetMuted(muted bool) {
	v.withEngine(func(e media.PlaybackEngine) { e.SetMuted(muted) })
}

func (v *InlineVideoPlayer) GetSubtitleTracks() []media.StreamInfo {
	var result []media.StreamInfo = nil
	v.withEngine(func(e media.PlaybackEngine) { result = e.GetSubtitleTracks() })
	return result
}

func (v *InlineVideoPlayer) SelectSubtitleTrack(idx int) error {
	var cb func(int)
	changed := false
	err := fmt.Errorf("no media loaded")
	source := v.withEngine(func(e media.PlaybackEngine) {
		err = e.SelectSubtitleTrack(idx)
		if err != nil {
			return
		}
		v.mu.Lock()
		if v.session != nil && v.session.engine == e && v.session.ctx.Err() == nil {
			changed = v.subtitleIdx != idx
			v.subtitleIdx = idx
			cb = v.onSubtitleTrk
		}
		v.mu.Unlock()
	})
	if err != nil {
		return err
	}
	if source != nil && changed && cb != nil {
		v.dispatchSession(source, func() { cb(idx) })
	}
	return nil
}

func (v *InlineVideoPlayer) DisableSubtitles() {
	var cb func(int)
	changed := false
	source := v.withEngine(func(e media.PlaybackEngine) {
		e.DisableSubtitles()
		v.mu.Lock()
		if v.session != nil && v.session.engine == e && v.session.ctx.Err() == nil {
			changed = v.subtitleIdx != -1
			v.subtitleIdx = -1
			cb = v.onSubtitleTrk
		}
		v.mu.Unlock()
	})
	if source != nil && changed && cb != nil {
		v.dispatchSession(source, func() { cb(-1) })
	}
}

func (v *InlineVideoPlayer) CurrentPath() string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.currentPath
}

func (v *InlineVideoPlayer) Close() {
	v.mu.Lock()
	s := v.session
	if s != nil {
		s.cancel()
	}
	v.session = nil
	v.diagnostics.Store(nil)
	v.engine, v.scrubber = nil, nil
	v.playing = false
	v.currentPath = ""
	wasSpeed, wasAudio, wasSub := v.speed != 1, v.audioIdx != -1, v.subtitleIdx != -1
	v.speed, v.audioIdx, v.subtitleIdx = 1, -1, -1
	speedCB, audioCB, subCB := v.onSpeedChange, v.onAudioTrack, v.onSubtitleTrk
	v.mu.Unlock()
	v.notifyPlayState()
	if wasSpeed && speedCB != nil {
		v.dispatchIdle(func() { speedCB(1.0) })
	}
	if wasAudio && audioCB != nil {
		v.dispatchIdle(func() { audioCB(-1) })
	}
	if wasSub && subCB != nil {
		v.dispatchIdle(func() { subCB(-1) })
	}
	v.post(func() {
		v.mu.Lock()
		idle := v.session == nil
		v.mu.Unlock()
		if !idle || v.player == nil {
			return
		}
		v.player.SetPlaying(false)
		v.player.SetFrame(nil)
		v.player.SetDuration(0)
		v.player.SetCurrentTime(0)
		v.player.SetChapters(nil)
		v.player.ClearThumbnailCache()
		v.player.ClearError()
		v.player.SetLoading(false)
		v.player.Refresh()
	})
	retireSession(s)
}

func (v *InlineVideoPlayer) growingFileWatcher(s *playerSession, path string, position float64) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	fi, err := os.Stat(path)
	if err != nil {
		return
	}
	size := fi.Size()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
		}
		fi, err = os.Stat(path)
		if err != nil {
			continue
		}
		if fi.Size() > size {
			next, err := v.loadSession(path, true, func(e media.PlaybackEngine) error { return e.OpenAuto(path) }, s)
			if err == nil {
				next.ops.Lock()
				if v.sessionCurrent(next) {
					next.engine.SetGrowingFile(true)
				}
				next.ops.Unlock()
				v.seekAndPlaySession(next, position)
			}
			return
		}
		size = fi.Size()
	}
}

func (v *InlineVideoPlayer) playbackLoop(s *playerSession, r *playbackRun) {
	defer logging.RecoverPanic()
	for v.runCurrent(s, r) {
		v.mu.Lock()
		playing := v.playing
		onProg, onFrm, rs := v.onProgress, v.onFrame, v.resumeState
		path := v.currentPath
		v.mu.Unlock()
		if !playing {
			return
		}
		s.ops.Lock()
		if !v.runCurrent(s, r) {
			s.ops.Unlock()
			return
		}
		img, err := s.engine.NextFrameContext(r.ctx)
		t := s.engine.CurrentTime()
		growing := s.engine.IsGrowingFile()
		v.mu.Lock()
		debug := v.diagnosticsEnabled
		v.mu.Unlock()
		if debug {
			stats := s.engine.Diagnostics()
			stats.Session = s.id
			v.mu.Lock()
			if v.runCurrentLocked(s, r) {
				stats.Playing = v.playing
				v.diagnostics.Store(&stats)
			}
			v.mu.Unlock()
		}
		s.ops.Unlock()
		if !v.runCurrent(s, r) {
			return
		}
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, media.ErrEngineClosed) {
				return
			}
			if !errors.Is(err, io.EOF) {
				v.stopRun(s, r, err)
				return
			}
			v.mu.Lock()
			if !v.runCurrentLocked(s, r) {
				v.mu.Unlock()
				return
			}
			v.playing = false
			end := v.onEnd
			var nextPath string
			if v.playlistIdx < len(v.playlist) {
				nextPath = v.playlist[v.playlistIdx]
				v.playlistIdx++
			}
			v.mu.Unlock()
			v.notifyPlayState()
			if !growing && rs != nil && path != "" {
				rs.MarkCompleted(path)
			}
			if growing {
				go v.growingFileWatcher(s, path, t)
				return
			}
			v.dispatchRun(s, r, func() {
				if v.player != nil {
					v.player.SetPlaying(false)
				}
				if end != nil {
					end()
				}
			})
			go func() {
				reload := path
				if nextPath != "" {
					reload = nextPath
				}
				if reload == "" {
					return
				}
				next, err := v.loadSession(reload, nextPath == "", func(e media.PlaybackEngine) error { return e.OpenAuto(reload) }, s)
				if err == nil && nextPath != "" {
					v.playSession(next)
				}
			}()
			return
		}
		v.mu.Lock()
		if !v.runCurrentLocked(s, r) {
			v.mu.Unlock()
			return
		}
		timing := ""
		if v.frameTimingVisible {
			v.frameTimingCount++
			now := time.Now()
			dt := "-"
			if !v.frameTimingLastTime.IsZero() {
				dt = fmt.Sprintf("%.1fms", now.Sub(v.frameTimingLastTime).Seconds()*1000)
			}
			timing = fmt.Sprintf("frm %d pts %.3f interval %s delta %+.3f", v.frameTimingCount, t, dt, t-v.frameTimingLastPTS)
			v.frameTimingLastTime, v.frameTimingLastPTS = now, t
		}
		save := rs != nil && t > 0 && time.Since(v.lastSave) >= 5*time.Second
		if save {
			v.lastSave = time.Now()
		}
		v.mu.Unlock()
		v.dispatchRun(s, r, func() {
			if v.player != nil {
				// Fyne Refresh can synchronously wait for its main thread.
				// No player lock may be held across this widget call.
				v.player.SetFrame(img)
				v.player.SetCurrentTime(t)
				if timing != "" {
					v.player.SetFrameTimingText(timing)
				}
			}
			if onFrm != nil {
				onFrm(img)
			}
		})
		if onProg != nil && v.runCurrent(s, r) {
			onProg(t)
		}
		if save && v.runCurrent(s, r) {
			rs.SavePosition(path, t, v.Duration())
		}
	}
}

func BuildInlinePlayerPane(size fyne.Size) (fyne.CanvasObject, *InlineVideoPlayer) {
	player := NewInlineVideoPlayer()
	return BuildPlayerContainer(player.Widget(), size), player
}

func (v *InlineVideoPlayer) SetDiagnosticsEnabled(enabled bool) {
	v.mu.Lock()
	v.diagnosticsEnabled = enabled
	v.mu.Unlock()
	v.withEngine(func(e media.PlaybackEngine) { e.SetDiagnosticsEnabled(enabled) })
	if !enabled {
		v.diagnostics.Store(nil)
	}
}

// Return the latest source-owned snapshot without blocking on native decode.
func (v *InlineVideoPlayer) Diagnostics() media.PlayerDebugStats {
	if d := v.diagnostics.Load(); d != nil {
		result := *d
		v.mu.Lock()
		result.Playing = v.playing
		v.mu.Unlock()
		return result
	}
	return media.PlayerDebugStats{}
}
