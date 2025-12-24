package sysinfo

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
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

	return info
}

// detectCPU returns CPU model and clock speed
func detectCPU() (model, mhz string) {
	switch runtime.GOOS {
	case "linux":
		return detectCPULinux()
	case "windows":
		return detectCPUWindows()
	case "darwin":
		return detectCPUDarwin()
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
	// Use wmic to get CPU info
	cmd := exec.Command("wmic", "cpu", "get", "name,maxclockspeed")
	output, err := cmd.Output()
	if err != nil {
		logging.Debug(logging.CatSystem, "failed to run wmic cpu: %v", err)
		return "Unknown CPU", "Unknown"
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) >= 2 {
		// Parse the second line (first is header)
		fields := strings.Fields(lines[1])
		if len(fields) >= 2 {
			mhzStr := fields[len(fields)-1] // Last field is clock speed
			model = strings.Join(fields[:len(fields)-1], " ")
			if mhzInt, err := strconv.Atoi(mhzStr); err == nil {
				mhz = fmt.Sprintf("%d MHz", mhzInt)
			}
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

func detectCPUDarwin() (model, mhz string) {
	// Use sysctl to get CPU info
	cmdModel := exec.Command("sysctl", "-n", "machdep.cpu.brand_string")
	if output, err := cmdModel.Output(); err == nil {
		model = strings.TrimSpace(string(output))
	}

	cmdMHz := exec.Command("sysctl", "-n", "hw.cpufrequency")
	if output, err := cmdMHz.Output(); err == nil {
		if hz, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64); err == nil {
			mhz = fmt.Sprintf("%.0f MHz", float64(hz)/1000000)
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

// detectGPU returns GPU model and driver version
func detectGPU() (model, driver string) {
	switch runtime.GOOS {
	case "linux":
		return detectGPULinux()
	case "windows":
		return detectGPUWindows()
	case "darwin":
		return detectGPUDarwin()
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
	output, err = cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		if len(lines) >= 2 {
			// Skip header, get first GPU
			line := strings.TrimSpace(lines[1])
			if line != "" {
				// Parse: Name  DriverVersion
				re := regexp.MustCompile(`^(.+?)\s+(\S+)$`)
				matches := re.FindStringSubmatch(line)
				if len(matches) == 3 {
					model = strings.TrimSpace(matches[1])
					driver = strings.TrimSpace(matches[2])
					return model, driver
				}
			}
		}
	}

	return "No GPU detected", "N/A"
}

func detectGPUDarwin() (model, driver string) {
	// macOS uses system_profiler for GPU info
	cmd := exec.Command("system_profiler", "SPDisplaysDataType")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Chipset Model:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					model = strings.TrimSpace(parts[1])
				}
			}
			if strings.Contains(line, "Metal:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					driver = "Metal " + strings.TrimSpace(parts[1])
				}
			}
		}
	}

	if model == "" {
		model = "Unknown GPU"
	}
	if driver == "" {
		driver = "Unknown"
	}

	return model, driver
}

// detectRAM returns total system RAM
func detectRAM() (readable string, mb uint64) {
	switch runtime.GOOS {
	case "linux":
		return detectRAMLinux()
	case "windows":
		return detectRAMWindows()
	case "darwin":
		return detectRAMDarwin()
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
	cmd := exec.Command("wmic", "computersystem", "get", "totalphysicalmemory")
	output, err := cmd.Output()
	if err != nil {
		logging.Debug(logging.CatSystem, "failed to run wmic computersystem: %v", err)
		return "Unknown", 0
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) >= 2 {
		bytesStr := strings.TrimSpace(lines[1])
		if bytes, err := strconv.ParseUint(bytesStr, 10, 64); err == nil {
			mb = bytes / (1024 * 1024)
			gb := float64(mb) / 1024.0
			readable = fmt.Sprintf("%.1f GB", gb)
			return readable, mb
		}
	}

	return "Unknown", 0
}

func detectRAMDarwin() (readable string, mb uint64) {
	cmd := exec.Command("sysctl", "-n", "hw.memsize")
	output, err := cmd.Output()
	if err != nil {
		logging.Debug(logging.CatSystem, "failed to run sysctl hw.memsize: %v", err)
		return "Unknown", 0
	}

	bytesStr := strings.TrimSpace(string(output))
	if bytes, err := strconv.ParseUint(bytesStr, 10, 64); err == nil {
		mb = bytes / (1024 * 1024)
		gb := float64(mb) / 1024.0
		readable = fmt.Sprintf("%.1f GB", gb)
		return readable, mb
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
