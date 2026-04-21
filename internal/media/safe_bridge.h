/*
 * safe_bridge.h — Pre-flight diagnostic wrapper for FFmpeg codec calls.
 *
 * See safe_bridge.c for full explanation.
 */

#pragma once
#include <stdint.h>
#include <libavcodec/avcodec.h>
#include <libswresample/swresample.h>

#ifdef __cplusplus
extern "C" {
#endif

/* Sentinel placed in *exc_code_out when a pre-flight null check fires. */
#define SAFE_BRIDGE_PREFLIGHT_FAIL 0xDEAD0001u

/* Sentinel for Windows SEH-caught access violations */
#define SAFE_BRIDGE_ACCESS_VIOLATION 0xDEAD0002u

/*
 * CodecDiagnostic — snapshot of key codec/packet state for crash diagnostics.
 * Safe to populate even when priv_data is NULL.
 */
typedef struct CodecDiagnostic {
    int ctx_ok;          /* 1 if ctx != NULL */
    int codec_ok;        /* 1 if ctx->codec != NULL */
    int priv_data_ok;    /* 1 if ctx->priv_data != NULL */
    int codec_id;        /* ctx->codec_id */
    int extradata_size;  /* ctx->extradata_size */
    int extradata_ok;    /* 1 if ctx->extradata != NULL */
    int sample_rate;     /* ctx->sample_rate */
    int channels;        /* ctx->ch_layout.nb_channels */
    int pkt_ok;          /* 1 if pkt != NULL */
    int pkt_size;        /* pkt->size */
    int pkt_data_ok;     /* 1 if pkt->data != NULL */
} CodecDiagnostic;

/*
 * diagnose_avcodec_state — fills *out with key fields from ctx/pkt.
 * Does NOT touch ctx->priv_data or any codec-private structure.
 */
void diagnose_avcodec_state(AVCodecContext* ctx, const AVPacket* pkt,
                             CodecDiagnostic* out);

/*
 * safe_avcodec_send_packet — pre-flight checked avcodec_send_packet.
 * Sets *exc_code_out = SAFE_BRIDGE_PREFLIGHT_FAIL if a null check fires.
 * Sets *exc_code_out = 0 on normal success or AVERROR return from FFmpeg.
 */
int safe_avcodec_send_packet(AVCodecContext* ctx, const AVPacket* pkt,
                              uint32_t* exc_code_out);

/*
 * safe_avcodec_receive_frame — pre-flight checked avcodec_receive_frame.
 * Includes SEH protection on Windows to catch D3D11VA access violations.
 * Sets *exc_code_out = SAFE_BRIDGE_ACCESS_VIOLATION on Windows AV caught.
 */
int safe_avcodec_receive_frame(AVCodecContext* ctx, AVFrame* frame,
                                uint32_t* exc_code_out);

/*
 * audio_swr_convert_packed — CGo-safe wrapper around swr_convert for packed
 * (interleaved) output formats (e.g. AV_SAMPLE_FMT_S16).
 *
 * The standard swr_convert signature takes uint8_t** for the output, which
 * requires passing &outPtr from Go — a Go stack pointer containing a Go heap
 * pointer.  Go 1.21+ CGo rules forbid this ("Go pointer to unpinned Go
 * pointer").  This wrapper takes a flat uint8_t* and forms the double-pointer
 * internally on the C stack, keeping the violation out of Go.
 *
 * out_buf  — caller-allocated output buffer (Go or C memory; must hold at
 *             least out_count * channels * bytes_per_sample bytes).
 * Returns the number of samples converted per channel, or a negative AVERROR.
 */
int audio_swr_convert_packed(SwrContext* swr, uint8_t* out_buf, int out_count,
                              AVFrame* frame);

#ifdef __cplusplus
}
#endif
