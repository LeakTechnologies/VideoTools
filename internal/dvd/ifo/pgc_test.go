package ifo

import (
	"encoding/binary"
	"testing"
)

// TestWritePGCITI_SectorPadded verifies the output is padded to a 2048-byte boundary.
func TestWritePGCITI_SectorPadded(t *testing.T) {
	pgc := BuildSingleCellPGC(0, 99, 30.0, true)
	data, err := WritePGCITI(pgc)
	if err != nil {
		t.Fatalf("WritePGCITI failed: %v", err)
	}
	if len(data)%2048 != 0 {
		t.Errorf("PGCITI length %d is not a multiple of 2048", len(data))
	}
	if len(data) < 2048 {
		t.Errorf("PGCITI length %d < 2048 (minimum one sector)", len(data))
	}
}

// TestWritePGCITI_NrOf verifies NrOf_VTS_PGCI == 1 at bytes 0-1.
func TestWritePGCITI_NrOf(t *testing.T) {
	pgc := BuildSingleCellPGC(0, 99, 30.0, true)
	data, err := WritePGCITI(pgc)
	if err != nil {
		t.Fatalf("WritePGCITI failed: %v", err)
	}
	nr := binary.BigEndian.Uint16(data[0:2])
	if nr != 1 {
		t.Errorf("NrOf_VTS_PGCI = %d, want 1", nr)
	}
}

// TestWritePGCITI_EntryCategory verifies the search pointer category == 0x8000 (entry PGC).
func TestWritePGCITI_EntryCategory(t *testing.T) {
	pgc := BuildSingleCellPGC(0, 99, 30.0, true)
	data, err := WritePGCITI(pgc)
	if err != nil {
		t.Fatalf("WritePGCITI failed: %v", err)
	}
	// PGCITI layout: 8-byte table header then 8-byte search pointer.
	// Category is at bytes 8-9.
	cat := binary.BigEndian.Uint16(data[8:10])
	if cat != 0x8000 {
		t.Errorf("Category = 0x%04X, want 0x8000 (entry PGC)", cat)
	}
}

// TestWritePGCITI_PGCStartByte verifies PGC_Start_Byte points past the table header + pointer.
func TestWritePGCITI_PGCStartByte(t *testing.T) {
	pgc := BuildSingleCellPGC(0, 99, 30.0, true)
	data, err := WritePGCITI(pgc)
	if err != nil {
		t.Fatalf("WritePGCITI failed: %v", err)
	}
	// PGC_Start_Byte is at bytes 12-15 within the search pointer (which starts at byte 8).
	pgcStart := binary.BigEndian.Uint32(data[12:16])
	const wantStart = uint32(16) // 8-byte header + 8-byte pointer = 16
	if pgcStart != wantStart {
		t.Errorf("PGC_Start_Byte = %d, want %d", pgcStart, wantStart)
	}
}

// TestWritePGCITI_EndByte verifies EndByte is consistent with the table content.
func TestWritePGCITI_EndByte(t *testing.T) {
	pgc := BuildSingleCellPGC(0, 99, 30.0, false)
	data, err := WritePGCITI(pgc)
	if err != nil {
		t.Fatalf("WritePGCITI failed: %v", err)
	}
	endByte := binary.BigEndian.Uint32(data[4:8])
	// endByte is 0-relative within the PGCITI; it must be < 2048 and > 15.
	if endByte < 16 {
		t.Errorf("EndByte = %d, want >= 16 (at least header+ptr+pgcHeader)", endByte)
	}
	if endByte >= 2048 {
		t.Errorf("EndByte = %d >= 2048 (exceeds one sector)", endByte)
	}
}

