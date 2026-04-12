# Audio Module Improvements Plan

## Overview

The audio module (`internal/app/modules/audio/`) has been largely ignored in implementation and is missing key features that would make it consistent with the Convert module and properly functional. This document outlines the improvements needed.

## Current State

### Files
- `audio_module.go` - Main module entry in `package main`
- `internal/app/modules/audio/view.go` - UI builder
- `internal/app/modules/audio/probe.go` - Audio track probing (stub)
- `internal/app/modules/audio/exec.go` - FFmpeg execution (stub)

### Issues Identified

1. **No video preview** - Can't see what video you're extracting audio from
2. **Inconsistent layout** - Uses custom `fixedHSplitLayout` instead of VSplit
3. **Limited track info** - Just codec/channel/sample rate
4. **Basic batch UI** - No sorting, filtering, or progress tracking
5. **No waveform** - Can't visualize audio content
6. **Missing drag-drop feedback** - Drop zone is just a label

---

## Phase 1: Layout Consistency (High Priority)

### 1.1 Replace VSplit with standard container.VSplash

**Current:**
```go
mainSplit := container.New(&fixedHSplitLayout{ratio: 0.5}, leftPanel, rightPanel)
```

**Target:**
```go
mainSplit := container.NewVSplit(leftPanel, rightPanel)
mainSplit.SetOffset(0.5)
```

**Files to modify:**
- `internal/app/modules/audio/view.go`

### 1.2 Use consistent module box styling

**Current:** Custom `buildAudioBox()` with manual styling

**Target:** Reuse `ui.NoisyBackgroundObjects()` pattern from Convert

**Files to modify:**
- `internal/app/modules/audio/view.go` - Replace `buildAudioBox()` with standard function

### 1.3 Add proper header bar

**Current:** Minimal header with back button

**Target:** Consistent with other modules:
- Module title with icon
- Queue status indicator
- Stats bar integration

**Files to modify:**
- `internal/app/modules/audio/view.go`

---

## Phase 2: Native Media Player Integration (High Priority)

### 2.1 Add InlineVideoPlayer to audio module

The audio module needs video preview capability like Convert.

**Architecture:**
1. Declare player singleton in `native_media.go` → `GetAudioPlayer()`
2. Add `Player *ui.InlineVideoPlayer` to Options in both real and stub files
3. Wire in `audio_module.go`
4. Build video pane using similar pattern to Convert

**Files to modify:**
- `native_media.go` - Add `audioInlinePlayer` singleton
- `native_media_stub.go` - Add stub
- `internal/app/modules/audio/view.go` - Add player to options
- `audio_module.go` - Wire player callbacks

### 2.2 Video pane layout in audio

**Structure:**
```
┌─────────────────────────────────────────────┐
│ Back Button    [Stats Bar]              │ ← Header
├─────────────────────────────────────────────┤
│                                         │
│  ┌─────────────┬─────────────────────────┐  │
│  │             │                         │  │
│  │  Video      │   Format Options       │  │
│  │  Preview   │   Quality             │  │
│  │             │   Bitrate            │  │
│  │  SMPTE     │   Normalize          │  │
│  │  Bars      │   Output Dir        │  │
│  │             │                         │  │
│  └─────────────┴───────────────���─────────┘  │
├─────────────────────────────────────────────┤
│ [Extract Now]  [Add to Queue]                │ ← Action bar
└─────────────────────────────────────────────┘
```

### 2.3 Idle state for video preview

When no file is loaded, show SMPTE color bars (like Convert):
- Use same `drawSMPTEBars()` from `internal/media/view.go`
- Set idle text: "DROP VIDEO TO LOAD"

**Files to modify:**
- `internal/app/modules/audio/view.go` - Add video preview container

---

## Phase 3: Track Selection Improvements (Medium Priority)

### 3.1 Enhanced track list display

**Current:** Simple checkboxes with basic info

**Target:**
- Track number badge (styled)
- Codec with icon
- Language flag (if available)
- Sample rate / bitrate
- Channels visual (icon)
- Duration
- Export format preview

**Files to modify:**
- `internal/app/modules/audio/view.go` - `buildTrackList()`

### 3.2 Track reordering for multi-track export

Allow users to set export order for files with multiple tracks.

**UI:**
- Drag handle on each track
- Up/Down buttons
- "Move to Top" / "Move to Bottom"

**Files to modify:**
- `internal/app/modules/audio/view.go` - UI components

