//go:build windows

package utils

import (
	"context"
	"os/exec"
	"syscall"
)

// CreateCommand is a platform-specific implementation for Windows.
// It ensures that the command is created without a new console window,
// preventing disruptive pop-ups when running console applications (like ffmpeg)
// from a GUI application.
//
// NOTE: NoInheritHandles must NOT be set here. Go's os/exec passes the child's
// stdout/stderr pipes via the standard-handle inheritance list; syscall's
// NoInheritHandles disables that ("no handles are inherited by the new
// process, not even the standard handles"), so cmd.Output()/CombinedOutput()/
// StdoutPipe() return nothing — which silently broke every ffprobe metadata
// read and ffmpeg -progress pipe on Windows. Modern Go (1.16+) already passes
// ONLY the std handles via PROC_THREAD_ATTRIBUTE_HANDLE_LIST, so arbitrary
// parent handles are not leaked to children; crash-safe child cleanup is
// handled separately by the Job Object (see jobobject_windows.go).
func CreateCommand(ctx context.Context, name string, arg ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	return cmd
}

// CreateCommandRaw is a platform-specific implementation for Windows, without a context.
// It applies the same console hiding behavior as CreateCommand. See CreateCommand
// for why NoInheritHandles is intentionally not set.
func CreateCommandRaw(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	return cmd
}
