package vob

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/LeakTechnologies/VideoTools/internal/logging"
)

// SRIEndOfCell is the VOBU_SRI sentinel meaning "no sector at this time offset".
// Hardware players skip seeking when they encounter this value.
const SRIEndOfCell = uint32(0x3FFFFFFF)

// Byte offsets within the 980-byte PCI payload (from DVD-Video spec, pci_gi_t).
const (
	pciOffNVPCKLBN   = 0  // nv_pck_lbn   uint32 — LBN of this NAV_PCK
	pciOffVOBUUOPCTL = 8  // vobu_uop_ctl uint32 — user operation control
	pciOffVOBUSPTM   = 12 // vobu_s_ptm   uint32 — VOBU start PTM (90kHz)
	pciOffVOBUEPTM   = 16 // vobu_e_ptm   uint32 — VOBU end PTM (90kHz)
	pciOffVOBUSEEPTM = 20 // vobu_se_e_ptm uint32 — still end PTM (= e_ptm for non-still)

	// hli_t layout per libdvdread ifo_types.h. pci_gi_t is 60 bytes
	// (nv_pck_lbn 4, vobu_cat 2, zero 2, uop_ctl 4, s/e/se_e ptm 12,
	// e_eltm 4, vobu_isrc 32); nsml_agli_t is 36 bytes (9 angles × 4).
	// hli_t therefore starts at PCI table offset 96.
	//
	// hl_gi_t (22 bytes, offsets 96–117):
	//   96  hli_ss        uint16 — 0=no HLI, 1=HLI new for this VOBU
	//   98  hli_s_ptm     uint32
	//  102  hli_e_ptm     uint32
	//  106  btn_se_e_ptm  uint32 — button select end PTM
	//  110  [zero:2][btngr_ns:2][zero:1][btngr1_dsp_ty:3]
	//  111  [zero:1][btngr2_dsp_ty:3][zero:1][btngr3_dsp_ty:3]
	//  112  btn_ofn       uint8 — button offset number (numeric select base)
	//  113  btn_ns        uint8 — number of buttons
	//  114  nsl_btn_ns    uint8 — numerically-selectable buttons
	//  115  zero          uint8
	//  116  fosl_btnn     uint8 — forced-select button on entry
	//  117  foac_btnn     uint8 — forced-action button
	// btn_colit_t (24 bytes, 118–141): 3 groups × { SL_COLI u32, AC_COLI u32 }
	// btnit (142+): 36 × 18-byte btni_t entries
	hliOffSS       = 96
	hliOffSPTM     = 98
	hliOffEPTM     = 102
	hliOffBtnSEPTM = 106
	hliOffBtnGr    = 110
	hliOffBtnOFN   = 112
	hliOffBtnNS    = 113
	hliOffNSLBtnNS = 114
	hliOffFOSLBtn  = 116
	hliOffFOACBtn  = 117
	hliOffColit    = 118
	pciOffBtnTable = 142 // first btni_t entry

	// Maximum buttons per PCI packet (DVD-Video spec limit).
	pciMaxButtons = 36
	// Each btni_t entry is 18 bytes (10 geometry/nav + 8-byte inline VM command).
	pciBtnEntrySize = 18
)

// Byte offsets within the 1018-byte DSI payload (from DVD-Video spec, dsi_gi_t).
const (
	dsiOffNVPCKSCR   = 0  // nv_pck_scr    uint32 — SCR of NAV_PCK (90kHz)
	dsiOffNVPCKLBN   = 4  // nv_pck_lbn    uint32 — LBN of NAV_PCK
	dsiOffVOBUEA     = 8  // vobu_ea       uint32 — last sector offset of VOBU from NAV_PCK
	dsiOff1STREFEA   = 12 // vobu_1stref_ea uint32
	dsiOff2NDREFEA   = 16 // vobu_2ndref_ea uint32
	dsiOff3RDREFEA   = 20 // vobu_3rdref_ea uint32
	dsiOffVOBUVOBIDN = 24 // vobu_vob_idn  uint16
	dsiOffVOBUCIDN   = 27 // vobu_c_idn    uint8

	// VOBU_SRI starts after DSI_GI (36 bytes) + SML_PBI (128 bytes) + SML_AGLI (32 bytes).
	dsiOffVOBUSRI   = 196
	dsiVOBUSRICount = 30 // 30 × uint32 = 120 bytes
)

