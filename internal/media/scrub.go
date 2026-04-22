//go:build native_media

package media

/*
#cgo !windows pkg-config: libavcodec libavformat libswscale libavutil
#cgo windows CFLAGS: -IC:/ffmpeg/include
#cgo windows LDFLAGS: -LC:/ffmpeg/lib -lavcodec -lavformat -lavutil -lswscale
#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libswscale/swscale.h>
#include <libavutil/avutil.h>
#include <libavutil/imgutils.h>
#include <libavutil/hwcontext.h>
*/
import "C"
import (
	"fmt"
	"image"
	"sync"
	"unsafe"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

const (
	scrubPreDecodeFrames = 30
	seekPrerollFrames    = 10
)

type FrameCache struct {
	frames    map[int64]*image.RGBA
	frameNums []int64
	maxSize   int
	mu        sync.RWMutex
}

func NewFrameCache(maxSize int) *FrameCache {
	if maxSize <= 0 {
		maxSize = 60
	}
	return &FrameCache{
		frames:    make(map[int64]*image.RGBA),
		frameNums: make([]int64, 0, maxSize),
		maxSize:   maxSize,
	}
}

func (c *FrameCache) Add(pts int64, frame *image.RGBA) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.frames) >= c.maxSize {
		if len(c.frameNums) > 0 {
			oldest := c.frameNums[0]
			delete(c.frames, oldest)
			c.frameNums = c.frameNums[1:]
		}
	}

	c.frames[pts] = frame
	c.frameNums = append(c.frameNums, pts)
}

func (c *FrameCache) Get(pts int64) (*image.RGBA, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	frame, ok := c.frames[pts]
	return frame, ok
}

func (c *FrameCache) GetNearest(pts int64) (*image.RGBA, int64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var nearest int64
	var nearestFrame *image.RGBA
	minDiff := int64(^uint64(0) >> 1)

	for p, frame := range c.frames {
		diff := p - pts
		if diff < 0 {
			diff = -diff
		}
		if diff < minDiff {
			minDiff = diff
			nearest = p
			nearestFrame = frame
		}
	}

	return nearestFrame, nearest, nearestFrame != nil
}

func (c *FrameCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.frames = make(map[int64]*image.RGBA)
	c.frameNums = c.frameNums[:0]
}

func (c *FrameCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.frames)
}

// SmoothScrubbing pre-decodes frames around the current seek position so that
// scrubbing feels instant.
//
// Thread-safety notes
// ───────────────────
// predecodeFrom / predecodeAhead call FFmpeg APIs that share state with the
// engine's demuxer loop and NextFrame:
//
//   - av_read_frame on formatCtx races with demuxerLoop  → protected by engine.formatMu
//   - avcodec_send/receive on videoCodecCtx races with NextFrame → protected by engine.videoCodecMu
//
// Each SmoothScrubbing instance owns its own AVFrame (s.frame) so it never
// writes to engine.frame.  The RGBA conversion context (s.swsCtx / s.rgbaFrame /
// s.rgbaBuffer) is also private to avoid racing with engine.toRGBA().
//
// NOTE: predecodeAhead was removed in dev44. It received signals on decodeQueue
// but nothing ever sent to that channel, making it dead code.
// Wire it up if you want frame pre-caching during scrub.
type SmoothScrubbing struct {
	engine      *Engine
	frameCache  *FrameCache
	seekQueue   chan float64
	stop        chan struct{}
	predecoding bool
	seekTarget  float64
	onFrame     func(*image.RGBA)
	mu          sync.RWMutex
	wg          sync.WaitGroup

	fmtCtx   *C.AVFormatContext
	codecCtx *C.AVCodecContext
	videoIdx C.int

	frame      *C.AVFrame
	swsCtx     *C.struct_SwsContext
	rgbaFrame  *C.AVFrame
	rgbaBuffer []byte
	convertMu  sync.Mutex
}

func (s *SmoothScrubbing) SetOnFrame(cb func(*image.RGBA)) {
	s.mu.Lock()
	s.onFrame = cb
	s.mu.Unlock()
}

func NewSmoothScrubbing(engine *Engine) *SmoothScrubbing {
	s := &SmoothScrubbing{
		engine:     engine,
		frameCache: NewFrameCache(scrubPreDecodeFrames),
		seekQueue:  make(chan float64, 1),
		stop:       make(chan struct{}),
	}

	if err := s.openDecoder(); err != nil {
		logging.Warning(logging.CatPlayer, "SmoothScrubbing: openDecoder failed: %v", err)
	}

	s.Start()
	return s
}

