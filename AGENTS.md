# VideoTools Agent Workflow Rules

These rules apply to any automation or agent working in this repo.

## Current Project State

- Current cycle: `v0.1.1-dev50` — **Media Engine gap closure complete. All 11 Phase 1 items shipped. HW decode default-on + error concealment shipped. Playlist, HDR tone-mapping, UDF reader fixes, collapsible player panel (all five player modules: Convert/Filters/Upscale/Inspect/Trim), logging improvements, CI DLL cache hardening, and updater sidecar refresh all shipped.** Priorities below.
- Public/stable baseline: `v0.1.1`.
- `dev49` shipped: engine.go subsystem split completed (3245→1117 lines; errors.go, hwdecode.go, framepool.go, subtitle_engine.go, buffer.go, playback.go extracted). Frame pacing overhaul: no-audio path uses WaitForPTS; WaitVsync removed from playbackLoop. Player default aspect ratio setting (4:3/16:9/5:3/21:9/9:16 idle SMPTE bars). Seek corruption fix: accurate fallback uses AVSEEK_FLAG_BACKWARD. Player singleton consolidation: 10 per-module singletons → 2 shared instances (GetPrimaryPlayer/GetPreviewPlayer). Engine-level bwdif deinterlace (libavfilter, Settings toggle default on). Thread safety formalisation (mu→formatMu→videoCodecMu→framepoolMu hierarchy, named helpers, lockdep build-tag). Rip module: menu VOB bleed fixed, chapter diagnostics, menu preservation, main/extra title naming, disc info to Source section, single browse button, format validation. C disc debug utility. Inuktitut transliteration package (internal/i18n/translit/) with iutools algorithm. Rip/Convert layout: buildRipBox sections, BuildCollapsibleHeader component, collapsible metadata/settings/log panels. Queue: blocking dialog removed, goroutine self-exit fixed. Log session rotation + Clear/Open in Settings. Dropdown active item text: ForegroundOnPrimary on VT_Green. Process management: NoInheritHandles (file-in-use fix), Queue.Stop() cancellation (zombie fix), Windows Job Object KILL_ON_JOB_CLOSE, Linux Pdeathsig:SIGKILL.
- `dev48` shipped: internal/theme/ package with VT_Navy palette, PillButton/PillIconButton widgets, text primitives. All module-level widget.Button calls migrated to MakePillButton/MakePillIconButton. VTSlider/VTProgressBar replace widget.Slider sitewide. STATUS_STACK_OVERFLOW caught by VEH in safe_bridge.c + 4 MB PE thread stack. Dual before/after player sync (SetPeer). Audio nil-widget crash fixed. Window recentering removed. Inuktitut script preference persists. Windows SignPath signing wired. VT_STARTUP_DEBUG crash diagnostics. CI Windows FFmpeg shared cache. Button straggler clean-up (about, compare, settings tabs, command_editor).
- `dev47` closed. Rip: disc info display (type/region/size) at top of view, UDF ReadFileData for ISO region detection, progress bar with ETA, flat exe-dir DLL fallback, DLL/ folder rename (was ffmpeg-dll/), log boxes at bottom, Burn ConsoleBox, Author log truncation removed, Settings Module Chaining section, CI Linux FFmpeg build fixes. Audio Phase 1-3 fully shipped.
- `dev46` closed. PAL→NTSC full-disc conversion pipeline with IFO regeneration, Upscale preset overhaul, Audio Phase 2 (InlineVideoPlayer) + Phase 3 (track selection).
- `dev45` closed. Convert Phase 1+2 (SR, Normalize, Deinterlace, H.264 Profile/Level, presets, AVI/TS/FLV), Convert i18n, Module Pipeline (&&), logging audit, FFmpeg DLL bootstrap.
- `dev44` closed. Player thread-safety audit, D3D11VA, audio sync, GStreamer removal, Queue UI polish, thumbnail improvements, flags/i18n.
- `dev43` closed. Player thread-safety: pixel format SIGSEGV fix, Close/demuxer WaitGroup, NextFrame/Close codec race, seekLoop goroutine leak.
- `dev42` closed. Player stabilisation (D3D11VA, audio sync, GStreamer removal), thumbnail 3-way output, audio i18n.
- Issue tracker active at `https://git.leaktechnologies.dev/leak_technologies/VideoTools/issues`.
- Primary planning source is `TODO.md`; shipped scope is tracked in `DONE.md`; release-facing history is `docs/CHANGELOG.md`.
- **Player debug log:** `docs/PLAYER_DEBUG.md` — keep this up to date as issues are found and fixed. Update it before closing any player-related issue.

## Immediate Handoff Priorities

### Rip Module Fixes
All items in `internal/app/modules/rip/`.

| Task | File(s) | Status |
|------|---------|--------|
| Menu VOB bleed (VTS_XX_0.VOB excluded from content sets) | `executor.go` | **SHIPPED** |
| Chapter embedding diagnostics | `executor.go` | **SHIPPED** |
| Menu preservation option (separate file export) | `executor.go`, `types.go`, `view.go`, `modulecfg/rip.go`, `rip_module.go` | **SHIPPED** |
| Main/extra title naming (main path + _Extra suffix) | `view.go` | **SHIPPED** |
| Disc info moved to Source section | `view.go` | **SHIPPED** |
| Single ... browse button (replaces ISO... + Folder...) | `view.go` | **SHIPPED** |
| Format validation (reject non-disc sources) | `view.go` | **SHIPPED** |

### VT Media Engine Refactoring (HIGH — start here)

All items in `internal/media/` and `internal/ui/inline_player.go`.

