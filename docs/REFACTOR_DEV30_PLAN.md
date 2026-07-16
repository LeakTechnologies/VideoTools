# Refactor Plan (Current — v0.1.1-dev53)

Goal: shrink root `package main` from ~16k lines toward a thin wiring layer. All logic moves into `internal/` packages. Each step is a small, build-verified commit.

## Current State

### Already extracted to `internal/app/modules/` (view layer only)

| Module | Files | Status |
|---|---|---|
| settings | `view.go`, `types.go`, `tabs.go` | View + callbacks wired |
| queue | `view.go`, `types.go` | View + callbacks wired |
| rip | `view.go`, `types.go`, `scan.go`, `launch.go`, `executor.go`, `iso_udf.go`, `ifo_regen.go` | Most complete extraction |
| filters | `view.go`, `chain.go` | View layer |
| upscale | `view.go`, `types.go`, `helpers.go` | View layer |
| compare | `view.go`, `fullscreen_native.go`, `fullscreen_stub.go` | View layer |
| inspect | `view.go`, `types.go` | View layer |
| trim | `view.go`, `stub.go` | View layer |
| audio | `view.go`, `executor.go` | View layer |
| subtitles | `view.go`, `types.go` | View layer |
| thumbnail | `view.go` | View layer |
| player | `view.go` | View layer |
| about | `dialog.go` | Complete |
| deps | `dialog.go` | Complete |
| mainmenu | `helpers.go` | Partial |

### Root `*_module.go` files (package main — not yet moved)

| File | Lines | Content |
|---|---|---|
| `author_module.go` | 4545 | DVD authoring UI + logic |
| `settings_module.go` | 1875 | Settings callbacks + update checker |
| `subtitles_module.go` | 1348 | Subtitle editing UI |
| `audio_module.go` | 500 | Audio extraction UI |
| `thumbnail_module.go` | 494 | Thumbnail generation UI |
| `queue_module.go` | 434 | Queue management UI |
| `filters_module.go` | 411 | Filters callbacks |
| `player_module.go` | 370 | Player singleton wiring |
| `mainmenu_module.go` | 360 | Main menu layout |
| `inspect_module.go` | 354 | Inspect callbacks |
| `file_manager_module.go` | 334 | File manager UI |
| `upscale_module.go` | 319 | Upscale callbacks |
| `rip_module.go` | 269 | Rip callbacks |
| `burn_module.go` | 242 | Disc burning UI |
| `compare_module.go` | 102 | Compare callbacks |
| `locale_module.go` | 62 | Locale helpers |
| `about_module.go` | 38 | About dialog |
| `deps_dialog_module.go` | 17 | Deps dialog |

### Major blocks still in `main.go` (~15,900 lines)

| Block | Lines | Description |
|---|---|---|
| `buildConvertView` | 4328 | Convert view builder — **largest single function** |
| `executeUpscaleJob` | 1048 | Upscale job executor |
| `executeConvertJob` | 937 | Convert job executor |
| `startConvert` | 864 | Convert orchestration |
| `handleDrop` | 579 | Drag-and-drop handler |
| `showMergeView` | 508 | Merge view builder |
| `buildFFmpegCommandFromJob` | 431 | FFmpeg command builder |
| `buildVideoPane` | 429 | Video player pane builder |
| `buildMetadataPanel` | 408 | Metadata panel builder |
| `executeSnippetJob` | 426 | Snippet job executor |
| `executeMergeJob` | 356 | Merge job executor |
| `handleModuleDrop` | 314 | Module-specific drop routing |
| `runGUI` | 306 | App startup |
| `probeVideo` | 287 | Video probing/metadata |
| `generateSnippet` | 244 | Snippet generation |
| Media layer | ~800 | `loadVideo`, `loadMultipleVideos`, `loadVideos`, `clearVideo`, etc. |
| Convert helpers | ~600 | `buildFFmpegCommandFromJob`, codec detection, aspect helpers |
| History | ~500 | `showHistoryDetails`, `deleteHistoryEntry`, etc. |
| Benchmark | ~400 | `runNewBenchmark`, `showBenchmarkHistory`, etc. |

