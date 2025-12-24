package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

func buildAuthorView(state *appState) fyne.CanvasObject {
	state.stopPreview()
	state.lastModule = state.active
	state.active = "author"

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

	topBar := ui.TintedBar(authorColor, container.NewHBox(backBtn, layout.NewSpacer(), queueBtn))
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
		rebuildList()
		state.updateAuthorSummary()
	})
	clearBtn.Importance = widget.MediumImportance

	compileBtn := widget.NewButton("COMPILE TO DVD", func() {
		if len(state.authorClips) == 0 {
			dialog.ShowInformation("No Clips", "Please add video clips first", state.window)
			return
		}
		state.startAuthorGeneration()
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
		container.NewVBox(chapterToggle, container.NewHBox(addBtn, clearBtn, compileBtn)),
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
				state.authorChapters = chapters
				state.authorChapterSource = "scenes"
				state.updateAuthorSummary()
				refreshChapters()
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

	controls := container.NewVBox(
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
		container.NewScroll(chapterList),
		container.NewHBox(addChapterBtn, exportBtn),
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
		container.NewHBox(addBtn, clearBtn),
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
	})
	if state.authorOutputType == "iso" {
		outputType.SetSelected("ISO Image")
	} else {
		outputType.SetSelected("DVD (VIDEO_TS)")
	}

	regionSelect := widget.NewSelect([]string{"AUTO", "NTSC", "PAL"}, func(value string) {
		state.authorRegion = value
		state.updateAuthorSummary()
	})
	if state.authorRegion == "" {
		regionSelect.SetSelected("AUTO")
	} else {
		regionSelect.SetSelected(state.authorRegion)
	}

	aspectSelect := widget.NewSelect([]string{"AUTO", "4:3", "16:9"}, func(value string) {
		state.authorAspectRatio = value
		state.updateAuthorSummary()
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
	}

	createMenuCheck := widget.NewCheck("Create DVD Menu", func(checked bool) {
		state.authorCreateMenu = checked
		state.updateAuthorSummary()
	})
	createMenuCheck.SetChecked(state.authorCreateMenu)

	discSizeSelect := widget.NewSelect([]string{"DVD5", "DVD9"}, func(value string) {
		state.authorDiscSize = value
		state.updateAuthorSummary()
	})
	if state.authorDiscSize == "" {
		discSizeSelect.SetSelected("DVD5")
	} else {
		discSizeSelect.SetSelected(state.authorDiscSize)
	}

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
	)

	return container.NewPadded(controls)
}

func buildAuthorDiscTab(state *appState) fyne.CanvasObject {
	generateBtn := widget.NewButton("GENERATE DVD", func() {
		if len(state.authorClips) == 0 && state.authorFile == nil {
			dialog.ShowInformation("No Content", "Please add video clips or select a single video file", state.window)
			return
		}
		state.startAuthorGeneration()
	})
	generateBtn.Importance = widget.HighImportance

	summaryLabel := widget.NewLabel(authorSummary(state))
	summaryLabel.Wrapping = fyne.TextWrapWord
	state.authorSummaryLabel = summaryLabel

	controls := container.NewVBox(
		widget.NewLabel("Generate DVD/ISO:"),
		widget.NewSeparator(),
		summaryLabel,
		widget.NewSeparator(),
		generateBtn,
	)

	return container.NewPadded(controls)
}

func authorSummary(state *appState) string {
	summary := "Ready to generate:\n\n"
	if len(state.authorClips) > 0 {
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
	s.updateAuthorSummary()
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
		output,
	}
	return runCommand(platformConfig.FFmpegPath, args)
}

func (s *appState) startAuthorGeneration() {
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
	continuePrompt := func() {
		s.promptAuthorOutput(paths, region, aspect, title)
	}
	if len(warnings) > 0 {
		dialog.ShowConfirm("Authoring Notes", strings.Join(warnings, "\n")+"\n\nContinue?", func(ok bool) {
			if ok {
				continuePrompt()
			}
		}, s.window)
		return
	}

	continuePrompt()
}

