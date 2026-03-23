# DVD/ISO Engine — Implementation Status

_Last updated: 2026-03-23_

This document describes the current state of VideoTools' native Go DVD authoring
engine (`internal/dvd/`). For the completion roadmap see `DVD_COMPLETION_PLAN.md`.

---

## Architecture

```
clips (any format)
  │
  ▼ FFmpeg encode (author_module.go)
MPEG-2 Program Stream (.mpg) — DVD-compliant bitrate, GOP, AC-3 audio
  │
  ▼ FFmpeg remux (-f dvd)
VTS_01_1.VOB  placed in VIDEO_TS/
  │
  ├── internal/dvd/ifo  →  VTS_01_0.IFO / .BUP + VIDEO_TS.IFO / .BUP
  │
  └── internal/dvd/udf  →  .iso  (UDF 1.02 + ISO 9660 hybrid filesystem)
```

---

## Layer Status

### 1. VOB Muxer — `internal/dvd/vob/`

| Component | Status | Notes |
|---|---|---|
| Pack header (MPEG-PS) | ✅ Complete | SCR encoding, mux rate |
| PES header | ✅ Complete | PTS/DTS, Private Stream 1 support |
| NAV_PCK write | ✅ Complete | PCI + DSI written per VOBU |
| `WriteVideo(data, pts)` | ✅ Complete | MPEG-2 video PES in a pack |
| `WriteAudio(data, pts, subStreamID)` | ✅ Complete | AC-3 Private Stream 1 PES |
| `TickSCR(ticks)` | ✅ Complete | Advance system clock between writes |
| Padding packets | ✅ Complete | Sector boundary alignment |
| PCI packet content | ✅ Complete | nv_pck_lbn, vobu_s/e_ptm auto-filled from muxer state |
| DSI packet content | ✅ Complete | nv_pck_scr/lbn auto-filled; VOBU_SRI set to end-of-cell |
| SCR increment | ✅ Complete | Frame-accurate: 900,900 ticks (NTSC) / 1,080,000 (PAL); SetFrameRate() |
| Menu VOB (`VIDEO_TS.VOB`) | ✅ Complete | Minimal single-NAV_PCK placeholder created |

### 2. IFO Generator — `internal/dvd/ifo/`

| Component | Status | Notes |
|---|---|---|
| `VTS_MAT` serialisation | ✅ Complete | Video/audio attributes, big-endian |
| `VMG_MAT` serialisation | ✅ Complete | Disc-level metadata |
| `GenerateVTS_IFO()` | ✅ Complete | Accepts `*ProgramChain`; writes MAT + PGCITI + ADMAP |
| `GenerateVMG_IFO()` | ✅ Complete | Accepts `*TT_SRPT`; writes MAT + TT_SRPT |
| `VOBU_ADMAP` | ✅ Complete | Sector map structure; passed as nil currently |
| PGC (Program Chain) types | ✅ Complete | `ProgramChain`, `CellPlayback`, `CellPosition`, `ProgramInfo` |
| `BuildSingleCellPGC()` | ✅ Complete | Single-cell PGC from duration; sector addresses placeholder |
| `WritePGCITI()` | ✅ Complete | Serializes PGCITI with one entry PGC |
| Title Search Pointer Table (TT_SRPT) | ✅ Complete | Built in authoring pipeline; one entry per title set |
| Cell Information Table | ⚠️ Single-cell only | Multi-chapter requires NAV_PCK sector mapping |
| Time Map Table (TMAPT) | ✅ Complete | Linear approximation from VOB file size + duration; 1-second intervals |
| VTS Attribute Table (VTS_ATRT) | ⚠️ Stub | Defined, not populated |
| Audio/subtitle track tables | ❌ Missing | Required for multi-track playback |

### 3. UDF Filesystem — `internal/dvd/udf/`

| Component | Status | Notes |
|---|---|---|
| UDF 1.02 descriptor types | ✅ Complete | PVD, LVD, FSD, FID, ICB, AVDP |
| FileNode tree + AddFile/AddDirectory | ✅ Complete | Recursive traversal |
| `AddDirFS()` | ✅ Complete | Walks OS directory into UDF tree |
| `Build()` | ✅ Complete | WriteHeader → assignSectors → writeNode |
| Descriptor serialisation + CRC | ✅ Complete | |
| Sector alignment (2048 bytes) | ✅ Complete | |
| ISO 9660 PVD | ✅ Complete | Written as hybrid disc |
| ISO 9660 directory tree | ✅ Complete | Path tables + directory records written; file data shared with UDF |
| Multi-partition map support | ⚠️ Limited | Fixed 64-byte partition map array |

### 4. SPU Encoder — `internal/dvd/spu/`

