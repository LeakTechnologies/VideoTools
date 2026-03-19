//go:build native_media

package media

/*
#cgo pkg-config: libavcodec libavformat libswscale libavutil
#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libswscale/swscale.h>
#include <libavutil/imgutils.h>
#include <libavutil/hwcontext.h>
#include <libavutil/dict.h>
*/
import "C"
import (
	"fmt"
	"image"
	"io"
	"math"
	"sync"
	"time"
	"unsafe"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

type SeekAccuracy int

const (
	SeekAccuracyFrame SeekAccuracy = iota
	SeekAccuracyKeyframe
	SeekAccuracyAccurate
)

type SeekFlags int

const (
	AVSEEK_FLAG_FRAME    = 0x01
	AVSEEK_FLAG_BACKWARD = 0x02
	AVSEEK_FLAG_ANY      = 0x04
	AVSEEK_FLAG_ACCURATE = 0x08
)

type VideoInfo struct {
	Width        int
	Height       int
	FrameRate    float64
	Duration     float64
	CodecName    string
	PixelFormat  string
	Bitrate      int64
	HasAudio     bool
	HasVideo     bool
	HasSubtitles bool
}

type Engine struct {
	formatCtx         *C.AVFormatContext
	videoStreamIdx    int
	audioStreamIdx    int
	subtitleStreamIdx int
	videoCodecCtx     *C.AVCodecContext
	audioCodecCtx     *C.AVCodecContext
	swsCtx            *C.struct_SwsContext

	videoQueue *PacketQueue
	audioQueue *PacketQueue

	audioPlayer *AudioPlayer
	clock       *MasterClock

	frame *C.AVFrame

	rgbaFrame  *C.AVFrame
	rgbaBuffer []byte

	videoTimeBase float64
	audioTimeBase float64

	mu      sync.Mutex
	running bool
	paused  bool
	stop    chan struct{}

	volume  float32
	muted   bool
	speed   float64
	seekAcc SeekAccuracy

	info *VideoInfo
}

func NewEngine() *Engine {
	return &Engine{
		videoStreamIdx:    -1,
		audioStreamIdx:    -1,
		subtitleStreamIdx: -1,
		videoQueue:        NewPacketQueue(),
		audioQueue:        NewPacketQueue(),
		clock:             NewMasterClock(),
		stop:              make(chan struct{}),
		volume:            1.0,
		speed:             1.0,
		seekAcc:           SeekAccuracyKeyframe,
	}
}

func (e *Engine) SetVolume(vol float32) {
	if vol < 0 {
		vol = 0
	}
	if vol > 1 {
		vol = 1
	}
	e.volume = vol
	if e.audioPlayer != nil {
		e.audioPlayer.SetVolume(vol)
	}
}

func (e *Engine) GetVolume() float32 {
	return e.volume
}

func (e *Engine) SetMuted(muted bool) {
	e.muted = muted
	if e.audioPlayer != nil {
		e.audioPlayer.SetMuted(muted)
	}
}

func (e *Engine) IsMuted() bool {
	return e.muted
}

func (e *Engine) SetSpeed(speed float64) {
	if speed <= 0 {
		speed = 0.1
	}
	if speed > 4 {
		speed = 4
	}
	e.speed = speed
}

func (e *Engine) GetSpeed() float64 {
	return e.speed
}

func (e *Engine) SetSeekAccuracy(acc SeekAccuracy) {
	e.seekAcc = acc
}

func (e *Engine) GetSeekAccuracy() SeekAccuracy {
	return e.seekAcc
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
	logging.Info(logging.CatPlayer, "Engine paused")
}

func (e *Engine) Resume() {
	e.mu.Lock()
	if !e.running || !e.paused {
		e.mu.Unlock()
		return
	}
	e.paused = false
	e.clock.SetPaused(false)
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

func (e *Engine) Info() *VideoInfo {
	return e.info
}

func (e *Engine) Open(path string) error {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	logging.Info(logging.CatPlayer, "Opening media file: %s", path)

	if C.avformat_open_input(&e.formatCtx, cPath, nil, nil) != 0 {
		return fmt.Errorf("failed to open input file: %s", path)
	}

	if C.avformat_find_stream_info(e.formatCtx, nil) < 0 {
		C.avformat_close_input(&e.formatCtx)
		return fmt.Errorf("failed to find stream info")
	}

	var videoCodec, audioCodec, subtitleCodec *C.AVCodec
	e.videoStreamIdx = int(C.av_find_best_stream(e.formatCtx, C.AVMEDIA_TYPE_VIDEO, -1, -1, &videoCodec, 0))
	e.audioStreamIdx = int(C.av_find_best_stream(e.formatCtx, C.AVMEDIA_TYPE_AUDIO, -1, -1, &audioCodec, 0))
	e.subtitleStreamIdx = int(C.av_find_best_stream(e.formatCtx, C.AVMEDIA_TYPE_SUBTITLE, -1, -1, &subtitleCodec, 0))

	streams := (*[1 << 30]*C.AVStream)(unsafe.Pointer(e.formatCtx.streams))

	e.info = &VideoInfo{
		Duration:     float64(e.formatCtx.duration) / float64(C.AV_TIME_BASE),
		Bitrate:      int64(e.formatCtx.bit_rate),
		HasVideo:     e.videoStreamIdx >= 0,
		HasAudio:     e.audioStreamIdx >= 0,
		HasSubtitles: e.subtitleStreamIdx >= 0,
	}

	if e.videoStreamIdx < 0 {
		C.avformat_close_input(&e.formatCtx)
		return fmt.Errorf("no video stream found")
	}

	e.videoCodecCtx = C.avcodec_alloc_context3(videoCodec)
	if e.videoCodecCtx == nil {
		C.avformat_close_input(&e.formatCtx)
		return fmt.Errorf("failed to allocate video codec context")
	}
	C.avcodec_parameters_to_context(e.videoCodecCtx, streams[e.videoStreamIdx].codecpar)

	e.videoTimeBase = float64(streams[e.videoStreamIdx].time_base.num) / float64(streams[e.videoStreamIdx].time_base.den)

	e.info.Width = int(e.videoCodecCtx.width)
	e.info.Height = int(e.videoCodecCtx.height)
	e.info.CodecName = C.GoString((*C.char)(unsafe.Pointer(e.videoCodecCtx.codec.name)))
	e.info.PixelFormat = C.GoString((*C.char)(unsafe.Pointer(av_get_pix_fmt_name(e.videoCodecCtx.pix_fmt))))

	avgFrameRate := streams[e.videoStreamIdx].avg_frame_rate
	if avgFrameRate.num > 0 {
		e.info.FrameRate = float64(avgFrameRate.num) / float64(avgFrameRate.den)
	}

	if C.avcodec_open2(e.videoCodecCtx, videoCodec, nil) < 0 {
		C.avcodec_free_context(&e.videoCodecCtx)
		C.avformat_close_input(&e.formatCtx)
		return fmt.Errorf("failed to open video codec")
	}

	if e.audioStreamIdx >= 0 {
		e.audioCodecCtx = C.avcodec_alloc_context3(audioCodec)
		if e.audioCodecCtx != nil {
			C.avcodec_parameters_to_context(e.audioCodecCtx, streams[e.audioStreamIdx].codecpar)
			e.audioTimeBase = float64(streams[e.audioStreamIdx].time_base.num) / float64(streams[e.audioStreamIdx].time_base.den)
			if C.avcodec_open2(e.audioCodecCtx, audioCodec, nil) < 0 {
				logging.Warning(logging.CatPlayer, "Failed to open audio codec")
				C.avcodec_free_context(&e.audioCodecCtx)
				e.audioCodecCtx = nil
				e.audioStreamIdx = -1
				e.info.HasAudio = false
			} else {
				ap, err := NewAudioPlayer(e.audioCodecCtx, e.audioQueue, e.clock, e.audioTimeBase)
				if err != nil {
					logging.Error(logging.CatPlayer, "Failed to create audio player: %v", err)
					C.avcodec_free_context(&e.audioCodecCtx)
					e.audioCodecCtx = nil
					e.audioStreamIdx = -1
					e.info.HasAudio = false
				} else {
					e.audioPlayer = ap
					e.audioPlayer.SetVolume(e.volume)
					e.audioPlayer.SetMuted(e.muted)
				}
			}
		}
	}

	e.frame = C.av_frame_alloc()
	if e.frame == nil {
		e.Close()
		return fmt.Errorf("failed to allocate frame")
	}

	e.swsCtx = C.sws_getContext(
		e.videoCodecCtx.width, e.videoCodecCtx.height, e.videoCodecCtx.pix_fmt,
		e.videoCodecCtx.width, e.videoCodecCtx.height, C.AV_PIX_FMT_RGBA,
		C.SWS_BILINEAR, nil, nil, nil,
	)

	if e.swsCtx == nil {
		e.Close()
		return fmt.Errorf("failed to create sws context")
	}

	e.rgbaFrame = C.av_frame_alloc()
	numBytes := C.av_image_get_buffer_size(C.AV_PIX_FMT_RGBA, e.videoCodecCtx.width, e.videoCodecCtx.height, 1)
	e.rgbaBuffer = make([]byte, int(numBytes))
	C.av_image_fill_arrays(
		&e.rgbaFrame.data[0], &e.rgbaFrame.linesize[0],
		(*C.uint8_t)(unsafe.Pointer(&e.rgbaBuffer[0])), C.AV_PIX_FMT_RGBA,
		e.videoCodecCtx.width, e.videoCodecCtx.height, 1,
	)

	logging.Info(logging.CatPlayer, "Media opened: %dx%d @ %.2ffps, duration: %.2fs",
		e.info.Width, e.info.Height, e.info.FrameRate, e.info.Duration)

	return nil
}

func av_get_pix_fmt_name(fmt C.enum_AVPixelFormat) *C.char {
	return C.av_get_pix_fmt_name(fmt)
}

func (e *Engine) Start() {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.paused = false
	e.mu.Unlock()

	go e.demuxerLoop()
}

func (e *Engine) demuxerLoop() {
	pkt := C.av_packet_alloc()
	defer C.av_packet_free(&pkt)

	for {
		select {
		case <-e.stop:
			return
		default:
			if C.av_read_frame(e.formatCtx, pkt) < 0 {
				e.videoQueue.Close()
				e.audioQueue.Close()
				return
			}

			if int(pkt.stream_index) == e.videoStreamIdx {
				e.videoQueue.Put(pkt)
			} else if int(pkt.stream_index) == e.audioStreamIdx {
				e.audioQueue.Put(pkt)
			}
			C.av_packet_unref(pkt)
		}
	}
}

func (e *Engine) Seek(seconds float64) error {
	logging.Info(logging.CatPlayer, "Seeking to %.2f seconds (accuracy: %v)", seconds, e.seekAcc)

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.formatCtx == nil || e.videoStreamIdx < 0 {
		return fmt.Errorf("no media opened")
	}

	target := C.int64_t(seconds / e.videoTimeBase)

	var flags C.int
	switch e.seekAcc {
	case SeekAccuracyFrame:
		flags = C.int(AVSEEK_FLAG_FRAME)
	case SeekAccuracyKeyframe:
		flags = 0
	case SeekAccuracyAccurate:
		flags = C.int(AVSEEK_FLAG_BACKWARD | AVSEEK_FLAG_ACCURATE)
	}

	if C.avformat_seek_file(e.formatCtx, C.int(e.videoStreamIdx), target, target, target, flags) < 0 {
		return fmt.Errorf("seek failed")
	}

	e.videoQueue.Flush()
	e.audioQueue.Flush()

	if e.videoCodecCtx != nil {
		C.avcodec_flush_buffers(e.videoCodecCtx)
	}
	if e.audioCodecCtx != nil {
		C.avcodec_flush_buffers(e.audioCodecCtx)
	}

	e.clock.SetTime(seconds)
	return nil
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

func (e *Engine) NextFrame() (*image.RGBA, error) {
	for {
		e.mu.Lock()
		paused := e.paused
		e.mu.Unlock()

		if paused {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		pkt, ok := e.videoQueue.Get()
		if !ok {
			return nil, io.EOF
		}
		defer C.av_packet_free(&pkt)

		if C.avcodec_send_packet(e.videoCodecCtx, pkt) != 0 {
			logging.Debug(logging.CatPlayer, "Failed to send packet to video decoder")
			continue
		}

		for C.avcodec_receive_frame(e.videoCodecCtx, e.frame) == 0 {
			pts := float64(e.frame.pts) * e.videoTimeBase

			adjustedPts := pts * e.speed
			delay := e.clock.SyncVideo(adjustedPts)
			if delay > 0 {
				time.Sleep(delay)
			}

			return e.toRGBA(), nil
		}
	}
}

func (e *Engine) toRGBA() *image.RGBA {
	C.sws_scale(
		e.swsCtx,
		&e.frame.data[0], &e.frame.linesize[0],
		0, e.videoCodecCtx.height,
		&e.rgbaFrame.data[0], &e.rgbaFrame.linesize[0],
	)

	w, h := int(e.videoCodecCtx.width), int(e.videoCodecCtx.height)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	copy(img.Pix, e.rgbaBuffer)
	return img
}

func (e *Engine) Duration() float64 {
	if e.formatCtx == nil {
		return 0
	}
	return float64(e.formatCtx.duration) / float64(C.AV_TIME_BASE)
}

func (e *Engine) CurrentTime() float64 {
	return e.clock.GetTime()
}

func (e *Engine) Close() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return
	}
	close(e.stop)
	e.running = false
	e.paused = false
	e.mu.Unlock()

	e.videoQueue.Close()
	e.audioQueue.Close()

	if e.audioPlayer != nil {
		e.audioPlayer.Close()
		e.audioPlayer = nil
	}

	if e.swsCtx != nil {
		C.sws_freeContext(e.swsCtx)
		e.swsCtx = nil
	}
	if e.videoCodecCtx != nil {
		C.avcodec_free_context(&e.videoCodecCtx)
		e.videoCodecCtx = nil
	}
	if e.audioCodecCtx != nil {
		C.avcodec_free_context(&e.audioCodecCtx)
		e.audioCodecCtx = nil
	}
	if e.formatCtx != nil {
		C.avformat_close_input(&e.formatCtx)
		e.formatCtx = nil
	}
	if e.frame != nil {
		C.av_frame_free(&e.frame)
		e.frame = nil
	}
	if e.rgbaFrame != nil {
		C.av_frame_free(&e.rgbaFrame)
		e.rgbaFrame = nil
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

func round(x float64) float64 {
	return math.Round(x)
}
