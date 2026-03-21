package settings

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const WindowsFFmpegZipURL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
const WindowsPythonURL = "https://www.python.org/ftp/python/3.12.9/python-3.12.9-embed-amd64.zip"
const GetPipURL = "https://bootstrap.pypa.io/get-pip.py"

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
