# VideoTools - Completed Features

## Version 0.1.1-dev45 (complete) - UI Polish & Parity

### Convert Module Improvements - Phase 1 (HIGH)
- **Audio Sample Rate dropdown** — `audioSampleRateSelect` wired in buildConvertView
- **Normalize Audio checkbox** — `normalizeAudioCheck` + LUFS/TruePeak sliders wired
- **Deinterlace Mode dropdown** — `deinterlaceModeSelect` + `deinterlaceMethodSelect` wired
- **H.264 Profile/Level controls** — `h264ProfileSelect` / `h264LevelSelect` wired; shown when H.264 codec is active

### Convert Module i18n (HIGH - Issue #5)
- **~42 hardcoded strings** i18n'd: checkboxes, buttons, dialog messages, back button
- **New keys added** to `internal/i18n/strings.go`, `en_ca.go`, `fr_ca.go`, `iu.go`, `iu_latin.go`

### Convert Module Improvements - Phase 2 (HIGH)
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

### Audio Module Phase 3 (MEDIUM) - done dev45
- **Enhanced track list** — Codec colors, language flags, duration display
- **Output naming preview** — Shows filename before extraction
- **Track reordering** — Up/down buttons (UI ready, logic wired)

### Audio Module Phase 1 (HIGH)
- **Consistent box styling** — Added `buildAudioBox()` helper, Convert-style boxes
- **Proper header bar** — `TintedBar` with module title + stats integration wired

### Upscale Module Improvements (dev44)
- **One-click presets** — Hobbyist SD→HD, Semi-Pro 1080p→4K, Anime, Restoration, Social Media workflows
- **UI clarity** — Preset dropdown with description labels, clear AI+RIFE workflow
- **Detection reliability** — VerifyTool() checks PATH + app-local bin + smoke test
- **Optimization guide** — See `docs/UPSCALE_OPTIMIZATION.md`
- **Hardware acceleration** — Sync upscale HW accel from master setting

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

### Module Pipeline (`&&` feature) (dev44)
- **Pipeline state machine** — `pipelineActive` on `appState` (off/waiting-step1/waiting-step2)
- **`&&` button** — Main menu header reflects state
- **Module tile dimming** — Invalid Step 2 targets dimmed
- **Queue integration** — `PipelineAfter` + `PipelineDeleteOnSuccess` fields on `queue.Job`
- **Intermediate files** — "Keep intermediate files" toggle in Settings → Preferences

### Logging Audit (dev45)
- **Remove unused categories** — `CatEnhance`, `CatRip` removed from `internal/logging/logging.go`
- **Add CatQueue** — New category for queue operations, wired in `queue.go` and `main.go`
- **Fix enhancement module** — `CatEnhance` → `CatModule` in `enhancement_module.go` and `onnx_model.go`
- **Fix rip module** — `CatRip` → `CatDisc` in `rip_module.go`
- **Fix recentfiles.go** — `CatSystem` misuse → `CatUI` via `cat()` helper

### FFmpeg DLL Bootstrap Fix (dev45)
- **Bundle DLLs in release** — FFmpeg shared DLLs built from source with all deps statically linked
- **Remove BtbN download** — `ffmpeg_bootstrap.go` no longer downloads from BtbN (eliminates `liblzma-5.dll` errors)
- **Build script** — `scripts/windows/build-ffmpeg-shared.ps1` builds FFmpeg shared DLLs from source
- **CI update** — `ci-build.ps1` bundles DLLs in `ffmpeg-dll/` subfolder (not root) within release ZIP
- **Legacy fallback** — `FFmpegDllDir()` still checks `%LOCALAPPDATA%\VideoTools\ffmpeg-dll` for old installs

---

## Version 0.1.1-dev44 (complete) - Playback & Sync Fixes

### Native Media Player — Playback & Sync Fixes (dev44)
- **Start/Resume state fix** — `Start()` was setting `e.paused=false` before starting the decode goroutine, but `Resume()` checked `!e.paused` and returned early. Decode loop never started on Play. Fixed: `Start()` now sets `e.paused=false` after launching goroutines.
- **Audio/video sync on load** — Audio clock drifted during `Load()` because `audioDecodeLoop` ran during `GrabFrame`, causing ~5 second offset before playback. Fixed by resetting clock to 0 in `ResetAfterGrab`.
- **Test pattern font** — Test pattern always renders with VCR OSD Mono font regardless of user preference.
- **FFmpeg bootstrap simplification** — Always downloads BtbN pre-built package to guarantee complete DLL set.

### Hardware Acceleration Parity (dev44)
- **Upscale module** — Initialize `upscaleHardwareAccel` from master `state.convert.HardwareAccel` so it's not empty on first use.
- **Filters module** — Add hardware acceleration dropdown with platform-appropriate options (nvenc, qsv, vaapi, amf on Linux; nvenc, qsv, amf on Windows). Wired to master setting via `HardwareAccel`/`SetHardwareAccel` callbacks.

### Audio Module Phase 2 — Native Player Integration (dev44)
- **InlineVideoPlayer singleton** — Added `GetAudioPlayer()` in `native_media.go` and stub in `native_media_stub.go`.
- **Video preview pane** — Added video container using `opts.Player.Widget()` in audio `BuildView()`, with idle state showing "DROP VIDEO TO LOAD" label.
- **Wired in audio_module.go** — Player passed to Options struct; video pane added to layout with HSplit (50/50 split).

### Settings
- **Player font preference** — Users can choose between IBM Plex Mono and VCR OSD Mono for the OSD. VCR OSD Mono has no Bold/Italic variants; UI gracefully falls back to Regular weight.

### Known Issues (pending)
- See `docs/PLAYER_DEBUG.md` for full list including: `predecodeFrom` sharing `formatCtx` with `demuxerLoop`, audio queue not flushed before seek, D3D11VA crashes when enabled.

## Version 0.1.1-dev43 (complete) - Player Stability Audit

### Native Media Player — Thread-Safety & Crash Fixes (dev43)
- [x] **Pixel format crash fix** — `GrabFrame` and `NextFrame` now use `frame.format` (actual decoded pixel format) instead of `videoCodecCtx.pix_fmt` when calling `ensureSwsCtx`. `videoCodecCtx.pix_fmt` can be `AV_PIX_FMT_NONE` until the codec processes its first SPS; passing it to `sws_getContext` returned nil, causing `sws_scale(nil, …)` → C SIGSEGV. `NextFrame` SW decode path was also missing the `ensureSwsCtx` call entirely.
- [x] **Close/demuxer race fix** — `Engine.Close()` previously freed `formatCtx` and `videoCodecCtx` immediately after `close(e.stop)`, while `demuxerLoop` may still be inside `av_read_frame`. Added `sync.WaitGroup` (`demuxerWg`) to `Engine`; `demuxerLoop` signals Done on exit, `Close()` waits before freeing any FFmpeg context.
- [x] **NextFrame/Close codec race fix** — `Close()` now acquires `videoCodecMu` before freeing `videoCodecCtx`, ensuring any in-flight `NextFrame` decode cycle has completed.
- [x] **seekLoop goroutine leak fix** — `InlineVideoPlayer.seekCh` was never closed, so the `seekLoop` goroutine leaked on every `Close()`. `seekCh` is now owned by `Load()`: closed and reallocated each time a file is opened; `Close()` closes it to drain the goroutine. `OnSeek` callback guards against nil/closed channel under the player mutex.

## Version 0.1.1-dev42 (complete) - Player Stabilization & Module Improvements

### Native Media Player — GStreamer Removal (dev42)
- [x] **GStreamer code removed** — All `internal/player/gstreamer*` deleted; `native_media` build tag is the only player path. No more GStreamer dependency.
- [x] **Player widget lifecycle** — `closeNativePlayer()` prevents audio hanging on module switch; `Widget().Refresh()` deferred after canvas swap to avoid blank panes.

### Native Media Player — D3D11VA / HW Decode Stabilisation (dev42)
- [x] **D3D11VA get_format callback** — `get_format` callback added to codec context; accepts `AV_PIX_FMT_D3D11VA_VLD` so D3D11VA decode starts on first packet.
- [x] **H.264 + D3D11VA crash fix** — Fixed `avcodec_send_packet` crash by pre-warming D3D11VA before first decode call.
- [x] **Dedicated HW frame buffers** — Separate `hwFramesCtx` prevents races between HW download and SW display paths.
- [x] **Lazy swsCtx creation** — `swsCtx` created on first `toRGBA()` call; avoids crash from invalid pixel format before first HW decode.
- [x] **HW frame transfer mutex** — `videoCodecMu` held during HW→SW transfer and RGBA conversion; eliminates concurrent AVCodecContext access.
- [x] **HW decode codec filtering** — Only codecs that work without `get_format` callback get HW decode enabled; prevents crashes on VC-1, MPEG-2 etc.
- [x] **AV_NOPTS_VALUE guard** — `GrabFrame` and `NextFrame` skip frames with invalid PTS instead of passing them to audio/player.
- [x] **D3D11VA flush guard** — `avcodec_flush_buffers` skipped before first decoded frame to prevent crash.
- [x] **Safe HW frame download** — `av_hwframe_transfer_data` wrapped in recover/retry; falls back to SW decode on failure.

### Native Media Player — Audio/A-V Sync (dev42)
- [x] **A/V clock double-speed fix** — Master clock `SetSpeed()` wired after speed changes; no more 2× playback after resume.
- [x] **AudioPlayer.Read() non-blocking** — Read returns immediately if buffer empty; prevents playback hang when audio underruns.
- [x] **Audio seek serialisation** — Codec operations serialized against Seek() to prevent hard crash from concurrent access.
- [x] **Pause spin-loop prevention** — Pause loop sleeps instead of busy-waiting; Close() doesn't race with pause state.
- [x] **Audio context pre-warm** — Audio context created at startup to avoid WASAPI initialization hang on first playback.
- [x] **SetSpeed deadlock fix** — Speed changes no longer block the audio callback thread.

### Native Media Player — SMPTE Bars & Idle State (dev42)
- [x] **SMPTE colour bars idle state** — Click-to-load dialog when no video is loaded; consistent across all module players.
- [x] **SMPTE bars in 4:3 ratio** — Letterboxing/pillarboxing for proper aspect ratio.
- [x] **SMPTE bars scaled to player size** — Dynamic sizing instead of fixed 1920×1080.
- [x] **SMPTE idle text scaled proportionally** — Text size adapts to bar width.

### Native Media Player — Misc (dev42)
- [x] **Native Fyne icons** — Replaced emoji transport controls (▶, ⏸, 🔊, ⛶) with `theme.IconName` equivalents.
- [x] **SmoothScrubbing crash fix** — Fixed crash on HW-decoded frames in thumbnail scrubber.
- [x] **GrabFrame deadlock fix** — Invalid PTS frames no longer block the decode loop.
- [x] **Letterbox fill performance** — Removed per-frame debug log; fixed fill colour on dark backgrounds.
- [x] **Initial frame display** — GrabFrame restored for first frame; skip only on subsequent calls to avoid crash.
- [x] **Panic recovery** — Added `recoverPanic` in GrabFrame, toRGBA, NextFrame, predecodeAhead, Load functions.

### Convert Module (dev42)
- [x] **Improvement plan** — Created `docs/CONVERT_MODULE_IMPROVEMENTS.md` with 10 phases covering missing UI controls, format presets, subtitle/audio track selection, video filters, metadata handling, presets, and i18n
- [x] **Clear button for output folder** — Added Clear button next to output directory in Convert settings.
- [x] **Output directory creation** — Ensure output directories exist before running convert/thumbnail/filter jobs.
- [x] **Drag-drop first frame fix** — `loadMultipleVideos` now calls `loadVideoNative` so the first frame appears immediately after drop.

### Audio Module (dev42)
- [x] **Improvement plan** — Created `docs/AUDIO_MODULE_IMPROVEMENTS.md` with 7 phases.
- [x] **VSplit layout** — Replaced custom HSplit with `container.NewVSplit` for consistency.
- [x] **Stats bar footer** — Added stats bar footer to Audio module.
- [x] **i18n all user-facing strings** — All Audio module labels and buttons use i18n keys.
- [x] **Drop label wrapping** — Wrapped drop label text for cleaner layout.

### Thumbnail Module (dev42)
- [x] **3-way output mode toggle** — Individual / Contact Sheet / Both selector replaces the old ContactSheet boolean.
- [x] **Image inspector** — Click any thumbnail or contact sheet tile to inspect at full window size.
- [x] **Contact sheet pad crash fix** — `trim` filter removed from filtergraph; time window via `-ss`/`-t` input options instead. Eliminates "padded dimensions cannot be smaller than input dimensions" on Xvid/MPEG-4 ASP.
- [x] **Contact sheet progress** — Full-panel display with live progress bar during generation.
- [x] **Contact sheet metadata** — Left-padding matched to logo (32px); metadata text vertically centred in header.
- [x] **CRLF line-break fix** — `\r` trimmed from ffprobe output to prevent `drawtext` line-break issues on Windows.
- [x] **TextWrapWord fix** — Placeholder label no longer stacks vertically in `NewCenter`.
- [x] **All to Queue per-file jobs** — "Add All to Queue" creates individual jobs per file instead of a single batch job.

### Subtitles Module (dev42)
- [x] **Video preview player** — Added video preview with synced subtitle overlay in Subtitles module.

### Snippet Module (dev42)
- [x] **Options drawer height** — Increased height to avoid unnecessary scrolling.

### CI & Build (dev42)
- [x] **libdrm-dev build dep** — Added `libdrm-dev` to Linux CI for FFmpeg.
- [x] **FFmpeg hwaccel disabled in CI** — GPU/hwaccel disabled in FFmpeg configure to avoid libdrm runtime dependency.
- [x] **Update script** — Linux binary replacement uses helper script.
- [x] **Update status icon** — Replaced ⬤ (black large circle) with ● (bullet) for reliable cross-platform rendering.

### Misc (dev42)
- [x] **Temp file cleanup** — Preview-frame and cover-art temp files cleaned up on video unload.
- [x] **FFmpeg install button removed** — FFmpeg is bundled in binary; redundant install button removed from Settings.

## Version 0.1.1-dev40 (in progress) - Burn Module & File Manager

### Upscale Module (dev40)
- [x] **Real-CUGAN support** — Added model catalog with Real-CUGAN (Pro, Standard, No Denoise)
- [x] **Model catalog abstraction** — Extensible ModelInfo struct for future models (SPAN, Waifu2x, etc.)
- [x] **Dual-binary execution** — Automatically selects correct ncnn binary based on model family
- [x] **Auto-download installer** — ensureAppLocalRealCUGAN() with GitHub releases integration
- [x] **Dependency UI** — Real-CUGAN install button in Settings → Dependencies

### DVD Authoring Fixes (dev40)
- [x] **UDF PartitionLength** — Fixed hardcoded 1000 sectors (~2MB) to `totalSectors - partitionStart`; VLC UDF path resolution now works for full-size DVDs
- [x] **Menu PTS timestamps** — Fixed menu VOB PTS incrementing by 27MHz ticks instead of 90kHz; was 300x too large, causing continuous VLC timestamp conversion errors
- [x] **Menu font** — Embedded IBM Plex Mono via `go:embed`; extracted to temp file for FFmpeg drawtext. No longer falls back to generic monospace on installed builds
- [x] **Chapter navigation** — Replaced linear interpolation (`ts/total*navCount`) with binary search on actual NAV_PCK PTMs for accurate chapter-to-sector mapping
- [x] **Return-to-menu** — Added `JumpVMGM_PGCN(1)` post-command to title PGCs in folder builds (was ISO-only); extras now also return to menu
- [x] **Extras return-to-menu** — Extra title PGCs get the same post-command as the main feature

