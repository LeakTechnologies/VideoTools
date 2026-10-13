# VideoTools Agent Workflow Rules

Inserted optical DVD test (2026-10-13): disc topology read and title opened as MPEG-2 720x480 at 29.97 fps with audio. Visible player showed “playback audio/video buffers did not become ready”; cached audio/video PTS remained zero. Repeated bwdif return -11 and generic-source scrub/thumbnail open failures were recorded. These are observations, not established root causes. Close logged clean engine disposal/source retirement. User was away, so no audible or continuous visual acceptance. DVD playback remains unaccepted; investigate interlaced-frame output handling, readiness and DVD-aware auxiliary decoding next. Media identity omitted.

VT_Player DVD requirement recorded: ISO/VIDEO_TS title playback exists in code but needs real-disc acceptance; VLC-style interactive menus are not implemented. Player-only implementation/test sequence: [DVD navigation plan](docs/VT_PLAYER_DVD_NAVIGATION.md). This records planning only; no playback change.

## Product and module naming — strict

- **VideoTools** is the product; **VT** may be used as its abbreviation. Every version, release, tag and published build belongs to VideoTools: `VideoTools v0.1.1-devN`.
- **VT_Player** is a module within VideoTools. Use this exact spelling in prose. Describe its changes as “VideoTools v0.1.1-devN — VT_Player improvements”; never describe a suite version as “VT_Player devN” or imply a separately versioned player product.
- Preserve established code identifiers, package paths and application metadata. This naming rule does not authorize unrelated renames or a separate player release.


Development CI for implementation commit 42862da5 passed Linux and Windows: https://github.com/LeakTechnologies/VideoTools/actions/runs/38440710576. User accepted alternate-container playback with no noticed lip-sync issues. Release token is absent; tag publication is a Human Director handoff, not a published release claim.

## Lead designer handoff — Player Clear source state

Confirmed static source-state defect outside this player implementation scope: internal/app/modules/player/view.go Clear resets opts.PlayerFile, while host state.playerFile remains populated in player_module.go. A subsequent host rebuild can restore the cleared source. Lead designer should wire authoritative host clearing and verify rebuilt panes. Do not modify other modules in this player task. Track-selector population before asynchronous Ready is a separate verification candidate, not a confirmed rendered defect.

## v0.1.1-dev97 — Player integration acceptance (2026-10-13)

Source-owned cancellation/join and retired-callback suppression are integrated with startup/resume continuity. Saved-position previews decode to the requested timestamp, audio preroll is discarded before buffering, bounded readiness precedes output, and the per-player output buffer is reduced. Opt-in diagnostics report queue occupancy, output levels, underruns, drops and clock adjustments. Native build/vet/full tests, race tests and non-native build passed. User-witnessed alternate-container playback had no noticed lip-sync issues; runtime counters showed no underruns/drops/snaps and clean teardown. Exact seeking, speed fallback quality, device-latency measurement and encrypted-disc acceptance remain open. CI/tag publication must be confirmed separately.

## Historical player continuity candidate — 2026-10-13

Current player branch candidate is dev97 (master advanced independently through dev95/dev96; candidate must be reconciled before landing). Startup no longer waits for a frame before starting its decoder. Normal resume preserves packet/frame/PCM queues and codec references; paused demux is preview/frame-demand driven, and interrupted presentation retains its next frame. Initial preview no longer discards live-source packets. Saved-position preview uses timed direct decoding, carrying forward the newer master fix. Native full gate and race tests pass. Visible checks: first presented frame 82ms, normal resume 33.5ms, six burst resumes about 14-18ms; reopening and close-during-load cancellation completed, with no ERROR/WARNING records. User reports quicker continuous playback without distortion and suggests the deliberate burst pauses explain perceived jitter. Residual 2x/end jitter, speed timing and exact seeking remain open. The previous away/pending-visual notes are historical; this session provides visual evidence for the startup/continuity repairs only. No master merge, CI or release acceptance is claimed by this branch update.


## Historical VT_Player source-ownership candidate

The second Stage 1 slice is implemented in the isolated player worktree: source sessions own load completion, native operations and presentation runs; cancellation/join gates replacements and transport; callbacks and automatic reload reject retired sources. Native Close disposes partial opens; paused audio Close retains its immutable stop channel; scrubber start/stop are idempotent. Runtime stress caught a candidate lock cycle (widget Refresh under player mu), repaired by queued UI frame refresh without that lock. Final automated media run delivered 398 frames and retired all three source sessions, but visual review is pending because the user was away. Keep this candidate off master/release until that acceptance. Known seek fallback warning and resume timeline movement remain open. Read docs/PLAYER_DEBUG.md and docs/VT_PLAYER_UPGRADE_PLAN.md. Candidate version dev95 must be rechecked against concurrent releases before landing.


## Dev94 player cancellation foundation

Dev94 foundation: Retirement-aware frame/clock waits and retired-read suppression implemented; full native gate passed. A visible software-decode session delivered 243 frames, exercised pause/resume/2x and closed cleanly; runtime log reviewed with no ERROR/WARN/drop records. User reported slight jitter at the end; smoothness remains open. Source generations, one presentation consumer, native I/O interruption and seek correctness remain outstanding. See docs/VT_PLAYER_UPGRADE_PLAN.md and docs/PLAYER_DEBUG.md. Release CI/tag status must be confirmed before claiming publication.


## Active VT_Player upgrade plan

[VT_PLAYER_UPGRADE_PLAN.md](docs/VT_PLAYER_UPGRADE_PLAN.md) records the user-authorized sequence: reliable playback -> trustworthy navigation -> synchronized comparison -> short upscale previews -> inspection/performance. First slice: retirement-aware frame reads; candidate branch only until real playback and log review. Preserve the InlineVideoPlayer API and approved dual-stream exceptions. Encrypted-disc acceptance remains outstanding; no backend/workflow change is included.

## VT_Player audit follow-up (2026-10-12; dev93 baseline)

[VT_PLAYER_AUDIT_DEV93.md](docs/VT_PLAYER_AUDIT_DEV93.md) records 15 open findings. Incorrect handwritten FFmpeg seek constants are confirmed against installed headers; precise seek passes byte/frame flags with timestamps. Repair source lifetime/presentation ownership and transport correctness before profiling. Audit build/vet/full tests passed; no current playback acceptance. Historical seekGen-lastSeekGen log spam remains fixed. PLAYER_DEBUG's old green table is historical. Backend/workflow changes remain subject to existing approval rules; encrypted-disc acceptance is outstanding.

Dev93 validation: scripts/windows/dev-verify.ps1 passed build, vet and the full native_media test suite. Real-disc tester acceptance remains outstanding.

Dev93 verification finding: The runtime card test also exposed bulk-selection lock reentry: toggleCard acquired cb.mu before checking the programmatic updating guard. The guard now returns before locking, so Select All/Deselect All can change a visible check while holding cb.mu.

