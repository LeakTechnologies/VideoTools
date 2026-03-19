//go:build native_media

package media

import (
	"fyne.io/fyne/v2"
)

type PiPController struct {
	enabled  bool
	position PiPPosition
	window   fyne.Window
}

type PiPPosition int

const (
	PiPPositionTopLeft PiPPosition = iota
	PiPPositionTopRight
	PiPPositionBottomLeft
	PiPPositionBottomRight
)

func NewPiPController(window fyne.Window) *PiPController {
	return &PiPController{
		enabled:  false,
		position: PiPPositionBottomRight,
		window:   window,
	}
}

func (p *PiPController) Enable() error {
	p.enabled = true
	return nil
}

func (p *PiPController) Disable() error {
	p.enabled = false
	return nil
}

func (p *PiPController) IsEnabled() bool {
	return p.enabled
}

func (p *PiPController) SetPosition(pos PiPPosition) {
	p.position = pos
}

func (p *PiPController) GetPosition() PiPPosition {
	return p.position
}

func (p *PiPController) CyclePosition() {
	switch p.position {
	case PiPPositionBottomRight:
		p.position = PiPPositionBottomLeft
	case PiPPositionBottomLeft:
		p.position = PiPPositionTopLeft
	case PiPPositionTopLeft:
		p.position = PiPPositionTopRight
	case PiPPositionTopRight:
		p.position = PiPPositionBottomRight
	}
}

func (p *PiPController) GetWindow() fyne.Window {
	return p.window
}
