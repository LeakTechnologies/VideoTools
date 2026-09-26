# Convert Module — Full Audit Brief

**Purpose:** a self-contained brief for an external auditor (GPT) to perform a full audit of the
Convert module. Every file path, line number, and line count below was read out of the tree — do
not re-derive them, and do not guess at anything not listed here. Where a claim needs verifying,
read the code.

**Repo:** `github.com/LeakTechnologies/VideoTools` (Go + Fyne desktop app, CGo media engine)
**Cycle at time of writing:** `v0.1.1-dev81`
**Primary user platform:** Windows. Primary dev platform: Linux. **No macOS support** — Linux and
Windows only.

---

## 1. Mission

The Convert module has accumulated real problems. The Human Director reports:

1. **The player panel and the metadata panel have multiple unresolved issues.**
2. **Some functionality inside the Convert module is outright broken.**

Produce a ranked, evidence-backed audit that identifies every defect you can prove from the code,
then proposes a fix order. You are auditing — **do not write or edit production code.** Output is
the report described in §8.

---

## 2. How to build and verify (use these, do not invent commands)

```bash
# THE build gate. This is the only command that proves the tree compiles.
scripts/windows/dev-verify.ps1        # Windows: go build -tags=native_media ./... + go vet ./...

# Linux equivalent of the gate:
CGO_ENABLED=1 go build -tags=native_media ./...
CGO_ENABLED=1 go vet -tags=native_media ./...
```

- **`gofmt -e` is a syntax check, NOT a build.** Never present a gofmt pass as a build result.
- A bare `go build` **fails at the CGo LDFLAGS gate** (`-Wl,--stack,4194304`) because
  `CGO_LDFLAGS_ALLOW` is only set in CI / by `dev-verify.ps1`. If you cannot build, say so
  explicitly and state why. Do not imply a build passed.
- Build tags in play: `native_media` (real media engine) and `vlc` (libVLC backend, opt-in).
  `native_media_stub.go` is the no-op twin of `native_media.go` and **must expose an identical
  method set** — a stub/reality mismatch is a build break, and a defect worth reporting.
- There are currently **zero unit tests for the Convert module.** The only `convert*_test.go`
  matches in the tree are vendored Fyne tests (`_fyne/data/binding/convert_test.go`) and are not
  ours. Treat the absence of tests as a finding, and propose what is worth testing.

---

## 3. File map of the Convert module

### 3.1 The bulk of it is one 4,300-line function in a 16,000-line file

`main.go` is **15,992 lines**. The Convert view is a single enormous function.

