package main

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"image/color"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

// Dependency represents a system dependency
type Dependency struct {
	Name          string
	Command       string // Command to check if installed
	Required      bool   // If true, core functionality requires this
	Description   string
	InstallCmd    string // Command to install (platform-specific)
	UninstallCmd string // Command to uninstall (platform-specific, optional)
	Platforms     []string // List of platforms: "windows", "linux", "darwin". Empty = all.
}

// dependencyCommand represents a command with optional arguments
// command must be non-empty; args may be empty
type dependencyCommand struct {
	command string
	args    []string
}

// dependencyCommandPair holds install/uninstall commands
// nil entries mean unavailable for current platform
type dependencyCommandPair struct {
	install   *dependencyCommand
	uninstall *dependencyCommand
}

const windowsFFmpegZipURL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"

func projectRoot() string {
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

func detectPkgManager() string {
	managers := []string{"apt-get", "dnf", "pacman", "zypper"}
	for _, m := range managers {
		if _, err := exec.LookPath(m); err == nil {
			return m
		}
	}
	return ""
}

func pkgManagerInstall(pkg string) *dependencyCommand {
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("brew"); err == nil {
			return &dependencyCommand{command: "brew", args: []string{"install", pkg}}
		}
	case "linux":
		switch detectPkgManager() {
		case "apt-get":
			return &dependencyCommand{command: "sudo", args: []string{"apt-get", "install", "-y", pkg}}
		case "dnf":
			return &dependencyCommand{command: "sudo", args: []string{"dnf", "install", "-y", pkg}}
		case "pacman":
			return &dependencyCommand{command: "sudo", args: []string{"pacman", "-S", "--needed", "--noconfirm", pkg}}
		case "zypper":
			return &dependencyCommand{command: "sudo", args: []string{"zypper", "install", "-y", pkg}}
		}
	case "windows":
		if _, err := exec.LookPath("choco"); err == nil {
			return &dependencyCommand{command: "powershell", args: []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", fmt.Sprintf("choco install -y %s", pkg)}}
		}
	}
	return nil
}

func pkgManagerUninstall(pkg string) *dependencyCommand {
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("brew"); err == nil {
			return &dependencyCommand{command: "brew", args: []string{"uninstall", pkg}}
		}
	case "linux":
		switch detectPkgManager() {
		case "apt-get":
			return &dependencyCommand{command: "sudo", args: []string{"apt-get", "remove", "-y", pkg}}
		case "dnf":
			return &dependencyCommand{command: "sudo", args: []string{"dnf", "remove", "-y", pkg}}
		case "pacman":
			return &dependencyCommand{command: "sudo", args: []string{"pacman", "-Rns", "--noconfirm", pkg}}
		case "zypper":
			return &dependencyCommand{command: "sudo", args: []string{"zypper", "remove", "-y", pkg}}
		}
	case "windows":
		if _, err := exec.LookPath("choco"); err == nil {
			return &dependencyCommand{command: "powershell", args: []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", fmt.Sprintf("choco uninstall -y %s", pkg)}}
		}
	}
	return nil
}

func getDependencyCommands(depName string) dependencyCommandPair {
	root := projectRoot()
	switch depName {
	case "ffmpeg":
		if runtime.GOOS == "windows" {
			// Windows uses the app-local installer from the Settings UI.
			return dependencyCommandPair{}
		}
		return dependencyCommandPair{
			install:   pkgManagerInstall("ffmpeg"),
			uninstall: pkgManagerUninstall("ffmpeg"),
		}
	case "dvdauthor":
		// Windows: Use WSL installation via Settings UI
		if runtime.GOOS == "windows" {
			return dependencyCommandPair{}
		}
		return dependencyCommandPair{
			install:   pkgManagerInstall("dvdauthor"),
			uninstall: pkgManagerUninstall("dvdauthor"),
		}
	case "xorriso":
		// Windows: Use WSL installation via Settings UI
		if runtime.GOOS == "windows" {
			return dependencyCommandPair{}
		}
		return dependencyCommandPair{
			install:   pkgManagerInstall("xorriso"),
			uninstall: pkgManagerUninstall("xorriso"),
		}
	case "realesrgan-ncnn-vulkan":
		// Best-effort: invoke existing installer with AI enabled
		installScript := filepath.Join(root, "scripts", "install.sh")
		switch runtime.GOOS {
		case "linux", "darwin":
			return dependencyCommandPair{
				install: &dependencyCommand{command: "bash", args: []string{installScript, "--skip-ai=false", "--skip-dvd", "--skip-whisper"}},
			}
		case "windows":
			// Not readily available via package manager; fall back to warning
			return dependencyCommandPair{}
		}
	case "whisper":
		if runtime.GOOS == "windows" {
			return dependencyCommandPair{
				install:   &dependencyCommand{command: "powershell", args: []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "py -m pip install --user openai-whisper"}},
				uninstall: &dependencyCommand{command: "powershell", args: []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "py -m pip uninstall -y openai-whisper"}},
			}
		}
		return dependencyCommandPair{
			install:   &dependencyCommand{command: "python3", args: []string{"-m", "pip", "install", "--user", "openai-whisper"}},
			uninstall: &dependencyCommand{command: "python3", args: []string{"-m", "pip", "uninstall", "-y", "openai-whisper"}},
		}
	case "tesseract":
		switch runtime.GOOS {
		case "windows":
			if _, err := exec.LookPath("choco"); err == nil {
				return dependencyCommandPair{
					install:   &dependencyCommand{command: "powershell", args: []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "choco install -y tesseract"}},
					uninstall: &dependencyCommand{command: "powershell", args: []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "choco uninstall -y tesseract"}},
				}
			}
			return dependencyCommandPair{}
		case "darwin":
			return dependencyCommandPair{
				install:   pkgManagerInstall("tesseract"),
				uninstall: pkgManagerUninstall("tesseract"),
			}
		default:
			pkg := "tesseract"
			if detectPkgManager() == "apt-get" {
				pkg = "tesseract-ocr"
			}
			return dependencyCommandPair{
				install:   pkgManagerInstall(pkg),
				uninstall: pkgManagerUninstall(pkg),
			}
		}
	}
	return dependencyCommandPair{}
}

