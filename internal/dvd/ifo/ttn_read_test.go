package ifo

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// chapterCells builds nChapter chapter cells of secs seconds each, in rising
// VOB sector order starting at startSector.
func chapterCells(nChapter int, startSector uint32, secs float64) []ChapterCell {
	cells := make([]ChapterCell, nChapter)
	for i := range cells {
		cells[i] = ChapterCell{
			FirstSector: startSector + uint32(i),
			LastSector:  startSector + uint32(i),
			Duration:    secs,
		}
	}
	return cells
}

// writeMultiPGCVTSIFO builds a synthetic VTS_01_0.IFO whose VTS_PGCITI holds
// one PGC per chapter-cell frame, with each SRP entry's TitleNr byte written
// like a real multi-PGC disc (bit7 set, value = last VTS TTN sharing the PGC).
func writeMultiPGCVTSIFO(t *testing.T, dir string, cells [][]ChapterCell, titleNrs []byte) string {
	t.Helper()
	if len(cells) != len(titleNrs) {
		t.Fatalf("cells/titleNrs length mismatch: %d vs %d", len(cells), len(titleNrs))
	}
	var pgcs []*ProgramChain
	for _, c := range cells {
		dur := 0.0
		for _, cc := range c {
			dur += cc.Duration
		}
		pgcs = append(pgcs, BuildChapterPGC(c, dur, false))
	}
	pgciti, err := WritePGCITIs(pgcs)
	if err != nil {
		t.Fatalf("WritePGCITIs: %v", err)
	}
	// Each 8-byte SRP entry begins after the 8-byte header; byte 0 is TitleNr.
	for i := range pgcs {
		pgciti[8+i*8] = titleNrs[i]
	}

	mat := NewVTSMAT()
	mat.VTS_PGCITI_Offset = 1
	var buf bytes.Buffer
	buf.Write(SerializeVTSMAT(mat))
	buf.Write(pgciti)

	ifoPath := filepath.Join(dir, "VTS_01_0.IFO")
	if err := os.WriteFile(ifoPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("write IFO: %v", err)
	}
	return ifoPath
}

func assertTitleDur(t *testing.T, got *TitleInfo, wantDur float64, wantChapters []float64) {
	t.Helper()
	if got == nil {
		t.Fatal("ReadTitleInfoForTTN returned nil info")
	}
	if math.Abs(got.Duration-wantDur) > 0.01 {
		t.Errorf("duration = %.3f, want %.3f", got.Duration, wantDur)
	}
	if len(got.Chapters) != len(wantChapters) {
		t.Fatalf("chapters = %v, want %v", got.Chapters, wantChapters)
	}
	for i := range wantChapters {
		if math.Abs(got.Chapters[i]-wantChapters[i]) > 0.01 {
			t.Errorf("chapter[%d] = %.3f, want %.3f", i, got.Chapters[i], wantChapters[i])
		}
	}
}

// TestReadTitleInfoForTTN_SharedPGC verifies the shared-PGC rule on a disc
// where two PGCs serve five titles (TitleNr stores the last sharing TTN).
func TestReadTitleInfoForTTN_SharedPGC(t *testing.T) {
	dir := t.TempDir()
	cells := [][]ChapterCell{
		chapterCells(2, 5, 30), // PGC A: 60 s, chapters 0/30
		chapterCells(3, 7, 50), // PGC B: 150 s, chapters 0/50/100
	}
	ifoPath := writeMultiPGCVTSIFO(t, dir, cells, []byte{0x82, 0x85})

	want := []struct {
		ttn     int
		dur     float64
		chapers []float64
	}{
		{1, 60, []float64{0, 30}},
		{2, 60, []float64{0, 30}}, // shares PGC A with TTN 1
		{3, 150, []float64{0, 50, 100}},
		{4, 150, []float64{0, 50, 100}}, // shares PGC B with TTN 3
		{5, 150, []float64{0, 50, 100}},
	}
	for _, w := range want {
		info, err := ReadTitleInfoForTTN(ifoPath, w.ttn)
		if err != nil {
			t.Fatalf("ReadTitleInfoForTTN(%d): %v", w.ttn, err)
		}
		assertTitleDur(t, info, w.dur, w.chapers)
	}

	// Out-of-range TTN falls back to the last PGC.
	info, err := ReadTitleInfoForTTN(ifoPath, 6)
	if err != nil {
		t.Fatalf("ReadTitleInfoForTTN(6): %v", err)
	}
	assertTitleDur(t, info, 150, []float64{0, 50, 100})

	// Legacy first title-domain PGC behaviour.
	info, err = ReadTitleInfo(ifoPath)
	if err != nil {
		t.Fatalf("ReadTitleInfo: %v", err)
	}
	assertTitleDur(t, info, 60, []float64{0, 30})
}

