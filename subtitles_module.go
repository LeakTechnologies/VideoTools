package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

const (
	subtitleModeExternal = "External SRT"
	subtitleModeEmbed    = "Embed Subtitle Track"
	subtitleModeBurn     = "Burn In Subtitles"
)

type subtitleCue struct {
	Start float64
	End   float64
	Text  string
}

type subtitlesConfig struct {
	OutputMode  string  `json:"outputMode"`
	ModelPath   string  `json:"modelPath"`
	BackendPath string  `json:"backendPath"`
	BurnOutput  string  `json:"burnOutput"`
	TimeOffset  float64 `json:"timeOffset"`
}

func defaultSubtitlesConfig() subtitlesConfig {
	return subtitlesConfig{
		OutputMode:  subtitleModeExternal,
		ModelPath:   "",
		BackendPath: "",
		BurnOutput:  "",
	}
}

func loadPersistedSubtitlesConfig() (subtitlesConfig, error) {
	var cfg subtitlesConfig
	path := moduleConfigPath("subtitles")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.OutputMode == "" {
		cfg.OutputMode = subtitleModeExternal
	}
	return cfg, nil
}

func savePersistedSubtitlesConfig(cfg subtitlesConfig) error {
	path := moduleConfigPath("subtitles")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (s *appState) applySubtitlesConfig(cfg subtitlesConfig) {
	s.subtitleOutputMode = cfg.OutputMode
	s.subtitleModelPath = cfg.ModelPath
	s.subtitleBackendPath = cfg.BackendPath
	s.subtitleBurnOutput = cfg.BurnOutput
	s.subtitleTimeOffset = cfg.TimeOffset
}

func (s *appState) persistSubtitlesConfig() {
	cfg := subtitlesConfig{
		OutputMode:  s.subtitleOutputMode,
		ModelPath:   s.subtitleModelPath,
		BackendPath: s.subtitleBackendPath,
		BurnOutput:  s.subtitleBurnOutput,
		TimeOffset:  s.subtitleTimeOffset,
	}
	if err := savePersistedSubtitlesConfig(cfg); err != nil {
		logging.Debug(logging.CatSystem, "failed to persist subtitles config: %v", err)
	}
}

func (s *appState) showSubtitlesView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "subtitles"

	if cfg, err := loadPersistedSubtitlesConfig(); err == nil {
		s.applySubtitlesConfig(cfg)
	} else if !errors.Is(err, os.ErrNotExist) {
		logging.Debug(logging.CatSystem, "failed to load persisted subtitles config: %v", err)
	}

	if s.subtitleOutputMode == "" {
		s.subtitleOutputMode = subtitleModeExternal
	}

	s.setContent(buildSubtitlesView(s))
}

