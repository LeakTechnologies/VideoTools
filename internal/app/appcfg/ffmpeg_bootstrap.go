//go:build native_media

package appcfg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
)

// FFmpegDllDir returns the directory where FFmpeg DLLs are expected.
// Lookup order:
//  1. <exe-dir>/DLL/   (CI/release bundled subfolder)
//  2. <exe-dir>/               (flat DLLs next to exe — local dev builds,
//                                flattened user extraction)
//  3. %LOCALAPPDATA%\VideoTools\DLL (legacy download path)
func FFmpegDllDir() string {
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)

		bundledDir := filepath.Join(exeDir, "DLL")
		if dllsPresent(bundledDir) {
			return bundledDir
		}

		if dllsPresent(exeDir) {
			return exeDir
		}
	}

	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		home, _ := os.UserHomeDir()
		if home != "" {
			base = filepath.Join(home, "AppData", "Local")
		}
	}
	return filepath.Join(base, "VideoTools", "DLL")
}

func dllsPresent(dir string) bool {
	matches, err := filepath.Glob(filepath.Join(dir, "avcodec*.dll"))
	return err == nil && len(matches) > 0
}

// FFmpegDllsPresent returns true if at least one avcodec*.dll exists in the
// resolved DLL directory.
func FFmpegDllsPresent() bool {
	dllDir := FFmpegDllDir()
	matches, err := filepath.Glob(filepath.Join(dllDir, "avcodec*.dll"))
	return err == nil && len(matches) > 0
}

// AddFFmpegDllsToPath finds the FFmpeg DLL directory and prepends it to PATH
// so the Windows loader (and FFmpeg's internal LoadLibrary) can find them.
// Returns nil on success, or an error describing what went wrong.
func AddFFmpegDllsToPath() error {
	dllDir := FFmpegDllDir()

	if !FFmpegDllsPresent() {
		err := fmt.Errorf("FFmpeg DLLs not found in %s (looked in: exe-dir/DLL/, exe-dir/, %%LOCALAPPDATA%%/VideoTools/DLL)", dllDir)
		logging.Error(logging.CatSystem, "%v", err)
		return err
	}

	currentPath := os.Getenv("PATH")
	newPath := dllDir + string(os.PathListSeparator) + currentPath
	if err := os.Setenv("PATH", newPath); err != nil {
		return fmt.Errorf("failed to set PATH for FFmpeg DLLs: %w", err)
	}
	logging.Debug(logging.CatSystem, "prepended DLL dir to PATH: %s", dllDir)
	return nil
}

// ExpectedFFmpegDLLs returns the set of DLL basenames that the FFmpeg shared
// build is expected to provide.  The list covers both the primary FFmpeg
// libraries and their transitive dependencies (liblzma for avformat, etc.).
// Missing any of these will cause ffmpeg.exe and ffprobe.exe to fail to load
// at runtime.
//
// The primary libraries use glob-friendly basenames (avcodec-*.dll) rather
// than hardcoded ABI versions so this list does not break when FFmpeg bumps
// a major version (e.g. -61 → -62).  The validation in ValidateFFmpegDLLs
// uses glob matching for these entries.
func ExpectedFFmpegDLLs() []string {
	return []string{
		"avcodec-*.dll",
		"avformat-*.dll",
		"avutil-*.dll",
		"swscale-*.dll",
		"swresample-*.dll",
		"avfilter-*.dll",
		"liblzma-*.dll",
	}
}

