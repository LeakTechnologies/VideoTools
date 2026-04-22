//go:build native_media

package main

import (
	"git.leaktechnologies.dev/stu/VideoTools/internal/media"
)

func applyVCRFontPreference(font string) {
	media.SetPlayerFontPreference(font)
}