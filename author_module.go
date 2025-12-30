package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

type authorConfig struct {
	OutputType      string  `json:"outputType"`
	Region          string  `json:"region"`
	AspectRatio     string  `json:"aspectRatio"`
	DiscSize        string  `json:"discSize"`
	Title           string  `json:"title"`
	CreateMenu      bool    `json:"createMenu"`
	TreatAsChapters bool    `json:"treatAsChapters"`
	SceneThreshold  float64 `json:"sceneThreshold"`
}

func defaultAuthorConfig() authorConfig {
	return authorConfig{
		OutputType:      "dvd",
		Region:          "AUTO",
		AspectRatio:     "AUTO",
		DiscSize:        "DVD5",
		Title:           "",
		CreateMenu:      false,
		TreatAsChapters: false,
		SceneThreshold:  0.3,
	}
}

func loadPersistedAuthorConfig() (authorConfig, error) {
	var cfg authorConfig
	path := moduleConfigPath("author")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.OutputType == "" {
		cfg.OutputType = "dvd"
	}
	if cfg.Region == "" {
		cfg.Region = "AUTO"
	}
	if cfg.AspectRatio == "" {
		cfg.AspectRatio = "AUTO"
	}
	if cfg.DiscSize == "" {
		cfg.DiscSize = "DVD5"
	}
	if cfg.SceneThreshold <= 0 {
		cfg.SceneThreshold = 0.3
	}
	return cfg, nil
}

func savePersistedAuthorConfig(cfg authorConfig) error {
	path := moduleConfigPath("author")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (s *appState) applyAuthorConfig(cfg authorConfig) {
	s.authorOutputType = cfg.OutputType
	s.authorRegion = cfg.Region
	s.authorAspectRatio = cfg.AspectRatio
	s.authorDiscSize = cfg.DiscSize
	s.authorTitle = cfg.Title
	s.authorCreateMenu = cfg.CreateMenu
	s.authorTreatAsChapters = cfg.TreatAsChapters
	s.authorSceneThreshold = cfg.SceneThreshold
}

func (s *appState) persistAuthorConfig() {
	cfg := authorConfig{
		OutputType:      s.authorOutputType,
		Region:          s.authorRegion,
		AspectRatio:     s.authorAspectRatio,
		DiscSize:        s.authorDiscSize,
		Title:           s.authorTitle,
		CreateMenu:      s.authorCreateMenu,
		TreatAsChapters: s.authorTreatAsChapters,
		SceneThreshold:  s.authorSceneThreshold,
	}
	if err := savePersistedAuthorConfig(cfg); err != nil {
		logging.Debug(logging.CatSystem, "failed to persist author config: %v", err)
	}
}

func buildAuthorView(state *appState) fyne.CanvasObject {
	state.stopPreview()
	state.lastModule = state.active
	state.active = "author"

	if cfg, err := loadPersistedAuthorConfig(); err == nil {
		state.applyAuthorConfig(cfg)
	}

	if state.authorOutputType == "" {
		state.authorOutputType = "dvd"
	}
	if state.authorRegion == "" {
		state.authorRegion = "AUTO"
	}
	if state.authorAspectRatio == "" {
		state.authorAspectRatio = "AUTO"
	}
	if state.authorDiscSize == "" {
		state.authorDiscSize = "DVD5"
	}

	authorColor := moduleColor("author")

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

	topBar := ui.TintedBar(authorColor, container.NewHBox(backBtn, layout.NewSpacer(), clearCompletedBtn, queueBtn))
	bottomBar := moduleFooter(authorColor, layout.NewSpacer(), state.statsBar)

	tabs := container.NewAppTabs(
		container.NewTabItem("Videos", buildVideoClipsTab(state)),
		container.NewTabItem("Chapters", buildChaptersTab(state)),
		container.NewTabItem("Subtitles", buildSubtitlesTab(state)),
		container.NewTabItem("Settings", buildAuthorSettingsTab(state)),
		container.NewTabItem("Generate", buildAuthorDiscTab(state)),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	return container.NewBorder(topBar, bottomBar, nil, nil, tabs)
}

func buildVideoClipsTab(state *appState) fyne.CanvasObject {
	state.authorVideoTSPath = strings.TrimSpace(state.authorVideoTSPath)
	list := container.NewVBox()
	listScroll := container.NewVScroll(list)

	var rebuildList func()
	var emptyOverlay *fyne.Container
	rebuildList = func() {
		list.Objects = nil

		if len(state.authorClips) == 0 {
			if emptyOverlay != nil {
				emptyOverlay.Show()
			}
			list.Refresh()
			return
		}

		if emptyOverlay != nil {
			emptyOverlay.Hide()
		}
		for i, clip := range state.authorClips {
			idx := i
			nameLabel := widget.NewLabel(clip.DisplayName)
			nameLabel.TextStyle = fyne.TextStyle{Bold: true}
			durationLabel := widget.NewLabel(fmt.Sprintf("%.2fs", clip.Duration))
			durationLabel.TextStyle = fyne.TextStyle{Italic: true}
			durationLabel.Alignment = fyne.TextAlignTrailing

			titleEntry := widget.NewEntry()
			titleEntry.SetPlaceHolder(fmt.Sprintf("Chapter %d", idx+1))
			titleEntry.SetText(clip.ChapterTitle)
			titleEntry.OnChanged = func(val string) {
				state.authorClips[idx].ChapterTitle = val
				if state.authorTreatAsChapters {
					state.authorChapters = chaptersFromClips(state.authorClips)
					state.authorChapterSource = "clips"
					state.updateAuthorSummary()
				}
			}

			removeBtn := widget.NewButton("Remove", func() {
				state.authorClips = append(state.authorClips[:idx], state.authorClips[idx+1:]...)
				rebuildList()
				state.updateAuthorSummary()
			})
			removeBtn.Importance = widget.MediumImportance

			row := container.NewBorder(
				nil,
				nil,
				nil,
				container.NewVBox(durationLabel, removeBtn),
				container.NewVBox(nameLabel, titleEntry),
			)
			cardBg := canvas.NewRectangle(utils.MustHex("#171C2A"))
			cardBg.CornerRadius = 6
			cardBg.SetMinSize(fyne.NewSize(0, nameLabel.MinSize().Height+durationLabel.MinSize().Height+12))
			list.Add(container.NewPadded(container.NewMax(cardBg, row)))
		}
		list.Refresh()
	}

	addBtn := widget.NewButton("Add Files", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			state.addAuthorFiles([]string{reader.URI().Path()})
			rebuildList()
		}, state.window)
	})
	addBtn.Importance = widget.HighImportance

	clearBtn := widget.NewButton("Clear All", func() {
		state.authorClips = []authorClip{}
		state.authorChapters = nil
		state.authorChapterSource = ""
		state.authorVideoTSPath = ""
		state.authorTitle = ""
		rebuildList()
		state.updateAuthorSummary()
	})
	clearBtn.Importance = widget.MediumImportance

	addQueueBtn := widget.NewButton("Add to Queue", func() {
		if len(state.authorClips) == 0 {
			dialog.ShowInformation("No Clips", "Please add video clips first", state.window)
			return
		}
		state.startAuthorGeneration(false)
	})
	addQueueBtn.Importance = widget.MediumImportance

	compileBtn := widget.NewButton("COMPILE TO DVD", func() {
		if len(state.authorClips) == 0 {
			dialog.ShowInformation("No Clips", "Please add video clips first", state.window)
			return
		}
		state.startAuthorGeneration(true)
	})
	compileBtn.Importance = widget.HighImportance

	chapterToggle := widget.NewCheck("Treat videos as chapters", func(checked bool) {
		state.authorTreatAsChapters = checked
		if checked {
			state.authorChapters = chaptersFromClips(state.authorClips)
			state.authorChapterSource = "clips"
		} else if state.authorChapterSource == "clips" {
			state.authorChapterSource = ""
			state.authorChapters = nil
		}
		state.updateAuthorSummary()
		state.persistAuthorConfig()
		if state.authorChaptersRefresh != nil {
			state.authorChaptersRefresh()
		}
	})
	chapterToggle.SetChecked(state.authorTreatAsChapters)

	dropTarget := ui.NewDroppable(listScroll, func(items []fyne.URI) {
		var paths []string
		for _, uri := range items {
			if uri.Scheme() == "file" {
				paths = append(paths, uri.Path())
			}
		}
		if len(paths) > 0 {
			state.addAuthorFiles(paths)
			rebuildList()
		}
	})

	emptyLabel := widget.NewLabel("Drag and drop video files here\nor click 'Add Files' to select videos")
	emptyLabel.Alignment = fyne.TextAlignCenter
	emptyOverlay = container.NewCenter(emptyLabel)

	listArea := container.NewMax(dropTarget, emptyOverlay)

	controls := container.NewBorder(
		widget.NewLabel("Videos:"),
		container.NewVBox(chapterToggle, container.NewHBox(addBtn, clearBtn, addQueueBtn, compileBtn)),
		nil,
		nil,
		listArea,
	)

	rebuildList()
	return container.NewPadded(controls)
}

