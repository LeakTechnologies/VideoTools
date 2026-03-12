package main

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

func (s *appState) showCompareView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "compare"
	s.maximizeWindow()
	s.setContent(buildCompareView(s))
}

func (s *appState) showCompareFullscreen() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "compare-fullscreen"
	s.setContent(buildCompareFullscreenView(s))
}


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
		comparisonText.WriteString("-----------------------------------------------------------------------\n")
		comparisonText.WriteString("                        VIDEO COMPARISON REPORT\n")
		comparisonText.WriteString("-----------------------------------------------------------------------\n\n")

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
		comparisonText.WriteString("\n\n")

		// Helper to get field value or placeholder
		getField := func(src *videoSource, getter func(*videoSource) string) string {
			if src == nil {
				return ""
			}
			return getter(src)
		}

		// File Info section
		comparisonText.WriteString(" FILE INFO \n")

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
		comparisonText.WriteString("\n VIDEO \n")
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
		comparisonText.WriteString("\n AUDIO \n")
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
		comparisonText.WriteString("\n OTHER \n")
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

		comparisonText.WriteString("\n-----------------------------------------------------------------------\n")

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
			" FILE INFO \n"+
				"Path: %s\n"+
				"File Size: %s\n"+
				"Format Family: %s\n"+
				"\n VIDEO \n"+
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
				"\n AUDIO \n"+
				"Codec: %s\n"+
				"Bitrate: %s\n"+
				"Sample Rate: %d Hz\n"+
				"Channels: %d\n"+
				"\n OTHER \n"+
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
	// Avoid rigid min sizes so window snapping works across modules.

	// Scrollable metadata area for file 2 - use smaller minimum
	file2InfoScroll := container.NewVScroll(file2Info)
	// Avoid rigid min sizes so window snapping works across modules.

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
	playBtn := widget.NewButton("- Play Both", func() {
		// TODO: When VT_Player API supports it, trigger synchronized playback
		dialog.ShowInformation("Synchronized Playback",
			"Synchronized playback control will be available when VT_Player API is enhanced.\n\n"+
				"For now, use individual player controls.",
			state.window)
	})
	playBtn.Importance = widget.HighImportance

	pauseBtn := widget.NewButton(" Pause Both", func() {
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
