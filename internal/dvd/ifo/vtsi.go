package ifo

import (
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

// VOBU_ADMAP represents the VOBU Address Map table.
type VOBU_ADMAP struct {
	EndByte uint32
	Sectors []uint32
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
