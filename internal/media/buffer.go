//go:build native_media

package media

import (
	"time"

	"github.com/LeakTechnologies/VideoTools/internal/logging"
)

func (e *Engine) SetBufferMode(mode BufferMode) {
	e.lockMu()
	defer e.unlockMu()
	e.bufferMode = mode
}

func (e *Engine) GetBufferMode() BufferMode {
	e.lockMu()
	defer e.unlockMu()
	return e.bufferMode
}

func (e *Engine) GetAdaptiveBufferSize() int {
	e.lockMu()
	defer e.unlockMu()

	switch e.bufferMode {
	case BufferModeMinimal:
		return 10
	case BufferModeNormal:
		return 50
	case BufferModeAggressive:
		return 100
	default:
		return 50
	}
}

func (e *Engine) recordDecodeTime(duration time.Duration) {
	e.lockMu()
	defer e.unlockMu()

	e.decodeTimes = append(e.decodeTimes, duration)
	if len(e.decodeTimes) > 30 {
		e.decodeTimes = e.decodeTimes[len(e.decodeTimes)-30:]
	}
	e.lastDecodeTime = time.Now()
}

func (e *Engine) GetDecodeTimeTrend() float64 {
	e.lockMu()
	defer e.unlockMu()

	if len(e.decodeTimes) < 5 {
		return 0
	}

	oldAvg := 0.0
	newAvg := 0.0
	half := len(e.decodeTimes) / 2

	for i, t := range e.decodeTimes {
		ms := t.Seconds() * 1000
		if i < half {
			oldAvg += ms
		} else {
			newAvg += ms
		}
	}

	oldAvg /= float64(half)
	newAvg /= float64(len(e.decodeTimes) - half)

	if oldAvg == 0 {
		return 0
	}

	return (newAvg - oldAvg) / oldAvg
}

func (e *Engine) AdjustBufferForPerformance() {
	trend := e.GetDecodeTimeTrend()

	e.lockMu()
	defer e.unlockMu()

	if trend > 0.3 {
		newSize := e.frameCache.Size() + 10
		if newSize > 100 {
			newSize = 100
		}
		if e.frameCache != nil {
			e.frameCache.SetMaxSize(newSize)
		}
		logging.Debug(logging.CatPlayer, "Buffer increased to %d (decode trend: %.1f%%)", newSize, trend*100)
	} else if trend < -0.2 && e.frameCache != nil {
		currentSize := e.frameCache.Size()
		if currentSize > 15 {
			newSize := currentSize - 10
			e.frameCache.SetMaxSize(newSize)
			logging.Debug(logging.CatPlayer, "Buffer decreased to %d (decode trend: %.1f%%)", newSize, trend*100)
		}
	}
}

func (e *Engine) GetAverageDecodeTime() time.Duration {
	e.lockMu()
	defer e.unlockMu()

	if len(e.decodeTimes) == 0 {
		return 0
	}

	var total time.Duration
	for _, t := range e.decodeTimes {
		total += t
	}
	return total / time.Duration(len(e.decodeTimes))
}

func (e *Engine) GetVideoBufferDepth() int {
	if e.videoQueue == nil {
		return 0
	}
	return e.videoQueue.Size()
}

func (e *Engine) GetAudioBufferDepth() int {
	if e.audioQueue == nil {
		return 0
	}
	return e.audioQueue.Size()
}

func (e *Engine) GetBufferHealth() float64 {
	videoDepth := e.GetVideoBufferDepth()
	audioDepth := e.GetAudioBufferDepth()

	videoMax := 50
	audioMax := 100

	if e.videoQueue != nil {
		videoMax = e.videoQueue.MaxSize()
	}
	if e.audioQueue != nil {
		audioMax = e.audioQueue.MaxSize()
	}

	videoHealth := float64(videoDepth) / float64(videoMax)
	audioHealth := float64(audioDepth) / float64(audioMax)

	return (videoHealth + audioHealth) / 2.0
}

func (e *Engine) IsBuffering() bool {
	health := e.GetBufferHealth()
	return health < 0.2
}
