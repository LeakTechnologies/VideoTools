//go:build native_media

package ui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"sync"
	"time"

	"github.com/LeakTechnologies/VideoTools/internal/logging"
	"github.com/LeakTechnologies/VideoTools/internal/media"
)

type playerSession struct {
	id       uint64
	ctx      context.Context
	cancel   context.CancelFunc
	loaded   chan struct{}
	ops      sync.Mutex
	engine   media.PlaybackEngine
	scrubber media.Scrubber
	run      *playbackRun // protected by the owning player's mu
}
type playbackRun struct {
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	previous <-chan struct{}
}

func (r *playbackRun) waitPrevious() bool {
	if r.previous != nil {
		// Join even when cancelled: a newer run also waits for us and must
		// never overtake an older consumer still inside native code.
		<-r.previous
	}
	return r.ctx.Err() == nil
}
func (v *InlineVideoPlayer) post(fn func()) {
	if v.dispatch != nil {
		v.dispatch(fn)
		return
	}
	fireOnMain(fn)
}
func (v *InlineVideoPlayer) sessionCurrent(s *playerSession) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return s != nil && v.session == s && s.ctx.Err() == nil
}
func (v *InlineVideoPlayer) runCurrentLocked(s *playerSession, r *playbackRun) bool {
	return s != nil && v.session == s && s.run == r && r.ctx.Err() == nil
}
func (v *InlineVideoPlayer) runCurrent(s *playerSession, r *playbackRun) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.runCurrentLocked(s, r)
}
func (v *InlineVideoPlayer) dispatchSession(s *playerSession, fn func()) {
	v.post(func() {
		if v.sessionCurrent(s) {
			fn()
		}
	})
}
func (v *InlineVideoPlayer) dispatchRun(s *playerSession, r *playbackRun, fn func()) {
	v.post(func() {
		if v.runCurrent(s, r) {
			fn()
		}
	})
}
func (v *InlineVideoPlayer) withEngine(fn func(media.PlaybackEngine)) *playerSession {
	v.mu.Lock()
	s := v.session
	ready := s != nil && v.engine != nil
	v.mu.Unlock()
	if !ready {
		return nil
	}
	s.ops.Lock()
	defer s.ops.Unlock()
	if !v.sessionCurrent(s) {
		return nil
	}
	fn(s.engine)
	return s
}
func (v *InlineVideoPlayer) dispatchIdle(fn func()) {
	v.post(func() {
		v.mu.Lock()
		idle := v.session == nil
		v.mu.Unlock()
		if idle {
			fn()
		}
	})
}

