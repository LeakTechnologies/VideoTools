# DVD IFO Troubleshooting Guide

This document captures known issues with DVD-Video IFO file generation and playback compatibility.

## RESOLVED (2026-07-08): TMAP zero_1 Validation Failure

**Root cause:** `WriteTMAPT` in `internal/dvd/ifo/vtsi.go` wrote the TMAP
header fields in the order `zero_1, Time_Unit`, but libdvdread's `vts_tmap_t`
is `{ uint8 tmu; uint8 zero_1; ... }` — Time_Unit comes **first**. With
Time_Unit=1 the player read `zero_1 = 0x01`, producing the error below. The
hexdump "verification" in the Verified Working Structures section used the
same swapped assumption, which is why the structure looked correct. Fixed by
swapping the two bytes; see `TestWriteTMAPT_EntryHeader`. The analysis below
is retained for historical context.

## Original Issue: TMAP zero_1 Validation Failure

### Error Message
```
dvdnav error: Zero check failed in src/ifo_read.c:1537 for vts_tmapt->tmap[i].zero_1 : 0x01
```

### Root Cause Analysis

The libdvdread library validates DVD-Video IFO structures using zero-check assertions. When `zero_1` fields contain non-zero values, playback fails.

#### Our Current TMAPT Implementation

 Located in `internal/dvd/ifo/vtsi.go`, the `WriteTMAPT` function generates:

```
[0-1]   NrOf_VTS_TMAPTs = 1 (uint16)
[2-3]   Reserved (uint16)
[4-7]   EndByte (uint32)
[8-11]  TMAP[0] start byte offset = 12 (uint32)
[12]    zero_1 (uint8, must be 0)
[13]    Time_Unit (uint8)
[14-15] NrOf_Entries (uint16)
[16+]   Entries (uint32 each, bit31=ECCE, bits0-30=sector)
```

#### Suspected Problem

libdvdread's `ifo_read.c:1537` checks `tmap[i].zero_1` per-entry, not just the header. This suggests one of:

1. **Per-entry reserved field** - Each TMAP entry may need its own `zero_1` byte, not just the header
2. **Sector encoding issue** - Our sector values may have bits set in reserved positions
3. **Structure misalignment** - The TMAP offset calculation may be wrong, causing libdvdread to read the wrong bytes

### Libdvdread TMAP Entry Structure

According to DVD-Video spec (referenced in libdvdread source):

```c
typedef struct {
    uint8_t zero_1;      // MUST be 0
    uint8_t time_unit;   // Time granularity
    uint16_t nr_of_entries;
    // Followed by entries...
} vts_tmapt_t;
```

Each TMAP entry structure may include:
```c
typedef struct {
    uint32_t sector;     // bits 0-30 = sector address, bit 31 = ECCE flag} vts_tmap_entry_t;
```

### Debugging Steps

1. **Dump the generated IFO bytes** at the TMAPT offset to verify structure
2. **Compare with a known-working IFO** from a commercial DVD
3. **Check sector values** - ensure no reserved bits are set
4. **Verify TMAPT offset** in VTS_MAT matches actual file position

### Testing Commands

```bash
# Hexdump the TMAPT region from generated IFO
xxd -s $((0x800 + TMAPT_OFFSET)) -l 256 VTS_01_0.IFO

# Compare with reference IFO
xxd -s $((0x800 + TMAPT_OFFSET)) -l 256 reference.VTS_01_0.IFO
```

### Resolution Status

**RESOLVED 2026-07-08** — header field order was swapped (see top of file).
Original investigation notes:

- [ ] Libdvdread source at ifo_types.h for exact TMAP entry layout
- [ ] DVD-Video specification for TMAPT structure
- [ ] Generated IFO binary verification
- [ ] Cross-reference with dvdauthor output

---

## CSS Decryption Errors on Authored VIDEO_TS

### Error Messages
```
dvdnav error: Could not open ... with libdvdcssdvdnav error: Device ... inaccessible, CSS authentication not available.
```

### Cause

This error occurs when attempting to open a freshly-authored VIDEO_TS folder as a DVD source. Libdvdcss attempts CSS authentication because:

1. The path is a directory (not an ISO)
2. No CSS key sector is present (newly authored, no encryption)
3. VLC/libdvdnav assumes it needs CSS keys

### Solution

For authored VIDEO_TS folders:

