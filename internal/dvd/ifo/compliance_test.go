package ifo

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// ─── TT_SRPT (Title Search Pointer Table) ────────────────────────────────────

// TestWriteTT_SRPT_SectorPadded verifies output is padded to a 2048-byte boundary.
func TestWriteTT_SRPT_SectorPadded(t *testing.T) {
	srpt := &TT_SRPT{
		NumTitles: 1,
		Titles:    []TitleSearchPointer{{VTSNumber: 1, VTS_TitleNumber: 1, StartSector: 20}},
	}
	data, err := WriteTT_SRPT(srpt)
	if err != nil {
		t.Fatalf("WriteTT_SRPT: %v", err)
	}
	if len(data)%2048 != 0 {
		t.Errorf("TT_SRPT length %d not a multiple of 2048", len(data))
	}
}

// TestWriteTT_SRPT_Header verifies NumTitles and EndByte match the spec formula.
func TestWriteTT_SRPT_Header(t *testing.T) {
	srpt := &TT_SRPT{
		NumTitles: 2,
		Titles: []TitleSearchPointer{
			{VTSNumber: 1, VTS_TitleNumber: 1, StartSector: 10},
			{VTSNumber: 2, VTS_TitleNumber: 1, StartSector: 20},
		},
	}
	data, err := WriteTT_SRPT(srpt)
	if err != nil {
		t.Fatalf("WriteTT_SRPT: %v", err)
	}
	if nr := binary.BigEndian.Uint16(data[0:2]); nr != 2 {
		t.Errorf("NumTitles = %d, want 2", nr)
	}
	// EndByte = 8 + N*12 - 1
	want := uint32(8 + 2*12 - 1)
	if eb := binary.BigEndian.Uint32(data[4:8]); eb != want {
		t.Errorf("EndByte = %d, want %d", eb, want)
	}
}

// TestWriteTT_SRPT_TitleEntry verifies the 12-byte layout of a title search pointer.
func TestWriteTT_SRPT_TitleEntry(t *testing.T) {
	srpt := &TT_SRPT{
		NumTitles: 1,
		Titles: []TitleSearchPointer{{
			TitleType:       0x80,
			NumAngles:       1,
			NumChapters:     3,
			VTSNumber:       1,
			VTS_TitleNumber: 1,
			StartSector:     0x100,
		}},
	}
	data, err := WriteTT_SRPT(srpt)
	if err != nil {
		t.Fatalf("WriteTT_SRPT: %v", err)
	}
	// First title entry starts at byte 8 (after 8-byte table header).
	e := data[8:]
	if e[0] != 0x80 {
		t.Errorf("TitleType = 0x%02X, want 0x80", e[0])
	}
	if e[1] != 1 {
		t.Errorf("NumAngles = %d, want 1", e[1])
	}
	if nc := binary.BigEndian.Uint16(e[2:4]); nc != 3 {
		t.Errorf("NumChapters = %d, want 3", nc)
	}
	if e[6] != 1 {
		t.Errorf("VTSNumber = %d, want 1", e[6])
	}
	if e[7] != 1 {
		t.Errorf("VTS_TitleNumber = %d, want 1", e[7])
	}
	if ss := binary.BigEndian.Uint32(e[8:12]); ss != 0x100 {
		t.Errorf("StartSector = 0x%X, want 0x100", ss)
	}
}

// ─── VMG_MAT mandatory constant fields ───────────────────────────────────────

// TestSerializeVMGMAT_DiscSide verifies Disc_Side = 1 at byte 42 (0x02A, side A)
// per libdvdread ifo_types.h vmgi_mat_t layout.
func TestSerializeVMGMAT_DiscSide(t *testing.T) {
	b := SerializeVMGMAT(NewVMGMAT())
	if b[42] != 1 {
		t.Errorf("Disc_Side (byte 42 / 0x02A) = %d, want 1 (side A)", b[42])
	}
}

// TestSerializeVMGMAT_NrOfVolumes verifies Nr_Of_Volumes = 1 at bytes 38-39 (0x026).
func TestSerializeVMGMAT_NrOfVolumes(t *testing.T) {
	b := SerializeVMGMAT(NewVMGMAT())
	if nr := binary.BigEndian.Uint16(b[38:40]); nr != 1 {
		t.Errorf("Nr_Of_Volumes (byte 38 / 0x026) = %d, want 1", nr)
	}
}

