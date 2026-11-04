# VideoTools Agent Workflow Rules

Optional-preview candidate: [AUTOMATIC_PREVIEWS.md](docs/AUTOMATIC_PREVIEWS.md), #43 under #37. Explicit automatic-session admission/retirement is isolated on a PR branch; no Settings switch/import suppression or new playback acceptance is claimed. Preserve manual-session ownership during later integration.

Every feature/redesign/repair follows the [evidence-to-decision planning contract](docs/ENGINEERING_WORKFLOW.md#every-feature-plan-starts-with-evidence): inspect current behaviour, justify the approach, expose unknowns and define observable success before code. Decision-critical gaps require bounded investigation; never invent evidence.

Document actionable failures and Git completion using [ERROR_REPORTING.md](docs/ERROR_REPORTING.md). Update the matching issue in the same session; separate observation, diagnosis, check outcome, publication and runtime acceptance. Preserve unrelated staged/unstaged work.

Suite audit and design direction: [2026-11-04 audit](docs/UI_SUITE_AUDIT_2026-11-04.md), #39. Shared visual language follows task priority; historic fixed chrome/splits may change with evidence. #37 is the next owner-requested processing-workflow delivery; retain correctness dependencies and locked/deferred scope.

## Evidence-led engineering — owner direction 2026-10-31

Read [ENGINEERING_WORKFLOW.md](docs/ENGINEERING_WORKFLOW.md) before new work and
the [dated development assessment](docs/DEVELOPMENT_REBASELINE_2026-10-31.md) when
resuming this project. This section and that workflow supersede conflicting
historical workflow wording; product decisions and regression constraints
remain intact. The owner authorized this rules/tracker adaptation, not a rewrite.

- **GitHub Issues lead:** [worklist #29](https://github.com/LeakTechnologies/VideoTools/issues/29)
  is the ordered queue. Before implementation, an issue records evidence, a bounded
  approach, affected paths, exclusions, success criteria and owner approval with
  honest provenance. Existing explicit chat approval can be recorded; silence and
  an agent-authored approval record are not an owner-authored GitHub comment.
- Keep proposed, approved, implemented, verified by checks, released and verified
  in use separate. No issue closes solely because code exists or CI is green.
  Record exact candidate/check outcomes and not-run/runtime gaps on the issue.
- PRs are proportional for a solo maintainer; use them for substantial media/FFI/
  backend or cross-module integration candidates. Small authorized verified work
  can land directly. Conflicting PR #28 is unmerged historical candidate work.
- Commit/push completed authorized work without repeat permission. Inspect and
  stage only exact task files; preserve unrelated changes/history. No force pushes,
  blanket staging or tag retargeting. Reference the issue in conventional commits.
- Documentation/tracker-only work does not bump the application version or create
  a binary release. User-facing changes retain the existing dev-release cadence.
- Rust/Eternal reuse is a design proposal (#33). Pin qualified source, dirty state,
  dependency/licence provenance and platform evidence first. The current Rust
  string bridge is not a media engine. No Rust/Slint rewrite or backend change is
  authorized by this governance task. Go/Fyne and InlineVideoPlayer remain current.
- Use UTF-8 deliberately and review encoding-sensitive diffs. Do not silently
  recode whole files or import pre-existing mojibake while making a small edit.
- One implementation writer by default; parallel tasks require disjoint file
  ownership. Reports stay brief, with durable evidence and explicit open gaps.

Current application baseline: **VideoTools v0.1.1-dev101** at `57704aad`, published
2026-10-13. Rebaseline task #30 completed documentation/tracker changes only. Next
correctness work is #31 (Rip cell addressing), #32 (Clear source), PR #28 integration
assessment and the encrypted-media acceptance ladder; details/order live in #29.

## Media ecosystem direction — owner decision 2026-11-01

VideoTools retains Go/Fyne. Adopt qualified concepts from Eternal and the Rust
applications through bounded Go changes or separately qualified shared components;
no full Rust/Slint rewrite. Eternal may support video and other media: emphasize
library/organization/consumption there and analysis/transformation/comparison/disc
workflows in VT, with useful overlap and independent operation. Manual VT_Player
and Inspect without playback remain required by #37. See
[ecosystem design](docs/MEDIA_ECOSYSTEM_DESIGN.md) and #33 for provenance,
component qualification, deliberate handoff and acceptance. This direction does
not itself select a replacement backend, dependency or FFI implementation.

## Product and module naming — strict

- **VideoTools** is the product; **VT** may be used as its abbreviation. Every version, release, tag and published build belongs to VideoTools: `VideoTools v0.1.1-devN`.
- **VT_Player** is a module within VideoTools. Use this exact spelling in prose. Describe its changes as “VideoTools v0.1.1-devN — VT_Player improvements”; never describe a suite version as “VT_Player devN” or imply a separately versioned player product.
- Preserve established code identifiers, package paths and application metadata. This naming rule does not authorize unrelated renames or a separate player release.


## Required reading and preflight

Read [active constraints](docs/AGENT_CONSTRAINTS.md) before implementation and the
linked module plans before touching those modules. Product, platform, player API,
Rip regression, privacy, localization, verification and release rules remain binding.

Run `pwsh -NoProfile -File scripts/tools/repo-preflight.ps1` from this checkout.
The manual helper reports local evidence only. It does not fetch or prove remote
freshness, issue approval, CI, installed builds or runtime acceptance. Use an
authorized fetch and the active issue to establish remote/task state. Inspection
failures are unknown, including unreadable submodule state.

## Current evidence and next work

Released application baseline is dev101; verify current HEAD independently.
Follow [worklist #29](https://github.com/LeakTechnologies/VideoTools/issues/29)
and each issue's approval and acceptance criteria. #38 adds preflight and separates
active rules from history without changing application behavior. #31 Rip addressing,
#32 Clear source and PR #28 reconciliation precede the remaining encrypted
DVD -> Rip -> Convert -> playback acceptance ladder.

#37 retains manual VT_Player and Inspect without playback; automatic previews
must be optional. Convert metadata outranks preview. Readability, coherent module
language and SMPTE branding remain requirements. Skip locked modules.

DVD playback remains unaccepted. The 2026-10-13 optical session reported readiness
failure, zero cached PTS, bwdif -11 and auxiliary-source failures. These are
observations, not root causes. Historical alternate-container acceptance does
not qualify DVD playback. Read [PLAYER_DEBUG](docs/PLAYER_DEBUG.md),
[DVD plan](docs/VT_PLAYER_DVD_NAVIGATION.md),
[player upgrade plan](docs/VT_PLAYER_UPGRADE_PLAN.md) and
[suite design](docs/UI_SUITE_DESIGN.md) for relevant work.

## History and maintenance

The [exact prior snapshot](docs/history/AGENTS_2026-11-03.md) preserves release
narratives and superseded instructions as historical evidence only. Its relative
links retain their original root context. DONE.md and docs/CHANGELOG.md record
shipped history. Keep this entry point focused on current authority and constraints;
do not append release diaries. Preserve human credits and commits as specified in
the active constraints.