package utils

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
)

// min returns the minimum of two float64 values
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// abs returns the absolute value of an int
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// GUIEnvironment contains information about the user's desktop environment
type GUIEnvironment struct {
	DisplayServer      string  // "x11", "wayland", "windows", "darwin"
	DesktopEnvironment string  // "gnome", "kde", "xfce", "windows11", "macos"
	ScaleFactor        float64 // Display scaling factor
	PrimaryMonitor     MonitorInfo
	HasCompositing     bool
	GPUInfo            GPUInfo
}

// MonitorInfo contains display monitor information
type MonitorInfo struct {
	Width       int
	Height      int
	ScaleFactor float64
	IsPrimary   bool
}

// GPUInfo contains graphics card information
type GPUInfo struct {
	Vendor    string // "nvidia", "amd", "intel", "unknown"
	Model     string
	Driver    string
	Supported bool // GPU acceleration support detected
}

// DetectGUIEnvironment performs comprehensive GUI environment detection
func DetectGUIEnvironment() GUIEnvironment {
	env := GUIEnvironment{
		ScaleFactor: 1.0,
		GPUInfo:     GPUInfo{Vendor: "unknown"},
	}

	switch runtime.GOOS {
	case "linux":
		env.detectLinuxGUI()
	case "windows":
		env.detectWindowsGUI()
	case "darwin":
		env.detectMacGUI()
	}

	return env
}

// detectLinuxGUI handles Linux-specific GUI detection
func (env *GUIEnvironment) detectLinuxGUI() {
	// Display server detection
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		env.DisplayServer = "wayland"
	} else if os.Getenv("DISPLAY") != "" {
		env.DisplayServer = "x11"
	} else {
		env.DisplayServer = "headless"
	}

	// Desktop environment detection
	if desktop := os.Getenv("XDG_CURRENT_DESKTOP"); desktop != "" {
		desktops := strings.ToLower(desktop)
		if strings.Contains(desktops, "gnome") {
			env.DesktopEnvironment = "gnome"
		} else if strings.Contains(desktops, "kde") {
			env.DesktopEnvironment = "kde"
		} else if strings.Contains(desktops, "xfce") {
			env.DesktopEnvironment = "xfce"
		} else if strings.Contains(desktops, "sway") {
			env.DesktopEnvironment = "sway"
		} else if strings.Contains(desktops, "i3") {
			env.DesktopEnvironment = "i3"
		} else {
			env.DesktopEnvironment = "unknown"
		}
	}

	// GPU detection
	env.GPUInfo.detectLinuxGPU()

	// Scale factor detection
	env.detectLinuxScale()
}

// detectLinuxGPU performs GPU detection on Linux
func (env *GPUInfo) detectLinuxGPU() {
	if cmd, err := exec.Command("lspci").Output(); err == nil {
		gpuInfo := string(cmd)
		lines := strings.Split(gpuInfo, "\n")

		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), "vga") ||
				strings.Contains(strings.ToLower(line), "3d") ||
				strings.Contains(strings.ToLower(line), "display") {

				line = strings.ToLower(line)
				if strings.Contains(line, "nvidia") {
					env.Vendor = "nvidia"
					env.Supported = true
				} else if strings.Contains(line, "amd") || strings.Contains(line, "radeon") || strings.Contains(line, "advanced micro devices") {
					env.Vendor = "amd"
					env.Supported = true
				} else if strings.Contains(line, "intel") {
					env.Vendor = "intel"
					env.Supported = true
				}

				// Extract model (simplified)
				if colonPos := strings.LastIndex(line, ":"); colonPos != -1 {
					env.Model = strings.TrimSpace(line[colonPos+1:])
					// Clean up common prefixes
					env.Model = strings.Replace(env.Model, "controller", "", -1)
					env.Model = strings.Replace(env.Model, "corporation", "", -1)
				}
				break
			}
		}
	}
}

// detectLinuxScale detects display scaling on Linux
func (env *GUIEnvironment) detectLinuxScale() {
	// Try to get scale factor from various sources
	scaleFactors := []float64{1.0}

	// Try GDK_SCALE (GNOME/GTK)
	if gdkScale := os.Getenv("GDK_SCALE"); gdkScale != "" {
		if scale, err := strconv.ParseFloat(gdkScale, 64); err == nil {
			scaleFactors = append(scaleFactors, scale)
		}
	}

	// Try QT_SCALE_FACTOR (Qt applications)
	if qtScale := os.Getenv("QT_SCALE_FACTOR"); qtScale != "" {
		if scale, err := strconv.ParseFloat(qtScale, 64); err == nil {
			scaleFactors = append(scaleFactors, scale)
		}
	}

	// Try Xft.dpi (X11 DPI)
	if xftDPI := os.Getenv("Xft.dpi"); xftDPI != "" {
		if dpi, err := strconv.ParseFloat(xftDPI, 64); err == nil {
			scale := dpi / 96.0
			scaleFactors = append(scaleFactors, scale)
		}
	}

	// Use the largest scale factor found (most likely to be correct)
	env.ScaleFactor = scaleFactors[0]
	for _, scale := range scaleFactors {
		if scale > env.ScaleFactor {
			env.ScaleFactor = scale
		}
	}

	// Clamp to reasonable range
	if env.ScaleFactor > 4.0 {
		env.ScaleFactor = 4.0
	} else if env.ScaleFactor < 0.5 {
		env.ScaleFactor = 0.5
	}
}

