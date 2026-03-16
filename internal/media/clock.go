package media

import (
	"sync"
	"time"
)

// MasterClock tracks the master playback time for A/V synchronization.
type MasterClock struct {
	mu        sync.Mutex
	pts       float64   // Current PTS in seconds
	ptsTime   time.Time // Real time when PTS was last updated
	paused    bool
	startTime time.Time
}

// NewMasterClock creates a new master clock.
func NewMasterClock() *MasterClock {
	return &MasterClock{
		startTime: time.Now(),
		ptsTime:   time.Now(),
	}
}

// SetTime updates the clock with a new PTS value.
func (c *MasterClock) SetTime(pts float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pts = pts
	c.ptsTime = time.Now()
}

// GetTime returns the current master time in seconds.
func (c *MasterClock) GetTime() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.paused {
		return c.pts
	}
	elapsed := time.Since(c.ptsTime).Seconds()
	return c.pts + elapsed
}

// SyncVideo calculates the delay needed to wait before displaying a video frame.
func (c *MasterClock) SyncVideo(pts float64) time.Duration {
	master := c.GetTime()
	diff := pts - master
	
	if diff <= 0 {
		return 0 // Too late, display immediately
	}
	
	if diff > 1.0 {
		return 10 * time.Millisecond // Sanity check for huge gaps
	}
	
	return time.Duration(diff * float64(time.Second))
}

// Pause/Resume
func (c *MasterClock) SetPaused(paused bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.paused == paused {
		return
	}
	if paused {
		c.pts = c.GetTime()
	} else {
		c.ptsTime = time.Now()
	}
	c.paused = paused
}
