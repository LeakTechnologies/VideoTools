// Package previewwork owns optional background preparation independently of
// processing jobs and explicit manual playback.
package previewwork

import (
	"context"
	"errors"
	"sync"
)

var ErrInactive = errors.New("automatic preview work request is disabled or retired")

type generation struct {
	ctx    context.Context
	cancel context.CancelFunc
	work   sync.WaitGroup
	retire sync.Once
	done   chan struct{}
}

// Controller bounds running work across all generations, including cancelled
// workers that have not exited. Construct with New; do not copy a Controller.
type Controller struct {
	mu          sync.Mutex
	slots       chan struct{}
	current     *generation
	generations map[*generation]struct{}
}

// New enables automatic work without starting any goroutines or I/O. limit is
// the host's positive concurrency budget, not a media-derived allocation size.
func New(limit int) *Controller {
	if limit < 1 {
		panic("previewwork: concurrency limit must be positive")
	}
	c := &Controller{slots: make(chan struct{}, limit), generations: make(map[*generation]struct{})}
	c.current = c.newGeneration()
	return c
}

func (c *Controller) newGeneration() *generation {
	ctx, cancel := context.WithCancel(context.Background())
	g := &generation{ctx: ctx, cancel: cancel, done: make(chan struct{})}
	c.generations[g] = struct{}{}
	return g
}

// Context captures the current request generation before scheduling work. Pass
// this exact context to Run and IsCurrent; nil means preparation is disabled.
// A context captured before disabling remains invalid after re-enabling.
func (c *Controller) Context() context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.current == nil {
		return nil
	}
	return c.current.ctx
}

// IsCurrent checks whether a queued result still belongs to the active policy.
// UI publication should check this on the same UI thread that changes policy,
// as well as checking the selected source identity. This is a snapshot, not a
// lock held across arbitrary UI code.
func (c *Controller) IsCurrent(request context.Context) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current != nil && request == c.current.ctx && request.Err() == nil
}

// Run joins work to its captured generation before waiting for a concurrency
// slot. The caller owns scheduling this synchronous call. Work must pass ctx to
// subprocesses/decoders and honor cancellation; completion never pretends a
// worker has exited merely because its context was cancelled.
func (c *Controller) Run(request context.Context, work func(context.Context) error) error {
	c.mu.Lock()
	g := c.current
	if g == nil || request != g.ctx || request.Err() != nil {
		c.mu.Unlock()
		if request != nil && request.Err() != nil {
			return request.Err()
		}
		return ErrInactive
	}
	g.work.Add(1)
	c.mu.Unlock()
	defer g.work.Done()
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	case <-request.Done():
		return request.Err()
	}
	if err := request.Err(); err != nil {
		return err
	}
	err := work(request)
	if request.Err() != nil {
		return request.Err()
	}
	return err
}

// SetEnabled changes admission immediately and returns a completion signal.
// Disabling cancels and joins every retiring generation; wait outside the UI
// thread and outside managed work (a worker cannot join its own completion).
// Re-enabling creates a fresh generation but never schedules work implicitly.
func (c *Controller) SetEnabled(enabled bool) <-chan struct{} {
	c.mu.Lock()
	var retiring []*generation
	if enabled {
		if c.current == nil {
			c.current = c.newGeneration()
		}
	} else {
		c.current = nil
		for g := range c.generations {
			g.cancel()
			retiring = append(retiring, g)
		}
	}
	c.mu.Unlock()
	done := make(chan struct{})
	if len(retiring) == 0 {
		close(done)
		return done
	}
	for _, g := range retiring {
		g.retire.Do(func() {
			go func() {
				g.work.Wait()
				c.mu.Lock()
				delete(c.generations, g)
				c.mu.Unlock()
				close(g.done)
			}()
		})
	}
	go func() {
		defer close(done)
		for _, g := range retiring {
			<-g.done
		}
	}()
	return done
}
