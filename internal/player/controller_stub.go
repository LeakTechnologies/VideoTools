//go:build !linux

package player

import "fmt"

type stubController struct{}

func newController() Controller {
	return &stubController{}
}

func (s *stubController) Load(string, float64) error   { return fmt.Errorf("player unavailable") }
func (s *stubController) SetWindow(int, int, int, int) {}
func (s *stubController) Play() error                  { return fmt.Errorf("player unavailable") }
func (s *stubController) Pause() error                 { return fmt.Errorf("player unavailable") }
func (s *stubController) Seek(float64) error           { return fmt.Errorf("player unavailable") }
func (s *stubController) SetVolume(float64) error      { return fmt.Errorf("player unavailable") }
func (s *stubController) FullScreen() error            { return fmt.Errorf("player unavailable") }
func (s *stubController) Stop() error                  { return fmt.Errorf("player unavailable") }
func (s *stubController) Close()                       {}
