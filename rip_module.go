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
	"git.leaktechnologies.dev/stu/VideoTools/internal/app/configpath"
	"git.leaktechnologies.dev/stu/VideoTools/internal/app/modulecfg"
	"git.leaktechnologies.dev/stu/VideoTools/internal/dvd/ifo"
	"git.leaktechnologies.dev/stu/VideoTools/internal/dvd/udf"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

const (
	ripFormatLosslessMKV = "Lossless MKV (Copy)"
	ripFormatH264MKV     = "H.264 MKV (CRF 18)"
	ripFormatH264MP4     = "H.264 MP4 (CRF 18)"
	ripFormatArchivist   = "Archivist (Reconstructible Project)"
)

type ripConfig = modulecfg.RipConfig

func defaultRipConfig() ripConfig {
	return modulecfg.DefaultRipConfig()
}

func loadPersistedRipConfig() (ripConfig, error) {
	return modulecfg.LoadRipConfig()
}

func savePersistedRipConfig(cfg ripConfig) error {
	return modulecfg.SaveRipConfig(cfg)
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
		s.ripStatusLabel.SetText(i18n.T().StatusReady)
	}
	s.setContent(buildRipView(s))
}

func buildRipView(state *appState) fyne.CanvasObject {
	ripColor := moduleColor("rip")
	t := i18n.T()

	backBtn := widget.NewButton("< "+strings.ToUpper(t.ModuleRip), func() {
		state.showMainMenu()
	})
	backBtn.Importance = widget.LowImportance

	queueBtn := widget.NewButton(t.ActionViewQueue, func() {
		state.showQueue()
	})
	state.queueBtn = queueBtn
	state.updateQueueButtonLabel()

	clearCompletedBtn := widget.NewButton("⌫", func() {
		state.clearCompletedJobs()
	})
	clearCompletedBtn.Importance = widget.LowImportance

	topBar := ui.TintedBar(ripColor, container.NewHBox(backBtn, layout.NewSpacer(), clearCompletedBtn, queueBtn))

	sourceEntry := widget.NewEntry()
	sourceEntry.SetPlaceHolder(t.RipDropPrompt)
	sourceEntry.SetText(state.ripSourcePath)
	sourceEntry.OnChanged = func(val string) {
		state.ripSourcePath = strings.TrimSpace(val)
		state.ripOutputPath = defaultRipOutputPath(state.ripSourcePath, state.ripFormat)
	}

	outputEntry := widget.NewEntry()
	outputEntry.SetPlaceHolder(t.RipOutputPath)
	outputEntry.SetText(state.ripOutputPath)
	outputEntry.OnChanged = func(val string) {
		state.ripOutputPath = strings.TrimSpace(val)
	}

	formatSelect := widget.NewSelect([]string{ripFormatLosslessMKV, ripFormatH264MKV, ripFormatH264MP4, ripFormatArchivist}, func(value string) {
		state.ripFormat = value
		state.ripOutputPath = defaultRipOutputPath(state.ripSourcePath, value)
		outputEntry.SetText(state.ripOutputPath)
		state.persistRipConfig()
	})
	formatSelect.SetSelected(state.ripFormat)

	statusLabel := widget.NewLabel(t.StatusReady)
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
	copyLogBtn := widget.NewButton(t.ActionCopyLog, func() {
		if strings.TrimSpace(state.ripLogText) == "" {
			return
		}
		state.window.Clipboard().SetContent(state.ripLogText)
	})
	copyLogBtn.Importance = widget.LowImportance

	addQueueBtn := widget.NewButton(t.RipAddToQueue, func() {
		if err := state.addRipToQueue(false); err != nil {
			dialog.ShowError(err, state.window)
			return
		}
		dialog.ShowInformation(t.RipJobQueuedTitle, t.RipJobQueuedMsg, state.window)
		if state.jobQueue != nil && !state.jobQueue.IsRunning() {
			state.jobQueue.Start()
		}
	})
	addQueueBtn.Importance = widget.MediumImportance

	runNowBtn := widget.NewButton(t.RipNow, func() {
		if err := state.addRipToQueue(true); err != nil {
			dialog.ShowError(err, state.window)
			return
		}
		if state.jobQueue != nil && !state.jobQueue.IsRunning() {
			state.jobQueue.Start()
		}
		dialog.ShowInformation(t.RipStartTitle, t.RipStartMsg, state.window)
	})
	runNowBtn.Importance = widget.HighImportance

	applyControls := func() {
		formatSelect.SetSelected(state.ripFormat)
		outputEntry.SetText(state.ripOutputPath)
	}

	loadCfgBtn := widget.NewButton(t.ActionLoadConfig, func() {
		cfg, err := loadPersistedRipConfig()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				dialog.ShowInformation(t.RipNoConfigTitle, t.RipNoConfigMsg, state.window)
			} else {
				dialog.ShowError(fmt.Errorf("failed to load config: %w", err), state.window)
			}
			return
		}
		state.applyRipConfig(cfg)
		state.ripOutputPath = defaultRipOutputPath(state.ripSourcePath, state.ripFormat)
		applyControls()
	})

	saveCfgBtn := widget.NewButton(t.ActionSaveConfig, func() {
		cfg := ripConfig{
			Format: state.ripFormat,
		}
		if err := savePersistedRipConfig(cfg); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save config: %w", err), state.window)
			return
		}
		dialog.ShowInformation(t.RipConfigSavedTitle, fmt.Sprintf(t.RipConfigSavedFmt, configpath.ModuleConfigPath("rip")), state.window)
	})

	resetBtn := widget.NewButton(t.ActionReset, func() {
		cfg := defaultRipConfig()
		state.applyRipConfig(cfg)
		state.ripOutputPath = defaultRipOutputPath(state.ripSourcePath, state.ripFormat)
		applyControls()
		state.persistRipConfig()
	})

	clearISOBtn := widget.NewButton(t.RipClearISO, func() {
		state.ripSourcePath = ""
		state.ripOutputPath = ""
		sourceEntry.SetText("")
		outputEntry.SetText("")
	})
	clearISOBtn.Importance = widget.LowImportance

	controls := container.NewVBox(
		widget.NewLabelWithStyle(t.RipSource, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		ui.NewDroppable(sourceEntry, func(items []fyne.URI) {
			path := firstLocalPath(items)
			if path != "" {
				state.ripSourcePath = path
				sourceEntry.SetText(path)

				// Dynamic detection for ISO files
				if strings.HasSuffix(strings.ToLower(path), ".iso") {
					if discType, err := udf.IdentifyDiscFormat(path); err == nil {
						logging.Info(logging.CatDVD, "User dropped ISO: detected as %s", discType)
					}
				} else {
					// Check if it's a VIDEO_TS folder
					if info, err := os.Stat(filepath.Join(path, "VIDEO_TS.IFO")); err == nil && !info.IsDir() {
						state.scanDVDStructure(path)
					}
				}

				state.ripOutputPath = defaultRipOutputPath(path, state.ripFormat)
				outputEntry.SetText(state.ripOutputPath)
			}
		}),
		clearISOBtn,
		widget.NewLabelWithStyle(t.RipFormatLabel, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		formatSelect,
		widget.NewLabelWithStyle(t.LabelOutput, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		outputEntry,
		container.NewHBox(resetBtn, loadCfgBtn, saveCfgBtn),
		widget.NewSeparator(),
		widget.NewLabelWithStyle(t.LabelStatus, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		statusLabel,
		progressBar,
		widget.NewSeparator(),
		container.NewHBox(
			widget.NewLabelWithStyle(t.RipLog, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			layout.NewSpacer(),
			copyLogBtn,
		),
		logScroll,
	)

	bottomBar := moduleFooter(ripColor, container.NewHBox(addQueueBtn, layout.NewSpacer(), runNowBtn), state.statsBar)
	return container.NewBorder(topBar, bottomBar, nil, nil, container.NewVScroll(container.NewPadded(controls)))
}

func (s *appState) scanDVDStructure(path string) error {
	vmgPath := filepath.Join(path, "VIDEO_TS.IFO")
	f, err := os.Open(vmgPath)
	if err != nil {
		return fmt.Errorf("open VIDEO_TS.IFO: %w", err)
	}
	defer f.Close()

	vmg, err := ifo.ReadVMGI(f)
	if err != nil {
		return fmt.Errorf("read VMGI: %w", err)
	}

	logging.Info(logging.CatDVD, "DVD Scan: Found %d title sets", vmg.NrOfTitleSets)

	// We'll populate a list of titles for the user to select
	// [TODO: Update UI with title list]

	return nil
}

func (s *appState) addRipToQueue(runNow bool) error {
	if s.jobQueue == nil {
		return fmt.Errorf("queue not initialized")
	}
	if strings.TrimSpace(s.ripSourcePath) == "" {
		return fmt.Errorf("%s", i18n.T().RipErrNoSource)
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
	if runNow && !s.jobQueue.IsRunning() {
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
	outputDir := outputPath
	if format != ripFormatArchivist {
		outputDir = filepath.Dir(outputPath)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if format == ripFormatArchivist {
		appendLog("Archivist Mode: Extracting individual streams for reconstruction...")
		src, err := probeVideo(set.Files[0])
		if err != nil {
			return fmt.Errorf("probe for archivist failed: %w", err)
		}

		args := []string{"-y", "-hide_banner", "-loglevel", "error", "-f", "concat", "-safe", "0", "-i", listFile}

		// Map Video
		args = append(args, "-map", "0:v:0", "-c:v", "copy", filepath.Join(outputDir, "video.m2v"))

		// Map all Audio
		for i, at := range src.Audio {
			args = append(args, "-map", fmt.Sprintf("0:%d", at.Index), "-c:a", "copy", filepath.Join(outputDir, fmt.Sprintf("audio_%d_%s.ac3", i, at.Language)))
		}

		// Map all Subtitles
		for i, st := range src.Subtitles {
			args = append(args, "-map", fmt.Sprintf("0:%d", st.Index), "-c:s", "copy", filepath.Join(outputDir, fmt.Sprintf("subs_%d_%s.sup", i, st.Language)))
		}

		appendLog(fmt.Sprintf(">> ffmpeg %s", strings.Join(args, " ")))
		updateProgress(20)
		if err := runCommandWithLogger(ctx, utils.GetFFmpegPath(), args, appendLog); err != nil {
			return err
		}

		// Create project file
		projPath := filepath.Join(outputDir, "author_project.json")
		appendLog(fmt.Sprintf("Creating project file: %s", projPath))

		project := map[string]interface{}{
			"title": filepath.Base(outputDir),
			"type":  "dvd", // DVD by default for now
			"assets": []map[string]interface{}{
				{
					"path": "video.m2v",
					"type": "feature",
				},
			},
		}

		projData, _ := json.MarshalIndent(project, "", "  ")
		_ = os.WriteFile(projPath, projData, 0644)

		updateProgress(100)
		appendLog("Archivist extraction completed successfully.")
		return nil
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
		logging.Info(logging.CatDVD, "Using native Go UDF reader for extraction: %s", path)

		tempDir, err := os.MkdirTemp(utils.TempDir(), "videotools-iso-")
		if err != nil {
			return "", nil, fmt.Errorf("failed to create temp dir: %w", err)
		}
		var cleanup func()
		cleanup = func() {
			_ = os.RemoveAll(tempDir)
		}

		f, err := os.Open(path)
		if err != nil {
			cleanup()
			return "", nil, err
		}
		defer f.Close()

		reader := udf.NewReader(f)

		// Determine target directory (VIDEO_TS for DVD, BDMV for Blu-ray)
		targetDir := "VIDEO_TS"
		discType, err := reader.DetectDiscType()
		if err == nil && discType == udf.DiscTypeBluRay {
			targetDir = "BDMV"
		}

		if err := reader.ExtractDirectory(targetDir, tempDir); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("native extraction failed: %w", err)
		}

		videoTS := filepath.Join(tempDir, targetDir)
		if info, err := os.Stat(videoTS); err == nil && info.IsDir() {
			return videoTS, cleanup, nil
		}
		cleanup()
		return "", nil, fmt.Errorf("%s not found in ISO", targetDir)
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
		text = i18n.T().StatusReady
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
