# Main.go Modularization - Incremental Refactoring Prompt

## Current State

**Problem:** main.go is 14,116 lines (420KB) causing:
- 5+ minute builds on Windows
- Poor code maintainability
- Difficult to navigate and modify

**Goal:** Extract modules into separate files following established patterns

**Already Extracted:**
- ✅ `author_module.go` (1,421 lines) - Pattern reference
- ✅ `subtitles_module.go` (756 lines) - Pattern reference
- ✅ `inspect_module.go` (292 lines) - Pattern reference

**Remaining:** 9 modules to extract (~10,000 lines)

---

## Extraction Order (Least Important → Most Important)

Work through these **one module at a time**, testing after each extraction:

### Phase 1: Simple Modules (Start Here)

1. **filters_module.go** (~236 lines)
   - Lines: ~13261-13497
   - Functions: `showFiltersView()`, `buildFiltersView()`
   - State fields: `filtersFile`, `filterBrightness`, `filterContrast`, etc.
   - Why first: Simple UI-only, no job execution

2. **upscale_module.go** (~889 lines + job executor)
   - Lines: ~13498-14387 + job executor at ~4568-5023
   - Functions: `showUpscaleView()`, `buildUpscaleView()`, `executeUpscaleJob()`, AI helpers
   - State fields: All `upscale*` fields (~25 fields)
   - Includes: AI backend detection, model helpers

3. **thumb_module.go** (~335 lines + job executor)
   - Lines: ~12837-13172 + executeThumbJob at ~4222-4264
   - Functions: `showThumbView()`, `buildThumbView()`, `executeThumbJob()`
   - State fields: `thumbFile`, `thumbCount`, `thumbWidth`, etc.

### Phase 2: Moderate Complexity

4. **player_module.go** (~87 lines + playSession ~450 lines)
   - Lines: ~13173-13260 + playSession at ~8929-9329
   - Functions: `showPlayerView()`, `buildPlayerView()`, `stopPlayer()`, entire `playSession` type
   - State fields: `playerFile`, `player`, `playerReady`, `playerVolume`, etc.
   - Types: `playSession`, `playerSurface`, `playSession` methods
   - Note: Includes recent performance improvements

5. **compare_module.go** (~503 lines + fullscreen)
   - Lines: ~12068-12571 + fullscreen at ~14277-14387
   - Functions: `showCompareView()`, `buildCompareView()`, `buildCompareFullscreenView()`, `showCompareFullscreen()`
   - State fields: `compareFile1`, `compareFile2`, `autoCompare`

### Phase 3: Complex Infrastructure

6. **merge_module.go** (~374 lines + job executor)
   - Lines: ~2752-3126 + executeMergeJob at ~3232-3564
   - Functions: `showMergeView()` (inline UI builder), `addMergeToQueue()`, `executeMergeJob()`
   - State fields: `mergeClips`, `mergeFormat`, `mergeOutput`, etc.
   - Types: `mergeClip` struct
   - Note: UI builder is inline, needs extraction to `buildMergeView()`

7. **benchmark_module.go** (~326 lines)
   - Lines: ~1938-2264 + config persistence at ~700-729
   - Functions: `showBenchmark()`, `showBenchmarkHistory()`, benchmark helpers
   - Types: `benchmarkConfig`, `benchmarkRun`

8. **queue_module.go** (~scattered functions)
   - Functions: `showQueue()`, `refreshQueueView()`, `jobExecutor()`, `buildFFmpegCommandFromJob()`
   - Lines: Various, needs gathering

### Phase 4: Most Critical (Do Last)

9. **convert_module.go** (~6,384 lines - THE BIG ONE)
   - Lines: ~5683-12067
   - Functions: 50+ functions including `buildConvertView()`, `showConvertView()`, all convert helpers
   - State fields: All `convert*` fields, `source`, `loadedVideos`, etc.
   - Types: `convertConfig`, `formatOption`
   - Shared components: `buildMetadataPanel()`, `buildVideoPane()` (used by other modules)
   - May need to split into multiple files if > 3000 lines

