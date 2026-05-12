# PAL → NTSC Conversion Pipeline

## Problem

The Rip module faithfully preserves whatever is on disc. A PAL DVD source
(720×576, 25 fps, interlaced) passes through unchanged. This is correct for
archival — but means:

- H.264 re-encode mode bakes in 576i interlacing (no `yadif` applied)
- The Convert module has no PAL→NTSC path
- Authoring the ripped file back as NTSC produces wrong IFO timestamps and
  a 4 % speed/pitch error

## Background: why audio needs pitch correction

PAL DVDs contain film that was originally shot at 24 fps (23.976 strictly).
The PAL master speeds the film up from 24→25 fps (a 4.167 % increase) and
records the audio at that higher speed. When converting back to NTSC
(23.976 fps, presented via 3:2 pulldown at 29.97 fps), the audio must be
slowed down by the reciprocal factor: `24/25 = 0.9600`.

Without pitch correction the output runs fast and sounds like chipmunks.

## Three gaps to close

### 1. Rip: deinterlace on H.264 re-encode

**File:** `internal/app/modules/rip/executor.go` → `BuildRipArgs`

When `Format` is `FormatH264MKV` or `FormatH264MP4` and the source IFO
indicates interlaced video (`titleInfo` scan field, or a quick ffprobe), add
`yadif=mode=1` to the video filter chain.

```go
// in BuildRipArgs, H.264 branch:
"-vf", "yadif=mode=1",
"-c:v", "libx264",
"-crf", "18",
"-preset", "medium",
```

Do **not** apply unconditionally — progressive sources must not be filtered.
Heuristic: if `titleInfo != nil && titleInfo.Interlaced` (field to add to
`ifo.TitleInfo`), apply the filter. Fall back to ffprobe `field_order` if
the IFO flag is absent.

### 2. Convert: PAL→NTSC preset

**File:** `internal/convert/presets.go` (or wherever DVD-specific presets live)

The complete ffmpeg filter chain:

```
Video:  yadif=mode=1, scale=720:480:flags=lanczos, fps=30000/1001
Audio:  atempo=0.9600
```

Full argument sketch:

```
ffmpeg -i input.mkv \
  -vf "yadif=mode=1,scale=720:480:flags=lanczos,fps=30000/1001" \
  -c:v libx264 -crf 18 -preset medium \
  -c:a aac -af "atempo=0.9600" \
  -b:a 192k \
  output_ntsc.mp4
```

Notes:
- `yadif=mode=1` — field-aware deinterlace (send frame mode); use `mode=3`
  for send field (doubles frame rate) if quality matters more than speed.
- `scale=720:480:flags=lanczos` — lanczos is slower but sharper than bilinear
  for downscale. Preserves the original 4:3 or 16:9 display aspect ratio
  because both 720×576 and 720×480 use SAR metadata to signal the true DAR.
- `fps=30000/1001` — changes the presentation rate to NTSC; combined with
  the audio slowdown this puts A/V back in sync.
- `atempo=0.9600` — valid range for a single `atempo` filter is 0.5–2.0,
  so 0.96 is fine as a single pass.

### 3. Convert UI: expose the preset

Surface the preset in the Convert module's format/preset picker under a
**"Disc"** group or similar. Label: `PAL → NTSC (DVD)`.

The preset should pre-fill:
- Output resolution: 720×480
- Frame rate: 29.97 fps
- Deinterlace: yadif (auto-on)
- Audio: AAC 192k + atempo pitch correction

## Files to touch

| File | Change |
|------|--------|
| `internal/dvd/ifo/extract.go` | Add `Interlaced bool` to `TitleInfo`; detect from video attr byte |
| `internal/app/modules/rip/executor.go` | Conditionally add `yadif` vf in `BuildRipArgs` |
| `internal/convert/presets.go` | New `DVDNTSCFromPAL()` preset function |
| Convert UI (wherever presets are surfaced) | Add `PAL → NTSC (DVD)` option |

## Do not touch

- `author_module.go`, `author_dvd_functions.go`, `author_menu.go` —
  currently reserved; gemini's DVD authoring work is active here.
- The rip `FormatArchivist` path — stream-copy only, no re-encode.
