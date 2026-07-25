//go:build native_media && vlc

package media

/*
#cgo windows LDFLAGS: -LC:/vlc/lib -llibvlc
#cgo windows CFLAGS: -IC:/vlc/include
#cgo linux pkg-config: libvlc

#include "vlc_glue.h"
#include <stdlib.h>
#include <string.h>
*/
import "C"

import (
	"errors"
	"fmt"
	"image"
	"io"
	"sync"
	"time"
	"unsafe"

	"github.com/LeakTechnologies/VideoTools/internal/logging"
	"github.com/LeakTechnologies/VideoTools/internal/media/filters"
)

// vlcEngine implements PlaybackEngine using libVLC for demux, decode, and A/V sync.
type vlcEngine struct {
	instance *C.libvlc_instance_t
	player   *C.libvlc_media_player_t
	media    *C.libvlc_media_t

	// Frame delivery: libVLC callback writes into frameBuf; display copies to frameCh.
	mu       sync.Mutex
	frameBuf []byte          // RGBA pixel buffer (libVLC writes here via lock callback)
	frameW   int             // current frame width
	frameH   int             // current frame height
	frameCh  chan *image.RGBA // decoded frames ready for consumption
	lastFrame *image.RGBA     // most recently decoded frame (for GrabFrame)

	// State
	duration  float64
	frameRate float64
	paused    bool
	running   bool
	seeked    bool // set on seek, cleared after next frame decode

	// Callbacks
	onProgress func(float64)
	onEOF      func()

	// Chapters
	chapters []Chapter

	// Titles
	titles       []Title
	currentTitle int

	// Settings (no-ops for VLC, stored for API compatibility)
	seekAccuracy   SeekAccuracy
	audioDelay     float64
	deinterlace    bool
	growingFile    bool
	abLoopEnabled  bool
	loopA, loopB   float64
	dropFrames     bool
	hwDevice       HWDeviceType
	filterPipeline *filters.FilterPipeline
}

// NewVLCEngine creates a new libVLC-based playback engine.
func NewVLCEngine() PlaybackEngine {
	args := []string{
		"--no-xlib",             // headless — no X11 display (Linux)
		"--verbose=0",           // minimal logging
		"--no-video-title-show", // no overlay text
		"--no-autoscale",
	}

	cArgs := make([]*C.char, len(args))
	for i, a := range args {
		cArgs[i] = C.CString(a)
	}
	defer func() {
		for _, a := range cArgs {
			C.free(unsafe.Pointer(a))
		}
	}()

	instance := C.libvlc_new(C.int(len(args)), &cArgs[0])
	if instance == nil {
		logging.Error(logging.CatPlayer, "VLC: libvlc_new failed")
		return nil
	}

	eng := &vlcEngine{
		instance: instance,
		frameCh:  make(chan *image.RGBA, 8),
		frameBuf: make([]byte, 0),
	}

	logging.Info(logging.CatPlayer, "VLC: engine created")
	return eng
}

// NewScrubber returns nil — VLC handles seeking internally via
// libvlc_media_player_set_position. No separate scrubber needed.
func (e *vlcEngine) NewScrubber() Scrubber { return nil }

// --- Lifecycle ---

func (e *vlcEngine) Close() {
	if e.player != nil {
		// Detach events before stopping to avoid callbacks into a freed engine.
		C.vlcDetachEvents(e.player, unsafe.Pointer(&vlcFrameCtx{engine: e}))
		C.libvlc_media_player_stop(e.player)
		C.libvlc_media_player_release(e.player)
		e.player = nil
	}
	if e.media != nil {
		C.libvlc_media_release(e.media)
		e.media = nil
	}
	if e.instance != nil {
		C.libvlc_release(e.instance)
		e.instance = nil
	}
	close(e.frameCh)
	logging.Info(logging.CatPlayer, "VLC: engine closed")
}

func (e *vlcEngine) Start() {
	if e.player == nil {
		return
	}
	C.libvlc_media_player_play(e.player)
	e.running = true
	e.paused = false
	logging.Info(logging.CatPlayer, "VLC: playback started")
}

