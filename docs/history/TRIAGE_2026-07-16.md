# Triage — 2026-07-16

## CI Status
- [pass] dev.yml — Linux + Windows builds green on master
- [pass] release.yml — last tag v0.1.1-dev53 published successfully

## Open Issues
- No open GitHub issues (tracker: https://github.com/LeakTechnologies/VideoTools/issues)

## Active Work
- **Player performance fixes** (dev53) — 6 of 7 targeted fixes implemented:
  - [x] Fix #2: videoDecodeLoop poll loop → TimedGet()
  - [x] Fix #3: seek-on-resume → lighter audio flush
  - [x] Fix #5: slider updates throttled to ~15fps
  - [x] Fix #8: subtitleCodecMu → atomic bool
  - [x] Fix #9: SWS_BICUBIC → SWS_FAST_BILINEAR
  - [ ] Fix #11: paused mutex → atomic.Bool (pending)
  - [ ] PLAYER_DEBUG.md update (pending)

## Stale TODOs
- `renderDualPlayerPreview` stub — no progress, needs design
- Dead-code retirement — DLL-folder branches, `updateSidecars` extraction
- Player interface extraction — 47 call sites, deferred

## Pending Sign-off
- dev51 build — release assets published, awaiting tester verification
- dev53 build — will need tester verification after player fixes land

## Suggested Next Actions
1. **Finish Fix #11** (paused mutex → atomic) — small, low-risk, completes the performance batch
2. **Update PLAYER_DEBUG.md** — document all 6 fixes for the player debug log
3. **Commit + version bump** — land the player performance batch as dev54
4. **Run triage again after commit** — verify CI is green with the new changes

## Token Budget Note
This triage was read-only. Implementation loops should be triggered manually.
