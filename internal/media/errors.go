//go:build native_media

package media

/*
#cgo !windows pkg-config: libavcodec libavformat libswscale libavutil
#cgo windows CFLAGS: -IC:/ffmpeg/include
#cgo windows LDFLAGS: -LC:/ffmpeg/lib -lavcodec -lavformat -lavutil -lswscale -lswresample -lavfilter -lbcrypt -lSecur32 -lWs2_32 -lmfplat -lstrmiids -lavrt -lole32 -luser32 -Wl,--stack,4194304
#include <libavcodec/avcodec.h>
#include <libavutil/hwcontext.h>

// vt_clear_hw_decode resets a codec context to software-only mode after HW
// decoding has been permanently disabled.  Unrefs and NULLs hw_device_ctx
// (which the codec context owns its own AVBufferRef reference to), clears the
// get_format callback and its opaque pointer so FFmpeg will not attempt to
// re-negotiate a HW pixel format on the next flush+decode cycle, and
// re-enables slice-level threading (which was disabled by vt_set_get_format
// for HW compatibility).
static void vt_clear_hw_decode(AVCodecContext *ctx) {
    if (ctx->hw_device_ctx) {
        av_buffer_unref(&ctx->hw_device_ctx);
        ctx->hw_device_ctx = NULL;
    }
    ctx->get_format   = NULL;
    ctx->opaque       = NULL;
    ctx->thread_count = 0;
    ctx->thread_type  = FF_THREAD_SLICE;
}
*/
import "C"
import (
	"time"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
)

const errorRingSize = 16

type ErrorRecord struct {
	Timestamp time.Time
	Code      string
	Message   string
	Retry     bool
}

// PlaybackError is the legacy single-error type preserved for backward
// compatibility with GetLastError() / ShouldRetry().
type PlaybackError struct {
	Code    string
	Message string
	Retry   bool
}

const (
	ErrCodeDecode       = "DECODE_ERROR"
	ErrCodeNetwork      = "NETWORK_ERROR"
	ErrCodeHWAccel      = "HW_ACCEL_ERROR"
	ErrCodeFileCorrupt  = "FILE_CORRUPT"
	ErrCodeCodecMissing = "CODEC_MISSING"
)

func (e *Engine) RecoverableError(code, message string) *PlaybackError {
	return &PlaybackError{
		Code:    code,
		Message: message,
		Retry:   code == ErrCodeNetwork || code == ErrCodeDecode,
	}
}

func (e *Engine) ShouldRetry(err *PlaybackError) bool {
	return err != nil && err.Retry
}

// SetError writes an error record into the ring buffer.
func (e *Engine) SetError(code, message string, retry bool) {
	e.errorMu.Lock()
	idx := e.errorRingNext % errorRingSize
	e.errorRing[idx] = ErrorRecord{
		Timestamp: time.Now(),
		Code:      code,
		Message:   message,
		Retry:     retry,
	}
	e.errorRingNext++
	e.errorMu.Unlock()
}

// GetLastError returns the most recent error as a *PlaybackError, or nil if
// the ring buffer is empty.  Backward-compatible with the pre-ring-buffer
// single-slot API.
func (e *Engine) GetLastError() *PlaybackError {
	e.errorMu.Lock()
	defer e.errorMu.Unlock()

	if e.errorRingNext == 0 {
		return nil
	}
	lastIdx := (e.errorRingNext - 1) % errorRingSize
	rec := e.errorRing[lastIdx]
	return &PlaybackError{
		Code:    rec.Code,
		Message: rec.Message,
		Retry:   rec.Retry,
	}
}

// ClearError clears the error ring buffer.
func (e *Engine) ClearError() {
	e.errorMu.Lock()
	e.errorRingNext = 0
	e.errorRing = [errorRingSize]ErrorRecord{}
	e.errorMu.Unlock()
}

// GetErrorHistory returns all error records in chronological order (oldest
// first).  At most errorRingSize entries are returned.
func (e *Engine) GetErrorHistory() []ErrorRecord {
	e.errorMu.Lock()
	defer e.errorMu.Unlock()

	n := e.errorRingNext
	if n > errorRingSize {
		n = errorRingSize
	}
	out := make([]ErrorRecord, 0, n)
	for i := 0; i < n; i++ {
		idx := i
		if e.errorRingNext > errorRingSize {
			idx = (e.errorRingNext + i) % errorRingSize
		}
		out = append(out, e.errorRing[idx])
	}
	return out
}

// ClearErrorHistory is an alias for ClearError.
func (e *Engine) ClearErrorHistory() {
	e.ClearError()
}

// DegradeToSoftware permanently switches from HW to SW decoding for the
// current session.  Acquires mu → videoCodecMu — must NOT be called while
// already holding videoCodecMu (reverse-order deadlock).  Callers must be
// outside the videoCodecMu critical section.
func (e *Engine) DegradeToSoftware() {
	if e.hwDevice == HWDeviceNone {
		return
	}

	logging.Warning(logging.CatPlayer, "Degrading from HW to software decoding")

	e.lockMu()
	e.hwDegraded = true

	e.lockVideoCodecMu()
	if e.hwFramesCtx != nil {
		C.av_buffer_unref(&e.hwFramesCtx)
		e.hwFramesCtx = nil
	}
	if e.hwDeviceCtx != nil {
		C.av_buffer_unref(&e.hwDeviceCtx)
		e.hwDeviceCtx = nil
	}
	if e.videoCodecCtx.hw_frames_ctx != nil {
		C.av_buffer_unref(&e.videoCodecCtx.hw_frames_ctx)
		e.videoCodecCtx.hw_frames_ctx = nil
	}
	// Reset the codec context's HW device reference and get_format callback so
	// the codec won't attempt HW pixel-format negotiation on the next decode
	// cycle.  Also flush buffered HW frames to prevent use-after-free when the
	// decode loop continues with the SW path.
	if e.videoCodecCtx != nil {
		C.vt_clear_hw_decode(e.videoCodecCtx)
		C.avcodec_flush_buffers(e.videoCodecCtx)
	}
	e.hwDevice = HWDeviceNone
	e.unlockVideoCodecMu()

	e.SetError(ErrCodeHWAccel, "Fell back to software decoding", false)
	e.unlockMu()
}

func (e *Engine) ShouldDegrade() bool {
	e.lockMu()
	defer e.unlockMu()

	if e.hwDegraded {
		return false
	}

	if e.hwFailCount >= 3 {
		return true
	}
	return false
}

func (e *Engine) RecordHWFailure() {
	e.lockMu()
	e.hwFailCount++
	e.unlockMu()
}

func (e *Engine) ResetHWFailureCount() {
	e.lockMu()
	e.hwFailCount = 0
	e.unlockMu()
}