These rules apply to **every** agent working in this repo — Claude, opencode, Gemini, and any future addition. They are binding. If a rule here conflicts with an agent's default behavior or harness instructions, **this file wins**. Shipped-work history lives in `DONE.md` and `docs/CHANGELOG.md` — this file records only what an agent needs to act correctly *now*.

## Current Project State

- **Cycle: v0.1.1-dev99** — Rip title cards have editable output names. Single-title files combine title name + disc Title, then optional chapter suffix. Project-scoped successful-job reset clears only the matching current disc Title; failures, queued filenames and newer project edits remain intact. [Design](docs/RIP_OUTPUT_NAMES.md). Recycled card handlers bind current ownership before programmatic SetText. Real-media output and visual acceptance remain open.

- **Cycle: v0.1.1-dev98** — Rip selected chapters accepts page-style chapter groups and queues a separate file for each group per selected title. Validate all titles before enqueue; new jobs reject unavailable runtime chapter bounds instead of clamping. All chapters and legacy ranges retain their established behavior. [Design](docs/RIP_SELECTED_CHAPTERS.md). Encrypted-disc/content acceptance remains outstanding.

- **Cycle: v0.1.1-dev96** — Root hygiene audit: removed the unused author DVD-rip tab and untracked generated test executable; moved historical reports to docs/history and the standalone QR prototype to scripts/legacy. Active package-main adapters, platform/stub variants and embedded assets remain required. Evidence: [ROOT_CLEANUP_DEV96.md](docs/ROOT_CLEANUP_DEV96.md). Current triage belongs in docs/TRIAGE.md. No workflow or playback change.

- **Cycle: v0.1.1-dev95** — Convert now gives metadata the flexible workspace while optional preview stays compact. Collapsed sections use their actual header height, replacing the 50/50 and 3-percent split. The native Frame-only strip is removed; labeled Player tools expose source-track and frame/cover actions. Load Video and source identity remain available with preview closed. SMPTE branding and saved preview preferences are retained. Saved-position loading now uses direct timed preview decoding before playback starts; precise seeking and transient resumed-frame distortion remain open. Validation: native build, vet and full tests passed; non-native build passed. Real Windows loaded-media playback, pause, optional-preview and tools checks plus runtime log review are recorded in PLAYER_DEBUG.md. Linux, full scaling/font coverage and encrypted-disc acceptance remain pending. Suite design [UI_SUITE_DESIGN.md](docs/UI_SUITE_DESIGN.md), initial evidence [UI_SUITE_AUDIT.md](docs/UI_SUITE_AUDIT.md). Human Director: readability is a requirement; modules share visual language but task priorities determine panel allocation. Convert metadata outranks preview; preview stays optional; SMPTE pattern remains central. Locked modules remain skipped; encrypted-disc acceptance and the coordinated player repair sequence remain outstanding.

- **Previous cycle: v0.1.1-dev93** — Rip Content Browser cards now render their labels, thumbnail area and selection checks. The dev91 owner wrapper embedded a container but was neither a concrete Fyne container nor a widget, so the canvas skipped its children. cardBox now extends BaseWidget and exposes the original container through a simple renderer; stable owner-based selection remains intact. Rendering regression coverage added. [Rip audit](docs/RIP_AUDIT_DEV93.md) records open defects without reopening settled constraints. Encrypted DVD → Rip → Convert → playback remains priority 1; source-state defects precede that acceptance. Tester playback remains outstanding.

