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
	if FFmpegDllsPresent() {
		return nil
	}

	logging.Info(logging.CatSystem, "Downloading FFmpeg DLLs from BtbN...")
	return downloadBtbNSharedBinaries()
}

func downloadBtbNSharedBinaries() error {
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

	return extractBtbNZip(zipPath, dllDir)
}

func extractBtbNZip(zipPath, dllDir string) error {
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