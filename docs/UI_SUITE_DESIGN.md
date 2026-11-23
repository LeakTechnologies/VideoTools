# VideoTools UI direction

## Processing players and Convert batch workspace — 2026-11-04

Owner clarification in chat: a global Settings choice must fully disable embedded
players outside VT_Player. This is stronger than pausing, folding a panel or
preventing automatic loads. #37 owns the setting and lifecycle integration; the
existing dev102 candidate is partial and must be evaluated against this clarified
contract before release.

- Proposed operator label: **Enable players in processing modules**. When off,
  processing views have no player widget, transport, preview-only placeholder or
  reserved player space, and no embedded playback action can bypass the setting.
  An explicit **Open in VT_Player** handoff remains available and does not change
  the processing source or its settings. Label/localization needs production UI
  review; the established saved opt-out can remain an internal implementation detail.
- Off cancels/joins owned preview preparation and removes playback-only resources;
  navigating, rebuilding or importing must not initialize them or reveal a manual
  session in another module. Preserve manual VT_Player sessions and processing jobs.
  Metadata and deliberate analysis/transformation remain available; generating
  requested thumbnails or conversion output is processing, not an embedded player.
- On permits a review panel for the selected source. Opening/closing that panel
  is a workspace preference subordinate to the global setting. Importing a batch
  must not prepare every file for playback. Enabling must not open the whole batch.
- Compare and other preview-oriented tools need an explicit availability/handoff
  treatment while off; do not silently start their alternate player paths. Keep
  locked/deferred modules locked/deferred and record unsupported tasks honestly.

Convert should expose the batch directly rather than making Previous/Next the
only practical overview. Source at candidate 76d9e4a1 has loadedVideos/currentIndex,
header Previous/Next/count and all-loaded queue actions. This supports investigating
a list-based workspace without establishing that its final layout is validated.
No measured import performance or fresh loaded-window acceptance is claimed here.

Proposed first production-component prototype: an input list with selection and
readable file identity, duration, source format/resolution, readiness and intended
output; details/output settings for the selection; explicit labels distinguishing
changes to one item, selected items or the whole batch. Keep the input collection
distinct from executable queued jobs. Queue actions must show their scope/count;
failed probes must remain visible and must not silently disappear from the batch.
Decide per-file overrides versus shared settings from actual executor/state evidence
before presenting controls that promise unsupported behaviour.

Evaluate one file and mixed batches, long names, failed probes, keyboard selection,
narrow/wide windows and larger text. A dense table is not mandatory: reflow or
progressive detail is acceptable when it preserves readability. Validate output
plans before/after bulk changes and preserve selection/settings across navigation.
Use a list as the next hypothesis to test, not a new permanent layout rule. Broader
suite changes follow module task priorities and the shared language below.

## Header quick settings — proposal, 2026-11-04

Owner suggested a possible cog-only header action opening a compact settings
dialog. Reuse the full Settings preference/policy callbacks; do not duplicate
state. Preserve current task/source/jobs, provide an accessible name and keyboard
access plus hover/focus help, and retain access to full Settings. Exact contents
and layout need task evidence. This is a proposal recorded under #39, not a
control implemented by #46 or authorization to expose incomplete #37 behavior.

## 2026-11-04 audit and delivery contract

The [refreshed suite audit](UI_SUITE_AUDIT_2026-11-04.md) defines shared source/scope, semantic styles, optional preview policy, responsive task allocation, help, readiness and result behaviour. Existing chrome, permanent status/footer heights and fixed split ratios may change with evidence. Preserve readability, SMPTE identity and module character. #39 tracks remaining loaded/platform acceptance; #37 is the next owner-requested processing-workflow delivery and #40 proposes shared idle-space improvements. These are records and recommendations, not implemented new screens.

## Agreed requirements

Space follows the user's task. Convert and Inspect prioritize readable source
properties and output/analysis decisions. Rip prioritizes content selection;
Merge prioritizes clip order; Player prioritizes viewing. Thumbnail prioritizes
generation and results; Author prioritizes content, disc settings and readiness.
Shared typography, spacing, module colors, source actions, status vocabulary and
action behavior make the suite coherent without imposing identical screens.

The SMPTE idle pattern remains a prominent VT brand element. Context-specific
loading instructions and an accessible Load action make it useful. Playback is
optional in working modules: the global setting above controls availability;
remember panel visibility only when players are enabled. Keep source loading,
metadata, output decisions and execution accessible with players disabled.
Readability is a requirement; do not compensate for clipping by silently
reducing the user's font. Collapsed headers retain their measured height.

## First slice: Convert workspace (dev95)

- Replace the fixed 50/50 player/metadata allocation and 3% collapsed panels
  with a layout that gives readable metadata the flexible space. Preview uses
  its natural compact height when both panels are open; either panel can close.
- Remove the permanent Frame-only bar in the shipped native pane (also used
  by the Player module). Move capture/cover and source track controls into a
  labeled tools dialog, available with loaded media.
- Keep source loading and source identity visible outside the optional player.
- Keep transport behavior unchanged by this presentation slice. The lifetime,
  seek and source-state findings in VT_PLAYER_AUDIT_DEV93.md remain open.
- Correct the discovered saved-position loading stall using direct timed preview
  decoding before playback starts. Accurate navigation remains separate.

Implementation: internal/ui/preview_details_layout.go, main.go,
convert_player_native.go and internal/i18n. Do not unlock Audio, Subtitles,
Filters, Trim or unavailable Burn. Upscale remains after encrypted-disc
acceptance. The full task inventory and initial evidence are in
UI_SUITE_AUDIT.md.

## Verification checklist

- [x] Large-font/VCR layout at restored/maximized sizes; metadata remains useful
  with preview open and closed; collapsed headers are fully visible. Plus Jakarta Sans remains a tester check.
- [x] Idle SMPTE has no Frame-only strip; Load Video works with preview closed.
- [x] Loaded media exposes transport and labeled tools; frame save/cover and
  audio/subtitle controls remain available.
- [ ] Narrow/current/wide window checks; no negative or overlapping layout
  geometry; metadata scroll remains usable.
- [x] Native build, vet and full tests; non-native build.
- [x] Real playback session and runtime log review recorded in PLAYER_DEBUG.

## Subsequent slices

Finish Rip source-state/readiness and encrypted DVD → Rip → Convert → playback.
Then audit loaded Convert content decisions, Queue/History result handoff and
the remaining unlocked modules individually. Evaluate Windows and Linux,
supported fonts/languages and display scaling; distinguish measured results
from untested combinations. Land small tester-identifiable versions rather
than a suite-wide redesign batch.
