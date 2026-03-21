package settings

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const WindowsFFmpegZipURL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
const WindowsFFmpegDllZipURL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl-shared.zip"
const WindowsPythonURL = "https://www.python.org/ftp/python/3.12.9/python-3.12.9-embed-amd64.zip"
const GetPipURL = "https://bootstrap.pypa.io/get-pip.py"

const FFmpegDllBinPath = "VideoTools" + string(filepath.Separator) + "ffmpeg-dll"

type PrefsConfig struct {
	AutoCheckFrequency string `json:"AutoCheckFrequency"`
	QueuePlayBehavior  string `json:"QueuePlayBehavior"`
}

type Dependency struct {
	Name         string
	Command      string
	Required     bool
	Description  string
	InstallCmd   string
	UninstallCmd string
	Platforms    []string
}

type DependencyCommand struct {
	Command string
	Args    []string
}

type DependencyCommandPair struct {
	Install   *DependencyCommand
	Uninstall *DependencyCommand
}

type UpdateInfo struct {
	LatestTag    string
	TagCommitSHA string
	ReleaseDate  int64
}

type BenchmarkConfig struct {
	History []BenchmarkRun `json:"history"`
}

type BenchmarkRun struct {
	Timestamp           int64   `json:"timestamp"`
	Encoder             string  `json:"encoder"`
	Preset              string  `json:"preset"`
	Resolution          string  `json:"resolution"`
	EncodingTimeSeconds float64 `json:"encodingTimeSeconds"`
	RecommendedEncoder  string  `json:"recommendedEncoder"`
	RecommendedPreset   string  `json:"recommendedPreset"`
	RecommendedCRF      int     `json:"recommendedCRF"`
}

func NewDependencyCommand(command string, args ...string) *DependencyCommand {
	return &DependencyCommand{Command: command, Args: args}
}

func NewDependencyCommandPair(install, uninstall *DependencyCommand) DependencyCommandPair {
	return DependencyCommandPair{Install: install, Uninstall: uninstall}
}

func ProjectRoot() string {
	if exe, err := os.Executable(); err == nil {
		if dir := filepath.Dir(exe); dir != "" {
			return dir
		}
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

func DetectPkgManager() string {
	managers := []string{"apt-get", "dnf", "pacman", "zypper"}
	for _, m := range managers {
		if _, err := exec.LookPath(m); err == nil {
			return m
		}
	}
	return ""
}

func PkgManagerInstall(pkg string) *DependencyCommand {
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("brew"); err == nil {
			return NewDependencyCommand("brew", "install", pkg)
		}
	case "linux":
		switch DetectPkgManager() {
		case "apt-get":
			return NewDependencyCommand("sudo", "apt-get", "install", "-y", pkg)
		case "dnf":
			return NewDependencyCommand("sudo", "dnf", "install", "-y", pkg)
		case "pacman":
			return NewDependencyCommand("sudo", "pacman", "-S", "--needed", "--noconfirm", pkg)
		case "zypper":
			return NewDependencyCommand("sudo", "zypper", "install", "-y", pkg)
		}
	case "windows":
		if _, err := exec.LookPath("choco"); err == nil {
			return NewDependencyCommand("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", fmt.Sprintf("choco install -y %s", pkg))
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
	avcodecDll := filepath.Join(dllDir, "avcodec.dll")
	if _, err := os.Stat(avcodecDll); err != nil {
		return false
	}
	return true
}

func BootstrapFFmpegDlls() error {
	if FFmpegDllsPresent() {
		return nil
	}

	dllDir := FFmpegDllDir()
	if err := os.MkdirAll(dllDir, 0o755); err != nil {
		return fmt.Errorf("create FFmpeg DLL directory: %w", err)
	}

	zipPath := filepath.Join(os.TempDir(), "videotools-ffmpeg-dlls.zip")
	defer os.Remove(zipPath)

	out, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer out.Close()

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(WindowsFFmpegDllZipURL)
	if err != nil {
		return fmt.Errorf("download FFmpeg DLLs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download FFmpeg DLLs: HTTP %d", resp.StatusCode)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("save FFmpeg DLLs: %w", err)
	}
	out.Close()

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
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

	return nil
}

func AddFFmpegDllsToPath() error {
	dllDir := FFmpegDllDir()
	if !FFmpegDllsPresent() {
		if err := BootstrapFFmpegDlls(); err != nil {
			return err
		}
	}

	currentPath := os.Getenv("PATH")
	newPath := dllDir + string(os.PathListSeparator) + currentPath
	return os.Setenv("PATH", newPath)
}
