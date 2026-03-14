package udf

import (
	"bytes"
	"encoding/binary"
	"io"
	"time"
)

// Standard DVD sector size.
const SectorSize = 2048

// UDF Character Set types.
const (
	CharSetTypeCS0 = 0
)

// Tag Identifier types for UDF descriptors.
const (
	TagIDPVD  = 1   // Primary Volume Descriptor
	TagIDAVDP = 2   // Anchor Volume Descriptor Pointer
	TagIDVDS  = 3   // Volume Descriptor Pointer
	TagIDIUVD = 4   // Implementation Use Volume Descriptor
	TagIDPD   = 5   // Partition Descriptor
	TagIDLVD  = 6   // Logical Volume Descriptor
	TagIDUSD  = 7   // Unallocated Space Descriptor
	TagIDTD   = 8   // Terminating Descriptor
	TagIDLVID = 9   // Logical Volume Integrity Descriptor
	TagIDFSD  = 256 // File Set Descriptor
	TagIDFID  = 257 // File Identifier Descriptor
	TagIDICB  = 261 // File Entry (ICB)
	TagIDTerm = 266 // Terminal Entry
)

// DescriptorTag is the common 16-byte header for all UDF descriptors.
type DescriptorTag struct {
	TagIdentifier     uint16
	DescriptorVersion uint16
	TagChecksum       uint8
	Reserved          uint8
	TagSerialNumber   uint16
	DescriptorCRC     uint16
	DescriptorCRCLen  uint16
	TagLocation       uint32
}

// ExtentAd describes an extent of data (length and location).
type ExtentAd struct {
	Len      uint32
	Location uint32
}

// EntityID identifies an implementation or domain.
type EntityID struct {
	Flags      uint8
	Identifier [23]byte
	Suffix     [8]byte
}

// AnchorVolumeDescriptorPointer (AVDP) - Located at sector 256.
type AnchorVolumeDescriptorPointer struct {
	Tag                        DescriptorTag
	MainVolumeDescriptorSeq    ExtentAd
	ReserveVolumeDescriptorSeq ExtentAd
	Reserved                   [480]byte
}

// PrimaryVolumeDescriptor (PVD) - Basic volume information.
type PrimaryVolumeDescriptor struct {
	Tag                                 DescriptorTag
	VolumeDescriptorSeqNumber           uint32
	PrimaryVolumeDescriptorNumber       uint32
	VolumeIdentifier                    [32]byte
	VolumeSequenceNumber                uint16
	MaximumVolumeSequenceNumber         uint16
	InterchangeLevel                    uint16
	MaximumInterchangeLevel             uint16
	CharacterSetList                    uint32
	MaximumCharacterSetList             uint32
	VolumeSetIdentifier                 [128]byte
	DescriptorCharacterSet              CharSpec
	ExplanatoryCharacterSet             CharSpec
	VolumeAbstract                      ExtentAd
	VolumeCopyrightNotice               ExtentAd
	ApplicationIdentifier               EntityID
	RecordingDateAndTime                Timestamp
	ImplementationIdentifier            EntityID
	ImplementationUse                   [64]byte
	PredecessorVolumeDescriptorSeqLocation uint32
	Flags                               uint16
	Reserved                            [22]byte
}

// LogicalVolumeDescriptor (LVD) - Defines the logical volume and partitions.
type LogicalVolumeDescriptor struct {
	Tag                           DescriptorTag
	VolumeDescriptorSeqNumber     uint32
	DescriptorCharacterSet        CharSpec
	LogicalVolumeIdentifier       [128]byte
	LogicalBlockSize              uint32
	DomainIdentifier              EntityID
	LogicalVolumeContentsUse      [16]byte
	MapTableLength                uint32
	NumberOfPartitionMaps         uint32
	ImplementationIdentifier      EntityID
	ImplementationUse             [128]byte
	IntegritySequenceExtent       ExtentAd
	PartitionMaps                 []byte // Variable length
}

// PartitionDescriptor (PD) - Defines a physical partition on the volume.
type PartitionDescriptor struct {
	Tag                       DescriptorTag
	VolumeDescriptorSeqNumber uint32
	PartitionFlags            uint16
	PartitionNumber           uint16
	PartitionContents         EntityID
	PartitionContentsUse      [128]byte
	AccessType                uint32
	PartitionStartingLocation uint32
	PartitionLength           uint32
	ImplementationIdentifier  EntityID
	ImplementationUse         [128]byte
	Reserved                  [156]byte
}

