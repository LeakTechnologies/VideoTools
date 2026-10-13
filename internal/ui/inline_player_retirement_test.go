//go:build native_media

package ui

import (
	"context"
	"errors"
	"image"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LeakTechnologies/VideoTools/internal/media"
)

type sessionEngine struct {
	media.PlaybackEngine
	entered      chan struct{}
	release      chan struct{}
	closed       chan struct{}
	closeOnce    sync.Once
	active       atomic.Int32
	maximum      atomic.Int32
	reads        atomic.Int32
	ignoreCancel bool
	fail         error
	finishErr    error
	waits        atomic.Int32
}

func fakeSessionEngine() *sessionEngine {
	return &sessionEngine{entered: make(chan struct{}, 200), closed: make(chan struct{})}
}
func (e *sessionEngine) SetSeekAccuracy(media.SeekAccuracy) {}
func (e *sessionEngine) SetAudioDelay(float64)              {}
func (e *sessionEngine) SetDropFrames(bool)                 {}
func (e *sessionEngine) SetHWDevice(media.HWDeviceType)     {}
func (e *sessionEngine) InitFrameCache(int)                 {}
func (e *sessionEngine) GetChapters() []media.Chapter       { return nil }
func (e *sessionEngine) Duration() float64                  { return 20 }
func (e *sessionEngine) GetFrameRate() float64              { return 25 }
func (e *sessionEngine) Start()                             {}
func (e *sessionEngine) ResetAfterGrab()                    {}
func (e *sessionEngine) Pause()                             {}
func (e *sessionEngine) Resume()                            {}
func (e *sessionEngine) IsRunning() bool                    { return true }
func (e *sessionEngine) FlushAudioCodec()                   {}
func (e *sessionEngine) WaitForFrame(time.Duration) bool    { e.waits.Add(1); return true }
func (e *sessionEngine) GrabFrame(time.Duration) (*image.RGBA, error) {
	return image.NewRGBA(image.Rect(0, 0, 2, 2)), nil
}
func (e *sessionEngine) NewScrubber() media.Scrubber                         { return nil }
func (e *sessionEngine) StartThumbnailExtraction(func(float64, *image.RGBA)) {}
func (e *sessionEngine) CurrentTime() float64                                { return 5 }
func (e *sessionEngine) GetLastVideoPTS() float64                            { return 5 }
func (e *sessionEngine) IsGrowingFile() bool                                 { return false }
func (e *sessionEngine) Seek(float64) error                                  { return nil }
func (e *sessionEngine) Close()                                              { e.closeOnce.Do(func() { close(e.closed) }) }
func (e *sessionEngine) NextFrameContext(ctx context.Context) (*image.RGBA, error) {
	n := e.active.Add(1)
	defer e.active.Add(-1)
	for n > e.maximum.Load() {
		old := e.maximum.Load()
		if n <= old || e.maximum.CompareAndSwap(old, n) {
			break
		}
	}
	e.reads.Add(1)
	select {
	case e.entered <- struct{}{}:
	default:
	}
	if e.fail != nil {
		return nil, e.fail
	}
	if e.ignoreCancel {
		<-e.release
		if e.finishErr != nil {
			return nil, e.finishErr
		}
		return nil, errors.New("retired native read")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-e.release:
		return nil, errors.New("test read")
	}
}
func (e *sessionEngine) StepContext(ctx context.Context, _ int) (*image.RGBA, error) {
	return e.NextFrameContext(ctx)
}
func waitSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("lifecycle operation did not finish")
	}
}
func attachedPlayer(e *sessionEngine) *InlineVideoPlayer {
	ctx, cancel := context.WithCancel(context.Background())
	s := &playerSession{ctx: ctx, cancel: cancel, loaded: make(chan struct{}), engine: e}
	close(s.loaded)
	return &InlineVideoPlayer{session: s, engine: e, speed: 1, audioIdx: -1, subtitleIdx: -1, dispatch: func(fn func()) { fn() }}
}
func TestPresentationRunsJoinBeforeRestart(t *testing.T) {
	e := fakeSessionEngine()
	e.ignoreCancel = true
	e.release = make(chan struct{})
	p := attachedPlayer(e)
	p.Play()
	waitSignal(t, e.entered)
	p.Pause()
	p.Play()
	time.Sleep(20 * time.Millisecond)
	if e.reads.Load() != 1 {
		t.Fatal("new consumer entered before retired native read finished")
	}
	close(e.release)
	deadline := time.Now().Add(time.Second)
	for p.IsPlaying() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if e.maximum.Load() != 1 {
		t.Fatalf("concurrent consumers: %d", e.maximum.Load())
	}
	p.Close()
	waitSignal(t, e.closed)
}
func TestRapidTransportHasSingleConsumer(t *testing.T) {
	e := fakeSessionEngine()
	p := attachedPlayer(e)
	p.Play()
	waitSignal(t, e.entered)
	for i := 0; i < 50; i++ {
		p.Pause()
		p.Play()
	}
	p.Close()
	waitSignal(t, e.closed)
	if e.maximum.Load() > 1 {
		t.Fatalf("concurrent frame reads: %d", e.maximum.Load())
	}
}
func TestCloseCancelsPausedSeekAndStep(t *testing.T) {
	for _, step := range []bool{false, true} {
		e := fakeSessionEngine()
		p := attachedPlayer(e)
		if step {
			p.StepFrame(1)
		} else {
			p.Seek(5)
		}
		waitSignal(t, e.entered)
		p.Close()
		waitSignal(t, e.closed)
	}
}
func TestPlaybackFailureStopsAuthorityAndCanRestart(t *testing.T) {
	e := fakeSessionEngine()
	e.fail = errors.New("decode failed")
	p := attachedPlayer(e)
	p.Play()
	waitSignal(t, e.entered)
	deadline := time.Now().Add(time.Second)
	for p.IsPlaying() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if p.IsPlaying() {
		t.Fatal("failed playback still reports playing")
	}
	p.Play()
	waitSignal(t, e.entered)
	if e.reads.Load() != 2 {
		t.Fatal("restart did not start a frame consumer")
	}
	p.Close()
	waitSignal(t, e.closed)
}
func TestStaleSessionCallbacksAreDiscarded(t *testing.T) {
	e := fakeSessionEngine()
	p := attachedPlayer(e)
	var queued []func()
	p.dispatch = func(fn func()) { queued = append(queued, fn) }
	called := false
	p.dispatchSession(p.session, func() { called = true })
	p.Close()
	for _, fn := range queued {
		fn()
	}
	if called {
		t.Fatal("retired source delivered queued callback")
	}
	waitSignal(t, e.closed)
}
func TestOlderLoadCannotReplaceNewerSource(t *testing.T) {
	enabled := media.HWDecodeEnabled()
	media.SetHWDecodeEnabled(false)
	defer media.SetHWDecodeEnabled(enabled)
	first, second := fakeSessionEngine(), fakeSessionEngine()
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var factories atomic.Int32
	p := &InlineVideoPlayer{newEngine: func() media.PlaybackEngine {
		if factories.Add(1) == 1 {
			return first
		}
		return second
	}, dispatch: func(fn func()) { fn() }, speed: 1, audioIdx: -1, subtitleIdx: -1}
	var firstErr error
	go func() {
		defer close(done)
		firstErr = p.loadViaOpen("first", true, func(media.PlaybackEngine) error { close(entered); <-release; return nil })
	}()
	waitSignal(t, entered)
	if err := p.loadViaOpen("second", true, func(media.PlaybackEngine) error { return nil }); err != nil {
		t.Fatal(err)
	}
	close(release)
	waitSignal(t, done)
	if !errors.Is(firstErr, context.Canceled) {
		t.Fatalf("stale load returned %v", firstErr)
	}
	p.mu.Lock()
	actual := p.engine
	p.mu.Unlock()
	if actual != second || p.CurrentPath() != "second" {
		t.Fatal("older load replaced current source")
	}
	waitSignal(t, first.closed)
	p.Close()
	waitSignal(t, second.closed)
}
func TestCloseDuringLoadCannotResurrectSource(t *testing.T) {
	enabled := media.HWDecodeEnabled()
	media.SetHWDecodeEnabled(false)
	defer media.SetHWDecodeEnabled(enabled)
	e := fakeSessionEngine()
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	p := &InlineVideoPlayer{newEngine: func() media.PlaybackEngine { return e }, dispatch: func(fn func()) { fn() }}
	var loadErr error
	go func() {
		defer close(done)
		loadErr = p.loadViaOpen("test", true, func(media.PlaybackEngine) error { close(entered); <-release; return nil })
	}()
	waitSignal(t, entered)
	p.Close()
	close(release)
	waitSignal(t, done)
	waitSignal(t, e.closed)
	if !errors.Is(loadErr, context.Canceled) {
		t.Fatalf("retired load returned %v", loadErr)
	}
	p.mu.Lock()
	actual := p.engine
	p.mu.Unlock()
	if actual != nil || p.CurrentPath() != "" {
		t.Fatal("load resurrected closed player")
	}
}

