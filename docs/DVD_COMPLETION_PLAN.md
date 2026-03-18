# DVD/ISO Engine — Completion Plan

_Created: 2026-03-17_

This document describes the work required to bring the native Go DVD authoring engine
to full spec compliance. See `DVD_IMPLEMENTATION_SUMMARY.md` for the current state.

**Definition of "feature complete":** ISOs play correctly on hardware DVD players,
chapter navigation works, menus display and respond to button presses.

---

## Phase 1 — IFO Navigation Tables (critical path)

This is the single most important gap. Without PGC/Cell data, no hardware player
will navigate the disc correctly.

### 1.1 Define PGC / Cell structures in `internal/dvd/ifo/types.go`

New types needed:

```go
type ProgramChain struct {
    NrOfPrograms    uint8
    NrOfCells       uint8
    PlaybackTime    PlaybackTime
    AudioControl    [8]uint16
    SubpictureCtl   [32]uint32
    NextPGCN        uint16
    PrevPGCN        uint16
    GoUpPGCN        uint16
    PGPlaybackMode  uint8
    StillTime       uint8
    Palette         [16][4]byte   // YCbCr palette for SPU
    CellPlaybackOffset uint16
    CellPositionOffset uint16
    Programs        []ProgramInfo
    CellPlayback    []CellPlayback
    CellPosition    []CellPosition
}

type CellPlayback struct {
    BlockMode       uint8
    BlockType       uint8
    SeamlessPlay    uint8
    Interleaved     uint8
    STCDiscontinuity uint8
    PlaybackMode    uint8
    RestrictedMode  uint8
    CellType        uint8
    StillTime       uint8
    CommandNr       uint8
    PlaybackTime    PlaybackTime
    FirstSector     uint32
    FirstILVUEndSector uint32
    LastVOBUStartSector uint32
    LastSector      uint32
}

type CellPosition struct {
    VOBID  uint16
    CellID uint8
}

type ProgramInfo struct {
    EntryCell uint8
}
```

### 1.2 Build PGC from VOB sector data

`ifo.Builder` needs a method to construct a PGC from the information available
after VOB encoding:

```go
func BuildPGCFromVOB(vobPath string, chapters []ChapterEntry) (*ProgramChain, error)
```

This function:
1. Scans the VOB file for NAV_PCK sector addresses (already collected in
   `Muxer.NAVPCKSectors`)
2. Maps chapter timestamps to sector addresses
3. Creates one `CellPlayback` entry per chapter
4. Calculates `PlaybackTime` from actual duration

For now, a simplified single-cell PGC (no chapters) is acceptable as an
intermediate step — at least hardware players can navigate to the title.

### 1.3 Add TT_SRPT (Title Search Pointer Table) to VMG

`VMG_MAT` currently writes a header with a zero offset to TT_SRPT. This table
must list each title set with:
- VTS number
- Title number within VTS
- Number of chapters
- Playback type flags

```go
type TT_SRPT_Entry struct {
    TitleType    uint8
    NrOfAngles   uint8
    NrOfPTTs     uint16  // chapters
    VTSN         uint8   // VTS number
    VTS_TTN      uint8   // title number within VTS
    LBA          uint32  // sector address of VTS
}
```

### 1.4 Add Time Map Table (TMAPT)

Required for fast-forward / rewind seek on hardware players. Maps time offsets
to sector addresses at 1-second intervals.

```go
type TimeMapEntry struct {
    TimeUnit    uint32  // in 90kHz ticks
    Sector      uint32
}
```

### 1.5 Wire PGC data into `GenerateVTS_IFO()`

Current signature:
```go
func (b *Builder) GenerateVTS_IFO(vtsNumber int, mat *VTS_MAT, admap *VOBU_ADMAP) error
```

Extend to:
```go
func (b *Builder) GenerateVTS_IFO(vtsNumber int, mat *VTS_MAT, pgc *ProgramChain, admap *VOBU_ADMAP) error
```

Update `author_module.go` to pass the built PGC.

---

## Phase 2 — NAV_PCK Content (PCI / DSI)

Currently NAV_PCKs are written with correct structure but zeroed content. The
packets are valid enough for most software players; hardware players use them
for seek and button highlight.

### 2.1 Populate PCI packet

In `nav.go`, `WriteNAV_PCK()` needs to fill:
- `LVOBU_S_PTM` — presentation start time (from SCR at VOBU start)
- `LVOBU_E_PTM` — presentation end time
- `HL_GI.BTN_SL_NS` — number of buttons (0 for non-menu VOBUs)

### 2.2 Populate DSI packet

- `NV_PCK_SCR` — SCR at NAV_PCK
- `VOBU_EA` — last sector offset of this VOBU
- `SRI` array — forward/backward sector offsets at time intervals (enables
  fast-forward/rewind on hardware)

The SRI values can be approximated from the `NAVPCKSectors` list collected
during muxing.

### 2.3 Fix SCR increment

