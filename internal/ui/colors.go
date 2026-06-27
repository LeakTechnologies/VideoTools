package ui

import (
	"image/color"
	"strings"

	"github.com/LeakTechnologies/VideoTools/internal/utils"
)

// Semantic Color System for VideoTools
// Based on professional NLE and broadcast tooling conventions

// Container / Format Colors (File Wrapper)
var (
	ColorMKV   = utils.MustHex("#3B82F6") // Blue - Flexible container
	ColorRemux = utils.MustHex("#9CA3AF") // Neutral Grey - Lossless remux
	ColorMP4   = utils.MustHex("#22C55E") // Green - Consumer-friendly
	ColorMOV   = utils.MustHex("#A855F7") // Purple - Pro / Apple lineage
	ColorAVI   = utils.MustHex("#F97316") // Orange - Legacy container
	ColorWEBM  = utils.MustHex("#14B8A6") // Teal - Web-native
	ColorTS    = utils.MustHex("#F59E0B") // Amber - Broadcast / transport streams
	ColorM2TS  = utils.MustHex("#EAB308") // Yellow - Broadcast / transport streams
)

// Video Codec Colors (Compression Method)
// Modern / Efficient Codecs
var (
	ColorAV1  = utils.MustHex("#F97316") // Orange - Modern, efficient
	ColorHEVC = utils.MustHex("#22C55E") // Green - Modern, efficient
	ColorH265 = utils.MustHex("#22C55E") // Green - Same as HEVC
	ColorVP9  = utils.MustHex("#8B5CF6") // Violet - Modern, efficient
)

// Established / Legacy Video Codecs
var (
	ColorH264  = utils.MustHex("#3B82F6") // Blue - Compatibility
	ColorAVC   = utils.MustHex("#3B82F6") // Blue - Same as H.264
	ColorMPEG2 = utils.MustHex("#EAB308") // Yellow - Legacy / broadcast
	ColorDivX  = utils.MustHex("#EF4444") // Red - Legacy
	ColorXviD  = utils.MustHex("#EF4444") // Red - Legacy
	ColorMPEG4 = utils.MustHex("#EF4444") // Red - Legacy
)

// Professional / Intermediate Codecs
var (
	ColorProRes = utils.MustHex("#A855F7") // Purple - Apple ProRes ecosystem
	ColorDNx    = utils.MustHex("#3B82F6") // Blue - Avid DNxHR/DNxHD
)

// Audio Codec Colors (Secondary but Distinct)
var (
	ColorOpus   = utils.MustHex("#EC4899") // Pink - Modern audio
	ColorAAC    = utils.MustHex("#06B6D4") // Cyan - Common audio
	ColorFLAC   = utils.MustHex("#A855F7") // Purple - Lossless audio
	ColorMP3    = utils.MustHex("#EF4444") // Red - Legacy audio
	ColorAC3    = utils.MustHex("#F59E0B") // Amber - Surround audio
	ColorVorbis = utils.MustHex("#22C55E") // Green - Open codec
	ColorPCM    = utils.MustHex("#94A3B8") // Slate - Uncompressed audio
	ColorDTS    = utils.MustHex("#F97316") // Orange - DTS audio
	ColorTrueHD = utils.MustHex("#14B8A6") // Teal - Dolby TrueHD
)

// Pixel Format / Colour Data (Technical Metadata)
var (
	ColorYUV420P = utils.MustHex("#94A3B8") // Slate - Standard
	ColorYUV422P = utils.MustHex("#64748B") // Slate-Blue - Intermediate
	ColorYUV444P = utils.MustHex("#475569") // Steel - High quality
	ColorHDR     = utils.MustHex("#06B6D4") // Cyan-Glow - HDR content
	ColorSDR     = utils.MustHex("#9CA3AF") // Neutral Grey - SDR content
)

// GetContainerColor returns the semantic color for a container format
func GetContainerColor(format string) color.Color {
	switch format {
	case "mkv", "matroska":
		return ColorMKV
	case "mp4", "m4v":
		return ColorMP4
	case "mov", "quicktime":
		return ColorMOV
	case "avi":
		return ColorAVI
	case "webm":
		return ColorWEBM
	case "ts":
		return ColorTS
	case "m2ts", "mts":
		return ColorM2TS
	default:
		return color.RGBA{100, 100, 100, 255} // Default grey
	}
}

// GetVideoCodecColor returns the semantic color for a video codec
func GetVideoCodecColor(codec string) color.Color {
	switch codec {
	case "av1":
		return ColorAV1
	case "hevc", "h265", "h.265":
		return ColorHEVC
	case "vp9":
		return ColorVP9
	case "h264", "avc", "h.264":
		return ColorH264
	case "mpeg2", "mpeg2video":
		return ColorMPEG2
	case "divx", "xvid", "mpeg4":
		return ColorDivX
	case "prores":
		return ColorProRes
	case "dnxhd", "dnxhr":
		return ColorDNx
	default:
		return color.RGBA{100, 100, 100, 255} // Default grey
	}
}

// GetAudioCodecColor returns the semantic color for an audio codec
func GetAudioCodecColor(codec string) color.Color {
	switch codec {
	case "opus":
		return ColorOpus
	case "aac":
		return ColorAAC
	case "flac":
		return ColorFLAC
	case "mp3":
		return ColorMP3
	case "ac3", "eac3":
		return ColorAC3
	case "vorbis":
		return ColorVorbis
	case "truehd":
		return ColorTrueHD
	case "dts":
		return ColorDTS
	case "pcm_s16le", "pcm_s24le", "pcm_s32le", "pcm_f32le", "pcm_u8":
		return ColorPCM
	default:
		return color.RGBA{100, 100, 100, 255} // Default grey
	}
}