## Phase 3 — Extract from root to `internal/` (current)

### Strategy: Extract-Verify-Commit per slice

Each extraction follows the same pattern:
1. Identify a cohesive block (one function or a small group)
2. Create new `<name>.go` in the appropriate `internal/` package
3. Move code verbatim — no behavior changes
4. Add a thin wiring shim in root if needed (temporary)
5. `go build` must pass
6. Commit as one atomic slice

**Never mix structural moves with feature work.**

### Priority order (by impact on main.go size)

#### P0 — Convert module (5,100+ lines removable)

The convert module is the single largest block. It has three layers:

1. **View builder** (`buildConvertView`, 4328 lines) — extract to `internal/app/modules/convert/view.go`
   - Already has a placeholder at `internal/app/modules/convert/view.go` (165 lines)
   - The real function needs appState decoupling first
   - Split into sub-builders: `buildEncodingPanel`, `buildOutputPanel`, `buildFormatPanel`

2. **Job executors** (`executeConvertJob`, `startConvert`, `buildFFmpegCommandFromJob` — 2,232 lines)
   - Extract to `internal/app/modules/convert/executor.go`
   - Pure logic, no UI dependencies — cleanest extraction target

3. **Queue helpers** (`addConvertToQueue*`, `addAllConvertToQueue` — ~400 lines)
   - Extract to `internal/app/modules/convert/queue.go`

#### P1 — Media layer (2,000+ lines)

The media/player layer is tightly coupled to `appState`:

- `probeVideo` (287) → `internal/media/probe/` or `internal/mediaprobe/`
- `loadVideo`, `loadMultipleVideos`, `loadVideos`, `clearVideo`, `switchToVideo` (~600) → `internal/app/modules/player/media.go`
- `handleDrop`, `handleModuleDrop` (~900) → `internal/app/modules/player/drop.go`
- `buildVideoPane`, `buildMetadataPanel` (~840) → already partially in `internal/app/modules/player/view.go`

#### P2 — Merge module (860 lines)

- `showMergeView` (508) → `internal/app/modules/merge/view.go`
- `executeMergeJob` (356) → `internal/app/modules/merge/executor.go`
- `addMergeToQueue` → `internal/app/modules/merge/queue.go`

#### P3 — Job executors (2,400 lines)

- `executeUpscaleJob` (1048) → `internal/app/modules/upscale/executor.go`
- `executeSnippetJob` (426) → `internal/app/modules/thumbnail/executor.go`
- `executeTrimJob` (161) → already in `trim` package as stub

#### P4 — History + Benchmark (900 lines)

- History functions (~500) → `internal/app/modules/settings/history.go`
- Benchmark functions (~400) → `internal/app/modules/benchmark/`

#### P5 — Root module file retirement

Once logic is in `internal/`, the root `*_module.go` files become thin wiring shims (import + call). Retire them by moving the wiring into the module packages themselves.

### Guardrails

- **Small commits only** — each move is one function or one cohesive group
- **No behavior changes** mixed into structural refactors
- **Build must pass** after every commit — `go build -tags=native_media`
- **Stubs for build-tag splits** stay in root only if `internal/` can't hold them
- **`cmd/videotools/`** entrypoint extraction is Phase 4 — not until Phase 3 is complete

## Phase 4 — Entrypoint (future)

Move `main()` and `runGUI()` to `cmd/videotools/main.go`. Root `package main` becomes zero lines.

## Verification

After each extraction:
1. `go build -tags=native_media ./...` — must pass
2. `go vet ./...` — no new warnings
3. Manual smoke test — load a video, play, convert
4. CI green on both platforms

## Token Budget

Each extraction loop should be:
- One function/group per commit
- Build verification = stop condition (no multi-agent review)
- Human-triggered, not scheduled
- Estimated: 2-5k tokens per extraction cycle