func buildSubtitlesView(state *appState) fyne.CanvasObject {
	subtitlesColor := moduleColor("subtitles")

	if strings.TrimSpace(state.subtitleBackendPath) == "" {
		if detected := detectWhisperBackend(); detected != "" {
			state.subtitleBackendPath = detected
			state.persistSubtitlesConfig()
		}
	}
	if strings.TrimSpace(state.subtitleModelPath) == "" {
		if detected := detectWhisperModel(); detected != "" {
			state.subtitleModelPath = detected
			state.persistSubtitlesConfig()
		}
	}

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

	topBar := ui.TintedBar(subtitlesColor, container.NewHBox(backBtn, layout.NewSpacer(), clearCompletedBtn, queueBtn))
	bottomBar := moduleFooter(subtitlesColor, layout.NewSpacer(), state.statsBar)

	videoEntry := widget.NewEntry()
	videoEntry.SetPlaceHolder("Video file path")
	logging.Debug(logging.CatModule, "buildSubtitlesView: creating videoEntry with subtitleVideoPath=%s", state.subtitleVideoPath)
	videoEntry.SetText(state.subtitleVideoPath)
	videoEntry.OnChanged = func(val string) {
		state.subtitleVideoPath = strings.TrimSpace(val)
	}

	subtitleEntry := widget.NewEntry()
	subtitleEntry.SetPlaceHolder("Subtitle file (.srt or .vtt)")
	subtitleEntry.SetText(state.subtitleFilePath)
	subtitleEntry.OnChanged = func(val string) {
		state.subtitleFilePath = strings.TrimSpace(val)
	}

	modelEntry := widget.NewEntry()
	modelEntry.SetPlaceHolder("Whisper model path (ggml-*.bin)")
	modelEntry.SetText(state.subtitleModelPath)
	modelEntry.OnChanged = func(val string) {
		state.subtitleModelPath = strings.TrimSpace(val)
		state.persistSubtitlesConfig()
	}

	backendEntry := widget.NewEntry()
	backendEntry.SetPlaceHolder("Whisper backend path (whisper.cpp/main)")
	backendEntry.SetText(state.subtitleBackendPath)
	backendEntry.OnChanged = func(val string) {
		state.subtitleBackendPath = strings.TrimSpace(val)
		state.persistSubtitlesConfig()
	}

	backendLabel := widget.NewLabel("")
	modelLabel := widget.NewLabel("")
	offlineHint := widget.NewLabel("Offline STT uses bundled ggml-small.bin (vendor/whisper).")
	offlineHint.Wrapping = fyne.TextWrapWord
	refreshWhisperUI := func() {
		missingModel := strings.TrimSpace(state.subtitleModelPath) == ""
		if missingModel {
			offlineHint.SetText("Offline STT uses bundled ggml-small.bin (vendor/whisper).")
		} else {
			offlineHint.SetText("Offline STT uses the selected ggml model.")
		}
		if strings.TrimSpace(state.subtitleBackendPath) != "" {
			backendLabel.SetText(fmt.Sprintf("Whisper backend: %s", state.subtitleBackendPath))
			backendEntry.Hide()
		} else {
			backendLabel.SetText("")
			backendEntry.Show()
		}
		if strings.TrimSpace(state.subtitleModelPath) != "" {
			modelLabel.SetText(fmt.Sprintf("Whisper model: %s", state.subtitleModelPath))
			modelEntry.Hide()
		} else {
			modelLabel.SetText("")
			modelEntry.Show()
		}
	}
	refreshWhisperUI()

	outputEntry := widget.NewEntry()
	outputEntry.SetPlaceHolder("Output video path (for embed/burn)")
	outputEntry.SetText(state.subtitleBurnOutput)
	outputEntry.OnChanged = func(val string) {
		state.subtitleBurnOutput = strings.TrimSpace(val)
		state.persistSubtitlesConfig()
	}

	statusLabel := widget.NewLabel("")
	statusLabel.Wrapping = fyne.TextWrapWord
	state.subtitleStatusLabel = statusLabel
	if state.subtitleStatus != "" {
		statusLabel.SetText(state.subtitleStatus)
	}

	// Create copy button for status text
	copyStatusBtn := widget.NewButton("Copy Status", func() {
		if state.subtitleStatus != "" {
			state.window.Clipboard().SetContent(state.subtitleStatus)
			dialog.ShowInformation("Copied", "Status text copied to clipboard", state.window)
		}
	})
	copyStatusBtn.Importance = widget.LowImportance

	// Create scrollable status container
	statusScroll := container.NewVScroll(statusLabel)
	statusScroll.SetMinSize(fyne.NewSize(0, 60))

	var rebuildCues func()
	cueList := container.NewVBox()
	listScroll := container.NewVScroll(cueList)
	var emptyOverlay *fyne.Container
	rebuildCues = func() {
		cueList.Objects = nil
		if len(state.subtitleCues) == 0 {
			if emptyOverlay != nil {
				emptyOverlay.Show()
			}
			cueList.Refresh()
			return
		}
		if emptyOverlay != nil {
			emptyOverlay.Hide()
		}
		for i, cue := range state.subtitleCues {
			idx := i

			startEntry := widget.NewEntry()
			startEntry.SetPlaceHolder("00:00:00,000")
			startEntry.SetText(formatSRTTimestamp(cue.Start))
			startEntry.OnChanged = func(val string) {
				if seconds, ok := parseSRTTimestamp(val); ok {
					state.subtitleCues[idx].Start = seconds
				}
			}

			endEntry := widget.NewEntry()
			endEntry.SetPlaceHolder("00:00:00,000")
			endEntry.SetText(formatSRTTimestamp(cue.End))
			endEntry.OnChanged = func(val string) {
				if seconds, ok := parseSRTTimestamp(val); ok {
					state.subtitleCues[idx].End = seconds
				}
			}

			textEntry := widget.NewMultiLineEntry()
			textEntry.SetText(cue.Text)
			textEntry.Wrapping = fyne.TextWrapWord
			textEntry.OnChanged = func(val string) {
				state.subtitleCues[idx].Text = val
			}

			removeBtn := widget.NewButton("Remove", func() {
				state.subtitleCues = append(state.subtitleCues[:idx], state.subtitleCues[idx+1:]...)
				rebuildCues()
			})
			removeBtn.Importance = widget.MediumImportance

			timesCol := container.NewVBox(
				widget.NewLabel("Start"),
				startEntry,
				widget.NewLabel("End"),
				endEntry,
			)

			row := container.NewBorder(nil, nil, timesCol, removeBtn, textEntry)
			cardBg := canvas.NewRectangle(utils.MustHex("#171C2A"))
			cardBg.CornerRadius = 6
			// cardBg.SetMinSize(fyne.NewSize(0, startEntry.MinSize().Height+endEntry.MinSize().Height+textEntry.MinSize().Height+24)) // Removed for flexible sizing
			cueList.Add(container.NewPadded(container.NewMax(cardBg, row)))
		}
		cueList.Refresh()
	}
	state.subtitleCuesRefresh = rebuildCues

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
			if videoPath == "" && state.isVideoFile(path) {
				videoPath = path
				logging.Debug(logging.CatModule, "subtitles handleDrop: identified as video: %s", path)
			}
			if subtitlePath == "" && state.isSubtitleFile(path) {
				subtitlePath = path
				logging.Debug(logging.CatModule, "subtitles handleDrop: identified as subtitle: %s", path)
			}
		}
		if videoPath != "" {
			logging.Debug(logging.CatModule, "subtitles handleDrop: setting video path to %s", videoPath)
			state.subtitleVideoPath = videoPath
			videoEntry.SetText(videoPath)
			logging.Debug(logging.CatModule, "subtitles handleDrop: videoEntry text set to %s", videoPath)
		}
		if subtitlePath != "" {
			logging.Debug(logging.CatModule, "subtitles handleDrop: setting subtitle path to %s", subtitlePath)
			subtitleEntry.SetText(subtitlePath)
			if err := state.loadSubtitleFile(subtitlePath); err != nil {
				state.setSubtitleStatus(err.Error())
			}
			rebuildCues()
		}
	}

	emptyLabel := widget.NewLabel("Drag and drop subtitle files here\nor generate subtitles from speech")
	emptyLabel.Alignment = fyne.TextAlignCenter
	emptyOverlay = container.NewCenter(emptyLabel)

	listArea := container.NewMax(listScroll, emptyOverlay)

	addCueBtn := widget.NewButton("Add Cue", func() {
		start := 0.0
		if len(state.subtitleCues) > 0 {
			start = state.subtitleCues[len(state.subtitleCues)-1].End
		}
		state.subtitleCues = append(state.subtitleCues, subtitleCue{
			Start: start,
			End:   start + 2.0,
			Text:  "",
		})
		rebuildCues()
	})
	addCueBtn.Importance = widget.HighImportance

	clearBtn := widget.NewButton("Clear All", func() {
		state.subtitleCues = nil
		rebuildCues()
	})

	loadBtn := widget.NewButton("Load Subtitles", func() {
		if err := state.loadSubtitleFile(state.subtitleFilePath); err != nil {
			state.setSubtitleStatus(err.Error())
			return
		}
		rebuildCues()
	})

	saveBtn := widget.NewButton("Save Subtitles", func() {
		path := strings.TrimSpace(state.subtitleFilePath)
		if path == "" {
			path = defaultSubtitlePath(state.subtitleVideoPath)
			state.subtitleFilePath = path
			subtitleEntry.SetText(path)
		}
		if err := state.saveSubtitleFile(path); err != nil {
			state.setSubtitleStatus(err.Error())
			return
		}
		state.setSubtitleStatus(fmt.Sprintf("Saved subtitles to %s", filepath.Base(path)))
	})

	generateBtn := widget.NewButton("Generate From Speech (Offline)", func() {
		state.generateSubtitlesFromSpeech()
		rebuildCues()
	})
	generateBtn.Importance = widget.HighImportance

	outputModeSelect := widget.NewSelect(
		[]string{subtitleModeExternal, subtitleModeEmbed, subtitleModeBurn},
		func(val string) {
			state.subtitleOutputMode = val
			state.persistSubtitlesConfig()
		},
	)
	outputModeSelect.SetSelected(state.subtitleOutputMode)

	applyBtn := widget.NewButton("Create Output", func() {
		state.applySubtitlesToVideo()
	})
	applyBtn.Importance = widget.HighImportance

	browseVideoBtn := widget.NewButton("Browse", func() {
		dialog.ShowFileOpen(func(file fyne.URIReadCloser, err error) {
			if err != nil || file == nil {
				return
			}
			defer file.Close()
			path := file.URI().Path()
			state.subtitleVideoPath = path
			videoEntry.SetText(path)
		}, state.window)
	})

	browseSubtitleBtn := widget.NewButton("Browse", func() {
		dialog.ShowFileOpen(func(file fyne.URIReadCloser, err error) {
			if err != nil || file == nil {
				return
			}
			defer file.Close()
			path := file.URI().Path()
			if err := state.loadSubtitleFile(path); err != nil {
				state.setSubtitleStatus(err.Error())
				return
			}
			subtitleEntry.SetText(path)
			rebuildCues()
		}, state.window)
	})

	offsetEntry := widget.NewEntry()
	offsetEntry.SetPlaceHolder("0.0")
	offsetEntry.SetText(fmt.Sprintf("%.2f", state.subtitleTimeOffset))
	offsetEntry.OnChanged = func(val string) {
		if offset, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
			state.subtitleTimeOffset = offset
			state.persistSubtitlesConfig()
		}
	}

	applyOffsetBtn := widget.NewButton("Apply Offset", func() {
		state.applySubtitleTimeOffset(state.subtitleTimeOffset)
	})
	applyOffsetBtn.Importance = widget.HighImportance

	offsetPlus1Btn := widget.NewButton("+1s", func() {
		state.applySubtitleTimeOffset(1.0)
	})

	offsetMinus1Btn := widget.NewButton("-1s", func() {
		state.applySubtitleTimeOffset(-1.0)
	})

	offsetPlus01Btn := widget.NewButton("+0.1s", func() {
		state.applySubtitleTimeOffset(0.1)
	})

	offsetMinus01Btn := widget.NewButton("-0.1s", func() {
		state.applySubtitleTimeOffset(-0.1)
	})

	applyControls := func() {
		outputModeSelect.SetSelected(state.subtitleOutputMode)
		backendEntry.SetText(state.subtitleBackendPath)
		modelEntry.SetText(state.subtitleModelPath)
		outputEntry.SetText(state.subtitleBurnOutput)
		offsetEntry.SetText(fmt.Sprintf("%.2f", state.subtitleTimeOffset))
		refreshWhisperUI()
	}

	loadCfgBtn := widget.NewButton("Load Config", func() {
		cfg, err := loadPersistedSubtitlesConfig()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				dialog.ShowInformation("No Config", "No saved config found yet. It will save automatically after your first change.", state.window)
			} else {
				dialog.ShowError(fmt.Errorf("failed to load config: %w", err), state.window)
			}
			return
		}
		state.applySubtitlesConfig(cfg)
		applyControls()
	})

	saveCfgBtn := widget.NewButton("Save Config", func() {
		cfg := subtitlesConfig{
			OutputMode:  state.subtitleOutputMode,
			ModelPath:   state.subtitleModelPath,
			BackendPath: state.subtitleBackendPath,
			BurnOutput:  state.subtitleBurnOutput,
			TimeOffset:  state.subtitleTimeOffset,
		}
		if err := savePersistedSubtitlesConfig(cfg); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save config: %w", err), state.window)
			return
		}
		dialog.ShowInformation("Config Saved", fmt.Sprintf("Saved to %s", moduleConfigPath("subtitles")), state.window)
	})

	resetBtn := widget.NewButton("Reset", func() {
		cfg := defaultSubtitlesConfig()
		state.applySubtitlesConfig(cfg)
		applyControls()
		state.persistSubtitlesConfig()
	})

	left := container.NewVBox(
		widget.NewLabelWithStyle("Sources", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, nil, browseVideoBtn, videoEntry),
		container.NewBorder(nil, nil, nil, browseSubtitleBtn, subtitleEntry),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Timing Adjustment", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Shift all subtitle times by offset (seconds):"),
		offsetEntry,
		container.NewHBox(offsetMinus1Btn, offsetMinus01Btn, offsetPlus01Btn, offsetPlus1Btn),
		applyOffsetBtn,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Offline Speech-to-Text (whisper.cpp)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		offlineHint,
		backendLabel,
		backendEntry,
		modelLabel,
		modelEntry,
		container.NewHBox(generateBtn),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Output", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		outputModeSelect,
		outputEntry,
		applyBtn,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Status", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		statusScroll,
		container.NewHBox(copyStatusBtn),
		widget.NewSeparator(),
		container.NewHBox(resetBtn, loadCfgBtn, saveCfgBtn),
	)

	right := container.NewBorder(
		container.NewVBox(
			widget.NewLabelWithStyle("Subtitle Cues", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			container.NewHBox(addCueBtn, clearBtn, loadBtn, saveBtn),
		),
		nil,
		nil,
		nil,
		listArea,
	)

	rebuildCues()

	// Wrap both panels in droppable so drops anywhere will work
	droppableLeft := ui.NewDroppable(left, handleDrop)
	droppableRight := ui.NewDroppable(right, handleDrop)
	content := container.NewGridWithColumns(2, droppableLeft, droppableRight)
	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(0, 0))
	return container.NewBorder(topBar, bottomBar, nil, nil, scroll)
}