| Task | File(s) | Status |
|------|---------|--------|
| Split 3245-line engine.go into subsystem files (hwdecode.go, playback.go, errors.go, framepool.go, subtitle_engine.go, buffer.go) | `internal/media/engine.go` | **SHIPPED** |
| Frame pacing: no-audio WaitForPTS, remove WaitVsync jitter, propagate frame rate | `internal/media/playback.go`, `internal/ui/inline_player.go` | **SHIPPED** |
| Player default aspect ratio setting (idle SMPTE bars) | `internal/media/view.go`, `internal/ui/inline_player.go`, `settings_module.go`, `tabs.go` | **SHIPPED** |
| Seek corruption fix: accurate fallback uses AVSEEK_FLAG_BACKWARD | `internal/media/playback.go` | **SHIPPED** |
| Verbose seek logging (flags, clock reset, frame queue drain, seekGen change) | `internal/media/playback.go`, `internal/ui/inline_player.go` | **SHIPPED** |
| Player singleton consolidation: 10 → 2 (GetPrimaryPlayer/GetPreviewPlayer) | `native_media.go`, `main.go` | **SHIPPED** |
| Media Engine Architecture document | `docs/MEDIA_ENGINE_ARCHITECTURE.md` | **SHIPPED** |
| **P0-1: Wire DegradeToSoftware() into decode paths** — dead code, never called | `internal/media/errors.go`, `internal/media/playback.go`, `internal/media/hwdecode.go` | **SHIPPED** |
| **P0-2: Fix NextFrame hang after videoDecodeDead** — no EOF sentinel, blocks forever | `internal/media/playback.go` | **SHIPPED** |
| **P0-3: Fix backward frame stepping** — Step() rejects negative `frames` | `internal/media/playback.go` | **SHIPPED** |
| **P0-4: Replace lastError with error ring buffer** — single slot, never read | `internal/media/errors.go`, `internal/media/engine.go` | **SHIPPED** |
| **P0-5: Add OpenAuto() with Open→OpenDVD fallback** | `internal/media/engine.go`, `internal/ui/inline_player.go` | **SHIPPED** |
| **P1-1: Network/URL streaming** — AVDictionary options, OpenURL, LoadURL | `internal/media/engine.go`, `internal/ui/inline_player.go` | **SHIPPED** |
| **P1-2: Resume/watch-later outside Trim module** | `internal/media/state/resume.go`, `internal/ui/inline_player.go` | **SHIPPED** |
| **P1-3: Audio delay adjustment** — no lip-sync correction exists | `internal/media/engine.go`, `internal/media/playback.go`, `internal/ui/inline_player.go` | **SHIPPED** |
| **P1-6: SeekAccuracy Settings UI** — locked to Keyframe, Frame/Accurate unreachable | `internal/app/modules/settings/tabs.go`, `types.go` | **SHIPPED** |
| **P1-9: Player Tuning Settings UI** — HW toggle, buffer size, thread count, etc. | `internal/app/modules/settings/tabs.go`, `types.go` | **SHIPPED** |
| Split 1438-line view.go (VideoPlayer widget) into components | `internal/media/view.go` | Planned (deferred) |
| Extract formal `Player` interface from `InlineVideoPlayer` for mock testing | `internal/ui/inline_player.go` | Planned (deferred) |
| Re-evaluate HW decode default-on with VEH/SEH bridge coverage | `internal/media/engine.go`, `internal/media/safe_bridge.c` | **SHIPPED** |
| **Error concealment (last-good-frame)** — `lastGoodFrame atomic.Pointer[image.RGBA]` + `decodeErrored atomic.Bool`; `NextFrame` returns frozen frame once on decode-error EOF | `internal/media/engine.go`, `internal/media/playback.go` | **SHIPPED** |
| **Playlist / sequential play** — `Enqueue(path)` / `ClearPlaylist()` / `PlaylistLen()`; `playbackLoop` auto-advances on clean EOF; direct `Load`/`LoadDVD`/`LoadURL` resets queue | `internal/ui/inline_player.go` | **SHIPPED** |
| **HDR tone-mapping** — `hdr.go`: `isFrameHDR` (PQ/HLG TRC detection), libavfilter graph `zscale(t=linear,npl=1000)→tonemap(hable)→zscale(t=bt709)→yuv420p`; `renderSWFrame()` centralises SW→RGBA with HDR+deinterlace; `hdrTonemapUnsupported` flag; `freeHDRFilter()` in `Close()` | `internal/media/hdr.go`, `internal/media/playback.go`, `internal/media/engine.go` | **SHIPPED** |
| **Collapsible player panel in Convert, Filters, Upscale, Inspect, Trim** — `BuildCollapsibleHeader(t.ConvertSectionPlayer, ...)` wraps video area in all five modules; Inspect uses `HSplit`; Trim uses `VSplit` so timeline stays visible | `main.go`, `filters/view.go`, `upscale/view.go`, `inspect/view.go`, `trim/view.go` | **SHIPPED** |
| **Logging: Windows Clear fix + version header** — `Clear()` uses close→O_TRUNC→O_APPEND pattern; `SetVersion(v)`+`sessionHeader()` embed version string in all session headers | `internal/logging/logging.go`, `main.go` | **SHIPPED** |
| **Updater sidecar refresh** — `updateSidecars()` extracts `DLL/*.dll`, `ffmpeg.exe`, `ffprobe.exe` from update zip alongside `VideoTools.exe` | `settings_module.go` | **SHIPPED** |
| **CI stale DLL cache detection** — skip-download now checks `liblzma-5.dll`; auto-wipes stale cache dirs if missing | `.forgejo/workflows/dev-packages.yml` | **SHIPPED** |
| **Per-codec HW deny-list** — `hwCodecDenyList` + `SetHWCodecDenyList(s)`; `PrefsConfig.HWCodecDenyList`; Settings → Player text entry; loaded at startup | `internal/media/hwdecode.go`, `settings/types.go`, `settings/tabs.go`, `settings_module.go`, `native_media.go` | **SHIPPED** |
| **Error resilience flags** — `setVideoCodecErrorFlags()`: `error_concealment = FF_EC_GUESS_MVS | FF_EC_DEBLOCK` explicit before `avcodec_open2` on both video codec init paths | `internal/media/engine.go` | **SHIPPED** |
| **Mid-playback audio track switching** — `SelectAudioTrack`: close old player first (use-after-free fix), reinit codec, seek to current PTS, resume if playing | `internal/media/engine.go` | **SHIPPED** |
| **Mid-playback subtitle track switching** — `SelectSubtitleTrack`: flush queue, reinit codec, clear overlay; `subtitleCodecMu` added for thread safety | `internal/media/engine.go`, `internal/media/subtitle_engine.go`, `internal/media/playback.go` | **SHIPPED** |
| Thread safety formalisation (lock hierarchy, lockdep, named helpers) | `internal/media/engine.go` | **SHIPPED** |
| **P1-4: Speed + pitch correction** — `AudioFilterGraph.Process()` C helper + `AudioPlayer.filterGraph` lazy-init | `internal/media/audio_filter.go`, `internal/media/audio.go` | **SHIPPED** |
| **P1-5: A-B loop** — SetLoopPoints, SetABLoopEnabled, NextFrame seek-back | `internal/media/playback.go`, `internal/media/engine.go`, `internal/ui/inline_player.go` | **SHIPPED** |
| **P1-8: Frame timing overlay** — per-frame PTS/delta/gap overlay, SetFrameTimingOverlayVisible toggle | `internal/media/view.go`, `internal/ui/inline_player.go` | **SHIPPED** |
| **P1-11: Clock drift correction** — SetTime underrun recovery | `internal/media/clock.go` | **SHIPPED** |

