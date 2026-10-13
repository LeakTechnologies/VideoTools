//go:build native_media

package media

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"image"
	"testing"
)

func TestPlayerContextKeysWorkWithCustomControls(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	v := NewVideoPlayer()
	v.DisableBuiltinControls()
	v.source.Store(image.NewRGBA(image.Rect(0, 0, 2, 2)))
	plays := 0
	v.onPlay = func() { plays++ }
	v.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	if plays != 0 {
		t.Fatal("other module custom controls enabled keys")
	}
	v.SetTransportKeysEnabled(true)
	v.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	if plays != 1 {
		t.Fatal("player Space ignored")
	}
	v.currentTime = 20
	v.duration = 100
	target := -1.0
	v.onSeek = func(s float64) { target = s }
	v.TypedKey(&fyne.KeyEvent{Name: fyne.KeyRight})
	if target != 25 {
		t.Fatalf("right seek=%v", target)
	}
	v.currentTime = 2
	v.TypedKey(&fyne.KeyEvent{Name: fyne.KeyLeft})
	if target != 0 {
		t.Fatalf("left seek=%v", target)
	}
}
func TestSeekConstantsMatchTimestampUnits(t *testing.T) {
	if AVSEEK_FLAG_BACKWARD != 1 || AVSEEK_FLAG_FRAME != 8 || AVSEEK_FLAG_ANY != 4 {
		t.Fatal("seek constants disagree with FFmpeg")
	}
}
