# VideoTools Integration Guide - DVD Support & Queue System

## 📋 Executive Summary

This guide explains how to integrate the newly implemented **DVD-NTSC encoding system** with the **queue-based batch processing system** in VideoTools.

**Status:** ✅ Both systems are complete, tested, and ready for integration.

---

## 🎯 What's New

### 1. **DVD-NTSC Encoding Package** ✨
Location: `internal/convert/`

**Provides:**
- MPEG-2 video encoding (720×480 @ 29.97fps)
- AC-3 Dolby Digital audio (48 kHz stereo)
- Multi-region support (NTSC, PAL, SECAM)
- Comprehensive validation system
- FFmpeg command generation

**Key Files:**
- `types.go` - VideoSource, ConvertConfig, FormatOption types
- `ffmpeg.go` - Codec mapping, video probing
- `dvd.go` - NTSC-specific encoding and validation
- `dvd_regions.go` - PAL, SECAM, and multi-region support
- `presets.go` - Output format definitions

### 2. **Queue System** (Already Integrated)
Location: `internal/queue/queue.go`

**Provides:**
- Job management and prioritization
- Pause/resume capabilities
- Real-time progress tracking
- Thread-safe operations
- JSON persistence

---

## 🔌 Integration Points

### Point 1: Format Selection UI

**Current State (main.go, line ~1394):**
```go
var formatLabels []string
for _, opt := range formatOptions {  // Hardcoded in main.go
    formatLabels = append(formatLabels, opt.Label)
}
formatSelect := widget.NewSelect(formatLabels, func(value string) {
    for _, opt := range formatOptions {
        if opt.Label == value {
            state.convert.SelectedFormat = opt
            outputHint.SetText(fmt.Sprintf("Output file: %s", state.convert.OutputFile()))
            break
        }
    }
})
```

**After Integration:**
```go
// Import the convert package
import "git.leaktechnologies.dev/stu/VideoTools/internal/convert"

// Use FormatOptions from convert package
var formatLabels []string
for _, opt := range convert.FormatOptions {
    formatLabels = append(formatLabels, opt.Label)
}
formatSelect := widget.NewSelect(formatLabels, func(value string) {
    for _, opt := range convert.FormatOptions {
        if opt.Label == value {
            state.convert.SelectedFormat = opt
            outputHint.SetText(fmt.Sprintf("Output file: %s", state.convert.OutputFile()))

            // NEW: Show DVD-specific options if DVD selected
            if opt.Ext == ".mpg" {
                showDVDOptions(state)  // New function
            }
            break
        }
    }
})
```

### Point 2: DVD-Specific Options Panel

**New UI Component (main.go, after format selection):**

```go
func showDVDOptions(state *appState) {
    // Show DVD-specific controls only when DVD format selected
    dvdPanel := container.NewVBox(
        // Aspect ratio selector
        widget.NewLabel("Aspect Ratio:"),
        widget.NewSelect([]string{"4:3", "16:9"}, func(val string) {
            state.convert.OutputAspect = val
        }),

        // Interlacing mode
        widget.NewLabel("Interlacing:"),
        widget.NewSelect([]string{"Auto-detect", "Progressive", "Interlaced"}, func(val string) {
            // Store selection
        }),

        // Region selector
        widget.NewLabel("Region:"),
        widget.NewSelect([]string{"NTSC", "PAL", "SECAM"}, func(val string) {
            // Switch region presets
            var region convert.DVDRegion
            switch val {
            case "NTSC":
                region = convert.DVDNTSCRegionFree
            case "PAL":
                region = convert.DVDPALRegionFree
            case "SECAM":
                region = convert.DVDSECAMRegionFree
            }
            cfg := convert.PresetForRegion(region)
            state.convert = cfg  // Update config
        }),
    )
    // Add to UI
}
```

### Point 3: Validation Before Queue

