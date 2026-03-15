package main

import (
	"fyne.io/fyne/v2"
	"git.leaktechnologies.dev/stu/VideoTools/internal/app/appcfg"
	"git.leaktechnologies.dev/stu/VideoTools/internal/i18n"
	"git.leaktechnologies.dev/stu/VideoTools/internal/ui"
)

const localePrefKey = "locale"

type localeConfig struct {
	Language string `json:"language"`
	Script   string `json:"script"`
}

// initLocale loads the persisted language preference and applies it.
// It registers a listener that calls refreshFn on the UI thread whenever
// the language changes (e.g. from the Settings selector).
// Call this once during app startup, before the first window is shown.
func initLocale(a fyne.App, refreshFn func()) {
	cfg := localeConfig{Language: "en-CA"}
	appcfg.LoadModuleJSON(localePrefKey, &cfg) // missing file is fine — defaults apply

	if cfg.Language != "" {
		i18n.SetLanguageWithScript(cfg.Language, i18n.ScriptVariant(cfg.Script))
	}

	i18n.RegisterListener(func() {
		ui.SetFontMode(i18n.CurrentFont())
		a.Driver().DoFromGoroutine(refreshFn, false)
	})
}

// persistLocale saves the current language selection to disk.
// Called by the Settings language selector when the user changes language.
func persistLocale(code string, script i18n.ScriptVariant) {
	_ = appcfg.SaveModuleJSON(localePrefKey, localeConfig{
		Language: code,
		Script:   string(script),
	})
}
