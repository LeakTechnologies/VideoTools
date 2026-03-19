//go:build native_media

package media

import (
	"sync"
	"time"
)

const (
	MaxDriftThreshold = 0.1
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
	}
}

func (c *MasterClock) SetTime(pts float64) {
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
	wasPaused := c.paused
	if !wasPaused {
		c.pts = c.GetTime()
		c.ptsTime = time.Now()
	}
	c.speed = speed
	if wasPaused {
		c.paused = true
	} else {
		c.paused = false
	}
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
