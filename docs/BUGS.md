# Bug Tracker

Track all bugs, issues, and behavioral problems here. Update this file whenever you discover or fix a bug.

**Last Updated**: 2026-06-12 UTC

---

## 🔴 Critical Bugs (Blocking Functionality)

### BUG-012: Windows releases have missing/mismatched FFmpeg DLLs
- **Status**: 🔴 OPEN (pipeline fix in progress)
- **Reporter**: tester (recurring over multiple dev cycles)
- **Module**: Convert / Audio / Thumbnail / Rip / all FFmpeg-dependent modules
- **Description**: Windows users downloading VT from the website encounter FFmpeg DLL load failures. ffmpeg.exe and ffprobe.exe subprocesses cannot start, making all conversion, audio, thumbnail, and rip operations fail silently or crash.
- **Steps to Reproduce**:
  1. Download VT Windows ZIP from website
  2. Extract and run VideoTools.exe
  3. Startup validation shows DLL errors (missing DLLs, load failures)
  4. Any FFmpeg-dependent operation fails
- **Impact**: Critical — the entire point of the app is video processing; DLL failures make it unusable
- **Root Cause** (three separate pipeline bugs):
  - **GitHub release.yml** (website downloads): Downloaded BtbN `gpl-shared` for BOTH static compile and runtime DLLs. Only bundled `av*.dll` + `sw*.dll` — missing all transitive deps (`liblzma-5.dll`, etc.). No `ffmpeg.exe`/`ffprobe.exe` bundled. No objdump transitive-dep scan. Used winlibs GCC instead of MSYS2 ucrt64 — different C runtime ABI.
  - **GitHub windows-msix.yml**: Same problems as release.yml — BtbN-only, missing transitive deps, no ffmpeg/ffprobe.
  - **Forgejo dev-packages.yml (BtbN DLL download step)**: The BtbN `latest` tag is a moving target. Between CI runs, BtbN can push a new FFmpeg version that changes ABI numbers (e.g. `avcodec-61.dll` → `avcodec-62.dll`), making the DLLs incompatible with the statically-linked exe and the bundled ffmpeg/ffprobe. The `ExpectedFFmpegDLLs()` list hardcoded `-61`/`-59`/etc. ABI numbers that will break when BtbN bumps FFmpeg 8.x → 9.x.
- **Fix in progress**:
  1. Rewrote GitHub release.yml to build FFmpeg from source (consistent with Forgejo CI and settled decision). Uses MSYS2 ucrt64 toolchain, source-built x264/x265/FFmpeg for static link, BtbN `lgpl-shared` for runtime DLLs + ffmpeg/ffprobe only. Added objdump transitive-dep scan, ffmpeg/ffprobe bundling, and `liblzma-5.dll` in `ExpectedFFmpegDLLs()`.
  2. Rewrote windows-msix.yml identically.
  3. Added `liblzma-5.dll` to `ExpectedFFmpegDLLs()` runtime validation.
  4. Forgejo pipeline already uses source-built FFmpeg + BtbN shared DLLs + objdump scan; the remaining risk is BtbN `latest` ABI drift (see BUG-013 below).
- **Files Changed**: `.github/workflows/release.yml`, `.github/workflows/windows-msix.yml`, `internal/app/appcfg/ffmpeg_bootstrap.go`
- **Assigned To**: opencode
- **Verified**: No (pipeline fix not yet tested in CI)

### BUG-013: BtbN "latest" shared DLLs can drift in ABI version
- **Status**: ✅ FIXED (2026-06-12)
- **Reporter**: dev analysis
- **Module**: All FFmpeg-dependent modules (Windows)
- **Description**: The CI pipelines downloaded BtbN `ffmpeg-master-latest-win64-lgpl-shared.zip` for runtime DLLs. The `latest` tag is a moving target — BtbN can push a new major FFmpeg version at any time, changing ABI numbers in DLL names (e.g. `avcodec-61.dll` → `avcodec-62.dll`), causing startup validation failures and subprocess crashes.
- **Root Cause**: Dependency on an external moving-target download for runtime binaries
- **Fix**: All three CI pipelines (Forgejo dev-packages, GitHub release, GitHub MSIX) now build FFmpeg 8.1 from source **twice**: once static (for CGo link) and once shared (for DLLs + ffmpeg.exe + ffprobe.exe). Both use the same x264/x265 static archives and the same FFmpeg source tarball. BtbN downloads completely eliminated. `ExpectedFFmpegDLLs()` also changed to glob patterns (`avcodec-*.dll`) instead of hardcoded ABI versions so future FFmpeg major bumps don't break validation.
- **Files Changed**: `.forgejo/workflows/dev-packages.yml`, `.github/workflows/release.yml`, `.github/workflows/windows-msix.yml`, `internal/app/appcfg/ffmpeg_bootstrap.go`, `AGENTS.md`
- **Assigned To**: opencode
- **Verified**: No (pipeline not yet tested in CI)

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
- **Assigned To**: Unassigned
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

### BUG-011: Thumbnail module hangs then crashes on open
- **Status**: 🔴 OPEN
- **Reporter**: User (2026-04-24)
- **Module**: Thumbnail
- **Description**: When clicking the Thumbnail module from the main menu, the app hangs for several seconds then crashes without any error output.
- **Steps to Reproduce**:
  1. Launch VT
  2. Click Thumbnail module tile in main menu
  3. Expected: View opens normally
  4. Actual: Long hang, then crash