// detectWindowsGUI handles Windows-specific GUI detection
func (env *GUIEnvironment) detectWindowsGUI() {
	env.DisplayServer = "windows"

	// Get Windows version info
	if cmd, err := exec.Command("cmd", "/c", "ver").Output(); err == nil {
		version := string(cmd)
		if strings.Contains(version, "10.0.") {
			// Check build number to distinguish Windows 11
			if buildCmd, err := exec.Command("powershell", "-Command", "(Get-CimInstance Win32_OperatingSystem).BuildNumber").Output(); err == nil {
				buildStr := strings.TrimSpace(string(buildCmd))
				if build, err := strconv.Atoi(buildStr); err == nil {
					if build >= 22000 {
						env.DesktopEnvironment = "windows11"
					} else {
						env.DesktopEnvironment = "windows10"
					}
				}
			} else {
				env.DesktopEnvironment = "windows10"
			}
		} else {
			env.DesktopEnvironment = "windows_legacy"
		}
	}

	// GPU detection for Windows
	env.GPUInfo.detectWindowsGPU()

	// Windows DPI detection
	env.detectWindowsScale()
}

// detectWindowsGPU performs GPU detection on Windows
func (env *GPUInfo) detectWindowsGPU() {
	if cmd, err := exec.Command("powershell", "-Command", "Get-WmiObject Win32_VideoController | Select-Object Name").Output(); err == nil {
		gpuName := strings.TrimSpace(string(cmd))
		env.Model = strings.ReplaceAll(gpuName, "\r", "")

		gpuNameLower := strings.ToLower(gpuName)
		if strings.Contains(gpuNameLower, "nvidia") {
			env.Vendor = "nvidia"
			env.Supported = true
		} else if strings.Contains(gpuNameLower, "amd") || strings.Contains(gpuNameLower, "radeon") {
			env.Vendor = "amd"
			env.Supported = true
		} else if strings.Contains(gpuNameLower, "intel") {
			env.Vendor = "intel"
			env.Supported = true
		}
	}
}

// IsLikelySoftwareOnlyAdapter returns true when the GPU name maps to a VM/basic display adapter.
func (env GPUInfo) IsLikelySoftwareOnlyAdapter() (bool, string) {
	if env.Model == "" {
		return false, ""
	}
	name := strings.ToLower(env.Model)
	markers := []string{
		"microsoft basic display adapter",
		"microsoft basic render driver",
		"vmware svga",
		"virtualbox",
		"parallels",
		"qxl",
		"virtio",
		"hyper-v",
		"remote display",
	}
	for _, marker := range markers {
		if strings.Contains(name, marker) {
			return true, marker
		}
	}
	return false, ""
}

// detectWindowsScale detects display scaling on Windows
func (env *GUIEnvironment) detectWindowsScale() {
	// Try to get DPI from PowerShell
	if cmd, err := exec.Command("powershell", "-Command", "Add-Type -TypeDefinition 'using System; using System.Runtime.InteropServices; public class DPI { [DllImport(\"user32.dll\")] public static extern IntPtr GetDC(IntPtr ptr); [DllImport(\"gdi32.dll\")] public static extern int GetDeviceCaps(IntPtr hdc, int nIndex); [DllImport(\"user32.dll\")] public static extern int ReleaseDC(IntPtr ptr, IntPtr hdc); public const int LOGPIXELSX = 88; public static double GetScale() { IntPtr hdc = GetDC(IntPtr.Zero); int dpi = GetDeviceCaps(hdc, LOGPIXELSX); ReleaseDC(IntPtr.Zero, hdc); return dpi / 96.0; } }; [DPI]::GetScale()").Output(); err == nil {
		if scaleStr := strings.TrimSpace(string(cmd)); scaleStr != "" {
			if scale, err := strconv.ParseFloat(scaleStr, 64); err == nil {
				env.ScaleFactor = scale
			}
		}
	}

	// Fallback to registry if PowerShell method fails
	if env.ScaleFactor == 1.0 {
		if cmd, err := exec.Command("reg", "query", "HKCU\\Control Panel\\Desktop", "/v", "LogPixels").Output(); err == nil {
			output := string(cmd)
			if strings.Contains(output, "0x") {
				// Parse hex value
				parts := strings.Fields(output)
				for _, part := range parts {
					if strings.HasPrefix(part, "0x") {
						if dpi, err := strconv.ParseInt(part, 0, 64); err == nil {
							env.ScaleFactor = float64(dpi) / 96.0
						}
						break
					}
				}
			}
		}
	}
}

