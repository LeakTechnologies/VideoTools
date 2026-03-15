package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/interlace"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

func (s *appState) showInspectView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "inspect"
	s.maximizeWindow()
	s.setContent(buildInspectView(s))
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

	clearCompletedBtn := widget.NewButton("⌫", func() {
		state.clearCompletedJobs()
	})
	clearCompletedBtn.Importance = widget.LowImportance

	topBar := ui.TintedBar(inspectColor, container.NewHBox(backBtn, layout.NewSpacer(), clearCompletedBtn, queueBtn))
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

	// Helper to build boxed sections matching Convert module style
	gridColor := utils.MustHex("#2A3A52")
	navyBlue := utils.MustHex("#191F35")

	buildInspectBox := func(title string, content fyne.CanvasObject) fyne.CanvasObject {
		bg := canvas.NewRectangle(navyBlue)
		bg.CornerRadius = 10
		bg.StrokeColor = gridColor
		bg.StrokeWidth = 1
		header := container.NewVBox(
			widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewSeparator(),
		)
		body := container.NewBorder(header, nil, nil, nil, content)
		layers := ui.NoisyBackgroundObjects(bg)
		layers = append(layers, container.NewPadded(body))
		return container.NewMax(layers...)
	}

	// Metadata scroll
	metadataScroll := container.NewScroll(metadataText)
	// metadataScroll.SetMinSize(fyne.NewSize(400, 200)) // Removed for flexible sizing

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
			videoContainer = buildVideoPane(state, fyne.NewSize(480, 270), state.inspectFile, nil)
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
				detector := interlace.NewDetector(utils.GetFFmpegPath(), utils.GetFFprobePath())
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

	rightColumn := buildInspectBox("Metadata", metadataScroll)

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
