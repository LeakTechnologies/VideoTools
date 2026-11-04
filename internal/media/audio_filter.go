//go:build native_media

package media

/*
#include <stdlib.h>
#include <string.h>
#include <libavfilter/avfilter.h>
#include <libavfilter/buffersink.h>
#include <libavfilter/buffersrc.h>
#include <libavutil/avutil.h>
#include <libavutil/channel_layout.h>
#include <libavutil/frame.h>
#include <libavutil/opt.h>

// vt_atempo_process pushes nb_samples of S16 stereo input through the atempo
// filter graph and returns a malloc'd buffer containing all output frames.
// Returns NULL if the filter has not yet produced output (still buffering).
// Caller must free() the returned pointer.
static uint8_t* vt_atempo_process(
    AVFilterContext *src, AVFilterContext *sink,
    const uint8_t *input, int nb_samples, int sample_rate,
    int *out_len)
{
    *out_len = 0;
    if (!src || !sink || !input || nb_samples <= 0) return NULL;

    AVFrame *in_frame = av_frame_alloc();
    if (!in_frame) return NULL;

    in_frame->format      = AV_SAMPLE_FMT_S16;
    in_frame->sample_rate = sample_rate;
    in_frame->nb_samples  = nb_samples;
    av_channel_layout_default(&in_frame->ch_layout, 2);

    if (av_frame_get_buffer(in_frame, 0) < 0) {
        av_frame_free(&in_frame);
        return NULL;
    }
    memcpy(in_frame->data[0], input, (size_t)nb_samples * 4);

    int ret = av_buffersrc_add_frame_flags(src, in_frame, AV_BUFFERSRC_FLAG_KEEP_REF);
    av_frame_unref(in_frame);
    av_frame_free(&in_frame);
    if (ret < 0) return NULL;

    uint8_t *out_buf  = NULL;
    size_t   out_size = 0;
    AVFrame *out_frame = av_frame_alloc();
    if (!out_frame) return NULL;

    while ((ret = av_buffersink_get_frame(sink, out_frame)) >= 0) {
        int    frame_bytes = out_frame->nb_samples * 4;
        uint8_t *tmp = (uint8_t*)realloc(out_buf, out_size + (size_t)frame_bytes);
        if (!tmp) { free(out_buf); av_frame_free(&out_frame); return NULL; }
        out_buf = tmp;
        memcpy(out_buf + out_size, out_frame->data[0], (size_t)frame_bytes);
        out_size += (size_t)frame_bytes;
        av_frame_unref(out_frame);
    }
    av_frame_free(&out_frame);

    *out_len = (int)out_size;
    return out_buf;
}
*/
import "C"
import (
	"fmt"
	"math"
	"unsafe"
)

type AudioFilterGraph struct {
	graph         *C.AVFilterGraph
	buffersrcCtx  *C.AVFilterContext
	buffersinkCtx *C.AVFilterContext
	atempoCtx     *C.AVFilterContext
	atempoTailCtx *C.AVFilterContext
	initialized   bool
	tempo         float64
	sampleRate    int
}

func NewAudioFilterGraph() *AudioFilterGraph {
	return &AudioFilterGraph{
		graph: nil,
		tempo: 1.0,
	}
}

// createAudioFilter keeps every temporary C string scoped to its native call.
func createAudioFilter(graph *C.AVFilterGraph, name, instance, args string) (*C.AVFilterContext, error) {
	cname, cinstance := C.CString(name), C.CString(instance)
	defer C.free(unsafe.Pointer(cname))
	defer C.free(unsafe.Pointer(cinstance))
	var cargs *C.char
	if args != "" {
		cargs = C.CString(args)
		defer C.free(unsafe.Pointer(cargs))
	}
	filter := C.avfilter_get_by_name(cname)
	if filter == nil {
		return nil, fmt.Errorf("audio filter %s unavailable", name)
	}
	var ctx *C.AVFilterContext
	if ret := C.avfilter_graph_create_filter(&ctx, filter, cinstance, cargs, nil, graph); ret < 0 {
		return nil, fmt.Errorf("create %s: %d", name, ret)
	}
	return ctx, nil
}

func (g *AudioFilterGraph) Init(sampleRate int, channels int) (err error) {
	g.Release()
	g.graph = C.avfilter_graph_alloc()
	if g.graph == nil {
		return fmt.Errorf("failed to allocate audio filter graph")
	}
	defer func() {
		if err != nil {
			g.Release()
		}
	}()
	args := fmt.Sprintf("time_base=1/%d:sample_rate=%d:sample_fmt=s16:channel_layout=0x%x", sampleRate, sampleRate, getChannelLayout(channels))
	if g.buffersrcCtx, err = createAudioFilter(g.graph, "abuffer", "audio_input", args); err != nil {
		return err
	}
	if g.buffersinkCtx, err = createAudioFilter(g.graph, "abuffersink", "audio_output", ""); err != nil {
		return err
	}
	// Two legal stages preserve the advertised 0.25x rate: each is >=0.5.
	factor := math.Sqrt(g.tempo)
	tempoArgs := fmt.Sprintf("tempo=%.12f", factor)
	if g.atempoCtx, err = createAudioFilter(g.graph, "atempo", "tempo_first", tempoArgs); err != nil {
		return err
	}
	if g.atempoTailCtx, err = createAudioFilter(g.graph, "atempo", "tempo_second", tempoArgs); err != nil {
		return err
	}
	chain := []*C.AVFilterContext{g.buffersrcCtx, g.atempoCtx, g.atempoTailCtx, g.buffersinkCtx}
	for i := 0; i < len(chain)-1; i++ {
		if ret := C.avfilter_link(chain[i], 0, chain[i+1], 0); ret < 0 {
			return fmt.Errorf("link audio filter: %d", ret)
		}
	}
	if ret := C.avfilter_graph_config(g.graph, nil); ret < 0 {
		return fmt.Errorf("configure audio filter graph: %d", ret)
	}
	g.sampleRate = sampleRate
	g.initialized = true
	return nil
}

