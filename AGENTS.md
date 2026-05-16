# VideoTools Agent Workflow Rules

These rules apply to any automation or agent working in this repo.

## Current Project State

- Current cycle: `v0.1.1-dev48`.
- Public/stable baseline: `v0.1.1`.
- `dev48` in progress. Theme system (internal/theme/), PillButton + PillIconButton, player transport controls migrated, text primitives, startup crash diagnostics, i18n script persistence, Windows signing wired, roadmap visual polish.
- `dev48` shipped so far: internal/theme/ package with VT_Navy palette, PillButton/PillIconButton widgets, text primitives (TitleLabel, SectionLabel, WrappingLabel, HintLabel, MonoLabel). MonoTheme + main.go reference theme vars. Player transport controls (speedBtn, subtitleBtn, play, volume, fullscreen, etc.) all migrated to PillButton/PillIconButton. Audio nil-widget crash fixed. Window recentering removed. Inuktitut script preference persists across restarts. Windows SignPath signing wired. VT_STARTUP_DEBUG crash diagnostics active.
- `dev47` closed. Rip: disc info display (type/region/size) at top of view, UDF ReadFileData for ISO region detection, progress bar with ETA, flat exe-dir DLL fallback, DLL/ folder rename (was ffmpeg-dll/), log boxes at bottom, Burn ConsoleBox, Author log truncation removed, Settings Module Chaining section, CI Linux FFmpeg build fixes. Audio Phase 1-3 fully shipped.
- `dev46` closed. PAL→NTSC full-disc conversion pipeline with IFO regeneration, Upscale preset overhaul, Audio Phase 2 (InlineVideoPlayer) + Phase 3 (track selection).
- `dev45` closed. Convert Phase 1+2 (SR, Normalize, Deinterlace, H.264 Profile/Level, presets, AVI/TS/FLV), Convert i18n, Module Pipeline (&&), logging audit, FFmpeg DLL bootstrap.
- `dev44` closed. Player thread-safety audit, D3D11VA, audio sync, GStreamer removal, Queue UI polish, thumbnail improvements, flags/i18n.
- `dev43` closed. Player thread-safety: pixel format SIGSEGV fix, Close/demuxer WaitGroup, NextFrame/Close codec race, seekLoop goroutine leak.
- `dev42` closed. Player stabilisation (D3D11VA, audio sync, GStreamer removal), thumbnail 3-way output, audio i18n.
- Issue tracker active at `https://git.leaktechnologies.dev/leak_technologies/VideoTools/issues`.
- Primary planning source is `TODO.md`; shipped scope is tracked in `DONE.md`; release-facing history is `docs/CHANGELOG.md`.
- **Player debug log:** `docs/PLAYER_DEBUG.md` — keep this up to date as issues are found and fixed. Update it before closing any player-related issue.

## Immediate Handoff Priorities

- **Remaining button migrations** — Compare (14), Audio (9), Rip (12), Filters (8), Upscale (5), Subtitles (23), Trim (16), Thumbnail (9), ui/queueview.go (20 buttons) still use widget.Button. Migrate module by module to PillButton.
- **Burn multi-drive batch** — Queue multiple ISOs across available burners. See `docs/BURN_MODULE_DESIGN.md` §Phase 2.
- **IMAPI2 COM replacement** — Replace isoburn.exe on Windows for proper progress/control. See `docs/BURN_MODULE_DESIGN.md` §Phase 3.
- **Main Menu refactor** — Extract `showMainMenu()` from root `mainmenu_module.go` into `internal/app/modules/mainmenu/`.
- **Linux CI speedup** — Pre-built container image for FFmpeg build dependencies.
- **C SEH bridge for D3D11VA crash** — P0 for player stability: C `__try`/`__except` wrapper around `avcodec_send_packet`/`avcodec_receive_frame`.
- **Do not expand scope beyond what is listed unless explicitly approved.**
- Keep the issue tracker in sync — close issues when work lands, open new ones for discovered bugs.