1. **Use ISO output** - ISO files work better with VLC menu playback
2. **Skip CSS authentication** - Configure libdvdnav to not expect CSS on authored content
3. **Region-free by design** - Archival discs should be region-free (0x00FFFFFE)

### Implementation Notes

The `internal/dvd/css/` package handles CSS detection and decryption for source DVDs. For authored output, CSS should never be applied - the content is already unencrypted.

---

## IFO Structure Validation Checklist

When generating DVD-Video IFO files, verify:

### VTS_MAT (Video Title Set Manager)

| Offset | Field | Value | Notes |
|--------|-------|-------|-------|
| 0x000 | Identifier | "DVDVIDEO-VTS" | 12 bytes ASCII |
| 0x0D4 | VTS_TMAPTI_Offset | Sector offset| Must point to valid TMAPT |
| 0x0E4 | VTS_VOBU_ADMAP_Offset | Sector offset | Must be > TMAPT offset |

### VTS_TMAPT (Time Map Table)

| Offset | Field | Value | Notes |
|--------|-------|-------|-------|
| 0x00 | NrOf_VTS_TMAPTs | 1 | uint16 |
| 0x02 | Reserved | 0 | uint16 |
| 0x04 | EndByte | Last byte+1 | uint32 |
| 0x08 | TMAP[0] offset | 12 | uint32 |
| 0x0C | zero_1 | 0 | uint8, **CRITICAL** |
| 0x0D | Time_Unit | 1-60 | uint8, seconds per entry |
| 0x0E | NrOf_Entries | Entry count | uint16 |

### Per-Entry Structure (Suspected Issue)

Each TMAP entry may need:
- 4 bytes: sector address (bit31=ECCE, bits0-30=sector)

**IMPORTANT**: The `zero_1` check failure suggests libdvdread expects `zero_1` to be 0 but reads `0x01`. This could mean:

1. The TMAPT offset in VTS_MAT is incorrect, pointing to wrong data
2. The sector values have unexpected bit patterns
3. There's a padding/alignment issue

---

## Reference: LibdvdreadSource Paths

Key files in libdvdread for IFO validation:

- `src/ifo_read.c` - IFO parsing with zero-check assertions
- `src/ifo_types.h` - Structure definitions
- `src/ifo_print.c` - Debug output for IFO structures

### Line 1537 Check

In`ifo_read.c`, the zero_1 check typically looks like:

```c
if(tmap->zero_1 != 0) {
    fprintf(stderr, "Zero check failed for vts_tmapt->tmap[%d].zero_1 : %02x\n", i, tmap->zero_1);
}
```

This happens during `ifoRead_VTS_TMAPT()` which parses the time map table.

---

## Future Work

1. **Add IFO dump tool** - Create `cmd/videotools/ifo-dump` to inspect generated files
2. **Compare with reference** - Generate IFO from dvdauthor and diff against ours
3. **Unit test real files** - Add compliance tests that parse generated IFOs with libdvdread
4. **Validate with VLC** - Automated test that opens generated VIDEO_TS in VLC

---

## Related Files

- `internal/dvd/ifo/vtsi.go` - VTS_TMAPT serialization
- `internal/dvd/ifo/builder.go` - IFO generation pipeline
- `internal/dvd/ifo/mat_serialize.go` - VTS_MAT serialization
- `author_module.go` - Menu PGC and VOB assembly

---

## VLCTesting Notes

### Opening VIDEO_TS vs ISO

VLC may fail to open a VIDEO_TS folder if:
1. Path contains spaces or special characters
2. Path is too deep
3. VLC tries libdvdcss authentication first (fails on non-encrypted folders)

**Recommended testing:**
1. Open ISO output instead of VIDEO_TS folder
2. Or open the VIDEO_TS subfolder directly (not parent):`...\disc\VIDEO_TS`, not `...\disc`
3. Clear VLC cache before re-testing

### Verified Working Structures

Our IFO dump shows correct values:

```
=== VTS_01_0.IFO ===
VTS_TMAPTI:        3
NrOf_VTS_TMAPTs:  1
zero_1:            0x00 ✓
Time_Unit:         1 seconds
NrOf_Entries:      705
```

The TMAPT structure is DVD-compliant. If VLC shows `zero_1 : 0x01`, the error may be:
- VLC reading from wrong file position
- VLC caching old data
- Path resolution issue opening parent folder instead of VIDEO_TS

## Change History

- 2026-04-07: Initial documentation of TMAP zero_1 failure
- 2026-04-07: Verified our IFO has correct zero_1=0x00; added VLC testing notes