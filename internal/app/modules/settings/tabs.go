package settings

import (
	"fmt"
	"runtime"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"image/color"

	"git.leaktechnologies.dev/stu/VideoTools/internal/app/appcfg"
	"git.leaktechnologies.dev/stu/VideoTools/internal/benchmark"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
	"git.leaktechnologies.dev/stu/VideoTools/internal/utils"
)

func BuildBenchmarkTab(cb BenchmarkCallbacks) fyne.CanvasObject {
	content := container.NewVBox()
	t := i18n.T()

	header := widget.NewLabel(t.BenchmarkTitle)
	header.TextStyle = fyne.TextStyle{Bold: true}
	content.Add(header)

	desc := widget.NewLabel(t.BenchmarkDesc)
	desc.Wrapping = fyne.TextWrapWord
	content.Add(desc)

	content.Add(widget.NewSeparator())

	runBtn := widget.NewButton(t.BenchmarkRunButton, func() {
		cb.ShowBenchmark()
	})
	runBtn.Importance = widget.MediumImportance
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
			timestamp := run.Timestamp.Format("Jan 2, 2006 at 3:04 PM")
			summary := fmt.Sprintf("%s - Recommended: %s", timestamp, benchmark.HWAccelLabel(run.RecommendedHWAccel))

			runLabel := widget.NewLabel(summary)
			runLabel.TextStyle = fyne.TextStyle{Italic: true}
			content.Add(runLabel)
		}
	}

	return content
}