func buildChaptersTab(state *appState) fyne.CanvasObject {
	var fileLabel *widget.Label
	if state.authorFile != nil {
		fileLabel = widget.NewLabel(fmt.Sprintf("File: %s", filepath.Base(state.authorFile.Path)))
		fileLabel.TextStyle = fyne.TextStyle{Bold: true}
	} else {
		fileLabel = widget.NewLabel("Select a single video file or use clips from Videos tab")
	}

	chapterList := container.NewVBox()
	sourceLabel := widget.NewLabel("")
	refreshChapters := func() {
		chapterList.Objects = nil
		sourceLabel.SetText("")
		if len(state.authorChapters) == 0 {
			if state.authorTreatAsChapters && len(state.authorClips) > 1 {
				state.authorChapters = chaptersFromClips(state.authorClips)
				state.authorChapterSource = "clips"
			}
		}
		if len(state.authorChapters) == 0 {
			chapterList.Add(widget.NewLabel("No chapters detected yet"))
			return
		}
		switch state.authorChapterSource {
		case "clips":
			sourceLabel.SetText("Source: Video clips (treat as chapters)")
		case "embedded":
			sourceLabel.SetText("Source: Embedded chapters")
		case "scenes":
			sourceLabel.SetText("Source: Scene detection")
		default:
			sourceLabel.SetText("Source: Chapters")
		}
		for i, ch := range state.authorChapters {
			title := ch.Title
			if title == "" {
				title = fmt.Sprintf("Chapter %d", i+1)
			}
			chapterList.Add(widget.NewLabel(fmt.Sprintf("%02d. %s (%s)", i+1, title, formatChapterTime(ch.Timestamp))))
		}
	}
	state.authorChaptersRefresh = refreshChapters

	selectBtn := widget.NewButton("Select Video", func() {
		dialog.ShowFileOpen(func(uc fyne.URIReadCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			defer uc.Close()
			path := uc.URI().Path()
			src, err := probeVideo(path)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to load video: %w", err), state.window)
				return
			}
			state.authorFile = src
			fileLabel.SetText(fmt.Sprintf("File: %s", filepath.Base(src.Path)))
            // Clear the custom title so it can be re-derived from the new content.
            // This addresses the user's request for the title to "reset".
            state.authorTitle = ""
            state.updateAuthorSummary()
            // Update the UI for the title entry if the settings tab is currently visible.
			if state.active == "author" && state.window.Canvas() != nil {
				app := fyne.CurrentApp()
				if app != nil && app.Driver() != nil {
					app.Driver().DoFromGoroutine(func() {
						state.showAuthorView() // Rebuild the module to refresh titleEntry
					}, false)
				}
			}
			state.loadEmbeddedChapters(path)
			refreshChapters()
		}, state.window)
	})

	thresholdLabel := widget.NewLabel(fmt.Sprintf("Detection Sensitivity: %.2f", state.authorSceneThreshold))
	thresholdSlider := widget.NewSlider(0.1, 0.9)
	thresholdSlider.Value = state.authorSceneThreshold
	thresholdSlider.Step = 0.05
	thresholdSlider.OnChanged = func(v float64) {
		state.authorSceneThreshold = v
		thresholdLabel.SetText(fmt.Sprintf("Detection Sensitivity: %.2f", v))
		state.persistAuthorConfig()
	}

	detectBtn := widget.NewButton("Detect Scenes", func() {
		targetPath := ""
		if state.authorFile != nil {
			targetPath = state.authorFile.Path
		} else if len(state.authorClips) > 0 {
			targetPath = state.authorClips[0].Path
		}
		if targetPath == "" {
			dialog.ShowInformation("No File", "Please select a video file first", state.window)
			return
		}

		progress := dialog.NewProgressInfinite("Scene Detection", "Analyzing scene changes with FFmpeg...", state.window)
		progress.Show()
		state.authorDetecting = true

		go func() {
			chapters, err := detectSceneChapters(targetPath, state.authorSceneThreshold)
			runOnUI(func() {
				progress.Hide()
				state.authorDetecting = false
				if err != nil {
					dialog.ShowError(err, state.window)
					return
				}
				if len(chapters) == 0 {
					dialog.ShowInformation("Scene Detection", "No scene changes detected at the current sensitivity.", state.window)
					return
				}
				// Show chapter preview dialog for visual verification
				state.showChapterPreview(targetPath, chapters, func(accepted bool) {
					if accepted {
						state.authorChapters = chapters
						state.authorChapterSource = "scenes"
						state.updateAuthorSummary()
						refreshChapters()
					}
				})
			})
		}()
	})
	detectBtn.Importance = widget.HighImportance

	addChapterBtn := widget.NewButton("+ Add Chapter", func() {
		dialog.ShowInformation("Add Chapter", "Manual chapter addition will be implemented.", state.window)
	})

	exportBtn := widget.NewButton("Export Chapters", func() {
		dialog.ShowInformation("Export", "Chapter export will be implemented", state.window)
	})

	controlsTop := container.NewVBox(
		fileLabel,
		selectBtn,
		widget.NewSeparator(),
		widget.NewLabel("Scene Detection:"),
		thresholdLabel,
		thresholdSlider,
		detectBtn,
		widget.NewSeparator(),
		widget.NewLabel("Chapters:"),
		sourceLabel,
	)

	listScroll := container.NewScroll(chapterList)
	bottomRow := container.NewHBox(addChapterBtn, exportBtn)

	controls := container.NewBorder(
		controlsTop,
		bottomRow,
		nil,
		nil,
		listScroll,
	)

	refreshChapters()
	return container.NewPadded(controls)
}

func buildSubtitlesTab(state *appState) fyne.CanvasObject {
	list := container.NewVBox()
	listScroll := container.NewVScroll(list)

	var buildSubList func()
	var emptyOverlay *fyne.Container
	buildSubList = func() {
		list.Objects = nil

		if len(state.authorSubtitles) == 0 {
			if emptyOverlay != nil {
				emptyOverlay.Show()
			}
			list.Refresh()
			return
		}

		if emptyOverlay != nil {
			emptyOverlay.Hide()
		}
		for i, path := range state.authorSubtitles {
			idx := i
			card := widget.NewCard(filepath.Base(path), "", nil)

			removeBtn := widget.NewButton("Remove", func() {
				state.authorSubtitles = append(state.authorSubtitles[:idx], state.authorSubtitles[idx+1:]...)
				buildSubList()
				state.updateAuthorSummary()
			})
			removeBtn.Importance = widget.MediumImportance

			cardContent := container.NewVBox(removeBtn)
			card.SetContent(cardContent)
			list.Add(card)
		}
		list.Refresh()
	}

	addBtn := widget.NewButton("Add Subtitles", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			state.authorSubtitles = append(state.authorSubtitles, reader.URI().Path())
			buildSubList()
			state.updateAuthorSummary()
		}, state.window)
	})
	addBtn.Importance = widget.HighImportance

	openSubtitlesBtn := widget.NewButton("Open Subtitles Tool", func() {
		if state.authorFile != nil {
			state.subtitleVideoPath = state.authorFile.Path
		} else if len(state.authorClips) > 0 {
			state.subtitleVideoPath = state.authorClips[0].Path
		}
		if len(state.authorSubtitles) > 0 {
			state.subtitleFilePath = state.authorSubtitles[0]
		}
		state.showSubtitlesView()
	})
	openSubtitlesBtn.Importance = widget.MediumImportance

	clearBtn := widget.NewButton("Clear All", func() {
		state.authorSubtitles = []string{}
		buildSubList()
		state.updateAuthorSummary()
	})
	clearBtn.Importance = widget.MediumImportance

	dropTarget := ui.NewDroppable(listScroll, func(items []fyne.URI) {
		var paths []string
		for _, uri := range items {
			if uri.Scheme() == "file" {
				paths = append(paths, uri.Path())
			}
		}
		if len(paths) > 0 {
			state.authorSubtitles = append(state.authorSubtitles, paths...)
			buildSubList()
			state.updateAuthorSummary()
		}
	})

	emptyLabel := widget.NewLabel("Drag and drop subtitle files here\nor click 'Add Subtitles' to select")
	emptyLabel.Alignment = fyne.TextAlignCenter
	emptyOverlay = container.NewCenter(emptyLabel)

	listArea := container.NewMax(dropTarget, emptyOverlay)

	controls := container.NewBorder(
		widget.NewLabel("Subtitle Tracks:"),
		container.NewHBox(addBtn, openSubtitlesBtn, clearBtn),
		nil,
		nil,
		listArea,
	)

	buildSubList()
	return container.NewPadded(controls)
}