| Component | Status | Notes |
|---|---|---|
| 2-bit RLE encoding | ✅ Complete | Row-by-row pixel encoding |
| Display Control Sequence (DCSQ) | ✅ Complete | Full implementation with area/offset |
| Color palette support | ✅ Complete | 4-color palette with user-defined colors |
| Contrast/timing DCSQ commands | ✅ Complete | SET_CONTR and timing implemented |
| Menu integration | ✅ Wired | Encoder called from authoring via vob.WriteSPU |

### 5. Theme / Menu Renderer — `internal/dvd/theme/`

| Component | Status | Notes |
|---|---|---|
| JSON theme loading | ✅ Complete | Text, button, image elements |
| `RenderMenu()` | ✅ Complete | Produces `image.Image` + highlight palette |
| `drawText()` with OpenType fonts | ✅ Complete | |
| `drawHighlight()` button regions | ✅ Complete | |
| CSS-like layout parsing | ✅ Complete | center, %, px |
| Button → DVD button definition | ✅ Complete | Button commands parsed → PGC cell command table entries |
| Menu SPU encoding bridge | ✅ Complete | Native Go SPU encoder (buildMenuSPU) replaces spumux; output wired into VIDEO_TS.VOB |
| Menu VOB muxing | ✅ Complete | menu_spu.mpg → VIDEO_TS.VOB; menu PGC with pre/cell commands written to VMG IFO |

### 6. Conversion Layer — `internal/convert/`

| Component | Status | Notes |
|---|---|---|
| `DVDNTSCPreset()` | ✅ Complete | 720×480, 29.97fps, AC-3, MPEG-2 |
| `BuildDVDFFmpegArgs()` | ✅ Complete | Full DVD-compliant FFmpeg command |
| `ValidateDVDNTSC()` | ✅ Complete | Framerate, resolution, audio, bitrate |
| `DVDRegion` + multi-region presets | ✅ Complete | NTSC / PAL / SECAM |
| `ValidateForDVDRegion()` | ✅ Complete | |

### 7. Authoring Pipeline — `author_module.go` / `author_dvd_functions.go`

| Component | Status | Notes |
|---|---|---|
| FFmpeg encode clips to MPEG-2 | ✅ Complete | Per-clip progress tracking |
| FFmpeg remux to `-f dvd` | ✅ Complete | Ensures sector-aligned packets |
| Concatenate multiple clips | ✅ Complete | Chapter markers from clip boundaries |
| `VIDEO_TS/` directory creation | ✅ Complete | Fixed 2026-03-17 |
| VOB placement (`VTS_01_1.VOB`) | ✅ Complete | Fixed 2026-03-17 |
| Extra clips as VTS_02+  | ✅ Complete | Fixed 2026-03-17 |
| IFO files go to `VIDEO_TS/` | ✅ Complete | Fixed 2026-03-17 |
| `AUDIO_TS/` creation | ✅ Complete | Empty, required by spec |
| ISO creation via UDF writer | ✅ Complete | |
| `analyzeDVDStructure()` | ✅ Complete | Real ffprobe-based analysis |
| `ripTitle()` | ✅ Complete | Real FFmpeg queue job |
| `ripAllTitles()` | ✅ Complete | Queues all titles |
| Menu generation and wiring | ✅ Complete | menu_spu.mpg copied to VIDEO_TS.VOB; menu PGC with button commands wired into VMG IFO |
| `VIDEO_TS.VOB` (menu VOB) | ✅ Complete | Created from spumux output when menus enabled; minimal placeholder otherwise |
| dvdauthor XML | ⚠️ Dead code | Generated but no longer called |

---

## Compatibility

| Player | Expected behaviour |
|---|---|
| VLC | Plays correctly — reads VOB directly, ignores missing PGC |
| FFmpeg / HandBrake | Reads correctly — probes VOB stream |
| mpv | Plays correctly |
| Kodi (strict mode) | May fail navigation — PGC required |
| Hardware DVD player | Basic play + menu navigation should work — PGC, TT_SRPT, menu PGC with button commands; seek limited (no TMAPT) |
| Windows DVD Player | Should work — IFO has PGC, TT_SRPT, and menu PGC with button commands |

---

## What the engine produces today

A structurally valid DVD folder or ISO with:
- Correctly encoded MPEG-2 video and AC-3 audio
- Properly named and placed VOB files
- IFO/BUP header files in the right location
- UDF + ISO 9660 hybrid filesystem (for ISO output)
- Full interactive menus with native Go SPU encoder (zero-dependency, no spumux required)

What it does **not yet** produce:
- Multi-chapter navigation (single-cell PGC only; sector addresses are placeholder 0)
- Fast-forward/rewind seek (requires TMAPT + DSI SRI values)