func (s *appState) setSubtitleStatus(msg string) {
	s.subtitleStatus = msg
	if s.subtitleStatusLabel != nil {
		s.subtitleStatusLabel.SetText(msg)
	}
}

func (s *appState) setSubtitleStatusAsync(msg string) {
	app := fyne.CurrentApp()
	if app == nil || app.Driver() == nil {
		s.setSubtitleStatus(msg)
		return
	}
	app.Driver().DoFromGoroutine(func() {
		s.setSubtitleStatus(msg)
	}, false)
}

func (s *appState) handleSubtitlesModuleDrop(items []fyne.URI) {
	logging.Debug(logging.CatModule, "handleSubtitlesModuleDrop called with %d items", len(items))
	var videoPath string
	var subtitlePath string
	for _, uri := range items {
		logging.Debug(logging.CatModule, "handleSubtitlesModuleDrop: uri scheme=%s path=%s", uri.Scheme(), uri.Path())
		if uri.Scheme() != "file" {
			continue
		}
		path := uri.Path()
		if videoPath == "" && s.isVideoFile(path) {
			videoPath = path
			logging.Debug(logging.CatModule, "handleSubtitlesModuleDrop: identified as video: %s", path)
		}
		if subtitlePath == "" && s.isSubtitleFile(path) {
			subtitlePath = path
			logging.Debug(logging.CatModule, "handleSubtitlesModuleDrop: identified as subtitle: %s", path)
		}
	}
	if videoPath == "" && subtitlePath == "" {
		logging.Debug(logging.CatModule, "handleSubtitlesModuleDrop: no video or subtitle found, returning")
		return
	}
	if videoPath != "" {
		logging.Debug(logging.CatModule, "handleSubtitlesModuleDrop: setting subtitleVideoPath to %s", videoPath)
		s.subtitleVideoPath = videoPath
	}
	if subtitlePath != "" {
		logging.Debug(logging.CatModule, "handleSubtitlesModuleDrop: loading subtitle file %s", subtitlePath)
		if err := s.loadSubtitleFile(subtitlePath); err != nil {
			s.setSubtitleStatus(err.Error())
		}
	}

	// Switch to subtitles module to show the loaded files
	logging.Debug(logging.CatModule, "handleSubtitlesModuleDrop: calling showModule(subtitles), subtitleVideoPath=%s", s.subtitleVideoPath)
	s.showModule("subtitles")
}