### Recently Shipped (dev48)
- **internal/theme/ package** — VT_Navy colour palette, PillButton, PillIconButton, text primitives (TitleLabel, SectionLabel, WrappingLabel, HintLabel, MonoLabel) extracted to `internal/theme/`. Both `ui/` and `media/` import from theme — no circular dependency. `ui/` re-exports for backward compat.
- **PillButton** — pill-shaped button with coloured border, hover/active/disabled states, bold text, initial-paint fix. Wired into main menu (History, Logs, &&), settings tabs (8 buttons), player Clear Video.
- **PillIconButton** — square icon-only pill button for transport controls and toolbar actions. Migrated play, volume, prev/next chapter, fullscreen, PiP, speed, subtitle buttons.
- **Text primitives** — `NewTitleLabel` (Monospace+Bold 24pt), `NewSectionLabel` (Bold), `NewWrappingLabel` (word wrap), `NewHintLabel` (Italic), `NewMonoLabel` (Monospace). `SectionHeader()` updated to use `NewSectionLabel`.
- **Player transport controls migrated** — speedBtn, subtitleBtn to PillButton; playBtn, volumeBtn, prev/nextChapter, fullscreenBtn, pipBtn to PillIconButton.
- **Audio nil-widget crash** — Guard against `Player.Widget()` returning nil.
- **Window recentering removed** — `CenterOnScreen()` removed from `maximizeWindow`; window stays where user placed it.
- **i18n script persistence** — Inuktitut syllabics/Latin preference survives app restarts via `ScriptPrefs` map in locale JSON.
- **Windows SignPath signing** — `SIGNPATH_API_TOKEN` + `SIGNPATH_ORGANIZATION_ID` both set in Forgejo secrets. ci-build.ps1 calls sign-exe.ps1 on every Windows build (non-fatal).
- **VT_STARTUP_DEBUG** — crash diagnostics env var traces widget CreateRenderer to stderr. Confirmed: STATUS_STACK_OVERFLOW is glfw.CreateWindow() GPU driver DLL injection, not VT code.

## Commit Discipline

- **ALWAYS stage and commit after every change. Do not wait for permission.**
- `git add -A` then `git commit -m "..."`.
- Do not leave unstaged changes in the worktree.
- Commit only files related to the current task.

## Documentation Discipline

Every time work lands, **all six documents must be updated in the same commit**:

| Document | What to update |
|---|---|
| `docs/roadmap.html` | Card status, cycle, desc; add new cards; update changelog/checklist data |
| `docs/ROADMAP.md` | Mermaid timeline entry; Current State prose; Now/Next sections |
| `docs/CHANGELOG.md` | New bullet under current dev cycle heading |
| `AGENTS.md` | Settled decisions, handoff priorities, current cycle state |
| `DONE.md` | New section entry for the completed feature |
| `TODO.md` | Check off completed items; add newly scoped items |

Additional rules:
- If behavior changes, also update `docs/INSTALLATION.md` and the relevant platform guide (`docs/INSTALL_WINDOWS.md`, `docs/INSTALL_LINUX.md`).
- `CHANGELOG.md` means `docs/CHANGELOG.md` in this repo (not the root).
- Avoid personal names; use `user report` or `dev report` only.
- The retired `docs.leaktechnologies.dev` site must not be used; active docs live in-repo and on the Forgejo wiki.

## Interactive Roadmap Maintenance (`docs/roadmap.html`)

The interactive roadmap is the **primary visual tracker** for the project. It must be kept current at all times. Both Claude and opencode are responsible for updating it whenever features land or scope changes.

### Card Status Values

| Value | Meaning |
|---|---|
| `shipped` | Merged, tested, confirmed working by a real user or tester |
| `done` | Code committed and CI green, but not yet tester-verified |
| `active` | Currently being worked on this cycle |
| `planned` | Scoped for an upcoming cycle — implementation not started |
| `future` | Deferred; no cycle assigned yet |

### What to Update When a Feature Lands

1. **Find the card** — search the `roadmap` JS object by `id` or `title`.
2. **Update `status`** — set to `done` when committed, `shipped` once tester confirms.
3. **Update `cycle`** — set to the current dev cycle (e.g. `'dev48'`).
4. **Update `desc`** — if the implementation detail differs from what was planned, rewrite it to match reality.
5. **Update `files`** — add or correct the key file paths.

