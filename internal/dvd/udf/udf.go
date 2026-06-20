package udf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LeakTechnologies/VideoTools/internal/logging"
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

// partitionStart is the absolute sector where the UDF partition begins.
// All LBN (Logical Block Number) values in UDF structures are relative to this.
const partitionStart = uint32(257)

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
	Len               uint32
	Location          uint32 // Logical Block Number
	Partition         uint16
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
	Tag                                    DescriptorTag
	VolumeDescriptorSeqNumber              uint32
	PrimaryVolumeDescriptorNumber          uint32
	VolumeIdentifier                       [32]byte
	VolumeSequenceNumber                   uint16
	MaximumVolumeSequenceNumber            uint16
	InterchangeLevel                       uint16
	MaximumInterchangeLevel                uint16
	CharacterSetList                       uint32
	MaximumCharacterSetList                uint32
	VolumeSetIdentifier                    [128]byte
	DescriptorCharacterSet                 CharSpec
	ExplanatoryCharacterSet                CharSpec
	VolumeAbstract                         ExtentAd
	VolumeCopyrightNotice                  ExtentAd
	ApplicationIdentifier                  EntityID
	RecordingDateAndTime                   Timestamp
	ImplementationIdentifier               EntityID
	ImplementationUse                      [64]byte
	PredecessorVolumeDescriptorSeqLocation uint32
	Flags                                  uint16
	Reserved                               [22]byte
}

// LogicalVolumeDescriptor (LVD) - Defines the logical volume and partitions.
type LogicalVolumeDescriptor struct {
	Tag                       DescriptorTag
	VolumeDescriptorSeqNumber uint32
	DescriptorCharacterSet    CharSpec
	LogicalVolumeIdentifier   [128]byte
	LogicalBlockSize          uint32
	DomainIdentifier          EntityID
	LogicalVolumeContentsUse  [16]byte
	MapTableLength            uint32
	NumberOfPartitionMaps     uint32
	ImplementationIdentifier  EntityID
	ImplementationUse         [128]byte
	IntegritySequenceExtent   ExtentAd
	PartitionMaps             [64]byte // Fixed for now
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
	Tag                             DescriptorTag
	RecordingDateAndTime            Timestamp
	InterchangeLevel                uint16
	MaximumInterchangeLevel         uint16
	CharacterSetList                uint32
	MaximumCharacterSetList         uint32
	FileSetNumber                   uint32
	FileSetDescriptorNumber         uint32
	LogicalVolumeIdentifierCharSpec CharSpec
	FileSetIdentifier               [32]byte
	CopyrightFileIdentifier         [32]byte
	AbstractFileIdentifier          [32]byte
	RootDirectoryICB                LongAd
	DomainIdentifier                EntityID
	NextExtent                      LongAd
	SystemStreamDirectoryICB        LongAd
	Reserved                        [48]byte
}

// FileIdentifierDescriptor (FID) - Directory entry.
type FileIdentifierDescriptor struct {
	Tag                       DescriptorTag
	FileVersionNumber         uint16
	FileCharacteristics       uint8
	LengthOfFileIdentifier    uint8
	ICB                       LongAd
	LengthOfImplementationUse uint16
}

// FileEntry (ICB) - Metadata for a file or directory.
type FileEntryICB struct {
	Tag                           DescriptorTag
	ICBTag                        ICBTag
	Uid                           uint32
	Gid                           uint32
	Permissions                   uint32
	FileLinkCount                 uint16
	RecordFormat                  uint8
	RecordDisplayAttributes       uint8
	RecordLength                  uint32
	InformationLength             uint64
	LogicalBlocksRecorded         uint64
	AccessTime                    Timestamp
	ModificationTime              Timestamp
	AttributeTime                 Timestamp
	Checkpoint                    uint32
	ExtendedAttributeICB          LongAd
	ImplementationIdentifier      EntityID
	UniqueId                      uint64
	LengthOfExtendedAttributes    uint32
	LengthOfAllocationDescriptors uint32
}

// ICBTag describes the type of ICB.
type ICBTag struct {
	PriorDirectEntryCount  uint32
	StrategyType           uint16
	StrategyParameter      uint16
	MaximumNumberOfEntries uint16
	Reserved               uint8
	FileType               uint8
	ParentICBLocation      ExtentAd
	Flags                  uint16
}

