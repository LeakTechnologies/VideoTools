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
	sourcePath            string
	outputPath            string
	format                string
	embedChapters         bool
	allAudioTracks        bool
	includeSubtitles      bool
	selectedSubtitleLangs []string // nil = not yet configured (legacy or disc-dependent)
	includeMenus          bool
	regionConvert         string // "" (none), "pal2ntsc", "ntsc2pal"
	extractMode           string // "" (selected scenes), "main" (main feature only), "full" (full disc with IFO regen)
	discTitle             string
	outputTouched         bool // output path was hand-edited, so auto-recompute should not clobber it
	progress              float64

	scanResult     *DiscScanResult
	selectedTitles map[int]bool // title Number → selected
	videoTSPath    string       // resolved VIDEO_TS dir; empty for ISOs / unloaded

	statusLabel *widget.Label
	progressBar *widget.ProgressBar
}

func (vs *viewState) applyConfig(cfg ripConfig) {
	vs.format = cfg.Format
	vs.embedChapters = cfg.EmbedChapters
	vs.allAudioTracks = cfg.AllAudioTracks
	vs.includeSubtitles = cfg.IncludeSubtitles
	vs.selectedSubtitleLangs = cfg.SelectedSubtitleLangs // nil signals "migrate on first disc load"
	vs.includeMenus = cfg.IncludeMenus
}

