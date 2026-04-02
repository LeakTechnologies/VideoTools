# DVD Menu System — Implementation Design

## Status: Active (dev39)

This document describes the current state of the DVD menu pipeline, identified
gaps, and the planned fixes required for software (VLC) and hardware player
compatibility.

---

## 1. Current Architecture

### 1.1 Menu Asset Generation (`author_menu.go`)

`buildDVDMenuAssets()` produces a `dvdMenuSet`:

| Field | Contents | Status |
|---|---|---|
| `MainMpg` | SPU-only VOB (subpicture + NAV_PCK, **no video**) | Broken |
| `MainButtons` | `[]dvdMenuButton` with X0/Y0/X1/Y1 and command | OK |
| `ChaptersMpgs` | Per-page SPU-only VOBs (same issue) | Broken |
| `ChaptersButtons` | Per-page button slices | OK |
| `ExtrasMpg` | SPU-only VOB for extras menu | Unused |
| `ExtrasButtons` | `[]dvdMenuButton` for extras | Unused |

`buildMenuSPU()` → `runNativeSpumux()` encodes the button overlay PNG as an
SPU subpicture stream and writes it with a zeroed NAV_PCK. The background PNG
(`menu_bg.png`) is rendered but **never encoded as MPEG-2 video**.

### 1.2 VIDEO_TS.VOB Assembly (`author_module.go:3472-3551`)

```
VIDEO_TS.VOB = concat(MainMpg, ChaptersMpgs[0], ChaptersMpgs[1], ...)
```

`ExtrasMpg` is generated but **never concatenated** and never added to the PGC
list. Extras buttons "jump title N" but the extras menu page is unreachable.

### 1.3 VMGM IFO Generation

The `menuPGCs` slice is built from main + chapter pages only. It is passed to
`WriteVMGM_PGCI_UT()` which wraps them in a single Language Unit.

**Missing from the ISO two-pass layout:**
- `vmgMat.VMGM_VOBS_Sector` is never set → player can't locate menu VOB
- Menu PGC `CellPlayback.FirstSector` / `LastSector` stay 0 → strict players fail

---

## 2. Identified Bugs and Gaps

### Bug A — Menu VOB contains no MPEG-2 video (CRITICAL)
**File:** `author_menu.go` → `runNativeSpumux()`

The function creates a VOB with only:
1. SPU pack (subpicture overlay)
2. Zeroed NAV_PCK

There is no video PES stream. Hardware players require a valid MPEG-2 video
stream. The background image must be encoded as a still-frame MPEG-2 video and
multiplexed into the VOB.

**Fix:** Call ffmpeg to encode `menu_bg.png` as a short MPEG-2 still video
(e.g., 10s loop at DVD spec resolution and framerate). Then write the VOB as:
`[NAV_PCK][video packs...][SPU pack]` using the native muxer, or mux with
ffmpeg and post-process to inject a valid NAV_PCK at sector 0.

The simplest production approach:
```
ffmpeg -loop 1 -i menu_bg.png -t <duration> -vcodec mpeg2video \
  -b:v 4000k -maxrate 9000k -bufsize 1835k \
  -s <width>x<height> -r <fps> -aspect <aspect> \
  -an menu_video.mpg
```
Then mux video + SPU into a single MPEG-2 PS. The output must have a NAV_PCK
at its first sector so the DVD player can navigate it.

A clean approach: use ffmpeg to produce the video-only MPEG-2, then use the
native `vob.Muxer` to write a proper VOB file:
`WriteNAV_PCK` → write video packs from ffmpeg output → `WriteSPU`

### Bug B — PCI button rectangles empty (CRITICAL for hardware)
**File:** `author_menu.go` → `runNativeSpumux()` and `internal/dvd/vob/nav.go`

The NAV_PCK written for the menu has `BTN_NS = 0` and no button coordinate
data in the PCI payload. Hardware players use these to render button highlight
regions and to know how many buttons exist.

The `PCIPacket` struct needs a `Buttons []PCIButton` field (or similar) and the
PCI serialization must write the button table at the correct PCI offset.

**DVD PCI button entry (on-disc, 18 bytes each):**
- `btn_coln` : 6-bit column flags (group 1/2 forced-select/activated)  
- `x_start`, `x_end` : horizontal pixel range (10 bits each)
- `auto_action_mode` : 1 bit
- `y_start`, `y_end` : vertical pixel range (10 bits each)
- `up_btn`, `down_btn`, `left_btn`, `right_btn` : neighbour button numbers (6 bits each)
- `cmd_nr` : cell command number to execute on select (8 bits)

The `dvdMenuButton` struct already has X0/Y0/X1/Y1. We need to map these into
the PCI button table and set `BTN_NS` to the button count.

### Bug C — `VMGM_VOBS_Sector` never set in ISO mode (HIGH)
**File:** `author_module.go` ~line 3660 (ISO two-pass layout)

The VMG_MAT field `vtsm_vobs` (at byte offset 0x0C0) must contain the
disc-absolute start sector of `VIDEO_TS.VOB`. Currently it is never set, so it
remains 0, and the player cannot find the menu VOB on disc.

**Fix:** In the ISO layout pass, after calling `PreAssignSectors()`:
```go
if first, _, ok := vtsSector("VIDEO_TS.VOB"); ok {
    vmgMat.VMGM_VOBS_Sector = first
}
```