// CharSpec defines a character set.
type CharSpec struct {
	CharacterSetType uint8
	CharacterSetInfo [63]byte
}

// Timestamp represents a UDF timestamp.
type Timestamp struct {
	TypeAndTimezone uint16
	Year            int16
	Month           uint8
	Day             uint8
	Hour            uint8
	Minute          uint8
	Second          uint8
	Centiseconds    uint8
	HundredsOfMicro uint8
	Microseconds    uint8
}

// Writer represents a UDF 1.02 ISO writer.
type Writer struct {
	w             io.Writer
	volumeLabel   string
	currentSector uint32
	files         []*FileEntry
	volumeTime    time.Time
}

// FileEntry represents a file or directory to be included in the UDF.
type FileEntry struct {
	Name        string
	IsDir       bool
	Size        int64
	Content     io.Reader
	ModTime     time.Time
	StartSector uint32
	Parent      *FileEntry
	Children    []*FileEntry
}

// NewWriter creates a new UDF 1.02 writer.
func NewWriter(w io.Writer, volumeLabel string) *Writer {
	return &Writer{
		w:           w,
		volumeLabel: volumeLabel,
		volumeTime:  time.Now(),
	}
}

// AddFile adds a file to the UDF structure.
func (uw *Writer) AddFile(name string, size int64, content io.Reader, modTime time.Time) error {
	uw.files = append(uw.files, &FileEntry{
		Name:    name,
		Size:    size,
		Content: content,
		ModTime: modTime,
	})
	return nil
}

// CalculateChecksum calculates the UDF descriptor tag checksum.
func CalculateChecksum(data []byte) uint8 {
	var sum uint8
	// Checksum is the sum of bytes 0-3 and 5-15 of the tag.
	for i := 0; i < 16; i++ {
		if i != 4 { // Skip TagChecksum field itself
			sum += data[i]
		}
	}
	return sum
}

// CalculateCRC calculates the UDF descriptor CRC (CRC-ITUT).
func CalculateCRC(data []byte) uint16 {
	var crc uint16 = 0
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// WriteDescriptor writes a UDF descriptor with automatic tag header and CRC calculation.
func (uw *Writer) WriteDescriptor(tagID uint16, descriptor interface{}) error {
	var buf bytes.Buffer
	// Create placeholder tag
	tag := DescriptorTag{
		TagIdentifier:     tagID,
		DescriptorVersion: 2,
		TagLocation:       uw.currentSector,
	}

	// Write tag then descriptor data
	if err := binary.Write(&buf, binary.LittleEndian, tag); err != nil {
		return err
	}
	if err := binary.Write(&buf, binary.LittleEndian, descriptor); err != nil {
		return err
	}

	data := buf.Bytes()
	
	// Calculate CRC for the data after the tag (16 bytes)
	crcLen := uint16(len(data) - 16)
	crc := CalculateCRC(data[16:])
	
	// Update Tag header fields in the buffer
	binary.LittleEndian.PutUint16(data[10:12], crc)
	binary.LittleEndian.PutUint16(data[12:14], crcLen)
	
	// Calculate Checksum for the tag (first 16 bytes)
	checksum := CalculateChecksum(data[:16])
	data[4] = checksum

	// Write padded sector
	fullSector := make([]byte, SectorSize)
	copy(fullSector, data)
	if _, err := uw.w.Write(fullSector); err != nil {
		return err
	}
	
	uw.currentSector++
	return nil
}

func (uw *Writer) writePadding(sectors int) error {
	padding := make([]byte, SectorSize)
	for i := 0; i < sectors; i++ {
		if _, err := uw.w.Write(padding); err != nil {
			return err
		}
		uw.currentSector++
	}
	return nil
}

// NewTimestamp creates a UDF timestamp from a time.Time.
func NewTimestamp(t time.Time) Timestamp {
	return Timestamp{
		TypeAndTimezone: 0x1000, // Local time
		Year:            int16(t.Year()),
		Month:           uint8(t.Month()),
		Day:             uint8(t.Day()),
		Hour:            uint8(t.Hour()),
		Minute:          uint8(t.Minute()),
		Second:          uint8(t.Second()),
	}
}