func buildAuthorSettingsTab(state *appState) fyne.CanvasObject {
	outputType := widget.NewSelect([]string{"DVD (VIDEO_TS)", "ISO Image"}, func(value string) {
		if value == "DVD (VIDEO_TS)" {
			state.authorOutputType = "dvd"
		} else {
			state.authorOutputType = "iso"
		}
		state.updateAuthorSummary()
		state.persistAuthorConfig()
	})
	if state.authorOutputType == "iso" {
		outputType.SetSelected("ISO Image")
	} else {
		outputType.SetSelected("DVD (VIDEO_TS)")
	}

	regionSelect := widget.NewSelect([]string{"AUTO", "NTSC", "PAL"}, func(value string) {
		state.authorRegion = value
		state.updateAuthorSummary()
		state.persistAuthorConfig()
	})
	if state.authorRegion == "" {
		regionSelect.SetSelected("AUTO")
	} else {
		regionSelect.SetSelected(state.authorRegion)
	}

	aspectSelect := widget.NewSelect([]string{"AUTO", "4:3", "16:9"}, func(value string) {
		state.authorAspectRatio = value
		state.updateAuthorSummary()
		state.persistAuthorConfig()
	})
	if state.authorAspectRatio == "" {
		aspectSelect.SetSelected("AUTO")
	} else {
		aspectSelect.SetSelected(state.authorAspectRatio)
	}

	titleEntry := widget.NewEntry()
	titleEntry.SetPlaceHolder("DVD Title")
	titleEntry.SetText(state.authorTitle)
	titleEntry.OnChanged = func(value string) {
		state.authorTitle = value
		state.updateAuthorSummary()
		state.persistAuthorConfig()
	}

	createMenuCheck := widget.NewCheck("Create DVD Menu", func(checked bool) {
		state.authorCreateMenu = checked
		state.updateAuthorSummary()
		state.persistAuthorConfig()
	})
	createMenuCheck.SetChecked(state.authorCreateMenu)

	discSizeSelect := widget.NewSelect([]string{"DVD5", "DVD9"}, func(value string) {
		state.authorDiscSize = value
		state.updateAuthorSummary()
		state.persistAuthorConfig()
	})
	if state.authorDiscSize == "" {
		discSizeSelect.SetSelected("DVD5")
	} else {
		discSizeSelect.SetSelected(state.authorDiscSize)
	}

	applyControls := func() {
		if state.authorOutputType == "iso" {
			outputType.SetSelected("ISO Image")
		} else {
			outputType.SetSelected("DVD (VIDEO_TS)")
		}
		if state.authorRegion == "" {
			regionSelect.SetSelected("AUTO")
		} else {
			regionSelect.SetSelected(state.authorRegion)
		}
		if state.authorAspectRatio == "" {
			aspectSelect.SetSelected("AUTO")
		} else {
			aspectSelect.SetSelected(state.authorAspectRatio)
		}
		if state.authorDiscSize == "" {
			discSizeSelect.SetSelected("DVD5")
		} else {
			discSizeSelect.SetSelected(state.authorDiscSize)
		}
		titleEntry.SetText(state.authorTitle)
		createMenuCheck.SetChecked(state.authorCreateMenu)
	}

	loadCfgBtn := widget.NewButton("Load Config", func() {
		cfg, err := loadPersistedAuthorConfig()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				dialog.ShowInformation("No Config", "No saved config found yet. It will save automatically after your first change.", state.window)
			} else {
				dialog.ShowError(fmt.Errorf("failed to load config: %w", err), state.window)
			}
			return
		}
		state.applyAuthorConfig(cfg)
		applyControls()
		state.updateAuthorSummary()
	})

	saveCfgBtn := widget.NewButton("Save Config", func() {
		cfg := authorConfig{
			OutputType:      state.authorOutputType,
			Region:          state.authorRegion,
			AspectRatio:     state.authorAspectRatio,
			DiscSize:        state.authorDiscSize,
			Title:           state.authorTitle,
			CreateMenu:      state.authorCreateMenu,
			TreatAsChapters: state.authorTreatAsChapters,
			SceneThreshold:  state.authorSceneThreshold,
		}
		if err := savePersistedAuthorConfig(cfg); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save config: %w", err), state.window)
			return
		}
		dialog.ShowInformation("Config Saved", fmt.Sprintf("Saved to %s", moduleConfigPath("author")), state.window)
	})

	resetBtn := widget.NewButton("Reset", func() {
		cfg := defaultAuthorConfig()
		state.applyAuthorConfig(cfg)
		applyControls()
		state.updateAuthorSummary()
		state.persistAuthorConfig()
	})

	info := widget.NewLabel("Requires: ffmpeg, dvdauthor, and mkisofs/genisoimage (for ISO).")
	info.Wrapping = fyne.TextWrapWord

	controls := container.NewVBox(
		widget.NewLabel("Output Settings:"),
		widget.NewSeparator(),
		widget.NewLabel("Output Type:"),
		outputType,
		widget.NewLabel("Region:"),
		regionSelect,
		widget.NewLabel("Aspect Ratio:"),
		aspectSelect,
		widget.NewLabel("Disc Size:"),
		discSizeSelect,
		widget.NewLabel("DVD Title:"),
		titleEntry,
		createMenuCheck,
		widget.NewSeparator(),
		info,
		widget.NewSeparator(),
		container.NewHBox(resetBtn, loadCfgBtn, saveCfgBtn),
	)

	return container.NewPadded(controls)
}

func buildAuthorDiscTab(state *appState) fyne.CanvasObject {
	generateBtn := widget.NewButton("GENERATE DVD", func() {
		if len(state.authorClips) == 0 && state.authorFile == nil {
			dialog.ShowInformation("No Content", "Please add video clips or select a single video file", state.window)
			return
		}
		state.startAuthorGeneration(true)
	})
	generateBtn.Importance = widget.HighImportance

	summaryLabel := widget.NewLabel(authorSummary(state))
	summaryLabel.Wrapping = fyne.TextWrapWord
	state.authorSummaryLabel = summaryLabel

	statusLabel := widget.NewLabel("Ready")
	statusLabel.Wrapping = fyne.TextWrapWord
	state.authorStatusLabel = statusLabel

	progressBar := widget.NewProgressBar()
	progressBar.SetValue(state.authorProgress / 100.0)
	state.authorProgressBar = progressBar

	logEntry := widget.NewMultiLineEntry()
	logEntry.Wrapping = fyne.TextWrapOff
	logEntry.Disable()
	logEntry.SetText(state.authorLogText)
	state.authorLogEntry = logEntry
	logScroll := container.NewVScroll(logEntry)
	logScroll.SetMinSize(fyne.NewSize(0, 200))
	state.authorLogScroll = logScroll

	// Log control buttons
	copyLogBtn := widget.NewButton("Copy Log", func() {
		if state.authorLogFilePath != "" {
			// Copy from file for accuracy
			if data, err := os.ReadFile(state.authorLogFilePath); err == nil {
				state.window.Clipboard().SetContent(string(data))
				dialog.ShowInformation("Copied", "Full authoring log copied to clipboard", state.window)
				return
			}
		}
		// Fallback to in-memory log
		state.window.Clipboard().SetContent(state.authorLogText)
		dialog.ShowInformation("Copied", "Authoring log copied to clipboard", state.window)
	})
	copyLogBtn.Importance = widget.LowImportance

	viewFullLogBtn := widget.NewButton("View Full Log", func() {
		if state.authorLogFilePath == "" || state.authorLogFilePath == "-" {
			dialog.ShowInformation("No Log File", "No log file available to view", state.window)
			return
		}
		if _, err := os.Stat(state.authorLogFilePath); err != nil {
			dialog.ShowError(fmt.Errorf("log file not found: %w", err), state.window)
			return
		}
		state.openLogViewer("Authoring Log", state.authorLogFilePath, false)
	})
	viewFullLogBtn.Importance = widget.LowImportance

	logControls := container.NewHBox(
		widget.NewLabel("Authoring Log (last 100 lines):"),
		layout.NewSpacer(),
		copyLogBtn,
		viewFullLogBtn,
	)

	controls := container.NewVBox(
		widget.NewLabel("Generate DVD/ISO:"),
		widget.NewSeparator(),
		summaryLabel,
		widget.NewSeparator(),
		widget.NewLabel("Status:"),
		statusLabel,
		progressBar,
		widget.NewSeparator(),
		logControls,
		logScroll,
		widget.NewSeparator(),
		generateBtn,
	)

	return container.NewPadded(controls)
}

func authorSummary(state *appState) string {
	summary := "Ready to generate:\n\n"
	if state.authorVideoTSPath != "" {
		summary += fmt.Sprintf("VIDEO_TS: %s\n", filepath.Base(filepath.Dir(state.authorVideoTSPath)))
	} else if len(state.authorClips) > 0 {
		summary += fmt.Sprintf("Videos: %d\n", len(state.authorClips))
		for i, clip := range state.authorClips {
			summary += fmt.Sprintf("  %d. %s (%.2fs)\n", i+1, clip.DisplayName, clip.Duration)
		}
	} else if state.authorFile != nil {
		summary += fmt.Sprintf("Video File: %s\n", filepath.Base(state.authorFile.Path))
	}

	if len(state.authorSubtitles) > 0 {
		summary += fmt.Sprintf("Subtitle Tracks: %d\n", len(state.authorSubtitles))
		for i, path := range state.authorSubtitles {
			summary += fmt.Sprintf("  %d. %s\n", i+1, filepath.Base(path))
		}
	}

	if count, label := state.authorChapterSummary(); count > 0 {
		summary += fmt.Sprintf("%s: %d\n", label, count)
	}

	summary += fmt.Sprintf("Output Type: %s\n", state.authorOutputType)
	summary += fmt.Sprintf("Disc Size: %s\n", state.authorDiscSize)
	summary += fmt.Sprintf("Region: %s\n", state.authorRegion)
	summary += fmt.Sprintf("Aspect Ratio: %s\n", state.authorAspectRatio)
	if outPath := authorDefaultOutputPath(state.authorOutputType, authorOutputTitle(state), authorSummaryPaths(state)); outPath != "" {
		summary += fmt.Sprintf("Output Path: %s\n", outPath)
	}
	if state.authorTitle != "" {
		summary += fmt.Sprintf("DVD Title: %s\n", state.authorTitle)
	}
	if totalDur := authorTotalDuration(state); totalDur > 0 {
		bitrate := authorTargetBitrateKbps(state.authorDiscSize, totalDur)
		summary += fmt.Sprintf("Estimated Target Bitrate: %dkbps\n", bitrate)
	}
	return summary
}

func (s *appState) addAuthorFiles(paths []string) {
	wasEmpty := len(s.authorClips) == 0
	for _, path := range paths {
		src, err := probeVideo(path)
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to load video %s: %w", filepath.Base(path), err), s.window)
			continue
		}

		clip := authorClip{
			Path:         path,
			DisplayName:  filepath.Base(path),
			Duration:     src.Duration,
			Chapters:     []authorChapter{},
			ChapterTitle: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		}
		s.authorClips = append(s.authorClips, clip)
	}

	if wasEmpty && len(s.authorClips) == 1 {
		s.loadEmbeddedChapters(s.authorClips[0].Path)
	} else if len(s.authorClips) > 1 && s.authorChapterSource == "embedded" {
		s.authorChapters = nil
		s.authorChapterSource = ""
	}
	s.authorTitle = ""
	s.updateAuthorSummary()
	// Update the UI for the title entry if the settings tab is currently visible.
	// This ensures the title entry visually resets as well.
	if s.active == "author" && s.window.Canvas() != nil {
		app := fyne.CurrentApp()
		if app != nil && app.Driver() != nil {
			app.Driver().DoFromGoroutine(func() {
				// Rebuild the settings tab to refresh its controls.
				// This is a bit heavy, but ensures the titleEntry reflects the change.
				s.showAuthorView()
			}, false)
		}
	}
}

