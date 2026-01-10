//go:build gstreamer

package player

/*
#cgo pkg-config: gstreamer-1.0 gstreamer-app-1.0 gstreamer-video-1.0
#include <gst/gst.h>
#include <gst/app/gstappsink.h>
#include <gst/video/video.h>
#include <stdlib.h>

static void vt_gst_set_str(GstElement* elem, const char* name, const char* value) {
	g_object_set(G_OBJECT(elem), name, value, NULL);
}
static void vt_gst_set_bool(GstElement* elem, const char* name, gboolean value) {
	g_object_set(G_OBJECT(elem), name, value, NULL);
}
static void vt_gst_set_int(GstElement* elem, const char* name, gint value) {
	g_object_set(G_OBJECT(elem), name, value, NULL);
}
static void vt_gst_set_float(GstElement* elem, const char* name, gdouble value) {
	g_object_set(G_OBJECT(elem), name, value, NULL);
}
static void vt_gst_set_obj(GstElement* elem, const char* name, gpointer value) {
	g_object_set(G_OBJECT(elem), name, value, NULL);
}
*/
import "C"

import (
	"errors"
	"image"
	"net/url"
	"path/filepath"
	"runtime"
	"sync"
	"time"
	"unsafe"
)

var gstInitOnce sync.Once

type GStreamerPlayer struct {
	mu       sync.Mutex
	pipeline *C.GstElement
	appsink  *C.GstElement
	paused   bool
	volume   float64
	preview  bool
	width    int
	height   int
	fps      float64
}

func NewGStreamerPlayer(config Config) (*GStreamerPlayer, error) {
	var initErr error
	gstInitOnce.Do(func() {
		if C.gst_init_check(nil, nil, nil) == 0 {
			initErr = errors.New("gstreamer init failed")
		}
	})
	if initErr != nil {
		return nil, initErr
	}

	return &GStreamerPlayer{
		paused:  true,
		volume:  config.Volume,
		preview: config.PreviewMode,
	}, nil
}

