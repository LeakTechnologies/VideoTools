# Burn Module Design

## Overview

The **Burn** module provides DVD/BD disc burning functionality from ISO files using OS-native tools only. No FFmpeg or other external dependencies required.

## Features

### Phase 1: Basic Disc Burning
- [ ] Burn ISO to disc (DVD/BD)
- [ ] Select target drive
- [ ] Show burn progress

### Phase 2: Advanced Features
- [ ] Burn multiple ISOs in queue
- [ ] Burn speed selection
- [ ] Eject after burn
- [ ] Handle disc types (DVD-R, DVD+R, DVD-RW, DVD+RW, BD-R, BD-RE)

## Technical Implementation

### Entry Points
- Main menu button (Disc category)
- Queue "Burn DVD" button for author jobs with ISO output
- Drag & drop ISO files

### UI Components
1. **Source Selection**
   - ISO file picker (drag & drop supported)
   - Recent sources list

2. **Drive Selection**
   - Auto-detect writable drives
   - Show disc capacity (DVD5, DVD9, BD25, BD50)

3. **Burn Options**
   - Burn speed (1x, 2x, 4x, max)
   - Eject disc when complete

4. **Progress Display**
   - Overall progress bar
   - Current speed
   - Time remaining

### Command Line Tools (OS-Native Only)

#### Windows
- **isoburn.exe** (built-in since Windows 8, no external dependencies)
  ```
  isoburn /q D:\path\to\disc.iso E:
  ```
  - `/q` - quiet mode (no UI)
  - First param: ISO path
  - Second param: drive letter

#### Linux
- **growisofs** (from dvd+rw-tools package, most Linux distributions include this)
  ```
  growisofs -Z /dev/dvd -V "DISC_LABEL" -r -J /path/to/disc.iso
  ```
  - `-Z` - burn/initialise
  - `-V` - volume label
  - `-r` - make rock ridge extensions
  - `-J` - make Joliet directory

### Architecture

```go
type BurnConfig struct {
    Source      string   // ISO file path
    Drive       string   // Target drive letter (Windows) or device (Linux, e.g. /dev/sr0)
    Speed       int      // Burn speed (0 = auto/max)
    Eject       bool     // Eject when complete
    Copies      int      // Number of copies (future)
}

type BurnProgress struct {
    Written  int64   // bytes written
    Total    int64   // total bytes
    Speed    float64 // MB/s
    ETA      time.Duration
}
```

### Drive Detection

#### Windows
- Use `GetDriveType()` Win32 API
- Filter for `DRIVE_CDROM` type
- Check for writable media using `DeviceIoControl`

#### Linux
- Scan `/dev/sr*` and `/dev/dvd*` for writable devices
- Use `sysfs` or `ioctl` to check disc status
- Common paths: `/dev/sr0`, `/dev/sr1`, `/dev/dvd`, `/dev/dvd1`

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

- [ ] Burn ISO to blank DVD-R
- [ ] Handle disc full error gracefully
- [ ] Handle write error and retry
- [ ] Test with DVD+R, DVD-RW, DVD+RW
- [ ] Test BD burning (if applicable)

## Dependencies

- **Windows**: isoburn.exe (built-in since Windows 8, no external deps)
- **Linux**: growisofs (from dvd+rw-tools, included in most distros)

## Notes

- No FFmpeg required for burning
- Use OS-native tools only
- ISOs must be pre-created (use Author module)
- No post-burn verification (not supported by OS tools)
