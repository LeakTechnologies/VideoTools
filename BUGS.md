# Bug Tracker

Track all bugs, issues, and behavioral problems here. Update this file whenever you discover or fix a bug.

**Last Updated**: 2026-01-06 19:35 UTC

---

## 🔴 Critical Bugs (Blocking Functionality)

### BUG-005: CRF quality settings not showing when CRF mode is selected
- **Status**: 🔴 OPEN
- **Reporter**: User (2026-01-04 21:10)
- **Module**: Convert
- **Description**: When user selects "CRF (Constant Rate Factor)" as the Bitrate Mode, the Quality Preset dropdown does not appear. The UI remains empty where the quality settings should be.
- **Steps to Reproduce**:
  1. Open Convert module
  2. Set Bitrate Mode to "CRF (Constant Rate Factor)"
  3. Expected: Quality Preset dropdown appears
  4. Actual: No quality controls show up
- **Impact**: High - Cannot adjust quality in CRF mode, which is the default and most common mode
- **Root Cause**: Likely over-corrected the visibility logic in `updateQualityVisibility()` function. Made rules too strict after fixing BUG-001 and BUG-002.
- **Investigation Notes**:
  - CBR mode is set by default (works correctly - hides quality controls)
  - Previous fixes made `updateQualityVisibility()` check for: `hide || hideQuality || remux`
  - Where `hideQuality = mode != "" && mode != "CRF"`
  - Need to verify the logic flow when switching to CRF mode
- **Files to Check**:
  - `main.go:8851-8883` - `updateQualityVisibility()` function
  - `main.go:8440-8532` - `updateEncodingControls()` function
  - `main.go:7875-7888` - Bitrate mode selection callback
- **Assigned To**: opencode (handoff from Claude)
- **Verified**: No (not fixed yet)

### BUG-006: Windows app crashes mid-conversion with no logs
- **Status**: 🔴 OPEN
- **Reporter**: User (2026-01-05)
- **Module**: Convert / Queue / Logging (Windows)
- **Description**: VT crashes during batch conversions (typically the 3rd or 4th job). FFmpeg continues running, but the app exits and no logs are created afterward.
- **Steps to Reproduce**:
  1. On Windows, queue multiple conversions in Convert module
  2. Start queue
  3. After several jobs, app crashes while FFmpeg keeps running
  4. Expected: App stays alive, logs updated
  5. Actual: App crashes, logs missing
- **Impact**: High - data loss in UI state, no diagnostics
- **Root Cause**: Unknown. Suspected background goroutine / UI thread issue or log path failure on Windows.
- **Investigation Notes**:
  - Need to confirm whether `%APPDATA%\VideoTools\logs\videotools.log` is created at all
  - Crash likely happens before log writes or after log file handle is lost
- **Files to Check**:
  - `internal/logging/logging.go` (init + panic handling)
  - `main.go` (conversion execution, queue updates)
  - Windows build path for `getLogsDir()`
- **Assigned To**: Unassigned
- **Verified**: No

---

## 🟠 High Priority Bugs (Major Issues)

### BUG-010: Upscale jobs show no progress during FFmpeg conversions
- **Status**: ✅ FIXED (2026-01-06, needs verification)
- **Reporter**: Jake (2026-01-06)
- **Module**: Upscale / Queue
- **Description**: Upscale jobs that rely on FFmpeg conversions stay at 0.0% progress even while running; status updates do not advance until completion.
- **Steps to Reproduce**:
  1. Run Upscale job that uses FFmpeg conversion
  2. Observe queue progress
  3. Expected: Progress updates during conversion
  4. Actual: Progress remains 0.0% until completion
- **Impact**: High - No feedback during long-running jobs
- **Root Cause**: FFmpeg progress parsing relied on stderr time updates; piped output used CR-only updates.
- **Files to Check**:
  - `main.go:executeUpscaleJob` (progress pipe parsing)
- **Fixed By**: Codex
- **Verified**: No

---

## 🟡 Medium Priority Bugs (Annoying but Workable)

None currently open.

---

## 🟢 Low Priority Bugs (Minor Issues)

None currently open.

---

## 🧭 Feature Requests (Planned)