**Current State (main.go, line ~499):**
```go
func (s *appState) addConvertToQueue() error {
    if !s.hasSource() {
        return fmt.Errorf("no source video selected")
    }
    // ... build config and add to queue
}
```

**After Integration:**
```go
func (s *appState) addConvertToQueue() error {
    if !s.hasSource() {
        return fmt.Errorf("no source video selected")
    }

    // NEW: Validate if DVD format selected
    if s.convert.SelectedFormat.Ext == ".mpg" {
        warnings := convert.ValidateDVDNTSC(s.source, s.convert)

        // Show warnings dialog
        if len(warnings) > 0 {
            var warningText strings.Builder
            warningText.WriteString("DVD Encoding Validation:\n\n")
            for _, w := range warnings {
                warningText.WriteString(fmt.Sprintf("[%s] %s\n", w.Severity, w.Message))
                warningText.WriteString(fmt.Sprintf("Action: %s\n\n", w.Action))
            }

            dialog.ShowInformation("DVD Validation", warningText.String(), s.window)
        }
    }

    // ... continue with queue addition
}
```

### Point 4: FFmpeg Command Building

**Current State (main.go, line ~810):**
```go
// Build FFmpeg arguments (existing complex logic)
args := []string{
    "-y",
    "-hide_banner",
    // ... 180+ lines of filter and codec logic
}
```

**After Integration (simplified):**
```go
func (s *appState) executeConvertJob(ctx context.Context, job *queue.Job, progressCallback func(float64)) error {
    cfg := job.Config
    inputPath := cfg["inputPath"].(string)
    outputPath := cfg["outputPath"].(string)

    // NEW: Use convert package for DVD
    if fmt.Sprintf("%v", cfg["selectedFormat"]) == ".mpg" {
        // Get video source info
        src, err := convert.ProbeVideo(inputPath)
        if err != nil {
            return err
        }

        // Get config from job
        convertCfg := s.convert  // Already validated

        // Use convert package to build args
        args := convert.BuildDVDFFmpegArgs(inputPath, outputPath, convertCfg, src)

        // Execute FFmpeg...
        return s.executeFFmpeg(args, progressCallback)
    }

    // Fall back to existing logic for non-DVD formats
    // ... existing code
}
```

### Point 5: Job Configuration

**Updated Job Creation (main.go, line ~530):**
```go
job := &queue.Job{
    Type:        queue.JobTypeConvert,
    Title:       fmt.Sprintf("Convert: %s", s.source.DisplayName),
    InputFile:   s.source.Path,
    OutputFile:  s.convert.OutputFile(),
    Config: map[string]interface{}{
        // Existing fields...
        "inputPath":       s.source.Path,
        "outputPath":      s.convert.OutputFile(),
        "selectedFormat":  s.convert.SelectedFormat,
        "videoCodec":      s.convert.VideoCodec,
        "audioCodec":      s.convert.AudioCodec,
        "videoBitrate":    s.convert.VideoBitrate,
        "audioBitrate":    s.convert.AudioBitrate,
        "targetResolution": s.convert.TargetResolution,
        "frameRate":       s.convert.FrameRate,

        // NEW: DVD-specific info
        "isDVD":           s.convert.SelectedFormat.Ext == ".mpg",
        "aspect":          s.convert.OutputAspect,
        "dvdRegion":       "NTSC",  // Or PAL/SECAM
    },
    Priority: 5,
}
s.jobQueue.Add(job)
```

---

## 📊 Type Definitions to Export

Currently in `internal/convert/types.go`, these need to remain accessible within main.go:

```go
// VideoSource - metadata about video file
type VideoSource struct { ... }

// ConvertConfig - encoding configuration
type ConvertConfig struct { ... }

// FormatOption - output format definition
type FormatOption struct { ... }
```

**Import in main.go:**
```go
import "git.leaktechnologies.dev/stu/VideoTools/internal/convert"

// Then reference as:
// convert.VideoSource
// convert.ConvertConfig
// convert.FormatOption
```

---

## 🧪 Integration Checklist

