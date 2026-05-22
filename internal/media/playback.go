//go:build native_media

package media

/*
#cgo !windows pkg-config: libavcodec libavformat libswscale libavutil
#cgo windows CFLAGS: -IC:/ffmpeg/include
#cgo windows LDFLAGS: -LC:/ffmpeg/lib -lavcodec -lavformat -lavutil -lswscale -lswresample -lavfilter -lbcrypt -lSecur32 -lWs2_32 -lmfplat -lstrmiids -lavrt -lole32 -luser32 -Wl,--stack,4194304
#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libswscale/swscale.h>
#include <libavutil/avutil.h>
#include <libavutil/hwcontext.h>

// avformat_get_stream — safely return the n-th AVStream* from an AVFormatContext.
static AVStream* avformat_get_stream(AVFormatContext *fmtCtx, unsigned int idx) {
    if (fmtCtx == NULL || idx >= (unsigned int)fmtCtx->nb_streams) {
        return NULL;
    }
    return fmtCtx->streams[idx];
}
*/
import "C"
import (
	"fmt"
	"image"
	"io"
	"math"
	"runtime"
	"sync/atomic"
	"time"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
)

func (e *Engine) Start() {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.paused = true
	e.mu.Unlock()

	logging.Info(logging.CatPlayer, "Engine.Start: starting demuxerLoop")
	// Clock stays paused here — Resume() unpauses it on the first Play().
	// Start() runs during Load (before the user presses Play); if we unpaused
	// here the clock would tick through the entire idle window and the first
	// video frame would be seconds behind by the time Play is pressed.

	e.demuxerWg.Add(1)
	go e.demuxerLoop()
}

func (e *Engine) demuxerLoop() {
	defer e.demuxerWg.Done()
	defer func() {
		if r := recover(); r != nil {
			logging.Error(logging.CatPlayer, "demuxerLoop panic: %v", r)
			e.videoQueue.SetEOF()
			e.audioQueue.SetEOF()
			e.subtitleQueue.SetEOF()
		}
	}()

	logging.Info(logging.CatPlayer, "demuxerLoop: started (vidIdx=%d audioIdx=%d)", e.videoStreamIdx, e.audioStreamIdx)

	pkt := C.av_packet_alloc()
	if pkt == nil {
		logging.Error(logging.CatPlayer, "demuxerLoop: av_packet_alloc returned nil")
		e.videoQueue.SetEOF()
		e.audioQueue.SetEOF()
		return
	}
	defer C.av_packet_free(&pkt)

	firstPkt := true
	for {
		select {
		case <-e.stop:
			logging.Info(logging.CatPlayer, "demuxerLoop: stop signal received, exiting")
			return
		default:
		}

		// Serialise av_read_frame against avformat_seek_file — AVFormatContext
		// is not thread-safe; concurrent access from Seek() causes hard crashes.
		e.formatMu.Lock()
		ret := C.av_read_frame(e.formatCtx, pkt)
		e.formatMu.Unlock()

		if firstPkt {
			firstPkt = false
			logging.Info(logging.CatPlayer, "demuxerLoop: first av_read_frame ret=%d stream=%d", int(ret), int(pkt.stream_index))
		}

		if ret < 0 {
			logging.Info(logging.CatPlayer, "demuxerLoop: av_read_frame EOF/error ret=%d, setting queue EOF", int(ret))
			e.videoQueue.SetEOF()
			e.audioQueue.SetEOF()
			e.subtitleQueue.SetEOF()
			return
		}

		streamIdx := int(pkt.stream_index)
		if streamIdx == e.videoStreamIdx {
			e.videoQueue.Put(pkt) // blocking: never drop video packets
		} else if streamIdx == e.audioStreamIdx {
			// Non-blocking: if the audio queue is saturated, discard the
			// packet rather than stalling the demuxer and starving the video
			// queue.  A skipped AAC frame (23 ms) is inaudible compared to
			// a several-second video freeze.
			e.audioQueue.TryPut(pkt)
		} else if streamIdx == e.subtitleStreamIdx && e.subtitleCodecCtx != nil {
			e.subtitleQueue.TryPut(pkt)
		}
		C.av_packet_unref(pkt)
	}
}

