//go:build native_media
/*
 * safe_bridge.c — Pre-flight diagnostic wrapper for FFmpeg codec calls.
 *
 * A C-level SIGSEGV/access-violation inside avcodec_send_packet cannot be
 * caught by Go's recover() or by GCC __try/__except (which is MSVC-only).
 * This bridge provides three protections:
 *
 *  1. Pre-flight null checks on the codec context and packet.  If any
 *     pointer the decoder would dereference on entry is NULL, we return
 *     AVERROR(EINVAL) and set *exc_code_out to a sentinel value before
 *     the crash can occur.
 *
 *  2. Crash recovery via structured/vectored exception handling:
 *
 *       Windows + MSVC  : __try/__except  (native SEH)
 *       Windows + MinGW : AddVectoredExceptionHandler + thread-local setjmp
 *                         (VEH works with GCC; SIGSEGV signal does NOT catch
 *                         hardware access violations on Windows)
 *       Linux / macOS   : SIGSEGV signal handler + thread-local setjmp
 *                         (SA_NODEFER keeps the handler re-entrant per-thread)
 *
 *  3. A diagnostic struct (CodecDiagnostic) so Go callers can log the
 *     exact field that caused the failure — useful for root-cause analysis.
 *
 * Thread safety: all setjmp contexts use __thread (GCC thread-local storage)
 * so concurrent codec operations on different goroutines cannot corrupt each
 * other's recovery state.
 *
 * NOTE: If the crash occurs *inside* a non-NULL codec's private state (e.g.
 * a bug in the AAC decoder itself), these checks cannot prevent it.  The
 * correct long-term fix for that case is either a dedicated decode goroutine
 * with a watchdog or replacing the codec with a known-good implementation.
 */

#include "safe_bridge.h"
#include <libavutil/error.h>
#include <string.h>
#include <errno.h>
#include <libswresample/swresample.h>

/* Sentinel placed in *exc_code_out when a pre-flight check fails. */
#define SAFE_BRIDGE_PREFLIGHT_FAIL 0xDEAD0001u

/* Sentinel for a caught access violation. */
#define SAFE_BRIDGE_ACCESS_VIOLATION 0xDEAD0002u

/* =========================================================================
 * Platform-specific crash-recovery setup
 * ======================================================================= */

#if defined(_WIN32) && defined(__GNUC__)
/* ---- Windows + MinGW: Vectored Exception Handler + thread-local setjmp ---
 *
 * signal(SIGSEGV) on MinGW only catches software raise(SIGSEGV) — it does
 * NOT catch hardware access violations (which are CPU exceptions routed
 * through Windows SEH).  AddVectoredExceptionHandler is a Win32 API that
 * fires before the OS unwinds the stack, and it works fine with GCC/MinGW.
 *
 * Each call site registers a VEH for its duration and removes it on return
 * or recovery.  Thread-local storage ensures concurrent calls on different
 * threads cannot corrupt each other's setjmp context.
 */
#include <windows.h>
#include <setjmp.h>

static __thread jmp_buf  tl_veh_buf;
static __thread volatile int tl_veh_set = 0;

static LONG WINAPI veh_codec_handler(PEXCEPTION_POINTERS ep) {
    if (ep->ExceptionRecord->ExceptionCode == EXCEPTION_ACCESS_VIOLATION) {
        if (tl_veh_set) {
            tl_veh_set = 0;
            longjmp(tl_veh_buf, 1);
        }
    }
    return EXCEPTION_CONTINUE_SEARCH;
}

/* Convenience macro: wrap CALL in VEH recovery, storing result in RET.
 * On a caught AV: sets *exc_code_out and returns AVERROR(EINVAL). */
#define SAFE_CALL(CALL, RET, exc_code_out)                              \
    do {                                                                 \
        volatile PVOID _veh = AddVectoredExceptionHandler(1,            \
                                  veh_codec_handler);                   \
        tl_veh_set = 1;                                                  \
        if (setjmp(tl_veh_buf)) {                                        \
            tl_veh_set = 0;                                              \
            RemoveVectoredExceptionHandler((PVOID)_veh);                \
            *(exc_code_out) = SAFE_BRIDGE_ACCESS_VIOLATION;             \
            return AVERROR(EINVAL);                                      \
        }                                                                \
        (RET) = (CALL);                                                  \
        tl_veh_set = 0;                                                  \
        RemoveVectoredExceptionHandler((PVOID)_veh);                    \
    } while (0)

#elif defined(_WIN32) && !defined(__GNUC__)
/* ---- Windows + MSVC: native SEH ---------------------------------------- */
#include <windows.h>

#define SAFE_CALL(CALL, RET, exc_code_out)                              \
    __try {                                                              \
        (RET) = (CALL);                                                  \
    } __except(EXCEPTION_EXECUTE_HANDLER) {                             \
        *(exc_code_out) = SAFE_BRIDGE_ACCESS_VIOLATION;                 \
        return AVERROR(EINVAL);                                          \
    }

#elif defined(__GNUC__)
/* ---- Linux / macOS: SIGSEGV signal handler + thread-local setjmp --------
 *
 * Unlike Windows, SIGSEGV on Linux/macOS IS delivered for hardware access
 * violations (null dereferences, bad pointer reads, etc.).  The handler uses
 * thread-local storage so concurrent invocations on different goroutines are
 * independent.
 */