// --- Open ---

func (e *vlcEngine) OpenAuto(path string) error {
	return e.openPath(path)
}

func (e *vlcEngine) OpenDVD(devicePath string, title int) error {
	// libVLC handles DVDs natively — pass the path and let it auto-detect.
	// For title selection, we parse the title list after open.
	return e.openPath(devicePath)
}

func (e *vlcEngine) OpenURL(url string, opts map[string]string) error {
	cURL := C.CString(url)
	defer C.free(unsafe.Pointer(cURL))

	e.media = C.libvlc_media_new_location(e.instance, cURL)
	if e.media == nil {
		return fmt.Errorf("VLC: failed to create media for URL: %s", url)
	}
	return e.setupPlayer()
}

func (e *vlcEngine) openPath(path string) error {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	e.media = C.libvlc_media_new_path(e.instance, cPath)
	if e.media == nil {
		return fmt.Errorf("VLC: failed to create media for: %s", path)
	}
	return e.setupPlayer()
}

func (e *vlcEngine) setupPlayer() error {
	e.player = C.libvlc_media_player_new_from_media(e.media)
	if e.player == nil {
		C.libvlc_media_release(e.media)
		e.media = nil
		return errors.New("VLC: failed to create media player")
	}

	// Create frame context for video callbacks.
	ctx := &vlcFrameCtx{
		engine: e,
		width:  1920, // default; updated by format callback
		height: 1080,
		stride: 1920 * 4,
	}
	ctx.pixels = make([]byte, ctx.stride*ctx.height)

	// Set up format callbacks (negotiate RGBA pixel format).
	C.vlcSetFormatCallbacks(e.player, unsafe.Pointer(ctx))

	// Set up video callbacks for frame delivery.
	C.vlcSetCallbacks(e.player, unsafe.Pointer(ctx))

	// Attach event handlers (EOF, position, error).
	C.vlcAttachEvents(e.player, unsafe.Pointer(ctx))

	// Parse media to get duration, chapters, and track info.
	C.libvlc_media_parse(e.media)

	// Get duration.
	dur := C.libvlc_media_get_duration(e.media)
	e.duration = float64(dur) / 1000.0

	// Extract frame rate from video track.
	e.frameRate = e.extractFrameRate()

	// Parse chapters from libVLC.
	e.parseChapters()

	// Parse titles from libVLC.
	e.parseTitles()

	logging.Info(logging.CatPlayer, "VLC: media opened, duration=%.2fs, chapters=%d, titles=%d, fps=%.2f",
		e.duration, len(e.chapters), len(e.titles), e.frameRate)
	return nil
}

// extractFrameRate reads the FPS from the first video track's track description.
func (e *vlcEngine) extractFrameRate() float64 {
	if e.player == nil {
		return 25.0
	}
	// libVLC 3.0 exposes fps via media track info after parse.
	// We use libvlc_media_get_stats as a fallback — or return a default.
	// The format callback could also negotiate fps, but libVLC doesn't
	// expose it directly in the C API without track enumeration.
	// For now, return a reasonable default; VLC syncs internally regardless.
	return 25.0
}

// parseChapters extracts chapter info from the parsed media via libVLC.
func (e *vlcEngine) parseChapters() {
	if e.player == nil {
		return
	}

	count := C.libvlc_media_player_get_chapter_count(e.player)
	if count <= 0 {
		return
	}

	var descs **C.libvlc_chapter_description_t
	n := C.libvlc_media_player_get_full_chapter_descriptions(e.player, -1, &descs)
	if n <= 0 || descs == nil {
		return
	}
	defer C.libvlc_chapter_descriptions_release(n, descs)

	// descs is a C array of pointers; iterate using unsafe pointer arithmetic.
	e.chapters = make([]Chapter, 0, n)
	for i := 0; i < int(n); i++ {
		// Each element is a *libvlc_chapter_description_t* (pointer to pointer).
		p := (*C.libvlc_chapter_description_t)(unsafe.Pointer(
			uintptr(unsafe.Pointer(descs)) + uintptr(i)*unsafe.Sizeof(*descs),
		))
		if p == nil || *p == nil {
			continue
		}
		ch := *p
		c := Chapter{
			Index:     i,
			StartTime: float64(ch.i_time_offset) / 1_000_000.0, // microseconds → seconds
			EndTime:   float64(ch.i_time_offset+ch.i_duration) / 1_000_000.0,
		}
		if ch.psz_name != nil {
			c.Title = C.GoString(ch.psz_name)
		}
		if c.Title == "" {
			c.Title = fmt.Sprintf("Chapter %d", i+1)
		}
		e.chapters = append(e.chapters, c)
	}
}