| Location | Lines | What it is |
|---|---|---|
| `main.go:896` | struct | `convertConfig` — the entire Convert settings state (large; read it in full) |
| `main.go:1007` | fn | `userPresetFromConfig` — user preset → config |
| `main.go:1054` | fn | `(convertConfig) OutputFile()` |
| `main.go:1062` | fn | `(convertConfig) CoverLabel()` |
| `main.go:1069` | fn | `defaultConvertConfig` |
| `main.go:1133–1181` | fns | `loadConvertRecovery` / `saveConvertRecovery` / `loadPersistedConvertConfig` / `savePersistedConvertConfig` |
| `main.go:1589` | fn | `(appState) persistConvertConfig` |
| `main.go:2305–2445` | fns | `addConvertToQueue`, `addConvertToQueueForSource`, `addAllConvertToQueue`, `addConvertToQueueForSourceWithOutputs` |
| `main.go:2305` | fn | `addAllConvertToQueue` returns `(int, error)` |
| `main.go:3798` | fn | `showConvertView` — entry point into the view |
| `main.go:3826` | fn | `videoSourceToConvertSource` → `convertmodule.VideoSourceInfo` |
| `main.go:3854` | fn | `convertSourceToVideoSource` (reverse mapping) |
| `main.go:5226` | fn | `executeConvertJob` — the actual ffmpeg job execution |
| **`main.go:8806–13151`** | **~4,346** | **`buildConvertView(state, src) fyne.CanvasObject` — the whole Convert UI in one function** |
| `main.go:13152–13169` | 18 | `makeLabeledPanel` |
| **`main.go:13170–13613`** | **~444** | **`buildMetadataPanel` — THE METADATA PANEL (flagged as broken)** |
| **`main.go:13614–14042`** | **~429** | **`buildVideoPane` — THE PLAYER PANEL (flagged as broken)** |
| `main.go:14043–14103` | ~61 | `previewAnimator` (Start/Pause/Play/showFrame/Stop) |
| `main.go:14104` | fn | `showFrameManual` |
| `main.go:14119` | fn | `captureCoverFromCurrent` — cover capture from player |
| `main.go:14135` | fn | `importCoverImage` |
| `main.go:14147–14725` | ~579 | `handleDrop` — module drop dispatch, incl. the convert branch |
| `main.go:14726` | fn | `detectModuleTileAtPosition` — which module a drop landed on |
| `main.go:14790` | fn | `loadVideo` |
| `main.go:14895` | fn | `loadMultipleVideos` |
| `main.go:14995` | fn | `clearVideo` |
| `main.go:15018` | fn | `releasePlaybackSession` |
| `main.go:15128` | fn | `switchToVideo` |
| `main.go:15180` / `15189` | fns | `nextVideo` / `prevVideo` |
| `main.go:15200` | fn | `crfForQuality` |
| `main.go:15219` / `15248` | fns | `detectBestH264Encoder` / `detectBestH265Encoder` |
| `main.go:15274` / `15342` | fns | `determineVideoCodec` / `determineAudioCodec` |
| `main.go:15363` / `15372` | fns | `cancelConvert` / `startConvert` |

### 3.2 The two panels the Director flagged

**Metadata panel — `main.go:13170` `buildMetadataPanel(state, src, min, accentColor, initiallyOpen, onToggle)`**
Returns `(fyne.CanvasObject, func())` — the second value is an update closure. Called from
`main.go:9288`. Notable internals:
- `main.go:13180` empty-source fallback: `noSrcBody` with `t.ConvertInspectHint`
- `main.go:13182` / `13397` **two** `ui.BuildCollapsibleHeader(t.ConvertSectionMetadata, ...)` calls
  in the same function — check whether both are live or one is dead
- `main.go:13204`, `13213`, `13242`, `13256` — metadata text assembly (`kbps`, `frames`, `File: %s`)
- `main.go:13343` `makeRow("Resolution", …)`, `13346` `makeRow("Frame Rate", …)`,
  `13365` `makeRow("Audio Rate", …)`
- `main.go:13380` `dialog.ShowInformation(t.DialogCopied, "Metadata copied to clipboard", …)`
- `main.go:13418`–`13590` — interlacing analysis UI: `analyzeBtn`, a **2-minute** `context.WithTimeout`,
  progress + result rendering, comparison-preview generation with a **1-minute** timeout

**Player panel — `main.go:13614` `buildVideoPane(state, min, src, onCover)`**
- `main.go:13615` — **branches**: `if HasNativeMediaPlayer() { return buildVideoPaneNative(state, min, src, onCover) }`
  Everything after that branch is the **non-native** path: a SMPTE-bar `canvas.NewRaster` drop
  target (`13652–13679`), a `canvas.Image` preview-frame stage (`13690+`), and `previewAnimator`.
- `convert_player_native.go:29` — `buildVideoPaneNative` (646 lines) — the real path; calls
  `GetConvertPlayer()` at `convert_player_native.go:67`
- `native_media.go:125` `HasNativeMediaPlayer` (real) / `native_media_stub.go:10` (stub)
- `native_media.go:137` `GetConvertPlayer() *ui.InlineVideoPlayer` — the Convert player singleton
- `native_media.go:307–363` — player control entry points: `Load`, `Play`, `Pause`, `Seek`,
  `StepFrame`, `ScrubTo`, `SelectAudioTrack`, `SetVolume`, `SetMuted`, `DisableSubtitles`,
  `SelectSubtitleTrack`, `Close`, and `BuildConvertPlayerPane`
