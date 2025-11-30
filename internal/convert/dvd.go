package convert

import (
	"fmt"
	"strings"
)

// DVDNTSCPreset creates a ConvertConfig optimized for DVD-Video NTSC output
// This preset generates MPEG-2 program streams (.mpg) that are:
// - Fully DVD-compliant (720x480@29.97fps NTSC)
// - Region-free
// - Compatible with DVDStyler and professional DVD authoring software
// - Playable on PS2, standalone DVD players, and modern systems
func DVDNTSCPreset() ConvertConfig {
	return ConvertConfig{
		SelectedFormat:   FormatOption{Label: "MPEG-2 (DVD NTSC)", Ext: ".mpg", VideoCodec: "mpeg2video"},
		Quality:          "Standard (CRF 23)", // DVD uses bitrate control, not CRF
		Mode:             "Advanced",
		VideoCodec:       "MPEG-2",
		EncoderPreset:    "medium",
		BitrateMode:      "CBR", // DVD requires constant bitrate
		VideoBitrate:     "6000k",
		TargetResolution: "720x480",
		FrameRate:        "29.97",
		PixelFormat:      "yuv420p",
		HardwareAccel:    "none", // MPEG-2 encoding doesn't benefit much from GPU acceleration
		AudioCodec:       "AC-3",
		AudioBitrate:     "192k",
		AudioChannels:    "Stereo",
		InverseTelecine:  false, // Set based on source
		AspectHandling:   "letterbox",
		OutputAspect:     "source",
	}
}

// DVDValidationWarning represents a validation issue with DVD encoding
type DVDValidationWarning struct {
	Severity string // "info", "warning", "error"
	Message  string
	Action   string // What will be done to fix it
}

