//go:build native_media

package media

/*
#cgo pkg-config: libavcodec libavformat libswscale libavutil
#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libswscale/swscale.h>
#include <libavutil/imgutils.h>
#include <libavutil/hwcontext.h>
#include <libavutil/dict.h>

static AVChapter* getChapter(AVFormatContext *fmtCtx, int index) {
    if (fmtCtx == NULL || index < 0 || index >= fmtCtx->nb_chapters) {
        return NULL;
    }
    return fmtCtx->chapters[index];
}
*/
import "C"
import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"sync"
	"time"

	"git.leaktechnologies.dev/stu/VideoTools/internal/filters"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

type SeekAccuracy int

const (
	SeekAccuracyFrame SeekAccuracy = iota
	SeekAccuracyKeyframe
	SeekAccuracyAccurate
)

type BufferMode int

const (
	BufferModeMinimal BufferMode = iota
	BufferModeNormal
	BufferModeAggressive
)

type SeekFlags int

const (
	AVSEEK_FLAG_FRAME    = 0x01
	AVSEEK_FLAG_BACKWARD = 0x02
	AVSEEK_FLAG_ANY      = 0x04
	AVSEEK_FLAG_ACCURATE = 0x08
)

type VideoInfo struct {
	Width          int
	Height         int
	FrameRate      float64
	Duration       float64
	CodecName      string
	PixelFormat    string
	Bitrate        int64
	HasAudio       bool
	HasVideo       bool
	HasSubtitles   bool
	HWDevice       HWDeviceType
	AudioTracks    []StreamInfo
	SubtitleTracks []StreamInfo
	VideoTracks    []StreamInfo
}

type StreamInfo struct {
	Index     int
	CodecName string
	Language  string
	Title     string
}

type SubtitleOverlay struct {
	Text    string
	X       int
	Y       int
	Width   int
	Height  int
	Palette [4]byte
	Visible bool
}

func (s *SubtitleOverlay) Bounds() image.Rectangle {
	return image.Rect(s.X, s.Y, s.X+s.Width, s.Y+s.Height)
}

type HWDeviceType int

const (
	HWDeviceNone HWDeviceType = iota
	HWDeviceVAAPI
	HWDeviceD3D11VA
	HWDeviceQSV
)

func DetectHWDevice() HWDeviceType {
	if checkVAAPIAvailable() {
		return HWDeviceVAAPI
	}
	if checkD3D11VAAvailable() {
		return HWDeviceD3D11VA
	}
	if checkQSVAvailable() {
		return HWDeviceQSV
	}
	return HWDeviceNone
}

func checkVAAPIAvailable() bool {
	var devCtx *C.AVHWDeviceContext
	if C.av_hwdevice_ctx_create(&devCtx, C.AV_HWDEVICE_TYPE_VAAPI, nil, nil, 0) == 0 {
		if devCtx != nil {
			C.av_buffer_unref(&devCtx.hwctx)
		}
		return true
	}
	return false
}

func checkD3D11VAAvailable() bool {
	var devCtx *C.AVHWDeviceContext
	if C.av_hwdevice_ctx_create(&devCtx, C.AV_HWDEVICE_TYPE_D3D11VA, nil, nil, 0) == 0 {
		if devCtx != nil {
			C.av_buffer_unref(&devCtx.hwctx)
		}
		return true
	}
	return false
}