- `native_media_stub.go:38` / `:64` — stub twins of `BuildConvertPlayerPane` / `GetConvertPlayer`

### 3.3 Engine and helper packages

| File | Lines | Role |
|---|---|---|
| `internal/convert/ffmpeg.go` | 351 | ffmpeg argument construction / invocation |
| `internal/convert/types.go` | 261 | config + job types |
| `internal/convert/dvd.go` | 301 | DVD paths |
| `internal/convert/dvd_regions.go` | 264 | region handling |
| `internal/convert/presets.go` | 91 | presets |
| `internal/convert/format_name.go` | 18 | format naming |
| `internal/app/modules/convert/view.go` | 148 | thin module shell (`VideoSourceInfo`, entry point) |
| `internal/metadata/naming.go` | 74 | metadata-driven output naming |

---

## 4. Confirmed findings — already verified, do not re-derive

These were read directly out of the tree. Treat them as established. Your job is to find what is
*not* listed here, and to assess severity and fix order.

### F1 — Two divergent player-panel implementations (high)
`buildVideoPane` splits at `main.go:13615` into a native path (`convert_player_native.go:29`) and a
non-native path (`main.go:13679` onward, ~360 lines of SMPTE raster + `canvas.Image` +
`previewAnimator`). `native_media` is the shipped player path on both platforms, so the non-native
branch is very likely vestigial — but it is still compiled, still reachable behind the stub, and
still mutates shared state (see F3). Either it is dead weight that should be deleted, or it is a
load-bearing second implementation that must be kept in parity. **Determine which, and say which.**

### F2 — Interlacing analysis is implemented twice (high)
One implementation lives inside `buildConvertView` (`main.go:9887`, with its own status label at
`13454` region and `dialog.ShowError(... "Analysis failed" ...)` at `13437`), and a second, separate
one lives inside `buildMetadataPanel` (`main.go:13418`–`13590`, with `2*time.Minute` then
`1*time.Minute` timeouts, its own `analyzeBtn`, its own result rendering, and a comparison-preview
generator). Two copies of the same user-facing operation with different timeouts and different
result presentation is a live inconsistency source. **Diff them and enumerate every behavioural
difference.**

### F3 — Widget construction mutates playback state (medium-high)
`main.go:13682` calls `state.stopPreview()` and `13687` assigns `state.currentFrame` — side effects
on `appState` from inside a view-builder. `previewAnimator` (`14043`–`14103`) and
`captureCoverFromCurrent` (`14119`) share that same mutable preview state with the cover-capture
feature. `buildConvertView` is called on every view build, so a rebuild can stop playback or
clobber the captured cover. **Trace exactly which state a rebuild mutates.**

### F4 — Untranslated user-facing strings (medium)
A grep for `widget.NewLabel("…` / `SetPlaceHolder("…` / `ShowInformation(` / `ShowError(fmt.Errorf("…`
across `main.go:8806–14042` returns **59 matches**. Some are false positives (data values such as
`crfEntry.SetText("23")` need no translation), but genuine violations include:

| Line | String |
|---|---|
| 13343, 13346, 13365 | `"Resolution"`, `"Frame Rate"`, `"Audio Rate"` |
| 13380 | `"Metadata copied to clipboard"` |
| 13482–13485 | `"%.1f%% interlaced frames"`, `"Field Order: %s"`, `"Confidence: %s"`, `result.Recommendation` |
| 13204, 13213, 13242, 13256 | `"%d kbps"`, `"%d frames"`, `"File: %s"` |
| 13524, 13538, 13543 | `"Creating comparison preview..."`, `"Preview generation failed"`, `"Failed to load preview"` |
| 10100 | `"Custom aspect ratio in use."` |
| 9888 / 13437 | `"Analysis failed: %w"` |
| 9716, 10346, 10495, 10132, 10815, 10839 | placeholders: `"Output folder path"`, `"System temp (recommended SSD)"`, `"Auto (from Quality preset)"`, `"e.g. 1.90 or 256:135"`, `"e.g., 250"` |

