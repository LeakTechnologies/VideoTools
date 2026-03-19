package ifo

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

// VTS_MAT represents the Video Title Set Information Management Table.
type VTS_MAT struct {
	VTS_Identifier          [12]byte // "DVDVIDEO-VTS"
	VTS_Last_Sector         uint32
	VTS_BUP_Last_Sector     uint32
	VTS_MAT_Last_Sector     uint32
	VTS_Category            uint32
	VTS_Attributes          VideoAttributes
	VTS_Audio_Streams_Count uint16
	VTS_Audio_Attributes    [8]AudioAttributes
	VTS_Subpicture_Count    uint16
	VTS_Subpicture_Attrs    [32]SubpictureAttributes
	
	// Table Offsets (relative to sector 0)
	VTS_PTT_SRPT_Offset     uint32 // Part of Title Search Pointer Table
	VTS_PGCITI_Offset       uint32 // PGC Information Table
	VTS_M_PGCI_UT_Offset    uint32 // Menu PGC Unit Table
	VTS_TMAPTI_Offset       uint32 // Time Map Table
	VTS_M_C_ADT_Offset      uint32 // Menu Cell Address Table
	VTS_M_VOBU_ADMAP_Offset uint32 // Menu VOBU Address Map
	VTS_C_ADT_Offset        uint32 // Title Cell Address Table
	VTS_VOBU_ADMAP_Offset   uint32 // Title VOBU Address Map
}

// VTS_TMAPT is the VTS Time Map Table — maps time offsets to sector addresses,
// enabling fast-forward/rewind seek on hardware players.
//
// One TMAP entry per TimeUnit seconds; Entry[i] gives the sector address at
// time i*TimeUnit. Bit 31 of each entry is the Entry Cell Change (ECCE) flag.
type VTS_TMAPT struct {
	TimeUnit uint8    // seconds per entry (1 = 1 entry/second)
	Sectors  []uint32 // sector addresses (bit 31 = ECCE; cleared to 0 here)
}

// BuildLinearTMAPT creates a TMAPT by linearly interpolating sector addresses
// from the VOB file size and duration. This is a constant-bitrate approximation
// suitable for hardware seek bar display.
//
// totalSectors is the size of the VOB in 2048-byte sectors.
// durationSeconds is the title duration.
// timeUnit is the seconds between entries (use 1 for maximum resolution).
func BuildLinearTMAPT(totalSectors uint32, durationSeconds float64, timeUnit int) *VTS_TMAPT {
	if durationSeconds <= 0 || totalSectors == 0 || timeUnit <= 0 {
		return nil
	}
	nEntries := int(durationSeconds/float64(timeUnit)) + 1
	sectors := make([]uint32, nEntries)
	for i := 0; i < nEntries; i++ {
		frac := float64(i) * float64(timeUnit) / durationSeconds
		if frac > 1.0 {
			frac = 1.0
		}
		sectors[i] = uint32(float64(totalSectors-1) * frac)
	}
	return &VTS_TMAPT{TimeUnit: uint8(timeUnit), Sectors: sectors}
}

// WriteTMAPT serializes a VTS_TMAPT (for a single title) and returns the
// sector-padded bytes ready to embed in a VTS IFO.
//
// On-disc layout (one title):
//
//	[0-1]  NrOf_VTS_TMAPTs = 1 (uint16)
//	[2-3]  Reserved (uint16)
//	[4-7]  EndByte (uint32)
//	[8-11] TMAP[0] start byte offset = 12 (uint32)
//	[12-13] NrOf_Entries (uint16)
//	[14]   Time_Unit (uint8)
//	[15]   Reserved (uint8)
//	[16+]  Entries (uint32 each)
func WriteTMAPT(t *VTS_TMAPT) ([]byte, error) {
	if t == nil || len(t.Sectors) == 0 {
		return nil, fmt.Errorf("WriteTMAPT: nil or empty TMAPT")
	}
	nEntries := len(t.Sectors)
	// Header: 8 + 4 (1 offset pointer) = 12 bytes before the TMAP data
	tmapOffset := uint32(12)
	tmapSize := 4 + nEntries*4           // NrOf_Entries(2) + TimeUnit(1) + Rsv(1) + entries
	endByte := tmapOffset + uint32(tmapSize) - 1

	logging.Debug(logging.CatDVD, "Building TMAPT: %d entries @ %ds intervals", nEntries, t.TimeUnit)

	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint16(1))        // NrOf_VTS_TMAPTs
	binary.Write(&buf, binary.BigEndian, uint16(0))        // Reserved
	binary.Write(&buf, binary.BigEndian, endByte)          // EndByte
	binary.Write(&buf, binary.BigEndian, tmapOffset)       // TMAP[0] start byte

	// TMAP header
	binary.Write(&buf, binary.BigEndian, uint16(nEntries)) // NrOf_Entries
	buf.WriteByte(t.TimeUnit)                              // Time_Unit
	buf.WriteByte(0x00)                                    // Reserved

	// Entries: bit 31 = ECCE (0 for single cell); bits 0-30 = sector address
	for _, s := range t.Sectors {
		binary.Write(&buf, binary.BigEndian, s&0x7FFFFFFF)
	}

	// Pad to sector boundary
	if rem := buf.Len() % 2048; rem != 0 {
		buf.Write(make([]byte, 2048-rem))
	}
	return buf.Bytes(), nil
}

