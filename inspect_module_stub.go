//go:build !native_media

package main

import (
	"fyne.io/fyne/v2/dialog"
)

func (s *appState) showInspectView() {
	dialog.ShowInformation("Module Unavailable", "The Inspect module is not available in this build.\n\nPlease download the latest release from leaktechnologies.dev.", s.window)
}

func (s *appState) showInspectViewForPath(path string) {
	// Inspect module requires native_media — fall back to the player module.
	s.showPlayerViewForPath(path)
}
