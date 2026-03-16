package media

/*
#cgo pkg-config: libavcodec libswresample libavutil
#include <libavcodec/avcodec.h>
#include <libswresample/swresample.h>
#include <libavutil/opt.h>
#include <libavutil/channel_layout.h>
*/
import "C"
import (
	"fmt"
	"unsafe"

	"github.com/ebitengine/oto/v3"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

const (
	TargetSampleRate = 48000
	TargetChannels   = 2
)

// AudioPlayer handles decoding and playback of audio streams.
type AudioPlayer struct {
	codecCtx *C.AVCodecContext
	swrCtx   *C.struct_SwrContext
	
	// Output
	otoCtx    *oto.Context
	otoPlayer *oto.Player
	
	// Buffers
	frame      *C.AVFrame
	resampleBuf []byte
}

// NewAudioPlayer creates a new audio player for a given codec context.
func NewAudioPlayer(codecCtx *C.AVCodecContext) (*AudioPlayer, error) {
	p := &AudioPlayer{
		codecCtx: codecCtx,
		frame:    C.av_frame_alloc(),
	}

	// 1. Initialize Resampler
	p.swrCtx = C.swr_alloc()
	
	// Set options for 48kHz Stereo S16 output
	C.av_opt_set_chlayout(unsafe.Pointer(p.swrCtx), C.CString("in_chlayout"), &p.codecCtx.ch_layout, 0)
	C.av_opt_set_int(unsafe.Pointer(p.swrCtx), C.CString("in_sample_rate"), C.int64_t(p.codecCtx.sample_rate), 0)
	C.av_opt_set_sample_fmt(unsafe.Pointer(p.swrCtx), C.CString("in_sample_fmt"), p.codecCtx.sample_fmt, 0)
	
	C.av_opt_set_chlayout(unsafe.Pointer(p.swrCtx), C.CString("out_chlayout"), (*C.AVChannelLayout)(unsafe.Pointer(&C.AV_CH_LAYOUT_STEREO)), 0)
	C.av_opt_set_int(unsafe.Pointer(p.swrCtx), C.CString("out_sample_rate"), TargetSampleRate, 0)
	C.av_opt_set_sample_fmt(unsafe.Pointer(p.swrCtx), C.CString("out_sample_fmt"), C.AV_SAMPLE_FMT_S16, 0)

	if C.swr_init(p.swrCtx) < 0 {
		return nil, fmt.Errorf("failed to initialize resampler")
	}

	// 2. Initialize Oto (Note: In a real app, one Oto context is shared)
	op := &oto.NewContextOptions{
		SampleRate:   TargetSampleRate,
		ChannelCount: TargetChannels,
		Format:       oto.FormatSignedInt16LE,
	}
	otoCtx, ready, err := oto.NewContext(op)
	if err != nil {
		return nil, err
	}
	<-ready
	
	p.otoCtx = otoCtx
	p.otoPlayer = otoCtx.NewPlayer(p)

	logging.Info(logging.CatPlayer, "Audio player initialized at 48kHz Stereo")
	return p, nil
}

// Read satisfies the io.Reader interface for oto.Player.
func (p *AudioPlayer) Read(buf []byte) (int, error) {
	// [Decoding and resampling logic will be triggered here by the playback loop]
	return 0, nil
}

// Close releases audio resources.
func (p *AudioPlayer) Close() {
	if p.swrCtx != nil {
		C.swr_free(&p.swrCtx)
	}
	if p.frame != nil {
		C.av_frame_free(&p.frame)
	}
	if p.otoPlayer != nil {
		p.otoPlayer.Close()
	}
}
