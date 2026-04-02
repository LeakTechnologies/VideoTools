package ifo

import "testing"

func TestAudioCodingModeFromCodec(t *testing.T) {
	cases := []struct {
		codec string
		want  uint8
	}{
		// AC-3 family → 0
		{"ac3", 0},
		{"AC3", 0},
		{"eac3", 0},
		{"truehd", 0},
		// MPEG audio → 2
		{"mp2", 2},
		{"mp3", 2},
		// LPCM family → 3
		{"lpcm", 3},
		{"pcm_s16be", 3},
		{"pcm_s24be", 3},
		{"pcm_s16le", 3},
		{"pcm_s24le", 3},
		// DTS family → 4
		{"dts", 4},
		{"dts-hd", 4},
		{"dtshd", 4},
		// Unknown → 0 (default AC-3)
		{"aac", 0},
		{"", 0},
		{"flac", 0},
	}
	for _, c := range cases {
		got := AudioCodingModeFromCodec(c.codec)
		if got != c.want {
			t.Errorf("AudioCodingModeFromCodec(%q) = %d, want %d", c.codec, got, c.want)
		}
	}
}

func TestLanguageCodeBytes(t *testing.T) {
	cases := []struct {
		lang string
		want [2]byte
	}{
		{"en", [2]byte{'e', 'n'}},
		{"fr", [2]byte{'f', 'r'}},
		{"de", [2]byte{'d', 'e'}},
		{"ja", [2]byte{'j', 'a'}},
		{"", [2]byte{0, 0}},
		{"x", [2]byte{0, 0}},
		{"eng", [2]byte{'e', 'n'}}, // truncates to first two bytes
	}
	for _, c := range cases {
		got := LanguageCodeBytes(c.lang)
		if got != c.want {
			t.Errorf("LanguageCodeBytes(%q) = %v, want %v", c.lang, got, c.want)
		}
	}
}

func TestNumChannelsField(t *testing.T) {
	cases := []struct {
		channels int
		want     uint8
	}{
		{0, 0}, // degenerate → mono
		{1, 0}, // mono
		{2, 1}, // stereo
		{6, 5}, // 5.1
		{8, 7}, // 7.1
		{9, 7}, // clamped to max 7
	}
	for _, c := range cases {
		got := NumChannelsField(c.channels)
		if got != c.want {
			t.Errorf("NumChannelsField(%d) = %d, want %d", c.channels, got, c.want)
		}
	}
}
