# VideoTools Agent Workflow Rules

These rules apply to any automation or agent working in this repo.

## Current Project State

- Current cycle: `v0.1.1-dev30`.
- Public/stable baseline: `v0.1.1`.
- `dev30` is in closeout, not open-ended feature expansion.
- Next cycle begins at `v0.1.1-dev31` after `dev30` release/tag validation is complete.
- The active closeout checklist is `docs/DEV30_FINALIZATION_CHECKLIST.md`.
- Primary planning source is `TODO.md`; shipped scope is tracked in `DONE.md`; release-facing history is `docs/CHANGELOG.md`.

## Immediate Handoff Priorities

- Do not add new feature scope to `dev30` unless explicitly approved.
- Finish `dev30` by validating CI, release assets, smoke tests, and tag state.
- Once `dev30` is closed, bump `VERSION`, `main.go`, and `FyneApp.toml` to `v0.1.1-dev31`.
- Start `dev31` by carrying forward only the unchecked items that remain relevant from `TODO.md`.

## Dev30 Closeout Rules

- Treat `docs/DEV30_FINALIZATION_CHECKLIST.md` as required, not optional.
- Before declaring `dev30` complete, verify:
  - latest `master` build passes for Linux and Windows
  - `publish-release` updates only the `VERSION` tag release
  - release assets are replaced cleanly
  - release notes use concise highlights, not raw changelog dumps
  - `v0.1.1-dev30` points to the intended final commit
- Do not start `dev31` version bumps until those checks are complete or explicitly waived by the user.

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
- Phase 2 is complete enough to proceed; Phase 3 has started.
- Completed Phase 3 slices already moved logic into:
  - `internal/app/modules/about`
  - `internal/app/modules/deps`
  - `internal/app/modules/mainmenu`
- Continue using thin `package main` shims while moving logic out of root files.
- Prefer small, reversible refactor slices. Do not combine structural moves with unrelated feature work.
- The long-term goal remains:
  - reduce root-level `package main` clutter
  - move app logic into `internal/app/`
  - move the executable entrypoint toward `cmd/videotools/`

## Validation Priorities For Dev31

- Highest-value carry-over items:
  - complete remaining Phase 3 / main.go modularization safely
  - validate Windows first-run FFmpeg bootstrap on a clean machine
  - validate cross-platform dependency actions
  - validate Forgejo packaging/release workflows end-to-end
  - keep UI/resolution behavior stable while refactoring
- Do not reopen bundled dependency packaging for dev builds unless explicitly requested; dev workflows currently publish standard packages only.
