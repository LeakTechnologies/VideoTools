# VideoTools UI direction

## Agreed requirements

Space follows the user's task. Convert and Inspect prioritize readable source
properties and output/analysis decisions. Rip prioritizes content selection;
Merge prioritizes clip order; Player prioritizes viewing. Thumbnail prioritizes
generation and results; Author prioritizes content, disc settings and readiness.
Shared typography, spacing, module colors, source actions, status vocabulary and
action behavior make the suite coherent without imposing identical screens.

The SMPTE idle pattern remains a prominent VT brand element. Context-specific
loading instructions and an accessible Load action make it useful. Playback is
optional in working modules: remember the per-module preference and keep source
loading, metadata, output decisions and execution accessible when it is closed.
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
