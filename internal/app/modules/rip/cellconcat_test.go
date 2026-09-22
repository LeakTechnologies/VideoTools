package rip

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LeakTechnologies/VideoTools/internal/dvd/ifo"
)

// writeTestVOBSet writes two VOB files of the given sector counts and returns
// the VIDEO_TS dir and a VobSet referencing them. Content VOBs are numbered
// VTS_01_1..VTS_01_N (VTS_01_0 is always the menu VOB and is excluded).
func writeTestVOBSet(t *testing.T, sectorCounts []uint32) (string, VobSet) {
	t.Helper()
	dir := t.TempDir()
	var files []string
	for i, sectors := range sectorCounts {
		path := filepath.Join(dir, fmt.Sprintf("VTS_01_%d.VOB", i+1))
		if err := os.WriteFile(path, make([]byte, int(sectors)*2048), 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		files = append(files, path)
	}
	return dir, VobSet{Name: "VTS_01", Files: files}
}

func TestCellConcatList_NoCells(t *testing.T) {
	dir, set := writeTestVOBSet(t, []uint32{100, 200})
	list, cleanup, err := cellConcatList(dir, set, nil, 0, 0)
	if err != nil {
		t.Fatalf("cellConcatList(nil info): %v", err)
	}
	if list != "" || cleanup != nil {
		t.Fatalf("no-cell title must fall back to whole-file concat, got list=%q cleanup=%v", list, cleanup != nil)
	}
}

func TestCellConcatList_HasAngles(t *testing.T) {
	dir, set := writeTestVOBSet(t, []uint32{100, 200})
	ti := &ifo.TitleInfo{
		HasAngles: true,
		Cells: []ifo.TitleCell{
			{VOBID: 1, CellID: 1, FirstSector: 10, LastSector: 20},
		},
	}
	list, cleanup, err := cellConcatList(dir, set, ti, 0, 0)
	if err != nil {
		t.Fatalf("cellConcatList(angles): %v", err)
	}
	if list != "" || cleanup != nil {
		t.Fatalf("angle content must not be sliced (interleaved data), got list=%q", list)
	}
}

func TestCellConcatList_CoversWholeSet(t *testing.T) {
	dir, set := writeTestVOBSet(t, []uint32{100, 200})
	ti := &ifo.TitleInfo{
		Cells: []ifo.TitleCell{
			{VOBID: 1, CellID: 1, FirstSector: 0, LastSector: 99},
			{VOBID: 2, CellID: 1, FirstSector: 0, LastSector: 199},
		},
	}
	list, cleanup, err := cellConcatList(dir, set, ti, 0, 0)
	if err != nil {
		t.Fatalf("cellConcatList(full cover): %v", err)
	}
	if list != "" || cleanup != nil {
		t.Fatalf("whole-set coverage must keep whole-file concat, got list=%q", list)
	}
}

func TestCellConcatList_MultiVOBPartial(t *testing.T) {
	dir, set := writeTestVOBSet(t, []uint32{100, 200})
	// A scene title occupies sectors 50-79 of VOB_1 and 10-99 of VOB_2.
	ti := &ifo.TitleInfo{
		Cells: []ifo.TitleCell{
			{VOBID: 1, CellID: 1, FirstSector: 50, LastSector: 79},
			{VOBID: 2, CellID: 1, FirstSector: 10, LastSector: 99},
		},
	}
	list, cleanup, err := cellConcatList(dir, set, ti, 0, 0)
	if err != nil {
		t.Fatalf("cellConcatList(partial): %v", err)
	}
	if list == "" || cleanup == nil {
		t.Fatal("partial title must produce a cell-accurate list")
	}
	defer cleanup()

	data, err := os.ReadFile(list)
	if err != nil {
		t.Fatalf("read list: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 slice entries, got %d: %q", len(lines), lines)
	}
	if !strings.Contains(lines[0], "vt-cell-") {
		t.Errorf("first entry %q does not reference a cell slice", lines[0])
	}
	// Slice files must exist and match the cell byte ranges exactly: VOB_1
	// [50,79] = 30 sectors, VOB_2 [10,99] = 90 sectors.
	sliceSizes := []int64{}
	for _, line := range lines {
		path := extractConcatPath(line)
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatalf("slice file %s missing: %v", path, err)
		}
		sliceSizes = append(sliceSizes, fi.Size())
	}
	want := []int64{30 * 2048, 90 * 2048}
	for i, w := range want {
		if sliceSizes[i] != w {
			t.Errorf("slice[%d] size = %d, want %d", i, sliceSizes[i], w)
		}
	}
}

func TestCellConcatList_MissingVOB(t *testing.T) {
	dir, set := writeTestVOBSet(t, []uint32{100, 200})
	// A cell references VOB_3, which does not exist in the set.
	ti := &ifo.TitleInfo{
		Cells: []ifo.TitleCell{
			{VOBID: 3, CellID: 1, FirstSector: 5, LastSector: 9},
		},
	}
	list, cleanup, err := cellConcatList(dir, set, ti, 0, 0)
	if err != nil {
		t.Fatalf("cellConcatList(missing VOB): %v", err)
	}
	if list != "" || cleanup != nil {
		t.Fatalf("unresolvable VOB must fall back to whole-file concat, got list=%q", list)
	}
}

