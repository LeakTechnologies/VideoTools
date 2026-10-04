# VideoTools Agent Workflow Rules

These rules apply to **every** agent working in this repo — Claude, opencode, Gemini, and any future addition. They are binding. If a rule here conflicts with an agent's default behavior or harness instructions, **this file wins**. Shipped-work history lives in `DONE.md` and `docs/CHANGELOG.md` — this file records only what an agent needs to act correctly *now*.

## Current Project State

- **Cycle:** `v0.1.1-dev84` — in review, not yet released. **Player — the dormant libVLC backend and its inert Settings toggle removed.** The tracker had carried "libVLC Player Backend (Phase 1)" as the next slice after dev83, implying work in progress; a partial implementation had landed 2026-07-24 behind a `vlc` build tag and could never have been built (five unresolved libVLC C symbols, and no workflow has ever passed that tag). The user-visible half was worse: Settings shipped a "Use libVLC backend" checkbox that persisted `UsePlayerVLC` and called `media.SetUseVLC`, but the only reader lived in `engine_factory_vlc.go` behind the tag no CI build compiles — so in every shipped binary the toggle did nothing, and its hint claimed libVLC "is more stable for seek/resume", never true for anyone (same defect class as the Convert two-pass toggle, one layer down). Removed `vlc_engine.go`, `vlc_video.go`, `vlc_glue.h`, `engine_factory_vlc.go`, `vlc_events.go` (it `#include`d the deleted header, so it could not have compiled either), `internal/player/vlc_controller.go` (502 lines of CLI-based VLC plumbing nothing referenced, and the factory already refused to construct it), the toggle + two i18n keys + `PrefsConfig` field + settings callbacks, `media.SetUseVLC`/`UseVLC`, and the `main` load-time call. Kept `media.PlaybackEngine` (the seam `InlineVideoPlayer` actually uses), `docs/VLC_PLAYER.md`, and `BackendVLC` (deprecated enum the factory already errored on). Stale `UsePlayerVLC` JSON is ignored, not rejected (`LoadModuleJSON` decodes leniently). `engine_factory_default.go` drops its now-meaningless `&& !vlc`, so `-tags "native_media vlc"` compiles to the same binary as `-tags native_media`. Guards `TestNoDeadVLCBackendControl` + `TestNoVLCBackendCode`, both mutation-verified. Commit `f7ce4d3d`. **This removes a lie, it does not add a feature.** **dev83 released 2026-10-04** (tag `v0.1.1-dev83`, release commit `c9e31d92` + post-release doc correction `30036eb0`): **Rip — LPCM (`pcm_dvd`) titles now rip losslessly + Convert two-pass honesty.**
- **Public/stable baseline:** `v0.1.1`.
- **Planning sources:** `TODO.md` (scope), `docs/roadmap.html` (canonical tracker), `DONE.md` + `docs/CHANGELOG.md` (shipped history).
- **Issue tracker:** https://github.com/LeakTechnologies/VideoTools/issues
- **Player debug log:** `docs/PLAYER_DEBUG.md` — update before closing any player-related issue.

## Current Priorities (unshipped only)

