# VideoTools Changelog

## v0.1.1-dev39 (March 2026)

### Player Module Fixes
- **Panic recovery** — Added defer/recover in player module to catch CGO crashes and show user-friendly error instead of hard crash.
- **FFmpeg DLL consistency** — Now checks for local FFmpeg at `C:\ffmpeg\bin` first (matches compilation), falls back to download only if not found.
- **Local build fix** — Build script now copies FFmpeg DLLs to output directory (matching CI behavior).

### Author Module
- **Interactive Preview tab** — New Preview tab shows full interactive DVD menu preview with working video playback.
- **Module extraction** — Extracted author module to `internal/app/modules/author/`.
- **Tab visibility** — Preview tab only appears when Enable Menus is checked.
- **IFO audio track table** — VTS_MAT audio attributes now correctly reflect actual track codec, channel count, and language. Added `AudioCodingModeFromCodec`, `LanguageCodeBytes`, and `NumChannelsField` helpers to `internal/dvd/ifo` with unit tests.
- **Drag crash fix (revised)** — Dragging a video file into the Author module no longer hard-crashes back to the login screen. Root cause was `handleDrop` triggering a full 7-tab `showAuthorView()` rebuild on the main thread during DnD completion, which rendered a 720×480 GPU texture concurrent with the XWayland/GLFW DnD handshake. Fixed by adding an `authorClipsRefresh` callback (same pattern as `authorChaptersRefresh`) so drops only update the clip list widget — no view rebuild, no GPU upload during DnD. `addAuthorFiles` also runs off the main thread.
- **VTS_MAT byte layout** — Corrected all field byte offsets in `mat_serialize.go`/`vtsi.go` to match the packed `vtsi_mat_t` struct in libdvdread `ifo_types.h`. Previous code wrote table offsets at 0x1A2–0x1BE (inside `zero_17`) and audio attributes at 0x08D (inside `zero_12`). Now: table offsets at 0x0C8–0x0E4, video attrs at 0x200, audio count/attrs at 0x203/0x204, subpicture count/attrs at 0x255/0x256; `vtsi_last_byte` at 0x080; `vtstt_vobs` (title VOB start sector) at 0x0C4. Fixes dvdnav `zero_12`/`zero_17` violations and `ifoRead_VTS_PTT_SRPT failed`.
- **DVD menu VOB (M1/M2)** — `runNativeSpumux` now encodes the background PNG as an MPEG-2 still-video via ffmpeg and muxes it with SPU subpicture data into a proper DVD Program Stream VOB (`VIDEO_TS.VOB`). Falls back to video-only output if ffmpeg SPU mux fails.
- **PCI button table (M3)** — `PCIButton` struct added to `internal/dvd/vob/nav.go`. `WriteNAV_PCK` serializes up to 36 buttons with libdvdread-compatible bit-packed coordinate encoding (BTN_SL_NS at byte 94, BTN_NS at 95, entries at 98).
- **VMGM_VOBS_Sector (M4)** — `vmgMat.VMGM_VOBS_Sector` now set from the ISO layout pass so dvdnav can locate `VIDEO_TS.VOB` on disc.
- **Menu PGC sector patching (M5)** — `CellPlayback[0]` First/LastSector fields in each menu PGC updated with disc-absolute sector ranges derived from per-MPG file sizes and the `VIDEO_TS.VOB` disc start sector.
- **ExtrasMpg wiring (M6)** — `menuSet.ExtrasMpg` concatenated into `VIDEO_TS.VOB`; extras PGC built and included in the VMGM PGC table.
- **JumpVMGM_PGCN command (M7)** — `JumpVMGM_PGCNCommand(pgcN)` added to `internal/dvd/ifo/commands.go`; `ParseButtonCommand` now translates `"jump menu N;"` / `"jump menu pgc N;"` to the correct inter-menu PGC jump opcode (0x30, 0x06).

### Filter Integration (Complete)
- **Standalone filter jobs** — Filters module can now queue filter-only jobs without upscaling. "Add to Queue" button added to Filters module UI.
- **Filter job execution** — `executeFilterJob` supports color correction (brightness/contrast/saturation), enhancement (sharpness/denoise), transform (flip/rotate/grayscale), and stylistic filters (VHS/80s/Webcam effects) via FFmpeg.

### CI Fixes
- **Submodule sync** — Pushed missing commits to lt_mirror/fyne.git to fix CI failures.

## v0.1.1-dev38 (March 2026)

### Native DVD Engine
- **Native Go SPU encoder** — Added `internal/dvd/spu` with RLE encoding, display sequences, and `vob.WriteSPU` to generate zero-dependency DVD menus without spumux.
- **Menu VOB wiring** — Native SPU now wired into Author module menu generation pipeline.

### CI Fixes (Fyne API Changes)
- **Windows CI** — Fixed multiple compile errors from Fyne API changes: `desktop.KeyEvent`, `fyne.Color`, `VT_SUBTITLE_TYPE_TEXT`.
- **FFmpeg pinning** — Pinned Windows CI to BtbN n7.1 build to match Linux CI.
- **Build directives** — Replaced pkg-config CGO directives with explicit CFLAGS/LDFLAGS for cross-platform compatibility.

### Module Extraction
- **Subtitles module** — Full extraction to `internal/app/modules/subtitles/` with types, adapter, and view.
- **Inspect module** — Extracted to `internal/app/modules/inspect/view.go`.
- **Queue module** — Extracted to `internal/app/modules/queue/view.go`.
- **Upscale module** — Full extraction to `internal/app/modules/upscale/` with helpers, types, and view.
- **Settings module** — Created `internal/app/modules/settings/` with view and types.

### i18n Compliance
- **Author module** — Replaced hardcoded strings with i18n calls.

## v0.1.1-dev36 (March 2026)

