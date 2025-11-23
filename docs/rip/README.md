# Rip Module

Extract and convert content from DVDs, Blu-rays, and disc images.

## Overview

The Rip module (formerly "Remux") handles extraction of video content from optical media and disc image files. It can rip directly from physical drives or work with ISO/IMG files, providing options for both lossless extraction and transcoding during the rip process.

> **Note:** This module is currently in planning phase. Features described below are proposed functionality.

## Features

### Source Support

#### Physical Media
- **DVD** - Standard DVDs with VOB structure
- **Blu-ray** - BD structure with M2TS files
- **CD** - Video CDs (VCD/SVCD)
- Direct drive access for ripping

#### Disc Images
- **ISO** - Standard disc image format
- **IMG** - Raw disc images
- **BIN/CUE** - CD image pairs
- Mount and extract without burning

### Title Selection

#### Auto-Detection
- Scan disc for all titles
- Identify main feature (longest title)
- List all extras/bonus content
- Show duration and chapter count for each

#### Manual Selection
- Preview titles before ripping
- Select multiple titles for batch rip
- Choose specific chapters from titles
- Merge chapters from different titles

### Track Management

#### Video Tracks
- Select video angle (for multi-angle DVDs)
- Choose video quality/stream

#### Audio Tracks
- List all audio tracks with language
- Select which tracks to include
- Reorder track priority
- Convert audio format during rip

#### Subtitle Tracks
- List all subtitle languages
- Extract or burn subtitles
- Select multiple subtitle tracks
- Convert subtitle formats

### Rip Modes

#### Direct Copy (Lossless)
Fast extraction with no quality loss:
- Copy VOB → MKV/MP4 container
- No re-encoding
- Preserves original quality
- Fastest option
- Larger file sizes

#### Transcode
Convert during extraction:
- Choose output codec (H.264, H.265, etc.)
- Set quality/bitrate
- Resize if desired
- Compress to smaller file
- Slower but more flexible

#### Smart Mode
Automatically choose best approach:
- Copy if already efficient codec
- Transcode if old/inefficient codec
- Optimize settings for content type

### Copy Protection Handling

#### DVD CSS
- Use libdvdcss when available
- Automatic decryption during rip
- Legal for personal use (varies by region)

#### Blu-ray AACS
- Use libaacs for AACS decryption
- Support for BD+ (limited)
- Requires key database

#### Region Codes
- Detect region restrictions
- Handle multi-region discs
- RPC-1 drive support

### Quality Settings

#### Presets
- **Archival** - Lossless or very high quality
- **Standard** - Good quality, moderate size
- **Efficient** - Smaller files, acceptable quality
- **Custom** - User-defined settings

#### Special Handling
- Deinterlace DVD content automatically
- Inverse telecine for film sources
- Upscale SD content to HD (optional)
- HDR passthrough for Blu-ray

### Batch Processing

#### Multiple Titles
- Queue all titles from disc
- Process sequentially
- Different settings per title
- Automatic naming

#### Multiple Discs
- Load multiple ISO files
- Batch rip entire series
- Consistent settings across discs
- Progress tracking

### Output Options

#### Naming Templates
Automatic file naming:
```
{disc_name}_Title{title_num}_Chapter{start}-{end}
Star_Wars_Title01_Chapter01-25.mp4
```

#### Metadata
- Auto-populate from disc info
- Lookup online databases (IMDB, TheTVDB)
- Chapter markers preserved
- Cover art extraction

#### Organization
- Create folder per disc
- Separate folders for extras
- Season/episode structure for TV
- Automatic file organization

## Usage Guide

### Ripping a DVD

1. **Insert Disc or Load ISO**
   - Physical disc: Insert and click "Scan Drive"
   - ISO file: Click "Load ISO" and select file

2. **Scan Disc**
   - Application analyzes disc structure
   - Lists all titles with duration/chapters
   - Main feature highlighted