// GetPixelFormatColor returns the semantic color for a pixel format
func GetPixelFormatColor(pixfmt string) color.Color {
	switch pixfmt {
	case "yuv420p", "yuv420p10le":
		return ColorYUV420P
	case "yuv422p", "yuv422p10le":
		return ColorYUV422P
	case "yuv444p", "yuv444p10le":
		return ColorYUV444P
	default:
		return ColorSDR
	}
}

// BuildFormatColorMap creates a color map for format labels
// Parses labels like "MKV (AV1)" and returns appropriate container color
func BuildFormatColorMap(formatLabels []string) map[string]color.Color {
	colorMap := make(map[string]color.Color)
	for _, label := range formatLabels {
		// Parse format from label (e.g., "MKV (AV1)" -> "mkv")
		parts := strings.Split(label, " ")
		if len(parts) > 0 {
			format := strings.ToLower(parts[0])
			// Special case for Remux
			if strings.Contains(strings.ToUpper(label), "REMUX") {
				colorMap[label] = ColorRemux
				continue
			}
			colorMap[label] = GetContainerColor(format)
		}
	}
	return colorMap
}

// BuildVideoCodecColorMap creates a color map for video codec options
func BuildVideoCodecColorMap(codecs []string) map[string]color.Color {
	colorMap := make(map[string]color.Color)
	for _, codec := range codecs {
		switch codec {
		case "H.264":
			colorMap[codec] = ColorH264
		case "H.265":
			colorMap[codec] = ColorHEVC
		case "VP9":
			colorMap[codec] = ColorVP9
		case "AV1":
			colorMap[codec] = ColorAV1
		case "MPEG-2":
			colorMap[codec] = ColorMPEG2
		case "Copy":
			colorMap[codec] = ColorRemux // Use remux color for copy
		default:
			colorMap[codec] = color.RGBA{100, 100, 100, 255}
		}
	}
	return colorMap
}

// BuildAudioCodecColorMap creates a color map for audio codec options
func BuildAudioCodecColorMap(codecs []string) map[string]color.Color {
	colorMap := make(map[string]color.Color)
	for _, codec := range codecs {
		switch codec {
		case "AAC":
			colorMap[codec] = ColorAAC
		case "AC-3":
			colorMap[codec] = ColorMP3 // Reuse MP3 color (both are legacy)
		case "Opus":
			colorMap[codec] = ColorOpus
		case "Vorbis":
			colorMap[codec] = ColorAAC // Reuse AAC color (both are lossy)
		case "MP3":
			colorMap[codec] = ColorMP3
		case "FLAC":
			colorMap[codec] = ColorFLAC
		case "Copy":
			colorMap[codec] = ColorRemux // Use remux color for copy
		default:
			colorMap[codec] = color.RGBA{100, 100, 100, 255}
		}
	}
	return colorMap
}

// BuildGenericColorMap creates a rainbow color map for any list of options
// Uses distinct, vibrant colors to make navigation faster
func BuildGenericColorMap(options []string) map[string]color.Color {
	colorMap := make(map[string]color.Color)

	// Rainbow palette - vibrant and distinct colors
	rainbowColors := []color.Color{
		utils.MustHex("#EF4444"), // Red
		utils.MustHex("#F97316"), // Orange
		utils.MustHex("#F59E0B"), // Amber
		utils.MustHex("#EAB308"), // Yellow
		utils.MustHex("#84CC16"), // Lime
		utils.MustHex("#22C55E"), // Green
		utils.MustHex("#10B981"), // Emerald
		utils.MustHex("#14B8A6"), // Teal
		utils.MustHex("#06B6D4"), // Cyan
		utils.MustHex("#0EA5E9"), // Sky
		utils.MustHex("#3B82F6"), // Blue
		utils.MustHex("#6366F1"), // Indigo
		utils.MustHex("#8B5CF6"), // Violet
		utils.MustHex("#A855F7"), // Purple
		utils.MustHex("#D946EF"), // Fuchsia
		utils.MustHex("#EC4899"), // Pink
	}

	for i, opt := range options {
		colorMap[opt] = rainbowColors[i%len(rainbowColors)]
	}
	return colorMap
}

// BuildQualityColorMap creates a gradient-based color map for quality/preset options
// Higher quality = cooler colors (blue), lower quality = warmer colors (red/orange)
func BuildQualityColorMap(options []string) map[string]color.Color {
	colorMap := make(map[string]color.Color)

	// Quality gradient: red (fast/low) -> yellow -> green -> blue (slow/high)
	qualityColors := []color.Color{
		utils.MustHex("#EF4444"), // Red - ultrafast/lowest
		utils.MustHex("#F97316"), // Orange - superfast
		utils.MustHex("#F59E0B"), // Amber - veryfast
		utils.MustHex("#EAB308"), // Yellow - faster
		utils.MustHex("#84CC16"), // Lime - fast
		utils.MustHex("#22C55E"), // Green - medium
		utils.MustHex("#10B981"), // Emerald - slow
		utils.MustHex("#14B8A6"), // Teal - slower
		utils.MustHex("#06B6D4"), // Cyan - veryslow
		utils.MustHex("#3B82F6"), // Blue - highest quality
	}

	for i, opt := range options {
		colorMap[opt] = qualityColors[i%len(qualityColors)]
	}
	return colorMap
}

// BuildPixelFormatColorMap creates a color map for pixel format options
func BuildPixelFormatColorMap(formats []string) map[string]color.Color {
	colorMap := make(map[string]color.Color)
	for _, format := range formats {
		colorMap[format] = GetPixelFormatColor(format)
	}
	return colorMap
}