// TestBuildLinearTMAPT_InvalidInputs verifies nil is returned for degenerate inputs.
func TestBuildLinearTMAPT_InvalidInputs(t *testing.T) {
	cases := []struct {
		name     string
		sectors  uint32
		duration float64
		timeUnit int
	}{
		{"zero duration", 1000, 0.0, 1},
		{"negative duration", 1000, -5.0, 1},
		{"zero sectors", 0, 60.0, 1},
		{"zero timeUnit", 1000, 60.0, 0},
		{"negative timeUnit", 1000, 60.0, -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := BuildLinearTMAPT(c.sectors, c.duration, c.timeUnit)
			if got != nil {
				t.Errorf("BuildLinearTMAPT(%d, %f, %d) = non-nil, want nil",
					c.sectors, c.duration, c.timeUnit)
			}
		})
	}
}

// TestBuildLinearTMAPT_EntryCount verifies the correct number of entries is produced.
func TestBuildLinearTMAPT_EntryCount(t *testing.T) {
	// 60 seconds at 1s/entry => int(60/1)+1 = 61 entries.
	tmapt := BuildLinearTMAPT(1000, 60.0, 1)
	if tmapt == nil {
		t.Fatal("BuildLinearTMAPT returned nil")
	}
	want := 61
	if len(tmapt.Sectors) != want {
		t.Errorf("entry count = %d, want %d", len(tmapt.Sectors), want)
	}
}

// TestBuildLinearTMAPT_Bounds verifies first entry is 0 and last approaches totalSectors-1.
func TestBuildLinearTMAPT_Bounds(t *testing.T) {
	const totalSectors = uint32(1000)
	tmapt := BuildLinearTMAPT(totalSectors, 60.0, 1)
	if tmapt == nil {
		t.Fatal("BuildLinearTMAPT returned nil")
	}
	if tmapt.Sectors[0] != 0 {
		t.Errorf("Sectors[0] = %d, want 0", tmapt.Sectors[0])
	}
	last := tmapt.Sectors[len(tmapt.Sectors)-1]
	if last != totalSectors-1 {
		t.Errorf("Sectors[last] = %d, want %d", last, totalSectors-1)
	}
}

// TestWriteTMAPT_SectorPadded verifies output is padded to a 2048-byte boundary.
func TestWriteTMAPT_SectorPadded(t *testing.T) {
	tmapt := BuildLinearTMAPT(500, 30.0, 1)
	data, err := WriteTMAPT(tmapt)
	if err != nil {
		t.Fatalf("WriteTMAPT failed: %v", err)
	}
	if len(data)%2048 != 0 {
		t.Errorf("TMAPT length %d is not a multiple of 2048", len(data))
	}
}

// TestWriteTMAPT_NrOf verifies NrOf_VTS_TMAPTs == 1 at bytes 0-1.
func TestWriteTMAPT_NrOf(t *testing.T) {
	tmapt := BuildLinearTMAPT(500, 30.0, 1)
	data, err := WriteTMAPT(tmapt)
	if err != nil {
		t.Fatalf("WriteTMAPT failed: %v", err)
	}
	nr := binary.BigEndian.Uint16(data[0:2])
	if nr != 1 {
		t.Errorf("NrOf_VTS_TMAPTs = %d, want 1", nr)
	}
}

// TestWriteTMAPT_EntryHeader verifies NrOf_Entries and TimeUnit within the TMAP body.
func TestWriteTMAPT_EntryHeader(t *testing.T) {
	const timeUnit = 2
	const duration = 60.0
	tmapt := BuildLinearTMAPT(500, duration, timeUnit)
	data, err := WriteTMAPT(tmapt)
	if err != nil {
		t.Fatalf("WriteTMAPT failed: %v", err)
	}
	// TMAP body starts at byte 12 (8-byte header + 4-byte offset pointer).
	// [12-13] NrOf_Entries, [14] TimeUnit, [15] Reserved.
	nrEntries := binary.BigEndian.Uint16(data[12:14])
	wantEntries := uint16(len(tmapt.Sectors))
	if nrEntries != wantEntries {
		t.Errorf("NrOf_Entries = %d, want %d", nrEntries, wantEntries)
	}
	if data[14] != timeUnit {
		t.Errorf("TimeUnit = %d, want %d", data[14], timeUnit)
	}
}

