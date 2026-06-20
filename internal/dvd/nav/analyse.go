package nav

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeakTechnologies/VideoTools/internal/dvd/ifo"
	"github.com/LeakTechnologies/VideoTools/internal/dvd/udf"
	"github.com/LeakTechnologies/VideoTools/internal/logging"
)

// DiscTopology is the complete structural analysis of a DVD disc, extracted
// from IFO files without reading the VOB data. It captures enough information
// for both playback navigation and authoring reverse-engineering: reconstructing
// the disc's navigation graph, chapter structure, and track metadata.
type DiscTopology struct {
	DiscType  string // "DVD-5", "DVD-9", "MiniDVD", "BD", or ""
	Region    string // "Region 1", "Region Free", "Regions 1, 4", etc.
	TotalSize int64  // disc size in bytes (ISO file size or VIDEO_TS directory sum)
	Titles    []TitleNode
}

// TitleNode describes a single playback title on the disc.
type TitleNode struct {
	Number     int       // 1-based title number from TT_SRPT
	VTSNumber  int       // which VTS (Video Title Set) hosts this title
	Duration   float64   // total playback time in seconds
	Chapters   []float64 // chapter start times in seconds (Chapters[0] == 0.0)
	Audio      []TrackNode
	Subtitles  []TrackNode
	HasAngles  bool
	Interlaced bool // true = video-originated (camera); false = film/progressive
}

// TrackNode describes one audio or subtitle stream.
type TrackNode struct {
	Index    int    // 0-based IFO stream index
	Language string // ISO 639-1 two-letter code, or ""
	Codec    string // e.g. "ac3", "dts", "lpcm", "dvd_subtitle"
	Channels int    // channel count; 0 for subtitle tracks
}

