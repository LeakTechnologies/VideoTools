# LosslessCut Features - Inspiration for VideoTools Trim Module

## Overview
LosslessCut is a mature, feature-rich video trimming application built on Electron/React with FFmpeg backend. This document extracts key features and UX patterns that should inspire VideoTools' Trim module development.

---

## 🎯 Core Trim Features to Adopt

### 1. **Segment-Based Editing** ⭐⭐⭐ (HIGHEST PRIORITY)
LosslessCut uses "segments" as first-class citizens rather than simple In/Out points.

**How it works:**
- Each segment has: start time, end time (optional), label, tags, and segment number
- Multiple segments can exist on timeline simultaneously
- Segments without end time = "markers" (vertical lines on timeline)
- Segments can be reordered by drag-drop in segment list

**Benefits for VideoTools:**
- User can mark multiple trim regions in one session
- Export all segments at once (batch trim)
- Save/load trim projects for later refinement
- More flexible than single In/Out point workflow

**Implementation priority:** HIGH
- Start with single segment (In/Out points)
- Phase 2: Add multiple segments support
- Phase 3: Add segment labels/tags

**Example workflow:**
```
1. User loads video
2. Finds first good section: 0:30 to 1:45 → Press I, seek, press O → Segment 1 created
3. Press + to add new segment
4. Finds second section: 3:20 to 5:10 → Segment 2 created
5. Export → Creates 2 output files (or 1 merged file if mode set)
```

---

### 2. **Keyboard-First Workflow** ⭐⭐⭐ (HIGHEST PRIORITY)
LosslessCut is designed for speed via keyboard shortcuts.

**Essential shortcuts:**
| Key | Action | Notes |
|-----|--------|-------|
| `SPACE` | Play/Pause | Standard |
| `I` | Set segment In point | Industry standard (Adobe, FCP) |
| `O` | Set segment Out point | Industry standard |
| `←` / `→` | Seek backward/forward | Frame or keyframe stepping |
| `,` / `.` | Frame step | Precise frame-by-frame (1 frame) |
| `+` | Add new segment | Quick workflow |
| `B` | Split segment at cursor | Divide segment into two |
| `BACKSPACE` | Delete segment/cutpoint | Quick removal |
| `E` | Export | Fast export shortcut |
| `C` | Capture screenshot | Snapshot current frame |
| Mouse wheel | Seek timeline | Smooth scrubbing |

**Why keyboard shortcuts matter:**
- Professional users edit faster with keyboard
- Reduces mouse movement fatigue
- Enables "flow state" editing
- Standard shortcuts reduce learning curve

**Implementation for VideoTools:**
- Integrate keyboard handling into VT_Player
- Show keyboard shortcut overlay (SHIFT+/)
- Allow user customization later

---

### 3. **Timeline Zoom** ⭐⭐⭐ (HIGH PRIORITY)
Timeline can zoom in/out for precision editing.

**How it works:**
- Zoom slider or mouse wheel on timeline
- Zoomed view shows: thumbnails, waveform, keyframes
- Timeline scrolls horizontally when zoomed
- Zoom follows playhead (keeps current position centered)

**Benefits:**
- Find exact cut points in long videos
- Frame-accurate editing even in 2-hour files
- See waveform detail for audio-based cuts

**Implementation notes:**
- VT_Player needs horizontal scrolling timeline widget
- Zoom level: 1x (full video) to 100x (extreme detail)
- Auto-scroll to keep playhead in view

---

### 4. **Waveform Display** ⭐⭐ (MEDIUM PRIORITY)
Audio waveform shown on timeline for visual reference.

**Features:**
- Shows amplitude over time
- Useful for finding speech/silence boundaries
- Click waveform to seek
- Updates as timeline zooms

**Use cases:**
- Trim silence from beginning/end
- Find exact start of dialogue
- Cut between sentences
- Detect audio glitches

**Implementation:**
- FFmpeg can generate waveform images: `ffmpeg -i input.mp4 -filter_complex showwavespic output.png`
- Display as timeline background
- Optional feature (enable/disable)

---

### 5. **Keyframe Visualization** ⭐⭐ (MEDIUM PRIORITY)
Timeline shows video keyframes (I-frames) as markers.

**Why keyframes matter:**
- Lossless copy (`-c copy`) only cuts at keyframes
- Cutting between keyframes requires re-encode
- Users need visual feedback on keyframe positions

**How LosslessCut handles it:**
- Vertical lines on timeline = keyframes
- Color-coded: bright = keyframe, dim = P/B frame
- "Smart cut" mode: cuts at keyframe + re-encodes small section

**Implementation for VideoTools:**
- Probe keyframes: `ffprobe -select_streams v -show_frames -show_entries frame=pict_type,pts_time`
- Display on timeline
- Warn user if cut point not on keyframe (when using `-c copy`)

