package ifo

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LeakTechnologies/VideoTools/internal/logging"
)

// Builder coordinates the construction of IFO and BUP files.
type Builder struct {
	outputDir string

	// MenuVOBSectors is the size of VIDEO_TS.VOB in 2048-byte sectors.
	// Set by the caller before GenerateVMG_IFO when a menu VOB exists so the
	// builder can compute VMGM_VOBS_Sector (VOB starts right after the IFO)
	// and VMG_Last_Sector (IFO + VOB + BUP). Zero when no menu VOB.
	MenuVOBSectors uint32

	// MenuNAVSectors holds the VOBS-relative sector offset of every NAV_PCK in
	// VIDEO_TS.VOB, used to build the VMGM_VOBU_ADMAP. Zero-length omits the
	// table. Populate by scanning the built VOB (vob.ScanVOBForNAVPCKs).
	MenuNAVSectors []uint32
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
	mat.VTSI_Last_Byte = nextSector*2048 - 1

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
//   - srpt      → TT_SRPT at sector 1 (title search pointer table)
//   - menuPGCs  → VMG_PGCITI (menu program chains; multiple for multi-page menus)
//   - vtsAtrt   → VMG_VTS_ATRT (VTS attribute table; cross-validates stream attrs)
func (b *Builder) GenerateVMG_IFO(mat *VMG_MAT, srpt *TT_SRPT, menuPGCs []*ProgramChain, vtsAtrt *VTS_ATRT) error {
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
	if len(menuPGCs) > 0 {
		var firstPGCInTable int
		var err error
		pgcitiData, firstPGCInTable, err = WriteVMGM_PGCI_UT(menuPGCs)
		if err != nil {
			return fmt.Errorf("serialize vmg_pgciti: %w", err)
		}
		mat.VMG_PGCITI_Offset = nextSector
		// first_play_pgc is the absolute byte offset within the IFO file of
		// the main menu PGC. DVD players jump here on disc insertion to show
		// the menu rather than skipping straight to title 1.
		mat.VMG_FirstPlayPGC = mat.VMG_PGCITI_Offset*2048 + uint32(firstPGCInTable)
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

	// VMGM_C_ADT — menu cell address table (audit finding A8). Built from the
	// menu PGC cells; emitted only once cell sectors have been assigned.
	var cadtData []byte
	if cadt := BuildMenuCADT(menuPGCs); cadt != nil {
		var err error
		cadtData, err = WriteVTS_C_ADT(cadt)
		if err != nil {
			return fmt.Errorf("serialize vmgm_c_adt: %w", err)
		}
		mat.VMG_M_C_ADT_Offset = nextSector
		nextSector += uint32(len(cadtData) / 2048)
	}

	// VMGM_VOBU_ADMAP — menu VOBU address map (audit finding A8). Sector
	// offsets of every NAV_PCK in VIDEO_TS.VOB, supplied by the caller.
	var mAdmapBuf bytes.Buffer
	if admap := BuildVOBU_ADMAP(b.MenuNAVSectors); admap != nil {
		if err := WriteVOBU_ADMAP(&mAdmapBuf, admap); err != nil {
			return fmt.Errorf("serialize vmgm_vobu_admap: %w", err)
		}
		if rem := mAdmapBuf.Len() % 2048; rem != 0 {
			mAdmapBuf.Write(make([]byte, 2048-rem))
		}
		mat.VMG_M_VOBU_ADMAP_Offset = nextSector
		nextSector += uint32(mAdmapBuf.Len() / 2048)
	}

	// Sector accounting (audit findings A6/A7). All offsets in VMG_MAT are
	// relative to the start of the VMG (= start of VIDEO_TS.IFO):
	//   VMGI (this IFO)      : sectors 0 .. ifoSectors-1
	//   VMGM_VOBS (menu VOB) : ifoSectors .. ifoSectors+MenuVOBSectors-1
	//   VMGI BUP             : the remainder
	ifoSectors := nextSector
	// 0x1C — last sector of the VMGI (this IFO), not of the BUP.
	mat.VMG_BUP_Last_Sector = ifoSectors - 1
	if len(menuPGCs) > 0 && b.MenuVOBSectors > 0 {
		mat.VMGM_VOBS_Sector = ifoSectors
	}
	// 0x0C — last sector of the whole VMG set: IFO + menu VOB + BUP.
	mat.VMG_Last_Sector = ifoSectors*2 + b.MenuVOBSectors - 1

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
	if cadtData != nil {
		buf.Write(cadtData)
	}
	if mAdmapBuf.Len() > 0 {
		buf.Write(mAdmapBuf.Bytes())
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
