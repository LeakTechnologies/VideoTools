package previewwork

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func await(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal("work did not finish")
	}
}

func pending(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
		t.Fatal("completed before worker exit")
	default:
	}
}

func TestRetiredContextCannotStartOrPublishAfterReenable(t *testing.T) {
	c := New(1)
	old := c.Context()
	if !c.IsCurrent(old) {
		t.Fatal("initial request not current")
	}
	await(t, c.SetEnabled(false))
	if c.Context() != nil || c.IsCurrent(old) {
		t.Fatal("disabled generation remains current")
	}
	if err := c.Run(nil, func(context.Context) error { t.Fatal("disabled work started"); return nil }); !errors.Is(err, ErrInactive) {
		t.Fatalf("disabled admission: %v", err)
	}
	await(t, c.SetEnabled(true))
	if err := c.Run(old, func(context.Context) error { t.Fatal("retired work started"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("retired admission: %v", err)
	}
	if c.IsCurrent(old) {
		t.Fatal("old result accepted after re-enable")
	}
	fresh := c.Context()
	if err := c.Run(fresh, func(ctx context.Context) error {
		if ctx != fresh {
			t.Fatal("lost owned context")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !c.IsCurrent(fresh) {
		t.Fatal("valid completed result cannot publish")
	}
	await(t, c.SetEnabled(false))
}

func TestConcurrencyBudgetAndQueuedCancellation(t *testing.T) {
	c := New(2)
	request := c.Context()
	entered := make(chan struct{}, 8)
	var running, maximum atomic.Int32
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() {
			results <- c.Run(request, func(ctx context.Context) error {
				n := running.Add(1)
				defer running.Add(-1)
				for {
					old := maximum.Load()
					if n <= old || maximum.CompareAndSwap(old, n) {
						break
					}
				}
				entered <- struct{}{}
				<-ctx.Done()
				return ctx.Err()
			})
		}()
	}
	await(t, entered)
	await(t, entered)
	select {
	case <-entered:
		t.Fatal("exceeded two running workers")
	case <-time.After(30 * time.Millisecond):
	}
	await(t, c.SetEnabled(false))
	for i := 0; i < 8; i++ {
		select {
		case err := <-results:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("work result: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("queued work did not exit")
		}
	}
	if maximum.Load() != 2 || running.Load() != 0 {
		t.Fatalf("maximum/current concurrency: %d/%d", maximum.Load(), running.Load())
	}
}

func TestDisableJoinsAllRetiringGenerations(t *testing.T) {
	c := New(2)
	start := func(request context.Context) (chan struct{}, chan struct{}) {
		entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
		go func() {
			defer close(finished)
			err := c.Run(request, func(context.Context) error { close(entered); <-release; return nil })
			if !errors.Is(err, context.Canceled) {
				t.Errorf("retired work result: %v", err)
			}
		}()
		await(t, entered)
		return release, finished
	}
	old := c.Context()
	releaseOld, oldFinished := start(old)
	first := c.SetEnabled(false)
	await(t, old.Done())
	pending(t, first)
	await(t, c.SetEnabled(true))
	releaseNew, newFinished := start(c.Context())
	second := c.SetEnabled(false)
	repeated := c.SetEnabled(false)
	pending(t, second)
	pending(t, repeated)
	close(releaseOld)
	await(t, oldFinished)
	await(t, first)
	pending(t, second)
	pending(t, repeated)
	close(releaseNew)
	await(t, newFinished)
	await(t, second)
	await(t, repeated)
}

func TestBudgetIncludesCancelledWorkersUntilTheyExit(t *testing.T) {
	c := New(1)
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	old := c.Context()
	go func() {
		defer close(finished)
		_ = c.Run(old, func(context.Context) error { close(entered); <-release; return nil })
	}()
	await(t, entered)
	retired := c.SetEnabled(false)
	await(t, c.SetEnabled(true))
	fresh := c.Context()
	newEntered, newFinished := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(newFinished)
		_ = c.Run(fresh, func(context.Context) error { close(newEntered); return nil })
	}()
	select {
	case <-newEntered:
		t.Fatal("new generation exceeded budget while old worker lived")
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	await(t, finished)
	await(t, retired)
	await(t, newEntered)
	await(t, newFinished)
	await(t, c.SetEnabled(false))
}

func TestForeignContextsRejectedAndWorkErrorsPreserved(t *testing.T) {
	c := New(1)
	if err := c.Run(context.Background(), func(context.Context) error { t.Fatal("foreign request admitted"); return nil }); !errors.Is(err, ErrInactive) {
		t.Fatal(err)
	}
	failure := errors.New("neutral preparation failure")
	if err := c.Run(c.Context(), func(context.Context) error { return failure }); !errors.Is(err, failure) {
		t.Fatalf("lost work error: %v", err)
	}
	if err := c.Run(c.Context(), func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	await(t, c.SetEnabled(false))
}

func TestConcurrentAdmissionAndPolicyChanges(t *testing.T) {
	c := New(2)
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		request := c.Context()
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := c.Run(request, func(context.Context) error { return nil })
			if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, ErrInactive) {
				t.Errorf("unexpected admission result: %v", err)
			}
		}()
		c.SetEnabled(false)
		c.SetEnabled(true)
	}
	await(t, c.SetEnabled(false))
	wg.Wait()
}
