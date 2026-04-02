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

### Phase 3: Multi-Drive Batch Burning
- [ ] Detect all writable optical drives on the system
- [ ] Assign ISOs to specific drives (auto or manual)
- [ ] Simultaneous parallel burning across multiple drives
- [ ] Per-drive progress tracking and status display
- [ ] Batch mode: burn a multi-volume set (e.g., Disc 1-4) across available drives
- [ ] Drive health monitoring — skip failed drives, retry on error
- [ ] Post-burn verification (read-back checksum comparison)
- [ ] Auto-eject and prompt for next blank disc (for single-drive sequential mode)

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

### Multi-Drive Batch Burning

For users with disc replicators or multi-drive systems, the burn module supports **parallel batch burning** — burning multiple discs simultaneously across all available optical drives.

#### Use Cases
- **Multi-volume sets** — A 4-disc compilation burned all at once, one disc per drive.
- **Small-line production** — Duplicate copies of the same disc across multiple drives (e.g., 10 copies across 4 drives = 3 rounds).
- **Distribution runs** — Burn a batch of different discs (e.g., training materials, event footage) in a single operation.

#### Architecture

```go
// BatchBurnJob represents a multi-drive burn operation
type BatchBurnJob struct {
    ISOs        []string          // ISO files to burn (ordered: Disc 1, Disc 2, ...)
    DriveMap    map[int]string    // disc index → drive path (auto-assigned if nil)
    Speed       int               // burn speed for all drives (0 = auto)
    Eject       bool              // eject all discs when complete
    Verify      bool              // verify each disc after burning
    Copies      int               // number of copies per ISO (1 = single copy)
}

// DriveStatus tracks the state of a single drive during batch burn
type DriveStatus struct {
    Path        string       // drive path
    ISO         string       // current ISO being burned
    Progress    BurnProgress // per-drive progress
    State       string       // "idle", "burning", "verifying", "done", "error"
    Error       error        // last error if any
}
```

#### Drive Assignment

When a batch burn starts, drives are auto-assigned:
1. **Detect** all writable optical drives (`/dev/sr0`, `/dev/sr1`, etc. on Linux; `\\.\CDROM0`, `\\.\CDROM1`, etc. on Windows)
2. **Filter** out drives with discs already inserted (unless overwriting)
3. **Assign** ISOs to drives in order (Disc 1 → Drive 0, Disc 2 → Drive 1, etc.)
4. **Queue** remaining ISOs if fewer drives than discs — as each drive finishes, it picks up the next ISO

#### UI Design

```
┌─────────────────────────────────────────────────────────────────────┐
│ Batch Burn — 4 Discs                                               │
│ Source: /home/stu/DVD_Convert/Show_S01/                            │
│ Copies: [1]  Speed: [Auto ▼]  [✓] Verify  [✓] Eject               │
├─────────────────────────────────────────────────────────────────────┤
│ Drive       │ Disc       │ Progress      │ Status    │ Time Left   │
├─────────────────────────────────────────────────────────────────────┤
│ /dev/sr0    │ Disc 1     │ ████████░░ 78%│ Burning   │ 2:15        │
│ /dev/sr1    │ Disc 2     │ ██████████ 100%│ Verifying │ 0:45        │
│ /dev/sr2    │ Disc 3     │ ███░░░░░░░ 25%│ Burning   │ 6:30        │
│ /dev/sr3    │ Disc 4     │ ───────────  0%│ Waiting   │ —           │
├─────────────────────────────────────────────────────────────────────┤
│ [Start Batch] [Pause] [Cancel]                                     │
└─────────────────────────────────────────────────────────────────────┘
```

#### Sequential Mode (Single Drive)

For users with only one drive, batch burning falls back to sequential mode:
1. Burn Disc 1 → eject → prompt for blank disc
2. User inserts blank → Burn Disc 2 → eject → prompt
3. Continue until all discs complete

#### Production Mode (Replicator)

For hardware disc replicators (e.g., Aleratec, PrimeArray):
- Detect replicator via USB/HID interface
- Send batch burn commands directly to replicator controller
- Monitor replicator status (slots loaded, burn progress, errors)
- This is a future extension — Phase 3 covers multi-drive parallel burning first.

#### Error Handling

- **Drive failure** — If a drive fails mid-burn, the job is marked failed for that disc; other drives continue.
- **Bad media** — If a blank disc is defective, the drive ejects and the user is prompted to insert a replacement.
- **Verification failure** — If read-back checksum doesn't match ISO, the disc is marked failed and the user can retry.
- **Partial batch** — If 3 of 4 discs complete successfully, the user can re-run the batch with only the failed disc(s).

#### Linux Multi-Drive Detection

```go
func detectWritableDrives() ([]string, error) {
    var drives []string
    // Scan /dev/sr* for optical drives
    for i := 0; i < 16; i++ {
        path := fmt.Sprintf("/dev/sr%d", i)
        if isWritableOpticalDrive(path) {
            drives = append(drives, path)
        }
    }
    return drives, nil
}
```

#### Windows Multi-Drive Detection

```go
func detectWritableDrives() ([]string, error) {
    var drives []string
    // Enumerate via IMAPI2 IDiscMaster2
    discMaster := com.NewIDiscMaster2()
    for _, recorderID := range discMaster.EnumRecorders() {
        recorder := com.NewIDiscRecorder2(recorderID)
        if recorder.IsWritableMediaPresent() {
            drives = append(drives, recorder.GetPath())
        }
    }
    return drives, nil
}
```

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
- [ ] Multi-drive detection (2+ drives)
- [ ] Parallel burn: 2 ISOs across 2 drives simultaneously
- [ ] Sequential mode: 4 ISOs on 1 drive with disc swap prompts
- [ ] Drive failure recovery: one drive fails, others continue
- [ ] Post-burn verification (read-back checksum)
- [ ] Batch burn with copies (e.g., 3 copies × 2 drives = 6 discs)

### Dependencies

- **Windows**: IMAPI2 COM interfaces via `golang.org/x/sys/windows`
- **Linux**: SG_IO ioctl via native `syscall` package

**No new dependencies added** - uses existing `golang.org/x/sys` (already in go.mod).

**FFmpeg is the core video processing dependency** and remains unchanged.
