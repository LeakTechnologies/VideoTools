//go:build native_media

package media

/*
#cgo !windows pkg-config: libavcodec libswresample libavutil
#cgo windows CFLAGS: -IC:/ffmpeg/include
#cgo windows LDFLAGS: -LC:/ffmpeg/lib -lavcodec -lavutil -lswresample
#include <libavcodec/avcodec.h>
#include <libswresample/swresample.h>
#include <libavutil/avutil.h>
#include <libavutil/opt.h>
#include <libavutil/channel_layout.h>
#include "safe_bridge.h"
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
	codecMu     sync.Mutex // serialises codec ops in Read() against FlushCodec() from Seek()
	volumeMul   float32
	eofReceived bool
	looping     bool
	firstPktLogged bool
	codecDead   bool // set when avcodec_send_packet raises a C-level exception; disables decode permanently
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

	var outLayout C.AVChannelLayout
	C.av_channel_layout_default(&outLayout, 2)
	C.av_opt_set_chlayout(unsafe.Pointer(p.swrCtx), C.CString("out_chlayout"), &outLayout, 0)
	C.av_opt_set_int(unsafe.Pointer(p.swrCtx), C.CString("out_sample_rate"), TargetSampleRate, 0)
	C.av_opt_set_sample_fmt(unsafe.Pointer(p.swrCtx), C.CString("out_sample_fmt"), C.AV_SAMPLE_FMT_S16, 0)

	logging.Info(logging.CatPlayer, "NewAudioPlayer: swr_init rate=%d fmt=%d", p.codecCtx.sample_rate, p.codecCtx.sample_fmt)
	if C.swr_init(p.swrCtx) < 0 {
		C.swr_free(&p.swrCtx)
		C.av_frame_free(&p.frame)
		return nil, fmt.Errorf("failed to initialize resampler")
	}
	logging.Info(logging.CatPlayer, "NewAudioPlayer: swr_init OK, getting audio context")

	otoCtx, err := GetSharedAudioContext()
	if err != nil {
		C.swr_free(&p.swrCtx)
		C.av_frame_free(&p.frame)
		return nil, fmt.Errorf("failed to create audio context: %w", err)
	}
	logging.Info(logging.CatPlayer, "NewAudioPlayer: got audio context, creating player")

	p.otoCtx = otoCtx
	p.otoPlayer = otoCtx.NewPlayer(p)
	logging.Info(logging.CatPlayer, "NewAudioPlayer: player created, calling Play()")
	p.otoPlayer.Play()
	logging.Info(logging.CatPlayer, "NewAudioPlayer: Play() returned OK")

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
	// Drain any leftover decoded audio first.
	if len(p.leftover) > 0 {
		n := copy(buf, p.leftover)
		p.leftover = p.leftover[n:]
		return n, nil
	}

	p.mu.Lock()
	paused := p.paused
	looping := p.looping
	p.mu.Unlock()

	// Paused — return silence immediately so oto's goroutine never blocks.
	if paused {
		for i := range buf {
			buf[i] = 0
		}
		return len(buf), nil
	}

	// Non-blocking packet fetch. If nothing is queued yet (e.g. engine.Start()
	// hasn't run) we return silence rather than blocking. This is what allows
	// NewAudioPlayer / Play() to return before the demux goroutine is running.
	pkt, ok := p.queue.TryGet()
	if !ok {
		// Distinguish "nothing yet" from "stream finished".
		if p.queue.IsClosedOrEOF() {
			if looping {
				p.mu.Lock()
				p.eofReceived = true
				p.mu.Unlock()
			} else {
				return 0, io.EOF
			}
		}
		// Queue temporarily empty — output silence.
		for i := range buf {
			buf[i] = 0
		}
		return len(buf), nil
	}

	// We have a packet — free it when done regardless of the decode path.
	defer C.av_packet_free(&pkt)

	p.mu.Lock()
	p.eofReceived = false
	p.mu.Unlock()

	// codecMu serialises decode operations against FlushCodec() called from
	// Seek(). AVCodecContext is not thread-safe — concurrent send/receive and
	// flush_buffers cause hard crashes.
	p.codecMu.Lock()

	// If a previous C-level exception killed the codec, return silence forever.
	if p.codecDead {
		p.codecMu.Unlock()
		for i := range buf {
			buf[i] = 0
		}
		return len(buf), nil
	}

	if !p.firstPktLogged {
		p.firstPktLogged = true
		var diag C.CodecDiagnostic
		C.diagnose_avcodec_state(p.codecCtx, pkt, &diag)
		logging.Info(logging.CatPlayer,
			"AudioPlayer.Read: first pkt (size=%d) — ctx=%d codec=%d priv_data=%d extradata_size=%d extradata=%d rate=%d ch=%d codec_id=%d",
			pkt.size,
			int(diag.ctx_ok), int(diag.codec_ok), int(diag.priv_data_ok),
			int(diag.extradata_size), int(diag.extradata_ok),
			int(diag.sample_rate), int(diag.channels), int(diag.codec_id))
	}

	var excCode C.uint32_t
	sendRet := C.safe_avcodec_send_packet(p.codecCtx, pkt, &excCode)
	if excCode != 0 {
		logging.Error(logging.CatPlayer, "AudioPlayer.Read: safe_avcodec_send_packet pre-flight failed (exc=0x%08X) — disabling audio decode", uint32(excCode))
		p.codecDead = true
		p.codecMu.Unlock()
		for i := range buf {
			buf[i] = 0
		}
		return len(buf), nil
	}
	if sendRet != 0 {
		p.codecMu.Unlock()
		logging.Debug(logging.CatPlayer, "Failed to send packet to audio decoder (ret=%d)", int(sendRet))
		for i := range buf {
			buf[i] = 0
		}
		return len(buf), nil
	}

	var recvExc C.uint32_t
	for {
		recvRet := C.safe_avcodec_receive_frame(p.codecCtx, p.frame, &recvExc)
		if recvExc != 0 {
			logging.Error(logging.CatPlayer, "AudioPlayer.Read: avcodec_receive_frame raised Windows exception 0x%08X — disabling audio decode", uint32(recvExc))
			p.codecDead = true
			break
		}
		if recvRet != 0 {
			break
		}
		pts := float64(p.frame.pts) * p.timeBase
		p.codecMu.Unlock()

		if p.clock != nil {
			p.clock.SetTime(pts)
		}

		data, err := p.resample()
		if err != nil {
			logging.Warning(logging.CatPlayer, "Audio resample error: %v", err)
			return len(buf), nil
		}
		if len(data) == 0 {
			return len(buf), nil
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
	p.codecMu.Unlock()

	// Packet decoded but yielded no audio frames — return silence.
	for i := range buf {
		buf[i] = 0
	}
	return len(buf), nil
}

// FlushCodec flushes the audio codec's internal buffers. Must be called from
// Seek() instead of avcodec_flush_buffers directly, so it is serialised
// against any concurrent decode in Read().
func (p *AudioPlayer) FlushCodec() {
	p.codecMu.Lock()
	C.avcodec_flush_buffers(p.codecCtx)
	p.codecMu.Unlock()
	p.leftover = p.leftover[:0]
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