---

## Instructions for Each Module Extraction

### Step-by-Step Process

For each module (do ONE at a time):

#### 1. Create the Module File

**File:** `{module}_module.go` (e.g., `filters_module.go`)

**Template:**
```go
package main

import (
    // Copy imports from main.go that this module needs
    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/container"
    // ... etc
)

// Move the show{Module}View() method here
func (s *appState) show{Module}View() {
    s.stopPreview()
    s.lastModule = s.active
    s.active = "{module}"
    s.setContent(build{Module}View(s))
}

// Move the build{Module}View() function here
func build{Module}View(state *appState) fyne.CanvasObject {
    // ... entire function body ...
}

// Move any helper functions specific to this module
// Move any job executor functions (execute{Module}Job)
```

#### 2. Remove from main.go

Delete the exact same functions you just moved. Use line numbers as guide.

**IMPORTANT:** Do NOT remove:
- Shared functions used by multiple modules (e.g., `moduleColor()`, `probeVideo()`)
- State fields in `appState` struct (leave all state in main.go)
- Type definitions used across modules

#### 3. Build and Test

```bash
# Must succeed
go build -o VideoTools .

# Check line count reduction
wc -l main.go {module}_module.go

# Test the module works
./VideoTools  # Navigate to the module and test basic functionality
```

#### 4. Commit

```bash
git add {module}_module.go main.go
git commit -m "Extract {module} module from main.go

- Create {module}_module.go (XXX lines)
- Move show{Module}View() and build{Module}View()
- Reduce main.go by XXX lines
- All tests pass, module functions correctly
"
```

#### 5. Move to Next Module

Repeat the process for the next module in the order above.

---

## Reference Patterns

Look at these existing extracted modules for the pattern:

### Pattern 1: Simple Module (inspect_module.go)
```go
package main

import (
    // Necessary imports
)

func (s *appState) showInspectView() {
    s.stopPreview()
    s.lastModule = s.active
    s.active = "inspect"
    s.setContent(buildInspectView(s))
}

func buildInspectView(state *appState) fyne.CanvasObject {
    // UI building code
}
```

### Pattern 2: Module with Job Executor (subtitles_module.go)
```go
package main

import (
    // Necessary imports
)

func (s *appState) showSubtitlesView() {
    // ...
}

func buildSubtitlesView(state *appState) fyne.CanvasObject {
    // ...
}

func (s *appState) executeSubtitlesJob(ctx context.Context, job *queue.Job, progressCallback func(float64)) error {
    // Job execution logic
}

// Helper functions specific to subtitles
```

### Pattern 3: Module with Types (author_module.go)
```go
package main

import (
    // ...
)

// Types specific to this module
type authorClip struct {
    // ...
}

func (s *appState) showAuthorView() {
    // ...
}

// Many helper functions specific to author module
```

---

## What Stays in main.go

**DO NOT MOVE these to module files:**

### Core Application
- `main()` function
- `runGUI()` function
- `type appState struct` definition (all fields stay)
- Window setup and initialization

### Shared Utilities (Used by 3+ modules)
- `moduleColor()` - Color scheme helper
- `moduleFooter()` - Footer bar builder
- `statusStrip()` - Status bar
- `probeVideo()` - Video file analysis (CRITICAL - used everywhere)
- `buildMetadataPanel()` - Used by convert and other modules
- `buildVideoPane()` - Video preview pane (used by many modules)
- `formatBitrate()`, `formatClock()`, etc. - Format helpers

### Navigation & State
- `showMainMenu()` - Main menu builder
- `setContent()` - Content switcher
- History sidebar code
- Stats bar updates
- Drop handlers

### Shared Types
- `videoSource` type and methods
- Shared constants and enums

---

