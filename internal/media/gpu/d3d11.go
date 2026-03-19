//go:build native_media && windows

package gpu

import (
	"fmt"
	"image"
	"image/color"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type D3D11Renderer struct {
	device         unsafe.Pointer
	context        unsafe.Pointer
	swapChain      unsafe.Pointer
	videoProcessor unsafe.Pointer
	available      bool
	adapterDesc    string
}

func NewD3D11Renderer() *D3D11Renderer {
	r := &D3D11Renderer{
		available: false,
	}
	r.detect()
	return r
}

func (r *D3D11Renderer) detect() {
	r.available = false
	r.adapterDesc = "Not detected"
}

func (r *D3D11Renderer) IsAvailable() bool {
	return r.available
}

func (r *D3D11Renderer) Name() string {
	return "Direct3D 11"
}

func (r *D3D11Renderer) MakeCurrent() error {
	if !r.available {
		return fmt.Errorf("D3D11 not available")
	}
	return nil
}

func (r *D3D11Renderer) SwapBuffers() error {
	if !r.available {
		return fmt.Errorf("D3D11 not available")
	}
	return nil
}

func (r *D3D11Renderer) Delete() {
	r.device = nil
	r.context = nil
	r.swapChain = nil
}

type D3D11Texture struct {
	resource   unsafe.Pointer
	shaderView unsafe.Pointer
	width      int
	height     int
	usage      uint32
	format     uint32
}

func NewD3D11Texture(resource, shaderView unsafe.Pointer, width, height int) *D3D11Texture {
	return &D3D11Texture{
		resource:   resource,
		shaderView: shaderView,
		width:      width,
		height:     height,
		format:     87,
	}
}

func (t *D3D11Texture) Upload(img *image.RGBA) error {
	if img == nil {
		return fmt.Errorf("nil image")
	}
	return nil
}

func (t *D3D11Texture) UploadBGRA(data []byte, width, height int) error {
	if len(data) == 0 {
		return fmt.Errorf("empty data")
	}
	return nil
}

func (t *D3D11Texture) Width() int {
	return t.width
}

func (t *D3D11Texture) Height() int {
	return t.height
}

func (t *D3D11Texture) Delete() {
	if t.shaderView != nil {
	}
	if t.resource != nil {
	}
}

type VideoRendererD3D11 struct {
	*VideoRenderer
	d3d *D3D11Renderer
}

func NewVideoRendererD3D11() *VideoRendererD3D11 {
	v := &VideoRendererD3D11{
		VideoRenderer: NewVideoRenderer(),
		d3d:           NewD3D11Renderer(),
	}
	return v
}

func (v *VideoRendererD3D11) CreateRenderer() fyne.WidgetRenderer {
	return &videoRendererD3D11Renderer{VideoRendererD3D11: v}
}

func (v *VideoRendererD3D11) IsAvailable() bool {
	return v.d3d.IsAvailable()
}

func (v *VideoRendererD3D11) Name() string {
	return v.d3d.Name()
}

func (v *VideoRendererD3D11) MakeCurrent() error {
	return v.d3d.MakeCurrent()
}

func (v *VideoRendererD3D11) SwapBuffers() error {
	return v.d3d.SwapBuffers()
}

type videoRendererD3D11Renderer struct {
	*VideoRendererD3D11
}

func (r *videoRendererD3D11Renderer) Objects() []fyne.CanvasObject {
	return nil
}

func (r *videoRendererD3D11Renderer) Layout(fyne.Size) {
}

func (r *videoRendererD3D11Renderer) MinSize() fyne.Size {
	return fyne.NewSize(320, 180)
}

func (r *videoRendererD3D11Renderer) Refresh() {
}

func (r *videoRendererD3D11Renderer) Destroy() {
}

type VideoPlayerD3D11 struct {
	*VideoPlayerGPU
	d3d *D3D11Renderer
}

func NewVideoPlayerD3D11() *VideoPlayerD3D11 {
	v := &VideoPlayerD3D11{
		VideoPlayerGPU: NewVideoPlayerGPU(),
		d3d:            NewD3D11Renderer(),
	}
	return v
}

func (v *VideoPlayerD3D11) CreateRenderer() fyne.WidgetRenderer {
	return &videoPlayerD3D11Renderer{VideoPlayerD3D11: v}
}

func (v *VideoPlayerD3D11) IsAvailable() bool {
	return v.d3d.IsAvailable()
}

type videoPlayerD3D11Renderer struct {
	*VideoPlayerD3D11
}

func (r *videoPlayerD3D11Renderer) Objects() []fyne.CanvasObject {
	return r.VideoPlayerGPU.CreateRenderer().Objects()
}

func (r *videoPlayerD3D11Renderer) Layout(size fyne.Size) {
}

func (r *videoPlayerD3D11Renderer) MinSize() fyne.Size {
	return fyne.NewSize(320, 180)
}

func (r *videoPlayerD3D11Renderer) Refresh() {
}

func (r *videoPlayerD3D11Renderer) Destroy() {
}

var _ unsafe.Pointer = unsafe.Pointer(nil)