- **Previous cycle:** `v0.1.1-dev92` — Rip: output path follows the Title field live. Editing the Title field did not update the output path box (the two only agreed at load time). Root cause: Fyne's `Entry.SetText` fires `OnChanged` **synchronously** when the text changes (`_fyne/widget/entry.go`), so every programmatic write to the output entry — `applyOutputPath` (source/format change, disc load), `applyControls` (config load / reset), and the Clear handler — ran the entry's own `OnChanged` and set `viewState.outputTouched = true`. The Title handler only re-derives the path while `!outputTouched`, so after the first programmatic write the flag was stuck true and later Title edits were ignored. Fix: a `viewState.outputUpdating` guard wraps every programmatic `outputEntry.SetText` and the entry's `OnChanged` early-returns while it is raised, so `outputTouched` (and the `SetRipOutputPath` persistence callback) fire only for genuine hand-edits. Contract preserved: a hand-edited path still stops auto-recompute; load / reset / config still re-derive from Title + source + format. Regression guard `TestInvariantOutputEntrySetTextIsGuarded`, mutation-verified against both the handler and the `applyOutputPath` write site. `dev-verify.ps1` full gate green. **Version cadence (Human Director): every run with a new fix or update is pushed as a new dev version; small dev versions, not crammed releases; `v0.1.2-dev*` is reserved for the suite working within its own ecosystem — author a DVD in VideoTools, then rip it back in VideoTools.** **Next: real-media acceptance (DVD → Rip → Convert → playback) on an ENCRYPTED disc is priority 1 — higher than the player; the seek concurrency fix is deliberately deferred (one lifecycle problem, not six independent patches).** Playback/real-disc verification is still outstanding, so `docs/PLAYER_DEBUG.md` remains un-updated by deliberate deferral.
- **Dev91 released**: Rip Content Browser selection corruption + rip-mode radio re-click fixed. Two tester-facing glitches. **(1) Scroll-selection corruption.** While scrolling a multi-title rip list, rows randomly ticked/un-ticked themselves and the "ready to rip N titles" count drifted from the selection. Root cause: `updateCard` re-wired each card's `OnChanged` handler **after** `SetChecked`, and the Fyne List recycles card widgets across rows while scrolling — `SetChecked` fires `OnChanged` synchronously, so a recycled card bound to row J ran row I's stale handler and wrote J's display state into I's selection (also clearing mode locks and firing the locked-click escape on stale locked rows). Fix: the handler is wired **once** per template card (`buildCardTemplate` → `toggleCard`) and resolves the current row through a per-card owner (`cardBox.owner`) bound to the state **before** `SetChecked`; `setAllSelected` only drives a control whose card actually belongs to the row it is updating. **(2) Mode-radio re-click.** A Fyne `RadioGroup` deselects an already-selected option when tapped again and fires `OnChanged("")`; the handler's `value == ""` branch hardcoded `SetSelected(t.RipModeMainFeature)`, so re-clicking any mode jumped the radio to Main feature — it now re-arms the *actually active* mode (`labelForMode(vs.extractMode)`), keeping the folder-source `regionConvert` guard. Two mutation-verified regression guards: `TestInvariantCardSelectionHandlerIsStableAcrossListRecycling`, `TestInvariantRadioReclickKeepsTheActiveMode`. **Next: real-media acceptance (DVD → Rip → Convert → playback) on an ENCRYPTED disc is priority 1 — higher than the player; the seek concurrency fix is deliberately deferred (one lifecycle problem, not six independent patches).** Playback/real-disc verification is still outstanding, so `docs/PLAYER_DEBUG.md` remains un-updated by deliberate deferral.
- **Dev90 released**: **H.264/H.265 restored in the shipped Windows FFmpeg.** The Windows sidecar `ffmpeg.exe` built by `dev.yml`/`release.yml` was configured without `--enable-libx264 --enable-libx265` (only `windows-msix.yml` ever built them), so every H.264/H.265 output — rip, convert, upscale, subtitle burn-in, thumbnails, benchmark — failed at encode time with `Unrecognized option 'crf'. Error splitting the argument list: Option not found`, while the UI still offered the codecs (the startup probe only *labels* the format list, it never gates it). Both Windows recipes now build x264 + x265 static-only from source before configure (the recipe already proven in `windows-msix.yml`), add `--enable-libx264 --enable-libx265`, add `git` to the MSYS2 install set, and bump the FFmpeg cache key `v12` → `v13`; the build hard-fails if the encoders are absent, and a new **Verify encoders (libx264 / libx265)** step runs every job including cache hits. Linux ships no FFmpeg (its jobs build libraries only, `--disable-programs`, and the tarball carries just `VideoTools`) and is unaffected. **Next: real-media acceptance (DVD → Rip → Convert → playback) on an ENCRYPTED disc is priority 1 — higher than the player; the seek concurrency fix is deliberately deferred (one lifecycle problem, not six independent patches).** Playback/real-disc verification is still outstanding, so `docs/PLAYER_DEBUG.md` remains un-updated by deliberate deferral.
- **Dev89 released**: Plus Jakarta Sans font option — Settings → Appearance → font select gains a third choice: **Plus Jakarta Sans** (OFL, four static TTF styles bundled from the upstream Tokotype repo — static instances, not the variable font, which Fyne's renderer does not select named instances from), alongside IBM Plex Mono (default, kept) and VCR OSD Mono (kept). Tester report: the monospace-only options read as "slightly too fancy or engineering focused". Preference value `"sans"` flows through the existing `PlayerFont` config key (no schema change; stale configs unaffected); `MonoTheme.Font` dispatches to the `sansFontData` family with full Bold/Italic resolution; Aboriginal Sans syllabics fallback and the SMPTE test-pattern font are unaffected. New i18n key `SettingsFontSans` (en/fr; iu falls back to en-CA, matching the existing font-name keys). `dev-verify.ps1` full gate green. **Version cadence (Human Director): every run with a new fix or update is pushed as a new dev version; small dev versions, not crammed releases; `v0.1.2-dev*` is reserved for the suite working within its own ecosystem — author a DVD in VideoTools, then rip it back in VideoTools.** **Next: real-media acceptance (DVD → Rip → Convert → playback) on an ENCRYPTED disc is priority 1 — higher than the player; the seek concurrency fix is deliberately deferred (one lifecycle problem, not six independent patches).** Playback/real-disc verification is still outstanding, so `docs/PLAYER_DEBUG.md` remains un-updated by deliberate deferral.
- **Dev88 released**: **Repo baseline reconciliation.** (1) **Pre-dev88 reconciliation** (`db51c395`, `docs/history/RECONCILIATION_DEV88.md`): provenance audit of the unexpected `445a0023` additions — `internal/ffi/` (Rust `vt_ffi` staticlib) + `internal/app/modules/bridge/` (CGo consumer via `InterceptMediaPayload`) are intentional and interdependent, retained; `**/__pycache__/` gitignore entry closes the committed-artifact hygiene violation. (2) **Seek audit findings preserved in version control** (TODO.md dev87 index): confirmed defects, disproved historical seekGen-lastSeekGen bug, UNVERIFIED layout claims D1–D15 kept distinct. (3) **No-confirmation-loop directive** (`14a189c7`): complete single-track tasks end-to-end before returning to the human — asking "should I continue?" when the next move is already determined is a defect.
- **Dev87 released**: **Player UI audit and agent toolchain.** (1) Forensic audit of media player seek logic — confirmed seeking ignored, erratic timeline jumps, timestamp drift, UI-thread stall (WaitForPTS unbounded); disproved historical seekGen-lastSeekGen bug (fixed at playback.go:607); root cause: Trim module bypasses Engine.Seek via SmoothScrubbing decoder. (2) Agent toolchain documentation — docs/AGENT_TOOLS.md with complete registry of 5 agents (vt-auditor, vt-reviewer, vt-verifier, vt-verifier-local, vt-auditor-local), exposing quota-exhausted cloud models and missing local fallback.
- **Dev85 released (2026-10-07)**: **CSS decryption wiring.** The rip executor decrypts CSS-encrypted discs entirely in software before ffmpeg sees them: `internal/dvd/css` implements the CSS1/A cipher family (disc key, title/match keys) and the libdvdcss `DVDCSS_METHOD_TITLE` key-recovery route — disc key recovered from the key-region period, per-VTS title keys + VMG key — and `DecryptVideoTS` copies the VIDEO_TS tree into a `vt-css-decrypt-*` scratch dir, decrypts VOB payloads per-VTS (VTS_XX_1..N share the VTS_XX_0 menu key; VIDEO_TS.VOB is its own VMG-domain group; IFO/BUP verbatim), swapped into the pipeline BEFORE `CollectVOBSets` so dvdvideo/concat/cells/menus/full-disc/archivist all read plaintext. Tables byte-identical to libdvdcss csstables.h (0 diffs); `cssTab1` is NOT an involution (64704/65536 round-trips fail) so the encrypted-sector direction is `S = cssTab1Inv[P ⊕ ks]`. CSS failures fast-fail with classifiable messages (css-crack generate/decrypt/scan/decode/no-CSS-crack/fromDisk/fromVTS/unsupported-state/unsupported-title/cssSectorBytes). Independently: **`verifyRipOutput`** refuses to declare success on an output that is missing, trivial (< 1 kB), or has no ffprobe video stream — the empty-container-reported-as-clean corruption class is closed. Commits: `252aa991` (fixture direction), `faafce65` (executor wiring), `6a33dd96` (output verification).
- **Dev84 released (2026-10-05)**: **Rip audit + stabilization, then the dormant libVLC backend cut.** The Rip audit was run to establish the module's *actual* state before touching it, and it changed the picture materially. **Four behaviours the tracker implied were defective are confirmed already correct and are now closed facts, pinned as mutation-verified regression constraints** — they are NOT cleanup candidates: (1) `dvdvideo -title` precedes `-i` (`executor.go:341`; the dev79 root cause, with a comment so it doesn't regress), (2) bulk Select All clears the mode lock and raises the per-card `updating` guard around `SetChecked` (`content_list.go:292`; dev79), (3) an all-chapter range is a whole-title passthrough (`executor.go:763`), (4) chapter ranges resolve semantically through `chapterRange()` and restrict the cell-accurate concat list. Guards live in `internal/app/modules/rip/invariants_test.go`; each was mutation-verified. Two real defects were found and fixed: the `dvdvideo`→concat **retry gate was a deny-list** that admitted output-side failures, so an unwritable destination or full disk launched a second full rip then reported `dvdvideo demuxer failed (…)` — now `classifyRipFailure` attributes source-structure/selection/output/execution/cancelled from ffmpeg's own stderr and only source-structure retries (`40ec5889`); and **`addToQueue` branched on title COUNT**, so a one-title disc bypassed the selection entirely — since dev76 made "Movie + extras" start empty, such a disc read "no titles selected" and then ripped anyway — now `ripTargetTitles` makes the selection the sole authority at every title count (`982e8066`). **Process correction:** `dev-verify.ps1` ran build+vet but never the tests, which is why a DLL `PATH` gap read as three packages of "pre-existing failures" across two commits; `internal/media`, `internal/app/appcfg`, and `internal/app/modules/rip` were **never failing** and are green (`b441655b`). **Next: real-media acceptance (DVD → Rip → Convert → playback), then assess Upscale.** Playback/real-disc verification is still outstanding, so `docs/PLAYER_DEBUG.md` remains un-updated for this cycle by deliberate deferral.
- **Public/stable baseline:** `v0.1.1`.
- **Planning sources:** `TODO.md` (scope), `docs/roadmap.html` (canonical tracker), `DONE.md` + `docs/CHANGELOG.md` (shipped history).
- **Issue tracker:** https://github.com/LeakTechnologies/VideoTools/issues
- **Player debug log:** `docs/PLAYER_DEBUG.md` — update before closing any player-related issue.

