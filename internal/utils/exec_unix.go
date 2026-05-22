//go:build !windows && !linux

package utils

import (
	"context"
	"os/exec"
)

// CreateCommand is a platform-specific implementation for non-Windows, non-Linux
// systems (e.g. macOS). No special SysProcAttr is needed here.
func CreateCommand(ctx context.Context, name string, arg ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, arg...)
}

// CreateCommandRaw is a platform-specific implementation for non-Windows, non-Linux
// systems without a context.
func CreateCommandRaw(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}
