package vob

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// ─── Pack header ──────────────────────────────────────────────────────────────

// TestWritePackHeader_Size verifies WritePackHeader writes exactly 14 bytes.
func TestWritePackHeader_Size(t *testing.T) {
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	if err := m.WritePackHeader(0); err != nil {
		t.Fatalf("WritePackHeader: %v", err)
	}
	if buf.Len() != 14 {
		t.Errorf("pack header size = %d, want 14", buf.Len())
	}
}

// TestWritePackHeader_StartCode verifies the pack header begins with 0x000001BA.
func TestWritePackHeader_StartCode(t *testing.T) {
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	if err := m.WritePackHeader(0); err != nil {
		t.Fatalf("WritePackHeader: %v", err)
	}
	sc := binary.BigEndian.Uint32(buf.Bytes()[0:4])
	if sc != PackStartCode {
		t.Errorf("pack header start code = 0x%08X, want 0x%08X", sc, PackStartCode)
	}
}

// TestWritePackHeader_StuffingByte verifies byte 13 encodes zero stuffing bytes
// (bits 7-3 = 11111 per the MPEG-PS spec).
func TestWritePackHeader_StuffingByte(t *testing.T) {
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	if err := m.WritePackHeader(0); err != nil {
		t.Fatalf("WritePackHeader: %v", err)
	}
	if buf.Bytes()[13]&0xF8 != 0xF8 {
		t.Errorf("pack header byte 13 = 0x%02X, want 0xF8 (stuffing=0)", buf.Bytes()[13])
	}
}

// ─── Sector alignment ─────────────────────────────────────────────────────────

// TestWriteVideo_SectorAligned verifies WriteVideo output is a multiple of 2048 bytes.
func TestWriteVideo_SectorAligned(t *testing.T) {
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	if err := m.WriteVideo(make([]byte, 512), 0); err != nil {
		t.Fatalf("WriteVideo: %v", err)
	}
	if buf.Len()%PackSize != 0 {
		t.Errorf("WriteVideo output %d bytes, not sector-aligned", buf.Len())
	}
}

// TestWriteAudio_SectorAligned verifies WriteAudio output is a multiple of 2048 bytes.
func TestWriteAudio_SectorAligned(t *testing.T) {
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	// 1536-byte AC-3 frame (typical size: 1536 samples at 48 kHz)
	if err := m.WriteAudio(make([]byte, 1536), 0, SubStreamAC3Base); err != nil {
		t.Fatalf("WriteAudio: %v", err)
	}
	if buf.Len()%PackSize != 0 {
		t.Errorf("WriteAudio output %d bytes, not sector-aligned", buf.Len())
	}
}

// TestWritePadding_StreamCode verifies the padding stream start code is 0x000001BE.
func TestWritePadding_StreamCode(t *testing.T) {
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	if err := m.WritePadding(100); err != nil {
		t.Fatalf("WritePadding: %v", err)
	}
	sc := binary.BigEndian.Uint32(buf.Bytes()[0:4])
	if sc != PaddingStreamCode {
		t.Errorf("padding stream code = 0x%08X, want 0x%08X", sc, PaddingStreamCode)
	}
}

// ─── SCR timing ───────────────────────────────────────────────────────────────

// TestMuxer_SCRAdvancesAfterVideo verifies the SCR increases by one NTSC frame (900900 ticks).
func TestMuxer_SCRAdvancesAfterVideo(t *testing.T) {
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	m.SetFrameRate(29.97)
	before := m.scr
	if err := m.WriteVideo(make([]byte, 200), 0); err != nil {
		t.Fatalf("WriteVideo: %v", err)
	}
	if m.scr <= before {
		t.Errorf("SCR did not advance after WriteVideo: before=%d after=%d", before, m.scr)
	}
	if m.scr-before != 900_900 {
		t.Errorf("NTSC SCR advance = %d, want 900900", m.scr-before)
	}
}

// TestMuxer_SCRAdvancesAfterAudio verifies the SCR increases by one AC-3 frame (864000 ticks).
func TestMuxer_SCRAdvancesAfterAudio(t *testing.T) {
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	before := m.scr
	if err := m.WriteAudio(make([]byte, 1536), 0, SubStreamAC3Base); err != nil {
		t.Fatalf("WriteAudio: %v", err)
	}
	if m.scr-before != ticksPerAC3Frame {
		t.Errorf("AC-3 SCR advance = %d, want %d", m.scr-before, ticksPerAC3Frame)
	}
}

// TestSetFrameRate_NTSC verifies 29.97 fps produces the exact 900900 tick interval
// (27MHz × 1001 / 30000) to avoid drift.
func TestSetFrameRate_NTSC(t *testing.T) {
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	m.SetFrameRate(29.97)
	const want = uint64(900_900)
	if m.ticksPerVideoFrame != want {
		t.Errorf("NTSC ticks/frame = %d, want %d", m.ticksPerVideoFrame, want)
	}
}

// TestSetFrameRate_PAL verifies 25 fps produces 1080000 ticks (27MHz / 25).
func TestSetFrameRate_PAL(t *testing.T) {
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	m.SetFrameRate(25.0)
	const want = uint64(27_000_000 / 25) // 1080000
	if m.ticksPerVideoFrame != want {
		t.Errorf("PAL ticks/frame = %d, want %d", m.ticksPerVideoFrame, want)
	}
}

