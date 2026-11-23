# Automatic previews — implementation sequence

Approved product scope and acceptance: [#37](https://github.com/LeakTechnologies/VideoTools/issues/37).
Candidate slices: [session ownership #43](https://github.com/LeakTechnologies/VideoTools/issues/43),
[work ownership and preference #45](https://github.com/LeakTechnologies/VideoTools/issues/45).

## Clarified product contract — 2026-11-04

The owner requires a global setting that fully disables players in processing
modules, retaining playback only in VT_Player. This supersedes interpreting off
as admission-only suppression or a collapsed preview panel. Off must remove the
embedded player/transport/idle placeholder and reclaim its layout space; no local
preview action overrides it. An explicit Open in VT_Player handoff is allowed.
Preserve processing sources/options/jobs and existing manual sessions. Metadata,
explicit analysis and requested media transformations remain functional. Preview
preparation/hardware must not start while off, including rebuild/navigation and
direct-player exceptions. Existing saved defaults are retained; operator wording
should explain availability, not imply that off merely stops automatic loading.

The existing candidate provides partial ownership infrastructure, not this complete
behaviour. When enabled, review concerns the selected visible source; batch import
must not prepare the entire collection for playback. Review remaining per-source
capture scheduling against this contract rather than accepting bounded concurrency
as completion. Independent Inspect controls, manual-session isolation and direct
exceptions remain required. See [suite direction](UI_SUITE_DESIGN.md#processing-players-and-convert-batch-workspace--2026-11-04)
for the batch-workspace hypothesis and required production-component checks.

The released dev101 shared Load API does not distinguish a processing preview from an
explicit manual session. Add that distinction before exposing a global switch.
Keep manual Load/LoadDVD/LoadURL semantics and InlineVideoPlayer as the API.

## Session boundary candidate

LoadAutomatic admits an automatic source under the same lock that replaces the
current session. SetAutomaticPreviewsEnabled changes admission under that lock;
disabling cancels all outstanding automatic sessions, detaches only a currently
automatic source, suppresses its retired callbacks and returns a completion
signal after load/presentation/native teardown joins. Manual sources are preserved.
Default admission is enabled. Re-enabling permits a new request; it does not
resurrect a retired source. Automatic ownership follows internal source reloads.
Native/stub public method sets remain aligned.

Tests use the production lifecycle boundary with deterministic fake engines to
cover denied admission, in-flight cancellation, queued callbacks, teardown wait,
manual replacement and re-enable. These are lifecycle checks, not real playback
or performance acceptance. Keep this as a PR candidate pending integration.

## Background-work and preference candidate (#45)

`previewwork.Controller` is a host-owned work scope with an explicit concurrency
budget. Capture `Context()` before scheduling, then call `Run` synchronously from
a bounded host worker pool. Pass its context to FFmpeg/analysis; use `IsCurrent`
on the UI thread together with source identity before publishing results. A
request captured before disable stays rejected after re-enable. A completed Run
is not permission to publish into a newer source. The controller bounds running
operations across retiring generations; it does not supply an unbounded queue or
bound the caller's number of scheduled goroutines.

`SetEnabled(false)` cancels all admitted generations and returns a signal after
all their work exits, including non-cooperative work still returning from native
calls. Repeated disable waits for prior retirements too. Enabling creates a fresh
scope without starting work. Admission and WaitGroup registration are serialized
with retirement so queued work cannot register after the join begins. The budget
is shared across generations; cancellation alone does not free a slot.

Preferences store `DisableAutomaticPreviews` as an opt-out. A missing/false/null
key preserves enabled behaviour for legacy configs; explicit true persists the
opt-out. `AutomaticPreviewsEnabled()` is the single interpretation of that value.
The model now initializes selected host preparation paths in #46, but the Settings control and complete processing-module suppression remain unfinished. Do not treat editing the key as a completed feature.

Host integration must combine worker and session retirement signals off the UI
thread, preserve job input/options and manual sessions, guard delayed publication
and adapt actual subprocess commands to use the owned context. Constructor tests
and a saved preference model do not establish subprocess termination or UI behaviour.

## Remaining #37 integration

Follow-up #46: Inspect analysis claims capture the host owner and exact policy
request on the UI thread before analysis scheduling. Detector commands share the
four-operation budget and cancellation, with the existing two-minute timeout
inside the admitted operation. At delivery, source-generation validation remains
first; retired policy can settle pending state with cancellation but cannot publish
a result, including after re-enable. Disabled imports do not mark analysis pending.
Explicit Convert analysis remains independent and available with previews off.
No new Inspect action or global Settings UI yet. Source replacement still rejects
publication rather than cancelling every old detector immediately; complete
source-owned analysis cancellation remains open.

Follow-up checks: Windows native build/vet/full tests passed (root 9.890s;
unchanged packages cached). Focused race tests executed disabled, retired and
real enabled automatic analysis plus explicit Convert analysis with policy off
(1.947s, no skips). Non-native build passed. Thirteen task files passed UTF-8,
81 local links, roadmap script/234 unique IDs and unchanged legacy encoding
markers. No GUI/player/audio, import benchmark or conversion-output acceptance
was run; the global switch remains unexposed.

1. Persist the agreed backwards-compatible preference; settings construction must
   not trigger a policy write. Keep one host-owned policy.
2. Route automatic import/rebuild/player calls through admission. Separate explicit
   manual VT_Player actions and direct Compare/Upscale exceptions. Resolve overlap
   with #32 and PR #28 before landing; do not merge historical work blindly.
3. Give frame capture, queue thumbnails and automatic analyses cancellable bounded
   ownership. Disabling must retire these too; session admission alone cannot do it.
4. Keep Inspect metadata independent of player Ready; expose deliberate deeper
   analysis and preserve processing sources/immutable job plans.
5. Measure actual single/batch cold/warm import time, working set and background
   activity on/off. Verify toggle/replacement/navigation/rebuild, manual playback,
   output parity and accessible production UI, native/non-native gates and actual
   playback logs. No predicted speedup or whole-movie RAM claim is justified yet.

No setting or application behaviour is changed merely by adding the unused API.
No backend, dependency, CI, locked module or release change is included here.

## Candidate checks — 2026-11-04

The Windows native build, vet and full test gate passed via
`scripts/windows/dev-verify.ps1`. Focused native lifecycle race checks and the
public automatic-load/idempotent-retirement race case passed. The non-native
stub policy test and full non-native application build passed. Candidate/PR
identity are recorded on #43. Fifteen task files passed UTF-8, local-link,
roadmap-script/unique-ID and staged diff checks; legacy encoding markers remain
unchanged. No real playback, performance measurement, Settings integration or
processing-output acceptance ran. Keep this candidate unmerged pending integration.

Work/preference slice #45 checks: `go test -race` for previewwork and settings
passed; Windows native build/vet/full tests and non-native `go build ./...`
passed. Unchanged packages were reported cached by Go in the full gate. Eleven
task files passed UTF-8/link/roadmap/diff checks. These results do not establish
FFmpeg termination, runtime memory savings, host integration or UI acceptance.

## Host/command integration candidate (#46) — dev102

The host initializes one lazy owner after preferences load, with a four-operation
budget shared by fallback frame capture and queue thumbnails. Four follows the
existing import pool ceiling in loadVideos; it is a provisional resource bound,
not a measured optimal value or speedup claim. Combined policy
application returns completion after work and shared automatic player sessions
retire; it does not clear processing source/options or explicit manual sessions.
Convert/Inspect automatic Load entry points use LoadAutomatic. Explicit VT_Player
Load paths remain manual. Convert import does not explicitly close manual playback
when its automatic policy is disabled; full navigation/render acceptance remains open.

Fallback captures now use the existing context-aware hidden-command utility;
cancel/error/no-output paths clean their own scratch directory. Thumbnail import
uses the same automatic capture helper; explicit thumbnail generation remains a
processing job. Queue thumbnail work snapshots input and request before scheduling,
uses unique temporary files even before a job receives its ID, passes the owned
context to extraction, and publishes on the UI thread only for a current policy,
unchanged input and still-empty thumbnail slot. Failed/retired artifacts are removed.

Neutral generated-source tests verify opt-out without opening a missing input,
real patterned 640x480 frame output, cancelled-capture scratch cleanup, and
cancellation/Wait of a real looping FFmpeg command through the combined host
retirement path while processing/manual source fields remain intact. The first
compile found another Thumbnail caller, which was corrected. First real-media
runs skipped for missing executable PATH; rerunning with existing sidecars on
PATH passed all three checks. Native/race/non-native final outcomes and exact
candidate identity are recorded on #46. These checks are not audible playback,
whole-film import measurements or conversion-output acceptance.

Remaining gaps before exposing the global switch: explicit analysis ownership and
independent Inspect actions; early request and source-generation guards for all
fallback publications; manual-vs-processing widget visibility; startup hardware
warmup policy and manual initialization; direct Compare/Upscale exceptions; bounded
host scheduling in addition to bounded running commands; actual Settings callback
and persistence/error UI; measured on/off single/batch imports and real playback.
Do not describe the global feature as implemented or accepted yet.

Version files identify this branch as dev102 because default background work now
shares the concurrency budget. No tag, merge, binary release or installed-build
claim follows. The latest published application baseline remains dev101.
Final #46 checks on Windows/amd64 with Go 1.26.6 and existing FFmpeg 8.1 sidecars:
native build/vet/full tests passed (some unchanged packages cached); final focused
root race checks passed with all three FFmpeg/host cases executed; non-native
build passed. Initial missing Thumbnail caller compile failure was fixed; initial
real-media checks skipped for PATH and were rerun successfully with sidecars.
Twenty task files passed UTF-8, 82 local links, roadmap script/234 unique IDs,
unchanged 358 legacy encoding markers, coherent dev102 manifests and diff checks.
No application GUI, audible playback, encrypted-disc or import/working-set
benchmark acceptance was run for this slice.
