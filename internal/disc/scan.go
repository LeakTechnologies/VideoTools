package disc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeakTechnologies/VideoTools/internal/dvd/ifo"
)

// scanVideoTS reads the VMG IFO and each VTS IFO. It never touches VOB bytes.
func scanVideoTS(videoTS string, kind SourceKind, sourcePath string, opts ScanOptions) (*ScanResult, error) {
	vmgPath := filepath.Join(videoTS, "VIDEO_TS.IFO")
	tsps, err := ifo.ReadTitleList(vmgPath)
	if err != nil {
		return nil, serviceError("read_title_list", "could not read VIDEO_TS.IFO title list", err)
	}

	regionCode, region := readRegion(vmgPath)
	totalSize := totalVideoTSSize(videoTS)
	discType := classifyDiscType(totalSize)

	result := &ScanResult{
		SourceKind: kind,
		SourcePath: sourcePath,
		DiscType:   discType,
		TotalSize:  totalSize,
		Region:     region,
		RegionCode: regionCode,
		Titles:     make([]Title, 0, len(tsps)),
	}

	identity := make([]string, 0, 3)
	if region != "" {
		identity = append(identity, region)
	}
	if discType != "" {
		identity = append(identity, discType)
	}
	titleWord := "titles"
	if len(tsps) == 1 {
		titleWord = "title"
	}
	identity = append(identity, fmt.Sprintf("%d %s", len(tsps), titleWord))
	opts.note(strings.Join(identity, " · "))

	pgcCache := map[string]*ifo.TitleInfo{}
	var firstInfo *ifo.TitleInfo
	for i, t := range tsps {
		title := Title{
			Number:      i + 1,
			VTSNumber:   int(t.VTSNumber),
			TTN:         int(t.VTS_TitleNumber),
			NumChapters: int(t.NumChapters),
		}
		key := fmt.Sprintf("%d:%d", t.VTSNumber, t.VTS_TitleNumber)
		info, cached := pgcCache[key]
		if !cached {
			vtsIFO := filepath.Join(videoTS, fmt.Sprintf("VTS_%02d_0.IFO", t.VTSNumber))
			var readErr error
			info, readErr = ifo.ReadTitleInfoForTTN(vtsIFO, int(t.VTS_TitleNumber))
			if readErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("VTS_%02d_0.IFO title %d: %v", t.VTSNumber, t.VTS_TitleNumber, readErr))
			}
			pgcCache[key] = info
		}
		if i == 0 && info != nil {
			firstInfo = info
		}
		if info != nil {
			title.Duration = info.Duration
			title.HasAngles = info.HasAngles
			title.Interlaced = info.Interlaced
			if len(info.Chapters) > 1 {
				title.NumChapters = len(info.Chapters)
				title.Chapters = append([]float64(nil), info.Chapters...)
			}
			for _, track := range info.Audio {
				title.Audio = append(title.Audio, Track{
					Index:    track.Index,
					Language: track.Language,
					Codec:    track.Codec,
					Channels: track.Channels,
				})
			}
			for _, track := range info.Subtitles {
				title.Subtitles = append(title.Subtitles, Track{
					Index:    track.Index,
					Language: track.Language,
					Codec:    track.Codec,
					Channels: track.Channels,
				})
			}
			for _, cell := range info.Cells {
				title.Cells = append(title.Cells, Cell{
					VOBID:       cell.VOBID,
					CellID:      cell.CellID,
					FirstSector: cell.FirstSector,
					LastSector:  cell.LastSector,
				})
			}
		}
		result.Titles = append(result.Titles, title)
		opts.note(titleNote(title))
	}

	if firstInfo != nil {
		result.VideoStandard = "PAL"
		if firstInfo.IsNTSC {
			result.VideoStandard = "NTSC"
		}
		opts.note("Video: " + result.VideoStandard)
	}
	result.MainTitleNumber = mainTitleNumber(result.Titles)
	return result, nil
}

func titleNote(title Title) string {
	duration := fmt.Sprintf("%.0fs", title.Duration)
	if title.Duration >= 60 {
		duration = fmt.Sprintf("%dm%02ds", int(title.Duration)/60, int(title.Duration)%60)
	}
	chapterWord := "chapters"
	if title.NumChapters == 1 {
		chapterWord = "chapter"
	}
	return fmt.Sprintf("T%02d  %s  %d %s", title.Number, duration, title.NumChapters, chapterWord)
}

func mainTitleNumber(titles []Title) int {
	main := 0
	longest := -1.0
	for _, title := range titles {
		if title.Duration > longest {
			main = title.Number
			longest = title.Duration
		}
	}
	return main
}

func totalVideoTSSize(videoTS string) int64 {
	var total int64
	_ = filepath.Walk(videoTS, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

func classifyDiscType(totalBytes int64) string {
	switch {
	case totalBytes < 0:
		return ""
	case totalBytes < 500_000_000:
		return "MiniDVD"
	case totalBytes < 4_500_000_000:
		return "DVD-5"
	case totalBytes < 8_500_000_000:
		return "DVD-9"
	case totalBytes < 9_000_000_000:
		return "DVD-10"
	case totalBytes < 15_000_000_000:
		return "DVD-18"
	default:
		return ""
	}
}

// readRegion returns a display label and a compact code:
// 0 = region-free, 1..8 = single region, -1 = multi-region/unknown.
func readRegion(vmgPath string) (int, string) {
	f, err := os.Open(vmgPath)
	if err != nil {
		return -1, ""
	}
	defer f.Close()
	mat, err := ifo.ReadVMGI(f)
	if err != nil {
		return -1, ""
	}
	mask := byte(mat.VMG_Category & 0xFF)
	if mask == 0 || mask == 0xFF {
		return 0, "Region Free"
	}
	var regions []int
	for i := 0; i < 8; i++ {
		if mask&(1<<i) != 0 {
			regions = append(regions, i+1)
		}
	}
	if len(regions) == 1 {
		return regions[0], fmt.Sprintf("Region %d", regions[0])
	}
	if len(regions) == 0 {
		return -1, ""
	}
	parts := make([]string, len(regions))
	for i, r := range regions {
		parts[i] = fmt.Sprintf("%d", r)
	}
	return -1, "Regions " + strings.Join(parts, ", ")
}
