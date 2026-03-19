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
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"
)

const (
	thumbnailWidth    = 160
	thumbnailHeight   = 90
	thumbnailInterval = 10.0
	maxThumbnails     = 100
)

type ThumbnailExtractor struct {
	formatCtx  *C.AVFormatContext
	width      int
	height     int
	timeBase   float64
	swsCtx     *C.struct_SwsContext
	rgbaBuffer []byte
	thumbnails map[int64]string
	tempDir    string
	mu         sync.Mutex
	stop       chan struct{}
}

func NewThumbnailExtractor(path string, tempDir string) (*ThumbnailExtractor, error) {
	e := &ThumbnailExtractor{
		thumbnails: make(map[int64]string),
		tempDir:    tempDir,
		stop:       make(chan struct{}),
	}

	if err := e.open(path); err != nil {
		return nil, err
	}

	return e, nil
}

func (e *ThumbnailExtractor) open(path string) error {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	if C.avformat_open_input(&e.formatCtx, cPath, nil, nil) != 0 {
		return fmt.Errorf("failed to open input file")
	}

	if C.avformat_find_stream_info(e.formatCtx, nil) < 0 {
		C.avformat_close_input(&e.formatCtx)
		return fmt.Errorf("failed to find stream info")
	}

	videoStreamIdx := C.int(-1)
	var videoCodec *C.AVCodec
	for i := 0; i < int(e.formatCtx.nb_streams); i++ {
		stream := *(**C.AVStream)(unsafe.Pointer(uintptr(unsafe.Pointer(e.formatCtx.streams)) + uintptr(i)*unsafe.Sizeof(uintptr(0))))
		if stream == nil {
			continue
		}
		if stream.codecpar.codec_type == C.AVMEDIA_TYPE_VIDEO {
			videoStreamIdx = C.int(i)
			videoCodec = C.avcodec_find_decoder(stream.codecpar.codec_id)
			e.width = int(stream.codecpar.width)
			e.height = int(stream.codecpar.height)
			e.timeBase = float64(stream.time_base.num) / float64(stream.time_base.den)
			break
		}
	}

	if videoStreamIdx < 0 {
		C.avformat_close_input(&e.formatCtx)
		return fmt.Errorf("no video stream found")
	}

	videoStream := *(**C.AVStream)(unsafe.Pointer(uintptr(unsafe.Pointer(e.formatCtx.streams)) + uintptr(videoStreamIdx)*unsafe.Sizeof(uintptr(0))))
	videoCodecCtx := C.avcodec_alloc_context3(videoCodec)
	if videoCodecCtx == nil {
		C.avformat_close_input(&e.formatCtx)
		return fmt.Errorf("failed to allocate codec context")
	}
	C.avcodec_parameters_to_context(videoCodecCtx, videoStream.codecpar)

	if C.avcodec_open2(videoCodecCtx, videoCodec, nil) < 0 {
		C.avcodec_free_context(&videoCodecCtx)
		C.avformat_close_input(&e.formatCtx)
		return fmt.Errorf("failed to open codec")
	}

	e.swsCtx = C.sws_getContext(
		C.int(e.width), C.int(e.height), videoCodecCtx.pix_fmt,
		C.int(e.width), C.int(e.height), C.AV_PIX_FMT_RGBA,
		C.SWS_BILINEAR, nil, nil, nil,
	)

	if e.swsCtx == nil {
		C.avcodec_free_context(&videoCodecCtx)
		C.avformat_close_input(&e.formatCtx)
		return fmt.Errorf("failed to create swscale context")
	}

	numBytes := C.av_image_get_buffer_size(C.AV_PIX_FMT_RGBA, C.int(e.width), C.int(e.height), 1)
	e.rgbaBuffer = make([]byte, int(numBytes))

	frame := C.av_frame_alloc()
	defer C.av_frame_free(&frame)

	C.avcodec_free_context(&videoCodecCtx)

	return nil
}

func (e *ThumbnailExtractor) ExtractAt(timestamp float64) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	pts := int64(timestamp / e.timeBase)

	if path, ok := e.thumbnails[pts]; ok {
		return path, nil
	}

	img, err := e.extractFrame(timestamp)
	if err != nil {
		return "", err
	}

	path, err := e.saveThumbnail(img, pts)
	if err != nil {
		return "", err
	}

	if len(e.thumbnails) >= maxThumbnails {
		e.cleanupOldest()
	}

	e.thumbnails[pts] = path
	return path, nil
}

