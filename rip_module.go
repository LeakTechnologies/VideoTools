package main

import (
	"context"
	"errors"
	"image/color"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	ripmod "git.leaktechnologies.dev/stu/VideoTools/internal/app/modules/rip"
	"git.leaktechnologies.dev/stu/VideoTools/internal/dvd/ifo"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

func (s *appState) showRipView() {
	s.stopPreview()
	s.lastModule = s.active
	s.active = "rip"
	s.maximizeWindow()
	s.setContent(s.buildRipView())
}

func (s *appState) buildRipView() fyne.CanvasObject {
	opts := ripmod.Options{
		Window:      s.window,
		ModuleColor: moduleColor("rip"),

		RipSourcePath: s.ripSourcePath,
		RipOutputPath: s.ripOutputPath,
		RipFormat:     s.ripFormat,
		RipLogText:    s.ripLogText,
		RipProgress:   s.ripProgress,

		QueueBtn: s.queueBtn,

		OnShowMainMenu:           s.showMainMenu,
		OnShowQueue:              s.showQueue,
		OnClearCompleted:         s.clearCompletedJobs,
		OnUpdateQueueButtonLabel: s.updateQueueButtonLabel,

		SetRipSourcePath: func(p string) { s.ripSourcePath = p },
		SetRipOutputPath: func(p string) { s.ripOutputPath = p },

		JobQueue: func() *queue.Queue { return s.jobQueue },
		AddJob:   func(job *queue.Job) { s.jobQueue.Add(job) },

		OnGetStatsBar: func() *ui.ConversionStatsBar { return s.statsBar },
		OnModuleFooter: func(col color.Color, actions fyne.CanvasObject, stats *ui.ConversionStatsBar) fyne.CanvasObject {
			return moduleFooter(col, actions, stats)
		},

		OnDropFirstLocal: func(items []fyne.URI) string {
			return firstLocalPath(items)
		},
		OnScanDVDStruct: func(path string) error {
			return s.scanDVDStructure(path)
		},
		OnProbeVideo: func(path string) (*ripmod.ProbeResult, error) {
			src, err := probeVideo(path)
			if err != nil {
				return nil, err
			}
			pr := &ripmod.ProbeResult{}
			for _, a := range src.Audio {
				pr.Audio = append(pr.Audio, ripmod.AudioStream{Index: a.Index, Language: a.Language})
			}
			for _, sub := range src.Subtitles {
				pr.Subtitles = append(pr.Subtitles, ripmod.SubtitleStream{Index: sub.Index, Language: sub.Language})
			}
			return pr, nil
		},

		SetQueueBtn: func(btn *widget.Button) {
			s.queueBtn = btn
			s.updateQueueButtonLabel()
		},
		SetRipStatusLabel: func(lbl *widget.Label) { s.ripStatusLabel = lbl },
		SetRipProgressBar: func(bar *widget.ProgressBar) { s.ripProgressBar = bar },
		SetRipLogEntry:    func(e *widget.Entry) { s.ripLogEntry = e },
		SetRipLogScroll:   func(sc *container.Scroll) { s.ripLogScroll = sc },
	}
	return ripmod.BuildView(opts)
}

func (s *appState) scanDVDStructure(path string) error {
	vmgPath := filepath.Join(path, "VIDEO_TS.IFO")
	f, err := os.Open(vmgPath)
	if err != nil {
		return errors.New("open VIDEO_TS.IFO: " + err.Error())
	}
	defer f.Close()

	vmg, err := ifo.ReadVMGI(f)
	if err != nil {
		return errors.New("read VMGI: " + err.Error())
	}

	logging.Info(logging.CatDVD, "DVD Scan: Found %d title sets", vmg.NrOfTitleSets)
	// [TODO: Update UI with title list]
	return nil
}

func (s *appState) executeRipJob(ctx context.Context, job *queue.Job, progressCallback func(float64)) error {
	cfg := job.Config
	if cfg == nil {
		logging.Error(logging.CatRip, "rip job config missing: job=%s", job.ID)
		return errors.New("rip job config missing")
	}
	sourcePath := toString(cfg["sourcePath"])
	outputPath := toString(cfg["outputPath"])
	format := toString(cfg["format"])
	if sourcePath == "" || outputPath == "" {
		logging.Error(logging.CatRip, "rip job missing paths: job=%s", job.ID)
		return errors.New("rip job missing paths")
	}

	execOpts := ripmod.ExecuteOptions{
		SourcePath: sourcePath,
		OutputPath: outputPath,
		Format:     format,
		GetLogsDir: getLogsDir,
		LogSuffix:  conversionLogSuffix,
		OnProbeVideo: func(path string) (*ripmod.ProbeResult, error) {
			src, err := probeVideo(path)
			if err != nil {
				return nil, err
			}
			pr := &ripmod.ProbeResult{}
			for _, a := range src.Audio {
				pr.Audio = append(pr.Audio, ripmod.AudioStream{Index: a.Index, Language: a.Language})
			}
			for _, sub := range src.Subtitles {
				pr.Subtitles = append(pr.Subtitles, ripmod.SubtitleStream{Index: sub.Index, Language: sub.Language})
			}
			return pr, nil
		},
		OnRunCommand: func(name string, args []string, logFn func(string)) error {
			return runCommandWithLogger(ctx, name, args, logFn)
		},
		OnAppendLog: func(line string) {
			app := fyne.CurrentApp()
			if app != nil && app.Driver() != nil {
				app.Driver().DoFromGoroutine(func() {
					s.appendRipLog(line)
				}, false)
			}
		},
		OnSetProgress: func(percent float64) {
			app := fyne.CurrentApp()
			if app != nil && app.Driver() != nil {
				app.Driver().DoFromGoroutine(func() {
					s.setRipProgress(percent)
				}, false)
			}
		},
		ProgressCallback: progressCallback,
		OnLogFileCreated: func(logPath string) { job.LogPath = logPath },
	}

	return ripmod.Execute(ctx, execOpts)
}

func defaultRipOutputPath(sourcePath, format string) string {
	return ripmod.DefaultOutputPath(sourcePath, format)
}

func firstLocalPath(items []fyne.URI) string {
	for _, uri := range items {
		if uri.Scheme() == "file" {
			return uri.Path()
		}
	}
	return ""
}

func (s *appState) resetRipLog() {
	s.ripLogText = ""
	if s.ripLogEntry != nil {
		s.ripLogEntry.SetText("")
	}
	if s.ripLogScroll != nil {
		s.ripLogScroll.ScrollToTop()
	}
}

func (s *appState) appendRipLog(line string) {
	if line == "" {
		return
	}
	s.ripLogText += line + "\n"
	if s.ripLogEntry != nil {
		s.ripLogEntry.SetText(s.ripLogText)
	}
	if s.ripLogScroll != nil {
		s.ripLogScroll.ScrollToBottom()
	}
}

func (s *appState) setRipStatus(text string) {
	if text == "" {
		text = i18n.T().StatusReady
	}
	if s.ripStatusLabel != nil {
		s.ripStatusLabel.SetText(text)
	}
}

func (s *appState) setRipProgress(percent float64) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	s.ripProgress = percent
	if s.ripProgressBar != nil {
		s.ripProgressBar.SetValue(percent / 100.0)
	}
}

func (s *appState) addRipToQueue(runNow bool) error {
	if s.jobQueue == nil {
		return errors.New("queue not initialized")
	}
	if s.ripSourcePath == "" {
		return errors.New(i18n.T().RipErrNoSource)
	}
	if s.ripOutputPath == "" {
		s.ripOutputPath = ripmod.DefaultOutputPath(s.ripSourcePath, s.ripFormat)
	}
	job := &queue.Job{
		Type:        queue.JobTypeRip,
		Title:       "Rip DVD: " + filepath.Base(s.ripSourcePath),
		Description: "Output: " + utils.ShortenMiddle(filepath.Base(s.ripOutputPath), 40),
		InputFile:   s.ripSourcePath,
		OutputFile:  s.ripOutputPath,
		Config: map[string]interface{}{
			"sourcePath": s.ripSourcePath,
			"outputPath": s.ripOutputPath,
			"format":     s.ripFormat,
		},
	}
	s.resetRipLog()
	s.setRipStatus("Queued rip job...")
	s.setRipProgress(0)
	s.jobQueue.Add(job)
	if runNow && !s.jobQueue.IsRunning() {
		s.jobQueue.Start()
	}
	return nil
}
