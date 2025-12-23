package main

import (
	"fmt"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
)

// buildVideoClipsTab creates the video clips tab with drag-and-drop support
func buildVideoClipsTab(state *appState) fyne.CanvasObject {
	// Video clips list with drag-and-drop support
	list := container.NewVBox()
	
	rebuildList := func() {
		list.Objects = nil
		
		if len(state.authorClips) == 0 {
			emptyLabel := widget.NewLabel("Drag and drop video files here\nor click 'Add Files' to select videos")
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
					state.addAuthorFiles(paths)
				}
			})
			
			list.Add(container.NewMax(emptyDrop))
		} else {
			for i, clip := range state.authorClips {
				idx := i
				card := widget.NewCard(clip.DisplayName, fmt.Sprintf("%.2fs", clip.Duration), nil)
				
				// Remove button
				removeBtn := widget.NewButton("Remove", func() {
					state.authorClips = append(state.authorClips[:idx], state.authorClips[idx+1:]...)
					rebuildList()
				})
				removeBtn.Importance = widget.MediumImportance
				
				// Duration label
				durationLabel := widget.NewLabel(fmt.Sprintf("Duration: %.2f seconds", clip.Duration))
				durationLabel.TextStyle = fyne.TextStyle{Italic: true}
				
				cardContent := container.NewVBox(
					durationLabel,
					widget.NewSeparator(),
					removeBtn,
				)
				card.SetContent(cardContent)
				list.Add(card)
			}
		}
	}
	
	// Add files button
	addBtn := widget.NewButton("Add Files", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			state.addAuthorFiles([]string{reader.URI().Path()})
		}, state.window)
	})
	addBtn.Importance = widget.HighImportance
	
	// Clear all button
	clearBtn := widget.NewButton("Clear All", func() {
		state.authorClips = []authorClip{}
		rebuildList()
	})
	clearBtn.Importance = widget.MediumImportance
	
	// Compile button
	compileBtn := widget.NewButton("COMPILE TO DVD", func() {
		if len(state.authorClips) == 0 {
			dialog.ShowInformation("No Clips", "Please add video clips first", state.window)
			return
		}
		// TODO: Implement compilation to DVD
		dialog.ShowInformation("Compile", "DVD compilation will be implemented", state.window)
	})
	compileBtn.Importance = widget.HighImportance
	
	controls := container.NewVBox(
		widget.NewLabel("Video Clips:"),
		container.NewScroll(list),
		widget.NewSeparator(),
		container.NewHBox(addBtn, clearBtn, compileBtn),
	)
	
	// Initialize the list
	rebuildList()
	
	return container.NewPadded(controls)
}

// addAuthorFiles helper function
func (s *appState) addAuthorFiles(paths []string) {
	for _, path := range paths {
		src, err := probeVideo(path)
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to load video %s: %w", filepath.Base(path), err), s.window)
			continue
		}
		
		clip := authorClip{
			Path:        path,
			DisplayName:  filepath.Base(path),
			Duration:    src.Duration,
			Chapters:     []authorChapter{},
		}
		s.authorClips = append(s.authorClips, clip)
	}
}

// buildSubtitlesTab creates the subtitles tab with drag-and-drop support
func buildSubtitlesTab(state *appState) fyne.CanvasObject {
	// Subtitle files list with drag-and-drop support
	list := container.NewVBox()
	
	rebuildSubList := func() {
		list.Objects = nil
		
		if len(state.authorSubtitles) == 0 {
			emptyLabel := widget.NewLabel("Drag and drop subtitle files here\nor click 'Add Subtitles' to select")
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
					state.authorSubtitles = append(state.authorSubtitles, paths...)
					rebuildSubList()
				}
			})
			
			list.Add(container.NewMax(emptyDrop))
		} else {
			for i, path := range state.authorSubtitles {
				idx := i
				card := widget.NewCard(filepath.Base(path), "", nil)
				
				// Remove button
				removeBtn := widget.NewButton("Remove", func() {
					state.authorSubtitles = append(state.authorSubtitles[:idx], state.authorSubtitles[idx+1:]...)
					rebuildSubList()
				})
				removeBtn.Importance = widget.MediumImportance
				
				cardContent := container.NewVBox(removeBtn)
				card.SetContent(cardContent)
				list.Add(card)
			}
		}
	}
	
	// Add subtitles button
	addBtn := widget.NewButton("Add Subtitles", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			state.authorSubtitles = append(state.authorSubtitles, reader.URI().Path())
			rebuildSubList()
		}, state.window)
	})
	addBtn.Importance = widget.HighImportance
	
	// Clear all button
	clearBtn := widget.NewButton("Clear All", func() {
		state.authorSubtitles = []string{}
		rebuildSubList()
	})
	clearBtn.Importance = widget.MediumImportance
	
	controls := container.NewVBox(
		widget.NewLabel("Subtitle Tracks:"),
		container.NewScroll(list),
		widget.NewSeparator(),
		container.NewHBox(addBtn, clearBtn),
	)
	
	// Initialize
	rebuildSubList()
	
	return container.NewPadded(controls)
}

