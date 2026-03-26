package ifo

import (
	"bytes"
	"encoding/binary"
	"math"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
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
// numButtons is used for documentation only; the actual commands come from cmdTable.
// duration is the menu loop duration in seconds.
func BuildMenuPGC(cmdTable *DVDCommandTable, duration float64, isNTSC bool) *ProgramChain {
	return &ProgramChain{
		NrOfPrograms: 1,
		NrOfCells:    1,
		PlaybackTime: SecondsToPlaybackTime(duration, isNTSC),
		Programs:     []ProgramInfo{{EntryCell: 1}},
		CellPlayback: []CellPlayback{{
			PlaybackTime: SecondsToPlaybackTime(duration, isNTSC),
			CommandNr:    0, // no auto cell command; buttons trigger cell command table
		}},
		CellPosition: []CellPosition{{VOBID: 1, CellID: 1}},
		CommandTable: cmdTable,
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