### Module Extraction
- **Settings module structure** — Created `internal/app/modules/settings/` with view.go and types.go
- **Settings types moved** — PrefsConfig, Dependency, DependencyCommand types extracted to module package
- **Constructor functions** — Added NewDependencyCommand, NewDependencyCommandPair for type safety
- **Inspect module** — Extracted `showInspectView` and `buildInspectView` to `internal/app/modules/inspect/view.go`
- **Queue module** — Extracted queue view builders and refresh helpers to `internal/app/modules/queue/view.go`
- **Subtitles module** — Extracted package structure, types, adapter, and view code to `internal/app/modules/subtitles/`
- **Upscale module** — Full module extracted to `internal/app/modules/upscale/` with helpers.go, types.go, and view.go

### GPU Rendering Pipeline
- **Fyne fork** — GPU texture optimization fork at `lt_mirror/fyne`
- **TexSubImage2D** — Efficient texture upload in GL painter
- **UpdatePixels wiring** — VideoPlayer SetFrame uses UpdatePixels for texture reuse
- **Debug logging** — Added TexSubImage2D vs TexImage2D logging in newGlRasterTexture

### Media Engine
- **Adaptive buffering** — GetBufferHealth, GetDecodeTimeTrend, AdjustBufferForPerformance
- **Error recovery** — DegradeToSoftware, ShouldDegrade for HW failure handling
- **Speed via keyboard** — `<` / `>` keys change playback speed (0.25x-2.0x)
- **Actual FPS seeking** — Frame seeking uses real video FPS instead of hardcoded 30

### Playback Enhancements
- **Phase 4 complete** — Buffering indicators, error recovery, performance tuning

## v0.1.1-dev35 (March 2026)

### Localization Updates (dev34 continuation)
- **Subtitles i18n** — 9 new strings added and wired up (SubtitlesOfflineHint, SubtitlesEmpty, SubtitlesExtractEmbed, SubtitlesOCROutput, SubtitlesOCRLanguage, SubtitlesShiftOffset, SubtitlesStart, SubtitlesEnd).
- **Audio/Filters/Inspect i18n wired** — All module views now use t.* i18n keys.
- **Status bar localization** — Added StatusNoActiveJobs to status bar.
- **Trim module compatibility** — Updated trim stub and native view to support OnAddToQueue callback and TrimClip struct.
- **Dialog title i18n** — 15+ new translation keys wired into main.go for Convert/Merge/Trim/Snippet modules (DialogInterlacingResults, DialogAutoCropDetection, DialogNoBlackBars, DialogQueueNotInit, DialogNoRunningJob, LabelSnippet, MergeStarted, TrimJobAdded).

### UI Fixes
- **Back button consistency** — Module name uppercase on all modules.
- **Auto-check dropdown fix** — Fixed language switching issue in Settings Updates section.
- **Thumbnail contact sheet** — Increased header height (130→150px) and added filename truncation.
- **Inspect preview placeholder** — Replaced stuck "Loading preview" with proper idle player state and icons.
- **Preview frame capture** — Captured before interlace analysis to avoid UI stuck states.

### Trim Module
- **Trim job submission** — `submitTrimJob` creates queue.Job with proper Type, InputFile, OutputFile, and Config fields.

### Media Engine Overhaul
- **SplitView fixes** — Fixed divider color using exact VT Green #4CE870; implemented draggable divider with MouseMoved/Dragged handlers.
- **AudioPlayer improvements** — Added volume control (SetVolume/GetVolume), mute functionality, pause/resume control, proper error handling with logging.
- **Engine enhancements** — Added VideoInfo struct for metadata (Width, Height, FrameRate, Codec, Bitrate), Pause/Resume/TogglePause controls, volume/mute/speed controls, seeking with configurable accuracy (Frame/Keyframe/Accurate).
- **Queue improvements** — Added configurable max size limits to prevent unbounded memory growth.
- **Subtitle extraction** — New SubtitleExtractor for parsing subtitle streams from video files; supports SRT and ASS export formats.
- **Tests** — Added comprehensive test suite for media package (queue, clock, subtitle time formatting).
- **Player deprecation** — BackendMPV and BackendVLC marked as deprecated; factory now only supports FFplay and Native engines.

---

## v0.1.1-dev34 (March 2026)

### Localization Engine
- **Full i18n framework** — New `internal/i18n` package with a typed `Strings` struct as the single source of truth for every user-visible string. `T()` returns the active locale; listener callbacks let the entire UI refresh instantly on language change without a restart.
- **English (Canada) — en-CA** — 100% coverage; serves as fallback for all other locales.
- **French (Canada) — fr-CA** — Initial translation pass covering all core UI strings.
- **Inuktitut — iu** — Initial translation pass in Traditional Syllabics (ᐃᓄᒃᑎᑐᑦ) with a Latin toggle in Settings.
- **Aboriginal Sans embedded** — Aboriginal Sans Regular/Bold embedded in the binary for correct UCAS/syllabics rendering with no external font install required.
- **Language selector in Settings** — Dropdown in General tab; change takes effect immediately across all visible UI including the active module.
- **Locale-aware module refresh** — Switching language now rebuilds whichever module is currently open, not just the main menu.

