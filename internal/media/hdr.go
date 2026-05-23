//go:build native_media

package media

/*
#cgo !windows pkg-config: libavcodec libavformat libswscale libavutil libavfilter
#cgo windows CFLAGS: -IC:/ffmpeg/include
#cgo windows LDFLAGS: -LC:/ffmpeg/lib -lavcodec -lavformat -lavutil -lswscale -lswresample -lavfilter -lbcrypt -lSecur32 -lWs2_32 -lmfplat -lstrmiids -lavrt -lole32 -luser32 -Wl,--stack,4194304

#include <libavfilter/avfilter.h>
#include <libavfilter/buffersrc.h>
#include <libavfilter/buffersink.h>
#include <libavutil/frame.h>
#include <libavutil/pixfmt.h>

// frame_is_hdr returns non-zero if the frame carries a transfer characteristic
// that indicates HDR content: PQ (SMPTE 2084) or HLG (ARIB STD B67).
static int frame_is_hdr(const AVFrame *f) {
    if (!f) return 0;
    return f->color_trc == AVCOL_TRC_SMPTE2084 ||
           f->color_trc == AVCOL_TRC_ARIB_STD_B67;
}

// create_hdr_tonemap_filter builds a filter graph:
//
//   buffer → zscale(linear) → format(gbrpf32le) → tonemap(hable)
//          → zscale(bt709)  → format(yuv420p)   → buffersink
//
// Color metadata (TRC, primaries, matrix, range) is passed in the buffersrc
// args so zscale can correctly identify the input transfer characteristic.
// Returns 0 on success; caller must call avfilter_graph_free(*graph) when done.
static int create_hdr_tonemap_filter(AVFilterGraph **graph,
                                     AVFilterContext **src_ctx,
                                     AVFilterContext **sink_ctx,
                                     int width, int height, int pix_fmt,
                                     int color_range, int color_trc,
                                     int colorspace, int color_primaries)
{
    int ret;
    char args[512];
    AVFilterGraph *fg = avfilter_graph_alloc();
    if (!fg) return AVERROR(ENOMEM);

    snprintf(args, sizeof(args),
        "video_size=%dx%d:pix_fmt=%d:time_base=1/1:sar=1/1"
        ":color_range=%d:color_trc=%d:colorspace=%d:color_primaries=%d",
        width, height, pix_fmt,
        color_range, color_trc, colorspace, color_primaries);

    const AVFilter *buffersrc = avfilter_get_by_name("buffer");
    if (!buffersrc) { avfilter_graph_free(&fg); return AVERROR_FILTER_NOT_FOUND; }
    AVFilterContext *src = NULL;
    ret = avfilter_graph_create_filter(&src, buffersrc, "in", args, NULL, fg);
    if (ret < 0) { avfilter_graph_free(&fg); return ret; }

    const AVFilter *buffersink = avfilter_get_by_name("buffersink");
    if (!buffersink) { avfilter_graph_free(&fg); return AVERROR_FILTER_NOT_FOUND; }
    AVFilterContext *sink = NULL;
    ret = avfilter_graph_create_filter(&sink, buffersink, "out", NULL, NULL, fg);
    if (ret < 0) { avfilter_graph_free(&fg); return ret; }

    AVFilterInOut *outputs = avfilter_inout_alloc();
    AVFilterInOut *inputs  = avfilter_inout_alloc();
    if (!outputs || !inputs) {
        avfilter_inout_free(&outputs);
        avfilter_inout_free(&inputs);
        avfilter_graph_free(&fg);
        return AVERROR(ENOMEM);
    }
    outputs->name       = av_strdup("in");
    outputs->filter_ctx = src;
    outputs->pad_idx    = 0;
    outputs->next       = NULL;
    inputs->name        = av_strdup("out");
    inputs->filter_ctx  = sink;
    inputs->pad_idx     = 0;
    inputs->next        = NULL;

    // zscale requires libzimg.  If unavailable the graph creation will fail
    // here; the Go caller handles that gracefully and falls back to no-op.
    //
    // Pipeline: convert input TRC to linear light (npl=1000 covers both
    // HDR10 and HLG mastering targets) → float GBR for tonemap → Hable
    // tone-mapping → BT.709 SDR → integer YUV for sws_scale.
    ret = avfilter_graph_parse_ptr(fg,
        "zscale=transfer=linear:primaries=bt709:matrix=bt709:npl=1000,"
        "format=gbrpf32le,"
        "tonemap=tonemap=hable:desat=0.5:peak=0,"
        "zscale=transfer=bt709:primaries=bt709:matrix=bt709:range=tv,"
        "format=yuv420p",
        &inputs, &outputs, NULL);
    avfilter_inout_free(&inputs);
    avfilter_inout_free(&outputs);
    if (ret < 0) { avfilter_graph_free(&fg); return ret; }

    ret = avfilter_graph_config(fg, NULL);
    if (ret < 0) { avfilter_graph_free(&fg); return ret; }

    *graph    = fg;
    *src_ctx  = src;
    *sink_ctx = sink;
    return 0;
}

// run_hdr_tonemap pushes frame through the HDR tone-mapping graph.
// On success *out is set; caller must av_frame_free(*out).  Returns 0 on success.
static int run_hdr_tonemap(AVFilterContext *src_ctx,
                           AVFilterContext *sink_ctx,
                           AVFrame *frame,
                           AVFrame **out)
{
    int ret = av_buffersrc_add_frame_flags(src_ctx, frame, AV_BUFFERSRC_FLAG_KEEP_REF);
    if (ret < 0) return ret;
    AVFrame *result = av_frame_alloc();
    if (!result) return AVERROR(ENOMEM);
    ret = av_buffersink_get_frame(sink_ctx, result);
    if (ret < 0) { av_frame_free(&result); return ret; }
    *out = result;
    return 0;
}
*/
import "C"
import (
	"fmt"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
)

