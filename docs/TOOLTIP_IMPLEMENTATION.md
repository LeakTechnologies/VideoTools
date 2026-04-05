# Tooltip Implementation Plan

## Overview

Add tooltips to all interactive UI elements in VideoTools modules using the `github.com/dweymouth/fyne-tooltip` package.

## Dependency

- **Package**: `github.com/dweymouth/fyne-tooltip`
- **Version**: v0.4.0
- **Import**: `tooltip "github.com/dweymouth/fyne-tooltip/widget"`

## Usage

```go
import tooltip "github.com/dweymouth/fyne-tooltip/widget"

// On a button or other widget
myButton := widget.NewButton("Label", handler)
tooltip.SetToolTip(myButton, "Tooltip text here")
```

## Implementation Strategy

### Phase 1: Core UI Components (Priority: High)
Add tooltips to reusable components in `internal/ui/`:

1. **ModuleTile** (`components.go`) - Main menu module buttons
   - Label + short description of module purpose
   
2. **Queue tiles** (`queueview.go`) - Queue job items
   - Job type, status, input/output paths

3. **Header buttons** - Sidebar toggle, Files dropdown, Logs, Queue

### Phase 2: Module-Specific Controls (Priority: Medium)
Add tooltips to individual module views:

1. **Convert Module** - Format/quality selectors, output path, add file buttons
2. **Author Module** - Output type (DVD/BD/ISO), title, chapters
3. **Settings Module** - All preference toggles

### Phase 3: Secondary Controls (Priority: Low)
- Play/Pause/Stop in player
- Trim markers
- Filter sliders

## i18n Integration

All tooltip text must use i18n system:

```go
// Add to strings.go
TooltipModuleConvert     string // "Convert video formats and codecs"
TooltipModuleAuthor      string // "Create DVD/Blu-ray discs"
TooltipQueueOpenOutput   string // "Open output file or folder"

// Use in code
tooltip.SetToolTip(btn, t.TooltipModuleConvert)
```

## Files to Modify

1. `internal/i18n/strings.go` - Add tooltip keys
2. `internal/i18n/en_ca.go` - Add English translations
3. `internal/i18n/fr_ca.go` - Add French translations
4. `internal/i18n/iu.go` / `iu_latin.go` - Add Inuktitut
5. `internal/ui/components.go` - ModuleTile tooltips
6. `internal/ui/mainmenu.go` - Header button tooltips
7. `internal/ui/queueview.go` - Queue item tooltips
8. Individual module files (convert, author, settings, etc.)

## Design Principles

1. **Concise** - Tooltips should be 1-3 words for buttons, short phrase for modules
2. **Action-oriented** - Focus on what the user can DO, not just what it IS
3. **Consistent** - Same pattern across all modules
4. **No redundancy** - Don't add tooltips to already-labeled elements

## Examples

| Element | Tooltip |
|---------|---------|
| Convert module tile | "Convert formats" |
| Author module tile | "Create disc" |
| Start button | "Start queue" |
| Pause button | "Pause all jobs" |
| Output path | "Where files are saved" |
| Format dropdown | "Video file format" |