func (e *Engine) Seek(seconds float64) error {
	if seconds < 0 {
		seconds = 0
	}
	logging.Info(logging.CatPlayer, "Seeking to %.2f seconds (accuracy: %v)", seconds, e.seekAcc)

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.formatCtx == nil || e.videoStreamIdx < 0 {
		return fmt.Errorf("no media opened")
	}

	target := C.int64_t(seconds / e.videoTimeBase)

	var flags C.int
	var minTS, maxTS C.int64_t
	maxTS = target
	switch e.seekAcc {
	case SeekAccuracyFrame:
		flags = C.int(AVSEEK_FLAG_FRAME)
		minTS = target
	case SeekAccuracyKeyframe:
		minTS = C.int64_t(math.MinInt64 / 2)
		if target == 0 {
			maxTS = 1000
		}
	case SeekAccuracyAccurate:
		flags = C.int(AVSEEK_FLAG_BACKWARD | AVSEEK_FLAG_ACCURATE)
		minTS = C.int64_t(math.MinInt64 / 2)
		if target == 0 {
			maxTS = 1000
		}
	}

	e.formatMu.Lock()
	seekRet := C.avformat_seek_file(e.formatCtx, C.int(e.videoStreamIdx), minTS, target, maxTS, flags)

	if seekRet >= 0 && e.seekAcc == SeekAccuracyKeyframe {
		landedSecs := -1.0
		peekPkt := C.av_packet_alloc()
		if peekPkt != nil {
			for i := 0; i < 5 && landedSecs < 0; i++ {
				if C.av_read_frame(e.formatCtx, peekPkt) < 0 {
					break
				}
				if int(peekPkt.stream_index) == e.videoStreamIdx &&
					int64(peekPkt.pts) != int64(C.AV_NOPTS_VALUE) {
					landedSecs = float64(peekPkt.pts) * e.videoTimeBase
				}
				C.av_packet_unref(peekPkt)
			}
			C.av_packet_free(&peekPkt)
		}

		if landedSecs >= 0 && math.Abs(landedSecs-seconds) > 2.0 {
			logging.Info(logging.CatPlayer, "Seek: keyframe landed at %.2f (target %.2f, diff=%.1fs) — accurate fallback", landedSecs, seconds, math.Abs(landedSecs-seconds))
			accurateRet := C.avformat_seek_file(e.formatCtx, C.int(e.videoStreamIdx),
				C.int64_t(math.MinInt64/2), target, target,
				C.int(AVSEEK_FLAG_ACCURATE))
			if accurateRet < 0 {
				logging.Warning(logging.CatPlayer, "Seek: accurate fallback failed (ret=%d) — restoring keyframe position", accurateRet)
				C.avformat_seek_file(e.formatCtx, C.int(e.videoStreamIdx), minTS, target, maxTS, flags)
			} else {
				logging.Info(logging.CatPlayer, "Seek: accurate fallback OK")
			}
		} else {
			C.avformat_seek_file(e.formatCtx, C.int(e.videoStreamIdx), minTS, target, maxTS, flags)
		}
	}

	e.formatMu.Unlock()
	if seekRet < 0 {
		logging.Warning(logging.CatPlayer, "Seek to %.2f failed (ret=%d 0x%08X)", seconds, seekRet, uint32(seekRet))
		return fmt.Errorf("seek failed")
	}
	logging.Info(logging.CatPlayer, "Seek to %.2f OK, flushing queues", seconds)

	e.videoQueue.Flush()
	e.audioQueue.Flush()
	logging.Info(logging.CatPlayer, "Seek: queues flushed, flushing video codec")

	if e.audioStreamIdx >= 0 && e.formatCtx != nil {
		stream := C.avformat_get_stream(e.formatCtx, C.uint(e.audioStreamIdx))
		if stream == nil || stream.codecpar == nil {
			logging.Warning(logging.CatPlayer, "Seek: audio stream %d no longer valid, searching for new audio stream", e.audioStreamIdx)
			for i := 0; i < int(e.formatCtx.nb_streams); i++ {
				s := C.avformat_get_stream(e.formatCtx, C.uint(i))
				if s != nil && s.codecpar != nil && s.codecpar.codec_type == C.AVMEDIA_TYPE_AUDIO {
					e.audioStreamIdx = i
					logging.Info(logging.CatPlayer, "Seek: re-set audio stream to %d", i)
					break
				}
			}
		}
	}

	e.seekFlushBefore.Store(math.Float64bits(seconds - 0.15))

	if e.videoCodecCtx != nil && e.videoDecoded {
		e.videoCodecMu.Lock()
		if _, sendExc := SafeSendPacket(e.videoCodecCtx, nil); sendExc != 0 {
			logging.Error(logging.CatPlayer, "Seek: flush send failed (exc=0x%08X)", sendExc)
			e.videoDecodeDead = true
			e.videoCodecMu.Unlock()
			return fmt.Errorf("flush send failed")
		}
		flushed := 0
		for {
			recvRet, recvExc := SafeReceiveFrame(e.videoCodecCtx, e.frame)
			if recvExc != 0 || recvRet != 0 {
				break
			}
			flushed++
		}
		C.avcodec_flush_buffers(e.videoCodecCtx)
		e.seekGen.Add(1)
		e.videoCodecMu.Unlock()
		logging.Info(logging.CatPlayer, "Seek: flushed %d frames", flushed)
	} else {
		logging.Info(logging.CatPlayer, "Seek: skipping video codec flush (no frames decoded yet), flushing audio codec")
	}

	if e.audioPlayer != nil {
		e.audioPlayer.SetSeekTarget(seconds)
		e.audioPlayer.FlushCodec()
		e.audioPlayer.ResetEOF()
	} else if e.audioCodecCtx != nil {
		C.avcodec_flush_buffers(e.audioCodecCtx)
	}
	logging.Info(logging.CatPlayer, "Seek: audio codec flushed, resetting clock")

	if e.audioPlayer != nil {
		e.clock.ResetTime(seconds - AudioBufferLatency.Seconds())
	} else {
		e.clock.ResetTime(seconds)
	}

	for {
		select {
		case <-e.frameQueue:
		default:
			goto drainDone
		}
	}
drainDone:
	e.decodeEOFSent = false

	logging.Info(logging.CatPlayer, "Seek: complete at %.2f", seconds)
	return nil
}

