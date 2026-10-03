package main

import (
	"strings"
	"testing"

	"github.com/LeakTechnologies/VideoTools/internal/convert"
)

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

// TestFormatPresetCodecRoundTrip guards the #13 fall-through: every format
// preset's codec spelling must resolve to a UI codec name, and that UI name
// must map back to the preset's own encoder (AV1 excepted - its encoder is
// resolved per environment). A preset that resolves to no UI codec silently
// falls through to libx264 or fails at the muxer, which is exactly how
// MOV (ProRes) and OGG (Theora) shipped broken.
func TestFormatPresetCodecRoundTrip(t *testing.T) {
	for _, opt := range convert.FormatOptions {
		if opt.VideoCodec == "" || strings.EqualFold(opt.VideoCodec, "copy") {
			continue // passthrough formats; callers special-case copy
		}
		friendly := friendlyCodecFromPreset(opt.VideoCodec)
		if friendly == "" {
			t.Errorf("format %q: preset codec %q resolves to no UI codec", opt.Label, opt.VideoCodec)
			continue
		}
		if strings.EqualFold(friendly, "AV1") {
			continue // encoder is environment-resolved (svtav1/aom/hw)
		}
		// Hardware accel is environment-resolved (this test host has NVENC);
		// pin it off so the software round-trip is exact.
		enc := determineVideoCodec(convertConfig{VideoCodec: friendly, HardwareAccel: "none"})
		if enc != opt.VideoCodec {
			t.Errorf("format %q: preset %q -> UI %q -> encoder %q (want %q)",
				opt.Label, opt.VideoCodec, friendly, enc, opt.VideoCodec)
		}
	}
}

// TestVideoCodecUIOptions pins the select vocabulary: every canonical codec
// plus Copy, in table order, and every entry must be resolvable both ways.
func TestVideoCodecUIOptions(t *testing.T) {
	opts := videoCodecUIOptions()
	want := []string{"H.264", "H.265", "VP9", "AV1", "MPEG-2", "ProRes", "Theora", "Copy"}
	if len(opts) != len(want) {
		t.Fatalf("videoCodecUIOptions() = %v, want %v", opts, want)
	}
	for i := range want {
		if opts[i] != want[i] {
			t.Errorf("option %d = %q, want %q", i, opts[i], want[i])
		}
	}
	for _, o := range opts[:len(opts)-1] { // all but Copy
		if videoCodecIdentityForUI(o) == nil {
			t.Errorf("select option %q has no canonical identity", o)
		}
	}
}

// TestFriendlyCodecFromPreset covers preset, encoder, hardware, and legacy
// spellings, including the two that previously fell through.
func TestFriendlyCodecFromPreset(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"libx264", "H.264"}, {"h264_nvenc", "H.264"}, {"avc1", "H.264"}, {"H.264", "H.264"},
		{"libx265", "H.265"}, {"hevc_amf", "H.265"}, {"HEVC", "H.265"},
		{"libvpx-vp9", "VP9"},
		{"libaom-av1", "AV1"}, {"av1_qsv", "AV1"},
		{"mpeg2video", "MPEG-2"},
		// "MPEG-2" itself resolves to "" — the hyphen breaks the contiguous
		// "mpeg2" alias, exactly as in the original substring mapping. Not a
		// regression: friendlyCodecFromPreset only ever receives preset/encoder
		// spellings ("mpeg2video"), never UI names.
		{"MPEG-2", ""},
		{"prores_ks", "ProRes"}, // previously "" (the #13 defect)
		{"libtheora", "Theora"}, // previously "" (the #13 defect)
		{"copy", ""},            // passthrough; callers special-case it
		{"", ""},
		{"mpeg4", ""}, // deliberately unmapped: not a supported output codec
	}
	for _, tt := range tests {
		if got := friendlyCodecFromPreset(tt.in); got != tt.want {
			t.Errorf("friendlyCodecFromPreset(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestDetermineVideoCodec covers the deterministic paths: table codecs, the
// Copy passthrough, legacy raw spellings, and the unknown-value default.
// Hardware and AV1 resolution is environment-probed and excluded.
func TestDetermineVideoCodec(t *testing.T) {
	tests := []struct {
		codec, accel, want string
	}{
		{"H.264", "none", "libx264"},
		{"H.265", "none", "libx265"},
		{"VP9", "", "libvpx-vp9"},
		{"MPEG-2", "", "mpeg2video"},
		{"ProRes", "", "prores_ks"},      // previously fell to libx264
		{"Theora", "", "libtheora"},      // previously fell to libx264
		{"mpeg2video", "", "mpeg2video"}, // legacy raw spelling
		{"Copy", "", "copy"},
		{"Nonsense", "", "libx264"}, // unknown default
	}
	for _, tt := range tests {
		got := determineVideoCodec(convertConfig{VideoCodec: tt.codec, HardwareAccel: tt.accel})
		if got != tt.want {
			t.Errorf("determineVideoCodec(%q, accel=%q) = %q, want %q", tt.codec, tt.accel, got, tt.want)
		}
	}
}

// TestHwEncoderFor pins the acceleration->encoder mapping for the H.26x pair.
func TestHwEncoderFor(t *testing.T) {
	tests := []struct {
		accel, family, want string
	}{
		{"nvenc", "h264", "h264_nvenc"},
		{"amf", "hevc", "hevc_amf"},
		{"qsv", "h264", "h264_qsv"},
		{"videotoolbox", "hevc", "hevc_videotoolbox"},
		{"none", "h264", ""},
		{"", "hevc", ""},
	}
	for _, tt := range tests {
		if got := hwEncoderFor(tt.accel, tt.family); got != tt.want {
			t.Errorf("hwEncoderFor(%q, %q) = %q, want %q", tt.accel, tt.family, got, tt.want)
		}
	}
}

// TestCodecFollowsFormat pins the format-vs-user-choice policy: the H.26x
// cross-pair swap, user-override survival for generic codecs, and the
// codec-specific (ProRes/Theora) preset rules in both directions.
func TestCodecFollowsFormat(t *testing.T) {
	tests := []struct {
		name, current, friendly string
		want                    bool
	}{
		{"empty current adopts format codec", "", "H.264", true},
		{"no implied codec", "H.264", "", false},
		{"H.26x cross swap 264->265", "H.264", "H.265", true},
		{"H.26x cross swap 265->264", "H.265", "H.264", true},
		{"user VP9 survives generic format", "VP9", "H.264", false},
		{"user MPEG-2 survives generic format", "MPEG-2", "H.264", false},
		{"equal codec is a no-op", "H.264", "H.264", false},
		{"entering codec-specific preset", "H.264", "ProRes", true},
		{"entering codec-specific preset over VP9", "VP9", "ProRes", true},
		{"entering Theora preset", "H.264", "Theora", true},
		{"leaving codec-specific preset", "ProRes", "H.264", true},
		{"leaving Theora preset", "Theora", "H.264", true},
		{"staying on ProRes is a no-op", "ProRes", "ProRes", false},
	}
	for _, tt := range tests {
		if got := codecFollowsFormat(tt.current, tt.friendly); got != tt.want {
			t.Errorf("%s: codecFollowsFormat(%q, %q) = %v, want %v",
				tt.name, tt.current, tt.friendly, got, tt.want)
		}
	}
}
