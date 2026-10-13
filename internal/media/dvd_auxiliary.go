//go:build native_media

package media

/*
#include <libavformat/avformat.h>
#include <stdlib.h>
*/
import "C"
import (
	"strconv"
	"unsafe"
)

// Auxiliary decoders must preserve the source demuxer and selected DVD title.
func openPlayerAuxiliaryInput(path string, dvd bool, title int) (*C.AVFormatContext, C.int) {
	var ctx *C.AVFormatContext
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	if !dvd {
		ret := C.avformat_open_input(&ctx, p, nil, nil)
		return ctx, ret
	}
	name := C.CString("dvdvideo")
	defer C.free(unsafe.Pointer(name))
	format := C.av_find_input_format(name)
	if format == nil {
		return nil, -1
	}
	var opts *C.AVDictionary
	defer C.av_dict_free(&opts)
	if title > 0 {
		key := C.CString("title")
		value := C.CString(strconv.Itoa(title))
		C.av_dict_set(&opts, key, value, 0)
		C.free(unsafe.Pointer(key))
		C.free(unsafe.Pointer(value))
	}
	ret := C.avformat_open_input(&ctx, p, format, &opts)
	return ctx, ret
}