## Rip — established state (do not re-audit; do not "fix" the closed facts)

The audit's job was to find out what was true. It is now known. Treat the following as **regression constraints on subsequent Rip work**, not as open items:

| Behaviour | Where | Why it is load-bearing |
|---|---|---|
| 1 | Real-media acceptance: DVD -> Rip -> Convert -> playback | The point of the audit was to reach a state where this ladder can be walked. Then assess what Upscale needs. Rip S1 defects are fixed and the suite is green; this is the remaining proof |
| `-f dvdvideo -title N -i <path>` — title before input | `executor.go:341` | FFmpeg binds a post-`-i` demuxer option to the *next* input; it landed on the ffmetadata chapters input, FFmpeg rejected the run, and every rip fell back to whole-file VOB concat, which rips the **first title's content for all titles** (dev79). Silent corruption: the job reports success. |
| Bulk Select All clears `locked`/`anchored` and raises `tc.updating` around `SetChecked` | `content_list.go:292` | Respecting the lock selects nothing (dev79). Removing the guard **deadlocks the UI thread**: `SetChecked` fires `OnChanged` synchronously and the handler re-acquires `cb.mu`, held across the loop. |
| Card `OnChanged` is wired **once** per template (`buildCardTemplate` → `toggleCard`); each card binds `box.owner` to its row **before** `SetChecked`; `setAllSelected` only drives a check its own card owns | `content_list.go` (`buildCardTemplate`, `updateCard`, `setAllSelected`) | The Fyne List **recycles card widgets across rows** while scrolling; a handler re-wired in `updateCard` after `SetChecked` runs the previous row's stale handler and writes one row's display state into another's selection (dev91 scroll corruption). Owner-before-`SetChecked` is what makes the once-wired handler resolve the *current* row; the ownership guard stops a bulk update from driving a check the card does not own. |
| Programmatic `outputEntry.SetText` raises `vs.outputUpdating`; the entry's `OnChanged` early-returns while raised | `view.go` (`applyOutputPath`, `applyControls`, Clear handler, output `OnChanged`) | `Entry.SetText` fires `OnChanged` **synchronously** when the text changes (dev92), so an unguarded programmatic write is indistinguishable from a hand-edit and sets `outputTouched`, freezing the Title→path auto-derive. The guard is what keeps `outputTouched` (and the `SetRipOutputPath` callback) firing only for genuine user edits. |
| `if !(cs == 1 && ce == n)` — all-chapter short-circuit | `executor.go:763` | A `From=1/To=N` range is the whole title. Removing it routes a whole-title range through the cell-slice path, re-slicing cells that already span the title. |
| `chapterRange()` → semantic span, restricted concat list, remapped chapters | `executor.go:743-775`, `cellconcat.go` | Chapter N of the output must still mark the same boundary. |
| Selection is the only authority for whether a title rips | `ripmode.go` `ripTargetTitles` | dev76 made "Movie + extras" start empty. Any count-based bypass makes the readiness line and the queue disagree. |

**Retry policy is now stated positively.** `classifyRipFailure` reads ffmpeg's stderr. Output markers are tested **before** source markers because ffmpeg emits `libdvdread` chatter on runs that later fail at the write stage. Unattributable failures deliberately do **not** retry — a retry buries the real error under the fallback's. Never reintroduce a `!isSomething` deny-list gate on the input-swapping retry.

## Next sequence (Human Director ruling)

`Rip audit → stabilize Rip → prove DVD (real media) → Rip → Convert → playback E2E → then determine what Upscale needs`

Real-media acceptance ladder: DVD source → select intended title/scenes → rip succeeds → output loads into Convert → converts to a known test format → plays. Only after that is reliable should Upscale be scoped. Player changes still require a real playback session + `docs/PLAYER_DEBUG.md` before landing.



## Current Priorities (unshipped only)

| Priority | Item | Notes |
|---|---|---|
| 1 | Real-media acceptance: DVD -> Rip -> Convert -> playback | The point of the audit was to reach a state where this ladder can be walked. Then assess what Upscale needs. Rip S1 defects are fixed, CSS decryption ships in software, and the suite is green; this is the remaining proof — now including an ENCRYPTED disc. dev90 restored the missing H.264/H.265 encoders in the shipped Windows FFmpeg, a prerequisite for this ladder |
| — | Dev-version cadence (standing) | Every run that lands a new fix or update is pushed as a new dev version. `v0.1.2-dev*` is reserved for the DVD suite working within its own ecosystem: author a DVD in VideoTools, then rip it back in VideoTools |
| 2 | Convert audit #20 + #11 (player state + lifecycle) | #20: one authoritative playback-state boundary for play/pause, speed, and track controls - no stale UI (`U-5/U-6/U-7/U-8`, `F-11`; sibling #18 already closed). #11: source never loaded on switch, rebuild fakes a pause, collapsed player still fed frames (`F-3`, `F-4`, `L-4`). Audit register is #17. Landed in code (`7fcd856d`, `43db26b5`) but **unverified on real media** |
| 3 | Tester verification of dvdvideo rip + concat fallback | CI ships libdvdread/libdvdnav + the dvdvideo demuxer, the runtime probe no longer forces VOB concat, and dev79 fixed the `-title` arg-ordering root cause that made every dvdvideo rip silently fall back to whole-file VOB concat. Re-run a multi-VOB rip and confirm the dvdvideo path engages (exit 0, clean output, no 26h-freeze / slideshow corruption) and that libdvdnav-rejected sources still fall back to the cell-accurate concat list |
| 4 | Updater hardening | Bake `buildCommit` via ldflags in CI builds (same-tag patch detection dead) — ask before touching workflows; Linux tar.gz extraction path; stale nightly-PATCH comment in `fetchUpdateInfo` |
| 5 | Dead-code retirement (post static-sidecar decision) | `scripts/windows/build-ffmpeg-shared.ps1`, DLL-folder branches in `ffmpeg_bootstrap.go`, `updateSidecars` DLL extraction — legacy-harmless, remove deliberately |
| — | Burn multi-drive batch / IMAPI2 COM | `docs/BURN_MODULE_DESIGN.md` §2–3 |
| — | Main Menu refactor to `internal/app/modules/mainmenu/` | LOW — deferred until engine stable |
| — | UDF 2.50/2.60 + BDMV; sparse/large-file UDF writer | Future |

