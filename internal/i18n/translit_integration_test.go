package i18n

import (
	"testing"
)

func TestTranslitFillLatinToSyllabics(t *testing.T) {
	// Simulate a field that's empty in iu (syllabics) but filled in iu-Latn.
	// This verifies the translitFill mechanism in SetLanguageWithScript.
	if _, ok := registry["iu-Latn"]; !ok {
		t.Skip("iu-Latn not registered")
	}

	// Store current state
	origLang := currentLang
	origScript := currentScript
	defer SetLanguageWithScript(origLang, origScript)

	// Switch to Inuktitut syllabics
	SetLanguageWithScript("iu", ScriptSyllabics)
	cur := T()

	// iu-Latn has "NUATAUSIMAUJUT" for MenuQueue. In syllabics mode,
	// this should have been transliterated to "ᓄᐊᑕᐅᓯᒪᔪᑦ".
	if cur.MenuQueue != "ᓄᐊᑕᐅᓯᒪᔪᑦ" {
		t.Errorf("expected ᓄᐊᑕᐅᓯᒪᔪᑦ, got %q", cur.MenuQueue)
	}

	// Check some other fields too
	if cur.ModuleConvert != "ᐊᓯᔾᔨᕆᐊᖅᑐᖅ" {
		t.Errorf("ModuleConvert: got %q", cur.ModuleConvert)
	}
}

func TestTranslitFillSyllabicsToLatin(t *testing.T) {
	if _, ok := registry["iu"]; !ok {
		t.Skip("iu not registered")
	}

	origLang := currentLang
	origScript := currentScript
	defer SetLanguageWithScript(origLang, origScript)

	// Switch to Inuktitut Latin
	SetLanguageWithScript("iu", ScriptLatin)
	cur := T()

	// iu has "ᓄᐊᑕᐅᓯᒪᔪᑦ" for MenuQueue. In Latin mode,
	// this should have been transliterated to "nuatausimaujut".
	// Note: case depends on the iu_latin.go value; if iu_latin already
	// has a value, the transliteration won't override it.
	if cur.MenuQueue == "" {
		t.Error("MenuQueue should not be empty in Latin mode")
	}
}

func TestTranslitFillFallback(t *testing.T) {
	origLang := currentLang
	origScript := currentScript
	defer SetLanguageWithScript(origLang, origScript)

	// Switch to a language that doesn't exist — should fall back to en-CA
	SetLanguageWithScript("zz", ScriptDefault)
	cur := T()

	if cur.ModuleConvert != "Convert" {
		t.Errorf("fallback to en-CA: got %q for ModuleConvert", cur.ModuleConvert)
	}
}
