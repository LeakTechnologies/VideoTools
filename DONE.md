# VideoTools - Completed Features

## v0.1.1-dev88 — Repo baseline reconciliation

- **Pre-dev88 reconciliation (`RECONCILIATION.md`, `db51c395`).** File-by-file provenance audit of the unexpected `445a0023` additions: `internal/ffi/` (Rust `vt_ffi` staticlib) and `internal/app/modules/bridge/` (CGo consumer via `InterceptMediaPayload`) confirmed intentional and interdependent — retained, no deletion. Committed `__pycache__` artifact neutralised with a `**/__pycache__/` gitignore entry.
- **Seek audit findings preserved in version control.** TODO.md dev87 carries the findings index — confirmed defects, the disproved historical seekGen-lastSeekGen bug, and UNVERIFIED layout claims D1–D15 kept distinct.
- **No-confirmation-loop directive (`14a189c7`).** AGENTS.md (repo + global opencode config): an agent with a single unambiguous track completes the task end-to-end before returning to the human.
- **Next.** Real-media acceptance on an encrypted disc (priority 1); seek concurrency fix deferred to a later dev version by deliberate scoping — small dev versions, not crammed releases.

## v0.1.1-dev87 — Player UI audit and agent toolchain

- **Forensic audit of media player seek logic.** Confirmed seeking ignored, erratic timeline jumps, timestamp drift, and UI-thread stall (WaitForPTS unbounded). Disproved historical seekGen-lastSeekGen bug (fixed at playback.go:607). Root cause: Trim module bypasses Engine.Seek via SmoothScrubbing decoder; seekGen never increments. Text clipping/padding overflow unverified without rendered screenshot.

- **Agent toolchain documentation.** Created docs/AGENT_TOOLS.md with complete registry: vt-auditor (Gemini 2.5 Flash, quota-exhausted), vt-reviewer (deprecated), vt-verifier (Gemini 2.5 Flash, quota-exhausted), vt-verifier-local (Ollama, model tag mismatch), and missing vt-auditor-local (planned). Documents fallback chain and known issues.

- **Next.** Real-media acceptance on an encrypted disc — DVD → Rip → Convert → playback (priority 1 ladder).

## v0.1.1-dev86 — Rip polish

- **Interlaced-lossless prompt.** A rip that would stream-copy an interlaced (video-originated) DVD title in the lossless MKV format now asks first whether to rip as H.264 (deinterlaced) or keep the lossless copy — the per-job choice does not touch the persisted format selection. The IFO scan's `FilmMode==0` signal is surfaced on `DiscTitle`; `interlaceRipClash` is mutation-verified. Commit `3a6c8cd1`.
- **Thumbnail console popups silenced.** Two ffmpeg probes in `internal/thumbnail/generator.go` (the drawtext `-filters` check and the interlace-detection frame probe) spawned without `CREATE_NO_WINDOW`, so every thumbnail run flashed a console window. Both now route through `hideCmd`. Commit `da78d96c`.
- **CSS log lines neutralised.** The rip log's triumphant "decryption complete" gloss became a factual statement of what is happening. Commit `0038560d`.
- **Next.** Real-media acceptance on an encrypted disc — DVD → Rip → Convert → playback (priority 1 ladder). Once the ladder closes, the milestone gate for `v0.1.2-dev*`: author a DVD in VideoTools and rip it back in VideoTools — the ecosystem is usable within itself for the full author → rip cycle.

## v0.1.1-dev85 â€” CSS Decryption Wiring (encrypted DVDs rip-able in software)

- **Encrypted (CSS) commercial DVDs can now be ripped end to end, entirely in software.** No external libdvdcss build, no special drive model, no player-triggered unit-key path, and no FFmpeg-with-libdvdcss dependency. `internal/dvd/css` gains the CSS1/A cipher family (disc key, title/match keys, IV, V2, V3) and the libdvdcss `DVDCSS_METHOD_TITLE` key-recovery route as an original Go implementation.
- **Key recovery.** The disc key is recovered by scanning the unencrypted key-region sectors for the repeating 406-byte period; on it, each VTS's title/match key and the VMG key are derived. This route needs only the key-data region, so it works on any drive and on plain ISOs.
- **Decryption pipeline (`DecryptVideoTS`).** Copies the VIDEO_TS tree into a `vt-css-decrypt-*` scratch dir (under `utils.TempDir()`), decrypts every VOB payload in place, per-VTS: `VTS_XX_1..N.VOB` share the `VTS_XX_0.VOB` title key, `VIDEO_TS.VOB` is its own VMG-domain group, IFO/BUP pass through byte-for-byte (they are never scrambled). The executor swaps the scratch tree in at `Execute` **before** `CollectVOBSets`, so dvdvideo, VOB concat, cell-accurate lists, menu preservation, full-disc, and archivist all read plaintext. Scratch dirs chain into the existing cleanup defer (a closure, so it could be reassigned).
- **Cipher correctness.** All five CSS tables are byte-identical to libdvdcss `csstables.h` (0 diffs). Decrypt is `P = cssTab1[S] ⊕ ks`. `cssTab1` is NOT an involution (64704/65536 round-trips fail — verified), so the encrypted-sector fixture direction uses the inverse permutation `cssTab1Inv`: `S = cssTab1Inv[P ⊕ ks]`. Tests cover single-file/multi-file/no-encryption crack recovery and IFO-verbatim + payload-decrypted decryption.
- **Failure honesty.** CSS paths fast-fail with classifiable errors (`css-crack generate/decrypt/scan/decode/no-CSS-crack/fromDisk/fromVTS/unsupported-state/unsupported-title/cssSectorBytes`) instead of the old plausible-looking log line that masked the real state — a cracked-but-unreadable disc reports why, and a rip never claims success on a disc it could not decrypt.
- **Output verification (`verifyRipOutput`).** A rip is no longer trusted because ffmpeg exited 0. The output must exist, be non-trivial (≥ 1 kB), and ffprobe must detect a video stream; otherwise the rip fails with an actionable error rather than "Rip completed successfully". Closes the empty-container-reported-as-clean corruption class (the dev79 shape).
- **Commits.** `252aa991` (cssTab1Inv + fixture direction), `faafce65` (executor wiring via decrypted scratch tree), `6a33dd96` (output verification guard).
- **Feature doc + tester checklist:** `docs/CSS_DECRYPTION.md`.
- **Next.** Real-media acceptance on an encrypted disc — DVD → Rip → Convert → playback (priority 1 ladder).

## v0.1.1-dev84 â€” Rip Audit + S1 Stabilization

The audit ran to establish Rip's *actual* state before any code was touched, and it materially changed the picture: four behaviours the tracker implied were defective were confirmed already correct, and two real defects the tracker did not show were found and fixed.

### Rip audit: four verified behaviours are now regression constraints

These are **closed facts about the current implementation, not open items.** All four are silent-corruption classes â€” the rip completes, the job reports success, and the output is simply wrong â€” so nothing else in the suite would catch a regression.

- **`dvdvideo -title` must precede `-i`** (`executor.go:341`). FFmpeg binds a demuxer option after `-i` to the *next* input, so it landed on the ffmetadata chapters input, FFmpeg rejected the invocation, and every rip fell back to whole-file VOB concat â€” which rips the **first title's content for all titles**. The dev79 root cause, with a comment so it does not regress.
- **Bulk Select All clears the mode lock and raises the per-card `updating` guard** (`content_list.go:292`). Respecting the lock selects nothing (dev79). Removing the guard **deadlocks the UI thread**: `SetChecked` fires `OnChanged` synchronously and the handler re-acquires `cb.mu`, which is held across the loop.
- **An all-chapter range is a whole-title passthrough** (`executor.go:763`). A `From=1/To=N` range is the whole title; routing it through the range pipeline re-slices cells that already span it.
- **Chapter ranges resolve semantically** through `chapterRange()`, restrict the cell-accurate concat list to the span, and remap embedded chapters so chapter N still marks the same boundary.

Guards live in `internal/app/modules/rip/invariants_test.go`. Every one was mutation-verified: reordering the dvdvideo flags, deleting the lock-clearing, deleting the `updating` guard, and removing the all-chapter short-circuit each turn the suite red with a message naming the defect.

### Rip: the `dvdvideo` â†’ VOB-concat retry no longer fires for output-side failures (`40ec5889`)

- **The defect.** The retry swaps the *input*, so it may only ever fire when the input is what failed. Its gate was a deny-list (`!isMuxerCodecError`), which admitted output-side failures. Verified by running ffmpeg directly and reading the stderr: a blocked output path gives `Error opening output file â€¦ No such file or directory`, a full disk gives `No space left on device`, an unreadable source gives `libdvdread: Could not open â€¦ Unable to open the DVD-Video structure`. The old gate could not tell those apart.
- **The consequence.** An unwritable destination or a full disk launched **a second full rip**, then surfaced the fallback's error â€” reporting `dvdvideo demuxer failed (â€¦)` for what was really a write failure, after paying for the duplicate rip.
- **The fix.** `classifyRipFailure` attributes a run to source-structure / selection / output / execution / cancelled from ffmpeg's own stderr; only source-structure retries. **Output markers are tested before source markers**, because ffmpeg emits `libdvdread` chatter on runs that later fail at the write stage. **Unattributable failures deliberately do not retry** â€” a retry buries the real error under the fallback's, which is exactly the behaviour being fixed.

### Rip: a one-title disc no longer rips when nothing is selected (`982e8066`)

- **The defect.** `addToQueue` branched on the **count** of scanned titles. More than one iterated the selection and refused an empty selection; exactly one â€” and no scan result at all â€” took an unconditional path that always enqueued. Since dev76 made "Movie + extras (choose titles)" start deliberately **empty**, a one-title disc in that mode displayed *"ready to rip â€” no titles selected"* and then ripped anyway: the readiness line and the queue disagreed about what would happen.
- **The fix.** `ripTargetTitles` makes the selection the sole authority for whether a title rips, at every title count. With no scan result there is nothing to select against, so the executor's own main-feature defaults still apply and that case stays unconditional. Mutation-verified.

### Test infrastructure: the native test suite is now actually run (`b441655b`)

`dev-verify.ps1` ran build+vet but **never the tests**, which is why a broken test invocation could stay invisible. `internal/media`, `internal/app/appcfg`, and `internal/app/modules/rip` had been recorded as "pre-existing environment failures" because a package that could not *start* (`0xc0000135` = `STATUS_DLL_NOT_FOUND` â€” the FFmpeg shared DLLs not on `PATH`) looked identical in the output to a real failure. **They were never failing.** They are green, and that baseline-failure claim was wrong â€” produced twice across two commit messages, then carried forward instead of chased down. The script now puts `C:\ffmpeg\bin` on `PATH`, runs the suite, and names `0xc0000135` explicitly so it is never again mistaken for a code defect.

### Scope note

Stabilization was deliberately **S1-only**. The larger architectural shape â€” explicit job/plan types, output verification as a first-class stage, source-identity generations â€” is **not** started. It is a shape to consider only when a defect demonstrates the current structure cannot express an invariant. The remaining proof is real-media acceptance: **DVD â†’ Rip â†’ Convert â†’ playback**, after which Upscale can be scoped.

## v0.1.1-dev84 â€” Player: The Dormant libVLC Backend and Its Inert Settings Toggle Removed