### What to Update When New Work Is Scoped

Add a new card object to the correct module's `items` array:

```javascript
{ id: 'unique-kebab-id', title: 'Feature Name',
  desc: 'What it does and why it matters — one or two sentences.',
  status: 'planned', cycle: 'dev49',
  files: ['internal/path/to/relevant.go'],
  deps: ['id-of-dependency-card'],
  docs: ['docs/DESIGN_DOC.md'] },
```

### What to Update When a Dev Cycle Closes

1. Add a new entry to the **`changelogData` array** at the top (newest first) with the cycle's sections and bullet points. Use the actual technical detail from `docs/CHANGELOG.md` — not just card titles.
2. Add the cycle's items to the **`checklistData` array** so testers can verify them.
3. Update the **subtitle line** (`<p class="subtitle">`) with the new version and date.

### What NOT to Do

- Do not leave a card at `status: 'planned'` after the code is committed.
- Do not leave a card at `status: 'done'` indefinitely — chase the tester sign-off and move it to `shipped`.
- Do not add a card for something that is already shipped and was never tracked — just note it in `CHANGELOG.md` instead.
- Do not edit roadmap.html in isolation — always sync `ROADMAP.md`, `CHANGELOG.md`, `DONE.md`, `TODO.md` in the same commit.

### New Feature Documentation

- Create `docs/FEATURE_NAME.md` before implementation begins
- Include: overview, features, technical implementation, files to modify, testing checklist
- Link design doc in TODO.md and AGENTS.md immediate priorities
- Example: `docs/BURN_MODULE_DESIGN.md`, `docs/AUTO_GREY_CODECS.md`

## Internationalization (i18n)

All user-facing strings MUST use the i18n system. Never hardcode display strings.

### Rules

1. **All UI strings** must use `i18n.T().KeyName` — never use `"hardcoded string"`
2. **Add new strings** to `internal/i18n/strings.go` first (defines the key)
3. **Add English translation** to `internal/i18n/en_ca.go` (source of truth)
4. **Add French translation** to `internal/i18n/fr_ca.go`
5. **Add Inuktitut placeholders** to `internal/i18n/iu.go` (syllabics) and `internal/i18n/iu_latin.go` (Latin). Machine-generated translations are acceptable but must carry a `// machine-generated, needs human review` comment. See `docs/localization-policy.md` for the full policy.
6. **Module names** already localized: use `t.ModuleXxx` (e.g., `t.ModuleConvert`)
6. **Actions** already localized: use `t.ActionXxx` (e.g., `t.ActionSave`)
7. **Common labels** already localized: use `t.LabelXxx` (e.g., `t.LabelNoFile`)

### Common Patterns

```go
// Bad
label := widget.NewLabel("Click here")

// Good
label := widget.NewLabel(t.ActionClick)  // if action exists
// or add new key to strings.go
```

### Checking for Missing Strings

When adding a new feature, grep for hardcoded strings:
- `widget.NewLabel("...` 
- `dialog.Show...("...`
- Any user-visible text in quotes

### String Key Naming

- `ModuleXxx` — module names (ModuleConvert, ModuleAudio)
- `ActionXxx` — button/actions (ActionSave, ActionCancel)
- `LabelXxx` — static labels (LabelNoFile, LabelOutput)
- `StatusXxx` — status messages (StatusComplete, StatusFailed)
- `DialogXxx` — dialog titles (DialogConfirmDelete)

## Version Bumping

- After every major feature/change: bump the version (main.go, VERSION, FyneApp.toml).
- After bumping: update DONE.md, TODO.md, and CHANGELOG.md.
- Versioning model:
  - `v0.1.1-devN` is the rolling dev/nightly line
  - `v0.1.1` is the public stable baseline
  - dev numbering is continuous across public releases
  - the next public bump is based on release readiness, not number of dev iterations

## Windows Install Flow

- Use `scripts/windows/install.ps1` or `scripts/windows/install.bat` from PowerShell/CMD.
- `scripts/linux/install.sh` is for bash shells only; do not run it from PowerShell.

