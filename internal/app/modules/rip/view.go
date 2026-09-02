package rip

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/LeakTechnologies/VideoTools/internal/app/configpath"
	"github.com/LeakTechnologies/VideoTools/internal/app/modulecfg"
	"github.com/LeakTechnologies/VideoTools/internal/i18n"
	"github.com/LeakTechnologies/VideoTools/internal/logging"
	"github.com/LeakTechnologies/VideoTools/internal/queue"
	"github.com/LeakTechnologies/VideoTools/internal/ui"
	"github.com/LeakTechnologies/VideoTools/internal/utils"
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
	sourcePath       string
	outputPath       string
	format           string
	embedChapters    bool
	allAudioTracks   bool
	includeSubtitles bool
	includeMenus     bool
	regionConvert    string // "" (none), "pal2ntsc", "ntsc2pal"
	extractMode      string // "" (main feature) or "full" (full disc with IFO regen)
	discTitle        string
	logText          string
	progress         float64

	scanResult     *DiscScanResult
	selectedTitles map[int]bool // title Number → selected
	videoTSPath    string       // resolved VIDEO_TS dir; empty for ISOs / unloaded

	statusLabel *widget.Label
	progressBar *widget.ProgressBar
	logEntry    *widget.Label
	logScroll   *container.Scroll
}

func (vs *viewState) applyConfig(cfg ripConfig) {
	vs.format = cfg.Format
	vs.embedChapters = cfg.EmbedChapters
	vs.allAudioTracks = cfg.AllAudioTracks
	vs.includeSubtitles = cfg.IncludeSubtitles
	vs.includeMenus = cfg.IncludeMenus
}

