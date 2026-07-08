package ifo

import (
	"bytes"
	"encoding/binary"
	"math"

	"github.com/LeakTechnologies/VideoTools/internal/logging"
)

// pgcHeaderSize is the fixed byte size of a PGC header on disc.
const pgcHeaderSize = 236

// BuildSingleCellPGC creates a minimal one-program, one-cell PGC from sector
// addresses and duration. firstSector/lastSector are the absolute VOB sectors
// for the single cell.
func BuildSingleCellPGC(firstSector, lastSector uint32, durationSeconds float64, isNTSC bool) *ProgramChain {
	pgc := &ProgramChain{
		NrOfPrograms: 1,
		NrOfCells:    1,
		PlaybackTime: SecondsToPlaybackTime(durationSeconds, isNTSC),
		Programs:     []ProgramInfo{{EntryCell: 1}},
		CellPlayback: []CellPlayback{
			{
				PlaybackTime:        SecondsToPlaybackTime(durationSeconds, isNTSC),
				FirstSector:         firstSector,
				FirstILVUEndSector:  lastSector,
				LastVOBUStartSector: lastSector,
				LastSector:          lastSector,
			},
		},
		CellPosition: []CellPosition{
			{VOBID: 1, CellID: 1},
		},
	}
	return pgc
}

// ChapterCell defines the disc sector extent and duration of one chapter cell.
type ChapterCell struct {
	FirstSector uint32
	LastSector  uint32
	Duration    float64 // seconds
}

// ChapterCellsFromNAV builds chapter cell boundaries from NAV_PCK sector
// positions and chapter timestamps (in seconds).
//
// navSectors is the ordered list of all VOBU sector addresses within the VOB
// (as returned by ScanVOBForNAVPCKs, VOB-relative). timestamps is the list of
// chapter start times in seconds, starting from 0.0 for the first chapter.
// lastVOBSector is the index of the last sector of the VOB file.
// totalDuration is the total duration of the title in seconds.
//
// Returns nil if fewer than 2 chapters or no NAV_PCKs.
//
// Deprecated: Use ChapterCellsFromNAVPtm for accurate PTS-based mapping.
func ChapterCellsFromNAV(navSectors []uint32, timestamps []float64, totalDuration float64, lastVOBSector uint32) []ChapterCell {
	if len(timestamps) < 2 || len(navSectors) == 0 || totalDuration <= 0 {
		return nil
	}
	nVOBU := float64(len(navSectors))
	n := len(timestamps)
	cells := make([]ChapterCell, n)

	vobuIdxFor := func(ts float64) int {
		idx := int(ts / totalDuration * nVOBU)
		if idx < 0 {
			idx = 0
		}
		if idx >= len(navSectors) {
			idx = len(navSectors) - 1
		}
		return idx
	}

	for i, ts := range timestamps {
		cells[i].FirstSector = navSectors[vobuIdxFor(ts)]
		if i+1 < n {
			nextIdx := vobuIdxFor(timestamps[i+1])
			if nextIdx > 0 {
				cells[i].LastSector = navSectors[nextIdx] - 1
			} else {
				cells[i].LastSector = cells[i].FirstSector
			}
			cells[i].Duration = timestamps[i+1] - ts
		} else {
			cells[i].LastSector = lastVOBSector
			cells[i].Duration = totalDuration - ts
		}
	}
	return cells
}

// NavPCKInfo carries the sector address and presentation timestamp of one VOBU.
type NavPCKInfo struct {
	Sector uint32
	PTM    uint32 // 90kHz ticks
}

