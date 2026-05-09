//go:build windows

package media

import "syscall"

var (
	_dwmapi   = syscall.NewLazyDLL("dwmapi.dll")
	_dwmFlush = _dwmapi.NewProc("DwmFlush")
)

// WaitVsync blocks until the next display composition cycle (vsync).
// On Windows, DwmFlush() synchronises to the Desktop Window Manager's
// refresh rate — the same boundary at which the display actually updates.
// Calling this immediately before updating the frame pointer ensures the
// frame swap lands at a vsync edge rather than mid-frame, eliminating the
// class of judder that comes from presenting at arbitrary subframe offsets.
func WaitVsync() {
	_dwmFlush.Call()
}
