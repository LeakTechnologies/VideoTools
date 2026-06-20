//go:build native_media

package media

/*
#include <libavfilter/avfilter.h>
#include <libavfilter/buffersrc.h>
#include <libavfilter/buffersink.h>
#include <libavutil/frame.h>

// create_bwdif_filter allocates and configures: [in]bwdif=mode=0:parity=-1:deint=0[out]
// On success *graph / *src_ctx / *sink_ctx are set; caller must eventually
// avfilter_graph_free(graph) which frees all sub-contexts.
static int create_bwdif_filter(AVFilterGraph **graph,
                                AVFilterContext **src_ctx,
                                AVFilterContext **sink_ctx,
                                int width, int height, int pix_fmt) {
    int ret;
    char args[256];
    AVFilterGraph *fg = avfilter_graph_alloc();
    if (!fg) return AVERROR(ENOMEM);

    snprintf(args, sizeof(args),
        "video_size=%dx%d:pix_fmt=%d:time_base=1/1:sar=1/1",
        width, height, pix_fmt);

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

    ret = avfilter_graph_parse_ptr(fg, "bwdif=mode=0:parity=-1:deint=0",
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

// run_bwdif pushes frame through the filter graph and returns the deinterlaced
// output in *out.  The caller must av_frame_free(out).  Returns 0 on success.
static int run_bwdif(AVFilterContext *src_ctx,
                      AVFilterContext *sink_ctx,
                      AVFrame *frame,
                      AVFrame **out) {
    int ret = av_buffersrc_add_frame_flags(src_ctx, frame, AV_BUFFERSRC_FLAG_KEEP_REF);
    if (ret < 0) return ret;
    AVFrame *result = av_frame_alloc();
    if (!result) return AVERROR(ENOMEM);
    ret = av_buffersink_get_frame(sink_ctx, result);
    if (ret < 0) { av_frame_free(&result); return ret; }
    *out = result;
    return 0;
}

// frame_is_interlaced checks AV_FRAME_FLAG_INTERLACED on the frame.
// This is the only portable check across FFmpeg 6.x and 7.x+ (the deprecated
// interlaced_frame field was removed in FFmpeg 7.0).
static int frame_is_interlaced(const AVFrame *frame) {
    return (frame->flags & AV_FRAME_FLAG_INTERLACED) != 0;
}
*/
import "C"
import (
	"fmt"

	"github.com/LeakTechnologies/VideoTools/internal/logging"
)

// isFrameInterlaced returns true if the AVFrame has the interlaced flag set.
func isFrameInterlaced(frame *C.AVFrame) bool {
	if frame == nil {
		return false
	}
	return C.frame_is_interlaced(frame) != 0
}

// initDeinterlaceFilter creates or rebuilds the bwdif filter graph for the
// given resolution and pixel format.  Any existing graph is freed first.
func (e *Engine) initDeinterlaceFilter(width, height int, pixFmt C.enum_AVPixelFormat) error {
	e.freeDeinterlaceFilter()

	ret := C.create_bwdif_filter(
		&e.deintFilterGraph,
		&e.deintBuffersrc,
		&e.deintBuffersink,
		C.int(width), C.int(height), C.int(pixFmt),
	)
	if ret < 0 {
		e.deintFilterGraph = nil
		e.deintBuffersrc = nil
		e.deintBuffersink = nil
		return fmt.Errorf("create_bwdif_filter failed: code=%d", int(ret))
	}

	logging.Debug(logging.CatPlayer, "deinterlace: created bwdif filter (%dx%d fmt=%d)", width, height, int(pixFmt))
	return nil
}

// applyDeinterlace pushes e.frame through the bwdif filter and returns the
// deinterlaced frame.  The caller MUST free the returned frame with
// C.av_frame_free.  Returns nil if deinterlacing cannot be performed.
func (e *Engine) applyDeinterlace() *C.AVFrame {
	width := int(e.frame.width)
	height := int(e.frame.height)
	pixFmt := C.enum_AVPixelFormat(e.frame.format)

	if e.deintFilterGraph == nil {
		if err := e.initDeinterlaceFilter(width, height, pixFmt); err != nil {
			logging.Warning(logging.CatPlayer, "deinterlace: init failed: %v", err)
			return nil
		}
	}

	var filtered *C.AVFrame
	ret := C.run_bwdif(e.deintBuffersrc, e.deintBuffersink, e.frame, &filtered)
	if ret < 0 {
		e.freeDeinterlaceFilter()
		logging.Warning(logging.CatPlayer, "deinterlace: run_bwdif failed: code=%d", int(ret))
		return nil
	}

	return filtered
}

// freeDeinterlaceFilter releases the bwdif filter graph and resets pointers.
func (e *Engine) freeDeinterlaceFilter() {
	if e.deintFilterGraph != nil {
		C.avfilter_graph_free(&e.deintFilterGraph)
		e.deintFilterGraph = nil
		e.deintBuffersrc = nil
		e.deintBuffersink = nil
	}
}
