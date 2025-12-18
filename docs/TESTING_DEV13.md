# VideoTools v0.1.0-dev13 Testing Guide

This document provides a comprehensive testing checklist for all dev13 features.

## Build Status
- ✅ **Compiles successfully** with no errors
- ✅ **CLI help** displays correctly with compare command
- ✅ **All imports** resolved correctly (regexp added for cropdetect)

## Features to Test

### 1. Compare Module

**Test Steps:**
1. Launch VideoTools GUI
2. Click "Compare" module button (pink/magenta color)
3. Click "Load File 1" and select a video
4. Click "Load File 2" and select another video
5. Click "COMPARE" button

**Expected Results:**
- File 1 and File 2 metadata displayed side-by-side
- Shows: Format, Resolution, Duration, Codecs, Bitrates, Frame Rate
- Shows: Pixel Format, Aspect Ratio, Color Space, Color Range
- Shows: GOP Size, Field Order, Chapters, Metadata flags
- formatBitrate() displays bitrates in human-readable format (Mbps/kbps)

**CLI Test:**
```bash
./VideoTools compare video1.mp4 video2.mp4
```

**Code Verification:**
- ✅ buildCompareView() function implemented (main.go:4916)
- ✅ HandleCompare() handler registered (main.go:59)
- ✅ Module button added to grid with pink color (main.go:69)
- ✅ formatBitrate() helper function (main.go:4900)
- ✅ compareFile1/compareFile2 added to appState (main.go:197-198)

---

### 2. Target File Size Encoding Mode

**Test Steps:**
1. Load a video in Convert module
2. Switch to Advanced mode
3. Set Bitrate Mode to "Target Size"
4. Enter target size (e.g., "25MB", "100MB", "8MB")
5. Start conversion or add to queue

**Expected Results:**
- FFmpeg calculates video bitrate from: target size, duration, audio bitrate
- Reserves 3% for container overhead
- Minimum 100 kbps sanity check applied
- Works in both direct convert and queue jobs

**Test Cases:**
- Video: 1 minute, Target: 25MB, Audio: 192k → Video bitrate calculated
- Video: 5 minutes, Target: 100MB, Audio: 192k → Video bitrate calculated
- Very small target that would be impossible → Falls back to 100 kbps minimum

**Code Verification:**
- ✅ TargetFileSize field added to convertConfig (main.go:125)
- ✅ Target Size UI entry with placeholder (main.go:1931-1936)
- ✅ ParseFileSize() parses KB/MB/GB (internal/convert/types.go:205)
- ✅ CalculateBitrateForTargetSize() with overhead calc (internal/convert/types.go:173)
- ✅ Applied in startConvert() (main.go:3993)
- ✅ Applied in executeConvertJob() (main.go:1109)
- ✅ Passed to queue config (main.go:611)

---

### 3. Automatic Black Bar Detection & Cropping

**Test Steps:**
1. Load a video with black bars (letterbox/pillarbox)
2. Switch to Advanced mode
3. Scroll to AUTO-CROP section
4. Click "Detect Crop" button
5. Wait for detection (button shows "Detecting...")
6. Review detection dialog showing savings estimate
7. Click "Apply" to use detected values
8. Verify AutoCrop checkbox is checked

**Expected Results:**
- Samples 10 seconds from middle of video
- Uses FFmpeg cropdetect filter (threshold 24)
- Shows original vs cropped dimensions
- Calculates and displays pixel reduction percentage
- Applies crop values to config
- Works for both direct convert and queue jobs

**Test Cases:**
- Video with letterbox bars (top/bottom) → Detects and crops
- Video with pillarbox bars (left/right) → Detects and crops
- Video with no black bars → Shows "already fully cropped" message
- Very short video (<10 seconds) → Still attempts detection

**Code Verification:**
- ✅ detectCrop() function with 30s timeout (main.go:4841)
- ✅ CropValues struct (main.go:4832)
- ✅ Regex parsing: crop=(\d+):(\d+):(\d+):(\d+) (main.go:4870)
- ✅ AutoCrop checkbox in UI (main.go:1765)
- ✅ Detect Crop button with background execution (main.go:1771)
- ✅ Confirmation dialog with savings calculation (main.go:1797)
- ✅ Crop filter applied before scaling (main.go:3996)
- ✅ Works in queue jobs (main.go:1023)
- ✅ CropWidth/Height/X/Y fields added (main.go:136-139)
- ✅ Passed to queue config (main.go:621-625)

