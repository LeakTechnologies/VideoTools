//go:build windows

package utils

import (
	"os"
	"path/filepath"
	"strings"
)

// EnsureGStreamerOnPath best-effort prepends common GStreamer bin locations to PATH for the
// current process. It does not modify system-wide PATH. This helps avoid missing-DLL issues
// when users install GStreamer but its bin directory is not on PATH.
func EnsureGStreamerOnPath() {
	candidates := []string{}

	// If the user set GSTREAMER_1_0_ROOT_X86_64, prefer its bin
	if root := os.Getenv("GSTREAMER_1_0_ROOT_X86_64"); root != "" {
		candidates = append(candidates, filepath.Join(root, "bin"))
	}

	// Common installer locations (64-bit)
	candidates = append(candidates,
		`C:\\gstreamer\\1.0\\msvc_x86_64\\bin`,
		`C:\\Program Files\\GStreamer\\1.0\\msvc_x86_64\\bin`,
	)

	pathVal := os.Getenv("PATH")
	parts := []string{}
	seen := map[string]bool{}

	// Prepend any found candidates that exist
	for _, cand := range candidates {
		if cand == "" {
			continue
		}
		if _, err := os.Stat(cand); err == nil {
			// Only add if not already present (case-insensitive on Windows)
			lower := strings.ToLower(cand)
			if !seen[lower] {
				parts = append(parts, cand)
				seen[lower] = true
			}
		}
	}

	// Append existing PATH entries, preserving order and deduping case-insensitively
	for _, p := range filepath.SplitList(pathVal) {
		lower := strings.ToLower(p)
		if seen[lower] {
			continue
		}
		parts = append(parts, p)
		seen[lower] = true
	}

	_ = os.Setenv("PATH", strings.Join(parts, string(os.PathListSeparator)))
}
