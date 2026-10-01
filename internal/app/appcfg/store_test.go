package appcfg

import (
	"encoding/json"
	"testing"
)

// legacyCfg mirrors the persisted convertConfig fields touched by migration.
// convertConfig carries no json tags, so encoding/json matches on the exported
// field name - which is exactly the key set these tests build by hand.
type legacyCfg struct {
	AudioCodec string

	ForceAspect   bool
	ShowUpscale   bool
	ShowDisc      bool
	OutputAspect  string
	AspectUserSet bool
	FrameRate     string
	BitrateMode   string

	NormalizeAudio    bool
	NormalizeLUFS     float64
	NormalizeTruePeak float64

	PlayerOpen   bool
	MetadataOpen bool
	SettingsOpen bool
}

// loadCfg replays the real load path: unmarshal the persisted JSON into the
// struct (absent keys take the Go zero value), then run the migration. Passing
// hand-written struct values instead would not test the defect - the whole
// point is that absent keys zero-fill before normalization sees them.
func loadCfg(t *testing.T, raw map[string]json.RawMessage) legacyCfg {
	t.Helper()

	full, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal raw: %v", err)
	}
	var cfg legacyCfg
	if err := json.Unmarshal(full, &cfg); err != nil {
		t.Fatalf("unmarshal cfg: %v", err)
	}

	n := NormalizeConvertFields(raw,
		cfg.ForceAspect, cfg.ShowUpscale, cfg.ShowDisc,
		cfg.OutputAspect, cfg.AspectUserSet, cfg.FrameRate, cfg.BitrateMode,
		cfg.NormalizeLUFS, cfg.NormalizeTruePeak,
	)
	cfg.ForceAspect = n.ForceAspect
	cfg.ShowUpscale = n.ShowUpscale
	cfg.ShowDisc = n.ShowDisc
	cfg.OutputAspect = n.OutputAspect
	cfg.AspectUserSet = n.AspectUserSet
	cfg.FrameRate = n.FrameRate
	cfg.BitrateMode = n.BitrateMode
	cfg.NormalizeLUFS = n.NormalizeLUFS
	cfg.NormalizeTruePeak = n.NormalizeTruePeak

	cfg.PlayerOpen, cfg.MetadataOpen, cfg.SettingsOpen = NormalizeLayoutFields(
		raw, cfg.PlayerOpen, cfg.MetadataOpen, cfg.SettingsOpen,
	)
	return cfg
}

func rawFromMap(m map[string]interface{}) map[string]json.RawMessage {
	raw := make(map[string]json.RawMessage, len(m))
	for k, v := range m {
		b, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		raw[k] = b
	}
	return raw
}

// TestLoad_LegacyConfigAllKeysAbsent covers case 1: a legacy config where every
// migrated key is absent. All ten fields must adopt their defaults, and the
// loudness targets must not arrive as 0.
func TestLoad_LegacyConfigAllKeysAbsent(t *testing.T) {
	cfg := loadCfg(t, rawFromMap(map[string]interface{}{
		"SelectedFormat": "H.264",
		"AudioCodec":     "AAC",
		"NormalizeAudio": true,
	}))

	if !cfg.ForceAspect {
		t.Error("ForceAspect: absent key must adopt true, got false")
	}
	if !cfg.ShowUpscale {
		t.Error("ShowUpscale: absent key must adopt true, got false")
	}
	if !cfg.ShowDisc {
		t.Error("ShowDisc: absent key must adopt true, got false")
	}
	if cfg.OutputAspect != DefaultOutputAspect {
		t.Errorf("OutputAspect = %q, want %q", cfg.OutputAspect, DefaultOutputAspect)
	}
	if cfg.FrameRate != DefaultFrameRate {
		t.Errorf("FrameRate = %q, want %q", cfg.FrameRate, DefaultFrameRate)
	}
	if cfg.BitrateMode != DefaultBitrateMode {
		t.Errorf("BitrateMode = %q, want %q", cfg.BitrateMode, DefaultBitrateMode)
	}
	if !cfg.PlayerOpen || !cfg.MetadataOpen || !cfg.SettingsOpen {
		t.Errorf("layout = (%v,%v,%v), want all true for legacy config",
			cfg.PlayerOpen, cfg.MetadataOpen, cfg.SettingsOpen)
	}
	// This is the #15 dependency: loudnorm must never be built from 0.
	if cfg.NormalizeLUFS != DefaultNormalizeLUFS {
		t.Errorf("NormalizeLUFS = %v, want %v (legacy config must not yield 0)", cfg.NormalizeLUFS, DefaultNormalizeLUFS)
	}
	if cfg.NormalizeTruePeak != DefaultNormalizeTP {
		t.Errorf("NormalizeTruePeak = %v, want %v (legacy config must not yield 0)", cfg.NormalizeTruePeak, DefaultNormalizeTP)
	}
}

