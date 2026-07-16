# VideoTools Agent Workflow Rules

These rules apply to **every** agent working in this repo — Claude, opencode, Gemini, and any future addition. They are binding. If a rule here conflicts with an agent's default behavior or harness instructions, **this file wins**. Shipped-work history lives in `DONE.md` and `docs/CHANGELOG.md` — this file records only what an agent needs to act correctly *now*.

## Current Project State

- **Cycle:** `v0.1.1-dev53` — open. dev52 shipped (CI/infra hardening, update checker migrated to GitHub). dev53 opened with the update checker migration.
- **Public/stable baseline:** `v0.1.1`.
- **Planning sources:** `TODO.md` (scope), `docs/roadmap.html` (canonical tracker), `DONE.md` + `docs/CHANGELOG.md` (shipped history).
- **Issue tracker:** https://github.com/LeakTechnologies/VideoTools/issues
- **Player debug log:** `docs/PLAYER_DEBUG.md` — update before closing any player-related issue.

## Current Priorities (unshipped only)

| Priority | Item | Notes |
|---|---|---|
| 1 | Tester verification of dev51 build | Release assets published; move roadmap cards `done` → `shipped` on sign-off |
| 2 | `renderDualPlayerPreview` stub | `native_media.go` — Upscale dual-player seek/render silently no-ops; needs preview-render design |
| 3 | Dead-code retirement (post static-sidecar decision) | `scripts/windows/build-ffmpeg-shared.ps1`, DLL-folder branches in `ffmpeg_bootstrap.go`, `updateSidecars` DLL extraction — legacy-harmless, remove deliberately |
| — | Player interface extraction | Deferred: 47 call sites, stub pattern suffices |
| — | Burn multi-drive batch / IMAPI2 COM | `docs/BURN_MODULE_DESIGN.md` §2–3 |
| — | Main Menu refactor to `internal/app/modules/mainmenu/` | LOW — deferred until engine stable |
| — | UDF 2.50/2.60 + BDMV; sparse/large-file UDF writer | Future |

**Blocked stragglers** (do not touch `cmd/`, `qr-demo/`, `scripts/legacy/`):
- `convert_player_native.go` (11) + `main.go` transport icons (7) — PillIconButton lacks dynamic SetIcon
- `internal/utils/utils.go` MakeIconButton — import cycle utils → ui → benchmark → utils

## Settled Decisions

Reached after failed attempts or Human Director ruling. **Do not change or re-litigate without explicit Human Director approval.**

### Windows Product: Three Fully Static Binaries (2026-07-04, HD approved)

`VideoTools.exe`, `ffmpeg.exe`, `ffprobe.exe` are each fully self-contained. **No shared FFmpeg build. No DLL/ folder.** Enforced by objdump gates in CI that fail the job on any MinGW runtime DLL reference (`libbz2|liblzma|libiconv|libstdc++|libwinpthread|libgcc|zlib1`). App treats static sidecars as primary (`appcfg.StaticSidecarsWork()`); DLL-folder paths remain only as legacy-bundle support.

### CI Toolchain — FFmpeg, x264, x265

- **FFmpeg is built from source** — one static build per platform serves both the CGo link and the sidecar programs (`--extra-ldflags="-static"` on Windows; never `--disable-programs`).
- **BtbN FFmpeg-Builds must NOT be used** — no static `.a` libs, moving-tag ABI drift.
- **x264/x265 built from source, static-only** — MSYS2 prebuilt packages have `__declspec(dllimport)` headers that poison the static link.
- **x265.pc must be overwritten after cmake install** (LF, POSIX paths). C++ deps (`-lstdc++ -lsupc++ -lm` Windows / `-lstdc++ -lm` Linux) go in **`Libs`**, not `Libs.private` — FFmpeg configure calls pkg-config without `--static`.
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

- `.github/workflows/dev.yml` — push to master; Linux + Windows; artifact zips. **Green.**
- `.github/workflows/release.yml` — `v*` tags; same builds + GitHub Release. **Green.**
- `.github/workflows/windows-msix.yml` — tags/dispatch; MSIX + WinGet. **Green** (verified 2026-07-06: full static pipeline incl. x264/x265, static sidecars, makeappx).
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