func (e *Engine) ResetAfterGrab() {
	defer func() {
		if r := recover(); r != nil {
			logging.Error(logging.CatPlayer, "ResetAfterGrab panic: %v", r)
		}
	}()

	logging.Info(logging.CatPlayer, "ResetAfterGrab: flushing queues and resetting clock")
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.formatCtx == nil {
		return
	}

	e.videoQueue.Flush()
	e.audioQueue.Flush()

	if e.audioPlayer != nil {
		e.audioPlayer.DrainPCM()
	}

	if e.videoCodecCtx != nil && e.videoDecoded {
		e.videoCodecMu.Lock()
		SafeSendPacket(e.videoCodecCtx, nil)
		for {
			ret, _ := SafeReceiveFrame(e.videoCodecCtx, e.frame)
			if ret != 0 {
				break
			}
		}
		C.avcodec_flush_buffers(e.videoCodecCtx)
		e.videoCodecMu.Unlock()
		logging.Info(logging.CatPlayer, "ResetAfterGrab: video codec flushed (B-frame drain)")
	}

	e.clock.ResetTime(0)
	e.clock.SetPaused(true)

	if e.audioPlayer != nil {
		e.audioPlayer.ResetLastPTS()
	}

	e.decodeEOFSent = false
	e.seekFlushBefore.Store(0)
	logging.Info(logging.CatPlayer, "ResetAfterGrab: done")
}

func (e *Engine) Step(frames int) (*image.RGBA, error) {
	if frames <= 0 {
		return nil, fmt.Errorf("invalid frame count")
	}

	var lastFrame *image.RGBA
	var err error
	for i := 0; i < frames; i++ {
		lastFrame, err = e.NextFrame()
		if err != nil {
			return nil, err
		}
	}
	return lastFrame, nil
}

