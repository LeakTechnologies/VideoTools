# VideoTools Changelog

## v0.1.1-dev45 (May 2026)

### Convert Module Improvements - Phase 1 (HIGH)
- **Audio Sample Rate dropdown** — `audioSampleRateSelect` wired in buildConvertView
- **Normalize Audio checkbox** — `normalizeAudioCheck` + LUFS/TruePeak sliders wired
- **Deinterlace Mode dropdown** — `deinterlaceModeSelect` + `deinterlaceMethodSelect` wired
- **H.264 Profile/Level controls** — `h264ProfileSelect` / `h264LevelSelect` wired; shown when H.264 codec is active

### Convert Module i18n (HIGH - Issue #5)
- **~42 hardcoded strings** i18n'd: checkboxes, buttons, dialog messages, back button
- **New keys added** to `internal/i18n/strings.go`, `en_ca.go`, `fr_ca.go`, `iu.go`, `iu_latin.go`

### Upscale Module Improvements (dev44)
- **One-click presets** — Hobbyist SD→HD, Semi-Pro 1080p→4K, Anime, Restoration, Social Media workflows
- **UI clarity** — Preset dropdown with description labels, clear AI+RIFE workflow
- **Detection reliability** — VerifyTool() checks PATH + app-local bin + smoke test
- **Optimization guide** — See `docs/UPSCALE_OPTIMIZATION.md` for hobbyist/semi-pro workflows
- **Hardware acceleration** — Sync upscale HW accel from master setting
- **Filters module HW accel** — Add hardware acceleration dropdown

### Audio Module Phase 2 (HIGH)
- **InlineVideoPlayer** — Add player singleton like Convert
- **Video preview pane** — Same layout pattern as Convert
- **SMPTE bars idle state** — "DROP VIDEO TO LOAD"

### Audio Module Phase 1 (HIGH)
- **Consistent box styling** — Added `buildAudioBox()` helper, Convert-style boxes
- **Proper header bar** — `TintedBar` with module title + stats integration wired

### Queue Module UI Polish (dev44)
- **TintedBar header** — Replaced custom header with `TintedBar` matching other modules
- **Status badge** — Shows active/completed/failed counts in header
- **48px bottom bar** — Restored VT green `TintedBar` (matches other modules)
- **Live output panel** — 4px VT green outline border
- **Thumbnail preview** — 90px tall with 3px module-color outline, auto-generated midpoint frame
- **Module colors** — `ModuleColor()` exactly matches main menu (all 13 modules)
- **Layout fixes** — Thumbnail left, text right; proper spacing

### Flags & i18n (dev44)
- **Language dropdown** — Fixed flag loading (removed incorrect `fs.Sub`); SVG flags now visible
- **Main menu** — "QUEUE" button uppercase in all 4 locales

### Thumbnail Quality (dev44)
- **Deinterlace filter** — `yadif=1` added to avoid interlaced frames
- **Interlace detection** — `findCleanFrameOffset()` skips to clean frames
- **Job log file** — Each thumbnail job writes FFmpeg output to timestamped log

### Module Pipeline (`&&` feature - dev44)
- **Pipeline state machine** — `pipelineActive` on `appState` (off / waiting-step1 / waiting-step2)
- **`&&` button** — Main menu header reflects state (off / glowing / showing "A → ?")
- **Module tile dimming** — Invalid Step 2 targets dimmed when in "pick Step 2" state
- **Queue integration** — `PipelineAfter` + `PipelineDeleteOnSuccess` fields on `queue.Job`
- **Intermediate files** — "Keep intermediate files" toggle in Settings → Preferences

---

## v0.1.1-dev44 (April 2026)

### Native Media Player — Playback & Sync Fixes
- **Start/Resume state fix** — `Start()` was setting `e.paused=false` before starting the decode goroutine, but `Resume()` checked `!e.paused` and returned early. Decode loop never started on Play. Fixed: `Start()` now sets `e.paused=false` after launching goroutines.
- **Audio/video sync on load** — Audio clock drifted during `Load()` because `audioDecodeLoop` ran during `GrabFrame`, causing ~5 second offset before playback. Fixed by resetting clock to 0 in `ResetAfterGrab`.
- **Test pattern font** — Test pattern always renders with VCR OSD Mono font regardless of user preference.
- **FFmpeg bootstrap simplification** — Always downloads BtbN pre-built package to guarantee complete DLL set (avoids local installs with missing dependencies like liblzma).

