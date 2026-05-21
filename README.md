# VideoTools

Video processing suite with native DVD authoring, disc ripping, and a CGo/FFmpeg media engine.

Built for **Linux and Windows** — macOS is not supported.

## Capabilities

- **Convert** — H.264/H.265/AV1/ProRes/VP9, lossless remux, DVD-compliant MPEG-2, deinterlace, normalize, crop, preset system
- **Rip** — DVD/ISO extraction via FFmpeg dvdvideo demuxer or VOB concat; IFO-based title/chapter/audio/subtitle scanning; region detection; PAL↔NTSC full-disc conversion with IFO regeneration; menu preservation
- **Author** — Native DVD-Video authoring: M1–M7 menu system, 4 menu resolutions (720→1920px), button highlights, multiple audio/subtitle streams, VOB muxer, UDF writer
- **Burn** — Multi-drive disc burning via xorriso/isoburn
- **Queue** — Batch job processing with progress tracking per module
- **Filters** — Real-time video filter chain (crop, scale, deinterlace, denoise, color, etc.)
- **Audio** — Track selection, language tagging, extraction
- **Subtitles** — Bitmap subtitle support, pass-through
- **Trim** — Frame-accurate cutting with preview
- **Thumbnail** — Grid/3-way output with image inspector
- **Compare** — Side-by-side before/after with independent engines
- **Upscale** — AI-assisted upscaling (Real-ESRGAN, waifu2x, etc.)
- **Inspect** — Codec, format, chapter, metadata analysis
- **Settings** — Language (en-CA, fr-CA, Inuktitut), HW decode, theme

## Platform Targets

| Platform | Status | Notes |
|----------|--------|-------|
| Linux x86_64 | Primary dev target | FFmpeg built from source with x264/x265 |
| Windows x86_64 | Primary user target | BtbN FFmpeg DLL bundle, isoburn for burning |
| macOS | Not supported | No darwin code paths in tree |

## Project Status

Under active development (`v0.1.1-dev49`). See **[Interactive Roadmap](docs/roadmap.html)** for module-by-module status with changelog, checklist, and card-level detail.

- **Dev builds:** https://git.leaktechnologies.dev/Leak_Technologies/VideoTools
- **Public releases:** https://github.com/LeakTechnologies/VideoTools

## Output Formats

| Format | Video | Audio | Container |
|--------|-------|-------|-----------|
| H.264 MKV | libx264 (CRF 18) | copy / aac | MKV |
| H.264 MP4 | libx264 (CRF 18) | aac | MP4 |
| H.265 MKV | libx265 | copy / aac | MKV |
| AV1 MKV | libaom-av1 / SVT-AV1 | copy / aac | MKV |
| ProRes MOV | prores_ks | pcm_s16le | MOV |
| VP9 WebM | libvpx-vp9 | libopus | WebM |
| Lossless MKV | stream copy | stream copy | MKV (remux) |
| DVD-NTSC | mpeg2video (CBR 6 Mbps) | ac3 (192k) | MPG |
| DVD-PAL | mpeg2video (CBR 6 Mbps) | ac3 (192k) | MPG |

## Quick Start

### Linux

```bash
bash scripts/linux/install.sh
source ~/.bashrc
VideoTools
```

### Windows (PowerShell)

```powershell
.\scripts\windows\install.ps1
```

For more detail: **docs/INSTALL_LINUX.md** · **docs/INSTALL_WINDOWS.md** · **docs/INSTALLATION.md**

## DVD Workflow

### Rip a Disc (Rip module)

1. Insert a DVD or load an ISO → **Rip** tab
2. Click **Start Scan** → IFO parsing shows titles, chapters, audio/subtitle tracks
3. Select titles to rip (menus excluded by default)
4. Choose output path → **Add to Queue**
5. **Start Queue** → FFmpeg dvdvideo or VOB concat extracts each title
6. Output: H.264 MKV per title (or MPEG-2 MPG)

### Author a DVD (Author module)

1. Encode source video as DVD-NTSC/PAL MPEG-2 in **Convert**
2. Switch to **Author** tab → add the encoded stream
3. Select menu template (M1–M7): choose backdrop, colours, layout
4. Set audio/subtitle languages
5. **Build** → creates VIDEO_TS structure with IFO/BUP/VOB
6. **Burn** → write to disc via the Burn module

## Documentation

| Document | What it covers |
|----------|----------------|
| **docs/roadmap.html** | Interactive roadmap with changelog, testing checklist, card-by-card status |
| **docs/ROADMAP.md** | Mermaid timeline, current state, now/next |
| **docs/INSTALLATION.md** | Comprehensive install guide (read first) |
| **docs/INSTALL_LINUX.md** | Linux-specific setup |
| **docs/INSTALL_WINDOWS.md** | Windows-specific setup |
| **docs/NATIVE_PLAYER.md** | Native CGo/FFmpeg media engine architecture |
| **docs/REFACTOR_DEV30_PLAN.md** | Long-term refactor plan (root → internal/) |
| **docs/RIP_MODULE_REDESIGN.md** | Rip module design and fixes |
| **docs/BURN_MODULE_DESIGN.md** | Burn module design |
| **docs/CHANGELOG.md** | Per-release changelog |
| **docs/localization-policy.md** | i18n strategy (en-CA, fr-CA, Inuktitut) |
| **docs/PLAYER_DEBUG.md** | Player crash diagnostics log |

