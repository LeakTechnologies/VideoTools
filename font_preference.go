//go:build native_media

package main

import (
	"git.leaktechnologies.dev/stu/VideoTools/internal/media"
)

func applyVCRFontPreference(useVCR bool) {
	media.SetUseVCRFontPreference(useVCR)
}