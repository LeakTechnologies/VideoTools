package iso9660

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// buildISO9660Image creates an in-memory ISO 9660-only volume containing a
// VIDEO_TS directory (IFO + multi-sector VOB) and an empty AUDIO_TS directory.
// Layout: 16=PVD, 17=VDS terminator, 18=root, 19=VIDEO_TS, 20=IFO file,
// 21-22=VOB file, 23=AUDIO_TS. "." and ".." entries use the single-byte
// identifiers 0x00/0x01 as defined by ISO 9660.
func buildISO9660Image(t *testing.T, ifoData, vobData []byte) []byte {
	t.Helper()
	img := make([]byte, 24*SectorSize)

	pvd := sector(img, pvdSector)
	pvd[0] = pvdType
	copy(pvd[1:], isoMagic)
	pvd[6] = 1
	copy(pvd[156:], makeDirRecord(18, 2048, dirFlag, "\x00")) // root directory record
	sector(img, 17)[0] = 255                                  // VDS terminator

	root := make([]byte, 0, SectorSize)
	root = append(root, makeDirRecord(18, 2048, dirFlag, "\x00")...)
	root = append(root, makeDirRecord(18, 2048, dirFlag, "\x01")...)
	root = append(root, makeDirRecord(19, 2048, dirFlag, "VIDEO_TS")...)
	root = append(root, makeDirRecord(23, 2048, dirFlag, "AUDIO_TS")...)
	copy(sector(img, 18), root)

	vt := make([]byte, 0, SectorSize)
	vt = append(vt, makeDirRecord(19, 2048, dirFlag, "\x00")...)
	vt = append(vt, makeDirRecord(18, 2048, dirFlag, "\x01")...)
	vt = append(vt, makeDirRecord(20, uint32(len(ifoData)), 0, "VIDEO_TS.IFO;1")...)
	vt = append(vt, makeDirRecord(21, uint32(len(vobData)), 0, "VTS_01_1.VOB;1")...)
	copy(sector(img, 19), vt)

	copy(sector(img, 20), ifoData)
	copy(sector(img, 21), vobData[:SectorSize])
	if len(vobData) > SectorSize {
		copy(sector(img, 22), vobData[SectorSize:])
	}

	aud := make([]byte, 0, SectorSize)
	aud = append(aud, makeDirRecord(23, 2048, dirFlag, "\x00")...)
	aud = append(aud, makeDirRecord(18, 2048, dirFlag, "\x01")...)
	copy(sector(img, 23), aud)

	return img
}

const dirFlag byte = 0x02

func sector(img []byte, n int) []byte { return img[n*SectorSize : (n+1)*SectorSize] }

// makeDirRecord builds one ISO 9660 directory record. Records are packed
// contiguously: the parser advances by the declared record length byte, so the
// returned slice is exactly recLen bytes (no inter-record padding).
func makeDirRecord(extent, size uint32, flags byte, name string) []byte {
	recLen := 33 + len(name)
	b := make([]byte, recLen)
	b[0] = byte(recLen)
	b[25] = flags
	binary.LittleEndian.PutUint32(b[2:6], extent)
	binary.LittleEndian.PutUint32(b[10:14], size)
	b[32] = byte(len(name))
	copy(b[33:], []byte(name))
	return b
}

func TestReadFileData(t *testing.T) {
	ifo := bytes.Repeat([]byte{0xAB}, 512)
	vob := make([]byte, 2*SectorSize)
	vob[0] = 0x42
	vob[SectorSize] = 0x99

	r := NewReader(bytes.NewReader(buildISO9660Image(t, ifo, vob)))

	got, err := r.ReadFileData("VIDEO_TS/VIDEO_TS.IFO")
	if err != nil {
		t.Fatalf("ReadFileData IFO: %v", err)
	}
	if !bytes.Equal(got, ifo) {
		t.Fatalf("IFO mismatch: got %d bytes, want %d", len(got), len(ifo))
	}

	got, err = r.ReadFileData("video_ts/video_ts.ifo")
	if err != nil {
		t.Fatalf("case-insensitive read: %v", err)
	}
	if !bytes.Equal(got, ifo) {
		t.Fatalf("case-insensitive IFO mismatch")
	}

	got, err = r.ReadFileData("VIDEO_TS/VTS_01_1.VOB")
	if err != nil {
		t.Fatalf("ReadFileData VOB: %v", err)
	}
	if len(got) != len(vob) || got[0] != 0x42 || got[SectorSize] != 0x99 {
		t.Fatalf("VOB mismatch: len=%d first=%#x mid=%#x", len(got), got[0], got[SectorSize])
	}
}

func TestReadFileDataMissing(t *testing.T) {
	r := NewReader(bytes.NewReader(buildISO9660Image(t, []byte{1}, make([]byte, 2*SectorSize))))
	if _, err := r.ReadFileData("VIDEO_TS/NOPE.IFO"); err == nil {
		t.Fatal("expected error for missing file")
	}
	if _, err := r.ReadFileData("PLAIN_TS/VIDEO_TS.IFO"); err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestExtractDirectory(t *testing.T) {
	ifo := []byte("IFO-HEADER")
	vobData := make([]byte, 2*SectorSize)
	copy(vobData, "VOB-CONTENT")

	r := NewReader(bytes.NewReader(buildISO9660Image(t, ifo, vobData)))
	dest := t.TempDir()
	if err := r.ExtractDirectory(context.Background(), "VIDEO_TS", dest); err != nil {
		t.Fatalf("ExtractDirectory: %v", err)
	}

	gotIFO, err := os.ReadFile(filepath.Join(dest, "VIDEO_TS.IFO"))
	if err != nil {
		t.Fatalf("read extracted IFO: %v", err)
	}
	if !bytes.Equal(gotIFO, ifo) {
		t.Fatalf("extracted IFO mismatch: %q", gotIFO)
	}

	gotVOB, err := os.ReadFile(filepath.Join(dest, "VTS_01_1.VOB"))
	if err != nil {
		t.Fatalf("read extracted VOB: %v", err)
	}
	if !bytes.Equal(gotVOB, vobData) {
		t.Fatalf("extracted VOB mismatch")
	}
}

func TestNotAValidISO9660(t *testing.T) {
	img := make([]byte, 24*SectorSize)
	copy(sector(img, pvdSector)[1:], "NOTISO")
	r := NewReader(bytes.NewReader(img))
	if err := r.ExtractDirectory(context.Background(), "VIDEO_TS", t.TempDir()); err == nil {
		t.Fatal("expected error extracting from non-ISO 9660 image")
	}
	if _, err := r.ReadFileData("VIDEO_TS/VIDEO_TS.IFO"); err == nil {
		t.Fatal("expected error reading from non-ISO 9660 image")
	}
}