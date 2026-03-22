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
	"sync"
	"unsafe"
)

const (
	preDecodeFrames   = 30
	seekPrerollFrames = 10
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

type SmoothScrubbing struct {
	engine      *Engine
	frameCache  *FrameCache
	seekQueue   chan float64
	decodeQueue chan struct{}
	stop        chan struct{}
	predecoding bool
	seekTarget  float64
	onFrame     func(*image.RGBA)
	mu          sync.RWMutex
}

func (s *SmoothScrubbing) SetOnFrame(cb func(*image.RGBA)) {
	s.mu.Lock()
	s.onFrame = cb
	s.mu.Unlock()
}

func NewSmoothScrubbing(engine *Engine) *SmoothScrubbing {
	return &SmoothScrubbing{
		engine:      engine,
		frameCache:  NewFrameCache(preDecodeFrames),
		seekQueue:   make(chan float64, 1),
		decodeQueue: make(chan struct{}, preDecodeFrames),
		stop:        make(chan struct{}),
	}
}

func (s *SmoothScrubbing) Start() {
	go s.predecodeLoop()
	go s.seekHandler()
}

func (s *SmoothScrubbing) Stop() {
	close(s.stop)
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

	s.engine.Seek(target)

	s.mu.Lock()
	s.predecoding = false
	s.mu.Unlock()

	go s.predecodeFrom(target)
}

func (s *SmoothScrubbing) predecodeFrom(startTime float64) {
	s.mu.RLock()
	predecodeCount := seekPrerollFrames
	videoTimeBase := s.engine.videoTimeBase
	s.mu.RUnlock()

	frame := s.engine.frame
	if frame == nil {
		return
	}

	pkt := C.av_packet_alloc()
	defer C.av_packet_free(&pkt)

	framesDecoded := 0
	maxPTS := startTime + float64(predecodeCount)*videoTimeBase*2
	firstFrame := true

	for framesDecoded < predecodeCount {
		select {
		case <-s.stop:
			return
		default:
		}

		if C.av_read_frame(s.engine.formatCtx, pkt) < 0 {
			break
		}

		if int(pkt.stream_index) != s.engine.videoStreamIdx {
			C.av_packet_unref(pkt)
			continue
		}

		pts := float64(pkt.pts) * videoTimeBase
		if pts > maxPTS {
			C.av_packet_unref(pkt)
			break
		}

		if C.avcodec_send_packet(s.engine.videoCodecCtx, pkt) != 0 {
			C.av_packet_unref(pkt)
			continue
		}

		for C.avcodec_receive_frame(s.engine.videoCodecCtx, frame) == 0 {
			pts = float64(frame.pts) * videoTimeBase

			rgba := s.convertFrameToRGBA(frame)
			if rgba != nil {
				s.frameCache.Add(int64(pts*1000), rgba)
				framesDecoded++
				if firstFrame {
					firstFrame = false
					s.mu.RLock()
					cb := s.onFrame
					s.mu.RUnlock()
					if cb != nil {
						cb(rgba)
					}
				}
			}

			if pts > maxPTS {
				break
			}
		}

		C.av_packet_unref(pkt)
	}
}

func (s *SmoothScrubbing) predecodeLoop() {
	for {
		select {
		case <-s.stop:
			return
		case <-s.decodeQueue:
			s.predecodeAhead()
		}
	}
}

func (s *SmoothScrubbing) predecodeAhead() {
	s.mu.RLock()
	if s.predecoding {
		s.mu.RUnlock()
		return
	}
	videoTimeBase := s.engine.videoTimeBase
	s.mu.RUnlock()

	currentTime := s.engine.CurrentTime()
	maxPTS := currentTime + float64(preDecodeFrames)*videoTimeBase*2

	frame := s.engine.frame
	if frame == nil {
		return
	}

	pkt := C.av_packet_alloc()
	defer C.av_packet_free(&pkt)

	framesDecoded := 0

	for framesDecoded < 5 {
		select {
		case <-s.stop:
			return
		default:
		}

		if C.av_read_frame(s.engine.formatCtx, pkt) < 0 {
			break
		}

		if int(pkt.stream_index) != s.engine.videoStreamIdx {
			C.av_packet_unref(pkt)
			continue
		}

		pts := float64(pkt.pts) * videoTimeBase
		if pts < currentTime {
			C.av_packet_unref(pkt)
			continue
		}
		if pts > maxPTS {
			C.av_packet_unref(pkt)
			break
		}

		if C.avcodec_send_packet(s.engine.videoCodecCtx, pkt) != 0 {
			C.av_packet_unref(pkt)
			continue
		}

		for C.avcodec_receive_frame(s.engine.videoCodecCtx, frame) == 0 {
			pts = float64(frame.pts) * videoTimeBase

			rgba := s.convertFrameToRGBA(frame)
			if rgba != nil {
				s.frameCache.Add(int64(pts*1000), rgba)
				framesDecoded++
			}
		}

		C.av_packet_unref(pkt)
	}
}

func (s *SmoothScrubbing) convertFrameToRGBA(frame *C.AVFrame) *image.RGBA {
	if s.engine.swsCtx == nil {
		return nil
	}

	C.sws_scale(
		s.engine.swsCtx,
		&frame.data[0],
		&frame.linesize[0],
		0,
		C.int(s.engine.videoCodecCtx.height),
		s.engine.rgbaFrame.data[:],
		s.engine.rgbaFrame.linesize[:],
	)

	width := int(s.engine.videoCodecCtx.width)
	height := int(s.engine.videoCodecCtx.height)
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	linesize := int(s.engine.rgbaFrame.linesize[0])
	data := s.engine.rgbaBuffer

	for y := 0; y < height; y++ {
		row := data[y*linesize : y*linesize+width*4]
		for x := 0; x < width; x++ {
			img.Set(x, y, image.RGBA{
				R: row[x*4],
				G: row[x*4+1],
				B: row[x*4+2],
				A: row[x*4+3],
			})
		}
	}

	return img
}

func (s *SmoothScrubbing) GetCachedFrame(pts float64) (*image.RGBA, bool) {
	return s.frameCache.GetNearest(int64(pts * 1000))
}

func (s *SmoothScrubbing) CacheSize() int {
	return s.frameCache.Size()
}

func (s *SmoothScrubbing) ClearCache() {
	s.frameCache.Clear()
}
