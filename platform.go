package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

// PlatformConfig holds platform-specific configuration
type PlatformConfig struct {
	FFmpegPath     string
	FFprobePath    string
	TempDir        string
	HWEncoders     []string
	ExeExtension   string
	PathSeparator  string
	IsWindows      bool
	IsLinux        bool
	IsDarwin       bool
}

// DetectPlatform detects the current platform and returns configuration
func DetectPlatform() *PlatformConfig {
	cfg := &PlatformConfig{
		IsWindows:     runtime.GOOS == "windows",
		IsLinux:       runtime.GOOS == "linux",
		IsDarwin:      runtime.GOOS == "darwin",
		PathSeparator: string(filepath.Separator),
	}

	if cfg.IsWindows {
		cfg.ExeExtension = ".exe"
	}

	cfg.FFmpegPath = findFFmpeg(cfg)
	cfg.FFprobePath = findFFprobe(cfg)
	cfg.TempDir = getTempDir(cfg)
	cfg.HWEncoders = detectHardwareEncoders(cfg)

	logging.Debug(logging.CatSystem, "Platform detected: %s/%s", runtime.GOOS, runtime.GOARCH)
	logging.Debug(logging.CatSystem, "FFmpeg path: %s", cfg.FFmpegPath)
	logging.Debug(logging.CatSystem, "FFprobe path: %s", cfg.FFprobePath)
	logging.Debug(logging.CatSystem, "Temp directory: %s", cfg.TempDir)
	logging.Debug(logging.CatSystem, "Hardware encoders: %v", cfg.HWEncoders)

	return cfg
}

// findFFmpeg locates the ffmpeg executable
func findFFmpeg(cfg *PlatformConfig) string {
	exeName := "ffmpeg"
	if cfg.IsWindows {
		exeName = "ffmpeg.exe"
	}

	// Priority 1: Bundled with application
	if exePath, err := os.Executable(); err == nil {
		bundled := filepath.Join(filepath.Dir(exePath), exeName)
		if _, err := os.Stat(bundled); err == nil {
			logging.Debug(logging.CatSystem, "Found bundled ffmpeg: %s", bundled)
			return bundled
		}
	}

	// Priority 2: Environment variable
	if envPath := os.Getenv("FFMPEG_PATH"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			logging.Debug(logging.CatSystem, "Found ffmpeg from FFMPEG_PATH: %s", envPath)
			return envPath
		}
	}

	// Priority 3: System PATH
	if path, err := exec.LookPath(exeName); err == nil {
		logging.Debug(logging.CatSystem, "Found ffmpeg in PATH: %s", path)
		return path
	}

	// Priority 4: Common install locations (Windows)
	if cfg.IsWindows {
		commonPaths := []string{
			filepath.Join(os.Getenv("ProgramFiles"), "ffmpeg", "bin", "ffmpeg.exe"),
			filepath.Join(os.Getenv("ProgramFiles(x86)"), "ffmpeg", "bin", "ffmpeg.exe"),
			`C:\ffmpeg\bin\ffmpeg.exe`,
		}
		for _, path := range commonPaths {
			if _, err := os.Stat(path); err == nil {
				logging.Debug(logging.CatSystem, "Found ffmpeg at common location: %s", path)
				return path
			}
		}
	}

	// Fallback: assume it's in PATH (will error later if not found)
	logging.Debug(logging.CatSystem, "FFmpeg not found, using fallback: %s", exeName)
	return exeName
}

// findFFprobe locates the ffprobe executable
func findFFprobe(cfg *PlatformConfig) string {
	exeName := "ffprobe"
	if cfg.IsWindows {
		exeName = "ffprobe.exe"
	}

	// Priority 1: Same directory as ffmpeg
	ffmpegDir := filepath.Dir(cfg.FFmpegPath)
	if ffmpegDir != "." && ffmpegDir != "" {
		probePath := filepath.Join(ffmpegDir, exeName)
		if _, err := os.Stat(probePath); err == nil {
			return probePath
		}
	}

	// Priority 2: Bundled with application
	if exePath, err := os.Executable(); err == nil {
		bundled := filepath.Join(filepath.Dir(exePath), exeName)
		if _, err := os.Stat(bundled); err == nil {
			return bundled
		}
	}

	// Priority 3: System PATH
	if path, err := exec.LookPath(exeName); err == nil {
		return path
	}

	// Fallback
	return exeName
}

// getTempDir returns platform-appropriate temp directory
func getTempDir(cfg *PlatformConfig) string {
	var base string

	if cfg.IsWindows {
		// Windows: Use AppData\Local\Temp\VideoTools
		appData := os.Getenv("LOCALAPPDATA")
		if appData != "" {
			base = filepath.Join(appData, "Temp", "VideoTools")
		} else {
			base = filepath.Join(os.TempDir(), "VideoTools")
		}
	} else {
		// Linux/macOS: Use /tmp/videotools
		base = filepath.Join(os.TempDir(), "videotools")
	}

	// Ensure directory exists
	if err := os.MkdirAll(base, 0755); err != nil {
		logging.Debug(logging.CatSystem, "Failed to create temp directory %s: %v", base, err)
		return os.TempDir()
	}

	return base
}

