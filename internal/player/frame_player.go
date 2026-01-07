package player

import (
	"image"
	"time"
)

type framePlayer interface {
	Load(path string, offset time.Duration) error
	Play() error
	Pause() error
	SeekToTime(offset time.Duration) error
	SeekToFrame(frame int64) error
	GetCurrentTime() time.Duration
	GetFrameImage() (*image.RGBA, error)
	SetVolume(level float64) error
	Close()
}
