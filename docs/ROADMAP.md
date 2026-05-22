# VideoTools Roadmap

A lightweight forward look. Updated at the start of each dev cycle.

**Interactive board:** open [`roadmap.html`](roadmap.html) in your browser — column-per-module, colour-coded cards, click any card for full details (key files, dependencies, related docs).

```mermaid
timeline
    title VideoTools Development Roadmap
    v0.1.1-dev47 (Shipped) : DLL/ folder rename : Flat exe-dir DLL fallback : Disc info at rip view top : UDF ReadFileData (ISO)
    v0.1.1-dev47 (Shipped) : Progress bar with ETA : ConsoleBox widget : Log refactor (Burn/Rip/Author) : PAL/NTSC full-disc convert
    v0.1.1-dev48 (Shipped) : Theme system (internal/theme/) : PillButton + PillIconButton : Transport controls migrated : Text primitives
    v0.1.1-dev48 (Shipped) : Startup crash diagnostics : i18n script persistence : Windows signing wired : Roadmap visual polish
    v0.1.1-dev48 (Shipped) : Full module button+slider migration : STATUS_STACK_OVERFLOW recovery : Dual before/after player sync : Button straggler clean-up
    v0.1.1-dev49 (Current) : Rip module menu bleed fix : Chapter diagnostics : Menu preservation option : Main/extra title naming
    v0.1.1-dev49 (Current) : Inuktitut transliteration package : Auto-fill i18n script variants : VT Media Engine refactor : VT ISO Engine refactor
    v0.1.1-dev49 (Current) : engine.go subsystem split (done) : Frame pacing PTS-driven overhaul : UDF reader robustness
    v0.1.1-dev49 (Current) : WaitForPTS for no-audio path : WaitVsync removed from playbackLoop : Frame rate propagation on load
    v0.1.1-dev49 (Current) : Rip module layout aligned to Convert style : buildRipBox sections : collapsible log : Open in Player to footer
    v0.1.1-dev49 (Current) : Rip source rework : disc info in Source section : single ... browse button : format validation
    v0.1.1-dev49 (Current) : Player idle aspect ratio setting : 4:3/16:9/5:3/21:9/9:16 SMPTE bars
    v0.1.1-dev49 (Current) : C disc debug utility (hex dump, dir listing, stat)
    v0.1.1-dev49 (Current) : Seek corruption fix (accurate fallback) : Player singleton consolidation (10→2) : Verbose seek logging : Media Engine architecture doc
    v0.1.1-dev49 (Current) : NoInheritHandles Windows subprocess (file-in-use fix) : Queue.Stop cancels running job (zombie FFmpeg fix)
    v0.1.1-dev49 (Current) : Windows Job Object KILL_ON_JOB_CLOSE (crash-safe FFmpeg cleanup) : Linux Pdeathsig SIGKILL on all subprocesses
    Next Up : Burn multi-drive batch : IMAPI2 COM replacement : Main Menu refactor : Linux CI speedup
    Player-Dependent : Trim module (frame-accurate cutting) : Enhancement module (AI models)
    Future : DVD menu playback : Video cropping tool : Professional workflow
```

## Legend

| Colour | Meaning |
|--------|---------|
| Blue | Shipped in dev47 |
| Teal | Shipped in dev48 |
| **Green** | **Current dev49 work** |
| Yellow | Next up (handoff priorities) |
| Orange | Blocked on player completion |
| Red | Future / deferred |

> **Status distinction:** The interactive board (`roadmap.html`) uses 5 statuses:
> `Shipped` → `Done (Untested)` → `In Progress` → `Planned` → `Deferred`.
> "Done" items are complete and committed but not yet verified by a tester.

## Current State (v0.1.1-dev49)

