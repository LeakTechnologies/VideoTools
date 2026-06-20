package convert

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/LeakTechnologies/VideoTools/internal/utils"
)

// FormatOption represents a video output format with its associated codec
type FormatOption struct {
	Label        string
	Ext          string
	VideoCodec   string
	Name         string // Alias for Label for flexibility
	DevicePreset string // Device preset name this format is paired with, if any
	SupportsHEVC bool   // Format container supports H.265
	SupportsAV1  bool   // Format container supports AV1
	Legacy       bool   // Marks format as legacy — available for remuxing old files, not recommended for new outputs
}

// ConvertConfig holds all configuration for a video conversion operation
type ConvertConfig struct {
	OutputBase     string
	SelectedFormat FormatOption
	Quality        string // Preset quality (Draft/Standard/High/Lossless)
	Mode           string // Simple or Advanced

	// Video encoding settings
	VideoCodec       string // H.264, H.265, VP9, AV1, Copy
	EncoderPreset    string // ultrafast, superfast, veryfast, faster, fast, medium, slow, slower, veryslow
	CRF              string // Manual CRF value (0-51, or empty to use Quality preset)
	BitrateMode      string // CRF, CBR, VBR, "Target Size"
	VideoBitrate     string // For CBR/VBR modes (e.g., "5000k")
	TargetFileSize   string // Target file size (e.g., "25MB", "100MB", "8MB") - requires BitrateMode="Target Size"
	TargetResolution string // Source, 720p, 1080p, 1440p, 4K, or custom
	FrameRate        string // Source, 24, 30, 60, or custom
	PixelFormat      string // yuv420p, yuv422p, yuv444p
	HardwareAccel    string // none, nvenc, vaapi, qsv, videotoolbox
	TwoPass          bool   // Enable two-pass encoding for VBR
	EncoderTune      string // None, Film, Animation, Grain, Stillimage, Fastdecode

	// Audio encoding settings
	AudioCodec    string // AAC, Opus, MP3, FLAC, Copy
	AudioBitrate  string // 128k, 192k, 256k, 320k
	AudioChannels string // Source, Mono, Stereo, 5.1

	// Other settings
	InverseTelecine  bool
	InverseAutoNotes string
	CoverArtPath     string
	AspectHandling   string
	OutputAspect     string
}

// OutputFile returns the complete output filename with extension
func (c ConvertConfig) OutputFile() string {
	base := strings.TrimSpace(c.OutputBase)
	if base == "" {
		base = "converted"
	}
	return base + c.SelectedFormat.Ext
}

// CoverLabel returns a display label for the cover art
func (c ConvertConfig) CoverLabel() string {
	if strings.TrimSpace(c.CoverArtPath) == "" {
		return "none"
	}
	return filepath.Base(c.CoverArtPath)
}

// Chapter represents a single chapter in a video file
type Chapter struct {
	Index     int
	StartTime float64
	EndTime   float64
	Title     string
}

// VideoSource represents metadata about a video file
type VideoSource struct {
	Path             string
	DisplayName      string
	Format           string
	Width            int
	Height           int
	Duration         float64
	VideoCodec       string
	AudioCodec       string
	Bitrate          int // Video bitrate in bits per second
	AudioBitrate     int // Audio bitrate in bits per second
	FrameRate        float64
	PixelFormat      string
	AudioRate        int
	Channels         int
	FieldOrder       string
	PreviewFrames    []string
	EmbeddedCoverArt string // Path to extracted embedded cover art, if any

	// Chapters
	Chapters []Chapter // Parsed chapter information

	// Advanced metadata
	SampleAspectRatio string // Pixel Aspect Ratio (SAR) - e.g., "1:1", "40:33"
	ColorSpace        string // Color space/primaries - e.g., "bt709", "bt601"
	ColorRange        string // Color range - "tv" (limited) or "pc" (full)
	GOPSize           int    // GOP size / keyframe interval
	HasChapters       bool   // Whether file has embedded chapters
	HasMetadata       bool   // Whether file has title/copyright/etc metadata
	Metadata          map[string]string
}

