package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/app/appcfg"
	"git.leaktechnologies.dev/stu/VideoTools/internal/app/modules/settings"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

type prefsConfig = settings.PrefsConfig
type Dependency = settings.Dependency
type dependencyCommand = settings.DependencyCommand
type dependencyCommandPair = settings.DependencyCommandPair

func loadPrefsConfig() (prefsConfig, error) {
	var cfg prefsConfig
	_, err := appcfg.LoadModuleJSON("prefs", &cfg)
	return cfg, err
}

func savePrefsConfig(cfg prefsConfig) error {
	return appcfg.SaveModuleJSON("prefs", cfg)
}

// getPipURL is the canonical bootstrap script for installing pip into a Python env.
const getPipURL = "https://bootstrap.pypa.io/get-pip.py"

func projectRoot() string {
	return settings.ProjectRoot()
}

func detectPkgManager() string {
	return settings.DetectPkgManager()
}

func pkgManagerInstall(pkg string) *dependencyCommand {
	return settings.PkgManagerInstall(pkg)
}

func pkgManagerUninstall(pkg string) *dependencyCommand {
	switch runtime.GOOS {
	case "linux":
		switch detectPkgManager() {
		case "apt-get":
			return &dependencyCommand{Command: "sudo", Args: []string{"apt-get", "remove", "-y", pkg}}
		case "dnf":
			return &dependencyCommand{Command: "sudo", Args: []string{"dnf", "remove", "-y", pkg}}
		case "pacman":
			return &dependencyCommand{Command: "sudo", Args: []string{"pacman", "-Rns", "--noconfirm", pkg}}
		case "zypper":
			return &dependencyCommand{Command: "sudo", Args: []string{"zypper", "remove", "-y", pkg}}
		}
	case "windows":
		if _, err := exec.LookPath("choco"); err == nil {
			return &dependencyCommand{Command: "powershell", Args: []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", fmt.Sprintf("choco uninstall -y %s", pkg)}}
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
			Install:   pkgManagerInstall("ffmpeg"),
			Uninstall: pkgManagerUninstall("ffmpeg"),
		}
	case "realesrgan-ncnn-vulkan":
		// Auto-download pre-built ncnn Vulkan binary from GitHub releases.
		// The UI install buttons call installRealESRGANFromUI directly, so this
		// path is only used for the legacy script-based flow on Linux.
		installScript := filepath.Join(root, "scripts", "install.sh")
		switch runtime.GOOS {
		case "linux":
			return dependencyCommandPair{
				Install: &dependencyCommand{Command: "bash", Args: []string{installScript, "--skip-ai=false", "--skip-dvd", "--skip-whisper"}},
			}
		}
	case "whisper":
		if runtime.GOOS == "windows" {
			// Use system Python if available; otherwise fall back to the app-local bundled Python.
			if pythonExe, ok := resolveWindowsPython(); ok {
				return dependencyCommandPair{
					Install:   &dependencyCommand{Command: pythonExe, Args: []string{"-m", "pip", "install", "openai-whisper"}},
					Uninstall: &dependencyCommand{Command: pythonExe, Args: []string{"-m", "pip", "uninstall", "-y", "openai-whisper"}},
				}
			}
			// No Python found — the UI install button will handle bootstrapping Python first.
			return dependencyCommandPair{}
		}
		return dependencyCommandPair{
			Install:   &dependencyCommand{Command: "python3", Args: []string{"-m", "pip", "install", "--user", "openai-whisper"}},
			Uninstall: &dependencyCommand{Command: "python3", Args: []string{"-m", "pip", "uninstall", "-y", "openai-whisper"}},
		}
	case "tesseract":
		switch runtime.GOOS {
		case "windows":
			if _, err := exec.LookPath("choco"); err == nil {
				return dependencyCommandPair{
					Install:   &dependencyCommand{Command: "powershell", Args: []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "choco install -y tesseract"}},
					Uninstall: &dependencyCommand{Command: "powershell", Args: []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "choco uninstall -y tesseract"}},
				}
			}
			return dependencyCommandPair{}
		default:
			pkg := "tesseract"
			if detectPkgManager() == "apt-get" {
				pkg = "tesseract-ocr"
			}
			return dependencyCommandPair{
				Install:   pkgManagerInstall(pkg),
				Uninstall: pkgManagerUninstall(pkg),
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

		cmd := utils.CreateCommand(ctx, depCmd.Command, depCmd.Args...)
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
		// Show the tail of the output so the result/error is always visible.
		output = "...\n" + output[len(output)-maxLen:]
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
	"author":    {"ffmpeg"},
	"rip":       {"ffmpeg"},
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
		Platforms:   []string{"windows", "linux"},
	},
	"realesrgan-ncnn-vulkan": {
		Name:        "Real-ESRGAN",
		Command:     "realesrgan-ncnn-vulkan",
		Required:    false,
		Description: "AI video upscaling (auto-downloaded from GitHub releases)",
		InstallCmd:  "Install from Settings — downloads pre-built ncnn Vulkan binary automatically",
		Platforms:   []string{"windows", "linux"},
	},
	"rife-ncnn-vulkan": {
		Name:        "RIFE",
		Command:     "rife-ncnn-vulkan",
		Required:    false,
		Description: "AI frame interpolation for smooth slow-motion and frame rate conversion (auto-downloaded from GitHub releases)",
		InstallCmd:  "Install from Settings — downloads pre-built ncnn Vulkan binary automatically",
		Platforms:   []string{"windows", "linux"},
	},
	"realcugan-ncnn-vulkan": {
		Name:        "Real-CUGAN",
		Command:     "realcugan-ncnn-vulkan",
		Required:    false,
		Description: "AI video upscaling optimized for animated content (auto-downloaded from GitHub releases)",
		InstallCmd:  "Install from Settings — downloads pre-built ncnn Vulkan binary automatically",
		Platforms:   []string{"windows", "linux"},
	},
	"whisper": {
		Name:        "Whisper",
		Command:     "whisper",
		Required:    false,
		Description: "AI subtitle generation",
		InstallCmd:  "pip3 install --user openai-whisper",
		Platforms:   []string{"windows", "linux"},
	},
	"tesseract": {
		Name:        "Tesseract OCR",
		Command:     "tesseract",
		Required:    false,
		Description: "OCR for image-based subtitles",
		InstallCmd:  "Install via package manager (tesseract-ocr)",
		Platforms:   []string{"windows", "linux"},
	},
}

func getFFmpegInstallCmd() string {
	switch runtime.GOOS {
	case "linux":
		return "Install from Settings (recommended) or run: ./scripts/linux/install.sh"
	case "windows":
		return "Install from Settings (recommended) or run: .\\scripts\\windows\\install.ps1"
	default:
		return "See ffmpeg.org for installation"
	}
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

// checkDependency checks if a command is available (system PATH or app-local bin).
// Bundled tools (realesrgan-ncnn-vulkan, realcugan-ncnn-vulkan, rife-ncnn-vulkan)
// are always considered installed since they're included in the release binary.
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

	// Bundled AI tools are always available in release builds
	switch command {
	case "realesrgan-ncnn-vulkan", "realcugan-ncnn-vulkan", "rife-ncnn-vulkan", "whisper", "tesseract":
		return true
	}

	// Check app-local bin directory for tools that may have been auto-downloaded.
	switch command {
	case "realesrgan-ncnn-vulkan", "rife-ncnn-vulkan", "realcugan-ncnn-vulkan":
		if _, err := os.Stat(appLocalToolPath(command)); err == nil {
			return true
		}
	}

	_, err := exec.LookPath(command)
	return err == nil
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
	resp, err := client.Get(settings.WindowsFFmpegZipURL)
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

// appLocalBinDir returns the per-user app-local bin directory for storing
// bundled tools (Real-ESRGAN, RIFE, etc.).
// Windows: %LOCALAPPDATA%\VideoTools\bin
// Linux:   $XDG_DATA_HOME/VideoTools/bin  (fallback ~/.local/share/VideoTools/bin)
func appLocalBinDir() string {
	switch runtime.GOOS {
	case "windows":
		base := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
		if base == "" {
			home, _ := os.UserHomeDir()
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, "VideoTools", "bin")
	default:
		base := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
		if base == "" {
			home, _ := os.UserHomeDir()
			base = filepath.Join(home, ".local", "share")
		}
		return filepath.Join(base, "VideoTools", "bin")
	}
}

func appLocalToolPath(name string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(appLocalBinDir(), name+".exe")
	}
	return filepath.Join(appLocalBinDir(), name)
}

// getLatestGitHubReleaseAssetURL queries the GitHub releases API for the
// latest release of owner/repo and returns the download URL of the asset
// whose name contains assetPattern.
func getLatestGitHubReleaseAssetURL(owner, repo, assetPattern string) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release struct {
		Assets []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("parse GitHub API response: %w", err)
	}

	for _, asset := range release.Assets {
		if strings.Contains(strings.ToLower(asset.Name), strings.ToLower(assetPattern)) {
			return asset.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("no release asset matching %q found in %s/%s", assetPattern, owner, repo)
}

// findGitHubAssetAcrossReleases searches the most recent releases for an asset
// whose name contains assetPattern (case-insensitive). Use this when the latest
// release might not have binary assets (e.g. projects that mix source and binary releases).
func findGitHubAssetAcrossReleases(owner, repo, assetPattern string, maxReleases int) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=%d", owner, repo, maxReleases)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var releases []struct {
		Assets []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return "", fmt.Errorf("parse GitHub API response: %w", err)
	}

	for _, release := range releases {
		for _, asset := range release.Assets {
			if strings.Contains(strings.ToLower(asset.Name), strings.ToLower(assetPattern)) {
				return asset.BrowserDownloadURL, nil
			}
		}
	}
	return "", fmt.Errorf("no release asset matching %q found in %s/%s (checked %d releases)", assetPattern, owner, repo, len(releases))
}

