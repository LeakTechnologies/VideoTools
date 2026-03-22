//go:build native_media

package media

/*
#cgo !windows pkg-config: libavcodec libswresample libavutil
#cgo windows CFLAGS: -IC:/ffmpeg/include
#cgo windows LDFLAGS: -LC:/ffmpeg/lib -lavcodec -lavutil -lswresample
#include <libavcodec/avcodec.h>
#include <libswresample/swresample.h>
#include <libavutil/opt.h>
#include <libavutil/channel_layout.h>
*/
import "C"
import (
	"fmt"
	"io"
	"sync"
	"time"
	"unsafe"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"github.com/ebitengine/oto/v3"
)

const (
	TargetSampleRate = 48000
	TargetChannels   = 2
)

type AudioPlayer struct {
	codecCtx *C.AVCodecContext
	swrCtx   *C.struct_SwrContext
	queue    *PacketQueue
	clock    *MasterClock

	otoCtx    *oto.Context
	otoPlayer *oto.Player

	frame       *C.AVFrame
	resampleBuf []byte
	leftover    []byte

	timeBase float64

	volume float32
	muted  bool
	paused bool

	mu          sync.Mutex
	volumeMul   float32
	eofReceived bool
	looping     bool
}

func NewAudioPlayer(codecCtx *C.AVCodecContext, queue *PacketQueue, clock *MasterClock, timeBase float64) (*AudioPlayer, error) {
	p := &AudioPlayer{
		codecCtx:  codecCtx,
		queue:     queue,
		clock:     clock,
		frame:     C.av_frame_alloc(),
		timeBase:  timeBase,
		volume:    1.0,
		volumeMul: 1.0,
	}

	if p.frame == nil {
		return nil, fmt.Errorf("failed to allocate audio frame")
	}

	p.swrCtx = C.swr_alloc()
	if p.swrCtx == nil {
		C.av_frame_free(&p.frame)
		return nil, fmt.Errorf("failed to allocate resampler")
	}

	C.av_opt_set_chlayout(unsafe.Pointer(p.swrCtx), C.CString("in_chlayout"), &p.codecCtx.ch_layout, 0)
	C.av_opt_set_int(unsafe.Pointer(p.swrCtx), C.CString("in_sample_rate"), C.int64_t(p.codecCtx.sample_rate), 0)
	C.av_opt_set_sample_fmt(unsafe.Pointer(p.swrCtx), C.CString("in_sample_fmt"), p.codecCtx.sample_fmt, 0)

	outLayout := C.AVChannelLayout(C.AV_CH_LAYOUT_STEREO)
	C.av_opt_set_chlayout(unsafe.Pointer(p.swrCtx), C.CString("out_chlayout"), &outLayout, 0)
	C.av_opt_set_int(unsafe.Pointer(p.swrCtx), C.CString("out_sample_rate"), TargetSampleRate, 0)
	C.av_opt_set_sample_fmt(unsafe.Pointer(p.swrCtx), C.CString("out_sample_fmt"), C.AV_SAMPLE_FMT_S16, 0)

	if C.swr_init(p.swrCtx) < 0 {
		C.swr_free(&p.swrCtx)
		C.av_frame_free(&p.frame)
		return nil, fmt.Errorf("failed to initialize resampler")
	}

	op := &oto.NewContextOptions{
		SampleRate:   TargetSampleRate,
		ChannelCount: TargetChannels,
		Format:       oto.FormatSignedInt16LE,
	}
	otoCtx, ready, err := oto.NewContext(op)
	if err != nil {
		C.swr_free(&p.swrCtx)
		C.av_frame_free(&p.frame)
		return nil, fmt.Errorf("failed to create oto context: %w", err)
	}
	<-ready

	p.otoCtx = otoCtx
	p.otoPlayer = otoCtx.NewPlayer(p)
	p.otoPlayer.Play()

	return p, nil
}

func (p *AudioPlayer) SetVolume(vol float32) {
	p.mu.Lock()
	p.volume = vol
	p.updateVolumeMul()
	p.mu.Unlock()
}

func (p *AudioPlayer) GetVolume() float32 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.volume
}

func (p *AudioPlayer) SetMuted(muted bool) {
	p.mu.Lock()
	p.muted = muted
	p.updateVolumeMul()
	p.mu.Unlock()
}

func (p *AudioPlayer) IsMuted() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.muted
}

