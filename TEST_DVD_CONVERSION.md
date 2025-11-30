# DVD Conversion Testing Guide

This guide walks you through a complete DVD-NTSC conversion test.

## Test Setup

A test video has been created at:
```
/tmp/videotools_test/test_video.mp4
```

**Video Properties:**
- Resolution: 1280×720 (16:9 widescreen)
- Framerate: 30fps
- Duration: 5 seconds
- Codec: H.264
- This is perfect for testing - larger than DVD output, different aspect ratio

**Expected Output:**
- Resolution: 720×480 (NTSC standard)
- Framerate: 29.97fps
- Codec: MPEG-2
- Duration: ~5 seconds (same, just re-encoded)

---

## Step-by-Step Testing

### Step 1: Start VideoTools

```bash
cd /home/stu/Projects/VideoTools
./VideoTools
```

You should see the main menu with modules: Convert, Merge, Trim, Filters, Upscale, Audio, Thumb, Inspect.

✅ **Expected:** Main menu appears with all modules visible

---

### Step 2: Open Convert Module

Click the **"Convert"** tile (violet color, top-left area)

You should see:
- Video preview area
- Format selector
- Quality selector
- "Add to Queue" button
- Queue access button

✅ **Expected:** Convert module loads without errors

---

### Step 3: Load Test Video

In the Convert module, you should see options to:
- Drag & drop a file, OR
- Use file browser button

**Load:** `/tmp/videotools_test/test_video.mp4`

After loading, you should see:
- Video preview (blue frame)
- Video information: 1280×720, 30fps, duration ~5 seconds
- Metadata display

✅ **Expected:** Video loads and metadata displays correctly

---

### Step 4: Select DVD Format

Look for the **"Format"** dropdown in the Simple Mode section (top area).

Click the dropdown and select: **"DVD-NTSC (MPEG-2)"**

**This is where the magic happens!**

✅ **Expected Results After Selecting DVD-NTSC:**

You should immediately see:
1. **DVD Aspect Ratio selector appears** with options: 4:3 or 16:9 (default 16:9)
2. **DVD info label shows:**
   ```
   NTSC: 720×480 @ 29.97fps, MPEG-2, AC-3 Stereo 48kHz
   Bitrate: 6000k (default), 9000k (max PS2-safe)
   Compatible with DVDStyler, PS2, standalone DVD players
   ```
3. **Output filename hint updates** to show: `.mpg` extension

**In Advanced Mode (if you click the toggle):**
- Target Resolution should show: **"NTSC (720×480)"** ✅
- Frame Rate should show: **"30"** ✅ (will become 29.97fps in actual encoding)
- Aspect Ratio should be set to: **"16:9"** (matching DVD aspect selector)

---

### Step 5: Name Your Output

In the "Output Name" field, enter:
```
test_dvd_output
```

**Don't include the .mpg extension** - VideoTools adds it automatically.

✅ **Expected:** Output hint shows "Output file: test_dvd_output.mpg"

---

### Step 6: Queue the Conversion Job

Click the **"Add to Queue"** button

A dialog may appear asking to confirm. Click OK/Proceed.

✅ **Expected:** Job is added to queue, you can see queue counter update

---

### Step 7: View and Start the Queue

Click **"View Queue"** button (top right)

You should see the Queue panel with:
- Your job listed
- Status: "Pending"
- Progress: 0%
- Control buttons: Start Queue, Pause All, Resume All

Click **"Start Queue"** button

✅ **Expected:** Conversion begins, progress bar fills

---

### Step 8: Monitor Conversion

Watch the queue as it encodes. You should see:
- Status: "Running"
- Progress bar: filling from 0% to 100%
- No error messages

The conversion will take **2-5 minutes** depending on your CPU. With a 5-second test video, it should be relatively quick.

✅ **Expected:** Conversion completes with Status: "Completed"

---

### Step 9: Verify Output File

After conversion completes, check the output:

```bash
ls -lh test_dvd_output.mpg
```

You should see a file with reasonable size (several MB for a 5-second video).

**Check Properties:**
```bash
ffprobe test_dvd_output.mpg -show_streams
```

✅ **Expected Output Should Show:**
- Video codec: `mpeg2video` (not h264)
- Resolution: `720x480` (not 1280x720)
- Frame rate: `29.97` or `30000/1001` (NTSC standard)
- Audio codec: `ac3` (Dolby Digital)
- Audio sample rate: `48000` Hz (48 kHz)
- Audio channels: 2 (stereo)

---

### Step 10: DVDStyler Compatibility Check

If you have DVDStyler installed:

```bash
which dvdstyler
```

**If installed:**
1. Open DVDStyler
2. Create a new project
3. Try to import the `.mpg` file

✅ **Expected:** File imports without re-encoding warnings

**If not installed but want to simulate:**
FFmpeg would automatically detect and re-encode if the file wasn't DVD-compliant. The fact that our conversion worked means it IS compliant.