// downloadAndExtractBinary downloads a ZIP from url, extracts the file named
// executableName into destPath, and makes it executable on Unix.
func downloadAndExtractBinary(url, executableName, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}

	zipFile, err := os.CreateTemp("", "videotools-tool-*.zip")
	if err != nil {
		return fmt.Errorf("create temp zip: %w", err)
	}
	zipPath := zipFile.Name()
	_ = zipFile.Close()
	defer os.Remove(zipPath)

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: unexpected status %d", resp.StatusCode)
	}
	out, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("open temp zip for write: %w", err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		return fmt.Errorf("write zip: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("finalize zip: %w", err)
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer zr.Close()

	target := strings.ToLower(executableName)
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if strings.ToLower(filepath.Base(f.Name)) == target {
			src, err := f.Open()
			if err != nil {
				return fmt.Errorf("extract %s: %w", executableName, err)
			}
			dst, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
			if err != nil {
				_ = src.Close()
				return fmt.Errorf("create %s: %w", destPath, err)
			}
			if _, err := io.Copy(dst, src); err != nil {
				_ = dst.Close()
				_ = src.Close()
				return fmt.Errorf("write %s: %w", destPath, err)
			}
			_ = dst.Close()
			_ = src.Close()
			return nil
		}
	}
	return fmt.Errorf("%s not found inside zip", executableName)
}