### VT ISO Engine Refactoring (HIGH)

All items in `internal/dvd/udf/` and `internal/app/modules/rip/`.

| Task | File(s) | Status |
|------|---------|--------|
| UDF reader robustness: ShortAd allocation descriptor parsing, partition offset on all LBNs, extractFile/ReadFileData from ICB InformationLength | `internal/dvd/udf/reader.go` | **SHIPPED** |
| Thread safety & progress: mutex-guarded Reader, progress callbacks, temp file cleanup | `internal/dvd/udf/reader.go`, `internal/app/modules/rip/iso_udf.go` | Planned |
| C disc debug utility (hex dump, dir listing, stat) | `internal/media/disc_debug.{c,h,go}` | **SHIPPED** |
| `NoInheritHandles` on Windows subprocess creation — fixes "file in use" after conversion | `internal/utils/exec_windows.go` | **SHIPPED** |
| `Queue.Stop()` cancels running job — fixes zombie FFmpeg on clean VT shutdown | `internal/queue/queue.go` | **SHIPPED** |
| Windows Job Object (`KILL_ON_JOB_CLOSE`) — crash-safe child process cleanup; `utils.StartCmd()` at all encode sites | `internal/utils/jobobject_windows.go`, `main.go`, `rip/executor.go`, `audio/executor.go`, `thumbnail/generator.go`, `interlace/detector.go` | **SHIPPED** |
| Linux `Pdeathsig:SIGKILL` on all subprocess creation — kernel-level crash-safe cleanup | `internal/utils/exec_linux.go` | **SHIPPED** |
| Dropdown active item text colour — ForegroundOnPrimary (dark) on VT_Green focus background | `_fyne/widget/menu_item.go` | **SHIPPED** |

### Button Stragglers (carry-forward from dev48)

All major module migrations are done. Three stragglers remain blocked and carry forward until their dependencies are resolved:
**Do not touch `internal/media/gpu/` or any file under `cmd/`, `qr-demo/`, or `scripts/legacy/`.**

| File | Count | Status |
|------|-------|--------|
| `convert_player_native.go` | 11 `NewButtonWithIcon` transport icons | **BLOCKED** — dynamic icon switching (play↔pause) not supported by PillIconButton |
| `main.go` (lines ~13617–13743, non-native player transport) | 7 `NewButtonWithIcon` | **BLOCKED** — same dynamic icon switching issue |
| `internal/utils/utils.go` | 1 `NewButton` (MakeIconButton) | **BLOCKED** — import cycle: `utils` → `ui` → `benchmark` → `utils` |

### Dev49 Handoff (carry forward)

- **GitHub mirror setup** — Push mirror via Forgejo built-in; issue migration script at `scripts/github-mirror/migrate-issues.ps1`. GitHub Actions release workflow at `.github/workflows/release.yml` (Windows ZIP + DLLs, Linux tarball, auto-release on `v*` tags). See `scripts/github-mirror/README.md`.
- **Burn multi-drive batch** — Queue multiple ISOs across available burners. See `docs/BURN_MODULE_DESIGN.md` §Phase 2.
- **IMAPI2 COM replacement** — Replace isoburn.exe on Windows for proper progress/control. See `docs/BURN_MODULE_DESIGN.md` §Phase 3.
- **Main Menu refactor** — *(LOW PRIORITY — deferred until engine is stable)* Extract `showMainMenu()` from root `mainmenu_module.go` into `internal/app/modules/mainmenu/`.
- **Linux CI speedup** — Pre-built container image for FFmpeg build dependencies.

