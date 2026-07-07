# Author Module & DVD Menu System — Code Audit (2026-07)

Audit of `author_module.go`, `author_menu.go`, `internal/dvd/vob/`, `internal/dvd/ifo/`
against the DVD-Video spec as implemented by libdvdread/libdvdnav (`ifo_types.h`,
`nav_read.c`) and dvdauthor reference output. Scope: the lingering "menus display
but don't fully work" class of issues. All M1–M7 items from
`DVD_MENU_SYSTEM_DESIGN.md` are implemented; the failures below are in the
byte-level details of those implementations.

**Verification note:** structure offsets cited below are from libdvdread's
`ifo_types.h`/`nav_read.c` layouts. Before fixing, re-verify each against the
actual libdvdread source; the audit's own confidence per finding is marked.

---

## CRITICAL — menu buttons cannot work on any compliant player

### A1. PCI highlight block written at the wrong offset (confidence: high)
`internal/dvd/vob/nav.go:24-41` assumes `hl_gi` starts at PCI offset **68**
("32-byte pci_gi + 36-byte nsml_agli"). Real `pci_gi_t` is **60 bytes**
(nv_pck_lbn 4, vobu_cat 2, zero 2, uop_ctl 4, s_ptm 4, e_ptm 4, se_e_ptm 4,
e_eltm 4, vobu_isrc 32). `hli_t` therefore starts at **96**, not 68:

| Field | Code writes at | Spec offset |
|---|---|---|
| hli_ss (HL status) | never written | 96–97 |
| hli_s_ptm / e_ptm / btn_se_e_ptm | never written | 98–109 |
| btngr_ns + display types | never written | 110–111 |
| btn_ns | 95 | 113 |
| fosl_btnn | 96 | 116 |
| btn_colit (color table, 24 B) | 70–93 | 118–141 |
| btnit[0] (button entries) | 98 | **142** |

Because `hli_ss` is never set non-zero, players skip highlight processing
entirely even if the offsets were right. Both `WriteNAV_PCK` (nav.go:132-233)
and `PatchVOBPCI` (nav.go:363-426) share this layout.

### A2. Button entries use a nonexistent `cmd_nr` indirection (confidence: high)
`btni_t` is 18 bytes: 10 bytes of packed geometry/neighbours + **8-byte inline
VM command** (`vm_cmd_t cmd`). The code writes a 1-byte index into the PGC cell
command table at entry byte 9 and zeros bytes 10–17 (nav.go:230-232). The spec
has no such indirection — button activation executes the inline command.
`ParseButtonCommand` results must be serialized *into the button entry*, not
into `CommandTable.Cell`. The bit packing of geometry also differs from
`btni_t` (btn_coln occupies bits 7-6 of byte 0 followed by x_start[9:4];
auto_action_mode is 2 bits at the start of the y-group, not bit 7 of byte 0).

### A3. NAV PES packets missing the substream ID byte (confidence: high)
DVD NAV packs carry PCI/DSI in private_stream_2 (0xBF) where the **first
payload byte is the substream ID** (0x00 = PCI, 0x01 = DSI); PES length 0x03D4
(980) *includes* that byte (PCI data = 979 bytes; DSI = 0x03FA incl. ID, 1017
data). `WriteNAV_PCK` (nav.go:268-276) writes 980/1018 bytes of raw table data
with no ID byte. Demuxers identify DSI by `payload[0]==0x01` — our DSI starts
with the SCR high byte, so **the DSI is never recognized**, and PCI fields are
parsed one byte off. (`PatchVOBPCI` scans ffmpeg-muxed files which *do* have
the ID byte, so its `i+6` base is additionally one byte off from its own
intended layout.)

---

## CRITICAL — the documented "UNRESOLVED" TMAP failure: root cause found

### A4. TMAP header bytes swapped (confidence: high)
`internal/dvd/ifo/vtsi.go:105-106` writes `zero_1` **then** `Time_Unit`.
libdvdread `vts_tmap_t` is `{ uint8_t tmu; uint8_t zero_1; uint16_t
nr_of_entries; }` — **tmu first**. With TimeUnit=1 the player reads
`zero_1 = 0x01`, which is *exactly* the error recorded in
`DVD_IFO_TROUBLESHOOTING.md` ("Zero check failed … zero_1 : 0x01").
The doc's hexdump verification used the same swapped field order, which is why
the structure "looked correct". **Fix: swap the two bytes.** Close the
troubleshooting doc item when done.

---

## HIGH — sector addressing uses the wrong base in ISO mode