func runDependencyCommandWithProgress(win fyne.Window, title, message string, depCmd *dependencyCommand, onDone func(output string, err error)) {
	if depCmd == nil {
		dialog.ShowError(fmt.Errorf("no command available for this platform"), win)
		return
	}
	progress := dialog.NewProgressInfinite(title, message, win)
	progress.Show()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		cmd := utils.CreateCommand(ctx, depCmd.command, depCmd.args...)
		cmd.Dir = projectRoot()
		output, err := cmd.CombinedOutput()
		trimmed := strings.TrimSpace(string(output))

		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			progress.Hide()
			onDone(trimmed, err)
		}, false)
	}()
}

func showCommandResult(win fyne.Window, title string, output string, err error) {
	const maxLen = 2000
	if len(output) > maxLen {
		output = output[:maxLen] + "..."
	}

	if err != nil {
		dialog.ShowError(fmt.Errorf("command failed: %w\n%s", err, output), win)
		return
	}
	if output == "" {
		dialog.ShowInformation(title, "Completed successfully.", win)
		return
	}
	dialog.ShowInformation(title, output, win)
}

// ModuleDependencies maps module IDs to their required dependencies
var moduleDependencies = map[string][]string{
	"convert":   {"ffmpeg"},
	"merge":     {"ffmpeg"},
	"trim":      {"ffmpeg"},
	"filters":   {"ffmpeg"},
	"upscale":   {"ffmpeg"}, // realesrgan-ncnn-vulkan is optional for AI upscaling
	"audio":     {"ffmpeg"},
	"author":    {"ffmpeg", "dvdauthor", "xorriso"},
	"rip":       {"ffmpeg", "xorriso"},
	"bluray":    {"ffmpeg"},
	"subtitles": {"ffmpeg"},
	"thumbnail": {"ffmpeg"},
	"compare":   {"ffmpeg"},
	"inspect":   {"ffmpeg"},
	"player":    {"ffmpeg"},
}

// AllDependencies defines all possible dependencies
var allDependencies = map[string]Dependency{
	"ffmpeg": {
		Name:        "FFmpeg",
		Command:     "ffmpeg",
		Required:    true,
		Description: "Core video processing engine",
		InstallCmd:  getFFmpegInstallCmd(),
		Platforms:   []string{"windows", "linux", "darwin"},
	},
	"dvdauthor": {
		Name:        "DVDAuthor",
		Command:     "dvdauthor",
		Required:    false,
		Description: "DVD authoring tool",
		InstallCmd:  getDVDAuthorInstallCmd(),
		Platforms:   []string{"windows", "linux", "darwin"},
	},
	"xorriso": {
		Name:        "xorriso",
		Command:     "xorriso",
		Required:    false,
		Description: "ISO creation and extraction",
		InstallCmd:  getXorrisoInstallCmd(),
		Platforms:   []string{"windows", "linux", "darwin"},
	},
	"realesrgan-ncnn-vulkan": {
		Name:        "Real-ESRGAN",
		Command:     "realesrgan-ncnn-vulkan",
		Required:    false,
		Description: "AI video upscaling",
		InstallCmd:  "See install.sh --skip-ai=false",
		Platforms:   []string{"linux", "darwin"},
	},
	"whisper": {
		Name:        "Whisper",
		Command:     "whisper",
		Required:    false,
		Description: "AI subtitle generation",
		InstallCmd:  "pip3 install --user openai-whisper",
		Platforms:   []string{"windows", "linux", "darwin"},
	},
	"tesseract": {
		Name:        "Tesseract OCR",
		Command:     "tesseract",
		Required:    false,
		Description: "OCR for image-based subtitles",
		InstallCmd:  "Install via package manager (tesseract-ocr)",
		Platforms:   []string{"windows", "linux", "darwin"},
	},
}