### Recently Shipped (dev49)
- **Engine-level bwdif deinterlace** — `internal/media/deinterlace.go` with libavfilter filter graph. Applied in `videoDecodeLoop` / `GrabFrame` when `AV_FRAME_FLAG_INTERLACED` set. `bwdif=mode=0:parity=-1:deint=0` auto field-order detection, full-rate output, flagged frames only. `toRGBA(src *C.AVFrame)` signature extended for direct filtered frame conversion. Settings toggle in Player section (default on), persisted in `PrefsConfig.AutoDeinterlace`. i18n keys in all four locales.
- **disc_debug.c double-include fix** — Removed `#include "disc_debug.c"` from `disc_debug.go` C preamble (CGo auto-compiles `.c` files; the `#include` caused duplicate symbols at link time). Added `#include <stdlib.h>` for `C.free`.

### Recently Shipped (dev48)
- **internal/theme/ package** — VT_Navy colour palette, PillButton, PillIconButton, text primitives (TitleLabel, SectionLabel, WrappingLabel, HintLabel, MonoLabel) extracted to `internal/theme/`. Both `ui/` and `media/` import from theme — no circular dependency. `ui/` re-exports for backward compat.
- **PillButton / PillIconButton** — pill-shaped and icon-only pill buttons. Wired into all modules and transport controls.
- **Text primitives** — `NewTitleLabel`, `NewSectionLabel`, `NewWrappingLabel`, `NewHintLabel`, `NewMonoLabel`.
- **Full module button migration** — All compare/audio/rip/filters/upscale/subtitles/trim/thumbnail/queueview/settings/benchmarkview/main.go module-level `widget.Button` calls migrated to `MakePillButton`/`MakePillIconButton`. `MakePillButton` renamed from `NewPillButton`; `MakePillIconButton` from `NewPillIconButton`.
- **VTSlider / VTProgressBar** — `widget.Slider` and `widget.ProgressBar` replaced sitewide with styled `ui.Slider`/`ui.MakeSlider`.
- **Queue + Benchmark header/footer alignment** — Both modules now use `TintedBar`, `NewTitleLabel`, `accentColor`, and the shared statsBar footer.
- **STATUS_STACK_OVERFLOW recovery** — MinGW VEH in `safe_bridge.c` now catches `STATUS_STACK_OVERFLOW` (0xC00000FD). `_resetstkoflw()` restores the guard page before `longjmp`. New `SAFE_BRIDGE_STACK_OVERFLOW` (0xDEAD0003) sentinel. PE default thread stack raised to 4 MB via `CGO_LDFLAGS_ALLOW=-Wl,--stack,.*` in CI and `build.ps1`.
- **Dual before/after player sync** — `InlineVideoPlayer.SetPeer()` designates a follower that mirrors every Play/Pause/Seek. Filters and Upscale before/after players now play simultaneously. Preview player is muted and has built-in controls disabled.
- **Audio nil-widget crash** — Guard against `Player.Widget()` returning nil.
- **Window recentering removed** — `CenterOnScreen()` removed from `maximizeWindow`; window stays where user placed it.
- **i18n script persistence** — Inuktitut syllabics/Latin preference survives app restarts via `ScriptPrefs` map in locale JSON.
- **Windows SignPath signing** — `SIGNPATH_API_TOKEN` + `SIGNPATH_ORGANIZATION_ID` both set in Forgejo secrets. ci-build.ps1 calls sign-exe.ps1 on every Windows build (non-fatal).
- **VT_STARTUP_DEBUG** — crash diagnostics env var traces widget CreateRenderer to stderr. Confirmed: STATUS_STACK_OVERFLOW is glfw.CreateWindow() GPU driver DLL injection, not VT code.
- **Windows CI FFmpeg shared cache** — `actions/cache` for `C:\ffmpeg-static`, matching the Linux cache. Both platforms skip the FFmpeg source build on cache hit.
- **Button straggler clean-up** — About dialog (2), compare fullscreen (2), settings tabs (2), command_editor (7) all migrated. utils.MakeIconButton return type fixed. 18 transport icon buttons remain (blocked: PillIconButton needs SetIcon).

## Commit Discipline

- **ALWAYS stage and commit after every change. Do not wait for permission.**
- `git add -A` then `git commit -m "..."`.
- Do not leave unstaged changes in the worktree.
- Commit only files related to the current task.

## Documentation Discipline

Every time work lands, **all six documents must be updated in the same commit**:

| Document | What to update |
|---|---|
| `docs/roadmap.html` | Card status, cycle, desc; add new cards; update changelog/checklist data |
| `docs/ROADMAP.md` | Mermaid timeline entry; Current State prose; Now/Next sections |
| `docs/CHANGELOG.md` | New bullet under current dev cycle heading |
| `AGENTS.md` | Settled decisions, handoff priorities, current cycle state |
| `DONE.md` | New section entry for the completed feature |
| `TODO.md` | Check off completed items; add newly scoped items |

Additional rules:
- If behavior changes, also update `docs/INSTALLATION.md` and the relevant platform guide (`docs/INSTALL_WINDOWS.md`, `docs/INSTALL_LINUX.md`).
- `CHANGELOG.md` means `docs/CHANGELOG.md` in this repo (not the root).
- Avoid personal names; use `user report` or `dev report` only.
- The retired `docs.leaktechnologies.dev` site must not be used; active docs live in-repo and on the Forgejo wiki.

## Interactive Roadmap Maintenance (`docs/roadmap.html`)

The interactive roadmap is the **primary visual tracker** for the project. It must be kept current at all times. Both Claude and opencode are responsible for updating it whenever features land or scope changes.

### Card Status Values

| Value | Meaning |
|---|---|
| `shipped` | Merged, tested, confirmed working by a real user or tester |
| `done` | Code committed and CI green, but not yet tester-verified |
| `active` | Currently being worked on this cycle |
| `planned` | Scoped for an upcoming cycle — implementation not started |
| `future` | Deferred; no cycle assigned yet |

### What to Update When a Feature Lands

