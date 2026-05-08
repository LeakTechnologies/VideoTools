package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"

	thumbpkg "git.leaktechnologies.dev/stu/VideoTools/internal/app/modules/thumbnail"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	thumbsvc "git.leaktechnologies.dev/stu/VideoTools/internal/thumbnail"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

func init() {
	// Load logo PNG from embedded assets for contact-sheet headers.
	if f, err := logoAssets.Open("assets/logo/VT_logo.png"); err == nil {
		if data, err2 := io.ReadAll(f); err2 == nil {
			thumbsvc.SetLogoData(data)
		}
		f.Close()
	}
	thumbsvc.SetFontData(ibmPlexMonoRegular)
	thumbsvc.SetBoldFontData(ibmPlexMonoBold)
}

// clearThumbnailLiveGrid resets the live preview grid before a new job starts.
func (s *appState) clearThumbnailLiveGrid() {
	if s.thumbnailLiveGrid == nil {
		s.thumbnailLiveGrid = container.NewGridWrap(fyne.NewSize(160, 100))
	} else {
		s.thumbnailLiveGrid.Layout = layout.NewGridWrapLayout(fyne.NewSize(160, 100))
		s.thumbnailLiveGrid.Objects = []fyne.CanvasObject{}
		s.thumbnailLiveGrid.Refresh()
	}
}

// setThumbnailLiveContactSheet replaces the live preview with a single full-panel
// contact sheet image. Uses MaxLayout so the image expands to fill the panel.
// Clicking the image opens it in a full-window inspector.
func (s *appState) setThumbnailLiveContactSheet(path string) {
	if s.thumbnailLiveGrid == nil {
		s.thumbnailLiveGrid = container.NewGridWrap(fyne.NewSize(160, 100))
	}
	img := canvas.NewImageFromFile(path)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(300, 200))
	tappable := ui.NewTappable(img, func() { s.showImageInspector(path) })
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		s.thumbnailLiveGrid.Layout = layout.NewMaxLayout()
		s.thumbnailLiveGrid.Objects = []fyne.CanvasObject{tappable}
		s.thumbnailLiveGrid.Refresh()
	}, false)
}

// showImageInspector opens a near-full-window dialog so the user can inspect a
// generated thumbnail or contact sheet at full size. Clicking the image closes it.
// Safe to call from any goroutine.
func (s *appState) showImageInspector(path string) {
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		winSize := s.window.Canvas().Size()
		img := canvas.NewImageFromFile(path)
		img.FillMode = canvas.ImageFillContain
		img.SetMinSize(fyne.NewSize(winSize.Width-80, winSize.Height-120))

		d := dialog.NewCustom("", "Close", img, s.window)

		// Click the image to close the dialog
		tappable := ui.NewTappable(img, func() {
			d.Hide()
		})

		// Replace content with tappable wrapper
		d.Hide()
		d = dialog.NewCustom("", "Close", tappable, s.window)
		d.Resize(fyne.NewSize(winSize.Width-40, winSize.Height-60))
		d.Show()
	}, false)
}

// addThumbnailLivePreview adds a single generated thumbnail to the live preview
// grid. Clicking the thumbnail opens it in a full-window inspector. Safe to call
// from any goroutine.
func (s *appState) addThumbnailLivePreview(path string) {
	if s.thumbnailLiveGrid == nil {
		s.thumbnailLiveGrid = container.NewGridWrap(fyne.NewSize(160, 100))
	}
	img := canvas.NewImageFromFile(path)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(160, 100))
	tappable := ui.NewTappable(img, func() { s.showImageInspector(path) })
	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		s.thumbnailLiveGrid.Add(tappable)
		s.thumbnailLiveGrid.Refresh()
	}, false)
}