// parseTitles extracts title info from the parsed media via libVLC.
func (e *vlcEngine) parseTitles() {
	if e.player == nil {
		return
	}

	count := C.libvlc_media_player_get_title_count(e.player)
	if count <= 0 {
		return
	}

	var descs **C.libvlc_title_description_t
	n := C.libvlc_media_player_get_full_title_descriptions(e.player, &descs)
	if n <= 0 || descs == nil {
		return
	}
	defer C.libvlc_title_descriptions_release(n, descs)

	e.titles = make([]Title, 0, n)
	for i := 0; i < int(n); i++ {
		p := (*C.libvlc_title_description_t)(unsafe.Pointer(
			uintptr(unsafe.Pointer(descs)) + uintptr(i)*unsafe.Sizeof(*descs),
		))
		if p == nil || *p == nil {
			continue
		}
		t := *p
		title := Title{
			Index:    i,
			Duration: float64(t.i_duration) / 1_000_000.0, // microseconds → seconds
			IsMenu:   (t.i_flags & C.libvlc_title_menu) != 0,
		}
		if t.psz_name != nil {
			title.Name = C.GoString(t.psz_name)
		}
		if title.Name == "" {
			title.Name = fmt.Sprintf("Title %d", i+1)
		}
		e.titles = append(e.titles, title)
	}
}

// --- Playback control ---

func (e *vlcEngine) Pause() {
	if e.player == nil {
		return
	}
	C.libvlc_media_player_pause(e.player)
	e.paused = true
}

func (e *vlcEngine) Resume() {
	if e.player == nil {
		return
	}
	C.libvlc_media_player_play(e.player)
	e.paused = false
	e.running = true
}

func (e *vlcEngine) IsPaused() bool { return e.paused }

func (e *vlcEngine) IsRunning() bool { return e.running }

func (e *vlcEngine) Seek(seconds float64) error {
	if e.player == nil {
		return nil
	}
	ms := C.int(seconds * 1000)
	C.libvlc_media_player_set_time(e.player, ms)
	e.seeked = true
	return nil
}

func (e *vlcEngine) NextFrame() (*image.RGBA, error) {
	if e.player == nil {
		return nil, io.EOF
	}

	// Check if playback has ended.
	if C.libvlc_media_player_get_state(e.player) == C.libvlc_media_state_end {
		return nil, io.EOF
	}

	// Wait for a frame from the display callback (up to 5s).
	select {
	case img := <-e.frameCh:
		if img == nil {
			return nil, io.EOF
		}
		e.lastFrame = img
		e.seeked = false
		return img, nil
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("VLC: timeout waiting for frame")
	}
}

func (e *vlcEngine) Step(frames int) (*image.RGBA, error) {
	if e.player == nil {
		return nil, io.EOF
	}
	// VLC doesn't have native frame stepping; seek forward by ~40ms per frame.
	dt := float64(frames) * 0.04
	ms := C.libvlc_media_player_get_time(e.player)
	target := float64(ms)/1000.0 + dt
	if target < 0 {
		target = 0
	}
	C.libvlc_media_player_set_time(e.player, C.int(target*1000))
	return e.NextFrame()
}