func getFFmpegInstallCmd() string {
	switch runtime.GOOS {
	case "linux":
		return "Install from Settings (recommended) or run: ./scripts/linux/install.sh"
	case "darwin":
		return "Install from Settings (recommended) or run: brew install ffmpeg"
	case "windows":
		return "Install from Settings (recommended) or run: .\\scripts\\windows\\install.ps1"
	default:
		return "See ffmpeg.org for installation"
	}
}

func getDVDAuthorInstallCmd() string {
	switch runtime.GOOS {
	case "linux":
		return "sudo apt-get install dvdauthor  # or dnf/pacman/zypper"
	case "darwin":
		return "brew install dvdauthor"
	case "windows":
		return "Install via Settings (auto-installs WSL with Ubuntu)"
	default:
		return "Install from package manager or Settings"
	}
}

func getXorrisoInstallCmd() string {
	switch runtime.GOOS {
	case "linux":
		return "sudo apt-get install xorriso  # or dnf/pacman/zypper"
	case "darwin":
		return "brew install xorriso"
	case "windows":
		return "Install via Settings (auto-installs WSL with Ubuntu)"
	default:
		return "Install from package manager or Settings"
	}
}

func checkWSLInstalled() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	cmd := exec.Command("wsl", "--status")
	return cmd.Run() == nil
}

func installWSLWithDvdTools(win fyne.Window, onDone func(success bool, message string)) {
	if runtime.GOOS != "windows" {
		onDone(false, "WSL installation is only available on Windows")
		return
	}

	progress := dialog.NewProgressInfinite("Installing WSL", "Installing WSL2 with Ubuntu and DVD authoring tools...", win)
	progress.Show()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		var log strings.Builder

		// Check if WSL is already installed
		checkCmd := exec.CommandContext(ctx, "wsl", "--status")
		if checkCmd.Run() == nil {
			log.WriteString("WSL already installed. ")
		} else {
			// Install WSL
			log.WriteString("Installing WSL2... ")
			wslCmd := exec.CommandContext(ctx, "powershell", "-Command", "wsl --install -d Ubuntu --no-interactive")
			wslCmd.Stdout = &log
			wslCmd.Stderr = &log
			if err := wslCmd.Run(); err != nil {
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					progress.Hide()
					onDone(false, "Failed to install WSL: "+log.String())
				}, false)
				return
			}
			log.WriteString("WSL installed. ")
		}

		// Install dvdauthor and xorriso in WSL
		log.WriteString("Installing dvdauthor and xorriso in WSL... ")
		updateCmd := exec.CommandContext(ctx, "wsl", "-d", "Ubuntu", "--", "sudo", "apt-get", "update", "-qq")
		updateCmd.Stdout = &log
		updateCmd.Stderr = &log
		if err := updateCmd.Run(); err != nil {
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				progress.Hide()
				onDone(false, "Failed to update WSL: "+log.String())
			}, false)
			return
		}

		installCmd := exec.CommandContext(ctx, "wsl", "-d", "Ubuntu", "--", "sudo", "apt-get", "install", "-y", "dvdauthor", "xorriso")
		installCmd.Stdout = &log
		installCmd.Stderr = &log
		if err := installCmd.Run(); err != nil {
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				progress.Hide()
				onDone(false, "Failed to install tools: "+log.String())
			}, false)
			return
		}

		log.WriteString("Done!")

		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			progress.Hide()
			onDone(true, "WSL with Ubuntu, dvdauthor, and xorriso installed successfully!")
		}, false)
	}()
}

func isDependencyAvailableForPlatform(dep Dependency) bool {
	if len(dep.Platforms) == 0 {
		return true
	}
	for _, p := range dep.Platforms {
		if p == runtime.GOOS {
			return true
		}
	}
	return false
}

