package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

type thumbConfig struct {
	ContactSheet   bool `json:"contactSheet"`
	ShowTimestamps bool `json:"showTimestamps"`
	Count          int  `json:"count"`
	Width          int  `json:"width"`
	SheetWidth     int  `json:"sheetWidth"`
	Columns        int  `json:"columns"`
	Rows           int  `json:"rows"`
}

func defaultThumbConfig() thumbConfig {
	return thumbConfig{
		ContactSheet:   false,
		ShowTimestamps: false,
		Count:          24,
		Width:          320,
		SheetWidth:     360,
		Columns:        4,
		Rows:           8,
	}
}

func loadPersistedThumbConfig() (thumbConfig, error) {
	var cfg thumbConfig
	path := moduleConfigPath("thumb")
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

func savePersistedThumbConfig(cfg thumbConfig) error {
	path := moduleConfigPath("thumb")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (s *appState) applyThumbConfig(cfg thumbConfig) {
	s.thumbContactSheet = cfg.ContactSheet
	s.thumbShowTimestamps = cfg.ShowTimestamps
	s.thumbCount = cfg.Count
	s.thumbWidth = cfg.Width
	s.thumbSheetWidth = cfg.SheetWidth
	s.thumbColumns = cfg.Columns
	s.thumbRows = cfg.Rows
}

func (s *appState) persistThumbConfig() {
	cfg := thumbConfig{
		ContactSheet:   s.thumbContactSheet,
		ShowTimestamps: s.thumbShowTimestamps,
		Count:          s.thumbCount,
		Width:          s.thumbWidth,
		SheetWidth:     s.thumbSheetWidth,
		Columns:        s.thumbColumns,
		Rows:           s.thumbRows,
	}
	if err := savePersistedThumbConfig(cfg); err != nil {
		logging.Debug(logging.CatSystem, "failed to persist thumb config: %v", err)
	}
}
