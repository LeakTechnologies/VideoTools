package udf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

// Reader provides methods to read files from a UDF 1.02 ISO image.
type Reader struct {
	rs io.ReadSeeker
}

// NewReader creates a new UDF reader.
func NewReader(rs io.ReadSeeker) *Reader {
	return &Reader{rs: rs}
}

// DiscType represents the detected format of the disc.
type DiscType string

const (
	DiscTypeDVD     DiscType = "DVD"
	DiscTypeBluRay  DiscType = "Blu-ray"
	DiscTypeUnknown DiscType = "Unknown"
)

// DetectDiscType scans the ISO for specific signatures.
func (r *Reader) DetectDiscType() (DiscType, error) {
	logging.Info(logging.CatDVD, "Detecting disc type via metadata scan...")
	
	buf := make([]byte, 1024*1024) // 1MB buffer
	if _, err := r.rs.Seek(0, io.SeekStart); err != nil {
		return DiscTypeUnknown, err
	}
	
	if _, err := io.ReadFull(r.rs, buf); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			return DiscTypeUnknown, err
		}
	}
	
	content := string(buf)
	if strings.Contains(content, "VIDEO_TS") {
		return DiscTypeDVD, nil
	}
	if strings.Contains(content, "BDMV") {
		return DiscTypeBluRay, nil
	}
	
	return DiscTypeUnknown, nil
}

// IdentifyDiscFormat is a static helper for quick detection.
func IdentifyDiscFormat(path string) (DiscType, error) {
	f, err := os.Open(path)
	if err != nil {
		return DiscTypeUnknown, err
	}
	defer f.Close()
	
	r := NewReader(f)
	return r.DetectDiscType()
}

// ExtractDirectory extracts a directory (like VIDEO_TS) from the ISO to a local path.
func (r *Reader) ExtractDirectory(targetDir, destPath string) error {
	logging.Info(logging.CatDVD, "Extracting directory %s from ISO to %s", targetDir, destPath)
	
	// Implementation:
	// For Phase 5 initial integration, we've established the detection.
	// We are currently implementing the sector-by-sector extraction logic.
	// For Rip module to be 100% native, we must handle ICB/FID parsing.
	
	return fmt.Errorf("native UDF extraction is a work-in-progress - please use folder source for now")
}

// ReadDescriptor reads a UDF descriptor from a specific sector.
func (r *Reader) ReadDescriptor(sector uint32) (uint16, []byte, error) {
	if _, err := r.rs.Seek(int64(sector)*SectorSize, io.SeekStart); err != nil {
		return 0, nil, err
	}
	
	header := make([]byte, 16)
	if _, err := io.ReadFull(r.rs, header); err != nil {
		return 0, nil, err
	}
	
	tagID := binary.LittleEndian.Uint16(header[0:2])
	dataLen := binary.LittleEndian.Uint16(header[12:14])
	
	if dataLen > SectorSize-16 {
		dataLen = SectorSize - 16
	}

	data := make([]byte, dataLen)
	if _, err := io.ReadFull(r.rs, data); err != nil {
		return 0, nil, err
	}
	
	return tagID, data, nil
}

func (r *Reader) readAVDP() (*AnchorVolumeDescriptorPointer, error) {
	tagID, data, err := r.ReadDescriptor(256)
	if err != nil {
		return nil, err
	}
	if tagID != TagIDAVDP {
		return nil, fmt.Errorf("not an AVDP (tag %d)", tagID)
	}
	avdp := &AnchorVolumeDescriptorPointer{}
	err = binary.Read(bytes.NewReader(data), binary.LittleEndian, avdp)
	return avdp, err
}

// GetVolumeLabel returns the disc label from the PVD.
func (r *Reader) GetVolumeLabel() (string, error) {
	tagID, data, err := r.ReadDescriptor(32) // PVD is usually at 32
	if err != nil || tagID != TagIDPVD {
		return "", fmt.Errorf("failed to read PVD")
	}
	pvd := &PrimaryVolumeDescriptor{}
	binary.Read(bytes.NewReader(data), binary.LittleEndian, pvd)
	
	// Decode CS0 label
	label := string(pvd.VolumeIdentifier[1:])
	return strings.TrimRight(label, "\x00 "), nil
}
