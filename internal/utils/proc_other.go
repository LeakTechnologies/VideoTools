//go:build !windows

package utils

import "os/exec"

// ApplyNoWindow is a no-op on non-Windows platforms.
// The cmd parameter is unused on these platforms but kept for interface compatibility.
func ApplyNoWindow(cmd *exec.Cmd) {
	_ = cmd // No CREATE_NO_WINDOW flag needed on non-Windows
}