### A5. PGC cell sectors: disc-absolute vs domain-relative (confidence: high)
Cell `FirstSector`/`LastSector` are addresses **relative to the domain VOBS**
(VTSTT_VOBS for titles, VMGM_VOBS for menus), starting at 0. The folder path
gets this right by accident (`firstSector := uint32(0)`,
author_module.go:3689, with the comment claiming it's a placeholder). The ISO
path (author_module.go:3818-3859) patches **disc-absolute** sectors from the
UDF layout into both title and menu PGCs. Meanwhile TMAPT and VOBU_ADMAP stay
VOB-relative — so ISO output is internally inconsistent: cells point kilometres
past the VOBS while the time map points inside it. This is very likely why ISO
navigation/seek behaves differently from folder output.
**Fix: remove the disc-absolute patching; menu PGC N's cells are relative to
VIDEO_TS.VOB start (first menu = sector 0), title cells relative to
VTS_xx_1.VOB start — identical in both output modes.**

### A6. `VMGM_VOBS_Sector` wrong base in both modes (confidence: medium)
Should be the start of VIDEO_TS.VOB **relative to the VMG start** = size of
VIDEO_TS.IFO in sectors (i.e. `VMGI_Last_Sector + 1`). ISO mode writes the
disc-absolute sector (3812); folder mode writes `VMG_Last_Sector + 1` (3774),
which per spec includes VOB+BUP and points past the whole VMG.

### A7. `VMG_BUP_Last_Sector` set from menu VOB size (confidence: medium)
author_module.go:3661 stores `menuVOB.Size()/2048` into `VMG_BUP_Last_Sector`.
That field is the last sector of the backup IFO region, not the menu VOB size.

---

## HIGH — menu domain structures incomplete

### A8. VMGM_C_ADT / VMGM_VOBU_ADMAP never generated (confidence: high)
The offset fields exist (`vmgi.go:37-38`, serialized in `mat_serialize.go`)
but no code ever builds or writes the menu Cell Address Table or menu VOBU
address map. dvdauthor always emits both when a menu VOB exists; strict
players resolve menu cells through C_ADT.

### A9. DSI `vobu_vob_idn`/`vobu_c_idn` always zero; CellPosition all (1,1)
(confidence: high) Every `WriteNAV_PCK` call passes a zero `DSIPacket` — IDs
are 1-based, 0 is invalid. And every menu PGC's `CellPosition` claims
VOBID=1/CellID=1 (`pgc.go:486`) even though each concatenated menu MPG is a
distinct VOB. Cell-change detection and C_ADT mapping cannot work. Each menu
MPG should carry a distinct VOB_IDN (1..N), matching CellPosition and C_ADT.

### A10. Concatenation never rebases NAV LBN/SCR (confidence: high)
`concatenateMenuFile` (author_module.go:3991-4000) is a raw `io.Copy`. Menus
2..N keep `nv_pck_lbn` starting at 0 and restart SCR at 0 mid-file — wrong
LBNs within VIDEO_TS.VOB and non-monotonic SCR at every join. NAV packs in
appended menus need their `nv_pck_lbn` (PCI+DSI) rebased by the cumulative
sector offset, and ideally SCR made monotonic.

---

## MEDIUM

### A11. VMGM entry PGC category uses Root (0x83) instead of Title (0x82)
(confidence: medium) `WriteVMGM_PGCI_UT` writes SRP[0] category 0x8300 and LU
attribute 0x83. In the VMGM domain the entry menu is the **Title menu (0x82)**;
Root (0x83) belongs to the VTSM domain. Remote "Top Menu" resolution can fail.

### A12. First-Play PGC only exists when menus exist (confidence: high)
`VMG_FirstPlayPGC` is set only inside the `len(menuPGCs) > 0` branch
(builder.go:167-177). A menu-less disc gets FP_PGC = 0 and libdvdnav cannot
start it. Also, pointing FP at menu PGC 1 (with cells + still) is
unconventional; the robust pattern is a standalone command-only FP_PGC whose
pre-command is JumpSS→VMGM (menu) or JumpTT 1 (no menu).

### A13. No VTSM domain at all (design gap, note only)
VTS_MAT menu fields stay zero and there is no VTS menu VOB. Legal, but the
"Menu" remote key inside a title targets the VTS Root menu; consider a later
cycle adding a minimal VTSM that redirects to the VMGM.

---

## What already checks out

- SPU carriage (0xBD + substream 0x20), SPU RLE encoder with DCSQ/contrast.
- Menu background is genuinely encoded as MPEG-2 still video and muxed
  natively (M1/M2); extras menu wired end-to-end (M6); `JumpVMGM_PGCN`
  implemented (M7); `VMGM_VOBS_Sector`/cell patching hooks exist (M4/M5 —
  right hooks, wrong sector base per A5/A6).
- TT_SRPT, PTT_SRPT, VTS_ATRT, VOBU_ADMAP generation present with tests.
- PGC palette/subpicture-ctl/prohibited-ops values match dvdauthor patterns.

## Recommended fix order

1. **A4** TMAP byte swap (one-line; closes the documented unresolved bug)
2. **A3** NAV substream ID bytes (framing prerequisite for everything else)
3. **A1+A2** PCI HLI relocation + btni_t packing + inline button commands
4. **A5–A7** sector base corrections (domain-relative everywhere)
5. **A9+A10** VOB/Cell IDs + concatenation rebase
6. **A8** VMGM C_ADT + VOBU_ADMAP generation
7. **A11+A12** entry category + standalone FP_PGC
8. Re-run compliance tests; update `internal/dvd/vob/nav_test.go` and
   `compliance_test.go` expectations (they currently assert the wrong offsets).
9. Validate: VLC (dvdnav verbose), `dvdread`-based ifodump, and a hardware
   player if available. Update `DVD_IFO_TROUBLESHOOTING.md` change history.
