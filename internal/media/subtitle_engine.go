//go:build native_media

package media

/*
#cgo !windows pkg-config: libavcodec libavformat libswscale libavutil
#cgo windows CFLAGS: -IC:/ffmpeg/include
#cgo windows LDFLAGS: -LC:/ffmpeg/lib -lavcodec -lavformat -lavutil -lswscale -lswresample -lavfilter -lbcrypt -lSecur32 -lWs2_32 -lmfplat -lstrmiids -lavrt -lole32 -luser32 -Wl,--stack,4194304
#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libavutil/avutil.h>

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
*/
import "C"
import (
	"image"
	"image/color"
	"image/draw"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
)

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
