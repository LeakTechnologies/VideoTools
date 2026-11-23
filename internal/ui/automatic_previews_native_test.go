//go:build native_media

package ui

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/LeakTechnologies/VideoTools/internal/media"
)

type automaticTestEngine struct {
	*sessionEngine
	opens  atomic.Int32
	closes atomic.Int32
}

func (e *automaticTestEngine) OpenAuto(string) error { e.opens.Add(1); return nil }
func (e *automaticTestEngine) Close()                { e.closes.Add(1); e.sessionEngine.Close() }

func TestAutomaticLoadPublicAPIAndRepeatedRetirement(t *testing.T) {
	e := &automaticTestEngine{sessionEngine: fakeSessionEngine()}
	p := previewTestPlayer(t, func() media.PlaybackEngine { return e })
	if err := p.LoadAutomatic("neutral-preview"); err != nil {
		t.Fatal(err)
	}
	first := p.SetAutomaticPreviewsEnabled(false)
	second := p.SetAutomaticPreviewsEnabled(false)
	waitSignal(t, first)
	waitSignal(t, second)
	if e.opens.Load() != 1 || e.closes.Load() != 1 {
		t.Fatalf("open/close counts = %d/%d", e.opens.Load(), e.closes.Load())
	}
	p.mu.Lock()
	pending := len(p.automaticSessions)
	p.mu.Unlock()
	if pending != 0 {
		t.Fatal("retired automatic source retained by policy")
	}
}

func previewTestPlayer(t *testing.T, factory func() media.PlaybackEngine) *InlineVideoPlayer {
	t.Helper()
	enabled := media.HWDecodeEnabled()
	media.SetHWDecodeEnabled(false)
	t.Cleanup(func() { media.SetHWDecodeEnabled(enabled) })
	p := &InlineVideoPlayer{newEngine: factory, dispatch: func(fn func()) { fn() }, speed: 1, audioIdx: -1, subtitleIdx: -1}
	t.Cleanup(p.Close)
	return p
}

func TestDisabledAutomaticLoadDoesNotAllocateOrReplaceManual(t *testing.T) {
	e := fakeSessionEngine()
	var factories atomic.Int32
	p := previewTestPlayer(t, func() media.PlaybackEngine { factories.Add(1); return e })
	if err := p.loadViaOpen("manual", true, func(media.PlaybackEngine) error { return nil }); err != nil {
		t.Fatal(err)
	}
	waitSignal(t, p.SetAutomaticPreviewsEnabled(false))
	if err := p.LoadAutomatic("automatic"); !errors.Is(err, ErrAutomaticPreviewsDisabled) {
		t.Fatalf("denied automatic load: %v", err)
	}
	if factories.Load() != 1 || p.CurrentPath() != "manual" || p.engine != e {
		t.Fatal("policy replaced or allocated over manual source")
	}
	select {
	case <-e.closed:
		t.Fatal("manual engine closed by preview policy")
	default:
	}
}

func TestAutomaticDisabledAtStartedDoesNotCreateEngine(t *testing.T) {
	var count atomic.Int32
	p := previewTestPlayer(t, func() media.PlaybackEngine { count.Add(1); return fakeSessionEngine() })
	var retired <-chan struct{}
	p.SetOnLoad(func(event LoadEvent) {
		if event.Phase == LoadPhaseStarted {
			retired = p.SetAutomaticPreviewsEnabled(false)
		}
	})
	_, err := p.loadOwnedSession("automatic", true, func(media.PlaybackEngine) error {
		t.Fatal("open called after policy cancellation")
		return nil
	}, nil, true)
	if !errors.Is(err, context.Canceled) || count.Load() != 0 {
		t.Fatalf("retired admission allocated engine: count=%d, err=%v", count.Load(), err)
	}
	waitSignal(t, retired)
}

func TestDisableWaitsForPresentationAndPreservesManualReplacement(t *testing.T) {
	auto, manual := fakeSessionEngine(), fakeSessionEngine()
	auto.ignoreCancel = true
	auto.release = make(chan struct{})
	var count atomic.Int32
	p := previewTestPlayer(t, func() media.PlaybackEngine {
		if count.Add(1) == 1 {
			return auto
		}
		return manual
	})
	if _, err := p.loadOwnedSession("automatic", true, func(media.PlaybackEngine) error { return nil }, nil, true); err != nil {
		t.Fatal(err)
	}
	p.Play()
	waitSignal(t, auto.entered)
	retired := p.SetAutomaticPreviewsEnabled(false)
	select {
	case <-retired:
		t.Fatal("retirement completed during native frame read")
	default:
	}
	if err := p.loadViaOpen("manual", true, func(media.PlaybackEngine) error { return nil }); err != nil {
		t.Fatal(err)
	}
	close(auto.release)
	waitSignal(t, retired)
	if p.CurrentPath() != "manual" || p.engine != manual {
		t.Fatal("automatic retirement replaced manual session")
	}
	select {
	case <-manual.closed:
		t.Fatal("automatic retirement closed manual session")
	default:
	}
}