func (e *ThumbnailExtractor) extractFrame(timestamp float64) (*image.RGBA, error) {
	C.av_seek_file(e.formatCtx, -1, 0, C.int64_t(timestamp/e.timeBase), 0, 0)

	pkt := C.av_packet_alloc()
	defer C.av_packet_free(&pkt)

	frame := C.av_frame_alloc()
	if frame == nil {
		return nil, fmt.Errorf("failed to allocate frame")
	}
	defer C.av_frame_free(&frame)

	rgbaFrame := C.av_frame_alloc()
	if rgbaFrame == nil {
		return nil, fmt.Errorf("failed to allocate rgba frame")
	}
	defer C.av_frame_free(&rgbaFrame)

	C.av_image_fill_arrays(
		&rgbaFrame.data[0], &rgbaFrame.linesize[0],
		(*C.uint8_t)(unsafe.Pointer(&e.rgbaBuffer[0])),
		C.AV_PIX_FMT_RGBA,
		C.int(e.width), C.int(e.height), 1,
	)

	for {
		if C.av_read_frame(e.formatCtx, pkt) < 0 {
			break
		}
		defer C.av_packet_unref(pkt)

		if C.avcodec_send_packet(C.avcodec_alloc_context3(nil), pkt) != 0 {
			continue
		}

		for C.avcodec_receive_frame(C.avcodec_alloc_context3(nil), frame) == 0 {
			C.sws_scale(
				e.swsCtx,
				&frame.data[0],
				&frame.linesize[0],
				0,
				C.int(e.height),
				&rgbaFrame.data[0],
				&rgbaFrame.linesize[0],
			)

			img := image.NewRGBA(image.Rect(0, 0, e.width, e.height))
			for y := 0; y < e.height; y++ {
				row := e.rgbaBuffer[y*rgbaFrame.linesize[0] : y*rgbaFrame.linesize[0]+e.width*4]
				for x := 0; x < e.width; x++ {
					img.Set(x, y, image.RGBA{
						R: row[x*4],
						G: row[x*4+1],
						B: row[x*4+2],
						A: row[x*4+3],
					})
				}
			}

			return img, nil
		}
	}

	return nil, fmt.Errorf("failed to extract frame")
}

func (e *ThumbnailExtractor) saveThumbnail(img *image.RGBA, pts int64) (string, error) {
	if e.tempDir == "" {
		e.tempDir = os.TempDir()
	}

	filename := filepath.Join(e.tempDir, fmt.Sprintf("vtt_thumb_%d.png", pts))
	file, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	return filename, png.Encode(file, img)
}

func (e *ThumbnailExtractor) cleanupOldest() {
	var oldest int64
	var oldestPath string
	for pts, path := range e.thumbnails {
		if oldest == 0 || pts < oldest {
			oldest = pts
			oldestPath = path
		}
	}
	if oldestPath != "" {
		os.Remove(oldestPath)
		delete(e.thumbnails, oldest)
	}
}

func (e *ThumbnailExtractor) GetNearest(timestamp float64) (int64, string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	var nearest int64
	var nearestPath string
	minDiff := int64(^uint64(0) >> 1)

	for pts, path := range e.thumbnails {
		diff := pts - int64(timestamp)
		if diff < 0 {
			diff = -diff
		}
		if diff < minDiff {
			minDiff = diff
			nearest = pts
			nearestPath = path
		}
	}

	return nearest, nearestPath, nearestPath != ""
}

func (e *ThumbnailExtractor) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()

	select {
	case <-e.stop:
	default:
		close(e.stop)
	}

	for _, path := range e.thumbnails {
		os.Remove(path)
	}
	e.thumbnails = nil

	if e.swsCtx != nil {
		C.sws_freeContext(e.swsCtx)
		e.swsCtx = nil
	}

	if e.formatCtx != nil {
		C.avformat_close_input(&e.formatCtx)
		e.formatCtx = nil
	}
}

func (e *ThumbnailExtractor) ExtractAll(duration float64, onProgress func(float64)) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	interval := thumbnailInterval
	if interval < 1 {
		interval = 1
	}

	for t := 0.0; t < duration; t += interval {
		select {
		case <-e.stop:
			return fmt.Errorf("extraction stopped")
		default:
		}

		if _, err := e.extractFrame(t); err != nil {
			continue
		}

		if onProgress != nil {
			onProgress(t / duration)
		}

		time.Sleep(10 * time.Millisecond)
	}

	return nil
}