// TestLoad_ExplicitFalseBooleans covers case 2: a current config that disables
// everything by hand. Every explicit false must survive.
func TestLoad_ExplicitFalseBooleans(t *testing.T) {
	cfg := loadCfg(t, rawFromMap(map[string]interface{}{
		"ForceAspect":    false,
		"ShowUpscale":    false,
		"ShowDisc":       false,
		"PlayerOpen":     false,
		"MetadataOpen":   false,
		"SettingsOpen":   false,
		"NormalizeAudio": false,
	}))

	if cfg.ForceAspect {
		t.Error("ForceAspect: explicit false must be preserved, got true")
	}
	if cfg.ShowUpscale {
		t.Error("ShowUpscale: explicit false must be preserved, got true")
	}
	if cfg.ShowDisc {
		t.Error("ShowDisc: explicit false must be preserved, got true")
	}
	if cfg.PlayerOpen || cfg.MetadataOpen || cfg.SettingsOpen {
		t.Errorf("layout = (%v,%v,%v), want all false preserved",
			cfg.PlayerOpen, cfg.MetadataOpen, cfg.SettingsOpen)
	}
}

// TestLoad_MixedPresence covers case 3: some keys present, some absent. Each
// field must be decided independently of its siblings.
func TestLoad_MixedPresence(t *testing.T) {
	cfg := loadCfg(t, rawFromMap(map[string]interface{}{
		"ForceAspect": false, // present false -> keep false
		"ShowDisc":    true,  // present true  -> keep true
		// ShowUpscale absent -> default true
		"FrameRate":  "30",  // present valid -> keep
		"PlayerOpen": false, // present false -> keep false
		// MetadataOpen / SettingsOpen absent -> default true
	}))

	if cfg.ForceAspect {
		t.Error("ForceAspect: explicit false must survive when a sibling key is absent")
	}
	if !cfg.ShowDisc {
		t.Error("ShowDisc: explicit true must survive when a sibling key is absent")
	}
	if !cfg.ShowUpscale {
		t.Error("ShowUpscale: absent key must default to true")
	}
	if cfg.FrameRate != "30" {
		t.Errorf("FrameRate = %q, want %q", cfg.FrameRate, "30")
	}
	if cfg.PlayerOpen {
		t.Error("PlayerOpen: explicit false must survive when siblings are absent")
	}
	if !cfg.MetadataOpen || !cfg.SettingsOpen {
		t.Errorf("layout = (%v,%v), want both absent keys defaulted to true",
			cfg.MetadataOpen, cfg.SettingsOpen)
	}
}

// TestLoad_IntentionallyFullyCollapsed covers case 4 - the regression this
// change exists to fix. Under the old all-false heuristic this layout was
// silently rewritten to all-open on every load.
func TestLoad_IntentionallyFullyCollapsed(t *testing.T) {
	cfg := loadCfg(t, rawFromMap(map[string]interface{}{
		"PlayerOpen":   false,
		"MetadataOpen": false,
		"SettingsOpen": false,
		"AudioCodec":   "FLAC",
	}))

	if cfg.PlayerOpen || cfg.MetadataOpen || cfg.SettingsOpen {
		t.Errorf("layout = (%v,%v,%v), want all false preserved - a deliberate collapse "+
			"must not be reinterpreted as a legacy config",
			cfg.PlayerOpen, cfg.MetadataOpen, cfg.SettingsOpen)
	}
	if cfg.AudioCodec != "FLAC" {
		t.Errorf("AudioCodec = %q, want FLAC (unrelated fields must be untouched)", cfg.AudioCodec)
	}
}

