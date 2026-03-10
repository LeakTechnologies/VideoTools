# Dev30 Finalization Checklist

Use this checklist to close `v0.1.1-dev30` cleanly and start `dev31`.

## 1) Scope Freeze

- [ ] Freeze structural refactors after current approved Phase 3 slices.
- [ ] Confirm no new feature creep is added to `dev30`.
- [ ] Confirm all carry-over work is documented for `dev31`.

## 2) CI and Release Pipeline Validation

- [ ] `windows-package` workflow succeeds on latest `master`.
- [ ] `linux-package` workflow succeeds on latest `master`.
- [ ] `publish-release` updates only the `VERSION` tag release (`v0.1.1-dev30`).
- [ ] Release assets are replaced (no duplicates from prior runs).
- [ ] Release notes show concise highlights (not full raw changelog dump).
- [ ] Stale-run guard works (older runs do not overwrite release state).

Evidence:
- Run URL(s):
- Commit SHA tested:
- Date:

## 3) Core Module Smoke Test

Run the core subset from `docs/TESTING_MODULE_CHECKLIST.md`.

- [ ] Main menu layout/containment is stable across common window sizes.
- [ ] Convert accepts drag/drop and probes source without PATH-only ffprobe dependency.
- [ ] Queue operations work (pause/resume/cancel/remove, copy error/command).
- [ ] Settings panel scrolling and tab behavior are stable and responsive.
- [ ] Subtitles extract/embed baseline flow works without sync drift regressions.
- [ ] Inspect/Compare basic metadata loads complete without UI breakage.

Evidence:
- Tester:
- Platform(s):
- Notes:

## 4) Dependency Validation

- [ ] Clean Windows machine/VM: first-run FFmpeg bootstrap succeeds.
- [ ] After bootstrap, modules unlock without app restart.
- [ ] Thumbnail/convert metadata probing works with app-local FFprobe path.

Evidence:
- Environment:
- Result:

## 5) Docs and Tracking Closeout

- [ ] `DONE.md` updated with final dev30 shipped scope.
- [ ] `TODO.md` updated with clear dev31 carry-overs.
- [ ] `docs/CHANGELOG.md` dev30 section finalized.
- [ ] No personal names introduced in new docs.

## 6) Tag and Release Finalization

- [ ] Verify `v0.1.1-dev30` points to intended final commit.
- [ ] Re-point tag only if required (delete remote tag, push corrected local tag).
- [ ] Confirm release page assets + notes match final commit.

## 7) Start Dev31

- [ ] Bump version markers to `v0.1.1-dev31`:
  - `VERSION`
  - `main.go` (`appVersion`)
  - `FyneApp.toml`
- [ ] Add dev31 kickoff notes to `DONE.md`, `TODO.md`, and `docs/CHANGELOG.md`.