### Burn Module (dev40) - completed dev45
- [x] **Design document** - Created docs/BURN_MODULE_DESIGN.md
- [x] **Module entry** - Wired showBurnView() in main.go
- [x] **UI implementation** - Source selection, drive detection, burn options
- [x] **Queue integration** - JobTypeBurn wired to executeBurnJob()
- [x] **Verify option** - Added checkbox with i18n support
- [x] **Drive capacity** - getDriveInfo() shows disc size in drive selector
- [x] **Main menu visibility** - Fixed burn/filemanager modules not appearing (were filtered out due to nil handler)
- [x] **Windows burn** - `isoburn.exe` (built-in) with eject via IOCTL_STORAGE_EJECT_MEDIA
- [x] **Linux burn** - `growisofs` (dvd+rw-tools) with progress parsing + SHA-256 verify
- [x] **Logging** - `CatBurn` category added, error handling improved

### File Manager (dev40)
- [x] **Design document** - Created docs/FILE_MANAGER_DESIGN.md with:
  - Lightweight UI with tabs, breadcrumbs, file list
  - Right-click context menu for module integration
  - Colour pills for module identification

### Quick Access Dropdown (dev40)
- [x] **Files dropdown** - Added to main menu header between sidebar toggle and Queue
- [x] **Context awareness** - Shows different options based on current module
- [x] **Open Files** - Button triggers OnOpenMore callback (placeholder)
- [x] **Open Output Folder** - Button triggers OnOpenFolder callback (placeholder)
- [x] **Recent Files** - Lists recent files with module context
- [x] **Design document** - Created docs/QUICK_ACCESS_DROPDOWN.md
- [x] **i18n** - All strings localized in en_ca, fr_ca, iu, iu_latin

### Queue Fixes (dev40)
- [x] **Right-click crash** - Fixed hard crash when right-clicking completed VIDEO_TS/author job
  - Added os.Stat check to detect directory outputs
  - Opens as folder instead of trying to probe as video

## Version 0.1.1-dev39 (complete) - Preview Tab, DVD Menu System & CI Green

### Player Crash Fixes (dev39)
- [x] **Panic recovery** - Added defer/recover in showPlayerViewForPath to catch CGO crashes
- [x] **Panic recovery** - Added panic recovery in loadVideoNative
- [x] **FFmpeg DLL** - Fixed local build script to copy FFmpeg DLLs to output
- [x] **FFmpeg DLL** - Use local FFmpeg at C:\ffmpeg\bin first (matches compilation)
- [x] **FFmpeg DLL** - Fall back to BtbN download only if no local FFmpeg found

### Author Module (dev39)
- [x] **Module extraction** - Extracted author module to `internal/app/modules/author/`
- [x] **Preview tab** - Added interactive Preview tab to Author module
- [x] **Video playback** - Preview tab can play videos by pressing menu buttons
- [x] **Tab visibility** - Preview tab shown only when Enable Menus is checked
- [x] **IFO audio track table** - VTS_MAT audio attributes now populated from actual track data (codec, channels, language) instead of hardcoded AC-3/stereo defaults
- [x] **Author drag crash fix** - `addAuthorFiles` moved off the UI thread; added `authorClipsRefresh` callback so drops no longer trigger a full 7-tab view rebuild (including GPU texture uploads) on the main thread during DnD completion
- [x] **VTS_MAT byte layout** - Fixed all field offsets in `mat_serialize.go` and `vtsi.go` to match libdvdread `vtsi_mat_t`; table offsets now at 0x0C8–0x0E4, title audio/video/subpicture attrs at correct positions, `vtsi_last_byte` and `vtstt_vobs` written; eliminates `zero_12`/`zero_17` violations and `ifoRead_VTS_PTT_SRPT failed` in dvdnav
- [x] **DVD menu VOB video (M1/M2)** - `runNativeSpumux` now encodes background PNG as MPEG-2 still video via ffmpeg and muxes video+SPU into proper DVD Program Stream VOB; falls back to video-only if SPU sub-stream mux fails
- [x] **PCI button table (M3)** - `PCIButton` struct added to `internal/dvd/vob/nav.go`; `WriteNAV_PCK` serializes up to 36 button entries with libdvdread-compatible coordinate packing at offset 98 within PCI payload; BTN_SL_NS/BTN_NS written at correct offsets 94/95
- [x] **VMGM_VOBS_Sector (M4)** - `vmgMat.VMGM_VOBS_Sector` set from `vtsSector("VIDEO_TS.VOB")` in ISO layout pass so dvdnav can locate the menu VOB on disc
- [x] **Menu PGC sector patching (M5)** - Each menu PGC `CellPlayback[0]` First/LastSector fields patched with actual disc sector range computed from per-MPG file sizes and the VIDEO_TS.VOB disc start sector; folder-mode equivalent added (cumulative file-size-based offsets, VMGM_VOBS_Sector set to VMG_Last_Sector+1 to ensure libdvdread opens VIDEO_TS.VOB)
- [x] **VOB sector counter fix** - `WriteVideo` restored to mutual-exclusive increment: `currentSector++` only in `else` branch when no padding; `WritePadding` handles it when padding is written. Fixes double-increment bug (introduced by opencode) that corrupted `nv_pck_lbn` in menu VOB NAV_PCKs → VLC/dvdnav crash
- [x] **ExtrasMpg wiring (M6)** - `menuSet.ExtrasMpg` concatenated into `VIDEO_TS.VOB`; extras PGC built and tracked in `menuMpgPaths` slice alongside main/chapters PGCs
- [x] **JumpVMGM_PGCN command (M7)** - `JumpVMGM_PGCNCommand(pgcN)` added to `internal/dvd/ifo/commands.go`; `ParseButtonCommand` translates `"jump menu N;"` / `"jump menu pgc N;"` to inter-menu PGC jump instructions

### CI Fixes (dev39)
- [x] **Submodule sync** - Pushed missing commits to lt_mirror/fyne.git
- [x] **filters_module.go build fix** - Removed invalid `.(*videoSource)` type assertion; `state.filtersFile` is already `*videoSource`, Go 1.26 CI failure fixed
- [x] **FFmpeg from source** - Switched to building FFmpeg/x264/x265 from source on both platforms; BtbN pre-built packages have no static `.a` libraries and DllImport-decorated x264/x265 headers
- [x] **x265.pc Libs fix** - C++ runtime deps moved from `Libs.private` to `Libs` so FFmpeg configure (which calls pkg-config without --static) sees them in its link test
- [x] **Windows multiple-definition fix** - Strip `-lsupc++` from CGO_LDFLAGS after pkg-config; prevents duplicate `std::type_info::operator==` between libsupc++.a and libstdc++.dll.a
- [x] **Windows disk space** - Added `-g0` to CGO_CFLAGS and MSYS2 cache cleanup step to prevent temp file exhaustion during Go build
- [x] **CI fully green** - Both Linux (run 1098) and Windows (run 1099) pass; release artifacts published (run 1100)

### Filter Integration (dev39)
- [x] **Create design document** - See docs/FILTER_INTEGRATION_DESIGN.md
- [x] **Add filters to Upscale module** - Integrate filter controls in upscale UI
- [x] **Refactor upscale pipeline** - Apply filters BEFORE upscale in encode chain
- [x] **Keep Filters module standalone** - Filters module can now queue filter-only jobs without upscaling; "Add to Queue" button added; executeFilterJob supports color correction, enhancement, transform, and stylistic filters via FFmpeg

## Version 0.1.1-dev38 (complete) - Module Extraction & Native Media Fixes

### CI Fixes (dev38)
- [x] **Windows CI** - Fixed `desktop.KeyEvent` → `fyne.KeyEvent` for new Fyne API
- [x] **Windows CI** - Fixed `fyne.Color` → `color.Color` for new Fyne API  
- [x] **Windows CI** - Fixed native_media linker error with VT_SUBTITLE_TYPE_TEXT
- [x] **Sub-agents** - Added usage guidelines to AGENTS.md for parallel task execution

### Native Go SPU Encoder (dev38)
- [x] **SPU encoder** - Added `MenuEncoder` to `internal/dvd/spu/spu.go`
- [x] **VOB WriteSPU** - Added `WriteSPU()` method to `vob.Muxer` in `internal/dvd/vob/vob.go`
- [x] **Menu wiring** - Replaced `runSpumux` calls with native `buildMenuSPU` in `author_menu.go`
- [x] **Zero-dep** - DVD menu generation now works without external spumux binary

### Module Extraction (dev38)
- [x] **Subtitles module** - Extracted to `internal/app/modules/subtitles/`
- [x] **Inspect module** - Extracted to `internal/app/modules/inspect/`
- [x] **Queue module** - Extracted to `internal/app/modules/queue/`
- [x] **Upscale module** - Extracted to `internal/app/modules/upscale/`
- [x] **Settings module** - Completed extraction to `internal/app/modules/settings/`

### Author Module i18n (dev38)
- [x] **i18n compliance** - Added 70+ Author* strings to i18n system
- [x] **Emoji removal** - Removed emoji from UI strings for portability
- [x] **Cross-platform** - Added spumux availability check with graceful fallback

## Version 0.1.1-dev37 (complete) - InlineVideoPlayer Wiring

### GPU Rendering Pipeline (NEW)
- [x] **Renderer interface** (`internal/media/gpu/renderer.go`) - Abstract GPU renderer with Texture interface
- [x] **OpenGL implementation** (`internal/media/gpu/opengl.go`) - OpenGL 4.6+ renderer scaffold
- [x] **Direct3D 11 implementation** (`internal/media/gpu/d3d11.go`) - D3D11 renderer scaffold for NVIDIA/AMD
- [x] **Texture utilities** (`internal/media/gpu/texture.go`) - Texture pooling, format conversion, scaling helpers
- [x] **Shader definitions** (`internal/media/gpu/shaders/`) - Vertex, fragment, and YUV→RGB shaders for GPU rendering
- [x] **Keyboard shortcuts** (`internal/media/gpu/shortcuts.go`) - Full shortcut handler (Space, arrows, F, M, 0-9, <>, etc.)
- [x] **Seekbar with thumbnails** (`internal/media/gpu/seekbar.go`) - ThumbnailCache, preview on hover, ThumbnailGenerator interface
- [x] **Volume control** (`internal/media/gpu/seekbar.go`) - VolumeControl widget with mute toggle
- [x] **FFmpeg filter pipeline** (`internal/media/filters/pipeline.go`) - Deinterlace, scale, color correction, denoise, sharpen, crop, rotate filters with presets
- [x] **VideoPlayer overlay controls** (`internal/media/view.go`) - Integrated player controls with play/pause, seek, volume, hover-to-reveal

### Playback Enhancements (NEW)
- [x] **Loading state** - SetLoading/IsLoading methods on Engine, loading spinner on VideoPlayer
- [x] **Playback speed control** - Speed button on VideoPlayer with preset speeds (0.25x-2x), SetSpeed/GetSpeed wired to Engine
- [x] **Chapter parsing** - Chapter struct, parseChapters() via FFmpeg, GetChapters() API on Engine
- [x] **Chapter support in VideoPlayer** - SetChapters() method, chapters stored in player state
- [x] **Chapter markers on seekbar** - Canvas.Raster overlay draws vtGreen tick marks at chapter boundaries
- [x] **Chapter navigation** - Prev/next chapter buttons, OnPrevChapter/OnNextChapter callbacks
- [x] **Thumbnail extraction** - ThumbnailExtractor for async keyframe extraction
- [x] **Smooth scrubbing** - SmoothScrubbing with FrameCache, pre-decodes frames ahead
- [x] **Resume playback** - ResumeState in internal/media/state, JSON config persistence
- [x] **Picture-in-Picture** - PiPController with 4 corner positions, Windows SetWindowDisplayAffinity
- [x] **Audio pitch correction** - TempoController with FFmpeg atempo filter (0.25x-2.0x)

### GPU Rendering (NEW)
- [x] **OpenGL texture upload** - GLTexture with glTexImage2D, GPUTextureUpload pool
- [x] **OpenGL context** - GLContext with shaders (vertex/fragment), VAO/VBO setup
- [x] **D3D11 context** - D3D11Context/D3D11Texture for Windows NVIDIA/AMD
- [x] **Thumbnail cache** - ThumbnailCache in gpu/seekbar.go with GetNearest()

### Buffering & Error Recovery (Phase 4)
- [x] **BufferMode type** - BufferModeMinimal, BufferModeNormal, BufferModeAggressive
- [x] **Adaptive buffer sizing** - GetAdaptiveBufferSize() returns 10/50/100 based on mode
- [x] **Decode time tracking** - recordDecodeTime(), GetAverageDecodeTime() for performance monitoring
- [x] **Error recovery types** - PlaybackError struct with ErrCode* constants (Decode, Network, HWAccel, FileCorrupt, CodecMissing)
- [x] **Retry logic** - RecoverableError() and ShouldRetry() methods for transient error handling

### Video Filters (Phase 6)
- [x] **FilterPipeline integration** - SetFilterPipeline, GetFilterPipeline wired into Engine
- [x] **Filter API** - SetFilter, EnableFilter, ClearFilters, GetFilterGraph methods
- [x] **Preset support** - SetPreset with vintage, warm, cool, high_contrast, soft, vivid

### Picture-in-Picture (Phase 8)
- [x] **PiPController** - PiPController struct with Enable/Disable/Toggle/IsEnabled
- [x] **PiP positions** - TopLeft, TopRight, BottomLeft, BottomRight
- [x] **Windows PiP implementation** - SetWindowDisplayAffinity via user32.dll
- [x] **PiP stub for non-Windows** - Cross-platform compilation support
- [x] **PiP button in VideoPlayer** - togglePiP, IsPiP, OnPiP callback

### Resume Playback (Phase 9)
- [x] **ResumeState** - PlaybackPosition struct, JSON persistence
- [x] **SavePosition** - Saves position/duration/volume/speed to config
- [x] **GetPosition** - Retrieves saved position with expiry (30 days)
- [x] **ShouldResume** - Logic for >5% remaining, <7 days old
- [x] **Trim integration** - Checks saved position on load, seeks to it
- [x] **Auto-save** - Periodic position saving every 5 seconds during playback

### Subtitle Rendering (Phase 10)
- [x] **Subtitle overlay** - SubtitleOverlay struct with Bounds() method
- [x] **RenderSubtitles** - Draws subtitle background on video frame
- [x] **Subtitle decoding** - initSubtitleDecoder, decodeSubtitle, UpdateSubtitles
- [x] **Subtitle track selection** - SelectSubtitleTrack, DisableSubtitles
- [x] **Subtitle toggle button** - CC button in VideoPlayer controls
- [x] **Subtitle callbacks** - OnSubtitles, IsSubtitlesEnabled, SetSubtitlesEnabled
- [x] **Subtitle text rendering** - Bitmap-style text drawing with configurable alpha

### GPU Rendering (Performance)
- [x] **Fast scaling** - scaleNearest() with nearest-neighbor interpolation
- [x] **Display frame tracking** - displayWidth, displayHeight, frameSeq for caching
- [x] **Optimized draw loop** - Pre-calculated scaling factors, direct pixel access
- [x] **Bicubic scaling** - SWS_BICUBIC|C.SWS_ACCURATE_RND for better quality
- [x] **HW decode support** - VAAPI/D3D11VA/QSV hardware acceleration
- [x] **Thumbnail cache** - In-memory cache of thumbnails keyed by timestamp
- [x] **GetHoverFrame** - Get nearest cached thumbnail for hover preview
- [x] **AddThumbnailFrame** - Add frames to thumbnail cache during playback
- [x] **OnHover callback** - Trigger thumbnail extraction on seekbar hover
- [x] **FrameCache** - Pre-decoding frames for smooth scrubbing in scrub.go
- [x] **Async thumbnail extraction** - StartThumbnailExtraction with callback
- [x] **PlaybackFrameCache** - Engine frame cache for smooth scrubbing
- [x] **InitFrameCache** - Initialize frame cache with configurable size

