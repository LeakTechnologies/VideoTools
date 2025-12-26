# Author Module - Complete Guide

## Overview

The Author module creates DVD/Blu-ray compliant disc structures from video files. It handles encoding, chapter management, and ISO generation with automatic timestamp correction for maximum compatibility.

---

## Quick Start

### Single File to ISO

1. **Select Files Tab** → Click "Select File" → Choose your video
2. **Settings Tab** → Configure output settings:
   - Output Type: DVD or Blu-ray
   - Region: NTSC (29.97fps) or PAL (25fps)
   - Aspect Ratio: 4:3 or 16:9
   - Disc Size: DVD5 (4.7GB) or DVD9 (8.5GB)
3. **Generate Tab** → Click "Generate DVD/ISO"
4. Output will be VIDEO_TS folder or .iso file

### Multiple Files to Single Disc

1. **Clips Tab** → Click "Add Video" → Add multiple files
2. Enable "Treat as Chapters" to merge into single title
3. **Settings Tab** → Configure as above
4. **Generate Tab** → Create multi-title or chaptered disc

---

## Features

### Automatic Chapter Detection

Detects scene changes and creates chapter markers automatically.

**How It Works:**
1. Load video in Files or Clips tab
2. **Chapters Tab** → Adjust "Detection Sensitivity" slider
   - Lower (0.1-0.2): Detects subtle changes, many chapters
   - Medium (0.3-0.4): Balanced, good for most videos
   - Higher (0.5-0.7): Only major scene changes
3. Click "Detect Scenes"
4. **Visual Preview:**
   - Thumbnail grid shows first 24 detected chapters
   - Each thumbnail displays frame at chapter timestamp
   - Verify detection quality visually
5. Click "Accept Chapters" to use, or "Reject" to try different sensitivity

**Technical Details:**
- Uses FFmpeg scene detection filter: `select='gt(scene,threshold)'`
- Threshold 0.30 = scene score must exceed 30% for detection
- Thumbnails extracted at 320x180, stored in temp directory
- Preview limited to 24 chapters for UI performance

**Use Cases:**
- Movies: Detect major scene changes (use 0.5-0.6)
- Documentaries: Detect segment breaks (use 0.4-0.5)
- TV Shows: Detect act breaks (use 0.3-0.4)
- Music Videos: Detect shot changes (use 0.2-0.3)

### Chapter Management

**Load Embedded Chapters:**
- Click "Load Embedded" to extract chapters from MKV/MP4 files
- Preserves original chapter titles and timestamps

**Manual Chapter Addition:**
- Click "+ Add Chapter" to insert custom chapter point
- Set timestamp and title manually

**Export Chapters:**
- Click "Export Chapters" to save chapter list
- Formats: XML, OGM, or text file

### DVD Timestamp Correction

**Problem Solved:**
DVD authoring tools (dvdauthor) require perfectly sequential MPEG-2 timestamps. FFmpeg direct encoding sometimes produces backwards-moving SCR (System Clock Reference) values, causing errors:
```
ERR: SCR moves backwards at inoffset 0, remultiplex input
```

**Solution:**
VideoTools automatically remultiplexes encoded MPEG files:
```bash
ffmpeg -fflags +genpts -i encoded.mpg -c copy -f dvd output.mpg
```
- `-fflags +genpts`: Regenerate presentation timestamps
- `-c copy`: No re-encoding (fast, no quality loss)
- `-f dvd`: Ensure DVD format compliance

**Result:**
- 100% dvdauthor compatibility
- Fixes SCR errors on AVI, MKV, MP4 sources
- Adds ~5 seconds per title (negligible)

---

## UI Features

### Authoring Log Viewer

Real-time log display during DVD generation.

**Performance Optimizations:**
- Displays last 100 lines only (tail behavior)
- Prevents memory bloat on long encodes
- Full log written to file for reference

**Controls:**
- **Copy Log**: Copies full log from file to clipboard
- **View Full Log**: Opens complete log in dedicated viewer
- Shows "Authoring Log (last 100 lines)" label

**Technical:**
- Circular buffer maintains fixed 100-line limit
- O(1) memory usage vs O(n) unbounded growth
- Rebuilds display text from buffer on each append
- Log file path tracked for full viewing

### Progress Tracking

- Real-time progress bar (0-100%)
- Status messages: "Encoding 1/3", "Authoring DVD structure", etc.
- Current operation displayed above progress bar

---

## Output Formats

### VIDEO_TS Folder Structure
```
output/
  VIDEO_TS/
    VIDEO_TS.IFO       # DVD table of contents
    VIDEO_TS.VOB       # Menu video (if applicable)
    VTS_01_0.IFO       # Title 1 information
    VTS_01_1.VOB       # Title 1 video data
    VTS_02_0.IFO       # Title 2 information (if multiple)
    VTS_02_1.VOB       # Title 2 video data
  AUDIO_TS/            # Empty (required by DVD spec)
```