// TestWriteTMAPT_NilError verifies WriteTMAPT returns an error for nil input.
func TestWriteTMAPT_NilError(t *testing.T) {
	_, err := WriteTMAPT(nil)
	if err == nil {
		t.Error("WriteTMAPT(nil) returned nil error, want error")
	}
}

// TestBuildVOBU_ADMAP_Nil verifies nil is returned for empty input.
func TestBuildVOBU_ADMAP_Nil(t *testing.T) {
	if got := BuildVOBU_ADMAP(nil); got != nil {
		t.Errorf("expected nil for empty input, got %+v", got)
	}
	if got := BuildVOBU_ADMAP([]uint32{}); got != nil {
		t.Errorf("expected nil for empty slice, got %+v", got)
	}
}

// TestBuildVOBU_ADMAP_EndByte verifies the EndByte field matches the DVD spec:
// EndByte = (total table size in bytes) - 1, where the table starts at the
// EndByte field itself (4 bytes) followed by N × 4-byte sector addresses.
func TestBuildVOBU_ADMAP_EndByte(t *testing.T) {
	sectors := []uint32{10, 20, 30}
	admap := BuildVOBU_ADMAP(sectors)
	if admap == nil {
		t.Fatal("expected non-nil ADMAP")
	}
	wantEndByte := uint32(4 + 3*4 - 1) // = 15
	if admap.EndByte != wantEndByte {
		t.Errorf("EndByte = %d, want %d", admap.EndByte, wantEndByte)
	}
}

// TestBuildVOBU_ADMAP_Sectors verifies sector addresses are preserved exactly.
func TestBuildVOBU_ADMAP_Sectors(t *testing.T) {
	want := []uint32{0, 15, 42, 100}
	admap := BuildVOBU_ADMAP(want)
	if admap == nil {
		t.Fatal("expected non-nil ADMAP")
	}
	if len(admap.Sectors) != len(want) {
		t.Fatalf("len(Sectors) = %d, want %d", len(admap.Sectors), len(want))
	}
	for i, s := range admap.Sectors {
		if s != want[i] {
			t.Errorf("Sectors[%d] = %d, want %d", i, s, want[i])
		}
	}
}

// TestSerializeVTSMAT_Identifier verifies the DVD identifier is at byte 0.
func TestSerializeVTSMAT_Identifier(t *testing.T) {
	mat := NewVTSMAT()
	b := SerializeVTSMAT(mat)
	if string(b[0:12]) != "DVDVIDEO-VTS" {
		t.Errorf("identifier = %q, want %q", string(b[0:12]), "DVDVIDEO-VTS")
	}
}

// TestSerializeVTSMAT_LastSector verifies VTS_Last_Sector is at byte 12.
func TestSerializeVTSMAT_LastSector(t *testing.T) {
	mat := NewVTSMAT()
	mat.VTS_Last_Sector = 0xDEADBEEF
	b := SerializeVTSMAT(mat)
	got := uint32(b[12])<<24 | uint32(b[13])<<16 | uint32(b[14])<<8 | uint32(b[15])
	if got != 0xDEADBEEF {
		t.Errorf("VTS_Last_Sector at byte 12 = 0x%X, want 0xDEADBEEF", got)
	}
}

