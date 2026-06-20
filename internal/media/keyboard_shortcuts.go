//go:build native_media

package media

import (
	"fyne.io/fyne/v2"
	"github.com/LeakTechnologies/VideoTools/internal/logging"
)

func (v *VideoPlayer) Tapped(ev *fyne.PointEvent) {
	if v.builtinControlsLocked {
		return
	}
	canvas := fyne.CurrentApp().Driver().CanvasForObject(v)
	if canvas != nil {
		canvas.Focus(v)
	}
	if v.hasError && v.errorMessage != "" {
		logging.Info(logging.CatPlayer, "VideoPlayer error: %s", v.errorMessage)
		return
	}
	if v.source.Load() == nil {
		if v.onTapEmpty != nil {
			v.onTapEmpty()
		}
		return
	}
	v.togglePlay()
}

func (v *VideoPlayer) TypedKey(event *fyne.KeyEvent) {
	if v.builtinControlsLocked {
		return
	}
	if v.source.Load() == nil {
		return
	}
	switch event.Name {
	case fyne.KeySpace:
		v.togglePlay()
	}
}

func (v *VideoPlayer) FocusGained() {}
func (v *VideoPlayer) FocusLost()  {}
func (v *VideoPlayer) TypedRune(r rune) {}

func (v *VideoPlayer) SetOnTapEmpty(fn func()) {
	v.onTapEmpty = fn
}