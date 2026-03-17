//go:build !native_media

package trim

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// Options mirrors the native Options struct so main.go compiles without
// the native_media build tag.
type Options struct {
	Window         fyne.Window
	OnShowMainMenu func()
}

// BuildView returns a placeholder when native media support is not compiled in.
func BuildView(opts Options) fyne.CanvasObject {
	return container.NewCenter(
		widget.NewLabel("Trim module requires native media support (build tag: native_media)."),
	)
}
