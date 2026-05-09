//go:build !windows

package media

import "time"

const _vsyncPeriod = time.Second / 60

// WaitVsync approximates vsync by sleeping to the next 60 Hz boundary.
// On Linux/macOS there is no guaranteed vsync notification from userspace
// without linking against a display library; a 60 Hz-aligned sleep is the
// portable substitute and still reduces subframe presentation jitter compared
// to an unaligned sleep.
func WaitVsync() {
	now := time.Now()
	next := now.Truncate(_vsyncPeriod).Add(_vsyncPeriod)
	d := next.Sub(now)
	if d < time.Millisecond {
		d += _vsyncPeriod
	}
	time.Sleep(d)
}