// TestSerializeVTSMAT_PGCITOffset verifies VTS_PGCITI_Offset is at byte 422 (0x1A6).
func TestSerializeVTSMAT_PGCITOffset(t *testing.T) {
	mat := NewVTSMAT()
	mat.VTS_PGCITI_Offset = 1 // sector 1 (typical first-available sector)
	b := SerializeVTSMAT(mat)
	got := uint32(b[422])<<24 | uint32(b[423])<<16 | uint32(b[424])<<8 | uint32(b[425])
	if got != 1 {
		t.Errorf("VTS_PGCITI_Offset at byte 422 = %d, want 1", got)
	}
	// Also verify the old position (byte 229) is NOT the offset field.
	// Byte 229 should be zero for a default MAT.
	old := uint32(b[229])<<24 | uint32(b[230])<<16 | uint32(b[231])<<8 | uint32(b[232])
	if old == 1 {
		t.Errorf("VTS_PGCITI_Offset appears at old wrong position (byte 229); fix not applied")
	}
}

// TestSerializeVTSMAT_TMAPTOffset verifies VTS_TMAPTI_Offset is at byte 430 (0x1AE).
func TestSerializeVTSMAT_TMAPTOffset(t *testing.T) {
	mat := NewVTSMAT()
	mat.VTS_TMAPTI_Offset = 2
	b := SerializeVTSMAT(mat)
	got := uint32(b[430])<<24 | uint32(b[431])<<16 | uint32(b[432])<<8 | uint32(b[433])
	if got != 2 {
		t.Errorf("VTS_TMAPTI_Offset at byte 430 = %d, want 2", got)
	}
}

// TestSerializeVTSMAT_VOBUADMAPOffset verifies VTS_VOBU_ADMAP_Offset is at byte 446 (0x1BE).
func TestSerializeVTSMAT_VOBUADMAPOffset(t *testing.T) {
	mat := NewVTSMAT()
	mat.VTS_VOBU_ADMAP_Offset = 3
	b := SerializeVTSMAT(mat)
	got := uint32(b[446])<<24 | uint32(b[447])<<16 | uint32(b[448])<<8 | uint32(b[449])
	if got != 3 {
		t.Errorf("VTS_VOBU_ADMAP_Offset at byte 446 = %d, want 3", got)
	}
}

// TestSerializeVTSMAT_SectorSize verifies output is exactly 2048 bytes.
func TestSerializeVTSMAT_SectorSize(t *testing.T) {
	b := SerializeVTSMAT(NewVTSMAT())
	if len(b) != 2048 {
		t.Errorf("SerializeVTSMAT len = %d, want 2048", len(b))
	}
}

// TestSerializeVMGMAT_Identifier verifies the DVD identifier is at byte 0.
func TestSerializeVMGMAT_Identifier(t *testing.T) {
	mat := NewVMGMAT()
	b := SerializeVMGMAT(mat)
	if string(b[0:12]) != "DVDVIDEO-VMG" {
		t.Errorf("identifier = %q, want %q", string(b[0:12]), "DVDVIDEO-VMG")
	}
}

// TestSerializeVMGMAT_TT_SRPTOffset verifies TT_SRPT_Offset is at byte 192 (0x0C0).
func TestSerializeVMGMAT_TT_SRPTOffset(t *testing.T) {
	mat := NewVMGMAT()
	mat.TT_SRPT_Offset = 1
	b := SerializeVMGMAT(mat)
	got := uint32(b[192])<<24 | uint32(b[193])<<16 | uint32(b[194])<<8 | uint32(b[195])
	if got != 1 {
		t.Errorf("TT_SRPT_Offset at byte 192 = %d, want 1", got)
	}
}

// TestSerializeVMGMAT_NrOfTitleSets verifies NrOfTitleSets is at byte 72 (0x048).
func TestSerializeVMGMAT_NrOfTitleSets(t *testing.T) {
	mat := NewVMGMAT()
	mat.NrOfTitleSets = 3
	b := SerializeVMGMAT(mat)
	got := uint16(b[72])<<8 | uint16(b[73])
	if got != 3 {
		t.Errorf("NrOfTitleSets at byte 72 = %d, want 3", got)
	}
}

