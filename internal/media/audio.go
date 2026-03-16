//go:build native_media

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
	"io"
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
	queue    *PacketQueue
	clock    *MasterClock
	
	// Output
	otoCtx    *oto.Context
	otoPlayer *oto.Player
	
	// Buffers
	frame      *C.AVFrame
	resampleBuf []byte
	leftover    []byte
	
	// Stream Info
	timeBase float64
}

// NewAudioPlayer creates a new audio player.
func NewAudioPlayer(codecCtx *C.AVCodecContext, queue *PacketQueue, clock *MasterClock, timeBase float64) (*AudioPlayer, error) {
	p := &AudioPlayer{
		codecCtx: codecCtx,
		queue:    queue,
		clock:    clock,
		frame:    C.av_frame_alloc(),
		timeBase: timeBase,
	}

	p.swrCtx = C.swr_alloc()
	C.av_opt_set_chlayout(unsafe.Pointer(p.swrCtx), C.CString("in_chlayout"), &p.codecCtx.ch_layout, 0)
	C.av_opt_set_int(unsafe.Pointer(p.swrCtx), C.CString("in_sample_rate"), C.int64_t(p.codecCtx.sample_rate), 0)
	C.av_opt_set_sample_fmt(unsafe.Pointer(p.swrCtx), C.CString("in_sample_fmt"), p.codecCtx.sample_fmt, 0)
	
	outLayout := C.AVChannelLayout(C.AV_CH_LAYOUT_STEREO)
	C.av_opt_set_chlayout(unsafe.Pointer(p.swrCtx), C.CString("out_chlayout"), &outLayout, 0)
	C.av_opt_set_int(unsafe.Pointer(p.swrCtx), C.CString("out_sample_rate"), TargetSampleRate, 0)
	C.av_opt_set_sample_fmt(unsafe.Pointer(p.swrCtx), C.CString("out_sample_fmt"), C.AV_SAMPLE_FMT_S16, 0)

	if C.swr_init(p.swrCtx) < 0 {
		return nil, fmt.Errorf("failed to initialize resampler")
	}

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
	p.otoPlayer.Play()

	return p, nil
}

// Read implements io.Reader for oto playback.
func (p *AudioPlayer) Read(buf []byte) (int, error) {
	if len(p.leftover) > 0 {
		n := copy(buf, p.leftover)
		p.leftover = p.leftover[n:]
		return n, nil
	}

	for {
		pkt, ok := p.queue.Get()
		if !ok {
			return 0, io.EOF
		}
		defer C.av_packet_free(&pkt)

		if C.avcodec_send_packet(p.codecCtx, pkt) == 0 {
			if C.avcodec_receive_frame(p.codecCtx, p.frame) == 0 {
				// Update Master Clock
				pts := float64(p.frame.pts) * p.timeBase
				p.clock.SetTime(pts)

				data, err := p.resample()
				if err != nil {
					continue
				}
				n := copy(buf, data)
				if n < len(data) {
					p.leftover = data[n:]
				}
				return n, nil
			}
		}
	}
}

func (p *AudioPlayer) resample() ([]byte, error) {
	maxSamples := int(C.swr_get_out_samples(p.swrCtx, p.frame.nb_samples))
	if len(p.resampleBuf) < maxSamples*4 {
		p.resampleBuf = make([]byte, maxSamples*4)
	}

	outPtr := (*C.uint8_t)(unsafe.Pointer(&p.resampleBuf[0]))
	gotSamples := C.swr_convert(p.swrCtx, &outPtr, C.int(maxSamples), &p.frame.data[0], p.frame.nb_samples)
	if gotSamples < 0 {
		return nil, fmt.Errorf("resample error")
	}

	return p.resampleBuf[:gotSamples*4], nil
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