func (s *appState) loadSubtitleFile(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("subtitle path is empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read subtitles: %w", err)
	}
	cues, err := parseSubtitlePayload(path, string(data))
	if err != nil {
		return err
	}
	s.subtitleFilePath = path
	s.subtitleCues = cues
	s.setSubtitleStatus(fmt.Sprintf("Loaded %d subtitle cues", len(cues)))
	return nil
}

func (s *appState) saveSubtitleFile(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("subtitle output path is empty")
	}
	if len(s.subtitleCues) == 0 {
		return fmt.Errorf("no subtitle cues to save")
	}
	payload := formatSRT(s.subtitleCues)
	if err := os.WriteFile(path, []byte(payload), 0644); err != nil {
		return fmt.Errorf("failed to write subtitles: %w", err)
	}
	return nil
}

func (s *appState) applySubtitleTimeOffset(offsetSeconds float64) {
	if len(s.subtitleCues) == 0 {
		s.setSubtitleStatus("No subtitle cues to adjust")
		return
	}
	for i := range s.subtitleCues {
		s.subtitleCues[i].Start += offsetSeconds
		s.subtitleCues[i].End += offsetSeconds
		if s.subtitleCues[i].Start < 0 {
			s.subtitleCues[i].Start = 0
		}
		if s.subtitleCues[i].End < 0 {
			s.subtitleCues[i].End = 0
		}
	}
	if s.subtitleCuesRefresh != nil {
		s.subtitleCuesRefresh()
	}
	s.setSubtitleStatus(fmt.Sprintf("Applied %.2fs offset to %d subtitle cues", offsetSeconds, len(s.subtitleCues)))
}