---

### 6. **Invert Cut Mode** ⭐⭐ (MEDIUM PRIORITY)
Yin-yang toggle: Keep segments vs. Remove segments

**Two modes:**
1. **Keep mode** (default): Export marked segments, discard rest
2. **Cut mode** (inverted): Remove marked segments, keep rest

**Example:**
```
Video: [────────────────────]
Segments: [  SEG1  ]  [  SEG2  ]

Keep mode → Output: SEG1.mp4, SEG2.mp4
Cut mode → Output: parts between segments (commercials removed)
```

**Use cases:**
- **Keep**: Extract highlights from long recording
- **Cut**: Remove commercials from TV recording

**Implementation:**
- Simple boolean toggle in UI
- Changes FFmpeg command logic
- Useful for both workflows

---

### 7. **Merge Mode** ⭐⭐ (MEDIUM PRIORITY)
Option to merge multiple segments into single output file.

**Export options:**
- **Separate files**: Each segment → separate file
- **Merge cuts**: All segments → 1 merged file
- **Merge + separate**: Both outputs

**FFmpeg technique:**
```bash
# Create concat file listing segments
echo "file 'segment1.mp4'" > concat.txt
echo "file 'segment2.mp4'" >> concat.txt

# Merge with concat demuxer
ffmpeg -f concat -safe 0 -i concat.txt -c copy merged.mp4
```

**Implementation:**
- UI toggle: "Merge segments"
- Temp directory for segment exports
- Concat demuxer for lossless merge
- Clean up temp files after

---

### 8. **Manual Timecode Entry** ⭐ (LOW PRIORITY)
Type exact timestamps instead of scrubbing.

**Features:**
- Click In/Out time → text input appears
- Type: `1:23:45.123` or `83.456`
- Formats: HH:MM:SS.mmm, MM:SS, seconds
- Paste timestamps from clipboard

**Use cases:**
- User has exact timestamps from notes
- Import cut times from CSV/spreadsheet
- Frame-accurate entry (1:23:45.033)

**Implementation:**
- Text input next to In/Out displays
- Parse various time formats
- Validate against video duration

---

### 9. **Project Files (.llc)** ⭐ (LOW PRIORITY - FUTURE)
Save segments to file, resume editing later.

**LosslessCut project format (JSON5):**
```json
{
  "version": 1,
  "cutSegments": [
    {
      "start": 30.5,
      "end": 105.3,
      "name": "Opening scene",
      "tags": { "category": "intro" }
    },
    {
      "start": 180.0,
      "end": 245.7,
      "name": "Action sequence"
    }
  ]
}
```

**Benefits:**
- Resume trim session after closing app
- Share trim points with team
- Version control trim decisions

**Implementation (later):**
- Simple JSON format
- Save/load from File menu
- Auto-save to temp on changes

---

## 🎨 UX Patterns to Adopt

### 1. **Timeline Interaction Model**
- Click timeline → seek to position
- Drag timeline → scrub (live preview)
- Mouse wheel → seek forward/backward
- Shift+wheel → zoom timeline
- Right-click → context menu (set In/Out, add segment, etc.)

### 2. **Visual Feedback**
- **Current time indicator**: Vertical line with triangular markers (top/bottom)
- **Segment visualization**: Colored rectangles on timeline
- **Hover preview**: Show timestamp on hover
- **Segment labels**: Display segment names on timeline

### 3. **Segment List Panel**
LosslessCut shows sidebar with all segments:
```
┌─ Segments ─────────────────┐
│ 1. [00:30 - 01:45] Intro   │ ← Selected
│ 2. [03:20 - 05:10] Action  │
│ 3. [07:00 - 09:30] Ending  │
└────────────────────────────┘
```
**Features:**
- Click segment → select & seek to start
- Drag to reorder
- Right-click for options (rename, delete, duplicate)

### 4. **Export Preview Dialog**
Before final export, show summary:
```
┌─ Export Preview ──────────────────────────┐
│ Export mode: Separate files               │
│ Output format: MP4 (same as source)       │
│ Keyframe mode: Smart cut                  │
│                                            │
│ Segments to export:                       │
│  1. Intro.mp4 (0:30 - 1:45) → 1.25 min   │
│  2. Action.mp4 (3:20 - 5:10) → 1.83 min  │
│  3. Ending.mp4 (7:00 - 9:30) → 2.50 min  │
│                                            │
│ Total output size: ~125 MB                │
│                                            │
│         [Cancel]  [Export]                │
└───────────────────────────────────────────┘
```

---

## 🚀 Advanced Features (Future Inspiration)

### 1. **Scene Detection**
Auto-create segments at scene changes.
```bash
ffmpeg -i input.mp4 -filter_complex \
  "select='gt(scene,0.4)',metadata=print:file=scenes.txt" \
  -f null -
```