func (s *SmoothScrubbing) openDecoder() error {
	s.engine.mu.Lock()
	path := s.engine.filePath
	s.engine.mu.Unlock()
	if path == "" {
		return fmt.Errorf("no file path")
	}

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	if C.avformat_open_input(&s.fmtCtx, cPath, nil, nil) != 0 {
		return fmt.Errorf("avformat_open_input failed")
	}

	if C.avformat_find_stream_info(s.fmtCtx, nil) < 0 {
		C.avformat_close_input(&s.fmtCtx)
		return fmt.Errorf("avformat_find_stream_info failed")
	}

	s.engine.mu.Lock()
	vidIdx := s.engine.videoStreamIdx
	s.engine.mu.Unlock()
	if vidIdx < 0 {
		C.avformat_close_input(&s.fmtCtx)
		return fmt.Errorf("no video stream")
	}
	s.videoIdx = C.int(vidIdx)

	streams := (*[1 << 30]*C.AVStream)(unsafe.Pointer(s.fmtCtx.streams))
	stream := streams[vidIdx]

	codec := C.avcodec_find_decoder(stream.codecpar.codec_id)
	if codec == nil {
		C.avformat_close_input(&s.fmtCtx)
		return fmt.Errorf("no decoder found")
	}

	s.codecCtx = C.avcodec_alloc_context3(codec)
	if s.codecCtx == nil {
		C.avformat_close_input(&s.fmtCtx)
		return fmt.Errorf("avcodec_alloc_context3 failed")
	}

	C.avcodec_parameters_to_context(s.codecCtx, stream.codecpar)
	s.codecCtx.thread_count = 2

	if C.avcodec_open2(s.codecCtx, codec, nil) < 0 {
		C.avcodec_free_context(&s.codecCtx)
		C.avformat_close_input(&s.fmtCtx)
		return fmt.Errorf("avcodec_open2 failed")
	}

	s.frame = C.av_frame_alloc()
	return nil
}

func (s *SmoothScrubbing) Start() {
	s.wg.Add(1)
	go func() { defer s.wg.Done(); s.seekHandler() }()
}

func (s *SmoothScrubbing) Stop() {
	close(s.stop)
	s.wg.Wait()

	s.convertMu.Lock()
	defer s.convertMu.Unlock()

	if s.frame != nil {
		C.av_frame_free(&s.frame)
		s.frame = nil
	}
	if s.rgbaFrame != nil {
		C.av_frame_free(&s.rgbaFrame)
		s.rgbaFrame = nil
	}
	if s.swsCtx != nil {
		C.sws_freeContext(s.swsCtx)
		s.swsCtx = nil
	}
	s.rgbaBuffer = nil
	if s.codecCtx != nil {
		C.avcodec_free_context(&s.codecCtx)
		s.codecCtx = nil
	}
	if s.fmtCtx != nil {
		C.avformat_close_input(&s.fmtCtx)
		s.fmtCtx = nil
	}
}

func (s *SmoothScrubbing) RequestSeek(target float64) {
	select {
	case s.seekQueue <- target:
	default:
	}
}

func (s *SmoothScrubbing) seekHandler() {
	for {
		select {
		case <-s.stop:
			return
		case target := <-s.seekQueue:
			s.handleSeek(target)
		}
	}
}

func (s *SmoothScrubbing) handleSeek(target float64) {
	s.mu.Lock()
	s.seekTarget = target
	s.predecoding = true
	s.frameCache.Clear()
	s.mu.Unlock()

	s.flushEngineQueues()
	s.seekOwnFormatCtx(target)

	s.mu.Lock()
	s.predecoding = false
	s.mu.Unlock()

	s.wg.Add(1)
	go func() { defer s.wg.Done(); s.predecodeFrom(target) }()
}

func (s *SmoothScrubbing) seekOwnFormatCtx(target float64) {
	if s.fmtCtx == nil || s.codecCtx == nil {
		return
	}

	s.engine.mu.Lock()
	timeBase := s.engine.videoTimeBase
	s.engine.mu.Unlock()

	pts := C.int64_t(target / timeBase)
	C.avformat_seek_file(s.fmtCtx, s.videoIdx, pts, pts, pts, 0)

	s.engine.videoCodecMu.Lock()
	C.avcodec_flush_buffers(s.codecCtx)
	s.engine.videoCodecMu.Unlock()
}

func (s *SmoothScrubbing) flushEngineQueues() {
	s.engine.mu.Lock()
	defer s.engine.mu.Unlock()
	s.engine.videoQueue.Flush()
	if s.engine.audioPlayer != nil {
		s.engine.audioPlayer.FlushCodec()
		s.engine.audioPlayer.ResetEOF()
	}
}