// buildAuthorSettingsTab creates the author settings tab
func buildAuthorSettingsTab(state *appState) fyne.CanvasObject {
	// Output type selection
	outputType := widget.NewSelect([]string{"DVD (VIDEO_TS)", "ISO Image"})
	outputType.OnChanged = func(value string) {
		if value == "DVD (VIDEO_TS)" {
			state.authorOutputType = "dvd"
		} else {
			state.authorOutputType = "iso"
		}
	})
	if state.authorOutputType == "iso" {
		outputType.SetSelected("ISO Image")
	}
	
	// Region selection
	regionSelect := widget.NewSelect([]string{"AUTO", "NTSC", "PAL"})
	regionSelect.OnChanged = func(value string) {
		state.authorRegion = value
	})
	if state.authorRegion == "" {
		state.authorRegion = "AUTO"
		regionSelect.SetSelected("AUTO")
	} else {
		regionSelect.SetSelected(state.authorRegion)
	}
	
	// Aspect ratio selection
	aspectSelect := widget.NewSelect([]string{"AUTO", "4:3", "16:9"})
	aspectSelect.OnChanged = func(value string) {
		state.authorAspectRatio = value
	})
	if state.authorAspectRatio == "" {
		state.authorAspectRatio = "AUTO"
		aspectSelect.SetSelected("AUTO")
	} else {
		aspectSelect.SetSelected(state.authorAspectRatio)
	}
	
	// DVD title entry
	titleEntry := widget.NewEntry()
	titleEntry.SetPlaceHolder("DVD Title")
	titleEntry.SetText(state.authorTitle)
	titleEntry.OnChanged = func(value string) {
		state.authorTitle = value
	}
	
	// Create menu checkbox
	createMenuCheck := widget.NewCheck("Create DVD Menu", func(checked bool) {
		state.authorCreateMenu = checked
	})
	createMenuCheck.SetChecked(state.authorCreateMenu)
	
	controls := container.NewVBox(
		widget.NewLabel("Output Settings:"),
		widget.NewSeparator(),
		widget.NewLabel("Output Type:"),
		outputType,
		widget.NewLabel("Region:"),
		regionSelect,
		widget.NewLabel("Aspect Ratio:"),
		aspectSelect,
		widget.NewLabel("DVD Title:"),
		titleEntry,
		createMenuCheck,
	)
	
	return container.NewPadded(controls)
}

// buildAuthorDiscTab creates the DVD generation tab
func buildAuthorDiscTab(state *appState) fyne.CanvasObject {
	// Generate DVD/ISO
	generateBtn := widget.NewButton("GENERATE DVD", func() {
		if len(state.authorClips) == 0 {
			dialog.ShowInformation("No Content", "Please add video clips first", state.window)
			return
		}
		
		// Show compilation options
		dialog.ShowInformation("DVD Generation", 
			"DVD/ISO generation will be implemented in next step.\n\n"+
			"Features planned:\n"+
			"• Create VIDEO_TS folder structure\n"+
			"• Generate burn-ready ISO\n"+
			"• Include subtitle tracks\n"+
			"• Include alternate audio tracks\n"+
			"• Support for alternate camera angles", state.window)
	})
	generateBtn.Importance = widget.HighImportance
	
	// Show summary
	summary := "Ready to generate:\n\n"
	if len(state.authorClips) > 0 {
		summary += fmt.Sprintf("Video Clips: %d\n", len(state.authorClips))
		for i, clip := range state.authorClips {
			summary += fmt.Sprintf("  %d. %s (%.2fs)\n", i+1, clip.DisplayName, clip.Duration)
		}
	}
	
	if len(state.authorSubtitles) > 0 {
		summary += fmt.Sprintf("Subtitle Tracks: %d\n", len(state.authorSubtitles))
		for i, path := range state.authorSubtitles {
			summary += fmt.Sprintf("  %d. %s\n", i+1, filepath.Base(path))
		}
	}
	
	summary += fmt.Sprintf("Output Type: %s\n", state.authorOutputType)
	summary += fmt.Sprintf("Region: %s\n", state.authorRegion)
	summary += fmt.Sprintf("Aspect Ratio: %s\n", state.authorAspectRatio)
	if state.authorTitle != "" {
		summary += fmt.Sprintf("DVD Title: %s\n", state.authorTitle)
	}
	
	summaryLabel := widget.NewLabel(summary)
	summaryLabel.Wrapping = fyne.TextWrapWord
	
	controls := container.NewVBox(
		widget.NewLabel("Generate DVD/ISO:"),
		widget.NewSeparator(),
		summaryLabel,
		widget.NewSeparator(),
		generateBtn,
	)
	
	return container.NewPadded(controls)
}