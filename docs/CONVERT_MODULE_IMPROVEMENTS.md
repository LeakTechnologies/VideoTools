# Convert Module Improvements Plan

## Overview

The Convert module is the most feature-rich module but has significant gaps compared to professional video tools like HandBrake, ShanaEncoder, and VidCoder. This document outlines improvements organized by priority.

## Current State

- `buildConvertView()` is ~4,100 lines in `main.go` (lines 8620-12732)
- `convert_player_native.go` handles native media player integration
- `internal/app/modules/convert/view.go` is a placeholder shim
- 12 format presets exist in the UI dropdown but `internal/convert/presets.go` only defines 5
- Config struct has fields (NormalizeAudio, AudioSampleRate, Deinterlace, DeinterlaceMethod, H264Profile, H264Level) that have **no UI controls**
- ~50+ hardcoded English strings need i18n

---

## Phase 1: Missing UI Controls for Existing Config Fields

### 1.1 Audio Sample Rate Dropdown

**Config field exists:** `AudioSampleRate` (Source, 44100, 48000)  
**UI:** None

**Implementation:**
- Add `Select` widget with options: Source, 22050 Hz, 44100 Hz, 48000 Hz, 96000 Hz
- Wire to `convertConfig.AudioSampleRate`
- Update FFmpeg command builder to add `-ar <rate>` when not "Source"

**Files:**
- `main.go` — `buildConvertView()` in audio encoding section
- `internal/convert/ffmpeg.go` — Add `-ar` to command
- `internal/i18n/strings.go` — Add `ConvertAudioSampleRate`

### 1.2 Normalize Audio Checkbox

**Config field exists:** `NormalizeAudio`  
**UI:** None

**Implementation:**
- Add checkbox in audio encoding section
- When enabled, add `-af loudnorm=I=-16:TP=-1.5:LRA=11` to FFmpeg command
- Show LUFS/TruePeak sliders when checked (similar to Audio module)

**Files:**
- `main.go` — Add checkbox and conditional sliders
- `internal/convert/ffmpeg.go` — Add loudnorm filter

### 1.3 Deinterlace Mode/Method Dropdowns

**Config fields exist:** `Deinterlace` (Auto/Force/Off), `DeinterlaceMethod` (yadif/bwdif)  
**UI:** Only "Smart Inverse Telecine" checkbox and "Analyze Interlacing" button

**Implementation:**
- Add `Deinterlace` dropdown: Off / Auto / Force
- Add `Deinterlace Method` dropdown: Yadif / BWDIF (shown when Force or Auto)
- Replace current "Analyze Interlacing" section with improved layout

**Files:**
- `main.go` — Add dropdowns near interlace section
- `internal/convert/ffmpeg.go` — Wire deinterlace filter selection

### 1.4 H.264 Profile/Level Controls

**Config fields exist:** `H264Profile`, `H264Level`  
**UI:** Only applied via device presets, no manual control

**Implementation:**
- Add Profile dropdown: Baseline / Main / High / High 10 (shown when codec is H.264)
- Add Level dropdown: 3.0 / 3.1 / 4.0 / 4.1 / 4.2 / 5.0 / 5.1 / 5.2
- Show only when H.264 is selected
- Add `-profile:v` and `-level` to FFmpeg command

**Files:**
- `main.go` — Add conditional dropdowns
- `internal/convert/ffmpeg.go` — Add profile/level params

---

## Phase 2: Format Preset Expansion

### 2.1 Add Missing Container/Codec Combinations

**Current presets (in UI dropdown):**
- MP4 (H.264), MP4 (H.265), MOV (H.264), MOV (H.265), MOV (ProRes)
- MKV (H.264), MKV (H.265), MKV (Remux)
- MKV (AV1), MP4 (AV1/SVT-AV1)
- WebM (VP9), WebM (AV1)
- DVD-NTSC, DVD-PAL

**Missing from `internal/convert/presets.go` (only 5 defined):**
- Many presets in the UI are defined inline in `main.go` rather than in `presets.go`
- Need to consolidate all format presets into `presets.go`
- Add: AVI, FLV, TS/M2TS, 3GP, OGG

