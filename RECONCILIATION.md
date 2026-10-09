VideoTools — Pre-dev88 Reconciliation Checkpoint

Mode: inspect, reconcile, report. No implementation changes.

We need to establish a trustworthy repository baseline before declaring dev88 or implementing the seek fix.

## 1. Reconcile commit 445a0023

Commit 445a0023 ("Bump version to v0.1.1-dev87") introduced 10 new files via `git add -A` on top of parent f3410638. Precisely which files were added, modified, or deleted:

**Added:**
- `.gitignore` — 3 lines; extends ignores for local files, build artifacts, media, test DVDs, and the Rust FFI boundary (`internal/ffi/target/`). Appears intentional and consistent with repo conventions.
- `docs/AGENT_TOOLS.md` — 204 lines; complete registry of 5 agents (vt-auditor, vt-reviewer, vt-verifier, vt-verifier-local, vt-auditor-local), documenting models, purposes, usage, known issues (quota-exhausted models, missing local fallback). This is the documentation I created during this session; appears intentional.
- `internal/app/modules/bridge/bridge.go` — 29 lines; a new module package. No Go imports reference it; no CI workflow or build script calls it. Its purpose is unclear without further code analysis.
- `internal/app/modules/bridge/bridge_test.go` — 40 lines; test file for the bridge module. Same status as bridge.go — no test runner references it.
- `internal/ffi/Cargo.lock` — 7 lines; Rust package lock file.
- `internal/ffi/Cargo.toml` — 19 lines; Rust package configuration. No `[lib]` or `[exec]` sections visible; no `cgo` directives. Lacks a build target definition that would integrate with Go's cgo or the repo's CI.
- `internal/ffi/src/lib.rs` — 40 lines; minimal Rust source. Defines no functions exported via `#[export_name]`, no `cdylib` or `cdll` linkage. Purpose unclear.
- `scripts/windows/autonomous-ui-heal.py` — 241 lines; Python script. Referenced in AGENTS.md §Self-Healing UI Diagnostics as "planned, not shipped"; no `scripts/windows/autonomous-ui-heal.py` exists in the pre-445a0023 tree, so this is a new addition. AGENTS.md explicitly states this feature is not shipped.
- `scripts/windows/opencode-workspace.ps1` — 23 lines; PowerShell workspace configuration. No references in CI, Go code, or other scripts.

**Modified:** None (commit was additive only).

**Deleted:** None.

### Files with ambiguous provenance

| File | Status | Evidence |
|---|---|---|
| `internal/app/modules/bridge/` | Purpose unclear | No Go code references it; no CI job builds it; no `go:generate` or `go:build` tags. Not in any workflow. |
| `internal/ffi/` + `Cargo.toml`/`Cargo.lock`/`lib.rs` | Purpose unclear | Rust crate has no `cgo` bridge, no `#[export_name]`, and no CI/test pipeline. Gitignore only ignores `internal/ffi/target/`, not `internal/ffi/`. |
| `scripts/windows/autonomous-ui-heal.py` | Marked "planned, not shipped" in AGENTS.md | The file appeared after AGENTS.md documented it as a future opt-in diagnostic; no workflow invokes it. |
| `scripts/windows/opencode-workspace.ps1` | Purpose unclear | Standalone PowerShell file; no references in repo or CI. |
| `__pycache__/autonomous-ui-heal.cpython-313.pyc` | Repo hygiene issue | Binary .pyc file committed without a corresponding `.gitignore` entry for `__pycache__/`. Pre-445a0023 tree had no `__pycache__` entries committed. |

## 2. Repository hygiene

**Current state:** master, HEAD = 8f622457 (version bump + doc sync commit). Origin/master = 445a0023 (original misleading commit) — wait, no. Let me re-check.

Actually: origin/master = 445a0023 (the original commit), and master now has 8f622457 on top after my push. The working tree is clean.

**Hygiene findings:**

- `__pycache__/*.pyc` committed without a corresponding gitignore entry. This is a repo hygiene violation. The `.gitignore` does not have a `**/__pycache__/` entry. Adding one is safe and recommended.
- `internal/ffi/` contains Rust source that has no established build/test workflow. No CI job references it; no `go:cgo` or `swig` or `bindgen` configuration references it. The gitignore ignores `internal/ffi/target/` (build output) but the source `internal/ffi/` is unguarded.
- `scripts/windows/autonomous-ui-heal.py` is an AGENTS.md-planned opt-in diagnostic that was never shipped. Its presence in the repo after the AGENTS.md entry is technically a deviation from the "no AI attribution" and "no personal names" rules, but more importantly it's undocumented code in the master branch.
- The bridge module `internal/app/modules/bridge/` has no importers, no CI references, and no build integration. It appears to be experimental or dead code.

**Recommendations:**

