package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

const (
	ripFormatLosslessMKV = "Lossless MKV (Copy)"
	ripFormatH264MKV     = "H.264 MKV (CRF 18)"
	ripFormatH264MP4     = "H.264 MP4 (CRF 18)"
)

type ripConfig struct {
	Format string `json:"format"`
}

func defaultRipConfig() ripConfig {
	return ripConfig{
		Format: ripFormatLosslessMKV,
	}
}

func loadPersistedRipConfig() (ripConfig, error) {
	var cfg ripConfig
	path := moduleConfigPath("rip")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Format == "" {
		cfg.Format = ripFormatLosslessMKV
	}
	return cfg, nil
}

func savePersistedRipConfig(cfg ripConfig) error {
	path := moduleConfigPath("rip")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (s *appState) applyRipConfig(cfg ripConfig) {
	s.ripFormat = cfg.Format
}

func (s *appState) persistRipConfig() {
	cfg := ripConfig{
		Format: s.ripFormat,
	}
	if err := savePersistedRipConfig(cfg); err != nil {
		logging.Debug(logging.CatSystem, "failed to persist rip config: %v", err)
	}
}

func (s *appState) showRipView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "rip"
	s.maximizeWindow()

	if cfg, err := loadPersistedRipConfig(); err == nil {
		s.applyRipConfig(cfg)
	} else if !errors.Is(err, os.ErrNotExist) {
		logging.Debug(logging.CatSystem, "failed to load persisted rip config: %v", err)
	}

	if s.ripFormat == "" {
		s.ripFormat = ripFormatLosslessMKV
	}
	if s.ripStatusLabel != nil {
		s.ripStatusLabel.SetText("Ready")
	}
	s.setContent(buildRipView(s))
}

