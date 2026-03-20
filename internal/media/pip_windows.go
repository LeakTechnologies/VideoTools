//go:build native_media && windows

package media

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"golang.org/x/sys/windows"
)

const (
	WDA_EXCLUDEFROMCAPTURE = 0x00000011
)

func EnablePiPExclude(hwnd windows.Handle) error {
	ret, _, _ := windows.NewLazyDLL("user32.dll").NewProc("SetWindowDisplayAffinity").Call(
		uintptr(hwnd), uintptr(WDA_EXCLUDEFROMCAPTURE))
	if ret != 0 {
		return nil
	}
	return windows.GetLastError()
}

func DisablePiPExclude(hwnd windows.Handle) error {
	ret, _, _ := windows.NewLazyDLL("user32.dll").NewProc("SetWindowDisplayAffinity").Call(
		uintptr(hwnd), uintptr(0))
	if ret != 0 {
		return nil
	}
	return windows.GetLastError()
}

func GetWindowHandleFromFyne(win fyne.Window) windows.Handle {
	if deskWin, ok := win.(desktop.Window); ok {
		if glfwWin, ok := deskWin.(interface {
			GetWin32Window() windows.Handle
		}); ok {
			return glfwWin.GetWin32Window()
		}
	}
	return 0
}

func ApplyPiPExclude(win fyne.Window, enable bool) error {
	hwnd := GetWindowHandleFromFyne(win)
	if hwnd == 0 {
		return nil
	}
	if enable {
		return EnablePiPExclude(hwnd)
	}
	return DisablePiPExclude(hwnd)
}
