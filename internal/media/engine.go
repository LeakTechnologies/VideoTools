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

// avformat_get_stream — safely return the n-th AVStream* from an AVFormatContext.
// Returns NULL if idx is out of range or the context is NULL.
AVStream* avformat_get_stream(AVFormatContext *fmtCtx, unsigned int idx) {
    if (fmtCtx == NULL || idx >= (unsigned int)fmtCtx->nb_streams) {
        return NULL;
    }
    return fmtCtx->streams[idx];
}

// vt_get_hw_format is the get_format callback for hardware-accelerated decoding.
// It is required by D3D11VA and other HW backends: without it FFmpeg cannot
// negotiate the HW pixel format, and avcodec_send_packet crashes on the first
// packet.
//
// When the target HW format is found, this callback also fully initialises
// hw_frames_ctx using avcodec_get_hw_frames_parameters (codec-specific
// constraints) with a fallback to NV12 + a 20-frame pool.  Setting
// hw_frames_ctx here is the pattern from FFmpeg's hw_decode.c example.
//
// ctx->opaque must hold the desired HW pixel format cast to void* (intptr_t).
//
// NOTE: D3D11VA decoders may offer either AV_PIX_FMT_D3D11 or
// AV_PIX_FMT_D3D11VA_VLD in the pix_fmts list depending on the FFmpeg
// version.  We accept both so the callback works with old and new builds.
static enum AVPixelFormat vt_get_hw_format(AVCodecContext *ctx,
                                            const enum AVPixelFormat *pix_fmts)
{
    enum AVPixelFormat target = (enum AVPixelFormat)(intptr_t)ctx->opaque;
    const enum AVPixelFormat *p;
    AVBufferRef *frames_ref;
    AVHWFramesContext *fc;
    int w, h;

    for (p = pix_fmts; *p != AV_PIX_FMT_NONE; p++) {
        // Accept the target format OR the legacy D3D11VA_VLD equivalent.
        // FFmpeg versions differ on which format name D3D11VA offers.
        if (*p != target
            && !(target == AV_PIX_FMT_D3D11 && *p == AV_PIX_FMT_D3D11VA_VLD))
            continue;

        // Use the codec-offered format, not our target, so sw_format and
        // hw_frames_ctx match what the decoder actually produces.
        enum AVPixelFormat chosen = *p;

        frames_ref = NULL;

        // Try codec-specific constraints first (needs SPS parsed; may return
        // an error on first call before SPS is available — that is expected).
        if (avcodec_get_hw_frames_parameters(ctx, ctx->hw_device_ctx,
                                             chosen, &frames_ref) < 0
                || frames_ref == NULL) {
            // Fall back: build a frames context with safe defaults.
            frames_ref = av_hwframe_ctx_alloc(ctx->hw_device_ctx);
            if (!frames_ref)
                return AV_PIX_FMT_NONE;

            fc            = (AVHWFramesContext *)frames_ref->data;
            w             = ctx->coded_width  ? ctx->coded_width  : ctx->width;
            h             = ctx->coded_height ? ctx->coded_height : ctx->height;
            fc->format            = chosen;
            fc->sw_format         = AV_PIX_FMT_NV12;
            fc->width             = w;
            fc->height            = h;
            fc->initial_pool_size = 20;
        }

        if (av_hwframe_ctx_init(frames_ref) < 0) {
            av_buffer_unref(&frames_ref);
            return AV_PIX_FMT_NONE;
        }

        // Attach the initialised pool to the codec context.
        av_buffer_unref(&ctx->hw_frames_ctx);
        ctx->hw_frames_ctx = av_buffer_ref(frames_ref);
        av_buffer_unref(&frames_ref);

        if (!ctx->hw_frames_ctx)
            return AV_PIX_FMT_NONE;

        return chosen;
    }

    return AV_PIX_FMT_NONE;
}

// vt_set_get_format wires the get_format callback, stores the target HW pixel
// format in ctx->opaque, and forces single-threaded decode.  D3D11VA (and
// other HW backends) do not support frame-level multi-threading: each worker
// thread gets a copy of the codec context with hw_frames_ctx still NULL,
// causing a crash in avcodec_send_packet.
// CGo cannot assign C function pointers to struct fields directly, so we use
// a C helper for the assignment.
//
// For D3D11VA, we prefer AV_PIX_FMT_D3D11 (new API) but the vt_get_hw_format
// callback also accepts AV_PIX_FMT_D3D11VA_VLD (legacy) to cover builds where
// the codec offers the old format name.
static void vt_set_get_format(AVCodecContext *ctx, int hw_pix_fmt) {
    ctx->opaque       = (void*)(intptr_t)hw_pix_fmt;
    ctx->get_format   = vt_get_hw_format;
    ctx->thread_count = 1;
    ctx->thread_type  = 0;
}
*/
import "C"
import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/media/filters"
)

