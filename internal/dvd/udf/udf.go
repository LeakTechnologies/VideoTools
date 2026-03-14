package udf

import (
	"bytes"
	"encoding/binary"
	"fmt"
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

// LongAd describes a location within a partition.
type LongAd struct {
	Len      uint32
	Location uint32 // Logical Block Number
	Partition uint16
	ImplementationUse [6]byte
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
	PartitionMaps                 [64]byte // Fixed for now
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

// FileSetDescriptor (FSD) - Defines the root of a file set.
type FileSetDescriptor struct {
	Tag                        DescriptorTag
	RecordingDateAndTime       Timestamp
	InterchangeLevel           uint16
	MaximumInterchangeLevel    uint16
	CharacterSetList           uint32
	MaximumCharacterSetList    uint32
	FileSetNumber              uint32
	FileSetDescriptorNumber    uint32
	LogicalVolumeIdentifierCharSpec CharSpec
	FileSetIdentifier          [32]byte
	CopyrightFileIdentifier    [32]byte
	AbstractFileIdentifier     [32]byte
	RootDirectoryICB           LongAd
	DomainIdentifier           EntityID
	NextExtent                 LongAd
	SystemStreamDirectoryICB   LongAd
	Reserved                   [48]byte
}

// FileIdentifierDescriptor (FID) - Directory entry.
type FileIdentifierDescriptor struct {
	Tag                        DescriptorTag
	FileVersionNumber          uint16
	FileCharacteristics        uint8
	LengthOfFileIdentifier     uint8
	ICB                        LongAd
	LengthOfImplementationUse  uint16
	// ImplementationUse and FileIdentifier follow
}

// FileEntry (ICB) - Metadata for a file or directory.
type FileEntryICB struct {
	Tag                        DescriptorTag
	ICBTag                     ICBTag
	Uid                        uint32
	Gid                        uint32
	Permissions                uint32
	FileLinkCount              uint16
	RecordFormat               uint8
	RecordDisplayAttributes    uint8
	RecordLength               uint32
	InformationLength          uint64
	LogicalBlocksRecorded      uint64
	AccessTime                 Timestamp
	ModificationTime           Timestamp
	AttributeTime              Timestamp
	Checkpoint                 uint32
	ExtendedAttributeICB       LongAd
	ImplementationIdentifier   EntityID
	UniqueId                   uint64
	LengthOfExtendedAttributes uint32
	LengthOfAllocationDescriptors uint32
	// ExtendedAttributes and AllocationDescriptors follow
}

// ICBTag describes the type of ICB.
type ICBTag struct {
	PriorDirectEntryCount      uint32
	StrategyType               uint16
	StrategyParameter          uint16
	MaximumNumberOfEntries     uint16
	Reserved                   uint8
	FileType                   uint8
	ParentICBLocation          ExtentAd
	Flags                      uint16
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
	files         []*FileNode
	volumeTime    time.Time
}

// FileNode represents a file or directory in the UDF tree.
type FileNode struct {
	Name        string
	IsDir       bool
	Size        int64
	Content     io.Reader
	ModTime     time.Time
	ICBSector   uint32
	DataSector  uint32
	Children    []*FileNode
}

// NewWriter creates a new UDF 1.02 writer.
func NewWriter(w io.Writer, volumeLabel string) *Writer {
	return &Writer{
		w:           w,
		volumeLabel: volumeLabel,
		volumeTime:  time.Now(),
	}
}

// CalculateChecksum calculates the UDF descriptor tag checksum.
func CalculateChecksum(data []byte) uint8 {
	var sum uint8
	for i := 0; i < 16; i++ {
		if i != 4 {
			sum += data[i]
		}
	}
	return sum
}

// CalculateCRC calculates the UDF descriptor CRC.
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
	tag := DescriptorTag{
		TagIdentifier:     tagID,
		DescriptorVersion: 2,
		TagLocation:       uw.currentSector,
	}

	if err := binary.Write(&buf, binary.LittleEndian, tag); err != nil {
		return err
	}
	if err := binary.Write(&buf, binary.LittleEndian, descriptor); err != nil {
		return err
	}

	data := buf.Bytes()
	crcLen := uint16(len(data) - 16)
	crc := CalculateCRC(data[16:])
	
	binary.LittleEndian.PutUint16(data[10:12], crc)
	binary.LittleEndian.PutUint16(data[12:14], crcLen)
	
	checksum := CalculateChecksum(data[:16])
	data[4] = checksum

	fullSector := make([]byte, SectorSize)
	copy(fullSector, data)
	if _, err := uw.w.Write(fullSector); err != nil {
		return err
	}
	
	uw.currentSector++
	return nil
}

// WriteHeader writes the initial ISO 9660 and UDF structures.
func (uw *Writer) WriteHeader() error {
	// 1. System Area (Sectors 0-15)
	if err := uw.writePadding(16); err != nil {
		return err
	}

	// 2. ISO 9660 PVD (Sector 16)
	pvd := ISO9660PrimaryVolumeDescriptor{
		Type:       ISO9660PVDType,
		Identifier: [5]byte{'C', 'D', '0', '0', '1'},
		Version:    1,
	}
	copy(pvd.VolumeIdentifier[:], uw.volumeLabel)
	if err := uw.writeSector(pvd); err != nil {
		return err
	}

	// 3. ISO 9660 Terminator (Sector 17)
	term := make([]byte, SectorSize)
	term[0] = ISO9660TermType
	copy(term[1:6], "CD001")
	term[6] = 1
	if _, err := uw.w.Write(term); err != nil {
		return err
	}
	uw.currentSector++

	// 4. Padding to Sector 32
	if err := uw.writePadding(32 - int(uw.currentSector)); err != nil {
		return err
	}

	// 5. UDF VDS Sequence (Sector 32-47)
	if err := uw.writeVDS(); err != nil {
		return err
	}

	// 6. Padding to Sector 256
	if err := uw.writePadding(256 - int(uw.currentSector)); err != nil {
		return err
	}

	// 7. UDF AVDP (Sector 256)
	avdp := AnchorVolumeDescriptorPointer{
		MainVolumeDescriptorSeq: ExtentAd{Len: 16 * SectorSize, Location: 32},
	}
	return uw.WriteDescriptor(TagIDAVDP, avdp)
}

func (uw *Writer) writeVDS() error {
	// PVD
	upvd := PrimaryVolumeDescriptor{
		VolumeDescriptorSeqNumber: 0,
	}
	copy(upvd.VolumeIdentifier[:], EncodeCS0(uw.volumeLabel, 32))
	if err := uw.WriteDescriptor(TagIDPVD, upvd); err != nil {
		return err
	}

	// LVD
	lvd := LogicalVolumeDescriptor{
		LogicalBlockSize: SectorSize,
	}
	if err := uw.WriteDescriptor(TagIDLVD, lvd); err != nil {
		return err
	}

	// PD
	pd := PartitionDescriptor{
		PartitionStartingLocation: 257,
		PartitionLength:           1000, // Dummy
	}
	if err := uw.WriteDescriptor(TagIDPD, pd); err != nil {
		return err
	}

	// Terminating
	if err := uw.writePadding(1); err != nil { 
		return err
	}

	// Pad remainder of 16 sectors
	return uw.writePadding(16 - (int(uw.currentSector) - 32))
}

func (uw *Writer) writeSector(data interface{}) error {
	fullSector := make([]byte, SectorSize)
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, data); err != nil {
		return err
	}
	copy(fullSector, buf.Bytes())
	if _, err := uw.w.Write(fullSector); err != nil {
		return err
	}
	uw.currentSector++
	return nil
}

func (uw *Writer) writePadding(sectors int) error {
	if sectors <= 0 {
		return nil
	}
	padding := make([]byte, SectorSize)
	for i := 0; i < sectors; i++ {
		if _, err := uw.w.Write(padding); err != nil {
			return err
		}
		uw.currentSector++
	}
	return nil
}

// EncodeCS0 encodes a string into UDF CS0.
func EncodeCS0(s string, length int) []byte {
	buf := make([]byte, length)
	if s == "" {
		return buf
	}
	buf[0] = 8 // Compression byte
	copy(buf[1:], s)
	return buf
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