- Core modules shipped: Convert, Merge, Filters, Audio, Thumb, Inspect, Compare, Rip, Author, Burn, Queue, Settings, Subtitles, Upscale, Enhancement (placeholder).
- Native Go DVD authoring engine with full M1-M7 menu system.
- Native media player: CGo/FFmpeg engine, InlineVideoPlayer API layer, D3D11VA, audio sync, thread-safe.
- Disc ripping: IFO scanning, ISO via UDF reader, region detection, progress with ETA.
- Theme system, PillButton/PillIconButton, text primitives, VTTheme — all button+slider migrations shipped in dev48.
- STATUS_STACK_OVERFLOW recovery, dual before/after player sync shipped in dev48.
- **dev49 focuses on VT Media Engine and VT ISO Engine production-readiness + Rip module fixes + Inuktitut transliteration + player QoL** — breaking monoliths into subsystem files, fixing menu VOB bleed and chapter embedding, adding menu preservation and main/extra title naming, auto-filling empty i18n script variants via iutools transliteration, configurable idle aspect ratio for SMPTE bars, C disc debug utility, seek corruption fix, player singleton consolidation, Media Engine architecture doc.
- PAL/NTSC full-disc conversion with IFO regeneration.
- Localization: en-CA, fr-CA, Inuktitut (syllabics + Latin, machine-translated).
- CI green on Linux + Windows with from-source FFmpeg static builds.

## Now (dev49 focus)