Currently `WriteVideo()` and `WriteAudio()` use a fixed 1800-tick SCR advance.
This should be derived from frame rate:
- NTSC 29.97fps: 90000 / 29.97 ≈ 3003 ticks per frame
- PAL 25fps: 90000 / 25 = 3600 ticks per frame

Pass frame rate through `NewMuxer()` or add a `SetFrameRate()` method.

---

## Phase 3 — Menu VOB

### 3.1 Create `VIDEO_TS.VOB`

Even an empty/still menu VOB is sufficient for most hardware players. The
minimum valid menu VOB is a single NAV_PCK followed by one frame of still video
and silence.

In `author_module.go`, after VOB placement:
```go
// Create a minimal menu VOB placeholder
menuVOBPath := filepath.Join(videoTSPath, "VIDEO_TS.VOB")
createMinimalMenuVOB(menuVOBPath)
```

`createMinimalMenuVOB()` writes a single VOBU with:
- NAV_PCK with correct PCI flags for a menu
- One sector of video padding (black frame)

### 3.2 Wire theme renderer → SPU encoder → menu VOB

This is the full menu pipeline. Only tackle after Phase 1 and 3.1 are done.

```
RenderMenu(theme)      →  image.Image + ButtonRect list
  ↓
spu.Encoder.Encode()   →  []byte (RLE subpicture)
  ↓
vob.Muxer.WriteVideo() +
vob.Muxer.WriteAudio() +
writeSPUPack()         →  VIDEO_TS.VOB sectors
  ↓
ifo: PGC with HL_GI button definitions
     (button coordinates, colour palette, nav_aid links)
```

Key missing pieces:
- `writeSPUPack()` — SPU data as Private Stream 1 (sub-stream 0x20)
- Button definition serialisation in PGC (highlight coordinates → `HL_GI`)
- Menu PGC navigation commands (Play Title N on button press)

---

## Phase 4 — Polish and Validation

### 4.1 ISO 9660 directory tree

`internal/dvd/udf/iso9660.go` writes the PVD but does not populate directory
records. This means strictly ISO 9660-only readers (rare on DVD players, common
on old CD-ROM drives) cannot read the disc. For DVD-Video this is low priority
but should be completed for full hybrid compliance.

### 4.2 SPU DCSQ timing and contrast

`internal/dvd/spu/spu.go` writes the DCSQ structure but leaves area offsets and
contrast values as zeros. Fix `Encode()` to:
- Calculate actual pixel area from the image dimensions
- Write `SET_CONTR` command with opacity values
- Write `SET_COLOR` command referencing PGC palette indices

### 4.3 Integration testing

Write a test that:
1. Encodes a short test clip (or uses a pre-encoded VOB)
2. Calls the full authoring pipeline
3. Validates the output with `ffprobe` (streams, duration)
4. Validates IFO structure (sector offsets, PGC presence)

---

## Work Order

| Phase | Item | Effort | Blocker for hardware players |
|---|---|---|---|
| 1.1 | Define PGC/Cell types | ✅ Done | — |
| 1.2 | `BuildSingleCellPGC()` | ✅ Done | — |
| 1.3 | TT_SRPT in VMG | ✅ Done | — |
| 1.5 | Wire PGC into `GenerateVTS_IFO()` | ✅ Done | — |
| 3.1 | Minimal `VIDEO_TS.VOB` | ✅ Done | — |
| 2.1 | Populate PCI (times only) | ✅ Done | — |
| 2.2 | Populate DSI SRI (seek) | ✅ Done | — |
| 2.3 | Fix SCR frame-rate accuracy | ✅ Done | — |
| 1.4 | Time Map Table | ✅ Done | No — enables seek |
| 3.2 | Full menu pipeline | ✅ Done | No — enables menus |
| 4.1 | ISO 9660 directory tree | ✅ Done | No |
| 4.2 | SPU DCSQ contrast/timing | S | No |
| 4.3 | Integration tests | M | No |

**Effort key:** S = 1–2 days, M = 3–5 days, L = 1–2 weeks

**Critical path to hardware player compatibility:** 1.1 → 1.2 → 1.3 → 1.5 → 3.1

---

## File Reference

| File | Role |
|---|---|
| `internal/dvd/vob/vob.go` | MPEG-PS pack/PES muxer |
| `internal/dvd/vob/nav.go` | NAV_PCK (PCI + DSI) writer |
| `internal/dvd/ifo/types.go` | DVD structure types |
| `internal/dvd/ifo/builder.go` | IFO/BUP file generation |
| `internal/dvd/ifo/vmgi.go` | VMG_MAT, TT_SRPT |
| `internal/dvd/ifo/vtsi.go` | VTS_MAT, VOBU_ADMAP |
| `internal/dvd/spu/spu.go` | 2-bit RLE subpicture encoder |
| `internal/dvd/theme/theme.go` | JSON menu theme loader |
| `internal/dvd/theme/renderer.go` | Menu image renderer |
| `internal/dvd/udf/udf.go` | UDF 1.02 + ISO 9660 writer |
| `author_module.go` | Authoring pipeline orchestration |
| `author_dvd_functions.go` | DVD rip tab, analyzeDVDStructure, ripTitle |