func (s *appState) showThumbnailView() {
	defer func() {
		if r := recover(); r != nil {
			logging.Crash(logging.CatModule, "panic in showThumbnailView: %v", r)
		}
	}()
	logging.Debug(logging.CatModule, "showThumbnailView: start")
	start := time.Now()
	s.stopPreview()
	entering := s.active != "thumbnail"
	s.lastModule = s.active
	s.active = "thumbnail"
	s.maximizeWindow()
	elapsed := time.Since(start)
	logging.Debug(logging.CatModule, "showThumbnailView: pre-config %v", elapsed)
	start = time.Now()
	// Only load persisted config when navigating into the module from elsewhere.
	// Internal refreshes (output mode change, file selection, etc.) already have
	// correct in-memory state; reading disk every rebuild adds unnecessary I/O.
	if entering {
		if cfg, err := loadPersistedThumbnailConfig(); err == nil {
			s.applyThumbnailConfig(cfg)
		}
	}
	elapsed = time.Since(start)
	logging.Debug(logging.CatModule, "showThumbnailView: config loaded %v", elapsed)
	start = time.Now()
	s.setContent(buildThumbnailView(s))
	elapsed = time.Since(start)
	logging.Debug(logging.CatModule, "showThumbnailView: done %v", elapsed)
}

func (s *appState) addThumbnailSource(src *videoSource) {
	if src == nil {
		return
	}
	for _, existing := range s.thumbnailFiles {
		if existing != nil && existing.Path == src.Path {
			return
		}
	}
	s.thumbnailFiles = append(s.thumbnailFiles, src)
}

func (s *appState) loadThumbnailSourceAtIndex(idx int) {
	if idx < 0 || idx >= len(s.thumbnailFiles) {
		return
	}
	current := s.thumbnailFiles[idx]
	if current == nil || current.Path == "" {
		return
	}
	if current.Width > 0 || current.Height > 0 || current.Duration > 0 {
		return
	}
	path := current.Path
	go func() {
		probed, err := probeVideo(path)
		if err != nil {
			logging.Debug(logging.CatModule, "failed to probe thumbnail source: %v", err)
			return
		}
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			s.thumbnailFiles[idx] = probed
			if s.thumbnailFile != nil && s.thumbnailFile.Path == path {
				s.thumbnailFile = probed
			}
			s.showThumbnailView()
		}, false)
	}()
}

func (s *appState) loadMultipleThumbnailVideos(paths []string) {
	if len(paths) == 0 {
		return
	}
	logging.Debug(logging.CatModule, "loading %d videos into thumbnails", len(paths))

	var valid []*videoSource
	var failed []string
	for _, path := range paths {
		src, err := probeVideo(path)
		if err != nil {
			logging.Debug(logging.CatFFMPEG, "ffprobe failed for %s: %v", path, err)
			failed = append(failed, filepath.Base(path))
			continue
		}
		valid = append(valid, src)
	}

	if len(valid) == 0 {
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			msg := fmt.Sprintf("Failed to analyze %d file(s):\n%s", len(failed), strings.Join(failed, ", "))
			s.showErrorWithCopy("Load Failed", fmt.Errorf("%s", msg))
		}, false)
		return
	}

	s.thumbnailFiles = valid
	s.thumbnailFile = valid[0]

	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		s.showThumbnailView()
		if len(failed) > 0 {
			logging.Debug(logging.CatModule, "%d file(s) failed to analyze: %s", len(failed), strings.Join(failed, ", "))
		}
	}, false)
}