### ISO Image
- Single .iso file containing complete VIDEO_TS structure
- Burnable to DVD-R/DVD-RW
- Mountable in virtual drives
- Playable in VLC, Kodi, standalone players

---

## Configuration Options

### Output Type
- **DVD**: DVD-Video format (MPEG-2, AC-3 audio)
- **Blu-ray**: Not yet implemented

### Region
- **NTSC**: 29.97fps, 720x480, North America/Japan
- **PAL**: 25fps, 720x576, Europe/Asia/Africa

### Aspect Ratio
- **4:3**: Standard (old TV format)
- **16:9**: Widescreen (modern format)
- **AUTO**: Detect from source video

### Disc Size
- **DVD5**: Single-layer, 4.7GB capacity
- **DVD9**: Dual-layer, 8.5GB capacity

### Create Menu
- Enable to generate simple title selection menu
- Requires additional processing time
- Menu background uses first frame of first title

### Treat as Chapters
- Merge multiple clips into single title with chapter points
- Chapter timestamps auto-calculated from clip durations
- Useful for episodic content or multi-part videos

---

## Encoding Settings

### Video
- Codec: MPEG-2 Video
- Resolution: 720x480 (NTSC) or 720x576 (PAL)
- Bitrate: 6000kbps (NTSC) / 8000kbps (PAL)
- Max Bitrate: 9000kbps (NTSC) / 9500kbps (PAL)
- GOP Size: 15 frames
- Pixel Format: YUV 4:2:0
- Interlacing: Enabled with ILME+ILDCT flags

### Audio
- Codec: AC-3 (Dolby Digital)
- Bitrate: 192kbps
- Sample Rate: 48kHz
- Channels: Stereo (2.0)

### Filters
- Scale to target resolution with aspect preservation
- Pad with black bars if needed to maintain aspect
- Force constant frame rate (CFR)

---

## Workflow Examples

### Example 1: Single Movie to DVD

**Scenario:** Convert vacation.mp4 to playable DVD

1. Author Module → Files Tab
2. Click "Select File" → Choose vacation.mp4
3. Settings Tab:
   - Output Type: DVD
   - Region: NTSC
   - Aspect Ratio: 16:9
   - Disc Size: DVD5
4. Chapters Tab:
   - Adjust sensitivity to 0.40
   - Click "Detect Scenes"
   - Preview thumbnails, verify chapter points
   - Accept chapters
5. Generate Tab:
   - Title: "Family Vacation"
   - Output Path: ~/Videos/vacation.iso
   - Click "Generate DVD/ISO"
6. Monitor progress in authoring log
7. Burn vacation.iso to DVD-R

**Result:** Playable DVD with auto-detected chapters

### Example 2: TV Series Disc (Multi-Title)

**Scenario:** Combine 3 episodes onto one DVD

1. Author Module → Clips Tab
2. Add Videos:
   - episode1.mkv
   - episode2.mkv
   - episode3.mkv
3. **Do NOT** enable "Treat as Chapters"
4. Settings Tab:
   - Output Type: DVD
   - Region: NTSC
   - Create Menu: Yes
5. Generate Tab:
   - Title: "Series S01"
   - Output Path: ~/Videos/series_s01.iso
   - Generate
6. Result: 3 separate titles with menu selection

### Example 3: Concert with Manual Chapters

**Scenario:** Concert video with chapters for each song

1. Author Module → Files Tab
2. Select concert.mp4
3. Chapters Tab:
   - Manual chapter addition:
     - 00:00 - "Opening"
     - 03:45 - "Song 1"
     - 07:30 - "Song 2"
     - etc.
4. Settings Tab → Configure as DVD
5. Generate → Create ISO with song navigation

---

## Troubleshooting

### "SCR moves backwards" Error

**Cause:** MPEG-2 timestamps not DVD-compliant

**Solution:** Automatic as of latest version
- Remux step added after encoding
- Regenerates timestamps with `-fflags +genpts`
- Error should not occur anymore

**If Still Occurring:**
- Verify you're using latest VideoTools build
- Check if source video has corruption
- Try re-downloading source file

### Chapter Detection Finds Too Many Chapters

**Cause:** Sensitivity too low (detects minor changes)

**Solution:**
- Increase sensitivity slider (0.5-0.7)
- Re-run "Detect Scenes"
- Preview thumbnails to verify
- Aim for 8-15 chapters per 90-minute video

### Chapter Detection Finds Too Few Chapters

**Cause:** Sensitivity too high (only detects major changes)

**Solution:**
- Decrease sensitivity slider (0.2-0.3)
- Re-run detection
- Verify with visual preview

### Authoring Log Causes UI Lag

**Cause:** Large log files (thousands of lines)

