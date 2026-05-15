package rip

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/utils"
)

// launchDVDPlayer opens the given disc source in an external DVD-capable player
// (VLC preferred, mpv as fallback). Both support full interactive menu navigation
// via libdvdnav, which our internal engine does not implement.
//
// sourcePath may be an ISO file, a VIDEO_TS directory, or the parent directory
// of a VIDEO_TS folder — the function normalises it to the DVD root.
func launchDVDPlayer(sourcePath string) error {
	if sourcePath == "" {
		return fmt.Errorf("no disc source loaded")
	}
	dvdRoot := resolveDVDRoot(sourcePath)

	// VLC: full DVD-nav with CSS decryption (via libdvdcss if installed)
	if vlcPath, ok := findVLC(); ok {
		// VLC accepts dvd:// URI or a plain path; dvd:// triggers the DVD input.
		// On all platforms: vlc dvd:///path  (triple slash = empty host + abs path)
		cmd := utils.CreateCommandRaw(vlcPath, "dvd://"+dvdRoot)
		if err := cmd.Start(); err == nil {
			go func() { _ = cmd.Wait() }()
			return nil
		}
	}

	// mpv: also full dvdnav via --dvd-device
	if mpvPath, err := exec.LookPath("mpv"); err == nil {
		cmd := utils.CreateCommandRaw(mpvPath, "dvd://", "--dvd-device="+dvdRoot)
		if err := cmd.Start(); err == nil {
			go func() { _ = cmd.Wait() }()
			return nil
		}
	}

	return fmt.Errorf("no DVD-capable player found — install VLC or mpv")
}

// resolveDVDRoot returns the DVD root directory suitable for passing to VLC/mpv.
// If sourcePath is an ISO, it is returned as-is. If it is a VIDEO_TS directory,
// the parent is returned. Otherwise the path is returned unchanged.
func resolveDVDRoot(sourcePath string) string {
	// ISO file: pass directly
	if strings.EqualFold(filepath.Ext(sourcePath), ".iso") {
		return sourcePath
	}
	// VIDEO_TS directory itself: go up one level to the disc root
	if strings.EqualFold(filepath.Base(sourcePath), "VIDEO_TS") {
		return filepath.Dir(sourcePath)
	}
	// Disc root or unknown: pass through
	return sourcePath
}

// findVLC returns the path to the VLC executable and true if found.
func findVLC() (string, bool) {
	// Check PATH first (covers Linux and properly-installed Windows)
	if p, err := exec.LookPath("vlc"); err == nil {
		return p, true
	}

	// Platform-specific well-known paths
	var candidates []string
	switch runtime.GOOS {
	case "windows":
		for _, envVar := range []string{"ProgramFiles", "ProgramFiles(x86)"} {
			base := os.Getenv(envVar)
			if base != "" {
				candidates = append(candidates, filepath.Join(base, "VideoLAN", "VLC", "vlc.exe"))
			}
		}
	case "darwin":
		candidates = []string{"/Applications/VLC.app/Contents/MacOS/VLC"}
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, true
		}
	}
	return "", false
}