// checkDependency checks if a command is available
func checkDependency(command string) bool {
	if command == "ffmpeg" {
		// Respect app-configured runtime paths so app-local FFmpeg bootstrap counts as installed.
		ffmpegPath := utils.GetFFmpegPath()
		if ffmpegPath != "" && ffmpegPath != "ffmpeg" && ffmpegPath != "ffmpeg.exe" {
			if _, err := os.Stat(ffmpegPath); err == nil {
				return true
			}
		}
	}

	// Check locally first
	_, err := exec.LookPath(command)
	if err == nil {
		return true
	}

	// On Windows, check WSL for dvdauthor and xorriso
	if runtime.GOOS == "windows" && (command == "dvdauthor" || command == "xorriso") {
		checkCmd := exec.Command("wsl", "-d", "Ubuntu", "--", "which", command)
		return checkCmd.Run() == nil
	}

	return false
}

func windowsAppLocalFFmpegPaths() (ffmpegPath, ffprobePath string, ok bool) {
	if runtime.GOOS != "windows" {
		return "", "", false
	}
	base := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", "", false
		}
		base = filepath.Join(home, "AppData", "Local")
	}
	binDir := filepath.Join(base, "VideoTools", "bin")
	ffmpegPath = filepath.Join(binDir, "ffmpeg.exe")
	ffprobePath = filepath.Join(binDir, "ffprobe.exe")
	return ffmpegPath, ffprobePath, true
}

func ensureWindowsAppLocalFFmpeg() (ffmpegPath, ffprobePath string, installed bool, err error) {
	ffmpegPath, ffprobePath, ok := windowsAppLocalFFmpegPaths()
	if !ok {
		return "", "", false, fmt.Errorf("unsupported platform")
	}

	if _, errFFmpeg := os.Stat(ffmpegPath); errFFmpeg == nil {
		if _, errFFprobe := os.Stat(ffprobePath); errFFprobe == nil {
			return ffmpegPath, ffprobePath, false, nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(ffmpegPath), 0o755); err != nil {
		return "", "", false, fmt.Errorf("create local ffmpeg directory: %w", err)
	}

	zipFile, err := os.CreateTemp("", "videotools-ffmpeg-*.zip")
	if err != nil {
		return "", "", false, fmt.Errorf("create temp zip: %w", err)
	}
	zipPath := zipFile.Name()
	_ = zipFile.Close()
	defer os.Remove(zipPath)

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(windowsFFmpegZipURL)
	if err != nil {
		return "", "", false, fmt.Errorf("download ffmpeg package: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", false, fmt.Errorf("download ffmpeg package: unexpected status %d", resp.StatusCode)
	}

	out, err := os.Create(zipPath)
	if err != nil {
		return "", "", false, fmt.Errorf("open temp zip for write: %w", err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		return "", "", false, fmt.Errorf("write ffmpeg package: %w", err)
	}
	if err := out.Close(); err != nil {
		return "", "", false, fmt.Errorf("finalize ffmpeg package: %w", err)
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", "", false, fmt.Errorf("open ffmpeg package: %w", err)
	}
	defer zr.Close()

	var copiedFFmpeg, copiedFFprobe bool
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := strings.ToLower(filepath.Base(f.Name))
		var target string
		switch base {
		case "ffmpeg.exe":
			target = ffmpegPath
		case "ffprobe.exe":
			target = ffprobePath
		default:
			continue
		}

		src, err := f.Open()
		if err != nil {
			return "", "", false, fmt.Errorf("extract %s: %w", base, err)
		}
		dst, err := os.Create(target)
		if err != nil {
			_ = src.Close()
			return "", "", false, fmt.Errorf("create %s: %w", target, err)
		}
		if _, err := io.Copy(dst, src); err != nil {
			_ = dst.Close()
			_ = src.Close()
			return "", "", false, fmt.Errorf("write %s: %w", target, err)
		}
		if err := dst.Close(); err != nil {
			_ = src.Close()
			return "", "", false, fmt.Errorf("close %s: %w", target, err)
		}
		_ = src.Close()

		if base == "ffmpeg.exe" {
			copiedFFmpeg = true
		}
		if base == "ffprobe.exe" {
			copiedFFprobe = true
		}
		if copiedFFmpeg && copiedFFprobe {
			break
		}
	}

	if !copiedFFmpeg || !copiedFFprobe {
		return "", "", false, fmt.Errorf("ffmpeg package missing ffmpeg.exe or ffprobe.exe")
	}

	return ffmpegPath, ffprobePath, true, nil
}

func (s *appState) installWindowsFFmpegFromUI(onSuccess func()) {
	progress := dialog.NewProgressInfinite("Installing FFmpeg", "Downloading and extracting FFmpeg. This may take a minute.", s.window)
	progress.Show()

	go func() {
		ffmpegPath, ffprobePath, _, err := ensureWindowsAppLocalFFmpeg()
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			progress.Hide()
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to install FFmpeg: %w", err), s.window)
				return
			}
			utils.SetFFmpegPaths(ffmpegPath, ffprobePath)
			if onSuccess != nil {
				onSuccess()
			}
		}, false)
	}()
}

