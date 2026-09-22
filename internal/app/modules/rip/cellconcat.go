package rip

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/LeakTechnologies/VideoTools/internal/dvd/ifo"
	"github.com/LeakTechnologies/VideoTools/internal/logging"
	"github.com/LeakTechnologies/VideoTools/internal/utils"
)

// vobRange is a contiguous sector range within a single VOB file.
type vobRange struct {
	vobID uint8
	first uint32
	last  uint32
}

// vobSectorCount returns the number of 2048-byte sectors in a VOB file, or 0
// when the file cannot be stat'ed or has a non-sector-aligned length.
func vobSectorCount(path string) uint32 {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() || fi.Size() <= 0 {
		return 0
	}
	return uint32(fi.Size() / 2048)
}

// vobSetFileByID maps a set's VOB files by their VOB id (the trailing number of
// VTS_XX_N.VOB). Returns nil when the set name isn't a VTS_XX pattern or a VOB
// file cannot be numbered.
func vobSetFileByID(videoTSPath string, set VobSet) map[uint16]string {
	key := strings.ToUpper(set.Name)
	if !strings.HasPrefix(key, "VTS_") {
		return nil
	}
	entries, err := os.ReadDir(videoTSPath)
	if err != nil {
		return nil
	}
	byID := map[uint16]string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(name), ".vob") {
			continue
		}
		upper := strings.ToUpper(strings.TrimSuffix(name, ".VOB"))
		if !strings.HasPrefix(upper, key+"_") {
			continue
		}
		n, err := strconv.Atoi(upper[len(key)+1:])
		if err != nil || n <= 0 {
			continue
		}
		byID[uint16(n)] = filepath.Join(videoTSPath, name)
	}
	return byID
}

// chapterCellSpan maps a 1-based inclusive chapter range (cs..ce, 1-based
// program numbers) onto the cell-index span [lo, hiEx) over ti.Cells using the
// PGC program map (ProgramEntryCells[p] = entry cell number of program p).
// The span covers every cell from chapter cs's entry cell up to the cell
// before the entry cell of the program after ce (or the last cell when ce is
// the final chapter). ok=false when the program map or cell list is missing or
// the range is ill-formed.
func chapterCellSpan(ti *ifo.TitleInfo, cs, ce int) (lo, hiEx int, ok bool) {
	prog := ti.ProgramEntryCells
	n := len(prog)
	if n == 0 || len(ti.Cells) == 0 {
		return 0, 0, false
	}
	if cs < 1 {
		cs = 1
	}
	if ce == 0 || ce > n {
		ce = n
	}
	if cs > n || ce < cs {
		return 0, 0, false
	}
	lo = int(prog[cs-1]) - 1
	if lo >= len(ti.Cells) {
		return 0, 0, false
	}
	if ce < n {
		hiEx = int(prog[ce]) - 1
	} else {
		hiEx = len(ti.Cells)
	}
	if hiEx <= lo {
		// Programs sharing an entry cell can collapse the span; keep at least
		// the entry cell so the range still slices something exact.
		hiEx = lo + 1
	}
	if hiEx > len(ti.Cells) {
		hiEx = len(ti.Cells)
	}
	return lo, hiEx, true
}

