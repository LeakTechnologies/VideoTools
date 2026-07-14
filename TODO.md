# VideoTools TODO

This file tracks upcoming features, improvements, and known issues.

## Dev48 Scope (closing)

- [x] **Theme system** — `internal/theme/` package with VT_Navy palette, PillButton, PillIconButton, text primitives. See `DONE.md` for full dev48 changelog.
- [x] **Transport controls migration** — Player speedBtn/subtitleBtn to PillButton; play/volume/fullscreen/PiP etc. to PillIconButton.
- [x] **Full module button migration** — All compare/audio/rip/filters/upscale/subtitles/trim/thumbnail/queueview/settings/benchmarkview/main.go migrated to `MakePillButton`/`MakePillIconButton`. `MakePillButton` renamed from `NewPillButton`.
- [x] **VTSlider / VTProgressBar** — `widget.Slider` and `widget.ProgressBar` replaced sitewide with styled `ui.Slider`/`ui.MakeSlider`.
- [x] **Queue + Benchmark header/footer alignment** — Both modules now use `TintedBar`, `NewTitleLabel`, `accentColor`, shared statsBar footer.
- [x] **STATUS_STACK_OVERFLOW recovery** — VEH in `safe_bridge.c` catches `STATUS_STACK_OVERFLOW`; `_resetstkoflw()` guard page restore; 4 MB PE stack via `CGO_LDFLAGS_ALLOW`.
- [x] **Dual before/after player sync** — `InlineVideoPlayer.SetPeer()` mirrors Play/Pause/Seek from primary to preview in Filters and Upscale modules.
- [x] **Audio nil-widget crash** — Guard against `Player.Widget()` returning nil.
- [x] **Window recentering removed** — `CenterOnScreen()` removed from `maximizeWindow`.
- [x] **i18n script persistence** — Inuktitut syllabics/Latin preference survives app restarts.
- [x] **Windows SignPath signing** — `SIGNPATH_API_TOKEN` + `SIGNPATH_ORGANIZATION_ID` both set in Forgejo secrets.
- [x] **Startup crash diagnostics** — `VT_STARTUP_DEBUG` env var, `logging.Sync()` pre-crash flush.
- [x] **CI fixes** — cache guard, ci-build.ps1 encoding, Windows FFmpeg shared cache.
- [x] **Roadmap visual polish** — deprecated status, cycle filter, testing checklist, drag-to-scroll modals, colour dots standardisation.
- [x] **Button stragglers** — All migrated (about, compare, settings tabs, command_editor, audio, burn, file_manager, mainmenu, settings, main.go). Remaining exceptions: `convert_player_native.go` + `main.go` transport icons (dynamic play↔pause — PillIconButton lacks SetIcon), `utils.MakeIconButton` (import cycle with ui→benchmark→utils).

## Dev50-51 Scope (current — preparing to ship)

### Player Overlay & Cleanup (dev51)

- [x] **P0: Error/loading/buffering overlay indicators wired** — four widgets now render centred over video
- [x] **P2: Stub method-set divergence fixed** — 9 missing methods added to `inline_player_stub.go`
- [x] **P2: Dead fields/callbacks removed** — `OnFrameRate`, `OnChapterSelect`, `OnHover`, etc.
- [x] **P2: Cosmetic fullscreen/PiP buttons removed** — booleans and buttons removed
- [x] **P2: CC button wired to engine** — `SelectSubtitleTrack(0)`/`DisableSubtitles()`
- [x] **Orphaned `internal/media/gpu/` deleted** — 8 Go files, zero imports
- [x] **P1: view.go component split** — 1442-line monolith → 5 focused files
- [x] **P1: UDF thread safety** — mutex-guarded partitionStart, progress callbacks
- [x] **Legacy alias vars removed** — 10 per-module vars cleaned from `native_media.go`

### Remaining high-priority items

- [ ] **Extract media-import/probe layer out of main.go** — `probeVideo` (~100 lines), `videoSource` type (43 refs in main.go), `isVideoFile`, `findVideoFiles`, `loadMultipleVideos`, `batchAddToQueue`, and the per-module `handleDrop` branches are all import concerns living in root `main.go`. Slice: (1) move `videoSource` + `probeVideo` + `isVideoFile` into `internal/mediaprobe` (or `internal/media/probe`), decoupling the type first; (2) move the drop/import orchestration behind a module. The AVI-import bug had to be fixed by editing main.go — symptom of this bloat. Phase-3 refactor candidate (opencode-owned), needs a dedicated slice, not mixed with feature/bugfix work.

- [x] **Windows import failure (NoInheritHandles)** — dev49's `NoInheritHandles: true` disabled std-pipe inheritance, so ffprobe returned no output and all Windows imports failed; removed. File-in-use stays fixed via Go's handle-list + Job Object.

- [ ] **DVD menu player validation** — A1–A12 spec fixes landed, validated against libdvdread (offsetof harness + ifoOpen parse, `docs/AUTHOR_MENU_AUDIT.md`); still need VLC/hardware playback of a real authored disc. Then close the audit.
- [ ] **VTS_ATRT full attribute record** — libdvdread wants ≥356 B/VTS (menu + title video/audio/subpicture attrs); we emit 266 B (title only). Non-fatal cross-check table.
- [ ] **vts_last_sector VOB-inclusive** — 0x0C must be the last sector of the whole VTS (IFO+VOBs+BUP); author currently leaves it IFO-only. Thread the disc-layout value in.
- [ ] **VTSM domain (audit A13)** — minimal in-title Root menu redirecting to VMGM; deferred design gap, not a bug.

- [ ] **Player interface extraction** — Formal Go `Player` interface from `InlineVideoPlayer` for mock-based unit tests.
- [ ] **renderDualPlayerPreview stub** — `native_media.go` has a TODO stub, returns silently without actually rendering.
- [ ] **Burn multi-drive batch** — Queue multiple ISOs across available burners. See `docs/BURN_MODULE_DESIGN.md` §Phase 2.
- [ ] **IMAPI2 COM replacement** — Replace `isoburn.exe` for proper progress/control. See `docs/BURN_MODULE_DESIGN.md` §Phase 3.
- [ ] **Main Menu refactor** — Extract `showMainMenu()` from root into `internal/app/modules/mainmenu/`.
- [x] **CI green + three-static-binaries** — all GitHub pipelines fixed and verified (dev52); Linux CI speedup moot (~1 min on cache hit).
- [ ] **UDF 2.50/2.60 + BDMV** — Extended reader for Blu-ray ISO parsing.
- [ ] **Sparse + large-file UDF Writer** — Files >2 GB (multi-extent); sparse sector allocation.

### Windows DLL Pipeline Fix (BUG-012 + BUG-013)

- [x] **GitHub release.yml rewritten** — source-built FFmpeg 8.1 + x264 + x265 (matching Forgejo CI), MSYS2 ucrt64 toolchain, objdump transitive-dep scan, ffmpeg.exe/ffprobe.exe bundled, all DLLs including liblzma-5.dll
- [x] **GitHub windows-msix.yml rewritten** — same pipeline pattern, MSIX layout includes DLL/ with full dep scan
- [x] **Forgejo dev-packages.yml rewritten** — replaced BtbN download with source-built shared FFmpeg, matching other two pipelines
- [x] **ExpectedFFmpegDLLs() uses glob patterns** — `avcodec-*.dll` instead of `avcodec-61.dll`; prevents breakage on ABI bumps
- [x] **BUG-013 closed** — BtbN `latest` moving tag eliminated from all CI pipelines; shared DLLs built from same FFmpeg 8.1 source
- [x] **AGENTS.md settled decision updated** — clarified "never use BtbN" covers both static and shared builds

### Phase 0 — Critical Stability (fix first — hangs/crashes/dead-code)

- [x] **P0-1: Wire DegradeToSoftware() into decode paths** — `errors.go` / `playback.go` / `hwdecode.go`: Added `vt_clear_hw_decode` C helper (clears `hw_device_ctx`, resets `get_format` callback, re-enables threading). `DegradeToSoftware()` now calls it + `avcodec_flush_buffers` so buffered HW frames are discarded and the codec won't re-init HW on next decode cycle. In `videoDecodeLoop`, first `videoDecodeDead` event (HW SEH) now calls `RecordHWFailure()` + `DegradeToSoftware()` + clears flag + continues with SW path instead of returning.
- [x] **P0-2: Fix NextFrame hang after videoDecodeDead** — `playback.go`: All three fatal return paths in `videoDecodeLoop` (SafeSendPacket SEH, SafeReceiveFrame SEH, and post-DegradeToSoftware already-degraded path) now send `decodeEOFPTS` sentinel into `frameQueue` before returning, so `NextFrame` unblocks and returns `io.EOF` instead of hanging forever.
- [x] **P0-3: Fix backward frame stepping** — `playback.go:335-338`: `Step()` rejects `frames <= 0` with error. All `StepFrame(-1)` callers silently fail. Fix: add true backward step — seek back 2 keyframes, decode forward to target frame.
- [x] **P0-4: Replace lastError with error ring buffer** — `errors.go:17-48`, `engine.go:205`: single `*PlaybackError` overwritten, `GetLastError()`/`ClearError()` never called. Fix: 16-entry ring buffer with timestamp+code+message, `SetError()` wired into all error paths (SEH catch in GrabFrame, videoDecodeLoop, retrieveHWFrame + DegradeToSoftware), exposed via `Engine.GetErrorHistory()`.
- [x] **P0-5: Add OpenAuto() with Open→OpenDVD fallback** — `engine.go` / `inline_player.go`: `Engine.OpenAuto(path)` tries `Open()`, falls back to `OpenDVD(path, 0)` on failure. `InlineVideoPlayer.Load()` now uses `OpenAuto` instead of `Open` directly — ISOs and VIDEO_TS directories load automatically in any module that calls `Load()`.

### Phase 1 — Player Completeness (missing basic features)

- [x] **P1-1: Network/URL streaming** — `Engine.OpenURL(url, opts)` with AVDictionary options (60s timeout, reconnect). `InlineVideoPlayer.LoadURL()`. HTTP/HTTPS/HLS/DASH/RTSP/RTMP supported.
- [x] **P1-2: Resume/watch-later** — InlineVideoPlayer auto-saves position every 5s during playback, restores on load, marks completed on EOF. Shared ResumeState wired to both player singletons.
- [x] **P1-3: Audio delay adjustment** — `Engine.audioDelayBits` (atomic float64). `WaitForPTS(pts + avDelay)` in NextFrame audio path. Settings → Player "A/V Offset (ms)" entry; persists in `PrefsConfig.AVOffset`. `setPlayerAVOffset` applies mid-session.
- [x] **P1-4: Speed + pitch correction** — `AudioFilterGraph.Process()` implemented with `vt_atempo_process` C helper; lazy-init filter graph in `AudioPlayer.SetSpeed()`; `Read()` routes through atempo when active; leftover path skips re-processing.
- [x] **P1-5: A-B loop** — Engine SetLoopPoints(a,b) + SetABLoopEnabled; NextFrame auto-seeks loopA→loopB. InlineVideoPlayer wiring.
- [x] **P1-6: SeekAccuracy Settings UI** — Dropdown in Settings → Player (Fast/Keyframe, Fastest/Frame, Precise/Slow). Persists in `PrefsConfig.SeekAccuracy`. `loadViaOpen` uses `media.DefaultSeekAccuracy()` instead of hardcoded keyframe. `setPlayerSeekAccuracy()` applies mid-session.
- [x] **P1-7: Bilinear scaling** — FFmpeg sws_scale confirmed using SWS_BICUBIC at engine.go:1445 and framepool.go:36. Clarified the scaleNearest docstring — the canvas blit is nearest-neighbour but the quality-determining swscale pipeline is bicubic.
- [x] **P1-8: Frame timing diagnostics overlay** — Per-frame PTS/delta/gap overlay in top-right corner; InlineVideoPlayer.SetFrameTimingOverlayVisible toggle.
- [x] **P1-9: Settings UI for player tuning** — HW decode toggle moved from Hardware card into Player card. Seek accuracy dropdown added (P1-6). Thread count locked to 1 (stability) and buffer size (`preDecodeFrames`) are constants — not user-configurable. Scale mode, audio buffer latency, and max drift threshold deferred.
- [x] **P1-10: Growing/in-progress file support** — InlineVideoPlayer poll-based growing-file watcher. On EOF with growing-file mode, polls file size every 2s; re-opens + seeks + resumes on growth.
- [x] **P1-11: Clock drift correction** — MasterClock.SetTime monotonic ratchet allows backward reset when jump > 1s and no PTS anchor in last 500ms (underrun recovery).