// isFrameHDR returns true if the AVFrame has a HDR transfer characteristic
// (PQ / SMPTE 2084 or HLG / ARIB STD B67).
func isFrameHDR(frame *C.AVFrame) bool {
	if frame == nil {
		return false
	}
	return C.frame_is_hdr(frame) != 0
}

// initHDRFilter creates or rebuilds the tone-mapping filter graph for the
// given frame properties.  Any existing graph is freed first.
// Returns an error if the required filters (zscale, tonemap) are unavailable.
func (e *Engine) initHDRFilter(frame *C.AVFrame) error {
	e.freeHDRFilter()

	ret := C.create_hdr_tonemap_filter(
		&e.hdrFilterGraph,
		&e.hdrBuffersrc,
		&e.hdrBuffersink,
		frame.width, frame.height,
		C.int(frame.format),
		C.int(frame.color_range),
		C.int(frame.color_trc),
		C.int(frame.colorspace),
		C.int(frame.color_primaries),
	)
	if ret < 0 {
		e.hdrFilterGraph = nil
		e.hdrBuffersrc = nil
		e.hdrBuffersink = nil
		return fmt.Errorf("create_hdr_tonemap_filter failed: code=%d", int(ret))
	}

	logging.Info(logging.CatPlayer, "hdr: created tonemap filter (%dx%d fmt=%d trc=%d)",
		int(frame.width), int(frame.height), int(frame.format), int(frame.color_trc))
	return nil
}

// applyHDRTonemap pushes frame through the tone-mapping graph and returns the
// SDR output frame (yuv420p).  The caller MUST free the returned frame with
// C.av_frame_free.  Returns nil if tone-mapping fails.
//
// If the filter graph has not been created yet (or the input geometry changed),
// initHDRFilter is called first.
func (e *Engine) applyHDRTonemap(frame *C.AVFrame) *C.AVFrame {
	if e.hdrFilterGraph == nil ||
		e.hdrInputPixFmt != C.enum_AVPixelFormat(frame.format) {
		if err := e.initHDRFilter(frame); err != nil {
			logging.Warning(logging.CatPlayer, "hdr: init failed (zscale/tonemap may be unavailable): %v", err)
			e.hdrTonemapUnsupported = true
			return nil
		}
		e.hdrInputPixFmt = C.enum_AVPixelFormat(frame.format)
	}

	var out *C.AVFrame
	ret := C.run_hdr_tonemap(e.hdrBuffersrc, e.hdrBuffersink, frame, &out)
	if ret < 0 {
		e.freeHDRFilter()
		logging.Warning(logging.CatPlayer, "hdr: run_hdr_tonemap failed: code=%d", int(ret))
		return nil
	}
	return out
}

// freeHDRFilter releases the HDR tone-mapping filter graph and resets pointers.
func (e *Engine) freeHDRFilter() {
	if e.hdrFilterGraph != nil {
		C.avfilter_graph_free(&e.hdrFilterGraph)
		e.hdrFilterGraph = nil
		e.hdrBuffersrc = nil
		e.hdrBuffersink = nil
	}
}
