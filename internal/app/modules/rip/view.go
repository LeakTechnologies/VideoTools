package rip

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"git.leaktechnologies.dev/stu/VideoTools/internal/app/configpath"
	"git.leaktechnologies.dev/stu/VideoTools/internal/app/modulecfg"
	"git.leaktechnologies.dev/stu/VideoTools/internal/dvd/udf"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
	"git.leaktechnologies.dev/stu/VideoTools/internal/queue"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

type ripConfig = modulecfg.RipConfig

func defaultRipConfig() ripConfig {
	return modulecfg.DefaultRipConfig()
}

func loadPersistedRipConfig() (ripConfig, error) {
	return modulecfg.LoadRipConfig()
}

func savePersistedRipConfig(cfg ripConfig) error {
	return modulecfg.SaveRipConfig(cfg)
}

// viewState holds local UI state while the rip view is active.
type viewState struct {
	sourcePath string
	outputPath string
	format     string
	logText    string
	progress   float64

	statusLabel *widget.Label
	progressBar *widget.ProgressBar
	logEntry    *widget.Entry
	logScroll   *container.Scroll
}

func (vs *viewState) applyConfig(cfg ripConfig) {
	vs.format = cfg.Format
}

func (vs *viewState) persistConfig() {
	cfg := ripConfig{Format: vs.format}
	if err := savePersistedRipConfig(cfg); err != nil {
		logging.Debug(logging.CatSystem, "failed to persist rip config: %v", err)
	}
}

func (vs *viewState) resetLog() {
	vs.logText = ""
	if vs.logEntry != nil {
		vs.logEntry.SetText("")
	}
	if vs.logScroll != nil {
		vs.logScroll.ScrollToTop()
	}
}

func (vs *viewState) appendLog(line string) {
	if strings.TrimSpace(line) == "" {
		return
	}
	vs.logText += line + "\n"
	if vs.logEntry != nil {
		vs.logEntry.SetText(vs.logText)
	}
	if vs.logScroll != nil {
		vs.logScroll.ScrollToBottom()
	}
}

func (vs *viewState) setStatus(text string) {
	if text == "" {
		text = i18n.T().StatusReady
	}
	if vs.statusLabel != nil {
		vs.statusLabel.SetText(text)
	}
}

func (vs *viewState) setProgress(percent float64) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	vs.progress = percent
	if vs.progressBar != nil {
		vs.progressBar.SetValue(percent / 100.0)
	}
}

