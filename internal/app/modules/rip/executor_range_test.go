package rip

import (
	"math"
	"os"
	"strings"
	"testing"

	"github.com/LeakTechnologies/VideoTools/internal/dvd/ifo"
)

// TestChapterRange verifies the chapter-range resolver: base = first chapter's
// start time, endSec = the chapter after the range (or the title duration for
// the last chapter), remap normalises start times to the range base, and the
// duration is the span length.
func TestChapterRange(t *testing.T) {
	chapters := []float64{0, 30, 50, 80, 100}
	dur := 150.0
	want := []struct {
		cs, ce int
		base   float64
		endSec float64
		span   float64
		remap  []float64
		ok     bool
	}{
		{2, 4, 30, 100, 70, []float64{0, 20, 50}, true},
		{1, 5, 0, 150, 150, []float64{0, 30, 50, 80, 100}, true},
		{3, 0, 50, 150, 100, []float64{0, 30, 50}, true}, // ce=0 ⇒ last chapter; remap = chapter starts
		{5, 5, 100, 150, 50, []float64{0}, true},
		{2, 2, 30, 50, 20, []float64{0}, true},
		{1, 1, 0, 30, 30, []float64{0}, true},
		{3, 2, 0, 0, 0, nil, false}, // inverted range
		{6, 6, 0, 0, 0, nil, false}, // out-of-range start (> n)
	}
	for _, w := range want {
		remap, base, endSec, span, ok := chapterRange(chapters, dur, w.cs, w.ce)
		if ok != w.ok {
			t.Errorf("chapterRange(cs=%d,ce=%d) ok = %v, want %v", w.cs, w.ce, ok, w.ok)
			continue
		}
		if !w.ok {
			continue
		}
		if math.Abs(base-w.base) > 0.001 || math.Abs(endSec-w.endSec) > 0.001 || math.Abs(span-w.span) > 0.001 {
			t.Errorf("chapterRange(cs=%d,ce=%d) = base %.2f end %.2f span %.2f, want base %.2f end %.2f span %.2f",
				w.cs, w.ce, base, endSec, span, w.base, w.endSec, w.span)
		}
		if len(remap) != len(w.remap) {
			t.Errorf("chapterRange(cs=%d,ce=%d) remap = %v, want %v", w.cs, w.ce, remap, w.remap)
			continue
		}
		for i := range remap {
			if math.Abs(remap[i]-w.remap[i]) > 0.001 {
				t.Errorf("chapterRange(cs=%d,ce=%d) remap[%d] = %.2f, want %.2f", w.cs, w.ce, i, remap[i], w.remap[i])
			}
		}
	}

	// A title without chapter data cannot resolve a range.
	if _, _, _, _, ok := chapterRange(nil, 100, 1, 2); ok {
		t.Error("chapterRange without chapters must report unresolvable")
	}
}

// TestCellConcatList_ChapterRangeWholeTitle ensures that (unlike a partial
// range) the range-bearing call STILL produces the exact cell list for a span
// covering the whole set — the whole-file short-circuit applies only when no
// range is active.
func TestCellConcatList_ChapterRangeWholeTitle(t *testing.T) {
	dir, set := writeTestVOBSet(t, []uint32{100, 200})
	ti := &ifo.TitleInfo{
		Cells: []ifo.TitleCell{
			{VOBID: 1, CellID: 1, FirstSector: 0, LastSector: 99},
			{VOBID: 2, CellID: 1, FirstSector: 0, LastSector: 199},
		},
		ProgramEntryCells: []int{1, 2},
	}
	list, cleanup, err := cellConcatList(dir, set, ti, 1, 2)
	if err != nil {
		t.Fatalf("cellConcatList(range 1-2): %v", err)
	}
	if list == "" || cleanup == nil {
		t.Fatal("an active range covering the whole set must still slice (not whole-file)")
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
}