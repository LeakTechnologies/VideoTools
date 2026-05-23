//go:build native_media

package media

import (
	"sync"
	"time"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
)

const (
	// MaxDriftThreshold is how far behind the clock a video frame can be
	// before it is dropped entirely. 300ms gives enough headroom for a slow
	// single-threaded H.264 I-frame decode (80-150ms) plus the synchronous
	// Fyne render dispatch (~14ms) without triggering spurious drops at
	// every GOP boundary (~2s for typical content).
	MaxDriftThreshold = 0.3
	MaxWaitTime       = 100 * time.Millisecond
	RealtimeSpeed     = 1.0
)

type MasterClock struct {
	mu        sync.Mutex
	pts       float64
	ptsTime   time.Time
	paused    bool
	speed     float64
	startTime time.Time
}

func NewMasterClock() *MasterClock {
	return &MasterClock{
		startTime: time.Now(),
		ptsTime:   time.Now(),
		speed:     RealtimeSpeed,
		paused: true, // stays paused through Load/GrabFrame; unpaused by Engine.Resume() on first Play.
	}
}

// SetTime advances the clock to pts. It is a monotonic ratchet: if pts is
// less than the *current computed clock value* (anchor + wall-elapsed) the
// call is a no-op. This prevents audio chunks — whether pre-buffered by oto
// before playback or re-anchored mid-stream — from resetting ptsTime and
// collapsing the wall-elapsed component, which was the root cause of
// WaitForPTS hanging after a brief backward clock jump.
//
// Exception: if the PTS is behind by more than 1s AND no SetTime has been
// called in the last 500ms, the ratchet allows the reset. This handles
// audio underrun recovery — the clock coasted on wall-time while audio was
// absent, and the next audio PTS reflects where playback actually is.
func (c *MasterClock) SetTime(pts float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var current float64
	if c.paused {
		current = c.pts
	} else {
		current = c.pts + time.Since(c.ptsTime).Seconds()*c.speed
	}
	if pts <= current {
		backwardJump := current - pts
		// Allow reset on underrun recovery: large backward jump (> 1s) with
		// no recent PTS anchoring (> 500ms idle).
		if backwardJump > 1.0 && time.Since(c.ptsTime) > 500*time.Millisecond {
			logging.Debug(logging.CatPlayer, "clock: SetTime underrun recovery, reset %.3f→%.3f (jump %.0fms, idle %.0fms)",
				current, pts, backwardJump*1000, time.Since(c.ptsTime).Seconds()*1000)
			c.pts = pts
			c.ptsTime = time.Now()
		}
		return
	}
	jump := pts - current
	if jump > 0.040 {
		// Clock advancing faster than real-time; log for sync diagnostics.
		// Normal: wall-time elapsed + 1 chunk period (~23 ms). Anything larger
		// suggests audio pre-buffering or a codec event drove the clock forward.
		logging.Debug(logging.CatPlayer, "clock: SetTime jump +%.0fms (%.3f→%.3f)", jump*1000, current, pts)
	}
	c.pts = pts
	c.ptsTime = time.Now()
}

// ResetTime unconditionally sets the clock to pts, including backward jumps.
// Use this after seek operations where the PTS will legitimately decrease.
func (c *MasterClock) ResetTime(pts float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pts = pts
	c.ptsTime = time.Now()
}

func (c *MasterClock) GetTime() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.paused {
		return c.pts
	}
	elapsed := time.Since(c.ptsTime).Seconds() * c.speed
	return c.pts + elapsed
}

func (c *MasterClock) SetPaused(paused bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.paused == paused {
		return
	}
	if paused {
		c.pts = c.pts + time.Since(c.ptsTime).Seconds()*c.speed
	} else {
		c.ptsTime = time.Now()
	}
	c.paused = paused
}

func (c *MasterClock) IsPaused() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.paused
}

func (c *MasterClock) SetSpeed(speed float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.paused {
		// Inline GetTime() logic — calling GetTime() here would deadlock because
		// that method also acquires c.mu and Go mutexes are not re-entrant.
		elapsed := time.Since(c.ptsTime).Seconds() * c.speed
		c.pts = c.pts + elapsed
		c.ptsTime = time.Now()
	}
	c.speed = speed
}

func (c *MasterClock) GetSpeed() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.speed
}

func (c *MasterClock) WaitForPTS(targetPTS float64) {
	c.mu.Lock()
	paused := c.paused
	c.mu.Unlock()

	if paused {
		return
	}

	for {
		c.mu.Lock()
		paused := c.paused
		c.mu.Unlock()
		if paused {
			return
		}

		master := c.GetTime()
		diff := targetPTS - master

		if diff <= 0 {
			return
		}

		if diff > MaxDriftThreshold {
			time.Sleep(MaxWaitTime)
			continue
		}

		time.Sleep(time.Duration(diff * float64(time.Second)))
		return
	}
}

func (c *MasterClock) SyncVideo(pts float64) time.Duration {
	master := c.GetTime()
	diff := pts - master

	if diff <= -MaxDriftThreshold {
		return -1
	}

	if diff <= 0 {
		return 0
	}

	if diff > 1.0 {
		return 10 * time.Millisecond
	}

	return time.Duration(diff * float64(time.Second))
}
