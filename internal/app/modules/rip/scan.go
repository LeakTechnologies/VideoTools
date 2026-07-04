package rip

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeakTechnologies/VideoTools/internal/dvd/ifo"
	"github.com/LeakTechnologies/VideoTools/internal/dvd/udf"
	"github.com/LeakTechnologies/VideoTools/internal/logging"
)

// classifyDiscType returns a human-readable disc type string based on total
// VIDEO_TS size in bytes, or "" when the path isn't available (ISO/BLURAY).
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

// classifyDiscRegion reads the VMG_Category from the VMG_MAT and returns a
// human-readable region string, or "" when it cannot be determined.
func classifyDiscRegion(category uint32) string {
	regionMask := byte(category & 0xFF)
	// All regions set or none set → region-free.
	if regionMask == 0 || regionMask == 0xFF {
		return "Region Free"
	}
	// Bit 0 = region 1, bit 1 = region 2, etc.
	for i := 0; i < 8; i++ {
		if regionMask == (1 << i) {
			return fmt.Sprintf("Region %d", i+1)
		}
	}
	// Multiple regions flagged → list them.
	var regions []string
	for i := 0; i < 8; i++ {
		if regionMask&(1<<i) != 0 {
			regions = append(regions, fmt.Sprintf("%d", i+1))
		}
	}
	if len(regions) > 0 {
		return "Regions " + strings.Join(regions, ", ")
	}
	return ""
}

// totalVideoTSSize sums all file sizes in the VIDEO_TS directory.
func totalVideoTSSize(videoTSPath string) int64 {
	var total int64
	filepath.Walk(videoTSPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		total += fi.Size()
		return nil
	})
	return total
}

// ScanDisc reads the VMG IFO and per-VTS IFOs to populate a DiscScanResult.
// It is safe to call from a goroutine; it performs no UI work.
func ScanDisc(videoTSPath string) (*DiscScanResult, error) {
	vmgPath := filepath.Join(videoTSPath, "VIDEO_TS.IFO")
	tsps, err := ifo.ReadTitleList(vmgPath)
	if err != nil {
		return nil, fmt.Errorf("read title list: %w", err)
	}

	// Read VMG_MAT for region info.
	vmgFile, err := os.Open(vmgPath)
	var region string
	if err == nil {
		if mat, rErr := ifo.ReadVMGI(vmgFile); rErr == nil {
			region = classifyDiscRegion(mat.VMG_Category)
		}
		vmgFile.Close()
	} else {
		logging.Warning(logging.CatDVD, "ScanDisc: failed to open VMG IFO for region: %v", err)
	}

	// Calculate total disc size and classify.
	discSize := totalVideoTSSize(videoTSPath)
	discType := classifyDiscType(discSize)

	// Cache per-VTS IFO reads — multiple titles can share a VTS.
	type vtsKey = int
	vtsCache := map[vtsKey]*ifo.TitleInfo{}

	result := &DiscScanResult{
		DiscType:  discType,
		TotalSize: discSize,
		Region:    region,
	}
	for i, t := range tsps {
		dt := DiscTitle{
			Number:      i + 1,
			VTSNumber:   int(t.VTSNumber),
			NumChapters: int(t.NumChapters),
		}

		vtsNum := int(t.VTSNumber)
		ti, cached := vtsCache[vtsNum]
		if !cached {
			vtsIFO := filepath.Join(videoTSPath, fmt.Sprintf("VTS_%02d_0.IFO", vtsNum))
			if info, err := ifo.ReadTitleInfo(vtsIFO); err == nil {
				ti = info
			} else {
				logging.Warning(logging.CatDVD, "ScanDisc: VTS_%02d IFO read failed: %v", vtsNum, err)
			}
			vtsCache[vtsNum] = ti
		}

		if ti != nil {
			dt.Duration = ti.Duration
			dt.HasAngles = ti.HasAngles
			if len(ti.Chapters) > 1 {
				dt.NumChapters = len(ti.Chapters)
			}
			for _, a := range ti.Audio {
				dt.Audio = append(dt.Audio, DiscTitleTrack{
					Language: a.Language,
					Codec:    a.Codec,
					Channels: a.Channels,
				})
			}
			for _, s := range ti.Subtitles {
				dt.Subtitles = append(dt.Subtitles, DiscTitleTrack{
					Language: s.Language,
					Codec:    s.Codec,
				})
			}
		}
		result.Titles = append(result.Titles, dt)
	}
	logging.Info(logging.CatDVD, "ScanDisc: %d titles in VIDEO_TS, type=%s, size=%d, region=%s",
		len(result.Titles), result.DiscType, result.TotalSize, result.Region)
	return result, nil
}

