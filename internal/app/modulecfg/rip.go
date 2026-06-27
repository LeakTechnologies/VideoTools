package modulecfg

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/LeakTechnologies/VideoTools/internal/app/configpath"
)

type RipConfig struct {
	Format           string `json:"format"`
	EmbedChapters    bool   `json:"embed_chapters"`
	AllAudioTracks   bool   `json:"all_audio_tracks"`
	IncludeSubtitles bool   `json:"include_subtitles"`
	IncludeMenus     bool   `json:"include_menus"`
}

func DefaultRipConfig() RipConfig {
	return RipConfig{
		Format:         "Lossless MKV (Copy)",
		EmbedChapters:  true,
		AllAudioTracks: true,
		// IncludeSubtitles defaults false — some discs have broken sub streams
	}
}

func LoadRipConfig() (RipConfig, error) {
	var cfg RipConfig
	path := configpath.ModuleConfigPath("rip")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Format == "" {
		cfg.Format = "Lossless MKV (Copy)"
	}
	return cfg, nil
}

func SaveRipConfig(cfg RipConfig) error {
	path := configpath.ModuleConfigPath("rip")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