// ShortAd is a UDF short allocation descriptor (8 bytes).
// Position is an LBN relative to the partition start.
type ShortAd struct {
	Len      uint32
	Position uint32
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
	root          *FileNode
	volumeTime    time.Time
}

// FileNode represents a file or directory in the UDF tree.
type FileNode struct {
	Name       string
	IsDir      bool
	Size       int64
	Content    io.Reader
	LocalPath  string
	ModTime    time.Time
	ICBSector  uint32
	DataSector uint32
	Children   []*FileNode
}

// NewWriter creates a new UDF 1.02 writer.
func NewWriter(w io.Writer, volumeLabel string) *Writer {
	logging.Info(logging.CatDVD, "Creating new UDF 1.02 writer for label: %s", volumeLabel)
	return &Writer{
		w:           w,
		volumeLabel: volumeLabel,
		volumeTime:  time.Now(),
		root: &FileNode{
			Name:  "",
			IsDir: true,
		},
	}
}

// AddFile adds a file to the root or a subdirectory.
func (uw *Writer) AddFile(path []string, name string, size int64, content io.Reader, modTime time.Time) error {
	dir := uw.findDir(path)
	if dir == nil {
		err := fmt.Errorf("directory not found: %v", path)
		logging.Error(logging.CatDVD, "AddFile failed: %v", err)
		return err
	}
	logging.Debug(logging.CatDVD, "Adding file: %s (size: %d) to path: %v", name, size, path)
	dir.Children = append(dir.Children, &FileNode{
		Name:    name,
		IsDir:   false,
		Size:    size,
		Content: content,
		ModTime: modTime,
	})
	return nil
}

// AddDirectory adds a directory.
func (uw *Writer) AddDirectory(path []string, name string, modTime time.Time) error {
	dir := uw.findDir(path)
	if dir == nil {
		err := fmt.Errorf("parent directory not found: %v", path)
		logging.Error(logging.CatDVD, "AddDirectory failed: %v", err)
		return err
	}
	logging.Debug(logging.CatDVD, "Adding directory: %s to path: %v", name, path)
	dir.Children = append(dir.Children, &FileNode{
		Name:    name,
		IsDir:   true,
		ModTime: modTime,
	})
	return nil
}

// AddDirFS recursively adds a local directory tree to the UDF writer.
// Files are stored with their host path for deferred opening during Build();
// file handles are not opened until Build() is called. This means the caller
// can overwrite files on disk between AddDirFS and Build() (e.g., to patch IFO
// sector addresses) and Build() will read the updated content.
func (uw *Writer) AddDirFS(localPath string) error {
	logging.Info(logging.CatDVD, "Recursively adding directory: %s", localPath)
	return filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(localPath, path)
		if err != nil {
			return err
		}

		if rel == "." {
			return nil
		}

		dirParts := strings.Split(filepath.Dir(rel), string(filepath.Separator))
		if dirParts[0] == "." {
			dirParts = nil
		}

		if info.IsDir() {
			return uw.AddDirectory(dirParts, info.Name(), info.ModTime())
		}

		// Store path for deferred open — no file handle held here.
		dir := uw.findDir(dirParts)
		if dir == nil {
			return fmt.Errorf("directory not found for %s: %v", info.Name(), dirParts)
		}
		dir.Children = append(dir.Children, &FileNode{
			Name:      info.Name(),
			IsDir:     false,
			Size:      info.Size(),
			LocalPath: path,
			ModTime:   info.ModTime(),
		})
		return nil
	})
}

// FileInfo holds the disc sector allocation for a single file node.
type FileInfo struct {
	DataSector  uint32 // first sector of file data (absolute LBA from disc start)
	SectorCount uint32 // total 2048-byte sectors occupied by this file
}

// PreAssignSectors runs the sector allocation algorithm without writing any
// data and returns a map of disc paths (e.g. "/VIDEO_TS/VTS_01_0.IFO") to
// their computed disc-level sector positions. Safe to call before Build();
// Build() re-runs the same algorithm and produces identical results as long as
// the file tree is not modified between calls.
func (uw *Writer) PreAssignSectors() map[string]FileInfo {
	uw.assignSectors()
	result := make(map[string]FileInfo)
	var walk func(n *FileNode, path string)
	walk = func(n *FileNode, path string) {
		fullPath := path + "/" + n.Name
		if !n.IsDir {
			count := uint32((n.Size + int64(SectorSize) - 1) / int64(SectorSize))
			result[fullPath] = FileInfo{DataSector: n.DataSector, SectorCount: count}
		}
		for _, child := range n.Children {
			walk(child, fullPath)
		}
	}
	for _, child := range uw.root.Children {
		walk(child, "")
	}
	return result
}