// PCIButton describes one button entry in the PCI highlight table.
// Coordinates are in DVD screen pixels (720×480 NTSC or 720×576 PAL).
// Neighbor button numbers are 1-based; 0 means "wrap to self".
// Cmd is the 8-byte DVD VM instruction executed on activation, written
// inline into the btni_t entry (there is no indirection into the PGC cell
// command table — audit finding A2).
type PCIButton struct {
	X0, X1     int     // left/right pixel columns (inclusive)
	Y0, Y1     int     // top/bottom pixel rows (inclusive)
	Up         uint8   // button number to move to on Up
	Down       uint8   // button number to move to on Down
	Left       uint8   // button number to move to on Left
	Right      uint8   // button number to move to on Right
	Cmd        [8]byte // inline VM command executed on activation
	AutoAction bool    // if true, button activates immediately on selection
}

// PCIPacket carries the caller-supplied fields for the PCI section of a NAV_PCK.
// Fields not set here are either zeroed or auto-filled from the muxer state.
type PCIPacket struct {
	// PCI General Information
	VOBU_UOP_CTL uint32 // user operation control (0 = all ops permitted)
	LVOBU_S_PTM  uint32 // VOBU start PTM in 90kHz ticks; 0 = auto from muxer SCR
	LVOBU_E_PTM  uint32 // VOBU end PTM in 90kHz ticks

	// Highlight General Information (for menus)
	HL_GI   HL_GI
	Buttons []PCIButton // up to pciMaxButtons (36) entries
}

// HL_GI carries highlight/button data for menu NAV_PCKs.
type HL_GI struct {
	HL_Status uint16
	HL_S_PTM  uint32
	HL_E_PTM  uint32
	BTN_SL_NS uint8 // initially-selected button (1-based; 0 = none)
	BTN_NS    uint8 // total number of buttons (deprecated: use len(PCIPacket.Buttons))
}

// DSIPacket carries the caller-supplied fields for the DSI section of a NAV_PCK.
// NV_PCK_SCR and NV_PCK_LBN are auto-filled from muxer state.
// VOBU_SRI is automatically set to SRIEndOfCell (no seek map).
type DSIPacket struct {
	// DSI General Information
	VOBU_EA        uint32 // sector offset of last sector of VOBU from NAV_PCK (0 = 1-sector VOBU)
	VOBU_1STREF_EA uint32 // sector offset of end of first reference picture
	VOBU_2NDREF_EA uint32
	VOBU_3RDREF_EA uint32
	VOBU_VOB_IDN   uint16 // VOB ID (1-based; 0 = not specified)
	VOBU_C_IDN     uint8  // Cell ID (1-based; 0 = not specified)
}

