# Burn Module Design

## Overview

The **Burn** module provides DVD/BD disc burning functionality from ISO files using direct OS APIs. Uses Windows IMAPI2 on Windows and SG_IO ioctl on Linux.

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

### Command Line Tools

**Important**: There are no pre-installed CLI tools for disc burning on any OS. We must use direct OS API calls for a truly insular implementation.

#### Windows: IMAPI2 (Image Mastering API)
- Use COM interface `IMapiDiscMaster`, `IDiscRecorder2`
- Located in `golang.org/x/sys/windows` and `github.com/go-ole/go-ole`
- No external dependencies beyond standard Windows SDK bindings

#### Linux: ioctl via syscall
- Direct ioctl calls to `/dev/sr*` (SG_IO)
- No external dependencies - uses native `syscall` package

**No FFmpeg, no cdrtools, no growisofs, no external CLI tools.**

### Architecture

```go
// BurnConfig holds configuration for a burn job
type BurnConfig struct {
    Source      string   // ISO file path
    Drive       string   // Target drive path (Windows: \\.\CDROM0, Linux: /dev/sr0)
    Speed       int      // Burn speed in KB/s (0 = auto/max)
    Eject       bool     // Eject disc when complete
}

// BurnProgress holds progress information
type BurnProgress struct {
    Written  int64   // bytes written
    Total    int64   // total bytes
    Speed    float64 // MB/s
    ETA      time.Duration
    Status   string  // current status message
}
```

### Implementation Strategy

#### Windows (IMAPI2 via COM)
- Use `IMapiDiscMaster` to enumerate drives
- Use `IDiscRecorder2` to open drive and write
- `IStream` for ISO file data transfer
- Progress via `IWriteEngine2` callback interface
- Requires `golang.org/x/sys/windows` (already in deps)

#### Linux (SG_IO ioctl via syscall)
- Open `/dev/sr0` with `O_RDWR|O_NONBLOCK`
- Use `SG_IO` ioctl with SCSI commands for burning
- CDB commands: MODE SELECT, WRITE (10/12)
- Requires only standard `syscall` package

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

### Dependencies

- **Windows**: IMAPI2 COM interfaces via `golang.org/x/sys/windows`
- **Linux**: SG_IO ioctl via native `syscall` package

**No new dependencies added** - uses existing `golang.org/x/sys` (already in go.mod).

**FFmpeg is the core video processing dependency** and remains unchanged.