func buildRipView(state *appState) fyne.CanvasObject {
	ripColor := moduleColor("rip")

	backBtn := widget.NewButton("< BACK", func() {
		state.showMainMenu()
	})
	backBtn.Importance = widget.LowImportance

	queueBtn := widget.NewButton("View Queue", func() {
		state.showQueue()
	})
	state.queueBtn = queueBtn
	state.updateQueueButtonLabel()

	clearCompletedBtn := widget.NewButton("⌫", func() {
		state.clearCompletedJobs()
	})
	clearCompletedBtn.Importance = widget.LowImportance

	topBar := ui.TintedBar(ripColor, container.NewHBox(backBtn, layout.NewSpacer(), clearCompletedBtn, queueBtn))
	bottomBar := moduleFooter(ripColor, layout.NewSpacer(), state.statsBar)

	sourceEntry := widget.NewEntry()
	sourceEntry.SetPlaceHolder("Drop DVD/ISO/VIDEO_TS path here")
	sourceEntry.SetText(state.ripSourcePath)
	sourceEntry.OnChanged = func(val string) {
		state.ripSourcePath = strings.TrimSpace(val)
		state.ripOutputPath = defaultRipOutputPath(state.ripSourcePath, state.ripFormat)
	}

	outputEntry := widget.NewEntry()
	outputEntry.SetPlaceHolder("Output path")
	outputEntry.SetText(state.ripOutputPath)
	outputEntry.OnChanged = func(val string) {
		state.ripOutputPath = strings.TrimSpace(val)
	}

	formatSelect := widget.NewSelect([]string{ripFormatLosslessMKV, ripFormatH264MKV, ripFormatH264MP4}, func(val string) {
		state.ripFormat = val
		state.ripOutputPath = defaultRipOutputPath(state.ripSourcePath, state.ripFormat)
		outputEntry.SetText(state.ripOutputPath)
		state.persistRipConfig()
	})
	formatSelect.SetSelected(state.ripFormat)

	statusLabel := widget.NewLabel("Ready")
	statusLabel.Wrapping = fyne.TextWrapWord
	state.ripStatusLabel = statusLabel

	progressBar := widget.NewProgressBar()
	progressBar.SetValue(state.ripProgress / 100.0)
	state.ripProgressBar = progressBar

	logEntry := widget.NewMultiLineEntry()
	logEntry.Wrapping = fyne.TextWrapOff
	logEntry.Disable()
	logEntry.SetText(state.ripLogText)
	state.ripLogEntry = logEntry
	logScroll := container.NewVScroll(logEntry)
	// logScroll.SetMinSize(fyne.NewSize(0, 200)) // Removed for flexible sizing
	state.ripLogScroll = logScroll
	copyLogBtn := widget.NewButton("Copy Log", func() {
		if strings.TrimSpace(state.ripLogText) == "" {
			return
		}
		state.window.Clipboard().SetContent(state.ripLogText)
	})
	copyLogBtn.Importance = widget.LowImportance

	addQueueBtn := widget.NewButton("Add Rip to Queue", func() {
		if err := state.addRipToQueue(false); err != nil {
			dialog.ShowError(err, state.window)
			return
		}
		dialog.ShowInformation("Queue", "Rip job added to queue.", state.window)
		if state.jobQueue != nil && !state.jobQueue.IsRunning() {
			state.jobQueue.Start()
		}
	})
	addQueueBtn.Importance = widget.MediumImportance

	runNowBtn := widget.NewButton("Rip Now", func() {
		if err := state.addRipToQueue(true); err != nil {
			dialog.ShowError(err, state.window)
			return
		}
		if state.jobQueue != nil && !state.jobQueue.IsRunning() {
			state.jobQueue.Start()
		}
		dialog.ShowInformation("Rip", "Rip started! Track progress in Job Queue.", state.window)
	})
	runNowBtn.Importance = widget.HighImportance

	applyControls := func() {
		formatSelect.SetSelected(state.ripFormat)
		outputEntry.SetText(state.ripOutputPath)
	}

	loadCfgBtn := widget.NewButton("Load Config", func() {
		cfg, err := loadPersistedRipConfig()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				dialog.ShowInformation("No Config", "No saved config found yet. It will save automatically after your first change.", state.window)
			} else {
				dialog.ShowError(fmt.Errorf("failed to load config: %w", err), state.window)
			}
			return
		}
		state.applyRipConfig(cfg)
		state.ripOutputPath = defaultRipOutputPath(state.ripSourcePath, state.ripFormat)
		applyControls()
	})

	saveCfgBtn := widget.NewButton("Save Config", func() {
		cfg := ripConfig{
			Format: state.ripFormat,
		}
		if err := savePersistedRipConfig(cfg); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save config: %w", err), state.window)
			return
		}
		dialog.ShowInformation("Config Saved", fmt.Sprintf("Saved to %s", moduleConfigPath("rip")), state.window)
	})

	resetBtn := widget.NewButton("Reset", func() {
		cfg := defaultRipConfig()
		state.applyRipConfig(cfg)
		state.ripOutputPath = defaultRipOutputPath(state.ripSourcePath, state.ripFormat)
		applyControls()
		state.persistRipConfig()
	})

	clearISOBtn := widget.NewButton("Clear ISO", func() {
		state.ripSourcePath = ""
		state.ripOutputPath = ""
		sourceEntry.SetText("")
		outputEntry.SetText("")
	})
	clearISOBtn.Importance = widget.LowImportance

	controls := container.NewVBox(
		widget.NewLabelWithStyle("Source", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		ui.NewDroppable(sourceEntry, func(items []fyne.URI) {
			path := firstLocalPath(items)
			if path != "" {
				state.ripSourcePath = path
				sourceEntry.SetText(path)
				state.ripOutputPath = defaultRipOutputPath(path, state.ripFormat)
				outputEntry.SetText(state.ripOutputPath)
			}
		}),
		clearISOBtn,
		widget.NewLabelWithStyle("Format", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		formatSelect,
		widget.NewLabelWithStyle("Output", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		outputEntry,
		container.NewHBox(addQueueBtn, runNowBtn),
		widget.NewSeparator(),
		container.NewHBox(resetBtn, loadCfgBtn, saveCfgBtn),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Status", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		statusLabel,
		progressBar,
		widget.NewSeparator(),
		container.NewHBox(
			widget.NewLabelWithStyle("Rip Log", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			layout.NewSpacer(),
			copyLogBtn,
		),
		logScroll,
	)

	return container.NewBorder(topBar, bottomBar, nil, nil, container.NewPadded(controls))
}

func (s *appState) addRipToQueue(startNow bool) error {
	if s.jobQueue == nil {
		return fmt.Errorf("queue not initialized")
	}
	if strings.TrimSpace(s.ripSourcePath) == "" {
		return fmt.Errorf("set a DVD/ISO/VIDEO_TS source path")
	}
	if strings.TrimSpace(s.ripOutputPath) == "" {
		s.ripOutputPath = defaultRipOutputPath(s.ripSourcePath, s.ripFormat)
	}
	job := &queue.Job{
		Type:        queue.JobTypeRip,
		Title:       fmt.Sprintf("Rip DVD: %s", filepath.Base(s.ripSourcePath)),
		Description: fmt.Sprintf("Output: %s", utils.ShortenMiddle(filepath.Base(s.ripOutputPath), 40)),
		InputFile:   s.ripSourcePath,
		OutputFile:  s.ripOutputPath,
		Config: map[string]interface{}{
			"sourcePath": s.ripSourcePath,
			"outputPath": s.ripOutputPath,
			"format":     s.ripFormat,
		},
	}
	s.resetRipLog()
	s.setRipStatus("Queued rip job...")
	s.setRipProgress(0)
	s.jobQueue.Add(job)
	if startNow && !s.jobQueue.IsRunning() {
		s.jobQueue.Start()
	}
	return nil
}

func (s *appState) executeRipJob(ctx context.Context, job *queue.Job, progressCallback func(float64)) error {
	cfg := job.Config
	if cfg == nil {
		return fmt.Errorf("rip job config missing")
	}
	sourcePath := toString(cfg["sourcePath"])
	outputPath := toString(cfg["outputPath"])
	format := toString(cfg["format"])
	if sourcePath == "" || outputPath == "" {
		return fmt.Errorf("rip job missing paths")
	}
	logFile, logPath, logErr := createRipLog(sourcePath, outputPath, format)
	if logErr != nil {
		logging.Debug(logging.CatSystem, "rip log open failed: %v", logErr)
	} else {
		job.LogPath = logPath
		defer logFile.Close()
	}

	appendLog := func(line string) {
		if logFile != nil {
			fmt.Fprintln(logFile, line)
		}
		app := fyne.CurrentApp()
		if app != nil && app.Driver() != nil {
			app.Driver().DoFromGoroutine(func() {
				s.appendRipLog(line)
			}, false)
		}
	}
	updateProgress := func(percent float64) {
		progressCallback(percent)
		app := fyne.CurrentApp()
		if app != nil && app.Driver() != nil {
			app.Driver().DoFromGoroutine(func() {
				s.setRipProgress(percent)
			}, false)
		}
	}

	appendLog(fmt.Sprintf("Rip started: %s", time.Now().Format(time.RFC3339)))
	appendLog(fmt.Sprintf("Source: %s", sourcePath))
	appendLog(fmt.Sprintf("Output: %s", outputPath))
	appendLog(fmt.Sprintf("Format: %s", format))

	videoTSPath, cleanup, err := resolveVideoTSPath(sourcePath)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	sets, err := collectVOBSets(videoTSPath)
	if err != nil {
		return err
	}
	if len(sets) == 0 {
		return fmt.Errorf("no VOB files found in VIDEO_TS")
	}

	set := sets[0]
	appendLog(fmt.Sprintf("Using title set: %s", set.Name))
	listFile, err := buildConcatList(set.Files)
	if err != nil {
		return err
	}
	defer os.Remove(listFile)

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	args := buildRipFFmpegArgs(listFile, outputPath, format)
	appendLog(fmt.Sprintf(">> ffmpeg %s", strings.Join(args, " ")))
	updateProgress(10)
	if err := runCommandWithLogger(ctx, utils.GetFFmpegPath(), args, appendLog); err != nil {
		return err
	}
	updateProgress(100)
	appendLog("Rip completed successfully.")
	return nil
}

func defaultRipOutputPath(sourcePath, format string) string {
	if sourcePath == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	baseDir := filepath.Join(home, "Videos", "VideoTools", "DVD_Rips")
	name := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	if strings.EqualFold(name, "video_ts") {
		name = filepath.Base(filepath.Dir(sourcePath))
	}
	name = sanitizeForPath(name)
	if name == "" {
		name = "dvd_rip"
	}
	ext := ".mkv"
	if format == ripFormatH264MP4 {
		ext = ".mp4"
	}
	return uniqueFilePath(filepath.Join(baseDir, name+ext))
}

func createRipLog(inputPath, outputPath, format string) (*os.File, string, error) {
	base := strings.TrimSuffix(filepath.Base(outputPath), filepath.Ext(outputPath))
	if base == "" {
		base = "rip"
	}
	logPath := filepath.Join(getLogsDir(), base+"-rip"+conversionLogSuffix)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, logPath, fmt.Errorf("create log dir: %w", err)
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, logPath, err
	}
	header := fmt.Sprintf(`VideoTools Rip Log
Started: %s
Source: %s
Output: %s
Format: %s

`, time.Now().Format(time.RFC3339), inputPath, outputPath, format)
	if _, err := f.WriteString(header); err != nil {
		_ = f.Close()
		return nil, logPath, err
	}
	return f, logPath, nil
}

func resolveVideoTSPath(path string) (string, func(), error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", nil, fmt.Errorf("source not found: %w", err)
	}
	if info.IsDir() {
		if strings.EqualFold(filepath.Base(path), "VIDEO_TS") {
			return path, nil, nil
		}
		videoTS := filepath.Join(path, "VIDEO_TS")
		if info, err := os.Stat(videoTS); err == nil && info.IsDir() {
			return videoTS, nil, nil
		}
		return "", nil, fmt.Errorf("no VIDEO_TS folder found in %s", path)
	}
	if strings.HasSuffix(strings.ToLower(path), ".iso") {
		// Try mount-based extraction first (works for UDF ISOs)
		videoTS, cleanup, err := tryMountISO(path)
		if err == nil {
			return videoTS, cleanup, nil
		}

		// Fall back to extraction tools
		tempDir, err := os.MkdirTemp(utils.TempDir(), "videotools-iso-")
		if err != nil {
			return "", nil, fmt.Errorf("failed to create temp dir: %w", err)
		}
		cleanup = func() {
			_ = os.RemoveAll(tempDir)
		}
		tool, args, err := buildISOExtractCommand(path, tempDir)
		if err != nil {
			cleanup()
			return "", nil, err
		}
		if err := runCommandWithLogger(context.Background(), tool, args, nil); err != nil {
			cleanup()
			return "", nil, err
		}
		videoTS = filepath.Join(tempDir, "VIDEO_TS")
		if info, err := os.Stat(videoTS); err == nil && info.IsDir() {
			return videoTS, cleanup, nil
		}
		cleanup()
		return "", nil, fmt.Errorf("VIDEO_TS not found in ISO")
	}
	return "", nil, fmt.Errorf("unsupported source: %s", path)
}

// tryMountISO attempts to mount the ISO and copy VIDEO_TS to a temp directory
func tryMountISO(isoPath string) (string, func(), error) {
	// Create mount point
	mountPoint, err := os.MkdirTemp(utils.TempDir(), "videotools-mount-")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create mount point: %w", err)
	}

	// Try to mount the ISO
	mountCmd := exec.Command("mount", "-o", "loop,ro", isoPath, mountPoint)
	if err := mountCmd.Run(); err != nil {
		os.RemoveAll(mountPoint)
		return "", nil, fmt.Errorf("mount failed: %w", err)
	}

	// Check if VIDEO_TS exists
	videoTSMounted := filepath.Join(mountPoint, "VIDEO_TS")
	if info, err := os.Stat(videoTSMounted); err != nil || !info.IsDir() {
		exec.Command("umount", mountPoint).Run()
		os.RemoveAll(mountPoint)
		return "", nil, fmt.Errorf("VIDEO_TS not found in mounted ISO")
	}

	// Copy VIDEO_TS to temp directory
	tempDir, err := os.MkdirTemp(utils.TempDir(), "videotools-iso-")
	if err != nil {
		exec.Command("umount", mountPoint).Run()
		os.RemoveAll(mountPoint)
		return "", nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Use cp to copy VIDEO_TS
	cpCmd := exec.Command("cp", "-r", videoTSMounted, tempDir)
	if err := cpCmd.Run(); err != nil {
		exec.Command("umount", mountPoint).Run()
		os.RemoveAll(mountPoint)
		os.RemoveAll(tempDir)
		return "", nil, fmt.Errorf("copy failed: %w", err)
	}

	// Unmount and clean up mount point
	exec.Command("umount", mountPoint).Run()
	os.RemoveAll(mountPoint)

	// Return path to copied VIDEO_TS
	videoTS := filepath.Join(tempDir, "VIDEO_TS")
	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}

	return videoTS, cleanup, nil
}

