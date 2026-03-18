package udf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

// ISO 9660 Volume Descriptor types.
const (
	ISO9660PVDType  = 1
	ISO9660SVDType  = 2
	ISO9660VPDType  = 3
	ISO9660TermType = 255
)

// ISO9660PrimaryVolumeDescriptor represents the ISO 9660 PVD.
type ISO9660PrimaryVolumeDescriptor struct {
	Type                        uint8
	Identifier                  [5]byte // "CD001"
	Version                     uint8
	Unused1                     uint8
	SystemIdentifier            [32]byte
	VolumeIdentifier            [32]byte
	Unused2                     [8]byte
	VolumeSpaceSize             [8]byte // both-endian uint32
	Unused3                     [32]byte
	VolumeSetSize               [4]byte
	VolumeSequenceNumber        [4]byte
	LogicalBlockSize            [4]byte
	PathTableSize               [8]byte // both-endian uint32
	TypeLPathTable              uint32
	OptionalTypeLPathTable      uint32
	TypeMPathTable              uint32
	OptionalTypeMPathTable      uint32
	RootDirectoryRecord         [34]byte
	VolumeSetIdentifier         [128]byte
	PublisherIdentifier         [128]byte
	DataPreparerIdentifier      [128]byte
	ApplicationIdentifier       [128]byte
	CopyrightFileIdentifier     [37]byte
	AbstractFileIdentifier      [37]byte
	BibliographicFileIdentifier [37]byte
	VolumeCreationDate          ISOTimestamp
	VolumeModificationDate      ISOTimestamp
	VolumeExpirationDate        ISOTimestamp
	VolumeEffectiveDate         ISOTimestamp
	FileStructureVersion        uint8
	Unused4                     uint8
	ApplicationUse              [512]byte
	Reserved                    [653]byte
}

// ISOTimestamp is the ISO 9660 17-byte date/time field.
type ISOTimestamp struct {
	Year       [4]byte
	Month      [2]byte
	Day        [2]byte
	Hour       [2]byte
	Minute     [2]byte
	Second     [2]byte
	Hundredths [2]byte
	Offset     int8
}

// NewISOTimestamp creates an ISO 9660 17-byte timestamp from a time.Time.
func NewISOTimestamp(t time.Time) ISOTimestamp {
	ts := ISOTimestamp{}
	copy(ts.Year[:], fmt.Sprintf("%04d", t.Year()))
	copy(ts.Month[:], fmt.Sprintf("%02d", int(t.Month())))
	copy(ts.Day[:], fmt.Sprintf("%02d", t.Day()))
	copy(ts.Hour[:], fmt.Sprintf("%02d", t.Hour()))
	copy(ts.Minute[:], fmt.Sprintf("%02d", t.Minute()))
	copy(ts.Second[:], fmt.Sprintf("%02d", t.Second()))
	copy(ts.Hundredths[:], "00")
	return ts
}

// isoDate7 encodes a time.Time as the 7-byte ISO 9660 directory-record timestamp.
func isoDate7(t time.Time) [7]byte {
	return [7]byte{
		uint8(t.Year() - 1900),
		uint8(t.Month()),
		uint8(t.Day()),
		uint8(t.Hour()),
		uint8(t.Minute()),
		uint8(t.Second()),
		0, // GMT offset quarters (0 = UTC)
	}
}

// isoLEBE32 encodes a uint32 in ISO 9660 both-endian format (LE then BE, 8 bytes).
func isoLEBE32(v uint32) [8]byte {
	var b [8]byte
	binary.LittleEndian.PutUint32(b[0:4], v)
	binary.BigEndian.PutUint32(b[4:8], v)
	return b
}

// isoLEBE16 encodes a uint16 in ISO 9660 both-endian format (LE then BE, 4 bytes).
func isoLEBE16(v uint16) [4]byte {
	var b [4]byte
	binary.LittleEndian.PutUint16(b[0:2], v)
	binary.BigEndian.PutUint16(b[2:4], v)
	return b
}

// writeISO9660DirRecord appends a single ISO 9660 directory record to buf.
// name must be uppercase; for "." use []byte{0x00}; for ".." use []byte{0x01}.
func writeISO9660DirRecord(buf *bytes.Buffer, name []byte, isDir bool, extentSector, dataSize uint32, t time.Time) {
	nameLen := byte(len(name))
	// Record length: 33 fixed + nameLen, padded to even
	recLen := byte(33 + nameLen)
	if recLen%2 != 0 {
		recLen++
	}

	buf.WriteByte(recLen)      // Length of Directory Record
	buf.WriteByte(0x00)        // Extended Attribute Record Length
	le32 := isoLEBE32(extentSector)
	buf.Write(le32[:])         // Location of Extent (both-endian)
	le32 = isoLEBE32(dataSize)
	buf.Write(le32[:])         // Data Length (both-endian)
	d := isoDate7(t)
	buf.Write(d[:])            // Recording Date and Time (7 bytes)
	if isDir {
		buf.WriteByte(0x02)    // File Flags: Directory
	} else {
		buf.WriteByte(0x00)    // File Flags: Regular file
	}
	buf.WriteByte(0x00)        // File Unit Size
	buf.WriteByte(0x00)        // Interleave Gap Size
	le16 := isoLEBE16(1)
	buf.Write(le16[:])         // Volume Sequence Number (both-endian)
	buf.WriteByte(nameLen)     // Length of File Identifier
	buf.Write(name)            // File Identifier
	if (33+nameLen)%2 != 0 {
		buf.WriteByte(0x00)    // Padding byte (to even boundary)
	}
}

