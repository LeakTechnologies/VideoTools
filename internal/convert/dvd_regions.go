package convert

import (
	"fmt"
	"strings"
)

// DVDRegion represents a DVD standard/region combination
type DVDRegion string

const (
	DVDNTSCRegionFree  DVDRegion = "ntsc-region-free"
	DVDPALRegionFree   DVDRegion = "pal-region-free"
	DVDSECAMRegionFree DVDRegion = "secam-region-free"
)

// DVDStandard represents the technical specifications for a DVD encoding standard
type DVDStandard struct {
	Region         DVDRegion
	Name           string
	Resolution     string // "720x480" or "720x576"
	FrameRate      string // "29.97" or "25.00"
	VideoFrames    int    // 30 or 25
	AudioRate      int    // 48000 Hz (universal)
	Type           string // "NTSC", "PAL", or "SECAM"
	Countries      []string
	DefaultBitrate string // "6000k" for NTSC, "8000k" for PAL
	MaxBitrate     string // "9000k" for NTSC, "9500k" for PAL
	AspectRatios   []string
	InterlaceMode  string // "interlaced" or "progressive"
	Description    string
}

// GetDVDStandard returns specifications for a given DVD region
func GetDVDStandard(region DVDRegion) *DVDStandard {
	standards := map[DVDRegion]*DVDStandard{
		DVDNTSCRegionFree: {
			Region:         DVDNTSCRegionFree,
			Name:           "DVD-Video NTSC (Region-Free)",
			Resolution:     "720x480",
			FrameRate:      "29.97",
			VideoFrames:    30,
			AudioRate:      48000,
			Type:           "NTSC",
			Countries:      []string{"USA", "Canada", "Japan", "Brazil", "Mexico", "Australia", "New Zealand"},
			DefaultBitrate: "6000k",
			MaxBitrate:     "9000k",
			AspectRatios:   []string{"4:3", "16:9"},
			InterlaceMode:  "interlaced",
			Description: `NTSC DVD Standard
Resolution: 720x480 pixels
Frame Rate: 29.97 fps (30000/1001)
Bitrate: 6000-9000 kbps
Audio: AC-3 Stereo, 48 kHz, 192 kbps
Regions: North America, Japan, Australia, and others`,
		},
		DVDPALRegionFree: {
			Region:         DVDPALRegionFree,
			Name:           "DVD-Video PAL (Region-Free)",
			Resolution:     "720x576",
			FrameRate:      "25.00",
			VideoFrames:    25,
			AudioRate:      48000,
			Type:           "PAL",
			Countries:      []string{"Europe", "Africa", "Asia (except Japan)", "Australia", "New Zealand", "Argentina", "Brazil"},
			DefaultBitrate: "8000k",
			MaxBitrate:     "9500k",
			AspectRatios:   []string{"4:3", "16:9"},
			InterlaceMode:  "interlaced",
			Description: `PAL DVD Standard
Resolution: 720x576 pixels
Frame Rate: 25.00 fps
Bitrate: 8000-9500 kbps
Audio: AC-3 Stereo, 48 kHz, 192 kbps
Regions: Europe, Africa, most of Asia, Australia, New Zealand`,
		},
		DVDSECAMRegionFree: {
			Region:         DVDSECAMRegionFree,
			Name:           "DVD-Video SECAM (Region-Free)",
			Resolution:     "720x576",
			FrameRate:      "25.00",
			VideoFrames:    25,
			AudioRate:      48000,
			Type:           "SECAM",
			Countries:      []string{"France", "Russia", "Greece", "Eastern Europe", "Central Asia"},
			DefaultBitrate: "8000k",
			MaxBitrate:     "9500k",
			AspectRatios:   []string{"4:3", "16:9"},
			InterlaceMode:  "interlaced",
			Description: `SECAM DVD Standard
Resolution: 720x576 pixels
Frame Rate: 25.00 fps
Bitrate: 8000-9500 kbps
Audio: AC-3 Stereo, 48 kHz, 192 kbps
Regions: France, Russia, Eastern Europe, Central Asia
Note: SECAM DVDs are technically identical to PAL in the DVD standard (color encoding differences are applied at display time)`,
		},
	}
	return standards[region]
}

// PresetForRegion creates a ConvertConfig preset for the specified DVD region
func PresetForRegion(region DVDRegion) ConvertConfig {
	std := GetDVDStandard(region)
	if std == nil {
		// Fallback to NTSC
		std = GetDVDStandard(DVDNTSCRegionFree)
	}

	// Determine resolution as string
	var resStr string
	if std.Resolution == "720x576" {
		resStr = "720x576"
	} else {
		resStr = "720x480"
	}

	return ConvertConfig{
		SelectedFormat:   FormatOption{Name: std.Name, Label: std.Name, Ext: ".mpg", VideoCodec: "mpeg2video"},
		Quality:          "Standard (CRF 23)",
		Mode:             "Advanced",
		VideoCodec:       "MPEG-2",
		EncoderPreset:    "medium",
		BitrateMode:      "CBR",
		VideoBitrate:     std.DefaultBitrate,
		TargetResolution: resStr,
		FrameRate:        std.FrameRate,
		PixelFormat:      "yuv420p",
		HardwareAccel:    "none",
		AudioCodec:       "AC-3",
		AudioBitrate:     "192k",
		AudioChannels:    "Stereo",
		InverseTelecine:  false,
		AspectHandling:   "letterbox",
		OutputAspect:     "source",
	}
}