func retireSession(s *playerSession) {
	if s == nil {
		return
	}
	go func() {
		<-s.loaded
		if s.run != nil {
			<-s.run.done
		}
		s.ops.Lock()
		defer s.ops.Unlock()
		if s.scrubber != nil {
			s.scrubber.Stop()
		}
		if s.engine != nil {
			s.engine.Close()
			logging.Info(logging.CatPlayer, "player source session %d retired", s.id)
		}
	}()
}
func (v *InlineVideoPlayer) beginRun(s *playerSession, playing bool) *playbackRun {
	v.mu.Lock()
	defer v.mu.Unlock()
	if s == nil || v.session != s || v.engine == nil || s.ctx.Err() != nil {
		return nil
	}
	r := &playbackRun{done: make(chan struct{})}
	r.ctx, r.cancel = context.WithCancel(s.ctx)
	if old := s.run; old != nil {
		old.cancel()
		r.previous = old.done
	}
	s.run = r
	v.playing = playing
	return r
}
func (v *InlineVideoPlayer) playSession(s *playerSession) {
	v.mu.Lock()
	already := s != nil && v.session == s && v.playing && s.run != nil && s.run.ctx.Err() == nil
	v.mu.Unlock()
	if already {
		return
	}
	r := v.beginRun(s, true)
	if r == nil {
		return
	}
	v.notifyPlayState()
	go func() {
		defer close(r.done)
		if r.waitPrevious() {
			v.playFromRun(s, r)
		}
	}()
}
func (v *InlineVideoPlayer) playFromRun(s *playerSession, r *playbackRun) {
	s.ops.Lock()
	if !v.runCurrent(s, r) {
		s.ops.Unlock()
		return
	}
	// Start the decoder before consuming frames. Waiting before Resume
	// cannot produce a frame on initial play, and flushing on resume
	// discards valid source content and codec references.
	if !s.engine.IsRunning() {
		s.engine.Start()
	}
	if !v.runCurrent(s, r) {
		s.ops.Unlock()
		return
	}
	if err := s.engine.PreparePlayback(r.ctx, 500*time.Millisecond); err != nil {
		s.ops.Unlock()
		v.stopRun(s, r, err)
		return
	}
	if !v.runCurrent(s, r) {
		s.ops.Unlock()
		return
	}
	s.engine.Resume()
	s.ops.Unlock()
	v.dispatchRun(s, r, func() {
		if v.player != nil {
			v.player.SetPlaying(true)
		}
	})
	v.mu.Lock()
	peer := v.peer
	v.mu.Unlock()
	if peer != nil {
		peer.Play()
	}
	v.playbackLoop(s, r)
}
func (v *InlineVideoPlayer) stopRun(s *playerSession, r *playbackRun, err error) {
	if errors.Is(err, context.Canceled) || errors.Is(err, media.ErrEngineClosed) {
		return
	}
	v.mu.Lock()
	if !v.runCurrentLocked(s, r) {
		v.mu.Unlock()
		return
	}
	v.playing = false
	v.mu.Unlock()
	s.ops.Lock()
	if v.runCurrent(s, r) {
		s.engine.Pause()
	}
	s.ops.Unlock()
	v.notifyPlayState()
	v.dispatchRun(s, r, func() {
		if v.player != nil {
			v.player.SetPlaying(false)
			v.player.SetError(err.Error())
		}
	})
}
func (v *InlineVideoPlayer) seekAndPlaySession(s *playerSession, position float64) {
	if !v.sessionCurrent(s) {
		return
	}
	s.ops.Lock()
	if v.sessionCurrent(s) {
		_ = s.engine.Seek(position)
	}
	s.ops.Unlock()
	v.playSession(s)
}
func (v *InlineVideoPlayer) loadSession(path string, reset bool, open func(media.PlaybackEngine) error, expected *playerSession) (s *playerSession, err error) {
	v.mu.Lock()
	if expected != nil && v.session != expected {
		v.mu.Unlock()
		return nil, context.Canceled
	}
	old := v.session
	if old != nil {
		old.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	v.sourceGeneration++
	s = &playerSession{id: v.sourceGeneration, ctx: ctx, cancel: cancel, loaded: make(chan struct{})}
	v.session = s
	v.engine, v.scrubber = nil, nil
	v.playing = false
	v.currentPath = path
	resetSpeed, resetAudio, resetSub := v.speed != 1, v.audioIdx != -1, v.subtitleIdx != -1
	v.speed, v.audioIdx, v.subtitleIdx = 1, -1, -1
	speedCB, audioCB, subCB := v.onSpeedChange, v.onAudioTrack, v.onSubtitleTrk
	factory := v.newEngine
	if factory == nil {
		factory = newPlaybackEngine
	}
	if reset {
		v.playlist = nil
		v.playlistIdx = 0
	}
	rs := v.resumeState
	debug := v.diagnosticsEnabled
	v.diagnostics.Store(nil)
	v.mu.Unlock()
	retireSession(old)
	logging.Info(logging.CatPlayer, "player source session %d started", s.id)
	v.notifyPlayState()
	if resetSpeed && speedCB != nil {
		v.dispatchSession(s, func() { speedCB(1.0) })
	}
	if resetAudio && audioCB != nil {
		v.dispatchSession(s, func() { audioCB(-1) })
	}
	if resetSub && subCB != nil {
		v.dispatchSession(s, func() { subCB(-1) })
	}
	defer close(s.loaded)
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("player load failed: %v", recovered)
		}
		if err != nil && s.engine != nil {
			if s.scrubber != nil {
				s.scrubber.Stop()
				s.scrubber = nil
			}
			s.engine.Close()
		}
	}()
	emit := func(phase LoadPhase, err error) {
		v.mu.Lock()
		fn := v.onLoad
		v.mu.Unlock()
		v.dispatchSession(s, func() {
			if fn != nil {
				fn(LoadEvent{Phase: phase, At: time.Now(), Err: err})
			}
		})
	}
	emit(LoadPhaseStarted, nil)
	v.dispatchSession(s, func() {
		if v.player != nil {
			v.player.ClearError()
			v.player.SetLoading(true)
			v.player.SetChapters(nil)
			v.player.ClearThumbnailCache()
			v.player.SetFrame(nil)
			v.player.SetPlaying(false)
		}
	})
	s.ops.Lock()
	defer s.ops.Unlock()
	s.engine = factory()
	eng := s.engine
	eng.SetDiagnosticsEnabled(debug)
	eng.SetSeekAccuracy(media.DefaultSeekAccuracy())
	eng.SetAudioDelay(media.DefaultAudioDelay())
	eng.SetDropFrames(true)
	if hw := media.DetectHWDevice(); hw != media.HWDeviceNone {
		eng.SetHWDevice(hw)
	}
	if err = open(eng); err != nil {
		emit(LoadPhaseFailed, err)
		v.dispatchSession(s, func() {
			if v.player != nil {
				v.player.SetError(err.Error())
				v.player.SetLoading(false)
			}
		})
		return s, err
	}
	if !v.sessionCurrent(s) {
		return s, context.Canceled
	}
	emit(LoadPhaseOpen, nil)
	eng.InitFrameCache(30)
	chapters, duration, fps := eng.GetChapters(), eng.Duration(), eng.GetFrameRate()
	eng.Start()
	first, grabErr := eng.GrabFrame(4 * time.Second)
	if !v.sessionCurrent(s) {
		return s, context.Canceled
	}
	if grabErr == nil {
		emit(LoadPhaseFirstFrame, nil)
	} else {
		logging.Warning(logging.CatPlayer, "Load: first frame unavailable: %v", grabErr)
	}
	eng.Pause()
	if rs != nil {
		if saved, ok := rs.GetPosition(path); ok && rs.ShouldResume(saved) {
			if img, previewErr := grabResumePreview(eng, saved.Position); previewErr == nil {
				first = img
			} else if s.ctx.Err() != nil {
				return s, s.ctx.Err()
			} else {
				logging.Warning(logging.CatPlayer, "saved-position preview unavailable: %v", previewErr)
			}
		}
	}
	s.scrubber = eng.NewScrubber()
	if s.scrubber != nil {
		s.scrubber.SetOnFrame(func(img *image.RGBA) {
			v.dispatchSession(s, func() {
				if v.player != nil {
					v.player.SetFrame(img)
				}
				v.mu.Lock()
				fn := v.onFrame
				v.mu.Unlock()
				if fn != nil {
					fn(img)
				}
			})
		})
		s.scrubber.Start()
	}
	v.mu.Lock()
	if v.session != s || s.ctx.Err() != nil {
		v.mu.Unlock()
		return s, context.Canceled
	}
	previewPTS := eng.GetLastVideoPTS()
	v.engine, v.scrubber = eng, s.scrubber
	v.mu.Unlock()
	v.dispatchSession(s, func() {
		if v.player == nil {
			return
		}
		v.player.SetChapters(chapters)
		v.player.OnPrevChapter(func() { v.prevChapter() })
		v.player.OnNextChapter(func() { v.nextChapter() })
		v.player.SetDuration(duration)
		v.player.SetFrameRate(fps)
		if first != nil {
			v.player.SetFrame(first)
			v.player.SetCurrentTime(previewPTS)
		}
		v.player.SetLoading(false)
		v.player.Refresh()
	})
	emit(LoadPhaseReady, nil)
	go func() {
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		select {
		case <-s.ctx.Done():
			return
		case <-timer.C:
		}
		s.ops.Lock()
		if v.sessionCurrent(s) {
			eng.StartThumbnailExtraction(func(t float64, img *image.RGBA) {
				v.mu.Lock()
				if v.player != nil && v.session == s && s.ctx.Err() == nil {
					v.player.AddThumbnailFrame(t, img)
				}
				v.mu.Unlock()
			})
		}
		s.ops.Unlock()
	}()
	return s, nil
}

// grabResumePreview keeps the dev95 timed decoder contract: the playback
// frame queue has no producer until a presentation run starts.
func grabResumePreview(eng media.PlaybackEngine, position float64) (*image.RGBA, error) {
	if err := eng.Seek(position); err != nil {
		return nil, err
	}
	const budget = 4 * time.Second
	deadline := time.Now().Add(budget)
	remaining := budget
	for {
		img, err := eng.GrabFrame(remaining)
		if err != nil {
			return nil, err
		}
		if eng.GetLastVideoPTS() >= position {
			return img, nil
		}
		remaining = time.Until(deadline)
		if remaining <= 0 {
			return nil, fmt.Errorf("saved-position preview did not reach requested timestamp")
		}
	}
}
