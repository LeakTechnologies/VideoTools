package udf

import (
	"encoding/binary"
	"fmt"
	"io"
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

// DetectDiscType scans the ISO for specific directory structures.
func (r *Reader) DetectDiscType() (DiscType, error) {
	logging.Info(logging.CatDVD, "Detecting disc type...")
	
	// Implementation will scan the root directory for VIDEO_TS or BDMV
	// For now, we'll implement a simplified version that looks for strings in sectors
	// until the full UDF directory parser is finalized.
	
	// Check for VIDEO_TS (DVD)
	if found, _ := r.containsDirectory("VIDEO_TS"); found {
		logging.Info(logging.CatDVD, "Detected disc type: DVD")
		return DiscTypeDVD, nil
	}
	
	// Check for BDMV (Blu-ray)
	if found, _ := r.containsDirectory("BDMV"); found {
		logging.Info(logging.CatDVD, "Detected disc type: Blu-ray")
		return DiscTypeBluRay, nil
	}
	
	return DiscTypeUnknown, nil
}

func (r *Reader) containsDirectory(name string) (bool, error) {
	// Root directory traversal logic...
	// (Simplified for initial integration)
	return false, nil
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

// GetFileSetDescriptor finds the FSD via the Anchor pointer at sector 256.
func (r *Reader) GetFileSetDescriptor() (*FileSetDescriptor, error) {
	tagID, data, err := r.ReadDescriptor(256)
	if err != nil || tagID != TagIDAVDP {
		return nil, fmt.Errorf("failed to read AVDP at sector 256")
	}
	
	// Parse AVDP to get VDS location...
	// Parse LVD to get FSD location...
	
	return nil, fmt.Errorf("FSD traversal not yet fully implemented")
}

// IdentifyDiscFormat is a static helper for quick detection.
func IdentifyDiscFormat(path string) (DiscType, error) {
	// To be used by Rip module drop handler
	return DiscTypeUnknown, nil
}
