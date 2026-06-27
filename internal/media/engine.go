//go:build native_media

package media

/*
#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libswscale/swscale.h>
#include <libavutil/avutil.h>
#include <libavutil/imgutils.h>
#include <libavutil/hwcontext.h>
#include <libavutil/dict.h>
#include <libavfilter/avfilter.h>

static AVChapter* getChapter(AVFormatContext *fmtCtx, int index) {
    if (fmtCtx == NULL || index < 0 || index >= fmtCtx->nb_chapters) {
        return NULL;
    }
    return fmtCtx->chapters[index];
}

// avformat_get_stream — safely return the n-th AVStream* from an AVFormatContext.
// Returns NULL if idx is out of range or the context is NULL.
AVStream* avformat_get_stream(AVFormatContext *fmtCtx, unsigned int idx) {
    if (fmtCtx == NULL || idx >= (unsigned int)fmtCtx->nb_streams) {
        return NULL;
    }
    return fmtCtx->streams[idx];
}
*/
import "C"
import (
	"fmt"
	"image"
	"io"
	"math"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/LeakTechnologies/VideoTools/internal/logging"
	"github.com/LeakTechnologies/VideoTools/internal/media/filters"
)

// decodedFrame is a fully decoded and colour-converted video frame ready for display.
type decodedFrame struct {
	img *image.RGBA
	pts float64
	gen uint64 // seek generation at time of decode; NextFrame drops stale generations
}

const (
	// preDecodeFrames is the size of the decode-ahead ring buffer.  8 frames at
	// 30 fps = 267 ms of headroom — enough to absorb a slow H.264 I-frame decode
	// (~150 ms single-threaded) without ever stalling the display goroutine.
	preDecodeFrames = 8

	// decodeEOFPTS is a sentinel PTS value used to signal end-of-stream through
	// the frameQueue channel without closing it (closing would prevent reuse after
	// a Seek-to-start / loop).
	decodeEOFPTS = -1.0
)

var defaultDeinterlaceEnabled = true

func SetDefaultDeinterlaceEnabled(enabled bool) {
	defaultDeinterlaceEnabled = enabled
}

func GetDefaultDeinterlaceEnabled() bool {
	return defaultDeinterlaceEnabled
}

type SeekAccuracy int

const (
	SeekAccuracyFrame SeekAccuracy = iota
	SeekAccuracyKeyframe
	SeekAccuracyAccurate
)

var defaultSeekAccuracy = SeekAccuracyKeyframe

func DefaultSeekAccuracy() SeekAccuracy {
	return defaultSeekAccuracy
}

func SetDefaultSeekAccuracy(acc SeekAccuracy) {
	defaultSeekAccuracy = acc
}

// defaultAudioDelay is the A/V offset applied to every newly opened engine.
// Stored in seconds; positive = video is delayed (audio appears early).
var defaultAudioDelayBits atomic.Uint64 // math.Float64bits; 0 = no delay

func DefaultAudioDelay() float64 {
	return math.Float64frombits(defaultAudioDelayBits.Load())
}

