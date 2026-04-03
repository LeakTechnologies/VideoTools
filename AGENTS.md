# VideoTools Agent Workflow Rules

These rules apply to any automation or agent working in this repo.

## Current Project State

- Current cycle: `v0.1.1-dev39`.
- Public/stable baseline: `v0.1.1`.
- `dev30` is closed. CI validated on 2026-03-10 (runs 219/220/221, commit 2cbb3a2).
- `dev32` closed. CI validated on 2026-03-15.
- `dev35` closed. Module extraction in progress.
- Issue tracker active at `https://git.leaktechnologies.dev/leak_technologies/VideoTools/issues`.
- Primary planning source is `TODO.md`; shipped scope is tracked in `DONE.md`; release-facing history is `docs/CHANGELOG.md`.

## Immediate Handoff Priorities

- **Burn module** — Implement disc burning; see docs/BURN_MODULE_DESIGN.md
- **Auto-grey codecs** — Filter incompatible codecs based on format; see docs/AUTO_GREY_CODECS.md
- **Filter integration** — Merge filters into upscale module; see docs/FILTER_INTEGRATION_DESIGN.md
- **Module extraction** — Continue settings_module.go extraction to `internal/app/modules/settings/`.
- **IFO audio table** — Done (dev39). Audio attributes now populated from track data; helpers in `internal/dvd/ifo/audio.go`.
- **VTS_MAT byte layout** — Done (dev39). All field offsets in `mat_serialize.go`/`vtsi.go` corrected to match libdvdread `vtsi_mat_t`; fixes dvdnav `zero_12`/`zero_17` violations and `ifoRead_VTS_PTT_SRPT failed`.
- **DVD menu system** — Done (dev39). All M1-M7 items complete. `runNativeSpumux` produces proper MPEG-2+SPU VOBs; PCI button table populated in NAV_PCK; VMGM_VOBS_Sector wired (ISO: from UDF layout pass; folder: VMG_Last_Sector+1); menu PGC cell sectors patched for both ISO and folder outputs; extras pipeline active; `JumpVMGM_PGCNCommand` handles inter-menu navigation. See `docs/DVD_MENU_SYSTEM_DESIGN.md`.
- **Issue #5** (Convert UI cleanup) — layout consistency and label clarity pass on `buildConvertView` in `main.go`.
- Do not expand scope beyond what is listed unless explicitly approved.
- Keep the issue tracker in sync — close issues when work lands, open new ones for discovered bugs.

## Dev30 Closeout (Complete)

- `dev30` closed 2026-03-11. Checklist at `docs/DEV30_FINALIZATION_CHECKLIST.md`.
- CI confirmed green on commit 2cbb3a2. Release assets verified on Forgejo.
- Smoke test and dependency validation carried forward as issues #3, #4, #5, #18.

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
5. **Module names** already localized: use `t.ModuleXxx` (e.g., `t.ModuleConvert`)
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
It also omits `Libs.private`, leaving C++ runtime and math symbols unresolved in
FFmpeg's configure link test. The echo-command overwrite (LF, POSIX paths, with
`Libs.private: -lstdc++ -lsupc++ -lm` on Windows / `-lstdc++ -lm` on Linux) is required.

**cmake must be in the Linux apt-get install deps** (for x265 source build).
**nasm and mingw-w64-ucrt-x86_64-cmake must be installed in the Windows MSYS2 step** (for x264/x265 source builds).

If CI is failing, read the build log carefully before changing the FFmpeg setup strategy.
Open an issue or ask before touching the "Setup static FFmpeg" steps.

## Coordination

- Ask before changing workflow entrypoints or automation behavior.
- If a change affects installs/builds, add a short note in docs.
- Keep Forgejo release publishing aligned to `VERSION`; do not retarget releases to older dev tags.
- Be careful with tag/release operations:
  - `v0.1.1-dev29` is historical
  - current release work must stay on `v0.1.1-dev30` until `dev31` starts
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
- `main.go` is now ~15,248 lines (down from ~16,726). Remaining large blocks:
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

## Validation Priorities For Dev31

- Issues #3 and #4 are closed. Issue #5 (Convert UI cleanup) is the remaining dev31 UI item.
- Phase 3 modularisation is ongoing secondary work — Inspect, Settings, Queue are the next candidates.
- Carry-forward validations (tracked as issues, not blocking dev31 code work):
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

## Planned Feature: Update-Install Guard (Not Yet Implemented)

**Problem:** `applyUpdate` downloads a new binary and calls `performRestart`, which terminates the running process. If a queue job or conversion is active this destroys in-progress work with no warning.

**Scope:** `settings_module.go` only. No structural changes required.

### Entry points to guard

There are exactly two places `applyUpdate` is called:

1. `settings_module.go` ~line 1040 — "Install Update" button inside the `CheckForUpdatesWithStatus` dialog (triggered when a newer version tag is available).
2. `settings_module.go` ~line 1082 — "Install Patches" button in the same flow (triggered when same version tag but binary hash mismatch).

Both are `widget.Button` `OnTapped` closures that call `applyUpdate(state, tag)` directly.

### Guard logic

Before calling `applyUpdate`, check two conditions:

```go
queueBusy := state.jobQueue != nil && state.jobQueue.IsRunning()
busy := state.convertBusy || queueBusy
```

If `busy == true`, **do not call `applyUpdate`**. Instead show a blocking information dialog:

```
title:   "Update Blocked"
message: "A job is currently running. Please wait for all jobs to finish before installing an update.\n\nUpdates require a restart and would interrupt active work."
button:  "OK"
```

If `busy == false`, proceed with the existing `applyUpdate(state, tag)` call unchanged.

### What NOT to change

- Do not add an auto-retry or deferred install ("install when idle") — keep it simple.
- Do not disable the "Check for Updates" button or hide the update dialog — only block the final install action.
- Do not touch `applyUpdate` itself or `performRestart`.
- Do not add the guard to `applyUpdateStatusToUI` — that function only updates UI labels, it never installs.
- The `ApplyUpdate` adapter on `preferencesAdapter` (~line 1596) does not need changing; the guard belongs at the call site in the dialog callbacks, not in the adapter.

### i18n strings to add

Add to `internal/i18n/strings.go` and both locale files (`en_ca.go`, `fr_ca.go`):

| Key | English value |
|---|---|
| `DialogUpdateBlocked` | `"Update Blocked"` |
| `StatusUpdateBlockedByJob` | `"A job is currently running. Please wait for all jobs to finish before installing an update.\n\nUpdates require a restart and would interrupt active work."` |

### Commit message

```
fix(settings): block update install while queue or conversion job is active
```

### Test plan

1. Start a conversion or queue job.
2. Open Settings → Updates tab → check for updates.
3. When the "Install Update" or "Install Patches" dialog appears, click the install button.
4. Verify the "Update Blocked" dialog appears and `applyUpdate` is NOT called.
5. Let all jobs finish, then retry — verify the update proceeds normally.
