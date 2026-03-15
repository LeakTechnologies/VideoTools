package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
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
	"git.leaktechnologies.dev/stu/VideoTools/internal/app/modulecfg"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

// audioTrackInfo represents an audio track detected in a video
type audioTrackInfo struct {
	Index      int
	Codec      string
	Channels   int
	SampleRate int
	Bitrate    int
	Language   string
	Title      string
	Default    bool
}

type audioConfig = modulecfg.AudioConfig

// defaultAudioConfig returns default audio extraction settings
func defaultAudioConfig() audioConfig {
	return modulecfg.DefaultAudioConfig()
}

// loadAudioConfig loads the persisted audio configuration
func loadAudioConfig() (audioConfig, error) {
	return modulecfg.LoadAudioConfig()
}

// saveAudioConfig saves the audio configuration to disk
func saveAudioConfig(cfg audioConfig) error {
	return modulecfg.SaveAudioConfig(cfg)
}

func buildAudioView(state *appState) fyne.CanvasObject {
	audioColor := utils.MustHex("#FF8F00") // Dark Amber for audio

	// Top bar with back button
	backBtn := widget.NewButton("< AUDIO", func() {
		state.showMainMenu()
	})
	backBtn.Importance = widget.LowImportance

	topBar := ui.TintedBar(audioColor, container.NewHBox(backBtn, layout.NewSpacer()))

	// Left panel - File selection and track list
	leftPanel := buildAudioLeftPanel(state)

	// Right panel - Extraction settings
	rightPanel := buildAudioRightPanel(state)

	// Main content split
	mainSplit := container.New(&fixedHSplitLayout{ratio: 0.5}, leftPanel, rightPanel)

	// Action buttons
	extractBtn := widget.NewButton("Extract Now", func() {
		state.startAudioExtraction(false)
	})
	extractBtn.Importance = widget.HighImportance

	queueBtn := widget.NewButton("Add to Queue", func() {
		state.startAudioExtraction(true)
	})

	actionBar := container.NewHBox(
		layout.NewSpacer(),
		extractBtn,
		queueBtn,
	)

	bottomBar := moduleFooter(audioColor, actionBar, state.statsBar)

	return container.NewBorder(topBar, bottomBar, nil, nil, mainSplit)
}

func buildAudioLeftPanel(state *appState) fyne.CanvasObject {
	// Drop zone for video files
	dropLabel := widget.NewLabel("Drop video file here or click to browse")
	dropLabel.Alignment = fyne.TextAlignCenter

	dropZone := ui.NewDroppable(dropLabel, func(items []fyne.URI) {
		if len(items) > 0 {
			if state.audioBatchMode {
				// Add all dropped files to batch
				for _, item := range items {
					state.addAudioBatchFile(item.Path())
				}
			} else {
				state.loadAudioFile(items[0].Path())
			}
		}
	})

	// Wrap drop zone in container with minimum size
	dropContainer := container.NewPadded(dropZone)

	browseBtn := widget.NewButton("Browse for Video", func() {
		if state.audioBatchMode {
			// Browse for multiple files
			dialog.ShowFileOpen(func(uc fyne.URIReadCloser, err error) {
				if err != nil || uc == nil {
					return
				}
				defer uc.Close()
				state.addAudioBatchFile(uc.URI().Path())
			}, state.window)
		} else {
			dialog.ShowFileOpen(func(uc fyne.URIReadCloser, err error) {
				if err != nil || uc == nil {
					return
				}
				defer uc.Close()
				state.loadAudioFile(uc.URI().Path())
			}, state.window)
		}
	})

	// File info display
	fileInfoLabel := widget.NewLabel("No file loaded")
	fileInfoLabel.Wrapping = fyne.TextWrapWord
	state.audioFileInfoLabel = fileInfoLabel

	// Track list
	trackListLabel := widget.NewLabel("Audio Tracks:")
	trackListLabel.TextStyle = fyne.TextStyle{Bold: true}

	trackListContainer := container.NewVBox()
	state.audioTrackListContainer = trackListContainer

	// Select all/deselect all buttons
	selectAllBtn := widget.NewButton("Select All", func() {
		state.selectAllAudioTracks(true)
	})
	selectAllBtn.Importance = widget.LowImportance

	deselectAllBtn := widget.NewButton("Deselect All", func() {
		state.selectAllAudioTracks(false)
	})
	deselectAllBtn.Importance = widget.LowImportance

	trackControls := container.NewHBox(selectAllBtn, deselectAllBtn)

	// Batch mode toggle
	batchCheck := widget.NewCheck("Batch Mode (multiple videos)", func(checked bool) {
		state.audioBatchMode = checked
		state.refreshAudioView()
	})

	// Batch files list
	batchFilesLabel := widget.NewLabel("Batch Files:")
	batchFilesLabel.TextStyle = fyne.TextStyle{Bold: true}

	batchListContainer := container.NewVBox()
	state.audioBatchListContainer = batchListContainer

	clearBatchBtn := widget.NewButton("Clear All", func() {
		state.audioBatchFiles = nil
		state.updateAudioBatchFilesList()
	})
	clearBatchBtn.Importance = widget.DangerImportance

	batchContent := container.NewVBox(
		dropContainer,
		browseBtn,
		widget.NewSeparator(),
		batchFilesLabel,
		container.NewVScroll(batchListContainer),
		clearBatchBtn,
		widget.NewSeparator(),
		batchCheck,
	)

	singleContent := container.NewVBox(
		dropContainer,
		browseBtn,
		widget.NewSeparator(),
		fileInfoLabel,
		widget.NewSeparator(),
		trackListLabel,
		trackControls,
		container.NewVScroll(trackListContainer),
		widget.NewSeparator(),
		batchCheck,
	)

	// Choose which content to show based on batch mode
	leftContent := container.NewMax()
	if state.audioBatchMode {
		leftContent.Objects = []fyne.CanvasObject{batchContent}
	} else {
		leftContent.Objects = []fyne.CanvasObject{singleContent}
	}
	state.audioLeftPanel = leftContent
	state.audioSingleContent = singleContent
	state.audioBatchContent = batchContent

	return leftContent
}

