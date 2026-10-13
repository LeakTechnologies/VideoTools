# VT_Player upgrade plan

## Dev97 integration checkpoint

Source-owned cancellation/join and retired-callback suppression are integrated with startup/resume continuity. Saved-position previews decode to the requested timestamp, audio preroll is discarded before buffering, bounded readiness precedes output, and the per-player output buffer is reduced. Opt-in diagnostics report queue occupancy, output levels, underruns, drops and clock adjustments. Native build/vet/full tests, race tests and non-native build passed. User-witnessed alternate-container playback had no noticed lip-sync issues; runtime counters showed no underruns/drops/snaps and clean teardown. Exact seeking, speed fallback quality, device-latency measurement and encrypted-disc acceptance remain open. CI/tag publication must be confirmed separately. Next order: instrument paused-output transitions; correct FFmpeg seek flags and validate returned PTS; exercise long-GOP/VFR frame navigation; then establish one comparison clock with one audible source.

## Purpose and boundaries

VT_Player is an inspection and verification surface for VideoTools, with synchronized original/processed comparison as its next major use. Reliable playback and exact frame positioning take precedence over a general-purpose player feature catalogue. The 2026-10-12 user direction authorizes starting the player upgrade sequence; encrypted DVD -> Rip -> Convert -> playback remains an outstanding project acceptance requirement.

Evidence: [dev93 audit](VT_PLAYER_AUDIT_DEV93.md). Preserve the InlineVideoPlayer module API, the existing Compare/Upscale dual-stream exceptions, static Windows packaging and Linux/Windows scope. Backend changes and workflow changes require separate approval. Do not start an Upscale redesign before the playback acceptance ladder is reliable.

## Proposed delivery order

| Stage | Scope | Exit gate |
|---|---|---|
| 1. Reliable playback | Source generations, one presentation consumer, cancellable waits, safe worker retirement, terminal error state, native resource disposal | Rapid pause/play, replacement, close during load/seek and EOF do not hang, resurrect sources or retain audio; real session and log reviewed |
| 2. Trustworthy navigation | Actual FFmpeg constants; newest-wins seek; preview versus committed scrub; returned PTS; forward/backward frame step | Paused/playing seek, drag release, chapter jump and stepping display the intended frame, including long GOP and VFR |
| 3. Synchronized comparison | One controller owns shared time and transport, timestamp mapping, paired presentation, loop interval and one audio source | Compare two existing files through play, pause, seek, step and buffer recovery without misleading temporal differences |
| 4. Short upscale previews | Select source interval, render settings snapshots, retain variants, choose settings for full render | Preview and full jobs share the processing configuration; source interval/settings provenance remains visible; project media acceptance already passed |
| 5. Inspection and measured tuning | Linked zoom/pan, fit/100%, split divider, consistent aspect/colour/scaling; profile decode/render/memory | Same scene region is comparable; recorded latency, frame age, drift, CPU and memory meet agreed machine-specific targets |

Source UI repairs from audit P11-P15 belong to stages 1/2 before comparison: clear host source, reset chapters/thumbnails, refresh track lists after Ready, preserve DVD title/network options and cancel auxiliary workers.

## First implementation slice: retirement-aware frame reads

Stage 1 is active. First make the Engine frame-read and PTS-wait boundary cancellable without closing a producer-owned frame queue. Empty-queue reads, paused polling and future-PTS waits must return on engine retirement or caller cancellation. Retirement is a distinct terminal error, not clean EOF. Keep NextFrame available for existing callers, and add a cancellable method for the next presentation-ownership slice. Use capped, speed-aware clock waits and revalidate frame generation after waiting.

Implementation files (including the retired-read suppression and behavioral tests): `internal/media/clock.go`, `internal/media/playback.go`, `internal/media/engine_interface.go`; behavioral regression tests under `internal/media/`. This first slice does not claim to solve the entire seek lifecycle: the next slice must wire cancellation through InlineVideoPlayer, join the old presentation consumer and reject retired source callbacks/commands. Native FFmpeg I/O interruption and partial-open disposal remain explicit follow-ups.

## Verification and release discipline

- Test caller cancellation and engine retirement while paused, blocked on an empty queue and waiting on future PTS; verify natural EOF stays distinguishable and ordinary timing still works.
- Run `scripts/windows/dev-verify.ps1`: native build, vet and full tests. Verify platform-independent code without introducing runtime dependencies.
- Run actual media through the player, exercise the changed paths, review session logs and record observations in PLAYER_DEBUG. Synthetic queue tests or a headless decode session alone do not substitute for user-facing playback acceptance.
- Keep unaccepted implementation on the upgrade branch. Land a small `v0.1.1-devN` release only after the player session gate is satisfied; synchronize version files, the six tracking documents, PLAYER_DEBUG and installation/platform notes. Watch development and release CI and do not silently omit tagging.

## Comparison contract for later stages

A shared comparison timestamp maps to original source time and processed preview time using the selected source interval. Matching resolution or frame number is insufficient when rates differ or interpolation adds frames. Paused seek/step presents both corresponding images together. Playback has one clock and an explicit policy when either decoder is late. Linked viewport transforms inspect the same normalized region, with explicit 100% inspection. Original and processed use consistent display colour/aspect treatment. Preview artifacts retain immutable source interval and settings metadata; modifying settings marks an old artifact as belonging to its previous configuration.

