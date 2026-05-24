package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/fontutil"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/app/appcfg"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/app/configpath"
	convertmodule "git.leaktechnologies.dev/leak_technologies/VideoTools/internal/app/modules/convert"
	queuemodule "git.leaktechnologies.dev/leak_technologies/VideoTools/internal/app/modules/queue"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/app/modules/trim"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/app/recentfiles"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/benchmark"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/convert"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/interlace"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/media"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/modules"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/player"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/queue"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/smpte"
	statepkg 	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/state"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/sysinfo"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/thumbnail"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/ui"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/utils"
	guitutils "git.leaktechnologies.dev/leak_technologies/VideoTools/internal/utils"
)

func ShowErrorLarge(err error, w fyne.Window) {
	scroll := container.NewScroll(widget.NewLabel(err.Error()))
	scroll.SetMinSize(fyne.NewSize(400, 200))
	d := dialog.NewCustom("Error", "OK", scroll, w)
	d.Show()
}

// abs returns the absolute value of an int32
func abs(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

// Module describes a high level tool surface that gets a tile on the menu.
type Module struct {
	ID        string
	Label     string
	Color     color.Color
	TextColor color.Color // foreground color for tile label (white or black)
	Category  string
	Handle    func(files []string)
}

var (
	debugFlag = flag.Bool("debug", false, "enable verbose logging (env: VIDEOTOOLS_DEBUG=1)")

	backgroundColor = ui.BgBase
	gridColor       = ui.BorderDim
	textColor       = ui.Text
	queueColor      = ui.Magenta

	conversionLogSuffix = ".videotools.log"

	logsDirDefaultOnce sync.Once
	logsDirDefault     string
	logsDirOverride    string
	logsDirMu          sync.RWMutex
	feedbackBundler    = utils.NewFeedbackBundler()
	appVersion         = "v0.1.1-dev50"
	buildCommit        = "dev"

	hwAccelProbeOnce sync.Once
	hwAccelSupported atomic.Value // map[string]bool

	hwProbesMu          sync.Mutex
	nvencProbeOK        *bool // nil = not yet probed or last probe failed; non-nil = confirmed available
	av1NvencProbeOK     *bool // separate: av1_nvenc requires Ada Lovelace (RTX 40xx+); Ampere fails
	qsvProbeOK          *bool
	av1QsvProbeOK       *bool // separate: av1_qsv requires Arc/Meteor Lake; older iGPUs fail
	vaapiProbeOK        *bool
	videotoolboxProbeOK *bool
	amfProbeOK          *bool
	av1AmfProbeOK       *bool // separate: av1_amf requires RDNA 3 (RX 7000+); RDNA 2 fails

	// 14-step HSL spectrum: H steps ~25.7° from VT_Purple (H=267°).
	// S=70% throughout; L is reduced for bright hues (yellow/green) so white
	// text always has sufficient contrast (≥3.3:1 for large bold labels).
	modulesList = []Module{
		{"convert", "Convert", utils.MustHex("#7225D0"), color.White, "Convert", modules.HandleConvert},
		{"merge", "Merge", utils.MustHex("#B423C7"), color.White, "Convert", modules.HandleMerge},
		{"trim", "Trim", utils.MustHex("#BF2290"), color.White, "Convert", modules.HandleTrim},
		{"filters", "Filters", utils.MustHex("#BF224C"), color.White, "Convert", modules.HandleFilters},
		{"audio", "Audio", utils.MustHex("#BF3C22"), color.White, "Convert", modules.HandleAudio},
		{"subtitles", "Subtitles", utils.MustHex("#AD741F"), color.White, "Convert", modules.HandleSubtitles},
		{"compare", "Compare", utils.MustHex("#91931A"), color.White, "Inspect", modules.HandleCompare},
		{"inspect", "Inspect", utils.MustHex("#629C1C"), color.White, "Inspect", modules.HandleInspect},
		{"upscale", "Upscale", utils.MustHex("#2B9C1C"), color.White, "Advanced", modules.HandleUpscale},
		{"author", "Author", utils.MustHex("#1C9C44"), color.White, "Disc", modules.HandleAuthor},
		{"rip", "Rip", utils.MustHex("#1A9373"), color.White, "Disc", modules.HandleRip},
		{"burn", "Burn", utils.MustHex("#178C8C"), color.White, "Disc", nil},
		{"filemanager", "Files", utils.MustHex("#0D7C8C"), color.White, "Disc", nil},
		{"player", "Player", utils.MustHex("#1D8EA5"), color.White, "Playback", modules.HandlePlayer},
		{"thumbnail", "Thumbnail", utils.MustHex("#2260BF"), color.White, "Screenshots", modules.HandleThumbnail},
		{"settings", "Settings", utils.MustHex("#2825D0"), color.White, "Settings", nil},
	}

	// Platform-specific configuration
	// platformConfig *PlatformConfig // Global platformConfig is now managed directly by utils.GetFFmpegPath and utils.GetFFprobePath
)

// moduleColor returns the color for a given module ID
func moduleColor(id string) color.Color {
	for _, m := range modulesList {
		if m.ID == id {
			return m.Color
		}
	}
	return queueColor
}

func openURL(url string) error {
	switch runtime.GOOS {
	case "windows":
		return utils.HideWindowExec("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

// statusStrip renders a consistent dark status area with the shared stats bar.
// recoloredSVG wraps an SVG icon resource and substitutes its fill colour.
// All assets use #E3E3E3 as the base fill; pass any hex colour to override.
type recoloredSVG struct {
	fyne.Resource
	hex string // e.g. "#4CE870"
}

// Name includes the hex colour so each tint gets its own SVG cache entry.
// Without this, Fyne's SVG cache is keyed only on the resource name and the
// first colour rendered for an icon is returned for all subsequent tints.
func (r recoloredSVG) Name() string {
	return strings.ToLower(r.hex) + "_" + r.Resource.Name()
}

func (r recoloredSVG) Content() []byte {
	s := string(r.Resource.Content())
	s = strings.ReplaceAll(s, "#e3e3e3", strings.ToLower(r.hex))
	s = strings.ReplaceAll(s, "#E3E3E3", strings.ToUpper(r.hex))
	return []byte(s)
}

func statusStrip(bar *ui.ConversionStatsBar) fyne.CanvasObject {
	bg := canvas.NewRectangle(color.NRGBA{R: 34, G: 34, B: 34, A: 255})
	bg.SetMinSize(fyne.NewSize(0, 32))
	// Make the entire bar area clickable by letting the bar fill the strip
	content := container.NewPadded(container.NewMax(bar))
	return container.NewMax(bg, content)
}

func fullVersion() string {
	if buildCommit == "" || buildCommit == "dev" {
		return appVersion
	}
	return fmt.Sprintf("%s-%s", appVersion, buildCommit)
}

func platformTag() string {
	platform := "linux"
	switch runtime.GOOS {
	case "windows":
		platform = "win"
	case "linux":
		platform = "linux"
	default:
		platform = runtime.GOOS
	}
	return platform
}

func versionWithPlatform() string {
	return fmt.Sprintf("%s_%s", fullVersion(), platformTag())
}

// moduleFooter stacks a dark status strip above a tinted action/footer band.
// If content is nil, a spacer is used for consistent height/color.
func moduleFooter(tint color.Color, content fyne.CanvasObject, bar *ui.ConversionStatsBar) fyne.CanvasObject {
	if content == nil {
		content = layout.NewSpacer()
	}
	bg := canvas.NewRectangle(tint)
	bg.SetMinSize(fyne.NewSize(0, 44))
	tinted := container.NewMax(bg, container.NewPadded(content))
	return container.NewVBox(statusStrip(bar), tinted)
}

type fixedHSplitLayout struct {
	ratio float32
}

func (l *fixedHSplitLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 2 {
		return
	}
	lead := objects[0]
	trail := objects[1]
	total := float64(size.Width)
	if total <= 0 {
		return
	}

	ratio := float64(l.ratio)
	if ratio <= 0 {
		ratio = 0.6
	}
	// Much more flexible split - allow dragging from 5% to 95%
	if ratio < 0.05 {
		ratio = 0.05
	} else if ratio > 0.95 {
		ratio = 0.95
	}

	leadWidth := float32(total * ratio)
	trailWidth := size.Width - leadWidth
	lead.Move(fyne.NewPos(0, 0))
	lead.Resize(fyne.NewSize(leadWidth, size.Height))
	trail.Move(fyne.NewPos(leadWidth, 0))
	trail.Resize(fyne.NewSize(trailWidth, size.Height))
}

func (l *fixedHSplitLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) < 2 {
		return fyne.NewSize(0, 0)
	}
	// Allow the window to shrink without being constrained by child min sizes.
	return fyne.NewSize(0, 0)
}

func displayAspectRatioForSource(src *videoSource) float64 {
	if src == nil {
		return 0
	}
	if val := utils.ParseAspectValue(strings.TrimSpace(src.DisplayAspectRatio)); val > 0 {
		return val
	}
	return utils.DisplayAspectRatioFloat(src.Width, src.Height, src.SampleAspectRatio)
}

func displayAspectRatioFromConfig(cfg map[string]interface{}) float64 {
	if cfg == nil {
		return 0
	}
	if dar, ok := cfg["displayAspectRatio"].(string); ok {
		if val := utils.ParseAspectValue(strings.TrimSpace(dar)); val > 0 {
			return val
		}
	}
	sourceWidth, _ := cfg["sourceWidth"].(int)
	sourceHeight, _ := cfg["sourceHeight"].(int)
	sampleAspectRatio, _ := cfg["sampleAspectRatio"].(string)
	return utils.DisplayAspectRatioFloat(sourceWidth, sourceHeight, sampleAspectRatio)
}

// resolveTargetAspect resolves an aspect ratio value or source aspect
func resolveTargetAspect(val string, src *videoSource) float64 {
	if strings.EqualFold(val, "source") {
		if src != nil {
			return displayAspectRatioForSource(src)
		}
		return 0
	}
	if r := utils.ParseAspectValue(val); r > 0 {
		return r
	}
	return 0
}

func createConversionLog(inputPath, outputPath string, args []string) (*os.File, string, error) {
	base := strings.TrimSuffix(filepath.Base(outputPath), filepath.Ext(outputPath))
	logPath := filepath.Join(getLogsDir(), base+conversionLogSuffix)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, logPath, fmt.Errorf("create log dir: %w", err)
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, logPath, err
	}
	header := fmt.Sprintf(`VideoTools Conversion Log
Version: %s
Started: %s
Input: %s
Output: %s
Command: ffmpeg %s

`, fullVersion(), time.Now().Format(time.RFC3339), inputPath, outputPath, strings.Join(args, " "))
	if _, err := f.WriteString(header); err != nil {
		_ = f.Close()
		return nil, logPath, err
	}
	return f, logPath, nil
}

func defaultVideoToolsRoot() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, "Videos", "VideoTools")
	}
	if cfgDir, err := os.UserConfigDir(); err == nil && cfgDir != "" {
		return filepath.Join(cfgDir, "VideoTools")
	}
	return ""
}

func defaultLogsDir() string {
	logsDirDefaultOnce.Do(func() {
		root := defaultVideoToolsRoot()
		if root != "" {
			logsDirDefault = filepath.Join(root, "logs")
		} else {
			logsDirDefault = filepath.Join(".", "logs")
		}
	})
	return logsDirDefault
}

func setLogsDirOverride(dir string) {
	logsDirMu.Lock()
	logsDirOverride = strings.TrimSpace(dir)
	logsDirMu.Unlock()
}

func getLogsDir() string {
	logsDirMu.RLock()
	override := logsDirOverride
	logsDirMu.RUnlock()
	if override != "" {
		return override
	}
	return defaultLogsDir()
}

// defaultBitrate picks a sane default when user leaves bitrate empty in bitrate modes.
func defaultBitrate(codec string, width int, sourceBitrate int) string {
	if sourceBitrate > 0 {
		return fmt.Sprintf("%dk", sourceBitrate/1000)
	}
	switch strings.ToLower(codec) {
	case "h.265", "hevc", "libx265", "hevc_nvenc", "hevc_qsv", "hevc_amf", "hevc_videotoolbox":
		if width >= 1920 {
			return "3500k"
		}
		if width >= 1280 {
			return "2000k"
		}
		return "1200k"
	case "av1", "libaom-av1", "av1_nvenc", "av1_amf", "av1_qsv", "av1_vaapi":
		if width >= 1920 {
			return "2800k"
		}
		if width >= 1280 {
			return "1600k"
		}
		return "1000k"
	default:
		if width >= 1920 {
			return "4500k"
		}
		if width >= 1280 {
			return "2500k"
		}
		return "1500k"
	}
}

// parseBitrateStringToBPS converts a bitrate string like "5000k", "5M", or "5000000"
// to bits-per-second. Returns 0 if the string cannot be parsed.
func parseBitrateStringToBPS(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	upper := strings.ToUpper(s)
	var numStr string
	var multiplier int
	switch {
	case strings.HasSuffix(upper, "M"):
		numStr = s[:len(s)-1]
		multiplier = 1_000_000
	case strings.HasSuffix(upper, "K"):
		numStr = s[:len(s)-1]
		multiplier = 1_000
	default:
		numStr = s
		multiplier = 1
	}
	val, err := utils.ParseInt(numStr)
	if err != nil || val <= 0 {
		return 0
	}
	return val * multiplier
}

// effectiveHardwareAccel resolves "auto" to a best-effort hardware encoder for the platform.
// It does actual runtime checks to find the best available encoder.
func effectiveHardwareAccel(cfg convertConfig) string {
	accel := strings.ToLower(cfg.HardwareAccel)
	if accel != "" && accel != "auto" {
		// User explicitly chose something - verify it's actually available
		if accel == "none" || hwAccelAvailable(accel) {
			return accel
		}
		// User chose something that isn't available - fall back to detection
		logging.Info(logging.CatFFMPEG, "user-selected hardware accel '%s' not available, auto-detecting", accel)
	}

	// Auto-detect the best available encoder
	detected := detectBestHardwareAccel()
	if detected != "none" {
		return detected
	}

	// No hardware acceleration available - use software
	return "none"
}

// detectBestHardwareAccel probes available hardware acceleration backends.
// Returns "none" if no known backend is available.
func detectBestHardwareAccel() string {
	if runtime.GOOS == "windows" {
		if hwAccelAvailable("nvenc") {
			return "nvenc"
		}
		if hwAccelAvailable("qsv") {
			return "qsv"
		}
		if hwAccelAvailable("amf") {
			return "amf"
		}
		return "none"
	}
	// Linux
	if hwAccelAvailable("nvenc") {
		return "nvenc"
	}
	if hwAccelAvailable("qsv") {
		return "qsv"
	}
	if hwAccelAvailable("vaapi") {
		return "vaapi"
	}
	if hwAccelAvailable("amf") {
		return "amf"
	}
	return "none"
}

// detectHardwareAccelStatus probes all hardware encoders and returns a multi-line status
// string describing the system hardware and which encoders are available.
func detectHardwareAccelStatus() (best string, status string) {
	hw := sysinfo.Detect()

	check := func(accel string) string {
		if hwAccelAvailable(accel) {
			return "✓"
		}
		return "✗"
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("CPU: %s (%d cores)", hw.CPU, hw.CPUCores))
	if hw.GPU != "" && hw.GPU != "No GPU detected" {
		lines = append(lines, fmt.Sprintf("GPU: %s", hw.GPU))
	}

	if runtime.GOOS == "windows" {
		lines = append(lines,
			fmt.Sprintf("NVENC (NVIDIA):  %s", check("nvenc")),
			fmt.Sprintf("QSV   (Intel):   %s", check("qsv")),
			fmt.Sprintf("AMF   (AMD):     %s", check("amf")),
		)
	} else {
		lines = append(lines,
			fmt.Sprintf("NVENC (NVIDIA):  %s", check("nvenc")),
			fmt.Sprintf("QSV   (Intel):   %s", check("qsv")),
			fmt.Sprintf("VAAPI (Linux):   %s", check("vaapi")),
			fmt.Sprintf("AMF   (AMD):     %s", check("amf")),
		)
	}

	best = detectBestHardwareAccel()
	lines = append(lines, fmt.Sprintf("Selected: %s", best))
	return best, strings.Join(lines, "\n")
}

// hwAccelAvailable checks if the hardware acceleration is actually usable on this system.
// It does a runtime probe to verify the hardware/driver is present and working.
func hwAccelAvailable(accel string) bool {
	accel = strings.ToLower(accel)
	if accel == "" || accel == "none" || accel == "auto" {
		return false
	}

	// Do a runtime check for each acceleration method
	switch accel {
	case "nvenc":
		return checkNvencRuntime()
	case "qsv":
		return checkQsvRuntime()
	case "vaapi":
		return checkVaapiRuntime()
	case "amf":
		return checkAmfRuntime()
	default:
		return false
	}
}

// probeHWAccel runs an FFmpeg null-encode probe and caches the result.
// Successes are cached permanently (hardware won't disappear mid-session).
// Failures are not cached — a later call may retry (handles startup timing issues).
func probeHWAccel(cached **bool, args []string, label string) bool {
	hwProbesMu.Lock()
	defer hwProbesMu.Unlock()
	if *cached != nil {
		return **cached
	}
	cmd := utils.CreateCommandRaw(utils.GetFFmpegPath(), args...)
	// Use CombinedOutput so the subprocess has a valid stdout/stderr handle.
	// In a Windows GUI app (-H windowsgui) there is no console, so the inherited
	// stdout handle is null/invalid; FFmpeg fails writing "-f null -" unless we
	// give it a real pipe to write into.
	out, err := cmd.CombinedOutput()
	if err != nil {
		logging.Info(logging.CatFFMPEG, "%s runtime check failed: %v — %s", label, err, strings.TrimSpace(string(out)))
		return false
	}
	ok := true
	*cached = &ok
	logging.Info(logging.CatFFMPEG, "%s runtime check passed", label)
	return true
}

// hwProbeSource is a lavfi color source that outputs yuv420p — the format all
// hardware encoders (NVENC, QSV, AMF, VAAPI) accept directly. nullsrc produces
// an unformatted raw buffer which can cause format negotiation failures.
// 192x192 is the minimum supported by NVENC on modern drivers (older require 256+).
// Using 1920x1080 to ensure compatibility across all hardware encoders.
const hwProbeSource = "color=black:size=1920x1080:rate=25"

// checkNvencRuntime does a real encode probe to verify NVIDIA GPU + drivers are working.
func checkNvencRuntime() bool {
	return probeHWAccel(&nvencProbeOK, []string{
		"-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", hwProbeSource,
		"-frames:v", "1", "-c:v", "h264_nvenc",
		"-f", "null", "-",
	}, "nvenc")
}

// checkAV1NvencRuntime probes av1_nvenc specifically.
// av1_nvenc requires Ada Lovelace (RTX 40-series+); Ampere (RTX 30-series) and older
// will pass the h264_nvenc probe but fail at runtime for AV1. Without this separate
// probe we incorrectly try av1_nvenc on Ampere GPUs and get "No capable devices found".
func checkAV1NvencRuntime() bool {
	return probeHWAccel(&av1NvencProbeOK, []string{
		"-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", hwProbeSource,
		"-frames:v", "1", "-c:v", "av1_nvenc",
		"-f", "null", "-",
	}, "av1_nvenc")
}

// checkQsvRuntime does a real encode probe to verify Intel Quick Sync is available.
func checkQsvRuntime() bool {
	return probeHWAccel(&qsvProbeOK, []string{
		"-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", hwProbeSource,
		"-frames:v", "1", "-c:v", "h264_qsv",
		"-preset", "veryfast",
		"-f", "null", "-",
	}, "qsv")
}

// checkAV1QsvRuntime probes av1_qsv specifically.
// av1_qsv requires Intel Arc or Meteor Lake iGPU; older iGPUs pass the h264_qsv
// probe but fail at runtime for AV1.
func checkAV1QsvRuntime() bool {
	return probeHWAccel(&av1QsvProbeOK, []string{
		"-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", hwProbeSource,
		"-frames:v", "1", "-c:v", "av1_qsv",
		"-f", "null", "-",
	}, "av1_qsv")
}

// checkAV1AmfRuntime probes av1_amf specifically.
// av1_amf requires RDNA 3 (RX 7000+); RDNA 2 passes the h264_amf probe but
// fails at runtime for AV1.
func checkAV1AmfRuntime() bool {
	return probeHWAccel(&av1AmfProbeOK, []string{
		"-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", hwProbeSource,
		"-frames:v", "1", "-c:v", "av1_amf",
		"-f", "null", "-",
	}, "av1_amf")
}

// checkVaapiRuntime does a real encode probe to verify VAAPI (Linux) is working.
func checkVaapiRuntime() bool {
	return probeHWAccel(&vaapiProbeOK, []string{
		"-hide_banner", "-loglevel", "error",
		"-vaapi_device", "/dev/dri/renderD128",
		"-f", "lavfi", "-i", hwProbeSource,
		"-frames:v", "1",
		"-vf", "format=nv12,hwupload",
		"-c:v", "h264_vaapi",
		"-f", "null", "-",
	}, "vaapi")
}

// checkVideotoolboxRuntime does a real encode probe to verify macOS VideoToolbox is working.
func checkVideotoolboxRuntime() bool {
	return probeHWAccel(&videotoolboxProbeOK, []string{
		"-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", hwProbeSource,
		"-frames:v", "1", "-c:v", "h264_videotoolbox",
		"-f", "null", "-",
	}, "videotoolbox")
}

// checkAmfRuntime does a real encode probe to verify AMD GPU + drivers are working.
// AMF is AMD's encoder API - it requires AMD GPU with proper drivers.
// Note: h264_amf does not use -preset; quality is set via -quality (speed|balanced|quality).
func checkAmfRuntime() bool {
	return probeHWAccel(&amfProbeOK, []string{
		"-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", hwProbeSource,
		"-frames:v", "1", "-c:v", "h264_amf",
		"-f", "null", "-",
	}, "amf")
}

// openLogViewer opens a simple dialog showing the log content. If live is true, it auto-refreshes.
func (s *appState) openLogViewer(title, path string, live bool) {
	t := i18n.T()
	if strings.TrimSpace(path) == "" {
		dialog.ShowInformation(t.DialogNoLog, "No log available.", s.window)
		return
	}

	// Create UI elements first
	text := widget.NewMultiLineEntry()
	text.SetText("Loading log file...")
	text.Wrapping = fyne.TextWrapWord
	text.TextStyle = fyne.TextStyle{Monospace: true}
	text.Disable()
	bg := canvas.NewRectangle(color.NRGBA{0x15, 0x1a, 0x24, 0xff}) // slightly lighter than app bg
	scroll := container.NewVScroll(container.NewMax(bg, text))
	// Adaptive min size - allows proper scaling on small screens
	// scroll.SetMinSize(fyne.NewSize(600, 350)) // Removed for flexible sizing

	stop := make(chan struct{})
	var d dialog.Dialog
	closeBtn := ui.MakePillButton("Close", ui.BorderDim, func() {
		if d != nil {
			d.Hide()
		}
	})
	copyBtn := ui.MakePillButton("Copy All", ui.BorderDim, func() {
		s.window.Clipboard().SetContent(text.Text)
	})
	buttons := container.NewHBox(copyBtn, layout.NewSpacer(), closeBtn)
	content := container.NewBorder(nil, buttons, nil, nil, scroll)
	d = dialog.NewCustom(title, "Close", content, s.window)
	d.SetOnClosed(func() { close(stop) })
	d.Show()

	readTail := func() string {
		const maxBytes int64 = 200 * 1024
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Sprintf("Failed to read log: %v", err)
		}
		size := info.Size()
		start := int64(0)
		if size > maxBytes {
			start = size - maxBytes
		}
		f, err := os.Open(path)
		if err != nil {
			return fmt.Sprintf("Failed to read log: %v", err)
		}
		defer f.Close()
		if start > 0 {
			if _, err := f.Seek(start, io.SeekStart); err != nil {
				return fmt.Sprintf("Failed to read log: %v", err)
			}
		}
		data, err := io.ReadAll(f)
		if err != nil {
			return fmt.Sprintf("Failed to read log: %v", err)
		}
		if start > 0 {
			return fmt.Sprintf("... showing last %d KB ...\n%s", maxBytes/1024, string(data))
		}
		return string(data)
	}

	// Read file asynchronously to avoid blocking UI
	go func() {
		content := readTail()
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			text.SetText(content)
			// Auto-scroll to bottom
			scroll.ScrollToBottom()
		}, false)

		// Start live updates if requested
		if live {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			var lastSize int64 = -1
			var lastText string
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					info, err := os.Stat(path)
					if err != nil {
						continue
					}
					if info.Size() == lastSize {
						continue
					}
					lastSize = info.Size()
					content := readTail()
					if content == lastText {
						continue
					}
					lastText = content
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						text.SetText(content)
					}, false)
				}
			}
		}
	}()
}

// openFolder tries to open a folder in the OS file browser.
// pathsAreSameFile reports whether a and b point to the same file on disk.
// Falls back to case-insensitive path comparison when the file doesn't exist yet.
func pathsAreSameFile(a, b string) bool {
	ai, err1 := os.Stat(a)
	bi, err2 := os.Stat(b)
	if err1 == nil && err2 == nil {
		return os.SameFile(ai, bi)
	}
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func openFolder(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is empty")
	}
	p := filepath.Clean(filepath.FromSlash(path))
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	info, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			if mkErr := os.MkdirAll(p, 0o755); mkErr != nil {
				return fmt.Errorf("failed to create folder: %w", mkErr)
			}
		} else {
			return err
		}
	} else if !info.IsDir() {
		p = filepath.Dir(p)
	}

	// Use exec.Command directly — CreateCommandRaw sets CREATE_NO_WINDOW which
	// silently hides GUI apps like Explorer and Finder. These are GUI launchers,
	// not CLI tools, so they must not have the window-suppression flag set.
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", p)
	default:
		cmd = exec.Command("xdg-open", p)
	}
	return cmd.Start()
}

// openFile tries to open a file in the OS default viewer.
func openFile(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is empty")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		p := filepath.Clean(filepath.FromSlash(path))
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		cmd = utils.CreateCommandRaw("cmd", "/c", "start", "", p)
	default:
		cmd = utils.CreateCommandRaw("xdg-open", path)
	}
	return cmd.Start()
}

// formatOption and hwPreset are type aliases to the canonical definitions in internal/convert.
type formatOption = convert.FormatOption
type hwPreset = convert.HWPreset

var formatOptions = convert.FormatOptions
var hwPresets = convert.HWPresets

// ensureCompatibleCodec adjusts cfg's VideoCodec and AudioCodec to be compatible
// with the selected format. If the current codec is incompatible, it is replaced
// with the first compatible option.
func ensureCompatibleCodec(cfg *convertConfig) {
	ext := strings.ToLower(cfg.SelectedFormat.Ext)
	if compatibleVideo, ok := convert.FormatVideoCodecs[ext]; ok && len(compatibleVideo) > 0 {
		found := false
		for _, c := range compatibleVideo {
			if c == cfg.VideoCodec {
				found = true
				break
			}
		}
		if !found {
			cfg.VideoCodec = compatibleVideo[0]
		}
	}
	if compatibleAudio, ok := convert.FormatAudioCodecs[ext]; ok && len(compatibleAudio) > 0 {
		found := false
		for _, c := range compatibleAudio {
			if c == cfg.AudioCodec {
				found = true
				break
			}
		}
		if !found {
			cfg.AudioCodec = compatibleAudio[0]
		}
	}
}

type convertConfig struct {
	OutputBase       string
	OutputDir        string
	SelectedFormat   formatOption
	Quality          string // Preset quality (Draft/Standard/High/Lossless)
	Mode             string // Simple or Advanced
	UseAutoNaming    bool
	AutoNameTemplate string // Template for metadata-driven naming, e.g., "<actress> - <studio> - <scene>"
	AppendSuffix     bool   // Append "-convert" suffix to output filename (off by default)
	PreserveChapters bool

	// Video encoding settings
	VideoCodec             string // H.264, H.265, VP9, AV1, Copy
	EncoderPreset          string // ultrafast, superfast, veryfast, faster, fast, medium, slow, slower, veryslow
	CRF                    string // Manual CRF value (0-51, or empty to use Quality preset)
	BitrateMode            string // CRF, CBR, VBR, "Target Size"
	BitratePreset          string // Friendly bitrate presets (codec-aware recommendations)
	VideoBitrate           string // For CBR/VBR modes (e.g., "5000k")
	TargetFileSize         string // Target file size (e.g., "25MB", "100MB") - requires BitrateMode="Target Size"
	TargetResolution       string // Source, 720p, 1080p, 1440p, 4K, or custom
	FrameRate              string // Source, 24, 30, 60, or custom
	UseMotionInterpolation bool   // Use motion interpolation for smooth frame rate changes
	PixelFormat            string // yuv420p, yuv422p, yuv444p
	HardwareAccel          string // auto, none, nvenc, amf, vaapi, qsv, videotoolbox
	TwoPass                bool   // Enable two-pass encoding for VBR
	EncoderTune            string // None, Film, Animation, Grain, Stillimage, Fastdecode (libx264/libx265 only)
	H264Profile            string // baseline, main, high (for H.264 compatibility)
	H264Level              string // 3.0, 3.1, 4.0, 4.1, 5.0, 5.1 (for H.264 compatibility)
	Deinterlace            string // Auto, Force, Off
	DeinterlaceMethod      string // yadif, bwdif (bwdif is higher quality but slower)
	AutoCrop               bool   // Auto-detect and remove black bars
	CropWidth              string // Manual crop width (empty = use auto-detect)
	CropHeight             string // Manual crop height (empty = use auto-detect)
	CropX                  string // Manual crop X offset (empty = use auto-detect)
	CropY                  string // Manual crop Y offset (empty = use auto-detect)
	FlipHorizontal         bool   // Flip video horizontally (mirror)
	FlipVertical           bool   // Flip video vertically (upside down)
	Rotation               string // 0, 90, 180, 270 (clockwise rotation in degrees)

	// Audio encoding settings
	AudioCodec      string // AAC, Opus, MP3, FLAC, Copy
	AudioBitrate    string // 128k, 192k, 256k, 320k
	AudioChannels   string // Source, Mono, Stereo, 5.1
	AudioSampleRate string // Source, 44100, 48000
	NormalizeAudio  bool   // Force stereo + 48kHz for compatibility
	NormalizeLUFS   float64 // LUFS target (default -16)
	NormalizeTruePeak float64 // TruePeak max (default -1.5)

	// Other settings
	InverseTelecine  bool
	InverseAutoNotes string
	CoverArtPath     string
	AspectHandling   string
	OutputAspect     string
	AspectUserSet    bool   // Tracks if user explicitly set OutputAspect
	ForceAspect      bool   // Force DAR/SAR metadata even when no aspect conversion
	TempDir          string // Optional temp/cache directory override
	LogDir           string // Optional log directory override

	// Master settings
	ShowUpscale bool
	ShowDisc    bool
}

type convertRecoveryState = appcfg.ConvertRecoveryState

// userPreset captures all encoding-relevant convertConfig fields under a user-defined name.
// Output paths, UI mode, and source-specific values are intentionally excluded.
type userPreset struct {
	Name                   string       `json:"name"`
	SelectedFormat         formatOption `json:"selectedFormat"`
	VideoCodec             string       `json:"videoCodec"`
	EncoderPreset          string       `json:"encoderPreset"`
	Quality                string       `json:"quality"`
	BitrateMode            string       `json:"bitrateMode"`
	BitratePreset          string       `json:"bitratePreset"`
	VideoBitrate           string       `json:"videoBitrate"`
	CRF                    string       `json:"crf"`
	TargetFileSize         string       `json:"targetFileSize"`
	TargetResolution       string       `json:"targetResolution"`
	FrameRate              string       `json:"frameRate"`
	UseMotionInterpolation bool         `json:"useMotionInterpolation"`
	PixelFormat            string       `json:"pixelFormat"`
	HardwareAccel          string       `json:"hardwareAccel"`
	TwoPass                bool         `json:"twoPass"`
	EncoderTune            string       `json:"encoderTune"`
	H264Profile            string       `json:"h264Profile"`
	H264Level              string       `json:"h264Level"`
	AudioCodec             string       `json:"audioCodec"`
	AudioBitrate           string       `json:"audioBitrate"`
	AudioChannels          string       `json:"audioChannels"`
	AudioSampleRate        string       `json:"audioSampleRate"`
	NormalizeAudio         bool         `json:"normalizeAudio"`
	NormalizeLUFS          float64      `json:"normalizeLUFS"`
	NormalizeTruePeak      float64      `json:"normalizeTruePeak"`
	OutputAspect           string       `json:"outputAspect"`
	AspectHandling         string       `json:"aspectHandling"`
	ForceAspect            bool         `json:"forceAspect"`
	PreserveChapters       bool         `json:"preserveChapters"`
}

type userPresetsConfig struct {
	Presets []userPreset `json:"presets"`
}

// userPresetFromConfig captures all encoding-relevant fields from the current config.
func userPresetFromConfig(name string, cfg convertConfig) userPreset {
	return userPreset{
		Name:                   name,
		SelectedFormat:         cfg.SelectedFormat,
		VideoCodec:             cfg.VideoCodec,
		EncoderPreset:          cfg.EncoderPreset,
		Quality:                cfg.Quality,
		BitrateMode:            cfg.BitrateMode,
		BitratePreset:          cfg.BitratePreset,
		VideoBitrate:           cfg.VideoBitrate,
		CRF:                    cfg.CRF,
		TargetFileSize:         cfg.TargetFileSize,
		TargetResolution:       cfg.TargetResolution,
		FrameRate:              cfg.FrameRate,
		UseMotionInterpolation: cfg.UseMotionInterpolation,
		PixelFormat:            cfg.PixelFormat,
		HardwareAccel:          cfg.HardwareAccel,
		TwoPass:                cfg.TwoPass,
		EncoderTune:            cfg.EncoderTune,
		H264Profile:            cfg.H264Profile,
		H264Level:              cfg.H264Level,
		AudioCodec:             cfg.AudioCodec,
		AudioBitrate:           cfg.AudioBitrate,
		AudioChannels:          cfg.AudioChannels,
		AudioSampleRate:        cfg.AudioSampleRate,
		NormalizeAudio:         cfg.NormalizeAudio,
		NormalizeLUFS:           cfg.NormalizeLUFS,
		NormalizeTruePeak:      cfg.NormalizeTruePeak,
		OutputAspect:           cfg.OutputAspect,
		AspectHandling:         cfg.AspectHandling,
		ForceAspect:            cfg.ForceAspect,
		PreserveChapters:       cfg.PreserveChapters,
	}
}

func loadUserPresets() ([]userPreset, error) {
	var cfg userPresetsConfig
	if _, err := appcfg.LoadModuleJSON("user_presets", &cfg); err != nil {
		return nil, err
	}
	return cfg.Presets, nil
}

func saveUserPresets(presets []userPreset) error {
	return appcfg.SaveModuleJSON("user_presets", userPresetsConfig{Presets: presets})
}

func (c convertConfig) OutputFile() string {
	base := strings.TrimSpace(c.OutputBase)
	if base == "" {
		base = "converted"
	}
	return base + c.SelectedFormat.Ext
}

func (c convertConfig) CoverLabel() string {
	if strings.TrimSpace(c.CoverArtPath) == "" {
		return "none"
	}
	return filepath.Base(c.CoverArtPath)
}

func defaultConvertConfig() convertConfig {
	return convertConfig{
		SelectedFormat:   formatOptions[0],
		OutputBase:       "converted",
		OutputDir:        "",
		Quality:          "Standard (CRF 23)",
		Mode:             "Simple",
		UseAutoNaming:    false,
		AutoNameTemplate: "<actress> - <studio> - <scene>",
		AppendSuffix:     false, // Don't append "-convert" by default
		PreserveChapters: true,

		VideoCodec:             "H.264",
		EncoderPreset:          "slow",
		CRF:                    "",
		BitrateMode:            "CBR",
		BitratePreset:          "2.5 Mbps - Medium Quality",
		VideoBitrate:           "5000k",
		TargetFileSize:         "",
		TargetResolution:       "Source",
		FrameRate:              "Source",
		UseMotionInterpolation: false,
		PixelFormat:            "yuv420p",
		HardwareAccel:          "auto",
		TwoPass:                false,
		H264Profile:            "main",
		H264Level:              "4.0",
		Deinterlace:            "Auto",
		DeinterlaceMethod:      "bwdif",
		AutoCrop:               false,
		CropWidth:              "",
		CropHeight:             "",
		CropX:                  "",
		CropY:                  "",
		FlipHorizontal:         false,
		FlipVertical:           false,
		Rotation:               "0",

		AudioCodec:      "AAC",
		AudioBitrate:    "192k",
		AudioChannels:   "Source",
		AudioSampleRate: "Source",
		NormalizeAudio:  false,
		NormalizeLUFS:   -16.0,
		NormalizeTruePeak: -1.5,

		InverseTelecine:  true,
		InverseAutoNotes: "Default smoothing for interlaced footage.",
		CoverArtPath:     "",
		AspectHandling:   "Auto",
		OutputAspect:     "Source",
		AspectUserSet:    false,
		ForceAspect:      true,
		TempDir:          "",
		LogDir:           "",
		ShowUpscale:      true,
		ShowDisc:         true,
	}
}

// defaultConvertConfigPath returns the path to the persisted convert config.
func loadConvertRecovery() (convertRecoveryState, error) {
	return appcfg.LoadConvertRecovery()
}

func saveConvertRecovery(state convertRecoveryState) error {
	return appcfg.SaveConvertRecovery(state)
}

// loadPersistedConvertConfig loads the saved convert configuration from disk.
func loadPersistedConvertConfig() (convertConfig, error) {
	var cfg convertConfig
	raw, err := appcfg.LoadModuleJSON("convert", &cfg)
	if err != nil {
		return cfg, err
	}
	if raw == nil {
		raw = map[string]json.RawMessage{}
	}
	norm := appcfg.NormalizeConvertFields(
		raw,
		cfg.ForceAspect,
		cfg.ShowUpscale,
		cfg.ShowDisc,
		cfg.OutputAspect,
		cfg.AspectUserSet,
		cfg.FrameRate,
		cfg.BitrateMode,
	)
	cfg.ForceAspect = norm.ForceAspect
	cfg.ShowUpscale = norm.ShowUpscale
	cfg.ShowDisc = norm.ShowDisc
	cfg.OutputAspect = norm.OutputAspect
	cfg.AspectUserSet = norm.AspectUserSet
	cfg.FrameRate = norm.FrameRate
	cfg.BitrateMode = norm.BitrateMode
	return cfg, nil
}

// savePersistedConvertConfig writes the convert configuration to disk.
func savePersistedConvertConfig(cfg convertConfig) error {
	return appcfg.SaveModuleJSON("convert", cfg)
}

// benchmarkRun represents a single benchmark test run
type benchmarkRun = appcfg.BenchmarkRun

// benchmarkConfig holds benchmark history
type benchmarkConfig = appcfg.BenchmarkConfig

func loadBenchmarkConfig() (benchmarkConfig, error) {
	return appcfg.LoadBenchmarkConfig()
}

func saveBenchmarkConfig(cfg benchmarkConfig) error {
	return appcfg.SaveBenchmarkConfig(cfg)
}

// historyConfig holds conversion history
type historyConfig = appcfg.HistoryConfig

func loadHistoryConfig() (historyConfig, error) {
	return appcfg.LoadHistoryConfig()
}

func saveHistoryConfig(cfg historyConfig) error {
	return appcfg.SaveHistoryConfig(cfg)
}

type appState struct {
	window                    fyne.Window
	windowAutoSized           bool
	active                    string
	lastModule                string
	queueBackTarget           string
	queueLastRefresh          time.Time
	navigationHistory         []string // Track module navigation history for back/forward buttons
	navigationHistoryPosition int      // Current position in navigation history
	navigationHistorySuppress bool     // Temporarily suppress history tracking during navigation
	source                    *videoSource
	loadedVideos              []*videoSource // Multiple loaded videos for navigation
	currentIndex              int            // Current video index in loadedVideos
	anim                      *previewAnimator
	convert                   convertConfig
	prefs                     prefsConfig
	currentFrame              string
	player                    player.Controller
	playerReady               bool
	playerVolume              float64
	playerMuted               bool
	lastVolume                float64
	playerPaused              bool
	playerPos                 float64
	playerLast                time.Time
	progressQuit              chan struct{}
	convertCancel             context.CancelFunc
	playerSurf                *playerSurface
	convertBusy               bool
	convertStatus             string
	convertActiveIn           string
	convertActiveOut          string
	convertActiveLog          string
	convertProgress           float64
	convertFPS                float64
	convertSpeed              float64
	convertETA                time.Duration
	filterBusy                bool
	filterCancel              context.CancelFunc
	filterActiveIn            string
	filterActiveOut           string
	filterProgress            float64
	filterFPS                 float64
	filterSpeed               float64
	filterETA                 time.Duration
	recentFiles               *recentfiles.Manager
	jobQueue                  *queue.Queue
	statsBar                  *ui.ConversionStatsBar
	queueBtn                  *ui.PillButton
	queueScroll               *container.Scroll
	queueOffset               fyne.Position
	compareFile1              *videoSource
	compareFile2              *videoSource
	inspectFile               *videoSource
	inspectInterlaceResult    *interlace.DetectionResult
	inspectInterlaceAnalyzing bool
	autoCompare               bool // Auto-load Compare module after conversion
	convertCommandPreviewShow bool // Show FFmpeg command preview in Convert module
	convertScrollShortcuts    bool

	// Merge state
	mergeClips               []mergeClip
	mergeFormat              string
	mergeOutputDir           string
	mergeOutputFilename      string
	mergeKeepAll             bool
	mergeCodecMode           string
	mergeChapters            bool
	mergeDVDRegion           string // "NTSC" or "PAL"
	mergeDVDAspect           string // "16:9" or "4:3"
	mergeFrameRate           string // Source, 24, 30, 60, or custom
	mergeMotionInterpolation bool   // Use motion interpolation for frame rate changes

	// Thumbnail module state
	thumbnailFile           *videoSource
	thumbnailFiles          []*videoSource
	thumbnailCount          int
	thumbnailWidth          int
	thumbnailOutputMode     string
	thumbnailShowTimestamps bool
	thumbnailSheetWidth     int
	thumbnailColumns        int
	thumbnailRows           int
	thumbnailLastOutputPath string          // Path to last generated output
	thumbnailLiveGrid       *fyne.Container // Persistent live-preview grid; survives view rebuilds

	// Player module state
	playerFile *videoSource

	// Filters module state
	filtersFile         *videoSource
	filterBrightness    float64
	filterContrast      float64
	filterSaturation    float64
	filterSharpness     float64
	filterDenoise       float64
	filterRotation      int // 0, 90, 180, 270
	filterFlipH         bool
	filterFlipV         bool
	filterGrayscale     bool
	filterActiveChain   []string // Active filter chain
	filterInterpEnabled bool
	filterInterpPreset  string
	filterInterpFPS     string

	// Stylistic effects state
	filterStylisticMode string  // "None", "70s", "80s", "90s", "VHS", "Webcam"
	filterScanlines     bool    // CRT scanline effect
	filterChromaNoise   float64 // 0.0-1.0, analog chroma noise
	filterColorBleeding bool    // VHS color bleeding effect
	filterTapeNoise     float64 // 0.0-1.0, magnetic tape noise
	filterTrackingError float64 // 0.0-1.0, VHS tracking errors
	filterDropout       float64 // 0.0-1.0, tape dropouts
	filterInterlacing   string  // "None", "Progressive", "Interlaced"

	// Upscale module state
	upscaleFile                *videoSource
	upscaleMethod              string   // lanczos, bicubic, spline, bilinear
	upscaleTargetRes           string   // 720p, 1080p, 1440p, 4K, 8K, Custom
	upscaleCustomWidth         int      // For custom resolution
	upscaleCustomHeight        int      // For custom resolution
	upscaleQualityPreset       string   // Lossless, Near-lossless, High
	upscaleAIEnabled           bool     // Use AI upscaling if available
	upscaleAIModel             string   // realesrgan, realesrgan-anime, none
	upscaleAIAvailable         bool     // Runtime detection
	upscaleAIBackend           string   // ncnn, python
	upscaleAIPreset            string   // Ultra Fast, Fast, Balanced, High Quality, Maximum Quality
	upscaleAIScale             float64  // Base outscale when not matching target
	upscaleAIScaleUseTarget    bool     // Use target resolution to compute scale
	upscaleAIOutputAdjust      float64  // Post scale adjustment multiplier
	upscaleAIFaceEnhance       bool     // Face enhancement (Python only)
	upscaleAIDenoise           float64  // Denoise strength (0-1, model-specific)
	upscaleAITile              int      // Tile size for AI upscaling
	upscaleAIGPU               int      // GPU index (if supported)
	upscaleAIGPUAuto           bool     // Auto-select GPU
	upscaleAIThreadsLoad       int      // Threading for load stage
	upscaleAIThreadsProc       int      // Threading for processing stage
	upscaleAIThreadsSave       int      // Threading for save stage
	upscaleAITTA               bool     // Test-time augmentation
	upscaleAIOutputFormat      string   // png, jpg, webp
	upscaleApplyFilters        bool     // Apply filters from Filters module
	upscaleFilterChain         []string // Transferred filters from Filters module
	upscaleFrameRate           string   // Source, 24, 30, 60, or custom
	upscaleMotionInterpolation bool     // Use motion interpolation for frame rate changes
	upscaleBlurEnabled         bool     // Apply blur in upscale pipeline
	upscaleBlurSigma           float64  // Blur strength (sigma)
	upscaleEncoderPreset       string   // libx264 preset for upscale output
	upscaleVideoCodec          string   // H.264, H.265, VP9, AV1, Copy
	upscaleBitrateMode         string   // CRF, CBR, VBR
	upscaleBitratePreset       string   // preset label for bitrate modes
	upscaleManualBitrate       string   // manual bitrate value (e.g., 2500k)
	upscaleRIFEBackend         string   // "ncnn" or "" (not yet checked or not found)
	upscaleRIFEAvailable       bool     // runtime detection result
	upscaleRIFEEnabled         bool     // user opted in
	upscaleRIFEMultiplier      int      // 2 or 4
	upscaleRIFEModel           string   // "rife-v4.6", "rife-v4.13-lite", "rife-anime"
	upscaleRealCUGANAvailable  bool     // runtime detection result for Real-CUGAN

	// Output & colour accuracy settings
	upscaleHardwareAccel   string // "auto", "none", "nvenc", "vaapi", "qsv", "videotoolbox"
	upscaleOutputContainer string // "mp4", "mkv", "mov", "webm"
	upscaleManualCRF       int    // 0-51, used when bitrateMode is CRF
	upscalePixelFormat     string // "yuv420p", "yuv444p", "yuv420p10le"
	upscaleSrcColorSpace   string // "auto", "bt601", "bt709", "bt2020"
	upscaleColorDepth      string // "8bit", "16bit"
	upscaleSkinTone        string // "off", "subtle", "strong"

	// Snippet settings
	snippetLength       int  // Length of snippet in seconds (default: 20)
	snippetSourceFormat bool // true = source format, false = conversion format (default: true)
	snippetFromCurrent  bool // true = use current playback position, false = use video midpoint (default: false)

	// Interlacing detection state
	interlaceResult    *interlace.DetectionResult
	interlaceAnalyzing bool

	// User-defined encoding presets
	userPresets []userPreset

	// History sidebar state
	historyEntries []ui.HistoryEntry
	sidebarVisible bool
	historyTabIdx  int

	// Author module state
	authorFile                    *videoSource
	authorChapters                []authorChapter
	authorSceneThreshold          float64
	authorDetecting               bool
	authorClips                   []authorClip       // Multiple video clips for compilation
	authorClipsMu                 sync.Mutex         // Guards addAuthorFiles against concurrent calls
	authorOutputType              string             // "dvd" or "iso"
	authorRegion                  string             // "NTSC", "PAL", "AUTO"
	authorAspectRatio             string             // "4:3", "16:9", "AUTO"
	authorCreateMenu              bool               // Whether to create DVD menu
	authorTabs                    *container.AppTabs // Author module tabs for dynamic updates
	authorMenuTemplate            string             // "Minimal", "Simple", "Dark", "Poster"
	authorMenuBackgroundImage     string             // Path to a user-selected background image
	authorMenuMotionBackground    string             // Path to a motion background video (MPG)
	authorMenuTheme               string             // "VideoTools", "Minimal", "Western", etc.
	authorMenuCustomBgColor       string             // Custom background color hex
	authorMenuCustomTextColor     string             // Custom text color hex
	authorMenuCustomAccentColor   string             // Custom accent color hex
	authorMenuTitleLogoEnabled    bool               // Enable title logo (main logo above menu)
	authorMenuTitleLogoPath       string             // Path to title logo image
	authorMenuTitleLogoPosition   string             // Position for title logo
	authorMenuTitleLogoScale      float64            // Scale for title logo
	authorMenuTitleLogoMargin     int                // Margin for title logo
	authorMenuStudioLogoEnabled   bool               // Enable studio logo (corner logo)
	authorMenuStudioLogoPath      string             // Path to studio logo image
	authorMenuStudioLogoPosition  string             // "Top Left", "Top Right", "Bottom Left", "Bottom Right"
	authorMenuStudioLogoScale     float64            // Scale for studio logo
	authorMenuStudioLogoMargin    int                // Margin for studio logo
	authorMenuStructure           string             // Feature only, Chapters, Extras
	authorMenuExtrasEnabled       bool               // Show extras menu
	authorMenuChapterThumbnailSrc string             // Auto, First Frame, Midpoint, Custom
	authorTitle                   string             // Disc title
	authorDiscTitleEntry          *widget.Entry      // Settings tab title entry (for cross-tab sync)
	authorVideosTitleEntry        *widget.Entry      // Videos tab title entry (for cross-tab sync)
	authorSubtitles               []string           // Subtitle file paths
	authorAudioTracks             []string           // Additional audio tracks
	authorSummaryLabel            *widget.Label
	authorDiscFillBar             *widget.ProgressBar
	authorDiscFillLabel           *widget.Label
	authorTreatAsChapters         bool   // Treat multiple clips as chapters
	authorChapterSource           string // embedded, scenes, clips, manual
	authorChaptersRefresh         func() // Refresh hook for chapter list UI
	authorClipsRefresh            func() // Refresh hook for video clips list UI
	authorDiscSize                string // "DVD5" or "DVD9"
	authorLogText                 string
	authorLogLines                []string // Circular buffer for last N lines
	authorLogFilePath             string   // Path to log file for full viewing
	authorLogEntry                *widget.Label
	authorLogScroll               *ui.FastVScroll
	authorProgress                float64
	authorProgressBar             *widget.ProgressBar
	authorStatusLabel             *widget.Label
	authorCancelBtn               *ui.PillButton
	authorVideoTSPath             string

	// Burn module state
	burnLogText     string
	burnLogEntry    *widget.Label
	burnLogScroll   *container.Scroll
	burnLogFilePath string

	// Rip module state
	ripSourcePath  string
	ripOutputPath  string
	ripFormat      string
	ripLogText     string
	ripLogEntry    *widget.Label
	ripLogScroll   *container.Scroll
	ripProgress    float64
	ripProgressBar *widget.ProgressBar
	ripStatusLabel *widget.Label

	queueAutoRefreshStop    chan struct{}
	queueAutoRefreshRunning bool
	queueView               queuemodule.ViewAPI
	queueElapsedStop        chan struct{}
	queueElapsedRunning     bool

	// Main menu refresh throttling
	mainMenuLastRefresh time.Time

	// Update check cache — populated by the first settings open or the
	// background auto-checker; re-used on subsequent settings visits.
	updateLastChecked time.Time
	updateCachedTag   string // latest release tag ("" = up to date or not yet checked)
	updateCachedPatch bool   // true when same tag but newer build commit available

	// Subtitles module state
	subtitleVideoPath   string
	subtitleFilePath    string
	subtitleCues        []subtitleCue
	subtitleModelPath   string
	subtitleBackendPath string
	subtitleStatus      string
	subtitleStatusLabel *widget.Label
	subtitleOutputMode  string
	subtitleBurnOutput  string
	subtitleBurnEnabled bool
	subtitleCuesRefresh func()
	subtitleTimeOffset  float64
	subtitleRipStreams  []subtitleStreamInfo
	subtitleRipIndex    int
	subtitleRipMode     string
	subtitleRipOutput   string
	subtitleOCRLanguage string
	subtitleOCROutput   string

	// Audio module state
	audioFile                 *videoSource
	audioTracks               []audioTrackInfo
	audioSelectedTracks       map[int]bool
	audioOutputFormat         string
	audioQuality              string
	audioBitrate              string
	audioNormalize            bool
	audioNormTargetLUFS       float64
	audioNormTruePeak         float64
	audioOutputDir            string
	audioBatchMode            bool
	audioBatchFiles           []*videoSource
	audioFileInfoLabel        *widget.Label
	audioTrackListContainer   *fyne.Container
	audioBitrateEntry         *widget.Entry
	audioNormOptionsContainer *fyne.Container
	audioStatusLabel          *widget.Label
	audioProgressBar          *widget.ProgressBar
	audioPreviewLabel         *widget.Label // Output filename preview
	audioBatchListContainer   *fyne.Container
	audioLeftPanel            *fyne.Container
	audioSingleContent        *fyne.Container
	audioBatchContent         *fyne.Container

	// Application Preferences
	defaultOutputDir     string
	defaultVideoCodec    string // "libx264", "libx265", etc.
	defaultAudioCodec    string // "aac", "libmp3lame", etc.
	hardwareAcceleration string // "auto", "none", "nvenc", "qsv", "vaapi"
	uiTheme              string // "Dark", "Light", "System"
	autoPreview          bool   // Enable auto-preview functionality

	// Module pipeline state ("" = off, "step1" = waiting for step1 job, "step2" = waiting for step2 job)
	pipelineStep        string
	pipelineStep1ID     string // Job ID of the step1 job once queued
	pipelineStep1OutFile string // Expected output file of step1 (for reference)
}

type mergeClip struct {
	Path     string
	Chapter  string
	Duration float64
}

type authorChapter struct {
	Timestamp float64 // Timestamp in seconds
	Title     string  // Chapter title/name
	Auto      bool    // True if auto-detected, false if manual
}

type authorClip struct {
	Path         string          // Video file path
	DisplayName  string          // Display name in UI
	Duration     float64         // Video duration
	Chapters     []authorChapter // Chapters for this clip
	ChapterTitle string          // Optional chapter title when treating clips as chapters
	IsExtra      bool            // Mark this clip as an extra

	// Multitrack support
	AudioTracks    []authorAudioTrack    // List of audio tracks to include
	SubtitleTracks []authorSubtitleTrack // List of subtitle tracks to include
}

type authorAudioTrack struct {
	Index        int    // FFmpeg stream index
	Language     string // ISO 639-1 code (e.g., "en", "fr")
	Label        string // UI label
	Codec        string // Detected codec
	Channels     int    // Channel count
	ExternalPath string // Optional path to external file (e.g., for Archivist mode)
}

type authorSubtitleTrack struct {
	Index        int    // FFmpeg stream index
	Language     string // ISO 639-1 code
	Label        string // UI label
	Forced       bool   // True if this is a forced track
	ExternalPath string // Optional path to external file
}

func (s *appState) persistConvertConfig() {
	if err := savePersistedConvertConfig(s.convert); err != nil {
		logging.Error(logging.CatConvert, "failed to persist convert config: err=%v", err)
	}
}

// addToHistory adds a completed job to the history
func (s *appState) addToHistory(job *queue.Job) {
	if job == nil {
		return
	}

	// Only add completed, failed, or cancelled jobs
	if job.Status != queue.JobStatusCompleted &&
		job.Status != queue.JobStatusFailed &&
		job.Status != queue.JobStatusCancelled {
		return
	}

	// Build FFmpeg command from job config
	cmdStr := buildFFmpegCommandFromJob(job)

	entry := ui.HistoryEntry{
		ID:          job.ID,
		Type:        job.Type,
		Status:      job.Status,
		Title:       job.Title,
		InputFile:   job.InputFile,
		OutputFile:  job.OutputFile,
		LogPath:     job.LogPath,
		Config:      job.Config,
		CreatedAt:   job.CreatedAt,
		StartedAt:   job.StartedAt,
		CompletedAt: job.CompletedAt,
		Error:       job.Error,
		FFmpegCmd:   cmdStr,
	}

	// Check for duplicates
	for _, existing := range s.historyEntries {
		if existing.ID == entry.ID {
			return // Already in history
		}
	}

	// Prepend to history (newest first)
	s.historyEntries = append([]ui.HistoryEntry{entry}, s.historyEntries...)

	// Save to disk
	cfg := historyConfig{Entries: s.historyEntries}
	if err := saveHistoryConfig(cfg); err != nil {
		logging.Debug(logging.CatSystem, "failed to save history: %v", err)
	}
}

// showHistoryDetails displays detailed information about a history entry
func (s *appState) showHistoryDetails(entry ui.HistoryEntry) {
	t := i18n.T()
	// Format config
	var configLines []string
	for key, value := range entry.Config {
		configLines = append(configLines, fmt.Sprintf("%s: %v", key, value))
	}
	sort.Strings(configLines)

	// Format timestamps
	createdStr := entry.CreatedAt.Format("2006-01-02 15:04:05")
	startedStr := "N/A"
	if entry.StartedAt != nil {
		startedStr = entry.StartedAt.Format("2006-01-02 15:04:05")
	}
	completedStr := "N/A"
	if entry.CompletedAt != nil {
		completedStr = entry.CompletedAt.Format("2006-01-02 15:04:05")
	}

	details := fmt.Sprintf(`Type: %s
Status: %s
Input: %s
Output: %s

Created:   %s
Started:   %s
Completed: %s

Config:
%s`, entry.Type, entry.Status, entry.InputFile, entry.OutputFile,
		createdStr, startedStr, completedStr, strings.Join(configLines, "\n"))

	if entry.Error != "" {
		details += fmt.Sprintf("\n\nError:\n%s", entry.Error)
	}

	detailsLabel := widget.NewLabel(details)
	detailsLabel.Wrapping = fyne.TextWrapWord

	// Buttons
	var buttons []fyne.CanvasObject

	if entry.OutputFile != "" {
		if _, err := os.Stat(entry.OutputFile); err == nil {
			buttons = append(buttons, ui.MakePillButton("Show in Folder", ui.BorderDim, func() {
				dir := filepath.Dir(entry.OutputFile)
				if err := openFolder(dir); err != nil {
					dialog.ShowError(err, s.window)
				}
			}))
		}
	}

	if entry.LogPath != "" {
		if _, err := os.Stat(entry.LogPath); err == nil {
			buttons = append(buttons, ui.MakePillButton("View Log", ui.BorderDim, func() {
				s.openLogViewer(entry.Title, entry.LogPath, false)
			}))
		}
	}

	closeBtn := ui.MakePillButton("Close", ui.BorderDim, nil)
	buttons = append(buttons, layout.NewSpacer(), closeBtn)

	// Job details in scrollable area
	detailsScroll := container.NewVScroll(detailsLabel)
	// detailsScroll.SetMinSize(fyne.NewSize(650, 250)) // Removed for flexible sizing

	// FFmpeg Command section at bottom
	var ffmpegSection fyne.CanvasObject
	if entry.FFmpegCmd != "" {
		cmdWidget := ui.NewFFmpegCommandWidget(entry.FFmpegCmd, s.window)
		cmdLabel := widget.NewLabel(t.ConvertFFmpegCommand)
		cmdLabel.TextStyle = fyne.TextStyle{Bold: true}
		ffmpegSection = container.NewVBox(
			widget.NewSeparator(),
			cmdLabel,
			cmdWidget,
		)
	}

	// Layout: details at top (scrollable), FFmpeg at bottom (fixed)
	bottomItems := []fyne.CanvasObject{}
	if ffmpegSection != nil {
		bottomItems = append(bottomItems, ffmpegSection)
	}
	bottomItems = append(bottomItems, container.NewHBox(buttons...))
	content := container.NewBorder(
		detailsScroll, // Top: job details (scrollable, takes priority)
		container.NewVBox( // Bottom: FFmpeg command (fixed)
			bottomItems...,
		),
		nil, nil,
		nil, // No center content - top and bottom fill the space
	)

	d := dialog.NewCustom("Job Details", "Close", content, s.window)
	d.Resize(fyne.NewSize(750, 650))
	closeBtn.OnTapped = func() { d.Hide() }
	d.Show()
}

func (s *appState) deleteHistoryEntry(entry ui.HistoryEntry) {
	if entry.Status == queue.JobStatusRunning || entry.Status == queue.JobStatusPending {
		return
	}

	if s.jobQueue != nil {
		_ = s.jobQueue.Remove(entry.ID)
	}

	if entry.LogPath != "" {
		_ = os.Remove(entry.LogPath)
	}

	// Remove entry from history
	var updated []ui.HistoryEntry
	for _, e := range s.historyEntries {
		if e.ID != entry.ID {
			updated = append(updated, e)
		}
	}
	s.historyEntries = updated

	// Save updated history
	cfg := historyConfig{Entries: s.historyEntries}
	if err := saveHistoryConfig(cfg); err != nil {
		logging.Debug(logging.CatUI, "failed to save history after delete: %v", err)
	}

	// Refresh main menu to update sidebar
	s.refreshMainMenuThrottled()
}

func (s *appState) clearHistoryEntries(tabIndex int) {
	if tabIndex == 0 {
		return
	}
	keep := s.historyEntries[:0]
	for _, entry := range s.historyEntries {
		isCompleted := entry.Status == queue.JobStatusCompleted
		isFailed := entry.Status != queue.JobStatusCompleted
		shouldClear := (tabIndex == 1 && isCompleted) || (tabIndex == 2 && isFailed)
		if shouldClear {
			if entry.LogPath != "" {
				_ = os.Remove(entry.LogPath)
			}
			continue
		}
		keep = append(keep, entry)
	}
	s.historyEntries = keep
	cfg := historyConfig{Entries: s.historyEntries}
	if err := saveHistoryConfig(cfg); err != nil {
		logging.Debug(logging.CatUI, "failed to save history after clear: %v", err)
	}
	s.refreshMainMenuThrottled()
}

func (s *appState) stopPreview() {
	if s.anim != nil {
		s.anim.Stop()
		s.anim = nil
	}
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func toFloat(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		if f, err := t.Float64(); err == nil {
			return f
		}
	}
	return 0
}

func toInt(v interface{}) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case int32:
		return int(t)
	case float64:
		return int(t)
	case float32:
		return int(t)
	}
	return 0
}

func (s *appState) updateStatsBar() {
	if s.statsBar == nil || s.jobQueue == nil {
		return
	}

	pending, running, completed, failed, cancelled := s.jobQueue.Stats()

	// Find the currently running job to get its progress and stats
	var progress, fps, speed float64
	var eta, elapsed, remaining, jobTitle string
	if running > 0 {
		jobs := s.jobQueue.List()
		for _, job := range jobs {
			if job.Status == queue.JobStatusRunning {
				progress = job.Progress
				jobTitle = job.Title

				// Extract stats from job config if available
				if job.Config != nil {
					if f, ok := job.Config["fps"].(float64); ok {
						fps = f
					}
					if sp, ok := job.Config["speed"].(float64); ok {
						speed = sp
					}
					if etaDuration, ok := job.Config["eta"].(time.Duration); ok && etaDuration > 0 {
						eta = etaDuration.Round(time.Second).String()
					}
				}

				// Calculate elapsed and remaining time
				if job.StartedAt != nil {
					elapsed = fmt.Sprintf("Elapsed: %s", time.Since(*job.StartedAt).Round(time.Second))
					if progress > 0 && progress < 100 {
						elapsedSec := time.Since(*job.StartedAt).Seconds()
						remainingSec := elapsedSec*(100/progress) - elapsedSec
						remaining = fmt.Sprintf("Remaining: %s", time.Duration(remainingSec*float64(time.Second)).Round(time.Second))
					}
				}
				break
			}
		}
	} else if s.convertBusy {
		// Reflect direct conversion as an active job in the stats bar
		running = 1
		in := filepath.Base(s.convertActiveIn)
		if in == "" && s.source != nil {
			in = filepath.Base(s.source.Path)
		}
		jobTitle = fmt.Sprintf("Direct convert: %s", in)
		progress = s.convertProgress
		fps = s.convertFPS
		speed = s.convertSpeed
		if s.convertETA > 0 {
			eta = s.convertETA.Round(time.Second).String()
		}
	}

	s.statsBar.UpdateStatsWithDetails(running, pending, completed, failed, cancelled, progress, fps, speed, eta, elapsed, remaining, jobTitle)
}

func (s *appState) queueProgressCounts() (completed, total int) {
	if s.jobQueue == nil {
		return 0, 0
	}
	pending, running, completedCount, failed, cancelled := s.jobQueue.Stats()
	// Total includes all jobs in memory, including cancelled/failed/pending
	total = len(s.jobQueue.List())
	// Include direct conversion as an in-flight item in totals
	if s.convertBusy {
		total++
	}
	completed = completedCount
	_ = pending
	_ = running
	_ = failed
	_ = cancelled
	return
}

func (s *appState) updateQueueButtonLabel() {
	if s.queueBtn == nil {
		return
	}
	completed, total := s.queueProgressCounts()
	// Include active direct conversion in totals
	if s.convertBusy {
		total++
	}
	label := "View Queue"
	if total > 0 {
		label = fmt.Sprintf("View Queue %d/%d", completed, total)
	}
	s.queueBtn.SetText(label)
}

type playerSurface struct {
	obj           fyne.CanvasObject
	width, height int
}

func (s *appState) setPlayerSurface(obj fyne.CanvasObject, w, h int) {
	s.playerSurf = &playerSurface{obj: obj, width: w, height: h}
	s.syncPlayerWindow()
}

func (s *appState) currentPlayerPos() float64 {
	if s.playerPaused {
		return s.playerPos
	}
	return s.playerPos + time.Since(s.playerLast).Seconds()
}

func (s *appState) stopProgressLoop() {
	if s.progressQuit != nil {
		close(s.progressQuit)
		s.progressQuit = nil
	}
}

func (s *appState) startProgressLoop(maxDur float64, slider *widget.Slider, update func(float64)) {
	s.stopProgressLoop()
	stop := make(chan struct{})
	s.progressQuit = stop
	ticker := time.NewTicker(200 * time.Millisecond)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				pos := s.currentPlayerPos()
				if pos < 0 {
					pos = 0
				}
				if pos > maxDur {
					pos = maxDur
				}
				if update != nil {
					update(pos)
				}
				if slider != nil {
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						slider.SetValue(pos)
					}, false)
				}
			}
		}
	}()
}

func (s *appState) syncPlayerWindow() {
	if s.player == nil || s.playerSurf == nil || s.playerSurf.obj == nil {
		return
	}
	driver := fyne.CurrentApp().Driver()
	pos := driver.AbsolutePositionForObject(s.playerSurf.obj)
	width := s.playerSurf.width
	height := s.playerSurf.height
	if width <= 0 || height <= 0 {
		return
	}
	s.player.SetWindow(int(pos.X), int(pos.Y), width, height)
	logging.Debug(logging.CatUI, "player window target pos=(%d,%d) size=%dx%d", int(pos.X), int(pos.Y), width, height)
}

func (s *appState) startPreview(frames []string, img *canvas.Image, slider *ui.Slider) {
	if len(frames) == 0 {
		return
	}
	anim := &previewAnimator{frames: frames, img: img, slider: slider, stop: make(chan struct{}), playing: true, state: s}
	s.anim = anim
	anim.Start()
}

func (s *appState) hasSource() bool {
	return s.source != nil
}

func (s *appState) applyInverseDefaults(src *videoSource) {
	if src == nil {
		return
	}
	if src.IsProgressive() {
		s.convert.InverseTelecine = false
		s.convert.InverseAutoNotes = "Progressive source detected; inverse telecine disabled."
	} else {
		s.convert.InverseTelecine = true
		s.convert.InverseAutoNotes = "Interlaced source detected; smoothing enabled."
	}
}

// pushNavigationHistory adds current module to navigation history
func (s *appState) pushNavigationHistory(module string) {
	// Skip if suppressed (during back/forward navigation)
	if s.navigationHistorySuppress {
		return
	}

	// Don't add if it's the same as current position
	if len(s.navigationHistory) > 0 && s.navigationHistoryPosition < len(s.navigationHistory) {
		if s.navigationHistory[s.navigationHistoryPosition] == module {
			return
		}
	}

	// Truncate forward history when navigating to a new module
	if s.navigationHistoryPosition < len(s.navigationHistory)-1 {
		s.navigationHistory = s.navigationHistory[:s.navigationHistoryPosition+1]
	}

	// Add new module to history
	s.navigationHistory = append(s.navigationHistory, module)
	s.navigationHistoryPosition = len(s.navigationHistory) - 1

	// Limit history to 50 entries
	if len(s.navigationHistory) > 50 {
		s.navigationHistory = s.navigationHistory[1:]
		s.navigationHistoryPosition--
	}
}

// navigateBack goes back in navigation history (mouse back button)
func (s *appState) navigateBack() {
	// Main menu is the root — there is nowhere further back to go.
	if s.active == "" || s.active == "mainmenu" {
		return
	}
	if s.navigationHistoryPosition > 0 {
		s.navigationHistoryPosition--
		module := s.navigationHistory[s.navigationHistoryPosition]
		s.navigationHistorySuppress = true
		s.showModule(module)
		s.navigationHistorySuppress = false
	}
}

// navigateForward goes forward in navigation history (mouse forward button)
func (s *appState) navigateForward() {
	if s.navigationHistoryPosition < len(s.navigationHistory)-1 {
		s.navigationHistoryPosition++
		module := s.navigationHistory[s.navigationHistoryPosition]
		s.navigationHistorySuppress = true
		s.showModule(module)
		s.navigationHistorySuppress = false
	}
}

// mouseButtonHandler wraps content and handles mouse back/forward buttons
type mouseButtonHandler struct {
	widget.BaseWidget
	content fyne.CanvasObject
	state   *appState
}

func newMouseButtonHandler(content fyne.CanvasObject, state *appState) *mouseButtonHandler {
	h := &mouseButtonHandler{
		content: content,
		state:   state,
	}
	h.ExtendBaseWidget(h)
	return h
}

func (m *mouseButtonHandler) CreateRenderer() fyne.WidgetRenderer {
	// Create a transparent background that captures mouse events
	bg := canvas.NewRectangle(color.Transparent)
	bg.SetMinSize(fyne.NewSize(0, 0)) // Allow to expand

	return &mouseButtonRenderer{
		handler: m,
		content: container.NewMax(bg, m.content),
	}
}

func (m *mouseButtonHandler) MouseDown(me *desktop.MouseEvent) {
	switch me.Button {
	case desktop.MouseButton4:
		m.state.navigateBack()
	case desktop.MouseButton5:
		active := m.state.active
		if active != "mainmenu" && active != "queue" {
			m.state.showQueue()
		} else {
			m.state.navigateForward()
		}
	}
}

func (m *mouseButtonHandler) MouseUp(*desktop.MouseEvent) {}

type mouseButtonRenderer struct {
	handler *mouseButtonHandler
	content fyne.CanvasObject
}

func (r *mouseButtonRenderer) Layout(size fyne.Size) {
	if r.content != nil {
		r.content.Resize(size)
		r.content.Move(fyne.NewPos(0, 0))
	}
}

func (r *mouseButtonRenderer) MinSize() fyne.Size {
	if r.content != nil {
		return r.content.MinSize()
	}
	return fyne.NewSize(0, 0)
}

func (r *mouseButtonRenderer) Refresh() {
	if r.content != nil {
		r.content.Refresh()
	}
}

func (r *mouseButtonRenderer) Objects() []fyne.CanvasObject {
	if r.content != nil {
		return []fyne.CanvasObject{r.content}
	}
	return []fyne.CanvasObject{}
}

func (r *mouseButtonRenderer) Destroy() {}
func (r *mouseButtonRenderer) BackgroundColor() color.Color {
	return color.Transparent
}

// registerPrimaryActionShortcut registers Ctrl+Enter to trigger an action
func registerPrimaryActionShortcut(window fyne.Window, action func()) {
	if c := window.Canvas(); c != nil {
		trigger := func() { action() }
		c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyReturn, Modifier: fyne.KeyModifierControl}, func(fyne.Shortcut) { trigger() })
		c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyEnter, Modifier: fyne.KeyModifierControl}, func(fyne.Shortcut) { trigger() })
	}
}

func (s *appState) setContent(body fyne.CanvasObject) {
	defer func() {
		if r := recover(); r != nil {
			logging.Crash(logging.CatUI, "panic in setContent: %v", r)
		}
	}()
	// Capture size before the async update so the window does not resize when
	// the new module's content has different minimum sizes (issue #4).
	var savedSize fyne.Size
	if c := s.window.Canvas(); c != nil {
		savedSize = c.Size()
	}

	update := func() {
		bg := canvas.NewRectangle(backgroundColor)
		if body == nil {
			s.window.SetContent(bg)
		} else {
			// Wrap content with mouse button handler for back/forward buttons
			wrapped := newMouseButtonHandler(container.NewMax(bg, body), s)
			s.window.SetContent(wrapped)
		}
		// Restore window size to prevent layout-driven resize on module switch.
		if savedSize.Width > 0 && savedSize.Height > 0 {
			s.window.Resize(savedSize)
		}
	}

	// Use async Do() instead of DoAndWait() to avoid deadlock when called from main goroutine
	fyne.Do(update)
}

func (s *appState) maximizeWindow() {
	if s.window == nil {
		return
	}
	if s.windowAutoSized {
		return
	}
	canvas := s.window.Canvas()
	if canvas != nil {
		size := canvas.Size()
		s.window.Resize(size)
		s.windowAutoSized = true
	}
}

func (s *appState) unmaximizeWindow() {
	if s.window == nil {
		return
	}
	s.window.Resize(fyne.NewSize(1024, 768))
	s.window.CenterOnScreen()
}

// showErrorWithCopy displays an error dialog with a "Copy Error" button
func (s *appState) showErrorWithCopy(title string, err error) {
	errMsg := err.Error()

	// Create error message label
	errorLabel := widget.NewLabel(errMsg)
	errorLabel.Wrapping = fyne.TextWrapWord

	// Create copy button
	copyBtn := ui.MakePillButton("Copy Error", ui.BorderDim, func() {
		s.window.Clipboard().SetContent(errMsg)
	})

	// Create dialog content
	content := container.NewBorder(
		errorLabel,
		copyBtn,
		nil,
		nil,
		nil,
	)

	// Show custom dialog
	d := dialog.NewCustom(title, "Close", content, s.window)
	d.Resize(fyne.NewSize(500, 200))
	d.Show()
}

// addConvertToQueue adds a conversion job to the queue
func (s *appState) addConvertToQueue(addToTop bool) error {
	if s.source == nil {
		return fmt.Errorf("no video loaded")
	}

	return s.addConvertToQueueForSource(s.source, addToTop)
}

func (s *appState) addConvertToQueueForSource(src *videoSource, addToTop bool) error {
	outputBase := s.resolveOutputBase(src, true)
	cfg := s.convert
	cfg.OutputBase = outputBase

	outDir := strings.TrimSpace(cfg.OutputDir)
	if outDir == "" {
		outDir = filepath.Dir(src.Path)
	}
	outName := cfg.OutputFile()
	if outName == "" {
		outName = "converted" + cfg.SelectedFormat.Ext
	}
	outPath := filepath.Join(outDir, outName)
	if outPath == src.Path {
		outPath = filepath.Join(outDir, "converted-"+outName)
	}

	// Align codec choice with the selected format when the preset implies a codec change.
	adjustedCodec := s.convert.VideoCodec
	if preset := s.convert.SelectedFormat.VideoCodec; preset != "" {
		if friendly := friendlyCodecFromPreset(preset); friendly != "" {
			if adjustedCodec == "" ||
				(strings.EqualFold(adjustedCodec, "H.264") && friendly == "H.265") ||
				(strings.EqualFold(adjustedCodec, "H.265") && friendly == "H.264") {
				adjustedCodec = friendly
			}
		}
	}

	// Create job config map
	config := map[string]interface{}{
		"inputPath":          src.Path,
		"outputPath":         outPath,
		"outputDir":          outDir,
		"outputBase":         cfg.OutputBase,
		"selectedFormat":     cfg.SelectedFormat,
		"quality":            cfg.Quality,
		"mode":               cfg.Mode,
		"preserveChapters":   cfg.PreserveChapters,
		"videoCodec":         adjustedCodec,
		"encoderPreset":      cfg.EncoderPreset,
		"crf":                cfg.CRF,
		"bitrateMode":        cfg.BitrateMode,
		"bitratePreset":      cfg.BitratePreset,
		"videoBitrate":       cfg.VideoBitrate,
		"targetFileSize":     cfg.TargetFileSize,
		"targetResolution":   cfg.TargetResolution,
		"frameRate":          cfg.FrameRate,
		"pixelFormat":        cfg.PixelFormat,
		"hardwareAccel":      cfg.HardwareAccel,
		"twoPass":            cfg.TwoPass,
		"encoderTune":        cfg.EncoderTune,
		"h264Profile":        cfg.H264Profile,
		"h264Level":          cfg.H264Level,
		"deinterlace":        cfg.Deinterlace,
		"deinterlaceMethod":  cfg.DeinterlaceMethod,
		"autoCrop":           cfg.AutoCrop,
		"cropWidth":          cfg.CropWidth,
		"cropHeight":         cfg.CropHeight,
		"cropX":              cfg.CropX,
		"cropY":              cfg.CropY,
		"flipHorizontal":     cfg.FlipHorizontal,
		"flipVertical":       cfg.FlipVertical,
		"rotation":           cfg.Rotation,
		"audioCodec":         cfg.AudioCodec,
		"audioBitrate":       cfg.AudioBitrate,
		"audioChannels":      cfg.AudioChannels,
		"audioSampleRate":    cfg.AudioSampleRate,
		"normalizeAudio":     cfg.NormalizeAudio,
		"inverseTelecine":    cfg.InverseTelecine,
		"coverArtPath":       cfg.CoverArtPath,
		"aspectHandling":     cfg.AspectHandling,
		"outputAspect":       cfg.OutputAspect,
		"forceAspect":        cfg.ForceAspect,
		"sourceWidth":        src.Width,
		"sourceHeight":       src.Height,
		"sampleAspectRatio":  src.SampleAspectRatio,
		"displayAspectRatio": src.DisplayAspectRatio,
		"sourceDuration":     src.Duration,
		"sourceBitrate":      src.Bitrate,
		"sourceFrameRate":    src.FrameRate,
		"fieldOrder":         src.FieldOrder,
		"autoCompare":        s.autoCompare, // Include auto-compare flag
	}
	if !cfg.AutoCrop {
		config["cropWidth"] = ""
		config["cropHeight"] = ""
		config["cropX"] = ""
		config["cropY"] = ""
	}

	job := &queue.Job{
		Type:        queue.JobTypeConvert,
		Title:       fmt.Sprintf("Convert %s", filepath.Base(src.Path)),
		Description: fmt.Sprintf("Output: %s", utils.ShortenMiddle(filepath.Base(outPath), 40)),
		InputFile:   src.Path,
		OutputFile:  outPath,
		Config:      config,
	}

	// Generate thumbnail in background
	s.generateJobThumbnail(job)

	// Add to top (after running job) if requested, queue is running, and no pipeline active
	if addToTop && s.jobQueue.IsRunning() && s.pipelineStep == "" {
		s.jobQueue.AddNext(job)
		logging.Debug(logging.CatQueue, "added convert job to top of queue: %s", job.ID)
	} else {
		s.pipelineAdd(job)
	}

	return nil
}

func (s *appState) addAllConvertToQueue() (int, error) {
	if len(s.loadedVideos) == 0 {
		return 0, fmt.Errorf("no videos loaded")
	}

	usedOutputs := make(map[string]struct{})
	count := 0
	for _, src := range s.loadedVideos {
		if err := s.addConvertToQueueForSourceWithOutputs(src, usedOutputs); err != nil {
			return count, fmt.Errorf("failed to add %s: %w", filepath.Base(src.Path), err)
		}
		count++
	}

	return count, nil
}

func (s *appState) addConvertToQueueForSourceWithOutputs(src *videoSource, used map[string]struct{}) error {
	outputBase := s.resolveOutputBase(src, false)
	cfg := s.convert
	cfg.OutputBase = outputBase

	outDir := strings.TrimSpace(cfg.OutputDir)
	if outDir == "" {
		if strings.TrimSpace(s.defaultOutputDir) != "" {
			outDir = s.defaultOutputDir
		} else {
			outDir = filepath.Dir(src.Path)
		}
	}
	outName := cfg.OutputFile()
	if outName == "" {
		outName = "converted" + cfg.SelectedFormat.Ext
	}
	outPath := filepath.Join(outDir, outName)
	if outPath == src.Path {
		outPath = filepath.Join(outDir, "converted-"+outName)
	}

	// Ensure unique output path within batch to avoid overwrites.
	ext := filepath.Ext(outPath)
	base := strings.TrimSuffix(outPath, ext)
	candidate := outPath
	for i := 2; ; i++ {
		if _, ok := used[candidate]; !ok {
			if _, err := os.Stat(candidate); os.IsNotExist(err) {
				break
			}
		}
		candidate = fmt.Sprintf("%s-%d%s", base, i, ext)
	}
	outPath = candidate
	used[outPath] = struct{}{}

	// Align codec choice with the selected format when the preset implies a codec change.
	adjustedCodec := s.convert.VideoCodec
	if preset := s.convert.SelectedFormat.VideoCodec; preset != "" {
		if friendly := friendlyCodecFromPreset(preset); friendly != "" {
			if adjustedCodec == "" ||
				(strings.EqualFold(adjustedCodec, "H.264") && friendly == "H.265") ||
				(strings.EqualFold(adjustedCodec, "H.265") && friendly == "H.264") {
				adjustedCodec = friendly
			}
		}
	}

	// Create job config map
	config := map[string]interface{}{
		"inputPath":          src.Path,
		"outputPath":         outPath,
		"outputBase":         cfg.OutputBase,
		"selectedFormat":     cfg.SelectedFormat,
		"quality":            cfg.Quality,
		"mode":               cfg.Mode,
		"preserveChapters":   cfg.PreserveChapters,
		"videoCodec":         adjustedCodec,
		"encoderPreset":      cfg.EncoderPreset,
		"crf":                cfg.CRF,
		"bitrateMode":        cfg.BitrateMode,
		"bitratePreset":      cfg.BitratePreset,
		"videoBitrate":       cfg.VideoBitrate,
		"targetFileSize":     cfg.TargetFileSize,
		"targetResolution":   cfg.TargetResolution,
		"frameRate":          cfg.FrameRate,
		"pixelFormat":        cfg.PixelFormat,
		"hardwareAccel":      cfg.HardwareAccel,
		"twoPass":            cfg.TwoPass,
		"encoderTune":        cfg.EncoderTune,
		"h264Profile":        cfg.H264Profile,
		"h264Level":          cfg.H264Level,
		"deinterlace":        cfg.Deinterlace,
		"deinterlaceMethod":  cfg.DeinterlaceMethod,
		"autoCrop":           cfg.AutoCrop,
		"cropWidth":          cfg.CropWidth,
		"cropHeight":         cfg.CropHeight,
		"cropX":              cfg.CropX,
		"cropY":              cfg.CropY,
		"flipHorizontal":     cfg.FlipHorizontal,
		"flipVertical":       cfg.FlipVertical,
		"rotation":           cfg.Rotation,
		"audioCodec":         cfg.AudioCodec,
		"audioBitrate":       cfg.AudioBitrate,
		"audioChannels":      cfg.AudioChannels,
		"audioSampleRate":    cfg.AudioSampleRate,
		"normalizeAudio":     cfg.NormalizeAudio,
		"inverseTelecine":    cfg.InverseTelecine,
		"coverArtPath":       cfg.CoverArtPath,
		"aspectHandling":     cfg.AspectHandling,
		"outputAspect":       cfg.OutputAspect,
		"forceAspect":        cfg.ForceAspect,
		"sourceWidth":        src.Width,
		"sourceHeight":       src.Height,
		"sampleAspectRatio":  src.SampleAspectRatio,
		"displayAspectRatio": src.DisplayAspectRatio,
		"sourceDuration":     src.Duration,
		"sourceBitrate":      src.Bitrate,
		"sourceFrameRate":    src.FrameRate,
		"fieldOrder":         src.FieldOrder,
		"autoCompare":        s.autoCompare,
	}

	job := &queue.Job{
		Type:        queue.JobTypeConvert,
		Title:       fmt.Sprintf("Convert %s", filepath.Base(src.Path)),
		Description: fmt.Sprintf("Output: %s", utils.ShortenMiddle(filepath.Base(outPath), 40)),
		InputFile:   src.Path,
		OutputFile:  outPath,
		Config:      config,
	}

	// Generate thumbnail in background
	s.generateJobThumbnail(job)

	s.pipelineAdd(job)

	return nil
}

func (s *appState) showBenchmark() {
	s.stopPreview()
	s.stopPlayer()
	s.active = "benchmark"

	// Detect hardware info upfront
	hwInfo := sysinfo.Detect()
	logging.Debug(logging.CatSystem, "detected hardware for benchmark: %s", hwInfo.Summary())

	// Check if we have recent benchmark results for this hardware
	cfg, err := loadBenchmarkConfig()
	if err == nil && len(cfg.History) > 0 {
		lastRun := cfg.History[0]

		// Check if hardware matches (same GPU)
		hardwareMatches := lastRun.HardwareInfo.GPU == hwInfo.GPU

		// If hardware matches, show last results instead of auto-running
		if hardwareMatches && len(lastRun.Results) > 0 {
			logging.Debug(logging.CatSystem, "found existing benchmark from %s, showing results", lastRun.Timestamp.Format("2006-01-02"))

			// Create recommendation from saved data
			rec := benchmark.Result{
				Encoder: lastRun.RecommendedEncoder,
				HWAccel: lastRun.RecommendedHWAccel,
				FPS:     lastRun.RecommendedFPS,
				Score:   lastRun.RecommendedFPS,
			}

			// Show results with "Run New Benchmark" option
			// "Run New Benchmark" sits in the action bar alongside a cache note
			runNewBtn := ui.MakePillButton("Run New Benchmark", ui.BorderDim, func() {
				s.runNewBenchmark()
			})

			cachedNote := ui.DarkTextLabel(fmt.Sprintf("Cached: %s", lastRun.Timestamp.Format("Jan 2, 2006 3:04 PM")))
			cachedNote.TextStyle = fyne.TextStyle{Italic: true}

			actionContent := container.NewHBox(cachedNote, layout.NewSpacer(), runNewBtn)

			resultsView := ui.BuildBenchmarkResultsView(
				lastRun.Results,
				rec,
				lastRun.HardwareInfo,
				func() {
					s.applyBenchmarkRecommendation(lastRun.RecommendedHWAccel)
					s.showSettingsView()
				},
				s.showSettingsView,
				utils.MustHex("#4CE870"),
				utils.MustHex("#1E1E1E"),
				s.statsBar,
				actionContent,
			)

			s.setContent(resultsView)
			return
		}
	}

	// No existing benchmark or hardware changed - run new benchmark
	s.runNewBenchmark()
}

func (s *appState) runNewBenchmark() {
	// Detect hardware info upfront
	hwInfo := sysinfo.Detect()
	logging.Debug(logging.CatSystem, "starting new benchmark for hardware: %s", hwInfo.Summary())

	// Create benchmark suite
	tmpDir := filepath.Join(utils.TempDir(), "videotools-benchmark")
	_ = os.MkdirAll(tmpDir, 0o755)

	suite := benchmark.NewSuite(utils.GetFFmpegPath(), tmpDir)

	benchComplete := atomic.Bool{}
	ctx, cancel := context.WithCancel(context.Background())

	// Build progress view with hardware info
	view := ui.BuildBenchmarkProgressView(
		hwInfo,
		func() {
			if benchComplete.Load() {
				s.showSettingsView()
				return
			}

			dialog.ShowConfirm("Cancel Benchmark?", "The benchmark is still running. Cancel it now?", func(ok bool) {
				if !ok {
					return
				}
				cancel()
				s.showSettingsView()
			}, s.window)
		},
		utils.MustHex("#4CE870"),
		utils.MustHex("#1E1E1E"),
		s.statsBar,
	)

	s.setContent(view.GetContainer())

	// Run benchmark in background
	go func() {
		// Generate test video
		view.UpdateProgress(0, 100, "Generating test video")
		testPath, err := suite.GenerateTestVideo(ctx, 30)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			logging.Debug(logging.CatSystem, "failed to generate test video: %v", err)
			fyne.CurrentApp().SendNotification(&fyne.Notification{
				Title:   "Benchmark Error",
				Content: fmt.Sprintf("Failed to generate test video: %v", err),
			})
			s.showMainMenu()
			return
		}
		logging.Debug(logging.CatSystem, "generated test video: %s", testPath)

		// Detect available encoders
		availableEncoders := s.detectHardwareEncoders()
		logging.Debug(logging.CatSystem, "detected %d available encoders", len(availableEncoders))

		// Set up progress callback
		suite.Progress = func(current, total int, label string) {
			logging.Debug(logging.CatSystem, "benchmark progress: %d/%d testing %s", current, total, label)
			view.UpdateProgress(current, total, label)
		}

		// Run benchmark suite
		err = suite.RunFullSuite(ctx, availableEncoders)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			logging.Debug(logging.CatSystem, "benchmark failed: %v", err)
			fyne.CurrentApp().SendNotification(&fyne.Notification{
				Title:   "Benchmark Error",
				Content: fmt.Sprintf("Benchmark failed: %v", err),
			})
			s.showMainMenu()
			return
		}

		// Display results as they come in
		for _, result := range suite.Results {
			view.AddResult(result)
		}

		// Mark complete
		view.SetComplete()
		benchComplete.Store(true)

		// Get recommendation
		encoder, rec := suite.GetRecommendation()

		// Save benchmark run to history
		if err := s.saveBenchmarkRun(suite.Results, rec); err != nil {
			logging.Debug(logging.CatSystem, "failed to save benchmark run: %v", err)
		}

		if encoder != "" {
			logging.Debug(logging.CatSystem, "benchmark recommendation: %s - %.1f FPS", encoder, rec.FPS)

			// Show results dialog with option to apply
			go func() {
				// Detect hardware info for display
				hwInfo := sysinfo.Detect()
				allResults := suite.Results // Show all results, not just top 10
				resultsView := ui.BuildBenchmarkResultsView(
					allResults,
					rec,
					hwInfo,
					func() {
						s.applyBenchmarkRecommendation(rec.HWAccel)
						s.showSettingsView()
					},
					s.showSettingsView,
					utils.MustHex("#4CE870"),
					utils.MustHex("#1E1E1E"),
					s.statsBar,
					nil,
				)

				s.setContent(resultsView)
			}()
		}

		// Clean up test video
		os.Remove(testPath)
	}()
}

func (s *appState) detectHardwareEncoders() []string {
	// Software encoders are always available
	available := []string{"libx264", "libx265"}

	// Hardware encoders — checked via the cached ffmpeg -encoders output
	// so we only spawn FFmpeg once regardless of how many encoders we probe.
	hwEncoders := []string{
		"h264_nvenc", "hevc_nvenc", "av1_nvenc", // NVIDIA
		"h264_qsv", "hevc_qsv", // Intel QuickSync
		"h264_amf", "hevc_amf", "av1_amf", // AMD AMF
		"h264_videotoolbox", // Apple VideoToolbox
	}
	for _, enc := range hwEncoders {
		if hasFFmpegEncoder(enc) {
			available = append(available, enc)
			logging.Debug(logging.CatSystem, "detected available encoder: %s", enc)
		}
	}

	return available
}

var ffmpegEncoderOnce sync.Once
var ffmpegEncoderOutput string

func getFFmpegEncoders() string {
	ffmpegEncoderOnce.Do(func() {
		cmd := utils.CreateCommandRaw(utils.GetFFmpegPath(), "-hide_banner", "-encoders")
		output, err := cmd.CombinedOutput()
		if err != nil {
			logging.Debug(logging.CatFFMPEG, "failed to query ffmpeg encoders: %v", err)
			return
		}
		ffmpegEncoderOutput = string(output)
	})
	return ffmpegEncoderOutput
}

func hasFFmpegEncoder(name string) bool {
	encoders := getFFmpegEncoders()
	if encoders == "" {
		return false
	}
	return strings.Contains(encoders, " "+name+" ") || strings.Contains(encoders, " "+name+"\n")
}

// resolveH264HWEncoder returns the best available hardware H.264 encoder for "auto" mode.
func resolveH264HWEncoder() (string, bool) {
	for _, enc := range []string{"h264_nvenc", "h264_vaapi", "h264_qsv", "h264_videotoolbox", "h264_amf"} {
		if hasFFmpegEncoder(enc) {
			return enc, true
		}
	}
	return "", false
}

// resolveHEVCHWEncoder returns the best available hardware H.265/HEVC encoder for "auto" mode.
func resolveHEVCHWEncoder() (string, bool) {
	for _, enc := range []string{"hevc_nvenc", "hevc_vaapi", "hevc_qsv", "hevc_videotoolbox", "hevc_amf"} {
		if hasFFmpegEncoder(enc) {
			return enc, true
		}
	}
	return "", false
}

func resolveAV1Encoder(hardwareAccel string) (string, bool) {
	switch hardwareAccel {
	case "nvenc":
		// Must runtime-probe av1_nvenc separately from h264_nvenc — AV1 NVENC
		// requires Ada Lovelace (RTX 40xx+) while h264_nvenc works on Ampere (RTX 30xx).
		// hasFFmpegEncoder only checks compilation support, not GPU capability.
		if checkAV1NvencRuntime() {
			return "av1_nvenc", true
		}
	case "qsv":
		// av1_qsv requires Arc/Meteor Lake; older iGPUs pass the h264_qsv probe but fail for AV1.
		if checkAV1QsvRuntime() {
			return "av1_qsv", true
		}
	case "amf":
		// av1_amf requires RDNA 3 (RX 7000+); RDNA 2 passes h264_amf but fails for AV1.
		if checkAV1AmfRuntime() {
			return "av1_amf", true
		}
	case "vaapi":
		if hasFFmpegEncoder("av1_vaapi") {
			return "av1_vaapi", true
		}
	}
	if hasFFmpegEncoder("libsvtav1") {
		return "libsvtav1", true
	}
	if hasFFmpegEncoder("libaom-av1") {
		return "libaom-av1", true
	}
	return "libx264", false
}

func (s *appState) saveBenchmarkRun(results []benchmark.Result, rec benchmark.Result) error {
	hwAccel := rec.HWAccel
	if hwAccel == "" {
		hwAccel = "none"
	}

	// Detect hardware info
	hwInfo := sysinfo.Detect()
	logging.Debug(logging.CatSystem, "detected hardware: %s", hwInfo.Summary())

	// Load existing config
	cfg, err := loadBenchmarkConfig()
	if err != nil {
		// Create new config if loading fails
		cfg = benchmarkConfig{History: []benchmarkRun{}}
	}

	// Create new benchmark run
	run := benchmarkRun{
		Timestamp:          time.Now(),
		Results:            results,
		RecommendedEncoder: rec.Encoder,
		RecommendedHWAccel: hwAccel,
		RecommendedFPS:     rec.FPS,
		HardwareInfo:       hwInfo,
	}

	// Add to history (keep last 10 runs)
	cfg.History = append([]benchmarkRun{run}, cfg.History...)
	if len(cfg.History) > 10 {
		cfg.History = cfg.History[:10]
	}

	// Save config
	if err := saveBenchmarkConfig(cfg); err != nil {
		return err
	}

	logging.Debug(logging.CatSystem, "saved benchmark run: encoder=%s hwaccel=%s fps=%.1f results=%d", rec.Encoder, hwAccel, rec.FPS, len(results))
	return nil
}

func (s *appState) applyBenchmarkRecommendation(hwAccel string) {
	logging.Debug(logging.CatSystem, "applying benchmark hardware recommendation: %s", hwAccel)

	if hwAccel == "" {
		hwAccel = "none"
	}

	// Intentionally do not modify codec or preset; benchmark only drives hardware path.
	s.convert.HardwareAccel = hwAccel
	s.persistConvertConfig()

	logging.Info(logging.CatSystem, "benchmark applied hardware acceleration: %s (codec/preset unchanged)", hwAccel)
}

func (s *appState) showBenchmarkHistory() {
	s.stopPreview()
	s.stopPlayer()
	s.active = "benchmark-history"

	// Load benchmark history
	cfg, err := loadBenchmarkConfig()
	if err != nil || len(cfg.History) == 0 {
		// Show empty state
		view := ui.BuildBenchmarkHistoryView(
			[]ui.BenchmarkHistoryRun{},
			nil,
			s.showSettingsView,
			utils.MustHex("#4CE870"),
			utils.MustHex("#1E1E1E"),
			s.statsBar,
		)
		s.setContent(view)
		return
	}

	// Convert history to UI format
	var historyRuns []ui.BenchmarkHistoryRun
	for _, run := range cfg.History {
		historyRuns = append(historyRuns, ui.BenchmarkHistoryRun{
			Timestamp:          run.Timestamp.Format("2006-01-02 15:04:05"),
			ResultCount:        len(run.Results),
			RecommendedHWAccel: run.RecommendedHWAccel,
			RecommendedFPS:     run.RecommendedFPS,
		})
	}

	// Build history view
	view := ui.BuildBenchmarkHistoryView(
		historyRuns,
		func(index int) {
			// Show detailed results for this run
			if index < 0 || index >= len(cfg.History) {
				return
			}
			run := cfg.History[index]

			// Create a fake recommendation result for the results view
			rec := benchmark.Result{
				Encoder: run.RecommendedEncoder,
				HWAccel: run.RecommendedHWAccel,
				FPS:     run.RecommendedFPS,
				Score:   run.RecommendedFPS,
			}

			resultsView := ui.BuildBenchmarkResultsView(
				run.Results,
				rec,
				run.HardwareInfo,
				func() {
					s.applyBenchmarkRecommendation(run.RecommendedHWAccel)
					s.showBenchmarkHistory()
				},
				s.showBenchmarkHistory,
				utils.MustHex("#4CE870"),
				utils.MustHex("#1E1E1E"),
				s.statsBar,
				nil,
			)

			s.setContent(resultsView)
		},
		s.showSettingsView,
		utils.MustHex("#4CE870"),
		utils.MustHex("#1E1E1E"),
		s.statsBar,
	)

	s.setContent(view)
}

func (s *appState) showModule(id string) {
	logging.Info(logging.CatModule, "showModule: id=%s", id)

	if id != "queue" {
		s.stopQueueAutoRefresh()
		s.stopQueueElapsedTicker()
		if s.queueView != nil {
			s.queueView.StopAnimations()
		}
	}

	// Check if module has missing dependencies
	if !isModuleAvailable(id) && id != "settings" {
		s.showMissingDependenciesDialog(id)
		return
	}

	// Track navigation history (queue manages its own push inside showQueue)
	if id != "queue" {
		s.pushNavigationHistory(id)
	}

	logging.Info(logging.CatModule, "showModule: dispatching to module handler for %s", id)

	switch id {
	case "convert":
		s.showConvertView(nil)
	case "merge":
		s.showMergeView()
	case "trim":
		s.showTrimView()
	case "compare":
		s.showCompareView()
	case "inspect":
		s.showInspectView()
	case "thumbnail":
		s.showThumbnailView()
	case "player":
		s.showPlayerView()
	case "filters":
		s.showFiltersView()
	case "upscale":
		s.showUpscaleView()
	// case "enhancement":
	//	s.showEnhancementView() // TODO: Implement when enhancement module is complete
	case "audio":
		logging.Info(logging.CatModule, "showModule: entering audio module")
		s.showAudioView()
		logging.Info(logging.CatModule, "showModule: audio module returned")
	case "author":
		s.showAuthorView()
	case "rip":
		s.showRipView()
	case "burn":
		s.showBurnView()
	case "filemanager":
		s.showFileManagerView()
	case "subtitles":
		s.showSubtitlesView()
	case "settings":
		s.showSettingsView()
	case "queue":
		s.showQueue()
	case "mainmenu":
		s.showMainMenu()
	default:
		logging.Debug(logging.CatUI, "UI module %s not wired yet", id)
	}
}

func (s *appState) handleModuleDrop(moduleID string, items []fyne.URI) {
	defer logging.RecoverPanic()
	t := i18n.T()
	logging.Info(logging.CatModule, "handleModuleDrop called: moduleID=%s itemCount=%d", moduleID, len(items))
	if len(items) == 0 {
		logging.Debug(logging.CatModule, "handleModuleDrop: no items to process")
		return
	}
	if moduleID == "subtitles" {
		s.handleSubtitlesModuleDrop(items)
		return
	}

	// Collect all video files (and audio files for audio module)
	var videoPaths []string
	for _, uri := range items {
		logging.Debug(logging.CatModule, "handleModuleDrop: processing uri scheme=%s path=%s", uri.Scheme(), uri.Path())
		if uri.Scheme() != "file" {
			logging.Debug(logging.CatModule, "handleModuleDrop: skipping non-file URI")
			continue
		}
		path := uri.Path()

		// Check if it's a directory
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			logging.Debug(logging.CatModule, "processing directory: %s", path)
			// Detect DVD VIDEO_TS directories — load as single disc instead
			// of enumerating individual VOBs, which picks the menu VOB
			// (~0s, zero audio) as the first file.
			if isDVDDisc(path) {
				logging.Info(logging.CatModule, "detected DVD disc directory on Convert drop, loading via dvdvideo demuxer")
				dvdRoot := dvdDiscRoot(path)
				p := GetPrimaryPlayer()
				if p != nil {
					p.Close()
					go func() { _ = p.LoadDVD(dvdRoot, 0) }()
				}
				videoPaths = append(videoPaths, path)
				continue
			}
			videos := s.findVideoFiles(path)
			videoPaths = append(videoPaths, videos...)
		} else if s.isVideoFile(path) || (moduleID == "audio" && s.isAudioFile(path)) {
			videoPaths = append(videoPaths, path)
		}
	}

	logging.Debug(logging.CatModule, "found %d video files to process", len(videoPaths))

	if len(videoPaths) == 0 {
		if msg := dropMismatchMessage(items, moduleID); msg != "" {
			ui.ShowToast(s.window, msg, ui.ToastWarning)
		}
		return
	}

	// If convert module and multiple files, add all to queue
	if moduleID == "convert" && len(videoPaths) > 1 {
		go s.batchAddToQueue(videoPaths)
		return
	}

	// If convert module with single file, load it
	if moduleID == "convert" && len(videoPaths) == 1 {
		go s.loadVideo(videoPaths[0])
		return
	}

	// If upscale module and multiple files, add all to queue
	if moduleID == "upscale" && len(videoPaths) > 1 {
		go s.batchAddToUpscaleQueue(videoPaths)
		return
	}

	// If upscale module with single file, probe and load it
	if moduleID == "upscale" && len(videoPaths) == 1 {
		go func() {
			defer logging.RecoverPanic()
			src, err := probeVideo(videoPaths[0])
			if err != nil {
				logging.Debug(logging.CatModule, "failed to load video for upscale: %v", err)
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					dialog.ShowError(fmt.Errorf("failed to load video: %w", err), s.window)
				}, false)
				return
			}

			time.Sleep(350 * time.Millisecond)
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				s.upscaleFile = src
				s.showUpscaleView()
				logging.Debug(logging.CatModule, "loaded video into upscale module")
			}, false)
		}()
		return
	}

	// If compare module, load up to 2 videos into compare slots
	if moduleID == "compare" {
		go func() {
			defer logging.RecoverPanic()
			// Load first video
			src1, err := probeVideo(videoPaths[0])
			if err != nil {
				logging.Debug(logging.CatModule, "failed to load first video for compare: %v", err)
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					dialog.ShowError(fmt.Errorf("failed to load video: %w", err), s.window)
				}, false)
				return
			}

			// Load second video if available
			var src2 *videoSource
			if len(videoPaths) >= 2 {
				src2, err = probeVideo(videoPaths[1])
				if err != nil {
					logging.Debug(logging.CatModule, "failed to load second video for compare: %v", err)
					// Continue with just first video
				}
			}

			// Show dialog if more than 2 videos
			if len(videoPaths) > 2 {
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					dialog.ShowInformation(t.DialogCompare,
						fmt.Sprintf("You dropped %d videos. Only the first two will be loaded for comparison.", len(videoPaths)),
						s.window)
				}, false)
			}

			// Update state and show module (with small delay to allow flash animation to be seen)
			time.Sleep(350 * time.Millisecond)
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				// Smart slot assignment: if dropping 2 videos, fill both slots
				if len(videoPaths) >= 2 {
					s.compareFile1 = src1
					s.compareFile2 = src2
				} else {
					// Single video: fill the empty slot, or slot 1 if both empty
					if s.compareFile1 == nil {
						s.compareFile1 = src1
					} else if s.compareFile2 == nil {
						s.compareFile2 = src1
					} else {
						// Both slots full, overwrite slot 1
						s.compareFile1 = src1
					}
				}
				s.showModule(moduleID)
				logging.Debug(logging.CatModule, "loaded %d video(s) for compare module", len(videoPaths))
			}, false)
		}()
		return
	}

	// If inspect module, load video into inspect slot
	if moduleID == "inspect" {
		path := videoPaths[0]
		go func() {
			src, err := probeVideo(path)
			if err != nil {
				logging.Debug(logging.CatModule, "failed to load video for inspect: %v", err)
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					dialog.ShowError(fmt.Errorf("failed to load video: %w", err), s.window)
				}, false)
				return
			}

			// Update state and show module (with small delay to allow flash animation)
			time.Sleep(350 * time.Millisecond)
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				s.inspectFile = src
				s.inspectInterlaceResult = nil
				s.inspectInterlaceAnalyzing = true
				s.showModule(moduleID)
				logging.Debug(logging.CatModule, "loaded video for inspect module")
			}, false)

			// Load native player after view is shown.
			if err := GetPrimaryPlayer().Load(path); err != nil {
				logging.Error(logging.CatPlayer, "inspect player load failed: %v", err)
			}

			// Auto-run interlacing detection in background
			go func() {
				// Capture preview frames before running interlace analysis
				if len(src.PreviewFrames) == 0 {
					if frames, ferr := capturePreviewFrames(path, src.Duration); ferr == nil && len(frames) > 0 {
						src.PreviewFrames = frames
					}
				}

				detector := interlace.NewDetector(utils.GetFFmpegPath(), utils.GetFFprobePath())
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()

				result, err := detector.QuickAnalyze(ctx, path)

				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					s.inspectInterlaceAnalyzing = false
					if err != nil {
						logging.Debug(logging.CatSystem, "auto interlacing analysis failed: %v", err)
						s.inspectInterlaceResult = nil
					} else {
						s.inspectInterlaceResult = result
						logging.Debug(logging.CatSystem, "auto interlacing analysis complete: %s", result.Status)
					}
					s.showInspectView() // Refresh to show results
				}, false)
			}()
		}()
		return
	}

	if moduleID == "merge" {
		go func() {
			var clips []mergeClip
			for _, p := range videoPaths {
				src, err := probeVideo(p)
				if err != nil {
					logging.Error(logging.CatMerge, "merge clip probe failed: path=%s err=%v", p, err)
					continue
				}
				clips = append(clips, mergeClip{
					Path:     p,
					Chapter:  strings.TrimSuffix(filepath.Base(p), filepath.Ext(p)),
					Duration: src.Duration,
				})
			}
			if len(clips) == 0 {
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					dialog.ShowInformation(t.ModuleMerge, "No valid video files found.", s.window)
				}, false)
				return
			}
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				s.mergeClips = append(s.mergeClips, clips...)
				if len(s.mergeClips) >= 2 && strings.TrimSpace(s.mergeOutputDir) == "" {
					s.mergeOutputDir = filepath.Dir(s.mergeClips[0].Path)
				}
				if len(s.mergeClips) >= 2 && strings.TrimSpace(s.mergeOutputFilename) == "" {
					s.mergeOutputFilename = "merged.mkv"
				}
				s.showMergeView()
			}, false)
		}()
		return
	}

	// If thumbnail module, load video into thumbnail slot
	if moduleID == "thumbnail" {
		path := videoPaths[0]
		go func() {
			src, err := probeVideo(path)
			if err != nil {
				logging.Debug(logging.CatModule, "failed to load video for thumbnail: %v", err)
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					dialog.ShowError(fmt.Errorf("failed to load video: %w", err), s.window)
				}, false)
				return
			}

			// Update state and show module (with small delay to allow flash animation)
			time.Sleep(350 * time.Millisecond)
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				s.thumbnailFile = src
				s.showModule(moduleID)
				logging.Debug(logging.CatModule, "loaded video for thumbnail module")
			}, false)
		}()
		return
	}

	// If audio module, load audio/video file into audio slot
	if moduleID == "audio" {
		path := videoPaths[0]
		go func() {
			src, err := probeVideo(path)
			if err != nil {
				logging.Debug(logging.CatModule, "failed to load file for audio: %v", err)
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					dialog.ShowError(fmt.Errorf("failed to load file: %w", err), s.window)
				}, false)
				return
			}

			// Update state and show module (with small delay to allow flash animation)
			time.Sleep(350 * time.Millisecond)
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				s.audioFile = src
				s.showModule(moduleID)
				logging.Debug(logging.CatModule, "loaded file for audio module")
			}, false)
		}()
		return
	}

	// Single file or non-convert module: load first video and show module
	path := videoPaths[0]
	logging.Debug(logging.CatModule, "drop on module %s path=%s - starting load", moduleID, path)

	go func() {
		logging.Debug(logging.CatModule, "loading video in goroutine")
		s.loadVideo(path)
		// After loading, switch to the module (with small delay to allow flash animation)
		time.Sleep(350 * time.Millisecond)
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			logging.Debug(logging.CatModule, "showing module %s after load", moduleID)
			s.showModule(moduleID)
		}, false)
	}()
}

// isVideoFile checks if a file has a video extension
func (s *appState) isVideoFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	videoExts := []string{
		".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm",
		".m4v", ".mpg", ".mpeg", ".3gp", ".ogv",
		".ts", ".m2ts", ".vob", // MPEG-2 transport streams and DVD video
	}
	for _, videoExt := range videoExts {
		if ext == videoExt {
			return true
		}
	}
	return false
}

func (s *appState) isAudioFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	audioExts := []string{".mp3", ".flac", ".aac", ".opus", ".m4a", ".wav", ".ogg", ".wma", ".alac", ".ape"}
	for _, audioExt := range audioExts {
		if ext == audioExt {
			return true
		}
	}
	return false
}

func (s *appState) isSubtitleFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	subtitleExts := []string{".srt", ".vtt", ".ass", ".ssa", ".mks"}
	for _, subtitleExt := range subtitleExts {
		if ext == subtitleExt {
			return true
		}
	}
	return false
}

// dropFileType classifies a single file path into a broad category used to
// produce format-mismatch notifications when files are dropped onto the wrong
// module.
type dropFileType int

const (
	dropTypeUnknown   dropFileType = iota
	dropTypeVideo                  // regular video file
	dropTypeAudio                  // audio-only file
	dropTypeSubtitle               // subtitle / caption file
	dropTypeDiscImage              // ISO, IMG, BIN/CUE, NRG, MDS/MDF
	dropTypeDVDFile                // IFO, VOB, BUP — DVD structure files
	dropTypeImage                  // JPEG, PNG, BMP, TIFF …
	dropTypeDocument               // PDF, DOCX, TXT …
)

func classifyDropFile(path string) dropFileType {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".iso", ".img", ".nrg", ".mdf", ".mds", ".bin", ".cue":
		return dropTypeDiscImage
	case ".ifo", ".vob", ".bup":
		return dropTypeDVDFile
	case ".jpg", ".jpeg", ".png", ".bmp", ".tiff", ".tif", ".webp", ".gif":
		return dropTypeImage
	case ".pdf", ".docx", ".doc", ".txt", ".rtf", ".odt", ".xlsx", ".csv":
		return dropTypeDocument
	case ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm",
		".m4v", ".mpg", ".mpeg", ".3gp", ".ogv", ".ts", ".m2ts":
		return dropTypeVideo
	case ".mp3", ".flac", ".aac", ".opus", ".m4a", ".wav", ".ogg",
		".wma", ".alac", ".ape":
		return dropTypeAudio
	case ".srt", ".vtt", ".ass", ".ssa", ".mks", ".sub":
		return dropTypeSubtitle
	}
	return dropTypeUnknown
}

// dropMismatchMessage returns a human-readable warning when the supplied items
// are a known type that doesn't belong in activeModule. Returns "" when the
// type is correct or unknown (to avoid false positives on exotic extensions).
func dropMismatchMessage(items []fyne.URI, activeModule string) string {
	if len(items) == 0 {
		return ""
	}

	var types []dropFileType
	for _, uri := range items {
		if uri.Scheme() != "file" {
			continue
		}
		t := classifyDropFile(uri.Path())
		if t != dropTypeUnknown {
			types = append(types, t)
		}
	}
	if len(types) == 0 {
		return ""
	}

	first := types[0]
	for _, t := range types[1:] {
		if t != first {
			return "" // mixed types — skip
		}
	}

	switch first {
	case dropTypeDiscImage:
		switch activeModule {
		case "convert", "merge", "trim", "upscale", "filters", "audio", "subtitles", "thumbnail":
			return "Disc images can't be opened here. Use Rip to extract video from an ISO, or Burn to write one to disc."
		}
	case dropTypeDVDFile:
		switch activeModule {
		case "convert", "merge", "trim", "upscale", "filters", "audio", "subtitles", "thumbnail":
			return "DVD structure files (.IFO/.VOB) belong in the Author or Rip module."
		}
	case dropTypeAudio:
		switch activeModule {
		case "convert", "merge", "trim", "upscale", "filters", "thumbnail":
			return "That's an audio file. Drop it on the Audio module to extract or convert it."
		}
	case dropTypeSubtitle:
		switch activeModule {
		case "convert", "merge", "trim", "upscale", "filters", "audio", "thumbnail":
			return "Subtitle files belong in the Subtitles module."
		}
	case dropTypeImage:
		return "Image files aren't supported here. VideoTools works with video and audio files."
	case dropTypeDocument:
		return "That file type isn't supported by VideoTools."
	case dropTypeVideo:
		switch activeModule {
		case "burn":
			return "Burn only accepts disc images (.ISO). Convert your video to ISO via Author first."
		case "rip":
			return "Rip extracts from discs or disc images, not video files. Try Convert, Trim, or Merge."
		}
	}
	return ""
}

func firstLocalDropPath(items []fyne.URI) string {
	for _, uri := range items {
		if uri.Scheme() == "file" {
			return uri.Path()
		}
	}
	return ""
}

// findVideoFiles recursively finds all video files in a directory
func (s *appState) findVideoFiles(dir string) []string {
	var videos []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !info.IsDir() && s.isVideoFile(path) {
			videos = append(videos, path)
		}
		return nil
	})

	if err != nil {
		logging.Debug(logging.CatModule, "error walking directory %s: %v", dir, err)
	}

	return videos
}

// batchAddToQueue adds multiple videos to the queue
func (s *appState) batchAddToQueue(paths []string) {
	logging.Debug(logging.CatModule, "batch adding %d videos to queue", len(paths))
	t := i18n.T()

	addedCount := 0
	failedCount := 0
	var failedFiles []string
	var firstValidPath string

	for _, path := range paths {
		// Load video metadata
		src, err := probeVideo(path)
		if err != nil {
			logging.Debug(logging.CatModule, "failed to parse metadata for %s: %v", path, err)
			failedCount++
			failedFiles = append(failedFiles, filepath.Base(path))
			continue
		}

		// Remember the first valid video to load later
		if firstValidPath == "" {
			firstValidPath = path
		}

		// Create job config
		outDir := filepath.Dir(path)
		outputBase := s.resolveOutputBase(src, false)
		outName := outputBase + s.convert.SelectedFormat.Ext
		outPath := filepath.Join(outDir, outName)

		config := map[string]interface{}{
			"inputPath":         path,
			"outputPath":        outPath,
			"outputBase":        outputBase,
			"selectedFormat":    s.convert.SelectedFormat,
			"quality":           s.convert.Quality,
			"mode":              s.convert.Mode,
			"preserveChapters":  s.convert.PreserveChapters,
			"videoCodec":        s.convert.VideoCodec,
			"encoderPreset":     s.convert.EncoderPreset,
			"crf":               s.convert.CRF,
			"bitrateMode":       s.convert.BitrateMode,
			"bitratePreset":     s.convert.BitratePreset,
			"videoBitrate":      s.convert.VideoBitrate,
			"targetResolution":  s.convert.TargetResolution,
			"frameRate":         s.convert.FrameRate,
			"pixelFormat":       s.convert.PixelFormat,
			"hardwareAccel":     s.convert.HardwareAccel,
			"twoPass":           s.convert.TwoPass,
			"encoderTune":       s.convert.EncoderTune,
			"h264Profile":       s.convert.H264Profile,
			"h264Level":         s.convert.H264Level,
			"deinterlace":       s.convert.Deinterlace,
			"deinterlaceMethod": s.convert.DeinterlaceMethod,
			"audioCodec":        s.convert.AudioCodec,
			"audioBitrate":      s.convert.AudioBitrate,
			"audioChannels":     s.convert.AudioChannels,
			"audioSampleRate":   s.convert.AudioSampleRate,
			"normalizeAudio":    s.convert.NormalizeAudio,
			"inverseTelecine":   s.convert.InverseTelecine,
			"coverArtPath":      "",
			"aspectHandling":    s.convert.AspectHandling,
			"outputAspect":      s.convert.OutputAspect,
			"forceAspect":       s.convert.ForceAspect,
			"sourceWidth":       src.Width,
			"sourceHeight":      src.Height,
			"sampleAspectRatio": src.SampleAspectRatio,
			"sourceBitrate":     src.Bitrate,
			"sourceDuration":    src.Duration,
			"fieldOrder":        src.FieldOrder,
		}

		job := &queue.Job{
			Type:        queue.JobTypeConvert,
			Title:       fmt.Sprintf("Convert %s", filepath.Base(path)),
			Description: fmt.Sprintf("Output: %s  %s", filepath.Base(path), filepath.Base(outPath)),
			InputFile:   path,
			OutputFile:  outPath,
			Config:      config,
		}

		// Generate thumbnail in background
		s.generateJobThumbnail(job)

		s.jobQueue.Add(job)
		addedCount++
	}

	// Show confirmation dialog
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		if addedCount > 0 {
			msg := fmt.Sprintf("Added %d video(s) to the queue!", addedCount)
			if failedCount > 0 {
				msg += fmt.Sprintf("\n\n%d file(s) failed to analyze:\n%s", failedCount, strings.Join(failedFiles, ", "))
			}
			dialog.ShowInformation(t.DialogBatchAdd, msg, s.window)
		} else {
			// All files failed
			msg := fmt.Sprintf("Failed to analyze %d file(s):\n%s", failedCount, strings.Join(failedFiles, ", "))
			s.showErrorWithCopy("Batch Add Failed", fmt.Errorf("%s", msg))
		}

		// Load all valid videos so user can navigate between them
		if firstValidPath != "" {
			combined := make([]string, 0, len(s.loadedVideos)+len(paths))
			seen := make(map[string]bool)
			for _, v := range s.loadedVideos {
				if v != nil && !seen[v.Path] {
					combined = append(combined, v.Path)
					seen[v.Path] = true
				}
			}
			for _, p := range paths {
				if !seen[p] {
					combined = append(combined, p)
					seen[p] = true
				}
			}
			s.loadVideos(combined)
			s.showModule("convert")
		}
	}, false)
}

// batchAddToUpscaleQueue adds multiple videos to the upscale queue
func (s *appState) batchAddToUpscaleQueue(paths []string) {
	logging.Debug(logging.CatModule, "batch adding %d videos to upscale queue", len(paths))
	t := i18n.T()

	addedCount := 0
	failedCount := 0
	var failedFiles []string
	var validPaths []string

	batchGroupID := fmt.Sprintf("upscale-batch-%d", time.Now().Unix())

	for _, path := range paths {
		src, err := probeVideo(path)
		if err != nil {
			logging.Debug(logging.CatModule, "failed to parse metadata for %s: %v", path, err)
			failedCount++
			failedFiles = append(failedFiles, filepath.Base(path))
			continue
		}

		validPaths = append(validPaths, path)

		outDir := filepath.Dir(path)
		outputBase := sanitizeForPath(src.Format) + "_upscale"
		outName := outputBase + ".mp4"
		outPath := filepath.Join(outDir, outName)

		targetWidth, targetHeight, preserveAR, _ := parseResolutionPreset(s.upscaleTargetRes, src.Width, src.Height)
		if s.upscaleTargetRes == "Custom" {
			targetWidth = s.upscaleCustomWidth
			targetHeight = s.upscaleCustomHeight
		}

		config := map[string]interface{}{
			"inputPath":              path,
			"outputPath":             outPath,
			"outputBase":             outputBase,
			"method":                 s.upscaleMethod,
			"targetRes":              s.upscaleTargetRes,
			"targetWidth":            targetWidth,
			"targetHeight":           targetHeight,
			"customWidth":            s.upscaleCustomWidth,
			"customHeight":           s.upscaleCustomHeight,
			"preserveAR":             preserveAR,
			"sourceWidth":            src.Width,
			"sourceHeight":           src.Height,
			"duration":               src.Duration,
			"sourceFrameRate":        src.FrameRate,
			"qualityPreset":          s.upscaleQualityPreset,
			"useAI":                  s.upscaleAIEnabled,
			"aiModel":                s.upscaleAIModel,
			"aiBackend":              s.upscaleAIBackend,
			"aiPreset":               s.upscaleAIPreset,
			"aiScale":                s.upscaleAIScale,
			"aiScaleUseTarget":       s.upscaleAIScaleUseTarget,
			"aiOutputAdjust":         s.upscaleAIOutputAdjust,
			"aiFaceEnhance":          s.upscaleAIFaceEnhance,
			"aiDenoise":              s.upscaleAIDenoise,
			"aiTile":                 s.upscaleAITile,
			"aiGPU":                  s.upscaleAIGPU,
			"aiGPUAuto":              s.upscaleAIGPUAuto,
			"aiThreadsLoad":          s.upscaleAIThreadsLoad,
			"aiThreadsProc":          s.upscaleAIThreadsProc,
			"aiThreadsSave":          s.upscaleAIThreadsSave,
			"aiTTA":                  s.upscaleAITTA,
			"aiOutputFormat":         s.upscaleAIOutputFormat,
			"applyFilters":           s.upscaleApplyFilters,
			"filterChain":            s.upscaleFilterChain,
			"frameRate":              s.upscaleFrameRate,
			"useMotionInterpolation": s.upscaleMotionInterpolation,
			"useRIFE":                s.upscaleRIFEEnabled,
			"rifeModel":              s.upscaleRIFEModel,
			"rifeMultiplier":         s.upscaleRIFEMultiplier,
			"blurEnabled":            s.upscaleBlurEnabled,
			"blurSigma":              s.upscaleBlurSigma,
			"encoderPreset":          s.upscaleEncoderPreset,
			"videoCodec":             s.upscaleVideoCodec,
			"bitrateMode":            s.upscaleBitrateMode,
			"bitratePreset":          s.upscaleBitratePreset,
			"manualBitrate":          s.upscaleManualBitrate,
			"hardwareAccel":          s.upscaleHardwareAccel,
			"outputContainer":        s.upscaleOutputContainer,
			"manualCRF":              s.upscaleManualCRF,
			"pixelFormat":            s.upscalePixelFormat,
			"srcColorSpace":          s.upscaleSrcColorSpace,
			"colorDepth":             s.upscaleColorDepth,
		}

		job := &queue.Job{
			Type:        queue.JobTypeUpscale,
			Title:       fmt.Sprintf("Upscale %s", filepath.Base(path)),
			Description: fmt.Sprintf("Output: %s", filepath.Base(outPath)),
			InputFile:   path,
			OutputFile:  outPath,
			Config:      config,
			GroupID:     batchGroupID,
		}

		// Generate thumbnail in background
		s.generateJobThumbnail(job)

		s.jobQueue.Add(job)
		addedCount++
	}

	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		if addedCount > 0 {
			msg := fmt.Sprintf("Added %d video(s) to the upscale queue!", addedCount)
			if failedCount > 0 {
				msg += fmt.Sprintf("\n\n%d file(s) failed to analyze:\n%s", failedCount, strings.Join(failedFiles, ", "))
			}
			dialog.ShowInformation(t.DialogBatchAdd, msg, s.window)
		} else {
			msg := fmt.Sprintf("Failed to analyze %d file(s):\n%s", failedCount, strings.Join(failedFiles, ", "))
			s.showErrorWithCopy("Batch Upscale Add Failed", fmt.Errorf("%s", msg))
		}

		if len(validPaths) > 0 {
			s.loadVideos(validPaths)
			s.showModule("upscale")
		}
	}, false)
}

func (s *appState) showConvertView(file *videoSource) {
	if s.active != "convert" {
		s.lastModule = s.active
	}
	s.active = "convert"

	// Build the content first
	content := buildConvertView(s, file)

	// Wrap with Droppable for drag-drop support
	s.setContent(ui.NewDroppable(content, func(items []fyne.URI) {
		s.handleDrop(fyne.NewPos(0, 0), items)
	}))
}

func videoSourceToConvertSource(v *videoSource) *convertmodule.VideoSourceInfo {
	if v == nil {
		return nil
	}
	return &convertmodule.VideoSourceInfo{
		Path:              v.Path,
		DisplayName:       v.DisplayName,
		Width:             v.Width,
		Height:            v.Height,
		Duration:          v.Duration,
		FrameRate:         v.FrameRate,
		Format:            v.Format,
		Bitrate:           v.Bitrate,
		VideoCodec:        v.VideoCodec,
		AudioCodec:        v.AudioCodec,
		AudioBitrate:      v.AudioBitrate,
		AudioRate:         v.AudioRate,
		FieldOrder:        v.FieldOrder,
		ColorSpace:        v.ColorSpace,
		ColorRange:        v.ColorRange,
		SampleAspectRatio: v.SampleAspectRatio,
		GOPSize:           v.GOPSize,
		HasChapters:       v.HasChapters,
		HasMetadata:       v.HasMetadata,
		PreviewFrames:     v.PreviewFrames,
	}
}

func convertSourceToVideoSource(v *convertmodule.VideoSourceInfo) *videoSource {
	if v == nil {
		return nil
	}
	return &videoSource{
		Path:              v.Path,
		DisplayName:       v.DisplayName,
		Width:             v.Width,
		Height:            v.Height,
		Duration:          v.Duration,
		FrameRate:         v.FrameRate,
		Format:            v.Format,
		Bitrate:           v.Bitrate,
		VideoCodec:        v.VideoCodec,
		AudioCodec:        v.AudioCodec,
		AudioBitrate:      v.AudioBitrate,
		AudioRate:         v.AudioRate,
		FieldOrder:        v.FieldOrder,
		ColorSpace:        v.ColorSpace,
		ColorRange:        v.ColorRange,
		SampleAspectRatio: v.SampleAspectRatio,
		GOPSize:           v.GOPSize,
		HasChapters:       v.HasChapters,
		HasMetadata:       v.HasMetadata,
		PreviewFrames:     v.PreviewFrames,
	}
}

func (s *appState) showAuthorView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "author"
	s.maximizeWindow()

	// Initialize scene detection threshold if not set
	if s.authorSceneThreshold == 0 {
		s.authorSceneThreshold = 0.3
	}

	// Clear DVD title for fresh start
	s.authorTitle = ""

	s.setContent(buildAuthorView(s))
}

func (s *appState) showTrimView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "trim"
	s.maximizeWindow()
	s.setContent(buildTrimView(s))
}

func buildTrimView(state *appState) fyne.CanvasObject {
	return trim.BuildView(trim.Options{
		Window:         state.window,
		ModuleColor:    moduleColor("trim"),
		StatsBar:       state.statsBar,
		Player:         GetPrimaryPlayer(),
		OnShowMainMenu: state.showMainMenu,
		OnShowQueue:    state.showQueue,
		OnAddToQueue: func(clip trim.TrimClip) {
			state.submitTrimJob(clip)
		},
		OnLoadFile: func(path string) {
			if src, err := probeVideo(path); err == nil {
				state.source = src
				state.currentFrame = ""
				if len(src.PreviewFrames) > 0 {
					state.currentFrame = src.PreviewFrames[0]
				}
				state.applyInverseDefaults(src)
				state.convert.OutputBase = state.resolveOutputBase(src, false)
				state.playerReady = false
				state.playerPos = 0
				state.playerPaused = true
			}
		},
		OnProbeVideo: func(path string) (float64, error) {
			src, err := probeVideo(path)
			if err != nil {
				return 0, err
			}
			return src.Duration, nil
		},
	}, "")
}

func (s *appState) submitTrimJob(clip trim.TrimClip) {
	logging.Info(logging.CatModule, "Submitting trim job for: %s", clip.Path)
	t := i18n.T()

	// Default mode/export when not set by UI (native view stub)
	mode := clip.Mode
	if mode == "" {
		mode = "keep"
	}
	export := clip.Export
	if export == "" {
		export = "copy"
	}

	// Normalise i18n label values to canonical keys
	switch mode {
	case t.TrimModeCut:
		mode = "cut"
	default:
		mode = "keep"
	}
	switch export {
	case t.TrimRecode:
		export = "reencode"
	default:
		export = "copy"
	}

	// Output filename: keep same container unless re-encoding, then default to mp4
	ext := filepath.Ext(clip.Path)
	if export == "reencode" && ext == "" {
		ext = ".mp4"
	}
	base := strings.TrimSuffix(filepath.Base(clip.Path), ext)
	suffix := "_trimmed"
	if mode == "cut" {
		suffix = "_cut"
	}
	outPath := filepath.Join(filepath.Dir(clip.Path), base+suffix+ext)

	modeLabel := "Keep Region"
	if mode == "cut" {
		modeLabel = "Cut Region"
	}
	exportLabel := "Smart Copy"
	if export == "reencode" {
		exportLabel = "Re-encode"
	}

	job := &queue.Job{
		Type:        queue.JobTypeTrim,
		Title:       fmt.Sprintf("Trim: %s", filepath.Base(clip.Path)),
		Description: fmt.Sprintf("%s / %s — %s to %s", modeLabel, exportLabel, formatTrimDuration(clip.InPoint), formatTrimDuration(clip.OutPoint)),
		InputFile:   clip.Path,
		OutputFile:  outPath,
		Config: map[string]interface{}{
			"inputPath":  clip.Path,
			"outputPath": outPath,
			"mode":       mode,
			"export":     export,
			"inPoint":    clip.InPoint.Seconds(),
			"outPoint":   clip.OutPoint.Seconds(),
			"duration":   clip.Duration.Seconds(),
		},
	}

	if s.jobQueue != nil {
		s.generateJobThumbnail(job)
		prevStep := s.pipelineStep
		s.pipelineAdd(job)
		if prevStep == "" {
			dialog.ShowInformation(t.DialogJobQueued, t.TrimJobAdded, s.window)
		}
	}
}

func formatTrimDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	sec := int(d.Seconds()) % 60
	ms := int(d.Milliseconds()) % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, sec, ms)
}

func (s *appState) executeTrimJob(ctx context.Context, job *queue.Job, progressCallback func(float64)) error {
	cfg := job.Config
	inputPath, _ := cfg["inputPath"].(string)
	outputPath, _ := cfg["outputPath"].(string)
	mode, _ := cfg["mode"].(string)
	export, _ := cfg["export"].(string)
	inPoint, _ := cfg["inPoint"].(float64)
	outPoint, _ := cfg["outPoint"].(float64)
	duration, _ := cfg["duration"].(float64)

	if inputPath == "" {
		return fmt.Errorf("trim job missing inputPath")
	}
	if outPoint <= inPoint {
		return fmt.Errorf("trim job: outPoint (%.3f) must be after inPoint (%.3f)", outPoint, inPoint)
	}

	ffmpeg := utils.GetFFmpegPath()

	if progressCallback != nil {
		progressCallback(0)
	}

	switch {
	case mode == "keep" && export == "copy":
		// Fast stream-copy: seek before decode for speed
		args := []string{
			"-y", "-hide_banner", "-loglevel", "error",
			"-ss", fmt.Sprintf("%.3f", inPoint),
			"-to", fmt.Sprintf("%.3f", outPoint),
			"-i", inputPath,
			"-map", "0",
			"-avoid_negative_ts", "make_zero",
			"-c", "copy",
			"-progress", "pipe:1", "-nostats",
			outputPath,
		}
		segDur := outPoint - inPoint
		return runFFmpegWithProgress(ctx, ffmpeg, args, segDur, progressCallback)

	case mode == "keep" && export == "reencode":
		args := []string{
			"-y", "-hide_banner", "-loglevel", "error",
			"-threads", "auto",
			"-ss", fmt.Sprintf("%.3f", inPoint),
			"-to", fmt.Sprintf("%.3f", outPoint),
			"-i", inputPath,
			"-map", "0",
			"-c:v", "libx264", "-crf", "18", "-preset", "slow",
			"-c:a", "aac", "-b:a", "192k",
			"-progress", "pipe:1", "-nostats",
			outputPath,
		}
		segDur := outPoint - inPoint
		return runFFmpegWithProgress(ctx, ffmpeg, args, segDur, progressCallback)

	case mode == "cut" && export == "copy":
		// Remove a region: extract before + after, then concat stream-copy
		tmpDir := utils.TempDir()
		ext := filepath.Ext(outputPath)

		seg1, err := os.CreateTemp(tmpDir, "vt-trim-seg1-*"+ext)
		if err != nil {
			return fmt.Errorf("trim cut: create seg1 temp: %w", err)
		}
		seg1Path := seg1.Name()
		seg1.Close()
		defer os.Remove(seg1Path)

		seg2, err := os.CreateTemp(tmpDir, "vt-trim-seg2-*"+ext)
		if err != nil {
			return fmt.Errorf("trim cut: create seg2 temp: %w", err)
		}
		seg2Path := seg2.Name()
		seg2.Close()
		defer os.Remove(seg2Path)

		// Pass 1: before the cut (0 → inPoint)
		args1 := []string{
			"-y", "-hide_banner", "-loglevel", "error",
			"-to", fmt.Sprintf("%.3f", inPoint),
			"-i", inputPath,
			"-map", "0", "-avoid_negative_ts", "make_zero", "-c", "copy",
			seg1Path,
		}
		if err := runFFmpegSimple(ctx, ffmpeg, args1); err != nil {
			return fmt.Errorf("trim cut seg1: %w", err)
		}
		if progressCallback != nil {
			progressCallback(40)
		}

		// Pass 2: after the cut (outPoint → end)
		args2 := []string{
			"-y", "-hide_banner", "-loglevel", "error",
			"-ss", fmt.Sprintf("%.3f", outPoint),
			"-i", inputPath,
			"-map", "0", "-avoid_negative_ts", "make_zero", "-c", "copy",
			seg2Path,
		}
		if err := runFFmpegSimple(ctx, ffmpeg, args2); err != nil {
			return fmt.Errorf("trim cut seg2: %w", err)
		}
		if progressCallback != nil {
			progressCallback(70)
		}

		// Pass 3: concat the two segments
		listFile, err := os.CreateTemp(tmpDir, "vt-trim-list-*.txt")
		if err != nil {
			return fmt.Errorf("trim cut: create list temp: %w", err)
		}
		defer os.Remove(listFile.Name())
		fmt.Fprintf(listFile, "file '%s'\n", strings.ReplaceAll(seg1Path, "'", "'\\''"))
		fmt.Fprintf(listFile, "file '%s'\n", strings.ReplaceAll(seg2Path, "'", "'\\''"))
		listFile.Close()

		args3 := []string{
			"-y", "-hide_banner", "-loglevel", "error",
			"-f", "concat", "-safe", "0", "-i", listFile.Name(),
			"-map", "0", "-c", "copy",
			"-progress", "pipe:1", "-nostats",
			outputPath,
		}
		keepDur := inPoint + (duration - outPoint)
		if keepDur <= 0 {
			keepDur = inPoint
		}
		return runFFmpegWithProgress(ctx, ffmpeg, args3, keepDur, progressCallback)

	case mode == "cut" && export == "reencode":
		// Remove a region using filter_complex trim+concat
		args := []string{
			"-y", "-hide_banner", "-loglevel", "error",
			"-threads", "auto",
			"-i", inputPath,
			"-filter_complex", fmt.Sprintf(
				"[0:v]trim=0:%.3f,setpts=PTS-STARTPTS[v1];"+
					"[0:a]atrim=0:%.3f,asetpts=PTS-STARTPTS[a1];"+
					"[0:v]trim=start=%.3f,setpts=PTS-STARTPTS[v2];"+
					"[0:a]atrim=start=%.3f,asetpts=PTS-STARTPTS[a2];"+
					"[v1][a1][v2][a2]concat=n=2:v=1:a=1[outv][outa]",
				inPoint, inPoint, outPoint, outPoint,
			),
			"-map", "[outv]", "-map", "[outa]",
			"-c:v", "libx264", "-crf", "18", "-preset", "slow",
			"-c:a", "aac", "-b:a", "192k",
			"-progress", "pipe:1", "-nostats",
			outputPath,
		}
		keepDur := inPoint + (duration - outPoint)
		if keepDur <= 0 {
			keepDur = inPoint
		}
		return runFFmpegWithProgress(ctx, ffmpeg, args, keepDur, progressCallback)
	}

	return fmt.Errorf("trim job: unknown mode=%q export=%q", mode, export)
}

// runFFmpegWithProgress runs an FFmpeg command and parses -progress pipe:1 output.
func runFFmpegWithProgress(ctx context.Context, ffmpegPath string, args []string, totalDur float64, progressCallback func(float64)) error {
	cmd := utils.CreateCommand(ctx, ffmpegPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := utils.StartCmd(cmd); err != nil {
		return fmt.Errorf("ffmpeg start: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}

	go func() {
		scanner := bufio.NewScanner(stdout)
		var lastPct float64
		for scanner.Scan() {
			parts := strings.SplitN(scanner.Text(), "=", 2)
			if len(parts) != 2 || parts[0] != "out_time_ms" || totalDur <= 0 || progressCallback == nil {
				continue
			}
			if ms, err := strconv.ParseFloat(parts[1], 64); err == nil {
				pct := (ms / 1000000.0 / totalDur) * 100
				if pct > 100 {
					pct = 100
				}
				if pct-lastPct >= 0.5 {
					lastPct = pct
					progressCallback(pct)
				}
			}
		}
	}()

	err = cmd.Wait()
	if progressCallback != nil {
		progressCallback(100)
	}
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("ffmpeg failed: %w\n%s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// runFFmpegSimple runs an FFmpeg command without progress tracking.
func runFFmpegSimple(ctx context.Context, ffmpegPath string, args []string) error {
	cmd := utils.CreateCommand(ctx, ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("ffmpeg failed: %w\n%s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func (s *appState) showMergeView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "merge"
	s.maximizeWindow()

	t := i18n.T()
	mergeColor := moduleColor("merge")

	if cfg, err := loadPersistedMergeConfig(); err == nil {
		s.applyMergeConfig(cfg)
	} else if !errors.Is(err, os.ErrNotExist) {
		logging.Error(logging.CatMerge, "failed to load persisted merge config: err=%v", err)
	}

	if s.mergeFormat == "" {
		s.mergeFormat = "mkv-copy"
	}
	if s.mergeDVDRegion == "" {
		s.mergeDVDRegion = "NTSC"
	}
	if s.mergeDVDAspect == "" {
		s.mergeDVDAspect = "16:9"
	}
	if s.mergeFrameRate == "" {
		s.mergeFrameRate = "Source"
	}

	backBtn := ui.MakePillButton("< "+strings.ToUpper(t.ModuleMerge), ui.BorderDim, func() {
		s.showMainMenu()
	})

	queueBtn := ui.MakePillButton("View Queue", ui.BorderDim, func() {
		s.showQueue()
	})
	s.queueBtn = queueBtn
	s.updateQueueButtonLabel()

	topBar := ui.TintedBar(mergeColor, container.NewHBox(backBtn, layout.NewSpacer(), queueBtn))

	listBox := container.NewVBox()
	var addFiles func([]string)
	var addQueueBtn *ui.PillButton
	var runNowBtn *ui.PillButton

	var buildList func()
	buildList = func() {
		listBox.Objects = nil
		if len(s.mergeClips) == 0 {
			emptyLabel := widget.NewLabel(t.MergeAddClipsHint)
			emptyLabel.Alignment = fyne.TextAlignCenter
			// Make empty state a drop target
			emptyDrop := ui.NewDroppable(container.NewCenter(emptyLabel), func(items []fyne.URI) {
				var paths []string
				for _, uri := range items {
					if uri.Scheme() == "file" {
						paths = append(paths, uri.Path())
					}
				}
				if len(paths) > 0 {
					addFiles(paths)
				}
			})
			listBox.Add(container.NewMax(emptyDrop))
		} else {
			for i, c := range s.mergeClips {
				idx := i
				name := filepath.Base(c.Path)
				durStr := ""
				if c.Duration > 0 {
					d := time.Duration(c.Duration * float64(time.Second))
					durStr = fmt.Sprintf(" [%02d:%02d:%02d]", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
				}
				label := widget.NewLabel(utils.ShortenMiddle(name, 40) + durStr)
				chEntry := widget.NewEntry()
				chEntry.SetText(c.Chapter)
				chEntry.SetPlaceHolder(fmt.Sprintf("Part %d", i+1))
				chEntry.OnChanged = func(val string) {
					s.mergeClips[idx].Chapter = val
				}
			upBtn := ui.MakePillButton("↑", ui.BorderDim, func() {
				if idx > 0 {
					s.mergeClips[idx-1], s.mergeClips[idx] = s.mergeClips[idx], s.mergeClips[idx-1]
					buildList()
				}
			})
			downBtn := ui.MakePillButton("↓", ui.BorderDim, func() {
				if idx < len(s.mergeClips)-1 {
					s.mergeClips[idx+1], s.mergeClips[idx] = s.mergeClips[idx], s.mergeClips[idx+1]
					buildList()
				}
			})
			delBtn := ui.MakePillButton("Remove", ui.BorderDim, func() {
				s.mergeClips = append(s.mergeClips[:idx], s.mergeClips[idx+1:]...)
				buildList()
			})
				row := container.NewBorder(
					nil, nil,
					container.NewVBox(upBtn, downBtn),
					delBtn,
					container.NewVBox(label, chEntry),
				)
				cardBg := canvas.NewRectangle(utils.MustHex("#171C2A"))
				cardBg.CornerRadius = 6
				cardBg.SetMinSize(fyne.NewSize(0, label.MinSize().Height+chEntry.MinSize().Height+12))
				listBox.Add(container.NewPadded(container.NewMax(cardBg, row)))
			}
		}
		listBox.Refresh()
		if addQueueBtn != nil && runNowBtn != nil {
			if len(s.mergeClips) >= 2 {
				addQueueBtn.Enable()
				runNowBtn.Enable()
			} else {
				addQueueBtn.Disable()
				runNowBtn.Disable()
			}
		}
	}

	addFiles = func(paths []string) {
		// Filter to video files only before probing.
		var videoPaths []string
		for _, p := range paths {
			if s.isVideoFile(p) {
				videoPaths = append(videoPaths, p)
			} else if msg := dropMismatchMessage([]fyne.URI{storage.NewFileURI(p)}, "merge"); msg != "" {
				ui.ShowToast(s.window, msg, ui.ToastWarning)
				return
			}
		}
		if len(videoPaths) == 0 {
			return
		}

		// Probe off the main thread — probeVideo runs ffprobe and must not block UI.
		go func() {
			var added []mergeClip
			for _, p := range videoPaths {
				src, err := probeVideo(p)
				if err != nil {
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						dialog.ShowError(fmt.Errorf("failed to probe %s: %w", filepath.Base(p), err), s.window)
					}, false)
					continue
				}
				added = append(added, mergeClip{
					Path:     p,
					Chapter:  strings.TrimSuffix(filepath.Base(p), filepath.Ext(p)),
					Duration: src.Duration,
				})
			}
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				s.mergeClips = append(s.mergeClips, added...)
				if len(s.mergeClips) >= 2 && s.mergeOutputDir == "" {
					s.mergeOutputDir = filepath.Dir(s.mergeClips[0].Path)
				}
				if len(s.mergeClips) >= 2 && s.mergeOutputFilename == "" {
					s.mergeOutputFilename = "merged.mkv"
				}
				buildList()
			}, false)
		}()
	}

	addBtn := ui.MakePillButton("Add Files", ui.BorderDim, func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			path := reader.URI().Path()
			reader.Close()
			addFiles([]string{path})
		}, s.window)
	})

	// Helper to get file extension for format
	getExtForFormat := func(format string) string {
		switch {
		case strings.HasPrefix(format, "dvd"):
			return ".mpg"
		case strings.HasPrefix(format, "webm"):
			return ".webm"
		case strings.HasPrefix(format, "av1"):
			return ".mp4"
		case strings.HasPrefix(format, "mkv"), strings.HasPrefix(format, "bd"):
			return ".mkv"
		case strings.HasPrefix(format, "mp4"):
			return ".mp4"
		default:
			return ".mkv"
		}
	}

	formatMap := map[string]string{
		"Fast Merge (No Re-encoding)": "mkv-copy",
		"Lossless MKV (Best Quality)": "mkv-lossless",
		"MP4 (H.264)":                 "mp4-h264",
		"MP4 (H.265)":                 "mp4-h265",
		"MP4 (AV1)":                   "av1",
		"WebM (VP9)":                  "webm-vp9",
		"DVD Format":                  "dvd",
	}
	// Maintain order for dropdown
	formatKeys := []string{
		"Fast Merge (No Re-encoding)",
		"Lossless MKV (Best Quality)",
		"MP4 (H.264)",
		"MP4 (H.265)",
		"MP4 (AV1)",
		"WebM (VP9)",
		"DVD Format",
	}

	keepAllCheck := widget.NewCheck("Keep all audio/subtitle tracks", func(v bool) {
		s.mergeKeepAll = v
		s.persistMergeConfig()
	})
	keepAllCheck.SetChecked(s.mergeKeepAll)

	chapterCheck := widget.NewCheck("Create chapters from each clip", func(v bool) {
		s.mergeChapters = v
		s.persistMergeConfig()
	})
	chapterCheck.SetChecked(s.mergeChapters)

	// Create output entry widgets first so they can be referenced in callbacks
	outputDirEntry := widget.NewEntry()
	outputDirEntry.SetPlaceHolder("Output folder path")
	outputDirEntry.SetText(s.mergeOutputDir)
	outputDirEntry.OnChanged = func(val string) {
		s.mergeOutputDir = val
	}

	outputFilenameEntry := widget.NewEntry()
	outputFilenameEntry.SetPlaceHolder("merged.mkv")
	outputFilenameEntry.SetText(s.mergeOutputFilename)
	outputFilenameEntry.OnChanged = func(val string) {
		s.mergeOutputFilename = val
	}

	clearBtn := ui.MakePillButton("Clear", ui.BorderDim, func() {
		s.mergeClips = nil
		s.mergeOutputDir = ""
		s.mergeOutputFilename = ""
		outputDirEntry.SetText("")
		outputFilenameEntry.SetText("")
		buildList()
	})

	// Helper to update output filename extension (requires outputFilenameEntry to exist)
	updateOutputExt := func() {
		if s.mergeOutputFilename == "" {
			return
		}
		currentExt := filepath.Ext(s.mergeOutputFilename)
		correctExt := getExtForFormat(s.mergeFormat)
		if currentExt != correctExt {
			s.mergeOutputFilename = strings.TrimSuffix(s.mergeOutputFilename, currentExt) + correctExt
			outputFilenameEntry.SetText(s.mergeOutputFilename)
		}
	}

	// DVD-specific options
	dvdRegionOptions := []string{"NTSC", "PAL"}
	dvdRegionSelect := widget.NewSelect(dvdRegionOptions, func(val string) {
		s.mergeDVDRegion = val
		s.persistMergeConfig()
	})
	dvdRegionSelect.SetSelected(s.mergeDVDRegion)

	dvdAspectOptions := []string{"16:9", "4:3"}
	dvdAspectSelect := widget.NewSelect(dvdAspectOptions, func(val string) {
		s.mergeDVDAspect = val
		s.persistMergeConfig()
	})
	dvdAspectSelect.SetSelected(s.mergeDVDAspect)

	dvdOptionsRow := container.NewHBox(
		widget.NewLabel(t.MergeRegion),
		dvdRegionSelect,
		widget.NewLabel(t.MergeAspect),
		dvdAspectSelect,
	)

	// Container for DVD options (can be shown/hidden)
	dvdOptionsContainer := container.NewVBox(dvdOptionsRow)

	// Create format selector (after outputEntry and updateOutputExt are defined)
	formatColors := ui.BuildFormatColorMap(formatKeys)
	formatSelect := ui.NewColoredSelect(formatKeys, formatColors, func(val string) {
		s.mergeFormat = formatMap[val]

		// Show/hide DVD options based on selection
		if s.mergeFormat == "dvd" {
			dvdOptionsContainer.Show()
		} else {
			dvdOptionsContainer.Hide()
		}

		// Set default output directory if not set
		if s.mergeOutputDir == "" && len(s.mergeClips) > 0 {
			s.mergeOutputDir = filepath.Dir(s.mergeClips[0].Path)
			outputDirEntry.SetText(s.mergeOutputDir)
		}

		// Set default output filename if not set
		if s.mergeOutputFilename == "" && len(s.mergeClips) > 0 {
			ext := getExtForFormat(s.mergeFormat)
			basename := "merged"
			if strings.HasPrefix(s.mergeFormat, "dvd") || s.mergeFormat == "dvd" {
				basename = "merged-dvd"
			} else if strings.HasPrefix(s.mergeFormat, "bd") {
				basename = "merged-bd"
			} else if s.mergeFormat == "mkv-lossless" {
				basename = "merged-lossless"
			}
			s.mergeOutputFilename = basename + ext
			outputFilenameEntry.SetText(s.mergeOutputFilename)
		} else {
			// Update extension of existing filename
			updateOutputExt()
		}
		s.persistMergeConfig()
	}, s.window)
	for label, val := range formatMap {
		if val == s.mergeFormat {
			formatSelect.SetSelected(label)
			break
		}
	}

	// Initialize DVD options visibility
	if s.mergeFormat == "dvd" {
		dvdOptionsContainer.Show()
	} else {
		dvdOptionsContainer.Hide()
	}

	// Frame Rate controls
	frameRateOptions := []string{"Source", "23.976", "24", "25", "29.97", "30", "50", "59.94", "60"}
	frameRateSelect := widget.NewSelect(frameRateOptions, func(val string) {
		s.mergeFrameRate = val
		s.persistMergeConfig()
	})
	frameRateSelect.SetSelected(s.mergeFrameRate)

	motionInterpCheck := widget.NewCheck("Use Motion Interpolation (slower, smoother)", func(checked bool) {
		s.mergeMotionInterpolation = checked
		s.persistMergeConfig()
	})
	motionInterpCheck.SetChecked(s.mergeMotionInterpolation)

	frameRateRow := container.NewVBox(
		widget.NewLabel(t.ConvertFrameRate),
		frameRateSelect,
		motionInterpCheck,
	)

	browseDirBtn := ui.MakePillButton("Browse Folder", ui.BorderDim, func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			s.mergeOutputDir = uri.Path()
			outputDirEntry.SetText(s.mergeOutputDir)
		}, s.window)
	})

	addQueueBtn = ui.MakePillButton("Add Merge to Queue", ui.BorderDim, func() {
		if err := s.addMergeToQueue(false); err != nil {
			dialog.ShowError(err, s.window)
			return
		}
		ui.ShowToast(s.window, "Merge job added to queue.", ui.ToastInfo)
		if s.jobQueue != nil && !s.jobQueue.IsRunning() {
			s.jobQueue.Start()
		}
	})
	runNowBtn = ui.MakePillButton("Merge Now", ui.BorderDim, func() {
		if err := s.addMergeToQueue(true); err != nil {
			dialog.ShowError(err, s.window)
			return
		}
		if s.jobQueue != nil && !s.jobQueue.IsRunning() {
			s.jobQueue.Start()
		}
		ui.ShowToast(s.window, t.MergeStarted, ui.ToastInfo)
	})
	if len(s.mergeClips) < 2 {
		addQueueBtn.Disable()
		runNowBtn.Disable()
	}

	applyMergeControls := func() {
		for label, val := range formatMap {
			if val == s.mergeFormat {
				formatSelect.SetSelected(label)
				break
			}
		}
		keepAllCheck.SetChecked(s.mergeKeepAll)
		chapterCheck.SetChecked(s.mergeChapters)
		dvdRegionSelect.SetSelected(s.mergeDVDRegion)
		dvdAspectSelect.SetSelected(s.mergeDVDAspect)
		frameRateSelect.SetSelected(s.mergeFrameRate)
		motionInterpCheck.SetChecked(s.mergeMotionInterpolation)
		if s.mergeFormat == "dvd" {
			dvdOptionsContainer.Show()
		} else {
			dvdOptionsContainer.Hide()
		}
		updateOutputExt()
	}

	loadCfgBtn := ui.MakePillButton("Load Config", ui.BorderDim, func() {
		cfg, err := loadPersistedMergeConfig()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				dialog.ShowInformation(t.DialogNoConfig, "No saved config found yet. It will save automatically after your first change.", s.window)
			} else {
				dialog.ShowError(fmt.Errorf("failed to load config: %w", err), s.window)
			}
			return
		}
		s.applyMergeConfig(cfg)
		applyMergeControls()
	})

	saveCfgBtn := ui.MakePillButton("Save Config", ui.BorderDim, func() {
		cfg := mergeConfig{
			Format:              s.mergeFormat,
			KeepAllStreams:      s.mergeKeepAll,
			Chapters:            s.mergeChapters,
			CodecMode:           s.mergeCodecMode,
			DVDRegion:           s.mergeDVDRegion,
			DVDAspect:           s.mergeDVDAspect,
			FrameRate:           s.mergeFrameRate,
			MotionInterpolation: s.mergeMotionInterpolation,
		}
		if err := savePersistedMergeConfig(cfg); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save config: %w", err), s.window)
			return
		}
		dialog.ShowInformation(t.DialogConfigSaved, fmt.Sprintf("Saved to %s", configpath.ModuleConfigPath("merge")), s.window)
	})

	resetBtn := ui.MakePillButton("Reset", ui.BorderDim, func() {
		cfg := defaultMergeConfig()
		s.applyMergeConfig(cfg)
		applyMergeControls()
		s.persistMergeConfig()
	})

	listScroll := container.NewVScroll(listBox)

	// Use border layout so the list expands to fill available vertical space
	leftTop := container.NewVBox(
		widget.NewLabelWithStyle(t.ConvertSectionClipsToMerge, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(addBtn, clearBtn),
	)

	left := container.NewBorder(
		leftTop, // top
		nil,     // bottom
		nil,     // left
		nil,     // right
		ui.NewDroppable(listScroll, func(items []fyne.URI) {
			var paths []string
			for _, uri := range items {
				if uri.Scheme() == "file" {
					paths = append(paths, uri.Path())
				}
			}
			if len(paths) > 0 {
				addFiles(paths)
			}
		}),
	)

	rightPanel := container.NewVBox(
		widget.NewLabelWithStyle(t.ConvertSectionOutputOptions, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(t.ConvertFormat),
		formatSelect,
		dvdOptionsContainer,
		widget.NewSeparator(),
		frameRateRow,
		widget.NewSeparator(),
		keepAllCheck,
		chapterCheck,
		widget.NewSeparator(),
		widget.NewLabel(t.ConvertOutputFolder),
		container.NewBorder(nil, nil, nil, browseDirBtn, outputDirEntry),
		widget.NewLabel(t.ConvertOutputFilename),
		outputFilenameEntry,
		widget.NewSeparator(),
		container.NewHBox(resetBtn, loadCfgBtn, saveCfgBtn),
	)

	right := container.NewVScroll(rightPanel)
	bottomBar := moduleFooter(mergeColor, container.NewHBox(addQueueBtn, layout.NewSpacer(), runNowBtn), s.statsBar)
	content := container.New(&fixedHSplitLayout{ratio: 0.6}, left, right)
	s.setContent(container.NewBorder(topBar, bottomBar, nil, nil, container.NewPadded(content)))

	buildList()
	s.updateStatsBar()
}

func (s *appState) addMergeToQueue(startNow bool) error {
	if len(s.mergeClips) < 2 {
		return fmt.Errorf("add at least two clips")
	}

	// Set defaults if not specified
	if strings.TrimSpace(s.mergeOutputDir) == "" {
		s.mergeOutputDir = filepath.Dir(s.mergeClips[0].Path)
	}
	if strings.TrimSpace(s.mergeOutputFilename) == "" {
		s.mergeOutputFilename = "merged.mkv"
	}

	// Ensure output filename has correct extension for selected format
	currentExt := filepath.Ext(s.mergeOutputFilename)
	var correctExt string
	switch {
	case strings.HasPrefix(s.mergeFormat, "dvd"):
		correctExt = ".mpg"
	case strings.HasPrefix(s.mergeFormat, "webm"):
		correctExt = ".webm"
	case strings.HasPrefix(s.mergeFormat, "av1"):
		correctExt = ".mp4"
	case strings.HasPrefix(s.mergeFormat, "mkv"), strings.HasPrefix(s.mergeFormat, "bd"):
		correctExt = ".mkv"
	case strings.HasPrefix(s.mergeFormat, "mp4"):
		correctExt = ".mp4"
	default:
		correctExt = ".mkv"
	}

	// Auto-fix extension if missing or wrong
	if currentExt == "" {
		s.mergeOutputFilename += correctExt
	} else if currentExt != correctExt {
		s.mergeOutputFilename = strings.TrimSuffix(s.mergeOutputFilename, currentExt) + correctExt
	}

	// Combine dir and filename to create full output path
	mergeOutput := filepath.Join(s.mergeOutputDir, s.mergeOutputFilename)
	clips := make([]map[string]interface{}, 0, len(s.mergeClips))
	for _, c := range s.mergeClips {
		name := c.Chapter
		if strings.TrimSpace(name) == "" {
			name = strings.TrimSuffix(filepath.Base(c.Path), filepath.Ext(c.Path))
		}
		clips = append(clips, map[string]interface{}{
			"path":     c.Path,
			"chapter":  name,
			"duration": c.Duration,
		})
	}

	config := map[string]interface{}{
		"clips":                  clips,
		"format":                 s.mergeFormat,
		"keepAllStreams":         s.mergeKeepAll,
		"chapters":               s.mergeChapters,
		"codecMode":              s.mergeCodecMode,
		"outputPath":             mergeOutput,
		"dvdRegion":              s.mergeDVDRegion,
		"dvdAspect":              s.mergeDVDAspect,
		"frameRate":              s.mergeFrameRate,
		"useMotionInterpolation": s.mergeMotionInterpolation,
	}

	job := &queue.Job{
		Type:        queue.JobTypeMerge,
		Title:       fmt.Sprintf("Merge %d clips", len(clips)),
		Description: fmt.Sprintf("Output: %s", utils.ShortenMiddle(filepath.Base(mergeOutput), 40)),
		InputFile:   clips[0]["path"].(string),
		OutputFile:  mergeOutput,
		Config:      config,
	}
	s.generateJobThumbnail(job)
	s.jobQueue.Add(job)
	if startNow && s.jobQueue != nil && !s.jobQueue.IsRunning() {
		s.jobQueue.Start()
	}
	return nil
}

// jobExecutor executes a job from the queue
func (s *appState) jobExecutor(ctx context.Context, job *queue.Job, progressCallback func(float64)) error {
	logging.Debug(logging.CatSystem, "executing job %s: %s", job.ID, job.Title)

	switch job.Type {
	case queue.JobTypeConvert:
		return s.executeConvertJob(ctx, job, progressCallback)
	case queue.JobTypeMerge:
		return s.executeMergeJob(ctx, job, progressCallback)
	case queue.JobTypeTrim:
		return s.executeTrimJob(ctx, job, progressCallback)
	case queue.JobTypeFilter:
		return s.executeFilterJob(ctx, job, progressCallback)
	case queue.JobTypeUpscale:
		return s.executeUpscaleJob(ctx, job, progressCallback)
	case queue.JobTypeAudio:
		return s.executeAudioJob(ctx, job, progressCallback)
	case queue.JobTypeThumbnail:
		return s.executeThumbnailJob(ctx, job, progressCallback)
	case queue.JobTypeSnippet:
		return s.executeSnippetJob(ctx, job, progressCallback)
	case queue.JobTypeAuthor:
		return s.executeAuthorJob(ctx, job, progressCallback)
	case queue.JobTypeRip:
		return s.executeRipJob(ctx, job, progressCallback)
	case queue.JobTypeBurn:
		return s.executeBurnJob(ctx, job, progressCallback)
	default:
		return fmt.Errorf("unknown job type: %s", job.Type)
	}
}

func (s *appState) executeMergeJob(ctx context.Context, job *queue.Job, progressCallback func(float64)) error {
	cfg := job.Config
	format, _ := cfg["format"].(string)
	keepAll, _ := cfg["keepAllStreams"].(bool)
	withChapters, ok := cfg["chapters"].(bool)
	if !ok {
		withChapters = true
	}
	_ = cfg["codecMode"] // Deprecated: kept for backward compatibility with old queue jobs
	outputPath, _ := cfg["outputPath"].(string)

	rawClips, _ := cfg["clips"].([]interface{})
	rawClipMaps, _ := cfg["clips"].([]map[string]interface{})
	var clips []mergeClip
	if len(rawClips) > 0 {
		for _, rc := range rawClips {
			if m, ok := rc.(map[string]interface{}); ok {
				clips = append(clips, mergeClip{
					Path:     toString(m["path"]),
					Chapter:  toString(m["chapter"]),
					Duration: toFloat(m["duration"]),
				})
			}
		}
	} else if len(rawClipMaps) > 0 {
		for _, m := range rawClipMaps {
			clips = append(clips, mergeClip{
				Path:     toString(m["path"]),
				Chapter:  toString(m["chapter"]),
				Duration: toFloat(m["duration"]),
			})
		}
	}
	if len(clips) < 2 {
		return fmt.Errorf("need at least two clips to merge")
	}

	tmpDir := utils.TempDir()
	listFile, err := os.CreateTemp(tmpDir, "vt-merge-list-*.txt")
	if err != nil {
		return err
	}
	defer os.Remove(listFile.Name())
	for _, c := range clips {
		fmt.Fprintf(listFile, "file '%s'\n", strings.ReplaceAll(c.Path, "'", "'\\''"))
	}
	_ = listFile.Close()

	var chapterFile *os.File
	if withChapters {
		chapterFile, err = os.CreateTemp(tmpDir, "vt-merge-chapters-*.txt")
		if err != nil {
			return err
		}
		var elapsed float64
		fmt.Fprintln(chapterFile, ";FFMETADATA1")
		for i, c := range clips {
			startMs := int64(elapsed * 1000)
			endMs := int64((elapsed + c.Duration) * 1000)
			fmt.Fprintln(chapterFile, "[CHAPTER]")
			fmt.Fprintln(chapterFile, "TIMEBASE=1/1000")
			fmt.Fprintf(chapterFile, "START=%d\n", startMs)
			fmt.Fprintf(chapterFile, "END=%d\n", endMs)
			name := c.Chapter
			if strings.TrimSpace(name) == "" {
				name = fmt.Sprintf("Part %d", i+1)
			}
			fmt.Fprintf(chapterFile, "title=%s\n", name)
			elapsed += c.Duration
		}
		_ = chapterFile.Close()
		defer os.Remove(chapterFile.Name())
	}

	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-threads", "auto",
	}
	if format == "mkv-copy" {
		args = append(args, "-fflags", "+genpts")
	}
	args = append(args,
		"-f", "concat",
		"-safe", "0",
		"-i", listFile.Name(),
	)
	if withChapters && chapterFile != nil {
		args = append(args, "-i", chapterFile.Name(), "-map_metadata", "1", "-map_chapters", "1")
	}

	// Map streams
	if keepAll {
		args = append(args, "-map", "0")
	} else {
		args = append(args, "-map", "0:v:0", "-map", "0:a:0")
	}

	// Output profile
	switch format {
	case "dvd":
		// Get DVD-specific settings from config
		dvdRegion, _ := cfg["dvdRegion"].(string)
		dvdAspect, _ := cfg["dvdAspect"].(string)
		if dvdRegion == "" {
			dvdRegion = "NTSC"
		}
		if dvdAspect == "" {
			dvdAspect = "16:9"
		}

		// Force MPEG-2 / AC-3
		// Note: Don't use -target flags as they strip metadata including chapters
		args = append(args,
			"-c:v", "mpeg2video",
			"-c:a", "ac3",
			"-b:a", "192k",
			"-max_muxing_queue_size", "1024",
		)

		if dvdRegion == "NTSC" {
			args = append(args,
				"-vf", "scale=720:480,setsar=1",
				"-r", "30000/1001",
				"-pix_fmt", "yuv420p",
				"-aspect", dvdAspect,
				"-b:v", "5000k", // DVD video bitrate
				"-maxrate", "8000k", // DVD max bitrate
				"-bufsize", "1835008", // DVD buffer size
				"-f", "dvd", // DVD format
			)
		} else {
			args = append(args,
				"-vf", "scale=720:576,setsar=1",
				"-r", "25",
				"-pix_fmt", "yuv420p",
				"-aspect", dvdAspect,
				"-b:v", "5000k", // DVD video bitrate
				"-maxrate", "8000k", // DVD max bitrate
				"-bufsize", "1835008", // DVD buffer size
				"-f", "dvd", // DVD format
			)
		}

	case "dvd-ntsc-169", "dvd-ntsc-43", "dvd-pal-169", "dvd-pal-43":
		// Legacy DVD formats for backward compatibility
		// Note: Don't use -target flags as they strip metadata including chapters
		args = append(args,
			"-c:v", "mpeg2video",
			"-c:a", "ac3",
			"-b:a", "192k",
			"-max_muxing_queue_size", "1024",
		)
		aspect := "16:9"
		if strings.Contains(format, "43") {
			aspect = "4:3"
		}
		if strings.Contains(format, "ntsc") {
			args = append(args,
				"-vf", "scale=720:480,setsar=1",
				"-r", "30000/1001",
				"-pix_fmt", "yuv420p",
				"-aspect", aspect,
				"-b:v", "5000k",
				"-maxrate", "8000k",
				"-bufsize", "1835008",
				"-f", "dvd",
			)
		} else {
			args = append(args,
				"-vf", "scale=720:576,setsar=1",
				"-r", "25",
				"-pix_fmt", "yuv420p",
				"-aspect", aspect,
				"-b:v", "5000k",
				"-maxrate", "8000k",
				"-bufsize", "1835008",
				"-f", "dvd",
			)
		}
	case "bd-h264":
		args = append(args,
			"-c:v", "libx264",
			"-preset", "slow",
			"-crf", "18",
			"-pix_fmt", "yuv420p",
			"-c:a", "ac3",
			"-b:a", "256k",
		)
	case "mkv-copy":
		args = append(args, "-c", "copy", "-avoid_negative_ts", "make_zero")
	case "mkv-h264":
		args = append(args,
			"-c:v", "libx264",
			"-preset", "medium",
			"-crf", "23",
			"-c:a", "copy",
		)
	case "mkv-h265":
		args = append(args,
			"-c:v", "libx265",
			"-preset", "medium",
			"-crf", "28",
			"-c:a", "copy",
		)
	case "mkv-lossless":
		// Lossless MKV with best quality settings
		args = append(args,
			"-c:v", "libx264",
			"-preset", "slow",
			"-crf", "18",
			"-c:a", "flac",
		)
	case "webm-vp9":
		args = append(args,
			"-c:v", "libvpx-vp9",
			"-b:v", "0",
			"-crf", "32",
			"-c:a", "libopus",
			"-b:a", "128k",
		)
	case "mp4-h264":
		args = append(args,
			"-c:v", "libx264",
			"-preset", "medium",
			"-crf", "23",
			"-c:a", "aac",
			"-b:a", "192k",
			"-movflags", "+faststart",
		)
	case "mp4-h265":
		args = append(args,
			"-c:v", "libx265",
			"-preset", "medium",
			"-crf", "28",
			"-c:a", "aac",
			"-b:a", "192k",
			"-movflags", "+faststart",
		)
	case "av1":
		args = append(args,
			"-c:v", "libaom-av1",
			"-crf", "30",
			"-b:v", "0",
			"-cpu-used", "6",
			"-c:a", "aac",
			"-b:a", "192k",
			"-movflags", "+faststart",
		)
	default:
		// Fallback to copy
		args = append(args, "-c", "copy")
	}

	// Frame rate handling (for non-DVD formats that don't lock frame rate)
	frameRate, _ := cfg["frameRate"].(string)
	useMotionInterp, _ := cfg["useMotionInterpolation"].(bool)
	if frameRate != "" && frameRate != "Source" && format != "dvd" && !strings.HasPrefix(format, "dvd-") {
		// Build frame rate filter
		var frFilter string
		if useMotionInterp {
			frFilter = fmt.Sprintf("minterpolate=fps=%s:mi_mode=mci:mc_mode=aobmc:me_mode=bidir:vsbmc=1", frameRate)
		} else {
			frFilter = "fps=" + frameRate
		}
		// Add as separate filter
		args = append(args, "-vf", frFilter)
	}

	// Add progress output for live updates (must be before output path)
	args = append(args, "-progress", "pipe:1", "-nostats")

	args = append(args, outputPath)

	// Execute
	cmd := utils.CreateCommand(ctx, utils.GetFFmpegPath(), args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("merge stdout pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if progressCallback != nil {
		progressCallback(0)
	}

	// Track total duration for progress
	var totalDur float64
	for i, c := range clips {
		if c.Duration > 0 {
			totalDur += c.Duration
			logging.Debug(logging.CatFFMPEG, "merge clip %d duration: %.2fs (path: %s)", i, c.Duration, filepath.Base(c.Path))
		}
	}
	logging.Debug(logging.CatFFMPEG, "merge total expected duration: %.2fs (%d clips)", totalDur, len(clips))

	if err := utils.StartCmd(cmd); err != nil {
		return fmt.Errorf("merge start failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}

	// Parse progress
	go func() {
		scanner := bufio.NewScanner(stdout)
		var lastPct float64
		var sampleCount int
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key, val := parts[0], parts[1]
			if key == "out_time_ms" && totalDur > 0 && progressCallback != nil {
				if ms, err := strconv.ParseFloat(val, 64); err == nil {
					// Note: out_time_ms is actually in microseconds, not milliseconds
					currentSec := ms / 1000000.0
					pct := (currentSec / totalDur) * 100

					// Log first few samples and when hitting milestones
					sampleCount++
					if sampleCount <= 5 || pct >= 25 && lastPct < 25 || pct >= 50 && lastPct < 50 || pct >= 75 && lastPct < 75 || pct >= 100 && lastPct < 100 {
						logging.Debug(logging.CatFFMPEG, "merge progress sample #%d: out_time_ms=%s (%.2fs) / total=%.2fs = %.1f%%", sampleCount, val, currentSec, totalDur, pct)
					}

					// Don't cap at 100% - let it go slightly over to avoid premature 100%
					// FFmpeg's concat can sometimes report slightly different durations
					if pct > 100 {
						pct = 100
					}

					// Only update if changed by at least 0.1%
					if pct-lastPct >= 0.1 || pct >= 100 {
						lastPct = pct
						progressCallback(pct)
					}
				}
			}
		}
	}()

	err = cmd.Wait()
	if progressCallback != nil {
		progressCallback(100)
	}
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("merge failed: %w\nFFmpeg output:\n%s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// executeConvertJob executes a conversion job from the queue
func (s *appState) executeConvertJob(ctx context.Context, job *queue.Job, progressCallback func(float64)) error {
	cfg := job.Config
	inputPath := cfg["inputPath"].(string)
	outputPath := cfg["outputPath"].(string)

	// Ensure output directory exists before attempting conversion
	if outputDir := filepath.Dir(outputPath); outputDir != "" {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// Safety: refuse to overwrite the source file
	if pathsAreSameFile(inputPath, outputPath) {
		err := fmt.Errorf("output path resolves to the same file as the input — refusing to overwrite source")
		logging.Error(logging.CatConvert, "convert job validation failed: input=%s output=%s err=%v", inputPath, outputPath, err)
		return err
	}

	// Track success to clean up broken files on failure
	var success bool
	defer func() {
		if !success && outputPath != "" && !pathsAreSameFile(inputPath, outputPath) {
			// Remove incomplete/broken output file on failure
			if _, err := os.Stat(outputPath); err == nil {
				logging.Debug(logging.CatFFMPEG, "removing incomplete output file: %s", outputPath)
				os.Remove(outputPath)
			}
		}
	}()

	// If a direct conversion is running, wait until it finishes before starting queued jobs.
	for s.convertBusy {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	// Build FFmpeg arguments
	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-threads", "auto",
	}

	// Check if this is a DVD format (special handling required)
	selectedFormat := formatOptions[0]
	switch v := cfg["selectedFormat"].(type) {
	case formatOption:
		selectedFormat = v
	case map[string]interface{}:
		if label, ok := v["Label"].(string); ok {
			selectedFormat.Label = label
		}
		if ext, ok := v["Ext"].(string); ok {
			selectedFormat.Ext = ext
		}
		if codec, ok := v["VideoCodec"].(string); ok {
			selectedFormat.VideoCodec = codec
		}
	}
	isDVD := selectedFormat.Ext == ".mpg"
	remux := strings.EqualFold(selectedFormat.VideoCodec, "copy")
	if vc, ok := cfg["videoCodec"].(string); ok && strings.EqualFold(vc, "Copy") {
		remux = true
	}

	// REMUX SAFETY: Validate compatibility and auto-fix issues
	if remux {
		src, probeErr := probeVideo(inputPath)
		if probeErr != nil {
			return fmt.Errorf("remux safety check failed - cannot probe source: %w", probeErr)
		}

		compatible, reason, autoFix := validateRemuxCompatibility(src, selectedFormat.Ext, inputPath)
		if !compatible {
			if autoFix {
				logging.Debug(logging.CatFFMPEG, "remux compatibility issue detected (auto-fixable): %s", reason)
				// Continue with remux but apply fixes below
			} else {
				logging.Debug(logging.CatFFMPEG, "remux not compatible: %s - forcing re-encode", reason)
				remux = false
				// Force to safe codec
				if selectedFormat.VideoCodec == "copy" {
					selectedFormat.VideoCodec = "libx264"
					cfg["videoCodec"] = "H.264"
				}
			}
		}
	}

	// DVD presets: enforce compliant codecs and audio settings
	// Note: We do NOT force resolution - user can choose Source or specific resolution
	if isDVD {
		if strings.Contains(selectedFormat.Label, "PAL") {
			// Only set frame rate if not already specified
			if fr, ok := cfg["frameRate"].(string); !ok || fr == "" || fr == "Source" {
				cfg["frameRate"] = "25"
			}
		} else {
			// Only set frame rate if not already specified
			if fr, ok := cfg["frameRate"].(string); !ok || fr == "" || fr == "Source" {
				cfg["frameRate"] = "29.97"
			}
		}
		cfg["videoCodec"] = "MPEG-2"
		cfg["audioCodec"] = "AC-3"
		if _, ok := cfg["audioBitrate"].(string); !ok || cfg["audioBitrate"] == "" {
			cfg["audioBitrate"] = "192k"
		}
		cfg["pixelFormat"] = "yuv420p"
	}

	// REMUX SAFETY FLAGS: Add comprehensive timestamp and compatibility fixes
	if remux {
		// Regenerate presentation timestamps to fix sync issues
		args = append(args, "-fflags", "+genpts")

		// Fix negative timestamp issues (common in AVI, FLV, MPEG-TS)
		args = append(args, "-avoid_negative_ts", "make_zero")

		// Analyze MPEG-2 and MPEG-TS more carefully for proper remuxing
		sourceExt := strings.ToLower(filepath.Ext(inputPath))
		if sourceExt == ".ts" || sourceExt == ".m2ts" || sourceExt == ".mts" {
			args = append(args, "-analyzeduration", "10000000", "-probesize", "10000000")
		}
	}

	// Hardware acceleration for decoding - MUST come BEFORE input file
	// Note: NVENC and AMF don't need -hwaccel for encoding, only for decoding
	hardwareAccel, _ := cfg["hardwareAccel"].(string)
	if hardwareAccel != "none" && hardwareAccel != "" {
		switch hardwareAccel {
		case "nvenc":
			// For NVENC, we don't add -hwaccel flags
			// The h264_nvenc/hevc_nvenc encoder handles GPU encoding directly
			// Only add hwaccel if we want GPU decoding too, which can cause issues
		case "amf":
			// For AMD AMF, we don't add -hwaccel flags
			// The h264_amf/hevc_amf/av1_amf encoders handle GPU encoding directly
		case "vaapi":
			args = append(args, "-hwaccel", "vaapi")
		case "qsv":
			args = append(args, "-hwaccel", "qsv")
		case "videotoolbox":
			args = append(args, "-hwaccel", "videotoolbox")
		}
	}

	args = append(args, "-i", inputPath)

	// Add cover art if available
	coverArtPath, _ := cfg["coverArtPath"].(string)
	hasCoverArt := coverArtPath != ""
	if isDVD {
		// DVD targets do not support attached cover art
		hasCoverArt = false
	}
	if hasCoverArt {
		args = append(args, "-i", coverArtPath)
	}

	// Source metrics (used for filters and bitrate defaults)
	sourceWidth, _ := cfg["sourceWidth"].(int)
	sourceHeight, _ := cfg["sourceHeight"].(int)
	sampleAspectRatio, _ := cfg["sampleAspectRatio"].(string)
	sourceBitrate := 0
	if v, ok := cfg["sourceBitrate"].(float64); ok {
		sourceBitrate = int(v)
	}

	// Video filters
	var vf []string
	if !remux {
		// Deinterlacing
		shouldDeinterlace := false
		deinterlaceMode, _ := cfg["deinterlace"].(string)
		fieldOrder, _ := cfg["fieldOrder"].(string)

		if deinterlaceMode == "Force" {
			shouldDeinterlace = true
		} else if deinterlaceMode == "Auto" || deinterlaceMode == "" {
			// Auto-detect based on field order
			if fieldOrder != "" && fieldOrder != "progressive" && fieldOrder != "unknown" {
				shouldDeinterlace = true
			}
		}

		// Legacy support
		if inverseTelecine, _ := cfg["inverseTelecine"].(bool); inverseTelecine {
			shouldDeinterlace = true
		}

		if shouldDeinterlace {
			// Choose deinterlacing method
			deintMethod, _ := cfg["deinterlaceMethod"].(string)
			if deintMethod == "" {
				deintMethod = "bwdif" // Default to bwdif (higher quality)
			}

			if deintMethod == "bwdif" {
				vf = append(vf, "bwdif=mode=send_frame:parity=auto")
			} else {
				vf = append(vf, "yadif=0:-1:0")
			}
		}

		// Auto-crop black bars (apply before scaling for best results)
		if autoCrop, _ := cfg["autoCrop"].(bool); autoCrop {
			cropWidth, _ := cfg["cropWidth"].(string)
			cropHeight, _ := cfg["cropHeight"].(string)
			cropX, _ := cfg["cropX"].(string)
			cropY, _ := cfg["cropY"].(string)

			if cropWidth != "" && cropHeight != "" {
				cropW := strings.TrimSpace(cropWidth)
				cropH := strings.TrimSpace(cropHeight)
				cropXStr := strings.TrimSpace(cropX)
				cropYStr := strings.TrimSpace(cropY)

				// Default to center crop if X/Y not specified
				if cropXStr == "" {
					cropXStr = "(in_w-out_w)/2"
				}
				if cropYStr == "" {
					cropYStr = "(in_h-out_h)/2"
				}

				cropFilter := fmt.Sprintf("crop=%s:%s:%s:%s", cropW, cropH, cropXStr, cropYStr)
				vf = append(vf, cropFilter)
				logging.Debug(logging.CatFFMPEG, "applying crop in queue job: %s", cropFilter)
			}
		}

		// Scaling/Resolution + Aspect ratio conversion
		targetResolution, _ := cfg["targetResolution"].(string)
		outputAspect, _ := cfg["outputAspect"].(string)
		aspectHandling, _ := cfg["aspectHandling"].(string)
		displayAspectRatio, _ := cfg["displayAspectRatio"].(string)
		forceAspect := true
		if v, ok := cfg["forceAspect"].(bool); ok {
			forceAspect = v
		}

		tempSrc := &videoSource{
			Width:              sourceWidth,
			Height:             sourceHeight,
			SampleAspectRatio:  sampleAspectRatio,
			DisplayAspectRatio: displayAspectRatio,
		}
		targetAspect := resolveTargetAspect(outputAspect, tempSrc)
		srcAspect := displayAspectRatioFromConfig(cfg)
		targetW, targetH := targetResolutionDims(targetResolution)

		aspectConversionNeeded := outputAspect != "" &&
			!strings.EqualFold(outputAspect, "source") &&
			targetAspect > 0 &&
			srcAspect > 0 &&
			!utils.RatiosApproxEqual(targetAspect, srcAspect, 0.01)

		useAspectScaling := aspectConversionNeeded && targetW > 0 && targetH > 0
		if aspectConversionNeeded {
			vf = append(vf, aspectFiltersWithTarget(targetAspect, aspectHandling, srcAspect, targetW, targetH)...)
			logging.Debug(logging.CatFFMPEG, "converting aspect ratio from %.2f to %.2f using %s mode", srcAspect, targetAspect, aspectHandling)
		}

		if targetResolution != "" && targetResolution != "Source" && !useAspectScaling {
			var scaleFilter string
			makeEven := func(v int) int {
				if v%2 != 0 {
					return v + 1
				}
				return v
			}
			switch targetResolution {
			case "720p":
				scaleFilter = "scale=-2:720"
			case "1080p":
				scaleFilter = "scale=-2:1080"
			case "1440p":
				scaleFilter = "scale=-2:1440"
			case "4K":
				scaleFilter = "scale=-2:2160"
			case "8K":
				scaleFilter = "scale=-2:4320"
			case "NTSC (720x480)":
				scaleFilter = "scale=720:480"
			case "PAL (720x540)":
				scaleFilter = "scale=720:540"
			case "PAL (720x576)":
				scaleFilter = "scale=720:576"
			case "2X (relative)":
				if sourceWidth > 0 && sourceHeight > 0 {
					w := makeEven(sourceWidth * 2)
					h := makeEven(sourceHeight * 2)
					scaleFilter = fmt.Sprintf("scale=%d:%d", w, h)
				}
			case "4X (relative)":
				if sourceWidth > 0 && sourceHeight > 0 {
					w := makeEven(sourceWidth * 4)
					h := makeEven(sourceHeight * 4)
					scaleFilter = fmt.Sprintf("scale=%d:%d", w, h)
				}
			}
			if scaleFilter != "" {
				vf = append(vf, scaleFilter)
			}
		}

		if forceAspect && targetAspect > 0 {
			if len(vf) == 0 {
				vf = append(vf, fmt.Sprintf("setdar=%.6f", targetAspect), "setsar=1")
			} else {
				vf = appendAspectMetadata(vf, targetAspect)
			}
		}

		// Flip horizontal
		flipH, _ := cfg["flipHorizontal"].(bool)
		if flipH {
			vf = append(vf, "hflip")
		}

		// Flip vertical
		flipV, _ := cfg["flipVertical"].(bool)
		if flipV {
			vf = append(vf, "vflip")
		}

		// Rotation
		rotation, _ := cfg["rotation"].(string)
		if rotation != "" && rotation != "0" {
			switch rotation {
			case "90":
				vf = append(vf, "transpose=1") // 90 degrees clockwise
			case "180":
				vf = append(vf, "transpose=1,transpose=1") // 180 degrees
			case "270":
				vf = append(vf, "transpose=2") // 90 degrees counter-clockwise (= 270 clockwise)
			}
		}

		// Frame rate
		frameRate, _ := cfg["frameRate"].(string)
		useMotionInterp, _ := cfg["useMotionInterpolation"].(bool)
		if frameRate != "" && frameRate != "Source" {
			if useMotionInterp {
				// Use motion interpolation for smooth frame rate changes
				vf = append(vf, fmt.Sprintf("minterpolate=fps=%s:mi_mode=mci:mc_mode=aobmc:me_mode=bidir:vsbmc=1", frameRate))
			} else {
				// Simple frame rate change (duplicates/drops frames)
				vf = append(vf, "fps="+frameRate)
			}
		}

		if len(vf) > 0 {
			args = append(args, "-vf", strings.Join(vf, ","))
		}

		// When the user chose "Source" frame rate, pin the output to the exact source
		// frame rate using -r. Without this, FFmpeg may silently re-timestamp frames
		// when muxing to MP4 from AVI (or other containers with imprecise timing),
		// causing the output to report a different frame rate (e.g. 30 instead of 25).
		if (frameRate == "" || frameRate == "Source") && !useMotionInterp {
			if srcFPS, ok := cfg["sourceFrameRate"].(float64); ok && srcFPS > 0 {
				// Format as a clean fraction where possible to avoid rounding artefacts.
				// For common rates (24000/1001, 30000/1001, etc.) just let FFmpeg pick
				// from the decimal; for everything else this is accurate enough.
				args = append(args, "-r", strconv.FormatFloat(srcFPS, 'f', 6, 64))
			}
		}
	}

	// Video codec
	videoCodec, _ := cfg["videoCodec"].(string)
	if friendly := friendlyCodecFromPreset(selectedFormat.VideoCodec); friendly != "" {
		if videoCodec == "" ||
			(strings.EqualFold(videoCodec, "H.264") && friendly == "H.265") ||
			(strings.EqualFold(videoCodec, "H.265") && friendly == "H.264") {
			videoCodec = friendly
			cfg["videoCodec"] = friendly
		}
	}
	if videoCodec == "Copy" && !isDVD {
		// REMUX MODE: Copy all streams safely
		args = append(args, "-c:v", "copy")

		// Map all streams to preserve everything (video, audio, subtitles, etc.)
		args = append(args, "-map", "0")

		// Preserve chapters if they exist
		args = append(args, "-map_chapters", "0")
	} else {
		// Determine the actual codec to use
		var actualCodec string
		if isDVD {
			// DVD requires MPEG-2 video
			actualCodec = "mpeg2video"
		} else {
			actualCodec = determineVideoCodec(convertConfig{
				VideoCodec:    videoCodec,
				HardwareAccel: hardwareAccel,
			})
		}
		args = append(args, "-c:v", actualCodec)

		// DVD-specific video settings
		if isDVD {
			// NTSC vs PAL settings
			if strings.Contains(selectedFormat.Label, "NTSC") {
				args = append(args, "-b:v", "6000k", "-maxrate", "9000k", "-bufsize", "1835k", "-g", "15")
			} else if strings.Contains(selectedFormat.Label, "PAL") {
				args = append(args, "-b:v", "8000k", "-maxrate", "9500k", "-bufsize", "2228k", "-g", "12")
			}
		} else {
			// Standard bitrate mode and quality for non-DVD
			bitrateMode, _ := cfg["bitrateMode"].(string)
			crfStr := ""
			if bitrateMode == "CRF" || bitrateMode == "" {
				crfStr, _ = cfg["crf"].(string)
				if crfStr == "" {
					quality, _ := cfg["quality"].(string)
					crfStr = crfForQuality(quality)
				}
				if actualCodec == "libx264" || actualCodec == "libx265" || actualCodec == "libvpx-vp9" || actualCodec == "libaom-av1" {
					args = append(args, "-crf", crfStr)
				}
			} else if bitrateMode == "CBR" {
				vb, _ := cfg["videoBitrate"].(string)
				if vb == "" {
					vb = defaultBitrate(videoCodec, sourceWidth, sourceBitrate)
				}
				// libaom-av1 does not support -minrate; use -b:v/-maxrate only
				if actualCodec == "libaom-av1" {
					args = append(args, "-b:v", vb, "-maxrate", vb, "-bufsize", vb)
				} else {
					args = append(args, "-b:v", vb, "-minrate", vb, "-maxrate", vb, "-bufsize", vb)
				}
			} else if bitrateMode == "VBR" {
				if videoBitrate, _ := cfg["videoBitrate"].(string); videoBitrate != "" {
					args = append(args, "-b:v", videoBitrate)
				}
			} else if bitrateMode == "Target Size" {
				// Calculate bitrate from target file size
				targetSizeStr, _ := cfg["targetFileSize"].(string)
				audioBitrateStr, _ := cfg["audioBitrate"].(string)
				duration, _ := cfg["sourceDuration"].(float64)

				if targetSizeStr != "" && duration > 0 {
					targetBytes, err := convert.ParseFileSize(targetSizeStr)
					if err == nil {
						// Parse audio bitrate (default to 192k if not set)
						audioBitrate := 192000
						if audioBitrateStr != "" {
							if rate, err := utils.ParseInt(strings.TrimSuffix(audioBitrateStr, "k")); err == nil {
								audioBitrate = rate * 1000
							}
						}

						// Calculate required video bitrate
						videoBitrate := convert.CalculateBitrateForTargetSize(targetBytes, duration, audioBitrate)
						videoBitrateStr := fmt.Sprintf("%dk", videoBitrate/1000)

						logging.Debug(logging.CatFFMPEG, "target size mode: %s -> video bitrate %s (audio %s)", targetSizeStr, videoBitrateStr, audioBitrateStr)
						args = append(args, "-b:v", videoBitrateStr)
					}
				}
			}

			pixelFormat, _ := cfg["pixelFormat"].(string)
			h264Profile, _ := cfg["h264Profile"].(string)

			// Encoder preset
			if encoderPreset, _ := cfg["encoderPreset"].(string); encoderPreset != "" {
				switch {
				case actualCodec == "libx264" || actualCodec == "libx265":
					args = append(args, "-preset", encoderPreset)
				case actualCodec == "libsvtav1":
					svtPreset := map[string]string{
						"veryslow": "3", "slower": "4", "slow": "5",
						"medium": "6", "fast": "8", "faster": "9",
						"veryfast": "10", "superfast": "11", "ultrafast": "12",
					}
					if p, ok := svtPreset[encoderPreset]; ok {
						args = append(args, "-preset", p)
					} else {
						args = append(args, "-preset", "8")
					}
				case actualCodec == "libaom-av1":
					// Map to cpu-used (0=slowest/best, 8=fastest)
					cpuUsed := map[string]string{
						"veryslow": "1", "slower": "2", "slow": "3",
						"medium": "4", "fast": "6", "faster": "7",
						"veryfast": "8", "superfast": "8", "ultrafast": "8",
					}
					if p, ok := cpuUsed[encoderPreset]; ok {
						args = append(args, "-cpu-used", p)
					} else {
						args = append(args, "-cpu-used", "4")
					}
				}
			} else if actualCodec == "libaom-av1" {
				// Always set a sensible cpu-used for libaom — default (1) is research-speed only
				args = append(args, "-cpu-used", "4")
			}

			// Encoder tune (libx264 / libx265 software only)
			if tune, _ := cfg["encoderTune"].(string); tune != "" && tune != "None" {
				if actualCodec == "libx264" || actualCodec == "libx265" {
					args = append(args, "-tune", strings.ToLower(tune))
				}
			}

			// Hardware encoder quality / preset
			switch actualCodec {
			case "h264_nvenc", "hevc_nvenc":
				// NVENC uses -preset p1..p7 and -rc vbr / -cq for quality
				nvencPreset := map[string]string{
					"veryslow": "p7", "slower": "p6", "slow": "p5",
					"medium": "p4", "fast": "p3", "faster": "p2",
					"veryfast": "p1", "superfast": "p1", "ultrafast": "p1",
				}
				encoderPreset, _ := cfg["encoderPreset"].(string)
				if p, ok := nvencPreset[encoderPreset]; ok {
					args = append(args, "-preset", p)
				} else {
					args = append(args, "-preset", "p4")
				}
				// Quality: map CRF value to -cq (0=auto/lossless, typical range 18-28)
				if crfStr != "" {
					args = append(args, "-rc", "vbr", "-cq", crfStr)
				}
			case "h264_amf", "hevc_amf":
				// AMF uses -quality speed|balanced|quality
				encoderPreset, _ := cfg["encoderPreset"].(string)
				amfQuality := map[string]string{
					"veryslow": "quality", "slower": "quality", "slow": "quality",
					"medium": "balanced", "fast": "speed", "faster": "speed",
					"veryfast": "speed", "superfast": "speed", "ultrafast": "speed",
				}
				if q, ok := amfQuality[encoderPreset]; ok {
					args = append(args, "-quality", q)
				} else {
					args = append(args, "-quality", "balanced")
				}
				// AMF quality control: -qp_i/-qp_p/-qp_b or -rc_mode
				if crfStr != "" {
					args = append(args, "-qp_i", crfStr, "-qp_p", crfStr, "-qp_b", crfStr)
				}
			case "h264_qsv", "hevc_qsv":
				// QSV uses -global_quality for CQ mode
				if crfStr != "" {
					args = append(args, "-global_quality", crfStr)
				}
				encoderPreset, _ := cfg["encoderPreset"].(string)
				qsvPreset := map[string]string{
					"veryslow": "veryslow", "slower": "slower", "slow": "slow",
					"medium": "medium", "fast": "fast", "faster": "faster",
					"veryfast": "veryfast",
				}
				if p, ok := qsvPreset[encoderPreset]; ok {
					args = append(args, "-preset", p)
				}
			}

			// Enforce true lossless for software HEVC when CRF is 0
			if actualCodec == "libx265" && crfStr == "0" {
				args = append(args, "-x265-params", "lossless=1")
			}

			// H.264 lossless requires High 4:4:4 profile and yuv444p pixel format
			if actualCodec == "libx264" && crfStr == "0" {
				if h264Profile == "" || strings.EqualFold(h264Profile, "auto") ||
					strings.EqualFold(h264Profile, "baseline") ||
					strings.EqualFold(h264Profile, "main") ||
					strings.EqualFold(h264Profile, "high") {
					h264Profile = "high444"
				}
				if pixelFormat == "" || strings.EqualFold(pixelFormat, "yuv420p") {
					pixelFormat = "yuv444p"
				}
			}

			// Pixel format
			if pixelFormat != "" {
				args = append(args, "-pix_fmt", pixelFormat)
			}

			// H.264 profile and level for compatibility
			if videoCodec == "H.264" && (strings.Contains(actualCodec, "264") || strings.Contains(actualCodec, "h264")) {
				if h264Profile != "" && h264Profile != "Auto" {
					// Use :v:0 if cover art is present to avoid applying to PNG stream
					if hasCoverArt {
						args = append(args, "-profile:v:0", h264Profile)
					} else {
						args = append(args, "-profile:v", h264Profile)
					}
				}
				if h264Level, _ := cfg["h264Level"].(string); h264Level != "" && h264Level != "Auto" {
					if hasCoverArt {
						args = append(args, "-level:v:0", h264Level)
					} else {
						args = append(args, "-level:v", h264Level)
					}
				}
			}
		}
	}

	// Audio codec and settings
	audioCodec, _ := cfg["audioCodec"].(string)
	isWebM := strings.EqualFold(selectedFormat.Ext, ".webm")
	if audioCodec == "Copy" && !isDVD {
		if isWebM {
			args = append(args, "-c:a", "copy")
		} else {
			args = append(args, "-c:a", "copy")
		}
	} else {
		var actualAudioCodec string
		if isDVD {
			// DVD requires AC-3 audio
			actualAudioCodec = "ac3"
		} else if isWebM {
			// WebM only supports Vorbis and Opus
			audioCodecLower := strings.ToLower(audioCodec)
			if audioCodecLower == "aac" || audioCodecLower == "" {
				actualAudioCodec = "libopus"
			} else if audioCodecLower == "mp3" || audioCodecLower == "ac-3" || audioCodecLower == "flac" {
				actualAudioCodec = "libopus"
			} else {
				actualAudioCodec = determineAudioCodec(convertConfig{AudioCodec: audioCodec})
			}
		} else {
			actualAudioCodec = determineAudioCodec(convertConfig{AudioCodec: audioCodec})
		}
		args = append(args, "-c:a", actualAudioCodec)

		// DVD-specific audio settings
		if isDVD {
			// DVD standard: AC-3 stereo at 48 kHz, 192 kbps
			args = append(args, "-b:a", "192k", "-ar", "48000", "-ac", "2")
		} else {
			// Standard audio settings for non-DVD
			if audioBitrate, _ := cfg["audioBitrate"].(string); audioBitrate != "" && actualAudioCodec != "flac" {
				args = append(args, "-b:a", audioBitrate)
			}

			// Audio normalization (compatibility mode)
			if normalizeAudio, _ := cfg["normalizeAudio"].(bool); normalizeAudio {
				args = append(args, "-ac", "2", "-ar", "48000")
			} else {
				if audioChannels, _ := cfg["audioChannels"].(string); audioChannels != "" && audioChannels != "Source" {
					switch audioChannels {
					case "Mono":
						args = append(args, "-ac", "1")
					case "Stereo":
						args = append(args, "-ac", "2")
					case "5.1":
						args = append(args, "-ac", "6")
					case "Left to Stereo":
						// Copy left channel to both left and right
						args = append(args, "-af", "pan=stereo|c0=c0|c1=c0")
					case "Right to Stereo":
						// Copy right channel to both left and right
						args = append(args, "-af", "pan=stereo|c0=c1|c1=c1")
					case "Mix to Stereo":
						// Downmix both channels together, then duplicate to L+R
						args = append(args, "-af", "pan=stereo|c0=0.5*c0+0.5*c1|c1=0.5*c0+0.5*c1")
					case "Swap L/R":
						// Swap left and right channels
						args = append(args, "-af", "pan=stereo|c0=c1|c1=c0")
					}
				}

				if audioSampleRate, _ := cfg["audioSampleRate"].(string); audioSampleRate != "" && audioSampleRate != "Source" {
					args = append(args, "-ar", audioSampleRate)
				}
			}
		}
	}

	// Map streams and metadata
	if hasCoverArt {
		// With cover art: map video, audio, subtitles, and cover art
		args = append(args, "-map", "0:v", "-map", "0:a?", "-map", "0:s?", "-map", "1:v")
		args = append(args, "-c:v:1", "png")
		args = append(args, "-disposition:v:1", "attached_pic")
	} else {
		// Without cover art: map video, audio, and subtitles
		args = append(args, "-map", "0:v", "-map", "0:a?", "-map", "0:s?")
	}

	// Preserve chapters and metadata
	preserveChapters := true
	if v, ok := cfg["preserveChapters"].(bool); ok {
		preserveChapters = v
	}
	if preserveChapters {
		args = append(args, "-map_chapters", "0")
	} else {
		args = append(args, "-map_chapters", "-1")
	}
	args = append(args, "-map_metadata", "0")

	// Copy subtitle streams by default (don't re-encode)
	args = append(args, "-c:s", "copy")

	if strings.EqualFold(selectedFormat.Ext, ".mp4") || strings.EqualFold(selectedFormat.Ext, ".mov") {
		args = append(args, "-movflags", "+faststart")
	}

	// Note: We no longer use -target because it forces resolution changes.
	// DVD-specific parameters are set manually in the video codec section below.

	// Fix VFR/desync issues - regenerate timestamps and enforce CFR
	if !remux {
		args = append(args, "-fflags", "+genpts")
		frameRateStr, _ := cfg["frameRate"].(string)
		sourceDuration, _ := cfg["sourceDuration"].(float64)
		if frameRateStr != "" && frameRateStr != "Source" {
			args = append(args, "-r", frameRateStr)
		} else if sourceDuration > 0 {
			// Calculate approximate source frame rate if available
			args = append(args, "-r", "30") // Safe default
		}
	} else {
		args = append(args, "-avoid_negative_ts", "make_zero")
	}

	// Inject BPS tag for MKV output so Windows Explorer and media tools show the
	// correct bitrate. Hardware encoders (AMF, NVENC) do not write per-stream stats
	// back to Matroska tag writers, leaving BPS as 0.
	if strings.EqualFold(selectedFormat.Ext, ".mkv") && !remux {
		bitrateMode, _ := cfg["bitrateMode"].(string)
		if bitrateMode == "CBR" || bitrateMode == "VBR" {
			vb, _ := cfg["videoBitrate"].(string)
			if vb == "" {
				vb = defaultBitrate(videoCodec, sourceWidth, sourceBitrate)
			}
			if bps := parseBitrateStringToBPS(vb); bps > 0 {
				args = append(args, "-metadata:s:v:0", fmt.Sprintf("BPS=%d", bps))
			}
		}
	}

	// Progress feed
	args = append(args, "-progress", "pipe:1", "-nostats")
	args = append(args, outputPath)

	logFile, logPath, logErr := createConversionLog(inputPath, outputPath, args)
	if logErr != nil {
		logging.Debug(logging.CatFFMPEG, "conversion log open failed: %v", logErr)
	} else {
		job.LogPath = logPath
		fmt.Fprintf(logFile, "Status: started\n\n")
		defer logFile.Close()
	}

	logging.Debug(logging.CatFFMPEG, "queue convert command: ffmpeg %s", strings.Join(args, " "))

	// Also print to stdout for debugging
	fmt.Printf("\n=== FFMPEG COMMAND ===\nffmpeg %s\n======================\n\n", strings.Join(args, " "))

	// Execute FFmpeg
	cmd := utils.CreateCommand(ctx, utils.GetFFmpegPath(), args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Capture stderr for error messages
	var stderrBuf strings.Builder
	if logFile != nil {
		cmd.Stderr = io.MultiWriter(&stderrBuf, logFile)
	} else {
		cmd.Stderr = &stderrBuf
	}

	if err := utils.StartCmd(cmd); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Parse progress
	stdoutReader := io.Reader(stdout)
	if logFile != nil {
		stdoutReader = io.TeeReader(stdout, logFile)
	}
	scanner := bufio.NewScanner(stdoutReader)
	var duration float64
	if d, ok := cfg["sourceDuration"].(float64); ok && d > 0 {
		duration = d
	}

	started := time.Now()
	var currentFPS float64
	var currentSpeed float64
	var currentETA time.Duration

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]

		// Capture FPS value
		if key == "fps" {
			if fps, err := strconv.ParseFloat(val, 64); err == nil {
				currentFPS = fps
			}
			continue
		}

		// Capture speed value
		if key == "speed" {
			// Speed comes as "1.5x" format, strip the 'x'
			speedStr := strings.TrimSuffix(val, "x")
			if speed, err := strconv.ParseFloat(speedStr, 64); err == nil {
				currentSpeed = speed
			}
			continue
		}

		if key == "out_time_ms" {
			if ms, err := strconv.ParseInt(val, 10, 64); err == nil && ms > 0 {
				currentSec := float64(ms) / 1000000.0
				if duration > 0 {
					progress := (currentSec / duration) * 100.0
					if progress > 100 {
						progress = 100
					}

					// Calculate ETA
					elapsedWall := time.Since(started).Seconds()
					if progress > 0 && elapsedWall > 0 && progress < 100 {
						remaining := elapsedWall * (100 - progress) / progress
						currentETA = time.Duration(remaining * float64(time.Second))
					}

					// Calculate speed if not provided by ffmpeg
					if currentSpeed == 0 && elapsedWall > 0 {
						currentSpeed = currentSec / elapsedWall
					}

					// Update job config with detailed stats for the stats bar to display
					job.Config["fps"] = currentFPS
					job.Config["speed"] = currentSpeed
					job.Config["eta"] = currentETA

					progressCallback(progress)
				}
			}
		} else if key == "duration_ms" {
			if ms, err := strconv.ParseInt(val, 10, 64); err == nil && ms > 0 {
				duration = float64(ms) / 1000000.0
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		// Check context first — if cancelled, return cancellation error immediately
		// so queue.processJobs() sees context.Canceled and marks the job as Cancelled.
		if ctx.Err() != nil {
			return ctx.Err()
		}

		stderrOutput := stderrBuf.String()
		errorExplanation := interpretFFmpegError(err)

		// Check if this is a hardware encoding failure
		isHardwareFailure := strings.Contains(stderrOutput, "No capable devices found") ||
			strings.Contains(stderrOutput, "Cannot load") ||
			strings.Contains(stderrOutput, "not available") &&
				(strings.Contains(stderrOutput, "nvenc") ||
					strings.Contains(stderrOutput, "amf") ||
					strings.Contains(stderrOutput, "qsv") ||
					strings.Contains(stderrOutput, "vaapi") ||
					strings.Contains(stderrOutput, "videotoolbox"))

		if isHardwareFailure && hardwareAccel != "none" && hardwareAccel != "" {
			logging.Debug(logging.CatFFMPEG, "hardware encoding failed, will suggest software fallback")
			return fmt.Errorf("hardware encoding (%s) failed - no compatible hardware found\n\nPlease disable hardware acceleration in the conversion settings and try again with software encoding.\n\nFFmpeg output:\n%s", hardwareAccel, stderrOutput)
		}

		var errorMsg string
		if errorExplanation != "" {
			errorMsg = fmt.Sprintf("ffmpeg failed: %v - %s", err, errorExplanation)
		} else {
			errorMsg = fmt.Sprintf("ffmpeg failed: %v", err)
		}

		if stderrOutput != "" {
			logging.Debug(logging.CatFFMPEG, "ffmpeg stderr: %s", stderrOutput)
			return fmt.Errorf("%s\n\nFFmpeg output:\n%s", errorMsg, stderrOutput)
		}
		return fmt.Errorf("%s", errorMsg)
	}

	if logFile != nil {
		fmt.Fprintf(logFile, "\nStatus: completed OK at %s\n", time.Now().Format(time.RFC3339))
	}
	logging.Debug(logging.CatFFMPEG, "queue conversion completed: %s", outputPath)

	// Auto-compare if enabled
	if autoCompare, ok := cfg["autoCompare"].(bool); ok && autoCompare {
		inputPath := cfg["inputPath"].(string)

		// Probe both original and converted files
		go func() {
			originalSrc, err1 := probeVideo(inputPath)
			convertedSrc, err2 := probeVideo(outputPath)

			if err1 != nil || err2 != nil {
				logging.Debug(logging.CatModule, "auto-compare: failed to probe files: original=%v, converted=%v", err1, err2)
				return
			}

			// Load into compare slots
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				s.compareFile1 = originalSrc  // Original
				s.compareFile2 = convertedSrc // Converted
				s.showCompareView()
				logging.Debug(logging.CatModule, "auto-compare from queue: loaded original vs converted")
			}, false)
		}()
	}

	// Mark as successful to prevent cleanup of output file
	success = true
	return nil
}

func (s *appState) executeSnippetJob(ctx context.Context, job *queue.Job, progressCallback func(float64)) error {
	cfg := job.Config
	inputPath := cfg["inputPath"].(string)
	outputPath := cfg["outputPath"].(string)

	// Get snippet length from config, default to 20 if not present
	snippetLength := 20
	if lengthVal, ok := cfg["snippetLength"].(float64); ok {
		snippetLength = int(lengthVal)
	}

	// Get snippet mode, default to source format (true)
	useSourceFormat := true
	if modeVal, ok := cfg["useSourceFormat"].(bool); ok {
		useSourceFormat = modeVal
	}

	// Get output extension to check for WebM
	outputExt, _ := cfg["outputExt"].(string)
	isWebM := strings.EqualFold(outputExt, ".webm")

	// Get start position: -1 means use midpoint, otherwise use the provided position
	startPosition := -1.0
	if posVal, ok := cfg["startPosition"].(float64); ok {
		startPosition = posVal
	}

	// Probe video to get duration
	src, err := probeVideo(inputPath)
	if err != nil {
		return err
	}

	// Calculate start time
	var start string
	if startPosition >= 0 {
		// Use explicit start position (from current playback)
		start = fmt.Sprintf("%.2f", startPosition)
	} else {
		// Default: center on midpoint
		halfLength := float64(snippetLength) / 2.0
		center := math.Max(0, src.Duration/2-halfLength)
		start = fmt.Sprintf("%.2f", center)
	}

	clampSnippetBitrate := func(bitrate string, width int) string {
		val := strings.TrimSpace(strings.ToLower(bitrate))
		val = strings.TrimSuffix(val, "bps")
		val = strings.TrimSuffix(val, "k")
		n, err := strconv.ParseFloat(val, 64)
		if err != nil || n <= 0 {
			n = 3500
		}
		capKbps := 5000.0
		if width >= 3840 {
			capKbps = 30000
		} else if width >= 1920 {
			capKbps = 15000
		} else if width >= 1280 {
			capKbps = 8000
		}
		if n > capKbps {
			n = capKbps
		}
		if n < 800 {
			n = 800
		}
		return fmt.Sprintf("%.0fk", n)
	}

	var args []string

	if useSourceFormat {
		// Source Format mode: Re-encode matching source format for PRECISE duration
		conv := s.convert
		isWMV := strings.HasSuffix(strings.ToLower(inputPath), ".wmv")

		args = []string{
			"-ss", start,
			"-i", inputPath,
			"-t", fmt.Sprintf("%d", snippetLength),
		}

		// Handle WMV files specially - use wmv2 encoder
		if isWMV {
			args = append(args, "-c:v", "wmv2")
			args = append(args, "-b:v", "2000k") // High quality bitrate for WMV
			// WebM doesn't support WMA audio - use Opus
			if isWebM {
				args = append(args, "-c:a", "libopus", "-b:a", "128k")
			} else {
				args = append(args, "-c:a", "wmav2")
				if conv.AudioBitrate != "" {
					args = append(args, "-b:a", conv.AudioBitrate)
				} else {
					args = append(args, "-b:a", "192k")
				}
			}
		} else {
			// For non-WMV: match source codec where possible, but cap bitrate for snippets
			videoCodec := strings.ToLower(strings.TrimSpace(src.VideoCodec))
			switch {
			case strings.Contains(videoCodec, "264"):
				videoCodec = "libx264"
			case strings.Contains(videoCodec, "265"), strings.Contains(videoCodec, "hevc"):
				videoCodec = "libx265"
			case strings.Contains(videoCodec, "vp9"):
				videoCodec = "libvpx-vp9"
			case strings.Contains(videoCodec, "av1"):
				resolved, ok := resolveAV1Encoder("none")
				if !ok {
					logging.Debug(logging.CatFFMPEG, "AV1 encoder unavailable for snippet; falling back to %s", resolved)
				}
				videoCodec = resolved
			default:
				videoCodec = "libx264"
			}

			// WebM only supports VP8, VP9, and AV1 - fallback if needed
			if isWebM && (videoCodec == "libx264" || videoCodec == "libx265") {
				videoCodec = "libvpx-vp9"
			}

			args = append(args, "-c:v", videoCodec)

			preset := conv.EncoderPreset
			if preset == "" {
				preset = "slow"
			}

			crfVal := conv.CRF
			if crfVal == "" {
				crfVal = "18"
			}
			if strings.TrimSpace(crfVal) == "0" {
				crfVal = "18"
			}

			targetBitrate := clampSnippetBitrate(strings.TrimSpace(conv.VideoBitrate), src.Width)
			if targetBitrate == "" {
				targetBitrate = clampSnippetBitrate(defaultBitrate(conv.VideoCodec, src.Width, src.Bitrate), src.Width)
			}
			if targetBitrate == "" {
				targetBitrate = clampSnippetBitrate("3500k", src.Width)
			}

			if strings.Contains(videoCodec, "x264") || strings.Contains(videoCodec, "x265") {
				args = append(args, "-preset", preset, "-crf", crfVal, "-maxrate", targetBitrate, "-bufsize", targetBitrate)
			} else if strings.Contains(videoCodec, "vp9") {
				args = append(args, "-b:v", "0", "-crf", crfVal, "-maxrate", targetBitrate, "-bufsize", targetBitrate)
			} else if strings.Contains(videoCodec, "svtav1") {
				// Map x264/x265 presets to SVT-AV1 presets (0-13, lower=slower/better)
				var svtPreset string
				switch preset {
				case "veryslow":
					svtPreset = "3"
				case "slower":
					svtPreset = "4"
				case "slow":
					svtPreset = "5"
				case "medium":
					svtPreset = "6"
				case "fast":
					svtPreset = "8"
				case "faster":
					svtPreset = "9"
				case "veryfast":
					svtPreset = "10"
				case "superfast":
					svtPreset = "11"
				case "ultrafast":
					svtPreset = "12"
				default:
					svtPreset = "8" // Fast preset for snippets
				}
				args = append(args, "-preset", svtPreset, "-crf", crfVal, "-b:v", "0", "-maxrate", targetBitrate, "-bufsize", targetBitrate)
			} else if strings.Contains(videoCodec, "av1") {
				args = append(args, "-b:v", "0", "-crf", crfVal, "-maxrate", targetBitrate, "-bufsize", targetBitrate)
			}

			// Audio codec
			audioCodec := src.AudioCodec
			if audioCodec == "" || strings.Contains(strings.ToLower(audioCodec), "wmav") {
				audioCodec = "aac"
			}
			// WebM only supports Vorbis and Opus
			if isWebM {
				audioCodec = "libopus"
			}

			args = append(args, "-c:a", audioCodec)
			// For AAC/MP3 use bitrate; for Opus/WebM use appropriate bitrate
			if strings.Contains(strings.ToLower(audioCodec), "aac") ||
				strings.Contains(strings.ToLower(audioCodec), "mp3") {
				if conv.AudioBitrate != "" {
					args = append(args, "-b:a", conv.AudioBitrate)
				} else {
					args = append(args, "-b:a", "192k")
				}
			} else if isWebM {
				// Opus for WebM
				args = append(args, "-b:a", "128k")
			}
		}

		args = append(args, "-y", "-hide_banner", "-loglevel", "error", "-threads", "auto")
	} else {
		// Conversion format mode: Use configured conversion settings
		// This allows previewing what the final converted output will look like
		conv := s.convert

		args = []string{
			"-y",
			"-hide_banner",
			"-loglevel", "error",
			"-threads", "auto",
			"-ss", start,
			"-i", inputPath,
			"-t", fmt.Sprintf("%d", snippetLength),
		}

		// Apply video codec settings with bitrate/CRF caps to avoid runaway bitrates on short clips
		targetBitrate := clampSnippetBitrate(strings.TrimSpace(conv.VideoBitrate), src.Width)
		if targetBitrate == "" {
			targetBitrate = clampSnippetBitrate(defaultBitrate(conv.VideoCodec, src.Width, src.Bitrate), src.Width)
		}
		if targetBitrate == "" {
			targetBitrate = clampSnippetBitrate("3500k", src.Width)
		}

		preset := conv.EncoderPreset
		if preset == "" {
			preset = "medium"
		}

		crfVal := conv.CRF
		if crfVal == "" {
			crfVal = crfForQuality(conv.Quality)
			if crfVal == "" {
				crfVal = "23"
			}
		}
		// Disallow lossless for snippets to avoid runaway bitrates
		if strings.TrimSpace(crfVal) == "0" {
			crfVal = "18"
		}

		videoCodec := strings.ToLower(conv.VideoCodec)
		// WebM only supports VP8, VP9, and AV1 - fallback if needed
		if isWebM && (videoCodec == "h.264" || videoCodec == "h.265" || videoCodec == "") {
			videoCodec = "vp9"
		}
		switch videoCodec {
		case "h.264", "":
			args = append(args, "-c:v", "libx264")
			args = append(args, "-preset", preset, "-crf", crfVal, "-maxrate", targetBitrate, "-bufsize", targetBitrate)
		case "h.265":
			args = append(args, "-c:v", "libx265")
			args = append(args, "-preset", preset, "-crf", crfVal, "-maxrate", targetBitrate, "-bufsize", targetBitrate)
		case "vp9":
			args = append(args, "-c:v", "libvpx-vp9")
			args = append(args, "-crf", crfVal, "-maxrate", targetBitrate, "-bufsize", targetBitrate)
		case "av1":
			resolved, ok := resolveAV1Encoder("none")
			if !ok {
				logging.Debug(logging.CatFFMPEG, "AV1 encoder unavailable for snippet conversion; falling back to %s", resolved)
			}
			args = append(args, "-c:v", resolved)
			if resolved == "libsvtav1" {
				// Map x264/x265 presets to SVT-AV1 presets (0-13, lower=slower/better)
				var svtPreset string
				switch preset {
				case "veryslow":
					svtPreset = "3"
				case "slower":
					svtPreset = "4"
				case "slow":
					svtPreset = "5"
				case "medium":
					svtPreset = "6"
				case "fast":
					svtPreset = "8"
				case "faster":
					svtPreset = "9"
				case "veryfast":
					svtPreset = "10"
				case "superfast":
					svtPreset = "11"
				case "ultrafast":
					svtPreset = "12"
				default:
					svtPreset = "8" // Fast preset for snippets
				}
				args = append(args, "-preset", svtPreset)
			} else if resolved == "libx264" || resolved == "libx265" {
				args = append(args, "-preset", preset)
			}
			args = append(args, "-crf", crfVal, "-maxrate", targetBitrate, "-bufsize", targetBitrate)
		case "copy":
			args = append(args, "-c:v", "copy")
		default:
			// Fallback to h264
			args = append(args, "-c:v", "libx264", "-preset", preset, "-crf", crfVal, "-maxrate", targetBitrate, "-bufsize", targetBitrate)
		}
		// Ensure standard pixel format
		args = append(args, "-pix_fmt", "yuv420p")

		// Apply audio codec settings
		audioCodec := strings.ToLower(conv.AudioCodec)
		// WebM only supports Vorbis and Opus
		if isWebM && (audioCodec == "" || audioCodec == "aac" || audioCodec == "mp3") {
			audioCodec = "opus"
		}
		switch audioCodec {
		case "aac", "":
			args = append(args, "-c:a", "aac")
			if conv.AudioBitrate != "" {
				args = append(args, "-b:a", conv.AudioBitrate)
			} else {
				args = append(args, "-b:a", "192k")
			}
		case "opus":
			args = append(args, "-c:a", "libopus")
			if conv.AudioBitrate != "" {
				args = append(args, "-b:a", conv.AudioBitrate)
			} else {
				args = append(args, "-b:a", "128k")
			}
		case "vorbis":
			args = append(args, "-c:a", "libvorbis")
			if conv.AudioBitrate != "" {
				args = append(args, "-b:a", conv.AudioBitrate)
			} else {
				args = append(args, "-b:a", "128k")
			}
		case "mp3":
			args = append(args, "-c:a", "libmp3lame")
			if conv.AudioBitrate != "" {
				args = append(args, "-b:a", conv.AudioBitrate)
			} else {
				args = append(args, "-b:a", "192k")
			}
		case "flac":
			args = append(args, "-c:a", "flac")
		case "copy":
			args = append(args, "-c:a", "copy")
		default:
			// Fallback to AAC (or Opus for WebM)
			if isWebM {
				args = append(args, "-c:a", "libopus", "-b:a", "128k")
			} else {
				args = append(args, "-c:a", "aac", "-b:a", "192k")
			}
		}

		// Common args appended after progress flags
	}

	// Add progress output for live updates (stdout) and finish with output path
	args = append(args, "-progress", "pipe:1", "-nostats", outputPath)

	if progressCallback != nil {
		progressCallback(0)
	}

	logFile, logPath, _ := createConversionLog(inputPath, outputPath, args)

	cmd := utils.CreateCommand(ctx, utils.GetFFmpegPath(), args...)

	stdout, err := cmd.StdoutPipe()

	if err != nil {

		return fmt.Errorf("snippet stdout pipe: %w", err)

	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := utils.StartCmd(cmd); err != nil {
		return fmt.Errorf("snippet start failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}

	// Track progress based on snippet length
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if logFile != nil {
				fmt.Fprintln(logFile, line)
			}
			if strings.HasPrefix(line, "out_time_ms=") && snippetLength > 0 {
				val := strings.TrimPrefix(line, "out_time_ms=")
				if ms, err := strconv.ParseFloat(val, 64); err == nil {
					currentSec := ms / 1_000_000.0
					pct := (currentSec / float64(snippetLength)) * 100.0
					if pct > 100 {
						pct = 100
					}
					if progressCallback != nil {
						progressCallback(pct)
					}
				}
			}
		}
	}()

	err = cmd.Wait()
	if err != nil {
		if logFile != nil {
			fmt.Fprintf(logFile, "\nStatus: failed at %s\nError: %v\nFFmpeg stderr:\n%s\n", time.Now().Format(time.RFC3339), err, strings.TrimSpace(stderr.String()))
			_ = logFile.Close()
		}
		return fmt.Errorf("snippet failed: %w\nFFmpeg stderr:\n%s", err, strings.TrimSpace(stderr.String()))
	}
	if logFile != nil {
		fmt.Fprintf(logFile, "\nStatus: completed at %s\n", time.Now().Format(time.RFC3339))
		_ = logFile.Close()
		job.LogPath = logPath
	}
	if progressCallback != nil {
		progressCallback(100)
	}
	return nil
}

func (s *appState) executeUpscaleJob(ctx context.Context, job *queue.Job, progressCallback func(float64)) error {
	cfg := job.Config
	inputPath := cfg["inputPath"].(string)
	outputPath := cfg["outputPath"].(string)
	method := cfg["method"].(string)
	targetWidth := int(cfg["targetWidth"].(float64))
	targetHeight := int(cfg["targetHeight"].(float64))
	targetPreset, _ := cfg["targetPreset"].(string)
	sourceWidth := int(toFloat(cfg["sourceWidth"]))

	// Ensure output directory exists before attempting upscale
	if outputDir := filepath.Dir(outputPath); outputDir != "" {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}
	sourceHeight := int(toFloat(cfg["sourceHeight"]))
	preserveAR := true
	if v, ok := cfg["preserveAR"].(bool); ok {
		preserveAR = v
	}
	useAI := false
	if v, ok := cfg["useAI"].(bool); ok {
		useAI = v
	}
	aiBackend, _ := cfg["aiBackend"].(string)
	aiModel, _ := cfg["aiModel"].(string)
	aiScale := toFloat(cfg["aiScale"])
	aiScaleUseTarget, _ := cfg["aiScaleUseTarget"].(bool)
	aiOutputAdjust := toFloat(cfg["aiOutputAdjust"])
	aiFaceEnhance, _ := cfg["aiFaceEnhance"].(bool)
	aiDenoise := toFloat(cfg["aiDenoise"])
	aiTile := int(toFloat(cfg["aiTile"]))
	aiGPU := int(toFloat(cfg["aiGPU"]))
	aiGPUAuto, _ := cfg["aiGPUAuto"].(bool)
	aiThreadsLoad := int(toFloat(cfg["aiThreadsLoad"]))
	aiThreadsProc := int(toFloat(cfg["aiThreadsProc"]))
	aiThreadsSave := int(toFloat(cfg["aiThreadsSave"]))
	aiTTA, _ := cfg["aiTTA"].(bool)
	aiOutputFormat, _ := cfg["aiOutputFormat"].(string)
	applyFilters := cfg["applyFilters"].(bool)
	frameRate, _ := cfg["frameRate"].(string)
	useMotionInterp, _ := cfg["useMotionInterpolation"].(bool)
	sourceFrameRate := toFloat(cfg["sourceFrameRate"])
	qualityPreset, _ := cfg["qualityPreset"].(string)
	encoderPreset, _ := cfg["encoderPreset"].(string)
	videoCodec, _ := cfg["videoCodec"].(string)
	bitrateMode, _ := cfg["bitrateMode"].(string)
	bitratePreset, _ := cfg["bitratePreset"].(string)
	manualBitrate, _ := cfg["manualBitrate"].(string)
	blurEnabled, _ := cfg["blurEnabled"].(bool)
	blurSigma := toFloat(cfg["blurSigma"])
	useRIFE := false
	if v, ok := cfg["useRIFE"].(bool); ok {
		useRIFE = v
	}
	rifeModel, _ := cfg["rifeModel"].(string)
	if rifeModel == "" {
		rifeModel = "rife-v4.6"
	}
	rifeMultiplier := 2
	if v := int(toFloat(cfg["rifeMultiplier"])); v > 0 {
		rifeMultiplier = v
	}

	// Output & colour accuracy settings
	hardwareAccel, _ := cfg["hardwareAccel"].(string)
	outputContainer, _ := cfg["outputContainer"].(string)
	if outputContainer == "" {
		outputContainer = "mp4"
	}
	manualCRF := int(toFloat(cfg["manualCRF"]))
	pixelFormat, _ := cfg["pixelFormat"].(string)
	if pixelFormat == "" {
		pixelFormat = "yuv420p"
	}
	srcColorSpace, _ := cfg["srcColorSpace"].(string)
	colorDepth, _ := cfg["colorDepth"].(string)
	skinTone, _ := cfg["skinTone"].(string)

	if progressCallback != nil {
		progressCallback(0)
	}

	// Recompute target dimensions from preset to avoid stale values
	if targetPreset != "" && targetPreset != "Custom" {
		if sourceWidth <= 0 || sourceHeight <= 0 {
			if src, err := probeVideo(inputPath); err == nil && src != nil {
				sourceWidth = src.Width
				sourceHeight = src.Height
			}
		}
		if w, h, keepAR, err := parseResolutionPreset(targetPreset, sourceWidth, sourceHeight); err == nil {
			targetWidth = w
			targetHeight = h
			preserveAR = keepAR
		}
	}

	// manualCRF takes precedence over quality presets when non-zero.
	crfValue := 16
	if manualCRF > 0 || manualCRF == 0 && qualityPreset == "Lossless (CRF 0)" {
		crfValue = manualCRF
	} else {
		switch qualityPreset {
		case "Lossless (CRF 0)":
			crfValue = 0
		case "High (CRF 18)":
			crfValue = 18
		case "Near-lossless (CRF 16)":
			crfValue = 16
		}
	}

	resolveBitrate := func() string {
		if strings.TrimSpace(manualBitrate) != "" {
			return manualBitrate
		}
		switch bitratePreset {
		case "0.5 Mbps - Ultra Low":
			return "500k"
		case "1.0 Mbps - Very Low":
			return "1000k"
		case "1.5 Mbps - Low":
			return "1500k"
		case "2.0 Mbps - Medium-Low":
			return "2000k"
		case "2.5 Mbps - Medium":
			return "2500k"
		case "4.0 Mbps - Good":
			return "4000k"
		case "6.0 Mbps - High":
			return "6000k"
		case "8.0 Mbps - Very High":
			return "8000k"
		default:
			return "2500k"
		}
	}

	parseBitrateKbps := func(val string) int {
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "" {
			return 0
		}
		mult := 1.0
		if strings.HasSuffix(v, "k") {
			v = strings.TrimSuffix(v, "k")
			mult = 1
		} else if strings.HasSuffix(v, "m") {
			v = strings.TrimSuffix(v, "m")
			mult = 1000
		}
		num, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0
		}
		return int(num * mult)
	}

	normalizeBitrateMode := func(mode string) string {
		switch {
		case strings.HasPrefix(strings.ToUpper(mode), "CBR"):
			return "CBR"
		case strings.HasPrefix(strings.ToUpper(mode), "VBR"):
			return "VBR"
		default:
			return "CRF"
		}
	}

	// Resolve video codec encoder with optional hardware acceleration.
	resolveVideoCodec := func(codec, accel string) string {
		accel = strings.ToLower(strings.TrimSpace(accel))
		switch strings.ToLower(codec) {
		case "h.264", "":
			switch accel {
			case "nvenc":
				return "h264_nvenc"
			case "vaapi":
				return "h264_vaapi"
			case "qsv":
				return "h264_qsv"
			case "videotoolbox":
				return "h264_videotoolbox"
			case "auto":
				if resolved, ok := resolveH264HWEncoder(); ok {
					return resolved
				}
			}
			return "libx264"
		case "h.265":
			switch accel {
			case "nvenc":
				return "hevc_nvenc"
			case "vaapi":
				return "hevc_vaapi"
			case "qsv":
				return "hevc_qsv"
			case "videotoolbox":
				return "hevc_videotoolbox"
			case "auto":
				if resolved, ok := resolveHEVCHWEncoder(); ok {
					return resolved
				}
			}
			return "libx265"
		case "vp9":
			return "libvpx-vp9"
		case "av1":
			if resolved, ok := resolveAV1Encoder("none"); ok {
				return resolved
			}
			return "libx264"
		case "copy":
			return "copy"
		default:
			return "libx264"
		}
	}
	videoEncoder := resolveVideoCodec(videoCodec, hardwareAccel)

	appendEncodingArgs := func(args []string) []string {
		preset := encoderPreset
		if strings.TrimSpace(preset) == "" {
			preset = "slow"
		}
		mode := normalizeBitrateMode(bitrateMode)

		args = append(args, "-preset", preset)
		switch mode {
		case "CBR", "VBR":
			bitrateVal := resolveBitrate()
			args = append(args, "-b:v", bitrateVal)
			kbps := parseBitrateKbps(bitrateVal)
			if kbps > 0 {
				maxrate := kbps
				bufsize := kbps * 2
				if mode == "VBR" {
					maxrate = kbps * 2
					bufsize = kbps * 4
				}
				args = append(args, "-maxrate", fmt.Sprintf("%dk", maxrate))
				args = append(args, "-bufsize", fmt.Sprintf("%dk", bufsize))
			}
		case "AV1":
			// AV1 uses different preset mapping
			args = append(args, "-crf", strconv.Itoa(crfValue))
			// Map x264 presets to SVT-AV1 presets (0-13, lower=slower/better)
			var svtPreset string
			switch preset {
			case "veryslow":
				svtPreset = "3"
			case "slower":
				svtPreset = "4"
			case "slow":
				svtPreset = "5"
			case "medium":
				svtPreset = "6"
			case "fast":
				svtPreset = "8"
			case "faster":
				svtPreset = "9"
			case "veryfast":
				svtPreset = "10"
			case "superfast":
				svtPreset = "11"
			case "ultrafast":
				svtPreset = "12"
			default:
				svtPreset = "8"
			}
			if strings.HasPrefix(videoEncoder, "libsvtav1") {
				args = append(args, "-preset", svtPreset)
			}
		default:
			args = append(args, "-crf", strconv.Itoa(crfValue))
		}
		return args
	}

	// Build filter chain
	var baseFilters []string

	// Add filters from Filters module if requested
	if applyFilters {
		if filterChain, ok := cfg["filterChain"].([]interface{}); ok {
			for _, f := range filterChain {
				if filterStr, ok := f.(string); ok {
					baseFilters = append(baseFilters, filterStr)
				}
			}
		} else if filterChain, ok := cfg["filterChain"].([]string); ok {
			baseFilters = append(baseFilters, filterChain...)
		}
	}

	// Add frame rate conversion if requested
	if frameRate != "" && frameRate != "Source" {
		if useMotionInterp {
			// Use motion interpolation for smooth frame rate changes
			baseFilters = append(baseFilters, fmt.Sprintf("minterpolate=fps=%s:mi_mode=mci:mc_mode=aobmc:me_mode=bidir:vsbmc=1", frameRate))
		} else {
			// Simple frame rate change (duplicates/drops frames)
			baseFilters = append(baseFilters, "fps="+frameRate)
		}
	}

	if blurEnabled && blurSigma > 0 {
		baseFilters = append(baseFilters, fmt.Sprintf("gblur=sigma=%.2f", blurSigma))
	}

	if useAI {
		if aiBackend != "ncnn" {
			return fmt.Errorf("AI upscaling backend not available")
		}

		if aiModel == "" {
			aiModel = "realesrgan-x4plus"
		}
		if aiOutputFormat == "" {
			aiOutputFormat = "png"
		}
		if aiOutputAdjust <= 0 {
			aiOutputAdjust = 1.0
		}
		if aiScale <= 0 {
			aiScale = 4.0
		}
		if aiThreadsLoad <= 0 {
			aiThreadsLoad = 1
		}
		if aiThreadsProc <= 0 {
			aiThreadsProc = 2
		}
		if aiThreadsSave <= 0 {
			aiThreadsSave = 2
		}

		outScale := aiScale
		if aiScaleUseTarget {
			switch targetPreset {
			case "", "Match Source":
				outScale = 1.0
			case "2X (relative)":
				outScale = 2.0
			case "4X (relative)":
				outScale = 4.0
			default:
				if sourceHeight > 0 && targetHeight > 0 {
					outScale = float64(targetHeight) / float64(sourceHeight)
				}
			}
		}
		outScale *= aiOutputAdjust
		if outScale < 0.1 {
			outScale = 0.1
		} else if outScale > 8.0 {
			outScale = 8.0
		}

		if progressCallback != nil {
			progressCallback(1)
		}

		workDir, err := os.MkdirTemp(utils.TempDir(), "vt-ai-upscale-")
		if err != nil {
			return fmt.Errorf("failed to create temp dir: %w", err)
		}
		defer os.RemoveAll(workDir)

		inputFramesDir := filepath.Join(workDir, "frames_in")
		outputFramesDir := filepath.Join(workDir, "frames_out")
		if err := os.MkdirAll(inputFramesDir, 0o755); err != nil {
			return fmt.Errorf("failed to create frames dir: %w", err)
		}
		if err := os.MkdirAll(outputFramesDir, 0o755); err != nil {
			return fmt.Errorf("failed to create frames dir: %w", err)
		}

		var preFilter string
		if len(baseFilters) > 0 {
			preFilter = strings.Join(baseFilters, ",")
		}

		// Determine the actual source colour space, defaulting by resolution when "auto".
		resolvedSrcCS := strings.ToLower(strings.TrimSpace(srcColorSpace))
		if resolvedSrcCS == "auto" || resolvedSrcCS == "" {
			// Probe the source to read embedded metadata first.
			if probed, err := probeVideo(inputPath); err == nil && probed != nil {
				switch strings.ToLower(probed.ColorSpace) {
				case "bt709", "rec709":
					resolvedSrcCS = "bt709"
				case "bt2020", "rec2020":
					resolvedSrcCS = "bt2020"
				case "bt601", "smpte170m", "smpte240m", "bt470bg":
					resolvedSrcCS = "bt601"
				default:
					// Fall back to resolution-based heuristic.
					if probed.Height > 0 && probed.Height <= 576 {
						resolvedSrcCS = "bt601"
					} else {
						resolvedSrcCS = "bt709"
					}
				}
			} else if sourceHeight > 0 && sourceHeight <= 576 {
				resolvedSrcCS = "bt601"
			} else {
				resolvedSrcCS = "bt709"
			}
		}

		// Build a colorspace normalisation filter when the source matrix differs
		// from the bt709 working space that AI models expect.  The filter also
		// linearises (gamma) and converts to full-range RGB, which eliminates the
		// "orange shift" caused by mismatched BT.601/BT.709 coefficients.
		colourFilters := []string{}
		if resolvedSrcCS != "bt709" {
			colourFilters = append(colourFilters,
				fmt.Sprintf("colorspace=all=bt709:iall=%s:fast=0", resolvedSrcCS),
			)
		}

		// For 16-bit depth, request rgb48 PNG so Real-ESRGAN gets full precision.
		use16bit := colorDepth == "16bit"

		// Always extract as PNG for colour accuracy (lossless; Real-ESRGAN reads PNG natively).
		frameExt := "png"
		framePattern := filepath.Join(inputFramesDir, "frame_%08d.png")
		extractArgs := []string{"-y", "-hide_banner", "-i", inputPath}

		// Merge colour filters with any pre-filters from the filters module.
		allExtractFilters := append(colourFilters, baseFilters...)
		if preFilter != "" {
			// preFilter already built from baseFilters; rebuild with colour prefix.
			allExtractFilters = append(colourFilters, strings.Split(preFilter, ",")...)
		} else {
			allExtractFilters = colourFilters
			if len(baseFilters) > 0 {
				allExtractFilters = append(allExtractFilters, baseFilters...)
			}
		}
		if len(allExtractFilters) > 0 {
			extractArgs = append(extractArgs, "-vf", strings.Join(allExtractFilters, ","))
		}
		if use16bit {
			extractArgs = append(extractArgs, "-pix_fmt", "rgb48le")
		}
		extractArgs = append(extractArgs, "-start_number", "0", framePattern)
		_ = frameExt // used in aiArgs below

		logFile, logPath, _ := createConversionLog(inputPath, outputPath, extractArgs)
		if logFile != nil {
			fmt.Fprintln(logFile, "Stage: extract frames for AI upscaling")
		}

		runFFmpegWithProgress := func(args []string, duration float64, startPct, endPct float64) error {
			if len(args) > 0 {
				last := args[len(args)-1]
				args = append(args[:len(args)-1], "-progress", "pipe:1", "-nostats", last)
			}
			cmd := exec.CommandContext(ctx, utils.GetFFmpegPath(), args...)
			utils.ApplyNoWindow(cmd)
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				return fmt.Errorf("failed to create stdout pipe: %w", err)
			}
			if logFile != nil {
				cmd.Stderr = logFile
			} else {
				cmd.Stderr = io.Discard
			}
			if err := utils.StartCmd(cmd); err != nil {
				return fmt.Errorf("failed to start ffmpeg: %w", err)
			}
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := scanner.Text()
				if logFile != nil {
					fmt.Fprintln(logFile, line)
				}
				if duration <= 0 {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) != 2 {
					continue
				}
				key, val := parts[0], parts[1]
				if key == "out_time_ms" {
					ms, err := strconv.ParseFloat(val, 64)
					if err != nil {
						continue
					}
					currentTime := ms / 1000000.0
					progress := startPct + ((currentTime / duration) * (endPct - startPct))
					if progressCallback != nil {
						progressCallback(progress)
					}
				}
			}
			return cmd.Wait()
		}

		duration := toFloat(cfg["duration"])
		if err := runFFmpegWithProgress(extractArgs, duration, 1, 35); err != nil {
			if logFile != nil {
				fmt.Fprintf(logFile, "\nStatus: failed during extraction at %s\nError: %v\n", time.Now().Format(time.RFC3339), err)
				_ = logFile.Close()
			}
			return fmt.Errorf("failed to extract frames: %w", err)
		}

		if progressCallback != nil {
			progressCallback(40)
		}

		aiBinary := "realesrgan-ncnn-vulkan"
		aiModelName := aiModel

		if strings.HasPrefix(aiModel, "realcugan") {
			aiBinary = "realcugan-ncnn-vulkan"
			switch aiModel {
			case "realcugan-pro":
				aiModelName = "pro"
			case "realcugan-se":
				aiModelName = "se"
			case "realcugan-no-denoise":
				aiModelName = "se"
			}
		}

		aiArgs := []string{
			"-i", inputFramesDir,
			"-o", outputFramesDir,
			"-n", aiModelName,
			"-s", fmt.Sprintf("%.2f", outScale),
			"-j", fmt.Sprintf("%d:%d:%d", aiThreadsLoad, aiThreadsProc, aiThreadsSave),
			"-f", frameExt,
		}
		if aiTile > 0 {
			aiArgs = append(aiArgs, "-t", strconv.Itoa(aiTile))
		}
		if !aiGPUAuto {
			aiArgs = append(aiArgs, "-g", strconv.Itoa(aiGPU))
		}
		if aiTTA {
			aiArgs = append(aiArgs, "-x")
		}
		if aiModel == "realesr-general-x4v3" {
			aiArgs = append(aiArgs, "-dn", fmt.Sprintf("%.2f", aiDenoise))
		}
		if strings.HasPrefix(aiModel, "realcugan") {
			denoiseLevel := 1
			if aiDenoise > 0.7 {
				denoiseLevel = 3
			} else if aiDenoise > 0.4 {
				denoiseLevel = 2
			} else if aiDenoise > 0.1 {
				denoiseLevel = 1
			} else {
				denoiseLevel = 0
			}
			if aiModel != "realcugan-no-denoise" {
				aiArgs = append(aiArgs, "-n", strconv.Itoa(denoiseLevel))
			}
		}
		if aiFaceEnhance && logFile != nil {
			fmt.Fprintln(logFile, "Note: face enhancement requested but not supported in ncnn backend")
		}

		if logFile != nil {
			fmt.Fprintln(logFile, "Stage: AI Upscale ("+aiBinary+")")
		}

		// Use full path from VerifyTool (checks PATH and app-local bin + smoke test)
		aiPath, aiPathFound := utils.VerifyTool(aiBinary)
		if !aiPathFound {
			return fmt.Errorf("AI upscaling tool not found or not working: %s (not in PATH or app-local bin, or fails to run)", aiBinary)
		}
		if logFile != nil {
			fmt.Fprintf(logFile, "Command: %s %s\n", aiPath, strings.Join(aiArgs, " "))
		}

		aiCmd := exec.CommandContext(ctx, aiPath, aiArgs...)
		utils.ApplyNoWindow(aiCmd)
		aiOut, err := aiCmd.CombinedOutput()
		if logFile != nil && len(aiOut) > 0 {
			fmt.Fprintln(logFile, string(aiOut))
		}
		if err != nil {
			if logFile != nil {
				fmt.Fprintf(logFile, "\nStatus: failed during AI upscale at %s\nError: %v\n", time.Now().Format(time.RFC3339), err)
				_ = logFile.Close()
			}
			return fmt.Errorf("AI upscaling failed: %w", err)
		}

		if progressCallback != nil {
			progressCallback(70)
		}

		if frameRate == "" || frameRate == "Source" {
			if sourceFrameRate <= 0 {
				if src, err := probeVideo(inputPath); err == nil && src != nil {
					sourceFrameRate = src.FrameRate
				}
			}
		} else if fps, err := strconv.ParseFloat(frameRate, 64); err == nil {
			sourceFrameRate = fps
		}

		if sourceFrameRate <= 0 {
			sourceFrameRate = 30.0
		}

		// Optional RIFE frame interpolation chained after Real-ESRGAN
		finalFramesDir := outputFramesDir
		finalFramePattern := fmt.Sprintf("frame_%%08d.%s", frameExt)
		outputFPS := sourceFrameRate
		if useRIFE {
			entries, _ := os.ReadDir(outputFramesDir)
			inputFrameCount := 0
			for _, e := range entries {
				if !e.IsDir() {
					inputFrameCount++
				}
			}
			rifeFramesDir := filepath.Join(workDir, "frames_rife")
			if mkErr := os.MkdirAll(rifeFramesDir, 0o755); mkErr != nil {
				return fmt.Errorf("create rife frames dir: %w", mkErr)
			}
			rifeArgs := []string{
				"-i", outputFramesDir,
				"-o", rifeFramesDir,
				"-m", rifeModel,
				"-n", strconv.Itoa(inputFrameCount * rifeMultiplier),
			}
			rifeBin, rifeFound := utils.VerifyTool("rife-ncnn-vulkan")
			if !rifeFound {
				return fmt.Errorf("RIFE interpolation failed: rife-ncnn-vulkan not found in PATH or app-local bin, or fails to run")
			}
			if logFile != nil {
				fmt.Fprintln(logFile, "Stage: RIFE frame interpolation")
				fmt.Fprintf(logFile, "Command: %s %s\n", rifeBin, strings.Join(rifeArgs, " "))
			}
			rifeCmd := exec.CommandContext(ctx, rifeBin, rifeArgs...)
			utils.ApplyNoWindow(rifeCmd)
			rifeOut, rifeErr := rifeCmd.CombinedOutput()
			if logFile != nil && len(rifeOut) > 0 {
				fmt.Fprintln(logFile, string(rifeOut))
			}
			if rifeErr != nil {
				return fmt.Errorf("RIFE interpolation failed: %w", rifeErr)
			}
			finalFramesDir = rifeFramesDir
			finalFramePattern = "%08d.png"
			outputFPS = sourceFrameRate * float64(rifeMultiplier)
			if progressCallback != nil {
				progressCallback(82)
			}
		}

		reassemblePattern := filepath.Join(finalFramesDir, finalFramePattern)
		reassembleArgs := []string{
			"-y",
			"-hide_banner",
			"-threads", "auto",
			"-framerate", fmt.Sprintf("%.3f", outputFPS),
			"-i", reassemblePattern,
			"-i", inputPath,
			"-map", "0:v:0",
			"-map", "1:a?",
		}

		// Build post-process filter chain for reassembly.
		// Skin tone preservation: hue-selective saturation nudge in the red-pink
		// band to compensate for chroma suppression from the AI model.
		// This only touches the Cr channel in the skin-tone hue sector.
		var reassembleFilters []string
		if targetPreset != "" && targetPreset != "Match Source" {
			reassembleFilters = append(reassembleFilters, buildUpscaleFilter(targetWidth, targetHeight, method, preserveAR))
		}
		switch skinTone {
		case "subtle":
			reassembleFilters = append(reassembleFilters,
				"huesaturation=hue=0:saturation=0.04:intensity=0:rH=1:rS=1:rI=0:rV=1",
			)
		case "strong":
			reassembleFilters = append(reassembleFilters,
				"huesaturation=hue=0:saturation=0.09:intensity=0:rH=1:rS=1:rI=0:rV=1",
			)
		}
		if len(reassembleFilters) > 0 {
			reassembleArgs = append(reassembleArgs, "-vf", strings.Join(reassembleFilters, ","))
		}

		reassembleArgs = append(reassembleArgs, "-c:v", videoEncoder)
		reassembleArgs = appendEncodingArgs(reassembleArgs)

		// Pixel format: honour user choice; fall back to yuv420p for compatibility.
		outPixFmt := pixelFormat
		if outPixFmt == "" {
			outPixFmt = "yuv420p"
		}
		// Hardware encoders require specific pixel formats; override if needed.
		if strings.HasSuffix(videoEncoder, "_nvenc") || strings.HasSuffix(videoEncoder, "_qsv") {
			if outPixFmt == "yuv444p" {
				outPixFmt = "yuv444p"
			} else {
				outPixFmt = "yuv420p"
			}
		}

		// Tag the output with the correct colour space metadata so players and
		// downstream tools do not apply a second (wrong) colour conversion.
		targetCS := "bt709" // we always output bt709 from the AI pipeline
		reassembleArgs = append(reassembleArgs,
			"-pix_fmt", outPixFmt,
			"-colorspace", targetCS,
			"-color_primaries", targetCS,
			"-color_trc", targetCS,
			"-c:a", "copy",
			"-shortest",
		)

		// Derive output path from container choice.
		outExt := "." + strings.ToLower(outputContainer)
		if outputContainer == "" {
			outExt = filepath.Ext(outputPath)
		}
		if outExt == "." || outExt == "" {
			outExt = ".mp4"
		}
		finalOutputPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + outExt
		outputPath = finalOutputPath

		// Inject BPS tag for MKV output so Windows Explorer shows the correct
		// bitrate. Hardware encoders (NVENC, AMF) don't write per-stream stats
		// to the Matroska tags block, leaving BPS=0.
		if strings.EqualFold(outExt, ".mkv") {
			mode := normalizeBitrateMode(bitrateMode)
			if mode == "CBR" || mode == "VBR" {
				if bps := parseBitrateStringToBPS(resolveBitrate()); bps > 0 {
					reassembleArgs = append(reassembleArgs, "-metadata:s:v:0", fmt.Sprintf("BPS=%d", bps))
				}
			}
		}

		reassembleArgs = append(reassembleArgs, finalOutputPath)

		if logFile != nil {
			fmt.Fprintln(logFile, "Stage: reassemble")
		}

		if err := runFFmpegWithProgress(reassembleArgs, duration, 70, 100); err != nil {
			if logFile != nil {
				fmt.Fprintf(logFile, "\nStatus: failed during reassemble at %s\nError: %v\n", time.Now().Format(time.RFC3339), err)
				_ = logFile.Close()
			}
			return fmt.Errorf("failed to reassemble upscaled video: %w", err)
		}

		if logFile != nil {
			fmt.Fprintf(logFile, "\nStatus: completed at %s\n", time.Now().Format(time.RFC3339))
			_ = logFile.Close()
			job.LogPath = logPath
		}

		if progressCallback != nil {
			progressCallback(100)
		}

		return nil
	}

	// RIFE-only path: extract frames → RIFE frame interpolation → reassemble
	if useRIFE {
		if frameRate != "" && frameRate != "Source" {
			if fps, err := strconv.ParseFloat(frameRate, 64); err == nil {
				sourceFrameRate = fps
			}
		}
		if sourceFrameRate <= 0 {
			if src, err := probeVideo(inputPath); err == nil && src != nil {
				sourceFrameRate = src.FrameRate
			}
		}
		if sourceFrameRate <= 0 {
			sourceFrameRate = 30.0
		}
		outputFPS := sourceFrameRate * float64(rifeMultiplier)

		workDir, err := os.MkdirTemp(utils.TempDir(), "vt-rife-")
		if err != nil {
			return fmt.Errorf("create rife temp dir: %w", err)
		}
		defer os.RemoveAll(workDir)

		inputFramesDir := filepath.Join(workDir, "frames_in")
		rifeFramesDir := filepath.Join(workDir, "frames_rife")
		if err := os.MkdirAll(inputFramesDir, 0o755); err != nil {
			return fmt.Errorf("create frames dir: %w", err)
		}
		if err := os.MkdirAll(rifeFramesDir, 0o755); err != nil {
			return fmt.Errorf("create rife dir: %w", err)
		}

		// Apply scale + any pre-filters during frame extraction
		scaleFilter := buildUpscaleFilter(targetWidth, targetHeight, method, preserveAR)
		extractFilters := append(baseFilters, scaleFilter)
		framePattern := filepath.Join(inputFramesDir, "frame_%08d.png")
		extractArgs := []string{"-y", "-hide_banner", "-i", inputPath}
		if len(extractFilters) > 0 {
			extractArgs = append(extractArgs, "-vf", strings.Join(extractFilters, ","))
		}
		extractArgs = append(extractArgs, "-start_number", "0", framePattern)

		logFile, logPath, _ := createConversionLog(inputPath, outputPath, extractArgs)
		if logFile != nil {
			fmt.Fprintln(logFile, "Stage: extract frames for RIFE")
		}
		if progressCallback != nil {
			progressCallback(1)
		}
		if err := runCommandWithLogger(ctx, utils.GetFFmpegPath(), extractArgs, func(line string) {
			if logFile != nil {
				fmt.Fprintln(logFile, line)
			}
		}); err != nil {
			return fmt.Errorf("failed to extract frames: %w", err)
		}
		if progressCallback != nil {
			progressCallback(35)
		}

		// Count extracted frames and run RIFE
		entries, _ := os.ReadDir(inputFramesDir)
		inputFrameCount := 0
		for _, e := range entries {
			if !e.IsDir() {
				inputFrameCount++
			}
		}
		rifeArgs := []string{
			"-i", inputFramesDir,
			"-o", rifeFramesDir,
			"-m", rifeModel,
			"-n", strconv.Itoa(inputFrameCount * rifeMultiplier),
		}
		rifeBin2, rifeFound2 := utils.FindTool("rife-ncnn-vulkan")
		if !rifeFound2 {
			return fmt.Errorf("RIFE interpolation failed: rife-ncnn-vulkan not found in PATH or app-local bin")
		}
		if logFile != nil {
			fmt.Fprintln(logFile, "Stage: RIFE frame interpolation")
			fmt.Fprintf(logFile, "Command: %s %s\n", rifeBin2, strings.Join(rifeArgs, " "))
		}
		rifeCmd := exec.CommandContext(ctx, rifeBin2, rifeArgs...)
		utils.ApplyNoWindow(rifeCmd)
		rifeOut, rifeErr := rifeCmd.CombinedOutput()
		if logFile != nil && len(rifeOut) > 0 {
			fmt.Fprintln(logFile, string(rifeOut))
		}
		if rifeErr != nil {
			return fmt.Errorf("RIFE interpolation failed: %w", rifeErr)
		}
		if progressCallback != nil {
			progressCallback(80)
		}

		// Reassemble with original audio
		reassemblePattern := filepath.Join(rifeFramesDir, "%08d.png")
		reassembleArgs := []string{
			"-y", "-hide_banner",
			"-threads", "auto",
			"-framerate", fmt.Sprintf("%.3f", outputFPS),
			"-i", reassemblePattern,
			"-i", inputPath,
			"-map", "0:v:0",
			"-map", "1:a?",
			"-c:v", videoEncoder,
		}
		reassembleArgs = appendEncodingArgs(reassembleArgs)
		reassembleArgs = append(reassembleArgs,
			"-pix_fmt", pixelFormat,
			"-c:a", "copy",
			"-shortest",
		)
		// Inject BPS tag for MKV output so Windows Explorer shows the correct
		// bitrate. Hardware encoders (NVENC, AMF) don't write per-stream stats
		// to the Matroska tags block, leaving BPS=0.
		if strings.EqualFold(filepath.Ext(outputPath), ".mkv") {
			mode := normalizeBitrateMode(bitrateMode)
			if mode == "CBR" || mode == "VBR" {
				if bps := parseBitrateStringToBPS(resolveBitrate()); bps > 0 {
					reassembleArgs = append(reassembleArgs, "-metadata:s:v:0", fmt.Sprintf("BPS=%d", bps))
				}
			}
		}
		reassembleArgs = append(reassembleArgs, outputPath)
		if logFile != nil {
			fmt.Fprintln(logFile, "Stage: reassemble")
		}
		if err := runCommandWithLogger(ctx, utils.GetFFmpegPath(), reassembleArgs, func(line string) {
			if logFile != nil {
				fmt.Fprintln(logFile, line)
			}
		}); err != nil {
			return fmt.Errorf("failed to reassemble: %w", err)
		}
		if logFile != nil {
			fmt.Fprintf(logFile, "\nStatus: completed at %s\n", time.Now().Format(time.RFC3339))
			_ = logFile.Close()
			job.LogPath = logPath
		}
		if progressCallback != nil {
			progressCallback(100)
		}
		return nil
	}

	// Add scale filter (preserve aspect by default)
	scaleFilter := buildUpscaleFilter(targetWidth, targetHeight, method, preserveAR)
	logging.Debug(logging.CatFFMPEG, "upscale: target=%dx%d preserveAR=%v method=%s filter=%s", targetWidth, targetHeight, preserveAR, method, scaleFilter)
	baseFilters = append(baseFilters, scaleFilter)

	// Combine filters
	var vfilter string
	if len(baseFilters) > 0 {
		vfilter = strings.Join(baseFilters, ",")
	}

	// Build FFmpeg command
	args := []string{
		"-y",
		"-hide_banner",
		"-threads", "auto",
		"-i", inputPath,
	}

	// Add video filter if we have any
	if vfilter != "" {
		args = append(args, "-vf", vfilter)
	}

	// Use MKV container by default; copy audio
	args = append(args, "-c:v", videoEncoder)
	args = appendEncodingArgs(args)
	args = append(args,
		"-pix_fmt", pixelFormat,
		"-c:a", "copy",
		"-progress", "pipe:1",
		"-nostats",
	)
	// Inject BPS tag for MKV output so Windows Explorer shows the correct
	// bitrate. Hardware encoders (NVENC, AMF) don't write per-stream stats
	// to the Matroska tags block, leaving BPS=0.
	if strings.EqualFold(filepath.Ext(outputPath), ".mkv") {
		mode := normalizeBitrateMode(bitrateMode)
		if mode == "CBR" || mode == "VBR" {
			if bps := parseBitrateStringToBPS(resolveBitrate()); bps > 0 {
				args = append(args, "-metadata:s:v:0", fmt.Sprintf("BPS=%d", bps))
			}
		}
	}
	args = append(args, outputPath)

	logFile, logPath, _ := createConversionLog(inputPath, outputPath, args)
	cmd := exec.CommandContext(ctx, utils.GetFFmpegPath(), args...)
	utils.ApplyNoWindow(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	if logFile != nil {
		cmd.Stderr = logFile
	} else {
		cmd.Stderr = io.Discard
	}

	if err := utils.StartCmd(cmd); err != nil {
		return fmt.Errorf("failed to start upscale: %w", err)
	}

	// Parse progress from FFmpeg stdout (-progress pipe:1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if logFile != nil {
				fmt.Fprintln(logFile, line)
			}

			if duration, ok := cfg["duration"].(float64); ok && duration > 0 {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) != 2 {
					continue
				}
				key, val := parts[0], parts[1]
				if key == "out_time_ms" {
					ms, err := strconv.ParseFloat(val, 64)
					if err != nil {
						continue
					}
					currentTime := ms / 1000000.0
					progress := (currentTime / duration) * 100.0
					if progress > 100.0 {
						progress = 100.0
					}
					if progressCallback != nil {
						progressCallback(progress)
					}
				}
			}
		}
	}()

	err = cmd.Wait()
	if err != nil {
		if logFile != nil {
			fmt.Fprintf(logFile, "\nStatus: failed at %s\nError: %v\n", time.Now().Format(time.RFC3339), err)
			_ = logFile.Close()
		}
		return fmt.Errorf("upscale failed: %w", err)
	}

	if logFile != nil {
		fmt.Fprintf(logFile, "\nStatus: completed at %s\n", time.Now().Format(time.RFC3339))
		_ = logFile.Close()
		job.LogPath = logPath
	}

	if progressCallback != nil {
		progressCallback(100)
	}

	return nil
}

// buildFFmpegCommandFromJob builds an FFmpeg command string from a queue job with INPUT/OUTPUT placeholders
func buildFFmpegCommandFromJob(job *queue.Job) string {
	if job == nil || job.Config == nil {
		return ""
	}

	cfg := job.Config
	args := []string{"-y", "-hide_banner", "-loglevel", "error"}

	// Hardware acceleration - MUST come BEFORE input file
	if hardwareAccel, _ := cfg["hardwareAccel"].(string); hardwareAccel != "" && hardwareAccel != "none" {
		switch hardwareAccel {
		case "vaapi":
			args = append(args, "-hwaccel", "vaapi")
		case "qsv":
			args = append(args, "-hwaccel", "qsv")
		case "videotoolbox":
			args = append(args, "-hwaccel", "videotoolbox")
		}
	}

	// Input
	args = append(args, "-i", "INPUT")

	// Cover art if present (convert jobs only)
	if job.Type == queue.JobTypeConvert {
		if coverArtPath, _ := cfg["coverArtPath"].(string); coverArtPath != "" {
			args = append(args, "-i", "[COVER_ART]")
		}
	}

	// Build video filters
	var vf []string

	// Deinterlacing
	if deinterlaceMode, _ := cfg["deinterlace"].(string); deinterlaceMode == "Force" {
		deintMethod, _ := cfg["deinterlaceMethod"].(string)
		if deintMethod == "" || deintMethod == "bwdif" {
			vf = append(vf, "bwdif=mode=send_frame:parity=auto")
		} else {
			vf = append(vf, "yadif=0:-1:0")
		}
	}

	autoCrop, _ := cfg["autoCrop"].(bool)
	// Cropping
	if autoCrop {
		if cropWidth, _ := cfg["cropWidth"].(string); cropWidth != "" {
			cropHeight, _ := cfg["cropHeight"].(string)
			cropX, _ := cfg["cropX"].(string)
			cropY, _ := cfg["cropY"].(string)
			if cropX == "" {
				cropX = "(in_w-out_w)/2"
			}
			if cropY == "" {
				cropY = "(in_h-out_h)/2"
			}
			vf = append(vf, fmt.Sprintf("crop=%s:%s:%s:%s", cropWidth, cropHeight, cropX, cropY))
		}
	}

	// Scaling
	if targetResolution, _ := cfg["targetResolution"].(string); targetResolution != "" && targetResolution != "Source" {
		var scaleFilter string
		switch targetResolution {
		case "360p":
			scaleFilter = "scale=-2:360"
		case "480p":
			scaleFilter = "scale=-2:480"
		case "540p":
			scaleFilter = "scale=-2:540"
		case "720p":
			scaleFilter = "scale=-2:720"
		case "1080p":
			scaleFilter = "scale=-2:1080"
		case "1440p":
			scaleFilter = "scale=-2:1440"
		case "4K":
			scaleFilter = "scale=-2:2160"
		case "8K":
			scaleFilter = "scale=-2:4320"
		default:
			// Try "WxH" format (e.g. "720x480", "1920x1080")
			if w, h, ok := parseResolutionWxH(targetResolution); ok {
				scaleFilter = fmt.Sprintf("scale=%d:%d:flags=lanczos", w, h)
				logging.Debug(logging.CatFFMPEG, "scale: parsed WxH resolution %s → %s", targetResolution, scaleFilter)
			}
		}
		if scaleFilter != "" {
			vf = append(vf, scaleFilter)
		}
	}

	// Aspect ratio handling (simplified)
	outputAspect, _ := cfg["outputAspect"].(string)
	if outputAspect != "" && outputAspect != "Source" {
		aspectHandling, _ := cfg["aspectHandling"].(string)
		if aspectHandling == "letterbox" {
			vf = append(vf, fmt.Sprintf("pad=iw:iw*(%s/(sar*dar)):(ow-iw)/2:(oh-ih)/2", outputAspect))
		} else if aspectHandling == "crop" {
			vf = append(vf, "crop=iw:iw/("+outputAspect+"):0:(ih-oh)/2")
		}
	}

	// Force aspect metadata when enabled
	forceAspect := true
	if v, ok := cfg["forceAspect"].(bool); ok {
		forceAspect = v
	}
	if srcAspect := displayAspectRatioFromConfig(cfg); srcAspect > 0 {
		sourceWidth, _ := cfg["sourceWidth"].(int)
		sourceHeight, _ := cfg["sourceHeight"].(int)
		sampleAspectRatio, _ := cfg["sampleAspectRatio"].(string)
		displayAspectRatio, _ := cfg["displayAspectRatio"].(string)
		rotation, _ := cfg["rotation"].(string)
		logging.Debug(
			logging.CatFFMPEG,
			"aspect: source=%dx%d sar=%s dar=%s rotation=%s sourceAR=%.4f target=%s handling=%s force=%v autoCrop=%v",
			sourceWidth,
			sourceHeight,
			strings.TrimSpace(sampleAspectRatio),
			strings.TrimSpace(displayAspectRatio),
			strings.TrimSpace(rotation),
			srcAspect,
			strings.TrimSpace(outputAspect),
			strings.TrimSpace(fmt.Sprint(cfg["aspectHandling"])),
			forceAspect,
			autoCrop,
		)
	}
	if forceAspect {
		sourceWidth, _ := cfg["sourceWidth"].(int)
		sourceHeight, _ := cfg["sourceHeight"].(int)
		sampleAspectRatio, _ := cfg["sampleAspectRatio"].(string)
		displayAspectRatio, _ := cfg["displayAspectRatio"].(string)
		tempSrc := &videoSource{
			Width:              sourceWidth,
			Height:             sourceHeight,
			SampleAspectRatio:  sampleAspectRatio,
			DisplayAspectRatio: displayAspectRatio,
		}
		outputAspect, _ := cfg["outputAspect"].(string)
		if targetAspect := resolveTargetAspect(outputAspect, tempSrc); targetAspect > 0 {
			if len(vf) == 0 {
				vf = append(vf, fmt.Sprintf("setdar=%.6f", targetAspect), "setsar=1")
			} else {
				vf = appendAspectMetadata(vf, targetAspect)
			}
		}
	}

	// Flipping
	if flipH, _ := cfg["flipHorizontal"].(bool); flipH {
		vf = append(vf, "hflip")
	}
	if flipV, _ := cfg["flipVertical"].(bool); flipV {
		vf = append(vf, "vflip")
	}

	// Rotation
	if rotation, _ := cfg["rotation"].(string); rotation != "" && rotation != "0" {
		switch rotation {
		case "90":
			vf = append(vf, "transpose=1")
		case "180":
			vf = append(vf, "transpose=1,transpose=1")
		case "270":
			vf = append(vf, "transpose=2")
		}
	}

	// Frame rate
	if frameRate, _ := cfg["frameRate"].(string); frameRate != "" && frameRate != "Source" {
		useMotionInterp, _ := cfg["useMotionInterpolation"].(bool)
		if useMotionInterp {
			vf = append(vf, fmt.Sprintf("minterpolate=fps=%s:mi_mode=mci:mc_mode=aobmc:me_mode=bidir:vsbmc=1", frameRate))
		} else {
			vf = append(vf, "fps="+frameRate)
		}
	}

	if len(vf) > 0 {
		args = append(args, "-vf", strings.Join(vf, ","))
	}

	// Video codec
	videoCodec, _ := cfg["videoCodec"].(string)
	if videoCodec == "Copy" {
		args = append(args, "-c:v", "copy")
	} else {
		// Determine codec (simplified)
		codec := "libx264"
		hardwareAccel, _ := cfg["hardwareAccel"].(string)

		// Resolve "auto" to actual GPU vendor
		if hardwareAccel == "auto" {
			hwInfo := sysinfo.Detect()
			switch hwInfo.GPUVendor() {
			case "nvidia":
				hardwareAccel = "nvenc"
				logging.Debug(logging.CatFFMPEG, "auto hardware accel resolved to nvenc (detected NVIDIA GPU)")
			case "amd":
				hardwareAccel = "amf"
				logging.Debug(logging.CatFFMPEG, "auto hardware accel resolved to amf (detected AMD GPU)")
			case "intel":
				hardwareAccel = "qsv"
				logging.Debug(logging.CatFFMPEG, "auto hardware accel resolved to qsv (detected Intel GPU)")
			default:
				hardwareAccel = "none"
				logging.Debug(logging.CatFFMPEG, "auto hardware accel resolved to none (no compatible GPU detected)")
			}
		}

		switch {
		case videoCodec == "H.265" && hardwareAccel == "nvenc":
			codec = "hevc_nvenc"
		case videoCodec == "H.265" && hardwareAccel == "qsv":
			codec = "hevc_qsv"
		case videoCodec == "H.265" && hardwareAccel == "amf":
			codec = "hevc_amf"
		case videoCodec == "H.265" && hardwareAccel == "videotoolbox":
			codec = "hevc_videotoolbox"
		case videoCodec == "H.265":
			codec = "libx265"
		case videoCodec == "H.264" && hardwareAccel == "nvenc":
			codec = "h264_nvenc"
		case videoCodec == "H.264" && hardwareAccel == "qsv":
			codec = "h264_qsv"
		case videoCodec == "H.264" && hardwareAccel == "amf":
			codec = "h264_amf"
		case videoCodec == "H.264" && hardwareAccel == "videotoolbox":
			codec = "h264_videotoolbox"
		case videoCodec == "AV1":
			resolved, ok := resolveAV1Encoder(hardwareAccel)
			if !ok {
				logging.Debug(logging.CatFFMPEG, "AV1 encoder unavailable; falling back to %s", resolved)
			}
			codec = resolved
		case videoCodec == "VP9":
			codec = "libvpx-vp9"
		case videoCodec == "MPEG-2":
			codec = "mpeg2video"
		}
		args = append(args, "-c:v", codec)

		// Quality/bitrate settings
		bitrateMode, _ := cfg["bitrateMode"].(string)
		if bitrateMode == "CRF" || bitrateMode == "" {
			crfStr, _ := cfg["crf"].(string)
			if crfStr == "" {
				quality, _ := cfg["quality"].(string)
				switch quality {
				case "Lossless":
					crfStr = "0"
				case "High":
					crfStr = "18"
				case "Medium":
					crfStr = "23"
				case "Low":
					crfStr = "28"
				default:
					crfStr = "23"
				}
			}
			if codec == "libx264" || codec == "libx265" || codec == "libvpx-vp9" || codec == "libsvtav1" || codec == "libaom-av1" {
				args = append(args, "-crf", crfStr)
			}
			// Hardware encoder quality flags
			switch codec {
			case "h264_nvenc", "hevc_nvenc":
				encoderPreset, _ := cfg["encoderPreset"].(string)
				nvencPreset := map[string]string{
					"veryslow": "p7", "slower": "p6", "slow": "p5",
					"medium": "p4", "fast": "p3", "faster": "p2",
					"veryfast": "p1", "superfast": "p1", "ultrafast": "p1",
				}
				if p, ok := nvencPreset[encoderPreset]; ok {
					args = append(args, "-preset", p)
				} else {
					args = append(args, "-preset", "p4")
				}
				if crfStr != "" {
					args = append(args, "-rc", "vbr", "-cq", crfStr)
				}
			case "h264_amf", "hevc_amf":
				encoderPreset, _ := cfg["encoderPreset"].(string)
				amfQuality := map[string]string{
					"veryslow": "quality", "slower": "quality", "slow": "quality",
					"medium": "balanced", "fast": "speed", "faster": "speed",
					"veryfast": "speed", "superfast": "speed", "ultrafast": "speed",
				}
				if q, ok := amfQuality[encoderPreset]; ok {
					args = append(args, "-quality", q)
				} else {
					args = append(args, "-quality", "balanced")
				}
				if crfStr != "" {
					args = append(args, "-qp_i", crfStr, "-qp_p", crfStr, "-qp_b", crfStr)
				}
			case "h264_qsv", "hevc_qsv":
				if crfStr != "" {
					args = append(args, "-global_quality", crfStr)
				}
				encoderPreset, _ := cfg["encoderPreset"].(string)
				qsvPreset := map[string]string{
					"veryslow": "veryslow", "slower": "slower", "slow": "slow",
					"medium": "medium", "fast": "fast", "faster": "faster",
					"veryfast": "veryfast",
				}
				if p, ok := qsvPreset[encoderPreset]; ok {
					args = append(args, "-preset", p)
				}
			}
		} else if bitrateMode == "CBR" {
			if videoBitrate, _ := cfg["videoBitrate"].(string); videoBitrate != "" {
				args = append(args, "-b:v", videoBitrate, "-minrate", videoBitrate, "-maxrate", videoBitrate, "-bufsize", videoBitrate)
			}
		} else if bitrateMode == "VBR" {
			if videoBitrate, _ := cfg["videoBitrate"].(string); videoBitrate != "" {
				args = append(args, "-b:v", videoBitrate)
			}
		}

		// Encoder preset
		if encoderPreset, _ := cfg["encoderPreset"].(string); encoderPreset != "" {
			if codec == "libx264" || codec == "libx265" {
				args = append(args, "-preset", encoderPreset)
			} else if codec == "libsvtav1" {
				// Map x264/x265 presets to SVT-AV1 presets (0-13, lower=slower/better)
				var svtPreset string
				switch encoderPreset {
				case "veryslow":
					svtPreset = "3"
				case "slower":
					svtPreset = "4"
				case "slow":
					svtPreset = "5"
				case "medium":
					svtPreset = "6" // Default for reasonable speed
				case "fast":
					svtPreset = "8"
				case "faster":
					svtPreset = "9"
				case "veryfast":
					svtPreset = "10"
				case "superfast":
					svtPreset = "11"
				case "ultrafast":
					svtPreset = "12"
				default:
					svtPreset = "6" // Medium
				}
				args = append(args, "-preset", svtPreset)
			}
		}

		// Pixel format
		if pixelFormat, _ := cfg["pixelFormat"].(string); pixelFormat != "" {
			args = append(args, "-pix_fmt", pixelFormat)
		}

		// H.264 profile/level
		if videoCodec == "H.264" {
			if h264Profile, _ := cfg["h264Profile"].(string); h264Profile != "" && h264Profile != "Auto" {
				args = append(args, "-profile:v", h264Profile)
			}
			if h264Level, _ := cfg["h264Level"].(string); h264Level != "" && h264Level != "Auto" {
				args = append(args, "-level:v", h264Level)
			}
		}
	}

	// Audio codec
	audioCodec, _ := cfg["audioCodec"].(string)
	if audioCodec == "Copy" {
		args = append(args, "-c:a", "copy")
	} else {
		codec := "aac"
		switch audioCodec {
		case "AAC":
			codec = "aac"
		case "Opus":
			codec = "libopus"
		case "Vorbis":
			codec = "libvorbis"
		case "MP3":
			codec = "libmp3lame"
		case "FLAC":
			codec = "flac"
		case "AC-3":
			codec = "ac3"
		}
		args = append(args, "-c:a", codec)

		if audioBitrate, _ := cfg["audioBitrate"].(string); audioBitrate != "" && codec != "flac" {
			args = append(args, "-b:a", audioBitrate)
		}

		// Audio channels
		if audioChannels, _ := cfg["audioChannels"].(string); audioChannels != "" && audioChannels != "Source" {
			switch audioChannels {
			case "Mono":
				args = append(args, "-ac", "1")
			case "Stereo":
				args = append(args, "-ac", "2")
			case "5.1":
				args = append(args, "-ac", "6")
			case "Left to Stereo":
				// Copy left channel to both left and right
				args = append(args, "-af", "pan=stereo|c0=c0|c1=c0")
			case "Right to Stereo":
				// Copy right channel to both left and right
				args = append(args, "-af", "pan=stereo|c0=c1|c1=c1")
			case "Mix to Stereo":
				// Downmix both channels together, then duplicate to L+R
				args = append(args, "-af", "pan=stereo|c0=0.5*c0+0.5*c1|c1=0.5*c0+0.5*c1")
			case "Swap L/R":
				// Swap left and right channels
				args = append(args, "-af", "pan=stereo|c0=c1|c1=c0")
			}
		}

	}

	// Output
	args = append(args, "OUTPUT")

	return "ffmpeg " + strings.Join(args, " ")
}

// parseResolutionWxH attempts to parse a "WxH" resolution string (e.g. "720x480").
// Returns the width, height, and true on success.
func parseResolutionWxH(s string) (int, int, bool) {
	parts := strings.Split(s, "x")
	if len(parts) != 2 {
		parts = strings.Split(s, "X")
	}
	if len(parts) != 2 {
		return 0, 0, false
	}
	w, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errW != nil || errH != nil || w <= 0 || h <= 0 {
		return 0, 0, false
	}
	return w, h, true
}

func (s *appState) shutdown() {
	s.persistConvertConfig()

	// Stop queue without saving - we want a clean slate each session
	if s.jobQueue != nil {
		s.jobQueue.Stop()
	}

	s.stopPlayer()
	if s.player != nil {
		s.player.Close()
	}
}

func (s *appState) stopPlayer() {
	if s.player != nil {
		s.player.Stop()
	}
	s.stopProgressLoop()
	s.playerReady = false
	s.playerPaused = true
	// Close native media player to stop audio and free resources
	if HasNativeMediaPlayer() {
		s.closeNativePlayer()
	}
}

func main() {
	if cfg, err := loadPersistedConvertConfig(); err == nil {
		if strings.TrimSpace(cfg.LogDir) != "" {
			setLogsDirOverride(cfg.LogDir)
		}
	}
	logging.SetLogsDir(getLogsDir())
	logging.SetVersion(fullVersion())
	logging.Init()
	defer logging.Close()
	defer logging.RecoverPanic() // Catch and log any panics with stack trace
	utils.InitJobObject() // Create Windows Job Object (no-op on Linux)

	flag.Parse()
	logging.SetDebug(*debugFlag || os.Getenv("VIDEOTOOLS_DEBUG") != "")
	logging.Info(logging.CatSystem, "starting VideoTools %s at %s", fullVersion(), time.Now().Format(time.RFC3339))

	// Detect platform and configure paths
	cfg := DetectPlatform()                               // Detect and initialize platform paths locally
	utils.SetFFmpegPaths(cfg.FFmpegPath, cfg.FFprobePath) // Set global paths in utils package

	// Check if FFmpeg was found; if not, log a warning (using utils.GetFFmpegPath)
	if utils.GetFFmpegPath() == "ffmpeg" || utils.GetFFmpegPath() == "ffmpeg.exe" {
		logging.Debug(logging.CatSystem, "WARNING: FFmpeg not found in expected locations, assuming it's in PATH")
	}

	args := flag.Args()
	if len(args) > 0 {
		if err := runCLI(args); err != nil {
			fmt.Fprintln(os.Stderr, "videotools:", err)
			fmt.Fprintln(os.Stderr)
			printUsage()
			os.Exit(1)
		}
		return
	}

	// Detect display server (X11 or Wayland)
	display := os.Getenv("DISPLAY")
	waylandDisplay := os.Getenv("WAYLAND_DISPLAY")
	xdgSessionType := os.Getenv("XDG_SESSION_TYPE")

	if waylandDisplay != "" {
		logging.Debug(logging.CatUI, "Wayland display server detected: WAYLAND_DISPLAY=%s", waylandDisplay)
	} else if display != "" {
		logging.Debug(logging.CatUI, "X11 display server detected: DISPLAY=%s", display)
	} else {
		logging.Debug(logging.CatUI, "No display server detected (DISPLAY and WAYLAND_DISPLAY are empty); GUI may not be visible in headless mode")
	}

	if xdgSessionType != "" {
		logging.Debug(logging.CatUI, "Session type: %s", xdgSessionType)
	}
	runGUI()
}

func failGUIStartup(reason interface{}) {
	msg := fmt.Sprint(reason)
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "ERROR: VideoTools failed to start the GUI.")
	if strings.TrimSpace(msg) != "" {
		fmt.Fprintln(os.Stderr, "Reason:", msg)
	}
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "opengl") || strings.Contains(lower, "wgl") || strings.Contains(lower, "glfw") || strings.Contains(lower, "apiunavailable") {
		fmt.Fprintln(os.Stderr, "This usually means your graphics driver or VM does not support OpenGL.")
		fmt.Fprintln(os.Stderr, "If you're running in a VM (e.g., GNOME Boxes), enable 3D acceleration or run on the host OS.")
	} else {
		fmt.Fprintln(os.Stderr, "If you're running in a VM or over Remote Desktop, GPU/OpenGL may be unavailable.")
	}
	fmt.Fprintln(os.Stderr, "Install/upgrade your GPU drivers and try again.")
	if runtime.GOOS == "windows" {
		fmt.Fprintln(os.Stderr, "Press any key to close...")
		_, _ = bufio.NewReader(os.Stdin).ReadByte()
	}
	os.Exit(1)
}

func preflightOpenGL() {
	if runtime.GOOS != "windows" {
		return
	}
	info := sysinfo.Detect()
	gpu := strings.ToLower(info.GPU)
	if gpu == "" {
		return
	}
	badGPU := []string{
		"microsoft basic",
		"microsoft remote display",
		"vmware",
		"virtualbox",
		"virtio",
		"qxl",
		"hyper-v",
		"llvmpipe",
		"svga",
	}
	for _, token := range badGPU {
		if strings.Contains(gpu, token) {
			failGUIStartup(fmt.Sprintf("OpenGL unavailable (%s)", info.GPU))
		}
	}
}

func runGUI() {
	preflightOpenGL()
	defer func() {
		if r := recover(); r != nil {
			failGUIStartup(r)
		}
	}()
	// Initialize UI colors
	ui.SetColors(gridColor, textColor)

	utils.EnsureLinuxDesktopEntry("com.leaktechnologies.videotools", "VideoTools")

	// Preflight GUI environment before creating the Fyne app.
	guiEnv := guitutils.DetectGUIEnvironment()
	logging.Debug(logging.CatUI, "detected GUI environment: %s", guiEnv.String())
	if runtime.GOOS == "windows" {
		if isSoftware, _ := guiEnv.GPUInfo.IsLikelySoftwareOnlyAdapter(); isSoftware {
			failGUIStartup(fmt.Sprintf("VideoTools could not start the GUI because the detected display adapter does not support OpenGL acceleration.\n\nDetected adapter:\n%s\n\nIf you are running in a VM, enable 3D acceleration or install GPU drivers, then try again.", guiEnv.GPUInfo.Model))
			return
		}
	}

	a := app.NewWithID("com.leaktechnologies.videotools")
	w := a.NewWindow("VideoTools")
	if subIconsFS, err := fs.Sub(iconsFS, "assets/icons"); err == nil {
		ui.SetIconsFS(subIconsFS)
	}
	if subFlagsFS, err := fs.Sub(flagsFS, "assets/flags"); err == nil {
		ui.SetFlagsFS(subFlagsFS)
	}
	ui.SetMonoFontData(ibmPlexMonoRegular, ibmPlexMonoItalic, ibmPlexMonoBold, ibmPlexMonoBoldItalic)
	ui.SetAboriginalFontData(aboriginalSansRegular, aboriginalSansItalic, aboriginalSansBold, aboriginalSansBoldItalic)
	smpte.SetVCRFont(vcrOSDMono)

	a.Settings().SetTheme(&ui.VTTheme{})
	// Pre-loop flush: clear any NotoSans cache entries built before SetTheme.
	fontutil.ClearFontCache()

	// In-loop flush: SetOnStarted fires inside runGL() after Fyne registers its
	// settings listener (which clears the font cache and refreshes all widgets).
	// Re-applying the theme here triggers that listener so the first render always
	// sees IBM Plex Mono for every text style, not stale DefaultTheme entries.
	a.Lifecycle().SetOnStarted(func() {
		logging.Info(logging.CatUI, "startup: SetOnStarted — applying theme to seed font cache")
		logging.Sync()
		a.Settings().SetTheme(a.Settings().Theme())
		logging.Info(logging.CatUI, "startup: SetOnStarted complete — render loop will start on next tick")
		logging.Sync()
	})

	fontutil.SetFontCacheDebugCallback(func(styleName, fontResourceName string) {
		logging.Info(logging.CatUI, "font-cache[%s] → %s", styleName, fontResourceName)
	})

	// Load app icon from embedded logo assets.
	// Use PNG on all platforms — Fyne decodes PNG natively and correctly.
	// The .ico file is kept for windres/exe embedding via scripts/videotools.rc only.
	iconPath := "assets/logo/VT_logo.png"
	if f, err := logoAssets.Open(iconPath); err == nil {
		iconData, _ := io.ReadAll(f)
		f.Close()
		if len(iconData) > 0 {
			iconRes := fyne.NewStaticResource("VT_logo.png", iconData)
			a.SetIcon(iconRes)
			w.SetIcon(iconRes)
			logging.Debug(logging.CatUI, "app icon loaded from embedded resources")
		}
	} else if iconRes := utils.LoadAppIcon(); iconRes != nil {
		// Fallback to file-based loading for development
		a.SetIcon(iconRes)
		w.SetIcon(iconRes)
		logging.Debug(logging.CatUI, "app icon loaded from file")
	} else {
		logging.Debug(logging.CatUI, "app icon not found; continuing without custom icon")
	}

	// Remove any leftover VideoTools temp files from previous sessions that
	// crashed or were force-killed before cleanup could run.
	go sweepOrphanTempFiles()

	// Bootstrap FFmpeg DLLs for native media engine.
	// Failure is non-fatal — the player degrades gracefully when DLLs are absent.
	if HasNativeMediaPlayer() {
		if err := appcfg.AddFFmpegDllsToPath(); err != nil {
			logging.Warning(logging.CatSystem, "FFmpeg DLLs not found — video player will be unavailable: %v", err)
		} else {
			logging.Info(logging.CatSystem, "FFmpeg DLLs ready for native media engine")
			// Pre-warm the shared audio context during startup so the first video
			// load doesn't block on WASAPI/oto initialisation.
			go func() {
				if _, err := media.GetSharedAudioContext(); err != nil {
					logging.Warning(logging.CatPlayer, "audio context pre-warm failed: %v", err)
				} else {
					logging.Info(logging.CatPlayer, "audio context ready")
				}
			}()
		}
	}

	// Adaptive window sizing for professional cross-resolution support
	w.SetFixedSize(false) // Allow manual resizing and maximizing

	// Use GUI environment to determine optimal window size
	optimalSize := guiEnv.GetOptimalWindowSize(800, 600)
	w.Resize(optimalSize)
	w.CenterOnScreen()

	// Log GPU acceleration support
	if guiEnv.SupportsGPUAcceleration() {
		logging.Debug(logging.CatUI, "GPU acceleration should be available on %s %s", guiEnv.GPUInfo.Vendor, guiEnv.GPUInfo.Model)
	} else {
		logging.Debug(logging.CatUI, "GPU acceleration may not be available - using software rendering")
	}

	logging.Debug(logging.CatUI, "window initialized at %v (auto-detected environment), manual resizing enabled", optimalSize)

	// Initialize audio module - load persisted config or use defaults
	audioDefaults, err := loadAudioConfig()
	if err != nil {
		logging.Debug(logging.CatSystem, "failed to load audio config, using defaults: %v", err)
		audioDefaults = defaultAudioConfig()
	}

	state := &appState{
		window:              w,
		convert:             defaultConvertConfig(),
		mergeChapters:       true,
		player:              player.New(),
		playerVolume:        100,
		lastVolume:          100,
		playerMuted:         false,
		playerPaused:        true,
		audioOutputFormat:   audioDefaults.OutputFormat,
		audioQuality:        audioDefaults.Quality,
		audioBitrate:        audioDefaults.Bitrate,
		audioNormalize:      audioDefaults.Normalize,
		audioNormTargetLUFS: audioDefaults.NormTargetLUFS,
		audioNormTruePeak:   audioDefaults.NormTruePeak,
		audioOutputDir:      audioDefaults.OutputDir,
		audioSelectedTracks: make(map[int]bool),
		recentFiles:         recentfiles.New(),
		// Filter defaults (must be set to avoid grey video)
		filterBrightness:    0,
		filterContrast:      1.0,
		filterSaturation:    1.0,
		// Application Preferences defaults
		defaultOutputDir:     "",
		defaultVideoCodec:    "libx264",
		defaultAudioCodec:    "aac",
		hardwareAcceleration: "auto",
		uiTheme:              "Dark",
		autoPreview:          true,
	}

	initLocale(a, func() {
		if state.active == "" {
			state.showMainMenu()
		} else {
			state.showModule(state.active)
		}
	})

	if rec, err := loadConvertRecovery(); err == nil && rec.Active {
		msg := fmt.Sprintf("A conversion was running when VideoTools last closed.\n\nInput: %s\nOutput: %s", rec.Input, rec.Output)
		if strings.TrimSpace(rec.LogPath) != "" {
			msg += fmt.Sprintf("\nLog: %s", rec.LogPath)
		}
		dialog.ShowInformation("Conversion Recovery", msg, w)
	}

	if cfg, err := loadPersistedConvertConfig(); err == nil {
		state.convert = cfg
		// Ensure FrameRate defaults to Source if not explicitly set
		if state.convert.FrameRate == "" {
			state.convert.FrameRate = "Source"
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		logging.Error(logging.CatConvert, "failed to load persisted convert config: err=%v", err)
	}

	if prefs, err := loadPrefsConfig(); err == nil {
		state.prefs = prefs
		state.defaultOutputDir = prefs.DefaultOutputDir
		ui.ShowTooltips = prefs.ShowTooltips
		setHWDecodeEnabled(prefs.HWDecodeEnabled)
		logging.SetVerboseDisc(prefs.VerboseDiscLogging)
		ui.SetFontSizePreference(prefs.FontSize)
	} else if !errors.Is(err, os.ErrNotExist) {
		logging.Debug(logging.CatSystem, "failed to load persisted prefs: %v", err)
	}

	if HasNativeMediaPlayer() {
		initNativeMediaAssets(state)
	}
	utils.SetTempDir(state.convert.TempDir)

	// Initialize user-defined encoding presets
	if presets, err := loadUserPresets(); err == nil {
		state.userPresets = presets
	} else {
		state.userPresets = []userPreset{}
		logging.Debug(logging.CatSystem, "failed to load user presets: %v", err)
	}

	// Initialize conversion history
	if historyCfg, err := loadHistoryConfig(); err == nil {
		state.historyEntries = historyCfg.Entries
	} else {
		state.historyEntries = []ui.HistoryEntry{}
		logging.Debug(logging.CatSystem, "failed to load history config: %v", err)
	}
	state.sidebarVisible = false

	// Initialize conversion stats bar
	state.statsBar = ui.NewConversionStatsBar(func() {
		// Clicking the stats bar opens the queue view
		state.showQueue()
	})

	// Initialize job queue
	state.jobQueue = queue.New(state.jobExecutor)

	state.jobQueue.SetChangeCallback(func() {
		app := fyne.CurrentApp()
		if app == nil || app.Driver() == nil {
			return
		}
		app.Driver().DoFromGoroutine(func() {
			historyCount := len(state.historyEntries)
			// Add completed jobs to history
			jobs := state.jobQueue.List()
			for _, job := range jobs {
				if job.Status == queue.JobStatusCompleted ||
					job.Status == queue.JobStatusFailed ||
					job.Status == queue.JobStatusCancelled {
					state.addToHistory(job)
				}
			}

			state.updateStatsBar()
			state.updateQueueButtonLabel()
			if state.active == "queue" {
				state.refreshQueueView()
			}
			// PERFORMANCE FIX: Only rebuild main menu if history ACTUALLY changed
			// This prevents constant rebuilds on every queue progress update
			if state.active == "mainmenu" && state.sidebarVisible && len(state.historyEntries) != historyCount {
				// Only refresh sidebar, not entire menu (much faster)
				state.refreshMainMenuSidebar()
			}
		}, false)
	})

	defer state.shutdown()
	w.SetOnDropped(func(pos fyne.Position, items []fyne.URI) {
		defer logging.RecoverPanic()
		logging.Info(logging.CatUI, "Drop event: pos=%v itemCount=%d", pos, len(items))
		state.handleDrop(pos, items)
	})
	state.showMainMenu()
	startAutoUpdateChecker(state)
	state.maybePromptWindowsDependencyBootstrap()
	logging.Debug(logging.CatUI, "main menu rendered with %d modules", len(modulesList))

	// Start stats bar update loop on a timer
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			app := fyne.CurrentApp()
			if app != nil && app.Driver() != nil {
				app.Driver().DoFromGoroutine(func() {
					state.updateStatsBar()
				}, false)
			}
		}
	}()

	logging.Info(logging.CatUI, "startup: ShowAndRun — window showing, entering GL event loop")
	logging.Sync()
	w.Show()
	w.RequestFocus()
	w.ShowAndRun()
}

func runCLI(args []string) error {
	cmd := strings.ToLower(args[0])
	cmdArgs := args[1:]
	logging.Debug(logging.CatCLI, "command=%s args=%v", cmd, cmdArgs)

	switch cmd {
	case "convert":
		return runConvertCLI(cmdArgs)
	case "combine", "merge":
		return runCombineCLI(cmdArgs)
	case "trim":
		modules.HandleTrim(cmdArgs)
	case "filters":
		modules.HandleFilters(cmdArgs)
	case "upscale":
		modules.HandleUpscale(cmdArgs)
	case "audio":
		modules.HandleAudio(cmdArgs)
	case "thumbnail":
		modules.HandleThumbnail(cmdArgs)
	case "compare":
		modules.HandleCompare(cmdArgs)
	case "inspect":
		modules.HandleInspect(cmdArgs)
	case "logs":
		return runLogsCLI()
	case "help":
		printUsage()
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
	return nil
}

func runConvertCLI(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("convert requires input and output files (e.g. videotools convert input.avi output.mp4)")
	}
	in, out := args[0], args[1]
	logging.Debug(logging.CatFFMPEG, "convert input=%s output=%s", in, out)
	modules.HandleConvert([]string{in, out})
	return nil
}

func runCombineCLI(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("combine requires input files and an output (e.g. videotools combine clip1.mov clip2.wav / final.mp4)")
	}
	inputs, outputs, err := splitIOArgs(args)
	if err != nil {
		return err
	}
	if len(inputs) == 0 || len(outputs) == 0 {
		return fmt.Errorf("combine expects one or more inputs, '/', then an output file")
	}
	logging.Debug(logging.CatFFMPEG, "combine inputs=%v output=%v", inputs, outputs)
	// For now feed inputs followed by outputs to the merge handler.
	modules.HandleMerge(append(inputs, outputs...))
	return nil
}

func splitIOArgs(args []string) (inputs []string, outputs []string, err error) {
	sep := -1
	for i, a := range args {
		if a == "/" {
			sep = i
			break
		}
	}
	if sep == -1 {
		return nil, nil, fmt.Errorf("missing '/' separator between inputs and outputs")
	}
	inputs = append(inputs, args[:sep]...)
	outputs = append(outputs, args[sep+1:]...)
	return inputs, outputs, nil
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  videotools convert <input> <output>")
	fmt.Println("  videotools combine <in1> <in2> ... / <output>")
	fmt.Println("  videotools trim <args>")
	fmt.Println("  videotools filters <args>")
	fmt.Println("  videotools upscale <args>")
	fmt.Println("  videotools audio <args>")
	fmt.Println("  videotools thumbnail <args>")
	fmt.Println("  videotools compare <file1> <file2>")
	fmt.Println("  videotools inspect <args>")
	fmt.Println("  videotools logs                 # tail recent log lines")
	fmt.Println("  videotools            # launch GUI")
	fmt.Println()
	fmt.Println("Set VIDEOTOOLS_DEBUG=1 or pass -debug for verbose logs.")
	fmt.Println("Logs are written to", logging.FilePath(), "or set VIDEOTOOLS_LOG_FILE to override.")
}

func runLogsCLI() error {
	path := logging.FilePath()
	if path == "" {
		return fmt.Errorf("log file unavailable")
	}
	logging.Debug(logging.CatCLI, "reading logs from %s", path)
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	const maxLines = 200
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	fmt.Printf("--- showing last %d log lines from %s ---\n", len(lines), path)
	for _, line := range lines {
		fmt.Println(line)
	}
	return nil
}

func (s *appState) executeAddToQueue() {
	// Add to end of queue
	if err := s.addConvertToQueue(false); err != nil {
		dialog.ShowError(err, s.window)
	} else {
		// Update queue button to show new count
		s.updateQueueButtonLabel()
		// Auto-start queue if not already running
		if s.jobQueue != nil && !s.jobQueue.IsRunning() && !s.convertBusy {
			s.jobQueue.Start()
			logging.Debug(logging.CatQueue, "queue auto-started after adding job")
		}
	}
}

func (s *appState) executeAddAllToQueue() {
	count, err := s.addAllConvertToQueue()
	if err != nil {
		dialog.ShowError(err, s.window)
	} else {
		// Update queue button to show new count
		s.updateQueueButtonLabel()
		logging.Debug(logging.CatQueue, "Added %d jobs to queue", count)
		// Auto-start queue if not already running
		if s.jobQueue != nil && !s.jobQueue.IsRunning() && !s.convertBusy {
			s.jobQueue.Start()
			logging.Debug(logging.CatQueue, "queue auto-started after adding %d jobs", count)
		}
	}
}

func (s *appState) executeConversion() {
	// Add job to queue (at top if queue is already running)
	if err := s.addConvertToQueue(true); err != nil {
		dialog.ShowError(err, s.window)
		return
	}

	// Start the queue if not already running
	if s.jobQueue != nil && !s.jobQueue.IsRunning() {
		s.jobQueue.Start()
		logging.Debug(logging.CatQueue, "started queue from Convert Now")
	}

	// Clear the loaded video from convert module
	s.clearVideo()

	s.showQueue()
}

// buildFormatBadge creates a color-coded badge for a format option
// Example: "MKV (AV1)"  teal badge with "MKV (AV1)" text
func buildFormatBadge(formatLabel string) fyne.CanvasObject {
	// Parse format label: "MKV (AV1)"  containerName: "mkv"
	parts := strings.Split(formatLabel, " (")
	if len(parts) < 1 {
		return widget.NewLabel(formatLabel)
	}

	containerName := strings.ToLower(strings.TrimSpace(parts[0]))

	// Get container color - use special color for Remux
	var badgeColor color.Color
	if strings.Contains(strings.ToLower(formatLabel), "remux") {
		badgeColor = ui.ColorRemux
	} else {
		badgeColor = ui.GetContainerColor(containerName)
	}

	// Create colored background
	bg := canvas.NewRectangle(badgeColor)
	bg.CornerRadius = 4
	bg.SetMinSize(fyne.NewSize(120, 32))

	// Create label
	label := canvas.NewText(formatLabel, color.White)
	label.TextStyle = fyne.TextStyle{Bold: true}
	label.Alignment = fyne.TextAlignCenter
	label.TextSize = 13

	// Stack background and label
	return container.NewMax(bg, container.NewCenter(label))
}

// buildVideoCodecBadge creates a color-coded badge for a video codec
func buildVideoCodecBadge(codecName string) fyne.CanvasObject {
	codecLower := strings.ToLower(strings.TrimSpace(codecName))

	// Get codec color
	badgeColor := ui.GetVideoCodecColor(codecLower)

	// Create colored background
	bg := canvas.NewRectangle(badgeColor)
	bg.CornerRadius = 4
	bg.SetMinSize(fyne.NewSize(72, 22))

	// Create label
	label := canvas.NewText(codecName, color.White)
	label.TextStyle = fyne.TextStyle{Bold: true}
	label.Alignment = fyne.TextAlignCenter
	label.TextSize = 11

	// Stack background and label
	return container.NewMax(bg, container.NewCenter(label))
}

// buildAudioCodecBadge creates a color-coded badge for an audio codec
func buildAudioCodecBadge(codecName string) fyne.CanvasObject {
	codecLower := strings.ToLower(strings.TrimSpace(codecName))

	// Get codec color
	badgeColor := ui.GetAudioCodecColor(codecLower)

	// Create colored background
	bg := canvas.NewRectangle(badgeColor)
	bg.CornerRadius = 4
	bg.SetMinSize(fyne.NewSize(72, 22))

	// Create label
	label := canvas.NewText(codecName, color.White)
	label.TextStyle = fyne.TextStyle{Bold: true}
	label.Alignment = fyne.TextAlignCenter
	label.TextSize = 11

	// Stack background and label
	return container.NewMax(bg, container.NewCenter(label))
}

func buildConvertView(state *appState, src *videoSource) fyne.CanvasObject {
	t := i18n.T()
	convertColor := moduleColor("convert")
	navyBlue := utils.MustHex("#191F35")

	buildConvertBox := func(title string, content fyne.CanvasObject) *fyne.Container {
		bg := canvas.NewRectangle(navyBlue)
		bg.CornerRadius = 10
		bg.StrokeColor = gridColor
		bg.StrokeWidth = 1

		// Colored section header bar — uses the module color so sections are
		// visually distinct and consistent with the rest of the module's chrome.
		headerBg := canvas.NewRectangle(convertColor)
		headerBg.CornerRadius = 10
		headerBg.SetMinSize(fyne.NewSize(0, 34))
		headerTitle := canvas.NewText(strings.ToUpper(title), color.White)
		headerTitle.TextStyle = fyne.TextStyle{Bold: true}
		headerTitle.TextSize = 12
		header := container.NewMax(
			headerBg,
			container.NewPadded(container.NewHBox(headerTitle, layout.NewSpacer())),
		)

		body := container.NewBorder(header, nil, nil, nil, container.NewPadded(content))
		layers := ui.NoisyBackgroundObjects(bg)
		layers = append(layers, body)
		return container.NewMax(layers...)
	}

	sectionGap := func() fyne.CanvasObject {
		gap := canvas.NewRectangle(color.Transparent)
		gap.SetMinSize(fyne.NewSize(0, 10))
		return gap
	}

	// Convert UI State Manager - eliminates sync boolean flags and widget duplication
	type convertUIState struct {
		// Quality
		quality         string
		qualityWidgets  []*ui.ColoredSelect
		onQualityChange []func(string)

		// Resolution
		resolution         string
		resolutionWidgets  []*widget.Select
		onResolutionChange []func(string)

		// Aspect Ratio
		aspect         string
		aspectWidgets  []*widget.Select
		onAspectChange []func(string)

		// Bitrate Preset
		bitratePreset         string
		bitratePresetWidgets  []*widget.Select
		onBitratePresetChange []func(string)

		// Callbacks for state updates
		updateEncodingControls    func()
		updateAspectBoxVisibility func()
		buildCommandPreview       func() fyne.CanvasObject
	}

	uiState := &convertUIState{
		quality:       state.convert.Quality,
		resolution:    state.convert.TargetResolution,
		aspect:        state.convert.OutputAspect,
		bitratePreset: state.convert.BitratePreset,
	}

	sourceAspectLabel := "Source"
	if src != nil {
		if aspectDesc := src.AspectRatioString(); aspectDesc != "--" {
			sourceAspectLabel = fmt.Sprintf("Source (%s)", aspectDesc)
		}
	}
	customAspectLabel := "Custom..."
	isStandardAspect := utils.IsStandardAspect
	customAspectActive := false
	customAspectValue := ""
	var updateCustomAspectUI func()
	aspectLabelForValue := func(val string) string {
		if strings.EqualFold(val, "source") || val == "" {
			return sourceAspectLabel
		}
		if isStandardAspect(val) {
			return val
		}
		return customAspectLabel
	}
	aspectValueForLabel := func(label string) string {
		if label == sourceAspectLabel {
			return "Source"
		}
		if label == customAspectLabel {
			return strings.TrimSpace(state.convert.OutputAspect)
		}
		return label
	}

	// Debouncing helper (use utils package)
	createDebouncedCallback := utils.CreateDebouncedCallback

	// Input validation helpers (use utils package)
	validateCRF := utils.ValidateCRF
	validateBitrate := utils.ValidateBitrate
	validateFileSize := utils.ValidateFileSize

	// Callback registry - eliminates nil checks and provides logging
	type callbackRegistry struct {
		callbacks map[string]func()
	}

	callbacks := &callbackRegistry{
		callbacks: make(map[string]func()),
	}

	registerCallback := func(name string, fn func()) {
		callbacks.callbacks[name] = fn
		logging.Debug(logging.CatUI, "registered callback: %s", name)
	}

	callCallback := func(name string) {
		if fn, exists := callbacks.callbacks[name]; exists {
			fn()
		} else {
			logging.Debug(logging.CatUI, "callback not registered: %s", name)
		}
	}

	// Suppress unused warning - will be used when we replace nil checks
	_ = registerCallback

	manualQualityOption := "Manual (CRF)"
	stateMgr := statepkg.NewStateManager(state.convert.Quality, state.convert.BitrateMode, manualQualityOption)
	var crfEntry *widget.Entry
	var manualCrfRow *fyne.Container
	var crfContainer *fyne.Container
	var manualCrfLabel *widget.Label
	var twoPassCheck *widget.Check
	var twoPassNote *widget.Label

	normalizeBitrateMode := utils.NormalizeBitrateMode

	// State setters with automatic widget synchronization
	applyQuality := func(val string) {
		if uiState.quality == val {
			return // No change
		}
		uiState.quality = val
		state.convert.Quality = val
		if val == manualQualityOption {
			if strings.TrimSpace(state.convert.CRF) == "" {
				state.convert.CRF = "23"
				if crfEntry != nil {
					crfEntry.SetText("23")
				}
			}
			if normalizeBitrateMode(state.convert.BitrateMode) == "CRF" {
				if manualCrfRow != nil {
					manualCrfRow.Show()
				}
				if crfEntry != nil {
					crfEntry.Enable()
				}
				if crfContainer != nil {
					crfContainer.Show()
				}
			}
		} else {
			if state.convert.CRF != "" {
				state.convert.CRF = ""
			}
			if crfEntry != nil && crfEntry.Text != "" {
				crfEntry.SetText("")
			}
			if manualCrfRow != nil {
				manualCrfRow.Hide()
			}
		}

		// Update all registered widgets silently (no callback loops)
		for _, w := range uiState.qualityWidgets {
			w.SetSelectedSilent(val)
		}

		// Trigger callbacks
		for _, cb := range uiState.onQualityChange {
			cb(val)
		}
		callCallback("updateEncodingControls")
	}

	stateMgr.OnQualityChange(applyQuality)

	setQuality := func(val string) {
		stateMgr.SetQuality(val)
	}

	setResolution := func(val string) {
		if uiState.resolution == val {
			return
		}
		uiState.resolution = val
		state.convert.TargetResolution = val

		for _, w := range uiState.resolutionWidgets {
			w.SetSelected(val)
		}

		for _, cb := range uiState.onResolutionChange {
			cb(val)
		}
	}

	setAspect := func(val string, userSet bool) {
		if val == "" {
			val = "Source"
		}
		if uiState.aspect == val && state.convert.AspectUserSet == userSet {
			return
		}
		uiState.aspect = val
		state.convert.OutputAspect = val
		// Only mark as user-set if explicitly choosing a specific aspect ratio, not "Source"
		if userSet && !strings.EqualFold(val, "source") {
			state.convert.AspectUserSet = true
		} else if strings.EqualFold(val, "source") {
			// Selecting "Source" means use video's native aspect ratio - reset the flag
			state.convert.AspectUserSet = false
		}
		if strings.EqualFold(val, "source") || isStandardAspect(val) {
			customAspectActive = false
		} else {
			customAspectActive = true
			customAspectValue = val
		}
		if updateCustomAspectUI != nil {
			updateCustomAspectUI()
		}

		for _, w := range uiState.aspectWidgets {
			w.SetSelected(aspectLabelForValue(val))
		}

		for _, cb := range uiState.onAspectChange {
			cb(val)
		}
		if uiState.updateAspectBoxVisibility != nil {
			uiState.updateAspectBoxVisibility()
		}
		logging.Debug(logging.CatUI, "target aspect set to %s", val)
		state.persistConvertConfig()
	}

	setBitratePreset := func(val string) {
		if uiState.bitratePreset == val {
			return
		}
		uiState.bitratePreset = val
		state.convert.BitratePreset = val

		for _, w := range uiState.bitratePresetWidgets {
			w.SetSelected(val)
		}

		for _, cb := range uiState.onBitratePresetChange {
			cb(val)
		}
	}

	back := ui.MakePillButton("< "+strings.ToUpper(t.ModuleConvert), ui.BorderDim, func() {
		state.showMainMenu()
	})

	// Navigation buttons for multiple loaded videos
	var navButtons fyne.CanvasObject
	if len(state.loadedVideos) > 1 {
		prevBtn := ui.MakePillButton("- "+t.ConvertNavPrev, ui.BorderDim, func() {
			state.prevVideo()
		})
		nextBtn := ui.MakePillButton(t.ConvertNavNext+" -", ui.BorderDim, func() {
			state.nextVideo()
		})
		videoCounter := widget.NewLabel(fmt.Sprintf(t.ConvertVideoOfFmt, state.currentIndex+1, len(state.loadedVideos)))
		navButtons = container.NewHBox(prevBtn, videoCounter, nextBtn)
	} else {
		navButtons = container.NewHBox()
	}

	// Queue button to view queue
	queueBtn := ui.MakePillButton(t.ActionViewQueue, ui.BorderDim, func() {
		state.showQueue()
	})
	state.queueBtn = queueBtn
	state.updateQueueButtonLabel()

	clearCompletedBtn := ui.MakePillButton(t.ActionQueueClearCompleted, ui.BorderDim, func() {
		state.clearCompletedJobs()
	})

	var commandDrawer *widget.PopUp
	var snippetDrawer *widget.PopUp
	drawerWidth := float32(420)
	drawerInset := float32(8)
	buildDrawer := func(title string, body fyne.CanvasObject, onClose func()) *widget.PopUp {
		closeBtn := ui.MakePillButton("-", ui.BorderDim, func() {
			if onClose != nil {
				onClose()
			}
		})
		header := container.NewBorder(nil, nil,
			widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			closeBtn,
		)
		bodyScroll := container.NewVScroll(body)
		bodyScroll.SetMinSize(fyne.NewSize(0, 200))
		panel := container.NewBorder(header, nil, nil, nil, bodyScroll)

		bg := canvas.NewRectangle(utils.MustHex("#13182B"))
		bg.CornerRadius = 10
		bg.StrokeColor = gridColor
		bg.StrokeWidth = 1
		drawer := container.NewMax(bg, container.NewPadded(panel))

		pop := widget.NewPopUp(drawer, state.window.Canvas())
		canvasSize := state.window.Canvas().Size()
		height := canvasSize.Height - (drawerInset * 2)
		if height < 220 {
			height = 220
		}
		pop.Resize(fyne.NewSize(drawerWidth, height))
		pop.ShowAtPosition(fyne.NewPos(canvasSize.Width-drawerWidth-drawerInset, drawerInset))
		return pop
	}

	// Bottom drawer for snippet options (slides up from below the button)
	buildBottomDrawer := func(title string, body fyne.CanvasObject, onClose func()) *widget.PopUp {
		closeBtn := ui.MakePillButton("x", ui.BorderDim, func() {
			if onClose != nil {
				onClose()
			}
		})
		header := container.NewBorder(nil, nil,
			widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			closeBtn,
		)
		bodyScroll := container.NewVScroll(body)
		bodyScroll.SetMinSize(fyne.NewSize(0, 180))
		panel := container.NewBorder(header, nil, nil, nil, bodyScroll)

		bg := canvas.NewRectangle(utils.MustHex("#13182B"))
		bg.CornerRadius = 10
		bg.StrokeColor = gridColor
		bg.StrokeWidth = 1
		drawer := container.NewMax(bg, container.NewPadded(panel))

		pop := widget.NewPopUp(drawer, state.window.Canvas())
		canvasSize := state.window.Canvas().Size()
		drawerHeight := float32(220)
		drawerWidth := canvasSize.Width - (drawerInset * 2)
		pop.Resize(fyne.NewSize(drawerWidth, drawerHeight))
		// Position: drawer slides UP from below. Top edge should be above the snippet row.
		// snippetRow is ~32px high in the footer. Position drawer so its bottom aligns
		// with the bottom of the button area (above stats bar).
		statsBarHeight := float32(40)
		footerRowHeight := float32(32)
		drawerTop := canvasSize.Height - drawerHeight - drawerInset - statsBarHeight - footerRowHeight
		pop.ShowAtPosition(fyne.NewPos(drawerInset, drawerTop))
		return pop
	}

	toggleDrawer := func(active **widget.PopUp, title string, body fyne.CanvasObject) {
		if *active != nil {
			(*active).Hide()
			*active = nil
			return
		}
		*active = buildDrawer(title, body, func() {
			if *active != nil {
				(*active).Hide()
				*active = nil
			}
		})
	}

	// Forward declare drawer-backed preview builder.
	var buildCommandPreview func() fyne.CanvasObject

	// Command Preview toggle button (drawer)
	cmdPreviewBtn := ui.MakePillButton(t.ConvertCommandPreview, ui.BorderDim, func() {
		if src == nil {
			return
		}
		body := buildCommandPreview()
		toggleDrawer(&commandDrawer, t.ConvertCommandPreview, body)
	})

	// Update button text and state based on preview visibility and source
	if src == nil {
		cmdPreviewBtn.Disable()
	}

	// mainSplit declared here so the settings panel onToggle callback can capture
	// it before the split container is created further down.
	var mainSplit *container.Split

	// Build back bar
	backBarItems := []fyne.CanvasObject{
		back,
		layout.NewSpacer(),
		navButtons,
		layout.NewSpacer(),
		cmdPreviewBtn,
		clearCompletedBtn,
		queueBtn,
	}

	backBar := ui.TintedBar(convertColor, container.NewHBox(backBarItems...))

	var updateCover func(string)
	var coverDisplay *widget.Label
	var updateMetaCover func()
	coverLabel := widget.NewLabel(state.convert.CoverLabel())
	updateCover = func(path string) {
		if strings.TrimSpace(path) == "" {
			return
		}
		state.convert.CoverArtPath = path
		coverLabel.SetText(state.convert.CoverLabel())
		if coverDisplay != nil {
			coverDisplay.SetText(t.ConvertCoverArtLabel + ": " + state.convert.CoverLabel())
		}
		if updateMetaCover != nil {
			updateMetaCover()
		}
	}

	// Make panel sizes responsive with modest minimums to avoid forcing the window beyond the screen.
	// Video pane uses VSplit with metadata at 50% - use moderate minimum for scaling.
	videoPanel := buildVideoPane(state, fyne.NewSize(480, 270), src, updateCover)

	// leftColumn declared here so both the player and metadata onToggle callbacks
	// can capture it before the split container is created further down.
	var leftColumn *container.Split

	playerHeader, _ := ui.BuildCollapsibleHeader(t.ConvertSectionPlayer, convertColor, func(open bool) {
		if open {
			leftColumn.SetOffset(0.5)
		} else {
			leftColumn.SetOffset(0.03)
		}
	})
	videoPanelWithHeader := container.NewBorder(playerHeader, nil, nil, nil, videoPanel)

	metaPanel, metaCoverUpdate := buildMetadataPanel(state, src, fyne.NewSize(0, 200), convertColor, func(open bool) {
		if open {
			leftColumn.SetOffset(0.5)
		} else {
			leftColumn.SetOffset(0.97)
		}
	})
	updateMetaCover = metaCoverUpdate

	// Forward declare functions needed by formatContainer callback
	var updateDVDOptions func()
	var updateChapterWarning func()
	var updateQualityOptions func()
	var updateQualityVisibility func()
	var updateRemuxVisibility func()
	var updateEncodingControls func()

	// Declare output widgets early to fix variable order issues
	var outputExtLabel *widget.Label
	var outputExtBG *canvas.Rectangle
	var updateOutputHint func()
	var videoCodecSelect *ui.ColoredSelect
	var audioCodecSelect *ui.ColoredSelect
	var normalizeAudioCheck *widget.Check
	var profileLevelContainer *fyne.Container
	var tuneContainer *fyne.Container
	var tuneSelect *widget.Select

	var formatLabels []string
	for _, opt := range formatOptions {
		label := opt.Label
		if opt.Legacy {
			label += " (legacy)"
		}
		formatLabels = append(formatLabels, label)
	}

	// Format selector
	formatColors := ui.BuildFormatColorMap(formatLabels)
	var syncingFormat bool
	applySelectedFormat := func(opt formatOption) {
		state.convert.SelectedFormat = opt
		logging.Debug(logging.CatUI, "format selected: %s", opt.Label)
		friendlyCodec := friendlyCodecFromPreset(opt.VideoCodec)
		if opt.VideoCodec == "copy" {
			friendlyCodec = "Copy"
		}
		if friendlyCodec != "" && state.convert.VideoCodec != friendlyCodec {
			state.convert.VideoCodec = friendlyCodec
			if videoCodecSelect != nil {
				videoCodecSelect.SetSelected(friendlyCodec)
			}
			if updateQualityOptions != nil {
				updateQualityOptions()
			}
			if updateQualityVisibility != nil {
				updateQualityVisibility()
			}
			if updateRemuxVisibility != nil {
				updateRemuxVisibility()
			}
		}

		// Update codec compatibility based on format
		ext := strings.ToLower(opt.Ext)
		compatibleVideo := convert.FormatVideoCodecs[ext]
		compatibleAudio := convert.FormatAudioCodecs[ext]

		// Update video codec select - grey out incompatible codecs
		if videoCodecSelect != nil {
			videoCodecSelect.EnableAllOptions()
			allVideoCodecs := []string{"H.264", "H.265", "VP9", "AV1", "MPEG-2", "Copy"}
			for _, codec := range allVideoCodecs {
				isCompatible := false
				for _, c := range compatibleVideo {
					if codec == c {
						isCompatible = true
						break
					}
				}
				if !isCompatible {
					videoCodecSelect.DisableOption(codec)
				}
			}
			// If current selection is now incompatible, switch to first compatible
			if !slices.Contains(compatibleVideo, state.convert.VideoCodec) && len(compatibleVideo) > 0 {
				state.convert.VideoCodec = compatibleVideo[0]
				videoCodecSelect.SetSelected(compatibleVideo[0])
			}
		}

		// Update audio codec select - grey out incompatible codecs
		if audioCodecSelect != nil {
			audioCodecSelect.EnableAllOptions()
			allAudioCodecs := []string{"AAC", "AC-3", "Opus", "Vorbis", "MP3", "FLAC", "Copy", "MP2"}
			for _, codec := range allAudioCodecs {
				isCompatible := false
				for _, c := range compatibleAudio {
					if codec == c {
						isCompatible = true
						break
					}
				}
				if !isCompatible {
					audioCodecSelect.DisableOption(codec)
				}
			}
			// If current selection is now incompatible, switch to first compatible
			if !slices.Contains(compatibleAudio, state.convert.AudioCodec) && len(compatibleAudio) > 0 {
				state.convert.AudioCodec = compatibleAudio[0]
				audioCodecSelect.SetSelected(compatibleAudio[0])
			}
		}

		if updateDVDOptions != nil {
			updateDVDOptions()
		}
		if outputExtLabel != nil {
			outputExtLabel.SetText(state.convert.SelectedFormat.Ext)
		}
		if outputExtBG != nil {
			outputExtBG.FillColor = ui.GetContainerColor(strings.TrimPrefix(state.convert.SelectedFormat.Ext, "."))
			outputExtBG.Refresh()
		}
		if updateOutputHint != nil {
			updateOutputHint()
		}
		if updateChapterWarning != nil {
			updateChapterWarning()
		}
		if buildCommandPreview != nil {
			buildCommandPreview()
		}

	}

	formatContainer := ui.NewColoredSelectWithTooltip(formatLabels, formatColors, func(selected string) {
		if syncingFormat {
			return
		}
		for _, opt := range formatOptions {
			if opt.Label == selected {
				applySelectedFormat(opt)
				break
			}
		}
	}, state.window, t.TooltipConvertFormat)
	formatContainer.SetSelected(state.convert.SelectedFormat.Label)

	getOutputDir := func() string {
		if strings.TrimSpace(state.convert.OutputDir) != "" {
			return state.convert.OutputDir
		}
		if strings.TrimSpace(state.defaultOutputDir) != "" {
			return state.defaultOutputDir
		}
		if root := defaultVideoToolsRoot(); root != "" {
			return root
		}
		if src != nil {
			return filepath.Dir(src.Path)
		}
		return ""
	}

	getOutputPathPreview := func() string {
		outDir := getOutputDir()
		if outDir == "" {
			return state.convert.OutputFile()
		}
		return filepath.Join(outDir, state.convert.OutputFile())
	}

	outputHint := widget.NewLabel(fmt.Sprintf(t.ConvertOutputFileFmt, getOutputPathPreview()))
	outputHint.Wrapping = fyne.TextWrapWord
	// Wrap hint in padded container to ensure proper text wrapping in narrow windows
	outputHintContainer := container.NewPadded(outputHint)

	updateOutputHint = func() {
		outputHint.SetText(fmt.Sprintf(t.ConvertOutputFileFmt, getOutputPathPreview()))
		if strings.TrimSpace(state.convert.OutputBase) == "" {
			outputHintContainer.Show()
		} else {
			outputHintContainer.Hide()
		}
	}
	updateOutputHint()

	// DVD-specific aspect ratio selector (only shown for DVD formats)
	dvdAspectOpts := []string{"4:3", "16:9"}
	dvdAspectSelect := widget.NewSelect(dvdAspectOpts, func(value string) {
		logging.Debug(logging.CatUI, "DVD aspect set to %s", value)
		state.convert.OutputAspect = value
	})
	dvdAspectLabel := widget.NewLabelWithStyle(t.ConvertSectionDVDApect, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	// DVD info label showing specs based on format selected
	dvdInfoLabel := widget.NewLabel("")
	dvdInfoLabel.Wrapping = fyne.TextWrapWord
	dvdInfoLabel.Alignment = fyne.TextAlignLeading

	dvdAspectBox := container.NewVBox(dvdAspectLabel, dvdAspectSelect, dvdInfoLabel)
	dvdAspectBox.Hide() // Hidden by default

	// Chapter preservation
	preserveChaptersCheck := widget.NewCheck(t.ConvertKeepChapters, func(checked bool) {
		state.convert.PreserveChapters = checked
	})
	preserveChaptersCheck.SetChecked(state.convert.PreserveChapters)

	// Forward declarations for encoding controls (used in reset/update callbacks)
	var (
		bitrateModeRadio          *widget.RadioGroup
		bitratePresetSelect        *widget.Select
		videoBitrateEntry          *widget.Entry
		manualBitrateRow           *fyne.Container
		targetFileSizeSelect       *widget.Select
		targetFileSizeEntry        *widget.Entry
		qualitySelectSimple        *ui.ColoredSelect
		qualitySelectAdv           *ui.ColoredSelect
		qualitySectionSimple       fyne.CanvasObject
		qualitySectionAdv          fyne.CanvasObject
		simpleBitrateSelect        *widget.Select
		bitrateContainer           *fyne.Container
		targetSizeContainer        *fyne.Container
		resetConvertDefaults       func()
		tabs                       *container.AppTabs
		simpleEncodingSection      *fyne.Container
		advancedVideoEncodingBlock *fyne.Container
		audioEncodingSection       *fyne.Container
		applyDevicePreset          func(hwPreset)
		applyUserPreset            func(userPreset)
	)
	// Device preset selector — applyDevicePreset is in the var block above and assigned later
	// once all encode widgets are constructed. The closure captures it by reference.
	devicePresetLabels := []string{"None"}
	for _, dp := range hwPresets {
		devicePresetLabels = append(devicePresetLabels, dp.Label)
	}
	devicePresetSelect := widget.NewSelect(devicePresetLabels, func(val string) {
		if val == "None" || val == "" {
			return
		}
		for _, dp := range hwPresets {
			if dp.Label == val {
				if applyDevicePreset != nil {
					applyDevicePreset(dp)
				}
				break
			}
		}
	})
	devicePresetSelect.SetSelected("None")

	// User preset selector — applyUserPreset assigned after all encode widgets exist.
	buildUserPresetLabels := func() []string {
		labels := []string{"None"}
		for _, p := range state.userPresets {
			labels = append(labels, p.Name)
		}
		return labels
	}
	userPresetSelect := widget.NewSelect(buildUserPresetLabels(), func(val string) {
		if val == "None" || val == "" {
			return
		}
		for _, p := range state.userPresets {
			if p.Name == val {
				if applyUserPreset != nil {
					applyUserPreset(p)
				}
				break
			}
		}
	})
	userPresetSelect.SetSelected("None")

	deleteUserPresetBtn := ui.MakePillButton(t.ActionDelete, ui.Red, func() {
		sel := userPresetSelect.Selected
		if sel == "None" || sel == "" {
			return
		}
		updated := state.userPresets[:0]
		for _, p := range state.userPresets {
			if p.Name != sel {
				updated = append(updated, p)
			}
		}
		state.userPresets = updated
		if err := saveUserPresets(state.userPresets); err != nil {
			logging.Error(logging.CatConvert, "failed to save user presets: %v", err)
		}
		userPresetSelect.Options = buildUserPresetLabels()
		userPresetSelect.SetSelected("None")
		userPresetSelect.Refresh()
	})

	saveUserPresetBtn := ui.MakePillButton(t.ConvertSavePresetBtn, convertColor, func() {
		entry := widget.NewEntry()
		entry.SetPlaceHolder(t.ConvertPresetNamePlaceholder)
		dialog.ShowCustomConfirm(t.ConvertSavePreset, t.ActionSave, t.ActionCancel, entry, func(ok bool) {
			if !ok {
				return
			}
			name := strings.TrimSpace(entry.Text)
			if name == "" {
				return
			}
			// Replace existing preset with same name, or append.
			preset := userPresetFromConfig(name, state.convert)
			replaced := false
			for i, p := range state.userPresets {
				if p.Name == name {
					state.userPresets[i] = preset
					replaced = true
					break
				}
			}
			if !replaced {
				state.userPresets = append(state.userPresets, preset)
			}
			if err := saveUserPresets(state.userPresets); err != nil {
				logging.Error(logging.CatConvert, "failed to save user presets: %v", err)
			}
			userPresetSelect.Options = buildUserPresetLabels()
			userPresetSelect.SetSelected(name)
			userPresetSelect.Refresh()
		}, state.window)
	})

	// updateQualityOptions: Update quality dropdown based on codec

	// Base quality options (without lossless or manual)
	baseQualityOptions := []string{
		"Draft (CRF 28)",
		"Standard (CRF 23)",
		"Balanced (CRF 20)",
		"High (CRF 18)",
		"Near-Lossless (CRF 16)",
	}

	// Helper function to check if codec supports lossless
	codecSupportsLossless := func(codec string) bool {
		return codec == "H.265" || codec == "AV1"
	}

	// Current quality options (dynamic based on codec)
	qualityOptions := append([]string{}, baseQualityOptions...)
	if codecSupportsLossless(state.convert.VideoCodec) {
		qualityOptions = append(qualityOptions, "Lossless")
	}
	qualityOptions = append(qualityOptions, manualQualityOption)

	// Quality select widgets - use state manager to eliminate sync flags
	// Convert quality selects to ColoredSelect and register with state manager
	qualityColorMap := ui.BuildQualityColorMap(qualityOptions)

	qualitySelectSimple = ui.NewColoredSelectWithTooltip(qualityOptions, qualityColorMap, func(value string) {
		logging.Debug(logging.CatUI, "quality preset %s (simple)", value)
		setQuality(value)
		if buildCommandPreview != nil {
			buildCommandPreview()
		}
	}, state.window, t.TooltipConvertQuality)

	qualitySelectAdv = ui.NewColoredSelectWithTooltip(qualityOptions, qualityColorMap, func(value string) {
		logging.Debug(logging.CatUI, "quality preset %s (advanced)", value)
		setQuality(value)
		if buildCommandPreview != nil {
			buildCommandPreview()
		}
	}, state.window, t.TooltipConvertQuality)

	if !slices.Contains(qualityOptions, state.convert.Quality) {
		state.convert.Quality = "Standard (CRF 23)"
	}
	qualitySelectSimple.SetSelected(state.convert.Quality)
	qualitySelectAdv.SetSelected(state.convert.Quality)

	// Register both quality widgets with state manager for automatic synchronization
	uiState.qualityWidgets = []*ui.ColoredSelect{qualitySelectSimple, qualitySelectAdv}

	// Update quality options based on codec
	updateQualityOptions = func() {
		var newOptions []string
		if codecSupportsLossless(state.convert.VideoCodec) {
			// H.265 and AV1 support lossless
			newOptions = append([]string{}, baseQualityOptions...)
			newOptions = append(newOptions, "Lossless")
		} else {
			// H.264, MPEG-2, etc. don't support lossless
			newOptions = append([]string{}, baseQualityOptions...)
			// If currently set to Lossless, fall back to Near-Lossless
			if state.convert.Quality == "Lossless" {
				state.convert.Quality = "Near-Lossless (CRF 16)"
			}
		}
		newOptions = append(newOptions, manualQualityOption)

		// Update options and color map for all registered quality widgets
		qualityColorMap := ui.BuildQualityColorMap(newOptions)
		for _, w := range uiState.qualityWidgets {
			w.UpdateOptions(newOptions, qualityColorMap)
		}

		// Use state manager to synchronize selected value across all widgets
		setQuality(state.convert.Quality)
	}

	outputEntry := widget.NewEntry()
	if state.source != nil {
		outputEntry.SetText(state.convert.OutputBase)
	}
	var updatingOutput bool
	var autoNameCheck *widget.Check
	outputEntry.OnChanged = func(val string) {
		if updatingOutput {
			return
		}
		if state.convert.UseAutoNaming {
			state.convert.UseAutoNaming = false
			if autoNameCheck != nil {
				autoNameCheck.SetChecked(false)
			}
		}
		state.convert.OutputBase = val
		updateOutputHint()
	}

	outputDirEntry := widget.NewEntry()
	outputDirEntry.SetPlaceHolder("Output folder path")
	outputDirEntry.SetText(state.convert.OutputDir)
	outputDirEntry.OnChanged = func(val string) {
		state.convert.OutputDir = val
		updateOutputHint()
		state.persistConvertConfig()
	}

	browseOutputDir := func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, state.window)
				return
			}
			if uri == nil {
				return
			}
			state.convert.OutputDir = uri.Path()
			outputDirEntry.SetText(state.convert.OutputDir)
			updateOutputHint()
			state.persistConvertConfig()
		}, state.window)
	}

	outputDirBtnLabel := canvas.NewText("Browse", textColor)
	outputDirBtnLabel.Alignment = fyne.TextAlignCenter
	outputDirBtnLabel.TextSize = 14
	outputDirBtnBG := canvas.NewRectangle(utils.MustHex("#344256"))
	outputDirBtnBG.CornerRadius = 8
	outputDirBtnBG.SetMinSize(fyne.NewSize(92, 36))
	outputDirBtn := ui.NewTappable(container.NewMax(outputDirBtnBG, container.NewPadded(outputDirBtnLabel)), browseOutputDir)

	outputDirClearBtnLabel := canvas.NewText("Clear", textColor)
	outputDirClearBtnLabel.Alignment = fyne.TextAlignCenter
	outputDirClearBtnLabel.TextSize = 14
	outputDirClearBtnBG := canvas.NewRectangle(utils.MustHex("#344256"))
	outputDirClearBtnBG.CornerRadius = 8
	outputDirClearBtnBG.SetMinSize(fyne.NewSize(60, 36))
	outputDirClearBtn := ui.NewTappable(container.NewMax(outputDirClearBtnBG, container.NewPadded(outputDirClearBtnLabel)), func() {
		state.convert.OutputDir = ""
		outputDirEntry.SetText("")
		updateOutputHint()
		state.persistConvertConfig()
	})

	outputExtLabel = widget.NewLabel(state.convert.SelectedFormat.Ext)
	outputExtLabel.Alignment = fyne.TextAlignCenter
	outputExtBG = canvas.NewRectangle(ui.GetContainerColor(strings.TrimPrefix(state.convert.SelectedFormat.Ext, ".")))
	outputExtBG.CornerRadius = 8
	outputExtBG.SetMinSize(fyne.NewSize(72, 36))
	outputExtPill := container.NewMax(outputExtBG, container.NewPadded(outputExtLabel))

	buildOutputRow := func(entry *widget.Entry, right fyne.CanvasObject) fyne.CanvasObject {
		bg := canvas.NewRectangle(utils.MustHex("#344256"))
		bg.CornerRadius = 8
		bg.SetMinSize(fyne.NewSize(0, 36))
		row := container.NewBorder(nil, nil, nil, right, container.NewMax(entry))
		return container.NewMax(bg, container.NewPadded(row))
	}

	outputDirRow := buildOutputRow(outputDirEntry, container.NewHBox(outputDirClearBtn, outputDirBtn))
	outputNameRow := buildOutputRow(outputEntry, outputExtPill)

	applyAutoName := func(force bool) {
		if !force && !state.convert.UseAutoNaming {
			return
		}
		newBase := state.resolveOutputBase(src, false)
		updatingOutput = true
		state.convert.OutputBase = newBase
		outputEntry.SetText(newBase)
		updatingOutput = false
		updateOutputHint()
	}

	autoNameCheck = widget.NewCheck(t.ConvertAutoNameFromMeta, func(checked bool) {
		state.convert.UseAutoNaming = checked
		applyAutoName(true)
	})
	autoNameCheck.Checked = state.convert.UseAutoNaming

	autoNameTemplate := widget.NewEntry()
	autoNameTemplate.SetPlaceHolder("<actress> - <studio> - <scene>")
	autoNameTemplate.SetText(state.convert.AutoNameTemplate)

	autoNameTemplate.OnChanged = func(val string) {
		state.convert.AutoNameTemplate = val
		if state.convert.UseAutoNaming {
			applyAutoName(true)
		}
	}

	autoNameHint := widget.NewLabel(t.ConvertAutoNameHint)
	autoNameHint.Wrapping = fyne.TextWrapWord

	if state.convert.UseAutoNaming {
		applyAutoName(true)
	}

	appendSuffixCheck := widget.NewCheck(t.ConvertAppendSuffix, func(checked bool) {
		state.convert.AppendSuffix = checked
		// Recalculate and update the output base to reflect the suffix change
		// Always pass false for keepExisting to regenerate from source
		newBase := state.resolveOutputBase(src, false)
		updatingOutput = true
		state.convert.OutputBase = newBase
		outputEntry.SetText(newBase)
		updatingOutput = false
		// Update output hint to show the change immediately
		if outputHint != nil {
			updateOutputHint()
		}
	})
	appendSuffixCheck.Checked = state.convert.AppendSuffix

	inverseCheck := widget.NewCheck(t.ConvertSmartITC, func(checked bool) {
		state.convert.InverseTelecine = checked
		state.persistConvertConfig()
	})
	inverseCheck.Checked = state.convert.InverseTelecine
	inverseHint := widget.NewLabel(state.convert.InverseAutoNotes)

	// Deinterlace Mode
	deinterlaceModeOptions := []string{"Off", "Auto", "Force"}
	deinterlaceModeSelect := widget.NewSelect(deinterlaceModeOptions, func(value string) {
		state.convert.Deinterlace = value
		logging.Debug(logging.CatUI, "deinterlace mode set to %s", value)
	})
	deinterlaceModeSelect.SetSelected(state.convert.Deinterlace)

	// Deinterlace Method
	deinterlaceMethodOptions := []string{"yadif", "bwdif"}
	deinterlaceMethodSelect := widget.NewSelect(deinterlaceMethodOptions, func(value string) {
		state.convert.DeinterlaceMethod = value
		logging.Debug(logging.CatUI, "deinterlace method set to %s", value)
	})
	deinterlaceMethodSelect.SetSelected(state.convert.DeinterlaceMethod)

	makePanelButton := func(label string, onTap func()) (*ui.PillButton, fyne.CanvasObject) {
		btn := ui.MakePillButton(label, ui.BorderDim, onTap)
		bg := canvas.NewRectangle(utils.MustHex("#344256"))
		bg.CornerRadius = 8
		bg.SetMinSize(fyne.NewSize(0, 36))
		return btn, container.NewMax(bg, container.NewPadded(btn))
	}

	// Interlacing Analysis Button (Simple Menu)
	var analyzeInterlaceBtn *ui.PillButton
	var analyzeInterlaceView fyne.CanvasObject
	analyzeInterlaceBtn, analyzeInterlaceView = makePanelButton(t.ConvertAnalyzeInterlacing, func() {
		if src == nil {
			dialog.ShowInformation(t.DialogInterlacing, t.DialogLoadVideoFirst, state.window)
			return
		}
		go func() {
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				analyzeInterlaceBtn.SetText(t.ConvertAnalyzing)
				analyzeInterlaceBtn.Disable()
			}, false)

			detector := interlace.NewDetector(utils.GetFFmpegPath(), utils.GetFFprobePath())
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			result, err := detector.QuickAnalyze(ctx, src.Path)

			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				analyzeInterlaceBtn.SetText(t.ConvertAnalyzeInterlacing)
				analyzeInterlaceBtn.Enable()

				if err != nil {
					logging.Debug(logging.CatSystem, "interlacing analysis failed: %v", err)
					dialog.ShowError(fmt.Errorf("Analysis failed: %w", err), state.window)
				} else {
					state.interlaceResult = result
					logging.Debug(logging.CatSystem, "interlacing analysis complete: %s", result.Status)

					// Show results dialog
					resultText := fmt.Sprintf(
						"Status: %s\n"+
							"Interlaced Frames: %.1f%%\n"+
							"Field Order: %s\n"+
							"Confidence: %s\n\n"+
							"Recommendation:\n%s\n\n"+
							"Frame Counts:\n"+
							"Progressive: %d\n"+
							"Top Field First: %d\n"+
							"Bottom Field First: %d\n"+
							"Undetermined: %d\n"+
							"Total Analyzed: %d",
						result.Status,
						result.InterlacedPercent,
						result.FieldOrder,
						result.Confidence,
						result.Recommendation,
						result.Progressive,
						result.TFF,
						result.BFF,
						result.Undetermined,
						result.TotalFrames,
					)

					dialog.ShowInformation(t.DialogInterlacingResults, resultText, state.window)

					// Auto-update deinterlace setting
					if result.SuggestDeinterlace && state.convert.Deinterlace == "Off" {
						state.convert.Deinterlace = "Auto"
						inverseCheck.SetChecked(true)
					}
				}
			}, false)
		}()
	})

	// Auto-crop controls
	autoCropCheck := widget.NewCheck(t.ConvertAutoDetectBlackBars, func(checked bool) {
		state.convert.AutoCrop = checked
		logging.Debug(logging.CatUI, "auto-crop set to %v", checked)
		state.persistConvertConfig()
	})
	autoCropCheck.Checked = state.convert.AutoCrop

	var detectCropBtn *ui.PillButton
	var detectCropView fyne.CanvasObject
	detectCropBtn, detectCropView = makePanelButton(t.ConvertDetectCrop, func() {
		if src == nil {
			dialog.ShowInformation(t.DialogAutoCrop, t.DialogLoadVideoFirst, state.window)
			return
		}
		// Run detection in background
		go func() {
			detectCropBtn.SetText(t.ConvertDetecting)
			detectCropBtn.Disable()
			defer func() {
				detectCropBtn.SetText(t.ConvertDetectCrop)
				detectCropBtn.Enable()
			}()

			crop := detectCrop(src.Path, src.Duration)
			if crop == nil {
				dialog.ShowInformation(t.DialogAutoCrop, t.DialogNoBlackBars, state.window)
				return
			}

			// Calculate savings
			originalPixels := src.Width * src.Height
			croppedPixels := crop.Width * crop.Height
			savingsPercent := (1.0 - float64(croppedPixels)/float64(originalPixels)) * 100

			// Show detection results and apply
			message := fmt.Sprintf("Detected crop:\n\n"+
				"Original: %dx%d\n"+
				"Cropped: %dx%d (offset %d,%d)\n"+
				"Estimated file size reduction: %.1f%%\n\n"+
				"Apply these crop values?",
				src.Width, src.Height,
				crop.Width, crop.Height, crop.X, crop.Y,
				savingsPercent)

			dialog.ShowConfirm(t.DialogAutoCropDetection, message, func(apply bool) {
				if apply {
					state.convert.CropWidth = fmt.Sprintf("%d", crop.Width)
					state.convert.CropHeight = fmt.Sprintf("%d", crop.Height)
					state.convert.CropX = fmt.Sprintf("%d", crop.X)
					state.convert.CropY = fmt.Sprintf("%d", crop.Y)
					state.convert.AutoCrop = true
					autoCropCheck.SetChecked(true)
					logging.Debug(logging.CatUI, "applied detected crop: %dx%d at %d,%d", crop.Width, crop.Height, crop.X, crop.Y)
				}
			}, state.window)
		}()
	})
	if src == nil {
		detectCropBtn.Disable()
	}

	autoCropHint := widget.NewLabel(t.ConvertAutoCropHint)
	autoCropHint.Wrapping = fyne.TextWrapWord

	// Flip and Rotation controls
	flipHorizontalCheck := widget.NewCheck(t.ConvertFlipHorizontal, func(checked bool) {
		state.convert.FlipHorizontal = checked
		logging.Debug(logging.CatUI, "flip horizontal set to %v", checked)
		state.persistConvertConfig()
	})
	flipHorizontalCheck.Checked = state.convert.FlipHorizontal

	flipVerticalCheck := widget.NewCheck(t.ConvertFlipVertical, func(checked bool) {
		state.convert.FlipVertical = checked
		logging.Debug(logging.CatUI, "flip vertical set to %v", checked)
		state.persistConvertConfig()
	})
	flipVerticalCheck.Checked = state.convert.FlipVertical

	rotationOptions := []string{"0 deg", "90 deg CW", "180 deg", "270 deg CW"}
	rotationSelect := widget.NewSelect(rotationOptions, func(value string) {
		var rotation string
		switch value {
		case "0 deg":
			rotation = "0"
		case "90 deg CW":
			rotation = "90"
		case "180 deg":
			rotation = "180"
		case "270 deg CW":
			rotation = "270"
		}
		state.convert.Rotation = rotation
		logging.Debug(logging.CatUI, "rotation set to %s", rotation)
	})
	if state.convert.Rotation == "" {
		state.convert.Rotation = "0"
	}
	rotationMap := map[string]string{"0": "0 deg", "90": "90 deg CW", "180": "180 deg", "270": "270 deg CW"}
	if label, ok := rotationMap[state.convert.Rotation]; ok {
		rotationSelect.SetSelected(label)
	} else {
		rotationSelect.SetSelected("0 deg")
	}

	transformHint := widget.NewLabel(t.ConvertTransformHint)
	transformHint.Wrapping = fyne.TextWrapWord

	aspectTargets := []string{sourceAspectLabel, "16:9", "4:3", "1:1", "9:16", "21:9", customAspectLabel}
	var (
		targetAspectSelect       *widget.Select
		targetAspectSelectSimple *widget.Select
	)
	var forceAspectChecks []*widget.Check
	syncForceAspect := func(checked bool) {
		state.convert.ForceAspect = checked
		for _, c := range forceAspectChecks {
			if c.Checked != checked {
				c.SetChecked(checked)
			}
		}
		if buildCommandPreview != nil {
			buildCommandPreview()
		}
	}
	makeForceAspectCheck := func() *widget.Check {
		check := widget.NewCheck(t.ConvertForceAspectMeta, func(checked bool) {
			syncForceAspect(checked)
		})
		check.SetChecked(state.convert.ForceAspect)
		forceAspectChecks = append(forceAspectChecks, check)
		return check
	}
	if val := strings.TrimSpace(state.convert.OutputAspect); val != "" &&
		!strings.EqualFold(val, "source") &&
		!isStandardAspect(val) {
		customAspectActive = true
		customAspectValue = val
	}
	var customAspectEntries []*widget.Entry
	var customAspectBoxes []*fyne.Container
	var customAspectHintLabels []*widget.Label
	updateCustomAspectUI = func() {
		show := customAspectActive
		for _, entry := range customAspectEntries {
			if entry.Text != customAspectValue {
				entry.SetText(customAspectValue)
			}
		}
		for _, box := range customAspectBoxes {
			if show {
				box.Show()
			} else {
				box.Hide()
			}
		}
	}
	applyCustomAspect := func(val string) {
		val = strings.TrimSpace(val)
		if val == "" {
			return
		}
		if utils.ParseAspectValue(val) <= 0 {
			for _, hint := range customAspectHintLabels {
				hint.SetText(t.ConvertAspectRatioEntry)
			}
			return
		}
		for _, hint := range customAspectHintLabels {
			hint.SetText("Custom aspect ratio in use.")
		}
		customAspectValue = val
		customAspectActive = true
		setAspect(val, true)
		updateCustomAspectUI()
	}

	// Aspect select widget - uses state manager to eliminate sync flag
	targetAspectSelect = widget.NewSelect(aspectTargets, func(value string) {
		if value == customAspectLabel {
			customAspectActive = true
			updateCustomAspectUI()
			if strings.TrimSpace(customAspectValue) != "" {
				applyCustomAspect(customAspectValue)
			}
			return
		}
		customAspectActive = false
		setAspect(aspectValueForLabel(value), true)
		updateCustomAspectUI()
	})
	if state.convert.OutputAspect == "" {
		state.convert.OutputAspect = "Source"
	}
	targetAspectSelect.SetSelected(aspectLabelForValue(state.convert.OutputAspect))
	targetAspectHint := widget.NewLabel(t.ConvertTargetAspectHint)
	targetAspectHint.Wrapping = fyne.TextWrapWord
	// Wrap hint in padded container to ensure proper text wrapping in narrow windows
	targetAspectHintContainer := container.NewPadded(targetAspectHint)

	customAspectEntry := widget.NewEntry()
	customAspectEntry.SetPlaceHolder("e.g. 1.90 or 256:135")
	customAspectEntry.OnChanged = func(val string) {
		applyCustomAspect(val)
	}
	customAspectHint := widget.NewLabel(t.ConvertCustomAspectHint)
	customAspectHint.Wrapping = fyne.TextWrapWord
	customAspectBox := container.NewVBox(
		widget.NewLabelWithStyle(t.ConvertSectionCustomAspect, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		customAspectEntry,
		customAspectHint,
	)
	customAspectEntries = append(customAspectEntries, customAspectEntry)
	customAspectBoxes = append(customAspectBoxes, customAspectBox)
	customAspectHintLabels = append(customAspectHintLabels, customAspectHint)
	updateCustomAspectUI()

	aspectOptions := widget.NewRadioGroup([]string{"Auto", "Crop", "Letterbox/Pillarbox", "Blur Fill", "Stretch"}, func(value string) {
		logging.Debug(logging.CatUI, "aspect handling set to %s", value)
		state.convert.AspectHandling = value
	})
	aspectOptions.Horizontal = false
	aspectOptions.Required = true

	// Map old separate options to new combined option for backwards compatibility
	if state.convert.AspectHandling == "Letterbox" || state.convert.AspectHandling == "Pillarbox" {
		state.convert.AspectHandling = "Letterbox/Pillarbox"
	}
	aspectOptions.SetSelected(state.convert.AspectHandling)

	backgroundHint := widget.NewLabel(t.ConvertBackgroundHint)
	aspectBox := container.NewVBox(
		widget.NewLabelWithStyle(t.ConvertSectionAspectHandling, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		aspectOptions,
		backgroundHint,
	)

	updateAspectBoxVisibility := func() {
		if src == nil {
			aspectBox.Hide()
			return
		}
		target := resolveTargetAspect(state.convert.OutputAspect, src)
		srcAspect := displayAspectRatioForSource(src)
		if target == 0 || srcAspect == 0 || utils.RatiosApproxEqual(target, srcAspect, 0.01) {
			aspectBox.Hide()
		} else {
			aspectBox.Show()
		}
	}
	updateAspectBoxVisibility()

	aspectOptions.OnChanged = func(value string) {
		logging.Debug(logging.CatUI, "aspect handling set to %s", value)
		state.convert.AspectHandling = value
	}

	// Cover art display on one line
	coverDisplay = widget.NewLabel(t.ConvertCoverArtLabel + ": " + state.convert.CoverLabel())

	// Create color-coded video codec select widget with colored dropdown items
	videoCodecOptions := []string{"H.264", "H.265", "VP9", "AV1", "MPEG-2", "Copy"}
	videoCodecColorMap := ui.BuildVideoCodecColorMap(videoCodecOptions)
	videoCodecSelect = ui.NewColoredSelect(videoCodecOptions, videoCodecColorMap, func(value string) {
		state.convert.VideoCodec = value
		logging.Debug(logging.CatUI, "video codec set to %s", value)
		// Toggle H.264 profile/level and tune visibility
		if value == "H.264" {
			profileLevelContainer.Show()
			tuneContainer.Show()
		} else if value == "H.265" {
			profileLevelContainer.Hide()
			tuneContainer.Show()
		} else {
			profileLevelContainer.Hide()
			tuneContainer.Hide()
		}
		var preferredExt string
		if state.convert.SelectedFormat.Ext != "" {
			preferredExt = state.convert.SelectedFormat.Ext
		}
		var match *formatOption
		var fallback *formatOption
		for _, opt := range formatOptions {
			friendly := friendlyCodecFromPreset(opt.VideoCodec)
			if opt.VideoCodec == "copy" {
				friendly = "Copy"
			}
			if friendly != value {
				continue
			}
			if preferredExt != "" && opt.Ext == preferredExt {
				match = &opt
				break
			}
			if fallback == nil {
				fallback = &opt
			}
		}
		if match == nil {
			match = fallback
		}
		if match != nil && match.Label != state.convert.SelectedFormat.Label {
			syncingFormat = true
			applySelectedFormat(*match)
			formatContainer.SetSelected(match.Label)
			syncingFormat = false
		}
		if updateQualityOptions != nil {
			updateQualityOptions()
		}
		if updateQualityVisibility != nil {
			updateQualityVisibility()
		}
		if updateRemuxVisibility != nil {
			updateRemuxVisibility()
		}
		if buildCommandPreview != nil {
			buildCommandPreview()
		}
	}, state.window)
	videoCodecSelect.SetSelected(state.convert.VideoCodec)
	videoCodecContainer := videoCodecSelect // Use the widget directly instead of wrapping

	// Chapter warning label (shown when converting file with chapters to DVD)
	chapterWarningLabel := widget.NewLabel("  " + t.ConvertChapterLostWarning)
	chapterWarningLabel.Wrapping = fyne.TextWrapWord
	chapterWarningLabel.TextStyle = fyne.TextStyle{Italic: true}
	updateChapterWarning = func() {
		isDVD := state.convert.SelectedFormat.Ext == ".mpg"
		if src != nil && src.HasChapters && isDVD {
			chapterWarningLabel.Show()
		} else {
			chapterWarningLabel.Hide()
		}
	}

	// Format section UI (commented out - incomplete implementation)
	// TODO: Implement format section with navy background and codec info display

	updateChapterWarning() // Initial visibility

	if !state.convert.AspectUserSet {
		state.convert.OutputAspect = "Source"
	}

	// Encoder Preset with hint
	encoderPresetHint := widget.NewLabel("")
	encoderPresetHint.Wrapping = fyne.TextWrapWord
	// Wrap hint in padded container to ensure proper text wrapping in narrow windows
	encoderPresetHintContainer := container.NewPadded(encoderPresetHint)

	updateEncoderPresetHint := func(preset string) {
		var hint string
		switch preset {
		case "ultrafast":
			hint = " Ultrafast: Fastest encoding, largest files (~10x faster than slow, ~30% larger files)"
		case "superfast":
			hint = " Superfast: Very fast encoding, large files (~7x faster than slow, ~20% larger files)"
		case "veryfast":
			hint = " Very Fast: Fast encoding, moderately large files (~5x faster than slow, ~15% larger files)"
		case "faster":
			hint = " Faster: Quick encoding, slightly large files (~3x faster than slow, ~10% larger files)"
		case "fast":
			hint = " Fast: Good speed, slightly large files (~2x faster than slow, ~5% larger files)"
		case "medium":
			hint = "- Medium (default): Balanced speed and quality (baseline for comparison)"
		case "slow":
			hint = " Slow (recommended): Best quality/size ratio (~2x slower than medium, ~5-10% smaller)"
		case "slower":
			hint = " Slower: Excellent compression (~3x slower than medium, ~10-15% smaller files)"
		case "veryslow":
			hint = " Very Slow: Maximum compression (~5x slower than medium, ~15-20% smaller files)"
		default:
			hint = ""
		}
		encoderPresetHint.SetText(hint)
	}

	encoderPresetOptions := []string{"veryslow", "slower", "slow", "medium", "fast", "faster", "veryfast", "superfast", "ultrafast"}
	presetColorMap := make(map[string]color.Color, len(encoderPresetOptions))
	for _, opt := range encoderPresetOptions {
		presetColorMap[opt] = utils.MustHex("#344256")
	}
	encoderPresetSelect := ui.NewColoredSelect(encoderPresetOptions, presetColorMap, func(value string) {
		state.convert.EncoderPreset = value
		logging.Debug(logging.CatUI, "encoder preset set to %s", value)
		updateEncoderPresetHint(value)
		if buildCommandPreview != nil {
			buildCommandPreview()
		}
	}, state.window)
	encoderPresetSelect.SetSelected(state.convert.EncoderPreset)
	updateEncoderPresetHint(state.convert.EncoderPreset)

	// Simple mode preset dropdown
	simplePresetSelect := ui.NewColoredSelect(encoderPresetOptions, presetColorMap, func(value string) {
		state.convert.EncoderPreset = value
		logging.Debug(logging.CatUI, "simple preset set to %s", value)
		updateEncoderPresetHint(value)
		if buildCommandPreview != nil {
			buildCommandPreview()
		}
	}, state.window)
	simplePresetSelect.SetSelected(state.convert.EncoderPreset)

	// Settings management for batch operations
	settingsInfoLabel := widget.NewLabel(t.ConvertSettingsInfo)
	settingsInfoLabel.Alignment = fyne.TextAlignCenter
	settingsInfoLabel.Wrapping = fyne.TextWrapWord
	// Wrap in padded container for proper text wrapping in narrow windows
	settingsInfoContainer := container.NewPadded(settingsInfoLabel)

	cacheDirLabel := widget.NewLabelWithStyle(t.ConvertSectionCacheDir, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	cacheDirEntry := widget.NewEntry()
	cacheDirEntry.SetPlaceHolder("System temp (recommended SSD)")
	cacheDirEntry.SetText(state.convert.TempDir)
	cacheDirHint := widget.NewLabel(t.ConvertCacheDirHint)
	cacheDirHint.Wrapping = fyne.TextWrapWord
	// Wrap in padded container for proper text wrapping in narrow windows
	cacheDirHintContainer := container.NewPadded(cacheDirHint)
	cacheDirEntry.OnChanged = func(val string) {
		state.convert.TempDir = strings.TrimSpace(val)
		utils.SetTempDir(state.convert.TempDir)
	}
	cacheBrowseBtn := ui.MakePillButton(t.ActionBrowse, ui.BorderDim, func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			cacheDirEntry.SetText(uri.Path())
			state.convert.TempDir = uri.Path()
			utils.SetTempDir(state.convert.TempDir)
		}, state.window)
	})
	cacheUseSystemBtn := ui.MakePillButton(t.ConvertUseSystemTemp, ui.BorderDim, func() {
		cacheDirEntry.SetText("")
		state.convert.TempDir = ""
		utils.SetTempDir("")
	})

	logsDirLabel := widget.NewLabelWithStyle(t.ConvertSectionLogsDir, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	logsDirEntry := widget.NewEntry()
	logsDirEntry.SetPlaceHolder(defaultLogsDir())
	logsDirEntry.SetText(state.convert.LogDir)
	logsDirHint := widget.NewLabel(fmt.Sprintf(t.ConvertDefaultPathFmt, defaultLogsDir()))
	logsDirHint.Wrapping = fyne.TextWrapWord
	logsDirHintContainer := container.NewPadded(logsDirHint)
	applyLogsDir := func(val string) {
		state.convert.LogDir = strings.TrimSpace(val)
		setLogsDirOverride(state.convert.LogDir)
		logging.SetLogsDir(getLogsDir())
		logging.Reopen()
		state.persistConvertConfig()
	}
	logsDirEntry.OnChanged = func(val string) {
		applyLogsDir(val)
	}
	logsBrowseBtn := ui.MakePillButton(t.ActionBrowse, ui.BorderDim, func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			logsDirEntry.SetText(uri.Path())
		}, state.window)
	})
	logsUseDefaultBtn := ui.MakePillButton(t.ConvertUseDefault, ui.BorderDim, func() {
		logsDirEntry.SetText("")
	})

	resetSettingsBtn := ui.MakePillButton(t.ConvertResetDefaults, ui.BorderDim, func() {
		if resetConvertDefaults != nil {
			resetConvertDefaults()
		}
	})

	settingsContent := container.NewVBox(
		settingsInfoContainer,
		widget.NewSeparator(),
		cacheDirLabel,
		container.NewBorder(nil, nil, nil, cacheBrowseBtn, cacheDirEntry),
		cacheUseSystemBtn,
		cacheDirHintContainer,
		widget.NewSeparator(),
		logsDirLabel,
		container.NewBorder(nil, nil, nil, logsBrowseBtn, logsDirEntry),
		logsUseDefaultBtn,
		logsDirHintContainer,
		resetSettingsBtn,
	)
	settingsContent.Hide()

	settingsVisible := false
	toggleSettingsLabel := widget.NewLabel(t.ConvertShowBatchSettings)
	toggleSettingsLabel.Wrapping = fyne.TextWrapWord
	toggleSettingsLabel.Alignment = fyne.TextAlignCenter

	var toggleSettingsBtn *ui.PillButton
	toggleSettingsBtn = ui.MakePillButton("", ui.BorderDim, func() {
		if settingsVisible {
			settingsContent.Hide()
			toggleSettingsLabel.SetText(t.ConvertShowBatchSettings)
		} else {
			settingsContent.Show()
			toggleSettingsLabel.SetText(t.ConvertHideBatchSettings)
		}
		settingsVisible = !settingsVisible
	})

	// Replace button text with wrapped label
	toggleSettingsBtnWithLabel := container.NewStack(
		toggleSettingsBtn,
		container.NewPadded(toggleSettingsLabel),
	)

	settingsBox := container.NewVBox(
		toggleSettingsBtnWithLabel,
		settingsContent,
		widget.NewSeparator(),
	)

	// Bitrate Mode - horizontal radio buttons
	bitrateModeOptions := []string{
		"CRF",
		"CBR",
		"VBR",
		"Target Size",
	}
	applyBitrateMode := func(value string) {
		state.convert.BitrateMode = normalizeBitrateMode(value)
		logging.Debug(logging.CatUI, "bitrate mode set to %s", state.convert.BitrateMode)
		if updateEncodingControls != nil {
			updateEncodingControls()
		}
		if updateQualityVisibility != nil {
			updateQualityVisibility()
		}
		if buildCommandPreview != nil {
			buildCommandPreview()
		}
	}
	stateMgr.OnBitrateModeChange(applyBitrateMode)

	// Radio buttons for bitrate mode
	bitrateModeRadio = widget.NewRadioGroup(bitrateModeOptions, func(value string) {
		if value != "" {
			stateMgr.SetBitrateMode(value)
		}
	})
	bitrateModeRadio.Horizontal = true

	// Set initial selection AFTER callback is registered, default to CRF if empty
	initialMode := state.convert.BitrateMode
	if initialMode == "" {
		initialMode = "CRF"
	}
	// Defer SetSelected to avoid potential race during widget init
	bitrateModeRadio.SetSelected(initialMode)
	state.convert.BitrateMode = normalizeBitrateMode(state.convert.BitrateMode)
	stateMgr.SetBitrateMode(state.convert.BitrateMode)

	// Manual CRF entry
	// CRF entry with debouncing (300ms delay) and validation
	crfEntry = widget.NewEntry()
	crfEntry.SetPlaceHolder("Auto (from Quality preset)")
	crfEntry.SetText(state.convert.CRF)
	crfEntry.Validator = validateCRF
	crfEntryBg := canvas.NewRectangle(utils.MustHex("#344256"))
	crfEntryBg.CornerRadius = 8
	crfEntryBg.SetMinSize(fyne.NewSize(0, 36))
	crfEntryWrapper := container.NewMax(crfEntryBg, container.NewPadded(crfEntry))
	updateCRFEntryState := func(val string) {
		if validateCRF(val) == nil {
			crfEntryBg.FillColor = utils.MustHex("#344256")
		} else {
			crfEntryBg.FillColor = utils.MustHex("#5A2A2A")
		}
		crfEntryBg.Refresh()
	}
	crfEntry.OnChanged = createDebouncedCallback(300*time.Millisecond, func(val string) {
		updateCRFEntryState(val)
		if validateCRF(val) == nil {
			state.convert.CRF = val
			if buildCommandPreview != nil {
				buildCommandPreview()
			}
		}
	})

	manualCrfRow = container.NewVBox(
		func() *widget.Label {
			manualCrfLabel = widget.NewLabelWithStyle(t.ConvertSectionManualCRF, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			return manualCrfLabel
		}(),
		crfEntryWrapper,
	)
	if state.convert.Quality == manualQualityOption {
		if strings.TrimSpace(state.convert.CRF) == "" {
			state.convert.CRF = "23"
			crfEntry.SetText("23")
		}
		updateCRFEntryState(crfEntry.Text)
	} else {
		manualCrfRow.Hide()
	}

	// Video Bitrate entry (for CBR/VBR) with validation
	videoBitrateEntry = widget.NewEntry()
	videoBitrateEntry.SetPlaceHolder("5000")
	videoBitrateUnitSelect := widget.NewSelect([]string{"Kbps", "Mbps", "Gbps"}, func(value string) {})
	videoBitrateUnitSelect.SetSelected("Kbps")
	videoBitrateEntry.Validator = func(input string) error {
		return validateBitrate(input, videoBitrateUnitSelect.Selected)
	}
	manualBitrateInput := container.NewBorder(nil, nil, nil, videoBitrateUnitSelect, videoBitrateEntry)

	parseBitrateParts := func(input string) (string, string, bool) {
		trimmed := strings.TrimSpace(input)
		if trimmed == "" {
			return "", "", false
		}
		upper := strings.ToUpper(trimmed)
		var num float64
		var unit string
		if _, err := fmt.Sscanf(upper, "%f%s", &num, &unit); err != nil {
			return "", "", false
		}
		numStr := strconv.FormatFloat(num, 'f', -1, 64)
		switch unit {
		case "K", "KBPS":
			unit = "Kbps"
		case "M", "MBPS":
			unit = "Mbps"
		case "G", "GBPS":
			unit = "Gbps"
		}
		return numStr, unit, true
	}

	normalizeBitrateUnit := func(label string) string {
		switch label {
		case "Kbps":
			return "k"
		case "Mbps":
			return "M"
		case "Gbps":
			return "G"
		default:
			return "k"
		}
	}

	var previousBitrateUnit = "Kbps" // Track previous unit for conversion
	updateBitrateState := func() {
		val := strings.TrimSpace(videoBitrateEntry.Text)
		if val == "" {
			state.convert.VideoBitrate = ""
			return
		}
		if num, unit, ok := parseBitrateParts(val); ok && unit != "" {
			if num != val {
				videoBitrateEntry.SetText(num)
				return
			}
			if unit != videoBitrateUnitSelect.Selected {
				videoBitrateUnitSelect.SetSelected(unit)
				return
			}
			val = num
		}
		unit := normalizeBitrateUnit(videoBitrateUnitSelect.Selected)
		state.convert.VideoBitrate = val + unit
		if buildCommandPreview != nil {
			buildCommandPreview()
		}
	}

	setManualBitrate := func(value string) {
		if value == "" {
			videoBitrateEntry.SetText("")
			return
		}
		if num, unit, ok := parseBitrateParts(value); ok {
			videoBitrateEntry.SetText(num)
			if unit != "" {
				videoBitrateUnitSelect.SetSelected(unit)
				previousBitrateUnit = unit // Update tracked unit
			}
		} else {
			videoBitrateEntry.SetText(value)
		}
		state.convert.VideoBitrate = value
	}

	videoBitrateUnitSelect.OnChanged = func(newUnit string) {
		if manualBitrateRow != nil && manualBitrateRow.Hidden {
			return
		}

		// Convert the numeric value when unit changes
		if previousBitrateUnit != newUnit {
			currentText := strings.TrimSpace(videoBitrateEntry.Text)
			if currentText != "" {
				if currentValue, err := strconv.ParseFloat(currentText, 64); err == nil {
					// Convert from previous unit to new unit
					var convertedValue float64

					// First convert to Kbps (base unit)
					var valueInKbps float64
					switch previousBitrateUnit {
					case "Kbps":
						valueInKbps = currentValue
					case "Mbps":
						valueInKbps = currentValue * 1000
					case "Gbps":
						valueInKbps = currentValue * 1000000
					}

					// Then convert from Kbps to new unit
					switch newUnit {
					case "Kbps":
						convertedValue = valueInKbps
					case "Mbps":
						convertedValue = valueInKbps / 1000
					case "Gbps":
						convertedValue = valueInKbps / 1000000
					}

					// Format the converted value, removing unnecessary decimals
					var formattedValue string
					if convertedValue == float64(int64(convertedValue)) {
						// No decimal part
						formattedValue = strconv.FormatInt(int64(convertedValue), 10)
					} else {
						// Has decimal part - format with precision
						formattedValue = strconv.FormatFloat(convertedValue, 'f', -1, 64)
					}

					// Update entry with converted value (debouncing handles update delays)
					videoBitrateEntry.SetText(formattedValue)
				}
			}
			previousBitrateUnit = newUnit
		}

		updateBitrateState()
	}

	// Apply debouncing to bitrate entry (300ms delay)
	debouncedBitrateUpdate := createDebouncedCallback(300*time.Millisecond, func(val string) {
		updateBitrateState()
	})
	videoBitrateEntry.OnChanged = func(val string) {
		debouncedBitrateUpdate(val)
	}

	if state.convert.VideoBitrate != "" {
		setManualBitrate(state.convert.VideoBitrate)
	}

	// Create CRF container (manual entry shown when quality is Manual)
	crfContainer = container.NewVBox(manualCrfRow)

	// Note: bitrateContainer creation moved below after bitratePresetSelect is initialized

	type bitratePreset struct {
		Label   string
		Bitrate string
		Codec   string
	}

	presets := []bitratePreset{
		{Label: "0.5 Mbps - Ultra Low", Bitrate: "500k", Codec: ""},
		{Label: "1.0 Mbps - Very Low", Bitrate: "1000k", Codec: ""},
		{Label: "1.5 Mbps - Low", Bitrate: "1500k", Codec: ""},
		{Label: "2.0 Mbps - Medium-Low", Bitrate: "2000k", Codec: ""},
		{Label: "2.5 Mbps - Medium", Bitrate: "2500k", Codec: ""},
		{Label: "4.0 Mbps - Good", Bitrate: "4000k", Codec: ""},
		{Label: "6.0 Mbps - High", Bitrate: "6000k", Codec: ""},
		{Label: "8.0 Mbps - Very High", Bitrate: "8000k", Codec: ""},
		{Label: "Manual", Bitrate: "", Codec: ""},
	}

	bitratePresetLookup := make(map[string]bitratePreset)
	var bitratePresetLabels []string
	for _, p := range presets {
		bitratePresetLookup[p.Label] = p
		bitratePresetLabels = append(bitratePresetLabels, p.Label)
	}

	normalizePresetLabel := func(label string) string {
		switch label {
		case "2.5 Mbps - Medium Quality":
			return "2.5 Mbps - Medium"
		case "2.0 Mbps - Medium-Low Quality":
			return "2.0 Mbps - Medium-Low"
		case "1.5 Mbps - Low Quality":
			return "1.5 Mbps - Low"
		case "4.0 Mbps - Good Quality":
			return "4.0 Mbps - Good"
		case "6.0 Mbps - High Quality":
			return "6.0 Mbps - High"
		case "8.0 Mbps - Very High Quality":
			return "8.0 Mbps - Very High"
		case "0.5 Mbps - Ultra Low":
			return label
		case "1.0 Mbps - Very Low":
			return label
		case "Manual":
			return "Manual"
		default:
			return label
		}
	}

	var applyBitratePreset func(string)

	// Bitrate preset select - uses state manager
	bitratePresetSelect = widget.NewSelect(bitratePresetLabels, func(value string) {
		setBitratePreset(value)
		if applyBitratePreset != nil {
			applyBitratePreset(value)
		}
	})
	state.convert.BitratePreset = normalizePresetLabel(state.convert.BitratePreset)
	if state.convert.BitratePreset == "" || bitratePresetLookup[state.convert.BitratePreset].Label == "" {
		state.convert.BitratePreset = "2.5 Mbps - Medium"
	}
	bitratePresetSelect.SetSelected(state.convert.BitratePreset)

	// Simple bitrate selector (shares presets) - uses state manager
	simpleBitrateSelect = widget.NewSelect(bitratePresetLabels, func(value string) {
		setBitratePreset(value)
		if applyBitratePreset != nil {
			applyBitratePreset(value)
		}
	})
	simpleBitrateSelect.SetSelected(state.convert.BitratePreset)

	// Manual bitrate row (hidden by default)
	manualBitrateLabel := widget.NewLabelWithStyle(t.ConvertSectionManualBitrate, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	manualBitrateRow = container.NewVBox(manualBitrateLabel, manualBitrateInput)
	manualBitrateRow.Hide()

	// Create bitrate container now that bitratePresetSelect is initialized
	bitrateContainer = container.NewVBox(
		widget.NewLabelWithStyle(t.ConvertSectionBitratePreset, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		bitratePresetSelect,
		manualBitrateRow,
	)

	// Simple resolution selector (separate widget to avoid double-parent issues)
	resolutionOptionsSimple := []string{
		"Source", "360p", "480p", "540p", "720p", "1080p", "1440p", "4K", "8K",
		"2X (relative)", "4X (relative)",
		"NTSC (720x480)", "PAL (720x540)", "PAL (720x576)",
	}
	// Resolution select (Simple mode) - uses state manager
	resolutionSelectSimple := widget.NewSelect(resolutionOptionsSimple, func(value string) {
		logging.Debug(logging.CatUI, "target resolution set to %s (simple)", value)
		setResolution(value)
	})
	resolutionSelectSimple.SetSelected(state.convert.TargetResolution)

	// Simple aspect selector (separate widget) - uses state manager
	targetAspectSelectSimple = widget.NewSelect(aspectTargets, func(value string) {
		if value == customAspectLabel {
			customAspectActive = true
			updateCustomAspectUI()
			if strings.TrimSpace(customAspectValue) != "" {
				applyCustomAspect(customAspectValue)
			}
			return
		}
		customAspectActive = false
		setAspect(aspectValueForLabel(value), true)
		updateCustomAspectUI()
	})
	if state.convert.OutputAspect == "" {
		state.convert.OutputAspect = "Source"
	}
	targetAspectSelectSimple.SetSelected(aspectLabelForValue(state.convert.OutputAspect))

	customAspectEntrySimple := widget.NewEntry()
	customAspectEntrySimple.SetPlaceHolder("e.g. 1.90 or 256:135")
	customAspectEntrySimple.OnChanged = func(val string) {
		applyCustomAspect(val)
	}
	customAspectHintSimple := widget.NewLabel(t.ConvertCustomAspectHint)
	customAspectHintSimple.Wrapping = fyne.TextWrapWord
	customAspectBoxSimple := container.NewVBox(
		widget.NewLabelWithStyle(t.ConvertSectionCustomAspect, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		customAspectEntrySimple,
		customAspectHintSimple,
	)
	customAspectEntries = append(customAspectEntries, customAspectEntrySimple)
	customAspectBoxes = append(customAspectBoxes, customAspectBoxSimple)
	customAspectHintLabels = append(customAspectHintLabels, customAspectHintSimple)
	updateCustomAspectUI()

	// Register updateAspectBoxVisibility callback with state manager
	uiState.updateAspectBoxVisibility = updateAspectBoxVisibility

	// Initialize aspect state
	setAspect(state.convert.OutputAspect, state.convert.AspectUserSet)

	// Target File Size with smart presets + manual entry and validation
	targetFileSizeEntry = widget.NewEntry()
	targetFileSizeEntry.SetPlaceHolder("e.g., 250")
	targetFileSizeEntry.Validator = validateFileSize
	targetFileSizeUnitSelect := widget.NewSelect([]string{"KB", "MB", "GB"}, func(value string) {})
	targetFileSizeUnitSelect.SetSelected("MB")
	targetSizeManualRow := container.NewBorder(nil, nil, nil, targetFileSizeUnitSelect, targetFileSizeEntry)
	targetSizeManualRow.Hide() // Hidden by default, show only when "Manual" is selected

	parseSizeParts := func(input string) (string, string, bool) {
		trimmed := strings.TrimSpace(input)
		if trimmed == "" {
			return "", "", false
		}
		upper := strings.ToUpper(trimmed)
		var num float64
		var unit string
		if _, err := fmt.Sscanf(upper, "%f%s", &num, &unit); err != nil {
			return "", "", false
		}
		numStr := strconv.FormatFloat(num, 'f', -1, 64)
		return numStr, unit, true
	}

	updateTargetSizeState := func() {
		val := strings.TrimSpace(targetFileSizeEntry.Text)
		if val == "" {
			state.convert.TargetFileSize = ""
			return
		}
		if num, unit, ok := parseSizeParts(val); ok && unit != "" {
			if num != val {
				targetFileSizeEntry.SetText(num)
				return
			}
			if unit != targetFileSizeUnitSelect.Selected {
				targetFileSizeUnitSelect.SetSelected(unit)
				return
			}
			val = num
		}
		unit := targetFileSizeUnitSelect.Selected
		if unit == "" {
			unit = "MB"
			targetFileSizeUnitSelect.SetSelected(unit)
		}
		state.convert.TargetFileSize = val + unit
		logging.Debug(logging.CatUI, "target file size set to %s", state.convert.TargetFileSize)
		if buildCommandPreview != nil {
			buildCommandPreview()
		}
	}

	setTargetFileSize := func(value string) {
		if value == "" {
			targetFileSizeEntry.SetText("")
			targetFileSizeUnitSelect.SetSelected("MB")
			state.convert.TargetFileSize = ""
			return
		}
		if num, unit, ok := parseSizeParts(value); ok {
			targetFileSizeEntry.SetText(num)
			if unit != "" {
				targetFileSizeUnitSelect.SetSelected(unit)
			}
		} else {
			targetFileSizeEntry.SetText(value)
		}
		state.convert.TargetFileSize = value
	}

	targetFileSizeUnitSelect.OnChanged = func(value string) {
		if targetFileSizeEntry.Hidden {
			return
		}
		updateTargetSizeState()
	}

	updateTargetSizeOptions := func() {
		options := []string{"Manual", "25MB", "50MB", "100MB", "200MB", "500MB", "1GB"}

		if src != nil {
			// Calculate smart reduction options based on source file size
			srcPath := src.Path
			fileInfo, err := os.Stat(srcPath)
			if err == nil {
				srcSize := fileInfo.Size()
				srcSizeMB := float64(srcSize) / (1024 * 1024)

				// Calculate smart reductions
				size25 := int(srcSizeMB * 0.75) // 25% reduction
				size33 := int(srcSizeMB * 0.67) // 33% reduction
				size50 := int(srcSizeMB * 0.50) // 50% reduction
				size75 := int(srcSizeMB * 0.25) // 75% reduction

				smartOptions := []string{"Manual"}

				if size75 > 5 {
					smartOptions = append(smartOptions, fmt.Sprintf("%dMB (75%% smaller)", size75))
				}
				if size50 > 10 {
					smartOptions = append(smartOptions, fmt.Sprintf("%dMB (50%% smaller)", size50))
				}
				if size33 > 15 {
					smartOptions = append(smartOptions, fmt.Sprintf("%dMB (33%% smaller)", size33))
				}
				if size25 > 20 {
					smartOptions = append(smartOptions, fmt.Sprintf("%dMB (25%% smaller)", size25))
				}

				// Add common sizes
				smartOptions = append(smartOptions, "25MB", "50MB", "100MB", "200MB", "500MB", "1GB")
				options = smartOptions
			}
		}

		targetFileSizeSelect.Options = options
		targetFileSizeSelect.Refresh()
	}

	targetSizeOpts := []string{"25MB", "50MB", "100MB", "200MB", "500MB", "1GB", "Manual"}
	targetFileSizeSelect = widget.NewSelect(targetSizeOpts, func(value string) {
		if value == "Manual" {
			targetSizeManualRow.Show()
			if state.convert.TargetFileSize != "" {
				if num, unit, ok := parseSizeParts(state.convert.TargetFileSize); ok {
					targetFileSizeEntry.SetText(num)
					if unit != "" {
						targetFileSizeUnitSelect.SetSelected(unit)
					}
				} else {
					targetFileSizeEntry.SetText(state.convert.TargetFileSize)
				}
			}
		} else {
			// Extract size from selection (handle "XMB (Y% smaller)" format)
			var sizeStr string
			if strings.Contains(value, "(") {
				// Format: "50MB (50% smaller)"
				sizeStr = strings.TrimSpace(strings.Split(value, "(")[0])
			} else {
				// Format: "100MB"
				sizeStr = value
			}
			state.convert.TargetFileSize = sizeStr
			if num, unit, ok := parseSizeParts(sizeStr); ok {
				targetFileSizeEntry.SetText(num)
				if unit != "" {
					targetFileSizeUnitSelect.SetSelected(unit)
				}
			} else {
				targetFileSizeEntry.SetText(sizeStr)
			}
			targetSizeManualRow.Hide()
		}
		logging.Debug(logging.CatUI, "target file size set to %s", state.convert.TargetFileSize)
	})
	targetFileSizeSelect.SetSelected("100MB")
	updateTargetSizeOptions()

	// Apply debouncing to target file size entry (300ms delay)
	debouncedTargetSizeUpdate := createDebouncedCallback(300*time.Millisecond, func(val string) {
		updateTargetSizeState()
	})
	targetFileSizeEntry.OnChanged = func(val string) {
		debouncedTargetSizeUpdate(val)
	}
	if state.convert.TargetFileSize != "" {
		if num, unit, ok := parseSizeParts(state.convert.TargetFileSize); ok {
			targetFileSizeEntry.SetText(num)
			if unit != "" {
				targetFileSizeUnitSelect.SetSelected(unit)
			}
		} else {
			targetFileSizeEntry.SetText(state.convert.TargetFileSize)
		}
	}

	// Create target size container
	targetSizeContainer = container.NewVBox(
		widget.NewLabelWithStyle(t.ConvertSectionTargetFileSize, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		targetFileSizeSelect,
		targetSizeManualRow,
	)

	encodingHint := widget.NewLabel("")
	encodingHint.Wrapping = fyne.TextWrapWord
	// Wrap hint in padded container to ensure proper text wrapping in narrow windows
	encodingHintContainer := container.NewPadded(encodingHint)

	applyBitratePreset = func(label string) {
		preset, ok := bitratePresetLookup[label]
		if !ok {
			label = "Manual"
			preset = bitratePresetLookup[label]
		}

		state.convert.BitratePreset = label

		// Show/hide manual bitrate entry based on selection
		if label == "Manual" {
			manualBitrateRow.Show()
		} else {
			manualBitrateRow.Hide()
		}
		if manualBitrateRow != nil {
			manualBitrateRow.Refresh()
		}
		if bitrateContainer != nil {
			bitrateContainer.Refresh()
		}

		// Move to CBR for predictable output when a preset is chosen
		if preset.Bitrate != "" && stateMgr.BitrateMode() != "CBR" && stateMgr.BitrateMode() != "VBR" {
			stateMgr.SetBitrateMode("CBR")
			bitrateModeRadio.SetSelected("CBR")
		}

		if preset.Bitrate != "" {
			state.convert.VideoBitrate = preset.Bitrate
			if setManualBitrate != nil {
				setManualBitrate(preset.Bitrate)
			} else {
				videoBitrateEntry.SetText(preset.Bitrate)
			}
		}

		// Adjust codec to match the preset intent (user can change back)
		if preset.Codec != "" && state.convert.VideoCodec != preset.Codec {
			state.convert.VideoCodec = preset.Codec
			videoCodecSelect.SetSelected(preset.Codec)
		}

		if updateEncodingControls != nil {
			updateEncodingControls()
		}
	}

	// Initialize bitrate preset through state manager
	setBitratePreset(state.convert.BitratePreset)
	if applyBitratePreset != nil {
		applyBitratePreset(state.convert.BitratePreset)
	}

	updateEncodingControls = func() {
		remux := strings.EqualFold(state.convert.SelectedFormat.VideoCodec, "copy") ||
			strings.EqualFold(state.convert.VideoCodec, "Copy")
		mode := normalizeBitrateMode(state.convert.BitrateMode)
		isLossless := state.convert.Quality == "Lossless"
		supportsLossless := codecSupportsLossless(state.convert.VideoCodec)

		if remux {
			// Remux = stream copy, no encoding controls
			if manualCrfRow != nil {
				manualCrfRow.Hide()
			}
			if crfContainer != nil {
				crfContainer.Hide()
			}
			if bitrateContainer != nil {
				bitrateContainer.Hide()
			}
			if targetSizeContainer != nil {
				targetSizeContainer.Hide()
			}
			if twoPassCheck != nil {
				twoPassCheck.Disable()
				if twoPassNote != nil {
					twoPassNote.Show()
				}
			}
			if encodingHint != nil {
				encodingHint.SetText(t.ConvertRemuxHint)
			}
			if updateQualityVisibility != nil {
				updateQualityVisibility()
			}
			if buildCommandPreview != nil {
				buildCommandPreview()
			}
			return
		}

		hint := ""
		showCRF := mode == "CRF" || mode == ""
		showBitrate := mode == "CBR" || mode == "VBR"
		showTarget := mode == "Target Size"
		showManualCRF := strings.EqualFold(state.convert.Quality, manualQualityOption)

		if !showCRF && state.convert.Quality == manualQualityOption {
			state.convert.Quality = "Standard (CRF 23)"
			for _, w := range uiState.qualityWidgets {
				w.SetSelectedSilent(state.convert.Quality)
			}
			showManualCRF = false
		}

		if !showCRF {
			state.convert.CRF = ""
			if crfEntry != nil && crfEntry.Text != "" {
				crfEntry.SetText("")
			}
			if manualCrfRow != nil {
				manualCrfRow.Hide()
			}
		}

		if isLossless && supportsLossless {
			// Lossless with H.265/AV1: Allow all bitrate modes
			// The lossless quality affects the encoding, but bitrate/target size still control output
			switch mode {
			case "CRF", "":
				if crfEntry.Text != "0" {
					crfEntry.SetText("0")
				}
				state.convert.CRF = "0"
				crfEntry.Disable()
				hint = t.ConvertBitrateModeHintLosslessCRF
			case "CBR":
				hint = t.ConvertBitrateModeHintLosslessCBR
			case "VBR":
				hint = t.ConvertBitrateModeHintLosslessVBR
			case "Target Size":
				hint = t.ConvertBitrateModeHintLosslessTarget
			}
		} else {
			crfEntry.Enable()
			switch mode {
			case "CRF", "":
				hint = t.ConvertBitrateModeHintCRF
			case "CBR":
				hint = t.ConvertBitrateModeHintCBR
			case "VBR":
				hint = t.ConvertBitrateModeHintVBR
			case "Target Size":
				hint = t.ConvertBitrateModeHintTargetSize
			}
		}

		if showCRF && showManualCRF {
			if manualCrfLabel != nil {
				manualCrfLabel.SetText(t.ConvertSectionManualCRF)
			}
			if manualCrfRow != nil {
				manualCrfRow.Show()
			}
			if crfEntry != nil {
				crfEntry.Enable()
			}
			if crfContainer != nil {
				crfContainer.Show()
			}
		} else {
			if manualCrfRow != nil {
				manualCrfRow.Hide()
			}
			if crfEntry != nil {
				crfEntry.Disable()
			}
			if crfContainer != nil {
				crfContainer.Hide()
			}
		}
		if showBitrate {
			bitrateContainer.Show()
		} else {
			bitrateContainer.Hide()
		}
		if showTarget {
			targetSizeContainer.Show()
		} else {
			targetSizeContainer.Hide()
		}

		encodingHint.SetText(hint)

		if twoPassCheck != nil {
			if mode == "CRF" || mode == "" {
				if state.convert.TwoPass {
					state.convert.TwoPass = false
					twoPassCheck.SetChecked(false)
				}
				twoPassCheck.Disable()
				if twoPassNote != nil {
					twoPassNote.Show()
				}
			} else {
				twoPassCheck.Enable()
				if twoPassNote != nil {
					twoPassNote.Hide()
				}
			}
		}

		// Let updateQualityVisibility() handle showing/hiding quality sections
		// to avoid duplicate logic and conflicts
		if updateQualityVisibility != nil {
			updateQualityVisibility()
		}

		if buildCommandPreview != nil {
			buildCommandPreview()
		}
	}
	uiState.onQualityChange = append(uiState.onQualityChange, func(string) {
		updateEncodingControls()
	})
	updateEncodingControls()
	if updateQualityVisibility != nil {
		updateQualityVisibility()
	}

	// Target Resolution (advanced)
	resolutionOptions := []string{
		"Source", "720p", "1080p", "1440p", "4K", "8K",
		"2X (relative)", "4X (relative)",
		"NTSC (720x480)", "PAL (720x540)", "PAL (720x576)",
	}
	// Resolution select (Advanced mode) - uses state manager
	resolutionSelect := widget.NewSelect(resolutionOptions, func(value string) {
		logging.Debug(logging.CatUI, "target resolution set to %s", value)
		setResolution(value)
	})
	if state.convert.TargetResolution == "" {
		state.convert.TargetResolution = "Source"
	}
	resolutionSelect.SetSelected(state.convert.TargetResolution)

	// Frame Rate with hint
	frameRateHint := widget.NewLabel("")
	frameRateHint.Wrapping = fyne.TextWrapWord
	// Wrap hint in padded container to ensure proper text wrapping in narrow windows
	frameRateHintContainer := container.NewPadded(frameRateHint)

	updateFrameRateHint := func() {
		if src == nil {
			frameRateHint.SetText("")
			return
		}

		selectedFPS := state.convert.FrameRate
		if selectedFPS == "" || selectedFPS == "Source" {
			frameRateHint.SetText("")
			return
		}

		// Parse target frame rate
		var targetFPS float64
		switch selectedFPS {
		case "23.976":
			targetFPS = 23.976
		case "24":
			targetFPS = 24.0
		case "25":
			targetFPS = 25.0
		case "29.97":
			targetFPS = 29.97
		case "30":
			targetFPS = 30.0
		case "50":
			targetFPS = 50.0
		case "59.94":
			targetFPS = 59.94
		case "60":
			targetFPS = 60.0
		default:
			frameRateHint.SetText("")
			return
		}

		sourceFPS := src.FrameRate
		if sourceFPS <= 0 {
			frameRateHint.SetText("")
			return
		}

		// Calculate potential savings
		if targetFPS < sourceFPS {
			ratio := targetFPS / sourceFPS
			reduction := (1.0 - ratio) * 100
			frameRateHint.SetText(fmt.Sprintf("Converting %.0f  %.0f fps: ~%.0f%% smaller file",
				sourceFPS, targetFPS, reduction))
		} else if targetFPS > sourceFPS {
			frameRateHint.SetText(fmt.Sprintf(" Upscaling from %.0f to %.0f fps (may cause judder)",
				sourceFPS, targetFPS))
		} else {
			frameRateHint.SetText("")
		}
	}

	frameRateOptions := []string{"Source", "23.976", "24", "25", "29.97", "30", "50", "59.94", "60"}
	frameRateSelect := widget.NewSelect(frameRateOptions, func(value string) {
		state.convert.FrameRate = value
		logging.Debug(logging.CatUI, "frame rate set to %s", value)
		updateFrameRateHint()
	})
	frameRateSelect.SetSelected(state.convert.FrameRate)
	updateFrameRateHint()

	// Motion Interpolation checkbox
	motionInterpCheck := widget.NewCheck(t.ConvertMotionInterp, func(checked bool) {
		state.convert.UseMotionInterpolation = checked
		logging.Debug(logging.CatUI, "motion interpolation set to %v", checked)
	})
	motionInterpCheck.Checked = state.convert.UseMotionInterpolation

	// Pixel Format
	pixelFormatOptions := []string{"yuv420p", "yuv422p", "yuv444p"}
	pixelFormatSelect := widget.NewSelect(pixelFormatOptions, func(value string) {
		state.convert.PixelFormat = value
		logging.Debug(logging.CatUI, "pixel format set to %s", value)
	})
	pixelFormatSelect.SetSelected(state.convert.PixelFormat)

	// Hardware Acceleration with hint
	hwAccelHint := widget.NewLabel(t.ConvertHwAccelHint)
	hwAccelHint.Wrapping = fyne.TextWrapWord
	// Wrap hint in padded container to ensure proper text wrapping in narrow windows
	hwAccelHintContainer := container.NewPadded(hwAccelHint)
	// Always show platform-relevant options so users can manually override
	// even when auto-detection fails. The "auto" mode still probes at encode time.
	hwAccelOptions := []string{"auto", "none"}
	if runtime.GOOS == "windows" {
		hwAccelOptions = append(hwAccelOptions, "nvenc", "qsv", "amf")
	} else {
		hwAccelOptions = append(hwAccelOptions, "nvenc", "qsv", "vaapi", "amf")
	}
	hwAccelSelect := widget.NewSelect(hwAccelOptions, func(value string) {
		state.convert.HardwareAccel = value
		logging.Debug(logging.CatUI, "hardware accel set to %s", value)
	})
	if state.convert.HardwareAccel == "" {
		state.convert.HardwareAccel = "auto"
	}
	// If persisted value is no longer available, reset to auto
	found := false
	for _, opt := range hwAccelOptions {
		if opt == state.convert.HardwareAccel {
			found = true
			break
		}
	}
	if !found {
		state.convert.HardwareAccel = "auto"
	}
	hwAccelSelect.SetSelected(state.convert.HardwareAccel)
	state.upscaleHardwareAccel = state.convert.HardwareAccel // sync upscale HW accel from master setting

	// Two-Pass encoding
	twoPassCheck = widget.NewCheck(t.ConvertEnableTwoPass, func(checked bool) {
		state.convert.TwoPass = checked
	})
	twoPassCheck.Checked = state.convert.TwoPass
	twoPassNote = widget.NewLabel(t.ConvertTwoPassNote)
	twoPassNote.Wrapping = fyne.TextWrapWord
	twoPassNote.Hide()

	// Create color-coded audio codec select widget with colored dropdown items
	audioCodecOptions := []string{"AAC", "AC-3", "Opus", "Vorbis", "MP3", "FLAC", "Copy"}
	audioCodecColorMap := ui.BuildAudioCodecColorMap(audioCodecOptions)
	audioCodecSelect = ui.NewColoredSelect(audioCodecOptions, audioCodecColorMap, func(value string) {
		state.convert.AudioCodec = value
		logging.Debug(logging.CatUI, "audio codec set to %s", value)
	}, state.window)
	audioCodecSelect.SetSelected(state.convert.AudioCodec)
	audioCodecContainer := audioCodecSelect // Use the widget directly instead of wrapping

	// Audio Bitrate
	audioBitrateOptions := []string{"128k", "192k", "256k", "320k"}
	audioBitrateSelect := widget.NewSelect(audioBitrateOptions, func(value string) {
		state.convert.AudioBitrate = value
		logging.Debug(logging.CatUI, "audio bitrate set to %s", value)
	})
	audioBitrateSelect.SetSelected(state.convert.AudioBitrate)

	// Audio Channels
	audioChannelsOptions := []string{
		"Source",
		"Mono",
		"Stereo",
		"5.1",
		"Left to Stereo",
		"Right to Stereo",
		"Mix to Stereo",
		"Swap L/R",
	}
	audioChannelsSelect := widget.NewSelect(audioChannelsOptions, func(value string) {
		state.convert.AudioChannels = value
		logging.Debug(logging.CatUI, "audio channels set to %s", value)
	})
	audioChannelsSelect.SetSelected(state.convert.AudioChannels)

	// Audio Sample Rate
	audioSampleRateOptions := []string{"Source", "22050", "44100", "48000", "96000"}
	audioSampleRateSelect := widget.NewSelect(audioSampleRateOptions, func(value string) {
		state.convert.AudioSampleRate = value
		logging.Debug(logging.CatUI, "audio sample rate set to %s", value)
	})
	audioSampleRateSelect.SetSelected(state.convert.AudioSampleRate)

	// Normalize Audio
	normalizeAudioCheck = widget.NewCheck(t.ConvertSectionAudioNormalize, func(checked bool) {
		state.convert.NormalizeAudio = checked
		logging.Debug(logging.CatUI, "normalize audio set to %v", checked)
	})
	normalizeAudioCheck.Checked = state.convert.NormalizeAudio

	// Normalize sliders (LUFS and TruePeak)
	normalizeLUFSValue := state.convert.NormalizeLUFS
	if normalizeLUFSValue == 0 {
		normalizeLUFSValue = -16.0
	}
	normalizeTruePeakValue := state.convert.NormalizeTruePeak
	if normalizeTruePeakValue == 0 {
		normalizeTruePeakValue = -1.5
	}
	lufsLabel := widget.NewLabel(fmt.Sprintf("%.0f LUFS", normalizeLUFSValue))
	truePeakLabel := widget.NewLabel(fmt.Sprintf("%.1f dBTP", normalizeTruePeakValue))
	normalizeLUFSSlider := ui.MakeSlider(-24, -9)
	normalizeLUFSSlider.Step = 1
	normalizeLUFSSlider.SetValue(normalizeLUFSValue)
	normalizeLUFSSlider.OnChanged = func(value float64) {
		state.convert.NormalizeLUFS = value
		lufsLabel.SetText(fmt.Sprintf("%.0f LUFS", value))
	}
	normalizeTruePeakSlider := ui.MakeSlider(-9, 0)
	normalizeTruePeakSlider.Step = 0.5
	normalizeTruePeakSlider.SetValue(normalizeTruePeakValue)
	normalizeTruePeakSlider.OnChanged = func(value float64) {
		state.convert.NormalizeTruePeak = value
		truePeakLabel.SetText(fmt.Sprintf("%.1f dBTP", value))
	}
	var normalizeSliderContainer *fyne.Container
	normalizeSliderContainer = container.NewVBox(
		container.NewGridWithColumns(2, lufsLabel, normalizeLUFSSlider),
		container.NewGridWithColumns(2, truePeakLabel, normalizeTruePeakSlider),
	)
	updateNormalizeSliders := func() {
		if state.convert.NormalizeAudio {
			normalizeSliderContainer.Show()
		} else {
			normalizeSliderContainer.Hide()
		}
	}
	normalizeAudioCheck.OnChanged = func(checked bool) {
		updateNormalizeSliders()
	}
	// Initialize visibility
	updateNormalizeSliders()

	// applyDevicePreset applies a complete hwPreset to all encoding state and widgets.
	// Defined here so all selects are in scope for the closure.
	applyDevicePreset = func(dp hwPreset) {
		// Apply container format first (updates codec compat lists etc.)
		if dp.FormatLabel != "" {
			for _, opt := range formatOptions {
				if opt.Label == dp.FormatLabel {
					applySelectedFormat(opt)
					formatContainer.SetSelected(opt.Label)
					break
				}
			}
		}

		// Apply video codec, H.264 profile/level, and reset tune.
		state.convert.VideoCodec = "H.264"
		state.convert.H264Profile = dp.H264Profile
		state.convert.H264Level = dp.H264Level
		state.convert.EncoderTune = "None"
		tuneSelect.SetSelected("None")
		videoCodecSelect.EnableAllOptions()
		if !dp.AllowHEVC {
			videoCodecSelect.DisableOption("H.265")
		}
		if !dp.AllowAV1 {
			videoCodecSelect.DisableOption("AV1")
		}
		videoCodecSelect.DisableOption("MPEG-2")
		videoCodecSelect.DisableOption("Copy")
		videoCodecSelect.SetSelected("H.264")

		// Apply quality via state manager (syncs all quality widgets).
		setQuality(dp.Quality)

		// Apply encoder preset on both simple and advanced selects.
		state.convert.EncoderPreset = dp.EncoderPreset
		encoderPresetSelect.SetSelected(dp.EncoderPreset)
		simplePresetSelect.SetSelected(dp.EncoderPreset)

		// Apply resolution via state manager (syncs both resolution widgets).
		if dp.TargetRes != "" {
			setResolution(dp.TargetRes)
		}

		// Apply pixel format.
		if dp.PixelFormat != "" {
			state.convert.PixelFormat = dp.PixelFormat
			pixelFormatSelect.SetSelected(dp.PixelFormat)
		}

		// Apply audio settings.
		if dp.AudioCodec != "" {
			state.convert.AudioCodec = dp.AudioCodec
			audioCodecSelect.SetSelected(dp.AudioCodec)
		}
		if dp.AudioBitrate != "" {
			state.convert.AudioBitrate = dp.AudioBitrate
			audioBitrateSelect.SetSelected(dp.AudioBitrate)
		}
		if dp.AudioChannels != "" {
			state.convert.AudioChannels = dp.AudioChannels
			audioChannelsSelect.SetSelected(dp.AudioChannels)
		}

		if updateQualityOptions != nil {
			updateQualityOptions()
		}
		if updateQualityVisibility != nil {
			updateQualityVisibility()
		}
		if buildCommandPreview != nil {
			buildCommandPreview()
		}
	}

	// applyUserPreset applies a saved user preset to all encoding state and widgets.
	applyUserPreset = func(up userPreset) {
		// Format
		if up.SelectedFormat.Label != "" {
			for _, opt := range formatOptions {
				if opt.Label == up.SelectedFormat.Label {
					applySelectedFormat(opt)
					formatContainer.SetSelected(opt.Label)
					break
				}
			}
		}

		// Video codec
		if up.VideoCodec != "" {
			state.convert.VideoCodec = up.VideoCodec
			videoCodecSelect.SetSelected(up.VideoCodec)
		}
		state.convert.H264Profile = up.H264Profile
		state.convert.H264Level = up.H264Level

		// Quality
		if up.Quality != "" {
			setQuality(up.Quality)
		}

		// Encoder preset
		if up.EncoderPreset != "" {
			state.convert.EncoderPreset = up.EncoderPreset
			encoderPresetSelect.SetSelected(up.EncoderPreset)
			simplePresetSelect.SetSelected(up.EncoderPreset)
		}

		// Bitrate mode
		if up.BitrateMode != "" {
			bitrateModeRadio.SetSelected(up.BitrateMode)
		}

		// Bitrate preset
		if up.BitratePreset != "" {
			setBitratePreset(up.BitratePreset)
		}

		// CRF (manual)
		if up.CRF != "" {
			state.convert.CRF = up.CRF
			if crfEntry != nil {
				crfEntry.SetText(up.CRF)
			}
		}

		// Resolution
		if up.TargetResolution != "" {
			setResolution(up.TargetResolution)
		}

		// Frame rate
		if up.FrameRate != "" {
			state.convert.FrameRate = up.FrameRate
			frameRateSelect.SetSelected(up.FrameRate)
			updateFrameRateHint()
		}

		// Motion interpolation
		state.convert.UseMotionInterpolation = up.UseMotionInterpolation
		motionInterpCheck.SetChecked(up.UseMotionInterpolation)

		// Pixel format
		if up.PixelFormat != "" {
			state.convert.PixelFormat = up.PixelFormat
			pixelFormatSelect.SetSelected(up.PixelFormat)
		}

		// Hardware accel
		if up.HardwareAccel != "" {
			state.convert.HardwareAccel = up.HardwareAccel
			hwAccelSelect.SetSelected(up.HardwareAccel)
		}

		// Two-pass
		state.convert.TwoPass = up.TwoPass
		twoPassCheck.SetChecked(up.TwoPass)

		// Tune
		tune := up.EncoderTune
		if tune == "" {
			tune = "None"
		}
		state.convert.EncoderTune = tune
		tuneSelect.SetSelected(tune)

		// Audio
		if up.AudioCodec != "" {
			state.convert.AudioCodec = up.AudioCodec
			audioCodecSelect.SetSelected(up.AudioCodec)
		}
		if up.AudioBitrate != "" {
			state.convert.AudioBitrate = up.AudioBitrate
			audioBitrateSelect.SetSelected(up.AudioBitrate)
		}
		if up.AudioChannels != "" {
			state.convert.AudioChannels = up.AudioChannels
			audioChannelsSelect.SetSelected(up.AudioChannels)
		}
		state.convert.AudioSampleRate = up.AudioSampleRate
		state.convert.NormalizeAudio = up.NormalizeAudio

		// Output / aspect
		state.convert.OutputAspect = up.OutputAspect
		state.convert.AspectHandling = up.AspectHandling
		state.convert.ForceAspect = up.ForceAspect
		state.convert.PreserveChapters = up.PreserveChapters
		preserveChaptersCheck.SetChecked(up.PreserveChapters)

		if updateQualityOptions != nil {
			updateQualityOptions()
		}
		if updateQualityVisibility != nil {
			updateQualityVisibility()
		}
		if updateEncodingControls != nil {
			updateEncodingControls()
		}
		if buildCommandPreview != nil {
			buildCommandPreview()
		}
	}

	// Now define updateDVDOptions with access to resolution and framerate selects
	wasDVD := false
	updateDVDOptions = func() {
		// Clear locks by default so non-DVD formats remain flexible
		resolutionSelectSimple.Enable()
		resolutionSelect.Enable()
		frameRateSelect.Enable()
		targetAspectSelectSimple.Enable()
		targetAspectSelect.Enable()
		pixelFormatSelect.Enable()
		hwAccelSelect.Enable()
		// videoCodecSelect.Enable()
		videoBitrateEntry.Enable()
		bitrateModeRadio.Enable()
		bitratePresetSelect.Enable()
		simpleBitrateSelect.Enable()
		targetFileSizeEntry.Enable()
		targetFileSizeSelect.Enable()
		crfEntry.Enable()
		bitratePresetSelect.Show()
		simpleBitrateSelect.Show()
		targetFileSizeEntry.Show()
		targetFileSizeSelect.Show()
		crfEntry.Show()

		isDVD := state.convert.SelectedFormat.Ext == ".mpg"
		if isDVD {
			if !strings.EqualFold(state.convert.TargetResolution, "NTSC (720x480)") &&
				!strings.EqualFold(state.convert.TargetResolution, "PAL (720x540)") &&
				!strings.EqualFold(state.convert.TargetResolution, "PAL (720x576)") {
				state.convert.TargetResolution = "Source"
			}
			if !strings.EqualFold(state.convert.FrameRate, "29.97") &&
				!strings.EqualFold(state.convert.FrameRate, "25") {
				state.convert.FrameRate = "Source"
			}
			if !strings.EqualFold(state.convert.OutputAspect, "4:3") &&
				!strings.EqualFold(state.convert.OutputAspect, "16:9") {
				state.convert.OutputAspect = "Source"
				state.convert.AspectUserSet = false
			}
			dvdAspectBox.Show()

			var (
				targetRes  string
				targetFPS  string
				targetAR   string
				dvdNotes   string
				dvdBitrate string
			)

			// Prefer the explicit DVD aspect select if set; otherwise derive from source
			targetAR = dvdAspectSelect.Selected

			if strings.Contains(state.convert.SelectedFormat.Label, "NTSC") {
				dvdNotes = "NTSC DVD: 720x480 @ 29.97fps, MPEG-2 Video, AC-3 Stereo 48kHz (bitrate 8000k, 9000k max PS2-safe)"
				targetRes = "NTSC (720x480)"
				targetFPS = "29.97"
				dvdBitrate = "8000k"
			} else if strings.Contains(state.convert.SelectedFormat.Label, "PAL") {
				dvdNotes = "PAL DVD: 720x540 @ 25fps, MPEG-2 Video, AC-3 Stereo 48kHz (bitrate 8000k default, 9500k max)"
				targetRes = "PAL (720x540)"
				targetFPS = "25"
				dvdBitrate = "8000k"
			} else {
				dvdNotes = "DVD format selected"
				targetRes = "NTSC (720x480)"
				targetFPS = "29.97"
				dvdBitrate = "8000k"
			}

			if strings.Contains(strings.ToLower(state.convert.SelectedFormat.Label), "4:3") {
				targetAR = "4:3"
			} else {
				targetAR = "16:9"
			}

			// If aspect still unset, derive from source
			if targetAR == "" || strings.EqualFold(targetAR, "Source") {
				if src != nil {
					if ar := displayAspectRatioForSource(src); ar > 0 && ar < 1.6 {
						targetAR = "4:3"
					} else {
						targetAR = "16:9"
					}
				} else {
					targetAR = "16:9"
				}
			}

			// Apply locked values for DVD compliance
			state.convert.TargetResolution = targetRes
			resolutionSelectSimple.SetSelected(targetRes)
			resolutionSelect.SetSelected(targetRes)
			resolutionSelectSimple.Disable()
			resolutionSelect.Disable()

			state.convert.FrameRate = targetFPS
			frameRateSelect.SetSelected(targetFPS)
			frameRateSelect.Disable()

			state.convert.OutputAspect = targetAR
			state.convert.AspectUserSet = false // DVD lock is not a user choice; don't persist as user-set
			targetAspectSelectSimple.SetSelected(targetAR)
			targetAspectSelect.SetSelected(targetAR)
			targetAspectSelectSimple.Disable()
			targetAspectSelect.Disable()
			dvdAspectSelect.SetSelected(targetAR)

			state.convert.PixelFormat = "yuv420p"
			pixelFormatSelect.SetSelected("yuv420p")
			pixelFormatSelect.Disable()

			state.convert.HardwareAccel = "none"
			hwAccelSelect.SetSelected("none")
			hwAccelSelect.Disable()

			state.convert.VideoCodec = "MPEG-2"
			videoCodecSelect.SetSelected("MPEG-2")
			// videoCodecSelect.Disable()

			state.convert.VideoBitrate = dvdBitrate
			if setManualBitrate != nil {
				setManualBitrate(dvdBitrate)
			} else {
				videoBitrateEntry.SetText(dvdBitrate)
			}
			videoBitrateEntry.Disable()
			state.convert.BitrateMode = "CBR"
			bitrateModeRadio.SetSelected("CBR")
			bitrateModeRadio.Disable()
			state.convert.BitratePreset = "Manual"
			bitratePresetSelect.SetSelected("Manual")
			bitratePresetSelect.Disable()
			simpleBitrateSelect.SetSelected("Manual")
			simpleBitrateSelect.Disable()
			targetFileSizeEntry.Disable()
			targetFileSizeSelect.Disable()
			crfEntry.Disable()

			// Hide bitrate/target-size fields to declutter in locked DVD mode
			bitratePresetSelect.Hide()
			simpleBitrateSelect.Hide()
			crfContainer.Hide()
			targetSizeContainer.Hide()
			// Show bitrate controls since DVD uses CBR
			bitrateContainer.Show()

			dvdInfoLabel.SetText(fmt.Sprintf("%s\nLocked: resolution, frame rate, aspect, codec, pixel format, bitrate, and GPU toggles for DVD compliance.", dvdNotes))
		} else {
			dvdAspectBox.Hide()
			// Reset DVD-locked values back to Source defaults when leaving DVD formats.
			if wasDVD {
				state.convert.TargetResolution = "Source"
				state.convert.FrameRate = "Source"
				state.convert.OutputAspect = "Source"
				state.convert.AspectUserSet = false
				resolutionSelectSimple.SetSelected("Source")
				resolutionSelect.SetSelected("Source")
				frameRateSelect.SetSelected("Source")
				targetAspectSelectSimple.SetSelected(sourceAspectLabel)
				targetAspectSelect.SetSelected(sourceAspectLabel)
				if src != nil {
					updateAspectBoxVisibility()
				}
			}
			// Re-enable normal visibility control through updateEncodingControls
			bitratePresetSelect.Show()
			simpleBitrateSelect.Show()
			if updateEncodingControls != nil {
				updateEncodingControls()
			}
		}
		wasDVD = isDVD
	}
	updateDVDOptions()

	qualitySectionSimple = container.NewVBox(
		widget.NewLabelWithStyle("--- QUALITY ---", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		qualitySelectSimple,
	)
	qualitySectionAdv = container.NewVBox(
		widget.NewLabelWithStyle(t.ConvertSectionQualityPreset, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		qualitySelectAdv,
	)

	updateQualityVisibility = func() {
		mode := normalizeBitrateMode(state.convert.BitrateMode)
		hideQuality := mode != "" && mode != "CRF"
		remux := strings.EqualFold(state.convert.SelectedFormat.VideoCodec, "copy") ||
			strings.EqualFold(state.convert.VideoCodec, "Copy")

		if qualitySectionSimple != nil {
			if hideQuality || remux {
				qualitySectionSimple.Hide()
			} else {
				qualitySectionSimple.Show()
			}
		}
		if qualitySectionAdv != nil {
			if hideQuality || remux {
				qualitySectionAdv.Hide()
			} else {
				qualitySectionAdv.Show()
			}
		}
	}
	// Call updateQualityVisibility now that the sections are created
	updateQualityVisibility()

	updateRemuxVisibility = func() {
		remux := strings.EqualFold(state.convert.SelectedFormat.VideoCodec, "copy") ||
			strings.EqualFold(state.convert.VideoCodec, "Copy")

		if simpleEncodingSection != nil {
			if remux {
				simpleEncodingSection.Hide()
			} else {
				simpleEncodingSection.Show()
			}
		}
		if advancedVideoEncodingBlock != nil {
			advancedVideoEncodingBlock.Show()
		}
		if audioEncodingSection != nil {
			audioEncodingSection.Show()
		}
		if videoCodecSelect != nil {
			if remux {
				state.convert.VideoCodec = "Copy"
				videoCodecSelect.SetSelected("Copy")
				videoCodecSelect.Disable()
			} else {
				videoCodecSelect.Enable()
			}
		}
		if audioCodecSelect != nil {
			if remux {
				state.convert.AudioCodec = "Copy"
				audioCodecSelect.SetSelected("Copy")
				audioCodecSelect.Disable()
			} else {
				audioCodecSelect.Enable()
			}
		}

		// Don't directly show/hide quality sections here - let updateQualityVisibility handle it
		// based on both remux state AND bitrate mode
		if updateQualityVisibility != nil {
			updateQualityVisibility()
		}
		if encoderPresetSelect != nil {
			if remux {
				encoderPresetSelect.Disable()
			} else {
				encoderPresetSelect.Enable()
			}
		}
		if bitrateModeRadio != nil {
			if remux {
				bitrateModeRadio.Hide()
				bitrateModeRadio.Disable()
			} else {
				bitrateModeRadio.Show()
				bitrateModeRadio.Enable()
			}
		}
		// Don't show/hide encoding containers here - let updateEncodingControls handle it
		// based on the selected bitrate mode (CRF/CBR/VBR/Target Size)
		if updateEncodingControls != nil {
			updateEncodingControls()
		}
		if encodingHintContainer != nil {
			if remux {
				encodingHintContainer.Hide()
			} else {
				encodingHintContainer.Show()
			}
		}
		if audioBitrateSelect != nil {
			if remux {
				audioBitrateSelect.Disable()
			} else {
				audioBitrateSelect.Enable()
			}
		}
		if audioChannelsSelect != nil {
			if remux {
				audioChannelsSelect.Disable()
			} else {
				audioChannelsSelect.Enable()
			}
		}
		if videoBitrateEntry != nil {
			if remux {
				videoBitrateEntry.Disable()
			} else {
				videoBitrateEntry.Enable()
			}
		}
		if bitratePresetSelect != nil {
			if remux {
				bitratePresetSelect.Hide()
				bitratePresetSelect.Disable()
			} else {
				bitratePresetSelect.Show()
				bitratePresetSelect.Enable()
			}
		}
		if simpleBitrateSelect != nil {
			if remux {
				simpleBitrateSelect.Hide()
				simpleBitrateSelect.Disable()
			} else {
				simpleBitrateSelect.Show()
				simpleBitrateSelect.Enable()
			}
		}
		if targetFileSizeEntry != nil {
			if remux {
				targetFileSizeEntry.Disable()
			} else {
				targetFileSizeEntry.Enable()
			}
		}
		if targetFileSizeSelect != nil {
			if remux {
				targetFileSizeSelect.Disable()
			} else {
				targetFileSizeSelect.Enable()
			}
		}
		if crfEntry != nil {
			if remux {
				crfEntry.Disable()
			} else {
				crfEntry.Enable()
			}
		}
		if motionInterpCheck != nil {
			if remux {
				motionInterpCheck.Disable()
			} else {
				motionInterpCheck.Enable()
			}
		}
		if twoPassCheck != nil {
			if remux {
				twoPassCheck.Disable()
			} else {
				twoPassCheck.Enable()
			}
		}
		if resolutionSelectSimple != nil {
			if remux {
				state.convert.TargetResolution = "Source"
				resolutionSelectSimple.SetSelected("Source")
				resolutionSelectSimple.Disable()
			} else {
				resolutionSelectSimple.Enable()
			}
		}
		if resolutionSelect != nil {
			if remux {
				resolutionSelect.SetSelected("Source")
				resolutionSelect.Disable()
			} else {
				resolutionSelect.Enable()
			}
		}
		if frameRateSelect != nil {
			if remux {
				state.convert.FrameRate = "Source"
				frameRateSelect.SetSelected("Source")
				frameRateSelect.Disable()
			} else {
				frameRateSelect.Enable()
			}
		}
		if pixelFormatSelect != nil {
			if remux {
				pixelFormatSelect.Disable()
			} else {
				pixelFormatSelect.Enable()
			}
		}
		if hwAccelSelect != nil {
			if remux {
				hwAccelSelect.Disable()
			} else {
				hwAccelSelect.Enable()
			}
		}
		if remux {
			setAspect("Source", false)
			if targetAspectSelectSimple != nil {
				targetAspectSelectSimple.Disable()
			}
			if targetAspectSelect != nil {
				targetAspectSelect.Disable()
			}
			aspectOptions.Disable()
			aspectBox.Hide()
		} else {
			if targetAspectSelectSimple != nil {
				targetAspectSelectSimple.Enable()
			}
			if targetAspectSelect != nil {
				targetAspectSelect.Enable()
			}
			aspectOptions.Enable()
			if updateAspectBoxVisibility != nil {
				updateAspectBoxVisibility()
			}
		}
	}

	simpleEncodingSection = buildConvertBox(t.ConvertSectionVideoEncoding, container.NewVBox(
		qualitySectionSimple,
		widget.NewLabelWithStyle(t.ConvertSectionEncoderSpeed, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(t.ConvertEncoderPresetHint),
		widget.NewLabelWithStyle(t.ConvertSectionEncoderPreset, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		simplePresetSelect,
		widget.NewSeparator(),
		widget.NewLabelWithStyle(t.ConvertSectionBitrateSimple, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		simpleBitrateSelect,
	))

	outputSectionSimple := buildConvertBox(t.ConvertSectionOutput, container.NewVBox(
		widget.NewLabelWithStyle(t.ConvertSectionFormat, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		formatContainer,
		chapterWarningLabel, // Warning when converting chapters to DVD
		preserveChaptersCheck,
		dvdAspectBox, // DVD options appear here when DVD format selected
		widget.NewSeparator(),
		widget.NewLabelWithStyle(t.ConvertSectionOutputFolder, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		outputDirRow,
		widget.NewSeparator(),
		widget.NewLabelWithStyle(t.ConvertSectionOutputFilename, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		outputNameRow,
		outputHintContainer,
		appendSuffixCheck,
	))

	resolutionSectionSimple := buildConvertBox(t.ConvertSectionResolutionFPS, container.NewVBox(
		widget.NewLabelWithStyle(t.ConvertSectionTargetResolution, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		resolutionSelectSimple,
		widget.NewSeparator(),
		widget.NewLabelWithStyle(t.ConvertSectionFrameRate, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		frameRateSelect,
		motionInterpCheck,
	))

	aspectSectionSimple := buildConvertBox(t.ConvertSectionAspectRatio, container.NewVBox(
		widget.NewLabelWithStyle(t.ConvertSectionTargetAspect, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		targetAspectSelectSimple,
		targetAspectHintContainer,
		widget.NewSeparator(),
		customAspectBoxSimple,
		makeForceAspectCheck(),
	))

	devicePresetSectionSimple := buildConvertBox(t.ConvertDevicePresets, container.NewVBox(
		widget.NewLabel(t.ConvertDevicePresetHint),
		devicePresetSelect,
	))

	userPresetSectionSimple := buildConvertBox(t.ConvertUserPresets, container.NewVBox(
		container.NewBorder(nil, nil, nil, deleteUserPresetBtn, userPresetSelect),
		saveUserPresetBtn,
	))

	// Simple mode options - minimal controls, aspect locked to Source
	simpleOptions := container.NewVBox(
		devicePresetSectionSimple,
		sectionGap(),
		userPresetSectionSimple,
		sectionGap(),
		outputSectionSimple,
		sectionGap(),
		simpleEncodingSection,
		sectionGap(),
		resolutionSectionSimple,
		sectionGap(),
		aspectSectionSimple,
		layout.NewSpacer(),
	)

	// Advanced mode options - full controls with organized sections
	videoCodecLabel := widget.NewLabelWithStyle(t.ConvertSectionVideoCodec, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	presetLabel := widget.NewLabelWithStyle(t.ConvertSectionEncoderPreset, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	videoCodecRow := ui.NewRatioRowWithGap(videoCodecLabel, presetLabel, 0.3, 10)
	videoCodecControls := ui.NewRatioRowWithGap(
		container.NewPadded(videoCodecContainer),
		container.NewPadded(encoderPresetSelect),
		0.3,
		10,
	)

	// H.264 Profile and Level (shown when H.264 is selected)
	h264ProfileOptions := []string{"Baseline", "Main", "High", "High 10"}
	h264ProfileSelect := widget.NewSelect(h264ProfileOptions, func(value string) {
		state.convert.H264Profile = value
		logging.Debug(logging.CatUI, "H.264 profile set to %s", value)
	})
	h264ProfileSelect.SetSelected(state.convert.H264Profile)

	h264LevelOptions := []string{"3.0", "3.1", "4.0", "4.1", "4.2", "5.0", "5.1", "5.2"}
	h264LevelSelect := widget.NewSelect(h264LevelOptions, func(value string) {
		state.convert.H264Level = value
		logging.Debug(logging.CatUI, "H.264 level set to %s", value)
	})
	h264LevelSelect.SetSelected(state.convert.H264Level)

	profileLevelContainer = container.NewVBox(
		container.NewGridWithColumns(2,
			container.NewPadded(container.NewVBox(
				widget.NewLabelWithStyle(t.ConvertH264Profile, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				h264ProfileSelect,
			)),
			container.NewPadded(container.NewVBox(
				widget.NewLabelWithStyle(t.ConvertH264Level, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				h264LevelSelect,
			)),
		),
	)
	profileLevelContainer.Hide()

	tuneOptions := []string{"None", "Film", "Animation", "Grain", "Stillimage", "Fastdecode"}
	tuneSelect = widget.NewSelect(tuneOptions, func(value string) {
		state.convert.EncoderTune = value
		if buildCommandPreview != nil {
			buildCommandPreview()
		}
	})
	if state.convert.EncoderTune == "" {
		state.convert.EncoderTune = "None"
	}
	tuneSelect.SetSelected(state.convert.EncoderTune)
	tuneContainer = container.NewVBox(
		container.NewGridWithColumns(2,
			container.NewPadded(container.NewVBox(
				widget.NewLabelWithStyle(t.ConvertSectionEncoderTune, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				tuneSelect,
			)),
		),
	)
	if state.convert.VideoCodec != "H.264" && state.convert.VideoCodec != "H.265" {
		tuneContainer.Hide()
	}

	advancedVideoEncodingBlock = buildConvertBox(t.ConvertSectionVideoEncoding, container.NewVBox(
		videoCodecRow,
		videoCodecControls,
		encoderPresetHintContainer,
		profileLevelContainer,
		tuneContainer,
		widget.NewSeparator(),
		widget.NewLabelWithStyle(t.ConvertSectionBitrateMode, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		bitrateModeRadio,
		qualitySectionAdv,
		crfContainer,
		bitrateContainer,
		targetSizeContainer,
		encodingHintContainer,
		widget.NewSeparator(),
		widget.NewLabelWithStyle(t.ConvertSectionTargetResolution, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		resolutionSelect,
		widget.NewSeparator(),
		widget.NewLabelWithStyle(t.ConvertSectionFrameRate, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		frameRateSelect,
		frameRateHintContainer,
		motionInterpCheck,
		widget.NewSeparator(),
		widget.NewLabelWithStyle(t.ConvertSectionPixelFormat, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		pixelFormatSelect,
		widget.NewSeparator(),
		widget.NewLabelWithStyle(t.ConvertSectionHardwareAccel, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		hwAccelSelect,
		hwAccelHintContainer,
		twoPassCheck,
		twoPassNote,
	))

	audioEncodingSection = buildConvertBox(t.ConvertSectionAudioEncoding, container.NewVBox(
		widget.NewLabelWithStyle(t.ConvertSectionAudioCodec, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		audioCodecContainer,
		widget.NewSeparator(),
		widget.NewLabelWithStyle(t.ConvertSectionAudioBitrate, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		audioBitrateSelect,
		widget.NewSeparator(),
		widget.NewLabelWithStyle(t.ConvertSectionAudioChannels, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		audioChannelsSelect,
		widget.NewSeparator(),
		widget.NewLabelWithStyle(t.ConvertSectionAudioSampleRate, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		audioSampleRateSelect,
		widget.NewSeparator(),
		normalizeAudioCheck,
		normalizeSliderContainer,
	))

	outputSectionAdvanced := buildConvertBox(t.ConvertSectionOutput, container.NewVBox(
		widget.NewLabelWithStyle(t.ConvertSectionFormat, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		formatContainer,
		chapterWarningLabel, // Warning when converting chapters to DVD
		preserveChaptersCheck,
		dvdAspectBox, // DVD options appear here when DVD format selected
		widget.NewSeparator(),
		widget.NewLabelWithStyle(t.ConvertSectionOutputFolder, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		outputDirRow,
		widget.NewSeparator(),
		widget.NewLabelWithStyle(t.ConvertSectionOutputFilename, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		outputNameRow,
		outputHintContainer,
		appendSuffixCheck,
	))

	aspectSectionAdvanced := buildConvertBox(t.ConvertSectionAspectRatio, container.NewVBox(
		widget.NewLabelWithStyle(t.ConvertSectionTargetAspect, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		targetAspectSelect,
		targetAspectHintContainer,
		widget.NewSeparator(),
		customAspectBox,
		aspectBox,
		makeForceAspectCheck(),
	))

	autoCropSection := buildConvertBox(t.ConvertSectionAutoCrop, container.NewVBox(
		autoCropCheck,
		detectCropView,
		autoCropHint,
	))

	transformSection := buildConvertBox(t.ConvertSectionTransformations, container.NewVBox(
		flipHorizontalCheck,
		flipVerticalCheck,
		widget.NewLabelWithStyle(t.ConvertSectionRotation, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		rotationSelect,
		transformHint,
	))

	deinterlaceSection := buildConvertBox(t.ConvertSectionDeinterlacing, container.NewVBox(
		widget.NewLabelWithStyle(t.ConvertDeinterlaceMode, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		deinterlaceModeSelect,
		widget.NewLabelWithStyle(t.ConvertDeinterlaceMethod, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		deinterlaceMethodSelect,
		widget.NewSeparator(),
		analyzeInterlaceView,
		inverseCheck,
		inverseHint,
	))

	devicePresetSectionAdv := buildConvertBox(t.ConvertDevicePresets, container.NewVBox(
		widget.NewLabel(t.ConvertDevicePresetHint),
		devicePresetSelect,
	))

	userPresetSectionAdv := buildConvertBox(t.ConvertUserPresets, container.NewVBox(
		container.NewBorder(nil, nil, nil, deleteUserPresetBtn, userPresetSelect),
		saveUserPresetBtn,
	))

	advancedOptions := container.NewVBox(
		devicePresetSectionAdv,
		sectionGap(),
		userPresetSectionAdv,
		sectionGap(),
		outputSectionAdvanced,
		sectionGap(),
		advancedVideoEncodingBlock,
		sectionGap(),
		aspectSectionAdvanced,
		sectionGap(),
		audioEncodingSection,
		sectionGap(),
		autoCropSection,
		sectionGap(),
		transformSection,
		sectionGap(),
		deinterlaceSection,
		layout.NewSpacer(),
	)

	resetConvertDefaults = func() {
		state.convert = defaultConvertConfig()
		logging.Debug(logging.CatUI, "convert settings reset to defaults")

		tabs.SelectIndex(0)
		state.convert.Mode = "Simple"

		// Format selection handled below in UI redesign
		devicePresetSelect.SetSelected("None")
		userPresetSelect.SetSelected("None")
		videoCodecSelect.SetSelected(state.convert.VideoCodec)
		qualitySelectSimple.SetSelected(state.convert.Quality)
		qualitySelectAdv.SetSelected(state.convert.Quality)
		simplePresetSelect.SetSelected(state.convert.EncoderPreset)
		encoderPresetSelect.SetSelected(state.convert.EncoderPreset)
		bitrateModeRadio.SetSelected(state.convert.BitrateMode)
		bitratePresetSelect.SetSelected(state.convert.BitratePreset)
		simpleBitrateSelect.SetSelected(state.convert.BitratePreset)
		crfEntry.SetText(state.convert.CRF)
		if manualCrfRow != nil {
			manualCrfRow.Hide()
		}
		setManualBitrate(state.convert.VideoBitrate)
		targetFileSizeSelect.SetSelected("Manual")
		setTargetFileSize(state.convert.TargetFileSize)
		autoNameCheck.SetChecked(state.convert.UseAutoNaming)
		autoNameTemplate.SetText(state.convert.AutoNameTemplate)
		appendSuffixCheck.SetChecked(state.convert.AppendSuffix)
		outputEntry.SetText(state.convert.OutputBase)
		outputDirEntry.SetText(state.convert.OutputDir)
		updateOutputHint()
		preserveChaptersCheck.SetChecked(state.convert.PreserveChapters)
		resolutionSelectSimple.SetSelected(state.convert.TargetResolution)
		resolutionSelect.SetSelected(state.convert.TargetResolution)
		frameRateSelect.SetSelected(state.convert.FrameRate)
		updateFrameRateHint()
		motionInterpCheck.SetChecked(state.convert.UseMotionInterpolation)
		setAspect(state.convert.OutputAspect, false)
		aspectOptions.SetSelected(state.convert.AspectHandling)
		pixelFormatSelect.SetSelected(state.convert.PixelFormat)
		hwAccelSelect.SetSelected(state.convert.HardwareAccel)
		twoPassCheck.SetChecked(state.convert.TwoPass)
		audioCodecSelect.SetSelected(state.convert.AudioCodec)
		audioBitrateSelect.SetSelected(state.convert.AudioBitrate)
		audioChannelsSelect.SetSelected(state.convert.AudioChannels)
		audioSampleRateSelect.SetSelected(state.convert.AudioSampleRate)
		normalizeAudioCheck.SetChecked(state.convert.NormalizeAudio)
		normalizeLUFSSlider.SetValue(state.convert.NormalizeLUFS)
		normalizeTruePeakSlider.SetValue(state.convert.NormalizeTruePeak)
		deinterlaceModeSelect.SetSelected(state.convert.Deinterlace)
		deinterlaceMethodSelect.SetSelected(state.convert.DeinterlaceMethod)
		h264ProfileSelect.SetSelected(state.convert.H264Profile)
		h264LevelSelect.SetSelected(state.convert.H264Level)
		cacheDirEntry.SetText(state.convert.TempDir)
		utils.SetTempDir(state.convert.TempDir)
		logsDirEntry.SetText(state.convert.LogDir)
		setLogsDirOverride(state.convert.LogDir)
		logging.SetLogsDir(getLogsDir())
		logging.Reopen()
		inverseCheck.SetChecked(state.convert.InverseTelecine)
		inverseHint.SetText(state.convert.InverseAutoNotes)
		coverLabel.SetText(state.convert.CoverLabel())
		if coverDisplay != nil {
			coverDisplay.SetText(t.ConvertCoverArtLabel + ": " + state.convert.CoverLabel())
		}

		updateAspectBoxVisibility()
		if updateDVDOptions != nil {
			updateDVDOptions()
		}
		// Re-apply defaults in case DVD options toggled any locks
		state.convert.TargetResolution = "Source"
		state.convert.FrameRate = "Source"
		resolutionSelectSimple.SetSelected("Source")
		resolutionSelect.SetSelected("Source")
		frameRateSelect.SetSelected("Source")
		updateFrameRateHint()
		if updateEncodingControls != nil {
			updateEncodingControls()
		}
		if updateQualityVisibility != nil {
			updateQualityVisibility()
		}
		if updateRemuxVisibility != nil {
			updateRemuxVisibility()
		}
		state.persistConvertConfig()
	}

	// Create tabs for Simple/Advanced modes
	// Wrap simple options with settings box at top
	simpleWithSettings := container.NewVBox(settingsBox, simpleOptions)

	// Both Simple and Advanced get their own fast scrolling (12x speed)
	simpleScrollBox := ui.NewFastVScroll(simpleWithSettings)
	advancedScrollBox := ui.NewFastVScroll(advancedOptions)

	if updateQualityVisibility != nil {
		updateQualityVisibility()
	}
	if updateRemuxVisibility != nil {
		updateRemuxVisibility()
	}
	if updateEncodingControls != nil {
		updateEncodingControls()
	}

	tabs = container.NewAppTabs(
		container.NewTabItem(t.ConvertTabSimple, simpleScrollBox),
		container.NewTabItem(t.ConvertTabAdvanced, advancedScrollBox),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	// Set initial tab based on mode
	if state.convert.Mode == "Advanced" {
		tabs.SelectIndex(1)
	}

	// Update mode when tab changes
	tabs.OnSelected = func(item *container.TabItem) {
		if item.Text == t.ConvertTabSimple {
			state.convert.Mode = "Simple"
			logging.Debug(logging.CatUI, "convert mode selected: Simple")
		} else {
			state.convert.Mode = "Advanced"
			logging.Debug(logging.CatUI, "convert mode selected: Advanced")
		}
	}

	if !state.convertScrollShortcuts && state.window != nil {
		state.convertScrollShortcuts = true
		isTextEntryFocused := func() bool {
			c := state.window.Canvas()
			if c == nil {
				return false
			}
			switch c.Focused().(type) {
			case *widget.Entry:
				return true
			default:
				return false
			}
		}

		scrollActive := func() *ui.FastVScroll {
			if tabs.Selected() != nil && tabs.Selected().Text == "Advanced" {
				return advancedScrollBox
			}
			return simpleScrollBox
		}

		state.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeySpace}, func(_ fyne.Shortcut) {
			if state.active != "convert" || isTextEntryFocused() {
				return
			}
			scroll := scrollActive()
			if scroll == nil {
				return
			}
			scroll.ScrollBy(scroll.PageStep())
		})
		state.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeySpace, Modifier: fyne.KeyModifierControl}, func(_ fyne.Shortcut) {
			if state.active != "convert" || isTextEntryFocused() {
				return
			}
			scroll := scrollActive()
			if scroll == nil {
				return
			}
			scroll.ScrollBy(-scroll.PageStep())
		})
	}

	optionsRect := canvas.NewRectangle(utils.MustHex("#13182B"))
	optionsRect.CornerRadius = 8
	optionsRect.StrokeColor = gridColor
	optionsRect.StrokeWidth = 1
	settingsTabsPanel := container.NewMax(optionsRect, container.NewPadded(tabs))
	settingsHeader, _ := ui.BuildCollapsibleHeader(t.ConvertSectionSettings, convertColor, func(open bool) {
		if open {
			mainSplit.SetOffset(0.65)
		} else {
			mainSplit.SetOffset(0.97)
		}
	})
	optionsPanel := container.NewBorder(settingsHeader, nil, nil, nil, settingsTabsPanel)

	// Initialize snippet settings defaults
	if state.snippetLength == 0 {
		state.snippetLength = 20 // Default to 20 seconds
	}

	// Snippet length configuration
	snippetLengthLabel := widget.NewLabel(fmt.Sprintf(t.ConvertSnippetLengthFmt, state.snippetLength))
	snippetLengthSlider := ui.MakeSlider(5, 60)
	snippetLengthSlider.SetValue(float64(state.snippetLength))
	snippetLengthSlider.Step = 1
	snippetLengthSlider.OnChanged = func(value float64) {
		state.snippetLength = int(value)
		snippetLengthLabel.SetText(fmt.Sprintf(t.ConvertSnippetLengthFmt, state.snippetLength))
	}

	// Snippet output mode
	snippetModeLabel := widget.NewLabel(t.ConvertSnippetOutput)
	snippetModeCheck := widget.NewCheck(t.ConvertMatchSourceFormat, func(checked bool) {
		state.snippetSourceFormat = checked
	})
	snippetModeCheck.SetChecked(state.snippetSourceFormat)
	snippetModeHint := widget.NewLabel(t.ConvertUseConvSettings)
	snippetModeHint.TextStyle = fyne.TextStyle{Italic: true}

	// Snippet position mode
	snippetPositionLabel := widget.NewLabel(t.ConvertSnippetPosition)
	snippetPositionCheck := widget.NewCheck(t.ConvertSnippetFromCurrent, func(checked bool) {
		state.snippetFromCurrent = checked
	})
	snippetPositionCheck.SetChecked(state.snippetFromCurrent)
	snippetPositionHint := widget.NewLabel(t.ConvertSnippetFromCurrentHint)
	snippetPositionHint.TextStyle = fyne.TextStyle{Italic: true}

	snippetConfigRow := container.NewVBox(
		snippetLengthLabel,
		snippetLengthSlider,
		widget.NewSeparator(),
		snippetModeLabel,
		snippetModeCheck,
		snippetModeHint,
		widget.NewSeparator(),
		snippetPositionLabel,
		snippetPositionCheck,
		snippetPositionHint,
	)

	snippetBtn := ui.MakePillButton(t.ConvertGenerateSnippet, ui.BorderDim, func() {
		if state.source == nil {
			dialog.ShowInformation(t.DialogSnippet, t.DialogLoadVideoFirst, state.window)
			return
		}
		if state.jobQueue == nil {
			dialog.ShowInformation(t.MenuQueue, t.DialogQueueNotInit, state.window)
			return
		}
		src := state.source

		// Determine output extension based on mode
		var ext string
		if state.snippetSourceFormat {
			// High Quality mode: use source extension
			ext = filepath.Ext(src.Path)
			if ext == "" {
				ext = ".mp4"
			}
		} else {
			// Conversion Settings mode: use configured output format
			ext = state.convert.SelectedFormat.Ext
			if ext == "" {
				ext = ".mp4"
			}
		}

		var startPosition float64 = -1 // -1 means use midpoint (default behavior)
		positionDesc := "midpoint"
		if state.snippetFromCurrent {
			if HasNativeMediaPlayer() {
				cp := GetPrimaryPlayer()
				if cp != nil {
					startPosition = cp.CurrentTime()
					positionDesc = "current position"
				} else {
					dialog.ShowInformation(t.DialogSnippet, t.ConvertSnippetNoVideoMsg, state.window)
				}
			} else {
				dialog.ShowInformation(t.DialogSnippet, t.ConvertSnippetNoVideoMsg, state.window)
			}
		}

		outName := fmt.Sprintf("%s-snippet-%d%s", strings.TrimSuffix(src.DisplayName, filepath.Ext(src.DisplayName)), time.Now().Unix(), ext)
		outPath := filepath.Join(filepath.Dir(src.Path), outName)

		modeDesc := "conversion settings"
		if state.snippetSourceFormat {
			modeDesc = "source format"
		}

		job := &queue.Job{
			Type:        queue.JobTypeSnippet,
			Title:       "Snippet: " + filepath.Base(src.Path),
			Description: fmt.Sprintf("%ds snippet at %s (%s)", state.snippetLength, positionDesc, modeDesc),
			InputFile:   src.Path,
			OutputFile:  outPath,
			Config: map[string]interface{}{
				"inputPath":       src.Path,
				"outputPath":      outPath,
				"outputExt":       ext,
				"snippetLength":   float64(state.snippetLength),
				"useSourceFormat": state.snippetSourceFormat,
				"startPosition":   startPosition,
			},
		}
		state.generateJobThumbnail(job)
		state.jobQueue.Add(job)
		if !state.jobQueue.IsRunning() {
			state.jobQueue.Start()
		}
		dialog.ShowInformation(t.DialogSnippet, fmt.Sprintf(t.ConvertSnippetJobQueuedFmt, state.snippetLength), state.window)
	})
	if src == nil {
		snippetBtn.Disable()
	}

	// Button to generate snippets for all loaded videos
	var snippetAllBtn *ui.PillButton
	if len(state.loadedVideos) > 1 {
		snippetAllBtn = ui.MakePillButton(t.ConvertGenerateAllSnippets, ui.Magenta, func() {
			if state.jobQueue == nil {
				dialog.ShowInformation(t.MenuQueue, t.DialogQueueNotInit, state.window)
				return
			}

			timestamp := time.Now().Unix()
			jobsAdded := 0

			modeDesc := "conversion settings"
			if state.snippetSourceFormat {
				modeDesc = "source format"
			}

			for _, src := range state.loadedVideos {
				if src == nil {
					continue
				}

				// Determine output extension based on mode
				var ext string
				if state.snippetSourceFormat {
					// High Quality mode: use source extension
					ext = filepath.Ext(src.Path)
					if ext == "" {
						ext = ".mp4"
					}
				} else {
					// Conversion Settings mode: use configured output format
					ext = state.convert.SelectedFormat.Ext
					if ext == "" {
						ext = ".mp4"
					}
				}

				outName := fmt.Sprintf("%s-snippet-%d%s", strings.TrimSuffix(src.DisplayName, filepath.Ext(src.DisplayName)), timestamp, ext)
				outPath := filepath.Join(filepath.Dir(src.Path), outName)

				// Determine start position
				var startPosition float64 = -1
				positionDesc := "midpoint"
				if state.snippetFromCurrent {
					if HasNativeMediaPlayer() {
						cp := GetPrimaryPlayer()
						if cp != nil {
							startPosition = cp.CurrentTime()
							positionDesc = "current position"
						}
					}
				}

				job := &queue.Job{
					Type:        queue.JobTypeSnippet,
					Title:       "Snippet: " + filepath.Base(src.Path),
					Description: fmt.Sprintf("%ds snippet at %s (%s)", state.snippetLength, positionDesc, modeDesc),
					InputFile:   src.Path,
					OutputFile:  outPath,
					Config: map[string]interface{}{
						"inputPath":       src.Path,
						"outputPath":      outPath,
						"outputExt":       ext,
						"snippetLength":   float64(state.snippetLength),
						"useSourceFormat": state.snippetSourceFormat,
						"startPosition":   startPosition,
					},
				}
				state.generateJobThumbnail(job)
				state.jobQueue.Add(job)
				jobsAdded++
			}

			if jobsAdded > 0 {
				if !state.jobQueue.IsRunning() {
					state.jobQueue.Start()
				}
				dialog.ShowInformation(t.DialogSnippets,
					fmt.Sprintf(t.ConvertSnippetAllQueuedFmt, jobsAdded, state.snippetLength),
					state.window)
			}
		})
	}

	snippetHint := widget.NewLabel(t.ConvertSnippetHint)

	var snippetOptionsBtn *ui.PillButton
	snippetOptionsBtn = ui.MakePillButton(t.ConvertSnippetOptions, ui.BorderDim, func() {
		if snippetDrawer != nil {
			snippetDrawer.Hide()
			snippetDrawer = nil
			return
		}
		snippetDrawer = buildBottomDrawer(t.ConvertSnippetOptions, snippetConfigRow, func() {
			if snippetDrawer != nil {
				snippetDrawer.Hide()
				snippetDrawer = nil
			}
		})
	})
	if src == nil {
		snippetOptionsBtn.Disable()
	}

	var snippetRow fyne.CanvasObject
	if snippetAllBtn != nil {
		snippetRow = container.NewHBox(snippetBtn, snippetAllBtn, snippetOptionsBtn, layout.NewSpacer(), snippetHint)
	} else {
		snippetRow = container.NewHBox(snippetBtn, snippetOptionsBtn, layout.NewSpacer(), snippetHint)
	}
	snippetPad := canvas.NewRectangle(color.Transparent)
	snippetPad.SetMinSize(fyne.NewSize(10, 0))
	snippetRow = container.NewBorder(nil, nil, snippetPad, snippetPad, snippetRow)

	// Left column: use VSplit to give 50% vertical space to video and metadata each.
	// videoPanelWithHeader wraps the player canvas in a collapsible header bar so
	// the user can collapse the player to give the full column to the metadata panel.
	// Do NOT wrap in VBox — VBox only gives children their minimum height, leaving
	// the rest of the VSplit's allocated space as an empty dark gap.
	metaPanelScroll := ui.NewFastVScroll(metaPanel)
	leftColumn = container.NewVSplit(videoPanelWithHeader, metaPanelScroll)
	leftColumn.SetOffset(0.5) // 50/50 split between video and metadata

	// Split: left side (player + metadata) takes priority | right side (settings).
	mainSplit = container.NewHSplit(
		leftColumn,
		optionsPanel)
	mainSplit.SetOffset(0.65) // 65/35 split

	// Add horizontal padding around the split (10px on each side)
	mainContent := container.NewPadded(mainSplit)

	resetBtn := ui.MakePillButton(t.ConvertReset, ui.BorderDim, func() {
		if resetConvertDefaults != nil {
			resetConvertDefaults()
		}
	})
	statusLabel := widget.NewLabel("")
	statusLabel.Wrapping = fyne.TextTruncate // Prevent text wrapping to new line
	if state.convertBusy {
		statusLabel.SetText(state.convertStatus)
	} else if src != nil {
		statusLabel.SetText(t.ConvertReadyToConvert)
	} else {
		statusLabel.SetText(t.ConvertLoadVideoToConvert)
	}
	activity := widget.NewProgressBarInfinite()
	activity.Stop()
	activity.Hide()
	if state.convertBusy {
		activity.Show()
		activity.Start()
	}
	var convertBtn *ui.PillButton
	var cancelBtn *ui.PillButton
	var cancelQueueBtn *ui.PillButton
	cancelBtn = ui.MakePillButton(t.ActionCancel, ui.BorderDim, func() {
		state.cancelConvert(cancelBtn, convertBtn, activity, statusLabel)
	})
	cancelBtn.Disable()

	cancelQueueBtn = ui.MakePillButton(t.ConvertActionCancelJob, ui.BorderDim, func() {
		if state.jobQueue == nil {
			dialog.ShowInformation(t.DialogCancel, t.DialogQueueNotInit, state.window)
			return
		}
		job := state.jobQueue.CurrentRunning()
		if job == nil {
			dialog.ShowInformation(t.DialogCancel, t.DialogNoRunningJob, state.window)
			return
		}
		if err := state.jobQueue.Cancel(job.ID); err != nil {
			dialog.ShowError(fmt.Errorf("failed to cancel job: %w", err), state.window)
			return
		}
		dialog.ShowInformation(t.DialogCancelled, fmt.Sprintf(t.DialogJobCancelledFmt, job.Title), state.window)
	})
	cancelQueueBtn.Disable()

	// Add to Queue button
	addQueueBtn := ui.MakePillButton(t.ActionAddToQueue, ui.BorderDim, func() {
		state.persistConvertConfig()
		state.executeAddToQueue()
	})

	// Add All to Queue button (only shown when multiple videos are loaded)
	addAllQueueBtn := ui.MakePillButton(t.ConvertAddAllToQueue, ui.BorderDim, func() {
		state.persistConvertConfig()
		state.executeAddAllToQueue()
	})
	if len(state.loadedVideos) <= 1 {
		addAllQueueBtn.Hide()
	}

	convertBtn = ui.MakePillButton(t.ConvertActionStart, ui.Magenta, func() {
		state.persistConvertConfig()
		state.executeConversion()
	})
	if src == nil {
		convertBtn.Disable()
	}

	viewLogBtn := ui.MakePillButton(t.ConvertViewLog, ui.BorderDim, func() {
		if state.convertActiveLog == "" {
			dialog.ShowInformation(t.DialogNoLog, t.DialogNoLogMsg, state.window)
			return
		}
		state.openLogViewer("Conversion Log", state.convertActiveLog, state.convertBusy)
	})
	if state.convertActiveLog == "" {
		viewLogBtn.Disable()
	}
	if state.convertBusy {
		// Allow queueing new jobs while current convert runs; just disable Convert Now and enable Cancel.
		convertBtn.Disable()
		cancelBtn.Enable()
		addQueueBtn.Enable()
		if len(state.loadedVideos) > 1 {
			addAllQueueBtn.Enable()
		}
		if state.convertActiveLog != "" {
			viewLogBtn.Enable()
		}
	}
	// Also disable if queue is running
	if state.jobQueue != nil && state.jobQueue.IsRunning() {
		convertBtn.Disable()
		addQueueBtn.Enable()
		if len(state.loadedVideos) > 1 {
			addAllQueueBtn.Enable()
		}
	}

	// Keyboard shortcut: Ctrl+Enter (Cmd+Enter on macOS maps to Super) -> Convert Now
	if c := state.window.Canvas(); c != nil {
		triggerNow := func() {
			if convertBtn != nil && !convertBtn.Disabled() {
				if convertBtn.OnTapped != nil {
					convertBtn.OnTapped()
				}
			}
		}
		c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyReturn, Modifier: fyne.KeyModifierControl}, func(fyne.Shortcut) {
			triggerNow()
		})
		c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyEnter, Modifier: fyne.KeyModifierControl}, func(fyne.Shortcut) {
			triggerNow()
		})
	}

	// Auto-compare checkbox
	autoCompareCheck := widget.NewCheck(t.ConvertCompareAfter, func(checked bool) {
		state.autoCompare = checked
	})
	autoCompareCheck.SetChecked(state.autoCompare)

	// Load/Save config buttons
	loadCfgBtn := ui.MakePillButton(t.ActionLoadConfig, ui.BorderDim, func() {
		cfg, err := loadPersistedConvertConfig()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				dialog.ShowInformation(t.DialogNoConfig, t.DialogNoConfigMsg, state.window)
			} else {
				dialog.ShowError(fmt.Errorf("failed to load config: %w", err), state.window)
			}
			return
		}
		state.convert = cfg
		state.showConvertView(state.source)
	})

	saveCfgBtn := ui.MakePillButton(t.ActionSaveConfig, ui.BorderDim, func() {
		if err := savePersistedConvertConfig(state.convert); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save config: %w", err), state.window)
			return
		}
		dialog.ShowInformation(t.DialogConfigSaved, fmt.Sprintf(t.DialogSavedToFmt, configpath.ModuleConfigPath("convert")), state.window)
	})

	// FFmpeg Command Preview
	var commandPreviewWidget *ui.FFmpegCommandWidget
	var commandPreviewBody *fyne.Container

	buildCommandPreview = func() fyne.CanvasObject {
		if src == nil {
			return widget.NewLabel(t.ConvertLoadVideoForCommand)
		}

		// Build command from current state
		cfg := state.convert
		config := map[string]interface{}{
			"quality":                cfg.Quality,
			"videoCodec":             cfg.VideoCodec,
			"encoderPreset":          cfg.EncoderPreset,
			"crf":                    cfg.CRF,
			"bitrateMode":            cfg.BitrateMode,
			"videoBitrate":           cfg.VideoBitrate,
			"targetFileSize":         cfg.TargetFileSize,
			"targetResolution":       cfg.TargetResolution,
			"frameRate":              cfg.FrameRate,
			"useMotionInterpolation": cfg.UseMotionInterpolation,
			"pixelFormat":            cfg.PixelFormat,
			"hardwareAccel":          cfg.HardwareAccel,
			"h264Profile":            cfg.H264Profile,
			"h264Level":              cfg.H264Level,
			"deinterlace":            cfg.Deinterlace,
			"deinterlaceMethod":      cfg.DeinterlaceMethod,
			"autoCrop":               cfg.AutoCrop,
			"cropWidth":              cfg.CropWidth,
			"cropHeight":             cfg.CropHeight,
			"cropX":                  cfg.CropX,
			"cropY":                  cfg.CropY,
			"flipHorizontal":         cfg.FlipHorizontal,
			"flipVertical":           cfg.FlipVertical,
			"rotation":               cfg.Rotation,
			"audioCodec":             cfg.AudioCodec,
			"audioBitrate":           cfg.AudioBitrate,
			"audioChannels":          cfg.AudioChannels,
			"normalizeAudio":         cfg.NormalizeAudio,
			"coverArtPath":           cfg.CoverArtPath,
			"aspectHandling":         cfg.AspectHandling,
			"outputAspect":           cfg.OutputAspect,
			"forceAspect":            cfg.ForceAspect,
			"sourceWidth":            src.Width,
			"sourceHeight":           src.Height,
			"sampleAspectRatio":      src.SampleAspectRatio,
			"displayAspectRatio":     src.DisplayAspectRatio,
			"sourceDuration":         src.Duration,
			"fieldOrder":             src.FieldOrder,
		}

		job := &queue.Job{
			Type:   queue.JobTypeConvert,
			Config: config,
		}
		cmdStr := buildFFmpegCommandFromJob(job)

		// Replace INPUT and OUTPUT placeholders with actual file paths for preview
		inputPath := src.Path
		outputPath := getOutputPathPreview()
		cmdStr = strings.ReplaceAll(cmdStr, "INPUT", inputPath)
		cmdStr = strings.ReplaceAll(cmdStr, "OUTPUT", outputPath)
		cmdStr = strings.ReplaceAll(cmdStr, "[COVER_ART]", state.convert.CoverArtPath)

		if commandPreviewWidget == nil {
			commandPreviewWidget = ui.NewFFmpegCommandWidget(cmdStr, state.window)
			commandPreviewBody = container.NewVBox(commandPreviewWidget)
		} else {
			commandPreviewWidget.SetCommand(cmdStr)
		}
		return commandPreviewBody
	}

	// Build initial preview if source is loaded
	_ = buildCommandPreview()

	leftControls := container.NewHBox(resetBtn, loadCfgBtn, saveCfgBtn, autoCompareCheck)
	rightControls := container.NewHBox(cancelBtn, cancelQueueBtn, viewLogBtn, addAllQueueBtn, addQueueBtn, convertBtn)
	actionBar := container.NewHBox(leftControls, layout.NewSpacer(), rightControls)

	// Start a UI refresh ticker to update widgets from state while conversion is active
	// This ensures progress updates even when navigating between modules
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()

		// Track the previous busy state to detect transitions
		wasBusy := state.convertBusy

		for {
			select {
			case <-ticker.C:
				isBusy := state.convertBusy

				// Update UI on the main thread
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					// Update status label from state
					if isBusy {
						statusLabel.SetText(state.convertStatus)
					} else if wasBusy {
						// Just finished - update one last time
						statusLabel.SetText(state.convertStatus)
					}

					// Update button states
					if isBusy {
						convertBtn.Disable()
						cancelBtn.Enable()
						if state.jobQueue != nil && state.jobQueue.CurrentRunning() != nil {
							cancelQueueBtn.Enable()
						} else {
							cancelQueueBtn.Disable()
						}
						activity.Show()
						if !activity.Running() {
							activity.Start()
						}
					} else {
						if src != nil {
							convertBtn.Enable()
						} else {
							convertBtn.Disable()
						}
						cancelBtn.Disable()
						if state.jobQueue != nil && state.jobQueue.CurrentRunning() != nil {
							cancelQueueBtn.Enable()
						} else {
							cancelQueueBtn.Disable()
						}
						activity.Stop()
						activity.Hide()
					}

					// Update stats bar to show live progress
					state.updateStatsBar()
				}, false)

				// If conversion finished, stop the ticker after one final update
				if wasBusy && !isBusy {
					return
				}
				wasBusy = isBusy

			case <-time.After(30 * time.Second):
				// Safety timeout - if no conversion after 30s, stop ticker
				if !state.convertBusy {
					return
				}
			}
		}
	}()

	// Update stats bar
	state.updateStatsBar()

	// Build footer sections
	footerSections := []fyne.CanvasObject{
		snippetRow,
		widget.NewSeparator(),
	}

	mainWithFooter := container.NewBorder(
		nil,
		container.NewVBox(footerSections...),
		nil, nil,
		mainContent,
	)
	return container.NewBorder(backBar, moduleFooter(convertColor, actionBar, state.statsBar), nil, nil, mainWithFooter)

}

func makeLabeledPanel(title, body string, min fyne.Size) *fyne.Container {
	rect := canvas.NewRectangle(utils.MustHex("#191F35"))
	rect.CornerRadius = 8
	rect.StrokeColor = gridColor
	rect.StrokeWidth = 1
	// Don't set rigid MinSize - let the container be flexible
	// rect.SetMinSize(min)

	header := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	desc := widget.NewLabel(body)
	desc.Wrapping = fyne.TextWrapWord

	box := container.NewVBox(header, desc, layout.NewSpacer())
	layers := ui.NoisyBackgroundObjects(rect)
	layers = append(layers, container.NewPadded(box))
	return container.NewMax(layers...)
}

func buildMetadataPanel(state *appState, src *videoSource, min fyne.Size, accentColor color.Color, onToggle func(open bool)) (fyne.CanvasObject, func()) {
	t := i18n.T()
	outer := canvas.NewRectangle(utils.MustHex("#191F35"))
	outer.CornerRadius = 8
	outer.StrokeColor = gridColor
	outer.StrokeWidth = 1
	// Don't set rigid MinSize - let the container be flexible for better splitter movement
	// outer.SetMinSize(min)

	if src == nil {
		nilHeader, _ := ui.BuildCollapsibleHeader(t.ConvertSectionMetadata, accentColor, onToggle)
		noSrcBody := container.NewBorder(nilHeader, nil, nil, nil,
			container.NewPadded(container.NewVBox(widget.NewLabel(t.ConvertInspectHint))))
		layers := ui.NoisyBackgroundObjects(outer)
		layers = append(layers, noSrcBody)
		return container.NewMax(layers...), func() {}
	}

	bitrate := "--"
	if src.Bitrate > 0 {
		bitrate = fmt.Sprintf("%d kbps", src.Bitrate/1000)
	}

	audioBitrate := "--"
	if src.AudioBitrate > 0 {
		prefix := ""
		if src.AudioBitrateEstimated {
			prefix = "~"
		}
		audioBitrate = fmt.Sprintf("%s%d kbps", prefix, src.AudioBitrate/1000)
	}

	// Format advanced metadata
	par := utils.FirstNonEmpty(src.SampleAspectRatio, "1:1 (Square)")
	if par == "1:1" || par == "1:1 (Square)" {
		par = "1:1 (Square)"
	} else {
		par = par + " (Non-square)"
	}

	colorSpace := utils.FirstNonEmpty(src.ColorSpace, "Unknown")
	if strings.EqualFold(colorSpace, "unknown") && strings.Contains(strings.ToLower(src.Format), "mp4") {
		colorSpace = "MP4 (ISO BMFF family)"
	}
	colorRange := utils.FirstNonEmpty(src.ColorRange, "Unknown")
	if colorRange == "tv" {
		colorRange = "Limited (TV)"
	} else if colorRange == "pc" || colorRange == "jpeg" {
		colorRange = "Full (PC)"
	}

	interlacing := "Progressive"
	if src.FieldOrder != "" && src.FieldOrder != "progressive" && src.FieldOrder != "unknown" {
		interlacing = "Interlaced (" + src.FieldOrder + ")"
	}

	gopSize := "--"
	if src.GOPSize > 0 {
		gopSize = fmt.Sprintf("%d frames", src.GOPSize)
	}

	chapters := "No"
	if src.HasChapters {
		chapters = "Yes"
	}

	metadata := "No"
	if src.HasMetadata {
		metadata = "Yes (title/copyright/etc)"
	}

	// Build metadata string for copying
	metadataText := fmt.Sprintf(`File: %s
Format: %s
Resolution: %dx%d
Aspect Ratio: %s
Pixel Aspect Ratio: %s
Duration: %s
Video Codec: %s
Video Bitrate: %s
Frame Rate: %.2f fps
Pixel Format: %s
Interlacing: %s
Color Space: %s
Color Range: %s
GOP Size: %s
Audio Codec: %s
Audio Bitrate: %s
Audio Rate: %d Hz
Channels: %s
Chapters: %s
Metadata: %s`,
		src.DisplayName,
		utils.FirstNonEmpty(src.Format, "Unknown"),
		src.Width, src.Height,
		src.AspectRatioString(),
		par,
		src.DurationString(),
		utils.FirstNonEmpty(src.VideoCodec, "Unknown"),
		bitrate,
		src.FrameRate,
		utils.FirstNonEmpty(src.PixelFormat, "Unknown"),
		interlacing,
		colorSpace,
		colorRange,
		gopSize,
		utils.FirstNonEmpty(src.AudioCodec, "Unknown"),
		audioBitrate,
		src.AudioRate,
		utils.ChannelLabel(src.Channels),
		chapters,
		metadata,
	)

	valueBg := utils.MustHex("#2B334A")
	valueBorder := utils.MustHex("#3A4360")

	makeValuePill := func(text string) fyne.CanvasObject {
		bg := canvas.NewRectangle(valueBg)
		bg.CornerRadius = 6
		bg.StrokeColor = valueBorder
		bg.StrokeWidth = 1
		label := widget.NewLabel(text)
		label.TextStyle = fyne.TextStyle{Monospace: true}
		label.Wrapping = fyne.TextTruncate
		pillContent := container.NewPadded(label)
		return container.NewMax(bg, pillContent)
	}
	makeValuePillWithChip := func(text string, chipColor color.Color) fyne.CanvasObject {
		bg := canvas.NewRectangle(valueBg)
		bg.CornerRadius = 6
		bg.StrokeColor = valueBorder
		bg.StrokeWidth = 1
		chip := canvas.NewRectangle(chipColor)
		chip.CornerRadius = 4
		chip.SetMinSize(fyne.NewSize(8, 0))
		label := widget.NewLabel(text)
		label.TextStyle = fyne.TextStyle{Monospace: true}
		label.Wrapping = fyne.TextTruncate
		pillContent := container.NewBorder(nil, nil, chip, nil, container.NewPadded(label))
		return container.NewMax(bg, pillContent)
	}

	makeRow := func(key string, value fyne.CanvasObject) fyne.CanvasObject {
		keyLabel := widget.NewLabel(key + ":")
		keyLabel.TextStyle = fyne.TextStyle{Bold: true}
		return container.NewBorder(nil, nil, keyLabel, nil, value)
	}
	makeCodecPill := func(codec string, getColor func(string) color.Color) fyne.CanvasObject {
		return makeValuePillWithChip(codec, getColor(codec))
	}

	// Filename gets its own full-width VBox layout to prevent vertical text
	fileValue := makeValuePill(src.DisplayName)
	fileRow := makeRow("File", fileValue)

	// Organize metadata into a compact two-column grid
	col1 := container.NewVBox(
		makeRow("Format", makeValuePill(utils.FirstNonEmpty(src.Format, "Unknown"))),
		makeRow("Resolution", makeValuePill(fmt.Sprintf("%dx%d", src.Width, src.Height))),
		makeRow("Aspect Ratio", makeValuePill(src.AspectRatioString())),
		makeRow("Duration", makeValuePill(src.DurationString())),
		makeRow("Frame Rate", makeValuePill(fmt.Sprintf("%.2f fps", src.FrameRate))),
		makeRow("Interlacing", makeValuePill(interlacing)),
		makeRow("Color Space", makeValuePill(colorSpace)),
		makeRow("Color Range", makeValuePill(colorRange)),
		makeRow("GOP Size", makeValuePill(gopSize)),
	)

	videoCodec := utils.FirstNonEmpty(src.VideoCodec, "Unknown")
	videoCodecValue := container.NewMax(makeCodecPill(videoCodec, ui.GetVideoCodecColor))
	audioCodec := utils.FirstNonEmpty(src.AudioCodec, "Unknown")
	audioCodecValue := container.NewMax(makeCodecPill(audioCodec, ui.GetAudioCodecColor))

	col2 := container.NewVBox(
		makeRow("Video Codec", videoCodecValue),
		makeRow("Video Bitrate", makeValuePill(bitrate)),
		makeRow("Pixel Format", makeValuePill(utils.FirstNonEmpty(src.PixelFormat, "Unknown"))),
		makeRow("Pixel AR", makeValuePill(par)),
		makeRow("Audio Codec", audioCodecValue),
		makeRow("Audio Bitrate", makeValuePill(audioBitrate)),
		makeRow("Audio Rate", makeValuePill(fmt.Sprintf("%d Hz", src.AudioRate))),
		makeRow("Channels", makeValuePill(utils.ChannelLabel(src.Channels))),
		makeRow("Chapters", makeValuePill(chapters)),
		makeRow("Metadata", makeValuePill(metadata)),
	)

	// Two-column grid with proper spacing
	twoColGrid := container.NewGridWithColumns(2, col1, col2)

	// Combine filename row with two-column grid
	info := container.NewVBox(fileRow, twoColGrid)

	// Copy metadata button - beside header text
	copyBtn := ui.MakePillButton("", ui.BorderDim, func() {
		state.window.Clipboard().SetContent(metadataText)
		dialog.ShowInformation(t.DialogCopied, "Metadata copied to clipboard", state.window)
	})

	// Clear button to remove the loaded video and reset UI - on the right
	clearBtn := ui.MakePillButton("Clear Video", ui.BorderDim, func() {
		if state != nil {
			state.clearVideo()
		}
	})

	metaHeader, _ := ui.BuildCollapsibleHeader(t.ConvertSectionMetadata, accentColor, onToggle, copyBtn, clearBtn)
	top := fyne.CanvasObject(metaHeader)

	// Cover art support removed - users can add cover art through metadata editor
	updateCoverDisplay := func() {
		// No-op: cover art display removed from this panel
	}

	// Interlacing Analysis Section
	analyzeBtn := ui.MakePillButton(t.ConvertAnalyzeInterlacing, ui.BorderDim, func() {
		if state.source == nil {
			return
		}
		state.interlaceAnalyzing = true
		state.interlaceResult = nil
		state.showConvertView(state.source) // Refresh to show "Analyzing..."

		go func() {
			detector := interlace.NewDetector(utils.GetFFmpegPath(), utils.GetFFprobePath())
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			result, err := detector.QuickAnalyze(ctx, state.source.Path)

			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				state.interlaceAnalyzing = false
				if err != nil {
					logging.Debug(logging.CatSystem, "interlacing analysis failed: %v", err)
					dialog.ShowError(fmt.Errorf("Analysis failed: %w", err), state.window)
				} else {
					state.interlaceResult = result
					logging.Debug(logging.CatSystem, "interlacing analysis complete: %s", result.Status)

					// Auto-update deinterlace setting based on recommendation
					if result.SuggestDeinterlace && state.convert.Deinterlace == "Off" {
						state.convert.Deinterlace = "Auto"
					}
				}
				state.showConvertView(state.source) // Refresh to show results
			}, false)
		}()
	})

	var interlaceSection fyne.CanvasObject
	if state.interlaceAnalyzing {
		statusLabel := widget.NewLabel(t.ConvertInterlaceAnalyzing)
		statusLabel.TextStyle = fyne.TextStyle{Italic: true}
		interlaceSection = container.NewVBox(
			widget.NewSeparator(),
			analyzeBtn,
			statusLabel,
		)
	} else if state.interlaceResult != nil {
		result := state.interlaceResult

		// Status color
		var statusColor color.Color
		switch result.Status {
		case "Progressive":
			statusColor = color.RGBA{R: 76, G: 232, B: 112, A: 255} // Green
		case "Interlaced":
			statusColor = color.RGBA{R: 255, G: 193, B: 7, A: 255} // Yellow
		default:
			statusColor = color.RGBA{R: 255, G: 136, B: 68, A: 255} // Orange
		}

		statusRect := canvas.NewRectangle(statusColor)
		statusRect.SetMinSize(fyne.NewSize(4, 0))
		statusRect.CornerRadius = 2

		statusLabel := widget.NewLabel(result.Status)
		statusLabel.TextStyle = fyne.TextStyle{Bold: true}

		percLabel := widget.NewLabel(fmt.Sprintf("%.1f%% interlaced frames", result.InterlacedPercent))
		fieldLabel := widget.NewLabel(fmt.Sprintf("Field Order: %s", result.FieldOrder))
		confLabel := widget.NewLabel(fmt.Sprintf("Confidence: %s", result.Confidence))
		recLabel := widget.NewLabel(result.Recommendation)
		recLabel.Wrapping = fyne.TextWrapWord

		// Frame counts (collapsed by default)
		detailsLabel := widget.NewLabel(fmt.Sprintf(
			"Progressive: %d | TFF: %d | BFF: %d | Undetermined: %d | Total: %d",
			result.Progressive, result.TFF, result.BFF, result.Undetermined, result.TotalFrames,
		))
		detailsLabel.TextStyle = fyne.TextStyle{Italic: true}
		detailsLabel.Wrapping = fyne.TextWrapWord

		resultCard := canvas.NewRectangle(utils.MustHex("#1E1E1E"))
		resultCard.CornerRadius = 4

		resultContent := container.NewBorder(
			nil, nil,
			statusRect,
			nil,
			container.NewVBox(
				statusLabel,
				percLabel,
				fieldLabel,
				confLabel,
				widget.NewSeparator(),
				recLabel,
				detailsLabel,
			),
		)

		// Preview button (only show if deinterlacing is recommended)
		var previewSection fyne.CanvasObject
		if result.SuggestDeinterlace {
		ui.MakePillButton("Generate Deinterlace Preview", ui.BorderDim, func() {
			if state.source == nil {
				return
			}

				go func() {
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						dialog.ShowInformation(t.DialogPreview, "Creating comparison preview...", state.window)
					}, false)

					detector := interlace.NewDetector(utils.GetFFmpegPath(), utils.GetFFprobePath())
					ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
					defer cancel()

					// Generate preview at 10 seconds into the video
					previewPath := filepath.Join(utils.TempDir(), fmt.Sprintf("deinterlace_preview_%d.png", time.Now().Unix()))
					err := detector.GenerateComparisonPreview(ctx, state.source.Path, 10.0, previewPath)

					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						if err != nil {
							logging.Debug(logging.CatSystem, "preview generation failed: %v", err)
							dialog.ShowError(fmt.Errorf("Preview generation failed: %w", err), state.window)
						} else {
							// Load and display the preview image
							img, err := fyne.LoadResourceFromPath(previewPath)
							if err != nil {
								dialog.ShowError(fmt.Errorf("Failed to load preview: %w", err), state.window)
								return
							}

							previewImg := canvas.NewImageFromResource(img)
							previewImg.FillMode = canvas.ImageFillContain
							// Adaptive size for small screens
							previewImg.SetMinSize(fyne.NewSize(640, 360))

							infoLabel := widget.NewLabel(t.ConvertInterlaceInfo)
							infoLabel.Alignment = fyne.TextAlignCenter
							infoLabel.TextStyle = fyne.TextStyle{Bold: true}

							content := container.NewBorder(
								infoLabel,
								nil, nil, nil,
								container.NewScroll(previewImg),
							)

							previewDialog := dialog.NewCustom("Deinterlace Preview", "Close", content, state.window)
							previewDialog.Resize(fyne.NewSize(900, 600))
							previewDialog.Show()

							// Clean up temp file after dialog closes
							go func() {
								time.Sleep(5 * time.Second)
								os.Remove(previewPath)
							}()
						}
					}, false)
				}()
			})

			var sectionItems []fyne.CanvasObject
			sectionItems = append(sectionItems,
				widget.NewSeparator(),
				analyzeBtn,
				container.NewPadded(container.NewMax(resultCard, resultContent)),
			)
			if previewSection != nil {
				sectionItems = append(sectionItems, previewSection)
			}

			interlaceSection = container.NewVBox(sectionItems...)
		}
	} else {
		interlaceSection = container.NewVBox(
			widget.NewSeparator(),
			analyzeBtn,
		)
	}

	// Layout: two-column metadata display with spacing
	contentArea := info

	// No inner VScroll here — the caller wraps metaPanel in ui.NewFastVScroll
	// (see showConvertView). A double-scroll causes the inner one to never
	// activate because NewMax gives it unlimited height equal to content.
	contentBody := container.NewPadded(container.NewVBox(contentArea, interlaceSection))
	body := container.NewBorder(top, nil, nil, nil, contentBody)
	layers := ui.NoisyBackgroundObjects(outer)
	layers = append(layers, body)
	return container.NewMax(layers...), updateCoverDisplay
}

// buildVideoPane creates the video preview pane for the Convert module.
// When native_media is enabled, it delegates to buildVideoPaneNative which
// uses the FFmpeg-based media engine for actual video playback.
func buildVideoPane(state *appState, min fyne.Size, src *videoSource, onCover func(string)) fyne.CanvasObject {
	if HasNativeMediaPlayer() {
		return buildVideoPaneNative(state, min, src, onCover)
	}

	t := i18n.T()
	outer := canvas.NewRectangle(utils.MustHex("#191F35"))
	outer.CornerRadius = 8
	outer.StrokeColor = gridColor
	outer.StrokeWidth = 1
	defaultAspect := 16.0 / 9.0
	if src != nil && src.Width > 0 && src.Height > 0 {
		defaultAspect = float64(src.Width) / float64(src.Height)
	}
	if defaultAspect < 0.6 {
		defaultAspect = 0.6
	} else if defaultAspect > 2.4 {
		defaultAspect = 2.4
	}
	targetWidth := float32(min.Width)
	targetHeight := float32(min.Height)
	if targetWidth <= 0 {
		targetWidth = 480
	}
	if targetHeight <= 0 {
		targetHeight = 360
	}
	aspect := float32(defaultAspect)
	stageWidth := targetWidth
	stageHeight := stageWidth / aspect
	if stageHeight < targetHeight {
		stageHeight = targetHeight
		stageWidth = stageHeight * aspect
	}
	// Don't set rigid MinSize - let the outer container be flexible
	// outer.SetMinSize(fyne.NewSize(targetWidth, targetHeight))

	if src == nil {
		smpteRaster := canvas.NewRaster(func(w, h int) image.Image {
			if w <= 0 || h <= 0 {
				return image.NewRGBA(image.Rect(0, 0, 1, 1))
			}
			// Match the native player: render bars at 4:3 with dark pillarboxing.
			const targetAspect = 4.0 / 3.0
			var smpteW, smpteH, offsetX, offsetY int
			if float64(w)/float64(h) > targetAspect {
				smpteH = h
				smpteW = int(float64(h) * targetAspect)
				offsetX = (w - smpteW) / 2
			} else {
				smpteW = w
				smpteH = int(float64(w) / targetAspect)
				offsetY = (h - smpteH) / 2
			}
			img := image.NewRGBA(image.Rect(0, 0, w, h))
			draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{R: 0x0F, G: 0x15, B: 0x29, A: 0xFF}), image.Point{}, draw.Src)
			bars := smpte.DrawBars(smpteW, smpteH, "DRAG TO LOAD VIDEO")
			draw.Draw(img, image.Rect(offsetX, offsetY, offsetX+smpteW, offsetY+smpteH), bars, image.Point{}, draw.Src)
			return img
		})
		smpteRaster.SetMinSize(fyne.NewSize(stageWidth, stageHeight))

		dropTarget := ui.NewDroppable(smpteRaster, func(items []fyne.URI) {
			state.handleDrop(fyne.NewPos(0, 0), items)
		})
		return container.NewMax(outer, container.NewPadded(dropTarget))
	}

	state.stopPreview()

	sourceFrame := ""
	if len(src.PreviewFrames) > 0 {
		sourceFrame = src.PreviewFrames[0]
		state.currentFrame = sourceFrame
	}

	var img *canvas.Image
	if sourceFrame != "" {
		img = canvas.NewImageFromFile(sourceFrame)
	} else {
		img = canvas.NewImageFromResource(nil)
	}
	img.FillMode = canvas.ImageFillContain
	// Don't set rigid MinSize on image - it will scale to container
	// img.SetMinSize(fyne.NewSize(targetWidth, targetHeight))
	stage := canvas.NewRectangle(utils.MustHex("#0F1529"))
	stage.CornerRadius = 6
	// No minimum size — image uses ImageFillContain so it scales freely.
	// A hard min here would override the VSplit offset and push video beyond 50%.
	// Overlay the image directly so it fills the stage while preserving aspect.
	videoStage := container.NewMax(stage, img)

	// Drop indicator - pulsing border when video is loaded
	dropIndicator := canvas.NewRectangle(color.NRGBA{R: 76, G: 175, B: 80, A: 0})
	dropIndicator.CornerRadius = 8
	dropIndicator.StrokeWidth = 3
	dropIndicator.StrokeColor = utils.MustHex("#4CE870")

	// Create animation for pulsing drop indicator
	dropAnimation := fyne.NewAnimation(800*time.Millisecond, func(progress float32) {
		// Pulse opacity from 255 to 0 and back
		alpha := uint8(255 * (1 - progress))
		dropIndicator.StrokeColor = color.NRGBA{R: 76, G: 175, B: 80, A: alpha}
		dropIndicator.Refresh()
		if progress >= 1.0 {
			dropIndicator.StrokeColor = color.NRGBA{R: 76, G: 175, B: 80, A: 0}
			dropIndicator.StrokeWidth = 0
			dropIndicator.Refresh()
		}
	})
	dropAnimation.AutoReverse = true
	dropAnimation.RepeatCount = 3

	// Start the drop animation when video is loaded
	dropAnimation.Start()

	videoStageWithIndicator := container.NewMax(dropIndicator, videoStage)

	coverBtn := utils.MakeIconButton("", "Set current frame as cover art", func() {
		path, err := state.captureCoverFromCurrent()
		if err != nil {
			dialog.ShowError(err, state.window)
			return
		}
		if onCover != nil {
			onCover(path)
		}
	})

	saveFrameBtn := utils.MakeIconButton("", "Save current frame as PNG", func() {
		framePath, err := state.captureCoverFromCurrent()
		if err != nil {
			dialog.ShowError(err, state.window)
			return
		}
		dlg := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
			if err != nil {
				dialog.ShowError(err, state.window)
				return
			}
			if w == nil {
				return
			}
			defer w.Close()

			data, readErr := os.ReadFile(framePath)
			if readErr != nil {
				dialog.ShowError(readErr, state.window)
				return
			}
			if _, writeErr := w.Write(data); writeErr != nil {
				dialog.ShowError(writeErr, state.window)
				return
			}
		}, state.window)
		dlg.SetFilter(storage.NewExtensionFileFilter([]string{".png"}))
		if src != nil {
			name := strings.TrimSuffix(src.DisplayName, filepath.Ext(src.DisplayName)) + "-frame.png"
			dlg.SetFileName(name)
		}
		dlg.Show()
	})

	importBtn := utils.MakeIconButton("", "Import cover art file", func() {
		dlg := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, state.window)
				return
			}
			if r == nil {
				return
			}
			path := r.URI().Path()
			r.Close()
			if dest, err := state.importCoverImage(path); err == nil {
				if onCover != nil {
					onCover(dest)
				}
			} else {
				dialog.ShowError(err, state.window)
			}
		}, state.window)
		dlg.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg"}))
		dlg.Show()
	})

	usePlayer := state.active != "thumbnail"

	currentTime := widget.NewLabel("0:00")
	totalTime := widget.NewLabel(src.DurationString())
	totalTime.Alignment = fyne.TextAlignTrailing
	var updatingProgress bool
	slider := ui.MakeSlider(0, math.Max(1, src.Duration))
	slider.Step = 0.5
	updateProgress := func(val float64) {
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			updatingProgress = true
			currentTime.SetText(formatClock(val))
			slider.SetValue(val)
			updatingProgress = false
		}, false)
	}

	var controls fyne.CanvasObject
	if usePlayer {
		// Frame counter label
		frameLabel := widget.NewLabel("Frame: 0")
		frameLabel.TextStyle = fyne.TextStyle{Monospace: true}

		var volIcon *widget.Button
		var updatingVolume bool
		ensureSession := func() bool {
			if !HasNativeMediaPlayer() {
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					dialog.ShowInformation(t.DialogPlayback, "Native media player not available.\n\nPlease report this issue.", state.window)
				}, false)
				return false
			}
			return true
		}

		// Immediate seeking for responsive playback
		slider.OnChanged = func(val float64) {
			if updatingProgress {
				return
			}
			updateProgress(val)
			if HasNativeMediaPlayer() {
				state.seekNative(val)
				return
			}
			if ensureSession() {
				state.seekNative(val)
			}
		}
		updateVolIcon := func() {
			if volIcon == nil {
				return
			}
			if state.playerMuted || state.playerVolume <= 0 {
				volIcon.Icon = ui.GetIcon("volume_mute")
			} else {
				volIcon.Icon = ui.GetIcon("volume_up")
			}
			volIcon.Refresh()
		}
		volIcon = widget.NewButtonWithIcon("", ui.GetIcon("volume_up"), func() {
			if !ensureSession() {
				return
			}
			if state.playerMuted {
				target := state.lastVolume
				if target <= 0 {
					target = 50
				}
				state.playerVolume = target
				state.playerMuted = false
				state.setVolumeNative(target)
			} else {
				state.lastVolume = state.playerVolume
				state.playerVolume = 0
				state.playerMuted = true
				state.setVolumeNative(0)
			}
			updateVolIcon()
		})
		volSlider := ui.MakeSlider(0, 100)
		volSlider.Step = 1
		volSlider.Value = state.playerVolume
		volSlider.Resize(fyne.NewSize(150, 40))
		volSlider.OnChanged = func(val float64) {
			if updatingVolume {
				return
			}
			state.playerVolume = val
			if val > 0 {
				state.lastVolume = val
				state.playerMuted = false
			} else {
				state.playerMuted = true
			}
			if ensureSession() {
				state.setVolumeNative(val)
			}
			updateVolIcon()
		}
		updateVolIcon()
		volSlider.Refresh()
		var playBtn *widget.Button
		playBtn = widget.NewButtonWithIcon("", ui.GetIcon("play_arrow"), func() {
			if state.playerPaused {
				state.playNative()
				state.playerPaused = false
				playBtn.Icon = ui.GetIcon("pause")
			} else {
				state.pauseNative()
				state.playerPaused = true
				playBtn.Icon = ui.GetIcon("play_arrow")
			}
			playBtn.Refresh()
		})
		playBtn.Importance = widget.LowImportance

		// Frame stepping buttons
		prevFrameBtn := widget.NewButtonWithIcon("", ui.GetIcon("skip_previous"), func() {
			state.stepFrameNative(-1)
		})
		prevFrameBtn.Importance = widget.LowImportance
		nextFrameBtn := widget.NewButtonWithIcon("", ui.GetIcon("skip_next"), func() {
			state.stepFrameNative(1)
		})
		nextFrameBtn.Importance = widget.LowImportance

		fullBtn := utils.MakeIconButton("", "Toggle fullscreen", func() {
			if state.window == nil {
				return
			}
			state.window.SetFullScreen(!state.window.FullScreen())
		})
		// ±10s skip buttons
		replay10Btn := widget.NewButtonWithIcon("", ui.GetIcon("replay_10"), func() {
			state.seekNative(math.Max(0, slider.Value-10))
		})
		replay10Btn.Importance = widget.LowImportance
		forward10Btn := widget.NewButtonWithIcon("", ui.GetIcon("forward_10"), func() {
			state.seekNative(math.Min(src.Duration, slider.Value+10))
		})
		forward10Btn.Importance = widget.LowImportance

		// Volume control row
		volBox := container.NewHBox(volIcon, container.NewMax(volSlider))

		// Seek row: [currentTime] [=========slider=========] [totalTime]
		seekRow := container.NewBorder(nil, nil, currentTime, totalTime, container.NewMax(slider))

		// Main controls: left = transport, right = volume + fullscreen
		leftBtns := container.NewHBox(replay10Btn, prevFrameBtn, playBtn, nextFrameBtn, forward10Btn)
		rightBtns := container.NewHBox(volBox, fullBtn)
		mainCtrlRow := container.NewBorder(nil, nil, leftBtns, rightBtns, nil)

		// Primary bar (dark pill)
		primaryBg := canvas.NewRectangle(color.NRGBA{R: 12, G: 17, B: 31, A: 230})
		primaryBar := container.NewMax(primaryBg, container.NewPadded(container.NewVBox(seekRow, mainCtrlRow)))

		// Advanced bar (frame tools — discrete, below primary bar)
		advancedBg := canvas.NewRectangle(utils.MustHex("#0C111F"))
		advancedBg.StrokeColor = gridColor
		advancedBg.StrokeWidth = 1
		frameTools := container.NewBorder(nil, nil,
			container.NewHBox(widget.NewSeparator(), frameLabel),
			container.NewHBox(coverBtn, saveFrameBtn, importBtn),
			nil,
		)
		advancedBar := container.NewMax(advancedBg, container.NewPadded(frameTools))

		controls = container.NewVBox(primaryBar, advancedBar)
	} else {
		slider := ui.MakeSlider(0, math.Max(1, float64(len(src.PreviewFrames)-1)))
		slider.Step = 1
		slider.OnChanged = func(val float64) {
			if state.anim != nil && state.anim.playing {
				state.anim.Pause()
			}
			idx := int(val)
			if idx >= 0 && idx < len(src.PreviewFrames) {
				state.showFrameManual(src.PreviewFrames[idx], img)
				if slider.Max > 0 {
					approx := (val / slider.Max) * src.Duration
					currentTime.SetText(formatClock(approx))
				}
			}
		}
		playBtn := widget.NewButtonWithIcon("", ui.GetIcon("play_pause"), func() {
			if len(src.PreviewFrames) == 0 {
				return
			}
			if state.anim == nil {
				state.startPreview(src.PreviewFrames, img, slider)
				return
			}
			if state.anim.playing {
				state.anim.Pause()
			} else {
				state.anim.Play()
			}
		})
		playBtn.Importance = widget.LowImportance
		seekRow := container.NewBorder(nil, nil, currentTime, totalTime, container.NewMax(slider))
		ctrlRow := container.NewBorder(nil, nil,
			container.NewHBox(playBtn),
			container.NewHBox(coverBtn, saveFrameBtn, importBtn),
			nil,
		)
		previewBg := canvas.NewRectangle(color.NRGBA{R: 12, G: 17, B: 31, A: 230})
		controls = container.NewMax(previewBg, container.NewPadded(container.NewVBox(seekRow, ctrlRow)))
		if len(src.PreviewFrames) > 1 {
			state.startPreview(src.PreviewFrames, img, slider)
		} else {
			playBtn.Disable()
		}
	}

	videoWithOverlay := videoStageWithIndicator
	if usePlayer {
		state.setPlayerSurface(videoStageWithIndicator, int(stageWidth), int(stageHeight))
	}

	stack := container.NewBorder(
		nil,
		controls,
		nil, nil,
		container.NewPadded(videoWithOverlay),
	)
	videoDropTarget := ui.NewDroppable(stack, func(items []fyne.URI) {
		state.handleDrop(fyne.NewPos(0, 0), items)
	})
	return container.NewMax(outer, container.NewPadded(videoDropTarget))
}

type previewAnimator struct {
	frames  []string
	img     *canvas.Image
	slider  *ui.Slider
	stop    chan struct{}
	playing bool
	state   *appState
	index   int
}

func (a *previewAnimator) Start() {
	if len(a.frames) == 0 {
		return
	}
	ticker := time.NewTicker(150 * time.Millisecond)
	go func() {
		defer ticker.Stop()
		idx := 0
		for {
			select {
			case <-a.stop:
				return
			case <-ticker.C:
				if !a.playing {
					continue
				}
				idx = (idx + 1) % len(a.frames)
				a.index = idx
				frame := a.frames[idx]
				a.showFrame(frame)
				if a.slider != nil {
					cur := float64(idx)
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						a.slider.SetValue(cur)
					}, false)
				}
			}
		}
	}()
}

func (a *previewAnimator) Pause() { a.playing = false }
func (a *previewAnimator) Play()  { a.playing = true }

func (a *previewAnimator) showFrame(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	frame, err := png.Decode(f)
	if err != nil {
		return
	}
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		a.img.Image = frame
		a.img.Refresh()
		if a.state != nil {
			a.state.currentFrame = path
		}
	}, false)
}

func (a *previewAnimator) Stop() {
	select {
	case <-a.stop:
	default:
		close(a.stop)
	}
}

func (s *appState) showFrameManual(path string, img *canvas.Image) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	frame, err := png.Decode(f)
	if err != nil {
		return
	}
	img.Image = frame
	img.Refresh()
	s.currentFrame = path
}

func (s *appState) captureCoverFromCurrent() (string, error) {
	// Use the current preview frame
	if s.currentFrame == "" {
		return "", fmt.Errorf("no frame available")
	}
	data, err := os.ReadFile(s.currentFrame)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(utils.TempDir(), fmt.Sprintf("videotools-cover-%d.png", time.Now().UnixNano()))
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return "", err
	}
	return dest, nil
}

func (s *appState) importCoverImage(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(utils.TempDir(), fmt.Sprintf("videotools-cover-import-%d%s", time.Now().UnixNano(), filepath.Ext(path)))
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return "", err
	}
	return dest, nil
}

func (s *appState) handleDrop(pos fyne.Position, items []fyne.URI) {
	defer logging.RecoverPanic()
	t := i18n.T()
	logging.Info(logging.CatUI, "handleDrop: active=%s pos=%v itemCount=%d", s.active, pos, len(items))
	if len(items) == 0 {
		return
	}

	// If on main menu, detect which module tile was dropped on
	if s.active == "" {
		moduleID := s.detectModuleTileAtPosition(pos)
		if moduleID != "" {
			logging.Debug(logging.CatUI, "drop on main menu tile=%s", moduleID)
			s.handleModuleDrop(moduleID, items)
			return
		}
		logging.Debug(logging.CatUI, "drop on main menu but not over any module tile")
		return
	}

	// If in convert module, handle all files
	if s.active == "convert" {
		logging.Debug(logging.CatUI, "handleDrop: in convert module, processing items")
		// Collect all video files from the dropped items
		var videoPaths []string
		for _, uri := range items {
			if uri.Scheme() != "file" {
				logging.Debug(logging.CatUI, "skipping non-file URI: %s", uri.String())
				continue
			}
			path := uri.Path()
			logging.Debug(logging.CatModule, "drop received path=%s", path)

			// Check if it's a directory
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				logging.Debug(logging.CatModule, "processing directory: %s", path)
				videos := s.findVideoFiles(path)
				videoPaths = append(videoPaths, videos...)
			} else if s.isVideoFile(path) {
				logging.Debug(logging.CatUI, "isVideoFile=true for: %s", path)
				videoPaths = append(videoPaths, path)
			} else {
				logging.Debug(logging.CatUI, "isVideoFile=false for: %s (ext=%s)", path, filepath.Ext(path))
			}
		}

		if len(videoPaths) == 0 {
			logging.Debug(logging.CatUI, "no valid video files in dropped items")
			if msg := dropMismatchMessage(items, "convert"); msg != "" {
				ui.ShowToast(s.window, msg, ui.ToastWarning)
			}
			return
		}

		logging.Debug(logging.CatUI, "video(s) dropped in convert module; loading %d into memory", len(videoPaths))
		go s.loadMultipleVideos(videoPaths)
		return
	}

	// If in subtitles module, handle video/subtitle files
	if s.active == "subtitles" {
		s.handleSubtitlesModuleDrop(items)
		return
	}

	// If in audio module, handle dropped video files
	if s.active == "audio" {
		var videoPaths []string
		for _, uri := range items {
			if uri.Scheme() != "file" {
				continue
			}
			path := uri.Path()
			logging.Debug(logging.CatModule, "drop received path=%s", path)
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				videos := s.findVideoFiles(path)
				videoPaths = append(videoPaths, videos...)
			} else if s.isVideoFile(path) {
				videoPaths = append(videoPaths, path)
			}
		}

		if len(videoPaths) == 0 {
			logging.Debug(logging.CatUI, "no valid video files in dropped items")
			if msg := dropMismatchMessage(items, "audio"); msg != "" {
				ui.ShowToast(s.window, msg, ui.ToastWarning)
			}
			return
		}

		if s.audioBatchMode {
			fyne.Do(func() {
				for _, path := range videoPaths {
					s.addAudioBatchFile(path)
				}
			})
			return
		}

		go s.loadAudioFile(videoPaths[0])
		return
	}

	// Author module has its own dedicated drop handler on the clip list widget for
	// regular video files. Only handle VIDEO_TS folder drops here (for re-authoring
	// existing DVDs), not regular video clips.
	if s.active == "author" {
		for _, uri := range items {
			if uri.Scheme() != "file" {
				continue
			}
			path := uri.Path()
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				var videoTSPath string
				if strings.EqualFold(filepath.Base(path), "VIDEO_TS") {
					videoTSPath = path
				} else {
					videoTSChild := filepath.Join(path, "VIDEO_TS")
					if info, err := os.Stat(videoTSChild); err == nil && info.IsDir() {
						videoTSPath = videoTSChild
					}
				}
				if videoTSPath != "" {
					go func() {
						s.authorVideoTSPath = videoTSPath
						s.authorClips = nil
						s.authorFile = nil
						s.authorOutputType = "iso"
						s.recentFiles.Add(videoTSPath, filepath.Base(videoTSPath), "author")
						s.loadVideoTSChapters(videoTSPath)
						fyne.CurrentApp().Driver().DoFromGoroutine(s.showAuthorView, false)
					}()
					return
				}
			}
		}
		// Regular video files are handled by author_module.go's drop handler
		return
	}

	// If in rip module, accept DVD/ISO/VIDEO_TS paths
	if s.active == "rip" {
		path := firstLocalDropPath(items)
		if path == "" {
			logging.Debug(logging.CatUI, "no valid paths in dropped items")
			return
		}
		s.ripSourcePath = path
		s.ripOutputPath = defaultRipOutputPath(path, s.ripFormat)
		s.showRipView()
		return
	}

	// If in compare module (normal or fullscreen), handle up to 2 video files
	if s.active == "compare" || s.active == "compare-fullscreen" {
		// Collect all video files from the dropped items
		var videoPaths []string
		for _, uri := range items {
			if uri.Scheme() != "file" {
				continue
			}
			path := uri.Path()
			logging.Debug(logging.CatModule, "drop received path=%s", path)

			// Only accept video files (not directories)
			if s.isVideoFile(path) {
				videoPaths = append(videoPaths, path)
			}
		}

		if len(videoPaths) == 0 {
			logging.Debug(logging.CatUI, "no valid video files in dropped items")
			msg := dropMismatchMessage(items, "compare")
			if msg == "" {
				msg = "No video files found in the dropped items."
			}
			ui.ShowToast(s.window, msg, ui.ToastWarning)
			return
		}

		// Show message if more than 2 videos dropped
		if len(videoPaths) > 2 {
			dialog.ShowInformation(t.DialogCompare,
				fmt.Sprintf("You dropped %d videos. Only the first two will be loaded for comparison.", len(videoPaths)),
				s.window)
		}

		// Load videos sequentially to avoid race conditions
		go func() {
			if len(videoPaths) == 1 {
				// Single video dropped - use smart slot assignment
				src, err := probeVideo(videoPaths[0])
				if err != nil {
					logging.Debug(logging.CatModule, "failed to load video: %v", err)
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						dialog.ShowError(fmt.Errorf("failed to load video: %w", err), s.window)
					}, false)
					return
				}

				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					// Smart slot assignment: fill the empty slot, or slot 1 if both empty
					if s.compareFile1 == nil {
						s.compareFile1 = src
						logging.Debug(logging.CatModule, "loaded video into empty slot 1")
					} else if s.compareFile2 == nil {
						s.compareFile2 = src
						logging.Debug(logging.CatModule, "loaded video into empty slot 2")
					} else {
						// Both slots full, overwrite slot 1
						s.compareFile1 = src
						logging.Debug(logging.CatModule, "both slots full, overwriting slot 1")
					}
					s.showCompareView()
				}, false)
			} else {
				// Multiple videos dropped - load into both slots
				src1, err := probeVideo(videoPaths[0])
				if err != nil {
					logging.Debug(logging.CatModule, "failed to load first video: %v", err)
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						dialog.ShowError(fmt.Errorf("failed to load video 1: %w", err), s.window)
					}, false)
					return
				}

				var src2 *videoSource
				if len(videoPaths) >= 2 {
					src2, err = probeVideo(videoPaths[1])
					if err != nil {
						logging.Debug(logging.CatModule, "failed to load second video: %v", err)
						// Continue with just first video
						fyne.CurrentApp().Driver().DoFromGoroutine(func() {
							dialog.ShowError(fmt.Errorf("failed to load video 2: %w", err), s.window)
						}, false)
					}
				}

				// Update both slots and refresh view once
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					s.compareFile1 = src1
					s.compareFile2 = src2
					s.showCompareView()
					logging.Debug(logging.CatModule, "loaded %d video(s) into both slots", len(videoPaths))
				}, false)
			}
		}()

		return
	}

	// If in inspect module, handle single video file
	if s.active == "inspect" {
		// Collect video files from dropped items
		var videoPaths []string
		for _, uri := range items {
			if uri.Scheme() != "file" {
				continue
			}
			path := uri.Path()
			if s.isVideoFile(path) {
				videoPaths = append(videoPaths, path)
			}
		}

		if len(videoPaths) == 0 {
			logging.Debug(logging.CatUI, "no valid video files in dropped items")
			msg := dropMismatchMessage(items, "inspect")
			if msg == "" {
				msg = "No video files found in the dropped items."
			}
			ui.ShowToast(s.window, msg, ui.ToastWarning)
			return
		}

		// Load first video
		videoPath := videoPaths[0]
		logging.Info(logging.CatInspect, "inspect: probing dropped file: %s", videoPath)
		go func() {
			src, err := probeVideo(videoPath)
			if err != nil {
				logging.Error(logging.CatInspect, "inspect probe failed: %v", err)
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					dialog.ShowError(fmt.Errorf("failed to load video: %w", err), s.window)
				}, false)
				return
			}
			logging.Info(logging.CatInspect, "inspect: probe complete, loading player")

			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				s.inspectFile = src
				s.inspectInterlaceResult = nil
				s.inspectInterlaceAnalyzing = true
				s.showInspectView()
				logging.Info(logging.CatInspect, "inspect: view refreshed with file metadata")
			}, false)

			// Load native player now that probe is done and view is being shown.
			if err := GetPrimaryPlayer().Load(videoPath); err != nil {
				logging.Error(logging.CatPlayer, "inspect player load failed: %v", err)
			}

			// Auto-run interlacing detection in background
			go func() {
				// Capture preview frames before running interlace analysis
				if len(src.PreviewFrames) == 0 {
					if frames, ferr := capturePreviewFrames(videoPath, src.Duration); ferr == nil && len(frames) > 0 {
						src.PreviewFrames = frames
					}
				}

				detector := interlace.NewDetector(utils.GetFFmpegPath(), utils.GetFFprobePath())
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()

				result, err := detector.QuickAnalyze(ctx, videoPath)

				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					s.inspectInterlaceAnalyzing = false
					if err != nil {
						logging.Debug(logging.CatSystem, "auto interlacing analysis failed: %v", err)
						s.inspectInterlaceResult = nil
					} else {
						s.inspectInterlaceResult = result
						logging.Debug(logging.CatSystem, "auto interlacing analysis complete: %s", result.Status)
					}
					s.showInspectView() // Refresh to show results
				}, false)
			}()
		}()

		return
	}

	// If in thumbnail module, handle single video file
	if s.active == "thumbnail" {
		// Collect video files from dropped items
		var videoPaths []string
		for _, uri := range items {
			if uri.Scheme() != "file" {
				continue
			}
			path := uri.Path()
			if s.isVideoFile(path) {
				videoPaths = append(videoPaths, path)
			}
		}

		if len(videoPaths) == 0 {
			logging.Debug(logging.CatUI, "no valid video files in dropped items")
			msg := dropMismatchMessage(items, "thumbnail")
			if msg == "" {
				msg = "No video files found in the dropped items."
			}
			ui.ShowToast(s.window, msg, ui.ToastWarning)
			return
		}

		go s.loadMultipleThumbnailVideos(videoPaths)

		return
	}

	// If in filters module, handle single video file
	if s.active == "filters" {
		var videoPaths []string
		for _, uri := range items {
			if uri.Scheme() != "file" {
				continue
			}
			path := uri.Path()
			if s.isVideoFile(path) {
				videoPaths = append(videoPaths, path)
			}
		}

		if len(videoPaths) == 0 {
			logging.Debug(logging.CatUI, "no valid video files in dropped items")
			msg := dropMismatchMessage(items, "filters")
			if msg == "" {
				msg = "No video files found in the dropped items."
			}
			ui.ShowToast(s.window, msg, ui.ToastWarning)
			return
		}

		go func() {
			src, err := probeVideo(videoPaths[0])
			if err != nil {
				logging.Debug(logging.CatModule, "failed to load video for filters: %v", err)
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					dialog.ShowError(fmt.Errorf("failed to load video: %w", err), s.window)
				}, false)
				return
			}

			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				s.filtersFile = src
				s.showFiltersView()
				logging.Debug(logging.CatModule, "loaded video into filters module")
			}, false)
		}()

		return
	}

	// If in upscale module, handle single video file
	if s.active == "upscale" {
		var videoPaths []string
		for _, uri := range items {
			if uri.Scheme() != "file" {
				continue
			}
			path := uri.Path()
			if s.isVideoFile(path) {
				videoPaths = append(videoPaths, path)
			}
		}

		if len(videoPaths) == 0 {
			logging.Debug(logging.CatUI, "no valid video files in dropped items")
			msg := dropMismatchMessage(items, "upscale")
			if msg == "" {
				msg = "No video files found in the dropped items."
			}
			ui.ShowToast(s.window, msg, ui.ToastWarning)
			return
		}

		go func() {
			src, err := probeVideo(videoPaths[0])
			if err != nil {
				logging.Debug(logging.CatModule, "failed to load video for upscale: %v", err)
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					dialog.ShowError(fmt.Errorf("failed to load video: %w", err), s.window)
				}, false)
				return
			}

			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				s.upscaleFile = src
				s.showUpscaleView()
				logging.Debug(logging.CatModule, "loaded video into upscale module")
			}, false)
		}()

		return
	}

	// If in merge module, handle multiple video files
	if s.active == "merge" {
		// Collect all video files from the dropped items
		var videoPaths []string
		for _, uri := range items {
			if uri.Scheme() != "file" {
				continue
			}
			path := uri.Path()
			logging.Debug(logging.CatModule, "drop received path=%s", path)

			// Check if it's a directory
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				logging.Debug(logging.CatModule, "processing directory: %s", path)
				videos := s.findVideoFiles(path)
				videoPaths = append(videoPaths, videos...)
			} else if s.isVideoFile(path) {
				videoPaths = append(videoPaths, path)
			}
		}

		if len(videoPaths) == 0 {
			logging.Debug(logging.CatUI, "no valid video files in dropped items")
			if msg := dropMismatchMessage(items, "merge"); msg != "" {
				ui.ShowToast(s.window, msg, ui.ToastWarning)
			}
			return
		}

		// Add all videos to merge clips sequentially
		go func() {
			for _, path := range videoPaths {
				src, err := probeVideo(path)
				if err != nil {
					logging.Debug(logging.CatModule, "failed to probe %s: %v", path, err)
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						dialog.ShowError(fmt.Errorf("failed to probe %s: %w", filepath.Base(path), err), s.window)
					}, false)
					continue
				}

				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					s.mergeClips = append(s.mergeClips, mergeClip{
						Path:     path,
						Chapter:  strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
						Duration: src.Duration,
					})

					// Set default output dir and filename if not set and we have at least 2 clips
					if len(s.mergeClips) >= 2 && strings.TrimSpace(s.mergeOutputDir) == "" {
						s.mergeOutputDir = filepath.Dir(s.mergeClips[0].Path)
					}
					if len(s.mergeClips) >= 2 && strings.TrimSpace(s.mergeOutputFilename) == "" {
						s.mergeOutputFilename = "merged.mkv"
					}

					// Refresh the merge view to show the new clips
					s.showMergeView()
				}, false)
			}

			logging.Debug(logging.CatModule, "added %d clips to merge list", len(videoPaths))
		}()

		return
	}

	// If in player module, handle video files and DVD discs.
	if s.active == "player" {
		// Check for a DVD disc first (ISO or directory with VIDEO_TS).
		for _, uri := range items {
			if uri.Scheme() != "file" {
				continue
			}
			if isDVDDisc(uri.Path()) {
				s.showDVDDiscView(uri.Path())
				return
			}
		}

		// Collect regular video files.
		var videoPaths []string
		for _, uri := range items {
			if uri.Scheme() != "file" {
				continue
			}
			path := uri.Path()
			if s.isVideoFile(path) {
				videoPaths = append(videoPaths, path)
			}
		}

		if len(videoPaths) == 0 {
			logging.Debug(logging.CatUI, "no valid video files in dropped items")
			dialog.ShowInformation(t.ModulePlayer, "No video files found in dropped items.", s.window)
			return
		}

		// Load first video.
		go func() {
			src, err := probeVideo(videoPaths[0])
			if err != nil {
				logging.Debug(logging.CatModule, "failed to load video for player: %v", err)
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					dialog.ShowError(fmt.Errorf("failed to load video: %w", err), s.window)
				}, false)
				return
			}

			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				s.playerFile = src
				s.showPlayerView()
				logging.Debug(logging.CatModule, "loaded video into player module")

				p := GetPrimaryPlayer()
				if p != nil {
					_ = p.Load(src.Path)
				}
			}, false)
		}()

		return
	}

	// Other modules don't handle file drops yet
	logging.Debug(logging.CatUI, "drop ignored; module %s cannot handle files", s.active)
}

// detectModuleTileAtPosition calculates which module tile is at the given position
// based on the main menu grid layout (3 columns)
func (s *appState) detectModuleTileAtPosition(pos fyne.Position) string {
	logging.Debug(logging.CatUI, "detecting module tile at position x=%.1f y=%.1f", pos.X, pos.Y)

	// Main menu layout:
	// - Window padding: ~6px
	// - Header (title + queue): ~70-80px height
	// - Padding: 14px
	// - Grid starts at approximately y=100
	// - Grid is 3 columns x 3 rows
	// - Each tile: 220x110 with padding

	// Approximate grid start position
	const gridStartY = 100.0
	const gridStartX = 6.0 // Window padding

	// Window width is 920, minus padding = 908
	// 3 columns = ~302px per column
	const columnWidth = 302.0

	// Each row is tile height (110) + vertical padding (~12) = ~122
	const rowHeight = 122.0

	// Calculate relative position within grid
	if pos.Y < gridStartY {
		logging.Debug(logging.CatUI, "position above grid (y=%.1f < %.1f)", pos.Y, gridStartY)
		return ""
	}

	relX := pos.X - gridStartX
	relY := pos.Y - gridStartY

	// Calculate column (0, 1, or 2)
	col := int(relX / columnWidth)
	if col < 0 || col > 2 {
		logging.Debug(logging.CatUI, "position outside grid columns (col=%d)", col)
		return ""
	}

	// Calculate row (0, 1, or 2)
	row := int(relY / rowHeight)
	if row < 0 || row > 2 {
		logging.Debug(logging.CatUI, "position outside grid rows (row=%d)", row)
		return ""
	}

	// Calculate module index in grid (row * 3 + col)
	moduleIndex := row*3 + col
	if moduleIndex >= len(modulesList) {
		logging.Debug(logging.CatUI, "module index %d out of range (total %d)", moduleIndex, len(modulesList))
		return ""
	}

	moduleID := modulesList[moduleIndex].ID
	logging.Debug(logging.CatUI, "detected module: row=%d col=%d index=%d id=%s", row, col, moduleIndex, moduleID)

	// Only return module ID if it's enabled (currently only "convert")
	if moduleID != "convert" {
		logging.Debug(logging.CatUI, "module %s is not enabled, ignoring drop", moduleID)
		return ""
	}

	return moduleID
}

func (s *appState) loadVideo(path string) {
	defer func() {
		if r := recover(); r != nil {
			logging.Crash(logging.CatSystem, "panic in loadVideo(%s): %v", path, r)
			logging.LogAllGoroutines()
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				s.showErrorWithCopy("Failed to Load Video", fmt.Errorf("panic while loading video: %v", r))
			}, false)
		}
	}()

	if HasNativeMediaPlayer() {
		s.closeNativePlayer()
	}

	s.stopProgressLoop()
	logging.Info(logging.CatModule, "loadVideo: probing %s", path)
	src, err := probeVideo(path)
	if err != nil {
		logging.Error(logging.CatConvert, "ffprobe failed: path=%s err=%v", path, err)
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			s.showErrorWithCopy("Failed to Analyze Video", fmt.Errorf("failed to analyze %s: %w", filepath.Base(path), err))
		}, false)
		return
	}
	logging.Info(logging.CatModule, "loadVideo: probe succeeded for %s", path)
	if frames, err := capturePreviewFrames(src.Path, src.Duration); err == nil {
		src.PreviewFrames = frames
		if len(frames) > 0 {
			s.currentFrame = frames[0]
		}
	} else {
		logging.Debug(logging.CatFFMPEG, "preview generation failed: %v", err)
		s.currentFrame = ""
	}
	s.applyInverseDefaults(src)
	s.convert.OutputBase = s.resolveOutputBase(src, false)
	// Use embedded cover art if present, otherwise clear
	if src.EmbeddedCoverArt != "" {
		s.convert.CoverArtPath = src.EmbeddedCoverArt
		logging.Debug(logging.CatFFMPEG, "using embedded cover art from video: %s", src.EmbeddedCoverArt)
	} else {
		s.convert.CoverArtPath = ""
	}
	s.convert.AspectHandling = "Auto"
	s.playerReady = false
	s.playerPos = 0
	s.playerPaused = true

	// Maintain/extend loaded video list for navigation
	found := -1
	for i, v := range s.loadedVideos {
		if v.Path == src.Path {
			found = i
			break
		}
	}

	if found >= 0 {
		cleanupVideoSourceTempFiles(s.loadedVideos[found])
		s.loadedVideos[found] = src
		s.currentIndex = found
	} else if len(s.loadedVideos) > 0 {
		s.loadedVideos = append(s.loadedVideos, src)
		s.currentIndex = len(s.loadedVideos) - 1
	} else {
		s.loadedVideos = []*videoSource{src}
		s.currentIndex = 0
	}

	// Load video in native media player if available
	if HasNativeMediaPlayer() {
		s.loadVideoNative(path)
	}

	logging.Debug(logging.CatModule, "video loaded %+v", src)
	s.recentFiles.Add(path, filepath.Base(path), "convert")

	// Capture active module before entering the UI goroutine so we can route back
	// to the correct view. Without this, dropping a file on the Player module's
	// video stage called loadVideo → showConvertView even when the user was in
	// the Player module.
	activeAtLoad := s.active

	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		s.source = src
		switch activeAtLoad {
		case "player":
			s.playerFile = src
			s.showPlayerView()
		default:
			s.showConvertView(src)
			// Refresh must run AFTER setContent's async fyne.Do(update) completes —
			// queuing a nested dispatch ensures the widget is already in the active
			// canvas when Refresh fires, so the stored first frame is actually painted.
			if HasNativeMediaPlayer() {
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					GetPrimaryPlayer().Widget().Refresh()
				}, false)
			}
		}
	}, false)
}

// loadMultipleVideos loads multiple videos into memory without auto-queuing
func (s *appState) loadMultipleVideos(paths []string) {
	logging.Info(logging.CatModule, "loadMultipleVideos: loading %d videos into memory", len(paths))

	var validVideos []*videoSource
	var failedFiles []string

	for _, path := range paths {
		logging.Debug(logging.CatModule, "loadMultipleVideos: probing %s", path)
		src, err := probeVideo(path)
		if err != nil {
			logging.Debug(logging.CatFFMPEG, "loadMultipleVideos: ffprobe failed for %s: %v", path, err)
			failedFiles = append(failedFiles, filepath.Base(path))
			continue
		}
		logging.Debug(logging.CatModule, "loadMultipleVideos: probe succeeded for %s", path)
		validVideos = append(validVideos, src)
	}

	if len(validVideos) == 0 {
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			msg := fmt.Sprintf("Failed to analyze %d file(s):\n%s", len(failedFiles), strings.Join(failedFiles, ", "))
			s.showErrorWithCopy("Load Failed", fmt.Errorf("%s", msg))
		}, false)
		return
	}

	// Clean up temp files from the previously loaded set before replacing.
	for _, v := range s.loadedVideos {
		cleanupVideoSourceTempFiles(v)
	}
	cleanupCoverArtIfTemp(s.convert.CoverArtPath)

	// Load all videos into loadedVideos array
	s.loadedVideos = validVideos
	s.currentIndex = 0

	// Load the first video to display
	firstVideo := validVideos[0]
	if frames, err := capturePreviewFrames(firstVideo.Path, firstVideo.Duration); err == nil {
		firstVideo.PreviewFrames = frames
		if len(frames) > 0 {
			s.currentFrame = frames[0]
		}
	}

	s.applyInverseDefaults(firstVideo)
	s.convert.OutputBase = s.resolveOutputBase(firstVideo, false)
	if firstVideo.EmbeddedCoverArt != "" {
		s.convert.CoverArtPath = firstVideo.EmbeddedCoverArt
	} else {
		s.convert.CoverArtPath = ""
	}
	s.convert.AspectHandling = "Auto"
	s.playerReady = false
	s.playerPos = 0
	s.playerPaused = true

	// Load into the native media player (same as loadVideo does).
	if HasNativeMediaPlayer() {
		s.loadVideoNative(firstVideo.Path)
	}

	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		s.source = firstVideo
		s.showConvertView(firstVideo)
		// Queue Refresh in a nested dispatch so it runs after setContent's
		// async fyne.Do(update) has committed the new window content. Calling
		// Refresh synchronously here fires before the canvas swap completes,
		// which leaves the player widget unredrawn.
		if HasNativeMediaPlayer() {
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				GetPrimaryPlayer().Widget().Refresh()
			}, false)
		}

		// Log any failed files for debugging
		if len(failedFiles) > 0 {
			logging.Debug(logging.CatModule, "%d file(s) failed to analyze: %s", len(failedFiles), strings.Join(failedFiles, ", "))
		}
	}, false)

	// Pre-generate preview frames for remaining videos in the background
	for _, v := range validVideos[1:] {
		video := v
		go func() {
			if frames, err := capturePreviewFrames(video.Path, video.Duration); err == nil && len(frames) > 0 {
				video.PreviewFrames = frames
				logging.Debug(logging.CatModule, "pre-generated preview frames for %s", filepath.Base(video.Path))
			}
		}()
	}

	logging.Debug(logging.CatModule, "loaded %d videos into memory", len(validVideos))
}

func (s *appState) clearVideo() {
	logging.Debug(logging.CatModule, "clearing loaded video")
	for _, v := range s.loadedVideos {
		cleanupVideoSourceTempFiles(v)
	}
	cleanupCoverArtIfTemp(s.convert.CoverArtPath)
	s.releasePlaybackSession()
	s.stopPlayer()
	s.source = nil
	s.loadedVideos = nil
	s.currentIndex = 0
	s.currentFrame = ""
	s.convertBusy = false
	s.convertStatus = ""
	s.convert.OutputBase = "converted"
	s.convert.CoverArtPath = ""
	s.convert.AspectHandling = "Auto"
	s.convert.OutputAspect = "Source"
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		s.showConvertView(nil)
	}, false)
}

func (s *appState) releasePlaybackSession() {
	s.stopPreview()
}

// loadVideos loads multiple videos for navigation
func (s *appState) loadVideos(paths []string) {
	if len(paths) == 0 {
		return
	}

	go func() {
		total := len(paths)
		type result struct {
			idx int
			src *videoSource
		}

		// Progress UI
		status := widget.NewLabel(fmt.Sprintf("Loading 0/%d", total))
		progress := widget.NewProgressBar()
		progress.Max = float64(total)
		var dlg dialog.Dialog
		fyne.Do(func() {
			dlg = dialog.NewCustomWithoutButtons("Loading Videos", container.NewVBox(status, progress), s.window)
			dlg.Show()
		})
		defer fyne.Do(func() {
			if dlg != nil {
				dlg.Hide()
			}
		})

		results := make([]*videoSource, total)
		var mu sync.Mutex
		done := 0

		workerCount := runtime.NumCPU()
		if workerCount > 4 {
			workerCount = 4
		}
		if workerCount < 1 {
			workerCount = 1
		}

		jobs := make(chan int, total)
		var wg sync.WaitGroup
		for w := 0; w < workerCount; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for idx := range jobs {
					path := paths[idx]
					src, err := probeVideo(path)
					if err == nil {
						if frames, ferr := capturePreviewFrames(src.Path, src.Duration); ferr == nil {
							src.PreviewFrames = frames
						}
						mu.Lock()
						results[idx] = src
						done++
						curDone := done
						mu.Unlock()
						fyne.Do(func() {
							status.SetText(fmt.Sprintf("Loading %d/%d", curDone, total))
							progress.SetValue(float64(curDone))
						})
					} else {
						logging.Debug(logging.CatFFMPEG, "ffprobe failed for %s: %v", path, err)
						mu.Lock()
						done++
						curDone := done
						mu.Unlock()
						fyne.Do(func() {
							status.SetText(fmt.Sprintf("Loading %d/%d", curDone, total))
							progress.SetValue(float64(curDone))
						})
					}
				}
			}()
		}
		for i := range paths {
			jobs <- i
		}
		close(jobs)
		wg.Wait()

		// Collect valid videos in original order
		var loaded []*videoSource
		for _, src := range results {
			if src != nil {
				loaded = append(loaded, src)
			}
		}

		if len(loaded) == 0 {
			fyne.Do(func() {
				s.showErrorWithCopy("Failed to Load Videos", fmt.Errorf("no valid videos to load"))
			})
			return
		}

		s.loadedVideos = loaded
		s.currentIndex = 0
		fyne.Do(func() {
			s.switchToVideo(0)
		})
	}()
}

// switchToVideo switches to a specific video by index
func (s *appState) switchToVideo(index int) {
	if index < 0 || index >= len(s.loadedVideos) {
		return
	}

	s.currentIndex = index
	src := s.loadedVideos[index]
	s.source = src

	s.applyInverseDefaults(src)
	s.convert.OutputBase = s.resolveOutputBase(src, false)

	if src.EmbeddedCoverArt != "" {
		s.convert.CoverArtPath = src.EmbeddedCoverArt
	} else {
		s.convert.CoverArtPath = ""
	}

	s.convert.AspectHandling = "Auto"
	s.playerReady = false
	s.playerPos = 0
	s.playerPaused = true

	if len(src.PreviewFrames) > 0 {
		s.currentFrame = src.PreviewFrames[0]
	} else {
		s.currentFrame = ""
	}

	// Show immediately with whatever frame we have (may be blank while frames generate)
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		s.showConvertView(src)
	}, false)

	// If frames aren't ready yet, generate them in background and refresh the view
	if len(src.PreviewFrames) == 0 {
		go func() {
			if frames, err := capturePreviewFrames(src.Path, src.Duration); err == nil && len(frames) > 0 {
				src.PreviewFrames = frames
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					// Only refresh if the user is still on this video
					if s.source == src {
						s.currentFrame = frames[0]
						s.showConvertView(src)
					}
				}, false)
			}
		}()
	}
}

// nextVideo switches to the next loaded video
func (s *appState) nextVideo() {
	if len(s.loadedVideos) == 0 {
		return
	}
	nextIndex := (s.currentIndex + 1) % len(s.loadedVideos)
	s.switchToVideo(nextIndex)
}

// prevVideo switches to the previous loaded video
func (s *appState) prevVideo() {
	if len(s.loadedVideos) == 0 {
		return
	}
	prevIndex := s.currentIndex - 1
	if prevIndex < 0 {
		prevIndex = len(s.loadedVideos) - 1
	}
	s.switchToVideo(prevIndex)
}

func crfForQuality(q string) string {
	switch q {
	case "Balanced (CRF 20)":
		return "20"
	case "Draft (CRF 28)":
		return "28"
	case "High (CRF 18)":
		return "18"
	case "Near-Lossless (CRF 16)":
		return "16"
	case "Lossless":
		return "0"
	default:
		return "23"
	}
}

// detectBestH264Encoder probes ffmpeg for available H.264 encoders and returns the best one
// Priority: h264_nvenc (NVIDIA) > h264_qsv (Intel) > h264_vaapi (VA-API) > libopenh264 > fallback
func detectBestH264Encoder() string {
	// List of encoders to try in priority order
	encoders := []string{"h264_nvenc", "h264_qsv", "h264_vaapi", "libopenh264"}

	for _, encoder := range encoders {
		cmd := utils.CreateCommandRaw(utils.GetFFmpegPath(), "-hide_banner", "-encoders")
		output, err := cmd.CombinedOutput()
		if err == nil {
			// Check if encoder is in the output
			if strings.Contains(string(output), " "+encoder+" ") || strings.Contains(string(output), " "+encoder+"\n") {
				logging.Debug(logging.CatFFMPEG, "detected hardware encoder: %s", encoder)
				return encoder
			}
		}
	}

	// Fallback: check if libx264 is available
	cmd := utils.CreateCommandRaw(utils.GetFFmpegPath(), "-hide_banner", "-encoders")
	output, err := cmd.CombinedOutput()
	if err == nil && (strings.Contains(string(output), " libx264 ") || strings.Contains(string(output), " libx264\n")) {
		logging.Debug(logging.CatFFMPEG, "using software encoder: libx264")
		return "libx264"
	}

	logging.Debug(logging.CatFFMPEG, "no H.264 encoder found, using libx264 as fallback")
	return "libx264"
}

// detectBestH265Encoder probes ffmpeg for available H.265 encoders and returns the best one
func detectBestH265Encoder() string {
	encoders := []string{"hevc_nvenc", "hevc_qsv", "hevc_vaapi"}

	for _, encoder := range encoders {
		cmd := utils.CreateCommandRaw(utils.GetFFmpegPath(), "-hide_banner", "-encoders")
		output, err := cmd.CombinedOutput()
		if err == nil {
			if strings.Contains(string(output), " "+encoder+" ") || strings.Contains(string(output), " "+encoder+"\n") {
				logging.Debug(logging.CatFFMPEG, "detected hardware encoder: %s", encoder)
				return encoder
			}
		}
	}

	cmd := utils.CreateCommandRaw(utils.GetFFmpegPath(), "-hide_banner", "-encoders")
	output, err := cmd.CombinedOutput()
	if err == nil && (strings.Contains(string(output), " libx265 ") || strings.Contains(string(output), " libx265\n")) {
		logging.Debug(logging.CatFFMPEG, "using software encoder: libx265")
		return "libx265"
	}

	logging.Debug(logging.CatFFMPEG, "no H.265 encoder found, using libx265 as fallback")
	return "libx265"
}

// determineVideoCodec maps user-friendly codec names to FFmpeg codec names
func determineVideoCodec(cfg convertConfig) string {
	accel := effectiveHardwareAccel(cfg)
	if accel != "" && accel != "none" && !hwAccelAvailable(accel) {
		accel = "none"
	}
	switch cfg.VideoCodec {
	case "H.264":
		if accel == "nvenc" {
			return "h264_nvenc"
		} else if accel == "amf" {
			return "h264_amf"
		} else if accel == "qsv" {
			return "h264_qsv"
		} else if accel == "videotoolbox" {
			return "h264_videotoolbox"
		}
		// When set to "none" or empty, use software encoder
		return "libx264"
	case "H.265":
		if accel == "nvenc" {
			return "hevc_nvenc"
		} else if accel == "amf" {
			return "hevc_amf"
		} else if accel == "qsv" {
			return "hevc_qsv"
		} else if accel == "videotoolbox" {
			return "hevc_videotoolbox"
		}
		// When set to "none" or empty, use software encoder
		return "libx265"
	case "VP9":
		return "libvpx-vp9"
	case "AV1":
		// resolveAV1Encoder handles per-backend capability checks (e.g. av1_nvenc
		// requires Ada Lovelace; a generic NVENC probe passing on Ampere is not enough).
		enc, _ := resolveAV1Encoder(accel)
		return enc
	case "MPEG-2":
		return "mpeg2video"
	case "mpeg2video":
		return "mpeg2video"
	case "Copy":
		return "copy"
	default:
		return "libx264"
	}
}

// friendlyCodecFromPreset maps a preset codec string (e.g., "libx265") to the UI-friendly codec name.
func friendlyCodecFromPreset(preset string) string {
	preset = strings.ToLower(preset)
	switch {
	case strings.Contains(preset, "265") || strings.Contains(preset, "hevc"):
		return "H.265"
	case strings.Contains(preset, "264"):
		return "H.264"
	case strings.Contains(preset, "vp9"):
		return "VP9"
	case strings.Contains(preset, "av1"):
		return "AV1"
	case strings.Contains(preset, "mpeg2"):
		return "MPEG-2"
	default:
		return ""
	}
}

// determineAudioCodec maps user-friendly codec names to FFmpeg codec names
func determineAudioCodec(cfg convertConfig) string {
	switch cfg.AudioCodec {
	case "AAC":
		return "aac"
	case "Opus":
		return "libopus"
	case "MP3":
		return "libmp3lame"
	case "AC-3":
		return "ac3"
	case "ac3":
		return "ac3"
	case "FLAC":
		return "flac"
	case "Copy":
		return "copy"
	default:
		return "aac"
	}
}

func (s *appState) cancelConvert(cancelBtn, btn *ui.PillButton, spinner *widget.ProgressBarInfinite, status *widget.Label) {
	if s.convertCancel == nil {
		return
	}
	s.convertStatus = "Cancelling"
	// Widget states will be updated by the UI refresh ticker
	s.convertCancel()
}

func (s *appState) startConvert(status *widget.Label, btn, cancelBtn *widget.Button, spinner *widget.ProgressBarInfinite) {
	t := i18n.T()
	setStatus := func(msg string) {
		s.convertStatus = msg
		logging.Debug(logging.CatFFMPEG, "convert status: %s", msg)
		// Note: Don't update widgets here - they may be stale if user navigated away
		// The UI will refresh from state.convertStatus via a ticker
	}
	if s.source == nil {
		dialog.ShowInformation(t.ModuleConvert, "Load a video first.", s.window)
		return
	}
	if s.convertBusy {
		return
	}
	src := s.source
	cfg := s.convert
	ensureCompatibleCodec(&cfg)
	sourceBitrate := src.Bitrate
	isDVD := cfg.SelectedFormat.Ext == ".mpg"
	outDir := filepath.Dir(src.Path)
	outName := cfg.OutputFile()
	if outName == "" {
		outName = "converted" + cfg.SelectedFormat.Ext
	}
	outPath := filepath.Join(outDir, outName)
	if outPath == src.Path {
		outPath = filepath.Join(outDir, "converted-"+outName)
	}

	// Guard against overwriting an existing file without confirmation
	if _, err := os.Stat(outPath); err == nil {
		confirm := make(chan bool, 1)
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			msg := fmt.Sprintf("Output file already exists:\n%s\n\nOverwrite it?", outPath)
			dialog.ShowConfirm("Overwrite File?", msg, func(ok bool) {
				confirm <- ok
			}, s.window)
		}, false)
		if ok := <-confirm; !ok {
			setStatus("Cancelled (existing output)")
			return
		}
	}

	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "error",
	}

	// Hardware acceleration for decoding - MUST come BEFORE input file
	if accel := effectiveHardwareAccel(cfg); accel != "none" && accel != "" && hwAccelAvailable(accel) {
		switch accel {
		case "nvenc":
			// NVENC encoders handle GPU directly; no hwaccel flag needed
		case "amf":
			// AMF encoders handle GPU directly
		case "vaapi":
			args = append(args, "-hwaccel", "vaapi")
		case "qsv":
			args = append(args, "-hwaccel", "qsv")
		case "videotoolbox":
			args = append(args, "-hwaccel", "videotoolbox")
		}
		logging.Debug(logging.CatFFMPEG, "hardware acceleration: %s", accel)
	}

	// DVD presets: enforce compliant codecs and audio settings
	// Note: We do NOT force resolution - user can choose Source or specific resolution
	if isDVD {
		if strings.Contains(cfg.SelectedFormat.Label, "PAL") {
			cfg.TargetResolution = "PAL (720x540)"
			cfg.FrameRate = "25"
		} else {
			cfg.TargetResolution = "NTSC (720x480)"
			cfg.FrameRate = "29.97"
		}
		cfg.VideoBitrate = "8000k"
		cfg.BitrateMode = "CBR"
		if strings.Contains(cfg.SelectedFormat.Label, "PAL") {
			// Only set frame rate if not already specified
			if cfg.FrameRate == "" || cfg.FrameRate == "Source" {
				cfg.FrameRate = "25"
			}
		} else {
			// Only set frame rate if not already specified
			if cfg.FrameRate == "" || cfg.FrameRate == "Source" {
				cfg.FrameRate = "29.97"
			}
		}
		cfg.VideoCodec = "MPEG-2"
		cfg.AudioCodec = "AC-3"
		if cfg.AudioBitrate == "" {
			cfg.AudioBitrate = "192k"
		}
		cfg.PixelFormat = "yuv420p"
	}

	args = append(args, "-i", src.Path)

	// Add cover art if available
	hasCoverArt := cfg.CoverArtPath != ""
	if isDVD {
		// DVD targets do not support attached cover art
		hasCoverArt = false
	}
	if hasCoverArt {
		args = append(args, "-i", cfg.CoverArtPath)
	}

	// Hardware acceleration for decoding (best-effort)
	if accel := effectiveHardwareAccel(cfg); accel != "none" && accel != "" && hwAccelAvailable(accel) {
		switch accel {
		case "nvenc":
			// NVENC encoders handle GPU directly; no hwaccel flag needed
		case "amf":
			// AMF encoders handle GPU directly
		case "vaapi":
			args = append(args, "-hwaccel", "vaapi")
		case "qsv":
			args = append(args, "-hwaccel", "qsv")
		case "videotoolbox":
			args = append(args, "-hwaccel", "videotoolbox")
		}
		logging.Debug(logging.CatFFMPEG, "hardware acceleration: %s", accel)
	}

	// Video filters.
	var vf []string

	// Deinterlacing
	shouldDeinterlace := false
	if cfg.Deinterlace == "Force" {
		shouldDeinterlace = true
		logging.Debug(logging.CatFFMPEG, "deinterlacing: forced on")
	} else if cfg.Deinterlace == "Auto" || cfg.Deinterlace == "" {
		// Auto-detect based on field order
		if src.FieldOrder != "" && src.FieldOrder != "progressive" && src.FieldOrder != "unknown" {
			shouldDeinterlace = true
			logging.Debug(logging.CatFFMPEG, "deinterlacing: auto-detected (field_order=%s)", src.FieldOrder)
		}
	} else if cfg.Deinterlace == "Off" {
		shouldDeinterlace = false
		logging.Debug(logging.CatFFMPEG, "deinterlacing: disabled")
	}

	// Legacy InverseTelecine support
	if cfg.InverseTelecine {
		shouldDeinterlace = true
		logging.Debug(logging.CatFFMPEG, "deinterlacing: enabled via legacy InverseTelecine")
	}

	if shouldDeinterlace {
		// Choose deinterlacing method
		deintMethod := cfg.DeinterlaceMethod
		if deintMethod == "" {
			deintMethod = "bwdif" // Default to bwdif (higher quality)
		}

		if deintMethod == "bwdif" {
			// Bob Weaver Deinterlacing - higher quality, slower
			vf = append(vf, "bwdif=mode=send_frame:parity=auto")
			logging.Debug(logging.CatFFMPEG, "using bwdif deinterlacing (high quality)")
		} else {
			// Yet Another Deinterlacing Filter - faster, good quality
			vf = append(vf, "yadif=0:-1:0")
			logging.Debug(logging.CatFFMPEG, "using yadif deinterlacing (fast)")
		}
	}

	// Auto-crop black bars (apply before scaling for best results)
	if cfg.AutoCrop {
		// Apply crop using detected or manual values
		if cfg.CropWidth != "" && cfg.CropHeight != "" {
			cropW := strings.TrimSpace(cfg.CropWidth)
			cropH := strings.TrimSpace(cfg.CropHeight)
			cropX := strings.TrimSpace(cfg.CropX)
			cropY := strings.TrimSpace(cfg.CropY)

			// Default to center crop if X/Y not specified
			if cropX == "" {
				cropX = "(in_w-out_w)/2"
			}
			if cropY == "" {
				cropY = "(in_h-out_h)/2"
			}

			cropFilter := fmt.Sprintf("crop=%s:%s:%s:%s", cropW, cropH, cropX, cropY)
			vf = append(vf, cropFilter)
			logging.Debug(logging.CatFFMPEG, "applying crop: %s", cropFilter)
		} else {
			logging.Debug(logging.CatFFMPEG, "auto-crop enabled but no crop values specified, skipping")
		}
	}

	targetW, targetH := targetResolutionDims(cfg.TargetResolution)
	srcAspect := displayAspectRatioForSource(src)
	targetAspect := resolveTargetAspect(cfg.OutputAspect, src)
	aspectConversionNeeded := cfg.OutputAspect != "" &&
		!strings.EqualFold(cfg.OutputAspect, "source") &&
		targetAspect > 0 &&
		srcAspect > 0 &&
		!utils.RatiosApproxEqual(targetAspect, srcAspect, 0.01)
	useAspectScaling := aspectConversionNeeded && targetW > 0 && targetH > 0

	// Scaling/Resolution
	if cfg.TargetResolution != "" && cfg.TargetResolution != "Source" && !useAspectScaling {
		var scaleFilter string
		makeEven := func(v int) int {
			if v%2 != 0 {
				return v + 1
			}
			return v
		}
		switch cfg.TargetResolution {
		case "720p":
			scaleFilter = "scale=-2:720"
		case "1080p":
			scaleFilter = "scale=-2:1080"
		case "1440p":
			scaleFilter = "scale=-2:1440"
		case "4K":
			scaleFilter = "scale=-2:2160"
		case "8K":
			scaleFilter = "scale=-2:4320"
		case "NTSC (720x480)":
			scaleFilter = "scale=720:480"
		case "PAL (720x540)":
			scaleFilter = "scale=720:540"
		case "PAL (720x576)":
			scaleFilter = "scale=720:576"
		case "2X (relative)":
			if src != nil {
				w := makeEven(src.Width * 2)
				h := makeEven(src.Height * 2)
				scaleFilter = fmt.Sprintf("scale=%d:%d", w, h)
			}
		case "4X (relative)":
			if src != nil {
				w := makeEven(src.Width * 4)
				h := makeEven(src.Height * 4)
				scaleFilter = fmt.Sprintf("scale=%d:%d", w, h)
			}
		}
		if scaleFilter != "" {
			vf = append(vf, scaleFilter)
		}
	}

	// Aspect ratio conversion (only if user explicitly changed from Source)
	if aspectConversionNeeded {
		vf = append(vf, aspectFiltersWithTarget(targetAspect, cfg.AspectHandling, srcAspect, targetW, targetH)...)
		logging.Debug(logging.CatFFMPEG, "converting aspect ratio from %.2f to %.2f using %s mode", srcAspect, targetAspect, cfg.AspectHandling)
	}
	if cfg.ForceAspect && targetAspect > 0 {
		if len(vf) == 0 {
			vf = append(vf, fmt.Sprintf("setdar=%.6f", targetAspect), "setsar=1")
		} else {
			vf = appendAspectMetadata(vf, targetAspect)
		}
	}

	// Flip horizontal
	if cfg.FlipHorizontal {
		vf = append(vf, "hflip")
	}

	// Flip vertical
	if cfg.FlipVertical {
		vf = append(vf, "vflip")
	}

	// Rotation
	if cfg.Rotation != "" && cfg.Rotation != "0" {
		switch cfg.Rotation {
		case "90":
			vf = append(vf, "transpose=1") // 90 degrees clockwise
		case "180":
			vf = append(vf, "transpose=1,transpose=1") // 180 degrees
		case "270":
			vf = append(vf, "transpose=2") // 90 degrees counter-clockwise (= 270 clockwise)
		}
	}

	// Frame rate
	if cfg.FrameRate != "" && cfg.FrameRate != "Source" {
		if cfg.UseMotionInterpolation {
			// Use motion interpolation for smooth frame rate changes
			vf = append(vf, fmt.Sprintf("minterpolate=fps=%s:mi_mode=mci:mc_mode=aobmc:me_mode=bidir:vsbmc=1", cfg.FrameRate))
		} else {
			// Simple frame rate change (duplicates/drops frames)
			vf = append(vf, "fps="+cfg.FrameRate)
		}
	}

	if len(vf) > 0 {
		args = append(args, "-vf", strings.Join(vf, ","))
	}

	// Video codec
	videoCodec := determineVideoCodec(cfg)
	if cfg.VideoCodec == "Copy" {
		args = append(args, "-c:v", "copy")
	} else {
		args = append(args, "-c:v", videoCodec)

		// Bitrate mode and quality
		if cfg.BitrateMode == "CRF" || cfg.BitrateMode == "" {
			// Use CRF mode
			crf := cfg.CRF
			if crf == "" {
				crf = crfForQuality(cfg.Quality)
			}
			if videoCodec == "libx264" || videoCodec == "libx265" || videoCodec == "libvpx-vp9" {
				args = append(args, "-crf", crf)
			}
		} else if cfg.BitrateMode == "CBR" {
			// Constant bitrate
			vb := cfg.VideoBitrate
			if vb == "" {
				vb = defaultBitrate(cfg.VideoCodec, src.Width, sourceBitrate)
			}
			// Set bufsize to 2x target for better encoder handling
			bitrateVal, err := utils.ParseInt(strings.TrimSuffix(vb, "k"))
			bufsize := vb
			if err == nil {
				bufsize = fmt.Sprintf("%dk", bitrateVal*2)
			}
			args = append(args, "-b:v", vb, "-minrate", vb, "-maxrate", vb, "-bufsize", bufsize)
		} else if cfg.BitrateMode == "VBR" {
			// Variable bitrate - use 2-pass for accuracy
			vb := cfg.VideoBitrate
			if vb == "" {
				vb = defaultBitrate(cfg.VideoCodec, src.Width, sourceBitrate)
			}
			args = append(args, "-b:v", vb)
			// VBR uses maxrate at 2x target for quality peaks, bufsize at 2x maxrate to enforce cap
			bitrateVal, err := utils.ParseInt(strings.TrimSuffix(vb, "k"))
			if err == nil {
				maxBitrate := fmt.Sprintf("%dk", bitrateVal*2)
				bufsize := fmt.Sprintf("%dk", bitrateVal*4)
				args = append(args, "-maxrate", maxBitrate, "-bufsize", bufsize)
			}
			// Force 2-pass for VBR accuracy
			if !cfg.TwoPass {
				cfg.TwoPass = true
			}
		} else if cfg.BitrateMode == "Target Size" {
			// Calculate bitrate from target file size
			if cfg.TargetFileSize != "" && src.Duration > 0 {
				targetBytes, err := convert.ParseFileSize(cfg.TargetFileSize)
				if err == nil {
					// Parse audio bitrate (default to 192k if not set)
					audioBitrate := 192000
					if cfg.AudioBitrate != "" {
						if rate, err := utils.ParseInt(strings.TrimSuffix(cfg.AudioBitrate, "k")); err == nil {
							audioBitrate = rate * 1000
						}
					}

					// Calculate required video bitrate
					videoBitrate := convert.CalculateBitrateForTargetSize(targetBytes, src.Duration, audioBitrate)
					videoBitrateStr := fmt.Sprintf("%dk", videoBitrate/1000)

					logging.Debug(logging.CatFFMPEG, "target size mode: %s -> video bitrate %s (audio %s)", cfg.TargetFileSize, videoBitrateStr, cfg.AudioBitrate)
					args = append(args, "-b:v", videoBitrateStr)
				}
			}
		}

		// Encoder preset (speed vs quality tradeoff)
		if cfg.EncoderPreset != "" && (videoCodec == "libx264" || videoCodec == "libx265") {
			args = append(args, "-preset", cfg.EncoderPreset)
		}

		// Pixel format
		if cfg.PixelFormat != "" {
			args = append(args, "-pix_fmt", cfg.PixelFormat)
		}

		// H.264 profile and level for compatibility (iPhone, etc.)
		if cfg.VideoCodec == "H.264" && (strings.Contains(videoCodec, "264") || strings.Contains(videoCodec, "h264")) {
			if cfg.H264Profile != "" && cfg.H264Profile != "Auto" {
				// Use :v:0 if cover art is present to avoid applying to PNG stream
				if hasCoverArt {
					args = append(args, "-profile:v:0", cfg.H264Profile)
				} else {
					args = append(args, "-profile:v", cfg.H264Profile)
				}
				logging.Debug(logging.CatFFMPEG, "H.264 profile: %s", cfg.H264Profile)
			}
			if cfg.H264Level != "" && cfg.H264Level != "Auto" {
				if hasCoverArt {
					args = append(args, "-level:v:0", cfg.H264Level)
				} else {
					args = append(args, "-level:v", cfg.H264Level)
				}
				logging.Debug(logging.CatFFMPEG, "H.264 level: %s", cfg.H264Level)
			}
		}
	}

	// Audio codec and settings
	if cfg.AudioCodec == "Copy" {
		args = append(args, "-c:a", "copy")
	} else {
		audioCodec := determineAudioCodec(cfg)
		args = append(args, "-c:a", audioCodec)

		// Audio bitrate
		if cfg.AudioBitrate != "" && audioCodec != "flac" {
			args = append(args, "-b:a", cfg.AudioBitrate)
		}

		// Audio channels
		if cfg.NormalizeAudio {
			// Force stereo for maximum compatibility
			args = append(args, "-ac", "2")
			logging.Debug(logging.CatFFMPEG, "audio normalization: forcing stereo")
		} else if cfg.AudioChannels != "" && cfg.AudioChannels != "Source" {
			switch cfg.AudioChannels {
			case "Mono":
				args = append(args, "-ac", "1")
			case "Stereo":
				args = append(args, "-ac", "2")
			case "5.1":
				args = append(args, "-ac", "6")
			case "Left to Stereo":
				// Copy left channel to both left and right
				args = append(args, "-af", "pan=stereo|c0=c0|c1=c0")
			case "Right to Stereo":
				// Copy right channel to both left and right
				args = append(args, "-af", "pan=stereo|c0=c1|c1=c1")
			case "Mix to Stereo":
				// Downmix both channels together, then duplicate to L+R
				args = append(args, "-af", "pan=stereo|c0=0.5*c0+0.5*c1|c1=0.5*c0+0.5*c1")
			case "Swap L/R":
				// Swap left and right channels
				args = append(args, "-af", "pan=stereo|c0=c1|c1=c0")
			}
		}

		// Audio sample rate and normalize
		if cfg.NormalizeAudio {
			// Apply loudnorm filter with LUFS and TruePeak values
			lufs := cfg.NormalizeLUFS
			if lufs == 0 {
				lufs = -16
			}
			tp := cfg.NormalizeTruePeak
			if tp == 0 {
				tp = -1.5
			}
			args = append(args, "-af", fmt.Sprintf("loudnorm=I=%.0f:TP=%.1f:LRA=11", lufs, tp))
			logging.Debug(logging.CatFFMPEG, "audio normalization: %.0f LUFS, %.1f dBTP", lufs, tp)
		} else if cfg.AudioSampleRate != "" && cfg.AudioSampleRate != "Source" {
			args = append(args, "-ar", cfg.AudioSampleRate)
		}
	}
	// Map cover art as attached picture (must be before movflags and progress)
	if hasCoverArt {
		// Need to explicitly map streams when adding cover art
		args = append(args, "-map", "0:v", "-map", "0:a?", "-map", "1:v")
		// Set cover art codec to PNG (MP4 requires PNG or MJPEG for attached pics)
		args = append(args, "-c:v:1", "png")
		args = append(args, "-disposition:v:1", "attached_pic")
		logging.Debug(logging.CatFFMPEG, "convert: mapped cover art as attached picture with PNG codec")
	}

	// Ensure quickstart for MP4/MOV outputs.
	if strings.EqualFold(cfg.SelectedFormat.Ext, ".mp4") || strings.EqualFold(cfg.SelectedFormat.Ext, ".mov") {
		args = append(args, "-movflags", "+faststart")
	}

	// Apply target for DVD (must come before output path)
	// Note: We no longer use -target because it forces resolution changes.
	// DVD-specific parameters are set manually in the video codec section below.

	// Fix VFR/desync issues - regenerate timestamps and enforce CFR
	args = append(args, "-fflags", "+genpts")
	if cfg.FrameRate != "" && cfg.FrameRate != "Source" {
		args = append(args, "-r", cfg.FrameRate)
		logging.Debug(logging.CatFFMPEG, "enforcing CFR at %s fps", cfg.FrameRate)
	} else {
		// Use source frame rate as CFR
		args = append(args, "-r", fmt.Sprintf("%.3f", src.FrameRate))
		logging.Debug(logging.CatFFMPEG, "enforcing CFR at source rate %.3f fps", src.FrameRate)
	}

	// Inject BPS tag for MKV output so Windows Explorer and media tools show the
	// correct bitrate. Hardware encoders (AMF, NVENC) do not write per-stream stats
	// back to Matroska tag writers, leaving BPS as 0.
	if strings.EqualFold(cfg.SelectedFormat.Ext, ".mkv") && cfg.VideoCodec != "Copy" {
		if cfg.BitrateMode == "CBR" || cfg.BitrateMode == "VBR" {
			vb := cfg.VideoBitrate
			if vb == "" {
				vb = defaultBitrate(cfg.VideoCodec, src.Width, src.Bitrate)
			}
			if bps := parseBitrateStringToBPS(vb); bps > 0 {
				args = append(args, "-metadata:s:v:0", fmt.Sprintf("BPS=%d", bps))
			}
		}
	}

	// Progress feed to stdout for live updates.
	args = append(args, "-progress", "pipe:1", "-nostats")
	args = append(args, outPath)

	logging.Debug(logging.CatFFMPEG, "convert command: ffmpeg %s", strings.Join(args, " "))
	s.convertBusy = true
	s.convertProgress = 0
	s.convertActiveIn = src.Path
	s.convertActiveOut = outPath
	s.convertActiveLog = ""
	logFile, logPath, logErr := createConversionLog(src.Path, outPath, args)
	if logErr != nil {
		logging.Debug(logging.CatFFMPEG, "conversion log open failed: %v", logErr)
	} else {
		fmt.Fprintf(logFile, "Status: started\n\n")
		s.convertActiveLog = logPath
	}
	recoveryStarted := time.Now().Format(time.RFC3339)
	_ = saveConvertRecovery(convertRecoveryState{
		Active:    true,
		StartedAt: recoveryStarted,
		Input:     src.Path,
		Output:    outPath,
		LogPath:   logPath,
	})
	_ = logPath
	setStatus("Preparing conversion")
	// Widget states will be updated by the UI refresh ticker

	ctx, cancel := context.WithCancel(context.Background())
	s.convertCancel = cancel

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logging.Crash(logging.CatFFMPEG, "convert worker panic: %v", r)
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					s.showErrorWithCopy("Conversion Failed", fmt.Errorf("conversion worker crashed: %v", r))
					s.convertBusy = false
					s.convertActiveIn = ""
					s.convertActiveOut = ""
					s.convertActiveLog = ""
					s.convertProgress = 0
					setStatus("Failed")
				}, false)
				s.convertCancel = nil
			}
		}()
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			setStatus("Running ffmpeg")
		}, false)
		if logFile != nil {
			defer logFile.Close()
		}

		started := time.Now()
		cmd := utils.CreateCommand(ctx, utils.GetFFmpegPath(), args...)
		utils.ApplyNoWindow(cmd)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			logging.Debug(logging.CatFFMPEG, "convert stdout pipe failed: %v", err)
			_ = saveConvertRecovery(convertRecoveryState{
				Active:    false,
				StartedAt: recoveryStarted,
				Input:     src.Path,
				Output:    outPath,
				LogPath:   logPath,
			})
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				s.showErrorWithCopy("Conversion Failed", fmt.Errorf("convert failed: %w", err))
				s.convertBusy = false
				setStatus("Failed")
			}, false)
			s.convertCancel = nil
			return
		}
		var stderr bytes.Buffer
		if logFile != nil {
			cmd.Stderr = io.MultiWriter(&stderr, logFile)
		} else {
			cmd.Stderr = &stderr
		}

		progressQuit := make(chan struct{})
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logging.Crash(logging.CatFFMPEG, "convert progress panic: %v", r)
				}
			}()
			stdoutReader := io.Reader(stdout)
			if logFile != nil {
				stdoutReader = io.TeeReader(stdout, logFile)
			}
			scanner := bufio.NewScanner(stdoutReader)
			var currentFPS float64
			for scanner.Scan() {
				select {
				case <-progressQuit:
					return
				default:
				}
				line := scanner.Text()
				parts := strings.SplitN(line, "=", 2)
				if len(parts) != 2 {
					continue
				}
				key, val := parts[0], parts[1]

				// Capture FPS value
				if key == "fps" {
					if fps, err := strconv.ParseFloat(val, 64); err == nil {
						currentFPS = fps
					}
					continue
				}

				if key != "out_time_ms" && key != "progress" {
					continue
				}
				if key == "out_time_ms" {
					ms, err := strconv.ParseFloat(val, 64)
					if err != nil {
						continue
					}
					elapsedProc := ms / 1000000.0
					total := src.Duration
					var pct float64
					if total > 0 {
						pct = math.Min(100, math.Max(0, (elapsedProc/total)*100))
					}
					elapsedWall := time.Since(started).Seconds()
					var eta string
					if pct > 0 && elapsedWall > 0 && pct < 100 {
						remaining := elapsedWall * (100 - pct) / pct
						eta = formatShortDuration(remaining)
					}
					speed := 0.0
					if elapsedWall > 0 {
						speed = elapsedProc / elapsedWall
					}

					var etaDuration time.Duration
					if pct > 0 && elapsedWall > 0 && pct < 100 {
						remaining := elapsedWall * (100 - pct) / pct
						etaDuration = time.Duration(remaining * float64(time.Second))
					}

					// Build status with FPS
					var lbl string
					if currentFPS > 0 {
						lbl = fmt.Sprintf("Converting %.0f%% | %.0f fps | elapsed %s | ETA %s | %.2fx", pct, currentFPS, formatShortDuration(elapsedWall), etaOrDash(eta), speed)
					} else {
						lbl = fmt.Sprintf("Converting %.0f%% | elapsed %s | ETA %s | %.2fx", pct, formatShortDuration(elapsedWall), etaOrDash(eta), speed)
					}

					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						s.convertProgress = pct
						s.convertFPS = currentFPS
						s.convertSpeed = speed
						s.convertETA = etaDuration
						setStatus(lbl)
						// Keep stats bar and queue view in sync during direct converts
						s.updateStatsBar()
						if s.active == "queue" {
							s.refreshQueueView()
						}
					}, false)
				}
				if key == "progress" && val == "end" {
					return
				}
			}
		}()

		if err := utils.StartCmd(cmd); err != nil {
			close(progressQuit)
			logging.Error(logging.CatConvert, "convert failed to start: input=%s output=%s err=%v", src.Path, outPath, err)
			_ = saveConvertRecovery(convertRecoveryState{
				Active:    false,
				StartedAt: recoveryStarted,
				Input:     src.Path,
				Output:    outPath,
				LogPath:   logPath,
			})
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				s.showErrorWithCopy("Conversion Failed", fmt.Errorf("convert failed: %w", err))
				s.convertBusy = false
				s.convertProgress = 0
				setStatus("Failed")
			}, false)
			s.convertCancel = nil
			return
		}

		err = cmd.Wait()
		close(progressQuit)
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				logging.Debug(logging.CatFFMPEG, "convert cancelled")
				if logFile != nil {
					fmt.Fprintf(logFile, "\nStatus: cancelled at %s\n", time.Now().Format(time.RFC3339))
				}
				_ = saveConvertRecovery(convertRecoveryState{
					Active:    false,
					StartedAt: recoveryStarted,
					Input:     src.Path,
					Output:    outPath,
					LogPath:   logPath,
				})
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					s.convertBusy = false
					s.convertActiveIn = ""
					s.convertActiveOut = ""
					s.convertActiveLog = ""
					s.convertProgress = 0
					setStatus("Cancelled")
				}, false)
				s.convertCancel = nil
				return
			}
			stderrOutput := strings.TrimSpace(stderr.String())
			logging.Error(logging.CatConvert, "convert failed: input=%s output=%s err=%v stderr=%s", src.Path, outPath, err, stderrOutput)
			if logFile != nil {
				fmt.Fprintf(logFile, "\nStatus: failed at %s\nError: %v\nStderr:\n%s\n", time.Now().Format(time.RFC3339), err, stderrOutput)
			}
			_ = saveConvertRecovery(convertRecoveryState{
				Active:    false,
				StartedAt: recoveryStarted,
				Input:     src.Path,
				Output:    outPath,
				LogPath:   logPath,
			})
			// Detect hardware failure and retry once in software before surfacing error
			resolvedAccel := effectiveHardwareAccel(s.convert)
			isHardwareFailure := strings.Contains(stderrOutput, "No capable devices found") ||
				strings.Contains(stderrOutput, "Cannot load") ||
				strings.Contains(stderrOutput, "not available") &&
					(strings.Contains(stderrOutput, "nvenc") ||
						strings.Contains(stderrOutput, "amf") ||
						strings.Contains(stderrOutput, "qsv") ||
						strings.Contains(stderrOutput, "vaapi") ||
						strings.Contains(stderrOutput, "videotoolbox"))

			if isHardwareFailure && !strings.EqualFold(s.convert.HardwareAccel, "none") && resolvedAccel != "none" && resolvedAccel != "" {
				s.convert.HardwareAccel = "none"
				if logFile != nil {
					fmt.Fprintf(logFile, "\nAuto-fallback: hardware encoder failed; switched to software for next attempt at %s\n", time.Now().Format(time.RFC3339))
					_ = logFile.Close()
				}
				s.convertCancel = nil
			}

			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				errorExplanation := interpretFFmpegError(err)
				var errorMsg error

				// Check if this is a hardware encoding failure
				if isHardwareFailure && resolvedAccel != "none" && resolvedAccel != "" {
					chosen := s.convert.HardwareAccel
					if chosen == "" {
						chosen = "auto"
					}
					if strings.EqualFold(chosen, "auto") {
						// Auto failed; fall back to software for next runs
						s.convert.HardwareAccel = "none"
					}
					errorMsg = fmt.Errorf("Hardware encoding (%s%s) failed - no compatible hardware found.\n\nSwitched hardware acceleration to 'none'. Please try again (software encoding).\n\nFFmpeg output:\n%s", chosen, resolvedAccel, stderrOutput)
				} else {
					baseMsg := "convert failed: " + err.Error()
					if errorExplanation != "" {
						baseMsg = fmt.Sprintf("convert failed: %v - %s", err, errorExplanation)
					}

					if stderrOutput != "" {
						errorMsg = fmt.Errorf("%s\n\nFFmpeg output:\n%s", baseMsg, stderrOutput)
					} else {
						errorMsg = fmt.Errorf("%s", baseMsg)
					}
				}
				s.showErrorWithCopy("Conversion Failed", errorMsg)
				s.convertBusy = false
				s.convertActiveIn = ""
				s.convertActiveOut = ""
				s.convertActiveLog = ""
				s.convertProgress = 0
				setStatus("Failed")
			}, false)
			s.convertCancel = nil
			return
		}
		if logFile != nil {
			fmt.Fprintf(logFile, "\nStatus: completed OK at %s\n", time.Now().Format(time.RFC3339))
		}
		_ = saveConvertRecovery(convertRecoveryState{
			Active:    false,
			StartedAt: recoveryStarted,
			Input:     src.Path,
			Output:    outPath,
			LogPath:   logPath,
		})
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			setStatus("Validating output")
		}, false)
		if _, probeErr := probeVideo(outPath); probeErr != nil {
			logging.Error(logging.CatConvert, "convert probe failed: input=%s output=%s err=%v", src.Path, outPath, probeErr)
			_ = saveConvertRecovery(convertRecoveryState{
				Active:    false,
				StartedAt: recoveryStarted,
				Input:     src.Path,
				Output:    outPath,
				LogPath:   logPath,
			})
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				s.showErrorWithCopy("Conversion Failed", fmt.Errorf("conversion output is invalid: %w", probeErr))
				s.convertBusy = false
				s.convertActiveIn = ""
				s.convertActiveOut = ""
				s.convertActiveLog = ""
				s.convertProgress = 0
				setStatus("Failed")
			}, false)
			s.convertCancel = nil
			return
		}
		logging.Debug(logging.CatFFMPEG, "convert completed: %s", outPath)
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			dialog.ShowInformation(t.ModuleConvert, fmt.Sprintf("Saved %s", outPath), s.window)
			s.convertBusy = false
			s.convertActiveIn = ""
			s.convertActiveOut = ""
			s.convertActiveLog = ""
			s.convertProgress = 100
			setStatus("Done")

			// Auto-compare if enabled
			if s.autoCompare {
				go func() {
					// Probe the output file
					convertedSrc, err := probeVideo(outPath)
					if err != nil {
						logging.Debug(logging.CatModule, "auto-compare: failed to probe converted file: %v", err)
						return
					}

					// Load original and converted into compare slots
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						s.compareFile1 = src          // Original
						s.compareFile2 = convertedSrc // Converted
						s.showCompareView()
						logging.Debug(logging.CatModule, "auto-compare: loaded original vs converted")
					}, false)
				}()
			}
		}, false)
		s.convertCancel = nil
	}()
}

func formatShortDuration(seconds float64) string {
	if seconds <= 0 {
		return "0s"
	}
	d := time.Duration(seconds * float64(time.Second))
	if d >= time.Hour {
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
	if d >= time.Minute {
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%.0fs", d.Seconds())
}

func etaOrDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "--"
	}
	return s
}

// interpretFFmpegError adds a human-readable explanation for common FFmpeg error codes
func interpretFFmpegError(err error) string {
	if err == nil {
		return ""
	}

	// Extract exit code from error
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitCode := exitErr.ExitCode()

		// Common FFmpeg/OS error codes and their meanings
		switch exitCode {
		case 1:
			return "Generic error (check FFmpeg output for details)"
		case 2:
			return "Invalid command line arguments"
		case 126:
			return "Command cannot execute (permission denied)"
		case 127:
			return "Command not found (is FFmpeg installed?)"
		case 137:
			return "Process killed (out of memory?)"
		case 139:
			return "Segmentation fault (FFmpeg crashed)"
		case 143:
			return "Process terminated by signal (SIGTERM)"
		case 187:
			return "Protocol/format not found or filter syntax error (check input file format and filter settings)"
		case 255:
			return "FFmpeg error (check output for details)"
		default:
			if exitCode > 128 && exitCode < 160 {
				signal := exitCode - 128
				return fmt.Sprintf("Process terminated by signal %d", signal)
			}
			return fmt.Sprintf("Exit code %d", exitCode)
		}
	}

	return ""
}

func aspectFilters(target float64, mode string) []string {
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

	// Letterbox/Pillarbox: pad with black bars (auto-detects direction based on aspect ratio change)
	// Also handles legacy "Letterbox" and "Pillarbox" options for backwards compatibility
	if strings.EqualFold(mode, "Letterbox/Pillarbox") || strings.EqualFold(mode, "Letterbox") || strings.EqualFold(mode, "Pillarbox") {
		pad := fmt.Sprintf("pad=w='trunc(max(iw,ih*%[1]s)/2)*2':h='trunc(max(ih,iw/%[1]s)/2)*2':x='(ow-iw)/2':y='(oh-ih)/2':color=black", ar)
		return []string{pad, setDAR, "setsar=1"}
	}

	// Default fallback: same as Letterbox/Pillarbox
	pad := fmt.Sprintf("pad=w='trunc(max(iw,ih*%[1]s)/2)*2':h='trunc(max(ih,iw/%[1]s)/2)*2':x='(ow-iw)/2':y='(oh-ih)/2':color=black", ar)
	return []string{pad, setDAR, "setsar=1"}
}

func appendAspectMetadata(vf []string, dar float64) []string {
	if dar <= 0 {
		return vf
	}
	hasSetDAR := false
	hasSetSAR := false
	for _, f := range vf {
		if strings.HasPrefix(f, "setdar=") {
			hasSetDAR = true
		} else if strings.HasPrefix(f, "setsar=") {
			hasSetSAR = true
		}
	}
	if !hasSetDAR {
		vf = append(vf, fmt.Sprintf("setdar=%.6f", dar))
	}
	if !hasSetSAR {
		vf = append(vf, "setsar=1")
	}
	return vf
}

func targetResolutionDims(label string) (int, int) {
	label = strings.TrimSpace(label)
	switch label {
	case "360p":
		return 640, 360
	case "480p":
		return 854, 480
	case "540p":
		return 960, 540
	case "720p":
		return 1280, 720
	case "1080p":
		return 1920, 1080
	case "1440p":
		return 2560, 1440
	case "4K":
		return 3840, 2160
	case "8K":
		return 7680, 4320
	}
	if label == "" || strings.EqualFold(label, "Source") {
		return 0, 0
	}
	re := regexp.MustCompile(`(\d{3,5})\D+(\d{3,5})`)
	m := re.FindStringSubmatch(label)
	if len(m) == 3 {
		if w, err := strconv.Atoi(m[1]); err == nil {
			if h, err := strconv.Atoi(m[2]); err == nil {
				if w%2 != 0 {
					w++
				}
				if h%2 != 0 {
					h++
				}
				return w, h
			}
		}
	}
	return 0, 0
}

func aspectFiltersWithTarget(target float64, mode string, srcAspect float64, targetW int, targetH int) []string {
	if target <= 0 {
		return nil
	}
	ar := fmt.Sprintf("%.6f", target)
	setDAR := fmt.Sprintf("setdar=%s", ar)

	if strings.EqualFold(mode, "Auto") {
		if srcAspect > 0 && !utils.RatiosApproxEqual(srcAspect, target, 0.01) {
			if srcAspect < target {
				mode = "Letterbox/Pillarbox"
			} else {
				mode = "Crop"
			}
		}
	}

	if targetW > 0 && targetH > 0 {
		switch {
		case strings.EqualFold(mode, "Crop"):
			scale := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase", targetW, targetH)
			crop := fmt.Sprintf("crop=%d:%d:(in_w-%d)/2:(in_h-%d)/2", targetW, targetH, targetW, targetH)
			return []string{scale, crop, setDAR, "setsar=1"}
		case strings.EqualFold(mode, "Letterbox/Pillarbox") || strings.EqualFold(mode, "Letterbox") || strings.EqualFold(mode, "Pillarbox"):
			scale := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", targetW, targetH)
			pad := fmt.Sprintf("pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black", targetW, targetH)
			return []string{scale, pad, setDAR, "setsar=1"}
		case strings.EqualFold(mode, "Stretch"):
			scale := fmt.Sprintf("scale=%d:%d", targetW, targetH)
			return []string{scale, setDAR, "setsar=1"}
		case strings.EqualFold(mode, "Blur Fill"):
			filterStr := fmt.Sprintf("split[bg][fg];[bg]scale=%d:%d:force_original_aspect_ratio=decrease,boxblur=20:5[blurred];[blurred][fg]overlay=(W-w)/2:(H-h)/2", targetW, targetH)
			return []string{filterStr, setDAR, "setsar=1"}
		}
	}

	return aspectFilters(target, mode)
}

func (s *appState) generateSnippet() {
	t := i18n.T()
	if s.source == nil {
		return
	}
	src := s.source
	center := math.Max(0, src.Duration/2-10)
	start := fmt.Sprintf("%.2f", center)
	outName := fmt.Sprintf("%s-snippet-%d.mp4", strings.TrimSuffix(src.DisplayName, filepath.Ext(src.DisplayName)), time.Now().Unix())
	outPath := filepath.Join(filepath.Dir(src.Path), outName)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Build ffmpeg command with aspect ratio conversion if needed
	args := []string{
		"-ss", start,
		"-i", src.Path,
	}

	// Ensure aspect defaults to Source for snippets when unset
	if s.convert.OutputAspect == "" {
		s.convert.OutputAspect = "Source"
	}

	// Add cover art if available
	hasCoverArt := s.convert.CoverArtPath != ""
	logging.Debug(logging.CatFFMPEG, "snippet: CoverArtPath=%s hasCoverArt=%v", s.convert.CoverArtPath, hasCoverArt)
	if hasCoverArt {
		args = append(args, "-i", s.convert.CoverArtPath)
		logging.Debug(logging.CatFFMPEG, "snippet: added cover art input %s", s.convert.CoverArtPath)
	}

	// Build video filters using current settings (respect upscaling/AR/FPS)
	var vf []string

	// Skip deinterlacing for snippets - they're meant to be fast previews
	// Full conversions will still apply deinterlacing

	// Resolution scaling for snippets (only if explicitly set)
	if s.convert.TargetResolution != "" && s.convert.TargetResolution != "Source" {
		var scaleFilter string
		switch s.convert.TargetResolution {
		case "720p":
			scaleFilter = "scale=-2:720"
		case "1080p":
			scaleFilter = "scale=-2:1080"
		case "1440p":
			scaleFilter = "scale=-2:1440"
		case "4K":
			scaleFilter = "scale=-2:2160"
		case "8K":
			scaleFilter = "scale=-2:4320"
		}
		if scaleFilter != "" {
			vf = append(vf, scaleFilter)
		}
	}

	// Check if aspect ratio conversion is needed (only if user explicitly set OutputAspect)
	aspectExplicit := s.convert.OutputAspect != "" && !strings.EqualFold(s.convert.OutputAspect, "Source")
	if aspectExplicit {
		srcAspect := displayAspectRatioForSource(src)
		targetAspect := resolveTargetAspect(s.convert.OutputAspect, src)
		aspectConversionNeeded := targetAspect > 0 && srcAspect > 0 && !utils.RatiosApproxEqual(targetAspect, srcAspect, 0.01)
		if aspectConversionNeeded {
			vf = append(vf, aspectFilters(targetAspect, s.convert.AspectHandling)...)
		}
	}
	if targetAspect := resolveTargetAspect(s.convert.OutputAspect, src); s.convert.ForceAspect && targetAspect > 0 {
		if len(vf) == 0 {
			vf = append(vf, fmt.Sprintf("setdar=%.6f", targetAspect), "setsar=1")
		} else {
			vf = appendAspectMetadata(vf, targetAspect)
		}
	}

	// Frame rate conversion (only if explicitly set and different from source)
	if s.convert.FrameRate != "" && s.convert.FrameRate != "Source" {
		vf = append(vf, "fps="+s.convert.FrameRate)
	}

	// Decide if we must re-encode: filters, non-copy codec, or WMV
	isWMV := strings.HasSuffix(strings.ToLower(src.Path), ".wmv")
	forcedCodec := !strings.EqualFold(s.convert.VideoCodec, "Copy")
	needsReencode := len(vf) > 0 || isWMV || forcedCodec

	if len(vf) > 0 {
		filterStr := strings.Join(vf, ",")
		args = append(args, "-vf", filterStr)
	}

	// Map streams (including cover art if present)
	if hasCoverArt {
		args = append(args, "-map", "0:v", "-map", "0:a?", "-map", "1:v")
		logging.Debug(logging.CatFFMPEG, "snippet: mapped video, audio, and cover art")
	}

	// Set video codec - snippets should copy when possible for speed
	if !needsReencode {
		// No filters needed - use stream copy for fast snippets
		if hasCoverArt {
			args = append(args, "-c:v:0", "copy")
		} else {
			args = append(args, "-c:v", "copy")
		}
	} else {
		// Filters/codec require re-encode; use current settings
		videoCodec := determineVideoCodec(s.convert)
		if videoCodec == "copy" {
			videoCodec = "libx264"
		}
		args = append(args, "-c:v", videoCodec)

		// Bitrate/quality from current mode
		mode := s.convert.BitrateMode
		if mode == "" {
			mode = "CRF"
		}
		switch mode {
		case "CBR", "VBR":
			vb := s.convert.VideoBitrate
			if vb == "" {
				vb = defaultBitrate(s.convert.VideoCodec, src.Width, src.Bitrate)
			}
			args = append(args, "-b:v", vb)
			if mode == "CBR" {
				args = append(args, "-minrate", vb, "-maxrate", vb, "-bufsize", vb)
			}
		default: // CRF/Target size fallback to CRF
			crf := s.convert.CRF
			if crf == "" {
				crf = crfForQuality(s.convert.Quality)
			}
			if videoCodec == "libx264" || videoCodec == "libx265" {
				args = append(args, "-crf", crf)
			}
		}

		// Preset from current settings
		if s.convert.EncoderPreset != "" && (strings.Contains(videoCodec, "264") || strings.Contains(videoCodec, "265")) {
			args = append(args, "-preset", s.convert.EncoderPreset)
		}

		// Pixel format
		if s.convert.PixelFormat != "" {
			args = append(args, "-pix_fmt", s.convert.PixelFormat)
		}
	}

	// Set cover art codec (must be PNG or MJPEG for MP4)
	if hasCoverArt {
		args = append(args, "-c:v:1", "png")
		logging.Debug(logging.CatFFMPEG, "snippet: set cover art codec to PNG")
	}

	// Set audio codec - snippets should copy when possible for speed
	if !needsReencode {
		// No video filters - use audio stream copy for fast snippets
		args = append(args, "-c:a", "copy")
	} else {
		// Video is being re-encoded - may need to re-encode audio too
		audioCodec := determineAudioCodec(s.convert)
		if audioCodec == "copy" {
			audioCodec = "aac"
		}
		args = append(args, "-c:a", audioCodec)

		// Audio bitrate
		if s.convert.AudioBitrate != "" && audioCodec != "flac" {
			args = append(args, "-b:a", s.convert.AudioBitrate)
		}

		// Audio channels
		if s.convert.AudioChannels != "" && s.convert.AudioChannels != "Source" {
			switch s.convert.AudioChannels {
			case "Mono":
				args = append(args, "-ac", "1")
			case "Stereo":
				args = append(args, "-ac", "2")
			case "5.1":
				args = append(args, "-ac", "6")
			case "Left to Stereo":
				// Copy left channel to both left and right
				args = append(args, "-af", "pan=stereo|c0=c0|c1=c0")
			case "Right to Stereo":
				// Copy right channel to both left and right
				args = append(args, "-af", "pan=stereo|c0=c1|c1=c1")
			case "Mix to Stereo":
				// Downmix both channels together, then duplicate to L+R
				args = append(args, "-af", "pan=stereo|c0=0.5*c0+0.5*c1|c1=0.5*c0+0.5*c1")
			case "Swap L/R":
				// Swap left and right channels
				args = append(args, "-af", "pan=stereo|c0=c1|c1=c0")
			}
		}
	}

	// Mark cover art as attached picture
	if hasCoverArt {
		args = append(args, "-disposition:v:1", "attached_pic")
		logging.Debug(logging.CatFFMPEG, "snippet: set cover art disposition")
	}

	// Limit output duration to 20 seconds (must come after all codec/mapping options)
	args = append(args, "-t", "20")

	args = append(args, outPath)

	cmd := exec.CommandContext(ctx, utils.GetFFmpegPath(), args...)
	utils.ApplyNoWindow(cmd)
	logging.Debug(logging.CatFFMPEG, "snippet command: %s", strings.Join(cmd.Args, " "))

	// Show progress dialog for snippets that need re-encoding (WMV, filters, etc.)
	var progressDialog dialog.Dialog
	if needsReencode {
		progressDialog = dialog.NewCustom("Generating Snippet", "Cancel",
			widget.NewLabel(fmt.Sprintf(t.ConvertSnippetGenerating, 20)),
			s.window)
		progressDialog.Show()
	}

	// Run the snippet generation
	if out, err := cmd.CombinedOutput(); err != nil {
		logging.Debug(logging.CatFFMPEG, "snippet stderr: %s", string(out))
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			if progressDialog != nil {
				progressDialog.Hide()
			}
			dialog.ShowError(fmt.Errorf("snippet failed: %w", err), s.window)
		}, false)
		return
	}

	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		if progressDialog != nil {
			progressDialog.Hide()
		}
		dialog.ShowInformation(t.DialogSnippetCreated, fmt.Sprintf("Saved %s", outPath), s.window)
	}, false)
}

// sweepOrphanTempFiles removes VideoTools temp files left behind by previous
// sessions that crashed before cleanup could run. Safe to call in a goroutine
// at startup — it only removes files/dirs matching well-known VT prefixes.
func sweepOrphanTempFiles() {
	tmp := os.TempDir()
	entries, err := os.ReadDir(tmp)
	if err != nil {
		return
	}
	prefixes := []string{
		"videotools-frames-",
		"videotools-dvd-",
		"videotools-author-",
		"videotools-benchmark",
		"videotools-chapter-thumbs",
		"vt-cover-",
		"vt-trim-",
		"vt-merge-",
		"vt-ai-upscale-",
		"vt-rife-",
	}
	for _, e := range entries {
		name := e.Name()
		for _, pfx := range prefixes {
			if strings.HasPrefix(name, pfx) {
				_ = os.RemoveAll(filepath.Join(tmp, name))
				break
			}
		}
	}
}

func capturePreviewFrames(path string, duration float64) ([]string, error) {
	defer func() {
		if r := recover(); r != nil {
			logging.Crash(logging.CatFFMPEG, "panic in capturePreviewFrames(%s): %v", path, r)
		}
	}()
	center := math.Max(0, duration/2-1)
	start := fmt.Sprintf("%.2f", center)
	dir, err := os.MkdirTemp("", "videotools-frames-*")
	if err != nil {
		return nil, err
	}
	pattern := filepath.Join(dir, "frame-%03d.png")
	cmd := utils.CreateCommandRaw(utils.GetFFmpegPath(),
		"-y",
		"-ss", start,
		"-i", path,
		"-t", "3",
		"-vf", "scale=640:-2:flags=lanczos,fps=8",
		pattern,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(dir)
		logging.Error(logging.CatConvert, "preview capture failed: path=%s duration=%.1f err=%v output=%s", path, duration, err, strings.TrimSpace(string(out)))
		return nil, fmt.Errorf("preview capture failed: %w", err)
	}
	files, err := filepath.Glob(filepath.Join(dir, "frame-*.png"))
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("no preview frames generated")
	}
	slices.Sort(files)
	return files, nil
}

// cleanupVideoSourceTempFiles removes temporary files that VideoTools created
// for src: the preview-frame directory (videotools-frames-*) and any extracted
// embedded cover-art PNG. Safe to call with nil.
func cleanupVideoSourceTempFiles(src *videoSource) {
	if src == nil {
		return
	}
	if len(src.PreviewFrames) > 0 {
		dir := filepath.Dir(src.PreviewFrames[0])
		if strings.Contains(filepath.Base(dir), "videotools-frames-") {
			_ = os.RemoveAll(dir)
		}
	}
	if src.EmbeddedCoverArt != "" {
		_ = os.Remove(src.EmbeddedCoverArt)
	}
}

// cleanupCoverArtIfTemp removes path only if it is a VideoTools-generated
// temp cover-art file. Never deletes user-selected originals.
func cleanupCoverArtIfTemp(path string) {
	if path == "" {
		return
	}
	base := filepath.Base(path)
	if strings.HasPrefix(base, "videotools-cover-") || strings.HasPrefix(base, "videotools-embedded-cover-") {
		_ = os.Remove(path)
	}
}

type audioStreamInfo struct {
	Index    int
	Codec    string
	Language string
	Channels int
}

type inspectChapter struct {
	Index     int
	StartTime float64
	EndTime   float64
	Title     string
}

type videoSource struct {
	Path                  string
	DisplayName           string
	Format                string
	Width                 int
	Height                int
	Duration              float64
	VideoCodec            string
	AudioCodec            string
	Bitrate               int // Video bitrate in bits per second
	AudioBitrate          int // Audio bitrate in bits per second
	AudioBitrateEstimated bool
	FrameRate             float64
	PixelFormat           string
	AudioRate             int
	Channels              int
	FieldOrder            string
	PreviewFrames         []string
	EmbeddedCoverArt      string // Path to extracted embedded cover art, if any

	// Chapters
	Chapters []inspectChapter // Parsed chapter information

	// Advanced metadata
	SampleAspectRatio  string // Pixel Aspect Ratio (SAR) - e.g., "1:1", "40:33"
	DisplayAspectRatio string // Display Aspect Ratio (DAR) - e.g., "16:9"
	ColorSpace         string // Color space/primaries - e.g., "bt709", "bt601"
	ColorRange         string // Color range - "tv" (limited) or "pc" (full)
	ColorTransfer      string // Color transfer function - e.g., "bt1886", "smpte2084" (PQ), "hlg"
	ColorPrimaries     string // Color primaries - e.g., "bt709", "bt2020"
	GOPSize            int    // GOP size / keyframe interval
	HasChapters        bool   // Whether file has embedded chapters
	HasMetadata        bool   // Whether file has title/copyright/etc metadata
	Metadata           map[string]string

	// Multi-stream tracks (populated when needed for multitrack authoring)
	Audio     []audioStreamInfo
	Subtitles []subtitleStreamInfo
}

func (v *videoSource) GetFilePath() string { return v.Path }

func (v *videoSource) DurationString() string {
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

func (v *videoSource) AspectRatioString() string {
	if v.Width <= 0 || v.Height <= 0 {
		return "--"
	}
	if dar := strings.TrimSpace(v.DisplayAspectRatio); dar != "" && dar != "0:1" {
		if ratio := utils.ParseAspectValue(dar); ratio > 0 {
			return fmt.Sprintf("%s (%.2f:1)", dar, ratio)
		}
	}
	num, den := utils.SimplifyRatio(v.Width, v.Height)
	if num == 0 || den == 0 {
		return "--"
	}
	ratio := float64(num) / float64(den)
	return fmt.Sprintf("%d:%d (%.2f:1)", num, den, ratio)
}

func formatClock(sec float64) string {
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

func (v *videoSource) IsProgressive() bool {
	order := strings.ToLower(v.FieldOrder)
	if strings.Contains(order, "progressive") {
		return true
	}
	if strings.Contains(order, "unknown") && strings.Contains(strings.ToLower(v.PixelFormat), "p") {
		return true
	}
	return false
}

// validateRemuxCompatibility checks if source codecs are compatible with target container
// Returns: (compatible, reason, autoFixable)
// - compatible: true if remux is safe
// - reason: explanation of why it's incompatible (if false)
// - autoFixable: true if we can fix with FFmpeg flags (genpts, avoid_negative_ts, etc)
func validateRemuxCompatibility(src *videoSource, targetExt string, sourcePath string) (bool, string, bool) {
	if src == nil {
		return false, "source probe returned nil", false
	}

	videoCodec := strings.ToLower(src.VideoCodec)
	audioCodec := strings.ToLower(src.AudioCodec)
	sourceExt := strings.ToLower(filepath.Ext(sourcePath))
	targetExt = strings.ToLower(targetExt)

	// Normalize codec names for comparison
	videoCodec = normalizeCodecName(videoCodec)
	audioCodec = normalizeCodecName(audioCodec)

	// === CRITICAL BLOCKS: Must re-encode ===

	// 1. WMV/ASF: Known to have issues with MKV/MP4 remux
	if sourceExt == ".wmv" || sourceExt == ".asf" {
		return false, "WMV/ASF containers often have timestamp and codec issues - re-encoding recommended", false
	}

	// 2. Old FLV with proprietary codecs
	if sourceExt == ".flv" {
		if strings.Contains(videoCodec, "sorenson") || strings.Contains(videoCodec, "vp6") {
			return false, "FLV with legacy codecs (Sorenson/VP6) not well supported - re-encoding required", false
		}
		// H.264 FLV can be remuxed but often has timestamp issues (auto-fixable)
		if strings.Contains(videoCodec, "h264") || strings.Contains(videoCodec, "avc") {
			return true, "FLV H.264 detected - will apply timestamp fixes", true
		}
	}

	// 3. Codec compatibility with target container
	switch targetExt {
	case ".mp4":
		// MP4 supports: H.264, H.265, MPEG-4, AAC, MP3, AC3
		// Does NOT support: VP8, VP9, AV1 (reliably), Theora, Vorbis, Opus (without tricks)
		if strings.Contains(videoCodec, "vp8") || strings.Contains(videoCodec, "vp9") {
			return false, "VP8/VP9 not reliably supported in MP4 - use MKV or WebM", false
		}
		if strings.Contains(videoCodec, "av1") {
			return false, "AV1 in MP4 is experimental - use MKV for better compatibility", false
		}
		if strings.Contains(videoCodec, "theora") {
			return false, "Theora not supported in MP4 - use MKV or re-encode to H.264", false
		}
		if strings.Contains(audioCodec, "vorbis") || strings.Contains(audioCodec, "opus") {
			return false, "Vorbis/Opus not reliably supported in MP4 - use MKV or convert to AAC", false
		}

	case ".mkv":
		// MKV is ultra-flexible, supports almost everything
		// Only block truly broken/exotic codecs
		if strings.Contains(videoCodec, "wmv") && strings.Contains(videoCodec, "drm") {
			return false, "DRM-protected WMV cannot be remuxed", false
		}

	case ".webm":
		// WebM only supports: VP8, VP9, AV1, Vorbis, Opus
		if !strings.Contains(videoCodec, "vp8") && !strings.Contains(videoCodec, "vp9") && !strings.Contains(videoCodec, "av1") {
			return false, fmt.Sprintf("WebM only supports VP8/VP9/AV1 video (source: %s)", videoCodec), false
		}
		if !strings.Contains(audioCodec, "vorbis") && !strings.Contains(audioCodec, "opus") && audioCodec != "" {
			return false, fmt.Sprintf("WebM only supports Vorbis/Opus audio (source: %s)", audioCodec), false
		}

	case ".mov":
		// MOV/QuickTime is fairly flexible but has quirks
		// Generally compatible with H.264, H.265, ProRes, MJPEG
		// Can have issues with exotic codecs
	}

	// === AUTO-FIXABLE ISSUES ===

	// AVI files often have timestamp issues (fixable with genpts)
	if sourceExt == ".avi" {
		return true, "AVI source - will apply timestamp regeneration (genpts)", true
	}

	// Old MPEG-TS/PS files may have timestamp issues
	if sourceExt == ".ts" || sourceExt == ".m2ts" || sourceExt == ".mts" {
		return true, "MPEG transport stream - will apply timestamp fixes", true
	}

	// VOB files (DVD rips) often need timestamp fixes
	if sourceExt == ".vob" {
		return true, "VOB source - will apply timestamp regeneration", true
	}

	// All checks passed
	return true, "", false
}

// normalizeCodecName standardizes codec names for comparison
func normalizeCodecName(codec string) string {
	codec = strings.ToLower(strings.TrimSpace(codec))

	// Map common variations to standard names
	replacements := map[string]string{
		"h264":       "h264",
		"avc":        "h264",
		"avc1":       "h264",
		"h.264":      "h264",
		"x264":       "h264",
		"h265":       "h265",
		"hevc":       "h265",
		"h.265":      "h265",
		"x265":       "h265",
		"mpeg4":      "mpeg4",
		"divx":       "mpeg4",
		"xvid":       "mpeg4",
		"mpeg-4":     "mpeg4",
		"mpeg2":      "mpeg2",
		"mpeg-2":     "mpeg2",
		"mpeg2video": "mpeg2",
		"aac":        "aac",
		"mp3":        "mp3",
		"ac3":        "ac3",
		"a_ac3":      "ac3",
		"eac3":       "eac3",
		"vorbis":     "vorbis",
		"opus":       "opus",
		"vp8":        "vp8",
		"vp9":        "vp9",
		"av1":        "av1",
		"libaom-av1": "av1",
		"theora":     "theora",
		"wmv3":       "wmv",
		"vc1":        "vc1",
		"prores":     "prores",
		"prores_ks":  "prores",
		"mjpeg":      "mjpeg",
		"png":        "png",
	}

	for old, new := range replacements {
		if strings.Contains(codec, old) {
			return new
		}
	}

	return codec
}

func probeVideo(path string) (*videoSource, error) {
	defer func() {
		if r := recover(); r != nil {
			logging.Crash(logging.CatFFMPEG, "panic in probeVideo(%s): %v", path, r)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var fileSize int64
	if info, err := os.Stat(path); err == nil {
		fileSize = info.Size()
	}

	cmd := utils.CreateCommand(ctx, utils.GetFFprobePath(),
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-show_chapters",
		path,
	)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	out, err := cmd.Output()
	if err != nil {
		if detail := strings.TrimSpace(stderrBuf.String()); detail != "" {
			return nil, fmt.Errorf("%w\n%s", err, detail)
		}
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("ffprobe produced no output for %q — check that ffprobe.exe and its DLLs are present", filepath.Base(path))
	}

	var result struct {
		Format struct {
			Filename   string                 `json:"filename"`
			Format     string                 `json:"format_long_name"`
			Duration   string                 `json:"duration"`
			FormatName string                 `json:"format_name"`
			BitRate    string                 `json:"bit_rate"`
			Tags       map[string]interface{} `json:"tags"`
		} `json:"format"`
		Streams []struct {
			Index          int                    `json:"index"`
			CodecType      string                 `json:"codec_type"`
			CodecName      string                 `json:"codec_name"`
			Width          int                    `json:"width"`
			Height         int                    `json:"height"`
			Duration       string                 `json:"duration"`
			BitRate        string                 `json:"bit_rate"`
			PixFmt         string                 `json:"pix_fmt"`
			SampleRate     string                 `json:"sample_rate"`
			Channels       int                    `json:"channels"`
			AvgFrameRate   string                 `json:"avg_frame_rate"`
			RFrameRate     string                 `json:"r_frame_rate"`
			FieldOrder     string                 `json:"field_order"`
			SampleAspect   string                 `json:"sample_aspect_ratio"`
			DisplayAspect  string                 `json:"display_aspect_ratio"`
			ColorSpace     string                 `json:"color_space"`
			ColorRange     string                 `json:"color_range"`
			ColorTransfer  string                 `json:"color_transfer"`
			ColorPrimaries string                 `json:"color_primaries"`
			Tags           map[string]interface{} `json:"tags"`
			Disposition    struct {
				AttachedPic int `json:"attached_pic"`
			} `json:"disposition"`
		} `json:"streams"`
		Chapters []struct {
			ID        int    `json:"id"`
			TimeBase  string `json:"time_base"`
			Start     int    `json:"start"`
			StartTime string `json:"start_time"`
			EndTime   string `json:"end_time"`
			Tags      struct {
				Title string `json:"title"`
			} `json:"tags"`
		} `json:"chapters"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, err
	}

	src := &videoSource{
		Path:        path,
		DisplayName: filepath.Base(path),
		Format:      utils.FirstNonEmpty(result.Format.Format, result.Format.FormatName),
	}
	var formatBitrate int
	if rate, err := utils.ParseInt(result.Format.BitRate); err == nil {
		formatBitrate = rate
		src.Bitrate = rate
	}
	if durStr := result.Format.Duration; durStr != "" {
		if val, err := utils.ParseFloat(durStr); err == nil {
			src.Duration = val
		}
	}

	if len(result.Format.Tags) > 0 {
		src.Metadata = normalizeTags(result.Format.Tags)
		if len(src.Metadata) > 0 {
			src.HasMetadata = true
		}
	}

	// Check for chapters
	if len(result.Chapters) > 0 {
		src.HasChapters = true
		src.Chapters = make([]inspectChapter, 0, len(result.Chapters))
		duration := src.Duration
		for _, ch := range result.Chapters {
			startTime := 0.0
			endTime := 0.0
			if st, err := utils.ParseFloat(ch.StartTime); err == nil {
				startTime = st
			}
			if st, err := utils.ParseFloat(ch.EndTime); err == nil {
				endTime = st
			} else if duration > 0 {
				endTime = duration
			}
			title := ch.Tags.Title
			if title == "" {
				title = fmt.Sprintf("Chapter %d", ch.ID)
			}
			src.Chapters = append(src.Chapters, inspectChapter{
				Index:     ch.ID,
				StartTime: startTime,
				EndTime:   endTime,
				Title:     title,
			})
		}
		logging.Debug(logging.CatFFMPEG, "found %d chapter(s) in video", len(result.Chapters))
	}

	// Track if we've found the main video stream (not cover art)
	foundMainVideo := false
	var coverArtStreamIndex int = -1
	var videoStreamBitrate int

	for _, stream := range result.Streams {
		switch stream.CodecType {
		case "video":
			// Check if this is an attached picture (cover art)
			if stream.Disposition.AttachedPic == 1 {
				coverArtStreamIndex = stream.Index
				logging.Debug(logging.CatFFMPEG, "found embedded cover art at stream %d", stream.Index)
				continue
			}
			// Only use the first non-cover-art video stream
			if !foundMainVideo {
				foundMainVideo = true
				src.VideoCodec = stream.CodecName
				src.FieldOrder = stream.FieldOrder
				if stream.Width > 0 {
					src.Width = stream.Width
				}
				if stream.Height > 0 {
					src.Height = stream.Height
				}
				if dur, err := utils.ParseFloat(stream.Duration); err == nil && dur > 0 {
					src.Duration = dur
				}
				if fr := utils.ParseFraction(stream.AvgFrameRate); fr > 0 {
					src.FrameRate = fr
				} else if fr := utils.ParseFraction(stream.RFrameRate); fr > 0 && fr < 1000 {
					src.FrameRate = fr
				}
				if stream.PixFmt != "" {
					src.PixelFormat = stream.PixFmt
				}
				if stream.SampleAspect != "" && stream.SampleAspect != "0:1" {
					src.SampleAspectRatio = stream.SampleAspect
				}
				if stream.DisplayAspect != "" && stream.DisplayAspect != "0:1" {
					src.DisplayAspectRatio = stream.DisplayAspect
				}
				if stream.ColorSpace != "" && stream.ColorSpace != "unknown" {
					src.ColorSpace = stream.ColorSpace
				} else if stream.ColorPrimaries != "" && stream.ColorPrimaries != "unknown" {
					src.ColorSpace = stream.ColorPrimaries
				}
				if stream.ColorRange != "" && stream.ColorRange != "unknown" {
					src.ColorRange = stream.ColorRange
				}
				if stream.ColorTransfer != "" && stream.ColorTransfer != "unknown" {
					src.ColorTransfer = stream.ColorTransfer
				}
				if stream.ColorPrimaries != "" && stream.ColorPrimaries != "unknown" {
					src.ColorPrimaries = stream.ColorPrimaries
				}
				if br, err := utils.ParseInt(stream.BitRate); err == nil && br > 0 {
					videoStreamBitrate = br
				}
			}
			if src.Bitrate == 0 {
				if br, err := utils.ParseInt(stream.BitRate); err == nil {
					src.Bitrate = br
				}
			}
		case "audio":
			if src.AudioCodec == "" {
				src.AudioCodec = stream.CodecName
				if rate, err := utils.ParseInt(stream.SampleRate); err == nil {
					src.AudioRate = rate
				}
				if stream.Channels > 0 {
					src.Channels = stream.Channels
				}
				if br, err := utils.ParseInt(stream.BitRate); err == nil && br > 0 {
					src.AudioBitrate = br
				} else if br := parseBitrateTag(stream.Tags); br > 0 {
					src.AudioBitrate = br
				}
			}
			lang := ""
			if l, ok := stream.Tags["language"].(string); ok {
				lang = l
			}
			src.Audio = append(src.Audio, audioStreamInfo{
				Index:    stream.Index,
				Codec:    stream.CodecName,
				Language: lang,
				Channels: stream.Channels,
			})
		case "subtitle":
			lang := ""
			if l, ok := stream.Tags["language"].(string); ok {
				lang = l
			}
			src.Subtitles = append(src.Subtitles, subtitleStreamInfo{
				Index:    stream.Index,
				Codec:    stream.CodecName,
				Language: lang,
			})
		}
	}

	if src.AudioCodec != "" && src.AudioBitrate == 0 {
		totalBps := 0
		if formatBitrate > 0 {
			totalBps = formatBitrate
		} else if src.Duration > 0 && fileSize > 0 {
			totalBps = int(float64(fileSize*8) / src.Duration)
		}

		baseVideo := videoStreamBitrate
		if baseVideo == 0 && formatBitrate == 0 && src.Bitrate > 0 {
			baseVideo = src.Bitrate
		}

		estimated := 0
		if totalBps > 0 && baseVideo > 0 && totalBps > baseVideo {
			estimated = totalBps - baseVideo
		}
		if estimated == 0 {
			estimated = defaultAudioBitrate(src.Channels)
		}
		if estimated > 0 {
			src.AudioBitrate = estimated
			src.AudioBitrateEstimated = true
		}
	}

	// Extract embedded cover art if present
	if coverArtStreamIndex >= 0 {
		coverPath := filepath.Join(utils.TempDir(), fmt.Sprintf("videotools-embedded-cover-%d.png", time.Now().UnixNano()))
		extractCmd := utils.CreateCommand(ctx, utils.GetFFmpegPath(),
			"-i", path,
			"-map", fmt.Sprintf("0:%d", coverArtStreamIndex),
			"-frames:v", "1",
			"-y",
			coverPath,
		)
		if err := extractCmd.Run(); err != nil {
			logging.Debug(logging.CatFFMPEG, "failed to extract embedded cover art: %v", err)
		} else {
			src.EmbeddedCoverArt = coverPath
			logging.Debug(logging.CatFFMPEG, "extracted embedded cover art to %s", coverPath)
		}
	}

	return src, nil
}

func parseBitrateTag(tags map[string]interface{}) int {
	if len(tags) == 0 {
		return 0
	}
	keys := []string{"BPS", "BPS-eng", "bit_rate", "variant_bitrate"}
	for _, key := range keys {
		if val, ok := tags[key]; ok {
			if rate, err := utils.ParseInt(fmt.Sprint(val)); err == nil && rate > 0 {
				return rate
			}
		}
	}
	return 0
}

func defaultAudioBitrate(channels int) int {
	switch channels {
	case 1:
		return 96000
	case 2:
		return 128000
	case 6:
		return 256000
	case 8:
		return 320000
	default:
		return 128000
	}
}

func normalizeTags(tags map[string]interface{}) map[string]string {
	normalized := make(map[string]string, len(tags))
	for k, v := range tags {
		key := strings.ToLower(strings.TrimSpace(k))
		if key == "" {
			continue
		}
		val := strings.TrimSpace(fmt.Sprint(v))
		if val != "" {
			normalized[key] = val
		}
	}
	return normalized
}

// CropValues represents detected crop parameters
type CropValues struct {
	Width  int
	Height int
	X      int
	Y      int
}

// detectCrop runs cropdetect analysis on a video to find black bars
// Returns nil if no crop is detected or if detection fails
func detectCrop(path string, duration float64) *CropValues {
	// First, get source video dimensions for validation
	src, err := probeVideo(path)
	if err != nil {
		logging.Debug(logging.CatFFMPEG, "failed to probe video for crop detection: %v", err)
		return nil
	}
	sourceWidth := src.Width
	sourceHeight := src.Height
	logging.Debug(logging.CatFFMPEG, "source dimensions: %dx%d", sourceWidth, sourceHeight)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Sample 10 seconds from the middle of the video
	sampleStart := duration / 2
	if sampleStart < 0 {
		sampleStart = 0
	}

	// Run ffmpeg with cropdetect filter
	cmd := utils.CreateCommand(ctx, utils.GetFFmpegPath(),
		"-ss", fmt.Sprintf("%.2f", sampleStart),
		"-i", path,
		"-t", "10", // 10-second sample
		"-vf", "cropdetect",
		"-f", "null",
		"-",
	)
	utils.ApplyNoWindow(cmd)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.Debug(logging.CatFFMPEG, "cropdetect failed: %v", err)
		return nil
	}

	// Parse the output to find the most common crop values
	// Look for lines like: [Parsed_cropdetect_0 @ 0x...] x1:0 x2:1919 y1:0 y2:803 w:1920 h:800 x:0 y:2 pts:... t:... crop=1920:800:0:2
	outputStr := string(output)
	cropRegex := regexp.MustCompile(`crop=(\d+):(\d+):(\d+):(\d+)`)

	// Find all crop suggestions
	matches := cropRegex.FindAllStringSubmatch(outputStr, -1)
	if len(matches) == 0 {
		logging.Debug(logging.CatFFMPEG, "no crop values detected")
		return nil
	}

	// Use the last crop value (most stable after initial detection)
	lastMatch := matches[len(matches)-1]
	if len(lastMatch) != 5 {
		return nil
	}

	width, _ := strconv.Atoi(lastMatch[1])
	height, _ := strconv.Atoi(lastMatch[2])
	x, _ := strconv.Atoi(lastMatch[3])
	y, _ := strconv.Atoi(lastMatch[4])

	logging.Debug(logging.CatFFMPEG, "detected crop: %dx%d at %d,%d", width, height, x, y)

	// Validate crop dimensions
	if width <= 0 || height <= 0 {
		logging.Debug(logging.CatFFMPEG, "invalid crop dimensions: width=%d height=%d", width, height)
		return nil
	}

	// Ensure crop doesn't exceed source dimensions
	if width > sourceWidth {
		logging.Debug(logging.CatFFMPEG, "crop width %d exceeds source width %d, clamping", width, sourceWidth)
		width = sourceWidth
	}
	if height > sourceHeight {
		logging.Debug(logging.CatFFMPEG, "crop height %d exceeds source height %d, clamping", height, sourceHeight)
		height = sourceHeight
	}

	// Ensure crop position + size doesn't exceed source
	if x+width > sourceWidth {
		logging.Debug(logging.CatFFMPEG, "crop x+width exceeds source, adjusting x from %d to %d", x, sourceWidth-width)
		x = sourceWidth - width
		if x < 0 {
			x = 0
			width = sourceWidth
		}
	}
	if y+height > sourceHeight {
		logging.Debug(logging.CatFFMPEG, "crop y+height exceeds source, adjusting y from %d to %d", y, sourceHeight-height)
		y = sourceHeight - height
		if y < 0 {
			y = 0
			height = sourceHeight
		}
	}

	// Ensure even dimensions (required for many codecs)
	if width%2 != 0 {
		width -= 1
		logging.Debug(logging.CatFFMPEG, "adjusted width to even number: %d", width)
	}
	if height%2 != 0 {
		height -= 1
		logging.Debug(logging.CatFFMPEG, "adjusted height to even number: %d", height)
	}

	// If crop is the same as source, no cropping needed
	if width == sourceWidth && height == sourceHeight {
		logging.Debug(logging.CatFFMPEG, "crop dimensions match source, no cropping needed")
		return nil
	}

	logging.Debug(logging.CatFFMPEG, "validated crop: %dx%d at %d,%d", width, height, x, y)
	return &CropValues{
		Width:  width,
		Height: height,
		X:      x,
		Y:      y,
	}
}

// formatBitrate formats a bitrate in bits/s to a human-readable string
func formatBitrate(bps int) string {
	if bps == 0 {
		return "N/A"
	}
	kbps := float64(bps) / 1000.0
	if kbps >= 1000 {
		return fmt.Sprintf("%.1f Mbps", kbps/1000.0)
	}
	return fmt.Sprintf("%.0f kbps", kbps)
}

// formatBitrateFull shows both Mbps and kbps.
func formatBitrateFull(bps int) string {
	if bps <= 0 {
		return "N/A"
	}
	kbps := float64(bps) / 1000.0
	mbps := kbps / 1000.0
	if kbps >= 1000 {
		return fmt.Sprintf("%.1f Mbps (%.0f kbps)", mbps, kbps)
	}
	return fmt.Sprintf("%.0f kbps (%.2f Mbps)", kbps, mbps)
}

// generateJobThumbnail generates a midpoint thumbnail for a queue job.
// It runs in a background goroutine and updates the job's ThumbnailPath when done.
func (s *appState) generateJobThumbnail(job *queue.Job) {
	if job == nil || job.InputFile == "" {
		return
	}
	// Check if thumbnail already exists
	if job.ThumbnailPath != "" {
		return
	}
	// Generate thumbnail in background
	go func() {
		tmpDir := os.TempDir()
		thumbPath := filepath.Join(tmpDir, fmt.Sprintf("vt-thumb-%s.jpg", job.ID))
		
		// Use the thumbnail generator to extract a midpoint frame
		generator := thumbnail.NewGenerator(utils.GetFFmpegPath())
		duration := 0.0
		if d, ok := job.Config["sourceDuration"].(float64); ok {
			duration = d
		}
		if duration <= 0 {
			// Try to get duration from ffprobe
			// For simplicity, just use 10s as default
			duration = 60.0
		}
		midpoint := duration / 2.0
		
		err := generator.ExtractFrame(context.Background(), job.InputFile, midpoint, thumbPath, 120, 68)
		if err != nil {
			logging.Debug(logging.CatSystem, "failed to generate thumbnail for job %s: %v", job.ID, err)
			return
		}
		
		// Update the job's ThumbnailPath and refresh the queue card
		job.ThumbnailPath = thumbPath
		logging.Debug(logging.CatSystem, "generated thumbnail for job %s: %s", job.ID, thumbPath)
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			s.refreshQueueView()
		}, false)
	}()
}
