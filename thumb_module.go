package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/thumbnail"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

func (s *appState) showThumbView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "thumb"
	if cfg, err := loadPersistedThumbConfig(); err == nil {
		s.applyThumbConfig(cfg)
	}
	s.setContent(buildThumbView(s))
}

func (s *appState) addThumbSource(src *videoSource) {
	if src == nil {
		return
	}
	for _, existing := range s.thumbFiles {
		if existing != nil && existing.Path == src.Path {
			return
		}
	}
	s.thumbFiles = append(s.thumbFiles, src)
}

func (s *appState) loadThumbSourceAtIndex(idx int) {
	if idx < 0 || idx >= len(s.thumbFiles) {
		return
	}
	current := s.thumbFiles[idx]
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
			s.thumbFiles[idx] = probed
			if s.thumbFile != nil && s.thumbFile.Path == path {
				s.thumbFile = probed
			}
			s.showThumbView()
		}, false)
	}()
}

func (s *appState) loadMultipleThumbVideos(paths []string) {
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

	s.thumbFiles = valid
	s.thumbFile = valid[0]

	fyne.CurrentApp().Driver().DoFromGoroutine(func() {
		s.showThumbView()
		if len(failed) > 0 {
			logging.Debug(logging.CatModule, "%d file(s) failed to analyze: %s", len(failed), strings.Join(failed, ", "))
		}
	}, false)
}

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

	clearCompletedBtn := widget.NewButton("⌫", func() {
		state.clearCompletedJobs()
	})
	clearCompletedBtn.Importance = widget.LowImportance

	topBar := ui.TintedBar(thumbColor, container.NewHBox(backBtn, layout.NewSpacer(), clearCompletedBtn, queueBtn))

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
	if state.thumbSheetWidth == 0 {
		state.thumbSheetWidth = 360
	}
	if state.thumbColumns == 0 {
		state.thumbColumns = 4 // 4 columns works well for widescreen videos
	}
	if state.thumbRows == 0 {
		state.thumbRows = 8 // 4x8 = 32 thumbnails
	}

	// File label and video preview
	fileLabel := widget.NewLabel("No file loaded")
	fileLabel.TextStyle = fyne.TextStyle{Bold: true}

	var videoContainer fyne.CanvasObject
	if state.thumbFile != nil && state.thumbFile.Width == 0 && state.thumbFile.Height == 0 {
		fileLabel.SetText(fmt.Sprintf("File: %s", filepath.Base(state.thumbFile.Path)))
		videoContainer = container.NewCenter(widget.NewLabel("Loading preview..."))
	} else if state.thumbFile != nil {
		fileLabel.SetText(fmt.Sprintf("File: %s", filepath.Base(state.thumbFile.Path)))
		videoContainer = buildVideoPane(state, fyne.NewSize(480, 270), state.thumbFile, nil)
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
			state.addThumbSource(src)
			state.showThumbView()
			logging.Debug(logging.CatModule, "loaded thumbnail file: %s", path)
		}, state.window)
	})

	// Clear button
	clearBtn := widget.NewButton("Clear", func() {
		state.thumbFile = nil
		state.thumbFiles = nil
		state.showThumbView()
	})
	clearBtn.Importance = widget.LowImportance

	// Contact sheet checkbox (wrapped)
	contactSheetCheck := widget.NewCheck("", func(checked bool) {
		state.thumbContactSheet = checked
		state.persistThumbConfig()
		state.showThumbView()
	})
	contactSheetCheck.Checked = state.thumbContactSheet
	contactSheetLabel := widget.NewLabel("Generate Contact Sheet (single image)")
	contactSheetLabel.Wrapping = fyne.TextWrapWord
	contactSheetToggle := ui.NewTappable(contactSheetLabel, func() {
		contactSheetCheck.SetChecked(!contactSheetCheck.Checked)
	})
	contactSheetRow := container.NewBorder(nil, nil, contactSheetCheck, nil, contactSheetToggle)

	timestampCheck := widget.NewCheck("", func(checked bool) {
		state.thumbShowTimestamps = checked
		state.persistThumbConfig()
	})
	timestampCheck.Checked = state.thumbShowTimestamps
	timestampLabel := widget.NewLabel("Show timestamps on thumbnails")
	timestampLabel.Wrapping = fyne.TextWrapWord
	timestampToggle := ui.NewTappable(timestampLabel, func() {
		timestampCheck.SetChecked(!timestampCheck.Checked)
	})
	timestampRow := container.NewBorder(nil, nil, timestampCheck, nil, timestampToggle)

	// Conditional settings based on contact sheet mode
	var settingsOptions fyne.CanvasObject
	if state.thumbContactSheet {
		// Contact sheet mode: show columns and rows
		colLabel := widget.NewLabel(fmt.Sprintf("Columns: %d", state.thumbColumns))
		rowLabel := widget.NewLabel(fmt.Sprintf("Rows: %d", state.thumbRows))

		totalThumbs := state.thumbColumns * state.thumbRows
		totalLabel := widget.NewLabel(fmt.Sprintf("Total thumbnails: %d", totalThumbs))
		totalLabel.TextStyle = fyne.TextStyle{Italic: true}
		totalLabel.Wrapping = fyne.TextWrapWord

		colSlider := widget.NewSlider(2, 9)
		colSlider.Value = float64(state.thumbColumns)
		colSlider.Step = 1
		colSlider.OnChanged = func(val float64) {
			state.thumbColumns = int(val)
			colLabel.SetText(fmt.Sprintf("Columns: %d", int(val)))
			totalLabel.SetText(fmt.Sprintf("Total thumbnails: %d", state.thumbColumns*state.thumbRows))
			state.persistThumbConfig()
		}

		rowSlider := widget.NewSlider(2, 12)
		rowSlider.Value = float64(state.thumbRows)
		rowSlider.Step = 1
		rowSlider.OnChanged = func(val float64) {
			state.thumbRows = int(val)
			rowLabel.SetText(fmt.Sprintf("Rows: %d", int(val)))
			totalLabel.SetText(fmt.Sprintf("Total thumbnails: %d", state.thumbColumns*state.thumbRows))
			state.persistThumbConfig()
		}

		sizeOptions := []string{"240 px", "300 px", "360 px", "420 px", "480 px"}
		sizeSelect := widget.NewSelect(sizeOptions, func(val string) {
			switch val {
			case "240 px":
				state.thumbSheetWidth = 240
			case "300 px":
				state.thumbSheetWidth = 300
			case "360 px":
				state.thumbSheetWidth = 360
			case "420 px":
				state.thumbSheetWidth = 420
			case "480 px":
				state.thumbSheetWidth = 480
			}
			state.persistThumbConfig()
		})
		switch state.thumbSheetWidth {
		case 240:
			sizeSelect.SetSelected("240 px")
		case 300:
			sizeSelect.SetSelected("300 px")
		case 420:
			sizeSelect.SetSelected("420 px")
		case 480:
			sizeSelect.SetSelected("480 px")
		default:
			sizeSelect.SetSelected("360 px")
		}

		settingsOptions = container.NewVBox(
			widget.NewSeparator(),
			widget.NewLabel("Contact Sheet Grid:"),
			widget.NewLabel("Thumbnail Size:"),
			sizeSelect,
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
			state.persistThumbConfig()
		}

		widthLabel := widget.NewLabel(fmt.Sprintf("Thumbnail Width: %d px", state.thumbWidth))
		widthSlider := widget.NewSlider(160, 640)
		widthSlider.Value = float64(state.thumbWidth)
		widthSlider.Step = 32
		widthSlider.OnChanged = func(val float64) {
			state.thumbWidth = int(val)
			widthLabel.SetText(fmt.Sprintf("Thumbnail Width: %d px", int(val)))
			state.persistThumbConfig()
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
		return state.createThumbJobForPath(state.thumbFile.Path)
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

		// Start queue if not already running
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

	addAllBtn := widget.NewButton("Add All to Queue", func() {
		if len(state.thumbFiles) == 0 {
			dialog.ShowInformation("No Videos", "Load videos first to add to queue.", state.window)
			return
		}
		if state.jobQueue == nil {
			dialog.ShowInformation("Queue", "Queue not initialized.", state.window)
			return
		}
		for _, src := range state.thumbFiles {
			if src == nil || src.Path == "" {
				continue
			}
			state.jobQueue.Add(state.createThumbJobForPath(src.Path))
		}
		dialog.ShowInformation("Queue", fmt.Sprintf("Queued %d thumbnail jobs.", len(state.thumbFiles)), state.window)
	})
	addAllBtn.Importance = widget.MediumImportance

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

		// If contact sheet mode, try to open contact sheet image
		if state.thumbContactSheet {
			contactSheetPath := filepath.Join(outputDir, fmt.Sprintf("%s_contact_sheet.jpg", videoBaseName))
			if _, err := os.Stat(contactSheetPath); err == nil {
				if err := openFile(contactSheetPath); err != nil {
					dialog.ShowError(fmt.Errorf("failed to open contact sheet: %w", err), state.window)
				}
				return
			}
		}

		// Otherwise, open first thumbnail
		firstThumb := filepath.Join(outputDir, "thumb_0001.jpg")
		if _, err := os.Stat(firstThumb); err == nil {
			if err := openFile(firstThumb); err != nil {
				dialog.ShowError(fmt.Errorf("failed to open thumbnail: %w", err), state.window)
			}
			return
		}

		// Fall back to opening the folder if no images found
		if err := openFolder(outputDir); err != nil {
			dialog.ShowError(fmt.Errorf("failed to open results folder: %w", err), state.window)
		}
	})
	viewResultsBtn.Importance = widget.MediumImportance
	if state.thumbFile == nil {
		viewResultsBtn.Disable()
	}

	// Settings panel
	settingsPanel := container.NewVBox(
		widget.NewLabel("Settings:"),
		widget.NewSeparator(),
		contactSheetRow,
		timestampRow,
		settingsOptions,
		widget.NewSeparator(),
		generateNowBtn,
		addQueueBtn,
		addAllBtn,
		viewQueueBtn,
		viewResultsBtn,
	)

	// Main content - split layout with preview on left, settings on right
	leftColumn := container.NewVBox(videoContainer)
	if len(state.thumbFiles) > 1 {
		list := widget.NewList(
			func() int { return len(state.thumbFiles) },
			func() fyne.CanvasObject { return widget.NewLabel("") },
			func(i widget.ListItemID, o fyne.CanvasObject) {
				if i < 0 || i >= len(state.thumbFiles) {
					return
				}
				label := o.(*widget.Label)
				src := state.thumbFiles[i]
				if src == nil {
					label.SetText("")
					return
				}
				label.SetText(utils.ShortenMiddle(filepath.Base(src.Path), 60))
			},
		)
		list.OnSelected = func(id widget.ListItemID) {
			if id < 0 || id >= len(state.thumbFiles) {
				return
			}
			state.thumbFile = state.thumbFiles[id]
			state.loadThumbSourceAtIndex(id)
			state.showThumbView()
		}
		if state.thumbFile != nil {
			for i, src := range state.thumbFiles {
				if src != nil && src.Path == state.thumbFile.Path {
					list.Select(i)
					break
				}
			}
		}
		listScroll := container.NewVScroll(list)
		listScroll.SetMinSize(fyne.NewSize(0, 140))
		leftColumn.Add(widget.NewLabel("Loaded Videos:"))
		leftColumn.Add(listScroll)
	}

	rightColumn := container.NewVScroll(settingsPanel)

	mainContent := container.New(&fixedHSplitLayout{ratio: 0.6}, leftColumn, rightColumn)

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