func setAudioTempo(ctx *C.AVFilterContext, tempo float64) error {
	cmd, arg := C.CString("tempo"), C.CString(fmt.Sprintf("%.12f", tempo))
	defer C.free(unsafe.Pointer(cmd))
	defer C.free(unsafe.Pointer(arg))
	// The filter command updates its dependent windows/state. Writing priv AVOptions
	// directly bypasses that update and can assert in af_atempo while streaming.
	if ret := C.avfilter_process_command(ctx, cmd, arg, nil, 0, 0); ret < 0 {
		return fmt.Errorf("audio tempo command: %d", ret)
	}
	return nil
}

func (g *AudioFilterGraph) SetTempo(tempo float64) error {
	if math.IsNaN(tempo) || math.IsInf(tempo, 0) || tempo < 0.25 || tempo > 4 {
		return fmt.Errorf("audio tempo out of range: %v", tempo)
	}
	if g.initialized {
		factor := math.Sqrt(tempo)
		if err := setAudioTempo(g.atempoCtx, factor); err != nil {
			return err
		}
		if err := setAudioTempo(g.atempoTailCtx, factor); err != nil {
			_ = setAudioTempo(g.atempoCtx, math.Sqrt(g.tempo))
			return err
		}
	}
	g.tempo = tempo
	return nil
}

func (g *AudioFilterGraph) GetTempo() float64 {
	return g.tempo
}

func (g *AudioFilterGraph) Process(input []byte) ([]byte, error) {
	if !g.initialized || g.graph == nil {
		return input, nil
	}
	if len(input) == 0 {
		return nil, nil
	}

	nbSamples := C.int(len(input) / 4) // S16 stereo = 4 bytes per frame
	var outLen C.int
	outPtr := C.vt_atempo_process(
		g.buffersrcCtx, g.buffersinkCtx,
		(*C.uint8_t)(unsafe.Pointer(&input[0])),
		nbSamples, C.int(g.sampleRate), &outLen,
	)

	if outPtr == nil || outLen == 0 {
		// Filter is still buffering; caller should return silence.
		return nil, nil
	}
	out := C.GoBytes(unsafe.Pointer(outPtr), outLen)
	C.free(unsafe.Pointer(outPtr))
	return out, nil
}

func (g *AudioFilterGraph) Release() {
	if g.graph != nil {
		C.avfilter_graph_free(&g.graph)
		g.graph = nil
	}
	g.buffersrcCtx = nil
	g.buffersinkCtx = nil
	g.atempoCtx = nil
	g.atempoTailCtx = nil
	g.initialized = false
}

func getChannelLayout(channels int) uint64 {
	switch channels {
	case 1:
		return 0x1
	case 2:
		return 0x3
	case 6:
		return 0x3F
	case 8:
		return 0xAFF
	default:
		return 0x3
	}
}

type TempoController struct {
	graph    *AudioFilterGraph
	minTempo float64
	maxTempo float64
	current  float64
}

func NewTempoController() *TempoController {
	return &TempoController{
		graph:    NewAudioFilterGraph(),
		minTempo: 0.25,
		maxTempo: 2.0,
		current:  1.0,
	}
}

func (tc *TempoController) Init(sampleRate, channels int) error {
	if err := tc.graph.Init(sampleRate, channels); err != nil {
		return err
	}
	return tc.graph.SetTempo(tc.current)
}

func (tc *TempoController) SetTempo(tempo float64) error {
	if tempo < tc.minTempo {
		tempo = tc.minTempo
	}
	if tempo > tc.maxTempo {
		tempo = tc.maxTempo
	}
	tc.current = tempo
	return tc.graph.SetTempo(tempo)
}

func (tc *TempoController) GetTempo() float64 {
	return tc.current
}

func (tc *TempoController) IncreaseTempo() error {
	newTempo := tc.current * 1.25
	if newTempo > tc.maxTempo {
		newTempo = tc.maxTempo
	}
	return tc.SetTempo(newTempo)
}

func (tc *TempoController) DecreaseTempo() error {
	newTempo := tc.current / 1.25
	if newTempo < tc.minTempo {
		newTempo = tc.minTempo
	}
	return tc.SetTempo(newTempo)
}

func (tc *TempoController) Release() {
	if tc.graph != nil {
		tc.graph.Release()
	}
}