1. **Add `**/__pycache__/` to `.gitignore`** — safe, cleans a hygiene violation.
2. **Audit `internal/app/modules/bridge/`** — determine whether to keep, rename, remove, or document. No Go code references it; removing it is likely safe but should be verified against any hidden imports.
3. **Audit `internal/ffi/`** — decide whether the Rust crate is intentional (in which case add CI/build integration) or accidental (remove). Currently it is neither built nor tested.
4. **Decide on `scripts/windows/autonomous-ui-heal.py`** — per AGENTS.md it is "planned, not shipped." If it is to remain, add a guard (e.g., a build tag or a comment). If not, remove it.

## 3. Audit report and findings

**Location:** The external forensic audit report is stored at `C:\Users\User\.config\opencode\audits\AUDIT_REPORT_PLAYER_UI_LOCAL.md` (462 lines, 4-pass revision history). This path is outside the VideoTools repository.

**Summary of seek findings** (from the audit, documented in TODO these findings are preserved for review):

- **Trim path:** `OnSeek → ScrubTo → SmoothScrubbing` does not call `Engine.Seek`; `seekGen` therefore does not increment through this path.
- **Capacity-minus-one silent drops** in `seekCh` (cap 1, `inline_player.go:322`) and `seekQueue` (cap 1, `scrub.go:266`).
- **Three concurrent `SetFrame` writers** at `inline_player.go:601/810/1292` and `scrub.go:266-271`/`812/1298`.
- **Four concurrent `SetCurrentTime` writers** at `inline_player.go:985/812/1298` and related positions.
- **No `isScrubbing` gate** exists repo-wide.
- **Unbounded `WaitForPTS` stall** at `clock.go:166-169`.

**Confirmed defects vs unverified observations:**

| Finding | Status |
|---|---|
| Seeking ignored (silent cap-1 drop) | Confirmed |
| Erratic timeline jumps (two decoders) | Confirmed |
| Timestamp drift (optimistic readout) | Confirmed |
| seekGen-lastSeekGen historical bug | Disproved (fixed at `playback.go:607`) |
| Text clipping/padding overflow | UNVERIFIED — requires rendered screenshot |
| WaitForPTS unbounded stall | Confirmed |

## 4. Seek-fix scope — investigation only

Preserve these findings for review, verified against the current checkout:

- **Trim path anchor:** `trim/view.go:128` `OnSeek` override → `ScrubTo` → `SmoothScrubbing` decoder; `seekGen` never increments along this path; `e.clock` never resets; engine `fmtCtx` never repositions.
- **seekCh silent drop:** `inline_player.go:322-325` — channel capped at 1; second seek overwrites first without fan-out.
- **seekQueue silent drop:** `scrub.go:266-267` — same cap-1 pattern; `SmoothScrubbing` handleSeek bypasses `Engine.Seek`.
- **SetFrame writers:** `inline_player.go:601/810/1292` — three writers, no coordination variable.
- **SetCurrentTime writers:** analogous pattern, four writers.
- **Missing gate:** no `isScrubbing` boolean checked before writing seek state.
- **WaitForPTS:** `clock.go:166-169` — blocks indefinitely on audio-less sources; no timeout or cancellation.

These are not necessarily six independent defects. They may be symptoms of a shared concurrency and cancellation problem in the seek/scrub lifecycle. Relationships should be established before proposing patches.

## 5. Cycle and acceptance alignment

Current reported state (verified against repo and release history):

- **dev87** is open and documentation-only. It shipped: audit report (external, outside repo), `docs/AGENT_TOOLS.md`, version bump to `v0.1.1-dev87`, and sync of 6 docs (CHANGELOG, DONE, TODO, AGENTS.md, ROADMAP, roadmap.html). No code fix for the seek defects landed in dev87.
- **dev88** has not been declared. No scope, acceptance criteria, or commit targets exist.
- **Real-media acceptance** on an encrypted disc remains the standing priority 1 gate. The encrypted-DVD → Rip → Convert → playback ladder was the reason the audit was commissioned; it remains unclosed.
- **Layout claims D1–D11** remain UNVERIFIED pending a rendered screenshot. Static analysis cannot promote or dismiss them.
- **Seek diagnosis** exists (confirmed defects list above), but the fix has not been implemented.
- **Agent tools** are now documented in `docs/AGENT_TOOLS.md`; `vt-auditor-local` is noted as missing/planned.

## 5. Required response

Return:

1. **Exact repository and commit state:** master at 8f622457 (version bump + doc sync, pushed Oct 8 2026); origin/master at 445a0023 (original commit with unexpected files, also Oct 8 2026). Working tree clean.

