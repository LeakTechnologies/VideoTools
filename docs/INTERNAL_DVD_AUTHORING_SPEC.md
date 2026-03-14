# Technical Specification: Native Go DVD/Blu-ray Authoring (VideoTools)

## 1. Introduction
This document defines the architecture and implementation details for a native Go library to replace `dvdauthor`, `spumux`, and `xorriso`. The goal is a cross-platform, dependency-free solution for generating DVD-Video and Blu-ray compliant images.

## 2. Architecture Overview
The system is divided into four primary layers, which can be used independently or integrated:

1.  **VOB Muxer (`internal/dvd/vob`)**: Program Stream (PS) muxing with Navigation Packs (NAV_PCK).
2.  **SPU Encoder (`internal/dvd/spu`)**: 2-bit RLE encoding for subpicture (button) overlays.
3.  **IFO Generator (`internal/dvd/ifo`)**: Binary serialization of DVD Management and Title sets.
4.  **UDF Bridge Writer (`internal/dvd/udf`)**: ISO 9660 + UDF 1.02 filesystem generation.

---

## 3. Layer 1: MPEG-PS / VOB Muxer
DVDs use a specific subset of MPEG-2 Program Streams called VOB (Video Object).

### 3.1 Navigation Packs (NAV_PCK)
Every VOB Unit (VOBU) **must** start with a NAV_PCK (System Header + PCI + DSI).
*   **PCI (Presentation Control Information)**:
    *   `LVOBU_S_PTM`: Start presentation time.
    *   `LVOBU_E_PTM`: End presentation time.
    *   `HL_GI`: Highlight General Information (button coordinates and colors).
*   **DSI (Data Search Information)**:
    *   `VOBU_SRI`: Search information for fast-forward/rewind (relative offsets to other VOBUs).
    *   `VOBU_EA`: End address of the current VOBU.

### 3.2 Stream IDs
*   `0xE0`: Video Stream (MPEG-2).
*   `0xBD`: Private Stream 1 (AC3, DTS, or SPU).
*   `0xBE`: Padding Stream.

---

## 4. Layer 2: SPU Encoder (Subpictures)
Used for menu buttons and subtitles.

### 4.1 2-bit RLE Encoding
The DVD subpicture format uses 4 colors (Background, Pattern, Emphasis 1, Emphasis 2).
*   **Encoder Logic**:
    1.  Downsample 32-bit RGBA overlays to 2-bit indexed color.
    2.  Apply RLE: `[count][color]`.
    3.  Generate `DCSQ` (Display Control Sequence) commands:
        *   `SET_COLOR`: Assign colors from the PGC palette.
        *   `SET_CONTR`: Set transparency (Alpha).
        *   `CHG_COLCON`: Change color/contrast for button highlights.

---

## 5. Layer 3: IFO/BUP Generation
IFO files are the "brains" of the DVD. They use a strict table-based binary format.

### 5.1 Required Tables
*   **VMGI (Video Manager Information)**: `VIDEO_TS.IFO`
    *   `TT_SRPT`: Title Search Pointer Table (maps titles to VTS).
    *   `PGCI_UT`: PGC Unit Table (Menu navigation).
*   **VTSI (Video Title Set Information)**: `VTS_01_0.IFO`
    *   `VTS_ATTRIBUTES`: Video/Audio/Subpicture attributes.
    *   `VTS_PGCITI`: Program Chain Information Table (Chapters and Playback order).
    *   `VTS_VOBU_ADMAP`: Sector addresses of every NAV_PCK in the VOB set.

---

## 6. Layer 4: UDF 1.02 / ISO 9660 Bridge
To be playable on hardware, the ISO must be a "Bridge" disc.

### 6.1 UDF 1.02 Requirements
*   **Sector Size**: 2048 bytes (Strict).
*   **Alignment**: `VIDEO_TS.IFO` and `VIDEO_TS.BUP` **must** be in different ECC blocks (usually handled by 32KB+ spacing) to ensure recovery if a sector is damaged.
*   **Anchor Volume Descriptor**: Must be at sector 256.
*   **Metadata**: Implementation of ECMA-167 for File Entry (ICB) and File Identifier Descriptors.

---

## 7. Data Flow Integration in VideoTools
1.  **Input**: User selects clips and defines menu in `Author Module`.
2.  **Transcode**: FFmpeg generates raw `.m2v` (Video) and `.ac3` (Audio).
3.  **Menu Gen**:
    *   `spu.Encode()` generates `.sub` binary from PNG buttons.
    *   `vob.Mux()` creates `menu.vob` with NAV_PCKs containing button highlights.
4.  **Authoring**:
    *   `vob.Mux()` joins clips into `VTS_01_1.VOB` ... `VTS_01_N.VOB`.
    *   `ifo.Generate()` scans VOBs to build `VTS_01_0.IFO` and `VIDEO_TS.IFO`.
5.  **Mastering**:
    *   `udf.Write()` takes the `VIDEO_TS` folder and produces the final `.iso`.

---

## 8. Standalone Capability
The library will be structured as a Go Module (`github.com/leak_technologies/go-dvdauthor`) with a clean API:

```go
// Example Standalone Usage
author := dvd.NewAuthor("output/")
author.AddTitle(videoPath, audioPath, chapters)
author.SetMenu(backgroundMPEG, buttons)
err := author.Build()
err = udf.CreateISO("dvd.iso", "output/")
```

## 9. Validation Strategy
1.  **Binary Comparison**: Compare generated IFO headers against `dvdauthor` output.
2.  **Structural Validation**: Use `dvdisaster` or `ifodump` to verify table integrity.
3.  **Hardware Emulation**: Validate ISOs in VLC and specialized DVD players (PowerDVD/MPC-HC) to ensure menu navigation works.