// WriteNAV_PCK writes a full Navigation Pack (PCI + DSI) as one 2048-byte sector.
// It auto-fills nv_pck_lbn from the current sector and nv_pck_scr / vobu_s_ptm
// from the current SCR. All VOBU_SRI entries are set to SRIEndOfCell.
func (m *Muxer) WriteNAV_PCK(pci *PCIPacket, dsi *DSIPacket) error {
	logging.Debug(logging.CatDVD, "Writing NAV_PCK at sector %d (SCR %d)", m.currentSector, m.scr)

	// Record sector address for IFO VOBU_ADMAP
	m.NAVPCKSectors = append(m.NAVPCKSectors, m.currentSector)

	scr90 := uint32(m.scr / 300) // SCR base in 90kHz

	// ── Build PCI data (979 bytes; the 0x00 substream ID byte that precedes
	// it on disc is written by writePESPrivate2) ─────────────────────────────
	pciData := make([]byte, 979)
	binary.BigEndian.PutUint32(pciData[pciOffNVPCKLBN:], m.currentSector)
	binary.BigEndian.PutUint32(pciData[pciOffVOBUUOPCTL:], pci.VOBU_UOP_CTL)

	sPTM := pci.LVOBU_S_PTM
	if sPTM == 0 {
		sPTM = scr90 // auto from current SCR
	}
	binary.BigEndian.PutUint32(pciData[pciOffVOBUSPTM:], sPTM)
	binary.BigEndian.PutUint32(pciData[pciOffVOBUEPTM:], pci.LVOBU_E_PTM)
	binary.BigEndian.PutUint32(pciData[pciOffVOBUSEEPTM:], pci.LVOBU_E_PTM) // still = end

	// Highlight information (menus). Shared with PatchVOBPCI.
	serializeHLI(pciData, pci.Buttons, pci.HL_GI.BTN_SL_NS, sPTM, pci.LVOBU_E_PTM)

	// ── Build DSI data (1017 bytes; 0x01 substream ID prepended on write) ────
	dsiData := make([]byte, 1017)
	binary.BigEndian.PutUint32(dsiData[dsiOffNVPCKSCR:], scr90)
	binary.BigEndian.PutUint32(dsiData[dsiOffNVPCKLBN:], m.currentSector)
	binary.BigEndian.PutUint32(dsiData[dsiOffVOBUEA:], dsi.VOBU_EA)
	binary.BigEndian.PutUint32(dsiData[dsiOff1STREFEA:], dsi.VOBU_1STREF_EA)
	binary.BigEndian.PutUint32(dsiData[dsiOff2NDREFEA:], dsi.VOBU_2NDREF_EA)
	binary.BigEndian.PutUint32(dsiData[dsiOff3RDREFEA:], dsi.VOBU_3RDREF_EA)
	binary.BigEndian.PutUint16(dsiData[dsiOffVOBUVOBIDN:], dsi.VOBU_VOB_IDN)
	dsiData[dsiOffVOBUCIDN] = dsi.VOBU_C_IDN

	// VOBU_SRI: end-of-cell sentinel — tells hardware there is no seek table
	for i := 0; i < dsiVOBUSRICount; i++ {
		binary.BigEndian.PutUint32(dsiData[dsiOffVOBUSRI+i*4:], SRIEndOfCell)
	}

	// ── Write sectors ─────────────────────────────────────────────────────────

	// 1. Pack Header (14 bytes)
	if err := m.WritePackHeader(m.scr); err != nil {
		return fmt.Errorf("nav_pck pack header: %w", err)
	}

	// 2. System Header (24 bytes)
	var sys [24]byte
	binary.BigEndian.PutUint32(sys[0:4], SystemHeaderCode)
	binary.BigEndian.PutUint16(sys[4:6], uint16(len(sys)-6))
	sys[6] = 0x80 | 0x01
	if _, err := m.w.Write(sys[:]); err != nil {
		logging.Error(logging.CatDVD, "Failed to write system header: %v", err)
		return fmt.Errorf("nav_pck system header: %w", err)
	}

	// 3. PCI PES Packet (Private Stream 2, 0xBF; payload = substream 0x00 + 979 bytes)
	if err := m.writePESPrivate2(0xBF, 0x00, pciData); err != nil {
		return fmt.Errorf("nav_pck pci: %w", err)
	}

	// 4. DSI PES Packet (Private Stream 2, 0xBF; payload = substream 0x01 + 1017 bytes)
	if err := m.writePESPrivate2(0xBF, 0x01, dsiData); err != nil {
		return fmt.Errorf("nav_pck dsi: %w", err)
	}

	m.currentSector++
	return nil
}