## CI Build — Settled Decisions (Do Not Revert)

The following decisions in `.forgejo/workflows/dev-packages.yml` were reached after multiple failed attempts and must not be changed without explicit approval:

**FFmpeg must be built from source — never use BtbN pre-built packages.**
BtbN Windows packages contain executables only, no static `.a` libraries.
BtbN Linux packages do not bundle `libx264.a`/`libx265.a`.
Both result in `cannot find -lx264/-lx265` linker errors in the CGO build.
`unzip` is not available in the MSYS2 bash environment — a BtbN `.zip` approach also fails immediately.

**x264 and x265 must be built from source as static-only.**
MSYS2's prebuilt x264/x265 packages install headers with `__declspec(dllimport)`,
causing `libavcodec.a` to reference `__imp_x264_*` symbols that can never be satisfied
in a static link.

**x265.pc must be overwritten after cmake install.**
CMake writes Windows-style paths and CRLF line endings that MSYS2 pkg-config cannot parse.
It also omits the C++ runtime from Libs. The echo-command overwrite (LF, POSIX paths) is required.
C++ deps (`-lstdc++ -lsupc++ -lm` on Windows / `-lstdc++ -lm` on Linux) must go in **`Libs`**
(not `Libs.private`) because FFmpeg configure calls `pkg-config --libs` without `--static`,
so `Libs.private` is never included in the configure link test. On Linux, `-lstdc++` resolves
to libstdc++.so.6 which exports operator new, RTTI vtables etc. On Windows, `-lstdc++` provides
the DLL import stub (for std::__throw_length_error) and `-lsupc++` provides the GCC-private static
archive (for operator new/delete, __cxa_guard, RTTI vtables).

**cmake must be in the Linux apt-get install deps** (for x265 source build).
**nasm and mingw-w64-ucrt-x86_64-cmake must be installed in the Windows MSYS2 step** (for x264/x265 source builds).

If CI is failing, read the build log carefully before changing the FFmpeg setup strategy.
Open an issue or ask before touching the "Setup static FFmpeg" steps.

## Roadmap & Planning — Settled Decisions (Do Not Revert)

The following decisions about the interactive roadmap (`docs/roadmap.html`) and feature tracking were reached after multiple audit passes and must not be changed without explicit approval:

**Interactive roadmap is the single source of truth.** The roadmap HTML board at `docs/roadmap.html` is the canonical feature tracker. TODO.md and DONE.md are narrative supplements; keep them in sync when landing work, but always update the roadmap first.

**Obsolete output formats (AVI, FLV, 3GP, OGG) are NOT output targets.** These exist only as `Legacy: true` entries in `internal/convert/presets.go` for remuxing legacy files. They have been removed from the roadmap entirely. Do not add them back.

**GStreamer was fully removed in dev42.** All `internal/player/gstreamer*` files, build scripts, and CI references were deleted. The native_media build tag is the only player path. Any stale GStreamer references in docs are historical and should be cleaned up when found.

**x264/x265 tuning presets shipped in dev45.** Film, Animation, Grain, Stillimage, Fastdecode presets are fully wired in the Convert module. Do not list this as future/planned work.

**Presets consolidation shipped in dev45.** Format definitions moved from inline builder code to `internal/convert/presets.go`. Do not list this as future/planned work.

**Drag-and-drop for Convert shipped in dev44.** Files dropped onto the Convert module are registered correctly. Do not list this as a known issue.

**Queue notifyChange race fix shipped in dev45.** The goroutine spawning without lock in `internal/queue/queue.go` was fixed. Do not list this as a known issue.

**Audio pre-warm shipped in dev42**, not dev47. Shared audio context (oto/WASAPI) initialized at startup was part of the original player stabilization cycle.

**PAL/NTSC full-disc conversion shipped in dev47**, not dev46. The IFO regeneration pipeline with full-disc extraction was completed in dev47.

