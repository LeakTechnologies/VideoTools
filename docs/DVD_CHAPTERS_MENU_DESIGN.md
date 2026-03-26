# DVD Multi-Page Chapters Menu Design

## Current State

### What's Implemented
1. **Dynamic Pagination** (`CalculateOptimalPaging`)
   - Balances chapters across pages based on available height
   - Examples: 9 chapters → [6,3], 14 chapters → [5,5,4]
   
2. **Page-Aware Button Building** (`buildChapterMenuButtonsForPage`)
   - Creates buttons for specific page
   - Supports Next/Prev navigation buttons
   - IsNav/NavType fields on dvdMenuButton

3. **Thumbnail Overlay** 
   - Re-enabled with simplified FFmpeg filter chain
   - Positioned right of chapter text (x=420-450)
   
4. **Thumbnail Modes**
   - Start: chapter timestamp + offset
   - Midpoint: chapter timestamp + (duration/2)
   - Custom: chapter timestamp + user-defined offset

---

## Architecture Overview

### DVD Menu Structure

```
VIDEO_TS.VOB (Menu Disc)
├── PGC 1: Main Menu (Play, Chapters, Extras)
├── PGC 2: Chapters Menu Page 1 (chapters 1-N)
├── PGC 3: Chapters Menu Page 2 (chapters N-M) [if needed]
├── ...
└── PGC X: Extras Menu [if enabled]
```

### Key Concepts

1. **PGC (Program Chain)**: A sequence of cells with navigation commands
2. **Cell**: A playback unit within a PGC
3. **Button**: Interactive element with highlight states

---

## Implementation Plan

### Phase 1: Single Menu Support (Complete)
- ✅ Generate single chapters page
- ✅ Build PGC with chapter buttons
- ✅ Thumbnail overlays

### Phase 2: Multi-Page Chapters (Complete)

#### Step 2.1: Update Data Structures
```go
type dvdMenuSet struct {
    MainMpg         string
    MainButtons     []dvdMenuButton
    ChaptersMpg     string              // Base path for chapters menus
    ChaptersButtons [][]dvdMenuButton   // [[page1 buttons], [page2 buttons], ...]
    ChaptersPages  int                 // Number of chapter pages
    ExtrasMpg       string
    ExtrasButtons   []dvdMenuButton
}
```

#### Step 2.2: Generate Multiple Menu Pages
```go
// In buildDVDMenuAssets
chaptersPerPage := CalculateOptimalPaging(len(chapters), height, headerHeight, buttonHeight, buttonSpacing)

for pageIndex, chapterCount := range chaptersPerPage {
    chapterStart, _ := GetChapterRangeForPage(chapters, chaptersPerPage, pageIndex)
    
    buttons := buildChapterMenuButtonsForPage(
        chapters,
        chapterStart,
        chapterCount,
        width, height,
        pageIndex > 0,           // hasPrevPage
        pageIndex < len(chaptersPerPage)-1, // hasNextPage
        pageIndex,
    )
    
    // Build thumbnail paths for this page's chapters
    thumbPaths := generateChapterThumbnails(...)
    
    // Build background with thumbnails
    bgPath := filepath.Join(workDir, fmt.Sprintf("chapters_menu_%d_bg.png", pageIndex+1))
    buildChaptersMenuBackground(ctx, bgPath, title, buttons, width, height, theme, thumbPaths, logFn)
}
```

#### Step 2.3: Build Multi-PGC VIDEO_TS.VOB ✅ Complete
Implementation in `author_module.go:3478-3520`:
```go
// Concatenate all menu MPGs into VIDEO_TS.VOB
menuFiles := []string{menuSet.MainMpg}
menuFiles = append(menuFiles, menuSet.ChaptersMpgs...)

// Process main menu (PGC 1) + chapter pages (PGC 2+)
for pageIdx, chapterMpg := range menuSet.ChaptersMpgs {
    concatenateMenuFile(outFile, chapterMpg)
    buttons := menuSet.ChaptersButtons[pageIdx]
    // Build PGC for this chapter page
    menuPGCs = append(menuPGCs, ifo.BuildMenuPGC(cmdTable, menuDuration, isNTSC))
}
```

#### Step 2.4: Update IFO Generation ✅ Complete
- `WritePGCITIs()` in `internal/dvd/ifo/pgc.go:213-260` handles multiple PGCs
- `GenerateVMG_IFO()` now accepts `[]*ProgramChain` instead of single PGC
- VMG_PGCITI_Offset updated correctly for multi-PGC layout

---

## Navigation Command Reference

DVD navigation commands for menu buttons:
```
"jump title 1;"              - Play title 1 from start
"jump title 1 chapter N;"    - Play title 1, chapter N
"jump menu 1;"               - Jump to main menu (PGC 1)
"jump menu 2;"               - Jump to chapters menu first page
"jump menu N;"               - Jump to PGC N in VIDEO_TS.VOB
"call menu N;"               - Call menu (returns to current position after)
```

---

## Dynamic Paging Algorithm

```go
// CalculateOptimalPaging(totalChapters, maxHeight, headerHeight, buttonHeight, buttonSpacing)
// Returns: []int where each element is chapter count for that page

Examples:
- 9 chapters, max 6 per page: [6, 3]      // Page 1: 6, Page 2: 3
- 14 chapters, max 6 per page: [5, 5, 4] // Balanced distribution
- 3 chapters, max 6 per page: [3]         // Single page
```

---

## UI Integration

### Settings Needed
1. **Chapter Thumbnails** (existing)
   - Enable/disable
   - Source video selection
   - Offset (start/midpoint/custom)
   
2. **Menu Structure** (new)
   - Chapters per page: Auto / Manual (6, 8, 10, 12)
   - This would override the CalculateOptimalPaging for user control

---

## Testing Checklist

- [ ] 1-3 chapters: Single page, no nav buttons
- [ ] 6 chapters: Single page, no nav buttons
- [ ] 7 chapters: 2 pages (6+1 or 4+3 with balanced algorithm)
- [ ] 14 chapters: 3 pages balanced
- [ ] All thumbnails render correctly
- [ ] Next/Prev navigation works
- [ ] Back button returns to main menu
- [ ] Chapter selection plays correct chapter
- [ ] Build completes without errors
- [ ] VIDEO_TS.VOB contains all menu pages concatenated
- [ ] VMG_PGCITI contains all PGCs with correct offsets

---

## Files Modified (Phase 2 Complete)

1. **`author_menu.go`** ✅
   - `dvdMenuSet` structure updated (line 528)
   - `buildChaptersMenuMPEGSet` returns `[]string, [][]dvdMenuButton` (line 652)
   - `buildChapterMenuButtonsForPage` with nav support (line 787)
   - `CalculateOptimalPaging` helper (line 728)
   - Thumbnail overlay with ChapterThumbMode (line 1583)

2. **`author_module.go`** ✅
   - Menu VOB concatenation (line 3478-3520)
   - Multi-PGC building loop (line 3488-3519)
   - `concatenateMenuFile` helper (line 3743)
   - Updated `GenerateVMG_IFO` calls (lines 3568, 3708)

3. **`internal/dvd/ifo/pgc.go`** ✅
   - `WritePGCITIs()` for multiple PGCs (line 213)
   - `BuildMenuPGC()` unchanged, reused

4. **`internal/dvd/ifo/builder.go`** ✅
   - `GenerateVMG_IFO()` signature updated to accept `[]*ProgramChain` (line 145)
   - PGCITI serialization updated (line 164)

5. **`author_dvd_functions.go`** ✅
   - Fixed non-constant format string (line 47)