### 2. **Silence Detection**
Auto-trim silent sections.
```bash
ffmpeg -i input.mp4 -af silencedetect=noise=-30dB:d=0.5 -f null -
```

### 3. **Black Screen Detection**
Find and remove black sections.
```bash
ffmpeg -i input.mp4 -vf blackdetect=d=0.5:pix_th=0.10 -f null -
```

### 4. **Chapter Import/Export**
- Load MKV/MP4 chapters as segments
- Export segments as chapter markers
- Useful for DVD/Blu-ray rips

### 5. **Thumbnail Scrubbing**
- Generate thumbnail strip
- Show preview on timeline hover
- Faster visual navigation

---

## 📋 Implementation Roadmap for VideoTools

### Phase 1: Essential Trim (Week 1-2)
**Goal:** Basic usable trim functionality
- ✅ VT_Player keyframing API (In/Out points)
- ✅ Keyboard shortcuts (I, O, Space, ←/→)
- ✅ Timeline markers visualization
- ✅ Single segment export
- ✅ Keep/Cut mode toggle

### Phase 2: Professional Workflow (Week 3-4)
**Goal:** Multi-segment editing
- Multiple segments support
- Segment list panel
- Drag-to-reorder segments
- Merge mode
- Timeline zoom

### Phase 3: Visual Enhancements (Week 5-6)
**Goal:** Precision editing
- Waveform display
- Keyframe visualization
- Frame-accurate stepping
- Manual timecode entry

### Phase 4: Advanced Features (Week 7+)
**Goal:** Power user tools
- Project save/load
- Scene detection
- Silence detection
- Export presets
- Batch processing

---

## 🎓 Key Lessons from LosslessCut

### 1. **Start Simple, Scale Later**
LosslessCut began with basic trim, added features over time. Don't over-engineer initial release.

### 2. **Keyboard Shortcuts are Critical**
Professional users demand keyboard efficiency. Design around keyboard-first workflow.

### 3. **Visual Feedback Matters**
Users need to SEE what they're doing:
- Timeline markers
- Segment rectangles
- Waveforms
- Keyframes

### 4. **Lossless is Tricky**
Educate users about keyframes, smart cut, and when re-encode is necessary.

### 5. **FFmpeg Does the Heavy Lifting**
LosslessCut is primarily a UI wrapper around FFmpeg. Focus on great UX, let FFmpeg handle processing.

---

## 🔗 References

- **LosslessCut GitHub**: https://github.com/mifi/lossless-cut
- **Documentation**: `~/tools/lossless-cut/docs.md`
- **Source code**: `~/tools/lossless-cut/src/`
- **Keyboard shortcuts**: `~/tools/lossless-cut/README.md` (search "keyboard")

---

## 💡 VideoTools-Specific Considerations

### Advantages VideoTools Has:
1. **Native Go + Fyne**: Faster startup, smaller binary than Electron
2. **Integrated workflow**: Trim → Convert → Compare in one app
3. **Queue system**: Already have batch processing foundation
4. **Smart presets**: Leverage existing quality presets

### Unique Features to Add:
1. **Trim + Convert**: Set In/Out, choose quality preset, export in one step
2. **Compare integration**: Auto-load trimmed vs. original for verification
3. **Batch trim**: Apply same trim offsets to multiple files (e.g., remove first 30s from all)
4. **Smart defaults**: Detect intros/outros and suggest trim points

---

## ✅ Action Items for VT_Player Team

Based on LosslessCut analysis, VT_Player needs:

### Essential APIs:
1. **Keyframe API**
   ```go
   SetInPoint(time.Duration)
   SetOutPoint(time.Duration)
   GetInPoint() (time.Duration, bool)
   GetOutPoint() (time.Duration, bool)
   ClearKeyframes()
   ```

2. **Timeline Visualization**
   - Draw In/Out markers on timeline
   - Highlight segment region between markers
   - Support multiple segments (future)

3. **Keyboard Shortcuts**
   - I/O for In/Out points
   - ←/→ for frame stepping
   - Space for play/pause
   - Mouse wheel for seek

4. **Frame Navigation**
   ```go
   StepForward()  // Next frame
   StepBackward() // Previous frame
   GetCurrentFrame() int64
   SeekToFrame(int64)
   ```

5. **Timeline Zoom** (Phase 2)
   ```go
   SetZoomLevel(float64) // 1.0 to 100.0
   GetZoomLevel() float64
   ScrollToTime(time.Duration)
   ```

### Reference Implementation:
- Study LosslessCut's Timeline.tsx for zoom logic
- Study TimelineSeg.tsx for segment visualization
- Study useSegments.tsx for segment state management

---

**Document created**: 2025-12-04
**Source**: LosslessCut v3.x codebase analysis
**Next steps**: Share with VT_Player team, begin Phase 1 implementation