// TestWriteVTS_PTT_SRPT_SectorPadded verifies output is padded to 2048 bytes.
func TestWriteVTS_PTT_SRPT_SectorPadded(t *testing.T) {
	data, err := WriteVTS_PTT_SRPT(&VTS_PTT_SRPT{NrOfChapters: 3})
	if err != nil {
		t.Fatalf("WriteVTS_PTT_SRPT: %v", err)
	}
	if len(data)%2048 != 0 {
		t.Errorf("length %d not a multiple of 2048", len(data))
	}
}

// TestWriteVTS_PTT_SRPT_Header verifies the table header fields.
func TestWriteVTS_PTT_SRPT_Header(t *testing.T) {
	data, err := WriteVTS_PTT_SRPT(&VTS_PTT_SRPT{NrOfChapters: 2})
	if err != nil {
		t.Fatalf("WriteVTS_PTT_SRPT: %v", err)
	}
	// NrOf_Srpts at [0:2] = 1
	if nr := binary.BigEndian.Uint16(data[0:2]); nr != 1 {
		t.Errorf("NrOf_Srpts = %d, want 1", nr)
	}
	// Offset[0] at [8:12] = 12
	if off := binary.BigEndian.Uint32(data[8:12]); off != 12 {
		t.Errorf("Offset[0] = %d, want 12", off)
	}
}

// TestWriteVTS_PTT_SRPT_Entries verifies PGCN/PGN fields for each chapter.
func TestWriteVTS_PTT_SRPT_Entries(t *testing.T) {
	const n = 4
	data, err := WriteVTS_PTT_SRPT(&VTS_PTT_SRPT{NrOfChapters: n})
	if err != nil {
		t.Fatalf("WriteVTS_PTT_SRPT: %v", err)
	}
	base := 12 // first PTT entry starts after header(8) + offset(4)
	for i := 0; i < n; i++ {
		off := base + i*4
		pgcn := binary.BigEndian.Uint16(data[off : off+2])
		pgn := data[off+2]
		if pgcn != 1 {
			t.Errorf("chapter %d: PGCN = %d, want 1", i+1, pgcn)
		}
		if int(pgn) != i+1 {
			t.Errorf("chapter %d: PGN = %d, want %d", i+1, pgn, i+1)
		}
	}
}

// TestChapterCellsFromNAV_Basic verifies sector ranges split at chapter timestamps.
func TestChapterCellsFromNAV_Basic(t *testing.T) {
	// 100 VOBUs spread evenly over 100 seconds (1 sector per second)
	navSectors := make([]uint32, 100)
	for i := range navSectors {
		navSectors[i] = uint32(i * 10) // sectors 0, 10, 20, ..., 990
	}
	timestamps := []float64{0, 30.0, 70.0} // 3 chapters
	cells := ChapterCellsFromNAV(navSectors, timestamps, 100.0, 999)
	if len(cells) != 3 {
		t.Fatalf("len(cells) = %d, want 3", len(cells))
	}
	// Chapter 0 starts at sector 0 (navSectors[0])
	if cells[0].FirstSector != 0 {
		t.Errorf("cell[0].FirstSector = %d, want 0", cells[0].FirstSector)
	}
	// Last chapter ends at lastVOBSector
	if cells[2].LastSector != 999 {
		t.Errorf("cell[2].LastSector = %d, want 999", cells[2].LastSector)
	}
	// Cell boundaries must be non-decreasing
	for i := 1; i < len(cells); i++ {
		if cells[i].FirstSector < cells[i-1].FirstSector {
			t.Errorf("cell[%d].FirstSector (%d) < cell[%d].FirstSector (%d)",
				i, cells[i].FirstSector, i-1, cells[i-1].FirstSector)
		}
	}
}

// TestChapterCellsFromNAV_TooFew verifies nil returned for fewer than 2 chapters.
func TestChapterCellsFromNAV_TooFew(t *testing.T) {
	nav := []uint32{0, 10, 20}
	if got := ChapterCellsFromNAV(nav, []float64{0}, 30.0, 29); got != nil {
		t.Errorf("expected nil for 1 chapter, got %v", got)
	}
	if got := ChapterCellsFromNAV(nav, nil, 30.0, 29); got != nil {
		t.Errorf("expected nil for nil timestamps, got %v", got)
	}
}

