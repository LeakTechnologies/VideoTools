package appcfg

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"git.leaktechnologies.dev/stu/VideoTools/internal/app/configpath"
)

func LoadModuleJSON(name string, out interface{}) (map[string]json.RawMessage, error) {
	path := configpath.ModuleConfigPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(data, &raw)
	if err := json.Unmarshal(data, out); err != nil {
		return nil, err
	}
	return raw, nil
}

func SaveModuleJSON(name string, in interface{}) error {
	path := configpath.ModuleConfigPath(name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

type ConvertNormalizedFields struct {
	ForceAspect bool
	ShowUpscale bool
	ShowAuthor  bool
	ShowRip     bool
	ShowBluRay  bool
	OutputAspect  string
	AspectUserSet bool
	FrameRate     string
	BitrateMode   string
}

func NormalizeConvertFields(raw map[string]json.RawMessage, forceAspect bool, showUpscale bool, showAuthor bool, showRip bool, showBluRay bool, outputAspect string, aspectUserSet bool, frameRate string, bitrateMode string) ConvertNormalizedFields {
	n := ConvertNormalizedFields{
		ForceAspect:   forceAspect,
		ShowUpscale:   showUpscale,
		ShowAuthor:    showAuthor,
		ShowRip:       showRip,
		ShowBluRay:    showBluRay,
		OutputAspect:  outputAspect,
		AspectUserSet: aspectUserSet,
		FrameRate:     frameRate,
		BitrateMode:   bitrateMode,
	}

	if _, ok := raw["ForceAspect"]; !ok {
		n.ForceAspect = true
	}
	if _, ok := raw["ShowUpscale"]; !ok {
		n.ShowUpscale = true
	}
	if _, ok := raw["ShowAuthor"]; !ok {
		n.ShowAuthor = true
	}
	if _, ok := raw["ShowRip"]; !ok {
		n.ShowRip = true
	}
	if _, ok := raw["ShowBluRay"]; !ok {
		n.ShowBluRay = true
	}

	if n.OutputAspect == "" || strings.EqualFold(n.OutputAspect, "Source") {
		n.OutputAspect = "Source"
		n.AspectUserSet = false
	} else if !n.AspectUserSet {
		n.OutputAspect = "Source"
		n.AspectUserSet = false
	}

	if n.FrameRate == "" {
		n.FrameRate = "Source"
	}

	switch n.BitrateMode {
	case "CRF", "CBR", "VBR", "Target Size":
	default:
		n.BitrateMode = "CBR"
	}

	return n
}
