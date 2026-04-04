# VideoTools Localization Policy

## Overview

VideoTools uses a custom Go-based localization system. All user-facing strings are
defined as fields on a `Strings` struct and served through a simple package API.
There is no dependency on external i18n libraries (the `go-i18n` reference visible in
`_fyne/` belongs to the Fyne submodule, not VideoTools).

## Supported Languages

| Code    | Name                   | Script        | Status                        |
|---------|------------------------|---------------|-------------------------------|
| `en-CA` | English (Canada)       | Latin         | Source of truth — complete    |
| `fr-CA` | French (Canada)        | Latin         | Active — maintained           |
| `iu`    | Inuktitut (ᐃᓄᒃᑎᑐᑦ)  | UCAS syllabics | Placeholder — needs review    |
| `iu-Latn` | Inuktitut (Qaliujaaqpait) | Latin romanization | Placeholder — needs review |

Cree and other languages listed in earlier planning documents are **not implemented**
and have no locale files. Do not reference them as supported.

## File Structure

All locale files live in `internal/i18n/`:

```
internal/i18n/
├── strings.go      — Strings struct definition (all keys); CompletionPercent helper
├── i18n.go         — T(), SetLanguage(), SetLanguageWithScript(), fallback logic
├── languages.go    — allLanguages registry (shown in Settings language picker)
├── en_ca.go        — English (Canada) — source of truth
├── fr_ca.go        — French (Canada)
├── iu.go           — Inuktitut syllabics
└── iu_latin.go     — Inuktitut romanized Latin (registered as "iu-Latn")
```

## API

```go
// Get the current Strings (en-CA fallback for any empty field):
t := i18n.T()
label := widget.NewLabel(t.ModuleConvert)
btn := widget.NewButton(t.ActionSave, onClick)

// Switch language (called from Settings):
i18n.SetLanguage("fr-CA")
i18n.SetLanguageWithScript("iu", i18n.ScriptLatin)

// Query active language:
code := i18n.CurrentCode()   // e.g. "iu"
font := i18n.CurrentFont()   // "mono" or "aboriginal"
```

## Adding a New String

1. Add the field to `internal/i18n/strings.go` with a comment:
   ```go
   DialogUpdateBlocked string // "Update Blocked"
   ```
2. Add the English value to `en_ca.go` — this is the source of truth.
3. Add the French translation to `fr_ca.go`.
4. Add Inuktitut placeholders to `iu.go` and `iu_latin.go` — see
   **Indigenous Language Handling** below for required comment format.

Never leave any locale file missing a key. Missing keys silently fall back to an empty
string (not en-CA) in the UI unless the `fallback()` function runs, which it does at
`SetLanguage` time — but only if the field was empty at registration. Safe practice is
to always populate all four files.

## Key Naming Conventions

Fields on `Strings` follow these prefixes:

| Prefix       | Use                              | Example                   |
|--------------|----------------------------------|---------------------------|
| `Module`     | Module/tab names                 | `ModuleConvert`           |
| `Action`     | Button labels / verb actions     | `ActionSave`, `ActionCancel` |
| `Label`      | Static descriptive labels        | `LabelOutput`, `LabelNoFile` |
| `Status`     | Status/progress messages         | `StatusComplete`, `StatusFailed` |
| `Dialog`     | Dialog titles and short messages | `DialogConfirm`, `DialogUpdateBlocked` |

## Localization Rules

### Brand

- **VT** and **VideoTools** are never translated.
- Do not translate proper nouns (codec names, container names, tool names).

### Terminology

- **Movie** — digital files (MP4, MKV, etc.)
- **Film** — physical media (8mm, 16mm, 35mm)
- Do not use these terms interchangeably.

### Fallback

The fallback chain is:

1. User-selected language and script variant
2. en-CA (for any empty/missing field in the selected locale)

There is no "emergency fallback string" beyond en-CA. If en-CA is missing a field the
UI will show an empty string — catch this with `CompletionPercent` before release.

### Script Variants (Inuktitut)

Inuktitut users can choose between syllabics (UCAS, `iu.go`) and Latin romanization
(`iu_latin.go`) in Settings. The language code stored in preferences is always `"iu"`;
the script variant is stored separately. `SetLanguageWithScript("iu", ScriptLatin)`
loads `iu_latin.go`. The font switches automatically: syllabics uses Aboriginal Sans,
Latin uses the standard mono font.

## Indigenous Language Handling

### Current Status

All strings in `iu.go` and `iu_latin.go` are **machine-generated placeholders**. They
have not been reviewed by a fluent speaker. As of April 2026, no Inuktitut strings are
human-reviewed. The files exist to exercise dual-script rendering and font switching,
not to provide authoritative translations.

### Why Accuracy Matters

Inuktitut is a living language spoken by Inuit communities across Nunavut, Nunavik,
and Nunatsiavut. Inaccurate or tokenistic translations in a public UI can cause harm.
Machine translation of Inuktitut is particularly unreliable because:

- LLM and MT systems have very limited Inuktitut training data.
- Technical software concepts ("job queue", "codec", "restart") have no
  community-agreed Inuktitut equivalent; machine output is often a phonetic borrowing
  or a misleading approximation.
- The UCAS syllabics orthography has regional variants; machine output may not match
  the conventions of any particular community.

### Rules for Adding New Inuktitut Strings

1. **Machine-generated placeholders are acceptable**, but must carry a comment:
   ```go
   // machine-generated, needs human review
   ```
   Include a plain-English gloss of the intended meaning when the concept is technical:
   ```go
   DialogUpdateBlocked: "Nutaaliqpallaarumasuq Nalinginnaanginnaq", // machine-generated, needs human review — means: software update cannot be installed right now
   ```

2. **Do not back-translate** to verify. Running the Inuktitut output through another
   model to check it produces false confidence. Human review is the only valid check.

3. **Never leave a key blank.** A marked placeholder is better than a silent fallback
   that breaks the UI for users who selected Inuktitut.

4. Both `iu.go` (syllabics) and `iu_latin.go` (Latin) must be updated together. Do
   not assume a syllabics translation is equivalent to its Latin romanization — they
   are treated as independent reviews.

### Path to Human Review

When a fluent speaker or community translator reviews a string:

- Remove the `// machine-generated, needs human review` comment.
- Add `// reviewed YYYY` (year is sufficient).
- Syllabics and Latin romanization reviews are independent — do not mark both reviewed
  based on one reviewer's approval unless they reviewed both scripts.

Inuktitut should not be advertised as a fully supported language until all strings in
both scripts carry `// reviewed`.

## Translation Completeness

`internal/i18n/strings.go` exposes `CompletionPercent(s, reference Strings) float64`
which compares a locale against en-CA. Run it manually or in a test to detect gaps
before release. There are currently no automated CI checks for translation completeness.

## Contributing

1. Follow the key naming conventions above.
2. Provide context comments for ambiguous terms.
3. For Inuktitut, follow the **Indigenous Language Handling** rules.
4. Never hardcode user-visible strings — all UI text must go through `i18n.T()`.