// ValidateFFmpegDLLs runs a live smoke test on the FFmpeg DLLs that have been
// added to PATH.  It checks that every expected DLL exists AND that the
// bundled ffprobe.exe (if present) can load the DLLs without error.
//
// Returns a consolidated error describing all failures, or nil if everything
// is ready.  The error text is suitable for display in a startup dialog.
func ValidateFFmpegDLLs() error {
	dllDir := FFmpegDllDir()
	var issues []string

	// — exist / ABI check — (glob patterns like avcodec-*.dll)
	for _, pattern := range ExpectedFFmpegDLLs() {
		matches, err := filepath.Glob(filepath.Join(dllDir, pattern))
		if err != nil || len(matches) == 0 {
			issues = append(issues, fmt.Sprintf("missing: %s (no file matching %s in %s)", pattern, pattern, dllDir))
		}
	}

	// — smoke test with bundled ffprobe (if present) —
	exeDir := exeDir()
	ffprobe := filepath.Join(exeDir, "ffprobe.exe")
	if _, err := os.Stat(ffprobe); err == nil {
		out, err := exec.Command(ffprobe, "-version").CombinedOutput()
		if err != nil {
			issues = append(issues, fmt.Sprintf("ffprobe smoke test FAILED (DLL load error): %v", err))
			// Include the first few lines of output — usually the DLL load error message.
			lines := strings.SplitN(string(out), "\n", 4)
			for _, l := range lines {
				if strings.TrimSpace(l) != "" {
					issues = append(issues, fmt.Sprintf("  -> %s", strings.TrimSpace(l)))
				}
			}
		} else {
			logging.Debug(logging.CatSystem, "FFmpeg DLL smoke test passed: ffprobe loaded successfully")
		}
	} else if !os.IsNotExist(err) {
		// ffprobe should exist in a release bundle; log if stat fails for
		// a reason other than "not found" (e.g. permission).
		logging.Debug(logging.CatSystem, "can't stat bundled ffprobe.exe (non-fatal): %v", err)
	}

	if len(issues) == 0 {
		return nil
	}
	return fmt.Errorf("FFmpeg DLL validation failed:\n  %s", strings.Join(issues, "\n  "))
}

// DiagnoseDLLSetup returns a multi-line string describing the current DLL
// search state.  Useful for the --dllcheck CLI flag and error dialogs.
func DiagnoseDLLSetup() string {
	var b strings.Builder

	b.WriteString("=== FFmpeg DLL Diagnostics ===\n")

	dllDir := FFmpegDllDir()
	b.WriteString(fmt.Sprintf("DLL directory: %s\n", dllDir))

	if dllsPresent(dllDir) {
		matches, _ := filepath.Glob(filepath.Join(dllDir, "*.dll"))
		b.WriteString(fmt.Sprintf("DLL files found: %d\n", len(matches)))
		for _, m := range matches {
			info, err := os.Stat(m)
			size := ""
			if err == nil {
				size = fmt.Sprintf(" (%d bytes)", info.Size())
			}
			b.WriteString(fmt.Sprintf("  %s%s\n", filepath.Base(m), size))
		}
	} else {
		b.WriteString("ERROR: no avcodec*.dll files found\n")
	}

	// Expected DLLs (glob patterns)
	for _, pattern := range ExpectedFFmpegDLLs() {
		matches, err := filepath.Glob(filepath.Join(dllDir, pattern))
		if err != nil || len(matches) == 0 {
			b.WriteString(fmt.Sprintf("  MISSING (expected): %s\n", pattern))
		}
	}

	// PATH
	b.WriteString(fmt.Sprintf("PATH entries: %d\n", len(filepath.SplitList(os.Getenv("PATH")))))
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if strings.Contains(strings.ToLower(p), "ffmpeg") ||
			strings.Contains(strings.ToLower(p), "dll") {
			b.WriteString(fmt.Sprintf("  FFMPEG/DLL: %s\n", p))
		}
	}

	// Bundled executables
	exeDir := exeDir()
	if exeDir != "" {
		for _, name := range []string{"ffmpeg.exe", "ffprobe.exe"} {
			path := filepath.Join(exeDir, name)
			if info, err := os.Stat(path); err == nil {
				b.WriteString(fmt.Sprintf("%s: %s (%d bytes)\n", name, path, info.Size()))
			} else {
				b.WriteString(fmt.Sprintf("%s: NOT FOUND beside exe\n", name))
			}
		}
	}

	b.WriteString("================================")
	return b.String()
}

// exeDir returns the directory containing the running executable.
func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exe)
}