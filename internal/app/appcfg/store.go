package appcfg

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeakTechnologies/VideoTools/internal/app/configpath"
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

// Convert defaults applied when a key is absent from the persisted config.
// A legacy config written before a field existed unmarshals to the Go zero
// value, which is almost never the intended default ("" , false, 0). Migration
// is therefore driven by key presence, never by the zero value alone.
const (
	DefaultForceAspect   = true
	DefaultShowUpscale   = true
	DefaultShowDisc      = true
	DefaultOutputAspect  = "Source"
	DefaultFrameRate     = "Source"
	DefaultBitrateMode   = "CBR"
	DefaultNormalizeLUFS = -16.0
	DefaultNormalizeTP   = -1.5

	// Layout fields are not normalised by presence alone - see
	// NormalizeLayoutFields for why an explicit false must survive.
	DefaultPlayerOpen   = true
	DefaultMetadataOpen = true
	DefaultSettingsOpen = true
)

type ConvertNormalizedFields struct {
	ForceAspect   bool
	ShowUpscale   bool
	ShowDisc      bool
	OutputAspect  string
	AspectUserSet bool
	FrameRate     string
	BitrateMode   string

	NormalizeLUFS     float64
	NormalizeTruePeak float64
}

// keyPresent reports whether the persisted config carried the named key.
//
// JSON key matching must mirror encoding/json, which falls back to a
// case-insensitive field-name match. A config written by hand as
// "forceAspect" populates the struct field but would be missed by a
// case-sensitive lookup, silently re-triggering migration over a value the
// user had already set.
func keyPresent(raw map[string]json.RawMessage, key string) bool {
	if _, ok := raw[key]; ok {
		return true
	}
	for k := range raw {
		if strings.EqualFold(k, key) {
			return true
		}
	}
	return false
}

func NormalizeConvertFields(raw map[string]json.RawMessage, forceAspect bool, showUpscale bool, showDisc bool, outputAspect string, aspectUserSet bool, frameRate string, bitrateMode string, lufs float64, truePeak float64) ConvertNormalizedFields {
	n := ConvertNormalizedFields{
		ForceAspect:   forceAspect,
		ShowUpscale:   showUpscale,
		ShowDisc:      showDisc,
		OutputAspect:  outputAspect,
		AspectUserSet: aspectUserSet,
		FrameRate:     frameRate,
		BitrateMode:   bitrateMode,
	}

	// Audio normalization targets. These are float64 fields introduced after the
	// first config format, so a legacy config leaves them at 0. A loudnorm
	// filter built from 0 would target 0 LUFS and 0 dBTP; guard on the numeric
	// value rather than on presence alone, because a present-but-zero value is
	// just as unusable as an absent one.
	n.NormalizeLUFS = normalizeLoudness(raw, keyNormalizeLUFS, lufs, DefaultNormalizeLUFS)
	n.NormalizeTruePeak = normalizeLoudness(raw, keyNormalizeTP, truePeak, DefaultNormalizeTP)

	// Booleans whose default is true: migrate only when the key is absent.
	// A present key is authoritative even when false - the user set it.
	if !keyPresent(raw, "ForceAspect") {
		n.ForceAspect = DefaultForceAspect
	}
	if !keyPresent(raw, "ShowUpscale") {
		n.ShowUpscale = DefaultShowUpscale
	}
	if !keyPresent(raw, "ShowDisc") {
		n.ShowDisc = DefaultShowDisc
	}

	if n.OutputAspect == "" || strings.EqualFold(n.OutputAspect, DefaultOutputAspect) {
		n.OutputAspect = DefaultOutputAspect
		n.AspectUserSet = false
	} else if !n.AspectUserSet {
		n.OutputAspect = DefaultOutputAspect
		n.AspectUserSet = false
	}

	if n.FrameRate == "" {
		n.FrameRate = DefaultFrameRate
	}

	switch n.BitrateMode {
	case "CRF", "CBR", "VBR", "Target Size":
	default:
		n.BitrateMode = DefaultBitrateMode
	}

	return n
}

// keyNormalizeLUFS / keyNormalizeTruePeak are the persisted JSON key names for
// the loudness targets.
const (
	keyNormalizeLUFS = "NormalizeLUFS"
	keyNormalizeTP   = "NormalizeTruePeak"
)

// normalizeLoudness resolves a loudness target, preferring the persisted value.
//
// Absent key -> the default. Present key -> keep the persisted value, unless
// that value is 0 (or otherwise unusable), in which case fall back to the
// default. This keeps the migration deterministic and idempotent: a migrated
// config re-normalizes to itself.
func normalizeLoudness(raw map[string]json.RawMessage, key string, value float64, def float64) float64 {
	if !keyPresent(raw, key) {
		return def
	}
	if value == 0 {
		return def
	}
	return value
}

// NormalizeLayoutFields resolves the three persisted layout booleans.
//
// The original migration assumed "all three false" meant "pre-layout config,
// default to all-open". That conflates two different states:
//
//   - a legacy config written before these fields existed, where all three keys
//     are genuinely absent and every field reads false because of the Go zero
//     value;
//   - a current config where the user deliberately collapsed all three panels,
//     where all three keys are present and false is a real user choice.
//
// The second case silently overwrites a deliberate choice on every load, so the
// migration is keyed on presence instead. Absent keys adopt the default;
// present keys are authoritative regardless of value.
func NormalizeLayoutFields(raw map[string]json.RawMessage, playerOpen, metadataOpen, settingsOpen bool) (player, metadata, settings bool) {
	player = playerOpen
	metadata = metadataOpen
	settings = settingsOpen

	if !keyPresent(raw, "PlayerOpen") {
		player = DefaultPlayerOpen
	}
	if !keyPresent(raw, "MetadataOpen") {
		metadata = DefaultMetadataOpen
	}
	if !keyPresent(raw, "SettingsOpen") {
		settings = DefaultSettingsOpen
	}

	return player, metadata, settings
}
