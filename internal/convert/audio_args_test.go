package convert

import (
	"slices"
	"testing"
)

func TestBuildAudioArgs(t *testing.T) {
	cases := []struct {
		name string
		cfg  AudioConfig
		want []string
	}{
		{
			name: "DVD forces AC-3 even for copy",
			cfg:  AudioConfig{OutputExt: ".mpg", Codec: "Copy", Bitrate: "320k", Channels: "5.1", SampleRate: "44100"},
			want: []string{"-c:a", "ac3", "-b:a", "192k", "-ar", "48000", "-ac", "2"},
		},
		{
			name: "DVD AAC forced AC-3",
			cfg:  AudioConfig{OutputExt: ".mpg", Codec: "AAC"},
			want: []string{"-c:a", "ac3", "-b:a", "192k", "-ar", "48000", "-ac", "2"},
		},
		{
			name: "WebM AAC becomes Opus",
			cfg:  AudioConfig{OutputExt: ".webm", Codec: "AAC", Bitrate: "128k"},
			want: []string{"-c:a", "libopus", "-b:a", "128k"},
		},
		{
			name: "WebM Vorbis stays Vorbis",
			cfg:  AudioConfig{OutputExt: ".webm", Codec: "Vorbis", Bitrate: "128k"},
			want: []string{"-c:a", "libvorbis", "-b:a", "128k"},
		},
		{
			name: "WebM empty becomes Opus",
			cfg:  AudioConfig{OutputExt: ".webm"},
			want: []string{"-c:a", "libopus"},
		},
		{
			name: "Ogg AAC becomes Vorbis",
			cfg:  AudioConfig{OutputExt: ".ogv", Codec: "AAC", Bitrate: "160k"},
			want: []string{"-c:a", "libvorbis", "-b:a", "160k"},
		},
		{
			name: "Ogg Vorbis stays Vorbis",
			cfg:  AudioConfig{OutputExt: ".ogv", Codec: "Vorbis"},
			want: []string{"-c:a", "libvorbis"},
		},
		{
			name: "Ogg FLAC stays FLAC without bitrate",
			cfg:  AudioConfig{OutputExt: ".ogv", Codec: "FLAC", Bitrate: "320k"},
			want: []string{"-c:a", "flac"},
		},
		{
			name: "MKV AAC with channels and sample rate",
			cfg:  AudioConfig{OutputExt: ".mkv", Codec: "AAC", Bitrate: "192k", Channels: "Stereo", SampleRate: "44100"},
			want: []string{"-c:a", "aac", "-b:a", "192k", "-ac", "2", "-ar", "44100"},
		},
		{
			name: "normalisation emits one filter and skips channel mixing",
			cfg:  AudioConfig{OutputExt: ".mkv", Codec: "AAC", Channels: "Mono", SampleRate: "44100", Normalize: true, LUFS: -18, TruePeak: -2},
			want: []string{"-c:a", "aac", "-ac", "2", "-ar", "48000", "-af", "loudnorm=I=-18:TP=-2.0:LRA=11"},
		},
		{
			name: "normalisation defaults",
			cfg:  AudioConfig{OutputExt: ".mp4", Codec: "AAC", Normalize: true},
			want: []string{"-c:a", "aac", "-ac", "2", "-ar", "48000", "-af", "loudnorm=I=-16:TP=-1.5:LRA=11"},
		},
		{
			name: "copy preserved for non-DVD",
			cfg:  AudioConfig{OutputExt: ".mkv", Codec: "Copy", Bitrate: "192k", Channels: "Stereo", SampleRate: "48000"},
			want: []string{"-c:a", "copy"},
		},
		{
			name: "copy preserved for WebM",
			cfg:  AudioConfig{OutputExt: ".webm", Codec: "Copy"},
			want: []string{"-c:a", "copy"},
		},
		{
			name: "FLAC omits bitrate in generic container",
			cfg:  AudioConfig{OutputExt: ".mkv", Codec: "FLAC", Bitrate: "320k"},
			want: []string{"-c:a", "flac"},
		},
		{
			name: "unknown codec falls back to AAC",
			cfg:  AudioConfig{OutputExt: ".mkv", Codec: "weird"},
			want: []string{"-c:a", "aac"},
		},
		{
			name: "channel pan filter",
			cfg:  AudioConfig{OutputExt: ".mkv", Codec: "AAC", Channels: "Swap L/R"},
			want: []string{"-c:a", "aac", "-af", "pan=stereo|c0=c1|c1=c0"},
		},
		{
			name: "source sample rate is omitted",
			cfg:  AudioConfig{OutputExt: ".mkv", Codec: "AAC", SampleRate: "Source"},
			want: []string{"-c:a", "aac"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildAudioArgs(tc.cfg)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("BuildAudioArgs(%+v) = %v, want %v", tc.cfg, got, tc.want)
			}
		})
	}
}
