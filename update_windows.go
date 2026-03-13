//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// performRestart schedules newBinaryPath to replace currentExePath after this process exits,
// then restarts the application. On Windows this is done via a hidden PowerShell one-liner.
func performRestart(newBinaryPath, currentExePath string) error {
	script := fmt.Sprintf(
		`Start-Sleep -Seconds 2; `+
			`Copy-Item -Force -Path '%s' -Destination '%s'; `+
			`Start-Process -FilePath '%s'; `+
			`Remove-Item -Force -Path '%s'`,
		newBinaryPath, currentExePath, currentExePath, newBinaryPath,
	)
	cmd := exec.Command("powershell",
		"-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden",
		"-Command", script,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch updater: %w", err)
	}
	os.Exit(0)
	return nil
}
