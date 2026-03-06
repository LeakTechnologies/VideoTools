package configpath

import (
	"os"
	"path/filepath"
)

func ModuleConfigPath(name string) string {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		home := os.Getenv("HOME")
		if home != "" {
			configDir = filepath.Join(home, ".config")
		}
	}
	if configDir == "" {
		return name + ".json"
	}
	return filepath.Join(configDir, "VideoTools", name+".json")
}