func (p *AudioPlayer) updateVolumeMul() {
	if p.muted {
		p.volumeMul = 0
	} else {
		p.volumeMul = p.volume
	}
}

func (p *AudioPlayer) Pause() {
	p.mu.Lock()
	p.paused = true
	p.mu.Unlock()
}

func (p *AudioPlayer) Resume() {
	p.mu.Lock()
	p.paused = false
	p.mu.Unlock()
}

func (p *AudioPlayer) IsPaused() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.paused
}

func (p *AudioPlayer) SetLooping(looping bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.looping = looping
}

func (p *AudioPlayer) IsLooping() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.looping
}

func (p *AudioPlayer) ResetEOF() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.eofReceived = false
}

func (p *AudioPlayer) Read(buf []byte) (int, error) {
	if len(p.leftover) > 0 {
		n := copy(buf, p.leftover)
		p.leftover = p.leftover[n:]
		return n, nil
	}

	for {
		p.mu.Lock()
		paused := p.paused
		looping := p.looping
		clock := p.clock
		p.mu.Unlock()

		if paused && clock != nil {
			clock.WaitForPTS(clock.GetTime())
			continue
		}

		if paused {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		pkt, ok := p.queue.Get()
		if !ok {
			if looping {
				p.mu.Lock()
				p.eofReceived = true
				p.mu.Unlock()
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return 0, io.EOF
		}

		p.mu.Lock()
		if p.eofReceived {
			p.eofReceived = false
		}
		p.mu.Unlock()

		defer C.av_packet_free(&pkt)

		if C.avcodec_send_packet(p.codecCtx, pkt) != 0 {
			logging.Debug(logging.CatPlayer, "Failed to send packet to audio decoder")
			continue
		}

		for C.avcodec_receive_frame(p.codecCtx, p.frame) == 0 {
			pts := float64(p.frame.pts) * p.timeBase

			if clock != nil {
				clock.SetTime(pts)
			}

			data, err := p.resample()
			if err != nil {
				logging.Warning(logging.CatPlayer, "Audio resample error: %v", err)
				continue
			}

			if len(data) == 0 {
				continue
			}

			p.mu.Lock()
			volMul := p.volumeMul
			p.mu.Unlock()

			if volMul != 1.0 && volMul != 0 {
				data = p.applyVolume(data, volMul)
			}

			n := copy(buf, data)
			if n < len(data) {
				p.leftover = data[n:]
			}
			return n, nil
		}
	}
}

func (p *AudioPlayer) applyVolume(data []byte, volMul float32) []byte {
	if volMul == 1.0 {
		return data
	}

	result := make([]byte, len(data))
	for i := 0; i < len(data)-1; i += 2 {
		sample := int16(data[i]) | int16(data[i+1])<<8
		sample = int16(float32(sample) * volMul)
		result[i] = byte(sample)
		result[i+1] = byte(sample >> 8)
	}
	return result
}

func (p *AudioPlayer) resample() ([]byte, error) {
	maxSamples := int(C.swr_get_out_samples(p.swrCtx, p.frame.nb_samples))
	if maxSamples <= 0 {
		maxSamples = 1024
	}

	neededSize := maxSamples * TargetChannels * 2
	if len(p.resampleBuf) < neededSize {
		p.resampleBuf = make([]byte, neededSize)
	}

	outPtr := (*C.uint8_t)(unsafe.Pointer(&p.resampleBuf[0]))
	gotSamples := C.swr_convert(p.swrCtx, &outPtr, C.int(maxSamples), &p.frame.data[0], p.frame.nb_samples)
	if gotSamples < 0 {
		logging.Error(logging.CatPlayer, "Audio resample failed with code: %d", gotSamples)
		return nil, fmt.Errorf("resample error: %d", gotSamples)
	}

	if gotSamples == 0 {
		return nil, nil
	}

	return p.resampleBuf[:int(gotSamples)*TargetChannels*2], nil
}

func (p *AudioPlayer) Close() {
	if p.otoPlayer != nil {
		p.otoPlayer.Close()
		p.otoPlayer = nil
	}
	if p.otoCtx != nil {
		p.otoCtx.Close()
		p.otoCtx = nil
	}
	if p.swrCtx != nil {
		C.swr_free(&p.swrCtx)
		p.swrCtx = nil
	}
	if p.frame != nil {
		C.av_frame_free(&p.frame)
		p.frame = nil
	}
}

func timeSleep(d time.Duration) {
	time.Sleep(d)
}
