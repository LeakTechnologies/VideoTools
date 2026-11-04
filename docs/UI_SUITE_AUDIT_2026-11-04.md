# Suite workflow and design audit — 2026-11-04

Tracking: [audit #39](https://github.com/LeakTechnologies/VideoTools/issues/39),
[ordered work #29](https://github.com/LeakTechnologies/VideoTools/issues/29).
Owner approved the suite audit, consistent visual language, optional processing
previews and reconsideration of rigid layouts in chat. This is an agent-recorded
approval provenance, not an owner-authored GitHub comment. Detailed layouts below
are recommendations, not implemented screens or blanket backend authority.

## Baseline and evidence limits

Source: clean master `3d1e8d0f039812b15756cba0156f95f47f1d676e`; fetch confirmed
HEAD and origin/master agree. Application VERSION remains dev101. Local master
has no configured upstream; the preflight reports that explicitly. Restricted
submodule inspection failed with access denied; no clean submodule claim follows.

The existing running app in this checkout displayed VideoTools v0.1.1-dev101_win
on its menu and dev101 in Settings; Settings identified its hash only as
"development build". This identifies the visible version, not an exact binary
commit or a fresh build. The existing executable SHA-256 is
`B21C47237B74D68525DFA80B90CAA70BADC8F33A46E03C1C62DB0A922BC1A6F7`.
No binary was rebuilt or installed for this audit.

Fresh visual observations: menu, Convert, Merge, Inspect, Compare, Rip, Author
Videos tab, Thumbnail, VT_Player, Settings Preferences, empty Queue and empty
History drawer. Initial menu capture was about 803x661; subsequent workspace
captures were about 1175x665 or 1175x719. Current visible font is the VCR-style
face. Capture dimensions are not monitor resolution or exhaustive scaling tests.
No private media was opened; no jobs were submitted or preferences changed.
Screenshots were inspected through computer-use; raw captures are not committed.

Source inspection supplements these observations. The older
[suite audit](UI_SUITE_AUDIT.md) remains historical evidence with its original
limitations. Loaded media, rendered tooltips/focus, all tabs, font/language/scaling
combinations, output correctness and resource measurements remain unverified here.
This is a refreshed task/design audit and empty-state visual pass, not completed
full module acceptance. Keep #39 open until the relevant evidence is collected.

## Working practices: provenance and adaptation

The running Eternal app revealed its canonical checkout at `H:\REA\eternal`.
Its AGENTS identifies `E:\Development\eternal` as a divergent historical checkout.
Read-only Git inspection confirmed canonical remote LeakTechnologies/eternal and
HEAD `ddc29e74c0dbbc12a7d4e9eed6822556cfc187ad`, with dirty app/theme/Android and
other files. No changes, builds or tests were performed in either checkout.
The October VT rebaseline's E-drive inspection remains a dated historical record;
it must not qualify current Eternal code or behaviour. Revalidate #33 against
canonical source before any reuse.

Canonical Eternal AGENTS contributes: one issue per actionable unit, sanitized
evidence, same-session tracker updates, preserve concurrent work, observable
closure criteria and real visual/device checks. Adapt these practices to VT's
existing issue-led workflow. Do not copy Android/security gates, library schemas,
Rust/Slint requirements or CD transport as VT requirements. Its historical visual
spec offers a useful semantic-style pattern; its palette/font is not VT's design.

The owner clarified that Tenon is the rebranded OpenCode client in
`E:\Development\opencode-mobile`, an independent product rather than upstream
OpenCode. README confirms the product name while repository/package identifiers
remain unchanged. Read-only Git inspection confirmed remote
LeakTechnologies/opencode-mobile, clean HEAD
`e9cd5e0477f5ed6de124991a0012b93fcab99c86`. Read AGENTS, OPERATIONS and
ISSUE-TRACKING: adopt one ordered queue, issue-sized changes, explicit authority,
evidence on the issue, layered checks, semantic styles and milestone review of
usability/reliability/compatibility/open device evidence. Preserve VT's existing
chat-approval provenance rule; do not import Tenon's stricter owner-comment
requirement, mobile release/toolchain commands, branch naming or hardware gates.
Tenon's published/received/verified distinction maps to VT's artifact/installed/
verified-in-use states. Its client semantics reinforce truthful task state; they
are not a reason to add OpenCode integration to VT. No Tenon files were changed.

## Shared design contract

1. **Source and scope stay visible.** Show source identity, selected item/count
   and whether edits apply to current, selected or all items. One accessible Load
   action stays available with preview closed. Clear selection, remove source,
   remove history and cancel processing have distinct meanings.
2. **Processing owns its input.** Metadata and immutable queued job settings do
   not depend on an active decoder. Probe first; explicit expensive analysis and
   image generation show progress/cancellation. Playback controls do not silently
   change output stream selection.
3. **Preview is a policy and a layout choice.** Global automatic-preview setting
   (#37) controls preparation; per-module open/closed state controls allocation.
   Hidden is not synonymous with unloaded. Manual VT_Player remains available.
   Preserve approved Compare/Upscale exceptions and do not unlock held modules.
4. **One visual vocabulary.** Use existing shared components first. Define semantic
   roles for background/surface, primary/secondary/disabled text, module accent,
   focus, selection, warning and failure. Module color identifies the tool; color
   alone never carries status. Preserve selected font/size. Measure contrast when
   tokens change; do not assume every spectrum color works for ordinary text.
5. **One task hierarchy.** Consistent source area, task workspace, output/readiness
   summary and primary action; secondary/advanced actions group contextually.
   No required fixed split, permanent footer height or identical module layout.
   Keep critical warnings and recoverable actions visible; avoid rows of idle
   commands and disabled chrome consuming the useful workspace (#40).
6. **SMPTE is retained.** Contextual branded source/preview affordance with Load
   and optional Preview actions. Its footprint follows the task. It must not force
   player resources to initialize or displace essential metadata.
7. **Help is contextual.** Short labels and necessary validation stay visible;
   explanations are available by hover, focus and explicit help activation.
   Do not hide current state, scope, destructive consequences or errors in help.
8. **Responsive means reflow.** Wrap actions, stack panels or move secondary
   actions into labeled menus as width decreases. Preserve reading size and
   scroll the task area. No fixed percentage may make a collapsed header vanish.
9. **Results are part of the task.** Show effective output, destination, readiness,
   actual job state and a deliberate next action: inspect/open/compare/show folder.
   Use the same state vocabulary across modules, Queue and History.

This explicitly relaxes old chrome/split/component arrangements. It preserves
correctness, accessibility, media privacy, i18n, deployment and resource ownership.
It does not authorize a framework, dependency or backend replacement.

## Module matrix

V = fresh visual observation above; S = current source inspected; H = historical
audit only. Recommendations require loaded/task checks before implementation or
acceptance. Rows define coverage, not competing task order outside #29.

| Surface | Main task and priority | Evidence / recommendation | Required proof / existing tracker |
|---|---|---|---|
| Convert | Batch/source list, output intent and settings first; optional input review | V/S: compact preview exists, metadata collapsed in current empty state, dense bottom actions and idle status consume space. Import still prepares frames. Show current/selected/all scope and effective output before queueing. | #37 first delivery; #11/#20 lifecycle/state overlap, #24 preview/executor parity, #23 unsupported two-pass, #22 i18n. Test one/batch, mixed errors, collisions, settings scope and output parity. |
| Rip | Intended title/chapter content, tracks and destination | V/S: clear source and title browser; idle Open in Player/Rip/Queue actions present without content. Group content mode/chapter choices with selected title; show exact filenames/ranges before execution. Existing compact cards/thumbnail fixes remain constraints. | #31 addressing, #27 many-title viewport, #34 ISO I/O; #1/#3/#25/#26 real selected-content and failure acceptance. No fresh scan performed. |
| Queue / History | Trust job state, recover and find results | V/S: empty Queue retains Start/Pause/Resume/Clear controls and large Live Output area; History opens an empty drawer. Make logs contextual, separate cancel from clearing records, preserve source/output identity. | #40 shared space; audit pending/running/success/failure/cancel/retry plus result routing. Existing callbacks are not proof every executor supports pause. |
| Inspect | Read/copy/export facts and deliberate deep analysis | V/S: half-width idle player beside empty information; Copy Metadata/Export visible. Fixed split and player-bound load milestone callback remain in view.go. Prioritize facts; expose deeper analysis separately from playback. | #37; test facts/chapters with no engine, accurate units/unknowns, replacement/cancellation, exports and disc-vs-stream identity. Historical ISO mismatch not re-reproduced. |
| VT_Player | Explicit open and reliable viewing | V/S: SMPTE gets primary space, File row and Clear visible; no separate Open button on opening screen. Make explicit keyboard-accessible Open discoverable; source and controls follow actual ready state. | #32 Clear; PR #28 and PLAYER_DEBUG acceptance. File/disc open, seek, tracks, close/reopen and resource retirement. Empty screen does not qualify playback. |
| Merge | Order clips, understand copy/encode compatibility, output | V/S: large clip area and scrollable options; Fast Merge shown with frame-rate/interpolation options. Keep ordered list primary, disclose compatibility and applicability from executor facts. | Multi-clip order, long names, mixed formats/streams, copy/transcode output, cancellation and chapters. Applicability ambiguity is a design question, not a newly confirmed encode defect. |
| Compare | Identify original/result and align comparison | V/S: File 1/File 2, independent source actions, Hide Player and Fullscreen Compare visible empty. Prefer explicit roles, property differences and sync/offset policy. | #37 exceptions; pair load/replace, manual fullscreen, temporal alignment, no wrong-source session reuse. No sync acceptance in this pass. |
| Thumbnail | Choose mode/count/timing/size and inspect generated images | V/S: third mode radio circle is visible but Both text is clipped; settings fixed at 0.32 width. Reflow mode choices and show predicted outputs before Generate; preserve useful result area. | Fresh evidence attached to #2; #22 strings. Individual/sheet/both, count/time/destination, large fonts and actual generated output. |
| Author | Content -> disc settings -> menu/chapters -> validate/generate | V/S: seven tabs, Videos tab has Compile to DVD plus footer Generate DVD and queue action. Clarify semantics/readiness; prioritize content/capacity and a single clear primary stage action. | Author task slice under #39; inspect handlers before consolidation. Neutral multi-title project, capacity, menu navigation, output validation and later author->rip milestone. No generation performed. |
| Settings / navigation | Find preferences and return without losing work | V/S: long Preferences page starts with Updates/Language; hash says development build. Group Playback/Performance clearly for #37; display persistence and scope; keep appearance accessible. | #37 preference migration/persistence; #6 build identity. Fonts/locales, keyboard, restart, reset, source preservation. No settings changed. |
| Files | Navigate source and route deliberately | V: Files entry visible in menu; not opened in this pass. H: earlier report called standalone Files unexposed. Do not equate entry presence with full module availability. | Pending chooser/navigation/access-error and routing checks; sanitized neutral fixtures. Do not claim this surface audited visually beyond menu availability. |
| Upscale | Source/model/scale, explicit preview and resource/output estimate | Inventory only. Menu tile is unlocked, but detailed audit remains deferred until encrypted-disc ladder. | Retain deferred status; later test quality/resource model/device/compare and cancellation. |
| Audio / Subtitles / Filters / Trim | Extraction/stream work, filters, temporal editing respectively | Menu locks observed; mainmenu_module.go lockedModules confirms all four. Inventory retained in older audit; no implementation or loaded audit. | Skip until separately unlocked/authorized. Do not infer availability from source packages. |
| Burn | ISO/media/drive, write/verify/eject | Locked tile observed. Inventory only; no media/drive actions. | #8/#9 future scope; no unlock or burn acceptance. |

## First implementation work order: #37

The owner endorsed this delivery in the current conversation. Record provenance
on #37; this audit does not itself complete the feature. Use one cross-module PR
because source/session policy spans host, shared UI and processing entry points.

**Source-confirmed preparation:** main.go loadVideo captures preview frames after
probe; loadMultipleVideos captures the first source, opens native playback and
spawns frame captures for remaining sources. batchAddToQueue probes sources but
also calls generateJobThumbnail. Inspect import at main.go loads the primary
player and captures analysis frames. native_media.go constructs primary/peer
InlineVideoPlayers in init and supplies automatic Load entry points. Inspect
view.go subscribes to player load milestones and fixes a 50/50 split; collapsing
uses 0.03. Convert's collapse handler pauses playback but does not retire all
preparation. These are actual boundaries to inventory, not evidence of whole
movies resident in RAM. No latency or RSS measurement was collected.

**Bounded approach:** persistent automatic-preview preference with legacy default
enabled; one host-owned policy; guard automatic import/rebuild/thumbnail/scrub
work at its owning boundaries; retire automatic work on disable/replacement;
preserve processing inputs, queued plans and explicit manual VT_Player. Make
Inspect facts independent of Ready, with explicit deep-analysis actions. Inventory
all native/stub/direct Compare and Upscale exceptions before code. Compare source
and PR #28/#32 overlap first; no blind merge of the old transport worktree.

**Acceptance:** off persists across restart and old configs keep the agreed
default; metadata and batch jobs remain equivalent; no automatic preview engine,
scrub, thumbnail or audio work when off; manual VT_Player still works; toggle,
navigation, rebuild, replacement and cancellation cannot publish retired sources.
On prepares only intended visible preview. Measure cold/warm time to usable
metadata and queue, peak working set, background FFmpeg count and retirement for
one long source and a batch with identical inputs/settings on/off. Record source
structure/build/platform/cache conditions. Do not set invented performance gains.
Native build/vet/full tests, non-native build, real playback/log review and
production UI checks precede release; full closure needs actual task acceptance.

## Accountable delivery and remaining sequence

Each implementation issue carries: problem/evidence, bounded write-set and
exclusions, owner authority, expected observable outcome, planned checks, exact
candidate/results, remaining gaps and next acceptance owner/action. At a milestone
update the issue and #29 in the same session. A matrix cell cannot turn green
solely because code or a screenshot exists. No agent self-certifies the system.

For each module collect empty/loading/ready/error and relevant running/cancelled/
completed states, preview on/off where applicable, source replacement and
keyboard use. Check a narrow usable window, current window and wide/maximized
window with supported fonts, large text, relevant languages and Windows/Linux.
Keep a readable minimum; record unsupported combinations rather than shrinking
the chosen text. Use neutral fixtures with known audio/subtitle/chapter content;
retain raw private captures outside Git. Successful processing requires checking
the actual intended output content, not just a nonempty file.

Delivery ordering: #37 is the next owner-requested processing-workflow unit.
Rip #31, Clear #32 and PR #28 correctness/overlap assessment remain explicit
dependencies and must not be buried by cosmetic work. Then qualify Queue/result
handoff and the DVD->Rip->Convert->playback ladder; pilot shared chrome #40 in a
bounded Convert change; follow with loaded Inspect, Merge, Compare, Thumbnail and
Author slices. Settings/navigation acceptance accompanies each slice. Do not
start deferred Upscale or locked modules as a side effect of this plan.

## Verification of this report

Performed: current source/task comparison, Git identity/fetch inspection,
canonical Eternal provenance correction and the visual observations above.
Documentation link/UTF-8/roadmap checks and publication identity are recorded on
#39 after editing. Not run: application build/vet/tests, media import/processing,
player acceptance, performance benchmark, Linux UI, exhaustive accessibility,
locale/scaling checks. Tenon workflow records were reviewed read-only; no source-project tests were run. Application version unchanged.