func (s *appState) updateAuthorSummary() {
	if s.authorSummaryLabel == nil {
		return
	}
	s.authorSummaryLabel.SetText(authorSummary(s))
}

func (s *appState) authorChapterSummary() (int, string) {
	if len(s.authorChapters) > 0 {
		switch s.authorChapterSource {
		case "embedded":
			return len(s.authorChapters), "Embedded Chapters"
		case "scenes":
			return len(s.authorChapters), "Scene Chapters"
		default:
			return len(s.authorChapters), "Chapters"
		}
	}
	if s.authorTreatAsChapters && len(s.authorClips) > 1 {
		return len(s.authorClips), "Clip Chapters"
	}
	return 0, ""
}

func authorTotalDuration(state *appState) float64 {
	if len(state.authorClips) > 0 {
		var total float64
		for _, clip := range state.authorClips {
			total += clip.Duration
		}
		return total
	}
	if state.authorFile != nil {
		return state.authorFile.Duration
	}
	return 0
}

func authorSummaryPaths(state *appState) []string {
	if state.authorVideoTSPath != "" {
		return []string{state.authorVideoTSPath}
	}
	if len(state.authorClips) > 0 {
		paths := make([]string, 0, len(state.authorClips))
		for _, clip := range state.authorClips {
			paths = append(paths, clip.Path)
		}
		return paths
	}
	if state.authorFile != nil {
		return []string{state.authorFile.Path}
	}
	return nil
}

func authorOutputTitle(state *appState) string {
	title := strings.TrimSpace(state.authorTitle)
	if title != "" {
		return title
	}
	if state.authorVideoTSPath != "" {
		return filepath.Base(filepath.Dir(state.authorVideoTSPath))
	}
	return defaultAuthorTitle(authorSummaryPaths(state))
}

func authorTargetBitrateKbps(discSize string, totalSeconds float64) int {
	if totalSeconds <= 0 {
		return 0
	}
	var targetBytes float64
	switch strings.ToUpper(strings.TrimSpace(discSize)) {
	case "DVD9":
		targetBytes = 7.3 * 1024 * 1024 * 1024
	default:
		targetBytes = 4.1 * 1024 * 1024 * 1024
	}
	totalBits := targetBytes * 8
	kbps := int(totalBits / totalSeconds / 1000)
	if kbps > 9500 {
		kbps = 9500
	}
	if kbps < 1500 {
		kbps = 1500
	}
	return kbps
}

func (s *appState) loadEmbeddedChapters(path string) {
	chapters, err := extractChaptersFromFile(path)
	if err != nil || len(chapters) == 0 {
		if s.authorChapterSource == "embedded" {
			s.authorChapters = nil
			s.authorChapterSource = ""
			s.updateAuthorSummary()
			if s.authorChaptersRefresh != nil {
				s.authorChaptersRefresh()
			}
		}
		return
	}
	s.authorChapters = chapters
	s.authorChapterSource = "embedded"
	s.updateAuthorSummary()
	if s.authorChaptersRefresh != nil {
		s.authorChaptersRefresh()
	}
}

func chaptersFromClips(clips []authorClip) []authorChapter {
	if len(clips) == 0 {
		return nil
	}
	var chapters []authorChapter
	var t float64
	firstTitle := strings.TrimSpace(clips[0].ChapterTitle)
	if firstTitle == "" {
		firstTitle = "Chapter 1"
	}
	chapters = append(chapters, authorChapter{Timestamp: 0, Title: firstTitle, Auto: true})
	for i := 1; i < len(clips); i++ {
		t += clips[i-1].Duration
		title := strings.TrimSpace(clips[i].ChapterTitle)
		if title == "" {
			title = fmt.Sprintf("Chapter %d", i+1)
		}
		chapters = append(chapters, authorChapter{
			Timestamp: t,
			Title:     title,
			Auto:      true,
		})
	}
	return chapters
}

func detectSceneChapters(path string, threshold float64) ([]authorChapter, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	filter := fmt.Sprintf("select='gt(scene,%.2f)',showinfo", threshold)
	cmd := exec.CommandContext(ctx, platformConfig.FFmpegPath,
		"-hide_banner",
		"-loglevel", "info",
		"-i", path,
		"-vf", filter,
		"-an",
		"-f", "null",
		"-",
	)
	utils.ApplyNoWindow(cmd)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	times := map[float64]struct{}{}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		idx := strings.Index(line, "pts_time:")
		if idx == -1 {
			continue
		}
		rest := line[idx+len("pts_time:"):]
		end := strings.IndexAny(rest, " ")
		if end == -1 {
			end = len(rest)
		}
		valStr := strings.TrimSpace(rest[:end])
		if valStr == "" {
			continue
		}
		if val, err := utils.ParseFloat(valStr); err == nil {
			times[val] = struct{}{}
		}
	}

	var vals []float64
	for v := range times {
		if v < 0.01 {
			continue
		}
		vals = append(vals, v)
	}
	sort.Float64s(vals)

	if len(vals) == 0 {
		if err != nil {
			return nil, fmt.Errorf("scene detection failed: %s", strings.TrimSpace(string(out)))
		}
		return nil, nil
	}

	chapters := []authorChapter{{Timestamp: 0, Title: "Chapter 1", Auto: true}}
	for i, v := range vals {
		chapters = append(chapters, authorChapter{
			Timestamp: v,
			Title:     fmt.Sprintf("Chapter %d", i+2),
			Auto:      true,
		})
	}
	return chapters, nil
}

