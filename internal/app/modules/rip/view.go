package rip

import (
	"bytes"
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
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/app/configpath"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/app/modulecfg"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/dvd/ifo"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/dvd/udf"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/queue"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/ui"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/utils"
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
}

func (vs *viewState) persistConfig() {
	cfg := ripConfig{
		Format:           vs.format,
		EmbedChapters:    vs.embedChapters,
		AllAudioTracks:   vs.allAudioTracks,
		IncludeSubtitles: vs.includeSubtitles,
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
	var discInfoLabel *widget.Label

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
	logScroll.SetMinSize(fyne.NewSize(0, 160))
	vs.logScroll = logScroll
	if opts.SetRipLogEntry != nil {
		opts.SetRipLogEntry(logEntry)
	}
	if opts.SetRipLogScroll != nil {
		opts.SetRipLogScroll(logScroll)
	}

	ripTeal := color.NRGBA{R: 0x1a, G: 0x93, B: 0x73, A: 0xff}
	logSection := ui.NewConsoleBox(
		t.RipLog,
		ripTeal,
		logScroll,
		func() string { return vs.logText },
		opts.Window,
	)

	// ── DVD Player ───────────────────────────────────────────────────────────
	dvdPlayer := ui.NewInlineVideoPlayer()
	dvdPlayer.SetIdleText("LOAD DISC TO RIP")

	var playerCanvas fyne.CanvasObject
	if w := dvdPlayer.Widget(); w != nil {
		playerCanvas = ui.BuildPlayerContainer(w, fyne.NewSize(0, 0))
	} else {
		// Non-native build: static dark placeholder
		bg := canvas.NewRectangle(utils.MustHex("#0F1529"))
		bg.CornerRadius = 8
		bg.StrokeColor = ui.GridColor
		bg.StrokeWidth = 1
		txt := canvas.NewText("LOAD DISC TO RIP", color.NRGBA{R: 80, G: 80, B: 80, A: 255})
		txt.TextStyle = fyne.TextStyle{Monospace: true}
		txt.Alignment = fyne.TextAlignCenter
		playerCanvas = container.NewMax(bg, container.NewCenter(txt))
	}

	// Title navigation (revealed when a multi-title disc is loaded)
	titleIdx := 0
	var titleNavSelect *widget.Select
	var prevTitleBtn, nextTitleBtn *ui.PillButton

	prevTitleBtn = ui.MakePillButton("◀", ui.BorderDim, func() {
		if vs.scanResult == nil || titleIdx <= 0 || vs.videoTSPath == "" {
			return
		}
		titleIdx--
		titleNavSelect.SetSelected(buildTitleNavLabel(vs.scanResult.Titles[titleIdx]))
	})
	nextTitleBtn = ui.MakePillButton("▶", ui.BorderDim, func() {
		if vs.scanResult == nil || titleIdx >= len(vs.scanResult.Titles)-1 || vs.videoTSPath == "" {
			return
		}
		titleIdx++
		titleNavSelect.SetSelected(buildTitleNavLabel(vs.scanResult.Titles[titleIdx]))
	})

	titleNavSelect = widget.NewSelect(nil, func(s string) {
		if vs.scanResult == nil || vs.videoTSPath == "" {
			return
		}
		for i, dt := range vs.scanResult.Titles {
			if buildTitleNavLabel(dt) == s {
				titleIdx = i
				if url := buildDiscConcatURL(vs.videoTSPath, dt.VTSNumber); url != "" {
					go func() { _ = dvdPlayer.Load(url) }()
				}
				return
			}
		}
	})

	titleNavRow := container.NewHBox(
		widget.NewLabel("Title:"),
		titleNavSelect,
		prevTitleBtn,
		nextTitleBtn,
	)
	titleNavRow.Hide()

	playWithMenusBtn := ui.MakePillButton("▶  Play with Menus", opts.ModuleColor, func() {
		if err := launchDVDPlayer(vs.sourcePath); err != nil {
			dialog.ShowError(err, opts.Window)
		}
	})

	playerBottomRow := container.NewVBox(titleNavRow, playWithMenusBtn)
	playerPane := container.NewBorder(nil, playerBottomRow, nil, nil, playerCanvas)

	rebuildTitleNav = func() {
		if vs.scanResult == nil || len(vs.scanResult.Titles) <= 1 || vs.videoTSPath == "" {
			titleNavRow.Hide()
			return
		}
		navOpts := make([]string, len(vs.scanResult.Titles))
		for i, dt := range vs.scanResult.Titles {
			navOpts[i] = buildTitleNavLabel(dt)
		}
		titleNavSelect.SetOptions(navOpts)
		titleIdx = 0
		titleNavSelect.SetSelected(navOpts[0])
		titleNavRow.Show()
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
			for _, dt := range vs.scanResult.Titles {
				if !vs.selectedTitles[dt.Number] {
					continue
				}
				titlePath := fmt.Sprintf("%s_Title_%02d%s", base, dt.Number, ext)
				jobs = append(jobs, titleJob{
					vtsNumber:   dt.VTSNumber,
					titleNumber: dt.Number,
					outputPath:  titlePath,
					jobTitle:    fmt.Sprintf("Rip DVD Title %d: %s", dt.Number, filepath.Base(vs.sourcePath)),
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
	titleEntry.SetPlaceHolder("Disc / movie title (embedded as metadata)")
	titleEntry.SetText(vs.discTitle)
	titleEntry.OnChanged = func(v string) { vs.discTitle = strings.TrimSpace(v) }

	chaptersCheck := widget.NewCheck("Embed chapters", func(v bool) {
		vs.embedChapters = v
		vs.persistConfig()
	})
	chaptersCheck.SetChecked(vs.embedChapters)

	allAudioCheck := widget.NewCheck("All audio tracks", func(v bool) {
		vs.allAudioTracks = v
		vs.persistConfig()
	})
	allAudioCheck.SetChecked(vs.allAudioTracks)

	subsCheck := widget.NewCheck("Include subtitles (DVD bitmap)", func(v bool) {
		vs.includeSubtitles = v
		vs.persistConfig()
	})
	subsCheck.SetChecked(vs.includeSubtitles)

	var fullDiscCheck *widget.Check // assigned below; referenced by ntscSelect callback
	fullDiscCheck = widget.NewCheck("Full disc extraction (DVD-Video with IFO regeneration)", func(v bool) {
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

	ntscSelect := widget.NewSelect([]string{"None", "PAL → NTSC", "NTSC → PAL"}, func(value string) {
		switch value {
		case "PAL → NTSC":
			vs.regionConvert = "pal2ntsc"
		case "NTSC → PAL":
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
	ntscSelect.SetSelected("None")

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

	buildTitleCheckLabel := func(dt DiscTitle) string {
		label := fmt.Sprintf("Title %d — %s, %d chapters", dt.Number, FormatDuration(dt.Duration), dt.NumChapters)
		if len(dt.Audio) > 0 {
			parts := make([]string, 0, len(dt.Audio))
			for _, a := range dt.Audio {
				c := strings.ToUpper(a.Codec)
				if a.Language != "" {
					c += " [" + strings.ToUpper(a.Language) + "]"
				}
				parts = append(parts, c)
			}
			label += " — " + strings.Join(parts, ", ")
		}
		return label
	}

	rebuildEnrich = func() {
		var mainTitle *DiscTitle
		if vs.scanResult != nil && len(vs.scanResult.Titles) > 0 {
			mainTitle = &vs.scanResult.Titles[0]
		}

		// Chapter checkbox
		chapLabel := "Embed chapters"
		if mainTitle != nil {
			if mainTitle.NumChapters > 1 {
				chapLabel = fmt.Sprintf("Embed chapters (%d)", mainTitle.NumChapters)
				chaptersCheck.Enable()
			} else {
				chapLabel = "Embed chapters (none on disc)"
				chaptersCheck.SetChecked(false)
				chaptersCheck.Disable()
			}
		} else {
			chaptersCheck.Enable()
		}
		chaptersCheck.Text = chapLabel
		chaptersCheck.Refresh()

		// Audio checkbox
		audioLabel := "All audio tracks"
		if mainTitle != nil && len(mainTitle.Audio) > 0 {
			if langs := langList(mainTitle.Audio); langs != "" {
				audioLabel = fmt.Sprintf("All audio tracks (%d: %s)", len(mainTitle.Audio), langs)
			} else {
				audioLabel = fmt.Sprintf("All audio tracks (%d)", len(mainTitle.Audio))
			}
		}
		allAudioCheck.Text = audioLabel
		allAudioCheck.Refresh()

		// Subtitle checkbox
		subsLabel := "Include subtitles (DVD bitmap)"
		if vs.format == FormatH264MP4 {
			subsLabel = "Include subtitles (not supported in MP4)"
			subsCheck.SetChecked(false)
			subsCheck.Disable()
		} else if mainTitle != nil {
			if len(mainTitle.Subtitles) == 0 {
				subsLabel = "Include subtitles (none on disc)"
				subsCheck.SetChecked(false)
				subsCheck.Disable()
			} else {
				subsLabel = fmt.Sprintf("Include subtitles (%d streams)", len(mainTitle.Subtitles))
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

		// Disc info label at the top of the view
		var discInfo string
		if vs.scanResult != nil {
			parts := []string{}
			if vs.scanResult.DiscType != "" {
				parts = append(parts, vs.scanResult.DiscType)
			}
			if vs.scanResult.Region != "" {
				parts = append(parts, vs.scanResult.Region)
			}
			if vs.scanResult.TotalSize > 0 {
				parts = append(parts, fmt.Sprintf("%.1f GB", float64(vs.scanResult.TotalSize)/1e9))
			}
			discInfo = strings.Join(parts, " · ")
		}
		if discInfoLabel != nil {
			if discInfo != "" {
				discInfoLabel.SetText("⏺  " + discInfo)
				discInfoLabel.Show()
			} else {
				discInfoLabel.Hide()
			}
		}

			// Rebuild content objects
		objs := []fyne.CanvasObject{
			widget.NewLabelWithStyle("Title", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			titleEntry,
			chaptersCheck,
			allAudioCheck,
			subsCheck,
			widget.NewLabelWithStyle("Region Conversion", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			ntscSelect,
			fullDiscCheck,
		}

		if vs.scanResult != nil && len(vs.scanResult.Titles) > 1 {
			objs = append(objs, widget.NewSeparator())
			objs = append(objs,
				widget.NewLabelWithStyle(
					fmt.Sprintf("Titles on disc (%d) — select to rip:", len(vs.scanResult.Titles)),
					fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
			for _, dt := range vs.scanResult.Titles {
				dt := dt // capture
				ch := widget.NewCheck(buildTitleCheckLabel(dt), func(v bool) {
					vs.selectedTitles[dt.Number] = v
				})
				ch.SetChecked(vs.selectedTitles[dt.Number])
				objs = append(objs, ch)
			}
		}

		enrichContent.Objects = objs
		enrichContent.Refresh()
	}

	// Initial render of enrichment panel (no scan result yet)
	rebuildEnrich()

	discInfoLabel = widget.NewLabel("")
	discInfoLabel.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	discInfoLabel.Hide()

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

				sourceChangedHook(path)
				vs.outputPath = DefaultOutputPath(path, vs.format)
				if opts.SetRipOutputPath != nil {
					opts.SetRipOutputPath(vs.outputPath)
				}
				outputEntry.SetText(vs.outputPath)

				// Reset previous scan state and trigger a new background scan.
				vs.scanResult = nil
				vs.selectedTitles = nil
				rebuildEnrich()

				isISO := strings.HasSuffix(strings.ToLower(path), ".iso")
				if isISO {
					go func() {
						fi, err := os.Stat(path)
						if err != nil {
							logging.Warning(logging.CatDVD, "ISO stat failed: %v", err)
							return
						}
						discType := classifyDiscType(fi.Size())
						udfType, _ := udf.IdentifyDiscFormat(path)
						if udfType == udf.DiscTypeBluRay {
							discType = "BD"
						}

						// Try to read region from VIDEO_TS.IFO within the ISO
						region := ""
						f, openErr := os.Open(path)
						if openErr == nil {
							udfReader := udf.NewReader(f)
							ifoData, readErr := udfReader.ReadFileData("VIDEO_TS/VIDEO_TS.IFO")
							if readErr == nil {
								if mat, matErr := ifo.ReadVMGI(bytes.NewReader(ifoData)); matErr == nil {
									region = classifyDiscRegion(mat.VMG_Category)
								}
							}
							f.Close()
						}

						result := &DiscScanResult{
							DiscType:  discType,
							Region:    region,
							TotalSize: fi.Size(),
						}
						fyne.CurrentApp().Driver().DoFromGoroutine(func() {
							vs.scanResult = result
							vs.selectedTitles = nil
							rebuildEnrich()
						}, false)
					}()
				} else {
					if info, err := os.Stat(filepath.Join(path, "VIDEO_TS.IFO")); err == nil && !info.IsDir() {
						if opts.OnScanDVDStruct != nil {
							opts.OnScanDVDStruct(path)
						}
					}
					go func() {
						vtsp, _, err := ResolveVideoTSPath(path)
						if err != nil {
							return
						}
						result, scanErr := ScanDisc(vtsp)
						fyne.CurrentApp().Driver().DoFromGoroutine(func() {
							if scanErr != nil {
								logging.Warning(logging.CatDVD, "disc scan failed: %v", scanErr)
							} else {
								vs.scanResult = result
								vs.videoTSPath = vtsp
								vs.selectedTitles = make(map[int]bool)
								for _, dt := range result.Titles {
									vs.selectedTitles[dt.Number] = true
								}
								if len(result.Titles) > 0 {
									if url := buildDiscConcatURL(vtsp, result.Titles[0].VTSNumber); url != "" {
										go func() { _ = dvdPlayer.Load(url) }()
									}
								}
								rebuildTitleNav()
							}
							rebuildEnrich()
						}, false)
					}()
				}
			}
		}),
		clearISOBtn,
		widget.NewLabelWithStyle(t.RipFormatLabel, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		formatSelect,
		discInfoLabel,
		enrichContent,
		widget.NewLabelWithStyle(t.LabelOutput, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		outputEntry,
		container.NewHBox(resetBtn, loadCfgBtn, saveCfgBtn),
		widget.NewSeparator(),
		widget.NewLabelWithStyle(t.LabelStatus, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		statusLabel,
		progressBar,
	)

	mainSplit := container.NewHSplit(
		playerPane,
		container.NewVScroll(container.NewPadded(controls)),
	)
	mainSplit.SetOffset(0.40)

	var bottomBar fyne.CanvasObject
	if opts.OnModuleFooter != nil {
		bottomBar = opts.OnModuleFooter(opts.ModuleColor, container.NewHBox(addQueueBtn, layout.NewSpacer(), runNowBtn), opts.OnGetStatsBar())
	}
	return container.NewBorder(topBar, bottomBar, nil, nil,
		container.NewBorder(nil, logSection, nil, nil,
			mainSplit,
		))
}

// buildTitleNavLabel builds the display label for a disc title in the nav dropdown.
func buildTitleNavLabel(dt DiscTitle) string {
	return fmt.Sprintf("T%02d  %s  (%d chap)", dt.Number, FormatDuration(dt.Duration), dt.NumChapters)
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
