# Region-Conversion Audit Brief

**Audience:** a fresh agent with zero prior context on this repository.
**Task:** audit every place the codebase decides, converts, or assumes a DVD
**video standard** (PAL/NTSC/SECAM) and every place a **stream copy** (`-c copy`)
interacts with those decisions. Produce a severity-ranked findings report.
**Critically:** this is a *research + decision-support* task. Do not change
production code — the deliverable is the report (see "Deliverables" below).
Fix prototypes are allowed in a scratch branch only.

> Read `docs/PAL_NTSC_CONVERSION.md` first — it documents the *intended* model.
> Several of the suspected issues below are contradictions between that doc and
> what the code actually does.

---

## 1. What "region" means in this codebase

Two DIFFERENT concepts share the word. Do not confuse them.

| Term | Meaning | Where |
|---|---|---|
| **`RegionConvert`** | DVD **video standard** conversion: `"pal2ntsc"` / `"ntsc2pal"` / `""` | rip executor args, view state |
| **`Region`** (scan result) | Publication **region mask** parsed from `VMG_Category` in `VIDEO_TS.IFO` (e.g. "Region 1", "Region Free") — metadata only, never drives conversion | `DiscScanResult.Region` |
| **`VideoStandard`** (scan result) | `"NTSC"` or `"PAL"` derived from the first title's VTS IFO PGC header frame-rate bits | `DiscScanResult.VideoStandard` |

This audit is about **`RegionConvert` / `VideoStandard`** (video standards).
The publication-region mask matters only for consistency checks (the Convert
module authors **region-free** output; converted full-disc IFOs should agree —
see `docs/DVD_IFO_TROUBLESHOOTING.md:119`).

Key technical constants (NTSC/PAL):

| Standard | Frames/s | Dimensions | Dominant PAR |
|---|---|---|---|
| NTSC | 30000/1001 ≈ 29.97 | 720×480 | 4:3 → 10/11; 16:9 → 32/27 |
| PAL | 25 | 720×576 | 4:3 → 12/11; 16:9 → 16/11 |
| SECAM | 25 (DVD = PAL-identical) | 720×576 | as PAL |

Film is commonly 23.976 fps; PAL masters run film at 25 (a **4.167 % speed-up**);
NTSC presents film at 23.976 via 3:2 pulldown to 29.97.

---

## 2. Project orientation (30 seconds)

- Go + Fyne desktop GUI. Go 1.26. **Linux + Windows only** (no macOS — delete
  `darwin` paths on sight).
- Rip module lives in `internal/app/modules/rip/` (`view.go` = UI,
  `executor.go` = ffmpeg orchestration, `scan.go` = disc scan, `ifo_regen.go` =
  IFO/BUP regeneration, `types.go` = shared types — **do not gofmt
  `types.go`**).
- Convert module presets: `internal/convert/`. Author module (DVD authoring):
  `author_*.go` in the repo root.
- History: `DONE.md`, `docs/CHANGELOG.md`, `docs/roadmap.html` (canonical
  tracker). Current cycle `v0.1.1-dev72`.
- Local build gate: `powershell -ExecutionPolicy Bypass -File
  scripts/windows/dev-verify.ps1` (builds `-tags=native_media ./...` + `go vet
  ./...` and sets the CGo env CI uses). Bare `go build` fails at the CGo gate
  on Windows without it. Never gofmt `internal/i18n/*.go` or
  `internal/app/modules/rip/types.go`.
- Already-shipped history that touches this audit: **dev47** PAL↔NTSC full-disc
  conversion pipeline (IFO regeneration), **dev56/dev57** NTSC/PAL detection
  (detection moved *after* the VTS-cache loop in dev57 — before that it never
  populated), **dev64** per-language subtitle selection + modes (region forces
  full-disc), **dev68** liar-IFO subtitle clamp, **dev69/dev71** VOB-concat
  stale-PTS caps.

## 3. The feature surfaces

### 3a. Scan (detection)

`internal/app/modules/rip/scan.go`
- `classifyDiscRegion(category uint32)` at `:40` — publication-region mask from
  `VMG_Category` (`ifo.ReadVMGI`, VMG_MAT read at `:90-100`; stored at `:128`
  and `:314`).
- **VideoStandard** detection at `:176-192` — derived from **the first title's
  VTS only** (`tsps[0]`), from `TitleInfo.IsNTSC` (PGC header frame-rate bits,
  set in `internal/dvd/ifo/extract.go`). Type field `types.go:105` ("NTSC",
  "PAL", or "").

### 3b. Per-title rip — `BuildRipArgs`

