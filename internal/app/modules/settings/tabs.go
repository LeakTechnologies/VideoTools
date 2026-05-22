package settings

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/app/appcfg"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/benchmark"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/ui"
	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/utils"
)

// settingsRow places a label immediately before a control with no spacer between
// them. The control renders at its natural (MinSize) width. The centeredPanel in
// view.go constrains total width so labels and controls stay close together.
func settingsRow(label string, control fyne.CanvasObject) fyne.CanvasObject {
	return container.NewHBox(widget.NewLabel(label), control)
}

// settingsCard wraps a group of controls in a titled dark-background section card.
func settingsCard(title string, items ...fyne.CanvasObject) fyne.CanvasObject {
	hdr := widget.NewLabel(title)
	hdr.TextStyle = fyne.TextStyle{Bold: true}
	bg := canvas.NewRectangle(utils.MustHex("#171C2A"))
	bg.CornerRadius = 6
	parts := make([]fyne.CanvasObject, 0, 2+len(items))
	parts = append(parts, hdr, widget.NewSeparator())
	parts = append(parts, items...)
	return container.NewPadded(container.NewMax(bg, container.NewPadded(container.NewVBox(parts...))))
}

// hint returns a standard italic, word-wrapped hint label.
func hint(text string) *widget.Label {
	lbl := widget.NewLabel(text)
	lbl.TextStyle = fyne.TextStyle{Italic: true}
	lbl.Wrapping = fyne.TextWrapWord
	return lbl
}

func BuildBenchmarkTab(cb BenchmarkCallbacks) fyne.CanvasObject {
	t := i18n.T()
	content := container.NewVBox()

	header := widget.NewLabel(t.BenchmarkTitle)
	header.TextStyle = fyne.TextStyle{Bold: true}
	content.Add(header)

	desc := widget.NewLabel(t.BenchmarkDesc)
	desc.Wrapping = fyne.TextWrapWord
	content.Add(desc)

	content.Add(widget.NewSeparator())

	runBtn := ui.MakePillButton(t.BenchmarkRunButton, utils.MustHex(ModuleColor), func() {
		cb.ShowBenchmark()
	})
	content.Add(container.NewCenter(runBtn))

	cfg, err := appcfg.LoadBenchmarkConfig()
	if err == nil && len(cfg.History) > 0 {
		content.Add(widget.NewSeparator())

		recentHeader := widget.NewLabel(t.BenchmarkRecent)
		recentHeader.TextStyle = fyne.TextStyle{Bold: true}
		content.Add(recentHeader)

		limit := 3
		if len(cfg.History) < limit {
			limit = len(cfg.History)
		}
		for i := 0; i < limit; i++ {
			run := cfg.History[i]
			ts := run.Timestamp.Format("Jan 2, 2006 at 3:04 PM")
			lbl := widget.NewLabel(fmt.Sprintf("%s — Recommended: %s", ts, benchmark.HWAccelLabel(run.RecommendedHWAccel)))
			lbl.TextStyle = fyne.TextStyle{Italic: true}
			content.Add(lbl)
		}
	}

	return content
}

