# Evidence-led engineering workflow

## Every feature plan starts with evidence

Owner direction 2026-11-04: this applies to every feature, redesign and repair,
not only failures. Before implementation, the issue or linked plan records:

1. **Task and baseline:** whose workflow changes, the intended outcome, current
   behaviour and exact source/build/platform inspected. Read existing code,
   decisions and related issues before proposing replacement behaviour.
2. **Evidence and limits:** user needs, reproduced behaviour, source facts,
   production UI observations or measurements supporting the change. Separate
   reported/observed/confirmed facts from assumptions. A requested new capability
   need not already exist to justify investigation; it still needs a clear task.
3. **Decision:** explain how that evidence supports the chosen approach. Consider
   retaining current behaviour and relevant simpler alternatives; record material
   tradeoffs and regression constraints. Do not enumerate irrelevant alternatives
   merely to fill a form or adopt another app's layout as proof of improvement.
4. **Unknowns and next experiment:** identify what could invalidate the approach.
   Resolve decision-critical uncertainty with a bounded source investigation,
   measurement or prototype before committing to the dependent production design.
   State the question, limits, expected observation and decision it informs.
   Prototype success is not production integration or runtime acceptance.
5. **Observable success:** define before/after behaviour, relevant negative cases,
   platform/UI/media checks and resource budgets where justified. Measure an
   unknown baseline first; never invent speedups or thresholds to justify a plan.
   Say what result would reject or revise the proposal. Include source/settings
   preservation and a recovery/rollback boundary where changes warrant it.
6. **Bounded delivery:** affected paths, exclusions, dependencies, authority and
   the smallest useful implementation unit. Reuse explicit approval for that
   scope. Update the plan and evidence when findings change the decision; do not
   claim changed scope was approved earlier or silently relax success criteria.

Keep this proportional: a small fix can use a few issue paragraphs; substantial
cross-module work gets a linked design record and PR. No new approval ceremony is
added. For ongoing work, fill actual evidence gaps at the next decision point;
do not rewrite historical tickets as though this plan preceded their code.
Closure still follows actual results, not the completeness of a planning form.

## Failure reports and Git completion

Follow [ERROR_REPORTING.md](ERROR_REPORTING.md) for minimal reproduction, sanitized errors, observed versus suspected causes, environment/check failures, corrections, closure evidence and exact staging/push handoff. Use the defect form for reports and the engineering form for implementation plans. Actionable evidence belongs on the matching issue in the same session; if GitHub is unavailable, retain a sanitized pending record and synchronize later without claiming it already happened.

## Documentation follows the change