// VTS_PTT_SRPT is the Part-of-Title Search Pointer Table.
// It maps chapter numbers (PTT = Part of Title) to PGC/program pairs,
// enabling hardware players to jump directly to any chapter.
//
// One VTS_PTT_SRPT per VTS; for our single-title-per-VTS layout:
//   NrOf_Srpts = 1 (one title in this VTS)
//   PTT[i]     = chapter i+1 → PGC 1, Program i+1
type VTS_PTT_SRPT struct {
	NrOfChapters uint16 // number of chapters (PTT entries)
}

// WriteVTS_PTT_SRPT serializes a VTS_PTT_SRPT for a single title with the
// given number of chapters and returns the sector-padded bytes.
//
// On-disc layout (one title, N chapters):
//
//	[0-1]  NrOf_Srpts = 1 (uint16)
//	[2-3]  Reserved (uint16)
//	[4-7]  EndByte (uint32) — last byte of table, 0-relative
//	[8-11] Offset[0] = 12 (uint32) — byte offset within table to title 0's PTT list
//	[12+]  N PTT entries × 4 bytes: PGCN(2) + PGN(1) + Reserved(1)
func WriteVTS_PTT_SRPT(srpt *VTS_PTT_SRPT) ([]byte, error) {
	n := int(srpt.NrOfChapters)
	if n == 0 {
		return nil, fmt.Errorf("WriteVTS_PTT_SRPT: zero chapters")
	}
	// Layout: 8 header + 4 offset + n*4 entries
	pttListOffset := uint32(12) // after 8-byte header + 4-byte offset for title 0
	endByte := pttListOffset + uint32(n*4) - 1

	logging.Debug(logging.CatDVD, "Building VTS_PTT_SRPT: %d chapter(s)", n)

	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint16(1))       // NrOf_Srpts
	binary.Write(&buf, binary.BigEndian, uint16(0))       // Reserved
	binary.Write(&buf, binary.BigEndian, endByte)         // EndByte
	binary.Write(&buf, binary.BigEndian, pttListOffset)   // Offset[0]

	for i := 0; i < n; i++ {
		binary.Write(&buf, binary.BigEndian, uint16(1))   // PGCN = 1 (always PGC 1)
		buf.WriteByte(uint8(i + 1))                       // PGN  = chapter number (1-indexed)
		buf.WriteByte(0x00)                               // Reserved
	}

	// Pad to sector boundary
	if rem := buf.Len() % 2048; rem != 0 {
		buf.Write(make([]byte, 2048-rem))
	}
	return buf.Bytes(), nil
}

// VTS_C_ADT is the Cell Address Table for a VTS title set.
// It maps each cell in the VTS to its VOB ID, cell ID, and disc sector range.
// Hardware players validate PGC cell references against this table and use it
// for physical seek operations.
//
// On-disc layout:
//
//	[0-1]  Nr_of_Cells (uint16)
//	[2-3]  Reserved (uint16)
//	[4-7]  EndByte (uint32) — last byte of table, 0-relative
//	[8+]   Nr_of_Cells × 12-byte Cell_ADT_Entry:
//	         [0-1]  VOB_ID (uint16, 1-indexed)
//	         [2]    Cell_ID (uint8, 1-indexed)
//	         [3]    Reserved (uint8)
//	         [4-7]  StartSector (uint32)
//	         [8-11] EndSector (uint32)
type VTS_C_ADT struct {
	Cells []CellADTEntry
}