func checkQSVAvailable() bool {
	var devCtx *C.AVHWDeviceContext
	if C.av_hwdevice_ctx_create(&devCtx, C.AV_HWDEVICE_TYPE_QSV, nil, nil, 0) == 0 {
		if devCtx != nil {
			C.av_buffer_unref(&devCtx.hwctx)
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

	var devCtx *C.AVHWDeviceContext
	if C.av_hwdevice_ctx_create(&devCtx, hwType, nil, nil, 0) != 0 {
		return fmt.Errorf("failed to create HW device context")
	}

	e.hwDeviceCtx = devCtx

	framesCtx := C.av_hwdevice_alloc_frame_ctx(devCtx)
	if framesCtx == nil {
		C.av_buffer_unref(&devCtx.hwctx)
		return fmt.Errorf("failed to create HW frames context")
	}

	e.hwFramesCtx = framesCtx
	e.hwFramesCtx.width = C.uint(e.info.Width)
	e.hwFramesCtx.height = C.uint(e.info.Height)

	hwPixFmt := e.getHWPixelFormat(hwType)
	if hwPixFmt == C.AV_PIX_FMT_NONE {
		C.av_buffer_unref(&framesCtx.hwctx)
		return fmt.Errorf("unsupported pixel format for HW device")
	}

	e.hwFramesCtx.sw_format = hwPixFmt
	e.hwFramesCtx.initial_pool_size = 20

	if C.av_hwframe_ctx_init(framesCtx) != 0 {
		C.av_buffer_unref(&framesCtx.hwctx)
		return fmt.Errorf("failed to init HW frames context")
	}

	e.videoCodecCtx.hw_frames_ctx = C.av_buffer_ref(framesCtx)
	if e.videoCodecCtx.hw_frames_ctx == nil {
		C.av_buffer_unref(&framesCtx.hwctx)
		return fmt.Errorf("failed to attach HW frames context")
	}

	logging.Info(logging.CatPlayer, "HW decode enabled: %v", e.hwDevice)
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

func (e *Engine) initSubtitleDecoder(streams *[1 << 30]*C.AVStream) {
	if e.subtitleStreamIdx < 0 {
		return
	}

	stream := streams[e.subtitleStreamIdx]
	codec := C.avcodec_find_decoder(stream.codecpar.codec_id)
	if codec == nil {
		logging.Warning(logging.CatPlayer, "No subtitle decoder found for stream %d", e.subtitleStreamIdx)
		return
	}

	e.subtitleCodecCtx = C.avcodec_alloc_context3(codec)
	if e.subtitleCodecCtx == nil {
		logging.Warning(logging.CatPlayer, "Failed to allocate subtitle codec context")
		return
	}

	C.avcodec_parameters_to_context(e.subtitleCodecCtx, stream.codecpar)
	e.subtitleTimeBase = float64(stream.time_base.num) / float64(stream.time_base.den)

	if C.avcodec_open2(e.subtitleCodecCtx, codec, nil) < 0 {
		C.avcodec_free_context(&e.subtitleCodecCtx)
		e.subtitleCodecCtx = nil
		logging.Warning(logging.CatPlayer, "Failed to open subtitle codec")
		return
	}

	logging.Info(logging.CatPlayer, "Subtitle decoder initialized for stream %d", e.subtitleStreamIdx)
}

func (e *Engine) decodeSubtitle(pts float64) *SubtitleOverlay {
	if e.subtitleCodecCtx == nil {
		return nil
	}

	for {
		pkt, ok := e.subtitleQueue.Get()
		if !ok {
			return nil
		}
		defer C.av_packet_free(&pkt)

		if C.avcodec_send_packet(e.subtitleCodecCtx, pkt) != 0 {
			continue
		}

		var sub *C.AVSubtitle
		var gotSub C.int

		if C.avcodec_receive_subtitle(e.subtitleCodecCtx, sub, &gotSub) == 0 && gotSub == 1 {
			if sub.num_rects > 0 && sub.rects != nil {
				rect := sub.rects[0]
				if rect != nil && rect.type_ == C.AV_SUBTITLE_TYPE_TEXT {
					text := C.GoString(rect.text)
					e.currentSubtitle = &SubtitleOverlay{
						Text:    text,
						X:       int(rect.x),
						Y:       int(rect.y),
						Width:   int(rect.w),
						Height:  int(rect.h),
						Visible: true,
					}
					e.subtitleExpiry = float64(sub.end_display_time) / 1000.0
					C.avsubtitle_free(sub)
					return e.currentSubtitle
				}
			}
			C.avsubtitle_free(sub)
		}
	}
}

func (e *Engine) RenderSubtitles(img *image.RGBA, currentPTS float64) *image.RGBA {
	if e.currentSubtitle == nil || !e.currentSubtitle.Visible {
		return img
	}

	if currentPTS > e.subtitleExpiry {
		e.currentSubtitle = nil
		return img
	}

	bounds := e.currentSubtitle.Bounds()
	if !bounds.Intersects(img.Bounds()) {
		return img
	}

	bounds = bounds.Intersect(img.Bounds())

	alpha := byte(200)
	if e.subtitleBgAlpha > 0 && e.subtitleBgAlpha <= 255 {
		alpha = byte(e.subtitleBgAlpha)
	}

	subBg := &image.Uniform{color.RGBA{R: 0, G: 0, B: 0, A: alpha}}
	draw.Draw(img, bounds, subBg, image.Point{}, draw.Over)

	if e.currentSubtitle.Text != "" {
		e.drawSubtitleText(img, &bounds)
	}

	return img
}

func (e *Engine) drawSubtitleText(img *image.RGBA, bounds *image.Rectangle) {
	if e.currentSubtitle == nil || e.currentSubtitle.Text == "" {
		return
	}

	padding := 10
	charWidth := 16
	textWidth := len(e.currentSubtitle.Text) * charWidth
	textHeight := 32

	startX := bounds.Min.X + padding
	startY := bounds.Max.Y - textHeight - padding

	if startY < bounds.Min.Y {
		startY = bounds.Min.Y + padding
	}
	if startX+textWidth > bounds.Max.X {
		startX = bounds.Max.X - textWidth - padding
	}
	if startX < bounds.Min.X {
		startX = bounds.Min.X
	}

	e.drawBitmapText(img, e.currentSubtitle.Text, startX, startY)
}

func (e *Engine) drawBitmapText(img *image.RGBA, text string, x, y int) {
	bgColor := color.RGBA{R: 0, G: 0, B: 0, A: 180}

	for i, ch := range text {
		charX := x + i*16

		for py := 0; py < 32; py++ {
			for px := 0; px < 16; px++ {
				dx := charX + px
				dy := y + py

				if dx < img.Bounds().Min.X || dx >= img.Bounds().Max.X {
					continue
				}
				if dy < img.Bounds().Min.Y || dy >= img.Bounds().Max.Y {
					continue
				}

				on := e.isCharPixel(ch, px, py)
				if on {
					img.Set(dx, dy, color.White)
				}
			}
		}
	}
}

func (e *Engine) isCharPixel(ch rune, px, py int) bool {
	col := px / 4
	row := py / 4

	hash := (int(ch)*31 + col*7 + row*13) % 100

	switch {
	case ch >= 'A' && ch <= 'Z':
		return hash > 30
	case ch >= 'a' && ch <= 'z':
		return hash > 35
	case ch >= '0' && ch <= '9':
		return hash > 25
	case ch == ' ':
		return false
	case ch == '.' || ch == '!' || ch == '?':
		return py < 8
	case ch == ',' || ch == ';':
		return py > 20
	case ch == '-' || ch == '_':
		return py >= 14 && py < 18
	default:
		return hash > 40
	}
}

type PlaybackFrameCache struct {
	frames    map[int64]*image.RGBA
	frameList []int64
	maxSize   int
	mu        sync.RWMutex
}

func NewPlaybackFrameCache(maxSize int) *PlaybackFrameCache {
	if maxSize <= 0 {
		maxSize = 30
	}
	return &PlaybackFrameCache{
		frames:    make(map[int64]*image.RGBA),
		frameList: make([]int64, 0, maxSize),
		maxSize:   maxSize,
	}
}

func (c *PlaybackFrameCache) Add(pts float64, frame *image.RGBA) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := int64(pts * 1000)

	if len(c.frames) >= c.maxSize && len(c.frameList) > 0 {
		oldest := c.frameList[0]
		delete(c.frames, oldest)
		c.frameList = c.frameList[1:]
	}

	c.frames[key] = frame
	c.frameList = append(c.frameList, key)
}

func (c *PlaybackFrameCache) Get(pts float64) (*image.RGBA, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := int64(pts * 1000)
	frame, ok := c.frames[key]
	return frame, ok
}

func (c *PlaybackFrameCache) GetNearest(pts float64) (*image.RGBA, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := int64(pts * 1000)

	if frame, ok := c.frames[key]; ok {
		return frame, true
	}

	var nearestFrame *image.RGBA
	minDiff := int64(^uint64(0) >> 1)

	for cachedKey, frame := range c.frames {
		diff := cachedKey - key
		if diff < 0 {
			diff = -diff
		}
		if diff < minDiff {
			minDiff = diff
			nearestFrame = frame
		}
	}

	return nearestFrame, nearestFrame != nil
}

func (c *PlaybackFrameCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.frames = make(map[int64]*image.RGBA)
	c.frameList = c.frameList[:0]
}

func (c *PlaybackFrameCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.frames)
}

func NewEngine() *Engine {
	return &Engine{
		videoStreamIdx:    -1,
		audioStreamIdx:    -1,
		subtitleStreamIdx: -1,
		videoQueue:        NewPacketQueue(),
		audioQueue:        NewPacketQueue(),
		subtitleQueue:     NewPacketQueue(),
		clock:             NewMasterClock(),
		stop:              make(chan struct{}),
		volume:            1.0,
		speed:             1.0,
		seekAcc:           SeekAccuracyKeyframe,
		numThreads:        0,
		framePool:         make([][]byte, 0, 4),
	}
}

func (e *Engine) SetVolume(vol float32) {
	if vol < 0 {
		vol = 0
	}
	if vol > 1 {
		vol = 1
	}
	e.volume = vol
	if e.audioPlayer != nil {
		e.audioPlayer.SetVolume(vol)
	}
}

func (e *Engine) GetVolume() float32 {
	return e.volume
}

func (e *Engine) SetMuted(muted bool) {
	e.muted = muted
	if e.audioPlayer != nil {
		e.audioPlayer.SetMuted(muted)
	}
}

func (e *Engine) IsMuted() bool {
	return e.muted
}

func (e *Engine) InitFrameCache(maxSize int) {
	e.frameCache = NewPlaybackFrameCache(maxSize)
}

func (e *Engine) GetCachedFrame(pts float64) (*image.RGBA, bool) {
	if e.frameCache == nil {
		return nil, false
	}
	return e.frameCache.GetNearest(pts)
}

func (e *Engine) AddFrameToCache(pts float64, frame *image.RGBA) {
	if e.frameCache != nil && frame != nil {
		e.frameCache.Add(pts, frame)
	}
}

func (e *Engine) ClearFrameCache() {
	if e.frameCache != nil {
		e.frameCache.Clear()
	}
}

func (e *Engine) SetSpeed(speed float64) {
	if speed <= 0 {
		speed = 0.1
	}
	if speed > 4 {
		speed = 4
	}
	e.speed = speed
}

func (e *Engine) GetSpeed() float64 {
	return e.speed
}

func (e *Engine) SetLoading(loading bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.loading = loading
}

func (e *Engine) IsLoading() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.loading
}

