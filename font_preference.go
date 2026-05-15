//go:build native_media

package main

import (
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/ui"
)

func applyVCRFontPreference(font string) {
	ui.SetMonoFontPreference(font)
}