func (uw *Writer) findDir(path []string) *FileNode {
	curr := uw.root
	for _, p := range path {
		found := false
		for _, child := range curr.Children {
			if child.IsDir && child.Name == p {
				curr = child
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}
	return curr
}

// Build finalizes the UDF structure and writes all data.
func (uw *Writer) Build() error {
	logging.Info(logging.CatDVD, "Starting UDF Build process...")

	logging.Debug(logging.CatDVD, "Assigning sectors to files and directories")
	uw.assignSectors()

	// Compute total sectors so PVD VolumeSpaceSize is accurate.
	totalSectors := uw.totalSectors()

	iso := buildISO9660Layout(uw.root, 18, uw.volumeTime)

	if err := uw.writeHeaderWithISO9660(iso, totalSectors); err != nil {
		logging.Error(logging.CatDVD, "Failed to write header: %v", err)
		return fmt.Errorf("udf write header: %w", err)
	}

	// FSD at sector 257. RootDirectoryICB.Location must be an LBN relative
	// to the partition start (257), not an absolute sector number.
	logging.Debug(logging.CatDVD, "Writing File Set Descriptor (FSD) at sector %d", uw.currentSector)
	fsd := FileSetDescriptor{
		RecordingDateAndTime: NewTimestamp(uw.volumeTime),
		RootDirectoryICB: LongAd{
			Len:      SectorSize,
			Location: uw.root.ICBSector - partitionStart,
		},
	}
	if err := uw.WriteDescriptor(TagIDFSD, fsd); err != nil {
		logging.Error(logging.CatDVD, "Failed to write FSD: %v", err)
		return fmt.Errorf("udf write fsd: %w", err)
	}

	logging.Info(logging.CatDVD, "Writing file data nodes recursively")
	if err := uw.writeNode(uw.root); err != nil {
		logging.Error(logging.CatDVD, "Failed to write file nodes: %v", err)
		return fmt.Errorf("udf write nodes: %w", err)
	}

	logging.Info(logging.CatDVD, "UDF Build process completed successfully. Final sector: %d", uw.currentSector)
	return nil
}

func (uw *Writer) assignSectors() {
	// Start after AVDP (Sector 256) and FSD (Sector 257)
	nextSector := uint32(258)

	var walk func(n *FileNode)
	walk = func(n *FileNode) {
		n.ICBSector = nextSector
		nextSector++
		if n.IsDir {
			n.DataSector = nextSector
			nextSector++
			for _, child := range n.Children {
				walk(child)
			}
		} else {
			n.DataSector = nextSector
			numSectors := uint32((n.Size + SectorSize - 1) / SectorSize)
			nextSector += numSectors
		}
	}
	walk(uw.root)
}

// totalSectors returns the absolute sector number of the first byte past the last
// file's data. Call only after assignSectors().
func (uw *Writer) totalSectors() uint32 {
	var max uint32
	var walk func(n *FileNode)
	walk = func(n *FileNode) {
		var end uint32
		if n.IsDir {
			end = n.DataSector + 1
		} else {
			end = n.DataSector + uint32((n.Size+SectorSize-1)/SectorSize)
		}
		if end > max {
			max = end
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(uw.root)
	return max
}

func (uw *Writer) writeNode(n *FileNode) error {
	if uw.currentSector != n.ICBSector {
		padding := int(n.ICBSector) - int(uw.currentSector)
		if err := uw.writePadding(padding); err != nil {
			return err
		}
	}

	if n.IsDir {
		if err := uw.writeDirEntry(n); err != nil {
			return err
		}
	} else {
		// File ICB: append a ShortAd pointing to the data sectors.
		if err := uw.writeFileEntry(n); err != nil {
			return err
		}
	}

	if uw.currentSector != n.DataSector {
		padding := int(n.DataSector) - int(uw.currentSector)
		if err := uw.writePadding(padding); err != nil {
			return err
		}
	}

	if n.IsDir {
		if err := uw.writeDirectoryData(n); err != nil {
			return err
		}
	} else {
		if n.Content != nil {
			if _, err := io.CopyN(uw.w, n.Content, n.Size); err != nil {
				return err
			}
			if rc, ok := n.Content.(io.ReadCloser); ok {
				rc.Close()
			}
		} else if n.LocalPath != "" {
			f, err := os.Open(n.LocalPath)
			if err != nil {
				return fmt.Errorf("open %s: %w", n.LocalPath, err)
			}
			if _, err := io.CopyN(uw.w, f, n.Size); err != nil {
				f.Close()
				return fmt.Errorf("copy %s: %w", n.LocalPath, err)
			}
			f.Close()
		}
		uw.currentSector += uint32((n.Size + SectorSize - 1) / SectorSize)
		padding := SectorSize - int(n.Size%SectorSize)
		if padding != SectorSize {
			if _, err := uw.w.Write(make([]byte, padding)); err != nil {
				return err
			}
		}
	}

	for _, child := range n.Children {
		if err := uw.writeNode(child); err != nil {
			return err
		}
	}
	return nil
}

func (uw *Writer) writeDirectoryData(n *FileNode) error {
	var dirBuf bytes.Buffer

	writeFID := func(name string, characteristics uint8, icbLBN uint32) {
		// CS0-encode the identifier (or empty for "." / ".." self-reference entries).
		var identBytes []byte
		if name != "" {
			identBytes = make([]byte, 1+len(name))
			identBytes[0] = 8 // CS0 compression byte
			copy(identBytes[1:], name)
		}
		identLen := uint8(len(identBytes))

		// FID body (everything after the 16-byte tag):
		// FileVersionNumber(2) + FileCharacteristics(1) + LengthOfFileIdentifier(1)
		// + ICB LongAd(16) + LengthOfImplementationUse(2) + FileIdentifier + padding
		var body bytes.Buffer
		binary.Write(&body, binary.LittleEndian, uint16(1)) // FileVersionNumber
		body.WriteByte(characteristics)
		body.WriteByte(identLen)
		icbLong := LongAd{Len: SectorSize, Location: icbLBN}
		binary.Write(&body, binary.LittleEndian, icbLong)
		binary.Write(&body, binary.LittleEndian, uint16(0)) // LengthOfImplementationUse
		if identLen > 0 {
			body.Write(identBytes)
		}
		// Pad entire FID (tag + body) to 4-byte boundary.
		total := 16 + body.Len()
		if total%4 != 0 {
			body.Write(make([]byte, 4-total%4))
		}

		bodyBytes := body.Bytes()
		crc := CalculateCRC(bodyBytes)
		tag := DescriptorTag{
			TagIdentifier:     TagIDFID,
			DescriptorVersion: 2,
			TagLocation:       uw.currentSector,
			DescriptorCRC:     crc,
			DescriptorCRCLen:  uint16(len(bodyBytes)),
		}
		var tagBuf bytes.Buffer
		binary.Write(&tagBuf, binary.LittleEndian, tag)
		tagBytes := tagBuf.Bytes()
		tagBytes[4] = CalculateChecksum(tagBytes)

		dirBuf.Write(tagBytes)
		dirBuf.Write(bodyBytes)
	}

	// "." — self reference (FileCharacteristics bit 3 = Parent set for self pointer)
	selfLBN := n.ICBSector - partitionStart
	writeFID("", 0x08, selfLBN) // bit3=Parent (used for self in some impls; bit1=dir not needed here)

	// ".." — parent reference (simplified: points to self for root)
	writeFID("", 0x0A, selfLBN) // bit1=dir, bit3=parent

	for _, child := range n.Children {
		childLBN := child.ICBSector - partitionStart
		fc := uint8(0x00)
		if child.IsDir {
			fc = 0x02
		}
		writeFID(child.Name, fc, childLBN)
	}

	sector := make([]byte, SectorSize)
	copy(sector, dirBuf.Bytes())
	if _, err := uw.w.Write(sector); err != nil {
		return err
	}
	uw.currentSector++
	return nil
}

// writeDirEntry writes a directory FileEntryICB with a ShortAd pointing to the
// directory data sector. Mirrors writeFileEntry but uses FileType=4 (directory).
func (uw *Writer) writeDirEntry(n *FileNode) error {
	dataLBN := n.DataSector - partitionStart
	alloc := ShortAd{
		Len:      SectorSize,
		Position: dataLBN,
	}
	allocSize := uint32(binary.Size(alloc))

	icb := FileEntryICB{
		ICBTag: ICBTag{
			StrategyType: 4,
			FileType:     4, // directory
			Flags:        0, // short allocation descriptors
		},
		Permissions:                   0x14A5,
		FileLinkCount:                 1,
		InformationLength:             uint64(SectorSize),
		LogicalBlocksRecorded:         1,
		AccessTime:                    NewTimestamp(n.ModTime),
		ModificationTime:              NewTimestamp(n.ModTime),
		AttributeTime:                 NewTimestamp(n.ModTime),
		Checkpoint:                    1,
		LengthOfAllocationDescriptors: allocSize,
	}

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, icb); err != nil {
		return err
	}
	if err := binary.Write(&buf, binary.LittleEndian, alloc); err != nil {
		return err
	}
	data := buf.Bytes()

	crcLen := uint16(len(data) - 16)
	crc := CalculateCRC(data[16:])
	binary.LittleEndian.PutUint16(data[8:10], crc)
	binary.LittleEndian.PutUint16(data[10:12], crcLen)
	binary.LittleEndian.PutUint16(data[0:2], TagIDICB)
	binary.LittleEndian.PutUint16(data[2:4], 2)
	binary.LittleEndian.PutUint32(data[12:16], uw.currentSector)
	crc = CalculateCRC(data[16:])
	binary.LittleEndian.PutUint16(data[8:10], crc)
	data[4] = CalculateChecksum(data[:16])

	sector := make([]byte, SectorSize)
	copy(sector, data)
	if _, err := uw.w.Write(sector); err != nil {
		return err
	}
	uw.currentSector++
	return nil
}

// writeFileEntry writes a FileEntryICB followed by a ShortAd allocation descriptor
// that points to the file's data sectors. The entire record fits in one sector.
func (uw *Writer) writeFileEntry(n *FileNode) error {
	numSectors := uint32((n.Size + SectorSize - 1) / SectorSize)
	dataLBN := n.DataSector - partitionStart

	alloc := ShortAd{
		Len:      uint32(n.Size),
		Position: dataLBN,
	}
	allocSize := uint32(binary.Size(alloc))

	icb := FileEntryICB{
		ICBTag: ICBTag{
			StrategyType: 4,
			FileType:     5, // regular file
			Flags:        1, // short allocation descriptors
		},
		Permissions:                   0x14A5,
		FileLinkCount:                 1,
		InformationLength:             uint64(n.Size),
		LogicalBlocksRecorded:         uint64(numSectors),
		AccessTime:                    NewTimestamp(n.ModTime),
		ModificationTime:              NewTimestamp(n.ModTime),
		AttributeTime:                 NewTimestamp(n.ModTime),
		Checkpoint:                    1,
		LengthOfAllocationDescriptors: allocSize,
	}

	// Serialize full ICB struct (includes DescriptorTag placeholder at bytes 0-15)
	// then append the ShortAd, compute CRC over bytes 16..end, then fix up the tag.
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, icb); err != nil {
		return err
	}
	if err := binary.Write(&buf, binary.LittleEndian, alloc); err != nil {
		return err
	}
	data := buf.Bytes()

	// CRC covers everything after the 16-byte tag.
	crcLen := uint16(len(data) - 16)
	crc := CalculateCRC(data[16:])
	binary.LittleEndian.PutUint16(data[8:10], crc)     // DescriptorCRC at offset 8
	binary.LittleEndian.PutUint16(data[10:12], crcLen) // DescriptorCRCLen at offset 10

	// Tag fields: TagIdentifier(0-1), DescriptorVersion(2-3), TagChecksum(4),
	// Reserved(5), TagSerialNumber(6-7), DescriptorCRC(8-9), DescriptorCRCLen(10-11),
	// TagLocation(12-15)
	binary.LittleEndian.PutUint16(data[0:2], TagIDICB)
	binary.LittleEndian.PutUint16(data[2:4], 2)                  // DescriptorVersion
	binary.LittleEndian.PutUint32(data[12:16], uw.currentSector) // TagLocation
	// Recompute CRC after setting TagLocation
	crc = CalculateCRC(data[16:])
	binary.LittleEndian.PutUint16(data[8:10], crc)
	data[4] = CalculateChecksum(data[:16])

	sector := make([]byte, SectorSize)
	copy(sector, data)
	if _, err := uw.w.Write(sector); err != nil {
		return err
	}
	uw.currentSector++
	return nil
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

// WriteDescriptor writes a UDF descriptor. Each descriptor struct must start
// with an embedded DescriptorTag (16 bytes) as its first field; this function
// patches those bytes with the correct tag ID, CRC, and sector location rather
// than prepending a second tag header.
func (uw *Writer) WriteDescriptor(tagID uint16, descriptor interface{}) error {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, descriptor); err != nil {
		return fmt.Errorf("binary write descriptor: %w", err)
	}
	data := buf.Bytes()

	// Ensure at least 16 bytes so the embedded DescriptorTag can be patched.
	for len(data) < 16 {
		data = append(data, 0)
	}

	// CRC covers all bytes after the 16-byte tag (the descriptor payload).
	crcLen := uint16(len(data) - 16)
	crc := CalculateCRC(data[16:])

	// Patch the embedded DescriptorTag at bytes 0-15.
	binary.LittleEndian.PutUint16(data[0:2], tagID)
	binary.LittleEndian.PutUint16(data[2:4], 2) // DescriptorVersion
	data[4] = 0                                  // TagChecksum placeholder
	data[5] = 0                                  // Reserved
	binary.LittleEndian.PutUint16(data[6:8], 0)  // TagSerialNumber
	binary.LittleEndian.PutUint16(data[8:10], crc)
	binary.LittleEndian.PutUint16(data[10:12], crcLen)
	binary.LittleEndian.PutUint32(data[12:16], uw.currentSector)
	data[4] = CalculateChecksum(data[:16])

	fullSector := make([]byte, SectorSize)
	copy(fullSector, data)
	if _, err := uw.w.Write(fullSector); err != nil {
		return fmt.Errorf("write sector %d: %w", uw.currentSector, err)
	}

	uw.currentSector++
	return nil
}

// writeHeaderWithISO9660 writes the initial ISO 9660 + UDF header sectors
// with a fully populated PVD (path tables and root directory record).
func (uw *Writer) writeHeaderWithISO9660(iso *iso9660Layout, totalSectors uint32) error {
	// Sectors 0-15: System Area (blank)
	if err := uw.writePadding(16); err != nil {
		return err
	}

	// Sector 16: ISO 9660 Primary Volume Descriptor
	ptSize := uint32(len(iso.PathTableL))
	rootDirSector := iso.DirSectors[0]

	pvd := ISO9660PrimaryVolumeDescriptor{
		Type:                 ISO9660PVDType,
		Identifier:           [5]byte{'C', 'D', '0', '0', '1'},
		Version:              1,
		FileStructureVersion: 1,
		TypeLPathTable:       iso.LPathSector,
		TypeMPathTable:       iso.MPathSector,
		RootDirectoryRecord:  buildRootDirRecord(rootDirSector, uw.volumeTime),
		VolumeSpaceSize:      isoLEBE32(totalSectors),
	}
	copy(pvd.VolumeIdentifier[:], uw.volumeLabel)
	pvd.VolumeSetSize = [4]byte{1, 0, 0, 1}        // both-endian uint16 = 1
	pvd.VolumeSequenceNumber = [4]byte{1, 0, 0, 1} // both-endian uint16 = 1
	pvd.LogicalBlockSize = [4]byte{0, 8, 8, 0}     // both-endian uint16 = 2048
	lebe := isoLEBE32(ptSize)
	copy(pvd.PathTableSize[:], lebe[:])
	pvd.VolumeCreationDate = NewISOTimestamp(uw.volumeTime)
	pvd.VolumeModificationDate = NewISOTimestamp(uw.volumeTime)
	copy(pvd.ApplicationIdentifier[:], "VIDEOTOOLS")

	if err := uw.writeSector(pvd); err != nil {
		return err
	}

	// Sector 17: Volume Descriptor Set Terminator
	term := make([]byte, SectorSize)
	term[0] = ISO9660TermType
	copy(term[1:6], "CD001")
	term[6] = 1
	if _, err := uw.w.Write(term); err != nil {
		return err
	}
	uw.currentSector++ // now at 18

	// Sector 18: L-type Path Table (little-endian)
	if err := uw.writePaddedSector(iso.PathTableL); err != nil {
		return err
	}
	// Sector 19: M-type Path Table (big-endian)
	if err := uw.writePaddedSector(iso.PathTableM); err != nil {
		return err
	}
	// Sectors 20+: ISO 9660 directory sectors
	for _, dirSector := range iso.Dirs {
		if _, err := uw.w.Write(dirSector); err != nil {
			return err
		}
		uw.currentSector++
	}

	// Pad to sector 32 for UDF VDS
	if err := uw.writePadding(32 - int(uw.currentSector)); err != nil {
		return err
	}

	if err := uw.writeVDS(totalSectors); err != nil {
		return err
	}

	if err := uw.writePadding(256 - int(uw.currentSector)); err != nil {
		return err
	}

	avdp := AnchorVolumeDescriptorPointer{
		MainVolumeDescriptorSeq: ExtentAd{Len: 16 * SectorSize, Location: 32},
	}
	return uw.WriteDescriptor(TagIDAVDP, avdp)
}

// writePaddedSector writes data zero-padded to exactly one SectorSize sector.
func (uw *Writer) writePaddedSector(data []byte) error {
	sector := make([]byte, SectorSize)
	if len(data) <= SectorSize {
		copy(sector, data)
	}
	if _, err := uw.w.Write(sector); err != nil {
		return err
	}
	uw.currentSector++
	return nil
}

func (uw *Writer) writeVDS(totalSectors uint32) error {
	upvd := PrimaryVolumeDescriptor{
		VolumeDescriptorSeqNumber: 1,
	}
	copy(upvd.VolumeIdentifier[:], EncodeCS0(uw.volumeLabel, 32))
	if err := uw.WriteDescriptor(TagIDPVD, upvd); err != nil {
		return err
	}

	// LogicalVolumeContentsUse holds a LongAd pointing to the File Set Descriptor.
	// The FSD lives at partition LBN 0 (absolute sector 257 = partitionStart + 0).
	lvd := LogicalVolumeDescriptor{
		VolumeDescriptorSeqNumber: 2,
		LogicalBlockSize:          SectorSize,
		// Type 1 Partition Map for partition 0 (6 bytes).
		MapTableLength:        6,
		NumberOfPartitionMaps: 1,
	}
	// OSTA UDF Compliant domain identifier (required for UDF 1.02 volumes).
	copy(lvd.DomainIdentifier.Identifier[:], "*OSTA UDF Compliant")
	lvd.DomainIdentifier.Suffix[0] = 0x02 // UDF revision minor (1.02)
	lvd.DomainIdentifier.Suffix[1] = 0x01 // UDF revision major
	// Type 1 Partition Map: type(1) + length(1) + VolumeSequenceNumber(2) + PartitionNumber(2)
	lvd.PartitionMaps[0] = 1 // PartitionMapType = 1
	lvd.PartitionMaps[1] = 6 // PartitionMapLength = 6
	binary.LittleEndian.PutUint16(lvd.PartitionMaps[2:4], 1) // VolumeSequenceNumber = 1
	binary.LittleEndian.PutUint16(lvd.PartitionMaps[4:6], 0) // PartitionNumber = 0
	// Encode FSD LongAd into LogicalVolumeContentsUse[0:16]
	// LongAd: Len(4) + Location(4) + Partition(2) + ImplementationUse(6)
	binary.LittleEndian.PutUint32(lvd.LogicalVolumeContentsUse[0:4], uint32(SectorSize))
	binary.LittleEndian.PutUint32(lvd.LogicalVolumeContentsUse[4:8], 0)  // LBN 0 = FSD
	binary.LittleEndian.PutUint16(lvd.LogicalVolumeContentsUse[8:10], 0) // partition 0
	if err := uw.WriteDescriptor(TagIDLVD, lvd); err != nil {
		return err
	}

	pd := PartitionDescriptor{
		VolumeDescriptorSeqNumber: 3,
		PartitionFlags:            1, // allocated
		PartitionStartingLocation: partitionStart,
		PartitionLength:           totalSectors - partitionStart,
	}
	if err := uw.WriteDescriptor(TagIDPD, pd); err != nil {
		return err
	}

	// Terminating Descriptor closes the VDS sequence.
	if err := uw.WriteDescriptor(TagIDTD, struct{}{}); err != nil {
		return err
	}

	return uw.writePadding(16 - (int(uw.currentSector) - 32))
}

func (uw *Writer) writeSector(data interface{}) error {
	fullSector := make([]byte, SectorSize)
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, data); err != nil {
		return fmt.Errorf("binary write sector: %w", err)
	}
	copy(fullSector, buf.Bytes())
	if _, err := uw.w.Write(fullSector); err != nil {
		return fmt.Errorf("write sector %d: %w", uw.currentSector, err)
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
			return fmt.Errorf("write padding sector %d: %w", uw.currentSector, err)
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