func BuildPreferencesTab(cb PreferencesCallbacks) fyne.CanvasObject {
	t := i18n.T()
	settingsColor := utils.MustHex(ModuleColor)
	prefs := cb.PrefsConfig()

	// ── Updates ───────────────────────────────────────────────────────────────
	versionValLabel := widget.NewLabel(cb.FullVersion())
	versionValLabel.TextStyle = fyne.TextStyle{Bold: true}

	hashDisplay := cb.BuildCommit()
	if hashDisplay == "" || hashDisplay == "dev" {
		hashDisplay = "development build"
	}
	hashValLabel := widget.NewLabel(hashDisplay)
	hashValLabel.TextStyle = fyne.TextStyle{Monospace: true}

	updateStatusIcon := widget.NewIcon(nil)
	updateStatusIcon.Hide()
	updateStatusLabel := widget.NewLabel("")
	updateStatusLabel.TextStyle = fyne.TextStyle{Italic: true}

	installBtn := ui.MakePillButton(t.UpdateInstall, settingsColor, nil)
	installBtn.Hide()

	onUpdateAvailable := func(tag string) {
		if tag == "" {
			installBtn.Hide()
			return
		}
		installBtn.OnTapped = func() { cb.ApplyUpdate(tag) }
		installBtn.Show()
	}

	checkBtn := ui.MakePillButton(t.UpdateCheckButton, settingsColor, func() {
		installBtn.Hide()
		cb.CheckForUpdatesWithStatus(updateStatusIcon, updateStatusLabel, onUpdateAvailable)
	})

	if !cb.UpdateLastChecked().IsZero() {
		cb.ApplyUpdateStatusToUI(updateStatusIcon, updateStatusLabel, onUpdateAvailable)
	} else {
		cb.CheckForUpdatesWithStatus(updateStatusIcon, updateStatusLabel, onUpdateAvailable)
	}

	autoCheckKeys := []string{
		"disabled", "every_hour", "every_2h", "every_3h", "every_4h",
		"every_6h", "every_12h", "daily", "semi_weekly", "weekly",
		"bi_weekly", "monthly", "bi_monthly",
	}
	autoCheckOptions := []string{
		t.UpdateDisabled, t.UpdateEveryHour, t.UpdateEvery2Hours, t.UpdateEvery3Hours,
		t.UpdateEvery4Hours, t.UpdateEvery6Hours, t.UpdateEvery12Hours, t.UpdateDaily,
		t.UpdateSemiWeekly, t.UpdateWeekly, t.UpdateBiWeekly, t.UpdateMonthly, t.UpdateBiMonthly,
	}
	legacyEnglishToKey := map[string]string{
		"Disabled":                    "disabled",
		"Every hour":                  "every_hour",
		"Every 2 hours":               "every_2h",
		"Every 3 hours":               "every_3h",
		"Every 4 hours":               "every_4h",
		"Every 6 hours":               "every_6h",
		"Every 12 hours":              "every_12h",
		"Daily":                       "daily",
		"Semi-weekly (every 3 days)":  "semi_weekly",
		"Weekly":                      "weekly",
		"Bi-weekly (every 2 weeks)":   "bi_weekly",
		"Monthly":                     "monthly",
		"Bi-monthly (every 2 months)": "bi_monthly",
	}
	normalizeKey := func(saved string) string {
		if saved == "" {
			return "daily"
		}
		for _, k := range autoCheckKeys {
			if k == saved {
				return saved
			}
		}
		if canonical, ok := legacyEnglishToKey[saved]; ok {
			prefs.AutoCheckFrequency = canonical
			cb.SavePrefsConfig()
			return canonical
		}
		return "daily"
	}
	keyToLabel := func(key string) string {
		for i, k := range autoCheckKeys {
			if k == key && i < len(autoCheckOptions) {
				return autoCheckOptions[i]
			}
		}
		return t.UpdateDaily
	}
	autoCheckSelect := widget.NewSelect(autoCheckOptions, func(selected string) {
		for i, opt := range autoCheckOptions {
			if opt == selected && i < len(autoCheckKeys) {
				prefs.AutoCheckFrequency = autoCheckKeys[i]
				break
			}
		}
		cb.SavePrefsConfig()
	})
	autoCheckSelect.SetSelected(keyToLabel(normalizeKey(prefs.AutoCheckFrequency)))

	updatesCard := settingsCard(t.SettingsTabUpdates,
		settingsRow(t.UpdateCurrentVersion, versionValLabel),
		settingsRow(t.UpdateVersionHash, hashValLabel),
		container.NewHBox(updateStatusIcon, updateStatusLabel),
		container.NewHBox(checkBtn, installBtn),
		settingsRow(t.UpdateAutoCheck, autoCheckSelect),
		hint(t.SettingsUpdatesAutoInfo),
	)

	// ── Language ──────────────────────────────────────────────────────────────
	scriptLabel := widget.NewLabel(t.SettingsLanguageScript)
	scriptLabel.Hide()
	scriptSelect := widget.NewSelect([]string{}, func(selected string) {})
	scriptSelect.Hide()

	if i18n.CurrentCode() == "iu" {
		scriptLabel.Show()
		scriptSelect.Show()
		currentScript := i18n.CurrentScript()
		scriptOptSyllabics := t.SettingsScriptSyllabics
		scriptOptLatin := t.SettingsScriptLatin
		scriptSelect.Options = []string{scriptOptSyllabics, scriptOptLatin}
		if currentScript == i18n.ScriptLatin {
			scriptSelect.SetSelected(scriptOptLatin)
		} else {
			scriptSelect.SetSelected(scriptOptSyllabics)
		}
		scriptSelect.OnChanged = func(selected string) {
			var script i18n.ScriptVariant
			if selected == scriptOptLatin {
				script = i18n.ScriptLatin
			} else {
				script = i18n.ScriptSyllabics
			}
			i18n.SetLanguageWithScript("iu", script)
			cb.PersistLocale("iu", script)
			cb.ShowSettingsView()
		}
	}

	langSelect := buildFlagLangSelect(i18n.All(), i18n.CurrentFont(), i18n.CurrentCode(), cb.Window(), func(code string) {
		if code == i18n.CurrentCode() {
			return
		}
		i18n.SetLanguage(code)
		cb.PersistLocale(code, i18n.CurrentScript())
		cb.ShowSettingsView()
	})

	langCard := settingsCard(t.SettingsLanguage,
		langSelect,
		container.NewHBox(scriptLabel, scriptSelect),
	)

	// ── Appearance ────────────────────────────────────────────────────────────
	fontSizeSelect := widget.NewSelect([]string{"large", "small"}, func(selected string) {
		cb.SetFontSize(selected)
	})
	fontSizeSelect.SetSelected(cb.FontSize())

	fontOptions := []string{t.SettingsFontIBM, t.SettingsFontVCR}
	currentFont := prefs.PlayerFont
	if currentFont == "" {
		currentFont = "ibm"
	}
	fontSelect := widget.NewSelect(fontOptions, func(selected string) {
		if selected == t.SettingsFontVCR {
			cb.SetPlayerFont("vcr")
		} else {
			cb.SetPlayerFont("ibm")
		}
	})
	if currentFont == "vcr" {
		fontSelect.SetSelected(t.SettingsFontVCR)
	} else {
		fontSelect.SetSelected(t.SettingsFontIBM)
	}

	showTooltipsCheck := widget.NewCheck(t.SettingsShowTooltips, func(enabled bool) {
		cb.SetShowTooltips(enabled)
	})
	showTooltipsCheck.Checked = prefs.ShowTooltips

	testPatternBtn := ui.MakePillButton(t.SettingsTestPattern, ui.BorderDim, cb.ShowPlayer())

	aspectOptions := []string{
		t.SettingsPlayerAspect4x3,
		t.SettingsPlayerAspect16x9,
		t.SettingsPlayerAspect5x3,
		t.SettingsPlayerAspect21x9,
		t.SettingsPlayerAspect9x16,
	}
	aspectValues := []string{"4:3", "16:9", "5:3", "21:9", "9:16"}
	currentAspect := cb.PlayerDefaultAspect()
	aspectSelect := widget.NewSelect(aspectOptions, func(selected string) {
		for i, opt := range aspectOptions {
			if opt == selected {
				cb.SetPlayerDefaultAspect(aspectValues[i])
				return
			}
		}
	})
	for i, val := range aspectValues {
		if val == currentAspect {
			aspectSelect.SetSelected(aspectOptions[i])
			break
		}
	}

	appearanceCard := settingsCard("Appearance",
		settingsRow("Font Size", fontSizeSelect),
		settingsRow(t.SettingsFont, fontSelect),
		hint(t.SettingsFontHint),
		showTooltipsCheck,
		container.NewHBox(testPatternBtn),
		hint(t.SettingsTestPatternHint),
		settingsRow(t.SettingsPlayerAspect, aspectSelect),
		hint(t.SettingsPlayerAspectHint),
	)

	// ── Hardware ──────────────────────────────────────────────────────────────
	hwSelect := widget.NewSelect([]string{"auto", "none", "nvenc", "qsv", "amf", "vaapi"}, func(selected string) {
		cb.SetConvertHardwareAccel(selected)
		cb.PersistConvertConfig()
	})
	hwSelect.SetSelected(cb.ConvertHardwareAccel())

	hwStatus := widget.NewLabel("Press Detect to scan your hardware.")
	hwStatus.TextStyle = fyne.TextStyle{Monospace: true}
	hwStatus.Wrapping = fyne.TextWrapWord

	var detectBtn *ui.PillButton
	detectBtn = ui.MakePillButton(t.SettingsDetect, settingsColor, func() {
		detectBtn.SetText("Detecting...")
		detectBtn.Disable()
		go func() {
			best, status := cb.DetectHardwareAccelStatus()
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				hwSelect.SetSelected(best)
				cb.SetConvertHardwareAccel(best)
				cb.PersistConvertConfig()
				hwStatus.SetText(status)
				detectBtn.SetText(t.SettingsDetect)
				detectBtn.Enable()
			}, false)
		}()
	})

	autoBtn := ui.MakePillButton(t.SettingsUseAuto, settingsColor, func() {
		hwSelect.SetSelected("auto")
		cb.SetConvertHardwareAccel("auto")
		cb.PersistConvertConfig()
		hwStatus.SetText("Set to auto — best available encoder selected at encode time.")
	})

	hwDecodeHeader := widget.NewLabel(t.SettingsHWDecode)
	hwDecodeHeader.TextStyle = fyne.TextStyle{Bold: true}

	hwDecodeStatus := widget.NewLabel(t.SettingsHWDecodeDetecting)
	hwDecodeStatus.TextStyle = fyne.TextStyle{Italic: true}

	hwDecodeAutoCheck := widget.NewCheck(t.SettingsHWDecodeAuto, func(enabled bool) {
		cb.SetHWDecodeEnabled(enabled)
	})

	go func() {
		available := appcfg.DetectHWDeviceType() != 0
		var label string
		if available {
			label = t.SettingsHWDecodeAvailable
		} else {
			label = t.SettingsHWDecodeNotCompatible
		}
		fyne.CurrentApp().Driver().DoFromGoroutine(func() {
			hwDecodeStatus.SetText(label)
			if prefs.HWDecodeEnabled && available {
				cb.SetHWDecodeEnabled(true)
			}
			hwDecodeAutoCheck.Checked = prefs.HWDecodeEnabled && available
			hwDecodeAutoCheck.Disable()
		}, false)
	}()

	hwCard := settingsCard(t.SettingsMasterSettings,
		settingsRow(t.SettingsHardwareAccel, hwSelect),
		container.NewHBox(detectBtn, autoBtn),
		hwStatus,
		widget.NewSeparator(),
		hwDecodeHeader,
		hwDecodeStatus,
		hwDecodeAutoCheck,
		hint(t.SettingsHWDecodeAutoHint),
	)

	// ── Module Visibility ─────────────────────────────────────────────────────
	showUpscale := widget.NewCheck(t.SettingsShowUpscale, func(checked bool) {
		cb.SetConvertShowUpscale(checked)
		cb.PersistConvertConfig()
	})
	showUpscale.SetChecked(cb.ConvertShowUpscale())

	showDisc := widget.NewCheck(t.SettingsShowDisc, func(checked bool) {
		cb.SetConvertShowDisc(checked)
		cb.PersistConvertConfig()
	})
	showDisc.SetChecked(cb.ConvertShowDisc())

	modulesCard := settingsCard(t.SettingsModuleVisibility,
		showUpscale,
		showDisc,
		hint(t.SettingsModuleVisibilityHint),
	)

	// ── Queue Behaviour ───────────────────────────────────────────────────────
	currentBehavior := prefs.QueuePlayBehavior
	if currentBehavior == "" {
		currentBehavior = "player"
	}
	queuePlayOpts := []string{t.SettingsQueuePlaySystem, t.SettingsQueuePlayInspect}
	selectedOpt := queuePlayOpts[0]
	if currentBehavior == "inspect" {
		selectedOpt = queuePlayOpts[1]
	}
	queuePlayRadio := widget.NewRadioGroup(queuePlayOpts, func(selected string) {
		if selected == t.SettingsQueuePlayInspect {
			prefs.QueuePlayBehavior = "inspect"
		} else {
			prefs.QueuePlayBehavior = "player"
		}
		cb.SavePrefsConfig()
	})
	queuePlayRadio.SetSelected(selectedOpt)

	queueCard := settingsCard(t.SettingsQueueSection,
		widget.NewLabel(t.SettingsQueuePlayLabel),
		queuePlayRadio,
		hint(t.SettingsQueuePlayHint),
	)

	// ── Pipeline ──────────────────────────────────────────────────────────────
	keepIntermediateCheck := widget.NewCheck(t.SettingsPipelineKeepIntermediate, func(checked bool) {
		prefs.PipelineKeepIntermediate = checked
		cb.SavePrefsConfig()
	})
	keepIntermediateCheck.SetChecked(prefs.PipelineKeepIntermediate)

	pipelineCard := settingsCard(t.SettingsPipelineSection,
		keepIntermediateCheck,
		hint(t.SettingsPipelineKeepIntermediateHint),
	)

	// ── Output Directory ──────────────────────────────────────────────────────
	defaultPath := "~/Videos/VideoTools"
	if runtime.GOOS == "windows" {
		homeDir := os.Getenv("USERPROFILE")
		if homeDir != "" {
			defaultPath = filepath.Join(homeDir, "Videos", "VideoTools")
		}
	}

	outputDirEntry := widget.NewEntry()
	outputDirEntry.SetPlaceHolder(defaultPath)
	currentDir := cb.DefaultOutputDir()
	if currentDir == "" {
		currentDir = defaultPath
	}
	outputDirEntry.SetText(currentDir)
	outputDirEntry.OnChanged = func(val string) {
		cb.SetDefaultOutputDir(strings.TrimSpace(val))
	}

	outputBrowseBtn := ui.MakePillButton(t.ActionBrowse, settingsColor, func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			path := uri.Path()
			outputDirEntry.SetText(path)
			cb.SetDefaultOutputDir(path)
		}, cb.Window())
	})

	outputClearBtn := ui.MakePillButton(t.ConvertUseDefault, settingsColor, func() {
		outputDirEntry.SetText("")
		cb.SetDefaultOutputDir("")
	})

	outputCard := settingsCard(t.SettingsDefaultOutputDir,
		container.NewBorder(nil, nil, nil,
			container.NewHBox(outputBrowseBtn, outputClearBtn),
			outputDirEntry,
		),
		hint(t.SettingsDefaultOutputDirHint),
	)

	// ── Developer Tools ───────────────────────────────────────────────────────
	verboseDiscCheck := widget.NewCheck("Enable verbose disc logging", func(checked bool) {
		cb.SetVerboseDiscLogging(checked)
	})
	verboseDiscCheck.SetChecked(cb.VerboseDiscLogging())

	developerCard := settingsCard("Developer Tools",
		verboseDiscCheck,
		hint("Logs detailed sector layout, IFO table offsets, SPU DCSQ parameters, and NAV_PCK button geometry to videotools.log. Enable when diagnosing disc authoring or menu issues."),
	)

	// ── Log File ──────────────────────────────────────────────────────────────
	logPathLabel := widget.NewLabel(cb.LogFilePath())
	logPathLabel.Wrapping = fyne.TextWrapBreak

	openLogBtn := ui.MakePillButton("Open Log Folder", settingsColor, func() {
		cb.OpenLogFolder()
	})

	clearLogBtn := ui.MakePillButton("Clear Log File", settingsColor, func() {
		dialog.ShowConfirm("Clear Log File", "This will erase the entire log file. Continue?", func(confirmed bool) {
			if !confirmed {
				return
			}
			if err := cb.ClearLogFile(); err != nil {
				dialog.ShowError(err, cb.Window())
				return
			}
			dialog.ShowInformation("Log Cleared", "The log file has been cleared.", cb.Window())
		}, cb.Window())
	})

	logCard := settingsCard("Log File",
		logPathLabel,
		container.NewHBox(openLogBtn, clearLogBtn),
		hint("Clear removes all previous sessions from the log file. The file stays open — no restart needed."),
	)

	return container.NewVBox(
		updatesCard,
		langCard,
		appearanceCard,
		hwCard,
		modulesCard,
		queueCard,
		pipelineCard,
		outputCard,
		developerCard,
		logCard,
	)
}

