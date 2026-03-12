package modulecfg

import (
	"encoding/json"
	"os"
	"path/filepath"

	"git.leaktechnologies.dev/stu/VideoTools/internal/app/configpath"
)

type AuthorConfig struct {
	OutputType             string  `json:"outputType"`
	Region                 string  `json:"region"`
	AspectRatio            string  `json:"aspectRatio"`
	DiscSize               string  `json:"discSize"`
	Title                  string  `json:"title"`
	CreateMenu             bool    `json:"createMenu"`
	MenuTemplate           string  `json:"menuTemplate"`
	MenuTheme              string  `json:"menuTheme"`
	MenuBackgroundImage    string  `json:"menuBackgroundImage"`
	MenuMotionBackground   string  `json:"menuMotionBackground"`
	MenuCustomBgColor     string  `json:"menuCustomBgColor"`
	MenuCustomTextColor  string  `json:"menuCustomTextColor"`
	MenuCustomAccentColor string  `json:"menuCustomAccentColor"`
	MenuTitleLogoEnabled   bool    `json:"menuTitleLogoEnabled"`
	MenuTitleLogoPath      string  `json:"menuTitleLogoPath"`
	MenuTitleLogoPosition  string  `json:"menuTitleLogoPosition"`
	MenuTitleLogoScale     float64 `json:"menuTitleLogoScale"`
	MenuTitleLogoMargin    int     `json:"menuTitleLogoMargin"`
	MenuStudioLogoEnabled  bool    `json:"menuStudioLogoEnabled"`
	MenuStudioLogoPath     string  `json:"menuStudioLogoPath"`
	MenuStudioLogoPosition string  `json:"menuStudioLogoPosition"`
	MenuStudioLogoScale    float64 `json:"menuStudioLogoScale"`
	MenuStudioLogoMargin   int     `json:"menuStudioLogoMargin"`
	MenuStructure          string  `json:"menuStructure"`
	MenuExtrasEnabled      bool    `json:"menuExtrasEnabled"`
	MenuChapterThumbSrc    string  `json:"menuChapterThumbSrc"`
	TreatAsChapters        bool    `json:"treatAsChapters"`
	SceneThreshold         float64 `json:"sceneThreshold"`
}

func DefaultAuthorConfig() AuthorConfig {
	return AuthorConfig{
		OutputType:             "dvd",
		Region:                 "AUTO",
		AspectRatio:            "AUTO",
		DiscSize:               "DVD5",
		Title:                  "",
		CreateMenu:             false,
		MenuTemplate:           "Simple",
		MenuTheme:              "VideoTools",
		MenuBackgroundImage:    "",
		MenuTitleLogoEnabled:   false,
		MenuTitleLogoPath:      "",
		MenuTitleLogoPosition:  "Center",
		MenuTitleLogoScale:     1.0,
		MenuTitleLogoMargin:    24,
		MenuStudioLogoEnabled:  true,
		MenuStudioLogoPath:     "",
		MenuStudioLogoPosition: "Top Right",
		MenuStudioLogoScale:    1.0,
		MenuStudioLogoMargin:   24,
		MenuStructure:          "Feature + Chapters",
		MenuExtrasEnabled:      false,
		MenuChapterThumbSrc:    "Auto",
		TreatAsChapters:        false,
		SceneThreshold:         0.3,
	}
}

func LoadAuthorConfig() (AuthorConfig, error) {
	var cfg AuthorConfig
	path := configpath.ModuleConfigPath("author")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.OutputType == "" {
		cfg.OutputType = "dvd"
	}
	if cfg.Region == "" {
		cfg.Region = "AUTO"
	}
	if cfg.AspectRatio == "" {
		cfg.AspectRatio = "AUTO"
	}
	if cfg.DiscSize == "" {
		cfg.DiscSize = "DVD5"
	}
	if cfg.MenuTemplate == "" {
		cfg.MenuTemplate = "Simple"
	}
	if cfg.MenuTheme == "" {
		cfg.MenuTheme = "VideoTools"
	}
	if cfg.MenuTitleLogoPosition == "" {
		cfg.MenuTitleLogoPosition = "Center"
	}
	if cfg.MenuTitleLogoScale == 0 {
		cfg.MenuTitleLogoScale = 1.0
	}
	if cfg.MenuTitleLogoMargin == 0 {
		cfg.MenuTitleLogoMargin = 24
	}
	if cfg.MenuStudioLogoPosition == "" {
		cfg.MenuStudioLogoPosition = "Top Right"
	}
	if cfg.MenuStudioLogoScale == 0 {
		cfg.MenuStudioLogoScale = 1.0
	}
	if cfg.MenuStudioLogoMargin == 0 {
		cfg.MenuStudioLogoMargin = 24
	}
	if cfg.MenuStructure == "" {
		cfg.MenuStructure = "Feature + Chapters"
	}
	if cfg.MenuChapterThumbSrc == "" {
		cfg.MenuChapterThumbSrc = "Auto"
	}
	if cfg.SceneThreshold <= 0 {
		cfg.SceneThreshold = 0.3
	}
	return cfg, nil
}

func SaveAuthorConfig(cfg AuthorConfig) error {
	path := configpath.ModuleConfigPath("author")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
