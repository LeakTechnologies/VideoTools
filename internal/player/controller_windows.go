//go:build windows

package player

import "fmt"

func newController() Controller {
	return &stubController{}
}

type stubController struct{}

func (s *stubController) Load(path string, offset float64) error {
	return fmt.Errorf("player unavailable")
}
func (s *stubController) SetWindow(x, y, w, h int)      {}
func (s *stubController) Play() error                   { return fmt.Errorf("player unavailable") }
func (s *stubController) Pause() error                  { return fmt.Errorf("player unavailable") }
func (s *stubController) Seek(offset float64) error     { return fmt.Errorf("player unavailable") }
func (s *stubController) SetVolume(level float64) error { return fmt.Errorf("player unavailable") }
func (s *stubController) FullScreen() error             { return fmt.Errorf("player unavailable") }
func (s *stubController) Stop() error                   { return fmt.Errorf("player unavailable") }
func (s *stubController) Close()                        {}
