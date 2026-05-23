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
#include <libavutil/imgutils.h>
#include <libavutil/hwcontext.h>

// vt_get_hw_format is the get_format callback for hardware-accelerated decoding.
// It is required by D3D11VA and other HW backends: without it FFmpeg cannot
// negotiate the HW pixel format, and avcodec_send_packet crashes on the first
// packet.
//
// When the target HW format is found, this callback also fully initialises
// hw_frames_ctx using avcodec_get_hw_frames_parameters (codec-specific
// constraints) with a fallback to NV12 + a 20-frame pool.  Setting
// hw_frames_ctx here is the pattern from FFmpeg's hw_decode.c example.
//
// ctx->opaque must hold the desired HW pixel format cast to void* (intptr_t).
//
// NOTE: D3D11VA decoders may offer either AV_PIX_FMT_D3D11 or
// AV_PIX_FMT_D3D11VA_VLD in the pix_fmts list depending on the FFmpeg
// version.  We accept both so the callback works with old and new builds.
static enum AVPixelFormat vt_get_hw_format(AVCodecContext *ctx,
                                            const enum AVPixelFormat *pix_fmts)
{
    enum AVPixelFormat target = (enum AVPixelFormat)(intptr_t)ctx->opaque;
    const enum AVPixelFormat *p;
    AVBufferRef *frames_ref;
    AVHWFramesContext *fc;
    int w, h;

    for (p = pix_fmts; *p != AV_PIX_FMT_NONE; p++) {
        if (*p != target
            && !(target == AV_PIX_FMT_D3D11 && *p == AV_PIX_FMT_D3D11VA_VLD))
            continue;

        enum AVPixelFormat chosen = *p;

        frames_ref = NULL;

        if (avcodec_get_hw_frames_parameters(ctx, ctx->hw_device_ctx,
                                             chosen, &frames_ref) < 0
                || frames_ref == NULL) {
            frames_ref = av_hwframe_ctx_alloc(ctx->hw_device_ctx);
            if (!frames_ref)
                return AV_PIX_FMT_NONE;

            fc            = (AVHWFramesContext *)frames_ref->data;
            w             = ctx->coded_width  ? ctx->coded_width  : ctx->width;
            h             = ctx->coded_height ? ctx->coded_height : ctx->height;
            fc->format            = chosen;
            fc->sw_format         = AV_PIX_FMT_NV12;
            fc->width             = w;
            fc->height            = h;
            fc->initial_pool_size = 20;
        }

        if (av_hwframe_ctx_init(frames_ref) < 0) {
            av_buffer_unref(&frames_ref);
            return AV_PIX_FMT_NONE;
        }

        av_buffer_unref(&ctx->hw_frames_ctx);
        ctx->hw_frames_ctx = av_buffer_ref(frames_ref);
        av_buffer_unref(&frames_ref);

        if (!ctx->hw_frames_ctx)
            return AV_PIX_FMT_NONE;

        return chosen;
    }

    return AV_PIX_FMT_NONE;
}

// vt_set_get_format wires the get_format callback, stores the target HW pixel
// format in ctx->opaque, and forces single-threaded decode.  D3D11VA (and
// other HW backends) do not support frame-level multi-threading: each worker
// thread gets a copy of the codec context with hw_frames_ctx still NULL,
// causing a crash in avcodec_send_packet.
// CGo cannot assign C function pointers to struct fields directly, so we use
// a C helper for the assignment.
//
// For D3D11VA, we prefer AV_PIX_FMT_D3D11 (new API) but the vt_get_hw_format
// callback also accepts AV_PIX_FMT_D3D11VA_VLD (legacy) to cover builds where
// the codec offers the old format name.
static void vt_set_get_format(AVCodecContext *ctx, int hw_pix_fmt) {
    ctx->opaque       = (void*)(intptr_t)hw_pix_fmt;
    ctx->get_format   = vt_get_hw_format;
    ctx->thread_count = 1;
    ctx->thread_type  = 0;
}
*/
import "C"
import (
	"fmt"
	"image"
	"sync"
	"unsafe"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
)

