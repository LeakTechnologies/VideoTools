package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

type mergeConfig struct {
	Format               string `json:"format"`
	KeepAllStreams       bool   `json:"keepAllStreams"`
	Chapters             bool   `json:"chapters"`
	CodecMode            string `json:"codecMode"`
	DVDRegion            string `json:"dvdRegion"`
	DVDAspect            string `json:"dvdAspect"`
	FrameRate            string `json:"frameRate"`
	MotionInterpolation  bool   `json:"motionInterpolation"`
}

func defaultMergeConfig() mergeConfig {
	return mergeConfig{
		Format:              "mkv-copy",
		KeepAllStreams:      false,
		Chapters:            true,
		CodecMode:           "",
		DVDRegion:           "NTSC",
		DVDAspect:           "16:9",
		FrameRate:           "Source",
		MotionInterpolation: false,
	}
}

func loadPersistedMergeConfig() (mergeConfig, error) {
	var cfg mergeConfig
	path := moduleConfigPath("merge")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Format == "" {
		cfg.Format = "mkv-copy"
	}
	if cfg.DVDRegion == "" {
		cfg.DVDRegion = "NTSC"
	}
	if cfg.DVDAspect == "" {
		cfg.DVDAspect = "16:9"
	}
	if cfg.FrameRate == "" {
		cfg.FrameRate = "Source"
	}
	return cfg, nil
}

func savePersistedMergeConfig(cfg mergeConfig) error {
	path := moduleConfigPath("merge")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (s *appState) applyMergeConfig(cfg mergeConfig) {
	s.mergeFormat = cfg.Format
	s.mergeKeepAll = cfg.KeepAllStreams
	s.mergeChapters = cfg.Chapters
	s.mergeCodecMode = cfg.CodecMode
	s.mergeDVDRegion = cfg.DVDRegion
	s.mergeDVDAspect = cfg.DVDAspect
	s.mergeFrameRate = cfg.FrameRate
	s.mergeMotionInterpolation = cfg.MotionInterpolation
}

func (s *appState) persistMergeConfig() {
	cfg := mergeConfig{
		Format:              s.mergeFormat,
		KeepAllStreams:      s.mergeKeepAll,
		Chapters:            s.mergeChapters,
		CodecMode:           s.mergeCodecMode,
		DVDRegion:           s.mergeDVDRegion,
		DVDAspect:           s.mergeDVDAspect,
		FrameRate:           s.mergeFrameRate,
		MotionInterpolation: s.mergeMotionInterpolation,
	}
	if err := savePersistedMergeConfig(cfg); err != nil {
		logging.Debug(logging.CatSystem, "failed to persist merge config: %v", err)
	}
}
