package modulecfg

import (
	"encoding/json"
	"os"
	"path/filepath"

	"git.leaktechnologies.dev/stu/VideoTools/internal/app/configpath"
)

type ThumbnailConfig struct {
	ContactSheet   bool `json:"contactSheet"`
	ShowTimestamps bool `json:"showTimestamps"`
	Count          int  `json:"count"`
	Width          int  `json:"width"`
	SheetWidth     int  `json:"sheetWidth"`
	Columns        int  `json:"columns"`
	Rows           int  `json:"rows"`
}

func DefaultThumbnailConfig() ThumbnailConfig {
	return ThumbnailConfig{
		ContactSheet:   false,
		ShowTimestamps: false,
		Count:          24,
		Width:          320,
		SheetWidth:     360,
		Columns:        4,
		Rows:           8,
	}
}

func LoadThumbnailConfig() (ThumbnailConfig, error) {
	var cfg ThumbnailConfig
	path := configpath.ModuleConfigPath("thumbnail")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Count == 0 {
		cfg.Count = 24
	}
	if cfg.Width == 0 {
		cfg.Width = 320
	}
	if cfg.SheetWidth == 0 {
		cfg.SheetWidth = 360
	}
	if cfg.Columns == 0 {
		cfg.Columns = 4
	}
	if cfg.Rows == 0 {
		cfg.Rows = 8
	}
	return cfg, nil
}

func SaveThumbnailConfig(cfg ThumbnailConfig) error {
	path := configpath.ModuleConfigPath("thumbnail")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