func buildAudioRightPanel(state *appState) fyne.CanvasObject {
	// Output format selection
	formatLabel := widget.NewLabel("Output Format:")
	formatLabel.TextStyle = fyne.TextStyle{Bold: true}

	formatRadio := widget.NewRadioGroup([]string{"MP3", "AAC", "FLAC", "WAV"}, func(value string) {
		state.audioOutputFormat = value
		state.updateAudioBitrateVisibility()
		state.persistAudioConfig()
	})
	formatRadio.Horizontal = true

	// Quality preset
	qualityLabel := widget.NewLabel("Quality Preset:")
	qualityLabel.TextStyle = fyne.TextStyle{Bold: true}

	qualitySelect := widget.NewSelect([]string{"Low", "Medium", "High", "Lossless"}, func(value string) {
		state.audioQuality = value
		state.updateAudioBitrateFromQuality()
		state.persistAudioConfig()
	})

	// Bitrate entry
	bitrateLabel := widget.NewLabel("Bitrate:")
	bitrateEntry := widget.NewEntry()
	bitrateEntry.SetText(state.audioBitrate)
	bitrateEntry.OnChanged = func(value string) {
		state.audioBitrate = value
		state.persistAudioConfig()
	}
	state.audioBitrateEntry = bitrateEntry

	// Set initial quality after bitrate entry is initialized
	qualitySelect.SetSelected(state.audioQuality)

	// Set initial format after bitrate entry is initialized
	formatRadio.SetSelected(state.audioOutputFormat)

	// Normalization section
	normalizeCheck := widget.NewCheck("Apply EBU R128 Normalization", func(checked bool) {
		state.audioNormalize = checked
		state.updateNormalizationVisibility()
		state.persistAudioConfig()
	})
	normalizeCheck.SetChecked(state.audioNormalize)

	// Normalization options
	lufsLabel := widget.NewLabel(fmt.Sprintf("Target LUFS: %.1f", state.audioNormTargetLUFS))
	lufsSlider := widget.NewSlider(-30, -10)
	lufsSlider.SetValue(state.audioNormTargetLUFS)
	lufsSlider.Step = 0.5
	lufsSlider.OnChanged = func(value float64) {
		state.audioNormTargetLUFS = value
		lufsLabel.SetText(fmt.Sprintf("Target LUFS: %.1f", value))
		state.persistAudioConfig()
	}

	peakLabel := widget.NewLabel(fmt.Sprintf("True Peak: %.1f dB", state.audioNormTruePeak))
	peakSlider := widget.NewSlider(-3, 0)
	peakSlider.SetValue(state.audioNormTruePeak)
	peakSlider.Step = 0.1
	peakSlider.OnChanged = func(value float64) {
		state.audioNormTruePeak = value
		peakLabel.SetText(fmt.Sprintf("True Peak: %.1f dB", value))
		state.persistAudioConfig()
	}

	normOptions := container.NewVBox(
		lufsLabel,
		lufsSlider,
		peakLabel,
		peakSlider,
	)
	state.audioNormOptionsContainer = normOptions

	// Output directory
	outputDirLabel := widget.NewLabel("Output Directory:")
	outputDirLabel.TextStyle = fyne.TextStyle{Bold: true}

	outputDirEntry := widget.NewEntry()
	if state.audioOutputDir == "" {
		home, _ := os.UserHomeDir()
		state.audioOutputDir = filepath.Join(home, "Music", "VideoTools", "AudioExtract")
	}
	outputDirEntry.SetText(state.audioOutputDir)
	outputDirEntry.OnChanged = func(value string) {
		state.audioOutputDir = value
		state.persistAudioConfig()
	}

	outputDirBrowseBtn := widget.NewButton("Browse", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			state.audioOutputDir = uri.Path()
			outputDirEntry.SetText(uri.Path())
			state.persistAudioConfig()
		}, state.window)
	})

	outputDirRow := container.NewBorder(nil, nil, nil, outputDirBrowseBtn, outputDirEntry)

	// Status and progress
	statusLabel := widget.NewLabel("Ready")
	state.audioStatusLabel = statusLabel

	progressBar := widget.NewProgressBar()
	progressBar.Hide()
	state.audioProgressBar = progressBar

	// Helper to build boxed sections matching Convert module style
	gridColor := utils.MustHex("#2A3A52")
	navyBlue := utils.MustHex("#191F35")

	buildAudioBox := func(title string, content fyne.CanvasObject) fyne.CanvasObject {
		bg := canvas.NewRectangle(navyBlue)
		bg.CornerRadius = 10
		bg.StrokeColor = gridColor
		bg.StrokeWidth = 1
		body := container.NewVBox(
			widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewSeparator(),
			content,
		)
		layers := ui.NoisyBackgroundObjects(bg)
		layers = append(layers, container.NewPadded(body))
		return container.NewMax(layers...)
	}

	rightContent := container.NewVBox(
		buildAudioBox("Format", container.NewVBox(formatLabel, formatRadio)),
		buildAudioBox("Quality", container.NewVBox(qualityLabel, qualitySelect)),
		buildAudioBox("Bitrate", container.NewVBox(bitrateLabel, bitrateEntry)),
		buildAudioBox("Normalization", container.NewVBox(normalizeCheck, normOptions)),
		buildAudioBox("Output", container.NewVBox(outputDirLabel, outputDirRow, statusLabel, progressBar)),
	)

	scrollable := ui.NewFastVScroll(rightContent)
	return scrollable
}

