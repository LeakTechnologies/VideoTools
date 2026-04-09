# VideoTools Feature Roadmap

This document provides a feature comparison with common video conversion tools and outlines the development roadmap for VideoTools. Each feature that needs implementation has a corresponding design document.

## Document Conventions

- ✅ **Implemented** — Feature exists in VideoTools
- ❌ **Not Implemented** — Feature does not exist yet
- 🔄 **Partial** — Feature exists but incomplete
- 📋 **Design Doc** — Separate design document exists

## Quick Reference

| Category | Status | Priority |
|----------|--------|----------|
| Core Encoding | ✅ Complete | — |
| Hardware Encoders | ✅ Complete | — |
| Device Presets | ✅ Complete | High |
| Fast Start (Web Optimized) | ✅ Complete | — |
| Video Filters | ❌ | High |
| Frame Cropping | ❌ | High |
| Batch Queue | ❌ | High |
| Custom Presets | ❌ | Medium |
| Subtitle Burn-in | 🔄 Partial | Medium |
| Queue Edit | ❌ | Low |

---

# Part 1: Implemented Features

## 1.1 Video Encoding

| Feature | VideoTools | Status | Notes |
|---------|------------|--------|-------|
| H.264 (libx264) | ✅ | ✅ | |
| H.265/HEVC (libx265) | ✅ | ✅ | |
| AV1 (libaom-av1) | ✅ | ✅ | |
| VP9 (libvpx-vp9) | ✅ | ✅ | |
| MPEG-2 | ✅ | ✅ | For DVD authoring |
| ProRes | ✅ | ✅ | |

### Hardware Encoders

| Encoder | VideoTools | Status |
|---------|------------|--------|
| NVIDIA NVENC | ✅ | ✅ |
| Intel QSV | ✅ | ✅ |
| AMD VCN/AMF | ✅ | ✅ |
| Apple VideoToolbox | ✅ | ✅ |
| VAAPI (Linux) | ✅ | ✅ |

### Quality Modes

| Mode | VideoTools | Status |
|------|------------|--------|
| Constant Quality (CRF) | ✅ | ✅ |
| Average Bitrate (CBR) | ✅ | ✅ |
| Variable Bitrate (VBR) | ✅ | ✅ |
| Target File Size | ✅ | ✅ |

### H.264 Profile/Level

| Setting | VideoTools | Status |
|---------|------------|--------|
| Baseline | ✅ | ✅ |
| Main | ✅ | ✅ |
| High | ✅ | ✅ |
| Level 3.0–5.1 | ✅ | ✅ |

---

## 1.2 Containers (File Formats)

| Container | VideoTools | Status |
|-----------|------------|--------|
| MP4 | ✅ | ✅ |
| MKV | ✅ | ✅ |
| MOV | ✅ | ✅ |
| WebM | ✅ | ✅ |
| MPG (MPEG-2) | ✅ | ✅ |

---

## 1.3 Audio

| Feature | VideoTools | Status |
|---------|------------|--------|
| AAC | ✅ | ✅ |
| MP3 | ✅ | ✅ |
| AC-3 (Dolby) | ✅ | ✅ |
| FLAC | ✅ | ✅ |
| Opus | ✅ | ✅ |
| Vorbis | ✅ | ✅ |
| Stereo downmix | ✅ | ✅ |
| 48kHz normalization | ✅ | ✅ | ✅ |
| Dynamic Range Compression | ✅ | ❌ | Medium priority |

---

## 1.4 Web Optimization

| Feature | Other Tools | VideoTools | Status |
|---------|-----------|------------|--------|
| Fast Start (moov at start) | ✅ | ✅ | ✅ Implemented in MP4 presets |

**Technical Details:**
- FFmpeg flag: `-movflags +faststart`
- Moves moov atom from end to beginning of file
- Enables progressive download and instant playback
- Already enabled for all MP4 presets

---

## 1.5 Device Presets

| Preset | Other Tools | VideoTools | Status |
|--------|-----------|------------|--------|
| iPhone | ✅ | ✅ | ✅ Just added |
| Android | ✅ | ✅ | ✅ Just added |
| Chromecast | ✅ | ✅ | ✅ Just added |
| Fire TV | ✅ | ✅ | ✅ Just added |
| Web | ✅ | ✅ | ✅ Just added |
| Apple TV | ✅ | ❌ | Low |
| Roku | ✅ | ❌ | Low |
| PS4/PS5 | ✅ | ❌ | Low |
| Xbox | ✅ | ❌ | Low |

