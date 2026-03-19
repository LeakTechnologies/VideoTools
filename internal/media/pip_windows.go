//go:build native_media && windows

package media

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	WDA_EXCLUDEFROMCAPTURE = 0x00000011
)

func EnablePiPExclude(hwnd windows.Handle) error {
	return windows.SetWindowDisplayAffinity(hwnd, WDA_EXCLUDEFROMCAPTURE)
}

func DisablePiPExclude(hwnd windows.Handle) error {
	return windows.SetWindowDisplayAffinity(hwnd, 0)
}

func GetWindowHandle(window interface{}) windows.Handle {
	return 0
}