func (e *vlcEngine) GrabFrame(timeout time.Duration) (*image.RGBA, error) {
	if e.player == nil {
		return nil, errors.New("VLC: no player")
	}

	// For GrabFrame, we need to play briefly to decode a frame, then pause.
	wasPaused := e.paused
	if !e.running {
		e.Start()
	}

	// Wait for a frame.
	select {
	case img := <-e.frameCh:
		if !wasPaused {
			e.Pause()
		}
		if img != nil {
			e.lastFrame = img
		}
		return img, nil
	case <-time.After(timeout):
		if !wasPaused {
			e.Pause()
		}
		return nil, fmt.Errorf("VLC: GrabFrame timeout after %v", timeout)
	}
}

func (e *vlcEngine) ResetAfterGrab() {
	if e.player == nil {
		return
	}
	C.libvlc_media_player_set_time(e.player, 0)
}

func (e *vlcEngine) WaitForFrame(timeout time.Duration) bool {
	select {
	case <-e.frameCh:
		return true
	case <-time.After(timeout):
		return false
	}
}

// --- Audio ---

func (e *vlcEngine) SetVolume(vol float32) {
	if e.player == nil {
		return
	}
	C.libvlc_audio_set_volume(e.player, C.int(vol*100))
}

func (e *vlcEngine) SetMuted(muted bool) {
	if e.player == nil {
		return
	}
	C.libvlc_audio_set_mute(e.player, boolToCInt(muted))
}

func (e *vlcEngine) DrainAudio()     {}
func (e *vlcEngine) FlushAudioCodec() {}

// --- Speed / timing ---

func (e *vlcEngine) SetSpeed(speed float64) {
	if e.player == nil {
		return
	}
	C.libvlc_media_player_set_rate(e.player, C.float(speed))
}

func (e *vlcEngine) CurrentTime() float64 {
	if e.player == nil {
		return 0
	}
	ms := C.libvlc_media_player_get_time(e.player)
	return float64(ms) / 1000.0
}

func (e *vlcEngine) Duration() float64 { return e.duration }

func (e *vlcEngine) GetFrameRate() float64 { return e.frameRate }

// --- Seeking / accuracy ---

func (e *vlcEngine) SetSeekAccuracy(acc SeekAccuracy) { e.seekAccuracy = acc }
func (e *vlcEngine) SetAudioDelay(d float64)          { e.audioDelay = d }

// --- Deinterlace / growing file / AB loop ---

func (e *vlcEngine) SetDeinterlaceEnabled(enabled bool) { e.deinterlace = enabled }
func (e *vlcEngine) SetGrowingFile(growing bool)        { e.growingFile = growing }
func (e *vlcEngine) IsGrowingFile() bool                { return e.growingFile }
func (e *vlcEngine) SetABLoopEnabled(enabled bool)      { e.abLoopEnabled = enabled }
func (e *vlcEngine) SetLoopPoints(a, b float64)         { e.loopA = a; e.loopB = b }

// --- Frame cache ---

func (e *vlcEngine) InitFrameCache(maxSize int) {} // VLC manages its own buffers

// --- Chapters ---

func (e *vlcEngine) GetChapters() []Chapter {
	if len(e.chapters) == 0 {
		return nil
	}
	return e.chapters
}

// --- Titles ---

func (e *vlcEngine) GetTitles() []Title {
	if len(e.titles) == 0 {
		return nil
	}
	return e.titles
}

func (e *vlcEngine) SelectTitle(index int) error {
	if e.player == nil {
		return nil
	}
	if index < 0 || index >= len(e.titles) {
		return fmt.Errorf("VLC: invalid title index %d", index)
	}
	C.libvlc_media_player_set_title(e.player, C.int(index))
	e.currentTitle = index
	logging.Info(logging.CatPlayer, "VLC: selected title %d", index)
	return nil
}

func (e *vlcEngine) GetCurrentTitle() int {
	if e.player == nil {
		return 0
	}
	return int(C.libvlc_media_player_get_title(e.player))
}

// --- Tracks ---

