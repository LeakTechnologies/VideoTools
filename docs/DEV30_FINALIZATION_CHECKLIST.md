# Dev30 Finalization Checklist

Use this checklist to close `v0.1.1-dev30` cleanly and start `dev31`.

## 1) Scope Freeze

- [x] Freeze structural refactors after current approved Phase 3 slices.
- [x] Confirm no new feature creep is added to `dev30`.
- [x] Confirm all carry-over work is documented for `dev31`.

## 2) CI and Release Pipeline Validation

- [x] `windows-package` workflow succeeds on latest `master`.
- [x] `linux-package` workflow succeeds on latest `master`.
- [x] `publish-release` updates only the `VERSION` tag release (`v0.1.1-dev30`).
- [x] Release assets are replaced (no duplicates from prior runs).
- [x] Release notes show concise highlights (not full raw changelog dump).
- [x] Stale-run guard works (older runs do not overwrite release state).

Evidence:
- Run URLs: workflow_dispatch runs 219 (windows-package), 220 (linux-package), 221 (publish-release)
- Commit SHA tested: 2cbb3a2a8083dbc74759fd135c26d50bf14bd6ab
- Date: 2026-03-10

## 3) Core Module Smoke Test

- [ ] Main menu layout/containment is stable across common window sizes.
- [ ] Convert accepts drag/drop and probes source without PATH-only ffprobe dependency.
- [ ] Queue operations work (pause/resume/cancel/remove, copy error/command).
- [ ] Settings panel scrolling and tab behavior are stable and responsive.
- [ ] Subtitles extract/embed baseline flow works without sync drift regressions.
- [ ] Inspect/Compare basic metadata loads complete without UI breakage.

**Waived for dev30 closeout** — carry forward to dev31. Tracked via issues #3, #4, #5.

Evidence:
- Tester:
- Platform(s):
- Notes:

## 4) Dependency Validation

- [ ] Clean Windows machine/VM: first-run FFmpeg bootstrap succeeds.
- [ ] After bootstrap, modules unlock without app restart.
- [ ] Thumbnail/convert metadata probing works with app-local FFprobe path.

**Waived for dev30 closeout** — carry forward to dev31. Tracked via issue #18.
No-cost option: Windows Sandbox (built into Windows 10/11 Pro).

Evidence:
- Environment:
- Result:

## 5) Docs and Tracking Closeout

- [x] `DONE.md` updated with final dev30 shipped scope.
- [x] `TODO.md` updated with clear dev31 carry-overs.
- [x] `docs/CHANGELOG.md` dev30 section finalized.
- [x] No personal names introduced in new docs.

## 6) Tag and Release Finalization

- [x] Verify `v0.1.1-dev30` points to intended final commit.
- [x] Re-point tag only if required (delete remote tag, push corrected local tag).
- [x] Confirm release page assets + notes match final commit.

Confirmed via publish-release run 221 — assets purged and re-uploaded against commit 2cbb3a2.

## 7) Start Dev31

- [x] Bump version markers to `v0.1.1-dev31`:
  - `VERSION`
  - `main.go` (`appVersion`)
  - `FyneApp.toml`
- [x] Add dev31 kickoff notes to `DONE.md`, `TODO.md`, and `docs/CHANGELOG.md`.