// cellConcatList builds an ffmpeg concat list that covers exactly the title's
// PGC cells, by slicing the set's VOB files to the per-cell sector byte ranges
// (each cell's FirstSector..LastSector are VOB-relative, so byte offsets are
// sector*2048). Returns:
//
//	list, cleanup, nil — a cell-accurate list to use instead of whole-file concat;
//	cleanup removes the temp slice files and the list file when the rip ends.
//	"", nil, nil        — cell slicing is not applicable/needed; use whole-file concat
//	                    (no cells, HasAngles, a chapter range with no resolvable
//	                    cell span, or cells already cover the whole VOB set with
//	                    NO chapter range active — i.e. the content is exactly the
//	                    file set).
//	"", nil, err        — the cell list could not be prepared (caller should warn
//	                    and keep the whole-file fallback).
//
// cs/ce are the 1-based inclusive chapter range to restrict the cells to; 0
// means the whole title.
func cellConcatList(videoTSPath string, set VobSet, ti *ifo.TitleInfo, cs, ce int) (string, func(), error) {
	if ti == nil || len(ti.Cells) == 0 || ti.HasAngles {
		return "", nil, nil
	}
	byID := vobSetFileByID(videoTSPath, set)
	if byID == nil {
		return "", nil, nil
	}

	// A chapter range restricts the cell span via the PGC program map. Without
	// a resolvable span the caller keeps the whole-file list and bounds the
	// rip with output-side -ss/-to instead.
	cells := ti.Cells
	rangeActive := cs > 0
	if rangeActive {
		lo, hiEx, ok := chapterCellSpan(ti, cs, ce)
		if !ok {
			return "", nil, nil
		}
		cells = ti.Cells[lo:hiEx]
	}

	// Coalesce cells that are adjacent (or overlapping) within the same VOB
	// into the minimal set of contiguous byte ranges, preserving playback order.
	var ranges []vobRange
	for _, c := range cells {
		vid := uint8(c.VOBID)
		if vid == 0 {
			return "", nil, nil
		}
		if len(ranges) > 0 && ranges[len(ranges)-1].vobID == vid && c.FirstSector <= ranges[len(ranges)-1].last+1 {
			if c.LastSector > ranges[len(ranges)-1].last {
				ranges[len(ranges)-1].last = c.LastSector
			}
			continue
		}
		ranges = append(ranges, vobRange{vobID: vid, first: c.FirstSector, last: c.LastSector})
	}

	// Validate every range against its VOB file and clamp to the file's physical
	// sector count. A malformed/unresolvable range means the PGC metadata cannot
	// be trusted for slicing — fall back to whole-file concat.
	fullCover := true
	for i := range ranges {
		path, ok := byID[uint16(ranges[i].vobID)]
		if !ok {
			return "", nil, nil
		}
		if ranges[i].first > ranges[i].last {
			return "", nil, nil
		}
		sectorCount := vobSectorCount(path)
		if sectorCount == 0 {
			return "", nil, nil
		}
		if ranges[i].last > sectorCount-1 {
			ranges[i].last = sectorCount - 1
		}
		if ranges[i].first != 0 || ranges[i].last != sectorCount-1 {
			fullCover = false
		}
	}

	// When the cells already span the entire VOB set, whole-file concat produces
	// identical content — slicing would only add copy time and temp space.
	// A chapter range that resolves to the full span still goes through the
	// slice path (the span is exact and -ss/-to get suppressed).
	if fullCover && !rangeActive {
		return "", nil, nil
	}

	// Slice each range out of its VOB into a temp file. Sector-aligned slicing
	// keeps full MPEG-2 packs intact, so the mpeg demuxer on the concat side
	// reads clean packets with their original timestamps.
	var slicePaths []string
	for _, r := range ranges {
		src, ok := byID[uint16(r.vobID)]
		if !ok {
			break
		}
		sf, err := os.CreateTemp(utils.TempDir(), "vt-cell-*.vob")
		if err != nil {
			break
		}
		slicePath := sf.Name()
		rf, err := os.Open(src)
		if err != nil {
			_ = sf.Close()
			break
		}
		startOff := int64(r.first) * 2048
		length := (int64(r.last) - int64(r.first) + 1) * 2048
		if _, err = rf.Seek(startOff, io.SeekStart); err != nil {
			_ = rf.Close()
			_ = sf.Close()
			break
		}
		if _, err = io.CopyN(sf, rf, length); err != nil {
			_ = rf.Close()
			_ = sf.Close()
			break
		}
		_ = rf.Close()
		if err = sf.Close(); err != nil {
			break
		}
		slicePaths = append(slicePaths, slicePath)
	}
	// Tear down partial slices on any failure — keep the byte-identical
	// whole-file concat as the fallback rather than risking a truncated rip.
	if len(slicePaths) != len(ranges) {
		for _, p := range slicePaths {
			_ = os.Remove(p)
		}
		return "", nil, fmt.Errorf("cell slicing incomplete (%d/%d ranges)", len(slicePaths), len(ranges))
	}

	listFile, err := BuildConcatList(slicePaths)
	if err != nil {
		for _, p := range slicePaths {
			_ = os.Remove(p)
		}
		return "", nil, fmt.Errorf("build cell concat list: %w", err)
	}

	cleanup := func() {
		_ = os.Remove(listFile)
		for _, p := range slicePaths {
			_ = os.Remove(p)
		}
	}
	logging.Info(logging.CatDVD,
		"Cell-accurate VOB concat: %d cell range(s) → %d VOB slice(s) for title PGC",
		len(ranges), len(slicePaths))
	return listFile, cleanup, nil
}