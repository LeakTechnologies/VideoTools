package utils

import (
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

// --- FFmpeg Path Management ---
var (
	globalFFmpegPath  atomic.Value
	globalFFprobePath atomic.Value
)

// SetFFmpegPaths sets the global FFmpeg and FFprobe paths.
// This should be called early in the application lifecycle after platform detection.
func SetFFmpegPaths(ffmpegPath, ffprobePath string) {
	globalFFmpegPath.Store(ffmpegPath)
	globalFFprobePath.Store(ffprobePath)
}

// GetFFmpegPath returns the globally configured FFmpeg executable path.
// It returns "ffmpeg" as a fallback if not explicitly set.
func GetFFmpegPath() string {
	if v := globalFFmpegPath.Load(); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return "ffmpeg" // Fallback
}

// GetFFprobePath returns the globally configured FFprobe executable path.
// It returns "ffprobe" as a fallback if not explicitly set.
func GetFFprobePath() string {
	if v := globalFFprobePath.Load(); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return "ffprobe" // Fallback
}

// GetFFplayPath returns the globally configured FFplay executable path.
// It returns "ffplay" as a fallback if not explicitly set.
func GetFFplayPath() string {
	return "ffplay" // Fallback
}

// --- Color utilities ---

// MustHex parses a hex color string or exits on error
func MustHex(h string) color.NRGBA {
	c, err := ParseHexColor(h)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid color %q: %v\n", h, err)
		os.Exit(1)
	}
	return c
}

// ParseHexColor parses a hex color string like "#RRGGBB"
func ParseHexColor(s string) (color.NRGBA, error) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return color.NRGBA{}, fmt.Errorf("want 6 digits, got %q", s)
	}
	var r, g, b uint8
	if _, err := fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b); err != nil {
		return color.NRGBA{}, err
	}
	return color.NRGBA{R: r, G: g, B: b, A: 0xff}, nil
}

// String utilities

// FirstNonEmpty returns the first non-empty string or "--"
func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return "--"
}

// ShortenMiddle shortens a string to max runes, keeping start and end with ellipsis in the middle.
func ShortenMiddle(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	ellipsis := "…"
	keep := max - utf8.RuneCountInString(ellipsis)
	if keep <= 0 {
		return s[:max]
	}
	left := keep / 2
	right := keep - left
	runes := []rune(s)
	if left+right >= len(runes) {
		return s
	}
	return string(runes[:left]) + ellipsis + string(runes[len(runes)-right:])
}

// Parsing utilities

// ParseFloat parses a float64 from a string
func ParseFloat(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}

// ParseInt parses an int from a string
func ParseInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	return strconv.Atoi(s)
}

// ParseFraction parses a fraction string like "24000/1001" or "30"
func ParseFraction(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0
	}
	parts := strings.Split(s, "/")
	num, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}
	if len(parts) == 1 {
		return num
	}
	den, err := strconv.ParseFloat(parts[1], 64)
	if err != nil || den == 0 {
		return 0
	}
	return num / den
}

// Math utilities

// GCD returns the greatest common divisor of two integers
func GCD(a, b int) int {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	for b != 0 {
		a, b = b, a%b
	}
	if a == 0 {
		return 1
	}
	return a
}

// SimplifyRatio simplifies a width/height ratio
func SimplifyRatio(w, h int) (int, int) {
	if w <= 0 || h <= 0 {
		return 0, 0
	}
	g := GCD(w, h)
	return w / g, h / g
}

// MaxInt returns the maximum of two integers
func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Aspect ratio utilities

// AspectRatioFloat calculates the aspect ratio as a float
func AspectRatioFloat(w, h int) float64 {
	if w <= 0 || h <= 0 {
		return 0
	}
	return float64(w) / float64(h)
}

