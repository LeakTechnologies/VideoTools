//go:build native_media

package appcfg

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

const FFmpegDllBinPath = "VideoTools" + string(filepath.Separator) + "ffmpeg-dll"
const FFmpegDllZipURL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl-shared.zip"

// LocalFFmpegDllDir returns the path to local FFmpeg DLLs if they exist.
// Checks the bundled dll/ subfolder next to the exe first, then common
// developer install paths.
func LocalFFmpegDllDir() string {
	// Check bundled dll/ folder alongside the executable (MSIX / portable release)
	if exe, err := os.Executable(); err == nil {
		bundled := filepath.Join(filepath.Dir(exe), "dll")
		if _, err := os.Stat(filepath.Join(bundled, "avcodec.dll")); err == nil {
			return bundled
		}
		// BtbN versioned names (e.g. avcodec-61.dll)
		if matches, _ := filepath.Glob(filepath.Join(bundled, "avcodec*.dll")); len(matches) > 0 {
			return bundled
		}
	}

	// Common developer installation paths
	paths := []string{
		"C:\\ffmpeg\\bin",
		"C:\\Program Files\\ffmpeg\\bin",
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "ffmpeg", "bin"),
	}
	for _, p := range paths {
		if _, err := os.Stat(filepath.Join(p, "avcodec.dll")); err == nil {
			return p
		}
	}
	return ""
}

// copyLocalFFmpegDlls copies FFmpeg DLLs from local installation to app data.
// This ensures version consistency with the DLLs used for compilation.
func copyLocalFFmpegDlls(srcDir string) error {
	dllDir := FFmpegDllDir()
	if err := os.MkdirAll(dllDir, 0o755); err != nil {
		return &FFmpegBootstrapError{Err: fmt.Errorf("create directory: %w", err)}
	}

	// DLLs needed for native media (exclude CLI tools)
	neededDLLs := []string{
		"avcodec.dll", "avformat.dll", "avutil.dll", "swscale.dll",
		"swresample.dll", "avfilter.dll", "avdevice.dll",
	}

	for _, dll := range neededDLLs {
		srcPath := filepath.Join(srcDir, dll)
		dstPath := filepath.Join(dllDir, dll)
		if _, err := os.Stat(srcPath); err == nil {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return &FFmpegBootstrapError{Err: fmt.Errorf("read %s: %w", dll, err)}
			}
			if err := os.WriteFile(dstPath, data, 0755); err != nil {
				return &FFmpegBootstrapError{Err: fmt.Errorf("write %s: %w", dll, err)}
			}
		}
	}
	return nil
}

func FFmpegDllDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		home, _ := os.UserHomeDir()
		if home != "" {
			base = filepath.Join(home, "AppData", "Local")
		}
	}
	return filepath.Join(base, FFmpegDllBinPath)
}

func FFmpegDllsPresent() bool {
	dllDir := FFmpegDllDir()
	// BtbN shared builds use versioned DLL names (e.g. avcodec-61.dll).
	// Accept both "avcodec.dll" and "avcodec-*.dll" so the presence check
	// works regardless of FFmpeg major version.
	matches, err := filepath.Glob(filepath.Join(dllDir, "avcodec*.dll"))
	if err != nil || len(matches) == 0 {
		return false
	}
	return true
}

type FFmpegBootstrapError struct {
	Err error
}

func (e *FFmpegBootstrapError) Error() string {
	return fmt.Sprintf("FFmpeg bootstrap failed: %v", e.Err)
}

func (e *FFmpegBootstrapError) Unwrap() error {
	return e.Err
}

func BootstrapFFmpegDlls() error {
	// First, try to use local FFmpeg DLLs (matching compilation)
	if localDir := LocalFFmpegDllDir(); localDir != "" {
		logging.Info(logging.CatSystem, "Using local FFmpeg DLLs from: %s", localDir)
		return copyLocalFFmpegDlls(localDir)
	}

	if FFmpegDllsPresent() {
		return nil
	}

	logging.Info(logging.CatSystem, "No local FFmpeg found, downloading from BtbN...")

	dllDir := FFmpegDllDir()
	if err := os.MkdirAll(dllDir, 0o755); err != nil {
		return &FFmpegBootstrapError{Err: fmt.Errorf("create directory: %w", err)}
	}

	zipPath := filepath.Join(os.TempDir(), "videotools-ffmpeg-dlls.zip")
	defer os.Remove(zipPath)

	out, err := os.Create(zipPath)
	if err != nil {
		return &FFmpegBootstrapError{Err: fmt.Errorf("create temp file: %w", err)}
	}
	defer out.Close()

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(FFmpegDllZipURL)
	if err != nil {
		return &FFmpegBootstrapError{Err: fmt.Errorf("download: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &FFmpegBootstrapError{Err: fmt.Errorf("HTTP %d", resp.StatusCode)}
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		return &FFmpegBootstrapError{Err: fmt.Errorf("save: %w", err)}
	}
	out.Close()

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return &FFmpegBootstrapError{Err: fmt.Errorf("open zip: %w", err)}
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}

		base := strings.ToLower(filepath.Base(f.Name))
		if !strings.HasSuffix(base, ".dll") && !strings.HasSuffix(base, ".exe") {
			continue
		}

		src, err := f.Open()
		if err != nil {
			continue
		}

		dstPath := filepath.Join(dllDir, filepath.Base(f.Name))
		dst, err := os.Create(dstPath)
		if err != nil {
			src.Close()
			continue
		}

		io.Copy(dst, src)
		src.Close()
		dst.Close()
	}

	if !FFmpegDllsPresent() {
		return &FFmpegBootstrapError{Err: fmt.Errorf("DLLs not found after extraction")}
	}

	return nil
}

func AddFFmpegDllsToPath() error {
	if !FFmpegDllsPresent() {
		if err := BootstrapFFmpegDlls(); err != nil {
			return err
		}
	}

	dllDir := FFmpegDllDir()
	currentPath := os.Getenv("PATH")
	newPath := dllDir + string(os.PathListSeparator) + currentPath
	return os.Setenv("PATH", newPath)
}