func (s *appState) generateSubtitlesFromSpeech() {
	videoPath := strings.TrimSpace(s.subtitleVideoPath)
	if videoPath == "" {
		s.setSubtitleStatus("Set a video file to generate subtitles.")
		return
	}
	if _, err := os.Stat(videoPath); err != nil {
		s.setSubtitleStatus("Video file not found.")
		return
	}
	modelPath := strings.TrimSpace(s.subtitleModelPath)
	if modelPath == "" {
		s.setSubtitleStatus("Whisper model missing. Install ggml-small.bin (vendor/whisper) or set a model path.")
		return
	}
	backendPath := strings.TrimSpace(s.subtitleBackendPath)
	if backendPath == "" {
		if detected := detectWhisperBackend(); detected != "" {
			backendPath = detected
			s.subtitleBackendPath = detected
		}
	}
	if backendPath == "" {
		s.setSubtitleStatus("Whisper backend not found. Set the backend path.")
		return
	}

	outputPath := strings.TrimSpace(s.subtitleFilePath)
	if outputPath == "" {
		outputPath = defaultSubtitlePath(videoPath)
		s.subtitleFilePath = outputPath
	}
	baseOutput := strings.TrimSuffix(outputPath, filepath.Ext(outputPath))

	go func() {
		tmpWav := filepath.Join(os.TempDir(), fmt.Sprintf("vt-stt-%d.wav", time.Now().UnixNano()))
		defer os.Remove(tmpWav)

		s.setSubtitleStatusAsync("Extracting audio for speech-to-text...")
		if err := runFFmpeg([]string{
			"-y",
			"-i", videoPath,
			"-vn",
			"-ac", "1",
			"-ar", "16000",
			"-f", "wav",
			tmpWav,
		}); err != nil {
			s.setSubtitleStatusAsync(fmt.Sprintf("Audio extraction failed: %v", err))
			return
		}

		s.setSubtitleStatusAsync("Running offline speech-to-text...")
		if err := runWhisper(backendPath, modelPath, tmpWav, baseOutput); err != nil {
			s.setSubtitleStatusAsync(fmt.Sprintf("Speech-to-text failed: %v", err))
			return
		}

		finalPath := baseOutput + ".srt"
		if err := s.loadSubtitleFile(finalPath); err != nil {
			s.setSubtitleStatusAsync(err.Error())
			return
		}
		s.setSubtitleStatusAsync(fmt.Sprintf("Generated subtitles: %s", filepath.Base(finalPath)))
		app := fyne.CurrentApp()
		if app != nil && app.Driver() != nil {
			app.Driver().DoFromGoroutine(func() {
				if s.active == "subtitles" {
					s.showSubtitlesView()
				}
			}, false)
		}
	}()
}

