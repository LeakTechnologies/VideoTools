package modulecfg

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/LeakTechnologies/VideoTools/internal/app/configpath"
)

type AudioConfig struct {
	OutputFormat   string  `json:"outputFormat"`
	Quality        string  `json:"quality"`
	Bitrate        string  `json:"bitrate"`
	Normalize      bool    `json:"normalize"`
	NormTargetLUFS float64 `json:"normTargetLUFS"`
	NormTruePeak   float64 `json:"normTruePeak"`
	OutputDir      string  `json:"outputDir"`
}

func DefaultAudioConfig() AudioConfig {
	return AudioConfig{
		OutputFormat:   "MP3",
		Quality:        "Medium",
		Bitrate:        "192k",
		Normalize:      false,
		NormTargetLUFS: -23.0,
		NormTruePeak:   -1.0,
		OutputDir:      "",
	}
}

func LoadAudioConfig() (AudioConfig, error) {
	var cfg AudioConfig
	path := configpath.ModuleConfigPath("audio")
	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultAudioConfig(), err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultAudioConfig(), err
	}
	return cfg, nil
}

func SaveAudioConfig(cfg AudioConfig) error {
	path := configpath.ModuleConfigPath("audio")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
