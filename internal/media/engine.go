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
	"unsafe"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

// Engine represents the native FFmpeg playback engine.
type Engine struct {
	formatCtx      *C.AVFormatContext
	videoStreamIdx int
	videoCodecCtx  *C.AVCodecContext
	swsCtx         *C.struct_SwsContext

	// Buffers for decoding
	packet *C.AVPacket
	frame  *C.AVFrame

	// Buffers for scaling
	rgbaFrame  *C.AVFrame
	rgbaBuffer []byte
}

// NewEngine creates a new media engine instance.
func NewEngine() *Engine {
	return &Engine{
		videoStreamIdx: -1,
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

	var videoCodec *C.AVCodec
	e.videoStreamIdx = int(C.av_find_best_stream(e.formatCtx, C.AVMEDIA_TYPE_VIDEO, -1, -1, &videoCodec, 0))
	if e.videoStreamIdx < 0 {
		return fmt.Errorf("no video stream found")
	}

	e.videoCodecCtx = C.avcodec_alloc_context3(videoCodec)
	if e.videoCodecCtx == nil {
		return fmt.Errorf("failed to allocate codec context")
	}

	streams := (*[1 << 30]*C.AVStream)(unsafe.Pointer(e.formatCtx.streams))
	if C.avcodec_parameters_to_context(e.videoCodecCtx, streams[e.videoStreamIdx].codecpar) < 0 {
		return fmt.Errorf("failed to copy codec parameters")
	}

	if C.avcodec_open2(e.videoCodecCtx, videoCodec, nil) < 0 {
		return fmt.Errorf("failed to open codec")
	}

	e.packet = C.av_packet_alloc()
	e.frame = C.av_frame_alloc()

	// Initialize scaling context (to RGBA)
	e.swsCtx = C.sws_getContext(
		e.videoCodecCtx.width, e.videoCodecCtx.height, e.videoCodecCtx.pix_fmt,
		e.videoCodecCtx.width, e.videoCodecCtx.height, C.AV_PIX_FMT_RGBA,
		C.SWS_BILINEAR, nil, nil, nil,
	)

	// Pre-allocate RGBA frame
	e.rgbaFrame = C.av_frame_alloc()
	e.rgbaFrame.format = C.AV_PIX_FMT_RGBA
	e.rgbaFrame.width = e.videoCodecCtx.width
	e.rgbaFrame.height = e.videoCodecCtx.height

	numBytes := C.av_image_get_buffer_size(C.AV_PIX_FMT_RGBA, e.videoCodecCtx.width, e.videoCodecCtx.height, 1)
	e.rgbaBuffer = make([]byte, int(numBytes))
	C.av_image_fill_arrays(
		&e.rgbaFrame.data[0], &e.rgbaFrame.linesize[0],
		(*C.uint8_t)(unsafe.Pointer(&e.rgbaBuffer[0])), C.AV_PIX_FMT_RGBA,
		e.videoCodecCtx.width, e.videoCodecCtx.height, 1,
	)

	logging.Info(logging.CatPlayer, "Media engine initialized. Resolution: %dx%d",
		int(e.videoCodecCtx.width), int(e.videoCodecCtx.height))

	return nil
}

// NextFrame decodes the next video frame.
func (e *Engine) NextFrame() (*image.RGBA, error) {
	for {
		if C.av_read_frame(e.formatCtx, e.packet) < 0 {
			return nil, io.EOF
		}

		if int(e.packet.stream_index) == e.videoStreamIdx {
			if C.avcodec_send_packet(e.videoCodecCtx, e.packet) == 0 {
				for C.avcodec_receive_frame(e.videoCodecCtx, e.frame) == 0 {
					C.av_packet_unref(e.packet)
					return e.toRGBA(), nil
				}
			}
		}
		C.av_packet_unref(e.packet)
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

// Close releases all FFmpeg resources.
func (e *Engine) Close() {
	if e.swsCtx != nil {
		C.sws_freeContext(e.swsCtx)
	}
	if e.videoCodecCtx != nil {
		C.avcodec_free_context(&e.videoCodecCtx)
	}
	if e.formatCtx != nil {
		C.avformat_close_input(&e.formatCtx)
	}
	if e.packet != nil {
		C.av_packet_free(&e.packet)
	}
	if e.frame != nil {
		C.av_frame_free(&e.frame)
	}
	if e.rgbaFrame != nil {
		C.av_frame_free(&e.rgbaFrame)
	}
}