// FormatDuration formats seconds as "Xh Ym" or "Ym Zs".
func FormatDuration(seconds float64) string {
	total := int(seconds)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%dh %02dm", h, m)
	}
	return fmt.Sprintf("%dm %02ds", m, s)
}

// scanISOViaUDF extracts IFO files from a DVD ISO image using the UDF reader,
// runs ScanDisc on the extracted data, and returns a full DiscScanResult.
// Disc size and type are taken from the ISO file itself (not from the temp dir).
func scanISOViaUDF(isoPath string) (*DiscScanResult, error) {
	fi, err := os.Stat(isoPath)
	if err != nil {
		return nil, fmt.Errorf("stat ISO: %w", err)
	}
	isoSize := fi.Size()
	discType := classifyDiscType(isoSize)

	udfType, _ := udf.IdentifyDiscFormat(isoPath)
	if udfType == udf.DiscTypeBluRay {
		discType = "BD"
	}

	f, err := os.Open(isoPath)
	if err != nil {
		return nil, fmt.Errorf("open ISO: %w", err)
	}
	defer f.Close()

	udfReader := udf.NewReader(f)

	vmgData, err := udfReader.ReadFileData("VIDEO_TS/VIDEO_TS.IFO")
	if err != nil {
		return nil, fmt.Errorf("read VMG IFO from ISO: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "vt_isoscan_*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	vtsTempDir := filepath.Join(tmpDir, "VIDEO_TS")
	if err := os.MkdirAll(vtsTempDir, 0755); err != nil {
		return nil, fmt.Errorf("create temp VIDEO_TS dir: %w", err)
	}

	vmgTmpPath := filepath.Join(vtsTempDir, "VIDEO_TS.IFO")
	if err := os.WriteFile(vmgTmpPath, vmgData, 0644); err != nil {
		return nil, fmt.Errorf("write temp VMG IFO: %w", err)
	}

	tsps, err := ifo.ReadTitleList(vmgTmpPath)
	if err != nil {
		return nil, fmt.Errorf("read title list: %w", err)
	}

	vtsSet := map[int]bool{}
	for _, t := range tsps {
		vtsSet[int(t.VTSNumber)] = true
	}

	for vtsNum := range vtsSet {
		ifoName := fmt.Sprintf("VTS_%02d_0.IFO", vtsNum)
		ifoData, readErr := udfReader.ReadFileData("VIDEO_TS/" + ifoName)
		if readErr != nil {
			logging.Warning(logging.CatDVD, "scanISOViaUDF: failed to read %s: %v", ifoName, readErr)
			continue
		}
		if writeErr := os.WriteFile(filepath.Join(vtsTempDir, ifoName), ifoData, 0644); writeErr != nil {
			logging.Warning(logging.CatDVD, "scanISOViaUDF: failed to write %s: %v", ifoName, writeErr)
		}
	}

	result, scanErr := ScanDisc(vtsTempDir)
	if scanErr != nil {
		logging.Warning(logging.CatDVD, "scanISOViaUDF: ScanDisc failed: %v", scanErr)
		var region string
		if mat, matErr := ifo.ReadVMGI(bytes.NewReader(vmgData)); matErr == nil {
			region = classifyDiscRegion(mat.VMG_Category)
		}
		return &DiscScanResult{
			DiscType:  discType,
			TotalSize: isoSize,
			Region:    region,
		}, nil
	}

	result.DiscType = discType
	result.TotalSize = isoSize
	return result, nil
}

// langList returns a comma-separated list of unique uppercase language codes.
func langList(tracks []DiscTitleTrack) string {
	seen := map[string]bool{}
	var parts []string
	for _, t := range tracks {
		if t.Language != "" && !seen[t.Language] {
			seen[t.Language] = true
			parts = append(parts, strings.ToUpper(t.Language))
		}
	}
	return strings.Join(parts, ", ")
}