// TestSerializeVMGMAT_ThisVolumeNr verifies This_Volume_Nr = 1 at bytes 40-41 (0x028).
func TestSerializeVMGMAT_ThisVolumeNr(t *testing.T) {
	b := SerializeVMGMAT(NewVMGMAT())
	if nr := binary.BigEndian.Uint16(b[40:42]); nr != 1 {
		t.Errorf("This_Volume_Nr (byte 40 / 0x028) = %d, want 1", nr)
	}
}

// TestSerializeVMGMAT_MATLastByte verifies VMGI_Last_Byte = 2047 at bytes 128-131 (0x080).
func TestSerializeVMGMAT_MATLastByte(t *testing.T) {
	b := SerializeVMGMAT(NewVMGMAT())
	if v := binary.BigEndian.Uint32(b[128:132]); v != 2047 {
		t.Errorf("VMGI_Last_Byte (byte 128 / 0x080) = %d, want 2047", v)
	}
}

// ─── Identifier rejection ─────────────────────────────────────────────────────

// TestReadVMGI_RejectsInvalid verifies ReadVMGI returns an error for non-VMG data.
func TestReadVMGI_RejectsInvalid(t *testing.T) {
	bad := make([]byte, 2048)
	copy(bad[0:12], "GARBAGE-DATA")
	if _, err := ReadVMGI(bytesReader(bad)); err == nil {
		t.Error("ReadVMGI: expected error for invalid identifier, got nil")
	}
}

// TestReadVTSI_RejectsInvalid verifies ReadVTSI returns an error for non-VTS data.
func TestReadVTSI_RejectsInvalid(t *testing.T) {
	bad := make([]byte, 2048)
	copy(bad[0:12], "DVDVIDEO-VMG") // VMG identifier is invalid in a VTS context
	if _, err := ReadVTSI(bytesReader(bad)); err == nil {
		t.Error("ReadVTSI: expected error for VMG identifier in VTS context, got nil")
	}
}

// ─── BCD / PlaybackTime encoding ──────────────────────────────────────────────

// TestSecondsToPlaybackTime_BCD verifies hours, minutes, and seconds encode as BCD.
func TestSecondsToPlaybackTime_BCD(t *testing.T) {
	cases := []struct {
		secs   float64
		isNTSC bool
		h, m, s uint8
	}{
		{3723.0, true, 0x01, 0x02, 0x03},  // 1h 2m 3s
		{3600.0, false, 0x01, 0x00, 0x00}, // exactly 1 hour
		{59.0, false, 0x00, 0x00, 0x59},   // BCD 59
		{0.0, true, 0x00, 0x00, 0x00},
	}
	for _, c := range cases {
		pt := SecondsToPlaybackTime(c.secs, c.isNTSC)
		if pt.Hour != c.h {
			t.Errorf("secs=%.0f: Hour = 0x%02X, want 0x%02X", c.secs, pt.Hour, c.h)
		}
		if pt.Minute != c.m {
			t.Errorf("secs=%.0f: Minute = 0x%02X, want 0x%02X", c.secs, pt.Minute, c.m)
		}
		if pt.Second != c.s {
			t.Errorf("secs=%.0f: Second = 0x%02X, want 0x%02X", c.secs, pt.Second, c.s)
		}
	}
}

// TestSecondsToPlaybackTime_NTSC_FrameRateBits verifies bits 7-6 = 0b11 for NTSC.
func TestSecondsToPlaybackTime_NTSC_FrameRateBits(t *testing.T) {
	pt := SecondsToPlaybackTime(1.0, true)
	if pt.FrameRate&0xC0 != 0xC0 {
		t.Errorf("NTSC FrameRate high bits = 0x%02X, want 0xC0 prefix", pt.FrameRate)
	}
}

// TestSecondsToPlaybackTime_PAL_FrameRateBits verifies bits 7-6 = 0b01 for PAL.
func TestSecondsToPlaybackTime_PAL_FrameRateBits(t *testing.T) {
	pt := SecondsToPlaybackTime(1.0, false)
	if pt.FrameRate&0xC0 != 0x40 {
		t.Errorf("PAL FrameRate high bits = 0x%02X, want 0x40 prefix", pt.FrameRate)
	}
}

// ─── PGC structure ────────────────────────────────────────────────────────────

