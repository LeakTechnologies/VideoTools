package vob

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestWriteNAV_PCK_SectorSize verifies WriteNAV_PCK produces exactly 2048 bytes.
func TestWriteNAV_PCK_SectorSize(t *testing.T) {
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	pci := &PCIPacket{}
	dsi := &DSIPacket{}
	if err := m.WriteNAV_PCK(pci, dsi); err != nil {
		t.Fatalf("WriteNAV_PCK failed: %v", err)
	}
	if buf.Len() != PackSize {
		t.Errorf("NAV_PCK size = %d bytes, want %d", buf.Len(), PackSize)
	}
}

// TestWriteNAV_PCK_PackStartCode verifies the pack starts with 0x000001BA.
func TestWriteNAV_PCK_PackStartCode(t *testing.T) {
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	if err := m.WriteNAV_PCK(&PCIPacket{}, &DSIPacket{}); err != nil {
		t.Fatalf("WriteNAV_PCK failed: %v", err)
	}
	data := buf.Bytes()
	startCode := binary.BigEndian.Uint32(data[0:4])
	if startCode != PackStartCode {
		t.Errorf("pack start code = 0x%08X, want 0x%08X (PackStartCode)", startCode, PackStartCode)
	}
}

// TestWriteNAV_PCK_SystemHeaderCode verifies the system header start code follows the pack header.
func TestWriteNAV_PCK_SystemHeaderCode(t *testing.T) {
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	if err := m.WriteNAV_PCK(&PCIPacket{}, &DSIPacket{}); err != nil {
		t.Fatalf("WriteNAV_PCK failed: %v", err)
	}
	data := buf.Bytes()
	// Pack header is 14 bytes; system header start code follows at byte 14.
	sysCode := binary.BigEndian.Uint32(data[14:18])
	if sysCode != SystemHeaderCode {
		t.Errorf("system header code = 0x%08X, want 0x%08X (SystemHeaderCode)", sysCode, SystemHeaderCode)
	}
}

// TestWriteNAV_PCK_SectorAdvance verifies the muxer advances its sector counter by 1.
func TestWriteNAV_PCK_SectorAdvance(t *testing.T) {
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	before := m.currentSector
	if err := m.WriteNAV_PCK(&PCIPacket{}, &DSIPacket{}); err != nil {
		t.Fatalf("WriteNAV_PCK failed: %v", err)
	}
	if m.currentSector != before+1 {
		t.Errorf("currentSector after NAV_PCK = %d, want %d", m.currentSector, before+1)
	}
}

// TestWriteNAV_PCK_RecordsSector verifies the sector address is appended to NAVPCKSectors.
func TestWriteNAV_PCK_RecordsSector(t *testing.T) {
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	m.currentSector = 42
	if err := m.WriteNAV_PCK(&PCIPacket{}, &DSIPacket{}); err != nil {
		t.Fatalf("WriteNAV_PCK failed: %v", err)
	}
	sectors := m.GetNAVPCKSectors()
	if len(sectors) != 1 {
		t.Fatalf("NAVPCKSectors len = %d, want 1", len(sectors))
	}
	if sectors[0] != 42 {
		t.Errorf("NAVPCKSectors[0] = %d, want 42", sectors[0])
	}
}

// TestWriteNAV_PCK_LBNInPCI verifies nv_pck_lbn in the PCI payload matches the muxer sector.
func TestWriteNAV_PCK_LBNInPCI(t *testing.T) {
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	m.currentSector = 100
	if err := m.WriteNAV_PCK(&PCIPacket{}, &DSIPacket{}); err != nil {
		t.Fatalf("WriteNAV_PCK failed: %v", err)
	}
	data := buf.Bytes()
	// PCI PES payload starts after:
	//   pack header   14 bytes
	//   system header 24 bytes
	//   PES header     6 bytes  (start_code[3] + stream_id[1] + length[2])
	// Total: 44 bytes to PES payload + 1 substream ID byte (0x00) = 45 bytes
	// to the PCI table data; nv_pck_lbn is its first uint32.
	const pciPayloadOff = 45
	lbn := binary.BigEndian.Uint32(data[pciPayloadOff+pciOffNVPCKLBN : pciPayloadOff+pciOffNVPCKLBN+4])
	if lbn != 100 {
		t.Errorf("PCI nv_pck_lbn = %d, want 100", lbn)
	}
}

// TestWriteNAV_PCK_VOBUSRIFilled verifies all VOBU_SRI entries in the DSI are SRIEndOfCell.
func TestWriteNAV_PCK_VOBUSRIFilled(t *testing.T) {
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	if err := m.WriteNAV_PCK(&PCIPacket{}, &DSIPacket{}); err != nil {
		t.Fatalf("WriteNAV_PCK failed: %v", err)
	}
	data := buf.Bytes()
	// DSI PES payload starts after PCI PES:
	//   pack header   14
	//   system header 24
	//   PCI PES hdr    6  + 980 payload = 986
	// Total: 14 + 24 + 986 = 1024 bytes before DSI PES header.
	const dsiPESHdrOff = 1024
	const dsiPayloadOff = dsiPESHdrOff + 7 // 6-byte PES header + substream ID (0x01)
	for i := 0; i < dsiVOBUSRICount; i++ {
		off := dsiPayloadOff + dsiOffVOBUSRI + i*4
		val := binary.BigEndian.Uint32(data[off : off+4])
		if val != SRIEndOfCell {
			t.Errorf("VOBU_SRI[%d] = 0x%08X, want SRIEndOfCell 0x%08X", i, val, SRIEndOfCell)
		}
	}
}
