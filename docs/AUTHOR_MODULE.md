# Author Module Guide

## What Does This Do?

The Author module turns your video files into DVDs that'll play in any DVD player - the kind you'd hook up to a TV. It handles all the technical stuff so you don't have to worry about it.

---

## Getting Started

### Making a Single Disc

1. Click **Author** from the main menu
2. **Files Tab** → Click "Add Files" → Pick your video
3. **Settings Tab**:
   - **Target Type**: DVD or Blu-ray
   - **Region/Standard**: NTSC/PAL (DVD) or 1080p/4K (Blu-ray)
   - **Aspect Ratio**: 16:9 or 4:3
4. **Generate Tab** → Click "Generate Disc/ISO"
5. Wait for it to finish, then burn the .iso file to a blank disc.

That's it. The disc will play in any standard player.

---

## Content Types: Feature, Extras, Galleries

The Author module treats every import as a **content type**, not just a file:

- **Feature**: the main movie title (supports chapters and chapter menus)
- **Extra**: bonus video titles (no chapters, separate titles)
- **Gallery**: still-image slideshows (photos, artwork, stills)

### Default Behavior

- All imported videos default to **Feature**
- You can change each video’s **Content Type** using the per-item toggle.

---

## Understanding the Settings

### Output Type

**DVD** - Standard Definition (480i/576i). Compatible with all DVD and Blu-ray players.
**Blu-ray** - High Definition (1080p) or Ultra HD (4K). Requires a Blu-ray player.

### Region / Standard

**For DVD**:
- **NTSC**: North America, Japan (30 fps)
- **PAL**: Europe, Australia (25 fps)

**For Blu-ray**:
- **1080p**: Standard High Definition
- **4K UHD**: Ultra High Definition with HDR support

Pick based on your source quality and target player.

### Aspect Ratio

**16:9** - Widescreen (Modern TV).
**4:3** - Standard (Old TV).
**AUTO** - Match source aspect ratio.

### Disc Size

**DVD5 / DVD9**: 4.7GB or 8.5GB (Standard DVD)
**BD25 / BD50**: 25GB or 50GB (Standard Blu-ray)
**BD66 / BD100**: 66GB or 100GB (4K UHD Blu-ray)

---

## What's Happening Behind the Scenes?

VideoTools uses a **Native Go Authoring Engine** to build your discs without external Linux-only tools like `dvdauthor` or `xorriso`.

### The Pipeline

1. **Encoding**: Video is transcoded to MPEG-2 (DVD) or H.264/HEVC (Blu-ray).
2. **Muxing**: Video, audio, and menu assets are combined into standard-compliant streams.
3. **Structure**: The `VIDEO_TS` or `BDMV` directory structure is generated with all necessary metadata (IFO/MPLS).
4. **Mastering**: A UDF 1.02 (DVD) or UDF 2.50 (Blu-ray) disc image (.iso) is created natively.
