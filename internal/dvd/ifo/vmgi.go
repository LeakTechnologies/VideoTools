package ifo

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"github.com/LeakTechnologies/VideoTools/internal/logging"
)

// VMG_MAT represents the Video Manager Information Management Table.
// Fields map to DVD-Video spec §3.2 byte offsets; see SerializeVMGMAT for layout.
type VMG_MAT struct {
	VMG_Identifier          [12]byte // "DVDVIDEO-VMG"
	VMG_Last_Sector         uint32
	VMG_BUP_Last_Sector     uint32 // VMGM_Last_Sector (last sector of VMGM VOBs; 0 if none)
	VMG_Category            uint32

	// Disc metadata (written by SerializeVMGMAT at correct byte offsets)
	NrOfTitleSets uint16 // total number of VTS (Video Title Sets) on this disc

	// First Play PGC (0x08E): byte offset within the IFO file of the PGC that
	// DVD players execute first when the disc is inserted. Set to the location
	// of the main menu PGC so the player shows the menu before starting playback.
	// A value of 0 means "no first play PGC — go directly to title 1".
	VMG_FirstPlayPGC uint32

	// Table Offsets — sector addresses within the IFO/disc (0x0C0–0x0DC per spec)
	VMGM_VOBS_Sector        uint32 // 0x0C0: start sector of VMGM VOBs (menu video; 0 if none)
	TT_SRPT_Offset          uint32 // 0x0C4: sector of Title Search Pointer Table
	VMG_PGCITI_Offset       uint32 // 0x0C8: sector of VMG Menu PGC Information Table (VMGM_PGCI_UT)
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

// vtsAtrtEntrySize is the fixed byte size of one serialized VTS_ATRT_Entry:
//   4  (VTS_MAT_Last_Sector)
//   2  (Video_Attrs packed)
//   2  (NumAudio)
//  64  (Audio_Attrs × 8)
//   2  (NumSubpicture)
// 192  (Subpicture_Attrs × 32)
// = 266 bytes
const vtsAtrtEntrySize = 4 + 2 + 2 + 8*8 + 2 + 32*6

// BuildVTS_ATRT constructs a VTS_ATRT from the slice of per-VTS attribute
// tables. Each VTS_MAT supplies the video/audio/subpicture attributes that
// hardware players cross-validate against the actual disc streams.
// Returns nil when mats is empty.
func BuildVTS_ATRT(mats []*VTS_MAT) *VTS_ATRT {
	n := len(mats)
	if n == 0 {
		return nil
	}
	entries := make([]VTS_ATRT_Entry, n)
	for i, m := range mats {
		entries[i] = VTS_ATRT_Entry{
			VTS_MAT_Last_Sector: m.VTS_Last_Sector,
			Video_Attrs:         m.VTS_Attributes,
			NumAudio:            m.VTS_Audio_Streams_Count,
			Audio_Attrs:         m.VTS_Audio_Attributes,
			NumSubpicture:       m.VTS_Subpicture_Count,
			Subpicture_Attrs:    m.VTS_Subpicture_Attrs,
		}
	}
	return &VTS_ATRT{
		NumVTS:  uint16(n),
		Entries: entries,
	}
}

// WriteVTS_ATRT serializes the VTS_ATRT and returns the sector-padded bytes.
//
// On-disc layout:
//
//	[0-1]  NumVTS (uint16)
//	[2-3]  Reserved (uint16)
//	[4-7]  EndByte (uint32) — last byte of table, 0-relative
//	[8 + i*4]  VTS_Offsets[i] (uint32) — byte offset to entry i from table start
//	[8 + N*4 + i*vtsAtrtEntrySize]  entry i:
//	  [0-3]   VTS_MAT_Last_Sector (uint32)
//	  [4-5]   Video_Attrs (2 bytes, packed)
//	  [6-7]   NumAudio (uint16)
//	  [8-71]  Audio_Attrs[8] (8 × 8 bytes)
//	  [72-73] NumSubpicture (uint16)
//	  [74-265] Subpicture_Attrs[32] (32 × 6 bytes)
func WriteVTS_ATRT(w io.Writer, table *VTS_ATRT) error {
	n := int(table.NumVTS)
	if n == 0 {
		return fmt.Errorf("WriteVTS_ATRT: empty table")
	}
	logging.Debug(logging.CatDVD, "Writing VTS_ATRT with %d entries", n)

	// Offset from start of table to the first entry.
	firstEntryOffset := uint32(8 + n*4)
	endByte := firstEntryOffset + uint32(n*vtsAtrtEntrySize) - 1

	var buf bytes.Buffer

	// Header
	binary.Write(&buf, binary.BigEndian, table.NumVTS)
	binary.Write(&buf, binary.BigEndian, table.Reserved)
	binary.Write(&buf, binary.BigEndian, endByte)

	// Offset array
	for i := 0; i < n; i++ {
		binary.Write(&buf, binary.BigEndian, firstEntryOffset+uint32(i*vtsAtrtEntrySize))
	}

	// Entries
	for _, e := range table.Entries {
		binary.Write(&buf, binary.BigEndian, e.VTS_MAT_Last_Sector)
		vb0, vb1 := packVideoAttrs(e.Video_Attrs)
		buf.WriteByte(vb0)
		buf.WriteByte(vb1)
		binary.Write(&buf, binary.BigEndian, e.NumAudio)
		for i := 0; i < 8; i++ {
			aa := e.Audio_Attrs[i]
			b0 := (aa.AudioCodingMode & 0x07) << 5
			if aa.Multichannel != 0 {
				b0 |= 0x10
			}
			b1 := ((aa.SampleRate & 0x03) << 4) | (aa.NumChannels & 0x07)
			buf.WriteByte(b0)
			buf.WriteByte(b1)
			buf.Write(aa.LanguageCode[:])
			buf.WriteByte(aa.SpecificCode)
			buf.Write([]byte{0, 0, 0}) // code_ext, unknown, app_mode_ext
		}
		binary.Write(&buf, binary.BigEndian, e.NumSubpicture)
		for i := 0; i < 32; i++ {
			sp := e.Subpicture_Attrs[i]
			buf.WriteByte(sp.CodingMode)
			buf.WriteByte(0x00) // reserved
			buf.Write(sp.LanguageCode[:])
			buf.WriteByte(sp.SpecificCode)
			buf.WriteByte(0x00) // code_extension
		}
	}

	// Pad to sector boundary
	if rem := buf.Len() % 2048; rem != 0 {
		buf.Write(make([]byte, 2048-rem))
	}

	_, err := w.Write(buf.Bytes())
	return err
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

// WriteVMGI serializes the VMG_MAT to an IFO file using the spec-correct
// offset-based layout from SerializeVMGMAT.
func WriteVMGI(w io.Writer, mat *VMG_MAT) error {
	logging.Info(logging.CatDVD, "Serializing VMGI Management Table (VMG_MAT)")
	if _, err := w.Write(SerializeVMGMAT(mat)); err != nil {
		logging.Error(logging.CatDVD, "Failed to write VMG_MAT: %v", err)
		return fmt.Errorf("write vmg_mat: %w", err)
	}
	logging.Debug(logging.CatDVD, "VMG_MAT written. Titles at offset: %d", mat.TT_SRPT_Offset)
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
	mat.VMG_Category            = binary.BigEndian.Uint32(buf[34:38])
	mat.NrOfTitleSets           = binary.BigEndian.Uint16(buf[62:64])
	mat.VMG_FirstPlayPGC        = binary.BigEndian.Uint32(buf[132:136])
	mat.VMGM_VOBS_Sector        = binary.BigEndian.Uint32(buf[192:196])
	mat.TT_SRPT_Offset          = binary.BigEndian.Uint32(buf[196:200])
	mat.VMG_PGCITI_Offset       = binary.BigEndian.Uint32(buf[200:204])
	mat.VMG_PTL_MAIT_Offset     = binary.BigEndian.Uint32(buf[204:208])
	mat.VMG_VTS_ATRT_Offset     = binary.BigEndian.Uint32(buf[208:212])
	mat.VMG_TXTDT_MG_Offset     = binary.BigEndian.Uint32(buf[212:216])
	mat.VMG_M_C_ADT_Offset      = binary.BigEndian.Uint32(buf[216:220])
	mat.VMG_M_VOBU_ADMAP_Offset = binary.BigEndian.Uint32(buf[220:224])
	return mat, nil
}