// ChapterCellsFromNAVPtm builds chapter cell boundaries using actual NAV_PCK
// presentation timestamps for accurate chapter-to-sector mapping.
//
// navs provides sector addresses and PTMs (in 90kHz ticks) for every VOBU.
// timestamps are chapter start times in seconds (first must be 0.0).
// lastVOBSector is the last sector of the VOB file.
// totalDuration is the full title duration in seconds.
func ChapterCellsFromNAVPtm(navs []NavPCKInfo, timestamps []float64, totalDuration float64, lastVOBSector uint32) []ChapterCell {
	if len(timestamps) < 2 || len(navs) == 0 || totalDuration <= 0 {
		return nil
	}

	vobuForTime := func(ts float64) int {
		targetPTM := uint32(ts * 90000)
		lo, hi := 0, len(navs)-1
		for lo < hi {
			mid := (lo + hi + 1) / 2
			if navs[mid].PTM <= targetPTM {
				lo = mid
			} else {
				hi = mid - 1
			}
		}
		return lo
	}

	n := len(timestamps)
	cells := make([]ChapterCell, n)
	for i, ts := range timestamps {
		idx := vobuForTime(ts)
		cells[i].FirstSector = navs[idx].Sector
		if i+1 < n {
			nextIdx := vobuForTime(timestamps[i+1])
			if nextIdx > idx {
				cells[i].LastSector = navs[nextIdx].Sector - 1
			} else {
				cells[i].LastSector = cells[i].FirstSector
			}
			cells[i].Duration = timestamps[i+1] - ts
		} else {
			cells[i].LastSector = lastVOBSector
			cells[i].Duration = totalDuration - ts
		}
	}
	return cells
}

// BuildChapterPGC creates a multi-program, multi-cell PGC where each cell
// corresponds to one chapter. This enables chapter navigation on hardware
// players when combined with a VTS_PTT_SRPT.
//
// cells must have at least one entry. For single-chapter content use
// BuildSingleCellPGC instead. totalDuration is the full title duration.
func BuildChapterPGC(cells []ChapterCell, totalDuration float64, isNTSC bool) *ProgramChain {
	n := len(cells)
	if n == 0 {
		return nil
	}
	programs := make([]ProgramInfo, n)
	cellPlayback := make([]CellPlayback, n)
	cellPosition := make([]CellPosition, n)

	for i, c := range cells {
		programs[i] = ProgramInfo{EntryCell: uint8(i + 1)}
		cellPlayback[i] = CellPlayback{
			PlaybackTime:        SecondsToPlaybackTime(c.Duration, isNTSC),
			FirstSector:         c.FirstSector,
			FirstILVUEndSector:  c.LastSector,
			LastVOBUStartSector: c.LastSector,
			LastSector:          c.LastSector,
		}
		cellPosition[i] = CellPosition{VOBID: 1, CellID: uint8(i + 1)}
	}

	return &ProgramChain{
		NrOfPrograms: uint8(n),
		NrOfCells:    uint8(n),
		PlaybackTime: SecondsToPlaybackTime(totalDuration, isNTSC),
		Programs:     programs,
		CellPlayback: cellPlayback,
		CellPosition: cellPosition,
	}
}

// SecondsToPlaybackTime converts a float64 duration in seconds to the DVD BCD
// PlaybackTime format.
func SecondsToPlaybackTime(secs float64, isNTSC bool) PlaybackTime {
	total := int(math.Round(secs))
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60

	frameRate := 25.0
	if isNTSC {
		frameRate = 29.97
	}
	frames := int((secs - float64(total)) * frameRate)
	if frames < 0 {
		frames = 0
	}

	var frByte uint8
	if isNTSC {
		frByte = 0xC0 | toBCD(frames)
	} else {
		frByte = 0x40 | toBCD(frames)
	}

	return PlaybackTime{
		Hour:      toBCD(h % 100),
		Minute:    toBCD(m),
		Second:    toBCD(s),
		FrameRate: frByte,
	}
}

func toBCD(n int) uint8 {
	if n < 0 {
		n = 0
	}
	return uint8((n/10)<<4 | (n % 10))
}