---

## 1.6 Queue System

| Feature | Other Tools | VideoTools | Status |
|---------|-----------|------------|--------|
| Add single job | ✅ | ✅ | ✅ |
| Start queue | ✅ | ✅ | ✅ |
| Pause/Resume | ✅ | ✅ | ✅ |
| Progress display | ✅ | ✅ | ✅ |
| Batch add (folder) | ✅ | ❌ | High priority |
| Edit queued job | ✅ | ❌ | Low priority |
| Reorder jobs | ✅ | ❌ | Low priority |

---

## 1.7 Chapter Markers

| Feature | Other Tools | VideoTools | Status |
|---------|-----------|------------|--------|
| Auto-detect chapters | ✅ | ✅ | In merge module |
| Add chapter markers | ✅ | ✅ | ✅ |
| Custom chapter names | ✅ | ❌ | Low priority |

---

## 1.8 Resolution & Frame Rate

| Feature | Other Tools | VideoTools | Status |
|---------|-----------|------------|--------|
| Resolution scaling | ✅ | ✅ | ✅ |
| Frame rate control | ✅ | ✅ | ✅ |
| Peak framerate limiting | ✅ | ✅ | ✅ |

---

# Part 2: Not Yet Implemented (Priority Order)

## 2.1 Video Filters 🔴 HIGH PRIORITY

### Design Document: `docs/FILTER_INTEGRATION_DESIGN.md`

| Filter | Other Tools | VideoTools | FFmpeg Filter | Priority |
|--------|-----------|------------|---------------|----------|
| Deinterlace (Yadif) | ✅ | ❌ | yadif | High |
| Deinterlace (Bwdif) | ✅ | ❌ | bwdif | High |
| Denoise (NLMeans) | ✅ | ❌ | nlmeans | High |
| Sharpen (Unsharp) | ✅ | ❌ | unsharp | Medium |
| Deblock | ✅ | ❌ | deblock | Medium |
| Colorspace | ✅ | ❌ | colorspace | Low |

**Implementation Notes:**
- Filters should be UI-togglable in Convert module
- Add filter settings panel with tune options
- Show filter preview before encoding

---

## 2.2 Frame Cropping 🔴 HIGH PRIORITY

### Design Document: `docs/CROP_DESIGN.md`

| Feature | Other Tools | VideoTools | Priority |
|---------|-----------|------------|----------|
| Auto crop (black bars) | ✅ | 🔄 Partial | High |
| Manual crop | ✅ | ❌ | High |
| Aspect ratio overlay | ❌ | ❌ | **New feature** |
| Draggable crop region | ❌ | ❌ | **New feature** |

**Implementation Notes:**
- Use `cropdetect` filter to auto-detect black bars
- Add manual crop UI with visual selector
- **White outline overlay** showing output aspect ratio
- **Draggable crop region** for precise positioning
- Store crop settings in convert config

---

## 2.3 Batch Queue Add 🔴 HIGH PRIORITY

### Design Document: `docs/BATCH_QUEUE_DESIGN.md`

| Feature | Other Tools | VideoTools | Priority |
|---------|-----------|------------|----------|
| Add all titles from folder | ✅ | ❌ | High |
| Add all titles from disc | ✅ | ❌ | High |

---

## 2.2 Frame Cropping 🔴 HIGH PRIORITY

| Feature | HandBrake | VideoTools | FFmpeg Filter | Priority |
|---------|-----------|------------|---------------|----------|
| Auto crop (black bars) | ✅ | 🔄 Partial | cropdetect | High |
| Manual crop | ✅ | ❌ | crop=w:h:x:y | High |
| Crop presets | ✅ | ❌ | — | Medium |

**Implementation Notes:**
- Use `cropdetect` filter to auto-detect black bars
- Add manual crop UI with visual selector
- Store crop settings in convert config

---

## 2.3 Batch Queue Add 🔴 HIGH PRIORITY

| Feature | Other Tools | VideoTools | Status | Priority |
|---------|-----------|------------|--------|----------|
| Add all titles from folder | ✅ | ❌ | — | High |
| Add all titles from disc | ✅ | ❌ | — | High |
| Filter by format | ✅ | ❌ | — | Medium |

**Implementation Notes:**
- File manager already supports multi-select
- Integrate with queue system to add batch
- Show preview before adding

---

## 2.4 Custom Presets 🟡 MEDIUM PRIORITY

