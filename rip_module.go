package main

import (
	"context"
	"errors"
	"image/color"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	ripmod "github.com/LeakTechnologies/VideoTools/internal/app/modules/rip"
	"github.com/LeakTechnologies/VideoTools/internal/dvd/ifo"
	"github.com/LeakTechnologies/VideoTools/internal/i18n"
	"github.com/LeakTechnologies/VideoTools/internal/logging"
	"github.com/LeakTechnologies/VideoTools/internal/queue"
	"github.com/LeakTechnologies/VideoTools/internal/ui"
	"github.com/LeakTechnologies/VideoTools/internal/utils"
)

func (s *appState) showRipView() {
	defer func() {
		if r := recover(); r != nil {
			logging.Crash(logging.CatModule, "panic building rip view: %v", r)
			logging.Error(logging.CatModule, "panic building rip view: %v", r)
			panic(r)
		}
	}()
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
		RipProgress:   s.ripProgress,

		QueueBtn: s.queueBtn,

		OnShowMainMenu:           s.showMainMenu,
		OnShowQueue:              s.showQueue,
		OnClearCompleted:         s.clearCompletedJobs,
		OnUpdateQueueButtonLabel: s.updateQueueButtonLabel,
		OnOpenInPlayer:           func(path string) { s.showDVDDiscView(path) },
		OnLoadDisc:               s.loadDiscFromDrive,

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

		SetQueueBtn: func(btn *ui.PillButton) {
			s.queueBtn = btn
			s.updateQueueButtonLabel()
		},
		SetRipStatusLabel: func(lbl *widget.Label) { s.ripStatusLabel = lbl },
		SetRipProgressBar: func(bar *widget.ProgressBar) { s.ripProgressBar = bar },
	}
	return ripmod.BuildView(opts)
}

// loadDiscFromDrive detects the optical drives, asks which one to use when
// several are present, and resolves the disc's VIDEO_TS folder. It returns
// ("", nil) when the user cancels, and a translated error when no usable DVD
// is available. The caller feeds the resolved path into the rip module.
func (s *appState) loadDiscFromDrive() (string, error) {
	t := i18n.T()
	drives := detectOpticalDrives()
	if len(drives) == 0 {
		return "", errors.New(t.RipErrNoDrive)
	}

	pick := drives[0]
	if len(drives) > 1 {
		got := make(chan string, 1)
		radio := widget.NewRadioGroup(drives, nil)
		radio.SetSelected(pick)
		content := container.NewVBox(
			widget.NewLabelWithStyle(t.RipSelectDriveTitle, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			radio,
		)
		d := dialog.NewCustomConfirm(
			t.RipSelectDriveTitle,
			t.ActionLoad, t.ActionCancel,
			content,
			func(ok bool) {
				if ok && radio.Selected != "" {
					got <- radio.Selected
				} else {
					got <- ""
				}
			},
			s.window,
		)
		d.Show()
		pick = <-got
		if pick == "" {
			return "", nil
		}
	}

	vtsp, err := resolveOpticalDriveVIDEOTS(pick)
	if err != nil {
		return "", err
	}
	return vtsp, nil
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
		logging.Error(logging.CatDisc, "rip job config missing: job=%s", job.ID)
		return errors.New("rip job config missing")
	}
	sourcePath := toString(cfg["sourcePath"])
	outputPath := toString(cfg["outputPath"])
	format := toString(cfg["format"])
	if sourcePath == "" || outputPath == "" {
		logging.Error(logging.CatDisc, "rip job missing paths: job=%s", job.ID)
		return errors.New("rip job missing paths")
	}

	vtsNumber := 0
	if v, ok := cfg["vtsNumber"]; ok {
		switch n := v.(type) {
		case int:
			vtsNumber = n
		case float64:
			vtsNumber = int(n)
		}
	}
	titleNumber := 0
	if v, ok := cfg["titleNumber"]; ok {
		switch n := v.(type) {
		case int:
			titleNumber = n
		case float64:
			titleNumber = int(n)
		}
	}

	execOpts := ripmod.ExecuteOptions{
		SourcePath:            sourcePath,
		OutputPath:            outputPath,
		Format:                format,
		VTSNumber:             vtsNumber,
		TitleNumber:           titleNumber,
		ExtractMode:           toString(cfg["extractMode"]),
		EmbedChapters:         toBool(cfg["embedChapters"]),
		AllAudioTracks:        toBool(cfg["allAudioTracks"]),
		IncludeSubtitles:      toBool(cfg["includeSubtitles"]),
		SelectedSubtitleLangs: toStringSlice(cfg["selectedSubtitleLangs"]),
		IncludeMenus:          toBool(cfg["includeMenus"]),
		RegionConvert:         toString(cfg["regionConvert"]),
		DiscTitle:             toString(cfg["discTitle"]),
		GetLogsDir:            getLogsDir,
		LogSuffix:             conversionLogSuffix,
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
		OnSetProgress: func(percent float64) {
			app := fyne.CurrentApp()
			if app != nil && app.Driver() != nil {
				app.Driver().DoFromGoroutine(func() {
					s.setRipProgress(percent)
				}, false)
			}
		},
		OnSetStatus: func(msg string) {
			app := fyne.CurrentApp()
			if app != nil && app.Driver() != nil {
				app.Driver().DoFromGoroutine(func() {
					s.setRipStatus(msg)
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
	s.setRipStatus("Queued rip job...")
	s.setRipProgress(0)
	s.jobQueue.Add(job)
	if runNow && !s.jobQueue.IsRunning() {
		s.jobQueue.Start()
	}
	return nil
}