func buildISOExtractCommand(isoPath, destDir string) (string, []string, error) {
	// Try xorriso first (best for UDF and ISO9660)
	if _, err := exec.LookPath("xorriso"); err == nil {
		return "xorriso", []string{"-osirrox", "on", "-indev", isoPath, "-extract", "/VIDEO_TS", destDir}, nil
	}

	// Try 7z (works well with both UDF and ISO9660)
	if _, err := exec.LookPath("7z"); err == nil {
		return "7z", []string{"x", "-o" + destDir, isoPath, "VIDEO_TS"}, nil
	}

	// Try bsdtar (works with ISO9660, may fail on UDF)
	if _, err := exec.LookPath("bsdtar"); err == nil {
		return "bsdtar", []string{"-C", destDir, "-xf", isoPath, "VIDEO_TS"}, nil
	}

	return "", nil, fmt.Errorf("no ISO extraction tool found (install xorriso, 7z, or bsdtar)")
}

type vobSet struct {
	Name  string
	Files []string
	Size  int64
}

func collectVOBSets(videoTS string) ([]vobSet, error) {
	entries, err := os.ReadDir(videoTS)
	if err != nil {
		return nil, fmt.Errorf("read VIDEO_TS: %w", err)
	}
	sets := map[string]*vobSet{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(name), ".vob") {
			continue
		}
		if !strings.HasPrefix(strings.ToUpper(name), "VTS_") {
			continue
		}
		parts := strings.Split(strings.TrimSuffix(name, ".VOB"), "_")
		if len(parts) < 3 {
			continue
		}
		setKey := strings.Join(parts[:2], "_")
		if sets[setKey] == nil {
			sets[setKey] = &vobSet{Name: setKey}
		}
		full := filepath.Join(videoTS, name)
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		sets[setKey].Files = append(sets[setKey].Files, full)
		sets[setKey].Size += info.Size()
	}
	var result []vobSet
	for _, set := range sets {
		sort.Strings(set.Files)
		result = append(result, *set)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Size > result[j].Size
	})
	return result, nil
}

