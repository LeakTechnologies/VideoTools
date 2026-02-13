//go:build windows

package utils

import (
	"os/exec"
	"syscall"
)

// ApplyNoWindow hides the console window for spawned processes on Windows.
func ApplyNoWindow(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW - prevents console window
	}
}