func (e *vlcEngine) GetAudioTracks() []StreamInfo {
	if e.player == nil {
		return nil
	}
	count := int(C.libvlc_audio_get_track_count(e.player))
	if count <= 0 {
		return nil
	}
	descs := C.libvlc_audio_get_track_descriptions(e.player)
	if descs == nil {
		return nil
	}
	defer C.libvlc_track_description_list_release(descs)

	tracks := make([]StreamInfo, 0, count)
	for i := 0; i < count; i++ {
		d := getTrackDesc(descs, i)
		if d == nil {
			continue
		}
		id := int(C.libvlc_track_description_get_id(d))
		name := C.GoString(C.libvlc_track_description_get_name(d))
		tracks = append(tracks, StreamInfo{
			Index:     id,
			CodecName: "",
			Language:  name,
			Title:     name,
		})
	}
	return tracks
}

func (e *vlcEngine) SelectAudioTrack(trackIndex int) error {
	if e.player == nil {
		return nil
	}
	ret := C.libvlc_audio_set_track(e.player, C.int(trackIndex))
	if ret != 0 {
		return fmt.Errorf("VLC: failed to select audio track %d", trackIndex)
	}
	return nil
}

func (e *vlcEngine) GetSubtitleTracks() []StreamInfo {
	if e.player == nil {
		return nil
	}
	count := int(C.libvlc_video_get_spu_count(e.player))
	if count <= 0 {
		return nil
	}
	descs := C.libvlc_video_get_spu_descriptions(e.player)
	if descs == nil {
		return nil
	}
	defer C.libvlc_track_description_list_release(descs)

	tracks := make([]StreamInfo, 0, count)
	for i := 0; i < count; i++ {
		d := getTrackDesc(descs, i)
		if d == nil {
			continue
		}
		id := int(C.libvlc_track_description_get_id(d))
		name := C.GoString(C.libvlc_track_description_get_name(d))
		tracks = append(tracks, StreamInfo{
			Index:     id,
			CodecName: "",
			Language:  name,
			Title:     name,
		})
	}
	return tracks
}

func (e *vlcEngine) SelectSubtitleTrack(trackIndex int) error {
	if e.player == nil {
		return nil
	}
	ret := C.libvlc_video_set_spu(e.player, C.int(trackIndex))
	if ret != 0 {
		return fmt.Errorf("VLC: failed to select subtitle track %d", trackIndex)
	}
	return nil
}

func (e *vlcEngine) DisableSubtitles() {
	if e.player == nil {
		return
	}
	C.libvlc_video_set_spu(e.player, 0)
}

// --- Filters ---

func (e *vlcEngine) SetFilterPipeline(pipeline *filters.FilterPipeline) {
	e.filterPipeline = pipeline
}

// --- HW decode ---

func (e *vlcEngine) SetHWDevice(hw HWDeviceType) { e.hwDevice = hw }

// --- PTS diagnostics ---

func (e *vlcEngine) GetLastVideoPTS() float64 {
	if e.player == nil {
		return 0
	}
	return e.CurrentTime()
}

func (e *vlcEngine) GetLastAudioPTS() float64 {
	return e.GetLastVideoPTS() // VLC keeps A/V sync internally
}

// --- Drop frames ---

func (e *vlcEngine) SetDropFrames(enabled bool) { e.dropFrames = enabled }

// --- Thumbnail extraction ---

func (e *vlcEngine) StartThumbnailExtraction(onFrame func(time float64, img *image.RGBA)) {
	// Phase 2: iterate seek positions and grab frames.
	// For Phase 1, thumbnails are not extracted.
}

// --- Event handling (called from C callbacks) ---

//go:linkname vlcOnEOF
func (e *vlcEngine) vlcOnEOF() {
	e.running = false
	if e.onEOF != nil {
		e.onEOF()
	}
}

// --- Helpers ---

func boolToCInt(b bool) C.int {
	if b {
		return 1
	}
	return 0
}

func getTrackDesc(first *C.libvlc_track_description_t, index int) *C.libvlc_track_description_t {
	cur := first
	for i := 0; i < index; i++ {
		if cur.p_next == nil {
			return nil
		}
		cur = cur.p_next
	}
	return cur
}

// Ensure vlcEngine satisfies PlaybackEngine at compile time.
var _ PlaybackEngine = (*vlcEngine)(nil)
