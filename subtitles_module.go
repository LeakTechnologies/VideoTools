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
	"sort"
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
	subtitleModeExternal = "External Subtitle File"
	subtitleModeEmbed    = "Embed Subtitle Track"
	subtitleModeBurn     = "Burn In Subtitles"
)

type subtitleCue struct {
	Start float64
	End   float64
	Text  string
}

type subtitleStreamInfo struct {
	Index    int
	Codec    string
	Language string
	Title    string
	Default  bool
	Forced   bool
	IsText   bool
	IsImage  bool
}

type subtitlesConfig struct {
	OutputMode  string  `json:"outputMode"`
	ModelPath   string  `json:"modelPath"`
	BackendPath string  `json:"backendPath"`
	BurnOutput  string  `json:"burnOutput"`
	TimeOffset  float64 `json:"timeOffset"`
	OCRLanguage string  `json:"ocrLanguage"`
	OCROutput   string  `json:"ocrOutput"`
}

func defaultSubtitlesConfig() subtitlesConfig {
	return subtitlesConfig{
		OutputMode:  subtitleModeExternal,
		ModelPath:   "",
		BackendPath: "",
		BurnOutput:  "",
		OCRLanguage: "eng",
		OCROutput:   "srt",
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
	if cfg.OutputMode == "External SRT" {
		cfg.OutputMode = subtitleModeExternal
	}
	if cfg.OCRLanguage == "" {
		cfg.OCRLanguage = "eng"
	}
	if cfg.OCROutput == "" {
		cfg.OCROutput = "srt"
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
	s.subtitleOCRLanguage = cfg.OCRLanguage
	s.subtitleOCROutput = cfg.OCROutput
}

func (s *appState) persistSubtitlesConfig() {
	cfg := subtitlesConfig{
		OutputMode:  s.subtitleOutputMode,
		ModelPath:   s.subtitleModelPath,
		BackendPath: s.subtitleBackendPath,
		BurnOutput:  s.subtitleBurnOutput,
		TimeOffset:  s.subtitleTimeOffset,
		OCRLanguage: s.subtitleOCRLanguage,
		OCROutput:   s.subtitleOCROutput,
	}
	if err := savePersistedSubtitlesConfig(cfg); err != nil {
		logging.Debug(logging.CatSystem, "failed to persist subtitles config: %v", err)
	}
}

func isTextSubtitleCodec(codec string) bool {
	switch strings.ToLower(codec) {
	case "subrip", "srt", "ass", "ssa", "mov_text", "webvtt", "text", "ttml":
		return true
	default:
		return false
	}
}

func isImageSubtitleCodec(codec string) bool {
	switch strings.ToLower(codec) {
	case "hdmv_pgs_subtitle", "dvd_subtitle", "dvb_subtitle", "xsub":
		return true
	default:
		return false
	}
}

func subtitleStreamLabel(info subtitleStreamInfo) string {
	lang := strings.TrimSpace(info.Language)
	if lang == "" {
		lang = "und"
	}
	title := strings.TrimSpace(info.Title)
	if title != "" {
		title = " - " + title
	}
	flags := []string{}
	if info.Default {
		flags = append(flags, "default")
	}
	if info.Forced {
		flags = append(flags, "forced")
	}
	flagText := ""
	if len(flags) > 0 {
		flagText = " (" + strings.Join(flags, ", ") + ")"
	}
	return fmt.Sprintf("#%d | %s | %s%s%s", info.Index, strings.ToUpper(lang), info.Codec, title, flagText)
}

func probeSubtitleStreams(path string) ([]subtitleStreamInfo, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("subtitle source path is empty")
	}
	cmd := exec.Command(utils.GetFFprobePath(),
		"-hide_banner",
		"-v", "error",
		"-select_streams", "s",
		"-show_streams",
		"-of", "json",
		path,
	)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}

	type ffprobeStream struct {
		Index       int               `json:"index"`
		CodecName   string            `json:"codec_name"`
		CodecType   string            `json:"codec_type"`
		Tags        map[string]string `json:"tags"`
		Disposition map[string]int    `json:"disposition"`
	}
	type ffprobeResp struct {
		Streams []ffprobeStream `json:"streams"`
	}
	var resp ffprobeResp
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	var results []subtitleStreamInfo
	for _, s := range resp.Streams {
		if s.CodecType != "subtitle" {
			continue
		}
		lang := ""
		title := ""
		if s.Tags != nil {
			lang = s.Tags["language"]
			if lang == "" {
				lang = s.Tags["LANGUAGE"]
			}
			title = s.Tags["title"]
			if title == "" {
				title = s.Tags["TITLE"]
			}
		}
		info := subtitleStreamInfo{
			Index:    s.Index,
			Codec:    s.CodecName,
			Language: lang,
			Title:    title,
			Default:  s.Disposition != nil && s.Disposition["default"] == 1,
			Forced:   s.Disposition != nil && s.Disposition["forced"] == 1,
			IsText:   isTextSubtitleCodec(s.CodecName),
			IsImage:  isImageSubtitleCodec(s.CodecName),
		}
		results = append(results, info)
	}

	return results, nil
}