// decodedFrame is a fully decoded and colour-converted video frame ready for display.
type decodedFrame struct {
	img *image.RGBA
	pts float64
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

// hwDecodeEnabled controls whether hardware-accelerated video decoding is used.
// D3D11VA crashes in avcodec_send_packet are C-level access violations that
// Go's recover() cannot catch, killing the process instantly.  Until the
// C bridge (safe_bridge.c) wraps FFmpeg calls in SEH/__try, HW decode is
// disabled to guarantee process stability.  SW decode handles 720p/1080p
// H.264/HEVC reliably on any modern CPU.
var hwDecodeEnabled = false

// SetHWDecodeEnabled allows the caller (e.g. Settings) to opt in to HW
// decode.  The default is off because D3D11VA crashes cannot be caught by Go.
func SetHWDecodeEnabled(enabled bool) {
	hwDecodeEnabled = enabled
}

func HWDecodeEnabled() bool {
	return hwDecodeEnabled
}

// hwDeviceDetected caches the result of the one-time hardware detection so
// that av_hwdevice_ctx_create is never called from a background goroutine
// after the GLFW message loop has started (on Windows, D3D11VA device creation
// uses COM STA dispatch and deadlocks with the GLFW message pump).
var (
	hwDeviceDetected   HWDeviceType
	hwDeviceDetectOnce sync.Once
)

func doHWDetect() {
	if checkVAAPIAvailable() {
		hwDeviceDetected = HWDeviceVAAPI
	} else if checkD3D11VAAvailable() {
		hwDeviceDetected = HWDeviceD3D11VA
	} else if checkQSVAvailable() {
		hwDeviceDetected = HWDeviceQSV
	}
	// HWDeviceNone is the zero value; no else branch needed.
}

// WarmHWDeviceCache runs the hardware-detection probes synchronously on the
// calling goroutine regardless of the hwDecodeEnabled flag, then caches the
// result.  Call this once from the main goroutine before ShowAndRun() so that
// subsequent DetectHWDevice() calls from background goroutines always return
// the cached value without touching COM.
//
// The unconditional probe is required to handle the case where the user starts
// with HW decode disabled and enables it mid-session via Settings: without this
// the sync.Once would fire later from a Load() background goroutine and
// deadlock with the GLFW message pump (Windows D3D11VA COM STA).
func WarmHWDeviceCache() {
	hwDeviceDetectOnce.Do(doHWDetect)
}

func DetectHWDevice() HWDeviceType {
	if !hwDecodeEnabled {
		return HWDeviceNone
	}
	// The sync.Once was already fired by WarmHWDeviceCache() at startup so
	// this Do() is a no-op on all subsequent calls from background goroutines.
	hwDeviceDetectOnce.Do(doHWDetect)
	return hwDeviceDetected
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

// codecCanUseHWDevice returns true only for codecs that are known to work
// correctly with hw_device_ctx and no get_format callback.
//
// Several codecs (notably VC-1/WMV3 and MPEG-2) advertise D3D11VA support via
// avcodec_get_hw_config but require a get_format callback to negotiate the HW
// pixel format at decode time.  Without that callback the decoder selects a SW
// format, then crashes inside avcodec_send_packet when it tries to allocate HW
// surfaces against the un-negotiated format.
//
// The codecs listed below have been validated to work with the simpler
// hw_device_ctx-only path (no get_format callback):
//
//	h264, hevc, vp9, av1, vp8
//
// Everything else falls back to software decode.
func (e *Engine) codecCanUseHWDevice(codec *C.AVCodec) bool {
	name := C.GoString((*C.char)(unsafe.Pointer(codec.name)))
	switch name {
	case "h264", "hevc", "vp9", "av1", "vp8":
		// Good — these work reliably with hw_device_ctx alone.
	default:
		return false
	}

	// Also verify the codec actually has an HW config entry for this device.
	var hwType C.enum_AVHWDeviceType
	switch e.hwDevice {
	case HWDeviceVAAPI:
		hwType = C.AV_HWDEVICE_TYPE_VAAPI
	case HWDeviceD3D11VA:
		hwType = C.AV_HWDEVICE_TYPE_D3D11VA
	case HWDeviceQSV:
		hwType = C.AV_HWDEVICE_TYPE_QSV
	default:
		return false
	}
	for i := C.int(0); ; i++ {
		cfg := C.avcodec_get_hw_config(codec, i)
		if cfg == nil {
			return false
		}
		if cfg.methods&C.AV_CODEC_HW_CONFIG_METHOD_HW_DEVICE_CTX != 0 &&
			cfg.device_type == hwType {
			return true
		}
	}
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

	// Attach hw_device_ctx; FFmpeg will use it to set up the HW frames context
	// once get_format selects the HW pixel format.
	// We keep our own ref in e.hwDeviceCtx for cleanup.
	e.videoCodecCtx.hw_device_ctx = C.av_buffer_ref(devCtxRef)
	if e.videoCodecCtx.hw_device_ctx == nil {
		C.av_buffer_unref(&devCtxRef)
		return fmt.Errorf("failed to attach HW device context to codec ctx")
	}
	e.hwDeviceCtx = devCtxRef

	// Set get_format callback so FFmpeg can properly negotiate the HW pixel
	// format at decode time.  Without this, FFmpeg's internal avcodec_send_packet
	// path crashes on the first packet when it tries to lazily allocate the HW
	// frame pool against an un-negotiated format (observed with H.264 + D3D11VA).
	//
	// vt_set_get_format is a C helper because CGo cannot assign C function
	// pointers to struct fields from Go code directly.
	hwFmt := e.getHWPixelFormat(hwType)
	C.vt_set_get_format(e.videoCodecCtx, C.int(hwFmt))

	logging.Info(logging.CatPlayer, "HW decode enabled: %v (hwFmt=%d)", e.hwDevice, int(hwFmt))
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
	framepoolMu  sync.Mutex // protects framePool only — must NOT be acquired under videoCodecMu

	videoTimeBase    float64
	audioTimeBase    float64
	subtitleTimeBase float64

	mu           sync.Mutex
	formatMu     sync.Mutex // serialises av_read_frame vs avformat_seek_file
	videoCodecMu sync.Mutex // serialises avcodec_send_packet / avcodec_receive_frame on videoCodecCtx
	demuxerWg    sync.WaitGroup // tracks demuxerLoop goroutine; Close waits before freeing contexts
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
	// seekFlushBefore is read/written by both Seek() and videoDecodeLoop.
	// Seek() holds e.mu; videoDecodeLoop holds videoCodecMu at the read site.
	// Acquiring e.mu inside videoCodecMu creates a lock-order deadlock with
	// Seek() (which does e.mu → videoCodecMu).  Atomic access eliminates
	// the nested lock entirely — no mutex needed for this field.
	seekFlushBefore  atomic.Uint64 // math.Float64bits; 0 = guard inactive
	lastVideoPTSBits atomic.Uint64 // math.Float64bits of the last video PTS handed to the display
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
		lastError:         nil,
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
}

func (e *Engine) DrainAudio() {
	if e.audioPlayer != nil {
		e.audioPlayer.DrainPCM()
	}
}

func (e *Engine) Resume() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		logging.Info(logging.CatPlayer, "Engine.Resume: not running, returning")
		return
	}
	if !e.paused {
		e.mu.Unlock()
		logging.Info(logging.CatPlayer, "Engine.Resume: not paused, returning")
		return
	}
	e.paused = false
	e.clock.SetPaused(false)

	// Start the decode-ahead goroutine on the first Resume after Start().
	// We defer this to Resume (not Start) because GrabFrame runs between
	// Start and the first Play and reads videoQueue directly — starting the
	// decode loop earlier would race with GrabFrame over the same packets.
	if !e.decodeLoopActive {
		e.decodeLoopActive = true
		e.decodeLoopWg.Add(1)
		go e.videoDecodeLoop()
	}
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

	// Cap probe depth so unusual files don't stall the engine indefinitely.
	// probesize is in bytes; max_analyze_duration is in AV_TIME_BASE units (µs).
	// 10 MB / 10 s is generous enough for any well-formed file, including those
	// with DTS/AC-3 audio that need more packets than AAC.
	e.formatCtx.probesize = C.int64_t(10_000_000)
	e.formatCtx.max_analyze_duration = C.int64_t(10_000_000)

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
		codecCtx.thread_count = 2
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

func (e *Engine) Start() {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.paused = true
	e.mu.Unlock()

	logging.Info(logging.CatPlayer, "Engine.Start: starting demuxerLoop")
	// Clock stays paused here — Resume() unpauses it on the first Play().
	// Start() runs during Load (before the user presses Play); if we unpaused
	// here the clock would tick through the entire idle window and the first
	// video frame would be seconds behind by the time Play is pressed.

	e.demuxerWg.Add(1)
	go e.demuxerLoop()
}

func (e *Engine) demuxerLoop() {
	defer e.demuxerWg.Done()
	defer func() {
		if r := recover(); r != nil {
			logging.Error(logging.CatPlayer, "demuxerLoop panic: %v", r)
			e.videoQueue.SetEOF()
			e.audioQueue.SetEOF()
			e.subtitleQueue.SetEOF()
		}
	}()

	logging.Info(logging.CatPlayer, "demuxerLoop: started (vidIdx=%d audioIdx=%d)", e.videoStreamIdx, e.audioStreamIdx)

	pkt := C.av_packet_alloc()
	if pkt == nil {
		logging.Error(logging.CatPlayer, "demuxerLoop: av_packet_alloc returned nil")
		e.videoQueue.SetEOF()
		e.audioQueue.SetEOF()
		return
	}
	defer C.av_packet_free(&pkt)

	firstPkt := true
	for {
		select {
		case <-e.stop:
			logging.Info(logging.CatPlayer, "demuxerLoop: stop signal received, exiting")
			return
		default:
		}

		// Serialise av_read_frame against avformat_seek_file — AVFormatContext
		// is not thread-safe; concurrent access from Seek() causes hard crashes.
		e.formatMu.Lock()
		ret := C.av_read_frame(e.formatCtx, pkt)
		e.formatMu.Unlock()

		if firstPkt {
			firstPkt = false
			logging.Info(logging.CatPlayer, "demuxerLoop: first av_read_frame ret=%d stream=%d", int(ret), int(pkt.stream_index))
		}

		if ret < 0 {
			logging.Info(logging.CatPlayer, "demuxerLoop: av_read_frame EOF/error ret=%d, setting queue EOF", int(ret))
			e.videoQueue.SetEOF()
			e.audioQueue.SetEOF()
			e.subtitleQueue.SetEOF()
			return
		}

		streamIdx := int(pkt.stream_index)
		if streamIdx == e.videoStreamIdx {
			e.videoQueue.Put(pkt) // blocking: never drop video packets
		} else if streamIdx == e.audioStreamIdx {
			// Non-blocking: if the audio queue is saturated, discard the
			// packet rather than stalling the demuxer and starving the video
			// queue.  A skipped AAC frame (23 ms) is inaudible compared to
			// a several-second video freeze.
			e.audioQueue.TryPut(pkt)
		} else if streamIdx == e.subtitleStreamIdx && e.subtitleCodecCtx != nil {
			e.subtitleQueue.TryPut(pkt)
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
	var minTS C.int64_t
	switch e.seekAcc {
	case SeekAccuracyFrame:
		flags = C.int(AVSEEK_FLAG_FRAME)
		minTS = target
	case SeekAccuracyKeyframe:
		// AVSEEK_FLAG_BACKWARD: seek to the keyframe at or before target.
		// Widening min_ts to 0 allows FFmpeg to find the preceding keyframe
		// when target falls between keyframes, which is the common case.
		// Without this, avformat_seek_file with min=target often fails for
		// backward seeks, leaving the position unchanged.
		flags = C.int(AVSEEK_FLAG_BACKWARD)
		minTS = 0
	case SeekAccuracyAccurate:
		flags = C.int(AVSEEK_FLAG_BACKWARD | AVSEEK_FLAG_ACCURATE)
		minTS = 0
	}

	e.formatMu.Lock()
	seekRet := C.avformat_seek_file(e.formatCtx, C.int(e.videoStreamIdx), minTS, target, target, flags)
	e.formatMu.Unlock()
	if seekRet < 0 {
		logging.Warning(logging.CatPlayer, "Seek to %.2f failed (ret=%d)", seconds, seekRet)
		return fmt.Errorf("seek failed")
	}
	logging.Info(logging.CatPlayer, "Seek to %.2f OK, flushing queues", seconds)

	e.videoQueue.Flush()
	e.audioQueue.Flush()
	logging.Info(logging.CatPlayer, "Seek: queues flushed, flushing video codec")

	// Re-set audio stream index after seek to prevent audio jumping to start
	// Some files with multiple audio tracks may have FFmpeg switch streams after seek.
	if e.audioStreamIdx >= 0 && e.formatCtx != nil {
		stream := C.avformat_get_stream(e.formatCtx, C.uint(e.audioStreamIdx))
		if stream == nil || stream.codecpar == nil {
			logging.Warning(logging.CatPlayer, "Seek: audio stream %d no longer valid, searching for new audio stream", e.audioStreamIdx)
			// Find a new audio stream
			for i := 0; i < int(e.formatCtx.nb_streams); i++ {
				s := C.avformat_get_stream(e.formatCtx, C.uint(i))
				if s != nil && s.codecpar != nil && s.codecpar.codec_type == C.AVMEDIA_TYPE_AUDIO {
					e.audioStreamIdx = i
					logging.Info(logging.CatPlayer, "Seek: re-set audio stream to %d", i)
					break
				}
			}
		}
	}

	// Set the pre-seek skip guard BEFORE flushing the codec.  After the queue
	// flush the demuxer immediately starts producing new-position packets;
	// videoDecodeLoop can pick one up and send it to the codec before we reach
	// avcodec_flush_buffers below.  Without the guard those early reference
	// frames would be converted to RGBA and queued, only to be dropped by
	// NextFrame.  Setting seekFlushBefore here ensures the decode loop skips
	// them even if it races ahead of the codec flush.
	e.seekFlushBefore.Store(math.Float64bits(seconds - 0.15))

	// videoCodecMu must be held around avcodec_flush_buffers to serialise
	// against NextFrame() which holds the same lock during send/receive.
	//
	// Guard: only flush if at least one frame has been decoded. For D3D11VA
	// hardware decoders the hardware frame pool is lazily allocated on the
	// first decoded frame; calling avcodec_flush_buffers before that happens
	// dereferences an uninitialized pool and causes an access violation crash.
	if e.videoCodecCtx != nil && e.videoDecoded {
		e.videoCodecMu.Lock()
		// Drain buffered frames before flushing so the codec's internal state is
		// clean.  avcodec_flush_buffers resets the codec for reuse after the drain.
		if _, sendExc := SafeSendPacket(e.videoCodecCtx, nil); sendExc != 0 {
			logging.Error(logging.CatPlayer, "Seek: flush send failed (exc=0x%08X)", sendExc)
			e.videoDecodeDead = true
			e.videoCodecMu.Unlock()
			return fmt.Errorf("flush send failed")
		}
		flushed := 0
		for {
			_, recvExc := SafeReceiveFrame(e.videoCodecCtx, e.frame)
			if recvExc != 0 {
				break
			}
			flushed++
		}
		C.avcodec_flush_buffers(e.videoCodecCtx)
		e.videoCodecMu.Unlock()
		logging.Info(logging.CatPlayer, "Seek: flushed %d frames", flushed)
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

	// Mirror the ResetAfterGrab latency offset: after flushing audio the oto
	// hardware buffer contains silence for ~AudioBufferLatency before new
	// audio from the seek position reaches the speakers.  Starting the clock
	// that amount behind the seek target makes WaitForPTS hold the first
	// post-seek video frame until audio output actually catches up.
	if e.audioPlayer != nil {
		e.clock.ResetTime(seconds - AudioBufferLatency.Seconds())
	} else {
		e.clock.ResetTime(seconds)
	}

	// Drain any pre-decoded frames from before the seek; they have stale PTS.
	for {
		select {
		case <-e.frameQueue:
		default:
			goto drainDone
		}
	}
drainDone:
	// Allow videoDecodeLoop to send a new EOF sentinel if needed after this seek.
	e.decodeEOFSent = false

	logging.Info(logging.CatPlayer, "Seek: complete at %.2f", seconds)
	return nil
}

// ResetAfterGrab repositions the format context to the start and resets all
// codec state after GrabFrame().  It is used exclusively by
// InlineVideoPlayer.Load() after GrabFrame() to prepare for clean playback.
func (e *Engine) ResetAfterGrab() {
	defer func() {
		if r := recover(); r != nil {
			logging.Error(logging.CatPlayer, "ResetAfterGrab panic: %v", r)
		}
	}()

	logging.Info(logging.CatPlayer, "ResetAfterGrab: flushing queues and resetting clock")
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.formatCtx == nil {
		return
	}

	// Flush queues so stale packets don't reach decode.
	e.videoQueue.Flush()
	e.audioQueue.Flush()

	// Drain any pre-buffered PCM chunks from audioDecodeLoop so the audio
	// clock doesn't jump forward when playback starts. audioDecodeLoop may have
	// pre-buffered several seconds of audio during GrabFrame before ResetAfterGrab
	// is called; if those chunks sit in pcmCh, the first AudioPlayer.Read() call
	// will consume them and set the clock to ~5s, dropping all initial video frames.
	if e.audioPlayer != nil {
		e.audioPlayer.DrainPCM()
	}

	// Flush the video codec to drain any frames GrabFrame left buffered inside
	// the decoder. H.264 B-frame reordering causes the codec to hold decoded
	// frames that were never retrieved after GrabFrame returned, making them
	// appear as the "first" frames when videoDecodeLoop starts (high-PTS frames
	// surfacing at position 0). videoDecodeLoop has not started yet so acquiring
	// videoCodecMu is safe here under the e.mu we already hold.
	if e.videoCodecCtx != nil && e.videoDecoded {
		e.videoCodecMu.Lock()
		SafeSendPacket(e.videoCodecCtx, nil)
		for {
			ret, _ := SafeReceiveFrame(e.videoCodecCtx, e.frame)
			if ret != 0 {
				break
			}
		}
		C.avcodec_flush_buffers(e.videoCodecCtx)
		e.videoCodecMu.Unlock()
		logging.Info(logging.CatPlayer, "ResetAfterGrab: video codec flushed (B-frame drain)")
	}

	// Reset clock unconditionally to 0. ResetTime (not SetTime) is required
	// here: SetTime is a monotonic ratchet and is silently a no-op if the
	// clock is already above 0.
	e.clock.ResetTime(0)
	e.clock.SetPaused(true)

	// Also reset the audio player's last PTS so the first Read()
	// doesn't jump the clock forward by pre-buffered audio.
	if e.audioPlayer != nil {
		e.audioPlayer.ResetLastPTS()
	}

	e.decodeEOFSent = false
	e.seekFlushBefore.Store(0)
	logging.Info(logging.CatPlayer, "ResetAfterGrab: done")
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
func (e *Engine) GrabFrame(timeout time.Duration) (retImg *image.RGBA, retErr error) {
	defer func() {
		if r := recover(); r != nil {
			logging.Error(logging.CatPlayer, "GrabFrame panic: %v", r)
			retImg = nil
			retErr = fmt.Errorf("GrabFrame panic: %v", r)
		}
	}()

	deadline := time.Now().Add(timeout)
	logging.Info(logging.CatPlayer, "GrabFrame: waiting for first video frame (timeout=%v, hwDevice=%v)", timeout, e.hwDevice)

	for time.Now().Before(deadline) {
		// Non-blocking fetch — retry until a packet arrives or EOF/timeout.
		pkt, ok := e.videoQueue.TryGet()
		if !ok {
			if e.videoQueue.IsClosedOrEOF() {
				logging.Info(logging.CatPlayer, "GrabFrame: video queue EOF/closed")
				return nil, io.EOF
			}
			time.Sleep(5 * time.Millisecond)
			continue
		}

		// Lock codec around send+receive so we don't race with NextFrame or
		// SmoothScrubbing if they start concurrently.
		logging.Info(logging.CatPlayer, "GrabFrame: sending packet to video codec")
		e.videoCodecMu.Lock()
		sendRet, excCode := SafeSendPacket(e.videoCodecCtx, pkt)
		C.av_packet_free(&pkt)
		if excCode != 0 {
			e.videoCodecMu.Unlock()
			logging.Error(logging.CatPlayer, "GrabFrame: avcodec_send_packet SEH exception (exc=0x%08X) — disabling video decode", excCode)
			e.videoDecodeDead = true
			return nil, fmt.Errorf("video decode access violation: 0x%08X", excCode)
		}
		if sendRet != 0 {
			e.videoCodecMu.Unlock()
			logging.Info(logging.CatPlayer, "GrabFrame: avcodec_send_packet returned %d, skipping", int(sendRet))
			continue // bad or non-video packet; try the next one
		}

		logging.Info(logging.CatPlayer, "GrabFrame: packet sent OK, calling avcodec_receive_frame")
		// Drain all frames produced by this packet; return the first valid one.
		for {
			recvRet, recvExc := SafeReceiveFrame(e.videoCodecCtx, e.frame)
			if recvExc != 0 {
				logging.Error(logging.CatPlayer, "GrabFrame: avcodec_receive_frame SEH exception (exc=0x%08X) — disabling video decode", recvExc)
				e.videoDecodeDead = true
				e.videoCodecMu.Unlock()
				return nil, fmt.Errorf("video decode access violation: 0x%08X", recvExc)
			}
			if recvRet != 0 {
				break // EAGAIN or EOF
			}
			e.videoDecoded = true

			// Skip frames with AV_NOPTS_VALUE PTS or zero dimensions.
			// These are codec artefacts (e.g. MPEG4 B-frames before the first
			// reference frame) and produce garbage when converted to RGBA.
			if e.frame.pts == C.AV_NOPTS_VALUE || e.frame.pts < 0 ||
				e.frame.width <= 0 || e.frame.height <= 0 {
				logging.Info(logging.CatPlayer, "GrabFrame: skipping invalid frame pts=%d w=%d h=%d", int64(e.frame.pts), int(e.frame.width), int(e.frame.height))
				continue
			}

			pts := float64(e.frame.pts) * e.videoTimeBase
			logging.Info(logging.CatPlayer, "GrabFrame: got frame pts=%.3f hw_frames_ctx=%v", pts, e.frame.hw_frames_ctx != nil)

			var img *image.RGBA
			if e.hwDevice != HWDeviceNone {
				var err error
				img, err = e.retrieveHWFrame()
				if err != nil {
					logging.Warning(logging.CatPlayer, "GrabFrame: HW retrieve failed (%v)", err)
					if e.frame.hw_frames_ctx != nil {
						logging.Info(logging.CatPlayer, "GrabFrame: frame is HW, cannot SW fallback — skipping")
						e.videoCodecMu.Unlock()
						continue
					}
					e.ensureSwsCtx(C.enum_AVPixelFormat(e.frame.format))
					img = e.toRGBA()
				}
			} else {
				e.ensureSwsCtx(C.enum_AVPixelFormat(e.frame.format))
				img = e.toRGBA()
			}
			e.videoCodecMu.Unlock()
			return img, nil
		}
		e.videoCodecMu.Unlock()
	}

	return nil, fmt.Errorf("timed out waiting for first video frame")
}

// sendToFrameQueue puts df into e.frameQueue, retrying until space is available,
// the decode loop is stopped, or the engine has been paused long enough that the
// full buffer (preDecodeFrames × frame-time) already covers the pause.
// Returns false only when the decode loop should exit entirely.
func (e *Engine) sendToFrameQueue(df decodedFrame) bool {
	pauseRetries := 0
	for {
		select {
		case e.frameQueue <- df:
			return true
		case <-e.decodeLoopStop:
			return false
		case <-time.After(5 * time.Millisecond):
			e.mu.Lock()
			paused := e.paused
			e.mu.Unlock()
			if paused {
				pauseRetries++
				// After 15 ms (3 × 5 ms) with a full queue while paused, drop
				// this frame. The queue already holds preDecodeFrames frames
				// (~267 ms at 30 fps), which is more than enough to resume
				// smoothly. Dropping one frame here is imperceptible.
				if pauseRetries >= 3 {
					return true
				}
			} else {
				pauseRetries = 0
			}
		}
	}
}

// videoDecodeLoop is the dedicated decode goroutine.  It reads packets from
// videoQueue, calls avcodec_send/receive_frame under videoCodecMu, converts
// each frame to RGBA, and queues the result in frameQueue for NextFrame to
// consume.  This decouples I-frame decode latency from the display path.
func (e *Engine) videoDecodeLoop() {
	defer e.decodeLoopWg.Done()
	defer func() {
		if r := recover(); r != nil {
			logging.Error(logging.CatPlayer, "videoDecodeLoop panic: %v", r)
		}
	}()

	logging.Info(logging.CatPlayer, "videoDecodeLoop: started")

	for {
		// Check stop signal without blocking.
		select {
		case <-e.decodeLoopStop:
			logging.Info(logging.CatPlayer, "videoDecodeLoop: stopped")
			return
		default:
		}

		e.mu.Lock()
		paused := e.paused
		e.mu.Unlock()

		if paused {
			// When paused with at least one queued frame, sleep — the queue
			// already has a seek-preview frame for NextFrame to return.
			// When the queue is empty (e.g. immediately after a seek while
			// paused) fall through and decode one frame so Seek() can get
			// a preview without hanging.
			if len(e.frameQueue) >= 1 {
				time.Sleep(10 * time.Millisecond)
				continue
			}
		}

		// Non-blocking packet fetch so we can check stop/pause between packets.
		rawPkt, ok := e.videoQueue.TryGet()
		if !ok {
			if e.videoQueue.IsClosedOrEOF() {
				// Only send the EOF sentinel once per stream.
				e.mu.Lock()
				sent := e.decodeEOFSent
				e.mu.Unlock()
				if !sent {
					e.mu.Lock()
					e.decodeEOFSent = true
					e.mu.Unlock()
					e.sendToFrameQueue(decodedFrame{pts: decodeEOFPTS})
				}
			}
			time.Sleep(1 * time.Millisecond)
			continue
		}

		// Decode the packet under videoCodecMu.
		e.videoCodecMu.Lock()
		sendRet, excCode := SafeSendPacket(e.videoCodecCtx, rawPkt)
		C.av_packet_free(&rawPkt)
		if excCode != 0 {
			e.videoCodecMu.Unlock()
			logging.Error(logging.CatPlayer, "videoDecodeLoop: avcodec_send_packet SEH exception (exc=0x%08X) — stopping decode", excCode)
			e.videoDecodeDead = true
			return
		}
		if sendRet != 0 {
			e.videoCodecMu.Unlock()
			continue
		}

		for {
			recvRet, recvExc := SafeReceiveFrame(e.videoCodecCtx, e.frame)
			if recvExc != 0 {
				e.videoCodecMu.Unlock()
				logging.Error(logging.CatPlayer, "videoDecodeLoop: avcodec_receive_frame SEH exception (exc=0x%08X) — stopping decode", recvExc)
				e.videoDecodeDead = true
				return
			}
			if recvRet != 0 {
				break
			}
			e.videoDecoded = true

			if e.frame.pts == C.AV_NOPTS_VALUE || e.frame.pts < 0 {
				continue
			}

			pts := float64(e.frame.pts) * e.videoTimeBase

			// Skip RGBA conversion for pre-seek reference frames.  After a
			// keyframe seek the codec must decode all frames from the GOP
			// boundary to the seek target; converting those to RGBA wastes
			// CPU and fills frameQueue with stale frames that NextFrame will
			// only drop.  We still need avcodec_receive_frame for the
			// reference-frame chain, so we can't skip the decode itself.
			// seekFlushBefore is atomic so we can read it here without acquiring
			// e.mu — holding e.mu inside videoCodecMu creates a lock-order
			// deadlock with Seek() (which does e.mu → videoCodecMu).
			flushBefore := math.Float64frombits(e.seekFlushBefore.Load())
			if flushBefore > 0 && pts < flushBefore {
				// Release videoCodecMu so Seek()'s avcodec_flush_buffers can
				// proceed if it is waiting.  After re-acquiring, the next
				// avcodec_receive_frame call returns EAGAIN if the codec was
				// flushed, cleanly exiting the inner loop.
				e.videoCodecMu.Unlock()
				runtime.Gosched()
				e.videoCodecMu.Lock()
				continue
			}
			if flushBefore > 0 {
				e.seekFlushBefore.Store(0)
			}

			// Convert to RGBA while still holding videoCodecMu — e.frame is
			// owned by the codec context and can be overwritten on the next
			// avcodec_receive_frame call.
			var img *image.RGBA
			if e.hwDevice != HWDeviceNone {
				var err error
				img, err = e.retrieveHWFrame()
				if err != nil {
					logging.Warning(logging.CatPlayer, "videoDecodeLoop: HW retrieve failed: %v", err)
					if e.frame.hw_frames_ctx != nil {
						e.videoCodecMu.Unlock()
						continue
					}
					e.ensureSwsCtx(C.enum_AVPixelFormat(e.frame.format))
					img = e.toRGBA()
				}
			} else {
				e.ensureSwsCtx(C.enum_AVPixelFormat(e.frame.format))
				img = e.toRGBA()
			}

			e.videoCodecMu.Unlock()

			if !e.sendToFrameQueue(decodedFrame{img: img, pts: pts}) {
				return // decodeLoopStop was closed
			}

			e.videoCodecMu.Lock()
		}
		e.videoCodecMu.Unlock()
	}
}

func (e *Engine) NextFrame() (retImg *image.RGBA, retErr error) {
	defer func() {
		if r := recover(); r != nil {
			logging.Error(logging.CatPlayer, "NextFrame panic: %v", r)
			retImg = nil
			retErr = fmt.Errorf("NextFrame panic: %v", r)
		}
	}()

	nf := atomic.AddInt64(&e.nextFrameCount, 1)
	verbose := nf <= 20

	for {
		e.mu.Lock()
		paused := e.paused
		hasAudio := e.hasAudio
		e.mu.Unlock()

		// Read from the pre-decoded frame queue.  videoDecodeLoop fills it
		// asynchronously so the display goroutine never blocks on codec work.
		var df decodedFrame
		select {
		case df = <-e.frameQueue:
			// fast path — frame ready without stalling
		default:
			if paused {
				// Queue empty while paused: decode loop will fill one frame
				// for a seek preview; yield and retry rather than hanging.
				time.Sleep(10 * time.Millisecond)
				continue
			}
			// Stall during active playback (startup latency or I-frame decode).
			// Pause the clock so wall time during the stall doesn't accumulate
			// as A/V drift, then block until the decode loop catches up.
			if hasAudio {
				e.clock.SetPaused(true)
			}
			df = <-e.frameQueue
			if hasAudio {
				// Resume clock at the paused position — don't ResetTime here.
				// AudioPlayer.Read() drives the clock via SetTime(pts-latency),
				// so it self-corrects to audio output position after any stall.
				e.clock.SetPaused(e.IsPaused())
			}
		}

		// EOF sentinel from videoDecodeLoop.
		if df.pts == decodeEOFPTS {
			if e.IsLooping() {
				if err := e.Seek(0); err != nil {
					return nil, err
				}
				continue
			}
			return nil, io.EOF
		}

		pts := df.pts
		img := df.img

		if verbose {
			clockNow := e.clock.GetTime()
			logging.Info(logging.CatPlayer, "NextFrame #%d: pts=%.3f clockNow=%.3f", nf, pts, clockNow)
		}

		// A/V sync: wait for the master clock to reach this frame's PTS.
		traceAction := "display"
		if hasAudio {
			e.clock.WaitForPTS(pts)
			// Snap: after a frameQueue stall (I-frame decode delay) or a startup
			// burst, audio may have advanced c.pts past pts+threshold while the
			// clock was nominally paused. If we let SyncVideo see that overshoot
			// it drops the frame and starts a cascade. Resetting to pts here
			// displays the frame; audio SetTime() calls re-advance the clock
			// within 1-2 frame periods.
			if e.clock.GetTime()-pts >= MaxDriftThreshold {
				e.clock.ResetTime(pts)
				traceAction = "snap"
			}
		} else {
			e.clock.SetTime(pts)
		}

		clockNow := e.clock.GetTime()
		delay := e.clock.SyncVideo(pts)
		if delay < 0 {
			logging.Warning(logging.CatPlayer, "frame DROP #%d pts=%.3f clock=%.3f behind=%.0fms", nf, pts, clockNow, (clockNow-pts)*1000)
			logging.PlayerFrameTrace(nf, pts, clockNow, "drop", (clockNow-pts)*1000)
			continue
		}
		if delay == 0 && clockNow-pts > 0.010 {
			logging.Debug(logging.CatPlayer, "frame LATE #%d pts=%.3f clock=%.3f behind=%.0fms", nf, pts, clockNow, (clockNow-pts)*1000)
		}
		logging.PlayerFrameTrace(nf, pts, clockNow, traceAction, (clockNow-pts)*1000)

		e.lastVideoPTSBits.Store(math.Float64bits(pts))

		if e.subtitleCodecCtx != nil {
			sub := e.decodeSubtitle(pts)
			if sub != nil {
				img = e.RenderSubtitles(img, pts)
			}
		}

		return img, nil
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

	// Transfer HW surface to CPU memory. This must happen while videoCodecMu
	// is held because e.frame (the HW frame) is owned by the codec context
	// and can be overwritten by the next avcodec_receive_frame call.
	if C.av_hwframe_transfer_data(swFrame, e.frame, 0) != 0 {
		logging.Warning(logging.CatPlayer, "retrieveHWFrame: av_hwframe_transfer_data failed")
		return nil, fmt.Errorf("failed to transfer HW frame to SW")
	}

	swFmt := C.enum_AVPixelFormat(swFrame.format)
	w := int(swFrame.width)
	h := int(swFrame.height)

	// Reuse cached sws context if format/width/height unchanged.
	if e.hwSwsCtx == nil || e.hwSwsFmt != swFmt || e.hwSwsW != w || e.hwSwsH != h {
		if e.hwSwsCtx != nil {
			C.sws_freeContext(e.hwSwsCtx)
		}
		e.hwSwsCtx = C.sws_getContext(
			swFrame.width, swFrame.height, swFmt,
			swFrame.width, swFrame.height, C.AV_PIX_FMT_RGBA,
			C.SWS_BILINEAR, nil, nil, nil,
		)
		if e.hwSwsCtx == nil {
			return nil, fmt.Errorf("failed to create sws context for hw pixel format %d", int(swFmt))
		}
		e.hwSwsFmt = swFmt
		e.hwSwsW = w
		e.hwSwsH = h
		logging.Info(logging.CatPlayer, "retrieveHWFrame: created cached swsCtx for fmt=%d %dx%d", int(swFmt), w, h)
	}

	rowStride := C.int(w * 4)
	bufSize := w * h * 4
	if len(e.hwRgbaBuffer) < bufSize || e.hwRgbaFrame == nil {
		if e.hwRgbaFrame != nil {
			C.av_frame_free(&e.hwRgbaFrame)
		}
		e.hwRgbaFrame = C.av_frame_alloc()
		e.hwRgbaBuffer = make([]byte, bufSize)
		C.av_image_fill_arrays(
			&e.hwRgbaFrame.data[0], &e.hwRgbaFrame.linesize[0],
			(*C.uint8_t)(unsafe.Pointer(&e.hwRgbaBuffer[0])), C.AV_PIX_FMT_RGBA,
			C.int(w), C.int(h), 1,
		)
		// Ensure linesize matches our stride expectation
		e.hwRgbaFrame.linesize[0] = rowStride
	}

	C.sws_scale(
		e.hwSwsCtx,
		&swFrame.data[0], &swFrame.linesize[0],
		0, swFrame.height,
		&e.hwRgbaFrame.data[0], &e.hwRgbaFrame.linesize[0],
	)

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	srcStride := int(e.hwRgbaFrame.linesize[0])
	for y := 0; y < h; y++ {
		src := e.hwRgbaBuffer[y*srcStride : y*srcStride+w*4]
		dst := img.Pix[y*img.Stride : y*img.Stride+w*4]
		copy(dst, src)
	}
	return img, nil
}

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
		C.SWS_BICUBIC|C.SWS_ACCURATE_RND, nil, nil, nil,
	)
	if e.swsCtx != nil {
		e.swsFmt = fmt
		logging.Info(logging.CatPlayer, "ensureSwsCtx: created swsCtx for fmt=%d", int(fmt))
	} else {
		logging.Warning(logging.CatPlayer, "ensureSwsCtx: sws_getContext failed for fmt=%d", int(fmt))
		e.swsFmt = 0
	}
}

func (e *Engine) toRGBA() (img *image.RGBA) {
	defer func() {
		if r := recover(); r != nil {
			logging.Error(logging.CatPlayer, "toRGBA panic: %v", r)
			img = nil
		}
	}()

	logging.Debug(logging.CatPlayer, "toRGBA: entering sws_scale, swsCtx=%v", e.swsCtx != nil)
	C.sws_scale(
		e.swsCtx,
		&e.frame.data[0], &e.frame.linesize[0],
		0, e.videoCodecCtx.height,
		&e.rgbaFrame.data[0], &e.rgbaFrame.linesize[0],
	)

	w, h := int(e.videoCodecCtx.width), int(e.videoCodecCtx.height)

	e.framepoolMu.Lock()
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
	e.framepoolMu.Unlock()

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

	e.framepoolMu.Lock()
	defer e.framepoolMu.Unlock()

	if len(e.framePool) < 4 {
		buf := make([]byte, len(img.Pix))
		copy(buf, img.Pix)
		e.framePool = append(e.framePool, buf)
	}
}

func (e *Engine) GetFramePoolSize() int {
	e.framepoolMu.Lock()
	defer e.framepoolMu.Unlock()
	return len(e.framePool)
}

func (e *Engine) Duration() float64 {
	e.formatMu.Lock()
	defer e.formatMu.Unlock()
	if e.formatCtx == nil {
		return 0
	}
	return float64(e.formatCtx.duration) / float64(C.AV_TIME_BASE)
}

func (e *Engine) CurrentTime() float64 {
	return e.clock.GetTime()
}

// GetLastVideoPTS returns the PTS of the most recent video frame handed to the display.
// Returns -1 before any frame has been shown.
func (e *Engine) GetLastVideoPTS() float64 {
	bits := e.lastVideoPTSBits.Load()
	if bits == 0 {
		return -1
	}
	return math.Float64frombits(bits)
}

// GetLastAudioPTS returns the PTS of the most recent audio chunk output by the player.
// Returns -1 when there is no audio player or no audio has been output yet.
func (e *Engine) GetLastAudioPTS() float64 {
	if e.audioPlayer == nil {
		return -1
	}
	return e.audioPlayer.GetLastPTS()
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

	// Acquire videoCodecMu to protect codec context access against
	// concurrent NextFrame/avcodec_* calls.
	e.videoCodecMu.Lock()
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
	e.videoCodecMu.Unlock()

	e.lastError = &PlaybackError{
		Code:    ErrCodeHWAccel,
		Message: "Fell back to software decoding",
		Retry:   false,
	}
e.mu.Unlock()
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
	e.running = false
	e.paused = false
	e.mu.Unlock()

	// Signal the demuxer goroutine to exit.
	close(e.stop)

	// Closing the queues unblocks any goroutine blocked in queue.Put() (which
	// waits when the queue is full). This lets the demuxer proceed to the next
	// loop iteration where it will see <-e.stop and exit.
	e.videoQueue.Close()
	e.audioQueue.Close()
	if e.subtitleQueue != nil {
		e.subtitleQueue.Close()
	}

	// Stop the decode-ahead goroutine.  It must exit before we acquire
	// videoCodecMu below, because it holds that lock during avcodec_send/receive.
	// Closing decodeLoopStop unblocks sendToFrameQueue if the loop is waiting
	// there.  Closing videoQueue (done above) unblocks TryGet + IsClosedOrEOF.
	close(e.decodeLoopStop)
	e.decodeLoopWg.Wait()

	// Drain any pre-decoded frames so the channel is empty before GC.
	for {
		select {
		case <-e.frameQueue:
		default:
			goto closeDrainDone
		}
	}
closeDrainDone:

	// Wait for demuxerLoop to fully exit before freeing any FFmpeg contexts.
	// Without this wait, demuxerLoop may still be inside av_read_frame when we
	// call avformat_close_input, causing a use-after-free crash.
	e.demuxerWg.Wait()

	// Close audio before freeing the audio codec context.
	if e.audioPlayer != nil {
		e.audioPlayer.Close()
		e.audioPlayer = nil
	}

	// Acquire videoCodecMu before freeing the video codec context. This ensures
	// that any in-flight NextFrame decode (which holds videoCodecMu during
	// avcodec_send/receive) has completed before we pull the context out from
	// under it. After videoQueue.Close() above, the next Get() in NextFrame
	// returns !ok, so NextFrame will not attempt to acquire videoCodecMu again.
	e.videoCodecMu.Lock()
	if e.swsCtx != nil {
		C.sws_freeContext(e.swsCtx)
		e.swsCtx = nil
		e.swsFmt = 0
	}
	if e.videoCodecCtx != nil {
		if e.videoCodecCtx.hw_frames_ctx != nil {
			C.av_buffer_unref(&e.videoCodecCtx.hw_frames_ctx)
		}
		C.avcodec_free_context(&e.videoCodecCtx)
		e.videoCodecCtx = nil
	}
	if e.hwFramesCtx != nil {
		C.av_buffer_unref(&e.hwFramesCtx)
		e.hwFramesCtx = nil
	}
	if e.hwDeviceCtx != nil {
		C.av_buffer_unref(&e.hwDeviceCtx)
		e.hwDeviceCtx = nil
	}
	if e.frame != nil {
		C.av_frame_free(&e.frame)
		e.frame = nil
	}
	if e.rgbaFrame != nil {
		C.av_frame_free(&e.rgbaFrame)
		e.rgbaFrame = nil
	}
	if e.hwRgbaFrame != nil {
		C.av_frame_free(&e.hwRgbaFrame)
		e.hwRgbaFrame = nil
	}
	e.videoCodecMu.Unlock()

	if e.audioCodecCtx != nil {
		C.avcodec_free_context(&e.audioCodecCtx)
		e.audioCodecCtx = nil
	}
	if e.subtitleCodecCtx != nil {
		C.avcodec_free_context(&e.subtitleCodecCtx)
		e.subtitleCodecCtx = nil
	}

	// formatCtx is safe to free now: demuxerWg.Wait() guarantees demuxerLoop
	// has exited (no concurrent av_read_frame), and Seek holds e.mu which we
	// would need to call — but Close already set running=false so Seek returns
	// early. Direct access is safe here.
	if e.formatCtx != nil {
		C.avformat_close_input(&e.formatCtx)
		e.formatCtx = nil
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