**Testing checklist lives in the roadmap.** The interactive roadmap at `docs/roadmap.html` includes a Testing Checklist modal (button next to Changelog). Items are grouped by module with pass/fail/untested status persisted to localStorage. Any new feature added to the roadmap must also be added to the testing checklist. The checklist server-side data (`checklistData` array in the roadmap JS) should be updated when features land.

## Player P0: D3D11VA Crash — C SEH Bridge Needed

The D3D11VA hardware decoder path in `internal/media/` has an access violation inside `avcodec_send_packet` when the FFmpeg D3D11VA hwaccel encounters corrupted/malformed H.264 NAL units (observed on some commercial DVDs and damaged files). This is a CGo boundary issue — Go's signal handling cannot recover from a Windows SEH exception raised inside native code.

A C SEH bridge (`__try`/`__except`) must be written in a `.c` file (compiled via CGO) that wraps the `avcodec_send_packet` / `avcodec_receive_frame` call pair and translates the access violation into a returned error code. This does not exist yet and is P0 for player stability.

**Approach:**
1. Create `internal/media/cbridge/sehbridge.c` and `internal/media/cbridge/sehbridge.h`
2. Export a C function like `int avcodec_send_packet_safe(AVCodecContext *ctx, const AVPacket *pkt)` that wraps the call in `__try`/`__except(EXCEPTION_EXECUTE_HANDLER)`
3. Call the bridge from Go via `// #include "sehbridge.h"` + `// #cgo LDFLAGS:` in a Go file
4. On error return, force software decode fallback for the remainder of playback

Until this bridge exists, users with problematic discs should disable D3D11VA in Settings or use software decode mode.

## Coordination

- Ask before changing workflow entrypoints or automation behavior.
- If a change affects installs/builds, add a short note in docs.
- Keep Forgejo release publishing aligned to `VERSION`; do not retarget releases to older dev tags.
- Be careful with tag/release operations — do not retarget or delete existing dev tags.
- Old workflow runs must not be used as evidence of current release state.

## Repository Hygiene

- Keep the repository root minimal. Root should contain only core project manifests, primary app entry source, and top-level docs (`README.md`, `TODO.md`, `DONE.md`).
- Put demos/tools under `cmd/` or `scripts/` (not the root).
- Put platform packaging assets under `packaging/<platform>/`.
- Do not commit ad-hoc logs, scratch files, backup files, or one-off test files to root.

### No New Root-Level Go Files — Hard Rule

**Do not add new `.go` files to the repository root.** The root already has 38 files in `package main`; every new file added there deepens the coupling and makes the eventual `internal/` migration harder.

When adding new functionality:
- Create it in the appropriate `internal/` sub-package from the start.
- If it belongs to an existing module (e.g. convert, player, author), add it to that module's directory under `internal/app/modules/` or create a new one.
- If it is truly app-level wiring (the thin glue between `main` and a module), it may live in a *temporary* root shim file — but that shim must be listed in the refactor plan (`docs/REFACTOR_DEV30_PLAN.md`) as a future extraction target.
- Types, helpers, and utilities that don't depend on `appState` must go in `internal/` immediately — there is no justification for adding them to root.

If you are unsure where new code belongs, default to `internal/` and ask rather than defaulting to root.

## Refactor Boundaries

- Current refactor plan: `docs/REFACTOR_DEV30_PLAN.md`.
- Phase 2 complete. Phase 3 in progress — **opencode owns Phase 3 going forward**.
- Completed Phase 3 slices (do not re-extract these):
  - `internal/app/modules/about` — About dialog
  - `internal/app/modules/deps` — Missing deps dialog
  - `internal/app/modules/mainmenu` — Main menu helpers
  - `internal/app/modules/convert` — `ShowView`, `ConvertState`, `ConvertCallbacks` entry point; full `buildConvertView` deferred (high appState coupling)
  - `player_module.go` — `showPlayerView` + `buildPlayerView`
  - `enhancement_module.go` — `buildEnhancementView` (placeholder)
  - `upscale_module.go` — full upscale view + AI helpers
  - `compare_module.go` — `showCompareView`, `showCompareFullscreen`, `buildCompareView`, `buildCompareFullscreenView`
