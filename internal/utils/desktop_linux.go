//go:build linux

package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
)

// EnsureLinuxDesktopEntry installs a user-level desktop entry and icon so GNOME can
// associate the running app with a stable icon.
func EnsureLinuxDesktopEntry(appID, appName string) {
	iconPath := findLinuxIconPath()
	if iconPath == "" {
		logging.Debug(logging.CatUI, "desktop entry skipped: icon not found")
		return
	}

	exe, err := os.Executable()
	if err != nil || exe == "" {
		logging.Debug(logging.CatUI, "desktop entry skipped: executable path unavailable")
		return
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		logging.Debug(logging.CatUI, "desktop entry skipped: home dir unavailable")
		return
	}

	iconDir := filepath.Join(home, ".local", "share", "icons", "hicolor", "256x256", "apps")
	if err := os.MkdirAll(iconDir, 0755); err != nil {
		logging.Debug(logging.CatUI, "desktop entry skipped: create icon dir failed: %v", err)
		return
	}
	iconTarget := filepath.Join(iconDir, fmt.Sprintf("%s.png", appID))
	if err := copyFileIfDifferent(iconPath, iconTarget); err != nil {
		logging.Debug(logging.CatUI, "desktop entry skipped: icon copy failed: %v", err)
		return
	}

	desktopDir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(desktopDir, 0755); err != nil {
		logging.Debug(logging.CatUI, "desktop entry skipped: create desktop dir failed: %v", err)
		return
	}
	desktopTarget := filepath.Join(desktopDir, fmt.Sprintf("%s.desktop", appID))
	desktopContents := fmt.Sprintf(`[Desktop Entry]
Name=%s
Exec=%s
Icon=%s
Type=Application
Categories=AudioVideo;Video;Utility;
Terminal=false
StartupWMClass=%s
`, appName, exe, appID, appID)

	if err := writeFileIfDifferent(desktopTarget, desktopContents); err != nil {
		logging.Debug(logging.CatUI, "desktop entry skipped: write failed: %v", err)
		return
	}
}

func findLinuxIconPath() string {
	candidates := []string{
		filepath.Join("assets", "logo", "VT_Icon.png"),
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates, filepath.Join(dir, "assets", "logo", "VT_Icon.png"))
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func copyFileIfDifferent(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if dstInfo, err := os.Stat(dst); err == nil {
		if srcInfo.Size() == dstInfo.Size() && srcInfo.ModTime().Equal(dstInfo.ModTime()) {
			return nil
		}
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime())
}

func writeFileIfDifferent(path, contents string) error {
	if existing, err := os.ReadFile(path); err == nil {
		if strings.TrimSpace(string(existing)) == strings.TrimSpace(contents) {
			return nil
		}
	}
	return os.WriteFile(path, []byte(contents), 0644)
}