func (s *appState) applySubtitlesToVideo() {
	videoPath := strings.TrimSpace(s.subtitleVideoPath)
	if videoPath == "" {
		s.setSubtitleStatus("Set a video file before creating output.")
		return
	}
	if _, err := os.Stat(videoPath); err != nil {
		s.setSubtitleStatus("Video file not found.")
		return
	}

	mode := s.subtitleOutputMode
	if mode == "" {
		mode = subtitleModeExternal
	}

	subPath := strings.TrimSpace(s.subtitleFilePath)
	if subPath == "" {
		subPath = defaultSubtitlePath(videoPath)
		s.subtitleFilePath = subPath
	}

	if err := s.saveSubtitleFile(subPath); err != nil {
		s.setSubtitleStatus(err.Error())
		return
	}

	if mode == subtitleModeExternal {
		s.setSubtitleStatus(fmt.Sprintf("Saved subtitles to %s", filepath.Base(subPath)))
		return
	}

	outputPath := strings.TrimSpace(s.subtitleBurnOutput)
	if outputPath == "" {
		outputPath = defaultSubtitleOutputPath(videoPath)
		s.subtitleBurnOutput = outputPath
	}

	go func() {
		s.setSubtitleStatusAsync("Creating output with subtitles...")
		var args []string
		switch mode {
		case subtitleModeEmbed:
			subCodec := subtitleCodecForOutput(outputPath)
			args = []string{
				"-y",
				"-i", videoPath,
				"-i", subPath,
				"-map", "0",
				"-map", "1",
				"-c", "copy",
				"-c:s", subCodec,
				outputPath,
			}
		case subtitleModeBurn:
			filterPath := escapeFFmpegFilterPath(subPath)
			args = []string{
				"-y",
				"-i", videoPath,
				"-vf", fmt.Sprintf("subtitles=%s", filterPath),
				"-c:v", "libx264",
				"-crf", "18",
				"-preset", "fast",
				"-c:a", "copy",
				outputPath,
			}
		}

		if err := runFFmpeg(args); err != nil {
			s.setSubtitleStatusAsync(fmt.Sprintf("Subtitle output failed: %v", err))
			return
		}
		s.setSubtitleStatusAsync(fmt.Sprintf("Output created: %s", filepath.Base(outputPath)))
	}()
}