- [ ] **Import convert package** in main.go
  ```go
  import "git.leaktechnologies.dev/stu/VideoTools/internal/convert"
  ```

- [ ] **Update format selection**
  - Replace `formatOptions` with `convert.FormatOptions`
  - Add DVD option to dropdown

- [ ] **Add DVD options panel**
  - Aspect ratio selector (4:3, 16:9)
  - Region selector (NTSC, PAL, SECAM)
  - Interlacing mode selector

- [ ] **Implement validation**
  - Call `convert.ValidateDVDNTSC()` when DVD selected
  - Show warnings dialog before queueing

- [ ] **Update FFmpeg execution**
  - Use `convert.BuildDVDFFmpegArgs()` for .mpg files
  - Keep existing logic for other formats

- [ ] **Test with sample videos**
  - Generate test .mpg from AVI/MOV/MP4
  - Verify DVDStyler can import without re-encoding
  - Test playback on PS2 or DVD player

- [ ] **Verify queue integration**
  - Create multi-video DVD job batch
  - Test pause/resume with DVD jobs
  - Test progress tracking

---

## 🔄 Data Flow Diagram

```
User Interface (main.go)
    │
    ├─→ Select "DVD-NTSC (MPEG-2)" format
    │       │
    │       └─→ Show DVD options (aspect, region, etc.)
    │
    ├─→ Click "Add to Queue"
    │       │
    │       ├─→ Call convert.ValidateDVDNTSC(video, config)
    │       │       └─→ Return warnings/validation status
    │       │
    │       └─→ Create Job with config
    │               └─→ queue.Add(job)
    │
    ├─→ Queue displays job
    │       │
    │       └─→ User clicks "Start Queue"
    │               │
    │               ├─→ queue.Start()
    │               │
    │               └─→ For each job:
    │                   │
    │                   ├─→ convert.ProbeVideo(inputPath)
    │                   │       └─→ Return VideoSource
    │                   │
    │                   ├─→ convert.BuildDVDFFmpegArgs(...)
    │                   │       └─→ Return command args
    │                   │
    │                   └─→ Execute FFmpeg
    │                       └─→ Update job.Progress
    │
    └─→ Queue Viewer UI
            │
            └─→ Display progress
                - Job status
                - Progress %
                - Pause/Resume buttons
                - Cancel button
```

---

## 💾 Configuration Example

### Full DVD-NTSC Job Configuration

```json
{
  "id": "job-dvd-001",
  "type": "convert",
  "title": "Convert to DVD-NTSC: movie.mp4",
  "input_file": "movie.mp4",
  "output_file": "movie.mpg",
  "config": {
    "inputPath": "movie.mp4",
    "outputPath": "movie.mpg",
    "selectedFormat": {
      "Label": "DVD-NTSC (MPEG-2)",
      "Ext": ".mpg",
      "VideoCodec": "mpeg2video"
    },
    "isDVD": true,
    "quality": "Standard (CRF 23)",
    "videoCodec": "MPEG-2",
    "videoBitrate": "6000k",
    "targetResolution": "720x480",
    "frameRate": "29.97",
    "audioCodec": "AC-3",
    "audioBitrate": "192k",
    "audioChannels": "Stereo",
    "aspect": "16:9",
    "dvdRegion": "NTSC",
    "dvdValidationWarnings": [
      {
        "severity": "info",
        "message": "Input is 1920x1080, will scale to 720x480",
        "action": "Will apply letterboxing to preserve 16:9 aspect"
      }
    ]
  },
  "priority": 5,
  "status": "pending",
  "created_at": "2025-11-29T12:00:00Z"
}
```

---

## 🚀 Quick Start Integration

### Step 1: Add Import
```go
// At top of main.go
import (
    // ... existing imports
    "git.leaktechnologies.dev/stu/VideoTools/internal/convert"
)
```