func TestDisableWaitsForAutomaticOpenAndRejectsLatePublication(t *testing.T) {
	e := fakeSessionEngine()
	p := previewTestPlayer(t, func() media.PlaybackEngine { return e })
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var loadErr error
	go func() {
		defer close(finished)
		_, loadErr = p.loadOwnedSession("automatic", true, func(media.PlaybackEngine) error {
			close(entered)
			<-release
			return nil
		}, nil, true)
	}()
	waitSignal(t, entered)
	p.mu.Lock()
	old := p.session
	p.mu.Unlock()
	retired := p.SetAutomaticPreviewsEnabled(false)
	select {
	case <-retired:
		t.Fatal("retirement completed before native open returned")
	default:
	}
	if p.CurrentPath() != "" || old.ctx.Err() == nil {
		t.Fatal("automatic source not detached and cancelled")
	}
	called := false
	p.dispatchSession(old, func() { called = true })
	if called {
		t.Fatal("retired automatic callback published")
	}
	close(release)
	waitSignal(t, finished)
	waitSignal(t, retired)
	waitSignal(t, e.closed)
	if !errors.Is(loadErr, context.Canceled) {
		t.Fatalf("retired load returned %v", loadErr)
	}
	if err := p.LoadAutomatic("late"); !errors.Is(err, ErrAutomaticPreviewsDisabled) {
		t.Fatalf("late request admitted: %v", err)
	}
}

func TestDisableJoinsReplacedAutomaticWithoutClosingManual(t *testing.T) {
	auto, manual := fakeSessionEngine(), fakeSessionEngine()
	var count atomic.Int32
	p := previewTestPlayer(t, func() media.PlaybackEngine {
		if count.Add(1) == 1 {
			return auto
		}
		return manual
	})
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(finished)
		_, _ = p.loadOwnedSession("automatic", true, func(media.PlaybackEngine) error {
			close(entered)
			<-release
			return nil
		}, nil, true)
	}()
	waitSignal(t, entered)
	if err := p.loadViaOpen("manual", true, func(media.PlaybackEngine) error { return nil }); err != nil {
		t.Fatal(err)
	}
	retired := p.SetAutomaticPreviewsEnabled(false)
	select {
	case <-retired:
		t.Fatal("forgot a replaced automatic load")
	default:
	}
	close(release)
	waitSignal(t, finished)
	waitSignal(t, retired)
	if p.CurrentPath() != "manual" || p.engine != manual {
		t.Fatal("manual replacement lost")
	}
	select {
	case <-manual.closed:
		t.Fatal("manual replacement closed")
	default:
	}
}

func TestAutomaticOwnershipSurvivesReloadAndReenableDoesNotResurrect(t *testing.T) {
	first, second, third := fakeSessionEngine(), fakeSessionEngine(), fakeSessionEngine()
	engines := []*sessionEngine{first, second, third}
	var count atomic.Int32
	p := previewTestPlayer(t, func() media.PlaybackEngine { return engines[count.Add(1)-1] })
	s, err := p.loadOwnedSession("automatic", true, func(media.PlaybackEngine) error { return nil }, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.loadSession("automatic-next", false, func(media.PlaybackEngine) error { return nil }, s); err != nil {
		t.Fatal(err)
	}
	waitSignal(t, p.SetAutomaticPreviewsEnabled(false))
	waitSignal(t, first.closed)
	waitSignal(t, second.closed)
	waitSignal(t, p.SetAutomaticPreviewsEnabled(true))
	if p.CurrentPath() != "" || count.Load() != 2 {
		t.Fatal("reenable resurrected old source")
	}
	if _, err = p.loadOwnedSession("fresh", true, func(media.PlaybackEngine) error { return nil }, nil, true); err != nil {
		t.Fatal(err)
	}
	if count.Load() != 3 || p.CurrentPath() != "fresh" {
		t.Fatal("fresh automatic request not admitted")
	}
	waitSignal(t, p.SetAutomaticPreviewsEnabled(false))
	waitSignal(t, third.closed)
}