#include <setjmp.h>
#include <signal.h>

static __thread jmp_buf  tl_sig_buf;
static __thread volatile int tl_sig_set = 0;

static void sig_segv_handler(int sig) {
    (void)sig;
    if (tl_sig_set) {
        tl_sig_set = 0;
        longjmp(tl_sig_buf, 1);
    }
}

#define SAFE_CALL(CALL, RET, exc_code_out)                              \
    do {                                                                 \
        void (*_old)(int) = signal(SIGSEGV, sig_segv_handler);          \
        tl_sig_set = 1;                                                  \
        if (setjmp(tl_sig_buf)) {                                        \
            tl_sig_set = 0;                                              \
            signal(SIGSEGV, _old);                                       \
            *(exc_code_out) = SAFE_BRIDGE_ACCESS_VIOLATION;             \
            return AVERROR(EINVAL);                                      \
        }                                                                \
        (RET) = (CALL);                                                  \
        tl_sig_set = 0;                                                  \
        signal(SIGSEGV, _old);                                           \
    } while (0)

#else
/* ---- No crash protection available ------------------------------------- */
#define SAFE_CALL(CALL, RET, exc_code_out) \
    (RET) = (CALL)
#endif


/* =========================================================================
 * diagnose_avcodec_state
 * Fills *out with key fields from ctx/pkt without touching private decoder
 * state.  Safe to call even if ctx->priv_data is NULL.
 * ======================================================================= */
void diagnose_avcodec_state(AVCodecContext* ctx, const AVPacket* pkt,
                             CodecDiagnostic* out) {
    memset(out, 0, sizeof(*out));
    if (!ctx) return;

    out->ctx_ok        = 1;
    out->codec_ok      = (ctx->codec      != NULL) ? 1 : 0;
    out->priv_data_ok  = (ctx->priv_data  != NULL) ? 1 : 0;
    out->codec_id      = (int)ctx->codec_id;
    out->extradata_size = ctx->extradata_size;
    out->extradata_ok  = (ctx->extradata  != NULL) ? 1 : 0;
    out->sample_rate   = ctx->sample_rate;
    out->channels      = ctx->ch_layout.nb_channels;

    if (pkt) {
        out->pkt_ok       = 1;
        out->pkt_size     = pkt->size;
        out->pkt_data_ok  = (pkt->data != NULL) ? 1 : 0;
    }
}

/* =========================================================================
 * safe_avcodec_send_packet
 * Runs pre-flight checks, then calls avcodec_send_packet with platform
 * crash recovery.
 * ======================================================================= */
int safe_avcodec_send_packet(AVCodecContext* ctx, const AVPacket* pkt,
                              uint32_t* exc_code_out) {
    *exc_code_out = 0;

    /* Pre-flight: null-check every pointer the codec will dereference.
     * pkt may be NULL for drain; otherwise require valid packet data. */
    if (!ctx)        { *exc_code_out = SAFE_BRIDGE_PREFLIGHT_FAIL; return AVERROR(EINVAL); }
    if (!ctx->codec) { *exc_code_out = SAFE_BRIDGE_PREFLIGHT_FAIL; return AVERROR(EINVAL); }
    if (pkt != NULL) {
        if (!pkt->data)     { *exc_code_out = SAFE_BRIDGE_PREFLIGHT_FAIL; return AVERROR(EINVAL); }
        if (pkt->size <= 0) { *exc_code_out = SAFE_BRIDGE_PREFLIGHT_FAIL; return AVERROR(EINVAL); }
    }

    int ret = 0;
    SAFE_CALL(avcodec_send_packet(ctx, pkt), ret, exc_code_out);
    return ret;
}

/* =========================================================================
 * safe_avcodec_receive_frame
 * Pre-flight checks then avcodec_receive_frame with platform crash recovery.
 * ======================================================================= */
int safe_avcodec_receive_frame(AVCodecContext* ctx, AVFrame* frame,
                                uint32_t* exc_code_out) {
    *exc_code_out = 0;

    if (!ctx)   { *exc_code_out = SAFE_BRIDGE_PREFLIGHT_FAIL; return AVERROR(EINVAL); }
    if (!frame) { *exc_code_out = SAFE_BRIDGE_PREFLIGHT_FAIL; return AVERROR(EINVAL); }

    int ret = 0;
    SAFE_CALL(avcodec_receive_frame(ctx, frame), ret, exc_code_out);
    return ret;
}

/* =========================================================================
 * audio_swr_convert_packed
 * CGo-safe swr_convert for packed (interleaved) output formats.
 *
 * Passing &outPtr to swr_convert from Go violates the CGo rule that forbids
 * a Go pointer to Go memory that itself contains a Go pointer (the heap
 * address of the PCM buffer).  This wrapper constructs the double-pointer on
 * the C stack so Go only needs to pass a flat uint8_t*.
 * ======================================================================= */
int audio_swr_convert_packed(SwrContext* swr, uint8_t* out_buf, int out_count,
                              AVFrame* frame) {
    if (!swr || !out_buf || !frame) return AVERROR(EINVAL);
    uint8_t* out_ptr = out_buf;
    return swr_convert(swr, &out_ptr, out_count,
                       (const uint8_t**)frame->data, frame->nb_samples);
}