// CellADTEntry is one record in the VTS_C_ADT table.
type CellADTEntry struct {
	VOBID       uint16
	CellID      uint8
	StartSector uint32
	EndSector   uint32
}

// BuildVTS_C_ADT constructs a VTS_C_ADT from a ProgramChain's cell playback
// table. Each cell in the PGC becomes one entry with VOBID=1 and CellID=i+1.
// Returns nil if the PGC has no cells or all cells have zero sector ranges.
func BuildVTS_C_ADT(pgc *ProgramChain) *VTS_C_ADT {
	if pgc == nil || len(pgc.CellPlayback) == 0 {
		return nil
	}
	entries := make([]CellADTEntry, len(pgc.CellPlayback))
	for i, c := range pgc.CellPlayback {
		entries[i] = CellADTEntry{
			VOBID:       1,
			CellID:      uint8(i + 1),
			StartSector: c.FirstSector,
			EndSector:   c.LastSector,
		}
	}
	return &VTS_C_ADT{Cells: entries}
}

// WriteVTS_C_ADT serializes the VTS_C_ADT and returns the sector-padded bytes.
func WriteVTS_C_ADT(cadt *VTS_C_ADT) ([]byte, error) {
	n := len(cadt.Cells)
	if n == 0 {
		return nil, fmt.Errorf("WriteVTS_C_ADT: empty cell table")
	}
	// 8-byte header + 12 bytes per cell
	endByte := uint32(8+n*12) - 1

	logging.Debug(logging.CatDVD, "Building VTS_C_ADT: %d cell(s)", n)

	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint16(n)) // Nr_of_Cells
	binary.Write(&buf, binary.BigEndian, uint16(0)) // Reserved
	binary.Write(&buf, binary.BigEndian, endByte)   // EndByte

	for _, e := range cadt.Cells {
		binary.Write(&buf, binary.BigEndian, e.VOBID)
		buf.WriteByte(e.CellID)
		buf.WriteByte(0x00) // Reserved
		binary.Write(&buf, binary.BigEndian, e.StartSector)
		binary.Write(&buf, binary.BigEndian, e.EndSector)
	}

	if rem := buf.Len() % 2048; rem != 0 {
		buf.Write(make([]byte, 2048-rem))
	}
	return buf.Bytes(), nil
}

// VOBU_ADMAP represents the VOBU Address Map table.
type VOBU_ADMAP struct {
	EndByte uint32
	Sectors []uint32
}

// BuildVOBU_ADMAP constructs a VOBU_ADMAP from a slice of NAV_PCK sector
// offsets (0-indexed, relative to the start of the VOB file / VTS VOB set).
// Returns nil when sectors is empty.
func BuildVOBU_ADMAP(sectors []uint32) *VOBU_ADMAP {
	if len(sectors) == 0 {
		return nil
	}
	// EndByte field (4 bytes) + one uint32 per entry; EndByte is 0-relative
	// within the ADMAP structure (not counting the EndByte field itself per
	// the DVD spec: EndByte = total_size_of_table - 1).
	endByte := uint32(4 + len(sectors)*4 - 1)
	return &VOBU_ADMAP{EndByte: endByte, Sectors: sectors}
}

// WriteVOBU_ADMAP serializes the VOBU_ADMAP to an IFO file.
func WriteVOBU_ADMAP(w io.Writer, admap *VOBU_ADMAP) error {
	logging.Debug(logging.CatDVD, "Writing VOBU_ADMAP with %d entries", len(admap.Sectors))
	
	if err := binary.Write(w, binary.BigEndian, admap.EndByte); err != nil {
		return err
	}
	
	for _, sector := range admap.Sectors {
		if err := binary.Write(w, binary.BigEndian, sector); err != nil {
			return err
		}
	}
	return nil
}

// WriteVTSI serializes the VTS_MAT to an IFO file using the spec-correct
// offset-based layout from SerializeVTSMAT.
func WriteVTSI(w io.Writer, mat *VTS_MAT) error {
	logging.Info(logging.CatDVD, "Serializing VTSI Management Table (VTS_MAT)")
	if _, err := w.Write(SerializeVTSMAT(mat)); err != nil {
		logging.Error(logging.CatDVD, "Failed to write VTS_MAT: %v", err)
		return fmt.Errorf("write vts_mat: %w", err)
	}
	logging.Debug(logging.CatDVD, "VTS_MAT written. Last sector: %d", mat.VTS_Last_Sector)
	return nil
}