// detectMacGUI handles macOS-specific GUI detection
func (env *GUIEnvironment) detectMacGUI() {
	env.DisplayServer = "darwin"
	env.DesktopEnvironment = "macos"

	// macOS has good built-in scaling
	if cmd, err := exec.Command("system_profiler", "SPDisplaysDataType", "-json").Output(); err == nil {
		// Parse macOS display info for retina/HiDPI detection
		displayInfo := string(cmd)
		if strings.Contains(displayInfo, "Retina") || strings.Contains(displayInfo, "HiDPI") {
			env.ScaleFactor = 2.0
		}
	}

	// GPU detection for macOS
	env.GPUInfo.detectMacGPU()
}

// detectMacGPU performs GPU detection on macOS
func (env *GPUInfo) detectMacGPU() {
	if cmd, err := exec.Command("system_profiler", "SPDisplaysDataType").Output(); err == nil {
		gpuInfo := string(cmd)
		if strings.Contains(strings.ToLower(gpuInfo), "amd") || strings.Contains(strings.ToLower(gpuInfo), "radeon") {
			env.Vendor = "amd"
			env.Supported = true
		} else if strings.Contains(strings.ToLower(gpuInfo), "intel") {
			env.Vendor = "intel"
			env.Supported = true
		} else if strings.Contains(strings.ToLower(gpuInfo), "nvidia") {
			env.Vendor = "nvidia"
			env.Supported = true
		}
	}
}

// GetOptimalWindowSize returns the optimal window size for the current environment
func (env GUIEnvironment) GetOptimalWindowSize(minWidth, minHeight int) fyne.Size {
	// Apply scale factor to minimum size
	scaledWidth := float64(minWidth) * env.ScaleFactor
	scaledHeight := float64(minHeight) * env.ScaleFactor

	// Apply platform-specific adjustments
	switch env.DisplayServer {
	case "windows":
		if env.DesktopEnvironment == "windows11" {
			// Windows 11 has modern UI scaling, but we should be reasonable
			scaledWidth = min(scaledWidth, 1600)
			scaledHeight = min(scaledHeight, 1200)
		}
	case "wayland":
		// Wayland handles scaling well, but we should be reasonable
		scaledWidth = min(scaledWidth, 1400)
		scaledHeight = min(scaledHeight, 1000)
	case "x11":
		// Traditional X11 might need more consideration for HiDPI
		if env.ScaleFactor > 1.5 {
			scaledWidth = min(scaledWidth, 1200)
			scaledHeight = min(scaledHeight, 900)
		}
	}

	return fyne.NewSize(float32(scaledWidth), float32(scaledHeight))
}

// String returns a human-readable description of the GUI environment
func (env GUIEnvironment) String() string {
	return fmt.Sprintf("Display: %s, Desktop: %s, Scale: %.1fx, GPU: %s %s",
		env.DisplayServer, env.DesktopEnvironment, env.ScaleFactor,
		env.GPUInfo.Vendor, env.GPUInfo.Model)
}

// SupportsGPUAcceleration returns true if GPU acceleration is likely available
func (env GUIEnvironment) SupportsGPUAcceleration() bool {
	return env.GPUInfo.Supported && env.ScaleFactor <= 2.0 // Don't use GPU acceleration on very high DPI displays
}

// GetModuleSpecificSize returns optimal size for specific modules
func (env GUIEnvironment) GetModuleSpecificSize(moduleID string) fyne.Size {
	baseMinWidth := 800
	baseMinHeight := 600

	switch moduleID {
	case "player":
		// Player needs more space for video preview
		baseMinWidth = 1024
		baseMinHeight = 768
	case "author":
		// Author module benefits from wider layout
		baseMinWidth = 900
		baseMinHeight = 700
	case "queue":
		// Queue can be more compact but needs vertical space
		baseMinWidth = 700
		baseMinHeight = 500
	case "settings":
		// Settings can be compact
		baseMinWidth = 600
		baseMinHeight = 500
	}

	return env.GetOptimalWindowSize(baseMinWidth, baseMinHeight)
}
