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
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/convert"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/modules"
	"git.leaktechnologies.dev/stu/VideoTools/internal/player"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
	"github.com/hajimehoshi/oto"
)

// Module describes a high level tool surface that gets a tile on the menu.
type Module struct {
	ID     string
	Label  string
	Color  color.Color
	Handle func(files []string)
}

var (
	debugFlag = flag.Bool("debug", false, "enable verbose logging (env: VIDEOTOOLS_DEBUG=1)")

	backgroundColor = utils.MustHex("#0B0F1A")
	gridColor       = utils.MustHex("#171C2A")
	textColor       = utils.MustHex("#E1EEFF")
	queueColor      = utils.MustHex("#5961FF")

	modulesList = []Module{
		{"convert", "Convert", utils.MustHex("#8B44FF"), modules.HandleConvert}, // Violet
		{"merge", "Merge", utils.MustHex("#4488FF"), modules.HandleMerge},       // Blue
		{"trim", "Trim", utils.MustHex("#44DDFF"), modules.HandleTrim},          // Cyan
		{"filters", "Filters", utils.MustHex("#44FF88"), modules.HandleFilters}, // Green
		{"upscale", "Upscale", utils.MustHex("#AAFF44"), modules.HandleUpscale}, // Yellow-Green
		{"audio", "Audio", utils.MustHex("#FFD744"), modules.HandleAudio},       // Yellow
		{"thumb", "Thumb", utils.MustHex("#FF8844"), modules.HandleThumb},       // Orange
		{"compare", "Compare", utils.MustHex("#FF44AA"), modules.HandleCompare}, // Pink
		{"inspect", "Inspect", utils.MustHex("#FF4444"), modules.HandleInspect}, // Red
	}
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

type formatOption struct {
	Label      string
	Ext        string
	VideoCodec string
}

var formatOptions = []formatOption{
	{"MP4 (H.264)", ".mp4", "libx264"},
	{"MKV (H.265)", ".mkv", "libx265"},
	{"MOV (ProRes)", ".mov", "prores_ks"},
	{"DVD-NTSC (MPEG-2)", ".mpg", "mpeg2video"},
	{"DVD-PAL (MPEG-2)", ".mpg", "mpeg2video"},
}

type convertConfig struct {
	OutputBase     string
	SelectedFormat formatOption
	Quality        string // Preset quality (Draft/Standard/High/Lossless)
	Mode           string // Simple or Advanced

	// Video encoding settings
	VideoCodec       string // H.264, H.265, VP9, AV1, Copy
	EncoderPreset    string // ultrafast, superfast, veryfast, faster, fast, medium, slow, slower, veryslow
	CRF              string // Manual CRF value (0-51, or empty to use Quality preset)
	BitrateMode      string // CRF, CBR, VBR, "Target Size"
	VideoBitrate     string // For CBR/VBR modes (e.g., "5000k")
	TargetFileSize   string // Target file size (e.g., "25MB", "100MB") - requires BitrateMode="Target Size"
	TargetResolution string // Source, 720p, 1080p, 1440p, 4K, or custom
	FrameRate        string // Source, 24, 30, 60, or custom
	PixelFormat      string // yuv420p, yuv422p, yuv444p
	HardwareAccel    string // none, nvenc, vaapi, qsv, videotoolbox
	TwoPass          bool   // Enable two-pass encoding for VBR
	H264Profile        string // baseline, main, high (for H.264 compatibility)
	H264Level          string // 3.0, 3.1, 4.0, 4.1, 5.0, 5.1 (for H.264 compatibility)
	Deinterlace        string // Auto, Force, Off
	DeinterlaceMethod  string // yadif, bwdif (bwdif is higher quality but slower)
	AutoCrop           bool   // Auto-detect and remove black bars
	CropWidth          string // Manual crop width (empty = use auto-detect)
	CropHeight         string // Manual crop height (empty = use auto-detect)
	CropX              string // Manual crop X offset (empty = use auto-detect)
	CropY              string // Manual crop Y offset (empty = use auto-detect)

	// Audio encoding settings
	AudioCodec       string // AAC, Opus, MP3, FLAC, Copy
	AudioBitrate     string // 128k, 192k, 256k, 320k
	AudioChannels    string // Source, Mono, Stereo, 5.1
	AudioSampleRate  string // Source, 44100, 48000
	NormalizeAudio   bool   // Force stereo + 48kHz for compatibility

	// Other settings
	InverseTelecine  bool
	InverseAutoNotes string
	CoverArtPath     string
	AspectHandling   string
	OutputAspect     string
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

type appState struct {
	window           fyne.Window
	active           string
	lastModule       string
	source           *videoSource
	loadedVideos     []*videoSource // Multiple loaded videos for navigation
	currentIndex     int            // Current video index in loadedVideos
	anim             *previewAnimator
	convert          convertConfig
	currentFrame     string
	player           player.Controller
	playerReady      bool
	playerVolume     float64
	playerMuted      bool
	lastVolume       float64
	playerPaused     bool
	playerPos        float64
	playerLast       time.Time
	progressQuit     chan struct{}
	convertCancel    context.CancelFunc
	playerSurf       *playerSurface
	convertBusy      bool
	convertStatus    string
	convertActiveIn  string
	convertActiveOut string
	convertProgress  float64
	playSess         *playSession
	jobQueue         *queue.Queue
	statsBar         *ui.ConversionStatsBar
	queueBtn         *widget.Button
	queueScroll      *container.Scroll
	queueOffset      fyne.Position
	compareFile1     *videoSource
	compareFile2     *videoSource
}

func (s *appState) stopPreview() {
	if s.anim != nil {
		s.anim.Stop()
		s.anim = nil
	}
}

func (s *appState) updateStatsBar() {
	if s.statsBar == nil || s.jobQueue == nil {
		return
	}

	pending, running, completed, failed := s.jobQueue.Stats()

	// Find the currently running job to get its progress
	var progress float64
	var jobTitle string
	if running > 0 {
		jobs := s.jobQueue.List()
		for _, job := range jobs {
			if job.Status == queue.JobStatusRunning {
				progress = job.Progress
				jobTitle = job.Title
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
	}

	s.statsBar.UpdateStats(running, pending, completed, failed, progress, jobTitle)
}

func (s *appState) queueProgressCounts() (completed, total int) {
	if s.jobQueue == nil {
		return 0, 0
	}
	pending, running, completedCount, failed := s.jobQueue.Stats()
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

func (s *appState) setContent(body fyne.CanvasObject) {
	update := func() {
		bg := canvas.NewRectangle(backgroundColor)
		// Don't set a minimum size - let content determine layout naturally
		if body == nil {
			s.window.SetContent(bg)
			return
		}
		s.window.SetContent(container.NewMax(bg, body))
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

	// Convert Module slice to ui.ModuleInfo slice
	var mods []ui.ModuleInfo
	for _, m := range modulesList {
		mods = append(mods, ui.ModuleInfo{
			ID:      m.ID,
			Label:   m.Label,
			Color:   m.Color,
			Enabled: m.ID == "convert" || m.ID == "compare", // Convert and compare modules are functional
		})
	}

	titleColor := utils.MustHex("#4CE870")

	// Get queue stats - show completed jobs out of total
	var queueCompleted, queueTotal int
	if s.jobQueue != nil {
		_, _, completed, _ := s.jobQueue.Stats()
		queueCompleted = completed
		queueTotal = len(s.jobQueue.List())
	}

	menu := ui.BuildMainMenu(mods, s.showModule, s.handleModuleDrop, s.showQueue, titleColor, queueColor, textColor, queueCompleted, queueTotal)

	// Update stats bar
	s.updateStatsBar()

	// Add stats bar at the bottom of the menu
	content := container.NewBorder(
		nil,                       // top
		s.statsBar,                // bottom
		nil,                       // left
		nil,                       // right
		container.NewPadded(menu), // center
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
			s.refreshQueueView() // Refresh
		},
		func() { // onClearAll
			s.jobQueue.ClearAll()
			s.clearVideo()
			s.refreshQueueView() // Refresh
		},
		utils.MustHex("#4CE870"), // titleColor
		gridColor,                // bgColor
		textColor,                // textColor
	)

	// Restore scroll offset
	s.queueScroll = scroll
	if s.queueScroll != nil {
		s.queueScroll.Offset = s.queueOffset
		s.queueScroll.Refresh()
	}

	s.setContent(container.NewPadded(view))
}

// addConvertToQueue adds a conversion job to the queue
func (s *appState) addConvertToQueue() error {
	if s.source == nil {
		return fmt.Errorf("no video loaded")
	}

	src := s.source
	cfg := s.convert

	outDir := filepath.Dir(src.Path)
	outName := cfg.OutputFile()
	if outName == "" {
		outName = "converted" + cfg.SelectedFormat.Ext
	}
	outPath := filepath.Join(outDir, outName)
	if outPath == src.Path {
		outPath = filepath.Join(outDir, "converted-"+outName)
	}

	// Create job config map
	config := map[string]interface{}{
		"inputPath":        src.Path,
		"outputPath":       outPath,
		"outputBase":       cfg.OutputBase,
		"selectedFormat":   cfg.SelectedFormat,
		"quality":          cfg.Quality,
		"mode":             cfg.Mode,
		"videoCodec":       cfg.VideoCodec,
		"encoderPreset":    cfg.EncoderPreset,
		"crf":              cfg.CRF,
		"bitrateMode":      cfg.BitrateMode,
		"videoBitrate":     cfg.VideoBitrate,
		"targetFileSize":   cfg.TargetFileSize,
		"targetResolution": cfg.TargetResolution,
		"frameRate":        cfg.FrameRate,
		"pixelFormat":      cfg.PixelFormat,
		"hardwareAccel":    cfg.HardwareAccel,
		"twoPass":          cfg.TwoPass,
		"h264Profile":        cfg.H264Profile,
		"h264Level":          cfg.H264Level,
		"deinterlace":        cfg.Deinterlace,
		"deinterlaceMethod":  cfg.DeinterlaceMethod,
		"autoCrop":           cfg.AutoCrop,
		"cropWidth":          cfg.CropWidth,
		"cropHeight":         cfg.CropHeight,
		"cropX":              cfg.CropX,
		"cropY":              cfg.CropY,
		"audioCodec":         cfg.AudioCodec,
		"audioBitrate":     cfg.AudioBitrate,
		"audioChannels":    cfg.AudioChannels,
		"audioSampleRate":  cfg.AudioSampleRate,
		"normalizeAudio":   cfg.NormalizeAudio,
		"inverseTelecine":  cfg.InverseTelecine,
		"coverArtPath":     cfg.CoverArtPath,
		"aspectHandling":   cfg.AspectHandling,
		"outputAspect":     cfg.OutputAspect,
		"sourceWidth":      src.Width,
		"sourceHeight":     src.Height,
		"sourceDuration":   src.Duration,
		"fieldOrder":       src.FieldOrder,
	}

	job := &queue.Job{
		Type:        queue.JobTypeConvert,
		Title:       fmt.Sprintf("Convert %s", filepath.Base(src.Path)),
		Description: fmt.Sprintf("Output: %s → %s", filepath.Base(src.Path), filepath.Base(outPath)),
		InputFile:   src.Path,
		OutputFile:  outPath,
		Config:      config,
	}

	s.jobQueue.Add(job)
	logging.Debug(logging.CatSystem, "added convert job to queue: %s", job.ID)

	return nil
}

func (s *appState) showModule(id string) {
	switch id {
	case "convert":
		s.showConvertView(nil)
	case "compare":
		s.showCompareView()
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

	// Single file or non-convert module: load first video and show module
	path := videoPaths[0]
	logging.Debug(logging.CatModule, "drop on module %s path=%s - starting load", moduleID, path)

	go func() {
		logging.Debug(logging.CatModule, "loading video in goroutine")
		s.loadVideo(path)
		// After loading, switch to the module
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
		baseName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		outName := baseName + "-converted" + s.convert.SelectedFormat.Ext
		outPath := filepath.Join(outDir, outName)

		config := map[string]interface{}{
			"inputPath":        path,
			"outputPath":       outPath,
			"outputBase":       baseName + "-converted",
			"selectedFormat":   s.convert.SelectedFormat,
			"quality":          s.convert.Quality,
			"mode":             s.convert.Mode,
			"videoCodec":       s.convert.VideoCodec,
			"encoderPreset":    s.convert.EncoderPreset,
			"crf":              s.convert.CRF,
			"bitrateMode":      s.convert.BitrateMode,
			"videoBitrate":     s.convert.VideoBitrate,
			"targetResolution": s.convert.TargetResolution,
			"frameRate":        s.convert.FrameRate,
			"pixelFormat":      s.convert.PixelFormat,
			"hardwareAccel":    s.convert.HardwareAccel,
			"twoPass":          s.convert.TwoPass,
			"h264Profile":        s.convert.H264Profile,
			"h264Level":          s.convert.H264Level,
			"deinterlace":        s.convert.Deinterlace,
			"deinterlaceMethod":  s.convert.DeinterlaceMethod,
			"audioCodec":         s.convert.AudioCodec,
			"audioBitrate":     s.convert.AudioBitrate,
			"audioChannels":    s.convert.AudioChannels,
			"audioSampleRate":  s.convert.AudioSampleRate,
			"normalizeAudio":   s.convert.NormalizeAudio,
			"inverseTelecine":  s.convert.InverseTelecine,
			"coverArtPath":     "",
			"aspectHandling":   s.convert.AspectHandling,
			"outputAspect":     s.convert.OutputAspect,
			"sourceWidth":      src.Width,
			"sourceHeight":     src.Height,
			"sourceDuration":   src.Duration,
			"fieldOrder":       src.FieldOrder,
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
	s.setContent(buildConvertView(s, s.source))
}

func (s *appState) showCompareView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "compare"
	s.setContent(buildCompareView(s))
}

// jobExecutor executes a job from the queue
func (s *appState) jobExecutor(ctx context.Context, job *queue.Job, progressCallback func(float64)) error {
	logging.Debug(logging.CatSystem, "executing job %s: %s", job.ID, job.Title)

	switch job.Type {
	case queue.JobTypeConvert:
		return s.executeConvertJob(ctx, job, progressCallback)
	case queue.JobTypeMerge:
		return fmt.Errorf("merge jobs not yet implemented")
	case queue.JobTypeTrim:
		return fmt.Errorf("trim jobs not yet implemented")
	case queue.JobTypeFilter:
		return fmt.Errorf("filter jobs not yet implemented")
	case queue.JobTypeUpscale:
		return fmt.Errorf("upscale jobs not yet implemented")
	case queue.JobTypeAudio:
		return fmt.Errorf("audio jobs not yet implemented")
	case queue.JobTypeThumb:
		return fmt.Errorf("thumb jobs not yet implemented")
	default:
		return fmt.Errorf("unknown job type: %s", job.Type)
	}
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
	selectedFormat, _ := cfg["selectedFormat"].(formatOption)
	isDVD := selectedFormat.Ext == ".mpg"
	var targetOption string

	// DVD presets: enforce compliant target, frame rate, resolution, codecs
	if isDVD {
		if strings.Contains(selectedFormat.Label, "PAL") {
			targetOption = "pal-dvd"
			cfg["frameRate"] = "25"
			cfg["targetResolution"] = "PAL (720×576)"
		} else {
			targetOption = "ntsc-dvd"
			cfg["frameRate"] = "29.97"
			cfg["targetResolution"] = "NTSC (720×480)"
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
	// Note: NVENC doesn't need -hwaccel for encoding, only for decoding
	hardwareAccel, _ := cfg["hardwareAccel"].(string)
	if hardwareAccel != "none" && hardwareAccel != "" {
		switch hardwareAccel {
		case "nvenc":
			// For NVENC, we don't add -hwaccel flags
			// The h264_nvenc/hevc_nvenc encoder handles GPU encoding directly
			// Only add hwaccel if we want GPU decoding too, which can cause issues
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
	srcAspect := utils.AspectRatioFloat(sourceWidth, sourceHeight)
	outputAspect, _ := cfg["outputAspect"].(string)
	aspectHandling, _ := cfg["aspectHandling"].(string)

	// Create temp source for aspect calculation
	tempSrc := &videoSource{Width: sourceWidth, Height: sourceHeight}
	targetAspect := resolveTargetAspect(outputAspect, tempSrc)
	if targetAspect > 0 && srcAspect > 0 && !utils.RatiosApproxEqual(targetAspect, srcAspect, 0.01) {
		vf = append(vf, aspectFilters(targetAspect, aspectHandling)...)
	}

	// Frame rate
	frameRate, _ := cfg["frameRate"].(string)
	if frameRate != "" && frameRate != "Source" {
		vf = append(vf, "fps="+frameRate)
	}

	if len(vf) > 0 {
		args = append(args, "-vf", strings.Join(vf, ","))
	}

	// Video codec
	videoCodec, _ := cfg["videoCodec"].(string)
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
			if bitrateMode == "CRF" || bitrateMode == "" {
				crfStr, _ := cfg["crf"].(string)
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

			// Encoder preset
			if encoderPreset, _ := cfg["encoderPreset"].(string); encoderPreset != "" && (actualCodec == "libx264" || actualCodec == "libx265") {
				args = append(args, "-preset", encoderPreset)
			}

			// Pixel format
			if pixelFormat, _ := cfg["pixelFormat"].(string); pixelFormat != "" {
				args = append(args, "-pix_fmt", pixelFormat)
			}

			// H.264 profile and level for compatibility
			if videoCodec == "H.264" && (strings.Contains(actualCodec, "264") || strings.Contains(actualCodec, "h264")) {
				if h264Profile, _ := cfg["h264Profile"].(string); h264Profile != "" && h264Profile != "Auto" {
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

	// Map cover art
	if hasCoverArt {
		args = append(args, "-map", "0:v", "-map", "0:a?", "-map", "1:v")
		args = append(args, "-c:v:1", "png")
		args = append(args, "-disposition:v:1", "attached_pic")
	}

	// Format-specific settings (already parsed above for DVD check)
	switch v := cfg["selectedFormat"].(type) {
	case formatOption:
		selectedFormat = v
	case map[string]interface{}:
		// Reconstruct from map (happens when loading from JSON)
		if label, ok := v["Label"].(string); ok {
			selectedFormat.Label = label
		}
		if ext, ok := v["Ext"].(string); ok {
			selectedFormat.Ext = ext
		}
		if codec, ok := v["VideoCodec"].(string); ok {
			selectedFormat.VideoCodec = codec
		}
	default:
		// Fallback to MP4
		selectedFormat = formatOptions[0]
	}

	if strings.EqualFold(selectedFormat.Ext, ".mp4") || strings.EqualFold(selectedFormat.Ext, ".mov") {
		args = append(args, "-movflags", "+faststart")
	}

	if targetOption != "" {
		args = append(args, "-target", targetOption)
	}

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

	logging.Debug(logging.CatFFMPEG, "queue convert command: ffmpeg %s", strings.Join(args, " "))

	// Also print to stdout for debugging
	fmt.Printf("\n=== FFMPEG COMMAND ===\nffmpeg %s\n======================\n\n", strings.Join(args, " "))

	// Execute FFmpeg
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Capture stderr for error messages
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Parse progress
	scanner := bufio.NewScanner(stdout)
	var duration float64
	if d, ok := cfg["sourceDuration"].(float64); ok && d > 0 {
		duration = d
	}
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "out_time_ms=") {
			val := strings.TrimPrefix(line, "out_time_ms=")
			if ms, err := strconv.ParseInt(val, 10, 64); err == nil && ms > 0 {
				currentSec := float64(ms) / 1000000.0
				if duration > 0 {
					progress := (currentSec / duration) * 100.0
					if progress > 100 {
						progress = 100
					}
					progressCallback(progress)
				}
			}
		} else if strings.HasPrefix(line, "duration_ms=") {
			val := strings.TrimPrefix(line, "duration_ms=")
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

	logging.Debug(logging.CatFFMPEG, "queue conversion completed: %s", outputPath)
	return nil
}

func (s *appState) shutdown() {
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
	// Use a generous default window size that fits typical desktops without overflowing.
	w.Resize(fyne.NewSize(1280, 800))
	w.SetFixedSize(false) // Allow manual resizing
	logging.Debug(logging.CatUI, "window initialized with manual resizing enabled")

	state := &appState{
		window: w,
		convert: convertConfig{
			OutputBase:     "converted",
			SelectedFormat: formatOptions[0],
			Quality:        "Standard (CRF 23)",
			Mode:           "Simple",

			// Video encoding defaults
			VideoCodec:       "H.264",
			EncoderPreset:    "medium",
			CRF:              "", // Empty means use Quality preset
			BitrateMode:      "CRF",
			VideoBitrate:     "5000k",
			TargetResolution: "Source",
			FrameRate:        "Source",
			PixelFormat:      "yuv420p",
			HardwareAccel:    "none",
			TwoPass:          false,
			H264Profile:        "main",
			H264Level:          "4.0",
			Deinterlace:        "Auto",
			DeinterlaceMethod:  "bwdif",
			AutoCrop:           false,

			// Audio encoding defaults
			AudioCodec:       "AAC",
			AudioBitrate:     "192k",
			AudioChannels:    "Source",
			AudioSampleRate:  "Source",
			NormalizeAudio:   false,

			// Other defaults
			InverseTelecine:  true,
			InverseAutoNotes: "Default smoothing for interlaced footage.",
			OutputAspect:     "Source",
			AspectHandling:   "Auto",
		},
		player:       player.New(),
		playerVolume: 100,
		lastVolume:   100,
		playerMuted:  false,
		playerPaused: true,
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
	videoPanel := buildVideoPane(state, fyne.NewSize(460, 260), src, updateCover)
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

	dvdAspectBox := container.NewVBox(dvdAspectLabel, dvdAspectSelect, dvdInfoLabel)
	dvdAspectBox.Hide() // Hidden by default

	// Placeholder for updateDVDOptions - will be defined after resolution/framerate selects are created
	var updateDVDOptions func()

	// Create formatSelect with callback that updates DVD options
	formatSelect := widget.NewSelect(formatLabels, func(value string) {
		for _, opt := range formatOptions {
			if opt.Label == value {
				logging.Debug(logging.CatUI, "format set to %s", value)
				state.convert.SelectedFormat = opt
				outputHint.SetText(fmt.Sprintf("Output file: %s", state.convert.OutputFile()))
				if updateDVDOptions != nil {
					updateDVDOptions() // Show/hide DVD options and auto-set resolution
				}
				break
			}
		}
	})
	formatSelect.SetSelected(state.convert.SelectedFormat.Label)

	qualitySelect := widget.NewSelect([]string{"Draft (CRF 28)", "Standard (CRF 23)", "High (CRF 18)", "Lossless"}, func(value string) {
		logging.Debug(logging.CatUI, "quality preset %s", value)
		state.convert.Quality = value
	})
	qualitySelect.SetSelected(state.convert.Quality)

	outputEntry := widget.NewEntry()
	outputEntry.SetText(state.convert.OutputBase)
	outputEntry.OnChanged = func(val string) {
		state.convert.OutputBase = val
		outputHint.SetText(fmt.Sprintf("Output file: %s", state.convert.OutputFile()))
	}

	inverseCheck := widget.NewCheck("Smart Inverse Telecine", func(checked bool) {
		state.convert.InverseTelecine = checked
	})
	inverseCheck.Checked = state.convert.InverseTelecine
	inverseHint := widget.NewLabel(state.convert.InverseAutoNotes)

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

	aspectTargets := []string{"Source", "16:9", "4:3", "1:1", "9:16", "21:9"}
	targetAspectSelect := widget.NewSelect(aspectTargets, func(value string) {
		logging.Debug(logging.CatUI, "target aspect set to %s", value)
		state.convert.OutputAspect = value
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

	// Settings management for batch operations
	settingsInfoLabel := widget.NewLabel("Settings persist across videos. Change them anytime to affect all subsequent videos.")
	// Don't wrap - let text scroll or truncate if needed
	settingsInfoLabel.Alignment = fyne.TextAlignCenter

	resetSettingsBtn := widget.NewButton("Reset to Defaults", func() {
		// Reset to default settings
		state.convert = convertConfig{
			SelectedFormat:   formatOptions[0],
			OutputBase:       "converted",
			Quality:          "Standard (CRF 23)",
			InverseTelecine:  false,
			OutputAspect:     "Source",
			AspectHandling:   "Auto",
			VideoCodec:       "H.264",
			EncoderPreset:    "medium",
			BitrateMode:      "CRF",
			CRF:              "",
			VideoBitrate:     "",
			TargetResolution: "Source",
			FrameRate:        "Source",
			PixelFormat:      "yuv420p",
			HardwareAccel:    "none",
			AudioCodec:       "AAC",
			AudioBitrate:     "192k",
			AudioChannels:    "Source",
		}
		logging.Debug(logging.CatUI, "settings reset to defaults")
		// Refresh all UI elements to show new settings
		formatSelect.SetSelected(state.convert.SelectedFormat.Label)
		qualitySelect.SetSelected(state.convert.Quality)
		outputEntry.SetText(state.convert.OutputBase)
	})
	resetSettingsBtn.Importance = widget.LowImportance

	// Create collapsible batch settings section
	settingsContent := container.NewVBox(
		settingsInfoLabel,
		resetSettingsBtn,
	)
	settingsContent.Hide() // Hidden by default

	// Use a pointer to track visibility state
	settingsVisible := false

	var toggleSettingsBtn *widget.Button
	toggleSettingsBtn = widget.NewButton("Show Batch Settings", func() {
		if settingsVisible {
			settingsContent.Hide()
			toggleSettingsBtn.SetText("Show Batch Settings")
			settingsVisible = false
		} else {
			settingsContent.Show()
			toggleSettingsBtn.SetText("Hide Batch Settings")
			settingsVisible = true
		}
	})
	toggleSettingsBtn.Importance = widget.LowImportance

	settingsBox := container.NewVBox(
		toggleSettingsBtn,
		settingsContent,
		widget.NewSeparator(),
	)

	// Simple mode options - minimal controls, aspect locked to Source
	simpleOptions := container.NewVBox(
		widget.NewLabelWithStyle("═══ OUTPUT ═══", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Format", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		formatSelect,
		dvdAspectBox, // DVD options appear here when DVD format selected
		widget.NewLabelWithStyle("Output Name", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		outputEntry,
		outputHint,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("═══ QUALITY ═══", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		qualitySelect,
		widget.NewLabel("Aspect ratio will match source video"),
		layout.NewSpacer(),
	)

	// Cover art display on one line
	coverDisplay = widget.NewLabel("Cover Art: " + state.convert.CoverLabel())

	// Video Codec selection
	videoCodecSelect := widget.NewSelect([]string{"H.264", "H.265", "VP9", "AV1", "Copy"}, func(value string) {
		state.convert.VideoCodec = value
		logging.Debug(logging.CatUI, "video codec set to %s", value)
	})
	videoCodecSelect.SetSelected(state.convert.VideoCodec)

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

	// Bitrate Mode
	bitrateModeSelect := widget.NewSelect([]string{"CRF", "CBR", "VBR", "Target Size"}, func(value string) {
		state.convert.BitrateMode = value
		logging.Debug(logging.CatUI, "bitrate mode set to %s", value)
	})
	bitrateModeSelect.SetSelected(state.convert.BitrateMode)

	// Manual CRF entry
	crfEntry := widget.NewEntry()
	crfEntry.SetPlaceHolder("Auto (from Quality preset)")
	crfEntry.SetText(state.convert.CRF)
	crfEntry.OnChanged = func(val string) {
		state.convert.CRF = val
	}

	// Video Bitrate entry (for CBR/VBR)
	videoBitrateEntry := widget.NewEntry()
	videoBitrateEntry.SetPlaceHolder("5000k")
	videoBitrateEntry.SetText(state.convert.VideoBitrate)
	videoBitrateEntry.OnChanged = func(val string) {
		state.convert.VideoBitrate = val
	}

	// Target File Size with smart presets + manual entry
	targetFileSizeEntry := widget.NewEntry()
	targetFileSizeEntry.SetPlaceHolder("e.g., 25MB, 100MB, 8MB")

	var targetFileSizeSelect *widget.Select

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

	// Target Resolution
	resolutionSelect := widget.NewSelect([]string{"Source", "720p", "1080p", "1440p", "4K", "NTSC (720×480)", "PAL (720×576)"}, func(value string) {
		state.convert.TargetResolution = value
		logging.Debug(logging.CatUI, "target resolution set to %s", value)
	})
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

	// Pixel Format
	pixelFormatSelect := widget.NewSelect([]string{"yuv420p", "yuv422p", "yuv444p"}, func(value string) {
		state.convert.PixelFormat = value
		logging.Debug(logging.CatUI, "pixel format set to %s", value)
	})
	pixelFormatSelect.SetSelected(state.convert.PixelFormat)

	// Hardware Acceleration
	hwAccelSelect := widget.NewSelect([]string{"none", "nvenc", "vaapi", "qsv", "videotoolbox"}, func(value string) {
		state.convert.HardwareAccel = value
		logging.Debug(logging.CatUI, "hardware accel set to %s", value)
	})
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
		isDVD := state.convert.SelectedFormat.Ext == ".mpg"
		if isDVD {
			dvdAspectBox.Show()
			// Auto-set resolution and framerate based on DVD format
			if strings.Contains(state.convert.SelectedFormat.Label, "NTSC") {
				dvdInfoLabel.SetText("NTSC: 720×480 @ 29.97fps, MPEG-2, AC-3 Stereo 48kHz\nBitrate: 6000k (default), 9000k (max PS2-safe)\nCompatible with DVDStyler, PS2, standalone DVD players")
				// Auto-set to NTSC resolution
				resolutionSelect.SetSelected("NTSC (720×480)")
				frameRateSelect.SetSelected("30") // Will be converted to 29.97fps
				state.convert.TargetResolution = "NTSC (720×480)"
				state.convert.FrameRate = "30"
			} else if strings.Contains(state.convert.SelectedFormat.Label, "PAL") {
				dvdInfoLabel.SetText("PAL: 720×576 @ 25.00fps, MPEG-2, AC-3 Stereo 48kHz\nBitrate: 8000k (default), 9500k (max PS2-safe)\nCompatible with European DVD players and authoring tools")
				// Auto-set to PAL resolution
				resolutionSelect.SetSelected("PAL (720×576)")
				frameRateSelect.SetSelected("25")
				state.convert.TargetResolution = "PAL (720×576)"
				state.convert.FrameRate = "25"
			} else {
				dvdInfoLabel.SetText("DVD Format selected")
			}
		} else {
			dvdAspectBox.Hide()
		}
	}

	// Advanced mode options - full controls with organized sections
	advancedOptions := container.NewVBox(
		widget.NewLabelWithStyle("═══ OUTPUT ═══", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Format", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		formatSelect,
		dvdAspectBox, // DVD options appear here when DVD format selected
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
		widget.NewLabelWithStyle("Quality Preset", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		qualitySelect,
		widget.NewLabelWithStyle("Bitrate Mode", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		bitrateModeSelect,
		widget.NewLabelWithStyle("Manual CRF (overrides Quality preset)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		crfEntry,
		widget.NewLabelWithStyle("Video Bitrate (for CBR/VBR)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		videoBitrateEntry,
		widget.NewLabelWithStyle("Target File Size", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		targetFileSizeSelect,
		targetFileSizeEntry,
		widget.NewLabelWithStyle("Target Resolution", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		resolutionSelect,
		widget.NewLabelWithStyle("Frame Rate", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		frameRateSelect,
		frameRateHint,
		widget.NewLabelWithStyle("Pixel Format", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		pixelFormatSelect,
		widget.NewLabelWithStyle("Hardware Acceleration", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		hwAccelSelect,
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

		widget.NewLabelWithStyle("═══ DEINTERLACING ═══", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		inverseCheck,
		inverseHint,
		layout.NewSpacer(),
	)

	// Create tabs for Simple/Advanced modes
	// Wrap simple options with settings box at top
	simpleWithSettings := container.NewVBox(
		settingsBox,
		simpleOptions,
	)

	// Keep Simple lightweight; wrap Advanced in its own scroll to avoid bloating MinSize.
	simpleScrollBox := simpleWithSettings
	advancedScrollBox := container.NewVScroll(advancedOptions)
	advancedScrollBox.SetMinSize(fyne.NewSize(0, 0))

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
			// Lock aspect ratio to Source in Simple mode
			state.convert.OutputAspect = "Source"
			targetAspectSelect.SetSelected("Source")
			updateAspectBoxVisibility()
			logging.Debug(logging.CatUI, "convert mode selected: Simple (aspect locked to Source)")
		} else {
			state.convert.Mode = "Advanced"
			logging.Debug(logging.CatUI, "convert mode selected: Advanced")
		}
	}

	// Ensure Simple mode starts with Source aspect
	if state.convert.Mode == "Simple" {
		state.convert.OutputAspect = "Source"
		targetAspectSelect.SetSelected("Source")
	}

	optionsRect := canvas.NewRectangle(utils.MustHex("#13182B"))
	optionsRect.CornerRadius = 8
	optionsRect.StrokeColor = gridColor
	optionsRect.StrokeWidth = 1
	optionsPanel := container.NewMax(optionsRect, container.NewPadded(tabs))

	snippetBtn := widget.NewButton("Generate Snippet", func() {
		if state.source == nil {
			dialog.ShowInformation("Snippet", "Load a video first.", state.window)
			return
		}
		go state.generateSnippet()
	})
	snippetBtn.Importance = widget.MediumImportance
	if src == nil {
		snippetBtn.Disable()
	}
	snippetHint := widget.NewLabel("Creates a 20s clip centred on the timeline midpoint.")
	snippetRow := container.NewHBox(snippetBtn, layout.NewSpacer(), snippetHint)

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
		qualitySelect.SetSelected("Standard (CRF 23)")
		aspectOptions.SetSelected("Auto")
		targetAspectSelect.SetSelected("Source")
		updateAspectBoxVisibility()
		logging.Debug(logging.CatUI, "convert settings reset to defaults")
	})
	statusLabel := widget.NewLabel("")
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
	cancelBtn = widget.NewButton("Cancel", func() {
		state.cancelConvert(cancelBtn, convertBtn, activity, statusLabel)
	})
	cancelBtn.Importance = widget.DangerImportance
	cancelBtn.Disable()

	// Add to Queue button
	addQueueBtn := widget.NewButton("Add to Queue", func() {
		if err := state.addConvertToQueue(); err != nil {
			dialog.ShowError(err, state.window)
		} else {
			dialog.ShowInformation("Queue", "Job added to queue!", state.window)
			// Auto-start queue if not already running
			if state.jobQueue != nil && !state.jobQueue.IsRunning() && !state.convertBusy {
				state.jobQueue.Start()
				logging.Debug(logging.CatUI, "queue auto-started after adding job")
			}
		}
	})
	if src == nil {
		addQueueBtn.Disable()
	}

	convertBtn = widget.NewButton("CONVERT NOW", func() {
		// Check if queue is already running
		if state.jobQueue != nil && state.jobQueue.IsRunning() {
			dialog.ShowInformation("Queue Active",
				"The conversion queue is currently running. Click \"Add to Queue\" to add this video to the queue instead.",
				state.window)
			// Auto-add to queue instead
			if err := state.addConvertToQueue(); err != nil {
				dialog.ShowError(err, state.window)
			} else {
				dialog.ShowInformation("Queue", "Job added to queue!", state.window)
			}
			return
		}
		state.startConvert(statusLabel, convertBtn, cancelBtn, activity)
	})
	convertBtn.Importance = widget.HighImportance
	if src == nil {
		convertBtn.Disable()
	}
	if state.convertBusy {
		// Allow queueing new jobs while current convert runs; just disable Convert Now and enable Cancel.
		convertBtn.Disable()
		cancelBtn.Enable()
		addQueueBtn.Enable()
	}
	// Also disable if queue is running
	if state.jobQueue != nil && state.jobQueue.IsRunning() {
		convertBtn.Disable()
		addQueueBtn.Enable()
	}

	leftControls := container.NewHBox(resetBtn)
	centerStatus := container.NewHBox(activity, statusLabel)
	rightControls := container.NewHBox(cancelBtn, addQueueBtn, convertBtn)
	actionInner := container.NewBorder(nil, nil, leftControls, rightControls, centerStatus)
	actionBar := ui.TintedBar(convertColor, actionInner)

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

	// Stack status + snippet + actions tightly to avoid dead air, outside the scroll area.
	bottomSection := container.NewVBox(state.statsBar, snippetRow, widget.NewSeparator(), actionBar)

	scrollableMain := container.NewVScroll(mainContent)

	return container.NewBorder(backBar, bottomSection, nil, nil, container.NewMax(scrollableMain))
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
		widget.NewFormItem("Format", widget.NewLabel(utils.FirstNonEmpty(src.Format, "Unknown"))),
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

	// Layout: metadata form on left, cover art on right (bottom-aligned)
	coverColumn := container.NewVBox(layout.NewSpacer(), coverContainer)
	contentArea := container.NewBorder(nil, nil, nil, coverColumn, info)

	body := container.NewVBox(
		top,
		widget.NewSeparator(),
		contentArea,
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
	outer.SetMinSize(fyne.NewSize(targetWidth, targetHeight))

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
	// Let the image grow with the available stage size
	img.SetMinSize(fyne.NewSize(targetWidth, targetHeight))
	stage := canvas.NewRectangle(utils.MustHex("#0F1529"))
	stage.CornerRadius = 6
	stage.SetMinSize(fyne.NewSize(targetWidth, targetHeight))
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
			container.NewHBox(playBtn, fullBtn, coverBtn, importBtn, layout.NewSpacer(), volBox),
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
			container.NewHBox(playBtn, coverBtn, importBtn, layout.NewSpacer(), widget.NewLabel("🔇"), container.NewMax(volSlider)),
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
	cmd := exec.Command("ffmpeg", args...)
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
	cmd := exec.Command("ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-ss", fmt.Sprintf("%.3f", offset),
		"-i", p.path,
		"-vn",
		"-ac", fmt.Sprintf("%d", channels),
		"-ar", fmt.Sprintf("%d", sampleRate),
		"-f", "s16le",
		"-",
	)
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

		// If multiple videos, add all to queue
		if len(videoPaths) > 1 {
			logging.Debug(logging.CatUI, "multiple videos dropped in convert module; adding all to queue")
			go s.batchAddToQueue(videoPaths)
		} else {
			// Single video: load it
			logging.Debug(logging.CatUI, "single video dropped in convert module; loading: %s", videoPaths[0])
			go s.loadVideo(videoPaths[0])
		}
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
	base := strings.TrimSuffix(src.DisplayName, filepath.Ext(src.DisplayName))
	s.convert.OutputBase = base + "-convert"
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
	base := strings.TrimSuffix(src.DisplayName, filepath.Ext(src.DisplayName))
	s.convert.OutputBase = base + "-convert"

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
	case "Draft (CRF 28)":
		return "28"
	case "High (CRF 18)":
		return "18"
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
		cmd := exec.Command("ffmpeg", "-hide_banner", "-encoders")
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
	cmd := exec.Command("ffmpeg", "-hide_banner", "-encoders")
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
		cmd := exec.Command("ffmpeg", "-hide_banner", "-encoders")
		output, err := cmd.CombinedOutput()
		if err == nil {
			if strings.Contains(string(output), " "+encoder+" ") || strings.Contains(string(output), " "+encoder+"\n") {
				logging.Debug(logging.CatFFMPEG, "detected hardware encoder: %s", encoder)
				return encoder
			}
		}
	}

	cmd := exec.Command("ffmpeg", "-hide_banner", "-encoders")
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
	switch cfg.VideoCodec {
	case "H.264":
		if cfg.HardwareAccel == "nvenc" {
			return "h264_nvenc"
		} else if cfg.HardwareAccel == "qsv" {
			return "h264_qsv"
		} else if cfg.HardwareAccel == "videotoolbox" {
			return "h264_videotoolbox"
		}
		// When set to "none" or empty, use software encoder
		return "libx264"
	case "H.265":
		if cfg.HardwareAccel == "nvenc" {
			return "hevc_nvenc"
		} else if cfg.HardwareAccel == "qsv" {
			return "hevc_qsv"
		} else if cfg.HardwareAccel == "videotoolbox" {
			return "hevc_videotoolbox"
		}
		// When set to "none" or empty, use software encoder
		return "libx265"
	case "VP9":
		return "libvpx-vp9"
	case "AV1":
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
	isDVD := cfg.SelectedFormat.Ext == ".mpg"
	var targetOption string
	outDir := filepath.Dir(src.Path)
	outName := cfg.OutputFile()
	if outName == "" {
		outName = "converted" + cfg.SelectedFormat.Ext
	}
	outPath := filepath.Join(outDir, outName)
	if outPath == src.Path {
		outPath = filepath.Join(outDir, "converted-"+outName)
	}

	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "error",
	}

	// DVD presets: enforce compliant codecs, frame rate, resolution, and target
	if isDVD {
		if strings.Contains(cfg.SelectedFormat.Label, "PAL") {
			targetOption = "pal-dvd"
			cfg.FrameRate = "25"
			cfg.TargetResolution = "PAL (720×576)"
		} else {
			targetOption = "ntsc-dvd"
			cfg.FrameRate = "29.97"
			cfg.TargetResolution = "NTSC (720×480)"
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

	// Hardware acceleration for decoding
	// Note: NVENC doesn't need -hwaccel for encoding, only for decoding
	if cfg.HardwareAccel != "none" && cfg.HardwareAccel != "" {
		switch cfg.HardwareAccel {
		case "nvenc":
			// For NVENC, we don't add -hwaccel flags
			// The h264_nvenc/hevc_nvenc encoder handles GPU encoding directly
		case "vaapi":
			args = append(args, "-hwaccel", "vaapi")
		case "qsv":
			args = append(args, "-hwaccel", "qsv")
		case "videotoolbox":
			args = append(args, "-hwaccel", "videotoolbox")
		}
		logging.Debug(logging.CatFFMPEG, "hardware acceleration: %s", cfg.HardwareAccel)
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
		}
		if scaleFilter != "" {
			vf = append(vf, scaleFilter)
		}
	}

	// Aspect ratio conversion
	srcAspect := utils.AspectRatioFloat(src.Width, src.Height)
	targetAspect := resolveTargetAspect(cfg.OutputAspect, src)
	if targetAspect > 0 && srcAspect > 0 && !utils.RatiosApproxEqual(targetAspect, srcAspect, 0.01) {
		vf = append(vf, aspectFilters(targetAspect, cfg.AspectHandling)...)
	}

	// Frame rate
	if cfg.FrameRate != "" && cfg.FrameRate != "Source" {
		vf = append(vf, "fps="+cfg.FrameRate)
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
			if cfg.VideoBitrate != "" {
				args = append(args, "-b:v", cfg.VideoBitrate, "-minrate", cfg.VideoBitrate, "-maxrate", cfg.VideoBitrate, "-bufsize", cfg.VideoBitrate)
			}
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
	if targetOption != "" {
		args = append(args, "-target", targetOption)
	}

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
	setStatus("Preparing conversion…")
	// Widget states will be updated by the UI refresh ticker

	ctx, cancel := context.WithCancel(context.Background())
	s.convertCancel = cancel

	go func() {
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			setStatus("Running ffmpeg…")
		}, false)

		started := time.Now()
		cmd := exec.CommandContext(ctx, "ffmpeg", args...)
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
		cmd.Stderr = &stderr

		progressQuit := make(chan struct{})
		go func() {
			scanner := bufio.NewScanner(stdout)
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
					lbl := fmt.Sprintf("Converting… %.0f%% | elapsed %s | ETA %s | %.2fx", pct, formatShortDuration(elapsedWall), etaOrDash(eta), speed)
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						s.convertProgress = pct
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
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					s.convertBusy = false
					s.convertActiveIn = ""
					s.convertActiveOut = ""
					s.convertProgress = 0
					setStatus("Cancelled")
				}, false)
				s.convertCancel = nil
				return
			}
			stderrOutput := strings.TrimSpace(stderr.String())
			logging.Debug(logging.CatFFMPEG, "convert failed: %v stderr=%s", err, stderrOutput)
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				errorExplanation := interpretFFmpegError(err)
				var errorMsg error

				// Check if this is a hardware encoding failure
				isHardwareFailure := strings.Contains(stderrOutput, "No capable devices found") ||
					strings.Contains(stderrOutput, "Cannot load") ||
					strings.Contains(stderrOutput, "not available") &&
					(strings.Contains(stderrOutput, "nvenc") ||
					 strings.Contains(stderrOutput, "qsv") ||
					 strings.Contains(stderrOutput, "vaapi") ||
					 strings.Contains(stderrOutput, "videotoolbox"))

				if isHardwareFailure && s.convert.HardwareAccel != "none" && s.convert.HardwareAccel != "" {
					errorMsg = fmt.Errorf("Hardware encoding (%s) failed - no compatible hardware found.\n\nPlease disable hardware acceleration in the conversion settings and try again with software encoding.\n\nFFmpeg output:\n%s", s.convert.HardwareAccel, stderrOutput)
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
				s.convertProgress = 0
				setStatus("Failed")
			}, false)
			s.convertCancel = nil
			return
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
			s.convertProgress = 100
			setStatus("Done")
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

	// Add cover art if available
	hasCoverArt := s.convert.CoverArtPath != ""
	logging.Debug(logging.CatFFMPEG, "snippet: CoverArtPath=%s hasCoverArt=%v", s.convert.CoverArtPath, hasCoverArt)
	if hasCoverArt {
		args = append(args, "-i", s.convert.CoverArtPath)
		logging.Debug(logging.CatFFMPEG, "snippet: added cover art input %s", s.convert.CoverArtPath)
	}

	// Build video filters (snippets should be fast - only apply essential filters)
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
		}
		if scaleFilter != "" {
			vf = append(vf, scaleFilter)
		}
	}

	// Check if aspect ratio conversion is needed
	srcAspect := utils.AspectRatioFloat(src.Width, src.Height)
	targetAspect := resolveTargetAspect(s.convert.OutputAspect, src)
	aspectConversionNeeded := targetAspect > 0 && srcAspect > 0 && !utils.RatiosApproxEqual(targetAspect, srcAspect, 0.01)
	if aspectConversionNeeded {
		vf = append(vf, aspectFilters(targetAspect, s.convert.AspectHandling)...)
	}

	// Frame rate conversion (only if explicitly set and different from source)
	if s.convert.FrameRate != "" && s.convert.FrameRate != "Source" {
		vf = append(vf, "fps="+s.convert.FrameRate)
	}

	// WMV files must be re-encoded for MP4 compatibility (wmv3/wmav2 can't be copied to MP4)
	isWMV := strings.HasSuffix(strings.ToLower(src.Path), ".wmv")
	needsReencode := len(vf) > 0 || isWMV

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
		// Filters required - must re-encode
		// Use configured codec or fallback to H.264 for compatibility
		videoCodec := determineVideoCodec(s.convert)
		if videoCodec == "copy" {
			videoCodec = "libx264"
		}
		args = append(args, "-c:v", videoCodec)

		// Use configured CRF or fallback to quality preset
		crf := s.convert.CRF
		if crf == "" {
			crf = crfForQuality(s.convert.Quality)
		}
		if videoCodec == "libx264" || videoCodec == "libx265" {
			args = append(args, "-crf", crf)
			// Use faster preset for snippets
			args = append(args, "-preset", "veryfast")
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

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
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
	cmd := exec.Command("ffmpeg",
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
	Path             string
	DisplayName      string
	Format           string
	Width            int
	Height           int
	Duration         float64
	VideoCodec       string
	AudioCodec       string
	Bitrate          int    // Video bitrate in bits per second
	AudioBitrate     int    // Audio bitrate in bits per second
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
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var result struct {
		Format struct {
			Filename   string `json:"filename"`
			Format     string `json:"format_long_name"`
			Duration   string `json:"duration"`
			FormatName string `json:"format_name"`
			BitRate    string `json:"bit_rate"`
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
		extractCmd := exec.CommandContext(ctx, "ffmpeg",
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Sample 10 seconds from the middle of the video
	sampleStart := duration / 2
	if sampleStart < 0 {
		sampleStart = 0
	}

	// Run ffmpeg with cropdetect filter
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-ss", fmt.Sprintf("%.2f", sampleStart),
		"-i", path,
		"-t", "10",
		"-vf", "cropdetect=24:16:0",
		"-f", "null",
		"-",
	)

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

// buildCompareView creates the UI for comparing two videos side by side
func buildCompareView(state *appState) fyne.CanvasObject {
	compareColor := moduleColor("compare")

	// Header
	title := canvas.NewText("COMPARE VIDEOS", compareColor)
	title.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	title.TextSize = 24

	backBtn := widget.NewButton("← Back to Menu", func() {
		state.showMainMenu()
	})
	backBtn.Importance = widget.LowImportance

	header := container.NewBorder(nil, nil, backBtn, nil, container.NewCenter(title))

	// Instructions
	instructions := widget.NewLabel("Load two videos to compare their metadata and visual differences side by side.")
	instructions.Wrapping = fyne.TextWrapWord
	instructions.Alignment = fyne.TextAlignCenter

	// File 1 (Source/Original)
	file1Label := widget.NewLabel("File 1: Not loaded")
	file1Label.TextStyle = fyne.TextStyle{Bold: true}

	file1SelectBtn := widget.NewButton("Load File 1", func() {
		// File picker for first file
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			path := reader.URI().Path()
			reader.Close()

			// Probe the video
			src, err := probeVideo(path)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to load video: %w", err), state.window)
				return
			}

			file1Label.SetText(fmt.Sprintf("File 1: %s", filepath.Base(path)))
			state.compareFile1 = src
			logging.Debug(logging.CatModule, "loaded compare file 1: %s", path)
		}, state.window)
	})

	file1Info := widget.NewLabel("No file loaded")
	file1Info.Wrapping = fyne.TextWrapWord

	// File 2 (Output/Converted)
	file2Label := widget.NewLabel("File 2: Not loaded")
	file2Label.TextStyle = fyne.TextStyle{Bold: true}

	file2SelectBtn := widget.NewButton("Load File 2", func() {
		// File picker for second file
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			path := reader.URI().Path()
			reader.Close()

			// Probe the video
			src, err := probeVideo(path)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to load video: %w", err), state.window)
				return
			}

			file2Label.SetText(fmt.Sprintf("File 2: %s", filepath.Base(path)))
			state.compareFile2 = src
			logging.Debug(logging.CatModule, "loaded compare file 2: %s", path)
		}, state.window)
	})

	file2Info := widget.NewLabel("No file loaded")
	file2Info.Wrapping = fyne.TextWrapWord

	// Compare button
	compareBtn := widget.NewButton("COMPARE", func() {
		if state.compareFile1 == nil || state.compareFile2 == nil {
			dialog.ShowInformation("Compare Videos", "Please load both files first.", state.window)
			return
		}

		// Build comparison data
		f1 := state.compareFile1
		f2 := state.compareFile2

		file1Info.SetText(fmt.Sprintf(
			"Format: %s\n"+
				"Resolution: %dx%d\n"+
				"Duration: %s\n"+
				"Video Codec: %s\n"+
				"Audio Codec: %s\n"+
				"Video Bitrate: %s\n"+
				"Audio Bitrate: %s\n"+
				"Frame Rate: %.2f fps\n"+
				"Pixel Format: %s\n"+
				"Aspect Ratio: %s\n"+
				"Color Space: %s\n"+
				"Color Range: %s\n"+
				"GOP Size: %d\n"+
				"Field Order: %s\n"+
				"Chapters: %v\n"+
				"Metadata: %v",
			f1.Format,
			f1.Width, f1.Height,
			f1.DurationString(),
			f1.VideoCodec,
			f1.AudioCodec,
			formatBitrate(f1.Bitrate),
			formatBitrate(f1.AudioBitrate),
			f1.FrameRate,
			f1.PixelFormat,
			f1.AspectRatioString(),
			f1.ColorSpace,
			f1.ColorRange,
			f1.GOPSize,
			f1.FieldOrder,
			f1.HasChapters,
			f1.HasMetadata,
		))

		file2Info.SetText(fmt.Sprintf(
			"Format: %s\n"+
				"Resolution: %dx%d\n"+
				"Duration: %s\n"+
				"Video Codec: %s\n"+
				"Audio Codec: %s\n"+
				"Video Bitrate: %s\n"+
				"Audio Bitrate: %s\n"+
				"Frame Rate: %.2f fps\n"+
				"Pixel Format: %s\n"+
				"Aspect Ratio: %s\n"+
				"Color Space: %s\n"+
				"Color Range: %s\n"+
				"GOP Size: %d\n"+
				"Field Order: %s\n"+
				"Chapters: %v\n"+
				"Metadata: %v",
			f2.Format,
			f2.Width, f2.Height,
			f2.DurationString(),
			f2.VideoCodec,
			f2.AudioCodec,
			formatBitrate(f2.Bitrate),
			formatBitrate(f2.AudioBitrate),
			f2.FrameRate,
			f2.PixelFormat,
			f2.AspectRatioString(),
			f2.ColorSpace,
			f2.ColorRange,
			f2.GOPSize,
			f2.FieldOrder,
			f2.HasChapters,
			f2.HasMetadata,
		))
	})
	compareBtn.Importance = widget.HighImportance

	// Layout
	file1Box := container.NewVBox(
		file1Label,
		file1SelectBtn,
		widget.NewSeparator(),
		container.NewScroll(file1Info),
	)

	file2Box := container.NewVBox(
		file2Label,
		file2SelectBtn,
		widget.NewSeparator(),
		container.NewScroll(file2Info),
	)

	content := container.NewVBox(
		header,
		widget.NewSeparator(),
		instructions,
		widget.NewSeparator(),
		compareBtn,
		widget.NewSeparator(),
		container.NewGridWithColumns(2,
			file1Box,
			file2Box,
		),
	)

	return container.NewPadded(content)
}