func (s *appState) maybePromptWindowsDependencyBootstrap() {
	if runtime.GOOS != "windows" {
		return
	}
	if checkDependency("ffmpeg") {
		return
	}

	message := widget.NewLabel("Core dependency FFmpeg is missing.\n\nInstall now to unlock most modules.\n\nInstall target:\n%LOCALAPPDATA%\\VideoTools\\bin")
	message.Wrapping = fyne.TextWrapWord

	var prompt dialog.Dialog
	installBtn := widget.NewButton("Install FFmpeg Now", func() {
		s.installWindowsFFmpegFromUI(func() {
			if prompt != nil {
				prompt.Hide()
			}
			s.showMainMenu()
			dialog.ShowInformation("FFmpeg Ready", "FFmpeg is installed for this user. Video modules are now available.", s.window)
		})
	})
	installBtn.Importance = widget.HighImportance

	settingsBtn := widget.NewButton("Open Settings", func() {
		if prompt != nil {
			prompt.Hide()
		}
		s.showSettingsView()
	})
	continueBtn := widget.NewButton("Continue Limited Mode", func() {
		if prompt != nil {
			prompt.Hide()
		}
	})

	content := container.NewVBox(
		message,
		widget.NewSeparator(),
		container.NewHBox(installBtn, settingsBtn, continueBtn),
	)

	prompt = dialog.NewCustom("First Run Dependency Setup", "Close", content, s.window)
	prompt.Resize(fyne.NewSize(560, 220))
	prompt.Show()
}

// getModuleDependencyStatus checks which dependencies a module is missing
func getModuleDependencyStatus(moduleID string) (missing []string, hasAll bool) {
	deps, ok := moduleDependencies[moduleID]
	if !ok {
		return nil, true // Module has no dependencies
	}

	for _, depName := range deps {
		dep, exists := allDependencies[depName]
		if !exists {
			continue
		}
		if !checkDependency(dep.Command) {
			missing = append(missing, depName)
		}
	}

	return missing, len(missing) == 0
}

// isModuleAvailable returns true if all required dependencies are installed
func isModuleAvailable(moduleID string) bool {
	_, hasAll := getModuleDependencyStatus(moduleID)
	return hasAll
}

func buildSettingsView(state *appState) fyne.CanvasObject {
	settingsColor := utils.MustHex("#607D8B") // Blue Grey for settings

	backBtn := widget.NewButton("< BACK", func() {
		state.showMainMenu()
	})
	backBtn.Importance = widget.LowImportance

	topBar := ui.TintedBar(settingsColor, container.NewHBox(backBtn, layout.NewSpacer()))
	bottomBar := moduleFooter(settingsColor, layout.NewSpacer(), state.statsBar)

	tabs := container.NewAppTabs(
		container.NewTabItem("Updates", ui.NewFastVScroll(container.NewPadded(buildUpdatesTab(state)))),
		container.NewTabItem("Dependencies", ui.NewFastVScroll(container.NewPadded(buildDependenciesTab(state)))),
		container.NewTabItem("Benchmark", ui.NewFastVScroll(container.NewPadded(buildBenchmarkTab(state)))),
		container.NewTabItem("Preferences", ui.NewFastVScroll(container.NewPadded(buildPreferencesTab(state)))),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	return container.NewBorder(topBar, bottomBar, nil, nil, tabs)
}

func buildUpdatesTab(state *appState) fyne.CanvasObject {
	content := container.NewVBox()

	// Header
	header := widget.NewLabel("Updates")
	header.TextStyle = fyne.TextStyle{Bold: true}
	content.Add(header)

	desc := widget.NewLabel("Check for updates and manage app updates.")
	desc.Wrapping = fyne.TextWrapWord
	content.Add(desc)

	content.Add(widget.NewSeparator())

	// Current version
	versionLabel := widget.NewLabel(fmt.Sprintf("Current Version: %s", fullVersion()))
	versionLabel.TextStyle = fyne.TextStyle{Bold: true}
	content.Add(versionLabel)

	content.Add(layout.NewSpacer())

	// Check for updates button
	checkBtn := widget.NewButton("Check for Updates", func() {
		checkForUpdates(state)
	})
	checkBtn.Importance = widget.MediumImportance
	content.Add(checkBtn)

	content.Add(layout.NewSpacer())

	// Info text
	infoLabel := widget.NewLabel("Automatic updates will check for new versions\nwhen the app starts. Update checking happens\nin the background.")
	infoLabel.Wrapping = fyne.TextWrapWord
	infoLabel.TextStyle = fyne.TextStyle{Italic: true}
	content.Add(infoLabel)

	return content
}

func checkForUpdates(state *appState) {
	progress := dialog.NewProgressInfinite("Checking for Updates", "Connecting to update server...", state.window)
	progress.Show()

	go func() {
		// Simulated update check - in production, this would call an update server
		time.Sleep(2 * time.Second)

		// For now, just show a message
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			progress.Hide()
			dialog.ShowInformation("No Updates", "You are running the latest version.", state.window)
		}, false)
	}()
}

