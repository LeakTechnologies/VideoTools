package main

import (
	"git.leaktechnologies.dev/stu/VideoTools/internal/app/modulecfg"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

type thumbnailConfig = modulecfg.ThumbnailConfig

func defaultThumbnailConfig() thumbnailConfig {
	return modulecfg.DefaultThumbnailConfig()
}

func loadPersistedThumbnailConfig() (thumbnailConfig, error) {
	return modulecfg.LoadThumbnailConfig()
}

func savePersistedThumbnailConfig(cfg thumbnailConfig) error {
	return modulecfg.SaveThumbnailConfig(cfg)
}

func (s *appState) applyThumbnailConfig(cfg thumbnailConfig) {
	s.thumbnailOutputMode = cfg.OutputMode
	s.thumbnailShowTimestamps = cfg.ShowTimestamps
	s.thumbnailCount = cfg.Count
	s.thumbnailWidth = cfg.Width
	s.thumbnailSheetWidth = cfg.SheetWidth
	s.thumbnailColumns = cfg.Columns
	s.thumbnailRows = cfg.Rows
}

func (s *appState) persistThumbnailConfig() {
	cfg := thumbnailConfig{
		OutputMode:     s.thumbnailOutputMode,
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
