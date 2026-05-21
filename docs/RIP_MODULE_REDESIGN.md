# Rip Module Redesign — Menu Handling, Chapters & Multi-Title Export

## Problem Summary

User report (Lusterne Teenies rip): menus bleed into the ripped video, chapters are missing from output, and there is no way to preserve menus separately or differentiate main feature from extras in multi-title exports.

## Root Cause Analysis

### #1 — Menu VOB bleeding into video

`internal/app/modules/rip/executor.go:CollectVOBSets()` groups all VOB files starting with `VTS_` by their `VTS_XX` prefix. This includes `VTS_XX_0.VOB` (the menu VOB) alongside content VOBs (`VTS_XX_1.VOB` onward). When sorted alphabetically, `VTS_01_0.VOB` comes before `VTS_01_1.VOB`, so the menu data is concatenated at the start of the output.

**Impact on chapters**: When the menu VOB is prepended, the chapter timestamps read from the VTS IFO (which only describe the content PGC, not the menu) are shifted by the menu duration. Chapters appear at the wrong times or FFmpeg discards them entirely due to mismatched timing.

**Fix**: Filter `VTS_XX_0.VOB` out of content VOB sets. Menu VOBs are moved to a separate collection for optional menu-only export.

### #2 — Chapters not appearing in output

The chapter data flow is correct in theory:
1. `ifo.ReadTitleInfo()` → `TitleInfo.Chapters []float64` (start times in seconds)
2. `WriteChapterFile()` → ffmetadata `[CHAPTER]` blocks
3. `-f ffmetadata -i chapters.txt -map_chapters 1` → FFmpeg copies chapters to output

The chapter data was likely correct but *shifted* by the included menu VOB duration (see #1). Fixing #1 should resolve this.

Additionally, the chapter names are generic ("Chapter 1", "Chapter 2"). Some discs embed names in the PTT_SRPT; we should try to extract them.

### #3 — No menu preservation option

Menu VOBs are either silently included (concat path glitch) or dumped into the main video (dvdvideo path issue). There is no UI toggle to preserve menus as separate files.

### #4 — No main/extra title differentiation

Every title gets the same `_Title_NN` suffix. The main feature should use the main output path; extras should use `_Extra`.

## Design

### Stage 1: Content VOB filtering (BUG FIX)

**File**: `internal/app/modules/rip/executor.go`

`CollectVOBSets`: add a suffix check. VOBs with `_0` suffix (menu VOBs for a VTS) are excluded from content VOB sets. A new `CollectMenuVOBs()` function returns all menu VOBs across all VTS sets plus `VIDEO_TS.VOB`.

No UI changes needed. This fixes both the menu glitch and chapter misalignment in one change.

### Stage 2: Chapter debugging

**File**: `internal/app/modules/rip/executor.go`

Add verbose logging to the chapter extraction path:
- Log chapter count and first/last timestamp
- Log the metafile path being passed to FFmpeg
- Log the actual ffmpeg args for inspection

### Stage 3: Menu preservation option

**File**: `internal/app/modules/rip/view.go`, `executor.go`, `types.go`, `modulecfg/rip.go`

Add `includeMenus bool` to:
- `viewState` — UI state
- `RipConfig` — persisted config
- `ExecuteOptions` — executor option

UI: Add "Preserve menus" checkbox in the enrichment section.
- Default: off (menus are skipped)
- When on: menu VOBs are extracted separately

Executor: When `includeMenus` is true:
1. Collect all menu VOBs (VTS_XX_0.VOB + VIDEO_TS.VOB)
2. For each menu VOB, create a separate ffmpeg job
3. Output named `{base}_Menu.vob` or `{base}_Menu.mkv` depending on format
4. Skip menu VOBs when `includeMenus` is false

### Stage 4: Main/extra title naming

**File**: `internal/app/modules/rip/view.go`

In `addToQueue()`, when building multi-title jobs:
1. Determine the main feature: the title with the longest duration
2. Main feature → `vs.outputPath` (no suffix)
3. Other titles → `_Extra` prefix instead of `_Title`

When there are 2+ titles, the first title in TT_SRPT is often the main feature (usually the longest). Use duration as a tiebreaker.

### Stage 5: Documentation

Update all 6 docs before/after each change.

## Implementation Order

1. Write this design doc ✓
2. Fix `CollectVOBSets` to filter menu VOBs
3. Add chapter debug logging
4. Add preserve-menus option
5. Add main/extra naming
6. Commit all changes + update roadmap docs
