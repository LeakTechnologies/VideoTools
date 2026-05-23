//go:build native_media

package media

// Lock hierarchy (all Engine-level mutexes, ordered by acquisition level):
//
//   Level 1: mu           — general engine state (running, paused, loading,
//                            deinterlaceEnabled, filterPipeline, bufferMode,
//                            decodeTimes, chapters, looping, hwDegraded, ...)
//   Level 2: formatMu     — serialises av_read_frame (demuxerLoop) vs
//                            avformat_seek_file (Seek) — AVFormatContext is
//                            NOT thread-safe; concurrent access crashes
//   Level 3: videoCodecMu — serialises avcodec_send_packet /
//                            avcodec_receive_frame on videoCodecCtx
//   Level 4: framepoolMu  — framePool byte-slice reuse pool (toRGBA /
//                            ReleaseFrame / GetFramePoolSize)
//
//   INDEPENDENT (not in hierarchy):
//     PlaybackFrameCache.mu — standalone RWMutex on a separate struct
//     seekFlushBefore / seekGen / lastVideoPTSBits — atomic.Uint64
//
// Rules:
//   1. Always acquire locks in ascending level order.
//      Correct:   mu → formatMu → videoCodecMu → framepoolMu
//      VIOLATION: videoCodecMu → mu  (reverse order → deadlock)
//   2. Release in descending order (not strictly required, but clearest).
//   3. A goroutine may acquire any single lock from the unlocked state.
//   4. Functions that need multiple locks must acquire them in level order
//      and must not drop a lower-level lock while holding a higher-level one
//      (doing so creates a window where another goroutine can observe the
//      lower lock released and acquire it in reverse order).
//   5. Special case — Close(): releases mu before taking videoCodecMu.
//      This is safe because running is flipped to false under mu before
//      mu is released, and all other operations check running under mu.
//      The gap between mu release and videoCodecMu acquisition is covered
//      by stop channel close + decodeLoopWg.Wait + demuxerWg.Wait.
//
// Lockdep: compile with -tags lockdep to enable goroutine-local ordering
// verification at every lock acquisition.  Catches reverse-order violations
// that would otherwise deadlock at runtime.

const (
	lockLevelGeneral   = 1 // mu
	lockLevelFormat    = 2 // formatMu
	lockLevelCodec     = 3 // videoCodecMu
	lockLevelFramepool = 4 // framepoolMu
)

// --- mu (level 1) helpers ---

func (e *Engine) lockMu() {
	e.mu.Lock()
	e.acquired(lockLevelGeneral)
}

func (e *Engine) unlockMu() {
	e.released(lockLevelGeneral)
	e.mu.Unlock()
}

// --- formatMu (level 2) helpers ---

func (e *Engine) lockFormatMu() {
	e.formatMu.Lock()
	e.acquired(lockLevelFormat)
}

func (e *Engine) unlockFormatMu() {
	e.released(lockLevelFormat)
	e.formatMu.Unlock()
}

// --- videoCodecMu (level 3) helpers ---

func (e *Engine) lockVideoCodecMu() {
	e.videoCodecMu.Lock()
	e.acquired(lockLevelCodec)
}

func (e *Engine) unlockVideoCodecMu() {
	e.released(lockLevelCodec)
	e.videoCodecMu.Unlock()
}

// --- framepoolMu (level 4) helpers ---

func (e *Engine) lockFramepoolMu() {
	e.framepoolMu.Lock()
	e.acquired(lockLevelFramepool)
}

func (e *Engine) unlockFramepoolMu() {
	e.released(lockLevelFramepool)
	e.framepoolMu.Unlock()
}

// guard wraps defer unlock for named return cleanup.  Usage:
//
//	defer e.guardMu()(&err)
//
// The deferred func captures the error pointer so cleanup that happens
// before the return can set err and the named return is still visible
// to the caller.  Currently unused; provided for future use in seek-like
// functions that need mu + formatMu + videoCodecMu with error returns.
func (e *Engine) guardMu() func(*error) {
	return func(errp *error) {
		e.unlockMu()
	}
}

// acquired and released are defined in lockdep_{on,off}.go.
// When the lockdep build tag is set, they verify lock ordering;
// otherwise they are no-ops.