func (e *Engine) GrabFrame(timeout time.Duration) (retImg *image.RGBA, retErr error) {
	defer func() {
		if r := recover(); r != nil {
			logging.Error(logging.CatPlayer, "GrabFrame panic: %v", r)
			retImg = nil
			retErr = fmt.Errorf("GrabFrame panic: %v", r)
		}
	}()

	deadline := time.Now().Add(timeout)
	logging.Info(logging.CatPlayer, "GrabFrame: waiting for first video frame (timeout=%v, hwDevice=%v)", timeout, e.hwDevice)

	for time.Now().Before(deadline) {
		pkt, ok := e.videoQueue.TryGet()
		if !ok {
			if e.videoQueue.IsClosedOrEOF() {
				logging.Info(logging.CatPlayer, "GrabFrame: video queue EOF/closed")
				return nil, io.EOF
			}
			time.Sleep(5 * time.Millisecond)
			continue
		}

		logging.Info(logging.CatPlayer, "GrabFrame: sending packet to video codec")
		e.videoCodecMu.Lock()
		sendRet, excCode := SafeSendPacket(e.videoCodecCtx, pkt)
		C.av_packet_free(&pkt)
		if excCode != 0 {
			e.videoCodecMu.Unlock()
			logging.Error(logging.CatPlayer, "GrabFrame: avcodec_send_packet SEH exception (exc=0x%08X) — disabling video decode", excCode)
			e.videoDecodeDead = true
			return nil, fmt.Errorf("video decode access violation: 0x%08X", excCode)
		}
		if sendRet != 0 {
			e.videoCodecMu.Unlock()
			logging.Info(logging.CatPlayer, "GrabFrame: avcodec_send_packet returned %d, skipping", int(sendRet))
			continue
		}

		logging.Info(logging.CatPlayer, "GrabFrame: packet sent OK, calling avcodec_receive_frame")
		for {
			recvRet, recvExc := SafeReceiveFrame(e.videoCodecCtx, e.frame)
			if recvExc != 0 {
				logging.Error(logging.CatPlayer, "GrabFrame: avcodec_receive_frame SEH exception (exc=0x%08X) — disabling video decode", recvExc)
				e.videoDecodeDead = true
				e.videoCodecMu.Unlock()
				return nil, fmt.Errorf("video decode access violation: 0x%08X", recvExc)
			}
			if recvRet != 0 {
				break
			}
			e.videoDecoded = true

			if e.frame.pts == C.AV_NOPTS_VALUE || e.frame.pts < 0 ||
				e.frame.width <= 0 || e.frame.height <= 0 {
				logging.Info(logging.CatPlayer, "GrabFrame: skipping invalid frame pts=%d w=%d h=%d", int64(e.frame.pts), int(e.frame.width), int(e.frame.height))
				continue
			}

			pts := float64(e.frame.pts) * e.videoTimeBase
			logging.Info(logging.CatPlayer, "GrabFrame: got frame pts=%.3f hw_frames_ctx=%v", pts, e.frame.hw_frames_ctx != nil)

			var img *image.RGBA
			if e.hwDevice != HWDeviceNone {
				var err error
				img, err = e.retrieveHWFrame()
				if err != nil {
					logging.Warning(logging.CatPlayer, "GrabFrame: HW retrieve failed (%v)", err)
					if e.frame.hw_frames_ctx != nil {
						logging.Info(logging.CatPlayer, "GrabFrame: frame is HW, cannot SW fallback — skipping")
						e.videoCodecMu.Unlock()
						continue
					}
					e.ensureSwsCtx(C.enum_AVPixelFormat(e.frame.format))
					img = e.toRGBA()
				}
			} else {
				e.ensureSwsCtx(C.enum_AVPixelFormat(e.frame.format))
				img = e.toRGBA()
			}
			e.videoCodecMu.Unlock()
			return img, nil
		}
		e.videoCodecMu.Unlock()
	}

	return nil, fmt.Errorf("timed out waiting for first video frame")
}

