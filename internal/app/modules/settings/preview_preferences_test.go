package settings

import (
	"encoding/json"
	"testing"
)

func TestAutomaticPreviewPreferenceLegacyAndExplicitValues(t *testing.T) {
	for _, tc := range []struct {
		name, json string
		enabled    bool
	}{
		{"legacy", `{"ShowTooltips":true,"PlayerFont":"sans"}`, true},
		{"explicit enabled", `{"DisableAutomaticPreviews":false}`, true},
		{"explicit disabled", `{"DisableAutomaticPreviews":true}`, false},
		{"null retains default", `{"DisableAutomaticPreviews":null}`, true},
		{"case insensitive", `{"disableautomaticpreviews":true}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var prefs PrefsConfig
			if err := json.Unmarshal([]byte(tc.json), &prefs); err != nil {
				t.Fatal(err)
			}
			if prefs.AutomaticPreviewsEnabled() != tc.enabled {
				t.Fatal("preview preference changed meaning")
			}
			encoded, err := json.Marshal(prefs)
			if err != nil {
				t.Fatal(err)
			}
			var restored PrefsConfig
			if err := json.Unmarshal(encoded, &restored); err != nil {
				t.Fatal(err)
			}
			if restored.AutomaticPreviewsEnabled() != tc.enabled {
				t.Fatal("preview preference did not survive save/load")
			}
			if tc.name == "legacy" && (!restored.ShowTooltips || restored.PlayerFont != "sans") {
				t.Fatal("legacy preferences lost")
			}
		})
	}
}
