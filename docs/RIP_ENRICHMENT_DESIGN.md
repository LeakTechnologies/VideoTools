# Rip Enrichment Design

## Overview

Enrich the Rip module output so that ripped video files play with full DVD fidelity
in VLC and other players: chapters at correct timecodes, all audio tracks with language
tags, optional subtitle streams, and disc/title metadata embedded as MKV/MP4 tags.

## What Gets Embedded

| Feature | Source | Output |
|---------|--------|--------|
| Chapters | `VTS_xx_0.IFO` PGC cell table | ffmetadata `[CHAPTER]` blocks |
| Audio tracks | All audio streams in VOB, language from IFO | `-map 0:a` + `-metadata:s:a:N language=xx` |
| Subtitles | DVD bitmap subpicture streams | `-map 0:s` + language tag (optional) |
| Title | UDF volume label (ISO) or parent folder name | `-metadata title=...` |
| Future: full metadata | TMDB/OMDb API via disc hash | MKV tags: year, description, cover art |

## Chapter Extraction

Source: `VTS_xx_0.IFO` — the VTS_PGCITI (PGC Information Table).

Pipeline:
1. `ReadVTSI()` → parse VTS_MAT, get `VTS_PGCITI_Offset`
2. Seek to that offset, read PGCITI header (NrOf_PGCI_SRP, EndByte)
3. Read PGCI_SRP table → find first title-domain PGC → read PGC
4. Extract `Programs[]` (entry cell per chapter) and `CellPlayback[]` (BCD durations)
5. Accumulate cell durations → chapter start times in seconds

Chapter format written to temp file (ffmetadata):
```
;FFMETADATA1
title=Disc Title

[CHAPTER]
TIMEBASE=1/1000
START=0
END=300000
title=Chapter 1
```

## Audio Language Tags

`ReadVTSI()` already reads `VTS_Audio_Attributes[i].LanguageCode` (ISO 639-1, 2 bytes).
The IFO audio track index N maps 1:1 to the Nth audio stream ffprobe finds in the VOB
(DVD stream IDs 0x80-0x87 for AC-3 are discovered in order).

FFmpeg flag: `-metadata:s:a:N language=xx` per track.

## Subtitle Streams

DVD subpicture streams (dvd_subtitle codec) embed as bitmap subtitle tracks in MKV.
VLC displays them correctly. They require `-fflags +genpts` (already in place) to avoid
timestamp rejection.

Flag: `-map 0:s` (optional, off by default — some discs have corrupt sub streams).
Language tag: `-metadata:s:s:N language=xx` from `VTS_Subpicture_Attrs[i].LanguageCode`.

## Disc Title Detection

Priority:
1. UDF volume label from ISO (via `udf.Reader.GetVolumeLabel()`)
2. Parent folder name (directory source)
3. User-editable text field in the UI

Future: DVD disc hash → TMDB/OMDb lookup for full metadata (title, year, genre, poster).

## UI Options

New checkboxes in the Rip module:
- [x] Embed chapters (on by default)
- [x] All audio tracks (on by default)
- [ ] Include subtitles (off by default — some discs have broken sub streams)

Title field: editable, pre-filled from disc label or folder name.

## Files Modified

1. `internal/dvd/ifo/extract.go` — NEW: `ReadTitleInfo()` returns chapters + track metadata
2. `internal/app/modules/rip/executor.go` — chapter file gen, enriched ffmpeg args
3. `internal/app/modules/rip/types.go` — new options fields
4. `internal/app/modulecfg/rip.go` — persist new options
5. `internal/app/modules/rip/view.go` — checkboxes + title field
6. `rip_module.go` — wire new options through

## Alternate Angles

Multi-angle DVDs interleave VOBUs from different angles. Detected via cell block mode
flags in the IFO (`BlockMode != 0`). For now: detect and warn; angle selection is a
future feature (would require demuxing specific cell blocks from the interleaved VOB).