1. **Find the card** — search the `roadmap` JS object by `id` or `title`.
2. **Update `status`** — set to `done` when committed, `shipped` once tester confirms.
3. **Update `cycle`** — set to the current dev cycle (e.g. `'dev48'`).
4. **Update `desc`** — if the implementation detail differs from what was planned, rewrite it to match reality.
5. **Update `files`** — add or correct the key file paths.

### What to Update When New Work Is Scoped

Add a new card object to the correct module's `items` array:

```javascript
{ id: 'unique-kebab-id', title: 'Feature Name',
  desc: 'What it does and why it matters — one or two sentences.',
  status: 'planned', cycle: 'dev49',
  files: ['internal/path/to/relevant.go'],
  deps: ['id-of-dependency-card'],
  docs: ['docs/DESIGN_DOC.md'] },
```

### What to Update When a Dev Cycle Closes

1. Add a new entry to the **`changelogData` array** at the top (newest first) with the cycle's sections and bullet points. Use the actual technical detail from `docs/CHANGELOG.md` — not just card titles.
2. Add the cycle's items to the **`checklistData` array** so testers can verify them.
3. Update the **subtitle line** (`<p class="subtitle">`) with the new version and date.

### What NOT to Do

- Do not leave a card at `status: 'planned'` after the code is committed.
- Do not leave a card at `status: 'done'` indefinitely — chase the tester sign-off and move it to `shipped`.
- Do not add a card for something that is already shipped and was never tracked — just note it in `CHANGELOG.md` instead.
- Do not edit roadmap.html in isolation — always sync `ROADMAP.md`, `CHANGELOG.md`, `DONE.md`, `TODO.md` in the same commit.

### New Feature Documentation

- Create `docs/FEATURE_NAME.md` before implementation begins
- Include: overview, features, technical implementation, files to modify, testing checklist
- Link design doc in TODO.md and AGENTS.md immediate priorities
- Example: `docs/BURN_MODULE_DESIGN.md`, `docs/AUTO_GREY_CODECS.md`

## Internationalization (i18n)

All user-facing strings MUST use the i18n system. Never hardcode display strings.

### Rules

1. **All UI strings** must use `i18n.T().KeyName` — never use `"hardcoded string"`
2. **Add new strings** to `internal/i18n/strings.go` first (defines the key)
3. **Add English translation** to `internal/i18n/en_ca.go` (source of truth)
4. **Add French translation** to `internal/i18n/fr_ca.go`
5. **Add Inuktitut placeholders** to `internal/i18n/iu.go` (syllabics) and `internal/i18n/iu_latin.go` (Latin). Machine-generated translations are acceptable but must carry a `// machine-generated, needs human review` comment. See `docs/localization-policy.md` for the full policy.
6. **Module names** already localized: use `t.ModuleXxx` (e.g., `t.ModuleConvert`)
6. **Actions** already localized: use `t.ActionXxx` (e.g., `t.ActionSave`)
7. **Common labels** already localized: use `t.LabelXxx` (e.g., `t.LabelNoFile`)

### Common Patterns

```go
// Bad
label := widget.NewLabel("Click here")

// Good
label := widget.NewLabel(t.ActionClick)  // if action exists
// or add new key to strings.go
```

### Checking for Missing Strings

When adding a new feature, grep for hardcoded strings:
- `widget.NewLabel("...` 
- `dialog.Show...("...`
- Any user-visible text in quotes

### String Key Naming

- `ModuleXxx` — module names (ModuleConvert, ModuleAudio)
- `ActionXxx` — button/actions (ActionSave, ActionCancel)
- `LabelXxx` — static labels (LabelNoFile, LabelOutput)
- `StatusXxx` — status messages (StatusComplete, StatusFailed)
- `DialogXxx` — dialog titles (DialogConfirmDelete)

## Version Bumping

- After every major feature/change: bump the version (main.go, VERSION, FyneApp.toml).
- After bumping: update DONE.md, TODO.md, and CHANGELOG.md.
- Versioning model:
  - `v0.1.1-devN` is the rolling dev/nightly line
  - `v0.1.1` is the public stable baseline
  - dev numbering is continuous across public releases
  - the next public bump is based on release readiness, not number of dev iterations

## Windows Install Flow

- Use `scripts/windows/install.ps1` or `scripts/windows/install.bat` from PowerShell/CMD.
- `scripts/linux/install.sh` is for bash shells only; do not run it from PowerShell.

## Settled Decisions

The following decisions were reached after multiple failed attempts and must not be changed or re-litigated without explicit Human Director approval. Each entry includes the rationale (what was tried and why it failed) to prevent future agents from repeating the same mistakes.

### CI Toolchain — FFmpeg, x264, x265

**FFmpeg must be built from source — never use BtbN pre-built packages.**
BtbN Windows packages contain executables only, no static `.a` libraries.
BtbN Linux packages do not bundle `libx264.a`/`libx265.a`.
Both result in `cannot find -lx264/-lx265` linker errors in the CGO build.
`unzip` is not available in the MSYS2 bash environment — a BtbN `.zip` approach also fails immediately.

**x264 and x265 must be built from source as static-only.**
MSYS2's prebuilt x264/x265 packages install headers with `__declspec(dllimport)`,
causing `libavcodec.a` to reference `__imp_x264_*` symbols that can never be satisfied
in a static link.

**x265.pc must be overwritten after cmake install.**
CMake writes Windows-style paths and CRLF line endings that MSYS2 pkg-config cannot parse.
It also omits the C++ runtime from Libs. The echo-command overwrite (LF, POSIX paths) is required.
C++ deps (`-lstdc++ -lsupc++ -lm` on Windows / `-lstdc++ -lm` on Linux) must go in **`Libs`**
(not `Libs.private`) because FFmpeg configure calls `pkg-config --libs` without `--static`,
so `Libs.private` is never included in the configure link test. On Linux, `-lstdc++` resolves
to libstdc++.so.6 which exports operator new, RTTI vtables etc. On Windows, `-lstdc++` provides
the DLL import stub (for std::__throw_length_error) and `-lsupc++` provides the GCC-private static
archive (for operator new/delete, __cxa_guard, RTTI vtables).