// ValidateForDVDRegion performs comprehensive validation for a specific DVD region
func ValidateForDVDRegion(src *VideoSource, region DVDRegion) []DVDValidationWarning {
	std := GetDVDStandard(region)
	if std == nil {
		std = GetDVDStandard(DVDNTSCRegionFree)
	}

	var warnings []DVDValidationWarning

	if src == nil {
		warnings = append(warnings, DVDValidationWarning{
			Severity: "error",
			Message:  "No video source selected",
			Action:   "Cannot proceed without a source video",
		})
		return warnings
	}

	// Add standard information
	warnings = append(warnings, DVDValidationWarning{
		Severity: "info",
		Message:  fmt.Sprintf("Encoding for: %s", std.Name),
		Action:   fmt.Sprintf("Resolution: %s @ %s fps", std.Resolution, std.FrameRate),
	})

	// 1. Target Resolution Validation
	var targetWidth, targetHeight int
	if strings.Contains(std.Resolution, "576") {
		targetWidth, targetHeight = 720, 576
	} else {
		targetWidth, targetHeight = 720, 480
	}

	if src.Width != targetWidth || src.Height != targetHeight {
		warnings = append(warnings, DVDValidationWarning{
			Severity: "info",
			Message:  fmt.Sprintf("Input resolution is %dx%d (target: %dx%d)", src.Width, src.Height, targetWidth, targetHeight),
			Action:   fmt.Sprintf("Will scale to %dx%d with aspect-ratio correction", targetWidth, targetHeight),
		})
	}

	// 2. Framerate Validation
	if src.FrameRate > 0 {
		var expectedRate float64
		if std.Type == "NTSC" {
			expectedRate = 29.97
		} else {
			expectedRate = 25.0
		}

		normalized := normalizeFrameRate(src.FrameRate)
		switch {
		case isFramerateClose(src.FrameRate, expectedRate):
			// Good
		case std.Type == "NTSC" && (normalized == "23.976" || normalized == "24.0"):
			warnings = append(warnings, DVDValidationWarning{
				Severity: "warning",
				Message:  fmt.Sprintf("Input framerate is %.2f fps (23.976p/24p)", src.FrameRate),
				Action:   "Will apply 3:2 pulldown to convert to 29.97fps",
			})
		case std.Type == "NTSC" && (normalized == "59.94" || normalized == "60.0"):
			warnings = append(warnings, DVDValidationWarning{
				Severity: "warning",
				Message:  fmt.Sprintf("Input framerate is %.2f fps (59.94p/60p)", src.FrameRate),
				Action:   "Will decimate to 29.97fps",
			})
		case normalized == "vfr":
			warnings = append(warnings, DVDValidationWarning{
				Severity: "error",
				Message:  "Input is Variable Frame Rate (VFR)",
				Action:   fmt.Sprintf("Will force constant frame rate at %s fps", std.FrameRate),
			})
		default:
			warnings = append(warnings, DVDValidationWarning{
				Severity: "warning",
				Message:  fmt.Sprintf("Input framerate is %.2f fps (standard is %s fps)", src.FrameRate, std.FrameRate),
				Action:   fmt.Sprintf("Will convert to %s fps", std.FrameRate),
			})
		}
	}

	// 3. Audio Sample Rate
	if src.AudioRate > 0 && src.AudioRate != 48000 {
		warnings = append(warnings, DVDValidationWarning{
			Severity: "warning",
			Message:  fmt.Sprintf("Audio sample rate is %d Hz (not 48 kHz)", src.AudioRate),
			Action:   "Will resample to 48 kHz (DVD standard)",
		})
	}

	// 4. Interlacing Analysis
	if !src.IsProgressive() {
		warnings = append(warnings, DVDValidationWarning{
			Severity: "info",
			Message:  "Input is interlaced",
			Action:   "Will preserve interlacing (optimal for DVD)",
		})
	} else {
		warnings = append(warnings, DVDValidationWarning{
			Severity: "info",
			Message:  "Input is progressive",
			Action:   "Will encode as progressive",
		})
	}

	// 5. Bitrate Safety Check
	warnings = append(warnings, DVDValidationWarning{
		Severity: "info",
		Message:  fmt.Sprintf("Bitrate range: %s (recommended) to %s (maximum PS2-safe)", std.DefaultBitrate, std.MaxBitrate),
		Action:   "Using standard bitrate settings for compatibility",
	})

	// 6. Aspect Ratio Information
	validAspects := std.AspectRatios
	warnings = append(warnings, DVDValidationWarning{
		Severity: "info",
		Message:  fmt.Sprintf("Supported aspect ratios: %s", strings.Join(validAspects, ", ")),
		Action:   "Output will preserve source aspect or apply specified handling",
	})

	return warnings
}

// isFramerateClose checks if a framerate is close to an expected value
func isFramerateClose(actual, expected float64) bool {
	diff := actual - expected
	if diff < 0 {
		diff = -diff
	}
	return diff < 0.1 // Within 0.1 fps
}

// parseMaxBitrate extracts the numeric bitrate from a string like "9000k"
func parseMaxBitrate(s string) int {
	var bitrate int
	fmt.Sscanf(strings.TrimSuffix(s, "k"), "%d", &bitrate)
	return bitrate
}

// ListAvailableDVDRegions returns information about all available DVD encoding regions
func ListAvailableDVDRegions() []DVDStandard {
	regions := []DVDRegion{DVDNTSCRegionFree, DVDPALRegionFree, DVDSECAMRegionFree}
	var standards []DVDStandard
	for _, region := range regions {
		if std := GetDVDStandard(region); std != nil {
			standards = append(standards, *std)
		}
	}
	return standards
}
