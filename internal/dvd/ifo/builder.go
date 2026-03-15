package ifo

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

// Builder coordinates the construction of IFO and BUP files.
type Builder struct {
	outputDir string
}

// NewBuilder creates a new IFO builder.
func NewBuilder(outputDir string) *Builder {
	return &Builder{outputDir: outputDir}
}

// GenerateVTS_IFO creates VTS_xx_0.IFO and VTS_xx_0.BUP.
func (b *Builder) GenerateVTS_IFO(vtsNumber int, mat *VTS_MAT) error {
	filename := fmt.Sprintf("VTS_%02d_0.IFO", vtsNumber)
	ifoPath := filepath.Join(b.outputDir, filename)
	bupPath := filepath.Join(b.outputDir, fmt.Sprintf("VTS_%02d_0.BUP", vtsNumber))
	
	logging.Info(logging.CatDVD, "Generating IFO/BUP for VTS %d", vtsNumber)
	
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, mat); err != nil {
		return fmt.Errorf("serialize vts_mat: %w", err)
	}
	
	// Pad to 2048 bytes (minimum IFO size)
	if buf.Len() < 2048 {
		buf.Write(make([]byte, 2048-buf.Len()))
	}
	
	data := buf.Bytes()
	
	// Write IFO
	if err := os.WriteFile(ifoPath, data, 0644); err != nil {
		logging.Error(logging.CatDVD, "Failed to write IFO file %s: %v", ifoPath, err)
		return err
	}
	
	// Write BUP (Exact copy)
	if err := os.WriteFile(bupPath, data, 0644); err != nil {
		logging.Error(logging.CatDVD, "Failed to write BUP file %s: %v", bupPath, err)
		return err
	}
	
	logging.Debug(logging.CatDVD, "VTS %d IFO/BUP files created successfully", vtsNumber)
	return nil
}

// GenerateVMG_IFO creates VIDEO_TS.IFO and VIDEO_TS.BUP.
func (b *Builder) GenerateVMG_IFO(mat *VMG_MAT) error {
	ifoPath := filepath.Join(b.outputDir, "VIDEO_TS.IFO")
	bupPath := filepath.Join(b.outputDir, "VIDEO_TS.BUP")
	
	logging.Info(logging.CatDVD, "Generating main VMGI IFO/BUP")
	
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, mat); err != nil {
		return fmt.Errorf("serialize vmg_mat: %w", err)
	}
	
	// Pad to 2048 bytes
	if buf.Len() < 2048 {
		buf.Write(make([]byte, 2048-buf.Len()))
	}
	
	data := buf.Bytes()
	
	if err := os.WriteFile(ifoPath, data, 0644); err != nil {
		logging.Error(logging.CatDVD, "Failed to write VIDEO_TS.IFO: %v", err)
		return err
	}
	
	if err := os.WriteFile(bupPath, data, 0644); err != nil {
		logging.Error(logging.CatDVD, "Failed to write VIDEO_TS.BUP: %v", err)
		return err
	}
	
	logging.Debug(logging.CatDVD, "Main VMGI IFO/BUP files created successfully")
	return nil
}
