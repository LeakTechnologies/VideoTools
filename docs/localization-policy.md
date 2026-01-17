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

## Contributing

### Translation Guidelines
1. Follow key naming conventions
2. Provide context for ambiguous terms
3. Maintain terminology consistency
4. Include cultural notes where relevant

### Indigenous Languages
- Must include reviewer credit/approval
- Dual-script preferred when possible
- Cultural appropriateness review required

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

For complete policy details, see: [localization-policy.md](../.opencode/plans/videotools-localization-policy.md)

For module-specific localization guides, see:
- [Convert Module Localization](convert/LOCALIZATION.md)
- [Player Module Localization](../internal/player/LOCALIZATION.md)
- [UI Components Localization](../internal/ui/LOCALIZATION.md)