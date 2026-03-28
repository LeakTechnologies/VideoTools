# Clone Disc Design

## Overview
Clone disc creates an exact sector-by-sector copy of a DVD or Blu-ray disc to an ISO file. Unlike ripping (which transcodes), clone produces a perfect backup that can be burned back to disc.

## Use Cases
1. **Perfect backup** - Keep an exact copy of original discs
2. **Archive** - Store disc library as ISO files
3. **No quality loss** - Raw sector copy, no re-encoding
4. **Disc imaging** - Create bootable or multi-disc ISOs

## Technical Implementation

### Windows
Use `isoburn.exe` (part of libburn) for sector-by-sector copy:
```
isoburn.exe /dev/sr0 output.iso
```

Or use PowerShell to mount and copy:
```powershell
$disk = Get-Volume -DriveLetter D
# Read raw sectors and write to ISO
```

### Linux
Use `dd` or direct read from `/dev/sr0`:
```bash
dd if=/dev/sr0 of=output.iso bs=2048
```

## UI Integration

### Option 1: Add to Rip Module
- Add "Clone Disc" button in Rip module
- Shows source selector (drive letter)
- Shows output path selector
- Progress bar for copy operation

### Option 2: Separate Clone Module
- Dedicated module for disc cloning
- Cleaner separation from rip/conversion

## Implementation Steps

1. **Add Clone button to Rip module**
   - Location: Near existing "Add to Queue" buttons
   - Label: "Clone Disc to ISO"

2. **Create CloneToISO function**
   - Input: Source drive/path
   - Output: ISO file path
   - Progress callback

3. **Wire to queue system**
   - New job type: `JobTypeClone`
   - Progress tracking
   - Logging

## Testing Checklist
- [ ] Clone DVD to ISO
- [ ] Verify ISO mounts correctly
- [ ] Compare file checksums
- [ ] Clone Blu-ray to ISO (if supported)
- [ ] Progress reporting works
- [ ] Error handling (disc not found, write errors)
