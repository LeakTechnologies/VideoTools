# VideoTools Localization Policy

## Overview

VideoTools uses a single authoritative localization system for all user-facing strings. Fyne's built-in localization is used only for system-level UI elements where we have limited influence. This ensures terminology control, Canadian language priorities, and Indigenous language quality requirements.

## Language Support Strategy

### Priority Order

1. **English (en-CA)** - Source language and authoritative default
2. **French (fr-CA)** - Official language parity for Canada
3. **Indigenous Languages** - Inuktitut, Cree (phased rollout with human review)
4. **Global Languages** - Future expansion when resources allow

### Language Pack Structure

```
localization/
├── en-CA/
│   ├── meta.json
│   ├── common.json
│   ├── convert.json
│   ├── merge.json
│   ├── player.json
│   ├── ui.json
│   └── errors.json
├── fr-CA/
│   ├── meta.json
│   ├── [same module structure as en-CA]
├── iu/
│   ├── meta.json
│   ├── common.syllabics.json
│   ├── common.roman.json
│   ├── convert.syllabics.json
│   ├── convert.roman.json
│   └── [other modules with dual scripts]
├── cr/
│   ├── meta.json
│   ├── [dual-script structure like iu]
└── README.md
```

## Authoritative Language Rules

### Canadian English (en-CA) Policy

- en-CA is **source and authoritative** language
- All localization keys originate in en-CA
- Spelling follows Canadian conventions: colour, centre, licence, defence
- Technical vocabulary uses North American terms: program (not programme), truck (not lorry)
- en-US is a **derived localization**, not a fallback

### Branding Rules

#### VT Brand Mark
- **VT is never translated**
- VT is never rendered in non-Latin scripts
- VT is never localized or modified

#### VideoTools Product Name
- Remains untranslated in Latin-script languages
- Translated by **meaning only** for non-Latin scripts
- Treated as display text, not part of logo asset
- Translation must reflect "tools for working with video," not phonetic approximation

### Script Handling

#### Indigenous Languages with Dual Scripts
- **Primary script**: Syllabics (first-class display)
- **Secondary script**: Romanized (accessibility/toggle option)
- Both scripts must be **human-reviewed** and approved
- No machine translation or automatic transliteration

#### Script Selection Logic
1. User-selected language + primary script
2. User-selected language + secondary script (if enabled)
3. English (en-CA)
4. Safe fallback string

## Terminology Rules

### Core Media Concepts

#### Movie vs Film (Strict Enforcement)
- **Movie**: Completed audiovisual work (digital files: MP4, MKV, MOV, AVI)
- **Film**: Physical or photochemical medium (8mm, 16mm, 35mm, reels, negatives)
- **Never interchangeable** in VT UI or documentation

#### Technical Terms
- **Video**: Visual component of audiovisual signal
- **Audio**: Sound component of media file
- **Container**: File format encapsulating streams (MP4, MKV, MOV)
- **Codec**: Encoding/decoding method (H.264, HEVC, AV1, AAC)
- **Stream**: Individual encoded channel within container

### User Interface Terminology

#### Standardized Terms
- **Tool**: Functional component performing specific task
- **Settings**: User-configurable options (preferred over "Preferences")
- **Menu**: Navigational UI element grouping actions
- **Module**: Major functional area within VT

#### Avoided Terms
Do not use unless explicitly defined:
- Media (too broad without context)
- Content (unless clearly scoped)
- Clip (ambiguous)
- Footage (unless raw source material)

## String Key Architecture

### Key Naming Convention

```
[module].[category].[item]
```

#### Examples
- `convert.output.format` - Output format dropdown label
- `merge.chapters.enable` - Chapter preservation checkbox
- `player.controls.play` - Play button label
- `ui.menu.file` - File menu header
- `error.file.not_found` - File not found error message
- `common.button.ok` - Generic OK button

### Key Rules

- **Semantic keys only** - never English strings as keys
- **Hierarchical organization** by module and category
- **Stable keys** - once defined, keys do not change
- **Descriptive, not terse** - `error.file.not_found` vs `file_error`

## Localization Implementation

### Wrapper API Design

#### Primary Localization Function
```go
func T(key string, args ...interface{}) string
```

#### Optional Generic Wrapper
```go
func Generic(key string) string // Only for whitelisted common terms
```