**cmake must be in the Linux apt-get install deps** (for x265 source build).
**nasm and mingw-w64-ucrt-x86_64-cmake must be installed in the Windows MSYS2 step** (for x264/x265 source builds).

If CI is failing, read the build log carefully before changing the FFmpeg setup strategy.
Open an issue or ask before touching the "Setup static FFmpeg" steps.

### Roadmap & Feature Tracking

**Interactive roadmap is the single source of truth.** The roadmap HTML board at `docs/roadmap.html` is the canonical feature tracker. TODO.md and DONE.md are narrative supplements; keep them in sync when landing work, but always update the roadmap first.

**Obsolete output formats (AVI, FLV, 3GP, OGG) are NOT output targets.** These exist only as `Legacy: true` entries in `internal/convert/presets.go` for remuxing legacy files. They have been removed from the roadmap entirely. Do not add them back.

**GStreamer was fully removed in dev42.** All `internal/player/gstreamer*` files, build scripts, and CI references were deleted. The native_media build tag is the only player path. Any stale GStreamer references in docs are historical and should be cleaned up when found.

**Testing checklist lives in the roadmap.** The interactive roadmap at `docs/roadmap.html` includes a Testing Checklist modal (button next to Changelog). Items are grouped by module with pass/fail/untested status persisted to localStorage. Any new feature added to the roadmap must also be added to the testing checklist. The checklist server-side data (`checklistData` array in the roadmap JS) should be updated when features land.

### Dev Cycle Boundary — Do Not List as Future/Planned

The following decisions prevent regression — these items shipped in the indicated cycles and must not reappear on roadmaps or TODO lists:

- **x264/x265 tuning presets** — shipped dev45 (Film, Animation, Grain, Stillimage, Fastdecode)
- **Presets consolidation** — shipped dev45 (format definitions in `internal/convert/presets.go`)
- **Drag-and-drop for Convert** — shipped dev44 (files dropped on module register correctly)
- **Queue notifyChange race fix** — shipped dev45 (goroutine spawning without lock fixed)
- **Audio pre-warm** — shipped dev42 (shared oto/WASAPI context at startup)
- **PAL/NTSC full-disc conversion** — shipped dev47 (IFO regeneration pipeline)
- **Engine-level bwdif deinterlace** — shipped dev49 (libavfilter filter graph in videoDecodeLoop, Settings toggle default on)
- **Thread safety formalisation** — shipped dev49 (lock hierarchy docs, lockdep assertions, all direct lock calls replaced with named helpers)

## Player: SEH/VEH Crash Recovery — SHIPPED dev48

The `internal/media/safe_bridge.c` file provides platform-specific crash recovery around all FFmpeg codec calls:

- **MinGW (Windows)** — `AddVectoredExceptionHandler` + thread-local `setjmp`. Catches `EXCEPTION_ACCESS_VIOLATION` (0xC0000005) and `STATUS_STACK_OVERFLOW` (0xC00000FD). `_resetstkoflw()` restores the guard page before `longjmp` on stack overflow. Sentinels: `SAFE_BRIDGE_ACCESS_VIOLATION` (0xDEAD0002), `SAFE_BRIDGE_STACK_OVERFLOW` (0xDEAD0003).
- **MSVC** — native `__try`/`__except`.
- **Linux/macOS** — `SIGSEGV` signal handler + thread-local `setjmp`.

All four paths (`safe_avcodec_send_packet`, `safe_avcodec_receive_frame`, `safe_av_hwframe_transfer_data`, `safe_sws_scale_frame`) are wrapped. Go callers check the `exc_code_out` sentinel and log diagnostics without crashing.

Do not rewrite or replace this bridge without understanding all four platform paths.

## Coordination

- Ask before changing workflow entrypoints or automation behavior.
- If a change affects installs/builds, add a short note in docs.
- Keep Forgejo release publishing aligned to `VERSION`; do not retarget releases to older dev tags.
- Be careful with tag/release operations — do not retarget or delete existing dev tags.
- Old workflow runs must not be used as evidence of current release state.

## Repository Hygiene

- Keep the repository root minimal. Root should contain only core project manifests, primary app entry source, and top-level docs (`README.md`, `TODO.md`, `DONE.md`).
- Put demos/tools under `cmd/` or `scripts/` (not the root).
- Put platform packaging assets under `packaging/<platform>/`.
- Do not commit ad-hoc logs, scratch files, backup files, or one-off test files to root.

### No New Root-Level Go Files — Hard Rule

**Do not add new `.go` files to the repository root.** The root already has 38 files in `package main`; every new file added there deepens the coupling and makes the eventual `internal/` migration harder.

When adding new functionality:
- Create it in the appropriate `internal/` sub-package from the start.
- If it belongs to an existing module (e.g. convert, player, author), add it to that module's directory under `internal/app/modules/` or create a new one.
- If it is truly app-level wiring (the thin glue between `main` and a module), it may live in a *temporary* root shim file — but that shim must be listed in the refactor plan (`docs/REFACTOR_DEV30_PLAN.md`) as a future extraction target.
- Types, helpers, and utilities that don't depend on `appState` must go in `internal/` immediately — there is no justification for adding them to root.

If you are unsure where new code belongs, default to `internal/` and ask rather than defaulting to root.

## Refactor Boundaries