## Localization

VideoTools ships with built-in support for multiple languages. Switch language in **Settings → General**.
Aboriginal Sans is embedded for proper rendering of Unified Canadian Aboriginal Syllabics (Inuktitut).

| Language | Code | Script | Coverage |
|---|---|---|---|
| English (Canada) | `en-CA` | Latin | 100% — source of truth |
| French (Canada) | `fr-CA` | Latin | 98% |
| Inuktitut | `iu` | Syllabics + Latin toggle | 11% — in progress |

> Coverage is measured against the en-CA field count. The `iu` translation is an ongoing community effort; untranslated strings fall back to English automatically.

> The Inuktitut syllabics↔roman transliteration algorithm is derived from the [iutools](https://github.com/NationalResearchCouncilCanada/iutools) project by the National Research Council Canada (MIT license). Empty script variants are auto-filled via transliteration; manually-entered strings take precedence.

## Requirements

**Runtime:**
- FFmpeg (auto-bundled on Windows via DLLs; Linux built from source in CI)
- Linux: X11 or Wayland display server
- Windows: Windows 10/11

**Build:**
- Go 1.21+
- MinGW-w64 (Windows) or GCC (Linux) — required for native_media CGo code
- pkg-config (Linux)
- nasm, cmake, git (for FFmpeg/x264/x265 source builds)
- Resource compiler (Windows): x86_64-w64-mingw32-windres

## System Architecture

```
cmd/videotools/          → Executable entrypoint (future)
internal/media/          → CGo/FFmpeg engine: demux, decode, audio (oto v3), HW accel
internal/media/          → VideoPlayer Fyne widget: frame rendering, control overlay, thumbnails
internal/ui/             → UI components, InlineVideoPlayer API layer
internal/theme/          → VT_Navy palette, PillButton/PillIconButton, text primitives
internal/i18n/           → Localization (en-CA, fr-CA, iu, iu-Latn)
internal/dvd/            → IFO parsing, UDF reader, CSS decrypt, IFO regeneration
internal/convert/        → Presets, FFmpeg pipeline config
internal/queue/          → Batch job queue with progress
internal/app/modules/    → All feature modules (convert, rip, author, burn, queue, etc.)
internal/app/            → App wiring (module registry, settings, config)
scripts/                 → Build/install automation (Linux + Windows)
```

## Build & Run

```bash
# One-time setup (Linux)
source scripts/alias.sh

# Run
VideoTools

# Rebuild
VideoToolsRebuild
```

**Windows:**
```powershell
.\scripts\windows\install.ps1
```

**Direct (any):**
```bash
go build -tags native_media -o VideoTools .
./VideoTools
VIDEOTOOLS_DEBUG=1 ./VideoTools
```

## Troubleshooting

- **BUILD_AND_RUN.md** — setup troubleshooting
- **videotools.log** — error details
- **VIDEOTOOLS_DEBUG=1** — verbose logging
- **docs/PLAYER_DEBUG.md** — player crash diagnostics
- **docs/INSTALL_LINUX.md** / **docs/INSTALL_WINDOWS.md** — platform-specific issues

## Getting Help

1. **docs/roadmap.html** — feature status and testing checklist
2. **docs/INSTALLATION.md** — setup guide
3. **docs/ROADMAP.md** — planned work
4. **docs/CHANGELOG.md** — release history
5. **Issue tracker:** https://git.leaktechnologies.dev/leak_technologies/VideoTools/issues

## Credits

VideoTools directly depends on and gratefully acknowledges these projects:

| Project | Used for | License |
|---|---|---|
| [FFmpeg](https://ffmpeg.org) | All media encode/decode/filter/mux operations | LGPL/GPL |
| [Fyne](https://fyne.io) | Cross-platform GUI toolkit | BSD-3 |
| [oto](https://github.com/hajimehoshi/oto) | Audio playback engine | Apache-2.0 |
| [x264](https://www.videolan.org/developers/x264.html) | H.264 encoding (static build, from source) | GPL-2 |
| [x265](https://www.videolan.org/developers/x265.html) | H.265/HEVC encoding (static build, from source) | GPL-2 |
| [iutools](https://github.com/NationalResearchCouncilCanada/iutools) | Inuktitut syllabics↔roman transliteration algorithm | MIT |
| [Aboriginal Sans](http://www.languagegeek.com/fonts.html) | Inuktitut syllabics font rendering | OFL |
| [Real-ESRGAN](https://github.com/xinntao/Real-ESRGAN) / [ncnn](https://github.com/nihui/real-esrgan-ncnn-vulkan) | AI upscaling engine | BSD-3 |
| [RIFE](https://github.com/nihui/rife-ncnn-vulkan) | Frame interpolation (ncnn port) | MIT |
| [Real-CUGAN](https://github.com/nihui/realcugan-ncnn-vulkan) | Anime upscaling (ncnn port) | MIT |
| [BtbN/FFmpeg-Builds](https://github.com/BtbN/FFmpeg-Builds) | Windows shared DLL CI bundle | LGPL/GPL |
| [linuxdeploy](https://github.com/linuxdeploy/linuxdeploy) + [AppImageKit](https://github.com/AppImage/AppImageKit) | Linux AppImage packaging | GPL-2 / MIT |
| [SignPath](https://about.signpath.io) / [SignPath.io](https://signpath.io) | Windows code signing | — |
