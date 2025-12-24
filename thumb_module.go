package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/thumbnail"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
)

func (s *appState) showThumbView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "thumb"
	s.setContent(buildThumbView(s))
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

		// If contact sheet mode, try to show contact sheet image
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
