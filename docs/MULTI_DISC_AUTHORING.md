# Multi-Disc Authoring Design

## Overview

Multi-disc authoring allows users to split a large title set (e.g., a TV season, compilation, or long-form content) across multiple DVD or Blu-ray discs. Each disc is a self-contained, playable volume with its own menus, navigation, and IFO structure. The feature exposes a "multi-volume" mode in the Author module where users assign clips to specific discs and generate each volume independently.

## Use Cases

- **TV seasons** — A 24-episode season split across 3-4 DVD volumes, each holding ~6-8 episodes with full menu navigation.
- **Compilation sets** — A curated collection (e.g., best-of, anthology, retrospective) distributed across multiple discs with consistent theming.
- **Long-form content** — Documentaries, concert recordings, or event coverage that exceeds single-disc capacity.

## Features

### Volume Management
- **Volume count** — User selects how many discs the set spans (2-9, matching DVD VTS numbering limits).
- **Per-volume clip assignment** — Drag-and-drop or list-based assignment of clips to specific volumes.
- **Capacity indicator** — Each volume shows estimated used space vs. disc capacity (DVD-5: 4.7 GB, DVD-9: 8.5 GB, BD-25: 25 GB, BD-50: 50 GB).
- **Auto-balance** — Optional auto-distribute clips across volumes to equalize duration or file size.

### Menu Consistency
- **Shared theme** — All volumes use the same menu theme, fonts, and color scheme.
- **Volume labeling** — Each disc's menu shows volume number (e.g., "Disc 1 of 3", "Volume 2/4").
- **Cross-disc navigation** — "Next Disc" / "Previous Disc" buttons in menus (optional; requires manual disc swap).

### Output
- **Per-volume folders** — Each disc generates its own `VIDEO_TS/` (or `BDMV/`) folder.
- **Naming convention** — `Disc_01/VIDEO_TS/`, `Disc_02/VIDEO_TS/`, etc.
- **ISO generation** — Optional per-volume ISO output with volume labels (`SHOW_S01_D1`, `SHOW_S01_D2`).
- **Batch compile** — Single "Compile All" action generates all volumes sequentially.

## Technical Implementation

### Data Model

```go
type MultiVolumeSet struct {
    Title       string          // Overall set title (e.g., "Show - Season 1")
    Volumes     []VolumeConfig  // Per-volume configuration
    Theme       MenuTheme       // Shared menu theme
    OutputType  string          // "dvd", "bluray", "iso"
}

type VolumeConfig struct {
    VolumeNumber int            // 1-based volume number
    Clips        []ClipEntry    // Clips assigned to this volume
    Label        string         // Volume label (e.g., "Disc 1 of 3")
    EstimatedSize int64         // Estimated output size in bytes
}
```

### Author Module Changes

1. **Volume selector** — New UI element in the Author toolbar to switch between volumes.
2. **Per-volume clip list** — Each volume maintains its own clip list; drag-and-drop reassignment between volumes.
3. **Capacity bar** — Visual indicator showing used/available space per volume.
4. **Batch compile button** — Replaces single "Compile" with "Compile All Volumes" when multi-volume mode is active.

### Pipeline Changes

The existing `runAuthoringPipeline` function already generates a single disc. Multi-disc extends this:

```
For each volume:
  1. Encode clips to MPEG-2 (one .mpg per clip)
  2. Build VTS IFO/BUP for the volume
  3. Build VMG IFO/BUP with volume-specific metadata
  4. Generate menu VOB with volume label
  5. Output to Disc_NN/VIDEO_TS/
```

Each volume is independent — no cross-volume IFO references. The only shared state is the menu theme and overall title metadata.

### DVD-Specific Considerations

- **VTS numbering** — Each volume starts with VTS 1. No cross-volume VTS references.
- **VMG identifiers** — Each volume's `VMG_Identifier` is "DVDVIDEO-VMG" (standard).
- **Volume labels** — Set via UDF/ISO 9660 volume descriptor during ISO creation.
- **Disc count metadata** — `Nr_Of_Volumes` in VMG_MAT can indicate total volumes (optional; not required for playback).

### Blu-ray Considerations

- BDMV structure is similarly per-volume.
- Each volume gets its own `BDMV/` folder with independent `index.bdmv` and `MovieObject.bdmv`.

## Files to Modify

1. **`author_module.go`** — Add volume management UI, per-volume clip lists, capacity indicators.
2. **`author_menu.go`** — Support volume labeling in menu generation.
3. **`internal/dvd/ifo/vmgi.go`** — `Nr_Of_Volumes` field already exists in `VMG_MAT`; wire it to user-configurable value.
4. **`internal/dvd/udf/`** — Volume label support in ISO generation.
5. **`internal/i18n/`** — New strings for volume management UI.

## UI Sketch

```
┌─────────────────────────────────────────────────────────┐
│ Author  │ [◀ Disc 1 of 3 ▶]  [Capacity: 3.2/4.7 GB]   │
├─────────────────────────────────────────────────────────┤
│ Clips:                                                  │
│  1. Episode 01 (00:22:15)                               │
│  2. Episode 02 (00:21:48)                               │
│  3. Episode 03 (00:23:01)                               │
│  4. Episode 04 (00:22:33)                               │
│  5. Episode 05 (00:21:55)                               │
│  6. Episode 06 (00:22:10)                               │
│                                                         │
│  [Add Clip] [Remove] [Move to Disc ▶]                   │
├─────────────────────────────────────────────────────────┤
│ [Preview] [Compile This Disc] [Compile All Discs]       │
└─────────────────────────────────────────────────────────┘
```

## Testing Checklist

- [ ] Single-volume mode unchanged (backwards compatible)
- [ ] Two-volume set generates two independent VIDEO_TS folders
- [ ] Each volume plays correctly in standalone DVD player
- [ ] Capacity indicator updates as clips are added/removed
- [ ] Auto-balance distributes clips reasonably
- [ ] Menu theme applies consistently across all volumes
- [ ] Volume labels appear in ISO output
- [ ] Cross-volume "Next Disc" button works (shows info dialog if disc not present)
