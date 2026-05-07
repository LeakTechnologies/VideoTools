//go:build native_media

package appcfg

import (
	"fmt"
	"os"
	"path/filepath"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

// FFmpegDllDir returns the directory where FFmpeg DLLs are expected.
// We look next to the executable first (bundled), then fall back to
// %LOCALAPPDATA%\VideoTools\ffmpeg-dll (legacy download path).
func FFmpegDllDir() string {
	// Check next to executable (bundled DLLs)
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		bundledDir := filepath.Join(exeDir, "ffmpeg-dll")
		if dllsPresent(bundledDir) {
			return bundledDir
		}
	}

	// Fall back to LOCALAPPDATA (legacy path for old installs)
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		home, _ := os.UserHomeDir()
		if home != "" {
			base = filepath.Join(home, "AppData", "Local")
		}
	}
	return filepath.Join(base, "VideoTools", "ffmpeg-dll")
}

func dllsPresent(dir string) bool {
	matches, err := filepath.Glob(filepath.Join(dir, "avcodec*.dll"))
	return err == nil && len(matches) > 0
}

func FFmpegDllsPresent() bool {
	dllDir := FFmpegDllDir()
	matches, err := filepath.Glob(filepath.Join(dllDir, "avcodec*.dll"))
	if err != nil || len(matches) == 0 {
		return false
	}
	return true
}

func AddFFmpegDllsToPath() error {
	dllDir := FFmpegDllDir()

	if !FFmpegDllsPresent() {
		logging.Error(logging.CatSystem, "FFmpeg DLLs not found in %s — video playback will be unavailable", dllDir)
		return fmt.Errorf("FFmpeg DLLs not found in %s (expected bundled ffmpeg-dll/ next to exe)", dllDir)
	}

	currentPath := os.Getenv("PATH")
	newPath := dllDir + string(os.PathListSeparator) + currentPath
	return os.Setenv("PATH", newPath)
}