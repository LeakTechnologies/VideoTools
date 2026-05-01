package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/app/modulecfg"
	"git.leaktechnologies.dev/stu/VideoTools/internal/app/modules/audio"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
)

// audioTrackInfo is an alias for the internal type.
type audioTrackInfo = audio.TrackInfo

type audioConfig = modulecfg.AudioConfig

func defaultAudioConfig() audioConfig {
	return modulecfg.DefaultAudioConfig()
}

func loadAudioConfig() (audioConfig, error) {
	return modulecfg.LoadAudioConfig()
}

func saveAudioConfig(cfg audioConfig) error {
	return modulecfg.SaveAudioConfig(cfg)
}

func buildAudioView(state *appState) fyne.CanvasObject {
	audioPlayer := GetAudioPlayer()
	audioPlayer.SetIdleText(i18n.T().LabelDropVideoToLoad)
	opts := audio.Options{
		Window:                     state.window,
		ModuleColor:                moduleColor("audio"),
		Player:                     audioPlayer,
		BatchMode:                  state.audioBatchMode,
		OutputFormat:               state.audioOutputFormat,
		Quality:                    state.audioQuality,
		Bitrate:                    state.audioBitrate,
		Normalize:                  state.audioNormalize,
		OutputDir:                  state.audioOutputDir,
		NormTargetLUFS:             state.audioNormTargetLUFS,
		NormTruePeak:               state.audioNormTruePeak,
		OnShowMainMenu:             func() { state.showMainMenu() },
		OnRefreshView:              func() {},
		OnUpdateBatchFilesList:     func() {},
		OnUpdateBitrateVisibility:  func() {},
		OnUpdateBitrateFromQuality: func() { state.updateAudioBitrateFromQuality() },
		OnUpdateNormVisibility:     func() { state.updateNormalizationVisibility() },
		OnPersistConfig:            func() { state.persistAudioConfig() },
		OnLoadFile:                 func(path string) { state.loadAudioFile(path) },
		OnAddBatchFile:             func(path string) { state.addAudioBatchFile(path) },
		OnClearBatchFiles:          func() { state.audioBatchFiles = nil },
		OnStartExtraction:          func(queue bool) { state.startAudioExtraction(queue) },
		OnDroppedFiles: func(paths []fyne.URI) {
			if len(paths) > 0 {
				if state.audioBatchMode {
					for _, item := range paths {
						state.addAudioBatchFile(item.Path())
					}
				} else {
					state.loadAudioFile(paths[0].Path())
				}
			}
		},
		OnGetStatsBar: func() fyne.CanvasObject { return state.statsBar },
	}
	return audio.BuildView(opts)
}

func (s *appState) showAudioView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "audio"
	s.maximizeWindow()
	s.setContent(buildAudioView(s))
}

func (s *appState) probeAudioTracks(path string) ([]audioTrackInfo, error) {
	return audio.ProbeAudioTracks(path)
}

func (s *appState) loadAudioFile(path string) {
	t := i18n.T()
	logging.Debug(logging.CatAudio, "loading audio file: %s", path)
	s.audioFileInfoLabel.SetText(t.AudioErrLoadFile + ": " + filepath.Base(path))

	src, err := probeVideo(path)
	if err != nil {
		logging.Error(logging.CatAudio, "audio probe failed: path=%s err=%v", path, err)
		dialog.ShowError(fmt.Errorf("%s: %v", t.AudioErrLoadFile, err), s.window)
		s.audioFileInfoLabel.SetText(t.AudioErrLoadFile)
		return
	}
	s.audioFile = src

	tracks, err := s.probeAudioTracks(path)
	if err != nil {
		logging.Error(logging.CatAudio, "audio track probe failed: path=%s err=%v", path, err)
		dialog.ShowError(fmt.Errorf("%s: %v", t.AudioErrDetectTracks, err), s.window)
		s.audioFileInfoLabel.SetText(t.AudioErrDetectTracks)
		return
	}

	if len(tracks) == 0 {
		dialog.ShowInformation(t.ModuleAudio, t.AudioNoTracksFound, s.window)
		s.audioFileInfoLabel.SetText(t.AudioNoTracksFound)
		return
	}

	s.audioTracks = tracks
	s.audioSelectedTracks = make(map[int]bool)
	for _, track := range tracks {
		s.audioSelectedTracks[track.Index] = true
	}
	s.updateAudioFileInfo()
	s.updateAudioTrackList()
	logging.Debug(logging.CatUI, "loaded %d audio tracks from %s", len(tracks), filepath.Base(path))
}

