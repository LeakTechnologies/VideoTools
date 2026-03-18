package vob

import (
	"encoding/binary"
	"fmt"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
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
	// hl_gi_t starts at offset 68 (32-byte pci_gi + 36-byte nsml_agli).
	// btn_ns (number of buttons) is at +26 within hl_gi_t.
	pciOffBtnNS = 94 // btn_ns uint8
)

// Byte offsets within the 1018-byte DSI payload (from DVD-Video spec, dsi_gi_t).
const (
	dsiOffNVPCKSCR  = 0  // nv_pck_scr    uint32 — SCR of NAV_PCK (90kHz)
	dsiOffNVPCKLBN  = 4  // nv_pck_lbn    uint32 — LBN of NAV_PCK
	dsiOffVOBUEA    = 8  // vobu_ea       uint32 — last sector offset of VOBU from NAV_PCK
	dsiOff1STREFEA  = 12 // vobu_1stref_ea uint32
	dsiOff2NDREFEA  = 16 // vobu_2ndref_ea uint32
	dsiOff3RDREFEA  = 20 // vobu_3rdref_ea uint32
	dsiOffVOBUVOBIDN = 24 // vobu_vob_idn  uint16
	dsiOffVOBUCIDN  = 27 // vobu_c_idn    uint8

	// VOBU_SRI starts after DSI_GI (36 bytes) + SML_PBI (128 bytes) + SML_AGLI (32 bytes).
	dsiOffVOBUSRI   = 196
	dsiVOBUSRICount = 30 // 30 × uint32 = 120 bytes
)

// PCIPacket carries the caller-supplied fields for the PCI section of a NAV_PCK.
// Fields not set here are either zeroed or auto-filled from the muxer state.
type PCIPacket struct {
	// PCI General Information
	VOBU_UOP_CTL uint32 // user operation control (0 = all ops permitted)
	LVOBU_S_PTM  uint32 // VOBU start PTM in 90kHz ticks; 0 = auto from muxer SCR
	LVOBU_E_PTM  uint32 // VOBU end PTM in 90kHz ticks

	// Highlight General Information (for menus)
	HL_GI HL_GI
}

// HL_GI carries highlight/button data for menu NAV_PCKs.
type HL_GI struct {
	HL_Status uint16
	HL_S_PTM  uint32
	HL_E_PTM  uint32
	BTN_SL_NS uint8 // number of buttons (0 for non-menu VOBUs)
	BTN_NS    uint8
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

	// ── Build PCI data (980 bytes) ────────────────────────────────────────────
	pciData := make([]byte, 980)
	binary.BigEndian.PutUint32(pciData[pciOffNVPCKLBN:], m.currentSector)
	binary.BigEndian.PutUint32(pciData[pciOffVOBUUOPCTL:], pci.VOBU_UOP_CTL)

	sPTM := pci.LVOBU_S_PTM
	if sPTM == 0 {
		sPTM = scr90 // auto from current SCR
	}
	binary.BigEndian.PutUint32(pciData[pciOffVOBUSPTM:], sPTM)
	binary.BigEndian.PutUint32(pciData[pciOffVOBUEPTM:], pci.LVOBU_E_PTM)
	binary.BigEndian.PutUint32(pciData[pciOffVOBUSEEPTM:], pci.LVOBU_E_PTM) // still = end
	pciData[pciOffBtnNS] = pci.HL_GI.BTN_SL_NS

	// ── Build DSI data (1018 bytes) ───────────────────────────────────────────
	dsiData := make([]byte, 1018)
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

	// 3. PCI PES Packet (Private Stream 2, 0xBF, 980 bytes payload)
	if err := m.writePESPrivate2(0xBF, pciData); err != nil {
		return fmt.Errorf("nav_pck pci: %w", err)
	}

	// 4. DSI PES Packet (Private Stream 2, 0xBF, 1018 bytes payload)
	if err := m.writePESPrivate2(0xBF, dsiData); err != nil {
		return fmt.Errorf("nav_pck dsi: %w", err)
	}

	m.currentSector++
	return nil
}

func (m *Muxer) writePESPrivate2(streamID uint8, payload []byte) error {
	var header [6]byte
	header[0] = 0x00
	header[1] = 0x00
	header[2] = 0x01
	header[3] = streamID
	binary.BigEndian.PutUint16(header[4:6], uint16(len(payload)))

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