func SetDefaultAudioDelay(d float64) {
	defaultAudioDelayBits.Store(math.Float64bits(d))
}

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
	swsFmt            C.enum_AVPixelFormat

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

	hwRgbaFrame  *C.AVFrame
	hwRgbaBuffer []byte
	hwSwsCtx     *C.struct_SwsContext // cached swscale context for HW→RGBA
	hwSwsFmt     C.enum_AVPixelFormat
	hwSwsW, hwSwsH int

	framePool    [][]byte
	framepoolMu  sync.Mutex // protects framePool only (level 4 — see lock.go for hierarchy)

	videoTimeBase    float64
	audioTimeBase    float64
	subtitleTimeBase float64

	mu             sync.Mutex // general engine state (level 1 — see lock.go for hierarchy)
	formatMu       sync.Mutex // serialises av_read_frame vs avformat_seek_file (level 2)
	videoCodecMu   sync.Mutex // serialises avcodec_send_packet / avcodec_receive_frame (level 3)
	subtitleCodecMu sync.Mutex // serialises subtitleCodecCtx access (level 3.5)
	demuxerWg      sync.WaitGroup // tracks demuxerLoop goroutine; Close waits before freeing contexts
	running      bool
	paused       bool
	stop         chan struct{}
	loading      bool

	volume  float32
	muted   bool
	speed   float64
	seekAcc SeekAccuracy

	dropFrames       bool
	consecutiveDrops int

	bufferMode     BufferMode
	lastDecodeTime time.Time
	decodeTimes    []time.Duration

	hwDevice       HWDeviceType
	hwDegraded     bool
	videoDecoded   bool   // set true after first successful video frame decode
	videoDecodeDead bool  // set true when SEH catches access violation in video decode
	hwFailCount    int

	looping     bool
	growingFile bool
	abLoopEnabled bool
	loopA         float64
	loopB         float64
	abLoopPending bool
	hasAudio    bool

	numThreads int

	currentSubtitle *SubtitleOverlay
	subtitleExpiry  float64
	subtitleBgAlpha int

	info           *VideoInfo
	chapters       []Chapter
	filterPipeline *filters.FilterPipeline

	frameCache *PlaybackFrameCache

	errorRing     [errorRingSize]ErrorRecord
	errorRingNext int
	errorMu       sync.Mutex

	gpuTexUpload interface{}

	// nextFrameCount counts NextFrame invocations; used for first-call verbose logging.
	nextFrameCount int64

	// Decode-ahead pipeline.  videoDecodeLoop runs as a dedicated goroutine that
	// pre-decodes and colour-converts frames into frameQueue.  NextFrame drains
	// the queue and handles PTS sync, keeping I-frame decode latency invisible.
	frameQueue      chan decodedFrame
	decodeLoopStop  chan struct{}
	decodeLoopWg    sync.WaitGroup
	decodeLoopActive bool    // true while videoDecodeLoop goroutine is alive
	decodeEOFSent       bool    // true once the EOF sentinel has been enqueued
	// Deinterlace (bwdif) filter graph — lazily created on first interlaced frame.
	deinterlaceEnabled bool
	deintFilterGraph   *C.AVFilterGraph
	deintBuffersrc     *C.AVFilterContext
	deintBuffersink    *C.AVFilterContext

	// HDR tone-mapping filter graph — lazily created on first HDR frame.
	// hdrTonemapUnsupported is set when zscale/tonemap are unavailable so we
	// don't retry the expensive graph creation on every subsequent frame.
	hdrFilterGraph       *C.AVFilterGraph
	hdrBuffersrc         *C.AVFilterContext
	hdrBuffersink        *C.AVFilterContext
	hdrInputPixFmt       C.enum_AVPixelFormat
	hdrTonemapUnsupported bool

	// seekFlushBefore is read/written by both Seek() and videoDecodeLoop.
	// Seek() holds e.mu; videoDecodeLoop holds videoCodecMu at the read site.
	// Acquiring e.mu inside videoCodecMu creates a lock-order deadlock with
	// Seek() (which does e.mu → videoCodecMu).  Atomic access eliminates
	// the nested lock entirely — no mutex needed for this field.
	seekFlushBefore  atomic.Uint64 // math.Float64bits; 0 = guard inactive
	seekGen          atomic.Uint64 // incremented on each Seek(); frames carry the gen at decode time
	lastVideoPTSBits atomic.Uint64 // math.Float64bits of the last video PTS handed to the display
	audioDelayBits   atomic.Uint64 // math.Float64bits; A/V offset in seconds (see SetAudioDelay)

	// lastGoodFrame holds the most recently displayed video frame.  When a decode
	// error causes videoDecodeLoop to exit early (SEH, HW fatal), NextFrame returns
	// this frozen frame instead of immediately returning io.EOF, preventing the
	// display from going black on transient decode failures.
	lastGoodFrame  atomic.Pointer[image.RGBA]
	// decodeErrored is set by videoDecodeLoop on fatal decode errors (SEH or
	// HW-after-degrade).  NextFrame reads it to distinguish error-EOF from
	// natural stream EOF — error-EOF returns the last good frame once.
	decodeErrored  atomic.Bool
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
		deinterlaceEnabled: defaultDeinterlaceEnabled,
		videoStreamIdx:     -1,
		audioStreamIdx:     -1,
		subtitleStreamIdx:  -1,
		// Smaller queue caps reduce demuxer blocking time and shrink the audio
		// tail after video EOF.  videoQueue at 50 packets gives ~1.7s of
		// encoded-packet headroom (enough for one GOP boundary) without letting
		// the blocking Put hold the demuxer away from audio for >1.7s.
		// audioQueue at 32 packets caps the post-EOF audio drain to <0.75s.
		videoQueue:        NewPacketQueueWithMaxSize(50),
		audioQueue:        NewPacketQueueWithMaxSize(32),
		subtitleQueue:     NewPacketQueue(),
		clock:             NewMasterClock(),
		stop:              make(chan struct{}),
		volume:            1.0,
		speed:             1.0,
		seekAcc:           SeekAccuracyKeyframe,
		numThreads:        0,
		framePool:         make([][]byte, 0, 4),
		errorRingNext:     0,
		hwDegraded:        false,
		hwFailCount:       0,
		frameQueue:        make(chan decodedFrame, preDecodeFrames),
		decodeLoopStop:    make(chan struct{}),
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
	e.clock.SetSpeed(speed)
	if e.audioPlayer != nil {
		e.audioPlayer.SetSpeed(speed)
	}
}