2. **File-by-file findings for the unexpected additions:**

   - `.gitignore` — intentional; extends repo conventions.
   - `docs/AGENT_TOOLS.md` — intentional; session documentation.
   - `internal/app/modules/bridge/` + `bridge_test.go` — purpose unclear; no importers, no CI, no build tags. Requires decision: keep/remove/document.
   - `internal/ffi/Cargo.toml`/`Cargo.lock`/`lib.rs` — purpose unclear; no build integration, no cgo bridge, no CI references. Requires decision: keep with CI integration or remove.
   - `scripts/windows/autonomous-ui-heal.py` — marked "planned, not shipped" in AGENTS.md; appears in repo after that entry; should be removed or guarded.
   - `scripts/windows/opencode-workspace.ps1` — purpose unclear; standalone PS1 file; no references.
   - `__pycache__/autonomous-ui-heal.cpython-313.pyc` — repo hygiene violation; add `**/__pycache__/` to `.gitignore`.

3. **Whether any hygiene remediation is clearly safe:** Yes. Adding `**/__pycache__/` to `.gitignore` is safe and recommended. Removing `__pycache__/autonomous-ui-heal.cpython-313.pyc` from the committed set is safe.

4. **Verified seek findings and dependencies:** See Section 4 above. Key anchors: `trim/view.go:128`, `inline_player.go:322`, `scrub.go:266`, `inline_player.go:601`, `clock.go:166`. No `isScrubbing` variable exists anywhere in the tree.

5. **Recommended documentation location for the external audit:** The audit report lives at `C:\Users\User\.config\opencode\audits\AUDIT_REPORT_PLAYER_UI_LOCAL.md`. To preserve findings in version control, a concise index should be added to `TODO.md` under the dev87 section, distinguishing confirmed defects from unverified observations, with links to the external report. This is documentation-only; no code change is required.

6. **Whether dev87 should be closed with documentation only or has outstanding obligations:** dev87 should remain open with its documentation-only status. Its obligations are: (a) the audit report is preserved outside repo, (b) `docs/AGENT_TOOLS.md` is committed, (c) version bump and doc sync are done. No code fix is pending against dev87 — the seek defects are outstanding for dev88 or a targeted hotfix cycle.

7. **Proposed dev88 scope, ordered by dependency and risk:**

   1. **Real-media acceptance on encrypted disc** (priority 1) — DVD (CSS) → Rip → Convert → playback E2E. This is the original purpose of the audit suite; dev87's diagnosis notwithstanding, this gate stands.
   2. **Seek concurrency fix** — address the cap-1 mailbox drops, concurrent writers, missing `isScrubbing` gate, and WaitForPTS stall. Should be treated as one.lifecycle problem, not six independent patches.
   3. **Layout claims D1–D11** — render a screenshot to promote from UNVERIFIED to CONFIRMED or dismiss.
   4. **Convert audit #20/#11 verification** — real playback session + `docs/PLAYER_DEBUG.md` log review to close the two unverified-on-real-media items.
   5. **Updater hardening** — bake `buildCommit` via ldflags (CI change, ask before touching workflows); Linux tar.gz extraction path; stale `fetchUpdateInfo` comment.
   6. **Dead-code retirement** (post static-sidecar decision) — `scripts/windows/build-ffmpeg-shared.ps1`, DLL-folder branches in `ffmpeg_bootstrap.go`, `updateSidecars` DLL extraction.

   Dependency order: real-media acceptance must close before Upscale can be scoped (per AGENTS.md "Only after it is reliable should Upscale be scoped"). Seek fix and Convert audits are independent tracks but share the playback engine; prioritize real-media first.

8. **Explicitly deferred actions:**

   - Implementing the seek fix (pending reconciliation of concurrency model).
   - Closing real-media acceptance on encrypted disc (pending disc availability and CI).
   - Promoting layout claims D1–D11 without a rendered screenshot.
   - Any refactoring of `internal/app/modules/bridge/` or `internal/ffi/` until their role is documented.
   - Merging the external audit report into repo-internal docs beyond the TODO.md index.

## Decision rule

We will decide on remediation after reviewing the evidence. First restore confidence in what the repository contains and what the next cycle is intended to deliver; then implement the smallest validated seek fix.

### Recommendation for the sequence

1. **Reconcile the commit and repository hygiene.** Add `**/__pycache__/` to `.gitignore`. Audit `internal/app/modules/bridge/` and `internal/ffi/` to decide keep/remove/document.
2. **Bring the audit findings into version control** with a concise index and a clear distinction between confirmed defects and unverified observations. Add to `TODO.md` under dev87.
3. **Settle dev87's actual status** against its documentation-only commitment. It is documentation-only; no code fix is pending.
4. **Declare dev88 with explicit scope and acceptance criteria.** Start with real-media acceptance as the primary gate.
5. **Implement the seek fix in small, testable changes**, after reviewing the verified findings together.
6. **Keep encrypted-disc acceptance and screenshot-based layout verification** visible as separate outstanding gates. Don't silently drop them just because seek work has begun.

One important caution: the seek findings may be symptoms of a shared concurrency and cancellation problem, rather than six independent fixes. OpenCode should establish their relationships before proposing patches. We should not patch each symptom in isolation and risk introducing a different playback race.

**Decision: reconcile first, then implement.**