**Triage the full 59 and produce a complete list of real violations** with the i18n key each one
should use.

### F5 — An unimplemented section is still wired into the UI (medium)
`main.go:10269` — `// TODO: Implement format section with navy background and codec info display`.
Adjacent helpers exist and are apparently unused by it: `buildFormatBadge` (`main.go:8730`),
`buildVideoCodecBadge` (`8763`), `buildAudioCodecBadge` (`8785`). **Find out whether these badges
are dead code, and what the user actually sees in that section today.**

### F6 — Zero test coverage on a 4,300-line function (high, structural)
`buildConvertView` is untestable as written. Any fix to it is unverifiable by test. Propose a
decomposition that creates seams worth testing, in dependency order.

---

## 5. Audit focus areas

Answer each with evidence. Where you cannot determine something from the code, say so explicitly
and state exactly what additional information would settle it.

**A. Player panel correctness** (`main.go:13614` + `convert_player_native.go`)
1. Trace the full lifecycle: load → play → pause → seek → step → track select → close. For each
   step, identify the state that can desynchronise between the widget, `appState`, and
   `GetConvertPlayer()`.
2. What happens on **re-entering** the Convert view — is playback stopped, is the player released,
   is the previous module's session leaked? (`releasePlaybackSession` at `main.go:15018`)
3. Cover capture (`captureCoverFromCurrent` `14119`, `importCoverImage` `14135`, `onCover` callback
   threaded from `main.go:9247`) — can it capture a stale or wrong frame? Is the cover path
   racy with playback?
4. Does the non-native branch (F1) still get exercised anywhere (stub builds, Linux, tests)? If a
   Linux or stub build renders it, its behaviour must be audited too.
5. `GetConvertPlayer()` returns a **singleton** (`native_media.go:137`). Check every call site for
   use-after-close and for cross-module interference (Compare and Upscale are the only sanctioned
   dual-stream exceptions; see §6).

**B. Metadata panel correctness** (`main.go:13170`)
1. Two `BuildCollapsibleHeader(t.ConvertSectionMetadata, …)` calls (`13182`, `13397`) — is one dead?
   Does the panel render twice, or is one an inner section?
2. What is the returned update closure (`func()`) actually for, who calls it, and is every caller
   consistent? Compare against the `onToggle`/`initiallyOpen` contract.
3. Empty-source handling: is `noSrcBody` (`13180`) reached on every path, including a source that
   loads but has no probe data?
4. Interlacing analysis: the `2*time.Minute` analysis timeout and the `1*time.Minute` preview
   timeout — what happens on timeout? Is `cancel` deferred on **all** paths (a leaked
   `context.WithTimeout` is a goroutine/context leak)? Same question for every `context.WithTimeout`
   in the module.
5. Does the panel's interlacing result write back into `state.convert` correctly, and does that
   write invalidate anything the player panel is showing?

**C. Settings correctness across the ~4,300-line view**
1. `convertConfig` (`main.go:896`) vs `defaultConvertConfig` (`1069`) vs
   `loadPersistedConvertConfig` (`1142`) — does every field round-trip through persistence, and
   does a config written by an older build still load? Enumerate fields that silently reset.
2. Simple vs Advanced mode (`Mode` field; mode switch around `main.go:12498`/`12501`) — are there
   fields that one mode writes and the other ignores, leaving stale values that leak into the
   ffmpeg arguments? The repo has been bitten by exactly this class of bug before.
3. **Derived-value consistency:** `determineVideoCodec` (`15274`), `determineAudioCodec` (`15342`),
   `crfForQuality` (`15200`), `detectBestH264Encoder`/`detectBestH265Encoder` (`15219`/`15248`),
   `OutputFile()` (`1054`), `CoverLabel()` (`1062`) — for each, find every input that changes its
   result and confirm no UI path can change an input without invalidating the derived value.