// Helper functions for audio module state

// probeAudioTracks detects all audio tracks in a video file
func (s *appState) probeAudioTracks(path string) ([]audioTrackInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := utils.CreateCommand(ctx, utils.GetFFprobePath(),
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-select_streams", "a",
		path,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var result struct {
		Streams []struct {
			Index       int                    `json:"index"`
			CodecName   string                 `json:"codec_name"`
			Channels    int                    `json:"channels"`
			SampleRate  string                 `json:"sample_rate"`
			BitRate     string                 `json:"bit_rate"`
			Tags        map[string]interface{} `json:"tags"`
			Disposition struct {
				Default int `json:"default"`
			} `json:"disposition"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	var tracks []audioTrackInfo
	for _, stream := range result.Streams {
		track := audioTrackInfo{
			Index:    stream.Index,
			Codec:    stream.CodecName,
			Channels: stream.Channels,
			Default:  stream.Disposition.Default == 1,
		}

		// Parse sample rate
		if sampleRate, err := strconv.Atoi(stream.SampleRate); err == nil {
			track.SampleRate = sampleRate
		}

		// Parse bitrate
		if bitrate, err := strconv.Atoi(stream.BitRate); err == nil {
			track.Bitrate = bitrate
		}

		// Extract language from tags
		if lang, ok := stream.Tags["language"].(string); ok {
			track.Language = lang
		}

		// Extract title from tags
		if title, ok := stream.Tags["title"].(string); ok {
			track.Title = title
		}

		tracks = append(tracks, track)
	}

	return tracks, nil
}

func (s *appState) loadAudioFile(path string) {
	logging.Debug(logging.CatUI, "loading audio file: %s", path)
	s.audioFileInfoLabel.SetText("Loading: " + filepath.Base(path))

	// Probe the file for metadata
	src, err := probeVideo(path)
	if err != nil {
		logging.Debug(logging.CatUI, "failed to probe video: %v", err)
		dialog.ShowError(fmt.Errorf("Failed to load file: %v", err), s.window)
		s.audioFileInfoLabel.SetText("Failed to load file")
		return
	}

	s.audioFile = src

	// Detect audio tracks
	tracks, err := s.probeAudioTracks(path)
	if err != nil {
		logging.Debug(logging.CatUI, "failed to probe audio tracks: %v", err)
		dialog.ShowError(fmt.Errorf("Failed to detect audio tracks: %v", err), s.window)
		s.audioFileInfoLabel.SetText("Failed to detect audio tracks")
		return
	}

	if len(tracks) == 0 {
		dialog.ShowInformation("No Audio", "This file does not contain any audio tracks.", s.window)
		s.audioFileInfoLabel.SetText("No audio tracks found")
		return
	}

	s.audioTracks = tracks
	s.audioSelectedTracks = make(map[int]bool)

	// Auto-select all tracks by default
	for _, track := range tracks {
		s.audioSelectedTracks[track.Index] = true
	}

	// Update UI
	s.updateAudioFileInfo()
	s.updateAudioTrackList()
	logging.Debug(logging.CatUI, "loaded %d audio tracks from %s", len(tracks), filepath.Base(path))
}

func (s *appState) updateAudioFileInfo() {
	if s.audioFile == nil {
		s.audioFileInfoLabel.SetText("No file loaded")
		return
	}

	info := fmt.Sprintf("File: %s\nDuration: %s\nFormat: %s",
		s.audioFile.DisplayName,
		formatShortDuration(s.audioFile.Duration),
		s.audioFile.Format,
	)
	s.audioFileInfoLabel.SetText(info)
}

func (s *appState) updateAudioTrackList() {
	s.audioTrackListContainer.Objects = nil

	for _, track := range s.audioTracks {
		trackCopy := track // Capture for closure

		// Format track info
		channelStr := fmt.Sprintf("%dch", track.Channels)
		if track.Channels == 1 {
			channelStr = "Mono"
		} else if track.Channels == 2 {
			channelStr = "Stereo"
		} else if track.Channels == 6 {
			channelStr = "5.1"
		}

		sampleRateStr := fmt.Sprintf("%d Hz", track.SampleRate)
		bitrateStr := ""
		if track.Bitrate > 0 {
			bitrateStr = fmt.Sprintf("%d kbps", track.Bitrate/1000)
		}

		trackLabel := fmt.Sprintf("[Track %d] %s %s %s",
			track.Index,
			track.Codec,
			channelStr,
			sampleRateStr,
		)

		if bitrateStr != "" {
			trackLabel += " " + bitrateStr
		}

		if track.Language != "" {
			trackLabel += fmt.Sprintf(" (%s)", track.Language)
		}

		if track.Title != "" {
			trackLabel += fmt.Sprintf(" - %s", track.Title)
		}

		check := widget.NewCheck(trackLabel, func(checked bool) {
			s.audioSelectedTracks[trackCopy.Index] = checked
		})
		check.SetChecked(s.audioSelectedTracks[trackCopy.Index])

		s.audioTrackListContainer.Add(check)
	}

	s.audioTrackListContainer.Refresh()
}

func (s *appState) selectAllAudioTracks(selectAll bool) {
	for _, track := range s.audioTracks {
		s.audioSelectedTracks[track.Index] = selectAll
	}
	s.updateAudioTrackList()
}

func (s *appState) refreshAudioView() {
	// Switch between single and batch UI
	if s.audioLeftPanel != nil {
		if s.audioBatchMode {
			s.audioLeftPanel.Objects = []fyne.CanvasObject{s.audioBatchContent}
			s.updateAudioBatchFilesList()
		} else {
			s.audioLeftPanel.Objects = []fyne.CanvasObject{s.audioSingleContent}
			s.updateAudioFileInfo()
			s.updateAudioTrackList()
		}
		s.audioLeftPanel.Refresh()
	}
}

func (s *appState) addAudioBatchFile(path string) {
	// Probe the file
	src, err := probeVideo(path)
	if err != nil {
		logging.Debug(logging.CatUI, "failed to probe video for batch: %v", err)
		dialog.ShowError(fmt.Errorf("Failed to load file: %v", err), s.window)
		return
	}

	// Check for duplicate
	for _, existing := range s.audioBatchFiles {
		if existing.Path == path {
			return // Already added
		}
	}

	s.audioBatchFiles = append(s.audioBatchFiles, src)
	s.updateAudioBatchFilesList()
	logging.Debug(logging.CatUI, "added batch file: %s", path)
}

func (s *appState) removeAudioBatchFile(index int) {
	if index >= 0 && index < len(s.audioBatchFiles) {
		s.audioBatchFiles = append(s.audioBatchFiles[:index], s.audioBatchFiles[index+1:]...)
		s.updateAudioBatchFilesList()
	}
}

func (s *appState) updateAudioBatchFilesList() {
	if s.audioBatchListContainer == nil {
		return
	}

	s.audioBatchListContainer.Objects = nil

	if len(s.audioBatchFiles) == 0 {
		s.audioBatchListContainer.Add(widget.NewLabel("No files added"))
	} else {
		for i, src := range s.audioBatchFiles {
			idx := i // Capture for closure
			fileLabel := widget.NewLabel(fmt.Sprintf("%d. %s", i+1, src.DisplayName))
			removeBtn := widget.NewButton("Remove", func() {
				s.removeAudioBatchFile(idx)
			})
			removeBtn.Importance = widget.LowImportance

			row := container.NewBorder(nil, nil, nil, removeBtn, fileLabel)
			s.audioBatchListContainer.Add(row)
		}
		s.audioBatchListContainer.Add(widget.NewLabel(fmt.Sprintf("Total: %d files", len(s.audioBatchFiles))))
	}

	s.audioBatchListContainer.Refresh()
}

func (s *appState) updateAudioBitrateVisibility() {
	// Hide bitrate entry for lossless formats
	if s.audioOutputFormat == "FLAC" || s.audioOutputFormat == "WAV" {
		s.audioBitrateEntry.Disable()
	} else {
		s.audioBitrateEntry.Enable()
	}
}

func (s *appState) updateAudioBitrateFromQuality() {
	// Update bitrate based on quality preset
	bitrateMap := map[string]map[string]string{
		"MP3": {
			"Low":      "128k",
			"Medium":   "192k",
			"High":     "256k",
			"Lossless": "320k",
		},
		"AAC": {
			"Low":      "128k",
			"Medium":   "192k",
			"High":     "256k",
			"Lossless": "256k",
		},
		"FLAC": {
			"Low":      "",
			"Medium":   "",
			"High":     "",
			"Lossless": "",
		},
		"WAV": {
			"Low":      "",
			"Medium":   "",
			"High":     "",
			"Lossless": "",
		},
	}

	if bitrate, ok := bitrateMap[s.audioOutputFormat][s.audioQuality]; ok {
		s.audioBitrate = bitrate
		if s.audioBitrateEntry != nil {
			s.audioBitrateEntry.SetText(bitrate)
		}
	}
}

func (s *appState) updateNormalizationVisibility() {
	if s.audioNormalize {
		s.audioNormOptionsContainer.Show()
	} else {
		s.audioNormOptionsContainer.Hide()
	}
}

func (s *appState) startAudioExtraction(addToQueue bool) {
	// Get output directory
	outputDir := s.audioOutputDir
	if outputDir == "" {
		homeDir, _ := os.UserHomeDir()
		outputDir = filepath.Join(homeDir, "Music", "VideoTools", "AudioExtract")
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		dialog.ShowError(fmt.Errorf("Failed to create output directory: %v", err), s.window)
		return
	}

	jobsCreated := 0

	if s.audioBatchMode {
		// Batch mode: process all batch files
		if len(s.audioBatchFiles) == 0 {
			dialog.ShowError(fmt.Errorf("No files added to batch"), s.window)
			return
		}

		// For each file in batch, extract first audio track (or all tracks if we want to expand this later)
		for _, src := range s.audioBatchFiles {
			// Detect audio tracks for this file
			tracks, err := s.probeAudioTracks(src.Path)
			if err != nil {
				logging.Debug(logging.CatUI, "failed to probe audio for %s: %v", src.Path, err)
				continue
			}

			if len(tracks) == 0 {
				logging.Debug(logging.CatUI, "no audio tracks in %s", src.Path)
				continue
			}

			// Extract first audio track (or all - configurable later)
			baseName := strings.TrimSuffix(filepath.Base(src.Path), filepath.Ext(src.Path))
			track := tracks[0] // Extract first track

			ext := s.getAudioFileExtension()
			langSuffix := ""
			if track.Language != "" && track.Language != "und" {
				langSuffix = "_" + track.Language
			}
			outputPath := filepath.Join(outputDir, fmt.Sprintf("%s_track%d%s.%s", baseName, track.Index, langSuffix, ext))

			config := map[string]interface{}{
				"trackIndex": track.Index,
				"format":     s.audioOutputFormat,
				"quality":    s.audioQuality,
				"bitrate":    s.audioBitrate,
				"normalize":  s.audioNormalize,
				"targetLUFS": s.audioNormTargetLUFS,
				"truePeak":   s.audioNormTruePeak,
			}

			job := &queue.Job{
				Type:        queue.JobTypeAudio,
				Title:       fmt.Sprintf("Extract Audio: %s", baseName),
				Description: fmt.Sprintf("Track %d → %s", track.Index, filepath.Base(outputPath)),
				InputFile:   src.Path,
				OutputFile:  outputPath,
				Config:      config,
			}

			if addToQueue {
				s.jobQueue.Add(job)
			} else {
				s.jobQueue.AddNext(job)
			}
			jobsCreated++
		}
	} else {
		// Single file mode
		if s.audioFile == nil {
			dialog.ShowError(fmt.Errorf("No file loaded"), s.window)
			return
		}

		// Count selected tracks
		selectedCount := 0
		for _, selected := range s.audioSelectedTracks {
			if selected {
				selectedCount++
			}
		}

		if selectedCount == 0 {
			dialog.ShowError(fmt.Errorf("No audio tracks selected"), s.window)
			return
		}

		baseName := strings.TrimSuffix(filepath.Base(s.audioFile.Path), filepath.Ext(s.audioFile.Path))

		for _, track := range s.audioTracks {
			if !s.audioSelectedTracks[track.Index] {
				continue
			}

			// Build output filename
			ext := s.getAudioFileExtension()
			langSuffix := ""
			if track.Language != "" && track.Language != "und" {
				langSuffix = "_" + track.Language
			}
			outputPath := filepath.Join(outputDir, fmt.Sprintf("%s_track%d%s.%s", baseName, track.Index, langSuffix, ext))

			// Prepare job config
			config := map[string]interface{}{
				"trackIndex": track.Index,
				"format":     s.audioOutputFormat,
				"quality":    s.audioQuality,
				"bitrate":    s.audioBitrate,
				"normalize":  s.audioNormalize,
				"targetLUFS": s.audioNormTargetLUFS,
				"truePeak":   s.audioNormTruePeak,
			}

			// Create job
			job := &queue.Job{
				Type:        queue.JobTypeAudio,
				Title:       fmt.Sprintf("Extract Audio Track %d", track.Index),
				Description: fmt.Sprintf("%s → %s", filepath.Base(s.audioFile.Path), filepath.Base(outputPath)),
				InputFile:   s.audioFile.Path,
				OutputFile:  outputPath,
				Config:      config,
			}

			if addToQueue {
				s.jobQueue.Add(job)
			} else {
				s.jobQueue.AddNext(job)
			}
			jobsCreated++
		}
	}

	// Start queue if not already running
	if !s.jobQueue.IsRunning() {
		s.jobQueue.Start()
	}

	// Update status
	s.audioStatusLabel.SetText(fmt.Sprintf("Queued %d extraction job(s)", jobsCreated))

	// Navigate to queue view if starting immediately
	if !addToQueue {
		s.showQueue()
	}
}

func (s *appState) getAudioFileExtension() string {
	switch s.audioOutputFormat {
	case "MP3":
		return "mp3"
	case "AAC":
		return "m4a"
	case "FLAC":
		return "flac"
	case "WAV":
		return "wav"
	default:
		return "mp3"
	}
}

func (s *appState) persistAudioConfig() {
	cfg := audioConfig{
		OutputFormat:   s.audioOutputFormat,
		Quality:        s.audioQuality,
		Bitrate:        s.audioBitrate,
		Normalize:      s.audioNormalize,
		NormTargetLUFS: s.audioNormTargetLUFS,
		NormTruePeak:   s.audioNormTruePeak,
		OutputDir:      s.audioOutputDir,
	}
	if err := saveAudioConfig(cfg); err != nil {
		logging.Debug(logging.CatSystem, "failed to persist audio config: %v", err)
	}
}

func (s *appState) executeAudioJob(ctx context.Context, job *queue.Job, progressCallback func(float64)) error {
	cfg := job.Config
	if cfg == nil {
		return fmt.Errorf("audio job config missing")
	}

	// Extract config
	trackIndex := int(cfg["trackIndex"].(float64))
	format := cfg["format"].(string)
	bitrate := cfg["bitrate"].(string)
	normalize := cfg["normalize"].(bool)

	inputPath := job.InputFile
	outputPath := job.OutputFile

	logging.Debug(logging.CatFFMPEG, "Audio extraction: track %d from %s to %s (format: %s, bitrate: %s, normalize: %v)",
		trackIndex, inputPath, outputPath, format, bitrate, normalize)

	// If normalization is requested, do two-pass loudnorm
	if normalize {
		targetLUFS := cfg["targetLUFS"].(float64)
		truePeak := cfg["truePeak"].(float64)

		logging.Debug(logging.CatFFMPEG, "Running two-pass loudnorm normalization (target LUFS: %.1f, true peak: %.1f)", targetLUFS, truePeak)

		// Pass 1: Analyze audio
		progressCallback(10.0)
		normParams, err := s.analyzeLoudnorm(ctx, inputPath, trackIndex, targetLUFS, truePeak)
		if err != nil {
			return fmt.Errorf("loudnorm analysis failed: %w", err)
		}

		progressCallback(30.0)

		// Pass 2: Apply normalization with measured values
		if err := s.extractAudioWithNormalization(ctx, inputPath, outputPath, trackIndex, format, bitrate, targetLUFS, truePeak, normParams, progressCallback); err != nil {
			return err
		}
	} else {
		// Simple extraction without normalization
		if err := s.extractAudioSimple(ctx, inputPath, outputPath, trackIndex, format, bitrate, progressCallback); err != nil {
			return err
		}
	}

	progressCallback(100.0)
	logging.Debug(logging.CatFFMPEG, "Audio extraction completed: %s", outputPath)
	return nil
}

// loudnormParams holds measured values from loudnorm analysis
type loudnormParams struct {
	MeasuredI      float64
	MeasuredTP     float64
	MeasuredLRA    float64
	MeasuredThresh float64
}

// analyzeLoudnorm runs the first pass to analyze audio levels
func (s *appState) analyzeLoudnorm(ctx context.Context, inputPath string, trackIndex int, targetI, targetTP float64) (*loudnormParams, error) {
	args := []string{
		"-i", inputPath,
		"-map", fmt.Sprintf("0:a:%d", trackIndex),
		"-af", fmt.Sprintf("loudnorm=I=%.1f:TP=%.1f:print_format=json", targetI, targetTP),
		"-f", "null",
		"-",
	}

	cmd := utils.CreateCommand(ctx, utils.GetFFmpegPath(), args...)
	logging.Debug(logging.CatFFMPEG, "Loudnorm analysis: %s %v", utils.GetFFmpegPath(), args)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.Debug(logging.CatFFMPEG, "Loudnorm analysis output: %s", string(output))
		return nil, fmt.Errorf("loudnorm analysis failed: %w", err)
	}

	// Parse JSON output from loudnorm
	// The output contains a JSON block at the end
	outputStr := string(output)
	jsonStart := strings.Index(outputStr, "{")
	if jsonStart == -1 {
		return nil, fmt.Errorf("no JSON output from loudnorm")
	}

	jsonData := outputStr[jsonStart:]
	jsonEnd := strings.LastIndex(jsonData, "}")
	if jsonEnd == -1 {
		return nil, fmt.Errorf("malformed JSON output from loudnorm")
	}
	jsonData = jsonData[:jsonEnd+1]

	var result struct {
		InputI      string `json:"input_i"`
		InputTP     string `json:"input_tp"`
		InputLRA    string `json:"input_lra"`
		InputThresh string `json:"input_thresh"`
	}

	if err := json.Unmarshal([]byte(jsonData), &result); err != nil {
		logging.Debug(logging.CatFFMPEG, "Failed to parse JSON: %s", jsonData)
		return nil, fmt.Errorf("failed to parse loudnorm JSON: %w", err)
	}

	params := &loudnormParams{}
	params.MeasuredI, _ = strconv.ParseFloat(result.InputI, 64)
	params.MeasuredTP, _ = strconv.ParseFloat(result.InputTP, 64)
	params.MeasuredLRA, _ = strconv.ParseFloat(result.InputLRA, 64)
	params.MeasuredThresh, _ = strconv.ParseFloat(result.InputThresh, 64)

	logging.Debug(logging.CatFFMPEG, "Loudnorm measured: I=%.2f, TP=%.2f, LRA=%.2f, thresh=%.2f",
		params.MeasuredI, params.MeasuredTP, params.MeasuredLRA, params.MeasuredThresh)

	return params, nil
}

// extractAudioWithNormalization performs the second pass with normalization
func (s *appState) extractAudioWithNormalization(ctx context.Context, inputPath, outputPath string, trackIndex int, format, bitrate string, targetI, targetTP float64, params *loudnormParams, progressCallback func(float64)) error {
	args := []string{
		"-y",
		"-i", inputPath,
		"-map", fmt.Sprintf("0:a:%d", trackIndex),
		"-af", fmt.Sprintf("loudnorm=I=%.1f:TP=%.1f:measured_I=%.2f:measured_TP=%.2f:measured_LRA=%.2f:measured_thresh=%.2f",
			targetI, targetTP, params.MeasuredI, params.MeasuredTP, params.MeasuredLRA, params.MeasuredThresh),
	}

	// Add codec settings
	args = append(args, s.getAudioCodecArgs(format, bitrate)...)
	args = append(args, outputPath)

	return s.runFFmpegExtraction(ctx, args, progressCallback, 30.0, 100.0)
}

// extractAudioSimple performs simple extraction without normalization
func (s *appState) extractAudioSimple(ctx context.Context, inputPath, outputPath string, trackIndex int, format, bitrate string, progressCallback func(float64)) error {
	args := []string{
		"-y",
		"-i", inputPath,
		"-map", fmt.Sprintf("0:a:%d", trackIndex),
	}

	// Add codec settings
	args = append(args, s.getAudioCodecArgs(format, bitrate)...)
	args = append(args, outputPath)

	return s.runFFmpegExtraction(ctx, args, progressCallback, 0.0, 100.0)
}

// getAudioCodecArgs returns codec-specific arguments
func (s *appState) getAudioCodecArgs(format, bitrate string) []string {
	switch format {
	case "MP3":
		args := []string{"-c:a", "libmp3lame"}
		if bitrate != "" {
			args = append(args, "-b:a", bitrate)
		}
		return args
	case "AAC":
		args := []string{"-c:a", "aac"}
		if bitrate != "" {
			args = append(args, "-b:a", bitrate)
		}
		return args
	case "FLAC":
		return []string{"-c:a", "flac"}
	case "WAV":
		return []string{"-c:a", "pcm_s16le"}
	default:
		return []string{"-c:a", "copy"}
	}
}

// runFFmpegExtraction executes FFmpeg and reports progress
func (s *appState) runFFmpegExtraction(ctx context.Context, args []string, progressCallback func(float64), startPct, endPct float64) error {
	cmd := utils.CreateCommand(ctx, utils.GetFFmpegPath(), args...)
	logging.Debug(logging.CatFFMPEG, "Running: %s %v", utils.GetFFmpegPath(), args)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	// Parse FFmpeg output for progress
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		logging.Debug(logging.CatFFMPEG, "FFmpeg: %s", line)

		// Report progress
		if strings.Contains(line, "time=") {
			// Report midpoint between start and end
			progressCallback(startPct + (endPct-startPct)/2)
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("FFmpeg failed: %w", err)
	}

	progressCallback(endPct)
	return nil
}

func (s *appState) showAudioView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "audio"
	s.maximizeWindow()
	s.setContent(buildAudioView(s))
}