### Settings
- **Player font preference** — Users can choose between IBM Plex Mono and VCR OSD Mono for the OSD. VCR OSD Mono has no Bold/Italic variants; UI gracefully falls back to Regular weight.

### Known Issues (documented for tracker)
- See `docs/PLAYER_DEBUG.md` for full list of known player issues including: `predecodeFrom` sharing `formatCtx` with `demuxerLoop`, audio queue not flushed before seek, speed changes not affecting audio tempo, D3D11VA crashes when enabled.

## v0.1.1-dev43 (April 2026)

### Native Media Player — Thread-Safety & Crash Fixes
- **Pixel format crash fix** — `GrabFrame` and `NextFrame` now derive the `sws` pixel format from `frame.format` (the actual decoded format) rather than `videoCodecCtx.pix_fmt`, which is `AV_PIX_FMT_NONE` until the codec parses its first SPS. A nil `swsCtx` produced by `sws_getContext` caused an unrecoverable C SIGSEGV inside `sws_scale`. `NextFrame`'s SW decode path was also missing the `ensureSwsCtx` call entirely.
- **Close/demuxer race** — Added `sync.WaitGroup demuxerWg` to `Engine`. `demuxerLoop` signals Done on exit. `Engine.Close()` now waits for the demuxer to fully exit before freeing `formatCtx`, `videoCodecCtx`, or any other FFmpeg resource, eliminating the use-after-free crash window.
- **NextFrame/Close codec race** — `Close()` acquires `videoCodecMu` before freeing `videoCodecCtx`, ensuring any in-flight `NextFrame` decode cycle has completed first.
- **seekLoop goroutine leak** — `InlineVideoPlayer.seekCh` was never closed, leaking the `seekLoop` goroutine on every `Close()`. Ownership moved to `Load()`: channel is closed and reallocated per file. `Close()` closes the channel to drain the goroutine. The `OnSeek` widget callback guards against a nil channel under the player mutex.

## v0.1.1-dev42 (April 2026)

### Native Media Player — GStreamer Removal
- **GStreamer fully removed** — All `internal/player/gstreamer*` deleted; `native_media` build tag is the only player path. No more GStreamer dependency at runtime or build time.
- **Player lifecycle fix** — `closeNativePlayer()` prevents audio hanging on module switch; `Widget().Refresh()` deferred after canvas swap.

### Native Media Player — D3D11VA / HW Decode Stabilisation
- **D3D11VA get_format callback** — Accepts `AV_PIX_FMT_D3D11VA_VLD` so D3D11VA decode starts on first packet.
- **H.264 + D3D11VA crash fix** — Pre-warm D3D11VA before first decode call.
- **Dedicated HW frame buffers** — Separate `hwFramesCtx` prevents races between HW download and SW display paths.
- **Lazy swsCtx creation** — Created on first `toRGBA()` call; avoids crash from invalid pixel format before first HW decode.
- **HW frame transfer mutex** — `videoCodecMu` held during HW→SW transfer; eliminates concurrent AVCodecContext access.
- **HW decode codec filtering** — Only codecs that work without `get_format` callback get HW decode enabled.
- **AV_NOPTS_VALUE guard** — Skip frames with invalid PTS instead of passing them downstream.
- **D3D11VA flush guard** — `avcodec_flush_buffers` skipped before first decoded frame.
- **Safe HW frame download** — `av_hwframe_transfer_data` wrapped in recover/retry; falls back to SW decode on failure.

### Native Media Player — Audio / A-V Sync
- **A/V clock fix** — Master clock `SetSpeed()` wired after speed changes; no more 2× playback after resume.
- **AudioPlayer.Read() non-blocking** — Returns immediately on empty buffer; prevents playback hang.
- **Audio seek serialisation** — Codec operations serialized against `Seek()`; prevents hard crash from concurrent access.
- **Pause spin-loop fix** — Sleep instead of busy-wait; `Close()` no longer races with pause state.
- **Audio context pre-warm** — Created at startup to avoid WASAPI initialization hang.
- **SetSpeed deadlock fix** — Speed changes no longer block the audio callback thread.

