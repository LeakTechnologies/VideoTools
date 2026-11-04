# Automatic previews — implementation sequence

Approved product scope and acceptance: [#37](https://github.com/LeakTechnologies/VideoTools/issues/37).
Candidate slices: [session ownership #43](https://github.com/LeakTechnologies/VideoTools/issues/43),
[work ownership and preference #45](https://github.com/LeakTechnologies/VideoTools/issues/45).

The current shared Load API does not distinguish a processing preview from an
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
This model is not connected to the running host or Settings control yet: editing
the key does not by itself suppress app preparation in this candidate.

Host integration must combine worker and session retirement signals off the UI
thread, preserve job input/options and manual sessions, guard delayed publication
and adapt actual subprocess commands to use the owned context. Constructor tests
and a saved preference model do not establish subprocess termination or UI behaviour.

## Remaining #37 integration

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