func buildDependenciesTab(state *appState) fyne.CanvasObject {
	content := container.NewVBox()

	// Header
	header := widget.NewLabel("System Dependencies")
	header.TextStyle = fyne.TextStyle{Bold: true}
	content.Add(header)

	desc := widget.NewLabel("Manage VideoTools dependencies. Some modules require specific tools to be installed.")
	desc.Wrapping = fyne.TextWrapWord
	content.Add(desc)

	content.Add(widget.NewSeparator())

	// Required dependencies first, then alphabetical.
	depNames := make([]string, 0, len(allDependencies))
	for depName, dep := range allDependencies {
		if !isDependencyAvailableForPlatform(dep) {
			continue
		}
		depNames = append(depNames, depName)
	}
	sort.Slice(depNames, func(i, j int) bool {
		di := allDependencies[depNames[i]]
		dj := allDependencies[depNames[j]]
		if di.Required != dj.Required {
			return di.Required && !dj.Required
		}
		return strings.ToLower(di.Name) < strings.ToLower(dj.Name)
	})

	// Check all dependencies
	for _, depName := range depNames {
		dep := allDependencies[depName]

		// Skip if no install command available for this platform
		cmds := getDependencyCommands(depName)
		if cmds.install == nil && dep.Command == "ffmpeg" && runtime.GOOS != "windows" {
			continue
		}

		isInstalled := checkDependency(dep.Command)

		nameLabel := widget.NewLabel(dep.Name)
		nameLabel.TextStyle = fyne.TextStyle{Bold: true}

		var statusIcon *widget.Icon
		var statusText string
		if isInstalled {
			statusIcon = widget.NewIcon(ui.GetIcon("check"))
			statusText = "Installed"
		} else {
			statusIcon = widget.NewIcon(ui.GetIcon("close"))
			statusText = "Not Installed"
		}
		statusLabel := widget.NewLabel(statusText)
		statusLabel.TextStyle = fyne.TextStyle{Italic: true}

		descLabel := widget.NewLabel(dep.Description)
		descLabel.TextStyle = fyne.TextStyle{Italic: true}
		descLabel.Wrapping = fyne.TextWrapWord

		installLabel := widget.NewLabel(dep.InstallCmd)
		installLabel.Wrapping = fyne.TextWrapWord

		var statusColor color.Color
		if isInstalled {
			statusColor = utils.MustHex("#4CAF50") // Green
		} else {
			statusColor = utils.MustHex("#F44336") // Red
		}

		statusBg := canvas.NewRectangle(statusColor)
		statusBg.CornerRadius = 3

		statusRow := container.NewHBox(statusIcon, statusBg, statusLabel)

		actions := container.NewHBox()
		cmds = getDependencyCommands(depName)

		if depName == "ffmpeg" && runtime.GOOS == "windows" {
			installBtn := widget.NewButton("Install", func() {
				state.installWindowsFFmpegFromUI(func() {
					dialog.ShowInformation("FFmpeg Ready", "FFmpeg is installed for this user and now available in the app.", state.window)
					state.showSettingsView()
				})
			})
			installBtn.Importance = widget.HighImportance
			if isInstalled {
				installBtn.Disable()
			}
			actions.Add(installBtn)
		}

		if cmds.install != nil {
			installBtn := widget.NewButton("Install", func() {
				runDependencyCommandWithProgress(state.window, fmt.Sprintf("Installing %s", dep.Name), dep.InstallCmd, cmds.install, func(out string, err error) {
					showCommandResult(state.window, fmt.Sprintf("%s Install", dep.Name), out, err)
					state.showSettingsView()
				})
			})
			installBtn.Importance = widget.HighImportance
			if isInstalled {
				installBtn.Disable()
			}
			actions.Add(installBtn)
		}

		// Windows: Special handling for dvdauthor/xorriso via WSL
		if runtime.GOOS == "windows" && (depName == "dvdauthor" || depName == "xorriso") {
			wslInstallBtn := widget.NewButton("Install via WSL", func() {
				installWSLWithDvdTools(state.window, func(success bool, message string) {
					if success {
						dialog.ShowInformation("Installation Complete", message, state.window)
					} else {
						dialog.ShowError(fmt.Errorf(message), state.window)
					}
					state.showSettingsView()
				})
			})
			wslInstallBtn.Importance = widget.HighImportance
			if isInstalled {
				wslInstallBtn.Disable()
			}
			actions.Add(wslInstallBtn)
		}

		if cmds.uninstall != nil {
			uninstallBtn := widget.NewButton("Uninstall", func() {
				dialog.ShowConfirm(fmt.Sprintf("Uninstall %s?", dep.Name), "This will attempt to remove the dependency using your package manager.", func(ok bool) {
					if !ok {
						return
					}
					runDependencyCommandWithProgress(state.window, fmt.Sprintf("Uninstalling %s", dep.Name), dep.InstallCmd, cmds.uninstall, func(out string, err error) {
						showCommandResult(state.window, fmt.Sprintf("%s Uninstall", dep.Name), out, err)
						state.showSettingsView()
					})
				}, state.window)
			})
			uninstallBtn.Importance = widget.LowImportance
			if !isInstalled {
				uninstallBtn.Disable()
			}
			actions.Add(uninstallBtn)
		}

		infoBox := container.NewVBox(
			container.NewHBox(nameLabel, layout.NewSpacer(), statusRow),
			descLabel,
		)
		if dep.Required {
			requiredLabel := widget.NewLabel("Core dependency")
			requiredLabel.TextStyle = fyne.TextStyle{Italic: true}
			infoBox.Add(requiredLabel)
		}

		if !isInstalled {
			installCmdLabel := widget.NewLabel("Install: " + installLabel.Text)
			installCmdLabel.Wrapping = fyne.TextWrapWord
			infoBox.Add(installCmdLabel)
		}

		if actions.Objects != nil && len(actions.Objects) > 0 {
			actionsContainer := container.NewHBox(actions.Objects...)
			infoBox.Add(actionsContainer)
		}

		// Check which modules need this dependency
		modulesNeeding := []string{}
		for modID, deps := range moduleDependencies {
			for _, d := range deps {
				if d == depName {
					// Find module name
					for _, m := range modulesList {
						if m.ID == modID {
							modulesNeeding = append(modulesNeeding, m.Label)
							break
						}
					}
					break
				}
			}
		}

		if len(modulesNeeding) > 0 {
			sort.Strings(modulesNeeding)
			neededLabel := widget.NewLabel("Required by: " + strings.Join(modulesNeeding, ", "))
			neededLabel.TextStyle = fyne.TextStyle{Italic: true}
			neededLabel.Wrapping = fyne.TextWrapWord
			infoBox.Add(neededLabel)
		}

		cardBg := canvas.NewRectangle(utils.MustHex("#171C2A"))
		cardBg.CornerRadius = 6
		card := container.NewPadded(container.NewMax(cardBg, infoBox))
		content.Add(card)
	}

	// Refresh button
	content.Add(widget.NewSeparator())
	refreshBtn := widget.NewButton("Refresh Status", func() {
		state.showSettingsView()
	})
	content.Add(refreshBtn)

	return content
}

