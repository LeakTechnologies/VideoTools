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
	"git.leaktechnologies.dev/stu/VideoTools/internal/sysinfo"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"

	"github.com/ebitengine/oto/v3"
	"github.com/skip2/go-qrcode"
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
	appVersion      = "v0.1.0-dev23"

	hwAccelProbeOnce sync.Once
	hwAccelSupported atomic.Value // map[string]bool

	nvencRuntimeOnce sync.Once
	nvencRuntimeOK   bool

	// Rainbow color palette: balanced ROYGBIV distribution (2 modules per color)
	// Optimized for white text readability
	modulesList = []Module{
		{"convert", "Convert", utils.MustHex("#673AB7"), "Convert", modules.HandleConvert},          // Deep Purple (primary conversion)
		{"merge", "Merge", utils.MustHex("#4CAF50"), "Convert", modules.HandleMerge},                // Green (combining)
		{"trim", "Trim", utils.MustHex("#F9A825"), "Convert", nil},                                  // Dark Yellow/Gold (not implemented yet)
		{"filters", "Filters", utils.MustHex("#00BCD4"), "Convert", modules.HandleFilters},          // Cyan (creative filters)
		{"upscale", "Upscale", utils.MustHex("#9C27B0"), "Advanced", modules.HandleUpscale},         // Purple (AI/advanced)
		{"enhancement", "Enhancement", utils.MustHex("#7C3AED"), "Advanced", modules.HandleEnhance}, // Cyan (AI enhancement)
		{"audio", "Audio", utils.MustHex("#FF8F00"), "Convert", modules.HandleAudio},                // Dark Amber - audio extraction
		{"author", "Author", utils.MustHex("#FF5722"), "Disc", modules.HandleAuthor},                // Deep Orange (authoring)
		{"rip", "Rip", utils.MustHex("#FF9800"), "Disc", modules.HandleRip},                         // Orange (extraction)
		{"bluray", "Blu-Ray", utils.MustHex("#2196F3"), "Disc", nil},                                // Blue (not implemented yet)
		{"subtitles", "Subtitles", utils.MustHex("#689F38"), "Convert", modules.HandleSubtitles},    // Dark Green (text)
		{"enhancement", "Enhancement", utils.MustHex("#7C3AED"), "Advanced", modules.HandleEnhance}, // Cyan (AI enhancement)
		{"thumb", "Thumb", utils.MustHex("#00ACC1"), "Screenshots", modules.HandleThumb},            // Dark Cyan (capture)
		{"compare", "Compare", utils.MustHex("#E91E63"), "Inspect", modules.HandleCompare},          // Pink (comparison)
		{"inspect", "Inspect", utils.MustHex("#F44336"), "Inspect", modules.HandleInspect},          // Red (analysis)
		{"player", "Player", utils.MustHex("#3F51B5"), "Playback", modules.HandlePlayer},            // Indigo (playback)
		{"settings", "Settings", utils.MustHex("#607D8B"), "Settings", nil},                         // Blue Grey (settings)
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
		// Prefer user config dir for logs
		if cfgDir, err := os.UserConfigDir(); err == nil && cfgDir != "" {
			logsDirPath = filepath.Join(cfgDir, "VideoTools", "logs")
		}
		// Fallback to logs folder next to the executable
		if logsDirPath == "" {
			if exe, err := os.Executable(); err == nil {
				if dir := filepath.Dir(exe); dir != "" {
					logsDirPath = filepath.Join(dir, "logs")
				}
			}
		}
		// Final fallback to cwd/logs
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
		cmd := utils.CreateCommandRaw(utils.GetFFmpegPath(), "-hide_banner", "-v", "error", "-hwaccels")
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
		cmd := utils.CreateCommandRaw(utils.GetFFmpegPath(),
			"-hide_banner", "-loglevel", "error",
			"-f", "lavfi", "-i", "color=size=16x16:rate=1",
			"-frames:v", "1",
			"-c:v", "h264_nvenc",
			"-f", "null", "-",
		)
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
	closeBtn := widget.NewButton("Close", func() {
		if d != nil {
			d.Hide()
		}
	})
	copyBtn := widget.NewButton("Copy All", func() {
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
func openFolder(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is empty")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = utils.CreateCommandRaw("explorer", path)
	case "darwin":
		cmd = utils.CreateCommandRaw("open", path)
	default:
		cmd = utils.CreateCommandRaw("xdg-open", path)
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
		cmd = utils.CreateCommandRaw("explorer", path)
	case "darwin":
		cmd = utils.CreateCommandRaw("open", path)
	default:
		cmd = utils.CreateCommandRaw("xdg-open", path)
	}
	return cmd.Start()
}

func generatePixelatedQRCode() (fyne.CanvasObject, error) {
	docURL := "https://docs.leaktechnologies.dev/VideoTools"

	// Generate QR code with fewer pixels for a chunkier, blockier look
	qrBytes, err := qrcode.Encode(docURL, qrcode.Low, 112)
	if err != nil {
		return nil, err
	}

	// Convert to Fyne image with pixelated look
	img := canvas.NewImageFromReader(bytes.NewReader(qrBytes), "qrcode.png")
	img.FillMode = canvas.ImageFillOriginal // Keep pixelated look
	img.SetMinSize(fyne.NewSize(112, 112))

	return img, nil
}

func (s *appState) showAbout() {
	version := fmt.Sprintf("VideoTools %s", appVersion)
	dev := "Leak Technologies"
	logsPath := getLogsDir()

	title := canvas.NewText("About / Support", textColor)
	title.TextSize = 20

	versionText := widget.NewLabel(version)
	devText := widget.NewLabel(fmt.Sprintf("Developer: %s", dev))

	loadLogo := func(name string, size float32) fyne.CanvasObject {
		candidates := []string{filepath.Join("assets", "logo", name)}
		if exe, err := os.Executable(); err == nil {
			dir := filepath.Dir(exe)
			candidates = append(candidates, filepath.Join(dir, "assets", "logo", name))
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				img := canvas.NewImageFromFile(p)
				img.FillMode = canvas.ImageFillContain
				img.SetMinSize(fyne.NewSize(size, size))
				return img
			}
		}
		return nil
	}

	vtLogo := loadLogo("VT_Icon.png", 96)
	ltLogo := loadLogo("LT_Logo-26.png", 72)

	logsLink := widget.NewButton("Logs Folder", func() {
		if err := openFolder(logsPath); err != nil {
			dialog.ShowError(fmt.Errorf("failed to open logs folder: %w", err), s.window)
		}
	})
	logsLink.Importance = widget.LowImportance

	feedbackLabel := widget.NewLabel("Feedback: use the Logs button on the main menu to view logs; send issues with attached logs.")
	feedbackLabel.Wrapping = fyne.TextWrapWord

	btcAddress := "bc1qcq5hmtvckhhh9c6y3gvm9wu9856fmet25yfr0v"
	btcLabel := widget.NewLabel("Bitcoin (BTC):")
	copyBg := canvas.NewRectangle(utils.MustHex("#344256"))
	copyBg.CornerRadius = 8
	copyBg.SetMinSize(fyne.NewSize(72, 32))
	copyText := canvas.NewText("Copy", textColor)
	copyText.Alignment = fyne.TextAlignCenter
	copyBtn := ui.NewTappable(container.NewMax(copyBg, container.NewPadded(copyText)), func() {
		s.window.Clipboard().SetContent(btcAddress)
		dialog.ShowInformation("Copied", "Bitcoin address copied to clipboard", s.window)
	})
	copyRow := container.NewBorder(nil, nil, nil, copyBtn, btcLabel)
	addressLabel := widget.NewLabel(btcAddress)

	mainContent := container.NewVBox(
		versionText,
		devText,
		widget.NewLabel(""),
		widget.NewLabel("Support Development"),
		copyRow,
		addressLabel,
		feedbackLabel,
	)

	logoColumn := container.NewVBox()
	if vtLogo != nil {
		logoColumn.Add(vtLogo)
	}
	if ltLogo != nil {
		logoColumn.Add(ltLogo)
	}

	// Add QR code for documentation
	qrCode, err := generatePixelatedQRCode()
	if err != nil {
		// Fallback to hyperlink if QR generation fails
		docURL, _ := url.Parse("https://docs.leaktechnologies.dev/VideoTools")
		fallbackLink := widget.NewHyperlink("View Documentation", docURL)
		logoColumn.Add(fallbackLink)
	} else {
		// Add QR code with label
		qrLabel := widget.NewLabel("Scan for docs")
		qrLabel.Alignment = fyne.TextAlignCenter
		logoColumn.Add(qrCode)
		logoColumn.Add(qrLabel)
	}

	logoColumn.Add(layout.NewSpacer())
	logoColumn.Add(logsLink)

	body := container.NewBorder(
		container.NewHBox(title),
		nil,
		nil,
		logoColumn,
		mainContent,
	)
	body = container.NewPadded(body)
	sizeShim := canvas.NewRectangle(color.Transparent)
	sizeShim.SetMinSize(fyne.NewSize(560, 280))
	dialog.ShowCustom("About & Support", "Close", container.NewMax(sizeShim, body), s.window)
}

type formatOption struct {
	Label      string
	Ext        string
	VideoCodec string
}

var formatOptions = []formatOption{
	// H.264 - Widely compatible, older standard
	{"MP4 (H.264)", ".mp4", "libx264"},
	{"MOV (H.264)", ".mov", "libx264"},
	// Remux - No re-encode
	{"MKV (Remux)", ".mkv", "copy"},
	// H.265/HEVC - Better compression than H.264
	{"MP4 (H.265)", ".mp4", "libx265"},
	{"MKV (H.265)", ".mkv", "libx265"},
	{"MOV (H.265)", ".mov", "libx265"},
	// AV1 - Best compression, slower encode
	{"MP4 (AV1)", ".mp4", "libaom-av1"},
	{"MKV (AV1)", ".mkv", "libaom-av1"},
	{"WebM (AV1)", ".webm", "libaom-av1"},
	// VP9 - Google codec, good for web
	{"WebM (VP9)", ".webm", "libvpx-vp9"},
	// ProRes - Professional/editing codec
	{"MOV (ProRes)", ".mov", "prores_ks"},
	// MPEG-2 - DVD standard
	{"DVD-NTSC (MPEG-2)", ".mpg", "mpeg2video"},
	{"DVD-PAL (MPEG-2)", ".mpg", "mpeg2video"},
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
	AspectUserSet    bool   // Tracks if user explicitly set OutputAspect
	TempDir          string // Optional temp/cache directory override
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
		BitrateMode:            "CRF",
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

		InverseTelecine:  true,
		InverseAutoNotes: "Default smoothing for interlaced footage.",
		CoverArtPath:     "",
		AspectHandling:   "Auto",
		OutputAspect:     "Source",
		AspectUserSet:    false,
		TempDir:          "",
	}
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
	if cfg.OutputAspect == "" || strings.EqualFold(cfg.OutputAspect, "Source") {
		cfg.OutputAspect = "Source"
		cfg.AspectUserSet = false
	} else if !cfg.AspectUserSet {
		// Treat legacy saved aspects (like 16:9 defaults) as unset
		cfg.OutputAspect = "Source"
		cfg.AspectUserSet = false
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
	Timestamp          time.Time            `json:"timestamp"`
	Results            []benchmark.Result   `json:"results"`
	RecommendedEncoder string               `json:"recommended_encoder"`
	RecommendedPreset  string               `json:"recommended_preset"`
	RecommendedHWAccel string               `json:"recommended_hwaccel"`
	RecommendedFPS     float64              `json:"recommended_fps"`
	HardwareInfo       sysinfo.HardwareInfo `json:"hardware_info"`
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

// historyConfig holds conversion history
type historyConfig struct {
	Entries []ui.HistoryEntry `json:"entries"`
}

func historyConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		home := os.Getenv("HOME")
		if home != "" {
			configDir = filepath.Join(home, ".config")
		}
	}
	if configDir == "" {
		return "history.json"
	}
	return filepath.Join(configDir, "VideoTools", "history.json")
}

func loadHistoryConfig() (historyConfig, error) {
	path := historyConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return historyConfig{Entries: []ui.HistoryEntry{}}, nil
		}
		return historyConfig{}, err
	}
	var cfg historyConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return historyConfig{}, err
	}
	return cfg, nil
}

func saveHistoryConfig(cfg historyConfig) error {
	// Limit to 20 most recent entries
	if len(cfg.Entries) > 20 {
		cfg.Entries = cfg.Entries[:20]
	}
	path := historyConfigPath()
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
	thumbFile           *videoSource
	thumbFiles          []*videoSource
	thumbCount          int
	thumbWidth          int
	thumbContactSheet   bool
	thumbShowTimestamps bool
	thumbSheetWidth     int
	thumbColumns        int
	thumbRows           int
	thumbLastOutputPath string // Path to last generated output

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

	// Snippet settings
	snippetLength       int  // Length of snippet in seconds (default: 20)
	snippetSourceFormat bool // true = source format, false = conversion format (default: true)

	// Interlacing detection state
	interlaceResult    *interlace.DetectionResult
	interlaceAnalyzing bool

	// History sidebar state
	historyEntries []ui.HistoryEntry
	sidebarVisible bool

	// Author module state
	authorFile            *videoSource
	authorChapters        []authorChapter
	authorSceneThreshold  float64
	authorDetecting       bool
	authorClips           []authorClip // Multiple video clips for compilation
	authorOutputType      string       // "dvd" or "iso"
	authorRegion          string       // "NTSC", "PAL", "AUTO"
	authorAspectRatio     string       // "4:3", "16:9", "AUTO"
	authorCreateMenu      bool         // Whether to create DVD menu
	authorTitle           string       // DVD title
	authorSubtitles       []string     // Subtitle file paths
	authorAudioTracks     []string     // Additional audio tracks
	authorSummaryLabel    *widget.Label
	authorTreatAsChapters bool   // Treat multiple clips as chapters
	authorChapterSource   string // embedded, scenes, clips, manual
	authorChaptersRefresh func() // Refresh hook for chapter list UI
	authorDiscSize        string // "DVD5" or "DVD9"
	authorLogText         string
	authorLogLines        []string // Circular buffer for last N lines
	authorLogFilePath     string   // Path to log file for full viewing
	authorLogEntry        *widget.Entry
	authorLogScroll       *container.Scroll
	authorProgress        float64
	authorProgressBar     *widget.ProgressBar
	authorStatusLabel     *widget.Label
	authorCancelBtn       *widget.Button
	authorVideoTSPath     string

	// Rip module state
	ripSourcePath  string
	ripOutputPath  string
	ripFormat      string
	ripLogText     string
	ripLogEntry    *widget.Entry
	ripLogScroll   *container.Scroll
	ripProgress    float64
	ripProgressBar *widget.ProgressBar
	ripStatusLabel *widget.Label

	queueAutoRefreshStop    chan struct{}
	queueAutoRefreshRunning bool
	queueView               *ui.QueueView
	queueElapsedStop        chan struct{}
	queueElapsedRunning     bool

	// Main menu refresh throttling
	mainMenuLastRefresh time.Time

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
	audioBatchListContainer   *fyne.Container
	audioLeftPanel            *fyne.Container
	audioSingleContent        *fyne.Container
	audioBatchContent         *fyne.Container
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
}