### Collapsible Player Panel (Filters + Upscale)

- [x] **Collapsible player panel in all five player modules** — `BuildCollapsibleHeader(t.ConvertSectionPlayer, …)` consistent across Convert, Filters, Upscale, Inspect, and Trim. Inspect: fixed GridWithColumns → HSplit; Trim: leftSide Border → VSplit so timeline stays visible when player collapses.

### Playlist / Sequential Playback

- [x] **Playlist / sequential play** — `Enqueue(path)` / `ClearPlaylist()` / `PlaylistLen()` on `InlineVideoPlayer`. `playbackLoop` auto-advances on clean EOF; direct `Load`/`LoadDVD`/`LoadURL` resets the queue.

### HDR Tone-Mapping

- [x] **HDR tone-mapping** — `internal/media/hdr.go`: `isFrameHDR` detects PQ/HLG via `color_trc`. `renderSWFrame` applies `applyHDRTonemap` before sws_scale. Filter graph: `zscale(t=linear,npl=1000)→format(gbrpf32le)→tonemap(hable,desat=0.5)→zscale(t=bt709,m=bt709)→format(yuv420p)`. `hdrTonemapUnsupported` suppresses retry when zscale unavailable. Wired into both `GrabFrame` and `videoDecodeLoop` (SW + HW→SW paths).

### Per-Codec HW Deny-List

- [x] **Per-codec HW decode deny-list** — `hwCodecDenyList` + `SetHWCodecDenyList(s)`; `PrefsConfig.HWCodecDenyList`; Settings → Player text entry; loaded at startup.

### Error Resilience

- [x] **Error resilience flags** — `setVideoCodecErrorFlags()`: explicit `error_concealment = FF_EC_GUESS_MVS | FF_EC_DEBLOCK` before `avcodec_open2` on both video codec init paths.

### Mid-Playback Track Switching

- [x] **Mid-playback audio track switching** — `SelectAudioTrack` fixed: close player first, reinit codec (thread_count=1), seek to current PTS, resume if playing.
- [x] **Mid-playback subtitle track switching** — `SelectSubtitleTrack` fixed: flush queue, reinit codec, clear overlay. `subtitleCodecMu` guards all subtitle codec access.

### HW Decode + Error Concealment

- [x] **HW decode default-on** — `hwDecodeEnabled = true` in `hwdecode.go`. SEH coverage confirmed complete. `DegradeToSoftware()` wired.
- [x] **Error concealment (last-good-frame)** — `lastGoodFrame atomic.Pointer[image.RGBA]` + `decodeErrored atomic.Bool`. `NextFrame` returns frozen frame exactly once on decode-error EOF, then `io.EOF`.

### ASS Subtitle Fixes

- [x] **`formatASSTime` centisecs bug** — Fixed: `(int(d.Milliseconds()) % 1000) / 10`.
- [x] **`escapeASSText` closing-brace over-escape** — Fixed: removed `}` → `\}` replacement; `}` is not special in ASS.

### DLL Startup Validation + CGo Consolidation

- [x] **`ValidateFFmpegDLLs()` smoke test** — runs `ffprobe.exe -version` at startup to verify DLLs actually load.
- [x] **`--dllcheck` CLI flag** — standalone DLL diagnostics tool; prints DLL dir, files, PATH, expected DLLs, smoke test.
- [x] **Non-blocking Fyne error dialog** — shown at startup when DLLs are missing or fail validation, with actionable guidance.
- [x] **CGo directive consolidation** — all 15 duplicate `#cgo windows CFLAGS/LDFLAGS` merged into `internal/media/cgo_preamble.go`.
- [x] **`docs/DLL_BOOTSTRAP.md`** — pipeline docs, common issues, troubleshooting, developer setup.

### VT ISO Engine (separate project — Media Engine opens ISOs as files)

- [x] **UDF reader robustness** — ShortAd allocation descriptor parsing; partition offset applied to all LBN reads; `extractFile`/`ReadFileData` use `InformationLength` from ICB. Multi-extent directory concatenation.
- [x] **Thread safety & progress** — Mutex-guarded Reader; extraction progress callbacks; temp file tracking for crash-safe cleanup.
- [ ] **UDF 2.50/2.60 + BDMV** — Extended reader for Blu-ray ISO parsing.
- [ ] **Sparse + large-file UDF Writer** — Files >2 GB (multi-extent); sparse sector allocation.

---

## Dev49 Scope (closed)