// ensureAppLocalRealESRGAN downloads the Real-ESRGAN ncnn Vulkan binary if not
// already present. Works on both Windows and Linux.
func ensureAppLocalRealESRGAN() (string, error) {
	execName := "realesrgan-ncnn-vulkan"
	destPath := appLocalToolPath(execName)

	if _, err := os.Stat(destPath); err == nil {
		return destPath, nil // already installed
	}

	var assetPattern string
	switch runtime.GOOS {
	case "windows":
		assetPattern = "windows"
	case "linux":
		assetPattern = "ubuntu"
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	// xinntao/Real-ESRGAN mixes source and binary releases; use list endpoint to find
	// a recent release that actually contains binary assets.
	downloadURL, err := findGitHubAssetAcrossReleases("xinntao", "Real-ESRGAN", assetPattern, 10)
	if err != nil {
		return "", fmt.Errorf("find Real-ESRGAN release: %w", err)
	}

	exeFile := execName
	if runtime.GOOS == "windows" {
		exeFile = execName + ".exe"
	}

	if err := downloadAndExtractBinary(downloadURL, exeFile, destPath); err != nil {
		return "", fmt.Errorf("install Real-ESRGAN: %w", err)
	}
	return destPath, nil
}

// ensureAppLocalRIFE downloads the RIFE ncnn Vulkan binary if not already present.
// Works on both Windows and Linux.
func ensureAppLocalRIFE() (string, error) {
	execName := "rife-ncnn-vulkan"
	destPath := appLocalToolPath(execName)

	if _, err := os.Stat(destPath); err == nil {
		return destPath, nil // already installed
	}

	var assetPattern string
	switch runtime.GOOS {
	case "windows":
		assetPattern = "windows"
	case "linux":
		assetPattern = "ubuntu"
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	downloadURL, err := getLatestGitHubReleaseAssetURL("nihui", "rife-ncnn-vulkan", assetPattern)
	if err != nil {
		return "", fmt.Errorf("find RIFE release: %w", err)
	}

	exeFile := execName
	if runtime.GOOS == "windows" {
		exeFile = execName + ".exe"
	}

	if err := downloadAndExtractBinary(downloadURL, exeFile, destPath); err != nil {
		return "", fmt.Errorf("install RIFE: %w", err)
	}
	return destPath, nil
}

// ensureAppLocalRealCUGAN downloads the Real-CUGAN ncnn Vulkan binary if not
// already present. Works on both Windows and Linux.
func ensureAppLocalRealCUGAN() (string, error) {
	execName := "realcugan-ncnn-vulkan"
	destPath := appLocalToolPath(execName)

	if _, err := os.Stat(destPath); err == nil {
		return destPath, nil // already installed
	}

	var assetPattern string
	switch runtime.GOOS {
	case "windows":
		assetPattern = "windows"
	case "linux":
		assetPattern = "ubuntu"
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	downloadURL, err := getLatestGitHubReleaseAssetURL("nihui", "realcugan-ncnn-vulkan", assetPattern)
	if err != nil {
		return "", fmt.Errorf("find Real-CUGAN release: %w", err)
	}

	exeFile := execName
	if runtime.GOOS == "windows" {
		exeFile = execName + ".exe"
	}

	if err := downloadAndExtractBinary(downloadURL, exeFile, destPath); err != nil {
		return "", fmt.Errorf("install Real-CUGAN: %w", err)
	}
	return destPath, nil
}

func (s *appState) installRealESRGANFromUI(onSuccess func()) {
	progress := dialog.NewProgressInfinite("Installing Real-ESRGAN", "Downloading Real-ESRGAN ncnn Vulkan. This may take a moment.", s.window)
	progress.Show()

	go func() {
		toolPath, err := ensureAppLocalRealESRGAN()
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			progress.Hide()
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to install Real-ESRGAN: %w", err), s.window)
				return
			}
			logging.Info(logging.CatSystem, "Real-ESRGAN installed at %s", toolPath)
			if onSuccess != nil {
				onSuccess()
			}
		}, false)
	}()
}

func (s *appState) installRealCUGANFromUI(onSuccess func()) {
	progress := dialog.NewProgressInfinite("Installing Real-CUGAN", "Downloading Real-CUGAN ncnn Vulkan. This may take a moment.", s.window)
	progress.Show()

	go func() {
		toolPath, err := ensureAppLocalRealCUGAN()
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			progress.Hide()
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to install Real-CUGAN: %w", err), s.window)
				return
			}
			logging.Info(logging.CatSystem, "Real-CUGAN installed at %s", toolPath)
			if onSuccess != nil {
				onSuccess()
			}
		}, false)
	}()
}

func (s *appState) installRIFEFromUI(onSuccess func()) {
	progress := dialog.NewProgressInfinite("Installing RIFE", "Downloading RIFE ncnn Vulkan. This may take a moment.", s.window)
	progress.Show()

	go func() {
		toolPath, err := ensureAppLocalRIFE()
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			progress.Hide()
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to install RIFE: %w", err), s.window)
				return
			}
			logging.Info(logging.CatSystem, "RIFE installed at %s", toolPath)
			if onSuccess != nil {
				onSuccess()
			}
		}, false)
	}()
}

// windowsEmbeddablePythonDir returns the directory where the bundled Python is installed.
func windowsEmbeddablePythonDir() string {
	return filepath.Join(appLocalBinDir(), "python")
}

// windowsEmbeddablePythonExe returns the path to the bundled python.exe.
func windowsEmbeddablePythonExe() string {
	return filepath.Join(windowsEmbeddablePythonDir(), "python.exe")
}

