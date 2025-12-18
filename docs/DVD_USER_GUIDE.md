# VideoTools DVD Encoding - User Guide

## 🎬 Creating DVD-Compliant Videos

VideoTools now has full DVD encoding support built into the Convert module. Follow this guide to create professional DVD-Video files.

---

## 📝 Quick Start (5 minutes)

### Step 1: Load a Video
1. Click the **Convert** tile from the main menu
2. Drag and drop a video file, or use the file browser
3. VideoTools will analyze the video and show its specs

### Step 2: Select DVD Format
1. In the **OUTPUT** section, click the **Format** dropdown
2. Choose either:
   - **DVD-NTSC (MPEG-2)** - For USA, Canada, Japan, Australia
   - **DVD-PAL (MPEG-2)** - For Europe, Africa, Asia
3. DVD-specific options will appear below

### Step 3: Choose Aspect Ratio
1. When DVD format is selected, a **DVD Aspect Ratio** option appears
2. Choose **4:3** or **16:9** based on your video:
   - Use **16:9** for widescreen (most modern videos)
   - Use **4:3** for older/square footage

### Step 4: Set Output Name
1. In **Output Name**, enter your desired filename (without .mpg extension)
2. The system will automatically add **.mpg** extension
3. Example: `myvideo` → `myvideo.mpg`

### Step 5: Queue the Job
1. Click **Add to Queue**
2. Your DVD encoding job is added to the queue
3. Click **View Queue** to see all pending jobs
4. Click **Start Queue** to begin encoding

### Step 6: Monitor Progress
- The queue displays:
  - Job status (pending, running, completed)
  - Real-time progress percentage
  - Estimated remaining time
- You can pause, resume, or cancel jobs anytime

---

## 🎯 DVD Format Specifications

### DVD-NTSC (North America, Japan, Australia)
```
Resolution:      720 × 480 pixels
Frame Rate:      29.97 fps (NTSC standard)
Video Bitrate:   6000 kbps (default), max 9000 kbps
Audio:           AC-3 Stereo, 192 kbps, 48 kHz
Container:       MPEG Program Stream (.mpg)
Compatibility:   DVDStyler, PS2, standalone DVD players
```

**Best for:** Videos recorded in 29.97fps or 30fps (NTSC regions)

### DVD-PAL (Europe, Africa, Asia)
```
Resolution:      720 × 576 pixels
Frame Rate:      25.00 fps (PAL standard)
Video Bitrate:   8000 kbps (default), max 9500 kbps
Audio:           AC-3 Stereo, 192 kbps, 48 kHz
Container:       MPEG Program Stream (.mpg)
Compatibility:   DVDStyler, PAL DVD players, European authoring tools
```

**Best for:** Videos recorded in 25fps (PAL regions) or European distribution

---

## 🔍 Understanding the Validation Messages

When you add a video to the DVD queue, VideoTools validates it and shows helpful messages:

### ℹ️ Info Messages (Blue)
- **"Input resolution is 1920x1080, will scale to 720x480"**
  - Normal - Your video will be scaled to DVD size
  - Action: Aspect ratio will be preserved

- **"Input framerate is 30.0 fps, will convert to 29.97 fps"**
  - Normal - NTSC standard requires exactly 29.97 fps
  - Action: Will adjust slightly (imperceptible to viewers)

- **"Audio sample rate is 44.1 kHz, will resample to 48 kHz"**
  - Normal - DVD requires 48 kHz audio
  - Action: Audio will be automatically resampled

### ⚠️ Warning Messages (Yellow)
- **"Input framerate is 60.0 fps"**
  - Means: Your video has double the DVD framerate
  - Action: Every other frame will be dropped
  - Result: Video still plays normally (60fps drops to 29.97fps)

- **"Input is VFR (Variable Frame Rate)"**
  - Means: Framerate isn't consistent (unusual)
  - Action: Will force constant 29.97fps
  - Warning: May cause slight audio sync issues

### ❌ Error Messages (Red)
- **"Bitrate exceeds DVD maximum"**
  - Means: Encoding settings are too high quality
  - Action: Will automatically cap at 9000k (NTSC) or 9500k (PAL)
  - Result: Still produces high-quality output

---

## 🎨 Aspect Ratio Guide

### What is Aspect Ratio?
The ratio of width to height. Common formats:
- **16:9** (widescreen) - Modern TVs, HD cameras, most YouTube videos
- **4:3** (standard) - Old TV broadcasts, some older cameras

### How to Choose
1. **Don't know?** Use **16:9** (most common today)
2. **Check your source:**
   - Wide/cinematic → **16:9**
   - Square/old TV → **4:3**
   - Same as input → Choose "16:9" as safe default

3. **VideoTools handles the rest:**
   - Scales video to 720×480 (NTSC) or 720×576 (PAL)
   - Adds black bars if needed to preserve original aspect
   - Creates perfectly formatted DVD-compliant output

---

## 📊 Recommended Settings

### For Most Users (Simple Mode)
```
Format:          DVD-NTSC (MPEG-2)  [or DVD-PAL for Europe]
Aspect Ratio:    16:9
Quality:         Standard (CRF 23)
Output Name:     [your_video_name]
```

This will produce broadcast-quality DVD video.

### For Maximum Compatibility (Advanced Mode)
```
Format:          DVD-NTSC (MPEG-2)
Video Codec:     MPEG-2 (auto-selected for DVD)
Quality Preset:  Standard (CRF 23)
Bitrate Mode:    CBR (Constant Bitrate)
Video Bitrate:   6000k
Target Resolution: 720x480
Frame Rate:      29.97
Audio Codec:     AC-3 (auto for DVD)
Audio Bitrate:   192k
Audio Channels:  Stereo
Aspect Ratio:    16:9
```

