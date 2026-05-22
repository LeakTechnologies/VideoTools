//go:build !windows

package utils

import "os/exec"

// InitJobObject is a no-op on non-Windows platforms.
// Linux child processes are already tied to VT via Pdeathsig.
func InitJobObject() {}

// initJobObject is a no-op on non-Windows platforms.
func initJobObject() {}

// StartCmd starts cmd. On Linux/macOS, Pdeathsig (set in CreateCommand's
// SysProcAttr) ties child processes to the parent; no extra tracking needed.
func StartCmd(cmd *exec.Cmd) error {
	return cmd.Start()
}