### Native Media Engine (Phase 1 — `native_media` build tag)
- **Core engine scaffolding** — CGO/FFmpeg engine in `internal/media/` providing a proper decode pipeline, gated behind `//go:build native_media` so standard builds are unaffected.
- **Demuxer + PacketQueue** — Thread-safe packet queue feeding a demuxer loop with audio stream discovery.
- **AudioPlayer** — Full audio decoding and resampling via libswresample + oto; integrated into the engine with playback state management.
- **MasterClock + A/V sync** — High-precision master clock drives frame timing; AudioPlayer syncs to it, eliminating the separate-process A/V drift described in issues #14–#16.
- **Frame stepping & Seek** — Frame-accurate step forward/back; Seek implementation with queue flushing for clean repositioning (issue #17 foundation).
- **SplitView widget** — Side-by-side video comparison widget; wired into Compare module under the `native_media` tag.

### Disc Authoring (continued)
- **Multitrack audio & subtitle support** — Author module now exposes per-track audio/subtitle stream selection from the source file with a mapping table in the authoring pipeline.
- **ScriptableTheme engine** — JSON-driven theme format allows defining DVD menu layouts, button positions, and colour palettes without recompiling. Default asset bundled.
- **Native Go menu renderer** — `internal/dvd/theme` renders menu backgrounds and overlays entirely in Go using `golang.org/x/image/font` — no ImageMagick dependency.
- **Archivist round-trip** — Rip → load → re-author pipeline validated; source disc metadata and track layout preserved through the cycle.
- **IFO reading (VTSI + VMGI)** — `internal/dvd/ifo` can now parse existing IFO files from real discs, enabling accurate re-authoring from ripped sources.
- **VOBU_ADMAP + VTS Attribute Table** — Sector-accurate seeking map and multi-VTS attribute table implemented for standards-compliant output.
- **Automated disc scan on drop** — Rip module detects and enumerates titles/tracks automatically when a folder, ISO, or VIDEO_TS path is dropped onto it.
- **Native UDF extraction** — `internal/dvd/udf` can extract files from existing UDF images.

### RIFE Frame Interpolation (Upscale module)
- **RIFE integration** (issue #23) — `rife-ncnn-vulkan` wired into the Upscale module with configurable frame multiplier and model selection. Estimated output FPS shown in real time. Falls back gracefully when the binary is not installed.

### Module Architecture Refactor
- **Seven modules extracted** — audio, filters, inspect, thumbnail, player, enhancement, and compare moved to `internal/app/modules/` with clean Options/callback boundaries, reducing root package size.
- **Module colour via Options** — All extracted modules receive their accent colour through `Options.ModuleColor` from the root; nav bar always matches the main menu tile colour.
- **Back button i18n + casing** — All modules now use `strings.ToUpper(t.ModuleXxx)` for back buttons — uppercase, locale-aware, and consistent across every module.

### UI & Bug Fixes
- **Inspect crash on no file** — Clicking Inspect with no video loaded caused an immediate nil-pointer crash; all 20 `OnGetXxx` callbacks now guard against nil source.
- **Hardware accel dropdown** — Only acceleration backends confirmed available by `ffmpeg -hwaccels` are shown. A saved value that is no longer available resets to auto.
- **Convert output prefill** — Output filename field no longer pre-populates with a stale name from a previous session when no file is loaded on startup.
- **Disc category consolidation** — Replaced three separate Author/Rip/Blu-ray show/hide toggles in Settings with a single "Show Disc category" toggle. Blu-ray tile retired from the main menu (functionality fully merged into Author and Rip).

### Branding
- **VT_LOGO-2** — New app icon and logo replacing the original placeholder design.

### Localization Updates (dev34 continuation)
- **Subtitles i18n** — 9 new strings added and wired up (SubtitlesOfflineHint, SubtitlesEmpty, SubtitlesExtractEmbed, SubtitlesOCROutput, SubtitlesOCRLanguage, SubtitlesShiftOffset, SubtitlesStart, SubtitlesEnd).
- **Audio/Filters/Inspect i18n wired** — All module views now use t.* i18n keys.
- **Status bar localization** — Added StatusNoActiveJobs to status bar.
- **Trim module compatibility** — Updated trim stub and native view to support OnAddToQueue callback and TrimClip struct.

### UI Fixes
- **Back button consistency** — Module name uppercase on all modules.
- **Auto-check dropdown fix** — Fixed language switching issue in Settings Updates section.
- **Thumbnail contact sheet** — Increased header height (130→150px) and added filename truncation.

---

## v0.1.1-dev33 (March 2026)

### Disc Authoring
- **Native DVD Engine Foundation** - Established the `internal/dvd` package structure for native authoring. This is the first step toward removing Linux-only dependencies (`dvdauthor`, `xorriso`) and enabling DVD/Blu-ray creation on Windows.
- **Phase 1: UDF Writer** - Started implementation of a native UDF 1.02 / ISO 9660 Bridge writer for standards-compliant disc images.

### Documentation
- **Wiki synchronization** - Migrated in-repo `docs/` to the Forgejo wiki at `git.leaktechnologies.dev`. Established a navigation sidebar and centralized documentation portal.

## v0.1.1-dev32 (March 2026)

### Settings
- **Dependency install buttons** - Each dependency in the Dependencies tab now has an actionable Install button. FFmpeg on Windows uses the app-local bootstrap.
- **Platform-filtered dependencies** - The Dependencies tab only lists tools relevant to the current platform.
- **WSL auto-install reverted** - Installing Ubuntu via WSL would consume 5-10 GB; removed from the lightweight app. dvdauthor/xorriso restricted to Linux/macOS builds.
- **Updates tab wired** - Check for Updates hits the Forgejo tags API (`/api/v1/repos/leak_technologies/VideoTools/tags?limit=1`) and compares against the running version; fixed owner mismatch in API URL.
- **Uninstall support** - Uninstall buttons shown per dependency where a package manager uninstall command is available.
- **Disc toggles hidden on Windows** - Author, Rip, Blu-ray visibility checkboxes hidden in Settings on Windows since those tools are unavailable on that platform.
- **cmd window popups suppressed** - All subprocess calls on Windows now use `HideWindowExec` / `HideWindowExecContext` to prevent console window flashes.

### Icons
- **SVG icon library** - ~150 Material Design SVG icons added to `assets/icons/`; ASCII placeholders replaced with real icon resources.
- **Icons embedded into binary** - Icons are baked into the binary at compile time via `//go:embed`; `GetIcon` reads from the embedded FS with no runtime disk access. Fixes blank icons on installed builds (issue #20).

### Convert Module
- **Player layout fixed** - Video pane used `NewVBox` which collapsed the canvas.Image to 0 px; rewritten with `NewBorder` so the video fills the centre and the transport bar is pinned to the bottom.
- **VSplit gap fixed** - Extra `NewVBox` wrapper around the video panel left dark empty space in the top half of the VSplit; removed so the video panel fills its full allocated area.
- **Player icons fixed** - ASCII fallback labels replaced with `widget.NewButtonWithIcon` using embedded SVG icons.
- **Active state fixed** - `s.active` was never set to `"convert"`, breaking drop handling and keyboard shortcuts inside the module.
- **Source state fixed** - `s.source` was not updated when loading a video via `loadVideo`; now set before rebuilding the view.
- **Convert UI cleanup (issue #5)** - Label alignments standardised, consistent separators added.

### Compare Module
- **Hide/show player toggle (issue #1)** - Toggle button added to hide/show both video players, giving more vertical space for comparison.

### Main Menu / Navigation
- **Author and Rip hidden on Windows** - Disc modules hidden from the main menu on Windows until cross-platform disc authoring is implemented.
- **Mouse back button fixed** - Side mouse buttons now navigate correctly; back button (button 4) returns to main menu.
- **Keyboard shortcuts simplified** - Ctrl+Enter is the universal confirm action on Linux/Windows.
- **Drag-to-scroll fixed (issue #19)** - `container.Scroll` was silently consuming desktop drag events via a mobile-only guard. Replaced with a custom `scrollClip` widget that does not implement `fyne.Draggable`, so drag events reach `FastVScroll` and 1:1 content tracking works.
- **Pulsing drop indicator** - Video drop zone pulses when a draggable file hovers over the convert player area.
- **FastVScroll on all settings panels** - Upscale module settings and Convert metadata panel now use FastVScroll for consistent drag-to-scroll experience.

### Auto-Update
- **In-app updates** - Windows and Linux builds now support in-app auto-update. Check for Updates detects new releases; Install Update/Patches buttons download and apply updates automatically.

### UI
- **Main menu tile colour consistency** - Unavailable/missing-dependency module tiles now show a consistently dimmed module colour on first load.

## v0.1.1-dev31 (March 2026)

### UI
- **Module settings scrolling** - Scroll containers added to all non-Convert module settings panels; primary action buttons (Rip Now, Create Output, Apply Filters, Generate Now, Merge Now) pinned to always-visible footer action bar.
- **Window resize stability** - Window size is now preserved across module switches; layout-driven auto-resize no longer occurs.
- **Convert video pane** - Removed rigid minimum size from loaded-video stage so VSplit 50/50 offset holds correctly at all window sizes.
- **Click-and-drag scrolling** - Scroll panels now support click-and-drag in addition to mouse wheel, mirroring mobile/touch-screen behaviour. Drag up to scroll down, drag down to scroll up.
- **Convert module partial modularisation** - Added `ShowView`, `ConvertState`, and `ConvertCallbacks` entry point in `internal/app/modules/convert/view.go` with shim and type-converter helpers in `main.go`. Full `buildConvertView` extraction deferred pending appState decoupling.
- **Convert UI cleanup** - Layout and control organization pass to prepare Convert for external developer testing (in progress).

## v0.1.1-dev30 (March 2026)

### Maintenance
- **Version bump** - Started the dev30 cycle and updated project version markers.
- **Dev release notes** - Forgejo dev release comments now include nightly build context plus the matching version section from `docs/CHANGELOG.md`.
- **Versioning documentation** - Clarified that `-devN` numbering is continuous across release lines and public releases use base versions.
- **Release readiness policy** - Added explicit public bump gates and a full module testing checklist for release candidate validation.
- **Repository hygiene** - Removed root-level scratch files, relocated QR demo entrypoint into `cmd/`, and documented root cleanliness rules for agents.
- **Refactor planning** - Added a phased dev30 refactor plan for gradual `internal/app` and `cmd/` migration with build-safety guardrails.
- **Refactor phase 2 start** - Moved shared module config path logic to `internal/app/configpath` and rewired module config save/load call sites.
- **Refactor phase 2 continuation** - Moved merge/thumbnail config persistence logic into `internal/app/modulecfg` with compatibility wrappers to keep behavior unchanged.
- **Refactor phase 2 continuation** - Moved naming metadata/output-base helper logic into `internal/app/naming` with wrappers to preserve existing behavior.
- **Refactor phase 2 continuation** - Moved rip/subtitles config persistence logic into `internal/app/modulecfg` with compatibility wrappers to keep runtime behavior stable.
- **Refactor phase 2 continuation** - Moved author config persistence logic into `internal/app/modulecfg` with compatibility wrappers to keep runtime behavior stable.
- **Refactor phase 2 continuation** - Moved audio config persistence logic into `internal/app/modulecfg` with compatibility wrappers to keep runtime behavior stable.
- **Refactor phase 2 continuation** - Replaced duplicated `main.go` config-path helper functions with shared `internal/app/configpath` lookups for convert/recovery/benchmark/history.
- **Refactor phase 2 continuation** - Moved recovery/benchmark/history persistence logic into `internal/app/appcfg` with type aliases and wrapper functions in `main.go`.
- **Refactor phase 2 continuation** - Moved convert config JSON load/save plumbing into shared `internal/app/appcfg` store helpers while preserving convert normalization behavior.
- **Refactor phase 2 continuation** - Moved convert config normalization rules into `internal/app/appcfg` and kept `main.go` wrappers minimal.
- **CI build fix** - Restored missing `path/filepath` import in `audio_module.go` to resolve Forgejo Linux/Windows packaging compile failures.
- **CI build fix** - Restored missing `path/filepath` import in `rip_module.go` to resolve Forgejo Linux/Windows packaging compile failures.
- **Forgejo release targeting** - Workflow now reads version from `VERSION` first and patches only the matched tag release metadata (name/body/prerelease), preventing stale tag drift.
- **CI build fix** - Restored missing `encoding/json` and `path/filepath` imports in `subtitles_module.go` to resolve Forgejo Linux/Windows packaging compile failures.
- **FFprobe path fix** - Convert drag/drop analysis now uses the configured FFprobe path (including app-local Windows installs) instead of requiring `ffprobe` on PATH.
- **Thumbnail probe path fix** - Thumbnail metadata probes now use the configured FFprobe path to match the rest of the app dependency resolution flow.
- **Release note cleanup** - Forgejo dev release comments now include concise highlights from the matching changelog section instead of the full raw version block.
- **Release publish guard** - Dev publish now skips stale workflow runs when a newer `master` commit exists, preventing older jobs from updating previous dev-tag releases.
- **Refactor phase 3 start** - Moved About dialog implementation into `internal/app/modules/about` and kept a thin root shim to preserve behavior.
- **Refactor phase 3 continuation** - Moved missing-dependencies dialog rendering into `internal/app/modules/deps` and kept a thin root shim to preserve behavior.
- **Documentation portal migration** - Replaced retired `docs.leaktechnologies.dev` links with Forgejo wiki/in-repo documentation links in About, QR demo, and install/readme docs.
- **Refactor phase 3 continuation** - Moved main menu visibility/dependency filtering and active-job mapping helpers into `internal/app/modules/mainmenu` while keeping runtime behavior unchanged.
- **Release closeout checklist** - Added `docs/DEV30_FINALIZATION_CHECKLIST.md` to standardize dev30 finalization and dev31 handoff steps.
- **Agent handoff update** - Expanded `AGENTS.md` with current release state, closeout requirements, refactor boundaries, and dev31 takeover priorities.
- **Dev30 closeout** - CI validated (runs 219/220/221, commit 2cbb3a2), checklist sections 1/2/5/6/7 complete, smoke test and dependency validation carried forward to dev31.

## v0.1.1-dev29 (March 2026)

### Build/CI
- **Workflow parsing** - Fixed Windows dev-packages YAML parsing for bundled dependency notes.
- **Go module imports** - Fixed module import paths in refactored files so vendor mode builds resolve internal packages.
- **Main menu compile fix** - Removed duplicate package declaration in main menu module file.
- **Convert compile fix** - Removed a duplicated aspect/scale block and restored custom-aspect declarations before use.
- **Windows compile fix** - Removed stale `go-qrcode` import from `main.go`.
- **Windows packaging fallback** - Added Tesseract language-data download fallback for `eng/fra/iku` in bundled builds.
- **GStreamer packaging policy** - Bundled builds treat GStreamer as optional and continue when it is unavailable.
- **Whisper packaging resilience** - Bundled workflows now try multiple whisper model sources and continue if download fails.
- **Linux bundled zip fix** - Added `zip` to Linux CI build dependencies for bundled artifact creation.
- **Dev packaging policy** - Dev channel builds now skip bundled package generation to keep nightly/pre-release runs stable.
- **Bundled artifact retirement** - Dev-packages workflow now publishes standard VT packages only (no bundled artifacts).
- **Main menu layout fix** - Module tiles now use wrapping bounds to prevent over-wide window expansion.
- **Main menu row consistency** - Module sections now render as a stable 3-column grid.
- **Release asset cleanup fix** - Publish workflow now reliably removes old assets before uploading new artifacts.
- **Publish endpoint fix** - Corrected Forgejo asset delete endpoint to avoid 404 failures during release publish.
- **Module visibility** - Added a Blu-ray visibility toggle in Preferences and main menu filtering.
- **Benchmark behavior** - Applying benchmark recommendations now updates hardware acceleration only and leaves codec/preset untouched.

## v0.1.1-dev28 (February 2026)

### Windows
- **First-run FFmpeg bootstrap** - Added an in-app Windows prompt that installs FFmpeg/FFprobe into `%LOCALAPPDATA%\VideoTools\bin` when missing.
- **Module lock recovery** - Main menu now offers a direct fix path on clean Windows machines instead of leaving users in a settings-only workflow.
- **App-local FFmpeg discovery** - Platform detection now checks `%LOCALAPPDATA%\VideoTools\bin` before PATH/common locations, so bootstrap installs persist across launches.

### Cross-Platform
- **Settings dependency ordering** - Dependencies in Settings are now listed with core requirements first, then alphabetically.
- **Settings FFmpeg actions** - FFmpeg install actions are available in Settings on supported platforms (Windows app-local install; Linux package-manager actions).
- **Convert UI text safety** - Replaced mojibake-prone Unicode/emoji labels in the Convert workflow with ASCII-safe text labels.
- **About page cleanup** - Removed the Bitcoin address from the About/Support dialog.
- **Snippet AV1 fallback** - Snippet generation now falls back when `libsvtav1` is unavailable.
- **Adaptive scrolling** - Settings and other long panels use adaptive scroll speed for smoother navigation across screen sizes.
- **Settings tab scrolling** - Each Settings tab now scrolls independently so the header stays in view.
- **Master settings** - Preferences now surface global hardware acceleration with auto-detect plus module visibility toggles.
- **Aspect ratio handling** - Source aspect now honors display aspect ratio metadata and adds a 17:9 target option.
- **Aspect ratio UI** - The Source aspect option now shows the detected source aspect ratio.
- **Aspect ratio logging** - Conversion logs now include source/target aspect details and ignore stale auto-crop values when auto-crop is disabled.
- **Custom aspect input** - Added a Custom... option for cinema/ultrawide ratios without cluttering the dropdown.
- **Aspect/scale alignment** - Aspect conversion now uses target resolution to avoid odd output sizes.
- **Window resize stability** - Module switches no longer auto-resize the window.
- **UI text cleanup** - Removed garbled characters from UI labels and prompts.
- **Conversion stability** - Conversion workers now catch internal panics and surface a failure dialog instead of closing the UI.
- **Conversion recovery notice** - The app now records active conversions and shows a notice on next launch if one was running.
- **Main menu refactor** - Main menu builder and refresh helpers moved into a dedicated module file.
- **About/Support refactor** - About/Support dialog moved into a dedicated module file.
- **Dependencies dialog refactor** - Missing dependencies dialog moved into a dedicated module file.
- **Queue view refactor** - Queue view builders and refresh logic moved into a dedicated module file.
- **Dev packages workflow** - Fixed YAML parsing in bundled deps note generation.
- **Module imports** - Fixed main menu and queue module imports to match module path.
- **Main menu module** - Fixed duplicate package declaration causing Windows builds to fail.
- **Subtitle ripping** - Subtitles can be extracted from embedded tracks (lossless or OCR/SRT/ASS for text) and re-embedded without sync drift.
- **Subtitle OCR** - Image-based DVD/BD subtitle tracks can be OCR'd with Tesseract into SRT or ASS.
- **Subtitle OCR cleanup** - OCR output is normalized and consecutive duplicate cues are merged for cleaner timing.
- **Language options** - Preferences now focus on Canadian English, Canadian French, and Inuktitut.
- **Bundled packages** - Added bundled builds with FFmpeg, Tesseract, and GStreamer for Windows and Linux.
- **Bundled whisper model** - Bundled packages now include the whisper.cpp small model and enforce required dependency payloads.

## v0.1.1-dev27 (February 2026)

### Maintenance
- **.gitignore updates** - Excluded Windows build artifacts and agent working directory.
- **Forgejo Windows outputs** - Emit `GITHUB_OUTPUT` as UTF-8 (no BOM) with PowerShell-compatible append to prevent host-runner post-step failures.
- **Forgejo Windows GUI build** - Package the Windows exe with `-H windowsgui` to avoid console windows.
- **Forgejo Windows package contents** - Limit zip contents to `VideoTools.exe` and `README.md`.
- **Forgejo Windows artifact layout** - Write zip directly to `dist/windows/` and drop build metadata files.
- **Forgejo Windows artifact upload** - Upload only the zip file, excluding diagnostics and folders.
- **Forgejo dev release workflow** - Build Linux/Windows artifacts in the same workflow run before publishing releases.
- **Forgejo release assets** - Upload only Linux AppImage/zip and Windows zip (skip build metadata files).
- **Forgejo workflow cleanup** - Remove redundant Windows packaging workflow.
- **Forgejo workflow cleanup** - Remove redundant test trigger workflow.
- **Forgejo release assets** - Delete existing assets with the same name before upload to avoid duplicates.
- **Forgejo release assets** - Purge existing assets before upload to avoid duplicates.
- **Forgejo release assets** - Fail publish step on unauthorized asset deletion/upload.
- **Forgejo mirror** - Use built-in push mirror settings for Codeberg.
- **Forgejo dev release notes** - Use a nightly build release note body.
- **Forgejo workflow cleanup** - Remove redundant Linux packaging workflow.

## v0.1.1-dev26 (January 2026)

### Infrastructure
- **Mirror hosting** - Created lt_mirror repository on git.leaktechnologies.dev for downloads when source sites block bots (GStreamer, DVDStyler, Whisper, FFmpeg)
- **Forgejo CI/CD** - Self-hosted runner setup, CI workflows, artifact versioning, optional EXE signing

### Windows Build System
- **Installer** - Switched from Scoop to Chocolatey, added MSYS2, dependency checking with early exit, progress bars, verification
- **Build scripts** - Console popup suppression, icon embedding, windowsgui flag, Go module caching, Unicode fixes

### Documentation
- Added Forgejo runner and Windows service setup docs

## v0.1.0-dev24 (January 2026)

### 🎨 Main Menu Palette
- **Rainbow+ palette refresh** - restored diverse, eye-friendly module colors with improved readability
- **Convert color preserved** - Convert remains the visual anchor across the UI
- **Larger tile labels** - main menu button text is larger for accessibility
- **Contrast tuning** - audio/rip/settings colors adjusted for clarity
- **Scrollbar removed** - main menu now scales without a scroll bar
- **Module silhouette** - player area keeps a steady footprint before media loads
- **Bespoke hues** - each module now has its own distinct color identity
- **Locked state clarity** - disabled modules keep their hue with subdued brightness
- **Locked hue visibility** - reduced stripe opacity and raised label brightness

## v0.1.0-dev23 (January 2026)

### 🎉 UI Cleanup
- **Colored select refinement** - one-click open, left accent bar, rounded corners, larger labels
- **Unified input styling** - settings panel backgrounds match dropdown tone
- **Convert panel polish** - Auto-crop and Interlacing actions match panel styling

### 🧩 About / Support
- **Mockup-aligned layout** - title row, VT + LT logos on the right, Logs Folder action
- **Support placeholder** - “Support coming soon” until donation details are available

### 🐛 Fixes
- **Audio module crash** - guarded initial quality selection to avoid nil entry panic

## v0.1.0-dev22 (January 2026)

### 🎉 Major Features

#### Automatic GPU Detection for Hardware Encoding
- **Auto-detect GPU vendor** (NVIDIA/AMD/Intel) via system info detection
- **Automatic hardware encoder selection** when hardware acceleration set to "auto"
- **Resolves to appropriate encoder**: nvenc for NVIDIA, amf for AMD, qsv for Intel
- **Fallback to software encoding** if no compatible GPU detected
- **Cross-platform detection**: nvidia-smi, lspci, wmic, system_profiler

#### SVT-AV1 Encoding Performance
- **Proper AV1 codec support** with hardware (av1_nvenc, av1_qsv, av1_amf) and software (libsvtav1) encoders
- **SVT-AV1 speed preset mapping** (0-13 scale) for encoder performance tuning
- **Prevents 80+ hour encodes** by applying appropriate speed presets
- **ultrafast preset** → ~10-15 hours instead of 80+ hours for typical 1080p encodes
- **CRF quality control** for AV1 encoding

#### UI/UX Improvements
- **Fluid UI splitter** - removed rigid minimum size constraints for smoother resizing
- **Format selector widget** - proper dropdown for container format selection
- **Semantic color system** - ColoredSelect ONLY for format/codec navigation (not rainbow everywhere)
- **Format colors**: MKV=teal, MP4=blue, MOV=indigo
- **Codec colors**: AV1=emerald, H.265=lime, H.264=sky, AAC=purple, Opus=violet

### 🔧 Technical Improvements

#### Hardware Encoding
- **GPUVendor() method** in sysinfo package for GPU vendor identification
- **Automatic encoder resolution** based on detected hardware
- **Better hardware encoder fallback** logic

#### Platform Support
- **Windows FFmpeg popup suppression** - proper build tags on exec_windows.go/exec_unix.go
- **Platform-specific command creation** with CREATE_NO_WINDOW flag on Windows
- **Fixed process creation attributes** for silent FFmpeg execution on Windows

#### Code Quality
- **Queue system type consistency** - standardized JobType constants (JobTypeFilter)
- **Fixed forward declarations** for updateDVDOptions and buildCommandPreview
- **Removed incomplete formatBackground** section with TODO for future implementation
- **Git remote correction** - restored git.leaktechnologies.dev repository URL

### 🐛 Bug Fixes

#### Encoding
- **Fixed AV1 forced H.264 conversion** - restored proper AV1 encoding support
- **Added missing preset mapping** for libsvtav1 encoder
- **Proper CRF handling** for AV1 codec

#### UI
- **Fixed dropdown reversion** - removed rainbow colors from non-codec dropdowns
- **Fixed splitter stiffness** - metadata and labeled panels now resize fluidly
- **Fixed formatContainer** missing widget definition

#### Build
- **Resolved all compilation errors** from previous session
- **Fixed syntax errors** in formatBackground section
- **Fixed JobType constant naming** (JobTypeFilter vs JobTypeFilters)
- **Moved WIP files** out of build path (execute_edit_job.go.wip)

#### Dependencies
- **Upscale module accessibility** - changed from requiring realesrgan to optional
- **FFmpeg-only scaling** now works without AI upscaler dependencies

### 📝 Coordination & Planning

#### Agent Coordination
- **Updated WORKING_ON.md** with coordination request for opencode
- **Analyzed uncommitted job editing feature** (edit.go, command_editor.go)
- **Documented integration gaps** and presented 3 options for dev23
- **Removed Gemini from active agent rotation**

### 🚧 Work in Progress (Deferred to Dev23)

#### Job Editing Feature (opencode)
- **Core logic complete** - edit.go (363 lines), command_editor.go (352 lines)
- **Compiles successfully** but missing integration
- **Needs**: main.go hookups, UI buttons, end-to-end testing
- **Status**: Held for proper integration in dev23

### 🔄 Breaking Changes

None - this is a bug-fix and enhancement release.

### ⚠️ Known Issues

- **Windows dropdown UI differences** - investigating appearance differences on Windows vs Linux (deferred to dev23)
- **Benchmark system** needs improvements (deferred to dev23)

### 📊 Development Stats

**Commits This Release**: 3 main commits
- feat: add automatic GPU detection for hardware encoding
- fix: resolve build errors and complete dev22 fixes
- docs: update WORKING_ON coordination file

**Files Modified**: 8 files
- FyneApp.toml (version bump)
- main.go (GPU detection, AV1 presets, UI fixes)
- internal/sysinfo/sysinfo.go (GPUVendor method)
- internal/queue/queue.go (JobType fixes)
- internal/utils/exec_windows.go (build tags)
- internal/utils/exec_unix.go (build tags)
- settings_module.go (Upscale dependencies)
- WORKING_ON.md (coordination)

---

## v0.1.0-dev14 (December 2025)

### 🎉 Major Features

#### Windows Compatibility Implementation
- **Cross-platform build system** with MinGW-w64 support
- **Platform detection system** (`platform.go`) for OS-specific configuration
- **FFmpeg path abstraction** supporting bundled and system installations
- **Hardware encoder detection** for Windows (NVENC, QSV, AMF)
- **Windows-specific process handling** and path validation
- **Cross-compilation script** (`scripts/windows/build-windows.sh`)

#### Professional Installation System
- **One-command installer** (`scripts/linux/install.sh`) with guided wizard
- **Automatic shell detection** (bash/zsh) and configuration
- **System-wide vs user-local installation** options
- **Convenience aliases** (`VideoTools`, `VideoToolsRebuild`, `VideoToolsClean`)
- **Comprehensive installation guide** (`INSTALLATION.md`)

#### DVD Auto-Resolution Enhancement
- **Automatic resolution setting** when selecting DVD formats
- **NTSC/PAL auto-configuration** (720×480 @ 29.97fps, 720×576 @ 25fps)
- **Simplified user workflow** - one click instead of three
- **Standards compliance** ensured automatically

#### Queue System Improvements
- **Enhanced thread-safety** with improved mutex locking
- **New queue control methods**: `PauseAll()`, `ResumeAll()`, `MoveUp()`, `MoveDown()`
- **Better job reordering** with up/down arrow controls
- **Improved status tracking** for running/paused/completed jobs
- **Batch operations** for queue management

### 🔧 Technical Improvements

#### Code Organization
- **Platform abstraction layer** for cross-platform compatibility
- **FFmpeg path variables** in internal packages
- **Improved error handling** for Windows-specific scenarios
- **Better process termination** handling across platforms

#### Build System
- **Cross-compilation support** from Linux to Windows
- **Optimized build flags** for Windows GUI applications
- **Dependency management** for cross-platform builds
- **Distribution packaging** for Windows releases

#### Documentation
- **Windows compatibility guide** (`WINDOWS_COMPATIBILITY.md`)
- **Implementation documentation** (`DEV14_WINDOWS_IMPLEMENTATION.md`)
- **Updated installation instructions** with platform-specific notes
- **Enhanced troubleshooting guides** for Windows users

### 🐛 Bug Fixes

#### Queue System
- **Fixed thread-safety issues** in queue operations
- **Resolved callback deadlocks** with goroutine execution
- **Improved error handling** for job state transitions
- **Better memory management** for long-running queues

#### Platform Compatibility
- **Fixed path separator handling** for cross-platform file operations
- **Resolved drive letter issues** on Windows systems
- **Improved UNC path support** for network locations
- **Better temp directory handling** across platforms

### 📚 Documentation Updates

#### New Documentation
- `INSTALLATION.md` - Comprehensive installation guide (360 lines)
- `WINDOWS_COMPATIBILITY.md` - Windows support planning (609 lines)
- `DEV14_WINDOWS_IMPLEMENTATION.md` - Implementation summary (325 lines)

#### Updated Documentation
- `README.md` - Updated Quick Start for install.sh
- `BUILD_AND_RUN.md` - Added Windows build instructions
- `docs/README.md` - Updated module implementation status
- `TODO.md` - Reorganized for dev15 planning

### 🔄 Breaking Changes

#### Build Process
- **New build requirement**: MinGW-w64 for Windows cross-compilation
- **Updated build scripts** with platform detection
- **Changed FFmpeg path handling** in internal packages

#### Configuration
- **Platform-specific configuration** now required
- **New environment variables** for FFmpeg paths
- **Updated hardware encoder detection** system

### 🚀 Performance Improvements

#### Build Performance
- **Faster incremental builds** with better dependency management
- **Optimized cross-compilation** with proper toolchain usage
- **Reduced binary size** with improved build flags

#### Runtime Performance
- **Better process management** on Windows
- **Improved queue performance** with optimized locking
- **Enhanced memory usage** for large file operations

### 🎯 Platform Support

#### Windows (New)
- ✅ Windows 10 support
- ✅ Windows 11 support  
- ✅ Cross-compilation from Linux
- ✅ Hardware acceleration (NVENC, QSV, AMF)
- ✅ Windows-specific file handling

#### Linux (Enhanced)
- ✅ Improved hardware encoder detection
- ✅ Better Wayland support
- ✅ Enhanced process management

#### Linux (Enhanced)
- ✅ Continued support with native builds
- ✅ Hardware acceleration (VAAPI, NVENC, QSV)
- ✅ Cross-platform compatibility

### 📊 Statistics

#### Code Changes
- **New files**: 3 (platform.go, build-windows.sh, install.sh)
- **Updated files**: 15+ across codebase
- **Documentation**: 1,300+ lines added/updated
- **Platform support**: 2 platforms (Linux, Windows)

#### Features
- **New major features**: 4 (Windows support, installer, auto-resolution, queue improvements)
- **Enhanced features**: 6 (build system, documentation, queue, DVD encoding)
- **Bug fixes**: 8+ across queue, platform, and build systems

### 🔮 Next Steps (dev15 Planning)

#### Immediate Priorities
- Windows environment testing and validation
- NSIS installer creation for Windows
- Performance optimization for large files
- UI/UX refinements and polish

#### Module Development
- Merge module implementation
- Trim module with timeline interface
- Filters module with real-time preview
- Advanced Convert features (2-pass, presets)

#### Platform Enhancements
- Native Windows builds
- Linux AppImage bundle creation
- Linux package distribution (.deb, .rpm)
- Auto-update mechanism

---

## v0.1.0-dev13 (November 2025)

### 🎉 Major Features

#### DVD Encoding System
- **Complete DVD-NTSC implementation** with professional specifications
- **Multi-region support** (NTSC, PAL, SECAM) with region-free output
- **Comprehensive validation system** with actionable warnings
- **FFmpeg command generation** for DVD-compliant output
- **Professional compatibility** (DVDStyler, PS2, standalone players)

#### Code Modularization
- **Extracted 1,500+ lines** from main.go into organized packages
- **New package structure**: `internal/convert/`, `internal/app/`
- **Type-safe APIs** with exported functions and structs
- **Independent testing capability** for modular components
- **Professional code organization** following Go best practices

#### Queue System Integration
- **Production-ready queue system** with 24 public methods
- **Thread-safe operations** with proper synchronization
- **Job persistence** with JSON serialization
- **Real-time progress tracking** and status management
- **Batch processing capabilities** with priority handling

### 📚 Documentation

#### New Comprehensive Guides
- `DVD_IMPLEMENTATION_SUMMARY.md` (432 lines) - Complete DVD system reference
- `QUEUE_SYSTEM_GUIDE.md` (540 lines) - Full queue system documentation  
- `INTEGRATION_GUIDE.md` (546 lines) - Step-by-step integration instructions
- `COMPLETION_SUMMARY.md` (548 lines) - Project completion overview

#### Updated Documentation
- `README.md` - Updated with DVD features and installation
- `MODULES.md` - Enhanced module descriptions and coverage
- `TODO.md` - Reorganized for dev14 planning

### 📚 Documentation Updates

#### New Documentation Added
- Enhanced `TODO.md` with Lossless-Cut inspired trim module specifications
- Updated `MODULES.md` with detailed trim module implementation plan
- Enhanced `docs/README.md` with VT_Player integration links

#### Documentation Enhancements
- **Trim Module Specifications** - Detailed Lossless-Cut inspired design
- **VT_Player Integration Notes** - Cross-project component reuse
- **Implementation Roadmap** - Clear development phases and priorities

---

*For detailed technical information, see the individual implementation documents in the `docs/` directory.*