func BuildDependenciesTab(cb DependencyCallbacks) fyne.CanvasObject {
	content := container.NewVBox()
	t := i18n.T()

	header := widget.NewLabel(t.DependenciesTitle)
	header.TextStyle = fyne.TextStyle{Bold: true}
	content.Add(header)

	desc := widget.NewLabel(t.DependenciesDesc)
	desc.Wrapping = fyne.TextWrapWord
	content.Add(desc)

	content.Add(widget.NewSeparator())

	allDeps := cb.AllDependencies()
	depNames := make([]string, 0, len(allDeps))
	for depName, dep := range allDeps {
		if !cb.IsDependencyAvailableForPlatform(dep) {
			continue
		}
		depNames = append(depNames, depName)
	}
	sort.Slice(depNames, func(i, j int) bool {
		di := allDeps[depNames[i]]
		dj := allDeps[depNames[j]]
		if di.Required != dj.Required {
			return di.Required && !dj.Required
		}
		return di.Name < dj.Name
	})

	for _, depName := range depNames {
		dep := allDeps[depName]

		cmds := cb.GetDependencyCommands(depName)
		if cmds.Install == nil && dep.Command == "ffmpeg" && runtime.GOOS != "windows" {
			continue
		}

		isInstalled := cb.CheckDependency(dep.Command)

		nameLabel := widget.NewLabel(dep.Name)
		nameLabel.TextStyle = fyne.TextStyle{Bold: true}

		var statusIcon *widget.Icon
		var statusText string
		if isInstalled {
			statusIcon = widget.NewIcon(ui.GetIcon("check"))
			statusText = t.DependenciesInstalled
		} else {
			statusIcon = widget.NewIcon(ui.GetIcon("close"))
			statusText = t.DependenciesNotInstalled
		}
		statusLabel := widget.NewLabel(statusText)
		statusLabel.TextStyle = fyne.TextStyle{Italic: true}

		descLabel := widget.NewLabel(dep.Description)
		descLabel.TextStyle = fyne.TextStyle{Italic: true}
		descLabel.Wrapping = fyne.TextWrapWord

		installLabel := widget.NewLabel(dep.InstallCmd)
		installLabel.Wrapping = fyne.TextWrapWord

		var statusColor color.Color
		if isInstalled {
			statusColor = utils.MustHex("#4CAF50")
		} else {
			statusColor = utils.MustHex("#F44336")
		}

		statusBg := canvas.NewRectangle(statusColor)
		statusBg.CornerRadius = 3

		statusRow := container.NewHBox(statusIcon, statusBg, statusLabel)

		bundledDeps := map[string]bool{
			"ffmpeg":                 true,
			"realesrgan-ncnn-vulkan": true,
			"realcugan-ncnn-vulkan":  true,
			"rife-ncnn-vulkan":       true,
			"whisper":                true,
			"tesseract":              true,
		}

		actions := container.NewHBox()
		cmds = cb.GetDependencyCommands(depName)

		depColor := utils.MustHex(ModuleColor)

		if cmds.Install != nil {
			if !bundledDeps[depName] {
				installBtn := ui.MakePillButton(t.DependenciesInstall, depColor, func() {
					cb.RunDependencyCommandWithProgress(fmt.Sprintf("Installing %s", dep.Name), dep.InstallCmd, cmds.Install, func(out string, err error) {
						cb.ShowCommandResult(fmt.Sprintf("%s Install", dep.Name), out, err)
						cb.ShowSettingsView()
					})
				})
				if isInstalled {
					installBtn.Disable()
				}
				actions.Add(installBtn)
			}
		}

		showUninstall := cmds.Uninstall != nil && !bundledDeps[depName]
		if showUninstall {
			uninstallBtn := ui.MakePillButton(t.DependenciesUninstall, depColor, func() {
				dialog.ShowConfirm(fmt.Sprintf("Uninstall %s?", dep.Name), "This will attempt to remove the dependency using your package manager.", func(ok bool) {
					if !ok {
						return
					}
					cb.RunDependencyCommandWithProgress(fmt.Sprintf("Uninstalling %s", dep.Name), dep.InstallCmd, cmds.Uninstall, func(out string, err error) {
						cb.ShowCommandResult(fmt.Sprintf("%s Uninstall", dep.Name), out, err)
						cb.ShowSettingsView()
					})
				}, cb.Window())
			})
			if !isInstalled {
				uninstallBtn.Disable()
			}
			actions.Add(uninstallBtn)
		}

		infoBox := container.NewVBox(
			container.NewHBox(nameLabel, layout.NewSpacer(), statusRow),
			descLabel,
		)
		if dep.Required {
			requiredLabel := widget.NewLabel(t.DependenciesCore)
			requiredLabel.TextStyle = fyne.TextStyle{Italic: true}
			infoBox.Add(requiredLabel)
		}

		if !isInstalled {
			installCmdLabel := widget.NewLabel(fmt.Sprintf(t.DependenciesInstallCmd, installLabel.Text))
			installCmdLabel.Wrapping = fyne.TextWrapWord
			infoBox.Add(installCmdLabel)
		}

		if len(actions.Objects) > 0 {
			infoBox.Add(container.NewHBox(actions.Objects...))
		}

		modulesNeeding := []string{}
		for modID, deps := range cb.ModuleDependencies() {
			for _, d := range deps {
				if d == depName {
					for _, m := range cb.ModulesList() {
						if m.ID == modID {
							modulesNeeding = append(modulesNeeding, m.Label)
							break
						}
					}
					break
				}
			}
		}

		if len(modulesNeeding) > 0 {
			sort.Strings(modulesNeeding)
			neededLabel := widget.NewLabel(t.DependenciesRequiredBy + " " + strings.Join(modulesNeeding, ", "))
			neededLabel.TextStyle = fyne.TextStyle{Italic: true}
			neededLabel.Wrapping = fyne.TextWrapWord
			infoBox.Add(neededLabel)
		}

		cardBg := canvas.NewRectangle(utils.MustHex("#171C2A"))
		cardBg.CornerRadius = 6
		card := container.NewPadded(container.NewMax(cardBg, infoBox))
		content.Add(card)
	}

	content.Add(widget.NewSeparator())
	refreshBtn := ui.MakePillButton(t.DependenciesRefresh, ui.BorderDim, func() {
		cb.ShowSettingsView()
	})
	content.Add(refreshBtn)

	return content
}

