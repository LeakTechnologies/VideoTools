# VideoTools Agent Workflow Rules

These rules apply to any automation or agent working in this repo.

## Current Project State

- Current cycle: `v0.1.1-dev44`.
- Public/stable baseline: `v0.1.1`.
- `dev43` closed. Player thread-safety audit: pixel format SIGSEGV fix, Close/demuxer WaitGroup, NextFrame/Close codec race gate, seekLoop goroutine leak. Pending smoke test.
- `dev42` closed. Player stabilisation (D3D11VA, audio sync, GStreamer removal), thumbnail 3-way output, audio i18n, convert drop fixes.
- `dev40` closed. CI validated.
- Issue tracker active at `https://git.leaktechnologies.dev/leak_technologies/VideoTools/issues`.
- Primary planning source is `TODO.md`; shipped scope is tracked in `DONE.md`; release-facing history is `docs/CHANGELOG.md`.
- **Player debug log:** `docs/PLAYER_DEBUG.md` — keep this up to date as issues are found and fixed. Update it before closing any player-related issue.

## Immediate Handoff Priorities

- **Player smoke test (dev44)** — Verify dev43 fixes hold: load and play H.264/AV1/MPEG4 files in SW decode mode; confirm no frame-5 crash. Update `docs/PLAYER_DEBUG.md` Videos Tested table with results.
- **Player P1 fixes (dev44)** — See `docs/PLAYER_DEBUG.md` Known Issues. Top items: `predecodeFrom` sharing `formatCtx` with `demuxerLoop`; audio queue not flushed before seek; dead `predecodeAhead` code.
- **Burn module** — Implement burn logic (IMAPI2 on Windows, SG_IO on Linux); UI is wired. See `docs/BURN_MODULE_DESIGN.md`.
- **Convert Module Improvements Phase 1** — Audio Sample Rate, Normalize Audio, Deinterlace, H.264 Profile/Level UI controls. See `docs/CONVERT_MODULE_IMPROVEMENTS.md`.
- **Issue #5** (Convert i18n) — ~50 hardcoded strings in buildConvertView need i18n keys.
- **Do not expand scope beyond what is listed unless explicitly approved.**
- Keep the issue tracker in sync — close issues when work lands, open new ones for discovered bugs.

## Commit Discipline

- After every change: `git add` then `git commit -m "..."`.
- Do not leave unstaged changes in the worktree.
- Commit only files related to the current task.

## Documentation Discipline

- If behavior changes, update:
  - `docs/INSTALLATION.md`
  - the relevant platform guide (`docs/INSTALL_WINDOWS.md`, `docs/INSTALL_LINUX.md`)
- Always update `DONE.md`, `TODO.md`, and `CHANGELOG.md` for completed or planned work.
- `CHANGELOG.md` means `docs/CHANGELOG.md` in this repo.
- Avoid personal names in documentation; use `user report` or `dev report` only.
- The retired `docs.leaktechnologies.dev` site must not be used; active docs live in-repo and on the Forgejo wiki.

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

## CI Build — Settled Decisions (Do Not Revert)

The following decisions in `.forgejo/workflows/dev-packages.yml` were reached after multiple failed attempts and must not be changed without explicit approval:

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

**All modules must use `ui.InlineVideoPlayer` as their API layer. Period.**

Do not call `media.NewEngine()` inside a module. Do not write a per-module playback goroutine. Do not call `media.NewVideoPlayer()` directly. Get the widget via `opts.Player.Widget()`.

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

### What the `native_media` Build Tag Gates

- `native_media.go` — real singletons, `HasNativeMediaPlayer() → true`, CGo player methods
- `native_media_stub.go` — no-op stubs, `HasNativeMediaPlayer() → false`
- `internal/media/` — CGo engine and widget (only compiled with tag)
- `internal/ui/inline_player.go` — real player (only compiled with tag)
- `internal/ui/inline_player_stub.go` — no-op stub (only compiled without tag)

Stub implementations must expose the **identical** method set as the real type so all callers compile on every target.

### Reference in `AGENTS.md` Line 158

The entry `player_module.go` in the completed Phase 3 list refers to the root-level `player_module.go` (shows the legacy static player view), **not** to the native media player stack above.

### Approved Exceptions to InlineVideoPlayer Rule

The following modules create `media.Engine()` directly instead of using `InlineVideoPlayer`. This is intentional because they require **two simultaneous video streams** which `InlineVideoPlayer` does not support.

| Module | File | Reason |
|--------|------|--------|
| Compare | `compare/fullscreen_native.go` | Side-by-side video comparison requires two independent engines |
| Upscale | `upscale_player_native.go` | Preview window + main player require two engines with custom playback loops |

**Do NOT refactor these modules to use InlineVideoPlayer** — the dual-engine architecture is required for their functionality.

---

## Validation Priorities For Dev40

- Issue #5 (Convert UI cleanup) is the remaining open UI item.
- Phase 3 modularisation is ongoing secondary work — Inspect, Settings, Queue are the next candidates.
- Carry-forward validations (tracked as issues, not blocking dev40 code work):
  - Windows first-run FFmpeg bootstrap — issue #18
  - cross-platform dependency actions — issue #7
  - Forgejo packaging/release workflows end-to-end — issues #8, #9, #10
  - Do not reopen bundled dependency packaging for dev builds unless explicitly requested.

## Using Sub-Agents

When multiple independent tasks exist within a single change, use sub-agents to parallelize work and reduce bottlenecks.

### When to Use Sub-Agents

- **Multiple similar fixes** (e.g., fixing same issue in 10 different files)
- **Independent tasks** that can run concurrently
- **Larger refactors** that can be split into logical chunks
- **CI build failures** affecting multiple files

### How to Use

Use the Task tool with `subagent_type: "general"` to spawn parallel workers:

```go
// Example: parallel CI fixes
task(description="Fix inspect module", prompt="...", subagent_type="general")
task(description="Fix trim module", prompt="...", subagent_type="general")
task(description="Fix other errors", prompt="...", subagent_type="general")
```

### Guidelines

- Provide clear, specific instructions in the prompt
- Include the expected commit message format
- Verify build passes after all sub-agents complete
- Merge any conflicting changes manually before committing

## Platform Scope

VideoTools targets **Linux and Windows only**. macOS is not a supported platform.

- Linux is the **primary development platform**. Implement features properly here first.
- Windows is the **primary user/tester platform**. Zero new runtime dependencies — use OS-built-ins only (e.g. `isoburn.exe` for disc burning).
- Linux may take small runtime dependencies where appropriate (e.g. `dvd+rw-tools` for disc burning).
- Do not add macOS-specific code paths, CI jobs, or documentation.
- **Existing darwin code should be removed** when found during code reviews or refactoring — there is no reason for any `case "darwin":` blocks to exist in this codebase.