// serializeHLI writes the hli_t highlight structure (hl_gi + btn_colit +
// btnit) into a 979-byte PCI table buffer at the spec offsets. No-op when
// buttons is empty. selBtn is the initially-selected button (1-based; 0 → 1).
// sPTM/ePTM bound the highlight validity window; ePTM 0 means "until still
// end" and is written as 0xFFFFFFFF.
func serializeHLI(pci []byte, buttons []PCIButton, selBtn uint8, sPTM, ePTM uint32) {
	btnCount := len(buttons)
	if btnCount == 0 {
		return
	}
	if btnCount > pciMaxButtons {
		btnCount = pciMaxButtons
	}
	if selBtn == 0 {
		selBtn = 1
	}
	end := ePTM
	if end == 0 {
		end = 0xFFFFFFFF // highlight valid until the still ends
	}

	// hl_gi
	binary.BigEndian.PutUint16(pci[hliOffSS:], 0x0001) // HLI new for this VOBU
	binary.BigEndian.PutUint32(pci[hliOffSPTM:], sPTM)
	binary.BigEndian.PutUint32(pci[hliOffEPTM:], end)
	binary.BigEndian.PutUint32(pci[hliOffBtnSEPTM:], end)
	// One button group, display type 0 (4:3): [zero:2][btngr_ns=1:2][zero:1][dsp_ty=0:3]
	pci[hliOffBtnGr] = 0x10
	pci[hliOffBtnGr+1] = 0x00
	pci[hliOffBtnOFN] = 0
	pci[hliOffBtnNS] = uint8(btnCount)
	pci[hliOffNSLBtnNS] = uint8(btnCount)
	pci[hliOffFOSLBtn] = selBtn
	pci[hliOffFOACBtn] = 0

	// btn_colit: 3 color groups × { SL_COLI, AC_COLI }. Each COLI is
	// [c3 c2 c1 c0 | a3 a2 a1 a0] as 8 big-endian nibbles (color palette
	// indices for SPU pixel values 3..0, then their alphas).
	// Buttons reference group 1 via btn_coln=1.
	// Selected: SPU pixel 1 → PGC palette 1 (white) at alpha 10 (~67%).
	binary.BigEndian.PutUint32(pci[hliOffColit:], 0x001000A0)
	// Activated: SPU pixel 1 → PGC palette 3 (gray), fully opaque flash.
	binary.BigEndian.PutUint32(pci[hliOffColit+4:], 0x003000F0)
	// Groups 2 and 3 unused (btngr_ns = 1).

	// btni_t entries (18 bytes): bitfields per libdvdread ifo_types.h —
	//   b0 = btn_coln(2) | x_start[9:4]     b1 = x_start[3:0] | zero(2) | x_end[9:8]
	//   b2 = x_end[7:0]                      b3 = auto_action(2) | y_start[9:4]
	//   b4 = y_start[3:0] | zero(2) | y_end[9:8]   b5 = y_end[7:0]
	//   b6..b9 = zero(2)+up(6), zero(2)+down(6), zero(2)+left(6), zero(2)+right(6)
	//   b10..b17 = inline 8-byte VM command
	for i, btn := range buttons {
		if i >= pciMaxButtons {
			break
		}
		off := pciOffBtnTable + i*pciBtnEntrySize
		btnNr := uint8(i + 1)

		x0 := uint16(btn.X0) & 0x3FF
		x1 := uint16(btn.X1) & 0x3FF
		y0 := uint16(btn.Y0) & 0x3FF
		y1 := uint16(btn.Y1) & 0x3FF

		autoAct := uint8(0)
		if btn.AutoAction {
			autoAct = 1
		}

		const btnColn = 1 // use color group 1 from btn_colit
		pci[off+0] = (btnColn << 6) | uint8(x0>>4)
		pci[off+1] = (uint8(x0&0xF) << 4) | uint8(x1>>8)
		pci[off+2] = uint8(x1)
		pci[off+3] = (autoAct << 6) | uint8(y0>>4)
		pci[off+4] = (uint8(y0&0xF) << 4) | uint8(y1>>8)
		pci[off+5] = uint8(y1)

		up, dn, lf, rt := btn.Up, btn.Down, btn.Left, btn.Right
		if up == 0 {
			up = btnNr
		}
		if dn == 0 {
			dn = btnNr
		}
		if lf == 0 {
			lf = btnNr
		}
		if rt == 0 {
			rt = btnNr
		}
		pci[off+6] = up & 0x3F
		pci[off+7] = dn & 0x3F
		pci[off+8] = lf & 0x3F
		pci[off+9] = rt & 0x3F

		copy(pci[off+10:off+18], btn.Cmd[:])
	}
}

