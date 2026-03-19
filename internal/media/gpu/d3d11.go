//go:build native_media && windows

package gpu

import (
	"fmt"
	"image"
	"image/draw"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	DXGI_FORMAT_R8G8B8A8_UNORM = 87
)

type D3D11Context struct {
	device      windows.Handle
	context     uintptr
	swapChain   uintptr
	texture     uintptr
	width       int
	height      int
	mu          sync.Mutex
	initialized bool
	adapterDesc string
}

func NewD3D11Context(width, height int) (*D3D11Context, error) {
	ctx := &D3D11Context{
		width:  width,
		height: height,
	}

	if err := ctx.init(); err != nil {
		return nil, err
	}

	return ctx, nil
}

func (c *D3D11Context) init() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.initialized = true
	return nil
}

func (c *D3D11Context) UploadTexture(img *image.RGBA) error {
	if img == nil {
		return fmt.Errorf("nil image")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return nil
}

func (c *D3D11Context) Render() error {
	return nil
}

func (c *D3D11Context) Present() error {
	return nil
}

func (c *D3D11Context) Resize(width, height int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.width = width
	c.height = height
}

func (c *D3D11Context) Delete() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.initialized = false
}

func (c *D3D11Context) IsAvailable() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.initialized
}

type D3D11Renderer struct {
	ctx   *D3D11Context
	avail bool
}

func NewD3D11Renderer() *D3D11Renderer {
	r := &D3D11Renderer{
		avail: false,
	}
	r.detect()
	return r
}

func (r *D3D11Renderer) detect() {
	r.avail = false
}

func (r *D3D11Renderer) IsAvailable() bool {
	return r.avail
}

func (r *D3D11Renderer) Name() string {
	return "Direct3D 11"
}

func (r *D3D11Renderer) MakeCurrent() error {
	return nil
}

func (r *D3D11Renderer) SwapBuffers() error {
	return nil
}

func (r *D3D11Renderer) Delete() {
}

type D3D11Texture struct {
	texture uintptr
	width   int
	height  int
	data    []byte
	pitch   int
	mu      sync.Mutex
}

func NewD3D11Texture(width, height int) (*D3D11Texture, error) {
	return &D3D11Texture{
		width:  width,
		height: height,
		pitch:  width * 4,
		data:   make([]byte, width*height*4),
	}, nil
}

func (t *D3D11Texture) Upload(img *image.RGBA) error {
	if img == nil {
		return fmt.Errorf("nil image")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	src := img
	srcBounds := src.Bounds()

	if srcBounds.Dx() != t.width || srcBounds.Dy() != t.height {
		newImg := image.NewRGBA(image.Rect(0, 0, t.width, t.height))
		draw.Draw(newImg, newImg.Bounds(), image.Black, image.Point{}, draw.Src)

		scaleX := float64(t.width) / float64(srcBounds.Dx())
		scaleY := float64(t.height) / float64(srcBounds.Dy())
		scale := scaleX
		if scaleY < scale {
			scale = scaleY
		}

		newW := int(float64(srcBounds.Dx()) * scale)
		newH := int(float64(srcBounds.Dy()) * scale)
		offsetX := (t.width - newW) / 2
		offsetY := (t.height - newH) / 2

		for y := 0; y < newH; y++ {
			for x := 0; x < newW; x++ {
				srcX := int(float64(x) / scale)
				srcY := int(float64(y) / scale)
				if srcX >= srcBounds.Dx() {
					srcX = srcBounds.Dx() - 1
				}
				if srcY >= srcBounds.Dy() {
					srcY = srcBounds.Dy() - 1
				}
				newImg.Set(x+offsetX, y+offsetY, src.At(srcX+srcBounds.Min.X, srcY+srcBounds.Min.Y))
			}
		}
		src = newImg
	}

	for y := 0; y < t.height; y++ {
		for x := 0; x < t.width; x++ {
			idx := (y*t.width + x) * 4
			r, g, b, a := src.At(x, y).RGBA()
			t.data[idx] = byte(r >> 8)
			t.data[idx+1] = byte(g >> 8)
			t.data[idx+2] = byte(b >> 8)
			t.data[idx+3] = byte(a >> 8)
		}
	}

	return nil
}

func (t *D3D11Texture) UploadBGRA(data []byte, width, height int) error {
	if len(data) == 0 {
		return fmt.Errorf("empty data")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if width != t.width || height != t.height {
		return fmt.Errorf("dimensions mismatch")
	}

	copy(t.data, data)
	return nil
}

func (t *D3D11Texture) Width() int {
	return t.width
}

func (t *D3D11Texture) Height() int {
	return t.height
}

func (t *D3D11Texture) Data() []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.data
}

func (t *D3D11Texture) Delete() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.data = nil
}

var _ unsafe.Pointer = unsafe.Pointer(nil)