- **The dormant libVLC backend and the Settings toggle that did nothing are gone.** A partial libVLC port had landed on 2026-07-24 behind a `vlc` build tag and could never have been built: five unresolved C symbols (`libvlc_audio_get_track_descriptions`, `libvlc_media_state_end`, `libvlc_track_description_get_id`/`_get_name`, `libvlc_video_get_spu_descriptions`), and no workflow has ever passed that tag â€” `dev.yml` and `release.yml` only build `-tags native_media` â€” so the flag was never exercised in CI either. Roughly 1,400 lines of dormant code read like an in-progress feature.
- **The user-visible defect was the toggle.** Settings offered "Use libVLC backend", persisted `UsePlayerVLC`, and called `media.SetUseVLC` â€” but the only reader lived in `engine_factory_vlc.go`, behind the tag no CI build compiles. In every shipped binary the checkbox did nothing, and its hint claimed libVLC "is more stable for seek/resume", never true for anyone. This is the same defect class as the Convert two-pass toggle (#23), one layer down: a control that looks real and changes nothing.
- **Removed.** `internal/media/vlc_engine.go` (696), `vlc_video.go` (117), `vlc_glue.h` (68), `engine_factory_vlc.go` (19), `vlc_events.go` (54 â€” it `#include`d the deleted header, so it could not have compiled either), and `internal/player/vlc_controller.go` (502 lines of CLI-based VLC plumbing that nothing referenced and that `factory.go` already refused to construct). Also the Settings toggle, its two i18n keys in the three locales that defined them, the `PrefsConfig` field, the settings callbacks and adapter methods, `media.SetUseVLC`/`UseVLC` and its package var, and the load-time call in `main.go`.
- **Kept deliberately.** `media.PlaybackEngine` â€” tagged only `native_media`, genuinely used by `InlineVideoPlayer`, and the seam a future backend slots into (only its stale comments changed); `docs/VLC_PLAYER.md` as the design record; `internal/player.BackendVLC` as a deprecated enum value the factory already errored on.
- **Config safety.** Prefs decode through `appcfg.LoadModuleJSON` with plain `json.Unmarshal` and no `DisallowUnknownFields`, so a stale `UsePlayerVLC` key is ignored rather than rejected â€” no migration needed.
- **Proof it was always inert.** `engine_factory_default.go` drops its now-meaningless `&& !vlc` condition, so `-tags "native_media vlc"` compiles to exactly the same binary as `-tags native_media`.
- **Guards.** `TestNoDeadVLCBackendControl` (no user-facing string in any locale may advertise a backend that does not exist) and `TestNoVLCBackendCode` (no production file may reintroduce the plumbing) â€” both mutation-verified.
- **Honest status.** This removes a lie; it does not add a feature. No playback behaviour changed, because the tag never reached a binary. Reviving libVLC remains reasonable but requires provisioning the SDK **and** adding a CI job that builds the tag, or this exact defect returns.

## v0.1.1-dev83 â€” Rip: LPCM (`pcm_dvd`) Titles Rip Losslessly + Convert Two-Pass Honesty

- **Rip: LPCM (`pcm_dvd`) titles now rip losslessly instead of failing at the muxer.** A DVD title carrying 20-bit LPCM audio could not be ripped in the lossless formats: the rip stream-copied audio (`-c copy`), and the Matroska muxer has no tag for `pcm_dvd`, so the run died writing the container header. Reproduced against ffmpeg 8.1 with a synthetic DVD-shaped VOB (`mpeg2video` + `pcm_dvd`): exit âˆ’22 (AVERROR(EINVAL)), `No wav codec tag found for codec pcm_dvd` â†’ `Could not write header (incorrect codec parameters ?): Invalid argument`. Never a demux problem, so a reader-side fix could not have addressed it. The only recovery was the dvdvideo â†’ VOB-concat fallback, which re-runs the same codecs against a different input â€” so it failed identically while replacing the real diagnostic with a misleading one, re-demuxing a whole title for nothing. `runFFmpegWithProgress` returns a typed `*ffmpegError` carrying ffmpeg's stderr (process error still reachable via `errors.Is`/`errors.As`); `isMuxerCodecError` classifies a codec/container rejection from it, deliberately *not* matching a bare "invalid argument" or "codec" (both appear throughout ordinary demux noise and would misclassify a recoverable failure). On a codec rejection the executor retries once **on the same input** with `-c:a flac` â€” lossless, video still stream-copied â€” and skips the doomed VOB fallback for that class; guarded on `AudioEncoder` being unset so it cannot loop, and applied to menu-VOB exports on success (same codec). The stream-copy path emits `-c:v copy` / `-c:s copy` / `-c:a <encoder>` instead of an all-or-nothing `-c copy` so the substitution is possible; MP4 is unaffected (already re-encodes to AAC). Tests: `audiocodec_test.go` (classification incl. the failures that must *not* classify, error reachability, argument shape incl. the no-bare-`-c copy` regression) + `audiocodec_integration_test.go` (real ffmpeg end to end over a synthetic `pcm_dvd` VOB â€” default attempt still fails *and* classifies, FLAC retry completes, ffprobe confirms `mpeg2video` + `flac`). Mutation-tested: neutering the classifier, dropping the retry, or reverting the argument split each turns the suite red. Gates: `dev-verify.ps1`, untagged stub build, package-main + internal suites green. Commit `f77cdd67`.
- **Convert: stop advertising two-pass encoding (it was never implemented, #23).** The module exposed an enabled "Two-pass encoding" checkbox in all four locales, described it as *"first pass collects data, second pass writes at target bitrate â€” best quality at a set size (VBR-style)"*, and showed *"Estimated size: N MB (2-pass)"*. No two-pass code existed anywhere: the checkbox was never read by the encoder argument builder, so enabling it changed nothing â€” and the disclosure was worse than the dead control, describing a two-pass VBR bitrate workflow as available when the app only ever did single-pass CRF encoding. The control is now permanently disabled with an honest label, the four-locale strings are corrected, the false VBR/two-pass size claim is removed, and the dead `cfg.TwoPass = true` write (UI mutating config nothing consumed) is gone. The audit question "should two-pass be implemented?" was answered from source rather than assumed: the design is video-bitrate two-pass, a materially different feature, deliberately left unimplemented â€” this cycle makes the UI tell the truth. `TestTwoPassDisclosedAsUnimplemented` + `TestNoTwoPassFfmpegArgs` guard it, both negative-tested. Commit `69615c3c`.

## v0.1.1-dev82 â€” Convert Module Audit Fixes (Output Allocation, Codec Vocabulary, Interlace Identity, Config Migration, Layout Reset)

Seven verified defects from the Convert-module audit (issues #10â€“#24) fixed: persisted-config migration (#19), queued loudness normalization (#15 F-7), Reset-through-toggle-transitions (#10), stub player-pane signature (#16), the canonical codec vocabulary with ProRes/Theora (#13), the shared interlace-analysis operation with source-identity claims (#21 + #14), and the deterministic queue output allocator (#12). Each carries regression tests â€” the format-codec round-trip over every preset, the interlace claim lifecycle incl. the Aâ†’Bâ†’A rejection and real end-to-end dispatch via the PATH ffmpeg, and the output-allocation matrix incl. the unwritten-batch-path case. Both build variants (native_media + untagged stub) build + vet green; full package-main + internal suites green. Full per-fix detail: `docs/CHANGELOG.md` (dev82 section). Landed commits: `a8fe9dbc`, `9dbeff23`, `9222ca26`, `ae0b4333`, `63a95962`, `154eefd4`, `53548707`.

## v0.1.1-dev81 â€” Rip a Title by Chapter Range

- **Rip: "Rip chapters only" + From/To chapter selects** on a scanned title. The rip view gains a per-rip toggle and two chapter selects once the scan reports chapter counts. The selects' upper bound is the greatest chapter count across ALL scanned titles (so the widget never invalidates when the picked title changes) and is clamped per title at execute; titles without PGC program data show "N/A" and the controls stay disabled. The toggle is inert for full-disc rips (the whole-disc job is one pass), region conversion, and the archivist format, and the selection is a per-rip transient â€” it is never persisted to the rip config. New i18n keys `RipChapterOnly`/`RipChapterFrom`/`RipChapterTo` across en/fr/iu/iu_latin.
- **Executor: output-side trim** â€” the dvdvideo path appends `-ss`/`-to` after ALL inputs (absolute chapter timestamps from the title's chapter list), so every input is declared before the trim options and the output timeline normalises to 0. Works for both the dvdvideo path and the concat fallback path.
- **Executor: cell-accurate slicing for VOB-concat fallback** â€” `ifo.TitleInfo` now retains the PGC program map (`ProgramEntryCells []int`, filled in the same PGC pass that reads chapters). The concat fallback slices each VOB to the range's cell span `[entry(cs)âˆ’1, entry(ce+1)âˆ’1)` via `chapterCellSpan` (cell serving program `p` = `ProgramEntryCells[p-1]`; whole-title end = len(cells); `cs > n` or `ce < cs` rejected; empty program spans handled by the union of adjacent program entry cells). Adjacent cells coalesce per VOB as before. The whole-cover short-circuit is skipped while a range is active (a ranged rip must always slice), and a cells-unresolvable range degrades to whole-file concat + the `-ss`/`-to` trim. The stale-PTS failover cap gets `range duration + 5 s` when the cell list is engaged.
- **Embedded chapters remap to the range base** (`firstChapter â†’ 0`), so the MKV's chapter list describes the trimmed file; progress tracks the range span, not the whole title.
- Verified by unit tests: `chapterRange` remap (base = chapter csâˆ’1, span cs..ce, end = chapter `ce` or the title duration), `chapterCellSpan` (shared entry cells, `cs > n` rejection, `ce < cs` rejection, whole-title equivalence), and `cellConcatList` ranged slicing with whole-cover detection disabled while ranged.
- Build + vet green (`dev-verify.ps1`); version triad bumped to v0.1.1-dev81 / Build 71. Feature doc + tester checklist: `docs/RIP_CHAPTER_RANGE.md`.

## v0.1.1-dev80 â€” Thumbnail "Both" Mode Produces the Separate Thumbnails + Queue Timestamp Bug + Full Job Log Capture

- **Thumbnail: output mode "both" now produces AND surfaces the separate thumbnails.** Selecting "both" previously ran the contact-sheet path only â€” the generator's run list treated "both" as the sheet and every thumbnail was routed through the sheet callback â€” so the live preview showed the sheet but no separate individual images were generated/reflected (user report: "it only gives me the information of the contact sheet, it doesn't actually give me the separate thumbnails"). The generator now exposes a distinct `OnContactSheetGenerated` callback (`generateContactSheet` falls back to `OnThumbGenerated` when the sheet callback is unset); "both" runs true individual generation AT the contact-sheet tile width with `count = columnsÃ—rows` (lock-step with the grid â€” a 4Ã—8 grid â‡’ 32 screenshots), and the live preview shows the sheet as a grid cell plus the accumulating individual thumbnails (the individual/both paths accumulate via `OnThumbGenerated`). Job descriptions in `createThumbnailJobForPath` are per-mode accurate ("Contact sheet + N thumbnails (Wpx, matches grid)"/"Contact sheet: CÃ—R grid (N thumbnails)"/"N individual thumbnails (Wpx width)").
- **Thumbnail: "both" settings render BOTH boxes; the individual box is a locked count readout.** The individual thumbnails box in "both" mode carries a locked "Count: N (matches contact sheet)" label (`ThumbnailCountMatchesSheetFmt`, new i18n key across en/fr/iu/iu_latin) that tracks the grid columns/rows sliders live; the separate individual-width select was removed (tiles use the sheet width), so the box is a deliberate display-only readout, not a dead control.
- **Queue: StartedAt/CompletedAt equals bug fixed.** The pop path did `nextJob.StartedAt = &now` then `now = time.Now()` â€” reassigning the SAME variable â€” so every popped job carried identical start/end timestamps (zero apparent elapsed). Separate `startedAt`/`completedAt` variables now capture the real instants.
- **Queue history now surfaces Progress.** `addToHistory` copied CompletedAt/Error/FFmpegCmd but never `Progress`, so finished jobs read `Progress: 0` in the history panel; `Progress: job.Progress` is now stored.
- **Thumbnail: the job log captures the whole run.** `generateIndividual` re-opened the log with `os.Create` (truncate) once per thumbnail, so the on-disk log ended up containing only the last screenshot's run. The log is now opened once with `os.OpenFile(..., os.O_APPEND|os.O_CREATE|os.O_WRONLY)` and a deferred Close, and each thumbnail writes under its own `===== thumbnail N (t=..) -> path =====` header; `generateContactSheet` still opens first and truncates, so a "both" job logs the sheet plus every individual thumbnail.
- Build + vet green (`dev-verify.ps1`); version triad bumped to v0.1.1-dev80 / Build 71.

## v0.1.1-dev79 â€” dvdvideo -title Arg Ordering Root-Cause Fix + Bulk-Selection State Sync + Release CI Fix

- **Rip: dvdvideo `-title` arg ordering â€” the root cause of every dvdvideo rip failure.** The shipped `-f dvdvideo` path failed on every disc; the 0/77 audit result was not the library, it was this one ordering bug. `-title N` was emitted on the far side of `-i VIDEO_TS` (`-f dvdvideo -i VIDEO_TS -title 1`). FFmpeg binds any option appearing between two `-i` flags (or before a later input) to the NEXT input; the rip also inserts a chapters ffmetadata input after the dvdvideo input, so `-title 1` bound to that second input â€” which has no `title` option â†’ "Option title not found" â†’ ffmpeg exit `0xabafb008`. Every intended-dvdvideo rip silently fell back to VOB concatenation, and for seamless-branching / multi-VOB titles whole-file VOB concat writes PTS discontinuities at VOB boundaries â€” the 26h-freeze / post-cutoff slideshow corruption reports. `-title [N]` is now placed **before** `-i` so it binds to the dvdvideo input itself (`-f dvdvideo -title 1 -i VIDEO_TS`). Verified live on a real grey-market DVD (a seamless-branching multi-VOB disc): exit 0, 112603 video packets, exact 40 ms cadence, zero backward PTS, audio + chapters + metadata preserved end to end.
- **Rip: ContentBrowser Select All / Deselect All now mirror the selection state.** Bulk selection was applied to the view model (`UpdateRipSummary` â€” seen in the summary radio/preview) but never propagated to `viewState.selectedTitles`, so titles blocked/locked by a rip-mode lock kept their stale selection state after a bulk Select All. New `SetOnBulkSelection` (ContentBrowser callback â†’ view wiring) mirrors bulk selection into `selectedTitles` so the checked cards, the readiness summary, and the queued rip all agree.
- **CI: release.yml setup-msys2 missing meson + ninja.** The dev78 harfbuzz build steps were added to all four Windows pipelines but the release.yml install list was missed â€” only dev.yml and windows-msix.yml declared `mingw-w64-ucrt-x86_64-meson/-ninja`, so the v0.1.1-dev78 release run failed with "meson: command not found" while the MSIX run (which installs them via pacman) passed. Both packages are now in the release install list.
- Build + vet green; version triad bumped to v0.1.1-dev79 / Build 70.

## v0.1.1-dev78 â€” CI Drawtext Complete (Harfbuzz) + dev77 Content Released

- **CI: drawtext filter fully restored in all Windows FFmpeg sidecar builds.** FFmpeg 8.x requires both `libfreetype` AND `libharfbuzz` for the drawtext filter (`drawtext_filter_deps="libfreetype libharfbuzz"`; `vf_drawtext.c:1372` calls `hb_ft_font_create_referenced`). All four Windows pipelines (dev/release/msix/forgejo) now build harfbuzz 14.4.0 from source via meson (`-Dfreetype=enabled`, all other backends off) into the ffmpeg prefix and rewrite `harfbuzz.pc` to put `-lfreetype -lstdc++ -lsupc++ -lm` in `Libs` directly (FFmpeg Windows configure calls pkg-config without `--static`; same precedent as x265.pc/dvdnav.pc). The CGo link expands `-lharfbuzz -lstdc++ -lfreetype` from pkg-config `--libs --static` (CI strips `-lsupc++`); no new Go flags needed. Cache keys bumped: dev/release `v12`, msix `v6`, forgejo `v9` + `.built-v10` marker.
- **CI: post-build drawtext gate corrected.** The gate now uses `ffmpeg -filters | grep drawtext` â€” FFmpeg 8.1 moved the filter list away from `-h filters` (which now prints only generic options), so the dev77 gate failed even on a correct build. Only the three GitHub workflows gate; the Forgejo build does not.
- **v0.1.1-dev77 tag abandoned.** The dev77 tag was pushed at commit `ee806367` before the harfbuzz fix landed; its release run failed (`FATAL: drawtext filter not enabled after FFmpeg build`). Repo rules forbid deleting or retargeting dev tags, so the dev77 content + this CI fix shipped together as dev78 from commit `2b81a0c7` (dev CI green, verified both gate and `CONFIG_DRAWTEXT_FILTER=yes`).
- Build + vet green; version triad bumped to v0.1.1-dev78 / Build 69.

## v0.1.1-dev77 â€” Thumbnail Native Resolution + Movie+Extras Auto-Select + Drawtext-Restored CI FFmpeg

- **Thumbnail: "Native (WxH)" size option.** Contact-sheet and individual thumbnail size selects gain a "Native (WxH)" entry resolving to the source video's real width, so screenshots can be judged at source resolution; blank/reset fallbacks unchanged. New i18n key across en/fr/iu/iu_latin.
- **Thumbnail: contact-sheet live total counter.** The "Total thumbnails" count came from a captured opts struct, so moving the columns/rows sliders left a stale count on screen; it now computed from the live slider values.
- **Thumbnail: drawtext-absent degradation.** With a static FFmpeg lacking libfreetype/libfontconfig, the old generator hard-failed ("No such filter: drawtext") on timestamp overlays. The generator now probes once per run (`ffmpeg -filters`), skips the overlay + metadata header when the filter is missing, and emits plain output plus a debug warning via a result `Warnings` list. CI now ships libfreetype (below), so the overlays are back in released binaries.
- **CI: freetype + harfbuzz rebuilt into all Windows FFmpeg sidecars (v0.1.1-dev78).** dev/release/msix/forgejo workflows build freetype 2.13.3 + harfbuzz 14.4.0 from source (static-only) and configure `--enable-libfreetype --enable-libharfbuzz`; each build gates on `ffmpeg -filters | grep drawtext`; cache keys bumped (dev/release v12, msix v6, forgejo v9 + marker v10). Pure-Go link unchanged.
- **Rip: "Movie + extras (choose titles)" auto-select refined.** On a scene-set disc the choose-titles mode now pre-selects the main feature + genuine extras, skipping the scene segments (the movie already contains them) and skipping duplicate whole-movie copies; no scene set â†’ still starts empty (what you tick is what rips). Representative whole copy forced only when present in the caller's titles. Unit tests updated/added (movie-only, genuine extras, no-scene-set empty, empty titles).
- **Queue: completed jobs read "Status: Completed"** (the redundant "| Duration: Ns" suffix dropped).
- **Rip: executor log header records the app version.**
- **Hygiene:** govulncheck back to 0 reachable (x/image v0.43.0, x/net v0.55.0, x/text v0.38.0, x/sys v0.45.0, go 1.26.6); staticcheck's three real findings fixed (UDF VDS labeled `break` at the terminating descriptor, dead `vtsATRTEntries`, dead `isFocused`); gitleaks clean.
- Build + vet green (`dev-verify.ps1`); version triad bumped to v0.1.1-dev77 / Build 68. *Note: never published â€” the drawtext CI fix was incomplete; content released as v0.1.1-dev78.*

## v0.1.1-dev76 â€” ISO 9660 Resolve Fix + Menu-Export Dedup + Choose-Titles Starts Empty

- **Rip: `resolveISOWithUDF` now finds flat-extracted disc content (the ISO rip breaker).** The native readers extract a target directory's descendants FLAT into the destination root â€” a DVD lands as `tempDir\VIDEO_TS.IFO` + `tempDir\VTS_01_0.IFO`, not inside a nested `tempDir\VIDEO_TS` folder â€” but the resolver only accepted the nested folder, so every ISO 9660-only grey-market image failed the rip path with "`VIDEO_TS not found in ISO 9660 image`" right after its scan had succeeded (user report on `ISO9660-sample-A.iso`, echoed by `ISO9660-sample-B.iso`; both burner-written with a junk UDF bridge and pure ISO 9660 underneath). `extractedVideoTSPath` now prefers the nested dir when present and otherwise returns the flat root when the DVD marker `targetDir.IFO` (or a Blu-ray's `index.bdmv`) exists; both the UDF-success and ISO 9660-fallback branches resolve through it. Verified end-to-end with the `VT_REAL_ISO`-gated test on both reported images: UDF extraction fails on the junk VDS (tags 0:10 1:1 4:1 5:1 6:1 7:1 8:1, no LVD), ISO 9660 fallback extracts 11/10 files, and the resolve returns the flat `VIDEO_TS`. New unit tests cover nested-preferred / flat-DVD-marker / flat-BD-marker / no-markers.
- **Rip: repeated title rips no longer duplicate the animated menus.** Every title rip re-collected the same per-VTS menu VOBs and appended a fresh `_Menu_<label>.mkv` to that output base, so ripping several titles of one disc produced identical copies per title (`1_2_Extra_Title_04/05/10/11` etc.). The executor now dedups against the output directory: a menu is skipped when an existing `*_Menu_<label><ext>` file there matches its source VOB size ("already exported (same menu content)"), which is layout-exact for identical VOBs and impervious to per-title base prefixes. The animated-menu capability remains intact for anyone doing a full reauthor; repeated rips just stop stacking duplicates.
- **Rip: "Movie + extras (choose titles)" starts empty â€” exactly what you tick is what rips.** The dev75 reshape re-derived choose-titles as *every* title, so a user switching modes to pick two extras found the whole disc ticked and the queue ripped the main feature plus every still-checked extra (user report: "It has made the title, and then `_03.mkv`, `_04.mkv`, `_05.mkv` for the extras. It's downloading more than we want"). `CanonicalSelection` now returns an empty selection for the `""` choose-titles mode (Select All still opts into everything; `"full"` keeps its single-Job path; `"main"`/`"segments"` defaults unchanged). The readiness line falls back to the existing no-selection state and queueing with nothing picked errors "no titles selected".
- Build + vet green (`dev-verify.ps1`); version triad bumped to v0.1.1-dev76 / Build 67.

## v0.1.1-dev75 â€” Dynamic Rip-Mode Selection + Select/Deselect All Deadlock Fix

- **Rip: mode transitions now reshape the selection â€” no manual intervention.** Tester report: "modify the settings to allow all movie and extras" messed up the content browser and hard-crashed. The browser kept the previous mode's restriction after a switch: leaving "Main feature only" for "Movie + extras (choose titles)" left only the main title ticked, and the user had to press Select All to recover (which then froze). Mode transitions now re-derive the selection from the new mode â€” `CanonicalSelection`: "Movie + extras" â†’ every title, "Main feature only" â†’ the single longest (anchored), "Scene segments only" â†’ just the detected scene segments. A track of the last-shaped mode (`viewState.lastShapeMode`) ensures selection is reshaped only at a *transition*, so per-title toggles made inside a mode persist (format/output/region changes no longer clobber them). Reshape and lock are decoupled: `ApplyModeLock` is a purely visual layer (locked/anchored greys), and `ReshapeSelection` replaces the whole selection map without firing per-title OnChanged callbacks. Pure logic moved to `internal/app/modules/rip/ripmode.go` (`CanonicalSelection`/`CanonicalLock`) with unit tests covering main/segments/choose/full modes and the no-scene-set case.
- **Rip: Select All / Deselect All deadlock fixed (the hard crash).** Root cause: `setAllSelected` held `cb.mu` while calling `tc.checked.SetChecked(v)`; Fyne fires the card's OnChanged synchronously, which re-acquired `cb.mu` â†’ the UI thread deadlocked (app frozen, required a kill). The bulk path now raises the per-card `updating` guard so OnChanged early-returns before touching the mutex; selection is written directly to the map and the list refreshed once. `onLockedModeExit` uses a transient `shapeBlocked` flag so a bulk Deselect All creates locks + deselects every title without the exit-reshape re-selecting everything (Deselect All stays a real "clear all").
- **Rip: fresh discs start clean.** `loadDisc` resets `extractMode` to "main" and clears `lastShapeMode`, so a previously "segments"-mode session can't leak the old disc's scene-segment restrictions into the newly loaded disc â€” the new disc re-shapes to Main-feature-only exactly once.
- Build + vet green (`dev-verify.ps1`); version triad bumped to v0.1.1-dev75 / Build 66.

## v0.1.1-dev74 â€” Scene-Segment Rip Modes + Cell-Accurate VOB Concat

- **Rip: scene-set detection + "Scene segments only" mode.** `DetectSceneSets` (sceneset.go) bins a scan's titles into whole-movie copies and scene segments: the longest run is the movie, near-equal durations (within tolerance) join it as duplicates, and the remaining shorter titles whose durations don't fit any run are the segments. Tolerant of the a 7-title scene-segmented disc quirk where T05's 1977.60 s â‰ˆ exactly 1/3 of a 5932.48 s run â€” the segment-sum heuristic keeps short titles from being absorbed into a run. When a scan detects a scene set, the rip mode radio gains "Scene segments only (skip full movie)": the ContentBrowser mode lock greys/locks the whole-movie titles, pre-selects every scene segment (individually toggleable), and clicking a greyed title (SetOnLockedSelect) or the locked-mode exit (SetOnLockedModeExit) returns to "Movie + extras (choose titles)". "Main feature only" locks every title but the main one with an anchored selection. Per-title ready summary counts scene segments ("Ready to rip 5 scene segments"). New i18n keys `RipModeScenesOnly`/`RipReadyScenesOne`/`RipReadyScenesManyFmt` across en/fr/iu/iu_latin. Unit tests cover whole+segments, no-segments, all-equal, and the duplicate-duration quirk.
- **Rip: cell-accurate VOB-concat fallback (root-cause content fix).** The underlying wrong-content bug: on Windows builds the ffmpeg dvdvideo demuxer can't open DVD sources (libdvdnav rejects them â€” see the scene-segment rip logs), every rip falls back to whole-file VOB concatenation, and a shared-VTS extra title therefore ripped as the movie's opening. `ifo.TitleInfo` now exposes per-title PGC cells (`Cells []TitleCell`: VOBID/CellID/FirstSector/LastSector) read from the PGC cell position table (offset bytes 234â€“235, 4 bytes per entry matching the libdvdread `cell_position_t` layout) combined with the sector extents of the PGC cell playback table (FirstSector bytes 8â€“11, LastSector bytes 20â€“23). `cellConcatList` (executor aux, new file) slices each VOB to the selected title's cell byte ranges (sector Ã— 2048, adjacent cells coalesced per VOB) into temp VOB slices and builds the concat list, wired into both the primary VOB-concat path and the dvdvideo-failure retry inside `Execute`. Slicing is skipped when the cells already span the whole VOB set (whole-file is then byte-identical), on `HasAngles` (interleaved angle data would be mis-sliced), or when any cell VOB can't be resolved in the set â€” each falling back to whole-file concat with a warning log. Unit tests (`cellconcat_test.go`) cover the no-cells, has-angles, whole-set-coverage, multi-VOB partial-slice (byte-exact slice sizes), and missing-VOB branches.
- **Rip: loadDisc no longer inherits the previous movie's Title.** Loading a new source path now resets `vs.discTitle`/`vs.outputTouched` and clears the Title field before re-deriving the output path, so the default filename comes from the new source (previously it kept e.g. a title-derived output filename after loading a different disc).
- Build + vet green (`dev-verify.ps1`); version triad bumped to v0.1.1-dev74 / Build 65.

## v0.1.1-dev73 â€” Per-Title PGC Duration Scan Consistency

- **Rip: the scan now reports each title's real duration and chapter count on multi-PGC discs.** Scene-segmented "extras" discs put several titles in one VTS, each served by its own PGC. The scan cached ONE `ifo.ReadTitleInfo` per VTS and the IFO reader only ever read the first title-domain PGC, so every title in that VTS showed the first PGC's play time â€” the 7-title scene-segmented disc listed all 7 titles as "1h 38m" even though T03â€“T07 were 14/10/33/22/19-minute scene segments sharing VTS_01 with the two full runs. New `internal/dvd/ifo.ReadTitleInfoForTTN(ifoPath, ttn)` selects the PGC the disc serves to a specific VTS title: it walks the PGCI_SRP table and takes the first entry whose masked TitleNr (bit7 is a flag, low 7 bits = last TTN sharing the PGC) is `â‰¥ ttn`, falling back to entry index `ttn-1` when authors leave TitleNr zero (some tools just order entries by TTN). `ReadTitleInfo` keeps the legacy first-title-domain behaviour (ttn=0).
- **Scan per (VTS, TTN)** â€” `ScanDisc` now caches `ifo.TitleInfo` per (VTS, TTN) rather than per VTS, so the title cards in the Content Browser, the scan-as-you-go snippets, the "longest title = main feature" pick, and the DiscSummary all use true per-title durations and chapter counts.
- **Rip per selected title** â€” the executor previously embedded chapters and sourced the VOB-concat `-t` cap from the first title-domain PGC too, so ripping a segment title would embed the whole movie's 15 chapters and cap against the wrong duration. `Execute` now resolves the picked title's per-VTS TTN from the VMG TT_SRPT (`resolveVTS_TTN` in executor.go â€” returns 0/legacy when the VMG can't be read, the title is out of range, or it lives in another VTS) and reads that title's PGC: a segment rip embeds its own 3 chapters and caps against its own PGC duration.
- Verified on `the 7-title scene-segmented disc's VIDEO_TS`: T01/T02 5926.48/5932.48 s (15 chapters), T03â€“T07 866.04/615.40/1977.60/1331.08/1141.16 s (3 chapters each) â€” durations/chapter counts matches the cell-bin sum analysis (Î£ 5926.48 s) and the per-title TT_SRPT chapter counts.
- Tests: `internal/dvd/ifo/ttn_read_test.go` builds synthetic multi-PGC VTS IFOs (via `NewVTSMAT`/`SerializeVTSMAT`/`WritePGCITIs` with SRP TitleNr bytes forced) and covers (a) the shared-PGC rule (TitleNr 0x82/0x85 over 5 TTNs â†’ PGC A for 1â€“2, PGC B for 3â€“5), (b) the index fallback when all TitleNr masks are 0, (c) ttn=0 â‰¡ `ReadTitleInfo` legacy, (d) out-of-range TTN â†’ last PGC. Build + vet green (`dev-verify.ps1`); version triad bumped to v0.1.1-dev73 / Build 64.

## v0.1.1-dev72 â€” ISO 9660 Fallback for Non-UDF Disc Images

- **Rip: ISO scanning and extraction can now read a disc image whose UDF volume is unusable.** Some grey-market DVDs carry a burner-written UDF bridge whose anchor at sector 256 points at a VDS that scans as tags {0:10 1:1 4:1 5:1 6:1 7:1 8:1} (PVD/IUVD/PD/LVD/USD/TD) but whose LVD fails to parse â€” the old UDF reader reported `LVD not found in VDS` and the disc could not be scanned or ripped (reported on the junk-UDF ISO). A new native ISO 9660 reader (`internal/dvd/iso9660/reader.go`) parses the Primary Volume Descriptor (magic `CD001`, type 1 at sector 16), reads both-endian extent/size pairs with little-endian preference, walks directory trees using the spec-correct single-byte `.`/`..` identifiers (0x00/0x01) and `;`-version-stripped filenames, and folds multi-extent continuation records (flag 0x80) into the owning file. `ReadFileData` mirrors the UDF reader surface for the scan path; `ExtractDirectory` mirrors it for the rip path.
- **Fallback wiring** â€” `scanISOViaUDF` (scan.go) and `resolveISOWithUDF` (iso_udf.go) both try the UDF reader first and fall back to the ISO 9660 reader on failure, logging which backend served the files under CatDVD; when both fail the returned error names both backends ("UDF: â€¦; ISO 9660: â€¦"), so a damaged image is surfaced honestly instead of being falsely labelled corrupt. UDF extraction failure also gets a fresh temp dir (the partial UDF output is removed) before the ISO 9660 extraction runs.
- Verified on the reported ISO: the ISO 9660 reader returns `VIDEO_TS.IFO` (12288 B) and `VTS_01_0.IFO` (94208 B) â€” exact byte counts matching the ISO 9660 directory listing â€” and the full `scanISOViaUDF` path returns both titles (8 chapters, 1 EN audio, 8241.8 s each).
- Tests: unit suite in `internal/dvd/iso9660/reader_test.go` (ReadFileData exact bytes + case-insensitive paths + `;1` stripping + multi-sector files; missing-file/missing-dir errors; ExtractDirectory round-trip; non-ISO-9660 detection), plus `VT_REAL_ISO`-gated integration tests for the reader and the scan fallback against real authored media.
- Build + vet green (`dev-verify.ps1`); version triad bumped to v0.1.1-dev72 / Build 63.

## v0.1.1-dev71 â€” VOB-Concat Stale-PTS Cap Hardened + Title-Driven Output Names

- **Rip: the VOB-concat stale-PTS duration cap now always engages.** A second grey-market disc exposed a probe-failure gap in dev69: the stale PTS offset bakes into the *entire* second VOB (it probes to +93822 s, and the bundled static ffprobe errors on it), so the old logic hit the "could not probe all VOB durations" branch and left the rip uncapped â€” 26hrs again. The cap source is now hierarchical: per-VOB media durations when sane (sum under 3Ã— the IFO PGC play time â€” exactly the dev69 behaviour on clean-PTS discs), otherwise the IFO's authored PGC duration (`titleInfo.Duration`), which is immune to stale file PTS offsets. A safe `-t ceil(cap + 60 s)` is computed regardless of probe health. Verified on the reported disc (VTS_02, same VOB layout as the dev69 title): raw `VTS_02_2.VOB` probes to 93822.98 s but the concat+genpts view is clean (last video packet 1822.784 s), and the IFO-derived `-t 1884` reproduces 1822.816 s â€” the real ~30m23s â€” instead of 26hrs.
- **Rip: the Title field now drives the output filename by default.** Setting a Title ("test") names the output `test.mkv` in the usual `DVD_Rips` folder (metadata embedding unchanged); a blank Title or a Source/format change falls back to the source folder's name, and full-disc/region-conversion runs get the title-based directory name too (`FullDiscOutputTitlePath`). Hand-editing the output path stops auto-naming for that run. New `DefaultOutputTitlePath`/`FullDiscOutputTitlePath` (with `defaultOutputDir`/`sourceBaseName`/`formatExt` factored out of `DefaultOutputPath`/`FullDiscOutputPath`) plus the `outputTouched` viewState flag and a title-aware `applyOutputPath()` at every auto-recompute site in `view.go`.
- Build + vet green (`dev-verify.ps1`); version triad bumped to v0.1.1-dev71 / Build 62.

## v0.1.1-dev70 â€” Rip UI Polish: Mode Labels + Title Card Info Line + Language Select-All

- **Rip mode radio reworded** â€” "Full movie (main feature)" is now "Main feature only" and "Selected scenes" is "Movie + extras (choose titles)" (the scenes option is really title-by-title selection where every title starts checked â€” movie + extras). The radio stacks vertically so the longer labels fit the narrow left column; "Main feature only" stays the default/first option. i18n values updated in en/fr/iu/iu_latin.
- **Content Browser title-card info line cleaned up** â€” the line repeated the header (T## + duration) and word-wrapped "1 audio" onto its own line. Now a single compact localized "N chapters Â· N audio Â· N subs" line with singular forms (`RipTitleAudioOne/Many`, `RipTitleSubsOne/Many`, new keys across all 5 locales); `RipTitleCardFmt` is now just "%d chapters" â€” the card header owns the T##/duration display.
- **Subtitle-language picker Select All / Deselect All** â€” two compact buttons appear when a title advertises multiple subtitle languages; they set the whole list in one click, applying the selection slice outright with per-language callbacks suppressed/restored (no duplicate selection entries), then persist config. Reuses `RipSelectAll`/`RipDeselectAll` shared with the Content Browser.
- Build + vet green (`dev-verify.ps1`); version triad bumped to v0.1.1-dev70 / Build 61.

## v0.1.1-dev69 â€” VOB-Concat Stale-PTS Duration Cap

- **Rip: concat-fallback output duration cap** â€” a fallback rip on a grey-market disc completed but the produced MKV "was classed as 26hrs" while the real title was 30m23s (chapters ended at the title duration). FFmpeg's concat + `-c copy` muxes a trailing phantom 32-byte video packet authored at ~+26h into the MKV, and that last-packet timestamp (95443.8 s) inflates the file's reported duration even though the real content is intact. The fallback now probes every VOB's duration (`probeDuration`) and caps the output with `-t ceil(Î£ durations + 60 s)` (new `RipArgs.MaxDuration`, wired into `BuildRipArgs` before `-max_interleave_delta`), stopping the muxer before those phantom-tail packets while preserving all real content; on healthy discs the cap sits above real content and changes nothing. Verified end-to-end on the reported disc (uncapped run reproduces 95443.815 s; capped run yields 1822.784 s, the real 30m23s title, exit 0) and on the lying-IFO disc (the dev68 clamped 9-subtitle run still completes with the cap applied, all labels intact, nothing truncated). Build + vet green (`dev-verify.ps1`).

## v0.1.1-dev68 â€” VOB-Concat Subtitle-Map Clamp for Lying-IFO Discs

- **Rip: concat-fallback subtitle-map clamp** â€” discs whose IFOs advertise more subtitle languages than the VOBs carry physical streams for (grey-market media) hard-failed the rip: dev64's per-stream `-map 0:s:<idx>` flags referenced streams that don't exist and ffmpeg aborted (`Stream map '0:s:N' matches no streams`). On the dvdvideoâ†’VOB-concat fallback, the executor now probes the concat input's real subtitle stream count (`probeSubtitleCount`, ffprobe `-select_streams s` on the concat list, âˆ’1 on probe failure to skip clamping) and clamps `SubtitleLangs`/`SubtitleSel` to the leading languages that exist, logging "VOB concatenation exposes N subtitle stream(s) (IFO advertised M) â€” dropping Mâˆ’N trailing subtitle mapping(s)". Verified end-to-end against a real 10-IFO-language/9-stream title: the ORIGINAL command fails at map validation exactly as reported; the clamped command produces an MKV with video + audio + 9 labelled subtitle streams (enâ€¦pt, trailing nl dropped) in one pass, exit 0.

## v0.1.1-dev67 â€” Settings Keyboard-Nav Fix + Rip De-clutter + Default Rip Mode

- **Settings keyboard navigation fixed** â€” the dev66 shortcuts were dead: the Fyne GLFW driver only synthesizes `desktop.CustomShortcut` when the modifier is non-zero (`internal/driver/glfw/window.go`), so unmodified PageUp/PageDown/Home/End never fired. Rewritten: `settings.Options` gained `KeyHandler *func(fyne.KeyName) bool` and `KeyCatcher *fyne.Focusable`; a focused 1Ã—1 transparent `settingsKeyNav` widget (`fyne.Focusable` + `desktop.Keyable`) serves `TypedKey` for every press including OS repeats, with canvas `desktop.Canvas` `SetOnKeyDown`/`SetOnKeyUp` as the fallback when nothing else has focus. Previous handlers are preserved; keys are unregistered on leaving Settings. PageUp/PageDown page the active tab by one viewport, Home/End jump to top/bottom.
- **Rip de-clutter** â€” tester-requested: the Disc Menu preview panel and the in-app Rip Log strip are removed outright so the Content Browser owns the full left-column height; rip activity/errors stay in the on-disk executor log. `menu_preview.go` deleted; viewState/Options log fields, `appendRipLog`/`resetRipLog`, and the `RipLog*`/`RipMenuPreview`/`RipPreserveMenus`/`RipLoadingMenu`/`RipNoMenuPlaceholder` i18n keys retired across en/fr/iu/iu_latin.
- **Rip: default rip mode** â€” the mode radio lists and defaults to "Full movie (main feature)" (was "Selected scenes"), so a fresh rip defaults to the full-movie path.

## v0.1.1-dev64 â€” Rip Modes + Per-Language Subtitle Selection + Convert SMPTE-Idle Restore

- **Rip mode selection (scenes / full movie)** â€” a horizontal radio under the subtitle checkbox switches between per-title "Selected scenes" rips and a single "Full movie (main feature)" rip of the longest title (one job through the existing per-title executor path with `vtsNumber`/`titleNumber` + `extractMode "main"`; falls back to executor defaults with no scan result). Hidden while region conversion forces full-disc extraction; the CTA line shows "Ready to rip main feature: Title N Â· duration". i18n keys added across en/fr/iu/iu_latin. Build + vet green (`dev-verify.ps1`).
- **Per-language subtitle selection** â€” the single "Include subtitles" checkbox becomes one checkbox per subtitle language on the main title. `RipArgs.SubtitleSel []int` maps each selected language to its `-map 0:s:<srcIdx>` stream (blanket `0:s?` fallback for legacy callers so nothing else changes); the executor muxes only streams whose language is selected (empty selection = all), keeping VOBSUB pairs together via their source index; `uniqueSubtitleLangs` drives the checkbox list; legacy configs (no recorded selection) default to all languages.
- **Rip scan-as-you-go snippets** â€” `ScanDisc`/`scanISOViaUDF`/`runISOScan` gained an `onNote` callback that emits disc facts as they are known (region Â· type Â· title count, per-title `T0N duration Â· ch Â· audio Â· subs` lines, video standard), surfaced incrementally in the DiscSummary scanning state via a new `SetSnippet` (first note materialises the snippet row).
- **Convert SMPTE-idle restore** â€” menu-driven re-entry to Convert left the previous module's last frame instead of idle SMPTE bars. `InlineVideoPlayer` gained `CurrentPath()` and `Close()` now clears it; `showConvertView` resets the shared primary player (via `Close`) on menu-driven re-entry unless it already shows the convert source.

## v0.1.1-dev66 â€” Settings Keyboard Navigation + Updater Messaging

- **Settings keyboard navigation** â€” PageUp / PageDown scroll the active Settings tab by one viewport; Home jumps to the top, End to the bottom. `settings.Options` gained `ActiveScroll *func() *ui.FastVScroll`: `BuildView` builds one `ui.NewFastVScroll` per tab, tracks the visible one via `tabs.OnSelected`, and fills the resolver. `showSettingsView` registers four `desktop.CustomShortcut` canvas shortcuts (scoped to the settings module and removed via the wrapped `OnBack`), each `ScrollBy`-ing the resolved scroll (`Size().Height` viewport, 480 fallback; Â±1e6 clamps Home/End against the `FastVScroll` offset bounds). Build + vet green (`dev-verify.ps1`).
- **Update check error-message hardening** â€” `describeUpdateError` classifies `*url.Error`/`net.Error` connection and timeout failures, GitHub API 403 rate-limits, 404 not-yet-published releases, empty-tag repos, and missing platform assets into actionable dialog text; the raw error is logged under `CatSystem`. Stale "nightly run" comment removed from `fetchUpdateInfo`.

## v0.1.1-dev65 â€” Rip Refinement Follow-up (Layout Pass + Load Disc)

- **Rip vertical-space layout pass** â€” the Content Browser (title list) is the flexible region of the left column but was starved by fixed-height consumers. The Rip Log is now a collapsed-by-default strip (VSplit offset 0.97, only the LOG pill control visible; expands to a bounded ~28% at 0.72 via the pill; auto-opens on rip activity/errors through the new `SetRipLogExpand` hook in Options, wired in `appendRipLog`) and the Disc Menu preview collapses to a ~40px strip until a menu frame loads (`hasMenu` gates the Preserve/Main check row; Clear ISO resets the preview). The title list reclaims the freed height with no row/typography changes or 55/45 HSplit touch. Committed as `4090b5ec`.
- **Rip: load disc directly from an optical drive** â€” a LOAD DISC button in the rip Source row (left of Browse) calls the root `OnLoadDisc` callback: `detectOpticalDrives()` (reused from the burn module) lists drives, a radio custom-confirm dialog (`RipSelectDriveTitle`) picks one when several exist (cancel returns unchanged), `resolveOpticalDriveVIDEOTS` resolves the disc (Windows: drive letter + `VIDEO_TS` exists; Linux: device symlink â†’ mount point from `/proc/mounts`), and the module feeds the path into the existing `loadDisc()` load/scan/rip path. Errors surface via `RipErrNoDrive`/`RipErrNoDVD`/`RipDriveNotMounted`. New i18n keys across en/fr/iu/iu_latin. Design: `docs/RIP_LOAD_DISC.md`. Committed as `4090b5ec`.

## v0.1.1-dev63 â€” Base Cycle + dev62 Follow-up Patches

- **Convert/Filters/Upscale metadata-panel fold bug fixed** â€” folding a metadata panel hid its tappable `METADATA` header because `metaHeader` lived inside the hidden `metaPanel` (folding hid the whole panel, so a folded panel could never be re-expanded). `buildMetadataPanel` gained an `initiallyOpen` param: the header row stays visible and only the body hides. The caller's `onToggle` no longer Show()/Hide()s the whole panel â€” it only persists the open state and adjusts the split offset. Convert honours `state.convert.MetadataOpen`; Filters/Upscale default open.
- **Rip ISO-load scanning-state bug fixed** â€” `loadDisc` called `SetScanning()` before `rebuildEnrich()`, whose `updateDiscInfo()` renders `SetEmpty()` for a nil scan result, clobbering the scanning state so loading an ISO never showed "Reading disc information". `SetScanning()` now runs after the enrich rebuild.
- **Rip ISO scan hardened** â€” `runISOScan` wraps `scanISOViaUDF` for the UI goroutine: a UDF-reader panic (malformed descriptor/data) converts to a visible `SetError` instead of killing the process or a silently stuck card, start/done/failure are logged under `CatDVD`, and re-entering the rip module re-scans a previously selected source (the view builds fresh scan state per entry, which previously left a restored path on "No disc loaded").

## v0.1.1-dev62 â€” Rip Layout Correction Pass

- **Action-bar band root cause fixed** â€” the dev61 action bar (readiness label + three PillButtons in one HBox) rendered as a giant band with vertically stretched buttons and a one-character-per-line label, clipping the right column behind it. Root cause: the label had `TextWrapWord` inside an HBox â€” Fyne HBox children stretch to the container's height, and a wrapping label collapses to its narrowest word-boundary width then re-measures multi-line tall; `PillButton` fills whatever size it is given, so it stretched to match. The bar is now a Border: buttons in the right edge at natural height, label as the centre with ellipsis truncation (single line, absorbs leftover width).
- **Compact empty states** â€” DiscSummary hides the tech/main-feature rows in empty/scanning/error states (hidden rows contribute no MinSize) and uses the proper `Truncation` field; MenuPreview collapses to a bounded 40px `NO MENU FOUND` strip (hidden 16:9 image min no longer contributes); ContentBrowser overlays a centred "Load an ISO or VIDEO_TS folder to begin." hint on the flexible title-list region that hides once titles load.
- **First in-app update channel exercise** â€” dev61â†’dev62 is the first release pair where the fixed Install Update path (dev61's asset-suffix fix) can be tested end-to-end from a released binary.

## v0.1.1-dev61 â€” Rip Density Refinement + Global Header Migration + Updater Fix

- **Two-column rip workspace** â€” dev58's linear vertical flow (SOURCEâ†’DISCâ†’TITLESâ†’OUTPUTâ†’ACTION as one long scroll) replaced with a two-column workspace: LEFT = CONTENT 55% (DiscSummary pinned top, MenuPreview pinned bottom, title list as the flexible center with internal scroll), RIGHT = PROCESSING 45% (Format/enrichment, Output as a plain bold subsection, Status); SOURCE + action bar (readiness line left, Add to Queue / Open in Player / RIP NOW right) span full width above/below.
- **`ui.SectionBox` shared component** â€” compact 28px teal header, navy body, optional header actions that don't grow the header. Replaces the per-module `buildXxxBox` duplication across the rip module (view.go, disc_summary.go, content_list.go, menu_preview.go) plus 7 other module helpers (audio/upscale/thumbnail/inspect/compare/filters/trim + trim stub). Trim's `*fyne.Container` return loosened to `fyne.CanvasObject`; audio's dead shadowed package-level `buildAudioBox` removed. Density: section gaps 10pxâ†’6px, thumbnails 80Ã—60â†’56Ã—42, menu preview 320Ã—180â†’280Ã—158, DiscSummary empty-state spacer removed. Build + vet + gofmt green across all 9 commits.
- **In-app updater asset-suffix fix** â€” `fetchReleaseAssetURL` looked for assets ending `_windows.zip`/`_linux.zip` but CI publishes `_windows_amd64.zip`/`_linux_amd64.tar.gz`, so "Install Update" always failed with "no compatible asset found in release" before downloading anything â€” this had been broken since at least dev51 (why updates were always manual downloads). Suffix now matches the real artifact names; from dev61 onward in-app updates work. Follow-ups flagged in TODO: bake `buildCommit` via ldflags (patch detection dead), Linux tar.gz extraction path, stale nightly-PATCH comment in `fetchUpdateInfo`.

## v0.1.1-dev60 â€” Release Cut (dev59 content + CI FFmpeg build resilience)

- **Ship happened as dev60, not dev59** â€” dev59 was tagged (`aa73122b`) but its GitHub Release was never published: the tag run's windows FFmpeg cold build red'd on a transient source flake, the `release` job (`needs: [linux, windows]`) got skipped, and GitHub's "Re-run failed jobs" does not re-run skipped jobs. dev59's content (below) + the CI resilience shipped from the hardened `0166aa61` as `v0.1.1-dev60`.
- **Published 2026-09-02** â€” `v0.1.1-dev60` GitHub Release live with both platform assets (`VideoTools_v0.1.1-dev60_linux_amd64.tar.gz` 27.4 MB, `_windows_amd64.zip` 58.3 MB). The hardened workflow's cold FFmpeg build (with retries) went green on both platforms on the first tag run â€” the resilience fix worked.
- **CI FFmpeg build resilience** â€” all five cache steps (dev/release Ã— linux/windows, msix) gained `restore-keys` so a new tag reuses the previous tag's FFmpeg build instead of a ~16 min cold rebuild; `curl` source downloads retry (`--retry 3 --retry-delay 5 --retry-all-errors`) and msix `wget` retries (`--tries=5 --retry-connrefused --waitretry=5`). release #14 + MSIX #19 had red'd on the cold build while dev #65 "passed" only via cache hit â€” dev-status is not evidence a cold build path is healthy.
- **MSIX FFmpeg step unblocked** â€” MSIX #19/#20 red'd twice on `Setup static FFmpeg (Windows)` (`configure: error: Package requirements (dvdread >= 6.0.0) were not met`). The msix step copied the dev/release dvdvideo block but dropped its `export PKG_CONFIG_PATH=/c/ffmpeg-static/lib/pkgconfig`; `dvdread.pc` was installed into `/c/ffmpeg-static/lib/pkgconfig` yet pkg-config never searched it, and the `.pc` copy to `/ucrt64/lib/pkgconfig` ran only after libdvdnav configure had already failed. Added the export + a `pkg-config --exists dvdread` fail-fast guard (matching dev/release), committed for the next MSIX run.

## v0.1.1-dev59 â€” Blocked-Straggler Cleanup + Rip Crash Fix (content shipped as dev60)

- **Rip view crash on open fixed** â€” `internal/app/modules/rip/view.go` assigned `updateDiscInfo` after the initial `rebuildEnrich()` call, but that call invokes it: nil-closure panic during `buildRipView()`, which escapes `setContent`'s recover (argument evaluated before recover armed) â†’ silent process death, no `crashes.log`. Now `discSummary`/`updateDiscInfo` are assigned first; `showRipView` also gained a recover that logs a stack trace to `crashes.log` before re-panicking.
- **Codeberg mirror retired** â€” Codeberg will not host majority machine-generated code; the Codeberg push URL was removed from `origin` (GitHub is now the only remote). TODO.md mirror-validation item marked done.
- **Upscale legacy render-based dual player removed** â€” `OnDualPlayerSeek`/`OnDualPlayerRender` (`types.go`) and the `renderDualPlayerPreview` no-op stub (`native_media.go` / `native_media_stub.go`) had no consumer; the module's dual-pane uses the InlineVideoPlayer `SetPeer` model, so the legacy render API was dead surface and was deleted instead of implemented. Unused `time` imports removed.
- **Local Windows builds unblocked** â€” `scripts/windows/dev-verify.ps1` sets the CGo stack-LDFLAG allow + quotes gcc/g++, then runs `go build -tags native_media ./...` + `go vet ./...`. Verified green cold (~9 min) and warm locally.
- **Stale blocker docs retired** â€” AGENTS.md "Blocked stragglers" line + `renderDualPlayerPreview` priority removed; PillIconButton `SetIcon` (exists since dev48) and the utilsâ†’uiâ†’benchmarkâ†’utils cycle (resolved at MakeIconButton revert) confirmed no longer blocks. Docs (AGENTS/TODO/roadmap/CHANGELOG/DONE) synced to dev59.
- **CI FFmpeg build resilience** â€” release #14 + MSIX #19 red'd on a cold FFmpeg build while dev #65 "passed" purely via cache hit (build skipped), so dev-status was not evidence the build path was healthy. All five cache steps (dev/release Ã— linux/windows, msix) gained `restore-keys: <prefix>-` so a new tag reuses the previous tag's FFmpeg build instead of a ~16 min rebuild; the `curl` source downloads gained `--retry 3 --retry-delay 5 --retry-all-errors` and msix `wget` gained `--tries=5 --retry-connrefused --waitretry=5` so a transient runner-side source-mirror blip no longer reds the pipeline.

## v0.1.1-dev58 â€” Rip Module Overhaul (UX) + release cut

- **Linear SOURCEâ†’DISCâ†’TITLESâ†’OUTPUTâ†’ACTION workflow** â€” single vertical flow replaces the two-panel split where the Content Browser dominated the screen. Source box â†’ DiscSummary card â†’ Content Browser (TITLES) â†’ Format â†’ Menu preview â†’ Output â†’ Action box (RIP NOW / Add to Queue / Open in Player).
- **Disc info populates reliably on load** â€” `ScanDisc.VideoStandard` was read before the VTS cache filled (NTSC/PAL never surfaced); detection moved after the loop (verified on a real disc: DVD-9, PAL, Region Free, 4 titles). Disc info decoupled into `updateDiscInfo()` so an enrichment failure can't hide it.
- **Dedicated `DiscSummary` card** â€” explicit empty/scanning/scanned/error states; type Â· standard Â· region Â· size Â· title count + â˜… Main Feature (longest title).
- **Advanced accordion** â€” menus preservation, PALâ†”NTSC conversion, and full-disc extraction grouped; common chapter/audio/subtitle options stay first-class. Format-aware visibility preserved.
- **Readiness line** â€” "Ready to rip N title(s)" updates on scan and selection change, anchoring the action box.
- **Compact collapsible log** â€” default 0.92 split offset, â–¼/â–¶ LOG toggle.
- **i18n pass** â€” all rip module hardcoded labels/options/dialogs/region strings localized across en/fr/iu/iu_latin.
- **Tester gate (dev58):** confirm disc-info populates on ISO/VIDEO_TS load, scan flow reads clean, and the dev58 build passes sign-off.

## v0.1.1-dev57 â€” Release Cut: dvdvideo detection fixes

- **dvdvideo detection string fixed everywhere** â€” both the CI verify grep and the rip module's `SupportsDVDVideo()` runtime probe used the spaced long name `"DVD video demuxer"`, which never matches FFmpeg 8.1 (real long_name is `DVD-Video`, hyphenated). The FATAL gate always fired on a correct build and the probe always returned false (silently forcing VOB concat on every rip). Both now match the `dvdvideo` short name â€” safe because a missing demuxer exits non-zero and returns early at the error gate.
- **CI green** (`eab76992`) â€” Windows FFmpeg builds pass the corrected verify grep; the demuxer ships in all three workflows.
- **Runtime probe verified** against a real FFmpeg 8.1 binary (`ffmpeg -h demuxer=dvdvideo` â†’ `True`). Cell-accurate dvdvideo rips finally activate.

## v0.1.1-dev56 â€” NTSC/PAL Detection + CI FFmpeg Build Fixes

- **NTSC/PAL video standard detection on disc load** â€” `DiscScanResult.VideoStandard` set to `"NTSC"` or `"PAL"` from the first title's VTS IFO PGC frame-rate bits. The `IsNTSC` flag was already extracted by the IFO layer (`extract.go:175`) but never surfaced. Displayed in the disc-info label: `DVD-9 Â· NTSC Â· Region 1 Â· 7.2 GB`.
- **CI: FFmpeg `.tar.xz` â†’ `.tar.bz2`** â€” MSYS2 tar may lack xz decompression on current GitHub-hosted runners. dev.yml and release.yml now match the proven MSIX workflow format. Cache bumped to v9.
- **CI: configure failure diagnostics** â€” `ffbuild/config.log` last 80 lines dumped on configure failure; dvdvideo diagnostic probes (pkg-config, dvdnav.pc dump, config.log grep) added before/after configure.
- **Content browser build errors fixed** â€” 7 compile errors: `dvdPlayer` scope, ContentBrowser/MenuPreview `CreateRenderer`, `float32` conversion, `formatTimestamp` helper (`24dc67ae`).
- **dvdvideo detection string fixed in two places** â€” CI verify grep `"DVD video demuxer"` â†’ `"dvdvideo"` (the demuxer's long_name is `DVD-Video`, hyphenated, so the FATAL gate always fired on a correct build; Windows pipelines now green, `eab76992`), and the rip module's `SupportsDVDVideo()` runtime probe, which used the same spaced string and **always returned false â€” silently forcing VOB concat on every rip**. The probe now matches the short name (missing demuxer fails the command and returns early), so cell-accurate dvdvideo rips finally activate at runtime.

## v0.1.1-dev55 â€” seekGen Crash Fix + Convert Layout + Resume Fix + VLC Decision

- **seekGen log spam crash fixed** â€” `lastSeekGen` was never updated after comparison, causing 60Ã—/sec "first frame after seek" log lines forever. I/O pressure killed the process (dev53 crash on seek). Fix: assign `lastSeekGen = gen` after the check.
- **Settings panel collapse fix** â€” captured `settingsHeaderUpdate` callback, calls with `state.convert.SettingsOpen` on toggle; `settingsTabsPanel.Hide()`/`Show()` for visual collapse.
- **Metadata header arrow sync** â€” captured `metaHeaderUpdate` callback, calls with `state.convert.MetadataOpen` on creation â€” arrow now reflects persisted state.
- **Config migration** â€” pre-dev54 configs (all layout fields false) default to all panels expanded.
- **Convert button colour** â€” changed from `ui.Magenta` to `convertColor` (#7225D0).
- **Resume crash fix** â€” `Resume()` now flushes stale video+audio queues, drains `frameQueue`, flushes video codec buffers, resets `decodeEOFSent`/`seekFlushBefore` â€” eliminates crash on app restart from stale state.
- **Thumbnail extraction deferred** â€” 3 seconds after load via goroutine; prevents thumbnail I/O from competing with initial playback.
- **Anti-rationalization table added to AGENTS.md** â€” pre-written rebuttals to common shortcuts.
- **Verification discipline section added to AGENTS.md** â€” formalized log-review-before-landing and state-tracking-var-must-be-updated rules.
- **Strategic decision: libVLC backend** â€” replace custom FFmpeg engine with libVLC for user-facing playback. Design doc: `docs/VLC_PLAYER.md`. FFmpeg engine stays as long-term plan.
- **dvdvideo demuxer actually ships (rip fix)** â€” libdvdread 6.1.3 + libdvdnav 6.1.1 built from source (static-only) in all three Windows workflows (dev, release, msix); FFmpeg configure gets `--enable-libdvdnav --enable-libdvdread`. `dvdnav.pc` rewritten after install so `-ldvdread` sits in `Libs` (x265.pc precedent â€” FFmpeg Windows configure calls pkg-config without `--static`, so the stock `Requires.private` was never expanded). Verify step gates on `ffmpeg -h demuxer=dvdvideo`.
- **No-scan rips use dvdvideo** â€” executor no longer requires `TitleNumber > 0`; defaults to title 1. Eliminates the VOB-boundary PTS discontinuity crash on multi-VOB discs (~25â€“32% mark).
- **Menu VOB audio tolerance** â€” `-map 0:a?` on menu VOB export paths so menu VOBs with no audio stream don't fail.
- **CI cache keys bumped** â€” Windows ffmpeg cache `v5`â†’`v6` (dev + release), msix `v2`â†’`v3`.
- **Player minimize now frees the full column for metadata** â€” Filters/Upscale/Inspect/Trim collapsible player headers only moved the split offset; the video content was never hidden, so Fyne's `Split` clamped to the video's 480Ã—270 min size and the player frame stayed on screen while the metadata panel stayed squashed. Player `onToggle` now calls `Hide()`/`Show()` on the video area (`videoArea` in Filters/Upscale, `videoContainer` in Inspect/Trim), mirroring Convert â€” collapsing the player shrinks the split to just the header bar so metadata (Filters/Upscale), info tabs (Inspect), or timeline+toolbar (Trim) take the full column. Filters/Upscale metadata toggle also `Hide()`/`Show()`s its panel so a folded metadata pane frees the full column for the player; Upscale's `metaPanel` was restructured to var-then-assign so the `onToggle` closure can reference it (Convert's existing pattern).
- **ContentBrowser replaces rip player pane** â€” scrollable list with cycling-still thumbnails (5 keyframes per title via ffmpeg), duration, chapter/audio/subtitle counts, and per-title selection checkboxes. Left accent bar: teal = selected for export, pink = not selected. Header with Select All / Deselect All. Card tap focuses for preview.
- **MenuPreview widget** â€” static menu frame capture via ffmpeg with Preserve Menus and Main Feature toggles. Placed between Format and Output in the rip view. Falls back to placeholder when no menu VOB found.
- **UDF reader fix** â€” `ReadDescriptor` had two bugs: `header[12:14]` read wrong bytes (should be `header[10:12]` for `DescriptorCRCLen`), and returned data excluded the 16-byte tag header that all caller structs expect. ISO scanning now succeeds on real-world discs.
- **CI Node.js 24 migration** â€” all GitHub Actions upgraded to Node 24-compatible versions; cache keys bumped (v7â†’v8 dev/release, v3â†’v4 msix).

## v0.1.1-dev54 â€” Player Performance Fixes

Six identified performance bottlenecks in the native media player fixed:

- **Decode loop CPU spin** â€” `TimedGet(20ms)` replaces `TryGet()` + 1ms poll; no more 100% CPU when queue empty.
- **Seek-on-resume stutter** â€” `FlushAudioCodec()` replaces full `Seek()` on unpause; eliminates 50-200ms audio stutter.
- **Slider update congestion** â€” Throttled to ~15fps (66ms min interval); reduces GUI thread pressure.
- **Per-frame subtitle lock** â€” `hasSubtitleActive` atomic.Bool replaces `subtitleCodecMu` lock/unlock 30x/sec when no subtitles active.
- **sws_scale performance** â€” `SWS_FAST_BILINEAR` replaces `SWS_BICUBIC` for same-res intermediate conversion.
- **Decode loop paused check** â€” `pausedAtomic` (atomic.Bool) replaces `lockMu()` for paused flag; ~1ns vs ~50ns.
- Convert module layout state (player/metadata/settings panel collapsed/expanded) persisted across sessions.

## v0.1.1-dev53 â€” Update Checker Migration

- **Update checker migrated from Forgejo to GitHub API** â€” `settings_module.go` now queries GitHub API (`api.github.com/repos/LeakTechnologies/VideoTools/`) for tags and releases instead of the old Forgejo instance (`git.leaktechnologies.dev`). All three API endpoints updated: tags, releases-by-tag, and releases page URL.

## v0.1.1-dev52 â€” CI & Infrastructure Hardening

- GitHub Actions CI green on both platforms (six root-cause Windows build fixes).
- Windows product: three fully static binaries; DLL/ folder retired; objdump gates in all pipelines (dev, release, msix, Forgejo).
- v0.1.1-dev51 release published from the new pipeline; MSIX pipeline verified green.
- AGENTS.md restructured to rules-only + release protocol; README/install docs refreshed; repo history cleaned of AI attribution.

## Version 0.1.1-dev51 (in progress)

### Player Overlay & Cleanup (dev51)

- **P0: Error/loading/buffering overlay indicators wired** â€” `loadingSpinner`, `bufferingLabel`, `errorLabel`, `errorIndicator` were created and mutated by `SetLoading()`/`SetBuffering()`/`SetError()`/`ClearError()` but never added to `videoPlayerRenderer.Objects()` or positioned in `Layout()`. Now all four widgets render centred over the video area with proper z-ordering. Loading spinner shows during file open, buffering label during buffer underrun, red circle + error message on decode/stream errors.
- **P2: Stub method-set divergence fixed** â€” `inline_player_stub.go` was missing 9 methods (`SetSeekAccuracy`, `SetAudioDelay`, `SetFilterPipeline`, `GetLastVideoPTS`, `GetLastAudioPTS`, `Enqueue`, `ClearPlaylist`, `PlaylistLen`, `SetPeer`). All added so both build targets expose the identical method set.
- **P2: Dead fields/callbacks removed from VideoPlayer** â€” `OnFrameRate`/`onFrameRate`, `OnChapterSelect`/`onChapterSelect`, `OnHover`/`onHover`, `GetHoverFrame`, `displayFrame`, `displayWidth`, `displayHeight`, `frameSeq`, `lastFrameSeq`, `chapterMark` â€” all declared but never used or wired. Removed from struct and methods.
- **P2: Cosmetic fullscreen/PiP buttons removed** â€” `toggleFullscreen`/`SetFullscreen`/`IsFullscreen`/`OnFullscreen`/`isFullscreen`/`fullscreenBtn` and `togglePiP`/`IsPiP`/`OnPiP`/`isPiP`/`pipBtn` flipped booleans and buttons but never entered fullscreen or picture-in-picture. Removed from struct, methods, and control bar layout.
- **P2: CC button wired to engine** â€” `toggleSubtitles()` now calls `OnSubtitles` callback. `InlineVideoPlayer.NewInlineVideoPlayer()` wires `OnSubtitles` to call `SelectSubtitleTrack(0)` when enabling (first available subtitle track) or `DisableSubtitles()` when disabling. Previously the CC button just flipped a boolean and logged.
- **Orphaned GPU package removed** â€” `internal/media/gpu/` (8 Go files, 3 GLSL shaders) and `docs/gpu/` (5 docs) deleted. Zero imports confirmed.
- **P1: view.go component split** â€” 1442-line monolith split into 5 focused files: `view.go` (566, struct/renderer/draw), `split_view.go` (193, independent SplitView widget), `control_overlay.go` (598, transport/OSD/callbacks), `keyboard_shortcuts.go` (50, tap/key handlers), `thumbnail_preview.go` (36, cache). Missing `OnSubtitles()` setter added back.
- **P1: UDF thread safety** â€” `partitionStartAbs` was read/written without mutex in 9 locations. Added `partitionStart()`/`setPartitionStart()` mutex-protected helpers. Added `SetProgressCallback()` for per-file extraction progress. `iso_udf.go` now uses `defer reader.Cleanup()`.
- **Legacy singleton alias vars removed** â€” 10 per-module vars (`convertInlinePlayer`, `convertPreviewPlayer`, `trimInlinePlayer`, etc.) removed from `native_media.go` as part of completing the dev49 singleton consolidation. All callers had already been migrated to `GetXxxPlayer()` getters.

## Version 0.1.1-dev50 (in progress)

### Windows DLL Pipeline Overhaul (BUG-012 + BUG-013)

- **All three CI pipelines now build FFmpeg shared from source** â€” Forgejo dev-packages.yml, GitHub release.yml, and GitHub windows-msix.yml all build FFmpeg 8.1 from the same source tarball twice: once static (for CGo link into VideoTools.exe) and once shared (for DLLs, ffmpeg.exe, ffprobe.exe). BtbN downloads completely eliminated. Shared build reuses the same x264/x265 static archives from the static build step. Cache keys versioned (`ffmpeg-shared-*-v1`).
- **GitHub `release.yml` rewritten** â€” replaced BtbN-for-everything with MSYS2 ucrt64 toolchain, source-built static FFmpeg 8.1, source-built shared FFmpeg 8.1, objdump transitive-dep scan, `ffmpeg.exe`/`ffprobe.exe` bundled. Previous workflow used BtbN for CGo link (broken â€” no `.a` libs), only copied `av*.dll`+`sw*.dll`, never included CLI tools.
- **GitHub `windows-msix.yml` rewritten** â€” same pipeline pattern. MSIX layout includes `DLL/` with full transitive-dep scan.
- **Forgejo `dev-packages.yml` rewritten** â€” replaced BtbN download step with source-built shared FFmpeg step, matching the other two pipelines. Same objdump transitive-dep scan in bash.
- **`ExpectedFFmpegDLLs()` uses glob patterns** â€” `avcodec-*.dll` instead of `avcodec-61.dll`. Prevents validation breakage when FFmpeg ABI version bumps. `ValidateFFmpegDLLs()` uses `filepath.Glob` instead of exact `os.Stat`.
- **BUG-013 closed** â€” BtbN `latest` moving tag eliminated. All DLLs built from pinned FFmpeg 8.1 source.
- **`AGENTS.md` settled decision updated** â€” clarified that "never use BtbN" means both static and shared builds. BtbN downloads removed from all CI pipelines.
- **`docs/DLL_BOOTSTRAP.md` updated** â€” architecture diagram shows source-built shared DLLs, not BtbN download. Pipeline table updated for all three workflows.

### DLL Startup Validation + CGo Consolidation

- **`ValidateFFmpegDLLs()` in `ffmpeg_bootstrap.go`** â€” after `AddFFmpegDllsToPath()` prepends the DLL dir to PATH, runs a live smoke test: bundles `ffprobe.exe -version` and checks every expected DLL exists. Failure shows a non-blocking Fyne error dialog at startup with clear instructions, instead of a silent log warning.
- **`--dllcheck` CLI flag** â€” standalone DLL diagnostics without launching GUI: prints DLL directory, all found DLLs with sizes, PATH inspection, expected DLL set, and ffprobe.exe presence/smoke test. Exit code 1 on validation failure.
- **`DiagnoseDLLSetup()`** â€” returns a multi-line diagnostic string for error dialogs and logging.
- **`ExpectedFFmpegDLLs()`** â€” canonical list of FFmpeg ABI DLLs the runtime expects.
- **CGo directive consolidation** â€” all 15 duplicate `#cgo windows CFLAGS/LDFLAGS` directives moved from individual source files into a single `internal/media/cgo_preamble.go`. Linux/macOS already uses pkg-config (unchanged). Windows hardcoded path kept as local-dev fallback, with a clear comment that CI overrides via environment variables.
- **`docs/DLL_BOOTSTRAP.md`** â€” comprehensive documentation: DLL pipeline, search order, startup validation, common issues and fixes, developer setup.
- **Goal:** zero-touch boot â€” VT always starts even if DLLs are missing or broken, with a clear error message instead of a crash or silent failure.

### Collapsible Player Panel (Convert, Filters, Upscale)

- **Collapsible player panel** in Convert module â€” `BuildCollapsibleHeader(t.ConvertSectionPlayer, convertColor, ...)` wraps `videoPanel`; toggle sets `leftColumn.SetOffset(0.5)` (open) or `0.03` (collapsed). `ConvertSectionPlayer` i18n key in all 4 locales.
- **Filters + Upscale collapsible player** â€” same `BuildCollapsibleHeader` wraps the video area in both modules; `resolveOffset()` tracks `playerOpen`+`metaOpen` so each toggle is additive; offset matrix: both open=0.65/0.60, player closed=0.03, meta closed=0.97.
- **Inspect collapsible player** â€” fixed `GridWithColumns(2)` â†’ `container.NewHSplit`; `BuildCollapsibleHeader` drives `mainSplit.SetOffset(0.5 / 0.03)`; tabbed info panel expands when player is folded.
- **Trim collapsible player** â€” `leftSide` Border â†’ `container.NewVSplit` (`leftVSplit` at 0.65); player collapses to 0.03, timeline + toolbar + in/out controls always remain visible in the lower half.

### Updater â€” Sidecar File Refresh

- **`updateSidecars()`** in `settings_module.go` â€” in-place update now extracts `DLL/*.dll`, `ffmpeg.exe`, `ffprobe.exe` from the zip alongside `VideoTools.exe`. Fixes stale DLLs after in-place updates.

### Logging â€” Windows Log-Clear Fix + Version Header

- **`logging.Clear()` Windows O_APPEND truncate fix** â€” close â†’ reopen O_TRUNC â†’ write header â†’ close â†’ reopen O_APPEND. Resolves "Access is denied" on Windows.
- **`logging.SetVersion(v)` + `sessionHeader()`** â€” version string embedded in startup and clear log headers.

### CI â€” Stale DLL Cache Detection

- **Forgejo CI runner DLL cache fix** â€” skip-download now also checks for `liblzma-5.dll`; auto-wipes stale cache if missing.

### UDF Reader â€” Allocation Descriptor Parsing + Partition Offset

- **`readFIDs` ShortAd parsing** â€” parse allocation descriptors from ICB data; multi-extent directory support.
- **Partition offset applied universally** in `findFSD`, `extractRecursively`, `extractFile`, `ReadFileData` â€” all LBNs now correctly offset by `partitionStartAbs`.
- **`extractFile` / `ReadFileData`** â€” read from allocation descriptor payload using `InformationLength`, not ICB location.

### Playlist / Sequential Playback

- **`InlineVideoPlayer.Enqueue(path)`** â€” appends to internal playlist. On clean EOF, `playbackLoop` advances to the next item, loads it, and auto-plays. `ClearPlaylist()` empties queue; `PlaylistLen()` reports remaining items. Direct `Load`/`LoadDVD`/`LoadURL` resets playlist. `playlist []string` + `playlistIdx int` fields in struct.

### HDR Tone-Mapping

- **`internal/media/hdr.go`** â€” `isFrameHDR` checks `frame->color_trc` (AVCOL_TRC_SMPTE2084 / AVCOL_TRC_ARIB_STD_B67). `renderSWFrame` in `playback.go` calls `applyHDRTonemap(e.frame)` before `ensureSwsCtx`+`toRGBA`. Filter graph: `zscale(t=linear,npl=1000)â†’format(gbrpf32le)â†’tonemap(hable,desat=0.5)â†’zscale(t=bt709,m=bt709)â†’format(yuv420p)`. `hdrTonemapUnsupported` flag prevents retry when libzimg/zscale are unavailable. Applied in all four SW decode sites (GrabFrame SW, GrabFrame HWâ†’SW fallback, videoDecodeLoop SW, videoDecodeLoop HWâ†’SW fallback). Engine struct: `hdrFilterGraph`, `hdrBuffersrc`, `hdrBuffersink`, `hdrInputPixFmt`, `hdrTonemapUnsupported`. `freeHDRFilter()` called from `Close()`.

### Per-Codec HW Decode Deny-List

- **`media.SetHWCodecDenyList(s)`** â€” comma-separated codec names forced to SW decode. `codecCanUseHWDevice` checks deny-list first. `PrefsConfig.HWCodecDenyList` persists. Settings â†’ Player text entry. Loaded at startup.

### Error Resilience

- **`setVideoCodecErrorFlags()`** â€” sets `error_concealment = FF_EC_GUESS_MVS | FF_EC_DEBLOCK` before `avcodec_open2` on both video codec init paths. Explicit assignment guards against `avcodec_parameters_to_context` resetting the default.

### Mid-Playback Audio and Subtitle Track Switching

- **`SelectAudioTrack` use-after-free fixed** â€” Close old `AudioPlayer` before freeing `audioCodecCtx`. Restores speed/volume/muted. Seeks to current PTS for A/V resync. Resumes if playing.
- **`SelectSubtitleTrack` codec reinit** â€” Flushes queue, frees old codec ctx, calls `initSubtitleDecoder` for new stream, clears stale overlay.
- **`Engine.subtitleCodecMu`** â€” New mutex protecting all `subtitleCodecCtx` access: demuxerLoop, NextFrame, decodeSubtitle, SelectSubtitleTrack, DisableSubtitles, Close.

### HW Decode Default-On + Error Concealment

- **`hwDecodeEnabled = true`** (`internal/media/hwdecode.go`) â€” D3D11VA/VAAPI/QSV enabled by default. All FFmpeg decode call sites are SEH-wrapped in `safe_bridge.c`. `DegradeToSoftware()` wired into decode loop.
- **`Engine.lastGoodFrame` + `decodeErrored`** (`internal/media/engine.go`, `internal/media/playback.go`) â€” `atomic.Pointer[image.RGBA]` stores most recently displayed frame; `atomic.Bool` set on fatal decode errors. `NextFrame` returns frozen frame once on decode-error EOF via `CompareAndSwap`, preventing black-screen on corrupt/HW-failed streams.

### ASS Subtitle Format Fixes

- **`formatASSTime`** (`internal/media/subtitle.go`): `(int(d.Milliseconds()) % 1000) / 10` â€” was dividing total ms by 10, giving wrong centiseconds. `% 1000` isolates sub-second part first.
- **`escapeASSText`**: Removed `strings.ReplaceAll(text, "}", "\\}")` â€” `}` is not a special character in ASS when unmatched; only `{` needs escaping.

### P1-5: A-B Loop

- **`Engine.SetLoopPoints(a, b float64)` / `SetABLoopEnabled(bool)`** â€” stores loopA and loopB PTS thresholds; when enabled, `NextFrame` checks PTS after each decoded frame and seeks back to loopA when PTS >= loopB via `abLoopPending` flag signal on the next `NextFrame` call.
- **`InlineVideoPlayer.SetABLoopEnabled(bool)` / `SetLoopPoints(a, b float64)`** â€” forwards to Engine through the standard InlineVideoPlayer API layer.
- **Zero overhead when disabled** â€” `abLoopEnabled` check is a single mutex-guarded lookup; no branching cost for normal playback when disabled.
- Builds clean, all UI/media package tests pass.

### P1-8: Frame Timing Diagnostics Overlay

- **`VideoPlayer.SetFrameTimingVisible(bool)` / `SetFrameTimingText(string)`** â€” overlay canvas elements (`frameTimingBg`, `frameTimingText`) rendered in top-right corner showing: sequential frame count, PTS (s), inter-frame delta (ms), and PTS delta (s). Auto-sized via `fyne.MeasureText`.
- **Per-frame collection at InlineVideoPlayer level** â€” `frameTimingCount`, `frameTimingLastPTS`, `frameTimingLastTime` fields updated on each valid frame; formatted string pushed to widget via `DoFromGoroutine`.
- **`InlineVideoPlayer.SetFrameTimingOverlayVisible(bool)`** â€” toggle that propagates to the VideoPlayer widget; resets counters when disabled.
- Builds clean, all UI/media package tests pass.

### P1-11: Clock Drift Correction

- **`MasterClock.SetTime()` underrun recovery exception** at `internal/media/clock.go:58-66`: the monotonic ratchet that prevents backward clock jumps now carves out a special case â€” if the backward jump exceeds 1s and no PTS anchor has arrived in the last 500ms, it's treated as an audio underrun recovery. The clock resets to the incoming PTS instead of staying frozen at the drifted value.
- **Targeted, no-regression fix**: preserves the original ratchet behavior for all normal cases (small jitter, pre-buffered audio). Only large jumps with stale anchors bypass the guard.

### P1-7: Bilinear Scaling (docs-only)

- **Confirmed SWS_BICUBIC** at `internal/media/engine.go:1445` (video decode â†’ RGBA sws_scale) and `framepool.go:36` (thumbnail pipeline). The FFmpeg swscale step that converts decoded frames to RGBA has always used bicubic interpolation â€” the most quality-determining step in the pipeline.
- **`scaleNearest` docstring clarified** in `internal/media/view.go:833` â€” explicitly notes that this is only the final canvas-positioning blit. The heavy lifting (colour space conversion + dimension scaling) is done by FFmpeg's SWS_BICUBIC before the canvas render.

### P1-10: Growing/In-Progress File Support

- **Engine-level toggle** â€” `Engine.SetGrowingFile(bool)` + `IsGrowingFile()` in `internal/media/engine.go`, implemented as simple locked accessors alongside `looping`. `InlineVideoPlayer.SetGrowingFile(bool)` wires through.
- **EOF polling** â€” `InlineVideoPlayer.growingFileWatcher(path, lastPos)` goroutine: polls `os.Stat` on a 2s ticker. Records initial file size; when `fi.Size()` exceeds the recorded size, calls `v.Load(path)` to re-open the grown file, `v.Seek(lastPos)` to restore the playback position, and `v.Play()` to resume. Exits the goroutine on success.
- **No-regression EOF path** â€” When `eng.IsGrowingFile()` is false (default), the existing behavior is preserved: `onEnd` fires, resume marks completed, and the file is reloaded for re-play. Growing-file only activates when explicitly toggled on and EOF is reached.
- Builds clean with `CGO_LDFLAGS_ALLOW`.

### P1-2: Resume/Watch-Later

- **`InlineVideoPlayer` fields** â€” `resumeState *state.ResumeState` and `lastSave time.Time` added to the struct. `SetResumeState(s)` allows any caller to attach a persisted playback-position store.
- **Auto-restore in `loadViaOpen`** â€” after scrubber starts, checks `resumeState.GetPosition(displayPath)` + `ShouldResume()`. If valid, calls `eng.Seek(saved.Position)` + `eng.NextFrame()` and sets `firstFrame` to the resume frame, so the widget shows the resume point immediately.
- **Auto-save in `playbackLoop`** â€” at the bottom of each loop iteration (after onProgress dispatch), snapshots `resumeState` and `currentPath` at the top of the loop, then saves position every 5s (throttled via `lastSave`) using `rs.SavePosition(path, t, dur)`. The snapshot-at-top ensures path and engine are always consistent even if `Load()` runs concurrently.
- **Auto-mark-completed on EOF** â€” before dispatching the end-of-stream reload, calls `rs.MarkCompleted(path)` so the same file won't resume again.
- **`state/resume.go` build constraint removed** â€” the `//go:build native_media` tag was unnecessary (pure Go, no CGo/FFmpeg deps). Without it, `inline_player_stub.go` can import the package directly without a matching stub type.
- **Shared ResumeState wired in `native_media.go:initNativeMediaAssets`** â€” created with `filepath.Join(defaultVideoToolsRoot(), "state")` and set on both `primaryInlinePlayer` and `previewPlayer`. Every module using the shared singletons (Convert, Inspect, Filters, Upscale, Trim, Audio, Subtitles) gets resume for free.
- Trim module retains its independent `ResumeState` (created with empty configDir) for backward compatibility â€” no functional overlap since it uses a separate state file.

### P1-1: Network/URL Streaming

- **`Engine.OpenURL(url string, opts map[string]string) error`** added to `internal/media/engine.go`: creates AVDictionary with `timeout=60000000` (60s), `reconnect_streamed=1`, `reconnect_on_network_error=1`, `reconnect_delay_max=5`. User-provided `opts` merge over defaults (caller wins). Passes `&dict` to `avformat_open_input` which enables any FFmpeg-supported network protocol (HTTP, HTTPS, HLS, DASH, RTSP, RTMP, RTMPS, MMS, TCP, UDP) without format hacks.
- **`InlineVideoPlayer.LoadURL(url string, opts map[string]string) error`** added to `internal/ui/inline_player.go`: delegates to `loadViaOpen(url, func(eng) error { return eng.OpenURL(url, opts) })`. Stub in `inline_player_stub.go` returns nil for non-native builds.
- **AVDictionary cleanup** â€” deferred `C.av_dict_free(&dict)` covers all paths including `avformat_open_input` failure (FFmpeg consumes the dict on success; our defer is a no-op in that case).
- Builds clean with `CGO_LDFLAGS_ALLOW`.

### P0-5: OpenAuto â€” Openâ†’OpenDVD Fallback

- **`Engine.OpenAuto(path string) error`** added to `internal/media/engine.go`: calls `Open(path)` first; on failure retries with `OpenDVD(path, 0)` (title 0 = longest/main-feature title). `avformat_open_input` sets `formatCtx` to NULL on failure so no cleanup is needed before the retry.
- **`InlineVideoPlayer.Load()`** now uses `eng.OpenAuto(path)` instead of `eng.Open(path)`. All modules that call `Load()` (Convert, Filters, Upscale, Inspect, Trim, Audio, Subtitles) transparently handle ISOs and VIDEO_TS directories. `LoadDVD(path, title)` remains for explicit title-numbered disc access.

### P0-1 + P0-2: HWâ†’SW Decoder Degradation + NextFrame Hang Fix

- **`vt_clear_hw_decode` C helper** added to `errors.go` CGo preamble: unrefs and NULLs `hw_device_ctx` from the codec context, resets `get_format` callback and `opaque` to NULL (so FFmpeg won't re-negotiate HW pixel format on the next flush+decode cycle), and re-enables `FF_THREAD_SLICE` threading that was disabled for HW compatibility.
- **`DegradeToSoftware()` completed**: Previously freed HW contexts but left `hw_device_ctx` on the codec context and the `get_format` callback wired, meaning the codec would try to re-init HW on the next packet. Now calls `vt_clear_hw_decode` (breaks the re-init cycle) and `avcodec_flush_buffers` (discards buffered HW frames, prevents use-after-free when the decode loop continues).
- **`videoDecodeLoop` degradation path** â€” when `retrieveHWFrame` sets `videoDecodeDead=true` (SEH in `av_hwframe_transfer_data` or `sws_scale`):
  - If not yet degraded (`!e.hwDegraded`): calls `RecordHWFailure()`, calls `DegradeToSoftware()` synchronously (safe â€” outside `videoCodecMu` at this point), clears `videoDecodeDead`, continues the loop. Next iteration: `hwDevice == HWDeviceNone`, takes the SW decode branch directly.
  - If already degraded: SW decode is also failing; sends `decodeEOFPTS` EOF sentinel and returns.
- **P0-2 EOF sentinel on all fatal return paths** â€” three paths in `videoDecodeLoop` that previously returned without signalling:
  - `SafeSendPacket` SEH exception: `videoDecodeDead=true` + EOF sentinel + return
  - `SafeReceiveFrame` SEH exception: `videoDecodeDead=true` + EOF sentinel + return
  - Already-degraded fatal path: EOF sentinel + return
  `NextFrame` unblocks and returns `io.EOF` instead of hanging forever at `df = <-e.frameQueue`.
- Build clean.

### P0-5: OpenAuto with Openâ†’OpenDVD Fallback

- **`Engine.OpenAuto(path string) error`** added to `internal/media/engine.go`: attempts `Open(path)` first; if it fails, retries with `OpenDVD(path, 0)` (title 0 = longest title). This lets ISOs and VIDEO_TS directories load seamlessly through the same `Load()` call as regular files.
- **`InlineVideoPlayer.Load(path)`** now calls `OpenAuto` instead of `Open` directly. All modules that use `Load()` (Convert, Trim, Inspect, Filters, Upscale, Audio, Subtitles) can now open disc structures without calling `LoadDVD()` explicitly.

### P0-3: Backward Frame Stepping (un-break StepFrame(-1))

- **`Engine.Step(frames int)`** at `playback.go:335-338` previously rejected `frames <= 0` with `"invalid frame count"` â€” all `StepFrame(-1)` callers (main.go, convert_player_native.go, trim/view.go) silently failed to step backward.
- **Implemented true backward step**: When `frames < 0`, calculates the target PTS via `CurrentTime() - abs(frames) * frameDur`, seeks ~2 seconds before that position (to land at a keyframe before target), then decodes forward `abs(frames)` frames via `NextFrame()` and returns the last one. Falls back to position 0 if below start.
- **Frame rate awareness**: Uses `Engine.GetFrameRate()` to compute `frameDur = 1/fps`. Returns error if frame rate is unknown (cannot calculate backward step without it).
- All existing media tests pass (2 pre-existing ASS subtitle failures unaffected).

### P0-4: Error Ring Buffer (replaces single-slot lastError)

- **Replaced `lastError *PlaybackError`** (single slot, written only in dead code, never read) with a 16-entry ring buffer (`errorRing [16]ErrorRecord` + `errorRingNext int`) in `internal/media/errors.go`.
- **Added `ErrorRecord` struct** with `Timestamp time.Time`, `Code`, `Message`, `Retry` fields â€” each entry captures when the error occurred.
- **Added `SetError(code, message, retry)`** â€” thread-safe method that writes into the ring buffer at the next slot (wrapping around). Uses a dedicated `errorMu sync.Mutex` independent of the engine lock hierarchy.
- **Wired `SetError` into all SEH catch paths:**
  - `GrabFrame` â€” `avcodec_send_packet` / `avcodec_receive_frame` SEH exceptions
  - `videoDecodeLoop` â€” `avcodec_send_packet` / `avcodec_receive_frame` SEH exceptions
  - `retrieveHWFrame` â€” `av_hwframe_transfer_data` / `sws_scale` SEH exceptions
  - `DegradeToSoftware()` â€” HWâ†’SW fallback event (previously assigned `lastError` directly)
- **`GetLastError()`** preserved as backward-compat: returns most recent record as `*PlaybackError` (or nil if empty).
- **`ClearError()` / `ClearErrorHistory()`** reset the ring buffer.
- **`GetErrorHistory() []ErrorRecord`** returns all entries in chronological order, oldest first.
- Build verified, all media package tests pass (2 pre-existing ASS subtitle failures unaffected).

### Comprehensive Media Engine Gap Analysis

- **Full audit of every missing player feature** conducted against `internal/media/` â€” catalogued 20+ gaps across 4 phases: Critical Stability (5 items), Player Completeness (11 items), ISO Engine (3 items), Polish & Diagnostics (4 items).
- **Dead code confirmed**: `DegradeToSoftware()`, `ShouldDegrade()`, `RecordHWFailure()`, `ResetHWFailureCount()` (`errors.go:57-111`) â€” all defined, **none called anywhere**. HWâ†’SW fallback is inline per-frame, retrying HW on every decode until `videoDecodeDead=true` permanently kills all decoding. No graceful degrade path exists.
- **NextFrame hang confirmed**: After SEH in `videoDecodeLoop`, remaining frame queue drains then `NextFrame` blocks forever at `<-e.frameQueue` because no EOF sentinel is sent.
- **Backward frame stepping broken**: `Step(frames int)` rejects `frames <= 0` â€” all callers pass `-1` and silently fail.
- **Error history orphaned**: `lastError` is a single `*PlaybackError` pointer, written only in dead code, read from nowhere. `GetLastError()`/`ClearError()` never called outside definition.
- **No network streaming**: `avformat_open_input` gets `nil, nil` â€” no timeout, reconnect, or protocol whitelist options. FFmpeg supports HTTP/HTTPS/HLS/DASH/RTSP/RTMP natively but VT exposes no URL opening path.
- **SeekAccuracy locked to Keyframe**: Frame and Accurate modes defined but unreachable â€” all callers hardcode `SeekAccuracyKeyframe`.
- **Only Trim module has resume**: `internal/media/state/resume.go` used exclusively by `trim/view.go` â€” no other module persists position.
- **No audio delay, speed+pitch, A-B loop, bilinear scaling, frame timing overlay, clock drift correction, or growing-file support**.
- **Design document created**: `docs/MEDIA_ENGINE_GAP_ANALYSIS.md` with full tier list, file:line references, effort estimates, and comparison to VLC/MPV handling.

---

## Version 0.1.1-dev49 (complete)

### Select / Dropdown â€” Active Item Text Colour Fix
- **Active menu item text colour fixed** (`_fyne/widget/menu_item.go`): `refreshText()` now uses `ColorNameForegroundOnPrimary` when the item is active (sitting on a `ColorNameFocus`/VT_Green background). `VTTheme` maps `ForegroundOnPrimary` to `BgBase` (#0B0F1A, near-black), giving high contrast against VT_Green (#22c55e). Previously `ColorNameForeground` (light #E1EEFF) was always used, making active dropdown rows illegible regardless of background colour.

### Engine-Level bwdif Deinterlace
- **`internal/media/deinterlace.go`** (new): libavfilter-based bwdif filter graph. `create_bwdif_filter()` allocates `[buffersrc â†’ bwdif=mode=0:parity=-1:deint=0 â†’ buffersink]`. `run_bwdif()` pushes frames with `AV_BUFFERSRC_FLAG_KEEP_REF` (no data steal) and pulls deinterlaced output. `frame_is_interlaced()` uses the portable `AV_FRAME_FLAG_INTERLACED` flag check (works on FFmpeg 7.x+ where `interlaced_frame` field was removed).
- **`Engine` struct**: Added `deinterlaceEnabled` (default `true`), `deintFilterGraph`, `deintBuffersrc`, `deintBuffersink` fields. `SetDeinterlaceEnabled()`/`IsDeinterlaceEnabled()` getter/setter with mutex guard. `applyDeinterlace()` lazily creates filter graph on first use; returns `*C.AVFrame` caller must free. `freeDeinterlaceFilter()` called from `Close()`.
- **`toRGBA()` signature**: Now `toRGBA(src *C.AVFrame)` â€” `nil` means use `e.frame`. Deinterlace produces a separate frame; passed directly to `SafeSwsScaleFrame`, then freed.
- **Integration points**: Applied in `videoDecodeLoop` SW path (both normal decode and HW fallback) and `GrabFrame` SW path. Gated on `e.deinterlaceEnabled && isFrameInterlaced(e.frame)`.
- **Global default**: `media.SetDefaultDeinterlaceEnabled()` / `GetDefaultDeinterlaceEnabled()` â€” `NewEngine` reads this so all created engines respect the user's saved preference.
- **Settings UI**: `AutoDeinterlace` field in `PrefsConfig`, `PreferencesCallbacks` interface methods, `settings_module.go` adapter, `tabs.go` Player section checkbox with `SettingsAutoDeinterlace` / `SettingsAutoDeinterlaceHint` i18n keys.
- **Bridge**: `setAutoDeinterlace()` in `native_media.go` sets the global default AND updates all running player engines. `autoDeinterlaceEnabled()` reads the global default. `initNativeMediaAssets()` applies pref at startup.
- **disc_debug.c double-include fix**: Removed `#include "disc_debug.c"` from preamble â€” CGo auto-compiles `.c` separately, causing duplicate symbols at link time. Added `#include <stdlib.h>` for `C.free`.

### Media Engine â€” Seek Corruption Fix & Player Consolidation
- **Seek corruption root cause found & fixed**: `Engine.Seek()` accurate fallback used `AVSEEK_FLAG_ACCURATE` without `AVSEEK_FLAG_BACKWARD`, landing the format context mid-GOP. `avcodec_flush_buffers` destroyed decoder reference state, causing the first P/B-frame after seek to produce garbage. Fixed by adding `AVSEEK_FLAG_BACKWARD` so the fallback lands at the keyframe immediately before the target.
- **Verbose seek logging added**: Human-readable seek flags, accurate fallback confirmation with position, clock reset target (including audio offset), stale frame queue drain count, seekGen change detection in videoDecodeLoop with frame format/dimensions/PTS logging, InlineVideoPlayer level seek completion logging.
- **Player singleton consolidation**: All 10 per-module `InlineVideoPlayer` singletons (`convertInlinePlayer`, `trimInlinePlayer`, `inspectInlinePlayer`, `filtersInlinePlayer`, `upscaleInlinePlayer`, etc.) consolidated into 2 shared instances: `GetPrimaryPlayer()` and `GetPreviewPlayer()`. Same absolute singleton count (2 engine instances), all modules share the same primary player. Eliminates per-module player state fragmentation.
- **Legacy getters forward**: `GetConvertPlayer()`, `GetTrimPlayer()`, `GetInspectPlayer()`, etc. now forward to `GetPrimaryPlayer()` / `GetPreviewPlayer()`. Callers can migrate gradually.
- **Architecture doc created**: `docs/MEDIA_ENGINE_ARCHITECTURE.md` documents the full three-layer stack, the 10-player problem, seek architecture with the fix, frame pacing design, known issues, and the consolidation plan.

### Rip Module â€” Menu Bleed, Chapters & Multi-Title Export
- **Menu VOB bleed fixed**: `CollectVOBSets` now excludes `VTS_XX_0.VOB` (menu VOB) from content title sets. Previously the menu VOB was concatenated at the rip start, causing menu frames to glitch into the video and shifting chapter timestamps.
- **Chapter diagnostics**: Added verbose logging of chapter count, first/last timestamp, and embed/no-embed decision to rip log for easier debugging.
- **Menu preservation option**: `IncludeMenus` config option + checkbox in rip view. Menu VOBs (`VIDEO_TS.VOB` + `VTS_XX_0.VOB`) exported as separate files alongside the main rip. Uses same format as the rip encoder.
- **Main/extra title naming**: The longest title (main feature) now gets the main output path; shorter titles use `_Extra_Title_NN` suffix. Title selection UI marks the main feature with a star (â˜…) indicator.
- **Design doc**: `docs/RIP_MODULE_REDESIGN.md` documents the redesign and root cause analysis.

### VT Media Engine â€” Subsystem Split (engine.go)
- **errors.go extracted** â€” PlaybackError type, ErrCode constants, error/degradation methods moved from engine.go to errors.go. C preamble duplicated for `av_buffer_unref` calls.
- **hwdecode.go extracted** â€” HWDeviceType, device detection, SetHWDevice/GetHWDevice, codecCanUseHWDevice, initHWDecode, getHWPixelFormat, retrieveHWFrame, C helper functions vt_get_hw_format/vt_set_get_format moved from engine.go to hwdecode.go.
- **framepool.go extracted** â€” Frame buffer pool fields + toRGBA, ReleaseFrame, GetFramePoolSize, ensureSwsCtx moved to framepool.go. SwScale context creation separated from decode loop.
- **subtitle_engine.go extracted** â€” SubtitleOverlay type + Bounds, initSubtitleDecoder, decodeSubtitle, RenderSubtitles, drawSubtitleText, drawBitmapText, isCharPixel moved to subtitle_engine.go. C preamble macros vt_sub_rect0/vt_sub_rect_type duplicated.
- **buffer.go extracted** â€” Buffer mode management, decode time tracking, adaptive buffer adjustment, buffer health reporting moved to buffer.go. Pure-Go file (no C preamble needed).
- **playback.go extracted** â€” Entire playback pipeline (Start, demuxerLoop, Seek, ResetAfterGrab, Step, GrabFrame, sendToFrameQueue, videoDecodeLoop, NextFrame, Pause, Resume, TogglePause, DrainAudio, WaitForFrame, Close) plus query helpers (Duration, CurrentTime, GetLastVideoPTS, GetLastAudioPTS, IsRunning, QueueStats) moved to playback.go. engine.go reduced from 3245â†’1117 lines.

### Inuktitut Transliteration â€” Auto-Fill i18n Script Variants
- **`internal/i18n/translit/` package**: Pure-Go reimplementation of the iutools syllabicsâ†”roman transliteration algorithm (MIT-licensed, National Research Council Canada). 270-entry mapping tables cover all 15 consonant series (p, t, k, g, m, n, s, l, j, v, r, q, ng, nng, Å‚) plus short/long vowels, finals, and `lh` ASCII fallback for `Å‚`.
- **Syllabicsâ†’Roman direction**: Handles compound sequences where r/q finals combine with k-series consonants (â†’ rq/qq+vowel) and ng/nng finals combine with g-series consonants (â†’ ng/nng+vowel).
- **Romanâ†’Syllabics direction**: Greedy longest-prefix matching using a single `map[string]string` hash table. Tries 5-char keys (e.g. `nngaa`) before 3-char (`nng`) or 2-char (`ng`) â€” no state machine needed. Format verbs (`%s`, `%d`, etc.) escaped before lookup so the `s` in `%s` is not mapped to á”….
- **Script detection**: `IsSyllabics(s)`, `SyllabicRatio(s)` for identifying script type; `RomanOnly(v)` mode toggle to skip romanâ†’syllabics on translatable strings.
- **i18n integration**: `translitFill()` in `i18n.go` uses reflection to iterate `Strings` struct fields. Empty `iu` fields auto-filled from `iu-latn` (and vice versa) via transliteration during `SetLanguageWithScript`. Manually-entered strings in `iu.go` take precedence as overrides â€” never overwritten.
- **Test coverage**: 15 translit unit tests with 276 round-trip assertions; 3 integration tests for the `i18n.SetLanguageWithScript` â†” translit bridge. All passing.

### Thread Safety Formalisation â€” Lock Hierarchy & Lockdep
- **Formal lock hierarchy established**: `mu (level 1) â†’ formatMu (level 2) â†’ videoCodecMu (level 3) â†’ framepoolMu (level 4)` with clear rules documented in `internal/media/lock.go` and `docs/PLAYER_DEBUG.md`. Lock-free `atomic.Uint64` fields (`seekFlushBefore`, `seekGen`, `lastVideoPTSBits`) documented as the mechanism that avoids `videoCodecMu â†’ mu` reverse-order deadlock.
- **Lock wrapper helpers**: Created `lock.go` with `lockMu()`/`unlockMu()`, `lockFormatMu()`/`unlockFormatMu()`, `lockVideoCodecMu()`/`unlockVideoCodecMu()`, `lockFramepoolMu()`/`unlockFramepoolMu()` helpers + `acquired()`/`released()` hook methods. All ~60 direct `.Lock()`/`.Unlock()` calls across engine.go, playback.go, errors.go, buffer.go, framepool.go, and scrub.go replaced with named helpers.
- **Lockdep runtime verification**: `lockdep_on.go` (`//go:build lockdep`) tracks lock acquisitions per goroutine via `sync.Map` + `runtime.Stack` goroutine ID parsing. Panics on reverse-order acquisition with diagnostic message. `lockdep_off.go` provides no-op stubs for release builds.
- **DegradeToSoftware**: Safety comment added documenting that it acquires `mu â†’ videoCodecMu` (correct order) and must not be called from within videoCodecMu. Guidance to use `go e.DegradeToSoftware()` if wired in future.
- **framepoolMu comment**: Engine struct field comment updated from "must NOT be acquired under videoCodecMu" to reference level-4 hierarchy position â€” the comment was wrong (toRGBA does hold videoCodecMu while acquiring framepoolMu), but the hierarchy is safe because no reverse path exists.
- **Lock hierarchy documentation**: Added to `docs/PLAYER_DEBUG.md` with level table, acquisition rules, the `Close()` exception, and lockdep build instructions.

### Frame Pacing â€” PTS-Driven Timing Overhaul
- **No-audio WaitForPTS**: Replaced `e.clock.SetTime(pts)` with `e.clock.WaitForPTS(pts)` in `NextFrame` for the no-audio path. Previously the clock was instantly snapped to each frame's PTS, erasing wall-time pacing and causing frames to be displayed at decode speed. Now the clock ticks forward in real time and `WaitForPTS` blocks until the correct PTS interval elapses, giving proper frame-duration spacing (e.g. 41.7ms for 24fps).
- **Removed WaitVsync from playbackLoop**: The `DwmFlush()` call after every `NextFrame` introduced 0-16.7ms of random jitter because the vsync phase varies per frame. With audio, the displayed interval became `frame_period + Î”V` (Î”V up to Â±16.7ms), a Â±40% variation at 24fps. Removing it eliminated all vsync-induced jitter; frame timing is now purely PTS-driven via `WaitForPTS`.
- **Frame rate propagation**: `v.player.SetFrameRate(eng.GetFrameRate())` added to `loadViaOpen` ready callback so the `VideoPlayer` always knows the source frame rate for frame-step calculations and display configuration.

### Settings â€” Log File Management
- **Session rotation**: `logging.Init()` writes a `=== VideoTools session started` marker on each boot; `rotateLog()` trims everything before the most recent previous session so the file never accumulates more than 2 sessions. Prevents stale binary noise from old installations polluting current diagnostics.
- **Clear Log File**: Settings â†’ Preferences â†’ Log File â€” truncates the file in-place (no restart), writes a cleared-at header. Confirmation dialog prevents accidental wipes.
- **Open Log Folder**: Reveals the logs directory in the system file manager.

### Queue & Process Management â€” File-in-Use & Zombie FFmpeg Fixes
- **`NoInheritHandles` on Windows subprocess creation**: `internal/utils/exec_windows.go` sets `NoInheritHandles: true` in `SysProcAttr` for both `CreateCommand` and `CreateCommandRaw`. The VT media engine's `avformat_open_input` file handles were inherited by FFmpeg child processes, holding the source video locked on Windows â€” preventing delete/move after conversion. `NoInheritHandles: true` closes the inheritance path entirely.
- **`Queue.Stop()` cancels running job**: `Stop()` now calls `cancelRunningLocked()` before clearing `q.running`. Previously, clean app shutdown left the active encode running as an orphan. Context cancellation now propagates through `exec.CommandContext` to terminate the child process.
- **Windows Job Object â€” crash-safe process cleanup**: `internal/utils/jobobject_windows.go` creates a global Job Object with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` at app startup via `utils.InitJobObject()`. All long-running encode processes are assigned to it immediately after start via `utils.StartCmd()`. When VT exits for any reason â€” including a crash â€” Windows automatically kills all Job Object members. Covers 4K crash scenarios where `Queue.Stop()` is never reached.
- **Linux `Pdeathsig: SIGKILL`**: `internal/utils/exec_linux.go` sets `SysProcAttr.Pdeathsig = syscall.SIGKILL` on all `CreateCommand`/`CreateCommandRaw` calls. Kernel sends SIGKILL to every child process when VT exits, no cooperative shutdown required.
- **`utils.StartCmd()` at all encode sites**: `main.go`, `rip/executor.go`, `audio/executor.go`, `thumbnail/generator.go`, `interlace/detector.go` all use `StartCmd` in place of `cmd.Start()` for long-running processes. On Windows this assigns each child to the Job Object; on Linux `Pdeathsig` in `SysProcAttr` covers it from `CreateCommand`.

### Queue Module â€” Convert Navigation & Progress Refresh Bug Fixes
- **Blocking dialog removed**: `convertNow()` replaced `dialog.ShowInformation` with `s.showQueue()`. User is taken directly to the queue after adding a job; no modal to dismiss.
- **Auto-refresh goroutine self-exit fixed**: `startQueueAutoRefresh` and `startQueueElapsedTicker` (queue_module.go) used `return` in the ticker case when `s.active != "queue"`, killing the goroutines while leaving `queueAutoRefreshRunning`/`queueElapsedRunning = true`. Next `showQueue()` call did nothing (guard check). Changed both `return` to `continue` so goroutines remain alive and simply skip work while not on the queue view.

### Convert Module â€” Collapsible Section Header Bars (BuildCollapsibleHeader)
- **`internal/ui/collapsible.go`**: `tappableBox` widget + `BuildCollapsibleHeader` function. Full-width module-colored accent bar (CornerRadius 10, h=34) with â–¼/â–¶ uppercase label, matching `buildConvertBox`/`buildRipBox` visual language. Accepts `extraRight` widgets and `onToggle(open bool)`.
- **Metadata panel**: Tappable header bar labeled "â–¼ METADATA" / "â–¶ METADATA". Copy/Clear buttons remain in the header right side. Drives `leftColumn` VSplit 0.5â†”0.97.
- **Settings panel**: Tappable header bar labeled "â–¼ SETTINGS" / "â–¶ SETTINGS". Drives `mainSplit` 0.65â†”0.97. Removes old pill button from top nav bar.
- **`ConvertSectionSettings` i18n**: New key in all four locale files.
- **Rip log toggle**: Relabeled "â–¼ LOG" / "â–¶ LOG" (was bare â–¼/â–¶).

### Rip Module â€” Layout Alignment to Convert Style
- **Player panel width**: HSplit offset 0.40 â†’ 0.65; player takes two-thirds of module width, matching the Convert module.
- **Section boxes**: Controls panel restructured with `buildRipBox()` header sections (teal accent bars) â€” four sections: Source, Format, Output, Status.
- **Open in Player relocated**: Moved from below the player canvas into the footer action bar.
- **Collapsible log**: â–¼/â–¶ toggle in the RIP LOG header collapses/expands the log (expanded 0.60, collapsed 0.97); overall VSplit corrected from 0.75 â†’ 0.60.

### Rip Module â€” Source Section Rework
- **Disc info moved to Source section**: The `discInfoLabel` (showing disc type/region/size) was relocated from the Format box to the Source section, giving users immediate access to basic disc metadata under the source path entry.
- **Single browse button**: The ISO... and Folder... buttons were replaced with a single `...` button that opens a 900Ã—640 file dialog for selecting ISO files. Folder selection is handled via the existing drag-and-drop droppable area on the source entry.
- **Format validation**: `loadDisc` now validates that the provided path is either an `.iso` file or contains a `VIDEO_TS` directory. Non-disc files (e.g. `.mkv`) are rejected with a user-visible error message (`RipErrNotDisc`) shown in the disc info label, replacing the previous silent failure.

### Player â€” Configurable Idle Aspect Ratio
- **Settings â†’ Preferences**: New "Idle Aspect Ratio" dropdown with options 4:3 (Standard), 16:9 (Widescreen), 5:3, 21:9 (Ultrawide), and 9:16 (Portrait). Persisted as `PlayerDefaultAspect` in `PrefsConfig` JSON.
- **VideoPlayer field**: Added `idleAspectRatio` field with `SetIdleAspectRatio()` / `IdleAspectRatio()` accessors. The `draw()` method now calls `v.IdleAspectRatio()` instead of the hardcoded `4.0/3.0`.
- **InlineVideoPlayer forwarding**: `SetIdleAspectRatio` method on `InlineVideoPlayer` forwards to the underlying `VideoPlayer`. A new `applyPlayerDefaultAspect()` function in `native_media.go` iterates all player singletons on startup and on pref change.
- **i18n**: 8 new string keys (SettingsPlayerAspect/Hint/4x3/16x9/5x3/21x9/9x16) in all four locale files.

### C Disc Debug Utility
- **New `internal/media/disc_debug.c`**: C-level debug tools for probing DVD/VIDEO_TS file systems. `read_file_hex` opens and dumps raw IFO bytes; `list_directory` enumerates files via `FindFirstFileA` (Windows) or `opendir` (POSIX); `dir_stat` counts files and sums sizes.
- **Go wrappers** (`disc_debug.go`): `DiscDebugHexDump(path, maxBytes)`, `DiscDebugListDir(dirPath, maxEntries)`, `DiscDebugDirStat(dirPath)` exposed to Go via CGo.
- **Stubs** (`disc_debug_stub.go`): No-op implementations for `!native_media` builds.
- **Build-gated**: `//go:build native_media` on all C and Go files; no new dependencies.

### VT ISO Engine â€” Roadmap
- **Roadmap columns** â€” VT Media Engine and VT ISO Engine added as dedicated columns on the interactive roadmap with individual status cards for each refactoring task (engine.go split, view.go split, Player interface, HW decode, thread safety, UDF reader, UDF thread safety).

## Version 0.1.1-dev48 (shipped)

### Theme System â€” internal/theme/ Package
- **VT_Navy colour palette** â€” `internal/theme/palette.go` defines BgBase/BgDark/BgLight/BgCard, Border/BorderDim, Text/TextMuted, InputBg, and all status colours (Green, Teal, Yellow, Blue, Orange, Purple, Magenta).
- **PillButton widget** â€” `internal/theme/pillbutton.go` â€” pill-shaped button with coloured border, hover/active/disabled states, bold text, initial-paint fix (`r.Refresh()` in CreateRenderer).
- **PillIconButton widget** â€” `internal/theme/pilliconbutton.go` â€” square icon-only pill button for transport controls. Supports fyne.Resource icon, hover lightens border.
- **Text primitives** â€” `internal/theme/text.go` â€” `NewTitleLabel` (Monospace+Bold 24pt), `NewSectionLabel` (Bold), `NewWrappingLabel` (word wrap), `NewHintLabel` (Italic), `NewMonoLabel` (Monospace).
- **MonoTheme updated** â€” references theme vars (InputBg) instead of hardcoded `#344256`.
- **main.go colours** â€” `backgroundColor`/`gridColor`/`textColor`/`queueColor` reference `ui.BgBase`/`ui.BorderDim`/`ui.Text`/`ui.Magenta`.
- **Circular dep resolved** â€” `ui/` re-exports theme symbols; `media/` imports theme directly. `ui` no longer defines palette/primitives â€” all source of truth is `internal/theme/`.

### Player Transport Controls Migration
- **speedBtn/subtitleBtn** â€” `widget.NewButton` â†’ `theme.NewPillButton`. Displays "1x"/speed and "CC" labels.
- **playBtn/volumeBtn/prevChapterBtn/nextChapterBtn/fullscreenBtn/pipBtn** â€” `widget.NewButtonWithIcon` â†’ `theme.NewPillIconButton`. Icon-only buttons with consistent pill styling.
- All transport buttons share navy-light background (`BgLight`), hover border lightens to `TextMuted`.

### Bug Fixes
- **Audio nil-widget crash** â€” `Player.Widget()` nil guard; falls back to SMPTE placeholder so HSplit never receives nil.
- **PillButton initial paint** â€” `CreateRenderer` calls `r.Refresh()` so colours are set before first paint (buttons were invisible until hover triggered a Refresh cycle).
- **Window recentering removed** â€” `CenterOnScreen()` removed from `maximizeWindow`; window stays where user placed it.
- **i18n script persistence** â€” `SetLanguageWithScript("iu", ScriptLatin)` stores preference; future `SetLanguage("iu")` restores it. `ScriptPrefs` map in locale JSON survives app restarts.

### Full Module Button + Slider Migration
- **All module-level `widget.Button` calls migrated** â€” compare, audio, rip, filters, upscale, subtitles, trim, thumbnail, queueview, settings, benchmarkview, main.go all use `ui.MakePillButton` / `ui.MakePillIconButton`. `NewPillButton` â†’ `MakePillButton`; `NewPillIconButton` â†’ `MakePillIconButton` (sitewide rename).
- **VTSlider / VTProgressBar** â€” `widget.Slider` and `widget.ProgressBar` replaced sitewide with `ui.Slider` / `ui.MakeSlider`. Styled to match VT_Navy theme.
- **Queue + Benchmark header/footer alignment** â€” Both modules now use `TintedBar` header, `NewTitleLabel`, `accentColor` button tinting, and the shared statsBar footer â€” consistent with all other modules.

### Player: STATUS_STACK_OVERFLOW Recovery
- **VEH extended to catch stack overflow** â€” `safe_bridge.c` MinGW VEH handler now catches `STATUS_STACK_OVERFLOW` (0xC00000FD) in addition to `EXCEPTION_ACCESS_VIOLATION`. `_resetstkoflw()` restores the guard page before `longjmp` to give the handler enough stack to execute.
- **New sentinel `SAFE_BRIDGE_STACK_OVERFLOW` (0xDEAD0003)** â€” Go callers can distinguish stack overflow from access violation in the `exc_code_out` return value.
- **PE default thread stack raised to 4 MB** â€” `-Wl,--stack,4194304` in `#cgo windows LDFLAGS`. `CGO_LDFLAGS_ALLOW=-Wl,--stack,.*` added to CI YAML and `scripts/windows/build.ps1` to pass Go's CGO security scanner.

### Player: Dual Before/After Sync
- **`InlineVideoPlayer.SetPeer(peer)`** â€” designates a follower player that mirrors every `Play`, `Pause`, and `Seek`; disables the follower's built-in controls so only the primary transport bar drives both. Peer calls are non-blocking (`go peer.Method()`).
- **Filters + Upscale wired** â€” `filtersInlinePlayer.SetPeer(filtersPreviewPlayer)` and `upscaleInlinePlayer.SetPeer(upscalePreviewPlayer)` called in `native_media.go` `init()`.
- **Preview players muted** â€” `SetMuted(true)` called on both preview players after load; audio plays from primary only.

### CI & Diagnostics
- **Windows SignPath signing** â€” `SIGNPATH_API_TOKEN` + `SIGNPATH_ORGANIZATION_ID` set in Forgejo secrets. ci-build.ps1 calls sign-exe.ps1 non-fatally.
- **Cache guard** â€” require `ffmpeg.exe` present on disk before skipping BtbN download on cache hit.
- **ci-build.ps1 encoding** â€” UTF-8 em dashes replaced with ASCII `--`.
- **VT_STARTUP_DEBUG** â€” env var gates per-widget CreateRenderer tracing to stderr. `logging.Sync()` force-flushes at crash-risk checkpoints. Confirmed: STATUS_STACK_OVERFLOW is glfw.CreateWindow() GPU driver DLL injection.
- **Windows CI FFmpeg shared cache** â€” `actions/cache` for `C:\ffmpeg-static` (BtbN shared DLL bundle) in the MSIX workflow; both CI pipelines now skip FFmpeg download on cache hit.

### Roadmap Visual Polish
- **Deprecated status** â€” purple (`#a855f7`), no strikethrough.
- **Cycle filter row** â€” dynamically built from roadmap data, sorted newest-first; current version gets green outline.
- **Testing Checklist modal** â€” pass/fail/untested per item, localStorage, grouped by module.
- **Column reorder** â€” empty columns drift right.
- **Back-to-top button** â€” appears at 300px scroll.
- **VT logo** â€” 96px, right-aligned, `align-items: flex-start`.
- **Modal drag-to-scroll** â€” changelog/checklist click-and-drag vertical scrolling.
- **Colour dots** â€” 16px circles across legend, status filter, module filter buttons.
- **Future status** â€” grey â†’ orange (`#f97316`).

### Button Straggler Migrations (dev48 close)
- **About dialog** â€” `widget.NewButton` â†’ `MakePillButton` (Logs Link, Open) in `internal/app/modules/about/dialog.go`.
- **Compare fullscreen** â€” backBtn `widget.NewButton` â†’ `MakePillButton` in both `fullscreen_native.go` and `fullscreen_stub.go`.
- **Settings tabs** â€” testPatternBtn + refreshBtn `widget.NewButton` â†’ `MakePillButton` in `settings/tabs.go`.
- **Command Editor** â€” All 7 `widget.NewButton[WithIcon]` â†’ text-only `MakePillButton` in `internal/ui/command_editor.go`. Struct fields changed from `*widget.Button` to `*PillButton`. Unused `theme` import removed.
- **MakeIconButton** â€” `internal/utils/utils.go` return type `*widget.Button` â†’ `fyne.CanvasObject`; body reverted to `widget.NewButton` to avoid `utils â†’ ui â†’ benchmark â†’ utils` import cycle.
- **Transport icon buttons (known remaining)** â€” 18 `widget.NewButtonWithIcon` icon-only transport controls in `convert_player_native.go` + `main.go` remain unconverted â€” PillIconButton lacks dynamic `SetIcon()` needed for playâ†”pause switching.

## Version 0.1.1-dev47 (complete)

### PALâ†’NTSC Full-Disc Conversion Pipeline â€” Stages 1-3 (HIGH)

- **Full-disc extraction mode** â€” New "Full disc extraction (DVD-Video with IFO regeneration)" checkbox in Rip module enrichment options. When enabled with a region conversion (PALâ†’NTSC or NTSCâ†’PAL), ALL VTS sets are extracted and VIDEO_TS.VOB menu is processed, producing a complete VIDEO_TS directory.
- **DVD-compliant MPEG-2 encoding** â€” `convertVOBWithRegion()` re-encodes each VTS set to MPEG-2 video + AC-3 audio with the region conversion filter chain (yadif deinterlace, scale, fps, atempo).
- **Menu VOB support** â€” `CollectMenuVOB()` gathers VIDEO_TS.VOB for conversion alongside title VOBs.
- **IFO/BUP regeneration (Stage 3)** â€” New `RegenerateIFOs()` function reads the original IFO structure and generates new VTS and VMG IFO files with correct NTSC/PAL video attributes, PGC playback timing, TMAPT sector maps, and chapter tables.
- **New files:** `internal/app/modules/rip/ifo_regen.go` â€” `RegenerateIFOs()`, `convertedVTS`, `readVTSMAT`, `readVMGMAT`, `buildVTSMat`, `buildVTSPGC`, `buildTTSRPT`

## Version 0.1.1-dev46 (complete)

---

## Version 0.1.1-dev45 (complete) - UI Polish & Parity

### Convert Module Improvements - Phase 1 (HIGH)
- **Audio Sample Rate dropdown** â€” `audioSampleRateSelect` wired in buildConvertView
- **Normalize Audio checkbox** â€” `normalizeAudioCheck` + LUFS/TruePeak sliders wired
- **Deinterlace Mode dropdown** â€” `deinterlaceModeSelect` + `deinterlaceMethodSelect` wired
- **H.264 Profile/Level controls** â€” `h264ProfileSelect` / `h264LevelSelect` wired; shown when H.264 codec is active

### Convert Module i18n (HIGH - Issue #5)
- **~42 hardcoded strings** i18n'd: checkboxes, buttons, dialog messages, back button
- **New keys added** to `internal/i18n/strings.go`, `en_ca.go`, `fr_ca.go`, `iu.go`, `iu_latin.go`

### Convert Module Improvements - Phase 2 (HIGH)
- **One-click presets** â€” Hobbyist SDâ†’HD, Semi-Pro 1080pâ†’4K, Anime, Restoration, Social Media workflows
- **UI clarity** â€” Preset dropdown with description labels, clear AI+RIFE workflow
- **Detection reliability** â€” VerifyTool() checks PATH + app-local bin + smoke test
- **Optimization guide** â€” See `docs/UPSCALE_OPTIMIZATION.md` for hobbyist/semi-pro workflows
- **Hardware acceleration** â€” Sync upscale HW accel from master setting
- **Filters module HW accel** â€” Add hardware acceleration dropdown

### Audio Module Phase 2 (HIGH)
- **InlineVideoPlayer** â€” Add player singleton like Convert
- **Video preview pane** â€” Same layout pattern as Convert
- **SMPTE bars idle state** â€” "DROP VIDEO TO LOAD"

### Audio Module Phase 3 (MEDIUM) - done dev45
- **Enhanced track list** â€” Codec colors, language flags, duration display
- **Output naming preview** â€” Shows filename before extraction
- **Track reordering** â€” Up/down buttons (UI ready, logic wired)

### Audio Module Phase 1 (HIGH)
- **Consistent box styling** â€” Added `buildAudioBox()` helper, Convert-style boxes
- **Proper header bar** â€” `TintedBar` with module title + stats integration wired

### Upscale Module Improvements (dev44)
- **One-click presets** â€” Hobbyist SDâ†’HD, Semi-Pro 1080pâ†’4K, Anime, Restoration, Social Media workflows
- **UI clarity** â€” Preset dropdown with description labels, clear AI+RIFE workflow
- **Detection reliability** â€” VerifyTool() checks PATH + app-local bin + smoke test
- **Optimization guide** â€” See `docs/UPSCALE_OPTIMIZATION.md`
- **Hardware acceleration** â€” Sync upscale HW accel from master setting

### Queue Module UI Polish (dev44)
- **TintedBar header** â€” Replaced custom header with `TintedBar` matching other modules
- **Status badge** â€” Shows active/completed/failed counts in header
- **48px bottom bar** â€” Restored VT green `TintedBar` (matches other modules)
- **Live output panel** â€” 4px VT green outline border
- **Thumbnail preview** â€” 90px tall with 3px module-color outline, auto-generated midpoint frame
- **Module colors** â€” `ModuleColor()` exactly matches main menu (all 13 modules)
- **Layout fixes** â€” Thumbnail left, text right; proper spacing

### Flags & i18n (dev44)
- **Language dropdown** â€” Fixed flag loading (removed incorrect `fs.Sub`); SVG flags now visible
- **Main menu** â€” "QUEUE" button uppercase in all 4 locales

### Thumbnail Quality (dev44)
- **Deinterlace filter** â€” `yadif=1` added to avoid interlaced frames
- **Interlace detection** â€” `findCleanFrameOffset()` skips to clean frames
- **Job log file** â€” Each thumbnail job writes FFmpeg output to timestamped log

### Module Pipeline (`&&` feature) (dev44)
- **Pipeline state machine** â€” `pipelineActive` on `appState` (off/waiting-step1/waiting-step2)
- **`&&` button** â€” Main menu header reflects state
- **Module tile dimming** â€” Invalid Step 2 targets dimmed
- **Queue integration** â€” `PipelineAfter` + `PipelineDeleteOnSuccess` fields on `queue.Job`
- **Intermediate files** â€” "Keep intermediate files" toggle in Settings â†’ Preferences

### Logging Audit (dev45)
- **Remove unused categories** â€” `CatEnhance`, `CatRip` removed from `internal/logging/logging.go`
- **Add CatQueue** â€” New category for queue operations, wired in `queue.go` and `main.go`
- **Fix enhancement module** â€” `CatEnhance` â†’ `CatModule` in `enhancement_module.go` and `onnx_model.go`
- **Fix rip module** â€” `CatRip` â†’ `CatDisc` in `rip_module.go`
- **Fix recentfiles.go** â€” `CatSystem` misuse â†’ `CatUI` via `cat()` helper

### FFmpeg DLL Bootstrap Fix (dev45)
- **Bundle DLLs in release** â€” FFmpeg shared DLLs built from source with all deps statically linked
- **Remove BtbN download** â€” `ffmpeg_bootstrap.go` no longer downloads from BtbN (eliminates `liblzma-5.dll` errors)
- **Build script** â€” `scripts/windows/build-ffmpeg-shared.ps1` builds FFmpeg shared DLLs from source
- **CI update** â€” `ci-build.ps1` bundles DLLs in `DLL/` subfolder (not root) within release ZIP
- **Legacy fallback** â€” `FFmpegDllDir()` still checks `%LOCALAPPDATA%\VideoTools\DLL` for old installs

---

## Version 0.1.1-dev44 (complete) - Playback & Sync Fixes

### Native Media Player â€” Playback & Sync Fixes (dev44)
- **Start/Resume state fix** â€” `Start()` was setting `e.paused=false` before starting the decode goroutine, but `Resume()` checked `!e.paused` and returned early. Decode loop never started on Play. Fixed: `Start()` now sets `e.paused=false` after launching goroutines.
- **Audio/video sync on load** â€” Audio clock drifted during `Load()` because `audioDecodeLoop` ran during `GrabFrame`, causing ~5 second offset before playback. Fixed by resetting clock to 0 in `ResetAfterGrab`.
- **Test pattern font** â€” Test pattern always renders with VCR OSD Mono font regardless of user preference.
- **FFmpeg bootstrap simplification** â€” Always downloads BtbN pre-built package to guarantee complete DLL set.

### Hardware Acceleration Parity (dev44)
- **Upscale module** â€” Initialize `upscaleHardwareAccel` from master `state.convert.HardwareAccel` so it's not empty on first use.
- **Filters module** â€” Add hardware acceleration dropdown with platform-appropriate options (nvenc, qsv, vaapi, amf on Linux; nvenc, qsv, amf on Windows). Wired to master setting via `HardwareAccel`/`SetHardwareAccel` callbacks.

### Audio Module Phase 2 â€” Native Player Integration (dev44)
- **InlineVideoPlayer singleton** â€” Added `GetAudioPlayer()` in `native_media.go` and stub in `native_media_stub.go`.
- **Video preview pane** â€” Added video container using `opts.Player.Widget()` in audio `BuildView()`, with idle state showing "DROP VIDEO TO LOAD" label.
- **Wired in audio_module.go** â€” Player passed to Options struct; video pane added to layout with HSplit (50/50 split).

### Settings
- **Player font preference** â€” Users can choose between IBM Plex Mono and VCR OSD Mono for the OSD. VCR OSD Mono has no Bold/Italic variants; UI gracefully falls back to Regular weight.

### Known Issues (pending)
- See `docs/PLAYER_DEBUG.md` for full list including: `predecodeFrom` sharing `formatCtx` with `demuxerLoop`, audio queue not flushed before seek, D3D11VA crashes when enabled.

## Version 0.1.1-dev43 (complete) - Player Stability Audit

### Native Media Player â€” Thread-Safety & Crash Fixes (dev43)
- [x] **Pixel format crash fix** â€” `GrabFrame` and `NextFrame` now use `frame.format` (actual decoded pixel format) instead of `videoCodecCtx.pix_fmt` when calling `ensureSwsCtx`. `videoCodecCtx.pix_fmt` can be `AV_PIX_FMT_NONE` until the codec processes its first SPS; passing it to `sws_getContext` returned nil, causing `sws_scale(nil, â€¦)` â†’ C SIGSEGV. `NextFrame` SW decode path was also missing the `ensureSwsCtx` call entirely.
- [x] **Close/demuxer race fix** â€” `Engine.Close()` previously freed `formatCtx` and `videoCodecCtx` immediately after `close(e.stop)`, while `demuxerLoop` may still be inside `av_read_frame`. Added `sync.WaitGroup` (`demuxerWg`) to `Engine`; `demuxerLoop` signals Done on exit, `Close()` waits before freeing any FFmpeg context.
- [x] **NextFrame/Close codec race fix** â€” `Close()` now acquires `videoCodecMu` before freeing `videoCodecCtx`, ensuring any in-flight `NextFrame` decode cycle has completed.
- [x] **seekLoop goroutine leak fix** â€” `InlineVideoPlayer.seekCh` was never closed, so the `seekLoop` goroutine leaked on every `Close()`. `seekCh` is now owned by `Load()`: closed and reallocated each time a file is opened; `Close()` closes it to drain the goroutine. `OnSeek` callback guards against nil/closed channel under the player mutex.

## Version 0.1.1-dev42 (complete) - Player Stabilization & Module Improvements

### Native Media Player â€” GStreamer Removal (dev42)
- [x] **GStreamer code removed** â€” All `internal/player/gstreamer*` deleted; `native_media` build tag is the only player path. No more GStreamer dependency.
- [x] **Player widget lifecycle** â€” `closeNativePlayer()` prevents audio hanging on module switch; `Widget().Refresh()` deferred after canvas swap to avoid blank panes.

### Native Media Player â€” D3D11VA / HW Decode Stabilisation (dev42)
- [x] **D3D11VA get_format callback** â€” `get_format` callback added to codec context; accepts `AV_PIX_FMT_D3D11VA_VLD` so D3D11VA decode starts on first packet.
- [x] **H.264 + D3D11VA crash fix** â€” Fixed `avcodec_send_packet` crash by pre-warming D3D11VA before first decode call.
- [x] **Dedicated HW frame buffers** â€” Separate `hwFramesCtx` prevents races between HW download and SW display paths.
- [x] **Lazy swsCtx creation** â€” `swsCtx` created on first `toRGBA()` call; avoids crash from invalid pixel format before first HW decode.
- [x] **HW frame transfer mutex** â€” `videoCodecMu` held during HWâ†’SW transfer and RGBA conversion; eliminates concurrent AVCodecContext access.
- [x] **HW decode codec filtering** â€” Only codecs that work without `get_format` callback get HW decode enabled; prevents crashes on VC-1, MPEG-2 etc.
- [x] **AV_NOPTS_VALUE guard** â€” `GrabFrame` and `NextFrame` skip frames with invalid PTS instead of passing them to audio/player.
- [x] **D3D11VA flush guard** â€” `avcodec_flush_buffers` skipped before first decoded frame to prevent crash.
- [x] **Safe HW frame download** â€” `av_hwframe_transfer_data` wrapped in recover/retry; falls back to SW decode on failure.

### Native Media Player â€” Audio/A-V Sync (dev42)
- [x] **A/V clock double-speed fix** â€” Master clock `SetSpeed()` wired after speed changes; no more 2Ã— playback after resume.
- [x] **AudioPlayer.Read() non-blocking** â€” Read returns immediately if buffer empty; prevents playback hang when audio underruns.
- [x] **Audio seek serialisation** â€” Codec operations serialized against Seek() to prevent hard crash from concurrent access.
- [x] **Pause spin-loop prevention** â€” Pause loop sleeps instead of busy-waiting; Close() doesn't race with pause state.
- [x] **Audio context pre-warm** â€” Audio context created at startup to avoid WASAPI initialization hang on first playback.
- [x] **SetSpeed deadlock fix** â€” Speed changes no longer block the audio callback thread.

### Native Media Player â€” SMPTE Bars & Idle State (dev42)
- [x] **SMPTE colour bars idle state** â€” Click-to-load dialog when no video is loaded; consistent across all module players.
- [x] **SMPTE bars in 4:3 ratio** â€” Letterboxing/pillarboxing for proper aspect ratio.
- [x] **SMPTE bars scaled to player size** â€” Dynamic sizing instead of fixed 1920Ã—1080.
- [x] **SMPTE idle text scaled proportionally** â€” Text size adapts to bar width.

### Native Media Player â€” Misc (dev42)
- [x] **Native Fyne icons** â€” Replaced emoji transport controls (â–¶, â¸, ðŸ”Š, â›¶) with `theme.IconName` equivalents.
- [x] **SmoothScrubbing crash fix** â€” Fixed crash on HW-decoded frames in thumbnail scrubber.
- [x] **GrabFrame deadlock fix** â€” Invalid PTS frames no longer block the decode loop.
- [x] **Letterbox fill performance** â€” Removed per-frame debug log; fixed fill colour on dark backgrounds.
- [x] **Initial frame display** â€” GrabFrame restored for first frame; skip only on subsequent calls to avoid crash.
- [x] **Panic recovery** â€” Added `recoverPanic` in GrabFrame, toRGBA, NextFrame, predecodeAhead, Load functions.

### Convert Module (dev42)
- [x] **Improvement plan** â€” Created `docs/CONVERT_MODULE_IMPROVEMENTS.md` with 10 phases covering missing UI controls, format presets, subtitle/audio track selection, video filters, metadata handling, presets, and i18n
- [x] **Clear button for output folder** â€” Added Clear button next to output directory in Convert settings.
- [x] **Output directory creation** â€” Ensure output directories exist before running convert/thumbnail/filter jobs.
- [x] **Drag-drop first frame fix** â€” `loadMultipleVideos` now calls `loadVideoNative` so the first frame appears immediately after drop.

### Audio Module (dev42)
- [x] **Improvement plan** â€” Created `docs/AUDIO_MODULE_IMPROVEMENTS.md` with 7 phases.
- [x] **VSplit layout** â€” Replaced custom HSplit with `container.NewVSplit` for consistency.
- [x] **Stats bar footer** â€” Added stats bar footer to Audio module.
- [x] **i18n all user-facing strings** â€” All Audio module labels and buttons use i18n keys.
- [x] **Drop label wrapping** â€” Wrapped drop label text for cleaner layout.

### Thumbnail Module (dev42)
- [x] **3-way output mode toggle** â€” Individual / Contact Sheet / Both selector replaces the old ContactSheet boolean.
- [x] **Image inspector** â€” Click any thumbnail or contact sheet tile to inspect at full window size.
- [x] **Contact sheet pad crash fix** â€” `trim` filter removed from filtergraph; time window via `-ss`/`-t` input options instead. Eliminates "padded dimensions cannot be smaller than input dimensions" on Xvid/MPEG-4 ASP.
- [x] **Contact sheet progress** â€” Full-panel display with live progress bar during generation.
- [x] **Contact sheet metadata** â€” Left-padding matched to logo (32px); metadata text vertically centred in header.
- [x] **CRLF line-break fix** â€” `\r` trimmed from ffprobe output to prevent `drawtext` line-break issues on Windows.
- [x] **TextWrapWord fix** â€” Placeholder label no longer stacks vertically in `NewCenter`.
- [x] **All to Queue per-file jobs** â€” "Add All to Queue" creates individual jobs per file instead of a single batch job.

### Subtitles Module (dev42)
- [x] **Video preview player** â€” Added video preview with synced subtitle overlay in Subtitles module.

### Snippet Module (dev42)
- [x] **Options drawer height** â€” Increased height to avoid unnecessary scrolling.

### CI & Build (dev42)
- [x] **libdrm-dev build dep** â€” Added `libdrm-dev` to Linux CI for FFmpeg.
- [x] **FFmpeg hwaccel disabled in CI** â€” GPU/hwaccel disabled in FFmpeg configure to avoid libdrm runtime dependency.
- [x] **Update script** â€” Linux binary replacement uses helper script.
- [x] **Update status icon** â€” Replaced â¬¤ (black large circle) with â— (bullet) for reliable cross-platform rendering.

### Misc (dev42)
- [x] **Temp file cleanup** â€” Preview-frame and cover-art temp files cleaned up on video unload.
- [x] **FFmpeg install button removed** â€” FFmpeg is bundled in binary; redundant install button removed from Settings.

## Version 0.1.1-dev40 (in progress) - Burn Module & File Manager

### Upscale Module (dev40)
- [x] **Real-CUGAN support** â€” Added model catalog with Real-CUGAN (Pro, Standard, No Denoise)
- [x] **Model catalog abstraction** â€” Extensible ModelInfo struct for future models (SPAN, Waifu2x, etc.)
- [x] **Dual-binary execution** â€” Automatically selects correct ncnn binary based on model family
- [x] **Auto-download installer** â€” ensureAppLocalRealCUGAN() with GitHub releases integration
- [x] **Dependency UI** â€” Real-CUGAN install button in Settings â†’ Dependencies

### DVD Authoring Fixes (dev40)
- [x] **UDF PartitionLength** â€” Fixed hardcoded 1000 sectors (~2MB) to `totalSectors - partitionStart`; VLC UDF path resolution now works for full-size DVDs
- [x] **Menu PTS timestamps** â€” Fixed menu VOB PTS incrementing by 27MHz ticks instead of 90kHz; was 300x too large, causing continuous VLC timestamp conversion errors
- [x] **Menu font** â€” Embedded IBM Plex Mono via `go:embed`; extracted to temp file for FFmpeg drawtext. No longer falls back to generic monospace on installed builds
- [x] **Chapter navigation** â€” Replaced linear interpolation (`ts/total*navCount`) with binary search on actual NAV_PCK PTMs for accurate chapter-to-sector mapping
- [x] **Return-to-menu** â€” Added `JumpVMGM_PGCN(1)` post-command to title PGCs in folder builds (was ISO-only); extras now also return to menu
- [x] **Extras return-to-menu** â€” Extra title PGCs get the same post-command as the main feature

### Burn Module (dev40) - completed dev45
- [x] **Design document** - Created docs/BURN_MODULE_DESIGN.md
- [x] **Module entry** - Wired showBurnView() in main.go
- [x] **UI implementation** - Source selection, drive detection, burn options
- [x] **Queue integration** - JobTypeBurn wired to executeBurnJob()
- [x] **Verify option** - Added checkbox with i18n support
- [x] **Drive capacity** - getDriveInfo() shows disc size in drive selector
- [x] **Main menu visibility** - Fixed burn/filemanager modules not appearing (were filtered out due to nil handler)
- [x] **Windows burn** - `isoburn.exe` (built-in) with eject via IOCTL_STORAGE_EJECT_MEDIA
- [x] **Linux burn** - `growisofs` (dvd+rw-tools) with progress parsing + SHA-256 verify
- [x] **Logging** - `CatBurn` category added, error handling improved

### File Manager (dev40)
- [x] **Design document** - Created docs/FILE_MANAGER_DESIGN.md with:
  - Lightweight UI with tabs, breadcrumbs, file list
  - Right-click context menu for module integration
  - Colour pills for module identification

### Quick Access Dropdown (dev40)
- [x] **Files dropdown** - Added to main menu header between sidebar toggle and Queue
- [x] **Context awareness** - Shows different options based on current module
- [x] **Open Files** - Button triggers OnOpenMore callback (placeholder)
- [x] **Open Output Folder** - Button triggers OnOpenFolder callback (placeholder)
- [x] **Recent Files** - Lists recent files with module context
- [x] **Design document** - Created docs/QUICK_ACCESS_DROPDOWN.md
- [x] **i18n** - All strings localized in en_ca, fr_ca, iu, iu_latin

### Queue Fixes (dev40)
- [x] **Right-click crash** - Fixed hard crash when right-clicking completed VIDEO_TS/author job
  - Added os.Stat check to detect directory outputs
  - Opens as folder instead of trying to probe as video

## Version 0.1.1-dev39 (complete) - Preview Tab, DVD Menu System & CI Green

### Player Crash Fixes (dev39)
- [x] **Panic recovery** - Added defer/recover in showPlayerViewForPath to catch CGO crashes
- [x] **Panic recovery** - Added panic recovery in loadVideoNative
- [x] **FFmpeg DLL** - Fixed local build script to copy FFmpeg DLLs to output
- [x] **FFmpeg DLL** - Use local FFmpeg at C:\ffmpeg\bin first (matches compilation)
- [x] **FFmpeg DLL** - Fall back to BtbN download only if no local FFmpeg found

### Author Module (dev39)
- [x] **Module extraction** - Extracted author module to `internal/app/modules/author/`
- [x] **Preview tab** - Added interactive Preview tab to Author module
- [x] **Video playback** - Preview tab can play videos by pressing menu buttons
- [x] **Tab visibility** - Preview tab shown only when Enable Menus is checked
- [x] **IFO audio track table** - VTS_MAT audio attributes now populated from actual track data (codec, channels, language) instead of hardcoded AC-3/stereo defaults
- [x] **Author drag crash fix** - `addAuthorFiles` moved off the UI thread; added `authorClipsRefresh` callback so drops no longer trigger a full 7-tab view rebuild (including GPU texture uploads) on the main thread during DnD completion
- [x] **VTS_MAT byte layout** - Fixed all field offsets in `mat_serialize.go` and `vtsi.go` to match libdvdread `vtsi_mat_t`; table offsets now at 0x0C8â€“0x0E4, title audio/video/subpicture attrs at correct positions, `vtsi_last_byte` and `vtstt_vobs` written; eliminates `zero_12`/`zero_17` violations and `ifoRead_VTS_PTT_SRPT failed` in dvdnav
- [x] **DVD menu VOB video (M1/M2)** - `runNativeSpumux` now encodes background PNG as MPEG-2 still video via ffmpeg and muxes video+SPU into proper DVD Program Stream VOB; falls back to video-only if SPU sub-stream mux fails
- [x] **PCI button table (M3)** - `PCIButton` struct added to `internal/dvd/vob/nav.go`; `WriteNAV_PCK` serializes up to 36 button entries with libdvdread-compatible coordinate packing at offset 98 within PCI payload; BTN_SL_NS/BTN_NS written at correct offsets 94/95
- [x] **VMGM_VOBS_Sector (M4)** - `vmgMat.VMGM_VOBS_Sector` set from `vtsSector("VIDEO_TS.VOB")` in ISO layout pass so dvdnav can locate the menu VOB on disc
- [x] **Menu PGC sector patching (M5)** - Each menu PGC `CellPlayback[0]` First/LastSector fields patched with actual disc sector range computed from per-MPG file sizes and the VIDEO_TS.VOB disc start sector; folder-mode equivalent added (cumulative file-size-based offsets, VMGM_VOBS_Sector set to VMG_Last_Sector+1 to ensure libdvdread opens VIDEO_TS.VOB)
- [x] **VOB sector counter fix** - `WriteVideo` restored to mutual-exclusive increment: `currentSector++` only in `else` branch when no padding; `WritePadding` handles it when padding is written. Fixes double-increment bug (introduced in refactoring) that corrupted `nv_pck_lbn` in menu VOB NAV_PCKs â†’ VLC/dvdnav crash
- [x] **ExtrasMpg wiring (M6)** - `menuSet.ExtrasMpg` concatenated into `VIDEO_TS.VOB`; extras PGC built and tracked in `menuMpgPaths` slice alongside main/chapters PGCs
- [x] **JumpVMGM_PGCN command (M7)** - `JumpVMGM_PGCNCommand(pgcN)` added to `internal/dvd/ifo/commands.go`; `ParseButtonCommand` translates `"jump menu N;"` / `"jump menu pgc N;"` to inter-menu PGC jump instructions

### CI Fixes (dev39)
- [x] **Submodule sync** - Pushed missing commits to lt_mirror/fyne.git
- [x] **filters_module.go build fix** - Removed invalid `.(*videoSource)` type assertion; `state.filtersFile` is already `*videoSource`, Go 1.26 CI failure fixed
- [x] **FFmpeg from source** - Switched to building FFmpeg/x264/x265 from source on both platforms; BtbN pre-built packages have no static `.a` libraries and DllImport-decorated x264/x265 headers
- [x] **x265.pc Libs fix** - C++ runtime deps moved from `Libs.private` to `Libs` so FFmpeg configure (which calls pkg-config without --static) sees them in its link test
- [x] **Windows multiple-definition fix** - Strip `-lsupc++` from CGO_LDFLAGS after pkg-config; prevents duplicate `std::type_info::operator==` between libsupc++.a and libstdc++.dll.a
- [x] **Windows disk space** - Added `-g0` to CGO_CFLAGS and MSYS2 cache cleanup step to prevent temp file exhaustion during Go build
- [x] **CI fully green** - Both Linux (run 1098) and Windows (run 1099) pass; release artifacts published (run 1100)

### Filter Integration (dev39)
- [x] **Create design document** - See docs/FILTER_INTEGRATION_DESIGN.md
- [x] **Add filters to Upscale module** - Integrate filter controls in upscale UI
- [x] **Refactor upscale pipeline** - Apply filters BEFORE upscale in encode chain
- [x] **Keep Filters module standalone** - Filters module can now queue filter-only jobs without upscaling; "Add to Queue" button added; executeFilterJob supports color correction, enhancement, transform, and stylistic filters via FFmpeg

## Version 0.1.1-dev38 (complete) - Module Extraction & Native Media Fixes

### CI Fixes (dev38)
- [x] **Windows CI** - Fixed `desktop.KeyEvent` â†’ `fyne.KeyEvent` for new Fyne API
- [x] **Windows CI** - Fixed `fyne.Color` â†’ `color.Color` for new Fyne API  
- [x] **Windows CI** - Fixed native_media linker error with VT_SUBTITLE_TYPE_TEXT
- [x] **Sub-agents** - Added usage guidelines to AGENTS.md for parallel task execution

### Native Go SPU Encoder (dev38)
- [x] **SPU encoder** - Added `MenuEncoder` to `internal/dvd/spu/spu.go`
- [x] **VOB WriteSPU** - Added `WriteSPU()` method to `vob.Muxer` in `internal/dvd/vob/vob.go`
- [x] **Menu wiring** - Replaced `runSpumux` calls with native `buildMenuSPU` in `author_menu.go`
- [x] **Zero-dep** - DVD menu generation now works without external spumux binary

### Module Extraction (dev38)
- [x] **Subtitles module** - Extracted to `internal/app/modules/subtitles/`
- [x] **Inspect module** - Extracted to `internal/app/modules/inspect/`
- [x] **Queue module** - Extracted to `internal/app/modules/queue/`
- [x] **Upscale module** - Extracted to `internal/app/modules/upscale/`
- [x] **Settings module** - Completed extraction to `internal/app/modules/settings/`

### Author Module i18n (dev38)
- [x] **i18n compliance** - Added 70+ Author* strings to i18n system
- [x] **Emoji removal** - Removed emoji from UI strings for portability
- [x] **Cross-platform** - Added spumux availability check with graceful fallback

## Version 0.1.1-dev37 (complete) - InlineVideoPlayer Wiring

### GPU Rendering Pipeline (NEW)
- [x] **Renderer interface** (`internal/media/gpu/renderer.go`) - Abstract GPU renderer with Texture interface
- [x] **OpenGL implementation** (`internal/media/gpu/opengl.go`) - OpenGL 4.6+ renderer scaffold
- [x] **Direct3D 11 implementation** (`internal/media/gpu/d3d11.go`) - D3D11 renderer scaffold for NVIDIA/AMD
- [x] **Texture utilities** (`internal/media/gpu/texture.go`) - Texture pooling, format conversion, scaling helpers
- [x] **Shader definitions** (`internal/media/gpu/shaders/`) - Vertex, fragment, and YUVâ†’RGB shaders for GPU rendering
- [x] **Keyboard shortcuts** (`internal/media/gpu/shortcuts.go`) - Full shortcut handler (Space, arrows, F, M, 0-9, <>, etc.)
- [x] **Seekbar with thumbnails** (`internal/media/gpu/seekbar.go`) - ThumbnailCache, preview on hover, ThumbnailGenerator interface
- [x] **Volume control** (`internal/media/gpu/seekbar.go`) - VolumeControl widget with mute toggle
- [x] **FFmpeg filter pipeline** (`internal/media/filters/pipeline.go`) - Deinterlace, scale, color correction, denoise, sharpen, crop, rotate filters with presets
- [x] **VideoPlayer overlay controls** (`internal/media/view.go`) - Integrated player controls with play/pause, seek, volume, hover-to-reveal

### Playback Enhancements (NEW)
- [x] **Loading state** - SetLoading/IsLoading methods on Engine, loading spinner on VideoPlayer
- [x] **Playback speed control** - Speed button on VideoPlayer with preset speeds (0.25x-2x), SetSpeed/GetSpeed wired to Engine
- [x] **Chapter parsing** - Chapter struct, parseChapters() via FFmpeg, GetChapters() API on Engine
- [x] **Chapter support in VideoPlayer** - SetChapters() method, chapters stored in player state
- [x] **Chapter markers on seekbar** - Canvas.Raster overlay draws vtGreen tick marks at chapter boundaries
- [x] **Chapter navigation** - Prev/next chapter buttons, OnPrevChapter/OnNextChapter callbacks
- [x] **Thumbnail extraction** - ThumbnailExtractor for async keyframe extraction
- [x] **Smooth scrubbing** - SmoothScrubbing with FrameCache, pre-decodes frames ahead
- [x] **Resume playback** - ResumeState in internal/media/state, JSON config persistence
- [x] **Picture-in-Picture** - PiPController with 4 corner positions, Windows SetWindowDisplayAffinity
- [x] **Audio pitch correction** - TempoController with FFmpeg atempo filter (0.25x-2.0x)

### GPU Rendering (NEW)
- [x] **OpenGL texture upload** - GLTexture with glTexImage2D, GPUTextureUpload pool
- [x] **OpenGL context** - GLContext with shaders (vertex/fragment), VAO/VBO setup
- [x] **D3D11 context** - D3D11Context/D3D11Texture for Windows NVIDIA/AMD
- [x] **Thumbnail cache** - ThumbnailCache in gpu/seekbar.go with GetNearest()

### Buffering & Error Recovery (Phase 4)
- [x] **BufferMode type** - BufferModeMinimal, BufferModeNormal, BufferModeAggressive
- [x] **Adaptive buffer sizing** - GetAdaptiveBufferSize() returns 10/50/100 based on mode
- [x] **Decode time tracking** - recordDecodeTime(), GetAverageDecodeTime() for performance monitoring
- [x] **Error recovery types** - PlaybackError struct with ErrCode* constants (Decode, Network, HWAccel, FileCorrupt, CodecMissing)
- [x] **Retry logic** - RecoverableError() and ShouldRetry() methods for transient error handling

### Video Filters (Phase 6)
- [x] **FilterPipeline integration** - SetFilterPipeline, GetFilterPipeline wired into Engine
- [x] **Filter API** - SetFilter, EnableFilter, ClearFilters, GetFilterGraph methods
- [x] **Preset support** - SetPreset with vintage, warm, cool, high_contrast, soft, vivid

### Picture-in-Picture (Phase 8)
- [x] **PiPController** - PiPController struct with Enable/Disable/Toggle/IsEnabled
- [x] **PiP positions** - TopLeft, TopRight, BottomLeft, BottomRight
- [x] **Windows PiP implementation** - SetWindowDisplayAffinity via user32.dll
- [x] **PiP stub for non-Windows** - Cross-platform compilation support
- [x] **PiP button in VideoPlayer** - togglePiP, IsPiP, OnPiP callback

### Resume Playback (Phase 9)
- [x] **ResumeState** - PlaybackPosition struct, JSON persistence
- [x] **SavePosition** - Saves position/duration/volume/speed to config
- [x] **GetPosition** - Retrieves saved position with expiry (30 days)
- [x] **ShouldResume** - Logic for >5% remaining, <7 days old
- [x] **Trim integration** - Checks saved position on load, seeks to it
- [x] **Auto-save** - Periodic position saving every 5 seconds during playback

### Subtitle Rendering (Phase 10)
- [x] **Subtitle overlay** - SubtitleOverlay struct with Bounds() method
- [x] **RenderSubtitles** - Draws subtitle background on video frame
- [x] **Subtitle decoding** - initSubtitleDecoder, decodeSubtitle, UpdateSubtitles
- [x] **Subtitle track selection** - SelectSubtitleTrack, DisableSubtitles
- [x] **Subtitle toggle button** - CC button in VideoPlayer controls
- [x] **Subtitle callbacks** - OnSubtitles, IsSubtitlesEnabled, SetSubtitlesEnabled
- [x] **Subtitle text rendering** - Bitmap-style text drawing with configurable alpha

### GPU Rendering (Performance)
- [x] **Fast scaling** - scaleNearest() with nearest-neighbor interpolation
- [x] **Display frame tracking** - displayWidth, displayHeight, frameSeq for caching
- [x] **Optimized draw loop** - Pre-calculated scaling factors, direct pixel access
- [x] **Bicubic scaling** - SWS_BICUBIC|C.SWS_ACCURATE_RND for better quality
- [x] **HW decode support** - VAAPI/D3D11VA/QSV hardware acceleration
- [x] **Thumbnail cache** - In-memory cache of thumbnails keyed by timestamp
- [x] **GetHoverFrame** - Get nearest cached thumbnail for hover preview
- [x] **AddThumbnailFrame** - Add frames to thumbnail cache during playback
- [x] **OnHover callback** - Trigger thumbnail extraction on seekbar hover
- [x] **FrameCache** - Pre-decoding frames for smooth scrubbing in scrub.go
- [x] **Async thumbnail extraction** - StartThumbnailExtraction with callback
- [x] **PlaybackFrameCache** - Engine frame cache for smooth scrubbing
- [x] **InitFrameCache** - Initialize frame cache with configurable size

### Fyne Fork for GPU Texture Optimization
- [x] **Fork created** - https://git.leaktechnologies.dev/lt_mirror/fyne
- [x] **TexSubImage2D** - Added to GL context interface for efficient texture updates
- [x] **All GL backends** - Implemented in gl_core.go, gl_es.go, gl_gomobile.go, gl_wasm.go
- [x] **UpdatePixels method** - Added to canvas.Raster for efficient pixel data updates
- [x] **Texture reuse** - newGlRasterTexture now reuses cached textures when size matches
- [x] **VideoTools integration** - go.mod uses replace directive to lt_mirror/fyne
- [x] **VideoPlayer wired** - SetFrame() uses UpdatePixels() when canvas size matches, enabling TexSubImage2D path

### Crash Logging & Error Recovery
- [x] **FFmpeg error logging** - avformat_open_input now logs FFmpeg error codes and messages
- [x] **avformat_find_stream_info logging** - Logs return code and error string on failure
- [x] **Stream detection logging** - Logs video/audio/subtitle stream indices
- [x] **Panic recovery in trim module** - loadVideo and playbackLoop have RecoverPanic
- [x] **Full goroutine dump** - LogAllGoroutines dumps all goroutines to crash log
- [x] **RecoverPanicWithCallback** - New logging helper with callback option
- [x] **Panic recovery in inspect module** - inspectLoadVideo and inspectPlaybackLoop have RecoverPanic

### VideoPlayer Consistency Across Modules
- [x] **Inspect module VideoPlayer** - Added `inspectState` struct with player and engine fields
- [x] **Inspect playback callbacks** - OnPlay, OnPause, OnSeek, OnSpeedChange wired to engine
- [x] **inspectPlaybackLoop** - Frame loop mirrors trim module pattern
- [x] **inspectLoadVideo** - File loading with engine initialization, mirrors trim module
- [x] **VideoPlayer widget integration** - Replaced static preview images with full VideoPlayer widget

### Loading & Error States (Phase 7)
- [x] **Buffering indicator** - SetBuffering/IsBuffering methods with "Buffering..." label
- [x] **Error state** - SetError/ClearError/HasError with red indicator + message
- [x] **Seeking indicator** - isSeeking state, FinishSeeking method
- [x] **Error logging** - Tapped on error logs message and shows crash log path

### i18n Strings (NEW)
- [x] **Player strings** - PlayerLoading, PlayerBuffering, PlayerSpeed, PlayerChapter, PlayerChapters, PlayerNoChapters, PlayerSpeed* (8 new strings)

### Bug Fixes (tester feedback)
- [x] **MKV 0 kbps bitrate tag** â€” Hardware encoders (AMF, NVENC) don't write per-stream BPS stats to Matroska; inject `-metadata:s:v:0 BPS=<bps>` for CBR/VBR MKV output so Windows Explorer and media tools display correct bitrate.
- [x] **Queue list flash with multiple jobs** â€” `UpdateJobs` now only calls `jobList.Refresh()` on structural changes; individual widget updates handle their own redraws. Eliminates rapid full-list redraws causing flicker. `Scroll.Refresh()` also called to fix blank body with multiple pending jobs.
- [x] **Queue status sidebar colour** â€” `statusRect` now tracked in `queueItemWidgets` and updated on status transition (Pendingâ†’Running colour change without rebuilding the whole card).

### CI & Packaging
- [x] **AppImage icon** â€” Switched from `VT_logo.png` (1024Ã—1024, rejected by linuxdeploy) to `VT_Logotype1.png` (512Ã—512). Updated `Icon=` in `VideoTools.desktop` to match icon filename stem; fixes "Could not find suitable icon" AppImage build error.

### DVD Authoring Engine (Phase 1â€“4.3)
- [x] **IFO command table** â€” `DVDCommandTable`, `JumpTTCommand`, `SetHL_BTNNCommand`, `ParseButtonCommand`, `SerializeCommandTable` in `internal/dvd/ifo/commands.go`.
- [x] **Menu PGC + VMG_PGCITI** â€” `BuildMenuPGC`, `WritePGCITI` wire button commands into VMG IFO; `GenerateVMG_IFO` accepts menu PGC; menu pipeline fully wired in `author_module.go`.
- [x] **TMAPT (time map table)** â€” `BuildLinearTMAPT` / `WriteTMAPT` in `vtsi.go`; linear sector approximation from VOB file size; wired into `GenerateVTS_IFO`.
- [x] **ISO 9660 hybrid disc** â€” Full path tables (L+M), directory records, shared file data sectors with UDF; `assignSectors` runs first so ISO 9660 can reference correct physical sector addresses.
- [x] **SPU DCSQ rewrite (Phase 4.2)** â€” Complete rewrite of `spu.go`: correct DCSQ header layout with `next_dcsq` offset, `SET_COLOR`/`SET_CONTR` commands with configurable `SPUOptions`, `SET_AREA` with `pack12pair`, `SET_ADDRESS` with computed field offsets, self-referencing DCSQ[1] terminator. `DefaultSPUOptions()` added.
- [x] **Integration tests (Phase 4.3)** â€” 30 tests across `spu`, `ifo`, `vob`, `udf` packages: SPU packet structure/commands/terminator/offsets, PGCITI sector padding/NrOf/Category, TMAPT entry count/bounds/header, NAV_PCK size/start codes/LBN/SRI, UDF/ISO 9660 PVD magic/VDS terminator/system area. All green.

### Localization (i18n) - dev34 carry-forward
- [x] **Subtitles i18n strings** â€” Added 9 new strings to `internal/i18n/strings.go` (SubtitlesOfflineHint, SubtitlesEmpty, SubtitlesExtractEmbed, SubtitlesOCROutput, SubtitlesOCRLanguage, SubtitlesShiftOffset, SubtitlesStart, SubtitlesEnd).
- [x] **French (fr-CA) translations** â€” Subtitles module fully translated.
- [x] **Audio i18n wired** â€” 5 new strings wired up in `internal/app/modules/audio/view.go`.
- [x] **Filters i18n wired** â€” 24 new strings wired up in `internal/app/modules/filters/view.go`.
- [x] **Inspect i18n wired** â€” 3 new strings wired up in `internal/app/modules/inspect/view.go`.
- [x] **Settings StatusNoActiveJobs** â€” Added to `internal/ui/components.go` status bar.
- [x] **Dialog title i18n** â€” 15+ new translation keys (DialogInterlacingResults, DialogAutoCropDetection, DialogNoBlackBars, DialogQueueNotInit, DialogNoRunningJob, LabelSnippet, MergeStarted, TrimJobAdded) wired into main.go for Convert/Merge/Trim/Snippet modules.

### Media Engine Overhaul
- [x] **SplitView fixes** â€” Fixed divider color using exact VT Green #4CE870; implemented MouseMoved/Dragged for draggable divider; added SetOnDividerMove callback.
- [x] **AudioPlayer improvements** â€” Added volume control (SetVolume/GetVolume), mute functionality (SetMuted/IsMuted), pause/resume control, proper error handling with logging, fixed resample buffer handling.
- [x] **Engine enhancements** â€” Added VideoInfo struct for metadata, Pause/Resume/TogglePause controls, volume/mute/speed controls, seeking with configurable accuracy (Frame/Keyframe/Accurate), CurrentTime() and QueueStats() methods.
- [x] **Queue improvements** â€” Added configurable max size limits, NewPacketQueueWithMaxSize constructor, SetMaxSize/GetMaxSize/IsFull methods.
- [x] **Subtitle extraction** â€” New `subtitle.go` with SubtitleExtractor for parsing subtitle streams, SRT and ASS export, SubtitleTrack and Subtitle types.
- [x] **Tests** â€” Added comprehensive tests for queue, clock, and utility functions in `media_test.go`.
- [x] **Player deprecation** â€” Marked BackendMPV and BackendVLC as deprecated; factory now only supports FFplay and Native engines.

### Module Updates
- [x] **Trim module stub** â€” Updated `internal/app/modules/trim/stub.go` to match main.go calls (`ModuleColor`, `OnShowQueue`, `OnAddToQueue`, `TrimClip` struct, second `initialPath` param).
- [x] **Trim view** â€” Added `TrimClip` struct and `OnAddToQueue` callback to native trim view.
- [x] **Trim handler** â€” Fixed `internal/modules/handlers.go` to use correct logging category.
- [x] **Trim job submission** â€” `submitTrimJob` creates queue.Job with proper Type, InputFile, OutputFile, and Config.
- [x] **Settings module extraction** â€” Moved tab builders to `internal/app/modules/settings/tabs.go`. Created callback interfaces (BenchmarkCallbacks, PreferencesCallbacks, DependencyCallbacks) for loose coupling. Reduced settings_module.go from 2316 to ~1700 lines.
- [x] **Inspect module extraction** â€” Moved `showInspectView` and `buildInspectView` to `internal/app/modules/inspect/view.go`. Root `inspect_module.go` is thin shim.
- [x] **Queue module extraction** â€” Moved queue view builders and refresh helpers to `internal/app/modules/queue/view.go`. Root `queue_module.go` delegates to internal package.
- [x] **Subtitles module extraction** â€” Moved package structure, types, adapter, and view code to `internal/app/modules/subtitles/`.
- [x] **Upscale module helpers** â€” Full module extracted to `internal/app/modules/upscale/` with helpers.go, types.go, and view.go. Root `upscale_module.go` is thin shim delegating to internal package.

### UI Fixes
- [x] **Back button consistency** â€” Module name uppercase on all modules.
- [x] **Auto-check dropdown** â€” Fixed language switching issue in Settings Updates section.
- [x] **Thumbnail contact sheet** â€” Increased header height (130â†’150px), added filename truncation.
- [x] **Inspect preview placeholder** â€” Replaced stuck "Loading preview" with proper idle player state and icons.

### Interlace Detection
- [x] **Preview frame capture** â€” Capture preview frames before running interlace analysis to avoid UI stuck states.

### Hardware Acceleration Detection Fix
- [x] **Runtime HW detection** â€” `hwAccelAvailable()` now does actual encode probes for each method (NVENC, QSV, VAAPI, VideoToolbox, AMF) instead of just checking `ffmpeg -hwaccels`. Prevents false positives like QSV being auto-selected on laptops without Intel GPUs.
- [x] **HW decode support** â€” Native media engine now supports GPU-accelerated video decoding via FFmpeg's hwcontext API. Supports VAAPI (Linux), D3D11VA (Windows), and QSV (Windows/Linux). Automatic fallback to software decoding if HW unavailable.

## Version 0.1.1-dev35 (2026-03-16) - Native Media Engine & Trim Module

- [x] **Trim Module UI** â€” Implemented a professional, dual-pane layout for the Trim module, matching the Convert module's "source of truth" visual style.
- [x] **Native VideoPlayer Widget** â€” Created a reusable `media.VideoPlayer` widget in the native FFmpeg-CGO engine for high-performance single-video playback.
- [x] **Trim Localization** â€” Added full i18n support for the Trim module (English, French, Inuktitut).
- [x] **Compare Native Integration** â€” Refactored the Compare module to use the native `SplitView` and dual-engine playback loops.

## Version 0.1.1-dev33 (2026-03-14) - Native Authoring Foundation

### Native DVD Engine
- [x] **Wiki synchronization** â€” Ported internal documentation to the Forgejo wiki with corrected links and navigation. Established Home, Documentation, and Sidebar pages.
- [x] **Native DVD Engine structure** â€” Created `internal/dvd/` modular package structure (`udf`, `ifo`, `vob`, `spu`).
- [x] **MPEG-PS / VOB Muxer Foundation** â€” Implemented MPEG-PS packetization, Pack/System/PES headers, and DVD-specific Navigation Pack (NAV_PCK) structures.
- [x] **IFO/BUP Structure Generation** â€” Implemented binary serialization for VTSI and VMGI tables, and created `IFOBuilder` for automatic IFO/BUP and backup file creation.
- [x] **SPU subpicture encoder** â€” 2-bit RLE subpicture encoder for DVD menu button highlights.
- [x] **UDF reader foundation** â€” UDF 1.02 disc type detection and reader scaffolding in `internal/dvd/udf`.
- [x] **Authoring Architecture Consolidation** â€” Unified DVD and Blu-ray workflows into the core Author and Rip modules. Removed the redundant standalone Blu-ray module to streamline UI/UX.
- [x] **Dependency Cleanup** â€” Fully removed `dvdauthor` and `xorriso` as dependencies. Author and Rip modules are now enabled cross-platform by default with optional visibility toggles in Settings.

### UI Alignment (dev33 polish pass)
- [x] **Module UI alignment** â€” Convert, Thumbnail, Filters, Audio, Compare, and Inspect module layouts aligned with the standardised Convert module style (consistent labels, separators, padding).

### Thumbnail / Contact Sheet
- [x] **IBM Plex Mono applied** â€” `MonoTheme` now set on the Fyne app at startup; IBM Plex Mono Regular and Bold used throughout the UI and in contact sheet text overlays.
- [x] **Bold title font in contact sheet** â€” Filename (line 1) rendered with IBM Plex Mono Bold; metadata lines 2/3 use Regular.
- [x] **VT Green contact sheet title** â€” Line 1 (filename) colour changed from white to `#4CE870`, matching the main menu "VideoTools" title.
- [x] **Contact sheet line-3 wrapping fixed** â€” FFmpeg `drawtext` treats `|` as a newline; replaced all ` | ` separators with ` Â· ` (U+00B7).
- [x] **Contact sheet progress bar** â€” `-progress pipe:1` flag was appended after the output path; moved before it so FFmpeg emits progress events correctly.
- [x] **Duplicate ffprobe calls eliminated** â€” `buildMetadataFilter` no longer calls `getVideoInfo`/`getDetailedVideoInfo` internally; pre-computed data passed from `generateContactSheet`, reducing ffprobe invocations from 6-7 down to the minimum needed.
- [x] **CMD windows suppressed** â€” All `exec.Command` calls in `internal/thumbnail` now call `hideCmd()` (platform-specific: `SysProcAttr{HideWindow: true}` on Windows, no-op elsewhere).

### Fixes
- [x] **Scroll passthrough on Entry/Select widgets** â€” Mouse wheel events were swallowed when hovering over text inputs or dropdowns inside `FastVScroll` panels. Made `scrollClip` implement `fyne.Scrollable` with forwarding to `FastVScroll`; added `IsClip()` to renderer for correct GL scissoring.
- [x] **Icons not loading** â€” `GetIcon` was reading from the embed root rather than the icons subdirectory. Fixed by passing `fs.Sub(iconsFS, "assets/icons")` to `ui.SetIconsFS()`.
- [x] **App icon path** â€” `logoAssets.Open` was using a bare filename; corrected to `assets/logo/VT_Icon.ico`.
- [x] **CI missing imports** â€” `internal/ui/components.go` was missing `"fyne.io/fyne/v2/driver/desktop"` and `"fyne.io/fyne/v2/layout"` imports; fixed to unblock Linux and Windows CI builds.

## Version 0.1.1-dev32 (2026-03-12) - UI Polish and Fixes

### Kickoff
- [x] Bumped version markers to v0.1.1-dev32 (main.go, VERSION, FyneApp.toml).

### Icons
- [x] SVG icon library added - ~150 Material Design SVG icons added to `assets/icons/`; ASCII icon placeholders replaced with real icon resources across the UI.
- [x] Icons embedded into binary (issue #20) - `icons_embed.go` uses `//go:embed assets/icons` so icons are baked in at compile time; `ui.SetIconsFS()` / `GetIcon()` rewritten to read from `fs.FS` with no runtime disk access. Resolves blank icons on installed builds.

### Settings â€” Dependencies
- [x] Platform-filtered dependency list - Dependencies tab now only shows entries relevant to the current platform using `isDependencyAvailableForPlatform`.
- [x] Install buttons per dependency - Each dependency row shows an actionable Install button; FFmpeg on Windows uses the existing app-local bootstrap.
- [x] Uninstall buttons - Uninstall button shown per dependency when an uninstall command is available.
- [x] WSL auto-install reverted - Installing Ubuntu via WSL would consume 5-10 GB; unacceptable for a lightweight app. dvdauthor/xorriso platforms set to `["linux","darwin"]` only.
- [x] Updates tab â€” Forgejo tags API wired - Check for Updates now hits `/api/v1/repos/leak_technologies/VideoTools/tags?limit=1`; compares against `appVersion`; fixed owner mismatch (`/stu/` â†’ `/leak_technologies/`).
- [x] Disc module toggles hidden on Windows - Author, Rip, and Blu-ray visibility checkboxes in Settings are hidden on Windows since dvdauthor/xorriso are unavailable on that platform.
- [x] cmd window popups suppressed on Windows - All `exec.Command` calls in settings and WSL utilities replaced with `utils.HideWindowExec`/`utils.HideWindowExecContext` (`SysProcAttr{HideWindow: true}`).

### Modules â€” Convert
- [x] Player layout fixed - Video pane used `container.NewVBox` which collapsed the canvas.Image to 0px; fixed with `container.NewBorder` (transport bar pinned bottom, video fills centre).
- [x] Player layout â€” VSplit gap fixed - `container.NewVBox(videoPanel, leftGap)` was leaving dark empty space in VSplit top half; `videoPanel` now passed directly to `container.NewVSplit`.
- [x] Player icons fixed - ASCII fallback labels (`-/`, `-|`, `|-`) replaced with `widget.NewButtonWithIcon` using embedded SVG icons (play_pause, skip_previous, skip_next).
- [x] `s.active` never set to "convert" fixed - `showConvertView` now sets `s.active = "convert"` so drop handling, keyboard shortcuts, and all `if s.active == "convert"` guards work correctly.
- [x] `s.source` not updated on single-video load fixed - `loadVideo` now sets `s.source = src` before calling `showConvertView`.
- [x] Convert UI cleanup (issue #5) - Label alignments standardised, consistent separators added.

### Modules â€” Compare
- [x] Hide/show player toggle (issue #1) - Compare module now has a toggle button to hide/show both video players, giving more vertical space for the diff view.

### Modules â€” Author / Rip
- [x] Hidden on Windows - Author and Rip modules are hidden from the main menu on Windows until cross-platform disc authoring is implemented.

### Navigation
- [x] Mouse back/forward buttons - Side mouse buttons (button 4/5) trigger back/forward navigation.
- [x] Mouse back button fixed - Back button now returns directly to main menu.
- [x] Keyboard shortcuts simplified - Ctrl+Enter is the universal confirm shortcut on Linux/Windows; Author module wired.

### UI
- [x] Main menu tile colour consistency - Unavailable module tiles now show dimmed module colour on first load, matching post-navigation appearance.
- [x] Drag-to-scroll on FastVScroll (issue #19) - `container.Scroll` implements `fyne.Draggable` but discards desktop drag events via a mobile-only guard. Replaced inner scroll with a custom `scrollClip` widget that does not implement `fyne.Draggable`, allowing drag events to reach `FastVScroll`.
- [x] Pulsing drop indicator on video stage - Video drop zone pulses when a draggable file is hovered over the convert player area.
- [x] FastVScroll on upscale settings and convert metadata - Both panels now use FastVScroll for consistent drag-to-scroll.

### Auto-Update
- [x] In-app updates - Windows and Linux builds support in-app auto-update via Forgejo releases API.

## Version 0.1.1-dev31 (2026-03-12) - UI Stability and Cleanup

### Kickoff
- [x] Bumped version markers to v0.1.1-dev31 (main.go, VERSION, FyneApp.toml).
- [x] Created Forgejo issue tracker from known issues and carry-forward items.
- [x] Closed dev30 with CI validation confirmed (runs 219/220/221, commit 2cbb3a2).

### UI
- [x] Module settings scrolling (issue #3) - Scroll containers added to all non-Convert module settings panels; primary action buttons moved to footer action bar for Rip, Subtitles, Filters, Thumbnail, Merge.
- [x] Window resize stability (issue #4) - setContent pins window to pre-switch size to prevent layout-driven resize on module change.
- [x] Convert video pane overflow - Removed rigid SetMinSize from loaded-video stage; VSplit 50/50 offset now holds correctly.
- [x] Convert module vertical layout - Changed left column to VSplit with 50/50 split between video player and metadata.
- [x] WSL compile fix - Fixed undefined windowsToWSLPath (wrong capitalisation) breaking both CI platforms.
- [x] Click-and-drag scrolling - FastVScroll now implements desktop.Mouseable and fyne.Draggable so users can click-and-drag content to scroll, mirroring mobile/touch behaviour.

### Author Module
- [x] Menu templates - Added Minimal template and separated templates from themes.
- [x] Menu themes - Added 8 preset themes (VideoTools, Minimal, Western, Film Noir, Classic Hollywood, Warm Cinema, Ocean, Nature).
- [x] Custom background for all templates - Background image now available for all template types.
- [x] Motion backgrounds - Added support for video loop backgrounds (MPG) with embedded audio.

### Refactor
- [x] Phase 3 slice â€” Player and Enhancement extracted from `main.go` into `player_module.go` and `enhancement_module.go`.
- [x] Phase 3 slice â€” Upscale view moved from `main.go` into `upscale_module.go` alongside existing helpers.
- [x] Phase 3 slice â€” Compare and Compare Fullscreen views moved from `main.go` into `compare_module.go`.
- [x] Convert module partial modularisation - Added `ShowView`, `ConvertState`, and `ConvertCallbacks` to `internal/app/modules/convert/view.go`; added `showConvertView` shim and type-converter helpers in `main.go`. Full `buildConvertView` extraction deferred due to high coupling with `appState` (~3,500 lines, ~30+ state fields).
- [x] WSL ISO creation on Windows - Added `internal/utils/wsl.go` with WSL detection, path conversion, and ISO tool detection for consistent DVD ISO generation on Windows.

### Documentation
- [x] Author menu templates scope - Added comprehensive TODO section for menu templates (Minimal, Classic, Grid, Filmstrip, Poster, Cinematic) and themes (Minimal, Classic Hollywood, Film Noir, VideoTools, Warm Cinema, Ocean, Nature, Custom).

## Version 0.1.1-dev30 (2026-03-04) - Development Cycle Kickoff

### Maintenance
- [x] Bumped app version metadata to v0.1.1-dev30 (main.go, VERSION, FyneApp.toml).
- [x] Updated dev release publishing to append the current version changelog section to the nightly release notes.
- [x] Documented versioning policy: continuous global `-devN` numbering with public releases using base versions only.
- [x] Added a full module testing checklist and public release gate criteria for deciding when to bump to the next public version.
- [x] Cleaned root structure by removing stray artifacts, relocating the QR demo entrypoint under `cmd/`, and adding repository hygiene rules to `AGENTS.md`.
- [x] Added a phased dev30 refactor plan (`docs/REFACTOR_DEV30_PLAN.md`) to guide gradual package/entrypoint cleanup.
- [x] Started Phase 2 refactor by moving module config path logic into `internal/app/configpath` and updating all module callers.
- [x] Continued Phase 2 refactor by moving merge/thumbnail config persistence into `internal/app/modulecfg` while keeping stable `package main` wrappers.
- [x] Continued Phase 2 refactor by moving naming metadata/output-base helper logic into `internal/app/naming` with compatibility wrappers.
- [x] Continued Phase 2 refactor by moving rip/subtitles config persistence into `internal/app/modulecfg` with compatibility wrappers.
- [x] Continued Phase 2 refactor by moving author config persistence into `internal/app/modulecfg` with compatibility wrappers.
- [x] Continued Phase 2 refactor by moving audio config persistence into `internal/app/modulecfg` with compatibility wrappers.
- [x] Continued Phase 2 refactor by replacing duplicated config-path helpers in `main.go` with shared `internal/app/configpath` lookups.
- [x] Continued Phase 2 refactor by moving recovery/benchmark/history config persistence into `internal/app/appcfg` with aliases/wrappers in `main.go`.
- [x] Continued Phase 2 refactor by moving convert config JSON load/save plumbing into shared `internal/app/appcfg` store helpers.
- [x] Continued Phase 2 refactor by moving convert config normalization rules into `internal/app/appcfg` with thin wrapper calls in `main.go`.
- [x] Fixed Forgejo Linux/Windows package build break by restoring `path/filepath` import in `audio_module.go` after refactor.
- [x] Fixed Forgejo Linux/Windows package build break by restoring `path/filepath` import in `rip_module.go` after refactor.
- [x] Updated Forgejo publish workflow to read version from `VERSION` first, patch matched release metadata, and keep dev updates scoped to the intended tag.
- [x] Fixed Forgejo Linux/Windows package build break by restoring missing `encoding/json` and `path/filepath` imports in `subtitles_module.go` after refactor.
- [x] Fixed convert drag/drop analysis on Windows by using the configured FFprobe path instead of a hardcoded `ffprobe` command in `probeVideo`.
- [x] Fixed thumbnail metadata probing to use the configured FFprobe path so app-local FFprobe works when PATH does not include FFprobe.
- [x] Simplified Forgejo dev release notes to publish concise version highlights instead of dumping the full changelog section into the release body.
- [x] Added a stale-run publish guard in Forgejo dev release workflow so only the latest `master` commit updates release metadata/assets.
- [x] Started Phase 3 refactor by moving About dialog UI implementation into `internal/app/modules/about` with a thin `package main` shim.
- [x] Continued Phase 3 refactor by moving missing-dependencies dialog rendering into `internal/app/modules/deps` with a thin `package main` shim.
- [x] Updated About/QR documentation links to use the Forgejo wiki URL after retiring `docs.leaktechnologies.dev`.
- [x] Updated installation/readme docs to point users at in-repo docs and Forgejo wiki as the active documentation locations.
- [x] Continued Phase 3 refactor by moving main menu visibility/dependency filtering and active-job mapping helpers into `internal/app/modules/mainmenu`.
- [x] Added `docs/DEV30_FINALIZATION_CHECKLIST.md` to formalize dev30 closeout gates (CI, smoke tests, dependency checks, docs, tagging, and dev31 kickoff).
- [x] Expanded `AGENTS.md` with a full `dev30` closeout and `dev31` handoff brief so a new coding agent can take over without reconstructing project state.

## Version 0.1.1-dev29 (2026-03-03) - Build and Runner Stabilization

### Build/CI
- [x] Fixed dev-packages.yml YAML parsing in the Windows bundled dependency note block.
- [x] Fixed module import paths after queue/main menu modularization so vendored builds compile correctly.
- [x] Fixed duplicate package main declaration in mainmenu_module.go that broke Windows packaging.
- [x] Fixed convert/aspect compile regressions from duplicate scaling block and late custom-aspect declarations.
- [x] Removed stale `go-qrcode` import from `main.go` after About module extraction.
- [x] Added Windows packaging fallback to download missing Tesseract `eng/fra/iku` language data.
- [x] Switched bundled packaging to treat GStreamer as optional on both Windows and Linux (no hard fail).
- [x] Added resilient whisper model download fallbacks and made missing model non-fatal for bundled packaging.
- [x] Added Linux `zip` dependency in CI build deps to prevent bundled zip step failures.
- [x] Disabled bundled package generation for dev channel builds to stabilize nightly/pre-release pipelines.
- [x] Removed bundled package generation from Linux/Windows dev-packages workflow; VT now publishes the standard package only.
- [x] Fixed main menu tile layout to a stable 3-column grid without expanding the window beyond screen bounds.
- [x] Fixed Forgejo release asset purge logic to reliably delete old assets before upload.
- [x] Fixed Forgejo release asset delete endpoint path to avoid 404 during publish.
- [x] Added a Blu-ray module visibility toggle in Preferences and wired it to main menu filtering.
- [x] Benchmark apply now updates hardware acceleration only and explicitly leaves codec/preset unchanged.
- [x] Bumped app version metadata to v0.1.1-dev29 (main.go, VERSION, FyneApp.toml).

## Version 0.1.1-dev28 (2026-02-21) - Windows First-Run Dependency Bootstrap

### Windows Dependencies
- [x] Added first-run in-app FFmpeg bootstrap prompt when FFmpeg is missing.
- [x] Added app-local FFmpeg install flow to `%LOCALAPPDATA%\VideoTools\bin` (downloads official Windows portable package and extracts `ffmpeg.exe` + `ffprobe.exe`).
- [x] Added app-local FFmpeg discovery in platform detection so installed binaries are reused on later launches.
- [x] Updated dependency checks to treat configured app-local FFmpeg paths as installed.
- [x] Added a Settings > Dependencies FFmpeg install action on Windows using the same app-local bootstrap flow.

### Cross-Platform Dependencies
- [x] Sorted Settings > Dependencies with required dependencies first and stable alphabetical ordering.
- [x] Added Settings > Dependencies FFmpeg install/uninstall actions for Linux via package-manager commands.
- [x] Replaced Convert UI Unicode/emoji labels with ASCII-safe strings to prevent mojibake in Windows terminal/font environments.

### UI
- [x] Removed the Bitcoin address from the About/Support dialog.
- [x] Added adaptive scroll speed for long panels to improve multi-resolution navigation.
- [x] Made Settings tabs scroll independently to keep tab headers visible.
- [x] Promoted master settings for hardware acceleration and module visibility.
- [x] Focused language options on Canadian English/French and Inuktitut.
- [x] Refactored main menu flow into a dedicated module file for easier maintenance.
- [x] Improved aspect ratio handling using display aspect ratio metadata and added a 17:9 target.
- [x] Show the detected source aspect ratio alongside the Source aspect option.
- [x] Added lightweight logging for source/target aspect details and ignored stale auto-crop values when auto-crop is off.
- [x] Added a Custom aspect option for clean ultrawide support with minimal UI clutter.
- [x] Aligned aspect conversion with target resolution to avoid odd output sizes (e.g., 1920x1082).
- [x] Stopped auto-resizing the window on each module switch to prevent misclicks.
- [x] Cleaned mojibake/garbled UI characters in core UI labels.
- [x] Conversion worker panics now surface a failure dialog instead of closing the UI.
- [x] Added a lightweight conversion recovery notice on next launch with persisted state.
- [x] Modularized the About/Support dialog into `about_module.go`.
- [x] Modularized the missing dependencies dialog into `deps_dialog_module.go`.
- [x] Modularized the queue view into `queue_module.go`.
- [x] Fixed dev-packages workflow YAML parsing for bundled deps note.
- [x] Fixed module imports for main menu and queue modules.
- [x] Fixed duplicate package declaration in `mainmenu_module.go`.

### Packaging
- [x] Added bundled Windows/Linux packages with FFmpeg, Tesseract, and GStreamer plus bundled launchers.
- [x] Bundled packages now include the whisper.cpp small model and enforce required dependency payloads.

### Subtitles
- [x] Added embedded subtitle extraction with lossless and text (SRT) modes.
- [x] Added safer subtitle embedding that preserves sync and warns on incompatible outputs.
- [x] Integrated Tesseract OCR for image-based subtitles with SRT/ASS output.
- [x] Normalized OCR output and merged consecutive duplicate cues for cleaner timing.

### Snippet
- [x] Added AV1 encoder fallback when `libsvtav1` is unavailable.

## Version 0.1.1-dev27 (2026-02-13) - Windows Build Artifact Cleanup

### Maintenance
- âœ… **.gitignore updates** - Excluded Windows build artifacts (*.syso) and agent working directory (.opencode/).
- **Forgejo Windows outputs** - Emit `GITHUB_OUTPUT` as UTF-8 (no BOM) to prevent host-runner post-step failures.

## Version 0.1.1-dev26 (2026-01-XX) - Windows Build System & Mirror Infrastructure

### Infrastructure
- **Mirror hosting** - Created lt_mirror repository on git.leaktechnologies.dev for downloads when source sites block bots. Used for GStreamer, DVDStyler, Whisper, FFmpeg.
- **Forgejo CI/CD** - Self-hosted runner setup for Windows, CI workflows for Windows/Linux, artifact versioning, optional EXE signing.

### Windows Build System
- **Installer** - Switched from Scoop to Chocolatey for dependencies, added MSYS2 for builds, dependency checking with early exit, progress bars for downloads, installer verification.
- **Build scripts** - Console popup suppression for CGO, icon embedding, windowsgui flag, Go module caching, fixed Unicode encoding.

### Documentation
- Added Forgejo runner and Windows service setup docs.

## Version 0.1.0-dev24 (2026-01-06) - DVD Menu Templating System

### Features
- Ã¢Å“â€¦ **DVD Menu Templating System**
  - Refactored `author_menu.go` to support multiple, selectable menu templates.
  - Implemented a `MenuTemplate` interface for easy extensibility.
  - Created three initial menu templates:
    - **Simple**: The default, clean menu style.
    - **Dark**: A dark-themed menu for a more cinematic feel.
    - **Poster**: A template that uses a user-provided image as a background.
- Ã¢Å“â€¦ **Menu Customization UI**
  - Added a "Menu Template" dropdown to the authoring settings tab.
  - Added a "Select Background Image" button that appears when the "Poster" template is selected.
  - User's menu template and background image choices are persisted in configuration.

### Maintenance
- Ã¢Å“â€¦ **Git author cleanup**
  - Rewrote commit history to ensure consistent commit attribution.
- Ã¢Å“â€¦ **Installer dependency parity**
  - Ensured pip is installed (Linux/Windows) and skipped Go/pip installs when already present.
- Ã¢Å“â€¦ **Windows installer parse fix**
  - Normalized PowerShell here-strings to prevent parse errors during installation.
- Ã¢Å“â€¦ **Go auto-install on Windows**
  - Removed the Go prompt in `install.sh`; missing Go is now installed automatically.
- Ã¢Å“â€¦ **Windows install workflow split**
  - `install.sh` now delegates to the Windows installer to avoid mixed-shell prompts.
- Ã¢Å“â€¦ **Windows installer entrypoint**
  - Added `install-windows.ps1` and made `install.sh` Windows-safe with a clear handoff message.
- Ã¢Å“â€¦ **Git Bash Windows handoff**
  - `install.sh` now runs the Windows installer in the same terminal via `winpty` when available.
- Ã¢Å“â€¦ **Windows root entrypoints**
  - Added `install.bat` and `install.ps1` to avoid Git Bash popping up from PowerShell.
- Ã¢Å“â€¦ **Windows scripts entrypoints**
  - Added `scripts/install.ps1` and `scripts/install.bat` to keep the Windows workflow inside PowerShell/CMD.
- Ã¢Å“â€¦ **Windows setup launcher alignment**
  - `scripts/_internal/setup-windows.bat` now delegates to `scripts/install.bat` for a single Windows flow.
- Adjusted Forgejo artifact actions to v3 for runner compatibility.
- Added Windows CI icon embedding via windres when available.
- Moved default logs to ~/Videos/VideoTools/logs with user override in Settings.
- Added Linux AppImage packaging in Forgejo builds with embedded VT icon.
- Ã¢Å“â€¦ **Agent workflow rules**
  - Added `AGENTS.md` to enforce staging, commits, and documentation updates.
- Fixed Linux script paths after scripts reorg (build/install/run).
- Updated Forgejo dev packaging to use appVersion-based artifacts and stable/dev release tagging.
- Ã¢Å“â€¦ **Player fullscreen toggle**
  - Added fullscreen toggle to the Player module controls.
- Ã¢Å“â€¦ **Player EOS handling + metadata access**
  - Stop playback cleanly on EOS and expose duration/FPS from GStreamer.
- Ã¢Å“â€¦ **Main menu title cleanup**
  - Header now shows "VideoTools" only; platform suffix moved to the footer version label.
- Ã¢Å“â€¦ **Main menu palette refresh**
  - Restored a diverse, eye-friendly rainbow palette while keeping Convert constant.
- Ã¢Å“â€¦ **Main menu readability**
  - Increased tile label size and adjusted colors for better contrast.
- Ã¢Å“â€¦ **Main menu contrast tuning**
  - Audio, Rip, and Settings colors refined for legibility.
- Ã¢Å“â€¦ **Main menu layout cleanup**
  - Removed scroll container so the main menu scales without scroll bars.
- Ã¢Å“â€¦ **Player silhouette placeholder**
  - Player pane keeps a stable footprint before media loads.
- Ã¢Å“â€¦ **Main menu palette tuning**
  - Adjusted audio/compare/subtitles colors for better separation.
- Ã¢Å“â€¦ **Main menu vibrancy pass**
  - Removed monochrome tiles outside Settings.
- Ã¢Å“â€¦ **Main menu bespoke hues**
  - Assigned unique hue families to each module for maximum legibility.
- Ã¢Å“â€¦ **Locked tile hue preservation**
  - Disabled modules stay colored while appearing subdued.
- Ã¢Å“â€¦ **Locked hue visibility**
  - Reduced stripe opacity and raised label brightness.

## Version 0.1.0-dev25 (2026-01-22) - Settings Preferences Expansion

### Features
- Ã¢Å“â€¦ **Language & Hardware Acceleration in Settings**
  - Added `Language` string to convertConfig (default: "System").
  - Decoupled benchmark: now only sets HardwareAccel; no codec/preset changes or confirmation dialogs.
  - Implemented Settings > Preferences UI with working selectors:
    - Language dropdown (System/en/es/fr/de/ja/zh) persists to convertConfig.Language.
    - Hardware Acceleration dropdown (auto/none/nvenc/qsv/amf/vaapi/videotoolbox) persists to convertConfig.HardwareAccel.
  - Removed placeholder "Coming soon" text; UI is functional and logical.

### Documentation
- Ã¢Å“â€¦ **TODO.md extended** to track remaining Preferences items (output directories, UI theme, auto-updates, reset/import).
- Ã¢Å“â€¦ **Documentation alignment** - Updated README, module overview, and project status to reflect current implementation and TODO/DONE state.
- Ã¢Å“â€¦ **README technical section** - Added preset codec and frame rate targets.
- Ã¢Å“â€¦ **README balance pass** - Updated capabilities, added status/doc links, and clarified DVD frame rate locking.
- Ã¢Å“â€¦ **Build links** - Added Daily (dev) and Stable (public) build locations to README and docs index.
- Ã¢Å“â€¦ **Build link fix** - Corrected Daily (dev) URL.
- Ã¢Å“â€¦ **Broken link audit** - Fixed internal doc links in README and docs, removed stale placeholders.
- Ã¢Å“â€¦ **Build metadata outputs** - Build scripts now emit zip artifacts and `build.json` metadata per channel and OS.
- Ã¢Å“â€¦ **Build docs update** - Documented `VT_BUILD_CHANNEL` and artifact locations in build/install guides.

### UI/UX
- [x] Module palette contrast - Updated module and queue colors to contrast-friendly palette.

### Maintenance
- [x] Replaced Scoop dependency with MSYS2 toolchain across Windows install/build scripts and docs.

### Windows Install
- [x] GCC preflight failures trigger MSYS2 MinGW-w64 reinstall offers; Scoop toolchains are ignored.
- [x] Added Windows GUI preflight to flag VM/basic display adapters before Fyne startup.
- [x] Windows build script pauses for a keypress on success or failure.
- [x] Removed duplicate GUI startup handler causing build failures.
- [x] Aligned Windows script output headers with Linux styling and printed build metadata.
- [x] Windows build script now refreshes PATH and can auto-repair missing MSYS2 GCC via pacman.
- [x] Standardized Windows build tooling on repo-local MSYS2 UCRT64 with a deterministic provisioner.
- [x] Reorganized scripts into platform-specific folders and removed top-level wrappers.

### Packaging
- [x] Added Forgejo Actions workflow for dev Windows/Linux packaging and artifacts.
- [x] Added Forgejo dev release upload (optional, requires token).
- [x] Added optional EXE signing step for Forgejo dev builds.
- [x] Added self-signed dev cert generator and MSIX signing in Forgejo pipeline.
- [x] Aligned Forgejo runner labels to `ubuntu` and `windows` for active runners.

### Docs
- [x] Removed personal names from documentation in favor of user report/dev report labels.

## Version 0.1.0-dev23 (2026-01-04) - UI Cleanup & About Dialog


### UI/UX
- Ã¢Å“â€¦ **Colored select polish** - one-click dropdown, left accent bar, softer blue-grey background, rounded corners, larger text
- Ã¢Å“â€¦ **Panel input styling** - input and panel backgrounds aligned to dropdown tone
- Ã¢Å“â€¦ **Convert panel buttons** - Auto-crop and interlace actions styled to match settings panel
- Ã¢Å“â€¦ **About / Support redesign** - mockup-aligned layout, VT + LT logos, Logs Folder placement, support placeholder

### Stability
- Ã¢Å“â€¦ **Audio module crash fix** - prevent nil entry panic on initial quality selection

## Version 0.1.0-dev22 (2026-01-01) - Bug Fixes & Documentation

### Bug Fixes
- Ã¢Å“â€¦ **Refactored Command Execution (Windows Console Fix Extended to Core Modules)**
  - Extended the refactoring of command execution to `audio_module.go`, `author_module.go`, and `platform.go`.
  - All direct calls to `exec.Command` and `exec.CommandContext` in these modules now use `utils.CreateCommand` and `utils.CreateCommandRaw`.
  - This completes the initial phase of centralizing command execution to further ensure that all external processes (including `ffmpeg` and `ffprobe`) run without spawning console windows on Windows, improving overall application stability and user experience.

- Ã¢Å“â€¦ **Refactored Command Execution (Windows Console Fix Extended)**
  - Systematically replaced direct calls to `exec.Command` and `exec.CommandContext` across `main.go` and `internal/benchmark/benchmark.go` with `utils.CreateCommand` and `utils.CreateCommandRaw`.
  - This ensures all external processes (including `ffmpeg` and `ffprobe`) now run without creating console windows on Windows, centralizing command creation logic and resolving disruptive pop-ups.

- Ã¢Å“â€¦ **Fixed Console Pop-ups on Windows**
  - Created a centralized utility function (`utils.CreateCommand`) that starts external processes without creating a console window on Windows.
  - Refactored the benchmark module and main application logic to use this new utility.
  - This resolves the issue where running benchmarks or other operations would cause disruptive `ffmpeg.exe` console windows to appear.

### Documentation
- Ã¢Å“â€¦ **Addressed Platform Gaps (Windows Guide)**
  - Created a new, comprehensive installation guide for native Windows (`docs/INSTALL_WINDOWS.md`).
  - Refactored the main `INSTALLATION.md` into a platform-agnostic hub that now links to the separate, detailed guides for Windows and Linux/WSL.
  - This provides a clear, user-friendly path for users on all major platforms.

- Ã¢Å“â€¦ **Aligned Documentation with Reality**
  - Audited and tagged all planned features in the documentation with `[PLANNED]`.
  - This provides a more honest representation of the project's capabilities.
  - Removed broken links from the documentation index.

- Ã¢Å“â€¦ **Created Project Status Page**
  - Created `docs/PROJECT_STATUS.md` to provide a single source of truth for project status.
  - Summarizes implemented, planned, and in-progress features.
  - Highlights critical known issues, like the player module bugs.
  - Linked from the main `README.md` to ensure users and developers have a clear, honest overview of the project's state.

This file tracks completed features, fixes, and milestones.

## Version 0.1.0-dev20+ (2025-12-28) - Queue UI Performance & Workflow Improvements

### Bug Fixes
- Ã¢Å“â€¦ **Player Module Investigation**
  - Investigated reported player crash
  - Discovered player is ALREADY fully internal and lightweight
  - Uses FFmpeg directly (no external VLC/MPV/FFplay dependencies)
  - Implementation: FFmpeg pipes raw frames + audio Ã¢â€ â€™ Oto library for output
  - Frame-accurate seeking and A/V sync built-in
  - Error handling: Falls back to video-only playback if audio fails
  - Player module re-enabled - follows VideoTools' core principles

### Workflow Enhancements
- Ã¢Å“â€¦ **Benchmark Result Caching**
  - Benchmark results now persist across app restarts
  - Opening Benchmark module shows cached results instead of auto-running
  - Clear timestamp display (e.g., "Showing cached results from December 28, 2025 at 2:45 PM")
  - "Run New Benchmark" button available when viewing cached results
  - Auto-runs only when no previous results exist or hardware has changed (GPU detection)
  - Saves to `~/.config/VideoTools/benchmark.json` with last 10 runs in history
  - No more redundant benchmarks every time you open the module

- Ã¢Å“â€¦ **Merge Module Output Path UX Improvement**
  - Split single output path field into separate folder and filename fields
  - "Output Folder" field with "Browse Folder" button for directory selection
  - "Output Filename" field for easy filename editing (e.g., "merged.mkv")
  - No more navigating through long paths to change filenames
  - Cleaner, more intuitive interface following standard file dialog patterns
  - Auto-population sets directory and filename independently

- Ã¢Å“â€¦ **Queue Priority System for Convert Now**
  - "Convert Now" during active conversions adds job to top of queue (after running job)
  - "Add to Queue" continues to add to end as expected
  - Implemented AddNext() method in queue package for priority insertion
  - User feedback message indicates queue position: "Added to top of queue!" vs "Conversion started!"
  - Better workflow when adding files during active batch conversions

- Ã¢Å“â€¦ **Auto-Cleanup for Failed Conversions**
  - Convert jobs now automatically delete incomplete/broken output files on failure
  - Success tracking ensures complete files are never removed
  - Prevents accumulation of partial files from crashed/cancelled conversions
  - Cleaner disk space management and error handling

- Ã¢Å“â€¦ **Queue List Jankiness Reduction**
  - Increased auto-refresh interval from 1000ms to 2000ms for smoother updates
  - Reduced scroll restoration delay from 50ms to 10ms for faster position recovery
  - Fixed race condition in scroll offset saving
  - Eliminated visible jumping during queue view rebuilds

### Performance Optimizations
- Ã¢Å“â€¦ **Queue View Button Responsiveness**
  - Fixed Windows-specific button lag after conversion completion
  - Eliminated redundant UI refreshes in queue button handlers (Pause, Resume, Cancel, Remove, Move Up/Down, etc.)
  - Queue onChange callback now handles all refreshes automatically - removed duplicate manual calls
  - Added stopQueueAutoRefresh() before navigation to prevent conflicting UI updates
  - Result: Instant button response on Windows (was 1-3 second lag)
  - Reported by: user report

- Ã¢Å“â€¦ **Main Menu Performance**
  - Fixed main menu lag when sidebar visible and queue active
  - Implemented 300ms throttling for main menu rebuilds (prevents excessive redraws)
  - Cached jobQueue.List() calls to eliminate multiple expensive copies (was 2-3 copies per refresh)
  - Smart conditional refresh: only rebuild sidebar when history actually changes
  - Result: 3-5x improvement in main menu responsiveness, especially on Windows
  - RAM usage confirmed: 220MB (lean and efficient for video processing app)

- Ã¢Å“â€¦ **Queue Auto-Refresh Optimization**
  - Reduced auto-refresh interval from 500ms to 1000ms (1 second)
  - Reduces UI thread pressure on Windows while maintaining smooth progress updates
  - Combined with 500ms manual throttle in refreshQueueView() for optimal balance

### User Experience Improvements
- Ã¢Å“â€¦ **Benchmark UI Cleanup**
  - Hide benchmark indicator in Convert module when settings are already applied
  - Only show "Benchmark: Not Applied" status when action is needed
  - Removes clutter from UI when using benchmark settings
  - Cleaner interface for active conversions with benchmark recommendations

- Ã¢Å“â€¦ **Queue Position Labeling**
  - Fixed confusing priority display in queue view
  - Changed from internal priority numbers (3, 2, 1) to user-friendly queue positions (1, 2, 3)
  - Now displays "Queue Position: 1" for first job, "Queue Position: 2" for second, etc.
  - Applied to both Pending and Paused jobs
  - Much clearer for users to understand execution order

### Remux Safety System (Fool-Proof Implementation)
- Ã¢Å“â€¦ **Comprehensive Codec Compatibility Validation**
  - Added validateRemuxCompatibility() function with format-specific checks
  - Automatically detects incompatible codec/container combinations
  - Validates before ANY remux operation to prevent silent failures

- Ã¢Å“â€¦ **Container-Specific Validation**
  - MP4: Blocks VP8, VP9, AV1, Theora, Vorbis, Opus (not reliably supported)
  - MKV: Allows almost everything (ultra-flexible)
  - WebM: Enforces VP8/VP9/AV1 video + Vorbis/Opus audio only
  - MOV: Apple-friendly codecs (H.264, H.265, ProRes, MJPEG)

- Ã¢Å“â€¦ **Automatic Fallback to Re-encoding**
  - WMV/ASF sources automatically re-encode (timestamp/codec issues)
  - FLV with legacy codecs (Sorenson/VP6) auto re-encode
  - Incompatible codec/container pairs auto re-encode to safe default (H.264)
  - User never gets broken files - system handles it transparently

- Ã¢Å“â€¦ **Auto-Fixable Format Detection**
  - AVI: Applies -fflags +genpts for timestamp regeneration
  - FLV (H.264): Applies timestamp fixes
  - MPEG-TS/M2TS/MTS: Extended analysis + timestamp fixes
  - VOB (DVD rips): Full timestamp regeneration
  - All apply -avoid_negative_ts make_zero automatically

- Ã¢Å“â€¦ **Enhanced FFmpeg Safety Flags**
  - All remux operations now include:
    - `-fflags +genpts` (regenerate timestamps)
    - `-avoid_negative_ts make_zero` (fix negative timestamps)
    - `-map 0` (preserve all streams)
    - `-map_chapters 0` (preserve chapters)
  - MPEG-TS sources get extended analysis parameters
  - Result: Robust, reliable remuxing with zero risk of corruption

- Ã¢Å“â€¦ **Codec Name Normalization**
  - Added normalizeCodecName() to handle codec name variations
  - Maps h264/avc/avc1/h.264/x264 Ã¢â€ â€™ h264
  - Maps h265/hevc/h.265/x265 Ã¢â€ â€™ h265
  - Maps divx/xvid/mpeg-4 Ã¢â€ â€™ mpeg4
  - Ensures accurate validation regardless of FFprobe output variations

### Technical Improvements
- Ã¢Å“â€¦ **Smart UI Update Strategy**
  - Throttled refreshes prevent cascading rebuilds
  - Conditional updates only when state actually changes
  - Queue list caching eliminates redundant memory allocations
  - Windows-optimized rendering pipeline

- Ã¢Å“â€¦ **Debug Logging**
  - Added comprehensive logging for remux compatibility decisions
  - Clear messages when auto-fixing vs auto re-encoding
  - Helps debugging and user understanding

## Version 0.1.0-dev20+ (2025-12-26) - Author Module & UI Enhancements

### Features
- Ã¢Å“â€¦ **Author Module - Real-time Progress Reporting**
  - Implemented granular progress updates for FFmpeg encoding steps in the Author module.
  - Progress bar now updates smoothly during video processing, providing better feedback.
  - Weighted progress calculation based on video durations for accurate overall progress.

- Ã¢Å“â€¦ **Author Module - "Add to Queue" & Output Title Clear**
  - Added an "Add to Queue" button to the Author module for non-immediate job execution.
  - Refactored authoring workflow to support queuing jobs via a `startNow` parameter.
  - Modified "Clear All" functionality to also clear the DVD Output Title, preventing naming conflicts.

- Ã¢Å“â€¦ **Main Menu - "Disc" Category for Author, Rip, and Blu-Ray**
  - Relocated "Author", "Rip", and "Blu-Ray" buttons to a new "Disc" category on the main menu.
  - Improved logical grouping of disc-related functionalities.

- Ã¢Å“â€¦ **Subtitles Module - Video File Path Population**
  - Fixed an issue where dragging and dropping a video file onto the Subtitles module would not populate the "Video File Path" section.
  - Ensured the video entry widget correctly reflects the dropped video's path.

## Version 0.1.0-dev20+ (2025-12-23) - Player UX & Installer Polish

### Features (2025-12-23 Session)
- Ã¢Å“â€¦ **Player Module UI Improvements**
  - Responsive video player sizing based on screen resolution
  - Screens < 1600px wide: 640x360 (prevents layout breaking)
  - Screens Ã¢â€°Â¥ 1600px wide: 1280x720 (larger viewing area)
  - Dynamically adapts to display when player view is built
  - Prevents excessive negative space on lower resolution displays

- Ã¢Å“â€¦ **Main Menu Cleanup**
  - Hidden "Logs" button from main menu (history sidebar replaces it)
  - Logs button only appears when onLogsClick callback is provided
  - Cleaner, less cluttered interface
  - Dynamic header controls based on available functionality

- Ã¢Å“â€¦ **Windows Installer Fix**
  - Fixed DVDStyler download from SourceForge mirrors
  - Added `-MaximumRedirection 10` to handle SourceForge redirects
  - Added browser user agent to prevent rejection
  - Resolves "invalid archive" error on Windows 11
  - Reported by: user report

### Technical Improvements
- Ã¢Å“â€¦ **Responsive Design Pattern**
  - Canvas size detection for adaptive UI sizing
  - Prevents window layout issues on smaller displays
  - Maintains larger preview on high-resolution screens

- Ã¢Å“â€¦ **PowerShell Download Robustness**
  - Proper redirect following for mirror systems
  - User agent spoofing for compatibility
  - Multiple fallback URLs for resilience

## Version 0.1.0-dev20 (2025-12-21) - VT_Player Framework Implementation

### Features (2025-12-21 Session)
- Ã¢Å“â€¦ **VT_Player Module - Complete Framework Implementation**
  - **Frame-Accurate Video Player Interface** (`internal/player/vtplayer.go`)
    - Microsecond precision seeking with `SeekToTime()` and `SeekToFrame()`
    - Frame extraction capabilities for preview systems (`ExtractFrame()`, `ExtractCurrentFrame()`)
    - Real-time callbacks for position and state updates
    - Preview mode support for trim/upscale/filter integration
  - **Multiple Backend Support**
    - **MPV Controller** (`internal/player/mpv_controller.go`)
      - Primary backend with best frame accuracy
      - High-precision seeking with `--hr-seek=yes` and `--hr-seek-framedrop=no`
      - Command-line MPV integration with IPC control foundation
      - Hardware acceleration and configuration options
    - **VLC Controller** (`internal/player/vlc_controller.go`)
      - Cross-platform fallback option
      - Command-line VLC integration for compatibility
      - Basic playback control foundation for RC interface expansion
    - **FFplay Wrapper** (`internal/player/ffplay_wrapper.go`)
      - Bridges existing ffplay controller to new VTPlayer interface
      - Maintains backward compatibility with current codebase
      - Provides smooth migration path to enhanced player system
  - **Factory Pattern Implementation** (`internal/player/factory.go`)
    - Automatic backend detection and selection
    - Priority order: MPV > VLC > FFplay for optimal performance
    - Runtime backend availability checking
    - Configuration-driven backend choice
  - **Fyne UI Integration** (`internal/player/fyne_ui.go`)
    - Clean, responsive interface with real-time controls
    - Frame-accurate seeking with visual feedback
    - Volume and speed controls
    - File loading and playback management
    - Cross-platform compatibility without icon dependencies
  - **Frame-Accurate Functionality**
    - Microsecond-precision seeking for professional editing workflows
    - Frame calculation based on actual video FPS
    - Real-time position callbacks with 50Hz update rate
    - Accurate duration tracking and state management
  - **Preview System Foundation**
    - `EnablePreviewMode()` for trim/upscale workflow integration
    - Frame extraction at specific timestamps for preview generation
    - Live preview support for filter parameter changes
    - Optimized for preview performance in professional workflows
  - **Demo and Testing** (`cmd/player_demo/main.go`)
    - Working demonstration of VT_Player capabilities
    - Backend detection and selection validation
    - Frame-accurate method testing
    - Integration example for other modules

### Technical Implementation Details
- **Cross-Platform Backend Support**: Command-line integration for MPV/VLC with future IPC expansion
- **Frame Accuracy**: Microsecond precision timing with time.Duration throughout
- **Error Handling**: Graceful fallbacks and comprehensive error reporting
- **Resource Management**: Proper process cleanup and context cancellation
- **Interface Design**: Clean separation between UI and playback engine
- **Future Extensibility**: Foundation for enhanced IPC control and additional backends

### Integration Points
- **Trim Module**: Frame-accurate preview of cut points and timeline navigation
- **Upscale Module**: Real-time preview with live parameter updates
- **Filters Module**: Frame-by-frame comparison and live effect preview
- **Convert Module**: Video loading and preview integration

### Documentation
- Ã¢Å“â€¦ Created comprehensive implementation documentation (`docs/VT_PLAYER_IMPLEMENTATION.md`)
- Ã¢Å“â€¦ Documented architecture decisions and backend selection logic
- Ã¢Å“â€¦ Provided integration examples for module developers
- Ã¢Å“â€¦ Outlined future enhancement roadmap

## Version 0.1.0-dev20 (2025-12-18 to 2025-12-20) - Convert Module Cleanup & UX Polish

### Features (2025-12-20 Session)
- Ã¢Å“â€¦ **History Sidebar - In Progress Tab**
  - Added "In Progress" tab to history sidebar
  - Shows running and pending jobs without opening queue
  - Animated striped progress bars per module color
  - Real-time progress updates (0-100%)
  - No delete button on active jobs (only completed/failed)
  - Dynamic status text ("Running..." or "Pending")

- Ã¢Å“â€¦ **Benchmark System Overhaul**
  - **Hardware Detection Module** (`internal/sysinfo/sysinfo.go`)
    - Cross-platform CPU detection (model, cores, clock speed)
    - GPU detection with driver version (NVIDIA via nvidia-smi)
    - RAM detection with human-readable formatting
    - Linux and Windows support
  - **Hardware Info Display**
    - Shown immediately in benchmark progress view (before tests run)
    - Displayed in benchmark results view
    - Saved with each benchmark run for history
  - **Settings Persistence**
    - Hardware acceleration settings saved with benchmarks
    - Settings persist between sessions via config file
    - GPU automatically detected and used
  - **UI Polish**
    - "Run Benchmark" button highlighted (HighImportance) on first run
    - Returns to normal styling after initial benchmark
  - Guides new users to run initial benchmark

- Ã¢Å“â€¦ **AI Upscale Integration (Real-ESRGAN)**
  - Added model presets with anime/general variants
  - Processing presets (Ultra Fast Ã¢â€ â€™ Maximum Quality) with tile/TTA tuning
  - Upscale factor selection + output adjustment slider
  - Tile size, output frame format, GPU and thread controls
  - ncnn backend pipeline (extract Ã¢â€ â€™ AI upscale Ã¢â€ â€™ reassemble)
  - Filters and frame rate conversion applied before AI upscaling

- Ã¢Å“â€¦ **Bitrate Preset Simplification**
  - Reduced from 13 confusing options to 6 clear presets
  - Removed resolution references (no more "1440p" confusion)
  - Codec-agnostic (presets don't change selected codec)
  - Quality-based naming: Low/Medium/Good/High/Very High Quality
  - Focused on common use cases (1.5-8 Mbps range)
  - Presets only set bitrate and switch to CBR mode
  - User codec choice (H.264, VP9, AV1, etc.) preserved

- Ã¢Å“â€¦ **Quality Preset Codec Compatibility**
  - "Lossless" quality option only available for H.265 and AV1
  - Dynamic quality dropdown based on selected codec
  - Automatic fallback to "Near-Lossless" when switching to non-lossless codec
  - Lossless + Target Size bitrate mode now supported for H.265/AV1
  - Prevents invalid codec/quality combinations

- Ã¢Å“â€¦ **App Icon Improvements**
  - Regenerated VT_Icon.ico with transparent background
  - Updated LoadAppIcon() to search PNG first (better Linux support)
  - Searches both current directory and executable directory
  - Added debug logging for icon loading troubleshooting

- Ã¢Å“â€¦ **UI Scaling for 800x600 Windows** (2025-12-20 continuation)
  - Reduced module tile size from 220x110 to 150x65
  - Reduced title text size from 28 to 18
  - Reduced queue tile from 160x60 to 120x40
  - Reduced section padding from 14 to 4 pixels
  - Reduced category labels to 12px
  - Removed extra padding wrapper around tiles
  - Removed scrolling requirement - everything fits without scrolling
  - All UI elements fit within 800x600 default window

- Ã¢Å“â€¦ **Header Layout Improvements** (2025-12-20 continuation)
  - Changed from HBox with spacer to border layout
  - Title on left, all controls grouped compactly on right
  - Shortened button labels for space efficiency
  - "Ã¢ËœÂ° History" Ã¢â€ â€™ "Ã¢ËœÂ°", "Run Benchmark" Ã¢â€ â€™ "Benchmark", "View Results" Ã¢â€ â€™ "Results"
  - Eliminates wasted horizontal space

- Ã¢Å“â€¦ **Queue Clear Behavior Fix** (2025-12-20 continuation)
  - "Clear Completed" now always returns to main menu
  - "Clear All" now always returns to main menu
  - Prevents unwanted navigation to convert module after clearing queue
  - Consistent and predictable behavior

- Ã¢Å“â€¦ **Threading Safety Fix** (2025-12-20 continuation)
  - Fixed Fyne threading errors in stats bar component
  - Removed Show()/Hide() calls from Layout() method
  - Layout() can be called from any thread during resize/redraw
  - Show/Hide logic remains only in Refresh() with proper DoFromGoroutine
  - Eliminates threading warnings during UI updates

- Ã¢Å“â€¦ **Preset UX Improvements** (2025-12-20 continuation)
  - Moved "Manual" option to bottom of all preset dropdowns
  - Bitrate preset default: "2.5 Mbps - Medium Quality"
  - Target size preset default: "100MB"
  - Manual input fields hidden by default
  - Manual fields appear only when "Manual" is selected
  - Encourages preset usage while maintaining advanced control
  - Reversed encoding preset order: veryslow first, ultrafast last
  - Better quality options now appear at top of list
  - Applied consistently to both simple and advanced modes

- Ã¢Å“â€¦ **Audio Channel Remixing** (2025-12-20 continuation)
  - Added advanced audio channel options for videos with imbalanced L/R channels
  - New options using FFmpeg pan filter:
    - "Left to Stereo" - Copy left channel to both speakers (music only)
    - "Right to Stereo" - Copy right channel to both speakers (vocals only)
    - "Mix to Stereo" - Downmix both channels together evenly
    - "Swap L/R" - Swap left and right channels
  - Implemented in all 4 command builders (DVD, convert, snippet)
  - Maintains existing options (Source, Mono, Stereo, 5.1)
  - Solves problem of videos with music in one ear and vocals in the other

- Ã¢Å“â€¦ **Author Module Skeleton** (2025-12-20 continuation)
  - Renamed "DVD Author" module to "Author" for broader scope
  - Created tabbed interface structure with 3 tabs:
    - **Chapters Tab** - Scene detection and chapter management
    - **Rip DVD/ISO Tab** - High-quality disc extraction (like FLAC from CD)
    - **Author Disc Tab** - VIDEO_TS/ISO creation for burning
  - Implemented basic Chapters tab UI:
    - File selection with video probing
    - Scene detection sensitivity slider (0.1-0.9 threshold)
    - Placeholder chapter list
    - Add/Export chapter buttons (to be implemented)
  - Added authorChapter struct for storing chapter data
  - Added author module state fields to appState
  - Foundation for complete disc production workflow

- Ã¢Å“â€¦ **Real-ESRGAN Automated Setup** (2025-12-20 continuation)
  - Created automated setup script for Linux (setup-realesrgan-linux.sh)
  - One-command installation: downloads, installs, configures
  - Installs binary to ~/.local/bin/realesrgan-ncnn-vulkan
  - Installs all AI models to ~/.local/share/realesrgan/models/ (45MB)
  - Includes 5 model sets: animevideov3, x4plus, x4plus-anime
  - Sets proper permissions and provides PATH setup instructions
  - Makes AI upscaling fully automated for users
  - No manual downloads or configuration needed

- Ã¢Å“â€¦ **Window Auto-Resize Fix** (2025-12-20 continuation)
  - Fixed window resizing itself when content changes
  - Window now maintains user-set size through all content updates
  - Progress bars and queue updates no longer trigger window resize
  - Preserved window size before/after SetContent() calls
  - User retains full control via manual resize or maximize
  - Improves professional appearance and stability
  - Reported by: user report

### Features (2025-12-18 Session)
- Ã¢Å“â€¦ **History Sidebar Enhancements**
  - Delete button ("Ãƒâ€”") on each history entry
  - Remove individual entries from history
  - Auto-save and refresh after deletion
  - Clean, unobtrusive button placement

- Ã¢Å“â€¦ **Command Preview Improvements**
  - Show/Hide button state based on preview visibility
  - Disabled when no video source loaded
  - Displays actual file paths instead of placeholders
  - Real-time live updates as settings change
  - Collapsible to save screen space

- Ã¢Å“â€¦ **Format Options Reorganization**
  - Grouped by codec family (H.264 Ã¢â€ â€™ H.265 Ã¢â€ â€™ AV1 Ã¢â€ â€™ VP9 Ã¢â€ â€™ ProRes Ã¢â€ â€™ MPEG-2)
  - Added descriptive comments for each codec type
  - Improved dropdown readability and navigation
  - Easier to find and compare similar formats

- Ã¢Å“â€¦ **Bitrate Mode Clarity**
  - Descriptive labels in dropdown:
    - CRF (Constant Rate Factor)
    - CBR (Constant Bitrate)
    - VBR (Variable Bitrate)
    - Target Size (Calculate from file size)
  - Immediate understanding without documentation
  - Preserves internal compatibility with short codes

- Ã¢Å“â€¦ **Root Folder Cleanup**
  - Moved all documentation .md files to docs/ folder
  - Kept only README.md, TODO.md, DONE.md in root
  - Cleaner project structure
  - Better organization for contributors

### Bug Fixes
- Ã¢Å“â€¦ **Critical Convert Module Crash Fixed**
  - Fixed nil pointer dereference when opening Convert module
  - Corrected widget initialization order
  - bitrateContainer now created after bitratePresetSelect initialized
  - Eliminated "invalid memory address" panic on startup

- Ã¢Å“â€¦ **Log Viewer Crash Fixed**
  - Fixed "close of closed channel" panic
  - Duplicate close handlers removed
  - Proper dialog cleanup

- Ã¢Å“â€¦ **Bitrate Control Improvements**
  - CBR: Set bufsize to 2x bitrate for better encoder handling
  - VBR: Increased maxrate cap from 1.5x to 2x target bitrate
  - VBR: Added bufsize at 4x target to enforce caps
  - Prevents runaway bitrates while maintaining quality peaks

### Technical Improvements
- Ã¢Å“â€¦ **Widget Initialization Order**
  - Fixed container creation dependencies
  - All Select widgets initialized before container use
  - Proper nil checking in UI construction

- Ã¢Å“â€¦ **Bidirectional Label Mapping**
  - Display labels map to internal storage codes
  - Config files remain compatible
  - Clean separation of UI and data layers

## Version 0.1.0-dev18 (2025-12-15)

### Features
- Ã¢Å“â€¦ **Thumbnail Module Enhancements**
  - Enhanced metadata display with 3 lines of comprehensive technical data
  - Added 8px padding between thumbnails in contact sheets
  - Increased thumbnail width to 280px for analyzable screenshots (4x8 grid = ~1144x1416)
  - Audio bitrate display alongside audio codec (e.g., "AAC 192kbps")
  - Concise bitrate display (removed "Total:" prefix)
  - Video codec, audio codec, FPS, and overall bitrate shown in metadata
  - Navy blue background (#0B0F1A) for professional appearance

- Ã¢Å“â€¦ **Player Module**
  - New Player button on main menu (Teal #44FFDD)
  - Access to VT_Player for video playback
  - Video loading and preview integration
  - Module handler for CLI support

- Ã¢Å“â€¦ **Filters Module - UI Complete**
  - Color correction controls (brightness, contrast, saturation)
  - Enhancement tools (sharpness, denoise)
  - Transform operations (rotation, flip horizontal/vertical)
  - Creative effects (grayscale)
  - Navigation to Upscale module with video transfer
  - Full state management for filter settings

- Ã¢Å“â€¦ **Upscale Module - Fully Functional**
  - Traditional FFmpeg scaling methods: Lanczos (sharp), Bicubic (smooth), Spline (balanced), Bilinear (fast)
  - Resolution presets: 720p, 1080p, 1440p, 4K, 8K
  - "UPSCALE NOW" button for immediate processing
  - "Add to Queue" button for batch processing
  - Job queue integration with real-time progress tracking
  - AI upscaling detection (Real-ESRGAN) with graceful fallback
  - High quality encoding (libx264, preset slow, CRF 18)
  - Navigation back to Filters module

- Ã¢Å“â€¦ **Snippet System Overhaul - Dual Output Modes**
  - **"Snippet to Default Format" (Checkbox CHECKED - Default)**:
    - Stream copy mode preserves exact source format, codec, bitrate
    - Zero quality loss - bit-perfect copy of source
    - Outputs to source container (.wmv Ã¢â€ â€™ .wmv, .avi Ã¢â€ â€™ .avi, etc.)
    - Fast processing (no re-encoding)
    - Duration: Keyframe-level precision (may vary Ã‚Â±1-2s)
    - Perfect for merge testing without quality changes
  - **"Snippet to Output Format" (Checkbox UNCHECKED)**:
    - Uses configured conversion settings from Convert tab
    - Applies video codec (H.264, H.265, VP9, AV1, etc.)
    - Applies audio codec (AAC, Opus, MP3, FLAC, etc.)
    - Uses encoder preset and CRF quality settings
    - Outputs to selected format (.mp4, .mkv, .webm, etc.)
    - Frame-perfect duration control (exactly configured length)
    - Perfect preview of final conversion output

- Ã¢Å“â€¦ **Configurable Snippet Length**
  - Adjustable snippet length (5-60 seconds, default: 20)
  - Slider control with real-time display
  - Snippets centered on video midpoint
  - Length persists across video loads

- Ã¢Å“â€¦ **Batch Snippet Generation**
  - "Generate All Snippets" button for multiple loaded videos
  - Processes all videos with same configured length
  - Consistent timestamp for uniform naming
  - Efficient queue integration
  - Shows confirmation with count of jobs added

- Ã¢Å“â€¦ **Smart Job Descriptions**
  - Displays snippet length and mode in job queue
  - "10s snippet centred on midpoint (source format)"
  - "20s snippet centred on midpoint (conversion settings)"

### Technical Improvements
- Ã¢Å“â€¦ **Dual-Mode Snippet System Implementation**
  - Default Format mode: Stream copy for bit-perfect source preservation
  - Output Format mode: Full conversion using user's configured settings
  - Automatic container/codec matching based on mode selection
  - Integration with conversion config (video/audio codecs, presets, CRF)
  - Smart extension handling (source format vs. selected output format)
- Ã¢Å“â€¦ **Queue/Status UI polish**
  - Animated striped progress bars per module color with faster motion for visibility
  - Footer refactor: consistent dark status strip + tinted action bar across modules
  - Status bar tap restored to open Job Queue; full-width clickable strip
- Ã¢Å“â€¦ **Snippet progress reporting**
  - Live progress from ffmpeg `-progress` output; 0Ã¢â‚¬â€œ100% updates in status bar and queue
  - Error/log capture preserved for snippet jobs

- Ã¢Å“â€¦ **Metadata Enhancement System**
  - New `getDetailedVideoInfo()` function using FFprobe
  - Extracts video codec, audio codec, FPS, video bitrate, audio bitrate
  - Multiple ffprobe calls for comprehensive data
  - Graceful fallback to format-level bitrate if stream bitrate unavailable

- Ã¢Å“â€¦ **Module Navigation Pattern**
  - Bidirectional navigation between Filters and Upscale
  - Video file transfer between modules
  - Filter chain transfer capability (foundation for future)

- Ã¢Å“â€¦ **Resolution Parsing System**
  - `parseResolutionPreset()` function for preset strings
  - Maps "1080p (1920x1080)" format to width/height integers
  - Support for custom resolution input (foundation)

- Ã¢Å“â€¦ **Upscale Filter Builder**
  - `buildUpscaleFilter()` constructs FFmpeg scale filters
  - Method-specific scaling: lanczos, bicubic, spline, bilinear
  - Filter chain combination support

### Bug Fixes
- Ã¢Å“â€¦ Fixed incorrect thumbnail count in contact sheets (was generating 34 instead of 40 for 5x8 grid)
- Ã¢Å“â€¦ Fixed frame selection FPS assumption (hardcoded 30fps removed)
- Ã¢Å“â€¦ Fixed module visibility (added thumb module to enabled check)
- Ã¢Å“â€¦ Fixed undefined function call (openFileManager Ã¢â€ â€™ openFolder)
- Ã¢Å“â€¦ Fixed dynamic total count not updating when changing grid dimensions
- Ã¢Å“â€¦ Added missing `strings` import to thumbnail/generator.go
- Ã¢Å“â€¦ Updated snippet UI labels for clarity (Default Format vs Output Format)

### Documentation
- Ã¢Å“â€¦ Updated ai-speak.md with comprehensive dev18 documentation
- Ã¢Å“â€¦ Created 24-item testing checklist for dev18
- Ã¢Å“â€¦ Documented all implementation details and technical decisions

## Version 0.1.0-dev17 (2025-12-14)

### Features
- Ã¢Å“â€¦ **Thumbnail Module - Complete Implementation**
  - Individual thumbnail generation with customizable count (3-50 thumbnails)
  - Contact sheet generation with metadata headers
  - Customizable grid layouts (2-12 columns, 2-12 rows)
  - Even timestamp distribution across video duration
  - JPEG output with configurable quality (default: 85)
  - Configurable thumbnail width (160-640px for individual, 200px for contact sheets)
  - Saves to `{video_directory}/{video_name}_thumbnails/` for easy access
  - DejaVu Sans Mono font matching app styling
  - App background color (#0B0F1A) for contact sheet padding
  - Dynamic total count display for grid layouts

- Ã¢Å“â€¦ **Thumbnail UI Integration**
  - Video preview window (640x360) in thumbnail module
  - Mode-specific controls (contact sheet: columns/rows, individual: count/width)
  - Dual button system:
    - "GENERATE NOW" - Adds to queue and starts immediately
    - "Add to Queue" - Adds for batch processing
  - "View Results" button with in-app contact sheet viewer (900x700 dialog)
  - "View Queue" button for queue access from thumbnail module
  - Drag-and-drop support for video files (universal across app)
  - Real-time grid total calculation as columns/rows change

- Ã¢Å“â€¦ **Job Queue Integration for Thumbnails**
  - Background thumbnail generation with progress tracking
  - Job queue support with live progress updates
  - Can queue multiple thumbnail jobs from different videos
  - Progress callback integration for thumbnail extraction
  - Proper context cancellation support

- Ã¢Å“â€¦ **Snippet Tool Improvement**
  - Changed from re-encoding to stream copy (`-c copy`)
  - Instant 20-second snippet extraction with zero quality loss
  - No encoding overhead - extracts source streams directly
  - Removed 148 lines of unnecessary encoding logic

### Technical Improvements
- Ã¢Å“â€¦ **Timestamp-based Frame Selection**
  - Fixed frame selection from FPS-dependent (`eq(n,frame_num)`) to timestamp-based (`gte(t,timestamp)`)
  - Ensures correct thumbnail count regardless of video frame rate
  - Works reliably with VFR (Variable Frame Rate) content
  - Uses `setpts=N/TB` for proper timestamp reset in contact sheets

- Ã¢Å“â€¦ **FFmpeg Filter Optimization**
  - Tile filter for grid layouts: `tile=COLUMNSxROWS`
  - Select filter with timestamp-based frame extraction
  - Pad filter with hex color codes for app background matching
  - Drawtext filter with font specification and positioning
  - Scale filter maintaining aspect ratios

- Ã¢Å“â€¦ **Module Architecture**
  - Added thumbnail state fields to appState (thumbFile, thumbCount, thumbWidth, thumbContactSheet, thumbColumns, thumbRows, thumbLastOutputPath)
  - Implemented `showThumbView()` for thumbnail module UI
  - Implemented `buildThumbView()` for split layout (preview 55%, settings 45%)
  - Implemented `executeThumbJob()` for job queue integration
  - Universal drag-and-drop handler for all modules

- Ã¢Å“â€¦ **Error Handling**
  - Disabled timestamp overlay on individual thumbnails to avoid font availability issues
  - Graceful handling of missing output directories
  - Proper error dialogs with context-specific messages
  - Exit status 234 resolution (font-related errors)

### Bug Fixes
- Ã¢Å“â€¦ Fixed incorrect thumbnail count in contact sheets (was generating 34 instead of 40 for 5x8 grid)
- Ã¢Å“â€¦ Fixed frame selection FPS assumption (hardcoded 30fps removed)
- Ã¢Å“â€¦ Fixed module visibility (added thumb module to enabled check)
- Ã¢Å“â€¦ Fixed undefined function call (openFileManager Ã¢â€ â€™ openFolder)
- Ã¢Å“â€¦ Fixed dynamic total count not updating when changing grid dimensions
- Ã¢Å“â€¦ Fixed font-related crash on systems without DejaVu Sans Mono

## Version 0.1.0-dev16 (2025-12-14)

### Features
- Ã¢Å“â€¦ **Interlacing Detection Module - Complete Implementation**
  - Automatic interlacing analysis using FFmpeg idet filter
  - Field order detection (TFF - Top Field First, BFF - Bottom Field First)
  - Frame-by-frame analysis with classifications:
    - Progressive frames
    - Top Field First interlaced frames
    - Bottom Field First interlaced frames
    - Undetermined frames
  - Interlaced percentage calculation
  - Status determination: Progressive (<5%), Interlaced (>95%), Mixed Content (5-95%)
  - Confidence levels: High (<5% undetermined), Medium (5-15%), Low (>15%)
  - Quick analyze mode (500 frames) for fast detection
  - Full video analysis option for comprehensive results

- Ã¢Å“â€¦ **Deinterlacing Recommendations**
  - Automatic deinterlacing recommendations based on analysis
  - Suggested filter selection (yadif for compatibility)
  - Human-readable recommendations
  - SuggestDeinterlace boolean flag for programmatic use

- Ã¢Å“â€¦ **Preview Generation**
  - Deinterlace preview at specific timestamps
  - Side-by-side comparison (original vs deinterlaced)
  - Uses yadif filter for preview generation
  - Frame extraction with proper scaling

### Technical Improvements
- Ã¢Å“â€¦ **Detector Implementation**
  - Created `/internal/interlace/detector.go` package
  - NewDetector() constructor accepting ffmpeg and ffprobe paths
  - Analyze() method with configurable sample frame count
  - QuickAnalyze() convenience method for 500-frame sampling
  - Regex-based parsing of idet filter output
  - Multi-frame detection statistics extraction

- Ã¢Å“â€¦ **Detection Result Structure**
  - Comprehensive DetectionResult type with all metrics
  - String() method for formatted output
  - Percentage calculations for interlaced content
  - Field order determination logic
  - Confidence calculation based on undetermined ratio

- Ã¢Å“â€¦ **FFmpeg Integration**
  - idet filter integration for interlacing detection
  - Proper stderr pipe handling for filter statistics
  - Context-aware command execution with cancellation support
  - Null output format for analysis-only operations

### Documentation
- Ã¢Å“â€¦ Added interlacing detection to module list
- Ã¢Å“â€¦ Documented detection algorithms and thresholds
- Ã¢Å“â€¦ Explained field order types and their implications

## Version 0.1.0-dev13 (In Progress - 2025-12-03)

### Features
- Ã¢Å“â€¦ **Automatic Black Bar Detection and Cropping**
  - Detects and removes black bars to reduce file size (15-30% typical reduction)
  - One-click "Detect Crop" button analyzes video using FFmpeg cropdetect
  - Samples 10 seconds from middle of video for stable detection
  - Shows estimated file size reduction percentage before applying
  - User confirmation dialog displays before/after dimensions
  - Manual crop override capability (width, height, X/Y offsets)
  - Applied before scaling for optimal results
  - Works in both direct convert and queue job execution
  - Proper handling for videos without black bars
  - 30-second timeout protection for detection process

- Ã¢Å“â€¦ **Frame Rate Conversion UI with Size Estimates**
  - Comprehensive frame rate options: Source, 23.976, 24, 25, 29.97, 30, 50, 59.94, 60
  - Intelligent file size reduction estimates (40-50% for 60Ã¢â€ â€™30 fps)
  - Real-time hints showing "Converting X Ã¢â€ â€™ Y fps: ~Z% smaller file"
  - Warning for upscaling attempts with judder notice
  - Automatic calculation based on source and target frame rates
  - Dynamic updates when video or frame rate changes
  - Supports both film (24 fps) and broadcast standards (25/29.97/30)
  - Uses FFmpeg fps filter for frame rate conversion

- Ã¢Å“â€¦ **Encoder Preset Descriptions with Speed/Quality Trade-offs**
  - Detailed information for all 9 preset options
  - Speed comparisons relative to "slow" and "medium" baselines
  - File size impact percentages for each preset
  - Visual icons indicating speed categories (Ã¢Å¡Â¡Ã¢ÂÂ©Ã¢Å¡â€“Ã¯Â¸ÂÃ°Å¸Å½Â¯Ã°Å¸ÂÅ’)
  - Recommends "slow" as best quality/size ratio
  - Dynamic hint updates when preset changes
  - Helps users make informed encoding time decisions
  - Ranges from ultrafast (~10x faster, ~30% larger) to veryslow (~5x slower, ~15-20% smaller)

- Ã¢Å“â€¦ **Compare Module**
  - Side-by-side video comparison interface
  - Load two videos and compare detailed metadata
  - Displays format, resolution, codecs, bitrates, frame rate, pixel format
  - Shows color space, color range, GOP size, field order
  - Indicates presence of chapters and metadata
  - Accessible via GUI button (pink color) or CLI: `videotools compare <file1> <file2>`
  - Added formatBitrate() helper function for consistent bitrate display

- Ã¢Å“â€¦ **Target File Size Encoding Mode**
  - New "Target Size" bitrate mode in convert module
  - Specify desired output file size (e.g., "25MB", "100MB", "8MB")
  - Automatically calculates required video bitrate based on:
    - Target file size
    - Video duration
    - Audio bitrate
    - Container overhead (3% reserved)
  - Implemented ParseFileSize() to parse size strings (KB, MB, GB)
  - Implemented CalculateBitrateForTargetSize() for bitrate calculation
  - Works in both GUI convert view and job queue execution
  - Minimum bitrate sanity check (100 kbps) to prevent invalid outputs

### Technical Improvements
- Ã¢Å“â€¦ Added compare command to CLI help text
- Ã¢Å“â€¦ Consistent "Target Size" naming throughout UI and code
- Ã¢Å“â€¦ Added compareFile1 and compareFile2 to appState for video comparison
- Ã¢Å“â€¦ Module button grid updated with compare button (pink/magenta color)

## Version 0.1.0-dev12 (2025-12-02)

### Features
- Ã¢Å“â€¦ **Automatic hardware encoder detection and selection**
  - Prioritizes NVIDIA NVENC > Intel QSV > VA-API > OpenH264
  - Falls back to software encoders (libx264/libx265) if no hardware acceleration available
  - Automatically uses best available encoder without user configuration
  - Significant performance improvement on systems with GPU encoding support

- Ã¢Å“â€¦ **iPhone/mobile device compatibility settings**
  - H.264 profile selection (baseline, main, high)
  - H.264 level selection (3.0, 3.1, 4.0, 4.1, 5.0, 5.1)
  - Defaults to main profile, level 4.0 for maximum compatibility
  - Ensures videos play on iPhone 4 and newer devices

- Ã¢Å“â€¦ **Advanced deinterlacing with dual methods**
  - Added bwdif (Bob Weaver) deinterlacing - higher quality than yadif
  - Kept yadif for faster processing when speed is priority
  - Auto-detect interlaced content based on field_order metadata
  - Deinterlace modes: Auto (detect and apply), Force, Off
  - Defaults to bwdif for best quality

- Ã¢Å“â€¦ **Audio normalization for compatibility**
  - Force stereo (2 channels) output
  - Force 48kHz sample rate
  - Ensures consistent playback across all devices
  - Optional toggle for maximum compatibility mode

- Ã¢Å“â€¦ **10-bit encoding for better compression**
  - Changed default pixel format from yuv420p to yuv420p10le
  - Provides 10-20% file size reduction at same visual quality
  - Better handling of color gradients and banding
  - Automatic for all H.264/H.265 conversions

- Ã¢Å“â€¦ **Browser desync fix**
  - Added `-fflags +genpts` to regenerate timestamps
  - Added `-r` flag to enforce constant frame rate (CFR)
  - Fixes "desync after multiple plays" issue in Chromium browsers (Chrome, Edge, Vivaldi)
  - Eliminates gradual audio drift when scrubbing/seeking

- Ã¢Å“â€¦ **Extended resolution support**
  - Added 8K (4320p) resolution option
  - Supports: 720p, 1080p, 1440p, 4K (2160p), 8K (4320p)
  - Prepared for future VR and ultra-high-resolution content

- Ã¢Å“â€¦ **Black bar cropping infrastructure**
  - Added AutoCrop configuration option
  - Cropdetect filter support for future auto-detection
  - Foundation for 15-30% file size reduction in dev13

### Technical Improvements
- Ã¢Å“â€¦ All new settings propagate to both direct convert and queue processing
- Ã¢Å“â€¦ Backward compatible with legacy InverseTelecine setting
- Ã¢Å“â€¦ Comprehensive logging for all encoding decisions
- Ã¢Å“â€¦ Settings persist across video loads

### Bug Fixes
- Ã¢Å“â€¦ Fixed VFR (Variable Frame Rate) handling that caused desync
- Ã¢Å“â€¦ Prevented timestamp drift in long videos
- Ã¢Å“â€¦ Improved browser playback compatibility

## Version 0.1.0-dev11 (2025-11-30)

### Features
- Ã¢Å“â€¦ Added persistent conversion stats bar visible on all screens
  - Real-time progress updates for running jobs
  - Displays pending/completed/failed job counts
  - Clickable to open queue view
  - Shows job title and progress percentage
- Ã¢Å“â€¦ Added multi-video navigation with Prev/Next buttons
  - Load multiple videos for batch queue setup
  - Switch between loaded videos to review settings before queuing
  - Shows "Video X of Y" counter
- Ã¢Å“â€¦ Added installation script with animated loading spinner
  - Braille character animations
  - Shows current task during build and install
  - Interactive path selection (system-wide or user-local)
  - Added error dialogs with "Copy Error" button
  - One-click error message copying for debugging
  - Applied to all major error scenarios
  - Better user experience when reporting issues

### Improvements
- Ã¢Å“â€¦ Align direct convert and queue behavior
  - Show active direct convert inline in queue with live progress
  - Preserve queue scroll position during updates
  - Back button from queue returns to originating module
  - Queue badge includes active direct conversions
  - Allow adding to queue while a convert is running
- Ã¢Å“â€¦ DVD-compliant outputs
  - Enforce MPEG-2 video + AC-3 audio, yuv420p
  - Apply NTSC/PAL targets with correct fps/resolution
  - Disable cover art for DVD targets to avoid mux errors
  - Unified settings for direct and queued jobs
- Ã¢Å“â€¦ Updated queue tile to show active/total jobs instead of completed/total
  - Shows pending + running jobs out of total
  - More intuitive status at a glance
- Ã¢Å“â€¦ Fixed critical deadlock in queue callback system
  - Callbacks now run in goroutines to prevent blocking
  - Prevents app freezing when adding jobs to queue
- Ã¢Å“â€¦ Improved batch file handling with detailed error reporting
  - Shows which specific files failed to analyze
  - Continues processing valid files when some fail
  - Clear summary messages
- Ã¢Å“â€¦ Fixed queue status display
  - Always shows progress percentage (even at 0%)
  - Clearer indication when job is running vs. pending
- Ã¢Å“â€¦ Fixed queue deserialization for formatOption struct
  - Handles JSON map conversion properly
  - Prevents panic when reloading saved queue on startup

### Bug Fixes
- Ã¢Å“â€¦ Fixed crash when dragging multiple files
  - Better error handling in batch processing
  - Graceful degradation for problematic files
- Ã¢Å“â€¦ Fixed deadlock when queue callbacks tried to read stats
- Ã¢Å“â€¦ Fixed formatOption deserialization from saved queue

## Version 0.1.0-dev7 (2025-11-23)

### Features
- Ã¢Å“â€¦ Changed default aspect ratio from 16:9 to Source across all instances
  - Updated initial state default
  - Updated empty fallback default
  - Updated reset button behavior
  - Updated clear video behavior
  - Updated hint label text

### Documentation
- Ã¢Å“â€¦ Created comprehensive MODULES.md with all planned modules
- Ã¢Å“â€¦ Created PERSISTENT_VIDEO_CONTEXT.md design document
- Ã¢Å“â€¦ Created VIDEO_PLAYER.md documenting custom player implementation
- Ã¢Å“â€¦ Reorganized docs into module-specific folders
- Ã¢Å“â€¦ Created detailed Convert module documentation
- Ã¢Å“â€¦ Created detailed Inspect module documentation
- Ã¢Å“â€¦ Created detailed Rip module documentation
- Ã¢Å“â€¦ Created docs/README.md navigation hub
- Ã¢Å“â€¦ Created TODO.md and DONE.md tracking files

## Version 0.1.0-dev6 and Earlier

### Core Application
- Ã¢Å“â€¦ Fyne-based GUI framework
- Ã¢Å“â€¦ Multi-module architecture with tile-based main menu
- Ã¢Å“â€¦ Application icon and branding
- Ã¢Å“â€¦ Debug logging system (VIDEOTOOLS_DEBUG environment variable)
- Ã¢Å“â€¦ Cross-module state management
- Ã¢Å“â€¦ Window initialization and sizing

### Convert Module (Partial Implementation)
- Ã¢Å“â€¦ Basic video conversion functionality
- Ã¢Å“â€¦ Format selection (MP4, MKV, WebM, MOV, AVI)
- Ã¢Å“â€¦ Codec selection (H.264, H.265, VP9)
- Ã¢Å“â€¦ Quality presets (CRF-based encoding)
- Ã¢Å“â€¦ Output aspect ratio selection
  - Source, 16:9, 4:3, 1:1, 9:16, 21:9
- Ã¢Å“â€¦ Aspect ratio handling methods
  - Auto, Letterbox, Pillarbox, Blur Fill
- Ã¢Å“â€¦ Deinterlacing options
  - Inverse telecine with default smoothing
- Ã¢Å“â€¦ Mode toggle (Simple/Advanced)
- Ã¢Å“â€¦ Output filename customization
- Ã¢Å“â€¦ Default output naming ("-convert" suffix)
- Ã¢Å“â€¦ Status indicator during conversion
- Ã¢Å“â€¦ Cancelable conversion process
- Ã¢Å“â€¦ FFmpeg command construction
- Ã¢Å“â€¦ Process management and execution

### Video Loading & Metadata
- Ã¢Å“â€¦ File selection dialog
- Ã¢Å“â€¦ FFprobe integration for metadata parsing
- Ã¢Å“â€¦ Video source structure with comprehensive metadata
  - Path, format, resolution, duration
  - Video/audio codecs
  - Bitrate, framerate, pixel format
  - Field order detection
- Ã¢Å“â€¦ Preview frame generation (24 frames)
- Ã¢Å“â€¦ Temporary directory management for previews

### Media Player
- Ã¢Å“â€¦ Embedded video playback using FFmpeg
- Ã¢Å“â€¦ Audio playback with SDL2
- Ã¢Å“â€¦ Frame-accurate rendering
- Ã¢Å“â€¦ Playback controls (play/pause)
- Ã¢Å“â€¦ Volume control
- Ã¢Å“â€¦ Seek functionality with progress bar
- Ã¢Å“â€¦ Player window sizing based on video aspect ratio
- Ã¢Å“â€¦ Frame pump system for smooth playback
- Ã¢Å“â€¦ Audio/video synchronization
- Ã¢Å“â€¦ Stable seeking and embedded video rendering

### Metadata Display
- Ã¢Å“â€¦ Metadata panel showing key video information
- Ã¢Å“â€¦ Resolution display
- Ã¢Å“â€¦ Duration formatting
- Ã¢Å“â€¦ Codec information
- Ã¢Å“â€¦ Aspect ratio display
- Ã¢Å“â€¦ Field order indication

### Inspect Module (Basic)
- Ã¢Å“â€¦ Video metadata viewing
- Ã¢Å“â€¦ Technical details display
- Ã¢Å“â€¦ Comprehensive information in Convert module metadata panel
- Ã¢Å“â€¦ Cover art preview capability

### UI Components
- Ã¢Å“â€¦ Main menu with 8 module tiles
  - Convert, Merge, Trim, Filters, Upscale, Audio, Thumb, Inspect
- Ã¢Å“â€¦ Module color coding for visual identification
- Ã¢Å“â€¦ Clear video control in metadata panel
- Ã¢Å“â€¦ Reset button for Convert settings
- Ã¢Å“â€¦ Status label for operation feedback
- Ã¢Å“â€¦ Progress indication during operations

### Git & Version Control
- Ã¢Å“â€¦ Git repository initialization
- Ã¢Å“â€¦ .gitignore configuration
- Ã¢Å“â€¦ Version tagging system (v0.1.0-dev1 through dev7)
- Ã¢Å“â€¦ Commit message formatting
- Ã¢Å“â€¦ Binary exclusion from repository
- Ã¢Å“â€¦ Build cache exclusion

### Build System
- Ã¢Å“â€¦ Go modules setup
- Ã¢Å“â€¦ Fyne dependencies integration
- Ã¢Å“â€¦ FFmpeg/FFprobe external tool integration
- Ã¢Å“â€¦ SDL2 integration for audio
- Ã¢Å“â€¦ OpenGL bindings (go-gl) for video rendering
- Ã¢Å“â€¦ Cross-platform file path handling

### Asset Management
- Ã¢Å“â€¦ Application icon (VT_Icon.svg)
- Ã¢Å“â€¦ Icon export to PNG format
- Ã¢Å“â€¦ Icon embedding in application

### Logging & Debugging
- Ã¢Å“â€¦ Category-based logging (SYS, UI, MODULE, etc.)
- Ã¢Å“â€¦ Timestamp formatting
- Ã¢Å“â€¦ Debug output toggle via environment variable
- Ã¢Å“â€¦ Log file output (videotools.log)

### Error Handling
- Ã¢Å“â€¦ FFmpeg execution error capture
- Ã¢Å“â€¦ File selection cancellation handling
- Ã¢Å“â€¦ Video parsing error messages
- Ã¢Å“â€¦ Process cancellation cleanup

### Utility Functions
- Ã¢Å“â€¦ Duration formatting (seconds to HH:MM:SS)
- Ã¢Å“â€¦ Aspect ratio parsing and calculation
- Ã¢Å“â€¦ File path manipulation
- Ã¢Å“â€¦ Temporary directory creation and cleanup

## Technical Achievements

### Architecture
- Ã¢Å“â€¦ Clean separation between UI and business logic
- Ã¢Å“â€¦ Shared state management across modules
- Ã¢Å“â€¦ Modular design allowing easy addition of new modules
- Ã¢Å“â€¦ Event-driven UI updates

### FFmpeg Integration
- Ã¢Å“â€¦ Dynamic FFmpeg command building
- Ã¢Å“â€¦ Filter chain construction for complex operations
- Ã¢Å“â€¦ Stream mapping for video/audio handling
- Ã¢Å“â€¦ Process execution with proper cleanup
- Ã¢Å“â€¦ Progress parsing from FFmpeg output (basic)

### Media Playback
- Ã¢Å“â€¦ Custom media player implementation
- Ã¢Å“â€¦ Frame extraction and display pipeline
- Ã¢Å“â€¦ Audio decoding and playback
- Ã¢Å“â€¦ Synchronization between audio and video
- Ã¢Å“â€¦ Embedded playback within application window
- Ã¢Å“â€¦ Seek functionality with progress bar
- Ã¢Å“â€¦ Player window sizing based on video aspect ratio
- Ã¢Å“â€¦ Frame pump system for smooth playback
- Ã¢Å“â€¦ Audio/video synchronization
- Ã¢Å“â€¦ Checkpoint system for playback position

### UI/UX
- Ã¢Å“â€¦ Responsive layout adapting to content
- Ã¢Å“â€¦ Intuitive module selection
- Ã¢Å“â€¦ Clear visual feedback during operations
- Ã¢Å“â€¦ Logical grouping of related controls
- Ã¢Å“â€¦ Helpful hint labels for user guidance

## Milestones

- **2025-11-23** - v0.1.0-dev7 released with Source aspect ratio default
- **2025-11-22** - Documentation reorganization and expansion
- **2025-11-21** - Last successful binary build (GCC compatibility)
- **Earlier** - v0.1.0-dev1 through dev6 with progressive feature additions
  - dev6: Aspect ratio controls and cancelable converts
  - dev5: Icon and basic UI improvements
  - dev4: Build cache management
  - dev3: Media player checkpoint
  - Earlier: Initial implementation and architecture

## Development Progress

### Lines of Code (Estimated)
- **main.go**: ~2,500 lines (comprehensive Convert module, UI, player)
- **Documentation**: ~1,500 lines across multiple files
- **Total**: ~4,000+ lines

### Modules Status
- **Convert**: 60% complete (core functionality working, advanced features pending)
- **Inspect**: 20% complete (basic metadata display, needs dedicated module)
- **Merge**: 0% (planned)
- **Trim**: 0% (planned)
- **Filters**: 0% (planned)
- **Upscale**: 0% (planned)
- **Audio**: 0% (planned)
- **Thumb**: 0% (planned)
- **Rip**: 0% (planned)

### Documentation Status
- **Module Documentation**: 30% complete
  - Ã¢Å“â€¦ Convert: Complete
  - Ã¢Å“â€¦ Inspect: Complete
  - Ã¢Å“â€¦ Rip: Complete
  - Ã¢ÂÂ³ Others: Pending
- **Design Documents**: 50% complete
  - Ã¢Å“â€¦ Persistent Video Context
  - Ã¢Å“â€¦ Module Overview
  - Ã¢ÂÂ³ Architecture
  - Ã¢ÂÂ³ FFmpeg Integration
- **User Guides**: 0% complete

## Bug Fixes & Improvements

### Recent Fixes
- Ã¢Å“â€¦ Fixed aspect ratio default from 16:9 to Source (dev7)
- Ã¢Å“â€¦ Ranked benchmark results by score and added cancel confirmation
- Ã¢Å“â€¦ Added estimated audio bitrate fallback when metadata is missing
- Ã¢Å“â€¦ Made target file size input unit-selectable with numeric-only entry
- Ã¢Å“â€¦ Prevented snippet runaway bitrates when using Match Source Format
- Ã¢Å“â€¦ History sidebar refreshes when jobs complete (snippet entries now appear)
- Ã¢Å“â€¦ Benchmark errors now show non-blocking notifications instead of OK popups
- Ã¢Å“â€¦ Fixed stats bar updates to run on the UI thread to avoid Fyne warnings
- Ã¢Å“â€¦ Defaulted Target Aspect Ratio back to Source unless user explicitly sets it
- Ã¢Å“â€¦ Synced Target Aspect Ratio between Simple and Advanced menus
- Ã¢Å“â€¦ Hide manual CRF input when Lossless quality is selected
- Ã¢Å“â€¦ Upscale now recomputes target dimensions from the preset to ensure 2X/4X apply
- Ã¢Å“â€¦ Added unit selector for manual video bitrate entry
- Ã¢Å“â€¦ Reset now restores full default convert settings even with no config file
- Ã¢Å“â€¦ Reset now forces resolution and frame rate back to Source
- Ã¢Å“â€¦ Fixed reset handler scope for convert tabs
- Ã¢Å“â€¦ Restored 25%/33%/50%/75% target size reduction presets
- Ã¢Å“â€¦ Default bitrate preset set to 2.5 Mbps and added 2.0 Mbps option
- Ã¢Å“â€¦ Default encoder preset set to slow
- Ã¢Å“â€¦ Bitrate mode now strictly hides unrelated controls (CRF only in CRF mode)
- Ã¢Å“â€¦ Removed CRF visibility toggle from quality updates to prevent CBR/VBR bleed-through
- Ã¢Å“â€¦ Added CRF preset dropdown with Manual option
- Ã¢Å“â€¦ Added 0.5/1.0 Mbps bitrate presets and simplified preset names
- Ã¢Å“â€¦ Default bitrate preset normalized to 2.5 Mbps to avoid "select one"
- Ã¢Å“â€¦ Linked simple and advanced bitrate presets so they stay in sync
- Ã¢Å“â€¦ Hide quality presets when bitrate mode is not CRF
- Ã¢Å“â€¦ Snippet UI now shows Convert Snippet + batch + options with context-sensitive controls
- Ã¢Å“â€¦ Reduced module video pane minimum sizes to allow GNOME window snapping
- Ã¢Å“â€¦ Added cache/temp directory setting with SSD recommendation and override
- Ã¢Å“â€¦ Snippet defaults now use conversion settings (not Match Source)
- Ã¢Å“â€¦ Added frame interpolation presets to Filters and wired filter chain to Upscale
- Ã¢Å“â€¦ Stabilized video seeking and embedded rendering
- Ã¢Å“â€¦ Improved player window positioning
- Ã¢Å“â€¦ Fixed clear video functionality
- Ã¢Å“â€¦ Resolved build caching issues
- Ã¢Å“â€¦ Removed binary from git repository

### Performance Improvements
- Ã¢Å“â€¦ Optimized preview frame generation
- Ã¢Å“â€¦ Efficient FFmpeg process management
- Ã¢Å“â€¦ Proper cleanup of temporary files
- Ã¢Å“â€¦ Responsive UI during long operations

## Acknowledgments

### Technologies Used
- **Fyne** - Cross-platform GUI framework
- **FFmpeg/FFprobe** - Video processing and analysis
- **SDL2** - Audio playback
- **OpenGL (go-gl)** - Video rendering
- **Go** - Primary programming language

### Community Resources
- FFmpeg documentation and community
- Fyne framework documentation
- Go community and standard library

---

*Last Updated: 2025-12-21*