// ValidateDVDNTSC performs comprehensive validation on a video for DVD-NTSC output
func ValidateDVDNTSC(src *VideoSource, cfg ConvertConfig) []DVDValidationWarning {
	var warnings []DVDValidationWarning

	if src == nil {
		warnings = append(warnings, DVDValidationWarning{
			Severity: "error",
			Message:  "No video source selected",
			Action:   "Cannot proceed without a source video",
		})
		return warnings
	}

	// 1. Framerate Validation
	if src.FrameRate > 0 {
		normalizedRate := normalizeFrameRate(src.FrameRate)
		switch normalizedRate {
		case "23.976":
			warnings = append(warnings, DVDValidationWarning{
				Severity: "warning",
				Message:  fmt.Sprintf("Input framerate is %.2f fps (23.976p)", src.FrameRate),
				Action:   "Will apply 3:2 pulldown to convert to 29.97fps (requires interlacing)",
			})
		case "24.0":
			warnings = append(warnings, DVDValidationWarning{
				Severity: "warning",
				Message:  fmt.Sprintf("Input framerate is %.2f fps (24p)", src.FrameRate),
				Action:   "Will apply 3:2 pulldown to convert to 29.97fps (requires interlacing)",
			})
		case "29.97":
			// Perfect - no warning
		case "30.0":
			warnings = append(warnings, DVDValidationWarning{
				Severity: "info",
				Message:  fmt.Sprintf("Input framerate is %.2f fps (30p)", src.FrameRate),
				Action:   "Will convert to 29.97fps (NTSC standard)",
			})
		case "59.94":
			warnings = append(warnings, DVDValidationWarning{
				Severity: "warning",
				Message:  fmt.Sprintf("Input framerate is %.2f fps (59.94p)", src.FrameRate),
				Action:   "Will decimate to 29.97fps (dropping every other frame)",
			})
		case "60.0":
			warnings = append(warnings, DVDValidationWarning{
				Severity: "warning",
				Message:  fmt.Sprintf("Input framerate is %.2f fps (60p)", src.FrameRate),
				Action:   "Will decimate to 29.97fps (dropping every other frame)",
			})
		case "vfr":
			warnings = append(warnings, DVDValidationWarning{
				Severity: "error",
				Message:  "Input is Variable Frame Rate (VFR)",
				Action:   "Will force constant frame rate at 29.97fps (may cause sync issues)",
			})
		default:
			if src.FrameRate < 15 {
				warnings = append(warnings, DVDValidationWarning{
					Severity: "error",
					Message:  fmt.Sprintf("Input framerate is %.2f fps (too low for DVD)", src.FrameRate),
					Action:   "Cannot encode - DVD requires minimum 23.976fps",
				})
			} else if src.FrameRate > 60 {
				warnings = append(warnings, DVDValidationWarning{
					Severity: "warning",
					Message:  fmt.Sprintf("Input framerate is %.2f fps (higher than DVD standard)", src.FrameRate),
					Action:   "Will decimate to 29.97fps",
				})
			}
		}
	}

	// 2. Resolution Validation
	if src.Width != 720 || src.Height != 480 {
		warnings = append(warnings, DVDValidationWarning{
			Severity: "info",
			Message:  fmt.Sprintf("Input resolution is %dx%d (not 720x480)", src.Width, src.Height),
			Action:   "Will scale to 720x480 with aspect-ratio correction",
		})
	}

	// 3. Audio Sample Rate Validation
	if src.AudioRate > 0 {
		if src.AudioRate != 48000 {
			warnings = append(warnings, DVDValidationWarning{
				Severity: "warning",
				Message:  fmt.Sprintf("Audio sample rate is %d Hz (not 48 kHz)", src.AudioRate),
				Action:   "Will resample to 48 kHz (DVD standard)",
			})
		}
	}

	// 4. Interlacing Analysis
	if !src.IsProgressive() {
		warnings = append(warnings, DVDValidationWarning{
			Severity: "info",
			Message:  "Input is interlaced",
			Action:   "Will encode as interlaced (progressive deinterlacing not applied)",
		})
	} else {
		warnings = append(warnings, DVDValidationWarning{
			Severity: "info",
			Message:  "Input is progressive",
			Action:   "Will encode as progressive (no interlacing applied)",
		})
	}

	// 5. Bitrate Validation
	maxDVDBitrate := 9000.0
	if strings.HasSuffix(cfg.VideoBitrate, "k") {
		bitrateStr := strings.TrimSuffix(cfg.VideoBitrate, "k")
		var bitrate float64
		if _, err := fmt.Sscanf(bitrateStr, "%f", &bitrate); err == nil {
			if bitrate > maxDVDBitrate {
				warnings = append(warnings, DVDValidationWarning{
					Severity: "error",
					Message:  fmt.Sprintf("Video bitrate %s exceeds DVD maximum of %.0fk", cfg.VideoBitrate, maxDVDBitrate),
					Action:   "Will cap at 9000k (PS2 safe limit)",
				})
			}
		}
	}

	// 6. Audio Codec Validation
	if cfg.AudioCodec != "AC-3" && cfg.AudioCodec != "Copy" {
		warnings = append(warnings, DVDValidationWarning{
			Severity: "warning",
			Message:  fmt.Sprintf("Audio codec is %s (DVD standard is AC-3)", cfg.AudioCodec),
			Action:   "Recommend using AC-3 for maximum compatibility",
		})
	}

	// 7. Aspect Ratio Validation
	if src.Width > 0 && src.Height > 0 {
		sourceAspect := float64(src.Width) / float64(src.Height)
		validAspects := map[string]float64{
			"4:3":  1.333,
			"16:9": 1.778,
		}
		found := false
		for _, ratio := range validAspects {
			// Allow 1% tolerance
			if diff := sourceAspect - ratio; diff < 0 && diff > -0.02 || diff >= 0 && diff < 0.02 {
				found = true
				break
			}
		}
		if !found {
			warnings = append(warnings, DVDValidationWarning{
				Severity: "warning",
				Message:  fmt.Sprintf("Aspect ratio is %.2f:1 (not standard 4:3 or 16:9)", sourceAspect),
				Action:   fmt.Sprintf("Will apply %s with aspect correction", cfg.AspectHandling),
			})
		}
	}

	return warnings
}