type subtitlePacketInfo struct {
	Start    float64
	Duration float64
}

func probeSubtitlePackets(path string, streamIndex int) ([]subtitlePacketInfo, error) {
	cmd := exec.Command(utils.GetFFprobePath(),
		"-hide_banner",
		"-v", "error",
		"-show_packets",
		"-of", "json",
		path,
	)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe packets failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}

	type ffprobePacket struct {
		StreamIndex  int    `json:"stream_index"`
		PtsTime      string `json:"pts_time"`
		DtsTime      string `json:"dts_time"`
		DurationTime string `json:"duration_time"`
	}
	type ffprobeResp struct {
		Packets []ffprobePacket `json:"packets"`
	}
	var resp ffprobeResp
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe packets: %w", err)
	}

	var packets []subtitlePacketInfo
	for _, p := range resp.Packets {
		if p.StreamIndex != streamIndex {
			continue
		}
		startStr := strings.TrimSpace(p.PtsTime)
		if startStr == "" {
			startStr = strings.TrimSpace(p.DtsTime)
		}
		if startStr == "" {
			continue
		}
		start, err := strconv.ParseFloat(startStr, 64)
		if err != nil {
			continue
		}
		duration := 0.0
		if strings.TrimSpace(p.DurationTime) != "" {
			if d, err := strconv.ParseFloat(strings.TrimSpace(p.DurationTime), 64); err == nil {
				duration = d
			}
		}
		packets = append(packets, subtitlePacketInfo{Start: start, Duration: duration})
	}
	return normalizeSubtitlePackets(packets), nil
}

func normalizeSubtitlePackets(packets []subtitlePacketInfo) []subtitlePacketInfo {
	if len(packets) == 0 {
		return packets
	}
	for i := range packets {
		if packets[i].Duration > 0 {
			continue
		}
		if i+1 < len(packets) {
			next := packets[i+1].Start - packets[i].Start
			if next > 0 {
				packets[i].Duration = next
				continue
			}
		}
		packets[i].Duration = 2.0
	}
	return packets
}

func normalizeOCRText(text string) string {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return ""
	}
	clean = strings.ReplaceAll(clean, "\r\n", "\n")
	clean = strings.ReplaceAll(clean, "\r", "\n")
	lines := strings.Split(clean, "\n")
	var kept []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		kept = append(kept, trimmed)
	}
	clean = strings.Join(kept, "\n")
	clean = strings.Join(strings.Fields(clean), " ")
	return clean
}