// ensureWindowsEmbeddablePython downloads and sets up an embeddable Python
// distribution in the app-local bin directory if not already present.
// It also installs pip into the embeddable distribution so that packages
// like openai-whisper can be installed.
func ensureWindowsEmbeddablePython() (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("only supported on Windows")
	}

	pythonExe := windowsEmbeddablePythonExe()
	if _, err := os.Stat(pythonExe); err == nil {
		return pythonExe, nil // already installed
	}

	pythonDir := windowsEmbeddablePythonDir()
	if err := os.MkdirAll(pythonDir, 0o755); err != nil {
		return "", fmt.Errorf("create python dir: %w", err)
	}

	// Step 1: Download and extract the embeddable Python zip.
	zipFile, err := os.CreateTemp("", "videotools-python-*.zip")
	if err != nil {
		return "", fmt.Errorf("create temp zip: %w", err)
	}
	zipPath := zipFile.Name()
	_ = zipFile.Close()
	defer os.Remove(zipPath)

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(settings.WindowsPythonURL)
	if err != nil {
		return "", fmt.Errorf("download Python: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download Python: unexpected status %d", resp.StatusCode)
	}
	out, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("open temp zip for write: %w", err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		return "", fmt.Errorf("write Python zip: %w", err)
	}
	if err := out.Close(); err != nil {
		return "", fmt.Errorf("finalize Python zip: %w", err)
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("open Python zip: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		destPath := filepath.Join(pythonDir, f.Name)
		src, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("extract %s: %w", f.Name, err)
		}
		dst, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			_ = src.Close()
			return "", fmt.Errorf("create %s: %w", destPath, err)
		}
		if _, err := io.Copy(dst, src); err != nil {
			_ = dst.Close()
			_ = src.Close()
			return "", fmt.Errorf("write %s: %w", destPath, err)
		}
		_ = dst.Close()
		_ = src.Close()
	}
	zr.Close()

	// Step 2: Patch the ._pth file to enable site-packages (required for pip).
	// The embeddable distribution has a file like python312._pth that contains
	// "#import site" — we need to uncomment it.
	entries, err := os.ReadDir(pythonDir)
	if err != nil {
		return "", fmt.Errorf("read python dir: %w", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "._pth") {
			pthPath := filepath.Join(pythonDir, entry.Name())
			data, err := os.ReadFile(pthPath)
			if err != nil {
				continue
			}
			patched := strings.ReplaceAll(string(data), "#import site", "import site")
			if err := os.WriteFile(pthPath, []byte(patched), 0o644); err != nil {
				return "", fmt.Errorf("patch %s: %w", entry.Name(), err)
			}
		}
	}

	// Step 3: Download get-pip.py and run it with the bundled Python.
	getPipFile, err := os.CreateTemp("", "get-pip-*.py")
	if err != nil {
		return "", fmt.Errorf("create get-pip temp: %w", err)
	}
	getPipPath := getPipFile.Name()
	_ = getPipFile.Close()
	defer os.Remove(getPipPath)

	pipResp, err := client.Get(getPipURL)
	if err != nil {
		return "", fmt.Errorf("download get-pip.py: %w", err)
	}
	defer pipResp.Body.Close()
	if pipResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download get-pip.py: unexpected status %d", pipResp.StatusCode)
	}
	pipOut, err := os.Create(getPipPath)
	if err != nil {
		return "", fmt.Errorf("open get-pip temp for write: %w", err)
	}
	if _, err := io.Copy(pipOut, pipResp.Body); err != nil {
		_ = pipOut.Close()
		return "", fmt.Errorf("write get-pip.py: %w", err)
	}
	if err := pipOut.Close(); err != nil {
		return "", fmt.Errorf("finalize get-pip.py: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, pythonExe, getPipPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("install pip: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	return pythonExe, nil
}

// resolveWindowsPython returns the path to a usable Python executable on Windows.
// Checks system PATH first, then falls back to the app-local bundled Python.
func resolveWindowsPython() (string, bool) {
	for _, name := range []string{"python", "py", "python3"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, true
		}
	}
	bundled := windowsEmbeddablePythonExe()
	if _, err := os.Stat(bundled); err == nil {
		return bundled, true
	}
	return "", false
}

