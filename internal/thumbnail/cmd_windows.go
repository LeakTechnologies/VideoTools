//go:build windows

package thumbnail

import (
	"os/exec"
	"syscall"
)

func hideCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}