func TestRetiredEOFDoesNotReloadOrStopReplacement(t *testing.T) {
	enabled := media.HWDecodeEnabled()
	media.SetHWDecodeEnabled(false)
	defer media.SetHWDecodeEnabled(enabled)
	old := fakeSessionEngine()
	old.ignoreCancel = true
	old.finishErr = io.EOF
	old.release = make(chan struct{})
	next := fakeSessionEngine()
	p := attachedPlayer(old)
	p.newEngine = func() media.PlaybackEngine { return next }
	var ended atomic.Int32
	p.SetOnEnd(func() { ended.Add(1) })
	p.Play()
	waitSignal(t, old.entered)
	if err := p.loadViaOpen("replacement", true, func(media.PlaybackEngine) error { return nil }); err != nil {
		t.Fatal(err)
	}
	p.Play()
	waitSignal(t, next.entered)
	close(old.release)
	waitSignal(t, old.closed)
	if !p.IsPlaying() || p.CurrentPath() != "replacement" || ended.Load() != 0 {
		t.Fatal("retired EOF affected replacement session")
	}
	p.Close()
	waitSignal(t, next.closed)
}

func TestPlayDoesNotWaitBeforeStartingDecoder(t *testing.T) {
	e := fakeSessionEngine()
	p := attachedPlayer(e)
	p.Play()
	waitSignal(t, e.entered)
	if e.waits.Load() != 0 {
		t.Fatal("Play waited for a frame before starting the decoder")
	}
	p.Close()
	waitSignal(t, e.closed)
}

func (e *sessionEngine) PreparePlayback(context.Context, time.Duration) error { return nil }

func (e *sessionEngine) SetDiagnosticsEnabled(bool)          {}
func (e *sessionEngine) Diagnostics() media.PlayerDebugStats { return media.PlayerDebugStats{} }