func (e *Engine) GetChapters() []Chapter {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.chapters
}

func (e *Engine) parseChapters() {
	if e.formatCtx == nil {
		return
	}

	e.chapters = make([]Chapter, 0, e.formatCtx.nb_chapters)
	for i := 0; i < int(e.formatCtx.nb_chapters); i++ {
		chapter := C.getChapter(e.formatCtx, C.int(i))
		if chapter == nil {
			continue
		}

		startTime := float64(chapter.start) / float64(chapter.time_base.den) * float64(chapter.time_base.num)
		endTime := float64(chapter.end) / float64(chapter.time_base.den) * float64(chapter.time_base.num)

		c := Chapter{
			Index:     i,
			StartTime: startTime,
			EndTime:   endTime,
			Title:     "",
		}

		if chapter.metadata != nil {
			entry := C.av_dict_get(chapter.metadata, C.CString("title"), nil, 0)
			if entry != nil {
				c.Title = C.GoString(entry.value)
			}
		}

		e.chapters = append(e.chapters, c)
	}
}

func (e *Engine) SetSeekAccuracy(acc SeekAccuracy) {
	e.seekAcc = acc
}

func (e *Engine) GetSeekAccuracy() SeekAccuracy {
	return e.seekAcc
}

func (e *Engine) SetDropFrames(enabled bool) {
	e.dropFrames = enabled
}

func (e *Engine) IsDropFramesEnabled() bool {
	return e.dropFrames
}

func (e *Engine) SetLooping(looping bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.looping = looping
	if e.audioPlayer != nil {
		e.audioPlayer.SetLooping(looping)
	}
}

func (e *Engine) IsLooping() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.looping
}

func (e *Engine) SetBufferMode(mode BufferMode) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bufferMode = mode
}

func (e *Engine) GetBufferMode() BufferMode {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.bufferMode
}

func (e *Engine) GetAdaptiveBufferSize() int {
	e.mu.Lock()
	defer e.mu.Unlock()

	switch e.bufferMode {
	case BufferModeMinimal:
		return 10
	case BufferModeNormal:
		return 50
	case BufferModeAggressive:
		return 100
	default:
		return 50
	}
}

func (e *Engine) recordDecodeTime(duration time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.decodeTimes = append(e.decodeTimes, duration)
	if len(e.decodeTimes) > 30 {
		e.decodeTimes = e.decodeTimes[len(e.decodeTimes)-30:]
	}
	e.lastDecodeTime = time.Now()
}

func (e *Engine) GetAverageDecodeTime() time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.decodeTimes) == 0 {
		return 0
	}

	var total time.Duration
	for _, t := range e.decodeTimes {
		total += t
	}
	return total / time.Duration(len(e.decodeTimes))
}

func (e *Engine) SetFilterPipeline(pipeline *filters.FilterPipeline) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.filterPipeline = pipeline
}

