//go:build native_media && windows

package media

import "syscall"

var (
	winmm              = syscall.NewLazyDLL("winmm.dll")
	procTimeBeginPeriod = winmm.NewProc("timeBeginPeriod")
)

func init() {
	// Request 1ms timer resolution so time.Sleep is precise enough for
	// frame pacing. The Windows default (15.6ms) causes up to half-frame
	// jitter at 30fps, making playback visibly rough.
	procTimeBeginPeriod.Call(1)
}
