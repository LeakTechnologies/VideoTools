// Package i18n provides localization for VideoTools.
//
// Usage:
//
//	i18n.SetLanguage("fr-CA")   // switch language; fires all listeners
//	s := i18n.T()               // get current Strings; use s.ModuleConvert etc.
//
// All languages fall back to English (Canada) for any untranslated field.
package i18n

import (
	"reflect"
	"sync"
)

// Language describes a supported language.
type Language struct {
	Code        string // BCP-47 code, e.g. "en-CA", "fr-CA", "iu"
	EnglishName string // e.g. "English (Canada)"
	NativeName  string // e.g. "Français (Canada)", "ᐃᓄᒃᑎᑐᑦ"
	Font        string // "mono" or "aboriginal" (Aboriginal Sans)
}

// ScriptVariant is an optional writing system choice (e.g. for Inuktitut).
type ScriptVariant string

const (
	ScriptDefault   ScriptVariant = ""
	ScriptSyllabics ScriptVariant = "syllabics"
	ScriptLatin     ScriptVariant = "latin"
)

var (
	mu            sync.RWMutex
	current       = enCA // active Strings
	currentLang   = "en-CA"
	currentScript ScriptVariant
	listeners     []func()
	listenersMu   sync.Mutex
)

// All returns the list of all registered languages.
func All() []Language {
	return allLanguages
}

// CurrentCode returns the active language code.
func CurrentCode() string {
	mu.RLock()
	defer mu.RUnlock()
	return currentLang
}

// CurrentFont returns the font family hint for the active language ("mono" or "aboriginal").
// Inuktitut in Latin script uses "mono" since the romanization is standard Latin.
func CurrentFont() string {
	mu.RLock()
	defer mu.RUnlock()
	if currentLang == "iu" && currentScript == ScriptLatin {
		return "mono"
	}
	for _, lang := range allLanguages {
		if lang.Code == currentLang {
			return lang.Font
		}
	}
	return "mono"
}

// CurrentScript returns the active script variant (relevant for Inuktitut).
func CurrentScript() ScriptVariant {
	mu.RLock()
	defer mu.RUnlock()
	return currentScript
}

// T returns the current Strings, with en-CA fallback for any empty field.
func T() Strings {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// SetLanguage switches the active language and notifies all listeners.
// Unknown codes fall back to en-CA silently.
func SetLanguage(code string) {
	SetLanguageWithScript(code, ScriptDefault)
}

// SetLanguageWithScript switches language and script variant simultaneously.
// For Inuktitut, ScriptLatin loads the romanized (Qaliujaaqpait) string set
// and switches the font to mono; ScriptSyllabics uses the syllabics set with
// Aboriginal Sans.  currentLang is always stored as "iu" (not "iu-Latn") so
// the Settings script toggle remains visible after a restart.
func SetLanguageWithScript(code string, script ScriptVariant) {
	// Resolve which registry entry to load.
	lookupCode := code
	if code == "iu" && script == ScriptLatin {
		lookupCode = "iu-Latn"
	}

	strings, ok := registry[lookupCode]
	if !ok {
		// "iu-Latn" not found — fall back to syllabics before giving up.
		strings, ok = registry[code]
		if !ok {
			code = "en-CA"
			strings = enCA
			script = ScriptDefault
		}
	}
	merged := fallback(strings, enCA)

	mu.Lock()
	current = merged
	currentLang = code // always "iu", not "iu-Latn"
	currentScript = script
	mu.Unlock()

	notify()
}

// RegisterListener registers a function to be called when the language changes.
// The function is called on the goroutine that called SetLanguage — callers
// should dispatch to the UI thread themselves if needed.
func RegisterListener(fn func()) {
	listenersMu.Lock()
	defer listenersMu.Unlock()
	listeners = append(listeners, fn)
}

// notify calls all registered listeners.
func notify() {
	listenersMu.Lock()
	fns := make([]func(), len(listeners))
	copy(fns, listeners)
	listenersMu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

// fallback returns a copy of s where every empty string field is replaced
// with the corresponding field from ref (the en-CA source of truth).
func fallback(s, ref Strings) Strings {
	sv := reflect.ValueOf(&s).Elem()
	rv := reflect.ValueOf(ref)
	for i := 0; i < sv.NumField(); i++ {
		if sv.Field(i).String() == "" {
			sv.Field(i).SetString(rv.Field(i).String())
		}
	}
	return s
}

// countFields returns the number of non-empty string fields in s.
func countFields(s Strings) int {
	v := reflect.ValueOf(s)
	n := 0
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).String() != "" {
			n++
		}
	}
	return n
}

// registry maps language codes to their Strings instances.
// Languages are registered by their language files via init().
var registry = map[string]Strings{}

func register(code string, s Strings) {
	registry[code] = s
}