func (e *Engine) GetFilterPipeline() *filters.FilterPipeline {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.filterPipeline
}

func (e *Engine) GetFilterGraph() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.filterPipeline == nil {
		return ""
	}
	graph, _ := e.filterPipeline.Generate()
	return graph
}

func (e *Engine) SetFilter(filterType filters.FilterType, params map[string]interface{}, enabled bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.filterPipeline == nil {
		e.filterPipeline = filters.NewFilterPipeline()
	}

	for i, f := range e.filterPipeline.Filters() {
		if f.Type == filterType {
			e.filterPipeline.Filters()[i].Params = params
			e.filterPipeline.Filters()[i].Enable = enabled
			return
		}
	}

	e.filterPipeline.Add(filters.FilterConfig{
		Type:   filterType,
		Params: params,
		Enable: enabled,
	})
}

func (e *Engine) EnableFilter(filterType filters.FilterType, enabled bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.filterPipeline != nil {
		e.filterPipeline.Enable(filterType, enabled)
	}
}

func (e *Engine) ClearFilters() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.filterPipeline != nil {
		e.filterPipeline.Clear()
	}
}

func (e *Engine) SetPreset(preset filters.Preset) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.filterPipeline == nil {
		e.filterPipeline = filters.NewFilterPipeline()
	}
	preset.Apply(e.filterPipeline)
}

func (e *Engine) SetGPUTextureUpload(upload interface{}) {
	e.gpuTexUpload = upload
}

func (e *Engine) GetGPUTextureUpload() interface{} {
	return e.gpuTexUpload
}

func (e *Engine) IsPaused() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.paused
}

func (e *Engine) Pause() {
	e.mu.Lock()
	if !e.running || e.paused {
		e.mu.Unlock()
		return
	}
	e.paused = true
	e.clock.SetPaused(true)
	e.mu.Unlock()

	if e.audioPlayer != nil {
		e.audioPlayer.Pause()
	}
	logging.Info(logging.CatPlayer, "Engine paused")
}

func (e *Engine) Resume() {
	e.mu.Lock()
	if !e.running || !e.paused {
		e.mu.Unlock()
		return
	}
	e.paused = false
	e.clock.SetPaused(false)
	e.mu.Unlock()

	if e.audioPlayer != nil {
		e.audioPlayer.Resume()
	}
	logging.Info(logging.CatPlayer, "Engine resumed")
}

func (e *Engine) TogglePause() {
	if e.IsPaused() {
		e.Resume()
	} else {
		e.Pause()
	}
}

func (e *Engine) Info() *VideoInfo {
	return e.info
}

func (e *Engine) GetAudioTracks() []StreamInfo {
	if e.info == nil {
		return nil
	}
	result := make([]StreamInfo, len(e.info.AudioTracks))
	copy(result, e.info.AudioTracks)
	return result
}

func (e *Engine) SelectAudioTrack(trackIndex int) error {
	if e.formatCtx == nil || e.info == nil {
		return fmt.Errorf("no media opened")
	}

	if trackIndex < 0 || trackIndex >= len(e.info.AudioTracks) {
		return fmt.Errorf("invalid audio track index: %d", trackIndex)
	}

	streams := (*[1 << 30]*C.AVStream)(unsafe.Pointer(e.formatCtx.streams))
	streamIdx := e.info.AudioTracks[trackIndex].Index

	codec := C.avcodec_find_decoder(streams[streamIdx].codecpar.codec_id)
	if codec == nil {
		return fmt.Errorf("no decoder found for audio stream")
	}

	e.audioQueue.Flush()

	if e.audioCodecCtx != nil {
		C.avcodec_free_context(&e.audioCodecCtx)
		e.audioCodecCtx = nil
	}

	e.audioCodecCtx = C.avcodec_alloc_context3(codec)
	if e.audioCodecCtx == nil {
		return fmt.Errorf("failed to allocate audio codec context")
	}

	C.avcodec_parameters_to_context(e.audioCodecCtx, streams[streamIdx].codecpar)
	e.audioTimeBase = float64(streams[streamIdx].time_base.num) / float64(streams[streamIdx].time_base.den)

	if C.avcodec_open2(e.audioCodecCtx, codec, nil) < 0 {
		C.avcodec_free_context(&e.audioCodecCtx)
		e.audioCodecCtx = nil
		return fmt.Errorf("failed to open audio codec")
	}

	if e.audioPlayer != nil {
		e.audioPlayer.Close()
	}

	ap, err := NewAudioPlayer(e.audioCodecCtx, e.audioQueue, e.clock, e.audioTimeBase)
	if err != nil {
		C.avcodec_free_context(&e.audioCodecCtx)
		e.audioCodecCtx = nil
		return fmt.Errorf("failed to create audio player: %w", err)
	}

	e.audioPlayer = ap
	e.audioPlayer.SetVolume(e.volume)
	e.audioPlayer.SetMuted(e.muted)
	e.audioStreamIdx = streamIdx
	e.hasAudio = true

	logging.Info(logging.CatPlayer, "Selected audio track %d", trackIndex)
	return nil
}

func (e *Engine) GetSubtitleTracks() []StreamInfo {
	if e.info == nil {
		return nil
	}
	result := make([]StreamInfo, len(e.info.SubtitleTracks))
	copy(result, e.info.SubtitleTracks)
	return result
}

func (e *Engine) SelectSubtitleTrack(trackIndex int) error {
	if e.formatCtx == nil || e.info == nil {
		return fmt.Errorf("no media opened")
	}

	if trackIndex < 0 || trackIndex >= len(e.info.SubtitleTracks) {
		return fmt.Errorf("invalid subtitle track index: %d", trackIndex)
	}

	e.subtitleStreamIdx = e.info.SubtitleTracks[trackIndex].Index
	logging.Info(logging.CatPlayer, "Selected subtitle track %d", trackIndex)
	return nil
}