4. `ensureCompatibleCodec` (`main.go:868`) and `effectiveHardwareAccel` (`447`) — is the
   compatibility repair idempotent and re-entrant? Can it fight the user's explicit choice?
5. The encoder probes (`15219`, `15248`) shell out and cache — when is the cache invalidated? Can a
   stale probe produce an argument list ffmpeg will reject?

**D. Job execution and queueing** (`main.go:5226`, `2305–2445`)
1. `executeConvertJob` — argument construction vs `internal/convert/ffmpeg.go`: is there duplicated
   or divergent logic between UI-side and package-side argument building?
2. `addAllConvertToQueue` vs `addConvertToQueueForSource*` — do they agree on validation, naming
   and overwrite policy? Name collisions across multiple sources?
3. `cancelConvert` / `startConvert` (`15363`/`15372`) vs the queue executor — double-start, cancel
   after completion, and orphaned-process paths.

**E. Drop and multi-source handling** (`main.go:14147–14725`, `14790–15189`)
1. `detectModuleTileAtPosition` — off-by-one/edge cases at grid boundaries; the drop lands on the
   wrong module.
2. Multi-video slots ("both slots full, overwriting slot 1" at `14358`) — is the overwrite policy
   communicated to the user? Is state left consistent?
3. `switchToVideo` / `nextVideo` / `prevVideo` — index bounds, playlist (`playlist`/`playlistIdx`
   in the player) coherence, and whether switching mid-playback leaks or double-loads.

---

## 6. Hard constraints — findings that violate these will be rejected

These are settled project decisions. **Do not propose changing them.** If you believe one is wrong,
flag it in a separate "Challenges" section rather than as a fix.

1. **The One Rule — the player API layer.** Every module talks to the player through
   `ui.InlineVideoPlayer`. There must be **no** `media.NewEngine()` in a module, no per-module
   playback goroutines, and no direct `media.NewVideoPlayer()`. The widget is obtained via
   `opts.Player.Widget()`. **A Convert-specific or module-local player is a non-starter.** The
   backend is chosen by the `vlc` build tag behind `media.PlaybackEngine`, so the module must not
   care which backend is active.
2. **Sanctioned dual-stream exceptions exist and are unrelated to Convert:** Compare
   (`compare/fullscreen_native.go`) and Upscale (`upscale_player_native.go`). Do not extend the
   pattern to Convert.
3. **i18n is mandatory.** Every user-facing string goes through `i18n.T().KeyName`. Keys live in
   `internal/i18n/strings.go`; **English in `en_ca.go` is the source of truth**; French
   (`fr_ca.go`) and Inuktitut (`iu.go`, `iu_latin.go`) must be added too. Key naming:
   `ModuleXxx` / `ActionXxx` / `LabelXxx` / `StatusXxx` / `DialogXxx`. A fix that adds a hardcoded
   English string is not a fix.
4. **No new root-level `.go` files — hard rule.** New code belongs in the appropriate `internal/`
   package. `main.go` must shrink, not grow.
5. **No `darwin` code paths.** Linux and Windows only; delete `case "darwin":` on sight.
6. **Windows: zero new runtime dependencies** — OS built-ins only.
7. **Player changes require a log review from a real playback session** before they land. "No
   errors logged" is **not** evidence of correctness — the absence of a crash report is the failure.
   Any player-panel fix must ship with a runtime-log review step, and `docs/PLAYER_DEBUG.md` is
   updated in the **same commit** as the fix or it does not get updated.
8. **State-tracking variables must be assigned at the point of comparison.** Any `if x != y` guard
   that mutates neither `x` nor `y` is a bug — this exact class of defect shipped before (a seek
   generation counter was read but never assigned, so a log line fired 60×/sec forever). Audit for
   this pattern in the panel state.