// detectHardwareEncoders detects available hardware encoders
func detectHardwareEncoders(cfg *PlatformConfig) []string {
	var encoders []string

	// Get list of available encoders from ffmpeg
	cmd := exec.Command(cfg.FFmpegPath, "-hide_banner", "-encoders")
	output, err := cmd.Output()
	if err != nil {
		logging.Debug(logging.CatSystem, "Failed to query ffmpeg encoders: %v", err)
		return encoders
	}

	encoderList := string(output)

	// Platform-specific encoder detection
	if cfg.IsWindows {
		// Windows: Check for NVENC, QSV, AMF
		if strings.Contains(encoderList, "h264_nvenc") {
			encoders = append(encoders, "nvenc")
			logging.Debug(logging.CatSystem, "Detected NVENC (NVIDIA) encoder")
		}
		if strings.Contains(encoderList, "h264_qsv") {
			encoders = append(encoders, "qsv")
			logging.Debug(logging.CatSystem, "Detected QSV (Intel) encoder")
		}
		if strings.Contains(encoderList, "h264_amf") {
			encoders = append(encoders, "amf")
			logging.Debug(logging.CatSystem, "Detected AMF (AMD) encoder")
		}
	} else if cfg.IsLinux {
		// Linux: Check for VAAPI, NVENC, QSV
		if strings.Contains(encoderList, "h264_vaapi") {
			encoders = append(encoders, "vaapi")
			logging.Debug(logging.CatSystem, "Detected VAAPI encoder")
		}
		if strings.Contains(encoderList, "h264_nvenc") {
			encoders = append(encoders, "nvenc")
			logging.Debug(logging.CatSystem, "Detected NVENC encoder")
		}
		if strings.Contains(encoderList, "h264_qsv") {
			encoders = append(encoders, "qsv")
			logging.Debug(logging.CatSystem, "Detected QSV encoder")
		}
	} else if cfg.IsDarwin {
		// macOS: Check for VideoToolbox, NVENC
		if strings.Contains(encoderList, "h264_videotoolbox") {
			encoders = append(encoders, "videotoolbox")
			logging.Debug(logging.CatSystem, "Detected VideoToolbox encoder")
		}
		if strings.Contains(encoderList, "h264_nvenc") {
			encoders = append(encoders, "nvenc")
			logging.Debug(logging.CatSystem, "Detected NVENC encoder")
		}
	}

	return encoders
}

// ValidateWindowsPath validates Windows-specific path constraints
func ValidateWindowsPath(path string) error {
	if runtime.GOOS != "windows" {
		return nil
	}

	if len(path) == 0 {
		return fmt.Errorf("empty path")
	}

	// Check for drive letter (C:, D:, etc.)
	if len(path) >= 2 && path[1] == ':' {
		drive := strings.ToUpper(string(path[0]))
		if drive < "A" || drive > "Z" {
			return fmt.Errorf("invalid drive letter: %s", drive)
		}
		return nil
	}

	// Check for UNC path (\\server\share)
	if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, `//`) {
		parts := strings.Split(strings.TrimPrefix(strings.TrimPrefix(path, `\\`), `//`), `\`)
		if len(parts) < 2 {
			return fmt.Errorf("invalid UNC path: %s", path)
		}
		return nil
	}

	// Relative path is OK
	return nil
}

// KillProcess kills a process in a platform-appropriate way
func KillProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	if runtime.GOOS == "windows" {
		// Windows: Kill directly (no SIGTERM support)
		return cmd.Process.Kill()
	}

	// Unix: Try graceful shutdown first
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		return cmd.Process.Kill()
	}

	// Give it a moment to shut down gracefully
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		return nil
	case <-time.After(2 * time.Second):
		// Timeout, force kill
		return cmd.Process.Kill()
	}
}

// GetEncoderName returns the full encoder name for a given hardware acceleration type and codec
func GetEncoderName(hwAccel, codec string) string {
	if hwAccel == "none" || hwAccel == "" {
		// Software encoding
		switch codec {
		case "H.264":
			return "libx264"
		case "H.265", "HEVC":
			return "libx265"
		case "VP9":
			return "libvpx-vp9"
		case "AV1":
			return "libaom-av1"
		default:
			return "libx264"
		}
	}

	// Hardware encoding
	codecSuffix := ""
	switch codec {
	case "H.264":
		codecSuffix = "h264"
	case "H.265", "HEVC":
		codecSuffix = "hevc"
	default:
		codecSuffix = "h264"
	}

	switch hwAccel {
	case "nvenc":
		return fmt.Sprintf("%s_nvenc", codecSuffix)
	case "qsv":
		return fmt.Sprintf("%s_qsv", codecSuffix)
	case "vaapi":
		return fmt.Sprintf("%s_vaapi", codecSuffix)
	case "videotoolbox":
		return fmt.Sprintf("%s_videotoolbox", codecSuffix)
	case "amf":
		return fmt.Sprintf("%s_amf", codecSuffix)
	default:
		return fmt.Sprintf("lib%s", strings.ToLower(codec))
	}
}