### 3.3 Output naming preview

Show what the output filename will be before extraction.

**Format:** `{basename}_track{index}_{language}.{ext}`

**Files to modify:**
- `internal/app/modules/audio/view.go` - Add preview label

---

## Phase 4: Batch Processing (Medium Priority)

### 4.1 Enhanced batch list

**Current:** Simple list with remove button

**Target:**
- File type icon (video format badge)
- Duration display
- Size display
- Audio track count badge
- Status indicator (pending/processing/done/error)
- Progress per file (when running)

**Files to modify:**
- `internal/app/modules/audio/view.go` - `buildBatchList()`

### 4.2 Batch sorting

Add sorting options:
- By name (A-Z, Z-A)
- By duration
- By size
- By status

### 4.3 Batch progress display

When batch is running, show:
- Current file being processed
- Overall progress (files completed / total)
- Per-file progress
- Estimated time remaining

---

## Phase 5: Audio Visualization (Low Priority)

### 5.1 Waveform display

Show audio waveform in the file info panel.

**Implementation options:**
1. Generate waveform on file load (async)
2. Show simplified waveform image
3. Use audio visualization during playback (if video has audio)

**Files to modify:**
- `internal/app/modules/audio/probe.go` - Waveform generation
- `internal/app/modules/audio/view.go` - Display

### 5.2 Audio level meters

During extraction, show:
- Input level (source)
- Output level (normalized)

---

## Phase 6: Output Handling (Medium Priority)

### 6.1 Output directory improvements

**Current:** Basic entry + browse

**Target:**
- Recent output directories dropdown
- "Open folder" button after extraction
- Create subdirectory option (e.g., `{filename}/audio`)
- Conflict handling: skip/overwrite/rename

### 6.2 Filename template

Allow custom naming:

**Tokens:**
- `{filename}` - Original basename
- `{track}` - Track number
- `{language}` - ISO code
- `{codec}` - Audio codec

**UI:**
- Custom format input with token buttons
- Live preview

---

## Phase 7: Consistency with Convert Module

### 7.1 Metadata panel

Add similar metadata display to Convert:

**Current:** "File: X, Duration: Y, Format: Z"

**Target:**
- Full video metadata
- Audio track metadata panel
- Chapter list (if applicable)
- Container format
- File size
- Bitrate

### 7.2 Module footer

Add consistent stats bar (per-file info):
- Processing time
- Input size → Output size
- Compression ratio

### 7.3 i18n strings

Add missing localized strings:
- Audio instructions placeholders
- Status messages
- Error messages

---

## Implementation Order

### Suggested sequence:

1. **Week 1:** Phase 1 (Layout consistency)
2. **Week 2:** Phase 2 (Native player integration)
3. **Week 3:** Phase 3 (Track selection)
4. **Week 4:** Phase 4 (Batch processing)
5. **Week 5:** Phase 5 (Visualization) - optional
6. **Week 6:** Phase 6 (Output handling)
7. **Week 7:** Phase 7 (Consistency)

---

## Testing Checklist

- [ ] Drag-drop video loads and displays in preview
- [ ] SMPTE bars show when no video loaded
- [ ] Track selection persists to extraction
- [ ] Batch mode adds multiple files
- [ ] Batch extraction runs sequentially
- [ ] Progress updates correctly
- [ ] Output directory is created if missing
- [ ] Named output files match template
- [ ] Module switching cleans up audio resources
- [ ] Window focus is maintained

---

## File Reference

### Files to modify (in order):

1. `internal/app/modules/audio/view.go` - Main UI
2. `audio_module.go` - Callbacks and state
3. `native_media.go` - Player singleton
4. `native_media_stub.go` - Player stub
5. `internal/i18n/strings.go` - Add audio strings
6. `internal/i18n/en_ca.go` - English translations
7. `internal/i18n/fr_ca.go` - French translations
8. `internal/i18n/iu.go`, `iu_latin.go` - Inuktitut

### New files to create:

1. `internal/app/modules/audio/waveform.go` - Waveform generation (Phase 5)
2. `internal/app/modules/audio/naming.go` - Filename template (Phase 6)

---

## Notes

- Audio module shares similarity with Convert - leverage existing patterns
- Use `ui.InlineVideoPlayer` for video preview (same API as Convert)
- Keep batch processing UI simple initially - enhance after core works
- Focus on Phase 1-2 for maximum impact with minimal effort