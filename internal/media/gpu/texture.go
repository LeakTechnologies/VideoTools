//go:build native_media

package gpu

import (
	"fmt"
	"image"
	"image/color"
	"sync"
	"sync/atomic"
	"unsafe"
)

type TextureFormat int

const (
	FormatRGBA TextureFormat = iota
	FormatBGRA
	FormatRGB
	FormatNV12
	FormatYUV420P
)

type TextureOptions struct {
	Format    TextureFormat
	Usage     uint32
	BindFlags uint32
	CPUAccess uint32
	MipLevels int
	Width     int
	Height    int
}

type TexturePool struct {
	pool      chan Texture
	maxSize   int
	available int32
	texWidth  int
	texHeight int
	texFormat TextureFormat
	mu        sync.Mutex
}

func NewTexturePool(factory func(w, h int) Texture, maxSize int) *TexturePool {
	pool := &TexturePool{
		pool:      make(chan Texture, maxSize),
		maxSize:   maxSize,
		texWidth:  -1,
		texHeight: -1,
	}

	for i := 0; i < maxSize; i++ {
		pool.pool <- factory(pool.texWidth, pool.texHeight)
	}

	return pool
}

func (p *TexturePool) Acquire(w, h int) (Texture, error) {
	if p.texWidth != w || p.texHeight != h {
		p.mu.Lock()
		p.texWidth = w
		p.texHeight = h
		p.mu.Unlock()

		for len(p.pool) > 0 {
			select {
			case tex := <-p.pool:
				tex.Delete()
			default:
				break
			}
		}
	}

	select {
	case tex := <-p.pool:
		atomic.AddInt32(&p.available, -1)
		return tex, nil
	default:
		return nil, fmt.Errorf("texture pool exhausted")
	}
}

func (p *TexturePool) Release(tex Texture) {
	if tex != nil {
		atomic.AddInt32(&p.available, 1)
		select {
		case p.pool <- tex:
		default:
			tex.Delete()
		}
	}
}

func (p *TexturePool) Available() int {
	return int(atomic.LoadInt32(&p.available))
}

func ConvertToRGBA(src *image.RGBA) *image.RGBA {
	if src == nil {
		return nil
	}

	if src.Stride == src.Rect.Size().X*4 {
		return src
	}

	dst := image.NewRGBA(src.Bounds())
	for y := src.Rect.Min.Y; y < src.Rect.Max.Y; y++ {
		for x := src.Rect.Min.X; x < src.Rect.Max.X; x++ {
			dst.Set(x, y, src.At(x, y))
		}
	}

	return dst
}

func ConvertBGRAToRGBA(data []byte, w, h int) []byte {
	if len(data) < w*h*4 {
		return data
	}

	result := make([]byte, len(data))
	for i := 0; i < w*h; i++ {
		offset := i * 4
		result[offset] = data[offset+2]
		result[offset+1] = data[offset+1]
		result[offset+2] = data[offset]
		result[offset+3] = data[offset+3]
	}

	return result
}

func ScaleImageToFit(src *image.RGBA, targetW, targetH int) (*image.RGBA, int, int, int, int) {
	if src == nil {
		return nil, 0, 0, 0, 0
	}

	srcW := src.Bounds().Dx()
	srcH := src.Bounds().Dy()

	if srcW == 0 || srcH == 0 {
		return nil, 0, 0, 0, 0
	}

	scaleX := float64(targetW) / float64(srcW)
	scaleY := float64(targetH) / float64(srcH)
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}

	newW := int(float64(srcW) * scale)
	newH := int(float64(srcH) * scale)

	offsetX := (targetW - newW) / 2
	offsetY := (targetH - newH) / 2

	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))

	for y := 0; y < newH; y++ {
		for x := 0; x < newW; x++ {
			srcX := int(float64(x) / scale)
			srcY := int(float64(y) / scale)
			if srcX >= srcW {
				srcX = srcW - 1
			}
			if srcY >= srcH {
				srcY = srcH - 1
			}
			dst.Set(x+offsetX, y+offsetY, src.At(srcX, srcY))
		}
	}

	return dst, newW, newH, offsetX, offsetY
}

func FillBlack(img *image.RGBA) {
	if img == nil {
		return
	}
	draw := img.Rect
	for y := draw.Min.Y; y < draw.Max.Y; y++ {
		for x := draw.Min.X; x < draw.Max.X; x++ {
			img.Set(x, y, color.Black)
		}
	}
}

type FrameScaler struct {
	tmpBuffer []byte
	tmpWidth  int
	tmpHeight int
}

func NewFrameScaler() *FrameScaler {
	return &FrameScaler{}
}

func (s *FrameScaler) Scale(src *image.RGBA, targetW, targetH int) (*image.RGBA, error) {
	if src == nil {
		return nil, fmt.Errorf("nil source image")
	}

	_, newH, _, _, _ := ScaleImageToFit(src, targetW, targetH)

	if newH <= 0 {
		return nil, fmt.Errorf("invalid scale result")
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	FillBlack(dst)

	_, _, offX, offY, _ := ScaleImageToFit(src, targetW, targetH)

	for y := 0; y < newH; y++ {
		for x := 0; x < targetW-offX*2; x++ {
			srcX := int(float64(x) * float64(src.Bounds().Dx()) / float64(targetW-offX*2))
			srcY := int(float64(y) * float64(src.Bounds().Dy()) / float64(newH))
			if srcX >= src.Bounds().Dx() {
				srcX = src.Bounds().Dx() - 1
			}
			if srcY >= src.Bounds().Dy() {
				srcY = src.Bounds().Dy() - 1
			}
			dst.Set(x+offX, y+offY, src.At(srcX+src.Bounds().Min.X, srcY+src.Bounds().Min.Y))
		}
	}

	return dst, nil
}

func GetBestRenderer() Renderer {
	d3d := NewD3D11Renderer()
	if d3d.IsAvailable() {
		return d3d
	}

	gl := NewGLRenderer()
	if gl.IsAvailable() {
		return gl
	}

	return nil
}

type GLRenderer struct {
	ctx   unsafe.Pointer
	avail bool
}

func NewGLRenderer() *GLRenderer {
	r := &GLRenderer{
		avail: false,
	}
	r.detect()
	return r
}

func (r *GLRenderer) detect() {
	r.avail = false
}

func (r *GLRenderer) IsAvailable() bool {
	return r.avail
}

func (r *GLRenderer) Name() string {
	return "OpenGL"
}

func (r *GLRenderer) MakeCurrent() error {
	return fmt.Errorf("opengl not available")
}

func (r *GLRenderer) SwapBuffers() error {
	return fmt.Errorf("opengl not available")
}

func (r *GLRenderer) CreateTexture(w, h int) (Texture, error) {
	return nil, fmt.Errorf("opengl not available")
}

func (r *GLRenderer) Delete() {
}

var unsafePtr interface{} = nil