## Current status

Plan recorded before implementation. First retirement-aware-read slice implemented as the dev94 candidate; native gate and visible software playback/log review complete. Broader Stage 1 remains active; no later stage is accepted or shipped. User-supplied local media exercised play/pause/resume/2x/close; 243 progress frames and clean engine teardown were logged. User reported slight jitter at the end; this remains open. Dev94 is reserved for this implementation slice; remote CI/tag publication remains pending.

## Active second lifecycle slice

Implement a source session that owns cancellation, native-operation serialization, load completion and the current presentation run. Source replacement and Close invalidate its callbacks immediately; retirement waits for load and presentation completion before disposal. Transport requests cancel the old read and wait for its consumer to exit before another consumer enters. Seek and step use the same ownership boundary. Playback errors publish paused/error state; natural EOF cannot reload a newer source. Load milestones, scrub frames, delayed thumbnails and growing-file reload are scoped to the originating source. Partial-open disposal must work before Engine.Start.

Regression gates: older load finishing last, Close during load, rapid pause/play with maximum one frame consumer, paused seek/step retirement, late frame/error/EOF callback suppression, and repeated close of a partially opened engine. Native blocking I/O interruption remains explicitly outside this slice; retirement rejects eventual results and defers freeing resources until the native call returns. This is implementation within Stage 1, not comparison/Upscale work.
## Second-slice verification record

The candidate is implemented with source-owned cancellation/load completion, presentation-run joins, serialized native commands and scoped callbacks. Runtime diagnosis repaired a Fyne refresh/player-mutex cycle and paused-audio cleanup losing its stop channel. Scrubber start/stop are idempotent. Automated local-media stress completed with 398 frames, three source retirements, close-during-load cancellation and reopening.

Visual review is required before landing because the user was away. Stage 1 is not declared fully accepted: native I/O interruption remains outstanding, and resume timeline continuity was observed to be incorrect under repeated resumes. Stage 2 still owns exact seek flags, final scrub commit and backward-step correctness. Candidate dev95 is not a published release; revalidate release numbering and concurrent UI changes at landing.
Final verification: scripts/windows/dev-verify.ps1 passed native build, vet and full tests. Race-enabled internal/ui and internal/media tests passed, including the late-EOF and paused-audio shutdown regressions.

## Playback continuity repair (2026-10-13)

User-visible checks confirmed a 503 ms first-Play wait and repeated resume packet/frame loss. Fix as one continuity boundary: start decoding without waiting before Resume; preserve packets, frames, PCM and codec references on pause/resume; suspend paused demux except bounded preview/frame demand; keep a consumed-but-unpresented frame when its read is cancelled. Initial preview must not flush the live source, and saved-position previews must use timed direct decoding as the newer master fix does. Add queue/PCM preservation, parked-frame and no-pre-start-wait regressions; run native/race gates and repeat visible 1x/2x and rapid-resume checks. Exact seeking remains open.
## Continuity check results

Native full and race gates passed. User-witnessed tests on 2026-10-13 confirm quicker continuous playback without the previous distortion. First presented frame: 82ms; normal resume: 33.5ms; burst resumes: 14-18ms. Close-during-load cancellation and reopening completed. Residual 2x/end jitter and speed/clock timing remain open; intentional burst pauses are not continuous-playback stutter. Candidate version is dev97 because master independently advanced through dev96. Reconcile current master and run integration/CI before publishing; exact navigation and encrypted-disc acceptance are still outstanding.
## Integrated saved-position audio readiness

Integration playback exposed silent then glitchy saved-position audio. The first accepted audio timestamp was 47.787 for target 45.610, so adding a fixed sleep cannot recover lost samples. Discard decoded preroll before resampling/PCM buffering, retain callback-side stale-data safety, preserve encoded audio under active playback/prebuffer backpressure, and prepare a bounded amount of valid decoded audio/video before enabling output. Ordinary pause/resume should reuse ready buffers. Keep the common clock paused during preparation; cancellation and a bounded deadline remain required. Validate target audio timestamps, audible output, startup responsiveness and paused readiness teardown before landing.
## Player transport/UI repair — 2026-10-13 candidate

Reported faults: blank/oversized filename heading, fullscreen seek feedback loop, inline scrub never committing transport, and unavailable context keys. Fix the player source-caption boundary, truncate its single-row heading, compact only the player module's tools trigger, guard timeline paints and commit on click/drag release. Ignore stale progress during an edit/pending seek. Preserve fullscreen image updates on the UI thread and retire pending callbacks when it closes. Enable Space and Left/Right (+/-5 seconds) in the player video focus context; dedicated fullscreen supports these plus Escape/F to exit. Preserve other modules' layouts and text-entry shortcuts. Use installed FFmpeg seek flags with timestamp units and decode preroll to the committed target; do not pass the nonexistent ACCURATE flag or frame units with timestamps.

Verification: callback-driven timeline regressions; focus/keyboard/seek-constant tests; native gate and non-native build; real loaded-media click/drag seeks, fullscreen play/seek/exit and title/tool appearance plus runtime-log review before landing. DVD candidate remains separate.
