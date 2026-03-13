package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type WSLConfig struct {
	Distro string
}

var wslDistro string

func init() {
	if runtime.GOOS == "windows" {
		if distro := os.Getenv("VT_WSL_DISTRO"); distro != "" {
			wslDistro = distro
		}
	}
}

func SetWSLDistro(distro string) {
	wslDistro = distro
}

func IsWSLAvailable() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	
	cmd := HideWindowExec("wsl", "--status")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	
	outputStr := string(output)
	return !strings.Contains(outputStr, "Error") && 
		   !strings.Contains(outputStr, "not recognized") &&
		   !strings.Contains(outputStr, "not found")
}

func FindWSLDistro() (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("WSL is only available on Windows")
	}

	if wslDistro != "" {
		if err := verifyWSLDistro(wslDistro); err == nil {
			return wslDistro, nil
		}
	}

	cmd := HideWindowExec("wsl", "-l", "-q")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list WSL distributions: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if verifyWSLDistro(line) == nil {
			return line, nil
		}
	}

	return "", fmt.Errorf("no WSL distribution found")
}

func verifyWSLDistro(distro string) error {
	cmd := HideWindowExec("wsl", "-t", distro)
	return cmd.Run()
}

func WSLRunCommand(distro string, command string, args ...string) *exec.Cmd {
	wslArgs := []string{"-d", distro, "--", command}
	wslArgs = append(wslArgs, args...)
	return HideWindowExec("wsl", wslArgs...)
}

func WSLRunCommandWithPaths(distro string, winWorkDir string, command string, args ...string) (*exec.Cmd, error) {
	absPath, err := filepath.Abs(winWorkDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	wslWorkDir, err := WindowsToWSLPath(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to convert path to WSL: %w", err)
	}

	fullCommand := fmt.Sprintf("cd %s && %s %s", wslWorkDir, command, strings.Join(args, " "))
	
	wslArgs := []string{"-d", distro, "--", "sh", "-c", fullCommand}
	return HideWindowExec("wsl", wslArgs...), nil
}

func WindowsToWSLPath(winPath string) (string, error) {
	winPath = filepath.ToSlash(winPath)
	
	if len(winPath) >= 2 && winPath[1] == ':' {
		letter := strings.ToLower(string(winPath[0]))
		return "/mnt/" + letter + winPath[2:], nil
	}
	
	return winPath, nil
}

func HasISOTools() bool {
	if runtime.GOOS != "windows" {
		return false
	}

	distro, err := FindWSLDistro()
	if err != nil {
		return false
	}

	cmd := WSLRunCommand(distro, "which", "xorriso")
	if err := cmd.Run(); err == nil {
		return true
	}

	cmd = WSLRunCommand(distro, "which", "mkisofs")
	if err := cmd.Run(); err == nil {
		return true
	}

	cmd = WSLRunCommand(distro, "which", "genisoimage")
	if err := cmd.Run(); err == nil {
		return true
	}

	return false
}
