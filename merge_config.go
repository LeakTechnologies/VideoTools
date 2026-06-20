package main

import (
	"github.com/LeakTechnologies/VideoTools/internal/app/modulecfg"
	"github.com/LeakTechnologies/VideoTools/internal/logging"
)

type mergeConfig = modulecfg.MergeConfig

func defaultMergeConfig() mergeConfig {
	return modulecfg.DefaultMergeConfig()
}

func loadPersistedMergeConfig() (mergeConfig, error) {
	return modulecfg.LoadMergeConfig()
}

func savePersistedMergeConfig(cfg mergeConfig) error {
	return modulecfg.SaveMergeConfig(cfg)
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
