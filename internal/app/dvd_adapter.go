package app

import "git.leaktechnologies.dev/stu/VideoTools/internal/convert"

// DVDConvertConfig wraps the convert.convertConfig for DVD-specific operations
// This adapter allows main.go to work with the convert package without refactoring
type DVDConvertConfig struct {
	cfg convert.ConvertConfig
}

// NewDVDConfig creates a new DVD-NTSC preset configuration
func NewDVDConfig() *DVDConvertConfig {
	return &DVDConvertConfig{
		cfg: convert.DVDNTSCPreset(),
	}
}

// GetFFmpegArgs builds the complete FFmpeg command arguments for DVD encoding
// This is the main interface that main.go should use for DVD conversions
func (d *DVDConvertConfig) GetFFmpegArgs(inputPath, outputPath string, videoWidth, videoHeight int, videoFramerate float64, audioSampleRate int, isProgressive bool) []string {
	// Create a minimal videoSource for passing to BuildDVDFFmpegArgs
	tempSrc := &convert.VideoSource{
		Width:      videoWidth,
		Height:     videoHeight,
		FrameRate:  videoFramerate,
		AudioRate:  audioSampleRate,
		FieldOrder: fieldOrderFromProgressive(isProgressive),
	}

	return convert.BuildDVDFFmpegArgs(inputPath, outputPath, d.cfg, tempSrc)
}

// ValidateForDVD performs all DVD validation checks
// Returns a list of validation warnings/errors
func (d *DVDConvertConfig) ValidateForDVD(videoWidth, videoHeight int, videoFramerate float64, audioSampleRate int, isProgressive bool) []convert.DVDValidationWarning {
	tempSrc := &convert.VideoSource{
		Width:      videoWidth,
		Height:     videoHeight,
		FrameRate:  videoFramerate,
		AudioRate:  audioSampleRate,
		FieldOrder: fieldOrderFromProgressive(isProgressive),
	}

	return convert.ValidateDVDNTSC(tempSrc, d.cfg)
}

// GetPresetInfo returns a description of the DVD-NTSC preset
func (d *DVDConvertConfig) GetPresetInfo() string {
	return convert.DVDNTSCInfo()
}

// helper function to convert boolean to field order string
func fieldOrderFromProgressive(isProgressive bool) string {
	if isProgressive {
		return "progressive"
	}
	return "interlaced"
}

// DVDPresetInfo provides information about DVD-NTSC capability
type DVDPresetInfo struct {
	Name           string
	Description    string
	VideoCodec     string
	AudioCodec     string
	Container      string
	Resolution     string
	FrameRate      string
	DefaultBitrate string
	MaxBitrate     string
	Features       []string
}

// GetDVDPresetInfo returns detailed information about the DVD-NTSC preset
func GetDVDPresetInfo() DVDPresetInfo {
	return DVDPresetInfo{
		Name:           "DVD-NTSC (Region-Free)",
		Description:    "Professional DVD-Video output compatible with DVD authoring tools and PS2",
		VideoCodec:     "MPEG-2",
		AudioCodec:     "AC-3 (Dolby Digital)",
		Container:      "MPEG Program Stream (.mpg)",
		Resolution:     "720x480 (NTSC Full D1)",
		FrameRate:      "29.97 fps",
		DefaultBitrate: "6000 kbps",
		MaxBitrate:     "9000 kbps (PS2-safe)",
		Features: []string{
			"DVDStyler-compatible output (no re-encoding)",
			"PlayStation 2 compatible",
			"Standalone DVD player compatible",
			"Automatic aspect ratio handling (4:3 or 16:9)",
			"Automatic audio resampling to 48kHz",
			"Framerate conversion (23.976p, 24p, 30p, 60p support)",
			"Interlacing detection and preservation",
			"Region-free authoring support",
		},
	}
}