// TestBuildSingleCellPGC_SectorFields verifies sector addresses are copied into the PGC.
func TestBuildSingleCellPGC_SectorFields(t *testing.T) {
	const first, last = uint32(10), uint32(99)
	pgc := BuildSingleCellPGC(first, last, 30.0, true)
	if pgc.NrOfPrograms != 1 {
		t.Errorf("NrOfPrograms = %d, want 1", pgc.NrOfPrograms)
	}
	if pgc.NrOfCells != 1 {
		t.Errorf("NrOfCells = %d, want 1", pgc.NrOfCells)
	}
	if pgc.CellPlayback[0].FirstSector != first {
		t.Errorf("FirstSector = %d, want %d", pgc.CellPlayback[0].FirstSector, first)
	}
	if pgc.CellPlayback[0].LastSector != last {
		t.Errorf("LastSector = %d, want %d", pgc.CellPlayback[0].LastSector, last)
	}
}

// TestBuildChapterPGC_Counts verifies program and cell counts and EntryCell values.
func TestBuildChapterPGC_Counts(t *testing.T) {
	cells := []ChapterCell{
		{FirstSector: 0, LastSector: 99, Duration: 10},
		{FirstSector: 100, LastSector: 199, Duration: 10},
		{FirstSector: 200, LastSector: 299, Duration: 10},
	}
	pgc := BuildChapterPGC(cells, 30.0, false)
	if pgc == nil {
		t.Fatal("BuildChapterPGC returned nil")
	}
	if int(pgc.NrOfPrograms) != len(cells) {
		t.Errorf("NrOfPrograms = %d, want %d", pgc.NrOfPrograms, len(cells))
	}
	if int(pgc.NrOfCells) != len(cells) {
		t.Errorf("NrOfCells = %d, want %d", pgc.NrOfCells, len(cells))
	}
	for i, p := range pgc.Programs {
		if int(p.EntryCell) != i+1 {
			t.Errorf("Programs[%d].EntryCell = %d, want %d", i, p.EntryCell, i+1)
		}
	}
}

// TestBuildChapterPGC_Nil verifies nil/empty input returns nil.
func TestBuildChapterPGC_Nil(t *testing.T) {
	if got := BuildChapterPGC(nil, 30.0, true); got != nil {
		t.Error("BuildChapterPGC(nil) should return nil")
	}
	if got := BuildChapterPGC([]ChapterCell{}, 30.0, true); got != nil {
		t.Error("BuildChapterPGC(empty) should return nil")
	}
}

// TestWritePGCITI_PGCHeaderFields verifies NrOfPrograms and NrOfCells are at the
// correct byte offsets within the serialized PGC (PGC starts at byte 16).
func TestWritePGCITI_PGCHeaderFields(t *testing.T) {
	pgc := BuildSingleCellPGC(0, 99, 60.0, false)
	data, err := WritePGCITI(pgc)
	if err != nil {
		t.Fatalf("WritePGCITI: %v", err)
	}
	// PGC data begins at byte 16 (8-byte table header + 8-byte search pointer).
	// Within the PGC: bytes 0-1 reserved, byte 2 = NrOfPrograms, byte 3 = NrOfCells.
	const pgcBase = 16
	if data[pgcBase+2] != 1 {
		t.Errorf("PGC NrOfPrograms at byte %d = %d, want 1", pgcBase+2, data[pgcBase+2])
	}
	if data[pgcBase+3] != 1 {
		t.Errorf("PGC NrOfCells at byte %d = %d, want 1", pgcBase+3, data[pgcBase+3])
	}
}

// ─── Builder integration ──────────────────────────────────────────────────────

// TestGenerateVMG_IFO_CreatesFiles verifies both IFO and BUP files are created.
func TestGenerateVMG_IFO_CreatesFiles(t *testing.T) {
	dir := t.TempDir()
	b := NewBuilder(dir)
	mat := NewVMGMAT()
	mat.NrOfTitleSets = 1
	if err := b.GenerateVMG_IFO(mat, nil, nil, nil); err != nil {
		t.Fatalf("GenerateVMG_IFO: %v", err)
	}
	for _, name := range []string{"VIDEO_TS.IFO", "VIDEO_TS.BUP"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s not created: %v", name, err)
		}
	}
}

