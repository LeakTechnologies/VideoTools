# Author Module Guide

## What Does This Do?

The Author module turns your video files into DVDs that'll play in any DVD player - the kind you'd hook up to a TV. It handles all the technical stuff so you don't have to worry about it.

---

## Getting Started

### Making a Single DVD

1. Click **Author** from the main menu
2. **Files Tab** → Click "Select File" → Pick your video
3. **Settings Tab**:
   - DVD or Blu-ray (pick DVD for now)
   - NTSC or PAL - pick NTSC if you're in the US
   - 16:9 or 4:3 - pick 16:9 for widescreen
4. **Generate Tab** → Click "Generate DVD/ISO"
5. Wait for it to finish, then burn the .iso file to a DVD-R

That's it. The DVD will play in any player.

---

## Content Types: Feature, Extras, Galleries

The Author module treats every import as a **content type**, not just a file:

- **Feature**: the main movie title (supports chapters and chapter menus)
- **Extra**: bonus video titles (no chapters, separate DVD titles)
- **Gallery**: still-image slideshows (photos, artwork, stills)

### Default Behavior

- All imported videos default to **Feature**
- You can change each video’s **Content Type** using the per-item dropdown

### Extras Subtypes

Extras must be assigned a subtype so they can be grouped in menus:

- Behind the Scenes
- Deleted Scenes
- Featurettes
- Interviews
- Trailers
- Commentary
- Other

When a video is switched to **Extra**:

- It is removed from Feature and chapter logic
- It becomes a separate DVD title under **Extras**

Galleries behave like DVD-accurate still slideshows:

- Next / Previous image navigation
- Optional auto-advance
- Separate from videos and chapters

---

## Chapter Thumbnails (Automatic, Feature Only)

Every **Feature** chapter gets a thumbnail image for the Chapters menu.

### How it works

- One thumbnail is generated per chapter (FFmpeg)
- Default capture is **2 seconds into the chapter**
- If capture fails, the first valid frame is used
- Users can optionally override a thumbnail with a custom image

Extras and galleries do **not** generate chapter thumbnails.

---

## Scene Detection - Finding Chapter Points Automatically

### What Are Chapters?

You know how DVDs let you skip to different parts of the movie? Those are chapters. The Author module can find these automatically by detecting when scenes change.

### How to Use It

1. Load your video (Files or Clips tab)
2. Go to **Chapters Tab**
3. Move the "Detection Sensitivity" slider:
   - Move it **left** for more chapters (catches small changes)
   - Move it **right** for fewer chapters (only big changes)
4. Click "Detect Scenes"
5. Look at the thumbnails that pop up - these show where chapters will be
6. If it looks good, click "Accept." If not, click "Reject" and try a different sensitivity

### What Sensitivity Should I Use?

It depends on your video:

- **Movies**: Use 0.5 - 0.6 (only major scene changes)
- **TV shows**: Use 0.3 - 0.4 (catches scene changes between commercial breaks)
- **Music videos**: Use 0.2 - 0.3 (lots of quick cuts)
- **Your phone videos**: Use 0.4 - 0.5 (depends on how much you moved around)

Don't stress about getting it perfect. Just adjust the slider and click "Detect Scenes" again until the preview looks right.

### The Preview Window

After detection runs, you'll see a grid of thumbnails. Each thumbnail is a freeze-frame from where a chapter starts. This lets you actually see if the detection makes sense - way better than just seeing a list of timestamps.

The preview shows the first 24 chapters. If more were detected, you'll see a message like "Found 152 chapters (showing first 24)". That's a sign you should increase the sensitivity slider.

---

## Understanding the Settings

### Output Type

**DVD** - Standard DVD format. Works everywhere.
**Blu-ray** - Not ready yet. Stick with DVD.

### Region

**NTSC** - US, Canada, Japan. Videos play at 30 frames per second.
**PAL** - Europe, Australia, most of the world. Videos play at 25 frames per second.

Pick based on where you live. If you're not sure, pick NTSC.

### Aspect Ratio

**16:9** - Widescreen. Use this for videos from phones, cameras, YouTube.
**4:3** - Old TV shape. Only use if your video is actually in this format (rare now).
**AUTO** - Let the software decide. Safe choice.

When in doubt, use 16:9.

### Disc Size

**DVD5** - Holds 4.7 GB. Standard blank DVDs you buy at the store.
**DVD9** - Holds 8.5 GB. Dual-layer discs (more expensive).

Use DVD5 unless you're making a really long video (over 2 hours).

---

## Common Scenarios

### Scenario 1: Burning Home Videos to DVD

You filmed stuff on your phone and want to give it to relatives who don't use computers much.

1. **Files Tab** → Select your phone video
2. **Chapters Tab** → Detect scenes with sensitivity around 0.4
3. Check the preview - should show major moments (birthday, cake, opening presents, etc.)
4. **Settings Tab**:
   - Output Type: DVD
   - Region: NTSC
   - Aspect Ratio: 16:9
5. **Menu Tab** (optional):
   - Enable DVD Menus if you want a playable menu
6. **Generate Tab**:
   - Title: "Birthday 2024"
   - Pick where to save it
   - Click Generate