// TestReadTitleInfo_Cells verifies per-cell sector extents, VOBID and CellID
// are populated from the PGC cell playback + cell position tables.
func TestReadTitleInfo_Cells(t *testing.T) {
	dir := t.TempDir()
	cells := [][]ChapterCell{
		{
			{FirstSector: 0x100, LastSector: 0x2FF, Duration: 30},
			{FirstSector: 0x300, LastSector: 0x3FF, Duration: 40},
			{FirstSector: 0x400, LastSector: 0x4FF, Duration: 50},
		},
	}
	ifoPath := writeMultiPGCVTSIFO(t, dir, cells, []byte{0x81})

	info, err := ReadTitleInfoForTTN(ifoPath, 1)
	if err != nil {
		t.Fatalf("ReadTitleInfoForTTN(1): %v", err)
	}
	if info == nil {
		t.Fatal("ReadTitleInfoForTTN returned nil")
	}
	if len(info.Cells) != 3 {
		t.Fatalf("len(Cells) = %d, want 3", len(info.Cells))
	}
	want := []TitleCell{
		{VOBID: 1, CellID: 1, FirstSector: 0x100, LastSector: 0x2FF},
		{VOBID: 1, CellID: 2, FirstSector: 0x300, LastSector: 0x3FF},
		{VOBID: 1, CellID: 3, FirstSector: 0x400, LastSector: 0x4FF},
	}
	for i, ow := range want {
		if info.Cells[i] != ow {
			t.Errorf("Cells[%d] = %+v, want %+v", i, info.Cells[i], ow)
		}
	}
	// ProgramEntryCells must mirror the PGC program map: program p → 1-based
	// entry cell number (one program per cell in these synthetic PGCs).
	wantProg := []int{1, 2, 3}
	if len(info.ProgramEntryCells) != len(wantProg) {
		t.Fatalf("ProgramEntryCells = %v, want %v", info.ProgramEntryCells, wantProg)
	}
	for i, w := range wantProg {
		if info.ProgramEntryCells[i] != w {
			t.Errorf("ProgramEntryCells[%d] = %d, want %d", i, info.ProgramEntryCells[i], w)
		}
	}
}

// TestReadTitleInfoForTTN_IndexFallback verifies TTN selection falls back to
// the entry index when disc authors leave the TitleNr field unset (all masks
// are 0) and instead order entries by TTN.
func TestReadTitleInfoForTTN_IndexFallback(t *testing.T) {
	dir := t.TempDir()
	cells := [][]ChapterCell{
		chapterCells(1, 1, 30), // 30 s
		chapterCells(2, 2, 30), // 60 s
		chapterCells(3, 3, 30), // 90 s
	}
	ifoPath := writeMultiPGCVTSIFO(t, dir, cells, []byte{0x80, 0x80, 0x80})

	want := []struct {
		ttn     int
		dur     float64
		chapers []float64
	}{
		{1, 30, []float64{0}},
		{2, 60, []float64{0, 30}},
		{3, 90, []float64{0, 30, 60}},
	}
	for _, w := range want {
		info, err := ReadTitleInfoForTTN(ifoPath, w.ttn)
		if err != nil {
			t.Fatalf("ReadTitleInfoForTTN(%d): %v", w.ttn, err)
		}
		assertTitleDur(t, info, w.dur, w.chapers)
	}
}

// TestReadTitleInfoForTTN_ZeroBehaviour verifies ttn <= 0 is the same as the
// legacy first title-domain read.
func TestReadTitleInfoForTTN_ZeroBehaviour(t *testing.T) {
	dir := t.TempDir()
	cells := [][]ChapterCell{
		chapterCells(2, 5, 30),
		chapterCells(3, 7, 50),
	}
	ifoPath := writeMultiPGCVTSIFO(t, dir, cells, []byte{0x82, 0x85})

	info, err := ReadTitleInfoForTTN(ifoPath, 0)
	if err != nil {
		t.Fatalf("ReadTitleInfoForTTN(0): %v", err)
	}
	legacy, err := ReadTitleInfo(ifoPath)
	if err != nil {
		t.Fatalf("ReadTitleInfo: %v", err)
	}
	if info.Duration != legacy.Duration || len(info.Chapters) != len(legacy.Chapters) {
		t.Errorf("ttn=0 (%f/%v) diverges from legacy (%f/%v)",
			info.Duration, info.Chapters, legacy.Duration, legacy.Chapters)
	}
}