3. **Select Title(s)**
   - Choose main feature or specific titles
   - Select desired chapters
   - Preview title information

4. **Configure Tracks**
   - Select audio tracks (e.g., English 5.1)
   - Choose subtitle tracks if desired
   - Set track order/defaults

5. **Choose Rip Mode**
   - Direct Copy for fastest/lossless
   - Transcode to save space
   - Configure quality settings

6. **Set Output**
   - Choose output folder
   - Set filename or use template
   - Select container format

7. **Start Rip**
   - Click "Start Ripping"
   - Monitor progress
   - Can queue multiple titles

### Ripping a Blu-ray

Similar to DVD but with additional considerations:
- Much larger files (20-40GB for feature)
- Better quality settings available
- HDR preservation options
- Multi-audio track handling

### Batch Ripping a TV Series

1. **Load all disc ISOs** for season
2. **Scan each disc** to identify episodes
3. **Enable batch mode**
4. **Configure naming** with episode numbers
5. **Set consistent quality** for all
6. **Start batch rip**

## FFmpeg Integration

### Direct Copy Example
```bash
# Extract VOB to MKV without re-encoding
ffmpeg -i /dev/dvd -map 0 -c copy output.mkv

# Extract specific title
ffmpeg -i dvd://1 -map 0 -c copy title_01.mkv
```

### Transcode Example
```bash
# Rip DVD with H.264 encoding
ffmpeg -i dvd://1 \
  -vf yadif,scale=720:480 \
  -c:v libx264 -crf 20 \
  -c:a aac -b:a 192k \
  output.mp4
```

### Multi-Track Example
```bash
# Preserve multiple audio and subtitle tracks
ffmpeg -i dvd://1 \
  -map 0:v:0 \
  -map 0:a:0 -map 0:a:1 \
  -map 0:s:0 -map 0:s:1 \
  -c copy output.mkv
```

## Tips & Best Practices

### DVD Quality
- Original DVD is 720×480 (NTSC) or 720×576 (PAL)
- Always deinterlace DVD content
- Consider upscaling to 1080p for modern displays
- Use inverse telecine for film sources (24fps)

### Blu-ray Handling
- Main feature typically 20-50GB
- Consider transcoding to H.265 to reduce size
- Preserve 1080p resolution
- Keep high bitrate audio (DTS-HD, TrueHD)

### File Size Management
| Source | Direct Copy | H.264 CRF 20 | H.265 CRF 24 |
|--------|-------------|--------------|--------------|
| DVD (2hr) | 4-8 GB | 2-4 GB | 1-3 GB |
| Blu-ray (2hr) | 30-50 GB | 6-10 GB | 4-6 GB |

### Legal Considerations
- Ripping for personal backup is legal in many regions
- Circumventing copy protection may have legal restrictions
- Distribution of ripped content is typically illegal
- Check local laws and regulations

### Drive Requirements
- DVD-ROM drive for DVD ripping
- Blu-ray drive for Blu-ray ripping (obviously)
- RPC-1 (region-free) firmware helpful
- External drives work fine

## Troubleshooting

### Can't Read Disc
- Clean disc surface
- Try different drive
- Check drive region code
- Verify disc isn't damaged

### Copy Protection Errors
- Install libdvdcss (DVD) or libaacs (Blu-ray)
- Update key database
- Check disc region compatibility
- Try different disc copy

### Slow Ripping
- Direct copy is fastest
- Transcoding is CPU-intensive
- Use hardware acceleration if available
- Check drive speed settings

### Audio/Video Sync Issues
- Common with VFR content
- Use -vsync parameter
- Force constant frame rate
- Check source for corruption

## See Also
- [Convert Module](../convert/) - Transcode ripped files further
- [Streams Module](../streams/) - Manage multi-track ripped files
- [Subtitle Module](../subtitle/) - Handle extracted subtitle tracks
- [Inspect Module](../inspect/) - Analyze ripped output quality