// extractConcatPath parses a `file 'path'` line back into its path.
func extractConcatPath(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "file '")
	line = strings.TrimSuffix(line, "'")
	return strings.ReplaceAll(line, "''", "'")
}

// TestChapterCellSpan maps chapter ranges onto the PGC program map. Programs
// share an entry cell here (program 5 and program 6 both start at cell 6, so
// program 5 owns no cells), and a program's span runs from its own entry cell
// up to the entry cell of the program AFTER the range — never the program
// index nor the range's own entry cell.
func TestChapterCellSpan(t *testing.T) {
	ti := &ifo.TitleInfo{
		Cells: make([]ifo.TitleCell, 6),
		// 1-based program → entry cell: prog1→1, prog2→2, prog3→4, prog4→5, prog5→6, prog6→6
		ProgramEntryCells: []int{1, 2, 4, 5, 6, 6},
	}
	// Spans in 0-based cell indices:
	//   ch1 = cell0; ch2 = cells1-2; ch3 = cell3; ch4 = cell4; ch5 = ∅; ch6 = cell5
	want := []struct {
		cs, ce   int
		lo, hiEx int
		wantOK   bool
	}{
		{1, 6, 0, 6, true},  // whole title = all cells
		{2, 4, 1, 5, true},  // cells 1-4 (ch2:1-2, ch3:3, ch4:4)
		{3, 3, 3, 4, true},  // ch3 = cell3
		{6, 6, 5, 6, true},  // ch6 = cell5
		{4, 5, 4, 5, true},  // ch4 = cell4, ch5 is empty
		{2, 2, 1, 3, true},  // ch2 = cells1-2 (its next program enters at cell4)
		{5, 4, 0, 0, false}, // inverted range is unresolvable
		{7, 7, 0, 0, false}, // out-of-range start is unresolvable
		{0, 6, 0, 6, true},  // cs=0 means whole title
	}
	for _, w := range want {
		lo, hiEx, ok := chapterCellSpan(ti, w.cs, w.ce)
		if ok != w.wantOK || lo != w.lo || hiEx != w.hiEx {
			t.Errorf("chapterCellSpan(cs=%d,ce=%d) = (%d,%d,%v), want (%d,%d,%v)",
				w.cs, w.ce, lo, hiEx, ok, w.lo, w.hiEx, w.wantOK)
		}
	}

	// Missing program map → not resolvable (caller relies on output -ss/-to).
	if _, _, ok := chapterCellSpan(&ifo.TitleInfo{Cells: make([]ifo.TitleCell, 3)}, 1, 3); ok {
		t.Error("chapterCellSpan without a program map must report unresolvable")
	}
	// No cells → not resolvable.
	if _, _, ok := chapterCellSpan(&ifo.TitleInfo{ProgramEntryCells: []int{1}}, 1, 1); ok {
		t.Error("chapterCellSpan without cells must report unresolvable")
	}
}

// TestCellConcatList_ChapterRange verifies a chapter range restricts the slice
// to exactly the span's cells — even when the full title would cover the whole
// VOB set (range mode must NOT short-circuit to whole-file concat).
func TestCellConcatList_ChapterRange(t *testing.T) {
	dir, set := writeTestVOBSet(t, []uint32{100, 200})
	// Three cells covering the whole set: chapters 1-2 in cell 1, chapter 3 in
	// cell 2, chapter 4 in cell 3.
	ti := &ifo.TitleInfo{
		Cells: []ifo.TitleCell{
			{VOBID: 1, CellID: 1, FirstSector: 0, LastSector: 99},
			{VOBID: 2, CellID: 1, FirstSector: 0, LastSector: 99},
			{VOBID: 2, CellID: 2, FirstSector: 100, LastSector: 199},
		},
		ProgramEntryCells: []int{1, 1, 2, 3},
	}
	// Chapters 3-4 → cells 2-3 (span [2,4)).
	list, cleanup, err := cellConcatList(dir, set, ti, 3, 4)
	if err != nil {
		t.Fatalf("cellConcatList(range 3-4): %v", err)
	}
	if list == "" || cleanup == nil {
		t.Fatal("a chapter range that covers the whole set must STILL produce a cell slice (not whole-file)")
	}
	defer cleanup()

	data, err := os.ReadFile(list)
	if err != nil {
		t.Fatalf("read list: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	// Cells 2 and 3 are adjacent in VOB_2 (0-99, 100-199) so coalescing merges
	// them into a single 200-sector slice covering the whole span.
	if len(lines) != 1 {
		t.Fatalf("expected 1 coalesced slice entry, got %d: %q", len(lines), lines)
	}
	want := int64(200 * 2048)
	path := extractConcatPath(lines[0])
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("slice file %s missing: %v", path, err)
	}
	if fi.Size() != want {
		t.Errorf("slice size = %d, want %d", fi.Size(), want)
	}
	// Without a resolvable program map the range falls back to whole-file concat
	// (the caller bounds the rip with output-side -ss/-to).
	ti.ProgramEntryCells = nil
	if list, cleanup, err := cellConcatList(dir, set, ti, 3, 4); list != "" || cleanup != nil {
		t.Errorf("range without a program map must fall back to whole-file concat, got list=%q err=%v", list, err)
	}
}