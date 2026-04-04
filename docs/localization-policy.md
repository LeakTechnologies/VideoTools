# VideoTools Localization Strategy and Implementation Guide

## Overview

This document provides a comprehensive guide to VideoTools' localization system, including implementation details, contribution guidelines, and cultural considerations.

## Quick Start

### Architecture
- **Primary System**: VideoTools-controlled localization via go-i18n
- **Fyne Integration**: Minimal, only for system dialogs
- **Strategy**: Module-by-module localization with cultural respect

### Language Priority
1. **English (en-CA)** - Source language
2. **French (fr-CA)** - Canadian official language
3. **Indigenous Languages** - Inuktitut, Cree (human-reviewed)
4. **Global Languages** - Future expansion

## Implementation Guide

### File Structure
```
localization/
├── en-CA/
│   ├── meta.json
│   ├── common.json
│   ├── convert.json
│   └── [module files]
├── fr-CA/
│   └── [same structure]
├── iu/
│   ├── meta.json
│   ├── common.syllabics.json
│   ├── common.roman.json
│   └── [dual-script modules]
└── cr/
    └── [dual-script structure]
```

### Key Naming Convention
```
[module].[category].[item]
```

Examples:
- `convert.output.format` - Output format label
- `player.controls.play` - Play button
- `error.file.not_found` - File not found error
- `common.button.ok` - Generic OK button

### Usage in Code
```go
// Primary localization
btn := widget.NewButton(localization.T("convert.start"), onClick)

// Optional generic (whitelisted only)
cancelBtn := widget.NewButton(localization.Generic("Cancel"), onClick)
```

## Localization Rules

### Brand Protection
- **VT**: Never translated, never localized
- **VideoTools**: Translated by meaning only for non-Latin scripts

### Terminology
- **Movie**: Digital files (MP4, MKV, MOV)
- **Film**: Physical media (8mm, 16mm, 35mm)
- Never use these terms interchangeably

### Script Handling
- **Indigenous languages**: Dual-script support (syllabics + romanized)
- **No machine translation** for Indigenous languages
- **Human review required** for all Indigenous translations

## Testing

### Automated Tests
- Translation completeness checking
- Key consistency validation
- UI layout expansion testing (pseudo-languages)
- Script rendering validation

### Manual Tests
- In-context translation verification
- Module-by-module validation
- Language switching functionality

## Indigenous Language Handling

### Current Status

The Inuktitut translations in `iu.go` (Unified Canadian Aboriginal Syllabics) and
`iu_latin.go` (romanized Latin) are **machine-generated placeholders**. They have not
been reviewed by a fluent speaker. The localization-policy statement "no machine
translation for Indigenous languages" reflects the long-term goal, not the current
state — these files exist to test dual-script rendering and provide a starting point,
not to represent authoritative Inuktitut.

### Why This Matters

Inuktitut is a living language spoken by Inuit communities across Nunavut, Nunavik,
and Nunatsiavut. Using inaccurate or culturally inappropriate translations — even in a
technical UI — can be perceived as disrespectful or tokenistic. Machine translation of
Inuktitut is particularly unreliable because:

- Standard LLM/MT systems have very limited Inuktitut training data.
- Technical vocabulary (software concepts like "queue", "job", "restart") has no
  established community-agreed Inuktitut equivalent; machine output is often a
  phonetic borrowing or a misleading approximation.
- The syllabics orthography has regional variants; machine output may not match the
  conventions familiar to a specific community.

### Rules for Adding New Strings

1. **Add the string to all locale files** — `en_ca.go`, `fr_ca.go`, `iu.go`,
   `iu_latin.go`. Never leave a locale file missing a key; missing keys fall back to
   an empty string in the UI.

2. **Machine translation is acceptable as a placeholder** but must be accompanied by
   a `// machine-generated, needs human review` comment on the same line. Example:
   ```go
   StatusUpdateBlockedByJob: "Suliaq malinngajumiittara...", // machine-generated, needs human review
   ```

3. **Flag technical-concept strings explicitly.** When a string introduces a concept
   with no clear Inuktitut equivalent (e.g. "job queue", "binary restart", "codec"),
   add a comment explaining the intended meaning in plain English so a future reviewer
   has context:
   ```go
   DialogUpdateBlocked: "Nutaaliqpallaarumasuq Nalinginnaanginnaq", // machine-generated — means: software update cannot be installed right now
   ```

4. **Do not back-translate** machine output to verify it. Generating Inuktitut with
   one model and checking it with another gives false confidence. The only valid review
   is by a fluent human speaker.

5. **Do not remove or leave blank** an Inuktitut string just because you are unsure of
   the translation. A placeholder — clearly marked — is better than a silent fallback
   to English, which breaks the UI contract for users who have selected Inuktitut.

### Path to Human Review

When a fluent Inuktitut speaker or a community translator reviews strings:

- Remove the `// machine-generated, needs human review` comment.
- Add a `// reviewed` comment with the review date (year is sufficient): `// reviewed 2026`.
- If the reviewer requests romanization changes (iu_latin.go) separately from
  syllabics changes (iu.go), treat them as independent reviews — do not assume
  syllabics approval implies Latin approval or vice versa.

### Scope of Current Machine-Generated Strings

All strings currently in `iu.go` and `iu_latin.go` are machine-generated unless
explicitly marked `// reviewed`. As of April 2026, no strings have been human-reviewed.
Any new strings added without a human reviewer must carry the `// machine-generated`
comment.

---

## Contributing

### Translation Guidelines
1. Follow key naming conventions
2. Provide context for ambiguous terms
3. Maintain terminology consistency
4. Include cultural notes where relevant

### Indigenous Languages
- Follow the rules in the **Indigenous Language Handling** section above.
- Must include reviewer credit/approval before the `// machine-generated` comment is removed.
- Dual-script (syllabics + Latin romanization) required for Inuktitut.
- Cultural appropriateness review required before any public-facing release targets Inuktitut as a supported language.

## Configuration

### Language Preference
```json
{
  "language": "en-CA",
  "secondaryScript": false,
  "autoDetect": true
}
```

### Fallback Chain
1. User-selected language
2. User-selected language + secondary script
3. English (en-CA)
4. Emergency fallback string

## Resources

For module-specific localization guides, see:
- Convert Module Localization (planned)
- Player Module Localization (planned)
- UI Components Localization (planned)