func (vs *viewState) persistConfig() {
	cfg := ripConfig{
		Format:                vs.format,
		EmbedChapters:         vs.embedChapters,
		AllAudioTracks:        vs.allAudioTracks,
		IncludeSubtitles:      vs.includeSubtitles,
		SelectedSubtitleLangs: vs.selectedSubtitleLangs,
		IncludeMenus:          vs.includeMenus,
	}
	if err := savePersistedRipConfig(cfg); err != nil {
		logging.Debug(logging.CatSystem, "failed to persist rip config: %v", err)
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
	var defaultOutput func() string
	var applyOutputPath func()

	vs := &viewState{
		sourcePath: opts.RipSourcePath,
		outputPath: opts.RipOutputPath,
		format:     opts.RipFormat,
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
		applyOutputPath()
	}

	outputEntry := widget.NewEntry()
	outputEntry.SetPlaceHolder(t.RipOutputPath)
	outputEntry.SetText(vs.outputPath)
	outputEntry.OnChanged = func(val string) {
		vs.outputTouched = true
		vs.outputPath = strings.TrimSpace(val)
		if opts.SetRipOutputPath != nil {
			opts.SetRipOutputPath(vs.outputPath)
		}
	}

	// defaultOutput returns the auto-derived output path: the Title field
	// drives the filename when the user has set one (cleared source paths and
	// empty titles fall back to the source folder's name). Once the user
	// hand-edits the path entry (outputTouched) auto-recompute stops.
	defaultOutput = func() string {
		// Full-disc extraction (region conversion) outputs a VIDEO_TS
		// directory — the Title still drives its name when set.
		if vs.regionConvert != "" && vs.extractMode == "full" {
			if vs.discTitle != "" {
				if p := FullDiscOutputTitlePath(vs.sourcePath, vs.discTitle); p != "" {
					return p
				}
			}
			return FullDiscOutputPath(vs.sourcePath)
		}
		if vs.discTitle != "" {
			if p := DefaultOutputTitlePath(vs.sourcePath, vs.format, vs.discTitle); p != "" {
				return p
			}
		}
		return DefaultOutputPath(vs.sourcePath, vs.format)
	}
	// applyOutputPath re-derives the output path from the current state and
	// marks it auto-managed again (a programmatic recompute cancels a prior
	// manual tweak only when the path is being forcibly regenerated, e.g.
	// source/format changes — mirroring the pre-existing overwrite contract).
	applyOutputPath = func() {
		vs.outputPath = defaultOutput()
		vs.outputTouched = false
		if opts.SetRipOutputPath != nil {
			opts.SetRipOutputPath(vs.outputPath)
		}
		outputEntry.SetText(vs.outputPath)
	}

	formatSelect := widget.NewSelect([]string{FormatLosslessMKV, FormatH264MKV, FormatH264MP4, FormatArchivist}, func(value string) {
		vs.format = value
		applyOutputPath()
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

	ripTeal := color.NRGBA{R: 0x1a, G: 0x93, B: 0x73, A: 0xff}

	ripNavy := utils.MustHex("#191F35")
	// buildRipBox is a thin wrapper over the shared ui.SectionBox, binding the
	// rip module's navy background and teal accent. See internal/ui/components.go.
	buildRipBox := func(title string, content fyne.CanvasObject) fyne.CanvasObject {
		return ui.SectionBox(ripNavy, ripTeal, title, content)
	}

	sectionGap := func() fyne.CanvasObject {
		gap := canvas.NewRectangle(color.Transparent)
		gap.SetMinSize(fyne.NewSize(0, 6))
		return gap
	}

	// subsectionLabel is a compact plain-text label for minor subsections
	// (e.g. OUTPUT PATH) that don't warrant their own coloured SectionBox.
	subsectionLabel := func(text string) fyne.CanvasObject {
		lbl := widget.NewLabelWithStyle(text, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		lbl.Truncation = fyne.TextTruncateEllipsis
		return lbl
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

	// ── Menu Preview (removed dev66→dev67 to free vertical space for the
	// content browser) ──────────────────────────────────────────────────────

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
			vs.outputPath = defaultOutput()
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
			vs.setStatus("Queued full-disc rip job...")
			vs.setProgress(0)
			if runNow && !jq.IsRunning() {
				jq.Start()
			}
			return nil
		}

		// Main-feature extraction is a single job for the longest title on the disc.
		// Without a scan result we fall back to the executor's own main-feature
		// defaults (largest VTS set / title 1).
		if vs.extractMode == "main" {
			vtsNum, titleNum := 0, 0
			if vs.scanResult != nil && len(vs.scanResult.Titles) > 0 {
				main := vs.scanResult.Titles[0]
				for _, dt := range vs.scanResult.Titles {
					if dt.Duration > main.Duration {
						main = dt
					}
				}
				vtsNum, titleNum = main.VTSNumber, main.Number
			}
			job := &queue.Job{
				Type:        queue.JobTypeRip,
				Title:       fmt.Sprintf("%s: %s", t.RipMainFeature, filepath.Base(vs.sourcePath)),
				Description: fmt.Sprintf("Output: %s", utils.ShortenMiddle(filepath.Base(vs.outputPath), 40)),
				InputFile:   vs.sourcePath,
				OutputFile:  vs.outputPath,
				Config: map[string]interface{}{
					"sourcePath":            vs.sourcePath,
					"outputPath":            vs.outputPath,
					"format":                vs.format,
					"embedChapters":         vs.embedChapters,
					"allAudioTracks":        vs.allAudioTracks,
					"includeSubtitles":      vs.includeSubtitles,
					"selectedSubtitleLangs": append([]string{}, vs.selectedSubtitleLangs...),
					"includeMenus":          vs.includeMenus,
					"regionConvert":         vs.regionConvert,
					"discTitle":             vs.discTitle,
					"vtsNumber":             vtsNum,
					"titleNumber":           titleNum,
					"extractMode":           "main",
				},
			}
			opts.AddJob(job)
			vs.setStatus(t.RipJobQueuedMsg)
			vs.setProgress(0)
			if runNow && !jq.IsRunning() {
				jq.Start()
			}
			return nil
		}

		// Build list of (vtsNumber, outputPath, title) for each job to enqueue.
		type titleJob struct {
			vtsNumber   int
			titleNumber int
			outputPath  string
			jobTitle    string
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
					"sourcePath":            vs.sourcePath,
					"outputPath":            j.outputPath,
					"format":                vs.format,
					"embedChapters":         vs.embedChapters,
					"allAudioTracks":        vs.allAudioTracks,
					"includeSubtitles":      vs.includeSubtitles,
					"selectedSubtitleLangs": append([]string{}, vs.selectedSubtitleLangs...),
					"includeMenus":          vs.includeMenus,
					"regionConvert":         vs.regionConvert,
					"discTitle":             vs.discTitle,
					"vtsNumber":             j.vtsNumber,
					"titleNumber":           j.titleNumber,
				},
			}
			opts.AddJob(job)
		}

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
		if vs.extractMode == "main" {
			main := vs.scanResult.Titles[0]
			for _, dt := range vs.scanResult.Titles {
				if dt.Duration > main.Duration {
					main = dt
				}
			}
			ripSummaryLbl.SetText(fmt.Sprintf(t.RipReadyMainFeatureFmt,
				fmt.Sprintf("%s %02d · %s", t.RipTitleShort, main.Number, FormatDuration(main.Duration))))
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
	// Single-line readiness line: it truncates rather than wraps. A wrapping
	// label inside an HBox collapses to its narrowest word-boundary width and
	// re-measures multi-line tall, which previously inflated the whole action
	// bar into a giant band (buttons stretched to match).
	ripSummaryLbl.Truncation = fyne.TextTruncateEllipsis
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
		applyOutputPath()
		applyControls()
	})

	saveCfgBtn := ui.MakePillButton(t.ActionSaveConfig, ui.BorderDim, func() {
		vs.persistConfig()
		dialog.ShowInformation(t.RipConfigSavedTitle, fmt.Sprintf(t.RipConfigSavedFmt, configpath.ModuleConfigPath("rip")), opts.Window)
	})

	resetBtn := ui.MakePillButton(t.ActionReset, ui.BorderDim, func() {
		cfg := defaultRipConfig()
		vs.applyConfig(cfg)
		applyOutputPath()
		applyControls()
		vs.persistConfig()
	})

	clearISOBtn := ui.MakePillButton(t.RipClearISO, ui.BorderDim, func() {
		vs.sourcePath = ""
		vs.outputPath = ""
		vs.videoTSPath = ""
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
	titleEntry.OnChanged = func(v string) {
		vs.discTitle = strings.TrimSpace(v)
		// The output follows the Title once set, unless the user has hand-edited
		// the path (full-disc/region outputs get the title-based folder name too).
		if !vs.outputTouched {
			applyOutputPath()
		}
	}

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

	// Subtitles render as one checkbox per distinct language on the main
	// title (rebuilt by rebuildEnrich after each scan). Checking a language
	// adds it to the rip selection; the master includeSubtitles flag is
	// derived from the selection so legacy configs keep working. The header
	// flips to the MP4 note when that format can't carry bitmap subs.
	var subLangChecks []*widget.Check
	subsHeader := widget.NewLabelWithStyle(t.RipIncludeSubtitles, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	buildSubtitleRow := func(mt *DiscTitle) []fyne.CanvasObject {
		if vs.format == FormatH264MP4 {
			subsHeader.SetText(t.RipIncludeSubtitlesMP4)
			subLangChecks = nil
			return []fyne.CanvasObject{subsHeader}
		}

		var avail []string
		if mt != nil {
			avail = uniqueSubtitleLangs(mt.Subtitles)
		}

		// A legacy config that only persisted includeSubtitles=true migrates to
		// "select every language" on the first scan (nil + false stays off).
		sel := vs.selectedSubtitleLangs
		if sel == nil && vs.includeSubtitles {
			sel = avail
			vs.selectedSubtitleLangs = avail
		}
		if len(avail) == 0 {
			subLangChecks = nil
			if vs.scanResult == nil {
				subsHeader.SetText(t.RipIncludeSubtitles)
			} else {
				subsHeader.SetText(t.RipIncludeSubtitlesNone)
			}
			return []fyne.CanvasObject{subsHeader}
		}

		subsHeader.SetText(t.RipIncludeSubtitles)
		selSet := map[string]bool{}
		for _, l := range sel {
			selSet[l] = true
		}

		subLangChecks = nil
		objects := []fyne.CanvasObject{subsHeader}

		// Select / Deselect all for the whole language list. Bulk-updating every
		// check directly would fire each per-check OnChanged and duplicate the
		// selection entries, so the slice is set outright and check states are
		// applied with callbacks suppressed (restored afterwards).
		if len(avail) > 1 {
			setAllLanguages := func(selected bool) {
				if selected {
					vs.selectedSubtitleLangs = append([]string{}, avail...)
				} else {
					vs.selectedSubtitleLangs = nil
				}
				vs.includeSubtitles = selected
				vs.persistConfig()
				for _, ck := range subLangChecks {
					cb := ck.OnChanged
					ck.OnChanged = nil
					ck.SetChecked(selected)
					ck.OnChanged = cb
				}
			}
			selectAllBtn := widget.NewButton(t.RipSelectAll, func() { setAllLanguages(true) })
			selectAllBtn.Importance = widget.LowImportance
			deselectAllBtn := widget.NewButton(t.RipDeselectAll, func() { setAllLanguages(false) })
			deselectAllBtn.Importance = widget.LowImportance
			objects = append(objects, container.NewHBox(selectAllBtn, deselectAllBtn))
		}

		for _, lang := range avail {
			lang := lang
			ck := widget.NewCheck(lang, nil)
			ck.SetChecked(selSet[lang])
			ck.OnChanged = func(v bool) {
				if v {
					vs.selectedSubtitleLangs = append(vs.selectedSubtitleLangs, lang)
				} else {
					keep := vs.selectedSubtitleLangs[:0]
					for _, l := range vs.selectedSubtitleLangs {
						if l != lang {
							keep = append(keep, l)
						}
					}
					vs.selectedSubtitleLangs = keep
				}
				vs.includeSubtitles = len(vs.selectedSubtitleLangs) > 0
				vs.persistConfig()
			}
			subLangChecks = append(subLangChecks, ck)
			objects = append(objects, ck)
		}
		return objects
	}

	// Rip mode: the single main feature (longest title) or selected scenes
	// (title by title). Full-disc extraction is forced by region conversion, so
	// the radio only reflects the scenes vs main-feature choice. Full Movie is
	// the first and default option. Constructed with a nil OnChanged so the
	// initial SetSelected below doesn't fire the callback; the default-state
	// extractMode is set explicitly alongside the selection.
	var modeRadio *widget.RadioGroup
	modeRadio = widget.NewRadioGroup([]string{t.RipModeMainFeature, t.RipModeScenes}, nil)
	modeRadio.Horizontal = false
	modeRadio.SetSelected(t.RipModeMainFeature)
	vs.extractMode = "main"
	modeRadio.OnChanged = func(value string) {
		// Region conversion forces full-disc extraction — the scenes /
		// main-feature choice is inert for as long as it is active.
		if vs.regionConvert != "" {
			vs.extractMode = "full"
			return
		}
		switch value {
		case t.RipModeMainFeature:
			vs.extractMode = "main"
		default:
			if value == "" {
				modeRadio.SetSelected(t.RipModeMainFeature)
			}
			vs.extractMode = ""
		}
	}

	menusCheck := widget.NewCheck(t.RipPreserveMenusFull, func(v bool) {
		vs.includeMenus = v
		vs.persistConfig()
	})
	menusCheck.SetChecked(vs.includeMenus)

	var fullDiscCheck *widget.Check // assigned below; referenced by ntscSelect callback
	fullDiscCheck = widget.NewCheck(t.RipFullDiscExtraction, func(v bool) {
		if v && vs.regionConvert != "" {
			vs.extractMode = "full"
		} else {
			if modeRadio.Selected == t.RipModeMainFeature {
				vs.extractMode = "main"
			} else {
				vs.extractMode = ""
			}
		}
		applyOutputPath()
	})
	fullDiscCheck.SetChecked(false)
	fullDiscCheck.Disable()

	// Advanced options live under a collapsible header (same fold visual as
	// the Filters/Upscale metadata panels) so the day-to-day output path stays
	// clean: menus preservation, PAL↔NTSC conversion, and full-disc extraction.
	// The header and body are persistent widgets — rebuildEnrich only swaps the
	// body children, so the fold stays closed across re-scans instead of the
	// old accordion cycling/overlapping the options above it.
	advancedBody := container.NewVBox()
	advancedHdr, advancedUpdate := ui.BuildCollapsibleHeader(t.RipAdvancedOptions, ripTeal, func(open bool) {
		if open {
			advancedBody.Show()
		} else {
			advancedBody.Hide()
		}
	})
	advancedBody.Hide()
	advancedUpdate(false) // start collapsed so the arrow matches the hidden body

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
			modeRadio.Hide()
		} else {
			if modeRadio.Selected == t.RipModeMainFeature {
				vs.extractMode = "main"
			} else {
				vs.extractMode = ""
			}
			fullDiscCheck.SetChecked(false)
			modeRadio.Show()
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

		// Region conversion dropdown — only shown on H.264 re-encode formats.
		if vs.format == FormatLosslessMKV || vs.format == FormatArchivist {
			ntscSelect.Hide()
			fullDiscCheck.Hide()
		} else {
			ntscSelect.Show()
			// Mode selector is hidden while region conversion forces full-disc
			// extraction (neither scenes nor main-feature applies).
			if vs.regionConvert != "" {
				modeRadio.Hide()
			} else {
				modeRadio.Show()
			}
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
		}
		objs = append(objs, buildSubtitleRow(mainTitle)...)
		objs = append(objs, modeRadio)

		if vs.scanResult != nil && len(vs.scanResult.Titles) > 1 {
			objs = append(objs, widget.NewSeparator())
			objs = append(objs,
				widget.NewLabelWithStyle(
					fmt.Sprintf(t.RipTitlesOnDiscFmt, len(vs.scanResult.Titles)),
					fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		}

		// Uncommon options live in the collapsed Advanced fold (declared above);
		// here we only swap the body children. Menus preservation, PAL↔NTSC
		// conversion, and full-disc extraction.
		advanced := container.NewVBox(
			menusCheck,
			widget.NewSeparator(),
			widget.NewLabelWithStyle(t.RipRegionConversion, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			ntscSelect,
			fullDiscCheck,
		)
		advancedBody.Objects = advanced.Objects
		advancedBody.Refresh()
		objs = append(objs, widget.NewSeparator())
		objs = append(objs, advancedHdr, advancedBody)

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

		vs.sourcePath = path
		sourceEntry.SetText(path)
		if opts.SetRipSourcePath != nil {
			opts.SetRipSourcePath(path)
		}
		sourceChangedHook(path)
		applyOutputPath()

		vs.scanResult = nil
		vs.selectedTitles = nil
		rebuildEnrich()

		// Set the scanning state AFTER the enrich rebuild — rebuildEnrich()
		// calls updateDiscInfo() which renders SetEmpty for a nil scan result,
		// so SetScanning() above would be clobbered immediately.
		if discSummary != nil {
			discSummary.SetScanning()
		}

		// pushDiscNote appends a scan-as-you-go fact line to the disc summary.
		// Notes arrive from the scan goroutine, so marshal onto the UI thread.
		pushDiscNote := func(note string) {
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				if discSummary != nil {
					discSummary.SetSnippet(note)
				}
			}, false)
		}

		if strings.HasSuffix(strings.ToLower(path), ".iso") {
			go func() {
				result, scanErr := runISOScan(path, pushDiscNote)
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
				result, scanErr := ScanDisc(vtsp, pushDiscNote)
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

	loadDiscBtn := ui.MakePillButton(t.RipLoadDisc, opts.ModuleColor, func() {
		if opts.OnLoadDisc == nil {
			return
		}
		discPath, err := opts.OnLoadDisc()
		if err != nil {
			dialog.ShowError(err, opts.Window)
			return
		}
		if discPath != "" {
			loadDisc(discPath)
		}
	})

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

	// ── Two-column workspace ────────────────────────────────────────────────
	// Source spans the full width; below it a 55/45 HSplit divides CONTENT
	// (disc summary + title list) from PROCESSING (format, output path,
	// monitoring). The action bar spans the full width. SOURCE is the input to
	// the whole operation; ACTION is the output of the whole operation — both
	// belong outside the two semantic columns.

	sourceBox := buildRipBox(t.RipSource, container.NewVBox(
		container.NewBorder(nil, nil, nil,
			container.NewHBox(loadDiscBtn, browseBtn, clearISOBtn),
			ui.NewDroppable(sourceEntry, func(items []fyne.URI) {
				if opts.OnDropFirstLocal != nil {
					loadDisc(opts.OnDropFirstLocal(items))
				}
			}),
		),
	))

	// LEFT = CONTENT. Disc summary pinned to the top and the title list as the
	// flexible centre that absorbs the available height. The list scrolls
	// internally; the summary keeps its natural (bounded) height.
	leftColumn := container.NewBorder(
		discSummary.GetContainer(),
		nil, nil, nil,
		contentBrowser.GetContainer(),
	)

	// RIGHT = PROCESSING / OUTPUT. Format + enrichment + output path +
	// monitoring, scrollable when the window is short.
	rightColumn := container.NewVScroll(container.NewPadded(
		container.NewVBox(
			buildRipBox(t.RipFormatLabel, container.NewVBox(
				formatSelect,
				enrichContent,
			)),
			sectionGap(),
			// OUTPUT PATH is a plain subsection (not a major coloured section):
			// a small bold label above the path entry, rather than a full
			// SectionBox header.
			container.NewVBox(
				subsectionLabel(t.LabelOutput),
				outputEntry,
				container.NewHBox(resetBtn, loadCfgBtn, saveCfgBtn),
			),
			sectionGap(),
			// Monitoring: rip status text + progress. The readiness line and
			// action buttons live in the full-width action bar below.
			buildRipBox(t.LabelStatus, container.NewVBox(
				statusLabel,
				progressBar,
			)),
		),
	))

	twoColumn := container.NewHSplit(leftColumn, rightColumn)
	twoColumn.SetOffset(0.55)

	// Full-width action bar: readiness line left, primary actions right.
	// The buttons live in a Border edge so they keep their natural height —
	// HBox children stretch to the container's height, which is how the
	// buttons previously turned into tall boxes. The label is the centre:
	// it absorbs the leftover width and truncates instead of wrapping.
	actionBar := container.NewBorder(nil, nil, nil,
		container.NewHBox(openInPlayerBtn, runNowBtn, addQueueBtn),
		ripSummaryLbl,
	)

	mainArea := container.NewBorder(
		sourceBox,
		actionBar,
		nil, nil,
		twoColumn,
	)

	var bottomBar fyne.CanvasObject
	if opts.OnModuleFooter != nil {
		bottomBar = opts.OnModuleFooter(opts.ModuleColor, nil, opts.OnGetStatsBar())
	}

	// Re-scan a previously selected source on re-entry. buildRipView creates a
	// fresh viewState (scanResult always nil), so without this a path restored
	// into the source field would show "No disc loaded" until the user browsed
	// again. loadDisc is idempotent for an empty/restored path and skips when
	// the entry is blank.
	if vs.sourcePath != "" {
		loadDisc(vs.sourcePath)
	}

	return container.NewBorder(topBar, bottomBar, nil, nil,
		mainArea,
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