| Feature | Other Tools | VideoTools | Status | Priority |
|---------|-----------|------------|--------|----------|
| Save current settings as preset | ✅ | ❌ | — | Medium |
| Load saved preset | ✅ | ❌ | — | Medium |
| Export preset | ✅ | ❌ | — | Low |
| Import preset | ✅ | ❌ | — | Low |
| Delete preset | ✅ | ❌ | — | Low |
| Default preset | ✅ | ❌ | — | Low |

**Implementation Notes:**
- Save presets to JSON in user config directory
- Include all convert settings (codec, quality, filters, etc.)
- Allow importing/exporting preset files

---

## 2.5 Subtitle Handling 🟡 MEDIUM PRIORITY

| Feature | Other Tools | VideoTools | Status | Priority |
|---------|-----------|------------|--------|----------|
| Subtitle passthrough | ✅ | ✅ | ✅ | — |
| Burn-in specific tracks | ✅ | 🔄 Partial | — | Medium |
| Track selection rules | ✅ | ❌ | — | Medium |
| Forced subtitles | ✅ | 🔄 Partial | — | Medium |

**Implementation Notes:**
- Add subtitle track selector in UI
- Allow selecting which tracks to burn vs passthrough
- Handle forced subtitle detection

---

## 2.6 Video Tunes 🟢 LOW PRIORITY

| Tune | Other Tools | VideoTools | FFmpeg | Priority |
|------|-----------|------------|--------|----------|
| Film | ✅ | ❌ | tune=film | Low |
| Animation | ✅ | ❌ | tune=animation | Low |
| Grain | ✅ | ❌ | tune=grain | Low |
| Stillimage | ✅ | ❌ | tune=stillimage | Low |
| PSNR | ✅ | ❌ | tune=psnr | Low |
| SSIM | ✅ | ❌ | tune=ssim | Low |

---

## 2.7 Point-to-Point Encoding 🟢 LOW PRIORITY

| Feature | Other Tools | VideoTools | Priority |
|---------|-----------|------------|----------|
| Start/End time selection | ✅ | 🔄 Partial (Trim) | Low |
| Chapter selection | ✅ | 🔄 Partial | Low |

---

## 2.8 Metadata Passthrough 🟢 LOW PRIORITY

| Feature | Other Tools | VideoTools | Priority |
|---------|-----------|------------|----------|
| Title | ✅ | ❌ | Low |
| Artist/Album | ✅ | ❌ | Low |
| Cover art | ✅ | ❌ | Low |
| Comments | ✅ | ❌ | Low |

---

# Part 3: VideoTools Unique Strengths

## 3.1 Already Superior

| Feature | VideoTools | Advantage |
|---------|------------|------------|
| DVD Authoring | Native Go | No external dependencies |
| Disc Ripping | CSS decryption | More disc format support |
| AI Upscaling | Built-in | Unique feature |
| Native Player | Built-in Fyne | Better integration |
| Multi-platform | Linux-first | Native Linux support |

## 3.2 Opportunities to Exceed

| Opportunity | How | Difficulty |
|-------------|-----|-------------|
| Real-time preview | Show filter effects before encode | Medium |
| Batch rename | Regex-powered file renaming | Low |
| Folder watching | Auto-encode new files | Medium |
| Template-based naming | Use tokens in output names | Low |
| Module chaining | Pipe output between modules | High |

---

# Part 4: Implementation Priority

## Immediate (dev41)

1. **Video Filters** — `docs/FILTER_INTEGRATION_DESIGN.md`
2. **Frame Cropping** — Auto crop + manual crop UI
3. **Batch Queue** — Add all from folder

## Short-term (dev42-43)

4. **Custom Presets** — Save/load user presets
5. **Subtitle burn-in** — Track selection UI

## Medium-term (dev44+)

6. **Video Tunes** — Film, animation, grain
7. **Queue Edit** — Modify queued jobs
8. **Metadata** — Title, cover art passthrough

---

# Appendix: Related Documents

| Document | Description |
|----------|-------------|
| `docs/FILTER_INTEGRATION_DESIGN.md` | Video filter implementation |
| `docs/BURN_MODULE_DESIGN.md` | Disc burning |
| `docs/DVD_COMPLETION_PLAN.md` | DVD authoring roadmap |
| `docs/AUTO_GREY_CODECS.md` | Codec compatibility |

---

*Last updated: 2026-04-09*
*Version: 1.0*