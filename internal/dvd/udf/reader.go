package udf

import (
	"encoding/binary"
	"io"
	"os"
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
// Note: This is a fast-path scanner that looks for directory signatures in the metadata.
func (r *Reader) DetectDiscType() (DiscType, error) {
	logging.Info(logging.CatDVD, "Detecting disc type via metadata scan...")
	
	// Scan first few MB for directory signatures
	buf := make([]byte, 1024*1024) // 1MB buffer
	if _, err := r.rs.Seek(0, io.SeekStart); err != nil {
		return DiscTypeUnknown, err
	}
	
	if _, err := io.ReadFull(r.rs, buf); err != nil {
		return DiscTypeUnknown, err
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
	
	data := make([]byte, dataLen)
	if _, err := io.ReadFull(r.rs, data); err != nil {
		return 0, nil, err
	}
	
	return tagID, data, nil
}