// TestGenerateVMG_IFO_ValidIdentifier verifies the output IFO has the VMG identifier at byte 0.
func TestGenerateVMG_IFO_ValidIdentifier(t *testing.T) {
	dir := t.TempDir()
	b := NewBuilder(dir)
	mat := NewVMGMAT()
	mat.NrOfTitleSets = 1
	if err := b.GenerateVMG_IFO(mat, nil, nil, nil); err != nil {
		t.Fatalf("GenerateVMG_IFO: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "VIDEO_TS.IFO"))
	if err != nil {
		t.Fatalf("read IFO: %v", err)
	}
	if string(data[0:12]) != "DVDVIDEO-VMG" {
		t.Errorf("IFO identifier = %q, want %q", string(data[0:12]), "DVDVIDEO-VMG")
	}
}

// TestGenerateVMG_IFO_BUPMatchesIFO verifies the BUP is an exact byte-for-byte copy of the IFO.
func TestGenerateVMG_IFO_BUPMatchesIFO(t *testing.T) {
	dir := t.TempDir()
	b := NewBuilder(dir)
	mat := NewVMGMAT()
	mat.NrOfTitleSets = 1
	srpt := &TT_SRPT{
		NumTitles: 1,
		Titles:    []TitleSearchPointer{{VTSNumber: 1, VTS_TitleNumber: 1, StartSector: 20}},
	}
	if err := b.GenerateVMG_IFO(mat, srpt, nil, nil); err != nil {
		t.Fatalf("GenerateVMG_IFO: %v", err)
	}
	ifo, _ := os.ReadFile(filepath.Join(dir, "VIDEO_TS.IFO"))
	bup, _ := os.ReadFile(filepath.Join(dir, "VIDEO_TS.BUP"))
	if !bytes.Equal(ifo, bup) {
		t.Error("VIDEO_TS.BUP differs from VIDEO_TS.IFO (BUP must be an exact copy)")
	}
}

// TestGenerateVTS_IFO_CreatesFiles verifies VTS_xx_0.IFO and VTS_xx_0.BUP are created.
func TestGenerateVTS_IFO_CreatesFiles(t *testing.T) {
	dir := t.TempDir()
	b := NewBuilder(dir)
	mat := NewVTSMAT()
	pgc := BuildSingleCellPGC(0, 499, 60.0, true)
	admap := BuildVOBU_ADMAP([]uint32{0, 50, 100})
	if err := b.GenerateVTS_IFO(1, mat, pgc, nil, admap, nil); err != nil {
		t.Fatalf("GenerateVTS_IFO: %v", err)
	}
	for _, name := range []string{"VTS_01_0.IFO", "VTS_01_0.BUP"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s not created: %v", name, err)
		}
	}
}

// TestGenerateVTS_IFO_ValidIdentifier verifies the output VTS IFO has the correct identifier.
func TestGenerateVTS_IFO_ValidIdentifier(t *testing.T) {
	dir := t.TempDir()
	b := NewBuilder(dir)
	mat := NewVTSMAT()
	pgc := BuildSingleCellPGC(0, 499, 60.0, true)
	if err := b.GenerateVTS_IFO(1, mat, pgc, nil, nil, nil); err != nil {
		t.Fatalf("GenerateVTS_IFO: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "VTS_01_0.IFO"))
	if err != nil {
		t.Fatalf("read VTS IFO: %v", err)
	}
	if string(data[0:12]) != "DVDVIDEO-VTS" {
		t.Errorf("VTS IFO identifier = %q, want %q", string(data[0:12]), "DVDVIDEO-VTS")
	}
}

// TestGenerateVTS_IFO_BUPMatchesIFO verifies the BUP is an exact copy of the IFO.
func TestGenerateVTS_IFO_BUPMatchesIFO(t *testing.T) {
	dir := t.TempDir()
	b := NewBuilder(dir)
	mat := NewVTSMAT()
	pgc := BuildSingleCellPGC(0, 499, 60.0, true)
	admap := BuildVOBU_ADMAP([]uint32{0, 50, 100})
	if err := b.GenerateVTS_IFO(1, mat, pgc, nil, admap, nil); err != nil {
		t.Fatalf("GenerateVTS_IFO: %v", err)
	}
	ifo, _ := os.ReadFile(filepath.Join(dir, "VTS_01_0.IFO"))
	bup, _ := os.ReadFile(filepath.Join(dir, "VTS_01_0.BUP"))
	if !bytes.Equal(ifo, bup) {
		t.Error("VTS_01_0.BUP differs from VTS_01_0.IFO (BUP must be an exact copy)")
	}
}

// TestGenerateVTS_IFO_SectorLayout verifies table offsets are non-zero and strictly ascending.
func TestGenerateVTS_IFO_SectorLayout(t *testing.T) {
	dir := t.TempDir()
	b := NewBuilder(dir)
	mat := NewVTSMAT()
	pgc := BuildSingleCellPGC(0, 999, 60.0, true)
	tmapt := BuildLinearTMAPT(1000, 60.0, 1)
	admap := BuildVOBU_ADMAP([]uint32{0, 10, 20})
	pttsrpt := &VTS_PTT_SRPT{NrOfChapters: 1}
	if err := b.GenerateVTS_IFO(1, mat, pgc, tmapt, admap, pttsrpt); err != nil {
		t.Fatalf("GenerateVTS_IFO: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "VTS_01_0.IFO"))
	got, err := ReadVTSI(bytesReader(data))
	if err != nil {
		t.Fatalf("ReadVTSI: %v", err)
	}
	// All offsets must point past sector 0 (the MAT).
	if got.VTS_PTT_SRPT_Offset == 0 {
		t.Error("VTS_PTT_SRPT_Offset is 0")
	}
	if got.VTS_PGCITI_Offset == 0 {
		t.Error("VTS_PGCITI_Offset is 0")
	}
	if got.VTS_TMAPTI_Offset == 0 {
		t.Error("VTS_TMAPTI_Offset is 0")
	}
	if got.VTS_VOBU_ADMAP_Offset == 0 {
		t.Error("VTS_VOBU_ADMAP_Offset is 0")
	}
	// Offsets must be strictly ascending in the order they are written.
	if got.VTS_PTT_SRPT_Offset >= got.VTS_PGCITI_Offset {
		t.Errorf("PTT_SRPT (%d) >= PGCITI (%d)", got.VTS_PTT_SRPT_Offset, got.VTS_PGCITI_Offset)
	}
	if got.VTS_PGCITI_Offset >= got.VTS_TMAPTI_Offset {
		t.Errorf("PGCITI (%d) >= TMAPT (%d)", got.VTS_PGCITI_Offset, got.VTS_TMAPTI_Offset)
	}
	if got.VTS_TMAPTI_Offset >= got.VTS_VOBU_ADMAP_Offset {
		t.Errorf("TMAPT (%d) >= ADMAP (%d)", got.VTS_TMAPTI_Offset, got.VTS_VOBU_ADMAP_Offset)
	}
}

// TestGenerateVTS_IFO_MultipleVTS verifies two distinct VTS files can be generated.
func TestGenerateVTS_IFO_MultipleVTS(t *testing.T) {
	dir := t.TempDir()
	b := NewBuilder(dir)
	for n := 1; n <= 2; n++ {
		mat := NewVTSMAT()
		pgc := BuildSingleCellPGC(0, 99, 10.0, true)
		if err := b.GenerateVTS_IFO(n, mat, pgc, nil, nil, nil); err != nil {
			t.Fatalf("GenerateVTS_IFO(%d): %v", n, err)
		}
	}
	for _, name := range []string{"VTS_01_0.IFO", "VTS_02_0.IFO"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s not created: %v", name, err)
		}
	}
}

// TestGenerateVTS_IFO_IFOSectorAligned verifies the IFO file size is sector-aligned.
func TestGenerateVTS_IFO_IFOSectorAligned(t *testing.T) {
	dir := t.TempDir()
	b := NewBuilder(dir)
	mat := NewVTSMAT()
	pgc := BuildSingleCellPGC(0, 299, 30.0, false)
	tmapt := BuildLinearTMAPT(300, 30.0, 1)
	admap := BuildVOBU_ADMAP([]uint32{0, 30, 60, 90})
	if err := b.GenerateVTS_IFO(1, mat, pgc, tmapt, admap, nil); err != nil {
		t.Fatalf("GenerateVTS_IFO: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "VTS_01_0.IFO"))
	if err != nil {
		t.Fatalf("stat IFO: %v", err)
	}
	if info.Size()%2048 != 0 {
		t.Errorf("IFO file size %d is not a multiple of 2048", info.Size())
	}
}

// TestGenerateVMG_IFO_TT_SRPTOffsetUpdated verifies TT_SRPT_Offset is written
// into the MAT when a srpt is provided.
func TestGenerateVMG_IFO_TT_SRPTOffsetUpdated(t *testing.T) {
	dir := t.TempDir()
	b := NewBuilder(dir)
	mat := NewVMGMAT()
	mat.NrOfTitleSets = 1
	srpt := &TT_SRPT{
		NumTitles: 1,
		Titles:    []TitleSearchPointer{{VTSNumber: 1, VTS_TitleNumber: 1, StartSector: 16}},
	}
	if err := b.GenerateVMG_IFO(mat, srpt, nil, nil); err != nil {
		t.Fatalf("GenerateVMG_IFO: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "VIDEO_TS.IFO"))
	got, err := ReadVMGI(bytesReader(data))
	if err != nil {
		t.Fatalf("ReadVMGI: %v", err)
	}
	if got.TT_SRPT_Offset == 0 {
		t.Error("TT_SRPT_Offset in written IFO is 0; expected non-zero when srpt provided")
	}
}
