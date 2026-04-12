//go:build native_media

package media

/*
#cgo !windows pkg-config: libavcodec libavformat libswscale libavutil
#cgo windows CFLAGS: -IC:/ffmpeg/include
#cgo windows LDFLAGS: -LC:/ffmpeg/lib -lavcodec -lavformat -lavutil -lswscale -lswresample -lavfilter -lbcrypt -lSecur32 -lWs2_32 -lmfplat -lstrmiids -lavrt -lole32 -luser32
#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libswscale/swscale.h>
#include <libavutil/avutil.h>
#include <libavutil/imgutils.h>
#include <libavutil/hwcontext.h>
#include <libavutil/dict.h>

// VT_SUBTITLE_TYPE_TEXT — stable numeric value of the "plain text" subtitle
// rect type (has been 2 in every FFmpeg release).  Using a macro avoids
// AV_SUBTITLE_TYPE_TEXT / SUBTITLE_TEXT rename churn across versions.
#define VT_SUBTITLE_TYPE_TEXT 2

// vt_sub_rect0 — safely returns the first AVSubtitleRect* from a subtitle.
static AVSubtitleRect* vt_sub_rect0(AVSubtitle *sub) {
    if (sub == NULL || sub->num_rects == 0 || sub->rects == NULL) return NULL;
    return sub->rects[0];
}
// vt_sub_rect_type — reads the type field (Go keyword; enum field access unreliable via CGO).
static int vt_sub_rect_type(AVSubtitleRect *rect) {
    if (rect == NULL) return -1;
    return (int)rect->type;
}

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
	"unsafe"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/media/filters"
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

	// Attach hw_device_ctx; FFmpeg will set up frames context internally at
	// avcodec_open2 time.  We keep our own ref in e.hwDeviceCtx for cleanup.
	e.videoCodecCtx.hw_device_ctx = C.av_buffer_ref(devCtxRef)
	if e.videoCodecCtx.hw_device_ctx == nil {
		C.av_buffer_unref(&devCtxRef)
		return fmt.Errorf("failed to attach HW device context to codec ctx")
	}
	e.hwDeviceCtx = devCtxRef

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

		var sub C.AVSubtitle
		var gotSub C.int

		if C.avcodec_decode_subtitle2(e.subtitleCodecCtx, &sub, &gotSub, pkt) >= 0 && gotSub == 1 {
			rect := C.vt_sub_rect0(&sub)
			// AV_SUBTITLE_TYPE_TEXT = 2 (has been stable since FFmpeg 0.6)
			if rect != nil && C.vt_sub_rect_type(rect) == 2 {
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
				C.avsubtitle_free(&sub)
				return e.currentSubtitle
			}
			C.avsubtitle_free(&sub)
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
	if !bounds.Overlaps(img.Bounds()) {
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

type Chapter struct {
	Index     int
	StartTime float64
	EndTime   float64
	Title     string
}

type Engine struct {
	filePath          string
	formatCtx         *C.AVFormatContext
	videoStreamIdx    int
	audioStreamIdx    int
	subtitleStreamIdx int
	videoCodecCtx     *C.AVCodecContext
	audioCodecCtx     *C.AVCodecContext
	subtitleCodecCtx  *C.AVCodecContext
	swsCtx            *C.struct_SwsContext

	// hwDeviceCtx and hwFramesCtx are AVBufferRef wrappers — the standard
	// FFmpeg ownership model.  av_hwdevice_ctx_create writes to *AVBufferRef
	// and av_hwframe_ctx_alloc returns one; both are freed with av_buffer_unref.
	hwDeviceCtx *C.AVBufferRef
	hwFramesCtx *C.AVBufferRef

	videoQueue    *PacketQueue
	audioQueue    *PacketQueue
	subtitleQueue *PacketQueue

	audioPlayer *AudioPlayer
	clock       *MasterClock

	frame *C.AVFrame

	rgbaFrame  *C.AVFrame
	rgbaBuffer []byte

	framePool [][]byte

	videoTimeBase    float64
	audioTimeBase    float64
	subtitleTimeBase float64

	mu           sync.Mutex
	formatMu     sync.Mutex // serialises av_read_frame vs avformat_seek_file
	videoCodecMu sync.Mutex // serialises avcodec_send_packet / avcodec_receive_frame on videoCodecCtx
	running   bool
	paused    bool
	stop      chan struct{}
	loading   bool

	volume  float32
	muted   bool
	speed   float64
	seekAcc SeekAccuracy

	dropFrames       bool
	consecutiveDrops int

	bufferMode     BufferMode
	lastDecodeTime time.Time
	decodeTimes    []time.Duration

	hwDevice      HWDeviceType
	hwDegraded    bool
	videoDecoded  bool // set true after first successful video frame decode
	hwFailCount int

	looping  bool
	hasAudio bool

	numThreads int

	currentSubtitle *SubtitleOverlay
	subtitleExpiry  float64
	subtitleBgAlpha int

	info           *VideoInfo
	chapters       []Chapter
	filterPipeline *filters.FilterPipeline

	frameCache *PlaybackFrameCache

	lastError *PlaybackError

	gpuTexUpload interface{}
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

func (c *PlaybackFrameCache) SetMaxSize(maxSize int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.maxSize = maxSize

	for len(c.frameList) > maxSize && len(c.frameList) > 0 {
		oldest := c.frameList[0]
		delete(c.frames, oldest)
		c.frameList = c.frameList[1:]
	}
}

func (c *PlaybackFrameCache) MaxSize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.maxSize
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
		lastError:         nil,
		hwDegraded:        false,
		hwFailCount:       0,
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

func (e *Engine) GetFrameRate() float64 {
	return e.info.FrameRate
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

func (e *Engine) GetDecodeTimeTrend() float64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.decodeTimes) < 5 {
		return 0
	}

	oldAvg := 0.0
	newAvg := 0.0
	half := len(e.decodeTimes) / 2

	for i, t := range e.decodeTimes {
		ms := t.Seconds() * 1000
		if i < half {
			oldAvg += ms
		} else {
			newAvg += ms
		}
	}

	oldAvg /= float64(half)
	newAvg /= float64(len(e.decodeTimes) - half)

	if oldAvg == 0 {
		return 0
	}

	return (newAvg - oldAvg) / oldAvg
}

func (e *Engine) AdjustBufferForPerformance() {
	trend := e.GetDecodeTimeTrend()

	e.mu.Lock()
	defer e.mu.Unlock()

	if trend > 0.3 {
		newSize := e.frameCache.Size() + 10
		if newSize > 100 {
			newSize = 100
		}
		if e.frameCache != nil {
			e.frameCache.SetMaxSize(newSize)
		}
		logging.Debug(logging.CatPlayer, "Buffer increased to %d (decode trend: %.1f%%)", newSize, trend*100)
	} else if trend < -0.2 && e.frameCache != nil {
		currentSize := e.frameCache.Size()
		if currentSize > 15 {
			newSize := currentSize - 10
			e.frameCache.SetMaxSize(newSize)
			logging.Debug(logging.CatPlayer, "Buffer decreased to %d (decode trend: %.1f%%)", newSize, trend*100)
		}
	}
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

func (e *Engine) GetVideoBufferDepth() int {
	if e.videoQueue == nil {
		return 0
	}
	return e.videoQueue.Size()
}

func (e *Engine) GetAudioBufferDepth() int {
	if e.audioQueue == nil {
		return 0
	}
	return e.audioQueue.Size()
}

func (e *Engine) GetBufferHealth() float64 {
	videoDepth := e.GetVideoBufferDepth()
	audioDepth := e.GetAudioBufferDepth()

	videoMax := 50
	audioMax := 100

	if e.videoQueue != nil {
		videoMax = e.videoQueue.MaxSize()
	}
	if e.audioQueue != nil {
		audioMax = e.audioQueue.MaxSize()
	}

	videoHealth := float64(videoDepth) / float64(videoMax)
	audioHealth := float64(audioDepth) / float64(audioMax)

	return (videoHealth + audioHealth) / 2.0
}

func (e *Engine) IsBuffering() bool {
	health := e.GetBufferHealth()
	return health < 0.2
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
	defer C.av_packet_free(&pkt)

	ctx := C.avcodec_alloc_context3(codec)
	if ctx == nil {
		return false
	}
	defer C.avcodec_free_context(&ctx)

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
	e.filePath = path

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

	ret = C.avformat_find_stream_info(e.formatCtx, nil)
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
	// av_find_best_stream returns a negative AVERROR code when no stream is
	// found — clamp to -1 so downstream >= 0 checks and log output are clean.
	if e.videoStreamIdx < 0 {
		e.videoStreamIdx = -1
	}
	if e.audioStreamIdx < 0 {
		e.audioStreamIdx = -1
	}
	if e.subtitleStreamIdx < 0 {
		e.subtitleStreamIdx = -1
	}
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

		mediaType := stream.codecpar.codec_type
		codec := C.avcodec_find_decoder(stream.codecpar.codec_id)
		if codec == nil {
			continue
		}
		codecName := C.GoString((*C.char)(unsafe.Pointer(codec.name)))

		var language, title string
		if stream.metadata != nil {
			var entry *C.AVDictionaryEntry
			for {
				entry = C.av_dict_iterate(stream.metadata, entry)
				if entry == nil {
					break
				}
				key := C.GoString(entry.key)
				if key == "language" {
					language = C.GoString(entry.value)
				} else if key == "title" {
					title = C.GoString(entry.value)
				}
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
			// Re-allocate the codec context clean, without any partial HW state
			C.avcodec_free_context(&e.videoCodecCtx)
			swCodec := C.avcodec_find_decoder(streams[e.videoStreamIdx].codecpar.codec_id)
			if swCodec == nil {
				C.avformat_close_input(&e.formatCtx)
				return fmt.Errorf("no software video decoder found for fallback")
			}
			videoCodec = swCodec
			e.videoCodecCtx = C.avcodec_alloc_context3(videoCodec)
			if e.videoCodecCtx == nil {
				C.avformat_close_input(&e.formatCtx)
				return fmt.Errorf("failed to allocate software video codec context")
			}
			C.avcodec_parameters_to_context(e.videoCodecCtx, streams[e.videoStreamIdx].codecpar)
			if e.numThreads > 0 {
				e.videoCodecCtx.thread_count = C.int(e.numThreads)
			}
			logging.Info(logging.CatPlayer, "SW fallback: re-allocated codec ctx for %s", C.GoString((*C.char)(unsafe.Pointer(videoCodec.name))))
		}
	}

	e.videoTimeBase = float64(streams[e.videoStreamIdx].time_base.num) / float64(streams[e.videoStreamIdx].time_base.den)

	if e.subtitleStreamIdx >= 0 {
		e.initSubtitleDecoder(streams)
	}

	e.info.Width = int(e.videoCodecCtx.width)
	e.info.Height = int(e.videoCodecCtx.height)
	if e.videoCodecCtx.codec != nil {
		e.info.CodecName = C.GoString((*C.char)(unsafe.Pointer(e.videoCodecCtx.codec.name)))
	}
	if fmtName := av_get_pix_fmt_name(e.videoCodecCtx.pix_fmt); fmtName != nil {
		e.info.PixelFormat = C.GoString((*C.char)(unsafe.Pointer(fmtName)))
	}

	avgFrameRate := streams[e.videoStreamIdx].avg_frame_rate
	if avgFrameRate.num > 0 {
		e.info.FrameRate = float64(avgFrameRate.num) / float64(avgFrameRate.den)
	}

	logging.Info(logging.CatPlayer, "Opening video codec: %s %dx%d pix_fmt=%d", e.info.CodecName, e.info.Width, e.info.Height, e.videoCodecCtx.pix_fmt)
	if C.avcodec_open2(e.videoCodecCtx, videoCodec, nil) < 0 {
		C.avcodec_free_context(&e.videoCodecCtx)
		C.avformat_close_input(&e.formatCtx)
		return fmt.Errorf("failed to open video codec")
	}
	logging.Info(logging.CatPlayer, "Video codec opened OK")

	if e.audioStreamIdx >= 0 {
		logging.Info(logging.CatPlayer, "Opening audio codec for stream %d", e.audioStreamIdx)
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
				logging.Info(logging.CatPlayer, "Audio codec opened OK, creating audio player")
				ap, err := NewAudioPlayer(e.audioCodecCtx, e.audioQueue, e.clock, e.audioTimeBase)
				if err != nil {
					logging.Error(logging.CatPlayer, "Failed to create audio player: %v", err)
					C.avcodec_free_context(&e.audioCodecCtx)
					e.audioCodecCtx = nil
					e.audioStreamIdx = -1
					e.info.HasAudio = false
				} else {
					logging.Info(logging.CatPlayer, "Audio player created OK")
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

// StartThumbnailExtraction opens an independent decoder (separate from the live
// playback engine) and extracts one 160×90 thumbnail every 10 seconds.
// Using a separate AVFormatContext means thumbnail extraction never races with
// the main engine's demuxer, queue, or paused/playing state.
func (e *Engine) StartThumbnailExtraction(onFrame func(time float64, img *image.RGBA)) {
	e.mu.Lock()
	path := e.filePath
	duration := e.info.Duration
	e.mu.Unlock()

	if path == "" || duration <= 0 {
		return
	}

	go func() {
		defer logging.RecoverPanic()

		const (
			thumbW    = 160
			thumbH    = 90
			interval  = 10.0
		)

		// --- open independent format context ---
		cPath := C.CString(path)
		defer C.free(unsafe.Pointer(cPath))

		var fmtCtx *C.AVFormatContext
		if C.avformat_open_input(&fmtCtx, cPath, nil, nil) != 0 {
			logging.Warning(logging.CatPlayer, "thumbnail: failed to open %s", path)
			return
		}
		defer C.avformat_close_input(&fmtCtx)

		if C.avformat_find_stream_info(fmtCtx, nil) < 0 {
			return
		}

		// find first video stream
		vidIdx := -1
		var codec *C.AVCodec
		for i := 0; i < int(fmtCtx.nb_streams); i++ {
			stream := *(**C.AVStream)(unsafe.Pointer(
				uintptr(unsafe.Pointer(fmtCtx.streams)) + uintptr(i)*unsafe.Sizeof(uintptr(0))))
			if stream.codecpar.codec_type == C.AVMEDIA_TYPE_VIDEO {
				vidIdx = i
				codec = C.avcodec_find_decoder(stream.codecpar.codec_id)
				break
			}
		}
		if vidIdx < 0 || codec == nil {
			return
		}

		stream := *(**C.AVStream)(unsafe.Pointer(
			uintptr(unsafe.Pointer(fmtCtx.streams)) + uintptr(vidIdx)*unsafe.Sizeof(uintptr(0))))

		codecCtx := C.avcodec_alloc_context3(codec)
		if codecCtx == nil {
			return
		}
		defer C.avcodec_free_context(&codecCtx)

		C.avcodec_parameters_to_context(codecCtx, stream.codecpar)
		codecCtx.thread_count = 2
		if C.avcodec_open2(codecCtx, codec, nil) < 0 {
			return
		}

		timeBase := float64(stream.time_base.num) / float64(stream.time_base.den)

		// scale to thumbnail size
		swsCtx := C.sws_getContext(
			codecCtx.width, codecCtx.height, codecCtx.pix_fmt,
			C.int(thumbW), C.int(thumbH), C.AV_PIX_FMT_RGBA,
			C.SWS_BICUBIC, nil, nil, nil,
		)
		if swsCtx == nil {
			return
		}
		defer C.sws_freeContext(swsCtx)

		rgbaFrame := C.av_frame_alloc()
		if rgbaFrame == nil {
			return
		}
		defer C.av_frame_free(&rgbaFrame)

		numBytes := int(C.av_image_get_buffer_size(C.AV_PIX_FMT_RGBA, C.int(thumbW), C.int(thumbH), 1))
		rgbaBuf := make([]byte, numBytes)
		C.av_image_fill_arrays(
			&rgbaFrame.data[0], &rgbaFrame.linesize[0],
			(*C.uint8_t)(unsafe.Pointer(&rgbaBuf[0])),
			C.AV_PIX_FMT_RGBA, C.int(thumbW), C.int(thumbH), 1,
		)

		frame := C.av_frame_alloc()
		if frame == nil {
			return
		}
		defer C.av_frame_free(&frame)

		pkt := C.av_packet_alloc()
		if pkt == nil {
			return
		}
		defer C.av_packet_free(&pkt)

		for t := 0.0; t < duration; t += interval {
			select {
			case <-e.stop:
				return
			default:
			}

			// seek to target
			target := C.int64_t(t / timeBase)
			C.avformat_seek_file(fmtCtx, C.int(vidIdx), 0, target, target, 0)
			C.avcodec_flush_buffers(codecCtx)

			// decode one frame
			decoded := false
			for !decoded {
				if C.av_read_frame(fmtCtx, pkt) < 0 {
					break
				}
				if int(pkt.stream_index) != vidIdx {
					C.av_packet_unref(pkt)
					continue
				}

				if C.avcodec_send_packet(codecCtx, pkt) == 0 {
					if C.avcodec_receive_frame(codecCtx, frame) == 0 {
						C.sws_scale(
							swsCtx,
							&frame.data[0], &frame.linesize[0],
							0, codecCtx.height,
							&rgbaFrame.data[0], &rgbaFrame.linesize[0],
						)

						img := image.NewRGBA(image.Rect(0, 0, thumbW, thumbH))
						stride := int(rgbaFrame.linesize[0])
						for y := 0; y < thumbH; y++ {
							src := rgbaBuf[y*stride : y*stride+thumbW*4]
							dst := img.Pix[y*img.Stride : y*img.Stride+thumbW*4]
							copy(dst, src)
						}

						if onFrame != nil {
							onFrame(t, img)
						}
						decoded = true
					}
				}
				C.av_packet_unref(pkt)
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

	// Unpause the clock so SyncVideo/WaitForPTS work correctly.
	// The clock is initialized paused so it doesn't advance during Open/setup.
	// Resetting ptsTime here means the clock starts from 0 when demuxing begins.
	e.clock.SetPaused(false)

	go e.demuxerLoop()
}

func (e *Engine) demuxerLoop() {
	defer func() {
		if r := recover(); r != nil {
			logging.Error(logging.CatPlayer, "demuxerLoop panic: %v", r)
			e.videoQueue.SetEOF()
			e.audioQueue.SetEOF()
			e.subtitleQueue.SetEOF()
		}
	}()

	pkt := C.av_packet_alloc()
	defer C.av_packet_free(&pkt)

	for {
		select {
		case <-e.stop:
			return
		default:
		}

		// Serialise av_read_frame against avformat_seek_file — AVFormatContext
		// is not thread-safe; concurrent access from Seek() causes hard crashes.
		e.formatMu.Lock()
		ret := C.av_read_frame(e.formatCtx, pkt)
		e.formatMu.Unlock()

		if ret < 0 {
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

	e.formatMu.Lock()
	seekRet := C.avformat_seek_file(e.formatCtx, C.int(e.videoStreamIdx), target, target, target, flags)
	e.formatMu.Unlock()
	if seekRet < 0 {
		logging.Warning(logging.CatPlayer, "Seek to %.2f failed (ret=%d)", seconds, seekRet)
		return fmt.Errorf("seek failed")
	}
	logging.Info(logging.CatPlayer, "Seek to %.2f OK, flushing queues", seconds)

	e.videoQueue.Flush()
	e.audioQueue.Flush()
	logging.Info(logging.CatPlayer, "Seek: queues flushed, flushing video codec")

	// videoCodecMu must be held around avcodec_flush_buffers to serialise
	// against NextFrame() which holds the same lock during send/receive.
	//
	// Guard: only flush if at least one frame has been decoded. For D3D11VA
	// hardware decoders the hardware frame pool is lazily allocated on the
	// first decoded frame; calling avcodec_flush_buffers before that happens
	// dereferences an uninitialized pool and causes an access violation crash.
	if e.videoCodecCtx != nil && e.videoDecoded {
		e.videoCodecMu.Lock()
		C.avcodec_flush_buffers(e.videoCodecCtx)
		e.videoCodecMu.Unlock()
		logging.Info(logging.CatPlayer, "Seek: video codec flushed, flushing audio codec")
	} else {
		logging.Info(logging.CatPlayer, "Seek: skipping video codec flush (no frames decoded yet), flushing audio codec")
	}

	// Audio codec flush must go through AudioPlayer.FlushCodec() to serialise
	// against the concurrent decode happening in AudioPlayer.Read() (oto
	// callback goroutine). Calling avcodec_flush_buffers directly while Read()
	// holds the codec causes a hard crash.
	if e.audioPlayer != nil {
		e.audioPlayer.FlushCodec()
		e.audioPlayer.ResetEOF()
	} else if e.audioCodecCtx != nil {
		C.avcodec_flush_buffers(e.audioCodecCtx)
	}
	logging.Info(logging.CatPlayer, "Seek: audio codec flushed, resetting clock")

	e.clock.SetTime(seconds)
	logging.Info(logging.CatPlayer, "Seek: complete at %.2f", seconds)
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

// GrabFrame decodes and returns the first available video frame without any
// clock synchronisation or frame-drop logic. It is safe to call from any
// goroutine and is designed for obtaining a preview frame at load time when
// the engine may be paused and the audio clock may not yet be stable.
//
// A timeout prevents hanging on files where the demuxer or codec stalls.
func (e *Engine) GrabFrame(timeout time.Duration) (*image.RGBA, error) {
	deadline := time.Now().Add(timeout)
	logging.Debug(logging.CatPlayer, "GrabFrame: waiting for first video frame (timeout=%v)", timeout)

	for time.Now().Before(deadline) {
		// Non-blocking fetch — retry until a packet arrives or EOF/timeout.
		pkt, ok := e.videoQueue.TryGet()
		if !ok {
			if e.videoQueue.IsClosedOrEOF() {
				logging.Debug(logging.CatPlayer, "GrabFrame: video queue EOF")
				return nil, io.EOF
			}
			time.Sleep(5 * time.Millisecond)
			continue
		}

		// Lock codec around send+receive so we don't race with NextFrame or
		// SmoothScrubbing if they start concurrently.
		e.videoCodecMu.Lock()
		sendRet := C.avcodec_send_packet(e.videoCodecCtx, pkt)
		C.av_packet_free(&pkt)
		if sendRet != 0 {
			e.videoCodecMu.Unlock()
			continue // bad or non-video packet; try the next one
		}

		// Drain all frames produced by this packet; return the first.
		for C.avcodec_receive_frame(e.videoCodecCtx, e.frame) == 0 {
			e.videoCodecMu.Unlock()
			e.videoDecoded = true
			logging.Info(logging.CatPlayer, "GrabFrame: got frame pts=%.3f", float64(e.frame.pts)*e.videoTimeBase)

			if e.hwDevice != HWDeviceNone {
				img, err := e.retrieveHWFrame()
				if err != nil {
					logging.Warning(logging.CatPlayer, "GrabFrame: HW retrieve failed (%v), falling back to SW", err)
					img = e.toRGBA()
				}
				return img, nil
			}
			return e.toRGBA(), nil
		}
		e.videoCodecMu.Unlock()
	}

	return nil, fmt.Errorf("timed out waiting for first video frame")
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

		// Hold videoCodecMu around every avcodec_send/receive call so that
		// SmoothScrubbing (which also touches videoCodecCtx) cannot race with us.
		// The lock is released before any slow operations (WaitForPTS, toRGBA) and
		// re-acquired only when we need to call avcodec_receive_frame again.
		e.videoCodecMu.Lock()
		if C.avcodec_send_packet(e.videoCodecCtx, pkt) != 0 {
			e.videoCodecMu.Unlock()
			logging.Debug(logging.CatPlayer, "Failed to send packet to video decoder")
			continue
		}

		for C.avcodec_receive_frame(e.videoCodecCtx, e.frame) == 0 {
			pts := float64(e.frame.pts) * e.videoTimeBase
			e.videoCodecMu.Unlock() // release before any blocking / rendering work
			e.videoDecoded = true

			adjustedPts := pts * e.speed

			if hasAudio {
				e.clock.WaitForPTS(adjustedPts)
			} else {
				e.clock.SetTime(adjustedPts)
			}

			delay := e.clock.SyncVideo(adjustedPts)
			if delay < 0 {
				e.videoCodecMu.Lock() // re-acquire for next avcodec_receive_frame
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

			if e.subtitleCodecCtx != nil {
				sub := e.decodeSubtitle(adjustedPts)
				if sub != nil {
					img = e.RenderSubtitles(img, adjustedPts)
				}
			}

			return img, nil
		}
		e.videoCodecMu.Unlock()
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

	// av_hwframe_transfer_data downloads the HW surface to CPU.
	// Passing a plain av_frame_alloc() frame (no hw_frames_ctx) causes FFmpeg
	// to allocate CPU pixel buffers automatically and set swFrame.format to the
	// actual SW format (NV12 for D3D11VA, YUV420P for VAAPI, etc.).
	if C.av_hwframe_transfer_data(swFrame, e.frame, 0) != 0 {
		return nil, fmt.Errorf("failed to transfer HW frame to SW")
	}

	// Use a temporary sws context matched to swFrame.format rather than
	// rebuilding e.swsCtx in place.  Modifying e.swsCtx here would corrupt the
	// next software-decoded frame (e.g. after a seek or codec reset) which may
	// be in YUV420P while e.swsCtx would now be configured for NV12.
	swFmt := C.enum_AVPixelFormat(swFrame.format)
	w := swFrame.width
	h := swFrame.height
	hwSwsCtx := C.sws_getContext(
		w, h, swFmt,
		w, h, C.AV_PIX_FMT_RGBA,
		C.SWS_BILINEAR, nil, nil, nil,
	)
	if hwSwsCtx == nil {
		return nil, fmt.Errorf("failed to create sws context for hw pixel format %d", int(swFmt))
	}
	defer C.sws_freeContext(hwSwsCtx)

	C.sws_scale(
		hwSwsCtx,
		&swFrame.data[0], &swFrame.linesize[0],
		0, h,
		&e.rgbaFrame.data[0], &e.rgbaFrame.linesize[0],
	)

	img := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	copy(img.Pix, e.rgbaBuffer)
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

func (e *Engine) GetLastError() *PlaybackError {
	return e.lastError
}

func (e *Engine) ClearError() {
	e.lastError = nil
}

func (e *Engine) DegradeToSoftware() {
	if e.hwDevice == HWDeviceNone {
		return
	}

	logging.Warning(logging.CatPlayer, "Degrading from HW to software decoding")

	e.mu.Lock()
	e.hwDegraded = true

	if e.hwFramesCtx != nil {
		C.av_buffer_unref(&e.hwFramesCtx)
		e.hwFramesCtx = nil
	}
	if e.hwDeviceCtx != nil {
		C.av_buffer_unref(&e.hwDeviceCtx)
		e.hwDeviceCtx = nil
	}
	if e.videoCodecCtx.hw_frames_ctx != nil {
		C.av_buffer_unref(&e.videoCodecCtx.hw_frames_ctx)
		e.videoCodecCtx.hw_frames_ctx = nil
	}
	e.hwDevice = HWDeviceNone

	e.lastError = &PlaybackError{
		Code:    ErrCodeHWAccel,
		Message: "Fell back to software decoding",
		Retry:   false,
	}
	e.mu.Unlock()

	e.ClearFrameCache()
}

func (e *Engine) ShouldDegrade() bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.hwDegraded {
		return false
	}

	if e.hwFailCount >= 3 {
		return true
	}
	return false
}

func (e *Engine) RecordHWFailure() {
	e.mu.Lock()
	e.hwFailCount++
	e.mu.Unlock()
}

func (e *Engine) ResetHWFailureCount() {
	e.mu.Lock()
	e.hwFailCount = 0
	e.mu.Unlock()
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
		C.av_buffer_unref(&e.hwFramesCtx)
		e.hwFramesCtx = nil
	}
	if e.hwDeviceCtx != nil {
		C.av_buffer_unref(&e.hwDeviceCtx)
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