`internal/app/modules/rip/executor.go:219-419`
- `RegionConvert` field: `:302`. Applied to video **only when `isH264`**
  (MKV/MP4) — `:360-371`:
  - `pal2ntsc` → `-vf "yadif=mode=1,scale=720:480:flags=lanczos,fps=30000/1001"`
  - `ntsc2pal` → `-vf "yadif=mode=1,scale=720:576:flags=lanczos,fps=25"`
  - interlace-only (no region convert) → `-vf "yadif=mode=1"`
- Audio atempo — `:381-386` (MKV) and `:395-400` (MP4):
  - `pal2ntsc` → `-af "atempo=0.9600"` (0.96 = 24/25)
  - `ntsc2pal` → `-af "atempo=1.0417"` (1.0417 = 25/24)
- **Default (Lossless MKV) branch is `-c copy`** — `:402`. Region-conversion
  filters do NOT apply there (they're H.264-only).
- MKV audio is `-c:a copy` (`:379`); MP4 re-encodes `aac@192k` (`:392`).
- Subtitle mapping `-map 0:s:<idx>` from `SubtitleSel` (`:339-346`).

### 3c. Full-disc extraction

`internal/app/modules/rip/executor.go`
- `executeFullDiscExtraction` (`:1000-1258`) — iterates every VTS set + menus
  (`CollectMenuVOB` `:1102`), builds concat lists, probes durations, converts
  each set, then `RegenerateIFOs` (`:1250`).
- `convertVOBWithRegion` (`:1262-1318`) — fair-use note: this is the function
  name, not third-party code.
  - **Always re-encodes video/audio** even with *no* region conversion:
    `-c:v mpeg2video -q:v 5` and `-c:a ac3 -b:a 192k` are unconditional
    (`:1292-1297`). With `vfFilter == ""` it *still* re-encodes — full-disc
    extraction is always lossy today.
  - Subtitles are **dropped when `vfFilter != ""`** (`:1274-1281`): `-map 0:s?`
    and `-c:s copy` are only added in the no-conversion branch.
- Filters chosen at `:1107-1140`; atempo `0.9600`/`1.0417` again.

### 3d. IFO regeneration

`internal/app/modules/rip/ifo_regen.go:25-76` — `RegenerateIFOs(..., isNTSC,
regionConvert, ...)`; `buildVTSMat(..., regionConvert != "")`.
- On conversion, `VTS_Subpicture_Count` is zeroed (`:209-214`) so players don't
  load subtitles that PTS-wise can't match the new frame rate.
- Sets `isNTSC` / PAL video attributes, PGC timing, TMAPT sector maps.

### 3e. UI gating

`internal/app/modules/rip/view.go`
- Formats at `:222`; defaults `vs.format = FormatLosslessMKV` at `:141`.
- Region dropdown (`RipRegionNone` / `RipRegionPALtoNTSC` / `RipRegionNTSCtoPAL`)
  at `:795-818`. Selecting a conversion **forces `extractMode = "full"`** and
  hides the mode radio (`:804-807`).
- Mode radio forces full-disc too (`:738-754`). Full-disc check `:762-776`.
- Output naming for full/region runs at `:194-201`.

### 3f. Concat fallback (dvdvideo → VOB concat)

`internal/app/modules/rip/executor.go:570-817`
- `-f dvdvideo` is preferred (`:570-587`); on failure it retries via concat
  (`probeSubtitleCount` clamp `:761-768`, stale-PTS `-t` cap `:770-811`).
- Known: concat + `-c copy` can write PTS discontinuities at VOB boundaries
  (`:747-749`); the cap defends the >=26hr phantom-tail case (`:770-775`).

### 3g. Secondary surfaces (scope-tier 2 — report only, don't fix)

- **Convert module DVD presets** — `internal/convert/dvd_regions.go`
  (`DVDStandard` table; `DVDNTSCRegionFree` / `DVDPALRegionFree` /
  `DVDSECAMRegionFree`) and `internal/convert/dvd.go`
  (`DVDNTSCPreset`, `ValidateDVDNTSC`, `BuildDVDFFmpegArgs`). Both targets are
  region-free. Presets alone do not change the frame rate unless the source
  differs; validation warns.
- **Author module** — `author_module.go` region select (`AUTO/NTSC/PAL`,
  `:798-828`) + `resolveAuthorRegion` (`:2635-2654`), `author_menu.go`
  `dvdMenuDimensions` (`:872-881`), region mask in `internal/dvd/region/`.

---

## 4. Audit questions (ranked; each needs an answer)

> For every question: **state the fact you verified**, the **code evidence**,
> and the **recommended action**. Line numbers are as of commit `c0e21121`
> (v0.1.1-dev72).

### Q1 — Audio/video sync: do the atempo ratios match the actual video model?

**Suspect: yes → real desync.** `docs/PAL_NTSC_CONVERSION.md:3-11` justifies
`atempo=0.9600` with the *film-cadence* model: PAL content is 24fps film sped to
25; "back to NTSC" means 23.976 material shown via 3:2 pulldown, so audio slows
by 24/25. But the implementation does **not** produce 23.976+pulldown — it runs
`fps=30000/1001` directly, which is a duration-preserving frame-rate retimer
(duplicate/drop frames), i.e. the video plays at real 29.97 with unchanged
duration. Audio at 0.96 takes **1/0.96 = 4.17 % longer** than the video.

- Verify mathematically: `fps=30000/1001` on a `25fps` input does *not* change
  wall-clock duration (last-frame timestamp ≈ input duration). Confirm with a
  synthetic clip: `ffmpeg -f lavfi -i testsrc=duration=60:size=720x576:rate=25
  -f lavfi -i sine=frequency=440:duration=60 -c:v mpeg2video -c:a ac3 -v
  error -f dvd out.mpg` then region-convert a *short* slice and compare
  `ffprobe -show_entries format=duration` of audio vs video and watch A/V drift
  over 10-20 s (`-af asetpts` analysis or a vlc/sync smoke test).
- Determine which model is intended. If duration-preserving (most likely — the
  whole pipeline preserves duration): atempo should be **1.0** (drop the filter)
  or the filter chain must implement true 23.976 + pulldown flags. If the film
  model is intended: the video chain must change, not just the audio.
- Same inverse check for `ntsc2pal` (1.0417 vs the 0.834... factor of a true
  29.97→25 duration change).
- Mirror finding into full-disc path (`executor.go:1132-1140`).

### Q2 — Full-disc without conversion is always a lossy re-encode

`convertVOBWithRegion` muxes `-c:v mpeg2video -q:v 5` + `-c:a ac3 -b:a 192k`
**unconditionally** (`executor.go:1292-1297`). "Full disc extraction (no region
conversion)" therefore re-encodes already-MPEG-2 DVDs lossily instead of
copying. Source VOBs are MPEG-2 + AC-3 — a straight `-c copy` is DVD-compliant
and lossless.
- Verify by diffing one converted VTS set vs source (bitrate, size, PSNR sample
  or `-vstats`).
