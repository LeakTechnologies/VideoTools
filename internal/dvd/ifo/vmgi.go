package ifo

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

// VMG_MAT represents the Video Manager Information Management Table.
// Fields map to DVD-Video spec §3.2 byte offsets; see SerializeVMGMAT for layout.
type VMG_MAT struct {
	VMG_Identifier          [12]byte // "DVDVIDEO-VMG"
	VMG_Last_Sector         uint32
	VMG_BUP_Last_Sector     uint32   // VMGM_VOBS_Last_Sector (menu VOB; 0 if none)
	VMG_MAT_Last_Sector     uint32   // VMGI_MAT_Last_Sector
	VMG_Category            uint32

	// Disc metadata (written by SerializeVMGMAT at correct byte offsets)
	NrOfTitleSets uint16 // total number of VTS (Video Title Sets) on this disc

	// Table Offsets (relative to sector 0 of VIDEO_TS.IFO)
	TT_SRPT_Offset          uint32 // Title Search Pointer Table (0x0C0 = 192)
	VMG_PTT_SRPT_Offset     uint32 // Part of Title Search Pointer Table (0 = absent)
	VMG_PGCITI_Offset       uint32 // VMG Menu PGC Information Table (VMGM_PGCI_UT)
	VMG_PTL_MAIT_Offset     uint32 // Parental Management Information (0 = absent)
	VMG_VTS_ATRT_Offset     uint32 // VTS Attribute Table (0 = absent)
	VMG_TXTDT_MG_Offset     uint32 // Text Data Manager (0 = absent)
	VMG_M_C_ADT_Offset      uint32 // Menu Cell Address Table (0 = absent)
	VMG_M_VOBU_ADMAP_Offset uint32 // Menu VOBU Address Map (0 = absent)
}

// VTS_ATRT represents the VTS Attribute Table.
type VTS_ATRT struct {
	NumVTS   uint16
	Reserved uint16
	EndByte  uint32
	VTS_Offsets []uint32 // Offsets to each VTS_ATRT_Entry relative to start of table
	Entries []VTS_ATRT_Entry
}

type VTS_ATRT_Entry struct {
	VTS_MAT_Last_Sector uint32
	Video_Attrs         VideoAttributes
	NumAudio            uint16
	Audio_Attrs         [8]AudioAttributes
	NumSubpicture       uint16
	Subpicture_Attrs    [32]SubpictureAttributes
}

// WriteVTS_ATRT serializes the VTS_ATRT to an IFO file.
func WriteVTS_ATRT(w io.Writer, table *VTS_ATRT) error {
	logging.Debug(logging.CatDVD, "Writing VTS_ATRT with %d entries", table.NumVTS)
	if err := binary.Write(w, binary.BigEndian, table.NumVTS); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, table.Reserved); err != nil {
		return err
	}
	_ = binary.Write(w, binary.BigEndian, table.EndByte)
	// [Implementation of full VTS_ATRT serialization will follow in builder logic]
	return nil
}

// TT_SRPT represents the Title Search Pointer Table.
type TT_SRPT struct {
	NumTitles uint16
	Reserved  uint16
	EndByte   uint32
	Titles    []TitleSearchPointer
}

type TitleSearchPointer struct {
	TitleType       uint8
	NumAngles       uint8
	NumChapters     uint16
	ParentalID      uint16
	VTSNumber       uint8
	VTS_TitleNumber uint8
	StartSector     uint32
}

// WriteTT_SRPT serializes a TT_SRPT and returns the sector-padded bytes.
func WriteTT_SRPT(srpt *TT_SRPT) ([]byte, error) {
	logging.Debug(logging.CatDVD, "Building TT_SRPT with %d title(s)", srpt.NumTitles)

	// 8-byte header + 12 bytes per title entry
	endByte := uint32(8+int(srpt.NumTitles)*12) - 1

	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, srpt.NumTitles)
	binary.Write(&buf, binary.BigEndian, srpt.Reserved)
	binary.Write(&buf, binary.BigEndian, endByte)

	for _, t := range srpt.Titles {
		buf.WriteByte(t.TitleType)
		buf.WriteByte(t.NumAngles)
		binary.Write(&buf, binary.BigEndian, t.NumChapters)
		binary.Write(&buf, binary.BigEndian, t.ParentalID)
		buf.WriteByte(t.VTSNumber)
		buf.WriteByte(t.VTS_TitleNumber)
		binary.Write(&buf, binary.BigEndian, t.StartSector)
	}

	// Pad to sector boundary
	if rem := buf.Len() % 2048; rem != 0 {
		buf.Write(make([]byte, 2048-rem))
	}
	return buf.Bytes(), nil
}

// WriteVMGI serializes the VMG_MAT to an IFO file.
func WriteVMGI(w io.Writer, mat *VMG_MAT) error {
	logging.Info(logging.CatDVD, "Serializing VMGI Management Table (VMG_MAT)")
	
	if err := binary.Write(w, binary.BigEndian, mat); err != nil {
		logging.Error(logging.CatDVD, "Failed to write VMG_MAT: %v", err)
		return fmt.Errorf("write vmg_mat: %w", err)
	}
	
	logging.Debug(logging.CatDVD, "VMG_MAT successfully written. Titles mapped at offset: %d", mat.TT_SRPT_Offset)
	return nil
}

// NewVMGMAT creates a VMG_MAT with default "DVDVIDEO-VMG" identifier.
func NewVMGMAT() *VMG_MAT {
	mat := &VMG_MAT{}
	copy(mat.VMG_Identifier[:], "DVDVIDEO-VMG")
	return mat
}

// ReadVMGI parses the VMG_MAT from the first sector of a VIDEO_TS.IFO file.
// Fields are read from their spec-correct byte offsets (matching SerializeVMGMAT).
func ReadVMGI(r io.Reader) (*VMG_MAT, error) {
	buf := make([]byte, 2048)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, fmt.Errorf("read vmg_mat sector: %w", err)
	}
	id := string(buf[0:12])
	if !strings.HasPrefix(id, "DVDVIDEO-VMG") {
		return nil, fmt.Errorf("invalid VMG identifier: %q", id)
	}
	mat := &VMG_MAT{}
	copy(mat.VMG_Identifier[:], buf[0:12])
	mat.VMG_Last_Sector         = binary.BigEndian.Uint32(buf[12:16])
	mat.VMG_BUP_Last_Sector     = binary.BigEndian.Uint32(buf[28:32])
	mat.VMG_MAT_Last_Sector     = binary.BigEndian.Uint32(buf[40:44])
	mat.VMG_Category            = binary.BigEndian.Uint32(buf[44:48])
	mat.NrOfTitleSets           = binary.BigEndian.Uint16(buf[72:74])
	mat.TT_SRPT_Offset          = binary.BigEndian.Uint32(buf[192:196])
	mat.VMG_PTT_SRPT_Offset     = binary.BigEndian.Uint32(buf[196:200])
	mat.VMG_PGCITI_Offset       = binary.BigEndian.Uint32(buf[200:204])
	mat.VMG_PTL_MAIT_Offset     = binary.BigEndian.Uint32(buf[204:208])
	mat.VMG_VTS_ATRT_Offset     = binary.BigEndian.Uint32(buf[208:212])
	mat.VMG_TXTDT_MG_Offset     = binary.BigEndian.Uint32(buf[212:216])
	mat.VMG_M_C_ADT_Offset      = binary.BigEndian.Uint32(buf[216:220])
	mat.VMG_M_VOBU_ADMAP_Offset = binary.BigEndian.Uint32(buf[220:224])
	return mat, nil
}
