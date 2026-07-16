//go:build native_media

package media

/*
#include <libavcodec/avcodec.h>
#include <libswscale/swscale.h>
#include <libavutil/avutil.h>
*/
import "C"
import (
	"image"

	"github.com/LeakTechnologies/VideoTools/internal/logging"
)

// ensureSwsCtx lazily creates e.swsCtx using the actual pixel format of the
// currently decoded frame. When HW decode is active, videoCodecCtx.pix_fmt is
// NONE at Open time, so we can't create swsCtx until we see a real frame.
func (e *Engine) ensureSwsCtx(fmt C.enum_AVPixelFormat) {
	if e.swsCtx != nil && e.swsFmt == fmt {
		return
	}
	if e.swsCtx != nil {
		C.sws_freeContext(e.swsCtx)
		e.swsCtx = nil
	}
	w := e.videoCodecCtx.width
	h := e.videoCodecCtx.height
	e.swsCtx = C.sws_getContext(
		w, h, fmt,
		w, h, C.AV_PIX_FMT_RGBA,
		C.SWS_FAST_BILINEAR, nil, nil, nil,
	)
	if e.swsCtx != nil {
		e.swsFmt = fmt
		logging.Info(logging.CatPlayer, "ensureSwsCtx: created swsCtx for fmt=%d", int(fmt))
	} else {
		logging.Warning(logging.CatPlayer, "ensureSwsCtx: sws_getContext failed for fmt=%d", int(fmt))
		e.swsFmt = 0
	}
}

func (e *Engine) toRGBA(src *C.AVFrame) (img *image.RGBA) {
	defer func() {
		if r := recover(); r != nil {
			logging.Error(logging.CatPlayer, "toRGBA panic: %v", r)
			img = nil
		}
	}()

	if src == nil {
		src = e.frame
	}

	logging.Debug(logging.CatPlayer, "toRGBA: entering sws_scale, swsCtx=%v", e.swsCtx != nil)
	swsRet, swsExc := SafeSwsScaleFrame(e.swsCtx, src, 0, int(e.videoCodecCtx.height), e.rgbaFrame)
	if swsRet < 0 {
		if swsExc != 0 {
			logging.Error(logging.CatPlayer, "toRGBA: sws_scale SEH exception (exc=0x%08X)", swsExc)
		} else {
			logging.Warning(logging.CatPlayer, "toRGBA: sws_scale failed")
		}
		return nil
	}

	w, h := int(e.videoCodecCtx.width), int(e.videoCodecCtx.height)

	e.lockFramepoolMu()
	if len(e.framePool) > 0 {
		buf := e.framePool[len(e.framePool)-1]
		e.framePool = e.framePool[:len(e.framePool)-1]
		if len(buf) >= w*h*4 {
			img = &image.RGBA{
				Pix:    buf[:w*h*4],
				Stride: w * 4,
				Rect:   image.Rect(0, 0, w, h),
			}
		}
	}
	e.unlockFramepoolMu()

	if img == nil {
		img = image.NewRGBA(image.Rect(0, 0, w, h))
	}

	copy(img.Pix, e.rgbaBuffer)
	return img
}

func (e *Engine) ReleaseFrame(img *image.RGBA) {
	if img == nil {
		return
	}

	e.lockFramepoolMu()
	defer e.unlockFramepoolMu()

	if len(e.framePool) < 4 {
		buf := make([]byte, len(img.Pix))
		copy(buf, img.Pix)
		e.framePool = append(e.framePool, buf)
	}
}

func (e *Engine) GetFramePoolSize() int {
	e.lockFramepoolMu()
	defer e.unlockFramepoolMu()
	return len(e.framePool)
}


