package subtitles

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

const (
	subtitleModeExternal = "External Subtitle File"
	subtitleModeEmbed    = "Embed Subtitle Track"
	subtitleModeBurn     = "Burn In Subtitles"

	ModuleColor = "#AD741F"
)

func BuildView(cb ViewCallbacks) fyne.CanvasObject {
	t := i18n.T()

	if strings.TrimSpace(cb.BackendPath()) == "" {
		if detected := cb.DetectWhisperBackend(); detected != "" {
			cb.SetBackendPath(detected)
			cb.PersistSubtitlesConfig()
		}
	}
	if strings.TrimSpace(cb.ModelPath()) == "" {
		if detected := cb.DetectWhisperModel(); detected != "" {
			cb.SetModelPath(detected)
			cb.PersistSubtitlesConfig()
		}
	}
	if strings.TrimSpace(cb.RipMode()) == "" {
		cb.SetRipMode("Text (SRT/ASS)")
	}
	if strings.TrimSpace(cb.OCRLanguage()) == "" {
		cb.SetOCRLanguage("eng")
	}
	if strings.TrimSpace(cb.OCROutput()) == "" {
		cb.SetOCROutput("srt")
	}

	backBtn := widget.NewButton("< "+strings.ToUpper(t.ModuleSubtitles), func() {
		cb.ShowMainMenu()
	})
	backBtn.Importance = widget.LowImportance

	queueBtn := widget.NewButton(t.ActionViewQueue, func() {
		cb.ShowQueue()
	})
	cb.SetQueueBtn(queueBtn)
	cb.UpdateQueueButtonLabel()

	clearCompletedBtn := widget.NewButton("⌫", func() {
		cb.ClearCompletedJobs()
	})
	clearCompletedBtn.Importance = widget.LowImportance

	subtitlesColor := utils.MustHex(ModuleColor)
	topBar := ui.TintedBar(subtitlesColor, container.NewHBox(backBtn, layout.NewSpacer(), clearCompletedBtn, queueBtn))

	videoEntry := widget.NewEntry()
	videoEntry.SetPlaceHolder(t.SubtitlesVideoPlaceholder)
	logging.Debug(logging.CatModule, "BuildView: creating videoEntry with VideoPath=%s", cb.VideoPath())
	videoEntry.SetText(cb.VideoPath())
	videoEntry.OnChanged = func(val string) {
		cb.SetVideoPath(strings.TrimSpace(val))
	}

	subtitleEntry := widget.NewEntry()
	subtitleEntry.SetPlaceHolder(t.SubtitlesFilePlaceholder)
	subtitleEntry.SetText(cb.FilePath())
	subtitleEntry.OnChanged = func(val string) {
		cb.SetFilePath(strings.TrimSpace(val))
	}

	modelEntry := widget.NewEntry()
	modelEntry.SetPlaceHolder(t.SubtitlesModelPlaceholder)
	modelEntry.SetText(cb.ModelPath())
	modelEntry.OnChanged = func(val string) {
		cb.SetModelPath(strings.TrimSpace(val))
		cb.PersistSubtitlesConfig()
	}

	backendEntry := widget.NewEntry()
	backendEntry.SetPlaceHolder(t.SubtitlesBackendPlaceholder)
	backendEntry.SetText(cb.BackendPath())
	backendEntry.OnChanged = func(val string) {
		cb.SetBackendPath(strings.TrimSpace(val))
		cb.PersistSubtitlesConfig()
	}

	backendLabel := widget.NewLabel("")
	modelLabel := widget.NewLabel("")
	offlineHint := widget.NewLabel(i18n.T().SubtitlesOfflineHint)
	offlineHint.Wrapping = fyne.TextWrapWord
	refreshWhisperUI := func() {
		missingModel := strings.TrimSpace(cb.ModelPath()) == ""
		if missingModel {
			offlineHint.SetText(i18n.T().SubtitlesOfflineHint)
		} else {
			offlineHint.SetText(i18n.T().SubtitlesOfflineModelHint)
		}
		if strings.TrimSpace(cb.BackendPath()) != "" {
			backendLabel.SetText(fmt.Sprintf(i18n.T().SubtitlesWhisperBackendFmt, cb.BackendPath()))
			backendEntry.Hide()
		} else {
			backendLabel.SetText("")
			backendEntry.Show()
		}
		if strings.TrimSpace(cb.ModelPath()) != "" {
			modelLabel.SetText(fmt.Sprintf(i18n.T().SubtitlesWhisperModelFmt, cb.ModelPath()))
			modelEntry.Hide()
		} else {
			modelLabel.SetText("")
			modelEntry.Show()
		}
	}
	refreshWhisperUI()

	outputEntry := widget.NewEntry()
	outputEntry.SetPlaceHolder(t.SubtitlesOutputPlaceholder)
	outputEntry.SetText(cb.BurnOutput())
	outputEntry.OnChanged = func(val string) {
		cb.SetBurnOutput(strings.TrimSpace(val))
		cb.PersistSubtitlesConfig()
	}

	statusLabel := widget.NewLabel("")
	statusLabel.Wrapping = fyne.TextWrapWord
	cb.SetStatusLabel(statusLabel)
	if cb.Status() != "" {
		statusLabel.SetText(cb.Status())
	}

	copyStatusBtn := widget.NewButton(t.SubtitlesCopyStatus, func() {
		if cb.Status() != "" {
			cb.Clipboard().SetContent(cb.Status())
			dialog.ShowInformation("Copied", "Status text copied to clipboard", cb.Window())
		}
	})
	copyStatusBtn.Importance = widget.LowImportance

	statusScroll := container.NewVScroll(statusLabel)
	statusScroll.SetMinSize(fyne.NewSize(0, 60))

	var rebuildCues func()
	cueList := container.NewVBox()
	listScroll := container.NewVScroll(cueList)
	var emptyOverlay *fyne.Container
	rebuildCues = func() {
		cueList.Objects = nil
		cues := cb.Cues()
		if len(cues) == 0 {
			if emptyOverlay != nil {
				emptyOverlay.Show()
			}
			cueList.Refresh()
			return
		}
		if emptyOverlay != nil {
			emptyOverlay.Hide()
		}
		for i, cue := range cues {
			idx := i

			startEntry := widget.NewEntry()
			startEntry.SetPlaceHolder("00:00:00,000")
			startEntry.SetText(formatSRTTimestamp(cue.Start))
			startEntry.OnChanged = func(val string) {
				if seconds, ok := parseSRTTimestamp(val); ok {
					cue.Start = seconds
					cb.UpdateCue(idx, cue)
				}
			}

			endEntry := widget.NewEntry()
			endEntry.SetPlaceHolder("00:00:00,000")
			endEntry.SetText(formatSRTTimestamp(cue.End))
			endEntry.OnChanged = func(val string) {
				if seconds, ok := parseSRTTimestamp(val); ok {
					cue.End = seconds
					cb.UpdateCue(idx, cue)
				}
			}

			textEntry := widget.NewMultiLineEntry()
			textEntry.SetText(cue.Text)
			textEntry.Wrapping = fyne.TextWrapWord
			textEntry.OnChanged = func(val string) {
				cue.Text = val
				cb.UpdateCue(idx, cue)
			}

			removeBtn := widget.NewButton(i18n.T().ActionRemove, func() {
				cb.RemoveCue(idx)
				rebuildCues()
			})
			removeBtn.Importance = widget.MediumImportance

			timesCol := container.NewVBox(
				widget.NewLabel(i18n.T().SubtitlesStart),
				startEntry,
				widget.NewLabel(i18n.T().SubtitlesEnd),
				endEntry,
			)

			row := container.NewBorder(nil, nil, timesCol, removeBtn, textEntry)
			cardBg := canvas.NewRectangle(utils.MustHex("#171C2A"))
			cardBg.CornerRadius = 6
			cueList.Add(container.NewPadded(container.NewMax(cardBg, row)))
		}
		cueList.Refresh()
	}

	handleDrop := func(items []fyne.URI) {
		logging.Debug(logging.CatModule, "subtitles handleDrop called with %d items", len(items))
		var videoPath string
		var subtitlePath string
		for _, uri := range items {
			logging.Debug(logging.CatModule, "subtitles handleDrop: uri scheme=%s path=%s", uri.Scheme(), uri.Path())
			if uri.Scheme() != "file" {
				continue
			}
			path := uri.Path()
			if videoPath == "" && cb.IsVideoFile(path) {
				videoPath = path
				logging.Debug(logging.CatModule, "subtitles handleDrop: identified as video: %s", path)
			}
			if subtitlePath == "" && cb.IsSubtitleFile(path) {
				subtitlePath = path
				logging.Debug(logging.CatModule, "subtitles handleDrop: identified as subtitle: %s", path)
			}
		}
		if videoPath != "" {
			logging.Debug(logging.CatModule, "subtitles handleDrop: setting video path to %s", videoPath)
			cb.SetVideoPath(videoPath)
			videoEntry.SetText(videoPath)
			logging.Debug(logging.CatModule, "subtitles handleDrop: videoEntry text set to %s", videoPath)
			cb.LoadVideoInPlayer(videoPath)
		}
		if subtitlePath != "" {
			logging.Debug(logging.CatModule, "subtitles handleDrop: setting subtitle path to %s", subtitlePath)
			subtitleEntry.SetText(subtitlePath)
			if err := cb.LoadSubtitleFile(subtitlePath); err != nil {
				cb.SetSubtitleStatus(err.Error())
			}
			rebuildCues()
		}
	}

	emptyLabel := widget.NewLabel(i18n.T().SubtitlesEmpty)
	emptyLabel.Alignment = fyne.TextAlignCenter
	emptyOverlay = container.NewCenter(emptyLabel)

	listArea := container.NewMax(listScroll, emptyOverlay)

	addCueBtn := widget.NewButton(t.SubtitlesAddCue, func() {
		cues := cb.Cues()
		start := 0.0
		if len(cues) > 0 {
			start = cues[len(cues)-1].End
		}
		newCues := append(cues, SubtitleCue{Start: start, End: start + 2.0, Text: ""})
		cb.SetCues(newCues)
		rebuildCues()
	})
	addCueBtn.Importance = widget.HighImportance

	clearBtn := widget.NewButton(t.ActionClearAll, func() {
		cb.SetCues(nil)
		rebuildCues()
	})

	loadBtn := widget.NewButton(t.SubtitlesLoadSubtitles, func() {
		if err := cb.LoadSubtitleFile(cb.FilePath()); err != nil {
			cb.SetSubtitleStatus(err.Error())
			return
		}
		rebuildCues()
	})

	saveBtn := widget.NewButton(t.SubtitlesSaveSubtitles, func() {
		path := strings.TrimSpace(cb.FilePath())
		if path == "" {
			path = defaultSubtitlePath(cb.VideoPath())
			cb.SetFilePath(path)
			subtitleEntry.SetText(path)
		}
		if err := cb.LoadSubtitleFile(path); err != nil {
			cb.SetSubtitleStatus(err.Error())
			return
		}
		cb.SetSubtitleStatus(fmt.Sprintf("Saved subtitles to %s", filepath.Base(path)))
	})

	generateBtn := widget.NewButton(t.SubtitlesGenerateSpeech, func() {
		cb.GenerateSubtitlesFromSpeech()
		rebuildCues()
	})
	generateBtn.Importance = widget.HighImportance

	outputModeSelect := widget.NewSelect(
		[]string{subtitleModeExternal, subtitleModeEmbed, subtitleModeBurn},
		func(val string) {
			cb.SetOutputMode(val)
			cb.PersistSubtitlesConfig()
		},
	)
	outputModeSelect.SetSelected(cb.OutputMode())

	applyBtn := widget.NewButton(t.SubtitlesCreateOutput, func() {
		cb.ApplySubtitlesToVideo()
	})
	applyBtn.Importance = widget.HighImportance

	browseVideoBtn := widget.NewButton(t.ActionBrowse, func() {
		dialog.ShowFileOpen(func(file fyne.URIReadCloser, err error) {
			if err != nil || file == nil {
				return
			}
			defer file.Close()
			path := file.URI().Path()
			cb.SetVideoPath(path)
			videoEntry.SetText(path)
			cb.LoadVideoInPlayer(path)
		}, cb.Window())
	})

	browseSubtitleBtn := widget.NewButton(t.ActionBrowse, func() {
		dialog.ShowFileOpen(func(file fyne.URIReadCloser, err error) {
			if err != nil || file == nil {
				return
			}
			defer file.Close()
			path := file.URI().Path()
			if err := cb.LoadSubtitleFile(path); err != nil {
				cb.SetSubtitleStatus(err.Error())
				return
			}
			cb.SetFilePath(path)
			subtitleEntry.SetText(path)
			rebuildCues()
		}, cb.Window())
	})

	streamSelect := widget.NewSelect([]string{}, func(val string) {
		streams := cb.RipStreams()
		for i, info := range streams {
			if subtitleStreamLabel(info) == val {
				cb.SetRipIndex(i)
				return
			}
		}
	})

	ripModeSelect := widget.NewSelect([]string{"Text (SRT/ASS)", "Original (lossless)"}, func(val string) {
		cb.SetRipMode(val)
	})
	ripModeSelect.SetSelected(cb.RipMode())

	ocrOutputSelect := widget.NewSelect([]string{"SRT", "ASS"}, func(val string) {
		cb.SetOCROutput(strings.ToLower(val))
		cb.PersistSubtitlesConfig()
	})
	if strings.EqualFold(cb.OCROutput(), "ass") {
		ocrOutputSelect.SetSelected("ASS")
	} else {
		ocrOutputSelect.SetSelected("SRT")
	}

	ocrLangEntry := widget.NewEntry()
	ocrLangEntry.SetPlaceHolder("eng")
	ocrLangEntry.SetText(cb.OCRLanguage())
	ocrLangEntry.OnChanged = func(val string) {
		cb.SetOCRLanguage(strings.TrimSpace(val))
		cb.PersistSubtitlesConfig()
	}

	refreshRipStreams := func() {
		options := []string{}
		streams := cb.RipStreams()
		for _, info := range streams {
			options = append(options, subtitleStreamLabel(info))
		}
		if len(options) == 0 {
			streamSelect.SetOptions([]string{})
			streamSelect.ClearSelected()
			cb.SetRipIndex(0)
			return
		}
		streamSelect.SetOptions(options)
		if cb.RipIndex() < 0 || cb.RipIndex() >= len(options) {
			cb.SetRipIndex(0)
		}
		streamSelect.SetSelected(options[cb.RipIndex()])
	}

	detectStreamsBtn := widget.NewButton(t.SubtitlesDetectStreams, func() {
		videoPath := strings.TrimSpace(cb.VideoPath())
		if videoPath == "" {
			cb.SetSubtitleStatus("Set a video file before detecting subtitle streams.")
			return
		}
		streams, err := probeSubtitleStreams(videoPath)
		if err != nil {
			cb.SetSubtitleStatus(err.Error())
			return
		}
		if len(streams) == 0 {
			cb.SetRipStreams(nil)
			refreshRipStreams()
			cb.SetSubtitleStatus("No embedded subtitle streams found.")
			return
		}
		cb.SetRipStreams(streams)
		refreshRipStreams()
		cb.SetSubtitleStatus(fmt.Sprintf("Detected %d subtitle streams.", len(streams)))
	})

	ripBtn := widget.NewButton(t.SubtitlesExtractSelected, func() {
		videoPath := strings.TrimSpace(cb.VideoPath())
		if videoPath == "" {
			cb.SetSubtitleStatus("Set a video file before extracting subtitles.")
			return
		}
		streams := cb.RipStreams()
		if len(streams) == 0 {
			cb.SetSubtitleStatus("No subtitle streams detected yet.")
			return
		}
		if cb.RipIndex() < 0 || cb.RipIndex() >= len(streams) {
			cb.SetSubtitleStatus("Select a subtitle stream to extract.")
			return
		}
		stream := streams[cb.RipIndex()]
		if strings.Contains(strings.ToLower(cb.RipMode()), "text") && !stream.IsText {
			if _, err := exec.LookPath("tesseract"); err != nil {
				cb.SetSubtitleStatus("Selected subtitle stream is image-based. Install Tesseract or use Original (lossless).")
				return
			}
		}
		cb.SetSubtitleStatus("Extracting subtitles...")
		go func() {
			outputPath, err := extractSubtitleStream(videoPath, stream, cb.RipMode(), cb.OCRLanguage(), cb.OCROutput())
			if err != nil {
				cb.SetSubtitleStatusAsync(err.Error())
				return
			}
			app := fyne.CurrentApp()
			if app != nil && app.Driver() != nil {
				app.Driver().DoFromGoroutine(func() {
					subtitleEntry.SetText(outputPath)
					cb.SetFilePath(outputPath)
					ext := strings.ToLower(filepath.Ext(outputPath))
					if ext == ".srt" || ext == ".vtt" {
						if err := cb.LoadSubtitleFile(outputPath); err != nil {
							cb.SetSubtitleStatus(err.Error())
							return
						}
						rebuildCues()
					}
					cb.SetSubtitleStatus(fmt.Sprintf("Extracted subtitles to %s", filepath.Base(outputPath)))
				}, false)
			} else {
				cb.SetSubtitleStatusAsync(fmt.Sprintf("Extracted subtitles to %s", filepath.Base(outputPath)))
			}
		}()
	})
	ripBtn.Importance = widget.HighImportance

	offsetEntry := widget.NewEntry()
	offsetEntry.SetPlaceHolder("0.0")
	offsetEntry.SetText(fmt.Sprintf("%.2f", cb.TimeOffset()))
	offsetEntry.OnChanged = func(val string) {
		if offset, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
			cb.SetTimeOffset(offset)
			cb.PersistSubtitlesConfig()
		}
	}

	applyOffsetBtn := widget.NewButton(t.SubtitlesApplyOffset, func() {
		cb.ApplySubtitleTimeOffset(cb.TimeOffset())
	})
	applyOffsetBtn.Importance = widget.HighImportance

	offsetPlus1Btn := widget.NewButton("+1s", func() {
		cb.ApplySubtitleTimeOffset(1.0)
	})

	offsetMinus1Btn := widget.NewButton("-1s", func() {
		cb.ApplySubtitleTimeOffset(-1.0)
	})

	offsetPlus01Btn := widget.NewButton("+0.1s", func() {
		cb.ApplySubtitleTimeOffset(0.1)
	})

	offsetMinus01Btn := widget.NewButton("-0.1s", func() {
		cb.ApplySubtitleTimeOffset(-0.1)
	})

	applyControls := func() {
		outputModeSelect.SetSelected(cb.OutputMode())
		backendEntry.SetText(cb.BackendPath())
		modelEntry.SetText(cb.ModelPath())
		outputEntry.SetText(cb.BurnOutput())
		offsetEntry.SetText(fmt.Sprintf("%.2f", cb.TimeOffset()))
		if strings.EqualFold(cb.OCROutput(), "ass") {
			ocrOutputSelect.SetSelected("ASS")
		} else {
			ocrOutputSelect.SetSelected("SRT")
		}
		ocrLangEntry.SetText(cb.OCRLanguage())
		refreshWhisperUI()
	}

	refreshRipStreams()

	loadCfgBtn := widget.NewButton(t.ActionLoadConfig, func() {
		cfg, err := cb.LoadConfig()
		if err != nil {
			if os.IsNotExist(err) {
				dialog.ShowInformation("No Config", "No saved config found yet. It will save automatically after your first change.", cb.Window())
			} else {
				dialog.ShowError(fmt.Errorf("failed to load config: %w", err), cb.Window())
			}
			return
		}
		cb.ApplySubtitlesConfig(cfg)
		applyControls()
	})

	saveCfgBtn := widget.NewButton(t.ActionSaveConfig, func() {
		cfg := SubtitleState{
			OutputMode:  cb.OutputMode(),
			ModelPath:   cb.ModelPath(),
			BackendPath: cb.BackendPath(),
			BurnOutput:  cb.BurnOutput(),
			TimeOffset:  cb.TimeOffset(),
			OCRLanguage: cb.OCRLanguage(),
			OCROutput:   cb.OCROutput(),
		}
		if err := cb.SaveConfig(cfg); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save config: %w", err), cb.Window())
			return
		}
		dialog.ShowInformation("Config Saved", fmt.Sprintf("Saved to subtitles config"), cb.Window())
	})

	resetBtn := widget.NewButton(t.ActionReset, func() {
		cb.ApplySubtitlesConfig(SubtitleState{})
		applyControls()
		cb.PersistSubtitlesConfig()
	})

	// Build the video preview player pane (native_media builds only).
	// subCueLabel shows the active subtitle cue in sync with playback.
	var playerSection fyne.CanvasObject
	subCueLabel := widget.NewLabel("")
	subCueLabel.Alignment = fyne.TextAlignCenter
	subCueLabel.Wrapping = fyne.TextWrapWord
	if cb.HasPlayer() {
		playerBg := canvas.NewRectangle(utils.MustHex("#0F1529"))
		playerBg.SetMinSize(fyne.NewSize(0, 260))
		playerPane := container.NewMax(playerBg, cb.PlayerWidget())

		cueBg := canvas.NewRectangle(utils.MustHex("#0D1118"))
		cueBg.SetMinSize(fyne.NewSize(0, 48))
		cueBar := container.NewMax(cueBg, container.NewPadded(subCueLabel))

		playerSection = container.NewVBox(playerPane, cueBar)

		cb.SetProgressCallback(func(t float64) {
			cues := cb.Cues()
			text := ""
			for _, cue := range cues {
				if t >= cue.Start && t <= cue.End {
					text = cue.Text
					break
				}
			}
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				subCueLabel.SetText(text)
			}, false)
		})
	}

	left := container.NewVBox(
		widget.NewLabelWithStyle(t.SubtitlesSources, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, nil, browseVideoBtn, videoEntry),
		container.NewBorder(nil, nil, nil, browseSubtitleBtn, subtitleEntry),
		widget.NewSeparator(),
		widget.NewLabelWithStyle(t.SubtitlesRipSection, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(i18n.T().SubtitlesExtractEmbed),
		streamSelect,
		container.NewHBox(detectStreamsBtn, ripBtn),
		ripModeSelect,
		widget.NewLabel(i18n.T().SubtitlesOCROutput),
		ocrOutputSelect,
		widget.NewLabel(i18n.T().SubtitlesOCRLanguage),
		ocrLangEntry,
		widget.NewSeparator(),
		widget.NewLabelWithStyle(t.SubtitlesTimingSection, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(i18n.T().SubtitlesShiftOffset),
		offsetEntry,
		container.NewHBox(offsetMinus1Btn, offsetMinus01Btn, offsetPlus01Btn, offsetPlus1Btn),
		applyOffsetBtn,
		widget.NewSeparator(),
		widget.NewLabelWithStyle(t.SubtitlesSTTSection, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		offlineHint,
		backendLabel,
		backendEntry,
		modelLabel,
		modelEntry,
		container.NewHBox(generateBtn),
		widget.NewSeparator(),
		widget.NewLabelWithStyle(t.SubtitlesOutputSection, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		outputModeSelect,
		outputEntry,
		widget.NewSeparator(),
		widget.NewLabelWithStyle(t.SubtitlesStatusSection, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		statusScroll,
		container.NewHBox(copyStatusBtn),
		widget.NewSeparator(),
		container.NewHBox(resetBtn, loadCfgBtn, saveCfgBtn),
	)

	right := container.NewBorder(
		container.NewVBox(
			widget.NewLabelWithStyle(t.SubtitlesCuesSection, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			container.NewHBox(addCueBtn, clearBtn, loadBtn, saveBtn),
		),
		nil,
		nil,
		nil,
		listArea,
	)

	rebuildCues()

	droppableLeft := ui.NewDroppable(left, handleDrop)
	droppableRight := ui.NewDroppable(right, handleDrop)
	twoCol := container.NewGridWithColumns(2, droppableLeft, droppableRight)
	scroll := container.NewVScroll(twoCol)
	scroll.SetMinSize(fyne.NewSize(0, 0))

	var content fyne.CanvasObject
	if playerSection != nil {
		content = container.NewBorder(playerSection, nil, nil, nil, scroll)
	} else {
		content = scroll
	}

	var bottomBar fyne.CanvasObject
	if cb.StatsBar() != nil {
		bottomBar = container.NewBorder(nil, nil, nil, applyBtn, cb.StatsBar())
	} else {
		bottomBar = container.NewBorder(nil, nil, nil, applyBtn, layout.NewSpacer())
	}

	return container.NewBorder(topBar, bottomBar, nil, nil, content)
}

func formatSRTTimestamp(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	totalMs := int64(seconds*1000 + 0.5)
	hours := totalMs / 3600000
	minutes := (totalMs % 3600000) / 60000
	secs := (totalMs % 60000) / 1000
	ms := totalMs % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, secs, ms)
}

func parseSRTTimestamp(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return 0, false
	}
	timePart := parts[0]
	msPart := parts[1]

	hms := strings.Split(timePart, ":")
	if len(hms) != 3 {
		return 0, false
	}
	hours := 0
	minutes := 0
	secs := 0
	if h, err := strconv.Atoi(hms[0]); err == nil {
		hours = h
	}
	if m, err := strconv.Atoi(hms[1]); err == nil {
		minutes = m
	}
	if s, err := strconv.Atoi(hms[2]); err == nil {
		secs = s
	}
	ms := 0
	if m, err := strconv.Atoi(msPart); err == nil {
		ms = m
	}
	totalMs := ((hours*60+minutes)*60+secs)*1000 + ms
	return float64(totalMs) / 1000.0, true
}

func subtitleStreamLabel(info SubtitleStreamInfo) string {
	lang := strings.TrimSpace(info.Language)
	title := strings.TrimSpace(info.Title)
	flagText := ""
	var flags []string
	if info.Default {
		flags = append(flags, "Default")
	}
	if info.Forced {
		flags = append(flags, "Forced")
	}
	if len(flags) > 0 {
		flagText = " (" + strings.Join(flags, ", ") + ")"
	}
	return fmt.Sprintf("#%d | %s | %s%s%s", info.Index, strings.ToUpper(lang), info.Codec, title, flagText)
}

func probeSubtitleStreams(path string) ([]SubtitleStreamInfo, error) {
	cmd := exec.Command(utils.GetFFprobePath(),
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-select_streams", "s",
		path,
	)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}

	var result struct {
		Streams []struct {
			Index       int
			CodecName   string `json:"codec_name"`
			CodecType   string `json:"codec_type"`
			Tags        map[string]string
			Disposition map[string]int
		}
	}

	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	var streams []SubtitleStreamInfo
	for _, s := range result.Streams {
		if s.CodecType != "subtitle" {
			continue
		}
		lang := s.Tags["language"]
		title := s.Tags["title"]
		streams = append(streams, SubtitleStreamInfo{
			Index:    s.Index,
			Codec:    s.CodecName,
			Language: lang,
			Title:    title,
			Default:  s.Disposition != nil && s.Disposition["default"] == 1,
			Forced:   s.Disposition != nil && s.Disposition["forced"] == 1,
			IsText:   isTextSubtitleCodec(s.CodecName),
			IsImage:  isImageSubtitleCodec(s.CodecName),
		})
	}
	return streams, nil
}

func isTextSubtitleCodec(codec string) bool {
	switch strings.ToLower(codec) {
	case "subrip", "srt", "ass", "ssa", "webvtt", "vtt", "mov_text":
		return true
	}
	return false
}

func isImageSubtitleCodec(codec string) bool {
	switch strings.ToLower(codec) {
	case "dvd_subtitle", "dvb_subtitle", "hdmv_pgs_subtitle":
		return true
	}
	return false
}

func extractSubtitleStream(videoPath string, info SubtitleStreamInfo, mode string, ocrLang string, ocrOutput string) (string, error) {
	outputDir := filepath.Dir(videoPath)
	base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))

	if strings.Contains(strings.ToLower(mode), "original") {
		ext := ".srt"
		if info.IsText {
			switch strings.ToLower(ocrOutput) {
			case "ass":
				ext = ".ass"
			}
		}
		outputPath := filepath.Join(outputDir, base+"_stream"+ext)
		args := []string{
			"-y",
			"-v", "quiet",
			"-i", videoPath,
			"-map", fmt.Sprintf("0:%d", info.Index),
			"-c:s", "copy",
			outputPath,
		}
		if err := runFFmpeg(args); err != nil {
			return "", err
		}
		return outputPath, nil
	}
	if strings.EqualFold(strings.TrimSpace(ocrOutput), "ass") {
		return "", fmt.Errorf("image subtitles cannot be output as ASS; use Original (lossless) mode")
	}
	tmpDir, err := os.MkdirTemp("", "vt-sub-rip-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	extracted := filepath.Join(tmpDir, "subtitle.mks")
	args := []string{
		"-y",
		"-v", "quiet",
		"-i", videoPath,
		"-map", fmt.Sprintf("0:%d", info.Index),
		"-c:s", "copy",
		extracted,
	}
	if err := runFFmpeg(args); err != nil {
		return "", err
	}

	if info.IsText {
		srtPath := filepath.Join(tmpDir, "subtitle.srt")
		convertArgs := []string{
			"-y",
			"-v", "quiet",
			"-i", extracted,
			"-c:s", "srt",
			srtPath,
		}
		if err := runFFmpeg(convertArgs); err != nil {
			return "", err
		}
		data, err := os.ReadFile(srtPath)
		if err != nil {
			return "", fmt.Errorf("failed to read converted subtitle: %w", err)
		}
		outputPath := filepath.Join(outputDir, base+"_stream.srt")
		if err := os.WriteFile(outputPath, data, 0o644); err != nil {
			return "", fmt.Errorf("failed to write subtitle: %w", err)
		}
		return outputPath, nil
	}

	frames, err := extractSubtitleFrames(videoPath, info, tmpDir)
	if err != nil {
		return "", err
	}
	defer func() {
		for _, f := range frames {
			os.Remove(f)
		}
	}()

	text, err := ocrSubtitleStream(videoPath, info, filepath.Join(tmpDir, "ocr.txt"), ocrLang, ocrOutput)
	if err != nil {
		return "", err
	}

	outputPath := filepath.Join(outputDir, base+"_stream.srt")
	if err := os.WriteFile(outputPath, []byte(text), 0o644); err != nil {
		return "", fmt.Errorf("failed to write OCR output: %w", err)
	}
	return outputPath, nil
}

func extractSubtitleFrames(videoPath string, info SubtitleStreamInfo, dir string) ([]string, error) {
	return nil, nil
}

func ocrSubtitleStream(videoPath string, info SubtitleStreamInfo, outputPath string, ocrLang string, ocrOutput string) (string, error) {
	return "", nil
}

func runFFmpeg(args []string) error {
	cmd := exec.Command(utils.GetFFmpegPath(), args...)
	utils.ApplyNoWindow(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func defaultSubtitlePath(videoPath string) string {
	dir := filepath.Dir(videoPath)
	base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	return filepath.Join(dir, base+".srt")
}