**Files:**
- `internal/convert/presets.go` — Expand with all 12+ format presets
- `main.go` — Remove inline format definitions, reference presets.go

### 2.2 Add x264/x265 Tuning Options

**Missing tunings (common in HandBrake):**
- Film
- Animation
- Grain
- Stillimage
- Fastdecode
- Zeroen lag

**Implementation:**
- Add "Tune" dropdown (visible when codec is H.264 or H.265)
- Add `-tune <value>` to FFmpeg command
- Show quality hint text based on tune selection

**Files:**
- `main.go` — Add tuning dropdown
- `internal/convert/ffmpeg.go` — Add `-tune` param

---

## Phase 3: Subtitle Track Selection in Convert

### 3.1 Per-Track Subtitle Mode Dropdown

**Design doc:** `docs/SUBTITLE_HANDLING_DESIGN.md`

**Implementation:**
- After loading a video with subtitle tracks, show a list:
  ```
  [Stream 2] English (SRT)     [Passthrough ▼]
  [Stream 3] French (SRT)      [Burn-in ▼]
  [Stream 4] Spanish (VOBSUB)  [None ▼]
  ```
- Three modes per track: Passthrough (copy), Burn-in (hardsub), None (exclude)
- Default: Passthrough for all tracks
- Add to `ConvertConfig`: `SubtitleTracks []SubtitleTrackConfig`

**Files:**
- `main.go` — Add subtitle section in buildConvertView
- `internal/convert/types.go` — Add SubtitleTrackConfig
- `internal/convert/ffmpeg.go` — Build `-map` and subtitle filter chains

### 3.2 Forced Subtitle Handling

- Detect forced subtitle tracks from source metadata
- Option to "Always burn forced subtitles" checkbox
- Add `-disposition:s <id> forced` for passthrough tracks flagged as forced

**Files:**
- `main.go` — Add forced subtitle option
- `internal/convert/ffmpeg.go` — Add forced subtitle handling

---

## Phase 4: Per-Stream Audio Mapping

### 4.1 Audio Track Selection UI

**Current:** All audio tracks or first track only (no per-track control)

**Target:**
```
[Stream 1] AAC Stereo 48kHz    [Encode AAC 192k ▼]
[Stream 2] AC3 5.1 48kHz        [Copy ▼]
[Stream 3] Commentary AAC       [None ▼]
```

**Implementation:**
- After loading, list all audio streams
- Per-track dropdown: Encode (with codec/bitrate options) / Copy / None
- Default: Keep first track, set others to None
- Add to `ConvertConfig`: `AudioStreamMaps []AudioStreamConfig`

**Files:**
- `main.go` — Add audio stream section in buildConvertView
- `internal/convert/types.go` — Add AudioStreamConfig
- `internal/convert/ffmpeg.go` — Build per-stream `-map` and codec args

### 4.2 Multiple Audio Output Tracks

- Allow encoding same source track to multiple output codecs
- Example: Encode audio track 1 as both AAC 192k (compatibility) and Opus 128k (quality)

**Files:**
- Same as 4.1, extend per-track config

---

## Phase 5: Video Filter Integration in Convert

### 5.1 Quick Filter Toggles

**Currently:** Filters exist in separate Filters module only

**Target:** Add quick toggle rows in Convert Advanced tab:

```
☐ Denoise (nlmeans)     Preset: [Light ▼]
☐ Sharpen (unsharp)     Amount: [Medium ▼]
☐ Deblock               Strength: [Light ▼]
☐ Stabilize (deshake)   □
```

**Implementation:**
- Add `ConvertFilters` struct to config: Denoise, DenoisePreset, Sharpen, SharpenPreset, Deblock, DeblockStrength, Stabilize
- Add UI checkboxes with conditional preset dropdowns
- Append `-vf` filters to FFmpeg command before video encoding args

**Files:**
- `main.go` — Add filter toggles in advanced tab
- `internal/convert/types.go` — Add ConvertFilters
- `internal/convert/ffmpeg.go` — Add filter chain construction

### 5.2 Speed Change (Not Just Framerate)

