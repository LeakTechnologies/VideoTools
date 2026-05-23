//go:build native_media

package media

/*
#cgo !windows pkg-config: libavcodec libavformat libswscale libavutil
#cgo windows CFLAGS: -IC:/ffmpeg/include
#cgo windows LDFLAGS: -LC:/ffmpeg/lib -lavcodec -lavformat -lavutil -lswscale -lswresample -lavfilter -lbcrypt -lSecur32 -lWs2_32 -lmfplat -lstrmiids -lavrt -lole32 -luser32 -Wl,--stack,4194304
#include <libavcodec/avcodec.h>
#include <libavutil/hwcontext.h>
*/
import "C"
import (
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
)

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

func (e *Engine) GetLastError() *PlaybackError {
	return e.lastError
}

func (e *Engine) ClearError() {
	e.lastError = nil
}

// DegradeToSoftware permanently switches from HW to SW decoding.
// Acquires mu → videoCodecMu — must NOT be called while already holding
// videoCodecMu (reverse-order deadlock).  Currently unused (dead code);
// HW→SW fallback happens inline in GrabFrame / videoDecodeLoop instead.
// When wiring up, ensure callers are outside the videoCodecMu critical
// section, or use a goroutine (go e.DegradeToSoftware()).
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
	e.hwDevice = HWDeviceNone
	e.unlockVideoCodecMu()

	e.lastError = &PlaybackError{
		Code:    ErrCodeHWAccel,
		Message: "Fell back to software decoding",
		Retry:   false,
	}
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
