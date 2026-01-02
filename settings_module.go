package main

import (
	"fmt"
	"image/color"
	"os/exec"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

// Dependency represents a system dependency
type Dependency struct {
	Name        string
	Command     string // Command to check if installed
	Required    bool   // If true, core functionality requires this
	Description string
	InstallCmd  string // Command to install (platform-specific)
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
	"subtitles": {"ffmpeg", "whisper"},
	"thumb":     {"ffmpeg"},
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
	},
	"dvdauthor": {
		Name:        "DVDAuthor",
		Command:     "dvdauthor",
		Required:    false,
		Description: "DVD authoring tool",
		InstallCmd:  getDVDAuthorInstallCmd(),
	},
	"xorriso": {
		Name:        "xorriso",
		Command:     "xorriso",
		Required:    false,
		Description: "ISO creation and extraction",
		InstallCmd:  getXorrisoInstallCmd(),
	},
	"realesrgan-ncnn-vulkan": {
		Name:        "Real-ESRGAN",
		Command:     "realesrgan-ncnn-vulkan",
		Required:    false,
		Description: "AI video upscaling",
		InstallCmd:  "See install.sh --skip-ai=false",
	},
	"whisper": {
		Name:        "Whisper",
		Command:     "whisper",
		Required:    false,
		Description: "AI subtitle generation",
		InstallCmd:  "pip3 install --user openai-whisper",
	},
}

func getFFmpegInstallCmd() string {
	switch runtime.GOOS {
	case "linux":
		return "sudo apt-get install ffmpeg  # or dnf/pacman/zypper"
	case "darwin":
		return "brew install ffmpeg"
	case "windows":
		return "Download from ffmpeg.org"
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
	default:
		return "./scripts/install.sh"
	}
}

func getXorrisoInstallCmd() string {
	switch runtime.GOOS {
	case "linux":
		return "sudo apt-get install xorriso  # or dnf/pacman/zypper"
	case "darwin":
		return "brew install xorriso"
	default:
		return "./scripts/install.sh"
	}
}

// checkDependency checks if a command is available
func checkDependency(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
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
		container.NewTabItem("Dependencies", buildDependenciesTab(state)),
		container.NewTabItem("Benchmark", buildBenchmarkTab(state)),
		container.NewTabItem("Preferences", buildPreferencesTab(state)),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	// Single fast scroll container for entire tabs area (12x speed)
	scrollableTabs := ui.NewFastVScroll(tabs)

	return container.NewBorder(topBar, bottomBar, nil, nil, scrollableTabs)
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

	// Check all dependencies
	for depName, dep := range allDependencies {
		isInstalled := checkDependency(dep.Command)

		nameLabel := widget.NewLabel(dep.Name)
		nameLabel.TextStyle = fyne.TextStyle{Bold: true}

		statusLabel := widget.NewLabel("")
		if isInstalled {
			statusLabel.SetText("✓ Installed")
			statusLabel.TextStyle = fyne.TextStyle{Italic: true}
		} else {
			statusLabel.SetText("✗ Not Installed")
			statusLabel.TextStyle = fyne.TextStyle{Italic: true}
		}

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
		// statusBg.SetMinSize(fyne.NewSize(12, 12)) // Removed for flexible sizing

		statusRow := container.NewHBox(statusBg, statusLabel)

		infoBox := container.NewVBox(
			container.NewHBox(nameLabel, layout.NewSpacer(), statusRow),
			descLabel,
		)

		if !isInstalled {
			installCmdLabel := widget.NewLabel("Install: " + installLabel.Text)
			installCmdLabel.Wrapping = fyne.TextWrapWord
			infoBox.Add(installCmdLabel)
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

	content.Add(widget.NewLabel("Preferences panel - Coming soon"))
	content.Add(widget.NewLabel("This will include settings for:"))
	content.Add(widget.NewLabel("• Default output directories"))
	content.Add(widget.NewLabel("• Default encoding presets"))
	content.Add(widget.NewLabel("• UI theme preferences"))
	content.Add(widget.NewLabel("• Automatic updates"))

	return content
}

func (s *appState) showSettingsView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "settings"
	s.setContent(buildSettingsView(s))
}
