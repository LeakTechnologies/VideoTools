package appcfg

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/app/configpath"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/benchmark"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/sysinfo"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/ui"
)

type ConvertRecoveryState struct {
	Active    bool   `json:"active"`
	StartedAt string `json:"startedAt"`
	Input     string `json:"input"`
	Output    string `json:"output"`
	LogPath   string `json:"logPath"`
}

func LoadConvertRecovery() (ConvertRecoveryState, error) {
	var state ConvertRecoveryState
	path := configpath.ModuleConfigPath("convert-recovery")
	data, err := os.ReadFile(path)
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, err
	}
	return state, nil
}

func SaveConvertRecovery(state ConvertRecoveryState) error {
	path := configpath.ModuleConfigPath("convert-recovery")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

type BenchmarkRun struct {
	Timestamp          time.Time             `json:"timestamp"`
	Results            []benchmark.Result    `json:"results"`
	RecommendedEncoder string                `json:"recommended_encoder"`
	RecommendedPreset  string                `json:"recommended_preset"`
	RecommendedHWAccel string                `json:"recommended_hwaccel"`
	RecommendedFPS     float64               `json:"recommended_fps"`
	HardwareInfo       sysinfo.HardwareInfo  `json:"hardware_info"`
}

type BenchmarkConfig struct {
	History []BenchmarkRun `json:"history"`
}

func LoadBenchmarkConfig() (BenchmarkConfig, error) {
	path := configpath.ModuleConfigPath("benchmark")
	data, err := os.ReadFile(path)
	if err != nil {
		return BenchmarkConfig{}, err
	}
	var cfg BenchmarkConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return BenchmarkConfig{}, err
	}
	return cfg, nil
}

func SaveBenchmarkConfig(cfg BenchmarkConfig) error {
	path := configpath.ModuleConfigPath("benchmark")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

type HistoryConfig struct {
	Entries []ui.HistoryEntry `json:"entries"`
}

func LoadHistoryConfig() (HistoryConfig, error) {
	path := configpath.ModuleConfigPath("history")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return HistoryConfig{Entries: []ui.HistoryEntry{}}, nil
		}
		return HistoryConfig{}, err
	}
	var cfg HistoryConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return HistoryConfig{}, err
	}
	return cfg, nil
}

func SaveHistoryConfig(cfg HistoryConfig) error {
	if len(cfg.Entries) > 20 {
		cfg.Entries = cfg.Entries[:20]
	}
	path := configpath.ModuleConfigPath("history")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
