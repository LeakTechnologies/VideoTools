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

// WriteVTSI serializes the VTS_MAT to an IFO file.
func WriteVTSI(w io.Writer, mat *VTS_MAT) error {
	logging.Info(logging.CatDVD, "Serializing VTSI Management Table (VTS_MAT)")
	
	// DVD-Video headers are Big Endian
	if err := binary.Write(w, binary.BigEndian, mat); err != nil {
		logging.Error(logging.CatDVD, "Failed to write VTS_MAT: %v", err)
		return fmt.Errorf("write vts_mat: %w", err)
	}
	
	logging.Debug(logging.CatDVD, "VTS_MAT successfully written. Last sector: %d", mat.VTS_Last_Sector)
	return nil
}

// NewVTSMAT creates a VTS_MAT with default "DVDVIDEO-VTS" identifier.
func NewVTSMAT() *VTS_MAT {
	mat := &VTS_MAT{}
	copy(mat.VTS_Identifier[:], "DVDVIDEO-VTS")
	return mat
}

// ReadVTSI parses a VTS IFO file from a reader.
func ReadVTSI(r io.Reader) (*VTS_MAT, error) {
	mat := &VTS_MAT{}
	if err := binary.Read(r, binary.BigEndian, mat); err != nil {
		return nil, fmt.Errorf("read vts_mat: %w", err)
	}
	
	// Basic validation
	id := string(mat.VTS_Identifier[:])
	if !strings.HasPrefix(id, "DVDVIDEO-VTS") {
		return nil, fmt.Errorf("invalid VTS identifier: %s", id)
	}
	
	return mat, nil
}