// WriteVMGM_PGCI_UT serializes menu PGCs into the VMGM_PGCI_UT (Language Unit)
// format required by DVD players for menu navigation.
//
// Unlike VTS's flat PGCIT, the VMGM domain uses a PGCI_UT that wraps PGCs in a
// Language Unit (LU) with an 8-byte LU record header. Without this wrapper,
// libdvdnav reads NrOf_PGCI_SRP as NrOf_LUs and then interprets our PGC search
// records as language unit entries, resulting in 0 PGCs and the menu being skipped.
//
// On-disc layout:
//
//	PGCI_UT header (8 bytes):
//	  [0-1]  NrOf_LUs = 1
//	  [2-3]  Reserved
//	  [4-7]  End_Byte
//	LU record (8 bytes):
//	  [8-9]  Language code ("en" = 0x656E)
//	  [10]   Country code modifier = 0
//	  [11]   LU attributes = 0x83 (root menu entry)
//	  [12-15] LU_Offset = 16 (from PGCI_UT start to LU data)
//	LU data (at byte 16):
//	  [0-1]  NrOf_PGCI_SRP = N
//	  [2-3]  Reserved
//	  [4-7]  LU End_Byte
//	  [8..8+8*N] PGC SRPs: Category(2) + Reserved(2) + PGC_Start_Byte(4)
//	             SRP[0]: category = 0x8300 (entry PGC, root menu)
//	             SRP[1+]: category = 0x0000 (non-entry PGCs)
//	             PGC_Start_Byte is relative to LU data start
//	  [8+8*N...] PGC data
//
// Returns the serialized bytes (sector-padded) and the byte offset of the first
// PGC within the returned slice (used to compute VMG_FirstPlayPGC).
func WriteVMGM_PGCI_UT(pgcs []*ProgramChain) ([]byte, int, error) {
	if len(pgcs) == 0 {
		return nil, 0, nil
	}

	var pgcDataList [][]byte
	for _, pgc := range pgcs {
		data, err := serializePGC(pgc)
		if err != nil {
			return nil, 0, err
		}
		pgcDataList = append(pgcDataList, data)
	}

	n := len(pgcs)
	const pgciUtHeaderSize = 8                           // NrOf_LUs + Reserved + End_Byte
	const luRecordSize = 8                               // lang_code(2) + country(1) + attrs(1) + offset(4)
	const luDataOffset = pgciUtHeaderSize + luRecordSize // = 16: LU data starts here

	const luHeaderSize = 8 // NrOf_SRP + Reserved + End_Byte (within LU data)
	const srpSize = 8      // Category + Reserved + PGC_Start_Byte

	firstPGCInLU := luHeaderSize + srpSize*n // offset within LU data to first PGC

	var pgcTotalSize int
	for _, d := range pgcDataList {
		pgcTotalSize += len(d)
	}

	luDataSize := luHeaderSize + srpSize*n + pgcTotalSize
	luEndByte := uint32(luDataSize - 1)
	totalSize := luDataOffset + luDataSize
	pgciUtEndByte := uint32(totalSize - 1)

	var buf bytes.Buffer

	// PGCI_UT header
	binary.Write(&buf, binary.BigEndian, uint16(1))     // NrOf_LUs
	binary.Write(&buf, binary.BigEndian, uint16(0))     // Reserved
	binary.Write(&buf, binary.BigEndian, pgciUtEndByte) // End_Byte

	// LU record
	buf.WriteByte(0x65)                                        // 'e'
	buf.WriteByte(0x6E)                                        // 'n' (language "en")
	buf.WriteByte(0x00)                                        // country code modifier
	// LU attributes: entry PGC (bit7) + menu type. In the VMGM domain the
	// entry menu is the Title menu (type 2 → 0x82); Root (type 3) belongs to
	// the VTSM domain (audit finding A11).
	buf.WriteByte(0x82)
	binary.Write(&buf, binary.BigEndian, uint32(luDataOffset)) // LU_Offset

	// LU data: PGCIT header
	binary.Write(&buf, binary.BigEndian, uint16(n)) // NrOf_PGCI_SRP
	binary.Write(&buf, binary.BigEndian, uint16(0)) // Reserved
	binary.Write(&buf, binary.BigEndian, luEndByte) // LU End_Byte

	// PGC SRPs
	currentPGCOffset := uint32(firstPGCInLU)
	for i, d := range pgcDataList {
		var cat uint16
		if i == 0 {
			cat = 0x8200 // entry PGC (bit15) + Title menu type (bits 3-0 = 2)
		}
		binary.Write(&buf, binary.BigEndian, cat)
		binary.Write(&buf, binary.BigEndian, uint16(0))
		binary.Write(&buf, binary.BigEndian, currentPGCOffset)
		currentPGCOffset += uint32(len(d))
	}

	// PGC data
	for _, d := range pgcDataList {
		buf.Write(d)
	}

	// Pad to sector boundary
	if rem := buf.Len() % 2048; rem != 0 {
		buf.Write(make([]byte, 2048-rem))
	}

	// firstPGCOffset is the byte offset of the first PGC within this slice.
	// The caller adds (sector * 2048) to get the IFO-absolute offset for first_play_pgc.
	firstPGCOffset := luDataOffset + firstPGCInLU
	return buf.Bytes(), firstPGCOffset, nil
}