// DisplayAspectRatioFloat calculates display aspect ratio using SAR when available.
func DisplayAspectRatioFloat(w, h int, sar string) float64 {
	base := AspectRatioFloat(w, h)
	if base <= 0 {
		return 0
	}
	sarVal := ParseAspectValue(strings.TrimSpace(sar))
	if sarVal <= 0 {
		return base
	}
	return base * sarVal
}

// ParseAspectValue parses an aspect ratio string like "16:9"
func ParseAspectValue(val string) float64 {
	val = strings.TrimSpace(val)
	switch val {
	case "16:9":
		return 16.0 / 9.0
	case "4:3":
		return 4.0 / 3.0
	case "1:1":
		return 1
	case "9:16":
		return 9.0 / 16.0
	case "21:9":
		return 21.0 / 9.0
	}
	parts := strings.Split(val, ":")
	if len(parts) == 2 {
		n, err1 := strconv.ParseFloat(parts[0], 64)
		d, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil && d != 0 {
			return n / d
		}
	}
	if f, err := strconv.ParseFloat(val, 64); err == nil && f > 0 {
		return f
	}
	return 0
}

// RatiosApproxEqual checks if two ratios are approximately equal
func RatiosApproxEqual(a, b, tol float64) bool {
	if a == 0 || b == 0 {
		return false
	}
	diff := math.Abs(a - b)
	if b != 0 {
		diff = diff / b
	}
	return diff <= tol
}

// Audio utilities

// ChannelLabel returns a human-readable label for a channel count
func ChannelLabel(ch int) string {
	switch ch {
	case 1:
		return "Mono"
	case 2:
		return "Stereo"
	case 6:
		return "5.1"
	case 8:
		return "7.1"
	default:
		if ch <= 0 {
			return ""
		}
		return fmt.Sprintf("%d ch", ch)
	}
}

// Image utilities

// CopyRGBToRGBA expands packed RGB bytes into RGBA while forcing opaque alpha
func CopyRGBToRGBA(dst, src []byte) {
	di := 0
	for si := 0; si+2 < len(src) && di+3 < len(dst); si, di = si+3, di+4 {
		dst[di] = src[si]
		dst[di+1] = src[si+1]
		dst[di+2] = src[si+2]
		dst[di+3] = 0xff
	}
}

// UI utilities

// MakeIconButton creates a low-importance button with a symbol
func MakeIconButton(symbol, tooltip string, tapped func()) *widget.Button {
	btn := widget.NewButton(symbol, tapped)
	btn.Importance = widget.LowImportance
	return btn
}

// LoadAppIcon loads the application icon from standard locations
func LoadAppIcon() fyne.Resource {
	var iconFiles []string
	if runtime.GOOS == "windows" {
		iconFiles = []string{"VT_Icon.ico"}
	} else {
		iconFiles = []string{"VT_Icon.png"}
	}
	var search []string

	// Search in current directory first
	for _, iconFile := range iconFiles {
		search = append(search, filepath.Join("assets", "logo", iconFile))
	}

	// Then search relative to executable
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, iconFile := range iconFiles {
			search = append(search, filepath.Join(dir, "assets", "logo", iconFile))
		}
	}

	for _, p := range search {
		if _, err := os.Stat(p); err == nil {
			res, err := fyne.LoadResourceFromPath(p)
			if err != nil {
				logging.Debug(logging.CatUI, "failed to load icon %s: %v", p, err)
				continue
			}
			logging.Debug(logging.CatUI, "loaded app icon from %s", p)
			return res
		}
	}
	logging.Debug(logging.CatUI, "no app icon found in search paths")
	return nil
}

var tempDirOverride atomic.Value

// SetTempDir overrides the app temp directory (empty string resets to system temp).
func SetTempDir(path string) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		tempDirOverride.Store("")
		return
	}
	tempDirOverride.Store(trimmed)
}

// TempDir returns the app temp directory, falling back to the system temp dir.
func TempDir() string {
	if v := tempDirOverride.Load(); v != nil {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return os.TempDir()
}
