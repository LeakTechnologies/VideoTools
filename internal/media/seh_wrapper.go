//go:build native_media

package media

/*
#include <libavcodec/avcodec.h>
#include "safe_bridge.h"
#include <stdint.h>
*/
import "C"

// SafeSendPacket wraps avcodec_send_packet with SEH protection on Windows.
// Returns (ret, excCode) where excCode != 0 means an exception was caught.
func SafeSendPacket(ctx *C.AVCodecContext, pkt *C.AVPacket) (int, uint32) {
	var excCode C.uint32_t
	ret := int(C.safe_avcodec_send_packet(ctx, pkt, &excCode))
	return ret, uint32(excCode)
}

// SafeReceiveFrame wraps avcodec_receive_frame with SEH protection on Windows.
func SafeReceiveFrame(ctx *C.AVCodecContext, frame *C.AVFrame) (int, uint32) {
	var excCode C.uint32_t
	ret := int(C.safe_avcodec_receive_frame(ctx, frame, &excCode))
	return ret, uint32(excCode)
}

// SafeHWFrameTransfer wraps av_hwframe_transfer_data with SEH protection.
// Returns (ret, excCode); excCode != 0 means an access violation was caught.
func SafeHWFrameTransfer(dst *C.AVFrame, src *C.AVFrame, flags int) (int, uint32) {
	var excCode C.uint32_t
	ret := int(C.safe_av_hwframe_transfer_data(dst, src, C.int(flags), &excCode))
	return ret, uint32(excCode)
}

// SafeSwsScaleFrame wraps sws_scale with SEH protection, taking AVFrame pointers
// instead of uint8_t** to avoid CGo double-pointer restrictions.
// Returns (ret, excCode); excCode != 0 means an access violation was caught.
func SafeSwsScaleFrame(ctx *C.struct_SwsContext, src *C.AVFrame, srcY, srcH int, dst *C.AVFrame) (int, uint32) {
	var excCode C.uint32_t
	ret := int(C.safe_sws_scale_frame(ctx, src, C.int(srcY), C.int(srcH), dst, &excCode))
	return ret, uint32(excCode)
}