### Native Media Player — SMPTE Bars & Idle State
- **SMPTE colour bars** — Click-to-load dialog when no video is loaded; consistent across all module players.
- **4:3 ratio with letterboxing** — Proper aspect ratio regardless of player size.
- **Dynamic sizing** — Bars scale to player widget dimensions instead of fixed 1920×1080.
- **Proportional idle text** — "Click to load video" text scales with bar width.

### Native Media Player — Misc
- **Native Fyne icons** — Replaced emoji transport controls with `theme.IconName` equivalents.
- **SmoothScrubbing crash fix** — Fixed crash on HW-decoded frames in thumbnail scrubber.
- **GrabFrame deadlock fix** — Invalid PTS frames no longer block the decode loop.
- **Letterbox fill** — Removed per-frame debug log; fixed fill colour on dark backgrounds.

### Convert Module
- **Clear button for output folder** — One-click reset of the output directory field.
- **Output directory creation** — Directories created before running convert/thumbnail/filter jobs.
- **Drag-drop first frame** — First frame appears immediately after file drop.

### Audio Module
- **VSplit layout** — Replaced custom HSplit with `container.NewVSplit`.
- **Stats bar footer** — Added to Audio module.
- **i18n** — All user-facing strings use i18n keys.
- **Drop label wrapping** — Cleaner layout on narrow windows.

### Thumbnail Module
- **3-way output mode** — Individual / Contact Sheet / Both selector replaces boolean toggle.
- **Image inspector** — Click any thumbnail or contact sheet tile to inspect at full window size.
- **Contact sheet pad crash fix** — `trim` filter removed; time window via `-ss`/`-t` input options.
- **CRLF line-break fix** — `\r` trimmed from ffprobe output on Windows.
- **Per-file "Add All to Queue"** — Creates individual jobs per file instead of a single batch.

### Subtitles Module
- **Video preview player** — Added video preview with synced subtitle overlay.

### CI & Build
- **libdrm-dev** — Added to Linux CI for FFmpeg build.
- **FFmpeg hwaccel disabled in CI** — Avoids libdrm runtime dependency.
- **Update status icon** — Replaced ⬤ with ● for cross-platform rendering.

### Misc
- **Temp file cleanup** — Preview-frame and cover-art temp files cleaned on video unload.
- **FFmpeg install button removed** — FFmpeg bundled in binary; redundant button removed.

## v0.1.1-dev41 (April 2026)

### Convert Module
- **Device presets** — Separated from format selector into a dedicated section; 11 presets (iPhone, Android, PS4, Xbox, Apple TV, etc.) each set ALL encoding variables (codec, bitrate, resolution, audio, pixel format).
- **User-defined presets** — Named presets that capture the full encoding configuration (codec, CRF, bitrate mode, frame rate, hardware accel, aspect, audio, etc.); save / apply / delete from the convert view; persisted to JSON via `appcfg`.
- **Batch output directory** — Batch convert jobs now respect the configured output directory instead of always writing beside the source file.
- **Source frame rate pinning** — `sourceFrameRate` stored in job config; `-r <fps>` explicitly passed to FFmpeg when "Source" is selected, preventing AVI→MP4 re-timestamping (e.g. 25 fps becoming 30 fps).
- **Device preset expansion** — HEVC and AV1 codec options added to device preset definitions.

### Upscale Module
- **Batch queue** — Multiple files can be drag-dropped into the upscale module; all are queued with full config (`targetWidth`, `targetHeight`, `sourceFrameRate`, `preserveAR`, `GroupID`) and the first valid video is shown for review.
- **Real-CUGAN support** — Model catalog abstraction added; Real-CUGAN available as an AI upscaling backend alongside Real-ESRGAN.