func (p *GStreamerPlayer) Load(path string, offset time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.closeLocked()

	playbinName := C.CString("playbin")
	playbin := C.gst_element_factory_make(playbinName, nil)
	C.free(unsafe.Pointer(playbinName))
	if playbin == nil {
		return errors.New("gstreamer playbin unavailable")
	}

	appsinkName := C.CString("appsink")
	appsink := C.gst_element_factory_make(appsinkName, nil)
	C.free(unsafe.Pointer(appsinkName))
	if appsink == nil {
		C.gst_object_unref(C.gpointer(playbin))
		return errors.New("gstreamer appsink unavailable")
	}

	capsStr := C.CString("video/x-raw,format=RGBA")
	caps := C.gst_caps_from_string(capsStr)
	C.free(unsafe.Pointer(capsStr))
	if caps != nil {
	capsName := C.CString("caps")
	C.vt_gst_set_obj(appsink, capsName, C.gpointer(caps))
	C.free(unsafe.Pointer(capsName))
		C.gst_caps_unref(caps)
	}
	emitSignals := C.CString("emit-signals")
	C.vt_gst_set_bool(appsink, emitSignals, C.gboolean(0))
	C.free(unsafe.Pointer(emitSignals))
	syncName := C.CString("sync")
	C.vt_gst_set_bool(appsink, syncName, C.gboolean(0))
	C.free(unsafe.Pointer(syncName))
	maxBuffers := C.CString("max-buffers")
	C.vt_gst_set_int(appsink, maxBuffers, C.gint(2))
	C.free(unsafe.Pointer(maxBuffers))
	dropName := C.CString("drop")
	C.vt_gst_set_bool(appsink, dropName, C.gboolean(1))
	C.free(unsafe.Pointer(dropName))

	var audioSink *C.GstElement
	if p.preview {
		fakeName := C.CString("fakesink")
		audioSink = C.gst_element_factory_make(fakeName, nil)
		C.free(unsafe.Pointer(fakeName))
	} else {
		autoName := C.CString("autoaudiosink")
		audioSink = C.gst_element_factory_make(autoName, nil)
		C.free(unsafe.Pointer(autoName))
	}
	if audioSink == nil {
		C.gst_object_unref(C.gpointer(playbin))
		C.gst_object_unref(C.gpointer(appsink))
		return errors.New("gstreamer audio sink unavailable")
	}

	uri := fileURI(path)
	uriC := C.CString(uri)
	uriName := C.CString("uri")
	C.vt_gst_set_str(playbin, uriName, uriC)
	C.free(unsafe.Pointer(uriName))
	C.free(unsafe.Pointer(uriC))
	videoSinkName := C.CString("video-sink")
	C.vt_gst_set_obj(playbin, videoSinkName, C.gpointer(appsink))
	C.free(unsafe.Pointer(videoSinkName))
	audioSinkName := C.CString("audio-sink")
	C.vt_gst_set_obj(playbin, audioSinkName, C.gpointer(audioSink))
	C.free(unsafe.Pointer(audioSinkName))

	if p.volume <= 0 {
		p.volume = 1.0
	}
	volumeName := C.CString("volume")
	C.vt_gst_set_float(playbin, volumeName, C.gdouble(p.volume))
	C.free(unsafe.Pointer(volumeName))

	p.pipeline = playbin
	p.appsink = appsink
	p.paused = true

	// Set to PAUSED to preroll (loads first frame)
	C.gst_element_set_state(playbin, C.GST_STATE_PAUSED)

	// Wait for preroll to complete (first frame ready)
	bus := C.gst_element_get_bus(playbin)
	if bus != nil {
		defer C.gst_object_unref(C.gpointer(bus))
		// Wait up to 5 seconds for preroll
		msg := C.gst_bus_timed_pop_filtered(bus, 5000000000, C.GST_MESSAGE_ASYNC_DONE|C.GST_MESSAGE_ERROR)
		if msg != nil {
			C.gst_message_unref(msg)
		}
	}

	if offset > 0 {
		_ = p.seekLocked(offset)
	}

	return nil
}

func (p *GStreamerPlayer) Play() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.pipeline == nil {
		return errors.New("no pipeline loaded")
	}
	C.gst_element_set_state(p.pipeline, C.GST_STATE_PLAYING)
	p.paused = false
	return nil
}

func (p *GStreamerPlayer) Pause() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.pipeline == nil {
		return errors.New("no pipeline loaded")
	}
	C.gst_element_set_state(p.pipeline, C.GST_STATE_PAUSED)
	p.paused = true
	return nil
}

func (p *GStreamerPlayer) SeekToTime(offset time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.seekLocked(offset)
}

func (p *GStreamerPlayer) seekLocked(offset time.Duration) error {
	if p.pipeline == nil {
		return errors.New("no pipeline loaded")
	}
	nanos := C.gint64(offset.Nanoseconds())
	flags := C.GstSeekFlags(C.GST_SEEK_FLAG_FLUSH | C.GST_SEEK_FLAG_KEY_UNIT)
	if C.gst_element_seek_simple(p.pipeline, C.GST_FORMAT_TIME, flags, nanos) == 0 {
		return errors.New("gstreamer seek failed")
	}
	return nil
}

func (p *GStreamerPlayer) SeekToFrame(frame int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fps <= 0 {
		return nil
	}
	seconds := float64(frame) / p.fps
	return p.seekLocked(time.Duration(seconds * float64(time.Second)))
}

func (p *GStreamerPlayer) GetCurrentTime() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pipeline == nil {
		return 0
	}
	var pos C.gint64
	if C.gst_element_query_position(p.pipeline, C.GST_FORMAT_TIME, &pos) == 0 {
		return 0
	}
	return time.Duration(pos)
}