// buildFlagLangSelect creates a language selector that shows a flag icon alongside
// each language name. Tapping opens a popup list; selecting calls onChange with the
// chosen language code.
func buildFlagLangSelect(langs []i18n.Language, activeFont, currentCode string, window fyne.Window, onChange func(string)) fyne.CanvasObject {
	var popup *widget.PopUp

	textCol := color.NRGBA{R: 230, G: 236, B: 245, A: 255}
	bgCol := color.NRGBA{R: 52, G: 66, B: 86, A: 255}

	displayName := func(lang i18n.Language) string {
		if lang.Font == activeFont {
			return lang.NativeName
		}
		return lang.EnglishName
	}

	makeRow := func(lang i18n.Language, bold bool) fyne.CanvasObject {
		objs := []fyne.CanvasObject{}
		if res := ui.GetFlag(lang.Flag); res != nil {
			img := canvas.NewImageFromResource(res)
			img.SetMinSize(fyne.NewSize(24, 12))
			img.FillMode = canvas.ImageFillContain
			objs = append(objs, img)
		}
		lbl := canvas.NewText(displayName(lang), textCol)
		lbl.Alignment = fyne.TextAlignLeading
		lbl.TextSize = 16
		if bold {
			lbl.TextStyle = fyne.TextStyle{Bold: true}
		}
		objs = append(objs, lbl)
		return container.NewHBox(objs...)
	}

	var currentLang i18n.Language
	for _, l := range langs {
		if l.Code == currentCode {
			currentLang = l
			break
		}
	}

	bg := canvas.NewRectangle(bgCol)
	bg.CornerRadius = 8
	bg.SetMinSize(fyne.NewSize(0, 36))

	caret := widget.NewIcon(theme.MenuDropDownIcon())

	buttonContent := container.NewBorder(nil, nil, nil, caret,
		container.NewPadded(makeRow(currentLang, false)))

	var tappable *ui.Tappable
	tappable = ui.NewTappable(container.NewMax(bg, buttonContent), func() {
		if popup != nil {
			popup.Hide()
			popup = nil
			return
		}

		items := make([]fyne.CanvasObject, len(langs))
		for i, lang := range langs {
			l := lang
			row := makeRow(l, l.Code == currentCode)
			items[i] = ui.NewTappable(container.NewPadded(row), func() {
				onChange(l.Code)
				time.AfterFunc(50*time.Millisecond, func() {
					fyne.Do(func() {
						if popup != nil {
							popup.Hide()
							popup = nil
						}
					})
				})
			})
		}

		list := container.NewVBox(items...)
		scroll := container.NewVScroll(list)
		popupW := float32(280)
		popupH := float32(len(langs)) * 44
		scroll.SetMinSize(fyne.NewSize(popupW, popupH))

		popup = widget.NewPopUp(scroll, window.Canvas())
		popup.Resize(fyne.NewSize(popupW, popupH))
		pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(tappable)
		pos.Y += tappable.Size().Height
		popup.ShowAtPosition(pos)
	})

	return tappable
}