7. When done, burn the .iso file to a DVD-R
8. Hand it to grandma - it'll just work in her DVD player

### Scenario 2: Multiple Episodes on One Disc

You downloaded 3 episodes of a show and want them on one disc with a menu.

1. **Clips Tab** → Click "Add Video" for each episode
2. Leave "Treat as Chapters" OFF - this keeps them as separate titles
3. **Settings Tab**:
   - Output Type: DVD
   - Region: NTSC
4. **Menu Tab**:
   - Enable DVD Menus
   - Menu Structure: Feature + Extras
5. **Generate Tab** → Generate the disc
6. The DVD will have a menu where you can pick which episode to watch

### Scenario 3: Concert Video with Song Chapters

You recorded a concert and want to skip to specific songs.

Option A - Automatic:
1. Load the concert video
2. **Chapters Tab** → Try sensitivity 0.3 first
3. Look at preview - if chapters line up with songs, you're done
4. If not, adjust sensitivity and try again

Option B - Manual:
1. Play through the video and note the times when songs start
2. **Chapters Tab** → Click "+ Add Chapter" for each song
3. Enter the time (like 3:45 for 3 minutes 45 seconds)
4. Name it (Song 1, Song 2, etc.)

---

## What's Happening Behind the Scenes?

You don't need to know this to use the software, but if you're curious:

### The Encoding Process

When you click Generate:

1. **Encoding**: Your video gets converted to MPEG-2 format (the DVD standard)
2. **Timestamp Fix**: The software makes sure the timestamps are perfectly sequential (DVDs are picky about this)
3. **Structure Creation**: It builds the VIDEO_TS folder structure that DVD players expect
4. **ISO Creation**: If you picked ISO, everything gets packed into one burnable file

### Why Does It Take So Long?

Converting video to MPEG-2 is CPU-intensive. A 90-minute video might take 30-60 minutes to encode, depending on your computer. You can queue multiple jobs and let it run overnight.

### The Timestamp Fix Thing

Some videos, especially .avi files, have timestamps that go slightly backwards occasionally. DVD players hate this and will error out. The software automatically fixes it by running the encoded video through a "remux" step - think of it like reformatting a document to fix the page numbers. Takes a few extra seconds but ensures the DVD actually works.

---

## Troubleshooting

### "I got 200 chapters, that's way too many"

Your sensitivity is too low. Move the slider right to 0.5 or higher and try again.

### "It only found 3 chapters in a 2-hour movie"

Sensitivity is too high. Move the slider left to 0.3 or 0.4.

### "The program is really slow when generating"

That's normal. Encoding video is slow. The good news is you can:
- Queue multiple jobs and walk away
- Work on other stuff - the encoding happens in the background
- Check the log to see progress

### "The authoring log is making everything lag"

This was a bug that's now fixed. The log only shows the last 100 lines. If you want to see everything, click "View Full Log" and it opens in a separate window.

### "My ISO file won't fit on a DVD-R"

Your video is too long or the quality is too high. Options:
- Use a dual-layer DVD-R (DVD9) instead
- Split into 2 discs
- Check if you accidentally loaded multiple long videos

### "The DVD plays but skips or stutters"

This is usually because your original video had variable frame rate (VFR) - phone videos often do this. The software will warn you if it detects this. Solution:
- Try generating again (sometimes it just works)
- Convert the source video to constant frame rate first using the Convert module
- Check if the source video itself plays smoothly

---

## File Size Reference

Here's roughly how much video fits on each disc type:

**DVD5 (4.7 GB)**
- About 2 hours of video at standard quality
- Most movies fit comfortably

**DVD9 (8.5 GB)**
- About 4 hours of video
- Good for director's cuts or multiple episodes

If you're over these limits, split your content across multiple discs.

---

## The Output Files Explained

### VIDEO_TS Folder

This is what DVD players actually read. It contains:
- .IFO files - the "table of contents"
- .VOB files - the actual video data

You can copy this folder to a USB drive and some DVD players can read it directly.

### ISO File

Think of this as a zip file of the VIDEO_TS folder, formatted specifically for burning to disc. When you burn an ISO to a DVD-R, it extracts everything into the right structure automatically.

---

## Tips

**Test Before Making Multiple Copies**
Make one disc, test it in a DVD player, make sure everything works. Then make more copies.

**Name Your Files Clearly**
Use names like "vacation_2024.iso" not "output.iso". Future you will thank you.

**Keep the Source Files**
Don't delete your original videos after making DVDs. Hard drives are cheap, memories aren't.

**Preview the Chapters**
Always check that chapter preview before accepting. It takes 10 seconds and prevents surprises.

**Use the Queue**
Got 5 videos to convert? Add them all to the queue and start it before bed. They'll all be done by morning.

---

## Related Guides

- **DVD_USER_GUIDE.md** - How to use the Convert module for DVD encoding
- **QUEUE_SYSTEM_GUIDE.md** - Managing multiple jobs
- **MODULES.md** - What all the other modules do

---

That's everything. Load a video, adjust some settings, click Generate. The software handles the complicated parts.