// TestSetFrameRate_Zero verifies SetFrameRate(0) defaults to NTSC (29.97 fps).
func TestSetFrameRate_Zero(t *testing.T) {
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	m.SetFrameRate(0)
	if m.ticksPerVideoFrame != 900_900 {
		t.Errorf("SetFrameRate(0) ticks/frame = %d, want 900900 (NTSC default)", m.ticksPerVideoFrame)
	}
}

// ─── NAV_PCK DSI field compliance ────────────────────────────────────────────

// TestWriteNAV_PCK_DSI_LBN verifies nv_pck_lbn in the DSI payload matches the
// muxer's current sector. This is distinct from the PCI nv_pck_lbn (already tested
// in nav_test.go); both fields must agree per the DVD-Video spec.
func TestWriteNAV_PCK_DSI_LBN(t *testing.T) {
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	m.currentSector = 77
	if err := m.WriteNAV_PCK(&PCIPacket{}, &DSIPacket{}); err != nil {
		t.Fatalf("WriteNAV_PCK: %v", err)
	}
	data := buf.Bytes()
	// Layout within the 2048-byte sector:
	//   Pack header   14 bytes
	//   System header 24 bytes
	//   PCI PES hdr    6 bytes  } 986 bytes total PCI PES
	//   PCI payload  980 bytes  }
	//   DSI PES hdr    6 bytes
	//   DSI payload starts at byte 1030
	const dsiPayloadOff = 14 + 24 + 986 + 6
	lbn := binary.BigEndian.Uint32(data[dsiPayloadOff+dsiOffNVPCKLBN : dsiPayloadOff+dsiOffNVPCKLBN+4])
	if lbn != 77 {
		t.Errorf("DSI nv_pck_lbn = %d, want 77", lbn)
	}
}

// TestWriteNAV_PCK_PTM_UsedWhenSet verifies an explicit LVOBU_S_PTM in PCIPacket
// is written into the PCI payload instead of the auto-derived SCR value.
func TestWriteNAV_PCK_PTM_UsedWhenSet(t *testing.T) {
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	const wantPTM = uint32(90_000) // 1 second in 90kHz ticks
	if err := m.WriteNAV_PCK(&PCIPacket{LVOBU_S_PTM: wantPTM}, &DSIPacket{}); err != nil {
		t.Fatalf("WriteNAV_PCK: %v", err)
	}
	data := buf.Bytes()
	// PCI payload starts at byte 44 (pack14 + sys24 + PES_hdr6).
	const pciPayloadOff = 44
	ptm := binary.BigEndian.Uint32(data[pciPayloadOff+pciOffVOBUSPTM : pciPayloadOff+pciOffVOBUSPTM+4])
	if ptm != wantPTM {
		t.Errorf("PCI vobu_s_ptm = %d, want %d", ptm, wantPTM)
	}
}

// ─── Sub-stream ID constants ──────────────────────────────────────────────────

// TestSubStreamIDs verifies the DVD-Video sub-stream base IDs match the spec.
func TestSubStreamIDs(t *testing.T) {
	// DVD-Video spec §4.2: AC3 = 0x80..0x87, DTS = 0x88..0x8F, SPU = 0x20..0x3F.
	if SubStreamAC3Base != 0x80 {
		t.Errorf("SubStreamAC3Base = 0x%02X, want 0x80", SubStreamAC3Base)
	}
	if SubStreamDTSBase != 0x88 {
		t.Errorf("SubStreamDTSBase = 0x%02X, want 0x88", SubStreamDTSBase)
	}
	if SubStreamSPUBase != 0x20 {
		t.Errorf("SubStreamSPUBase = 0x%02X, want 0x20", SubStreamSPUBase)
	}
}

// ─── SRI layout constants ─────────────────────────────────────────────────────

// TestSRIByteOffset verifies sriByteOffset = 1226, derived from the MPEG-PS sector layout:
//
//	Pack header (14) + System header (24) + PCI PES (986) + DSI PES header (6) = 1030
//	DSI_GI (36) + SML_PBI (128) + SML_AGLI (32) = 196 bytes to VOBU_SRI
//	→ 1030 + 196 = 1226
func TestSRIByteOffset(t *testing.T) {
	const want = 1226
	if sriByteOffset != want {
		t.Errorf("sriByteOffset = %d, want %d", sriByteOffset, want)
	}
}

// TestSRICount verifies dsiVOBUSRICount = 30 (20 forward + 10 backward per DVD spec).
func TestSRICount(t *testing.T) {
	if dsiVOBUSRICount != 30 {
		t.Errorf("dsiVOBUSRICount = %d, want 30", dsiVOBUSRICount)
	}
}

// TestSRIEndOfCellValue verifies SRIEndOfCell = 0x3FFFFFFF per the DVD-Video spec.
func TestSRIEndOfCellValue(t *testing.T) {
	if SRIEndOfCell != 0x3FFFFFFF {
		t.Errorf("SRIEndOfCell = 0x%08X, want 0x3FFFFFFF", SRIEndOfCell)
	}
}

// TestTickSCR_Accumulates verifies TickSCR correctly advances the SCR by the given amount.
func TestTickSCR_Accumulates(t *testing.T) {
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	m.TickSCR(27_000_000) // 1 second at 27 MHz
	if m.scr != 27_000_000 {
		t.Errorf("scr after TickSCR(27000000) = %d, want 27000000", m.scr)
	}
	m.TickSCR(27_000_000)
	if m.scr != 54_000_000 {
		t.Errorf("scr after second TickSCR = %d, want 54000000", m.scr)
	}
}
