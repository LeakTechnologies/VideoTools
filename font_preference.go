//go:build native_media

package main

import (
	"github.com/LeakTechnologies/VideoTools/internal/ui"
)

func applyVCRFontPreference(font string) {
	ui.SetMonoFontPreference(font)
}