func (e *Engine) DisableSubtitles() {
	e.subtitleStreamIdx = -1
	if e.subtitleCodecCtx != nil {
		C.avcodec_free_context(&e.subtitleCodecCtx)
		e.subtitleCodecCtx = nil
	}
	logging.Info(logging.CatPlayer, "Subtitles disabled")
}

func (e *Engine) GetVideoTracks() []StreamInfo {
	if e.info == nil {
		return nil
	}
	result := make([]StreamInfo, len(e.info.VideoTracks))
	copy(result, e.info.VideoTracks)
	return result
}

func (e *Engine) SelectVideoTrack(trackIndex int) error {
	if e.formatCtx == nil || e.info == nil {
		return fmt.Errorf("no media opened")
	}

	if trackIndex < 0 || trackIndex >= len(e.info.VideoTracks) {
		return fmt.Errorf("invalid video track index: %d", trackIndex)
	}

	streams := (*[1 << 30]*C.AVStream)(unsafe.Pointer(e.formatCtx.streams))
	streamIdx := e.info.VideoTracks[trackIndex].Index

	if streamIdx == e.videoStreamIdx {
		return nil
	}

	codec := C.avcodec_find_decoder(streams[streamIdx].codecpar.codec_id)
	if codec == nil {
		return fmt.Errorf("no decoder found for video stream")
	}

	e.videoQueue.Flush()

	if e.videoCodecCtx != nil {
		C.avcodec_free_context(&e.videoCodecCtx)
		e.videoCodecCtx = nil
	}

	e.videoCodecCtx = C.avcodec_alloc_context3(codec)
	if e.videoCodecCtx == nil {
		return fmt.Errorf("failed to allocate video codec context")
	}

	C.avcodec_parameters_to_context(e.videoCodecCtx, streams[streamIdx].codecpar)

	if e.numThreads > 0 {
		e.videoCodecCtx.thread_count = C.int(e.numThreads)
	}

	if C.avcodec_open2(e.videoCodecCtx, codec, nil) < 0 {
		C.avcodec_free_context(&e.videoCodecCtx)
		e.videoCodecCtx = nil
		return fmt.Errorf("failed to open video codec")
	}

	e.videoTimeBase = float64(streams[streamIdx].time_base.num) / float64(streams[streamIdx].time_base.den)
	e.videoStreamIdx = streamIdx

	logging.Info(logging.CatPlayer, "Selected video track %d", trackIndex)
	return nil
}

func (e *Engine) SetNumThreads(threads int) {
	if threads < 0 {
		threads = 0
	}
	e.numThreads = threads
	if e.videoCodecCtx != nil {
		e.videoCodecCtx.thread_count = C.int(threads)
	}
}

func (e *Engine) GetNumThreads() int {
	return e.numThreads
}

func (e *Engine) CheckCodecSupport(codecName string) bool {
	codec := C.avcodec_find_decoder_by_name(C.CString(codecName))
	if codec == nil {
		return false
	}

	pkt := C.av_packet_alloc()
	if pkt == nil {
		return false
	}
	defer C.av_packet_free(pkt)

	ctx := C.avcodec_alloc_context3(codec)
	if ctx == nil {
		return false
	}
	defer C.avcodec_free_context(ctx)

	if C.avcodec_open2(ctx, codec, nil) < 0 {
		return false
	}

	return true
}

func (e *Engine) Thumbnail(seconds float64) (*image.RGBA, error) {
	e.mu.Lock()
	origStreamIdx := e.videoStreamIdx
	origCodecCtx := e.videoCodecCtx
	origTimeBase := e.videoTimeBase
	e.mu.Unlock()

	if err := e.Seek(seconds); err != nil {
		return nil, err
	}

	img, err := e.NextFrame()
	if err != nil && err != io.EOF {
		return nil, err
	}

	e.mu.Lock()
	e.videoStreamIdx = origStreamIdx
	e.videoCodecCtx = origCodecCtx
	e.videoTimeBase = origTimeBase
	e.mu.Unlock()

	return img, nil
}

