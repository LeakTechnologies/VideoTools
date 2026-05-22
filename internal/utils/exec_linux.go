//go:build linux

package utils

import (
	"context"
	"os/exec"
	"syscall"
)

// CreateCommand is the Linux implementation. Pdeathsig causes the kernel to
// send SIGKILL to this child process the moment the parent (VT) process exits
// for any reason, including crashes. This prevents zombie FFmpeg processes from
// persisting after VT closes mid-conversion.
func CreateCommand(ctx context.Context, name string, arg ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}
	return cmd
}

// CreateCommandRaw is the Linux implementation without a context.
func CreateCommandRaw(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}
	return cmd
}