### FEAT-003: Enhancement module blur control
- **Status**: 🧭 IN PROGRESS
- **Reporter**: Jake (2026-01-06)
- **Module**: Enhancement
- **Description**: Enhancement panel should include a blur control in addition to sharpen/denoise.
- **Impact**: Medium - expected control in enhancement workflow
- **Notes**: Implemented blur controls in Upscale first; Enhancement UI still pending.

### FEAT-004: Upscale output quality should use Bitrate Mode controls
- **Status**: 🧭 PLANNED
- **Reporter**: Jake (2026-01-06)
- **Module**: Upscale
- **Description**: Replace Upscale "Output Quality" with the Bitrate Mode controls used in Convert Advanced.
- **Impact**: Medium - consistent workflow across modules
- **Notes**: Reuse Convert bitrate UI pattern once state manager is stable.

---

## ✅ Recently Fixed (Last 7 Days)

### BUG-001: Quality Preset showing in CBR/VBR modes (should only show in CRF)
- **Status**: ✅ FIXED (2026-01-04)
- **Reporter**: User
- **Module**: Convert
- **Description**: Quality Preset dropdown was showing when Bitrate Mode = CBR instead of only showing for CRF mode
- **Impact**: Confusing UI, wrong controls visible
- **Root Cause**: Duplicate visibility logic in `updateEncodingControls()` and `updateQualityVisibility()` conflicting with each other. Also `updateRemuxVisibility()` unconditionally showing containers.
- **Fix**:
  - Consolidated visibility logic into `updateQualityVisibility()` as single source of truth
  - Made `updateRemuxVisibility()` call `updateEncodingControls()` instead of directly showing containers
  - Removed explicit `.Hide()` call on quality section initialization
  - Added `updateQualityVisibility()` call after section creation
- **Files Changed**: `main.go:8522-8537, 8850, 8875, 8924-8928`
- **Fixed By**: Claude
- **Verified**: Yes, build passes

### BUG-002: Target File Size showing when not in Target Size mode
- **Status**: ✅ FIXED (2026-01-04)
- **Reporter**: User
- **Module**: Convert
- **Description**: Target File Size container was visible even when Bitrate Mode was set to CRF
- **Impact**: Confusing UI, irrelevant controls showing
- **Root Cause**: `updateRemuxVisibility()` was unconditionally showing all encoding containers when not in remux mode
- **Fix**: Changed `updateRemuxVisibility()` to call `updateEncodingControls()` which properly shows/hides based on bitrate mode
- **Files Changed**: `main.go:8924-8928`
- **Fixed By**: Claude
- **Verified**: Yes, build passes