- Recommended: `-c:v copy -c:a copy` when `vfFilter == ""`, keep the
  re-encode only under conversion. Check `RegenerateIFOs` still matches
  (MPEG-2 attributes unchanged under copy).
- This is the strongest candidate for a real P1/P2 quality bug.

### Q3 — Is the per-title (H.264) region path reachable at all?

`BuildRipArgs` region branches are gated on `isH264` (`:360-371`), but the UI
forces `extractMode = "full"` the moment a conversion is picked
(`view.go:804-807`, `:738-754`) — the per-title path runs only when
`extractMode != "full"`. Trace `executeRipJob` / `launch.go` to confirm whether
`opts.RegionConvert` can ever reach `BuildRipArgs` with a non-full mode.
- If unreachable: the H.264 region branches + their atempo are **dead code**
  (defensive only) — decide: keep as guard rails or strip for clarity. Also
  confirm the lossless/archivist formats genuinely hide the controls (per
  `docs/PAL_NTSC_CONVERSION.md:34-37`) so a user can't silently pick a region
  conversion on a `-c copy` format and have it ignored.

### Q4 — SAR / aspect ratio through `scale=...`

`docs/PAL_NTSC_CONVERSION.md:50-51` claims "SAR metadata carries the original
DAR so 4:3/16:9 is not distorted". FFmpeg's `scale` filter does **not**
automatically correct SAR; with `scale=720:576→720:480` keeping the source SAR
the output DAR changes.
- Verify on synthetic 16:9 PAL and NTSC sources (SAR 16/11 and 32/27): encode
  → region-convert → `ffprobe -show_entries stream=sample_aspect_ratio,display_aspect_ratio`.
- If distorted: add `setsar=` after scale (e.g. PAL→NTSC needs SAR 10/11 for
  4:3, 32/27 for 16:9) or derive from source DAR.
- Applies to both `BuildRipArgs` and `convertVOBWithRegion` chains.

### Q5 — Subtitle correctness across every branch