// buildISO9660PathTableEntry builds one path table entry (L or M endian).
func buildISO9660PathTableEntry(dirName string, extentSector uint32, parentDirNum uint16, bigEndian bool) []byte {
	nameBytes := []byte(strings.ToUpper(dirName))
	nameLen := byte(len(nameBytes))
	var buf bytes.Buffer
	buf.WriteByte(nameLen)
	buf.WriteByte(0x00) // Extended Attribute Record Length
	if bigEndian {
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, extentSector)
		buf.Write(b)
		b2 := make([]byte, 2)
		binary.BigEndian.PutUint16(b2, parentDirNum)
		buf.Write(b2)
	} else {
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, extentSector)
		buf.Write(b)
		b2 := make([]byte, 2)
		binary.LittleEndian.PutUint16(b2, parentDirNum)
		buf.Write(b2)
	}
	buf.Write(nameBytes)
	if nameLen%2 != 0 {
		buf.WriteByte(0x00) // Padding to even length
	}
	return buf.Bytes()
}

// iso9660Layout precomputes sector numbers and byte content for ISO 9660
// structures (path tables and directory sectors). File data sectors are shared
// with UDF — no duplication.
//
// Sector assignments (relative to firstSector):
//
//	firstSector+0 : L-type path table
//	firstSector+1 : M-type path table
//	firstSector+2 : root directory
//	firstSector+3 : first child directory (if any)
//	...
type iso9660Layout struct {
	LPathSector uint32
	MPathSector uint32
	DirSectors  []uint32 // [root, child0, child1, ...]
	PathTableL  []byte
	PathTableM  []byte
	Dirs        [][]byte // one 2048-byte sector per directory
}

// buildISO9660Layout computes the full ISO 9660 layout from the FileNode tree.
// root must have DataSector values already assigned (i.e. assignSectors() called).
// firstSector is the sector where the L-type path table will be placed (typically 18).
func buildISO9660Layout(root *FileNode, firstSector uint32, volumeTime time.Time) *iso9660Layout {
	// Collect directories in order: root first, then root's subdirectories.
	type dirEntry struct {
		node         *FileNode
		parentDirNum uint16 // 1-based ISO 9660 directory number of parent
	}
	var dirs []dirEntry
	dirs = append(dirs, dirEntry{root, 1})
	for _, child := range root.Children {
		if child.IsDir {
			dirs = append(dirs, dirEntry{child, 1}) // parent = root = 1
		}
	}

	lyt := &iso9660Layout{
		LPathSector: firstSector,
		MPathSector: firstSector + 1,
	}
	for i := range dirs {
		lyt.DirSectors = append(lyt.DirSectors, firstSector+2+uint32(i))
	}

	// Build path tables.
	dirSectorOf := func(i int) uint32 { return lyt.DirSectors[i] }

	buildPathTable := func(bigEndian bool) []byte {
		var buf bytes.Buffer
		// Root entry: name = 0x00 (1 byte)
		entry := buildISO9660PathTableEntry("\x00", dirSectorOf(0), 1, bigEndian)
		buf.Write(entry)
		// Subdirectory entries
		for i := 1; i < len(dirs); i++ {
			entry = buildISO9660PathTableEntry(dirs[i].node.Name, dirSectorOf(i), dirs[i].parentDirNum, bigEndian)
			buf.Write(entry)
		}
		return buf.Bytes()
	}
	lyt.PathTableL = buildPathTable(false)
	lyt.PathTableM = buildPathTable(true)

	// Build directory sectors.
	for i, de := range dirs {
		sector := make([]byte, SectorSize)
		var buf bytes.Buffer
		selfSector := dirSectorOf(i)
		parentSector := dirSectorOf(0) // default parent = root
		if i == 0 {
			parentSector = selfSector // root's ".." points to itself
		}

		// "." entry
		writeISO9660DirRecord(&buf, []byte{0x00}, true, selfSector, SectorSize, volumeTime)
		// ".." entry
		writeISO9660DirRecord(&buf, []byte{0x01}, true, parentSector, SectorSize, volumeTime)

		if i == 0 {
			// Root: list child directories
			for j := 1; j < len(dirs); j++ {
				name := []byte(strings.ToUpper(dirs[j].node.Name))
				writeISO9660DirRecord(&buf, name, true, dirSectorOf(j), SectorSize, volumeTime)
			}
		} else {
			// Subdirectory: list its files
			for _, child := range de.node.Children {
				if child.IsDir {
					continue // nested subdirs not supported in this layout
				}
				// ISO 9660 filename must be UPPERCASE with ";1" version suffix
				name := []byte(strings.ToUpper(child.Name) + ";1")
				writeISO9660DirRecord(&buf, name, false, child.DataSector, uint32(child.Size), volumeTime)
			}
		}

		copy(sector, buf.Bytes())
		lyt.Dirs = append(lyt.Dirs, sector)
	}

	return lyt
}

// buildRootDirRecord builds the 34-byte PVD root directory record.
func buildRootDirRecord(rootDirSector uint32, volumeTime time.Time) [34]byte {
	var buf bytes.Buffer
	writeISO9660DirRecord(&buf, []byte{0x00}, true, rootDirSector, SectorSize, volumeTime)
	var rec [34]byte
	copy(rec[:], buf.Bytes())
	return rec
}