func (e *Engine) sendToFrameQueue(df decodedFrame) bool {
	pauseRetries := 0
	for {
		select {
		case e.frameQueue <- df:
			return true
		case <-e.decodeLoopStop:
			return false
		case <-time.After(5 * time.Millisecond):
			e.mu.Lock()
			paused := e.paused
			e.mu.Unlock()
			if paused {
				pauseRetries++
				if pauseRetries >= 3 {
					return true
				}
			} else {
				pauseRetries = 0
			}
		}
	}
}

func (e *Engine) videoDecodeLoop() {
	defer e.decodeLoopWg.Done()
	defer func() {
		if r := recover(); r != nil {
			logging.Error(logging.CatPlayer, "videoDecodeLoop panic: %v", r)
		}
	}()

	logging.Info(logging.CatPlayer, "videoDecodeLoop: started")

	for {
		select {
		case <-e.decodeLoopStop:
			logging.Info(logging.CatPlayer, "videoDecodeLoop: stopped")
			return
		default:
		}

		e.mu.Lock()
		paused := e.paused
		e.mu.Unlock()

		if paused {
			if len(e.frameQueue) >= 1 {
				time.Sleep(10 * time.Millisecond)
				continue
			}
		}

		rawPkt, ok := e.videoQueue.TryGet()
		if !ok {
			if e.videoQueue.IsClosedOrEOF() {
				e.mu.Lock()
				sent := e.decodeEOFSent
				e.mu.Unlock()
				if !sent {
					e.mu.Lock()
					e.decodeEOFSent = true
					e.mu.Unlock()
					e.sendToFrameQueue(decodedFrame{pts: decodeEOFPTS, gen: e.seekGen.Load()})
				}
			}
			time.Sleep(1 * time.Millisecond)
			continue
		}

		e.videoCodecMu.Lock()
		sendRet, excCode := SafeSendPacket(e.videoCodecCtx, rawPkt)
		C.av_packet_free(&rawPkt)
		if excCode != 0 {
			e.videoCodecMu.Unlock()
			logging.Error(logging.CatPlayer, "videoDecodeLoop: avcodec_send_packet SEH exception (exc=0x%08X) — stopping decode", excCode)
			e.videoDecodeDead = true
			return
		}
		if sendRet != 0 {
			e.videoCodecMu.Unlock()
			continue
		}

		for {
			recvRet, recvExc := SafeReceiveFrame(e.videoCodecCtx, e.frame)
			if recvExc != 0 {
				e.videoCodecMu.Unlock()
				logging.Error(logging.CatPlayer, "videoDecodeLoop: avcodec_receive_frame SEH exception (exc=0x%08X) — stopping decode", recvExc)
				e.videoDecodeDead = true
				return
			}
			if recvRet != 0 {
				break
			}
			e.videoDecoded = true

			if e.frame.pts == C.AV_NOPTS_VALUE || e.frame.pts < 0 {
				continue
			}

			pts := float64(e.frame.pts) * e.videoTimeBase

			flushBefore := math.Float64frombits(e.seekFlushBefore.Load())
			if flushBefore > 0 && pts < flushBefore {
				e.videoCodecMu.Unlock()
				runtime.Gosched()
				e.videoCodecMu.Lock()
				continue
			}
			if flushBefore > 0 {
				e.seekFlushBefore.Store(0)
			}

			var img *image.RGBA
			if e.hwDevice != HWDeviceNone {
				var err error
				img, err = e.retrieveHWFrame()
				if err != nil {
					if e.videoDecodeDead {
						e.videoCodecMu.Unlock()
						return
					}
					logging.Warning(logging.CatPlayer, "videoDecodeLoop: HW retrieve failed: %v", err)
					if e.frame.hw_frames_ctx != nil {
						e.videoCodecMu.Unlock()
						continue
					}
					e.ensureSwsCtx(C.enum_AVPixelFormat(e.frame.format))
					img = e.toRGBA()
				}
			} else {
				e.ensureSwsCtx(C.enum_AVPixelFormat(e.frame.format))
				img = e.toRGBA()
			}

			gen := e.seekGen.Load()
			e.videoCodecMu.Unlock()

			if !e.sendToFrameQueue(decodedFrame{img: img, pts: pts, gen: gen}) {
				return
			}

			e.videoCodecMu.Lock()
		}
		e.videoCodecMu.Unlock()
	}
}