#### Development Guidelines
- **Always use T()** for VT-specific content
- **Use Generic()** only for approved common terms (OK, Cancel)
- **Never mix systems** within single UI component
- **All domain/technical terms** use T()

### Fyne Integration Limits

#### Fyne Localization Scope
- File dialogs (open, save, folder selection)
- System error dialogs (where we have no control)
- Optional: whitelisted generic terms only

#### Fyne Translation Treatment
- Treated as **non-authoritative** conveniences
- Do not let Fyne keys leak into VT string IDs
- Never rely on Fyne for VT-specific terminology

## Quality Assurance

### Translation Requirements

#### All Languages
- Human translation required
- Context provided for all keys
- Terminology consistency enforced via glossary
- UI testing with actual translations

#### Indigenous Languages (Additional Requirements)
- **Human-reviewed by fluent speaker or language authority**
- **No machine translation**
- **Syllabics validation** for proper rendering
- **Cultural appropriateness review**
- **Community feedback** incorporated before release

### Testing Strategy

#### Automated Testing
- **Translation completeness**: All keys exist in each language
- **Key consistency**: No orphaned or deprecated keys
- **UI layout expansion**: Test with long strings (pseudo-language)
- **Script rendering**: Test syllabics and right-to-left support

#### Manual Testing
- **In-context verification**: All UI elements with actual translations
- **Module-by-module validation**: Each module tested independently
- **Language switching**: Dynamic switching functionality
- **Error conditions**: Error messages with all supported languages

## Contribution Guidelines

### Translation Contributions

#### General Languages
- Pull requests for new translations welcome
- Must follow key naming conventions
- Include context for ambiguous terms
- Maintain consistency with existing terminology

#### Indigenous Languages
- **Must include reviewer credit/approval**
- **Dual-script preferred** (syllabics + romanized)
- **Cultural notes** included for context
- **Community consultation** evidence encouraged

### Translation Review Process

#### Reviewer Requirements
- Fluent in target language
- Familiar with video processing terminology
- Understanding of Canadian context (for French/Indigenous languages)

#### Review Checklist
- [ ] Terminology consistency with glossary
- [ ] Cultural appropriateness
- [ ] Technical accuracy
- [ ] Script rendering correctness
- [ ] Context preservation

## Configuration Integration

### Language Preference Storage

#### Configuration File
```json
{
  "language": "en-CA",
  "secondaryScript": false,
  "autoDetect": true,
  "fallbackLanguage": "en-CA"
}
```

#### Fallback Chain
1. User-selected language
2. User-selected language + secondary script (if enabled)
3. English (en-CA)
4. Hardcoded emergency string

### Migration Path

#### Existing Configurations
- Language preference added to existing config files
- Backward compatibility maintained
- Graceful fallback to en-CA if language not set

## File Management

### Translation File Format

#### JSON Structure
```json
{
  "meta": {
    "language": "Inuktitut",
    "region": "CA",
    "script": "syllabics",
    "reviewer": "Inuit Language Authority",
    "version": "2024.01"
  },
  "translations": {
    "convert.output.format": "ᐃᓚᓴᐃᔭᕐᓂᖅ ᐱᓕᕆᔭᐅᔪᖅ",
    "common.button.ok": "ᐱᕙᕚᐅᖅᑐᖅ",
    "error.file.not_found": "ᐊᓂᑦ ᑎᑎᖃᕐᕕᒃ ᐱᓕᕆᐊᖅᑐᐃᓐᓇᐃᒃ"
  }
}
```

#### Validation Rules
- Valid JSON structure
- All required keys present
- No duplicate keys
- Proper UTF-8 encoding
- Script-appropriate characters

## Future Considerations

### Scalability
- Modular structure allows new language addition without core changes
- Separate files per module enable distributed translation work
- Metadata system supports language evolution

### Technical Debt Prevention
- Single authoritative system prevents fragmentation
- Semantic keys prevent coupling to specific languages
- Documentation ensures long-term consistency

### Community Growth
- Clear contribution guidelines enable community participation
- Review process maintains quality
- Indigenous language requirements ensure cultural respect

## Status

This policy is authoritative and binding for all VideoTools localization work. It applies to UI text, documentation, and all external-facing content.