// TestLoad_CurrentConfigAllFieldsPresent covers case 5: a fully-populated modern
// config must pass through unchanged.
func TestLoad_CurrentConfigAllFieldsPresent(t *testing.T) {
	cfg := loadCfg(t, rawFromMap(map[string]interface{}{
		"ForceAspect":       false,
		"ShowUpscale":       false,
		"ShowDisc":          false,
		"OutputAspect":      "16:9",
		"AspectUserSet":     true,
		"FrameRate":         "30",
		"BitrateMode":       "VBR",
		"NormalizeLUFS":     -14.0,
		"NormalizeTruePeak": -2.0,
		"PlayerOpen":        true,
		"MetadataOpen":      true,
		"SettingsOpen":      false,
	}))

	if cfg.ForceAspect || cfg.ShowUpscale || cfg.ShowDisc {
		t.Error("explicit false booleans must survive")
	}
	if cfg.OutputAspect != "16:9" {
		t.Errorf("OutputAspect = %q, want 16:9", cfg.OutputAspect)
	}
	if !cfg.AspectUserSet {
		t.Error("AspectUserSet must survive when a custom aspect is set")
	}
	if cfg.FrameRate != "30" {
		t.Errorf("FrameRate = %q, want 30", cfg.FrameRate)
	}
	if cfg.BitrateMode != "VBR" {
		t.Errorf("BitrateMode = %q, want VBR", cfg.BitrateMode)
	}
	if cfg.NormalizeLUFS != -14.0 || cfg.NormalizeTruePeak != -2.0 {
		t.Errorf("loudness = (%v,%v), want (-14,-2)",
			cfg.NormalizeLUFS, cfg.NormalizeTruePeak)
	}
	if !cfg.PlayerOpen || !cfg.MetadataOpen || cfg.SettingsOpen {
		t.Errorf("layout = (%v,%v,%v), want (true,true,false)",
			cfg.PlayerOpen, cfg.MetadataOpen, cfg.SettingsOpen)
	}
}

// TestLoad_NormalizationExplicitlyDisabled covers case 6: normalization off must
// stay off, while the targets remain valid for when the user re-enables it.
func TestLoad_NormalizationExplicitlyDisabled(t *testing.T) {
	cfg := loadCfg(t, rawFromMap(map[string]interface{}{
		"NormalizeAudio":    false,
		"NormalizeLUFS":     -23.0,
		"NormalizeTruePeak": -3.0,
	}))

	if cfg.NormalizeAudio {
		t.Error("NormalizeAudio: explicit false must stay false")
	}
	if cfg.NormalizeLUFS != -23.0 || cfg.NormalizeTruePeak != -3.0 {
		t.Errorf("loudness = (%v,%v), want (-23,-3) preserved while disabled",
			cfg.NormalizeLUFS, cfg.NormalizeTruePeak)
	}
}

// TestLoad_LoudnessZeroRepaired covers a persisted 0, which is unusable for
// loudnorm. A present-but-zero value is as broken as an absent one.
func TestLoad_LoudnessZeroRepaired(t *testing.T) {
	cfg := loadCfg(t, rawFromMap(map[string]interface{}{
		"NormalizeAudio":    true,
		"NormalizeLUFS":     0.0,
		"NormalizeTruePeak": 0.0,
	}))

	if cfg.NormalizeLUFS != DefaultNormalizeLUFS {
		t.Errorf("NormalizeLUFS = %v, want repair to %v", cfg.NormalizeLUFS, DefaultNormalizeLUFS)
	}
	if cfg.NormalizeTruePeak != DefaultNormalizeTP {
		t.Errorf("NormalizeTruePeak = %v, want repair to %v", cfg.NormalizeTruePeak, DefaultNormalizeTP)
	}
}

// TestLoad_InvalidValuesStillNormalized ensures the pre-existing value-based
// sanitisation was not lost in the migration.
//
// FrameRate is only ever sanitised for emptiness - a non-empty unrecognised
// value has always been passed through, and widening that check is out of
// scope for this change. BitrateMode is the field that carries an allowlist.
func TestLoad_InvalidValuesStillNormalized(t *testing.T) {
	cfg := loadCfg(t, rawFromMap(map[string]interface{}{
		"BitrateMode": "Nonsense",
	}))

	if cfg.BitrateMode != DefaultBitrateMode {
		t.Errorf("BitrateMode = %q, want allowlist fallback %q", cfg.BitrateMode, DefaultBitrateMode)
	}

	empty := loadCfg(t, rawFromMap(map[string]interface{}{"FrameRate": ""}))
	if empty.FrameRate != DefaultFrameRate {
		t.Errorf("FrameRate = %q, want empty fallback %q", empty.FrameRate, DefaultFrameRate)
	}
}