func (vs *viewState) persistConfig() {
	cfg := ripConfig{
		Format:           vs.format,
		EmbedChapters:    vs.embedChapters,
		AllAudioTracks:   vs.allAudioTracks,
		IncludeSubtitles: vs.includeSubtitles,
		IncludeMenus:     vs.includeMenus,
	}
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

	// rebuildEnrich / rebuildTitleNav are assigned after their widgets are created;
	// declared here so formatSelect and the drop handler can capture them by ref.
	var rebuildEnrich func()
	var rebuildTitleNav func()
	var updateDiscInfo func()
	var updateRipSummary func()
	var discSummary *DiscSummary
	var ripSummaryLbl *widget.Label
	var logVSplit *container.Split

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

	backBtn := ui.MakePillButton("< "+strings.ToUpper(t.ModuleRip), ui.BorderDim, opts.OnShowMainMenu)

	queueBtn := opts.QueueBtn
	if queueBtn == nil {
		queueBtn = ui.MakePillButton(t.ActionViewQueue, ui.BorderDim, opts.OnShowQueue)
	}
	if opts.SetQueueBtn != nil {
		opts.SetQueueBtn(queueBtn)
	}
	if opts.OnUpdateQueueButtonLabel != nil {
		opts.OnUpdateQueueButtonLabel()
	}

	clearCompletedBtn := ui.MakePillButton("⌫", ui.BorderDim, func() {
		if opts.OnClearCompleted != nil {
			opts.OnClearCompleted()
		}
	})

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
		if rebuildEnrich != nil {
			rebuildEnrich()
		}
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

	logEntry := widget.NewLabel("")
	logEntry.Wrapping = fyne.TextWrapWord
	logEntry.TextStyle = fyne.TextStyle{Monospace: true}
	if vs.logText != "" {
		logEntry.SetText(vs.logText)
	}
	vs.logEntry = logEntry
	logScroll := container.NewVScroll(logEntry)
	logScroll.SetMinSize(fyne.NewSize(0, 40))
	vs.logScroll = logScroll
	if opts.SetRipLogEntry != nil {
		opts.SetRipLogEntry(logEntry)
	}
	if opts.SetRipLogScroll != nil {
		opts.SetRipLogScroll(logScroll)
	}

	ripTeal := color.NRGBA{R: 0x1a, G: 0x93, B: 0x73, A: 0xff}

	var collapseLogBtn *ui.PillButton
	collapseLogBtn = ui.MakePillButton(t.RipLogOpen, ui.BorderDim, func() {
		if logVSplit.Offset > 0.9 {
			logVSplit.SetOffset(0.60)
			collapseLogBtn.SetText(t.RipLogOpen)
		} else {
			logVSplit.SetOffset(0.97)
			collapseLogBtn.SetText(t.RipLogClose)
		}
	})

	logSection := ui.NewConsoleBox(
		t.RipLog,
		ripTeal,
		logScroll,
		func() string {
			if vs.logEntry != nil {
				return vs.logEntry.Text
			}
			return vs.logText
		},
		opts.Window,
		collapseLogBtn,
	)

	ripNavy := utils.MustHex("#191F35")
	buildRipBox := func(title string, content fyne.CanvasObject) *fyne.Container {
		bg := canvas.NewRectangle(ripNavy)
		bg.CornerRadius = 10
		bg.StrokeColor = ui.GridColor
		bg.StrokeWidth = 1
		headerBg := canvas.NewRectangle(ripTeal)
		headerBg.CornerRadius = 10
		headerBg.SetMinSize(fyne.NewSize(0, 34))
		headerTitle := canvas.NewText(strings.ToUpper(title), color.White)
		headerTitle.TextStyle = fyne.TextStyle{Bold: true}
		headerTitle.TextSize = 12
		header := container.NewMax(
			headerBg,
			container.NewPadded(container.NewHBox(headerTitle, layout.NewSpacer())),
		)
		body := container.NewBorder(header, nil, nil, nil, container.NewPadded(content))
		layers := ui.NoisyBackgroundObjects(bg)
		layers = append(layers, body)
		return container.NewMax(layers...)
	}

	sectionGap := func() fyne.CanvasObject {
		gap := canvas.NewRectangle(color.Transparent)
		gap.SetMinSize(fyne.NewSize(0, 10))
		return gap
	}

	// ── DVD Player (for background playback; hidden from main layout) ──────
	dvdPlayer := ui.NewInlineVideoPlayer()
	dvdPlayer.SetIdleText("LOAD DISC TO RIP")

	// ── Content Browser ─────────────────────────────────────────────────────
	contentBrowser := NewContentBrowser()
	contentBrowser.SetOnSelect(func(titleNum int, selected bool) {
		vs.selectedTitles[titleNum] = selected
		if updateRipSummary != nil {
			updateRipSummary()
		}
	})
	contentBrowser.SetOnPreview(func(titleNum int) {
		contentBrowser.SetFocused(titleNum)
		discRoot := resolveDVDRoot(vs.sourcePath)
		go func() { _ = dvdPlayer.LoadDVD(discRoot, titleNum) }()
	})

	// ── Menu Preview ───────────────────────────────────────────────────────
	menuPreview := NewMenuPreview()

	openInPlayerBtn := ui.MakePillButton(t.RipOpenInPlayer, opts.ModuleColor, func() {
		if vs.sourcePath == "" {
			dialog.ShowError(fmt.Errorf("%s", t.RipErrNoDiscLoaded), opts.Window)
			return
		}
		if opts.OnOpenInPlayer != nil {
			opts.OnOpenInPlayer(vs.sourcePath)
		}
	})

	// rebuildTitleNav now updates the ContentBrowser with scan results.
	rebuildTitleNav = func() {
		contentBrowser.SetScanResult(vs.scanResult, vs.sourcePath)
		if vs.scanResult != nil && len(vs.scanResult.Titles) > 0 {
			menuPreview.SetSourcePath(vs.sourcePath)
		}
	}

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

		// Full-disc extraction is always a single job.
		if vs.extractMode == "full" {
			job := &queue.Job{
				Type:        queue.JobTypeRip,
				Title:       fmt.Sprintf("Full disc: %s", filepath.Base(vs.sourcePath)),
				Description: fmt.Sprintf("Output: %s", utils.ShortenMiddle(filepath.Base(vs.outputPath), 40)),
				InputFile:   vs.sourcePath,
				OutputFile:  vs.outputPath,
				Config: map[string]interface{}{
					"sourcePath":    vs.sourcePath,
					"outputPath":    vs.outputPath,
					"format":        vs.format,
					"regionConvert": vs.regionConvert,
					"extractMode":   vs.extractMode,
					"discTitle":     vs.discTitle,
				},
			}
			opts.AddJob(job)
			vs.resetLog()
			vs.setStatus("Queued full-disc rip job...")
			vs.setProgress(0)
			if runNow && !jq.IsRunning() {
				jq.Start()
			}
			return nil
		}

		// Build list of (vtsNumber, outputPath, title) for each job to enqueue.
		type titleJob struct {
			vtsNumber    int
			titleNumber  int
			outputPath   string
			jobTitle     string
		}
		var jobs []titleJob

		if vs.scanResult != nil && len(vs.scanResult.Titles) > 1 {
			ext := filepath.Ext(vs.outputPath)
			base := strings.TrimSuffix(vs.outputPath, ext)

			// Find the main feature (longest duration).
			mainIdx := 0
			mainDur := 0.0
			for i, dt := range vs.scanResult.Titles {
				if dt.Duration > mainDur {
					mainDur = dt.Duration
					mainIdx = i
				}
			}

			for i, dt := range vs.scanResult.Titles {
				if !vs.selectedTitles[dt.Number] {
					continue
				}
				titlePath := vs.outputPath
				jobLabel := fmt.Sprintf("Rip DVD: %s", filepath.Base(vs.sourcePath))
				if i != mainIdx {
					titlePath = fmt.Sprintf("%s_Extra_Title_%02d%s", base, dt.Number, ext)
					jobLabel = fmt.Sprintf("Rip DVD Title %d (extra): %s", dt.Number, filepath.Base(vs.sourcePath))
				}
				jobs = append(jobs, titleJob{
					vtsNumber:   dt.VTSNumber,
					titleNumber: dt.Number,
					outputPath:  titlePath,
					jobTitle:    jobLabel,
				})
			}
			if len(jobs) == 0 {
				return fmt.Errorf("no titles selected")
			}
		} else {
			vtsNumber := 0
			titleNumber := 0
			if vs.scanResult != nil && len(vs.scanResult.Titles) == 1 {
				vtsNumber = vs.scanResult.Titles[0].VTSNumber
				titleNumber = vs.scanResult.Titles[0].Number
			}
			jobs = []titleJob{{
				vtsNumber:   vtsNumber,
				titleNumber: titleNumber,
				outputPath:  vs.outputPath,
				jobTitle:    fmt.Sprintf("Rip DVD: %s", filepath.Base(vs.sourcePath)),
			}}
		}

		for _, j := range jobs {
			job := &queue.Job{
				Type:        queue.JobTypeRip,
				Title:       j.jobTitle,
				Description: fmt.Sprintf("Output: %s", utils.ShortenMiddle(filepath.Base(j.outputPath), 40)),
				InputFile:   vs.sourcePath,
				OutputFile:  j.outputPath,
			Config: map[string]interface{}{
				"sourcePath":       vs.sourcePath,
				"outputPath":       j.outputPath,
				"format":           vs.format,
				"embedChapters":    vs.embedChapters,
				"allAudioTracks":   vs.allAudioTracks,
				"includeSubtitles": vs.includeSubtitles,
				"includeMenus":     vs.includeMenus,
				"regionConvert":    vs.regionConvert,
				"discTitle":        vs.discTitle,
				"vtsNumber":        j.vtsNumber,
				"titleNumber":      j.titleNumber,
			},
			}
			opts.AddJob(job)
		}

		vs.resetLog()
		vs.setStatus(fmt.Sprintf("Queued %d rip job(s)...", len(jobs)))
		vs.setProgress(0)
		if runNow && !jq.IsRunning() {
			jq.Start()
		}
		return nil
	}

	addQueueBtn := ui.MakePillButton(t.RipAddToQueue, opts.ModuleColor, func() {
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

	runNowBtn := ui.MakePillButton(t.RipNow, opts.ModuleColor, func() {
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

	// countSelected returns the number of titles currently ticked for rip.
	countSelected := func() int {
		n := 0
		for _, dt := range vs.scanResult.Titles {
			if vs.selectedTitles[dt.Number] {
				n++
			}
		}
		return n
	}
	// updateRipSummary refreshes the CTA line ("Ready to rip N title(s)").
	updateRipSummary = func() {
		if ripSummaryLbl == nil {
			return
		}
		if vs.scanResult == nil || len(vs.scanResult.Titles) == 0 {
			ripSummaryLbl.SetText(t.RipReadyNoTitles)
			return
		}
		sel := countSelected()
		if sel == 0 {
			ripSummaryLbl.SetText(t.RipReadyNoSelection)
			return
		}
		if sel == 1 {
			ripSummaryLbl.SetText(t.RipReadyOne)
			return
		}
		ripSummaryLbl.SetText(fmt.Sprintf(t.RipReadyManyFmt, sel))
	}
	ripSummaryLbl = widget.NewLabel("")
	ripSummaryLbl.Importance = widget.MediumImportance
	ripSummaryLbl.Wrapping = fyne.TextWrapWord
	loadCfgBtn := ui.MakePillButton(t.ActionLoadConfig, ui.BorderDim, func() {
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

	saveCfgBtn := ui.MakePillButton(t.ActionSaveConfig, ui.BorderDim, func() {
		vs.persistConfig()
		dialog.ShowInformation(t.RipConfigSavedTitle, fmt.Sprintf(t.RipConfigSavedFmt, configpath.ModuleConfigPath("rip")), opts.Window)
	})

	resetBtn := ui.MakePillButton(t.ActionReset, ui.BorderDim, func() {
		cfg := defaultRipConfig()
		vs.applyConfig(cfg)
		vs.outputPath = DefaultOutputPath(vs.sourcePath, vs.format)
		if opts.SetRipOutputPath != nil {
			opts.SetRipOutputPath(vs.outputPath)
		}
		applyControls()
		vs.persistConfig()
	})

	clearISOBtn := ui.MakePillButton(t.RipClearISO, ui.BorderDim, func() {
		vs.sourcePath = ""
		vs.outputPath = ""
		vs.videoTSPath = ""
		vs.resetLog()
		vs.scanResult = nil
		vs.selectedTitles = nil
		dvdPlayer.Close()
		rebuildTitleNav()
		rebuildEnrich()
		if opts.SetRipSourcePath != nil {
			opts.SetRipSourcePath("")
		}
		if opts.SetRipOutputPath != nil {
			opts.SetRipOutputPath("")
		}
		sourceEntry.SetText("")
		outputEntry.SetText("")
	})
	// ── Enrichment options ───────────────────────────────────────────────────
	titleEntry := widget.NewEntry()
	titleEntry.SetPlaceHolder(t.RipTitlePlaceholder)
	titleEntry.SetText(vs.discTitle)
	titleEntry.OnChanged = func(v string) { vs.discTitle = strings.TrimSpace(v) }

	chaptersCheck := widget.NewCheck(t.RipEmbedChapters, func(v bool) {
		vs.embedChapters = v
		vs.persistConfig()
	})
	chaptersCheck.SetChecked(vs.embedChapters)

	allAudioCheck := widget.NewCheck(t.RipAllAudioTracks, func(v bool) {
		vs.allAudioTracks = v
		vs.persistConfig()
	})
	allAudioCheck.SetChecked(vs.allAudioTracks)

	subsCheck := widget.NewCheck(t.RipIncludeSubtitles, func(v bool) {
		vs.includeSubtitles = v
		vs.persistConfig()
	})
	subsCheck.SetChecked(vs.includeSubtitles)

	menusCheck := widget.NewCheck(t.RipPreserveMenusFull, func(v bool) {
		vs.includeMenus = v
		vs.persistConfig()
	})
	menusCheck.SetChecked(vs.includeMenus)

	var fullDiscCheck *widget.Check // assigned below; referenced by ntscSelect callback
	fullDiscCheck = widget.NewCheck(t.RipFullDiscExtraction, func(v bool) {
		if v && vs.regionConvert != "" {
			vs.extractMode = "full"
			vs.outputPath = FullDiscOutputPath(vs.sourcePath)
		} else {
			vs.extractMode = ""
			vs.outputPath = DefaultOutputPath(vs.sourcePath, vs.format)
		}
		if opts.SetRipOutputPath != nil {
			opts.SetRipOutputPath(vs.outputPath)
		}
		outputEntry.SetText(vs.outputPath)
	})
	fullDiscCheck.SetChecked(false)
	fullDiscCheck.Disable()

	ntscSelect := widget.NewSelect([]string{t.RipRegionNone, t.RipRegionPALtoNTSC, t.RipRegionNTSCtoPAL}, func(value string) {
		switch value {
		case t.RipRegionPALtoNTSC:
			vs.regionConvert = "pal2ntsc"
		case t.RipRegionNTSCtoPAL:
			vs.regionConvert = "ntsc2pal"
		default:
			vs.regionConvert = ""
		}
		if vs.regionConvert != "" && vs.scanResult != nil && len(vs.scanResult.Titles) > 0 {
			vs.extractMode = "full"
			fullDiscCheck.SetChecked(true)
		} else {
			vs.extractMode = ""
			fullDiscCheck.SetChecked(false)
		}
	})
	ntscSelect.SetSelected(t.RipRegionNone)

	enrichContent := container.NewVBox()

	// Pre-fill title from source path when source changes
	sourceChangedHook := func(path string) {
		if vs.discTitle == "" && path != "" {
			base := filepath.Base(strings.TrimSuffix(path, string(filepath.Separator)))
			if strings.EqualFold(base, "VIDEO_TS") {
				base = filepath.Base(filepath.Dir(path))
			}
			base = strings.TrimSuffix(base, filepath.Ext(base))
			titleEntry.SetText(base)
			vs.discTitle = base
		}
	}

	rebuildEnrich = func() {
		var mainTitle *DiscTitle
		if vs.scanResult != nil && len(vs.scanResult.Titles) > 0 {
			mainTitle = &vs.scanResult.Titles[0]
		}

		// Chapter checkbox
		chapLabel := t.RipEmbedChapters
		if mainTitle != nil {
			if mainTitle.NumChapters > 1 {
				chapLabel = fmt.Sprintf(t.RipEmbedChaptersCountFmt, mainTitle.NumChapters)
				chaptersCheck.Enable()
			} else {
				chapLabel = t.RipEmbedChaptersNone
				chaptersCheck.SetChecked(false)
				chaptersCheck.Disable()
			}
		} else {
			chaptersCheck.Enable()
		}
		chaptersCheck.Text = chapLabel
		chaptersCheck.Refresh()

		// Audio checkbox
		audioLabel := t.RipAllAudioTracks
		if mainTitle != nil && len(mainTitle.Audio) > 0 {
			if langs := langList(mainTitle.Audio); langs != "" {
				audioLabel = fmt.Sprintf(t.RipAllAudioTracksLangsFmt, len(mainTitle.Audio), langs)
			} else {
				audioLabel = fmt.Sprintf(t.RipAllAudioTracksCountFmt, len(mainTitle.Audio))
			}
		}
		allAudioCheck.Text = audioLabel
		allAudioCheck.Refresh()

		// Subtitle checkbox
		subsLabel := t.RipIncludeSubtitles
		if vs.format == FormatH264MP4 {
			subsLabel = t.RipIncludeSubtitlesMP4
			subsCheck.SetChecked(false)
			subsCheck.Disable()
		} else if mainTitle != nil {
			if len(mainTitle.Subtitles) == 0 {
				subsLabel = t.RipIncludeSubtitlesNone
				subsCheck.SetChecked(false)
				subsCheck.Disable()
			} else {
				subsLabel = fmt.Sprintf(t.RipIncludeSubtitlesCountFmt, len(mainTitle.Subtitles))
				subsCheck.Enable()
			}
		} else {
			subsCheck.Enable()
		}
		subsCheck.Text = subsLabel
		subsCheck.Refresh()

		// Region conversion dropdown — only shown on H.264 re-encode formats.
		if vs.format == FormatLosslessMKV || vs.format == FormatArchivist {
			ntscSelect.Hide()
			fullDiscCheck.Hide()
		} else {
			ntscSelect.Show()
			// Full-disc checkbox is only relevant when region conversion is active
			if vs.regionConvert != "" && vs.scanResult != nil && len(vs.scanResult.Titles) > 0 {
				fullDiscCheck.Show()
				fullDiscCheck.Enable()
			} else {
				fullDiscCheck.Hide()
			}
		}

		// Disc info label at the top of the view — decoupled into its own
		// function so a partial failure in the enrichment rebuild can never
		// hide the disc summary, and so it can be called independently on
		// scan completion.
		updateDiscInfo()

		// Rebuild content objects
		objs := []fyne.CanvasObject{
			widget.NewLabelWithStyle(t.RipTitleLabel, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			titleEntry,
			chaptersCheck,
			allAudioCheck,
			subsCheck,
		}

		if vs.scanResult != nil && len(vs.scanResult.Titles) > 1 {
			objs = append(objs, widget.NewSeparator())
			objs = append(objs,
				widget.NewLabelWithStyle(
					fmt.Sprintf(t.RipTitlesOnDiscFmt, len(vs.scanResult.Titles)),
					fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		}

		// Uncommon options live in an Advanced accordion so the day-to-day
		// output path stays clean: menus preservation, PAL↔NTSC conversion,
		// and full-disc extraction.
		advanced := container.NewVBox(
			menusCheck,
			widget.NewSeparator(),
			widget.NewLabelWithStyle(t.RipRegionConversion, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			ntscSelect,
			fullDiscCheck,
		)
		objs = append(objs, widget.NewSeparator())
		objs = append(objs, widget.NewAccordion(widget.NewAccordionItem(t.RipAdvancedOptions, advanced)))

		enrichContent.Objects = objs
		enrichContent.Refresh()

		if updateRipSummary != nil {
			updateRipSummary()
		}
	}

	discSummary = NewDiscSummary()

	// updateDiscInfo pushes the scan result into the disc summary card. It is
	// called independently on scan completion (and from rebuildEnrich) so the
	// summary reliably tracks scan state regardless of the rest of the
	// enrichment panel. It must be assigned BEFORE the initial rebuildEnrich()
	// below — that call invokes it to render the empty state, and calling the
	// still-nil closure panics the process.
	updateDiscInfo = func() {
		if discSummary == nil {
			return
		}
		if vs.scanResult == nil {
			discSummary.SetEmpty()
			return
		}
		discTitle := vs.discTitle
		if discTitle == "" && vs.sourcePath != "" {
			base := filepath.Base(strings.TrimSuffix(vs.sourcePath, string(filepath.Separator)))
			if strings.EqualFold(base, "VIDEO_TS") {
				base = filepath.Base(filepath.Dir(vs.sourcePath))
			}
			discTitle = strings.TrimSuffix(base, filepath.Ext(base))
		}
		discSummary.SetResult(vs.scanResult, discTitle)
	}

	// Initial render of enrichment panel (no scan result yet)
	rebuildEnrich()

	// loadDisc is the single entry-point for loading an ISO or VIDEO_TS path —
	// shared by drop, Browse, and the old Folder picker path.
	loadDisc := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}

		// Reject non-disc files: only .iso and VIDEO_TS directories are valid.
		lower := strings.ToLower(path)
		isISO := strings.HasSuffix(lower, ".iso")
		isVideoTS := strings.Contains(lower, "video_ts")
		if !isISO && !isVideoTS {
			if discSummary != nil {
				discSummary.SetError(t.RipErrNotDisc)
			}
			return
		}
		if discSummary != nil {
			discSummary.SetScanning()
		}

		vs.sourcePath = path
		sourceEntry.SetText(path)
		if opts.SetRipSourcePath != nil {
			opts.SetRipSourcePath(path)
		}
		sourceChangedHook(path)
		vs.outputPath = DefaultOutputPath(path, vs.format)
		if opts.SetRipOutputPath != nil {
			opts.SetRipOutputPath(vs.outputPath)
		}
		outputEntry.SetText(vs.outputPath)

		vs.scanResult = nil
		vs.selectedTitles = nil
		rebuildEnrich()

		if strings.HasSuffix(strings.ToLower(path), ".iso") {
			go func() {
				result, scanErr := scanISOViaUDF(path)
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					if scanErr != nil {
						logging.Warning(logging.CatDVD, "ISO scan failed: %v", scanErr)
						if discSummary != nil {
							discSummary.SetError(shortScanError(scanErr))
						}
					} else {
						vs.scanResult = result
						if len(result.Titles) > 0 {
							vs.selectedTitles = make(map[int]bool)
							for _, dt := range result.Titles {
								vs.selectedTitles[dt.Number] = true
							}
							go func() { _ = dvdPlayer.LoadDVD(path, result.Titles[0].Number) }()
						}
						rebuildTitleNav()
						rebuildEnrich()
						updateDiscInfo()
					}
				}, false)
			}()
		} else {
			go func() {
				vtsp, _, err := ResolveVideoTSPath(context.Background(), path)
				if err != nil {
					logging.Warning(logging.CatDVD, "ResolveVideoTSPath failed: %v", err)
					fyne.CurrentApp().Driver().DoFromGoroutine(func() {
						if discSummary != nil {
							discSummary.SetError(shortScanError(err))
						}
					}, false)
					return
				}
				result, scanErr := ScanDisc(vtsp)
				fyne.CurrentApp().Driver().DoFromGoroutine(func() {
					if scanErr != nil {
						logging.Warning(logging.CatDVD, "disc scan failed: %v", scanErr)
						if discSummary != nil {
							discSummary.SetError(shortScanError(scanErr))
						}
					} else {
						vs.scanResult = result
						vs.videoTSPath = vtsp
						vs.selectedTitles = make(map[int]bool)
						for _, dt := range result.Titles {
							vs.selectedTitles[dt.Number] = true
						}
						if len(result.Titles) > 0 {
							discRoot := resolveDVDRoot(vs.sourcePath)
							go func() { _ = dvdPlayer.LoadDVD(discRoot, result.Titles[0].Number) }()
						}
						rebuildTitleNav()
						rebuildEnrich()
						updateDiscInfo()
					}
				}, false)
			}()
		}
	}

	browseBtn := ui.MakePillButton("...", ui.BorderDim, func() {
		d := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			loadDisc(reader.URI().Path())
		}, opts.Window)
		d.Resize(fyne.NewSize(900, 640))
		d.Show()
	})

	// ── Linear workflow: SOURCE → DISC → TITLES → OUTPUT → ACTION ──────────
	flow := container.NewVBox(
		buildRipBox(t.RipSource, container.NewVBox(
			container.NewBorder(nil, nil, nil,
				container.NewHBox(browseBtn, clearISOBtn),
				ui.NewDroppable(sourceEntry, func(items []fyne.URI) {
					if opts.OnDropFirstLocal != nil {
						loadDisc(opts.OnDropFirstLocal(items))
					}
				}),
			),
		)),
		sectionGap(),
		discSummary.GetContainer(),
		sectionGap(),
		// Titles: the Content Browser lists each title with selection toggles,
		// main-feature badge and metadata — reused as the workflow centerpiece.
		contentBrowser.GetContainer(),
		sectionGap(),
		buildRipBox(t.RipFormatLabel, container.NewVBox(
			formatSelect,
			enrichContent,
		)),
		sectionGap(),
		menuPreview.GetContainer(),
		sectionGap(),
		buildRipBox(t.LabelOutput, container.NewVBox(
			outputEntry,
			container.NewHBox(resetBtn, loadCfgBtn, saveCfgBtn),
		)),
		sectionGap(),
		buildRipBox(t.LabelStatus, container.NewVBox(
			ripSummaryLbl,
			statusLabel,
			progressBar,
			container.NewHBox(addQueueBtn, layout.NewSpacer(), openInPlayerBtn, runNowBtn),
		)),
	)
	mainScroll := container.NewVScroll(container.NewPadded(flow))

	var bottomBar fyne.CanvasObject
	if opts.OnModuleFooter != nil {
		bottomBar = opts.OnModuleFooter(opts.ModuleColor, nil, opts.OnGetStatsBar())
	}

	logVSplit = container.NewVSplit(mainScroll, logSection)
	// Default to a compact log strip; the ▼▶ LOG toggle expands it during a rip.
	logVSplit.SetOffset(0.92)
	return container.NewBorder(topBar, bottomBar, nil, nil,
		logVSplit,
	)
}

// collectVTSVOBFiles returns the content VOB paths for a VTS set in playback order.
// VTS_XX_0.VOB is the menu VOB and is excluded; VTS_XX_1.VOB onward are content.
func collectVTSVOBFiles(videoTSPath string, vtsNum int) []string {
	if vtsNum <= 0 {
		vtsNum = 1
	}
	prefix := strings.ToUpper(fmt.Sprintf("VTS_%02d_", vtsNum))
	entries, err := os.ReadDir(videoTSPath)
	if err != nil {
		return nil
	}
	var vobs []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		upper := strings.ToUpper(entry.Name())
		if strings.HasPrefix(upper, prefix) &&
			strings.HasSuffix(upper, ".VOB") &&
			!strings.HasSuffix(upper, "_0.VOB") {
			vobs = append(vobs, filepath.Join(videoTSPath, entry.Name()))
		}
	}
	sort.Strings(vobs)
	return vobs
}

// buildDiscConcatURL returns an ffmpeg concat: protocol URL covering all content
// VOBs for the given VTS set. Returns "" if no VOBs are found.
func buildDiscConcatURL(videoTSPath string, vtsNum int) string {
	vobs := collectVTSVOBFiles(videoTSPath, vtsNum)
	if len(vobs) == 0 {
		return ""
	}
	if len(vobs) == 1 {
		return vobs[0] // single file — no concat protocol needed
	}
	parts := make([]string, len(vobs))
	for i, p := range vobs {
		// concat: protocol uses | as separator; convert backslashes and encode spaces
		p = filepath.ToSlash(p)
		p = strings.ReplaceAll(p, " ", "%20")
		parts[i] = p
	}
	return "concat:" + strings.Join(parts, "|")
}