- **Impact**: Critical - Thumbnail module is unusable
- **Investigation Notes**:
  - Added panic recovery to `setContent()` in main.go (debug logging in place)
  - Added debug logging to `showThumbnailView()` in thumbnail_module.go
  - Need user to test and check logs for root cause
- **Files to Check**:
  - `main.go:2216-2246` - setContent() with panic recovery
  - `thumbnail_module.go:109-127` - showThumbnailView() with debug logging
  - Log output for crash details
- **Assigned To**: Unassigned
- **Verified**: No (investigating)

---

## 🟠 High Priority Bugs (Major Issues)

### BUG-010: Upscale jobs show no progress during FFmpeg conversions
- **Status**: ✅ FIXED (2026-01-06, needs verification)
- **Reporter**: user report (2026-01-06)
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
- **Fixed By**: dev team
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
- **Reporter**: user report (2026-01-06)
- **Module**: Enhancement
- **Description**: Enhancement panel should include a blur control in addition to sharpen/denoise.
- **Impact**: Medium - expected control in enhancement workflow
- **Notes**: Implemented blur controls in Upscale first; Enhancement UI still pending.

### FEAT-004: Upscale output quality should use Bitrate Mode controls
- **Status**: 🧭 PLANNED
- **Reporter**: user report (2026-01-06)
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
- **Fixed By**: dev team
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
- **Fixed By**: dev team
- **Verified**: Yes, build passes

### BUG-003: AAC and OPUS audio codec colors too similar
- **Status**: ✅ FIXED (2026-01-04)
- **Reporter**: User
- **Module**: Convert (Audio codec selection)
- **Description**: AAC (#7C3AED purple-blue) and OPUS (#8B5CF6 violet) colors were too similar to distinguish easily
- **Impact**: Usability issue - hard to tell which codec is selected
- **Fix**: Changed AAC color from #7C3AED (purple-blue) to #06B6D4 (bright cyan) for much better contrast
- **Files Changed**: `internal/ui/colors.go:47`
- **Fixed By**: dev team
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
- **Fixed By**: dev team
- **Verified**: Yes, build passes

### BUG-007: Copy Error button lacked actionable details
- **Status**: ✅ FIXED (2026-01-05)
- **Reporter**: User
- **Module**: Queue UI
- **Description**: Copy Error only included a truncated error string with no context
- **Fix**: Copy now includes job title, module, input/output, full error text, log path, and log tail
- **Files Changed**: `main.go`
- **Fixed By**: dev team
- **Verified**: Yes, build passes

### BUG-008: About page "Logs Folder" not opening on Windows
- **Status**: ✅ FIXED (2026-01-05)
- **Reporter**: User
- **Module**: About / OS integration
- **Description**: Clicking Logs Folder did nothing on Windows
- **Fix**: Use `explorer` with normalized path, ensure folder exists
- **Files Changed**: `main.go`
- **Fixed By**: dev team
- **Verified**: Yes, build passes

### BUG-009: Contact sheet output saved inside thumbnails folder
- **Status**: ✅ FIXED (2026-01-05)
- **Reporter**: User
- **Module**: Thumbnails
- **Description**: Contact sheet was stored inside `_thumbnails` folder, adding extra navigation
- **Fix**: Contact sheet now saves alongside source video; individual thumbs still use folder
- **Files Changed**: `thumb_module.go`
- **Fixed By**: dev team
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
- **Status**: 🔄 IN PROGRESS
- **Module**: Convert UI
- **Description**: 4 widget pairs still need deduplication
- **Details**:
  - Pattern established with quality widgets (main.go:7075-7128)
  - Remaining pairs:
    - resolutionSelectSimple & resolutionSelect
    - targetAspectSelect & targetAspectSelectSimple
    - encoderPresetSelect & simplePresetSelect
    - bitratePresetSelect & simpleBitrateSelect
- **Plan**: Handed off

### ISSUE-003: ColoredSelect Expansion - Incomplete
- **Status**: 🔄 IN PROGRESS
- **Module**: Convert UI
- **Description**: 32 widgets still need ColoredSelect conversion
- **Details**: Resolution, aspect, preset, bitrate, frame rate selectors need semantic color coding
- **Plan**: Handed off

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
- **Assigned To**: (assignee)
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
- 🔴 Critical Open: 3 (BUG-005, BUG-006, BUG-012)
- 🟠 High Priority Open: 0
- 🟡 Medium Priority Open: 0
- 🟢 Low Priority Open: 0
- ✅ Fixed (Last 7 Days): 8

**Trends**:
- 2026-06-12: BUG-013 (BtbN ABI drift) fixed — all CI pipelines now build shared FFmpeg from source
- 2026-06-12: BUG-012 (DLL pipeline) opened — root cause identified, fix in progress
- 2026-01-05: 3 bugs fixed, 1 new critical bug opened

---

## 🎯 Next Steps

1. **BUG-012** (Critical): Verify Windows DLL pipeline fix in CI — all three workflows need a real build run to confirm
2. **BUG-006** (Critical): Windows mid-conversion crash — investigate logging and goroutine lifecycle
3. **BUG-005** (Critical): Fix CRF quality settings visibility - Unassigned
4. **ISSUE-002**: Complete widget deduplication (4 pairs remaining) - Unassigned
5. **ISSUE-003**: Complete ColoredSelect expansion (32 widgets) - Unassigned

---

## 💡 Notes

- This file should be updated by ALL agents when they discover or fix bugs
- When creating git issues, reference the BUG-XXX number from this file
- Keep the statistics section updated
- Move fixed bugs to "Recently Fixed" after 7 days, then archive them