## Special Cases & Gotchas

### Convert Module (Do Last!)

The convert module is massive and may need splitting:

**Option A: Single File** (if < 3000 lines after extraction)
- `convert_module.go`

**Option B: Split into Multiple Files** (if > 3000 lines)
- `convert_module.go` - Main UI and show function
- `convert_config.go` - Configuration types and persistence
- `convert_job.go` - Job execution logic
- `convert_helpers.go` - Codec/format helpers

**Shared Components:**
- `buildMetadataPanel()` and `buildVideoPane()` might need to move to `shared_ui.go` if used by many modules

### Merge Module Inline UI

The merge module has UI building inline in `showMergeView()`. You'll need to:

1. Extract inline UI to `buildMergeView()` function
2. Then follow standard pattern

### Player Module with playSession

The `playSession` type and all its methods should move to `player_module.go` together.

### Import Adjustments

Some modules might need internal package imports that aren't in main.go:
- Check for missing imports after extraction
- Run `go build` to catch import errors
- Add necessary imports to the module file

---

## Success Criteria for Each Extraction

Before moving to next module, verify:

- ✅ `go build` succeeds with no errors
- ✅ Module file has clear structure (show function, build function, helpers)
- ✅ main.go reduced by expected lines
- ✅ Module works when tested in GUI
- ✅ No duplicate code between main.go and module file
- ✅ Git commit created with clear message
- ✅ Can navigate to module and use basic features

---

## Tracking Progress

After each extraction, update this checklist:

- [ ] 1. filters_module.go (~236 lines)
- [ ] 2. upscale_module.go (~1200 lines)
- [ ] 3. thumb_module.go (~400 lines)
- [ ] 4. player_module.go (~800 lines)
- [ ] 5. compare_module.go (~550 lines)
- [ ] 6. merge_module.go (~700 lines)
- [ ] 7. benchmark_module.go (~400 lines)
- [ ] 8. queue_module.go (~500 lines)
- [ ] 9. convert_module.go (~6400 lines - may split)

**Target:** main.go from 14,116 lines → ~4,000 lines

---

## Expected Timeline

- **Simple modules (1-3):** 20-30 min each
- **Moderate modules (4-5):** 30-45 min each
- **Complex modules (6-8):** 45-60 min each
- **Convert module (9):** 2-3 hours (largest, most critical)

**Total:** ~8-12 hours of incremental work

---

## Final Build Performance Goal

After all extractions:

**Before:**
- main.go: 14,329 lines (426KB)
- Build time (Windows): 5+ minutes

**After:**
- main.go: ~4,000 lines (~120KB)
- Module files: 10-12 files (~800-1500 lines each)
- Build time (Windows): < 2 minutes (hopefully < 1 minute)

---

## Questions to Ask If Stuck

1. **"Does this function use state from multiple modules?"**
   - Yes → Keep in main.go
   - No → Move to module file

2. **"Is this function called by 3+ different modules?"**
   - Yes → Keep in main.go as shared utility
   - No → Move to the module that uses it

3. **"Is this a type used across modules?"**
   - Yes → Keep type definition in main.go
   - No → Move to module file

4. **"Where are the line numbers for this module?"**
   - Check this document's module listing
   - Use grep to find function: `grep -n "func buildXView" main.go`

---

## Emergency Rollback

If an extraction breaks the build:

```bash
# Restore main.go from backup
git checkout HEAD -- main.go

# Remove the broken module file
rm {module}_module.go

# Try again with more care
```

---

## Start Here

**First module to extract:** `filters_module.go`

1. Find lines ~13261-13497 in main.go
2. Create `filters_module.go`
3. Copy `showFiltersView()` and `buildFiltersView()`
4. Remove from main.go
5. Build, test, commit
6. Move to next module

Good luck! Take it one module at a time, test after each extraction, and the convert module will be much easier once you've established the pattern with the simpler modules.
