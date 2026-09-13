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
	list, cleanup, err := cellConcatList(dir, set, nil)
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
	list, cleanup, err := cellConcatList(dir, set, ti)
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
	list, cleanup, err := cellConcatList(dir, set, ti)
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
	list, cleanup, err := cellConcatList(dir, set, ti)
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
	list, cleanup, err := cellConcatList(dir, set, ti)
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