- Current refactor plan: `docs/REFACTOR_DEV30_PLAN.md`.
- Phase 2 complete. Phase 3 in progress — **opencode owns Phase 3 going forward**.
- Completed Phase 3 slices (do not re-extract these):
  - `internal/app/modules/about` — About dialog
  - `internal/app/modules/deps` — Missing deps dialog
  - `internal/app/modules/mainmenu` — Main menu helpers
  - `internal/app/modules/convert` — `ShowView`, `ConvertState`, `ConvertCallbacks` entry point; full `buildConvertView` deferred (high appState coupling)
  - `player_module.go` — `showPlayerView` + `buildPlayerView`
  - `enhancement_module.go` — `buildEnhancementView` (placeholder)
  - `upscale_module.go` — full upscale view + AI helpers
  - `compare_module.go` — `showCompareView`, `showCompareFullscreen`, `buildCompareView`, `buildCompareFullscreenView`
- `main.go` is now ~16,925 lines. Remaining large blocks:
  - `buildConvertView` (~3,500 lines) — entry point shim exists; full extraction requires appState decoupling first
  - Inspect view (`showInspectView` + `buildInspectView`)
  - Settings view (`showSettingsView` + `buildSettingsView`)
  - Queue view (`showQueue` + related)
  - Remaining `*_module.go` candidates: `filters_module.go`, `subtitles_module.go` (already separate files, no further extraction needed unless moving to `internal/`)
- Pattern for pure-move slices (no `appState` interface change needed):
  1. Create `<name>_module.go` in `package main`
  2. Move `show*` and `build*` functions verbatim
  3. Copy only the imports the moved functions need
  4. Remove the block from `main.go`
  5. Commit as a single focused slice
- Continue using thin `package main` shims while moving logic out of root files.
- Prefer small, reversible refactor slices. Do not combine structural moves with unrelated feature work.
- The long-term goal remains:
  - reduce root-level `package main` clutter
  - move app logic into `internal/app/`
  - move the executable entrypoint toward `cmd/videotools/`

## Native Media Player

Architecture reference: `docs/NATIVE_PLAYER.md` — read this before touching any player code.

### The One Rule

**All modules must use `ui.InlineVideoPlayer` as their API layer.**

Do not call `media.NewEngine()` inside a module. Do not write a per-module playback goroutine. Do not call `media.NewVideoPlayer()` directly. Get the widget via `opts.Player.Widget()`. The only exceptions are the dual-engine modules documented under Approved Exceptions below.

### Three-Layer Stack

```
internal/media   Engine          — CGo/FFmpeg demux + decode + audio (oto v3)
internal/media   VideoPlayer     — Fyne widget: renders frames, built-in controls overlay
internal/ui      InlineVideoPlayer — THE API layer every module talks to
```

### Adding a New Module with Video

1. Declare a singleton `*ui.InlineVideoPlayer` in `native_media.go` (real) and `native_media_stub.go` (stub returning `ui.NewInlineVideoPlayer()`).
2. Add `Player *ui.InlineVideoPlayer` to the module's `Options` struct in **both** `view.go` and `stub.go`.
3. Wire `Player: GetXxxPlayer()` in `main.go`.
4. Use only `InlineVideoPlayer` methods inside the module — see `docs/NATIVE_PLAYER.md` for the full API surface.

### Current Module Singletons

| Module  | Getter              | Notes |
|---------|---------------------|-------|
| Convert | `GetConvertPlayer()` | Custom control row; `DisableBuiltinControls()` called |
| Trim    | `GetTrimPlayer()`    | Built-in controls overlay; in/out markers; preview region via `SetOnProgress` |
| Inspect | `GetInspectPlayer()` | Built-in controls disabled; `SetOnLoad` wired to Sync tab load-state pills |
| Filters (original) | `GetFiltersPlayer()` | Primary player; `SetPeer(GetFiltersPreviewPlayer())` wired in `init()` |
| Filters (preview)  | `GetFiltersPreviewPlayer()` | Follows primary via `SetPeer`; built-in controls disabled; muted after load |
| Upscale (original) | `GetUpscalePlayer()` | Primary player; `SetPeer(GetUpscalePreviewPlayer())` wired in `init()` |
| Upscale (preview)  | `GetUpscalePreviewPlayer()` | Follows primary via `SetPeer`; built-in controls disabled; muted after load |

### What the `native_media` Build Tag Gates

- `native_media.go` — real singletons, `HasNativeMediaPlayer() → true`, CGo player methods
- `native_media_stub.go` — no-op stubs, `HasNativeMediaPlayer() → false`
- `internal/media/` — CGo engine and widget (only compiled with tag)
- `internal/ui/inline_player.go` — real player (only compiled with tag)
- `internal/ui/inline_player_stub.go` — no-op stub (only compiled without tag)

Stub implementations must expose the **identical** method set as the real type so all callers compile on every target.

### Approved Exceptions to InlineVideoPlayer Rule

The following modules create `media.Engine()` directly instead of using `InlineVideoPlayer`. This is intentional because they require **two simultaneous video streams** which `InlineVideoPlayer` does not support.

| Module | File | Reason |
|--------|------|--------|
| Compare | `compare/fullscreen_native.go` | Side-by-side video comparison requires two independent engines |
| Upscale | `upscale_player_native.go` | Preview window + main player require two engines with custom playback loops |

**Do NOT refactor these modules to use InlineVideoPlayer** — the dual-engine architecture is required for their functionality.

## Agent Pipeline

LT uses a three-tier agent hierarchy defined in the Unified Framework v1.2 (§6). Roles and scope:

| Agent | Role | Scope | Examples |
|---|---|---|---|
| **Claude (Primary)** | Architect, systems planner, triage, documentation | Cross-session context, major architectural decisions, design discussions | Feature design, CI pipeline architecture, release strategy |
| **opencode (Secondary)** | Refactoring, code hygiene, smaller features | Module extraction, structural work, tasks without broad context | Button migration, file splits, lint fixes |
| **Gemini (Tertiary)** | Isolated specialist tasks | Contained problems with minimal cross-project context | DVD authoring pipeline, wiki sync |

