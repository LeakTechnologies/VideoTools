# VideoTools Agent Workflow Rules

These rules apply to any automation or agent working in this repo.

## Current Project State

- Current cycle: `v0.1.1-dev31`.
- Public/stable baseline: `v0.1.1`.
- `dev30` is closed. CI validated on 2026-03-10 (runs 219/220/221, commit 2cbb3a2).
- `dev31` focus: Phase 3 modularisation (main.go reduction) and Convert UI cleanup (issue #5).
- Issues #3 (module scroll + action bar) and #4 (window resize) are complete.
- Issue tracker active at `https://git.leaktechnologies.dev/leak_technologies/VideoTools/issues`.
- Primary planning source is `TODO.md`; shipped scope is tracked in `DONE.md`; release-facing history is `docs/CHANGELOG.md`.

## Immediate Handoff Priorities

- **Phase 3 modularisation** is the primary task. See `## Refactor Boundaries` below for exact state and next steps.
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
  - `internal/app/modules/convert` — Convert placeholder (actual logic still in `main.go`)
  - `player_module.go` — `showPlayerView` + `buildPlayerView`
  - `enhancement_module.go` — `buildEnhancementView` (placeholder)
  - `upscale_module.go` — full upscale view + AI helpers
  - `compare_module.go` — `showCompareView`, `showCompareFullscreen`, `buildCompareView`, `buildCompareFullscreenView`
- `main.go` is now ~15,185 lines (down from ~16,726). Remaining large blocks:
  - `buildConvertView` (~3,500 lines) — do not split without a clear interface plan
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