func buildThumbnailView(state *appState) fyne.CanvasObject {
	thumbFiles := make([]any, len(state.thumbnailFiles))
	thumbFilePaths := make([]string, len(state.thumbnailFiles))
	for i, f := range state.thumbnailFiles {
		thumbFiles[i] = f
		if f != nil {
			thumbFilePaths[i] = f.Path
		}
	}

	t := i18n.T()

	var thumbFileName string
	var thumbFileNames []string
	var thumbPreviewFrame string
	if state.thumbnailFile != nil {
		thumbFileName = filepath.Base(state.thumbnailFile.Path)
		if len(state.thumbnailFile.PreviewFrames) > 0 {
			thumbPreviewFrame = state.thumbnailFile.PreviewFrames[0]
		}
	}
	for _, f := range state.thumbnailFiles {
		if f != nil {
			thumbFileNames = append(thumbFileNames, filepath.Base(f.Path))
		} else {
			thumbFileNames = append(thumbFileNames, "")
		}
	}

	// Assign live grid only when non-nil. Assigning a nil *fyne.Container to a
	// fyne.CanvasObject interface produces a non-nil interface (typed nil), which
	// fools the nil check in BuildView and causes a hard crash when Fyne tries to
	// call methods on the underlying nil pointer.
	var liveGrid fyne.CanvasObject
	if state.thumbnailLiveGrid != nil {
		liveGrid = state.thumbnailLiveGrid
	}

	opts := thumbpkg.Options{
		Window:                  state.window,
		ModuleColor:             moduleColor("thumbnail"),
		LivePreviewGrid:         liveGrid,
		ThumbnailFile:           state.thumbnailFile,
		ThumbnailFiles:          thumbFiles,
		ThumbnailFilePaths:      thumbFilePaths,
		ThumbnailFileName:       thumbFileName,
		ThumbnailFileNames:      thumbFileNames,
		ThumbnailPreviewFrame:   thumbPreviewFrame,
		ThumbnailCount:          state.thumbnailCount,
		ThumbnailWidth:          state.thumbnailWidth,
		ThumbnailSheetWidth:     state.thumbnailSheetWidth,
		ThumbnailColumns:        state.thumbnailColumns,
		ThumbnailRows:           state.thumbnailRows,
		ThumbnailOutputMode:     state.thumbnailOutputMode,
		ThumbnailContactSheet:   state.thumbnailOutputMode == "contactSheet" || state.thumbnailOutputMode == "both",
		ThumbnailShowTimestamps: state.thumbnailShowTimestamps,
		OnShowMainMenu:          func() { state.showMainMenu() },
		OnShowQueue:             func() { state.showQueue() },
		OnShowThumbnailView:     func() { state.showThumbnailView() },
		OnClearCompletedJobs:    func() { state.clearCompletedJobs() },
		OnGetStatsBar: func() fyne.CanvasObject {
				if state.statsBar == nil {
					return nil
				}
				return state.statsBar
			},
		OnLoadFile: func(path string) {
			// probeVideo shells out to ffprobe and must not run on the UI goroutine.
			go func() {
				src, err := probeVideo(path)
				if err != nil {
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						dialog.ShowError(fmt.Errorf("failed to load video: %w", err), state.window)
					}, false)
					return
				}
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					state.thumbnailFile = src
					state.addThumbnailSource(src)
					state.showThumbnailView()
					logging.Debug(logging.CatModule, "loaded thumbnail file: %s", path)
				}, false)
				if len(src.PreviewFrames) == 0 {
					if frames, ferr := capturePreviewFrames(path, src.Duration); ferr == nil && len(frames) > 0 {
						src.PreviewFrames = frames
						fyne.CurrentApp().Driver().DoFromGoroutine(func() {
							state.showThumbnailView()
						}, false)
					}
				}
			}()
		},
		OnClearFiles: func() {
			state.thumbnailFile = nil
			state.thumbnailFiles = nil
		},
		OnAddThumbnailSource: func(src any) {
			state.addThumbnailSource(src.(*videoSource))
		},
		OnSetThumbnailCount:          func(i int) { state.thumbnailCount = i },
		OnSetThumbnailWidth:          func(i int) { state.thumbnailWidth = i },
		OnSetThumbnailSheetWidth:     func(i int) { state.thumbnailSheetWidth = i },
		OnSetThumbnailColumns:        func(i int) { state.thumbnailColumns = i },
		OnSetThumbnailRows:           func(i int) { state.thumbnailRows = i },
		OnSetThumbnailOutputMode:     func(mode string) { state.thumbnailOutputMode = mode },
		OnSetThumbnailContactSheet:   func(b bool) {},
		OnSetThumbnailShowTimestamps: func(b bool) { state.thumbnailShowTimestamps = b },
		OnCreateThumbJob: func() {
			if state.thumbnailFile == nil {
				return
			}
			job := state.createThumbnailJobForPath(state.thumbnailFile.Path)
			state.generateJobThumbnail(job)
			state.pipelineAdd(job)
			if !state.jobQueue.IsRunning() {
				state.jobQueue.Start()
			}
		},
		OnCreateThumbJobForPath: func(path string) {
			job := state.createThumbnailJobForPath(path)
			state.generateJobThumbnail(job)
			state.jobQueue.Add(job)
			if !state.jobQueue.IsRunning() {
				state.jobQueue.Start()
			}
		},
		OnSelectThumbnailFile: func(id int) {
			if id >= 0 && id < len(state.thumbnailFiles) {
				state.thumbnailFile = state.thumbnailFiles[id]
				state.showThumbnailView()
			}
		},
		OnPersistConfig: func() { state.persistThumbnailConfig() },

		// Labels
		BackLabel:               "< " + strings.ToUpper(t.ModuleThumbnail),
		ViewQueueLabel:          t.MenuQueue,
		InstructionsLabel:       t.ThumbnailInstructions,
		NoFileLabel:             t.ThumbnailNoFile,
		FileLoadedLabel:         t.ThumbnailFileLoaded,
		LoadVideoLabel:          t.ThumbnailLoadVideo,
		ClearLabel:              t.ActionClear,
		ContactSheetToggleLabel: t.ThumbnailContactSheetToggle,
		ShowTimestampsLabel:     t.ThumbnailShowTimestamps,
		ContactSheetGridLabel:   t.ThumbnailContactSheetGrid,
		IndividualThumbsLabel:   t.ThumbnailIndividual,
		ThumbnailSizeLabel:      t.ThumbnailSize,
		OutputModeLabel:         t.ThumbnailOutputMode,
		ModeIndividualLabel:     t.ThumbnailModeIndividual,
		ModeContactSheetLabel:   t.ThumbnailModeContactSheet,
		ModeBothLabel:           t.ThumbnailModeBoth,
		ColumnsFmt:              t.ThumbnailColumnsFmt,
		RowsFmt:                 t.ThumbnailRowsFmt,
		TotalFmt:                t.ThumbnailTotalFmt,
		CountFmt:                t.ThumbnailCountFmt,
		WidthFmt:                t.ThumbnailWidthFmt,
		GenerateNowLabel:        t.ThumbnailGenerateNow,
		AddToQueueLabel:         t.ThumbnailAddToQueue,
		AddAllToQueueLabel:      t.ThumbnailAddAllToQueue,
		LoadedVideosLabel:       t.ThumbnailLoadedVideos,
		VideoFmt:                t.ThumbnailVideoFmt,
		NoVideoTitle:            t.ThumbnailNoVideoTitle,
		NoVideoMsg:              t.ThumbnailNoVideoMsg,
		StartedTitle:            t.ThumbnailStartedTitle,
		StartedMsg:              t.ThumbnailStartedMsg,
		JobQueuedTitle:          t.ThumbnailJobQueuedTitle,
		JobQueuedMsg:            t.ThumbnailJobQueuedMsg,
		NoVideosTitle:           t.ThumbnailNoVideosTitle,
		NoVideosMsg:             t.ThumbnailNoVideosMsg,
		JobsQueuedFmt:           t.ThumbnailJobsQueuedFmt,
	}
	return thumbpkg.BuildView(opts)
}