Pipeline phases: **Design** (Claude) → **Implementation** (Claude + opencode + Gemini) → **Refinement** → **Versioning**. Each phase has clear entry/exit criteria.

## Using Sub-Agents

Claude Code supports spawning purpose-built sub-agents for parallelising independent work or protecting the main context window from large search results.

### Available Agent Types

| Type | Use for |
|------|---------|
| `Explore` | Fast read-only code search — finding files by pattern, grepping for symbols, answering "where is X defined?" Don't use for open-ended analysis or cross-file consistency checks. |
| `Plan` | Architecture and implementation planning before writing code. Returns step-by-step plans and identifies critical files. |
| `general-purpose` | Multi-step research, searches across the codebase, any task that may need several rounds of tool use. |
| `claude-code-guide` | Questions about the Claude Code CLI, Claude API, or Anthropic SDK. |

### When to Spawn a Sub-Agent

- **Multiple similar fixes** across many files (e.g. same CI error in 10 modules)
- **Independent tasks** with no shared state (parallelize freely)
- **Large codebase searches** that would flood the main context
- **Architecture review** before starting a feature (use `Plan`)

### Correct Invocation Syntax

Sub-agents are invoked via the `Agent` tool — not as Go code:

```
Agent(
  description="Fix inspect module build errors",
  subagent_type="general-purpose",
  prompt="Fix all build errors in internal/app/modules/inspect/. ..."
)
```

To run agents in parallel, emit multiple Agent tool calls in a single response.

### Guidelines

- Brief the agent like a colleague who has no prior context — include file paths, what was tried, and the expected outcome.
- Verify results: an agent's summary describes intent, not necessarily what it did. Read diffs before reporting work complete.
- Prefer foreground agents when their result informs the next step; use `run_in_background: true` only for genuinely independent work.
- After all agents complete, verify the build passes before committing.

## Hooks & Automation

Claude Code hooks are shell commands that fire at defined lifecycle points (before/after tool calls, before commit, etc.). Configure them in `.claude/settings.json` under the `hooks` key, or use `/update-config` to add them interactively.

### Recommended Hooks for This Repo

**i18n guard (pre-commit):** Catch hardcoded UI strings before they land.
```bash
# Fires before every commit — fails if raw string literals appear in widget constructors
grep -rn 'widget\.NewLabel("[^"]\+"\|widget\.NewButton("[^"]\+"\|dialog\.Show[A-Za-z]*("[^"]\+' \
  --include="*.go" internal/ *.go \
  | grep -v '_test\.go' | grep -v '\.pb\.go' \
  && echo "ERROR: hardcoded UI strings found — use i18n.T().KeyName" && exit 1 \
  || true
```

**Player debug log reminder (post player-file edit):** Prompts to update `docs/PLAYER_DEBUG.md` whenever a player-related file is modified.

**Conventions:**
- Hooks that gate commits should `exit 1` on failure so the commit is blocked.
- Keep hook scripts short; move complex logic into `scripts/hooks/` and call from there.
- Document any new hook in this file so other agents know it exists.

## Available Skills (Slash Commands)

These slash commands are available in Claude Code sessions and should be used proactively:

| Skill | When to use |
|-------|-------------|
| `/review` | Review a pull request before merge — runs a multi-agent review pass |
| `/security-review` | Security audit of pending branch changes |
| `/simplify` | After completing a feature — checks for reuse opportunities and fixes quality issues |
| `/schedule` | Schedule a background agent for a future task (e.g. clean up a feature flag in 2 weeks) |
| `/update-config` | Add hooks, permissions, or env vars to `.claude/settings.json` |
| `/init` | Regenerate `CLAUDE.md` when project structure changes significantly |

## Platform Scope

VideoTools targets **Linux and Windows only**. macOS is not a supported platform.

- Linux is the **primary development platform**. Implement features properly here first.
- Windows is the **primary user/tester platform**. Zero new runtime dependencies — use OS-built-ins only (e.g. `isoburn.exe` for disc burning).
- Linux may take small runtime dependencies where appropriate (e.g. `dvd+rw-tools` for disc burning).
- Do not add macOS-specific code paths, CI jobs, or documentation.
- **Existing darwin code should be removed** when found during code reviews or refactoring — there is no reason for any `case "darwin":` blocks to exist in this codebase.

## Next Steps (dev50 — engine stabilisation)

All 11 Phase 1 media engine items shipped. **Current priority: bring the VT Media Engine to 100% completeness and stability.** All player/metadata features must be applied consistently across every module. Refactoring is secondary unless it is critical for engine correctness. Main Menu refactor and similar housekeeping are explicitly deferred until the engine is fully stable.

**Recently shipped:** DLL startup validation — live `ffprobe.exe -version` smoke test on boot, `--dllcheck` CLI diagnostics flag, non-blocking Fyne error dialog on DLL failure, all 15 duplicate Windows CGo directives consolidated into `cgo_preamble.go`, and `docs/DLL_BOOTSTRAP.md` pipeline documentation.

Open engine items (in priority order):
1. **UDF thread safety & progress** — mutex-guarded Reader, extraction progress callbacks, temp-file tracking (`internal/dvd/udf/reader.go`)
2. **view.go component split** — break 1438-line `VideoPlayer` widget into `control_overlay.go`, `keyboard_shortcuts.go`, `thumbnail_preview.go` (deferred but needed before any new overlay features)
3. **Player interface extraction** — formal Go `Player` interface from `InlineVideoPlayer` for mock-based unit tests