func (p *GStreamerPlayer) GetFrameImage() (*image.RGBA, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.appsink == nil {
		return nil, errors.New("gstreamer appsink unavailable")
	}
	sample := C.gst_app_sink_try_pull_sample((*C.GstAppSink)(unsafe.Pointer(p.appsink)), 0)
	if sample == nil {
		return nil, nil
	}
	defer C.gst_sample_unref(sample)

	caps := C.gst_sample_get_caps(sample)
	if caps == nil {
		return nil, errors.New("gstreamer caps unavailable")
	}
	str := C.gst_caps_get_structure(caps, 0)
	var width C.gint
	var height C.gint
	widthName := C.CString("width")
	C.gst_structure_get_int(str, widthName, &width)
	C.free(unsafe.Pointer(widthName))
	heightName := C.CString("height")
	C.gst_structure_get_int(str, heightName, &height)
	C.free(unsafe.Pointer(heightName))
	if width > 0 && height > 0 {
		p.width = int(width)
		p.height = int(height)
	}
	var fpsNum C.gint
	var fpsDen C.gint
	fpsName := C.CString("framerate")
	if C.gst_structure_get_fraction(str, fpsName, &fpsNum, &fpsDen) != 0 && fpsDen != 0 {
		p.fps = float64(fpsNum) / float64(fpsDen)
	}
	C.free(unsafe.Pointer(fpsName))

	buffer := C.gst_sample_get_buffer(sample)
	if buffer == nil {
		return nil, errors.New("gstreamer buffer unavailable")
	}
	var mapInfo C.GstMapInfo
	if C.gst_buffer_map(buffer, &mapInfo, C.GST_MAP_READ) == 0 {
		return nil, errors.New("gstreamer buffer map failed")
	}
	defer C.gst_buffer_unmap(buffer, &mapInfo)

	if p.width == 0 || p.height == 0 {
		return nil, errors.New("invalid frame size")
	}
	frameSize := p.width * p.height * 4
	if int(mapInfo.size) < frameSize {
		return nil, errors.New("incomplete frame")
	}

	img := image.NewRGBA(image.Rect(0, 0, p.width, p.height))
	data := unsafe.Slice((*byte)(unsafe.Pointer(mapInfo.data)), frameSize)
	copy(img.Pix, data)
	return img, nil
}

func (p *GStreamerPlayer) SetVolume(level float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.volume = level
	if p.pipeline != nil {
		volumeName := C.CString("volume")
		C.vt_gst_set_float(p.pipeline, volumeName, C.gdouble(level))
		C.free(unsafe.Pointer(volumeName))
	}
	return nil
}

func (p *GStreamerPlayer) SetWindow(x, y, w, h int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// GStreamer with appsink doesn't need window positioning
	// The frames are extracted and displayed by Fyne
	// Store dimensions for frame sizing
	if w > 0 && h > 0 {
		p.width = w
		p.height = h
	}
}

func (p *GStreamerPlayer) SetFullScreen(fullscreen bool) error {
	// Fullscreen is handled by the application window, not GStreamer
	// GStreamer with appsink just provides frames
	return nil
}

func (p *GStreamerPlayer) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pipeline != nil {
		C.gst_element_set_state(p.pipeline, C.GST_STATE_NULL)
	}
	return nil
}

func (p *GStreamerPlayer) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closeLocked()
}

func (p *GStreamerPlayer) closeLocked() {
	if p.pipeline != nil {
		C.gst_element_set_state(p.pipeline, C.GST_STATE_NULL)
		C.gst_object_unref(C.gpointer(p.pipeline))
		p.pipeline = nil
	}
	if p.appsink != nil {
		C.gst_object_unref(C.gpointer(p.appsink))
		p.appsink = nil
	}
}

func fileURI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	abs = filepath.ToSlash(abs)
	if runtime.GOOS == "windows" && len(abs) >= 2 && abs[1] == ':' {
		abs = "/" + abs
	}
	u := url.URL{Scheme: "file", Path: abs}
	return u.String()
}