### Thumbnail Module
- **Contact sheet pad crash fixed** — `trim` filter removed from filtergraph; time window applied via `-ss`/`-t` input options instead. Eliminates "padded dimensions cannot be smaller than input dimensions" on Xvid/MPEG-4 ASP sources with MCU-padded coded dimensions.
- **Individual thumbnail timestamps fixed** — Timestamps were always `00:00:00.000` because input seek resets the PTS counter. Drawtext now hardcodes the formatted timestamp string (`HH\:MM\:SS.mmm`) for individually-seeked frames; contact sheet continues to use `%{pts:hms}` via `setpts`.
- **`setsar=1` added** — Normalises sample aspect ratio before scale so non-square-pixel sources render correctly.
- **Live preview grid** — Thumbnails appear in the module UI as they are generated. A persistent `GridWrap` container is updated in real time via `OnThumbGenerated`; the grid resets at the start of each new job. Layout redesigned: settings panel (32%) left, live preview panel (68%) right.

### Player Module
- **Routing bug fixed** — Loading a video while in the Player module no longer navigates to the Convert module. `loadVideo` now captures the active module and routes back to `showPlayerView()` when appropriate.
- **File state persistence fixed** — `state.playerFile` was never updated via the file-dialog load path, causing the view to rebuild showing "No video loaded." `OnPlayerFileLoaded` callback added to persist the probed source before the view rebuilds.
- **Layout** — When a video is loaded the player pane fills the full available space via `container.NewBorder` with the video as the expanding center; empty state shows centered instructions and load button.

### UI / Visual
- **Module color alignment** — Subtitles (`#AD741F`), Inspect (`#629C1C`), Upscale (`#2B9C1C`), and Compare (`#91931A`) interior colors now match their main menu button colors.

### DVD Authoring
- **UDF PartitionLength fix** — Calculated from actual file sizes instead of hardcoded 1000 sectors. VLC can now resolve UDF paths for full-size DVDs.
- **Menu PTS timestamps** — Menu VOB PTS was 300x too large (27MHz instead of 90kHz ticks), causing continuous VLC timestamp errors. Fixed.
- **Menu font embedding** — IBM Plex Mono TTF embedded in binary via `go:embed`; no more fallback to generic monospace on installed builds.
- **Chapter navigation** — Chapter cell boundaries now use binary search on actual NAV_PCK PTMs instead of linear interpolation, fixing chapter skip errors on VBR content.
- **Return-to-menu** — `JumpVMGM_PGCN(1)` post-command added to all title PGCs (feature + extras) in both folder and ISO builds.

## v0.1.1-dev40 (April 2026)

### Native Media Player
- **InlineVideoPlayer unified** — All modules (Convert, Trim, Inspect) now use `ui.InlineVideoPlayer` as the single API layer. No module manages its own engine or playback goroutine.
- **Inspect module player** — `GetInspectPlayer()` singleton wired; player widget embedded in Inspect view; `StartThumbnailExtraction` fires on load.
- **Inspect freeze fix** — `probeVideo` (ffprobe subprocess) moved off the main goroutine; UI renders immediately and refreshes when probe completes.
- **EOF handling** — Playback loop distinguishes clean end-of-stream from errors; `SetOnEnd` fires on main goroutine; play button resets correctly.
- **Dual-engine exceptions documented** — Compare and Upscale use two engines directly (required for simultaneous streams); documented in `AGENTS.md` and `docs/NATIVE_PLAYER.md`.

### Logging Standardisation
- **Module categories** — Added `CatConvert`, `CatTrim`, `CatMerge`, `CatFilters`, `CatAudio`, `CatAuthor` log categories.
- **Error elevation** — Config load, probe, and encode failures promoted from `Debug` to `Error` with full path context across Convert, Trim, Merge, Filters, Audio, and Author modules.

### UI / UX
- **Tooltip system** — Global enable/disable in Settings; tooltips on module tiles and Convert controls.
- **Drop targets** — `.ts`, `.m2ts`, `.vob` added to valid video extensions; toast notifications for wrong-format drops; green border hover feedback on drop zones.
- **Trim style** — Player and footer aligned to match other modules.

### Documentation
- **`docs/NATIVE_PLAYER.md`** — Full reference for the three-layer player stack, API surface, module integration pattern, and approved exceptions.
- **`AGENTS.md` Native Player section** — Architecture rule enforced; dual-engine exceptions documented.
- Deleted six stale player docs superseded by the above.

