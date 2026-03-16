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
func (b *Builder) GenerateVTS_IFO(vtsNumber int, mat *VTS_MAT, admap *VOBU_ADMAP) error {
	filename := fmt.Sprintf("VTS_%02d_0.IFO", vtsNumber)
	ifoPath := filepath.Join(b.outputDir, filename)
	bupPath := filepath.Join(b.outputDir, fmt.Sprintf("VTS_%02d_0.BUP", vtsNumber))
	
	logging.Info(logging.CatDVD, "Generating IFO/BUP for VTS %d", vtsNumber)
	
	var buf bytes.Buffer
	
	// If admap is provided, calculate its offset and update MAT
	if admap != nil {
		// MAT is always 2048 bytes (1 sector)
		mat.VTS_VOBU_ADMAP_Offset = 1 
		// Set last sector (simplified: MAT(1) + ADMAP sectors)
		admapLen := 4 + (len(admap.Sectors) * 4)
		admapSectors := uint32((admapLen + 2047) / 2048)
		mat.VTS_Last_Sector = 1 + admapSectors - 1
		mat.VTS_MAT_Last_Sector = 0 // Sector 0 is the MAT itself
	}

	// 1. Write MAT (Sector 0)
	if err := binary.Write(&buf, binary.BigEndian, mat); err != nil {
		return fmt.Errorf("serialize vts_mat: %w", err)
	}
	
	// Pad MAT to 2048 bytes
	if buf.Len() < 2048 {
		buf.Write(make([]byte, 2048-buf.Len()))
	}
	
	// 2. Write VOBU_ADMAP (Sector 1+)
	if admap != nil {
		if err := WriteVOBU_ADMAP(&buf, admap); err != nil {
			return fmt.Errorf("serialize vobu_admap: %w", err)
		}
		// Pad to sector boundary
		if buf.Len()%2048 != 0 {
			padding := 2048 - (buf.Len() % 2048)
			buf.Write(make([]byte, padding))
		}
	}
	
	data := buf.Bytes()
	
	if err := os.WriteFile(ifoPath, data, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(bupPath, data, 0644); err != nil {
		return err
	}
	
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
	
	if buf.Len() < 2048 {
		buf.Write(make([]byte, 2048-buf.Len()))
	}
	
	data := buf.Bytes()
	
	if err := os.WriteFile(ifoPath, data, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(bupPath, data, 0644); err != nil {
		return err
	}
	
	return nil
}
