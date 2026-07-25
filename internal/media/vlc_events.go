//go:build native_media && vlc

package media

/*
#include "vlc_glue.h"
#include <stdlib.h>
*/
import "C"

import (
	"unsafe"

	"github.com/LeakTechnologies/VideoTools/internal/logging"
)

//export goVLCEventEOF
func goVLCEventEOF(opaque unsafe.Pointer) {
	ctx := (*vlcFrameCtx)(opaque)
	if ctx == nil || ctx.engine == nil {
		return
	}
	logging.Info(logging.CatPlayer, "VLC: EOF event")
	ctx.engine.vlcOnEOF()
}

//export goVLCEventPositionChanged
func goVLCEventPositionChanged(opaque unsafe.Pointer, position C.float) {
	ctx := (*vlcFrameCtx)(opaque)
	if ctx == nil || ctx.engine == nil {
		return
	}
	eng := ctx.engine
	eng.mu.Lock()
	cb := eng.onProgress
	eng.mu.Unlock()
	if cb != nil {
		// position is 0.0–1.0; convert to seconds.
		dur := eng.Duration()
		if dur > 0 {
			cb(float64(position) * dur)
		}
	}
}

//export goVLCEventError
func goVLCEventError(opaque unsafe.Pointer) {
	ctx := (*vlcFrameCtx)(opaque)
	if ctx == nil || ctx.engine == nil {
		return
	}
	logging.Error(logging.CatPlayer, "VLC: playback error event")
	ctx.engine.vlcOnEOF() // treat as end-of-stream for UI reset
}
