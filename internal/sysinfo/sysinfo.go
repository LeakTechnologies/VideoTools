package sysinfo

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/LeakTechnologies/VideoTools/internal/logging"
	"github.com/LeakTechnologies/VideoTools/internal/utils"
)

// HardwareInfo contains system hardware information
type HardwareInfo struct {
	CPU       string `json:"cpu"`
	CPUCores  int    `json:"cpu_cores"`
	CPUMHz    string `json:"cpu_mhz"`
	GPU       string `json:"gpu"`
	GPUDriver string `json:"gpu_driver"`
	RAM       string `json:"ram"`
	RAMMBytes uint64 `json:"ram_mb"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	Distro    string `json:"distro"`
	Desktop   string `json:"desktop"`
}

// Detect gathers system hardware information
func Detect() HardwareInfo {
	info := HardwareInfo{
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		CPUCores: runtime.NumCPU(),
	}

	// Detect CPU
	info.CPU, info.CPUMHz = detectCPU()

	// Detect GPU
	info.GPU, info.GPUDriver = detectGPU()

	// Detect RAM
	info.RAM, info.RAMMBytes = detectRAM()

	// Detect Linux distro and desktop environment
	if runtime.GOOS == "linux" {
		info.Distro = detectDistro()
		info.Desktop = detectDesktopEnv()
	}

	return info
}

// GPUVendor extracts the GPU vendor from the GPU string
func (h *HardwareInfo) GPUVendor() string {
	gpuLower := strings.ToLower(h.GPU)
	switch {
	case strings.Contains(gpuLower, "nvidia"):
		return "nvidia"
	case strings.Contains(gpuLower, "amd") || strings.Contains(gpuLower, "radeon"):
		return "amd"
	case strings.Contains(gpuLower, "intel"):
		return "intel"
	default:
		return "unknown"
	}
}

// detectCPU returns CPU model and clock speed
func detectCPU() (model, mhz string) {
	switch runtime.GOOS {
	case "linux":
		return detectCPULinux()
	case "windows":
		return detectCPUWindows()
	default:
		return "Unknown CPU", "Unknown"
	}
}

func detectCPULinux() (model, mhz string) {
	// Read /proc/cpuinfo
	cmd := exec.Command("cat", "/proc/cpuinfo")
	output, err := cmd.Output()
	if err != nil {
		logging.Debug(logging.CatSystem, "failed to read /proc/cpuinfo: %v", err)
		return "Unknown CPU", "Unknown"
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				model = strings.TrimSpace(parts[1])
			}
		}
		if strings.HasPrefix(line, "cpu MHz") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				mhzStr := strings.TrimSpace(parts[1])
				if mhzFloat, err := strconv.ParseFloat(mhzStr, 64); err == nil {
					mhz = fmt.Sprintf("%.0f MHz", mhzFloat)
				}
			}
		}
		// Exit early once we have both
		if model != "" && mhz != "" {
			break
		}
	}

	if model == "" {
		model = "Unknown CPU"
	}
	if mhz == "" {
		mhz = "Unknown"
	}

	return model, mhz
}

func detectCPUWindows() (model, mhz string) {
	// Use PowerShell + CIM (preferred on Windows 10/11; wmic is deprecated).
	// Output format: "Name|MaxClockSpeed"
	psCmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		`$p = Get-CimInstance Win32_Processor | Select-Object -First 1; "$($p.Name)|$($p.MaxClockSpeed)"`)
	utils.ApplyNoWindow(psCmd)
	if out, err := psCmd.Output(); err == nil {
		parts := strings.SplitN(strings.TrimSpace(string(out)), "|", 2)
		if len(parts) == 2 {
			model = strings.TrimSpace(parts[0])
			if mhzInt, err2 := strconv.Atoi(strings.TrimSpace(parts[1])); err2 == nil {
				mhz = fmt.Sprintf("%d MHz", mhzInt)
			}
			if model != "" {
				if mhz == "" {
					mhz = "Unknown"
				}
				return model, mhz
			}
		}
	}

	// Fallback: wmic (older Windows versions)
	cmd := exec.Command("wmic", "cpu", "get", "name,maxclockspeed")
	utils.ApplyNoWindow(cmd)
	if output, err := cmd.Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			fields := strings.Fields(strings.TrimSpace(line))
			if len(fields) < 2 {
				continue
			}
			// wmic may order columns alphabetically: MaxClockSpeed then Name,
			// or in requested order: Name then MaxClockSpeed. Try to parse
			// the numeric field as the clock speed.
			last := fields[len(fields)-1]
			if mhzInt, err2 := strconv.Atoi(last); err2 == nil && mhzInt > 100 {
				model = strings.Join(fields[:len(fields)-1], " ")
				mhz = fmt.Sprintf("%d MHz", mhzInt)
				break
			}
			first := fields[0]
			if mhzInt, err2 := strconv.Atoi(first); err2 == nil && mhzInt > 100 {
				model = strings.Join(fields[1:], " ")
				mhz = fmt.Sprintf("%d MHz", mhzInt)
				break
			}
		}
	} else {
		logging.Debug(logging.CatSystem, "failed to run wmic cpu: %v", err)
	}

	if model == "" {
		model = "Unknown CPU"
	}
	if mhz == "" {
		mhz = "Unknown"
	}
	return model, mhz
}

// detectGPU returns GPU model and driver version
func detectGPU() (model, driver string) {
	switch runtime.GOOS {
	case "linux":
		return detectGPULinux()
	case "windows":
		return detectGPUWindows()
	default:
		return "Unknown GPU", "Unknown"
	}
}

func detectGPULinux() (model, driver string) {
	// Try nvidia-smi first (most common for encoding)
	cmd := exec.Command("nvidia-smi", "--query-gpu=name,driver_version", "--format=csv,noheader")
	output, err := cmd.Output()
	if err == nil {
		parts := strings.Split(strings.TrimSpace(string(output)), ",")
		if len(parts) >= 2 {
			model = strings.TrimSpace(parts[0])
			driver = "NVIDIA " + strings.TrimSpace(parts[1])
			return model, driver
		}
	}

	// Try lspci for any GPU
	cmd = exec.Command("lspci")
	output, err = cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), "vga compatible") ||
				strings.Contains(strings.ToLower(line), "3d controller") {
				// Extract GPU name from lspci output
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					model = strings.TrimSpace(parts[1])
					driver = "Unknown"
					return model, driver
				}
			}
		}
	}

	return "No GPU detected", "N/A"
}

func detectGPUWindows() (model, driver string) {
	// Use nvidia-smi if available (NVIDIA GPUs)
	cmd := exec.Command("nvidia-smi", "--query-gpu=name,driver_version", "--format=csv,noheader")
	utils.ApplyNoWindow(cmd) // Hide command window on Windows
	output, err := cmd.Output()
	if err == nil {
		parts := strings.Split(strings.TrimSpace(string(output)), ",")
		if len(parts) >= 2 {
			model = strings.TrimSpace(parts[0])
			driver = "NVIDIA " + strings.TrimSpace(parts[1])
			return model, driver
		}
	}

	// Try wmic for generic GPU info
	cmd = exec.Command("wmic", "path", "win32_VideoController", "get", "name,driverversion")
	utils.ApplyNoWindow(cmd) // Hide command window on Windows
	output, err = cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		// Iterate through all video controllers, skip virtual/non-physical adapters
		for i, line := range lines {
			if i == 0 { // Skip header
				continue
			}
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// Filter out virtual/software adapters
			lineLower := strings.ToLower(line)
			if strings.Contains(lineLower, "virtual") ||
				strings.Contains(lineLower, "microsoft basic") ||
				strings.Contains(lineLower, "remote") ||
				strings.Contains(lineLower, "vnc") ||
				strings.Contains(lineLower, "parsec") ||
				strings.Contains(lineLower, "teamviewer") {
				logging.Debug(logging.CatSystem, "skipping virtual GPU: %s", line)
				continue
			}

			// Parse: Name  DriverVersion
			// Use flexible regex to handle varying whitespace
			re := regexp.MustCompile(`^(.+?)\s+(\S+)$`)
			matches := re.FindStringSubmatch(line)
			if len(matches) == 3 {
				model = strings.TrimSpace(matches[1])
				driver = strings.TrimSpace(matches[2])
				logging.Debug(logging.CatSystem, "detected physical GPU: %s (driver: %s)", model, driver)
				return model, driver
			}
		}
	}

	return "No GPU detected", "N/A"
}

// detectRAM returns total system RAM
func detectRAM() (readable string, mb uint64) {
	switch runtime.GOOS {
	case "linux":
		return detectRAMLinux()
	case "windows":
		return detectRAMWindows()
	default:
		return "Unknown", 0
	}
}

func detectRAMLinux() (readable string, mb uint64) {
	cmd := exec.Command("cat", "/proc/meminfo")
	output, err := cmd.Output()
	if err != nil {
		logging.Debug(logging.CatSystem, "failed to read /proc/meminfo: %v", err)
		return "Unknown", 0
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if kb, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
					mb = kb / 1024
					gb := float64(mb) / 1024.0
					readable = fmt.Sprintf("%.1f GB", gb)
					return readable, mb
				}
			}
		}
	}

	return "Unknown", 0
}

func detectRAMWindows() (readable string, mb uint64) {
	// Use PowerShell + CIM (preferred on Windows 10/11; wmic is deprecated).
	psCmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		"(Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory")
	utils.ApplyNoWindow(psCmd)
	if out, err := psCmd.Output(); err == nil {
		bytesStr := strings.TrimSpace(string(out))
		if bytes, err2 := strconv.ParseUint(bytesStr, 10, 64); err2 == nil && bytes > 0 {
			mb = bytes / (1024 * 1024)
			gb := float64(mb) / 1024.0
			readable = fmt.Sprintf("%.1f GB", gb)
			return readable, mb
		}
	}

	// Fallback: wmic (older Windows versions)
	cmd := exec.Command("wmic", "computersystem", "get", "totalphysicalmemory")
	utils.ApplyNoWindow(cmd)
	output, err := cmd.Output()
	if err != nil {
		logging.Debug(logging.CatSystem, "failed to run wmic computersystem: %v", err)
		return "Unknown", 0
	}

	for _, line := range strings.Split(string(output), "\n") {
		bytesStr := strings.TrimSpace(line)
		if bytes, err2 := strconv.ParseUint(bytesStr, 10, 64); err2 == nil && bytes > 0 {
			mb = bytes / (1024 * 1024)
			gb := float64(mb) / 1024.0
			readable = fmt.Sprintf("%.1f GB", gb)
			return readable, mb
		}
	}

	return "Unknown", 0
}

// Summary returns a human-readable summary of hardware info
func (h HardwareInfo) Summary() string {
	return fmt.Sprintf("%s\n%s (%d cores @ %s)\nGPU: %s\nDriver: %s\nRAM: %s",
		h.OS+"/"+h.Arch,
		h.CPU,
		h.CPUCores,
		h.CPUMHz,
		h.GPU,
		h.GPUDriver,
		h.RAM,
	)
}

// detectDistro detects the Linux distribution
func detectDistro() string {
	// Check /etc/os-release first
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		content := string(data)
		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
			}
			if strings.HasPrefix(line, "NAME=") && !strings.HasPrefix(line, "NAME_LIKE") {
				return strings.Trim(strings.TrimPrefix(line, "NAME="), "\"")
			}
		}
	}

	// Fallback: check specific distribution files
	distros := []string{
		"/etc/arch-release",   // Arch
		"/etc/debian_version", // Debian
		"/etc/fedora-release", // Fedora
		"/etc/gentoo-release", // Gentoo
		"/etc/SuSE-release",   // openSUSE
		"/etc/redhat-release", // RHEL/CentOS
		"/etc/ubuntu-release", // Ubuntu
	}

	for _, d := range distros {
		if _, err := os.Stat(d); err == nil {
			if data, err := os.ReadFile(d); err == nil {
				return strings.TrimSpace(string(data))
			}
		}
	}

	return "Linux (unknown)"
}

// detectDesktopEnv detects the desktop environment
func detectDesktopEnv() string {
	// Check common desktop environment variables
	envVars := []string{
		"XDG_CURRENT_DESKTOP",
		"DESKTOP_SESSION",
		"GNOME_DESKTOP_SESSION_ID",
		"KDE_FULL_SESSION",
		"XFCE4_SESSION",
	}

	for _, env := range envVars {
		if val := os.Getenv(env); val != "" {
			return val
		}
	}

	// Check for running desktop environment processes
	desktops := []string{"gnome", "kde", "xfce", "mate", "lxde", "cinnamon", "i3", "sway"}
	for _, desktop := range desktops {
		if out, err := exec.Command("pgrep", "-x", desktop).Output(); err == nil && len(out) > 0 {
			return strings.ToUpper(desktop)
		}
	}

	return "Unknown"
}