func BuildPreferencesTab(cb PreferencesCallbacks) fyne.CanvasObject {
	content := container.NewVBox()
	t := i18n.T()

	header := widget.NewLabel(t.SettingsAppPreferences)
	header.TextStyle = fyne.TextStyle{Bold: true}
	content.Add(header)

	content.Add(widget.NewSeparator())

	updatesHeader := widget.NewLabel(t.SettingsTabUpdates)
	updatesHeader.TextStyle = fyne.TextStyle{Bold: true}
	content.Add(updatesHeader)

	versionLabel := widget.NewLabel(fmt.Sprintf("%s %s", t.UpdateCurrentVersion, cb.FullVersion()))
	versionLabel.TextStyle = fyne.TextStyle{Bold: true}
	content.Add(versionLabel)

	hashDisplay := cb.BuildCommit()
	if hashDisplay == "" || hashDisplay == "dev" {
		hashDisplay = "development build"
	}
	hashLabel := widget.NewLabel(fmt.Sprintf("%s %s", t.UpdateVersionHash, hashDisplay))
	hashLabel.TextStyle = fyne.TextStyle{Monospace: true}
	content.Add(hashLabel)

	updateStatusIcon := widget.NewIcon(nil)
	updateStatusIcon.Hide()
	updateStatusLabel := widget.NewLabel("")
	updateStatusLabel.TextStyle = fyne.TextStyle{Italic: true}
	content.Add(container.NewHBox(updateStatusIcon, updateStatusLabel))

	installBtn := widget.NewButton(t.UpdateInstall, nil)
	installBtn.Importance = widget.HighImportance
	installBtn.Hide()

	onUpdateAvailable := func(tag string) {
		if tag == "" {
			installBtn.Hide()
			return
		}
		installBtn.OnTapped = func() { cb.ApplyUpdate(tag) }
		installBtn.Show()
	}

	checkBtn := widget.NewButton(t.UpdateCheckButton, func() {
		installBtn.Hide()
		cb.CheckForUpdatesWithStatus(updateStatusIcon, updateStatusLabel, onUpdateAvailable)
	})
	checkBtn.Importance = widget.MediumImportance

	content.Add(container.NewHBox(checkBtn, installBtn))

	if !cb.UpdateLastChecked().IsZero() {
		cb.ApplyUpdateStatusToUI(updateStatusIcon, updateStatusLabel, onUpdateAvailable)
	} else {
		cb.CheckForUpdatesWithStatus(updateStatusIcon, updateStatusLabel, onUpdateAvailable)
	}

	autoCheckLabel := widget.NewLabel(t.UpdateAutoCheck)
	autoCheckLabel.TextStyle = fyne.TextStyle{}

	autoCheckKeys := []string{
		"disabled", "every_hour", "every_2h", "every_3h", "every_4h",
		"every_6h", "every_12h", "daily", "semi_weekly", "weekly",
		"bi_weekly", "monthly", "bi_monthly",
	}
	autoCheckOptions := []string{
		t.UpdateDisabled,
		t.UpdateEveryHour,
		t.UpdateEvery2Hours,
		t.UpdateEvery3Hours,
		t.UpdateEvery4Hours,
		t.UpdateEvery6Hours,
		t.UpdateEvery12Hours,
		t.UpdateDaily,
		t.UpdateSemiWeekly,
		t.UpdateWeekly,
		t.UpdateBiWeekly,
		t.UpdateMonthly,
		t.UpdateBiMonthly,
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

	prefs := cb.PrefsConfig()
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

	autoCheckRow := container.NewHBox(autoCheckLabel, autoCheckSelect)
	content.Add(autoCheckRow)

	infoLabel := widget.NewLabel(t.SettingsUpdatesAutoInfo)
	infoLabel.Wrapping = fyne.TextWrapWord
	infoLabel.TextStyle = fyne.TextStyle{Italic: true}
	content.Add(infoLabel)

	content.Add(widget.NewSeparator())

	langLabel := widget.NewLabel(t.SettingsLanguage)
	langLabel.TextStyle = fyne.TextStyle{Bold: true}

	langOptions := i18n.All()
	langNames := make([]string, len(langOptions))
	langCodes := make([]string, len(langOptions))
	activeFont := i18n.CurrentFont()
	for i, lang := range langOptions {
		if lang.Font != activeFont {
			langNames[i] = lang.EnglishName
		} else {
			langNames[i] = lang.NativeName
		}
		langCodes[i] = lang.Code
	}

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

	langSelect := widget.NewSelect(langNames, func(selected string) {
		for i, name := range langNames {
			if name == selected {
				if langCodes[i] == i18n.CurrentCode() {
					return
				}
				i18n.SetLanguage(langCodes[i])
				cb.PersistLocale(langCodes[i], i18n.CurrentScript())
				cb.ShowSettingsView()
				break
			}
		}
	})
	currentCode := i18n.CurrentCode()
	for i, code := range langCodes {
		if code == currentCode {
			langSelect.SetSelected(langNames[i])
			break
		}
	}

	langSection := container.NewVBox(
		langLabel,
		langSelect,
		container.NewHBox(scriptLabel, scriptSelect),
	)
	content.Add(langSection)

	content.Add(widget.NewSeparator())

	masterHeader := widget.NewLabel(t.SettingsMasterSettings)
	masterHeader.TextStyle = fyne.TextStyle{Bold: true}
	content.Add(masterHeader)

	hwLabel := widget.NewLabel(t.SettingsHardwareAccel)
	hwLabel.TextStyle = fyne.TextStyle{Bold: true}

	hwStatus := widget.NewLabel("Press Detect to scan your hardware.")
	hwStatus.TextStyle = fyne.TextStyle{Monospace: true}
	hwStatus.Wrapping = fyne.TextWrapWord

	hwSelect := widget.NewSelect([]string{"auto", "none", "nvenc", "qsv", "amf", "vaapi"}, func(selected string) {
		cb.SetConvertHardwareAccel(selected)
		cb.PersistConvertConfig()
	})
	hwSelect.SetSelected(cb.ConvertHardwareAccel())

	var detectBtn *widget.Button
	detectBtn = widget.NewButton(t.SettingsDetect, func() {
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
	detectBtn.Importance = widget.HighImportance

	autoBtn := widget.NewButton(t.SettingsUseAuto, func() {
		hwSelect.SetSelected("auto")
		cb.SetConvertHardwareAccel("auto")
		cb.PersistConvertConfig()
		hwStatus.SetText("Set to auto — best available encoder selected at encode time.")
	})
	autoBtn.Importance = widget.MediumImportance

	content.Add(container.NewVBox(
		hwLabel,
		hwSelect,
		container.NewHBox(detectBtn, autoBtn),
		hwStatus,
	))

	content.Add(widget.NewSeparator())

	moduleHeader := widget.NewLabel(t.SettingsModuleVisibility)
	moduleHeader.TextStyle = fyne.TextStyle{Bold: true}
	content.Add(moduleHeader)

	showUpscale := widget.NewCheck(t.SettingsShowUpscale, func(checked bool) {
		cb.SetConvertShowUpscale(checked)
		cb.PersistConvertConfig()
	})
	showUpscale.SetChecked(cb.ConvertShowUpscale())

	visibilityItems := []fyne.CanvasObject{showUpscale}

	showDisc := widget.NewCheck(t.SettingsShowDisc, func(checked bool) {
		cb.SetConvertShowDisc(checked)
		cb.PersistConvertConfig()
	})
	showDisc.SetChecked(cb.ConvertShowDisc())

	visibilityItems = append(visibilityItems, showDisc)

	visibilityHint := widget.NewLabel(t.SettingsModuleVisibilityHint)
	visibilityHint.TextStyle = fyne.TextStyle{Italic: true}
	visibilityHint.Wrapping = fyne.TextWrapWord
	visibilityItems = append(visibilityItems, visibilityHint)

	content.Add(container.NewVBox(visibilityItems...))

	content.Add(widget.NewSeparator())

	queueHeader := widget.NewLabel(t.SettingsQueueSection)
	queueHeader.TextStyle = fyne.TextStyle{Bold: true}
	content.Add(queueHeader)

	queuePlayLabel := widget.NewLabel(t.SettingsQueuePlayLabel)
	content.Add(queuePlayLabel)

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
	content.Add(queuePlayRadio)

	queuePlayHint := widget.NewLabel(t.SettingsQueuePlayHint)
	queuePlayHint.TextStyle = fyne.TextStyle{Italic: true}
	queuePlayHint.Wrapping = fyne.TextWrapWord
	content.Add(queuePlayHint)

	content.Add(widget.NewSeparator())

	tooltipsHeader := widget.NewLabel(t.SettingsShowTooltips)
	tooltipsHeader.TextStyle = fyne.TextStyle{Bold: true}
	content.Add(tooltipsHeader)

	showTooltipsCheck := widget.NewCheck("", func(enabled bool) {
		cb.SetShowTooltips(enabled)
	})
	showTooltipsCheck.Checked = prefs.ShowTooltips
	content.Add(showTooltipsCheck)

	content.Add(widget.NewSeparator())

	outputHeader := widget.NewLabel(t.SettingsDefaultOutputDir)
	outputHeader.TextStyle = fyne.TextStyle{Bold: true}
	content.Add(outputHeader)

	outputDirEntry := widget.NewEntry()
	outputDirEntry.SetPlaceHolder("~/Videos/VideoTools")
	outputDirEntry.SetText(cb.DefaultOutputDir())
	outputDirEntry.OnChanged = func(val string) {
		cb.SetDefaultOutputDir(strings.TrimSpace(val))
	}

	outputBrowseBtn := widget.NewButton(t.ActionBrowse, func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			path := uri.Path()
			outputDirEntry.SetText(path)
			cb.SetDefaultOutputDir(path)
		}, cb.Window())
	})
	outputBrowseBtn.Importance = widget.MediumImportance

	outputClearBtn := widget.NewButton(t.ConvertUseDefault, func() {
		outputDirEntry.SetText("")
		cb.SetDefaultOutputDir("")
	})
	outputClearBtn.Importance = widget.LowImportance

	outputHint := widget.NewLabel(t.SettingsDefaultOutputDirHint)
	outputHint.TextStyle = fyne.TextStyle{Italic: true}
	outputHint.Wrapping = fyne.TextWrapWord

	content.Add(container.NewBorder(nil, nil, nil,
		container.NewHBox(outputBrowseBtn, outputClearBtn),
		outputDirEntry,
	))
	content.Add(outputHint)

	return content
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

		actions := container.NewHBox()
		cmds = cb.GetDependencyCommands(depName)

		if depName == "ffmpeg" && runtime.GOOS == "windows" {
			installBtn := widget.NewButton(t.DependenciesInstall, func() {
				cb.InstallWindowsFFmpeg(func() {
					dialog.ShowInformation("FFmpeg Ready", "FFmpeg is installed for this user and now available in the app.", cb.Window())
					cb.ShowSettingsView()
				})
			})
			installBtn.Importance = widget.HighImportance
			if isInstalled {
				installBtn.Disable()
			}
			actions.Add(installBtn)
		}

		if depName == "realesrgan-ncnn-vulkan" && (runtime.GOOS == "windows" || runtime.GOOS == "linux") {
			installBtn := widget.NewButton(t.DependenciesInstall, func() {
				cb.InstallRealESRGAN(func() {
					dialog.ShowInformation("Real-ESRGAN Ready", "Real-ESRGAN is installed and available for AI upscaling.", cb.Window())
					cb.ShowSettingsView()
				})
			})
			installBtn.Importance = widget.HighImportance
			if isInstalled {
				installBtn.Disable()
			}
			actions.Add(installBtn)
		}

		if depName == "rife-ncnn-vulkan" && (runtime.GOOS == "windows" || runtime.GOOS == "linux") {
			installBtn := widget.NewButton(t.DependenciesInstall, func() {
				cb.InstallRIFE(func() {
					dialog.ShowInformation("RIFE Ready", "RIFE is installed and available for frame interpolation.", cb.Window())
					cb.ShowSettingsView()
				})
			})
			installBtn.Importance = widget.HighImportance
			if isInstalled {
				installBtn.Disable()
			}
			actions.Add(installBtn)
		}

		if depName == "whisper" && runtime.GOOS == "windows" && cmds.Install == nil && !isInstalled {
			installBtn := widget.NewButton("Install (Python + Whisper)", func() {
				cb.InstallWindowsPython(func(pythonExe string) {
					cb.RunDependencyCommandWithProgress("Installing Whisper", "Installing openai-whisper...",
						NewDependencyCommand(pythonExe, "-m", "pip", "install", "openai-whisper"),
						func(out string, err error) {
							cb.ShowCommandResult("Whisper Install", out, err)
							cb.ShowSettingsView()
						})
				})
			})
			installBtn.Importance = widget.HighImportance
			actions.Add(installBtn)
		}

		if cmds.Install != nil {
			// Skip generic install button if we already added a special case button
			// (realesrgan-ncnn-vulkan and rife-ncnn-vulkan have custom install handlers)
			hasSpecialInstallButton := (depName == "realesrgan-ncnn-vulkan" || depName == "rife-ncnn-vulkan") &&
				(runtime.GOOS == "windows" || runtime.GOOS == "linux")

			if !hasSpecialInstallButton {
				installBtn := widget.NewButton(t.DependenciesInstall, func() {
					cb.RunDependencyCommandWithProgress(fmt.Sprintf("Installing %s", dep.Name), dep.InstallCmd, cmds.Install, func(out string, err error) {
						cb.ShowCommandResult(fmt.Sprintf("%s Install", dep.Name), out, err)
						cb.ShowSettingsView()
					})
				})
				installBtn.Importance = widget.HighImportance
				if isInstalled {
					installBtn.Disable()
				}
				actions.Add(installBtn)
			}
		}

		if cmds.Uninstall != nil {
			uninstallBtn := widget.NewButton(t.DependenciesUninstall, func() {
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
			uninstallBtn.Importance = widget.LowImportance
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

		if actions.Objects != nil && len(actions.Objects) > 0 {
			actionsContainer := container.NewHBox(actions.Objects...)
			infoBox.Add(actionsContainer)
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
	refreshBtn := widget.NewButton(t.DependenciesRefresh, func() {
		cb.ShowSettingsView()
	})
	content.Add(refreshBtn)

	return content
}