9. **Chesterton's Fence.** Code whose purpose you cannot establish from the code, its comments, or
   git history is *not* dead code. Say "purpose unclear — needs history" instead of proposing
   deletion.
10. **Documentation discipline.** No personal names in docs; refer to `user report` / `dev report`.

---

## 7. Severity scale — use these definitions, not vibes

| Severity | Definition |
|---|---|
| **S1 Critical** | Data loss, wrong output written to disk, crash/hang, or silent corruption of user media. |
| **S2 High** | A user-facing feature is broken or produces wrong results, with no workaround. |
| **S3 Medium** | Degraded behaviour, missing translation, duplicated logic at risk of divergence, untestable structure. |
| **S4 Low** | Cosmetic, dead code, comment/behaviour drift, minor duplication. |

Every finding needs a **provable failure scenario**: "user does X → Y happens → that is wrong
because Z." A finding you cannot express that way is a hypothesis — label it as such and put it in
a separate, clearly-marked section.

---

## 8. Required output format

Produce, in this order:

### 8.1 Executive summary
Max 15 lines. The state of the Convert module in plain language, the top 5 findings by severity,
and the single highest-value structural change.

### 8.2 Findings table

| # | Severity | Area | Location (`file:line`) | One-line summary | Confidence |
|---|---|---|---|---|---|

`Confidence` ∈ {Proven, Strong, Hypothesis}. Sort by severity, then by area.

### 8.3 Per-finding detail — for **every** finding
```
### F-N: <title>
- Severity: S1..S4   Confidence: Proven|Strong|Hypothesis   Area: player|metadata|settings|jobs|drop
- Location: path/file.go:LINE (list every relevant site)
- Failure scenario: <user action> → <observed> → <why it is wrong>
- Evidence: the actual code you read (short excerpts, not paraphrase)
- Impact: what the user loses
- Proposed fix: smallest change that resolves the root cause
- Risk of the fix: what could regress; what must be retested
- Test that would have caught it: <specific test or manual procedure>
```

### 8.4 Structural assessment
Address the 4,300-line `buildConvertView` directly: propose a decomposition into named units with a
dependency-ordered sequence, a target file layout honouring §6.4, and which units become testable at
each step. Be concrete about ordering — the first slice must be safe to land on its own.

### 8.5 Duplication register
Every case of duplicated logic found (F1/F2 and any others), as a table: concept → all sites →
whether they currently agree → recommended single source of truth.

### 8.6 Test plan
A prioritised list of tests worth writing, split into: pure unit tests (no GUI, no ffmpeg) /
integration tests requiring a sample file / manual tester checklist items. For each, name the
target file and the specific behaviour pinned.

### 8.7 Challenges (optional, separate)
Anything you believe violates a §6 constraint, stated as a challenge with your reasoning. Never
as a fix.

---

## 9. Definition of a good audit

- Every claim is anchored to a `file:line` you actually read. **If you did not read it, do not
  assert it.** Mark inferences as `Hypothesis`.
- Failure scenarios are concrete and user-observable, not abstract ("this could race" is not a
  scenario; "cover capture after switching videos grabs the previous video's frame" is).
- The i18n violation list is **complete** for the module, not a sample.
- You distinguish dead code from unclear code, and say which you cannot resolve.
- The fix proposals are the smallest thing that works. No speculative rewrites of the 4,300-line
  function presented as a single change.
- You respect §6. Any constraint you wish to challenge goes in §8.7 only.

## 10. After the audit

The report is triaged by the project agent, which turns accepted findings into issues, slices the
structural work into small reversible commits, and updates `AGENTS.md`, `docs/CHANGELOG.md`,
`docs/roadmap.html`, `docs/ROADMAP.md`, `DONE.md`, and `TODO.md` in the same commit as any landed
change. Verification is `scripts/windows/dev-verify.ps1`; player-panel fixes additionally require a
real playback session with a log review before landing.
