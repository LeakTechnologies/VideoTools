//go:build !windows

package utils

import "os/exec"

// ApplyNoWindow is a no-op on non-Windows platforms.
func ApplyNoWindow(cmd *exec.Cmd) {
	_ = cmd
}