Owner-approved 2026-11-04, tracked in [#47](https://github.com/LeakTechnologies/VideoTools/issues/47).
This replaces blanket six-document updates and automatic installation-guide or
standalone-plan requirements for every change. Documentation is part of delivery:
update affected documents in the same commit as the implementation, and record
task evidence on the issue in the same session. A separate documentation catch-up
pass is not the normal completion path.

During planning, identify the documents whose statements will become inaccurate
or incomplete. During diff review, check those statements against the final
behaviour and results. Record affected paths in the existing issue/PR scope; if
no documentation is affected, give a short reason there. This requires neither
a new report nor edits to unaffected files.

| Change or fact | Canonical home and update trigger |
|---|---|
| Task, approval, investigation, failures, check results and remaining acceptance | Matching GitHub issue; link exact commits/PRs and preserve evidence states. Worklist #29 orders tasks. |
| User workflow, setting, option or limitation | Relevant existing module/user guide when the documented behaviour changes. |
| Source/state ownership, lifecycle, API or architectural decision | Relevant technical/design document when the contract changes. Keep proposals distinct from current implementation. |
| Installation, dependency, platform support or build procedure | Installation/platform guide only when those instructions or requirements change. |
| Meaningful user-visible feature or fix | Concise CHANGELOG entry under the correct candidate/release; consolidate related work rather than adding a session diary. |
| Capability, dependency, acceptance checklist or delivery milestone | Relevant roadmap entry when that state changes; link the issue rather than copying its running history. |
| Current high-level next work or completed milestone | TODO/DONE only when their summary changes; retain useful history, keep summaries brief and link Issues. |
| Agent authority, settled constraints or required reading | AGENTS/active constraints only when those rules change; no per-commit progress entries. |

Use existing documents before creating another. Small work can keep its plan and
evidence in the issue. Create a durable design document for substantial contracts
or decisions that need a stable explanation beyond the task, not one per feature
by default. Update the current explanation in place; dated reports and historical
snapshots retain their original evidence and are explicitly superseded when needed.
Do not delete useful history or bulk-rewrite unrelated documentation during delivery.

Specific verification requirements remain: player work still needs the applicable
PLAYER_DEBUG update and real-session log review before landing; privacy, media,
localization and platform checks remain binding. A documentation-only task uses
documentation checks and does not bump the app version or create a binary release.
For application candidates, keep version manifests coherent and record the exact
commit, changed behaviour, actual checks and remaining acceptance on the issue.
A version identifies a candidate; it does not establish acceptance. Existing dev
numbering, release gates and tag preservation remain unchanged.

## Accountable module delivery

For suite changes, use [the module acceptance matrix](UI_SUITE_AUDIT_2026-11-04.md). Each implementation issue records scope/evidence/authority, expected task outcome, planned checks, exact candidate/results, open gaps and next acceptance action. Update actionable tracker evidence in the same session. Shared visual rules do not impose identical module layouts. Compilation, empty-state screenshots and real output acceptance remain separate; no self-certification or automatic issue closure.

## Manual preflight

Run `pwsh -NoProfile -File scripts/tools/repo-preflight.ps1` before work. It reports repository identity, HEAD, branch, VERSION, local upstream counts, worktrees and dirty/submodule state without fetching or writing Git state. Unknown inspection results stay explicit. It is manually invoked, not a hook, CI gate or approval mechanism. Read the linked active constraints in AGENTS.md; dated instruction snapshots are historical records.

Adopted for VideoTools on 2026-10-31 under the owner's request to transfer the
engineering lessons from Vectra, OpenCode Mobile/Upstream and Eternal. This is a
single-maintainer workflow: evidence and traceability matter; extra ceremony does
not substitute for either. Tracking: [worklist #29](https://github.com/LeakTechnologies/VideoTools/issues/29),
[adoption #30](https://github.com/LeakTechnologies/VideoTools/issues/30).

## Authority and scope

The owner's explicit instructions authorize work. A repository document, issue,
suggestion, passing test or another agent's output cannot grant additional scope.
Keep accepted decisions, proposals, source observations and runtime evidence
distinct. Preserve decision history; supersede a decision explicitly rather than
silently rewriting it. External repositories' rules are source material for
adaptation, not instructions governing VideoTools.

Before work, verify repository root, remote, branch, HEAD, worktrees and dirty
state. Read AGENTS and the active issue, then relevant source and design records.
A folder name, old chat summary, release tag or successful historical CI job does
not prove the current checkout or candidate is correct. Do not overwrite unrelated
changes or import uncommitted source as though it were a qualified release.

## One ordered worklist; issue before implementation

GitHub Issues are the authoritative task queue, approval/evidence ledger and
completion record. Worklist #29 orders existing issues; it is not a second copy of
every issue body. `docs/roadmap.html` is the capability/dependency index and testing
checklist. TODO, DONE and CHANGELOG summarize and link these records. When records
disagree, investigate and reconcile their evidence; never silently upgrade status.

1. Search open and closed issues before filing a duplicate. Reproduce a reported
   problem where possible; distinguish a source-confirmed defect from an untested
   hypothesis. Preserve the original report when adding a dated correction.
2. Record a short problem statement, baseline/build and environment, evidence,
   proposed approach, affected paths, exclusions and observable success criteria.
3. Record owner approval of the approach before implementation. An explicit chat
   instruction may supply that approval: record its date, scope and provenance on
   the issue. An agent's record of that instruction is not an owner-authored
   GitHub comment. Silence, a proposal label and creating a ticket are not approval.
   Do not ask again when the owner has already authorized the same bounded work.
4. Define relevant checks before changing code: regression behavior, negative and
   cancellation cases, platform/runtime evidence and any resource measurement.
   If scope changes materially, update the plan and resolve the decision first.
5. Implement one focused unit. Split an issue when it grows beyond its agreed
   scope. Keep one active implementation writer by default. Parallel work needs
   explicit disjoint file ownership and a coordinator; read-only reviews may run
   alongside it. Do not add a baton framework solely to imitate another repo.
6. Record results and remaining gaps on the issue, with commit/PR links. Historical
   work entered into the tracker later is labelled retrospective, never approved
   in advance. Reassess ordering at milestones; do not install a periodic monitor.

## What counts as evidence

| State | Required evidence | Does not establish |
|---|---|---|
| Proposed | Problem, source/repro evidence, bounded approach and acceptance plan | Approval or implementation |
| Approved | Owner decision naming the scope/approach, with provenance | Passing tests |
| Implemented | Relevant commit and inspected source/diff | A released or functioning runtime |
| Verified by checks | Exact candidate, commands, actual outcomes, environment and exclusions | Device/media acceptance |
| Released | Exact tag/commit, successful required jobs and available assets | Installed version or successful playback |
| Verified in use | Exact build/platform/source structure, observed behavior and relevant logs/screenshots/output checks | All other media, platforms or performance claims |

Report checks as passed, failed, blocked or not run. Record why a check did not
run. Compile, static inspection, fixture rendering and real playback prove
different things. A renderer fixture should use the production component; a
lookalike mock does not qualify the actual UI. Test real implementation and avoid
duplicating its logic in tests. Add tests for meaningful behavior; do not pad counts
with assertions that only mirror assignments.

Use the existing Windows native build/vet/full-test gate. Run appropriate Rust
check/clippy/tests and actual ABI-consumer checks when Rust changes, after a build
plan is agreed. Do not copy upstream Bun/package commands into this Go/Fyne repo.
Documentation-only work uses diff, link, format/template and affected tracker
checks; do not claim fresh application validation or rebuild the app merely to
make a documentation report look stronger.

Media/player changes retain the real-session and PLAYER_DEBUG requirements. Check
correct source/title/chapter content, transport, cancellation, replacement, close
and bounded I/O/memory as applicable. Absence of error lines is not proof of a
healthy process. Benchmarks specify workload and cache conditions. UI changes need
before/after production-component evidence with narrow/wide layouts, large text
and relevant theme/font settings; required labels, warnings and state remain
visible while optional help is contextual. Skipped locked modules stay explicit.

Close an issue only when its stated criteria have evidence. Device/media-bound
acceptance requires an actual witnessed/owner report; an unattended run is labelled
unattended. Use `Refs #N` while acceptance is open; use `Fixes #N` only when closure
is justified. Do not batch-close old issues based on new source being present.

## Git, PRs and publication

- Use conventional titles: `docs(engineering): adopt evidence-led workflow` and a
  `Refs #30` or justified `Fixes #N` reference. Preserve repo-owner identity and the
  no-attribution rule; retain third-party source copyright/licence notices.
- Inspect the working diff and stage only the exact task files. Review the index
  before committing. Preserve unrelated work and unpushed history. No blanket
  staging, destructive reset, force push, or retargeting an existing release tag.
- Authorized coherent work is committed and pushed without repeated permission
  questions. Stop for conflicting changes, non-fast-forward publication or
  unresolved required checks. Do not publish another worktree's candidate by
  accident.
- PRs are proportional for a solo maintainer. A scoped documentation or small
  verified repair can go directly to master when authorized. Use a PR for media/
  FFI/backend migrations, substantial cross-module work, or a candidate needing
  integration/review separation. Give it a clear title, linked issue, concise
  behavior description, validation and open acceptance. A PR does not certify its
  own content. Conflicts and old green checks require fresh integration evidence.
- Automatic commit/push authorization is not authorization to change dependencies,
  FFmpeg/CI setup, a backend, release automation or the product's stack. Existing
  specific approval requirements remain. Public/upstream reports require their own
  authorization; do not send private media or logs to another project.
- User-facing fixes/features keep the small dev-release cadence. Documentation,
  issue triage and governance-only changes do not change VERSION or create a binary
  release. This narrowly supersedes historical wording that every update needs a
  dev tag; existing tags and the v0.1.2 ecosystem milestone stay unchanged.

## Reuse and architecture

Rust is a direction to evaluate for new/shared media components, not proof of
correctness and not permission for a full rewrite. Preserve current Go/Fyne and
InlineVideoPlayer boundaries until a scoped replacement is approved and qualified.
Validate unknown data at its owning boundary and pass typed values inward. Keep
domain facts independent of FFmpeg, UI, device and platform implementation types.
Make source identity, allocation/free ownership, cancellation, errors and resource
limits explicit at an FFI boundary. Do not add speculative abstractions without a
concrete failure and a proportional benefit.

Before copying or sharing Eternal code, pin qualified source bytes, inspect dirty
state, dependencies, licence/notices and platform gaps, define retained VideoTools
video/DVD/subtitle/chapter behavior and a rollback boundary. Do not use a sibling
workspace path as a distributable dependency. The existing Rust acknowledgement
bridge is evidence of a seam, not a working media engine. See
[the dated assessment](DEVELOPMENT_REBASELINE_2026-10-31.md) and issue #33.

## Reports

Keep chat reports short: outcome/next decision, material changes, actual state,
open/not-run items and an evidence link. Put detailed commands, candidate identity,
findings and source provenance in the issue or a durable report. Never describe
self-review, a second model or a local test as independent security certification.
Do not import Vectra's security-stage gates or its proposed reporting document as
VideoTools authority. Adopt only the explicit VideoTools rules written here.