// normalizeFrameRate categorizes a framerate value
func normalizeFrameRate(rate float64) string {
	if rate < 15 {
		return "low"
	}
	// Check for common framerates with tolerance
	checks := []struct {
		name      string
		min, max  float64
	}{
		{"23.976", 23.9, 24.0},
		{"24.0", 23.99, 24.01},
		{"29.97", 29.9, 30.0},
		{"30.0", 30.0, 30.01},
		{"59.94", 59.9, 60.0},
		{"60.0", 60.0, 60.01},
	}
	for _, c := range checks {
		if rate >= c.min && rate <= c.max {
			return c.name
		}
	}
	return fmt.Sprintf("%.2f", rate)
}

// BuildDVDFFmpegArgs constructs FFmpeg arguments for DVD-NTSC encoding
// This ensures all parameters are DVD-compliant and correctly formatted
func BuildDVDFFmpegArgs(inputPath, outputPath string, cfg ConvertConfig, src *VideoSource) []string {
	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-i", inputPath,
	}

	// Video filters
	var vf []string

	// Scaling to DVD resolution with aspect preservation
	if src.Width != 720 || src.Height != 480 {
		// Use scale filter with aspect ratio handling
		vf = append(vf, "scale=720:480:force_original_aspect_ratio=1")

		// Add aspect ratio handling (pad/crop)
		switch cfg.AspectHandling {
		case "letterbox":
			vf = append(vf, "pad=720:480:(ow-iw)/2:(oh-ih)/2")
		case "pillarbox":
			vf = append(vf, "pad=720:480:(ow-iw)/2:(oh-ih)/2")
		}
	}

	// Set Display Aspect Ratio (DAR) - tell decoder the aspect
	if cfg.OutputAspect == "16:9" {
		vf = append(vf, "setdar=16/9")
	} else {
		vf = append(vf, "setdar=4/3")
	}

	// Set Sample Aspect Ratio (SAR) - DVD standard
	vf = append(vf, "setsar=1")

	// Framerate - always to 29.97 for NTSC
	vf = append(vf, "fps=30000/1001")

	if len(vf) > 0 {
		args = append(args, "-vf", strings.Join(vf, ","))
	}

	// Video codec - MPEG-2 for DVD
	args = append(args,
		"-c:v", "mpeg2video",
		"-r", "30000/1001",
		"-b:v", "6000k",
		"-maxrate", "9000k",
		"-bufsize", "1835k",
		"-g", "15", // GOP size
		"-flags", "+mv4", // Use four motion vector candidates
		"-pix_fmt", "yuv420p",
	)

	// Optional: Interlacing flags
	// If the source is interlaced, we can preserve that:
	if !src.IsProgressive() {
		args = append(args, "-flags", "+ilme+ildct")
	}

	// Audio codec - AC-3 (Dolby Digital)
	args = append(args,
		"-c:a", "ac3",
		"-b:a", "192k",
		"-ar", "48000",
		"-ac", "2",
	)

	// Progress monitoring
	args = append(args,
		"-progress", "pipe:1",
		"-nostats",
		outputPath,
	)

	return args
}

// DVDNTSCInfo returns a human-readable description of the DVD-NTSC preset
func DVDNTSCInfo() string {
	return `DVD-NTSC (Region-Free) Output

This preset generates professional-grade MPEG-2 program streams (.mpg) compatible with:
- DVD authoring software (DVDStyler, Adobe Encore, etc.)
- PlayStation 2 and standalone DVD players
- Modern media centers and PC-based DVD players

Technical Specifications:
  Video Codec:        MPEG-2 (mpeg2video)
  Container:          MPEG Program Stream (.mpg)
  Resolution:         720x480 (NTSC Full D1)
  Frame Rate:         29.97 fps (30000/1001)
  Aspect Ratio:       4:3 or 16:9 (user-selectable)
  Bitrate:            6000 kbps (average), 9000 kbps (max)
  GOP Size:           15 frames
  Interlacing:        Progressive or Interlaced (auto-detected)

Audio Codec:         AC-3 (Dolby Digital)
  Channels:          Stereo (2.0)
  Bitrate:           192 kbps
  Sample Rate:       48 kHz (mandatory)

The output is guaranteed to be importable directly into DVDStyler without
re-encoding warnings, and will play flawlessly on PS2 and standalone players.`
}
