package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"git.leaktechnologies.dev/stu/VideoTools/internal/app/configpath"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

type thumbnailConfig struct {
	ContactSheet   bool `json:"contactSheet"`
	ShowTimestamps bool `json:"showTimestamps"`
	Count          int  `json:"count"`
	Width          int  `json:"width"`
	SheetWidth     int  `json:"sheetWidth"`
	Columns        int  `json:"columns"`
	Rows           int  `json:"rows"`
}

func defaultThumbnailConfig() thumbnailConfig {
	return thumbnailConfig{
		ContactSheet:   false,
		ShowTimestamps: false,
		Count:          24,
		Width:          320,
		SheetWidth:     360,
		Columns:        4,
		Rows:           8,
	}
}

func loadPersistedThumbnailConfig() (thumbnailConfig, error) {
	var cfg thumbnailConfig
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

func savePersistedThumbnailConfig(cfg thumbnailConfig) error {
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

func (s *appState) applyThumbnailConfig(cfg thumbnailConfig) {
	s.thumbnailContactSheet = cfg.ContactSheet
	s.thumbnailShowTimestamps = cfg.ShowTimestamps
	s.thumbnailCount = cfg.Count
	s.thumbnailWidth = cfg.Width
	s.thumbnailSheetWidth = cfg.SheetWidth
	s.thumbnailColumns = cfg.Columns
	s.thumbnailRows = cfg.Rows
}

func (s *appState) persistThumbnailConfig() {
	cfg := thumbnailConfig{
		ContactSheet:   s.thumbnailContactSheet,
		ShowTimestamps: s.thumbnailShowTimestamps,
		Count:          s.thumbnailCount,
		Width:          s.thumbnailWidth,
		SheetWidth:     s.thumbnailSheetWidth,
		Columns:        s.thumbnailColumns,
		Rows:           s.thumbnailRows,
	}
	if err := savePersistedThumbnailConfig(cfg); err != nil {
		logging.Debug(logging.CatSystem, "failed to persist thumb config: %v", err)
	}
}
