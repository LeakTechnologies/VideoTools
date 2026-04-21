//go:build native_media

package media

/*
#cgo !windows pkg-config: libavcodec libavutil
#cgo windows CFLAGS: -IC:/ffmpeg/include
#cgo windows LDFLAGS: -LC:/ffmpeg/lib -lavcodec -lavutil
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