### BUG-003: AAC and OPUS audio codec colors too similar
- **Status**: ✅ FIXED (2026-01-04)
- **Reporter**: User
- **Module**: Convert (Audio codec selection)
- **Description**: AAC (#7C3AED purple-blue) and OPUS (#8B5CF6 violet) colors were too similar to distinguish easily
- **Impact**: Usability issue - hard to tell which codec is selected
- **Fix**: Changed AAC color from #7C3AED (purple-blue) to #06B6D4 (bright cyan) for much better contrast
- **Files Changed**: `internal/ui/colors.go:47`
- **Fixed By**: Claude
- **Verified**: Yes, build passes

### BUG-004: Audio module missing drag & drop support
- **Status**: ✅ FIXED (2026-01-04)
- **Reporter**: User
- **Module**: Audio
- **Description**: Could not drag and drop audio or video files onto the Audio module tile
- **Impact**: Forced manual file selection via browse button
- **Fix**:
  - Added `isAudioFile()` helper function to detect audio file extensions
  - Modified `handleModuleDrop()` to accept audio files when dropping on audio module
  - Added audio module handler in `handleModuleDrop()` to load files and show audio view
- **Files Changed**: `main.go:2978, 3158-3180, 3186-3195`
- **Fixed By**: Claude
- **Verified**: Yes, build passes

### BUG-007: Copy Error button lacked actionable details
- **Status**: ✅ FIXED (2026-01-05)
- **Reporter**: User
- **Module**: Queue UI
- **Description**: Copy Error only included a truncated error string with no context
- **Fix**: Copy now includes job title, module, input/output, full error text, log path, and log tail
- **Files Changed**: `main.go`
- **Fixed By**: Codex
- **Verified**: Yes, build passes

### BUG-008: About page "Logs Folder" not opening on Windows
- **Status**: ✅ FIXED (2026-01-05)
- **Reporter**: User
- **Module**: About / OS integration
- **Description**: Clicking Logs Folder did nothing on Windows
- **Fix**: Use `explorer` with normalized path, ensure folder exists
- **Files Changed**: `main.go`
- **Fixed By**: Codex
- **Verified**: Yes, build passes

### BUG-009: Contact sheet output saved inside thumbnails folder
- **Status**: ✅ FIXED (2026-01-05)
- **Reporter**: User
- **Module**: Thumbnails
- **Description**: Contact sheet was stored inside `_thumbnails` folder, adding extra navigation
- **Fix**: Contact sheet now saves alongside source video; individual thumbs still use folder
- **Files Changed**: `thumb_module.go`
- **Fixed By**: Codex
- **Verified**: Yes, build passes

---

## 📋 Known Issues (Not Bugs - Design/Incomplete Features)

### ISSUE-001: Enhancement Module - Incomplete Implementation
- **Status**: ⏸️ ON HOLD
- **Module**: Enhancement
- **Description**: Enhancement module framework exists but is not fully implemented
- **Details**:
  - SkinToneAnalysis features are placeholder implementations
  - No UI wired up yet
  - Module commented out in navigation
- **Plan**: Complete in future dev cycle

### ISSUE-002: Widget Deduplication - Incomplete
- **Status**: 🔄 IN PROGRESS (opencode)
- **Module**: Convert UI
- **Description**: 4 widget pairs still need deduplication
- **Details**:
  - Pattern established with quality widgets (main.go:7075-7128)
  - Remaining pairs:
    - resolutionSelectSimple & resolutionSelect
    - targetAspectSelect & targetAspectSelectSimple
    - encoderPresetSelect & simplePresetSelect
    - bitratePresetSelect & simpleBitrateSelect
- **Plan**: Handed off to opencode agent

### ISSUE-003: ColoredSelect Expansion - Incomplete
- **Status**: 🔄 IN PROGRESS (opencode)
- **Module**: Convert UI
- **Description**: 32 widgets still need ColoredSelect conversion
- **Details**: Resolution, aspect, preset, bitrate, frame rate selectors need semantic color coding
- **Plan**: Handed off to opencode agent

---

## 🔧 How to Report a Bug

When you find a bug, add it here with:

```markdown
### BUG-XXX: Short descriptive title
- **Status**: 🔴 OPEN / 🔄 IN PROGRESS / ✅ FIXED
- **Reporter**: Name/Date
- **Module**: Which module (Convert, Audio, Player, etc.)
- **Description**: What's wrong? What should happen?
- **Steps to Reproduce**:
  1. Step one
  2. Step two
  3. Expected vs Actual behavior
- **Impact**: How bad is it?
- **Root Cause**: (fill in when investigating)
- **Fix**: (fill in when fixed)
- **Files Changed**: (list files)
- **Assigned To**: (agent name)
- **Verified**: Yes/No
```

---

## 🔍 Bug Statuses

- 🔴 **OPEN**: Bug confirmed, not yet being worked on
- 🔄 **IN PROGRESS**: Someone is actively fixing it
- ✅ **FIXED**: Fix implemented and verified
- ⏸️ **BLOCKED**: Waiting on something else
- ❌ **WONTFIX**: Decided not to fix (explain why)
- 🔁 **DUPLICATE**: Same as another bug (reference it)

---

## 📊 Bug Statistics

**Current Status**:
- 🔴 Critical Open: 2 (BUG-005, BUG-006)
- 🟠 High Priority Open: 0
- 🟡 Medium Priority Open: 0
- 🟢 Low Priority Open: 0
- ✅ Fixed (Last 7 Days): 7

**Trends**:
- 2026-01-05: 3 bugs fixed, 1 new critical bug opened
- Focus Area: Convert UI visibility + Windows stability/logging

---

## 🎯 Next Steps

1. **BUG-005** (Critical): Fix CRF quality settings visibility - Assigned to opencode
2. **ISSUE-002**: Complete widget deduplication (4 pairs remaining) - Assigned to opencode
3. **ISSUE-003**: Complete ColoredSelect expansion (32 widgets) - Assigned to opencode

---

## 💡 Notes

- This file should be updated by ALL agents when they discover or fix bugs
- When creating git issues, reference the BUG-XXX number from this file
- Keep the statistics section updated
- Move fixed bugs to "Recently Fixed" after 7 days, then archive them