---

### 4. Frame Rate Conversion UI with Size Estimates

**Test Steps:**
1. Load a 60fps video in Convert module
2. Switch to Advanced mode
3. Find "Frame Rate" dropdown
4. Select "30" fps
5. Observe hint message below dropdown

**Expected Results:**
- Shows: "Converting 60 → 30 fps: ~50% smaller file"
- Hint updates dynamically when selection changes
- Warning shown for upscaling: "⚠ Upscaling from 30 to 60 fps (may cause judder)"
- No hint when "Source" selected or target equals source

**Test Cases:**
- 60fps → 30fps: Shows ~50% reduction
- 60fps → 24fps: Shows ~60% reduction
- 30fps → 60fps: Shows upscaling warning
- 30fps → 30fps: No hint (same as source)
- Video with unknown fps: No hint shown

**Frame Rate Options:**
- Source, 23.976, 24, 25, 29.97, 30, 50, 59.94, 60

**Code Verification:**
- ✅ All frame rate options added (main.go:2107)
- ✅ updateFrameRateHint() function (main.go:2051)
- ✅ Calculates reduction percentage (main.go:2094-2098)
- ✅ Upscaling warning (main.go:2099-2101)
- ✅ frameRateHint label in UI (main.go:2215)
- ✅ Updates on selection change (main.go:2110)
- ✅ FFmpeg fps filter already applied (main.go:4643-4646)

---

### 5. Encoder Preset Descriptions

**Test Steps:**
1. Load any video in Convert module
2. Switch to Advanced mode
3. Find "Encoder Preset" dropdown
4. Select different presets and observe hint

**Expected Results:**
- Each preset shows speed vs quality trade-off
- Visual icons: ⚡⏩⚖️🎯🐌
- Shows percentage differences vs baseline
- Recommends "slow" as best quality/size ratio

**Preset Information:**
- ultrafast: ⚡ ~10x faster than slow, ~30% larger
- superfast: ⚡ ~7x faster than slow, ~20% larger
- veryfast: ⚡ ~5x faster than slow, ~15% larger
- faster: ⏩ ~3x faster than slow, ~10% larger
- fast: ⏩ ~2x faster than slow, ~5% larger
- medium: ⚖️ Balanced (default baseline)
- slow: 🎯 Best ratio ~2x slower, ~5-10% smaller (RECOMMENDED)
- slower: 🎯 ~3x slower, ~10-15% smaller
- veryslow: 🐌 ~5x slower, ~15-20% smaller

**Code Verification:**
- ✅ updateEncoderPresetHint() function (main.go:2006)
- ✅ All 9 presets with descriptions (main.go:2009-2027)
- ✅ Visual icons for categories (main.go:2010, 2016, 2020, 2022, 2026)
- ✅ encoderPresetHint label in UI (main.go:2233)
- ✅ Updates on selection change (main.go:2036)
- ✅ Initialized with current preset (main.go:2039)

---

## Integration Testing

### Queue System Integration
**All features must work when added to queue:**
- [ ] Compare module (N/A - not a conversion operation)
- [ ] Target File Size mode in queue job
- [ ] Auto-crop in queue job
- [ ] Frame rate conversion in queue job
- [ ] Encoder preset in queue job

**Code Verification:**
- ✅ All config fields passed to queue (main.go:599-634)
- ✅ executeConvertJob() handles all new fields
- ✅ Target Size: lines 1109-1133
- ✅ Auto-crop: lines 1023-1048
- ✅ Frame rate: line 1091-1094
- ✅ Encoder preset: already handled via encoderPreset field

### Settings Persistence
**Settings should persist across video loads:**
- [ ] Auto-crop checkbox state persists
- [ ] Frame rate selection persists
- [ ] Encoder preset selection persists
- [ ] Target file size value persists

**Code Verification:**
- ✅ All settings stored in state.convert
- ✅ Settings not reset when loading new video
- ✅ Reset button available to restore defaults (main.go:1823)

---

## Known Limitations

1. **Auto-crop detection:**
   - Samples only 10 seconds (may miss variable content)
   - 30-second timeout for very slow systems
   - Assumes black bars are consistent throughout video

2. **Frame rate conversion:**
   - Estimates are approximate (actual savings depend on content)
   - No motion interpolation (drops/duplicates frames only)

3. **Target file size:**
   - Estimate based on single-pass encoding
   - Container overhead assumed at 3%
   - Actual file size may vary by ±5%

