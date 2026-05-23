//go:build native_media && !lockdep

package media

func (e *Engine) acquired(level int) {}

func (e *Engine) released(level int) {}
