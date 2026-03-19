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
// All table pointers may be nil; when non-nil they are written in spec order:
//   - pttsrpt → VTS_PTT_SRPT at sector 1 (chapter table)
//   - pgc     → VTS_PGCITI (PGC navigation)
//   - tmapt   → VTS_TMAPT (seek-bar time map)
//   - cadt    → VTS_C_ADT (cell address table; used by hardware for cell seek)
//   - admap   → VOBU_ADMAP (VOBU address map for trick play)
func (b *Builder) GenerateVTS_IFO(vtsNumber int, mat *VTS_MAT, pgc *ProgramChain, tmapt *VTS_TMAPT, admap *VOBU_ADMAP, pttsrpt *VTS_PTT_SRPT) error {
	filename := fmt.Sprintf("VTS_%02d_0.IFO", vtsNumber)
	ifoPath := filepath.Join(b.outputDir, filename)
	bupPath := filepath.Join(b.outputDir, fmt.Sprintf("VTS_%02d_0.BUP", vtsNumber))

	logging.Info(logging.CatDVD, "Generating IFO/BUP for VTS %d", vtsNumber)

	// Sector layout:
	//   Sector 0:   VTS_MAT
	//   Sector 1:   VTS_PTT_SRPT (if pttsrpt != nil)
	//   Next:       VTS_PGCITI   (if pgc != nil)
	//   Next:       VTS_TMAPT    (if tmapt != nil)
	//   Next:       VOBU_ADMAP   (if admap != nil)
	nextSector := uint32(1)

	var pttsrptData []byte
	if pttsrpt != nil {
		var err error
		pttsrptData, err = WriteVTS_PTT_SRPT(pttsrpt)
		if err != nil {
			return fmt.Errorf("serialize vts_ptt_srpt: %w", err)
		}
		mat.VTS_PTT_SRPT_Offset = nextSector
		nextSector += uint32(len(pttsrptData) / 2048)
	}

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

	// VTS_C_ADT — build from PGC if available; gives hardware players the
	// cell-to-sector mapping needed for validated cell navigation.
	var cadtData []byte
	if pgc != nil && len(pgc.CellPlayback) > 0 {
		cadt := BuildVTS_C_ADT(pgc)
		if cadt != nil {
			var err error
			cadtData, err = WriteVTS_C_ADT(cadt)
			if err != nil {
				return fmt.Errorf("serialize vts_c_adt: %w", err)
			}
			mat.VTS_C_ADT_Offset = nextSector
			nextSector += uint32(len(cadtData) / 2048)
		}
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

	if pttsrptData != nil {
		buf.Write(pttsrptData)
	}
	if pgcitiData != nil {
		buf.Write(pgcitiData)
	}
	if tmaptData != nil {
		buf.Write(tmaptData)
	}
	if cadtData != nil {
		buf.Write(cadtData)
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
// All table parameters may be nil; when non-nil they are written in spec order:
//   - srpt    → TT_SRPT at sector 1 (title search pointer table)
//   - menuPGC → VMG_PGCITI (menu program chain)
//   - vtsAtrt → VMG_VTS_ATRT (VTS attribute table; cross-validates stream attrs)
func (b *Builder) GenerateVMG_IFO(mat *VMG_MAT, srpt *TT_SRPT, menuPGC *ProgramChain, vtsAtrt *VTS_ATRT) error {
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

	// VTS_ATRT — sector-padded buffer built via WriteVTS_ATRT.
	var atrtBuf bytes.Buffer
	if vtsAtrt != nil {
		if err := WriteVTS_ATRT(&atrtBuf, vtsAtrt); err != nil {
			return fmt.Errorf("serialize vts_atrt: %w", err)
		}
		if rem := atrtBuf.Len() % 2048; rem != 0 {
			atrtBuf.Write(make([]byte, 2048-rem))
		}
		mat.VMG_VTS_ATRT_Offset = nextSector
		nextSector += uint32(atrtBuf.Len() / 2048)
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
	if atrtBuf.Len() > 0 {
		buf.Write(atrtBuf.Bytes())
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