func (s *appState) executeThumbJob(ctx context.Context, job *queue.Job, progressCallback func(float64)) error {
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

	generator := thumbnail.NewGenerator(utils.GetFFmpegPath())
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
		ShowTimestamp: showTimestamp,
		ShowMetadata:  contactSheet,
		Progress: func(pct float64) {
			if progressCallback != nil {
				progressCallback(pct)
			}
		},
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

func (s *appState) createThumbJobForPath(path string) *queue.Job {
	videoDir := filepath.Dir(path)
	videoBaseName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	outputDir := filepath.Join(videoDir, fmt.Sprintf("%s_thumbnails", videoBaseName))

	var count, width int
	var description string
	if s.thumbContactSheet {
		count = s.thumbColumns * s.thumbRows
		width = s.thumbSheetWidth
		description = fmt.Sprintf("Contact sheet: %dx%d grid (%d thumbnails)", s.thumbColumns, s.thumbRows, count)
	} else {
		count = s.thumbCount
		width = s.thumbWidth
		description = fmt.Sprintf("%d individual thumbnails (%dpx width)", count, width)
	}

	return &queue.Job{
		Type:        queue.JobTypeThumb,
		Title:       "Thumbnails: " + filepath.Base(path),
		Description: description,
		InputFile:   path,
		OutputFile:  outputDir,
		Config: map[string]interface{}{
			"inputPath":     path,
			"outputDir":     outputDir,
			"count":         float64(count),
			"width":         float64(width),
			"contactSheet":  s.thumbContactSheet,
			"showTimestamp": s.thumbShowTimestamps,
			"columns":       float64(s.thumbColumns),
			"rows":          float64(s.thumbRows),
		},
	}
}
