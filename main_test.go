package main

import "testing"

// TestNormalizeLoudnessFromJob guards the silent-zero class of bug: a queued
// job whose loudness target is missing or unusable must never reach ffmpeg as 0,
// because a loudnorm filter built from 0 targets 0 LUFS and 0 dBTP.
func TestNormalizeLoudnessFromJob(t *testing.T) {
	tests := []struct {
		name string
		cfg  map[string]interface{}
		key  string
		def  float64
		want float64
	}{
		{
			name: "present float64 is used",
			cfg:  map[string]interface{}{"normalizeLUFS": -14.0},
			key:  "normalizeLUFS",
			def:  -16.0,
			want: -14.0,
		},
		{
			name: "absent key falls back to default",
			cfg:  map[string]interface{}{"audioCodec": "AAC"},
			key:  "normalizeLUFS",
			def:  -16.0,
			want: -16.0,
		},
		{
			name: "nil map falls back to default",
			cfg:  nil,
			key:  "normalizeLUFS",
			def:  -16.0,
			want: -16.0,
		},
		{
			name: "explicit zero is repaired",
			cfg:  map[string]interface{}{"normalizeLUFS": 0.0},
			key:  "normalizeLUFS",
			def:  -16.0,
			want: -16.0,
		},
		{
			name: "wrong type falls back to default",
			cfg:  map[string]interface{}{"normalizeLUFS": "-14"},
			key:  "normalizeLUFS",
			def:  -16.0,
			want: -16.0,
		},
		{
			name: "true peak default",
			cfg:  map[string]interface{}{"normalizeTruePeak": -2.0},
			key:  "normalizeTruePeak",
			def:  -1.5,
			want: -2.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeLoudnessFromJob(tt.cfg, tt.key, tt.def); got != tt.want {
				t.Errorf("normalizeLoudnessFromJob(%v, %q, %v) = %v, want %v",
					tt.cfg, tt.key, tt.def, got, tt.want)
			}
		})
	}
}

// TestNormalizeLoudnessFromJobJSONRoundTrip mirrors how the queue persists jobs:
// JSON numbers decode into interface{} as float64. A job enqueued by a build
// predating the loudness fields has no key at all and must still resolve to a
// usable target rather than 0.
func TestNormalizeLoudnessFromJobJSONRoundTrip(t *testing.T) {
	cfg := map[string]interface{}{"normalizeLUFS": -14.0, "normalizeTruePeak": -2.0}

	if got := normalizeLoudnessFromJob(cfg, "normalizeLUFS", -16.0); got != -14.0 {
		t.Errorf("enqueued value = %v, want -14", got)
	}

	legacy := map[string]interface{}{"audioCodec": "AAC"}
	if got := normalizeLoudnessFromJob(legacy, "normalizeLUFS", -16.0); got != -16.0 {
		t.Errorf("legacy job = %v, want default -16", got)
	}
}