func (s *appState) persistConvertConfig() {
	if err := savePersistedConvertConfig(s.convert); err != nil {
		logging.Debug(logging.CatSystem, "failed to persist convert config: %v", err)
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
			buttons = append(buttons, widget.NewButton("Show in Folder", func() {
				dir := filepath.Dir(entry.OutputFile)
				if err := openFolder(dir); err != nil {
					dialog.ShowError(err, s.window)
				}
			}))
		}
	}

	if entry.LogPath != "" {
		if _, err := os.Stat(entry.LogPath); err == nil {
			buttons = append(buttons, widget.NewButton("View Log", func() {
				s.openLogViewer(entry.Title, entry.LogPath, false)
			}))
		}
	}

	closeBtn := widget.NewButton("Close", nil)
	buttons = append(buttons, layout.NewSpacer(), closeBtn)

	// Job details in scrollable area
	detailsScroll := container.NewVScroll(detailsLabel)
	// detailsScroll.SetMinSize(fyne.NewSize(650, 250)) // Removed for flexible sizing

	// FFmpeg Command section at bottom
	var ffmpegSection fyne.CanvasObject
	if entry.FFmpegCmd != "" {
		cmdWidget := ui.NewFFmpegCommandWidget(entry.FFmpegCmd, s.window)
		cmdLabel := widget.NewLabel("FFmpeg Command:")
		cmdLabel.TextStyle = fyne.TextStyle{Bold: true}
		ffmpegSection = container.NewVBox(
			widget.NewSeparator(),
			cmdLabel,
			cmdWidget,
		)
	}

	// Layout: details at top (scrollable), FFmpeg at bottom (fixed)
	content := container.NewBorder(
		detailsScroll, // Top: job details (scrollable, takes priority)
		container.NewVBox( // Bottom: FFmpeg command (fixed)
			ffmpegSection,
			container.NewHBox(buttons...),
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
	s.showMainMenu()
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
		// Preserve current window size to prevent auto-resizing when content changes
		// This ensures the window maintains the size the user set, even when content
		// like progress bars or queue items change dynamically
		currentSize := s.window.Canvas().Size()

		bg := canvas.NewRectangle(backgroundColor)
		if body == nil {
			s.window.SetContent(bg)
			// Restore window size after setting content
			s.window.Resize(currentSize)
			return
		}
		// Wrap content with mouse button handler
		wrapped := newMouseButtonHandler(container.NewMax(bg, body), s)
		s.window.SetContent(wrapped)
		// Restore window size after setting content
		s.window.Resize(currentSize)
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
	s.stopQueueAutoRefresh()
	s.stopQueueElapsedTicker()
	if s.queueView != nil {
		s.queueView.StopAnimations()
	}
	s.active = ""
	s.queueBackTarget = ""

	// Track navigation history
	s.pushNavigationHistory("mainmenu")

	// Convert Module slice to ui.ModuleInfo slice
	var mods []ui.ModuleInfo
	for _, m := range modulesList {
		hasHandler := m.Handle != nil
		depsAvailable := isModuleAvailable(m.ID)

		// Module is enabled if: (1) it's Settings (special case) OR (2) it has a handler AND dependencies are available
		enabled := m.ID == "settings" || (hasHandler && depsAvailable)

		// Missing dependencies = has handler but dependencies not available
		missingDeps := hasHandler && !depsAvailable && m.ID != "settings"

		mods = append(mods, ui.ModuleInfo{
			ID:                  m.ID,
			Label:               m.Label,
			Color:               m.Color,
			Category:            m.Category,
			Enabled:             enabled,
			MissingDependencies: missingDeps,
		})
	}

	titleColor := utils.MustHex("#4CE870")

	// PERFORMANCE: Cache queue list to avoid multiple expensive copies
	var queueList []*queue.Job
	if s.jobQueue != nil {
		queueList = s.jobQueue.List()
	}

	// Get queue stats - show completed jobs out of total
	var queueCompleted, queueTotal int
	if s.jobQueue != nil {
		_, _, completed, _, _ := s.jobQueue.Stats()
		queueCompleted = completed
		queueTotal = len(queueList)
	}

	// Build sidebar if visible
	var sidebar fyne.CanvasObject
	if s.sidebarVisible {
		// Get active jobs from queue (running/pending)
		var activeJobs []ui.HistoryEntry
		if s.jobQueue != nil {
			for _, job := range queueList {
				if job.Status == queue.JobStatusRunning || job.Status == queue.JobStatusPending {
					// Convert queue.Job to ui.HistoryEntry
					entry := ui.HistoryEntry{
						ID:         job.ID,
						Type:       job.Type,
						Status:     job.Status,
						Title:      job.Title,
						InputFile:  job.InputFile,
						OutputFile: job.OutputFile,
						LogPath:    job.LogPath,
						Config:     job.Config,
						CreatedAt:  job.CreatedAt,
						StartedAt:  job.StartedAt,
						Error:      job.Error,
						Progress:   job.Progress / 100.0, // Convert 0-100 to 0.0-1.0
					}
					activeJobs = append(activeJobs, entry)
				}
			}
		}

		sidebar = ui.BuildHistorySidebar(
			s.historyEntries,
			activeJobs,
			s.showHistoryDetails,
			s.deleteHistoryEntry,
			titleColor,
			utils.MustHex("#1A1F2E"),
			textColor,
		)
	}

	// Check if benchmark has been run
	hasBenchmark := false
	if cfg, err := loadBenchmarkConfig(); err == nil && len(cfg.History) > 0 {
		hasBenchmark = true
	}

	menu := ui.BuildMainMenu(mods, s.showModule, s.handleModuleDrop, s.showQueue, nil, s.showBenchmark, s.showBenchmarkHistory, func() {
		// Toggle sidebar - use throttled refresh to prevent lag
		s.sidebarVisible = !s.sidebarVisible
		s.refreshMainMenuThrottled()
	}, s.sidebarVisible, sidebar, titleColor, queueColor, textColor, queueCompleted, queueTotal, hasBenchmark)

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

// refreshMainMenuThrottled rebuilds main menu but throttles to prevent excessive redraws
// Windows GUI is sensitive to rapid rebuilds, so we enforce a minimum delay
func (s *appState) refreshMainMenuThrottled() {
	now := time.Now()
	if !s.mainMenuLastRefresh.IsZero() && now.Sub(s.mainMenuLastRefresh) < 300*time.Millisecond {
		// Too soon since last refresh - skip to prevent lag
		return
	}
	s.mainMenuLastRefresh = now
	s.showMainMenu()
}

// refreshMainMenuSidebar is a lightweight refresh for sidebar-only updates
// This prevents full main menu rebuilds when only history changes
func (s *appState) refreshMainMenuSidebar() {
	// For now, use throttled refresh to prevent cascading rebuilds
	// In the future, could optimize to only update sidebar component
	s.refreshMainMenuThrottled()
}

func (s *appState) showQueue() {
	s.stopPreview()
	s.stopPlayer()
	if s.active != "queue" {
		s.lastModule = s.active
		s.queueBackTarget = s.active
	}
	s.active = "queue"
	s.refreshQueueView()
	if s.queueView != nil {
		s.setContent(s.queueView.Root)
	}
	s.startQueueAutoRefresh()
	s.startQueueElapsedTicker()
}

// clearCompletedJobs removes all completed and failed jobs from the queue
func (s *appState) clearCompletedJobs() {
	if s.jobQueue != nil {
		s.jobQueue.Clear()
	}
}

// refreshQueueView rebuilds the queue UI while preserving scroll position and inline active conversion.
func (s *appState) refreshQueueView() {
	if s.active == "queue" {
		now := time.Now()
		if !s.queueLastRefresh.IsZero() && now.Sub(s.queueLastRefresh) < 200*time.Millisecond {
			return
		}
		s.queueLastRefresh = now
	}

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

	if s.queueView == nil {
		view := ui.BuildQueueView(
			jobs,
			func() { // onBack
				// Stop auto-refresh before navigating away for snappy response
				s.stopQueueAutoRefresh()
				target := s.queueBackTarget
				if target == "" {
					target = s.lastModule
				}
				if target != "" && target != "queue" && target != "menu" {
					s.showModule(target)
				} else {
					s.showMainMenu()
				}
			},
			func(id string) { // onPause
				if err := s.jobQueue.Pause(id); err != nil {
					logging.Debug(logging.CatSystem, "failed to pause job: %v", err)
				}
			},
			func(id string) { // onResume
				if err := s.jobQueue.Resume(id); err != nil {
					logging.Debug(logging.CatSystem, "failed to resume job: %v", err)
				}
			},
			func(id string) { // onCancel
				if err := s.jobQueue.Cancel(id); err != nil {
					logging.Debug(logging.CatSystem, "failed to cancel job: %v", err)
				}
			},
			func(id string) { // onRemove
				if err := s.jobQueue.Remove(id); err != nil {
					logging.Debug(logging.CatSystem, "failed to remove job: %v", err)
				}
			},
			func(id string) { // onMoveUp
				if err := s.jobQueue.MoveUp(id); err != nil {
					logging.Debug(logging.CatSystem, "failed to move job up: %v", err)
				}
			},
			func(id string) { // onMoveDown
				if err := s.jobQueue.MoveDown(id); err != nil {
					logging.Debug(logging.CatSystem, "failed to move job down: %v", err)
				}
			},
			func() { // onPauseAll
				s.jobQueue.PauseAll()
			},
			func() { // onResumeAll
				s.jobQueue.ResumeAll()
			},
			func() { // onStart
				s.jobQueue.ResumeAll()
			},
			func() { // onClear
				// Stop auto-refresh to prevent double UI updates
				s.stopQueueAutoRefresh()
				s.jobQueue.Clear()

				// Always return to main menu after clearing
				if len(s.jobQueue.List()) == 0 {
					s.showMainMenu()
				} else {
					// Restart auto-refresh and do single refresh
					s.startQueueAutoRefresh()
					s.refreshQueueView()
				}
			},
			func() { // onClearAll
				// Stop auto-refresh to prevent double UI updates during navigation
				s.stopQueueAutoRefresh()
				s.jobQueue.ClearAll()
				// Return to the module we were working on if possible
				if s.lastModule != "" && s.lastModule != "queue" && s.lastModule != "menu" {
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
			func(id string) { // onCopyCommand
				job, err := s.jobQueue.Get(id)
				if err != nil {
					logging.Debug(logging.CatSystem, "copy command failed: %v", err)
					return
				}
				cmdStr := buildFFmpegCommandFromJob(job)
				if cmdStr == "" {
					dialog.ShowInformation("No Command", "Unable to generate FFmpeg command for this job.", s.window)
					return
				}
				s.window.Clipboard().SetContent(cmdStr)
				dialog.ShowInformation("Copied", "FFmpeg command copied to clipboard", s.window)
			},
			utils.MustHex("#4CE870"), // titleColor
			gridColor,                // bgColor
			textColor,                // textColor
		)

		s.queueView = view
		s.queueScroll = view.Scroll
		s.setContent(view.Root)

		// Restore scroll offset
		if s.queueScroll != nil && s.active == "queue" {
			savedOffset := s.queueOffset
			go func() {
				time.Sleep(10 * time.Millisecond)
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					if s.queueScroll != nil {
						s.queueScroll.Offset = savedOffset
						s.queueScroll.Refresh()
					}
				}, false)
			}()
		}
	} else {
		s.queueView.UpdateJobs(jobs)
	}
}

func (s *appState) startQueueAutoRefresh() {
	if s.queueAutoRefreshRunning {
		return
	}
	stop := make(chan struct{})
	s.queueAutoRefreshStop = stop
	s.queueAutoRefreshRunning = true
	go func() {
		// Short interval keeps elapsed/progress responsive with incremental UI updates.
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if s.active != "queue" {
					return
				}
				app := fyne.CurrentApp()
				if app == nil || app.Driver() == nil {
					continue
				}
				app.Driver().DoFromGoroutine(func() {
					if s.active == "queue" {
						s.refreshQueueView()
					}
				}, false)
			}
		}
	}()
}

func (s *appState) stopQueueAutoRefresh() {
	if !s.queueAutoRefreshRunning {
		return
	}
	if s.queueAutoRefreshStop != nil {
		close(s.queueAutoRefreshStop)
	}
	s.queueAutoRefreshStop = nil
	s.queueAutoRefreshRunning = false

	if s.queueView != nil {
		s.queueView.StopAnimations()
	}
}

func (s *appState) startQueueElapsedTicker() {
	if s.queueElapsedRunning {
		return
	}
	stop := make(chan struct{})
	s.queueElapsedStop = stop
	s.queueElapsedRunning = true
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if s.active != "queue" || s.queueView == nil {
					return
				}
				app := fyne.CurrentApp()
				if app == nil || app.Driver() == nil {
					continue
				}
				app.Driver().DoFromGoroutine(func() {
					if s.active == "queue" && s.queueView != nil {
						s.queueView.UpdateRunningStatus(s.jobQueue.List())
					}
				}, false)
			}
		}
	}()
}

