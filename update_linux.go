//go:build linux

package main

import (
	"fmt"
	"io"
	"os"
	"syscall"
)

// performRestart replaces currentExePath with newBinaryPath then re-executes the process.
func performRestart(newBinaryPath, currentExePath string) error {
	if err := os.Chmod(newBinaryPath, 0755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}
	if err := os.Rename(newBinaryPath, currentExePath); err != nil {
		// Cross-device rename (e.g. /tmp → /home); fall back to copy.
		if cpErr := copyUpdateFile(newBinaryPath, currentExePath); cpErr != nil {
			return fmt.Errorf("replace binary: %w", err)
		}
		os.Remove(newBinaryPath)
	}
	return syscall.Exec(currentExePath, os.Args, os.Environ())
}

func copyUpdateFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
