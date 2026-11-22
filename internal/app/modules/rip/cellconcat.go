package rip

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/LeakTechnologies/VideoTools/internal/dvd/ifo"
	"github.com/LeakTechnologies/VideoTools/internal/logging"
	"github.com/LeakTechnologies/VideoTools/internal/utils"
)

// vobRange is a contiguous sector range within a single physical VOB file.
type vobRange struct {
	path  string
	first uint32
	last  uint32
}

// vobSegment is one physical file in the ordered VTS title stream. first is
// its first global title-domain sector, not a file-relative sector.
type vobSegment struct {
	path    string
	first   uint64
	sectors uint64
}

// sectorRange is a global title-domain sector range. DVD title cells address
// the concatenated VTS title stream, not an individual VOB file.
type sectorRange struct {
	first uint64
	last  uint64
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

// vobSetSegments returns the physical VOB files in title-stream order and
// assigns each one its global starting sector. A gap in the VTS_XX_N sequence
// is an error: exact cell slicing cannot skip a missing segment.
func vobSetSegments(videoTSPath string, set VobSet) ([]vobSegment, error) {
	byID := vobSetFileByID(videoTSPath, set)
	if len(byID) == 0 {
		return nil, fmt.Errorf("title VOB set %s has no content VOBs", set.Name)
	}
	ids := make([]int, 0, len(byID))
	for id := range byID {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)

	segments := make([]vobSegment, 0, len(ids))
	var first uint64
	for i, id := range ids {
		if id != i+1 {
			return nil, fmt.Errorf("title VOB stream %s has a missing segment before %s_%d.VOB", set.Name, set.Name, id)
		}
		path := byID[uint16(id)]
		sectors := uint64(vobSectorCount(path))
		if sectors == 0 {
			return nil, fmt.Errorf("title VOB segment %s is unavailable or not sector-aligned", filepath.Base(path))
		}
		segments = append(segments, vobSegment{path: path, first: first, sectors: sectors})
		first += sectors
	}
	return segments, nil
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

// coalesceTitleSectors validates cells and merges contiguous or overlapping
// global sector ranges. It deliberately rejects out-of-order cells instead of
// guessing at playback order.
func coalesceTitleSectors(cells []ifo.TitleCell, total uint64) ([]sectorRange, error) {
	ranges := make([]sectorRange, 0, len(cells))
	for _, cell := range cells {
		if cell.FirstSector > cell.LastSector {
			return nil, fmt.Errorf("title cell %d/%d has inverted sector range %d..%d", cell.VOBID, cell.CellID, cell.FirstSector, cell.LastSector)
		}
		first, last := uint64(cell.FirstSector), uint64(cell.LastSector)
		if last >= total {
			return nil, fmt.Errorf("title cell %d/%d sector %d is outside the %d-sector VOB stream", cell.VOBID, cell.CellID, last, total)
		}
		if len(ranges) > 0 {
			previous := &ranges[len(ranges)-1]
			if first < previous.first {
				return nil, fmt.Errorf("title cell %d/%d starts before the previous cell in the global sector stream", cell.VOBID, cell.CellID)
			}
			if first <= previous.last+1 {
				if last > previous.last {
					previous.last = last
				}
				continue
			}
		}
		ranges = append(ranges, sectorRange{first: first, last: last})
	}
	return ranges, nil
}

// mapSectorRange maps one global title-domain range onto the physical VOB
// files. A range that spans files produces one range per file; a range that
// does not cover its full length is an error.
func mapSectorRange(segments []vobSegment, r sectorRange) ([]vobRange, error) {
	if r.first > r.last {
		return nil, fmt.Errorf("invalid global sector range %d..%d", r.first, r.last)
	}
	var ranges []vobRange
	var covered uint64
	for _, segment := range segments {
		segmentLast := segment.first + segment.sectors - 1
		if r.last < segment.first || r.first > segmentLast {
			continue
		}
		start := max(r.first, segment.first)
		end := min(r.last, segmentLast)
		ranges = append(ranges, vobRange{
			path:  segment.path,
			first: uint32(start - segment.first),
			last:  uint32(end - segment.first),
		})
		covered += end - start + 1
	}
	if want := r.last - r.first + 1; covered != want {
		return nil, fmt.Errorf("global sector range %d..%d spans missing VOB data", r.first, r.last)
	}
	return ranges, nil
}

// cellConcatList builds an ffmpeg concat list that covers exactly the title's
// PGC cells, by slicing the set's VOB files to the per-cell global sector byte
// ranges (each cell's FirstSector..LastSector addresses the concatenated VTS
// title stream, so byte offsets are sector*2048 after mapping through the
// ordered physical segments). Returns:
//
//	list, cleanup, nil -- a cell-accurate list to use instead of whole-file concat;
//	cleanup removes the temp slice files and the list file when the rip ends.
//	"", nil, nil        -- cell slicing is not applicable/needed; use whole-file concat
//	                    (no cells, HasAngles, or cells already cover the whole VOB set with
//	                    NO chapter range active -- i.e. the content is exactly the file set).
//	"", nil, err        -- the selected title needs exact cell mapping but the
//	                    metadata or physical VOB stream cannot supply it. The
//	                    caller must fail rather than silently substitute a
//	                    whole-VTS rip.
//
// cs/ce are the 1-based inclusive chapter range to restrict the cells to; 0
// means the whole title.
func cellConcatList(videoTSPath string, set VobSet, ti *ifo.TitleInfo, cs, ce int) (string, func(), error) {
	if ti == nil || len(ti.Cells) == 0 || ti.HasAngles {
		return "", nil, nil
	}
	segments, err := vobSetSegments(videoTSPath, set)
	if err != nil {
		return "", nil, err
	}
	var total uint64
	for _, segment := range segments {
		total += segment.sectors
	}

	// A chapter range restricts the cell span via the PGC program map. Without
	// a resolvable span the caller keeps the whole-file list and bounds the
	// rip with output-side -ss/-to instead.
	cells := ti.Cells
	rangeActive := cs > 0
	if rangeActive {
		lo, hiEx, ok := chapterCellSpan(ti, cs, ce)
		if !ok {
			return "", nil, fmt.Errorf("chapter range %d-%d has no resolvable cell span", cs, ce)
		}
		cells = ti.Cells[lo:hiEx]
	}

	globalRanges, err := coalesceTitleSectors(cells, total)
	if err != nil {
		return "", nil, err
	}
	if len(globalRanges) == 0 {
		return "", nil, nil
	}

	// When the cells already span the entire VOB set, whole-file concat produces
	// identical content -- slicing would only add copy time and temp space.
	// A chapter range that resolves to the full span still goes through the
	// slice path (the span is exact and -ss/-to get suppressed).
	fullCover := len(globalRanges) == 1 && globalRanges[0].first == 0 && globalRanges[0].last == total-1
	if fullCover && !rangeActive {
		return "", nil, nil
	}

	var ranges []vobRange
	for _, global := range globalRanges {
		mapped, err := mapSectorRange(segments, global)
		if err != nil {
			return "", nil, err
		}
		ranges = append(ranges, mapped...)
	}

	// Slice each range out of its VOB into a temp file. Sector-aligned slicing
	// keeps full MPEG-2 packs intact, so the mpeg demuxer on the concat side
	// reads clean packets with their original timestamps.
	var slicePaths []string
	for _, r := range ranges {
		sf, err := os.CreateTemp(utils.TempDir(), "vt-cell-*.vob")
		if err != nil {
			break
		}
		slicePath := sf.Name()
		rf, err := os.Open(r.path)
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
	// Tear down partial slices on any failure -- keep the byte-identical
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
		"Cell-accurate VOB concat: %d global cell range(s) -> %d VOB slice(s) for title PGC",
		len(globalRanges), len(slicePaths))
	return listFile, cleanup, nil
}
