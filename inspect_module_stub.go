//go:build !native_media

package main

func (s *appState) showInspectView() {}
func (s *appState) showInspectViewForPath(path string) {
	// Inspect module requires native_media — fall back to the player module.
	s.showPlayerViewForPath(path)
}
