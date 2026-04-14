//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// performRestart replaces currentExePath with newBinaryPath then re-executes the process.
// On Linux we use a helper script to handle the replace asynchronously.
func performRestart(newBinaryPath, currentExePath string) error {
	if err := os.Chmod(newBinaryPath, 0755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	absCurrent, err := filepath.Abs(currentExePath)
	if err != nil {
		absCurrent = currentExePath
	}
	absNew, err := filepath.Abs(newBinaryPath)
	if err != nil {
		absNew = newBinaryPath
	}

	script := fmt.Sprintf(`#!/bin/bash
src='%s'
dst='%s'
ok=false
for i in $(seq 1 60); do
	sleep 0.5
	if cp -f "$src" "$dst" 2>/dev/null; then
		ok=true
		break
	fi
done
rm -f "$src"
if $ok; then
	exec "$dst"
fi
`, absNew, absCurrent)

	scriptPath := absNew + ".update_helper"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return fmt.Errorf("write helper: %w", err)
	}

	cmd := exec.Command(scriptPath)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch updater: %w", err)
	}
	os.Exit(0)
	return nil
}