### Rip Module Fixes & Enhancements
- [x] **Menu VOB bleed** — `CollectVOBSets` excludes `VTS_XX_0.VOB` (menu VOB) from content title sets. Fixes menu frames glitching into output video and shifted chapter timestamps.
- [x] **Chapter embedding diagnostics** — Verbose logging of chapter count, timestamps, and embed decision to rip log.
- [x] **Menu preservation option** — New `IncludeMenus` config + "Preserve menus" checkbox. Menu VOBs exported as separate files: `{base}_Menu_{VTS_Name}.{ext}`.
- [x] **Main/extra title naming** — Main feature (longest title) uses main output path; extras get `_Extra_Title_NN` suffix. Star indicator in title selection UI.
- [x] **Layout alignment to Convert style** — buildRipBox sections, HSplit 0.65, collapsible log, Open in Player to footer.
- [x] **Convert collapsible metadata panel** — ▼/▶ toggle in metadata header; leftColumn VSplit 0.5↔0.97.
- [x] **Convert collapsible settings panel** — ◀/▶ toggle in top bar; mainSplit 0.65↔0.97.
- [x] **Design doc** — `docs/RIP_MODULE_REDESIGN.md` documents the redesign.
- [x] **Log session rotation** — `rotateLog()` keeps last 2 sessions; `=== VideoTools session started` marker written on each boot.
- [x] **Settings log management** — Clear Log File + Open Log Folder buttons in Settings → Preferences.
- [x] **Queue blocking dialog removed** — `convertNow()` now calls `s.showQueue()` directly; no more modal dialog after queuing a job.
- [x] **Queue auto-refresh goroutine self-exit fixed** — `startQueueAutoRefresh` + `startQueueElapsedTicker` changed `return` to `continue` so goroutines stay alive when not on queue view; progress bar now updates correctly.
- [x] **Dropdown active item text colour** — `_fyne/widget/menu_item.go` `refreshText()` uses `ColorNameForegroundOnPrimary` (near-black) when item is active on VT_Green focus background; was using light `ColorNameForeground`.
- [x] **`NoInheritHandles` on Windows subprocess creation** — `internal/utils/exec_windows.go` `SysProcAttr` sets `NoInheritHandles: true` on both `CreateCommand`/`CreateCommandRaw`. Fixes "file in use" error on Windows when trying to delete/move source video after conversion; player engine's `avformat_open_input` handles were inherited by FFmpeg child processes.
- [x] **`Queue.Stop()` cancels running job** — `internal/queue/queue.go` `Stop()` now calls `cancelRunningLocked()` before clearing `q.running`. Fixes zombie FFmpeg processes on clean VT shutdown.
- [x] **Windows Job Object crash-safe cleanup** — `internal/utils/jobobject_windows.go`; global Job Object with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` created at startup; `utils.StartCmd()` assigns each encode process to it; Windows kills all members on VT exit including crashes. Fixes zombie FFmpeg on 4K crash.
- [x] **Linux `Pdeathsig: SIGKILL`** — `internal/utils/exec_linux.go`; kernel-level child process cleanup on VT exit; no cooperative shutdown required.
- [x] **`utils.StartCmd()` replaces `cmd.Start()` at all encode sites** — `main.go`, `rip/executor.go`, `audio/executor.go`, `thumbnail/generator.go`, `interlace/detector.go`.
- [x] **BuildCollapsibleHeader component** — `internal/ui/collapsible.go`; `tappableBox` widget + `BuildCollapsibleHeader`; labeled module-colored tappable bars replace clumsy pill buttons for Metadata and Settings panels. Rip log toggle relabeled "▼ LOG"/"▶ LOG".
- [x] **Disc info moved to Source section** — `discInfoLabel` relocated from Format box to Source section; disc type/region/size now visible directly under source path.
- [x] **Single browse button** — ISO... and Folder... buttons replaced with single `...` button opening a 900×640 file dialog.
- [x] **Format validation** — `loadDisc` rejects non-ISO/VIDEO_TS paths with a user-visible error in `discInfoLabel`.

### Player Default Aspect Ratio (idle SMPTE bars)
- [x] **Settings → Preferences** — New "Idle Aspect Ratio" dropdown (4:3, 16:9, 5:3, 21:9, 9:16) persists as `PlayerDefaultAspect` in `PrefsConfig`.
- [x] **VideoPlayer field** — `idleAspectRatio` with `SetIdleAspectRatio`/`IdleAspectRatio`; `draw()` uses it instead of hardcoded `4.0/3.0`.
- [x] **InlineVideoPlayer forwarding** — `SetIdleAspectRatio` method; all player singletons updated on startup and pref change.

### C Disc Debug Utility
- [x] **C-level disc probing** — `internal/media/disc_debug.{c,h,go}`: `DiscDebugHexDump` for raw IFO hex dumps, `DiscDebugListDir` for directory listing via FindFirstFile/opendir, `DiscDebugDirStat` for file count/size.
- [x] **Stub support** — No-op stubs for `!native_media` builds.

### Inuktitut Transliteration — Auto-Fill i18n Script Variants
- [x] **translit package** — `internal/i18n/translit/` with 270-entry mapping tables, greedy longest-prefix roman→syllabics, compound sequence handling for syllabics→roman, format verb escaping, script detection helpers.
- [x] **i18n integration** — `translitFill()` auto-fills empty Inuktitut fields from the other script variant; manual overrides take precedence.

### VT Media Engine Refactoring (HIGH — monolith breakup)

- [x] **engine.go subsystem split** — 3245→1117 lines: errors.go, hwdecode.go, framepool.go, subtitle_engine.go, buffer.go, playback.go extracted
- [x] **Frame pacing fix** — Replaced SetTime(pts) with WaitForPTS(pts) in no-audio path for proper PTS-driven frame timing; removed WaitVsync from playbackLoop (0-16ms DwmFlush jitter was causing ±16ms interval variation); propagated source frame rate to VideoPlayer widget on load
- [x] **Seek corruption fix** — Accurate fallback in `Engine.Seek()` now includes `AVSEEK_FLAG_BACKWARD` so the format context lands at a keyframe before the target PTS. Previously used `AVSEEK_FLAG_ACCURATE` alone, landing mid-GOP, and `avcodec_flush_buffers` destroyed decoder reference state, producing garbled frames for 1-3s after seek.
- [x] **Player singleton consolidation** — All 10 per-module singletons consolidated into 2 shared instances: `GetPrimaryPlayer()` and `GetPreviewPlayer()`. Per-module getters (`GetConvertPlayer`, `GetTrimPlayer`, `GetInspectPlayer`, etc.) now forward to these.
- [x] **Verbose seek logging** — Human-readable seek flags, accurate fallback confirmation, clock reset with audio offset, frame queue drain count, seekGen change detection in videoDecodeLoop, first frame after seek diagnostics.
- [x] **Engine-level bwdif deinterlace** — `internal/media/deinterlace.go` with libavfilter filter graph; applied in `videoDecodeLoop` when `AV_FRAME_FLAG_INTERLACED` set; Settings toggle in Player section (default on); `toRGBA(src *C.AVFrame)` extended signature
- [x] **Thread safety formalisation** — Lock hierarchy docs, lockdep assertions, eliminate reverse-order paths like DegradeToSoftware

*Remaining open items carried forward to Dev50 — see Dev50 Scope above.*

## Dev42 Scope (closed)

Player stabilisation complete. D3D11VA decode, audio sync, GStreamer removal, and SMPTE idle state all ship. Audio module i18n and layout partially done. Thumbnail module gets 3-way output mode and image inspector.

See DONE.md for full dev42 change log.

## Dev43 Scope (closed)

Player thread-safety audit shipped. Pixel format crash, Close/demuxer race, and seekLoop goroutine leak all fixed. See DONE.md.

## Dev44 Scope (closed)

### Convert Module Improvements — Phase 1 (HIGH)

See `docs/CONVERT_MODULE_IMPROVEMENTS.md` for full plan.

- [x] **Audio Sample Rate dropdown** — `audioSampleRateSelect` wired in buildConvertView
- [x] **Normalize Audio checkbox** — `normalizeAudioCheck` + LUFS/TruePeak sliders wired
- [x] **Deinterlace Mode dropdown** — `deinterlaceModeSelect` + `deinterlaceMethodSelect` wired
- [x] **H.264 Profile/Level controls** — `h264ProfileSelect` / `h264LevelSelect` wired; shown when H.264 codec is active

### Convert Module i18n (HIGH — Issue #5)

- [x] **~42 hardcoded strings** i18n'd: checkboxes, buttons, dialog messages, back button
- [x] New keys added to `internal/i18n/strings.go`, `en_ca.go`, `fr_ca.go`, `iu.go`, `iu_latin.go`

### Convert Module Improvements — Phase 2 (HIGH)

- [x] **Consolidate presets** — Move inline format definitions to `presets.go`
- [x] **x264/x265 tuning** — Film/Animation/Grain/Stillimage/Fastdecode

### Audio Module Improvements

See `docs/AUDIO_MODULE_IMPROVEMENTS.md` for full plan.

#### Phase 1: Layout Consistency (HIGH) — partially done in dev42
- [x] **Replace custom HSplit** with `container.NewVSplit` — done in dev42
- [x] **Consistent module box styling** — Added `buildAudioBox()` helper, Convert-style boxes
- [x] **Proper header bar** with module title, stats integration — `TintedBar` header + stats bar wired

#### Phase 2: Native Player Integration (HIGH) - done dev44
- [x] **InlineVideoPlayer** — Add player singleton like Convert
- [x] **Video preview pane** — Same layout pattern as Convert
- [x] **SMPTE bars idle state** — "DROP VIDEO TO LOAD"

#### Phase 3: Track Selection (MEDIUM) - done dev46
- [x] **Enhanced track list** — Codec colors, language flags, duration display
- [x] **Output naming preview** — Show filename before extraction
- [x] **Track reordering** — Up/down buttons enabled; boundary conditions (first/last track) handled

### Upscale Module Improvements (dev44) - done
- [x] **One-click presets** — Hobbyist SD→HD, Semi-Pro 1080p→4K, Anime, Restoration, Social Media workflows
- [x] **UI clarity** — Preset dropdown with description labels, clear AI+RIFE workflow
- [x] **Detection reliability** — VerifyTool() checks PATH + app-local bin + smoke test
- [x] **Optimization guide** — See `docs/UPSCALE_OPTIMIZATION.md` for hobbyist/semi-pro workflows
- [x] **Hardware acceleration** — Sync upscale HW accel from master setting
- [x] **Filters module HW accel** — Add hardware acceleration dropdown

### Burn Module — Logic (HIGH) - done dev45
- [x] **Windows** — `isoburn.exe` (built-in Windows 7+ tool) with eject via IOCTL_STORAGE_EJECT_MEDIA
- [x] **Linux** — `growisofs` (dvd+rw-tools) with SG_IO progress parsing + SHA-256 verify
- [x] **Drive info** — `getDriveInfo()` shows disc capacity (DVD5/9, BD25/50)
- [ ] **Multi-drive batch burning** — See `docs/BURN_MODULE_DESIGN.md` §Phase 3 (future)
- [ ] **IMAPI2 COM** — Replace `isoburn.exe` for proper progress/control (future, complex)

### Module Pipeline (`&&` feature)

- [x] `pipelineActive` state machine on `appState` (off / waiting-step1 / waiting-step2)
- [x] `&&` button in main menu header, visually reflects state
- [x] Module tile dimming for invalid Step 2 targets
- [x] `PipelineAfter` + `PipelineDeleteOnSuccess` fields on `queue.Job`
- [x] Queue runner: chain execution + intermediate file cleanup
- [x] `PipelineKeepIntermediate` field in PrefsConfig
- [x] "Keep intermediate files" toggle in Settings → Preferences

### Quick Access Dropdown
- [x] **Recent file tracking** — Persisted to disk; audio, inspect, player modules now track recent files
- [x] **Module callbacks** — `OnOpenMore` opens file dialog per module; `OnOpenFolder` respects audio output dir

### File Manager
- [ ] **Multi-tab support** — Open multiple folders in tabs

### Logging — Audit (done dev45)
- [x] **Remove unused categories** — `CatEnhance`, `CatRip` removed from `internal/logging/logging.go`
- [x] **Add CatQueue** — New category for queue operations
- [x] **Wire logging fixes** — `enhancement_module.go`, `onnx_model.go` now use `CatModule`; `rip_module.go` now uses `CatDisc`
- [x] **Fix recentfiles.go** — Changed `CatSystem` misuse to `CatUI` via `cat()` helper

### FFmpeg DLL Bootstrap Fix (done dev45)
- [x] **Bundle DLLs in release** — FFmpeg shared DLLs built from source, bundled in `DLL/` subfolder (not root)
- [x] **Remove BtbN download** — `ffmpeg_bootstrap.go` no longer downloads from BtbN (eliminates `liblzma-5.dll` errors)
- [x] **Build script** — `scripts/windows/build-ffmpeg-shared.ps1` builds FFmpeg shared DLLs with all deps statically linked
- [x] **CI update** — `ci-build.ps1` bundles DLLs in `DLL/` subfolder within release ZIP

### Main Menu Refactor (dev46)
- [ ] **Move main menu UI** — Extract main menu code from `main.go` to `internal/app/modules/mainmenu/`
- [x] **Wire Quick Access Dropdown** — `FilesDropdownData` wired, `OnFileClick`/`OnOpenFolder`/`OnOpenMore` callbacks functional
- [ ] **Refactor showMainMenu()** — Move from `mainmenu_module.go` to `internal/app/modules/mainmenu/`, keep `showModule()` routing in `main.go`
- [ ] **Estimated work** — 4-6 hours (extract `showMainMenu()` from `mainmenu_module.go`, test)

### Windows CI Fix (dev46) - DONE
- [x] **Build FFmpeg shared DLLs** — Added CI step to build shared DLLs from source
- [x] **Bundle in release** — `DLL/` subfolder now included in Windows ZIP
- [x] **Fix Red CI** — Windows job now builds and packages correctly

### Carry-forward Issues
- [ ] **Issue #5** — Convert UI layout consistency and label clarity pass
- [x] **Issue #21** — Native disc authoring — PTS validation added: ValidateNAVPTMs + SynthesizeAndPatchPTMs synthesize timestamps when FFmpeg writes zero PTMs; hardware player testing needed
- [ ] **Linux CI** — Pre-built container image to reduce apt install time

### PAL→NTSC Conversion Pipeline — See `docs/PAL_NTSC_CONVERSION.md`

Full-disc extraction with region conversion and IFO regeneration is now implemented:

- [x] **IFO interlace detection** — `TitleInfo.Interlaced` from FilmMode byte (`internal/dvd/ifo/extract.go`)
- [x] **Convert-during-rip checkbox** — "Convert PAL → NTSC during rip" in Rip module enrichment options; injects `yadif=mode=1,scale=720:480:flags=lanczos,fps=30000/1001` + `atempo=0.9600` for H.264 formats only
- [x] **Stage 1 — Full-disc extraction mode** — Rip module iterates all `VTS_nn` sets and the `VIDEO_TS.VOB` menu; CSS decryption; `CollectMenuVOB()` + `executeFullDiscRip()`
- [x] **Stage 2 — Per-stream NTSC conversion including menus** — All VOBs re-encoded as MPEG-2 (DVD compliance) with region conversion filter chain; AC-3 audio with atempo; menu VOBs included
- [x] **Stage 3 — IFO/BUP regeneration from converted streams** — `RegenerateIFOs()` reads original IFO structure, creates new VTS_MAT/VMG_MAT with NTSC attributes, generates PGC/TMAPT/PTT_SRPT, and writes complete IFO/BUP files

---

## Dev40 Scope (closed)

### DVD Menu System (see docs/DVD_MENU_SYSTEM_DESIGN.md)
- [x] **M1/M2** — Encode menu background as MPEG-2 still video via ffmpeg; mux video+SPU into proper DVD VOB
- [x] **M3** — Populate PCI button rectangles (BTN_NS, button coords) in NAV_PCK
- [x] **M4** — Set `VMGM_VOBS_Sector` from ISO disc layout pass
- [x] **M5** — Patch menu PGC CellPlayback sectors from VIDEO_TS.VOB disc location (ISO + folder mode)
- [x] **M6** — Wire ExtrasMpg/ExtrasButtons into VIDEO_TS.VOB and menuPGCs
- [x] **M7** — Implement JumpVMGM_PGCN command; fix chapters/extras button commands

### CI
- [x] **Confirm Windows CI** — Build passes after submodule sync
- [x] **Confirm Linux CI** — Build passes after submodule sync
- [x] **FFmpeg DLL fix** — Use local FFmpeg first, fall back to download
- [x] **filters_module.go build fix** — Removed invalid `*videoSource` type assertion (Go 1.26 CI failure)
- [x] **From-source FFmpeg** — Build FFmpeg/x264/x265 from source; fix x265.pc Libs, Windows multiple-definition, disk space (dev39, CI green runs 1098/1099/1100)

### Burn Module (NEW)
- [x] **Create design document** — See docs/BURN_MODULE_DESIGN.md
- [x] **Add module entry** — Wire in main.go showBurnView()
- [x] **Implement UI** — Source selection, drive detection, burn options
- [x] **Queue integration** — Wire JobTypeBurn to executeBurnJob()
- [ ] **Implement burn logic** — Use IMAPI2 (Windows) or SG_IO (Linux) for direct API calls
  - [ ] Windows: Implement IMAPI2 COM interface in burn_windows.go
  - [ ] Linux: Implement SG_IO ioctl in burn_linux.go
  - [ ] Add getDriveInfo() to show disc capacity (DVD5/9, BD25/50)
  - [x] Add verify option (post-burn read-back)
- [ ] **Multi-drive batch burning** — See docs/BURN_MODULE_DESIGN.md §Phase 3

### File Manager (NEW)
- [x] **Create design document** — See docs/FILE_MANAGER_DESIGN.md
- [x] **Basic navigation** — List files, folder navigation, breadcrumb trail
- [ ] **Multi-tab support** — Open multiple folders in tabs
- [x] **Context menu** — Right-click to open files in modules
  - [x] "Open in Convert" for video files (red pill)
  - [x] "Open in Audio" for audio files (orange pill)
  - [x] "Open in Subtitles" for subtitle files (yellow pill)
  - [x] "Open in Inspect" for any file (green pill)
  - [x] "Open in Author" for DVD files (teal pill)

### Quick Access Dropdown (NEW)
- [x] **Design document** — See docs/QUICK_ACCESS_DROPDOWN.md
- [x] **Files dropdown** — Added to main menu header
- [x] **Open Files callback** — OnOpenMore callback wired
- [x] **Open Output Folder callback** — OnOpenFolder callback wired
- [x] **Recent Files** — Lists recent files with module context
- [ ] **Recent file tracking** — Need to persist recent files list
- [ ] **Module callbacks** — Wire OnOpenMore/OnOpenFolder per module

### Logging Improvements (Dev40)
- [x] **Add CatConvert** — Logging category for convert module
- [x] **Add CatTrim** — Logging category for trim module
- [x] **Add CatMerge** — Logging category for merge module
- [x] **Error-level logging** — Elevate config load and probe failures to Error level with context
- [x] **Preview frame capture** — Add error logging for ffmpeg frame capture failures

### VR360 Video Processing (FUTURE)
- [ ] **Create design document** — See docs/VR360_VIDEO_PROCESSING.md
- [ ] **Viewport extraction** — Convert 360° to flat video with fixed perspective
  - [ ] Detect 360° video format
  - [ ] Add v360 filter to FFmpeg build
  - [ ] Build UI with yaw/pitch/FOV sliders
  - [ ] Live preview with view cone overlay
  - [ ] Export to flat MP4
- [ ] **Stabilization** — Reduce shakiness in handheld 360° footage
- [ ] **Format conversion** — Equirect ↔ Cubemap, fisheye de-warp

### Module Pipeline (`&&` feature) — SHIPPED (dev45)

### UI Improvements
- [x] **Auto-grey incompatible codecs** — See docs/AUTO_GREY_CODECS.md
  - [x] When format selected, filter codec/audio options to compatible only
  - [x] Grey out incompatible options in dropdown
  - [x] Auto-select compatible codec when format changes

### Snippet Module
- [x] **Move drawer to bottom** — Collapsible drawer above stats bar
- [x] **Add "from current position" option** — Use playback position instead of midpoint

### Trim Module (REWORK)
- [x] **Add draggable timeline widget** — See docs/TRIM_MODULE_DESIGN.md
  - [x] Green handle for in-point
  - [x] Red handle for out-point
  - [x] Visual timeline bar with selected region
- [ ] **Multi-segment support** — Add/remove trim segments
  - [ ] Output mode: split to clips vs keep + chapters

### Filter Integration (REWORK)
- [x] **Create design document** — See docs/FILTER_INTEGRATION_DESIGN.md
- [x] **Add filters to Upscale module** — Integrate filter controls in upscale UI
- [x] **Refactor upscale pipeline** — Apply filters BEFORE upscale in encode chain
- [x] **Keep Filters module standalone** — For non-upscale filter jobs

### Author Module
- [x] **Interactive Preview tab** — Full DVD menu preview with video playback
- [x] **Module extraction** — Author module extracted to internal/app/modules/author/
- [x] **IFO audio track table** — VTS_MAT audio attributes populated from actual track codec/channels/language
- [ ] Wire subtitle track authoring through FFmpeg mapping pipeline
- [ ] Wire multi-audio track AC3 encoding
- [ ] **Multi-disc authoring** — See docs/MULTI_DISC_AUTHORING.md
  - Volume management UI (2-9 discs per set)
  - Per-volume clip assignment with drag-and-drop
  - Capacity indicators per volume
  - Auto-balance clips across volumes
  - Batch compile all volumes
  - Per-volume ISO output with volume labels

### Rip Module (document gaps, fix when prioritised)
- [ ] Handle `.m2ts` files in `collectVOBSets` (currently only `.vob`)
- [ ] Implement title set selection UI (hardcoded TODO)
- [ ] **Clone Disc to ISO** — See docs/CLONE_DISC_DESIGN.md
  - Add clone button to Rip module
  - Implement sector-by-sector disc copy
  - Queue integration

### Author Module
- [x] **Interactive Preview tab** — Full DVD menu preview with video playback
- [x] **Module extraction** — Author module extracted to internal/app/modules/author/
- [x] Wire subtitle track authoring through FFmpeg mapping pipeline
- [ ] Wire multi-audio track AC3 encoding

### GPU Rendering Pipeline (NEW)
- [x] **Renderer interface** (`internal/media/gpu/renderer.go`) - Abstract GPU renderer with Texture interface
- [x] **OpenGL implementation** (`internal/media/gpu/opengl.go`) - OpenGL 4.6+ renderer scaffold
- [x] **Direct3D 11 implementation** (`internal/media/gpu/d3d11.go`) - D3D11 renderer scaffold for NVIDIA/AMD
- [x] **Texture utilities** (`internal/media/gpu/texture.go`) - Texture pooling, format conversion, scaling
- [x] **Shader definitions** (`internal/media/gpu/shaders/`) - Vertex, fragment, and YUV→RGB shaders
- [x] **Keyboard shortcuts** (`internal/media/gpu/shortcuts.go`) - Full shortcut handler (Space, arrows, F, M, 0-9, etc.)
- [x] **Seekbar with thumbnails** (`internal/media/gpu/seekbar.go`) - ThumbnailCache, preview on hover, ThumbnailGenerator interface
- [x] **Volume control** (`internal/media/gpu/seekbar.go`) - VolumeControl widget with mute toggle
- [x] **FFmpeg filter pipeline** (`internal/media/filters/pipeline.go`) - Deinterlace, scale, color correction, denoise, sharpen filters with presets
- [x] **VideoPlayer overlay controls** (`internal/media/view.go`) - Integrated player controls with play/pause, seek, volume, hover-to-reveal
- [x] **Fyne fork for GPU texture optimization** (`lt_mirror/fyne`) - Fork adds TexSubImage2D and efficient texture reuse
- [x] **Texture upload pipeline** - Using TexSubImage2D for efficient GPU texture updates (replaces full recreation)
- [x] **Thumbnail extraction** - Async keyframe extraction during file load

Note: Full direct OpenGL/D3D11 integration requires deeper Fyne modifications. Current implementation achieves
~60fps via efficient CPU-to-GPU texture uploads using TexSubImage2D with texture reuse.

### Playback Enhancements
- [x] **Loading indicators** - SetLoading/IsLoading on Engine, loading spinner on VideoPlayer
- [x] **Playback speed control** - Speed button on VideoPlayer, SetSpeed/GetSpeed wired to Engine
- [x] **Chapter parsing** - Chapter struct, parseChapters(), GetChapters() on Engine
- [x] **Chapter markers on seekbar** - Visual chapter markers in VideoPlayer (vtGreen tick marks)
- [x] **Chapter navigation** - Prev/next chapter buttons with callbacks
- [x] **Thumbnail extraction** - ThumbnailExtractor in thumbnail.go
- [x] **Smooth scrubbing** - SmoothScrubbing with FrameCache, pre-decode ahead
- [x] **Resume playback** - ResumeState with JSON config storage
- [x] **Picture-in-Picture** - PiPController with 4 corner positions
- [x] **Audio pitch correction** - TempoController with atempo filter (0.25x-2.0x)
- [x] **Buffering & error recovery** - BufferMode, adaptive sizing, PlaybackError types
- [x] **Video filters** - FilterPipeline wired into Engine, presets supported

### Localization (dev34 carry-forward)
- [x] **Subtitles i18n** — 9 strings added and wired in subtitles_module.go.
- [x] **Audio/Filters/Inspect i18n wired** — Module views use t.* strings.
- [x] **Trim module compatibility** — OnAddToQueue callback and TrimClip struct.
- [x] **Dialog title i18n** — 15+ new keys wired into main.go Convert/Merge/Trim/Snippet modules.

### UI Fixes
- [x] **Back button consistency** — Module name uppercase.
- [x] **Auto-check dropdown fix** — Language switching works.
- [x] **Thumbnail contact sheet** — Header height + filename truncation.
- [x] **Inspect preview placeholder** — No more stuck "Loading preview" state.
- [x] **Preview frame capture** — Captured before interlace analysis.

### Trim Module
- [x] **Trim job submission** — submitTrimJob creates queue.Job properly.

### CI & Release
- [x] **CI green check** — Linux and Windows builds passing (build 512/513).
- [x] **Push to origin** — All fixes pushed.

### Module Extraction (issue #22)
- [x] **`settings_module.go`** — Tab builders extracted to `internal/app/modules/settings/tabs.go`. Callbacks implemented via adapter pattern. Reduced settings_module.go from 2316 to ~1700 lines.
- [x] **`queue_module.go`** — Already uses `internal/ui/queueview.go`. Thin wrappers remain in root.
- [x] **`subtitles_module.go`** — Extracted to `internal/app/modules/subtitles/`. Package structure, types, adapter, and view code moved.
- [x] **`upscale_module.go`** — Extracted to `internal/app/modules/upscale/` with helpers.go, types.go, and view.go. Root file is thin shim delegating to internal package.

### Media Engine
- [x] **SplitView fixes** — Divider color and draggable divider.
- [x] **AudioPlayer** — Volume, mute, pause/resume, error handling.
- [x] **Engine** — VideoInfo, pause/resume, volume/mute/speed, seek accuracy.
- [x] **Queue** — Configurable max size limits.
- [x] **Subtitle extraction** — SubtitleExtractor with SRT/ASS export.
- [x] **Tests** — Comprehensive tests for media package.
- [x] **Player deprecation** — MPV/VLC deprecated, FFplay + Native only.
- [x] **HW accel detection** — Runtime encode probes instead of ffmpeg -hwaccels to prevent false positives.
- [x] **HW decode support** — GPU-accelerated video decoding via FFmpeg hwcontext API (VAAPI, D3D11VA, QSV).

### Disc Authoring
- [x] **Native engine stabilisation** — PGC/TT_SRPT/TMAPT/menu VOB/ISO 9660 complete.
- [x] **DVD engine Phase 4.2–4.3** — SPU DCSQ rewrite + 30 integration tests across spu/ifo/vob/udf. All green.

## Dev34 Scope (completed ✓)

- [x] **`internal/i18n/` package** — Typed `Strings` struct; `T()`, `SetLanguage()`, listener callbacks; full UI refresh on language change without restart.
- [x] **English (Canada) — en-CA** — 100% source-of-truth translation file.
- [x] **French (Canada) — fr-CA** — Initial translation pass.
- [x] **Inuktitut — iu** — Initial translation pass (Syllabics + Latin toggle).
- [x] **Language selector in Settings** — Dropdown; change takes effect immediately including active module.
- [x] **Aboriginal Sans font embedded** — Regular + Bold embedded for UCAS/syllabics rendering.
- [x] **RIFE frame interpolation** (issue #23) — `rife-ncnn-vulkan` integrated into Upscale module.
- [x] **Native media engine Phase 1** — Core decode pipeline, PacketQueue, AudioPlayer, MasterClock, A/V sync, frame stepping, Seek. Gated behind `native_media` build tag.
- [x] **SplitView widget** — Side-by-side comparison wired into Compare under `native_media` tag.
- [x] **Module extraction** — audio, filters, inspect, thumbnail, player, enhancement, compare → `internal/app/modules/`.
- [x] **Module colour consistency** — All modules receive `ModuleColor` via Options; nav bar always matches main menu tile.
- [x] **Back button i18n + uppercase** — All modules use `strings.ToUpper(t.ModuleXxx)`.
- [x] **Inspect crash fix** — nil guard on all `OnGetXxx` callbacks.
- [x] **Hardware accel dropdown** — Only available backends shown; stale value resets to auto.
- [x] **Convert output prefill fix** — No stale filename on fresh launch.
- [x] **Disc category consolidation** — Blu-ray tile retired; single "Show Disc category" toggle in Settings.
- [x] **DVD authoring advances** — Multitrack, ScriptableTheme, native renderer, Archivist round-trip, IFO reading, VOBU_ADMAP, VTS Attribute Table, auto disc scan.
- [x] **VT_LOGO-2** — New app icon and logo.
- [x] **Subtitles i18n strings** — 9 new strings added (SubtitlesOfflineHint, SubtitlesEmpty, SubtitlesExtractEmbed, SubtitlesOCROutput, SubtitlesOCRLanguage, SubtitlesShiftOffset, SubtitlesStart, SubtitlesEnd).
- [x] **Subtitles i18n wired** — All 9 strings wired up in subtitles_module.go.
- [x] **Audio/Filters/Inspect i18n wired** — Module views now use t.* strings.
- [x] **Trim module stub update** — Added OnAddToQueue, TrimClip, ModuleColor, OnShowQueue to match main.go.
- [x] **Back button consistency** — Module name uppercase.
- [x] **Thumbnail contact sheet fix** — Header height + filename truncation.

## Code Quality Issues (dev39 carry-forward)

### Dead Code / Unused Code
- [x] **Remove darwin/macOS code blocks** — AGENTS.md states macOS not supported. Removed from main.go, settings_module.go, internal/utils/gui_detection.go, internal/sysinfo/sysinfo.go, internal/player/factory.go, internal/app/modules/settings/types.go. Remaining only in _fyne (vendored).
- [x] **Fix unused parameters** — Added explanatory comments in validation.go and proc_other.go

### Silent Error Handling
- [ ] **Log instead of discard errors** — Multiple places where errors are silently ignored:
  - `internal/dvd/vob/sri.go:70` — `io.ReadFull` error discarded
  - `internal/player/gstreamer_player.go:269` — Seek error explicitly discarded
  - `internal/queue/edit.go:139-144,192-197` — JSON marshal errors silently ignored
  - `internal/convert/dvd.go:155-163` — `fmt.Sscanf` error ignored

### Missing i18n Strings (50+ hardcoded)
- [x] **internal/app/modules/deps/dialog.go:32** — "Missing Dependencies" → Added DependenciesMissing
- [ ] **main.go** — ~50 hardcoded strings ("Close", "Cancel", "Convert Now", dialog titles, button labels)
- [ ] **internal/ui/command_editor.go** — ~15 hardcoded strings ("Ready", dialog strings)
- [ ] **internal/ui/benchmarkview.go** — ~20 hardcoded strings ("CPU:", "GPU:", labels)

### Debug Output in Production
- [ ] **Replace fmt.Println/Printf** — Debug statements should use proper logging:
  - `internal/modules/handlers.go:16-101` — Multiple `fmt.Println()` debug output
  - `internal/ui/components.go:269,275,287` — `fmt.Printf()` in drop handler

### Race Conditions
- [x] **Queue notifyChange goroutine** — `internal/queue/queue.go:100-104` spawns goroutine accessing queue state without locking (fixed dev45)


## Future: Canadian Aboriginal Languages

Aboriginal Sans covers all Canadian Indigenous languages. Future language options to add:
- Inuktitut (syllabic + Latin)
- Cree (syllabic + Latin)
- Ojibwe (Anishinaabemowin)
- Mohawk
- Mi'kmaq
- Dene
- Oji-Creek
- Dakota/Sioux

## Future: Indigenous Language Whisper Support

Whisper (especially whisper.cpp) supports many Canadian Indigenous languages. Combining:
- **UI localization** (i18n we built)
- **Whisper subtitle extraction** in Indigenous languages
- **DVD authoring** with Indigenous language subtitles/menus

This would be a powerful pipeline for Indigenous content creators.

Approach:
- [ ] **Ship whisper.cpp binary** (compiled, ~1-2MB) - embed in app or download on first use
- [ ] **Model hosting** - Base model (~74MB) needs a new home (GitHub Releases or Hugging Face)
- [ ] **Fix Whisper dependency** - Currently awkward to integrate; make it seamless like FFmpeg
- [ ] **Whisper in Dependencies menu** - Proper install/uninstall flow
- Haida
- Tlingit

## Dev33 Scope

- [x] **App icon embedding** — `logo_embed.go` at root; logo loaded directly in `main.go`; fixes taskbar icon not showing.
- [x] **FFmpeg hwaccel order fix** — hwaccel flags now passed before the input file in convert pipeline.
- [x] **Upscale Ctrl+Enter shortcut** — Added keyboard shortcut to Upscale module.
- [x] **Version hash in Settings** — Settings > About now shows build commit hash for debugging.
- [x] **Windows CI buildCommit** — Windows CI workflow now passes `-ldflags "-X main.buildCommit=..."` (Linux already had this).
- [x] **CI syntax error fix** — Duplicate `else` block in icon loading code removed; Windows package build now passes.
- [x] **Linux CI apt caching** — Added `actions/cache@v4` for apt packages to speed up Linux CI builds.
- [x] **Wiki maintenance** — Synchronized `docs/` to Forgejo wiki; established `Home.md`, `Documentation.md`, and `_Sidebar.md`.
- [ ] **Native Disc Authoring (Issue #21)** — Implement native Go replacement for `dvdauthor`, `spumux`, and `xorriso` within a unified Author/Rip framework.
    - [x] Phase 1: UDF 1.02 / ISO 9660 Bridge writer.
    - [x] Phase 2: MPEG-PS / VOB muxer with NAV_PCK support.
    - [x] Phase 3: IFO/BUP structure generation.
    - [x] Phase 4: SPU (subpicture) encoding for menu buttons.
    - [x] Phase 5: Integration with Author/Rip modules (Unified UI, native UDF reader, cross-platform enablement).
- [ ] **Root folder hygiene** (issue #22) — 24 `package main` files clutter the project root; should be progressively extracted to `internal/app/modules/` with thin root shims, following the pattern already established for `about`, `deps`, and `mainmenu`.
- [x] **Drag and drop into Convert** — Files dragged onto the Convert module drop zone not being registered (carry-forward from dev32). Shipped dev44 — `showConvertView` wraps content in `ui.NewDroppable`, loaded/placeholder panes have explicit drop targets, single/multi-file drops unified via `loadMultipleVideos` (`67a7f126`, `d42ef26a`, `009bc26f`).
- [ ] **Linux CI build optimization** — Even with apt caching, Linux builds install ~100 packages. Consider a pre-built container image with deps baked in to cut build times.

## Dev32 Scope

- [x] **Drag-to-scroll** (issue #19) — Fixed by replacing inner `container.Scroll` in `FastVScroll` with a custom `scrollClip` widget that does not implement `fyne.Draggable`; drag events now reach `FastVScroll`.
- [x] **Windows icons** (issue #20) — Icons embedded via `//go:embed assets/icons`; `GetIcon` reads from embedded `fs.FS` with no runtime disk access.
- [x] **Updates tab — real check** — Wired to Forgejo tags API (`/api/v1/repos/leak_technologies/VideoTools/tags?limit=1`); fixed owner mismatch in URL.
- [x] **Dependency install buttons** — Per-dependency Install/Uninstall buttons in Settings.
- [x] **SVG icon library** — Material Design icons added to `assets/icons/`.
- [x] **Convert UI cleanup** (issue #5) — Label alignments and separators standardised.
- [x] **Hide/show player in Compare** (issue #1) — Toggle button added to Compare module.
- [x] **Author/Rip hidden on Windows** — Disc modules hidden from main menu on Windows until cross-platform disc authoring is implemented.
- [x] **Convert player layout** — Video fills centre of player pane; transport bar pinned to bottom; VSplit gap removed.
- [x] **Convert active state** — `s.active = "convert"` now set correctly; drop handling and keyboard shortcuts work inside the module.
- [x] **Drag and drop into Convert** — Files dragged onto the Convert module drop zone not being registered. Shipped dev44, see above.
- [x] **Phase 3 modularisation — Inspect, Settings, Queue**

## Dev31 Scope

- [x] **Module settings panel scrolling** (issue #3)
  - Scroll containers added to all non-Convert module settings panels. Action buttons moved to footer action bar.
- [x] **Window resize stability** (issue #4)
  - setContent now pins window size after SetContent to prevent layout-driven resize on module switch.
- [x] **Convert UI cleanup** (issue #5)
  - Layout consistency, label clarity, and control organization pass for external developer testing readiness. Completed in dev33 alignment pass.
- [x] **Phase 3 modularisation — Inspect, Settings, Queue**
   - Move `buildInspectView`, `showSettingsView`/`buildSettingsView`, and `showQueue`/queue view out of `main.go` into dedicated `*_module.go` files.
   - Inspect, queue, subtitles, and upscale modules now in `internal/app/modules/`.
   - **Note**: Convert module partially modularized (entry point + state/callbacks in `internal/app/modules/convert/view.go`). Full `buildConvertView` extraction deferred due to high coupling with appState (~3500 lines, ~30+ state fields). Future work should consider extracting logical subsections first.

## Near-Term Milestone: Frame Interpolation (RIFE)

**Issue #23** — Add RIFE frame interpolation as a section in the Upscale module (or its own module later).

RIFE (Real-Time Intermediate Flow Estimation) increases frame rate by synthesising intermediate frames.
It uses `rife-ncnn-vulkan` — same deployment model as `realesrgan-ncnn-vulkan`.

- [ ] **Detect rife-ncnn-vulkan** — check `$PATH` at startup; surface install link in Settings if missing.
- [ ] **UI: Frame Interpolation section** — multipier select (2×/4×/8×), show estimated output fps.
- [ ] **Pipeline** — extract frames → rife-ncnn-vulkan → reassemble (can chain after Real-ESRGAN upscale).
- [ ] **Settings install button** — same pattern as Real-ESRGAN dependency install button.

## Near-Term Milestone: Native Media Engine (VideoTools Player)

**Priority: Critical** — Replaces GStreamer dependency with a native FFmpeg-based engine (CGO). Enables "pro grade" features like split-view comparison and frame-accurate trimming.

- [ ] **Plan & Architecture**
    - [x] Design Native Media Engine plan (`plans/native-player-engine.md`).
    - [ ] Create `internal/media` package structure.
- [ ] **Phase 1: Basic Decode**
    - [x] CGO bindings for `libav*` and `engine.go` scaffolding.
    - [x] Single frame decoding to `image.RGBA`.
- [ ] **Phase 2: Playback & Sync**
    - [x] Thread-safe packet queue.
    - [x] Audio decoding (`libavcodec`) and output (`oto`).
    - [x] A/V sync master clock and frame timing.
- [ ] **Phase 3: Advanced Control**
    - [x] Frame-accurate seeking and stepping.
- [ ] **Phase 4: Split-View Integration**
    - [x] Integrate native `SplitView` into Compare module.
    - [x] Implement dual-engine playback loop in `compare/view.go`.
- [ ] **Phase 5: Trim Module Integration**
    - [x] Implement professional UI layout (Internal Trim module).
    - [x] Create reusable `media.VideoPlayer` widget.
    - [ ] Implement frame-accurate scrubbing loop.
    - [ ] Link In/Out points to `internal/queue` for lossless processing.
- [x] **Phase 6: GStreamer Removal**
    - [x] Remove `internal/player/gstreamer*` (done dev42).
    - [x] Remove GStreamer from build scripts and CI (done dev42).


## Road to v0.1.2 — First Public Stable Release

GitHub is now the primary origin for both dev and public releases.
Dev builds ship via GitHub Actions CI; public releases via a separate release workflow.

### Stability Criteria (must all be green before tagging v0.1.2)

- [ ] **CI green on both platforms** — Linux and Windows package builds pass consistently.
- [ ] **Root folder hygiene complete** — All modules extracted; root has only the files that must live there (embed files, main.go, platform shims).
- [ ] **main.go under control** — Core `appState` coupling reduced enough that `main.go` is a thin orchestrator, not a monolith.
- [ ] **Cross-platform player** — Unified player backend working on Linux and Windows (unblocks Upscale live preview and Trim).
- [ ] **Localization baseline** — English + French (Canada) shipped; translation percentage tracking in place; dynamic UI switching working.
- [ ] **No critical known bugs** — Convert, Upscale, Thumbnail, Filters, Audio, and Subtitles modules all function correctly on both platforms.
- [ ] **Drag and drop working** — Files dropped onto the Convert module are registered (issue carried from dev32).
- [ ] **GitHub Actions workflow** — `.github/workflows/` produces release artifacts (Windows MSIX + Linux AppImage/deb) for the GitHub release page.
- [ ] **Public README / release notes** — Codebase is presentable for public contributors.

### Module Extraction Plan (issue #22) — ordered by priority

Modules already properly extracted to `internal/app/modules/`:
- [x] `about`, `convert`, `deps`, `mainmenu` (dev33 and earlier)
- [x] `audio`, `filters`, `inspect`, `thumbnail`, `player`, `enhancement`, `compare` (dev34)

Remaining root files to extract — largest first (each becomes a dev cycle task):

| Priority | Root file(s) | Lines | Target |
|----------|-------------|-------|--------|
| 1 | `author_module.go`, `author_menu.go`, `author_dvd_functions.go` | ~5,800 | `internal/app/modules/author/` |
| 2 | `subtitles_module.go` | 1,782 | `internal/app/modules/subtitles/` |
| 3 | `settings_module.go` | 1,381 | `internal/app/modules/settings/` — in progress |
| 4 | `upscale_module.go` | 993 | `internal/app/modules/upscale/` |
| 5 | `rip_module.go` | 704 | `internal/app/modules/rip/` — pair with author extraction |
| 6 | `queue_module.go` | 355 | `internal/app/modules/queue/` — in progress |
| — | `thumbnail_config.go` | 45 | `internal/thumbnail/` |
| — | `naming_helpers.go` | 62 | `internal/utils/` |
| — | `merge_config.go` | 47 | `internal/app/` or `internal/convert/` |
| — | `deps_dialog_module.go` | 19 | `internal/app/modules/deps/` |

**Files that must stay at root** (`go:embed` path constraint):
`main.go`, `fonts_embed.go`, `icons_embed.go`, `logo_embed.go`

**Platform shims — fine at root:**
`update_windows.go`, `update_linux.go`, `platform.go`

**Approach**: Extract one or two modules per dev cycle. Each extraction follows the established pattern:
logic moves to `internal/app/modules/{name}/`, a thin `package main` shim at root delegates to it, `appState` fields needed by the module are passed as parameters or callbacks.

## Maintenance


- [X] **About dialog cleanup**
  - Remove the Bitcoin address from the About/Support page.
- [X] **Snippet AV1 fallback**
  - Fall back to available AV1 encoders (or H.264) when `libsvtav1` is missing.
- [X] **Adaptive scroll speed**
  - Smooth settings and long panels across different window sizes.
- [X] **Click-and-drag scrolling**
  - FastVScroll supports click-and-drag (desktop.Mouseable + fyne.Draggable) for mobile/touch-like interaction.
- [X] **Settings tab scrolling**
  - Scroll within tabs while keeping the Settings header visible.
- [X] **Aspect ratio handling**
  - Use display aspect ratio metadata when available and surface a 17:9 target for non-16:9 sources (user report).
- [X] **Aspect ratio UI clarity**
  - Show the detected source aspect ratio next to the Source aspect option.
- [X] **Aspect ratio logging + crop hygiene**
  - Log source/target aspect details and ignore stored crop values when auto-crop is disabled.
- [X] **Custom aspect ratio input**
  - Provide a Custom... option instead of bloating the dropdown with cinema ratios.
- [X] **Aspect/scale alignment**
  - Ensure aspect conversion uses target resolution so outputs land on exact sizes (e.g., 1920x1080).
- [X] **Window resize stability**
  - Avoid resizing the main window on every module switch to prevent misclicks.
- [X] **Conversion stability**
  - Catch conversion worker panics and surface a failure dialog instead of closing the UI.
- [X] **Conversion recovery notice**
  - Persist the last conversion state and show a lightweight notice on next launch if it was active.
- [X] **Convert compile regressions**
  - Fixed duplicate aspect/scale block and late custom-aspect declarations in `main.go`.
- [X] **Windows import regression**
  - Removed stale `go-qrcode` import in `main.go` after About module extraction.
- [X] **Windows Tesseract packaging fallback**
  - Download missing `eng/fra/iku` traineddata during bundled packaging before enforcing required checks.
- [X] **GStreamer CI packaging policy**
  - Treat GStreamer as optional in bundled artifacts and do not fail packaging when absent.
  - Note: GStreamer fully removed in dev42 — all references above are historical.
- [X] **Whisper model packaging resilience**
  - Added multi-source fallback download and continue-on-failure behavior in bundled packaging.
- [X] **Linux bundled zip tooling**
  - Added `zip` to Linux workflow dependency install list.
- [X] **Dev pipeline simplification**
  - Skip bundled package generation on dev channel; keep bundled artifacts for stable channel only.
- [X] **Bundled artifact retirement**
  - Removed bundled package generation from Linux/Windows dev-packages workflow.
- [X] **Main menu width containment**
  - Replaced incorrect adaptive-grid usage with wrapping tile layout to prevent oversized windows.
- [X] **Main menu row alignment**
  - Set module sections to a stable 3-column grid for consistent row structure.
- [X] **Release asset replacement**
  - Fixed purge + name-based asset deletion in publish workflow to prevent release zip accumulation.
- [X] **Publish delete endpoint fix**
  - Updated Forgejo asset delete endpoint path to stop 404 failures in publish job.
- [X] **Release tag targeting**
  - Forgejo release publish now sources version from `VERSION` first and updates only the matching tag release metadata.
- [X] **FFprobe path consistency**
  - Use configured/app-local FFprobe paths in convert analysis and thumbnail metadata probes instead of hardcoded `ffprobe`.
- [X] **Release note readability**
  - Publish concise highlights from the matching changelog version section instead of embedding the full raw section in Forgejo release comments.
- [X] **Stale publish run guard**
  - Skip dev release publish when a newer `master` commit exists so old runs cannot overwrite newer release/tag state.
- [X] **Docs portal migration**
  - Repoint About/QR documentation links and install docs from retired `docs.leaktechnologies.dev` to GitHub wiki + in-repo docs.
- [X] **Blu-ray visibility toggle**
  - Added `Show Blu-ray module` preference and tied it to main menu module visibility.
- [X] **Benchmark scope clarity**
  - Benchmark apply path now changes hardware acceleration only (not codec or preset).
- [X] **Versioning docs clarity**
  - Documented continuous global `-devN` numbering and public release base-version behavior.
- [X] **Module testing coverage checklist**
  - Added a full module-by-module release checklist in `docs/TESTING_MODULE_CHECKLIST.md`.
- [X] **Root structure hygiene pass**
  - Removed root-level scratch artifacts, moved QR demo entry to `cmd/qr_about_demo/`, and documented root cleanliness rules in `AGENTS.md`.
- [ ] **Dev30 phased refactor (build-safe)**
  - [x] Phase 2a: Moved module config path helper to `internal/app/configpath` and updated module call sites.
  - [x] Phase 2b: Moved merge/thumbnail config persistence logic into `internal/app/modulecfg` with compatibility wrappers in `package main`.
  - [x] Phase 2c: Moved naming metadata/output-base helper logic into `internal/app/naming` with wrappers in `package main`.
  - [x] Phase 2d: Moved rip/subtitles config persistence logic into `internal/app/modulecfg` with compatibility wrappers in module files.
  - [x] Phase 2e: Moved author config persistence logic into `internal/app/modulecfg` with compatibility wrappers in `author_module.go`.
  - [x] Phase 2f: Moved audio config persistence logic into `internal/app/modulecfg` with compatibility wrappers in `audio_module.go`.
  - [x] Phase 2g: Replaced duplicated config path helpers in `main.go` with shared `internal/app/configpath` usage for convert/recovery/benchmark/history.
  - [x] Phase 2h: Moved recovery/benchmark/history config persistence logic into `internal/app/appcfg` with aliases/wrappers in `main.go`.
  - [x] Phase 2i: Moved convert config JSON load/save plumbing to shared `internal/app/appcfg` store helpers while keeping convert normalization logic in `main.go`.
  - [x] Phase 2j: Moved convert config normalization rules into `internal/app/appcfg` and kept `main.go` as a thin wrapper.
  - [x] Phase 3a: Moved About dialog implementation to `internal/app/modules/about` with a root shim.
  - [x] Phase 3b: Moved missing-dependencies dialog rendering to `internal/app/modules/deps` with a root shim.
  - [x] Phase 3c: Moved main menu visibility/dependency filtering and active-job mapping helpers to `internal/app/modules/mainmenu`.
  - [x] CI fix: Restored missing `path/filepath` import in `audio_module.go` after refactor to fix Linux/Windows package build failures.
  - [x] CI fix: Restored missing `path/filepath` import in `rip_module.go` after refactor to fix Linux/Windows package build failures.
  - [x] CI fix: Restored missing `encoding/json` and `path/filepath` imports in `subtitles_module.go` after refactor to fix Linux/Windows package build failures.
  - Phase 2: Introduce `internal/app/` boundaries and move low-risk helper files from root.
  - Phase 3: Move module builders into `internal/app/modules/` with compatibility shims if needed.
  - Phase 4: Move primary app entrypoint toward `cmd/videotools/` once app package is stable.
- [ ] **Main.go modularization pass** - Move UI builders and helpers into module files (convert remaining)
- [X] **Main menu modularization** - Moved main menu builder + refresh helpers into `mainmenu_module.go`.
- [X] **Settings modularization** - Settings view lives in `settings_module.go` (no main.go UI builder).
- [X] **About/Support modularization** - Moved About/Support dialog into `about_module.go`.
- [X] **Missing dependencies dialog modularization** - Moved the missing dependencies dialog into `deps_dialog_module.go`.
- [X] **Queue view modularization** - Moved queue view builders and refresh helpers into `queue_module.go`.
- [X] **Inspect view modularization** - Moved inspect view into `inspect_module.go`.
- [X] **Subtitle OCR support**
  - Enable OCR for image-based DVD/BD subtitle tracks (VobSub/PGS) to produce SRT/ASS.
- [X] **Subtitle OCR cleanup**
  - Normalize OCR text and merge consecutive duplicate cues for cleaner timing.
- [ ] **Windows dev28 dependency bootstrap validation**
  - Verify first-run FFmpeg bootstrap on a clean Windows machine with no FFmpeg in PATH.
  - Confirm modules unlock immediately after in-app install without restarting the app.
- [ ] **Cross-platform dependency actions validation**
  - Verify Settings > Dependencies FFmpeg install/uninstall buttons behave correctly on Linux package managers (apt/dnf/pacman/zypper).
- [ ] **Convert UI icon pass**
  - Replace temporary ASCII-safe Convert control labels with proper icon resources once the icon set is finalized.

- [ ] **Git converter retirement**
  - Preserve presets, then deprecate `scripts/legacy/git_converter`.
- [ ] **Forgejo Windows runner validation**
  - Confirm the Windows packaging workflow completes without context canceled after the UTF-8 `GITHUB_OUTPUT` fix.
- [ ] **Forgejo Windows package validation**
  - Confirm Windows zip contains only `VideoTools.exe` and `README.md`.
  - Confirm Windows zip is written to `dist/windows/` with no build metadata files.
  - Confirm diagnostics folder is not included in uploaded artifacts.
- [ ] **Forgejo dev release validation**
  - Confirm dev release workflow builds Linux and Windows artifacts before publish steps.
  - Confirm release assets exclude build metadata files.
  - Confirm release assets replace existing files with the same name.
  - Confirm release assets are purged before upload (no duplicates remain).
  - Confirm publish fails loudly when asset deletion/upload is unauthorized.
  - Confirm dev release note body includes nightly build context plus concise highlights from matching `docs/CHANGELOG.md` version section.
- [x] **Dev30 finalization execution**
  - CI validated (runs 219/220/221, commit 2cbb3a2). Smoke test and dependency validation carried forward as issues #3/#4/#5/#18.
- [x] **Dev31 kickoff**
  - Bumped to `v0.1.1-dev31`. Issue tracker populated. AGENTS.md updated.
- [ ] **Forgejo workflow validation**
  - Confirm redundant Windows workflow removal doesn't break release publishing.
  - Confirm test trigger workflow removal doesn't affect CI visibility.
- [ ] **Forgejo workflow validation**
  - Confirm redundant Linux workflow removal doesn't break release publishing.
- [X] **Forgejo dev-packages workflow**
  - Fixed YAML parsing in bundled deps note generation.
- [ ] **Forgejo mirror validation**
  - Confirm built-in push mirror updates Codeberg successfully.
- [ ] **Documentation naming hygiene**
  - Review new docs for personal names; use user report/dev report labels only.
- [X] **Installer dependency parity**
  - Ensure pip is installed on Linux/Windows and skip Go/pip when already present.
- [X] **Windows installer parse fix**
- [X] **Windows GCC PATH verification** - Align MSYS2 GCC detection with PATH checks.
  - Normalize PowerShell here-strings to prevent parser errors.
- [X] **Go auto-install in install.sh**
  - Skip prompting for Go and install automatically when missing.
- [X] **Windows install workflow split**
  - Route Windows to the dedicated installer to avoid mixed-shell prompts.
- [X] **Windows installer entrypoint**
  - Provide a PowerShell entrypoint and direct Windows users to it from install.sh.
- [X] **Windows DVDStyler mirror fallback**
  - Added Leak Technologies mirror URL and EXE fallback for DVD authoring tools.
- [X] **Windows script output alignment**
  - Match Linux-style headers and show build metadata in build.ps1 output.
- [X] **Windows build toolchain repair**
  - Refresh PATH and auto-repair missing MSYS2 GCC during builds when possible.
- [X] **Forgejo dev packaging**
  - Add Forgejo Actions workflow for Windows/Linux dev packages and artifacts.
- [X] **Bundled whisper model**
  - Include the whisper.cpp small model in bundled Windows/Linux packages.
- [X] **Forgejo dev release upload**
  - Upload dev artifacts to a Forgejo release when a token is provided.
- [ ] **Windows signing**
  - Provide production signing cert and wire it into Forgejo secrets for signed releases.
- [ ] **Forgejo runner labels**
  - Ensure runners are online with labels: ubuntu (Linux) and windows.
- [X] **Whisper model mirror fallback**
  - Prefer Leak Technologies mirror for whisper.cpp small model downloads.
- [X] **Git Bash handoff**
  - Keep Windows installs in the same Git Bash terminal using `winpty` when available.
- [X] **Windows root entrypoints**
- [X] **Linux script path fix**
  - Correct Linux build/install/run paths after scripts reorg.
  - Provide `install.bat` and `install.ps1` for PowerShell-first installs.
- [X] **Windows scripts entrypoints**
  - Provide `scripts/install.ps1` and `scripts/install.bat` to avoid Git Bash pop-ups.
- [X] **Windows setup launcher alignment**
  - Route `scripts/_internal/setup-windows.bat` through `scripts/install.bat`.
- [X] **Agent workflow rules**
  - Add `AGENTS.md` to enforce staging, commits, and documentation updates.
- [X] **Player fullscreen toggle**
  - Add fullscreen toggle to the Player module controls.
- [X] **Player EOS handling + metadata access**
  - Stop playback cleanly on EOS and expose duration/FPS from GStreamer.
- [X] **Main menu title cleanup**
  - Header shows "VideoTools" only; platform suffix moved to footer version label.
- [X] **Main menu palette refresh**
  - Restore a diverse, eye-friendly rainbow palette while keeping Convert constant.
- [X] **Main menu readability**
  - Increase tile label size and adjust low-contrast colors.
- [X] **Main menu contrast tuning**
  - Refine Audio, Rip, and Settings colors for legibility.
- [X] **Main menu layout cleanup**
  - Remove the scroll container so the main menu scales without scroll bars.
- [X] **Player silhouette placeholder**
  - Keep a stable player footprint before media loads.
- [X] **Main menu palette tuning**
  - Adjust audio/compare/subtitles colors for clearer separation.
- [X] **Main menu vibrancy pass**
  - Remove monochrome tiles outside Settings.
- [X] **Main menu bespoke hues**
  - Assign unique hue families to each module for maximum legibility.
- [X] **Locked tile hue preservation**
  - Keep disabled modules colored while subdued.
- [X] **Locked hue visibility**
  - Reduce stripe opacity and raise label brightness.
- [ ] **Main package layout cleanup**
  - Move root `package main` files into `cmd/videotools` when the build is stable.
- [ ] **Windows packaging prep**
  - [x] Draft MSIX + WinGet layout under `packaging/windows/`.
  - [x] Add GitHub Actions workflow to build MSIX and upload release artifacts.
  - [ ] Wire signing step (SignTool) once certificate is available.
  - [ ] Keep Windows installer aligned with MSIX/WinGet production path.

- [ ] **Git converter integration**
  - Move git_converter workflow into the main VT UI and retire legacy scripts.
- [ ] **Windows installer validation**
  - Test `scripts\_internal\install-deps-windows.ps1` MSYS2 flow and GStreamer MSI install on Windows 10/11.
  - Re-test GStreamer MSI download and local MSI override after variable fix.
  - Confirm mirror fallback works when the primary download returns HTML.
  - Verify winget fallback works when MSI downloads fail.
  - Confirm winget-first flow succeeds on clean Windows VM.
  - Verify DVDStyler winget fallback sets dvdauthor/mkisofs on PATH.
  - Validate MSI-first flow installs GStreamer and DVDStyler without winget.
  - Validate that GStreamer devel MSI is skipped unless build tools are selected.
  - Verify Whisper model prompt/download and mirror override on Windows.

## Documentation: Fix Structural Errors

**Priority:** High

- [X] **Audit All Docs for Broken Links:**
  - Systematically check all 46 `.md` files for internal links that point to non-existent files or sections.
  - Create placeholder stubs for missing documents that are essential (e.g., `CONTRIBUTING.md`) or remove the links if they are not.
  - This ensures a professional and navigable documentation experience.

## Critical Priority: dev24

### AUTHOR MODULE: CONTENT TYPES + GALLERIES + CHAPTER THUMBS

- [ ] **Content classification (Feature/Extra/Gallery)**
  - Feature: supports chapters + chapter menus
  - Extra: separate DVD titles; no chapters
  - Gallery: still-image slideshow title under Extras
  - Extras require subtype (behind_the_scenes, deleted_scenes, featurettes, interviews, trailers, commentary, other)
- [ ] **Cross-platform DVD authoring parity**
  - Ensure Windows and Linux use the same dvdauthor XML + ISO tool flags
  - Treat DVDStyler as a CLI tool bundle only (no GUI authoring dependency)
- [ ] **Chapter screenshot generation (Feature only)**
  - Auto-generate one still per chapter (default 2s offset)
  - Fallback to first valid frame on failure
  - Allow per-chapter override image
- [ ] **Menu structure rules**
  - Main: Play Feature, Chapters (if any), Extras (if extras/galleries)
  - Extras menu groups by subtype; galleries listed separately
- [ ] **UI layout guardrails**
  - Separate Feature / Extras / Galleries sections
  - Chapters disabled when content type is not Feature
- [ ] **Schema + config updates**
  - Add content_type per video, gallery assets list, chapter thumb config
  - Persist extras subtype and gallery behavior (auto-advance, loop)

## AUTHOR MODULE: MENU TEMPLATES & THEMES

**Template** = Layout/structure (how buttons are arranged)
**Theme** = Visual aesthetic (colors, feel - sci-fi, western, minimal, etc.)

### Template Layouts (Menu Structure)

- [x] **Minimal** - Black background, white text, vertical button list (cleanest style)
- [x] **Classic** - Title at top, buttons below centered (traditional DVD)
- [x] **Grid** - 2x2 or 3x2 button matrix (Star Wars style)
- [x] **Filmstrip** - Thumbnail preview buttons for scene selection
- [x] **Poster** - Large background image with overlay (current implementation)
- [ ] **Cinematic** - Wide banner title, full-bleed background (modern)

### Preset Themes (Visual Aesthetic)

- [x] **VideoTools** - Background: #0F172A, Text: #E1EEFF, Accent: #7C3AED (sci-fi/purple)
- [x] **Minimal** - Background: #000000, Text: #FFFFFF, Accent: #AAAAAA (clean B&W)
- [x] **Western** - Background: #1A1408, Text: #F5DEB3, Accent: #8B4513 (warm browns/golds)
- [x] **Film Noir** - Background: #1A1A1A, Text: #E0E0E0, Accent: #808080 (dark gray)
- [ ] **Classic Hollywood** - Background: #000000, Text: #F5F5DC, Accent: #D4AF37 (gold)
- [ ] **Warm Cinema** - Background: #1A0F0A, Text: #FFF5E6, Accent: #E67E22 (orange)
- [ ] **Ocean** - Background: #0A1A2A, Text: #E0F0FF, Accent: #00CED1 (cyan)
- [ ] **Nature** - Background: #0A1A0A, Text: #E6FFE6, Accent: #DAA520 (green-gold)
- [ ] **Custom** - User-defined colors via color pickers

### Menu Types (What Gets Generated)

- [ ] **Main Menu** - Primary entry point with Play, Scene Selection, Special Features, Setup
- [ ] **Scene Selection** - Chapter thumbnails with preview images
- [ ] **Special Features** - Extras/trailers submenu
- [ ] **Setup/Languages** - Audio/subtitle language selection
- [ ] **Movie Title** - Opening video before menu (pre-menu)

### Customization Options (Core Features)

- [x] Background image selection per template (currently works for Poster template)
- [x] **Custom background for ALL templates** - Allow user to provide their own background image (PNG/JPG) for any template
- [x] **Motion backgrounds** - Support looping video backgrounds (MPG) - audio is embedded in the video file
- [x] Logo overlay (title/studio) with position/scale/margin controls
- [ ] Custom font support (TTF/OTF)
- [ ] Button style/shape options

### Professional Features (Archivist Focus)

- [x] **Chapter thumbnail generation** - Auto-capture thumbnails from video for scene selection
- [ ] **Menu preset save/load** - Save theme+template combinations as reusable presets
- [ ] Multiple audio track support - Multiple language audio streams
- [ ] **Subtitle selection** - Include subtitle streams in DVD
- [ ] **Disc label/cover** - Generate printable disc label images
- [ ] **Widescreen (16:9) support** - Proper 16:9 DVD authoring
- [ ] **DVD-9 (dual-layer) support** - Handle discs larger than 4.7GB

### Dependencies & Cross-Platform

- [X] **WSL ISO creation on Windows** - Use WSL xorriso for consistent DVD ISO generation
- [ ] **ISO creation verification** - Test that Windows WSL path conversion works correctly
- [ ] **Error handling** - Clear messages when WSL not installed or tools missing

### Implementation Priority

**Phase 1: Core Menu System**
1. Separate Template and Theme dropdowns in UI
2. Add themes: VideoTools, Minimal, Western, Film Noir, Classic Hollywood
3. Minimal template implementation (the clean black bg + white text style)
4. Custom background image support for all templates
5. Classic template

**Phase 2: Visual Polish**
6. Custom theme color pickers (Custom theme)
7. Animated background support (motion menus)
8. Background audio support
9. Grid template
10. Filmstrip template

**Phase 3: Professional Features**
11. Chapter thumbnail generation for scene selection
12. Menu preset save/load
13. Multiple audio/subtitle tracks
14. Disc label generation
15. DVD-9 dual-layer support

### VIDEO PLAYER IMPLEMENTATION

**CRITICAL BLOCKER:** All advanced features (enhancement, trim, advanced filters) depend on stable player foundation.

#### Current Player Issues (from docs/PLAYER_PERFORMANCE_ISSUES.md):

1. **Separate A/V Processes** (lines 10184-10185 in main.go)
   - Video and audio run in completely separate FFmpeg processes
   - No synchronization mechanism between them
   - They will inevitably drift apart, causing A/V desync and stuttering
   - **FIX:** Implement unified FFmpeg process with multiplexed output

2. **Audio Buffer Too Small** (lines 8960, 9274 in main.go)
   - Currently 8192 samples = 170ms buffer
   - Modern systems need 100-200ms buffers for smooth playback
   - **FIX:** Increase to 16384-32768 samples (340-680ms)

3. **Volume Processing in Hot Path** (lines 9294-9318 in main.go)
   - Processes volume on EVERY audio sample in real-time
   - CPU-intensive and blocks audio read loop
   - **FIX:** Move volume processing to FFmpeg filters

4. **Video Frame Pacing Issues** (lines 9200-9203 in main.go)
   - time.Sleep() is not precise, cumulative timing errors
   - No correction mechanism if we fall behind
   - **FIX:** Implement adaptive timing with drift correction

5. **UI Thread Blocking** (lines 9207-9215 in main.go)
   - Frame updates queue up if UI thread is busy
   - No frame dropping mechanism
   - **FIX:** Implement proper frame buffer management

6. **No Frame-Accurate Seeking** (lines 10018-10028 in main.go)
   - Seeking kills and restarts both FFmpeg processes
   - 100-500ms gap during seek operations
   - No keyframe awareness
   - **FIX:** Implement frame-level seeking without process restart

#### Player Implementation Plan:

**Phase 1: Foundation (Week 1-2)**
- [ ] **Unified FFmpeg Architecture**
  - Single process with multiplexed A/V output using pipes
  - Master clock reference for synchronization
  - PTS-based drift correction mechanisms
  - Ring buffers for audio and video

- [x] **Hardware Acceleration Integration** (done dev44)
  - [x] Upscale module: sync upscaleHardwareAccel from master setting
  - [x] Filters module: add hardware acceleration dropdown
  - Auto-detect available backends (CUDA, VA-API, VideoToolbox)
  - FFmpeg hardware acceleration through native flags
  - Fallback to software acceleration when hardware unavailable

- [ ] **Frame Extraction System**
  - Frame extraction without restarting playback
  - Keyframe detection and indexing
  - Frame buffer pooling to reduce GC pressure

**Phase 2: Core Features (Week 3-4)**
- [ ] **Frame-Accurate Seeking**
  - Seek to specific frames without restarts
  - Keyframe-aware seeking for performance
  - Frame extraction at seek points for preview

- [ ] **Chapter System Integration**
  - Port scene detection from Author module
  - Manual chapter support with keyframing
  - Chapter navigation (next/previous)
  - Chapter display in UI

- [ ] **Performance Optimization**
  - Adaptive frame timing with drift correction
  - Frame dropping when UI thread can't keep up
  - Memory pool management for frame buffers
  - CPU usage optimization

**Phase 3: Advanced Features (Week 5-6)**
- [ ] **Preview System**
  - Real-time frame extraction
  - Thumbnail generation from keyframes
  - Frame buffer caching for previews

- [ ] **Error Recovery**
  - Graceful failure handling
  - Resume capability after crashes
  - Smart fallback mechanisms

### ENHANCEMENT MODULE FOUNDATION

**DEPENDS ON PLAYER COMPLETION**

#### Current State:
- [X] Basic filters module with color correction, sharpening, transforms
- [X] Stylistic effects (8mm, 16mm, B&W Film, Silent Film, VHS, Webcam)
- [X] AI upscaling with Real-ESRGAN integration
- [X] Basic AI model management
- [ ] No content-aware processing
- [ ] No multi-pass enhancement pipeline
- [ ] No before/after preview system

#### Enhancement Module Plan:

**Phase 1: Architecture (Week 1-2 - POST PLAYER)**
- [ ] **Model Registry System**
  - Abstract AI model interface for easy extension
  - Dynamic model discovery and registration
  - Model requirements validation
  - Configuration management for different model types

- [ ] **Content Detection Pipeline**
  - Automatic content type detection (general/anime/film)
  - Quality assessment algorithms
  - Progressive vs interlaced detection
  - Artifact analysis (compression noise, film grain)

- [ ] **Unified Enhancement Workflow**
  - Combine Filters + Upscale into single module
  - Content-aware model selection logic
  - Multi-pass processing framework
  - Quality preservation controls

**Phase 2: Model Integration (Week 3-4)**
- [ ] **Open-Source AI Model Expansion**
  - BasicVSR integration (video-specific super-resolution)
  - RIFE models for frame interpolation
  - Real-CUGan for anime/cartoon enhancement
  - Model selection based on content type

- [ ] **Advanced Processing Features**
  - Sequential model application capabilities
  - Custom enhancement pipeline creation
  - Parameter fine-tuning for different models
  - Quality vs Speed presets

### TRIM MODULE ENHANCEMENT

**DEPENDS ON PLAYER COMPLETION**

#### Current State:
- [X] Basic planning completed
- [ ] No timeline interface
- [ ] No frame-accurate cutting
- [ ] No chapter integration from Author module

#### Trim Module Plan:

**Phase 1: Foundation (Week 1-2 - POST PLAYER)**
- [ ] **Timeline Interface**
  - Frame-accurate timeline visualization
  - Zoom capabilities for precise editing
  - Scrubbing with real-time preview
  - Time/frame dual display modes

- [ ] **Chapter Integration**
  - Import scene detection from Author module
  - Manual chapter marker creation
  - Chapter navigation controls
  - Visual chapter markers on timeline

- [ ] **Frame-Accurate Cutting**
  - Exact frame selection for in/out points
  - Preview before/after trim points
  - Multiple segment trimming support

**Phase 2: Advanced Features (Week 3-4)**
- [ ] **Smart Export System**
  - Lossless vs re-encode decision logic
  - Format preservation when possible
  - Quality-aware encoding settings
  - Batch trimming operations

### DOCUMENTATION UPDATES

- [X] **Create PLAYER_MODULE.md** - Comprehensive player architecture documentation
- [X] **Update MODULES.md** - Player and enhancement integration details
- [X] **Update ROADMAP.md** - Player-first development strategy
- [ ] **Create enhancement integration guide** - How modules work together
- [ ] **API documentation** - Player interface for module developers

## Future Enhancements (dev24+)

### AI Model Expansion
- [ ] **Diffusion-based models** - SeedVR2, SVFR integration
- [ ] **Advanced restoration** - Scratch repair, dust removal, color fading
- [ ] **Face enhancement** - GFPGAN integration for portrait content
- [ ] **Specialized models** - Content-specific models (sports, archival, etc.)

### Professional Features
- [ ] **Batch enhancement queue** - Process multiple videos with enhancement pipeline
- [ ] **Hardware optimization** - Multi-GPU support, memory management
- [ ] **Export system** - Professional format support (ProRes, DNxHD, etc.)
- [ ] **Plugin architecture** - Extensible system for community contributions

### Integration Improvements
- [ ] **Module communication** - Seamless data flow between modules
- [ ] **Unified settings** - Shared configuration across modules
- [ ] **Performance monitoring** - Resource usage tracking and optimization
- [ ] **Cross-platform testing** - Linux and Windows parity

### Queue Module UI Polish
- [x] **Button styling** - Queue action buttons have proper Importance settings (Medium, Low, Danger)
- [x] **Thumbnail preview** - 90px thumbnails with 3px module-color outline
- [x] **Module colors** - `ModuleColor()` now exactly matches main menu (all 13 modules)
- [x] **Layout** - TintedBar header, 48px bottom bar, status badge
- [ ] **Visual feedback** - Improve button states and hover effects in queue view

## Technical Debt Addressed

### Player Architecture
- [X] Identified root causes of instability
- [X] Planned Go-based unified solution
- [X] Hardware acceleration strategy defined
- [X] Frame-accurate seeking approach designed

### Enhancement Strategy
- [X] Open-source model ecosystem researched
- [X] Scalable architecture designed
- [X] Content-aware processing planned
- [X] Future-proof model integration system

## Notes

- **Player stability is BLOCKER**: Cannot proceed with enhancement features until player is stable
- **Go implementation preferred**: Maintains single codebase, excellent testing ecosystem
- **Open-source focus**: No commercial dependencies, community-driven model ecosystem
- **Modular design**: Each enhancement system can be developed and tested independently