- `main.go` is now ~16,925 lines. Remaining large blocks:
  - `buildConvertView` (~3,500 lines) — entry point shim exists; full extraction requires appState decoupling first
  - Inspect view (`showInspectView` + `buildInspectView`)
  - Settings view (`showSettingsView` + `buildSettingsView`)
  - Queue view (`showQueue` + related)
  - Remaining `*_module.go` candidates: `filters_module.go`, `subtitles_module.go` (already separate files, no further extraction needed unless moving to `internal/`)
- Pattern for pure-move slices (no `appState` interface change needed):
  1. Create `<name>_module.go` in `package main`
  2. Move `show*` and `build*` functions verbatim
  3. Copy only the imports the moved functions need
  4. Remove the block from `main.go`
  5. Commit as a single focused slice
- Continue using thin `package main` shims while moving logic out of root files.
- Prefer small, reversible refactor slices. Do not combine structural moves with unrelated feature work.
- The long-term goal remains:
  - reduce root-level `package main` clutter
  - move app logic into `internal/app/`
  - move the executable entrypoint toward `cmd/videotools/`

## Native Media Player

Architecture reference: `docs/NATIVE_PLAYER.md` — read this before touching any player code.

### The One Rule

**All modules must use `ui.InlineVideoPlayer` as their API layer. Period.**

Do not call `media.NewEngine()` inside a module. Do not write a per-module playback goroutine. Do not call `media.NewVideoPlayer()` directly. Get the widget via `opts.Player.Widget()`.

### Three-Layer Stack

```
internal/media   Engine          — CGo/FFmpeg demux + decode + audio (oto v3)
internal/media   VideoPlayer     — Fyne widget: renders frames, built-in controls overlay
internal/ui      InlineVideoPlayer — THE API layer every module talks to
```

### Adding a New Module with Video

1. Declare a singleton `*ui.InlineVideoPlayer` in `native_media.go` (real) and `native_media_stub.go` (stub returning `ui.NewInlineVideoPlayer()`).
2. Add `Player *ui.InlineVideoPlayer` to the module's `Options` struct in **both** `view.go` and `stub.go`.
3. Wire `Player: GetXxxPlayer()` in `main.go`.
4. Use only `InlineVideoPlayer` methods inside the module — see `docs/NATIVE_PLAYER.md` for the full API surface.

### Current Module Singletons

| Module  | Getter              | Notes |
|---------|---------------------|-------|
| Convert | `GetConvertPlayer()` | Custom control row; `DisableBuiltinControls()` called |
| Trim    | `GetTrimPlayer()`    | Built-in controls overlay; in/out markers; preview region via `SetOnProgress` |
| Inspect | `GetInspectPlayer()` | Built-in controls disabled; `SetOnLoad` wired to Sync tab load-state pills |

### What the `native_media` Build Tag Gates

- `native_media.go` — real singletons, `HasNativeMediaPlayer() → true`, CGo player methods
- `native_media_stub.go` — no-op stubs, `HasNativeMediaPlayer() → false`
- `internal/media/` — CGo engine and widget (only compiled with tag)
- `internal/ui/inline_player.go` — real player (only compiled with tag)
- `internal/ui/inline_player_stub.go` — no-op stub (only compiled without tag)

Stub implementations must expose the **identical** method set as the real type so all callers compile on every target.

### Reference in `AGENTS.md` Line 158

The entry `player_module.go` in the completed Phase 3 list refers to the root-level `player_module.go` (shows the legacy static player view), **not** to the native media player stack above.

### Approved Exceptions to InlineVideoPlayer Rule

The following modules create `media.Engine()` directly instead of using `InlineVideoPlayer`. This is intentional because they require **two simultaneous video streams** which `InlineVideoPlayer` does not support.

| Module | File | Reason |
|--------|------|--------|
| Compare | `compare/fullscreen_native.go` | Side-by-side video comparison requires two independent engines |
| Upscale | `upscale_player_native.go` | Preview window + main player require two engines with custom playback loops |

**Do NOT refactor these modules to use InlineVideoPlayer** — the dual-engine architecture is required for their functionality.

## Using Sub-Agents

