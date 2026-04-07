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
	// Retry the copy until the old process releases the file lock (up to 30 s).
	// A fixed sleep is not reliable — CGo/FFmpeg teardown can exceed 2 s, causing
	// Copy-Item to fail silently and Start-Process to re-launch the old binary.
	script := fmt.Sprintf(
		`$src='%s'; $dst='%s'; `+
			`$deadline=[DateTime]::Now.AddSeconds(30); `+
			`$ok=$false; `+
			`do { `+
			`  Start-Sleep -Milliseconds 500; `+
			`  try { Copy-Item -Force -Path $src -Destination $dst; $ok=$true; break } catch {} `+
			`} while ([DateTime]::Now -lt $deadline); `+
			`if ($ok) { Start-Process -FilePath '%s' }; `+
			`Remove-Item -Force -Path $src -ErrorAction SilentlyContinue`,
		newBinaryPath, currentExePath, currentExePath,
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
