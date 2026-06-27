package modulecfg

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/LeakTechnologies/VideoTools/internal/app/configpath"
)

type SubtitlesConfig struct {
	OutputMode  string  `json:"outputMode"`
	ModelPath   string  `json:"modelPath"`
	BackendPath string  `json:"backendPath"`
	BurnOutput  string  `json:"burnOutput"`
	TimeOffset  float64 `json:"timeOffset"`
	OCRLanguage string  `json:"ocrLanguage"`
	OCROutput   string  `json:"ocrOutput"`
}

func DefaultSubtitlesConfig() SubtitlesConfig {
	return SubtitlesConfig{
		OutputMode:  "External Subtitle File",
		ModelPath:   "",
		BackendPath: "",
		BurnOutput:  "",
		OCRLanguage: "eng",
		OCROutput:   "srt",
	}
}

func LoadSubtitlesConfig() (SubtitlesConfig, error) {
	var cfg SubtitlesConfig
	path := configpath.ModuleConfigPath("subtitles")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.OutputMode == "" {
		cfg.OutputMode = "External Subtitle File"
	}
	if cfg.OutputMode == "External SRT" {
		cfg.OutputMode = "External Subtitle File"
	}
	if cfg.OCRLanguage == "" {
		cfg.OCRLanguage = "eng"
	}
	if cfg.OCROutput == "" {
		cfg.OCROutput = "srt"
	}
	return cfg, nil
}

func SaveSubtitlesConfig(cfg SubtitlesConfig) error {
	path := configpath.ModuleConfigPath("subtitles")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