func (s *appState) promptAuthorOutput(paths []string, region, aspect, title string) {
	outputType := strings.ToLower(strings.TrimSpace(s.authorOutputType))
	if outputType == "" {
		outputType = "dvd"
	}

	if outputType == "iso" {
		dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err != nil || writer == nil {
				return
			}
			path := writer.URI().Path()
			writer.Close()
			if !strings.HasSuffix(strings.ToLower(path), ".iso") {
				path += ".iso"
			}
			s.generateAuthoring(paths, region, aspect, title, path, true)
		}, s.window)
		return
	}

	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}
		discRoot := filepath.Join(uri.Path(), authorOutputFolderName(title, paths))
		s.generateAuthoring(paths, region, aspect, title, discRoot, false)
	}, s.window)
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

func (s *appState) generateAuthoring(paths []string, region, aspect, title, outputPath string, makeISO bool) {
	if err := ensureAuthorDependencies(makeISO); err != nil {
		dialog.ShowError(err, s.window)
		return
	}

	progress := dialog.NewProgressInfinite("Authoring DVD", "Encoding sources...", s.window)
	progress.Show()

	go func() {
		err := s.runAuthoringPipeline(paths, region, aspect, title, outputPath, makeISO)
		message := "DVD authoring complete."
		if makeISO {
			message = fmt.Sprintf("ISO image created:\n%s", outputPath)
		} else {
			message = fmt.Sprintf("DVD folders created:\n%s", outputPath)
		}
		runOnUI(func() {
			progress.Hide()
			if err != nil {
				dialog.ShowError(err, s.window)
				return
			}
			dialog.ShowInformation("Authoring Complete", message, s.window)
		})
	}()
}

func (s *appState) runAuthoringPipeline(paths []string, region, aspect, title, outputPath string, makeISO bool) error {
	workDir, err := os.MkdirTemp(utils.TempDir(), "videotools-author-")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	discRoot := outputPath
	var cleanup func()
	if makeISO {
		tempRoot, err := os.MkdirTemp(utils.TempDir(), "videotools-dvd-")
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

	mpgPaths, err := encodeAuthorSources(paths, region, aspect, workDir)
	if err != nil {
		return err
	}

	chapters := s.authorChapters
	if len(chapters) == 0 && s.authorTreatAsChapters && len(s.authorClips) > 1 {
		chapters = chaptersFromClips(s.authorClips)
		s.authorChapterSource = "clips"
	}
	if len(chapters) == 0 && len(mpgPaths) == 1 {
		if embed, err := extractChaptersFromFile(paths[0]); err == nil && len(embed) > 0 {
			chapters = embed
			s.authorChapterSource = "embedded"
		}
	}

	if s.authorTreatAsChapters && len(mpgPaths) > 1 {
		concatPath := filepath.Join(workDir, "titles_joined.mpg")
		if err := concatDVDMpg(mpgPaths, concatPath); err != nil {
			return err
		}
		mpgPaths = []string{concatPath}
	}

	if len(mpgPaths) > 1 {
		chapters = nil
	}

	xmlPath := filepath.Join(workDir, "dvd.xml")
	if err := writeDVDAuthorXML(xmlPath, mpgPaths, region, aspect, chapters); err != nil {
		return err
	}

	if err := runCommand("dvdauthor", []string{"-o", discRoot, "-x", xmlPath}); err != nil {
		return err
	}

	if err := runCommand("dvdauthor", []string{"-o", discRoot, "-T"}); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(discRoot, "AUDIO_TS"), 0755); err != nil {
		return fmt.Errorf("failed to create AUDIO_TS: %w", err)
	}

	if makeISO {
		tool, args, err := buildISOCommand(outputPath, discRoot, title)
		if err != nil {
			return err
		}
		if err := runCommand(tool, args); err != nil {
			return err
		}
	}

	return nil
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

func runOnUI(fn func()) {
	fn()
}