func (e *Engine) Open(path string) error {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	logging.Info(logging.CatPlayer, "Opening media file: %s", path)

	ret := C.avformat_open_input(&e.formatCtx, cPath, nil, nil)
	if ret != 0 {
		errBuf := make([]byte, 256)
		C.av_strerror(ret, (*C.char)(unsafe.Pointer(&errBuf[0])), 256)
		errStr := C.GoString((*C.char)(unsafe.Pointer(&errBuf[0])))
		if errStr == "" {
			errStr = "unknown FFmpeg error"
		}
		logging.Error(logging.CatPlayer, "avformat_open_input failed for %s: %s (code: %d)", path, errStr, ret)
		return fmt.Errorf("failed to open input file: %s", path)
	}

	ret := C.avformat_find_stream_info(e.formatCtx, nil)
	if ret < 0 {
		errBuf := make([]byte, 256)
		C.av_strerror(ret, (*C.char)(unsafe.Pointer(&errBuf[0])), 256)
		errStr := C.GoString((*C.char)(unsafe.Pointer(&errBuf[0])))
		logging.Error(logging.CatPlayer, "avformat_find_stream_info failed: %s (code: %d)", errStr, ret)
		C.avformat_close_input(&e.formatCtx)
		return fmt.Errorf("failed to find stream info: %s", errStr)
	}
	logging.Info(logging.CatPlayer, "avformat_find_stream_info succeeded")

	var videoCodec, audioCodec, subtitleCodec *C.AVCodec
	e.videoStreamIdx = int(C.av_find_best_stream(e.formatCtx, C.AVMEDIA_TYPE_VIDEO, -1, -1, &videoCodec, 0))
	e.audioStreamIdx = int(C.av_find_best_stream(e.formatCtx, C.AVMEDIA_TYPE_AUDIO, -1, -1, &audioCodec, 0))
	e.subtitleStreamIdx = int(C.av_find_best_stream(e.formatCtx, C.AVMEDIA_TYPE_SUBTITLE, -1, -1, &subtitleCodec, 0))
	logging.Info(logging.CatPlayer, "Streams found - video: %d, audio: %d, subtitle: %d",
		e.videoStreamIdx, e.audioStreamIdx, e.subtitleStreamIdx)

	streams := (*[1 << 30]*C.AVStream)(unsafe.Pointer(e.formatCtx.streams))

	e.info = &VideoInfo{
		Duration:       float64(e.formatCtx.duration) / float64(C.AV_TIME_BASE),
		Bitrate:        int64(e.formatCtx.bit_rate),
		HasVideo:       e.videoStreamIdx >= 0,
		HasAudio:       e.audioStreamIdx >= 0,
		HasSubtitles:   e.subtitleStreamIdx >= 0,
		HWDevice:       e.hwDevice,
		AudioTracks:    []StreamInfo{},
		SubtitleTracks: []StreamInfo{},
		VideoTracks:    []StreamInfo{},
	}

	for i := 0; i < int(e.formatCtx.nb_streams); i++ {
		stream := streams[i]
		if stream == nil {
			continue
		}

		mediaType := C.AVMediaType(stream.codecpar.codec_type)
		codec := C.avcodec_find_decoder(stream.codecpar.codec_id)
		if codec == nil {
			continue
		}
		codecName := C.GoString((*C.char)(unsafe.Pointer(codec.name)))

		var language, title string
		if stream.metadata != nil {
			entry := stream.metadata
			for entry != nil {
				key := C.GoString(entry.key)
				if key == "language" {
					language = C.GoString(entry.value)
				} else if key == "title" {
					title = C.GoString(entry.value)
				}
				entry = entry.next
			}
		}

		info := StreamInfo{
			Index:     i,
			CodecName: codecName,
			Language:  language,
			Title:     title,
		}

		if mediaType == C.AVMEDIA_TYPE_VIDEO {
			e.info.VideoTracks = append(e.info.VideoTracks, info)
		} else if mediaType == C.AVMEDIA_TYPE_AUDIO {
			e.info.AudioTracks = append(e.info.AudioTracks, info)
		} else if mediaType == C.AVMEDIA_TYPE_SUBTITLE {
			e.info.SubtitleTracks = append(e.info.SubtitleTracks, info)
		}
	}

	if e.videoStreamIdx < 0 {
		C.avformat_close_input(&e.formatCtx)
		return fmt.Errorf("no video stream found")
	}

	e.videoCodecCtx = C.avcodec_alloc_context3(videoCodec)
	if e.videoCodecCtx == nil {
		C.avformat_close_input(&e.formatCtx)
		return fmt.Errorf("failed to allocate video codec context")
	}
	C.avcodec_parameters_to_context(e.videoCodecCtx, streams[e.videoStreamIdx].codecpar)

	if e.numThreads > 0 {
		e.videoCodecCtx.thread_count = C.int(e.numThreads)
	}

	if e.hwDevice != HWDeviceNone {
		if err := e.initHWDecode(); err != nil {
			logging.Warning(logging.CatPlayer, "HW decode init failed, falling back to software: %v", err)
			e.hwDevice = HWDeviceNone
		}
	}

	e.videoTimeBase = float64(streams[e.videoStreamIdx].time_base.num) / float64(streams[e.videoStreamIdx].time_base.den)

	if e.subtitleStreamIdx >= 0 {
		e.initSubtitleDecoder(streams)
	}

	e.info.Width = int(e.videoCodecCtx.width)
	e.info.Height = int(e.videoCodecCtx.height)
	e.info.CodecName = C.GoString((*C.char)(unsafe.Pointer(e.videoCodecCtx.codec.name)))
	e.info.PixelFormat = C.GoString((*C.char)(unsafe.Pointer(av_get_pix_fmt_name(e.videoCodecCtx.pix_fmt))))

	avgFrameRate := streams[e.videoStreamIdx].avg_frame_rate
	if avgFrameRate.num > 0 {
		e.info.FrameRate = float64(avgFrameRate.num) / float64(avgFrameRate.den)
	}

	if C.avcodec_open2(e.videoCodecCtx, videoCodec, nil) < 0 {
		C.avcodec_free_context(&e.videoCodecCtx)
		C.avformat_close_input(&e.formatCtx)
		return fmt.Errorf("failed to open video codec")
	}

	if e.audioStreamIdx >= 0 {
		e.audioCodecCtx = C.avcodec_alloc_context3(audioCodec)
		if e.audioCodecCtx != nil {
			C.avcodec_parameters_to_context(e.audioCodecCtx, streams[e.audioStreamIdx].codecpar)
			e.audioTimeBase = float64(streams[e.audioStreamIdx].time_base.num) / float64(streams[e.audioStreamIdx].time_base.den)
			if C.avcodec_open2(e.audioCodecCtx, audioCodec, nil) < 0 {
				logging.Warning(logging.CatPlayer, "Failed to open audio codec")
				C.avcodec_free_context(&e.audioCodecCtx)
				e.audioCodecCtx = nil
				e.audioStreamIdx = -1
				e.info.HasAudio = false
			} else {
				ap, err := NewAudioPlayer(e.audioCodecCtx, e.audioQueue, e.clock, e.audioTimeBase)
				if err != nil {
					logging.Error(logging.CatPlayer, "Failed to create audio player: %v", err)
					C.avcodec_free_context(&e.audioCodecCtx)
					e.audioCodecCtx = nil
					e.audioStreamIdx = -1
					e.info.HasAudio = false
				} else {
					e.audioPlayer = ap
					e.audioPlayer.SetVolume(e.volume)
					e.audioPlayer.SetMuted(e.muted)
					e.hasAudio = true
				}
			}
		}
	}

	e.frame = C.av_frame_alloc()
	if e.frame == nil {
		e.Close()
		return fmt.Errorf("failed to allocate frame")
	}

	e.swsCtx = C.sws_getContext(
		e.videoCodecCtx.width, e.videoCodecCtx.height, e.videoCodecCtx.pix_fmt,
		e.videoCodecCtx.width, e.videoCodecCtx.height, C.AV_PIX_FMT_RGBA,
		C.SWS_BICUBIC|C.SWS_ACCURATE_RND, nil, nil, nil,
	)

	if e.swsCtx == nil {
		e.Close()
		return fmt.Errorf("failed to create sws context")
	}

	e.rgbaFrame = C.av_frame_alloc()
	numBytes := C.av_image_get_buffer_size(C.AV_PIX_FMT_RGBA, e.videoCodecCtx.width, e.videoCodecCtx.height, 1)
	e.rgbaBuffer = make([]byte, int(numBytes))
	C.av_image_fill_arrays(
		&e.rgbaFrame.data[0], &e.rgbaFrame.linesize[0],
		(*C.uint8_t)(unsafe.Pointer(&e.rgbaBuffer[0])), C.AV_PIX_FMT_RGBA,
		e.videoCodecCtx.width, e.videoCodecCtx.height, 1,
	)

	e.parseChapters()

	logging.Info(logging.CatPlayer, "Media opened: %dx%d @ %.2ffps, duration: %.2fs, chapters: %d",
		e.info.Width, e.info.Height, e.info.FrameRate, e.info.Duration, len(e.chapters))

	return nil
}