// WritePGCITI serializes a single PGC into a VTS_PGCITI block and returns
// the bytes (padded to a full 2048-byte sector).
func WritePGCITI(pgc *ProgramChain) ([]byte, error) {
	logging.Debug(logging.CatDVD, "Building PGCITI for %d program(s), %d cell(s)",
		pgc.NrOfPrograms, pgc.NrOfCells)

	pgcData, err := serializePGC(pgc)
	if err != nil {
		return nil, err
	}

	// PGCITI layout:
	//   Bytes 0-1:  NrOf_VTS_PGCI (uint16)
	//   Bytes 2-3:  Reserved (uint16)
	//   Bytes 4-7:  EndByte (uint32) — last byte of table, 0-relative
	//   Bytes 8-15: One PGC search pointer (8 bytes)
	//     Bytes 8-9:   Category (uint16) — 0x8000 = entry PGC
	//     Bytes 10-11: Reserved (uint16)
	//     Bytes 12-15: PGC_Start_Byte (uint32) — offset of PGC within PGCITI
	// Total header + 1 pointer = 16 bytes; PGC follows immediately.

	const tableHeaderSize = 8 // NrOf + Reserved + EndByte
	const ptrSize = 8         // Category + Reserved + PGC_Start_Byte
	pgcStartByte := uint32(tableHeaderSize + ptrSize)
	endByte := pgcStartByte + uint32(len(pgcData)) - 1

	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint16(1))      // NrOf_VTS_PGCI
	binary.Write(&buf, binary.BigEndian, uint16(0))      // Reserved
	binary.Write(&buf, binary.BigEndian, endByte)        // EndByte
	binary.Write(&buf, binary.BigEndian, uint16(0x8000)) // Category: entry PGC
	binary.Write(&buf, binary.BigEndian, uint16(0))      // Reserved
	binary.Write(&buf, binary.BigEndian, pgcStartByte)   // PGC_Start_Byte
	buf.Write(pgcData)

	// Pad to sector boundary
	if rem := buf.Len() % 2048; rem != 0 {
		buf.Write(make([]byte, 2048-rem))
	}

	return buf.Bytes(), nil
}

// WritePGCITIs serializes multiple PGCs into a VTS_PGCITI block.
// Each PGC gets its own search pointer with correct offset.
func WritePGCITIs(pgcs []*ProgramChain) ([]byte, error) {
	if len(pgcs) == 0 {
		return nil, nil
	}
	if len(pgcs) == 1 {
		return WritePGCITI(pgcs[0])
	}

	const tableHeaderSize = 8
	const ptrSize = 8

	var pgcDataList [][]byte
	var pgcData bytes.Buffer
	for _, pgc := range pgcs {
		data, err := serializePGC(pgc)
		if err != nil {
			return nil, err
		}
		pgcDataList = append(pgcDataList, data)
		pgcData.Write(data)
	}

	pgcStartByte := uint32(tableHeaderSize + ptrSize*len(pgcs))
	endByte := pgcStartByte + uint32(pgcData.Len()) - 1

	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint16(len(pgcs)))
	binary.Write(&buf, binary.BigEndian, uint16(0))
	binary.Write(&buf, binary.BigEndian, endByte)

	currentOffset := pgcStartByte
	for i := range pgcs {
		var category uint16 = 0x8000
		if i == 0 {
			category = 0x8000
		}
		binary.Write(&buf, binary.BigEndian, category)
		binary.Write(&buf, binary.BigEndian, uint16(0))
		binary.Write(&buf, binary.BigEndian, currentOffset)
		currentOffset += uint32(len(pgcDataList[i]))
	}

	buf.Write(pgcData.Bytes())

	if rem := buf.Len() % 2048; rem != 0 {
		buf.Write(make([]byte, 2048-rem))
	}

	return buf.Bytes(), nil
}