// DurationString returns a human-readable duration string (HH:MM:SS or MM:SS)
func (v *VideoSource) DurationString() string {
	if v.Duration <= 0 {
		return "--"
	}
	d := time.Duration(v.Duration * float64(time.Second))
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// AspectRatioString returns a human-readable aspect ratio string
func (v *VideoSource) AspectRatioString() string {
	if v.Width <= 0 || v.Height <= 0 {
		return "--"
	}
	num, den := utils.SimplifyRatio(v.Width, v.Height)
	if num == 0 || den == 0 {
		return "--"
	}
	ratio := float64(num) / float64(den)
	return fmt.Sprintf("%d:%d (%.2f:1)", num, den, ratio)
}

// IsProgressive returns true if the video is progressive (not interlaced)
func (v *VideoSource) IsProgressive() bool {
	order := strings.ToLower(v.FieldOrder)
	if strings.Contains(order, "progressive") {
		return true
	}
	if strings.Contains(order, "unknown") && strings.Contains(strings.ToLower(v.PixelFormat), "p") {
		return true
	}
	return false
}

// FormatClock converts seconds to a human-readable time string (H:MM:SS or MM:SS)
func FormatClock(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	d := time.Duration(sec * float64(time.Second))
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// ResolveTargetAspect resolves a target aspect ratio string to a float64 value
func ResolveTargetAspect(val string, src *VideoSource) float64 {
	if strings.EqualFold(val, "source") {
		if src != nil {
			return utils.DisplayAspectRatioFloat(src.Width, src.Height, src.SampleAspectRatio)
		}
		return 0
	}
	if r := utils.ParseAspectValue(val); r > 0 {
		return r
	}
	return 0
}

// CalculateBitrateForTargetSize calculates the required video bitrate to hit a target file size
// targetSize: target file size in bytes
// duration: video duration in seconds
// audioBitrate: audio bitrate in bits per second
// Returns: video bitrate in bits per second
func CalculateBitrateForTargetSize(targetSize int64, duration float64, audioBitrate int) int {
	if duration <= 0 {
		return 0
	}

	// Reserve 3% for container overhead
	targetSize = int64(float64(targetSize) * 0.97)

	// Calculate total bits available
	totalBits := targetSize * 8

	// Calculate audio bits
	audioBits := int64(float64(audioBitrate) * duration)

	// Remaining bits for video
	videoBits := totalBits - audioBits
	if videoBits < 0 {
		videoBits = totalBits / 2 // Fallback: split 50/50 if audio is too large
	}

	// Calculate video bitrate
	videoBitrate := int(float64(videoBits) / duration)

	// Minimum bitrate sanity check (100 kbps)
	if videoBitrate < 100000 {
		videoBitrate = 100000
	}

	return videoBitrate
}

// ParseFileSize parses a file size string like "25MB", "100MB", "1.5GB" into bytes
func ParseFileSize(sizeStr string) (int64, error) {
	sizeStr = strings.TrimSpace(strings.ToUpper(sizeStr))
	if sizeStr == "" {
		return 0, fmt.Errorf("empty size string")
	}

	// Extract number and unit
	var value float64
	var unit string

	_, err := fmt.Sscanf(sizeStr, "%f%s", &value, &unit)
	if err != nil {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}
	if unit == "" {
		unit = "MB"
	}

	// Convert to bytes
	multiplier := int64(1)
	switch unit {
	case "K", "KB":
		multiplier = 1024
	case "M", "MB":
		multiplier = 1024 * 1024
	case "G", "GB":
		multiplier = 1024 * 1024 * 1024
	case "B", "":
		multiplier = 1
	default:
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}

	return int64(value * float64(multiplier)), nil
}

// AspectFilters returns FFmpeg filter strings for aspect ratio conversion
func AspectFilters(target float64, mode string) []string {
	if target <= 0 {
		return nil
	}
	ar := fmt.Sprintf("%.6f", target)
	setDAR := fmt.Sprintf("setdar=%s", ar)

	// Crop mode: center crop to target aspect ratio
	if strings.EqualFold(mode, "Crop") || strings.EqualFold(mode, "Auto") {
		// Crop to target aspect ratio with even dimensions for H.264 encoding
		// Use trunc/2*2 to ensure even dimensions
		crop := fmt.Sprintf("crop=w='trunc(if(gt(a,%[1]s),ih*%[1]s,iw)/2)*2':h='trunc(if(gt(a,%[1]s),ih,iw/%[1]s)/2)*2':x='(iw-out_w)/2':y='(ih-out_h)/2'", ar)
		return []string{crop, setDAR, "setsar=1"}
	}

	// Stretch mode: just change the aspect ratio without cropping or padding
	if strings.EqualFold(mode, "Stretch") {
		scale := fmt.Sprintf("scale=w='trunc(ih*%[1]s/2)*2':h='trunc(iw/%[1]s/2)*2'", ar)
		return []string{scale, setDAR, "setsar=1"}
	}

	// Blur Fill: create blurred background then overlay original video
	if strings.EqualFold(mode, "Blur Fill") {
		// Complex filter chain:
		// 1. Split input into two streams
		// 2. Blur and scale one stream to fill the target canvas
		// 3. Overlay the original video centered on top
		// Output dimensions with even numbers
		outW := fmt.Sprintf("trunc(max(iw,ih*%[1]s)/2)*2", ar)
		outH := fmt.Sprintf("trunc(max(ih,iw/%[1]s)/2)*2", ar)

		// Filter: split[bg][fg]; [bg]scale=outW:outH,boxblur=20:5[blurred]; [blurred][fg]overlay=(W-w)/2:(H-h)/2
		filterStr := fmt.Sprintf("split[bg][fg];[bg]scale=%s:%s:force_original_aspect_ratio=increase,boxblur=20:5[blurred];[blurred][fg]overlay=(W-w)/2:(H-h)/2", outW, outH)
		return []string{filterStr, setDAR, "setsar=1"}
	}

	// Letterbox/Pillarbox: keep source resolution, just pad to target aspect with black bars
	pad := fmt.Sprintf("pad=w='trunc(max(iw,ih*%[1]s)/2)*2':h='trunc(max(ih,iw/%[1]s)/2)*2':x='(ow-iw)/2':y='(oh-ih)/2':color=black", ar)
	return []string{pad, setDAR, "setsar=1"}
}
