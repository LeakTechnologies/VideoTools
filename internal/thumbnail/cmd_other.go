//go:build !windows

package thumbnail

import "os/exec"

func hideCmd(cmd *exec.Cmd) {}
