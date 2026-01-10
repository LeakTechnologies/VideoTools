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
static char* vt_gst_error_from_message(GstMessage* msg) {
	GError* err = NULL;
	gchar* debug = NULL;
	gst_message_parse_error(msg, &err, &debug);
	if (debug != NULL) {
		g_free(debug);
	}
	if (err == NULL) {
		return NULL;
	}
	char* out = g_strdup(err->message != NULL ? err->message : "gstreamer error");
	g_error_free(err);
	return out;
}
static void vt_gst_free_error(char* msg) {
	if (msg != NULL) {
		g_free(msg);
	}
}
static gboolean vt_gst_message_is_error(GstMessage* msg) {
	return GST_MESSAGE_TYPE(msg) == GST_MESSAGE_ERROR;
}
static GstSample* vt_gst_pull_sample(GstAppSink* sink, GstClockTime timeout, gboolean paused) {
	if (paused) {
		return gst_app_sink_try_pull_preroll(sink, timeout);
	}
	return gst_app_sink_try_pull_sample(sink, timeout);
}
static GstMessageType vt_gst_message_mask(void) {
	return GST_MESSAGE_ERROR
		| GST_MESSAGE_EOS
		| GST_MESSAGE_STATE_CHANGED
		| GST_MESSAGE_DURATION_CHANGED
		| GST_MESSAGE_ASYNC_DONE
		| GST_MESSAGE_CLOCK_LOST;
}
static GstMessageType vt_gst_message_type(GstMessage* msg) {
	return GST_MESSAGE_TYPE(msg);
}
static void vt_gst_parse_state_changed(GstMessage* msg, GstState* old_state, GstState* new_state, GstState* pending) {
	gst_message_parse_state_changed(msg, old_state, new_state, pending);
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
	seekMu   sync.Mutex
	pipeline *C.GstElement
	appsink  *C.GstElement
	bus      *C.GstBus
	busQuit  chan struct{}
	busDone  chan struct{}
	events   chan busEvent
	paused   bool
	volume   float64
	preview  bool
	width    int
	height   int
	fps      float64
	queued   *image.RGBA
	lastErr  string
	eos      bool
	state    C.GstState
	duration time.Duration
	mode     PlayerState
}

type busEvent struct {
	Kind  string
	Info  string
	State C.GstState
}

type PlayerState int

const (
	StateIdle PlayerState = iota
	StateLoading
	StatePaused
	StatePlaying
	StateSeeking
	StateStepping
	StateStopped
	StateError
	StateEOS
)

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
		events:  make(chan busEvent, 8),
		paused:  true,
		volume:  config.Volume,
		preview: config.PreviewMode,
		mode:    StateIdle,
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
	if p.preview {
		C.vt_gst_set_bool(appsink, syncName, C.gboolean(0))
	} else {
		C.vt_gst_set_bool(appsink, syncName, C.gboolean(1))
	}
	C.free(unsafe.Pointer(syncName))
	maxBuffers := C.CString("max-buffers")
	if p.preview {
		C.vt_gst_set_int(appsink, maxBuffers, C.gint(2))
	} else {
		C.vt_gst_set_int(appsink, maxBuffers, C.gint(1))
	}
	C.free(unsafe.Pointer(maxBuffers))
	dropName := C.CString("drop")
	if p.preview {
		C.vt_gst_set_bool(appsink, dropName, C.gboolean(1))
	} else {
		C.vt_gst_set_bool(appsink, dropName, C.gboolean(0))
	}
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
	p.eos = false
	p.lastErr = ""
	p.mode = StateLoading

	// Set to PAUSED to preroll (loads first frame)
	if C.gst_element_set_state(playbin, C.GST_STATE_PAUSED) == C.GST_STATE_CHANGE_FAILURE {
		p.mode = StateError
		p.closeLocked()
		return errors.New("gstreamer failed to enter paused state")
	}

	// Wait for preroll to complete (first frame ready)
	bus := C.gst_element_get_bus(playbin)
	if bus != nil {
		defer C.gst_object_unref(C.gpointer(bus))
		// Wait up to 5 seconds for preroll
		msg := C.gst_bus_timed_pop_filtered(bus, 5000000000, C.GST_MESSAGE_ASYNC_DONE|C.GST_MESSAGE_ERROR)
		if msg != nil {
			if C.vt_gst_message_is_error(msg) != 0 {
				errMsg := C.vt_gst_error_from_message(msg)
				C.gst_message_unref(msg)
				p.closeLocked()
				if errMsg != nil {
					defer C.vt_gst_free_error(errMsg)
					p.mode = StateError
					return errors.New(C.GoString(errMsg))
				}
				p.mode = StateError
				return errors.New("gstreamer error while loading")
			}
			C.gst_message_unref(msg)
		}
	}

	if offset > 0 {
		_ = p.seekLocked(offset)
	}

	p.mode = StatePaused
	p.startBusLoopLocked()
	return nil
}