- Add speed control: 0.25x / 0.5x / 0.75x / 1.0x / 1.25x / 1.5x / 2.0x / Custom
- Apply via `setpts=PTS/<speed>` for video and `atempo=<speed>` for audio

**Files:**
- `main.go` — Add speed control dropdown
- `internal/convert/ffmpeg.go` — Add setpts/atempo filters

---

## Phase 6: Metadata Handling

### 6.1 Metadata Passthrough

**Current:** Source metadata is detected (`HasMetadata`, `Metadata` map) but NOT written to output

**Target:**
- Checkbox: "Copy source metadata" (default: ON)
- When enabled, add `-map_metadata 0` to FFmpeg command
- When disabled, add `-map_metadata -1` (strip metadata)

**Files:**
- `main.go` — Add checkbox in output section
- `internal/convert/ffmpeg.go` — Add metadata mapping

### 6.2 Custom Metadata Editor

- Expandable section for custom metadata:
  ```
  Title:    [________________]
  Artist:   [________________]
  Album:    [________________]
  Year:     [____]
  Comment:  [________________]
  ```
- Apply via `-metadata title="..." -metadata artist="..."` etc.
- Auto-populate from source when "Copy source metadata" is checked

**Files:**
- `main.go` — Add metadata editor section
- `internal/convert/ffmpeg.go` — Add metadata args

### 6.3 Cover Art in Output

- Option to embed cover art from:
  - Source file (passthrough)
  - External file (browse button)
  - Screenshot at current position
- Apply via `-i cover.png -map 0:v -map 0:a -map 1:0 -c:v:1 copy -disposition:v:1 attached_pic`

**Files:**
- `main.go` — Extend cover art section
- `internal/convert/ffmpeg.go` — Add cover art mapping

---

## Phase 7: Preset System Completion

### 7.1 User Preset Save/Load/Delete UI

**Current:** `userPreset` struct exists with full config fields, but no Save/Load/Delete UI in buildConvertView

**Implementation:**
- "Save Preset" button opens dialog:
  ```
  Preset Name: [________________]
  ☑ Video settings
  ☑ Audio settings
  ☑ Aspect/crop settings
  ☑ Filters
  ```
- Preset dropdown lists saved presets
- "Delete" button with confirmation
- Presets stored as JSON in app data directory

**Files:**
- `main.go` — Add preset save/load/delete UI in both tabs
- `internal/convert/presets.go` — JSON persistence
- `internal/convert/types.go` — UserPreset serialization

### 7.2 Built-in Device Presets

**Currently partially implemented** (some device presets exist as devicePresetSelect)

**Add:**
- iPhone (H.264, 1080p, AAC Stereo)
- iPad (H.264, 1080p, AAC Stereo)
- Android Phone (H.264, 720p, AAC)
- Android Tablet (H.264, 1080p, AAC)
- Apple TV (H.264, 1080p, AC3 5.1)
- Chromecast (H.264, 1080p, AAC)
- PlayStation (H.264, 1080p, AAC)
- Xbox (H.264, 1080p, AAC)
- YouTube (H.264, 1080p, AAC)
- Vimeo (H.264, 1080p, AAC)
- Discord (H.264, 720p, AAC, <8MB)
- Email (H.264, 480p, AAC, <25MB)

**Files:**
- `internal/convert/presets.go` — Add all device presets
- `main.go` — Wire device preset application logic

---

## Phase 8: Queue & Batch Improvements

### 8.1 Batch Add from Folder/Disc

**Design doc:** `docs/BATCH_QUEUE_DESIGN.md`

**Implementation:**
- "Add Folder" button in queue or convert view
- Folder picker + recursive video file search
- Pre-filter by extension
- Show progress dialog during scan

**Files:**
- `main.go` — Add folder batch dialog
- `internal/queue/queue.go` — Add batch add methods

### 8.2 Queue Completion Action

- After all jobs complete, option to:
  - Show notification
  - Play sound
  - Open output folder
  - Sleep/shutdown computer (platform-specific)

**Files:**
- `main.go` — Add post-completion action dropdown
- `internal/queue/queue.go` — Add completion callback

### 8.3 Estimated Time Remaining

