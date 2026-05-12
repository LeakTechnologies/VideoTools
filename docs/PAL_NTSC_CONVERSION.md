# PAL → NTSC Conversion Pipeline

## Background: why audio needs pitch correction

PAL DVDs contain film originally shot at 24 fps (23.976 strictly).
The PAL master speeds the film up from 24→25 fps (a 4.167 % increase) and
records audio at that higher speed. When converting back to NTSC
(23.976 fps, presented via 3:2 pulldown at 29.97 fps), audio must be
slowed down by the reciprocal factor: `24/25 = 0.9600`.

Without pitch correction the output runs fast and sounds like chipmunks.

---

## Status: what is implemented

### IFO interlace detection

**File:** `internal/dvd/ifo/extract.go`

`TitleInfo.Interlaced bool` is derived from the `FilmMode` byte in each
VTS attribute block:

- `FilmMode == 0` → camera/interlaced → `Interlaced = true`
- `FilmMode == 1` → film/progressive → `Interlaced = false`

The detection result is logged at parse time.

### Convert-during-rip (Rip module)

**File:** `internal/app/modules/rip/view.go` + `executor.go`

A **"Convert PAL → NTSC during rip"** checkbox is present in the Rip module's
enrichment options section. It is enabled only for H.264 re-encode formats
(MKV and MP4); it is disabled with an explanatory label for Lossless and
Archivist formats (which are stream-copy only).

When the checkbox is ticked the executor (`BuildRipArgs`) injects:

```
Video:  -vf "yadif=mode=1,scale=720:480:flags=lanczos,fps=30000/1001"
Audio:  -af "atempo=0.9600"
```

If the checkbox is off but the IFO reports an interlaced source and the
format is H.264, only `yadif=mode=1` is applied (no scale or fps change).

Filter notes:
- `yadif=mode=1` — field-aware deinterlace (send-frame mode)
- `scale=720:480:flags=lanczos` — lanczos preserves sharpness; SAR metadata
  carries the original DAR so 4:3 / 16:9 is not distorted
- `fps=30000/1001` — sets presentation rate to 29.97 NTSC
- `atempo=0.9600` — single-pass pitch correction; valid range 0.5–2.0

The job config key is `"convertToNTSC": bool`, read in `rip_module.go /
executeRipJob`.

---

## What is not yet implemented: full disc with menu preservation

Convert-during-rip works for the main feature of a single title set. A true
one-touch PAL→NTSC with working menus requires significantly more:

### What "full disc" means

A DVD disc contains:
- **VMGI** (`VIDEO_TS.IFO` / `VIDEO_TS.VOB`) — the disc menu
- **VTS sets** (`VTS_nn_0.IFO`, `VTS_nn_n.VOB`) — one per title set

The current Rip module only extracts the main feature VOB from one VTS set.
Menu VOBs and secondary title sets are not touched.

### The full pipeline (not yet built)

#### Stage 1 — Extract full disc

The Rip module needs an "extract full disc" mode that:

1. Iterates every VTS set (`VTS_01` … `VTS_nn`) and the VMGI menu
2. Decrypts each VOB via CSS (already available in `internal/dvd/css`)
3. Demuxes into per-VTS elementary streams (audio + video per stream, keeping
   all audio tracks and subtitle streams)

#### Stage 2 — Convert each stream to NTSC

Each extracted VOB/elementary stream is passed through the same filter chain:

```
Video:  yadif=mode=1, scale=720:480:flags=lanczos, fps=30000/1001
Audio:  atempo=0.9600  (per audio track)
```

Menu VOBs contain MPEG-2 video — they must be re-encoded to MPEG-2 at 720×480
29.97 fps (menus are not H.264). Audio in menus is typically AC-3; pitch
correction still applies.

#### Stage 3 — Author NTSC disc structure

The converted streams and timing data are handed off to the Author module to:

1. Regenerate all IFO files with correct NTSC timing (cell durations, PGC
   timestamps, chapter offsets — all derived from the new frame count and fps)
2. Re-author the menu VOB with updated navigation commands if cell addresses
   changed
3. Write the final `VIDEO_TS/` directory ready for burning or ISO creation

This is the largest gap. The Author module already exists
(`author_module.go`, `author_dvd_functions.go`, `author_menu.go`) but needs
an automated "receive converted streams and regenerate NTSC disc" mode. This
work is currently **reserved for the gemini agent**.

### Known hard problems

| Problem | Notes |
|---------|-------|
| Menu button highlight colours | DVD menus use a CLUT palette for button highlights; addresses change if VOB cell layout shifts |
| Multi-angle titles | Each angle stream must be converted separately and re-muxed |
| Seamless branching | Some discs use PGC branching chains; cell addresses in new IFOs must exactly match new VOB pack positions |
| Audio track mapping | Multi-language discs have several audio streams per VTS; each needs independent atempo pass |
| Forced subtitles | Bitmap subtitle streams (DVD_SUBTITLE) are not affected by fps/scale changes but their display timestamps must be remapped |

---

## Roadmap

| # | Milestone | Owner | Status |
|---|-----------|-------|--------|
| 1 | IFO interlace detection | done | ✅ |
| 2 | Convert-during-rip checkbox (single title set, H.264) | done | ✅ |
| 3 | Multi-VTS extraction mode in Rip module | opencode | queued |
| 4 | Per-stream NTSC conversion (including menu re-encode) | opencode | queued |
| 5 | Author module: automated NTSC IFO regeneration from converted streams | gemini | queued |
| 6 | Burn: one-touch "rip PAL → burn NTSC" end-to-end path | — | future |

---

## Files not to touch

- `author_module.go`, `author_dvd_functions.go`, `author_menu.go` —
  reserved for gemini's DVD authoring work.
- `internal/convert/` — PAL→NTSC is a disc-pipeline feature; the Convert
  module handles arbitrary file re-encodes and has no DVD-specific paths.
- The Rip `FormatArchivist` path — stream-copy only, no re-encode filters.
