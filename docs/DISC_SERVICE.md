# Disc Service Contract

Status: **candidate, inspect-only**. No rip, author or burn command is exposed yet.

The disc service is the UI-free boundary between the proven DVD/ISO inspection
code and the VT host. It lets the desktop add-on, CLI and future headless mode
read the same disc facts without importing Fyne or writing VOBs.

## Commands

```text
vt-disc scan --source <dvd-folder|iso|video-ts|ifo|vob> [--json]
```

`--json` is the default. Progress facts are written to stderr; the final result
is a single JSON object on stdout. Errors are JSON on stderr and exit non-zero.

## Source kinds

| Kind | Meaning |
|---|---|
| `dvd_folder` | A folder containing `VIDEO_TS/` |
| `video_ts` | The `VIDEO_TS` directory itself |
| `iso` | A DVD ISO image |
| `file` | An IFO/VOB inside a `VIDEO_TS` directory |

ISO scanning reads only the IFO files through the native UDF reader, with an
ISO 9660 fallback for images whose UDF bridge is missing or damaged. Full
VIDEO_TS extraction remains a later rip-service concern.

## Result shape

```json
{
  "sourceKind": "dvd_folder",
  "sourcePath": "D:\\",
  "discType": "DVD-9",
  "totalSize": 8123456789,
  "region": "Region 1",
  "regionCode": 1,
  "videoStandard": "NTSC",
  "mainTitleNumber": 1,
  "titles": [
    {
      "number": 1,
      "vtsNumber": 1,
      "ttn": 1,
      "numChapters": 24,
      "duration": 6138.4,
      "chapters": [0, 120.2],
      "audio": [
        {"index": 0, "language": "en", "codec": "ac3", "channels": 6}
      ],
      "subtitles": [
        {"index": 0, "language": "en", "codec": "dvd_subtitle"}
      ],
      "hasAngles": false,
      "interlaced": true,
      "cells": [
        {"vobId": 1, "cellId": 1, "firstSector": 0, "lastSector": 511}
      ]
    }
  ],
  "warnings": []
}
```

- `regionCode`: `0` region-free, `1`..`8` single region, `-1` multi-region or
  unknown.
- `mainTitleNumber` is the longest title by duration; zero when no title has a
  duration.
- `warnings` records a missing/unreadable VTS IFO without discarding the titles
  that could be read.

## Error shape

```json
{"error":{"code":"read_title_list","message":"could not read VIDEO_TS.IFO title list"}}
```

Codes: `source_not_found`, `no_video_ts`, `unsupported_source`, `unsupported_bluray`,
`iso_open`, `iso_no_video_ts`, `read_title_list`, `temp_dir`, `temp_write`,
`iso_seek`, `scan_failed`.

## Scope and non-goals

Included:

- DVD folder/VIDEO_TS/ISO/file resolution;
- region, standard, disc type and size;
- title list, duration, chapters, audio/subtitle tracks, angles, interlacing;
- global cell ranges for future exact rip planning.

Not included:

- Blu-ray/BDMV inspection;
- physical-drive authoring behaviour beyond a directory that contains VIDEO_TS;
- full ISO extraction, decryption or VOB copying;
- rip jobs, queue integration, authoring or burning;
- Studio UI or add-on wiring.

## Integration boundary

The VT host or add-on calls this service and renders the result. It must not
parse IFOs itself or bypass the service with a second disc implementation.
Rip planning and execution will be added as separate commands behind the same
contract; the inspect shape above is the stable first slice.