// BuildView constructs the full rip module UI and returns the canvas object.
// It also calls back the Set* functions on opts so the root can track widget refs.
func BuildView(opts Options) fyne.CanvasObject {
	t := i18n.T()

	vs := &viewState{
		sourcePath: opts.RipSourcePath,
		outputPath: opts.RipOutputPath,
		format:     opts.RipFormat,
		logText:    opts.RipLogText,
		progress:   opts.RipProgress,
	}

	// Load persisted config.
	if cfg, err := loadPersistedRipConfig(); err == nil {
		vs.applyConfig(cfg)
	} else if !errors.Is(err, os.ErrNotExist) {
		logging.Debug(logging.CatSystem, "failed to load persisted rip config: %v", err)
	}
	if vs.format == "" {
		vs.format = FormatLosslessMKV
	}

	backBtn := widget.NewButton("< "+strings.ToUpper(t.ModuleRip), opts.OnShowMainMenu)
	backBtn.Importance = widget.LowImportance

	queueBtn := opts.QueueBtn
	if queueBtn == nil {
		queueBtn = widget.NewButton(t.ActionViewQueue, opts.OnShowQueue)
	}
	if opts.SetQueueBtn != nil {
		opts.SetQueueBtn(queueBtn)
	}
	if opts.OnUpdateQueueButtonLabel != nil {
		opts.OnUpdateQueueButtonLabel()
	}

	clearCompletedBtn := widget.NewButton("⌫", func() {
		if opts.OnClearCompleted != nil {
			opts.OnClearCompleted()
		}
	})
	clearCompletedBtn.Importance = widget.LowImportance

	topBar := ui.TintedBar(opts.ModuleColor, container.NewHBox(backBtn, layout.NewSpacer(), clearCompletedBtn, queueBtn))

	sourceEntry := widget.NewEntry()
	sourceEntry.SetPlaceHolder(t.RipDropPrompt)
	sourceEntry.SetText(vs.sourcePath)
	sourceEntry.OnChanged = func(val string) {
		vs.sourcePath = strings.TrimSpace(val)
		if opts.SetRipSourcePath != nil {
			opts.SetRipSourcePath(vs.sourcePath)
		}
		vs.outputPath = DefaultOutputPath(vs.sourcePath, vs.format)
		if opts.SetRipOutputPath != nil {
			opts.SetRipOutputPath(vs.outputPath)
		}
	}

	outputEntry := widget.NewEntry()
	outputEntry.SetPlaceHolder(t.RipOutputPath)
	outputEntry.SetText(vs.outputPath)
	outputEntry.OnChanged = func(val string) {
		vs.outputPath = strings.TrimSpace(val)
		if opts.SetRipOutputPath != nil {
			opts.SetRipOutputPath(vs.outputPath)
		}
	}

	formatSelect := widget.NewSelect([]string{FormatLosslessMKV, FormatH264MKV, FormatH264MP4, FormatArchivist}, func(value string) {
		vs.format = value
		vs.outputPath = DefaultOutputPath(vs.sourcePath, value)
		outputEntry.SetText(vs.outputPath)
		if opts.SetRipOutputPath != nil {
			opts.SetRipOutputPath(vs.outputPath)
		}
		vs.persistConfig()
	})
	formatSelect.SetSelected(vs.format)

	statusLabel := widget.NewLabel(t.StatusReady)
	statusLabel.Wrapping = fyne.TextWrapWord
	vs.statusLabel = statusLabel
	if vs.statusLabel != nil {
		vs.statusLabel.SetText(t.StatusReady)
	}
	if opts.SetRipStatusLabel != nil {
		opts.SetRipStatusLabel(statusLabel)
	}

	progressBar := widget.NewProgressBar()
	progressBar.SetValue(vs.progress / 100.0)
	vs.progressBar = progressBar
	if opts.SetRipProgressBar != nil {
		opts.SetRipProgressBar(progressBar)
	}

	logEntry := widget.NewMultiLineEntry()
	logEntry.Wrapping = fyne.TextWrapOff
	logEntry.Disable()
	logEntry.SetText(vs.logText)
	vs.logEntry = logEntry
	logScroll := container.NewVScroll(logEntry)
	vs.logScroll = logScroll
	if opts.SetRipLogEntry != nil {
		opts.SetRipLogEntry(logEntry)
	}
	if opts.SetRipLogScroll != nil {
		opts.SetRipLogScroll(logScroll)
	}

	copyLogBtn := widget.NewButton(t.ActionCopyLog, func() {
		if strings.TrimSpace(vs.logText) == "" {
			return
		}
		opts.Window.Clipboard().SetContent(vs.logText)
	})
	copyLogBtn.Importance = widget.LowImportance

	applyControls := func() {
		formatSelect.SetSelected(vs.format)
		outputEntry.SetText(vs.outputPath)
	}

	addToQueue := func(runNow bool) error {
		jq := opts.JobQueue()
		if jq == nil {
			return fmt.Errorf("queue not initialized")
		}
		if strings.TrimSpace(vs.sourcePath) == "" {
			return fmt.Errorf("%s", t.RipErrNoSource)
		}
		if strings.TrimSpace(vs.outputPath) == "" {
			vs.outputPath = DefaultOutputPath(vs.sourcePath, vs.format)
		}
		job := &queue.Job{
			Type:        queue.JobTypeRip,
			Title:       fmt.Sprintf("Rip DVD: %s", filepath.Base(vs.sourcePath)),
			Description: fmt.Sprintf("Output: %s", utils.ShortenMiddle(filepath.Base(vs.outputPath), 40)),
			InputFile:   vs.sourcePath,
			OutputFile:  vs.outputPath,
			Config: map[string]interface{}{
				"sourcePath": vs.sourcePath,
				"outputPath": vs.outputPath,
				"format":     vs.format,
			},
		}
		vs.resetLog()
		vs.setStatus("Queued rip job...")
		vs.setProgress(0)
		opts.AddJob(job)
		if runNow && !jq.IsRunning() {
			jq.Start()
		}
		return nil
	}

	addQueueBtn := widget.NewButton(t.RipAddToQueue, func() {
		if err := addToQueue(false); err != nil {
			dialog.ShowError(err, opts.Window)
			return
		}
		dialog.ShowInformation(t.RipJobQueuedTitle, t.RipJobQueuedMsg, opts.Window)
		jq := opts.JobQueue()
		if jq != nil && !jq.IsRunning() {
			jq.Start()
		}
	})
	addQueueBtn.Importance = widget.MediumImportance

	runNowBtn := widget.NewButton(t.RipNow, func() {
		if err := addToQueue(true); err != nil {
			dialog.ShowError(err, opts.Window)
			return
		}
		jq := opts.JobQueue()
		if jq != nil && !jq.IsRunning() {
			jq.Start()
		}
		dialog.ShowInformation(t.RipStartTitle, t.RipStartMsg, opts.Window)
	})
	runNowBtn.Importance = widget.HighImportance

	loadCfgBtn := widget.NewButton(t.ActionLoadConfig, func() {
		cfg, err := loadPersistedRipConfig()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				dialog.ShowInformation(t.RipNoConfigTitle, t.RipNoConfigMsg, opts.Window)
			} else {
				dialog.ShowError(fmt.Errorf("failed to load config: %w", err), opts.Window)
			}
			return
		}
		vs.applyConfig(cfg)
		vs.outputPath = DefaultOutputPath(vs.sourcePath, vs.format)
		if opts.SetRipOutputPath != nil {
			opts.SetRipOutputPath(vs.outputPath)
		}
		applyControls()
	})

	saveCfgBtn := widget.NewButton(t.ActionSaveConfig, func() {
		cfg := ripConfig{Format: vs.format}
		if err := savePersistedRipConfig(cfg); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save config: %w", err), opts.Window)
			return
		}
		dialog.ShowInformation(t.RipConfigSavedTitle, fmt.Sprintf(t.RipConfigSavedFmt, configpath.ModuleConfigPath("rip")), opts.Window)
	})

	resetBtn := widget.NewButton(t.ActionReset, func() {
		cfg := defaultRipConfig()
		vs.applyConfig(cfg)
		vs.outputPath = DefaultOutputPath(vs.sourcePath, vs.format)
		if opts.SetRipOutputPath != nil {
			opts.SetRipOutputPath(vs.outputPath)
		}
		applyControls()
		vs.persistConfig()
	})

	clearISOBtn := widget.NewButton(t.RipClearISO, func() {
		vs.sourcePath = ""
		vs.outputPath = ""
		if opts.SetRipSourcePath != nil {
			opts.SetRipSourcePath("")
		}
		if opts.SetRipOutputPath != nil {
			opts.SetRipOutputPath("")
		}
		sourceEntry.SetText("")
		outputEntry.SetText("")
	})
	clearISOBtn.Importance = widget.LowImportance

	controls := container.NewVBox(
		widget.NewLabelWithStyle(t.RipSource, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		ui.NewDroppable(sourceEntry, func(items []fyne.URI) {
			path := ""
			if opts.OnDropFirstLocal != nil {
				path = opts.OnDropFirstLocal(items)
			}
			if path != "" {
				vs.sourcePath = path
				sourceEntry.SetText(path)
				if opts.SetRipSourcePath != nil {
					opts.SetRipSourcePath(path)
				}

				// Dynamic detection for ISO files
				if strings.HasSuffix(strings.ToLower(path), ".iso") {
					if discType, err := udf.IdentifyDiscFormat(path); err == nil {
						logging.Info(logging.CatDVD, "User dropped ISO: detected as %s", discType)
					}
				} else {
					// Check if it's a VIDEO_TS folder
					if info, err := os.Stat(filepath.Join(path, "VIDEO_TS.IFO")); err == nil && !info.IsDir() {
						if opts.OnScanDVDStruct != nil {
							opts.OnScanDVDStruct(path)
						}
					}
				}

				vs.outputPath = DefaultOutputPath(path, vs.format)
				if opts.SetRipOutputPath != nil {
					opts.SetRipOutputPath(vs.outputPath)
				}
				outputEntry.SetText(vs.outputPath)
			}
		}),
		clearISOBtn,
		widget.NewLabelWithStyle(t.RipFormatLabel, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		formatSelect,
		widget.NewLabelWithStyle(t.LabelOutput, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		outputEntry,
		container.NewHBox(resetBtn, loadCfgBtn, saveCfgBtn),
		widget.NewSeparator(),
		widget.NewLabelWithStyle(t.LabelStatus, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		statusLabel,
		progressBar,
		widget.NewSeparator(),
		container.NewHBox(
			widget.NewLabelWithStyle(t.RipLog, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			layout.NewSpacer(),
			copyLogBtn,
		),
		logScroll,
	)

	var bottomBar fyne.CanvasObject
	if opts.OnModuleFooter != nil {
		bottomBar = opts.OnModuleFooter(opts.ModuleColor, container.NewHBox(addQueueBtn, layout.NewSpacer(), runNowBtn), opts.OnGetStatsBar())
	}
	return container.NewBorder(topBar, bottomBar, nil, nil, container.NewVScroll(container.NewPadded(controls)))
}