// Hardware-accelerated pixel formats for the get_format callback.
// D3D11VA decoders may offer either D3D11 or D3D11VA_VLD depending on
// the FFmpeg version; both are accepted by vt_get_hw_format.

// HWDeviceType represents a supported hardware acceleration backend.
type HWDeviceType int

const (
	HWDeviceNone   HWDeviceType = iota
	HWDeviceVAAPI
	HWDeviceD3D11VA
	HWDeviceQSV
)

// hwDecodeEnabled controls whether hardware-accelerated video decoding is used.
// D3D11VA/VAAPI/QSV are enabled by default.  All FFmpeg call sites in the video
// decode path (avcodec_send_packet, avcodec_receive_frame, av_hwframe_transfer_data,
// sws_scale) are wrapped in safe_bridge.c SEH/__try guards, so C-level access
// violations are caught and converted to recoverable Go errors rather than
// killing the process.  DegradeToSoftware() is wired into the decode loop and
// will fall back to SW decode on the first HW failure.
var hwDecodeEnabled = true

// SetHWDecodeEnabled allows the caller (e.g. Settings) to opt in to HW
// decode.  The default is off because D3D11VA crashes cannot be caught by Go.
func SetHWDecodeEnabled(enabled bool) {
	hwDecodeEnabled = enabled
}

func HWDecodeEnabled() bool {
	return hwDecodeEnabled
}

// hwDeviceDetected caches the result of the one-time hardware detection so
// that av_hwdevice_ctx_create is never called from a background goroutine
// after the GLFW message loop has started (on Windows, D3D11VA device creation
// uses COM STA dispatch and deadlocks with the GLFW message pump).
var (
	hwDeviceDetected   HWDeviceType
	hwDeviceDetectOnce sync.Once
)

func doHWDetect() {
	if checkVAAPIAvailable() {
		hwDeviceDetected = HWDeviceVAAPI
	} else if checkD3D11VAAvailable() {
		hwDeviceDetected = HWDeviceD3D11VA
	} else if checkQSVAvailable() {
		hwDeviceDetected = HWDeviceQSV
	}
}

// WarmHWDeviceCache runs the hardware-detection probes synchronously on the
// calling goroutine regardless of the hwDecodeEnabled flag, then caches the
// result.  Call this once from the main goroutine before ShowAndRun() so that
// subsequent DetectHWDevice() calls from background goroutines always return
// the cached value without touching COM.
//
// The unconditional probe is required to handle the case where the user starts
// with HW decode disabled and enables it mid-session via Settings: without this
// the sync.Once would fire later from a Load() background goroutine and
// deadlock with the GLFW message pump (Windows D3D11VA COM STA).
func WarmHWDeviceCache() {
	hwDeviceDetectOnce.Do(doHWDetect)
}

func DetectHWDevice() HWDeviceType {
	if !hwDecodeEnabled {
		return HWDeviceNone
	}
	hwDeviceDetectOnce.Do(doHWDetect)
	return hwDeviceDetected
}

func checkVAAPIAvailable() bool {
	var devCtx *C.AVBufferRef
	if C.av_hwdevice_ctx_create(&devCtx, C.AV_HWDEVICE_TYPE_VAAPI, nil, nil, 0) == 0 {
		if devCtx != nil {
			C.av_buffer_unref(&devCtx)
		}
		return true
	}
	return false
}

func checkD3D11VAAvailable() bool {
	var devCtx *C.AVBufferRef
	if C.av_hwdevice_ctx_create(&devCtx, C.AV_HWDEVICE_TYPE_D3D11VA, nil, nil, 0) == 0 {
		if devCtx != nil {
			C.av_buffer_unref(&devCtx)
		}
		return true
	}
	return false
}

func checkQSVAvailable() bool {
	var devCtx *C.AVBufferRef
	if C.av_hwdevice_ctx_create(&devCtx, C.AV_HWDEVICE_TYPE_QSV, nil, nil, 0) == 0 {
		if devCtx != nil {
			C.av_buffer_unref(&devCtx)
		}
		return true
	}
	return false
}

