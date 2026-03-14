# VideoTools TODO (v0.1.1-dev33 plan)

This file tracks upcoming features, improvements, and known issues.

## Dev33 Scope

- [x] **App icon embedding** — `logo_embed.go` at root; logo loaded directly in `main.go`; fixes taskbar icon not showing.
- [x] **FFmpeg hwaccel order fix** — hwaccel flags now passed before the input file in convert pipeline.
- [x] **Upscale Ctrl+Enter shortcut** — Added keyboard shortcut to Upscale module.
- [x] **Version hash in Settings** — Settings > About now shows build commit hash for debugging.
- [x] **Windows CI buildCommit** — Windows CI workflow now passes `-ldflags "-X main.buildCommit=..."` (Linux already had this).
- [x] **CI syntax error fix** — Duplicate `else` block in icon loading code removed; Windows package build now passes.
- [x] **Linux CI apt caching** — Added `actions/cache@v4` for apt packages to speed up Linux CI builds.
- [x] **Wiki maintenance** — Synchronized `docs/` to Forgejo wiki; established `Home.md`, `Documentation.md`, and `_Sidebar.md`.
- [ ] **Native Disc Authoring (Issue #21)** — Implement native Go replacement for `dvdauthor`, `spumux`, and `xorriso`.
    - [ ] Phase 1: UDF 1.02 / ISO 9660 Bridge writer.
    - [ ] Phase 2: MPEG-PS / VOB muxer with NAV_PCK support.
    - [ ] Phase 3: IFO/BUP structure generation.
    - [ ] Phase 4: SPU (subpicture) encoding for menu buttons.
- [ ] **Root folder hygiene** (issue #22) — 24 `package main` files clutter the project root; should be progressively extracted to `internal/app/modules/` with thin root shims, following the pattern already established for `about`, `deps`, and `mainmenu`.
- [ ] **Drag and drop into Convert** — Files dragged onto the Convert module drop zone not being registered (carry-forward from dev32).
- [ ] **Linux CI build optimization** — Even with apt caching, Linux builds install ~100 packages. Consider a pre-built container image with deps baked in to cut build times.

## Dev32 Scope

- [x] **Drag-to-scroll** (issue #19) — Fixed by replacing inner `container.Scroll` in `FastVScroll` with a custom `scrollClip` widget that does not implement `fyne.Draggable`; drag events now reach `FastVScroll`.
- [x] **Windows icons** (issue #20) — Icons embedded via `//go:embed assets/icons`; `GetIcon` reads from embedded `fs.FS` with no runtime disk access.
- [x] **Updates tab — real check** — Wired to Forgejo tags API (`/api/v1/repos/leak_technologies/VideoTools/tags?limit=1`); fixed owner mismatch in URL.
- [x] **Dependency install buttons** — Per-dependency Install/Uninstall buttons in Settings.
- [x] **SVG icon library** — Material Design icons added to `assets/icons/`.
- [x] **Convert UI cleanup** (issue #5) — Label alignments and separators standardised.
- [x] **Hide/show player in Compare** (issue #1) — Toggle button added to Compare module.
- [x] **Author/Rip hidden on Windows** — Disc modules hidden from main menu on Windows until cross-platform disc authoring is implemented.
- [x] **Convert player layout** — Video fills centre of player pane; transport bar pinned to bottom; VSplit gap removed.
- [x] **Convert active state** — `s.active = "convert"` now set correctly; drop handling and keyboard shortcuts work inside the module.
- [ ] **Drag and drop into Convert** — Files dragged onto the Convert module drop zone not being registered (opencode investigating).
- [ ] **Phase 3 modularisation — Inspect, Settings, Queue** (opencode)

## Dev31 Scope

- [x] **Module settings panel scrolling** (issue #3)
  - Scroll containers added to all non-Convert module settings panels. Action buttons moved to footer action bar.
- [x] **Window resize stability** (issue #4)
  - setContent now pins window size after SetContent to prevent layout-driven resize on module switch.
- [ ] **Convert UI cleanup** (issue #5)
  - Layout consistency, label clarity, and control organization pass for external developer testing readiness.
- [ ] **Phase 3 modularisation — Inspect, Settings, Queue** (opencode)
  - Move `buildInspectView`, `showSettingsView`/`buildSettingsView`, and `showQueue`/queue view out of `main.go` into dedicated `*_module.go` files.
  - See `AGENTS.md` Refactor Boundaries for the pattern and completed-slice list.
  - **Note**: Convert module partially modularized (entry point + state/callbacks in `internal/app/modules/convert/view.go`). Full `buildConvertView` extraction deferred due to high coupling with appState (~3500 lines, ~30+ state fields). Future work should consider extracting logical subsections first.

## Near-Term Milestone: Frame Interpolation (RIFE)

**Issue #23** — Add RIFE frame interpolation as a section in the Upscale module (or its own module later).

RIFE (Real-Time Intermediate Flow Estimation) increases frame rate by synthesising intermediate frames.
It uses `rife-ncnn-vulkan` — same deployment model as `realesrgan-ncnn-vulkan`.

- [ ] **Detect rife-ncnn-vulkan** — check `$PATH` at startup; surface install link in Settings if missing.
- [ ] **UI: Frame Interpolation section** — multipier select (2×/4×/8×), show estimated output fps.
- [ ] **Pipeline** — extract frames → rife-ncnn-vulkan → reassemble (can chain after Real-ESRGAN upscale).
- [ ] **Settings install button** — same pattern as Real-ESRGAN dependency install button.

## Near-Term Milestone: Cross-Platform Video Player

**Priority: High** — blocks Upscale (live preview) and Trim (frame-accurate timeline) reaching their full potential.

The current player stack uses GStreamer on Linux and a separate fallback on Windows. Before Upscale and Trim can be completed properly, a unified player that works identically on both platforms is needed.

- [ ] **Evaluate player backends** — shortlist: MPV (via libmpv), FFplay wrapper, VLC bindings. Needs: frame-accurate seeking, A/V sync, hardware decode, cross-platform.
- [ ] **Design unified player interface** — abstract `vtplayer` interface that all backends implement; modules talk only to the interface.
- [ ] **Implement chosen backend** — wire up on both Linux and Windows; replace the GStreamer-only path.
- [ ] **Upscale live preview** — depends on player foundation; real-time before/after preview during upscale processing.
- [ ] **Trim module** — depends on player foundation; frame-accurate timeline, in/out points, chapter markers.
- [ ] **Linux CI cleanup** — once GStreamer is no longer mandatory, remove it from the CI dependency install list and cut ~2 min off Linux build times.

## Agent Work Tracking

Items currently being worked on by other agents — update when assignments change.

| Agent    | Current Task                                              | Status      |
|----------|-----------------------------------------------------------|-------------|
| opencode | Drag and drop into Convert (issue carried from dev32)     | In progress |
| opencode | Phase 3 modularisation — Inspect, Settings, Queue         | In progress |
| gemini   | *(unknown — update this)*                                 | Unknown     |

## Maintenance

- [ ] **Root folder hygiene (issue #22)** — The project root currently has 27 `.go` files, only 3 of which legitimately belong there:
  - **Must stay at root** (go:embed requires paths relative to the file): `main.go`, `icons_embed.go`, `logo_embed.go`
  - **Platform-specific shims that can stay near main**: `update_windows.go`, `update_linux.go`, `platform.go`
  - **Should move to `internal/app/modules/{module}/`** with thin root shims (following the `about`, `deps`, `mainmenu` pattern already established): `audio_module.go`, `author_module.go`, `author_dvd_functions.go`, `author_menu.go`, `compare_module.go`, `enhancement_module.go`, `filters_module.go`, `inspect_module.go`, `player_module.go`, `queue_module.go`, `rip_module.go`, `settings_module.go`, `subtitles_module.go`, `thumbnail_module.go`, `upscale_module.go`
  - **Config/helpers to move**: `merge_config.go`, `naming_helpers.go`, `thumbnail_config.go`, `deps_dialog_module.go`
  - **Approach**: Extract logic incrementally per module; keep `package main` shims at root until `appState` coupling is reduced enough to move the entry point to `cmd/videotools/`.


- [X] **About dialog cleanup**
  - Remove the Bitcoin address from the About/Support page.
- [X] **Snippet AV1 fallback**
  - Fall back to available AV1 encoders (or H.264) when `libsvtav1` is missing.
- [X] **Adaptive scroll speed**
  - Smooth settings and long panels across different window sizes.
- [X] **Click-and-drag scrolling**
  - FastVScroll supports click-and-drag (desktop.Mouseable + fyne.Draggable) for mobile/touch-like interaction.
- [X] **Settings tab scrolling**
  - Scroll within tabs while keeping the Settings header visible.
- [X] **Aspect ratio handling**
  - Use display aspect ratio metadata when available and surface a 17:9 target for non-16:9 sources (user report).
- [X] **Aspect ratio UI clarity**
  - Show the detected source aspect ratio next to the Source aspect option.
- [X] **Aspect ratio logging + crop hygiene**
  - Log source/target aspect details and ignore stored crop values when auto-crop is disabled.
- [X] **Custom aspect ratio input**
  - Provide a Custom... option instead of bloating the dropdown with cinema ratios.
- [X] **Aspect/scale alignment**
  - Ensure aspect conversion uses target resolution so outputs land on exact sizes (e.g., 1920x1080).
- [X] **Window resize stability**
  - Avoid resizing the main window on every module switch to prevent misclicks.
- [X] **Conversion stability**
  - Catch conversion worker panics and surface a failure dialog instead of closing the UI.
- [X] **Conversion recovery notice**
  - Persist the last conversion state and show a lightweight notice on next launch if it was active.
- [X] **Convert compile regressions**
  - Fixed duplicate aspect/scale block and late custom-aspect declarations in `main.go`.
- [X] **Windows import regression**
  - Removed stale `go-qrcode` import in `main.go` after About module extraction.
- [X] **Windows Tesseract packaging fallback**
  - Download missing `eng/fra/iku` traineddata during bundled packaging before enforcing required checks.
- [X] **GStreamer CI packaging policy**
  - Treat GStreamer as optional in bundled artifacts and do not fail packaging when absent.
- [X] **Whisper model packaging resilience**
  - Added multi-source fallback download and continue-on-failure behavior in bundled packaging.
- [X] **Linux bundled zip tooling**
  - Added `zip` to Linux workflow dependency install list.
- [X] **Dev pipeline simplification**
  - Skip bundled package generation on dev channel; keep bundled artifacts for stable channel only.
- [X] **Bundled artifact retirement**
  - Removed bundled package generation from Linux/Windows dev-packages workflow.
- [X] **Main menu width containment**
  - Replaced incorrect adaptive-grid usage with wrapping tile layout to prevent oversized windows.
- [X] **Main menu row alignment**
  - Set module sections to a stable 3-column grid for consistent row structure.
- [X] **Release asset replacement**
  - Fixed purge + name-based asset deletion in publish workflow to prevent release zip accumulation.
- [X] **Publish delete endpoint fix**
  - Updated Forgejo asset delete endpoint path to stop 404 failures in publish job.
- [X] **Release tag targeting**
  - Forgejo release publish now sources version from `VERSION` first and updates only the matching tag release metadata.
- [X] **FFprobe path consistency**
  - Use configured/app-local FFprobe paths in convert analysis and thumbnail metadata probes instead of hardcoded `ffprobe`.
- [X] **Release note readability**
  - Publish concise highlights from the matching changelog version section instead of embedding the full raw section in Forgejo release comments.
- [X] **Stale publish run guard**
  - Skip dev release publish when a newer `master` commit exists so old runs cannot overwrite newer release/tag state.
- [X] **Docs portal migration**
  - Repoint About/QR documentation links and install docs from retired `docs.leaktechnologies.dev` to Forgejo wiki + in-repo docs.
- [X] **Blu-ray visibility toggle**
  - Added `Show Blu-ray module` preference and tied it to main menu module visibility.
- [X] **Benchmark scope clarity**
  - Benchmark apply path now changes hardware acceleration only (not codec or preset).
- [X] **Versioning docs clarity**
  - Documented continuous global `-devN` numbering and public release base-version behavior.
- [X] **Module testing coverage checklist**
  - Added a full module-by-module release checklist in `docs/TESTING_MODULE_CHECKLIST.md`.
- [X] **Root structure hygiene pass**
  - Removed root-level scratch artifacts, moved QR demo entry to `cmd/qr_about_demo/`, and documented root cleanliness rules in `AGENTS.md`.
- [ ] **Dev30 phased refactor (build-safe)**
  - [x] Phase 2a: Moved module config path helper to `internal/app/configpath` and updated module call sites.
  - [x] Phase 2b: Moved merge/thumbnail config persistence logic into `internal/app/modulecfg` with compatibility wrappers in `package main`.
  - [x] Phase 2c: Moved naming metadata/output-base helper logic into `internal/app/naming` with wrappers in `package main`.
  - [x] Phase 2d: Moved rip/subtitles config persistence logic into `internal/app/modulecfg` with compatibility wrappers in module files.
  - [x] Phase 2e: Moved author config persistence logic into `internal/app/modulecfg` with compatibility wrappers in `author_module.go`.
  - [x] Phase 2f: Moved audio config persistence logic into `internal/app/modulecfg` with compatibility wrappers in `audio_module.go`.
  - [x] Phase 2g: Replaced duplicated config path helpers in `main.go` with shared `internal/app/configpath` usage for convert/recovery/benchmark/history.
  - [x] Phase 2h: Moved recovery/benchmark/history config persistence logic into `internal/app/appcfg` with aliases/wrappers in `main.go`.
  - [x] Phase 2i: Moved convert config JSON load/save plumbing to shared `internal/app/appcfg` store helpers while keeping convert normalization logic in `main.go`.
  - [x] Phase 2j: Moved convert config normalization rules into `internal/app/appcfg` and kept `main.go` as a thin wrapper.
  - [x] Phase 3a: Moved About dialog implementation to `internal/app/modules/about` with a root shim.
  - [x] Phase 3b: Moved missing-dependencies dialog rendering to `internal/app/modules/deps` with a root shim.
  - [x] Phase 3c: Moved main menu visibility/dependency filtering and active-job mapping helpers to `internal/app/modules/mainmenu`.
  - [x] CI fix: Restored missing `path/filepath` import in `audio_module.go` after refactor to fix Linux/Windows package build failures.
  - [x] CI fix: Restored missing `path/filepath` import in `rip_module.go` after refactor to fix Linux/Windows package build failures.
  - [x] CI fix: Restored missing `encoding/json` and `path/filepath` imports in `subtitles_module.go` after refactor to fix Linux/Windows package build failures.
  - Phase 2: Introduce `internal/app/` boundaries and move low-risk helper files from root.
  - Phase 3: Move module builders into `internal/app/modules/` with compatibility shims if needed.
  - Phase 4: Move primary app entrypoint toward `cmd/videotools/` once app package is stable.
- [ ] **Main.go modularization pass** - Move UI builders and helpers into module files (convert remaining)
- [X] **Main menu modularization** - Moved main menu builder + refresh helpers into `mainmenu_module.go`.
- [X] **Settings modularization** - Settings view lives in `settings_module.go` (no main.go UI builder).
- [X] **About/Support modularization** - Moved About/Support dialog into `about_module.go`.
- [X] **Missing dependencies dialog modularization** - Moved the missing dependencies dialog into `deps_dialog_module.go`.
- [X] **Queue view modularization** - Moved queue view builders and refresh helpers into `queue_module.go`.
- [X] **Inspect view modularization** - Moved inspect view into `inspect_module.go`.
- [X] **Subtitle OCR support**
  - Enable OCR for image-based DVD/BD subtitle tracks (VobSub/PGS) to produce SRT/ASS.
- [X] **Subtitle OCR cleanup**
  - Normalize OCR text and merge consecutive duplicate cues for cleaner timing.
- [ ] **Windows dev28 dependency bootstrap validation**
  - Verify first-run FFmpeg bootstrap on a clean Windows machine with no FFmpeg in PATH.
  - Confirm modules unlock immediately after in-app install without restarting the app.
- [ ] **Cross-platform dependency actions validation**
  - Verify Settings > Dependencies FFmpeg install/uninstall buttons behave correctly on Linux package managers (apt/dnf/pacman/zypper).
- [ ] **Convert UI icon pass**
  - Replace temporary ASCII-safe Convert control labels with proper icon resources once the icon set is finalized.

- [ ] **Git converter retirement**
  - Preserve presets, then deprecate `scripts/legacy/git_converter`.
- [ ] **Forgejo Windows runner validation**
  - Confirm the Windows packaging workflow completes without context canceled after the UTF-8 `GITHUB_OUTPUT` fix.
- [ ] **Forgejo Windows package validation**
  - Confirm Windows zip contains only `VideoTools.exe` and `README.md`.
  - Confirm Windows zip is written to `dist/windows/` with no build metadata files.
  - Confirm diagnostics folder is not included in uploaded artifacts.
- [ ] **Forgejo dev release validation**
  - Confirm dev release workflow builds Linux and Windows artifacts before publish steps.
  - Confirm release assets exclude build metadata files.
  - Confirm release assets replace existing files with the same name.
  - Confirm release assets are purged before upload (no duplicates remain).
  - Confirm publish fails loudly when asset deletion/upload is unauthorized.
  - Confirm dev release note body includes nightly build context plus concise highlights from matching `docs/CHANGELOG.md` version section.
- [x] **Dev30 finalization execution**
  - CI validated (runs 219/220/221, commit 2cbb3a2). Smoke test and dependency validation carried forward as issues #3/#4/#5/#18.
- [x] **Dev31 kickoff**
  - Bumped to `v0.1.1-dev31`. Issue tracker populated. AGENTS.md updated.
- [ ] **Forgejo workflow validation**
  - Confirm redundant Windows workflow removal doesn't break release publishing.
  - Confirm test trigger workflow removal doesn't affect CI visibility.
- [ ] **Forgejo workflow validation**
  - Confirm redundant Linux workflow removal doesn't break release publishing.
- [X] **Forgejo dev-packages workflow**
  - Fixed YAML parsing in bundled deps note generation.
- [ ] **Forgejo mirror validation**
  - Confirm built-in push mirror updates Codeberg successfully.
- [ ] **Documentation naming hygiene**
  - Review new docs for personal names; use user report/dev report labels only.
- [X] **Installer dependency parity**
  - Ensure pip is installed on Linux/Windows and skip Go/pip when already present.
- [X] **Windows installer parse fix**
- [X] **Windows GCC PATH verification** - Align MSYS2 GCC detection with PATH checks.
  - Normalize PowerShell here-strings to prevent parser errors.
- [X] **Go auto-install in install.sh**
  - Skip prompting for Go and install automatically when missing.
- [X] **Windows install workflow split**
  - Route Windows to the dedicated installer to avoid mixed-shell prompts.
- [X] **Windows installer entrypoint**
  - Provide a PowerShell entrypoint and direct Windows users to it from install.sh.
- [X] **Windows DVDStyler mirror fallback**
  - Added Leak Technologies mirror URL and EXE fallback for DVD authoring tools.
- [X] **Windows script output alignment**
  - Match Linux-style headers and show build metadata in build.ps1 output.
- [X] **Windows build toolchain repair**
  - Refresh PATH and auto-repair missing MSYS2 GCC during builds when possible.
- [X] **Forgejo dev packaging**
  - Add Forgejo Actions workflow for Windows/Linux dev packages and artifacts.
- [X] **Bundled whisper model**
  - Include the whisper.cpp small model in bundled Windows/Linux packages.
- [X] **Forgejo dev release upload**
  - Upload dev artifacts to a Forgejo release when a token is provided.
- [ ] **Windows signing**
  - Provide production signing cert and wire it into Forgejo secrets for signed releases.
- [ ] **Forgejo runner labels**
  - Ensure runners are online with labels: ubuntu (Linux) and windows.
- [X] **Whisper model mirror fallback**
  - Prefer Leak Technologies mirror for whisper.cpp small model downloads.
- [X] **Git Bash handoff**
  - Keep Windows installs in the same Git Bash terminal using `winpty` when available.
- [X] **Windows root entrypoints**
- [X] **Linux script path fix**
  - Correct Linux build/install/run paths after scripts reorg.
  - Provide `install.bat` and `install.ps1` for PowerShell-first installs.
- [X] **Windows scripts entrypoints**
  - Provide `scripts/install.ps1` and `scripts/install.bat` to avoid Git Bash pop-ups.
- [X] **Windows setup launcher alignment**
  - Route `scripts/_internal/setup-windows.bat` through `scripts/install.bat`.
- [X] **Agent workflow rules**
  - Add `AGENTS.md` to enforce staging, commits, and documentation updates.
- [X] **Player fullscreen toggle**
  - Add fullscreen toggle to the Player module controls.
- [X] **Player EOS handling + metadata access**
  - Stop playback cleanly on EOS and expose duration/FPS from GStreamer.
- [X] **Main menu title cleanup**
  - Header shows "VideoTools" only; platform suffix moved to footer version label.
- [X] **Main menu palette refresh**
  - Restore a diverse, eye-friendly rainbow palette while keeping Convert constant.
- [X] **Main menu readability**
  - Increase tile label size and adjust low-contrast colors.
- [X] **Main menu contrast tuning**
  - Refine Audio, Rip, and Settings colors for legibility.
- [X] **Main menu layout cleanup**
  - Remove the scroll container so the main menu scales without scroll bars.
- [X] **Player silhouette placeholder**
  - Keep a stable player footprint before media loads.
- [X] **Main menu palette tuning**
  - Adjust audio/compare/subtitles colors for clearer separation.
- [X] **Main menu vibrancy pass**
  - Remove monochrome tiles outside Settings.
- [X] **Main menu bespoke hues**
  - Assign unique hue families to each module for maximum legibility.
- [X] **Locked tile hue preservation**
  - Keep disabled modules colored while subdued.
- [X] **Locked hue visibility**
  - Reduce stripe opacity and raise label brightness.
- [ ] **Main package layout cleanup**
  - Move root `package main` files into `cmd/videotools` when the build is stable.
- [ ] **Windows packaging prep**
  - [x] Draft MSIX + WinGet layout under `packaging/windows/`.
  - [x] Add GitHub Actions workflow to build MSIX and upload release artifacts.
  - [ ] Wire signing step (SignTool) once certificate is available.
  - [ ] Keep Windows installer aligned with MSIX/WinGet production path.

- [ ] **Git converter integration**
  - Move git_converter workflow into the main VT UI and retire legacy scripts.
- [ ] **Windows installer validation**
  - Test `scripts\_internal\install-deps-windows.ps1` MSYS2 flow and GStreamer MSI install on Windows 10/11.
  - Re-test GStreamer MSI download and local MSI override after variable fix.
  - Confirm mirror fallback works when the primary download returns HTML.
  - Verify winget fallback works when MSI downloads fail.
  - Confirm winget-first flow succeeds on clean Windows VM.
  - Verify DVDStyler winget fallback sets dvdauthor/mkisofs on PATH.
  - Validate MSI-first flow installs GStreamer and DVDStyler without winget.
  - Validate that GStreamer devel MSI is skipped unless build tools are selected.
  - Verify Whisper model prompt/download and mirror override on Windows.

## Documentation: Fix Structural Errors

**Priority:** High

- [X] **Audit All Docs for Broken Links:**
  - Systematically check all 46 `.md` files for internal links that point to non-existent files or sections.
  - Create placeholder stubs for missing documents that are essential (e.g., `CONTRIBUTING.md`) or remove the links if they are not.
  - This ensures a professional and navigable documentation experience.

## Critical Priority: dev24

### AUTHOR MODULE: CONTENT TYPES + GALLERIES + CHAPTER THUMBS

- [ ] **Content classification (Feature/Extra/Gallery)**
  - Feature: supports chapters + chapter menus
  - Extra: separate DVD titles; no chapters
  - Gallery: still-image slideshow title under Extras
  - Extras require subtype (behind_the_scenes, deleted_scenes, featurettes, interviews, trailers, commentary, other)
- [ ] **Cross-platform DVD authoring parity**
  - Ensure Windows and Linux use the same dvdauthor XML + ISO tool flags
  - Treat DVDStyler as a CLI tool bundle only (no GUI authoring dependency)
- [ ] **Chapter screenshot generation (Feature only)**
  - Auto-generate one still per chapter (default 2s offset)
  - Fallback to first valid frame on failure
  - Allow per-chapter override image
- [ ] **Menu structure rules**
  - Main: Play Feature, Chapters (if any), Extras (if extras/galleries)
  - Extras menu groups by subtype; galleries listed separately
- [ ] **UI layout guardrails**
  - Separate Feature / Extras / Galleries sections
  - Chapters disabled when content type is not Feature
- [ ] **Schema + config updates**
  - Add content_type per video, gallery assets list, chapter thumb config
  - Persist extras subtype and gallery behavior (auto-advance, loop)

## AUTHOR MODULE: MENU TEMPLATES & THEMES

**Template** = Layout/structure (how buttons are arranged)
**Theme** = Visual aesthetic (colors, feel - sci-fi, western, minimal, etc.)

### Template Layouts (Menu Structure)

- [x] **Minimal** - Black background, white text, vertical button list (cleanest style)
- [x] **Classic** - Title at top, buttons below centered (traditional DVD)
- [x] **Grid** - 2x2 or 3x2 button matrix (Star Wars style)
- [x] **Filmstrip** - Thumbnail preview buttons for scene selection
- [x] **Poster** - Large background image with overlay (current implementation)
- [ ] **Cinematic** - Wide banner title, full-bleed background (modern)

### Preset Themes (Visual Aesthetic)

- [x] **VideoTools** - Background: #0F172A, Text: #E1EEFF, Accent: #7C3AED (sci-fi/purple)
- [x] **Minimal** - Background: #000000, Text: #FFFFFF, Accent: #AAAAAA (clean B&W)
- [x] **Western** - Background: #1A1408, Text: #F5DEB3, Accent: #8B4513 (warm browns/golds)
- [x] **Film Noir** - Background: #1A1A1A, Text: #E0E0E0, Accent: #808080 (dark gray)
- [ ] **Classic Hollywood** - Background: #000000, Text: #F5F5DC, Accent: #D4AF37 (gold)
- [ ] **Warm Cinema** - Background: #1A0F0A, Text: #FFF5E6, Accent: #E67E22 (orange)
- [ ] **Ocean** - Background: #0A1A2A, Text: #E0F0FF, Accent: #00CED1 (cyan)
- [ ] **Nature** - Background: #0A1A0A, Text: #E6FFE6, Accent: #DAA520 (green-gold)
- [ ] **Custom** - User-defined colors via color pickers

### Menu Types (What Gets Generated)

- [ ] **Main Menu** - Primary entry point with Play, Scene Selection, Special Features, Setup
- [ ] **Scene Selection** - Chapter thumbnails with preview images
- [ ] **Special Features** - Extras/trailers submenu
- [ ] **Setup/Languages** - Audio/subtitle language selection
- [ ] **Movie Title** - Opening video before menu (pre-menu)

### Customization Options (Core Features)

- [x] Background image selection per template (currently works for Poster template)
- [x] **Custom background for ALL templates** - Allow user to provide their own background image (PNG/JPG) for any template
- [x] **Motion backgrounds** - Support looping video backgrounds (MPG) - audio is embedded in the video file
- [x] Logo overlay (title/studio) with position/scale/margin controls
- [ ] Custom font support (TTF/OTF)
- [ ] Button style/shape options

### Professional Features (Archivist Focus)

- [x] **Chapter thumbnail generation** - Auto-capture thumbnails from video for scene selection
- [ ] **Menu preset save/load** - Save theme+template combinations as reusable presets
- [ ] Multiple audio track support - Multiple language audio streams
- [ ] **Subtitle selection** - Include subtitle streams in DVD
- [ ] **Disc label/cover** - Generate printable disc label images
- [ ] **Widescreen (16:9) support** - Proper 16:9 DVD authoring
- [ ] **DVD-9 (dual-layer) support** - Handle discs larger than 4.7GB

### Dependencies & Cross-Platform

- [X] **WSL ISO creation on Windows** - Use WSL xorriso for consistent DVD ISO generation
- [ ] **ISO creation verification** - Test that Windows WSL path conversion works correctly
- [ ] **Error handling** - Clear messages when WSL not installed or tools missing

### Implementation Priority

**Phase 1: Core Menu System**
1. Separate Template and Theme dropdowns in UI
2. Add themes: VideoTools, Minimal, Western, Film Noir, Classic Hollywood
3. Minimal template implementation (the clean black bg + white text style)
4. Custom background image support for all templates
5. Classic template

**Phase 2: Visual Polish**
6. Custom theme color pickers (Custom theme)
7. Animated background support (motion menus)
8. Background audio support
9. Grid template
10. Filmstrip template

**Phase 3: Professional Features**
11. Chapter thumbnail generation for scene selection
12. Menu preset save/load
13. Multiple audio/subtitle tracks
14. Disc label generation
15. DVD-9 dual-layer support

### VIDEO PLAYER IMPLEMENTATION

**CRITICAL BLOCKER:** All advanced features (enhancement, trim, advanced filters) depend on stable player foundation.

#### Current Player Issues (from docs/PLAYER_PERFORMANCE_ISSUES.md):

1. **Separate A/V Processes** (lines 10184-10185 in main.go)
   - Video and audio run in completely separate FFmpeg processes
   - No synchronization mechanism between them
   - They will inevitably drift apart, causing A/V desync and stuttering
   - **FIX:** Implement unified FFmpeg process with multiplexed output

2. **Audio Buffer Too Small** (lines 8960, 9274 in main.go)
   - Currently 8192 samples = 170ms buffer
   - Modern systems need 100-200ms buffers for smooth playback
   - **FIX:** Increase to 16384-32768 samples (340-680ms)

3. **Volume Processing in Hot Path** (lines 9294-9318 in main.go)
   - Processes volume on EVERY audio sample in real-time
   - CPU-intensive and blocks audio read loop
   - **FIX:** Move volume processing to FFmpeg filters

4. **Video Frame Pacing Issues** (lines 9200-9203 in main.go)
   - time.Sleep() is not precise, cumulative timing errors
   - No correction mechanism if we fall behind
   - **FIX:** Implement adaptive timing with drift correction

5. **UI Thread Blocking** (lines 9207-9215 in main.go)
   - Frame updates queue up if UI thread is busy
   - No frame dropping mechanism
   - **FIX:** Implement proper frame buffer management

6. **No Frame-Accurate Seeking** (lines 10018-10028 in main.go)
   - Seeking kills and restarts both FFmpeg processes
   - 100-500ms gap during seek operations
   - No keyframe awareness
   - **FIX:** Implement frame-level seeking without process restart

#### Player Implementation Plan:

**Phase 1: Foundation (Week 1-2)**
- [ ] **Unified FFmpeg Architecture**
  - Single process with multiplexed A/V output using pipes
  - Master clock reference for synchronization
  - PTS-based drift correction mechanisms
  - Ring buffers for audio and video

- [ ] **Hardware Acceleration Integration**
  - Auto-detect available backends (CUDA, VA-API, VideoToolbox)
  - FFmpeg hardware acceleration through native flags
  - Fallback to software acceleration when hardware unavailable

- [ ] **Frame Extraction System**
  - Frame extraction without restarting playback
  - Keyframe detection and indexing
  - Frame buffer pooling to reduce GC pressure

**Phase 2: Core Features (Week 3-4)**
- [ ] **Frame-Accurate Seeking**
  - Seek to specific frames without restarts
  - Keyframe-aware seeking for performance
  - Frame extraction at seek points for preview

- [ ] **Chapter System Integration**
  - Port scene detection from Author module
  - Manual chapter support with keyframing
  - Chapter navigation (next/previous)
  - Chapter display in UI

- [ ] **Performance Optimization**
  - Adaptive frame timing with drift correction
  - Frame dropping when UI thread can't keep up
  - Memory pool management for frame buffers
  - CPU usage optimization

**Phase 3: Advanced Features (Week 5-6)**
- [ ] **Preview System**
  - Real-time frame extraction
  - Thumbnail generation from keyframes
  - Frame buffer caching for previews

- [ ] **Error Recovery**
  - Graceful failure handling
  - Resume capability after crashes
  - Smart fallback mechanisms

### ENHANCEMENT MODULE FOUNDATION

**DEPENDS ON PLAYER COMPLETION**

#### Current State:
- [X] Basic filters module with color correction, sharpening, transforms
- [X] Stylistic effects (8mm, 16mm, B&W Film, Silent Film, VHS, Webcam)
- [X] AI upscaling with Real-ESRGAN integration
- [X] Basic AI model management
- [ ] No content-aware processing
- [ ] No multi-pass enhancement pipeline
- [ ] No before/after preview system

#### Enhancement Module Plan:

**Phase 1: Architecture (Week 1-2 - POST PLAYER)**
- [ ] **Model Registry System**
  - Abstract AI model interface for easy extension
  - Dynamic model discovery and registration
  - Model requirements validation
  - Configuration management for different model types

- [ ] **Content Detection Pipeline**
  - Automatic content type detection (general/anime/film)
  - Quality assessment algorithms
  - Progressive vs interlaced detection
  - Artifact analysis (compression noise, film grain)

- [ ] **Unified Enhancement Workflow**
  - Combine Filters + Upscale into single module
  - Content-aware model selection logic
  - Multi-pass processing framework
  - Quality preservation controls

**Phase 2: Model Integration (Week 3-4)**
- [ ] **Open-Source AI Model Expansion**
  - BasicVSR integration (video-specific super-resolution)
  - RIFE models for frame interpolation
  - Real-CUGan for anime/cartoon enhancement
  - Model selection based on content type

- [ ] **Advanced Processing Features**
  - Sequential model application capabilities
  - Custom enhancement pipeline creation
  - Parameter fine-tuning for different models
  - Quality vs Speed presets

### TRIM MODULE ENHANCEMENT

**DEPENDS ON PLAYER COMPLETION**

#### Current State:
- [X] Basic planning completed
- [ ] No timeline interface
- [ ] No frame-accurate cutting
- [ ] No chapter integration from Author module

#### Trim Module Plan:

**Phase 1: Foundation (Week 1-2 - POST PLAYER)**
- [ ] **Timeline Interface**
  - Frame-accurate timeline visualization
  - Zoom capabilities for precise editing
  - Scrubbing with real-time preview
  - Time/frame dual display modes

- [ ] **Chapter Integration**
  - Import scene detection from Author module
  - Manual chapter marker creation
  - Chapter navigation controls
  - Visual chapter markers on timeline

- [ ] **Frame-Accurate Cutting**
  - Exact frame selection for in/out points
  - Preview before/after trim points
  - Multiple segment trimming support

**Phase 2: Advanced Features (Week 3-4)**
- [ ] **Smart Export System**
  - Lossless vs re-encode decision logic
  - Format preservation when possible
  - Quality-aware encoding settings
  - Batch trimming operations

### DOCUMENTATION UPDATES

- [X] **Create PLAYER_MODULE.md** - Comprehensive player architecture documentation
- [X] **Update MODULES.md** - Player and enhancement integration details
- [X] **Update ROADMAP.md** - Player-first development strategy
- [ ] **Create enhancement integration guide** - How modules work together
- [ ] **API documentation** - Player interface for module developers

## Future Enhancements (dev24+)

### AI Model Expansion
- [ ] **Diffusion-based models** - SeedVR2, SVFR integration
- [ ] **Advanced restoration** - Scratch repair, dust removal, color fading
- [ ] **Face enhancement** - GFPGAN integration for portrait content
- [ ] **Specialized models** - Content-specific models (sports, archival, etc.)

### Professional Features
- [ ] **Batch enhancement queue** - Process multiple videos with enhancement pipeline
- [ ] **Hardware optimization** - Multi-GPU support, memory management
- [ ] **Export system** - Professional format support (ProRes, DNxHD, etc.)
- [ ] **Plugin architecture** - Extensible system for community contributions

### Integration Improvements
- [ ] **Module communication** - Seamless data flow between modules
- [ ] **Unified settings** - Shared configuration across modules
- [ ] **Performance monitoring** - Resource usage tracking and optimization
- [ ] **Cross-platform testing** - Linux and Windows parity

## Technical Debt Addressed

### Player Architecture
- [X] Identified root causes of instability
- [X] Planned Go-based unified solution
- [X] Hardware acceleration strategy defined
- [X] Frame-accurate seeking approach designed

### Enhancement Strategy
- [X] Open-source model ecosystem researched
- [X] Scalable architecture designed
- [X] Content-aware processing planned
- [X] Future-proof model integration system

## Notes

- **Player stability is BLOCKER**: Cannot proceed with enhancement features until player is stable
- **Go implementation preferred**: Maintains single codebase, excellent testing ecosystem
- **Open-source focus**: No commercial dependencies, community-driven model ecosystem
- **Modular design**: Each enhancement system can be developed and tested independently