func (e *Engine) NextFrame() (retImg *image.RGBA, retErr error) {
	defer func() {
		if r := recover(); r != nil {
			logging.Error(logging.CatPlayer, "NextFrame panic: %v", r)
			retImg = nil
			retErr = fmt.Errorf("NextFrame panic: %v", r)
		}
	}()

	nf := atomic.AddInt64(&e.nextFrameCount, 1)
	verbose := nf <= 20

	for {
		e.mu.Lock()
		paused := e.paused
		hasAudio := e.hasAudio
		e.mu.Unlock()

		var df decodedFrame
		select {
		case df = <-e.frameQueue:
		default:
			if paused {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			if hasAudio {
				e.clock.SetPaused(true)
			}
			df = <-e.frameQueue
			if hasAudio {
				e.clock.SetPaused(e.IsPaused())
			}
		}

		if df.pts == decodeEOFPTS {
			if e.IsLooping() {
				if err := e.Seek(0); err != nil {
					return nil, err
				}
				continue
			}
			return nil, io.EOF
		}

		pts := df.pts
		img := df.img

		if df.gen != e.seekGen.Load() {
			continue
		}

		if verbose {
			clockNow := e.clock.GetTime()
			logging.Info(logging.CatPlayer, "NextFrame #%d: pts=%.3f clockNow=%.3f", nf, pts, clockNow)
		}

		traceAction := "display"
		if hasAudio {
			e.clock.WaitForPTS(pts)
			if e.clock.GetTime()-pts >= MaxDriftThreshold {
				e.clock.ResetTime(pts)
				traceAction = "snap"
			}
		} else {
			e.clock.SetTime(pts)
		}

		clockNow := e.clock.GetTime()
		delay := e.clock.SyncVideo(pts)
		if delay < 0 {
			logging.Warning(logging.CatPlayer, "frame DROP #%d pts=%.3f clock=%.3f behind=%.0fms", nf, pts, clockNow, (clockNow-pts)*1000)
			logging.PlayerFrameTrace(nf, pts, clockNow, "drop", (clockNow-pts)*1000)
			continue
		}
		if delay == 0 && clockNow-pts > 0.010 {
			logging.Debug(logging.CatPlayer, "frame LATE #%d pts=%.3f clock=%.3f behind=%.0fms", nf, pts, clockNow, (clockNow-pts)*1000)
		}
		logging.PlayerFrameTrace(nf, pts, clockNow, traceAction, (clockNow-pts)*1000)

		e.lastVideoPTSBits.Store(math.Float64bits(pts))

		if e.subtitleCodecCtx != nil {
			sub := e.decodeSubtitle(pts)
			if sub != nil {
				img = e.RenderSubtitles(img, pts)
			}
		}

		return img, nil
	}
}

func (e *Engine) IsPaused() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.paused
}

func (e *Engine) Pause() {
	e.mu.Lock()
	if !e.running || e.paused {
		e.mu.Unlock()
		return
	}
	e.paused = true
	e.clock.SetPaused(true)
	e.mu.Unlock()

	if e.audioPlayer != nil {
		e.audioPlayer.Pause()
	}
}

func (e *Engine) DrainAudio() {
	if e.audioPlayer != nil {
		e.audioPlayer.DrainPCM()
	}
}

func (e *Engine) WaitForFrame(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if len(e.frameQueue) > 0 {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return len(e.frameQueue) > 0
}

func (e *Engine) Resume() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		logging.Info(logging.CatPlayer, "Engine.Resume: not running, returning")
		return
	}
	if !e.paused {
		e.mu.Unlock()
		logging.Info(logging.CatPlayer, "Engine.Resume: not paused, returning")
		return
	}
	e.paused = false
	e.clock.SetPaused(false)

	if !e.decodeLoopActive {
		e.decodeLoopActive = true
		e.decodeLoopWg.Add(1)
		go e.videoDecodeLoop()
	}
	e.mu.Unlock()

	if e.audioPlayer != nil {
		e.audioPlayer.Resume()
	}
	logging.Info(logging.CatPlayer, "Engine resumed")
}