func extractChaptersFromFile(path string) ([]authorChapter, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, platformConfig.FFprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_chapters",
		path,
	)
	utils.ApplyNoWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var result struct {
		Chapters []struct {
			StartTime string                 `json:"start_time"`
			Tags      map[string]interface{} `json:"tags"`
		} `json:"chapters"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, err
	}

	var chapters []authorChapter
	for i, ch := range result.Chapters {
		t, err := utils.ParseFloat(ch.StartTime)
		if err != nil {
			continue
		}
		title := ""
		if ch.Tags != nil {
			if v, ok := ch.Tags["title"]; ok {
				title = fmt.Sprintf("%v", v)
			}
		}
		if title == "" {
			title = fmt.Sprintf("Chapter %d", i+1)
		}
		chapters = append(chapters, authorChapter{
			Timestamp: t,
			Title:     title,
			Auto:      true,
		})
	}

	return chapters, nil
}

func chaptersToDVDAuthor(chapters []authorChapter) string {
	if len(chapters) == 0 {
		return ""
	}
	var times []float64
	for _, ch := range chapters {
		if ch.Timestamp < 0 {
			continue
		}
		times = append(times, ch.Timestamp)
	}
	if len(times) == 0 {
		return ""
	}
	sort.Float64s(times)
	if times[0] > 0.01 {
		times = append([]float64{0}, times...)
	}
	seen := map[int]struct{}{}
	var parts []string
	for _, t := range times {
		key := int(t * 1000)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		parts = append(parts, formatChapterTime(t))
	}
	return strings.Join(parts, ",")
}

func formatChapterTime(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	d := time.Duration(sec * float64(time.Second))
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func concatDVDMpg(inputs []string, output string) error {
	listPath := filepath.Join(filepath.Dir(output), "concat_list.txt")
	listFile, err := os.Create(listPath)
	if err != nil {
		return fmt.Errorf("failed to create concat list: %w", err)
	}
	for _, path := range inputs {
		fmt.Fprintf(listFile, "file '%s'\n", strings.ReplaceAll(path, "'", "'\\''"))
	}
	if err := listFile.Close(); err != nil {
		return fmt.Errorf("failed to write concat list: %w", err)
	}
	defer os.Remove(listPath)

	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-f", "concat",
		"-safe", "0",
		"-i", listPath,
		"-c", "copy",
		"-f", "dvd",           // Maintain DVD format
		"-muxrate", "10080000", // DVD mux rate
		"-packetsize", "2048",  // DVD packet size
		output,
	}
	return runCommand(platformConfig.FFmpegPath, args)
}

func (s *appState) resetAuthorLog() {
	s.authorLogText = ""
	s.authorLogLines = nil
	s.authorLogFilePath = ""
	if s.authorLogEntry != nil {
		s.authorLogEntry.SetText("")
	}
	if s.authorLogScroll != nil {
		s.authorLogScroll.ScrollToTop()
	}
}

func (s *appState) appendAuthorLog(line string) {
	if strings.TrimSpace(line) == "" {
		return
	}

	// Keep only last 100 lines for UI display (tail behavior)
	const maxLines = 100
	s.authorLogLines = append(s.authorLogLines, line)
	if len(s.authorLogLines) > maxLines {
		s.authorLogLines = s.authorLogLines[len(s.authorLogLines)-maxLines:]
	}

	// Rebuild text from buffer
	s.authorLogText = strings.Join(s.authorLogLines, "\n")

	if s.authorLogEntry != nil {
		s.authorLogEntry.SetText(s.authorLogText)
	}
	if s.authorLogScroll != nil {
		s.authorLogScroll.ScrollToBottom()
	}
}

func (s *appState) setAuthorStatus(text string) {
	if text == "" {
		text = "Ready"
	}
	if s.authorStatusLabel != nil {
		s.authorStatusLabel.SetText(text)
	}
}

func (s *appState) setAuthorProgress(percent float64) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	s.authorProgress = percent
	if s.authorProgressBar != nil {
		s.authorProgressBar.SetValue(percent / 100.0)
	}
}

func (s *appState) startAuthorGeneration(startNow bool) {
	if s.authorVideoTSPath != "" {
		title := authorOutputTitle(s)
		outputPath := authorDefaultOutputPath("iso", title, []string{s.authorVideoTSPath})
		if outputPath == "" {
			dialog.ShowError(fmt.Errorf("failed to resolve output path"), s.window)
			return
		}
		if err := s.addAuthorVideoTSToQueue(s.authorVideoTSPath, title, outputPath, startNow); err != nil {
			dialog.ShowError(err, s.window)
		}
		return
	}

	paths, primary, err := s.authorSourcePaths()
	if err != nil {
		dialog.ShowError(err, s.window)
		return
	}

	region := resolveAuthorRegion(s.authorRegion, primary)
	aspect := resolveAuthorAspect(s.authorAspectRatio, primary)
	title := strings.TrimSpace(s.authorTitle)
	if title == "" {
		title = defaultAuthorTitle(paths)
	}

	warnings := authorWarnings(s)
	uiCall := func(fn func()) {
		app := fyne.CurrentApp()
		if app != nil && app.Driver() != nil {
			app.Driver().DoFromGoroutine(fn, false)
			return
		}
		fn()
	}
	continuePrompt := func() {
		uiCall(func() {
			s.promptAuthorOutput(paths, region, aspect, title, startNow)
		})
	}
	if len(warnings) > 0 {
		uiCall(func() {
			dialog.ShowConfirm("Authoring Notes", strings.Join(warnings, "\n")+"\n\nContinue?", func(ok bool) {
				if ok {
					continuePrompt()
				}
			}, s.window)
		})
		return
	}

	continuePrompt()
}

func (s *appState) promptAuthorOutput(paths []string, region, aspect, title string, startNow bool) {
	outputType := strings.ToLower(strings.TrimSpace(s.authorOutputType))
	if outputType == "" {
		outputType = "dvd"
	}

	outputPath := authorDefaultOutputPath(outputType, title, paths)
	if outputType == "iso" {
		s.generateAuthoring(paths, region, aspect, title, outputPath, true, startNow)
		return
	}
	s.generateAuthoring(paths, region, aspect, title, outputPath, false, startNow)
}

func authorWarnings(state *appState) []string {
	var warnings []string
	if state.authorCreateMenu {
		warnings = append(warnings, "DVD menus are not implemented yet; the disc will play titles directly.")
	}
	if len(state.authorSubtitles) > 0 {
		warnings = append(warnings, "Subtitle tracks are not authored yet; they will be ignored.")
	}
	if len(state.authorAudioTracks) > 0 {
		warnings = append(warnings, "Additional audio tracks are not authored yet; they will be ignored.")
	}
	if totalDur := authorTotalDuration(state); totalDur > 0 {
		bitrate := authorTargetBitrateKbps(state.authorDiscSize, totalDur)
		if bitrate < 3000 {
			warnings = append(warnings, fmt.Sprintf("Long runtime detected; target bitrate ~%dkbps may reduce quality.", bitrate))
		}
	}
	return warnings
}

func (s *appState) authorSourcePaths() ([]string, *videoSource, error) {
	if len(s.authorClips) > 0 {
		paths := make([]string, 0, len(s.authorClips))
		for _, clip := range s.authorClips {
			paths = append(paths, clip.Path)
		}
		primary, err := probeVideo(paths[0])
		if err != nil {
			return nil, nil, fmt.Errorf("failed to probe source: %w", err)
		}
		return paths, primary, nil
	}

	if s.authorFile != nil {
		return []string{s.authorFile.Path}, s.authorFile, nil
	}

	return nil, nil, fmt.Errorf("no authoring content selected")
}

func resolveAuthorRegion(pref string, src *videoSource) string {
	pref = strings.ToUpper(strings.TrimSpace(pref))
	if pref == "NTSC" || pref == "PAL" {
		return pref
	}
	if src != nil {
		if src.FrameRate > 0 {
			if src.FrameRate <= 26 {
				return "PAL"
			}
			return "NTSC"
		}
		if src.Height == 576 {
			return "PAL"
		}
		if src.Height == 480 {
			return "NTSC"
		}
	}
	return "NTSC"
}

func resolveAuthorAspect(pref string, src *videoSource) string {
	pref = strings.TrimSpace(pref)
	if pref == "4:3" || pref == "16:9" {
		return pref
	}
	if src != nil && src.Width > 0 && src.Height > 0 {
		ratio := float64(src.Width) / float64(src.Height)
		if ratio >= 1.55 {
			return "16:9"
		}
		return "4:3"
	}
	return "16:9"
}

func defaultAuthorTitle(paths []string) string {
	if len(paths) == 0 {
		return "DVD"
	}
	base := filepath.Base(paths[0])
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func authorOutputFolderName(title string, paths []string) string {
	name := strings.TrimSpace(title)
	if name == "" {
		name = defaultAuthorTitle(paths)
	}
	name = sanitizeForPath(name)
	if name == "" {
		name = "dvd_output"
	}
	return name
}

func authorDefaultOutputDir(outputType string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	dir := filepath.Join(home, "Videos", "VideoTools")
	if strings.EqualFold(outputType, "iso") {
		return filepath.Join(dir, "ISO_Convert")
	}
	return filepath.Join(dir, "DVD_Convert")
}

func authorDefaultOutputPath(outputType, title string, paths []string) string {
	outputType = strings.ToLower(strings.TrimSpace(outputType))
	if outputType == "" {
		outputType = "dvd"
	}
	baseDir := authorDefaultOutputDir(outputType)
	name := strings.TrimSpace(title)
	if name == "" {
		name = defaultAuthorTitle(paths)
	}
	name = sanitizeForPath(name)
	if name == "" {
		name = "dvd_output"
	}
	if outputType == "iso" {
		return uniqueFilePath(filepath.Join(baseDir, name+".iso"))
	}
	return uniqueFolderPath(filepath.Join(baseDir, name))
}

func authorTempRoot(outputPath string) string {
	trimmed := strings.TrimSpace(outputPath)
	if trimmed == "" {
		return utils.TempDir()
	}
	lower := strings.ToLower(trimmed)
	root := trimmed
	if strings.HasSuffix(lower, ".iso") {
		root = filepath.Dir(trimmed)
	} else if ext := filepath.Ext(trimmed); ext != "" {
		root = filepath.Dir(trimmed)
	}
	if root == "" || root == "." {
		return utils.TempDir()
	}
	return root
}

func uniqueFolderPath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	for i := 1; i < 1000; i++ {
		tryPath := fmt.Sprintf("%s-%d", path, i)
		if _, err := os.Stat(tryPath); os.IsNotExist(err) {
			return tryPath
		}
	}
	return fmt.Sprintf("%s-%d", path, time.Now().Unix())
}

func uniqueFilePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 1; i < 1000; i++ {
		tryPath := fmt.Sprintf("%s-%d%s", base, i, ext)
		if _, err := os.Stat(tryPath); os.IsNotExist(err) {
			return tryPath
		}
	}
	return fmt.Sprintf("%s-%d%s", base, time.Now().Unix(), ext)
}

func (s *appState) generateAuthoring(paths []string, region, aspect, title, outputPath string, makeISO, startNow bool) {
	if err := s.addAuthorToQueue(paths, region, aspect, title, outputPath, makeISO, startNow); err != nil {
		dialog.ShowError(err, s.window)
	}
}

func (s *appState) addAuthorToQueue(paths []string, region, aspect, title, outputPath string, makeISO bool, startNow bool) error {
	if s.jobQueue == nil {
		return fmt.Errorf("queue not initialized")
	}

	clips := make([]map[string]interface{}, 0, len(s.authorClips))
	for _, clip := range s.authorClips {
		clips = append(clips, map[string]interface{}{
			"path":         clip.Path,
			"displayName":  clip.DisplayName,
			"duration":     clip.Duration,
			"chapterTitle": clip.ChapterTitle,
		})
	}
	chapters := make([]map[string]interface{}, 0, len(s.authorChapters))
	for _, ch := range s.authorChapters {
		chapters = append(chapters, map[string]interface{}{
			"timestamp": ch.Timestamp,
			"title":     ch.Title,
			"auto":      ch.Auto,
		})
	}

	config := map[string]interface{}{
		"paths":            paths,
		"region":           region,
		"aspect":           aspect,
		"title":            title,
		"outputPath":       outputPath,
		"makeISO":          makeISO,
		"treatAsChapters":  s.authorTreatAsChapters,
		"clips":            clips,
		"chapters":         chapters,
		"discSize":         s.authorDiscSize,
		"outputType":       s.authorOutputType,
		"authorTitle":      s.authorTitle,
		"authorRegion":     s.authorRegion,
		"authorAspect":     s.authorAspectRatio,
		"chapterSource":    s.authorChapterSource,
		"subtitleTracks":   append([]string{}, s.authorSubtitles...),
		"additionalAudios": append([]string{}, s.authorAudioTracks...),
	}

	titleLabel := title
	if strings.TrimSpace(titleLabel) == "" {
		titleLabel = "DVD"
	}
	job := &queue.Job{
		Type:        queue.JobTypeAuthor,
		Title:       fmt.Sprintf("Author DVD: %s", titleLabel),
		Description: fmt.Sprintf("Output: %s", utils.ShortenMiddle(filepath.Base(outputPath), 40)),
		InputFile:   paths[0],
		OutputFile:  outputPath,
		Config:      config,
	}

	s.resetAuthorLog()
	s.setAuthorStatus("Queued authoring job...")
	s.setAuthorProgress(0)
	s.jobQueue.Add(job)
	if startNow && !s.jobQueue.IsRunning() {
		s.jobQueue.Start()
	}
	return nil
}

func (s *appState) addAuthorVideoTSToQueue(videoTSPath, title, outputPath string, startNow bool) error {
	if s.jobQueue == nil {
		return fmt.Errorf("queue not initialized")
	}
	job := &queue.Job{
		Type:        queue.JobTypeAuthor,
		Title:       fmt.Sprintf("Author ISO: %s", title),
		Description: fmt.Sprintf("VIDEO_TS -> %s", utils.ShortenMiddle(filepath.Base(outputPath), 40)),
		InputFile:   videoTSPath,
		OutputFile:  outputPath,
		Config: map[string]interface{}{
			"videoTSPath": videoTSPath,
			"outputPath":  outputPath,
			"makeISO":     true,
			"title":       title,
		},
	}

	s.resetAuthorLog()
	s.setAuthorStatus("Queued authoring job...")
	s.setAuthorProgress(0)
	s.jobQueue.Add(job)
	if startNow && !s.jobQueue.IsRunning() {
		s.jobQueue.Start()
	}
	return nil
}

func (s *appState) runAuthoringPipeline(ctx context.Context, paths []string, region, aspect, title, outputPath string, makeISO bool, clips []authorClip, chapters []authorChapter, treatAsChapters bool, logFn func(string), progressFn func(float64)) error {
	tempRoot := authorTempRoot(outputPath)
	if err := os.MkdirAll(tempRoot, 0755); err != nil {
		return fmt.Errorf("failed to create temp root: %w", err)
	}
	workDir, err := os.MkdirTemp(tempRoot, "videotools-author-")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(workDir)
	if logFn != nil {
		logFn(fmt.Sprintf("Temp workspace: %s", workDir))
	}

	discRoot := outputPath
	var cleanup func()
	if makeISO {
		tempRoot, err := os.MkdirTemp(tempRoot, "videotools-dvd-")
		if err != nil {
			return fmt.Errorf("failed to create DVD output directory: %w", err)
		}
		discRoot = tempRoot
		cleanup = func() {
			_ = os.RemoveAll(tempRoot)
		}
	}
	if cleanup != nil {
		defer cleanup()
	}

	if err := prepareDiscRoot(discRoot); err != nil {
		return err
	}

	var totalDuration float64
	for _, path := range paths {
		src, err := probeVideo(path)
		if err == nil {
			totalDuration += src.Duration
		}
	}

	encodingProgressShare := 80.0
	otherStepsProgressShare := 20.0
	otherStepsCount := 2.0
	if makeISO {
		otherStepsCount++
	}
	progressForOtherStep := otherStepsProgressShare / otherStepsCount
	var accumulatedProgress float64

	var mpgPaths []string
	for i, path := range paths {
		if logFn != nil {
			logFn(fmt.Sprintf("Encoding %d/%d: %s", i+1, len(paths), filepath.Base(path)))
		}
		outPath := filepath.Join(workDir, fmt.Sprintf("title_%02d.mpg", i+1))
		src, err := probeVideo(path)
		if err != nil {
			return fmt.Errorf("failed to probe %s: %w", filepath.Base(path), err)
		}

		clipProgressShare := 0.0
		if totalDuration > 0 {
			clipProgressShare = (src.Duration / totalDuration) * encodingProgressShare
		}

		ffmpegProgressFn := func(stepPct float64) {
			overallPct := accumulatedProgress + (stepPct / 100.0 * clipProgressShare)
			if progressFn != nil {
				progressFn(overallPct)
			}
		}

		args := buildAuthorFFmpegArgs(path, outPath, region, aspect, src.IsProgressive())
		if logFn != nil {
			logFn(fmt.Sprintf(">> ffmpeg %s", strings.Join(args, " ")))
		}

		if err := runAuthorFFmpeg(ctx, args, src.Duration, logFn, ffmpegProgressFn); err != nil {
			return err
		}

		accumulatedProgress += clipProgressShare
		if progressFn != nil {
			progressFn(accumulatedProgress)
		}

		remuxPath := filepath.Join(workDir, fmt.Sprintf("title_%02d_remux.mpg", i+1))
		remuxArgs := []string{
			"-fflags", "+genpts",
			"-i", outPath,
			"-c", "copy",
			"-f", "dvd",
			"-muxrate", "10080000",
			"-packetsize", "2048",
			"-y", remuxPath,
		}
		if logFn != nil {
			logFn(fmt.Sprintf(">> ffmpeg %s (remuxing for DVD compliance)", strings.Join(remuxArgs, " ")))
		}
		if err := runCommandWithLogger(ctx, platformConfig.FFmpegPath, remuxArgs, logFn); err != nil {
			return fmt.Errorf("remux failed: %w", err)
		}
		os.Remove(outPath)
		mpgPaths = append(mpgPaths, remuxPath)
	}

	// Generate clips from paths if clips is empty (fallback for when job didn't save clips)
	if len(clips) == 0 && len(paths) > 1 {
		for i, path := range paths {
			src, err := probeVideo(path)
			duration := 0.0
			displayName := filepath.Base(path)
			if err == nil {
				duration = src.Duration
				displayName = src.DisplayName
			}
			clips = append(clips, authorClip{
				Path:         path,
				DisplayName:  displayName,
				Duration:     duration,
				ChapterTitle: fmt.Sprintf("Chapter %d", i+1),
			})
		}
		if logFn != nil {
			logFn(fmt.Sprintf("Generated %d clips from input paths for chapter markers", len(clips)))
		}
	}

	// Generate chapters from clips if available (for professional DVD navigation)
	if len(chapters) == 0 && len(clips) > 1 {
		chapters = chaptersFromClips(clips)
		if logFn != nil {
			logFn(fmt.Sprintf("Generated %d chapter markers from video clips", len(chapters)))
		}
	}

	// Try to extract embedded chapters from single file
	if len(chapters) == 0 && len(mpgPaths) == 1 {
		if embed, err := extractChaptersFromFile(paths[0]); err == nil && len(embed) > 0 {
			chapters = embed
			if logFn != nil {
				logFn(fmt.Sprintf("Extracted %d embedded chapters from source", len(chapters)))
			}
		}
	}

	// For professional DVD: always concatenate multiple files into one title with chapters
	if len(mpgPaths) > 1 {
		concatPath := filepath.Join(workDir, "titles_joined.mpg")
		if logFn != nil {
			logFn(fmt.Sprintf("Combining %d videos into single title with chapter markers...", len(mpgPaths)))
		}
		if err := concatDVDMpg(mpgPaths, concatPath); err != nil {
			return fmt.Errorf("failed to concatenate videos: %w", err)
		}
		mpgPaths = []string{concatPath}
	}

	// Log details about encoded MPG files
	if logFn != nil {
		logFn(fmt.Sprintf("Created %d MPEG file(s):", len(mpgPaths)))
		for i, mpg := range mpgPaths {
			if info, err := os.Stat(mpg); err == nil {
				logFn(fmt.Sprintf("  %d. %s (%d bytes)", i+1, filepath.Base(mpg), info.Size()))
			} else {
				logFn(fmt.Sprintf("  %d. %s (stat failed: %v)", i+1, filepath.Base(mpg), err))
			}
		}
	}

	xmlPath := filepath.Join(workDir, "dvd.xml")
	if err := writeDVDAuthorXML(xmlPath, mpgPaths, region, aspect, chapters); err != nil {
		return err
	}

	// Log chapter information
	if len(chapters) > 0 {
		if logFn != nil {
			logFn(fmt.Sprintf("Final DVD structure: 1 title with %d chapters", len(chapters)))
			for i, ch := range chapters {
				logFn(fmt.Sprintf("  Chapter %d: %s at %s", i+1, ch.Title, formatChapterTime(ch.Timestamp)))
			}
		}
	}

	// Log the XML content for debugging
	if xmlContent, err := os.ReadFile(xmlPath); err == nil {
		logFn("Generated DVD XML:")
		logFn(string(xmlContent))
	}

	logFn("Authoring DVD structure...")
	logFn(fmt.Sprintf(">> dvdauthor -o %s -x %s", discRoot, xmlPath))
	if err := runCommandWithLogger(ctx, "dvdauthor", []string{"-o", discRoot, "-x", xmlPath}, logFn); err != nil {
		logFn(fmt.Sprintf("ERROR: dvdauthor failed: %v", err))
		return fmt.Errorf("dvdauthor structure creation failed: %w", err)
	}
	accumulatedProgress += progressForOtherStep
	progressFn(accumulatedProgress)

	logFn("Building DVD tables...")
	logFn(fmt.Sprintf(">> dvdauthor -o %s -T", discRoot))
	if err := runCommandWithLogger(ctx, "dvdauthor", []string{"-o", discRoot, "-T"}, logFn); err != nil {
		logFn(fmt.Sprintf("ERROR: dvdauthor -T failed: %v", err))
		return fmt.Errorf("dvdauthor table build failed: %w", err)
	}
	accumulatedProgress += progressForOtherStep
	progressFn(accumulatedProgress)

	if err := os.MkdirAll(filepath.Join(discRoot, "AUDIO_TS"), 0755); err != nil {
		return fmt.Errorf("failed to create AUDIO_TS: %w", err)
	}

	if makeISO {
		// Create output directory for ISO file if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return fmt.Errorf("failed to create ISO output directory: %w", err)
		}
		tool, args, err := buildISOCommand(outputPath, discRoot, title)
		if err != nil {
			logFn(fmt.Sprintf("ERROR: ISO tool not found: %v", err))
			return fmt.Errorf("ISO creation setup failed: %w", err)
		}
		logFn("Creating ISO image...")
		logFn(fmt.Sprintf(">> %s %s", tool, strings.Join(args, " ")))
		if err := runCommandWithLogger(ctx, tool, args, logFn); err != nil {
			logFn(fmt.Sprintf("ERROR: ISO creation failed: %v", err))
			return fmt.Errorf("ISO creation failed: %w", err)
		}
		accumulatedProgress += progressForOtherStep
		progressFn(accumulatedProgress)

		// Verify ISO was created
		if info, err := os.Stat(outputPath); err == nil {
			logFn(fmt.Sprintf("ISO created successfully: %s (%d bytes)", filepath.Base(outputPath), info.Size()))
		} else {
			logFn(fmt.Sprintf("WARNING: ISO file verification failed: %v", err))
		}
	}

	progressFn(100.0)
	return nil
}

func runAuthorFFmpeg(ctx context.Context, args []string, duration float64, logFn func(string), progressFn func(float64)) error {
	finalArgs := append([]string{"-progress", "pipe:1", "-nostats"}, args...)
	cmd := exec.CommandContext(ctx, platformConfig.FFmpegPath, finalArgs...)
	utils.ApplyNoWindow(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start failed: %w", err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			if logFn != nil {
				logFn(scanner.Text())
			}
		}
	}()
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.SplitN(line, "=", 2)
			if len(parts) < 2 {
				continue
			}
			key, val := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			if key == "out_time_ms" {
				if ms, err := strconv.ParseInt(val, 10, 64); err == nil && ms > 0 {
					currentSec := float64(ms) / 1000000.0
					if duration > 0 {
						stepPct := (currentSec / duration) * 100.0
						if stepPct > 100 {
							stepPct = 100
						}
						if progressFn != nil {
							progressFn(stepPct)
						}
					}
				}
			}
			if logFn != nil {
				logFn(line)
			}
		}
	}()
	err = cmd.Wait()
	wg.Wait()
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w", err)
	}
	return nil
}


func (s *appState) executeAuthorJob(ctx context.Context, job *queue.Job, progressCallback func(float64)) error {
	cfg := job.Config
	if cfg == nil {
		return fmt.Errorf("author job config missing")
	}
	if videoTSPath := strings.TrimSpace(toString(cfg["videoTSPath"])); videoTSPath != "" {
		outputPath := toString(cfg["outputPath"])
		title := toString(cfg["title"])
		if err := ensureAuthorDependencies(true); err != nil {
			return err
		}

		logFile, logPath, logErr := createAuthorLog([]string{videoTSPath}, outputPath, true, "", "", title)
		if logErr != nil {
			logging.Debug(logging.CatSystem, "author log open failed: %v", logErr)
		} else {
			job.LogPath = logPath
			s.authorLogFilePath = logPath // Store for UI access
			defer logFile.Close()
		}

		appendLog := func(line string) {
			if logFile != nil {
				fmt.Fprintln(logFile, line)
			}
			app := fyne.CurrentApp()
			if app != nil && app.Driver() != nil {
				app.Driver().DoFromGoroutine(func() {
					s.appendAuthorLog(line)
				}, false)
			}
		}

		updateProgress := func(percent float64) {
			progressCallback(percent)
			app := fyne.CurrentApp()
			if app != nil && app.Driver() != nil {
				app.Driver().DoFromGoroutine(func() {
					s.setAuthorProgress(percent)
				}, false)
			}
		}

		appendLog(fmt.Sprintf("Authoring ISO from VIDEO_TS: %s", videoTSPath))
		// Create output directory for ISO file if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return fmt.Errorf("failed to create ISO output directory: %w", err)
		}
		tool, args, err := buildISOCommand(outputPath, videoTSPath, title)
		if err != nil {
			return err
		}
		appendLog(fmt.Sprintf(">> %s %s", tool, strings.Join(args, " ")))
		updateProgress(10)
		if err := runCommandWithLogger(ctx, tool, args, appendLog); err != nil {
			return err
		}
		updateProgress(100)
		appendLog("ISO creation completed successfully.")
		return nil
	}

	rawPaths, _ := cfg["paths"].([]interface{})
	var paths []string
	for _, p := range rawPaths {
		paths = append(paths, toString(p))
	}
	if len(paths) == 0 {
		if path, ok := cfg["paths"].([]string); ok {
			paths = append(paths, path...)
		}
	}
	if len(paths) == 0 {
		if input, ok := cfg["inputPath"].(string); ok && input != "" {
			paths = append(paths, input)
		}
	}
	if len(paths) == 0 {
		return fmt.Errorf("no input paths for author job")
	}

	region := toString(cfg["region"])
	aspect := toString(cfg["aspect"])
	title := toString(cfg["title"])
	outputPath := toString(cfg["outputPath"])
	makeISO, _ := cfg["makeISO"].(bool)
	treatAsChapters, _ := cfg["treatAsChapters"].(bool)

	if err := ensureAuthorDependencies(makeISO); err != nil {
		return err
	}

	var clips []authorClip
	if rawClips, ok := cfg["clips"].([]interface{}); ok {
		for _, rc := range rawClips {
			if m, ok := rc.(map[string]interface{}); ok {
				clips = append(clips, authorClip{
					Path:         toString(m["path"]),
					DisplayName:  toString(m["displayName"]),
					Duration:     toFloat(m["duration"]),
					ChapterTitle: toString(m["chapterTitle"]),
				})
			}
		}
	}

	var chapters []authorChapter
	if rawChapters, ok := cfg["chapters"].([]interface{}); ok {
		for _, rc := range rawChapters {
			if m, ok := rc.(map[string]interface{}); ok {
				chapters = append(chapters, authorChapter{
					Timestamp: toFloat(m["timestamp"]),
					Title:     toString(m["title"]),
					Auto:      toBool(m["auto"]),
				})
			}
		}
	}

	logFile, logPath, logErr := createAuthorLog(paths, outputPath, makeISO, region, aspect, title)
	if logErr != nil {
		logging.Debug(logging.CatSystem, "author log open failed: %v", logErr)
	} else {
		job.LogPath = logPath
		s.authorLogFilePath = logPath // Store for UI access
		defer logFile.Close()
	}

	appendLog := func(line string) {
		if logFile != nil {
			fmt.Fprintln(logFile, line)
		}
		app := fyne.CurrentApp()
		if app != nil && app.Driver() != nil {
			app.Driver().DoFromGoroutine(func() {
				s.appendAuthorLog(line)
			}, false)
		}
	}

	updateProgress := func(percent float64) {
		progressCallback(percent)
		app := fyne.CurrentApp()
		if app != nil && app.Driver() != nil {
			app.Driver().DoFromGoroutine(func() {
				s.setAuthorProgress(percent)
			}, false)
		}
	}

	appendLog(fmt.Sprintf("Authoring started: %s", time.Now().Format(time.RFC3339)))
	appendLog(fmt.Sprintf("Inputs: %s", strings.Join(paths, ", ")))
	appendLog(fmt.Sprintf("Output: %s", outputPath))
	if makeISO {
		appendLog("Output mode: ISO")
	} else {
		appendLog("Output mode: VIDEO_TS")
	}

	app := fyne.CurrentApp()
	if app != nil && app.Driver() != nil {
		app.Driver().DoFromGoroutine(func() {
			s.setAuthorStatus("Authoring in progress...")
		}, false)
	}

	err := s.runAuthoringPipeline(ctx, paths, region, aspect, title, outputPath, makeISO, clips, chapters, treatAsChapters, appendLog, updateProgress)
	if err != nil {
		friendly := authorFriendlyError(err)
		appendLog("ERROR: " + friendly)
		if app != nil && app.Driver() != nil {
			app.Driver().DoFromGoroutine(func() {
				s.setAuthorStatus(friendly)
			}, false)
		}
		return fmt.Errorf("%s\nSee Authoring Log for details.", friendly)
	}

	if app != nil && app.Driver() != nil {
		app.Driver().DoFromGoroutine(func() {
			s.setAuthorStatus("Authoring complete")
			s.setAuthorProgress(100)
		}, false)
	}
	appendLog("Authoring completed successfully.")
	return nil
}

func authorFriendlyError(err error) string {
	if err == nil {
		return "Authoring failed"
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "disk quota exceeded"),
		strings.Contains(lower, "no space left"),
		strings.Contains(lower, "not enough space"):
		return "Not enough disk space for authoring output."
	case strings.Contains(lower, "output folder must be empty"):
		return "Output folder must be empty before authoring."
	case strings.Contains(lower, "dvdauthor not found"):
		return "dvdauthor not found. Install DVD authoring tools."
	case strings.Contains(lower, "mkisofs"),
		strings.Contains(lower, "genisoimage"),
		strings.Contains(lower, "xorriso"):
		return "ISO tool not found. Install mkisofs/genisoimage/xorriso."
	case strings.Contains(lower, "permission denied"):
		return "Permission denied writing to output folder."
	case strings.Contains(lower, "ffmpeg"):
		return "FFmpeg failed during DVD encoding."
	default:
		if len(msg) > 140 {
			return "Authoring failed. See Authoring Log for details."
		}
		return msg
	}
}

func prepareDiscRoot(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to read output directory: %w", err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("output folder must be empty: %s", path)
	}
	return nil
}

func encodeAuthorSources(paths []string, region, aspect, workDir string) ([]string, error) {
	var mpgPaths []string
	for i, path := range paths {
		idx := i + 1
		outPath := filepath.Join(workDir, fmt.Sprintf("title_%02d.mpg", idx))
		src, err := probeVideo(path)
		if err != nil {
			return nil, fmt.Errorf("failed to probe %s: %w", filepath.Base(path), err)
		}
		args := buildAuthorFFmpegArgs(path, outPath, region, aspect, src.IsProgressive())
		if err := runCommand(platformConfig.FFmpegPath, args); err != nil {
			return nil, err
		}
		mpgPaths = append(mpgPaths, outPath)
	}
	return mpgPaths, nil
}

func buildAuthorFFmpegArgs(inputPath, outputPath, region, aspect string, progressive bool) []string {
	width := 720
	height := 480
	fps := "30000/1001"
	gop := "15"
	bitrate := "6000k"
	maxrate := "9000k"

	if region == "PAL" {
		height = 576
		fps = "25"
		gop = "12"
		bitrate = "8000k"
		maxrate = "9500k"
	}

	vf := []string{
		fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", width, height),
		fmt.Sprintf("pad=%d:%d:(ow-iw)/2:(oh-ih)/2", width, height),
		fmt.Sprintf("setdar=%s", aspect),
		"setsar=1",
		fmt.Sprintf("fps=%s", fps),
	}

	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "error",
		"-i", inputPath,
		"-vf", strings.Join(vf, ","),
		"-c:v", "mpeg2video",
		"-r", fps,
		"-b:v", bitrate,
		"-maxrate", maxrate,
		"-bufsize", "1835k",
		"-g", gop,
		"-pix_fmt", "yuv420p",
	}

	if !progressive {
		args = append(args, "-flags", "+ilme+ildct")
	}

	args = append(args,
		"-c:a", "ac3",
		"-b:a", "192k",
		"-ar", "48000",
		"-ac", "2",
		"-f", "dvd",           // DVD-compliant MPEG-PS format
		"-muxrate", "10080000", // DVD mux rate (10.08 Mbps)
		"-packetsize", "2048",  // DVD packet size
		outputPath,
	)

	return args
}

func writeDVDAuthorXML(path string, mpgPaths []string, region, aspect string, chapters []authorChapter) error {
	format := strings.ToLower(region)
	if format != "pal" {
		format = "ntsc"
	}

	var b strings.Builder
	b.WriteString("<dvdauthor>\n")
	b.WriteString("  <vmgm />\n")
	b.WriteString("  <titleset>\n")
	b.WriteString("    <titles>\n")
	b.WriteString(fmt.Sprintf("      <video format=\"%s\" aspect=\"%s\" />\n", format, aspect))
	for _, mpg := range mpgPaths {
		b.WriteString("      <pgc>\n")
		if len(chapters) > 0 {
			b.WriteString(fmt.Sprintf("        <vob file=\"%s\" chapters=\"%s\" />\n", escapeXMLAttr(mpg), chaptersToDVDAuthor(chapters)))
		} else {
			b.WriteString(fmt.Sprintf("        <vob file=\"%s\" />\n", escapeXMLAttr(mpg)))
		}
		b.WriteString("      </pgc>\n")
	}
	b.WriteString("    </titles>\n")
	b.WriteString("  </titleset>\n")
	b.WriteString("</dvdauthor>\n")

	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		return fmt.Errorf("failed to write dvdauthor XML: %w", err)
	}
	return nil
}

func escapeXMLAttr(value string) string {
	var b strings.Builder
	if err := xml.EscapeText(&b, []byte(value)); err != nil {
		return strings.ReplaceAll(value, "\"", "&quot;")
	}
	escaped := b.String()
	return strings.ReplaceAll(escaped, "\"", "&quot;")
}

func ensureAuthorDependencies(makeISO bool) error {
	if err := ensureExecutable(platformConfig.FFmpegPath, "ffmpeg"); err != nil {
		return err
	}
	if _, err := exec.LookPath("dvdauthor"); err != nil {
		return fmt.Errorf("dvdauthor not found in PATH")
	}
	if makeISO {
		if _, _, err := buildISOCommand("output.iso", "output", "VIDEO_TOOLS"); err != nil {
			return err
		}
	}
	return nil
}

func createAuthorLog(inputs []string, outputPath string, makeISO bool, region, aspect, title string) (*os.File, string, error) {
	base := strings.TrimSuffix(filepath.Base(outputPath), filepath.Ext(outputPath))
	if base == "" {
		base = "author"
	}
	logPath := filepath.Join(getLogsDir(), base+"-author"+conversionLogSuffix)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, logPath, fmt.Errorf("create log dir: %w", err)
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, logPath, err
	}
	mode := "VIDEO_TS"
	if makeISO {
		mode = "ISO"
	}
	header := fmt.Sprintf(`VideoTools Authoring Log
Started: %s
Inputs: %s
Output: %s
Mode: %s
Region: %s
Aspect: %s
Title: %s

`, time.Now().Format(time.RFC3339), strings.Join(inputs, ", "), outputPath, mode, region, aspect, title)
	if _, err := f.WriteString(header); err != nil {
		_ = f.Close()
		return nil, logPath, err
	}
	return f, logPath, nil
}

func runCommandWithLogger(ctx context.Context, name string, args []string, logFn func(string)) error {
	cmd := exec.CommandContext(ctx, name, args...)
	utils.ApplyNoWindow(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("%s stdout: %w", name, err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("%s stderr: %w", name, err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s start: %w", name, err)
	}

	var wg sync.WaitGroup
	stream := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			if logFn != nil {
				logFn(scanner.Text())
			}
		}
	}
	wg.Add(2)
	go stream(stdout)
	go stream(stderr)

	err = cmd.Wait()
	wg.Wait()
	if err != nil {
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return nil
}

func toBool(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return strings.EqualFold(val, "true")
	case float64:
		return val != 0
	case int:
		return val != 0
	default:
		return false
	}
}

func ensureExecutable(path, label string) error {
	if filepath.IsAbs(path) {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}
	if _, err := exec.LookPath(path); err == nil {
		return nil
	}
	return fmt.Errorf("%s not found (%s)", label, path)
}

func buildISOCommand(outputISO, discRoot, title string) (string, []string, error) {
	tool, prefixArgs, err := findISOTool()
	if err != nil {
		return "", nil, err
	}
	label := isoVolumeLabel(title)
	args := append([]string{}, prefixArgs...)
	args = append(args, "-dvd-video", "-V", label, "-o", outputISO, discRoot)
	return tool, args, nil
}

func findISOTool() (string, []string, error) {
	if path, err := exec.LookPath("mkisofs"); err == nil {
		return path, nil, nil
	}
	if path, err := exec.LookPath("genisoimage"); err == nil {
		return path, nil, nil
	}
	if path, err := exec.LookPath("xorriso"); err == nil {
		return path, []string{"-as", "mkisofs"}, nil
	}
	return "", nil, fmt.Errorf("mkisofs, genisoimage, or xorriso not found in PATH")
}

func isoVolumeLabel(title string) string {
	label := strings.ToUpper(strings.TrimSpace(title))
	if label == "" {
		label = "VIDEO_TOOLS"
	}
	var b strings.Builder
	for _, r := range label {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-':
			b.WriteRune('_')
		default:
			b.WriteRune('_')
		}
	}
	clean := strings.Trim(b.String(), "_")
	if clean == "" {
		clean = "VIDEO_TOOLS"
	}
	if len(clean) > 32 {
		clean = clean[:32]
	}
	return clean
}

func runCommand(name string, args []string) error {
	cmd := exec.Command(name, args...)
	utils.ApplyNoWindow(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %s", name, strings.TrimSpace(string(output)))
	}
	return nil
}

func (s *appState) showChapterPreview(videoPath string, chapters []authorChapter, callback func(bool)) {
	dlg := dialog.NewCustom("Chapter Preview", "Close", container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Detected %d chapters - generating thumbnails...", len(chapters))),
		widget.NewProgressBarInfinite(),
	), s.window)
	dlg.Resize(fyne.NewSize(800, 600))
	dlg.Show()

	go func() {
		// Limit preview to first 24 chapters for performance
		previewCount := len(chapters)
		if previewCount > 24 {
			previewCount = 24
		}

		thumbnails := make([]fyne.CanvasObject, 0, previewCount)
		for i := 0; i < previewCount; i++ {
			ch := chapters[i]
			thumbPath, err := extractChapterThumbnail(videoPath, ch.Timestamp)
			if err != nil {
				logging.Debug(logging.CatSystem, "failed to extract thumbnail at %.2f: %v", ch.Timestamp, err)
				continue
			}

			img := canvas.NewImageFromFile(thumbPath)
			img.FillMode = canvas.ImageFillContain
			img.SetMinSize(fyne.NewSize(160, 90))

			timeLabel := widget.NewLabel(fmt.Sprintf("%.2fs", ch.Timestamp))
			timeLabel.Alignment = fyne.TextAlignCenter

			thumbCard := container.NewVBox(
				container.NewMax(img),
				timeLabel,
			)
			thumbnails = append(thumbnails, thumbCard)
		}

		runOnUI(func() {
			dlg.Hide()

			if len(thumbnails) == 0 {
				dialog.ShowError(fmt.Errorf("failed to generate chapter thumbnails"), s.window)
				return
			}

			grid := container.NewGridWrap(fyne.NewSize(170, 120), thumbnails...)
			scroll := container.NewVScroll(grid)
			scroll.SetMinSize(fyne.NewSize(780, 500))

			infoText := fmt.Sprintf("Found %d chapters", len(chapters))
			if len(chapters) > previewCount {
				infoText += fmt.Sprintf(" (showing first %d)", previewCount)
			}
			info := widget.NewLabel(infoText)
			info.Wrapping = fyne.TextWrapWord

			var previewDlg *dialog.CustomDialog
			acceptBtn := widget.NewButton("Accept Chapters", func() {
				previewDlg.Hide()
				callback(true)
			})
			acceptBtn.Importance = widget.HighImportance

			rejectBtn := widget.NewButton("Reject", func() {
				previewDlg.Hide()
				callback(false)
			})

			content := container.NewBorder(
				container.NewVBox(info, widget.NewSeparator()),
				container.NewHBox(rejectBtn, acceptBtn),
				nil,
				nil,
				scroll,
			)

			previewDlg = dialog.NewCustom("Chapter Preview", "Close", content, s.window)
			previewDlg.Resize(fyne.NewSize(800, 600))
			previewDlg.Show()
		})
	}()
}

func extractChapterThumbnail(videoPath string, timestamp float64) (string, error) {
	tmpDir := filepath.Join(os.TempDir(), "videotools-chapter-thumbs")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}

	outputPath := filepath.Join(tmpDir, fmt.Sprintf("thumb_%.2f.jpg", timestamp))
	args := []string{
		"-ss", fmt.Sprintf("%.2f", timestamp),
		"-i", videoPath,
		"-frames:v", "1",
		"-q:v", "2",
		"-vf", "scale=320:180",
		"-y",
		outputPath,
	}

	cmd := exec.Command(platformConfig.FFmpegPath, args...)
	utils.ApplyNoWindow(cmd)
	if err := cmd.Run(); err != nil {
		return "", err
	}

	return outputPath, nil
}

func runOnUI(fn func()) {
	fn()
}
