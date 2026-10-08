# VideoTools Agent Workflow Rules

These rules apply to **every** agent working in this repo — Claude, opencode, Gemini, and any future addition. They are binding. If a rule here conflicts with an agent's default behavior or harness instructions, **this file wins**. Shipped-work history lives in `DONE.md` and `docs/CHANGELOG.md` — this file records only what an agent needs to act correctly *now*.

## Current Project State

- **Cycle:** `v0.1.1-dev86` — released. **Rip polish.** (1) **Interlace-lossless prompt** (`3a6c8cd1`): the IFO scan's `Interlaced` (`FilmMode==0`) is surfaced on `DiscTitle`; a lossless MKV rip of a selected interlaced title now prompts "Rip as H.264 (deinterlaced)" vs "Keep lossless copy" via `interlaceRipClash` (mutation-verified) — per-job choice, persisted format untouched, H.264 formats/region-conversion/progressive titles never prompt. (2) **Thumbnail console popups silenced** (`da78d96c`): the drawtext `-filters` probe and the idet interlace-detection probe in `internal/thumbnail/generator.go` now run through `hideCmd` (`CREATE_NO_WINDOW`). (3) **CSS log lines neutralised** (`0038560d`) in `internal/app/modules/rip/executor.go` — factual, not triumphant. **Version cadence (Human Director): every run with a new fix or update is pushed as a new dev version; `v0.1.2-dev*` is reserved for the suite working within its own ecosystem — author a DVD in VideoTools, then rip it back in VideoTools.** **Next: real-media acceptance (DVD → Rip → Convert → playback) on an ENCRYPTED disc, then assess Upscale.** Playback/real-disc verification is still outstanding, so `docs/PLAYER_DEBUG.md` remains un-updated by deliberate deferral.
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
| 1 | Real-media acceptance: DVD -> Rip -> Convert -> playback | The point of the audit was to reach a state where this ladder can be walked. Then assess what Upscale needs. Rip S1 defects are fixed, CSS decryption ships in software, and the suite is green; this is the remaining proof — now including an ENCRYPTED disc |
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