func (s *appState) installWindowsPythonFromUI(onSuccess func(pythonExe string)) {
	progress := dialog.NewProgressInfinite("Installing Python", "Downloading Python 3.12 embeddable. This may take a moment.", s.window)
	progress.Show()

	go func() {
		pythonExe, err := ensureWindowsEmbeddablePython()
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			progress.Hide()
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to install Python: %w", err), s.window)
				return
			}
			if onSuccess != nil {
				onSuccess(pythonExe)
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

	t := i18n.T()
	message := widget.NewLabel(t.SettingsFFmpegMissing)
	message.Wrapping = fyne.TextWrapWord

	var prompt dialog.Dialog
	installBtn := widget.NewButton(t.SettingsInstallFFmpeg, func() {
		s.installWindowsFFmpegFromUI(func() {
			if prompt != nil {
				prompt.Hide()
			}
			s.showMainMenu()
			dialog.ShowInformation("FFmpeg Ready", "FFmpeg is installed for this user. Video modules are now available.", s.window)
		})
	})
	installBtn.Importance = widget.HighImportance

	settingsBtn := widget.NewButton(t.SettingsOpenSettings, func() {
		if prompt != nil {
			prompt.Hide()
		}
		s.showSettingsView()
	})
	continueBtn := widget.NewButton(t.SettingsContinueLimited, func() {
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

const (
	forgejoTagsAPI        = "https://git.leaktechnologies.dev/api/v1/repos/leak_technologies/VideoTools/tags?limit=1"
	forgejoReleasesTagAPI = "https://git.leaktechnologies.dev/api/v1/repos/leak_technologies/VideoTools/releases/tags/"
	forgejoReleasesPage   = "https://git.leaktechnologies.dev/leak_technologies/VideoTools/releases"
)

type updateInfo struct {
	latestTag    string
	tagCommitSHA string    // SHA of the commit the latest release tag points to
	releaseDate  time.Time // Release date
}

const (
	// Update check intervals
	UpdateCheckHourly     = time.Hour
	UpdateCheck2Hours     = 2 * time.Hour
	UpdateCheck3Hours     = 3 * time.Hour
	UpdateCheck4Hours     = 4 * time.Hour
	UpdateCheck6Hours     = 6 * time.Hour
	UpdateCheck12Hours    = 12 * time.Hour
	UpdateCheckDaily      = 24 * time.Hour
	UpdateCheckSemiWeekly = 24 * 3 * time.Hour // Every 3 days
	UpdateCheckWeekly     = 24 * 7 * time.Hour
	UpdateCheckBiWeekly   = 24 * 14 * time.Hour // Every 2 weeks
	UpdateCheckMonthly    = 24 * 30 * time.Hour // Approx monthly
	UpdateCheckBiMonthly  = 24 * 60 * time.Hour // Approx bi-monthly
)

func checkForUpdates(state *appState) {
	progress := dialog.NewProgressInfinite("Checking for Updates", "Connecting to update server...", state.window)
	progress.Show()

	go func() {
		info, err := fetchUpdateInfo()
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			progress.Hide()
			if err != nil {
				dialog.ShowError(fmt.Errorf("could not reach update server: %w", err), state.window)
				return
			}

			// Different version tag available
			if info.latestTag != appVersion {
				msg := fmt.Sprintf(
					"A new version is available: %s\n\nYou are currently on %s.\n\nClick 'Install Update' to download and restart automatically.",
					info.latestTag, appVersion,
				)
				lbl := widget.NewLabel(msg)
				lbl.Wrapping = fyne.TextWrapWord
				d := dialog.NewCustom("Update Available", "Close", lbl, state.window)
				latestTag := info.latestTag
				installBtn := widget.NewButton("Install Update", func() {
					d.Hide()
					queueBusy := state.jobQueue != nil && state.jobQueue.IsRunning()
					if state.convertBusy || queueBusy {
						t := i18n.T()
						dialog.ShowInformation(t.DialogUpdateBlocked, t.StatusUpdateBlockedByJob, state.window)
						return
					}
					applyUpdate(state, latestTag)
				})
				installBtn.Importance = widget.HighImportance
				d.SetButtons([]fyne.CanvasObject{
					installBtn,
					widget.NewButton("Open Releases Page", func() {
						d.Hide()
						_ = openURL(forgejoReleasesPage)
					}),
					widget.NewButton("Close", func() { d.Hide() }),
				})
				d.Show()
				return
			}

			// Same version — check whether the running binary matches the release tag's commit.
			currentShort := buildCommit
			if len(currentShort) > 7 {
				currentShort = currentShort[:7]
			}
			tagShort := info.tagCommitSHA
			if len(tagShort) > 7 {
				tagShort = tagShort[:7]
			}
			patchesAvailable := currentShort != "" && currentShort != "dev" &&
				tagShort != "" && currentShort != tagShort

			if patchesAvailable {
				if currentShort == "" {
					currentShort = "unknown"
				}
				newShort := tagShort
				msg := fmt.Sprintf(
					"You are on %s (the latest release tag).\n\nHash mismatch detected:\n  Current: %s\n  Latest:  %s\n\nClick 'Install Patches' to download the latest build and restart automatically.",
					appVersion, currentShort, newShort,
				)
				lbl := widget.NewLabel(msg)
				lbl.Wrapping = fyne.TextWrapWord
				d := dialog.NewCustom("Patches Available", "Close", lbl, state.window)
				currentTag := appVersion
				installBtn := widget.NewButton("Install Patches", func() {
					d.Hide()
					queueBusy := state.jobQueue != nil && state.jobQueue.IsRunning()
					if state.convertBusy || queueBusy {
						t := i18n.T()
						dialog.ShowInformation(t.DialogUpdateBlocked, t.StatusUpdateBlockedByJob, state.window)
						return
					}
					applyUpdate(state, currentTag)
				})
				installBtn.Importance = widget.HighImportance
				d.SetButtons([]fyne.CanvasObject{
					installBtn,
					widget.NewButton("Close", func() { d.Hide() }),
				})
				d.Show()
				return
			}

			dialog.ShowInformation("Up to Date", fmt.Sprintf("You are running the latest version (%s).", appVersion), state.window)
		}, false)
	}()
}

// applyUpdateStatusToUI renders a previously-fetched update result into the
// status icon/label without hitting the network again.
func applyUpdateStatusToUI(state *appState, statusIcon *widget.Icon, statusLabel *widget.Label, onAvailable func(tag string)) {
	t := i18n.T()
	currentShort := buildCommit
	if len(currentShort) > 7 {
		currentShort = currentShort[:7]
	}
	age := formatRelativeTime(state.updateLastChecked)
	if state.updateCachedTag != "" && state.updateCachedTag != appVersion {
		statusIcon.SetResource(recoloredSVG{ui.GetIcon("change_circle"), "#FFAB40"})
		statusIcon.Show()
		statusLabel.SetText(fmt.Sprintf(t.UpdateAvailableFmt, state.updateCachedTag, age))
		statusLabel.TextStyle = fyne.TextStyle{Bold: true}
		onAvailable(state.updateCachedTag)
		return
	}
	if state.updateCachedPatch {
		statusIcon.SetResource(recoloredSVG{ui.GetIcon("build_circle"), "#FFAB40"})
		statusIcon.Show()
		statusLabel.SetText(fmt.Sprintf(t.UpdateNewBuildAvailable, currentShort, age))
		statusLabel.TextStyle = fyne.TextStyle{Bold: true}
		onAvailable(appVersion)
		return
	}
	statusIcon.SetResource(recoloredSVG{ui.GetIcon("check_circle"), "#4CE870"})
	statusIcon.Show()
	statusLabel.SetText(fmt.Sprintf(t.UpdateUpToDateFmt, appVersion, age))
	statusLabel.TextStyle = fyne.TextStyle{Italic: true}
	onAvailable("")
}

// checkForUpdatesWithStatus checks for updates and drives the icon+label status row.
// onAvailable is called with the installable tag when an update/patch is found,
// or with "" when the build is already up to date or on error.
func checkForUpdatesWithStatus(state *appState, statusIcon *widget.Icon, statusLabel *widget.Label, onAvailable func(tag string)) {
	t := i18n.T()
	statusIcon.Hide()
	statusLabel.SetText(t.UpdateChecking)
	statusLabel.TextStyle = fyne.TextStyle{Italic: true}

	go func() {
		info, err := fetchUpdateInfo()
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			t := i18n.T()
			if err != nil {
				logging.Info(logging.CatSystem, "Update check failed: %v", err)
				statusLabel.SetText(t.UpdateError)
				statusLabel.TextStyle = fyne.TextStyle{Italic: true}
				onAvailable("")
				return
			}

			currentShort := buildCommit
			if len(currentShort) > 7 {
				currentShort = currentShort[:7]
			}
			tagShort := info.tagCommitSHA
			if len(tagShort) > 7 {
				tagShort = tagShort[:7]
			}
			patchesAvailable := currentShort != "" && currentShort != "dev" &&
				tagShort != "" && currentShort != tagShort

			// Write result to cache so subsequent settings visits reuse it.
			state.updateLastChecked = time.Now()
			state.updateCachedTag = info.latestTag
			state.updateCachedPatch = info.latestTag == appVersion && patchesAvailable

			// Different version tag — full update available
			if info.latestTag != appVersion {
				age := formatRelativeTime(info.releaseDate)
				statusIcon.SetResource(recoloredSVG{ui.GetIcon("change_circle"), "#FFAB40"})
				statusIcon.Show()
				statusLabel.SetText(fmt.Sprintf(t.UpdateAvailableFmt, info.latestTag, age))
				statusLabel.TextStyle = fyne.TextStyle{Bold: true}
				onAvailable(info.latestTag)
				return
			}

			// Same tag but newer build commit — patches available
			if patchesAvailable {
				age := formatRelativeTime(info.releaseDate)
				statusIcon.SetResource(recoloredSVG{ui.GetIcon("build_circle"), "#FFAB40"})
				statusIcon.Show()
				statusLabel.SetText(fmt.Sprintf(t.UpdateNewBuildAvailable, tagShort, age))
				statusLabel.TextStyle = fyne.TextStyle{Bold: true}
				onAvailable(appVersion) // same tag, re-download latest build
				return
			}

			age := formatRelativeTime(info.releaseDate)
			statusIcon.SetResource(recoloredSVG{ui.GetIcon("check_circle"), "#4CE870"})
			statusIcon.Show()
			statusLabel.SetText(fmt.Sprintf(t.UpdateUpToDateFmt, info.latestTag, age))
			statusLabel.TextStyle = fyne.TextStyle{Italic: true}
			onAvailable("")
		}, false)
	}()
}