func (s *appState) stopQueueElapsedTicker() {
	if !s.queueElapsedRunning {
		return
	}
	if s.queueElapsedStop != nil {
		close(s.queueElapsedStop)
	}
	s.queueElapsedStop = nil
	s.queueElapsedRunning = false
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
		"inputPath":         src.Path,
		"outputPath":        outPath,
		"outputDir":         outDir,
		"outputBase":        cfg.OutputBase,
		"selectedFormat":    cfg.SelectedFormat,
		"quality":           cfg.Quality,
		"mode":              cfg.Mode,
		"preserveChapters":  cfg.PreserveChapters,
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
		"sourceBitrate":     src.Bitrate,
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

	// Add to top (after running job) if requested and queue is running
	if addToTop && s.jobQueue.IsRunning() {
		s.jobQueue.AddNext(job)
		logging.Debug(logging.CatSystem, "added convert job to top of queue: %s", job.ID)
	} else {
		s.jobQueue.Add(job)
		logging.Debug(logging.CatSystem, "added convert job to queue: %s", job.ID)
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

	outDir := filepath.Dir(src.Path)
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
		"inputPath":         src.Path,
		"outputPath":        outPath,
		"outputBase":        cfg.OutputBase,
		"selectedFormat":    cfg.SelectedFormat,
		"quality":           cfg.Quality,
		"mode":              cfg.Mode,
		"preserveChapters":  cfg.PreserveChapters,
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
		"sourceBitrate":     src.Bitrate,
		"fieldOrder":        src.FieldOrder,
		"autoCompare":       s.autoCompare,
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
				Preset:  lastRun.RecommendedPreset,
				FPS:     lastRun.RecommendedFPS,
				Score:   lastRun.RecommendedFPS,
			}

			// Show results with "Run New Benchmark" option
			resultsView := ui.BuildBenchmarkResultsView(
				lastRun.Results,
				rec,
				lastRun.HardwareInfo,
				func() {
					// Apply recommended settings
					s.applyBenchmarkRecommendation(lastRun.RecommendedEncoder, lastRun.RecommendedPreset)
					s.showMainMenu()
				},
				func() {
					// Close - go back to main menu
					s.showMainMenu()
				},
				utils.MustHex("#4CE870"),
				utils.MustHex("#1E1E1E"),
				utils.MustHex("#FFFFFF"),
			)

			// Add "Run New Benchmark" button at the bottom
			runNewBtn := widget.NewButton("Run New Benchmark", func() {
				s.runNewBenchmark()
			})
			runNewBtn.Importance = widget.MediumImportance

			cachedNote := widget.NewLabel(fmt.Sprintf("Showing cached results from %s", lastRun.Timestamp.Format("January 2, 2006 at 3:04 PM")))
			cachedNote.Alignment = fyne.TextAlignCenter
			cachedNote.TextStyle = fyne.TextStyle{Italic: true}

			viewWithButton := container.NewBorder(
				nil,
				container.NewVBox(
					widget.NewSeparator(),
					cachedNote,
					container.NewCenter(runNewBtn),
				),
				nil, nil,
				resultsView,
			)

			s.setContent(viewWithButton)
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
				s.showMainMenu()
				return
			}

			dialog.ShowConfirm("Cancel Benchmark?", "The benchmark is still running. Cancel it now?", func(ok bool) {
				if !ok {
					return
				}
				cancel()
				s.showMainMenu()
			}, s.window)
		},
		utils.MustHex("#4CE870"),
		utils.MustHex("#1E1E1E"),
		utils.MustHex("#FFFFFF"),
	)

	s.setContent(view.GetContainer())

	// Run benchmark in background
	go func() {
		// Generate test video
		view.UpdateProgress(0, 100, "Generating test video", "")
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
		suite.Progress = func(current, total int, encoder, preset string) {
			logging.Debug(logging.CatSystem, "benchmark progress: %d/%d testing %s (%s)", current, total, encoder, preset)
			view.UpdateProgress(current, total, encoder, preset)
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
		encoder, preset, rec := suite.GetRecommendation()

		// Save benchmark run to history
		if err := s.saveBenchmarkRun(suite.Results, encoder, preset, rec.FPS); err != nil {
			logging.Debug(logging.CatSystem, "failed to save benchmark run: %v", err)
		}

		if encoder != "" {
			logging.Debug(logging.CatSystem, "benchmark recommendation: %s (preset: %s) - %.1f FPS", encoder, preset, rec.FPS)

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
		cmd := utils.CreateCommandRaw(utils.GetFFmpegPath(), "-hide_banner", "-encoders")
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
		RecommendedEncoder: encoder,
		RecommendedPreset:  preset,
		RecommendedHWAccel: hwAccel,
		RecommendedFPS:     fps,
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

	// Respect user's quality preference: if they have slow/slower set, upgrade the preset
	currentPreset := strings.ToLower(s.convert.EncoderPreset)
	if currentPreset == "slow" || currentPreset == "slower" {
		// User prefers quality over speed - upgrade benchmark preset to slower
		preset = "slow"
		logging.Debug(logging.CatSystem, "user prefers quality - upgraded preset to 'slow'")
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
				run.HardwareInfo,
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

func (s *appState) showMissingDependenciesDialog(moduleID string) {
	missing, _ := getModuleDependencyStatus(moduleID)

	if len(missing) == 0 {
		return // No missing dependencies
	}

	// Build message with missing dependencies and install commands
	var message strings.Builder
	message.WriteString("This module requires the following dependencies:\n\n")

	for _, depName := range missing {
		if dep, ok := allDependencies[depName]; ok {
			message.WriteString(fmt.Sprintf("• %s\n", dep.Name))
			if dep.InstallCmd != "" {
				message.WriteString(fmt.Sprintf("  Install: %s\n\n", dep.InstallCmd))
			}
		}
	}

	// Create dialog
	dialog.ShowInformation(
		"Missing Dependencies",
		message.String(),
		s.window,
	)
}

func (s *appState) showModule(id string) {
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
	// case "enhancement":
	//	s.showEnhancementView() // TODO: Implement when enhancement module is complete
	case "audio":
		s.showAudioView()
	case "author":
		s.showAuthorView()
	case "rip":
		s.showRipView()
	case "subtitles":
		s.showSubtitlesView()
	case "settings":
		s.showSettingsView()
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
	if moduleID == "subtitles" {
		s.handleSubtitlesModuleDrop(items)
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

func (s *appState) isSubtitleFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	subtitleExts := []string{".srt", ".vtt", ".ass", ".ssa"}
	for _, subtitleExt := range subtitleExts {
		if ext == subtitleExt {
			return true
		}
	}
	return false
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

func (s *appState) showPlayerView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "player"
	s.setContent(buildPlayerView(s))
}

func (s *appState) showAuthorView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "author"

	// Initialize scene detection threshold if not set
	if s.authorSceneThreshold == 0 {
		s.authorSceneThreshold = 0.3
	}

	// Clear DVD title for fresh start
	s.authorTitle = ""

	s.setContent(buildAuthorView(s))
}

func (s *appState) showMergeView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "merge"

	mergeColor := moduleColor("merge")

	if cfg, err := loadPersistedMergeConfig(); err == nil {
		s.applyMergeConfig(cfg)
	} else if !errors.Is(err, os.ErrNotExist) {
		logging.Debug(logging.CatSystem, "failed to load persisted merge config: %v", err)
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
		if len(s.mergeClips) >= 2 && s.mergeOutputDir == "" {
			s.mergeOutputDir = filepath.Dir(s.mergeClips[0].Path)
		}
		if len(s.mergeClips) >= 2 && s.mergeOutputFilename == "" {
			s.mergeOutputFilename = "merged.mkv"
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

	clearBtn := widget.NewButton("Clear", func() {
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
		widget.NewLabel("Region:"),
		dvdRegionSelect,
		widget.NewLabel("Aspect:"),
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
		widget.NewLabel("Frame Rate"),
		frameRateSelect,
		motionInterpCheck,
	)

	browseDirBtn := widget.NewButton("Browse Folder", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			s.mergeOutputDir = uri.Path()
			outputDirEntry.SetText(s.mergeOutputDir)
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

	loadCfgBtn := widget.NewButton("Load Config", func() {
		cfg, err := loadPersistedMergeConfig()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				dialog.ShowInformation("No Config", "No saved config found yet. It will save automatically after your first change.", s.window)
			} else {
				dialog.ShowError(fmt.Errorf("failed to load config: %w", err), s.window)
			}
			return
		}
		s.applyMergeConfig(cfg)
		applyMergeControls()
	})

	saveCfgBtn := widget.NewButton("Save Config", func() {
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
		dialog.ShowInformation("Config Saved", fmt.Sprintf("Saved to %s", moduleConfigPath("merge")), s.window)
	})

	resetBtn := widget.NewButton("Reset", func() {
		cfg := defaultMergeConfig()
		s.applyMergeConfig(cfg)
		applyMergeControls()
		s.persistMergeConfig()
	})

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
		widget.NewLabel("Output Folder"),
		container.NewBorder(nil, nil, nil, browseDirBtn, outputDirEntry),
		widget.NewLabel("Output Filename"),
		outputFilenameEntry,
		widget.NewSeparator(),
		container.NewHBox(resetBtn, loadCfgBtn, saveCfgBtn),
		widget.NewSeparator(),
		container.NewHBox(addQueueBtn, runNowBtn),
	)

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
		return s.executeAudioJob(ctx, job, progressCallback)
	case queue.JobTypeThumb:
		return s.executeThumbJob(ctx, job, progressCallback)
	case queue.JobTypeSnippet:
		return s.executeSnippetJob(ctx, job, progressCallback)
	case queue.JobTypeAuthor:
		return s.executeAuthorJob(ctx, job, progressCallback)
	case queue.JobTypeRip:
		return s.executeRipJob(ctx, job, progressCallback)
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

	// Track success to clean up broken files on failure
	var success bool
	defer func() {
		if !success && outputPath != "" {
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

	// Source metrics (used for filters and bitrate defaults)
	sourceWidth, _ := cfg["sourceWidth"].(int)
	sourceHeight, _ := cfg["sourceHeight"].(int)
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
				videoCodec = "libsvtav1"
			default:
				videoCodec = "libx264"
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
				args = append(args, "-crf", crfVal, "-maxrate", targetBitrate, "-bufsize", targetBitrate)
			} else if strings.Contains(videoCodec, "av1") || strings.Contains(videoCodec, "svtav1") {
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
				args = append(args, "-preset", svtPreset, "-crf", crfVal, "-maxrate", targetBitrate, "-bufsize", targetBitrate)
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
			args = append(args, "-preset", svtPreset, "-crf", crfVal, "-maxrate", targetBitrate, "-bufsize", targetBitrate)
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

	cmd := utils.CreateCommand(ctx, utils.GetFFmpegPath(), args...)

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
	targetPreset, _ := cfg["targetPreset"].(string)
	sourceWidth := int(toFloat(cfg["sourceWidth"]))
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

	crfValue := 16
	switch qualityPreset {
	case "Lossless (CRF 0)":
		crfValue = 0
	case "High (CRF 18)":
		crfValue = 18
	case "Near-lossless (CRF 16)":
		crfValue = 16
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

		frameExt := strings.ToLower(aiOutputFormat)
		if frameExt == "jpeg" {
			frameExt = "jpg"
		}
		framePattern := filepath.Join(inputFramesDir, "frame_%08d."+frameExt)
		extractArgs := []string{"-y", "-hide_banner", "-i", inputPath}
		if preFilter != "" {
			extractArgs = append(extractArgs, "-vf", preFilter)
		}
		extractArgs = append(extractArgs, "-start_number", "0", framePattern)

		logFile, logPath, _ := createConversionLog(inputPath, outputPath, extractArgs)
		if logFile != nil {
			fmt.Fprintln(logFile, "Stage: extract frames for AI upscaling")
		}

		runFFmpegWithProgress := func(args []string, duration float64, startPct, endPct float64) error {
			cmd := exec.CommandContext(ctx, utils.GetFFmpegPath(), args...)
			utils.ApplyNoWindow(cmd)
			stderr, err := cmd.StderrPipe()
			if err != nil {
				return fmt.Errorf("failed to create stderr pipe: %w", err)
			}
			if err := cmd.Start(); err != nil {
				return fmt.Errorf("failed to start ffmpeg: %w", err)
			}
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				line := scanner.Text()
				if logFile != nil {
					fmt.Fprintln(logFile, line)
				}
				if strings.Contains(line, "time=") && duration > 0 {
					if idx := strings.Index(line, "time="); idx != -1 {
						timeStr := line[idx+5:]
						if spaceIdx := strings.Index(timeStr, " "); spaceIdx != -1 {
							timeStr = timeStr[:spaceIdx]
						}
						var h, m int
						var s float64
						if _, err := fmt.Sscanf(timeStr, "%d:%d:%f", &h, &m, &s); err == nil {
							currentTime := float64(h*3600+m*60) + s
							progress := startPct + ((currentTime / duration) * (endPct - startPct))
							if progressCallback != nil {
								progressCallback(progress)
							}
						}
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

		aiArgs := []string{
			"-i", inputFramesDir,
			"-o", outputFramesDir,
			"-n", aiModel,
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
		if aiFaceEnhance && logFile != nil {
			fmt.Fprintln(logFile, "Note: face enhancement requested but not supported in ncnn backend")
		}

		if logFile != nil {
			fmt.Fprintln(logFile, "Stage: Real-ESRGAN")
			fmt.Fprintf(logFile, "Command: realesrgan-ncnn-vulkan %s\n", strings.Join(aiArgs, " "))
		}

		aiCmd := exec.CommandContext(ctx, "realesrgan-ncnn-vulkan", aiArgs...)
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

		reassemblePattern := filepath.Join(outputFramesDir, "frame_%08d."+frameExt)
		reassembleArgs := []string{
			"-y",
			"-hide_banner",
			"-framerate", fmt.Sprintf("%.3f", sourceFrameRate),
			"-i", reassemblePattern,
			"-i", inputPath,
			"-map", "0:v:0",
			"-map", "1:a?",
		}

		// Final scale to ensure target height/aspect (optional)
		if targetPreset != "" && targetPreset != "Match Source" {
			finalScale := buildUpscaleFilter(targetWidth, targetHeight, method, preserveAR)
			reassembleArgs = append(reassembleArgs, "-vf", finalScale)
		}

		reassembleArgs = append(reassembleArgs,
			"-c:v", "libx264",
			"-preset", "slow",
			"-crf", strconv.Itoa(crfValue),
			"-pix_fmt", "yuv420p",
			"-c:a", "copy",
			"-shortest",
			outputPath,
		)

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
		"-crf", strconv.Itoa(crfValue),
		"-pix_fmt", "yuv420p",
		"-c:a", "copy",
		outputPath,
	)

	logFile, logPath, _ := createConversionLog(inputPath, outputPath, args)
	cmd := exec.CommandContext(ctx, utils.GetFFmpegPath(), args...)
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

// buildFFmpegCommandFromJob builds an FFmpeg command string from a queue job with INPUT/OUTPUT placeholders
func buildFFmpegCommandFromJob(job *queue.Job) string {
	if job == nil || job.Config == nil {
		return ""
	}

	cfg := job.Config
	args := []string{"-y", "-hide_banner", "-loglevel", "error"}

	// Input
	args = append(args, "-i", "INPUT")

	// Cover art if present (convert jobs only)
	if job.Type == queue.JobTypeConvert {
		if coverArtPath, _ := cfg["coverArtPath"].(string); coverArtPath != "" {
			args = append(args, "-i", "[COVER_ART]")
		}
	}

	// Hardware acceleration
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

	// Cropping
	if autoCrop, _ := cfg["autoCrop"].(bool); autoCrop {
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
		}
		if scaleFilter != "" {
			vf = append(vf, scaleFilter)
		}
	}

	// Aspect ratio handling (simplified)
	if outputAspect, _ := cfg["outputAspect"].(string); outputAspect != "" && outputAspect != "Source" {
		aspectHandling, _ := cfg["aspectHandling"].(string)
		if aspectHandling == "letterbox" {
			vf = append(vf, fmt.Sprintf("pad=iw:iw*(%s/(sar*dar)):(ow-iw)/2:(oh-ih)/2", outputAspect))
		} else if aspectHandling == "crop" {
			vf = append(vf, "crop=iw:iw/("+outputAspect+"):0:(ih-oh)/2")
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
		case videoCodec == "AV1" && hardwareAccel == "nvenc":
			codec = "av1_nvenc"
		case videoCodec == "AV1" && hardwareAccel == "qsv":
			codec = "av1_qsv"
		case videoCodec == "AV1" && hardwareAccel == "amf":
			codec = "av1_amf"
		case videoCodec == "AV1":
			codec = "libsvtav1"
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
			if strings.Contains(codec, "264") || strings.Contains(codec, "265") || codec == "libvpx-vp9" || codec == "libsvtav1" {
				args = append(args, "-crf", crfStr)
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
	if os.Getenv("VIDEOTOOLS_LOG_FILE") == "" {
		_ = os.Setenv("VIDEOTOOLS_LOG_FILE", filepath.Join(getLogsDir(), "videotools.log"))
	}
	logging.Init()
	defer logging.Close()
	defer logging.RecoverPanic() // Catch and log any panics with stack trace

	flag.Parse()
	logging.SetDebug(*debugFlag || os.Getenv("VIDEOTOOLS_DEBUG") != "")
	logging.Debug(logging.CatSystem, "starting VideoTools prototype at %s", time.Now().Format(time.RFC3339))

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

	// Use compact default size (800x600) that fits on any screen
	// Window can be resized or maximized by user using window manager controls
	w.Resize(fyne.NewSize(800, 600))
	w.CenterOnScreen()

	logging.Debug(logging.CatUI, "window initialized at 800x600 (compact default), manual resizing enabled")

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
	utils.SetTempDir(state.convert.TempDir)

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
	// Add to end of queue
	if err := s.addConvertToQueue(false); err != nil {
		dialog.ShowError(err, s.window)
	} else {
		// Update queue button to show new count
		s.updateQueueButtonLabel()
		// Auto-start queue if not already running
		if s.jobQueue != nil && !s.jobQueue.IsRunning() && !s.convertBusy {
			s.jobQueue.Start()
			logging.Debug(logging.CatUI, "queue auto-started after adding job")
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
		logging.Debug(logging.CatUI, "Added %d jobs to queue", count)
		// Auto-start queue if not already running
		if s.jobQueue != nil && !s.jobQueue.IsRunning() && !s.convertBusy {
			s.jobQueue.Start()
			logging.Debug(logging.CatUI, "queue auto-started after adding %d jobs", count)
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
		logging.Debug(logging.CatSystem, "started queue from Convert Now")
	}

	// Clear the loaded video from convert module
	s.clearVideo()

	// Show success message
	if s.jobQueue != nil && s.jobQueue.IsRunning() {
		dialog.ShowInformation("Convert", "Added to top of queue! View progress in Job Queue.", s.window)
	} else {
		dialog.ShowInformation("Convert", "Conversion started! View progress in Job Queue.", s.window)
	}
}

// buildFormatBadge creates a color-coded badge for a format option
// Example: "MKV (AV1)" → teal badge with "MKV (AV1)" text
func buildFormatBadge(formatLabel string) fyne.CanvasObject {
	// Parse format label: "MKV (AV1)" → containerName: "mkv"
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
	bg.SetMinSize(fyne.NewSize(100, 28))

	// Create label
	label := canvas.NewText(codecName, color.White)
	label.TextStyle = fyne.TextStyle{Bold: true}
	label.Alignment = fyne.TextAlignCenter
	label.TextSize = 12

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
	bg.SetMinSize(fyne.NewSize(100, 28))

	// Create label
	label := canvas.NewText(codecName, color.White)
	label.TextStyle = fyne.TextStyle{Bold: true}
	label.Alignment = fyne.TextAlignCenter
	label.TextSize = 12

	// Stack background and label
	return container.NewMax(bg, container.NewCenter(label))
}

func buildConvertView(state *appState, src *videoSource) fyne.CanvasObject {
	convertColor := moduleColor("convert")

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
		buildCommandPreview       func()
	}

	uiState := &convertUIState{
		quality:       state.convert.Quality,
		resolution:    state.convert.TargetResolution,
		aspect:        state.convert.OutputAspect,
		bitratePreset: state.convert.BitratePreset,
	}

	// Debouncing helper - delays function execution until user stops typing
	createDebouncedCallback := func(delay time.Duration, callback func(string)) func(string) {
		var timer *time.Timer
		var mu sync.Mutex

		return func(value string) {
			mu.Lock()
			defer mu.Unlock()

			if timer != nil {
				timer.Stop()
			}

			timer = time.AfterFunc(delay, func() {
				callback(value)
			})
		}
	}

	// Input validation helpers
	validateCRF := func(input string) error {
		if input == "" {
			return nil // Empty is valid (uses quality preset)
		}
		val, err := strconv.Atoi(input)
		if err != nil {
			return fmt.Errorf("CRF must be a number")
		}
		if val < 0 || val > 51 {
			return fmt.Errorf("CRF must be between 0 and 51")
		}
		return nil
	}

	validateBitrate := func(input string, unit string) error {
		if input == "" {
			return nil // Empty is valid
		}
		val, err := strconv.ParseFloat(input, 64)
		if err != nil {
			return fmt.Errorf("Bitrate must be a number")
		}
		if val <= 0 {
			return fmt.Errorf("Bitrate must be positive")
		}
		// Warn on extremes
		kbps := val
		switch unit {
		case "Mbps":
			kbps *= 1000
		case "Gbps":
			kbps *= 1000000
		}
		// Warnings logged but don't fail validation
		if kbps < 100 {
			logging.Debug(logging.CatUI, "Very low bitrate (%.0f kbps) may produce poor quality", kbps)
		}
		if kbps > 50000 {
			logging.Debug(logging.CatUI, "Very high bitrate (%.0f kbps) approaching lossless", kbps)
		}
		return nil
	}

	validateFileSize := func(input string) error {
		if input == "" {
			return nil // Empty is valid
		}
		val, err := strconv.ParseFloat(input, 64)
		if err != nil {
			return fmt.Errorf("File size must be a number")
		}
		if val <= 0 {
			return fmt.Errorf("File size must be positive")
		}
		return nil
	}

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

	// State setters with automatic widget synchronization
	setQuality := func(val string) {
		if uiState.quality == val {
			return // No change
		}
		uiState.quality = val
		state.convert.Quality = val

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
		if userSet {
			state.convert.AspectUserSet = true
		}

		for _, w := range uiState.aspectWidgets {
			w.SetSelected(val)
		}

		for _, cb := range uiState.onAspectChange {
			cb(val)
		}
		if uiState.updateAspectBoxVisibility != nil {
			uiState.updateAspectBoxVisibility()
		}
		logging.Debug(logging.CatUI, "target aspect set to %s", val)
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

	clearCompletedBtn := widget.NewButton("⌫", func() {
		state.clearCompletedJobs()
	})
	clearCompletedBtn.Importance = widget.LowImportance

	// Command Preview toggle button
	cmdPreviewBtn := widget.NewButton("Command Preview", func() {
		state.convertCommandPreviewShow = !state.convertCommandPreviewShow
		state.showModule("convert")
	})
	cmdPreviewBtn.Importance = widget.LowImportance

	// Update button text and state based on preview visibility and source
	if src == nil {
		cmdPreviewBtn.Disable()
	} else if state.convertCommandPreviewShow {
		cmdPreviewBtn.SetText("Hide Preview")
	} else {
		cmdPreviewBtn.SetText("Show Preview")
	}

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

	// Forward declare functions needed by formatContainer callback
	var updateDVDOptions func()
	var buildCommandPreview func()

	// Declare output widgets early to fix variable order issues
	var outputExtLabel *widget.Label
	var outputExtBG *canvas.Rectangle
	var updateOutputHint func()

	var formatLabels []string
	for _, opt := range formatOptions {
		formatLabels = append(formatLabels, opt.Label)
	}

	// Format selector
	formatColors := ui.BuildFormatColorMap(formatLabels)
	formatContainer := ui.NewColoredSelect(formatLabels, formatColors, func(selected string) {
		for _, opt := range formatOptions {
			if opt.Label == selected {
				state.convert.SelectedFormat = opt
				logging.Debug(logging.CatUI, "format selected: %s", selected)
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
				if buildCommandPreview != nil {
					buildCommandPreview()
				}
				break
			}
		}
	}, state.window)
	formatContainer.SetSelected(state.convert.SelectedFormat.Label)

	getOutputDir := func() string {
		if strings.TrimSpace(state.convert.OutputDir) != "" {
			return state.convert.OutputDir
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

	outputHint := widget.NewLabel(fmt.Sprintf("Output file: %s", getOutputPathPreview()))
	outputHint.Wrapping = fyne.TextWrapWord
	// Wrap hint in padded container to ensure proper text wrapping in narrow windows
	outputHintContainer := container.NewPadded(outputHint)

	updateOutputHint = func() {
		outputHint.SetText(fmt.Sprintf("Output file: %s", getOutputPathPreview()))
	}

	// DVD-specific aspect ratio selector (only shown for DVD formats)
	dvdAspectOpts := []string{"4:3", "16:9"}
	dvdAspectSelect := widget.NewSelect(dvdAspectOpts, func(value string) {
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

	// Chapter preservation
	preserveChaptersCheck := widget.NewCheck("Keep chapters", func(checked bool) {
		state.convert.PreserveChapters = checked
	})
	preserveChaptersCheck.SetChecked(state.convert.PreserveChapters)

	// Forward declarations for encoding controls (used in reset/update callbacks)
	var (
		bitrateModeSelect          *widget.Select
		bitratePresetSelect        *widget.Select
		crfPresetSelect            *widget.Select
		crfEntry                   *widget.Entry
		manualCrfRow               *fyne.Container
		videoBitrateEntry          *widget.Entry
		manualBitrateRow           *fyne.Container
		targetFileSizeSelect       *widget.Select
		targetFileSizeEntry        *widget.Entry
		qualitySelectSimple        *ui.ColoredSelect
		qualitySelectAdv           *ui.ColoredSelect
		qualitySectionSimple       fyne.CanvasObject
		qualitySectionAdv          fyne.CanvasObject
		simpleBitrateSelect        *widget.Select
		crfContainer               *fyne.Container
		bitrateContainer           *fyne.Container
		targetSizeContainer        *fyne.Container
		resetConvertDefaults       func()
		tabs                       *container.AppTabs
		simpleEncodingSection      *fyne.Container
		advancedVideoEncodingBlock *fyne.Container
		audioEncodingSection       *fyne.Container
		audioCodecSelect           *ui.ColoredSelect
	)
	var (
		updateEncodingControls  func()
		updateQualityVisibility func()
		updateRemuxVisibility   func()
		updateQualityOptions    func() // Update quality dropdown based on codec
	)

	// Base quality options (without lossless)
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
	qualityOptions := baseQualityOptions
	if codecSupportsLossless(state.convert.VideoCodec) {
		qualityOptions = append(qualityOptions, "Lossless")
	}

	// Quality select widgets - use state manager to eliminate sync flags
	// Convert quality selects to ColoredSelect and register with state manager
	qualityColorMap := ui.BuildQualityColorMap(qualityOptions)

	qualitySelectSimple = ui.NewColoredSelect(qualityOptions, qualityColorMap, func(value string) {
		logging.Debug(logging.CatUI, "quality preset %s (simple)", value)
		setQuality(value)
		if buildCommandPreview != nil {
			buildCommandPreview()
		}
	}, state.window)

	qualitySelectAdv = ui.NewColoredSelect(qualityOptions, qualityColorMap, func(value string) {
		logging.Debug(logging.CatUI, "quality preset %s (advanced)", value)
		setQuality(value)
		if buildCommandPreview != nil {
			buildCommandPreview()
		}
	}, state.window)

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
			newOptions = append(baseQualityOptions, "Lossless")
		} else {
			// H.264, MPEG-2, etc. don't support lossless
			newOptions = baseQualityOptions
			// If currently set to Lossless, fall back to Near-Lossless
			if state.convert.Quality == "Lossless" {
				state.convert.Quality = "Near-Lossless (CRF 16)"
			}
		}

		// Update options and color map for all registered quality widgets
		qualityColorMap := ui.BuildQualityColorMap(newOptions)
		for _, w := range uiState.qualityWidgets {
			w.UpdateOptions(newOptions, qualityColorMap)
		}

		// Use state manager to synchronize selected value across all widgets
		setQuality(state.convert.Quality)
	}

	outputEntry := widget.NewEntry()
	outputEntry.SetText(state.convert.OutputBase)
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
		row := container.NewBorder(nil, nil, nil, right, entry)
		return container.NewMax(bg, container.NewPadded(row))
	}

	outputDirRow := buildOutputRow(outputDirEntry, outputDirBtn)
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

	autoNameCheck = widget.NewCheck("Auto-name from metadata", func(checked bool) {
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

	appendSuffixCheck := widget.NewCheck("Append \"-convert\" to filename", func(checked bool) {
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

	inverseCheck := widget.NewCheck("Smart Inverse Telecine", func(checked bool) {
		state.convert.InverseTelecine = checked
	})
	inverseCheck.Checked = state.convert.InverseTelecine
	inverseHint := widget.NewLabel(state.convert.InverseAutoNotes)

	makePanelButton := func(label string, onTap func()) (*widget.Button, fyne.CanvasObject) {
		btn := widget.NewButton(label, onTap)
		btn.Importance = widget.LowImportance
		bg := canvas.NewRectangle(utils.MustHex("#344256"))
		bg.CornerRadius = 8
		bg.SetMinSize(fyne.NewSize(0, 36))
		return btn, container.NewMax(bg, container.NewPadded(btn))
	}

	// Interlacing Analysis Button (Simple Menu)
	var analyzeInterlaceBtn *widget.Button
	var analyzeInterlaceView fyne.CanvasObject
	analyzeInterlaceBtn, analyzeInterlaceView = makePanelButton("Analyze Interlacing", func() {
		if src == nil {
			dialog.ShowInformation("Interlacing Analysis", "Load a video first.", state.window)
			return
		}
		go func() {
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				analyzeInterlaceBtn.SetText("Analyzing...")
				analyzeInterlaceBtn.Disable()
			}, false)

			detector := interlace.NewDetector(utils.GetFFmpegPath(), utils.GetFFprobePath())
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

	// Auto-crop controls
	autoCropCheck := widget.NewCheck("Auto-Detect Black Bars", func(checked bool) {
		state.convert.AutoCrop = checked
		logging.Debug(logging.CatUI, "auto-crop set to %v", checked)
	})
	autoCropCheck.Checked = state.convert.AutoCrop

	var detectCropBtn *widget.Button
	var detectCropView fyne.CanvasObject
	detectCropBtn, detectCropView = makePanelButton("Detect Crop", func() {
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

	rotationOptions := []string{"0°", "90° CW", "180°", "270° CW"}
	rotationSelect := widget.NewSelect(rotationOptions, func(value string) {
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
	var (
		targetAspectSelect       *widget.Select
		targetAspectSelectSimple *widget.Select
	)
	// Aspect select widget - uses state manager to eliminate sync flag
	targetAspectSelect = widget.NewSelect(aspectTargets, func(value string) {
		setAspect(value, true)
	})
	if state.convert.OutputAspect == "" {
		state.convert.OutputAspect = "Source"
	}
	targetAspectSelect.SetSelected(state.convert.OutputAspect)
	targetAspectHint := widget.NewLabel("Pick desired output aspect (default Source).")
	targetAspectHint.Wrapping = fyne.TextWrapWord
	// Wrap hint in padded container to ensure proper text wrapping in narrow windows
	targetAspectHintContainer := container.NewPadded(targetAspectHint)

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

	backgroundHint := widget.NewLabel("Crop removes edges, Letterbox/Pillarbox adds black bars to fit.")
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

	aspectOptions.OnChanged = func(value string) {
		logging.Debug(logging.CatUI, "aspect handling set to %s", value)
		state.convert.AspectHandling = value
	}

	// Cover art display on one line
	coverDisplay = widget.NewLabel("Cover Art: " + state.convert.CoverLabel())

	// Create color-coded video codec select widget with colored dropdown items
	videoCodecOptions := []string{"H.264", "H.265", "VP9", "AV1", "MPEG-2", "Copy"}
	videoCodecColorMap := ui.BuildVideoCodecColorMap(videoCodecOptions)
	videoCodecSelect := ui.NewColoredSelect(videoCodecOptions, videoCodecColorMap, func(value string) {
		state.convert.VideoCodec = value
		logging.Debug(logging.CatUI, "video codec set to %s", value)
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

	encoderPresetOptions := []string{"veryslow", "slower", "slow", "medium", "fast", "faster", "veryfast", "superfast", "ultrafast"}
	encoderPresetSelect := widget.NewSelect(encoderPresetOptions, func(value string) {
		state.convert.EncoderPreset = value
		logging.Debug(logging.CatUI, "encoder preset set to %s", value)
		updateEncoderPresetHint(value)
		if buildCommandPreview != nil {
			buildCommandPreview()
		}
	})
	encoderPresetSelect.SetSelected(state.convert.EncoderPreset)
	updateEncoderPresetHint(state.convert.EncoderPreset)

	// Simple mode preset dropdown
	simplePresetSelect := widget.NewSelect(encoderPresetOptions, func(value string) {
		state.convert.EncoderPreset = value
		logging.Debug(logging.CatUI, "simple preset set to %s", value)
		updateEncoderPresetHint(value)
		if buildCommandPreview != nil {
			buildCommandPreview()
		}
	})
	simplePresetSelect.SetSelected(state.convert.EncoderPreset)

	// Settings management for batch operations
	settingsInfoLabel := widget.NewLabel("Settings persist across videos. Change them anytime to affect all subsequent videos.")
	settingsInfoLabel.Alignment = fyne.TextAlignCenter
	settingsInfoLabel.Wrapping = fyne.TextWrapWord
	// Wrap in padded container for proper text wrapping in narrow windows
	settingsInfoContainer := container.NewPadded(settingsInfoLabel)

	cacheDirLabel := widget.NewLabelWithStyle("Cache/Temp Directory", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	cacheDirEntry := widget.NewEntry()
	cacheDirEntry.SetPlaceHolder("System temp (recommended SSD)")
	cacheDirEntry.SetText(state.convert.TempDir)
	cacheDirHint := widget.NewLabel("Use an SSD for best performance. Leave blank to use system temp.")
	cacheDirHint.Wrapping = fyne.TextWrapWord
	// Wrap in padded container for proper text wrapping in narrow windows
	cacheDirHintContainer := container.NewPadded(cacheDirHint)
	cacheDirEntry.OnChanged = func(val string) {
		state.convert.TempDir = strings.TrimSpace(val)
		utils.SetTempDir(state.convert.TempDir)
	}
	cacheBrowseBtn := widget.NewButton("Browse...", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			cacheDirEntry.SetText(uri.Path())
			state.convert.TempDir = uri.Path()
			utils.SetTempDir(state.convert.TempDir)
		}, state.window)
	})
	cacheBrowseBtn.Importance = widget.MediumImportance
	cacheUseSystemBtn := widget.NewButton("Use System Temp", func() {
		cacheDirEntry.SetText("")
		state.convert.TempDir = ""
		utils.SetTempDir("")
	})
	cacheUseSystemBtn.Importance = widget.LowImportance

	resetSettingsBtn := widget.NewButton("Reset to Defaults", func() {
		if resetConvertDefaults != nil {
			resetConvertDefaults()
		}
	})
	resetSettingsBtn.Importance = widget.MediumImportance

	settingsContent := container.NewVBox(
		settingsInfoContainer,
		widget.NewSeparator(),
		cacheDirLabel,
		container.NewBorder(nil, nil, nil, cacheBrowseBtn, cacheDirEntry),
		cacheUseSystemBtn,
		cacheDirHintContainer,
		resetSettingsBtn,
	)
	settingsContent.Hide()

	settingsVisible := false
	toggleSettingsLabel := widget.NewLabel("Show Batch Settings")
	toggleSettingsLabel.Wrapping = fyne.TextWrapWord
	toggleSettingsLabel.Alignment = fyne.TextAlignCenter

	var toggleSettingsBtn *widget.Button
	toggleSettingsBtn = widget.NewButton("", func() {
		if settingsVisible {
			settingsContent.Hide()
			toggleSettingsLabel.SetText("Show Batch Settings")
		} else {
			settingsContent.Show()
			toggleSettingsLabel.SetText("Hide Batch Settings")
		}
		settingsVisible = !settingsVisible
	})
	toggleSettingsBtn.Importance = widget.LowImportance

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

	// Bitrate Mode with descriptions
	bitrateModeOptions := []string{
		"CRF (Constant Rate Factor)",
		"CBR (Constant Bitrate)",
		"VBR (Variable Bitrate)",
		"Target Size (Calculate from file size)",
	}
	bitrateModeMap := map[string]string{
		"CRF (Constant Rate Factor)":             "CRF",
		"CBR (Constant Bitrate)":                 "CBR",
		"VBR (Variable Bitrate)":                 "VBR",
		"Target Size (Calculate from file size)": "Target Size",
	}
	reverseMap := map[string]string{
		"CRF":         "CRF (Constant Rate Factor)",
		"CBR":         "CBR (Constant Bitrate)",
		"VBR":         "VBR (Variable Bitrate)",
		"Target Size": "Target Size (Calculate from file size)",
	}
	bitrateModeSelect = widget.NewSelect(bitrateModeOptions, func(value string) {
		// Extract short code from label
		if shortCode, ok := bitrateModeMap[value]; ok {
			state.convert.BitrateMode = shortCode
		} else {
			state.convert.BitrateMode = value
		}
		logging.Debug(logging.CatUI, "bitrate mode set to %s", state.convert.BitrateMode)
		if updateEncodingControls != nil {
			updateEncodingControls()
		}
		if buildCommandPreview != nil {
			buildCommandPreview()
		}
	})
	// Set selected using full label
	if fullLabel, ok := reverseMap[state.convert.BitrateMode]; ok {
		bitrateModeSelect.SetSelected(fullLabel)
	} else {
		bitrateModeSelect.SetSelected(state.convert.BitrateMode)
	}

	// Manual CRF entry
	// CRF entry with debouncing (300ms delay) and validation
	crfEntry = widget.NewEntry()
	crfEntry.SetPlaceHolder("Auto (from Quality preset)")
	crfEntry.SetText(state.convert.CRF)
	crfEntry.Validator = validateCRF
	crfEntry.OnChanged = createDebouncedCallback(300*time.Millisecond, func(val string) {
		if validateCRF(val) == nil {
			state.convert.CRF = val
			if buildCommandPreview != nil {
				buildCommandPreview()
			}
		}
	})

	manualCrfRow = container.NewVBox(
		widget.NewLabelWithStyle("Manual CRF (overrides Quality preset)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		crfEntry,
	)
	manualCrfRow.Hide()

	crfPresetOptions := []string{
		"Auto (from Quality preset)",
		"18 (High)",
		"20 (Balanced)",
		"23 (Standard)",
		"28 (Draft)",
		"Manual",
	}
	crfPresetSelect = widget.NewSelect(crfPresetOptions, func(value string) {
		switch value {
		case "Auto (from Quality preset)":
			state.convert.CRF = ""
			crfEntry.SetText("")
			manualCrfRow.Hide()
		case "18 (High)":
			state.convert.CRF = "18"
			crfEntry.SetText("18")
			manualCrfRow.Hide()
		case "20 (Balanced)":
			state.convert.CRF = "20"
			crfEntry.SetText("20")
			manualCrfRow.Hide()
		case "23 (Standard)":
			state.convert.CRF = "23"
			crfEntry.SetText("23")
			manualCrfRow.Hide()
		case "28 (Draft)":
			state.convert.CRF = "28"
			crfEntry.SetText("28")
			manualCrfRow.Hide()
		case "Manual":
			manualCrfRow.Show()
		}
		if buildCommandPreview != nil {
			buildCommandPreview()
		}
	})
	switch state.convert.CRF {
	case "":
		crfPresetSelect.SetSelected("Auto (from Quality preset)")
	case "18":
		crfPresetSelect.SetSelected("18 (High)")
	case "20":
		crfPresetSelect.SetSelected("20 (Balanced)")
	case "23":
		crfPresetSelect.SetSelected("23 (Standard)")
	case "28":
		crfPresetSelect.SetSelected("28 (Draft)")
	default:
		crfPresetSelect.SetSelected("Manual")
		manualCrfRow.Show()
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

	// Create CRF container (crfEntry already initialized)
	crfContainer = container.NewVBox(
		widget.NewLabelWithStyle("CRF Preset", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		crfPresetSelect,
		manualCrfRow,
	)

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
	manualBitrateLabel := widget.NewLabelWithStyle("Manual Bitrate", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	manualBitrateRow = container.NewVBox(manualBitrateLabel, manualBitrateInput)
	manualBitrateRow.Hide()

	// Create bitrate container now that bitratePresetSelect is initialized
	bitrateContainer = container.NewVBox(
		widget.NewLabelWithStyle("Bitrate Preset", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		bitratePresetSelect,
		manualBitrateRow,
	)

	// Simple resolution selector (separate widget to avoid double-parent issues)
	resolutionOptionsSimple := []string{
		"Source", "360p", "480p", "540p", "720p", "1080p", "1440p", "4K", "8K",
		"2X (relative)", "4X (relative)",
		"NTSC (720×480)", "PAL (720×540)", "PAL (720×576)",
	}
	// Resolution select (Simple mode) - uses state manager
	resolutionSelectSimple := widget.NewSelect(resolutionOptionsSimple, func(value string) {
		logging.Debug(logging.CatUI, "target resolution set to %s (simple)", value)
		setResolution(value)
	})
	resolutionSelectSimple.SetSelected(state.convert.TargetResolution)

	// Simple aspect selector (separate widget) - uses state manager
	targetAspectSelectSimple = widget.NewSelect(aspectTargets, func(value string) {
		setAspect(value, true)
	})
	if state.convert.OutputAspect == "" {
		state.convert.OutputAspect = "Source"
	}
	targetAspectSelectSimple.SetSelected(state.convert.OutputAspect)

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
		widget.NewLabelWithStyle("Target File Size", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
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

		// Move to CBR for predictable output when a preset is chosen
		if preset.Bitrate != "" && state.convert.BitrateMode != "CBR" && state.convert.BitrateMode != "VBR" {
			state.convert.BitrateMode = "CBR"
			bitrateModeSelect.SetSelected("CBR")
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

	updateEncodingControls = func() {
		mode := state.convert.BitrateMode
		isLossless := state.convert.Quality == "Lossless"
		supportsLossless := codecSupportsLossless(state.convert.VideoCodec)

		hint := ""
		showCRF := mode == "CRF" || mode == ""
		showBitrate := mode == "CBR" || mode == "VBR"
		showTarget := mode == "Target Size"

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
				if crfPresetSelect != nil {
					crfPresetSelect.SetSelected("Manual")
				}
				if manualCrfRow != nil {
					manualCrfRow.Show()
				}
				hint = "Lossless mode with CRF 0. Perfect quality preservation for H.265/AV1."
			case "CBR":
				hint = "Lossless quality with constant bitrate. May achieve smaller file size than pure lossless CRF."
			case "VBR":
				hint = "Lossless quality with variable bitrate. Efficient file size while maintaining lossless quality."
			case "Target Size":
				hint = "Lossless quality with target size. Calculates bitrate to achieve exact file size with best possible quality."
			}
		} else {
			crfEntry.Enable()
			switch mode {
			case "CRF", "":
				// Show only CRF controls
				hint = "CRF mode: Constant quality - file size varies. Lower CRF = better quality."
			case "CBR":
				// Show only bitrate controls
				hint = "CBR mode: Constant bitrate - predictable file size, variable quality. Use for strict size requirements or streaming."
			case "VBR":
				// Show only bitrate controls
				hint = "VBR mode: Variable bitrate - targets average bitrate with 2x peak cap. Efficient quality. Uses 2-pass encoding."
			case "Target Size":
				// Show only target size controls
				hint = "Target Size mode: Calculates bitrate to hit exact file size. Best for strict size limits."
			}
		}

		if showCRF {
			crfContainer.Show()
		} else {
			crfContainer.Hide()
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
		if buildCommandPreview != nil {
			buildCommandPreview()
		}
	}
	updateEncodingControls()

	// Target Resolution (advanced)
	resolutionOptions := []string{
		"Source", "720p", "1080p", "1440p", "4K", "8K",
		"2X (relative)", "4X (relative)",
		"NTSC (720×480)", "PAL (720×540)", "PAL (720×576)",
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
			frameRateHint.SetText(fmt.Sprintf("Converting %.0f → %.0f fps: ~%.0f%% smaller file",
				sourceFPS, targetFPS, reduction))
		} else if targetFPS > sourceFPS {
			frameRateHint.SetText(fmt.Sprintf("⚠ Upscaling from %.0f to %.0f fps (may cause judder)",
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
	motionInterpCheck := widget.NewCheck("Use Motion Interpolation (slower, smoother frame rate changes)", func(checked bool) {
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
	hwAccelHint := widget.NewLabel("Auto picks the best GPU path; if encode fails, switch to none (software).")
	hwAccelHint.Wrapping = fyne.TextWrapWord
	// Wrap hint in padded container to ensure proper text wrapping in narrow windows
	hwAccelHintContainer := container.NewPadded(hwAccelHint)
	hwAccelOptions := []string{"auto", "none", "nvenc", "amf", "vaapi", "qsv", "videotoolbox"}
	hwAccelSelect := widget.NewSelect(hwAccelOptions, func(value string) {
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

	// Create color-coded audio codec select widget with colored dropdown items
	audioCodecOptions := []string{"AAC", "Opus", "MP3", "FLAC", "Copy"}
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
		// videoCodecSelect.Enable()
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
			// videoCodecSelect.Disable()

			state.convert.VideoBitrate = dvdBitrate
			if setManualBitrate != nil {
				setManualBitrate(dvdBitrate)
			} else {
				videoBitrateEntry.SetText(dvdBitrate)
			}
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
			crfContainer.Hide()
			targetSizeContainer.Hide()
			// Show bitrate controls since DVD uses CBR
			bitrateContainer.Show()

			dvdInfoLabel.SetText(fmt.Sprintf("%s\nLocked: resolution, frame rate, aspect, codec, pixel format, bitrate, and GPU toggles for DVD compliance.", dvdNotes))
		} else {
			dvdAspectBox.Hide()
			// Re-enable normal visibility control through updateEncodingControls
			bitratePresetSelect.Show()
			simpleBitrateSelect.Show()
			if updateEncodingControls != nil {
				updateEncodingControls()
			}
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
		hideQuality := state.convert.BitrateMode != "" && state.convert.BitrateMode != "CRF"

		if qualitySectionSimple != nil {
			if hide || hideQuality {
				qualitySectionSimple.Hide()
			} else {
				qualitySectionSimple.Show()
			}
		}
		if qualitySectionAdv != nil {
			if hide || hideQuality {
				qualitySectionAdv.Hide()
			} else {
				qualitySectionAdv.Show()
			}
		}
	}

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
			videoCodecSelect.Enable()
		}
		if audioCodecSelect != nil {
			if remux {
				audioCodecSelect.Disable()
			} else {
				audioCodecSelect.Enable()
			}
		}
		if qualitySectionAdv != nil {
			if remux {
				qualitySectionAdv.Hide()
			} else {
				qualitySectionAdv.Show()
			}
		}
		if encoderPresetSelect != nil {
			if remux {
				encoderPresetSelect.Disable()
			} else {
				encoderPresetSelect.Enable()
			}
		}
		if bitrateModeSelect != nil {
			if remux {
				bitrateModeSelect.Disable()
			} else {
				bitrateModeSelect.Enable()
			}
		}
		if crfContainer != nil {
			if remux {
				crfContainer.Hide()
			} else {
				crfContainer.Show()
			}
		}
		if bitrateContainer != nil {
			if remux {
				bitrateContainer.Hide()
			} else {
				bitrateContainer.Show()
			}
		}
		if targetSizeContainer != nil {
			if remux {
				targetSizeContainer.Hide()
			} else {
				targetSizeContainer.Show()
			}
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

	simpleEncodingSection = container.NewVBox(
		qualitySectionSimple,
		widget.NewLabelWithStyle("Encoder Speed/Quality", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Choose slower for better compression, faster for speed"),
		widget.NewLabelWithStyle("Encoder Preset", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		simplePresetSelect,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Bitrate (simple presets)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		simpleBitrateSelect,
	)

	// Simple mode options - minimal controls, aspect locked to Source
	simpleOptions := container.NewVBox(
		widget.NewLabelWithStyle("═══ OUTPUT ═══", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Format", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		formatContainer,
		chapterWarningLabel, // Warning when converting chapters to DVD
		preserveChaptersCheck,
		dvdAspectBox, // DVD options appear here when DVD format selected
		widget.NewLabelWithStyle("Output Folder", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		outputDirRow,
		widget.NewLabelWithStyle("Output Filename", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		outputNameRow,
		outputHintContainer,
		appendSuffixCheck,
		widget.NewSeparator(),
		simpleEncodingSection,
		widget.NewLabelWithStyle("Target Resolution", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		resolutionSelectSimple,
		widget.NewLabelWithStyle("Frame Rate", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		frameRateSelect,
		motionInterpCheck,
		widget.NewLabelWithStyle("Target Aspect Ratio", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		targetAspectSelectSimple,
		targetAspectHintContainer,
		layout.NewSpacer(),
	)

	// Advanced mode options - full controls with organized sections
	videoCodecLabel := widget.NewLabelWithStyle("Video Codec", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	presetLabel := widget.NewLabelWithStyle("Encoder Preset (speed vs quality)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	videoCodecRow := ui.NewRatioRow(videoCodecLabel, presetLabel, 0.3)
	videoCodecControls := ui.NewRatioRow(
		container.NewPadded(videoCodecContainer),
		container.NewPadded(encoderPresetSelect),
		0.3,
	)

	advancedVideoEncodingBlock = container.NewVBox(
		widget.NewLabelWithStyle("═══ VIDEO ENCODING ═══", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		videoCodecRow,
		videoCodecControls,
		encoderPresetHintContainer,
		qualitySectionAdv,
		widget.NewLabelWithStyle("Bitrate Mode", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		bitrateModeSelect,
		crfContainer,
		bitrateContainer,
		targetSizeContainer,
		encodingHintContainer,
		widget.NewLabelWithStyle("Target Resolution", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		resolutionSelect,
		widget.NewLabelWithStyle("Frame Rate", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		frameRateSelect,
		frameRateHintContainer,
		motionInterpCheck,
		widget.NewLabelWithStyle("Pixel Format", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		pixelFormatSelect,
		widget.NewLabelWithStyle("Hardware Acceleration", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		hwAccelSelect,
		hwAccelHintContainer,
		twoPassCheck,
	)

	audioEncodingSection = container.NewVBox(
		widget.NewLabelWithStyle("═══ AUDIO ENCODING ═══", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Audio Codec", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		audioCodecContainer,
		widget.NewLabelWithStyle("Audio Bitrate", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		audioBitrateSelect,
		widget.NewLabelWithStyle("Audio Channels", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		audioChannelsSelect,
	)

	advancedOptions := container.NewVBox(
		widget.NewLabelWithStyle("═══ OUTPUT ═══", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Format", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		formatContainer,
		chapterWarningLabel, // Warning when converting chapters to DVD
		preserveChaptersCheck,
		dvdAspectBox, // DVD options appear here when DVD format selected
		widget.NewLabelWithStyle("Output Folder", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		outputDirRow,
		widget.NewLabelWithStyle("Output Filename", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		outputNameRow,
		outputHintContainer,
		appendSuffixCheck,
		widget.NewSeparator(),
		advancedVideoEncodingBlock,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("═══ ASPECT RATIO ═══", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Target Aspect Ratio", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		targetAspectSelect,
		targetAspectHintContainer,
		aspectBox,
		widget.NewSeparator(),

		audioEncodingSection,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("═══ AUTO-CROP ═══", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		autoCropCheck,
		detectCropView,
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
		analyzeInterlaceView,
		inverseCheck,
		inverseHint,
		layout.NewSpacer(),
	)

	resetConvertDefaults = func() {
		state.convert = defaultConvertConfig()
		logging.Debug(logging.CatUI, "convert settings reset to defaults")

		tabs.SelectIndex(0)
		state.convert.Mode = "Simple"

		// Format selection handled below in UI redesign
		videoCodecSelect.SetSelected(state.convert.VideoCodec)
		qualitySelectSimple.SetSelected(state.convert.Quality)
		qualitySelectAdv.SetSelected(state.convert.Quality)
		simplePresetSelect.SetSelected(state.convert.EncoderPreset)
		encoderPresetSelect.SetSelected(state.convert.EncoderPreset)
		bitrateModeSelect.SetSelected(reverseMap[state.convert.BitrateMode])
		bitratePresetSelect.SetSelected(state.convert.BitratePreset)
		simpleBitrateSelect.SetSelected(state.convert.BitratePreset)
		crfEntry.SetText(state.convert.CRF)
		if crfPresetSelect != nil {
			crfPresetSelect.SetSelected("Auto (from Quality preset)")
		}
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
		cacheDirEntry.SetText(state.convert.TempDir)
		utils.SetTempDir(state.convert.TempDir)
		inverseCheck.SetChecked(state.convert.InverseTelecine)
		inverseHint.SetText(state.convert.InverseAutoNotes)
		coverLabel.SetText(state.convert.CoverLabel())
		if coverDisplay != nil {
			coverDisplay.SetText("Cover Art: " + state.convert.CoverLabel())
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

	tabs = container.NewAppTabs(
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

	if !state.convertScrollShortcuts && state.window != nil {
		state.convertScrollShortcuts = true
		isTextEntryFocused := func() bool {
			c := state.window.Canvas()
			if c == nil {
				return false
			}
			switch c.Focused().(type) {
			case *widget.Entry, *widget.MultiLineEntry:
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
	optionsPanel := container.NewMax(optionsRect, container.NewPadded(tabs))

	// Initialize snippet settings defaults
	if state.snippetLength == 0 {
		state.snippetLength = 20 // Default to 20 seconds
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

	snippetConfigRow.Hide()
	snippetOptionsVisible := false
	var snippetOptionsBtn *widget.Button
	snippetOptionsBtn = widget.NewButton("Convert Options", func() {
		if snippetOptionsVisible {
			snippetConfigRow.Hide()
			snippetOptionsBtn.SetText("Convert Options")
		} else {
			snippetConfigRow.Show()
			snippetOptionsBtn.SetText("Hide Options")
		}
		snippetOptionsVisible = !snippetOptionsVisible
	})
	snippetOptionsBtn.Importance = widget.LowImportance
	if src == nil {
		snippetOptionsBtn.Disable()
	}

	var snippetRow fyne.CanvasObject
	if snippetAllBtn != nil {
		snippetRow = container.NewHBox(snippetBtn, snippetAllBtn, snippetOptionsBtn, layout.NewSpacer(), snippetHint)
	} else {
		snippetRow = container.NewHBox(snippetBtn, snippetOptionsBtn, layout.NewSpacer(), snippetHint)
	}

	// Stack video and metadata with 10px spacing between them
	// Create a 10px spacer using a container with fixed size
	spacerRect := canvas.NewRectangle(color.Transparent)
	spacerRect.SetMinSize(fyne.NewSize(1, 10))
	spacer := container.NewMax(spacerRect)
	spacer.Resize(fyne.NewSize(1, 10))

	leftColumn := container.NewVBox(videoPanel, spacer, metaPanel)

	// Add minimal spacing (10px) between left and right panels
	horizontalSpacer := canvas.NewRectangle(color.Transparent)
	horizontalSpacer.SetMinSize(fyne.NewSize(10, 1))

	// Split: left side (video + metadata) takes 50% | right side (options) takes 50%
	mainSplit := container.NewHSplit(
		leftColumn,
		optionsPanel)
	mainSplit.SetOffset(0.5) // 50/50 split

	// Add horizontal padding around the split (10px on each side)
	mainContent := container.NewPadded(mainSplit)

	resetBtn := widget.NewButton("Reset", func() {
		if resetConvertDefaults != nil {
			resetConvertDefaults()
		}
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

	// Add All to Queue button (only shown when multiple videos are loaded)
	addAllQueueBtn := widget.NewButton("Add All to Queue", func() {
		state.persistConvertConfig()
		state.executeAddAllToQueue()
	})
	addAllQueueBtn.Importance = widget.MediumImportance
	if len(state.loadedVideos) <= 1 {
		addAllQueueBtn.Hide()
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
	viewLogBtn.Importance = widget.MediumImportance
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
	loadCfgBtn.Importance = widget.MediumImportance

	saveCfgBtn := widget.NewButton("Save Config", func() {
		if err := savePersistedConvertConfig(state.convert); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save config: %w", err), state.window)
			return
		}
		dialog.ShowInformation("Config Saved", fmt.Sprintf("Saved to %s", defaultConvertConfigPath()), state.window)
	})
	saveCfgBtn.Importance = widget.MediumImportance

	// FFmpeg Command Preview
	var commandPreviewWidget *ui.FFmpegCommandWidget
	var commandPreviewRow *fyne.Container

	buildCommandPreview = func() {
		if src == nil || !state.convertCommandPreviewShow {
			if commandPreviewRow != nil {
				commandPreviewRow.Hide()
			}
			return
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
			"sourceWidth":            src.Width,
			"sourceHeight":           src.Height,
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
			commandLabel := widget.NewLabel("FFmpeg Command Preview:")
			commandLabel.TextStyle = fyne.TextStyle{Bold: true}
			commandPreviewRow = container.NewVBox(
				widget.NewSeparator(),
				commandLabel,
				commandPreviewWidget,
			)
		} else {
			commandPreviewWidget.SetCommand(cmdStr)
		}
		if commandPreviewRow != nil {
			commandPreviewRow.Show()
		}
	}

	// Build initial preview if source is loaded
	buildCommandPreview()

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
		snippetConfigRow,
		widget.NewSeparator(),
	}
	if commandPreviewRow != nil && state.convertCommandPreviewShow {
		footerSections = append(footerSections, commandPreviewRow)
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
	return container.NewMax(rect, container.NewPadded(box))
}

func buildMetadataPanel(state *appState, src *videoSource, min fyne.Size) (fyne.CanvasObject, func()) {
	outer := canvas.NewRectangle(utils.MustHex("#191F35"))
	outer.CornerRadius = 8
	outer.StrokeColor = gridColor
	outer.StrokeWidth = 1
	// Don't set rigid MinSize - let the container be flexible for better splitter movement
	// outer.SetMinSize(min)

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

	// Helper function to create compact key-value rows
	makeRow := func(key, value string) fyne.CanvasObject {
		keyLabel := widget.NewLabel(key + ":")
		keyLabel.TextStyle = fyne.TextStyle{Bold: true}
		valueLabel := widget.NewLabel(value)
		// Don't wrap metadata values - they're short and wrapping causes vertical text
		return container.NewHBox(keyLabel, valueLabel)
	}

	// Filename gets its own full-width VBox layout to prevent vertical text
	fileKeyLabel := widget.NewLabel("File:")
	fileKeyLabel.TextStyle = fyne.TextStyle{Bold: true}
	fileValueLabel := widget.NewLabel(src.DisplayName)
	fileValueLabel.Wrapping = fyne.TextWrapWord
	fileRow := container.NewVBox(fileKeyLabel, fileValueLabel)

	// Organize metadata into a compact two-column grid
	col1 := container.NewVBox(
		makeRow("Format", utils.FirstNonEmpty(src.Format, "Unknown")),
		makeRow("Resolution", fmt.Sprintf("%dx%d", src.Width, src.Height)),
		makeRow("Aspect Ratio", src.AspectRatioString()),
		makeRow("Duration", src.DurationString()),
		makeRow("Frame Rate", fmt.Sprintf("%.2f fps", src.FrameRate)),
		makeRow("Interlacing", interlacing),
		makeRow("Color Space", colorSpace),
		makeRow("Color Range", colorRange),
		makeRow("GOP Size", gopSize),
	)

	col2 := container.NewVBox(
		makeRow("Video Codec", utils.FirstNonEmpty(src.VideoCodec, "Unknown")),
		makeRow("Video Bitrate", bitrate),
		makeRow("Pixel Format", utils.FirstNonEmpty(src.PixelFormat, "Unknown")),
		makeRow("Pixel AR", par),
		makeRow("Audio Codec", utils.FirstNonEmpty(src.AudioCodec, "Unknown")),
		makeRow("Audio Bitrate", audioBitrate),
		makeRow("Audio Rate", fmt.Sprintf("%d Hz", src.AudioRate)),
		makeRow("Channels", utils.ChannelLabel(src.Channels)),
		makeRow("Chapters", chapters),
		makeRow("Metadata", metadata),
	)

	// Two-column grid with proper spacing
	twoColGrid := container.NewGridWithColumns(2, col1, col2)

	// Combine filename row with two-column grid
	info := container.NewVBox(fileRow, twoColGrid)

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

	// Cover art support removed - users can add cover art through metadata editor
	updateCoverDisplay := func() {
		// No-op: cover art display removed from this panel
	}

	// Interlacing Analysis Section
	analyzeBtn := widget.NewButton("Analyze Interlacing", func() {
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
			widget.NewButton("Generate Deinterlace Preview", func() {
				if state.source == nil {
					return
				}

				go func() {
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						dialog.ShowInformation("Generating Preview", "Creating comparison preview...", state.window)
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
	if state != nil && state.window != nil {
		winSize := state.window.Canvas().Size()
		if winSize.Height >= 900 {
			desiredHeight := float32(360)
			desiredWidth := desiredHeight / float32(defaultAspect)
			maxWidth := winSize.Width - 48
			maxHeight := winSize.Height - 200
			if maxWidth > 0 && desiredWidth > maxWidth {
				desiredWidth = maxWidth
				desiredHeight = desiredWidth * float32(defaultAspect)
			}
			if maxHeight > 0 && desiredHeight > maxHeight {
				desiredHeight = maxHeight
				desiredWidth = desiredHeight / float32(defaultAspect)
			}
			if desiredWidth > 0 && desiredHeight > 0 {
				targetWidth = desiredWidth
				targetHeight = desiredHeight
			}
		}
	}
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

	usePlayer := state.active != "thumb"

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
		// Frame counter label
		frameLabel := widget.NewLabel("Frame: 0")
		frameLabel.TextStyle = fyne.TextStyle{Monospace: true}
		updateFrame := func(frameNum int) {
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				frameLabel.SetText(fmt.Sprintf("Frame: %d", frameNum))
			}, false)
		}

		var volIcon *widget.Button
		var updatingVolume bool
		ensureSession := func() bool {
			if state.playSess == nil {
				state.playSess = newPlaySession(src.Path, src.Width, src.Height, src.FrameRate, src.Duration, int(targetWidth-28), int(targetHeight-40), updateProgress, updateFrame, img)
				state.playSess.SetVolume(state.playerVolume)
				state.playerPaused = true
			}
			return state.playSess != nil
		}

		// Immediate seeking for responsive playback
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

		// Frame stepping buttons
		prevFrameBtn := utils.MakeIconButton("◀|", "Previous frame (Left Arrow)", func() {
			if !ensureSession() {
				return
			}
			state.playSess.StepFrame(-1)
		})
		nextFrameBtn := utils.MakeIconButton("|▶", "Next frame (Right Arrow)", func() {
			if !ensureSession() {
				return
			}
			state.playSess.StepFrame(1)
		})

		fullBtn := utils.MakeIconButton("⛶", "Toggle fullscreen", func() {
			// Placeholder: embed fullscreen toggle into playback surface later.
		})
		volBox := container.NewHBox(volIcon, container.NewMax(volSlider))
		progress := container.NewBorder(nil, nil, currentTime, totalTime, container.NewMax(slider))
		controls = container.NewVBox(
			container.NewHBox(prevFrameBtn, playBtn, nextFrameBtn, fullBtn, coverBtn, saveFrameBtn, importBtn, layout.NewSpacer(), frameLabel, volBox),
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
	if usePlayer {
		state.setPlayerSurface(videoStage, int(targetWidth-12), int(targetHeight-12))
	}

	stack := container.NewVBox(
		container.NewPadded(videoWithOverlay),
	)
	return container.NewMax(outer, container.NewPadded(stack))
}

type playSession struct {
	path        string
	fps         float64
	width       int
	height      int
	targetW     int
	targetH     int
	volume      float64
	muted       bool
	paused      bool
	current     float64
	stop        chan struct{}
	done        chan struct{}
	prog        func(float64)
	frameFunc   func(int) // Callback for frame number updates
	img         *canvas.Image
	mu          sync.Mutex
	videoCmd    *exec.Cmd
	audioCmd    *exec.Cmd
	frameN      int
	duration    float64 // Total duration in seconds
	startTime   time.Time
	audioTime   atomic.Value // float64 - Audio master clock time
	videoTime   float64      // Last video frame time
	syncOffset  float64      // A/V sync offset for adjustment
	audioActive atomic.Bool  // Whether audio stream is running
}

var audioCtxGlobal struct {
	once sync.Once
	ctx  *oto.Context
	err  error
}

func getAudioContext(sampleRate, channels, bytesPerSample int) (*oto.Context, error) {
	audioCtxGlobal.once.Do(func() {
		_ = bytesPerSample
		// Larger buffer prevents audio stuttering and underruns
		ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
			SampleRate:   sampleRate,
			ChannelCount: channels,
			Format:       oto.FormatSignedInt16LE,
			BufferSize:   170 * time.Millisecond,
		})
		if err == nil && ready != nil {
			<-ready
		}
		audioCtxGlobal.ctx = ctx
		audioCtxGlobal.err = err
	})
	return audioCtxGlobal.ctx, audioCtxGlobal.err
}

func newPlaySession(path string, w, h int, fps, duration float64, targetW, targetH int, prog func(float64), frameFunc func(int), img *canvas.Image) *playSession {
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
		path:      path,
		fps:       fps,
		width:     w,
		height:    h,
		targetW:   targetW,
		targetH:   targetH,
		volume:    100,
		duration:  duration,
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
		prog:      prog,
		frameFunc: frameFunc,
		img:       img,
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

// StepFrame moves forward or backward by a specific number of frames.
// Positive delta moves forward, negative moves backward.
func (p *playSession) StepFrame(delta int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fps <= 0 {
		return
	}

	// Calculate current frame from time position (not from p.frameN which resets on seek)
	currentFrame := int(p.current * p.fps)
	targetFrame := currentFrame + delta

	// Clamp to valid range
	if targetFrame < 0 {
		targetFrame = 0
	}
	maxFrame := int(p.duration * p.fps)
	if targetFrame > maxFrame {
		targetFrame = maxFrame
	}

	// Convert to time offset
	offset := float64(targetFrame) / p.fps
	if offset < 0 {
		offset = 0
	}
	if offset > p.duration {
		offset = p.duration
	}

	// Auto-pause when frame stepping
	p.paused = true
	p.current = offset
	p.stopLocked()
	p.startLocked(p.current)
	p.paused = true

	// Ensure pause is maintained
	time.AfterFunc(30*time.Millisecond, func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.paused = true
	})

	if p.prog != nil {
		p.prog(p.current)
	}
	if p.frameFunc != nil {
		p.frameFunc(targetFrame)
	}
}

// GetCurrentFrame returns the current frame number
func (p *playSession) GetCurrentFrame() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.frameN
}

func (p *playSession) SetVolume(v float64) {
	p.mu.Lock()
	oldVolume := p.volume
	oldMuted := p.muted
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
	p.mu.Unlock()

	// If volume changed significantly, restart audio with new volume filter
	// This is necessary because volume is now handled by FFmpeg
	if math.Abs(oldVolume-v) > 5 || oldMuted != (v <= 0) {
		p.mu.Lock()
		if p.audioCmd != nil {
			// Restart audio with new volume
			currentPos := p.current
			p.mu.Unlock()
			// Stop and restart audio (video keeps playing)
			p.restartAudio(currentPos)
		} else {
			p.mu.Unlock()
		}
	}
}

func (p *playSession) restartAudio(offset float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Kill existing audio
	if p.audioCmd != nil && p.audioCmd.Process != nil {
		_ = p.audioCmd.Process.Kill()
		_ = p.audioCmd.Wait()
	}
	p.audioCmd = nil
	// Start new audio with current volume
	p.runAudio(offset)
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
	p.startTime = time.Now()
	p.audioTime.Store(offset)
	p.videoTime = offset
	p.syncOffset = 0
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
	cmd := utils.CreateCommandRaw(utils.GetFFmpegPath(), args...)
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

			// Sync video to audio master clock (if audio is active)
			videoFrameTime := offset + (float64(p.frameN) / p.fps)
			p.videoTime = videoFrameTime

			if p.audioActive.Load() {
				// Audio is active - sync video to audio master clock
				var audioClockTime float64
				if audioTimeVal := p.audioTime.Load(); audioTimeVal != nil {
					audioClockTime = audioTimeVal.(float64)
				} else {
					audioClockTime = videoFrameTime
				}

				// Calculate A/V sync difference
				avDiff := videoFrameTime - audioClockTime

				// Adaptive timing based on A/V sync
				if avDiff < -frameDur.Seconds()*3 {
					// Video is way ahead of audio (>3 frames) - wait longer
					time.Sleep(frameDur * 2)
					if p.frameN%30 == 0 {
						logging.Debug(logging.CatFFMPEG, "A/V sync: video ahead %.0fms, slowing down", -avDiff*1000)
					}
				} else if avDiff > frameDur.Seconds()*3 {
					// Video is way behind audio (>3 frames) - drop frame
					if p.frameN%30 == 0 {
						logging.Debug(logging.CatFFMPEG, "A/V sync: video behind %.0fms, dropping frame", avDiff*1000)
					}
					p.frameN++
					p.current = offset + (float64(p.frameN) / p.fps)
					continue
				} else if avDiff > frameDur.Seconds() {
					// Video slightly behind - speed up (skip sleep)
					if p.frameN%60 == 0 {
						logging.Debug(logging.CatFFMPEG, "A/V sync: video slightly behind %.0fms, catching up", avDiff*1000)
					}
				} else if avDiff < -frameDur.Seconds() {
					// Video slightly ahead - slow down
					if p.frameN%60 == 0 {
						logging.Debug(logging.CatFFMPEG, "A/V sync: video slightly ahead %.0fms, waiting", -avDiff*1000)
					}
					time.Sleep(frameDur + time.Duration(math.Abs(avDiff)*float64(time.Second)))
				} else {
					// In sync - normal timing
					if p.frameN%180 == 0 && p.frameN > 0 {
						logging.Debug(logging.CatFFMPEG, "A/V sync: good sync (diff %.1fms)", avDiff*1000)
					}
					time.Sleep(frameDur)
				}
			} else {
				// No audio - just pace video at its natural frame rate
				time.Sleep(frameDur)
			}
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
			if p.frameFunc != nil {
				p.frameFunc(p.frameN)
			}
		}
	}()
}

func (p *playSession) runAudio(offset float64) {
	const sampleRate = 48000
	const channels = 2
	const bytesPerSample = 2
	var stderr bytes.Buffer

	// Build FFmpeg arguments with volume control moved to FFmpeg
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-ss", fmt.Sprintf("%.3f", offset),
		"-i", p.path,
		"-vn",
		"-ac", fmt.Sprintf("%d", channels),
		"-ar", fmt.Sprintf("%d", sampleRate),
	}

	// Add volume filter to FFmpeg instead of processing in Go (much faster)
	p.mu.Lock()
	volume := p.volume
	muted := p.muted
	p.mu.Unlock()

	if muted || volume <= 0 {
		args = append(args, "-af", "volume=0")
	} else if math.Abs(volume-100) > 0.1 {
		args = append(args, "-af", fmt.Sprintf("volume=%.2f", volume/100.0))
	}

	args = append(args, "-f", "s16le", "-")

	cmd := utils.CreateCommandRaw(utils.GetFFmpegPath(), args...)
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logging.Debug(logging.CatFFMPEG, "audio pipe error: %v", err)
		return
	}
	if err := cmd.Start(); err != nil {
		logging.Debug(logging.CatFFMPEG, "audio start failed (video-only playback): %v (%s)", err, strings.TrimSpace(stderr.String()))
		p.audioActive.Store(false)
		return
	}
	p.audioCmd = cmd
	ctx, err := getAudioContext(sampleRate, channels, bytesPerSample)
	if err != nil {
		logging.Debug(logging.CatFFMPEG, "audio context error (video-only playback): %v", err)
		p.audioActive.Store(false)
		return
	}
	pr, pw := io.Pipe()
	player := ctx.NewPlayer(pr)
	if player == nil {
		logging.Debug(logging.CatFFMPEG, "audio player creation failed (video-only playback)")
		p.audioActive.Store(false)
		return
	}
	player.Play()
	p.audioActive.Store(true) // Mark audio as active
	localPlayer := player
	go func() {
		defer cmd.Process.Kill()
		defer localPlayer.Close()
		defer pr.Close()
		defer pw.Close()
		defer p.audioActive.Store(false) // Mark audio as inactive when done
		// Increased from 4096 (21ms) to 16384 (85ms) for smoother playback
		// Larger chunks reduce read frequency and improve performance
		chunk := make([]byte, 16384)
		loggedFirst := false
		bytesWritten := int64(0) // Track total audio bytes for master clock
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
				// Volume is now handled by FFmpeg, just write directly
				// This eliminates per-sample processing overhead
				if _, werr := pw.Write(chunk[:n]); werr != nil {
					logging.Debug(logging.CatFFMPEG, "audio write failed: %v", werr)
					return
				}

				// Update audio master clock for A/V sync
				bytesWritten += int64(n)
				// Calculate elapsed audio time: bytes / (sampleRate * channels * bytesPerSample)
				elapsedTime := float64(bytesWritten) / float64(sampleRate*channels*bytesPerSample)
				currentAudioTime := offset + elapsedTime
				p.audioTime.Store(currentAudioTime)
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
		dest := filepath.Join(utils.TempDir(), fmt.Sprintf("videotools-cover-%d.png", time.Now().UnixNano()))
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

	// If in author module, add video clips
	if s.active == "author" {
		var videoPaths []string
		var videoTSPath string
		for _, uri := range items {
			if uri.Scheme() != "file" {
				continue
			}
			path := uri.Path()
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				if strings.EqualFold(filepath.Base(path), "VIDEO_TS") {
					videoTSPath = path
					break
				}
				videoTSChild := filepath.Join(path, "VIDEO_TS")
				if info, err := os.Stat(videoTSChild); err == nil && info.IsDir() {
					videoTSPath = videoTSChild
					break
				}
				videos := s.findVideoFiles(path)
				videoPaths = append(videoPaths, videos...)
			} else if s.isVideoFile(path) {
				videoPaths = append(videoPaths, path)
			}
		}

		if videoTSPath != "" {
			s.authorVideoTSPath = videoTSPath
			s.authorClips = nil
			s.authorFile = nil
			s.authorOutputType = "iso"
			s.loadVideoTSChapters(videoTSPath)
			s.showAuthorView()
			return
		}

		if len(videoPaths) == 0 {
			logging.Debug(logging.CatUI, "no valid video files in dropped items")
			return
		}

		s.addAuthorFiles(videoPaths)
		s.showAuthorView()
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

		go s.loadMultipleThumbVideos(videoPaths)

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

	// If in player module, handle single video file
	if s.active == "player" {
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
			dialog.ShowInformation("VT_Player", "No video files found in dropped items.", s.window)
			return
		}

		// Load first video
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
		cmd := utils.CreateCommand(ctx, utils.GetFFmpegPath(), args...)
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

	// Letterbox/Pillarbox: pad with black bars (auto-detects direction based on aspect ratio change)
	// Also handles legacy "Letterbox" and "Pillarbox" options for backwards compatibility
	if strings.EqualFold(mode, "Letterbox/Pillarbox") || strings.EqualFold(mode, "Letterbox") || strings.EqualFold(mode, "Pillarbox") {
		pad := fmt.Sprintf("pad=w='trunc(max(iw,ih*%[1]s)/2)*2':h='trunc(max(ih,iw/%[1]s)/2)*2':x='(ow-iw)/2':y='(oh-ih)/2':color=black", ar)
		return []string{pad, "setsar=1"}
	}

	// Default fallback: same as Letterbox/Pillarbox
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
	cmd := utils.CreateCommandRaw(utils.GetFFmpegPath(),
		"-y",
		"-ss", start,
		"-i", path,
		"-t", "3",
		"-vf", "scale=640:-1:flags=lanczos,fps=8",
		pattern,
	)
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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var fileSize int64
	if info, err := os.Stat(path); err == nil {
		fileSize = info.Size()
	}

	cmd := utils.CreateCommand(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-show_chapters",
		path,
	)
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
			Index        int                    `json:"index"`
			CodecType    string                 `json:"codec_type"`
			CodecName    string                 `json:"codec_name"`
			Width        int                    `json:"width"`
			Height       int                    `json:"height"`
			Duration     string                 `json:"duration"`
			BitRate      string                 `json:"bit_rate"`
			PixFmt       string                 `json:"pix_fmt"`
			SampleRate   string                 `json:"sample_rate"`
			Channels     int                    `json:"channels"`
			AvgFrameRate string                 `json:"avg_frame_rate"`
			FieldOrder   string                 `json:"field_order"`
			Tags         map[string]interface{} `json:"tags"`
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
				}
				if stream.PixFmt != "" {
					src.PixelFormat = stream.PixFmt
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

	// Determine video pane size based on screen resolution
	screenSize := fyne.CurrentApp().Driver().AllWindows()[0].Canvas().Size()
	var playerSize fyne.Size
	if screenSize.Width < 1600 {
		// Use smaller size for lower resolution displays
		playerSize = fyne.NewSize(640, 360)
	} else {
		// Use larger size for higher resolution displays
		playerSize = fyne.NewSize(1280, 720)
	}

	var videoContainer fyne.CanvasObject
	if state.playerFile != nil {
		fileLabel.SetText(fmt.Sprintf("File: %s", filepath.Base(state.playerFile.Path)))
		videoContainer = buildVideoPane(state, playerSize, state.playerFile, nil)
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

func buildEnhancementView(state *appState) fyne.CanvasObject {
	// TODO: Define enhancement color when needed

	// TODO: Implement enhancement view with AI model selection
	// For now, show placeholder
	content := container.NewVBox(
		widget.NewLabel("🚀 Video Enhancement"),
		widget.NewSeparator(),
		widget.NewLabel("AI-powered video enhancement is coming soon!"),
		widget.NewLabel("Features planned:"),
		widget.NewLabel("• Real-ESRGAN Super-Resolution"),
		widget.NewLabel("• BasicVSR Video Enhancement"),
		widget.NewLabel("• Content-Aware Processing"),
		widget.NewLabel("• Real-time Preview"),
		widget.NewSeparator(),
		widget.NewLabel("This will use the unified FFmpeg player foundation"),
		widget.NewLabel("for frame-accurate enhancement processing."),
	)

	outer := canvas.NewRectangle(utils.MustHex("#191F35"))
	outer.CornerRadius = 8
	outer.StrokeColor = gridColor
	outer.StrokeWidth = 1

	container := container.NewBorder(
		widget.NewLabelWithStyle("Enhancement", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil, nil, nil,
		content,
	)

	// Remove color variable as it's not used
	return container
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
		state.upscaleAIModel = "realesrgan-x4plus" // General purpose AI model
	}
	if state.upscaleFrameRate == "" {
		state.upscaleFrameRate = "Source"
	}
	if state.upscaleQualityPreset == "" {
		state.upscaleQualityPreset = "Near-lossless (CRF 16)"
	}
	if state.upscaleAIPreset == "" {
		state.upscaleAIPreset = "Balanced"
		state.upscaleAIScale = 4.0
		state.upscaleAIScaleUseTarget = true
		state.upscaleAIOutputAdjust = 1.0
		state.upscaleAIDenoise = 0.5
		state.upscaleAITile = 512
		state.upscaleAIOutputFormat = "png"
		state.upscaleAIGPUAuto = true
		state.upscaleAIThreadsLoad = 1
		state.upscaleAIThreadsProc = 2
		state.upscaleAIThreadsSave = 2
	}

	// Check AI availability on first load
	if state.upscaleAIBackend == "" {
		state.upscaleAIBackend = detectAIUpscaleBackend()
		state.upscaleAIAvailable = state.upscaleAIBackend == "ncnn"
	}
	if len(state.filterActiveChain) > 0 {
		state.upscaleFilterChain = append([]string{}, state.filterActiveChain...)
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
		videoContainer = buildVideoPane(state, fyne.NewSize(480, 270), state.upscaleFile, nil)
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

	qualitySelect := widget.NewSelect([]string{
		"Lossless (CRF 0)",
		"Near-lossless (CRF 16)",
		"High (CRF 18)",
	}, func(s string) {
		state.upscaleQualityPreset = s
	})
	qualitySelect.SetSelected(state.upscaleQualityPreset)

	qualitySection := widget.NewCard("Output Quality", "", container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel("Quality:"),
			qualitySelect,
		),
		widget.NewLabel("Lower CRF = higher quality/larger files"),
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

	aiModelOptions := aiUpscaleModelOptions()
	aiModelLabel := aiUpscaleModelLabel(state.upscaleAIModel)
	if aiModelLabel == "" && len(aiModelOptions) > 0 {
		aiModelLabel = aiModelOptions[0]
	}

	// AI Upscaling Section
	var aiSection *widget.Card
	if state.upscaleAIAvailable {
		var aiTileSelect *widget.Select
		var aiTTACheck *widget.Check
		var aiDenoiseSlider *widget.Slider
		var denoiseHint *widget.Label

		applyAIPreset := func(preset string) {
			state.upscaleAIPreset = preset
			switch preset {
			case "Ultra Fast":
				state.upscaleAITile = 800
				state.upscaleAITTA = false
			case "Fast":
				state.upscaleAITile = 800
				state.upscaleAITTA = false
			case "Balanced":
				state.upscaleAITile = 512
				state.upscaleAITTA = false
			case "High Quality":
				state.upscaleAITile = 256
				state.upscaleAITTA = false
			case "Maximum Quality":
				state.upscaleAITile = 0
				state.upscaleAITTA = true
			}
			if aiTileSelect != nil {
				switch state.upscaleAITile {
				case 256:
					aiTileSelect.SetSelected("256")
				case 512:
					aiTileSelect.SetSelected("512")
				case 800:
					aiTileSelect.SetSelected("800")
				default:
					aiTileSelect.SetSelected("Auto")
				}
			}
			if aiTTACheck != nil {
				aiTTACheck.SetChecked(state.upscaleAITTA)
			}
		}

		updateDenoiseAvailability := func(model string) {
			if aiDenoiseSlider == nil || denoiseHint == nil {
				return
			}
			if model == "realesr-general-x4v3" {
				aiDenoiseSlider.Enable()
				denoiseHint.SetText("Denoise available on General Tiny model")
			} else {
				aiDenoiseSlider.Disable()
				denoiseHint.SetText("Denoise only supported on General Tiny model")
			}
		}

		aiEnabledCheck := widget.NewCheck("Use AI Upscaling", func(checked bool) {
			state.upscaleAIEnabled = checked
		})
		aiEnabledCheck.SetChecked(state.upscaleAIEnabled)

		aiModelSelect := widget.NewSelect(aiModelOptions, func(s string) {
			state.upscaleAIModel = aiUpscaleModelID(s)
			aiModelLabel = s
			updateDenoiseAvailability(state.upscaleAIModel)
		})
		if aiModelLabel != "" {
			aiModelSelect.SetSelected(aiModelLabel)
		}

		aiPresetSelect := widget.NewSelect([]string{"Ultra Fast", "Fast", "Balanced", "High Quality", "Maximum Quality"}, func(s string) {
			applyAIPreset(s)
		})
		aiPresetSelect.SetSelected(state.upscaleAIPreset)

		aiScaleSelect := widget.NewSelect([]string{"Match Target", "1x", "2x", "3x", "4x", "8x"}, func(s string) {
			if s == "Match Target" {
				state.upscaleAIScaleUseTarget = true
				return
			}
			state.upscaleAIScaleUseTarget = false
			switch s {
			case "1x":
				state.upscaleAIScale = 1
			case "2x":
				state.upscaleAIScale = 2
			case "3x":
				state.upscaleAIScale = 3
			case "4x":
				state.upscaleAIScale = 4
			case "8x":
				state.upscaleAIScale = 8
			}
		})
		if state.upscaleAIScaleUseTarget {
			aiScaleSelect.SetSelected("Match Target")
		} else {
			aiScaleSelect.SetSelected(fmt.Sprintf("%.0fx", state.upscaleAIScale))
		}

		aiAdjustLabel := widget.NewLabel(fmt.Sprintf("Adjustment: %.2fx", state.upscaleAIOutputAdjust))
		aiAdjustSlider := widget.NewSlider(0.5, 2.0)
		aiAdjustSlider.Value = state.upscaleAIOutputAdjust
		aiAdjustSlider.Step = 0.05
		aiAdjustSlider.OnChanged = func(v float64) {
			state.upscaleAIOutputAdjust = v
			aiAdjustLabel.SetText(fmt.Sprintf("Adjustment: %.2fx", v))
		}

		aiDenoiseLabel := widget.NewLabel(fmt.Sprintf("Denoise: %.2f", state.upscaleAIDenoise))
		aiDenoiseSlider = widget.NewSlider(0.0, 1.0)
		aiDenoiseSlider.Value = state.upscaleAIDenoise
		aiDenoiseSlider.Step = 0.05
		aiDenoiseSlider.OnChanged = func(v float64) {
			state.upscaleAIDenoise = v
			aiDenoiseLabel.SetText(fmt.Sprintf("Denoise: %.2f", v))
		}

		aiTileSelect = widget.NewSelect([]string{"Auto", "256", "512", "800"}, func(s string) {
			switch s {
			case "Auto":
				state.upscaleAITile = 0
			case "256":
				state.upscaleAITile = 256
			case "512":
				state.upscaleAITile = 512
			case "800":
				state.upscaleAITile = 800
			}
		})
		switch state.upscaleAITile {
		case 256:
			aiTileSelect.SetSelected("256")
		case 512:
			aiTileSelect.SetSelected("512")
		case 800:
			aiTileSelect.SetSelected("800")
		default:
			aiTileSelect.SetSelected("Auto")
		}

		aiOutputFormatSelect := widget.NewSelect([]string{"PNG", "JPG", "WEBP"}, func(s string) {
			state.upscaleAIOutputFormat = strings.ToLower(s)
		})
		switch strings.ToLower(state.upscaleAIOutputFormat) {
		case "jpg", "jpeg":
			aiOutputFormatSelect.SetSelected("JPG")
		case "webp":
			aiOutputFormatSelect.SetSelected("WEBP")
		default:
			aiOutputFormatSelect.SetSelected("PNG")
		}

		aiFaceCheck := widget.NewCheck("Face Enhancement (requires Python/GFPGAN)", func(checked bool) {
			state.upscaleAIFaceEnhance = checked
		})
		aiFaceAvailable := checkAIFaceEnhanceAvailable(state.upscaleAIBackend)
		if !aiFaceAvailable {
			aiFaceCheck.Disable()
		}
		aiFaceCheck.SetChecked(state.upscaleAIFaceEnhance && aiFaceAvailable)

		aiTTACheck = widget.NewCheck("Enable TTA (slower, higher quality)", func(checked bool) {
			state.upscaleAITTA = checked
		})
		aiTTACheck.SetChecked(state.upscaleAITTA)

		aiGPUSelect := widget.NewSelect([]string{"Auto", "0", "1", "2"}, func(s string) {
			if s == "Auto" {
				state.upscaleAIGPUAuto = true
				return
			}
			state.upscaleAIGPUAuto = false
			if gpu, err := strconv.Atoi(s); err == nil {
				state.upscaleAIGPU = gpu
			}
		})
		if state.upscaleAIGPUAuto {
			aiGPUSelect.SetSelected("Auto")
		} else {
			aiGPUSelect.SetSelected(strconv.Itoa(state.upscaleAIGPU))
		}

		threadOptions := []string{"1", "2", "3", "4"}
		aiThreadsLoad := widget.NewSelect(threadOptions, func(s string) {
			if v, err := strconv.Atoi(s); err == nil {
				state.upscaleAIThreadsLoad = v
			}
		})
		aiThreadsLoad.SetSelected(strconv.Itoa(state.upscaleAIThreadsLoad))

		aiThreadsProc := widget.NewSelect(threadOptions, func(s string) {
			if v, err := strconv.Atoi(s); err == nil {
				state.upscaleAIThreadsProc = v
			}
		})
		aiThreadsProc.SetSelected(strconv.Itoa(state.upscaleAIThreadsProc))

		aiThreadsSave := widget.NewSelect(threadOptions, func(s string) {
			if v, err := strconv.Atoi(s); err == nil {
				state.upscaleAIThreadsSave = v
			}
		})
		aiThreadsSave.SetSelected(strconv.Itoa(state.upscaleAIThreadsSave))

		denoiseHint = widget.NewLabel("")
		denoiseHint.TextStyle = fyne.TextStyle{Italic: true}
		updateDenoiseAvailability(state.upscaleAIModel)

		aiSection = widget.NewCard("AI Upscaling", "✓ Available", container.NewVBox(
			widget.NewLabel("Real-ESRGAN detected - enhanced quality available"),
			aiEnabledCheck,
			container.NewGridWithColumns(2,
				widget.NewLabel("AI Model:"),
				aiModelSelect,
			),
			container.NewGridWithColumns(2,
				widget.NewLabel("Processing Preset:"),
				aiPresetSelect,
			),
			container.NewGridWithColumns(2,
				widget.NewLabel("Upscale Factor:"),
				aiScaleSelect,
			),
			container.NewVBox(aiAdjustLabel, aiAdjustSlider),
			container.NewVBox(aiDenoiseLabel, aiDenoiseSlider, denoiseHint),
			container.NewGridWithColumns(2,
				widget.NewLabel("Tile Size:"),
				aiTileSelect,
			),
			container.NewGridWithColumns(2,
				widget.NewLabel("Output Frames:"),
				aiOutputFormatSelect,
			),
			aiFaceCheck,
			aiTTACheck,
			widget.NewSeparator(),
			widget.NewLabel("Advanced (ncnn backend)"),
			container.NewGridWithColumns(2,
				widget.NewLabel("GPU:"),
				aiGPUSelect,
			),
			container.NewGridWithColumns(2,
				widget.NewLabel("Threads (Load/Proc/Save):"),
				container.NewGridWithColumns(3, aiThreadsLoad, aiThreadsProc, aiThreadsSave),
			),
			widget.NewLabel("Note: AI upscaling is slower but produces higher quality results"),
		))
	} else {
		backendNote := "Real-ESRGAN not detected. Install for enhanced quality:"
		if state.upscaleAIBackend == "python" {
			backendNote = "Python Real-ESRGAN detected, but the ncnn backend is required for now."
		}
		aiSection = widget.NewCard("AI Upscaling", "Not Available", container.NewVBox(
			widget.NewLabel(backendNote),
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
				"targetPreset":           state.upscaleTargetRes,
				"sourceWidth":            float64(state.upscaleFile.Width),
				"sourceHeight":           float64(state.upscaleFile.Height),
				"preserveAR":             preserveAspect,
				"useAI":                  state.upscaleAIEnabled && state.upscaleAIAvailable,
				"aiModel":                state.upscaleAIModel,
				"qualityPreset":          state.upscaleQualityPreset,
				"aiBackend":              state.upscaleAIBackend,
				"aiPreset":               state.upscaleAIPreset,
				"aiScale":                state.upscaleAIScale,
				"aiScaleUseTarget":       state.upscaleAIScaleUseTarget,
				"aiOutputAdjust":         state.upscaleAIOutputAdjust,
				"aiFaceEnhance":          state.upscaleAIFaceEnhance,
				"aiDenoise":              state.upscaleAIDenoise,
				"aiTile":                 float64(state.upscaleAITile),
				"aiGPU":                  float64(state.upscaleAIGPU),
				"aiGPUAuto":              state.upscaleAIGPUAuto,
				"aiThreadsLoad":          float64(state.upscaleAIThreadsLoad),
				"aiThreadsProc":          float64(state.upscaleAIThreadsProc),
				"aiThreadsSave":          float64(state.upscaleAIThreadsSave),
				"aiTTA":                  state.upscaleAITTA,
				"aiOutputFormat":         state.upscaleAIOutputFormat,
				"applyFilters":           state.upscaleApplyFilters,
				"filterChain":            state.upscaleFilterChain,
				"duration":               state.upscaleFile.Duration,
				"sourceFrameRate":        state.upscaleFile.FrameRate,
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
		qualitySection,
		frameRateSection,
		aiSection,
		filterIntegrationSection,
		container.NewGridWithColumns(2, applyBtn, addQueueBtn),
	)

	settingsScroll := container.NewVScroll(settingsPanel)
	// Adaptive height for small screens
	settingsScroll.SetMinSize(fyne.NewSize(400, 400))

	mainContent := container.New(&fixedHSplitLayout{ratio: 0.6},
		container.NewVBox(leftPanel, videoContainer),
		settingsScroll,
	)

	content := container.NewPadded(mainContent)

	return container.NewBorder(topBar, bottomBar, nil, nil, content)
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
