//go:build !windows

package utils

import (
	"context"
	"os/exec"
)

// HideWindowExec runs a command - no-op on non-Windows
func HideWindowExec(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

// HideWindowExecContext runs a command with context - no-op on non-Windows
func HideWindowExecContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}