func buildBenchmarkTab(state *appState) fyne.CanvasObject {
	content := container.NewVBox()

	// Header
	header := widget.NewLabel("Hardware Benchmark")
	header.TextStyle = fyne.TextStyle{Bold: true}
	content.Add(header)

	desc := widget.NewLabel("Test your system's video encoding performance to get optimal encoder recommendations.")
	desc.Wrapping = fyne.TextWrapWord
	content.Add(desc)

	content.Add(widget.NewSeparator())

	// Run benchmark button
	runBtn := widget.NewButton("Run Hardware Benchmark", func() {
		state.showBenchmark()
	})
	runBtn.Importance = widget.MediumImportance
	content.Add(container.NewCenter(runBtn))

	// Show recent results if available
	cfg, err := loadBenchmarkConfig()
	if err == nil && len(cfg.History) > 0 {
		content.Add(widget.NewSeparator())

		recentHeader := widget.NewLabel("Recent Benchmarks")
		recentHeader.TextStyle = fyne.TextStyle{Bold: true}
		content.Add(recentHeader)

		for _, run := range cfg.History[:min(3, len(cfg.History))] {
			timestamp := run.Timestamp.Format("Jan 2, 2006 at 3:04 PM")
			summary := fmt.Sprintf("%s - Recommended: %s (%s)",
				timestamp, run.RecommendedEncoder, run.RecommendedPreset)

			runLabel := widget.NewLabel(summary)
			runLabel.TextStyle = fyne.TextStyle{Italic: true}
			content.Add(runLabel)
		}
	}

	return content
}

