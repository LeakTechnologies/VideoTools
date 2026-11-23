//go:build native_media

package ui

// SetAutomaticPreviewsEnabled changes automatic-load admission immediately.
// Disabling returns a completion signal for every outstanding automatic source,
// including a source replaced by a manual load while native work was in flight.
// The caller must await it off the UI goroutine before claiming resources freed.
// Enabling admits future requests and never implicitly reloads old sources.
func (v *InlineVideoPlayer) SetAutomaticPreviewsEnabled(enabled bool) <-chan struct{} {
	v.mu.Lock()
	v.automaticPreviewsDisabled = !enabled
	var sessions []*playerSession
	if !enabled {
		for s := range v.automaticSessions {
			s.cancel()
			sessions = append(sessions, s)
		}
	}
	if !enabled {
		v.closeSessionLocked(true)
	} else {
		v.mu.Unlock()
	}
	done := make(chan struct{})
	if len(sessions) == 0 {
		close(done)
		return done
	}
	waits := make([]<-chan struct{}, 0, len(sessions))
	for _, s := range sessions {
		waits = append(waits, retireSession(s))
	}
	go func() {
		defer close(done)
		for _, retired := range waits {
			<-retired
		}
	}()
	return done
}