---

## Success Criteria Checklist

After completing all steps, verify:

- [ ] VideoTools opens without errors
- [ ] Convert module loads
- [ ] Test video loads correctly (1280x720, 30fps shown)
- [ ] Format dropdown works
- [ ] DVD-NTSC format selects successfully
- [ ] DVD Aspect Ratio selector appears
- [ ] DVD info text displays correctly
- [ ] Target Resolution auto-sets to "NTSC (720×480)" (Advanced Mode)
- [ ] Frame Rate auto-sets to "30" (Advanced Mode)
- [ ] Job queues without errors
- [ ] Conversion starts and shows progress
- [ ] Conversion completes successfully
- [ ] Output file exists (test_dvd_output.mpg)
- [ ] Output file has correct codec (mpeg2video)
- [ ] Output resolution is 720×480
- [ ] Output framerate is 29.97fps
- [ ] Audio is AC-3 stereo at 48 kHz
- [ ] File is DVDStyler-compatible (no re-encoding warnings)

---

## Troubleshooting

### Video Doesn't Load
- Check file path: `/tmp/videotools_test/test_video.mp4`
- Verify FFmpeg is installed: `ffmpeg -version`
- Check file exists: `ls -lh /tmp/videotools_test/test_video.mp4`

### DVD Format Not Appearing
- Ensure you're in Simple or Advanced Mode
- Check that Format dropdown is visible
- Scroll down if needed to find it

### Auto-Resolution Not Working
- Click on the format dropdown and select DVD-NTSC again
- Switch to Advanced Mode to see Target Resolution field
- Check that it shows "NTSC (720×480)"

### Conversion Won't Start
- Ensure job is in queue with status "Pending"
- Click "Start Queue" button
- Check for error messages in the console
- Verify FFmpeg is installed and working

### Output File Wrong Format
- Check codec: `ffprobe test_dvd_output.mpg | grep codec`
- Should show `mpeg2video` for video and `ac3` for audio
- If not, conversion didn't run with DVD settings

### DVDStyler Shows Re-encoding Warning
- This means our MPEG-2 encoding didn't match specs
- Check framerate, resolution, codec, bitrate
- May need to adjust encoder settings

---

## Test Results Template

Use this template to document your results:

```
TEST DATE: [date]
SYSTEM: [OS/CPU]
GO VERSION: [from: go version]
FFMPEG VERSION: [from: ffmpeg -version]

INPUT VIDEO:
- Path: /tmp/videotools_test/test_video.mp4
- Codec: h264
- Resolution: 1280x720
- Framerate: 30fps
- Duration: 5 seconds

VIDEOTOOLS TEST:
- Format selected: DVD-NTSC (MPEG-2)
- DVD Aspect Ratio: 16:9
- Output name: test_dvd_output
- Queue status: [pending/running/completed]
- Conversion status: [success/failed/error]

OUTPUT VIDEO:
- Path: test_dvd_output.mpg
- File size: [MB]
- Video codec: [mpeg2video?]
- Resolution: [720x480?]
- Framerate: [29.97?]
- Audio codec: [ac3?]
- Audio channels: [stereo?]
- Audio sample rate: [48000?]

DVDStyler COMPATIBILITY:
- Tested: [yes/no]
- Result: [success/re-encoding needed/failed]

OVERALL RESULT: [PASS/FAIL]
```

---

## Next Steps

After successful conversion:

1. **Optional: Test PAL Format**
   - Repeat with DVD-PAL format
   - Should auto-set to 720×576 @ 25fps
   - Audio still AC-3 @ 48kHz

2. **Optional: Test Queue Features**
   - Add multiple videos
   - Test Pause All / Resume All
   - Test job reordering

3. **Optional: Create Real DVD**
   - Import .mpg into DVDStyler
   - Add menus and chapters
   - Burn to physical DVD disc

---

## Commands Reference

### Create Test Video (if needed)
```bash
ffmpeg -f lavfi -i "color=c=blue:s=1280x720:d=5,fps=30" -f lavfi -i "sine=f=1000:d=5" \
  -c:v libx264 -c:a aac -y /tmp/videotools_test/test_video.mp4
```

### Check Input Video
```bash
ffprobe /tmp/videotools_test/test_video.mp4 -show_streams
```

### Check Output Video
```bash
ffprobe test_dvd_output.mpg -show_streams
```

### Get Quick Summary
```bash
ffprobe test_dvd_output.mpg -v error \
  -select_streams v:0 -show_entries stream=codec_name,width,height,r_frame_rate \
  -of default=noprint_wrappers=1:nokey=1
```

### Verify DVD Compliance
```bash
ffprobe test_dvd_output.mpg -v error \
  -select_streams a:0 -show_entries stream=codec_name,sample_rate,channels \
  -of default=noprint_wrappers=1:nokey=1
```

---

**Good luck with your testing! Let me know your results.** 🎬