func (e *Engine) StartThumbnailExtraction(onFrame func(time float64, img *image.RGBA)) {
	go func() {
		defer logging.RecoverPanic()

		duration := e.Duration()
		if duration <= 0 {
			return
		}

		interval := 10.0
		if interval < 1 {
			interval = 1
		}

		thumbSize := 160
		thumbHeight := 90

		swsCtx := C.sws_getContext(
			e.videoCodecCtx.width, e.videoCodecCtx.height, e.videoCodecCtx.pix_fmt,
			C.int(thumbSize), C.int(thumbHeight), C.AV_PIX_FMT_RGBA,
			C.SWS_BICUBIC|C.SWS_ACCURATE_RND, nil, nil, nil,
		)
		if swsCtx == nil {
			return
		}
		defer C.sws_freeContext(swsCtx)

		thumbBuffer := make([]byte, thumbSize*thumbHeight*4)

		for t := 0.0; t < duration; t += interval {
			select {
			case <-e.stop:
				return
			default:
			}

			if err := e.Seek(t); err != nil {
				continue
			}

			img, err := e.NextFrame()
			if err != nil || img == nil {
				continue
			}

			if onFrame != nil {
				onFrame(t, img)
			}
		}
	}()
}

func av_get_pix_fmt_name(fmt C.enum_AVPixelFormat) *C.char {
	return C.av_get_pix_fmt_name(fmt)
}

func (e *Engine) Start() {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.paused = false
	e.mu.Unlock()

	go e.demuxerLoop()
}

func (e *Engine) demuxerLoop() {
	pkt := C.av_packet_alloc()
	defer C.av_packet_free(&pkt)

	for {
		select {
		case <-e.stop:
			return
		default:
			if C.av_read_frame(e.formatCtx, pkt) < 0 {
				e.videoQueue.SetEOF()
				e.audioQueue.SetEOF()
				e.subtitleQueue.SetEOF()
				return
			}

			streamIdx := int(pkt.stream_index)
			if streamIdx == e.videoStreamIdx {
				e.videoQueue.Put(pkt)
			} else if streamIdx == e.audioStreamIdx {
				e.audioQueue.Put(pkt)
			} else if streamIdx == e.subtitleStreamIdx && e.subtitleCodecCtx != nil {
				e.subtitleQueue.Put(pkt)
			}
			C.av_packet_unref(pkt)
		}
	}
}

func (e *Engine) Seek(seconds float64) error {
	logging.Info(logging.CatPlayer, "Seeking to %.2f seconds (accuracy: %v)", seconds, e.seekAcc)

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.formatCtx == nil || e.videoStreamIdx < 0 {
		return fmt.Errorf("no media opened")
	}

	target := C.int64_t(seconds / e.videoTimeBase)

	var flags C.int
	switch e.seekAcc {
	case SeekAccuracyFrame:
		flags = C.int(AVSEEK_FLAG_FRAME)
	case SeekAccuracyKeyframe:
		flags = 0
	case SeekAccuracyAccurate:
		flags = C.int(AVSEEK_FLAG_BACKWARD | AVSEEK_FLAG_ACCURATE)
	}

	if C.avformat_seek_file(e.formatCtx, C.int(e.videoStreamIdx), target, target, target, flags) < 0 {
		return fmt.Errorf("seek failed")
	}

	e.videoQueue.Flush()
	e.audioQueue.Flush()

	if e.audioPlayer != nil {
		e.audioPlayer.ResetEOF()
	}

	if e.videoCodecCtx != nil {
		C.avcodec_flush_buffers(e.videoCodecCtx)
	}
	if e.audioCodecCtx != nil {
		C.avcodec_flush_buffers(e.audioCodecCtx)
	}

	e.clock.SetTime(seconds)
	return nil
}

func (e *Engine) Step(frames int) (*image.RGBA, error) {
	if frames <= 0 {
		return nil, fmt.Errorf("invalid frame count")
	}

	var lastFrame *image.RGBA
	var err error
	for i := 0; i < frames; i++ {
		lastFrame, err = e.NextFrame()
		if err != nil {
			return nil, err
		}
	}
	return lastFrame, nil
}

