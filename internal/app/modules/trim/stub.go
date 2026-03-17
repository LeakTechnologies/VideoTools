//go:build !native_media

package trim

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// Options mirrors the native Options struct so main.go compiles without
// the native_media build tag.
type Options struct {
	Window         fyne.Window
	ModuleColor    color.Color
	OnShowMainMenu func()
	OnShowQueue    func()
	OnAddToQueue   func(clip TrimClip)
}

// TrimClip mirrors the native struct for stub compatibility.
type TrimClip struct {
	Path     string
	InPoint  time.Duration
	OutPoint time.Duration
	Mode     string
	Export   string
}

// BuildView returns a placeholder when native media support is not compiled in.
func BuildView(opts Options, initialPath string) fyne.CanvasObject {
	return container.NewCenter(
		widget.NewLabel("Trim module requires native media support (build tag: native_media)."),
	)
}