Claude Code supports spawning purpose-built sub-agents for parallelising independent work or protecting the main context window from large search results.

### Available Agent Types

| Type | Use for |
|------|---------|
| `Explore` | Fast read-only code search — finding files by pattern, grepping for symbols, answering "where is X defined?" Don't use for open-ended analysis or cross-file consistency checks. |
| `Plan` | Architecture and implementation planning before writing code. Returns step-by-step plans and identifies critical files. |
| `general-purpose` | Multi-step research, searches across the codebase, any task that may need several rounds of tool use. |
| `claude-code-guide` | Questions about the Claude Code CLI, Claude API, or Anthropic SDK. |

### When to Spawn a Sub-Agent

- **Multiple similar fixes** across many files (e.g. same CI error in 10 modules)
- **Independent tasks** with no shared state (parallelize freely)
- **Large codebase searches** that would flood the main context
- **Architecture review** before starting a feature (use `Plan`)

### Correct Invocation Syntax

Sub-agents are invoked via the `Agent` tool — not as Go code:

```
Agent(
  description="Fix inspect module build errors",
  subagent_type="general-purpose",
  prompt="Fix all build errors in internal/app/modules/inspect/. ..."
)
```

To run agents in parallel, emit multiple Agent tool calls in a single response.

### Guidelines

- Brief the agent like a colleague who has no prior context — include file paths, what was tried, and the expected outcome.
- Verify results: an agent's summary describes intent, not necessarily what it did. Read diffs before reporting work complete.
- Prefer foreground agents when their result informs the next step; use `run_in_background: true` only for genuinely independent work.
- After all agents complete, verify the build passes before committing.

## Hooks & Automation

Claude Code hooks are shell commands that fire at defined lifecycle points (before/after tool calls, before commit, etc.). Configure them in `.claude/settings.json` under the `hooks` key, or use `/update-config` to add them interactively.

### Recommended Hooks for This Repo

**i18n guard (pre-commit):** Catch hardcoded UI strings before they land.
```bash
# Fires before every commit — fails if raw string literals appear in widget constructors
grep -rn 'widget\.NewLabel("[^"]\+"\|widget\.NewButton("[^"]\+"\|dialog\.Show[A-Za-z]*("[^"]\+' \
  --include="*.go" internal/ *.go \
  | grep -v '_test\.go' | grep -v '\.pb\.go' \
  && echo "ERROR: hardcoded UI strings found — use i18n.T().KeyName" && exit 1 \
  || true
```

**Player debug log reminder (post player-file edit):** Prompts to update `docs/PLAYER_DEBUG.md` whenever a player-related file is modified.

**Conventions:**
- Hooks that gate commits should `exit 1` on failure so the commit is blocked.
- Keep hook scripts short; move complex logic into `scripts/hooks/` and call from there.
- Document any new hook in this file so other agents know it exists.

## Available Skills (Slash Commands)

These slash commands are available in Claude Code sessions and should be used proactively:

| Skill | When to use |
|-------|-------------|
| `/review` | Review a pull request before merge — runs a multi-agent review pass |
| `/security-review` | Security audit of pending branch changes |
| `/simplify` | After completing a feature — checks for reuse opportunities and fixes quality issues |
| `/schedule` | Schedule a background agent for a future task (e.g. clean up a feature flag in 2 weeks) |
| `/update-config` | Add hooks, permissions, or env vars to `.claude/settings.json` |
| `/init` | Regenerate `CLAUDE.md` when project structure changes significantly |

## Platform Scope

VideoTools targets **Linux and Windows only**. macOS is not a supported platform.

- Linux is the **primary development platform**. Implement features properly here first.
- Windows is the **primary user/tester platform**. Zero new runtime dependencies — use OS-built-ins only (e.g. `isoburn.exe` for disc burning).
- Linux may take small runtime dependencies where appropriate (e.g. `dvd+rw-tools` for disc burning).
- Do not add macOS-specific code paths, CI jobs, or documentation.
- **Existing darwin code should be removed** when found during code reviews or refactoring — there is no reason for any `case "darwin":` blocks to exist in this codebase.
