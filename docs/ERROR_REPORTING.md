# Error evidence and Git completion

Adopted 2026-11-04 under [#41](https://github.com/LeakTechnologies/VideoTools/issues/41).
Use with [the engineering workflow](ENGINEERING_WORKFLOW.md). GitHub Issues is
the actionable record; [#29](https://github.com/LeakTechnologies/VideoTools/issues/29)
orders work. This is a reporting contract, not a new logger, CI gate or telemetry.

## When a failure appears

Search existing issues first. Add actionable evidence to the matching issue in
the same work session; create one bounded issue when no match exists. A user
report is valid before an engineer reproduces it. Do not require a guessed root
cause, a patch or a solution approval to report a defect.

Record user reports, directly observed failures, source-confirmed defects and
hypotheses distinctly. An old issue is not proof a current candidate still fails.
If reproduction is unavailable, record why and the next useful check. Capture a
failure before changing the candidate; avoid repeated runs that add no evidence.

Use the [defect form](../.github/ISSUE_TEMPLATE/bug_report.yml) for initial reports
and the engineering form for a bounded implementation plan. Add a short update
using the following format; omit fields genuinely irrelevant to the update.

```text
Candidate: commit / build / installed version (unknown values explicit)
Environment: OS, relevant tool/backend versions, appearance or source structure
Evidence: user-reported / directly observed / source-confirmed / hypothesis
Trigger: minimal numbered steps or sanitized command, plus frequency
Expected:
Observed: exact error/exit code or visible behaviour, stage and task impact
Evidence reference: sanitized excerpt, source location or local capture reference
Cause: unknown / suspected with reason / confirmed with supporting evidence
Check result: passed / failed / blocked / not run; what this result establishes
Next action: bounded investigation or repair; who/action needed for acceptance
Related issue / candidate / PR:
```

For media describe structural facts: container/codec, duration class, stream and
chapter counts, interlace, encryption, VTS/cell structure when relevant. For UI
include window/capture dimensions, font/size, locale, preview state and affected
control. For lifecycle failures include replacement/cancel/navigation sequence.
Report original task impact separately from the cost or severity of the cause.
An intermittent or unattended result must retain that qualification.

## Evidence handling and corrections

- Copy exact useful errors and exit codes after sanitizing secrets, real media
  titles, private paths and personal details. Use neutral fixtures and aliases.
  Do not publish entire logs, raw captures, chat transcripts or credentials.
  Keep raw evidence outside tracked files or in an already ignored local folder;
  check ignore status before saving there. Never force-add ignored evidence.
- Preserve the original report. Add dated corrections that identify which claim
  changed and why. Confirmed/refuted/unverified are distinct; do not silently
  rewrite a disproved diagnosis as if it were always known.
- A failed command is not automatically a failed application test. If the test
  process could not start, record the environment failure and that the test did
  not execute. Example: Windows `0xc0000135` means missing DLL startup, not a test
  assertion. A permission-denied preflight leaves inspection unknown, not clean.
- A valid output container or PNG is insufficient if intended content is absent,
  wrong or black. Check the content needed by the task. No error log is not proof
  of healthy playback; compilation is not rendered UI or real-disc acceptance.
- Severity describes impact (loss/corruption, blocked task, degraded task, cosmetic);
  priority describes ordering. State the reason. Use current area/priority labels
  where useful; do not invent a severity score unsupported by evidence.
- If GitHub is unavailable, retain a sanitized pending entry in the relevant task
  report, or create docs/TRIAGE.md when a local triage record is actually needed.
  Mark it unsynchronized and link/sync it to the issue when access returns. Do not
  maintain a permanent second backlog or claim an issue was updated when it was not.

Neutral example: "dev101, Windows, Convert, preview collapsed; importing a batch
still invokes frame captures (source-confirmed). Import cost unmeasured; whole
movie residency not established. Next: measure on/off preparation under #37."
This records what is known without turning a tester's explanation into a diagnosis.

## Verification and closure update

```text
Implemented: commit / PR, actual behaviour changed
Checks: candidate + command/action + environment + outcome
Failed/blocked/not run: reason and resulting limit
Runtime: witnessed / owner-reported / unattended / not run; exact build if known
Published: pushed branch/commit, CI/release URL and actual state when applicable
Acceptance remaining: criterion + next action
Closure: criteria evidenced, or keep open with Refs #N
```

Report failure paths as well as the successful path when relevant. If a repair
reveals another defect, record/link it separately rather than widening the task
silently. Tests passing does not close runtime-bound criteria. Release availability
does not identify an installed binary. Update PLAYER_DEBUG for player work as
required by the active constraints. Close only the criteria actually satisfied.

## Git completion checklist

1. **Establish ownership and baseline.** Run preflight, read the issue and inspect
   worktrees, tracked and untracked changes. Use an authorized fetch to establish
   remote state; local tracking refs have no freshness guarantee. Confirm owner
   identity before committing. Preserve unrelated staged and unstaged work.
2. **Keep one coherent change.** Define affected paths and exclusions on the issue.
   Split materially different work. Use a proportional PR for cross-module/media/
   FFI/backend integration; a small authorized verified change may land directly.
3. **Review before staging.** Inspect the actual diff, including encoding, generated
   files, captures and private data. Stage only exact task paths or exact hunks if
   a file also contains unrelated edits. File-level staging is not safe merely
   because the filename belongs to the task.
4. **Review the index.** Inspect `git diff --cached --name-only`, the staged diff
   and `git diff --cached --check`. Ensure the index contains only this task. Do
   not consume another writer's staged changes, run blanket staging or use an
   automatic commit-all flag. Required checks refer to the candidate being landed;
   rerun affected checks when later edits invalidate them.
5. **Commit and publish.** Use `type(scope): summary`, with `Refs #N` while criteria
   remain open and `Fixes #N` only when closure is justified. Preserve owner
   authorship and no-attribution rules. Commit and push completed authorized work
   without repeat permission. Do not create binary versions/tags for docs alone.
6. **Verify and hand off.** Confirm the pushed commit against the intended remote
   branch, inspect post-commit status, and distinguish task leftovers from preexisting
   changes. Record commit/PR, checks and remaining acceptance on the issue in the
   same session. CI queued/running/failed is recorded as such; no green claim until
   the exact candidate's required jobs have succeeded.

On push rejection, inspect remote advancement and overlapping changes. Preserve
local commits and other worktrees. Do not force-push, reset away work, amend
someone else's commit, merge/rebase blindly or retarget tags. Reconcile an
authorized integration with fresh checks; report a genuine conflict or unresolved
authority. A failed push leaves work committed locally, not published. A failed
tracker update leaves publication and tracker synchronization as separate states.

## Adapted sources

Tenon (product name; repository opencode-mobile) workflow records at inspected
HEAD `e9cd5e0477f5ed6de124991a0012b93fcab99c86` contribute issue-sized work,
layered verification and evidence carried on the issue. Canonical Eternal at
`H:\REA\eternal`, inspected HEAD `ddc29e74c0dbbc12a7d4e9eed6822556cfc187ad`
with concurrent dirty work, contributes same-session actionable tracking,
sanitized reports, observable closure and completed-work synchronization. These
are source-document observations, not qualification of their application code.
No source-project files changed or tests ran for this adaptation. VT retains
its own chat-approval provenance, Go/Fyne checks, branch/release policy and media
acceptance; no phone, upstream, security or toolchain gates are imported.