func (p *GStreamerPlayer) Play() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.pipeline == nil {
		return errors.New("no pipeline loaded")
	}
	if C.gst_element_set_state(p.pipeline, C.GST_STATE_PLAYING) == C.GST_STATE_CHANGE_FAILURE {
		p.mode = StateError
		return errors.New("gstreamer failed to enter playing state")
	}
	p.paused = false
	p.mode = StatePlaying
	return nil
}

func (p *GStreamerPlayer) Pause() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.pipeline == nil {
		return errors.New("no pipeline loaded")
	}
	if C.gst_element_set_state(p.pipeline, C.GST_STATE_PAUSED) == C.GST_STATE_CHANGE_FAILURE {
		p.mode = StateError
		return errors.New("gstreamer failed to enter paused state")
	}
	p.paused = true
	p.mode = StatePaused
	return nil
}

func (p *GStreamerPlayer) SeekToTime(offset time.Duration) error {
	p.seekMu.Lock()
	defer p.seekMu.Unlock()

	p.mu.Lock()
	prevMode := p.mode
	p.mode = StateSeeking
	p.mu.Unlock()

	err := p.seekLocked(offset)

	p.mu.Lock()
	if err != nil {
		p.mode = StateError
	} else {
		p.mode = prevMode
	}
	p.mu.Unlock()
	return err
}

func (p *GStreamerPlayer) seekLocked(offset time.Duration) error {
	return p.seekLockedWithFlags(offset, C.GST_SEEK_FLAG_FLUSH|C.GST_SEEK_FLAG_KEY_UNIT)
}

func (p *GStreamerPlayer) seekLockedWithFlags(offset time.Duration, flags C.GstSeekFlags) error {
	if p.pipeline == nil {
		return errors.New("no pipeline loaded")
	}
	nanos := C.gint64(offset.Nanoseconds())
	if C.gst_element_seek_simple(p.pipeline, C.GST_FORMAT_TIME, flags, nanos) == 0 {
		return errors.New("gstreamer seek failed")
	}
	p.primeAfterSeekLocked()
	return nil
}

func (p *GStreamerPlayer) SeekToFrame(frame int64) error {
	p.seekMu.Lock()
	defer p.seekMu.Unlock()

	p.mu.Lock()
	if p.fps <= 0 {
		p.mu.Unlock()
		return nil
	}
	prevMode := p.mode
	p.mode = StateStepping
	seconds := float64(frame) / p.fps
	p.mu.Unlock()

	flags := C.GstSeekFlags(C.GST_SEEK_FLAG_FLUSH | C.GST_SEEK_FLAG_ACCURATE)
	err := p.seekLockedWithFlags(time.Duration(seconds*float64(time.Second)), flags)

	p.mu.Lock()
	if err != nil {
		p.mode = StateError
	} else {
		p.mode = prevMode
	}
	p.mu.Unlock()
	return err
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
	if p.queued != nil {
		frame := p.queued
		p.queued = nil
		return frame, nil
	}
	pullTimeout := C.GstClockTime(50 * 1000 * 1000)
	if p.paused {
		pullTimeout = C.GstClockTime(200 * 1000 * 1000)
	}
	return p.readFrameLocked(pullTimeout)
}

func (p *GStreamerPlayer) readFrameLocked(timeout C.GstClockTime) (*image.RGBA, error) {
	if p.appsink == nil {
		return nil, errors.New("gstreamer appsink unavailable")
	}
	paused := C.gboolean(0)
	if p.paused {
		paused = C.gboolean(1)
	}
	sample := C.vt_gst_pull_sample((*C.GstAppSink)(unsafe.Pointer(p.appsink)), timeout, paused)
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

func (p *GStreamerPlayer) primeAfterSeekLocked() {
	if p.appsink == nil {
		return
	}
	p.drainPendingLocked()
	frame, err := p.readFrameLocked(C.GstClockTime(200 * 1000 * 1000))
	if err != nil || frame == nil {
		return
	}
	p.queued = frame
}

func (p *GStreamerPlayer) drainPendingLocked() {
	if p.appsink == nil {
		return
	}
	for i := 0; i < 5; i++ {
		sample := C.gst_app_sink_try_pull_sample((*C.GstAppSink)(unsafe.Pointer(p.appsink)), C.GstClockTime(0))
		if sample == nil {
			return
		}
		C.gst_sample_unref(sample)
	}
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
	p.mode = StateStopped
	return nil
}

func (p *GStreamerPlayer) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mode = StateStopped
	p.closeLocked()
}

