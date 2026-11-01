# VideoTools development rebaseline — 2026-10-31

Scope: the owner requested development-state reconciliation and adoption of
evidence-led planning, testing and GitHub discipline from the newer projects.
The follow-up explicitly prioritized Issues over PR ceremony for a solo project.
This is documentation/tracker work (#30), not approval to port code or rewrite
VideoTools. Ordered work: [#29](https://github.com/LeakTechnologies/VideoTools/issues/29).

## Baseline actually inspected

- The initial clean checkout was local master at `ce3438ab`, VERSION dev93, fifteen
  commits behind origin/master. A fetch confirmed no local-only commits. A
  fast-forward restored the existing dev101 tree at
  `57704aad3e3edce87e2171ea37859818fe559cb0`; no history was rewritten.
- [VideoTools v0.1.1-dev101](https://github.com/LeakTechnologies/VideoTools/releases/tag/v0.1.1-dev101)
  was published 2026-10-13T22:11:54Z with Windows zip and Linux tarball. Artifact
  availability was rechecked through GitHub. This does not identify the installed
  application or provide fresh runtime acceptance.
- The released work includes source-owned player sessions, Convert's metadata-first
  optional preview, compact Rip title names/cards, chapter choices, bounded optical
  previews and contextual help. The detailed evidence remains in PLAYER_DEBUG,
  UI_SUITE_AUDIT and UI_CONTEXTUAL_HELP; past test results were not rerun here.
- Open [PR #28](https://github.com/LeakTechnologies/VideoTools/pull/28) has candidate
  `da3612a88f0d376d83a7c388934d67bfd3c4a9bc`, conflicts with master, and successful
  historical Linux/Windows checks from 2026-10-13. It includes DVD preparation,
  auxiliary decoding and interlace/readiness work. It is not merged, rebased,
  independently reviewed or accepted by this intake. Another worktree remains
  separate and was not changed.

## Current work and acceptance gaps

| Work | Evidence and honest state | Tracking |
|---|---|---|
| Encrypted DVD -> Rip -> Convert -> playback | Still unaccepted. One recognizable encrypted optical preview is not a full rip or playback result. | #3, #26, #1 |
| Cell fallback mapping | Source still maps logical VOBID to a physical VOB filename and clamps global cell sectors to that file. The preview mapper already handles global VTS sectors. Preserve chapter semantics while repairing addressing. | #31 |
| VT_Player Clear | View clears its local options; host state is supplied again on rebuild. Source-confirmed, not a new UI reproduction. | #32; inspect #11/#20 and PR #28 overlap |
| CSS decryption/output verification | Calls are present at executor.go:647 and :1100, with helpers at :1414 and :1433. Original issue descriptions claiming absence are stale. End-to-end and failure-path acceptance remain open. | #26, #25 |
| Convert source/control state | Implementations and historical acceptance notes exist, but current combined acceptance must be reconciled. | #11, #20, register #17 |
| ISO preview workload | Source resolution still extracts an ISO before bounded preview sampling begins. | #34 |
| Many-title viewport, localization, command preview, two-pass | Existing issues need their own evidence and criteria; no batch closure. | #27, #22, #24, #23 |
| libVLC proposal | Implementation was removed in dev84. Historical Phase 1/2 tickets are held design work, not a shipped backend or an active replacement mandate. | #4, #5 |
| Updater, static-sidecar cleanup, Burn/UDF | Retain existing sequencing and approval boundaries. | #6, #7, #8, #9 |
| Roadmap text encoding | 358 pre-existing encoding-marker occurrences match the released baseline. Repair needs intended-codepoint and rendered evidence, not blanket decoding. | #35 |

Correctness blockers precede the real-media ladder. Reconcile PR #28 against the
current baseline before new overlapping player implementation. Rust work is a
separate design track and cannot silently supersede that ladder.

## Lessons adopted, adapted and excluded

| Source | Adopted for VideoTools | Deliberately not imported |
|---|---|---|
| Canonical Vectra | Verify checkout/remote identity; preserve accepted/proposed distinction, provenance and bounded authorization; report evidence and open gaps concisely; never turn self-review into certification. | Security-stage gates, its assurance tiers or root-specific execution restrictions; proposed documents as automatically accepted policy. |
| OpenCode Mobile | Issue before code, explicit approach approval/provenance, one ordered worklist, implementation/release/device states, focused commits and meaningful evidence on the issue. | Phone-specific acceptance, Expo/runtime policy and repository-specific no-publication defaults. |
| OpenCode Upstream | Concise focused PRs, real production-component tests, boundary validation, semantic theme roles and compatibility checks; no speculative dependency/framework expansion. | Bun/Effect commands, branch naming, root-test restrictions, upstream maintainer authority or automatic permission to contact upstream. |
| Eternal | Typed media facts, stable source identity, separation of platform APIs from domain core, explicit ownership/lifecycle, local-first behavior and preservation of uncertainty. | Music-specific schema, approved Rust/Slint stack decision as a VT mandate, or historical tests as current VT evidence. |
| Agent workflow template | Disjoint write ownership and coordinator responsibility when parallel work is useful. | An additional baton/queue framework, role roster and scripts without a demonstrated coordination need. |

The initial `vectra` folder was inspected before canonical identity was learned.
`vectra-git/AGENTS.md` identifies that older non-Git folder as an obsolete snapshot.
Its text is excluded from the adopted-policy source set.

## Eternal reuse: actual candidates and limits

Eternal HEAD was `1f7f7b117c7ad60f32199d24b8ac9fd32ecc30e4`, but its working tree
contains modified app/audio/UI files and an **untracked player.rs**. The observations
below describe inspected working bytes, not that commit alone. No source-project
files were edited, built, tested or imported. Before reuse, the owning project must
freeze/qualify the selected source and its provenance.

| Candidate | Inspected source | Assessment |
|---|---|---|
| Typed inspection boundary | eternal-core/src/lib.rs:238 | Good boundary pattern; preserve VT's additional video/DVD/chapter/subtitle facts. |
| Audio metadata reader | eternal-scan/src/reader.rs:60 | Bounded first-port candidate behind a VT-owned interface; supported-format and resource tests required. |
| Incremental scan/identity | eternal-scan/src/persist.rs:22; eternal-storage/src/worker.rs:71 | Reuse suitable identity/worker ideas; avoid importing the music-library projection or whole schema. |
| Audio decode/output | eternal-audio/src/decode.rs; symphonia_decode.rs; wasapi.rs | Useful audio components, not a qualified video/DVD engine. Windows WASAPI evidence is recorded historically. |
| Player ownership | eternal-audio/src/player.rs:281-351 | One owner thread is useful. Current Linux default selects NullSink; unbounded channels and synchronous shutdown join need review. |

Paths above are relative to Eternal's `crates/`. Application code at
eternal-app/src/main.rs:88, :128, :944 and :1029 wires player, persisted scanning
and events, despite older README/TODO/design text saying they are absent. Source
presence and runtime success are different claims. Startup also waits for a scan
reply (:108); it does not establish a responsive, progress-reporting scan.

Before adopting its audio transport, investigate position based on submitted
frames (player.rs:123), Symphonia seek-result/decoder-reset handling
(symphonia_decode.rs:127-140), presentation timing, close/cancellation and bounded
queues. Historical Windows checks do not qualify Linux output, gapless playback,
physical device removal, video sync or DVD navigation. Video, DVD and conversion
are not provided by the inspected audio stack.

VideoTools is primarily Go/Fyne with a small Rust static-library scaffold.
`internal/ffi/src/lib.rs:11-27` formats an acknowledgement, not media metadata.
`internal/app/modules/bridge/bridge.go:5` links ntdll unconditionally, and the
inspected call sites are tests. A passing round-trip test cannot establish media
inspection, cross-platform linking or a production integration.

Recommended next architecture unit, [#33](https://github.com/LeakTechnologies/VideoTools/issues/33):
design a versioned Rust metadata-inspection boundary with exact supported facts,
typed errors, input encoding, allocation/free ownership, cancellation and Windows/
Linux build acceptance. Compare Go retention, a Rust core through C ABI and a later
application migration using compatibility, deployment cost and measured behavior.
No full Rust or Slint migration, backend switch, dependency addition or port is
authorized by this report. Preserve source/dependency licence and copyright
notices; the manifests' GPLv3-family inventory is not a completed licensing review.

## Source identities

| Source | Git identity inspected | Qualification |
|---|---|---|
| VideoTools | 57704aad3e3edce87e2171ea37859818fe559cb0 | Clean released baseline before this documentation task. |
| vectra-git | 19b4457d23e76d2d77115e0192ad8d6b340b401e | Canonical remote verified as LeakTechnologies/vectra; source documents carry their own accepted/proposed status. |
| opencode-mobile | 3cc9b2dfc9a17d3cba35ee4c396c9da608dd94d0 | Local AGENTS and OPERATIONS inspected as adaptation sources. |
| opencode-upstream/opencode | 39572e3e128b47c892932fad53ddbee30bdc624c | Local AGENTS, CONTRIBUTING and the external MOBILE-HANDOFF inspected; no upstream request sent. |
| Eternal | 1f7f7b117c7ad60f32199d24b8ac9fd32ecc30e4 + dirty working files | No committed/qualified player-source claim; freeze exact bytes before any port. |

## Checks and exclusions

Performed: repository/remote/branch/worktree/dirty-state inspection, fetch,
fast-forward ancestry check, release asset inspection, issue duplicate/history
review, PR status/files/check identity review, and source/document comparison.
Documentation/template/link/roadmap checks and the publication commit are recorded
on #30 with actual outcomes after editing.

Editing checks passed: UTF-8 round trip for the ten task files; issue-form YAML
parse and required-field/unique-ID checks using the existing local js-yaml parser;
60 local Markdown links; one roadmap JavaScript block parsed by Node's VM;
232 unique capability IDs and the new cards' document targets; unchanged
application source/version files; and `git diff --check`. The historical roadmap
encoding-marker count is unchanged at 358 before/after, tracked separately in #35.
These are document checks, not a rendered-browser or application acceptance pass.

Not run in this documentation-only task: application builds/vet/tests, Rust checks,
media playback, rip/convert jobs, benchmarks, hardware tests or independent security
review. No observed application failure was marked repaired, no media acceptance
was invented, and no other project's working files were changed.
