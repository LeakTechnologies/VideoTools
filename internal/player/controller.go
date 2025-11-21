package player

// Controller defines playback controls for embedding ffplay.
type Controller interface {
	Load(path string, offset float64) error
	SetWindow(x, y, w, h int)
	Play() error
	Pause() error
	Seek(offset float64) error
	SetVolume(level float64) error
	FullScreen() error
	Stop() error
	Close()
}

// New returns a platform-specific implementation when available.
func New() Controller {
	return newController()
}
