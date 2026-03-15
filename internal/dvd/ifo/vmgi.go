package ifo

import (
	"encoding/binary"
	"fmt"
	"io"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

// VMG_MAT represents the Video Manager Information Management Table.
type VMG_MAT struct {
	VMG_Identifier          [12]byte // "DVDVIDEO-VMG"
	VMG_Last_Sector         uint32
	VMG_BUP_Last_Sector     uint32
	VMG_MAT_Last_Sector     uint32
	Reserved1               uint32
	VMG_Category            uint32
	VMG_Attributes          VideoAttributes
	VMG_Audio_Streams_Count uint16
	VMG_Audio_Attributes    [8]AudioAttributes
	VMG_Subpicture_Count    uint16
	VMG_Subpicture_Attrs    [32]SubpictureAttributes
	
	// Table Offsets (relative to sector 0)
	TT_SRPT_Offset          uint32 // Title Search Pointer Table
	VMG_PTT_SRPT_Offset     uint32 // Part of Title Search Pointer Table
	VMG_PGCITI_Offset       uint32 // VMG PGC Information Table
	VMG_M_PGCI_UT_Offset    uint32 // Menu PGC Unit Table
	VMG_PTL_MAIT_Offset     uint32 // Parental Management Information
	VMG_VTS_ATRT_Offset     uint32 // VTS Attribute Table
	VMG_TXTDT_MG_Offset     uint32 // Text Data Manager
	VMG_M_C_ADT_Offset      uint32 // Menu Cell Address Table
	VMG_M_VOBU_ADMAP_Offset uint32 // Menu VOBU Address Map
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