func (s *appState) executeThumbnailJob(ctx context.Context, job *queue.Job, progressCallback func(float64)) error {
	cfg := job.Config
	inputPath := cfg["inputPath"].(string)
	outputDir := cfg["outputDir"].(string)
	count := int(cfg["count"].(float64))
	width := int(cfg["width"].(float64))
	outputMode := "individual"
	if raw, ok := cfg["outputMode"]; ok {
		if v, ok := raw.(string); ok {
			outputMode = v
		}
	}
	showTimestamp := false
	if raw, ok := cfg["showTimestamp"]; ok {
		if v, ok := raw.(bool); ok {
			showTimestamp = v
		}
	}
	columns := int(cfg["columns"].(float64))
	rows := int(cfg["rows"].(float64))

	if progressCallback != nil {
		progressCallback(0)
	}

	// Reset the live preview grid so the user sees only the current job's output.
	s.clearThumbnailLiveGrid()

	totalThumbs := count
	if outputMode == "contactSheet" || outputMode == "both" {
		totalThumbs = columns * rows
	}
	perThumb := 20 * time.Second
	timeout := time.Duration(totalThumbs) * perThumb
	if timeout < 2*time.Minute {
		timeout = 2 * time.Minute
	} else if timeout > 20*time.Minute {
		timeout = 20 * time.Minute
	}
	jobCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	generator := thumbsvc.NewGenerator(utils.GetFFmpegPath())

	// Create log file for thumbnail job
	logDir := outputDir
	if outputMode == "contactSheet" || outputMode == "both" {
		logDir = filepath.Dir(outputDir)
	}
	logPath := filepath.Join(logDir, fmt.Sprintf("thumbnail_%s.log", time.Now().Format("20060102_150405")))
	job.LogPath = logPath

	config := thumbsvc.Config{
		VideoPath:     inputPath,
		OutputDir:     outputDir,
		Count:         count,
		Width:         width,
		Format:        "jpg",
		Quality:       85,
		OutputMode:    outputMode,
		Columns:       columns,
		Rows:          rows,
		ShowTimestamp: showTimestamp,
		ShowMetadata:  outputMode == "contactSheet" || outputMode == "both",
		Progress: func(pct float64) {
			if progressCallback != nil {
				progressCallback(pct)
			}
		},
		LogPath: logPath,
	}
	if outputMode == "contactSheet" || outputMode == "both" {
		config.OnThumbGenerated = func(path string) {
			s.setThumbnailLiveContactSheet(path)
		}
	} else {
		config.OnThumbGenerated = func(path string) {
			s.addThumbnailLivePreview(path)
		}
	}

	result, err := generator.Generate(jobCtx, config)
	if err != nil {
		return fmt.Errorf("thumbnail generation failed: %w", err)
	}

	logging.Debug(logging.CatSystem, "generated %d thumbnails", len(result.Thumbnails))

	if progressCallback != nil {
		progressCallback(100)
	}

	return nil
}

