//go:build !linux

package utils

// EnsureLinuxDesktopEntry is a no-op on non-Linux platforms.
func EnsureLinuxDesktopEntry(appID, appName string) {}