func buildConcatList(files []string) (string, error) {
	if len(files) == 0 {
		return "", fmt.Errorf("no VOB files to concatenate")
	}
	listFile, err := os.CreateTemp(utils.TempDir(), "vt-rip-list-*.txt")
	if err != nil {
		return "", err
	}
	writer := bufio.NewWriter(listFile)
	for _, f := range files {
		fmt.Fprintf(writer, "file '%s'\n", strings.ReplaceAll(f, "'", "'\\''"))
	}
	_ = writer.Flush()
	_ = listFile.Close()
	return listFile.Name(), nil
}

func buildRipFFmpegArgs(listFile, outputPath, format string) []string {
	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-f", "concat",
		"-safe", "0",
		"-i", listFile,
	}
	switch format {
	case ripFormatH264MKV:
		args = append(args,
			"-c:v", "libx264",
			"-crf", "18",
			"-preset", "medium",
			"-c:a", "copy",
		)
	case ripFormatH264MP4:
		args = append(args,
			"-c:v", "libx264",
			"-crf", "18",
			"-preset", "medium",
			"-c:a", "aac",
			"-b:a", "192k",
		)
	default:
		args = append(args, "-c", "copy")
	}
	args = append(args, outputPath)
	return args
}

func firstLocalPath(items []fyne.URI) string {
	for _, uri := range items {
		if uri.Scheme() == "file" {
			return uri.Path()
		}
	}
	return ""
}

func (s *appState) resetRipLog() {
	s.ripLogText = ""
	if s.ripLogEntry != nil {
		s.ripLogEntry.SetText("")
	}
	if s.ripLogScroll != nil {
		s.ripLogScroll.ScrollToTop()
	}
}

func (s *appState) appendRipLog(line string) {
	if strings.TrimSpace(line) == "" {
		return
	}
	s.ripLogText += line + "\n"
	if s.ripLogEntry != nil {
		s.ripLogEntry.SetText(s.ripLogText)
	}
	if s.ripLogScroll != nil {
		s.ripLogScroll.ScrollToBottom()
	}
}

func (s *appState) setRipStatus(text string) {
	if text == "" {
		text = "Ready"
	}
	if s.ripStatusLabel != nil {
		s.ripStatusLabel.SetText(text)
	}
}

func (s *appState) setRipProgress(percent float64) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	s.ripProgress = percent
	if s.ripProgressBar != nil {
		s.ripProgressBar.SetValue(percent / 100.0)
	}
}
