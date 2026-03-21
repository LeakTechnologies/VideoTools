//go:build !native_media

package main

import (
	"fyne.io/fyne/v2/dialog"

	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
)

func (s *appState) showInspectView() {
	dialog.ShowInformation("Module Unavailable", "The Inspect module requires the native_media build.\n\nRebuild VideoTools with:\n  go build -tags native_media .", s.window)
}

func (s *appState) showInspectViewForPath(path string) {
	// Inspect module requires native_media — fall back to the player module.
	s.showPlayerViewForPath(path)
}