4. **Encoder presets:**
   - Speed/size estimates are averages
   - Actual performance depends on video complexity
   - GPU acceleration may alter speed ratios

---

## Manual Testing Checklist

### Pre-Testing Setup
- [ ] Have test videos ready:
  - [ ] 60fps video for frame rate testing
  - [ ] Video with black bars for crop detection
  - [ ] Short video (< 1 min) for quick testing
  - [ ] Long video (> 5 min) for queue testing

### Compare Module
- [ ] Load two different videos
- [ ] Compare button shows both metadata
- [ ] Bitrates display correctly (Mbps/kbps)
- [ ] All fields populated correctly
- [ ] "Back to Menu" returns to main menu

### Target File Size
- [ ] Set target of 25MB on 1-minute video
- [ ] Verify conversion completes
- [ ] Check output file size (should be close to 25MB ±5%)
- [ ] Test with very small target (e.g., 1MB)
- [ ] Verify in queue job

### Auto-Crop
- [ ] Detect crop on letterbox video
- [ ] Verify savings percentage shown
- [ ] Apply detected values
- [ ] Convert with crop applied
- [ ] Compare output dimensions
- [ ] Test with no-black-bar video (should say "already fully cropped")
- [ ] Verify in queue job

### Frame Rate Conversion
- [ ] Load 60fps video
- [ ] Select 30fps
- [ ] Verify hint shows "~50% smaller"
- [ ] Select 60fps (same as source)
- [ ] Verify no hint shown
- [ ] Select 24fps
- [ ] Verify different percentage shown
- [ ] Try upscaling (30→60)
- [ ] Verify warning shown

### Encoder Presets
- [ ] Select "ultrafast" - verify hint shows
- [ ] Select "medium" - verify balanced description
- [ ] Select "slow" - verify recommendation shown
- [ ] Select "veryslow" - verify maximum compression note
- [ ] Test actual encoding with different presets
- [ ] Verify speed differences are noticeable

### Error Cases
- [ ] Auto-crop with no video loaded → Should show error dialog
- [ ] Very short video for crop detection → Should still attempt
- [ ] Invalid target file size (e.g., "abc") → Should handle gracefully
- [ ] Extremely small target size → Should apply 100kbps minimum

---

## Performance Testing

### Auto-Crop Detection Speed
- Expected: ~2-5 seconds for typical video
- Timeout: 30 seconds maximum
- [ ] Test on 1080p video
- [ ] Test on 4K video
- [ ] Test on very long video (should still sample 10s)

### Memory Usage
- [ ] Load multiple videos in compare mode
- [ ] Check memory doesn't leak
- [ ] Test with large (4K+) videos

---

## Regression Testing

Verify existing features still work:
- [ ] Basic video conversion works
- [ ] Queue add/remove/execute works
- [ ] Direct convert (not queued) works
- [ ] Simple mode still functional
- [ ] Advanced mode shows all controls
- [ ] Aspect ratio handling works
- [ ] Deinterlacing works
- [ ] Audio settings work
- [ ] Hardware acceleration detection works

---

## Documentation Review

- ✅ DONE.md updated with all features
- ✅ TODO.md marked features as complete
- ✅ Commit messages are descriptive
- ✅ Code comments explain complex logic
- [ ] README.md updated (if needed)

---

## Code Quality

### Code Review Completed:
- ✅ No compilation errors
- ✅ All imports resolved
- ✅ No obvious logic errors
- ✅ Error handling present (dialogs, nil checks)
- ✅ Logging added for debugging
- ✅ Function names are descriptive
- ✅ Code follows existing patterns

### Potential Issues to Watch:
- Crop detection regex assumes specific FFmpeg output format
- Frame rate hint calculations assume source FPS is accurate
- Target size calculation assumes consistent bitrate encoding
- 30-second timeout for crop detection might be too short on very slow systems

---

## Sign-off

**Build Status:** ✅ PASSING
**Code Review:** ✅ COMPLETED
**Manual Testing:** ⏳ PENDING (requires video files)
**Documentation:** ✅ COMPLETED

**Ready for User Testing:** YES (with video files)

---

## Testing Commands

```bash
# Build
go build -o VideoTools

# CLI Help
./VideoTools help

# Compare (CLI)
./VideoTools compare video1.mp4 video2.mp4

# GUI
./VideoTools

# Debug mode
VIDEOTOOLS_DEBUG=1 ./VideoTools
```

---

Last Updated: 2025-12-03