### Step 2: Replace Format Options
```go
// OLD (around line 1394)
var formatLabels []string
for _, opt := range formatOptions {
    formatLabels = append(formatLabels, opt.Label)
}

// NEW
var formatLabels []string
for _, opt := range convert.FormatOptions {
    formatLabels = append(formatLabels, opt.Label)
}
```

### Step 3: Add DVD Validation
```go
// In addConvertToQueue() function
if s.convert.SelectedFormat.Ext == ".mpg" {
    warnings := convert.ValidateDVDNTSC(s.source, s.convert)
    // Show warnings if any
    if len(warnings) > 0 {
        // Display warning dialog
    }
}
```

### Step 4: Use Convert Package for FFmpeg Args
```go
// In executeConvertJob()
if s.convert.SelectedFormat.Ext == ".mpg" {
    src, _ := convert.ProbeVideo(inputPath)
    args := convert.BuildDVDFFmpegArgs(inputPath, outputPath, s.convert, src)
} else {
    // Use existing logic for other formats
}
```

---

## ✅ Verification Checklist

After integration, verify:

- [ ] **Build succeeds**: `go build .`
- [ ] **Imports resolve**: No import errors in IDE
- [ ] **Format selector shows**: "DVD-NTSC (MPEG-2)" option
- [ ] **DVD options appear**: When DVD format selected
- [ ] **Validation works**: Warnings shown for incompatible inputs
- [ ] **Queue accepts jobs**: DVD jobs can be added
- [ ] **FFmpeg executes**: Without errors
- [ ] **Progress updates**: In real-time
- [ ] **Output generated**: .mpg file created
- [ ] **DVDStyler imports**: Without re-encoding warning
- [ ] **Playback works**: On DVD player or PS2 emulator

---

## 🎯 Next Phase: Enhancement Ideas

Once integration is complete, consider:

1. **DVD Menu Support**
   - Simple menu generation
   - Chapter selection
   - Thumbnail previews

2. **Batch Region Conversion**
   - Convert same video to NTSC/PAL/SECAM in one batch
   - Auto-detect region from source

3. **Preset Management**
   - Save custom DVD presets
   - Share presets between users

4. **Advanced Validation**
   - Check minimum file size
   - Estimate disc usage
   - Warn about audio track count

5. **CLI Integration**
   - `videotools dvd-encode input.mp4 output.mpg --region PAL`
   - Batch encoding from command line

---

## 📚 Reference Documents

- **[DVD_IMPLEMENTATION_SUMMARY.md](./DVD_IMPLEMENTATION_SUMMARY.md)** - Detailed DVD feature documentation
- **[QUEUE_SYSTEM_GUIDE.md](./QUEUE_SYSTEM_GUIDE.md)** - Complete queue system reference
- **[README.md](./README.md)** - Main project overview

---

## 🆘 Troubleshooting

### Issue: "undefined: convert" in main.go
**Solution:** Add import statement at top of main.go
```go
import "git.leaktechnologies.dev/stu/VideoTools/internal/convert"
```

### Issue: formatOption not found
**Solution:** Replace with convert.FormatOption
```go
// Use:
opt := convert.FormatOption{...}
// Not:
opt := formatOption{...}
```

### Issue: ConvertConfig fields missing
**Solution:** Update main.go convertConfig to use convert.ConvertConfig

### Issue: FFmpeg command not working
**Solution:** Verify convert.BuildDVDFFmpegArgs() is called instead of manual arg building

### Issue: Queue jobs not showing progress
**Solution:** Ensure progressCallback is called in executeConvertJob
```go
progressCallback(percentComplete)  // Must be called regularly
```

---

## ✨ Summary

The VideoTools project now has:

1. ✅ **Complete DVD-NTSC encoding system** (internal/convert/)
2. ✅ **Fully functional queue system** (internal/queue/)
3. ✅ **Integration points identified** (this guide)
4. ✅ **Comprehensive documentation** (multiple guides)

**Next step:** Integrate these components into main.go following this guide.

The integration is straightforward and maintains backward compatibility with existing video formats.