// migratedFields is the exact set this change owns. Idempotency must be
// asserted over these fields only - unrelated config values are not
// round-tripped by this test and would otherwise compare as zero.
func migratedFields(c legacyCfg) legacyCfg {
	return legacyCfg{
		ForceAspect:       c.ForceAspect,
		ShowUpscale:       c.ShowUpscale,
		ShowDisc:          c.ShowDisc,
		OutputAspect:      c.OutputAspect,
		AspectUserSet:     c.AspectUserSet,
		FrameRate:         c.FrameRate,
		BitrateMode:       c.BitrateMode,
		NormalizeLUFS:     c.NormalizeLUFS,
		NormalizeTruePeak: c.NormalizeTruePeak,
		PlayerOpen:        c.PlayerOpen,
		MetadataOpen:      c.MetadataOpen,
		SettingsOpen:      c.SettingsOpen,
	}
}

// TestLoad_CaseInsensitiveKeys ensures a hand-edited config using lowercase keys
// is not mistaken for a legacy config. encoding/json matches field names
// case-insensitively, so the migration must too.
func TestLoad_CaseInsensitiveKeys(t *testing.T) {
	cfg := loadCfg(t, rawFromMap(map[string]interface{}{
		"forceAspect":   false,
		"normalizeLUFS": -20.0,
		"playerOpen":    false,
	}))

	if cfg.ForceAspect {
		t.Error("ForceAspect: lowercase key must be honoured, not treated as absent")
	}
	if cfg.NormalizeLUFS != -20.0 {
		t.Errorf("NormalizeLUFS = %v, want -20 from lowercase key", cfg.NormalizeLUFS)
	}
	if cfg.PlayerOpen {
		t.Error("PlayerOpen: lowercase key must be honoured, not treated as absent")
	}
}

// TestLoad_Idempotent verifies the migration is reversible and deterministic:
// feeding a migrated config back through the loader must be a fixed point.
func TestLoad_Idempotent(t *testing.T) {
	seeds := map[string]map[string]interface{}{
		"legacy (all absent)": {"AudioCodec": "AAC"},
		"fully collapsed":     {"PlayerOpen": false, "MetadataOpen": false, "SettingsOpen": false},
		"mixed presence":      {"ForceAspect": false, "ShowDisc": true, "FrameRate": "30"},
		"all fields present":  {"ForceAspect": false, "ShowUpscale": false, "ShowDisc": false, "OutputAspect": "16:9", "AspectUserSet": true, "FrameRate": "30", "BitrateMode": "VBR", "NormalizeLUFS": -14.0, "NormalizeTruePeak": -2.0, "PlayerOpen": true, "MetadataOpen": true, "SettingsOpen": false},
		"loudness zero":       {"NormalizeAudio": true, "NormalizeLUFS": 0.0, "NormalizeTruePeak": 0.0},
	}

	for name, seed := range seeds {
		t.Run(name, func(t *testing.T) {
			first := loadCfg(t, rawFromMap(seed))

			// Re-persist exactly what the first pass resolved, as the app
			// would on save, then reload.
			roundTrip := map[string]interface{}{
				"ForceAspect":       first.ForceAspect,
				"ShowUpscale":       first.ShowUpscale,
				"ShowDisc":          first.ShowDisc,
				"OutputAspect":      first.OutputAspect,
				"AspectUserSet":     first.AspectUserSet,
				"FrameRate":         first.FrameRate,
				"BitrateMode":       first.BitrateMode,
				"NormalizeLUFS":     first.NormalizeLUFS,
				"NormalizeTruePeak": first.NormalizeTruePeak,
				"PlayerOpen":        first.PlayerOpen,
				"MetadataOpen":      first.MetadataOpen,
				"SettingsOpen":      first.SettingsOpen,
			}
			second := loadCfg(t, rawFromMap(roundTrip))

			if migratedFields(first) != migratedFields(second) {
				t.Errorf("migration is not idempotent:\n first: %+v\nsecond: %+v",
					migratedFields(first), migratedFields(second))
			}
		})
	}
}

// TestLoad_Deterministic guards against map-iteration order leaking into the
// result.
func TestLoad_Deterministic(t *testing.T) {
	raw := rawFromMap(map[string]interface{}{"AudioCodec": "AAC"})
	first := migratedFields(loadCfg(t, raw))
	for i := 0; i < 25; i++ {
		if got := migratedFields(loadCfg(t, raw)); got != first {
			t.Fatalf("run %d differs:\n%+v\n%+v", i, first, got)
		}
	}
}