---

## 🔄 Workflow: From Video to DVD Disc

### Complete Process
1. **Encode with VideoTools**
   - Select DVD format
   - Add to queue and encode
   - Produces: `myvideo.mpg`

2. **Import into DVDStyler** (free, open-source)
   - Open DVDStyler
   - Create new DVD project
   - Drag `myvideo.mpg` into the video area
   - VideoTools output imports WITHOUT re-encoding
   - No quality loss in authoring

3. **Create Menu** (optional)
   - Add chapter points
   - Design menu interface
   - Add audio tracks if desired

4. **Render to Disc**
   - Choose ISO output or direct to disc
   - Select NTSC or PAL (must match your video)
   - Burn to blank DVD-R

5. **Test Playback**
   - Play on DVD player or PS2
   - Verify video and audio quality
   - Check menu navigation

---

## 🐛 Troubleshooting

### Problem: DVD format option doesn't appear
**Solution:** Make sure you're in the Convert module and have selected a video file

### Problem: "Video will be re-encoded" warning in DVDStyler
**Solution:** This shouldn't happen with VideoTools DVD output. If it does:
- Verify you used "DVD-NTSC" or "DVD-PAL" format (not MP4/MKV)
- Check that the .mpg file was fully encoded (file size reasonable)
- Try re-importing or check DVDStyler preferences

### Problem: Audio/video sync issues during playback
**Solution:**
- Verify input video is CFR (Constant Frame Rate), not VFR
- If input was VFR, VideoTools will have warned you
- Re-encode with "Smart Inverse Telecine" option enabled if input has field order issues

### Problem: Output file is larger than expected
**Solution:** This is normal. MPEG-2 (DVD standard) produces larger files than H.264/H.265
- NTSC: ~500-700 MB per hour of video (6000k bitrate)
- PAL: ~600-800 MB per hour of video (8000k bitrate)
- This is expected and fits on single-layer DVD (4.7GB)

### Problem: Framerate conversion caused stuttering
**Solution:**
- VideoTools automatically handles common framerates
- Stuttering is usually imperceptible for 23.976→29.97 conversions
- If significant, consider pre-processing input with ffmpeg before VideoTools

---

## 💡 Pro Tips

### Tip 1: Batch Processing
- Load multiple videos at once
- Add them all to queue with same settings
- Start queue - they'll process in order
- Great for converting entire movie collections to DVD

### Tip 2: Previewing Before Encoding
- Use the preview scrubber to check source quality
- Look at aspect ratio and framerates shown
- Makes sure you selected right DVD format

### Tip 3: File Organization
- Keep source videos and DVDs in separate folders
- Name output files clearly with region (NTSC_movie.mpg, PAL_movie.mpg)
- This prevents confusion when authoring discs

### Tip 4: Testing Small Segment First
- If unsure about settings, encode just the first 5 minutes
- Author to test disc before encoding full feature
- Saves time and disc resources

### Tip 5: Backup Your MPG Files
- Keep VideoTools .mpg output as backup
- You can always re-author them to new discs later
- Re-encoding loses quality

---

## 🎥 Example: Converting a Home Video

### Scenario: Convert home video to DVD for grandparents

**Step 1: Load video**
- Load `family_vacation.mp4` from phone

**Step 2: Check specs** (shown automatically)
- Resolution: 1920x1080 (HD)
- Framerate: 29.97 fps (perfect for NTSC)
- Audio: 48 kHz (perfect)
- Duration: 45 minutes

**Step 3: Select format**
- Choose: **DVD-NTSC (MPEG-2)**
- Why: Video is 29.97 fps and will play on standard DVD players

**Step 4: Set aspect ratio**
- Choose: **16:9**
- Why: Modern phone videos are widescreen

**Step 5: Name output**
- Type: `Family Vacation`
- Output will be: `Family Vacation.mpg`

**Step 6: Queue and encode**
- Click "Add to Queue"
- System estimates: ~45 min encoding (depending on hardware)
- Click "Start Queue"

**Step 7: Author to disc**
- After encoding completes:
- Open DVDStyler
- Drag `Family Vacation.mpg` into video area
- Add title menu
- Render to ISO
- Burn ISO to blank DVD-R
- Total time to disc: ~2 hours

**Result:**
- Playable on any standalone DVD player
- Works on PlayStation 2
- Can mail to family members worldwide
- Professional quality video

---

## 📚 Additional Resources

- **DVD_IMPLEMENTATION_SUMMARY.md** - Technical specifications
- **INTEGRATION_GUIDE.md** - How features were implemented
- **QUEUE_SYSTEM_GUIDE.md** - Complete queue system reference

---

## ✅ Checklist: Before Hitting "Start Queue"

- [ ] Video file is loaded and previewed
- [ ] DVD format selected (NTSC or PAL)
- [ ] Aspect ratio chosen (4:3 or 16:9)
- [ ] Output filename entered
- [ ] Any warnings are understood and acceptable
- [ ] You have disk space for output (~5-10GB for full length feature)
- [ ] You have time for encoding (varies by computer speed)

---

## 🎊 You're Ready!

Your VideoTools is now ready to create professional DVD-Video files. Start with the Quick Start steps above, and you'll have DVD-compliant video in minutes.

Happy encoding! 📀

---

*Generated with Claude Code*
*For support, check the comprehensive guides in the project repository*
