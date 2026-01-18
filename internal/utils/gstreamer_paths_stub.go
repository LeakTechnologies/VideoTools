//go:build !windows

package utils

// EnsureGStreamerOnPath is a no-op on non-Windows platforms.
func EnsureGStreamerOnPath() {}
