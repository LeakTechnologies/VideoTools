//go:build !windows

package utils

import (
	"context"
	"os/exec"
)

// CreateCommand is a platform-specific implementation for Unix-like systems (Linux, macOS).
// On these systems, external commands generally do not spawn new visible console windows
// unless explicitly configured to do so by the user's terminal environment.
// No special SysProcAttr is typically needed for console hiding on Unix.
func CreateCommand(ctx context.Context, name string, arg ...string) *exec.Cmd {
	// For Unix-like systems, exec.CommandContext typically does not create a new console window.
	// We just return the standard command.
	return exec.CommandContext(ctx, name, arg...)
}

// CreateCommandRaw is a platform-specific implementation for Unix-like systems, without a context.
// No special SysProcAttr is typically needed for console hiding on Unix.
func CreateCommandRaw(name string, arg ...string) *exec.Cmd {
	// For Unix-like systems, exec.Command typically does not create a new console window.
	// We just return the standard command.
	return exec.Command(name, arg...)
}