func mergeOCRCues(cues []subtitleCue) []subtitleCue {
	if len(cues) == 0 {
		return cues
	}
	var merged []subtitleCue
	for _, cue := range cues {
		text := normalizeOCRText(cue.Text)
		if text == "" {
			continue
		}
		cue.Text = text
		if len(merged) == 0 {
			merged = append(merged, cue)
			continue
		}
		last := &merged[len(merged)-1]
		// Merge identical text within a small gap.
		if strings.EqualFold(last.Text, cue.Text) && cue.Start <= last.End+0.25 {
			if cue.End > last.End {
				last.End = cue.End
			}
			continue
		}
		// Clamp overlaps.
		if cue.Start < last.End {
			cue.Start = last.End
		}
		if cue.End <= cue.Start {
			cue.End = cue.Start + 1.5
		}
		merged = append(merged, cue)
	}
	return merged
}

func extractSubtitleFrames(videoPath string, info subtitleStreamInfo, dir string) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	pattern := filepath.Join(dir, "sub_%05d.png")
	args := []string{
		"-y",
		"-i", videoPath,
		"-map", fmt.Sprintf("0:%d", info.Index),
		"-vsync", "0",
		"-f", "image2",
		"-c:v", "png",
		pattern,
	}
	if err := runFFmpeg(args); err != nil {
		return nil, err
	}
	frames, err := filepath.Glob(filepath.Join(dir, "sub_*.png"))
	if err != nil {
		return nil, err
	}
	sort.Strings(frames)
	return frames, nil
}

