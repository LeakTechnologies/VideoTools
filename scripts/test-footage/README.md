# Test Footage Scripts

Create test footage in multiple formats for VideoTools player testing.

## Purpose

When debugging video playback issues, it's helpful to test against the same content in multiple formats. This helps identify codec-specific bugs vs general player issues.

## Usage

### Linux/macOS (bash)

```bash
cd scripts/test-footage
chmod +x create-test-footage.sh
./create-test-footage.sh /path/to/source/video.mp4 [duration]
```

### Windows (CMD/PowerShell)

```cmd
cd scripts\test-footage
create-test-footage.bat C:\path\to\source\video.mp4 [duration]
```

### Windows (PowerShell)

```powershell
.\create-test-footage.bat "C:\path\to\source\video.mp4" 10
```

## Output

Creates a directory `test-footage-YYYYMMDD-HHMMSS/` with:

| File | Format | Container | Notes |
|------|--------|----------|-------|
| `*_h264.mp4` | H.264 | MP4 | Most common web format |
| `*_h265.mp4` | H.265/HEVC | MP4 | Efficient but less supported |
| `*_vp9.webm` | VP9 | WebM | WebM standard |
| `*_mpeg4.avi` | MPEG-4 | AVI | Legacy format |
| `*_vp8.webm` | VP8 | WebM | Older WebM |
| `*_av1.mkv` | AV1 | MKV | Newest codec |
| `*_h264.mov` | H.264 | MOV | QuickTime format |
| `*_prores422.mov` | ProRes 422 | MOV | High quality reference |

## Testing Workflow

1. Create test footage from a known-good source
2. Load each format in VideoTools player
3. Note any failures or visual artifacts
4. Compare behavior across formats to identify codec vs general issues

## Notes

- AV1 encoding is slow - consider increasing `-cpu-used` for faster encode
- ProRes encode requires FFmpeg with prores_ks support
- Scripts require FFmpeg with all relevant codecs installed