func (p *GStreamerPlayer) Events() <-chan busEvent {
	return p.events
}

func (p *GStreamerPlayer) State() PlayerState {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.mode
}

func (p *GStreamerPlayer) closeLocked() {
	p.stopBusLoopLocked()
	if p.pipeline != nil {
		C.gst_element_set_state(p.pipeline, C.GST_STATE_NULL)
		C.gst_object_unref(C.gpointer(p.pipeline))
		p.pipeline = nil
	}
	if p.appsink != nil {
		C.gst_object_unref(C.gpointer(p.appsink))
		p.appsink = nil
	}
	if p.bus != nil {
		C.gst_object_unref(C.gpointer(p.bus))
		p.bus = nil
	}
}

func (p *GStreamerPlayer) startBusLoopLocked() {
	if p.pipeline == nil || p.bus != nil {
		return
	}
	bus := C.gst_element_get_bus(p.pipeline)
	if bus == nil {
		return
	}
	p.bus = bus
	p.busQuit = make(chan struct{})
	p.busDone = make(chan struct{})
	go p.busLoop()
}

func (p *GStreamerPlayer) stopBusLoopLocked() {
	if p.busQuit == nil {
		return
	}
	close(p.busQuit)
	if p.busDone != nil {
		<-p.busDone
	}
	p.busQuit = nil
	p.busDone = nil
}

func (p *GStreamerPlayer) busLoop() {
	defer func() {
		p.mu.Lock()
		if p.busDone != nil {
			close(p.busDone)
		}
		p.mu.Unlock()
	}()

	for {
		select {
		case <-p.busQuit:
			return
		default:
		}

		p.mu.Lock()
		bus := p.bus
		p.mu.Unlock()
		if bus == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		msg := C.gst_bus_timed_pop_filtered(bus, 200*1000*1000, C.vt_gst_message_mask())
		if msg == nil {
			continue
		}

		msgType := C.vt_gst_message_type(msg)
		switch msgType {
		case C.GST_MESSAGE_ERROR:
			errMsg := C.vt_gst_error_from_message(msg)
			p.mu.Lock()
			if errMsg != nil {
				p.lastErr = C.GoString(errMsg)
				C.vt_gst_free_error(errMsg)
			} else {
				p.lastErr = "gstreamer error"
			}
			p.mode = StateError
			evt := busEvent{Kind: "error", Info: p.lastErr}
			p.mu.Unlock()
			p.pushEvent(evt)
		case C.GST_MESSAGE_EOS:
			p.mu.Lock()
			p.eos = true
			p.mode = StateEOS
			p.mu.Unlock()
			p.pushEvent(busEvent{Kind: "eos"})
		case C.GST_MESSAGE_STATE_CHANGED:
			var oldState C.GstState
			var newState C.GstState
			var pending C.GstState
			C.vt_gst_parse_state_changed(msg, &oldState, &newState, &pending)
			p.mu.Lock()
			p.state = newState
			p.mu.Unlock()
			p.pushEvent(busEvent{Kind: "state_changed", State: newState})
		case C.GST_MESSAGE_DURATION_CHANGED:
			p.updateDuration()
			p.pushEvent(busEvent{Kind: "duration_changed"})
		case C.GST_MESSAGE_CLOCK_LOST:
			p.mu.Lock()
			shouldRecover := !p.paused && p.pipeline != nil
			p.mu.Unlock()
			if shouldRecover {
				C.gst_element_set_state(p.pipeline, C.GST_STATE_PAUSED)
				C.gst_element_set_state(p.pipeline, C.GST_STATE_PLAYING)
			}
			p.pushEvent(busEvent{Kind: "clock_lost"})
		}
		C.gst_message_unref(msg)
	}
}

func (p *GStreamerPlayer) updateDuration() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pipeline == nil {
		return
	}
	var dur C.gint64
	if C.gst_element_query_duration(p.pipeline, C.GST_FORMAT_TIME, &dur) == 0 {
		return
	}
	p.duration = time.Duration(dur)
}

func (p *GStreamerPlayer) pushEvent(evt busEvent) {
	if p.events == nil {
		return
	}
	select {
	case p.events <- evt:
	default:
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
