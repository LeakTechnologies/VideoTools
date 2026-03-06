package modulecfg

import (
	"encoding/json"
	"os"
	"path/filepath"

	"git.leaktechnologies.dev/stu/VideoTools/internal/app/configpath"
)

type MergeConfig struct {
	Format              string `json:"format"`
	KeepAllStreams      bool   `json:"keepAllStreams"`
	Chapters            bool   `json:"chapters"`
	CodecMode           string `json:"codecMode"`
	DVDRegion           string `json:"dvdRegion"`
	DVDAspect           string `json:"dvdAspect"`
	FrameRate           string `json:"frameRate"`
	MotionInterpolation bool   `json:"motionInterpolation"`
}

func DefaultMergeConfig() MergeConfig {
	return MergeConfig{
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

func LoadMergeConfig() (MergeConfig, error) {
	var cfg MergeConfig
	path := configpath.ModuleConfigPath("merge")
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

func SaveMergeConfig(cfg MergeConfig) error {
	path := configpath.ModuleConfigPath("merge")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