func parseSubtitlePayload(path, content string) ([]subtitleCue, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".vtt":
		content = stripVTTHeader(content)
		return parseSRT(content), nil
	case ".srt":
		return parseSRT(content), nil
	case ".ass", ".ssa":
		return nil, fmt.Errorf("ASS/SSA subtitles are not supported yet")
	default:
		return nil, fmt.Errorf("unsupported subtitle format")
	}
}

func stripVTTHeader(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	var kept []string
	for i, line := range lines {
		if i == 0 && strings.HasPrefix(strings.TrimSpace(line), "WEBVTT") {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "NOTE") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func parseSRT(content string) []subtitleCue {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	scanner := bufio.NewScanner(strings.NewReader(content))
	var cues []subtitleCue
	var inCue bool
	var start float64
	var end float64
	var lines []string

	flush := func() {
		if inCue && len(lines) > 0 {
			cues = append(cues, subtitleCue{
				Start: start,
				End:   end,
				Text:  strings.Join(lines, "\n"),
			})
		}
		inCue = false
		lines = nil
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			flush()
			continue
		}

		if strings.Contains(line, "-->") {
			parts := strings.Split(line, "-->")
			if len(parts) >= 2 {
				if s, ok := parseSRTTimestamp(strings.TrimSpace(parts[0])); ok {
					if e, ok := parseSRTTimestamp(strings.TrimSpace(parts[1])); ok {
						start = s
						end = e
						inCue = true
						lines = nil
						continue
					}
				}
			}
		}

		if !inCue {
			continue
		}
		lines = append(lines, line)
	}

	flush()
	return cues
}

func parseSRTTimestamp(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	value = strings.ReplaceAll(value, ",", ".")
	parts := strings.Split(value, ":")
	if len(parts) != 3 {
		return 0, false
	}
	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, false
	}
	minutes, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, false
	}
	secParts := strings.SplitN(parts[2], ".", 2)
	seconds, err := strconv.Atoi(secParts[0])
	if err != nil {
		return 0, false
	}
	ms := 0
	if len(secParts) == 2 {
		msStr := secParts[1]
		if len(msStr) > 3 {
			msStr = msStr[:3]
		}
		for len(msStr) < 3 {
			msStr += "0"
		}
		ms, err = strconv.Atoi(msStr)
		if err != nil {
			return 0, false
		}
	}
	totalMs := ((hours*60+minutes)*60+seconds)*1000 + ms
	return float64(totalMs) / 1000.0, true
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

func formatSRT(cues []subtitleCue) string {
	var b strings.Builder
	for i, cue := range cues {
		b.WriteString(fmt.Sprintf("%d\n", i+1))
		b.WriteString(fmt.Sprintf("%s --> %s\n", formatSRTTimestamp(cue.Start), formatSRTTimestamp(cue.End)))
		b.WriteString(strings.TrimSpace(cue.Text))
		b.WriteString("\n\n")
	}
	return b.String()
}

func defaultSubtitlePath(videoPath string) string {
	if videoPath == "" {
		return ""
	}
	dir := filepath.Dir(videoPath)
	base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	return filepath.Join(dir, base+".srt")
}

func defaultSubtitleOutputPath(videoPath string) string {
	if videoPath == "" {
		return ""
	}
	dir := filepath.Dir(videoPath)
	base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	ext := filepath.Ext(videoPath)
	if ext == "" {
		ext = ".mp4"
	}
	return filepath.Join(dir, base+"-subtitled"+ext)
}

func subtitleCodecForOutput(outputPath string) string {
	ext := strings.ToLower(filepath.Ext(outputPath))
	switch ext {
	case ".mp4", ".m4v", ".mov":
		return "mov_text"
	default:
		return "srt"
	}
}