func fetchUpdateInfo() (updateInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	// Fetch latest tag
	resp, err := client.Get(forgejoTagsAPI)
	if err != nil {
		return updateInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return updateInfo{}, fmt.Errorf("tags API returned %s", resp.Status)
	}
	var tags []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return updateInfo{}, fmt.Errorf("parse tags response: %w", err)
	}
	if len(tags) == 0 || tags[0].Name == "" {
		return updateInfo{}, fmt.Errorf("no tags found in repository")
	}
	tagName := tags[0].Name

	// Fetch the release to get target_commitish — the actual commit the CI
	// built from. The tag's commit SHA is stale (set on first tag creation and
	// never moved), but target_commitish is PATCHed on every nightly run.
	rResp, rErr := client.Get(forgejoReleasesTagAPI + tagName)
	if rErr != nil {
		return updateInfo{}, fmt.Errorf("releases API: %w", rErr)
	}
	defer rResp.Body.Close()
	if rResp.StatusCode != http.StatusOK {
		return updateInfo{}, fmt.Errorf("releases API returned %s", rResp.Status)
	}
	var release struct {
		TargetCommitish string    `json:"target_commitish"`
		CreatedAt       time.Time `json:"created_at"`
		Assets          []struct {
			CreatedAt time.Time `json:"created_at"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(rResp.Body).Decode(&release); err != nil {
		return updateInfo{}, fmt.Errorf("parse release response: %w", err)
	}

	// Use the newest asset upload time so the date reflects the latest build,
	// not the original release creation (which never changes on nightly rebuilds).
	releaseDate := release.CreatedAt
	for _, a := range release.Assets {
		if a.CreatedAt.After(releaseDate) {
			releaseDate = a.CreatedAt
		}
	}

	return updateInfo{
		latestTag:    tagName,
		tagCommitSHA: release.TargetCommitish,
		releaseDate:  releaseDate,
	}, nil
}

// fetchReleaseAssetURL returns the download URL for the platform-appropriate asset in a release.
// isZip is true when the asset is a zip archive that must be extracted; false for direct binary (AppImage).
func fetchReleaseAssetURL(tag string) (downloadURL string, isZip bool, err error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(forgejoReleasesTagAPI + tag)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("releases API returned %s", resp.Status)
	}
	var release struct {
		Assets []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", false, fmt.Errorf("parse release: %w", err)
	}

	// Ordered list of suffixes to try, most preferred first.
	var candidates []string
	switch runtime.GOOS {
	case "windows":
		candidates = []string{"_windows.zip"}
	case "linux":
		if os.Getenv("APPIMAGE") != "" {
			// Running as AppImage; prefer the AppImage asset, fall back to zip.
			candidates = []string{"_linux.AppImage", "_linux.zip"}
		} else {
			candidates = []string{"_linux.zip"}
		}
	default:
		return "", false, fmt.Errorf("auto-update not supported on %s", runtime.GOOS)
	}

	for _, suffix := range candidates {
		for _, a := range release.Assets {
			if strings.HasSuffix(a.Name, suffix) {
				return a.BrowserDownloadURL, strings.HasSuffix(a.Name, ".zip"), nil
			}
		}
	}
	return "", false, fmt.Errorf("no compatible asset found in release %s", tag)
}

// downloadToTemp downloads url into a temporary file and returns its path.
// progressFn is called periodically with (bytesDownloaded, totalBytes); totalBytes
// is -1 when the server does not send Content-Length. Pass nil to skip tracking.
func downloadToTemp(url string, progressFn func(downloaded, total int64)) (string, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned %s", resp.Status)
	}
	out, err := os.CreateTemp("", "vt_update_*")
	if err != nil {
		return "", err
	}
	defer out.Close()

	if progressFn == nil {
		if _, err := io.Copy(out, resp.Body); err != nil {
			os.Remove(out.Name())
			return "", err
		}
		return out.Name(), nil
	}

	total := resp.ContentLength // -1 if unknown
	var downloaded int64
	buf := make([]byte, 32*1024)
	lastReport := time.Now()
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				os.Remove(out.Name())
				return "", werr
			}
			downloaded += int64(n)
			if time.Since(lastReport) >= 200*time.Millisecond {
				progressFn(downloaded, total)
				lastReport = time.Now()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			os.Remove(out.Name())
			return "", readErr
		}
	}
	progressFn(downloaded, total) // final update
	return out.Name(), nil
}

// extractFileFromZip extracts a single named entry from a zip archive to destPath.
func extractFileFromZip(zipPath, entryName, destPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name == entryName {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()
			out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			defer out.Close()
			_, err = io.Copy(out, rc)
			return err
		}
	}
	return fmt.Errorf("entry %q not found in zip", entryName)
}

// autoCheckKeyToInterval maps a canonical AutoCheckFrequency key to a duration.
// Returns (0, false) for "disabled" or unknown keys.
func autoCheckKeyToInterval(key string) (time.Duration, bool) {
	m := map[string]time.Duration{
		"every_hour":  UpdateCheckHourly,
		"every_2h":    UpdateCheck2Hours,
		"every_3h":    UpdateCheck3Hours,
		"every_4h":    UpdateCheck4Hours,
		"every_6h":    UpdateCheck6Hours,
		"every_12h":   UpdateCheck12Hours,
		"daily":       UpdateCheckDaily,
		"semi_weekly": UpdateCheckSemiWeekly,
		"weekly":      UpdateCheckWeekly,
		"bi_weekly":   UpdateCheckBiWeekly,
		"monthly":     UpdateCheckMonthly,
		"bi_monthly":  UpdateCheckBiMonthly,
	}
	d, ok := m[key]
	return d, ok
}

// startAutoUpdateChecker launches a background goroutine that checks for
// updates on the interval stored in state.prefs.AutoCheckFrequency.
// The timer is relative to app launch — "every hour" means 1 h after the app
// opens, then every 1 h thereafter. No top-of-clock logic is used.
// If prefs.AutoCheckFrequency is "disabled" or empty, nothing runs.
func startAutoUpdateChecker(state *appState) {
	interval, ok := autoCheckKeyToInterval(state.prefs.AutoCheckFrequency)
	if !ok {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			// Re-read preference — user may have changed it while the app is open.
			newInterval, still := autoCheckKeyToInterval(state.prefs.AutoCheckFrequency)
			if !still {
				return // disabled
			}
			if newInterval != interval {
				// Interval changed — restart with new value.
				ticker.Reset(newInterval)
				interval = newInterval
			}
			info, err := fetchUpdateInfo()
			if err != nil {
				logging.Debug(logging.CatSystem, "auto update check failed: %v", err)
				continue
			}
			currentShort := buildCommit
			if len(currentShort) > 7 {
				currentShort = currentShort[:7]
			}
			tagShort := info.tagCommitSHA
			if len(tagShort) > 7 {
				tagShort = tagShort[:7]
			}
			hasUpdate := info.latestTag != appVersion
			hasPatches := !hasUpdate && currentShort != "" && currentShort != "dev" &&
				tagShort != "" && currentShort != tagShort
			if hasUpdate || hasPatches {
				fyne.CurrentApp().SendNotification(&fyne.Notification{
					Title:   "VideoTools Update Available",
					Content: fmt.Sprintf("Version %s is available. Open Settings → Updates to install.", info.latestTag),
				})
			}
		}
	}()
}

// applyUpdate downloads the release asset for tag and replaces the running binary, then restarts.
func applyUpdate(state *appState, tag string) {
	progress := dialog.NewProgress("Installing Update", "Connecting...", state.window)
	progress.Show()

	// updateLabel switches the dialog to show bytes downloaded vs total.
	updateLabel := func(downloaded, total int64) {
		var msg string
		if total > 0 {
			msg = fmt.Sprintf("Downloading... %.1f / %.1f MB", float64(downloaded)/1e6, float64(total)/1e6)
		} else {
			msg = fmt.Sprintf("Downloading... %.1f MB", float64(downloaded)/1e6)
		}
		frac := 0.0
		if total > 0 {
			frac = float64(downloaded) / float64(total)
		}
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			progress.SetValue(frac)
			_ = msg // label update not available on dialog.Progress; value conveys progress
		}, false)
	}

	go func() {
		downloadURL, isZip, err := fetchReleaseAssetURL(tag)
		if err != nil {
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				progress.Hide()
				dialog.ShowError(fmt.Errorf("could not find update asset: %w", err), state.window)
			}, false)
			return
		}

		// Resolve the path of the binary we need to replace.
		exePath, err := os.Executable()
		if err != nil {
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				progress.Hide()
				dialog.ShowError(fmt.Errorf("could not determine executable path: %w", err), state.window)
			}, false)
			return
		}
		if resolved, err2 := filepath.EvalSymlinks(exePath); err2 == nil {
			exePath = resolved
		}
		// On Linux, if running as an AppImage, replace the AppImage file directly.
		if runtime.GOOS == "linux" {
			if ai := os.Getenv("APPIMAGE"); ai != "" {
				exePath = ai
			}
		}

		// Download the asset.
		var newBinaryPath string
		if isZip {
			tmpZip, err := downloadToTemp(downloadURL, updateLabel)
			if err != nil {
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					progress.Hide()
					dialog.ShowError(fmt.Errorf("download failed: %w", err), state.window)
				}, false)
				return
			}
			defer os.Remove(tmpZip)

			entryName := "VideoTools"
			ext := ""
			if runtime.GOOS == "windows" {
				entryName = "VideoTools.exe"
				ext = ".exe"
			}
			newBinaryPath = filepath.Join(os.TempDir(), "VideoTools_update"+ext)
			if err := extractFileFromZip(tmpZip, entryName, newBinaryPath); err != nil {
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					progress.Hide()
					dialog.ShowError(fmt.Errorf("extract failed: %w", err), state.window)
				}, false)
				return
			}
		} else {
			// Direct asset (AppImage).
			tmpPath, err := downloadToTemp(downloadURL, updateLabel)
			if err != nil {
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					progress.Hide()
					dialog.ShowError(fmt.Errorf("download failed: %w", err), state.window)
				}, false)
				return
			}
			newBinaryPath = tmpPath
		}

		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			progress.SetValue(1)
			progress.Hide()
		}, false)

		if err := performRestart(newBinaryPath, exePath); err != nil {
			os.Remove(newBinaryPath)
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				dialog.ShowError(fmt.Errorf("update failed: %w", err), state.window)
			}, false)
		}
	}()
}

func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	now := time.Now()
	d := now.Sub(t)

	if d < time.Hour {
		minutes := int(d.Minutes())
		if minutes <= 1 {
			return "just now"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	if d < 48*time.Hour {
		return "yesterday"
	}
	days := int(d.Hours() / 24)
	if days < 7 {
		return fmt.Sprintf("%d days ago", days)
	}
	if days < 14 {
		return "1 week ago"
	}
	if days < 30 {
		weeks := days / 7
		return fmt.Sprintf("%d weeks ago", weeks)
	}
	if days < 60 {
		return "1 month ago"
	}
	if days < 90 {
		return "2 months ago"
	}
	months := days / 30
	return fmt.Sprintf("%d months ago", months)
}

type benchmarkAdapter struct {
	s *appState
}

func (a *benchmarkAdapter) Window() fyne.Window {
	return a.s.window
}

func (a *benchmarkAdapter) ShowBenchmark() {
	a.s.showBenchmark()
}

type preferencesAdapter struct {
	s *appState
}

func (a *preferencesAdapter) Window() fyne.Window {
	return a.s.window
}

func (a *preferencesAdapter) ShowSettingsView() {
	a.s.showSettingsView()
}

func (a *preferencesAdapter) FullVersion() string {
	return fullVersion()
}

func (a *preferencesAdapter) BuildCommit() string {
	return buildCommit
}

func (a *preferencesAdapter) UpdateLastChecked() time.Time {
	return a.s.updateLastChecked
}

func (a *preferencesAdapter) ApplyUpdate(tag string) {
	applyUpdate(a.s, tag)
}

func (a *preferencesAdapter) CheckForUpdatesWithStatus(statusIcon *widget.Icon, statusLabel *widget.Label, onAvailable func(tag string)) {
	checkForUpdatesWithStatus(a.s, statusIcon, statusLabel, onAvailable)
}

func (a *preferencesAdapter) ApplyUpdateStatusToUI(statusIcon *widget.Icon, statusLabel *widget.Label, onAvailable func(tag string)) {
	applyUpdateStatusToUI(a.s, statusIcon, statusLabel, onAvailable)
}

func (a *preferencesAdapter) DetectBestHardwareAccel() string {
	return detectBestHardwareAccel()
}

func (a *preferencesAdapter) DetectHardwareAccelStatus() (best string, status string) {
	return detectHardwareAccelStatus()
}

func (a *preferencesAdapter) PersistConvertConfig() {
	a.s.persistConvertConfig()
}

func (a *preferencesAdapter) ConvertHardwareAccel() string {
	return a.s.convert.HardwareAccel
}

func (a *preferencesAdapter) SetConvertHardwareAccel(accel string) {
	a.s.convert.HardwareAccel = accel
}

func (a *preferencesAdapter) ConvertShowUpscale() bool {
	return a.s.convert.ShowUpscale
}

func (a *preferencesAdapter) SetConvertShowUpscale(show bool) {
	a.s.convert.ShowUpscale = show
}

func (a *preferencesAdapter) ConvertShowDisc() bool {
	return a.s.convert.ShowDisc
}

func (a *preferencesAdapter) SetConvertShowDisc(show bool) {
	a.s.convert.ShowDisc = show
}

func (a *preferencesAdapter) PersistLocale(code string, script i18n.ScriptVariant) {
	persistLocale(code, script)
}

func (a *preferencesAdapter) SavePrefsConfig() error {
	err := savePrefsConfig(a.s.prefs)
	if err != nil {
		logging.Error(logging.CatSystem, "SavePrefsConfig failed: %v", err)
	}
	return err
}

func (a *preferencesAdapter) PrefsConfig() prefsConfig {
	return a.s.prefs
}

func (a *preferencesAdapter) SetShowTooltips(enabled bool) {
	a.s.prefs.ShowTooltips = enabled
	ui.ShowTooltips = enabled
	if err := savePrefsConfig(a.s.prefs); err != nil {
		logging.Error(logging.CatSystem, "SetShowTooltips save failed: %v", err)
	}
}

func (a *preferencesAdapter) HWDecodeEnabled() bool {
	return a.s.prefs.HWDecodeEnabled
}

func (a *preferencesAdapter) ShowPlayer() func() {
	return func() { a.s.showPlayerView() }
}

func (a *preferencesAdapter) SetHWDecodeEnabled(enabled bool) {
	setHWDecodeEnabled(enabled)
	a.s.prefs.HWDecodeEnabled = enabled
	if err := savePrefsConfig(a.s.prefs); err != nil {
		logging.Error(logging.CatSystem, "SetHWDecodeEnabled save failed: %v", err)
	}
}

func (a *preferencesAdapter) DefaultOutputDir() string {
	return a.s.defaultOutputDir
}

func (a *preferencesAdapter) SetDefaultOutputDir(dir string) {
	a.s.defaultOutputDir = dir
	a.s.prefs.DefaultOutputDir = dir
	savePrefsConfig(a.s.prefs)
}

type dependencyAdapter struct {
	s *appState
}

func (a *dependencyAdapter) Window() fyne.Window {
	return a.s.window
}

func (a *dependencyAdapter) ShowSettingsView() {
	a.s.showSettingsView()
}

func (a *dependencyAdapter) InstallWindowsFFmpeg(onDone func()) {
	a.s.installWindowsFFmpegFromUI(onDone)
}

func (a *dependencyAdapter) InstallRealESRGAN(onDone func()) {
	a.s.installRealESRGANFromUI(onDone)
}

func (a *dependencyAdapter) InstallRealCUGAN(onDone func()) {
	a.s.installRealCUGANFromUI(onDone)
}

func (a *dependencyAdapter) InstallRIFE(onDone func()) {
	a.s.installRIFEFromUI(onDone)
}

func (a *dependencyAdapter) InstallWindowsPython(onDone func(pythonExe string)) {
	a.s.installWindowsPythonFromUI(onDone)
}

func (a *dependencyAdapter) RunDependencyCommandWithProgress(title, message string, depCmd *dependencyCommand, onDone func(output string, err error)) {
	runDependencyCommandWithProgress(a.s.window, title, message, depCmd, onDone)
}

func (a *dependencyAdapter) ShowCommandResult(title, output string, err error) {
	showCommandResult(a.s.window, title, output, err)
}

func (a *dependencyAdapter) AllDependencies() map[string]Dependency {
	return allDependencies
}

func (a *dependencyAdapter) IsDependencyAvailableForPlatform(dep Dependency) bool {
	return isDependencyAvailableForPlatform(dep)
}

func (a *dependencyAdapter) GetDependencyCommands(depName string) dependencyCommandPair {
	return getDependencyCommands(depName)
}

func (a *dependencyAdapter) CheckDependency(command string) bool {
	return checkDependency(command)
}

func (a *dependencyAdapter) ModuleDependencies() map[string][]string {
	return moduleDependencies
}

func (a *dependencyAdapter) ModulesList() []settings.ModuleInfo {
	result := make([]settings.ModuleInfo, len(modulesList))
	for i, m := range modulesList {
		result[i] = settings.ModuleInfo{ID: m.ID, Label: m.Label}
	}
	return result
}

func (s *appState) showSettingsView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "settings"
	s.maximizeWindow()
	s.setContent(settings.BuildView(settings.Options{
		Window:               s.window,
		StatsBar:             s.statsBar,
		OnBack:               s.showMainMenu,
		BuildPreferencesTab:  func() fyne.CanvasObject { return settings.BuildPreferencesTab(&preferencesAdapter{s: s}) },
		BuildDependenciesTab: func() fyne.CanvasObject { return settings.BuildDependenciesTab(&dependencyAdapter{s: s}) },
		BuildBenchmarkTab:    func() fyne.CanvasObject { return settings.BuildBenchmarkTab(&benchmarkAdapter{s: s}) },
	}))
}
