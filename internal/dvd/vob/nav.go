package vob

import (
	"encoding/binary"
	"fmt"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

// PCI (Presentation Control Information) Packet
type PCIPacket struct {
	// PCI General Information
	LVOBU_S_PTM uint32
	LVOBU_E_PTM uint32
	VOBU_UOP_CTL uint32
	
	// Highlight General Information (for menus)
	HL_GI HL_GI
}

type HL_GI struct {
	HL_Status uint16
	HL_S_PTM  uint32
	HL_E_PTM  uint32
	BTN_SL_NS uint8 // Number of buttons
	BTN_NS    uint8
}

// DSI (Data Search Information) Packet
type DSIPacket struct {
	// DSI General Information
	NV_PCK_SCR uint32
	VOBU_EA    uint32
	VOBU_1STREF_EA uint32
	VOBU_2NDREF_EA uint32
	VOBU_3RDREF_EA uint32
	
	// VOBU Search Information (SRI)
	SRI [31]uint32
}

// WriteNAV_PCK writes a full Navigation Pack (PCI + DSI) to the stream.
func (m *Muxer) WriteNAV_PCK(pci *PCIPacket, dsi *DSIPacket) error {
	logging.Debug(logging.CatDVD, "Writing Navigation Pack (NAV_PCK) at SCR %d", m.scr)
	
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
	
	// 3. PCI PES Packet (Stream ID 0xBF - Private Stream 2)
	pciData := make([]byte, 980)
	if err := m.writePESPrivate2(0xBF, pciData); err != nil {
		return fmt.Errorf("nav_pck pci: %w", err)
	}
	
	// 4. DSI PES Packet (Stream ID 0xBF - Private Stream 2)
	dsiData := make([]byte, 1018)
	if err := m.writePESPrivate2(0xBF, dsiData); err != nil {
		return fmt.Errorf("nav_pck dsi: %w", err)
	}
	
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
