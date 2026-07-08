package vob

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
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

// TestWriteNAV_PCK_HLILayout locks the spec hli_t layout (audit A1/A2):
// hl_gi at PCI offset 96, btn_colit at 118, btni_t entries at 142 with the
// inline 8-byte VM command in entry bytes 10–17.
func TestWriteNAV_PCK_HLILayout(t *testing.T) {
	var buf bytes.Buffer
	m := NewMuxer(&buf)

	cmd := [8]byte{0x30, 0x02, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00} // JumpTT 1
	pci := &PCIPacket{
		HL_GI: HL_GI{BTN_SL_NS: 1},
		Buttons: []PCIButton{
			{X0: 100, X1: 300, Y0: 200, Y1: 240, Up: 2, Down: 2, Cmd: cmd},
			{X0: 100, X1: 300, Y0: 260, Y1: 300, Up: 1, Down: 1},
		},
	}
	if err := m.WriteNAV_PCK(pci, &DSIPacket{}); err != nil {
		t.Fatalf("WriteNAV_PCK: %v", err)
	}
	data := buf.Bytes()

	// PCI table data begins after 14+24+6 headers + 1 substream ID byte.
	const p = 45

	if got := binary.BigEndian.Uint16(data[p+96:]); got != 0x0001 {
		t.Errorf("hli_ss = 0x%04x, want 0x0001", got)
	}
	if data[p+110] != 0x10 {
		t.Errorf("btngr byte = 0x%02x, want 0x10 (btngr_ns=1, dsp_ty=0)", data[p+110])
	}
	if data[p+113] != 2 {
		t.Errorf("btn_ns = %d, want 2", data[p+113])
	}
	if data[p+114] != 2 {
		t.Errorf("nsl_btn_ns = %d, want 2", data[p+114])
	}
	if data[p+116] != 1 {
		t.Errorf("fosl_btnn = %d, want 1", data[p+116])
	}
	if got := binary.BigEndian.Uint32(data[p+118:]); got != 0x001000A0 {
		t.Errorf("SL_COLI = 0x%08x, want 0x001000A0", got)
	}
	if got := binary.BigEndian.Uint32(data[p+122:]); got != 0x003000F0 {
		t.Errorf("AC_COLI = 0x%08x, want 0x003000F0", got)
	}

	// btni_t entry 0 at 142. Expected packing for x0=100, x1=300, y0=200,
	// y1=240, btn_coln=1:
	//   b0 = 1<<6 | 100>>4 = 0x40|0x06 = 0x46
	//   b1 = (100&0xF)<<4 | 300>>8 = 0x40|0x01 = 0x41
	//   b2 = 300&0xFF = 0x2C
	//   b3 = 0<<6 | 200>>4 = 0x0C
	//   b4 = (200&0xF)<<4 | 240>>8 = 0x80|0x00 = 0x80
	//   b5 = 240&0xFF = 0xF0
	e := p + 142
	want := []byte{0x46, 0x41, 0x2C, 0x0C, 0x80, 0xF0}
	for i, w := range want {
		if data[e+i] != w {
			t.Errorf("btnit[0] byte %d = 0x%02x, want 0x%02x", i, data[e+i], w)
		}
	}
	// Neighbours: up=2, down=2, left/right default to self (1).
	if data[e+6] != 2 || data[e+7] != 2 || data[e+8] != 1 || data[e+9] != 1 {
		t.Errorf("neighbours = %v, want [2 2 1 1]", data[e+6:e+10])
	}
	// Inline command bytes 10-17.
	if !bytes.Equal(data[e+10:e+18], cmd[:]) {
		t.Errorf("btnit[0] cmd = % x, want % x", data[e+10:e+18], cmd[:])
	}
}

// TestAppendMenuVOB_RebasesNAV verifies LBN rebasing and VOB/Cell ID stamping
// across a concatenated multi-menu VOB (audit A9/A10).
func TestAppendMenuVOB_RebasesNAV(t *testing.T) {
	dir := t.TempDir()
	mpg := filepath.Join(dir, "menu.mpg")

	// Build a 3-sector menu MPG: NAV_PCK + two padding sectors.
	f, err := os.Create(mpg)
	if err != nil {
		t.Fatal(err)
	}
	m := NewMuxer(f)
	if err := m.WriteNAV_PCK(&PCIPacket{}, &DSIPacket{}); err != nil {
		t.Fatal(err)
	}
	if err := m.WritePadding(2 * PackSize); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	var out bytes.Buffer
	// Append the same file twice, as menus 1 and 2.
	n1, err := AppendMenuVOB(&out, mpg, 0, 1, 1)
	if err != nil {
		t.Fatalf("append 1: %v", err)
	}
	n2, err := AppendMenuVOB(&out, mpg, n1, 2, 1)
	if err != nil {
		t.Fatalf("append 2: %v", err)
	}
	if n1 != n2 {
		t.Fatalf("sector counts differ: %d vs %d", n1, n2)
	}

	data := out.Bytes()
	// First NAV (sector 0): LBN 0, VOB 1.
	pci0 := data[navSectorPCIOff:]
	dsi0 := data[navSectorDSIOff:]
	if got := binary.BigEndian.Uint32(pci0[pciOffNVPCKLBN:]); got != 0 {
		t.Errorf("menu1 PCI lbn = %d, want 0", got)
	}
	if got := binary.BigEndian.Uint16(dsi0[dsiOffVOBUVOBIDN:]); got != 1 {
		t.Errorf("menu1 vob_idn = %d, want 1", got)
	}
	// Second menu's NAV is at sector n1: LBN n1, VOB 2, cell 1.
	base := int(n1) * PackSize
	pci1 := data[base+navSectorPCIOff:]
	dsi1 := data[base+navSectorDSIOff:]
	if got := binary.BigEndian.Uint32(pci1[pciOffNVPCKLBN:]); got != n1 {
		t.Errorf("menu2 PCI lbn = %d, want %d", got, n1)
	}
	if got := binary.BigEndian.Uint32(dsi1[dsiOffNVPCKLBN:]); got != n1 {
		t.Errorf("menu2 DSI lbn = %d, want %d", got, n1)
	}
	if got := binary.BigEndian.Uint16(dsi1[dsiOffVOBUVOBIDN:]); got != 2 {
		t.Errorf("menu2 vob_idn = %d, want 2", got)
	}
	if dsi1[dsiOffVOBUCIDN] != 1 {
		t.Errorf("menu2 c_idn = %d, want 1", dsi1[dsiOffVOBUCIDN])
	}
}
