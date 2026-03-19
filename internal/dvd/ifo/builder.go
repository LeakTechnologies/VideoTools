package ifo

import (
	"bytes"
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
// pgc, tmapt, and admap may each be nil. When provided:
//   - pgc  → PGCITI written at sector 1, offsets updated
//   - tmapt → TMAPT written after PGCITI, VTS_TMAPTI_Offset updated
//   - admap → VOBU_ADMAP written last, VTS_VOBU_ADMAP_Offset updated
func (b *Builder) GenerateVTS_IFO(vtsNumber int, mat *VTS_MAT, pgc *ProgramChain, tmapt *VTS_TMAPT, admap *VOBU_ADMAP) error {
	filename := fmt.Sprintf("VTS_%02d_0.IFO", vtsNumber)
	ifoPath := filepath.Join(b.outputDir, filename)
	bupPath := filepath.Join(b.outputDir, fmt.Sprintf("VTS_%02d_0.BUP", vtsNumber))

	logging.Info(logging.CatDVD, "Generating IFO/BUP for VTS %d", vtsNumber)

	// Sector layout:
	//   Sector 0:   VTS_MAT
	//   Sector 1:   VTS_PGCITI  (if pgc != nil)
	//   Next:       VTS_TMAPT   (if tmapt != nil)
	//   Next:       VOBU_ADMAP  (if admap != nil)
	nextSector := uint32(1)

	var pgcitiData []byte
	if pgc != nil {
		var err error
		pgcitiData, err = WritePGCITI(pgc)
		if err != nil {
			return fmt.Errorf("serialize pgciti: %w", err)
		}
		mat.VTS_PGCITI_Offset = nextSector
		nextSector += uint32(len(pgcitiData) / 2048)
	}

	var tmaptData []byte
	if tmapt != nil {
		var err error
		tmaptData, err = WriteTMAPT(tmapt)
		if err != nil {
			return fmt.Errorf("serialize tmapt: %w", err)
		}
		mat.VTS_TMAPTI_Offset = nextSector
		nextSector += uint32(len(tmaptData) / 2048)
	}

	if admap != nil {
		mat.VTS_VOBU_ADMAP_Offset = nextSector
		admapLen := 4 + len(admap.Sectors)*4
		admapSectors := uint32((admapLen + 2047) / 2048)
		nextSector += admapSectors
	}

	mat.VTS_Last_Sector = nextSector - 1
	mat.VTS_MAT_Last_Sector = 0

	var buf bytes.Buffer

	// Sector 0: MAT — use spec-correct byte-offset serializer, not binary.Write
	buf.Write(SerializeVTSMAT(mat))

	if pgcitiData != nil {
		buf.Write(pgcitiData)
	}
	if tmaptData != nil {
		buf.Write(tmaptData)
	}

	if admap != nil {
		if err := WriteVOBU_ADMAP(&buf, admap); err != nil {
			return fmt.Errorf("serialize vobu_admap: %w", err)
		}
		if rem := buf.Len() % 2048; rem != 0 {
			buf.Write(make([]byte, 2048-rem))
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
// srpt may be nil; menuPGC may be nil. When provided, srpt is written at sector 1
// and menuPGC is written as VMG_PGCITI at the next available sector.
func (b *Builder) GenerateVMG_IFO(mat *VMG_MAT, srpt *TT_SRPT, menuPGC *ProgramChain) error {
	ifoPath := filepath.Join(b.outputDir, "VIDEO_TS.IFO")
	bupPath := filepath.Join(b.outputDir, "VIDEO_TS.BUP")

	logging.Info(logging.CatDVD, "Generating main VMGI IFO/BUP")

	nextSector := uint32(1) // sector after MAT

	var srptData []byte
	if srpt != nil {
		var err error
		srptData, err = WriteTT_SRPT(srpt)
		if err != nil {
			return fmt.Errorf("serialize tt_srpt: %w", err)
		}
		mat.TT_SRPT_Offset = nextSector
		nextSector += uint32(len(srptData) / 2048)
	}

	var pgcitiData []byte
	if menuPGC != nil {
		var err error
		pgcitiData, err = WritePGCITI(menuPGC)
		if err != nil {
			return fmt.Errorf("serialize vmg_pgciti: %w", err)
		}
		mat.VMG_PGCITI_Offset = nextSector
		nextSector += uint32(len(pgcitiData) / 2048)
	}

	mat.VMG_Last_Sector = nextSector - 1

	var buf bytes.Buffer
	// Sector 0: MAT — use spec-correct byte-offset serializer, not binary.Write
	buf.Write(SerializeVMGMAT(mat))

	if srptData != nil {
		buf.Write(srptData)
	}
	if pgcitiData != nil {
		buf.Write(pgcitiData)
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
