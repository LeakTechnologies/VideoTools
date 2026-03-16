//go:build native_media

package media

/*
#cgo pkg-config: libavcodec libavformat libswscale libavutil
#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libswscale/swscale.h>
#include <libavutil/imgutils.h>
*/
import "C"
import (
	"fmt"
	"image"
	"io"
	"sync"
	"time"
	"unsafe"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

// Engine represents the native FFmpeg playback engine.
type Engine struct {
	formatCtx      *C.AVFormatContext
	videoStreamIdx int
	audioStreamIdx int
	videoCodecCtx  *C.AVCodecContext
	audioCodecCtx  *C.AVCodecContext
	swsCtx         *C.struct_SwsContext

	// Queues
	videoQueue *PacketQueue
	audioQueue *PacketQueue

	// Audio Player
	audioPlayer *AudioPlayer
	clock       *MasterClock

	// Buffers for decoding
	frame *C.AVFrame

	// Buffers for scaling
	rgbaFrame  *C.AVFrame
	rgbaBuffer []byte

	// Timing
	videoTimeBase float64
	audioTimeBase float64
	
	// State
	mu      sync.Mutex
	running bool
	stop    chan struct{}
}

// NewEngine creates a new media engine instance.
func NewEngine() *Engine {
	return &Engine{
		videoStreamIdx: -1,
		audioStreamIdx: -1,
		videoQueue:     NewPacketQueue(),
		audioQueue:     NewPacketQueue(),
		clock:          NewMasterClock(),
		stop:           make(chan struct{}),
	}
}

// Open probes a media file and initializes the necessary decoders.
func (e *Engine) Open(path string) error {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	logging.Info(logging.CatPlayer, "Opening media file: %s", path)

	if C.avformat_open_input(&e.formatCtx, cPath, nil, nil) != 0 {
		return fmt.Errorf("failed to open input file: %s", path)
	}

	if C.avformat_find_stream_info(e.formatCtx, nil) < 0 {
		return fmt.Errorf("failed to find stream info")
	}

	var videoCodec, audioCodec *C.AVCodec
	e.videoStreamIdx = int(C.av_find_best_stream(e.formatCtx, C.AVMEDIA_TYPE_VIDEO, -1, -1, &videoCodec, 0))
	e.audioStreamIdx = int(C.av_find_best_stream(e.formatCtx, C.AVMEDIA_TYPE_AUDIO, -1, -1, &audioCodec, 0))

	if e.videoStreamIdx < 0 {
		return fmt.Errorf("no video stream found")
	}

	streams := (*[1 << 30]*C.AVStream)(unsafe.Pointer(e.formatCtx.streams))

	// Setup Video
	e.videoCodecCtx = C.avcodec_alloc_context3(videoCodec)
	C.avcodec_parameters_to_context(e.videoCodecCtx, streams[e.videoStreamIdx].codecpar)
	if C.avcodec_open2(e.videoCodecCtx, videoCodec, nil) < 0 {
		return fmt.Errorf("failed to open video codec")
	}
	e.videoTimeBase = float64(streams[e.videoStreamIdx].time_base.num) / float64(streams[e.videoStreamIdx].time_base.den)

	// Setup Audio
	if e.audioStreamIdx >= 0 {
		e.audioCodecCtx = C.avcodec_alloc_context3(audioCodec)
		C.avcodec_parameters_to_context(e.audioCodecCtx, streams[e.audioStreamIdx].codecpar)
		if C.avcodec_open2(e.audioCodecCtx, audioCodec, nil) < 0 {
			logging.Error(logging.CatPlayer, "Failed to open audio codec")
			e.audioStreamIdx = -1
		} else {
			e.audioTimeBase = float64(streams[e.audioStreamIdx].time_base.num) / float64(streams[e.audioStreamIdx].time_base.den)
			ap, err := NewAudioPlayer(e.audioCodecCtx, e.audioQueue, e.clock, e.audioTimeBase)
			if err != nil {
				logging.Error(logging.CatPlayer, "Failed to create audio player: %v", err)
				e.audioStreamIdx = -1
			} else {
				e.audioPlayer = ap
			}
		}
	}

	e.frame = C.av_frame_alloc()

	e.swsCtx = C.sws_getContext(
		e.videoCodecCtx.width, e.videoCodecCtx.height, e.videoCodecCtx.pix_fmt,
		e.videoCodecCtx.width, e.videoCodecCtx.height, C.AV_PIX_FMT_RGBA,
		C.SWS_BILINEAR, nil, nil, nil,
	)

	e.rgbaFrame = C.av_frame_alloc()
	numBytes := C.av_image_get_buffer_size(C.AV_PIX_FMT_RGBA, e.videoCodecCtx.width, e.videoCodecCtx.height, 1)
	e.rgbaBuffer = make([]byte, int(numBytes))
	C.av_image_fill_arrays(
		&e.rgbaFrame.data[0], &e.rgbaFrame.linesize[0],
		(*C.uint8_t)(unsafe.Pointer(&e.rgbaBuffer[0])), C.AV_PIX_FMT_RGBA,
		e.videoCodecCtx.width, e.videoCodecCtx.height, 1,
	)

	return nil
}

// Start launches the playback.
func (e *Engine) Start() {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
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

// Seek jumps to a specific time in seconds.
func (e *Engine) Seek(seconds float64) error {
	logging.Info(logging.CatPlayer, "Seeking to %.2f seconds", seconds)
	
	e.mu.Lock()
	defer e.mu.Unlock()

	target := C.int64_t(seconds / e.videoTimeBase)
	
	if C.avformat_seek_file(e.formatCtx, C.int(e.videoStreamIdx), target, target, target, C.AVSEEK_FLAG_FRAME) < 0 {
		return fmt.Errorf("seek failed")
	}

	// Flush queues
	e.videoQueue.Flush()
	e.audioQueue.Flush()

	// Flush decoders
	if e.videoCodecCtx != nil {
		C.avcodec_flush_buffers(e.videoCodecCtx)
	}
	if e.audioCodecCtx != nil {
		C.avcodec_flush_buffers(e.audioCodecCtx)
	}

	e.clock.SetTime(seconds)
	return nil
}

// Step advances the video by a specific number of frames.
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

// NextFrame retrieves the next decoded video frame.
func (e *Engine) NextFrame() (*image.RGBA, error) {
	for {
		pkt, ok := e.videoQueue.Get()
		if !ok {
			return nil, io.EOF
		}
		defer C.av_packet_free(&pkt)

		if C.avcodec_send_packet(e.videoCodecCtx, pkt) == 0 {
			for C.avcodec_receive_frame(e.videoCodecCtx, e.frame) == 0 {
				pts := float64(e.frame.pts) * e.videoTimeBase
				
				delay := e.clock.SyncVideo(pts)
				if delay > 0 {
					time.Sleep(delay)
				}
				
				return e.toRGBA(), nil
			}
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

// Close stops the engine and releases all resources.
func (e *Engine) Close() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return
	}
	close(e.stop)
	e.running = false
	e.mu.Unlock()

	e.videoQueue.Close()
	e.audioQueue.Close()

	if e.audioPlayer != nil {
		e.audioPlayer.Close()
	}

	if e.swsCtx != nil {
		C.sws_freeContext(e.swsCtx)
	}
	if e.videoCodecCtx != nil {
		C.avcodec_free_context(&e.videoCodecCtx)
	}
	if e.audioCodecCtx != nil {
		C.avcodec_free_context(&e.audioCodecCtx)
	}
	if e.formatCtx != nil {
		C.avformat_close_input(&e.formatCtx)
	}
	if e.frame != nil {
		C.av_frame_free(&e.frame)
	}
	if e.rgbaFrame != nil {
		C.av_frame_free(&e.rgbaFrame)
	}
}
