package settings

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
)

const WindowsFFmpegZipURL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
const WindowsPythonURL = "https://www.python.org/ftp/python/3.12.9/python-3.12.9-embed-amd64.zip"
const GetPipURL = "https://bootstrap.pypa.io/get-pip.py"

type BenchmarkCallbacks interface {
	Window() fyne.Window
	ShowBenchmark()
}

type PreferencesCallbacks interface {
	Window() fyne.Window
	ShowSettingsView()
	FullVersion() string
	BuildCommit() string
	UpdateLastChecked() time.Time
	ApplyUpdate(tag string)
	CheckForUpdatesWithStatus(statusIcon *widget.Icon, statusLabel *widget.Label, onAvailable func(tag string))
	ApplyUpdateStatusToUI(statusIcon *widget.Icon, statusLabel *widget.Label, onAvailable func(tag string))
	DetectBestHardwareAccel() string
	DetectHardwareAccelStatus() (best string, status string)
	PersistConvertConfig()
	ConvertHardwareAccel() string
	SetConvertHardwareAccel(accel string)
	ConvertShowUpscale() bool
	SetConvertShowUpscale(show bool)
	ConvertShowDisc() bool
	SetConvertShowDisc(show bool)
	PersistLocale(code string, script i18n.ScriptVariant)
	SavePrefsConfig() error
	PrefsConfig() PrefsConfig
	SetShowTooltips(enabled bool)
	DefaultOutputDir() string
	SetDefaultOutputDir(dir string)
}

type DependencyCallbacks interface {
	Window() fyne.Window
	ShowSettingsView()
	InstallWindowsFFmpeg(onDone func())
	InstallRealESRGAN(onDone func())
	InstallRealCUGAN(onDone func())
	InstallRIFE(onDone func())
	InstallWindowsPython(onDone func(pythonExe string))
	RunDependencyCommandWithProgress(title, message string, depCmd *DependencyCommand, onDone func(output string, err error))
	ShowCommandResult(title, output string, err error)
	AllDependencies() map[string]Dependency
	IsDependencyAvailableForPlatform(dep Dependency) bool
	GetDependencyCommands(depName string) DependencyCommandPair
	CheckDependency(command string) bool
	ModuleDependencies() map[string][]string
	ModulesList() []ModuleInfo
}

type ModuleInfo struct {
	ID    string
	Label string
}

type PrefsConfig struct {
	AutoCheckFrequency string `json:"AutoCheckFrequency"`
	QueuePlayBehavior  string `json:"QueuePlayBehavior"`
	DefaultOutputDir   string `json:"DefaultOutputDir"`
	ShowTooltips       bool   `json:"ShowTooltips"`
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