// BuildMenuPGC creates a PGC for a DVD menu with the given button command table.
// duration is the menu loop duration in seconds (used for playback time fields;
// the cell still-time is set to 0xFF = infinite so the player waits for input).
func BuildMenuPGC(cmdTable *DVDCommandTable, duration float64, isNTSC bool) *ProgramChain {
	pgc := &ProgramChain{
		NrOfPrograms: 1,
		NrOfCells:    1,
		PlaybackTime: SecondsToPlaybackTime(duration, isNTSC),

		// Restrict user operations that don't apply to menus (fast-forward, angle,
		// audio/subpicture stream change, etc.). Value matches dvdauthor output for
		// standard menus: allow button navigation and title jump only.
		ProhibitedOps: 0x024C08C4,

		// Enable subpicture stream 0 for the menu overlay (button highlights).
		// Each SubpictureCtl entry is a uint32 stored big-endian on disc:
		//   byte 0 (bits 31-24): 4:3 mode      — bit 7 = active, bits 6-0 = stream number
		//   byte 1 (bits 23-16): widescreen     — bit 7 = active, bits 6-0 = stream number
		//   byte 2 (bits 15-8):  letterbox      — bit 7 = active, bits 6-0 = stream number
		//   byte 3 (bits 7-0):   pan & scan     — bit 7 = active, bits 6-0 = stream number
		// libdvdread zero-check: (value & 0x1f1f1f1f) must equal 0 (reserved bits).
		// 0x80000000 = 4:3 mode active with stream 0; other modes inactive/unused.
		SubpictureCtl: func() [32]uint32 {
			var s [32]uint32
			s[0] = 0x80000000 // 4:3 mode active, stream 0
			return s
		}(),

		// YCbCr palette entries for SPU button highlights (indices 0-3).
		// These match the spu.DefaultPalette() RGB values converted to studio-swing YCbCr.
		// Entry format: [0x00, Y, Cb, Cr]
		//   Index 0: transparent (black, alpha=0) — Y=16, Cb=128, Cr=128
		//   Index 1: white (text/button highlight) — Y=235, Cb=128, Cr=128
		//   Index 2: black (outline)               — Y=16,  Cb=128, Cr=128
		//   Index 3: gray (shadow)                 — Y=128, Cb=128, Cr=128
		Palette: [16][4]byte{
			{0x00, 0x10, 0x80, 0x80}, // 0: transparent/black
			{0x00, 0xEB, 0x80, 0x80}, // 1: white
			{0x00, 0x10, 0x80, 0x80}, // 2: black outline
			{0x00, 0x80, 0x80, 0x80}, // 3: gray shadow
		},

		// StillTime: 0xFF = infinite still — hold the menu indefinitely until
		// the user presses a button. Without this, the player advances after
		// the playback duration expires.
		StillTime: 0xFF,

		Programs: []ProgramInfo{{EntryCell: 1}},
		CellPlayback: []CellPlayback{{
			PlaybackTime: SecondsToPlaybackTime(duration, isNTSC),
			CommandNr:    0,    // buttons trigger cell command table; no auto-command
			StillTime:    0xFF, // hold cell indefinitely
		}},
		CellPosition: []CellPosition{{VOBID: 1, CellID: 1}},
		CommandTable: cmdTable,
	}
	return pgc
}

// BuildFirstPlayPGC returns a command-only First-Play PGC: no programs or
// cells, a single pre-command executed when the disc is inserted (typically
// JumpTT 1 for a menu-less disc, or a JumpSS to the VMGM for a disc with
// menus). The DVD player runs the pre-command and exits (audit finding A12).
func BuildFirstPlayPGC(cmd DVDCommand) *ProgramChain {
	return &ProgramChain{
		CommandTable: &DVDCommandTable{Pre: []DVDCommand{cmd}},
	}
}

