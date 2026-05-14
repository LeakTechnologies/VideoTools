//go:build native_media

package appcfg

import (
	"fmt"
	"os"
	"path/filepath"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
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

		// 1. DLL/ subfolder (CI/release packaging)
		bundledDir := filepath.Join(exeDir, "DLL")
		if dllsPresent(bundledDir) {
			return bundledDir
		}

		// 2. Flat DLLs next to the exe (local dev builds,
		//    or users who extracted files flat from the ZIP)
		if dllsPresent(exeDir) {
			return exeDir
		}
	}

	// 3. Fall back to LOCALAPPDATA (legacy path for old installs)
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
		return fmt.Errorf("FFmpeg DLLs not found in %s (looked in: DLL/ next to exe, exe directory, %%LOCALAPPDATA%%/VideoTools/DLL)", dllDir)
	}

	currentPath := os.Getenv("PATH")
	newPath := dllDir + string(os.PathListSeparator) + currentPath
	return os.Setenv("PATH", newPath)
}