func (e *Engine) GetSpeed() float64 {
	return e.speed
}

func (e *Engine) GetFrameRate() float64 {
	return e.info.FrameRate
}

func (e *Engine) SetLoading(loading bool) {
	e.lockMu()
	defer e.unlockMu()
	e.loading = loading
}

func (e *Engine) IsLoading() bool {
	e.lockMu()
	defer e.unlockMu()
	return e.loading
}

func (e *Engine) GetChapters() []Chapter {
	e.lockMu()
	defer e.unlockMu()
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

// SetAudioDelay sets the A/V offset in seconds. Positive values delay video
// presentation relative to the audio clock, compensating for audio that
// arrives early at the listener (e.g. Bluetooth speaker latency). Negative
// values advance video, compensating for audio that arrives late.
// Changes take effect immediately on the next decoded frame.
func (e *Engine) SetAudioDelay(d float64) {
	e.audioDelayBits.Store(math.Float64bits(d))
}

func (e *Engine) GetAudioDelay() float64 {
	return math.Float64frombits(e.audioDelayBits.Load())
}

func (e *Engine) SetDeinterlaceEnabled(enabled bool) {
	e.lockMu()
	defer e.unlockMu()
	e.deinterlaceEnabled = enabled
	if !enabled {
		e.freeDeinterlaceFilter()
	}
}

func (e *Engine) IsDeinterlaceEnabled() bool {
	e.lockMu()
	defer e.unlockMu()
	return e.deinterlaceEnabled
}

func (e *Engine) SetDropFrames(enabled bool) {
	e.dropFrames = enabled
}

func (e *Engine) IsDropFramesEnabled() bool {
	return e.dropFrames
}

func (e *Engine) SetLooping(looping bool) {
	e.lockMu()
	defer e.unlockMu()
	e.looping = looping
	if e.audioPlayer != nil {
		e.audioPlayer.SetLooping(looping)
	}
}

func (e *Engine) IsLooping() bool {
	e.lockMu()
	defer e.unlockMu()
	return e.looping
}

func (e *Engine) SetGrowingFile(growing bool) {
	e.lockMu()
	defer e.unlockMu()
	e.growingFile = growing
}

func (e *Engine) IsGrowingFile() bool {
	e.lockMu()
	defer e.unlockMu()
	return e.growingFile
}

func (e *Engine) SetABLoopEnabled(enabled bool) {
	e.lockMu()
	defer e.unlockMu()
	e.abLoopEnabled = enabled
	if !enabled {
		e.abLoopPending = false
	}
}

func (e *Engine) IsABLoopEnabled() bool {
	e.lockMu()
	defer e.unlockMu()
	return e.abLoopEnabled
}

func (e *Engine) SetLoopPoints(a, b float64) {
	e.lockMu()
	defer e.unlockMu()
	e.loopA = a
	e.loopB = b
}

func (e *Engine) LoopPoints() (float64, float64) {
	e.lockMu()
	defer e.unlockMu()
	return e.loopA, e.loopB
}

func (e *Engine) SetFilterPipeline(pipeline *filters.FilterPipeline) {
	e.lockMu()
	defer e.unlockMu()
	e.filterPipeline = pipeline
}

func (e *Engine) GetFilterPipeline() *filters.FilterPipeline {
	e.lockMu()
	defer e.unlockMu()
	return e.filterPipeline
}

func (e *Engine) GetFilterGraph() string {
	e.lockMu()
	defer e.unlockMu()
	if e.filterPipeline == nil {
		return ""
	}
	graph, _ := e.filterPipeline.Generate()
	return graph
}

func (e *Engine) SetFilter(filterType filters.FilterType, params map[string]interface{}, enabled bool) {
	e.lockMu()
	defer e.unlockMu()

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
	e.lockMu()
	defer e.unlockMu()
	if e.filterPipeline != nil {
		e.filterPipeline.Enable(filterType, enabled)
	}
}

func (e *Engine) ClearFilters() {
	e.lockMu()
	defer e.unlockMu()
	if e.filterPipeline != nil {
		e.filterPipeline.Clear()
	}
}

func (e *Engine) SetPreset(preset filters.Preset) {
	e.lockMu()
	defer e.unlockMu()
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

	// Snapshot playback state before touching any shared fields.
	e.lockMu()
	wasPlaying := e.running && !e.paused
	speed := e.speed
	vol := e.volume
	muted := e.muted
	e.unlockMu()

	// Close the old AudioPlayer FIRST — stops audioDecodeLoop which uses
	// audioCodecCtx.  Freeing the codec context before the goroutine exits
	// is a use-after-free.
	if e.audioPlayer != nil {
		e.audioPlayer.Close()
		e.audioPlayer = nil
	}

	// Safe to flush and free now that the decode goroutine is gone.
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
	e.audioCodecCtx.thread_count = 1
	e.audioCodecCtx.thread_type = 0

	if C.avcodec_open2(e.audioCodecCtx, codec, nil) < 0 {
		C.avcodec_free_context(&e.audioCodecCtx)
		e.audioCodecCtx = nil
		return fmt.Errorf("failed to open audio codec")
	}

	ap, err := NewAudioPlayer(e.audioCodecCtx, e.audioQueue, e.clock, e.audioTimeBase)
	if err != nil {
		C.avcodec_free_context(&e.audioCodecCtx)
		e.audioCodecCtx = nil
		return fmt.Errorf("failed to create audio player: %w", err)
	}
	ap.SetVolume(vol)
	ap.SetMuted(muted)
	if speed != 1.0 {
		ap.SetSpeed(speed)
	}

	e.audioPlayer = ap
	e.audioStreamIdx = streamIdx
	e.hasAudio = true

	// Seek to current position so the new track starts in sync with video.
	currentPTS := math.Float64frombits(e.lastVideoPTSBits.Load())
	if e.running && currentPTS > 0 {
		if err := e.Seek(currentPTS); err != nil {
			logging.Warning(logging.CatPlayer, "SelectAudioTrack: seek to %.2f failed: %v", currentPTS, err)
		}
	}

	if wasPlaying {
		ap.Resume()
	}

	logging.Info(logging.CatPlayer, "Selected audio track %d (stream %d)", trackIndex, streamIdx)
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

	newStreamIdx := e.info.SubtitleTracks[trackIndex].Index
	streams := (*[1 << 30]*C.AVStream)(unsafe.Pointer(e.formatCtx.streams))

	// Tear down old subtitle codec under the lock so decodeSubtitle
	// (called from NextFrame on the playback goroutine) doesn't race.
	e.subtitleCodecMu.Lock()
	e.subtitleStreamIdx = newStreamIdx
	e.subtitleQueue.Flush()
	if e.subtitleCodecCtx != nil {
		C.avcodec_free_context(&e.subtitleCodecCtx)
		e.subtitleCodecCtx = nil
	}
	e.currentSubtitle = nil
	e.initSubtitleDecoder(streams)
	e.subtitleCodecMu.Unlock()

	logging.Info(logging.CatPlayer, "Selected subtitle track %d (stream %d)", trackIndex, newStreamIdx)
	return nil
}

func (e *Engine) DisableSubtitles() {
	e.subtitleCodecMu.Lock()
	e.subtitleStreamIdx = -1
	if e.subtitleCodecCtx != nil {
		C.avcodec_free_context(&e.subtitleCodecCtx)
		e.subtitleCodecCtx = nil
	}
	e.currentSubtitle = nil
	e.subtitleCodecMu.Unlock()
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

// setVideoCodecErrorFlags configures error concealment and resilience on a
// freshly allocated AVCodecContext before avcodec_open2.  Must be called after
// avcodec_parameters_to_context (which may reset defaults) and before open.
func setVideoCodecErrorFlags(ctx *C.AVCodecContext) {
	if ctx == nil {
		return
	}
	// Explicit concealment: motion-vector extrapolation for missing macroblocks
	// (GUESS_MVS) and deblocking filter on concealed regions (DEBLOCK).
	ctx.error_concealment = C.FF_EC_GUESS_MVS | C.FF_EC_DEBLOCK
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
	setVideoCodecErrorFlags(e.videoCodecCtx)

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
	e.lockMu()
	origStreamIdx := e.videoStreamIdx
	origCodecCtx := e.videoCodecCtx
	origTimeBase := e.videoTimeBase
	e.unlockMu()

	if err := e.Seek(seconds); err != nil {
		return nil, err
	}

	img, err := e.NextFrame()
	if err != nil && err != io.EOF {
		return nil, err
	}

	e.lockMu()
	e.videoStreamIdx = origStreamIdx
	e.videoCodecCtx = origCodecCtx
	e.videoTimeBase = origTimeBase
	e.unlockMu()

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

	return e.openFinalize()
}

// OpenAuto tries Open first, then falls back to OpenDVD(path, 0) on failure.
// This handles ISOs and VIDEO_TS directories that avformat_open_input rejects
// when passed nil format — the dvdvideo demuxer is required for those sources.
// Title 0 selects the longest (main-feature) title automatically.
func (e *Engine) OpenAuto(path string) error {
	if err := e.Open(path); err == nil {
		return nil
	}
	logging.Info(logging.CatPlayer, "OpenAuto: Open failed, retrying as DVD/ISO via OpenDVD(title=0)")
	return e.OpenDVD(path, 0)
}

// OpenURL opens a network stream or URL with AVDictionary options passed
// directly to avformat_open_input.  opts may be nil for defaults; otherwise
// each key/value pair is set on the AVDictionary before opening.
//
// Sensible network defaults are applied and may be overridden via opts:
//
//	timeout (µs)              60000000  (60s I/O timeout)
//	reconnect_streamed        1         (reconnect on broken pipe)
//	reconnect_on_network_error 1        (reconnect on TCP/network err)
//	reconnect_delay_max (s)   5         (max 5s between retries)
//
// Supported URL schemes: http, https, hls, dash, rtsp, rtmp, rtmpe, rtmps,
// mms, tcp, udp, and any other protocol FFmpeg was built with.
func (e *Engine) OpenURL(url string, opts map[string]string) error {
	e.filePath = url

	cURL := C.CString(url)
	defer C.free(unsafe.Pointer(cURL))

	var dict *C.AVDictionary
	defer func() {
		if dict != nil {
			C.av_dict_free(&dict)
		}
	}()

	setOpt := func(key, val string) {
		cK := C.CString(key)
		cV := C.CString(val)
		C.av_dict_set(&dict, cK, cV, 0)
		C.free(unsafe.Pointer(cK))
		C.free(unsafe.Pointer(cV))
	}

	setOpt("timeout", "60000000")
	setOpt("reconnect_streamed", "1")
	setOpt("reconnect_on_network_error", "1")
	setOpt("reconnect_delay_max", "5")

	for k, v := range opts {
		setOpt(k, v)
	}

	logging.Info(logging.CatPlayer, "OpenURL: opening %s", url)

	ret := C.avformat_open_input(&e.formatCtx, cURL, nil, &dict)
	if ret != 0 {
		errBuf := make([]byte, 256)
		C.av_strerror(ret, (*C.char)(unsafe.Pointer(&errBuf[0])), 256)
		errStr := C.GoString((*C.char)(unsafe.Pointer(&errBuf[0])))
		if errStr == "" {
			errStr = "unknown FFmpeg error"
		}
		logging.Error(logging.CatPlayer, "OpenURL: avformat_open_input failed for %s: %s (code: %d)", url, errStr, ret)
		return fmt.Errorf("failed to open URL %s: %s", url, errStr)
	}

	return e.openFinalize()
}

// openFinalize runs after avformat_open_input succeeds: probes streams, allocates
// codec contexts, and sets up frame buffers. Called by both Open and OpenDVD.
func (e *Engine) openFinalize() error {
	// Cap probe depth so unusual files don't stall the engine indefinitely.
	// probesize is in bytes; max_analyze_duration is in AV_TIME_BASE units (µs).
	// 10 MB / 10 s is generous enough for any well-formed file, including those
	// with DTS/AC-3 audio that need more packets than AAC.
	e.formatCtx.probesize = C.int64_t(10_000_000)
	e.formatCtx.max_analyze_duration = C.int64_t(10_000_000)

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
		codecName := C.GoString((*C.char)(unsafe.Pointer(videoCodec.name)))
		if !e.codecCanUseHWDevice(videoCodec) {
			logging.Info(logging.CatPlayer, "Codec %s has no HW config for device %v — using SW decode", codecName, e.hwDevice)
			e.hwDevice = HWDeviceNone
		} else if err := e.initHWDecode(); err != nil {
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

	// SW decode: force single-threaded (thread_count=1).
	// FF_THREAD_FRAME (multi-frame parallel decode) causes avcodec_receive_frame
	// to block while internal frame threads wait for reference data; if the
	// demuxer queue is flushed mid-decode (seek) those threads deadlock and
	// avcodec_flush_buffers never returns.  FF_THREAD_SLICE causes a Win32
	// thread-environment crash on first packet.  The 8-frame pre-decode buffer
	// (preDecodeFrames) already hides single-thread I-frame latency.
	e.videoCodecCtx.thread_count = 1
	logging.Info(logging.CatPlayer, "SW video decode: thread_count=1 (single-threaded, seek-safe)")
	setVideoCodecErrorFlags(e.videoCodecCtx)

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
			// Force single-threaded decode for audio. FFmpeg's lazy thread-pool
			// initialisation inside avcodec_send_packet can crash on Windows when
			// the caller is the oto audio goroutine rather than a Go-owned goroutine.
			e.audioCodecCtx.thread_count = 1
			e.audioCodecCtx.thread_type = 0
			if C.avcodec_open2(e.audioCodecCtx, audioCodec, nil) < 0 {
				logging.Warning(logging.CatPlayer, "Failed to open audio codec")
				C.avcodec_free_context(&e.audioCodecCtx)
				e.audioCodecCtx = nil
				e.audioStreamIdx = -1
				e.info.HasAudio = false
			} else {
				logging.Info(logging.CatPlayer, "Audio codec opened OK, creating audio player")
				logging.Info(logging.CatPlayer, "Audio codec: sample_rate=%d fmt=%d codec_id=%d extradata_size=%d",
					e.audioCodecCtx.sample_rate, e.audioCodecCtx.sample_fmt,
					e.audioCodecCtx.codec_id, e.audioCodecCtx.extradata_size)
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

	// Lazy swsCtx creation: defer until we know the actual pixel format.
	// For HW decode (D3D11VA), videoCodecCtx.pix_fmt is still NONE at open
	// time and only gets set after the first avcodec_receive_frame. Creating
	// swsCtx here with format NONE would produce an invalid context that
	// crashes sws_scale. Instead, create it lazily on first frame decode
	// using the actual frame format (e.frame.format).

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

// OpenDVD opens a DVD disc (ISO file or VIDEO_TS parent directory) using
// FFmpeg's dvdvideo demuxer backed by libdvdnav/libdvdread — the same
// libraries used by VLC and mpv. title=0 auto-selects the longest title
// (main feature); title>0 selects that title by DVD title number.
func (e *Engine) OpenDVD(devicePath string, title int) error {
	e.filePath = devicePath

	cPath := C.CString(devicePath)
	defer C.free(unsafe.Pointer(cPath))

	cFmtName := C.CString("dvdvideo")
	defer C.free(unsafe.Pointer(cFmtName))

	dvdFmt := C.av_find_input_format(cFmtName)
	if dvdFmt == nil {
		return fmt.Errorf("dvdvideo demuxer not available in this FFmpeg build")
	}

	var opts *C.AVDictionary
	defer func() {
		if opts != nil {
			C.av_dict_free(&opts)
		}
	}()
	if title > 0 {
		cKey := C.CString("title")
		cVal := C.CString(strconv.Itoa(title))
		C.av_dict_set(&opts, cKey, cVal, 0)
		C.free(unsafe.Pointer(cKey))
		C.free(unsafe.Pointer(cVal))
	}

	logging.Info(logging.CatPlayer, "OpenDVD: opening %s (title=%d)", devicePath, title)

	ret := C.avformat_open_input(&e.formatCtx, cPath, dvdFmt, &opts)
	if ret != 0 {
		errBuf := make([]byte, 256)
		C.av_strerror(ret, (*C.char)(unsafe.Pointer(&errBuf[0])), 256)
		errStr := C.GoString((*C.char)(unsafe.Pointer(&errBuf[0])))
		if errStr == "" {
			errStr = "unknown FFmpeg error"
		}
		logging.Error(logging.CatPlayer, "OpenDVD: avformat_open_input failed for %s: %s (code: %d)", devicePath, errStr, ret)
		return fmt.Errorf("failed to open DVD %s: %s", devicePath, errStr)
	}

	return e.openFinalize()
}

// StartThumbnailExtraction opens an independent decoder (separate from the live
// playback engine) and extracts one 160×90 thumbnail every 10 seconds.
// Using a separate AVFormatContext means thumbnail extraction never races with
// the main engine's demuxer, queue, or paused/playing state.
func (e *Engine) StartThumbnailExtraction(onFrame func(time float64, img *image.RGBA)) {
	e.lockMu()
	path := e.filePath
	duration := e.info.Duration
	e.unlockMu()

	if path == "" || duration <= 0 {
		return
	}

	go func() {
		defer logging.RecoverPanic()

		const (
			thumbW   = 160
			thumbH   = 90
			interval = 10.0
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
		codecCtx.thread_count = 1
		if C.avcodec_open2(codecCtx, codec, nil) < 0 {
			return
		}

		timeBase := float64(stream.time_base.num) / float64(stream.time_base.den)
		if timeBase <= 0 {
			logging.Warning(logging.CatPlayer, "thumbnail: stream %d has invalid timeBase (%v), skipping", vidIdx, timeBase)
			return
		}

		// swsCtx is created lazily after the first decoded frame so we can use
		// frame.format (the actual SW pixel format) rather than codecCtx.pix_fmt
		// (set at open time, may be 0/NONE or a HW format placeholder).
		var swsCtx *C.struct_SwsContext
		defer func() {
			if swsCtx != nil {
				C.sws_freeContext(swsCtx)
			}
		}()

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

		logging.Info(logging.CatPlayer, "thumbnail: starting extraction (duration=%.1fs interval=%.0fs timeBase=%g)", duration, interval, timeBase)

		for t := 0.0; t < duration; t += interval {
			select {
			case <-e.stop:
				return
			default:
			}

			// seek to target
			target := C.int64_t(t / timeBase)
			logging.Info(logging.CatPlayer, "thumbnail: seeking to t=%.1fs (pts=%d)", t, int64(target))
			C.avformat_seek_file(fmtCtx, C.int(vidIdx), 0, target, target, 0)
			C.avcodec_flush_buffers(codecCtx)

			// decode one frame
			decoded := false
			for !decoded {
				select {
				case <-e.stop:
					return
				default:
				}

				if C.av_read_frame(fmtCtx, pkt) < 0 {
					break
				}
				if int(pkt.stream_index) != vidIdx {
					C.av_packet_unref(pkt)
					continue
				}

				if C.avcodec_send_packet(codecCtx, pkt) == 0 {
					if C.avcodec_receive_frame(codecCtx, frame) == 0 {
						// Lazy-init swsCtx using actual frame pixel format.
						// Using frame.format instead of codecCtx.pix_fmt avoids
						// mismatches when the codec updates the format after open.
						if swsCtx == nil {
							pixFmt := C.enum_AVPixelFormat(frame.format)
							logging.Info(logging.CatPlayer, "thumbnail: creating swsCtx (pixFmt=%d w=%d h=%d)", int(pixFmt), int(codecCtx.width), int(codecCtx.height))
							swsCtx = C.sws_getContext(
								codecCtx.width, codecCtx.height, pixFmt,
								C.int(thumbW), C.int(thumbH), C.AV_PIX_FMT_RGBA,
								C.SWS_BICUBIC, nil, nil, nil,
							)
							if swsCtx == nil {
								logging.Warning(logging.CatPlayer, "thumbnail: sws_getContext failed for pixFmt=%d", int(pixFmt))
								C.av_packet_unref(pkt)
								break // skip this thumbnail, try next interval
							}
						}

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