### Fyne Fork for GPU Texture Optimization
- [x] **Fork created** - https://git.leaktechnologies.dev/lt_mirror/fyne
- [x] **TexSubImage2D** - Added to GL context interface for efficient texture updates
- [x] **All GL backends** - Implemented in gl_core.go, gl_es.go, gl_gomobile.go, gl_wasm.go
- [x] **UpdatePixels method** - Added to canvas.Raster for efficient pixel data updates
- [x] **Texture reuse** - newGlRasterTexture now reuses cached textures when size matches
- [x] **VideoTools integration** - go.mod uses replace directive to lt_mirror/fyne
- [x] **VideoPlayer wired** - SetFrame() uses UpdatePixels() when canvas size matches, enabling TexSubImage2D path

### Crash Logging & Error Recovery
- [x] **FFmpeg error logging** - avformat_open_input now logs FFmpeg error codes and messages
- [x] **avformat_find_stream_info logging** - Logs return code and error string on failure
- [x] **Stream detection logging** - Logs video/audio/subtitle stream indices
- [x] **Panic recovery in trim module** - loadVideo and playbackLoop have RecoverPanic
- [x] **Full goroutine dump** - LogAllGoroutines dumps all goroutines to crash log
- [x] **RecoverPanicWithCallback** - New logging helper with callback option
- [x] **Panic recovery in inspect module** - inspectLoadVideo and inspectPlaybackLoop have RecoverPanic

### VideoPlayer Consistency Across Modules
- [x] **Inspect module VideoPlayer** - Added `inspectState` struct with player and engine fields
- [x] **Inspect playback callbacks** - OnPlay, OnPause, OnSeek, OnSpeedChange wired to engine
- [x] **inspectPlaybackLoop** - Frame loop mirrors trim module pattern
- [x] **inspectLoadVideo** - File loading with engine initialization, mirrors trim module
- [x] **VideoPlayer widget integration** - Replaced static preview images with full VideoPlayer widget

### Loading & Error States (Phase 7)
- [x] **Buffering indicator** - SetBuffering/IsBuffering methods with "Buffering..." label
- [x] **Error state** - SetError/ClearError/HasError with red indicator + message
- [x] **Seeking indicator** - isSeeking state, FinishSeeking method
- [x] **Error logging** - Tapped on error logs message and shows crash log path

### i18n Strings (NEW)
- [x] **Player strings** - PlayerLoading, PlayerBuffering, PlayerSpeed, PlayerChapter, PlayerChapters, PlayerNoChapters, PlayerSpeed* (8 new strings)

### Bug Fixes (tester feedback)
- [x] **MKV 0 kbps bitrate tag** — Hardware encoders (AMF, NVENC) don't write per-stream BPS stats to Matroska; inject `-metadata:s:v:0 BPS=<bps>` for CBR/VBR MKV output so Windows Explorer and media tools display correct bitrate.
- [x] **Queue list flash with multiple jobs** — `UpdateJobs` now only calls `jobList.Refresh()` on structural changes; individual widget updates handle their own redraws. Eliminates rapid full-list redraws causing flicker. `Scroll.Refresh()` also called to fix blank body with multiple pending jobs.
- [x] **Queue status sidebar colour** — `statusRect` now tracked in `queueItemWidgets` and updated on status transition (Pending→Running colour change without rebuilding the whole card).

### CI & Packaging
- [x] **AppImage icon** — Switched from `VT_logo.png` (1024×1024, rejected by linuxdeploy) to `VT_Logotype1.png` (512×512). Updated `Icon=` in `VideoTools.desktop` to match icon filename stem; fixes "Could not find suitable icon" AppImage build error.

### DVD Authoring Engine (Phase 1–4.3)
- [x] **IFO command table** — `DVDCommandTable`, `JumpTTCommand`, `SetHL_BTNNCommand`, `ParseButtonCommand`, `SerializeCommandTable` in `internal/dvd/ifo/commands.go`.
- [x] **Menu PGC + VMG_PGCITI** — `BuildMenuPGC`, `WritePGCITI` wire button commands into VMG IFO; `GenerateVMG_IFO` accepts menu PGC; menu pipeline fully wired in `author_module.go`.
- [x] **TMAPT (time map table)** — `BuildLinearTMAPT` / `WriteTMAPT` in `vtsi.go`; linear sector approximation from VOB file size; wired into `GenerateVTS_IFO`.
- [x] **ISO 9660 hybrid disc** — Full path tables (L+M), directory records, shared file data sectors with UDF; `assignSectors` runs first so ISO 9660 can reference correct physical sector addresses.
- [x] **SPU DCSQ rewrite (Phase 4.2)** — Complete rewrite of `spu.go`: correct DCSQ header layout with `next_dcsq` offset, `SET_COLOR`/`SET_CONTR` commands with configurable `SPUOptions`, `SET_AREA` with `pack12pair`, `SET_ADDRESS` with computed field offsets, self-referencing DCSQ[1] terminator. `DefaultSPUOptions()` added.
- [x] **Integration tests (Phase 4.3)** — 30 tests across `spu`, `ifo`, `vob`, `udf` packages: SPU packet structure/commands/terminator/offsets, PGCITI sector padding/NrOf/Category, TMAPT entry count/bounds/header, NAV_PCK size/start codes/LBN/SRI, UDF/ISO 9660 PVD magic/VDS terminator/system area. All green.

### Localization (i18n) - dev34 carry-forward
- [x] **Subtitles i18n strings** — Added 9 new strings to `internal/i18n/strings.go` (SubtitlesOfflineHint, SubtitlesEmpty, SubtitlesExtractEmbed, SubtitlesOCROutput, SubtitlesOCRLanguage, SubtitlesShiftOffset, SubtitlesStart, SubtitlesEnd).
- [x] **French (fr-CA) translations** — Subtitles module fully translated.
- [x] **Audio i18n wired** — 5 new strings wired up in `internal/app/modules/audio/view.go`.
- [x] **Filters i18n wired** — 24 new strings wired up in `internal/app/modules/filters/view.go`.
- [x] **Inspect i18n wired** — 3 new strings wired up in `internal/app/modules/inspect/view.go`.
- [x] **Settings StatusNoActiveJobs** — Added to `internal/ui/components.go` status bar.
- [x] **Dialog title i18n** — 15+ new translation keys (DialogInterlacingResults, DialogAutoCropDetection, DialogNoBlackBars, DialogQueueNotInit, DialogNoRunningJob, LabelSnippet, MergeStarted, TrimJobAdded) wired into main.go for Convert/Merge/Trim/Snippet modules.

### Media Engine Overhaul
- [x] **SplitView fixes** — Fixed divider color using exact VT Green #4CE870; implemented MouseMoved/Dragged for draggable divider; added SetOnDividerMove callback.
- [x] **AudioPlayer improvements** — Added volume control (SetVolume/GetVolume), mute functionality (SetMuted/IsMuted), pause/resume control, proper error handling with logging, fixed resample buffer handling.
- [x] **Engine enhancements** — Added VideoInfo struct for metadata, Pause/Resume/TogglePause controls, volume/mute/speed controls, seeking with configurable accuracy (Frame/Keyframe/Accurate), CurrentTime() and QueueStats() methods.
- [x] **Queue improvements** — Added configurable max size limits, NewPacketQueueWithMaxSize constructor, SetMaxSize/GetMaxSize/IsFull methods.
- [x] **Subtitle extraction** — New `subtitle.go` with SubtitleExtractor for parsing subtitle streams, SRT and ASS export, SubtitleTrack and Subtitle types.
- [x] **Tests** — Added comprehensive tests for queue, clock, and utility functions in `media_test.go`.
- [x] **Player deprecation** — Marked BackendMPV and BackendVLC as deprecated; factory now only supports FFplay and Native engines.

### Module Updates
- [x] **Trim module stub** — Updated `internal/app/modules/trim/stub.go` to match main.go calls (`ModuleColor`, `OnShowQueue`, `OnAddToQueue`, `TrimClip` struct, second `initialPath` param).
- [x] **Trim view** — Added `TrimClip` struct and `OnAddToQueue` callback to native trim view.
- [x] **Trim handler** — Fixed `internal/modules/handlers.go` to use correct logging category.
- [x] **Trim job submission** — `submitTrimJob` creates queue.Job with proper Type, InputFile, OutputFile, and Config.
- [x] **Settings module extraction** — Moved tab builders to `internal/app/modules/settings/tabs.go`. Created callback interfaces (BenchmarkCallbacks, PreferencesCallbacks, DependencyCallbacks) for loose coupling. Reduced settings_module.go from 2316 to ~1700 lines.
- [x] **Inspect module extraction** — Moved `showInspectView` and `buildInspectView` to `internal/app/modules/inspect/view.go`. Root `inspect_module.go` is thin shim.
- [x] **Queue module extraction** — Moved queue view builders and refresh helpers to `internal/app/modules/queue/view.go`. Root `queue_module.go` delegates to internal package.
- [x] **Subtitles module extraction** — Moved package structure, types, adapter, and view code to `internal/app/modules/subtitles/`.
- [x] **Upscale module helpers** — Full module extracted to `internal/app/modules/upscale/` with helpers.go, types.go, and view.go. Root `upscale_module.go` is thin shim delegating to internal package.

### UI Fixes
- [x] **Back button consistency** — Module name uppercase on all modules.
- [x] **Auto-check dropdown** — Fixed language switching issue in Settings Updates section.
- [x] **Thumbnail contact sheet** — Increased header height (130→150px), added filename truncation.
- [x] **Inspect preview placeholder** — Replaced stuck "Loading preview" with proper idle player state and icons.

### Interlace Detection
- [x] **Preview frame capture** — Capture preview frames before running interlace analysis to avoid UI stuck states.

### Hardware Acceleration Detection Fix
- [x] **Runtime HW detection** — `hwAccelAvailable()` now does actual encode probes for each method (NVENC, QSV, VAAPI, VideoToolbox, AMF) instead of just checking `ffmpeg -hwaccels`. Prevents false positives like QSV being auto-selected on laptops without Intel GPUs.
- [x] **HW decode support** — Native media engine now supports GPU-accelerated video decoding via FFmpeg's hwcontext API. Supports VAAPI (Linux), D3D11VA (Windows), and QSV (Windows/Linux). Automatic fallback to software decoding if HW unavailable.

## Version 0.1.1-dev35 (2026-03-16) - Native Media Engine & Trim Module

- [x] **Trim Module UI** — Implemented a professional, dual-pane layout for the Trim module, matching the Convert module's "source of truth" visual style.
- [x] **Native VideoPlayer Widget** — Created a reusable `media.VideoPlayer` widget in the native FFmpeg-CGO engine for high-performance single-video playback.
- [x] **Trim Localization** — Added full i18n support for the Trim module (English, French, Inuktitut).
- [x] **Compare Native Integration** — Refactored the Compare module to use the native `SplitView` and dual-engine playback loops.

## Version 0.1.1-dev33 (2026-03-14) - Native Authoring Foundation

### Native DVD Engine
- [x] **Wiki synchronization** — Ported internal documentation to the Forgejo wiki with corrected links and navigation. Established Home, Documentation, and Sidebar pages.
- [x] **Native DVD Engine structure** — Created `internal/dvd/` modular package structure (`udf`, `ifo`, `vob`, `spu`).
- [x] **MPEG-PS / VOB Muxer Foundation** — Implemented MPEG-PS packetization, Pack/System/PES headers, and DVD-specific Navigation Pack (NAV_PCK) structures.
- [x] **IFO/BUP Structure Generation** — Implemented binary serialization for VTSI and VMGI tables, and created `IFOBuilder` for automatic IFO/BUP and backup file creation.
- [x] **SPU subpicture encoder** — 2-bit RLE subpicture encoder for DVD menu button highlights.
- [x] **UDF reader foundation** — UDF 1.02 disc type detection and reader scaffolding in `internal/dvd/udf`.
- [x] **Authoring Architecture Consolidation** — Unified DVD and Blu-ray workflows into the core Author and Rip modules. Removed the redundant standalone Blu-ray module to streamline UI/UX.
- [x] **Dependency Cleanup** — Fully removed `dvdauthor` and `xorriso` as dependencies. Author and Rip modules are now enabled cross-platform by default with optional visibility toggles in Settings.

### UI Alignment (dev33 polish pass)
- [x] **Module UI alignment** — Convert, Thumbnail, Filters, Audio, Compare, and Inspect module layouts aligned with the standardised Convert module style (consistent labels, separators, padding).

### Thumbnail / Contact Sheet
- [x] **IBM Plex Mono applied** — `MonoTheme` now set on the Fyne app at startup; IBM Plex Mono Regular and Bold used throughout the UI and in contact sheet text overlays.
- [x] **Bold title font in contact sheet** — Filename (line 1) rendered with IBM Plex Mono Bold; metadata lines 2/3 use Regular.
- [x] **VT Green contact sheet title** — Line 1 (filename) colour changed from white to `#4CE870`, matching the main menu "VideoTools" title.
- [x] **Contact sheet line-3 wrapping fixed** — FFmpeg `drawtext` treats `|` as a newline; replaced all ` | ` separators with ` · ` (U+00B7).
- [x] **Contact sheet progress bar** — `-progress pipe:1` flag was appended after the output path; moved before it so FFmpeg emits progress events correctly.
- [x] **Duplicate ffprobe calls eliminated** — `buildMetadataFilter` no longer calls `getVideoInfo`/`getDetailedVideoInfo` internally; pre-computed data passed from `generateContactSheet`, reducing ffprobe invocations from 6-7 down to the minimum needed.
- [x] **CMD windows suppressed** — All `exec.Command` calls in `internal/thumbnail` now call `hideCmd()` (platform-specific: `SysProcAttr{HideWindow: true}` on Windows, no-op elsewhere).

### Fixes
- [x] **Scroll passthrough on Entry/Select widgets** — Mouse wheel events were swallowed when hovering over text inputs or dropdowns inside `FastVScroll` panels. Made `scrollClip` implement `fyne.Scrollable` with forwarding to `FastVScroll`; added `IsClip()` to renderer for correct GL scissoring.
- [x] **Icons not loading** — `GetIcon` was reading from the embed root rather than the icons subdirectory. Fixed by passing `fs.Sub(iconsFS, "assets/icons")` to `ui.SetIconsFS()`.
- [x] **App icon path** — `logoAssets.Open` was using a bare filename; corrected to `assets/logo/VT_Icon.ico`.
- [x] **CI missing imports** — `internal/ui/components.go` was missing `"fyne.io/fyne/v2/driver/desktop"` and `"fyne.io/fyne/v2/layout"` imports; fixed to unblock Linux and Windows CI builds.

## Version 0.1.1-dev32 (2026-03-12) - UI Polish and Fixes

### Kickoff
- [x] Bumped version markers to v0.1.1-dev32 (main.go, VERSION, FyneApp.toml).

