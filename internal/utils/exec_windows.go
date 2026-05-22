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
func CreateCommand(ctx context.Context, name string, arg ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, arg...)
	// SysProcAttr is used to control process creation parameters on Windows.
	// HideWindow: If true, the new process's console window will be hidden.
	// CreationFlags: CREATE_NO_WINDOW (0x08000000) prevents the creation of a console window.
	// This is crucial for a smooth GUI experience when launching CLI tools.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:       true,
		CreationFlags:    0x08000000, // CREATE_NO_WINDOW
		NoInheritHandles: true,
	}
	return cmd
}

// CreateCommandRaw is a platform-specific implementation for Windows, without a context.
// It applies the same console hiding behavior as CreateCommand.
func CreateCommandRaw(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:       true,
		CreationFlags:    0x08000000, // CREATE_NO_WINDOW
		NoInheritHandles: true,
	}
	return cmd
}