func (e *Engine) SetHWDevice(hw HWDeviceType) {
	e.hwDevice = hw
}

func (e *Engine) GetHWDevice() HWDeviceType {
	return e.hwDevice
}

// codecCanUseHWDevice returns true only for codecs that are known to work
// correctly with hw_device_ctx and no get_format callback.
//
// Several codecs (notably VC-1/WMV3 and MPEG-2) advertise D3D11VA support via
// avcodec_get_hw_config but require a get_format callback to negotiate the HW
// pixel format at decode time.  Without that callback the decoder selects a SW
// format, then crashes inside avcodec_send_packet when it tries to allocate HW
// surfaces against the un-negotiated format.
//
// The codecs listed below have been validated to work with the simpler
// hw_device_ctx-only path (no get_format callback):
//
//	h264, hevc, vp9, av1, vp8
//
// Everything else falls back to software decode.
func (e *Engine) codecCanUseHWDevice(codec *C.AVCodec) bool {
	name := C.GoString((*C.char)(unsafe.Pointer(codec.name)))
	switch name {
	case "h264", "hevc", "vp9", "av1", "vp8":
	default:
		return false
	}

	var hwType C.enum_AVHWDeviceType
	switch e.hwDevice {
	case HWDeviceVAAPI:
		hwType = C.AV_HWDEVICE_TYPE_VAAPI
	case HWDeviceD3D11VA:
		hwType = C.AV_HWDEVICE_TYPE_D3D11VA
	case HWDeviceQSV:
		hwType = C.AV_HWDEVICE_TYPE_QSV
	default:
		return false
	}
	for i := C.int(0); ; i++ {
		cfg := C.avcodec_get_hw_config(codec, i)
		if cfg == nil {
			return false
		}
		if cfg.methods&C.AV_CODEC_HW_CONFIG_METHOD_HW_DEVICE_CTX != 0 &&
			cfg.device_type == hwType {
			return true
		}
	}
}

func (e *Engine) initHWDecode() error {
	var hwType C.enum_AVHWDeviceType
	switch e.hwDevice {
	case HWDeviceVAAPI:
		hwType = C.AV_HWDEVICE_TYPE_VAAPI
	case HWDeviceD3D11VA:
		hwType = C.AV_HWDEVICE_TYPE_D3D11VA
	case HWDeviceQSV:
		hwType = C.AV_HWDEVICE_TYPE_QSV
	default:
		return fmt.Errorf("unsupported HW device type: %v", e.hwDevice)
	}

	var devCtxRef *C.AVBufferRef
	if C.av_hwdevice_ctx_create(&devCtxRef, hwType, nil, nil, 0) != 0 {
		return fmt.Errorf("failed to create HW device context")
	}

	e.videoCodecCtx.hw_device_ctx = C.av_buffer_ref(devCtxRef)
	if e.videoCodecCtx.hw_device_ctx == nil {
		C.av_buffer_unref(&devCtxRef)
		return fmt.Errorf("failed to attach HW device context to codec ctx")
	}
	e.hwDeviceCtx = devCtxRef

	hwFmt := e.getHWPixelFormat(hwType)
	C.vt_set_get_format(e.videoCodecCtx, C.int(hwFmt))

	logging.Info(logging.CatPlayer, "HW decode enabled: %v (hwFmt=%d)", e.hwDevice, int(hwFmt))
	return nil
}

func (e *Engine) getHWPixelFormat(hwType C.enum_AVHWDeviceType) C.enum_AVPixelFormat {
	switch hwType {
	case C.AV_HWDEVICE_TYPE_VAAPI:
		return C.AV_PIX_FMT_VAAPI
	case C.AV_HWDEVICE_TYPE_D3D11VA:
		return C.AV_PIX_FMT_D3D11
	case C.AV_HWDEVICE_TYPE_QSV:
		return C.AV_PIX_FMT_QSV
	}
	return C.AV_PIX_FMT_NONE
}