func (s *SmoothScrubbing) predecodeFrom(startTime float64) {
	if s.fmtCtx == nil || s.codecCtx == nil {
		return
	}

	s.mu.RLock()
	predecodeCount := seekPrerollFrames
	timeBase := s.engine.videoTimeBase
	s.mu.RUnlock()

	s.convertMu.Lock()
	frame := s.frame
	s.convertMu.Unlock()

	pkt := C.av_packet_alloc()
	defer C.av_packet_free(&pkt)

	framesDecoded := 0
	maxPTS := startTime + float64(predecodeCount)*timeBase*2
	firstFrame := true

	for framesDecoded < predecodeCount {
		select {
		case <-s.stop:
			return
		default:
		}

		if C.av_read_frame(s.fmtCtx, pkt) < 0 {
			break
		}

		if int(pkt.stream_index) != int(s.videoIdx) {
			C.av_packet_unref(pkt)
			continue
		}

		pts := float64(pkt.pts) * timeBase
		if pts > maxPTS {
			C.av_packet_unref(pkt)
			break
		}

		s.engine.videoCodecMu.Lock()
		if C.avcodec_send_packet(s.codecCtx, pkt) != 0 {
			s.engine.videoCodecMu.Unlock()
			C.av_packet_unref(pkt)
			continue
		}
		s.engine.videoCodecMu.Unlock()
		C.av_packet_unref(pkt)

		s.engine.videoCodecMu.Lock()
		for C.avcodec_receive_frame(s.codecCtx, frame) == 0 {
			pts = float64(frame.pts) * timeBase
			s.engine.videoCodecMu.Unlock()

			rgba := s.convertFrameToRGBA(frame)

			s.engine.videoCodecMu.Lock()

			if rgba != nil {
				s.frameCache.Add(int64(pts*1000), rgba)
				framesDecoded++
				if firstFrame {
					firstFrame = false
					s.mu.RLock()
					cb := s.onFrame
					s.mu.RUnlock()
					if cb != nil {
						s.engine.videoCodecMu.Unlock()
						cb(rgba)
						s.engine.videoCodecMu.Lock()
					}
				}
			}

			if pts > maxPTS {
				break
			}
		}
		s.engine.videoCodecMu.Unlock()
	}
}

// convertFrameToRGBA converts a decoded AVFrame to *image.RGBA using private
// conversion buffers (s.swsCtx / s.rgbaFrame / s.rgbaBuffer).  Lazy-initialises
// those buffers on first call.  Must NOT be called while holding videoCodecMu.
//
// For hardware-decoded frames (hw_frames_ctx != nil, e.g. D3D11VA), the frame
// is first transferred to CPU memory via av_hwframe_transfer_data before any
// sws conversion.  Calling sws_scale on a HW frame directly would read GPU
// texture memory as CPU memory and crash.
func (s *SmoothScrubbing) convertFrameToRGBA(frame *C.AVFrame) *image.RGBA {
	// Download HW frame to CPU before doing anything with pixel data.
	// This must happen outside convertMu because it can be slow.
	if frame.hw_frames_ctx != nil {
		swFrame := C.av_frame_alloc()
		if swFrame == nil {
			return nil
		}
		defer C.av_frame_free(&swFrame)
		if C.av_hwframe_transfer_data(swFrame, frame, 0) != 0 {
			return nil
		}
		frame = swFrame
	}

	s.convertMu.Lock()

	// Lazy-init: create our own sws context and output buffers.
	// We use frame.format (the actual SW pixel format) rather than
	// videoCodecCtx.pix_fmt, because for HW-decoded streams the codec ctx
	// reports a HW format (e.g. AV_PIX_FMT_D3D11) which sws cannot handle.
	// videoCodecCtx.width/height are set once at open and are safe to read here.
	if s.swsCtx == nil {
		w := s.engine.videoCodecCtx.width
		h := s.engine.videoCodecCtx.height
		pixFmt := C.enum_AVPixelFormat(frame.format)

		s.swsCtx = C.sws_getContext(
			w, h, pixFmt,
			w, h, C.AV_PIX_FMT_RGBA,
			C.SWS_BILINEAR, nil, nil, nil,
		)
		if s.swsCtx == nil {
			s.convertMu.Unlock()
			return nil
		}

		s.rgbaFrame = C.av_frame_alloc()
		if s.rgbaFrame == nil {
			C.sws_freeContext(s.swsCtx)
			s.swsCtx = nil
			s.convertMu.Unlock()
			return nil
		}

		numBytes := C.av_image_get_buffer_size(C.AV_PIX_FMT_RGBA, w, h, 1)
		s.rgbaBuffer = make([]byte, int(numBytes))
		C.av_image_fill_arrays(
			&s.rgbaFrame.data[0], &s.rgbaFrame.linesize[0],
			(*C.uint8_t)(unsafe.Pointer(&s.rgbaBuffer[0])),
			C.AV_PIX_FMT_RGBA, w, h, 1,
		)
	}

	swsCtx := s.swsCtx
	rgbaFrame := s.rgbaFrame
	rgbaBuffer := s.rgbaBuffer
	width := int(s.engine.videoCodecCtx.width)
	height := int(s.engine.videoCodecCtx.height)
	s.convertMu.Unlock()

	C.sws_scale(
		swsCtx,
		&frame.data[0],
		&frame.linesize[0],
		0,
		C.int(height),
		&rgbaFrame.data[0],
		&rgbaFrame.linesize[0],
	)

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	copy(img.Pix, rgbaBuffer)
	return img
}

func (s *SmoothScrubbing) GetCachedFrame(pts float64) (*image.RGBA, bool) {
	img, _, ok := s.frameCache.GetNearest(int64(pts * 1000))
	return img, ok
}

func (s *SmoothScrubbing) CacheSize() int {
	return s.frameCache.Size()
}

func (s *SmoothScrubbing) ClearCache() {
	s.frameCache.Clear()
}
