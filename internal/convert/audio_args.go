package convert

import (
	"fmt"
	"strings"
)

// AudioConfig is the single input for audio argument construction shared by the
// queue executor and the command preview.
type AudioConfig struct {
	OutputExt  string
	Codec      string
	Bitrate    string
	Channels   string
	SampleRate string
	Normalize  bool
	LUFS       float64
	TruePeak   float64
}

// BuildAudioArgs returns the audio argument sequence for a conversion. The
// output container decides codec compatibility; DVD forces AC-3, WebM supports
// Opus/Vorbis, and Ogg supports Vorbis/Opus/FLAC. Copy is preserved except for
// DVD, where the container requires a re-encode.
func BuildAudioArgs(cfg AudioConfig) []string {
	ext := strings.ToLower(strings.TrimSpace(cfg.OutputExt))
	isDVD := ext == ".mpg"
	isWebM := ext == ".webm"
	isOGV := ext == ".ogv"

	codec := canonicalAudioCodec(cfg.Codec)
	if codec == "copy" && !isDVD {
		return []string{"-c:a", "copy"}
	}

	actual := codec
	switch {
	case isDVD:
		actual = "ac3"
	case isWebM:
		if codec != "libopus" && codec != "libvorbis" {
			actual = "libopus"
		}
	case isOGV:
		if codec != "libvorbis" && codec != "libopus" && codec != "flac" {
			actual = "libvorbis"
		}
	}

	args := []string{"-c:a", actual}

	if isDVD {
		// DVD standard: AC-3 stereo at 48 kHz, 192 kbps.
		return append(args, "-b:a", "192k", "-ar", "48000", "-ac", "2")
	}

	if actual != "flac" {
		if bitrate := strings.TrimSpace(cfg.Bitrate); bitrate != "" {
			args = append(args, "-b:a", bitrate)
		}
	}

	if cfg.Normalize {
		lufs := cfg.LUFS
		if lufs == 0 {
			lufs = -16
		}
		truePeak := cfg.TruePeak
		if truePeak == 0 {
			truePeak = -1.5
		}
		// One -af only: loudnorm replaces channel filtering.
		args = append(args, "-ac", "2", "-ar", "48000", "-af",
			fmt.Sprintf("loudnorm=I=%.0f:TP=%.1f:LRA=11", lufs, truePeak))
		return args
	}

	appendChannelArgs(&args, cfg.Channels)
	if rate := strings.TrimSpace(cfg.SampleRate); rate != "" && rate != "Source" {
		args = append(args, "-ar", rate)
	}
	return args
}

func appendChannelArgs(args *[]string, channels string) {
	switch channels {
	case "Mono":
		*args = append(*args, "-ac", "1")
	case "Stereo":
		*args = append(*args, "-ac", "2")
	case "5.1":
		*args = append(*args, "-ac", "6")
	case "Left to Stereo":
		*args = append(*args, "-af", "pan=stereo|c0=c0|c1=c0")
	case "Right to Stereo":
		*args = append(*args, "-af", "pan=stereo|c0=c1|c1=c1")
	case "Mix to Stereo":
		*args = append(*args, "-af", "pan=stereo|c0=0.5*c0+0.5*c1|c1=0.5*c0+0.5*c1")
	case "Swap L/R":
		*args = append(*args, "-af", "pan=stereo|c0=c1|c1=c0")
	}
}

func canonicalAudioCodec(codec string) string {
	switch strings.ToLower(strings.TrimSpace(codec)) {
	case "aac", "":
		return "aac"
	case "opus", "libopus":
		return "libopus"
	case "vorbis", "libvorbis":
		return "libvorbis"
	case "mp3", "libmp3lame":
		return "libmp3lame"
	case "ac-3", "ac3":
		return "ac3"
	case "flac":
		return "flac"
	case "copy":
		return "copy"
	default:
		return "aac"
	}
}
