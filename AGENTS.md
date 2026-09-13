# VideoTools Agent Workflow Rules

These rules apply to **every** agent working in this repo — Claude, opencode, Gemini, and any future addition. They are binding. If a rule here conflicts with an agent's default behavior or harness instructions, **this file wins**. Shipped-work history lives in `DONE.md` and `docs/CHANGELOG.md` — this file records only what an agent needs to act correctly *now*.

## Current Project State

- **Cycle:** `v0.1.1-dev76` — **current**. **dev76 = ISO 9660 resolve fix + menu-export dedup + choose-titles starts empty**: (1) every ISO 9660-only grey-market image failed the *rip* path with "VIDEO_TS not found in ISO 9660 image" right after a successful scan (`Lewd Conduct 1.iso` + `Barely Legal #11.iso`) — the native UDF/ISO 9660 readers extract a target directory's descendants FLAT into the destination root (`tempDir\VIDEO_TS.IFO`, no nested `tempDir\VIDEO_TS`), but `resolveISOWithUDF` demanded the nested folder; `extractedVideoTSPath` now accepts nested or flat (marker `targetDir.IFO` for DVD, `index.bdmv` for BD), used by both the UDF-success and ISO 9660-fallback branches. (2) repeated title rips of one disc no longer write a fresh identical `_Menu_<label>.mkv` per output base — `menuAlreadyExported` skips same-(label,size) menus already in the output dir ("already exported (same menu content)"), keeping animated-menu reauthoring without duplicate sprawl. (3) dev75's choose-titles reshape selected EVERY title on mode transition, so "rip two extras" queued the whole disc ("made the title, and then `_03/_04/_05`… downloading more than we want"); `CanonicalSelection` now returns empty for `""` mode — what is ticked is exactly what rips (Select All still opts into all; `"full"` keeps its single-job path); `"main"`/`"segments"` defaults unchanged. **dev75 released 2026-09-13** (tag `v0.1.1-dev75`, `db5831f8`): **dynamic rip-mode selection + Select/Deselect All deadlock fix**: switching the rip-mode radio now *reshapes* the title selection instead of leaving the previous mode's restrictions behind — "Movie + extras (choose titles)" auto-selects every title, "Main feature only" keeps only the main one (anchored), "Scene segments only" selects just the detected segments; manual per-title toggles inside a mode persist (only a mode *transition* re-shapes) — note dev76 reworks the choose-titles default to start empty. Root-cause crash fix: Select All / Deselect All called `SetChecked` while holding the ContentBrowser mutex, so the checkbox's OnChanged deadlocked back on the same mutex and froze the UI; bulk selections now raise the card's updating guard, `ApplyModeLock` is a purely visual layer (locked/anchored only), and selection is owned by `ReshapeSelection` (full replacement, no per-title OnChanged). `CanonicalSelection`/`CanonicalLock` (new `internal/app/modules/rip/ripmode.go`, unit-tested) centralise mode → selection/lock; fresh discs reset the rip mode to "Main feature only". **dev74 released 2026-09-13** (tag `v0.1.1-dev74`, `b6f90be6`): **scene-segment rip modes + cell-accurate VOB concat fallback**: (1) the rip view detects scene-segmented discs (`DetectSceneSets` bins scan titles into whole-movie copies vs scene segments, tolerant of the `Red Hairy Teens (2008)` duplicate-duration quirks — T01/T02 5926.48/5932.48 s runs, T03–T07 866.04/615.40/1977.60/1331.08/1141.16 s segments) and offers a third radio option "Scene segments only (skip full movie)" once a scene set is detected: it greys/locks the whole-movie titles via the ContentBrowser mode lock, pre-selects all segments (individually toggleable), and clicking a greyed title (or the locked-mode exit) returns to "Movie + extras (choose titles)"; "Main feature only" locks every title but the main one. New i18n keys `RipModeScenesOnly`/`RipReadyScenesOne`/`RipReadyScenesManyFmt` (en/fr/iu/iu_latin). (2) **the VOB-concat fallback is now cell-accurate** — the root-cause fix for still-wrong scene rips: on Windows, ffmpeg's dvdvideo demuxer can't open the source (libdvdnav rejects it), every rip falls back to whole-file VOB concatenation, and a shared-VTS extra title ripped the movie's opening. `ifo.TitleInfo` now exposes per-title PGC cells (`Cells []TitleCell`, VOBID/CellID/FirstSector/LastSector) read from the PGC cell position table + cell playback sector extents; `cellConcatList` slices each VOB to the selected title's cell byte ranges (sector×2048, adjacent cells coalesced per VOB) and feeds the concat path (both primary fallback and dvdvideo-failure retry). Skipped when cells span the whole VOB set (whole-file is then exact), on `HasAngles`, or when a cell VOB can't resolve — each logs a fallback warning. (3) `loadDisc` resets `discTitle`/`outputTouched` so a newly loaded disc never inherits the previous movie's Title in its default output path. **dev73 released 2026-09-13** (tag `v0.1.1-dev73`, `cfa9e879`): **per-title PGC duration scan consistency**: on multi-PGC VTS discs (scene-segmented "extras" titles — e.g. the `Red Hairy Teens (2008)` disc, all 7 titles in VTS_01), the rip scan cached one `ifo.ReadTitleInfo` per VTS and the reader only looked at the first title-domain PGC, so EVERY title reported that PGC's duration — T01–T07 all read "1h 38m" while T03–T07 were 14/10/33/22/19-minute scene segments. New `ifo.ReadTitleInfoForTTN` selects a title's own PGC from the PGCI_SRP TitleNr byte (shared-PGC rule with index fallback for authors that leave it zero); the scan caches per (VTS, TTN) so title cards/snippets/the main-feature pick all show true per-title durations, and the executor resolves the selected title's per-VTS TTN from the VMG TT_SRPT (`resolveVTS_TTN`) so embedded chapters + the IFO-sourced `-t` cap describe the ripped title. Verified on `Red Hairy Teens (2008)`: T01/T02 5926.48/5932.48 s (15 ch), T03–T07 866.04/615.40/1977.60/1331.08/1141.16 s (3 ch each) — matches the cell-bin analysis. Unit tests cover shared-PGC / index-fallback / ttn=0 legacy. **dev72 released 2026-09-12** (tag `v0.1.1-dev72`, `c0e21121`): **ISO 9660 fallback for non-UDF disc images**: some grey-market DVDs ship with a burner-written UDF bridge whose anchor scans as PVD/IUVD/PD/LVD/USD/TD tags but whose LVD fails to parse — the UDF reader reported `LVD not found in VDS` and the disc couldn't be scanned or ripped (reported on `Blowjob Fantasies 3`; verified the image is ISO 9660-only with a junk UDF VDS at sector 48/32). A new native reader `internal/dvd/iso9660` (PVD magic `CD001` at sector 16, spec-correct single-byte `.`/`..` identifiers 0x00/0x01, `;`-version-stripped names, multi-extent continuation folding) mirrors the UDF reader surface (`ReadFileData`/`ExtractDirectory`); scan (`scanISOViaUDF`) and rip (`resolveISOWithUDF`) both try UDF first then fall back to ISO 9660, logging the backend; when BOTH fail the error names both backends so a damaged image is never falsely called corrupt. Verified on the reported ISO: IFO reads byte-exact vs the ISO 9660 listing (VIDEO_TS.IFO 12288 B, VTS_01_0.IFO 94208 B) and the full scan path returns both titles. Unit tests + `VT_REAL_ISO`-gated integration tests. **dev71 released 2026-09-12** (tag `v0.1.1-dev71`, `983f668d`): **VOB-concat stale-PTS cap hardening + Title-driven output names** — (1) the dev69 cap never engaged on a second grey-market disc (`11 (2000)`, same VOB layout as the dev69 title) — the stale PTS offset bakes into the *entire* second VOB, so it probes to +93822 s and the bundled static ffprobe errors on it, hitting the old "could not probe all VOB durations" branch → rip still 26h (and seeking into that inflated region crashes MPV — the file's timestamps are structurally broken, not just mis-labelled). The `-t` cap source is now hierarchical: per-VOB media-duration sum when sane (sum < 3× the IFO PGC play time — dev69 behaviour preserved), else the IFO PGC `titleInfo.Duration` (immune to stale file PTS offsets), so a safe cap ALWAYS engages; verified on `11 (2000)` (`-t 1884` → 1822.816 s, the real ~30m23s; concat+genpts view is clean with continuous packets — no seek breakage). (2) **the rip Title field drives the output filename by default** — a set Title names the file (`test` → `test.mkv` in DVD_Rips) and the full-disc dir (`FullDiscOutputTitlePath`); blank Title / source-format changes fall back to the source name; hand-editing the output path stops auto-naming (`outputTouched`). **dev70 released 2026-09-12** (tag `v0.1.1-dev70`, `83155474`): **rip UI polish** — mode radio reworded ("Movie + extras (choose titles)" / "Main feature only", stacked vertically, still defaults to the latter); title-card info line on one localized "N chapters · N audio · N subs" line (no word-wrap split, no duplicated T##/duration; new singular/plural `RipTitleAudioOne/Many` + `RipTitleSubsOne/Many` keys across en/fr/iu/iu_latin, `RipTitleCardFmt` now just "%d chapters"); subtitle-language picker gains Select All / Deselect All buttons (existing `RipSelectAll`/`RipDeselectAll` keys reused, per-language `OnChanged` suppressed/restored so the selection never duplicates, persisted). **dev69 released 2026-09-12** (tag `v0.1.1-dev69`, `e4e94407`): VOB-concat stale-PTS duration cap — the executor probes per-VOB durations (`probeDuration`) and caps the output with `-t ceil(Σ + 60 s)` via new `RipArgs.MaxDuration`; verified end-to-end on the Bose Mosen `06 (1998)` disc (uncapped 95443.815 s → capped 1822.784 s). **dev68 released 2026-09-12** (tag `v0.1.1-dev68`, `bf1e6522`): VOB-concat subtitle-map clamp for liar-IFO discs — `probeSubtitleCount` clamps `SubtitleLangs`/`SubtitleSel` to the physical streams that exist so the per-stream `-map 0:s:<idx>` flags never reference missing streams. Earlier history in DONE.md.
- **Public/stable baseline:** `v0.1.1`.
- **Planning sources:** `TODO.md` (scope), `docs/roadmap.html` (canonical tracker), `DONE.md` + `docs/CHANGELOG.md` (shipped history).
- **Issue tracker:** https://github.com/LeakTechnologies/VideoTools/issues
- **Player debug log:** `docs/PLAYER_DEBUG.md` — update before closing any player-related issue.

## Current Priorities (unshipped only)

| Priority | Item | Notes |
|---|---|---|
| 1 | libVLC Player Backend (Phase 1) | `docs/VLC_PLAYER.md` — PlaybackEngine interface + VLCBackend CGo wrapper; FFmpeg engine stays as long-term plan |
| 2 | Tester verification of dev76 + dev75 + dev74 + dev73 + dev72 + dev71 + dev70 + dev69 + dev68 + dev67 + dev66 + dev65 + dev64 releases | **dev76**: (1) load `Lewd Conduct 1.iso` / `Barely Legal #11.iso` — the rip completes with no "VIDEO_TS not found in ISO 9660 image" (the resolve path accepts the flat extraction layout now; expect "Extracted VIDEO_TS via ISO 9660 fallback" on rip); (2) rip several titles of one disc with menus on — menus export once per output dir, later titles log "already exported (same menu content)" and skip; (3) switch the radio to "Movie + extras (choose titles)" — the list starts EMPTY (nothing pre-ticked, "ready to rip" shows no-selection) and only what you tick rips ("main" and "segments" defaults unchanged). **dev75**: switching the rip-mode radio to "Movie + extras (choose titles)" now starts EMPTY per dev76 (the old dev75 auto-select-ALL was the over-production report); switching to "Main feature only" shows only the main title selected+anchored and everything else greyed; "Scene segments only" (on a scene-segmented disc) greys the whole-movie titles and pre-selects the segments; Select All / Deselect All work without freezing even right after a mode switch; toggling individual titles inside a mode stays manual until the next mode change; a freshly loaded disc always starts in "Main feature only". **dev74**: scan the `Red Hairy Teens (2008)` disc — the rip-mode radio gains "Scene segments only (skip full movie)"; selecting it greys the two whole-movie titles and pre-selects the five segments (readiness reads "Ready to rip 5 scene segments"); clicking a greyed title returns to "Movie + extras (choose titles)". Rip one scene segment on a Windows build (dvdvideo fallback): the output must be the ACTUAL segment (~14 min / 3 chapters), not the movie's opening — the log shows "using cell-accurate input list (N cell ranges)". Also confirm loading a disc after ripping another disc with a set Title produces a fresh source-derived filename, and "Main feature only" greys everything but the main title. **dev73**: scan the `Red Hairy Teens (2008)` disc — the title list shows true per-title durations/chapters (T03–T07 ≈ 14/10/33/22/19 min, 3 chapters each; T01/T02 ≈ 1h 38m, 15 chapters) instead of all-1h-38m; rip one segment title and confirm the output's embedded chapters + duration match the segment, not the movie. **dev72**: load the `Blowjob Fantasies 3` ISO — scan returns both titles (no `LVD not found in VDS`), the full rip completes via the ISO 9660 fallback, and a genuinely damaged ISO (neither UDF nor ISO 9660) reports an error naming both backends without claiming corruption. **dev71**: (1) re-run the `11 (2000)` Extra Title rip — the log shows the IFO-sourced "VOB concat: capping output at N s (IFO PGC duration ... + 60 s margin)" line and the output is the real ~30m23s (1822.8 s); no player/Explorer shows 26hrs AND skipping around the playback timeline does not crash MPV (the prior file was structurally broken, not just mis-labelled); (2) set a Title before ripping → the output file (and the full-disc output dir) is named from the Title (e.g. `test` → `test.mkv`), a blank Title keeps the source-folder name, and hand-editing the output path keeps the custom name. **dev70**: radio reads "Movie + extras (choose titles)" / "Main feature only", stacks vertically, defaults to "Main feature only"; title-card info line is one clear "N chapters · N audio · N subs" (no word-wrapped "1 audio", no duplicated T##/duration); Select All / Deselect All buttons appear for multi-language subtitle lists and persist to config. **dev69**: re-run the `06 (1998)` Extra Title 04 rip (Achtzehneinhalb/Bose Mosen disc) — log shows "VOB concat: capping output at N s" and the output duration is the real ~30m23s (no player/Explorer shows 26hrs), chapters end at the title duration. **dev68**: liar-IFO disc (more IFO subtitle languages than physical streams) rips cleanly via VOB-concat fallback — the log shows the clamped "dropping N trailing subtitle mapping(s)" line and the job completes; re-verify `Young Harlots - The Academy (2006) DVD5` Main-Feature rip end-to-end. **dev67**: rip mode radio defaults to and lists first "Full movie (main feature)"; Settings PageUp/PageDown/Home/End actually page the active tab (focused key-catcher widget; test on every tab, plus keys stop after leaving Settings); the Disc Menu panel and in-app Rip Log are gone, Content Browser owns the full left-column height at 1600×900 (4+ titles without scrolling). **dev66**: the update-check failure dialog explains the cause (rate-limit / not-yet-published / no network). **dev65**: LOAD DISC loads a physical DVD (drive picker when several drives; data disc/empty/no drive → clean visible error). **dev64**: rip mode radio, per-language subtitle checkboxes (deselect a language → streams stripped; legacy configs default to all), scan-as-you-go DiscSummary snippets, Convert SMPTE idle restored on menu-driven re-entry + the dev63/dev62 patches (metadata fold keeps the tappable header visible; ISO load shows "Reading disc information"; dev62 layout corrections) + the dev61→dev62 in-app update from a dev61 binary; move roadmap cards `rip-overhaul`/`rip-refinement`/`rip-refine-followup` `done` → `shipped` on sign-off |
| 3 | Tester verification of dvdvideo rip + concat fallback (dev55–dev57) | CI now builds libdvdread/libdvdnav + ships the dvdvideo demuxer, runtime probe no longer forces VOB concat, and dvdvideo now falls back to concat when libdvdnav rejects a source. Re-run a no-scan multi-VOB rip and confirm no crash at the VOB boundary |
| 4 | Updater hardening | Bake `buildCommit` via ldflags in CI builds (same-tag patch detection dead) — ask before touching workflows; Linux tar.gz extraction path; stale nightly-PATCH comment in `fetchUpdateInfo` |
| 5 | Dead-code retirement (post static-sidecar decision) | `scripts/windows/build-ffmpeg-shared.ps1`, DLL-folder branches in `ffmpeg_bootstrap.go`, `updateSidecars` DLL extraction — legacy-harmless, remove deliberately |
| — | Burn multi-drive batch / IMAPI2 COM | `docs/BURN_MODULE_DESIGN.md` §2–3 |
| — | Main Menu refactor to `internal/app/modules/mainmenu/` | LOW — deferred until engine stable |
| — | UDF 2.50/2.60 + BDMV; sparse/large-file UDF writer | Future |

## Settled Decisions

Reached after failed attempts or Human Director ruling. **Do not change or re-litigate without explicit Human Director approval.**

### libVLC Player Backend (2026-07-21, HD approved)

Replace the custom FFmpeg demux/decode/sync engine with libVLC for user-facing playback. The FFmpeg engine stays as a long-term plan when it matures to a usable state. The custom engine has had 10+ crash-fix cycles across dev43–dev55; its architecture (6 goroutines, 4 mutexes, 3 packet queues) is too complex to stabilise. libVLC has solved these problems for 20+ years. `adrg/libvlc-go/v3` does NOT expose `libvlc_video_set_callbacks` — a custom CGo wrapper is required (~300 lines vs FFmpeg's ~1500). Design doc: `docs/VLC_PLAYER.md`.

### Windows Product: Three Fully Static Binaries (2026-07-04, HD approved)

`VideoTools.exe`, `ffmpeg.exe`, `ffprobe.exe` are each fully self-contained. **No shared FFmpeg build. No DLL/ folder.** Enforced by objdump gates in CI that fail the job on any MinGW runtime DLL reference (`libbz2|liblzma|libiconv|libstdc++|libwinpthread|libgcc|zlib1`). App treats static sidecars as primary (`appcfg.StaticSidecarsWork()`); DLL-folder paths remain only as legacy-bundle support.

### CI Toolchain — FFmpeg, x264, x265, libdvdread/libdvdnav

- **FFmpeg is built from source** — one static build per platform serves both the CGo link and the sidecar programs (`--extra-ldflags="-static"` on Windows; never `--disable-programs`).
- **BtbN FFmpeg-Builds must NOT be used** — no static `.a` libs, moving-tag ABI drift.
- **x264/x265 built from source, static-only** — MSYS2 prebuilt packages have `__declspec(dllimport)` headers that poison the static link.
- **x265.pc must be overwritten after cmake install** (LF, POSIX paths). C++ deps (`-lstdc++ -lsupc++ -lm` Windows / `-lstdc++ -lm` Linux) go in **`Libs`**, not `Libs.private` — FFmpeg configure calls pkg-config without `--static`.
- **dvdvideo demuxer: libdvdread 6.1.3 + libdvdnav 6.1.1 built from source, static-only**, in all Windows workflows (dev/release/msix), installed into the ffmpeg prefix, then `dvdnav.pc` overwritten so `-ldvdread` sits in **`Libs`** (same reasoning as x265.pc — FFmpeg's Windows configure calls pkg-config without `--static`, so the stock `Requires.private: dvdread` is never expanded and the `dvdnav_open2` link test fails). Linux needs no overwrite (`--pkg-config-flags="--static"` expands `Requires.private`). Each pipeline gates with `ffmpeg -h demuxer=dvdvideo`. Changing FFmpeg/dvd-lib setup invalidates the ffmpeg cache — bump the Windows `v6`/msix `v3` keys.
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
- The retired `docs.leaktechnologies.dev` site must not be referenced.
- New features get `docs/FEATURE_NAME.md` (overview, implementation, files, testing checklist) **before** implementation, linked from TODO.md and this file.

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
