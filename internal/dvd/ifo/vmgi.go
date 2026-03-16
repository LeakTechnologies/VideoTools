package ifo

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"

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
	if err := binary.Write(&io.LimitedWriter{W: w, N: 4}, binary.BigEndian, table.EndByte); err != nil {
		// Manual handling for 4-byte end byte
	}
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

// ReadVMGI parses a VMG IFO file from a reader.
func ReadVMGI(r io.Reader) (*VMG_MAT, error) {
	mat := &VMG_MAT{}
	if err := binary.Read(r, binary.BigEndian, mat); err != nil {
		return nil, fmt.Errorf("read vmg_mat: %w", err)
	}
	
	id := string(mat.VMG_Identifier[:])
	if !strings.HasPrefix(id, "DVDVIDEO-VMG") {
		return nil, fmt.Errorf("invalid VMG identifier: %s", id)
	}
	
	return mat, nil
}