// serializePGC writes a PGC into its on-disc binary representation.
func serializePGC(pgc *ProgramChain) ([]byte, error) {
	nProg := int(pgc.NrOfPrograms)
	nCell := int(pgc.NrOfCells)

	// Offsets within PGC (from start of PGC data):
	progMapOffset := uint16(pgcHeaderSize) // right after the 236-byte header
	cellPlayOffset := progMapOffset + uint16(nProg)
	cellPosOffset := cellPlayOffset + uint16(nCell)*24

	// Command table follows cell position table (0 = no commands)
	var cmdTblOffset uint16
	var cmdTblData []byte
	if pgc.CommandTable != nil && !pgc.CommandTable.Empty() {
		cmdTblOffset = cellPosOffset + uint16(nCell)*4
		cmdTblData = SerializeCommandTable(pgc.CommandTable)
	}

	var buf bytes.Buffer

	// ── 236-byte PGC header ──────────────────────────────────────────────────
	binary.Write(&buf, binary.BigEndian, uint16(0)) // Reserved
	buf.WriteByte(pgc.NrOfPrograms)
	buf.WriteByte(pgc.NrOfCells)

	// Playback time (4 bytes BCD)
	buf.WriteByte(pgc.PlaybackTime.Hour)
	buf.WriteByte(pgc.PlaybackTime.Minute)
	buf.WriteByte(pgc.PlaybackTime.Second)
	buf.WriteByte(pgc.PlaybackTime.FrameRate)

	binary.Write(&buf, binary.BigEndian, pgc.ProhibitedOps) // 4 bytes

	for _, ac := range pgc.AudioControl {
		binary.Write(&buf, binary.BigEndian, ac)
	}
	for _, sp := range pgc.SubpictureCtl {
		binary.Write(&buf, binary.BigEndian, sp)
	}

	binary.Write(&buf, binary.BigEndian, pgc.NextPGCN)
	binary.Write(&buf, binary.BigEndian, pgc.PrevPGCN)
	binary.Write(&buf, binary.BigEndian, pgc.GoUpPGCN)
	buf.WriteByte(pgc.StillTime)
	buf.WriteByte(pgc.PGPlaybackMode)

	for _, pe := range pgc.Palette {
		buf.Write(pe[:])
	}

	binary.Write(&buf, binary.BigEndian, cmdTblOffset) // Command_TBL_Offset (0 = no cmds)
	binary.Write(&buf, binary.BigEndian, progMapOffset)
	binary.Write(&buf, binary.BigEndian, cellPlayOffset)
	binary.Write(&buf, binary.BigEndian, cellPosOffset)

	// Verify we're at byte 236
	if buf.Len() != pgcHeaderSize {
		logging.Error(logging.CatDVD, "PGC header size mismatch: got %d, want %d", buf.Len(), pgcHeaderSize)
	}

	// ── Program map ──────────────────────────────────────────────────────────
	for _, p := range pgc.Programs {
		buf.WriteByte(p.EntryCell)
	}

	// ── Cell playback table (24 bytes per cell) ───────────────────────────────
	for _, c := range pgc.CellPlayback {
		// Byte 0: BlockMode(7-6) | BlockType(5-4) | SeamlessPlay(3) | Interleaved(2) | STCDisc(1) | 0
		buf.WriteByte((c.BlockMode << 6) | (c.BlockType << 4))
		// Byte 1: PlaybackMode(7) | RestrictedMode(6) | 0 | 0 | 0 | 0 | 0 | 0
		buf.WriteByte(0x00)
		buf.WriteByte(c.StillTime)
		buf.WriteByte(c.CommandNr)
		buf.WriteByte(c.PlaybackTime.Hour)
		buf.WriteByte(c.PlaybackTime.Minute)
		buf.WriteByte(c.PlaybackTime.Second)
		buf.WriteByte(c.PlaybackTime.FrameRate)
		binary.Write(&buf, binary.BigEndian, c.FirstSector)
		binary.Write(&buf, binary.BigEndian, c.FirstILVUEndSector)
		binary.Write(&buf, binary.BigEndian, c.LastVOBUStartSector)
		binary.Write(&buf, binary.BigEndian, c.LastSector)
	}

	// ── Cell position table (4 bytes per cell) ────────────────────────────────
	for _, cp := range pgc.CellPosition {
		binary.Write(&buf, binary.BigEndian, cp.VOBID)
		buf.WriteByte(0x00) // Reserved
		buf.WriteByte(cp.CellID)
	}

	// ── Command table (if present) ────────────────────────────────────────────
	if len(cmdTblData) > 0 {
		buf.Write(cmdTblData)
	}

	return buf.Bytes(), nil
}