// TestReadVMGI_RoundTrip verifies SerializeVMGMAT → ReadVMGI recovers all fields.
func TestReadVMGI_RoundTrip(t *testing.T) {
	mat := NewVMGMAT()
	mat.VMG_Last_Sector = 0xABCD
	mat.NrOfTitleSets = 5
	mat.TT_SRPT_Offset = 1
	mat.VMG_PGCITI_Offset = 2

	b := SerializeVMGMAT(mat)
	got, err := ReadVMGI(bytesReader(b))
	if err != nil {
		t.Fatalf("ReadVMGI: %v", err)
	}
	if got.VMG_Last_Sector != mat.VMG_Last_Sector {
		t.Errorf("VMG_Last_Sector = %d, want %d", got.VMG_Last_Sector, mat.VMG_Last_Sector)
	}
	if got.NrOfTitleSets != 5 {
		t.Errorf("NrOfTitleSets = %d, want 5", got.NrOfTitleSets)
	}
	if got.TT_SRPT_Offset != 1 {
		t.Errorf("TT_SRPT_Offset = %d, want 1", got.TT_SRPT_Offset)
	}
	if got.VMG_PGCITI_Offset != 2 {
		t.Errorf("VMG_PGCITI_Offset = %d, want 2", got.VMG_PGCITI_Offset)
	}
}

// TestReadVTSI_RoundTrip verifies SerializeVTSMAT → ReadVTSI recovers key fields.
func TestReadVTSI_RoundTrip(t *testing.T) {
	mat := NewVTSMAT()
	mat.VTS_Last_Sector = 0x1234
	mat.VTS_Audio_Streams_Count = 2
	mat.VTS_Audio_Attributes[0] = AudioAttributes{
		AudioCodingMode: 0,   // AC-3
		SampleRate:      0,   // 48 kHz
		NumChannels:     1,   // 2ch
	}
	mat.VTS_PGCITI_Offset = 3
	mat.VTS_VOBU_ADMAP_Offset = 7

	b := SerializeVTSMAT(mat)
	got, err := ReadVTSI(bytesReader(b))
	if err != nil {
		t.Fatalf("ReadVTSI: %v", err)
	}
	if got.VTS_Last_Sector != 0x1234 {
		t.Errorf("VTS_Last_Sector = %d, want 0x1234", got.VTS_Last_Sector)
	}
	if got.VTS_Audio_Streams_Count != 2 {
		t.Errorf("VTS_Audio_Streams_Count = %d, want 2", got.VTS_Audio_Streams_Count)
	}
	if got.VTS_Audio_Attributes[0].AudioCodingMode != 0 {
		t.Errorf("Audio[0].AudioCodingMode = %d, want 0 (AC-3)", got.VTS_Audio_Attributes[0].AudioCodingMode)
	}
	if got.VTS_PGCITI_Offset != 3 {
		t.Errorf("VTS_PGCITI_Offset = %d, want 3", got.VTS_PGCITI_Offset)
	}
	if got.VTS_VOBU_ADMAP_Offset != 7 {
		t.Errorf("VTS_VOBU_ADMAP_Offset = %d, want 7", got.VTS_VOBU_ADMAP_Offset)
	}
}

// bytesReader wraps a []byte in an io.Reader for use in round-trip tests.
func bytesReader(b []byte) interface{ Read([]byte) (int, error) } {
	return &sliceReader{b: b}
}

type sliceReader struct {
	b   []byte
	pos int
}

func (r *sliceReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.b) {
		return 0, nil
	}
	n := copy(p, r.b[r.pos:])
	r.pos += n
	return n, nil
}
