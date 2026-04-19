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

// pcmChannelCap is the number of decoded PCM chunks the decode goroutine can
// buffer ahead of Read().  Each AAC frame is 1024 samples (~23 ms at 44.1 kHz);
// 64 slots gives ~1.5 s of headroom before backpressure kicks in.
const pcmChannelCap = 64

// audioChunk is a decoded, resampled PCM buffer tagged with its stream PTS.
// The PTS is carried for diagnostics; the master clock is driven purely by
// wall time (engine.Resume → clock.SetPaused(false)) and not updated here.
type audioChunk struct {
	pts  float64
	data []byte
}

// AudioPlayer decodes audio packets on a dedicated goroutine (audioDecodeLoop)
// and exposes the result through the io.Reader interface used by oto.
//
// CRITICAL: no FFmpeg calls are made from the oto goroutine.  avcodec_send_packet
// and avcodec_receive_frame run exclusively inside audioDecodeLoop.  This avoids
// the class of Windows C-level access violations that kill the process when those
// calls are made from oto's audio-callback goroutine.
type AudioPlayer struct {
	codecCtx *C.AVCodecContext
	swrCtx   *C.struct_SwrContext
	queue    *PacketQueue
	clock    *MasterClock

	otoCtx    *oto.Context
	otoPlayer *oto.Player

	frame       *C.AVFrame
	resampleBuf []byte

	timeBase float64

	volume float32
	muted  bool
	paused bool

	mu        sync.Mutex
	codecMu   sync.Mutex // serialises avcodec_send/receive against FlushCodec()
	volumeMul float32
	looping   bool

	// Decode bridge: audioDecodeLoop → Read()
	pcmCh      chan audioChunk // decoded, resampled, volume-applied PCM chunks
	leftover   []byte          // partial chunk carried across Read() calls
	decodeStop chan struct{}    // closed by Close() to stop audioDecodeLoop
	decodeWg   sync.WaitGroup

	// Diagnostic / state flags (all protected by codecMu or mu as noted)
	codecDead bool // set when codec raises a pre-flight error; stops decode loop
}