- Show per-job and total queue ETA
- Calculate from average encoding speed
- Display in queue view

**Files:**
- `internal/queue/queue.go` — Add ETA calculation
- Queue UI — Add ETA column

---

## Phase 9: i18n Completion (Issue #5)

### 9.1 Hardcoded String Audit

**~50+ hardcoded strings in `main.go` buildConvertView:**

| Category | Count | Examples |
|----------|-------|---------|
| Button labels | ~15 | "Convert Now", "Add to Queue", "Cancel", "Reset" |
| Section headers | ~8 | "Video Encoding", "Audio Encoding", "Aspect Ratio" |
| Format/codec labels | ~20 | "MP4 (H.264)", "H.264", all bitrate strings |
| Hint/description | ~10 | "Apply all encoding settings optimised for a target device." |
| Dialog text | ~5 | "Save Current Settings as Preset...", "Chapters will be lost..." |

**Implementation:**
- Create i18n keys: `ConvertVideo*`, `ConvertAudio*`, `ConvertFormat*`, `ConvertPreset*`, etc.
- Add to `internal/i18n/strings.go`
- Add English to `en_ca.go`
- Add French to `fr_ca.go`
- Add Inuktitut to `iu.go` / `iu_latin.go`
- Replace all hardcoded strings in buildConvertView

---

## Phase 10: Color/HDR Handling

### 10.1 Color Space Override

- Add color space dropdown: Auto / BT.709 / BT.2020 / BT.601
- Add color primaries: Auto / BT.709 / BT.2020
- Add transfer characteristics: Auto / SRG / PQ (HDR) / HLG
- Add `-color_primaries`, `-color_trc`, `-colorspace` to FFmpeg

### 10.2 HDR Passthrough

- Checkbox: "Preserve HDR metadata" (default: ON for HDR sources)
- Adds `-color_primaries bt2020 -color_trc smpte2084 -colorspace bt2020nc`
- Only shown when source is detected as HDR

---

## Implementation Order

| Phase | Priority | Estimated Effort | Dependency |
|-------|----------|-----------------|------------|
| 1. Missing UI Controls | HIGH | 2-3 days | None |
| 2. Format Presets | HIGH | 1-2 days | None |
| 3. Subtitle Track Selection | HIGH | 3-4 days | Phase 1 |
| 4. Audio Stream Mapping | HIGH | 3-4 days | Phase 1 |
| 5. Video Filter Integration | MEDIUM | 2-3 days | Phase 1 |
| 6. Metadata Handling | MEDIUM | 2-3 days | None |
| 7. Preset System | MEDIUM | 2-3 days | Phase 2 |
| 8. Queue/Batch | MEDIUM | 2-3 days | None |
| 9. i18n (Issue #5) | HIGH | 3-4 days | None |
| 10. Color/HDR | LOW | 2-3 days | Phase 1 |

---

## Eternal Music Reuse

The following components built here will be directly reusable in Eternal Music (`e:\development\eternal-music`):

| Component | Reuse Target |
|-----------|--------------|
| Audio stream mapping UI (Phase 4) | Track picker in Eternal Music |
| Normalize audio with LUFS/TruePeak controls (Phase 1.2) | Mastering in Eternal Music |
| Metadata editor (Phase 6.2) | Tag editor in Eternal Music |
| Audio sample rate/channel selection (Phase 1.1) | Format conversion in Eternal Music |
| Preset system (Phase 7) | Encoding presets in Eternal Music |
| i18n string pattern (Phase 9) | All i18n in Eternal Music |

---

## Testing Checklist

- [ ] All new UI controls properly save/restore from config
- [ ] FFmpeg command preview updates with new controls
- [ ] Config persistence works for new fields
- [ ] Device presets apply all new fields correctly
- [ ] Subtitle burn-in produces visible subtitles
- [ ] Subtitle passthrough preserves all tracks
- [ ] Audio stream mapping produces correct `-map` args
- [ ] Denoise/sharpen filters apply in correct order
- [ ] Metadata passthrough preserves tags
- [ ] Custom metadata appears in output
- [ ] Cover art embeds correctly
- [ ] User presets save/load/delete
- [ ] All new strings are i18n'd