func (e *Engine) NextFrame() (*image.RGBA, error) {
	for {
		e.mu.Lock()
		paused := e.paused
		hasAudio := e.hasAudio
		e.mu.Unlock()

		if paused {
			if hasAudio {
				e.clock.WaitForPTS(e.clock.GetTime())
			}
			continue
		}

		pkt, ok := e.videoQueue.Get()
		if !ok {
			if e.IsLooping() {
				if err := e.Seek(0); err != nil {
					return nil, err
				}
				continue
			}
			return nil, io.EOF
		}
		defer C.av_packet_free(&pkt)

		if C.avcodec_send_packet(e.videoCodecCtx, pkt) != 0 {
			logging.Debug(logging.CatPlayer, "Failed to send packet to video decoder")
			continue
		}

		for C.avcodec_receive_frame(e.videoCodecCtx, e.frame) == 0 {
			pts := float64(e.frame.pts) * e.videoTimeBase

			adjustedPts := pts * e.speed

			if hasAudio {
				e.clock.WaitForPTS(adjustedPts)
			} else {
				e.clock.SetTime(adjustedPts)
			}

			delay := e.clock.SyncVideo(adjustedPts)
			if delay < 0 {
				continue
			}

			var img *image.RGBA
			if e.hwDevice != HWDeviceNone {
				var err error
				img, err = e.retrieveHWFrame()
				if err != nil {
					logging.Warning(logging.CatPlayer, "HW frame retrieve failed: %v", err)
					img = e.toRGBA()
				}
			} else {
				img = e.toRGBA()
			}

			e.UpdateSubtitles(adjustedPts)
			if e.subtitleCodecCtx != nil {
				sub := e.decodeSubtitle(adjustedPts)
				if sub != nil {
					img = e.RenderSubtitles(img, adjustedPts)
				}
			}

			return img, nil
		}
	}
}

func (e *Engine) retrieveHWFrame() (*image.RGBA, error) {
	if e.frame.hw_frames_ctx == nil {
		return e.toRGBA(), nil
	}

	swFrame := C.av_frame_alloc()
	if swFrame == nil {
		return nil, fmt.Errorf("failed to allocate sw frame")
	}
	defer C.av_frame_free(&swFrame)

	if C.av_hwframe_get_buffer(e.hwFramesCtx, swFrame, 0) != 0 {
		return nil, fmt.Errorf("failed to get HW frame buffer")
	}

	swFrame.pict_type = 0

	if C.av_hwframe_transfer_data(swFrame, e.frame, 0) != 0 {
		return nil, fmt.Errorf("failed to transfer HW frame to SW")
	}

	origFrame := e.frame
	e.frame = swFrame
	img := e.toRGBA()
	e.frame = origFrame

	return img, nil
}

func (e *Engine) toRGBA() *image.RGBA {
	C.sws_scale(
		e.swsCtx,
		&e.frame.data[0], &e.frame.linesize[0],
		0, e.videoCodecCtx.height,
		&e.rgbaFrame.data[0], &e.rgbaFrame.linesize[0],
	)

	w, h := int(e.videoCodecCtx.width), int(e.videoCodecCtx.height)

	e.mu.Lock()
	var img *image.RGBA
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
	e.mu.Unlock()

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

	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.framePool) < 4 {
		buf := make([]byte, len(img.Pix))
		copy(buf, img.Pix)
		e.framePool = append(e.framePool, buf)
	}
}

func (e *Engine) GetFramePoolSize() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.framePool)
}

func (e *Engine) Duration() float64 {
	if e.formatCtx == nil {
		return 0
	}
	return float64(e.formatCtx.duration) / float64(C.AV_TIME_BASE)
}

func (e *Engine) CurrentTime() float64 {
	return e.clock.GetTime()
}

type PlaybackError struct {
	Code    string
	Message string
	Retry   bool
}

const (
	ErrCodeDecode       = "DECODE_ERROR"
	ErrCodeNetwork      = "NETWORK_ERROR"
	ErrCodeHWAccel      = "HW_ACCEL_ERROR"
	ErrCodeFileCorrupt  = "FILE_CORRUPT"
	ErrCodeCodecMissing = "CODEC_MISSING"
)

func (e *Engine) RecoverableError(code, message string) *PlaybackError {
	return &PlaybackError{
		Code:    code,
		Message: message,
		Retry:   code == ErrCodeNetwork || code == ErrCodeDecode,
	}
}

func (e *Engine) ShouldRetry(err *PlaybackError) bool {
	return err != nil && err.Retry
}

func (e *Engine) Close() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return
	}
	close(e.stop)
	e.running = false
	e.paused = false
	e.mu.Unlock()

	e.videoQueue.Close()
	e.audioQueue.Close()

	if e.audioPlayer != nil {
		e.audioPlayer.Close()
		e.audioPlayer = nil
	}

	if e.swsCtx != nil {
		C.sws_freeContext(e.swsCtx)
		e.swsCtx = nil
	}
	if e.videoCodecCtx != nil {
		if e.videoCodecCtx.hw_frames_ctx != nil {
			C.av_buffer_unref(&e.videoCodecCtx.hw_frames_ctx)
		}
		C.avcodec_free_context(&e.videoCodecCtx)
		e.videoCodecCtx = nil
	}
	if e.audioCodecCtx != nil {
		C.avcodec_free_context(&e.audioCodecCtx)
		e.audioCodecCtx = nil
	}
	if e.subtitleCodecCtx != nil {
		C.avcodec_free_context(&e.subtitleCodecCtx)
		e.subtitleCodecCtx = nil
	}
	if e.subtitleQueue != nil {
		e.subtitleQueue.Close()
	}
	if e.hwFramesCtx != nil {
		C.av_buffer_unref(&e.hwFramesCtx.hwctx)
		e.hwFramesCtx = nil
	}
	if e.hwDeviceCtx != nil {
		C.av_buffer_unref(&e.hwDeviceCtx.hwctx)
		e.hwDeviceCtx = nil
	}
	if e.formatCtx != nil {
		C.avformat_close_input(&e.formatCtx)
		e.formatCtx = nil
	}
	if e.frame != nil {
		C.av_frame_free(&e.frame)
		e.frame = nil
	}
	if e.rgbaFrame != nil {
		C.av_frame_free(&e.rgbaFrame)
		e.rgbaFrame = nil
	}

	logging.Info(logging.CatPlayer, "Engine closed")
}

func (e *Engine) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

func (e *Engine) QueueStats() (videoSize, audioSize int) {
	if e.videoQueue != nil {
		videoSize = e.videoQueue.Size()
	}
	if e.audioQueue != nil {
		audioSize = e.audioQueue.Size()
	}
	return
}