### Bug Fixes
- **Queue right-click safety** — Nil checks prevent crashes on context menu.
- **DnD dispatch** — `DropTarget` wired correctly for Convert and module tile drops.
- **Trim type assertion** — Removed invalid `Widget().(*media.VideoPlayer)` assertion.
- **CI** — Windows build green (runs 1184+); Linux green throughout.

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
- **Menu PGC sector patching (M5)** — `CellPlayback[0]` First/LastSector fields in each menu PGC updated with disc-absolute sector ranges derived from per-MPG file sizes and the `VIDEO_TS.VOB` disc start sector. Folder-mode equivalent added: sector offsets computed from individual MPG file sizes (cumulative), and `VMGM_VOBS_Sector` set to `VMG_Last_Sector+1` so libdvdread opens `VIDEO_TS.VOB` for the VMGM domain instead of treating it as absent.
- **VOB sector counter fix** — `WriteVideo` in `vob.go` no longer double-increments `currentSector`. The unconditional post-padding `currentSector++` has been restored to an `else` branch so `WritePadding` (which already increments) and the direct increment are mutually exclusive. This corrects `nv_pck_lbn` values in all NAV_PCKs for the menu VOB, fixing a VLC/dvdnav crash when navigating a generated DVD folder.
- **ExtrasMpg wiring (M6)** — `menuSet.ExtrasMpg` concatenated into `VIDEO_TS.VOB`; extras PGC built and included in the VMGM PGC table.
- **JumpVMGM_PGCN command (M7)** — `JumpVMGM_PGCNCommand(pgcN)` added to `internal/dvd/ifo/commands.go`; `ParseButtonCommand` now translates `"jump menu N;"` / `"jump menu pgc N;"` to the correct inter-menu PGC jump opcode (0x30, 0x06).

### Filter Integration (Complete)
- **Standalone filter jobs** — Filters module can now queue filter-only jobs without upscaling. "Add to Queue" button added to Filters module UI.
- **Filter job execution** — `executeFilterJob` supports color correction (brightness/contrast/saturation), enhancement (sharpness/denoise), transform (flip/rotate/grayscale), and stylistic filters (VHS/80s/Webcam effects) via FFmpeg.

### CI Fixes
- **Submodule sync** — Pushed missing commits to lt_mirror/fyne.git to fix CI failures.
- **filters_module.go** — Removed invalid `*videoSource` type assertion at line 110; `state.filtersFile` is `*videoSource` not `interface{}`, causing a Go 1.26 build failure in CI.
- **FFmpeg from source (Windows/Linux)** — Switched from BtbN pre-built packages to building FFmpeg, x264, and x265 from source. BtbN packages provide executables only (no `.a` static libraries) and bundle x264/x265 headers with `__declspec(dllimport)`, making static CGO linking impossible.
- **x265.pc C++ deps in Libs** — Moved `-lstdc++`/`-lsupc++`/`-lm` from `Libs.private` to `Libs` in the generated x265.pc. FFmpeg configure calls `pkg-config --libs` without `--static`, so `Libs.private` is never seen during the configure link test; the C++ runtime must be in the public `Libs` field.
- **Windows multiple-definition fix** — Stripped `-lsupc++` from `CGO_LDFLAGS` after `pkg-config --libs --static`. `libsupc++.a` (real static archive) and `libstdc++.dll.a` (DLL import stub) both define `std::type_info::operator==`; removing `-lsupc++` from the final Go binary link eliminates the duplicate symbol. `-lsupc++` remains in x265.pc for FFmpeg configure.
- **Windows disk space** — Added `-g0` to `CGO_CFLAGS` to suppress DWARF debug info in CGO intermediate `.s` files (FFmpeg headers otherwise generate multi-GB temp files in `C:\WINDOWS\TEMP`). Added MSYS2 package-cache/temp cleanup step before the Go build.
- **Linux cache invalidation** — Bumped Linux cache key `v2→v3` to force a rebuild with the corrected x265.pc after the `Libs.private` → `Libs` change.

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