// AnalyseDisc performs a full structural analysis of a DVD disc.
// path may be an ISO image file or a directory that contains VIDEO_TS.
// The returned topology is suitable for both playback navigation and
// authoring reverse-engineering (disc topology, chapter map, track inventory).
func AnalyseDisc(path string) (*DiscTopology, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("AnalyseDisc: stat %s: %w", path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	if !fi.IsDir() && ext == ".iso" {
		return analyseISO(path)
	}

	// Treat as directory — descend into VIDEO_TS if needed.
	videoTSPath := path
	if !strings.EqualFold(filepath.Base(path), "VIDEO_TS") {
		candidate := filepath.Join(path, "VIDEO_TS")
		if _, err := os.Stat(candidate); err == nil {
			videoTSPath = candidate
		}
	}
	return analyseVideoTS(videoTSPath, -1, "")
}

// analyseVideoTS reads IFO files from a VIDEO_TS directory and builds a DiscTopology.
// If discSize is negative the directory is walked to compute the size.
// If discType is "" the size-based classifier is applied.
func analyseVideoTS(videoTSPath string, discSize int64, discType string) (*DiscTopology, error) {
	vmgPath := filepath.Join(videoTSPath, "VIDEO_TS.IFO")

	tsps, err := ifo.ReadTitleList(vmgPath)
	if err != nil {
		return nil, fmt.Errorf("analyseVideoTS: %w", err)
	}

	var region string
	if f, err := os.Open(vmgPath); err == nil {
		if mat, rErr := ifo.ReadVMGI(f); rErr == nil {
			region = classifyRegion(mat.VMG_Category)
		}
		f.Close()
	}

	if discSize < 0 {
		discSize = sumDirSize(videoTSPath)
	}
	if discType == "" {
		discType = classifyDiscType(discSize)
	}

	vtsCache := map[int]*ifo.TitleInfo{}

	topo := &DiscTopology{
		DiscType:  discType,
		Region:    region,
		TotalSize: discSize,
	}

	for i, t := range tsps {
		node := TitleNode{
			Number:    i + 1,
			VTSNumber: int(t.VTSNumber),
		}

		vtsNum := int(t.VTSNumber)
		ti, cached := vtsCache[vtsNum]
		if !cached {
			vtsIFO := filepath.Join(videoTSPath, fmt.Sprintf("VTS_%02d_0.IFO", vtsNum))
			if info, err := ifo.ReadTitleInfo(vtsIFO); err == nil {
				ti = info
			} else {
				logging.Warning(logging.CatDVD, "AnalyseDisc: VTS_%02d IFO: %v", vtsNum, err)
			}
			vtsCache[vtsNum] = ti
		}

		if ti != nil {
			node.Duration   = ti.Duration
			node.Chapters   = ti.Chapters
			node.HasAngles  = ti.HasAngles
			node.Interlaced = ti.Interlaced
			for _, a := range ti.Audio {
				node.Audio = append(node.Audio, TrackNode{
					Index:    a.Index,
					Language: a.Language,
					Codec:    a.Codec,
					Channels: a.Channels,
				})
			}
			for _, s := range ti.Subtitles {
				node.Subtitles = append(node.Subtitles, TrackNode{
					Index:    s.Index,
					Language: s.Language,
					Codec:    s.Codec,
				})
			}
		}
		topo.Titles = append(topo.Titles, node)
	}

	logging.Info(logging.CatDVD, "AnalyseDisc: %d title(s), type=%s, region=%s",
		len(topo.Titles), topo.DiscType, topo.Region)
	return topo, nil
}

// analyseISO extracts IFO files from a DVD ISO via the UDF reader then analyses them.
func analyseISO(isoPath string) (*DiscTopology, error) {
	fi, err := os.Stat(isoPath)
	if err != nil {
		return nil, fmt.Errorf("analyseISO: %w", err)
	}
	isoSize := fi.Size()
	discType := classifyDiscType(isoSize)

	udfType, _ := udf.IdentifyDiscFormat(isoPath)
	if udfType == udf.DiscTypeBluRay {
		discType = "BD"
	}

	f, err := os.Open(isoPath)
	if err != nil {
		return nil, fmt.Errorf("analyseISO: open: %w", err)
	}
	defer f.Close()

	r := udf.NewReader(f)

	vmgData, err := r.ReadFileData("VIDEO_TS/VIDEO_TS.IFO")
	if err != nil {
		return nil, fmt.Errorf("analyseISO: read VMG IFO: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "vt_nav_*")
	if err != nil {
		return nil, fmt.Errorf("analyseISO: temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	vtsTmp := filepath.Join(tmpDir, "VIDEO_TS")
	if err := os.MkdirAll(vtsTmp, 0755); err != nil {
		return nil, fmt.Errorf("analyseISO: mkdir temp: %w", err)
	}

	vmgTmp := filepath.Join(vtsTmp, "VIDEO_TS.IFO")
	if err := os.WriteFile(vmgTmp, vmgData, 0644); err != nil {
		return nil, fmt.Errorf("analyseISO: write VMG IFO: %w", err)
	}

	tsps, err := ifo.ReadTitleList(vmgTmp)
	if err != nil {
		return nil, fmt.Errorf("analyseISO: read title list: %w", err)
	}

	vtsSet := map[int]bool{}
	for _, t := range tsps {
		vtsSet[int(t.VTSNumber)] = true
	}
	for vtsNum := range vtsSet {
		name := fmt.Sprintf("VTS_%02d_0.IFO", vtsNum)
		data, rErr := r.ReadFileData("VIDEO_TS/" + name)
		if rErr != nil {
			logging.Warning(logging.CatDVD, "analyseISO: read %s: %v", name, rErr)
			continue
		}
		_ = os.WriteFile(filepath.Join(vtsTmp, name), data, 0644)
	}

	return analyseVideoTS(vtsTmp, isoSize, discType)
}

// classifyDiscType returns a human-readable disc type based on total byte count.
func classifyDiscType(n int64) string {
	switch {
	case n < 0:
		return ""
	case n < 500_000_000:
		return "MiniDVD"
	case n < 4_500_000_000:
		return "DVD-5"
	case n < 8_500_000_000:
		return "DVD-9"
	case n < 9_000_000_000:
		return "DVD-10"
	case n < 15_000_000_000:
		return "DVD-18"
	default:
		return ""
	}
}

// classifyRegion converts a VMG_Category region byte to a human-readable string.
func classifyRegion(category uint32) string {
	mask := byte(category & 0xFF)
	if mask == 0 || mask == 0xFF {
		return "Region Free"
	}
	for i := 0; i < 8; i++ {
		if mask == (1 << i) {
			return fmt.Sprintf("Region %d", i+1)
		}
	}
	var regions []string
	for i := 0; i < 8; i++ {
		if mask&(1<<i) != 0 {
			regions = append(regions, fmt.Sprintf("%d", i+1))
		}
	}
	if len(regions) > 0 {
		return "Regions " + strings.Join(regions, ", ")
	}
	return ""
}

// sumDirSize returns the total size of all files under dir.
func sumDirSize(dir string) int64 {
	var total int64
	filepath.Walk(dir, func(_ string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() {
			total += fi.Size()
		}
		return nil
	})
	return total
}
