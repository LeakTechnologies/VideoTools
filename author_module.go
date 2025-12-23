package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
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
		container.NewTabItem("Video Clips", buildVideoClipsTab(state)),
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

	var rebuildList func()
	rebuildList = func() {
		list.Objects = nil

		if len(state.authorClips) == 0 {
			emptyLabel := widget.NewLabel("Drag and drop video files here\nor click 'Add Files' to select videos")
			emptyLabel.Alignment = fyne.TextAlignCenter

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
			return
		}

		for i, clip := range state.authorClips {
			idx := i
			card := widget.NewCard(clip.DisplayName, fmt.Sprintf("%.2fs", clip.Duration), nil)

			removeBtn := widget.NewButton("Remove", func() {
				state.authorClips = append(state.authorClips[:idx], state.authorClips[idx+1:]...)
				rebuildList()
			})
			removeBtn.Importance = widget.MediumImportance

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

	clearBtn := widget.NewButton("Clear All", func() {
		state.authorClips = []authorClip{}
		rebuildList()
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

	controls := container.NewVBox(
		widget.NewLabel("Video Clips:"),
		container.NewScroll(list),
		widget.NewSeparator(),
		container.NewHBox(addBtn, clearBtn, compileBtn),
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
		fileLabel = widget.NewLabel("Select a single video file or use clips from Video Clips tab")
	}

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
		if state.authorFile == nil && len(state.authorClips) == 0 {
			dialog.ShowInformation("No File", "Please select a video file first", state.window)
			return
		}
		dialog.ShowInformation("Scene Detection", "Scene detection will be implemented", state.window)
	})
	detectBtn.Importance = widget.HighImportance

	chapterList := widget.NewLabel("No chapters detected yet")

	addChapterBtn := widget.NewButton("+ Add Chapter", func() {
		dialog.ShowInformation("Add Chapter", "Manual chapter addition will be implemented", state.window)
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
		container.NewScroll(chapterList),
		container.NewHBox(addChapterBtn, exportBtn),
	)

	return container.NewPadded(controls)
}

func buildSubtitlesTab(state *appState) fyne.CanvasObject {
	list := container.NewVBox()

	var buildSubList func()
	buildSubList = func() {
		list.Objects = nil

		if len(state.authorSubtitles) == 0 {
			emptyLabel := widget.NewLabel("Drag and drop subtitle files here\nor click 'Add Subtitles' to select")
			emptyLabel.Alignment = fyne.TextAlignCenter

			emptyDrop := ui.NewDroppable(container.NewCenter(emptyLabel), func(items []fyne.URI) {
				var paths []string
				for _, uri := range items {
					if uri.Scheme() == "file" {
						paths = append(paths, uri.Path())
					}
				}
				if len(paths) > 0 {
					state.authorSubtitles = append(state.authorSubtitles, paths...)
					buildSubList()
				}
			})

			list.Add(container.NewMax(emptyDrop))
			return
		}

		for i, path := range state.authorSubtitles {
			idx := i
			card := widget.NewCard(filepath.Base(path), "", nil)

			removeBtn := widget.NewButton("Remove", func() {
				state.authorSubtitles = append(state.authorSubtitles[:idx], state.authorSubtitles[idx+1:]...)
				buildSubList()
			})
			removeBtn.Importance = widget.MediumImportance

			cardContent := container.NewVBox(removeBtn)
			card.SetContent(cardContent)
			list.Add(card)
		}
	}

	addBtn := widget.NewButton("Add Subtitles", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			state.authorSubtitles = append(state.authorSubtitles, reader.URI().Path())
			buildSubList()
		}, state.window)
	})
	addBtn.Importance = widget.HighImportance

	clearBtn := widget.NewButton("Clear All", func() {
		state.authorSubtitles = []string{}
		buildSubList()
	})
	clearBtn.Importance = widget.MediumImportance

	controls := container.NewVBox(
		widget.NewLabel("Subtitle Tracks:"),
		container.NewScroll(list),
		widget.NewSeparator(),
		container.NewHBox(addBtn, clearBtn),
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
	})
	if state.authorOutputType == "iso" {
		outputType.SetSelected("ISO Image")
	} else {
		outputType.SetSelected("DVD (VIDEO_TS)")
	}

	regionSelect := widget.NewSelect([]string{"AUTO", "NTSC", "PAL"}, func(value string) {
		state.authorRegion = value
	})
	if state.authorRegion == "" {
		regionSelect.SetSelected("AUTO")
	} else {
		regionSelect.SetSelected(state.authorRegion)
	}

	aspectSelect := widget.NewSelect([]string{"AUTO", "4:3", "16:9"}, func(value string) {
		state.authorAspectRatio = value
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
	}

	createMenuCheck := widget.NewCheck("Create DVD Menu", func(checked bool) {
		state.authorCreateMenu = checked
	})
	createMenuCheck.SetChecked(state.authorCreateMenu)

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
		summary += fmt.Sprintf("Video Clips: %d\n", len(state.authorClips))
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

	summary += fmt.Sprintf("Output Type: %s\n", state.authorOutputType)
	summary += fmt.Sprintf("Region: %s\n", state.authorRegion)
	summary += fmt.Sprintf("Aspect Ratio: %s\n", state.authorAspectRatio)
	if state.authorTitle != "" {
		summary += fmt.Sprintf("DVD Title: %s\n", state.authorTitle)
	}
	return summary
}

func (s *appState) addAuthorFiles(paths []string) {
	for _, path := range paths {
		src, err := probeVideo(path)
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to load video %s: %w", filepath.Base(path), err), s.window)
			continue
		}

		clip := authorClip{
			Path:        path,
			DisplayName: filepath.Base(path),
			Duration:    src.Duration,
			Chapters:    []authorChapter{},
		}
		s.authorClips = append(s.authorClips, clip)
	}
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

	xmlPath := filepath.Join(workDir, "dvd.xml")
	if err := writeDVDAuthorXML(xmlPath, mpgPaths, region, aspect); err != nil {
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

func writeDVDAuthorXML(path string, mpgPaths []string, region, aspect string) error {
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
		b.WriteString(fmt.Sprintf("        <vob file=\"%s\" />\n", escapeXMLAttr(mpg)))
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
