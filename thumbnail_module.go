package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"

	thumbpkg "git.leaktechnologies.dev/stu/VideoTools/internal/app/modules/thumbnail"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	thumbsvc "git.leaktechnologies.dev/stu/VideoTools/internal/thumbnail"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

func init() {
	// Load logo PNG from embedded assets for contact-sheet headers.
	if f, err := logoAssets.Open("assets/logo/VT_Icon.png"); err == nil {
		if data, err2 := io.ReadAll(f); err2 == nil {
			thumbsvc.SetLogoData(data)
		}
		f.Close()
	}
	thumbsvc.SetFontData(ibmPlexMonoRegular)
	thumbsvc.SetBoldFontData(ibmPlexMonoBold)
}

func (s *appState) showThumbnailView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "thumbnail"
	s.maximizeWindow()
	if cfg, err := loadPersistedThumbnailConfig(); err == nil {
		s.applyThumbnailConfig(cfg)
	}
	s.setContent(buildThumbnailView(s))
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
	for i, f := range state.thumbnailFiles {
		thumbFiles[i] = f
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

	opts := thumbpkg.Options{
		Window:                  state.window,
		ModuleColor:             moduleColor("thumbnail"),
		ThumbnailFile:           state.thumbnailFile,
		ThumbnailFiles:          thumbFiles,
		ThumbnailFileName:       thumbFileName,
		ThumbnailFileNames:      thumbFileNames,
		ThumbnailPreviewFrame:   thumbPreviewFrame,
		ThumbnailCount:          state.thumbnailCount,
		ThumbnailWidth:          state.thumbnailWidth,
		ThumbnailSheetWidth:     state.thumbnailSheetWidth,
		ThumbnailColumns:        state.thumbnailColumns,
		ThumbnailRows:           state.thumbnailRows,
		ThumbnailContactSheet:   state.thumbnailContactSheet,
		ThumbnailShowTimestamps: state.thumbnailShowTimestamps,
		OnShowMainMenu:          func() { state.showMainMenu() },
		OnShowQueue:             func() { state.showQueue() },
		OnShowThumbnailView:     func() { state.showThumbnailView() },
		OnClearCompletedJobs:    func() { state.clearCompletedJobs() },
		OnGetStatsBar:           func() fyne.CanvasObject { return state.statsBar },
		OnLoadFile: func(path string) {
			src, err := probeVideo(path)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to load video: %w", err), state.window)
				return
			}
			state.thumbnailFile = src
			state.addThumbnailSource(src)
			state.showThumbnailView()
			logging.Debug(logging.CatModule, "loaded thumbnail file: %s", path)
			go func() {
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
		OnSetThumbnailContactSheet:   func(b bool) { state.thumbnailContactSheet = b },
		OnSetThumbnailShowTimestamps: func(b bool) { state.thumbnailShowTimestamps = b },
		OnCreateThumbJob: func() {
			if state.thumbnailFile == nil {
				return
			}
			job := state.createThumbnailJobForPath(state.thumbnailFile.Path)
			state.jobQueue.Add(job)
			if !state.jobQueue.IsRunning() {
				state.jobQueue.Start()
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
	contactSheet := cfg["contactSheet"].(bool)
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

	totalThumbs := count
	if contactSheet {
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
	config := thumbsvc.Config{
		VideoPath:     inputPath,
		OutputDir:     outputDir,
		Count:         count,
		Width:         width,
		Format:        "jpg",
		Quality:       85,
		ContactSheet:  contactSheet,
		Columns:       columns,
		Rows:          rows,
		ShowTimestamp: showTimestamp,
		ShowMetadata:  contactSheet,
		Progress: func(pct float64) {
			if progressCallback != nil {
				progressCallback(pct)
			}
		},
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
	if s.thumbnailContactSheet {
		outputDir = videoDir
		outputFile = filepath.Join(videoDir, fmt.Sprintf("%s_contact_sheet.jpg", videoBaseName))
	}

	var count, width int
	var description string
	if s.thumbnailContactSheet {
		count = s.thumbnailColumns * s.thumbnailRows
		width = s.thumbnailSheetWidth
		description = fmt.Sprintf("Contact sheet: %dx%d grid (%d thumbnails)", s.thumbnailColumns, s.thumbnailRows, count)
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
			"contactSheet":  s.thumbnailContactSheet,
			"showTimestamp": s.thumbnailShowTimestamps,
			"columns":       float64(s.thumbnailColumns),
			"rows":          float64(s.thumbnailRows),
		},
	}
}