- Per-title no-conversion: `-map 0:s<idx>` + `-c:s copy` (or mux) — VOBSUB PTS
  valid, fine.
- Per-title conversion (if reachable, Q3): subtitles are mapped (`:339-346`)
  but is there any remap? VOBSUB PTS are baked to source frame rate — check
  whether converted rips actually include syncable subs or should drop them
  like full-disc does (`:1274-1281`).
- Full-disc conversion: subs dropped + `VTS_Subpicture_Count = 0`
  (`ifo_regen.go:209-214`) — verify no other IFO field still references
  subtitle streams (e.g. VMG/VTSM subpicture attributes).
- Interlaced-only (`yadif` with no fps change): no atempo, subs kept via
  `-c:s copy` — PTS unchanged so correct. Confirm.
- MP4 language metadata is skipped (`:411-417` vs `:412`) — assert intended.

### Q6 — `-c copy` robustness in the concat fallback

The dev69/71 `-t` cap stops stale-PTS phantom tails (`:770-811`), but:
- Multi-VOB boundary PTS discontinuities *within* the real title (`:747-749`) —
  does `+genpts` (`:1267`, and the dvdvideo-preference comment `:570-573`)
  actually guarantee continuous timestamps for copy muxing, or should the
  fallback add a normalise/`setpts` pass?
- Do chapters survive the fallback (they come from `-map_chapters metaInputIdx`
  — when metaInput is the concat list, are chapters empty)?

### Q7 — Detection assumptions

- `VideoStandard` samples only the first title's VTS (`scan.go:176-192`). On a
  mixed-standard disc (rare) it is wrong. Flag for report; recommend sampling
  every title or the longest title and consolidating.
- `classifyDiscRegion` vs `VideoStandard` can disagree (region mask is
  publication, standard is technical) — nothing should conflate them; audit any
  call site that does.

### Q8 — Full-disc conversion of menus & hard problems

`docs/PAL_NTSC_CONVERSION.md:114-123` lists known hard problems (button CLUT
addresses after cell re-layout, multi-angle, seamless branching addressing,
forced subtitles). Verify each is either handled or explicitly logged as
unsupported when a conversion rip runs — a silently mis-muxed menu is a P1 user
facing bug; a clearly-logged limitation is acceptable.

---

## 5. Verification tooling

- `powershell -ExecutionPolicy Bypass -File scripts/windows/dev-verify.ps1`
  after any code touch.
- No real media required for the synthetic checks: `ffmpeg -f lavfi -i
  testsrc=...` + `sine=...` at both standards, then the region-convert filters,
  then `ffprobe -show_entries format=duration:stream=codec_type,duration,
  sample_aspect_ratio,display_aspect_ratio -of json`.
- Real-disc claims (Q5/Q8 full-disc flows, Q6 real grey-market VOBs) must be
  flagged `needs-tester`, not inferred.
- The bundled sidecar binaries are static; for quick probing use `ffmpeg.exe` /
  `ffprobe.exe` from the release zip or any local static build.

## 6. Constraints (binding)

- **No AI attribution** anywhere (commits/docs): no `Co-Authored-By`,
  "Generated with" or session credit. Author identity = repo owner.
- **Research only** unless the Human Director approves a fix. If approved, each
  landing syncs all six docs in the same commit: `docs/roadmap.html` (single
  source of truth), `docs/ROADMAP.md`, `docs/CHANGELOG.md`, `AGENTS.md`,
  `DONE.md`, `TODO.md` — plus behavior docs (`docs/INSTALLATION.md`,
  `docs/PAL_NTSC_CONVERSION.md` as applicable).
- All user-facing strings via `i18n.T().KeyName` (en/fr/iu/iu_latin) — never
  hardcoded literal labels in constructors.
- Linux + Windows only; zero new Windows runtime deps.
- Ask before touching CI workflows; do not gofmt `internal/i18n/*.go`,
  `types.go`; do not rewrite `internal/dvd/iso9660`/`internal/dvd/udf` reader
  externals unless a finding demands it.

## 7. Deliverables

1. `docs/REGION_CONVERSION_AUDIT_FINDINGS.md` — one section per question above,
   each with: verified fact, `file:line` evidence, severity (P0/P1/P2/P3),
   reproduction commands, recommended fix, `needs-tester` flags.
2. A short decision table: "keep as-is / fix now / fix later / document-only"
   per question — this is what the Human Director will act on.
3. A proposed tester checklist (matching `checklistData` style in
   `docs/roadmap.html`) covering the top findings.
4. Optionally, an approved-fix prototype on a scratch branch (never on master).