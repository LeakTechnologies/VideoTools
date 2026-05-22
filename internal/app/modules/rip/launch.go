package rip

import (
	"path/filepath"
	"strings"
)

// resolveDVDRoot returns the DVD root path suitable for dvdPlayer.LoadDVD.
// ISO files are passed through as-is; VIDEO_TS directories are normalised to
// their parent (the disc root); anything else is returned unchanged.
func resolveDVDRoot(sourcePath string) string {
	if strings.EqualFold(filepath.Ext(sourcePath), ".iso") {
		return sourcePath
	}
	if strings.EqualFold(filepath.Base(sourcePath), "VIDEO_TS") {
		return filepath.Dir(sourcePath)
	}
	return sourcePath
}