| Priority | Item | Notes |
|---|---|---|
| 1 | Convert audit #17 + #22 (player state + lifecycle) | #17: one authoritative playback-state boundary for play/pause, speed, and track controls — no stale UI (`U-5/U-6/U-7/U-8`, `F-11`). #22: source never loaded on switch, rebuild fakes a pause, collapsed player still fed frames (`F-3`, `F-4`, `L-4`). Player changes need a real playback session + `docs/PLAYER_DEBUG.md` before landing |
| 2 | Tester verification of dev82 (published 2026-10-03, tag `v0.1.1-dev82`) + dev81 (published 2026-09-22, tag `v0.1.1-dev81`) + dev80 (published 2026-09-18, tag `v0.1.1-dev80`) + dev79 (published 2026-09-17, tag `v0.1.1-dev79`) + dev78 (published 2026-09-15, tag `v0.1.1-dev78`; supersedes the abandoned dev77 tag) + dev76 + dev75 + dev74 + dev73 + dev72 + dev71 + dev70 + dev69 + dev68 + dev67 + dev66 + dev65 + dev64 releases | **dev82** (per the roadmap checklist): (1) queue a conversion whose output name already exists on disk — the job takes the deterministic `-2` suffix, not a silent overwrite; queue two same-named conversions in one batch — the second takes the next suffix even though neither output exists on disk yet; (2) with a module OutputDir (or the app default output dir) configured, batch-added drops write into the configured directory, not next to each source; (3) select the MOV (ProRes) format — the codec select follows to ProRes and the output encodes `prores_ks` (ffprobe: prores), not silently H.264; (4) select the OGG (Theora) format with default AAC audio — the job completes (audio substitutes to Vorbis-family; previously the muxer rejected the invocation outright) and the output plays; (5) select source A in Convert, start the interlace analysis, switch to B while it runs — no A result ever renders in B's context; Inspect keeps its own independent result across the same sequence; Clear File clears the interlace state; (6) with a pre-existing config lacking the newer fields, the loudness sliders read the defaults (−16 LUFS / −1.5 dBTP), not 0, and a config saved with all three layout panels deliberately collapsed STAYS collapsed across restarts; (7) queue a conversion with audio normalization on — the job log shows the loudnorm filter with the configured targets and the command preview shows the same filter; (8) collapse the player and/or metadata panels, click Reset — the panels return visible with matching header arrows and the split follows the shared policy. **dev81** (per `docs/RIP_CHAPTER_RANGE.md`): (1) "Rip chapters only" From/To on a scanned title rips just those chapters — dvdvideo path fades in at chapter N with chapters N..M embedded, timeline starts at 0, no chapter 1 content; (2) on a libdvdnav-rejected source the cell-accurate concat fallback slices the range's cells (log "using cell-accurate input list (N cell ranges)") and the output spans only the range; (3) readability/readiness + progress reflect the range span, not the whole title; (4) range controls disabled for full-disc rip / region conversion / archivist, and an out-of-range/From>To range is rejected cleanly; (5) toggle off → identical whole-title behaviour. **dev80**: (1) set the thumbnail output mode to "both": the live preview + output dir contain the contact sheet AND the same number of individual screenshots as the grid (a 4×8 grid ⇒ 32 screenshots), and the "both" settings show BOTH boxes with the individual count reading "matches contact sheet" and tracking the grid sliders; individual mode still generates just the separate thumbnails and contact-sheet mode just the sheet; (2) queue: a completed job's history row shows its real elapsed time and Progress (not 0/StartedAt == CompletedAt); (3) a "both" thumbnail job's log file shows the sheet AND every individual thumbnail under its own header. **dev79**: (1) rip a seamless-branching / multi-VOB title on a Windows build — the log shows the dvdvideo demuxer path engaging (no `-title` binding error, exit 0) and the output plays cleanly with NO 26h-freeze / post-cutoff slideshow corruption (the whole-file VOB-concat fallback was silently masking every dvdvideo rip); a disc where libdvdnav still rejects the source falls back to the cell-accurate concat list as before; (2) bulk Select All inside a locked rip mode actually selects the previously greyed/locked titles — the readiness summary and the queued rip match the visible checkboxes, and Deselect All leaves everything unticked; (3) all dev78 items below still pass. **dev78**: (1) thumbnails — individual + contact-sheet size selects offer "Native (WxH)" derived from the source video's real width; the contact-sheet "Total thumbnails" count updates live as the columns/rows sliders move; a CI-build contact sheet shows the timestamp/metadata header (drawtext restored via libfreetype + libharfbuzz), and a build without drawtext completes with plain output + a debug warning instead of "No such filter"; (2) on a scene-set disc (a 7-title scene-segmented disc), switch the radio to "Movie + extras (choose titles)" — the list pre-selects ONLY the main feature + genuine extras, scene segments AND duplicate whole-movie copies stay unticked, readiness counts just those; a non-scene-set disc still starts empty; (3) queue — a completed job reads "Status: Completed" with no elapsed suffix; (4) the dev76 ISO rip reproductions still pass — `ISO9660-sample-A.iso` / `ISO9660-sample-B.iso` complete with "Extracted VIDEO_TS via ISO 9660 fallback" on rip (listed under **dev76** too). **dev76**: (1) load `ISO9660-sample-A.iso` / `ISO9660-sample-B.iso` — the rip completes with no "VIDEO_TS not found in ISO 9660 image" (the resolve path accepts the flat extraction layout now; expect "Extracted VIDEO_TS via ISO 9660 fallback" on rip); (2) rip several titles of one disc with menus on — menus export once per output dir, later titles log "already exported (same menu content)" and skip; (3) switch the radio to "Movie + extras (choose titles)" — the list starts EMPTY (nothing pre-ticked, "ready to rip" shows no-selection) and only what you tick rips ("main" and "segments" defaults unchanged). **dev75**: switching the rip-mode radio to "Movie + extras (choose titles)" now starts EMPTY per dev76 (the old dev75 auto-select-ALL was the over-production report); switching to "Main feature only" shows only the main title selected+anchored and everything else greyed; "Scene segments only" (on a scene-segmented disc) greys the whole-movie titles and pre-selects the segments; Select All / Deselect All work without freezing even right after a mode switch; toggling individual titles inside a mode stays manual until the next mode change; a freshly loaded disc always starts in "Main feature only". **dev74**: scan the 7-title scene-segmented disc — the rip-mode radio gains "Scene segments only (skip full movie)"; selecting it greys the two whole-movie titles and pre-selects the five segments (readiness reads "Ready to rip 5 scene segments"); clicking a greyed title returns to "Movie + extras (choose titles)". Rip one scene segment on a Windows build (dvdvideo fallback): the output must be the ACTUAL segment (~14 min / 3 chapters), not the movie's opening — the log shows "using cell-accurate input list (N cell ranges)". Also confirm loading a disc after ripping another disc with a set Title produces a fresh source-derived filename, and "Main feature only" greys everything but the main title. **dev73**: scan the 7-title scene-segmented disc — the title list shows true per-title durations/chapters (T03–T07 ≈ 14/10/33/22/19 min, 3 chapters each; T01/T02 ≈ 1h 38m, 15 chapters) instead of all-1h-38m; rip one segment title and confirm the output's embedded chapters + duration match the segment, not the movie. **dev72**: load the junk-UDF ISO — scan returns both titles (no `LVD not found in VDS`), the full rip completes via the ISO 9660 fallback, and a genuinely damaged ISO (neither UDF nor ISO 9660) reports an error naming both backends without claiming corruption. **dev71**: (1) re-run the Extra Title rip on a second grey-market disc — the log shows the IFO-sourced "VOB concat: capping output at N s (IFO PGC duration ... + 60 s margin)" line and the output is the real ~30m23s (1822.8 s); no player/Explorer shows 26hrs AND skipping around the playback timeline does not crash MPV (the prior file was structurally broken, not just mis-labelled); (2) set a Title before ripping → the output file (and the full-disc output dir) is named from the Title (e.g. `test` → `test.mkv`), a blank Title keeps the source-folder name, and hand-editing the output path keeps the custom name. **dev70**: radio reads "Movie + extras (choose titles)" / "Main feature only", stacks vertically, defaults to "Main feature only"; title-card info line is one clear "N chapters · N audio · N subs" (no word-wrapped "1 audio", no duplicated T##/duration); Select All / Deselect All buttons appear for multi-language subtitle lists and persist to config. **dev69**: re-run the Extra Title 04 rip (a grey-market disc) — log shows "VOB concat: capping output at N s" and the output duration is the real ~30m23s (no player/Explorer shows 26hrs), chapters end at the title duration. **dev68**: liar-IFO disc (more IFO subtitle languages than physical streams) rips cleanly via VOB-concat fallback — the log shows the clamped "dropping N trailing subtitle mapping(s)" line and the job completes; re-verify a liar-IFO DVD5 disc Main-Feature rip end-to-end. **dev67**: rip mode radio defaults to and lists first "Full movie (main feature)"; Settings PageUp/PageDown/Home/End actually page the active tab (focused key-catcher widget; test on every tab, plus keys stop after leaving Settings); the Disc Menu panel and in-app Rip Log are gone, Content Browser owns the full left-column height at 1600×900 (4+ titles without scrolling). **dev66**: the update-check failure dialog explains the cause (rate-limit / not-yet-published / no network). **dev65**: LOAD DISC loads a physical DVD (drive picker when several drives; data disc/empty/no drive → clean visible error). **dev64**: rip mode radio, per-language subtitle checkboxes (deselect a language → streams stripped; legacy configs default to all), scan-as-you-go DiscSummary snippets, Convert SMPTE idle restored on menu-driven re-entry + the dev63/dev62 patches (metadata fold keeps the tappable header visible; ISO load shows "Reading disc information"; dev62 layout corrections) + the dev61→dev62 in-app update from a dev61 binary; move roadmap cards `rip-overhaul`/`rip-refinement`/`rip-refine-followup` `done` → `shipped` on sign-off |
| 3 | Tester verification of dvdvideo rip + concat fallback | CI ships libdvdread/libdvdnav + the dvdvideo demuxer, the runtime probe no longer forces VOB concat, and dev79 fixed the `-title` arg-ordering root cause that made every dvdvideo rip silently fall back to whole-file VOB concat. Re-run a multi-VOB rip and confirm the dvdvideo path engages (exit 0, clean output, no 26h-freeze / slideshow corruption) and that libdvdnav-rejected sources still fall back to the cell-accurate concat list |
| 4 | Updater hardening | Bake `buildCommit` via ldflags in CI builds (same-tag patch detection dead) — ask before touching workflows; Linux tar.gz extraction path; stale nightly-PATCH comment in `fetchUpdateInfo` |
| 5 | Dead-code retirement (post static-sidecar decision) | `scripts/windows/build-ffmpeg-shared.ps1`, DLL-folder branches in `ffmpeg_bootstrap.go`, `updateSidecars` DLL extraction — legacy-harmless, remove deliberately |
| — | Burn multi-drive batch / IMAPI2 COM | `docs/BURN_MODULE_DESIGN.md` §2–3 |
| — | Main Menu refactor to `internal/app/modules/mainmenu/` | LOW — deferred until engine stable |
| — | UDF 2.50/2.60 + BDMV; sparse/large-file UDF writer | Future |