func NewAudioPlayer(codecCtx *C.AVCodecContext, queue *PacketQueue, clock *MasterClock, timeBase float64) (*AudioPlayer, error) {
	p := &AudioPlayer{
		codecCtx:   codecCtx,
		queue:      queue,
		clock:      clock,
		frame:      C.av_frame_alloc(),
		timeBase:   timeBase,
		volume:     1.0,
		volumeMul:  1.0,
		paused:     true, // stay silent until engine.Resume() — prevents audio bleed during Load/GrabFrame
		pcmCh:      make(chan audioChunk, pcmChannelCap),
		decodeStop: make(chan struct{}),
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

	// Start the decode bridge goroutine before creating the oto player.
	// oto calls Read() as soon as Play() is called; the decode loop must be
	// running first so it can start filling pcmCh.
	p.decodeWg.Add(1)
	go p.audioDecodeLoop()

	p.otoPlayer = otoCtx.NewPlayer(p)
	logging.Info(logging.CatPlayer, "NewAudioPlayer: player created, calling Play()")
	p.otoPlayer.Play()
	logging.Info(logging.CatPlayer, "NewAudioPlayer: Play() returned OK")

	return p, nil
}

// audioDecodeLoop runs on a dedicated goroutine.  It is the ONLY place that
// calls avcodec_send_packet / avcodec_receive_frame — keeping FFmpeg off the
// oto goroutine.  Decoded PCM is sent to pcmCh for Read() to consume.
func (p *AudioPlayer) audioDecodeLoop() {
	defer p.decodeWg.Done()
	defer close(p.pcmCh) // signals Read() that no more PCM is coming

	defer func() {
		if r := recover(); r != nil {
			logging.Error(logging.CatPlayer, "audioDecodeLoop panic: %v", r)
		}
	}()

	logging.Info(logging.CatPlayer, "audioDecodeLoop: started (decode bridge goroutine)")
	firstPkt := true

	for {
		// Check stop signal without blocking.
		select {
		case <-p.decodeStop:
			return
		default:
		}

		p.mu.Lock()
		paused := p.paused
		p.mu.Unlock()

		if paused {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		// Non-blocking packet fetch.
		pkt, ok := p.queue.TryGet()
		if !ok {
			if p.queue.IsClosedOrEOF() {
				logging.Info(logging.CatPlayer, "audioDecodeLoop: queue EOF/closed, exiting")
				return
			}
			time.Sleep(1 * time.Millisecond)
			continue
		}

		p.codecMu.Lock()

		if p.codecDead {
			p.codecMu.Unlock()
			C.av_packet_free(&pkt)
			return
		}

		if firstPkt {
			firstPkt = false
			var diag C.CodecDiagnostic
			C.diagnose_avcodec_state(p.codecCtx, pkt, &diag)
			logging.Info(logging.CatPlayer,
				"audioDecodeLoop: first pkt size=%d ctx=%d codec=%d priv_data=%d extradata_size=%d extradata=%d rate=%d ch=%d codec_id=%d",
				pkt.size,
				int(diag.ctx_ok), int(diag.codec_ok), int(diag.priv_data_ok),
				int(diag.extradata_size), int(diag.extradata_ok),
				int(diag.sample_rate), int(diag.channels), int(diag.codec_id))
		}

		var excCode C.uint32_t
		sendRet := C.safe_avcodec_send_packet(p.codecCtx, pkt, &excCode)
		C.av_packet_free(&pkt)

		if excCode != 0 {
			logging.Error(logging.CatPlayer,
				"audioDecodeLoop: avcodec_send_packet pre-flight failed (exc=0x%08X) — disabling audio",
				uint32(excCode))
			p.codecDead = true
			p.codecMu.Unlock()
			return
		}
		if sendRet != 0 {
			p.codecMu.Unlock()
			continue
		}

		logging.Info(logging.CatPlayer, "audioDecodeLoop: avcodec_send_packet OK, receiving frames")

		// Receive all frames produced by this packet.
		for {
			var recvExc C.uint32_t
			recvRet := C.safe_avcodec_receive_frame(p.codecCtx, p.frame, &recvExc)
			if recvExc != 0 {
				logging.Error(logging.CatPlayer,
					"audioDecodeLoop: avcodec_receive_frame exception (exc=0x%08X) — disabling audio",
					uint32(recvExc))
				p.codecDead = true
				p.codecMu.Unlock()
				return
			}
			if recvRet != 0 {
				break // EAGAIN or EOF — need the next packet
			}

			pts := float64(p.frame.pts) * p.timeBase
			p.codecMu.Unlock()

			// The master clock is NOT driven from audio at all.
			// It runs on pure wall time from the moment engine.Resume() is
			// called. Audio and video both anchor to that same reference, so
			// no clock.SetTime() calls are needed here or in Read().

			data, err := p.resample()
			if err != nil {
				logging.Warning(logging.CatPlayer, "audioDecodeLoop: resample error: %v", err)
				p.codecMu.Lock()
				break
			}

			if len(data) > 0 {
				p.mu.Lock()
				volMul := p.volumeMul
				p.mu.Unlock()
				if volMul != 1.0 && volMul != 0 {
					data = p.applyVolume(data, volMul)
				}

				// Copy to avoid sharing the resampler's internal buffer.
				pcmData := make([]byte, len(data))
				copy(pcmData, data)

				// Send to Read() tagged with this frame's PTS so the clock
				// advances when the data is played, not when it is decoded.
				select {
				case p.pcmCh <- audioChunk{pts: pts, data: pcmData}:
				case <-p.decodeStop:
					return
				}
			}

			p.codecMu.Lock()
		}
		p.codecMu.Unlock()
	}
}

// Read is called by oto's audio goroutine.  It ONLY reads from pcmCh — no
// FFmpeg calls are made here.
func (p *AudioPlayer) Read(buf []byte) (int, error) {
	// Drain any leftover from the previous chunk first.
	if len(p.leftover) > 0 {
		n := copy(buf, p.leftover)
		p.leftover = p.leftover[n:]
		return n, nil
	}

	p.mu.Lock()
	paused := p.paused
	looping := p.looping
	p.mu.Unlock()

	if paused {
		// Drain one buffered chunk per call without advancing the clock.
		// This empties pcmCh while paused so that on resume the clock does
		// not jump forward by however much audio was pre-decoded (up to ~1.5s),
		// which was causing visible seek jumps when toggling pause.
		select {
		case <-p.pcmCh:
		default:
		}
		for i := range buf {
			buf[i] = 0
		}
		return len(buf), nil
	}

	// Non-blocking receive: if the decode goroutine hasn't produced a chunk
	// yet, return silence so oto's goroutine never stalls.
	select {
	case chunk, ok := <-p.pcmCh:
		if !ok {
			// Channel closed — decode goroutine exited (EOF or error).
			if looping {
				p.mu.Lock()
				p.mu.Unlock()
				for i := range buf {
					buf[i] = 0
				}
				return len(buf), nil
			}
			return 0, io.EOF
		}
		// Drive the master clock from audio output position.
		// This chunk is being handed to oto's hardware buffer; it will actually
		// reach the speakers after AudioBufferLatency.  Subtracting that latency
		// makes the clock represent "what's being heard right now" rather than
		// "what's been decoded/buffered".  Video WaitForPTS then waits until the
		// clock (= audio output position) reaches the video frame's PTS,
		// producing true A/V sync without any fixed wall-time offset.
		p.clock.SetTime(chunk.pts - AudioBufferLatency.Seconds())
		n := copy(buf, chunk.data)
		if n < len(chunk.data) {
			p.leftover = chunk.data[n:]
		}
		return n, nil
	default:
		// No decoded audio ready yet — return silence.
		for i := range buf {
			buf[i] = 0
		}
		return len(buf), nil
	}
}

// FlushCodec discards buffered PCM and flushes the codec's internal state.
// Called from Seek() — must not be called concurrently with Close().
func (p *AudioPlayer) FlushCodec() {
	// Discard any pending decoded audio so seek takes effect immediately.
	p.leftover = p.leftover[:0]
	for len(p.pcmCh) > 0 {
		<-p.pcmCh
	}
	// Flush the codec under its lock.
	p.codecMu.Lock()
	C.avcodec_flush_buffers(p.codecCtx)
	p.codecMu.Unlock()
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
	// Nothing to reset — EOF is signalled by pcmCh being closed.
}

func (p *AudioPlayer) Close() {
	// Stop the decode goroutine first; it must exit before we free the codec.
	if p.decodeStop != nil {
		close(p.decodeStop)
		p.decodeStop = nil
	}
	p.decodeWg.Wait()

	// Drain any remaining decoded PCM so the closed channel can be GC'd.
	for len(p.pcmCh) > 0 {
		<-p.pcmCh
	}

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

	// audio_swr_convert_packed is a CGo-safe wrapper: it takes a flat
	// uint8_t* and forms the double-pointer on the C stack internally,
	// avoiding the "Go pointer to unpinned Go pointer" CGo violation that
	// occurs when passing &outPtr directly to swr_convert from Go.
	outPtr := (*C.uint8_t)(unsafe.Pointer(&p.resampleBuf[0]))
	gotSamples := C.audio_swr_convert_packed(p.swrCtx, outPtr, C.int(maxSamples), p.frame)
	if gotSamples < 0 {
		logging.Error(logging.CatPlayer, "Audio resample failed with code: %d", gotSamples)
		return nil, fmt.Errorf("resample error: %d", gotSamples)
	}
	if gotSamples == 0 {
		return nil, nil
	}
	return p.resampleBuf[:int(gotSamples)*TargetChannels*2], nil
}

func timeSleep(d time.Duration) {
	time.Sleep(d)
}