func buildPreferencesTab(state *appState) fyne.CanvasObject {
	content := container.NewVBox()

	header := widget.NewLabel("Application Preferences")
	header.TextStyle = fyne.TextStyle{Bold: true}
	content.Add(header)

	content.Add(widget.NewSeparator())

	masterHeader := widget.NewLabel("Master Settings")
	masterHeader.TextStyle = fyne.TextStyle{Bold: true}
	content.Add(masterHeader)

	hwLabel := widget.NewLabel("Hardware Acceleration (Global)")
	hwLabel.TextStyle = fyne.TextStyle{Bold: true}

	hwStatus := widget.NewLabel("")
	hwStatus.TextStyle = fyne.TextStyle{Italic: true}
	hwStatus.Wrapping = fyne.TextWrapWord

	updateHwStatus := func() {
		detected := detectBestHardwareAccel()
		if detected == "" {
			detected = "none"
		}
		hwStatus.SetText(fmt.Sprintf("Detected: %s", detected))
	}

	hwSelect := widget.NewSelect([]string{"auto", "none", "nvenc", "qsv", "amf", "vaapi", "videotoolbox"}, func(selected string) {
		state.convert.HardwareAccel = selected
		state.persistConvertConfig()
		updateHwStatus()
	})
	hwSelect.SetSelected(state.convert.HardwareAccel)

	detectBtn := widget.NewButton("Detect", func() {
		best := detectBestHardwareAccel()
		if best == "" {
			best = "none"
		}
		hwSelect.SetSelected(best)
		state.convert.HardwareAccel = best
		state.persistConvertConfig()
		updateHwStatus()
	})
	detectBtn.Importance = widget.HighImportance

	autoBtn := widget.NewButton("Use Auto", func() {
		hwSelect.SetSelected("auto")
		state.convert.HardwareAccel = "auto"
		state.persistConvertConfig()
		updateHwStatus()
	})
	autoBtn.Importance = widget.MediumImportance

	updateHwStatus()

	content.Add(container.NewVBox(
		hwLabel,
		hwSelect,
		container.NewHBox(detectBtn, autoBtn),
		hwStatus,
	))

	content.Add(widget.NewSeparator())

	moduleHeader := widget.NewLabel("Module Visibility")
	moduleHeader.TextStyle = fyne.TextStyle{Bold: true}
	content.Add(moduleHeader)

	showUpscale := widget.NewCheck("Show Upscale module", func(checked bool) {
		state.convert.ShowUpscale = checked
		state.persistConvertConfig()
	})
	showUpscale.SetChecked(state.convert.ShowUpscale)

	showAuthor := widget.NewCheck("Show Author module", func(checked bool) {
		state.convert.ShowAuthor = checked
		state.persistConvertConfig()
	})
	showAuthor.SetChecked(state.convert.ShowAuthor)

	showRip := widget.NewCheck("Show Rip module", func(checked bool) {
		state.convert.ShowRip = checked
		state.persistConvertConfig()
	})
	showRip.SetChecked(state.convert.ShowRip)

	showBluRay := widget.NewCheck("Show Blu-ray module", func(checked bool) {
		state.convert.ShowBluRay = checked
		state.persistConvertConfig()
	})
	showBluRay.SetChecked(state.convert.ShowBluRay)

	visibilityHint := widget.NewLabel("Module visibility applies on the main menu.")
	visibilityHint.TextStyle = fyne.TextStyle{Italic: true}
	visibilityHint.Wrapping = fyne.TextWrapWord

	content.Add(container.NewVBox(
		showUpscale,
		showAuthor,
		showRip,
		showBluRay,
		visibilityHint,
	))

	content.Add(widget.NewSeparator())

	// Language selection (persisted UI language)
	langLabel := widget.NewLabel("Language")
	langOptions := map[string]string{
		"System":          "System",
		"English (Canada)": "en-CA",
		"French (Canada)":  "fr-CA",
		"Inuktitut":        "iu-Cans",
	}
	langSelect := widget.NewSelect([]string{"System", "English (Canada)", "French (Canada)", "Inuktitut"}, func(selected string) {
		value := langOptions[selected]
		if value == "" {
			value = "System"
		}
		state.convert.Language = value
		state.persistConvertConfig()
	})
	selectedLabel := "System"
	for label, value := range langOptions {
		if value == state.convert.Language {
			selectedLabel = label
			break
		}
	}
	langSelect.SetSelected(selectedLabel)
	content.Add(container.NewVBox(langLabel, langSelect))

	return content
}

func (s *appState) showSettingsView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "settings"
	s.maximizeWindow()
	s.setContent(buildSettingsView(s))
}