// writePESPrivate2 writes a private_stream_2 PES packet whose first payload
// byte is the DVD substream ID (0x00 = PCI, 0x01 = DSI). The PES packet
// length includes the substream ID byte, matching real discs (0x03D4 for
// PCI = 1 + 979, 0x03FA for DSI = 1 + 1017). Demuxers identify PCI/DSI by
// this byte — omitting it made the DSI unrecognizable and shifted every PCI
// field by one byte (audit finding A3).
func (m *Muxer) writePESPrivate2(streamID, subStreamID uint8, payload []byte) error {
	var header [7]byte
	header[0] = 0x00
	header[1] = 0x00
	header[2] = 0x01
	header[3] = streamID
	binary.BigEndian.PutUint16(header[4:6], uint16(len(payload)+1))
	header[6] = subStreamID

	if _, err := m.w.Write(header[:]); err != nil {
		logging.Error(logging.CatDVD, "Failed to write Private2 header (0x%X): %v", streamID, err)
		return fmt.Errorf("private2 header: %w", err)
	}

	if _, err := m.w.Write(payload); err != nil {
		logging.Error(logging.CatDVD, "Failed to write Private2 payload: %v", err)
		return fmt.Errorf("private2 payload: %w", err)
	}
	return nil
}

// PatchVOBPCI opens the DVD VOB at path and patches the PCI highlight/button
// table in every NAV_PCK sector found in the file.
//
// ffmpeg's dvd muxer generates NAV_PCKs with zeroed PCI highlight data, so
// button geometry and command numbers must be injected as a post-processing
// step. This function scans the file sector-by-sector (2048 bytes/sector),
// identifies NAV_PCK sectors by the PACK header (00 00 01 BA) followed by a
// PCI PES packet (00 00 01 BF 03 D4), and patches the button table in each.
//
// buttons provides the geometry, neighbours, and the inline 8-byte VM
// command executed on activation (PCIButton.Cmd). Neighbours left zero
// default to wrapping to the button itself.
func PatchVOBPCI(path string, buttons []PCIButton) error {
	if len(buttons) == 0 {
		return nil
	}

	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("PatchVOBPCI open: %w", err)
	}
	defer f.Close()

	sector := make([]byte, 2048)
	var sectorIdx int64
	patched := 0

	for {
		_, err := io.ReadFull(f, sector)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return fmt.Errorf("PatchVOBPCI read sector %d: %w", sectorIdx, err)
		}

		// Only inspect sectors that begin with a PS PACK header.
		if sector[0] != 0x00 || sector[1] != 0x00 || sector[2] != 0x01 || sector[3] != 0xBA {
			sectorIdx++
			continue
		}

		// Locate the PCI PES packet: 00 00 01 BF 03 D4 00 — the 980-byte PES
		// payload starts with substream ID 0x00; PCI table data follows it.
		pciStart := -1
		for i := 4; i < 2048-6; i++ {
			if sector[i] == 0x00 && sector[i+1] == 0x00 && sector[i+2] == 0x01 &&
				sector[i+3] == 0xBF && sector[i+4] == 0x03 && sector[i+5] == 0xD4 &&
				sector[i+6] == 0x00 {
				pciStart = i + 7
				break
			}
		}

		if pciStart < 0 || pciStart+979 > 2048 {
			sectorIdx++
			continue
		}

		pci := sector[pciStart : pciStart+979]

		// Patch the highlight structure using the existing VOBU PTM window.
		sPTM := binary.BigEndian.Uint32(pci[pciOffVOBUSPTM:])
		ePTM := binary.BigEndian.Uint32(pci[pciOffVOBUEPTM:])
		serializeHLI(pci, buttons, 1, sPTM, ePTM)

		// Seek back to this sector's start and write the patched data.
		if _, err := f.Seek(sectorIdx*2048, io.SeekStart); err != nil {
			return fmt.Errorf("PatchVOBPCI seek: %w", err)
		}
		if _, err := f.Write(sector); err != nil {
			return fmt.Errorf("PatchVOBPCI write: %w", err)
		}
		patched++
		sectorIdx++
	}

	logging.Info(logging.CatDVD, "PatchVOBPCI: patched %d NAV_PCK sectors in %s with %d buttons", patched, path, len(buttons))
	return nil
}