### Bug D — Menu PGC CellPlayback sectors stay 0 (MEDIUM)
**File:** `author_module.go` ~line 3712, `internal/dvd/ifo/pgc.go:387`

`BuildMenuPGC()` creates CellPlayback entries with `FirstSector=0, LastSector=0`.
These must be patched to the actual sectors within `VIDEO_TS.VOB` before writing
the final IFO.

Since `VIDEO_TS.VOB` contains concatenated menu MPGs, the sector range for each
PGC can be computed at runtime from the cumulative file sizes:

```
PGC[0]: disc sector first .. first + sectors(MainMpg) - 1
PGC[1]: first + sectors(MainMpg) .. first + sectors(MainMpg) + sectors(ChMpg[0]) - 1
...
```

**Fix:** After `vtsSector("VIDEO_TS.VOB")` lookup, iterate menuPGCs and patch
each CellPlayback using cumulative sector offsets computed from the mpg file
sizes on disk before the ISO is built.

### Bug E — Extras menu disconnected from pipeline (MEDIUM)
**File:** `author_module.go:3472-3551`

`menuSet.ExtrasMpg` is generated but never:
1. Concatenated into `VIDEO_TS.VOB`
2. Added to `menuPGCs` as a PGC
3. Reachable from the main menu (inter-PGC jump command is NOP)

The main menu "Extras" button currently issues a `JumpTT` command — but it
should navigate to the extras menu PGC (a VMGM domain PGC, not a title jump).

**Fix:**
1. Append `menuSet.ExtrasMpg` to `menuFiles` before `VIDEO_TS.VOB` is built
2. Build and append an extras PGC to `menuPGCs`
3. Implement `JumpMenuPGCCommand(pgcN int)` in `commands.go`:
   - DVD VM opcode for "call/jump to menu PGC N in current VMGM" is known
4. Update main menu extras button command to use `JumpMenuPGCCommand(extrasPageN)`

---

## 3. Button Navigation on the Main Menu

`buildDVDMenuButtons()` creates the main menu buttons. When extras exist, it
adds an "Extras" button. These buttons currently use "jump title N" commands.

The correct commands for a multi-PGC VMGM menu are:

| Button | Target | DVD VM Instruction |
|---|---|---|
| "Play" | Title 1 | `JumpTT(1)` (0x30 opcode) |
| "Chapters" | Chapters PGC (PGC 2) | `JumpVMGM_PGCN(2)` |
| "Extras" | Extras PGC (last PGC) | `JumpVMGM_PGCN(N)` |
| Each extras button | Title N | `JumpTT(N)` |

The `JumpVMGM_PGCN` opcode (jump to VMGM PGC N): encoding from dvdauthor reference:
`0x30 0x06 0x00 0x00 0x00 0x00 PGC_LO PGC_HI` (big-endian PGC number).
*(Verify against `dvdnav/read_cache.h` or dvdauthor source before implementing.)*

---

## 4. Target State After Fixes

A disc authored with menus will produce a `VIDEO_TS.VOB` containing:
```
[NAV_PCK with BTN_NS=N, button rects]
[MPEG-2 video packs — background still]
[SPU packs — button highlight overlay]
```

The `VIDEO_TS.IFO` will contain:
```
VMGM_VOBS_Sector → disc sector of VIDEO_TS.VOB start
VMGM_PGCI_UT:
  PGC 1 (main menu):   cells → VIDEO_TS.VOB[0 .. mainMenuSectors-1]
  PGC 2 (chapters p1): cells → VIDEO_TS.VOB[mainMenuSectors .. +chP1Sectors-1]
  ...
  PGC N (extras):      cells → VIDEO_TS.VOB[... last sections]
```

---

## 5. Work Items

| ID | Description | File(s) | Agent |
|---|---|---|---|
| M1 | Encode menu background as MPEG-2 still video via ffmpeg | `author_menu.go` | Agent A |
| M2 | Write proper DVD VOB with video+SPU muxed correctly | `author_menu.go`, `vob/nav.go` | Agent A |
| M3 | Populate PCI button rectangles in NAV_PCK | `vob/nav.go`, `author_menu.go` | Agent A |
| M4 | Set `VMGM_VOBS_Sector` from ISO disc layout | `author_module.go` | Agent B |
| M5 | Patch menu PGC cell sectors from `VIDEO_TS.VOB` disc location | `author_module.go` | Agent B |
| M6 | Wire `ExtrasMpg`/`ExtrasButtons` into `VIDEO_TS.VOB` and `menuPGCs` | `author_module.go` | Agent B |
| M7 | Implement `JumpVMGM_PGCN` command; update extras/chapters button commands | `ifo/commands.go`, `author_module.go` | Agent B |

---

## 6. Testing Checklist

After fixes:
- [ ] VLC opens disc and shows menu background image (not black screen)
- [ ] Button highlights render over correct regions
- [ ] "Play" button starts title 1 playback
- [ ] "Chapters" button shows chapter menu (if chapters > 1)
- [ ] "Extras" button shows extras menu (if extras present)
- [ ] Chapter menu buttons jump to correct chapter of title 1
- [ ] Extras menu buttons play correct extra title
- [ ] Hardware player (or handbrake/vlc disc scan) accepts IFO without errors