func (s *appState) updateAudioFileInfo() {
	t := i18n.T()
	if s.audioFile == nil {
		s.audioFileInfoLabel.SetText(t.LabelNoFile)
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
		trackCopy := track

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

		trackLabel := fmt.Sprintf("[Track %d] %s %s %s", track.Index, track.Codec, channelStr, sampleRateStr)
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
	src, err := probeVideo(path)
	if err != nil {
		logging.Error(logging.CatAudio, "audio batch probe failed: path=%s err=%v", path, err)
		dialog.ShowError(fmt.Errorf("Failed to load file: %v", err), s.window)
		return
	}
	for _, existing := range s.audioBatchFiles {
		if existing.Path == path {
			return
		}
	}
	s.audioBatchFiles = append(s.audioBatchFiles, src)
	s.updateAudioBatchFilesList()
	logging.Debug(logging.CatAudio, "added batch file: %s", path)
}

func (s *appState) removeAudioBatchFile(index int) {
	if index >= 0 && index < len(s.audioBatchFiles) {
		s.audioBatchFiles = append(s.audioBatchFiles[:index], s.audioBatchFiles[index+1:]...)
		s.updateAudioBatchFilesList()
	}
}

func (s *appState) updateAudioBatchFilesList() {
	t := i18n.T()
	if s.audioBatchListContainer == nil {
		return
	}
	s.audioBatchListContainer.Objects = nil

	if len(s.audioBatchFiles) == 0 {
		s.audioBatchListContainer.Add(widget.NewLabel(t.AudioNoFilesAdded))
	} else {
		for i, src := range s.audioBatchFiles {
			idx := i
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
	if s.audioOutputFormat == "FLAC" || s.audioOutputFormat == "WAV" {
		s.audioBitrateEntry.Disable()
	} else {
		s.audioBitrateEntry.Enable()
	}
}

func (s *appState) updateAudioBitrateFromQuality() {
	bitrateMap := map[string]map[string]string{
		"MP3":  {"Low": "128k", "Medium": "192k", "High": "256k", "Lossless": "320k"},
		"AAC":  {"Low": "128k", "Medium": "192k", "High": "256k", "Lossless": "256k"},
		"FLAC": {"Low": "", "Medium": "", "High": "", "Lossless": ""},
		"WAV":  {"Low": "", "Medium": "", "High": "", "Lossless": ""},
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
	outputDir := s.audioOutputDir
	if outputDir == "" {
		homeDir, _ := os.UserHomeDir()
		outputDir = filepath.Join(homeDir, "Music", "VideoTools", "AudioExtract")
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		dialog.ShowError(fmt.Errorf("Failed to create output directory: %v", err), s.window)
		return
	}

	inPipeline := s.pipelineStep != ""
	jobsCreated := 0

	if s.audioBatchMode {
		if len(s.audioBatchFiles) == 0 {
			dialog.ShowError(fmt.Errorf("No files added to batch"), s.window)
			return
		}
		for _, src := range s.audioBatchFiles {
			tracks, err := s.probeAudioTracks(src.Path)
			if err != nil || len(tracks) == 0 {
				continue
			}
			baseName := strings.TrimSuffix(filepath.Base(src.Path), filepath.Ext(src.Path))
			track := tracks[0]
			ext := audio.GetAudioFileExtension(s.audioOutputFormat)
			langSuffix := ""
			if track.Language != "" && track.Language != "und" {
				langSuffix = "_" + track.Language
			}
			outputPath := filepath.Join(outputDir, fmt.Sprintf("%s_track%d%s.%s", baseName, track.Index, langSuffix, ext))
			job := &queue.Job{
				Type:        queue.JobTypeAudio,
				Title:       fmt.Sprintf("Extract Audio: %s", baseName),
				Description: fmt.Sprintf("Track %d → %s", track.Index, filepath.Base(outputPath)),
				InputFile:   src.Path,
				OutputFile:  outputPath,
				Config: map[string]interface{}{
					"trackIndex": track.Index,
					"format":     s.audioOutputFormat,
					"quality":    s.audioQuality,
					"bitrate":    s.audioBitrate,
					"normalize":  s.audioNormalize,
					"targetLUFS": s.audioNormTargetLUFS,
					"truePeak":   s.audioNormTruePeak,
				},
			}
			if addToQueue {
				s.jobQueue.Add(job)
			} else {
				s.jobQueue.AddNext(job)
			}
			jobsCreated++
		}
	} else {
		if s.audioFile == nil {
			dialog.ShowError(fmt.Errorf("No file loaded"), s.window)
			return
		}
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
			ext := audio.GetAudioFileExtension(s.audioOutputFormat)
			langSuffix := ""
			if track.Language != "" && track.Language != "und" {
				langSuffix = "_" + track.Language
			}
			outputPath := filepath.Join(outputDir, fmt.Sprintf("%s_track%d%s.%s", baseName, track.Index, langSuffix, ext))
			job := &queue.Job{
				Type:        queue.JobTypeAudio,
				Title:       fmt.Sprintf("Extract Audio Track %d", track.Index),
				Description: fmt.Sprintf("%s → %s", filepath.Base(s.audioFile.Path), filepath.Base(outputPath)),
				InputFile:   s.audioFile.Path,
				OutputFile:  outputPath,
				Config: map[string]interface{}{
					"trackIndex": track.Index,
					"format":     s.audioOutputFormat,
					"quality":    s.audioQuality,
					"bitrate":    s.audioBitrate,
					"normalize":  s.audioNormalize,
					"targetLUFS": s.audioNormTargetLUFS,
					"truePeak":   s.audioNormTruePeak,
				},
			}
			if s.pipelineStep != "" {
				s.pipelineAdd(job)
			} else if addToQueue {
				s.jobQueue.Add(job)
			} else {
				s.jobQueue.AddNext(job)
			}
			jobsCreated++
		}
	}

	if !s.jobQueue.IsRunning() {
		s.jobQueue.Start()
	}
	if !inPipeline {
		s.audioStatusLabel.SetText(fmt.Sprintf("Queued %d extraction job(s)", jobsCreated))
		if !addToQueue {
			s.showQueue()
		}
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
		logging.Error(logging.CatAudio, "failed to persist audio config: err=%v", err)
	}
}

func (s *appState) executeAudioJob(ctx context.Context, job *queue.Job, progressCallback func(float64)) error {
	return audio.ExecuteFromJob(ctx, job, progressCallback)
}