func escapeFFmpegFilterPath(path string) string {
	escaped := strings.ReplaceAll(path, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, ":", "\\:")
	escaped = strings.ReplaceAll(escaped, "'", "\\'")
	return escaped
}

func detectWhisperBackend() string {
	candidates := []string{"whisper.cpp", "whisper", "main", "main.exe", "whisper.exe"}
	for _, candidate := range candidates {
		if found, err := exec.LookPath(candidate); err == nil {
			return found
		}
	}
	return ""
}

func detectWhisperModel() string {
	preferred := []string{
		filepath.Join("models", "ggml-small.bin"),
		filepath.Join("models", "ggml-base.bin"),
		filepath.Join("models", "ggml-medium.bin"),
		filepath.Join("models", "ggml-large.bin"),
		filepath.Join("vendor", "whisper", "ggml-small.bin"),
		filepath.Join("vendor", "whisper", "ggml-base.bin"),
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		preferred = append(preferred, filepath.Join(dir, "vendor", "whisper", "ggml-small.bin"))
		preferred = append(preferred, filepath.Join(dir, "vendor", "whisper", "ggml-base.bin"))
		preferred = append(preferred, filepath.Join(dir, "models", "ggml-small.bin"))
		preferred = append(preferred, filepath.Join(dir, "models", "ggml-base.bin"))
	}
	for _, candidate := range preferred {
		if path, err := filepath.Abs(candidate); err == nil {
			if _, statErr := os.Stat(path); statErr == nil {
				return path
			}
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	home, _ := os.UserHomeDir()
	search := []string{}
	if home != "" {
		search = append(search,
			filepath.Join(home, ".cache", "whisper"),
			filepath.Join(home, ".local", "share", "whisper.cpp"),
			filepath.Join(home, "whisper.cpp", "models"),
		)
	}

	for _, dir := range search {
		matches, _ := filepath.Glob(filepath.Join(dir, "ggml-*.bin"))
		if len(matches) == 0 {
			continue
		}
		for _, match := range matches {
			base := filepath.Base(match)
			if base == "ggml-small.bin" {
				return match
			}
		}
		for _, match := range matches {
			base := filepath.Base(match)
			if base == "ggml-base.bin" {
				return match
			}
		}
		return matches[0]
	}
	return ""
}

func runWhisper(binaryPath, modelPath, inputPath, outputBase string) error {
	args := []string{
		"-m", modelPath,
		"-f", inputPath,
		"-of", outputBase,
		"-osrt",
	}
	stderr, err := runWhisperCommand(binaryPath, args)
	if err == nil {
		return nil
	}

	lower := strings.ToLower(stderr)
	if strings.Contains(lower, "usage: whisper") ||
		strings.Contains(lower, "argument --output_format") ||
		strings.Contains(lower, "unrecognized arguments: -m") {
		return runPythonWhisper(binaryPath, modelPath, inputPath, outputBase)
	}
	return fmt.Errorf("whisper failed: %w (%s)", err, strings.TrimSpace(stderr))
}

func runWhisperCommand(binaryPath string, args []string) (string, error) {
	cmd := exec.Command(binaryPath, args...)
	utils.ApplyNoWindow(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	return strings.TrimSpace(stderr.String()), err
}

func runPythonWhisper(binaryPath, modelPath, inputPath, outputBase string) error {
	model := strings.TrimSpace(modelPath)
	if model == "" {
		return fmt.Errorf("whisper model is required")
	}
	if strings.HasSuffix(strings.ToLower(model), ".bin") || strings.Contains(model, string(os.PathSeparator)) {
		return fmt.Errorf("whisper backend is python CLI; set model name (e.g., base, small) or use whisper.cpp with ggml models")
	}

	outputDir := filepath.Dir(outputBase)
	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	target := outputBase + ".srt"

	args := []string{
		"--model", model,
		"--output_format", "srt",
		"--output_dir", outputDir,
		inputPath,
	}
	stderr, err := runWhisperCommand(binaryPath, args)
	if err != nil {
		return fmt.Errorf("whisper failed: %w (%s)", err, strings.TrimSpace(stderr))
	}

	generated := filepath.Join(outputDir, base+".srt")
	if generated != target {
		if err := os.Rename(generated, target); err != nil {
			return fmt.Errorf("whisper output rename failed: %w", err)
		}
	}
	return nil
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
