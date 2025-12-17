package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
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
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/benchmark"
	"git.leaktechnologies.dev/stu/VideoTools/internal/convert"
	"git.leaktechnologies.dev/stu/VideoTools/internal/interlace"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/modules"
	"git.leaktechnologies.dev/stu/VideoTools/internal/player"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/thumbnail"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
	"github.com/hajimehoshi/oto"
)

// Module describes a high level tool surface that gets a tile on the menu.
type Module struct {
	ID       string
	Label    string
	Color    color.Color
	Category string
	Handle   func(files []string)
}

var (
	debugFlag = flag.Bool("debug", false, "enable verbose logging (env: VIDEOTOOLS_DEBUG=1)")

	backgroundColor = utils.MustHex("#0B0F1A")
	gridColor       = utils.MustHex("#171C2A")
	textColor       = utils.MustHex("#E1EEFF")
	queueColor      = utils.MustHex("#5961FF")

	conversionLogSuffix = ".videotools.log"

	logsDirOnce     sync.Once
	logsDirPath     string
	feedbackBundler = utils.NewFeedbackBundler()
	appVersion      = "v0.1.0-dev18"

	hwAccelProbeOnce sync.Once
	hwAccelSupported atomic.Value // map[string]bool

	nvencRuntimeOnce sync.Once
	nvencRuntimeOK   bool

	modulesList = []Module{
		{"convert", "Convert", utils.MustHex("#8B44FF"), "Convert", modules.HandleConvert},  // Violet
		{"merge", "Merge", utils.MustHex("#4488FF"), "Convert", modules.HandleMerge},        // Blue
		{"trim", "Trim", utils.MustHex("#44DDFF"), "Convert", modules.HandleTrim},           // Cyan
		{"filters", "Filters", utils.MustHex("#44FF88"), "Convert", modules.HandleFilters},  // Green
		{"upscale", "Upscale", utils.MustHex("#AAFF44"), "Advanced", modules.HandleUpscale}, // Yellow-Green
		{"audio", "Audio", utils.MustHex("#FFD744"), "Convert", modules.HandleAudio},        // Yellow
		{"thumb", "Thumb", utils.MustHex("#FF8844"), "Screenshots", modules.HandleThumb},    // Orange
		{"compare", "Compare", utils.MustHex("#FF44AA"), "Inspect", modules.HandleCompare},  // Pink
		{"inspect", "Inspect", utils.MustHex("#FF4444"), "Inspect", modules.HandleInspect},  // Red
		{"player", "Player", utils.MustHex("#44FFDD"), "Playback", modules.HandlePlayer},    // Teal
	}

	// Platform-specific configuration
	platformConfig *PlatformConfig
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

// statusStrip renders a consistent dark status area with the shared stats bar.
func statusStrip(bar *ui.ConversionStatsBar) fyne.CanvasObject {
	bg := canvas.NewRectangle(color.NRGBA{R: 34, G: 34, B: 34, A: 255})
	bg.SetMinSize(fyne.NewSize(0, 32))
	// Make the entire bar area clickable by letting the bar fill the strip
	content := container.NewPadded(container.NewMax(bar))
	return container.NewMax(bg, content)
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

// resolveTargetAspect resolves an aspect ratio value or source aspect
func resolveTargetAspect(val string, src *videoSource) float64 {
	if strings.EqualFold(val, "source") {
		if src != nil {
			return utils.AspectRatioFloat(src.Width, src.Height)
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
Started: %s
Input: %s
Output: %s
Command: ffmpeg %s

`, time.Now().Format(time.RFC3339), inputPath, outputPath, strings.Join(args, " "))
	if _, err := f.WriteString(header); err != nil {
		_ = f.Close()
		return nil, logPath, err
	}
	return f, logPath, nil
}

func getLogsDir() string {
	logsDirOnce.Do(func() {
		// Prefer a logs folder next to the executable
		if exe, err := os.Executable(); err == nil {
			if dir := filepath.Dir(exe); dir != "" {
				logsDirPath = filepath.Join(dir, "logs")
			}
		}
		// Fallback to cwd/logs
		if logsDirPath == "" {
			logsDirPath = filepath.Join(".", "logs")
		}
		_ = os.MkdirAll(logsDirPath, 0o755)
	})
	return logsDirPath
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

// effectiveHardwareAccel resolves "auto" to a best-effort hardware encoder for the platform.
func effectiveHardwareAccel(cfg convertConfig) string {
	accel := strings.ToLower(cfg.HardwareAccel)
	if accel != "" && accel != "auto" {
		return accel
	}

	switch runtime.GOOS {
	case "windows":
		// Prefer NVENC, then Intel (QSV), then AMD (AMF)
		return "nvenc"
	case "darwin":
		return "videotoolbox"
	default: // linux and others
		// Prefer NVENC, then Intel (QSV), then VAAPI
		return "nvenc"
	}
}

// hwAccelAvailable checks ffmpeg -hwaccels once and caches the result.
func hwAccelAvailable(accel string) bool {
	accel = strings.ToLower(accel)
	if accel == "" || accel == "none" {
		return false
	}

	hwAccelProbeOnce.Do(func() {
		supported := make(map[string]bool)
		cmd := exec.Command("ffmpeg", "-hide_banner", "-v", "error", "-hwaccels")
		output, err := cmd.Output()
		if err != nil {
			hwAccelSupported.Store(supported)
			return
		}
		for _, line := range strings.Split(string(output), "\n") {
			line = strings.ToLower(strings.TrimSpace(line))
			switch line {
			case "cuda":
				supported["nvenc"] = true
			case "qsv":
				supported["qsv"] = true
			case "vaapi":
				supported["vaapi"] = true
			case "videotoolbox":
				supported["videotoolbox"] = true
			}
		}
		hwAccelSupported.Store(supported)
	})

	val := hwAccelSupported.Load()
	if val == nil {
		return false
	}
	supported := val.(map[string]bool)

	// Treat AMF as available if any GPU accel was detected; ffmpeg -hwaccels may not list it.
	if accel == "amf" {
		return supported["nvenc"] || supported["qsv"] || supported["vaapi"] || supported["videotoolbox"]
	}
	if accel == "nvenc" && supported["nvenc"] {
		if !nvencRuntimeAvailable() {
			return false
		}
	}
	return supported[accel]
}

// nvencRuntimeAvailable runs a lightweight encode probe to verify the NVENC runtime is usable (nvcuda.dll loaded).
func nvencRuntimeAvailable() bool {
	nvencRuntimeOnce.Do(func() {
		cmd := exec.Command(platformConfig.FFmpegPath,
			"-hide_banner", "-loglevel", "error",
			"-f", "lavfi", "-i", "color=size=16x16:rate=1",
			"-frames:v", "1",
			"-c:v", "h264_nvenc",
			"-f", "null", "-",
		)
		utils.ApplyNoWindow(cmd)
		if err := cmd.Run(); err == nil {
			nvencRuntimeOK = true
		} else {
			logging.Debug(logging.CatFFMPEG, "nvenc runtime check failed: %v", err)
		}
	})
	return nvencRuntimeOK
}

// openLogViewer opens a simple dialog showing the log content. If live is true, it auto-refreshes.
func (s *appState) openLogViewer(title, path string, live bool) {
	if strings.TrimSpace(path) == "" {
		dialog.ShowInformation("No Log", "No log available.", s.window)
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to read log: %w", err), s.window)
		return
	}

	text := widget.NewMultiLineEntry()
	text.SetText(string(data))
	text.Wrapping = fyne.TextWrapWord
	text.TextStyle = fyne.TextStyle{Monospace: true}
	text.Disable()
	bg := canvas.NewRectangle(color.NRGBA{0x15, 0x1a, 0x24, 0xff}) // slightly lighter than app bg
	scroll := container.NewVScroll(container.NewMax(bg, text))
	// Adaptive min size - allows proper scaling on small screens
	scroll.SetMinSize(fyne.NewSize(600, 350))

	stop := make(chan struct{})
	if live {
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					b, err := os.ReadFile(path)
					if err != nil {
						b = []byte(fmt.Sprintf("failed to read log: %v", err))
					}
					content := string(b)
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						text.SetText(content)
					}, false)
				}
			}
		}()
	}

	closeBtn := widget.NewButton("Close", func() {
		close(stop)
	})
	copyBtn := widget.NewButton("Copy All", func() {
		s.window.Clipboard().SetContent(text.Text)
	})
	buttons := container.NewHBox(copyBtn, layout.NewSpacer(), closeBtn)
	content := container.NewBorder(nil, buttons, nil, nil, scroll)
	d := dialog.NewCustom(title, "Close", content, s.window)
	d.SetOnClosed(func() { close(stop) })
	d.Show()
}

// openFolder tries to open a folder in the OS file browser.
func openFolder(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is empty")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	utils.ApplyNoWindow(cmd)
	return cmd.Start()
}

func (s *appState) showAbout() {
	version := fmt.Sprintf("VideoTools %s", appVersion)
	dev := "Leak Technologies"
	logsPath := getLogsDir()

	versionText := widget.NewLabel(version)
	devText := widget.NewLabel(fmt.Sprintf("Developer: %s", dev))
	logsLink := widget.NewButton("Open Logs Folder", func() {
		if err := openFolder(logsPath); err != nil {
			dialog.ShowError(fmt.Errorf("failed to open logs folder: %w", err), s.window)
		}
	})
	logsLink.Importance = widget.LowImportance

	donateURL, _ := url.Parse("https://leaktechnologies.dev/support")
	donateLink := widget.NewHyperlink("Support development", donateURL)

	body := container.NewVBox(
		versionText,
		devText,
		logsLink,
		donateLink,
		widget.NewLabel("Feedback: use the Logs button on the main menu to view logs; send issues with attached logs."),
	)
	dialog.ShowCustom("About & Support", "Close", body, s.window)
}

type formatOption struct {
	Label      string
	Ext        string
	VideoCodec string
}

var formatOptions = []formatOption{
	{"MP4 (H.264)", ".mp4", "libx264"},
	{"MP4 (H.265)", ".mp4", "libx265"},
	{"MKV (H.265)", ".mkv", "libx265"},
	{"MOV (ProRes)", ".mov", "prores_ks"},
	{"DVD-NTSC (MPEG-2)", ".mpg", "mpeg2video"},
	{"DVD-PAL (MPEG-2)", ".mpg", "mpeg2video"},
}

type convertConfig struct {
	OutputBase       string
	SelectedFormat   formatOption
	Quality          string // Preset quality (Draft/Standard/High/Lossless)
	Mode             string // Simple or Advanced
	UseAutoNaming    bool
	AutoNameTemplate string // Template for metadata-driven naming, e.g., "<actress> - <studio> - <scene>"

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

	// Other settings
	InverseTelecine  bool
	InverseAutoNotes string
	CoverArtPath     string
	AspectHandling   string
	OutputAspect     string
	AspectUserSet    bool // Tracks if user explicitly set OutputAspect
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

// defaultConvertConfigPath returns the path to the persisted convert config.
func defaultConvertConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		home := os.Getenv("HOME")
		if home != "" {
			configDir = filepath.Join(home, ".config")
		}
	}
	if configDir == "" {
		return "convert.json"
	}
	return filepath.Join(configDir, "VideoTools", "convert.json")
}

// loadPersistedConvertConfig loads the saved convert configuration from disk.
func loadPersistedConvertConfig() (convertConfig, error) {
	var cfg convertConfig
	path := defaultConvertConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.OutputAspect == "" {
		cfg.OutputAspect = "Source"
		cfg.AspectUserSet = false
	} else if !strings.EqualFold(cfg.OutputAspect, "Source") {
		cfg.AspectUserSet = true
	}
	// Always default FrameRate to Source if not set to avoid unwanted conversions
	if cfg.FrameRate == "" {
		cfg.FrameRate = "Source"
	}
	return cfg, nil
}

// savePersistedConvertConfig writes the convert configuration to disk.
func savePersistedConvertConfig(cfg convertConfig) error {
	path := defaultConvertConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// benchmarkRun represents a single benchmark test run
type benchmarkRun struct {
	Timestamp          time.Time          `json:"timestamp"`
	Results            []benchmark.Result `json:"results"`
	RecommendedEncoder string             `json:"recommended_encoder"`
	RecommendedPreset  string             `json:"recommended_preset"`
	RecommendedHWAccel string             `json:"recommended_hwaccel"`
	RecommendedFPS     float64            `json:"recommended_fps"`
}

// benchmarkConfig holds benchmark history
type benchmarkConfig struct {
	History []benchmarkRun `json:"history"`
}

func benchmarkConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		home := os.Getenv("HOME")
		if home != "" {
			configDir = filepath.Join(home, ".config")
		}
	}
	if configDir == "" {
		return "benchmark.json"
	}
	return filepath.Join(configDir, "VideoTools", "benchmark.json")
}

func loadBenchmarkConfig() (benchmarkConfig, error) {
	path := benchmarkConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return benchmarkConfig{}, err
	}
	var cfg benchmarkConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return benchmarkConfig{}, err
	}
	return cfg, nil
}

func saveBenchmarkConfig(cfg benchmarkConfig) error {
	path := benchmarkConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

type appState struct {
	window                    fyne.Window
	active                    string
	lastModule                string
	navigationHistory         []string // Track module navigation history for back/forward buttons
	navigationHistoryPosition int      // Current position in navigation history
	navigationHistorySuppress bool     // Temporarily suppress history tracking during navigation
	source                    *videoSource
	loadedVideos              []*videoSource // Multiple loaded videos for navigation
	currentIndex              int            // Current video index in loadedVideos
	anim                      *previewAnimator
	convert                   convertConfig
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
	playSess                  *playSession
	jobQueue                  *queue.Queue
	statsBar                  *ui.ConversionStatsBar
	queueBtn                  *widget.Button
	queueScroll               *container.Scroll
	queueOffset               fyne.Position
	compareFile1              *videoSource
	compareFile2              *videoSource
	inspectFile               *videoSource
	inspectInterlaceResult    *interlace.DetectionResult
	inspectInterlaceAnalyzing bool
	autoCompare               bool // Auto-load Compare module after conversion

	// Merge state
	mergeClips               []mergeClip
	mergeFormat              string
	mergeOutput              string
	mergeKeepAll             bool
	mergeCodecMode           string
	mergeChapters            bool
	mergeDVDRegion           string // "NTSC" or "PAL"
	mergeDVDAspect           string // "16:9" or "4:3"
	mergeFrameRate           string // Source, 24, 30, 60, or custom
	mergeMotionInterpolation bool   // Use motion interpolation for frame rate changes

	// Thumbnail module state
	thumbFile           *videoSource
	thumbCount          int
	thumbWidth          int
	thumbContactSheet   bool
	thumbColumns        int
	thumbRows           int
	thumbLastOutputPath string // Path to last generated output

	// Player module state
	playerFile *videoSource

	// Filters module state
	filtersFile       *videoSource
	filterBrightness  float64
	filterContrast    float64
	filterSaturation  float64
	filterSharpness   float64
	filterDenoise     float64
	filterRotation    int // 0, 90, 180, 270
	filterFlipH       bool
	filterFlipV       bool
	filterGrayscale   bool
	filterActiveChain []string // Active filter chain

	// Upscale module state
	upscaleFile                *videoSource
	upscaleMethod              string   // lanczos, bicubic, spline, bilinear
	upscaleTargetRes           string   // 720p, 1080p, 1440p, 4K, 8K, Custom
	upscaleCustomWidth         int      // For custom resolution
	upscaleCustomHeight        int      // For custom resolution
	upscaleAIEnabled           bool     // Use AI upscaling if available
	upscaleAIModel             string   // realesrgan, realesrgan-anime, none
	upscaleAIAvailable         bool     // Runtime detection
	upscaleApplyFilters        bool     // Apply filters from Filters module
	upscaleFilterChain         []string // Transferred filters from Filters module
	upscaleFrameRate           string   // Source, 24, 30, 60, or custom
	upscaleMotionInterpolation bool     // Use motion interpolation for frame rate changes

	// Snippet settings
	snippetLength       int  // Length of snippet in seconds (default: 20)
	snippetSourceFormat bool // true = source format, false = conversion format (default: true)

	// Interlacing detection state
	interlaceResult    *interlace.DetectionResult
	interlaceAnalyzing bool
}

type mergeClip struct {
	Path     string
	Chapter  string
	Duration float64
}

func (s *appState) persistConvertConfig() {
	if err := savePersistedConvertConfig(s.convert); err != nil {
		logging.Debug(logging.CatSystem, "failed to persist convert config: %v", err)
	}
}

func (s *appState) stopPreview() {
	if s.anim != nil {
		s.anim.Stop()
		s.anim = nil
	}
}

func toString(v interface{}) string {
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

func (s *appState) updateStatsBar() {
	if s.statsBar == nil || s.jobQueue == nil {
		return
	}

	pending, running, completed, failed, cancelled := s.jobQueue.Stats()

	// Find the currently running job to get its progress and stats
	var progress, fps, speed float64
	var eta, jobTitle string
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

	s.statsBar.UpdateStatsWithDetails(running, pending, completed, failed, cancelled, progress, fps, speed, eta, jobTitle)
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

func (s *appState) startPreview(frames []string, img *canvas.Image, slider *widget.Slider) {
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
	return &mouseButtonRenderer{
		handler: m,
		content: m.content,
	}
}

func (m *mouseButtonHandler) MouseDown(me *desktop.MouseEvent) {
	// Button 3 = Back button (typically mouse button 4)
	// Button 4 = Forward button (typically mouse button 5)
	if me.Button == desktop.MouseButtonTertiary+1 { // Back button
		m.state.navigateBack()
	} else if me.Button == desktop.MouseButtonTertiary+2 { // Forward button
		m.state.navigateForward()
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

func (s *appState) setContent(body fyne.CanvasObject) {
	update := func() {
		bg := canvas.NewRectangle(backgroundColor)
		// Don't set a minimum size - let content determine layout naturally
		if body == nil {
			s.window.SetContent(bg)
			return
		}
		// Wrap content with mouse button handler
		wrapped := newMouseButtonHandler(container.NewMax(bg, body), s)
		s.window.SetContent(wrapped)
	}

	// Use async Do() instead of DoAndWait() to avoid deadlock when called from main goroutine
	fyne.Do(update)
}

// showErrorWithCopy displays an error dialog with a "Copy Error" button
func (s *appState) showErrorWithCopy(title string, err error) {
	errMsg := err.Error()

	// Create error message label
	errorLabel := widget.NewLabel(errMsg)
	errorLabel.Wrapping = fyne.TextWrapWord

	// Create copy button
	copyBtn := widget.NewButton("Copy Error", func() {
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

func (s *appState) showMainMenu() {
	s.stopPreview()
	s.stopPlayer()
	s.active = ""

	// Track navigation history
	s.pushNavigationHistory("mainmenu")

	// Convert Module slice to ui.ModuleInfo slice
	var mods []ui.ModuleInfo
	for _, m := range modulesList {
		mods = append(mods, ui.ModuleInfo{
			ID:       m.ID,
			Label:    m.Label,
			Color:    m.Color,
			Category: m.Category,
			Enabled:  m.ID == "convert" || m.ID == "compare" || m.ID == "inspect" || m.ID == "merge" || m.ID == "thumb" || m.ID == "player" || m.ID == "filters" || m.ID == "upscale", // Enabled modules
		})
	}

	titleColor := utils.MustHex("#4CE870")

	// Get queue stats - show completed jobs out of total
	var queueCompleted, queueTotal int
	if s.jobQueue != nil {
		_, _, completed, _, _ := s.jobQueue.Stats()
		queueCompleted = completed
		queueTotal = len(s.jobQueue.List())
	}

	menu := ui.BuildMainMenu(mods, s.showModule, s.handleModuleDrop, s.showQueue, func() {
		logDir := getLogsDir()
		_ = os.MkdirAll(logDir, 0o755)

		openFolderBtn := widget.NewButton("Open Logs Folder", func() {
			if err := openFolder(logDir); err != nil {
				dialog.ShowError(fmt.Errorf("failed to open logs folder: %w", err), s.window)
			}
		})

		appLogPath := strings.TrimSpace(logging.FilePath())
		viewAppLogBtn := widget.NewButton("View App Log", func() {
			if appLogPath == "" {
				dialog.ShowInformation("No Log", "No app log file found yet.", s.window)
				return
			}
			s.openLogViewer("App Log", appLogPath, false)
		})
		if appLogPath == "" {
			viewAppLogBtn.Disable()
		}

		infoLabel := widget.NewLabel(fmt.Sprintf("Logs directory: %s", logDir))
		infoLabel.Wrapping = fyne.TextWrapWord

		logOptions := container.NewVBox(
			infoLabel,
			openFolderBtn,
			viewAppLogBtn,
		)
		dialog.ShowCustom("Logs", "Close", logOptions, s.window)
	}, s.showBenchmark, s.showBenchmarkHistory, titleColor, queueColor, textColor, queueCompleted, queueTotal)

	// Update stats bar
	s.updateStatsBar()

	// Footer with version info and a small About/Support button
	versionLabel := widget.NewLabel(fmt.Sprintf("VideoTools %s", appVersion))
	versionLabel.Alignment = fyne.TextAlignLeading
	aboutBtn := widget.NewButton("About / Support", func() {
		s.showAbout()
	})
	aboutBtn.Importance = widget.LowImportance
	footer := container.NewBorder(nil, nil, nil, aboutBtn, versionLabel)

	// Add stats bar at the bottom of the menu
	content := container.NewBorder(
		nil,                                   // top
		container.NewVBox(s.statsBar, footer), // bottom
		nil,                                   // left
		nil,                                   // right
		container.NewPadded(menu),             // center
	)

	s.setContent(content)
}

func (s *appState) showQueue() {
	s.stopPreview()
	s.stopPlayer()
	s.lastModule = s.active
	s.active = "queue"
	s.refreshQueueView()
}

// refreshQueueView rebuilds the queue UI while preserving scroll position and inline active conversion.
func (s *appState) refreshQueueView() {
	// Preserve current scroll offset if we already have a view
	if s.queueScroll != nil {
		s.queueOffset = s.queueScroll.Offset
	}

	jobs := s.jobQueue.List()
	// If a direct conversion is running but not represented in the queue, surface it as a pseudo job.
	if s.convertBusy {
		in := filepath.Base(s.convertActiveIn)
		if in == "" && s.source != nil {
			in = filepath.Base(s.source.Path)
		}
		out := filepath.Base(s.convertActiveOut)
		jobs = append([]*queue.Job{{
			ID:          "active-convert",
			Type:        queue.JobTypeConvert,
			Status:      queue.JobStatusRunning,
			Title:       fmt.Sprintf("Direct convert: %s", in),
			Description: fmt.Sprintf("Output: %s", out),
			Progress:    s.convertProgress,
			Config: map[string]interface{}{
				"fps":   s.convertFPS,
				"speed": s.convertSpeed,
				"eta":   s.convertETA,
			},
		}}, jobs...)
	}

	view, scroll := ui.BuildQueueView(
		jobs,
		func() { // onBack
			if s.lastModule != "" && s.lastModule != "queue" && s.lastModule != "menu" {
				s.showModule(s.lastModule)
			} else {
				s.showMainMenu()
			}
		},
		func(id string) { // onPause
			if err := s.jobQueue.Pause(id); err != nil {
				logging.Debug(logging.CatSystem, "failed to pause job: %v", err)
			}
			s.refreshQueueView() // Refresh
		},
		func(id string) { // onResume
			if err := s.jobQueue.Resume(id); err != nil {
				logging.Debug(logging.CatSystem, "failed to resume job: %v", err)
			}
			s.refreshQueueView() // Refresh
		},
		func(id string) { // onCancel
			if err := s.jobQueue.Cancel(id); err != nil {
				logging.Debug(logging.CatSystem, "failed to cancel job: %v", err)
			}
			s.refreshQueueView() // Refresh
		},
		func(id string) { // onRemove
			if err := s.jobQueue.Remove(id); err != nil {
				logging.Debug(logging.CatSystem, "failed to remove job: %v", err)
			}
			s.refreshQueueView() // Refresh
		},
		func(id string) { // onMoveUp
			if err := s.jobQueue.MoveUp(id); err != nil {
				logging.Debug(logging.CatSystem, "failed to move job up: %v", err)
			}
			s.refreshQueueView() // Refresh
		},
		func(id string) { // onMoveDown
			if err := s.jobQueue.MoveDown(id); err != nil {
				logging.Debug(logging.CatSystem, "failed to move job down: %v", err)
			}
			s.refreshQueueView() // Refresh
		},
		func() { // onPauseAll
			s.jobQueue.PauseAll()
			s.refreshQueueView()
		},
		func() { // onResumeAll
			s.jobQueue.ResumeAll()
			s.refreshQueueView()
		},
		func() { // onStart
			s.jobQueue.ResumeAll()
			s.refreshQueueView()
		},
		func() { // onClear
			s.jobQueue.Clear()
			s.clearVideo()

			// If queue is now empty, return to previous module
			if len(s.jobQueue.List()) == 0 {
				if s.lastModule != "" && s.lastModule != "queue" {
					s.showModule(s.lastModule)
				} else {
					s.showMainMenu()
				}
			} else {
				s.refreshQueueView() // Refresh if jobs remain
			}
		},
		func() { // onClearAll
			s.jobQueue.ClearAll()
			s.clearVideo()
			// Return to previous module or main menu
			if s.lastModule != "" && s.lastModule != "queue" {
				s.showModule(s.lastModule)
			} else {
				s.showMainMenu()
			}
		},
		func(id string) { // onCopyError
			job, err := s.jobQueue.Get(id)
			if err != nil {
				logging.Debug(logging.CatSystem, "copy error text failed: %v", err)
				return
			}
			text := strings.TrimSpace(job.Error)
			if text == "" {
				text = fmt.Sprintf("%s: no error message available", job.Title)
			}
			s.window.Clipboard().SetContent(text)
		},
		func(id string) { // onViewLog
			job, err := s.jobQueue.Get(id)
			if err != nil {
				logging.Debug(logging.CatSystem, "view log failed: %v", err)
				return
			}
			path := strings.TrimSpace(job.LogPath)
			if path == "" {
				dialog.ShowInformation("No Log", "No log path recorded for this job.", s.window)
				return
			}
			data, err := os.ReadFile(path)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to read log: %w", err), s.window)
				return
			}
			text := widget.NewMultiLineEntry()
			text.SetText(string(data))
			text.Wrapping = fyne.TextWrapWord
			text.Disable()
			dialog.ShowCustom("Conversion Log", "Close", container.NewVScroll(text), s.window)
		},
		utils.MustHex("#4CE870"), // titleColor
		gridColor,                // bgColor
		textColor,                // textColor
	)

	// Restore scroll offset
	s.queueScroll = scroll
	if s.queueScroll != nil && s.active == "queue" {
		// Use ScrollTo instead of directly setting Offset to prevent rubber banding
		// Defer to allow UI to settle first
		go func() {
			time.Sleep(50 * time.Millisecond)
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				if s.queueScroll != nil {
					s.queueScroll.Offset = s.queueOffset
					s.queueScroll.Refresh()
				}
			}, false)
		}()
	}

	s.setContent(container.NewPadded(view))
}

// addConvertToQueue adds a conversion job to the queue
func (s *appState) addConvertToQueue() error {
	if s.source == nil {
		return fmt.Errorf("no video loaded")
	}

	src := s.source
	outputBase := s.resolveOutputBase(src, true)
	s.convert.OutputBase = outputBase
	cfg := s.convert
	cfg.OutputBase = outputBase

	outDir := filepath.Dir(src.Path)
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
				s.convert.VideoCodec = friendly
			}
		}
	}

	// Create job config map
	config := map[string]interface{}{
		"inputPath":         src.Path,
		"outputPath":        outPath,
		"outputBase":        cfg.OutputBase,
		"selectedFormat":    cfg.SelectedFormat,
		"quality":           cfg.Quality,
		"mode":              cfg.Mode,
		"videoCodec":        adjustedCodec,
		"encoderPreset":     cfg.EncoderPreset,
		"crf":               cfg.CRF,
		"bitrateMode":       cfg.BitrateMode,
		"bitratePreset":     cfg.BitratePreset,
		"videoBitrate":      cfg.VideoBitrate,
		"targetFileSize":    cfg.TargetFileSize,
		"targetResolution":  cfg.TargetResolution,
		"frameRate":         cfg.FrameRate,
		"pixelFormat":       cfg.PixelFormat,
		"hardwareAccel":     cfg.HardwareAccel,
		"twoPass":           cfg.TwoPass,
		"h264Profile":       cfg.H264Profile,
		"h264Level":         cfg.H264Level,
		"deinterlace":       cfg.Deinterlace,
		"deinterlaceMethod": cfg.DeinterlaceMethod,
		"autoCrop":          cfg.AutoCrop,
		"cropWidth":         cfg.CropWidth,
		"cropHeight":        cfg.CropHeight,
		"cropX":             cfg.CropX,
		"cropY":             cfg.CropY,
		"flipHorizontal":    cfg.FlipHorizontal,
		"flipVertical":      cfg.FlipVertical,
		"rotation":          cfg.Rotation,
		"audioCodec":        cfg.AudioCodec,
		"audioBitrate":      cfg.AudioBitrate,
		"audioChannels":     cfg.AudioChannels,
		"audioSampleRate":   cfg.AudioSampleRate,
		"normalizeAudio":    cfg.NormalizeAudio,
		"inverseTelecine":   cfg.InverseTelecine,
		"coverArtPath":      cfg.CoverArtPath,
		"aspectHandling":    cfg.AspectHandling,
		"outputAspect":      cfg.OutputAspect,
		"sourceWidth":       src.Width,
		"sourceHeight":      src.Height,
		"sourceDuration":    src.Duration,
		"fieldOrder":        src.FieldOrder,
		"autoCompare":       s.autoCompare, // Include auto-compare flag
	}

	job := &queue.Job{
		Type:        queue.JobTypeConvert,
		Title:       fmt.Sprintf("Convert %s", filepath.Base(src.Path)),
		Description: fmt.Sprintf("Output: %s → %s", utils.ShortenMiddle(filepath.Base(src.Path), 40), utils.ShortenMiddle(filepath.Base(outPath), 40)),
		InputFile:   src.Path,
		OutputFile:  outPath,
		Config:      config,
	}

	s.jobQueue.Add(job)
	logging.Debug(logging.CatSystem, "added convert job to queue: %s", job.ID)

	return nil
}

func (s *appState) showBenchmark() {
	s.stopPreview()
	s.stopPlayer()
	s.active = "benchmark"

	// Create benchmark suite
	tmpDir := filepath.Join(os.TempDir(), "videotools-benchmark")
	_ = os.MkdirAll(tmpDir, 0o755)

	suite := benchmark.NewSuite(platformConfig.FFmpegPath, tmpDir)

	// Build progress view
	view := ui.BuildBenchmarkProgressView(
		func() {
			// Cancel benchmark
			s.showMainMenu()
		},
		utils.MustHex("#4CE870"),
		utils.MustHex("#1E1E1E"),
		utils.MustHex("#FFFFFF"),
	)

	s.setContent(view.GetContainer())

	// Run benchmark in background
	go func() {
		ctx := context.Background()

		// Generate test video
		view.UpdateProgress(0, 100, "Generating test video", "")
		testPath, err := suite.GenerateTestVideo(ctx, 30)
		if err != nil {
			logging.Debug(logging.CatSystem, "failed to generate test video: %v", err)
			dialog.ShowError(fmt.Errorf("failed to generate test video: %w", err), s.window)
			s.showMainMenu()
			return
		}
		logging.Debug(logging.CatSystem, "generated test video: %s", testPath)

		// Detect available encoders
		availableEncoders := s.detectHardwareEncoders()
		logging.Debug(logging.CatSystem, "detected %d available encoders", len(availableEncoders))

		// Set up progress callback
		suite.Progress = func(current, total int, encoder, preset string) {
			logging.Debug(logging.CatSystem, "benchmark progress: %d/%d testing %s (%s)", current, total, encoder, preset)
			view.UpdateProgress(current, total, encoder, preset)
		}

		// Run benchmark suite
		err = suite.RunFullSuite(ctx, availableEncoders)
		if err != nil {
			logging.Debug(logging.CatSystem, "benchmark failed: %v", err)
			dialog.ShowError(fmt.Errorf("benchmark failed: %w", err), s.window)
			s.showMainMenu()
			return
		}

		// Display results as they come in
		for _, result := range suite.Results {
			view.AddResult(result)
		}

		// Mark complete
		view.SetComplete()

		// Get recommendation
		encoder, preset, rec := suite.GetRecommendation()

		// Save benchmark run to history
		if err := s.saveBenchmarkRun(suite.Results, encoder, preset, rec.FPS); err != nil {
			logging.Debug(logging.CatSystem, "failed to save benchmark run: %v", err)
		}

		if encoder != "" {
			logging.Debug(logging.CatSystem, "benchmark recommendation: %s (preset: %s) - %.1f FPS", encoder, preset, rec.FPS)

			// Show results dialog with option to apply
			go func() {
				allResults := suite.Results // Show all results, not just top 10
				resultsView := ui.BuildBenchmarkResultsView(
					allResults,
					rec,
					func() {
						// Apply recommended settings
						s.applyBenchmarkRecommendation(encoder, preset)
						s.showMainMenu()
					},
					func() {
						// Close without applying
						s.showMainMenu()
					},
					utils.MustHex("#4CE870"),
					utils.MustHex("#1E1E1E"),
					utils.MustHex("#FFFFFF"),
				)

				s.setContent(resultsView)
			}()
		}

		// Clean up test video
		os.Remove(testPath)
	}()
}

func (s *appState) detectHardwareEncoders() []string {
	var available []string

	// Always add software encoders
	available = append(available, "libx264", "libx265")

	// Check for hardware encoders by trying to get codec info
	encodersToCheck := []string{
		"h264_nvenc", "hevc_nvenc", // NVIDIA
		"h264_qsv", "hevc_qsv", // Intel QuickSync
		"h264_amf", "hevc_amf", // AMD AMF
		"h264_videotoolbox", // Apple VideoToolbox
	}

	for _, encoder := range encodersToCheck {
		cmd := exec.Command(platformConfig.FFmpegPath, "-hide_banner", "-encoders")
		output, err := cmd.CombinedOutput()
		if err == nil && strings.Contains(string(output), encoder) {
			available = append(available, encoder)
			logging.Debug(logging.CatSystem, "detected available encoder: %s", encoder)
		}
	}

	return available
}

func (s *appState) saveBenchmarkRun(results []benchmark.Result, encoder, preset string, fps float64) error {
	// Map encoder to hardware acceleration setting
	var hwAccel string
	switch {
	case strings.Contains(encoder, "nvenc"):
		hwAccel = "nvenc"
	case strings.Contains(encoder, "qsv"):
		hwAccel = "qsv"
	case strings.Contains(encoder, "amf"):
		hwAccel = "amf"
	case strings.Contains(encoder, "videotoolbox"):
		hwAccel = "videotoolbox"
	default:
		hwAccel = "none"
	}

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
		RecommendedEncoder: encoder,
		RecommendedPreset:  preset,
		RecommendedHWAccel: hwAccel,
		RecommendedFPS:     fps,
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

	logging.Debug(logging.CatSystem, "saved benchmark run: encoder=%s preset=%s fps=%.1f results=%d", encoder, preset, fps, len(results))
	return nil
}

func (s *appState) applyBenchmarkRecommendation(encoder, preset string) {
	logging.Debug(logging.CatSystem, "applied benchmark recommendation: encoder=%s preset=%s", encoder, preset)

	// Map encoder to hardware acceleration setting
	hwAccel := "none"
	switch {
	case strings.Contains(encoder, "nvenc"):
		hwAccel = "nvenc"
	case strings.Contains(encoder, "qsv"):
		hwAccel = "qsv"
	case strings.Contains(encoder, "amf"):
		hwAccel = "amf"
	case strings.Contains(encoder, "videotoolbox"):
		hwAccel = "videotoolbox"
	}

	// Map encoder to friendly codec to align Convert defaults
	if codec := friendlyCodecFromPreset(encoder); codec != "" {
		s.convert.VideoCodec = codec
	}
	s.convert.EncoderPreset = preset
	s.convert.HardwareAccel = hwAccel
	s.persistConvertConfig()

	dialog.ShowInformation("Benchmark Settings Applied",
		fmt.Sprintf("Applied recommended defaults:\n\nEncoder: %s\nPreset: %s\nHardware Accel: %s\n\nThese are now set as your Convert defaults.",
			encoder, preset, hwAccel), s.window)
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
			s.showMainMenu,
			utils.MustHex("#4CE870"),
			utils.MustHex("#1E1E1E"),
			utils.MustHex("#FFFFFF"),
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
			RecommendedEncoder: run.RecommendedEncoder,
			RecommendedPreset:  run.RecommendedPreset,
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
				Preset:  run.RecommendedPreset,
				FPS:     run.RecommendedFPS,
				Score:   run.RecommendedFPS,
			}

			resultsView := ui.BuildBenchmarkResultsView(
				run.Results,
				rec,
				func() {
					// Apply this recommendation
					s.applyBenchmarkRecommendation(run.RecommendedEncoder, run.RecommendedPreset)
					s.showBenchmarkHistory()
				},
				func() {
					// Back to history
					s.showBenchmarkHistory()
				},
				utils.MustHex("#4CE870"),
				utils.MustHex("#1E1E1E"),
				utils.MustHex("#FFFFFF"),
			)

			s.setContent(resultsView)
		},
		s.showMainMenu,
		utils.MustHex("#4CE870"),
		utils.MustHex("#1E1E1E"),
		utils.MustHex("#FFFFFF"),
	)

	s.setContent(view)
}

func (s *appState) showModule(id string) {
	// Track navigation history
	s.pushNavigationHistory(id)

	switch id {
	case "convert":
		s.showConvertView(nil)
	case "merge":
		s.showMergeView()
	case "compare":
		s.showCompareView()
	case "inspect":
		s.showInspectView()
	case "thumb":
		s.showThumbView()
	case "player":
		s.showPlayerView()
	case "filters":
		s.showFiltersView()
	case "upscale":
		s.showUpscaleView()
	case "mainmenu":
		s.showMainMenu()
	default:
		logging.Debug(logging.CatUI, "UI module %s not wired yet", id)
	}
}

func (s *appState) handleModuleDrop(moduleID string, items []fyne.URI) {
	logging.Debug(logging.CatModule, "handleModuleDrop called: moduleID=%s itemCount=%d", moduleID, len(items))
	if len(items) == 0 {
		logging.Debug(logging.CatModule, "handleModuleDrop: no items to process")
		return
	}

	// Collect all video files (including from folders)
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
			videos := s.findVideoFiles(path)
			videoPaths = append(videoPaths, videos...)
		} else if s.isVideoFile(path) {
			videoPaths = append(videoPaths, path)
		}
	}

	logging.Debug(logging.CatModule, "found %d video files to process", len(videoPaths))

	if len(videoPaths) == 0 {
		return
	}

	// If convert module and multiple files, add all to queue
	if moduleID == "convert" && len(videoPaths) > 1 {
		go s.batchAddToQueue(videoPaths)
		return
	}

	// If compare module, load up to 2 videos into compare slots
	if moduleID == "compare" {
		go func() {
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
					dialog.ShowInformation("Compare Videos",
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

				// Auto-run interlacing detection in background
				go func() {
					detector := interlace.NewDetector(platformConfig.FFmpegPath, platformConfig.FFprobePath)
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
			}, false)
		}()
		return
	}

	if moduleID == "merge" {
		go func() {
			var clips []mergeClip
			for _, p := range videoPaths {
				src, err := probeVideo(p)
				if err != nil {
					logging.Debug(logging.CatModule, "failed to probe merge clip %s: %v", p, err)
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
					dialog.ShowInformation("Merge", "No valid video files found.", s.window)
				}, false)
				return
			}
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				s.mergeClips = append(s.mergeClips, clips...)
				if len(s.mergeClips) >= 2 && strings.TrimSpace(s.mergeOutput) == "" {
					first := filepath.Dir(s.mergeClips[0].Path)
					s.mergeOutput = filepath.Join(first, "merged.mkv")
				}
				s.showMergeView()
			}, false)
		}()
		return
	}

	// If thumb module, load video into thumb slot
	if moduleID == "thumb" {
		path := videoPaths[0]
		go func() {
			src, err := probeVideo(path)
			if err != nil {
				logging.Debug(logging.CatModule, "failed to load video for thumb: %v", err)
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					dialog.ShowError(fmt.Errorf("failed to load video: %w", err), s.window)
				}, false)
				return
			}

			// Update state and show module (with small delay to allow flash animation)
			time.Sleep(350 * time.Millisecond)
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				s.thumbFile = src
				s.showModule(moduleID)
				logging.Debug(logging.CatModule, "loaded video for thumb module")
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
	videoExts := []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".mpg", ".mpeg", ".3gp", ".ogv"}
	for _, videoExt := range videoExts {
		if ext == videoExt {
			return true
		}
	}
	return false
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
			"sourceWidth":       src.Width,
			"sourceHeight":      src.Height,
			"sourceBitrate":     src.Bitrate,
			"sourceDuration":    src.Duration,
			"fieldOrder":        src.FieldOrder,
		}

		job := &queue.Job{
			Type:        queue.JobTypeConvert,
			Title:       fmt.Sprintf("Convert %s", filepath.Base(path)),
			Description: fmt.Sprintf("Output: %s → %s", filepath.Base(path), filepath.Base(outPath)),
			InputFile:   path,
			OutputFile:  outPath,
			Config:      config,
		}

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
			dialog.ShowInformation("Batch Add", msg, s.window)
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

func (s *appState) showConvertView(file *videoSource) {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "convert"
	if file != nil {
		s.source = file
	}
	if s.source == nil {
		s.convert.OutputBase = "converted"
		s.convert.CoverArtPath = ""
		s.convert.AspectHandling = "Auto"
	}
	if !s.convert.AspectUserSet || s.convert.OutputAspect == "" {
		s.convert.OutputAspect = "Source"
		s.convert.AspectUserSet = false
	}
	s.setContent(buildConvertView(s, s.source))
}

func (s *appState) showCompareView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "compare"
	s.setContent(buildCompareView(s))
}

func (s *appState) showInspectView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "inspect"
	s.setContent(buildInspectView(s))
}

func (s *appState) showThumbView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "thumb"
	s.setContent(buildThumbView(s))
}

func (s *appState) showPlayerView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "player"
	s.setContent(buildPlayerView(s))
}

func (s *appState) showFiltersView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "filters"
	s.setContent(buildFiltersView(s))
}

func (s *appState) showUpscaleView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "upscale"
	s.setContent(buildUpscaleView(s))
}

func (s *appState) showMergeView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "merge"

	mergeColor := moduleColor("merge")

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

	backBtn := widget.NewButton("< MERGE", func() {
		s.showMainMenu()
	})
	backBtn.Importance = widget.LowImportance

	queueBtn := widget.NewButton("View Queue", func() {
		s.showQueue()
	})
	s.queueBtn = queueBtn
	s.updateQueueButtonLabel()

	topBar := ui.TintedBar(mergeColor, container.NewHBox(backBtn, layout.NewSpacer(), queueBtn))
	bottomBar := moduleFooter(mergeColor, layout.NewSpacer(), s.statsBar)

	listBox := container.NewVBox()
	var addFiles func([]string)
	var addQueueBtn *widget.Button
	var runNowBtn *widget.Button

	var buildList func()
	buildList = func() {
		listBox.Objects = nil
		if len(s.mergeClips) == 0 {
			emptyLabel := widget.NewLabel("Add at least two clips to merge.")
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
				label := widget.NewLabel(utils.ShortenMiddle(name, 50))
				chEntry := widget.NewEntry()
				chEntry.SetText(c.Chapter)
				chEntry.SetPlaceHolder(fmt.Sprintf("Part %d", i+1))
				chEntry.OnChanged = func(val string) {
					s.mergeClips[idx].Chapter = val
				}
				upBtn := widget.NewButton("↑", func() {
					if idx > 0 {
						s.mergeClips[idx-1], s.mergeClips[idx] = s.mergeClips[idx], s.mergeClips[idx-1]
						buildList()
					}
				})
				downBtn := widget.NewButton("↓", func() {
					if idx < len(s.mergeClips)-1 {
						s.mergeClips[idx+1], s.mergeClips[idx] = s.mergeClips[idx], s.mergeClips[idx+1]
						buildList()
					}
				})
				delBtn := widget.NewButton("Remove", func() {
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
		for _, p := range paths {
			src, err := probeVideo(p)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to probe %s: %w", p, err), s.window)
				continue
			}
			s.mergeClips = append(s.mergeClips, mergeClip{
				Path:     p,
				Chapter:  strings.TrimSuffix(filepath.Base(p), filepath.Ext(p)),
				Duration: src.Duration,
			})
		}
		if len(s.mergeClips) >= 2 && s.mergeOutput == "" {
			first := filepath.Dir(s.mergeClips[0].Path)
			s.mergeOutput = filepath.Join(first, "merged.mkv")
		}
		buildList()
	}

	addBtn := widget.NewButton("Add Files…", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			path := reader.URI().Path()
			reader.Close()
			addFiles([]string{path})
		}, s.window)
	})
	clearBtn := widget.NewButton("Clear", func() {
		s.mergeClips = nil
		buildList()
	})

	// Helper to get file extension for format
	getExtForFormat := func(format string) string {
		switch {
		case strings.HasPrefix(format, "dvd"):
			return ".mpg"
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
		"High Quality MP4 (H.264)":    "mp4-h264",
		"High Quality MP4 (H.265)":    "mp4-h265",
		"DVD Format":                  "dvd",
		"Blu-ray Format":              "bd-h264",
	}
	// Maintain order for dropdown
	formatKeys := []string{
		"Fast Merge (No Re-encoding)",
		"Lossless MKV (Best Quality)",
		"High Quality MP4 (H.264)",
		"High Quality MP4 (H.265)",
		"DVD Format",
		"Blu-ray Format",
	}

	keepAllCheck := widget.NewCheck("Keep all audio/subtitle tracks", func(v bool) {
		s.mergeKeepAll = v
	})
	keepAllCheck.SetChecked(s.mergeKeepAll)

	chapterCheck := widget.NewCheck("Create chapters from each clip", func(v bool) {
		s.mergeChapters = v
	})
	chapterCheck.SetChecked(s.mergeChapters)

	// Create output entry widget first so it can be referenced in callbacks
	outputEntry := widget.NewEntry()
	outputEntry.SetPlaceHolder("merged output path")
	outputEntry.SetText(s.mergeOutput)
	outputEntry.OnChanged = func(val string) {
		s.mergeOutput = val
	}

	// Helper to update output path extension (requires outputEntry to exist)
	updateOutputExt := func() {
		if s.mergeOutput == "" {
			return
		}
		currentExt := filepath.Ext(s.mergeOutput)
		correctExt := getExtForFormat(s.mergeFormat)
		if currentExt != correctExt {
			s.mergeOutput = strings.TrimSuffix(s.mergeOutput, currentExt) + correctExt
			outputEntry.SetText(s.mergeOutput)
		}
	}

	// DVD-specific options
	dvdRegionSelect := widget.NewSelect([]string{"NTSC", "PAL"}, func(val string) {
		s.mergeDVDRegion = val
	})
	dvdRegionSelect.SetSelected(s.mergeDVDRegion)

	dvdAspectSelect := widget.NewSelect([]string{"16:9", "4:3"}, func(val string) {
		s.mergeDVDAspect = val
	})
	dvdAspectSelect.SetSelected(s.mergeDVDAspect)

	dvdOptionsRow := container.NewHBox(
		widget.NewLabel("Region:"),
		dvdRegionSelect,
		widget.NewLabel("Aspect:"),
		dvdAspectSelect,
	)

	// Container for DVD options (can be shown/hidden)
	dvdOptionsContainer := container.NewVBox(dvdOptionsRow)

	// Create format selector (after outputEntry and updateOutputExt are defined)
	formatSelect := widget.NewSelect(formatKeys, func(val string) {
		s.mergeFormat = formatMap[val]

		// Show/hide DVD options based on selection
		if s.mergeFormat == "dvd" {
			dvdOptionsContainer.Show()
		} else {
			dvdOptionsContainer.Hide()
		}

		// Set default output path if not set
		if s.mergeOutput == "" && len(s.mergeClips) > 0 {
			dir := filepath.Dir(s.mergeClips[0].Path)
			ext := getExtForFormat(s.mergeFormat)
			basename := "merged"
			if strings.HasPrefix(s.mergeFormat, "dvd") || s.mergeFormat == "dvd" {
				basename = "merged-dvd"
			} else if strings.HasPrefix(s.mergeFormat, "bd") {
				basename = "merged-bd"
			} else if s.mergeFormat == "mkv-lossless" {
				basename = "merged-lossless"
			}
			s.mergeOutput = filepath.Join(dir, basename+ext)
			outputEntry.SetText(s.mergeOutput)
		} else {
			// Update extension of existing path
			updateOutputExt()
		}
	})
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
	frameRateSelect := widget.NewSelect([]string{"Source", "23.976", "24", "25", "29.97", "30", "50", "59.94", "60"}, func(val string) {
		s.mergeFrameRate = val
	})
	frameRateSelect.SetSelected(s.mergeFrameRate)

	motionInterpCheck := widget.NewCheck("Use Motion Interpolation (slower, smoother)", func(checked bool) {
		s.mergeMotionInterpolation = checked
	})
	motionInterpCheck.SetChecked(s.mergeMotionInterpolation)

	frameRateRow := container.NewVBox(
		widget.NewLabel("Frame Rate"),
		frameRateSelect,
		motionInterpCheck,
	)

	browseOut := widget.NewButton("Browse", func() {
		dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err != nil || writer == nil {
				return
			}
			s.mergeOutput = writer.URI().Path()
			outputEntry.SetText(s.mergeOutput)
			writer.Close()
		}, s.window)
	})

	addQueueBtn = widget.NewButton("Add Merge to Queue", func() {
		if err := s.addMergeToQueue(false); err != nil {
			dialog.ShowError(err, s.window)
			return
		}
		dialog.ShowInformation("Queue", "Merge job added to queue.", s.window)
		if s.jobQueue != nil && !s.jobQueue.IsRunning() {
			s.jobQueue.Start()
		}
	})
	runNowBtn = widget.NewButton("Merge Now", func() {
		if err := s.addMergeToQueue(true); err != nil {
			dialog.ShowError(err, s.window)
			return
		}
		if s.jobQueue != nil && !s.jobQueue.IsRunning() {
			s.jobQueue.Start()
		}
		dialog.ShowInformation("Merge", "Merge started! Track progress in Job Queue.", s.window)
	})
	if len(s.mergeClips) < 2 {
		addQueueBtn.Disable()
		runNowBtn.Disable()
	}

	listScroll := container.NewVScroll(listBox)

	// Use border layout so the list expands to fill available vertical space
	leftTop := container.NewVBox(
		widget.NewLabelWithStyle("Clips to Merge", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
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

	right := container.NewVBox(
		widget.NewLabelWithStyle("Output Options", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Format"),
		formatSelect,
		dvdOptionsContainer,
		widget.NewSeparator(),
		frameRateRow,
		widget.NewSeparator(),
		keepAllCheck,
		chapterCheck,
		widget.NewSeparator(),
		widget.NewLabel("Output Path"),
		container.NewBorder(nil, nil, nil, browseOut, outputEntry),
		widget.NewSeparator(),
		container.NewHBox(addQueueBtn, runNowBtn),
	)

	content := container.NewHSplit(left, right)
	content.Offset = 0.55
	s.setContent(container.NewBorder(topBar, bottomBar, nil, nil, container.NewPadded(content)))

	buildList()
	s.updateStatsBar()
}

func (s *appState) addMergeToQueue(startNow bool) error {
	if len(s.mergeClips) < 2 {
		return fmt.Errorf("add at least two clips")
	}
	if strings.TrimSpace(s.mergeOutput) == "" {
		firstDir := filepath.Dir(s.mergeClips[0].Path)
		s.mergeOutput = filepath.Join(firstDir, "merged.mkv")
	}

	// Ensure output path has correct extension for selected format
	currentExt := filepath.Ext(s.mergeOutput)
	var correctExt string
	switch {
	case strings.HasPrefix(s.mergeFormat, "dvd"):
		correctExt = ".mpg"
	case strings.HasPrefix(s.mergeFormat, "mkv"), strings.HasPrefix(s.mergeFormat, "bd"):
		correctExt = ".mkv"
	case strings.HasPrefix(s.mergeFormat, "mp4"):
		correctExt = ".mp4"
	default:
		correctExt = ".mkv"
	}

	// Auto-fix extension if missing or wrong
	if currentExt == "" {
		s.mergeOutput += correctExt
	} else if currentExt != correctExt {
		s.mergeOutput = strings.TrimSuffix(s.mergeOutput, currentExt) + correctExt
	}
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
		"outputPath":             s.mergeOutput,
		"dvdRegion":              s.mergeDVDRegion,
		"dvdAspect":              s.mergeDVDAspect,
		"frameRate":              s.mergeFrameRate,
		"useMotionInterpolation": s.mergeMotionInterpolation,
	}

	job := &queue.Job{
		Type:        queue.JobTypeMerge,
		Title:       fmt.Sprintf("Merge %d clips", len(clips)),
		Description: fmt.Sprintf("Output: %s", utils.ShortenMiddle(filepath.Base(s.mergeOutput), 40)),
		InputFile:   clips[0]["path"].(string),
		OutputFile:  s.mergeOutput,
		Config:      config,
	}
	s.jobQueue.Add(job)
	if startNow && s.jobQueue != nil && !s.jobQueue.IsRunning() {
		s.jobQueue.Start()
	}
	return nil
}

func (s *appState) showCompareFullscreen() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "compare-fullscreen"
	s.setContent(buildCompareFullscreenView(s))
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
		return fmt.Errorf("trim jobs not yet implemented")
	case queue.JobTypeFilter:
		return fmt.Errorf("filter jobs not yet implemented")
	case queue.JobTypeUpscale:
		return s.executeUpscaleJob(ctx, job, progressCallback)
	case queue.JobTypeAudio:
		return fmt.Errorf("audio jobs not yet implemented")
	case queue.JobTypeThumb:
		return s.executeThumbJob(ctx, job, progressCallback)
	case queue.JobTypeSnippet:
		return s.executeSnippetJob(ctx, job, progressCallback)
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

	tmpDir := os.TempDir()
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
		"-f", "concat",
		"-safe", "0",
		"-i", listFile.Name(),
	}
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
		args = append(args, "-c", "copy")
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
	cmd := exec.CommandContext(ctx, platformConfig.FFmpegPath, args...)
	utils.ApplyNoWindow(cmd)
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

	if err := cmd.Start(); err != nil {
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

	// Hardware acceleration for decoding
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

	// Video filters
	var vf []string

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

	// Scaling/Resolution
	targetResolution, _ := cfg["targetResolution"].(string)
	if targetResolution != "" && targetResolution != "Source" {
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
		}
		if scaleFilter != "" {
			vf = append(vf, scaleFilter)
		}
	}

	// Aspect ratio conversion
	sourceWidth, _ := cfg["sourceWidth"].(int)
	sourceHeight, _ := cfg["sourceHeight"].(int)
	// Get source bitrate if present
	sourceBitrate := 0
	if v, ok := cfg["sourceBitrate"].(float64); ok {
		sourceBitrate = int(v)
	}
	srcAspect := utils.AspectRatioFloat(sourceWidth, sourceHeight)
	outputAspect, _ := cfg["outputAspect"].(string)
	aspectHandling, _ := cfg["aspectHandling"].(string)

	// Create temp source for aspect calculation
	tempSrc := &videoSource{Width: sourceWidth, Height: sourceHeight}
	targetAspect := resolveTargetAspect(outputAspect, tempSrc)
	if targetAspect > 0 && srcAspect > 0 && !utils.RatiosApproxEqual(targetAspect, srcAspect, 0.01) {
		vf = append(vf, aspectFilters(targetAspect, aspectHandling)...)
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
		args = append(args, "-c:v", "copy")
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
				if actualCodec == "libx264" || actualCodec == "libx265" || actualCodec == "libvpx-vp9" {
					args = append(args, "-crf", crfStr)
				}
			} else if bitrateMode == "CBR" {
				if videoBitrate, _ := cfg["videoBitrate"].(string); videoBitrate != "" {
					args = append(args, "-b:v", videoBitrate, "-minrate", videoBitrate, "-maxrate", videoBitrate, "-bufsize", videoBitrate)
				} else {
					vb := defaultBitrate(videoCodec, sourceWidth, sourceBitrate)
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
			if encoderPreset, _ := cfg["encoderPreset"].(string); encoderPreset != "" && (actualCodec == "libx264" || actualCodec == "libx265") {
				args = append(args, "-preset", encoderPreset)
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
	if audioCodec == "Copy" && !isDVD {
		args = append(args, "-c:a", "copy")
	} else {
		var actualAudioCodec string
		if isDVD {
			// DVD requires AC-3 audio
			actualAudioCodec = "ac3"
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
	args = append(args, "-map_chapters", "0", "-map_metadata", "0")

	// Copy subtitle streams by default (don't re-encode)
	args = append(args, "-c:s", "copy")

	if strings.EqualFold(selectedFormat.Ext, ".mp4") || strings.EqualFold(selectedFormat.Ext, ".mov") {
		args = append(args, "-movflags", "+faststart")
	}

	// Note: We no longer use -target because it forces resolution changes.
	// DVD-specific parameters are set manually in the video codec section below.

	// Fix VFR/desync issues - regenerate timestamps and enforce CFR
	args = append(args, "-fflags", "+genpts")
	frameRateStr, _ := cfg["frameRate"].(string)
	sourceDuration, _ := cfg["sourceDuration"].(float64)
	if frameRateStr != "" && frameRateStr != "Source" {
		args = append(args, "-r", frameRateStr)
	} else if sourceDuration > 0 {
		// Calculate approximate source frame rate if available
		args = append(args, "-r", "30") // Safe default
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
	cmd := exec.CommandContext(ctx, platformConfig.FFmpegPath, args...)
	utils.ApplyNoWindow(cmd)
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

	if err := cmd.Start(); err != nil {
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

	return nil
}

func (s *appState) executeThumbJob(ctx context.Context, job *queue.Job, progressCallback func(float64)) error {
	cfg := job.Config
	inputPath := cfg["inputPath"].(string)
	outputDir := cfg["outputDir"].(string)
	count := int(cfg["count"].(float64))
	width := int(cfg["width"].(float64))
	contactSheet := cfg["contactSheet"].(bool)
	columns := int(cfg["columns"].(float64))
	rows := int(cfg["rows"].(float64))

	if progressCallback != nil {
		progressCallback(0)
	}

	generator := thumbnail.NewGenerator(platformConfig.FFmpegPath)
	config := thumbnail.Config{
		VideoPath:     inputPath,
		OutputDir:     outputDir,
		Count:         count,
		Width:         width,
		Format:        "jpg",
		Quality:       85,
		ContactSheet:  contactSheet,
		Columns:       columns,
		Rows:          rows,
		ShowTimestamp: false, // Disabled to avoid font issues
		ShowMetadata:  contactSheet,
	}

	result, err := generator.Generate(ctx, config)
	if err != nil {
		return fmt.Errorf("thumbnail generation failed: %w", err)
	}

	logging.Debug(logging.CatSystem, "generated %d thumbnails", len(result.Thumbnails))

	if progressCallback != nil {
		progressCallback(1)
	}

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

	// Probe video to get duration
	src, err := probeVideo(inputPath)
	if err != nil {
		return err
	}

	// Calculate start time centered on midpoint
	halfLength := float64(snippetLength) / 2.0
	center := math.Max(0, src.Duration/2-halfLength)
	start := fmt.Sprintf("%.2f", center)

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
			args = append(args, "-c:a", "wmav2")
			if conv.AudioBitrate != "" {
				args = append(args, "-b:a", conv.AudioBitrate)
			} else {
				args = append(args, "-b:a", "192k")
			}
		} else {
			// For non-WMV: use source codec or fallback to H.264
			videoCodec := src.VideoCodec
			if videoCodec == "" || strings.Contains(strings.ToLower(videoCodec), "wmv") {
				videoCodec = "libx264"
			}

			args = append(args, "-c:v", videoCodec)

			// Apply encoder preset if supported codec
			if strings.Contains(strings.ToLower(videoCodec), "264") ||
				strings.Contains(strings.ToLower(videoCodec), "265") {
				if conv.EncoderPreset != "" {
					args = append(args, "-preset", conv.EncoderPreset)
				} else {
					args = append(args, "-preset", "slow")
				}
				if conv.CRF != "" {
					args = append(args, "-crf", conv.CRF)
				} else {
					args = append(args, "-crf", "18")
				}
			}

			// Audio codec
			audioCodec := src.AudioCodec
			if audioCodec == "" || strings.Contains(strings.ToLower(audioCodec), "wmav") {
				audioCodec = "aac"
			}

			args = append(args, "-c:a", audioCodec)
			if strings.Contains(strings.ToLower(audioCodec), "aac") ||
				strings.Contains(strings.ToLower(audioCodec), "mp3") {
				if conv.AudioBitrate != "" {
					args = append(args, "-b:a", conv.AudioBitrate)
				} else {
					args = append(args, "-b:a", "192k")
				}
			}
		}

		args = append(args, "-y", "-hide_banner", "-loglevel", "error")
	} else {
		// Conversion format mode: Use configured conversion settings
		// This allows previewing what the final converted output will look like
		conv := s.convert

		args = []string{
			"-y",
			"-hide_banner",
			"-loglevel", "error",
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
			args = append(args, "-c:v", "libsvtav1")
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
			// Fallback to AAC
			args = append(args, "-c:a", "aac", "-b:a", "192k")
		}

		// Common args appended after progress flags
	}

	// Add progress output for live updates (stdout) and finish with output path
	args = append(args, "-progress", "pipe:1", "-nostats", outputPath)

	if progressCallback != nil {
		progressCallback(0)
	}

	logFile, logPath, _ := createConversionLog(inputPath, outputPath, args)
	cmd := exec.CommandContext(ctx, platformConfig.FFmpegPath, args...)
	utils.ApplyNoWindow(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("snippet stdout pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
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
	preserveAR := true
	if v, ok := cfg["preserveAR"].(bool); ok {
		preserveAR = v
	}
	// useAI := cfg["useAI"].(bool) // TODO: Implement AI upscaling in future
	applyFilters := cfg["applyFilters"].(bool)
	frameRate, _ := cfg["frameRate"].(string)
	useMotionInterp, _ := cfg["useMotionInterpolation"].(bool)

	if progressCallback != nil {
		progressCallback(0)
	}

	// Build filter chain
	var filters []string

	// Add filters from Filters module if requested
	if applyFilters {
		if filterChain, ok := cfg["filterChain"].([]interface{}); ok {
			for _, f := range filterChain {
				if filterStr, ok := f.(string); ok {
					filters = append(filters, filterStr)
				}
			}
		}
	}

	// Add scale filter (preserve aspect by default)
	scaleFilter := buildUpscaleFilter(targetWidth, targetHeight, method, preserveAR)
	filters = append(filters, scaleFilter)

	// Add frame rate conversion if requested
	if frameRate != "" && frameRate != "Source" {
		if useMotionInterp {
			// Use motion interpolation for smooth frame rate changes
			filters = append(filters, fmt.Sprintf("minterpolate=fps=%s:mi_mode=mci:mc_mode=aobmc:me_mode=bidir:vsbmc=1", frameRate))
		} else {
			// Simple frame rate change (duplicates/drops frames)
			filters = append(filters, "fps="+frameRate)
		}
	}

	// Combine filters
	var vfilter string
	if len(filters) > 0 {
		vfilter = strings.Join(filters, ",")
	}

	// Build FFmpeg command
	args := []string{
		"-y",
		"-hide_banner",
		"-i", inputPath,
	}

	// Add video filter if we have any
	if vfilter != "" {
		args = append(args, "-vf", vfilter)
	}

	// Use lossless MKV by default for upscales; copy audio
	args = append(args,
		"-c:v", "libx264",
		"-preset", "slow",
		"-crf", "0", // lossless
		"-pix_fmt", "yuv420p",
		"-c:a", "copy",
		outputPath,
	)

	logFile, logPath, _ := createConversionLog(inputPath, outputPath, args)
	cmd := exec.CommandContext(ctx, platformConfig.FFmpegPath, args...)
	utils.ApplyNoWindow(cmd)

	// Create progress reader for stderr
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start upscale: %w", err)
	}

	// Parse progress from FFmpeg stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if logFile != nil {
				fmt.Fprintln(logFile, line)
			}

			// Parse progress from "time=00:01:23.45"
			if strings.Contains(line, "time=") {
				// Get duration from job config
				if duration, ok := cfg["duration"].(float64); ok && duration > 0 {
					// Extract time from FFmpeg output
					if idx := strings.Index(line, "time="); idx != -1 {
						timeStr := line[idx+5:]
						if spaceIdx := strings.Index(timeStr, " "); spaceIdx != -1 {
							timeStr = timeStr[:spaceIdx]
						}

						// Parse time string (HH:MM:SS.ms)
						var h, m int
						var s float64
						if _, err := fmt.Sscanf(timeStr, "%d:%d:%f", &h, &m, &s); err == nil {
							currentTime := float64(h*3600+m*60) + s
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
	if s.playSess != nil {
		s.playSess.Stop()
		s.playSess = nil
	}
	if s.player != nil {
		s.player.Stop()
	}
	s.stopProgressLoop()
	s.playerReady = false
	s.playerPaused = true
}

func main() {
	logging.Init()
	defer logging.Close()

	flag.Parse()
	logging.SetDebug(*debugFlag || os.Getenv("VIDEOTOOLS_DEBUG") != "")
	logging.Debug(logging.CatSystem, "starting VideoTools prototype at %s", time.Now().Format(time.RFC3339))

	// Detect platform and configure paths
	platformConfig = DetectPlatform()
	if platformConfig.FFmpegPath == "ffmpeg" || platformConfig.FFmpegPath == "ffmpeg.exe" {
		logging.Debug(logging.CatSystem, "WARNING: FFmpeg not found in expected locations, assuming it's in PATH")
	}

	// Set paths in convert package
	convert.FFmpegPath = platformConfig.FFmpegPath
	convert.FFprobePath = platformConfig.FFprobePath

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

func runGUI() {
	// Initialize UI colors
	ui.SetColors(gridColor, textColor)

	a := app.NewWithID("com.leaktechnologies.videotools")

	// Always start with a clean slate: wipe any persisted app storage (queue or otherwise)
	if root := a.Storage().RootURI(); root != nil && root.Scheme() == "file" {
		_ = os.RemoveAll(root.Path())
	}

	a.Settings().SetTheme(&ui.MonoTheme{})
	logging.Debug(logging.CatUI, "created fyne app: %#v", a)
	w := a.NewWindow("VideoTools")
	if icon := utils.LoadAppIcon(); icon != nil {
		a.SetIcon(icon)
		w.SetIcon(icon)
		logging.Debug(logging.CatUI, "app icon loaded and applied")
	} else {
		logging.Debug(logging.CatUI, "app icon not found; continuing without custom icon")
	}
	// Adaptive window sizing for professional cross-resolution support
	w.SetFixedSize(false) // Allow manual resizing and maximizing

	// Use conservative default size that fits on small laptop screens (1280x768)
	// Window can be maximized by user using window manager controls
	w.Resize(fyne.NewSize(1200, 700))
	w.CenterOnScreen()

	logging.Debug(logging.CatUI, "window initialized at 1200x700 (fits 1280x768+ screens), manual resizing enabled")

	state := &appState{
		window: w,
		convert: convertConfig{
			OutputBase:       "converted",
			SelectedFormat:   formatOptions[0],
			Quality:          "Standard (CRF 23)",
			Mode:             "Simple",
			UseAutoNaming:    false,
			AutoNameTemplate: "<actress> - <studio> - <scene>",

			// Video encoding defaults
			VideoCodec:        "H.264",
			EncoderPreset:     "medium",
			CRF:               "", // Empty means use Quality preset
			BitrateMode:       "CRF",
			BitratePreset:     "Manual",
			VideoBitrate:      "5000k",
			TargetResolution:  "Source",
			FrameRate:         "Source",
			PixelFormat:       "yuv420p",
			HardwareAccel:     "auto",
			TwoPass:           false,
			H264Profile:       "main",
			H264Level:         "4.0",
			Deinterlace:       "Auto",
			DeinterlaceMethod: "bwdif",
			AutoCrop:          false,

			// Audio encoding defaults
			AudioCodec:      "AAC",
			AudioBitrate:    "192k",
			AudioChannels:   "Source",
			AudioSampleRate: "Source",
			NormalizeAudio:  false,

			// Other defaults
			InverseTelecine:  true,
			InverseAutoNotes: "Default smoothing for interlaced footage.",
			OutputAspect:     "Source",
			AspectHandling:   "Auto",
			AspectUserSet:    false,
		},
		mergeChapters: true,
		player:        player.New(),
		playerVolume:  100,
		lastVolume:    100,
		playerMuted:   false,
		playerPaused:  true,
	}

	if cfg, err := loadPersistedConvertConfig(); err == nil {
		state.convert = cfg
		// Ensure FrameRate defaults to Source if not explicitly set
		if state.convert.FrameRate == "" {
			state.convert.FrameRate = "Source"
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		logging.Debug(logging.CatSystem, "failed to load persisted convert config: %v", err)
	}

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
			state.updateStatsBar()
			state.updateQueueButtonLabel()
			if state.active == "queue" {
				state.refreshQueueView()
			}
		}, false)
	})

	defer state.shutdown()
	w.SetOnDropped(func(pos fyne.Position, items []fyne.URI) {
		state.handleDrop(pos, items)
	})
	state.showMainMenu()
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
	case "thumb":
		modules.HandleThumb(cmdArgs)
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
	fmt.Println("  videotools thumb <args>")
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
	if err := s.addConvertToQueue(); err != nil {
		dialog.ShowError(err, s.window)
	} else {
		dialog.ShowInformation("Queue", "Job added to queue!", s.window)
		// Auto-start queue if not already running
		if s.jobQueue != nil && !s.jobQueue.IsRunning() && !s.convertBusy {
			s.jobQueue.Start()
			logging.Debug(logging.CatUI, "queue auto-started after adding job")
		}
	}
}

func (s *appState) executeConversion() {
	// Add job to queue and start immediately
	if err := s.addConvertToQueue(); err != nil {
		dialog.ShowError(err, s.window)
		return
	}

	// Start the queue if not already running
	if s.jobQueue != nil && !s.jobQueue.IsRunning() {
		s.jobQueue.Start()
		logging.Debug(logging.CatSystem, "started queue from Convert Now")
	}

	// Clear the loaded video from convert module
	s.clearVideo()

	// Show success message
	dialog.ShowInformation("Convert", "Conversion started! View progress in Job Queue.", s.window)
}

func buildConvertView(state *appState, src *videoSource) fyne.CanvasObject {
	convertColor := moduleColor("convert")

	back := widget.NewButton("< CONVERT", func() {
		state.showMainMenu()
	})
	back.Importance = widget.LowImportance

	// Navigation buttons for multiple loaded videos
	var navButtons fyne.CanvasObject
	if len(state.loadedVideos) > 1 {
		prevBtn := widget.NewButton("◀ Prev", func() {
			state.prevVideo()
		})
		nextBtn := widget.NewButton("Next ▶", func() {
			state.nextVideo()
		})
		videoCounter := widget.NewLabel(fmt.Sprintf("Video %d of %d", state.currentIndex+1, len(state.loadedVideos)))
		navButtons = container.NewHBox(prevBtn, videoCounter, nextBtn)
	} else {
		navButtons = container.NewHBox()
	}

	// Queue button to view queue
	queueBtn := widget.NewButton("View Queue", func() {
		state.showQueue()
	})
	state.queueBtn = queueBtn
	state.updateQueueButtonLabel()

	backBar := ui.TintedBar(convertColor, container.NewHBox(back, layout.NewSpacer(), navButtons, layout.NewSpacer(), queueBtn))

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
			coverDisplay.SetText("Cover Art: " + state.convert.CoverLabel())
		}
		if updateMetaCover != nil {
			updateMetaCover()
		}
	}

	// Make panel sizes responsive with modest minimums to avoid forcing the window beyond the screen
	// Use a smaller minimum size to allow window to be more flexible
	// The video pane will scale to fit available space
	videoPanel := buildVideoPane(state, fyne.NewSize(320, 180), src, updateCover)
	metaPanel, metaCoverUpdate := buildMetadataPanel(state, src, fyne.NewSize(0, 200))
	updateMetaCover = metaCoverUpdate

	var formatLabels []string
	for _, opt := range formatOptions {
		formatLabels = append(formatLabels, opt.Label)
	}
	outputHint := widget.NewLabel(fmt.Sprintf("Output file: %s", state.convert.OutputFile()))
	outputHint.Wrapping = fyne.TextWrapWord

	// DVD-specific aspect ratio selector (only shown for DVD formats)
	dvdAspectSelect := widget.NewSelect([]string{"4:3", "16:9"}, func(value string) {
		logging.Debug(logging.CatUI, "DVD aspect set to %s", value)
		state.convert.OutputAspect = value
	})
	dvdAspectSelect.SetSelected("16:9")
	dvdAspectLabel := widget.NewLabelWithStyle("DVD Aspect Ratio", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	// DVD info label showing specs based on format selected
	dvdInfoLabel := widget.NewLabel("")
	dvdInfoLabel.Wrapping = fyne.TextWrapWord
	dvdInfoLabel.Alignment = fyne.TextAlignLeading

	dvdAspectBox := container.NewVBox(dvdAspectLabel, dvdAspectSelect, dvdInfoLabel)
	dvdAspectBox.Hide() // Hidden by default

	// Placeholder for updateDVDOptions - will be defined after resolution/framerate selects are created
	var updateDVDOptions func()

	// Forward declarations for encoding controls (used in reset/update callbacks)
	var (
		bitrateModeSelect    *widget.Select
		bitratePresetSelect  *widget.Select
		crfEntry             *widget.Entry
		videoBitrateEntry    *widget.Entry
		targetFileSizeSelect *widget.Select
		targetFileSizeEntry  *widget.Entry
		qualitySelectSimple  *widget.Select
		qualitySelectAdv     *widget.Select
		qualitySectionSimple fyne.CanvasObject
		qualitySectionAdv    fyne.CanvasObject
		simpleBitrateSelect  *widget.Select
	)
	var (
		updateEncodingControls  func()
		updateQualityVisibility func()
	)

	qualityOptions := []string{
		"Draft (CRF 28)",
		"Standard (CRF 23)",
		"Balanced (CRF 20)",
		"High (CRF 18)",
		"Near-Lossless (CRF 16)",
		"Lossless",
	}
	var syncingQuality bool

	qualitySelectSimple = widget.NewSelect(qualityOptions, func(value string) {
		if syncingQuality {
			return
		}
		syncingQuality = true
		logging.Debug(logging.CatUI, "quality preset %s (simple)", value)
		state.convert.Quality = value
		if qualitySelectAdv != nil {
			qualitySelectAdv.SetSelected(value)
		}
		if updateEncodingControls != nil {
			updateEncodingControls()
		}
		syncingQuality = false
	})

	qualitySelectAdv = widget.NewSelect(qualityOptions, func(value string) {
		if syncingQuality {
			return
		}
		syncingQuality = true
		logging.Debug(logging.CatUI, "quality preset %s (advanced)", value)
		state.convert.Quality = value
		if qualitySelectSimple != nil {
			qualitySelectSimple.SetSelected(value)
		}
		if updateEncodingControls != nil {
			updateEncodingControls()
		}
		syncingQuality = false
	})

	if !slices.Contains(qualityOptions, state.convert.Quality) {
		state.convert.Quality = "Standard (CRF 23)"
	}
	qualitySelectSimple.SetSelected(state.convert.Quality)
	qualitySelectAdv.SetSelected(state.convert.Quality)

	outputEntry := widget.NewEntry()
	outputEntry.SetText(state.convert.OutputBase)
	var updatingOutput bool
	outputEntry.OnChanged = func(val string) {
		if updatingOutput {
			return
		}
		state.convert.OutputBase = val
		outputHint.SetText(fmt.Sprintf("Output file: %s", state.convert.OutputFile()))
	}

	applyAutoName := func(force bool) {
		if !force && !state.convert.UseAutoNaming {
			return
		}
		newBase := state.resolveOutputBase(src, false)
		updatingOutput = true
		state.convert.OutputBase = newBase
		outputEntry.SetText(newBase)
		updatingOutput = false
		outputHint.SetText(fmt.Sprintf("Output file: %s", state.convert.OutputFile()))
	}

	autoNameCheck := widget.NewCheck("Auto-name from metadata", func(checked bool) {
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

	autoNameHint := widget.NewLabel("Tokens: <actress>, <studio>, <scene>, <title>, <series>, <date>, <filename>")
	autoNameHint.Wrapping = fyne.TextWrapWord

	if state.convert.UseAutoNaming {
		applyAutoName(true)
	}

	inverseCheck := widget.NewCheck("Smart Inverse Telecine", func(checked bool) {
		state.convert.InverseTelecine = checked
	})
	inverseCheck.Checked = state.convert.InverseTelecine
	inverseHint := widget.NewLabel(state.convert.InverseAutoNotes)

	// Interlacing Analysis Button (Simple Menu)
	var analyzeInterlaceBtn *widget.Button
	analyzeInterlaceBtn = widget.NewButton("Analyze Interlacing", func() {
		if src == nil {
			dialog.ShowInformation("Interlacing Analysis", "Load a video first.", state.window)
			return
		}
		go func() {
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				analyzeInterlaceBtn.SetText("Analyzing...")
				analyzeInterlaceBtn.Disable()
			}, false)

			detector := interlace.NewDetector(platformConfig.FFmpegPath, platformConfig.FFprobePath)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			result, err := detector.QuickAnalyze(ctx, src.Path)

			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				analyzeInterlaceBtn.SetText("Analyze Interlacing")
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

					dialog.ShowInformation("Interlacing Analysis Results", resultText, state.window)

					// Auto-update deinterlace setting
					if result.SuggestDeinterlace && state.convert.Deinterlace == "Off" {
						state.convert.Deinterlace = "Auto"
						inverseCheck.SetChecked(true)
					}
				}
			}, false)
		}()
	})
	analyzeInterlaceBtn.Importance = widget.MediumImportance

	// Auto-crop controls
	autoCropCheck := widget.NewCheck("Auto-Detect Black Bars", func(checked bool) {
		state.convert.AutoCrop = checked
		logging.Debug(logging.CatUI, "auto-crop set to %v", checked)
	})
	autoCropCheck.Checked = state.convert.AutoCrop

	var detectCropBtn *widget.Button
	detectCropBtn = widget.NewButton("Detect Crop", func() {
		if src == nil {
			dialog.ShowInformation("Auto-Crop", "Load a video first.", state.window)
			return
		}
		// Run detection in background
		go func() {
			detectCropBtn.SetText("Detecting...")
			detectCropBtn.Disable()
			defer func() {
				detectCropBtn.SetText("Detect Crop")
				detectCropBtn.Enable()
			}()

			crop := detectCrop(src.Path, src.Duration)
			if crop == nil {
				dialog.ShowInformation("Auto-Crop", "No black bars detected. Video is already fully cropped.", state.window)
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

			dialog.ShowConfirm("Auto-Crop Detection", message, func(apply bool) {
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

	autoCropHint := widget.NewLabel("Removes black bars to reduce file size (15-30% typical reduction)")
	autoCropHint.Wrapping = fyne.TextWrapWord

	// Flip and Rotation controls
	flipHorizontalCheck := widget.NewCheck("Flip Horizontal (Mirror)", func(checked bool) {
		state.convert.FlipHorizontal = checked
		logging.Debug(logging.CatUI, "flip horizontal set to %v", checked)
	})
	flipHorizontalCheck.Checked = state.convert.FlipHorizontal

	flipVerticalCheck := widget.NewCheck("Flip Vertical (Upside Down)", func(checked bool) {
		state.convert.FlipVertical = checked
		logging.Debug(logging.CatUI, "flip vertical set to %v", checked)
	})
	flipVerticalCheck.Checked = state.convert.FlipVertical

	rotationSelect := widget.NewSelect([]string{"0°", "90° CW", "180°", "270° CW"}, func(value string) {
		var rotation string
		switch value {
		case "0°":
			rotation = "0"
		case "90° CW":
			rotation = "90"
		case "180°":
			rotation = "180"
		case "270° CW":
			rotation = "270"
		}
		state.convert.Rotation = rotation
		logging.Debug(logging.CatUI, "rotation set to %s", rotation)
	})
	if state.convert.Rotation == "" {
		state.convert.Rotation = "0"
	}
	rotationMap := map[string]string{"0": "0°", "90": "90° CW", "180": "180°", "270": "270° CW"}
	if label, ok := rotationMap[state.convert.Rotation]; ok {
		rotationSelect.SetSelected(label)
	} else {
		rotationSelect.SetSelected("0°")
	}

	transformHint := widget.NewLabel("Apply flips and rotation to correct video orientation")
	transformHint.Wrapping = fyne.TextWrapWord

	aspectTargets := []string{"Source", "16:9", "4:3", "5:4", "5:3", "1:1", "9:16", "21:9"}
	targetAspectSelect := widget.NewSelect(aspectTargets, func(value string) {
		logging.Debug(logging.CatUI, "target aspect set to %s", value)
		state.convert.OutputAspect = value
		state.convert.AspectUserSet = true
	})
	if state.convert.OutputAspect == "" {
		state.convert.OutputAspect = "Source"
	}
	targetAspectSelect.SetSelected(state.convert.OutputAspect)
	targetAspectHint := widget.NewLabel("Pick desired output aspect (default Source).")

	aspectOptions := widget.NewRadioGroup([]string{"Auto", "Crop", "Letterbox", "Pillarbox", "Blur Fill", "Stretch"}, func(value string) {
		logging.Debug(logging.CatUI, "aspect handling set to %s", value)
		state.convert.AspectHandling = value
	})
	aspectOptions.Horizontal = false
	aspectOptions.Required = true
	aspectOptions.SetSelected(state.convert.AspectHandling)

	aspectOptions.SetSelected(state.convert.AspectHandling)

	backgroundHint := widget.NewLabel("Shown when aspect differs; choose padding/fill style.")
	aspectBox := container.NewVBox(
		widget.NewLabelWithStyle("Aspect Handling", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		aspectOptions,
		backgroundHint,
	)

	updateAspectBoxVisibility := func() {
		if src == nil {
			aspectBox.Hide()
			return
		}
		target := resolveTargetAspect(state.convert.OutputAspect, src)
		srcAspect := utils.AspectRatioFloat(src.Width, src.Height)
		if target == 0 || srcAspect == 0 || utils.RatiosApproxEqual(target, srcAspect, 0.01) {
			aspectBox.Hide()
		} else {
			aspectBox.Show()
		}
	}
	updateAspectBoxVisibility()
	targetAspectSelect.OnChanged = func(value string) {
		logging.Debug(logging.CatUI, "target aspect set to %s", value)
		state.convert.OutputAspect = value
		updateAspectBoxVisibility()
	}
	aspectOptions.OnChanged = func(value string) {
		logging.Debug(logging.CatUI, "aspect handling set to %s", value)
		state.convert.AspectHandling = value
	}

	// Cover art display on one line
	coverDisplay = widget.NewLabel("Cover Art: " + state.convert.CoverLabel())

	// Video Codec selection
	videoCodecSelect := widget.NewSelect([]string{"H.264", "H.265", "VP9", "AV1", "MPEG-2", "Copy"}, func(value string) {
		state.convert.VideoCodec = value
		logging.Debug(logging.CatUI, "video codec set to %s", value)
		if updateQualityVisibility != nil {
			updateQualityVisibility()
		}
	})
	videoCodecSelect.SetSelected(state.convert.VideoCodec)

	// Map format preset codec names to the UI-facing codec selector value
	mapFormatCodec := func(codec string) string {
		codec = strings.ToLower(codec)
		switch {
		case strings.Contains(codec, "265") || strings.Contains(codec, "hevc"):
			return "H.265"
		case strings.Contains(codec, "264"):
			return "H.264"
		case strings.Contains(codec, "vp9"):
			return "VP9"
		case strings.Contains(codec, "av1"):
			return "AV1"
		case strings.Contains(codec, "mpeg2"):
			return "MPEG-2"
		default:
			return state.convert.VideoCodec
		}
	}

	// Chapter warning label (shown when converting file with chapters to DVD)
	chapterWarningLabel := widget.NewLabel("⚠️  Chapters will be lost - DVD format doesn't support embedded chapters. Use MKV/MP4 to preserve chapters.")
	chapterWarningLabel.Wrapping = fyne.TextWrapWord
	chapterWarningLabel.TextStyle = fyne.TextStyle{Italic: true}
	var updateChapterWarning func()
	updateChapterWarning = func() {
		isDVD := state.convert.SelectedFormat.Ext == ".mpg"
		if src != nil && src.HasChapters && isDVD {
			chapterWarningLabel.Show()
		} else {
			chapterWarningLabel.Hide()
		}
	}

	formatSelect := widget.NewSelect(formatLabels, func(value string) {
		for _, opt := range formatOptions {
			if opt.Label == value {
				logging.Debug(logging.CatUI, "format set to %s", value)
				state.convert.SelectedFormat = opt
				outputHint.SetText(fmt.Sprintf("Output file: %s", state.convert.OutputFile()))
				if updateDVDOptions != nil {
					updateDVDOptions() // Show/hide DVD options and auto-set resolution
				}
				if updateChapterWarning != nil {
					updateChapterWarning() // Show/hide chapter warning
				}

				// Keep the codec selector aligned with the chosen format by default
				newCodec := mapFormatCodec(opt.VideoCodec)
				if newCodec != "" {
					state.convert.VideoCodec = newCodec
					videoCodecSelect.SetSelected(newCodec)
				}
				if updateQualityVisibility != nil {
					updateQualityVisibility()
				}
				break
			}
		}
	})
	formatSelect.SetSelected(state.convert.SelectedFormat.Label)
	updateChapterWarning() // Initial visibility

	if !state.convert.AspectUserSet {
		state.convert.OutputAspect = "Source"
	}

	// Encoder Preset with hint
	encoderPresetHint := widget.NewLabel("")
	encoderPresetHint.Wrapping = fyne.TextWrapWord

	updateEncoderPresetHint := func(preset string) {
		var hint string
		switch preset {
		case "ultrafast":
			hint = "⚡ Ultrafast: Fastest encoding, largest files (~10x faster than slow, ~30% larger files)"
		case "superfast":
			hint = "⚡ Superfast: Very fast encoding, large files (~7x faster than slow, ~20% larger files)"
		case "veryfast":
			hint = "⚡ Very Fast: Fast encoding, moderately large files (~5x faster than slow, ~15% larger files)"
		case "faster":
			hint = "⏩ Faster: Quick encoding, slightly large files (~3x faster than slow, ~10% larger files)"
		case "fast":
			hint = "⏩ Fast: Good speed, slightly large files (~2x faster than slow, ~5% larger files)"
		case "medium":
			hint = "⚖️ Medium (default): Balanced speed and quality (baseline for comparison)"
		case "slow":
			hint = "🎯 Slow (recommended): Best quality/size ratio (~2x slower than medium, ~5-10% smaller)"
		case "slower":
			hint = "🎯 Slower: Excellent compression (~3x slower than medium, ~10-15% smaller files)"
		case "veryslow":
			hint = "🐌 Very Slow: Maximum compression (~5x slower than medium, ~15-20% smaller files)"
		default:
			hint = ""
		}
		encoderPresetHint.SetText(hint)
	}

	encoderPresetSelect := widget.NewSelect([]string{"ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow", "slower", "veryslow"}, func(value string) {
		state.convert.EncoderPreset = value
		logging.Debug(logging.CatUI, "encoder preset set to %s", value)
		updateEncoderPresetHint(value)
	})
	encoderPresetSelect.SetSelected(state.convert.EncoderPreset)
	updateEncoderPresetHint(state.convert.EncoderPreset)

	// Simple mode preset dropdown
	simplePresetSelect := widget.NewSelect([]string{"ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow", "slower", "veryslow"}, func(value string) {
		state.convert.EncoderPreset = value
		logging.Debug(logging.CatUI, "simple preset set to %s", value)
		updateEncoderPresetHint(value)
	})
	simplePresetSelect.SetSelected(state.convert.EncoderPreset)

	// Settings management for batch operations
	settingsInfoLabel := widget.NewLabel("Settings persist across videos. Change them anytime to affect all subsequent videos.")
	settingsInfoLabel.Alignment = fyne.TextAlignCenter

	resetSettingsBtn := widget.NewButton("Reset to Defaults", func() {
		state.convert = convertConfig{
			SelectedFormat:   formatOptions[0],
			OutputBase:       "converted",
			Quality:          "Standard (CRF 23)",
			InverseTelecine:  false,
			OutputAspect:     "Source",
			AspectHandling:   "Auto",
			AspectUserSet:    false,
			VideoCodec:       "H.264",
			EncoderPreset:    "medium",
			BitrateMode:      "CRF",
			BitratePreset:    "Manual",
			CRF:              "",
			VideoBitrate:     "",
			TargetResolution: "Source",
			FrameRate:        "Source",
			PixelFormat:      "yuv420p",
			HardwareAccel:    "auto",
			AudioCodec:       "AAC",
			AudioBitrate:     "192k",
			AudioChannels:    "Source",
			UseAutoNaming:    false,
			AutoNameTemplate: "<actress> - <studio> - <scene>",
		}
		logging.Debug(logging.CatUI, "settings reset to defaults")
		formatSelect.SetSelected(state.convert.SelectedFormat.Label)
		videoCodecSelect.SetSelected(state.convert.VideoCodec)
		qualitySelectSimple.SetSelected(state.convert.Quality)
		qualitySelectAdv.SetSelected(state.convert.Quality)
		simplePresetSelect.SetSelected(state.convert.EncoderPreset)
		bitrateModeSelect.SetSelected(state.convert.BitrateMode)
		bitratePresetSelect.SetSelected(state.convert.BitratePreset)
		crfEntry.SetText(state.convert.CRF)
		videoBitrateEntry.SetText(state.convert.VideoBitrate)
		targetFileSizeSelect.SetSelected("Manual")
		targetFileSizeEntry.SetText(state.convert.TargetFileSize)
		autoNameCheck.SetChecked(state.convert.UseAutoNaming)
		autoNameTemplate.SetText(state.convert.AutoNameTemplate)
		outputEntry.SetText(state.convert.OutputBase)
		if updateEncodingControls != nil {
			updateEncodingControls()
		}
		if updateQualityVisibility != nil {
			updateQualityVisibility()
		}
	})
	resetSettingsBtn.Importance = widget.LowImportance

	settingsContent := container.NewVBox(
		settingsInfoLabel,
		resetSettingsBtn,
	)
	settingsContent.Hide()

	settingsVisible := false
	var toggleSettingsBtn *widget.Button
	toggleSettingsBtn = widget.NewButton("Show Batch Settings", func() {
		if settingsVisible {
			settingsContent.Hide()
			toggleSettingsBtn.SetText("Show Batch Settings")
		} else {
			settingsContent.Show()
			toggleSettingsBtn.SetText("Hide Batch Settings")
		}
		settingsVisible = !settingsVisible
	})
	toggleSettingsBtn.Importance = widget.LowImportance

	settingsBox := container.NewVBox(
		toggleSettingsBtn,
		settingsContent,
		widget.NewSeparator(),
	)

	// Bitrate Mode
	bitrateModeSelect = widget.NewSelect([]string{"CRF", "CBR", "VBR", "Target Size"}, func(value string) {
		state.convert.BitrateMode = value
		logging.Debug(logging.CatUI, "bitrate mode set to %s", value)
		if updateEncodingControls != nil {
			updateEncodingControls()
		}
	})
	bitrateModeSelect.SetSelected(state.convert.BitrateMode)

	// Manual CRF entry
	crfEntry = widget.NewEntry()
	crfEntry.SetPlaceHolder("Auto (from Quality preset)")
	crfEntry.SetText(state.convert.CRF)
	crfEntry.OnChanged = func(val string) {
		state.convert.CRF = val
	}

	// Video Bitrate entry (for CBR/VBR)
	videoBitrateEntry = widget.NewEntry()
	videoBitrateEntry.SetPlaceHolder("5000k")
	videoBitrateEntry.SetText(state.convert.VideoBitrate)
	videoBitrateEntry.OnChanged = func(val string) {
		state.convert.VideoBitrate = val
	}

	type bitratePreset struct {
		Label   string
		Bitrate string
		Codec   string
	}

	presets := []bitratePreset{
		{Label: "Manual", Bitrate: "", Codec: ""},
		{Label: "AV1 1080p - 1200k (smallest)", Bitrate: "1200k", Codec: "AV1"},
		{Label: "AV1 1080p - 1400k (sweet spot)", Bitrate: "1400k", Codec: "AV1"},
		{Label: "AV1 1080p - 1800k (headroom)", Bitrate: "1800k", Codec: "AV1"},
		{Label: "H.265 1080p - 2000k (balanced)", Bitrate: "2000k", Codec: "H.265"},
		{Label: "H.265 1080p - 2400k (noisy sources)", Bitrate: "2400k", Codec: "H.265"},
		{Label: "AV1 1440p - 2600k (balanced)", Bitrate: "2600k", Codec: "AV1"},
		{Label: "H.265 1440p - 3200k (balanced)", Bitrate: "3200k", Codec: "H.265"},
		{Label: "H.265 1440p - 4000k (noisy sources)", Bitrate: "4000k", Codec: "H.265"},
		{Label: "AV1 4K - 5M (balanced)", Bitrate: "5000k", Codec: "AV1"},
		{Label: "H.265 4K - 6M (balanced)", Bitrate: "6000k", Codec: "H.265"},
		{Label: "AV1 4K - 7M (archive)", Bitrate: "7000k", Codec: "AV1"},
		{Label: "H.265 4K - 9M (fast/Topaz)", Bitrate: "9000k", Codec: "H.265"},
	}

	bitratePresetLookup := make(map[string]bitratePreset)
	var bitratePresetLabels []string
	for _, p := range presets {
		bitratePresetLookup[p.Label] = p
		bitratePresetLabels = append(bitratePresetLabels, p.Label)
	}

	var applyBitratePreset func(string)

	bitratePresetSelect = widget.NewSelect(bitratePresetLabels, func(value string) {
		if applyBitratePreset != nil {
			applyBitratePreset(value)
		}
	})
	if state.convert.BitratePreset == "" || bitratePresetLookup[state.convert.BitratePreset].Label == "" {
		state.convert.BitratePreset = "Manual"
	}
	bitratePresetSelect.SetSelected(state.convert.BitratePreset)

	// Simple bitrate selector (shares presets)
	simpleBitrateSelect = widget.NewSelect(bitratePresetLabels, func(value string) {
		state.convert.BitratePreset = value
		if applyBitratePreset != nil {
			applyBitratePreset(value)
		}
	})
	simpleBitrateSelect.SetSelected(state.convert.BitratePreset)

	// Simple resolution selector (separate widget to avoid double-parent issues)
	resolutionSelectSimple := widget.NewSelect([]string{
		"Source", "360p", "480p", "540p", "720p", "1080p", "1440p", "4K", "8K",
		"2X (relative)", "4X (relative)",
		"NTSC (720×480)", "PAL (720×540)", "PAL (720×576)",
	}, func(value string) {
		state.convert.TargetResolution = value
		logging.Debug(logging.CatUI, "target resolution set to %s (simple)", value)
	})
	resolutionSelectSimple.SetSelected(state.convert.TargetResolution)

	// Simple aspect selector (separate widget)
	targetAspectSelectSimple := widget.NewSelect(aspectTargets, func(value string) {
		logging.Debug(logging.CatUI, "target aspect set to %s (simple)", value)
		state.convert.OutputAspect = value
		state.convert.AspectUserSet = true
		updateAspectBoxVisibility()
	})
	if state.convert.OutputAspect == "" {
		state.convert.OutputAspect = "Source"
	}
	targetAspectSelectSimple.SetSelected(state.convert.OutputAspect)

	// Target File Size with smart presets + manual entry
	targetFileSizeEntry = widget.NewEntry()
	targetFileSizeEntry.SetPlaceHolder("e.g., 25MB, 100MB, 8MB")

	updateTargetSizeOptions := func() {
		if src == nil {
			targetFileSizeSelect.Options = []string{"Manual", "25MB", "50MB", "100MB", "200MB", "500MB", "1GB"}
			return
		}

		// Calculate smart reduction options based on source file size
		srcPath := src.Path
		fileInfo, err := os.Stat(srcPath)
		if err != nil {
			targetFileSizeSelect.Options = []string{"Manual", "25MB", "50MB", "100MB", "200MB", "500MB", "1GB"}
			return
		}

		srcSize := fileInfo.Size()
		srcSizeMB := float64(srcSize) / (1024 * 1024)

		// Calculate smart reductions
		size33 := int(srcSizeMB * 0.67) // 33% reduction
		size50 := int(srcSizeMB * 0.50) // 50% reduction
		size75 := int(srcSizeMB * 0.25) // 75% reduction

		options := []string{"Manual"}

		if size75 > 5 {
			options = append(options, fmt.Sprintf("%dMB (75%% smaller)", size75))
		}
		if size50 > 10 {
			options = append(options, fmt.Sprintf("%dMB (50%% smaller)", size50))
		}
		if size33 > 15 {
			options = append(options, fmt.Sprintf("%dMB (33%% smaller)", size33))
		}

		// Add common sizes
		options = append(options, "25MB", "50MB", "100MB", "200MB", "500MB", "1GB")

		targetFileSizeSelect.Options = options
	}

	targetFileSizeSelect = widget.NewSelect([]string{"Manual", "25MB", "50MB", "100MB", "200MB", "500MB", "1GB"}, func(value string) {
		if value == "Manual" {
			targetFileSizeEntry.Show()
			targetFileSizeEntry.SetText(state.convert.TargetFileSize)
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
			targetFileSizeEntry.SetText(sizeStr)
			targetFileSizeEntry.Hide()
		}
		logging.Debug(logging.CatUI, "target file size set to %s", state.convert.TargetFileSize)
	})
	targetFileSizeSelect.SetSelected("Manual")
	updateTargetSizeOptions()

	targetFileSizeEntry.SetText(state.convert.TargetFileSize)
	targetFileSizeEntry.OnChanged = func(val string) {
		state.convert.TargetFileSize = val
	}

	encodingHint := widget.NewLabel("")
	encodingHint.Wrapping = fyne.TextWrapWord

	applyBitratePreset = func(label string) {
		preset, ok := bitratePresetLookup[label]
		if !ok {
			label = "Manual"
			preset = bitratePresetLookup[label]
		}

		state.convert.BitratePreset = label

		// Move to CBR for predictable output when a preset is chosen
		if preset.Bitrate != "" && state.convert.BitrateMode != "CBR" && state.convert.BitrateMode != "VBR" {
			state.convert.BitrateMode = "CBR"
			bitrateModeSelect.SetSelected("CBR")
		}

		if preset.Bitrate != "" {
			state.convert.VideoBitrate = preset.Bitrate
			videoBitrateEntry.SetText(preset.Bitrate)
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

	updateEncodingControls = func() {
		mode := state.convert.BitrateMode
		isLossless := state.convert.Quality == "Lossless"

		// Default: enable everything
		crfEntry.Enable()
		videoBitrateEntry.Enable()
		targetFileSizeEntry.Enable()
		targetFileSizeSelect.Enable()
		bitratePresetSelect.Enable()

		hint := ""

		if isLossless {
			// Lossless forces CRF 0; ignore bitrate/preset/target size to reduce confusion
			if mode != "CRF" {
				state.convert.BitrateMode = "CRF"
				bitrateModeSelect.SetSelected("CRF")
				mode = "CRF"
			}
			if crfEntry.Text != "0" {
				crfEntry.SetText("0")
			}
			state.convert.CRF = "0"
			crfEntry.Disable()
			videoBitrateEntry.Disable()
			targetFileSizeEntry.Disable()
			targetFileSizeSelect.Disable()
			bitratePresetSelect.Disable()
			hint = "Lossless forces CRF 0 for H.265/AV1; bitrate and target size are ignored."
		} else {
			switch mode {
			case "CRF", "":
				videoBitrateEntry.Disable()
				targetFileSizeEntry.Disable()
				targetFileSizeSelect.Disable()
				bitratePresetSelect.Disable()
				hint = "CRF mode uses the quality preset/CRF only."
			case "CBR", "VBR":
				crfEntry.Disable()
				targetFileSizeEntry.Disable()
				targetFileSizeSelect.Disable()
				hint = "Bitrate mode uses the value above; presets auto-fill common choices."
			case "Target Size":
				crfEntry.Disable()
				videoBitrateEntry.Disable()
				bitratePresetSelect.Disable()
				targetFileSizeEntry.Enable()
				targetFileSizeSelect.Enable()
				hint = "Target size calculates bitrate automatically from duration."
			}
		}

		encodingHint.SetText(hint)
	}
	updateEncodingControls()

	// Target Resolution (advanced)
	resolutionSelect := widget.NewSelect([]string{
		"Source", "720p", "1080p", "1440p", "4K", "8K",
		"2X (relative)", "4X (relative)",
		"NTSC (720×480)", "PAL (720×540)", "PAL (720×576)",
	}, func(value string) {
		state.convert.TargetResolution = value
		logging.Debug(logging.CatUI, "target resolution set to %s", value)
	})
	if state.convert.TargetResolution == "" {
		state.convert.TargetResolution = "Source"
	}
	resolutionSelect.SetSelected(state.convert.TargetResolution)

	// Frame Rate with hint
	frameRateHint := widget.NewLabel("")
	frameRateHint.Wrapping = fyne.TextWrapWord

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
			frameRateHint.SetText(fmt.Sprintf("Converting %.0f → %.0f fps: ~%.0f%% smaller file",
				sourceFPS, targetFPS, reduction))
		} else if targetFPS > sourceFPS {
			frameRateHint.SetText(fmt.Sprintf("⚠ Upscaling from %.0f to %.0f fps (may cause judder)",
				sourceFPS, targetFPS))
		} else {
			frameRateHint.SetText("")
		}
	}

	frameRateSelect := widget.NewSelect([]string{"Source", "23.976", "24", "25", "29.97", "30", "50", "59.94", "60"}, func(value string) {
		state.convert.FrameRate = value
		logging.Debug(logging.CatUI, "frame rate set to %s", value)
		updateFrameRateHint()
	})
	frameRateSelect.SetSelected(state.convert.FrameRate)
	updateFrameRateHint()

	// Motion Interpolation checkbox
	motionInterpCheck := widget.NewCheck("Use Motion Interpolation (slower, smoother frame rate changes)", func(checked bool) {
		state.convert.UseMotionInterpolation = checked
		logging.Debug(logging.CatUI, "motion interpolation set to %v", checked)
	})
	motionInterpCheck.Checked = state.convert.UseMotionInterpolation

	// Pixel Format
	pixelFormatSelect := widget.NewSelect([]string{"yuv420p", "yuv422p", "yuv444p"}, func(value string) {
		state.convert.PixelFormat = value
		logging.Debug(logging.CatUI, "pixel format set to %s", value)
	})
	pixelFormatSelect.SetSelected(state.convert.PixelFormat)

	// Hardware Acceleration with hint
	hwAccelHint := widget.NewLabel("Auto picks the best GPU path; if encode fails, switch to none (software).")
	hwAccelHint.Wrapping = fyne.TextWrapWord
	hwAccelSelect := widget.NewSelect([]string{"auto", "none", "nvenc", "amf", "vaapi", "qsv", "videotoolbox"}, func(value string) {
		state.convert.HardwareAccel = value
		logging.Debug(logging.CatUI, "hardware accel set to %s", value)
	})
	if state.convert.HardwareAccel == "" {
		state.convert.HardwareAccel = "auto"
	}
	hwAccelSelect.SetSelected(state.convert.HardwareAccel)

	// Two-Pass encoding
	twoPassCheck := widget.NewCheck("Enable Two-Pass Encoding", func(checked bool) {
		state.convert.TwoPass = checked
	})
	twoPassCheck.Checked = state.convert.TwoPass

	// Audio Codec
	audioCodecSelect := widget.NewSelect([]string{"AAC", "Opus", "MP3", "FLAC", "Copy"}, func(value string) {
		state.convert.AudioCodec = value
		logging.Debug(logging.CatUI, "audio codec set to %s", value)
	})
	audioCodecSelect.SetSelected(state.convert.AudioCodec)

	// Audio Bitrate
	audioBitrateSelect := widget.NewSelect([]string{"128k", "192k", "256k", "320k"}, func(value string) {
		state.convert.AudioBitrate = value
		logging.Debug(logging.CatUI, "audio bitrate set to %s", value)
	})
	audioBitrateSelect.SetSelected(state.convert.AudioBitrate)

	// Audio Channels
	audioChannelsSelect := widget.NewSelect([]string{"Source", "Mono", "Stereo", "5.1"}, func(value string) {
		state.convert.AudioChannels = value
		logging.Debug(logging.CatUI, "audio channels set to %s", value)
	})
	audioChannelsSelect.SetSelected(state.convert.AudioChannels)

	// Now define updateDVDOptions with access to resolution and framerate selects
	updateDVDOptions = func() {
		// Clear locks by default so non-DVD formats remain flexible
		resolutionSelectSimple.Enable()
		resolutionSelect.Enable()
		frameRateSelect.Enable()
		targetAspectSelectSimple.Enable()
		targetAspectSelect.Enable()
		pixelFormatSelect.Enable()
		hwAccelSelect.Enable()
		videoCodecSelect.Enable()
		videoBitrateEntry.Enable()
		bitrateModeSelect.Enable()
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
				dvdNotes = "NTSC DVD: 720×480 @ 29.97fps, MPEG-2 Video, AC-3 Stereo 48kHz (bitrate 8000k, 9000k max PS2-safe)"
				targetRes = "NTSC (720×480)"
				targetFPS = "29.97"
				dvdBitrate = "8000k"
			} else if strings.Contains(state.convert.SelectedFormat.Label, "PAL") {
				dvdNotes = "PAL DVD: 720×540 @ 25fps, MPEG-2 Video, AC-3 Stereo 48kHz (bitrate 8000k default, 9500k max)"
				targetRes = "PAL (720×540)"
				targetFPS = "25"
				dvdBitrate = "8000k"
			} else {
				dvdNotes = "DVD format selected"
				targetRes = "NTSC (720×480)"
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
					if ar := utils.AspectRatioFloat(src.Width, src.Height); ar > 0 && ar < 1.6 {
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
			state.convert.AspectUserSet = true
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
			videoCodecSelect.Disable()

			state.convert.VideoBitrate = dvdBitrate
			videoBitrateEntry.SetText(dvdBitrate)
			videoBitrateEntry.Disable()
			state.convert.BitrateMode = "CBR"
			bitrateModeSelect.SetSelected("CBR")
			bitrateModeSelect.Disable()
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
			targetFileSizeEntry.Hide()
			targetFileSizeSelect.Hide()
			crfEntry.Hide()

			dvdInfoLabel.SetText(fmt.Sprintf("%s\nLocked: resolution, frame rate, aspect, codec, pixel format, bitrate, and GPU toggles for DVD compliance.", dvdNotes))
		} else {
			dvdAspectBox.Hide()
			// Re-show hidden controls
			bitratePresetSelect.Show()
			simpleBitrateSelect.Show()
			targetFileSizeEntry.Show()
			targetFileSizeSelect.Show()
			crfEntry.Show()
		}
	}
	updateDVDOptions()

	qualitySectionSimple = container.NewVBox(
		widget.NewLabelWithStyle("═══ QUALITY ═══", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		qualitySelectSimple,
	)
	qualitySectionAdv = container.NewVBox(
		widget.NewLabelWithStyle("Quality Preset", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		qualitySelectAdv,
	)

	updateQualityVisibility = func() {
		hide := strings.Contains(strings.ToLower(state.convert.SelectedFormat.Label), "h.265") ||
			strings.EqualFold(state.convert.VideoCodec, "H.265")

		if qualitySectionSimple != nil {
			if hide {
				qualitySectionSimple.Hide()
			} else {
				qualitySectionSimple.Show()
			}
		}
		if qualitySectionAdv != nil {
			if hide {
				qualitySectionAdv.Hide()
			} else {
				qualitySectionAdv.Show()
			}
		}
	}

	// Simple mode options - minimal controls, aspect locked to Source
	simpleOptions := container.NewVBox(
		widget.NewLabelWithStyle("═══ OUTPUT ═══", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Format", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		formatSelect,
		chapterWarningLabel, // Warning when converting chapters to DVD
		dvdAspectBox,        // DVD options appear here when DVD format selected
		widget.NewLabelWithStyle("Output Name", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		outputEntry,
		outputHint,
		widget.NewSeparator(),
		qualitySectionSimple,
		widget.NewLabelWithStyle("Encoder Speed/Quality", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Choose slower for better compression, faster for speed"),
		widget.NewLabelWithStyle("Encoder Preset", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		simplePresetSelect,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Bitrate (simple presets)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		simpleBitrateSelect,
		widget.NewLabelWithStyle("Target Resolution", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		resolutionSelectSimple,
		widget.NewLabelWithStyle("Frame Rate", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		frameRateSelect,
		motionInterpCheck,
		widget.NewLabelWithStyle("Target Aspect Ratio", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		targetAspectSelectSimple,
		targetAspectHint,
		layout.NewSpacer(),
	)

	// Advanced mode options - full controls with organized sections
	advancedOptions := container.NewVBox(
		widget.NewLabelWithStyle("═══ OUTPUT ═══", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Format", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		formatSelect,
		chapterWarningLabel, // Warning when converting chapters to DVD
		dvdAspectBox,        // DVD options appear here when DVD format selected
		widget.NewLabelWithStyle("Output Name", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		outputEntry,
		outputHint,
		coverDisplay,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("═══ VIDEO ENCODING ═══", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Video Codec", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		videoCodecSelect,
		widget.NewLabelWithStyle("Encoder Preset (speed vs quality)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		encoderPresetSelect,
		encoderPresetHint,
		qualitySectionAdv,
		widget.NewLabelWithStyle("Bitrate Mode", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		bitrateModeSelect,
		widget.NewLabelWithStyle("Manual CRF (overrides Quality preset)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		crfEntry,
		widget.NewLabelWithStyle("Video Bitrate (for CBR/VBR)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		videoBitrateEntry,
		widget.NewLabelWithStyle("Recommended Bitrate Preset", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		bitratePresetSelect,
		encodingHint,
		widget.NewLabelWithStyle("Target File Size", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		targetFileSizeSelect,
		targetFileSizeEntry,
		widget.NewLabelWithStyle("Target Resolution", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		resolutionSelect,
		widget.NewLabelWithStyle("Frame Rate", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		frameRateSelect,
		frameRateHint,
		motionInterpCheck,
		widget.NewLabelWithStyle("Pixel Format", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		pixelFormatSelect,
		widget.NewLabelWithStyle("Hardware Acceleration", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		hwAccelSelect,
		hwAccelHint,
		twoPassCheck,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("═══ ASPECT RATIO ═══", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Target Aspect Ratio", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		targetAspectSelect,
		targetAspectHint,
		aspectBox,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("═══ AUDIO ENCODING ═══", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Audio Codec", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		audioCodecSelect,
		widget.NewLabelWithStyle("Audio Bitrate", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		audioBitrateSelect,
		widget.NewLabelWithStyle("Audio Channels", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		audioChannelsSelect,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("═══ AUTO-CROP ═══", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		autoCropCheck,
		detectCropBtn,
		autoCropHint,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("═══ VIDEO TRANSFORMATIONS ═══", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		flipHorizontalCheck,
		flipVerticalCheck,
		widget.NewLabelWithStyle("Rotation", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		rotationSelect,
		transformHint,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("═══ DEINTERLACING ═══", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		analyzeInterlaceBtn,
		inverseCheck,
		inverseHint,
		layout.NewSpacer(),
	)

	// Create tabs for Simple/Advanced modes
	// Wrap simple options with settings box at top
	simpleWithSettings := container.NewVBox(settingsBox, simpleOptions)

	// Keep Simple lightweight; wrap Advanced in its own scroll to avoid bloating MinSize.
	simpleScrollBox := simpleWithSettings
	advancedScrollBox := container.NewVScroll(advancedOptions)
	advancedScrollBox.SetMinSize(fyne.NewSize(0, 0))

	if updateQualityVisibility != nil {
		updateQualityVisibility()
	}

	tabs := container.NewAppTabs(
		container.NewTabItem("Simple", simpleScrollBox),
		container.NewTabItem("Advanced", advancedScrollBox),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	// Set initial tab based on mode
	if state.convert.Mode == "Advanced" {
		tabs.SelectIndex(1)
	}

	// Update mode when tab changes
	tabs.OnSelected = func(item *container.TabItem) {
		if item.Text == "Simple" {
			state.convert.Mode = "Simple"
			logging.Debug(logging.CatUI, "convert mode selected: Simple")
		} else {
			state.convert.Mode = "Advanced"
			logging.Debug(logging.CatUI, "convert mode selected: Advanced")
		}
	}

	optionsRect := canvas.NewRectangle(utils.MustHex("#13182B"))
	optionsRect.CornerRadius = 8
	optionsRect.StrokeColor = gridColor
	optionsRect.StrokeWidth = 1
	optionsPanel := container.NewMax(optionsRect, container.NewPadded(tabs))

	// Initialize snippet settings defaults
	if state.snippetLength == 0 {
		state.snippetLength = 20 // Default to 20 seconds
	}
	// Default to source format if not set
	if !state.snippetSourceFormat {
		state.snippetSourceFormat = true
	}

	// Snippet length configuration
	snippetLengthLabel := widget.NewLabel(fmt.Sprintf("Snippet Length: %d seconds", state.snippetLength))
	snippetLengthSlider := widget.NewSlider(5, 60)
	snippetLengthSlider.SetValue(float64(state.snippetLength))
	snippetLengthSlider.Step = 1
	snippetLengthSlider.OnChanged = func(value float64) {
		state.snippetLength = int(value)
		snippetLengthLabel.SetText(fmt.Sprintf("Snippet Length: %d seconds", state.snippetLength))
	}

	// Snippet output mode
	snippetModeLabel := widget.NewLabel("Snippet Output:")
	snippetModeCheck := widget.NewCheck("Match Source Format", func(checked bool) {
		state.snippetSourceFormat = checked
	})
	snippetModeCheck.SetChecked(state.snippetSourceFormat)
	snippetModeHint := widget.NewLabel("Unchecked = Use Conversion Settings")
	snippetModeHint.TextStyle = fyne.TextStyle{Italic: true}

	snippetConfigRow := container.NewVBox(
		snippetLengthLabel,
		snippetLengthSlider,
		widget.NewSeparator(),
		snippetModeLabel,
		snippetModeCheck,
		snippetModeHint,
	)

	snippetBtn := widget.NewButton("Generate Snippet", func() {
		if state.source == nil {
			dialog.ShowInformation("Snippet", "Load a video first.", state.window)
			return
		}
		if state.jobQueue == nil {
			dialog.ShowInformation("Queue", "Queue not initialized.", state.window)
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

		outName := fmt.Sprintf("%s-snippet-%d%s", strings.TrimSuffix(src.DisplayName, filepath.Ext(src.DisplayName)), time.Now().Unix(), ext)
		outPath := filepath.Join(filepath.Dir(src.Path), outName)

		modeDesc := "conversion settings"
		if state.snippetSourceFormat {
			modeDesc = "source format"
		}

		job := &queue.Job{
			Type:        queue.JobTypeSnippet,
			Title:       "Snippet: " + filepath.Base(src.Path),
			Description: fmt.Sprintf("%ds snippet centred on midpoint (%s)", state.snippetLength, modeDesc),
			InputFile:   src.Path,
			OutputFile:  outPath,
			Config: map[string]interface{}{
				"inputPath":       src.Path,
				"outputPath":      outPath,
				"snippetLength":   float64(state.snippetLength),
				"useSourceFormat": state.snippetSourceFormat,
			},
		}
		state.jobQueue.Add(job)
		if !state.jobQueue.IsRunning() {
			state.jobQueue.Start()
		}
		dialog.ShowInformation("Snippet", fmt.Sprintf("%ds snippet job added to queue.", state.snippetLength), state.window)
	})
	snippetBtn.Importance = widget.MediumImportance
	if src == nil {
		snippetBtn.Disable()
	}

	// Button to generate snippets for all loaded videos
	var snippetAllBtn *widget.Button
	if len(state.loadedVideos) > 1 {
		snippetAllBtn = widget.NewButton("Generate All Snippets", func() {
			if state.jobQueue == nil {
				dialog.ShowInformation("Queue", "Queue not initialized.", state.window)
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

				job := &queue.Job{
					Type:        queue.JobTypeSnippet,
					Title:       "Snippet: " + filepath.Base(src.Path),
					Description: fmt.Sprintf("%ds snippet centred on midpoint (%s)", state.snippetLength, modeDesc),
					InputFile:   src.Path,
					OutputFile:  outPath,
					Config: map[string]interface{}{
						"inputPath":       src.Path,
						"outputPath":      outPath,
						"snippetLength":   float64(state.snippetLength),
						"useSourceFormat": state.snippetSourceFormat,
					},
				}
				state.jobQueue.Add(job)
				jobsAdded++
			}

			if jobsAdded > 0 {
				if !state.jobQueue.IsRunning() {
					state.jobQueue.Start()
				}
				dialog.ShowInformation("Snippets",
					fmt.Sprintf("Added %d snippet jobs to queue.\nEach %ds long.", jobsAdded, state.snippetLength),
					state.window)
			}
		})
		snippetAllBtn.Importance = widget.HighImportance
	}

	snippetHint := widget.NewLabel("Creates a clip centred on the timeline midpoint.")

	var snippetRow fyne.CanvasObject
	if snippetAllBtn != nil {
		snippetRow = container.NewHBox(snippetBtn, snippetAllBtn, layout.NewSpacer(), snippetHint)
	} else {
		snippetRow = container.NewHBox(snippetBtn, layout.NewSpacer(), snippetHint)
	}

	// Stack video and metadata directly so metadata sits immediately under the player.
	leftColumn := container.NewVBox(videoPanel, metaPanel)

	// Split: left side (video + metadata VSplit) takes 55% | right side (options) takes 45%
	mainSplit := container.NewHSplit(leftColumn, optionsPanel)
	mainSplit.Offset = 0.55 // Video/metadata column gets 55%, options gets 45%

	// Core content now just the split; ancillary controls stack in bottomSection.
	mainContent := container.NewMax(mainSplit)

	resetBtn := widget.NewButton("Reset", func() {
		tabs.SelectIndex(0) // Select Simple tab
		state.convert.Mode = "Simple"
		formatSelect.SetSelected("MP4 (H.264)")
		state.convert.Quality = "Standard (CRF 23)"
		qualitySelectSimple.SetSelected("Standard (CRF 23)")
		qualitySelectAdv.SetSelected("Standard (CRF 23)")
		aspectOptions.SetSelected("Auto")
		targetAspectSelect.SetSelected("Source")
		updateAspectBoxVisibility()
		if updateEncodingControls != nil {
			updateEncodingControls()
		}
		if updateQualityVisibility != nil {
			updateQualityVisibility()
		}
		state.persistConvertConfig()
		logging.Debug(logging.CatUI, "convert settings reset to defaults")
	})
	statusLabel := widget.NewLabel("")
	statusLabel.Wrapping = fyne.TextTruncate // Prevent text wrapping to new line
	if state.convertBusy {
		statusLabel.SetText(state.convertStatus)
	} else if src != nil {
		statusLabel.SetText("Ready to convert")
	} else {
		statusLabel.SetText("Load a video to convert")
	}
	activity := widget.NewProgressBarInfinite()
	activity.Stop()
	activity.Hide()
	if state.convertBusy {
		activity.Show()
		activity.Start()
	}
	var convertBtn *widget.Button
	var cancelBtn *widget.Button
	var cancelQueueBtn *widget.Button
	cancelBtn = widget.NewButton("Cancel", func() {
		state.cancelConvert(cancelBtn, convertBtn, activity, statusLabel)
	})
	cancelBtn.Importance = widget.DangerImportance
	cancelBtn.Disable()

	cancelQueueBtn = widget.NewButton("Cancel Active Job", func() {
		if state.jobQueue == nil {
			dialog.ShowInformation("Cancel", "Queue not initialized.", state.window)
			return
		}
		job := state.jobQueue.CurrentRunning()
		if job == nil {
			dialog.ShowInformation("Cancel", "No running job to cancel.", state.window)
			return
		}
		if err := state.jobQueue.Cancel(job.ID); err != nil {
			dialog.ShowError(fmt.Errorf("failed to cancel job: %w", err), state.window)
			return
		}
		dialog.ShowInformation("Cancelled", fmt.Sprintf("Cancelled job: %s", job.Title), state.window)
	})
	cancelQueueBtn.Importance = widget.DangerImportance
	cancelQueueBtn.Disable()

	// Add to Queue button
	addQueueBtn := widget.NewButton("Add to Queue", func() {
		state.persistConvertConfig()
		state.executeAddToQueue()
	})
	if src == nil {
		addQueueBtn.Disable()
	}

	convertBtn = widget.NewButton("CONVERT NOW", func() {
		state.persistConvertConfig()
		state.executeConversion()
	})
	convertBtn.Importance = widget.HighImportance
	if src == nil {
		convertBtn.Disable()
	}

	viewLogBtn := widget.NewButton("View Log", func() {
		if state.convertActiveLog == "" {
			dialog.ShowInformation("No Log", "No conversion log available.", state.window)
			return
		}
		state.openLogViewer("Conversion Log", state.convertActiveLog, state.convertBusy)
	})
	viewLogBtn.Importance = widget.LowImportance
	if state.convertActiveLog == "" {
		viewLogBtn.Disable()
	}
	if state.convertBusy {
		// Allow queueing new jobs while current convert runs; just disable Convert Now and enable Cancel.
		convertBtn.Disable()
		cancelBtn.Enable()
		addQueueBtn.Enable()
		if state.convertActiveLog != "" {
			viewLogBtn.Enable()
		}
	}
	// Also disable if queue is running
	if state.jobQueue != nil && state.jobQueue.IsRunning() {
		convertBtn.Disable()
		addQueueBtn.Enable()
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
		// macOS Command+Enter is reported as Super+Enter
		c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyReturn, Modifier: fyne.KeyModifierSuper}, func(fyne.Shortcut) {
			triggerNow()
		})
		c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyEnter, Modifier: fyne.KeyModifierSuper}, func(fyne.Shortcut) {
			triggerNow()
		})
	}

	// Auto-compare checkbox
	autoCompareCheck := widget.NewCheck("Compare After", func(checked bool) {
		state.autoCompare = checked
	})
	autoCompareCheck.SetChecked(state.autoCompare)

	// Load/Save config buttons
	loadCfgBtn := widget.NewButton("Load Config", func() {
		cfg, err := loadPersistedConvertConfig()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				dialog.ShowInformation("No Config", "No saved config found yet. It will save automatically after your first change.", state.window)
			} else {
				dialog.ShowError(fmt.Errorf("failed to load config: %w", err), state.window)
			}
			return
		}
		state.convert = cfg
		state.showConvertView(state.source)
	})
	saveCfgBtn := widget.NewButton("Save Config", func() {
		if err := savePersistedConvertConfig(state.convert); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save config: %w", err), state.window)
			return
		}
		dialog.ShowInformation("Config Saved", fmt.Sprintf("Saved to %s", defaultConvertConfigPath()), state.window)
	})

	leftControls := container.NewHBox(resetBtn, loadCfgBtn, saveCfgBtn, autoCompareCheck)
	rightControls := container.NewHBox(cancelBtn, cancelQueueBtn, viewLogBtn, addQueueBtn, convertBtn)
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

	scrollableMain := container.NewVScroll(mainContent)

	mainWithFooter := container.NewBorder(
		nil,
		container.NewVBox(
			snippetConfigRow,
			snippetRow,
			widget.NewSeparator(),
		),
		nil, nil,
		container.NewMax(scrollableMain),
	)
	return container.NewBorder(backBar, moduleFooter(convertColor, actionBar, state.statsBar), nil, nil, mainWithFooter)

}

func makeLabeledPanel(title, body string, min fyne.Size) *fyne.Container {
	rect := canvas.NewRectangle(utils.MustHex("#191F35"))
	rect.CornerRadius = 8
	rect.StrokeColor = gridColor
	rect.StrokeWidth = 1
	rect.SetMinSize(min)

	header := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	desc := widget.NewLabel(body)
	desc.Wrapping = fyne.TextWrapWord

	box := container.NewVBox(header, desc, layout.NewSpacer())
	return container.NewMax(rect, container.NewPadded(box))
}

func buildMetadataPanel(state *appState, src *videoSource, min fyne.Size) (fyne.CanvasObject, func()) {
	outer := canvas.NewRectangle(utils.MustHex("#191F35"))
	outer.CornerRadius = 8
	outer.StrokeColor = gridColor
	outer.StrokeWidth = 1
	outer.SetMinSize(min)

	header := widget.NewLabelWithStyle("Metadata", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	var top fyne.CanvasObject = header

	if src == nil {
		body := container.NewVBox(
			top,
			widget.NewSeparator(),
			widget.NewLabel("Load a clip to inspect its technical details."),
			layout.NewSpacer(),
		)
		return container.NewMax(outer, container.NewPadded(body)), func() {}
	}

	bitrate := "--"
	if src.Bitrate > 0 {
		bitrate = fmt.Sprintf("%d kbps", src.Bitrate/1000)
	}

	audioBitrate := "--"
	if src.AudioBitrate > 0 {
		audioBitrate = fmt.Sprintf("%d kbps", src.AudioBitrate/1000)
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

	info := widget.NewForm(
		widget.NewFormItem("File", widget.NewLabel(src.DisplayName)),
		widget.NewFormItem("Format Family", widget.NewLabel(utils.FirstNonEmpty(src.Format, "Unknown"))),
		widget.NewFormItem("Resolution", widget.NewLabel(fmt.Sprintf("%dx%d", src.Width, src.Height))),
		widget.NewFormItem("Aspect Ratio", widget.NewLabel(src.AspectRatioString())),
		widget.NewFormItem("Pixel Aspect Ratio", widget.NewLabel(par)),
		widget.NewFormItem("Duration", widget.NewLabel(src.DurationString())),
		widget.NewFormItem("Video Codec", widget.NewLabel(utils.FirstNonEmpty(src.VideoCodec, "Unknown"))),
		widget.NewFormItem("Video Bitrate", widget.NewLabel(bitrate)),
		widget.NewFormItem("Frame Rate", widget.NewLabel(fmt.Sprintf("%.2f fps", src.FrameRate))),
		widget.NewFormItem("Pixel Format", widget.NewLabel(utils.FirstNonEmpty(src.PixelFormat, "Unknown"))),
		widget.NewFormItem("Interlacing", widget.NewLabel(interlacing)),
		widget.NewFormItem("Color Space", widget.NewLabel(colorSpace)),
		widget.NewFormItem("Color Range", widget.NewLabel(colorRange)),
		widget.NewFormItem("GOP Size", widget.NewLabel(gopSize)),
		widget.NewFormItem("Audio Codec", widget.NewLabel(utils.FirstNonEmpty(src.AudioCodec, "Unknown"))),
		widget.NewFormItem("Audio Bitrate", widget.NewLabel(audioBitrate)),
		widget.NewFormItem("Audio Rate", widget.NewLabel(fmt.Sprintf("%d Hz", src.AudioRate))),
		widget.NewFormItem("Channels", widget.NewLabel(utils.ChannelLabel(src.Channels))),
		widget.NewFormItem("Chapters", widget.NewLabel(chapters)),
		widget.NewFormItem("Metadata", widget.NewLabel(metadata)),
	)
	for _, item := range info.Items {
		if lbl, ok := item.Widget.(*widget.Label); ok {
			lbl.Wrapping = fyne.TextWrapWord
			lbl.TextStyle = fyne.TextStyle{} // prevent selection
		}
	}

	// Copy metadata button - beside header text
	copyBtn := widget.NewButton("📋", func() {
		state.window.Clipboard().SetContent(metadataText)
		dialog.ShowInformation("Copied", "Metadata copied to clipboard", state.window)
	})
	copyBtn.Importance = widget.LowImportance

	// Clear button to remove the loaded video and reset UI - on the right
	clearBtn := widget.NewButton("Clear Video", func() {
		if state != nil {
			state.clearVideo()
		}
	})
	clearBtn.Importance = widget.LowImportance

	headerRow := container.NewHBox(header, copyBtn)
	top = container.NewBorder(nil, nil, nil, clearBtn, headerRow)

	// Cover art display area - 40% larger (168x168)
	coverImg := canvas.NewImageFromFile("")
	coverImg.FillMode = canvas.ImageFillContain
	coverImg.SetMinSize(fyne.NewSize(168, 168))

	placeholderRect := canvas.NewRectangle(utils.MustHex("#0F1529"))
	placeholderRect.SetMinSize(fyne.NewSize(168, 168))
	placeholderText := widget.NewLabel("Drop cover\nart here")
	placeholderText.Alignment = fyne.TextAlignCenter
	placeholderText.TextStyle = fyne.TextStyle{Italic: true}
	placeholder := container.NewMax(placeholderRect, container.NewCenter(placeholderText))

	// Update cover art when changed
	updateCoverDisplay := func() {
		if state.convert.CoverArtPath != "" {
			coverImg.File = state.convert.CoverArtPath
			coverImg.Refresh()
			placeholder.Hide()
			coverImg.Show()
		} else {
			coverImg.Hide()
			placeholder.Show()
		}
	}
	updateCoverDisplay()

	coverContainer := container.NewMax(placeholder, coverImg)

	// Interlacing Analysis Section
	analyzeBtn := widget.NewButton("Analyze Interlacing", func() {
		if state.source == nil {
			return
		}
		state.interlaceAnalyzing = true
		state.interlaceResult = nil
		state.showConvertView(state.source) // Refresh to show "Analyzing..."

		go func() {
			detector := interlace.NewDetector(platformConfig.FFmpegPath, platformConfig.FFprobePath)
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
	analyzeBtn.Importance = widget.MediumImportance

	var interlaceSection fyne.CanvasObject
	if state.interlaceAnalyzing {
		statusLabel := widget.NewLabel("Analyzing interlacing... (first 500 frames)")
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
			previewBtn := widget.NewButton("Generate Deinterlace Preview", func() {
				if state.source == nil {
					return
				}

				go func() {
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						dialog.ShowInformation("Generating Preview", "Creating comparison preview...", state.window)
					}, false)

					detector := interlace.NewDetector(platformConfig.FFmpegPath, platformConfig.FFprobePath)
					ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
					defer cancel()

					// Generate preview at 10 seconds into the video
					previewPath := filepath.Join(os.TempDir(), fmt.Sprintf("deinterlace_preview_%d.png", time.Now().Unix()))
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

							infoLabel := widget.NewLabel("Left: Original | Right: Deinterlaced")
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
			previewBtn.Importance = widget.LowImportance
			previewSection = previewBtn
		}

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
	} else {
		interlaceSection = container.NewVBox(
			widget.NewSeparator(),
			analyzeBtn,
		)
	}

	// Layout: metadata form on left, cover art on right (bottom-aligned)
	coverColumn := container.NewVBox(layout.NewSpacer(), coverContainer)
	contentArea := container.NewBorder(nil, nil, nil, coverColumn, info)

	body := container.NewVBox(
		top,
		widget.NewSeparator(),
		contentArea,
		interlaceSection,
	)
	return container.NewMax(outer, container.NewPadded(body)), updateCoverDisplay
}

func buildVideoPane(state *appState, min fyne.Size, src *videoSource, onCover func(string)) fyne.CanvasObject {
	outer := canvas.NewRectangle(utils.MustHex("#191F35"))
	outer.CornerRadius = 8
	outer.StrokeColor = gridColor
	outer.StrokeWidth = 1
	defaultAspect := 9.0 / 16.0
	if src != nil && src.Width > 0 && src.Height > 0 {
		defaultAspect = float64(src.Height) / float64(src.Width)
	}
	baseWidth := float64(min.Width)
	targetWidth := float32(baseWidth)
	_ = defaultAspect
	targetHeight := float32(min.Height)
	// Don't set rigid MinSize - let the outer container be flexible
	// outer.SetMinSize(fyne.NewSize(targetWidth, targetHeight))

	if src == nil {
		icon := canvas.NewText("▶", utils.MustHex("#4CE870"))
		icon.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
		icon.TextSize = 42
		hintMain := widget.NewLabelWithStyle("Drop a video or open one to start playback", fyne.TextAlignCenter, fyne.TextStyle{Monospace: true, Bold: true})
		hintSub := widget.NewLabel("MP4, MOV, MKV and more")
		hintSub.Alignment = fyne.TextAlignCenter

		open := widget.NewButton("Open File…", func() {
			logging.Debug(logging.CatUI, "convert open file dialog requested")
			dlg := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
				if err != nil {
					logging.Debug(logging.CatUI, "file open error: %v", err)
					return
				}
				if r == nil {
					return
				}
				path := r.URI().Path()
				r.Close()
				go state.loadVideo(path)
			}, state.window)
			dlg.Resize(fyne.NewSize(600, 400))
			dlg.Show()
		})

		addMultiple := widget.NewButton("Add Multiple…", func() {
			logging.Debug(logging.CatUI, "convert add multiple files dialog requested")
			dlg := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
				if err != nil {
					logging.Debug(logging.CatUI, "file open error: %v", err)
					return
				}
				if r == nil {
					return
				}
				path := r.URI().Path()
				r.Close()
				// For now, load the first selected file
				// In a real multi-select dialog, you'd get all selected files
				go state.loadVideo(path)
			}, state.window)
			dlg.Resize(fyne.NewSize(600, 400))
			dlg.Show()
		})

		placeholder := container.NewVBox(
			container.NewCenter(icon),
			container.NewCenter(hintMain),
			container.NewCenter(hintSub),
			container.NewHBox(open, addMultiple),
		)
		return container.NewMax(outer, container.NewCenter(container.NewPadded(placeholder)))
	}

	state.stopPreview()

	sourceFrame := ""
	if len(src.PreviewFrames) == 0 {
		if thumb, err := capturePreviewFrames(src.Path, src.Duration); err == nil && len(thumb) > 0 {
			sourceFrame = thumb[0]
			src.PreviewFrames = thumb
		}
	} else {
		sourceFrame = src.PreviewFrames[0]
	}
	if sourceFrame != "" {
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
	// Set minimum size based on source aspect ratio
	stageWidth := float32(200)
	stageHeight := float32(113) // Default 16:9
	if src != nil && src.Width > 0 && src.Height > 0 {
		// Calculate height based on actual aspect ratio
		aspectRatio := float32(src.Width) / float32(src.Height)
		stageHeight = stageWidth / aspectRatio
	}
	stage.SetMinSize(fyne.NewSize(stageWidth, stageHeight))
	// Overlay the image directly so it fills the stage while preserving aspect.
	videoStage := container.NewMax(stage, img)

	coverBtn := utils.MakeIconButton("⌾", "Set current frame as cover art", func() {
		path, err := state.captureCoverFromCurrent()
		if err != nil {
			dialog.ShowError(err, state.window)
			return
		}
		if onCover != nil {
			onCover(path)
		}
	})

	saveFrameBtn := utils.MakeIconButton("💾", "Save current frame as PNG", func() {
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

	importBtn := utils.MakeIconButton("⬆", "Import cover art file", func() {
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

	usePlayer := true

	currentTime := widget.NewLabel("0:00")
	totalTime := widget.NewLabel(src.DurationString())
	totalTime.Alignment = fyne.TextAlignTrailing
	var updatingProgress bool
	slider := widget.NewSlider(0, math.Max(1, src.Duration))
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
		var volIcon *widget.Button
		var updatingVolume bool
		ensureSession := func() bool {
			if state.playSess == nil {
				state.playSess = newPlaySession(src.Path, src.Width, src.Height, src.FrameRate, int(targetWidth-28), int(targetHeight-40), updateProgress, img)
				state.playSess.SetVolume(state.playerVolume)
				state.playerPaused = true
			}
			return state.playSess != nil
		}
		slider.OnChanged = func(val float64) {
			if updatingProgress {
				return
			}
			updateProgress(val)
			if ensureSession() {
				state.playSess.Seek(val)
			}
		}
		updateVolIcon := func() {
			if volIcon == nil {
				return
			}
			if state.playerMuted || state.playerVolume <= 0 {
				volIcon.SetText("🔇")
			} else {
				volIcon.SetText("🔊")
			}
		}
		volIcon = utils.MakeIconButton("🔊", "Mute/Unmute", func() {
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
				state.playSess.SetVolume(target)
			} else {
				state.lastVolume = state.playerVolume
				state.playerVolume = 0
				state.playerMuted = true
				state.playSess.SetVolume(0)
			}
			updateVolIcon()
		})
		volSlider := widget.NewSlider(0, 100)
		volSlider.Step = 1
		volSlider.Value = state.playerVolume
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
				state.playSess.SetVolume(val)
			}
			updateVolIcon()
		}
		updateVolIcon()
		volSlider.Refresh()
		playBtn := utils.MakeIconButton("▶/⏸", "Play/Pause", func() {
			if !ensureSession() {
				return
			}
			if state.playerPaused {
				state.playSess.Play()
				state.playerPaused = false
			} else {
				state.playSess.Pause()
				state.playerPaused = true
			}
		})
		fullBtn := utils.MakeIconButton("⛶", "Toggle fullscreen", func() {
			// Placeholder: embed fullscreen toggle into playback surface later.
		})
		volBox := container.NewHBox(volIcon, container.NewMax(volSlider))
		progress := container.NewBorder(nil, nil, currentTime, totalTime, container.NewMax(slider))
		controls = container.NewVBox(
			container.NewHBox(playBtn, fullBtn, coverBtn, saveFrameBtn, importBtn, layout.NewSpacer(), volBox),
			progress,
		)
	} else {
		slider := widget.NewSlider(0, math.Max(1, float64(len(src.PreviewFrames)-1)))
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
		playBtn := utils.MakeIconButton("▶/⏸", "Play/Pause", func() {
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
		volSlider := widget.NewSlider(0, 100)
		volSlider.Disable()
		progress := container.NewBorder(nil, nil, currentTime, totalTime, container.NewMax(slider))
		controls = container.NewVBox(
			container.NewHBox(playBtn, coverBtn, saveFrameBtn, importBtn, layout.NewSpacer(), widget.NewLabel("🔇"), container.NewMax(volSlider)),
			progress,
		)
		if len(src.PreviewFrames) > 1 {
			state.startPreview(src.PreviewFrames, img, slider)
		} else {
			playBtn.Disable()
		}
	}

	barBg := canvas.NewRectangle(color.NRGBA{R: 12, G: 17, B: 31, A: 180})
	barBg.SetMinSize(fyne.NewSize(targetWidth-32, 72))
	overlayBar := container.NewMax(barBg, container.NewPadded(controls))

	overlay := container.NewVBox(layout.NewSpacer(), overlayBar)
	videoWithOverlay := container.NewMax(videoStage, overlay)
	state.setPlayerSurface(videoStage, int(targetWidth-12), int(targetHeight-12))

	stack := container.NewVBox(
		container.NewPadded(videoWithOverlay),
	)
	return container.NewMax(outer, container.NewPadded(stack))
}

type playSession struct {
	path     string
	fps      float64
	width    int
	height   int
	targetW  int
	targetH  int
	volume   float64
	muted    bool
	paused   bool
	current  float64
	stop     chan struct{}
	done     chan struct{}
	prog     func(float64)
	img      *canvas.Image
	mu       sync.Mutex
	videoCmd *exec.Cmd
	audioCmd *exec.Cmd
	frameN   int
}

var audioCtxGlobal struct {
	once sync.Once
	ctx  *oto.Context
	err  error
}

func getAudioContext(sampleRate, channels, bytesPerSample int) (*oto.Context, error) {
	audioCtxGlobal.once.Do(func() {
		audioCtxGlobal.ctx, audioCtxGlobal.err = oto.NewContext(sampleRate, channels, bytesPerSample, 2048)
	})
	return audioCtxGlobal.ctx, audioCtxGlobal.err
}

func newPlaySession(path string, w, h int, fps float64, targetW, targetH int, prog func(float64), img *canvas.Image) *playSession {
	if fps <= 0 {
		fps = 24
	}
	if targetW <= 0 {
		targetW = 640
	}
	if targetH <= 0 {
		targetH = int(float64(targetW) * (float64(h) / float64(utils.MaxInt(w, 1))))
	}
	return &playSession{
		path:    path,
		fps:     fps,
		width:   w,
		height:  h,
		targetW: targetW,
		targetH: targetH,
		volume:  100,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
		prog:    prog,
		img:     img,
	}
}

func (p *playSession) Play() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.videoCmd == nil && p.audioCmd == nil {
		p.startLocked(p.current)
		return
	}
	p.paused = false
}

func (p *playSession) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paused = true
}

func (p *playSession) Seek(offset float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if offset < 0 {
		offset = 0
	}
	paused := p.paused
	p.current = offset
	p.stopLocked()
	p.startLocked(p.current)
	p.paused = paused
	if p.paused {
		// Ensure loops honor paused right after restart.
		time.AfterFunc(30*time.Millisecond, func() {
			p.mu.Lock()
			defer p.mu.Unlock()
			p.paused = true
		})
	}
	if p.prog != nil {
		p.prog(p.current)
	}
}

func (p *playSession) SetVolume(v float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	p.volume = v
	if v > 0 {
		p.muted = false
	} else {
		p.muted = true
	}
}

func (p *playSession) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
}

func (p *playSession) stopLocked() {
	select {
	case <-p.stop:
	default:
		close(p.stop)
	}
	if p.videoCmd != nil && p.videoCmd.Process != nil {
		_ = p.videoCmd.Process.Kill()
		_ = p.videoCmd.Wait()
	}
	if p.audioCmd != nil && p.audioCmd.Process != nil {
		_ = p.audioCmd.Process.Kill()
		_ = p.audioCmd.Wait()
	}
	p.videoCmd = nil
	p.audioCmd = nil
	p.stop = make(chan struct{})
	p.done = make(chan struct{})
}

func (p *playSession) startLocked(offset float64) {
	p.paused = false
	p.current = offset
	p.frameN = 0
	logging.Debug(logging.CatFFMPEG, "playSession start path=%s offset=%.3f fps=%.3f target=%dx%d", p.path, offset, p.fps, p.targetW, p.targetH)
	p.runVideo(offset)
	p.runAudio(offset)
}

func (p *playSession) runVideo(offset float64) {
	var stderr bytes.Buffer
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-ss", fmt.Sprintf("%.3f", offset),
		"-i", p.path,
		"-vf", fmt.Sprintf("scale=%d:%d", p.targetW, p.targetH),
		"-f", "rawvideo",
		"-pix_fmt", "rgb24",
		"-r", fmt.Sprintf("%.3f", p.fps),
		"-",
	}
	cmd := exec.Command(platformConfig.FFmpegPath, args...)
	utils.ApplyNoWindow(cmd)
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logging.Debug(logging.CatFFMPEG, "video pipe error: %v", err)
		return
	}
	if err := cmd.Start(); err != nil {
		logging.Debug(logging.CatFFMPEG, "video start failed: %v (%s)", err, strings.TrimSpace(stderr.String()))
		return
	}
	// Pace frames to the source frame rate instead of hammering refreshes as fast as possible.
	frameDur := time.Second
	if p.fps > 0 {
		frameDur = time.Duration(float64(time.Second) / math.Max(p.fps, 0.1))
	}
	nextFrameAt := time.Now()
	p.videoCmd = cmd
	frameSize := p.targetW * p.targetH * 3
	buf := make([]byte, frameSize)
	go func() {
		defer cmd.Process.Kill()
		for {
			select {
			case <-p.stop:
				logging.Debug(logging.CatFFMPEG, "video loop stop")
				return
			default:
			}
			if p.paused {
				time.Sleep(30 * time.Millisecond)
				nextFrameAt = time.Now().Add(frameDur)
				continue
			}
			_, err := io.ReadFull(stdout, buf)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				msg := strings.TrimSpace(stderr.String())
				logging.Debug(logging.CatFFMPEG, "video read failed: %v (%s)", err, msg)
				return
			}
			if delay := time.Until(nextFrameAt); delay > 0 {
				time.Sleep(delay)
			}
			nextFrameAt = nextFrameAt.Add(frameDur)
			// Allocate a fresh frame to avoid concurrent texture reuse issues.
			frame := image.NewRGBA(image.Rect(0, 0, p.targetW, p.targetH))
			utils.CopyRGBToRGBA(frame.Pix, buf)
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				if p.img != nil {
					// Ensure we render the live frame, not a stale resource preview.
					p.img.Resource = nil
					p.img.File = ""
					p.img.Image = frame
					p.img.Refresh()
				}
			}, false)
			if p.frameN < 3 {
				logging.Debug(logging.CatFFMPEG, "video frame %d drawn (%.2fs)", p.frameN+1, p.current)
			}
			p.frameN++
			if p.fps > 0 {
				p.current = offset + (float64(p.frameN) / p.fps)
			}
			if p.prog != nil {
				p.prog(p.current)
			}
		}
	}()
}

func (p *playSession) runAudio(offset float64) {
	const sampleRate = 48000
	const channels = 2
	const bytesPerSample = 2
	var stderr bytes.Buffer
	cmd := exec.Command(platformConfig.FFmpegPath,
		"-hide_banner", "-loglevel", "error",
		"-ss", fmt.Sprintf("%.3f", offset),
		"-i", p.path,
		"-vn",
		"-ac", fmt.Sprintf("%d", channels),
		"-ar", fmt.Sprintf("%d", sampleRate),
		"-f", "s16le",
		"-",
	)
	utils.ApplyNoWindow(cmd)
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logging.Debug(logging.CatFFMPEG, "audio pipe error: %v", err)
		return
	}
	if err := cmd.Start(); err != nil {
		logging.Debug(logging.CatFFMPEG, "audio start failed: %v (%s)", err, strings.TrimSpace(stderr.String()))
		return
	}
	p.audioCmd = cmd
	ctx, err := getAudioContext(sampleRate, channels, bytesPerSample)
	if err != nil {
		logging.Debug(logging.CatFFMPEG, "audio context error: %v", err)
		return
	}
	player := ctx.NewPlayer()
	if player == nil {
		logging.Debug(logging.CatFFMPEG, "audio player creation failed")
		return
	}
	localPlayer := player
	go func() {
		defer cmd.Process.Kill()
		defer localPlayer.Close()
		chunk := make([]byte, 4096)
		tmp := make([]byte, 4096)
		loggedFirst := false
		for {
			select {
			case <-p.stop:
				logging.Debug(logging.CatFFMPEG, "audio loop stop")
				return
			default:
			}
			if p.paused {
				time.Sleep(30 * time.Millisecond)
				continue
			}
			n, err := stdout.Read(chunk)
			if n > 0 {
				if !loggedFirst {
					logging.Debug(logging.CatFFMPEG, "audio stream delivering bytes")
					loggedFirst = true
				}
				gain := p.volume / 100.0
				if gain < 0 {
					gain = 0
				}
				if gain > 2 {
					gain = 2
				}
				copy(tmp, chunk[:n])
				if p.muted || gain <= 0 {
					for i := 0; i < n; i++ {
						tmp[i] = 0
					}
				} else if math.Abs(1-gain) > 0.001 {
					for i := 0; i+1 < n; i += 2 {
						sample := int16(binary.LittleEndian.Uint16(tmp[i:]))
						amp := int(float64(sample) * gain)
						if amp > math.MaxInt16 {
							amp = math.MaxInt16
						}
						if amp < math.MinInt16 {
							amp = math.MinInt16
						}
						binary.LittleEndian.PutUint16(tmp[i:], uint16(int16(amp)))
					}
				}
				localPlayer.Write(tmp[:n])
			}
			if err != nil {
				if !errors.Is(err, io.EOF) {
					logging.Debug(logging.CatFFMPEG, "audio read failed: %v (%s)", err, strings.TrimSpace(stderr.String()))
				}
				return
			}
		}
	}()
}

type previewAnimator struct {
	frames  []string
	img     *canvas.Image
	slider  *widget.Slider
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
	// If we have a play session active, capture the current playing frame
	if s.playSess != nil && s.playSess.img != nil && s.playSess.img.Image != nil {
		dest := filepath.Join(os.TempDir(), fmt.Sprintf("videotools-cover-%d.png", time.Now().UnixNano()))
		f, err := os.Create(dest)
		if err != nil {
			return "", err
		}
		defer f.Close()
		if err := png.Encode(f, s.playSess.img.Image); err != nil {
			return "", err
		}
		return dest, nil
	}

	// Otherwise use the current preview frame
	if s.currentFrame == "" {
		return "", fmt.Errorf("no frame available")
	}
	data, err := os.ReadFile(s.currentFrame)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(os.TempDir(), fmt.Sprintf("videotools-cover-%d.png", time.Now().UnixNano()))
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
	dest := filepath.Join(os.TempDir(), fmt.Sprintf("videotools-cover-import-%d%s", time.Now().UnixNano(), filepath.Ext(path)))
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return "", err
	}
	return dest, nil
}

func (s *appState) handleDrop(pos fyne.Position, items []fyne.URI) {
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
			return
		}

		// Load all videos into memory (don't auto-queue)
		// This allows users to adjust settings or generate snippets before manually queuing
		if len(videoPaths) > 1 {
			logging.Debug(logging.CatUI, "multiple videos dropped in convert module; loading all into memory")
			go s.loadMultipleVideos(videoPaths)
		} else {
			// Single video: load it
			logging.Debug(logging.CatUI, "single video dropped in convert module; loading: %s", videoPaths[0])
			go s.loadVideo(videoPaths[0])
		}
		return
	}

	// If in compare module, handle up to 2 video files
	if s.active == "compare" {
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
			dialog.ShowInformation("Compare Videos", "No video files found in dropped items.", s.window)
			return
		}

		// Show message if more than 2 videos dropped
		if len(videoPaths) > 2 {
			dialog.ShowInformation("Compare Videos",
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
			dialog.ShowInformation("Inspect Video", "No video files found in dropped items.", s.window)
			return
		}

		// Load first video
		go func() {
			src, err := probeVideo(videoPaths[0])
			if err != nil {
				logging.Debug(logging.CatModule, "failed to load video for inspect: %v", err)
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					dialog.ShowError(fmt.Errorf("failed to load video: %w", err), s.window)
				}, false)
				return
			}

			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				s.inspectFile = src
				s.inspectInterlaceResult = nil
				s.inspectInterlaceAnalyzing = true
				s.showInspectView()
				logging.Debug(logging.CatModule, "loaded video into inspect module")

				// Auto-run interlacing detection in background
				videoPath := videoPaths[0]
				go func() {
					detector := interlace.NewDetector(platformConfig.FFmpegPath, platformConfig.FFprobePath)
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
			}, false)
		}()

		return
	}

	// If in thumb module, handle single video file
	if s.active == "thumb" {
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
			dialog.ShowInformation("Thumbnail Generation", "No video files found in dropped items.", s.window)
			return
		}

		// Load first video
		go func() {
			src, err := probeVideo(videoPaths[0])
			if err != nil {
				logging.Debug(logging.CatModule, "failed to load video for thumb: %v", err)
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					dialog.ShowError(fmt.Errorf("failed to load video: %w", err), s.window)
				}, false)
				return
			}

			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				s.thumbFile = src
				s.showThumbView()
				logging.Debug(logging.CatModule, "loaded video into thumb module")
			}, false)
		}()

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
			dialog.ShowInformation("Filters", "No video files found in dropped items.", s.window)
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
			dialog.ShowInformation("Upscale", "No video files found in dropped items.", s.window)
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

					// Set default output path if not set and we have at least 2 clips
					if len(s.mergeClips) >= 2 && strings.TrimSpace(s.mergeOutput) == "" {
						first := filepath.Dir(s.mergeClips[0].Path)
						s.mergeOutput = filepath.Join(first, "merged.mkv")
					}

					// Refresh the merge view to show the new clips
					s.showMergeView()
				}, false)
			}

			logging.Debug(logging.CatModule, "added %d clips to merge list", len(videoPaths))
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
	if s.playSess != nil {
		s.playSess.Stop()
		s.playSess = nil
	}
	s.stopProgressLoop()
	src, err := probeVideo(path)
	if err != nil {
		logging.Debug(logging.CatFFMPEG, "ffprobe failed for %s: %v", path, err)
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			s.showErrorWithCopy("Failed to Analyze Video", fmt.Errorf("failed to analyze %s: %w", filepath.Base(path), err))
		}, false)
		return
	}
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
		s.loadedVideos[found] = src
		s.currentIndex = found
	} else if len(s.loadedVideos) > 0 {
		s.loadedVideos = append(s.loadedVideos, src)
		s.currentIndex = len(s.loadedVideos) - 1
	} else {
		s.loadedVideos = []*videoSource{src}
		s.currentIndex = 0
	}

	logging.Debug(logging.CatModule, "video loaded %+v", src)
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		s.showConvertView(src)
	}, false)
}

// loadMultipleVideos loads multiple videos into memory without auto-queuing
func (s *appState) loadMultipleVideos(paths []string) {
	logging.Debug(logging.CatModule, "loading %d videos into memory", len(paths))

	var validVideos []*videoSource
	var failedFiles []string

	for _, path := range paths {
		src, err := probeVideo(path)
		if err != nil {
			logging.Debug(logging.CatFFMPEG, "ffprobe failed for %s: %v", path, err)
			failedFiles = append(failedFiles, filepath.Base(path))
			continue
		}
		validVideos = append(validVideos, src)
	}

	if len(validVideos) == 0 {
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			msg := fmt.Sprintf("Failed to analyze %d file(s):\n%s", len(failedFiles), strings.Join(failedFiles, ", "))
			s.showErrorWithCopy("Load Failed", fmt.Errorf("%s", msg))
		}, false)
		return
	}

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

	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		// Silently load videos - showing the convert view is sufficient feedback
		s.showConvertView(firstVideo)

		// Log any failed files for debugging
		if len(failedFiles) > 0 {
			logging.Debug(logging.CatModule, "%d file(s) failed to analyze: %s", len(failedFiles), strings.Join(failedFiles, ", "))
		}
	}, false)

	logging.Debug(logging.CatModule, "loaded %d videos into memory", len(validVideos))
}

func (s *appState) clearVideo() {
	logging.Debug(logging.CatModule, "clearing loaded video")
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

	if len(src.PreviewFrames) > 0 {
		s.currentFrame = src.PreviewFrames[0]
	} else {
		s.currentFrame = ""
	}

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

	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		s.showConvertView(src)
	}, false)
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
		cmd := exec.Command(platformConfig.FFmpegPath, "-hide_banner", "-encoders")
		utils.ApplyNoWindow(cmd)
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
	cmd := exec.Command(platformConfig.FFmpegPath, "-hide_banner", "-encoders")
	utils.ApplyNoWindow(cmd)
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
		cmd := exec.Command(platformConfig.FFmpegPath, "-hide_banner", "-encoders")
		utils.ApplyNoWindow(cmd)
		output, err := cmd.CombinedOutput()
		if err == nil {
			if strings.Contains(string(output), " "+encoder+" ") || strings.Contains(string(output), " "+encoder+"\n") {
				logging.Debug(logging.CatFFMPEG, "detected hardware encoder: %s", encoder)
				return encoder
			}
		}
	}

	cmd := exec.Command(platformConfig.FFmpegPath, "-hide_banner", "-encoders")
	utils.ApplyNoWindow(cmd)
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
		if accel == "amf" {
			return "av1_amf"
		} else if accel == "nvenc" {
			return "av1_nvenc"
		} else if accel == "qsv" {
			return "av1_qsv"
		} else if accel == "vaapi" {
			return "av1_vaapi"
		}
		// When set to "none" or empty, use software encoder
		return "libaom-av1"
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

func (s *appState) cancelConvert(cancelBtn, btn *widget.Button, spinner *widget.ProgressBarInfinite, status *widget.Label) {
	if s.convertCancel == nil {
		return
	}
	s.convertStatus = "Cancelling…"
	// Widget states will be updated by the UI refresh ticker
	s.convertCancel()
}

func (s *appState) startConvert(status *widget.Label, btn, cancelBtn *widget.Button, spinner *widget.ProgressBarInfinite) {
	setStatus := func(msg string) {
		s.convertStatus = msg
		logging.Debug(logging.CatFFMPEG, "convert status: %s", msg)
		// Note: Don't update widgets here - they may be stale if user navigated away
		// The UI will refresh from state.convertStatus via a ticker
	}
	if s.source == nil {
		dialog.ShowInformation("Convert", "Load a video first.", s.window)
		return
	}
	if s.convertBusy {
		return
	}
	src := s.source
	cfg := s.convert
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

	// DVD presets: enforce compliant codecs and audio settings
	// Note: We do NOT force resolution - user can choose Source or specific resolution
	if isDVD {
		if strings.Contains(cfg.SelectedFormat.Label, "PAL") {
			cfg.TargetResolution = "PAL (720×540)"
			cfg.FrameRate = "25"
		} else {
			cfg.TargetResolution = "NTSC (720×480)"
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

	// Scaling/Resolution
	if cfg.TargetResolution != "" && cfg.TargetResolution != "Source" {
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
		case "NTSC (720×480)":
			scaleFilter = "scale=720:480"
		case "PAL (720×540)":
			scaleFilter = "scale=720:540"
		case "PAL (720×576)":
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
	if cfg.OutputAspect != "" && !strings.EqualFold(cfg.OutputAspect, "source") {
		srcAspect := utils.AspectRatioFloat(src.Width, src.Height)
		targetAspect := resolveTargetAspect(cfg.OutputAspect, src)
		if targetAspect > 0 && srcAspect > 0 && !utils.RatiosApproxEqual(targetAspect, srcAspect, 0.01) {
			vf = append(vf, aspectFilters(targetAspect, cfg.AspectHandling)...)
			logging.Debug(logging.CatFFMPEG, "converting aspect ratio from %.2f to %.2f using %s mode", srcAspect, targetAspect, cfg.AspectHandling)
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
			args = append(args, "-b:v", vb, "-minrate", vb, "-maxrate", vb, "-bufsize", vb)
		} else if cfg.BitrateMode == "VBR" {
			// Variable bitrate (2-pass if enabled)
			if cfg.VideoBitrate != "" {
				args = append(args, "-b:v", cfg.VideoBitrate)
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
			}
		}

		// Audio sample rate
		if cfg.NormalizeAudio {
			// Force 48kHz for maximum compatibility
			args = append(args, "-ar", "48000")
			logging.Debug(logging.CatFFMPEG, "audio normalization: forcing 48kHz sample rate")
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
	_ = logPath
	setStatus("Preparing conversion…")
	// Widget states will be updated by the UI refresh ticker

	ctx, cancel := context.WithCancel(context.Background())
	s.convertCancel = cancel

	go func() {
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			setStatus("Running ffmpeg…")
		}, false)
		if logFile != nil {
			defer logFile.Close()
		}

		started := time.Now()
		cmd := exec.CommandContext(ctx, platformConfig.FFmpegPath, args...)
		utils.ApplyNoWindow(cmd)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			logging.Debug(logging.CatFFMPEG, "convert stdout pipe failed: %v", err)
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
						lbl = fmt.Sprintf("Converting… %.0f%% | %.0f fps | elapsed %s | ETA %s | %.2fx", pct, currentFPS, formatShortDuration(elapsedWall), etaOrDash(eta), speed)
					} else {
						lbl = fmt.Sprintf("Converting… %.0f%% | elapsed %s | ETA %s | %.2fx", pct, formatShortDuration(elapsedWall), etaOrDash(eta), speed)
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

		if err := cmd.Start(); err != nil {
			close(progressQuit)
			logging.Debug(logging.CatFFMPEG, "convert failed to start: %v", err)
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
			logging.Debug(logging.CatFFMPEG, "convert failed: %v stderr=%s", err, stderrOutput)
			if logFile != nil {
				fmt.Fprintf(logFile, "\nStatus: failed at %s\nError: %v\nStderr:\n%s\n", time.Now().Format(time.RFC3339), err, stderrOutput)
			}
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
					errorMsg = fmt.Errorf("Hardware encoding (%s→%s) failed - no compatible hardware found.\n\nSwitched hardware acceleration to 'none'. Please try again (software encoding).\n\nFFmpeg output:\n%s", chosen, resolvedAccel, stderrOutput)
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
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			setStatus("Validating output…")
		}, false)
		if _, probeErr := probeVideo(outPath); probeErr != nil {
			logging.Debug(logging.CatFFMPEG, "convert probe failed: %v", probeErr)
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
			dialog.ShowInformation("Convert", fmt.Sprintf("Saved %s", outPath), s.window)
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

	// Crop mode: center crop to target aspect ratio
	if strings.EqualFold(mode, "Crop") || strings.EqualFold(mode, "Auto") {
		// Crop to target aspect ratio with even dimensions for H.264 encoding
		// Use trunc/2*2 to ensure even dimensions
		crop := fmt.Sprintf("crop=w='trunc(if(gt(a,%[1]s),ih*%[1]s,iw)/2)*2':h='trunc(if(gt(a,%[1]s),ih,iw/%[1]s)/2)*2':x='(iw-out_w)/2':y='(ih-out_h)/2'", ar)
		return []string{crop, "setsar=1"}
	}

	// Stretch mode: just change the aspect ratio without cropping or padding
	if strings.EqualFold(mode, "Stretch") {
		scale := fmt.Sprintf("scale=w='trunc(ih*%[1]s/2)*2':h='trunc(iw/%[1]s/2)*2'", ar)
		return []string{scale, "setsar=1"}
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
		return []string{filterStr, "setsar=1"}
	}

	// Letterbox/Pillarbox: keep source resolution, just pad to target aspect with black bars
	pad := fmt.Sprintf("pad=w='trunc(max(iw,ih*%[1]s)/2)*2':h='trunc(max(ih,iw/%[1]s)/2)*2':x='(ow-iw)/2':y='(oh-ih)/2':color=black", ar)
	return []string{pad, "setsar=1"}
}

func (s *appState) generateSnippet() {
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
		srcAspect := utils.AspectRatioFloat(src.Width, src.Height)
		targetAspect := resolveTargetAspect(s.convert.OutputAspect, src)
		aspectConversionNeeded := targetAspect > 0 && srcAspect > 0 && !utils.RatiosApproxEqual(targetAspect, srcAspect, 0.01)
		if aspectConversionNeeded {
			vf = append(vf, aspectFilters(targetAspect, s.convert.AspectHandling)...)
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

	cmd := exec.CommandContext(ctx, platformConfig.FFmpegPath, args...)
	utils.ApplyNoWindow(cmd)
	logging.Debug(logging.CatFFMPEG, "snippet command: %s", strings.Join(cmd.Args, " "))

	// Show progress dialog for snippets that need re-encoding (WMV, filters, etc.)
	var progressDialog dialog.Dialog
	if needsReencode {
		progressDialog = dialog.NewCustom("Generating Snippet", "Cancel",
			widget.NewLabel("Generating 20-second snippet...\nThis may take 20-30 seconds for WMV files."),
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
		dialog.ShowInformation("Snippet Created", fmt.Sprintf("Saved %s", outPath), s.window)
	}, false)
}

func capturePreviewFrames(path string, duration float64) ([]string, error) {
	center := math.Max(0, duration/2-1)
	start := fmt.Sprintf("%.2f", center)
	dir, err := os.MkdirTemp("", "videotools-frames-*")
	if err != nil {
		return nil, err
	}
	pattern := filepath.Join(dir, "frame-%03d.png")
	cmd := exec.Command(platformConfig.FFmpegPath,
		"-y",
		"-ss", start,
		"-i", path,
		"-t", "3",
		"-vf", "scale=640:-1:flags=lanczos,fps=8",
		pattern,
	)
	utils.ApplyNoWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("preview capture failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	files, err := filepath.Glob(filepath.Join(dir, "frame-*.png"))
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("no preview frames generated")
	}
	slices.Sort(files)
	return files, nil
}

type videoSource struct {
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

	// Advanced metadata
	SampleAspectRatio string // Pixel Aspect Ratio (SAR) - e.g., "1:1", "40:33"
	ColorSpace        string // Color space/primaries - e.g., "bt709", "bt601"
	ColorRange        string // Color range - "tv" (limited) or "pc" (full)
	GOPSize           int    // GOP size / keyframe interval
	HasChapters       bool   // Whether file has embedded chapters
	HasMetadata       bool   // Whether file has title/copyright/etc metadata
	Metadata          map[string]string
}

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

func probeVideo(path string) (*videoSource, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-show_chapters",
		path,
	)
	utils.ApplyNoWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
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
			Index        int    `json:"index"`
			CodecType    string `json:"codec_type"`
			CodecName    string `json:"codec_name"`
			Width        int    `json:"width"`
			Height       int    `json:"height"`
			Duration     string `json:"duration"`
			BitRate      string `json:"bit_rate"`
			PixFmt       string `json:"pix_fmt"`
			SampleRate   string `json:"sample_rate"`
			Channels     int    `json:"channels"`
			AvgFrameRate string `json:"avg_frame_rate"`
			FieldOrder   string `json:"field_order"`
			Disposition  struct {
				AttachedPic int `json:"attached_pic"`
			} `json:"disposition"`
		} `json:"streams"`
		Chapters []struct {
			ID int `json:"id"`
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
	if rate, err := utils.ParseInt(result.Format.BitRate); err == nil {
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
		logging.Debug(logging.CatFFMPEG, "found %d chapter(s) in video", len(result.Chapters))
	}

	// Track if we've found the main video stream (not cover art)
	foundMainVideo := false
	var coverArtStreamIndex int = -1

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
				}
				if stream.PixFmt != "" {
					src.PixelFormat = stream.PixFmt
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
			}
		}
	}

	// Extract embedded cover art if present
	if coverArtStreamIndex >= 0 {
		coverPath := filepath.Join(os.TempDir(), fmt.Sprintf("videotools-embedded-cover-%d.png", time.Now().UnixNano()))
		extractCmd := exec.CommandContext(ctx, platformConfig.FFmpegPath,
			"-i", path,
			"-map", fmt.Sprintf("0:%d", coverArtStreamIndex),
			"-frames:v", "1",
			"-y",
			coverPath,
		)
		utils.ApplyNoWindow(extractCmd)
		if err := extractCmd.Run(); err != nil {
			logging.Debug(logging.CatFFMPEG, "failed to extract embedded cover art: %v", err)
		} else {
			src.EmbeddedCoverArt = coverPath
			logging.Debug(logging.CatFFMPEG, "extracted embedded cover art to %s", coverPath)
		}
	}

	return src, nil
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
	cmd := exec.CommandContext(ctx, platformConfig.FFmpegPath,
		"-ss", fmt.Sprintf("%.2f", sampleStart),
		"-i", path,
		"-t", "10",
		"-vf", "cropdetect=24:16:0",
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

// buildCompareView creates the UI for comparing two videos side by side
func buildCompareView(state *appState) fyne.CanvasObject {
	compareColor := moduleColor("compare")

	// Back button
	backBtn := widget.NewButton("< COMPARE", func() {
		state.showMainMenu()
	})
	backBtn.Importance = widget.LowImportance

	// Top bar with module color
	queueBtn := widget.NewButton("View Queue", func() {
		state.showQueue()
	})
	state.queueBtn = queueBtn
	state.updateQueueButtonLabel()
	topBar := ui.TintedBar(compareColor, container.NewHBox(backBtn, layout.NewSpacer(), queueBtn))
	bottomBar := moduleFooter(compareColor, layout.NewSpacer(), state.statsBar)

	// Instructions
	instructions := widget.NewLabel("Load two videos to compare their metadata side by side. Drag videos here or use buttons below.")
	instructions.Wrapping = fyne.TextWrapWord
	instructions.Alignment = fyne.TextAlignCenter

	// Fullscreen Compare button
	fullscreenBtn := widget.NewButton("Fullscreen Compare", func() {
		if state.compareFile1 == nil && state.compareFile2 == nil {
			dialog.ShowInformation("No Videos", "Load two videos to use fullscreen comparison.", state.window)
			return
		}
		state.showCompareFullscreen()
	})
	fullscreenBtn.Importance = widget.MediumImportance

	// Copy Comparison button - copies both files' metadata side by side
	copyComparisonBtn := widget.NewButton("Copy Comparison", func() {
		if state.compareFile1 == nil && state.compareFile2 == nil {
			dialog.ShowInformation("No Videos", "Load at least one video to copy comparison metadata.", state.window)
			return
		}

		// Format side-by-side comparison
		var comparisonText strings.Builder
		comparisonText.WriteString("═══════════════════════════════════════════════════════════════════════\n")
		comparisonText.WriteString("                        VIDEO COMPARISON REPORT\n")
		comparisonText.WriteString("═══════════════════════════════════════════════════════════════════════\n\n")

		// File names header
		file1Name := "Not loaded"
		file2Name := "Not loaded"
		if state.compareFile1 != nil {
			file1Name = filepath.Base(state.compareFile1.Path)
		}
		if state.compareFile2 != nil {
			file2Name = filepath.Base(state.compareFile2.Path)
		}

		comparisonText.WriteString(fmt.Sprintf("FILE 1: %s\n", file1Name))
		comparisonText.WriteString(fmt.Sprintf("FILE 2: %s\n", file2Name))
		comparisonText.WriteString("───────────────────────────────────────────────────────────────────────\n\n")

		// Helper to get field value or placeholder
		getField := func(src *videoSource, getter func(*videoSource) string) string {
			if src == nil {
				return "—"
			}
			return getter(src)
		}

		// File Info section
		comparisonText.WriteString("━━━ FILE INFO ━━━\n")

		var file1SizeBytes int64
		file1Size := getField(state.compareFile1, func(src *videoSource) string {
			if fi, err := os.Stat(src.Path); err == nil {
				file1SizeBytes = fi.Size()
				return utils.FormatBytes(fi.Size())
			}
			return "Unknown"
		})
		file2Size := getField(state.compareFile2, func(src *videoSource) string {
			if fi, err := os.Stat(src.Path); err == nil {
				if file1SizeBytes > 0 {
					return utils.DeltaBytes(fi.Size(), file1SizeBytes)
				}
				return utils.FormatBytes(fi.Size())
			}
			return "Unknown"
		})

		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n", "File Size:", file1Size, file2Size))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Format Family:",
			getField(state.compareFile1, func(s *videoSource) string { return s.Format }),
			getField(state.compareFile2, func(s *videoSource) string { return s.Format })))

		// Video section
		comparisonText.WriteString("\n━━━ VIDEO ━━━\n")
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Codec:",
			getField(state.compareFile1, func(s *videoSource) string { return s.VideoCodec }),
			getField(state.compareFile2, func(s *videoSource) string { return s.VideoCodec })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Resolution:",
			getField(state.compareFile1, func(s *videoSource) string { return fmt.Sprintf("%dx%d", s.Width, s.Height) }),
			getField(state.compareFile2, func(s *videoSource) string { return fmt.Sprintf("%dx%d", s.Width, s.Height) })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Aspect Ratio:",
			getField(state.compareFile1, func(s *videoSource) string { return s.AspectRatioString() }),
			getField(state.compareFile2, func(s *videoSource) string { return s.AspectRatioString() })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Frame Rate:",
			getField(state.compareFile1, func(s *videoSource) string { return fmt.Sprintf("%.2f fps", s.FrameRate) }),
			getField(state.compareFile2, func(s *videoSource) string { return fmt.Sprintf("%.2f fps", s.FrameRate) })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Bitrate:",
			getField(state.compareFile1, func(s *videoSource) string { return formatBitrateFull(s.Bitrate) }),
			getField(state.compareFile2, func(s *videoSource) string {
				if state.compareFile1 != nil {
					return utils.DeltaBitrate(s.Bitrate, state.compareFile1.Bitrate)
				}
				return formatBitrateFull(s.Bitrate)
			})))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Pixel Format:",
			getField(state.compareFile1, func(s *videoSource) string { return s.PixelFormat }),
			getField(state.compareFile2, func(s *videoSource) string { return s.PixelFormat })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Color Space:",
			getField(state.compareFile1, func(s *videoSource) string { return s.ColorSpace }),
			getField(state.compareFile2, func(s *videoSource) string { return s.ColorSpace })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Color Range:",
			getField(state.compareFile1, func(s *videoSource) string { return s.ColorRange }),
			getField(state.compareFile2, func(s *videoSource) string { return s.ColorRange })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Field Order:",
			getField(state.compareFile1, func(s *videoSource) string { return s.FieldOrder }),
			getField(state.compareFile2, func(s *videoSource) string { return s.FieldOrder })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"GOP Size:",
			getField(state.compareFile1, func(s *videoSource) string { return fmt.Sprintf("%d", s.GOPSize) }),
			getField(state.compareFile2, func(s *videoSource) string { return fmt.Sprintf("%d", s.GOPSize) })))

		// Audio section
		comparisonText.WriteString("\n━━━ AUDIO ━━━\n")
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Codec:",
			getField(state.compareFile1, func(s *videoSource) string { return s.AudioCodec }),
			getField(state.compareFile2, func(s *videoSource) string { return s.AudioCodec })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Bitrate:",
			getField(state.compareFile1, func(s *videoSource) string { return formatBitrateFull(s.AudioBitrate) }),
			getField(state.compareFile2, func(s *videoSource) string { return formatBitrateFull(s.AudioBitrate) })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Sample Rate:",
			getField(state.compareFile1, func(s *videoSource) string { return fmt.Sprintf("%d Hz", s.AudioRate) }),
			getField(state.compareFile2, func(s *videoSource) string { return fmt.Sprintf("%d Hz", s.AudioRate) })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Channels:",
			getField(state.compareFile1, func(s *videoSource) string { return fmt.Sprintf("%d", s.Channels) }),
			getField(state.compareFile2, func(s *videoSource) string { return fmt.Sprintf("%d", s.Channels) })))

		// Other section
		comparisonText.WriteString("\n━━━ OTHER ━━━\n")
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Duration:",
			getField(state.compareFile1, func(s *videoSource) string { return s.DurationString() }),
			getField(state.compareFile2, func(s *videoSource) string { return s.DurationString() })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"SAR (Pixel Aspect):",
			getField(state.compareFile1, func(s *videoSource) string { return s.SampleAspectRatio }),
			getField(state.compareFile2, func(s *videoSource) string { return s.SampleAspectRatio })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Chapters:",
			getField(state.compareFile1, func(s *videoSource) string { return fmt.Sprintf("%v", s.HasChapters) }),
			getField(state.compareFile2, func(s *videoSource) string { return fmt.Sprintf("%v", s.HasChapters) })))
		comparisonText.WriteString(fmt.Sprintf("%-25s | %-20s | %s\n",
			"Metadata:",
			getField(state.compareFile1, func(s *videoSource) string { return fmt.Sprintf("%v", s.HasMetadata) }),
			getField(state.compareFile2, func(s *videoSource) string { return fmt.Sprintf("%v", s.HasMetadata) })))

		comparisonText.WriteString("\n═══════════════════════════════════════════════════════════════════════\n")

		state.window.Clipboard().SetContent(comparisonText.String())
		dialog.ShowInformation("Copied", "Comparison metadata copied to clipboard", state.window)
	})
	copyComparisonBtn.Importance = widget.LowImportance

	// Clear All button
	clearAllBtn := widget.NewButton("Clear All", func() {
		state.compareFile1 = nil
		state.compareFile2 = nil
		state.showCompareView()
	})
	clearAllBtn.Importance = widget.LowImportance

	instructionsRow := container.NewBorder(nil, nil, nil, container.NewHBox(fullscreenBtn, copyComparisonBtn, clearAllBtn), instructions)

	// File labels
	file1Label := widget.NewLabel("File 1: Not loaded")
	file1Label.TextStyle = fyne.TextStyle{Bold: true}

	file2Label := widget.NewLabel("File 2: Not loaded")
	file2Label.TextStyle = fyne.TextStyle{Bold: true}

	// Video player containers
	file1VideoContainer := container.NewMax()
	file2VideoContainer := container.NewMax()

	// Initialize with placeholders
	file1VideoContainer.Objects = []fyne.CanvasObject{container.NewCenter(widget.NewLabel("No video loaded"))}
	file2VideoContainer.Objects = []fyne.CanvasObject{container.NewCenter(widget.NewLabel("No video loaded"))}

	// Info labels
	file1Info := widget.NewLabel("No file loaded")
	file1Info.Wrapping = fyne.TextWrapWord
	file1Info.TextStyle = fyne.TextStyle{} // non-selectable label

	file2Info := widget.NewLabel("No file loaded")
	file2Info.Wrapping = fyne.TextWrapWord
	file2Info.TextStyle = fyne.TextStyle{} // non-selectable label

	// Helper function to format metadata (optionally comparing to a reference video)
	formatMetadata := func(src *videoSource, ref *videoSource) string {
		var (
			fileSize       = "Unknown"
			refSize  int64 = 0
		)
		if fi, err := os.Stat(src.Path); err == nil {
			if ref != nil {
				if rfi, err := os.Stat(ref.Path); err == nil {
					refSize = rfi.Size()
				}
			}
			if refSize > 0 {
				fileSize = utils.DeltaBytes(fi.Size(), refSize)
			} else {
				fileSize = utils.FormatBytes(fi.Size())
			}
		}

		var (
			bitrateStr = "--"
			refBitrate = 0
		)
		if ref != nil {
			refBitrate = ref.Bitrate
		}
		if src.Bitrate > 0 {
			if refBitrate > 0 {
				bitrateStr = utils.DeltaBitrate(src.Bitrate, refBitrate)
			} else {
				bitrateStr = formatBitrateFull(src.Bitrate)
			}
		}

		return fmt.Sprintf(
			"━━━ FILE INFO ━━━\n"+
				"Path: %s\n"+
				"File Size: %s\n"+
				"Format Family: %s\n"+
				"\n━━━ VIDEO ━━━\n"+
				"Codec: %s\n"+
				"Resolution: %dx%d\n"+
				"Aspect Ratio: %s\n"+
				"Frame Rate: %.2f fps\n"+
				"Bitrate: %s\n"+
				"Pixel Format: %s\n"+
				"Color Space: %s\n"+
				"Color Range: %s\n"+
				"Field Order: %s\n"+
				"GOP Size: %d\n"+
				"\n━━━ AUDIO ━━━\n"+
				"Codec: %s\n"+
				"Bitrate: %s\n"+
				"Sample Rate: %d Hz\n"+
				"Channels: %d\n"+
				"\n━━━ OTHER ━━━\n"+
				"Duration: %s\n"+
				"SAR (Pixel Aspect): %s\n"+
				"Chapters: %v\n"+
				"Metadata: %v",
			filepath.Base(src.Path),
			fileSize,
			src.Format,
			src.VideoCodec,
			src.Width, src.Height,
			src.AspectRatioString(),
			src.FrameRate,
			bitrateStr,
			src.PixelFormat,
			src.ColorSpace,
			src.ColorRange,
			src.FieldOrder,
			src.GOPSize,
			src.AudioCodec,
			formatBitrate(src.AudioBitrate),
			src.AudioRate,
			src.Channels,
			src.DurationString(),
			src.SampleAspectRatio,
			src.HasChapters,
			src.HasMetadata,
		)
	}

	// Helper to truncate filename if too long
	truncateFilename := func(filename string, maxLen int) string {
		if len(filename) <= maxLen {
			return filename
		}
		// Keep extension visible
		ext := filepath.Ext(filename)
		nameWithoutExt := strings.TrimSuffix(filename, ext)

		// If extension is too long, just truncate the whole thing
		if len(ext) > 10 {
			return filename[:maxLen-3] + "..."
		}

		// Truncate name but keep extension
		availableLen := maxLen - len(ext) - 3 // 3 for "..."
		if availableLen < 1 {
			return filename[:maxLen-3] + "..."
		}
		return nameWithoutExt[:availableLen] + "..." + ext
	}

	// Helper to update file display
	updateFile1 := func() {
		if state.compareFile1 != nil {
			filename := filepath.Base(state.compareFile1.Path)
			displayName := truncateFilename(filename, 35)
			file1Label.SetText(fmt.Sprintf("File 1: %s", displayName))
			file1Info.SetText(formatMetadata(state.compareFile1, state.compareFile2))
			// Build video player with compact size for side-by-side
			file1VideoContainer.Objects = []fyne.CanvasObject{
				buildVideoPane(state, fyne.NewSize(320, 180), state.compareFile1, nil),
			}
			file1VideoContainer.Refresh()
		} else {
			file1Label.SetText("File 1: Not loaded")
			file1Info.SetText("No file loaded")
			file1VideoContainer.Objects = []fyne.CanvasObject{
				container.NewCenter(widget.NewLabel("No video loaded")),
			}
			file1VideoContainer.Refresh()
		}
	}

	updateFile2 := func() {
		if state.compareFile2 != nil {
			filename := filepath.Base(state.compareFile2.Path)
			displayName := truncateFilename(filename, 35)
			file2Label.SetText(fmt.Sprintf("File 2: %s", displayName))
			file2Info.SetText(formatMetadata(state.compareFile2, state.compareFile1))
			// Build video player with compact size for side-by-side
			file2VideoContainer.Objects = []fyne.CanvasObject{
				buildVideoPane(state, fyne.NewSize(320, 180), state.compareFile2, nil),
			}
			file2VideoContainer.Refresh()
		} else {
			file2Label.SetText("File 2: Not loaded")
			file2Info.SetText("No file loaded")
			file2VideoContainer.Objects = []fyne.CanvasObject{
				container.NewCenter(widget.NewLabel("No video loaded")),
			}
			file2VideoContainer.Refresh()
		}
	}

	// Initialize with any already-loaded files
	updateFile1()
	updateFile2()

	file1SelectBtn := widget.NewButton("Load File 1", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			path := reader.URI().Path()
			reader.Close()

			src, err := probeVideo(path)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to load video: %w", err), state.window)
				return
			}

			state.compareFile1 = src
			updateFile1()
			logging.Debug(logging.CatModule, "loaded compare file 1: %s", path)
		}, state.window)
	})

	file2SelectBtn := widget.NewButton("Load File 2", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			path := reader.URI().Path()
			reader.Close()

			src, err := probeVideo(path)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to load video: %w", err), state.window)
				return
			}

			state.compareFile2 = src
			updateFile2()
			logging.Debug(logging.CatModule, "loaded compare file 2: %s", path)
		}, state.window)
	})

	// File 1 action buttons
	file1CopyBtn := widget.NewButton("Copy Metadata", func() {
		if state.compareFile1 == nil {
			return
		}
		metadata := formatMetadata(state.compareFile1, state.compareFile2)
		state.window.Clipboard().SetContent(metadata)
		dialog.ShowInformation("Copied", "Metadata copied to clipboard", state.window)
	})
	file1CopyBtn.Importance = widget.LowImportance

	file1ClearBtn := widget.NewButton("Clear", func() {
		state.compareFile1 = nil
		updateFile1()
	})
	file1ClearBtn.Importance = widget.LowImportance

	// File 2 action buttons
	file2CopyBtn := widget.NewButton("Copy Metadata", func() {
		if state.compareFile2 == nil {
			return
		}
		metadata := formatMetadata(state.compareFile2, state.compareFile1)
		state.window.Clipboard().SetContent(metadata)
		dialog.ShowInformation("Copied", "Metadata copied to clipboard", state.window)
	})
	file2CopyBtn.Importance = widget.LowImportance

	file2ClearBtn := widget.NewButton("Clear", func() {
		state.compareFile2 = nil
		updateFile2()
	})
	file2ClearBtn.Importance = widget.LowImportance

	// File 1 header (label + buttons)
	file1Header := container.NewVBox(
		file1Label,
		container.NewHBox(file1SelectBtn, file1CopyBtn, file1ClearBtn),
	)

	// File 2 header (label + buttons)
	file2Header := container.NewVBox(
		file2Label,
		container.NewHBox(file2SelectBtn, file2CopyBtn, file2ClearBtn),
	)

	// Scrollable metadata area for file 1 - use smaller minimum
	file1InfoScroll := container.NewVScroll(file1Info)
	file1InfoScroll.SetMinSize(fyne.NewSize(250, 150))

	// Scrollable metadata area for file 2 - use smaller minimum
	file2InfoScroll := container.NewVScroll(file2Info)
	file2InfoScroll.SetMinSize(fyne.NewSize(250, 150))

	// File 1 column: header, video player, metadata (using Border to make metadata expand)
	file1Column := container.NewBorder(
		container.NewVBox(
			file1Header,
			widget.NewSeparator(),
			file1VideoContainer,
			widget.NewSeparator(),
		),
		nil, nil, nil,
		file1InfoScroll,
	)

	// File 2 column: header, video player, metadata (using Border to make metadata expand)
	file2Column := container.NewBorder(
		container.NewVBox(
			file2Header,
			widget.NewSeparator(),
			file2VideoContainer,
			widget.NewSeparator(),
		),
		nil, nil, nil,
		file2InfoScroll,
	)

	// Main content: instructions at top, then two columns side by side
	content := container.NewBorder(
		container.NewVBox(instructionsRow, widget.NewSeparator()),
		nil, nil, nil,
		container.NewGridWithColumns(2, file1Column, file2Column),
	)

	return container.NewBorder(topBar, bottomBar, nil, nil, content)
}

// buildInspectView creates the UI for inspecting a single video with player
func buildInspectView(state *appState) fyne.CanvasObject {
	inspectColor := moduleColor("inspect")

	// Back button
	backBtn := widget.NewButton("< INSPECT", func() {
		state.showMainMenu()
	})
	backBtn.Importance = widget.LowImportance

	// Top bar with module color
	queueBtn := widget.NewButton("View Queue", func() {
		state.showQueue()
	})
	state.queueBtn = queueBtn
	state.updateQueueButtonLabel()
	topBar := ui.TintedBar(inspectColor, container.NewHBox(backBtn, layout.NewSpacer(), queueBtn))
	bottomBar := moduleFooter(inspectColor, layout.NewSpacer(), state.statsBar)

	// Instructions
	instructions := widget.NewLabel("Load a video to inspect its properties and preview playback. Drag a video here or use the button below.")
	instructions.Wrapping = fyne.TextWrapWord
	instructions.Alignment = fyne.TextAlignCenter

	// Clear button
	clearBtn := widget.NewButton("Clear", func() {
		state.inspectFile = nil
		state.showInspectView()
	})
	clearBtn.Importance = widget.LowImportance

	instructionsRow := container.NewBorder(nil, nil, nil, nil, instructions)

	// File label
	fileLabel := widget.NewLabel("No file loaded")
	fileLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Metadata text
	metadataText := widget.NewLabel("No file loaded")
	metadataText.Wrapping = fyne.TextWrapWord

	// Metadata scroll
	metadataScroll := container.NewScroll(metadataText)
	metadataScroll.SetMinSize(fyne.NewSize(400, 200))

	// Helper function to format metadata
	formatMetadata := func(src *videoSource) string {
		fileSize := "Unknown"
		if fi, err := os.Stat(src.Path); err == nil {
			fileSize = utils.FormatBytes(fi.Size())
		}

		metadata := fmt.Sprintf(
			"━━━ FILE INFO ━━━\n"+
				"Path: %s\n"+
				"File Size: %s\n"+
				"Format Family: %s\n"+
				"\n━━━ VIDEO ━━━\n"+
				"Codec: %s\n"+
				"Resolution: %dx%d\n"+
				"Aspect Ratio: %s\n"+
				"Frame Rate: %.2f fps\n"+
				"Bitrate: %s\n"+
				"Pixel Format: %s\n"+
				"Color Space: %s\n"+
				"Color Range: %s\n"+
				"Field Order: %s\n"+
				"GOP Size: %d\n"+
				"\n━━━ AUDIO ━━━\n"+
				"Codec: %s\n"+
				"Bitrate: %s\n"+
				"Sample Rate: %d Hz\n"+
				"Channels: %d\n"+
				"\n━━━ OTHER ━━━\n"+
				"Duration: %s\n"+
				"SAR (Pixel Aspect): %s\n"+
				"Chapters: %v\n"+
				"Metadata: %v",
			filepath.Base(src.Path),
			fileSize,
			src.Format,
			src.VideoCodec,
			src.Width, src.Height,
			src.AspectRatioString(),
			src.FrameRate,
			formatBitrateFull(src.Bitrate),
			src.PixelFormat,
			src.ColorSpace,
			src.ColorRange,
			src.FieldOrder,
			src.GOPSize,
			src.AudioCodec,
			formatBitrateFull(src.AudioBitrate),
			src.AudioRate,
			src.Channels,
			src.DurationString(),
			src.SampleAspectRatio,
			src.HasChapters,
			src.HasMetadata,
		)

		// Add interlacing detection results if available
		if state.inspectInterlaceAnalyzing {
			metadata += "\n\n━━━ INTERLACING DETECTION ━━━\n"
			metadata += "Analyzing... (first 500 frames)"
		} else if state.inspectInterlaceResult != nil {
			result := state.inspectInterlaceResult
			metadata += "\n\n━━━ INTERLACING DETECTION ━━━\n"
			metadata += fmt.Sprintf("Status: %s\n", result.Status)
			metadata += fmt.Sprintf("Interlaced Frames: %.1f%%\n", result.InterlacedPercent)
			metadata += fmt.Sprintf("Field Order: %s\n", result.FieldOrder)
			metadata += fmt.Sprintf("Confidence: %s\n", result.Confidence)
			metadata += fmt.Sprintf("Recommendation: %s\n", result.Recommendation)
			metadata += fmt.Sprintf("\nFrame Counts:\n")
			metadata += fmt.Sprintf("  Progressive: %d\n", result.Progressive)
			metadata += fmt.Sprintf("  Top Field First: %d\n", result.TFF)
			metadata += fmt.Sprintf("  Bottom Field First: %d\n", result.BFF)
			metadata += fmt.Sprintf("  Undetermined: %d\n", result.Undetermined)
			metadata += fmt.Sprintf("  Total Analyzed: %d", result.TotalFrames)
		}

		return metadata
	}

	// Video player container
	var videoContainer fyne.CanvasObject = container.NewCenter(widget.NewLabel("No video loaded"))

	// Update display function
	updateDisplay := func() {
		if state.inspectFile != nil {
			filename := filepath.Base(state.inspectFile.Path)
			// Truncate if too long
			if len(filename) > 50 {
				ext := filepath.Ext(filename)
				nameWithoutExt := strings.TrimSuffix(filename, ext)
				if len(ext) > 10 {
					filename = filename[:47] + "..."
				} else {
					availableLen := 47 - len(ext)
					if availableLen < 1 {
						filename = filename[:47] + "..."
					} else {
						filename = nameWithoutExt[:availableLen] + "..." + ext
					}
				}
			}
			fileLabel.SetText(fmt.Sprintf("File: %s", filename))
			metadataText.SetText(formatMetadata(state.inspectFile))

			// Build video player
			videoContainer = buildVideoPane(state, fyne.NewSize(640, 360), state.inspectFile, nil)
		} else {
			fileLabel.SetText("No file loaded")
			metadataText.SetText("No file loaded")
			videoContainer = container.NewCenter(widget.NewLabel("No video loaded"))
		}
	}

	// Initialize display
	updateDisplay()

	// Load button
	loadBtn := widget.NewButton("Load Video", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			path := reader.URI().Path()
			reader.Close()

			src, err := probeVideo(path)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to load video: %w", err), state.window)
				return
			}

			state.inspectFile = src
			state.inspectInterlaceResult = nil
			state.inspectInterlaceAnalyzing = true
			state.showInspectView()
			logging.Debug(logging.CatModule, "loaded inspect file: %s", path)

			// Auto-run interlacing detection in background
			go func() {
				detector := interlace.NewDetector(platformConfig.FFmpegPath, platformConfig.FFprobePath)
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()

				result, err := detector.QuickAnalyze(ctx, path)

				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					state.inspectInterlaceAnalyzing = false
					if err != nil {
						logging.Debug(logging.CatSystem, "auto interlacing analysis failed: %v", err)
						state.inspectInterlaceResult = nil
					} else {
						state.inspectInterlaceResult = result
						logging.Debug(logging.CatSystem, "auto interlacing analysis complete: %s", result.Status)
					}
					state.showInspectView() // Refresh to show results
				}, false)
			}()
		}, state.window)
	})

	// Copy metadata button
	copyBtn := widget.NewButton("Copy Metadata", func() {
		if state.inspectFile == nil {
			return
		}
		metadata := formatMetadata(state.inspectFile)
		state.window.Clipboard().SetContent(metadata)
		dialog.ShowInformation("Copied", "Metadata copied to clipboard", state.window)
	})
	copyBtn.Importance = widget.LowImportance

	logPath := ""
	if state.inspectFile != nil {
		base := strings.TrimSuffix(filepath.Base(state.inspectFile.Path), filepath.Ext(state.inspectFile.Path))
		p := filepath.Join(getLogsDir(), base+conversionLogSuffix)
		if _, err := os.Stat(p); err == nil {
			logPath = p
		}
	}
	viewLogBtn := widget.NewButton("View Conversion Log", func() {
		if logPath == "" {
			dialog.ShowInformation("No Log", "No conversion log found for this file.", state.window)
			return
		}
		state.openLogViewer("Conversion Log", logPath, false)
	})
	viewLogBtn.Importance = widget.LowImportance
	if logPath == "" {
		viewLogBtn.Disable()
	}

	// Action buttons
	actionButtons := container.NewHBox(loadBtn, copyBtn, viewLogBtn, clearBtn)

	// Main layout: left side is video player, right side is metadata
	leftColumn := container.NewBorder(
		fileLabel,
		nil, nil, nil,
		videoContainer,
	)

	rightColumn := container.NewBorder(
		widget.NewLabel("Metadata:"),
		nil, nil, nil,
		metadataScroll,
	)

	// Bottom bar with module color
	bottomBar = moduleFooter(inspectColor, layout.NewSpacer(), state.statsBar)

	// Main content
	content := container.NewBorder(
		container.NewVBox(instructionsRow, actionButtons, widget.NewSeparator()),
		nil, nil, nil,
		container.NewGridWithColumns(2, leftColumn, rightColumn),
	)

	return container.NewBorder(topBar, bottomBar, nil, nil, content)
}

// buildThumbView creates the thumbnail generation UI
func buildThumbView(state *appState) fyne.CanvasObject {
	thumbColor := moduleColor("thumb")

	// Back button
	backBtn := widget.NewButton("< THUMBNAILS", func() {
		state.showMainMenu()
	})
	backBtn.Importance = widget.LowImportance

	// Top bar with module color
	queueBtn := widget.NewButton("View Queue", func() {
		state.showQueue()
	})
	state.queueBtn = queueBtn
	state.updateQueueButtonLabel()
	topBar := ui.TintedBar(thumbColor, container.NewHBox(backBtn, layout.NewSpacer(), queueBtn))

	// Instructions
	instructions := widget.NewLabel("Generate thumbnails from a video file. Load a video and configure settings.")
	instructions.Wrapping = fyne.TextWrapWord
	instructions.Alignment = fyne.TextAlignCenter

	// Initialize state defaults
	if state.thumbCount == 0 {
		state.thumbCount = 24 // Default to 24 thumbnails (good for contact sheets)
	}
	if state.thumbWidth == 0 {
		state.thumbWidth = 320
	}
	if state.thumbColumns == 0 {
		state.thumbColumns = 4 // 4 columns works well for widescreen videos
	}
	if state.thumbRows == 0 {
		state.thumbRows = 6 // 4x6 = 24 thumbnails
	}

	// File label and video preview
	fileLabel := widget.NewLabel("No file loaded")
	fileLabel.TextStyle = fyne.TextStyle{Bold: true}

	var videoContainer fyne.CanvasObject
	if state.thumbFile != nil {
		fileLabel.SetText(fmt.Sprintf("File: %s", filepath.Base(state.thumbFile.Path)))
		videoContainer = buildVideoPane(state, fyne.NewSize(640, 360), state.thumbFile, nil)
	} else {
		videoContainer = container.NewCenter(widget.NewLabel("No video loaded"))
	}

	// Load button
	loadBtn := widget.NewButton("Load Video", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			path := reader.URI().Path()
			reader.Close()

			src, err := probeVideo(path)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to load video: %w", err), state.window)
				return
			}

			state.thumbFile = src
			state.showThumbView()
			logging.Debug(logging.CatModule, "loaded thumbnail file: %s", path)
		}, state.window)
	})

	// Clear button
	clearBtn := widget.NewButton("Clear", func() {
		state.thumbFile = nil
		state.showThumbView()
	})
	clearBtn.Importance = widget.LowImportance

	// Contact sheet checkbox
	contactSheetCheck := widget.NewCheck("Generate Contact Sheet (single image)", func(checked bool) {
		state.thumbContactSheet = checked
		state.showThumbView()
	})
	contactSheetCheck.Checked = state.thumbContactSheet

	// Conditional settings based on contact sheet mode
	var settingsOptions fyne.CanvasObject
	if state.thumbContactSheet {
		// Contact sheet mode: show columns and rows
		colLabel := widget.NewLabel(fmt.Sprintf("Columns: %d", state.thumbColumns))
		rowLabel := widget.NewLabel(fmt.Sprintf("Rows: %d", state.thumbRows))

		totalThumbs := state.thumbColumns * state.thumbRows
		totalLabel := widget.NewLabel(fmt.Sprintf("Total thumbnails: %d", totalThumbs))
		totalLabel.TextStyle = fyne.TextStyle{Italic: true}

		colSlider := widget.NewSlider(2, 12)
		colSlider.Value = float64(state.thumbColumns)
		colSlider.Step = 1
		colSlider.OnChanged = func(val float64) {
			state.thumbColumns = int(val)
			colLabel.SetText(fmt.Sprintf("Columns: %d", int(val)))
			totalLabel.SetText(fmt.Sprintf("Total thumbnails: %d", state.thumbColumns*state.thumbRows))
		}

		rowSlider := widget.NewSlider(2, 12)
		rowSlider.Value = float64(state.thumbRows)
		rowSlider.Step = 1
		rowSlider.OnChanged = func(val float64) {
			state.thumbRows = int(val)
			rowLabel.SetText(fmt.Sprintf("Rows: %d", int(val)))
			totalLabel.SetText(fmt.Sprintf("Total thumbnails: %d", state.thumbColumns*state.thumbRows))
		}

		settingsOptions = container.NewVBox(
			widget.NewSeparator(),
			widget.NewLabel("Contact Sheet Grid:"),
			colLabel,
			colSlider,
			rowLabel,
			rowSlider,
			totalLabel,
		)
	} else {
		// Individual thumbnails mode: show count and width
		countLabel := widget.NewLabel(fmt.Sprintf("Thumbnail Count: %d", state.thumbCount))
		countSlider := widget.NewSlider(3, 50)
		countSlider.Value = float64(state.thumbCount)
		countSlider.Step = 1
		countSlider.OnChanged = func(val float64) {
			state.thumbCount = int(val)
			countLabel.SetText(fmt.Sprintf("Thumbnail Count: %d", int(val)))
		}

		widthLabel := widget.NewLabel(fmt.Sprintf("Thumbnail Width: %d px", state.thumbWidth))
		widthSlider := widget.NewSlider(160, 640)
		widthSlider.Value = float64(state.thumbWidth)
		widthSlider.Step = 32
		widthSlider.OnChanged = func(val float64) {
			state.thumbWidth = int(val)
			widthLabel.SetText(fmt.Sprintf("Thumbnail Width: %d px", int(val)))
		}

		settingsOptions = container.NewVBox(
			widget.NewSeparator(),
			widget.NewLabel("Individual Thumbnails:"),
			countLabel,
			countSlider,
			widthLabel,
			widthSlider,
		)
	}

	// Helper function to create thumbnail job
	createThumbJob := func() *queue.Job {
		// Create output directory in same folder as video
		videoDir := filepath.Dir(state.thumbFile.Path)
		videoBaseName := strings.TrimSuffix(filepath.Base(state.thumbFile.Path), filepath.Ext(state.thumbFile.Path))
		outputDir := filepath.Join(videoDir, fmt.Sprintf("%s_thumbnails", videoBaseName))

		// Configure based on mode
		var count, width int
		var description string
		if state.thumbContactSheet {
			// Contact sheet: count is determined by grid, use larger width for analyzable screenshots
			count = state.thumbColumns * state.thumbRows
			width = 280 // Larger width for contact sheets to make screenshots analyzable (4x8 grid = ~1144x1416)
			description = fmt.Sprintf("Contact sheet: %dx%d grid (%d thumbnails)", state.thumbColumns, state.thumbRows, count)
		} else {
			// Individual thumbnails: use user settings
			count = state.thumbCount
			width = state.thumbWidth
			description = fmt.Sprintf("%d individual thumbnails (%dpx width)", count, width)
		}

		return &queue.Job{
			Type:        queue.JobTypeThumb,
			Title:       "Thumbnails: " + filepath.Base(state.thumbFile.Path),
			Description: description,
			InputFile:   state.thumbFile.Path,
			OutputFile:  outputDir,
			Config: map[string]interface{}{
				"inputPath":    state.thumbFile.Path,
				"outputDir":    outputDir,
				"count":        float64(count),
				"width":        float64(width),
				"contactSheet": state.thumbContactSheet,
				"columns":      float64(state.thumbColumns),
				"rows":         float64(state.thumbRows),
			},
		}
	}

	// Generate Now button - adds to queue and starts it
	generateNowBtn := widget.NewButton("GENERATE NOW", func() {
		if state.thumbFile == nil {
			dialog.ShowInformation("No Video", "Please load a video file first.", state.window)
			return
		}

		if state.jobQueue == nil {
			dialog.ShowInformation("Queue", "Queue not initialized.", state.window)
			return
		}

		job := createThumbJob()
		state.jobQueue.Add(job)

		// Start the queue if not already running
		if !state.jobQueue.IsRunning() {
			state.jobQueue.Start()
			logging.Debug(logging.CatSystem, "started queue from Generate Now")
		}

		dialog.ShowInformation("Thumbnails", "Thumbnail generation started! View progress in Job Queue.", state.window)
	})
	generateNowBtn.Importance = widget.HighImportance

	if state.thumbFile == nil {
		generateNowBtn.Disable()
	}

	// Add to Queue button
	addQueueBtn := widget.NewButton("Add to Queue", func() {
		if state.thumbFile == nil {
			dialog.ShowInformation("No Video", "Please load a video file first.", state.window)
			return
		}

		if state.jobQueue == nil {
			dialog.ShowInformation("Queue", "Queue not initialized.", state.window)
			return
		}

		job := createThumbJob()
		state.jobQueue.Add(job)

		dialog.ShowInformation("Queue", "Thumbnail job added to queue!", state.window)
	})
	addQueueBtn.Importance = widget.MediumImportance

	if state.thumbFile == nil {
		addQueueBtn.Disable()
	}

	// View Queue button
	viewQueueBtn := widget.NewButton("View Queue", func() {
		state.showQueue()
	})
	viewQueueBtn.Importance = widget.MediumImportance

	// View Results button - shows output folder if it exists
	viewResultsBtn := widget.NewButton("View Results", func() {
		if state.thumbFile == nil {
			dialog.ShowInformation("No Video", "Load a video first to locate results.", state.window)
			return
		}

		videoDir := filepath.Dir(state.thumbFile.Path)
		videoBaseName := strings.TrimSuffix(filepath.Base(state.thumbFile.Path), filepath.Ext(state.thumbFile.Path))
		outputDir := filepath.Join(videoDir, fmt.Sprintf("%s_thumbnails", videoBaseName))

		// Check if output exists
		if _, err := os.Stat(outputDir); os.IsNotExist(err) {
			dialog.ShowInformation("No Results", "No generated thumbnails found. Generate thumbnails first.", state.window)
			return
		}

		// If contact sheet mode, try to show the contact sheet image
		if state.thumbContactSheet {
			contactSheetPath := filepath.Join(outputDir, "contact_sheet.jpg")
			if _, err := os.Stat(contactSheetPath); err == nil {
				// Show contact sheet in a dialog
				go func() {
					img := canvas.NewImageFromFile(contactSheetPath)
					img.FillMode = canvas.ImageFillContain
					// Adaptive size for small screens - use scrollable dialog
					img.SetMinSize(fyne.NewSize(640, 480))

					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						// Wrap in scroll container for large contact sheets
						scroll := container.NewScroll(img)
						d := dialog.NewCustom("Contact Sheet", "Close", scroll, state.window)
						// Adaptive dialog size that fits on 1280x768 screens
						d.Resize(fyne.NewSize(700, 600))
						d.Show()
					}, false)
				}()
				return
			}
		}

		// Otherwise, open folder
		openFolder(outputDir)
	})
	viewResultsBtn.Importance = widget.MediumImportance
	if state.thumbFile == nil {
		viewResultsBtn.Disable()
	}

	// Settings panel
	settingsPanel := container.NewVBox(
		widget.NewLabel("Settings:"),
		widget.NewSeparator(),
		contactSheetCheck,
		settingsOptions,
		widget.NewSeparator(),
		generateNowBtn,
		addQueueBtn,
		viewQueueBtn,
		viewResultsBtn,
	)

	// Main content - split layout with preview on left, settings on right
	leftColumn := container.NewVBox(
		videoContainer,
	)

	rightColumn := container.NewVBox(
		settingsPanel,
	)

	mainContent := container.NewHSplit(leftColumn, rightColumn)
	mainContent.Offset = 0.55 // Give more space to preview

	content := container.NewBorder(
		container.NewVBox(instructions, widget.NewSeparator(), fileLabel, container.NewHBox(loadBtn, clearBtn)),
		nil,
		nil,
		nil,
		mainContent,
	)

	bottomBar := moduleFooter(thumbColor, layout.NewSpacer(), state.statsBar)

	return container.NewBorder(topBar, bottomBar, nil, nil, content)
}

// buildPlayerView creates the VT_Player UI
func buildPlayerView(state *appState) fyne.CanvasObject {
	playerColor := moduleColor("player")

	// Back button
	backBtn := widget.NewButton("< PLAYER", func() {
		state.showMainMenu()
	})
	backBtn.Importance = widget.LowImportance

	// Top bar with module color
	queueBtn := widget.NewButton("View Queue", func() {
		state.showQueue()
	})
	state.queueBtn = queueBtn
	state.updateQueueButtonLabel()
	topBar := ui.TintedBar(playerColor, container.NewHBox(backBtn, layout.NewSpacer(), queueBtn))

	// Instructions
	instructions := widget.NewLabel("VT_Player - Advanced video playback with frame-accurate seeking and analysis tools.")
	instructions.Wrapping = fyne.TextWrapWord
	instructions.Alignment = fyne.TextAlignCenter

	// File label
	fileLabel := widget.NewLabel("No file loaded")
	fileLabel.TextStyle = fyne.TextStyle{Bold: true}

	var videoContainer fyne.CanvasObject
	if state.playerFile != nil {
		fileLabel.SetText(fmt.Sprintf("File: %s", filepath.Base(state.playerFile.Path)))
		videoContainer = buildVideoPane(state, fyne.NewSize(960, 540), state.playerFile, nil)
	} else {
		videoContainer = container.NewCenter(widget.NewLabel("No video loaded"))
	}

	// Load button
	loadBtn := widget.NewButton("Load Video", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()

			path := reader.URI().Path()
			go func() {
				src, err := probeVideo(path)
				if err != nil {
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						dialog.ShowError(err, state.window)
					}, false)
					return
				}

				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					state.playerFile = src
					state.showPlayerView()
				}, false)
			}()
		}, state.window)
	})
	loadBtn.Importance = widget.HighImportance

	// Main content
	mainContent := container.NewVBox(
		instructions,
		widget.NewSeparator(),
		fileLabel,
		loadBtn,
		videoContainer,
	)

	content := container.NewPadded(mainContent)
	bottomBar := moduleFooter(playerColor, layout.NewSpacer(), state.statsBar)

	return container.NewBorder(topBar, bottomBar, nil, nil, content)
}

// buildFiltersView creates the Filters module UI
func buildFiltersView(state *appState) fyne.CanvasObject {
	filtersColor := moduleColor("filters")

	// Back button
	backBtn := widget.NewButton("< FILTERS", func() {
		state.showMainMenu()
	})
	backBtn.Importance = widget.LowImportance

	// Queue button
	queueBtn := widget.NewButton("View Queue", func() {
		state.showQueue()
	})
	state.queueBtn = queueBtn
	state.updateQueueButtonLabel()

	// Top bar with module color
	topBar := ui.TintedBar(filtersColor, container.NewHBox(backBtn, layout.NewSpacer(), queueBtn))
	bottomBar := moduleFooter(filtersColor, layout.NewSpacer(), state.statsBar)

	// Instructions
	instructions := widget.NewLabel("Apply filters and color corrections to your video. Preview changes in real-time.")
	instructions.Wrapping = fyne.TextWrapWord
	instructions.Alignment = fyne.TextAlignCenter

	// Initialize state defaults
	if state.filterBrightness == 0 && state.filterContrast == 0 && state.filterSaturation == 0 {
		state.filterBrightness = 0.0 // -1.0 to 1.0
		state.filterContrast = 1.0   // 0.0 to 3.0
		state.filterSaturation = 1.0 // 0.0 to 3.0
		state.filterSharpness = 0.0  // 0.0 to 5.0
		state.filterDenoise = 0.0    // 0.0 to 10.0
	}

	// File label
	fileLabel := widget.NewLabel("No file loaded")
	fileLabel.TextStyle = fyne.TextStyle{Bold: true}

	var videoContainer fyne.CanvasObject
	if state.filtersFile != nil {
		fileLabel.SetText(fmt.Sprintf("File: %s", filepath.Base(state.filtersFile.Path)))
		videoContainer = buildVideoPane(state, fyne.NewSize(640, 360), state.filtersFile, nil)
	} else {
		videoContainer = container.NewCenter(widget.NewLabel("No video loaded"))
	}

	// Load button
	loadBtn := widget.NewButton("Load Video", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()

			path := reader.URI().Path()
			go func() {
				src, err := probeVideo(path)
				if err != nil {
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						dialog.ShowError(err, state.window)
					}, false)
					return
				}

				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					state.filtersFile = src
					state.showFiltersView()
				}, false)
			}()
		}, state.window)
	})
	loadBtn.Importance = widget.HighImportance

	// Navigation to Upscale module
	upscaleNavBtn := widget.NewButton("Send to Upscale →", func() {
		if state.filtersFile != nil {
			state.upscaleFile = state.filtersFile
			// TODO: Transfer active filter chain to upscale
			// state.upscaleFilterChain = state.filterActiveChain
		}
		state.showUpscaleView()
	})

	// Color Correction Section
	colorSection := widget.NewCard("Color Correction", "", container.NewVBox(
		widget.NewLabel("Adjust brightness, contrast, and saturation"),
		container.NewGridWithColumns(2,
			widget.NewLabel("Brightness:"),
			widget.NewSlider(-1.0, 1.0),
			widget.NewLabel("Contrast:"),
			widget.NewSlider(0.0, 3.0),
			widget.NewLabel("Saturation:"),
			widget.NewSlider(0.0, 3.0),
		),
	))

	// Enhancement Section
	enhanceSection := widget.NewCard("Enhancement", "", container.NewVBox(
		widget.NewLabel("Sharpen, blur, and denoise"),
		container.NewGridWithColumns(2,
			widget.NewLabel("Sharpness:"),
			widget.NewSlider(0.0, 5.0),
			widget.NewLabel("Denoise:"),
			widget.NewSlider(0.0, 10.0),
		),
	))

	// Transform Section
	transformSection := widget.NewCard("Transform", "", container.NewVBox(
		widget.NewLabel("Rotate and flip video"),
		container.NewGridWithColumns(2,
			widget.NewLabel("Rotation:"),
			widget.NewSelect([]string{"0°", "90°", "180°", "270°"}, func(s string) {}),
			widget.NewLabel("Flip Horizontal:"),
			widget.NewCheck("", func(b bool) { state.filterFlipH = b }),
			widget.NewLabel("Flip Vertical:"),
			widget.NewCheck("", func(b bool) { state.filterFlipV = b }),
		),
	))

	// Creative Effects Section
	creativeSection := widget.NewCard("Creative Effects", "", container.NewVBox(
		widget.NewLabel("Apply artistic effects"),
		widget.NewCheck("Grayscale", func(b bool) { state.filterGrayscale = b }),
	))

	// Apply button
	applyBtn := widget.NewButton("Apply Filters", func() {
		if state.filtersFile == nil {
			dialog.ShowInformation("No Video", "Please load a video first.", state.window)
			return
		}
		// TODO: Implement filter application
		dialog.ShowInformation("Coming Soon", "Filter application will be implemented soon.", state.window)
	})
	applyBtn.Importance = widget.HighImportance

	// Main content
	leftPanel := container.NewVBox(
		instructions,
		widget.NewSeparator(),
		fileLabel,
		loadBtn,
		upscaleNavBtn,
	)

	settingsPanel := container.NewVBox(
		colorSection,
		enhanceSection,
		transformSection,
		creativeSection,
		applyBtn,
	)

	settingsScroll := container.NewVScroll(settingsPanel)
	// Adaptive height for small screens - allow content to flow
	settingsScroll.SetMinSize(fyne.NewSize(350, 400))

	mainContent := container.NewHSplit(
		container.NewVBox(leftPanel, videoContainer),
		settingsScroll,
	)
	mainContent.SetOffset(0.55) // 55% for video preview, 45% for settings

	content := container.NewPadded(mainContent)

	return container.NewBorder(topBar, bottomBar, nil, nil, content)
}

// buildUpscaleView creates the Upscale module UI
func buildUpscaleView(state *appState) fyne.CanvasObject {
	upscaleColor := moduleColor("upscale")

	// Back button
	backBtn := widget.NewButton("< UPSCALE", func() {
		state.showMainMenu()
	})
	backBtn.Importance = widget.LowImportance

	// Queue button
	queueBtn := widget.NewButton("View Queue", func() {
		state.showQueue()
	})
	state.queueBtn = queueBtn
	state.updateQueueButtonLabel()

	// Top bar with module color
	topBar := ui.TintedBar(upscaleColor, container.NewHBox(backBtn, layout.NewSpacer(), queueBtn))
	bottomBar := moduleFooter(upscaleColor, layout.NewSpacer(), state.statsBar)

	// Instructions
	instructions := widget.NewLabel("Upscale your video to higher resolution using traditional or AI-powered methods.")
	instructions.Wrapping = fyne.TextWrapWord
	instructions.Alignment = fyne.TextAlignCenter

	// Initialize state defaults
	if state.upscaleMethod == "" {
		state.upscaleMethod = "lanczos" // Best general-purpose traditional method
	}
	if state.upscaleTargetRes == "" {
		state.upscaleTargetRes = "Match Source"
	}
	if state.upscaleAIModel == "" {
		state.upscaleAIModel = "realesrgan" // General purpose AI model
	}
	if state.upscaleFrameRate == "" {
		state.upscaleFrameRate = "Source"
	}

	// Check AI availability on first load
	if !state.upscaleAIAvailable {
		state.upscaleAIAvailable = checkAIUpscaleAvailable()
	}

	// File label
	fileLabel := widget.NewLabel("No file loaded")
	fileLabel.TextStyle = fyne.TextStyle{Bold: true}

	var videoContainer fyne.CanvasObject
	var sourceResLabel *widget.Label
	if state.upscaleFile != nil {
		fileLabel.SetText(fmt.Sprintf("File: %s", filepath.Base(state.upscaleFile.Path)))
		sourceResLabel = widget.NewLabel(fmt.Sprintf("Source: %dx%d", state.upscaleFile.Width, state.upscaleFile.Height))
		sourceResLabel.TextStyle = fyne.TextStyle{Italic: true}
		videoContainer = buildVideoPane(state, fyne.NewSize(640, 360), state.upscaleFile, nil)
	} else {
		sourceResLabel = widget.NewLabel("Source: N/A")
		sourceResLabel.TextStyle = fyne.TextStyle{Italic: true}
		videoContainer = container.NewCenter(widget.NewLabel("No video loaded"))
	}

	// Load button
	loadBtn := widget.NewButton("Load Video", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()

			path := reader.URI().Path()
			go func() {
				src, err := probeVideo(path)
				if err != nil {
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						dialog.ShowError(err, state.window)
					}, false)
					return
				}

				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					state.upscaleFile = src
					state.showUpscaleView()
				}, false)
			}()
		}, state.window)
	})
	loadBtn.Importance = widget.HighImportance

	// Navigation to Filters module
	filtersNavBtn := widget.NewButton("← Adjust Filters", func() {
		if state.upscaleFile != nil {
			state.filtersFile = state.upscaleFile
		}
		state.showFiltersView()
	})

	// Traditional Scaling Section
	methodLabel := widget.NewLabel(fmt.Sprintf("Method: %s", state.upscaleMethod))
	methodSelect := widget.NewSelect([]string{
		"lanczos",  // Sharp, best general purpose
		"bicubic",  // Smooth
		"spline",   // Balanced
		"bilinear", // Fast, lower quality
	}, func(s string) {
		state.upscaleMethod = s
		methodLabel.SetText(fmt.Sprintf("Method: %s", s))
	})
	methodSelect.SetSelected(state.upscaleMethod)

	methodInfo := widget.NewLabel("Lanczos: Sharp, best quality\nBicubic: Smooth\nSpline: Balanced\nBilinear: Fast")
	methodInfo.TextStyle = fyne.TextStyle{Italic: true}
	methodInfo.Wrapping = fyne.TextWrapWord

	traditionalSection := widget.NewCard("Traditional Scaling (FFmpeg)", "", container.NewVBox(
		widget.NewLabel("Classic upscaling methods - always available"),
		container.NewGridWithColumns(2,
			widget.NewLabel("Scaling Algorithm:"),
			methodSelect,
		),
		methodLabel,
		widget.NewSeparator(),
		methodInfo,
	))

	// Resolution Selection Section
	resLabel := widget.NewLabel(fmt.Sprintf("Target: %s", state.upscaleTargetRes))
	resSelect := widget.NewSelect([]string{
		"Match Source",
		"2X (relative)",
		"4X (relative)",
		"720p (1280x720)",
		"1080p (1920x1080)",
		"1440p (2560x1440)",
		"4K (3840x2160)",
		"8K (7680x4320)",
		"Custom",
	}, func(s string) {
		state.upscaleTargetRes = s
		resLabel.SetText(fmt.Sprintf("Target: %s", s))
	})
	resSelect.SetSelected(state.upscaleTargetRes)

	resolutionSection := widget.NewCard("Target Resolution", "", container.NewVBox(
		widget.NewLabel("Select output resolution"),
		container.NewGridWithColumns(2,
			widget.NewLabel("Resolution:"),
			resSelect,
		),
		resLabel,
		sourceResLabel,
	))

	// Frame Rate Section
	frameRateLabel := widget.NewLabel(fmt.Sprintf("Frame Rate: %s", state.upscaleFrameRate))
	frameRateSelect := widget.NewSelect([]string{"Source", "23.976", "24", "25", "29.97", "30", "50", "59.94", "60"}, func(s string) {
		state.upscaleFrameRate = s
		frameRateLabel.SetText(fmt.Sprintf("Frame Rate: %s", s))
	})
	frameRateSelect.SetSelected(state.upscaleFrameRate)

	motionInterpCheck := widget.NewCheck("Use Motion Interpolation (slower, smoother)", func(checked bool) {
		state.upscaleMotionInterpolation = checked
	})
	motionInterpCheck.SetChecked(state.upscaleMotionInterpolation)

	frameRateSection := widget.NewCard("Frame Rate", "", container.NewVBox(
		widget.NewLabel("Convert frame rate (optional)"),
		container.NewGridWithColumns(2,
			widget.NewLabel("Target FPS:"),
			frameRateSelect,
		),
		frameRateLabel,
		motionInterpCheck,
		widget.NewLabel("Motion interpolation creates smooth in-between frames"),
	))

	// AI Upscaling Section
	var aiSection *widget.Card
	if state.upscaleAIAvailable {
		aiModelSelect := widget.NewSelect([]string{
			"realesrgan (General Purpose)",
			"realesrgan-anime (Anime/Animation)",
		}, func(s string) {
			if strings.Contains(s, "anime") {
				state.upscaleAIModel = "realesrgan-anime"
			} else {
				state.upscaleAIModel = "realesrgan"
			}
		})
		if strings.Contains(state.upscaleAIModel, "anime") {
			aiModelSelect.SetSelected("realesrgan-anime (Anime/Animation)")
		} else {
			aiModelSelect.SetSelected("realesrgan (General Purpose)")
		}

		aiEnabledCheck := widget.NewCheck("Use AI Upscaling", func(checked bool) {
			state.upscaleAIEnabled = checked
		})
		aiEnabledCheck.SetChecked(state.upscaleAIEnabled)

		aiSection = widget.NewCard("AI Upscaling", "✓ Available", container.NewVBox(
			widget.NewLabel("Real-ESRGAN detected - enhanced quality available"),
			aiEnabledCheck,
			container.NewGridWithColumns(2,
				widget.NewLabel("AI Model:"),
				aiModelSelect,
			),
			widget.NewLabel("Note: AI upscaling is slower but produces higher quality results"),
		))
	} else {
		aiSection = widget.NewCard("AI Upscaling", "Not Available", container.NewVBox(
			widget.NewLabel("Real-ESRGAN not detected. Install for enhanced quality:"),
			widget.NewLabel("https://github.com/xinntao/Real-ESRGAN"),
			widget.NewLabel("Traditional scaling methods will be used."),
		))
	}

	// Filter Integration Section
	applyFiltersCheck := widget.NewCheck("Apply filters before upscaling", func(checked bool) {
		state.upscaleApplyFilters = checked
	})
	applyFiltersCheck.SetChecked(state.upscaleApplyFilters)

	filterIntegrationSection := widget.NewCard("Filter Integration", "", container.NewVBox(
		widget.NewLabel("Apply color correction and filters from Filters module"),
		applyFiltersCheck,
		widget.NewLabel("Filters will be applied before upscaling for best quality"),
	))

	// Helper function to create upscale job
	createUpscaleJob := func() (*queue.Job, error) {
		if state.upscaleFile == nil {
			return nil, fmt.Errorf("no video loaded")
		}

		// Parse target resolution (preserve aspect by default)
		targetWidth, targetHeight, preserveAspect, err := parseResolutionPreset(state.upscaleTargetRes, state.upscaleFile.Width, state.upscaleFile.Height)
		if err != nil {
			return nil, fmt.Errorf("invalid resolution: %w", err)
		}

		// Build output path
		videoDir := filepath.Dir(state.upscaleFile.Path)
		videoBaseName := strings.TrimSuffix(filepath.Base(state.upscaleFile.Path), filepath.Ext(state.upscaleFile.Path))
		slug := sanitizeForPath(state.upscaleTargetRes)
		if slug == "" {
			slug = "source"
		}
		outputPath := filepath.Join(videoDir, fmt.Sprintf("%s_upscaled_%s_%s.mkv",
			videoBaseName, slug, state.upscaleMethod))

		// Build description
		description := fmt.Sprintf("Upscale to %s using %s", state.upscaleTargetRes, state.upscaleMethod)
		if state.upscaleAIEnabled && state.upscaleAIAvailable {
			description += fmt.Sprintf(" + AI (%s)", state.upscaleAIModel)
		}

		desc := fmt.Sprintf("%s → %s", description, filepath.Base(outputPath))

		return &queue.Job{
			Type:        queue.JobTypeUpscale,
			Title:       "Upscale: " + filepath.Base(state.upscaleFile.Path),
			Description: desc,
			OutputFile:  outputPath,
			Config: map[string]interface{}{
				"inputPath":              state.upscaleFile.Path,
				"outputPath":             outputPath,
				"method":                 state.upscaleMethod,
				"targetWidth":            float64(targetWidth),
				"targetHeight":           float64(targetHeight),
				"preserveAR":             preserveAspect,
				"useAI":                  state.upscaleAIEnabled && state.upscaleAIAvailable,
				"aiModel":                state.upscaleAIModel,
				"applyFilters":           state.upscaleApplyFilters,
				"filterChain":            state.upscaleFilterChain,
				"duration":               state.upscaleFile.Duration,
				"frameRate":              state.upscaleFrameRate,
				"useMotionInterpolation": state.upscaleMotionInterpolation,
			},
		}, nil
	}

	// Apply/Queue buttons
	applyBtn := widget.NewButton("UPSCALE NOW", func() {
		job, err := createUpscaleJob()
		if err != nil {
			dialog.ShowError(err, state.window)
			return
		}

		state.jobQueue.Add(job)
		if !state.jobQueue.IsRunning() {
			state.jobQueue.Start()
		}
		dialog.ShowInformation("Upscale Started",
			fmt.Sprintf("Upscaling to %s.\nCheck the queue for progress.", state.upscaleTargetRes),
			state.window)
	})
	applyBtn.Importance = widget.HighImportance

	addQueueBtn := widget.NewButton("Add to Queue", func() {
		job, err := createUpscaleJob()
		if err != nil {
			dialog.ShowError(err, state.window)
			return
		}

		state.jobQueue.Add(job)
		dialog.ShowInformation("Added to Queue",
			fmt.Sprintf("Upscale job added.\nTarget: %s, Method: %s", state.upscaleTargetRes, state.upscaleMethod),
			state.window)
	})
	addQueueBtn.Importance = widget.MediumImportance

	// Main content
	leftPanel := container.NewVBox(
		instructions,
		widget.NewSeparator(),
		fileLabel,
		loadBtn,
		filtersNavBtn,
	)

	settingsPanel := container.NewVBox(
		traditionalSection,
		resolutionSection,
		frameRateSection,
		aiSection,
		filterIntegrationSection,
		container.NewGridWithColumns(2, applyBtn, addQueueBtn),
	)

	settingsScroll := container.NewVScroll(settingsPanel)
	// Adaptive height for small screens
	settingsScroll.SetMinSize(fyne.NewSize(400, 400))

	mainContent := container.NewHSplit(
		container.NewVBox(leftPanel, videoContainer),
		settingsScroll,
	)
	mainContent.SetOffset(0.55) // 55% for video preview, 45% for settings

	content := container.NewPadded(mainContent)

	return container.NewBorder(topBar, bottomBar, nil, nil, content)
}

// checkAIUpscaleAvailable checks if Real-ESRGAN is available on the system
func checkAIUpscaleAvailable() bool {
	// Check for realesrgan-ncnn-vulkan (most common binary distribution)
	cmd := exec.Command("realesrgan-ncnn-vulkan", "--help")
	if err := cmd.Run(); err == nil {
		return true
	}

	// Check for Python-based Real-ESRGAN
	cmd = exec.Command("python3", "-c", "import realesrgan")
	if err := cmd.Run(); err == nil {
		return true
	}

	// Check for alternative Python command
	cmd = exec.Command("python", "-c", "import realesrgan")
	if err := cmd.Run(); err == nil {
		return true
	}

	return false
}

// parseResolutionPreset parses resolution preset strings and returns target dimensions and whether to preserve aspect.
// Special presets like "Match Source" and relative (2X/4X) use source dimensions to preserve AR.
func parseResolutionPreset(preset string, srcW, srcH int) (width, height int, preserveAspect bool, err error) {
	// Default: preserve aspect
	preserveAspect = true

	// Sanitize source
	if srcW < 1 || srcH < 1 {
		srcW, srcH = 1920, 1080 // fallback to avoid zero division
	}

	switch preset {
	case "", "Match Source":
		return srcW, srcH, true, nil
	case "2X (relative)":
		return srcW * 2, srcH * 2, true, nil
	case "4X (relative)":
		return srcW * 4, srcH * 4, true, nil
	}

	presetMap := map[string][2]int{
		"720p (1280x720)":   {1280, 720},
		"1080p (1920x1080)": {1920, 1080},
		"1440p (2560x1440)": {2560, 1440},
		"4K (3840x2160)":    {3840, 2160},
		"8K (7680x4320)":    {7680, 4320},
		"720p":              {1280, 720},
		"1080p":             {1920, 1080},
		"1440p":             {2560, 1440},
		"4K":                {3840, 2160},
		"8K":                {7680, 4320},
	}

	if dims, ok := presetMap[preset]; ok {
		// Keep aspect by default: use target height and let FFmpeg derive width
		return dims[0], dims[1], true, nil
	}

	return 0, 0, true, fmt.Errorf("unknown resolution preset: %s", preset)
}

// buildUpscaleFilter builds the FFmpeg scale filter string with the selected method
func buildUpscaleFilter(targetWidth, targetHeight int, method string, preserveAspect bool) string {
	// Ensure even dimensions for encoders
	makeEven := func(v int) int {
		if v%2 != 0 {
			return v + 1
		}
		return v
	}

	h := makeEven(targetHeight)
	w := targetWidth
	if preserveAspect || w <= 0 {
		w = -2 // FFmpeg will derive width from height while preserving AR
	}
	return fmt.Sprintf("scale=%d:%d:flags=%s", w, h, method)
}

// sanitizeForPath creates a simple slug for filenames from user-visible labels
func sanitizeForPath(label string) string {
	r := strings.NewReplacer(" ", "", "(", "", ")", "", "×", "x", "/", "-", "\\", "-", ":", "-", ",", "", ".", "", "_", "")
	return strings.ToLower(r.Replace(label))
}

// buildCompareFullscreenView creates fullscreen side-by-side comparison with synchronized controls
func buildCompareFullscreenView(state *appState) fyne.CanvasObject {
	compareColor := moduleColor("compare")

	// Back button
	backBtn := widget.NewButton("< BACK TO COMPARE", func() {
		state.showCompareView()
	})
	backBtn.Importance = widget.LowImportance

	// Top bar with module color
	topBar := ui.TintedBar(compareColor, container.NewHBox(backBtn, layout.NewSpacer()))

	// Video player containers - large size for fullscreen
	file1VideoContainer := container.NewMax()
	file2VideoContainer := container.NewMax()

	// Build players if videos are loaded - use flexible size that won't force window expansion
	if state.compareFile1 != nil {
		file1VideoContainer.Objects = []fyne.CanvasObject{
			buildVideoPane(state, fyne.NewSize(400, 225), state.compareFile1, nil),
		}
	} else {
		file1VideoContainer.Objects = []fyne.CanvasObject{
			container.NewCenter(widget.NewLabel("No video loaded")),
		}
	}

	if state.compareFile2 != nil {
		file2VideoContainer.Objects = []fyne.CanvasObject{
			buildVideoPane(state, fyne.NewSize(400, 225), state.compareFile2, nil),
		}
	} else {
		file2VideoContainer.Objects = []fyne.CanvasObject{
			container.NewCenter(widget.NewLabel("No video loaded")),
		}
	}

	// File labels
	file1Name := "File 1: Not loaded"
	if state.compareFile1 != nil {
		file1Name = fmt.Sprintf("File 1: %s", filepath.Base(state.compareFile1.Path))
	}

	file2Name := "File 2: Not loaded"
	if state.compareFile2 != nil {
		file2Name = fmt.Sprintf("File 2: %s", filepath.Base(state.compareFile2.Path))
	}

	file1Label := widget.NewLabel(file1Name)
	file1Label.TextStyle = fyne.TextStyle{Bold: true}
	file1Label.Alignment = fyne.TextAlignCenter

	file2Label := widget.NewLabel(file2Name)
	file2Label.TextStyle = fyne.TextStyle{Bold: true}
	file2Label.Alignment = fyne.TextAlignCenter

	// Synchronized playback controls (note: actual sync would require VT_Player API enhancement)
	playBtn := widget.NewButton("▶ Play Both", func() {
		// TODO: When VT_Player API supports it, trigger synchronized playback
		dialog.ShowInformation("Synchronized Playback",
			"Synchronized playback control will be available when VT_Player API is enhanced.\n\n"+
				"For now, use individual player controls.",
			state.window)
	})
	playBtn.Importance = widget.HighImportance

	pauseBtn := widget.NewButton("⏸ Pause Both", func() {
		// TODO: Synchronized pause
		dialog.ShowInformation("Synchronized Playback",
			"Synchronized playback control will be available when VT_Player API is enhanced.",
			state.window)
	})

	syncControls := container.NewHBox(
		layout.NewSpacer(),
		playBtn,
		pauseBtn,
		layout.NewSpacer(),
	)

	// Info text
	infoLabel := widget.NewLabel("Side-by-side fullscreen comparison. Use individual player controls until synchronized playback is implemented in VT_Player.")
	infoLabel.Wrapping = fyne.TextWrapWord
	infoLabel.Alignment = fyne.TextAlignCenter

	// Left column (File 1)
	leftColumn := container.NewBorder(
		file1Label,
		nil, nil, nil,
		file1VideoContainer,
	)

	// Right column (File 2)
	rightColumn := container.NewBorder(
		file2Label,
		nil, nil, nil,
		file2VideoContainer,
	)

	// Bottom bar with module color
	bottomBar := ui.TintedBar(compareColor, container.NewHBox(state.statsBar, layout.NewSpacer()))

	// Main content
	content := container.NewBorder(
		container.NewVBox(infoLabel, syncControls, widget.NewSeparator()),
		nil, nil, nil,
		container.NewGridWithColumns(2, leftColumn, rightColumn),
	)

	return container.NewBorder(topBar, bottomBar, nil, nil, content)
}
