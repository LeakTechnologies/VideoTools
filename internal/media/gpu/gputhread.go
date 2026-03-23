//go:build native_media

package gpu

import (
	"fmt"
	"image"
	"image/draw"
	"sync"
	"unsafe"

	"github.com/go-gl/gl/v2.1/gl"
)

const (
	textureUnitVideo = 0
)

type GLTexture struct {
	id       uint32
	width    int
	height   int
	pbo      uint32
	pboIndex int
	data     []byte
	valid    bool
	mu       sync.RWMutex
}

func NewGLTexture(width, height int) (*GLTexture, error) {
	if err := initGLFW(); err != nil {
		return nil, fmt.Errorf("failed to init glfw: %v", err)
	}

	if err := gl.Init(); err != nil {
		return nil, fmt.Errorf("failed to init GL: %v", err)
	}

	var texID uint32
	gl.GenTextures(1, &texID)
	if texID == 0 {
		return nil, fmt.Errorf("failed to generate texture")
	}

	gl.BindTexture(gl.TEXTURE_2D, texID)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

	dataSize := width * height * 4
	gl.TexImage2D(
		gl.TEXTURE_2D,
		0,
		gl.RGBA,
		int32(width),
		int32(height),
		0,
		gl.RGBA,
		gl.UNSIGNED_BYTE,
		nil,
	)

	gl.BindTexture(gl.TEXTURE_2D, 0)

	var pboID uint32
	gl.GenBuffers(1, &pboID)

	return &GLTexture{
		id:       texID,
		width:    width,
		height:   height,
		pbo:      pboID,
		pboIndex: 0,
		data:     make([]byte, dataSize),
		valid:    false,
	}, nil
}

func (t *GLTexture) Upload(img *image.RGBA) error {
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

	if len(t.data) < len(src.Pix) {
		t.data = make([]byte, len(src.Pix))
	}
	copy(t.data, src.Pix)

	gl.BindTexture(gl.TEXTURE_2D, t.id)
	gl.TexSubImage2D(
		gl.TEXTURE_2D,
		0,
		0, 0,
		int32(t.width),
		int32(t.height),
		gl.RGBA,
		gl.UNSIGNED_BYTE,
		unsafe.Pointer(&t.data[0]),
	)

	t.valid = true
	return nil
}

func (t *GLTexture) UploadBGRA(data []byte, width, height int) error {
	if len(data) == 0 {
		return fmt.Errorf("empty data")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if width != t.width || height != t.height {
		return fmt.Errorf("dimensions mismatch: expected %dx%d, got %dx%d", t.width, t.height, width, height)
	}

	gl.BindTexture(gl.TEXTURE_2D, t.id)
	gl.TexSubImage2D(
		gl.TEXTURE_2D,
		0,
		0, 0,
		int32(width),
		int32(height),
		gl.RGBA,
		gl.UNSIGNED_BYTE,
		unsafe.Pointer(&data[0]),
	)

	t.valid = true
	return nil
}

func (t *GLTexture) Width() int {
	return t.width
}

func (t *GLTexture) Height() int {
	return t.height
}

func (t *GLTexture) IsValid() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.valid
}

func (t *GLTexture) Bind() {
	gl.ActiveTexture(gl.TEXTURE0 + textureUnitVideo)
	gl.BindTexture(gl.TEXTURE_2D, t.id)
}

func (t *GLTexture) Delete() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.pbo > 0 {
		gl.DeleteBuffers(1, &t.pbo)
		t.pbo = 0
	}
	if t.id > 0 {
		gl.DeleteTextures(1, &t.id)
		t.id = 0
	}
	t.valid = false
}

type GPUTextureUpload struct {
	texture  *GLTexture
	width    int
	height   int
	mu       sync.Mutex
	pool     chan *GLTexture
	poolSize int
}

func NewGPUTextureUpload(poolSize int) *GPUTextureUpload {
	return &GPUTextureUpload{
		poolSize: poolSize,
		pool:     make(chan *GLTexture, poolSize),
	}
}

func (u *GPUTextureUpload) SetSize(width, height int) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.width == width && u.height == height && u.texture != nil {
		return
	}

	if u.texture != nil {
		u.texture.Delete()
	}

	tex, err := NewGLTexture(width, height)
	if err != nil {
		return
	}

	u.texture = tex
	u.width = width
	u.height = height
}

func (u *GPUTextureUpload) UploadFrame(img *image.RGBA) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if img == nil {
		return fmt.Errorf("nil frame")
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if u.texture == nil || u.width != width || u.height != height {
		tex, err := NewGLTexture(width, height)
		if err != nil {
			return err
		}
		if u.texture != nil {
			u.texture.Delete()
		}
		u.texture = tex
		u.width = width
		u.height = height
	}

	return u.texture.Upload(img)
}

func (u *GPUTextureUpload) Texture() *GLTexture {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.texture
}

func (u *GPUTextureUpload) Width() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.width
}

func (u *GPUTextureUpload) Height() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.height
}

func (u *GPUTextureUpload) Delete() {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.texture != nil {
		u.texture.Delete()
		u.texture = nil
	}
}

func (u *GPUTextureUpload) Acquire() (*GLTexture, error) {
	select {
	case tex := <-u.pool:
		return tex, nil
	default:
		tex, err := NewGLTexture(u.width, u.height)
		if err != nil {
			return nil, err
		}
		return tex, nil
	}
}

func (u *GPUTextureUpload) Release(tex *GLTexture) {
	if tex == nil {
		return
	}
	select {
	case u.pool <- tex:
	default:
		tex.Delete()
	}
}
