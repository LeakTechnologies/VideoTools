package vob

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
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
	// Layout within hl_gi_t:
	//   +0  HL_STATUS     uint16
	//   +2  BTTN_GXCOL_NS [3]uint64  (3 × 8 bytes = 24 bytes of button group colors)
	//   +26 BTN_SL_NS     uint8  — initially-selected button (1-based)
	//   +27 BTN_NS        uint8  — total number of buttons
	//   +28 FOSL_BTTN     uint8
	//   +29 zero          uint8
	//   +30 button entries start (18 bytes each, up to 36 buttons)
	pciOffBtnSLNS  = 94 // BTN_SL_NS uint8 — initially-selected button number
	pciOffBtnNS    = 95 // BTN_NS    uint8 — total button count
	pciOffBtnTable = 98 // first button entry (18 bytes each)

	// Maximum buttons per PCI packet (DVD-Video spec limit).
	pciMaxButtons = 36
	// Each button entry in the PCI button table is 18 bytes.
	pciBtnEntrySize = 18
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

// PCIButton describes one button entry in the PCI highlight table.
// Coordinates are in DVD screen pixels (720×480 NTSC or 720×576 PAL).
// Neighbor button numbers are 1-based; 0 means "wrap to self".
// CmdNr is the 1-based index into the PGC cell-command table (0 = no command).
type PCIButton struct {
	X0, X1    int   // left/right pixel columns (inclusive)
	Y0, Y1    int   // top/bottom pixel rows (inclusive)
	Up        uint8 // button number to move to on Up
	Down      uint8 // button number to move to on Down
	Left      uint8 // button number to move to on Left
	Right     uint8 // button number to move to on Right
	CmdNr     uint8 // cell command to execute when activated (1-based)
	AutoAction bool // if true, button activates immediately on selection
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
	// Write button counts. BTN_SL_NS is the initially selected button (1-based);
	// BTN_NS is the total number of buttons in this NAV_PCK.
	btnCount := len(pci.Buttons)
	if btnCount > pciMaxButtons {
		btnCount = pciMaxButtons
	}
	pciData[pciOffBtnSLNS] = pci.HL_GI.BTN_SL_NS
	pciData[pciOffBtnNS] = uint8(btnCount)
	// FOSL_BTTN at byte 96 (hl_gi_t +28): force-select button 1 when > 0 buttons.
	if btnCount > 0 {
		pciData[96] = 1
	}

	// BTTN_GXCOL_NS at hl_gi_t +2 (PCI bytes 70-93): three 8-byte button color groups.
	// Per DVD-Video spec (Table 5.8), each 8-byte group has two separate 32-bit fields:
	//   BTTN_COLI (bytes N+0..N+3): palette indices for SPU pixel values 3,2,1,0
	//     big-endian: byte N+0 = (c3<<4)|c2, byte N+1 = (c1<<4)|c0, N+2..N+3 = 0
	//   BTTN_ALPHA (bytes N+4..N+7): alpha values for SPU pixel values 3,2,1,0
	//     big-endian: byte N+4 = (a3<<4)|a2, byte N+5 = (a1<<4)|a0, N+6..N+7 = 0
	// Group 0 (Normal): all transparent — button areas invisible at rest.
	// Group 1 (Selected): SPU pixel 1 → palette 1 (white), alpha 15 (opaque).
	// Group 2 (Activated): SPU pixel 1 → palette 2 (dark), alpha 15 (opaque).
	if btnCount > 0 {
		// Group 0 — normal: bytes 70-77 stay zero (all transparent)

		// Group 1 — selected (bytes 78-85):
		// BTTN_COLI: c1=1 (palette 1 = white), c0=0; c2=c3=0
		pciData[79] = (1 << 4) | 0 // (c1<<4)|c0 = 0x10
		// BTTN_ALPHA: a1=10 (~67% opacity) so button label remains readable; a0=0
		pciData[83] = (10 << 4) | 0 // (a1<<4)|a0 = 0xA0

		// Group 2 — activated (bytes 86-93):
		// BTTN_COLI: c1=3 (palette 3 = gray), c0=0 — visually distinct from selected
		pciData[87] = (3 << 4) | 0 // (c1<<4)|c0 = 0x30
		// BTTN_ALPHA: a1=15 (fully opaque flash), a0=0
		pciData[91] = (15 << 4) | 0 // (a1<<4)|a0 = 0xF0
	}

	// Write button position entries (18 bytes each).
	// btn_posi_t layout as decoded by libdvdnav (packing from ifo_types.h):
	//   byte 0: [btn_coln:6][x_start_hi:2]
	//     btn_coln bits 7-2 = auto_action (bit7) | button-color-set (bits 6-2, use 0)
	//     bits 1-0 = x_start[9:8]
	//   byte 1: x_start[7:0]
	//   byte 2: x_end[9:2]
	//   byte 3: [x_end[1:0]:2][y_start[9:4]:6]
	//   byte 4: [y_start[3:0]:4][y_end[9:6]:4]
	//   byte 5: [y_end[5:0]:6][reserved:2]
	//   byte 6: [up:6][down_hi:2]
	//   byte 7: [down_lo:4][left_hi:4]
	//   byte 8: [left_lo:2][right:6]
	//   byte 9: cmd_nr
	//   bytes 10-17: zero
	for i, btn := range pci.Buttons {
		if i >= pciMaxButtons {
			break
		}
		off := pciOffBtnTable + i*pciBtnEntrySize
		btnNr := uint8(i + 1) // 1-based button number

		x0 := uint16(btn.X0) & 0x3FF
		x1 := uint16(btn.X1) & 0x3FF
		y0 := uint16(btn.Y0) & 0x3FF
		y1 := uint16(btn.Y1) & 0x3FF

		autoAct := uint8(0)
		if btn.AutoAction {
			autoAct = 1
		}

		// Byte 0: auto_action in bit 7; btn_coln (color set) = 0 in bits 6-2; x_start[9:8] in bits 1-0.
		// Button number is implicit from position in table — do NOT place it in btn_coln.
		pciData[off+0] = (autoAct << 7) | uint8(x0>>8)
		pciData[off+1] = uint8(x0)
		pciData[off+2] = uint8(x1 >> 2)
		pciData[off+3] = (uint8(x1&0x3) << 6) | uint8(y0>>4)
		pciData[off+4] = (uint8(y0&0xF) << 4) | uint8(y1>>6)
		pciData[off+5] = uint8(y1&0x3F) << 2
		// Neighbours packed: up(6)|down(6)|left(6)|right(6) = 24 bits = 3 bytes
		up := btn.Up
		if up == 0 {
			up = btnNr
		}
		dn := btn.Down
		if dn == 0 {
			dn = btnNr
		}
		lf := btn.Left
		if lf == 0 {
			lf = btnNr
		}
		rt := btn.Right
		if rt == 0 {
			rt = btnNr
		}
		pciData[off+6] = (up << 2) | (dn >> 4)
		pciData[off+7] = (dn << 4) | (lf >> 2)
		pciData[off+8] = (lf << 6) | rt
		// cmd_nr in byte 9 (6 bytes total command field, rest zero)
		pciData[off+9] = btn.CmdNr
		// bytes 10-17 remain zero
	}

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

// PatchVOBPCI opens the DVD VOB at path and patches the PCI highlight/button
// table in every NAV_PCK sector found in the file.
//
// ffmpeg's dvd muxer generates NAV_PCKs with zeroed PCI highlight data, so
// button geometry and command numbers must be injected as a post-processing
// step. This function scans the file sector-by-sector (2048 bytes/sector),
// identifies NAV_PCK sectors by the PACK header (00 00 01 BA) followed by a
// PCI PES packet (00 00 01 BF 03 D4), and patches the button table in each.
//
// buttons provides the button geometry. CmdNr for each should be its 1-based
// index in the PGC cell command table (1 = first button's action, etc.).
// Navigation neighbours (Up/Down/Left/Right) default to vertical wrapping
// if left zero.
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

		// Locate the PCI PES packet: 00 00 01 BF 03 D4
		// The PCI payload is 980 bytes (0x03D4) and immediately follows the 6-byte PES header.
		pciStart := -1
		for i := 4; i < 2048-5; i++ {
			if sector[i] == 0x00 && sector[i+1] == 0x00 && sector[i+2] == 0x01 &&
				sector[i+3] == 0xBF && sector[i+4] == 0x03 && sector[i+5] == 0xD4 {
				pciStart = i + 6
				break
			}
		}

		if pciStart < 0 || pciStart+980 > 2048 {
			sectorIdx++
			continue
		}

		pci := sector[pciStart : pciStart+980]

		// ── Patch HL_GI button data ───────────────────────────────────────────
		n := len(buttons)
		if n > pciMaxButtons {
			n = pciMaxButtons
		}
		pci[pciOffBtnSLNS] = 1       // initially-selected button (1-based)
		pci[pciOffBtnNS] = uint8(n)  // total button count

		for i, btn := range buttons[:n] {
			off := pciOffBtnTable + i*pciBtnEntrySize
			btnNr := uint8(i + 1) // 1-based

			x0 := uint16(btn.X0) & 0x3FF
			x1 := uint16(btn.X1) & 0x3FF
			y0 := uint16(btn.Y0) & 0x3FF
			y1 := uint16(btn.Y1) & 0x3FF

			autoAct := uint8(0)
			if btn.AutoAction {
				autoAct = 1
			}

			// Bit-pack coordinates per libdvdread btn_posi_t layout.
			pci[off+0] = (autoAct << 7) | (btnNr << 2) | uint8(x0>>8)
			pci[off+1] = uint8(x0)
			pci[off+2] = uint8(x1 >> 2)
			pci[off+3] = (uint8(x1&0x3) << 6) | uint8(y0>>4)
			pci[off+4] = (uint8(y0&0xF) << 4) | uint8(y1>>6)
			pci[off+5] = uint8(y1&0x3F) << 2

			// Navigation neighbours (6 bits each, packed into 3 bytes).
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
			pci[off+6] = (up << 2) | (dn >> 4)
			pci[off+7] = (dn << 4) | (lf >> 2)
			pci[off+8] = (lf << 6) | rt

			// Cell command index to execute on button activation (1-based).
			pci[off+9] = btn.CmdNr
			// bytes 10–17 remain zero
		}

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