- **Rip module menu VOB bleed fixed** — `CollectVOBSets` excludes `VTS_XX_0.VOB` from content sets; chapter timestamps no longer shifted by menu duration.
- **Rip chapter diagnostics** — Verbose logging of chapter count, timestamps, and embedding decisions.
- **Rip menu preservation** — New `IncludeMenus` config + checkbox; menu VOBs export as separate files.
- **Rip main/extra naming** — Main feature (longest title) gets main output path; extras get `_Extra_Title_NN` suffix.
- **Rip disc info in Source section** — `discInfoLabel` (type/region/size) moved from Format box to Source section.
- **Rip single browse button** — ISO... and Folder... replaced with `...` opening 900×640 dialog.
- **Rip format validation** — Non-ISO/VIDEO_TS paths rejected with user-visible error.
- **Inuktitut transliteration** — `internal/i18n/translit/` package with iutools algorithm for syllabics↔roman conversion; auto-fills empty i18n fields via `translitFill()` in `SetLanguageWithScript`.
- **VT Media Engine engine.go split** — Break 3245-line Engine monolith: hwdecode.go, playback.go, errors.go, framepool.go, subtitle_engine.go, buffer.go.
- **VT ISO Engine UDF reader robustness** — Fallback AVDP scanning, format validation, multi-extent files, ISO 9660 bridge.
- **Player idle aspect ratio setting** — Configurable SMPTE bars ratio (4:3/16:9/5:3/21:9/9:16) in Settings → Preferences.
- **C disc debug utility** — `DiscDebugHexDump`, `DiscDebugListDir`, `DiscDebugDirStat` for low-level IFO and directory probing.
- **Seek corruption fix** — Accurate fallback seeks with `AVSEEK_FLAG_BACKWARD` to land at keyframe before target, preventing `avcodec_flush_buffers` from destroying unrecoverable decoder state.
- **Player singleton consolidation** — 10 per-module `InlineVideoPlayer` singletons consolidated into 2 shared instances (`GetPrimaryPlayer` / `GetPreviewPlayer`), eliminating per-module player state fragmentation.
- **Verbose seek logging** — Human-readable seek flags, clock reset with audio offset, frame queue drain count, seekGen change tracking with first frame diagnostics.
- **Media Engine Architecture document** — `docs/MEDIA_ENGINE_ARCHITECTURE.md` with full stack, issues, consolidation plan.
- **Log session rotation** — `rotateLog()` keeps last 2 sessions on startup; stale binary noise disappears automatically after two runs.
- **Settings log management** — Clear Log File + Open Log Folder in Settings → Preferences.
- **`NoInheritHandles` Windows subprocess fix** — `CreateCommand`/`CreateCommandRaw` in `internal/utils/exec_windows.go` now set `NoInheritHandles: true`, preventing the player engine's open `avformat_open_input` handles from being inherited by FFmpeg subprocesses. Fixes the "file in use" error when trying to delete or move the source video after conversion on Windows.
- **`Queue.Stop()` cancels running job** — `Stop()` now calls `cancelRunningLocked()`, propagating context cancellation to the running FFmpeg subprocess via `exec.CommandContext`. Fixes zombie FFmpeg on clean VT shutdown.
- **Windows Job Object — crash-safe child process cleanup** — `utils.InitJobObject()` (called at app startup) creates a global Job Object with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`. All long-running encode processes are assigned via `utils.StartCmd()`. When VT exits for any reason — including crashes during 4K encodes — Windows automatically kills all Job Object members. Covers the crash scenario where `Queue.Stop()` is never reached.
- **Linux `Pdeathsig: SIGKILL`** — `exec_linux.go` now sets `SysProcAttr.Pdeathsig = syscall.SIGKILL` on all `CreateCommand`/`CreateCommandRaw` calls. Kernel sends SIGKILL to all child processes when VT exits for any reason.
- **Queue convert navigation fix** — `convertNow()` now calls `s.showQueue()` directly instead of a blocking `dialog.ShowInformation`. User lands on the queue immediately after queuing a job.
- **Queue auto-refresh goroutine fix** — `startQueueAutoRefresh` and `startQueueElapsedTicker` changed `return` to `continue` in the ticker guard; goroutines stay alive between queue visits, so progress bars update correctly.
- **view.go component split** — Break 1438-line VideoPlayer widget: control_overlay.go, keyboard_shortcuts.go, thumbnail_preview.go.
- **Player interface** — Extract formal Go `Player` interface from `InlineVideoPlayer` for mock testing.
- **HW decode default-on** — Re-evaluate D3D11VA default with VEH/SEH bridge coverage; add per-codec HW blacklist.
- **Thread safety formalisation** — Document lock hierarchy, add lockdep assertions, eliminate reverse-order paths.
- **UDF thread safety & progress** — Mutex-guarded Reader, extraction progress callbacks, temp file cleanup.

## Remaining dev49 work

- **Burn multi-drive batch** — Queue multiple ISOs across available burners.
  See `docs/BURN_MODULE_DESIGN.md` §Phase 2.

- **IMAPI2 COM** — Replace isoburn.exe on Windows for proper progress callbacks.
  See `docs/BURN_MODULE_DESIGN.md` §Phase 3.

- **Main Menu refactor** — Extract `showMainMenu()` from root `mainmenu_module.go` into `internal/app/modules/mainmenu/`.

- **Linux CI speedup** — Pre-built container image for FFmpeg build dependencies.

## Next

- **Enhancement module** — DEPENDS ON PLAYER
  - Open-source AI model integration (BasicVSR, RIFE, RealCUGan)
  - Model registry for easy addition
  - Content-aware model selection

- **Trim module** — DEPENDS ON PLAYER
  - Frame-accurate trimming and cutting
  - Visual timeline with chapter markers
  - Preview-based frame selection

- **Professional workflow**
  - Seamless module chaining (Player ↔ Enhancement ↔ Trim)
  - Batch processing through queue
  - Hardware-accelerated enhancement pipeline

## Localization

See `docs/localization-policy.md` for the full policy.

- en-CA and fr-CA maintained and complete.
- Inuktitut (syllabics + Latin) machine-generated, needs human review.
- All user-facing strings use `i18n.T().KeyName`.

## Versioning

Continuous global `dev` counter, not reset per public version.

Examples:
- `v0.1.1-dev55`
- `v0.1.4-dev72`

Public releases use the base version only (e.g. `v0.1.2`).

## Public Version Bump Policy

Minimum gate for `v0.1.1-devN` → `v0.1.2`:
- Windows and Linux package workflows green on release candidate.
- Full module smoke test pass per `docs/TESTING_MODULE_CHECKLIST.md`.
- No known P0/P1 regressions in conversion, queue, or subtitle sync.
- Changelog complete and matches release scope.
- Deferred items documented in `TODO.md` with explicit carry-over.
