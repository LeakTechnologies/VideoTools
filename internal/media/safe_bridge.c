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
 *  2. SEH (Structured Exception Handling) on Windows to catch access
 *     violations from D3D11VA hardware decode.  When an AV is caught,
 *     we set *exc_code_out to SAFE_BRIDGE_ACCESS_VIOLATION and return
 *     AVERROR(EINVAL) so the Go caller can handle it gracefully.
 *
 *  3. A diagnostic struct (CodecDiagnostic) so Go callers can log the
 *     exact field that caused the failure — useful for root-cause analysis.
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

/* Sentinel for Windows SEH-caught access violations */
#define SAFE_BRIDGE_ACCESS_VIOLATION 0xDEAD0002u

/* Windows SEH includes */
#ifdef _WIN32
#include <windows.h>
#endif

/* -------------------------------------------------------------------------
 * diagnose_avcodec_state
 * Fills *out with key fields from ctx/pkt without touching private decoder
 * state.  Safe to call even if ctx->priv_data is NULL.
 * ---------------------------------------------------------------------- */
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

/* -------------------------------------------------------------------------
 * safe_avcodec_send_packet
 * Runs pre-flight checks, then calls avcodec_send_packet with SEH on Windows.
 * *exc_code_out == 0 on success or normal AVERROR return.
 * *exc_code_out == SAFE_BRIDGE_PREFLIGHT_FAIL if a pre-flight check fires.
 * *exc_code_out == SAFE_BRIDGE_ACCESS_VIOLATION if SEH catches an AV (Windows).
 * ---------------------------------------------------------------------- */
int safe_avcodec_send_packet(AVCodecContext* ctx, const AVPacket* pkt,
                              uint32_t* exc_code_out) {
    *exc_code_out = 0;

    /* Pre-flight: null-check every pointer the codec will dereference. */
    if (!ctx)            { *exc_code_out = SAFE_BRIDGE_PREFLIGHT_FAIL; return AVERROR(EINVAL); }
    if (!ctx->codec)     { *exc_code_out = SAFE_BRIDGE_PREFLIGHT_FAIL; return AVERROR(EINVAL); }
    if (!pkt)            { *exc_code_out = SAFE_BRIDGE_PREFLIGHT_FAIL; return AVERROR(EINVAL); }
    if (!pkt->data)      { *exc_code_out = SAFE_BRIDGE_PREFLIGHT_FAIL; return AVERROR(EINVAL); }
    if (pkt->size <= 0)  { *exc_code_out = SAFE_BRIDGE_PREFLIGHT_FAIL; return AVERROR(EINVAL); }

#if defined(_WIN32) && !defined(__GNUC__)
    /* SEH wrapper to catch access violations from D3D11VA HW decode (MSVC only, not MinGW) */
    __try {
        return avcodec_send_packet(ctx, pkt);
    } __except(EXCEPTION_EXECUTE_HANDLER) {
        *exc_code_out = SAFE_BRIDGE_ACCESS_VIOLATION;
        return AVERROR(EINVAL);
    }
#else
    return avcodec_send_packet(ctx, pkt);
#endif
}

/* -------------------------------------------------------------------------
 * safe_avcodec_receive_frame
 * Pre-flight checks then avcodec_receive_frame with SEH on Windows.
 * *exc_code_out == SAFE_BRIDGE_ACCESS_VIOLATION if SEH catches an AV (Windows).
 * ---------------------------------------------------------------------- */
int safe_avcodec_receive_frame(AVCodecContext* ctx, AVFrame* frame,
                                uint32_t* exc_code_out) {
    *exc_code_out = 0;

    if (!ctx)            { *exc_code_out = SAFE_BRIDGE_PREFLIGHT_FAIL; return AVERROR(EINVAL); }
    if (!frame)          { *exc_code_out = SAFE_BRIDGE_PREFLIGHT_FAIL; return AVERROR(EINVAL); }

#if defined(_WIN32) && !defined(__GNUC__)
    __try {
        return avcodec_receive_frame(ctx, frame);
    } __except(EXCEPTION_EXECUTE_HANDLER) {
        *exc_code_out = SAFE_BRIDGE_ACCESS_VIOLATION;
        return AVERROR(EINVAL);
    }
#else
    return avcodec_receive_frame(ctx, frame);
#endif
}

/* -------------------------------------------------------------------------
 * audio_swr_convert_packed
 * CGo-safe swr_convert for packed (interleaved) output formats.
 *
 * Passing &outPtr to swr_convert from Go violates the CGo rule that forbids
 * a Go pointer to Go memory that itself contains a Go pointer (the heap
 * address of the PCM buffer).  This wrapper constructs the double-pointer on
 * the C stack so Go only needs to pass a flat uint8_t*.
 * ---------------------------------------------------------------------- */
int audio_swr_convert_packed(SwrContext* swr, uint8_t* out_buf, int out_count,
                              AVFrame* frame) {
    if (!swr || !out_buf || !frame) return AVERROR(EINVAL);
    /* C-local pointer — not visible to the Go GC. */
    uint8_t* out_ptr = out_buf;
    return swr_convert(swr, &out_ptr, out_count,
                       (const uint8_t**)frame->data, frame->nb_samples);
}