**Fixed:** As of latest version
- Only last 100 lines displayed in UI
- Full log accessible via "View Full Log" button
- Memory usage capped

**If Still Slow:**
- Click "View Full Log" to open in separate viewer
- Main UI will remain responsive

### ISO File Won't Burn to Disc

**Cause:** File size exceeds disc capacity

**Solution:**
- Check ISO size: Should be <4.7GB for DVD5
- If larger, use DVD9 disc or split content
- Reduce video bitrate in Settings if needed

### Video Playback Stutters on DVD Player

**Cause:** Framerate conversion or interlacing issues

**Solution:**
- Verify source video is Constant Frame Rate (CFR)
- If Variable Frame Rate (VFR), convert to CFR first
- Try enabling/disabling interlacing flags

---

## Technical Implementation

### Encoding Pipeline

1. **Source Analysis**
   - Probe video with ffprobe
   - Detect resolution, framerate, codec
   - Identify interlacing mode

2. **MPEG-2 Encoding**
   ```bash
   ffmpeg -i input.mp4 \
     -vf "scale=720:480,setdar=16:9,setsar=1,fps=30000/1001" \
     -c:v mpeg2video -r 30000/1001 \
     -b:v 6000k -maxrate 9000k -bufsize 1835k \
     -g 15 -pix_fmt yuv420p -flags +ilme+ildct \
     -c:a ac3 -b:a 192k -ar 48000 -ac 2 \
     output.mpg
   ```

3. **Timestamp Remux**
   ```bash
   ffmpeg -fflags +genpts -i output.mpg \
     -c copy -f dvd output_remux.mpg
   ```

4. **DVD Structure Creation**
   - Generate dvdauthor XML with chapter markers
   - Run: `dvdauthor -o VIDEO_TS -x dvd.xml`
   - Finalize: `dvdauthor -o VIDEO_TS -T`
   - Create AUDIO_TS folder

5. **ISO Generation** (if requested)
   - Detect ISO tool: mkisofs, genisoimage, or xorriso
   - Create ISO: `mkisofs -dvd-video -o output.iso VIDEO_TS/`

### Chapter XML Format

```xml
<dvdauthor>
  <vmgm />
  <titleset>
    <titles>
      <video format="ntsc" aspect="16:9" />
      <pgc>
        <vob file="title_01.mpg" chapters="0:00,3:45.5,7:30.2" />
      </pgc>
    </titles>
  </titleset>
</dvdauthor>
```

### Temporary Files

**Location:** System temp + VideoTools prefix
- `videotools-author-XXXXXX/`: Encoding workspace
  - `title_01.mpg`: Initial encode
  - `title_01_remux.mpg`: Timestamp-corrected version
  - `dvd.xml`: DVDAuthor configuration
- `videotools-dvd-XXXXXX/`: VIDEO_TS structure (if making ISO)
- `videotools-chapter-thumbs/`: Chapter preview thumbnails

**Cleanup:** Automatic on completion or error

---

## Dependencies

### Required
- **ffmpeg**: Video encoding and analysis
- **dvdauthor**: DVD structure creation

### Optional
- **mkisofs** or **genisoimage** or **xorriso**: ISO generation
- Only needed if creating .iso files (not needed for VIDEO_TS folders)

### Installation

**Fedora/RHEL:**
```bash
sudo dnf install ffmpeg dvdauthor genisoimage
```

**Ubuntu/Debian:**
```bash
sudo apt install ffmpeg dvdauthor genisoimage
```

**Arch:**
```bash
sudo pacman -S ffmpeg dvdauthor cdrtools
```

---

## Performance Tips

1. **Use SSD for Output**: Faster read/write during authoring
2. **Limit Concurrent Jobs**: DVD encoding is CPU-intensive
3. **Preview Before Full Encode**: Test 5-minute segment first
4. **Batch Similar Files**: Use same settings for multiple encodes
5. **Monitor Log File Size**: Large logs indicate potential issues

---

## File Size Estimates

### NTSC (6000kbps video + 192kbps audio)
- 30 minutes: ~1.3 GB
- 60 minutes: ~2.6 GB
- 90 minutes: ~3.9 GB (fits DVD5)
- 120 minutes: ~5.2 GB (requires DVD9)

### PAL (8000kbps video + 192kbps audio)
- 30 minutes: ~1.7 GB
- 60 minutes: ~3.4 GB
- 90 minutes: ~5.1 GB (requires DVD9)
- 120 minutes: ~6.8 GB (requires DVD9)

---

## Keyboard Shortcuts

None specific to Author module. Use main application shortcuts.

---

## Related Documentation

- **DVD_USER_GUIDE.md**: Creating DVD-compliant video files
- **QUEUE_SYSTEM_GUIDE.md**: Job queue management
- **MODULES.md**: Overview of all VideoTools modules