## Settled Decisions

Reached after failed attempts or Human Director ruling. **Do not change or re-litigate without explicit Human Director approval.**

### libVLC Player Backend — implementation CUT (2026-10-04, dev84, HD approved)

The 2026-07-21 ruling below stands as the *design* direction, but the implementation was removed in dev84 (`f7ce4d3d`). What shipped was never a backend: a `vlc`-tagged partial port that failed to compile (five unresolved libVLC C symbols) behind a tag **no CI workflow ever built**, surfaced to users as a Settings checkbox that did nothing and a hint making a stability claim that was never true for anyone. Do not reintroduce a second backend without all three of: (1) the libVLC SDK provisioned in CI, (2) a CI job that actually builds the backend's build tag, (3) a guarded user-facing control wired to a real reader. The `PlaybackEngine` interface is retained deliberately — it is the seam a real backend plugs into. `docs/VLC_PLAYER.md` is retained as the design record. `TestNoDeadVLCBackendControl` + `TestNoVLCBackendCode` enforce this; re-adding an advert-for-a-missing-backend string, or reintroducing the plumbing, turns them red.

### libVLC Player Backend (2026-07-21, HD approved)

Replace the custom FFmpeg demux/decode/sync engine with libVLC for user-facing playback. The FFmpeg engine stays as a long-term plan when it matures to a usable state. The custom engine has had 10+ crash-fix cycles across dev43–dev55; its architecture (6 goroutines, 4 mutexes, 3 packet queues) is too complex to stabilise. libVLC has solved these problems for 20+ years. `adrg/libvlc-go/v3` does NOT expose `libvlc_video_set_callbacks` — a custom CGo wrapper is required (~300 lines vs FFmpeg's ~1500). Design doc: `docs/VLC_PLAYER.md`.

### Windows Product: Three Fully Static Binaries (2026-07-04, HD approved)

`VideoTools.exe`, `ffmpeg.exe`, `ffprobe.exe` are each fully self-contained. **No shared FFmpeg build. No DLL/ folder.** Enforced by objdump gates in CI that fail the job on any MinGW runtime DLL reference (`libbz2|liblzma|libiconv|libstdc++|libwinpthread|libgcc|zlib1`). App treats static sidecars as primary (`appcfg.StaticSidecarsWork()`); DLL-folder paths remain only as legacy-bundle support.

### CI Toolchain — FFmpeg, x264, x265, libdvdread/libdvdnav

- **FFmpeg is built from source** — one static build per platform serves both the CGo link and the sidecar programs (`--extra-ldflags="-static"` on Windows; never `--disable-programs`).
- **BtbN FFmpeg-Builds must NOT be used** — no static `.a` libs, moving-tag ABI drift.
- **x264/x265 built from source, static-only** — MSYS2 prebuilt packages have `__declspec(dllimport)` headers that poison the static link.
- **x265.pc must be overwritten after cmake install** (LF, POSIX paths). C++ deps (`-lstdc++ -lsupc++ -lm` Windows / `-lstdc++ -lm` Linux) go in **`Libs`**, not `Libs.private` — FFmpeg configure calls pkg-config without `--static`.
- **dvdvideo demuxer: libdvdread 6.1.3 + libdvdnav 6.1.1 built from source, static-only**, in all Windows workflows (dev/release/msix), installed into the ffmpeg prefix, then `dvdnav.pc` overwritten so `-ldvdread` sits in **`Libs`** (same reasoning as x265.pc — FFmpeg's Windows configure calls pkg-config without `--static`, so the stock `Requires.private: dvdread` is never expanded and the `dvdnav_open2` link test fails). Linux needs no overwrite (`--pkg-config-flags="--static"` expands `Requires.private`). Each pipeline gates with `ffmpeg -h demuxer=dvdvideo`. Changing FFmpeg/dvd-lib setup invalidates the ffmpeg cache — bump the Windows dev/release `v12` / msix `v6` / forgejo `v9` + `.built-v10` marker keys.
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
- **Author identity = repo owner.** At session start, before the first commit: `git config user.name "Stu Leak" && git config user.email "leaktechnologies@proton.me"`. History was rewritten 2026-07-05 to purge AI authorship — do not reintroduce it.

## Verification Discipline

- **Player changes require a log review** from a real playback session before landing. The dev53 crash had zero ERROR/WARN lines — the process was killed by I/O pressure from a log-spam bug that no test catches. "No errors logged" is not evidence of correctness; the absence of a crash report is the failure.
- **State-tracking variables must be updated at the point of comparison**, not just read. The seekGen bug: `gen != lastSeekGen` was checked but `lastSeekGen` was never assigned, so the "first frame after seek" log fired 60×/sec forever. Any `if x != y` guard that mutates neither `x` nor `y` is a bug.
- **`gofmt -e` is a syntax check, not a build.** It catches parse errors, not logic errors. `go build -tags=native_media ./...` is the real gate; when CGo LDFLAGS block local builds, say so explicitly rather than implying syntax-OK means build-OK.
- **Local Windows builds: use `scripts/windows/dev-verify.ps1`** (build `-tags=native_media ./...` + `go vet ./...`). Bare `go build` fails at the CGo gate on the `-Wl,--stack,4194304` LDFLAG because `CGO_LDFLAGS_ALLOW` is only set in CI/bash. The script sets `CGO_ENABLED=1`, `CGO_LDFLAGS_ALLOW=-Wl,.*`, and quotes a discovered gcc/g++ — the same env CI uses. Expect a long cold build (~9 min), then incremental runs are fast.

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
- **Token:** use `$VT_RELEASE_TOKEN` (fine-grained PAT: Contents RW + Actions RW on this repo, set as a Claude Code environment variable) for tag pushes (`git push https://x-access-token:$VT_RELEASE_TOKEN@github.com/LeakTechnologies/VideoTools.git vX.Y.Z-devN`) and workflow dispatches (`POST /repos/LeakTechnologies/VideoTools/actions/workflows/<wf>.yml/dispatches`). If the token is absent, hand the Human Director the exact commands instead — never skip the tagging step silently.
- Tag-triggered runs execute the workflow file **as of the tagged commit** — always tag a commit that already contains the current workflow fixes.

## Internationalization (i18n)

All user-facing strings use `i18n.T().KeyName` — never hardcoded literals.

1. Add key to `internal/i18n/strings.go`, English to `en_ca.go` (source of truth), French to `fr_ca.go`, Inuktitut to `iu.go` + `iu_latin.go` (machine-generated OK with `// machine-generated, needs human review`; see `docs/localization-policy.md`).
2. Key naming: `ModuleXxx`, `ActionXxx`, `LabelXxx`, `StatusXxx`, `DialogXxx`.
3. Before landing, grep for stragglers: `widget.NewLabel("`, `widget.NewButton("`, `dialog.Show...("`.

## Repository Hygiene

- Root stays minimal: core manifests, primary app entry source, `README.md`/`TODO.md`/`DONE.md`. Demos/tools under `cmd/` or `scripts/`; packaging under `packaging/<platform>/`. No ad-hoc logs/scratch/backup files in root.
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

**Daily rhythm:**
1. Run `/triage` → reads CI, issues, TODOs, writes TRIAGE.md
2. Read TRIAGE.md → decide what to work on
3. Implement → commit → push
4. Repeat

**What's cheap (do these):**
- `/triage` — read-only discovery, ~50 lines output
- Skills — written once, loaded only when triggered
- State files — TRIAGE.md, TODO.md, DONE.md (agent reads, doesn't generate)

**What's expensive (avoid unless necessary):**
- Automated scheduled loops — burn tokens while you're away
- Multi-agent verification chains — each agent is a full context load
- "Fix it and keep iterating until tests pass" — unpredictable token cost

**Token rules:**
- Triage under 50 lines. Implementation under 1 commit per task.
- Never run two agents on the same file simultaneously.
- If a loop would take >3 turns, break it into smaller human-triggered steps.
- Verifier sub-agent only for high-risk changes (engine, CI, release).

## Coordination

- Ask before changing workflow entrypoints or automation behavior.
- Install/build changes get a docs note.
- Release publishing stays aligned to `VERSION`; never retarget or delete existing dev tags; old workflow runs are not evidence of current release state.
