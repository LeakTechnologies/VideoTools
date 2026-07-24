//go:build native_media && vlc

package media

/*
#include "vlc_glue.h"
#include <stdlib.h>
#include <string.h>

// Forward declaration for the opaque pointer cast.
struct vlc_engine;
*/
import "C"

import (
	"image"
	"unsafe"
)

// vlcFrameCtx is the context passed through the opaque pointer to C callbacks.
// It holds the pixel buffer and dimensions negotiated with libVLC.
type vlcFrameCtx struct {
	engine  *vlcEngine
	pixels  []byte
	width   int
	height  int
	stride  int // bytes per row (width * 4 for RGBA)
}

//export goVLCLockCB
func goVLCLockCB(opaque, planes unsafe.Pointer) unsafe.Pointer {
	ctx := (*vlcFrameCtx)(opaque)
	if ctx == nil || ctx.engine == nil {
		return nil
	}

	// Ensure buffer is large enough for the current frame.
	needed := ctx.stride * ctx.height
	if needed <= 0 {
		// Not yet negotiated — use a default 1920x1080 buffer.
		ctx.stride = 1920 * 4
		ctx.height = 1080
		needed = ctx.stride * ctx.height
	}
	if cap(ctx.pixels) < needed {
		ctx.pixels = make([]byte, needed)
	}
	ctx.pixels = ctx.pixels[:needed]

	// Set planes[0] to point to our pixel buffer.
	*(*unsafe.Pointer)(planes) = unsafe.Pointer(&ctx.pixels[0])

	return unsafe.Pointer(ctx)
}

//export goVLCUnlockCB
func goVLCUnlockCB(opaque, picture, planes unsafe.Pointer) {
	// Decoding is complete; the frame data is now readable from the buffer.
	// Nothing to do here — display callback handles delivery.
}

//export goVLCDisplayCB
func goVLCDisplayCB(opaque, picture unsafe.Pointer) {
	ctx := (*vlcFrameCtx)(opaque)
	if ctx == nil || ctx.engine == nil {
		return
	}

	// Create an RGBA image from the decoded pixel data.
	img := image.NewRGBA(image.Rect(0, 0, ctx.width, ctx.height))
	copy(img.Pix, ctx.pixels[:ctx.stride*ctx.height])

	// Deliver the frame to the engine's frame channel.
	select {
	case ctx.engine.frameCh <- img:
	default:
		// Channel full — drop frame to avoid blocking the decode thread.
	}

	// Store as lastFrame for GrabFrame.
	ctx.engine.mu.Lock()
	ctx.engine.lastFrame = img
	ctx.engine.mu.Unlock()
}

// vlcFormatSetup is called by libVLC to negotiate the pixel format.
// We request RGBA (RV32) at whatever dimensions the decoder suggests.
//
//export goVLCFormatSetup
func goVLCFormatSetup(opaque unsafe.Pointer, chroma *C.char, width, height *C.uint, pitches, lines *C.uint) C.uint {
	// Request RGBA format — write 4 bytes + null terminator.
	*(*C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(chroma)) + 0)) = 'R'
	*(*C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(chroma)) + 1)) = 'G'
	*(*C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(chroma)) + 2)) = 'B'
	*(*C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(chroma)) + 3)) = 'A'
	*(*C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(chroma)) + 4)) = 0

	w := uint(*width)
	h := uint(*height)
	if w == 0 || h == 0 {
		w = 1920
		h = 1080
	}

	*pitches = C.uint(w * 4) // 4 bytes per pixel
	*lines = C.uint(h)

	// Update the frame context dimensions.
	ctx := (*vlcFrameCtx)(opaque)
	if ctx != nil {
		ctx.width = int(w)
		ctx.height = int(h)
		ctx.stride = int(w * 4)
	}

	return 1 // 1 plane for RGBA
}