func (s *appState) createThumbnailJobForPath(path string) *queue.Job {
	videoDir := filepath.Dir(path)
	videoBaseName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	outputDir := filepath.Join(videoDir, fmt.Sprintf("%s_thumbnails", videoBaseName))
	outputFile := outputDir

	needContactSheet := s.thumbnailOutputMode == "contactSheet" || s.thumbnailOutputMode == "both"
	needIndividual := s.thumbnailOutputMode == "individual" || s.thumbnailOutputMode == "both"

	if needContactSheet && !needIndividual {
		outputDir = videoDir
		outputFile = filepath.Join(videoDir, fmt.Sprintf("%s_contact_sheet.jpg", videoBaseName))
	} else if needContactSheet && needIndividual {
		outputFile = filepath.Join(outputDir, fmt.Sprintf("%s_contact_sheet.jpg", videoBaseName))
	}

	var count, width int
	var description string
	if needContactSheet {
		count = s.thumbnailColumns * s.thumbnailRows
		width = s.thumbnailSheetWidth
		description = fmt.Sprintf("Contact sheet: %dx%d grid (%d thumbnails)", s.thumbnailColumns, s.thumbnailRows, count)
		if needIndividual {
			description = fmt.Sprintf("Contact sheet + %d thumbnails (%dpx)", s.thumbnailCount, s.thumbnailWidth)
		}
	} else {
		count = s.thumbnailCount
		width = s.thumbnailWidth
		description = fmt.Sprintf("%d individual thumbnails (%dpx width)", count, width)
	}

	return &queue.Job{
		Type:        queue.JobTypeThumbnail,
		Title:       filepath.Base(path),
		Description: description,
		InputFile:   path,
		OutputFile:  outputFile,
		Config: map[string]interface{}{
			"inputPath":     path,
			"outputDir":     outputDir,
			"count":         float64(count),
			"width":         float64(width),
			"outputMode":    s.thumbnailOutputMode,
			"showTimestamp": s.thumbnailShowTimestamps,
			"columns":       float64(s.thumbnailColumns),
			"rows":          float64(s.thumbnailRows),
		},
	}
}