// NewVTSMAT creates a VTS_MAT with default "DVDVIDEO-VTS" identifier.
func NewVTSMAT() *VTS_MAT {
	mat := &VTS_MAT{}
	copy(mat.VTS_Identifier[:], "DVDVIDEO-VTS")
	return mat
}

// ReadVTSI parses the VTS_MAT from the first sector of a VTS_xx_0.IFO file.
// Fields are read from their spec-correct byte offsets (matching SerializeVTSMAT).
func ReadVTSI(r io.Reader) (*VTS_MAT, error) {
	buf := make([]byte, 2048)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, fmt.Errorf("read vts_mat sector: %w", err)
	}
	id := string(buf[0:12])
	if !strings.HasPrefix(id, "DVDVIDEO-VTS") {
		return nil, fmt.Errorf("invalid VTS identifier: %q", id)
	}
	mat := &VTS_MAT{}
	copy(mat.VTS_Identifier[:], buf[0:12])
	mat.VTS_Last_Sector         = binary.BigEndian.Uint32(buf[12:16])
	mat.VTS_BUP_Last_Sector     = binary.BigEndian.Uint32(buf[28:32])
	mat.VTS_MAT_Last_Sector     = binary.BigEndian.Uint32(buf[40:44])
	mat.VTS_Category            = binary.BigEndian.Uint32(buf[45:49])
	mat.VTS_Audio_Streams_Count = binary.BigEndian.Uint16(buf[139:141])
	for i := 0; i < 8; i++ {
		off := 141 + i*8
		aa := &mat.VTS_Audio_Attributes[i]
		aa.AudioCodingMode = (buf[off+0] >> 5) & 0x07
		aa.Multichannel    = (buf[off+0] >> 4) & 0x01
		aa.SampleRate      = (buf[off+1] >> 4) & 0x03
		aa.NumChannels     = buf[off+1] & 0x07
		copy(aa.LanguageCode[:], buf[off+2:off+4])
		aa.SpecificCode    = buf[off+4]
	}
	mat.VTS_Subpicture_Count = binary.BigEndian.Uint16(buf[222:224])
	for i := 0; i < 32; i++ {
		off := 224 + i*6
		sp := &mat.VTS_Subpicture_Attrs[i]
		sp.CodingMode = buf[off+0]
		copy(sp.LanguageCode[:], buf[off+2:off+4])
		sp.SpecificCode = buf[off+4]
	}
	// 0x200 (512): VTS title video attributes
	mat.VTS_Attributes.CompressionMode  = (buf[512] >> 6) & 0x03
	mat.VTS_Attributes.TVSystem         = (buf[512] >> 4) & 0x03
	mat.VTS_Attributes.AspectRatio      = (buf[512] >> 2) & 0x03
	mat.VTS_Attributes.PermittedDisplay = buf[512] & 0x03
	mat.VTS_Attributes.Line21_1         = (buf[513] >> 7) & 0x01
	mat.VTS_Attributes.Line21_2         = (buf[513] >> 6) & 0x01
	mat.VTS_Attributes.Resolution       = (buf[513] >> 2) & 0x03
	mat.VTS_Attributes.Letterboxed      = (buf[513] >> 1) & 0x01
	mat.VTS_Attributes.FilmMode         = buf[513] & 0x01

	mat.VTS_PTT_SRPT_Offset     = binary.BigEndian.Uint32(buf[418:422])
	mat.VTS_PGCITI_Offset       = binary.BigEndian.Uint32(buf[422:426])
	mat.VTS_M_PGCI_UT_Offset    = binary.BigEndian.Uint32(buf[426:430])
	mat.VTS_TMAPTI_Offset       = binary.BigEndian.Uint32(buf[430:434])
	mat.VTS_M_C_ADT_Offset      = binary.BigEndian.Uint32(buf[434:438])
	mat.VTS_M_VOBU_ADMAP_Offset = binary.BigEndian.Uint32(buf[438:442])
	mat.VTS_C_ADT_Offset        = binary.BigEndian.Uint32(buf[442:446])
	mat.VTS_VOBU_ADMAP_Offset   = binary.BigEndian.Uint32(buf[446:450])
	return mat, nil
}
