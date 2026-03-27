# Burn Module Design

## Overview

The **Burn** module provides DVD/BD disc burning functionality from ISO files or VIDEO_TS folders.

## Features

### Phase 1: Basic Disc Burning
- [ ] Burn DVD ISO to disc
- [ ] Burn VIDEO_TS folder to disc
- [ ] Select target drive
- [ ] Show burn progress

### Phase 2: Advanced Features
- [ ] Burn multiple ISOs in queue
- [ ] Verify after burn
- [ ] Burn speed selection
- [ ] Eject after burn

## Technical Implementation

### Entry Points
- Main menu button (Disc category)
- Queue "Burn DVD" button for author jobs with ISO output
- Drag & drop ISO files

### UI Components
1. **Source Selection**
   - ISO file picker
   - VIDEO_TS folder picker
   - Recent sources list

2. **Drive Selection**
   - Auto-detect writable drives
   - Show disc capacity (DVD5, DVD9, BD25, BD50)
   - Show available space

3. **Burn Options**
   - Burn speed (1x, 2x, 4x, max)
   - Verify written data
   - Eject disc when complete
   - Number of copies

4. **Progress Display**
   - Overall progress bar
   - Current speed
   - Time remaining
   - Write method (dao, incremental)

### Command Line Tools

#### Windows
- **isoburn.exe** (built-in since Windows 8)
  ```
  isoburn /q D:\path\to\disc.iso E:
  ```
- **cdrecord** (via cdrtools)
- **imgburn** (optional, if installed)

#### Linux
- **growisofs** (dvd+rw-tools)
  ```
  growisofs -Z /dev/dvd -V "DVD_LABEL" -r -J /path/to/VIDEO_TS
  ```
- **cdrecord** (cdrtools)

### Architecture

```go
type BurnConfig struct {
    Source      string   // ISO path or VIDEO_TS folder
    Drive       string   // Target drive letter (Windows) or device (Linux)
    Speed       int      // Burn speed (0 = auto/max)
    Verify      bool     // Verify written data
    Eject       bool     // Eject when complete
    Copies      int      // Number of copies
}

type BurnProgress struct {
    Written  int64   // bytes written
    Total    int64   // total bytes
    Speed    float64 // MB/s
    ETA      time.Duration
}
```

### Queue Integration
- Add `JobTypeBurn` to queue types
- `OnBurnISO` callback wired in queue module
- Progress updates via queue job progress

## Files to Modify

1. **`main.go`**
   - Add `burn` module entry in `modulesList`
   - Implement `showBurnView()` function
   - Add `executeBurnJob()` for queue

2. **`queue_module.go`**
   - Wire `OnBurnISO` to burn module

3. **`internal/ui/`**
   - Create `burn.go` for burn UI components

4. **i18n**
   - Add ModuleBurn strings
   - Add burn-specific dialog strings

## Testing Checklist

- [ ] Burn DVD ISO to blank DVD-R
- [ ] Burn VIDEO_TS folder to blank DVD-R
- [ ] Verify written data matches source
- [ ] Handle disc full error gracefully
- [ ] Handle write error and retry
- [ ] Test with DVD+R, DVD-RW, DVD+RW
- [ ] Test BD burning (if applicable)

## Dependencies

- Windows: isoburn.exe (built-in since Windows 8)
- Linux: dvd+rw-tools (growisofs)

## Notes

- No FFmpeg required for burning
- Use OS-native tools only
- Support both DAO (disc-at-once) and incremental