### Icons
- [x] SVG icon library added - ~150 Material Design SVG icons added to `assets/icons/`; ASCII icon placeholders replaced with real icon resources across the UI.
- [x] Icons embedded into binary (issue #20) - `icons_embed.go` uses `//go:embed assets/icons` so icons are baked in at compile time; `ui.SetIconsFS()` / `GetIcon()` rewritten to read from `fs.FS` with no runtime disk access. Resolves blank icons on installed builds.

### Settings — Dependencies
- [x] Platform-filtered dependency list - Dependencies tab now only shows entries relevant to the current platform using `isDependencyAvailableForPlatform`.
- [x] Install buttons per dependency - Each dependency row shows an actionable Install button; FFmpeg on Windows uses the existing app-local bootstrap.
- [x] Uninstall buttons - Uninstall button shown per dependency when an uninstall command is available.
- [x] WSL auto-install reverted - Installing Ubuntu via WSL would consume 5-10 GB; unacceptable for a lightweight app. dvdauthor/xorriso platforms set to `["linux","darwin"]` only.
- [x] Updates tab — Forgejo tags API wired - Check for Updates now hits `/api/v1/repos/leak_technologies/VideoTools/tags?limit=1`; compares against `appVersion`; fixed owner mismatch (`/stu/` → `/leak_technologies/`).
- [x] Disc module toggles hidden on Windows - Author, Rip, and Blu-ray visibility checkboxes in Settings are hidden on Windows since dvdauthor/xorriso are unavailable on that platform.
- [x] cmd window popups suppressed on Windows - All `exec.Command` calls in settings and WSL utilities replaced with `utils.HideWindowExec`/`utils.HideWindowExecContext` (`SysProcAttr{HideWindow: true}`).

### Modules — Convert
- [x] Player layout fixed - Video pane used `container.NewVBox` which collapsed the canvas.Image to 0px; fixed with `container.NewBorder` (transport bar pinned bottom, video fills centre).
- [x] Player layout — VSplit gap fixed - `container.NewVBox(videoPanel, leftGap)` was leaving dark empty space in VSplit top half; `videoPanel` now passed directly to `container.NewVSplit`.
- [x] Player icons fixed - ASCII fallback labels (`-/`, `-|`, `|-`) replaced with `widget.NewButtonWithIcon` using embedded SVG icons (play_pause, skip_previous, skip_next).
- [x] `s.active` never set to "convert" fixed - `showConvertView` now sets `s.active = "convert"` so drop handling, keyboard shortcuts, and all `if s.active == "convert"` guards work correctly.
- [x] `s.source` not updated on single-video load fixed - `loadVideo` now sets `s.source = src` before calling `showConvertView`.
- [x] Convert UI cleanup (issue #5) - Label alignments standardised, consistent separators added.

### Modules — Compare
- [x] Hide/show player toggle (issue #1) - Compare module now has a toggle button to hide/show both video players, giving more vertical space for the diff view.

### Modules — Author / Rip
- [x] Hidden on Windows - Author and Rip modules are hidden from the main menu on Windows until cross-platform disc authoring is implemented.

### Navigation
- [x] Mouse back/forward buttons - Side mouse buttons (button 4/5) trigger back/forward navigation.
- [x] Mouse back button fixed - Back button now returns directly to main menu.
- [x] Keyboard shortcuts simplified - Ctrl+Enter is the universal confirm shortcut on Linux/Windows; Author module wired.

### UI
- [x] Main menu tile colour consistency - Unavailable module tiles now show dimmed module colour on first load, matching post-navigation appearance.
- [x] Drag-to-scroll on FastVScroll (issue #19) - `container.Scroll` implements `fyne.Draggable` but discards desktop drag events via a mobile-only guard. Replaced inner scroll with a custom `scrollClip` widget that does not implement `fyne.Draggable`, allowing drag events to reach `FastVScroll`.
- [x] Pulsing drop indicator on video stage - Video drop zone pulses when a draggable file is hovered over the convert player area.
- [x] FastVScroll on upscale settings and convert metadata - Both panels now use FastVScroll for consistent drag-to-scroll.

### Auto-Update
- [x] In-app updates - Windows and Linux builds support in-app auto-update via Forgejo releases API.

## Version 0.1.1-dev31 (2026-03-12) - UI Stability and Cleanup

### Kickoff
- [x] Bumped version markers to v0.1.1-dev31 (main.go, VERSION, FyneApp.toml).
- [x] Created Forgejo issue tracker from known issues and carry-forward items.
- [x] Closed dev30 with CI validation confirmed (runs 219/220/221, commit 2cbb3a2).

### UI
- [x] Module settings scrolling (issue #3) - Scroll containers added to all non-Convert module settings panels; primary action buttons moved to footer action bar for Rip, Subtitles, Filters, Thumbnail, Merge.
- [x] Window resize stability (issue #4) - setContent pins window to pre-switch size to prevent layout-driven resize on module change.
- [x] Convert video pane overflow - Removed rigid SetMinSize from loaded-video stage; VSplit 50/50 offset now holds correctly.
- [x] Convert module vertical layout - Changed left column to VSplit with 50/50 split between video player and metadata.
- [x] WSL compile fix - Fixed undefined windowsToWSLPath (wrong capitalisation) breaking both CI platforms.
- [x] Click-and-drag scrolling - FastVScroll now implements desktop.Mouseable and fyne.Draggable so users can click-and-drag content to scroll, mirroring mobile/touch behaviour.

### Author Module
- [x] Menu templates - Added Minimal template and separated templates from themes.
- [x] Menu themes - Added 8 preset themes (VideoTools, Minimal, Western, Film Noir, Classic Hollywood, Warm Cinema, Ocean, Nature).
- [x] Custom background for all templates - Background image now available for all template types.
- [x] Motion backgrounds - Added support for video loop backgrounds (MPG) with embedded audio.

### Refactor
- [x] Phase 3 slice — Player and Enhancement extracted from `main.go` into `player_module.go` and `enhancement_module.go`.
- [x] Phase 3 slice — Upscale view moved from `main.go` into `upscale_module.go` alongside existing helpers.
- [x] Phase 3 slice — Compare and Compare Fullscreen views moved from `main.go` into `compare_module.go`.
- [x] Convert module partial modularisation - Added `ShowView`, `ConvertState`, and `ConvertCallbacks` to `internal/app/modules/convert/view.go`; added `showConvertView` shim and type-converter helpers in `main.go`. Full `buildConvertView` extraction deferred due to high coupling with `appState` (~3,500 lines, ~30+ state fields).
- [x] WSL ISO creation on Windows - Added `internal/utils/wsl.go` with WSL detection, path conversion, and ISO tool detection for consistent DVD ISO generation on Windows.

### Documentation
- [x] Author menu templates scope - Added comprehensive TODO section for menu templates (Minimal, Classic, Grid, Filmstrip, Poster, Cinematic) and themes (Minimal, Classic Hollywood, Film Noir, VideoTools, Warm Cinema, Ocean, Nature, Custom).

## Version 0.1.1-dev30 (2026-03-04) - Development Cycle Kickoff

### Maintenance
- [x] Bumped app version metadata to v0.1.1-dev30 (main.go, VERSION, FyneApp.toml).
- [x] Updated dev release publishing to append the current version changelog section to the nightly release notes.
- [x] Documented versioning policy: continuous global `-devN` numbering with public releases using base versions only.
- [x] Added a full module testing checklist and public release gate criteria for deciding when to bump to the next public version.
- [x] Cleaned root structure by removing stray artifacts, relocating the QR demo entrypoint under `cmd/`, and adding repository hygiene rules to `AGENTS.md`.
- [x] Added a phased dev30 refactor plan (`docs/REFACTOR_DEV30_PLAN.md`) to guide gradual package/entrypoint cleanup.
- [x] Started Phase 2 refactor by moving module config path logic into `internal/app/configpath` and updating all module callers.
- [x] Continued Phase 2 refactor by moving merge/thumbnail config persistence into `internal/app/modulecfg` while keeping stable `package main` wrappers.
- [x] Continued Phase 2 refactor by moving naming metadata/output-base helper logic into `internal/app/naming` with compatibility wrappers.
- [x] Continued Phase 2 refactor by moving rip/subtitles config persistence into `internal/app/modulecfg` with compatibility wrappers.
- [x] Continued Phase 2 refactor by moving author config persistence into `internal/app/modulecfg` with compatibility wrappers.
- [x] Continued Phase 2 refactor by moving audio config persistence into `internal/app/modulecfg` with compatibility wrappers.
- [x] Continued Phase 2 refactor by replacing duplicated config-path helpers in `main.go` with shared `internal/app/configpath` lookups.
- [x] Continued Phase 2 refactor by moving recovery/benchmark/history config persistence into `internal/app/appcfg` with aliases/wrappers in `main.go`.
- [x] Continued Phase 2 refactor by moving convert config JSON load/save plumbing into shared `internal/app/appcfg` store helpers.
- [x] Continued Phase 2 refactor by moving convert config normalization rules into `internal/app/appcfg` with thin wrapper calls in `main.go`.
- [x] Fixed Forgejo Linux/Windows package build break by restoring `path/filepath` import in `audio_module.go` after refactor.
- [x] Fixed Forgejo Linux/Windows package build break by restoring `path/filepath` import in `rip_module.go` after refactor.
- [x] Updated Forgejo publish workflow to read version from `VERSION` first, patch matched release metadata, and keep dev updates scoped to the intended tag.
- [x] Fixed Forgejo Linux/Windows package build break by restoring missing `encoding/json` and `path/filepath` imports in `subtitles_module.go` after refactor.
- [x] Fixed convert drag/drop analysis on Windows by using the configured FFprobe path instead of a hardcoded `ffprobe` command in `probeVideo`.
- [x] Fixed thumbnail metadata probing to use the configured FFprobe path so app-local FFprobe works when PATH does not include FFprobe.
- [x] Simplified Forgejo dev release notes to publish concise version highlights instead of dumping the full changelog section into the release body.
- [x] Added a stale-run publish guard in Forgejo dev release workflow so only the latest `master` commit updates release metadata/assets.
- [x] Started Phase 3 refactor by moving About dialog UI implementation into `internal/app/modules/about` with a thin `package main` shim.
- [x] Continued Phase 3 refactor by moving missing-dependencies dialog rendering into `internal/app/modules/deps` with a thin `package main` shim.
- [x] Updated About/QR documentation links to use the Forgejo wiki URL after retiring `docs.leaktechnologies.dev`.
- [x] Updated installation/readme docs to point users at in-repo docs and Forgejo wiki as the active documentation locations.
- [x] Continued Phase 3 refactor by moving main menu visibility/dependency filtering and active-job mapping helpers into `internal/app/modules/mainmenu`.
- [x] Added `docs/DEV30_FINALIZATION_CHECKLIST.md` to formalize dev30 closeout gates (CI, smoke tests, dependency checks, docs, tagging, and dev31 kickoff).
- [x] Expanded `AGENTS.md` with a full `dev30` closeout and `dev31` handoff brief so a new coding agent can take over without reconstructing project state.

## Version 0.1.1-dev29 (2026-03-03) - Build and Runner Stabilization

### Build/CI
- [x] Fixed dev-packages.yml YAML parsing in the Windows bundled dependency note block.
- [x] Fixed module import paths after queue/main menu modularization so vendored builds compile correctly.
- [x] Fixed duplicate package main declaration in mainmenu_module.go that broke Windows packaging.
- [x] Fixed convert/aspect compile regressions from duplicate scaling block and late custom-aspect declarations.
- [x] Removed stale `go-qrcode` import from `main.go` after About module extraction.
- [x] Added Windows packaging fallback to download missing Tesseract `eng/fra/iku` language data.
- [x] Switched bundled packaging to treat GStreamer as optional on both Windows and Linux (no hard fail).
- [x] Added resilient whisper model download fallbacks and made missing model non-fatal for bundled packaging.
- [x] Added Linux `zip` dependency in CI build deps to prevent bundled zip step failures.
- [x] Disabled bundled package generation for dev channel builds to stabilize nightly/pre-release pipelines.
- [x] Removed bundled package generation from Linux/Windows dev-packages workflow; VT now publishes the standard package only.
- [x] Fixed main menu tile layout to a stable 3-column grid without expanding the window beyond screen bounds.
- [x] Fixed Forgejo release asset purge logic to reliably delete old assets before upload.
- [x] Fixed Forgejo release asset delete endpoint path to avoid 404 during publish.
- [x] Added a Blu-ray module visibility toggle in Preferences and wired it to main menu filtering.
- [x] Benchmark apply now updates hardware acceleration only and explicitly leaves codec/preset unchanged.
- [x] Bumped app version metadata to v0.1.1-dev29 (main.go, VERSION, FyneApp.toml).

## Version 0.1.1-dev28 (2026-02-21) - Windows First-Run Dependency Bootstrap

### Windows Dependencies
- [x] Added first-run in-app FFmpeg bootstrap prompt when FFmpeg is missing.
- [x] Added app-local FFmpeg install flow to `%LOCALAPPDATA%\VideoTools\bin` (downloads official Windows portable package and extracts `ffmpeg.exe` + `ffprobe.exe`).
- [x] Added app-local FFmpeg discovery in platform detection so installed binaries are reused on later launches.
- [x] Updated dependency checks to treat configured app-local FFmpeg paths as installed.
- [x] Added a Settings > Dependencies FFmpeg install action on Windows using the same app-local bootstrap flow.

### Cross-Platform Dependencies
- [x] Sorted Settings > Dependencies with required dependencies first and stable alphabetical ordering.
- [x] Added Settings > Dependencies FFmpeg install/uninstall actions for Linux via package-manager commands.
- [x] Replaced Convert UI Unicode/emoji labels with ASCII-safe strings to prevent mojibake in Windows terminal/font environments.

### UI
- [x] Removed the Bitcoin address from the About/Support dialog.
- [x] Added adaptive scroll speed for long panels to improve multi-resolution navigation.
- [x] Made Settings tabs scroll independently to keep tab headers visible.
- [x] Promoted master settings for hardware acceleration and module visibility.
- [x] Focused language options on Canadian English/French and Inuktitut.
- [x] Refactored main menu flow into a dedicated module file for easier maintenance.
- [x] Improved aspect ratio handling using display aspect ratio metadata and added a 17:9 target.
- [x] Show the detected source aspect ratio alongside the Source aspect option.
- [x] Added lightweight logging for source/target aspect details and ignored stale auto-crop values when auto-crop is off.
- [x] Added a Custom aspect option for clean ultrawide support with minimal UI clutter.
- [x] Aligned aspect conversion with target resolution to avoid odd output sizes (e.g., 1920x1082).
- [x] Stopped auto-resizing the window on each module switch to prevent misclicks.
- [x] Cleaned mojibake/garbled UI characters in core UI labels.
- [x] Conversion worker panics now surface a failure dialog instead of closing the UI.
- [x] Added a lightweight conversion recovery notice on next launch with persisted state.
- [x] Modularized the About/Support dialog into `about_module.go`.
- [x] Modularized the missing dependencies dialog into `deps_dialog_module.go`.
- [x] Modularized the queue view into `queue_module.go`.
- [x] Fixed dev-packages workflow YAML parsing for bundled deps note.
- [x] Fixed module imports for main menu and queue modules.
- [x] Fixed duplicate package declaration in `mainmenu_module.go`.

### Packaging
- [x] Added bundled Windows/Linux packages with FFmpeg, Tesseract, and GStreamer plus bundled launchers.
- [x] Bundled packages now include the whisper.cpp small model and enforce required dependency payloads.

### Subtitles
- [x] Added embedded subtitle extraction with lossless and text (SRT) modes.
- [x] Added safer subtitle embedding that preserves sync and warns on incompatible outputs.
- [x] Integrated Tesseract OCR for image-based subtitles with SRT/ASS output.
- [x] Normalized OCR output and merged consecutive duplicate cues for cleaner timing.

### Snippet
- [x] Added AV1 encoder fallback when `libsvtav1` is unavailable.

## Version 0.1.1-dev27 (2026-02-13) - Windows Build Artifact Cleanup

### Maintenance
- ✅ **.gitignore updates** - Excluded Windows build artifacts (*.syso) and agent working directory (.opencode/).
- **Forgejo Windows outputs** - Emit `GITHUB_OUTPUT` as UTF-8 (no BOM) to prevent host-runner post-step failures.

## Version 0.1.1-dev26 (2026-01-XX) - Windows Build System & Mirror Infrastructure

### Infrastructure
- **Mirror hosting** - Created lt_mirror repository on git.leaktechnologies.dev for downloads when source sites block bots. Used for GStreamer, DVDStyler, Whisper, FFmpeg.
- **Forgejo CI/CD** - Self-hosted runner setup for Windows, CI workflows for Windows/Linux, artifact versioning, optional EXE signing.

### Windows Build System
- **Installer** - Switched from Scoop to Chocolatey for dependencies, added MSYS2 for builds, dependency checking with early exit, progress bars for downloads, installer verification.
- **Build scripts** - Console popup suppression for CGO, icon embedding, windowsgui flag, Go module caching, fixed Unicode encoding.

### Documentation
- Added Forgejo runner and Windows service setup docs.

## Version 0.1.0-dev24 (2026-01-06) - DVD Menu Templating System

### Features
- âœ… **DVD Menu Templating System**
  - Refactored `author_menu.go` to support multiple, selectable menu templates.
  - Implemented a `MenuTemplate` interface for easy extensibility.
  - Created three initial menu templates:
    - **Simple**: The default, clean menu style.
    - **Dark**: A dark-themed menu for a more cinematic feel.
    - **Poster**: A template that uses a user-provided image as a background.
- âœ… **Menu Customization UI**
  - Added a "Menu Template" dropdown to the authoring settings tab.
  - Added a "Select Background Image" button that appears when the "Poster" template is selected.
  - User's menu template and background image choices are persisted in configuration.

### Maintenance
- âœ… **Git author cleanup**
  - Rewrote commit history to ensure consistent commit attribution.
- âœ… **Installer dependency parity**
  - Ensured pip is installed (Linux/Windows) and skipped Go/pip installs when already present.
- âœ… **Windows installer parse fix**
  - Normalized PowerShell here-strings to prevent parse errors during installation.
- âœ… **Go auto-install on Windows**
  - Removed the Go prompt in `install.sh`; missing Go is now installed automatically.
- âœ… **Windows install workflow split**
  - `install.sh` now delegates to the Windows installer to avoid mixed-shell prompts.
- âœ… **Windows installer entrypoint**
  - Added `install-windows.ps1` and made `install.sh` Windows-safe with a clear handoff message.
- âœ… **Git Bash Windows handoff**
  - `install.sh` now runs the Windows installer in the same terminal via `winpty` when available.
- âœ… **Windows root entrypoints**
  - Added `install.bat` and `install.ps1` to avoid Git Bash popping up from PowerShell.
- âœ… **Windows scripts entrypoints**
  - Added `scripts/install.ps1` and `scripts/install.bat` to keep the Windows workflow inside PowerShell/CMD.
- âœ… **Windows setup launcher alignment**
  - `scripts/_internal/setup-windows.bat` now delegates to `scripts/install.bat` for a single Windows flow.
- Adjusted Forgejo artifact actions to v3 for runner compatibility.
- Added Windows CI icon embedding via windres when available.
- Moved default logs to ~/Videos/VideoTools/logs with user override in Settings.
- Added Linux AppImage packaging in Forgejo builds with embedded VT icon.
- âœ… **Agent workflow rules**
  - Added `AGENTS.md` to enforce staging, commits, and documentation updates.
- Fixed Linux script paths after scripts reorg (build/install/run).
- Updated Forgejo dev packaging to use appVersion-based artifacts and stable/dev release tagging.
- âœ… **Player fullscreen toggle**
  - Added fullscreen toggle to the Player module controls.
- âœ… **Player EOS handling + metadata access**
  - Stop playback cleanly on EOS and expose duration/FPS from GStreamer.
- âœ… **Main menu title cleanup**
  - Header now shows "VideoTools" only; platform suffix moved to the footer version label.
- âœ… **Main menu palette refresh**
  - Restored a diverse, eye-friendly rainbow palette while keeping Convert constant.
- âœ… **Main menu readability**
  - Increased tile label size and adjusted colors for better contrast.
- âœ… **Main menu contrast tuning**
  - Audio, Rip, and Settings colors refined for legibility.
- âœ… **Main menu layout cleanup**
  - Removed scroll container so the main menu scales without scroll bars.
- âœ… **Player silhouette placeholder**
  - Player pane keeps a stable footprint before media loads.
- âœ… **Main menu palette tuning**
  - Adjusted audio/compare/subtitles colors for better separation.
- âœ… **Main menu vibrancy pass**
  - Removed monochrome tiles outside Settings.
- âœ… **Main menu bespoke hues**
  - Assigned unique hue families to each module for maximum legibility.
- âœ… **Locked tile hue preservation**
  - Disabled modules stay colored while appearing subdued.
- âœ… **Locked hue visibility**
  - Reduced stripe opacity and raised label brightness.

## Version 0.1.0-dev25 (2026-01-22) - Settings Preferences Expansion

### Features
- âœ… **Language & Hardware Acceleration in Settings**
  - Added `Language` string to convertConfig (default: "System").
  - Decoupled benchmark: now only sets HardwareAccel; no codec/preset changes or confirmation dialogs.
  - Implemented Settings > Preferences UI with working selectors:
    - Language dropdown (System/en/es/fr/de/ja/zh) persists to convertConfig.Language.
    - Hardware Acceleration dropdown (auto/none/nvenc/qsv/amf/vaapi/videotoolbox) persists to convertConfig.HardwareAccel.
  - Removed placeholder "Coming soon" text; UI is functional and logical.

### Documentation
- âœ… **TODO.md extended** to track remaining Preferences items (output directories, UI theme, auto-updates, reset/import).
- âœ… **Documentation alignment** - Updated README, module overview, and project status to reflect current implementation and TODO/DONE state.
- âœ… **README technical section** - Added preset codec and frame rate targets.
- âœ… **README balance pass** - Updated capabilities, added status/doc links, and clarified DVD frame rate locking.
- âœ… **Build links** - Added Daily (dev) and Stable (public) build locations to README and docs index.
- âœ… **Build link fix** - Corrected Daily (dev) URL.
- âœ… **Broken link audit** - Fixed internal doc links in README and docs, removed stale placeholders.
- âœ… **Build metadata outputs** - Build scripts now emit zip artifacts and `build.json` metadata per channel and OS.
- âœ… **Build docs update** - Documented `VT_BUILD_CHANNEL` and artifact locations in build/install guides.

### UI/UX
- [x] Module palette contrast - Updated module and queue colors to contrast-friendly palette.

### Maintenance
- [x] Replaced Scoop dependency with MSYS2 toolchain across Windows install/build scripts and docs.

### Windows Install
- [x] GCC preflight failures trigger MSYS2 MinGW-w64 reinstall offers; Scoop toolchains are ignored.
- [x] Added Windows GUI preflight to flag VM/basic display adapters before Fyne startup.
- [x] Windows build script pauses for a keypress on success or failure.
- [x] Removed duplicate GUI startup handler causing build failures.
- [x] Aligned Windows script output headers with Linux styling and printed build metadata.
- [x] Windows build script now refreshes PATH and can auto-repair missing MSYS2 GCC via pacman.
- [x] Standardized Windows build tooling on repo-local MSYS2 UCRT64 with a deterministic provisioner.
- [x] Reorganized scripts into platform-specific folders and removed top-level wrappers.

### Packaging
- [x] Added Forgejo Actions workflow for dev Windows/Linux packaging and artifacts.
- [x] Added Forgejo dev release upload (optional, requires token).
- [x] Added optional EXE signing step for Forgejo dev builds.
- [x] Added self-signed dev cert generator and MSIX signing in Forgejo pipeline.
- [x] Aligned Forgejo runner labels to `ubuntu` and `windows` for active runners.

### Docs
- [x] Removed personal names from documentation in favor of user report/dev report labels.

## Version 0.1.0-dev23 (2026-01-04) - UI Cleanup & About Dialog


### UI/UX
- âœ… **Colored select polish** - one-click dropdown, left accent bar, softer blue-grey background, rounded corners, larger text
- âœ… **Panel input styling** - input and panel backgrounds aligned to dropdown tone
- âœ… **Convert panel buttons** - Auto-crop and interlace actions styled to match settings panel
- âœ… **About / Support redesign** - mockup-aligned layout, VT + LT logos, Logs Folder placement, support placeholder

### Stability
- âœ… **Audio module crash fix** - prevent nil entry panic on initial quality selection

## Version 0.1.0-dev22 (2026-01-01) - Bug Fixes & Documentation

### Bug Fixes
- âœ… **Refactored Command Execution (Windows Console Fix Extended to Core Modules)**
  - Extended the refactoring of command execution to `audio_module.go`, `author_module.go`, and `platform.go`.
  - All direct calls to `exec.Command` and `exec.CommandContext` in these modules now use `utils.CreateCommand` and `utils.CreateCommandRaw`.
  - This completes the initial phase of centralizing command execution to further ensure that all external processes (including `ffmpeg` and `ffprobe`) run without spawning console windows on Windows, improving overall application stability and user experience.

- âœ… **Refactored Command Execution (Windows Console Fix Extended)**
  - Systematically replaced direct calls to `exec.Command` and `exec.CommandContext` across `main.go` and `internal/benchmark/benchmark.go` with `utils.CreateCommand` and `utils.CreateCommandRaw`.
  - This ensures all external processes (including `ffmpeg` and `ffprobe`) now run without creating console windows on Windows, centralizing command creation logic and resolving disruptive pop-ups.

- âœ… **Fixed Console Pop-ups on Windows**
  - Created a centralized utility function (`utils.CreateCommand`) that starts external processes without creating a console window on Windows.
  - Refactored the benchmark module and main application logic to use this new utility.
  - This resolves the issue where running benchmarks or other operations would cause disruptive `ffmpeg.exe` console windows to appear.

### Documentation
- âœ… **Addressed Platform Gaps (Windows Guide)**
  - Created a new, comprehensive installation guide for native Windows (`docs/INSTALL_WINDOWS.md`).
  - Refactored the main `INSTALLATION.md` into a platform-agnostic hub that now links to the separate, detailed guides for Windows and Linux/WSL.
  - This provides a clear, user-friendly path for users on all major platforms.

- âœ… **Aligned Documentation with Reality**
  - Audited and tagged all planned features in the documentation with `[PLANNED]`.
  - This provides a more honest representation of the project's capabilities.
  - Removed broken links from the documentation index.

- âœ… **Created Project Status Page**
  - Created `docs/PROJECT_STATUS.md` to provide a single source of truth for project status.
  - Summarizes implemented, planned, and in-progress features.
  - Highlights critical known issues, like the player module bugs.
  - Linked from the main `README.md` to ensure users and developers have a clear, honest overview of the project's state.

This file tracks completed features, fixes, and milestones.

## Version 0.1.0-dev20+ (2025-12-28) - Queue UI Performance & Workflow Improvements

### Bug Fixes
- âœ… **Player Module Investigation**
  - Investigated reported player crash
  - Discovered player is ALREADY fully internal and lightweight
  - Uses FFmpeg directly (no external VLC/MPV/FFplay dependencies)
  - Implementation: FFmpeg pipes raw frames + audio â†’ Oto library for output
  - Frame-accurate seeking and A/V sync built-in
  - Error handling: Falls back to video-only playback if audio fails
  - Player module re-enabled - follows VideoTools' core principles

### Workflow Enhancements
- âœ… **Benchmark Result Caching**
  - Benchmark results now persist across app restarts
  - Opening Benchmark module shows cached results instead of auto-running
  - Clear timestamp display (e.g., "Showing cached results from December 28, 2025 at 2:45 PM")
  - "Run New Benchmark" button available when viewing cached results
  - Auto-runs only when no previous results exist or hardware has changed (GPU detection)
  - Saves to `~/.config/VideoTools/benchmark.json` with last 10 runs in history
  - No more redundant benchmarks every time you open the module

- âœ… **Merge Module Output Path UX Improvement**
  - Split single output path field into separate folder and filename fields
  - "Output Folder" field with "Browse Folder" button for directory selection
  - "Output Filename" field for easy filename editing (e.g., "merged.mkv")
  - No more navigating through long paths to change filenames
  - Cleaner, more intuitive interface following standard file dialog patterns
  - Auto-population sets directory and filename independently

- âœ… **Queue Priority System for Convert Now**
  - "Convert Now" during active conversions adds job to top of queue (after running job)
  - "Add to Queue" continues to add to end as expected
  - Implemented AddNext() method in queue package for priority insertion
  - User feedback message indicates queue position: "Added to top of queue!" vs "Conversion started!"
  - Better workflow when adding files during active batch conversions

- âœ… **Auto-Cleanup for Failed Conversions**
  - Convert jobs now automatically delete incomplete/broken output files on failure
  - Success tracking ensures complete files are never removed
  - Prevents accumulation of partial files from crashed/cancelled conversions
  - Cleaner disk space management and error handling

- âœ… **Queue List Jankiness Reduction**
  - Increased auto-refresh interval from 1000ms to 2000ms for smoother updates
  - Reduced scroll restoration delay from 50ms to 10ms for faster position recovery
  - Fixed race condition in scroll offset saving
  - Eliminated visible jumping during queue view rebuilds

### Performance Optimizations
- âœ… **Queue View Button Responsiveness**
  - Fixed Windows-specific button lag after conversion completion
  - Eliminated redundant UI refreshes in queue button handlers (Pause, Resume, Cancel, Remove, Move Up/Down, etc.)
  - Queue onChange callback now handles all refreshes automatically - removed duplicate manual calls
  - Added stopQueueAutoRefresh() before navigation to prevent conflicting UI updates
  - Result: Instant button response on Windows (was 1-3 second lag)
  - Reported by: user report

- âœ… **Main Menu Performance**
  - Fixed main menu lag when sidebar visible and queue active
  - Implemented 300ms throttling for main menu rebuilds (prevents excessive redraws)
  - Cached jobQueue.List() calls to eliminate multiple expensive copies (was 2-3 copies per refresh)
  - Smart conditional refresh: only rebuild sidebar when history actually changes
  - Result: 3-5x improvement in main menu responsiveness, especially on Windows
  - RAM usage confirmed: 220MB (lean and efficient for video processing app)

- âœ… **Queue Auto-Refresh Optimization**
  - Reduced auto-refresh interval from 500ms to 1000ms (1 second)
  - Reduces UI thread pressure on Windows while maintaining smooth progress updates
  - Combined with 500ms manual throttle in refreshQueueView() for optimal balance

### User Experience Improvements
- âœ… **Benchmark UI Cleanup**
  - Hide benchmark indicator in Convert module when settings are already applied
  - Only show "Benchmark: Not Applied" status when action is needed
  - Removes clutter from UI when using benchmark settings
  - Cleaner interface for active conversions with benchmark recommendations

- âœ… **Queue Position Labeling**
  - Fixed confusing priority display in queue view
  - Changed from internal priority numbers (3, 2, 1) to user-friendly queue positions (1, 2, 3)
  - Now displays "Queue Position: 1" for first job, "Queue Position: 2" for second, etc.
  - Applied to both Pending and Paused jobs
  - Much clearer for users to understand execution order

### Remux Safety System (Fool-Proof Implementation)
- âœ… **Comprehensive Codec Compatibility Validation**
  - Added validateRemuxCompatibility() function with format-specific checks
  - Automatically detects incompatible codec/container combinations
  - Validates before ANY remux operation to prevent silent failures

- âœ… **Container-Specific Validation**
  - MP4: Blocks VP8, VP9, AV1, Theora, Vorbis, Opus (not reliably supported)
  - MKV: Allows almost everything (ultra-flexible)
  - WebM: Enforces VP8/VP9/AV1 video + Vorbis/Opus audio only
  - MOV: Apple-friendly codecs (H.264, H.265, ProRes, MJPEG)

- âœ… **Automatic Fallback to Re-encoding**
  - WMV/ASF sources automatically re-encode (timestamp/codec issues)
  - FLV with legacy codecs (Sorenson/VP6) auto re-encode
  - Incompatible codec/container pairs auto re-encode to safe default (H.264)
  - User never gets broken files - system handles it transparently

- âœ… **Auto-Fixable Format Detection**
  - AVI: Applies -fflags +genpts for timestamp regeneration
  - FLV (H.264): Applies timestamp fixes
  - MPEG-TS/M2TS/MTS: Extended analysis + timestamp fixes
  - VOB (DVD rips): Full timestamp regeneration
  - All apply -avoid_negative_ts make_zero automatically

- âœ… **Enhanced FFmpeg Safety Flags**
  - All remux operations now include:
    - `-fflags +genpts` (regenerate timestamps)
    - `-avoid_negative_ts make_zero` (fix negative timestamps)
    - `-map 0` (preserve all streams)
    - `-map_chapters 0` (preserve chapters)
  - MPEG-TS sources get extended analysis parameters
  - Result: Robust, reliable remuxing with zero risk of corruption

- âœ… **Codec Name Normalization**
  - Added normalizeCodecName() to handle codec name variations
  - Maps h264/avc/avc1/h.264/x264 â†’ h264
  - Maps h265/hevc/h.265/x265 â†’ h265
  - Maps divx/xvid/mpeg-4 â†’ mpeg4
  - Ensures accurate validation regardless of FFprobe output variations

### Technical Improvements
- âœ… **Smart UI Update Strategy**
  - Throttled refreshes prevent cascading rebuilds
  - Conditional updates only when state actually changes
  - Queue list caching eliminates redundant memory allocations
  - Windows-optimized rendering pipeline

- âœ… **Debug Logging**
  - Added comprehensive logging for remux compatibility decisions
  - Clear messages when auto-fixing vs auto re-encoding
  - Helps debugging and user understanding

## Version 0.1.0-dev20+ (2025-12-26) - Author Module & UI Enhancements

### Features
- âœ… **Author Module - Real-time Progress Reporting**
  - Implemented granular progress updates for FFmpeg encoding steps in the Author module.
  - Progress bar now updates smoothly during video processing, providing better feedback.
  - Weighted progress calculation based on video durations for accurate overall progress.

- âœ… **Author Module - "Add to Queue" & Output Title Clear**
  - Added an "Add to Queue" button to the Author module for non-immediate job execution.
  - Refactored authoring workflow to support queuing jobs via a `startNow` parameter.
  - Modified "Clear All" functionality to also clear the DVD Output Title, preventing naming conflicts.

- âœ… **Main Menu - "Disc" Category for Author, Rip, and Blu-Ray**
  - Relocated "Author", "Rip", and "Blu-Ray" buttons to a new "Disc" category on the main menu.
  - Improved logical grouping of disc-related functionalities.

- âœ… **Subtitles Module - Video File Path Population**
  - Fixed an issue where dragging and dropping a video file onto the Subtitles module would not populate the "Video File Path" section.
  - Ensured the video entry widget correctly reflects the dropped video's path.

## Version 0.1.0-dev20+ (2025-12-23) - Player UX & Installer Polish

### Features (2025-12-23 Session)
- âœ… **Player Module UI Improvements**
  - Responsive video player sizing based on screen resolution
  - Screens < 1600px wide: 640x360 (prevents layout breaking)
  - Screens â‰¥ 1600px wide: 1280x720 (larger viewing area)
  - Dynamically adapts to display when player view is built
  - Prevents excessive negative space on lower resolution displays

- âœ… **Main Menu Cleanup**
  - Hidden "Logs" button from main menu (history sidebar replaces it)
  - Logs button only appears when onLogsClick callback is provided
  - Cleaner, less cluttered interface
  - Dynamic header controls based on available functionality

- âœ… **Windows Installer Fix**
  - Fixed DVDStyler download from SourceForge mirrors
  - Added `-MaximumRedirection 10` to handle SourceForge redirects
  - Added browser user agent to prevent rejection
  - Resolves "invalid archive" error on Windows 11
  - Reported by: user report

### Technical Improvements
- âœ… **Responsive Design Pattern**
  - Canvas size detection for adaptive UI sizing
  - Prevents window layout issues on smaller displays
  - Maintains larger preview on high-resolution screens

- âœ… **PowerShell Download Robustness**
  - Proper redirect following for mirror systems
  - User agent spoofing for compatibility
  - Multiple fallback URLs for resilience

## Version 0.1.0-dev20 (2025-12-21) - VT_Player Framework Implementation

### Features (2025-12-21 Session)
- âœ… **VT_Player Module - Complete Framework Implementation**
  - **Frame-Accurate Video Player Interface** (`internal/player/vtplayer.go`)
    - Microsecond precision seeking with `SeekToTime()` and `SeekToFrame()`
    - Frame extraction capabilities for preview systems (`ExtractFrame()`, `ExtractCurrentFrame()`)
    - Real-time callbacks for position and state updates
    - Preview mode support for trim/upscale/filter integration
  - **Multiple Backend Support**
    - **MPV Controller** (`internal/player/mpv_controller.go`)
      - Primary backend with best frame accuracy
      - High-precision seeking with `--hr-seek=yes` and `--hr-seek-framedrop=no`
      - Command-line MPV integration with IPC control foundation
      - Hardware acceleration and configuration options
    - **VLC Controller** (`internal/player/vlc_controller.go`)
      - Cross-platform fallback option
      - Command-line VLC integration for compatibility
      - Basic playback control foundation for RC interface expansion
    - **FFplay Wrapper** (`internal/player/ffplay_wrapper.go`)
      - Bridges existing ffplay controller to new VTPlayer interface
      - Maintains backward compatibility with current codebase
      - Provides smooth migration path to enhanced player system
  - **Factory Pattern Implementation** (`internal/player/factory.go`)
    - Automatic backend detection and selection
    - Priority order: MPV > VLC > FFplay for optimal performance
    - Runtime backend availability checking
    - Configuration-driven backend choice
  - **Fyne UI Integration** (`internal/player/fyne_ui.go`)
    - Clean, responsive interface with real-time controls
    - Frame-accurate seeking with visual feedback
    - Volume and speed controls
    - File loading and playback management
    - Cross-platform compatibility without icon dependencies
  - **Frame-Accurate Functionality**
    - Microsecond-precision seeking for professional editing workflows
    - Frame calculation based on actual video FPS
    - Real-time position callbacks with 50Hz update rate
    - Accurate duration tracking and state management
  - **Preview System Foundation**
    - `EnablePreviewMode()` for trim/upscale workflow integration
    - Frame extraction at specific timestamps for preview generation
    - Live preview support for filter parameter changes
    - Optimized for preview performance in professional workflows
  - **Demo and Testing** (`cmd/player_demo/main.go`)
    - Working demonstration of VT_Player capabilities
    - Backend detection and selection validation
    - Frame-accurate method testing
    - Integration example for other modules

### Technical Implementation Details
- **Cross-Platform Backend Support**: Command-line integration for MPV/VLC with future IPC expansion
- **Frame Accuracy**: Microsecond precision timing with time.Duration throughout
- **Error Handling**: Graceful fallbacks and comprehensive error reporting
- **Resource Management**: Proper process cleanup and context cancellation
- **Interface Design**: Clean separation between UI and playback engine
- **Future Extensibility**: Foundation for enhanced IPC control and additional backends

### Integration Points
- **Trim Module**: Frame-accurate preview of cut points and timeline navigation
- **Upscale Module**: Real-time preview with live parameter updates
- **Filters Module**: Frame-by-frame comparison and live effect preview
- **Convert Module**: Video loading and preview integration

### Documentation
- âœ… Created comprehensive implementation documentation (`docs/VT_PLAYER_IMPLEMENTATION.md`)
- âœ… Documented architecture decisions and backend selection logic
- âœ… Provided integration examples for module developers
- âœ… Outlined future enhancement roadmap

## Version 0.1.0-dev20 (2025-12-18 to 2025-12-20) - Convert Module Cleanup & UX Polish

### Features (2025-12-20 Session)
- âœ… **History Sidebar - In Progress Tab**
  - Added "In Progress" tab to history sidebar
  - Shows running and pending jobs without opening queue
  - Animated striped progress bars per module color
  - Real-time progress updates (0-100%)
  - No delete button on active jobs (only completed/failed)
  - Dynamic status text ("Running..." or "Pending")

- âœ… **Benchmark System Overhaul**
  - **Hardware Detection Module** (`internal/sysinfo/sysinfo.go`)
    - Cross-platform CPU detection (model, cores, clock speed)
    - GPU detection with driver version (NVIDIA via nvidia-smi)
    - RAM detection with human-readable formatting
    - Linux and Windows support
  - **Hardware Info Display**
    - Shown immediately in benchmark progress view (before tests run)
    - Displayed in benchmark results view
    - Saved with each benchmark run for history
  - **Settings Persistence**
    - Hardware acceleration settings saved with benchmarks
    - Settings persist between sessions via config file
    - GPU automatically detected and used
  - **UI Polish**
    - "Run Benchmark" button highlighted (HighImportance) on first run
    - Returns to normal styling after initial benchmark
  - Guides new users to run initial benchmark

- âœ… **AI Upscale Integration (Real-ESRGAN)**
  - Added model presets with anime/general variants
  - Processing presets (Ultra Fast â†’ Maximum Quality) with tile/TTA tuning
  - Upscale factor selection + output adjustment slider
  - Tile size, output frame format, GPU and thread controls
  - ncnn backend pipeline (extract â†’ AI upscale â†’ reassemble)
  - Filters and frame rate conversion applied before AI upscaling

- âœ… **Bitrate Preset Simplification**
  - Reduced from 13 confusing options to 6 clear presets
  - Removed resolution references (no more "1440p" confusion)
  - Codec-agnostic (presets don't change selected codec)
  - Quality-based naming: Low/Medium/Good/High/Very High Quality
  - Focused on common use cases (1.5-8 Mbps range)
  - Presets only set bitrate and switch to CBR mode
  - User codec choice (H.264, VP9, AV1, etc.) preserved

- âœ… **Quality Preset Codec Compatibility**
  - "Lossless" quality option only available for H.265 and AV1
  - Dynamic quality dropdown based on selected codec
  - Automatic fallback to "Near-Lossless" when switching to non-lossless codec
  - Lossless + Target Size bitrate mode now supported for H.265/AV1
  - Prevents invalid codec/quality combinations

- âœ… **App Icon Improvements**
  - Regenerated VT_Icon.ico with transparent background
  - Updated LoadAppIcon() to search PNG first (better Linux support)
  - Searches both current directory and executable directory
  - Added debug logging for icon loading troubleshooting

- âœ… **UI Scaling for 800x600 Windows** (2025-12-20 continuation)
  - Reduced module tile size from 220x110 to 150x65
  - Reduced title text size from 28 to 18
  - Reduced queue tile from 160x60 to 120x40
  - Reduced section padding from 14 to 4 pixels
  - Reduced category labels to 12px
  - Removed extra padding wrapper around tiles
  - Removed scrolling requirement - everything fits without scrolling
  - All UI elements fit within 800x600 default window

- âœ… **Header Layout Improvements** (2025-12-20 continuation)
  - Changed from HBox with spacer to border layout
  - Title on left, all controls grouped compactly on right
  - Shortened button labels for space efficiency
  - "â˜° History" â†’ "â˜°", "Run Benchmark" â†’ "Benchmark", "View Results" â†’ "Results"
  - Eliminates wasted horizontal space

- âœ… **Queue Clear Behavior Fix** (2025-12-20 continuation)
  - "Clear Completed" now always returns to main menu
  - "Clear All" now always returns to main menu
  - Prevents unwanted navigation to convert module after clearing queue
  - Consistent and predictable behavior

- âœ… **Threading Safety Fix** (2025-12-20 continuation)
  - Fixed Fyne threading errors in stats bar component
  - Removed Show()/Hide() calls from Layout() method
  - Layout() can be called from any thread during resize/redraw
  - Show/Hide logic remains only in Refresh() with proper DoFromGoroutine
  - Eliminates threading warnings during UI updates

- âœ… **Preset UX Improvements** (2025-12-20 continuation)
  - Moved "Manual" option to bottom of all preset dropdowns
  - Bitrate preset default: "2.5 Mbps - Medium Quality"
  - Target size preset default: "100MB"
  - Manual input fields hidden by default
  - Manual fields appear only when "Manual" is selected
  - Encourages preset usage while maintaining advanced control
  - Reversed encoding preset order: veryslow first, ultrafast last
  - Better quality options now appear at top of list
  - Applied consistently to both simple and advanced modes

- âœ… **Audio Channel Remixing** (2025-12-20 continuation)
  - Added advanced audio channel options for videos with imbalanced L/R channels
  - New options using FFmpeg pan filter:
    - "Left to Stereo" - Copy left channel to both speakers (music only)
    - "Right to Stereo" - Copy right channel to both speakers (vocals only)
    - "Mix to Stereo" - Downmix both channels together evenly
    - "Swap L/R" - Swap left and right channels
  - Implemented in all 4 command builders (DVD, convert, snippet)
  - Maintains existing options (Source, Mono, Stereo, 5.1)
  - Solves problem of videos with music in one ear and vocals in the other

- âœ… **Author Module Skeleton** (2025-12-20 continuation)
  - Renamed "DVD Author" module to "Author" for broader scope
  - Created tabbed interface structure with 3 tabs:
    - **Chapters Tab** - Scene detection and chapter management
    - **Rip DVD/ISO Tab** - High-quality disc extraction (like FLAC from CD)
    - **Author Disc Tab** - VIDEO_TS/ISO creation for burning
  - Implemented basic Chapters tab UI:
    - File selection with video probing
    - Scene detection sensitivity slider (0.1-0.9 threshold)
    - Placeholder chapter list
    - Add/Export chapter buttons (to be implemented)
  - Added authorChapter struct for storing chapter data
  - Added author module state fields to appState
  - Foundation for complete disc production workflow

- âœ… **Real-ESRGAN Automated Setup** (2025-12-20 continuation)
  - Created automated setup script for Linux (setup-realesrgan-linux.sh)
  - One-command installation: downloads, installs, configures
  - Installs binary to ~/.local/bin/realesrgan-ncnn-vulkan
  - Installs all AI models to ~/.local/share/realesrgan/models/ (45MB)
  - Includes 5 model sets: animevideov3, x4plus, x4plus-anime
  - Sets proper permissions and provides PATH setup instructions
  - Makes AI upscaling fully automated for users
  - No manual downloads or configuration needed

- âœ… **Window Auto-Resize Fix** (2025-12-20 continuation)
  - Fixed window resizing itself when content changes
  - Window now maintains user-set size through all content updates
  - Progress bars and queue updates no longer trigger window resize
  - Preserved window size before/after SetContent() calls
  - User retains full control via manual resize or maximize
  - Improves professional appearance and stability
  - Reported by: user report

### Features (2025-12-18 Session)
- âœ… **History Sidebar Enhancements**
  - Delete button ("Ã—") on each history entry
  - Remove individual entries from history
  - Auto-save and refresh after deletion
  - Clean, unobtrusive button placement

- âœ… **Command Preview Improvements**
  - Show/Hide button state based on preview visibility
  - Disabled when no video source loaded
  - Displays actual file paths instead of placeholders
  - Real-time live updates as settings change
  - Collapsible to save screen space

- âœ… **Format Options Reorganization**
  - Grouped by codec family (H.264 â†’ H.265 â†’ AV1 â†’ VP9 â†’ ProRes â†’ MPEG-2)
  - Added descriptive comments for each codec type
  - Improved dropdown readability and navigation
  - Easier to find and compare similar formats

- âœ… **Bitrate Mode Clarity**
  - Descriptive labels in dropdown:
    - CRF (Constant Rate Factor)
    - CBR (Constant Bitrate)
    - VBR (Variable Bitrate)
    - Target Size (Calculate from file size)
  - Immediate understanding without documentation
  - Preserves internal compatibility with short codes

- âœ… **Root Folder Cleanup**
  - Moved all documentation .md files to docs/ folder
  - Kept only README.md, TODO.md, DONE.md in root
  - Cleaner project structure
  - Better organization for contributors

### Bug Fixes
- âœ… **Critical Convert Module Crash Fixed**
  - Fixed nil pointer dereference when opening Convert module
  - Corrected widget initialization order
  - bitrateContainer now created after bitratePresetSelect initialized
  - Eliminated "invalid memory address" panic on startup

- âœ… **Log Viewer Crash Fixed**
  - Fixed "close of closed channel" panic
  - Duplicate close handlers removed
  - Proper dialog cleanup

- âœ… **Bitrate Control Improvements**
  - CBR: Set bufsize to 2x bitrate for better encoder handling
  - VBR: Increased maxrate cap from 1.5x to 2x target bitrate
  - VBR: Added bufsize at 4x target to enforce caps
  - Prevents runaway bitrates while maintaining quality peaks

### Technical Improvements
- âœ… **Widget Initialization Order**
  - Fixed container creation dependencies
  - All Select widgets initialized before container use
  - Proper nil checking in UI construction

- âœ… **Bidirectional Label Mapping**
  - Display labels map to internal storage codes
  - Config files remain compatible
  - Clean separation of UI and data layers

## Version 0.1.0-dev18 (2025-12-15)

### Features
- âœ… **Thumbnail Module Enhancements**
  - Enhanced metadata display with 3 lines of comprehensive technical data
  - Added 8px padding between thumbnails in contact sheets
  - Increased thumbnail width to 280px for analyzable screenshots (4x8 grid = ~1144x1416)
  - Audio bitrate display alongside audio codec (e.g., "AAC 192kbps")
  - Concise bitrate display (removed "Total:" prefix)
  - Video codec, audio codec, FPS, and overall bitrate shown in metadata
  - Navy blue background (#0B0F1A) for professional appearance

- âœ… **Player Module**
  - New Player button on main menu (Teal #44FFDD)
  - Access to VT_Player for video playback
  - Video loading and preview integration
  - Module handler for CLI support

- âœ… **Filters Module - UI Complete**
  - Color correction controls (brightness, contrast, saturation)
  - Enhancement tools (sharpness, denoise)
  - Transform operations (rotation, flip horizontal/vertical)
  - Creative effects (grayscale)
  - Navigation to Upscale module with video transfer
  - Full state management for filter settings

- âœ… **Upscale Module - Fully Functional**
  - Traditional FFmpeg scaling methods: Lanczos (sharp), Bicubic (smooth), Spline (balanced), Bilinear (fast)
  - Resolution presets: 720p, 1080p, 1440p, 4K, 8K
  - "UPSCALE NOW" button for immediate processing
  - "Add to Queue" button for batch processing
  - Job queue integration with real-time progress tracking
  - AI upscaling detection (Real-ESRGAN) with graceful fallback
  - High quality encoding (libx264, preset slow, CRF 18)
  - Navigation back to Filters module

- âœ… **Snippet System Overhaul - Dual Output Modes**
  - **"Snippet to Default Format" (Checkbox CHECKED - Default)**:
    - Stream copy mode preserves exact source format, codec, bitrate
    - Zero quality loss - bit-perfect copy of source
    - Outputs to source container (.wmv â†’ .wmv, .avi â†’ .avi, etc.)
    - Fast processing (no re-encoding)
    - Duration: Keyframe-level precision (may vary Â±1-2s)
    - Perfect for merge testing without quality changes
  - **"Snippet to Output Format" (Checkbox UNCHECKED)**:
    - Uses configured conversion settings from Convert tab
    - Applies video codec (H.264, H.265, VP9, AV1, etc.)
    - Applies audio codec (AAC, Opus, MP3, FLAC, etc.)
    - Uses encoder preset and CRF quality settings
    - Outputs to selected format (.mp4, .mkv, .webm, etc.)
    - Frame-perfect duration control (exactly configured length)
    - Perfect preview of final conversion output

- âœ… **Configurable Snippet Length**
  - Adjustable snippet length (5-60 seconds, default: 20)
  - Slider control with real-time display
  - Snippets centered on video midpoint
  - Length persists across video loads

- âœ… **Batch Snippet Generation**
  - "Generate All Snippets" button for multiple loaded videos
  - Processes all videos with same configured length
  - Consistent timestamp for uniform naming
  - Efficient queue integration
  - Shows confirmation with count of jobs added

- âœ… **Smart Job Descriptions**
  - Displays snippet length and mode in job queue
  - "10s snippet centred on midpoint (source format)"
  - "20s snippet centred on midpoint (conversion settings)"

### Technical Improvements
- âœ… **Dual-Mode Snippet System Implementation**
  - Default Format mode: Stream copy for bit-perfect source preservation
  - Output Format mode: Full conversion using user's configured settings
  - Automatic container/codec matching based on mode selection
  - Integration with conversion config (video/audio codecs, presets, CRF)
  - Smart extension handling (source format vs. selected output format)
- âœ… **Queue/Status UI polish**
  - Animated striped progress bars per module color with faster motion for visibility
  - Footer refactor: consistent dark status strip + tinted action bar across modules
  - Status bar tap restored to open Job Queue; full-width clickable strip
- âœ… **Snippet progress reporting**
  - Live progress from ffmpeg `-progress` output; 0â€“100% updates in status bar and queue
  - Error/log capture preserved for snippet jobs

- âœ… **Metadata Enhancement System**
  - New `getDetailedVideoInfo()` function using FFprobe
  - Extracts video codec, audio codec, FPS, video bitrate, audio bitrate
  - Multiple ffprobe calls for comprehensive data
  - Graceful fallback to format-level bitrate if stream bitrate unavailable

- âœ… **Module Navigation Pattern**
  - Bidirectional navigation between Filters and Upscale
  - Video file transfer between modules
  - Filter chain transfer capability (foundation for future)

- âœ… **Resolution Parsing System**
  - `parseResolutionPreset()` function for preset strings
  - Maps "1080p (1920x1080)" format to width/height integers
  - Support for custom resolution input (foundation)

- âœ… **Upscale Filter Builder**
  - `buildUpscaleFilter()` constructs FFmpeg scale filters
  - Method-specific scaling: lanczos, bicubic, spline, bilinear
  - Filter chain combination support

### Bug Fixes
- âœ… Fixed incorrect thumbnail count in contact sheets (was generating 34 instead of 40 for 5x8 grid)
- âœ… Fixed frame selection FPS assumption (hardcoded 30fps removed)
- âœ… Fixed module visibility (added thumb module to enabled check)
- âœ… Fixed undefined function call (openFileManager â†’ openFolder)
- âœ… Fixed dynamic total count not updating when changing grid dimensions
- âœ… Added missing `strings` import to thumbnail/generator.go
- âœ… Updated snippet UI labels for clarity (Default Format vs Output Format)

### Documentation
- âœ… Updated ai-speak.md with comprehensive dev18 documentation
- âœ… Created 24-item testing checklist for dev18
- âœ… Documented all implementation details and technical decisions

## Version 0.1.0-dev17 (2025-12-14)

### Features
- âœ… **Thumbnail Module - Complete Implementation**
  - Individual thumbnail generation with customizable count (3-50 thumbnails)
  - Contact sheet generation with metadata headers
  - Customizable grid layouts (2-12 columns, 2-12 rows)
  - Even timestamp distribution across video duration
  - JPEG output with configurable quality (default: 85)
  - Configurable thumbnail width (160-640px for individual, 200px for contact sheets)
  - Saves to `{video_directory}/{video_name}_thumbnails/` for easy access
  - DejaVu Sans Mono font matching app styling
  - App background color (#0B0F1A) for contact sheet padding
  - Dynamic total count display for grid layouts

- âœ… **Thumbnail UI Integration**
  - Video preview window (640x360) in thumbnail module
  - Mode-specific controls (contact sheet: columns/rows, individual: count/width)
  - Dual button system:
    - "GENERATE NOW" - Adds to queue and starts immediately
    - "Add to Queue" - Adds for batch processing
  - "View Results" button with in-app contact sheet viewer (900x700 dialog)
  - "View Queue" button for queue access from thumbnail module
  - Drag-and-drop support for video files (universal across app)
  - Real-time grid total calculation as columns/rows change

- âœ… **Job Queue Integration for Thumbnails**
  - Background thumbnail generation with progress tracking
  - Job queue support with live progress updates
  - Can queue multiple thumbnail jobs from different videos
  - Progress callback integration for thumbnail extraction
  - Proper context cancellation support

- âœ… **Snippet Tool Improvement**
  - Changed from re-encoding to stream copy (`-c copy`)
  - Instant 20-second snippet extraction with zero quality loss
  - No encoding overhead - extracts source streams directly
  - Removed 148 lines of unnecessary encoding logic

### Technical Improvements
- âœ… **Timestamp-based Frame Selection**
  - Fixed frame selection from FPS-dependent (`eq(n,frame_num)`) to timestamp-based (`gte(t,timestamp)`)
  - Ensures correct thumbnail count regardless of video frame rate
  - Works reliably with VFR (Variable Frame Rate) content
  - Uses `setpts=N/TB` for proper timestamp reset in contact sheets

- âœ… **FFmpeg Filter Optimization**
  - Tile filter for grid layouts: `tile=COLUMNSxROWS`
  - Select filter with timestamp-based frame extraction
  - Pad filter with hex color codes for app background matching
  - Drawtext filter with font specification and positioning
  - Scale filter maintaining aspect ratios

- âœ… **Module Architecture**
  - Added thumbnail state fields to appState (thumbFile, thumbCount, thumbWidth, thumbContactSheet, thumbColumns, thumbRows, thumbLastOutputPath)
  - Implemented `showThumbView()` for thumbnail module UI
  - Implemented `buildThumbView()` for split layout (preview 55%, settings 45%)
  - Implemented `executeThumbJob()` for job queue integration
  - Universal drag-and-drop handler for all modules

- âœ… **Error Handling**
  - Disabled timestamp overlay on individual thumbnails to avoid font availability issues
  - Graceful handling of missing output directories
  - Proper error dialogs with context-specific messages
  - Exit status 234 resolution (font-related errors)

### Bug Fixes
- âœ… Fixed incorrect thumbnail count in contact sheets (was generating 34 instead of 40 for 5x8 grid)
- âœ… Fixed frame selection FPS assumption (hardcoded 30fps removed)
- âœ… Fixed module visibility (added thumb module to enabled check)
- âœ… Fixed undefined function call (openFileManager â†’ openFolder)
- âœ… Fixed dynamic total count not updating when changing grid dimensions
- âœ… Fixed font-related crash on systems without DejaVu Sans Mono

## Version 0.1.0-dev16 (2025-12-14)

### Features
- âœ… **Interlacing Detection Module - Complete Implementation**
  - Automatic interlacing analysis using FFmpeg idet filter
  - Field order detection (TFF - Top Field First, BFF - Bottom Field First)
  - Frame-by-frame analysis with classifications:
    - Progressive frames
    - Top Field First interlaced frames
    - Bottom Field First interlaced frames
    - Undetermined frames
  - Interlaced percentage calculation
  - Status determination: Progressive (<5%), Interlaced (>95%), Mixed Content (5-95%)
  - Confidence levels: High (<5% undetermined), Medium (5-15%), Low (>15%)
  - Quick analyze mode (500 frames) for fast detection
  - Full video analysis option for comprehensive results

- âœ… **Deinterlacing Recommendations**
  - Automatic deinterlacing recommendations based on analysis
  - Suggested filter selection (yadif for compatibility)
  - Human-readable recommendations
  - SuggestDeinterlace boolean flag for programmatic use

- âœ… **Preview Generation**
  - Deinterlace preview at specific timestamps
  - Side-by-side comparison (original vs deinterlaced)
  - Uses yadif filter for preview generation
  - Frame extraction with proper scaling

### Technical Improvements
- âœ… **Detector Implementation**
  - Created `/internal/interlace/detector.go` package
  - NewDetector() constructor accepting ffmpeg and ffprobe paths
  - Analyze() method with configurable sample frame count
  - QuickAnalyze() convenience method for 500-frame sampling
  - Regex-based parsing of idet filter output
  - Multi-frame detection statistics extraction

- âœ… **Detection Result Structure**
  - Comprehensive DetectionResult type with all metrics
  - String() method for formatted output
  - Percentage calculations for interlaced content
  - Field order determination logic
  - Confidence calculation based on undetermined ratio

- âœ… **FFmpeg Integration**
  - idet filter integration for interlacing detection
  - Proper stderr pipe handling for filter statistics
  - Context-aware command execution with cancellation support
  - Null output format for analysis-only operations

### Documentation
- âœ… Added interlacing detection to module list
- âœ… Documented detection algorithms and thresholds
- âœ… Explained field order types and their implications

## Version 0.1.0-dev13 (In Progress - 2025-12-03)

### Features
- âœ… **Automatic Black Bar Detection and Cropping**
  - Detects and removes black bars to reduce file size (15-30% typical reduction)
  - One-click "Detect Crop" button analyzes video using FFmpeg cropdetect
  - Samples 10 seconds from middle of video for stable detection
  - Shows estimated file size reduction percentage before applying
  - User confirmation dialog displays before/after dimensions
  - Manual crop override capability (width, height, X/Y offsets)
  - Applied before scaling for optimal results
  - Works in both direct convert and queue job execution
  - Proper handling for videos without black bars
  - 30-second timeout protection for detection process

- âœ… **Frame Rate Conversion UI with Size Estimates**
  - Comprehensive frame rate options: Source, 23.976, 24, 25, 29.97, 30, 50, 59.94, 60
  - Intelligent file size reduction estimates (40-50% for 60â†’30 fps)
  - Real-time hints showing "Converting X â†’ Y fps: ~Z% smaller file"
  - Warning for upscaling attempts with judder notice
  - Automatic calculation based on source and target frame rates
  - Dynamic updates when video or frame rate changes
  - Supports both film (24 fps) and broadcast standards (25/29.97/30)
  - Uses FFmpeg fps filter for frame rate conversion

- âœ… **Encoder Preset Descriptions with Speed/Quality Trade-offs**
  - Detailed information for all 9 preset options
  - Speed comparisons relative to "slow" and "medium" baselines
  - File size impact percentages for each preset
  - Visual icons indicating speed categories (âš¡â©âš–ï¸ðŸŽ¯ðŸŒ)
  - Recommends "slow" as best quality/size ratio
  - Dynamic hint updates when preset changes
  - Helps users make informed encoding time decisions
  - Ranges from ultrafast (~10x faster, ~30% larger) to veryslow (~5x slower, ~15-20% smaller)

- âœ… **Compare Module**
  - Side-by-side video comparison interface
  - Load two videos and compare detailed metadata
  - Displays format, resolution, codecs, bitrates, frame rate, pixel format
  - Shows color space, color range, GOP size, field order
  - Indicates presence of chapters and metadata
  - Accessible via GUI button (pink color) or CLI: `videotools compare <file1> <file2>`
  - Added formatBitrate() helper function for consistent bitrate display

- âœ… **Target File Size Encoding Mode**
  - New "Target Size" bitrate mode in convert module
  - Specify desired output file size (e.g., "25MB", "100MB", "8MB")
  - Automatically calculates required video bitrate based on:
    - Target file size
    - Video duration
    - Audio bitrate
    - Container overhead (3% reserved)
  - Implemented ParseFileSize() to parse size strings (KB, MB, GB)
  - Implemented CalculateBitrateForTargetSize() for bitrate calculation
  - Works in both GUI convert view and job queue execution
  - Minimum bitrate sanity check (100 kbps) to prevent invalid outputs

### Technical Improvements
- âœ… Added compare command to CLI help text
- âœ… Consistent "Target Size" naming throughout UI and code
- âœ… Added compareFile1 and compareFile2 to appState for video comparison
- âœ… Module button grid updated with compare button (pink/magenta color)

## Version 0.1.0-dev12 (2025-12-02)

### Features
- âœ… **Automatic hardware encoder detection and selection**
  - Prioritizes NVIDIA NVENC > Intel QSV > VA-API > OpenH264
  - Falls back to software encoders (libx264/libx265) if no hardware acceleration available
  - Automatically uses best available encoder without user configuration
  - Significant performance improvement on systems with GPU encoding support

- âœ… **iPhone/mobile device compatibility settings**
  - H.264 profile selection (baseline, main, high)
  - H.264 level selection (3.0, 3.1, 4.0, 4.1, 5.0, 5.1)
  - Defaults to main profile, level 4.0 for maximum compatibility
  - Ensures videos play on iPhone 4 and newer devices

- âœ… **Advanced deinterlacing with dual methods**
  - Added bwdif (Bob Weaver) deinterlacing - higher quality than yadif
  - Kept yadif for faster processing when speed is priority
  - Auto-detect interlaced content based on field_order metadata
  - Deinterlace modes: Auto (detect and apply), Force, Off
  - Defaults to bwdif for best quality

- âœ… **Audio normalization for compatibility**
  - Force stereo (2 channels) output
  - Force 48kHz sample rate
  - Ensures consistent playback across all devices
  - Optional toggle for maximum compatibility mode

- âœ… **10-bit encoding for better compression**
  - Changed default pixel format from yuv420p to yuv420p10le
  - Provides 10-20% file size reduction at same visual quality
  - Better handling of color gradients and banding
  - Automatic for all H.264/H.265 conversions

- âœ… **Browser desync fix**
  - Added `-fflags +genpts` to regenerate timestamps
  - Added `-r` flag to enforce constant frame rate (CFR)
  - Fixes "desync after multiple plays" issue in Chromium browsers (Chrome, Edge, Vivaldi)
  - Eliminates gradual audio drift when scrubbing/seeking

- âœ… **Extended resolution support**
  - Added 8K (4320p) resolution option
  - Supports: 720p, 1080p, 1440p, 4K (2160p), 8K (4320p)
  - Prepared for future VR and ultra-high-resolution content

- âœ… **Black bar cropping infrastructure**
  - Added AutoCrop configuration option
  - Cropdetect filter support for future auto-detection
  - Foundation for 15-30% file size reduction in dev13

### Technical Improvements
- âœ… All new settings propagate to both direct convert and queue processing
- âœ… Backward compatible with legacy InverseTelecine setting
- âœ… Comprehensive logging for all encoding decisions
- âœ… Settings persist across video loads

### Bug Fixes
- âœ… Fixed VFR (Variable Frame Rate) handling that caused desync
- âœ… Prevented timestamp drift in long videos
- âœ… Improved browser playback compatibility

## Version 0.1.0-dev11 (2025-11-30)

### Features
- âœ… Added persistent conversion stats bar visible on all screens
  - Real-time progress updates for running jobs
  - Displays pending/completed/failed job counts
  - Clickable to open queue view
  - Shows job title and progress percentage
- âœ… Added multi-video navigation with Prev/Next buttons
  - Load multiple videos for batch queue setup
  - Switch between loaded videos to review settings before queuing
  - Shows "Video X of Y" counter
- âœ… Added installation script with animated loading spinner
  - Braille character animations
  - Shows current task during build and install
  - Interactive path selection (system-wide or user-local)
  - Added error dialogs with "Copy Error" button
  - One-click error message copying for debugging
  - Applied to all major error scenarios
  - Better user experience when reporting issues

### Improvements
- âœ… Align direct convert and queue behavior
  - Show active direct convert inline in queue with live progress
  - Preserve queue scroll position during updates
  - Back button from queue returns to originating module
  - Queue badge includes active direct conversions
  - Allow adding to queue while a convert is running
- âœ… DVD-compliant outputs
  - Enforce MPEG-2 video + AC-3 audio, yuv420p
  - Apply NTSC/PAL targets with correct fps/resolution
  - Disable cover art for DVD targets to avoid mux errors
  - Unified settings for direct and queued jobs
- âœ… Updated queue tile to show active/total jobs instead of completed/total
  - Shows pending + running jobs out of total
  - More intuitive status at a glance
- âœ… Fixed critical deadlock in queue callback system
  - Callbacks now run in goroutines to prevent blocking
  - Prevents app freezing when adding jobs to queue
- âœ… Improved batch file handling with detailed error reporting
  - Shows which specific files failed to analyze
  - Continues processing valid files when some fail
  - Clear summary messages
- âœ… Fixed queue status display
  - Always shows progress percentage (even at 0%)
  - Clearer indication when job is running vs. pending
- âœ… Fixed queue deserialization for formatOption struct
  - Handles JSON map conversion properly
  - Prevents panic when reloading saved queue on startup

### Bug Fixes
- âœ… Fixed crash when dragging multiple files
  - Better error handling in batch processing
  - Graceful degradation for problematic files
- âœ… Fixed deadlock when queue callbacks tried to read stats
- âœ… Fixed formatOption deserialization from saved queue

## Version 0.1.0-dev7 (2025-11-23)

### Features
- âœ… Changed default aspect ratio from 16:9 to Source across all instances
  - Updated initial state default
  - Updated empty fallback default
  - Updated reset button behavior
  - Updated clear video behavior
  - Updated hint label text

### Documentation
- âœ… Created comprehensive MODULES.md with all planned modules
- âœ… Created PERSISTENT_VIDEO_CONTEXT.md design document
- âœ… Created VIDEO_PLAYER.md documenting custom player implementation
- âœ… Reorganized docs into module-specific folders
- âœ… Created detailed Convert module documentation
- âœ… Created detailed Inspect module documentation
- âœ… Created detailed Rip module documentation
- âœ… Created docs/README.md navigation hub
- âœ… Created TODO.md and DONE.md tracking files

## Version 0.1.0-dev6 and Earlier

### Core Application
- âœ… Fyne-based GUI framework
- âœ… Multi-module architecture with tile-based main menu
- âœ… Application icon and branding
- âœ… Debug logging system (VIDEOTOOLS_DEBUG environment variable)
- âœ… Cross-module state management
- âœ… Window initialization and sizing

### Convert Module (Partial Implementation)
- âœ… Basic video conversion functionality
- âœ… Format selection (MP4, MKV, WebM, MOV, AVI)
- âœ… Codec selection (H.264, H.265, VP9)
- âœ… Quality presets (CRF-based encoding)
- âœ… Output aspect ratio selection
  - Source, 16:9, 4:3, 1:1, 9:16, 21:9
- âœ… Aspect ratio handling methods
  - Auto, Letterbox, Pillarbox, Blur Fill
- âœ… Deinterlacing options
  - Inverse telecine with default smoothing
- âœ… Mode toggle (Simple/Advanced)
- âœ… Output filename customization
- âœ… Default output naming ("-convert" suffix)
- âœ… Status indicator during conversion
- âœ… Cancelable conversion process
- âœ… FFmpeg command construction
- âœ… Process management and execution

### Video Loading & Metadata
- âœ… File selection dialog
- âœ… FFprobe integration for metadata parsing
- âœ… Video source structure with comprehensive metadata
  - Path, format, resolution, duration
  - Video/audio codecs
  - Bitrate, framerate, pixel format
  - Field order detection
- âœ… Preview frame generation (24 frames)
- âœ… Temporary directory management for previews

### Media Player
- âœ… Embedded video playback using FFmpeg
- âœ… Audio playback with SDL2
- âœ… Frame-accurate rendering
- âœ… Playback controls (play/pause)
- âœ… Volume control
- âœ… Seek functionality with progress bar
- âœ… Player window sizing based on video aspect ratio
- âœ… Frame pump system for smooth playback
- âœ… Audio/video synchronization
- âœ… Stable seeking and embedded video rendering

### Metadata Display
- âœ… Metadata panel showing key video information
- âœ… Resolution display
- âœ… Duration formatting
- âœ… Codec information
- âœ… Aspect ratio display
- âœ… Field order indication

### Inspect Module (Basic)
- âœ… Video metadata viewing
- âœ… Technical details display
- âœ… Comprehensive information in Convert module metadata panel
- âœ… Cover art preview capability

### UI Components
- âœ… Main menu with 8 module tiles
  - Convert, Merge, Trim, Filters, Upscale, Audio, Thumb, Inspect
- âœ… Module color coding for visual identification
- âœ… Clear video control in metadata panel
- âœ… Reset button for Convert settings
- âœ… Status label for operation feedback
- âœ… Progress indication during operations

### Git & Version Control
- âœ… Git repository initialization
- âœ… .gitignore configuration
- âœ… Version tagging system (v0.1.0-dev1 through dev7)
- âœ… Commit message formatting
- âœ… Binary exclusion from repository
- âœ… Build cache exclusion

### Build System
- âœ… Go modules setup
- âœ… Fyne dependencies integration
- âœ… FFmpeg/FFprobe external tool integration
- âœ… SDL2 integration for audio
- âœ… OpenGL bindings (go-gl) for video rendering
- âœ… Cross-platform file path handling

### Asset Management
- âœ… Application icon (VT_Icon.svg)
- âœ… Icon export to PNG format
- âœ… Icon embedding in application

### Logging & Debugging
- âœ… Category-based logging (SYS, UI, MODULE, etc.)
- âœ… Timestamp formatting
- âœ… Debug output toggle via environment variable
- âœ… Log file output (videotools.log)

### Error Handling
- âœ… FFmpeg execution error capture
- âœ… File selection cancellation handling
- âœ… Video parsing error messages
- âœ… Process cancellation cleanup

### Utility Functions
- âœ… Duration formatting (seconds to HH:MM:SS)
- âœ… Aspect ratio parsing and calculation
- âœ… File path manipulation
- âœ… Temporary directory creation and cleanup

## Technical Achievements

### Architecture
- âœ… Clean separation between UI and business logic
- âœ… Shared state management across modules
- âœ… Modular design allowing easy addition of new modules
- âœ… Event-driven UI updates

### FFmpeg Integration
- âœ… Dynamic FFmpeg command building
- âœ… Filter chain construction for complex operations
- âœ… Stream mapping for video/audio handling
- âœ… Process execution with proper cleanup
- âœ… Progress parsing from FFmpeg output (basic)

### Media Playback
- âœ… Custom media player implementation
- âœ… Frame extraction and display pipeline
- âœ… Audio decoding and playback
- âœ… Synchronization between audio and video
- âœ… Embedded playback within application window
- âœ… Seek functionality with progress bar
- âœ… Player window sizing based on video aspect ratio
- âœ… Frame pump system for smooth playback
- âœ… Audio/video synchronization
- âœ… Checkpoint system for playback position

### UI/UX
- âœ… Responsive layout adapting to content
- âœ… Intuitive module selection
- âœ… Clear visual feedback during operations
- âœ… Logical grouping of related controls
- âœ… Helpful hint labels for user guidance

## Milestones

- **2025-11-23** - v0.1.0-dev7 released with Source aspect ratio default
- **2025-11-22** - Documentation reorganization and expansion
- **2025-11-21** - Last successful binary build (GCC compatibility)
- **Earlier** - v0.1.0-dev1 through dev6 with progressive feature additions
  - dev6: Aspect ratio controls and cancelable converts
  - dev5: Icon and basic UI improvements
  - dev4: Build cache management
  - dev3: Media player checkpoint
  - Earlier: Initial implementation and architecture

## Development Progress

### Lines of Code (Estimated)
- **main.go**: ~2,500 lines (comprehensive Convert module, UI, player)
- **Documentation**: ~1,500 lines across multiple files
- **Total**: ~4,000+ lines

### Modules Status
- **Convert**: 60% complete (core functionality working, advanced features pending)
- **Inspect**: 20% complete (basic metadata display, needs dedicated module)
- **Merge**: 0% (planned)
- **Trim**: 0% (planned)
- **Filters**: 0% (planned)
- **Upscale**: 0% (planned)
- **Audio**: 0% (planned)
- **Thumb**: 0% (planned)
- **Rip**: 0% (planned)

### Documentation Status
- **Module Documentation**: 30% complete
  - âœ… Convert: Complete
  - âœ… Inspect: Complete
  - âœ… Rip: Complete
  - â³ Others: Pending
- **Design Documents**: 50% complete
  - âœ… Persistent Video Context
  - âœ… Module Overview
  - â³ Architecture
  - â³ FFmpeg Integration
- **User Guides**: 0% complete

## Bug Fixes & Improvements

### Recent Fixes
- âœ… Fixed aspect ratio default from 16:9 to Source (dev7)
- âœ… Ranked benchmark results by score and added cancel confirmation
- âœ… Added estimated audio bitrate fallback when metadata is missing
- âœ… Made target file size input unit-selectable with numeric-only entry
- âœ… Prevented snippet runaway bitrates when using Match Source Format
- âœ… History sidebar refreshes when jobs complete (snippet entries now appear)
- âœ… Benchmark errors now show non-blocking notifications instead of OK popups
- âœ… Fixed stats bar updates to run on the UI thread to avoid Fyne warnings
- âœ… Defaulted Target Aspect Ratio back to Source unless user explicitly sets it
- âœ… Synced Target Aspect Ratio between Simple and Advanced menus
- âœ… Hide manual CRF input when Lossless quality is selected
- âœ… Upscale now recomputes target dimensions from the preset to ensure 2X/4X apply
- âœ… Added unit selector for manual video bitrate entry
- âœ… Reset now restores full default convert settings even with no config file
- âœ… Reset now forces resolution and frame rate back to Source
- âœ… Fixed reset handler scope for convert tabs
- âœ… Restored 25%/33%/50%/75% target size reduction presets
- âœ… Default bitrate preset set to 2.5 Mbps and added 2.0 Mbps option
- âœ… Default encoder preset set to slow
- âœ… Bitrate mode now strictly hides unrelated controls (CRF only in CRF mode)
- âœ… Removed CRF visibility toggle from quality updates to prevent CBR/VBR bleed-through
- âœ… Added CRF preset dropdown with Manual option
- âœ… Added 0.5/1.0 Mbps bitrate presets and simplified preset names
- âœ… Default bitrate preset normalized to 2.5 Mbps to avoid "select one"
- âœ… Linked simple and advanced bitrate presets so they stay in sync
- âœ… Hide quality presets when bitrate mode is not CRF
- âœ… Snippet UI now shows Convert Snippet + batch + options with context-sensitive controls
- âœ… Reduced module video pane minimum sizes to allow GNOME window snapping
- âœ… Added cache/temp directory setting with SSD recommendation and override
- âœ… Snippet defaults now use conversion settings (not Match Source)
- âœ… Added frame interpolation presets to Filters and wired filter chain to Upscale
- âœ… Stabilized video seeking and embedded rendering
- âœ… Improved player window positioning
- âœ… Fixed clear video functionality
- âœ… Resolved build caching issues
- âœ… Removed binary from git repository

### Performance Improvements
- âœ… Optimized preview frame generation
- âœ… Efficient FFmpeg process management
- âœ… Proper cleanup of temporary files
- âœ… Responsive UI during long operations

## Acknowledgments

### Technologies Used
- **Fyne** - Cross-platform GUI framework
- **FFmpeg/FFprobe** - Video processing and analysis
- **SDL2** - Audio playback
- **OpenGL (go-gl)** - Video rendering
- **Go** - Primary programming language

### Community Resources
- FFmpeg documentation and community
- Fyne framework documentation
- Go community and standard library

---

*Last Updated: 2025-12-21*