func ocrSubtitleStream(videoPath string, info subtitleStreamInfo, outputPath string, ocrLang string, ocrOutput string) (string, error) {
	if _, err := exec.LookPath("tesseract"); err != nil {
		return "", fmt.Errorf("tesseract not found; install it to OCR image-based subtitles")
	}
	tmpDir, err := os.MkdirTemp("", "vt-sub-ocr-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	packets, err := probeSubtitlePackets(videoPath, info.Index)
	if err != nil {
		return "", err
	}
	if len(packets) == 0 {
		return "", fmt.Errorf("no subtitle packets found for OCR")
	}

	frames, err := extractSubtitleFrames(videoPath, info, tmpDir)
	if err != nil {
		return "", err
	}
	if len(frames) == 0 {
		return "", fmt.Errorf("no subtitle images extracted for OCR")
	}

	count := len(packets)
	if len(frames) < count {
		count = len(frames)
	}

	var cues []subtitleCue
	for i := 0; i < count; i++ {
		text, err := runTesseract(frames[i], ocrLang)
		if err != nil {
			return "", err
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		start := packets[i].Start
		end := packets[i].Start + packets[i].Duration
		if end <= start {
			end = start + 2.0
		}
		cues = append(cues, subtitleCue{
			Start: start,
			End:   end,
			Text:  text,
		})
	}

	cues = mergeOCRCues(cues)
	if len(cues) == 0 {
		return "", fmt.Errorf("OCR completed but produced no subtitle text")
	}

	var payload string
	if strings.EqualFold(strings.TrimSpace(ocrOutput), "ass") {
		payload = formatASS(cues)
	} else {
		payload = formatSRT(cues)
	}

	if err := os.WriteFile(outputPath, []byte(payload), 0o644); err != nil {
		return "", fmt.Errorf("failed to write OCR subtitles: %w", err)
	}
	return outputPath, nil
}

func runTesseract(imagePath string, lang string) (string, error) {
	if strings.TrimSpace(lang) == "" {
		lang = "eng"
	}
	args := []string{imagePath, "stdout", "-l", lang, "--dpi", "300"}
	cmd := exec.Command("tesseract", args...)
	utils.ApplyNoWindow(cmd)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("tesseract failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func subtitleRipOutputPath(videoPath string, info subtitleStreamInfo, mode string, outputFormat string) string {
	dir := filepath.Dir(videoPath)
	base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	lang := strings.TrimSpace(info.Language)
	if lang == "" {
		lang = "und"
	}
	suffix := fmt.Sprintf("subtrack%d-%s", info.Index, sanitizeForPath(lang))
	ext := ".srt"
	if strings.Contains(strings.ToLower(mode), "original") {
		ext = ".mks"
	} else if strings.EqualFold(strings.TrimSpace(outputFormat), "ass") {
		ext = ".ass"
	}
	return filepath.Join(dir, fmt.Sprintf("%s.%s%s", base, suffix, ext))
}

func extractSubtitleStream(videoPath string, info subtitleStreamInfo, mode string, ocrLang string, ocrOutput string) (string, error) {
	outputPath := subtitleRipOutputPath(videoPath, info, mode, ocrOutput)
	args := []string{
		"-y",
		"-i", videoPath,
		"-map", fmt.Sprintf("0:%d", info.Index),
	}

	if strings.Contains(strings.ToLower(mode), "original") {
		args = append(args, "-c:s", "copy")
	} else if info.IsText {
		if strings.EqualFold(strings.TrimSpace(ocrOutput), "ass") {
			args = append(args, "-c:s", "ass")
		} else {
			args = append(args, "-c:s", "srt", "-f", "srt")
		}
	} else {
		return ocrSubtitleStream(videoPath, info, outputPath, ocrLang, ocrOutput)
	}
	args = append(args, outputPath)

	if err := runFFmpeg(args); err != nil {
		return "", err
	}
	return outputPath, nil
}

func subtitleCodecForFile(subPath, outputPath string) string {
	ext := strings.ToLower(filepath.Ext(subPath))
	switch ext {
	case ".ass":
		return "ass"
	case ".ssa":
		return "ssa"
	case ".vtt", ".webvtt":
		return "webvtt"
	case ".srt":
		return subtitleCodecForOutput(outputPath)
	case ".mks":
		return "copy"
	default:
		return subtitleCodecForOutput(outputPath)
	}
}

func (s *appState) showSubtitlesView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "subtitles"
	s.maximizeWindow()

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
	if strings.TrimSpace(state.subtitleRipMode) == "" {
		state.subtitleRipMode = "Text (SRT/ASS)"
	}
	if strings.TrimSpace(state.subtitleOCRLanguage) == "" {
		state.subtitleOCRLanguage = "eng"
	}
	if strings.TrimSpace(state.subtitleOCROutput) == "" {
		state.subtitleOCROutput = "srt"
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
	subtitleEntry.SetPlaceHolder("Subtitle file (.srt, .vtt, or .mks)")
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

	streamSelect := widget.NewSelect([]string{}, func(val string) {
		for i, info := range state.subtitleRipStreams {
			if subtitleStreamLabel(info) == val {
				state.subtitleRipIndex = i
				return
			}
		}
	})

	ripModeSelect := widget.NewSelect([]string{"Text (SRT/ASS)", "Original (lossless)"}, func(val string) {
		state.subtitleRipMode = val
	})
	ripModeSelect.SetSelected(state.subtitleRipMode)

	ocrOutputSelect := widget.NewSelect([]string{"SRT", "ASS"}, func(val string) {
		state.subtitleOCROutput = strings.ToLower(val)
		state.persistSubtitlesConfig()
	})
	if strings.EqualFold(state.subtitleOCROutput, "ass") {
		ocrOutputSelect.SetSelected("ASS")
	} else {
		ocrOutputSelect.SetSelected("SRT")
	}

	ocrLangEntry := widget.NewEntry()
	ocrLangEntry.SetPlaceHolder("eng")
	ocrLangEntry.SetText(state.subtitleOCRLanguage)
	ocrLangEntry.OnChanged = func(val string) {
		state.subtitleOCRLanguage = strings.TrimSpace(val)
		state.persistSubtitlesConfig()
	}

	refreshRipStreams := func() {
		options := []string{}
		for _, info := range state.subtitleRipStreams {
			options = append(options, subtitleStreamLabel(info))
		}
		if len(options) == 0 {
			streamSelect.SetOptions([]string{})
			streamSelect.ClearSelected()
			state.subtitleRipIndex = 0
			return
		}
		streamSelect.SetOptions(options)
		if state.subtitleRipIndex < 0 || state.subtitleRipIndex >= len(options) {
			state.subtitleRipIndex = 0
		}
		streamSelect.SetSelected(options[state.subtitleRipIndex])
	}

	detectStreamsBtn := widget.NewButton("Detect Streams", func() {
		videoPath := strings.TrimSpace(state.subtitleVideoPath)
		if videoPath == "" {
			state.setSubtitleStatus("Set a video file before detecting subtitle streams.")
			return
		}
		streams, err := probeSubtitleStreams(videoPath)
		if err != nil {
			state.setSubtitleStatus(err.Error())
			return
		}
		if len(streams) == 0 {
			state.subtitleRipStreams = nil
			refreshRipStreams()
			state.setSubtitleStatus("No embedded subtitle streams found.")
			return
		}
		state.subtitleRipStreams = streams
		refreshRipStreams()
		state.setSubtitleStatus(fmt.Sprintf("Detected %d subtitle streams.", len(streams)))
	})

	ripBtn := widget.NewButton("Extract Selected", func() {
		videoPath := strings.TrimSpace(state.subtitleVideoPath)
		if videoPath == "" {
			state.setSubtitleStatus("Set a video file before extracting subtitles.")
			return
		}
		if len(state.subtitleRipStreams) == 0 {
			state.setSubtitleStatus("No subtitle streams detected yet.")
			return
		}
		if state.subtitleRipIndex < 0 || state.subtitleRipIndex >= len(state.subtitleRipStreams) {
			state.setSubtitleStatus("Select a subtitle stream to extract.")
			return
		}
		stream := state.subtitleRipStreams[state.subtitleRipIndex]
		if strings.Contains(strings.ToLower(state.subtitleRipMode), "text") && !stream.IsText {
			if _, err := exec.LookPath("tesseract"); err != nil {
				state.setSubtitleStatus("Selected subtitle stream is image-based. Install Tesseract or use Original (lossless).")
				return
			}
		}
		state.setSubtitleStatus("Extracting subtitles...")
		go func() {
			outputPath, err := extractSubtitleStream(videoPath, stream, state.subtitleRipMode, state.subtitleOCRLanguage, state.subtitleOCROutput)
			if err != nil {
				state.setSubtitleStatusAsync(err.Error())
				return
			}
			state.subtitleRipOutput = outputPath
			state.subtitleFilePath = outputPath
			app := fyne.CurrentApp()
			if app != nil && app.Driver() != nil {
				app.Driver().DoFromGoroutine(func() {
					subtitleEntry.SetText(outputPath)
					ext := strings.ToLower(filepath.Ext(outputPath))
					if ext == ".srt" || ext == ".vtt" {
						if err := state.loadSubtitleFile(outputPath); err != nil {
							state.setSubtitleStatus(err.Error())
							return
						}
						rebuildCues()
					}
					state.setSubtitleStatus(fmt.Sprintf("Extracted subtitles to %s", filepath.Base(outputPath)))
				}, false)
			} else {
				state.setSubtitleStatusAsync(fmt.Sprintf("Extracted subtitles to %s", filepath.Base(outputPath)))
			}
		}()
	})
	ripBtn.Importance = widget.HighImportance

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
		if strings.EqualFold(state.subtitleOCROutput, "ass") {
			ocrOutputSelect.SetSelected("ASS")
		} else {
			ocrOutputSelect.SetSelected("SRT")
		}
		ocrLangEntry.SetText(state.subtitleOCRLanguage)
		refreshWhisperUI()
	}

	refreshRipStreams()

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
			OCRLanguage: state.subtitleOCRLanguage,
			OCROutput:   state.subtitleOCROutput,
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
		widget.NewLabelWithStyle("Rip Embedded Subtitles", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Extract embedded subtitle tracks from DVD/BD files or media containers."),
		streamSelect,
		container.NewHBox(detectStreamsBtn, ripBtn),
		ripModeSelect,
		widget.NewLabel("OCR output (image-based subtitles only):"),
		ocrOutputSelect,
		widget.NewLabel("OCR language (tesseract, e.g. eng, fra, iku):"),
		ocrLangEntry,
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

	if ext := strings.ToLower(filepath.Ext(subPath)); ext == ".srt" || ext == ".vtt" {
		if err := s.saveSubtitleFile(subPath); err != nil {
			s.setSubtitleStatus(err.Error())
			return
		}
	}

	if mode == subtitleModeExternal {
		if ext := strings.ToLower(filepath.Ext(subPath)); ext == ".srt" || ext == ".vtt" {
			s.setSubtitleStatus(fmt.Sprintf("Saved subtitles to %s", filepath.Base(subPath)))
		} else {
			s.setSubtitleStatus(fmt.Sprintf("Using subtitle file: %s", filepath.Base(subPath)))
		}
		return
	}
	if _, err := os.Stat(subPath); err != nil {
		s.setSubtitleStatus("Subtitle file not found.")
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
			subCodec := subtitleCodecForFile(subPath, outputPath)
			outExt := strings.ToLower(filepath.Ext(outputPath))
			if subCodec == "copy" && outExt != ".mkv" {
				s.setSubtitleStatusAsync("Lossless subtitle embedding requires MKV output. Choose a .mkv output or convert to SRT.")
				return
			}
			if (outExt == ".mp4" || outExt == ".mov" || outExt == ".m4v") && subCodec != "mov_text" {
				s.setSubtitleStatusAsync("MP4/MOV output requires mov_text subtitles. Convert subtitles to SRT first.")
				return
			}
			args = []string{
				"-y",
				"-i", videoPath,
				"-i", subPath,
				"-map", "0",
				"-map", "1",
				"-c", "copy",
			}
			if subCodec != "copy" {
				args = append(args, "-c:s", subCodec)
			}
			args = append(args, outputPath)
		case subtitleModeBurn:
			if !s.isSubtitleFile(subPath) {
				s.setSubtitleStatusAsync("Burn-in requires a text subtitle file (SRT/VTT/ASS/SSA).")
				return
			}
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

func formatASSTimestamp(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	totalCs := int64(seconds*100 + 0.5)
	hours := totalCs / 360000
	minutes := (totalCs % 360000) / 6000
	secs := (totalCs % 6000) / 100
	cs := totalCs % 100
	return fmt.Sprintf("%d:%02d:%02d.%02d", hours, minutes, secs, cs)
}

func formatASS(cues []subtitleCue) string {
	var b strings.Builder
	b.WriteString("[Script Info]\n")
	b.WriteString("ScriptType: v4.00+\n")
	b.WriteString("PlayResX: 1920\n")
	b.WriteString("PlayResY: 1080\n")
	b.WriteString("WrapStyle: 0\n")
	b.WriteString("\n")
	b.WriteString("[V4+ Styles]\n")
	b.WriteString("Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	b.WriteString("Style: Default,Arial,48,&H00FFFFFF,&H000000FF,&H00000000,&H64000000,0,0,0,0,100,100,0,0,1,2,1,2,50,50,50,1\n")
	b.WriteString("\n")
	b.WriteString("[Events]\n")
	b.WriteString("Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	for _, cue := range cues {
		text := strings.TrimSpace(cue.Text)
		text = strings.ReplaceAll(text, "\n", "\\N")
		b.WriteString(fmt.Sprintf("Dialogue: 0,%s,%s,Default,,0,0,0,,%s\n", formatASSTimestamp(cue.Start), formatASSTimestamp(cue.End), text))
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
