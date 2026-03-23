//go:build native_media && windows

package media

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
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
	nw, ok := win.(driver.NativeWindow)
	if !ok {
		return 0
	}
	var hwnd windows.Handle
	nw.RunNative(func(ctx any) {
		if wctx, ok := ctx.(driver.WindowsWindowContext); ok {
			hwnd = windows.Handle(wctx.HWND)
		}
	})
	return hwnd
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
