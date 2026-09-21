# Rip a Title by Chapter Range (dev81)

**Status:** active (dev81)
**Type:** Rip module capability
**Core ask:** *"rip a title by chapters, so we could rip just a specific scene if we need to"* — select a start and end chapter of a title and rip only that span.

## Overview

Every ripped title can already be restricted to a contiguous chapter span. Instead of ripping
a whole movie (or a whole scene-segment title), the Rip view gains a **"Rip chapters only"**
option with **From chapter** / **To chapter** selects. The rip then produces only the chosen
chapters:

- the **dvdvideo** (`-f dvdvideo`) path seeks the input to the start chapter with output-side
  `-ss <startSec>` and bounds it with `-to <endSec>`;
- the **VOB-concat** path slices the VOBs to exactly the cells that belong to the selected
  programs (chapter range → PGC program map entry cells → cell byte ranges);
- the embedded chapter metadata is **remapped** so the output's chapters describe the range
  (Chapter 1 = range start), matching the trimmed timeline.

## Motivation / use cases

- A disc has one multi-hour title and the tester only wants a specific scene, not the whole
  movie. Scene-segmented titles already cover some discs, but a disc without per-scene titles
  leaves only whole-title rips.

## Design

### Program → cell mapping (IFO)

`ifo.TitleInfo` already exposes `Chapters []float64` (program start times) and `Cells`
(VOBID/CellID/sector extents). The link between them — the PGC **program map** (one byte per
program = 1-based entry cell number) — is read in `readChapters` but not retained.

New field: `TitleInfo.ProgramEntryCells []int` — `ProgramEntryCells[p]` = entry cell number
(1-based) of program `p`. Filled in `readChapters` from the already-read `progMap`.

### Chapter range semantics

`ExecuteOptions.ChapterStart` / `ChapterEnd` (1-based program numbers, inclusive;
`0` = whole title = today's behaviour). For a title with `N` chapters:

- `base    = Chapters[ChapterStart-1]`
- `endSec  = Chapters[ChapterEnd]` when `ChapterEnd < N`, else `Duration`
- `rangeDur = endSec - base`
- chapters in range = `{ Chapters[cs-1]-base, …, Chapters[ce-1]-base }`

The range is clamped at execute time: `ChapterStart` stuck to `[1, N]`, `ChapterEnd` to
`[ChapterStart, N]`. Titles with fewer chapters than the UI's selects simply clamp.

### ffmpeg identity

`RipArgs` gains:

- `ChapterStartSec float64` / `ChapterEndSec float64` — output-side `-ss <start>` / `-to <end>`
  emitted after the inputs (valid for both the dvdvideo input and the concat input). ffmpeg
  normalises the output timeline to start at 0, so the remapped chapter file stays aligned.

### VOB-concat cell restriction

`cellConcatList` gains the range: when active, cells are restricted to the span
`[ ProgramEntryCells[cs-1]-1 , upperEntry )` where `upperEntry = ProgramEntryCells[ce]` if
`ce < N`, else the last cell. The "cells cover the whole VOB set" short-circuit is skipped
when a range is active (whole-file concat would rip the whole title, not the span). When a
range is active but no cells are resolvable, the executor does NOT treat slicing as
inapplicable — it keeps the whole-file list and relies on output-side `-ss`/`-to` instead.

### Engine wiring (summary)

- **Executor** (`internal/app/modules/rip/executor.go`)
  - decode the range from `ExecuteOptions` once `titleInfo` is available; clamp; set
    `ra.ChapterStartSec/EndSec`; compute `rangeDur`.
  - chapter metafile: when a range is active, write the **remapped** chapter list with
    `totalDuration = rangeDur` (single `WriteChapterFile` call, `RipArgs.MetaFile` unchanged).
  - progress duration `dur` = `rangeDur` when active.
  - both `cellConcatList` call sites (primary concat path + dvdvideo-failure retry) pass the
    range; when a cell-sliced list is engaged, clear `-ss`/`-to` (cells already bound the span —
    double trimming would shave extra frames).
  - failover stale-PTS cap: when a range is active and cells are engaged, override the cap to
    `ceil(rangeDur + 5)`; when cells are not engaged the `-to` bounds it and the existing cap
    logic still guards phantom tails.
- **Scan** (`internal/app/modules/rip/scan.go`) — `DiscTitle` gains `Chapters []float64`
  (carried from the per-(VTS,TTN) IFO read) so the view can bound the chapter selects per title.
- **View** (`internal/app/modules/rip/view.go`) — new "Rip chapters only" checkbox + From/To
  `widget.Select` (options `1…N`). Bounds: main-title chapter count in main mode; max chapter
  count over selected titles in choose-titles mode; hidden/disabled when the range cannot apply
  (no scan, no chapters, full-disc / region-conversion mode). Readiness summary appends
  "chapters _a_–_b_" when active. Per-title jobs carry `chapterStart`/`chapterEnd` config keys.
- **Job decode** (`rip_module.go`) — `executeRipJob` maps `chapterStart`/`chapterEnd` → opts.
- **i18n** — new keys (`RipChapterOnly`, `RipChapterFrom`, `RipChapterTo`, range format) across
  en/fr/iu/iu_latin.

### Interaction with existing options

- **Embed chapters** still applies — the embedded markers become the range's chapters.
- **Region conversion** forces full-disc extraction where the chapter UI is hidden.
- **Full-disc** mode never shows the chapter controls.
- The range is a **per-rip temporary state** (like extractMode / regionConvert), not persisted
  to `modulecfg.RipConfig`.

## Files

| File | Change |
|---|---|
| `internal/dvd/ifo/extract.go` | `TitleInfo.ProgramEntryCells []int`, filled from `progMap` |
| `internal/app/modules/rip/executor.go` | `RipArgs.ChapterStartSec/EndSec`, `-ss`/`-to` in `BuildRipArgs`, range decode/clamp/remap/`dur` in `Execute`, failover cap override |
| `internal/app/modules/rip/cellconcat.go` | range-aware cell filtering, skip whole-cover short-circuit when ranged |
| `internal/app/modules/rip/types.go` | `ExecuteOptions.ChapterStart/ChapterEnd`, `DiscTitle.Chapters` |
| `internal/app/modules/rip/scan.go` | carry `Chapters` into `DiscTitle` |
| `internal/app/modules/rip/view.go` | "Rip chapters only" + From/To selects + readiness + job Config |
| `rip_module.go` | decode `chapterStart`/`chapterEnd` |
| `internal/i18n/{strings,en_ca,fr_ca,iu,iu_latin}.go` | new keys |

## Testing checklist

- [ ] Multi-chapter title, H.264 MKV: enable "Rip chapters only", From 2 To 4 → output contains
      only chapters 2–4 (embedded chapter markers start at 0/Chapter 2 times); log shows the
      `-ss`/`-to` command on the dvdvideo path.
- [ ] Same range on a disc whose dvdvideo opens but the rip is forced to the concat fallback →
      log shows the cell-accurate list; output duration ≈ sum of range cells, NOT the whole title.
- [ ] From = To (single chapter) rips exactly that chapter.
- [ ] From 1 To N (whole title) ≈ the plain rip.
- [ ] A title with fewer chapters than a previous title's selects → executor clamps, no error.
- [ ] Readiness line + queued job Config agree with the From/To selects.
- [ ] Chapter-only with **Embed chapters off** still trims; title metadata intact.
- [ ] Reset/Clear ISO clears the chapter range.
- [ ] Scene-segment mode + chapter range compose (segment title + its own chapters).