func (e *Engine) TogglePause() {
	if e.IsPaused() {
		e.Resume()
	} else {
		e.Pause()
	}
}

func (e *Engine) Duration() float64 {
	e.formatMu.Lock()
	defer e.formatMu.Unlock()
	if e.formatCtx == nil {
		return 0
	}
	return float64(e.formatCtx.duration) / float64(C.AV_TIME_BASE)
}

func (e *Engine) CurrentTime() float64 {
	return e.clock.GetTime()
}

func (e *Engine) GetLastVideoPTS() float64 {
	bits := e.lastVideoPTSBits.Load()
	if bits == 0 {
		return -1
	}
	return math.Float64frombits(bits)
}

func (e *Engine) GetLastAudioPTS() float64 {
	if e.audioPlayer == nil {
		return -1
	}
	return e.audioPlayer.GetLastPTS()
}

func (e *Engine) Close() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return
	}
	e.running = false
	e.paused = false
	e.mu.Unlock()

	close(e.stop)

	e.videoQueue.Close()
	e.audioQueue.Close()
	if e.subtitleQueue != nil {
		e.subtitleQueue.Close()
	}

	close(e.decodeLoopStop)
	e.decodeLoopWg.Wait()

	for {
		select {
		case <-e.frameQueue:
		default:
			goto closeDrainDone
		}
	}
closeDrainDone:

	e.demuxerWg.Wait()

	if e.audioPlayer != nil {
		e.audioPlayer.Close()
		e.audioPlayer = nil
	}

	e.videoCodecMu.Lock()
	if e.swsCtx != nil {
		C.sws_freeContext(e.swsCtx)
		e.swsCtx = nil
		e.swsFmt = 0
	}
	if e.hwSwsCtx != nil {
		C.sws_freeContext(e.hwSwsCtx)
		e.hwSwsCtx = nil
		e.hwSwsFmt = 0
		e.hwSwsW = 0
		e.hwSwsH = 0
	}
	if e.videoCodecCtx != nil {
		if e.videoCodecCtx.hw_frames_ctx != nil {
			C.av_buffer_unref(&e.videoCodecCtx.hw_frames_ctx)
		}
		C.avcodec_free_context(&e.videoCodecCtx)
		e.videoCodecCtx = nil
	}
	if e.hwFramesCtx != nil {
		C.av_buffer_unref(&e.hwFramesCtx)
		e.hwFramesCtx = nil
	}
	if e.hwDeviceCtx != nil {
		C.av_buffer_unref(&e.hwDeviceCtx)
		e.hwDeviceCtx = nil
	}
	if e.frame != nil {
		C.av_frame_free(&e.frame)
		e.frame = nil
	}
	if e.rgbaFrame != nil {
		C.av_frame_free(&e.rgbaFrame)
		e.rgbaFrame = nil
	}
	if e.hwRgbaFrame != nil {
		C.av_frame_free(&e.hwRgbaFrame)
		e.hwRgbaFrame = nil
	}
	e.videoCodecMu.Unlock()

	if e.audioCodecCtx != nil {
		C.avcodec_free_context(&e.audioCodecCtx)
		e.audioCodecCtx = nil
	}
	if e.subtitleCodecCtx != nil {
		C.avcodec_free_context(&e.subtitleCodecCtx)
		e.subtitleCodecCtx = nil
	}

	if e.formatCtx != nil {
		C.avformat_close_input(&e.formatCtx)
		e.formatCtx = nil
	}

	logging.Info(logging.CatPlayer, "Engine closed")
}

func (e *Engine) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

func (e *Engine) QueueStats() (videoSize, audioSize int) {
	if e.videoQueue != nil {
		videoSize = e.videoQueue.Size()
	}
	if e.audioQueue != nil {
		audioSize = e.audioQueue.Size()
	}
	return
}