func (e *Engine) retrieveHWFrame() (*image.RGBA, error) {
	if e.frame.hw_frames_ctx == nil {
		return e.toRGBA(nil), nil
	}

	swFrame := C.av_frame_alloc()
	if swFrame == nil {
		return nil, fmt.Errorf("failed to allocate sw frame")
	}
	defer C.av_frame_free(&swFrame)

	transferRet, transferExc := SafeHWFrameTransfer(swFrame, e.frame, 0)
	if transferRet != 0 {
		if transferExc != 0 {
			logging.Error(logging.CatPlayer, "retrieveHWFrame: av_hwframe_transfer_data SEH exception (exc=0x%08X) — disabling HW decode", transferExc)
			e.SetError(ErrCodeHWAccel, fmt.Sprintf("av_hwframe_transfer_data SEH 0x%08X", transferExc), false)
			e.videoDecodeDead = true
		} else {
			logging.Warning(logging.CatPlayer, "retrieveHWFrame: av_hwframe_transfer_data failed")
		}
		return nil, fmt.Errorf("failed to transfer HW frame to SW")
	}

	swFmt := C.enum_AVPixelFormat(swFrame.format)
	w := int(swFrame.width)
	h := int(swFrame.height)

	if e.hwSwsCtx == nil || e.hwSwsFmt != swFmt || e.hwSwsW != w || e.hwSwsH != h {
		if e.hwSwsCtx != nil {
			C.sws_freeContext(e.hwSwsCtx)
		}
		e.hwSwsCtx = C.sws_getContext(
			swFrame.width, swFrame.height, swFmt,
			swFrame.width, swFrame.height, C.AV_PIX_FMT_RGBA,
			C.SWS_BILINEAR, nil, nil, nil,
		)
		if e.hwSwsCtx == nil {
			return nil, fmt.Errorf("failed to create sws context for hw pixel format %d", int(swFmt))
		}
		e.hwSwsFmt = swFmt
		e.hwSwsW = w
		e.hwSwsH = h
		logging.Info(logging.CatPlayer, "retrieveHWFrame: created cached swsCtx for fmt=%d %dx%d", int(swFmt), w, h)
	}

	rowStride := C.int(w * 4)
	bufSize := w * h * 4
	if len(e.hwRgbaBuffer) < bufSize || e.hwRgbaFrame == nil {
		if e.hwRgbaFrame != nil {
			C.av_frame_free(&e.hwRgbaFrame)
		}
		e.hwRgbaFrame = C.av_frame_alloc()
		e.hwRgbaBuffer = make([]byte, bufSize)
		C.av_image_fill_arrays(
			&e.hwRgbaFrame.data[0], &e.hwRgbaFrame.linesize[0],
			(*C.uint8_t)(unsafe.Pointer(&e.hwRgbaBuffer[0])), C.AV_PIX_FMT_RGBA,
			C.int(w), C.int(h), 1,
		)
	}

	e.hwRgbaFrame.linesize[0] = rowStride

	scaleRet, scaleExc := SafeSwsScaleFrame(e.hwSwsCtx, swFrame, 0, int(swFrame.height), e.hwRgbaFrame)
	if scaleRet < 0 {
		if scaleExc != 0 {
			logging.Error(logging.CatPlayer, "retrieveHWFrame: sws_scale SEH exception (exc=0x%08X) — disabling HW decode", scaleExc)
			e.SetError(ErrCodeHWAccel, fmt.Sprintf("sws_scale SEH 0x%08X", scaleExc), false)
			e.videoDecodeDead = true
		} else {
			logging.Warning(logging.CatPlayer, "retrieveHWFrame: sws_scale failed")
		}
		return nil, fmt.Errorf("sws_scale failed during HW→RGBA conversion")
	}

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	srcStride := int(e.hwRgbaFrame.linesize[0])
	for y := 0; y < h; y++ {
		src := e.hwRgbaBuffer[y*srcStride : y*srcStride+w*4]
		dst := img.Pix[y*img.Stride : y*img.Stride+w*4]
		copy(dst, src)
	}
	return img, nil
}
