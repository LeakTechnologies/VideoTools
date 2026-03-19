//go:build native_media

package gpu

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

type ShortcutHandler struct {
	player       PlayerControl
	onFullscreen func()
	onPiP        func()
}

type PlayerControl interface {
	HandleKey(key string) bool
	SetPlaying(playing bool)
	SetVolume(vol float64)
	IsPlaying() bool
	Volume() float64
	CurrentTime() float64
	Duration() float64
	Seek(target float64)
	ToggleMute()
}

func NewShortcutHandler(player PlayerControl) *ShortcutHandler {
	return &ShortcutHandler{
		player: player,
	}
}

func (h *ShortcutHandler) OnFullscreen(cb func()) {
	h.onFullscreen = cb
}

func (h *ShortcutHandler) OnPiP(cb func()) {
	h.onPiP = cb
}

func (h *ShortcutHandler) HandleShortcut(shortcut fyne.Shortcut) bool {
	switch s := shortcut.(type) {
	case *desktop.ShortcutKey:
		return h.handleKeyEvent(s.KeyName, s.Modifier)
	}
	return false
}

func (h *ShortcutHandler) handleKeyEvent(keyName string, modifier desktop.KeyModifier) bool {
	if h.player == nil {
		return false
	}

	switch keyName {
	case fyne.KeySpace:
		h.player.SetPlaying(!h.player.IsPlaying())
		return true

	case fyne.KeyLeft:
		if modifier&desktop.ShiftModifier != 0 {
			h.seekFrame(-1)
		} else {
			h.seekSeconds(-5)
		}
		return true

	case fyne.KeyRight:
		if modifier&desktop.ShiftModifier != 0 {
			h.seekFrame(1)
		} else {
			h.seekSeconds(5)
		}
		return true

	case fyne.KeyUp:
		h.adjustVolume(0.1)
		return true

	case fyne.KeyDown:
		h.adjustVolume(-0.1)
		return true

	case fyne.KeyM:
		h.player.ToggleMute()
		return true

	case fyne.KeyF:
		if h.onFullscreen != nil {
			h.onFullscreen()
		}
		return true

	case fyne.KeyP:
		if h.onPiP != nil {
			h.onPiP()
		}
		return true

	case fyne.KeyHome:
		h.player.Seek(0)
		return true

	case fyne.KeyEnd:
		h.player.Seek(h.player.Duration())
		return true

	case fyne.KeyLess:
		h.adjustSpeed(-0.25)
		return true

	case fyne.KeyGreater:
		h.adjustSpeed(0.25)
		return true

	case fyne.Key0, fyne.Key1, fyne.Key2, fyne.Key3, fyne.Key4,
		fyne.Key5, fyne.Key6, fyne.Key7, fyne.Key8, fyne.Key9:
		h.seekToPercent(keyName)
		return true

	case fyne.KeyN:
		h.nextFrame()
		return true

	case fyne.KeyB:
		h.prevFrame()
		return true
	}

	return false
}

func (h *ShortcutHandler) seekSeconds(delta float64) {
	target := h.player.CurrentTime() + delta
	duration := h.player.Duration()

	if target < 0 {
		target = 0
	} else if target > duration {
		target = duration
	}

	h.player.Seek(target)
}

func (h *ShortcutHandler) seekFrame(delta int) {
	fps := 30.0
	frameTime := 1.0 / fps
	target := h.player.CurrentTime() + (float64(delta) * frameTime)
	duration := h.player.Duration()

	if target < 0 {
		target = 0
	} else if target > duration {
		target = duration
	}

	h.player.Seek(target)
}

func (h *ShortcutHandler) adjustVolume(delta float64) {
	newVol := h.player.Volume() + delta
	if newVol < 0 {
		newVol = 0
	} else if newVol > 1 {
		newVol = 1
	}
	h.player.SetVolume(newVol)
}

func (h *ShortcutHandler) adjustSpeed(delta float64) {
}

func (h *ShortcutHandler) seekToPercent(keyName string) {
	var percent float64
	switch keyName {
	case fyne.Key0:
		percent = 0
	case fyne.Key1:
		percent = 0.1
	case fyne.Key2:
		percent = 0.2
	case fyne.Key3:
		percent = 0.3
	case fyne.Key4:
		percent = 0.4
	case fyne.Key5:
		percent = 0.5
	case fyne.Key6:
		percent = 0.6
	case fyne.Key7:
		percent = 0.7
	case fyne.Key8:
		percent = 0.8
	case fyne.Key9:
		percent = 0.9
	}

	target := h.player.Duration() * percent
	h.player.Seek(target)
}

func (h *ShortcutHandler) nextFrame() {
	h.seekFrame(1)
}

func (h *ShortcutHandler) prevFrame() {
	h.seekFrame(-1)
}

type PlaybackSpeed int

const (
	Speed025 PlaybackSpeed = iota
	Speed050
	Speed075
	Speed100
	Speed125
	Speed150
	Speed175
	Speed200
)

func (s PlaybackSpeed) Value() float64 {
	switch s {
	case Speed025:
		return 0.25
	case Speed050:
		return 0.5
	case Speed075:
		return 0.75
	case Speed100:
		return 1.0
	case Speed125:
		return 1.25
	case Speed150:
		return 1.5
	case Speed175:
		return 1.75
	case Speed200:
		return 2.0
	default:
		return 1.0
	}
}

func (s PlaybackSpeed) Label() string {
	switch s {
	case Speed025:
		return "0.25x"
	case Speed050:
		return "0.5x"
	case Speed075:
		return "0.75x"
	case Speed100:
		return "1x"
	case Speed125:
		return "1.25x"
	case Speed150:
		return "1.5x"
	case Speed175:
		return "1.75x"
	case Speed200:
		return "2x"
	default:
		return "1x"
	}
}

func SpeedFromValue(v float64) PlaybackSpeed {
	speeds := []PlaybackSpeed{Speed025, Speed050, Speed075, Speed100, Speed125, Speed150, Speed175, Speed200}
	for _, s := range speeds {
		if s.Value() == v {
			return s
		}
	}
	return Speed100
}