## Settled Decisions

Reached after failed attempts or Human Director ruling. **Do not change or re-litigate without explicit Human Director approval.**

### libVLC Player Backend — implementation CUT (2026-10-04, dev84, HD approved)

The 2026-07-21 ruling below stands as the *design* direction, but the implementation was removed in dev84 (`3b054dd8`). What shipped was never a backend: a `vlc`-tagged partial port that failed to compile (five unresolved libVLC C symbols) behind a tag **no CI workflow ever built**, surfaced to users as a Settings checkbox that did nothing and a hint making a stability claim that was never true for anyone. Do not reintroduce a second backend without all three of: (1) the libVLC SDK provisioned in CI, (2) a CI job that actually builds the backend's build tag, (3) a guarded user-facing control wired to a real reader. The `PlaybackEngine` interface is retained deliberately — it is the seam a real backend plugs into. `docs/VLC_PLAYER.md` is retained as the design record. `TestNoDeadVLCBackendControl` + `TestNoVLCBackendCode` enforce this; re-adding an advert-for-a-missing-backend string, or reintroducing the plumbing, turns them red.

### libVLC Player Backend (2026-07-21, HD approved)

Replace the custom FFmpeg demux/decode/sync engine with libVLC for user-facing playback. The FFmpeg engine stays as a long-term plan when it matures to a usable state. The custom engine has had 10+ crash-fix cycles across dev43–dev55; its architecture (6 goroutines, 4 mutexes, 3 packet queues) is too complex to stabilise. libVLC has solved these problems for 20+ years. `adrg/libvlc-go/v3` does NOT expose `libvlc_video_set_callbacks` — a custom CGo wrapper is required (~300 lines vs FFmpeg's ~1500). Design doc: `docs/VLC_PLAYER.md`.

### Windows Product: Three Fully Static Binaries (2026-07-04, HD approved)

`VideoTools.exe`, `ffmpeg.exe`, `ffprobe.exe` are each fully self-contained. **No shared FFmpeg build. No DLL/ folder.** Enforced by objdump gates in CI that fail the job on any MinGW runtime DLL reference (`libbz2|liblzma|libiconv|libstdc++|libwinpthread|libgcc|zlib1`). App treats static sidecars as primary (`appcfg.StaticSidecarsWork()`); DLL-folder paths remain only as legacy-bundle support.

### CI Toolchain — FFmpeg, x264, x265, libdvdread/libdvdnav

- **FFmpeg is built from source** — one static build per platform serves both the CGo link and the sidecar programs (`--extra-ldflags="-static"` on Windows; never `--disable-programs`).
- **BtbN FFmpeg-Builds must NOT be used** — no static `.a` libs, moving-tag ABI drift.
- **x264/x265 built from source, static-only** — MSYS2 prebuilt packages have `__declspec(dllimport)` headers that poison the static link. Built in **all three Windows pipelines** (dev/release/msix; dev90 added them to dev/release after dev89 shipped without them and every H.264/H.265 encode failed). The bundled sidecar `ffmpeg.exe` must expose both encoders; the build gates on `ffmpeg -encoders | grep libx264|libx265` and a **Verify encoders** step runs on every job including cache hits.
- **x265.pc must be overwritten after cmake install** (LF, POSIX paths). C++ deps (`-lstdc++ -lsupc++ -lm` Windows / `-lstdc++ -lm` Linux) go in **`Libs`**, not `Libs.private` — FFmpeg configure calls pkg-config without `--static`.
- **dvdvideo demuxer: libdvdread 6.1.3 + libdvdnav 6.1.1 built from source, static-only**, in all Windows workflows (dev/release/msix), installed into the ffmpeg prefix, then `dvdnav.pc` overwritten so `-ldvdread` sits in **`Libs`** (same reasoning as x265.pc — FFmpeg's Windows configure calls pkg-config without `--static`, so the stock `Requires.private: dvdread` is never expanded and the `dvdnav_open2` link test fails). Linux needs no overwrite (`--pkg-config-flags="--static"` expands `Requires.private`). Each pipeline gates with `ffmpeg -h demuxer=dvdvideo`. Changing FFmpeg/dvd-lib setup invalidates the ffmpeg cache — bump the Windows dev/release `v13` / msix `v6` / forgejo `v9` + `.built-v10` marker keys.
- **drawtext filter: libfreetype 2.13.3 + libharfbuzz 14.4.0 both required from FFmpeg 8.x** — `drawtext_filter_deps="libfreetype libharfbuzz"` (`vf_drawtext.c:1372` calls `hb_ft_font_create_referenced`), so a freetype-only build silently omits drawtext. Both are built from source static-only into the ffmpeg prefix in all four Windows pipelines (freetype via `./configure --without-harfbuzz --without-brotli --without-bzip2 --without-png --with-zlib=no`; harfbuzz 14.4.0 via meson `-Dfreetype=enabled`, every other backend disabled), configured with `--enable-libfreetype --enable-libharfbuzz`. `harfbuzz.pc` is rewritten so `-lfreetype -lstdc++ -lsupc++ -lm` sit in **`Libs`** directly (FFmpeg's Windows configure calls pkg-config without `--static` — same precedent as x265.pc/dvdnav.pc). CI strips `-lsupc++` from pkg-config output; the Go link then expands `-lharfbuzz -lstdc++ -lfreetype` and needs no new flags. The post-build drawtext gate must use `ffmpeg -filters | grep drawtext` — FFmpeg 8.1 moved the filter list away from `-h filters` (which now prints generic options only and would fail on a correct build). Note: only the three GitHub workflows gate; the Forgejo build does not.
- **cmake** in Linux apt deps; **nasm + mingw-w64-ucrt-x86_64-cmake** in Windows MSYS2 install.
- Read the build log before changing FFmpeg setup. Ask before touching the FFmpeg build steps.

### CI Windows Build-Step Facts (each was a real failure — do not regress)

- Build steps run in `shell: msys2 {0}`, **never** `shell: bash` (Git Bash resolves wrong gcc + Strawberry pkg-config).
- `GOROOT` derived inside the shell: `GOROOT=$(ls -d /c/hostedtoolcache/windows/go/*/x64 | tail -1)`.
- `setup-msys2` installs to `D:\a\_temp\msys64` — never hardcode `C:\msys64`; use `CC=$(cygpath -m /ucrt64/bin/gcc)`.
- FFmpeg link flags from `pkg-config --libs --static` with loud `exit 1` on empty output (silent failure previously fell back to the local-dev `-LC:/ffmpeg/lib` in `cgo_preamble.go`).
- Strip `-lsupc++` from pkg-config output; do NOT add a second `-lstdc++` (multiple-definition errors).
- FFmpeg 8.1 needs `-lcrypt32 -lncrypt` (Schannel TLS) beyond pkg-config output.
- Static archives for bz2/z/lzma/iconv/stdc++ are promoted into the ffmpeg prefix (first `-L` dir) so ld never picks MSYS2 `.dll.a` import libs.
- `CGO_LDFLAGS_ALLOW: "-Wl,.*"` at workflow env level.
- GitHub-hosted runners' MSYS2 lacks `git`/`wget` inside the environment — install via pacman, never assume image contents.

### Windows subprocess handles — do NOT set `NoInheritHandles`

`internal/utils/exec_windows.go` must **not** set `SysProcAttr.NoInheritHandles`.
Go's doc is explicit: it blocks inheritance of *all* handles "not even the
standard handles", so the child never receives the stdout/stderr pipes and
`cmd.Output()`/`CombinedOutput()`/`StdoutPipe()` return nothing — which
silently broke every ffprobe metadata read (all imports) and ffmpeg
`-progress pipe:1` on Windows (dev49–dev52). Modern Go (1.16+) passes ONLY the
std-pipe handles via `PROC_THREAD_ATTRIBUTE_HANDLE_LIST`, so the CGo engine's
`avformat_open_input` file handles are NOT leaked to children even without the
flag — the original "file in use" concern does not regress. Crash-safe child
cleanup is the Job Object's job (`jobobject_windows.go`), not this flag.

### CI Workflows

- `.github/workflows/dev.yml` — push to master; Linux + Windows; artifact zips. FFmpeg cache steps carry `restore-keys` so a tag/branch ref reuses the prior ref's cached build.
- `.github/workflows/release.yml` — `v*` tags; same builds + GitHub Release. FFmpeg cache `restore-keys` + curl `--retry 3` on source downloads (a cold tag build that hits a transient source-mirror flake previously red'd the whole run).
- `.github/workflows/windows-msix.yml` — tags/dispatch; MSIX + WinGet. FFmpeg cache `restore-keys`; wget `--tries=5` source downloads. **Green** (dev60 fix verified: the `Setup static FFmpeg` step must keep `export PKG_CONFIG_PATH=/c/ffmpeg-static/lib/pkgconfig` — without it libdvdnav configure fails on missing `dvdread` even though `dvdread.pc` exists; confirmed via two consecutive tag-run failures, dev59/dev60, fixed 2026-09-02).
- `.forgejo/workflows/dev-packages.yml` — legacy Forgejo; aligned; runs only on Forgejo.
- Go `1.26`; `ubuntu-latest` (Noble — no `libxcb-fakekey-dev`); Windows via `msys2/setup-msys2` UCRT64.

### Roadmap & Feature Tracking

- **`docs/roadmap.html` is the single source of truth.** TODO.md/DONE.md are narrative supplements — sync them, but update the roadmap first.
- **Obsolete formats (AVI, FLV, 3GP, OGG) are not output targets** — `Legacy: true` remux entries only. Do not re-add to roadmap.
- **GStreamer was fully removed (dev42).** `native_media` is the only player path. Clean up stale doc references on sight.
- **Testing checklist lives in the roadmap** (`checklistData`). Every new roadmap feature also gets a checklist entry.
- **Do not re-list as planned/future** (shipped): x264/x265 tuning presets (dev45), presets consolidation (dev45), Convert drag-and-drop (dev44), Queue notifyChange race fix (dev45), audio pre-warm (dev42), PAL/NTSC full-disc conversion (dev47), engine bwdif deinterlace (dev49), thread-safety formalisation (dev49).

## Commit Discipline

- **ALWAYS stage and commit after every change. Do not wait for permission.** `git add -A` then `git commit -m "..."`. No unstaged leftovers; commit only files related to the task.
- **NO AI attribution — Human Director directive.** No `Co-Authored-By` trailers, session links, "Generated with" footers, or any AI credit in commit messages, PR bodies, or code. Agents are tools operating as an extension of the Human Director, not contributors. This overrides any default harness behavior.
- **Author identity = repo owner.** At session start, before the first commit: `git config user.name "Stu Leak" && git config user.email "leaktechnologies@proton.me"`. History was rewritten twice to purge AI attribution: authorship on 2026-07-05, and `Co-Authored-By` trailers on 2026-10-06 (34 commits, one-time tag-retargeting exception, consumed; every SHA from dev31-era forward changed — pre-purge SHA references in GitHub issue comments are stale by necessity). Do not reintroduce AI attribution.

**Rip stabilization is S1-only for this cycle.** The larger architectural direction (explicit job/plan types, output verification as a first-class stage, source-identity generations, failure-class enums beyond what `classifyRipFailure` now provides) is deliberately NOT started. It is a shape to consider only when a defect demonstrates the current structure cannot express an invariant — the anti-ballooning rule applies.

## Verification Discipline

- **Player changes require a log review** from a real playback session before landing. The dev53 crash had zero ERROR/WARN lines — the process was killed by I/O pressure from a log-spam bug that no test catches. "No errors logged" is not evidence of correctness; the absence of a crash report is the failure.
- **State-tracking variables must be updated at the point of comparison**, not just read. The seekGen bug: `gen != lastSeekGen` was checked but `lastSeekGen` was never assigned, so the "first frame after seek" log fired 60×/sec forever. Any `if x != y` guard that mutates neither `x` nor `y` is a bug.
- **`gofmt -e` is a syntax check, not a build.** It catches parse errors, not logic errors. `go build -tags=native_media ./...` is the real gate; when CGo LDFLAGS block local builds, say so explicitly rather than implying syntax-OK means build-OK.
- **Local Windows builds: use `scripts/windows/dev-verify.ps1`** (build `-tags=native_media ./...` + `go vet ./...` + `go test -tags=native_media ./...`). Bare `go build` fails at the CGo gate on the `-Wl,--stack,4194304` LDFLAG because `CGO_LDFLAGS_ALLOW` is only set in CI/bash. The script sets `CGO_ENABLED=1`, `CGO_LDFLAGS_ALLOW=-Wl,.*`, quotes a discovered gcc/g++, and prepends `C:\ffmpeg\bin` to `PATH`. Expect a long cold build (~9 min), then incremental runs are fast.
- **`0xc0000135` in a Go test is `STATUS_DLL_NOT_FOUND`, not a test failure.** It means the binary could not *start*. The CGo-linked packages' test binaries need the FFmpeg shared DLLs from `C:\ffmpeg\bin`. `dev-verify.ps1` now sets that up and names this code explicitly. Do not record a package as carrying pre-existing failures on this signal — that misdiagnosis was made once and produced two wrong commit messages plus a false "baseline failure" note carried across three packages. If a package cannot start, fix the environment before concluding anything about its tests.

## Anti-Rationalization Table

Pre-written rebuttals to shortcuts an agent (or a tired engineer) might take. If you catch yourself thinking the left column, read the right.

| Excuse | Rebuttal |
|---|---|
| "The log spam is cosmetic, ship it." | 60 lines/sec of I/O forever caused a hard process crash in dev53. Cosmetic log bugs are stability bugs. |
| "This task is too small to need a version bump." | If it fixes a crash or changes user-visible behavior, it's not small. The tester needs a build number to anchor feedback to. |
| "Tests pass, ship it." | Passing tests are evidence, not proof. dev53 had zero test failures and still crashed. Did you check the runtime log? Did a human play the video? |
| "I'll update PLAYER_DEBUG.md later." | Later is the load-bearing word. There is no later. The debug doc is updated in the same commit as the fix or it doesn't get updated. |
| "The existing code looks wrong but I don't know why — I'll refactor it." | Chesterton's Fence. If you don't know why it's there, you don't know what it protects. Read the history, ask, or leave it. |
| "I can't build locally (CGo LDFLAGS), so I'll just verify with gofmt." | gofmt checks syntax, not semantics. State explicitly that the build wasn't verified and why. Don't imply confidence you don't have. |
| "The agent before me did it this way, so it must be fine." | The previous agent may have been wrong. Verify against the rules, not against history. |

## Documentation Discipline

Every landing **updates all six documents in the same commit**:

| Document | Update |
|---|---|
| `docs/roadmap.html` | Card status/cycle/desc; new cards; changelog + checklist data |
| `docs/ROADMAP.md` | Timeline entry; Current State; Now/Next |
| `docs/CHANGELOG.md` | Bullet under current dev cycle |
| `AGENTS.md` | Settled decisions, priorities, current state |
| `DONE.md` | Completed-feature entry |
| `TODO.md` | Check off done; add newly scoped |

- Behavior changes also update `docs/INSTALLATION.md` + the platform guide (`docs/INSTALL_WINDOWS.md` / `docs/INSTALL_LINUX.md`).
- No personal names in docs — `user report` / `dev report` only.
- **No media/disc titles anywhere in the repo** — not in code, comments, log
  messages, test fixtures, docs, **commit messages**, or GitHub issues (titles,
  bodies, comments). The rip corpus is adult material; refer to discs neutrally
  as "a grey-market DVD", "a scene-segmented disc", "a disc with N titles in
  VTS_01", "the ISO 9660-only sample", etc. Behaviour that depends on a
  specific disc is described by its *structure* (PGC layout, stale-PTS offset,
  liar-IFO subtitle count, shared-VTS cells), never by what the film is called.
  A repo-wide `rg -i -e '<title fragment>' -g '!_fyne/**'` scan should come
  back empty before landing a commit that touches rip docs.
- **Do not over-scrub.** Generic placeholder filenames and example labels are
  fine and must be left alone — `test.mkv`, `output.iso`, `title_01.mpg`,
  `vacation.mp4`, `Family Vacation`, `VTS_XX_0.VOB`, `previous title.mkv`.
  The rule targets real film titles, not innocuous sample names. Feature prose
  may also keep neutral `test` / `Title A` style placeholders.
- The retired `docs.leaktechnologies.dev` site must not be referenced.
- New features get `docs/FEATURE_NAME.md` (overview, implementation, files, testing checklist) **before** implementation, linked from TODO.md and this file.
- Active feature: `docs/RIP_CHAPTER_RANGE.md` (dev81 — rip a title by a chapter range).

### Roadmap Card Rules

Statuses: `shipped` (tester-confirmed) / `done` (committed, CI green) / `active` / `planned` / `future`.
On landing: set `done`, set `cycle`, correct `desc`/`files`. On tester sign-off: `shipped`. On cycle close: add `changelogData` + `checklistData` entries, bump subtitle. Never leave landed work at `planned`; never park at `done` indefinitely; never edit roadmap.html without syncing the other five docs.

## Version Bumping & Release Protocol

- After every major change: bump `main.go`, `VERSION`, `FyneApp.toml`; update DONE.md/TODO.md/CHANGELOG.md.
- `v0.1.1-devN` = rolling dev line; `v0.1.1` = stable baseline; dev numbering continuous; public bumps are readiness-based.
- **When work warrants a new dev version** (major feature, significant fix batch, anything tester-facing), the agent drives the release end-to-end:
  1. Bump the three version files + the six docs (same commit), push to master, confirm dev CI green.
  2. Tag the release commit `v0.1.1-devN` and push the tag — this triggers `release.yml` (GitHub Release with both platform assets).
  3. Watch the release run to green; report the release URL.
- **Release authorization:** the Human Director authorizes automatic commit and release-tag pushes using existing Git credentials. Prefer `$VT_RELEASE_TOKEN` when available; otherwise push through the configured origin. Do not require manual tagging merely because that environment variable is absent. Never retarget existing tags.
- Tag-triggered runs execute the workflow file **as of the tagged commit** — always tag a commit that already contains the current workflow fixes.

## Internationalization (i18n)

All user-facing strings use `i18n.T().KeyName` — never hardcoded literals.

1. Add key to `internal/i18n/strings.go`, English to `en_ca.go` (source of truth), French to `fr_ca.go`, Inuktitut to `iu.go` + `iu_latin.go` (machine-generated OK with `// machine-generated, needs human review`; see `docs/localization-policy.md`).
2. Key naming: `ModuleXxx`, `ActionXxx`, `LabelXxx`, `StatusXxx`, `DialogXxx`.
3. Before landing, grep for stragglers: `widget.NewLabel("`, `widget.NewButton("`, `dialog.Show...("`.

## Repository Hygiene

- Root stays minimal: core manifests, primary app entry source, `README.md`/`TODO.md`/`DONE.md`. Demos/tools under `cmd/` or `scripts/`; packaging under `packaging/<platform>/`. Historical reports belong in `docs/history/`; active triage belongs in `docs/TRIAGE.md`. No ad-hoc logs/scratch/backup files or compiled test executables in the tracked root.
- **No new root-level `.go` files — hard rule.** New code goes in the appropriate `internal/` package. App-level glue may use a *temporary* root shim only if listed as an extraction target in `docs/REFACTOR_DEV30_PLAN.md`. When unsure, default to `internal/` and ask.

## Refactor Boundaries

- Plan: `docs/REFACTOR_DEV30_PLAN.md`. Phase 2 complete; **opencode owns Phase 3**.
- Already extracted (do not re-extract): `about`, `deps`, `mainmenu` helpers, `convert` entry point, `player_module.go`, `enhancement_module.go`, `upscale_module.go`, `compare_module.go`.
- Remaining large blocks in `main.go` (~16.9k lines): `buildConvertView` (~3.5k, needs appState decoupling first), Inspect view, Settings view, Queue view.
- Pure-move slice pattern: new `<name>_module.go` in `package main` → move `show*`/`build*` verbatim → copy needed imports → delete from `main.go` → commit as one slice. Small, reversible; never mix structural moves with feature work.
- Long-term: root `package main` shrinks, logic moves to `internal/app/`, entrypoint toward `cmd/videotools/`.

## Platform Scope

- **Linux + Windows only. No macOS** — no darwin code paths, CI jobs, or docs; delete `case "darwin":` blocks on sight.
- Linux = primary dev platform (implement properly here first; small runtime deps OK, e.g. `dvd+rw-tools`).
- Windows = primary user/tester platform (**zero new runtime dependencies** — OS built-ins only, e.g. `isoburn.exe`).
- Windows installs via `scripts/windows/install.ps1|.bat`; `scripts/linux/install.sh` is bash-only.

## Native Media Player

Architecture reference: `docs/NATIVE_PLAYER.md` — read before touching player code.

**The One Rule: all modules use `ui.InlineVideoPlayer` as their API layer.** No `media.NewEngine()` in modules, no per-module playback goroutines, no direct `media.NewVideoPlayer()`. Widget via `opts.Player.Widget()`.

```
internal/media   Engine            — CGo/FFmpeg demux + decode + audio (oto v3)
internal/media   VideoPlayer       — Fyne widget: frames + controls overlay
internal/ui      InlineVideoPlayer — THE API layer every module talks to
```

New module with video: singleton in `native_media.go` (+ stub in `native_media_stub.go`), `Player *ui.InlineVideoPlayer` in Options of both `view.go` and `stub.go`, wire `Player: GetXxxPlayer()` in `main.go`.

| Module | Getter | Notes |
|---|---|---|
| Convert | `GetConvertPlayer()` | custom controls; builtin disabled |
| Trim | `GetTrimPlayer()` | builtin overlay; in/out markers |
| Inspect | `GetInspectPlayer()` | builtin disabled; `SetOnLoad` → Sync pills |
| Filters | `GetFiltersPlayer()` / `GetFiltersPreviewPlayer()` | preview follows via `SetPeer`, muted |
| Upscale | `GetUpscalePlayer()` / `GetUpscalePreviewPlayer()` | same pattern |

Build-tag gating: `native_media.go` (real) / `native_media_stub.go` (no-op); `internal/media/` + `inline_player.go` compile only with the tag. **Stubs must expose the identical method set.**

**Approved exceptions** (dual simultaneous streams — do NOT refactor to InlineVideoPlayer): Compare `compare/fullscreen_native.go`, Upscale `upscale_player_native.go`.

**SEH/VEH crash bridge** (`internal/media/safe_bridge.c`): MinGW VEH (`ACCESS_VIOLATION`, `STACK_OVERFLOW` + `_resetstkoflw`), MSVC `__try`, Linux/macOS SIGSEGV handler. Wraps `safe_avcodec_send_packet`, `safe_avcodec_receive_frame`, `safe_av_hwframe_transfer_data`, `safe_sws_scale_frame`. Do not rewrite without understanding all platform paths.

## Agent Pipeline

| Agent | Role | Scope |
|---|---|---|
| **Claude (Primary)** | Architect, systems planner, triage, docs | Cross-session context, architecture, CI/release strategy |
| **opencode (Secondary)** | Refactoring, hygiene, small features | Module extraction, structural work (owns Phase 3) |
| **Gemini (Tertiary)** | Isolated specialist tasks | Contained problems, minimal cross-project context |

Phases: Design (Claude) → Implementation (all) → Refinement → Versioning.

### Sub-Agents (Claude Code)

Types: `Explore` (read-only search), `Plan` (architecture), `general-purpose` (multi-step), `claude-code-guide` (CLI/API questions). Spawn for: repeated fixes across many files, independent parallel tasks, large searches, pre-feature architecture review. Brief like a colleague with zero context (paths, attempts, expected outcome). **Verify results by reading diffs** — a summary describes intent, not fact. Verify build passes before committing.

### Hooks

Configure in `.claude/settings.json` (`/update-config`). Commit-gating hooks `exit 1` on failure; complex logic in `scripts/hooks/`; document new hooks here. Recommended: pre-commit i18n guard (grep widget/dialog constructors for raw strings); post-edit reminder to update `docs/PLAYER_DEBUG.md` on player-file changes.

### Skills (slash commands)

`/review` (PR review) · `/security-review` (branch audit) · `/simplify` (post-feature cleanup) · `/schedule` (future background task) · `/update-config` (hooks/permissions) · `/init` (regenerate CLAUDE.md) · `/triage` (project discovery — CI, issues, TODOs, next actions).

## Loop Engineering (cost-conscious)

The human is the scheduler. Loops are human-triggered, not timer-triggered.

**No confirmation loops (Human Director directive).** If the agent has a single, unambiguous track to follow to complete an assigned task, COMPLETE the task before returning to the human. Do not stop mid-track to ask permission for a step whose answer is already determined by the standing instructions or by the Human Director's last message — asking "should I continue with X?" when X is the only possible next move wastes development time and frustrates the Human Director. Asking again after the answer was already given is worse (see the dev87 version-bump exchange for the canonical example). Ask only when: (1) a genuine decision exists with materially different options, (2) standing rules require approval (workflow entrypoints, release operations, CI changes), or (3) the next step is truly ambiguous and no repo evidence resolves it.

**Daily rhythm:**
1. Run `/triage` → reads CI, issues, TODOs, writes docs/TRIAGE.md
2. Read docs/TRIAGE.md → decide what to work on
3. Implement → commit → push
4. Repeat

**What's cheap (do these):**
- `/triage` — read-only discovery, ~50 lines output
- Skills — written once, loaded only when triggered
- State files — docs/TRIAGE.md, TODO.md, DONE.md (agent reads, doesn't generate)

**What's expensive (avoid unless necessary):**
- Automated scheduled loops — burn tokens while you're away
- Multi-agent verification chains — each agent is a full context load
- "Fix it and keep iterating until tests pass" — unpredictable token cost

**Token rules:**
- Triage under 50 lines. Implementation under 1 commit per task.
- Never run two agents on the same file simultaneously.
- If a loop would take >3 turns, break it into smaller human-triggered steps.
- Verifier sub-agent only for high-risk changes (engine, CI, release).

## Self-Healing UI Diagnostics (opt-in, not a gate)

A self-healing UI harness (capture a layout-clipping / broken-canvas / thread-hang state, feed the snapshot + `FYNE_DEBUG` trace to a free-tier vision model, apply the suggested layout fix, verify, commit) is **planned, not shipped**. Rules if/when it is built:

- **Do NOT gate edits on it.** It is an opt-in diagnostic, never a required pre-merge check. No `scripts/windows/autonomous-ui-heal.py` exists yet; do not assume it does.
- **Go paths, not cargo.** Any harness must drive `VideoTools.exe` + `scripts/windows/dev-verify.ps1` (this is a Go/Fyne repo, not Rust); vision calls read `GOOGLE_API_KEY` from the environment at runtime and never commit a key.
- **Mock-first:** the harness must be proven end-to-end on a synthetic UI-failure fixture (through a real git commit) before it is pointed at a live view.
- **Always human-reviewed:** model-suggested patches are candidates only; the agent applies them, passes the standard build/vet/test gate, and commits with no AI attribution.

## Coordination

- Ask before changing workflow entrypoints or automation behavior.
- Install/build changes get a docs note.
- Release publishing stays aligned to `VERSION`; never retarget or delete existing dev tags; old workflow runs are not evidence of current release state.
