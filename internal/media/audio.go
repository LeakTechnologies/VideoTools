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
	"math"
	"sync"
	"sync/atomic"
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
// Read() drives the master clock via clock.SetTime(pts - AudioBufferLatency).
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
	speed    float64 // playback speed; 1.0 = normal, 0.5 = half speed, 2.0 = double speed

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

	// lastPTSBits holds math.Float64bits of the most recent audio PTS fed to
	// the master clock via Read().  Atomic so GetLastPTS() is lock-free.
	lastPTSBits atomic.Uint64

	// Diagnostic / state flags (all protected by codecMu or mu as noted)
	codecDead bool        // set when codec raises a pre-flight error; stops decode loop
	closed    atomic.Bool // set by Close(); causes Read() to return io.EOF immediately
}

func NewAudioPlayer(codecCtx *C.AVCodecContext, queue *PacketQueue, clock *MasterClock, timeBase float64) (*AudioPlayer, error) {
	p := &AudioPlayer{
		codecCtx:   codecCtx,
		queue:      queue,
		clock:      clock,
		frame:      C.av_frame_alloc(),
		timeBase:   timeBase,
		speed:      1.0,
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
	// Do NOT call Play() here - audio will start playing immediately.
	// Instead, audio is started when user presses Play() via AudioPlayer.Resume().
	// Set paused=true initially so Read() returns silence until Play is called.
	p.paused = true

	logging.Info(logging.CatPlayer, "NewAudioPlayer: player created (not playing yet)")

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

		// Block up to 20 ms for the next audio packet.  This avoids the
		// 1 ms spin loop that competed with videoDecodeLoop for CPU during
		// SW H.264 decode when the audio queue was frequently empty.
		// TryPut calls cond.Signal(), so TimedGet wakes immediately on
		// a new packet rather than waiting the full 20 ms.
		pkt, ok := p.queue.TimedGet(20 * time.Millisecond)
		if !ok {
			if p.queue.IsClosedOrEOF() {
				logging.Info(logging.CatPlayer, "audioDecodeLoop: queue EOF/closed, exiting")
				return
			}
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

			// PCM is pushed to pcmCh; Read() will call clock.SetTime(pts-latency)
			// when it consumes the chunk, driving the master clock from audio output.

			data, err := p.resample()
			if err != nil {
				logging.Warning(logging.CatPlayer, "audioDecodeLoop: resample error: %v", err)
				p.codecMu.Lock()
				break
			}

			if len(data) > 0 {
				// Copy to avoid sharing the resampler's internal buffer.
				pcmData := make([]byte, len(data))
				copy(pcmData, data)

				// Volume is applied in Read() — do it once there on the final
				// S16 buffer so it's applied to the exact bytes sent to hardware.
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

func (p *AudioPlayer) Read(buf []byte) (int, error) {
	if p.closed.Load() {
		return 0, io.EOF
	}

	p.mu.Lock()
	speed := p.speed
	p.mu.Unlock()

	// Drain any leftover from the previous chunk first.
	p.mu.Lock()
	leftoverLen := len(p.leftover)
	var n int
	if leftoverLen > 0 {
		n = copy(buf, p.leftover)
		p.leftover = p.leftover[n:]
	}
	volumeMul := p.volumeMul
	p.mu.Unlock()

	if leftoverLen > 0 {
		if volumeMul != 1.0 {
			applyVolumeS16(buf[:n], volumeMul)
		}
		if speed != 1.0 {
			adjusted := p.adjustSamplesForSpeed(buf[:n], speed)
			n = copy(buf, adjusted)
		}
		return n, nil
	}

	p.mu.Lock()
	paused := p.paused
	looping := p.looping
	p.mu.Unlock()

	if paused {
		// Drain one buffered chunk per call without advancing the clock.
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
		p.clock.SetTime(chunk.pts - AudioBufferLatency.Seconds())
		p.lastPTSBits.Store(math.Float64bits(chunk.pts))

		// Adjust chunk data based on speed.
		adjustedData := p.adjustSamplesForSpeed(chunk.data, speed)

		n := copy(buf, adjustedData)

		p.mu.Lock()
		volumeMul := p.volumeMul
		p.mu.Unlock()
		if volumeMul != 1.0 {
			applyVolumeS16(buf[:n], volumeMul)
		}

		if n < len(adjustedData) {
			p.mu.Lock()
			p.leftover = adjustedData[n:]
			p.mu.Unlock()
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

// adjustSamplesForSpeed adjusts audio samples based on playback speed.
// At 0.5x: duplicates samples (slower playback).
// At 2.0x: skips samples (faster playback).
func (p *AudioPlayer) adjustSamplesForSpeed(data []byte, speed float64) []byte {
	if speed <= 0 {
		speed = 1.0
	}

	// S16 stereo: 4 bytes per sample frame (2 bytes per channel, 2 channels).
	bytesPerFrame := 4
	if speed == 1.0 {
		return data
	}

	// Calculate how many output frames we need from input frames.
	inputFrames := len(data) / bytesPerFrame
	var outputFrames int
	if speed < 1.0 {
		// Slower: we need more frames (duplicate).
		outputFrames = int(float64(inputFrames) / speed)
	} else {
		// Faster: we need fewer frames (skip).
		outputFrames = int(float64(inputFrames) / speed)
	}

	output := make([]byte, outputFrames*bytesPerFrame)
	outputIdx := 0
	for i := 0; i < inputFrames && outputIdx < len(output); i++ {
		srcStart := i * bytesPerFrame
		srcEnd := srcStart + bytesPerFrame
		if speed < 1.0 {
			// Duplicate this frame.
			copy(output[outputIdx:], data[srcStart:srcEnd])
			outputIdx += bytesPerFrame
			// For very slow speeds, duplicate multiple times.
			duplicates := int(1.0 / speed)
			for d := 1; d < duplicates && outputIdx < len(output); d++ {
				copy(output[outputIdx:], data[srcStart:srcEnd])
				outputIdx += bytesPerFrame
			}
		} else {
			// Skip frames (we already divided inputFrames by speed).
			copy(output[outputIdx:], data[srcStart:srcEnd])
			outputIdx += bytesPerFrame
		}
	}
	return output[:outputIdx]
}

func (p *AudioPlayer) FlushCodec() {
	p.mu.Lock()
	p.leftover = p.leftover[:0]
	p.mu.Unlock()
drain:
	for {
		select {
		case <-p.pcmCh:
		default:
			break drain
		}
	}
	p.codecMu.Lock()
	C.avcodec_flush_buffers(p.codecCtx)
	p.codecMu.Unlock()
}

// DrainPCM discards all buffered PCM chunks without flushing the codec.
// Used by ResetAfterGrab to clear pre-buffered audio from GrabFrame without
// resetting codec state (the initial packet/frame has already been sent).
func (p *AudioPlayer) DrainPCM() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.leftover = p.leftover[:0]
	for len(p.pcmCh) > 0 {
		<-p.pcmCh
	}
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

func applyVolumeS16(buf []byte, vol float32) {
	// S16 stereo: each sample is 2 bytes (L, R interleaved).
	// Process in 2-byte steps.
	n := len(buf) &^ 1 // round down to even
	for i := 0; i < n; i += 2 {
		sample := int16(buf[i]) | int16(buf[i+1])<<8
		sample = int16(float32(sample) * vol)
		buf[i] = byte(sample)
		buf[i+1] = byte(sample >> 8)
	}
}

func (p *AudioPlayer) Pause() {
	p.mu.Lock()
	p.paused = true
	p.mu.Unlock()
	if p.otoPlayer != nil {
		p.otoPlayer.Pause()
	}
}

func (p *AudioPlayer) Resume() {
	p.mu.Lock()
	p.paused = false
	p.mu.Unlock()
	if p.otoPlayer != nil {
		p.otoPlayer.Play()
	}
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

// GetLastPTS returns the PTS of the most recent audio chunk consumed by Read().
// Returns -1 if no audio has been output yet.
func (p *AudioPlayer) GetLastPTS() float64 {
	bits := p.lastPTSBits.Load()
	if bits == 0 {
		return -1
	}
	return math.Float64frombits(bits)
}

func (p *AudioPlayer) ResetLastPTS() {
	p.lastPTSBits.Store(0)
	logging.Info(logging.CatPlayer, "AudioPlayer: ResetLastPTS (clock reset)")
}

func (p *AudioPlayer) SetSpeed(speed float64) {
	p.mu.Lock()
	p.speed = speed
	p.mu.Unlock()
	logging.Info(logging.CatPlayer, "AudioPlayer.SetSpeed: speed=%.2f", speed)
}

func (p *AudioPlayer) Close() {
	// Mark closed immediately so any in-flight Read() call returns io.EOF
	// rather than accessing fields we are about to tear down.
	p.closed.Store(true)

	// Stop the decode goroutine. When it exits its deferred close(p.pcmCh)
	// fires, which causes any pending Read() select to unblock with ok=false.
	if p.decodeStop != nil {
		close(p.decodeStop)
		p.decodeStop = nil
	}
	p.decodeWg.Wait()

	// Pause then close the oto player. Pause() stops audio output immediately;
	// Close() blocks until the current Read() invocation (if any) completes,
	// ensuring no concurrent access to p's fields from the oto callback thread
	// after this point.
	if p.otoPlayer != nil {
		p.otoPlayer.Pause()
		p.otoPlayer.Close()
		p.otoPlayer = nil
	}
	p.otoCtx = nil

	// Drain any remaining PCM now that both the decode goroutine and oto are
	// fully stopped — no concurrent receiver on pcmCh any more.
	for len(p.pcmCh) > 0 {
		<-p.pcmCh
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
