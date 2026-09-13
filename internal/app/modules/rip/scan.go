package rip

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeakTechnologies/VideoTools/internal/dvd/ifo"
	"github.com/LeakTechnologies/VideoTools/internal/dvd/iso9660"
	"github.com/LeakTechnologies/VideoTools/internal/dvd/udf"
	"github.com/LeakTechnologies/VideoTools/internal/i18n"
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
	t := i18n.T()
	regionMask := byte(category & 0xFF)
	// All regions set or none set → region-free.
	if regionMask == 0 || regionMask == 0xFF {
		return t.RipRegionFree
	}
	// Bit 0 = region 1, bit 1 = region 2, etc.
	for i := 0; i < 8; i++ {
		if regionMask == (1 << i) {
			return fmt.Sprintf(t.RipRegionFmt, i+1)
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
		return fmt.Sprintf(t.RipRegionsFmt, strings.Join(regions, ", "))
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
// It is safe to call from a goroutine; it performs no UI work. When onNote is
// non-nil it is invoked as facts become known (disc identity, standard, and a
// per-title line) so the UI can show scan-as-you-go notes during the scan.
func ScanDisc(videoTSPath string, onNote func(string)) (*DiscScanResult, error) {
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

	// Scan-as-you-go: emit identity facts as soon as they are known, then one
	// line per title as it parses. onNote is invoked from the scan goroutine —
	// the caller marshals it to the UI thread.
	if onNote != nil {
		facts := make([]string, 0, 3)
		if region != "" {
			facts = append(facts, region)
		}
		if discType != "" {
			facts = append(facts, discType)
		}
		facts = append(facts, fmt.Sprintf("%d %s", len(tsps), i18n.T().RipTitleCount))
		onNote(strings.Join(facts, " · "))
	}

	// Cache per-(VTS, TTN) IFO reads — each PGC in a multi-PGC VTS reports its
	// own duration/chapters. Reading only the first title-domain PGC once per
	// VTS made every title in a scene-segmented VTS show that first PGC's play
	// time (e.g. every title of a 7-title disc reporting the full movie's "1h 38m").
	pgcCache := map[string]*ifo.TitleInfo{}
	var firstVTSInfo *ifo.TitleInfo

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
		ttn := int(t.VTS_TitleNumber)
		key := fmt.Sprintf("%d:%d", vtsNum, ttn)
		ti, cached := pgcCache[key]
		if !cached {
			vtsIFO := filepath.Join(videoTSPath, fmt.Sprintf("VTS_%02d_0.IFO", vtsNum))
			if info, err := ifo.ReadTitleInfoForTTN(vtsIFO, ttn); err == nil {
				ti = info
			} else {
				logging.Warning(logging.CatDVD, "ScanDisc: VTS_%02d TTN %d IFO read failed: %v", vtsNum, ttn, err)
			}
			pgcCache[key] = ti
		}
		if i == 0 && ti != nil {
			firstVTSInfo = ti
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
		if onNote != nil {
			onNote(titleSnippet(dt))
		}
	}

	// Determine video standard (NTSC/PAL) from the first title's PGC header.
	// The IsNTSC flag is set from the PGC frame-rate bits during ReadTitleInfo;
	// sample it from the first title's cached info after the loop.
	if len(tsps) > 0 && firstVTSInfo != nil {
		std := "PAL"
		if firstVTSInfo.IsNTSC {
			std = "NTSC"
		}
		result.VideoStandard = std
		if onNote != nil {
			onNote(fmt.Sprintf("Video: %s", std))
		}
	}

	logging.Info(logging.CatDVD, "ScanDisc: %d titles in VIDEO_TS, type=%s, size=%d, region=%s, std=%s",
		len(result.Titles), result.DiscType, result.TotalSize, result.Region, result.VideoStandard)
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

// shortScanError condenses a scan error into a short human-readable string for
// the disc summary card, trimming wrapper prefixes like "read title list:".
func shortScanError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if i := strings.LastIndex(msg, ": "); i >= 0 && i > 20 {
		msg = msg[i+2:]
	}
	if len(msg) > 200 {
		msg = msg[:200] + "…"
	}
	return msg
}

// scanISOViaUDF extracts IFO files from a DVD ISO image using the UDF reader,
// runs ScanDisc on the extracted data, and returns a full DiscScanResult.
// Disc size and type are taken from the ISO file itself (not from the temp dir).
// When the image has no usable UDF volume it falls back to the native ISO 9660
// reader so ISO 9660-only burns scan like any other disc.
func scanISOViaUDF(isoPath string, onNote func(string)) (*DiscScanResult, error) {
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
	readFile := udfReader.ReadFileData

	vmgData, err := readFile("VIDEO_TS/VIDEO_TS.IFO")
	if err != nil {
		udfVMGErr := err
		logging.Warning(logging.CatDVD, "scanISOViaUDF: UDF read of VIDEO_TS.IFO failed (%v); trying ISO 9660 reader", err)
		isoR := iso9660.NewReader(f)
		readFile = isoR.ReadFileData
		vmgData, err = readFile("VIDEO_TS/VIDEO_TS.IFO")
		if err != nil {
			return nil, fmt.Errorf("read VMG IFO from ISO (UDF: %v; ISO 9660: %v)", udfVMGErr, err)
		}
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
		ifoData, readErr := readFile("VIDEO_TS/" + ifoName)
		if readErr != nil {
			logging.Warning(logging.CatDVD, "scanISOViaUDF: failed to read %s: %v", ifoName, readErr)
			continue
		}
		if writeErr := os.WriteFile(filepath.Join(vtsTempDir, ifoName), ifoData, 0644); writeErr != nil {
			logging.Warning(logging.CatDVD, "scanISOViaUDF: failed to write %s: %v", ifoName, writeErr)
		}
	}

	result, scanErr := ScanDisc(vtsTempDir, onNote)
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

// runISOScan wraps scanISOViaUDF for the UI goroutine. A malformed UDF image or
// nil-deref in the reader would otherwise panic inside the background goroutine
// (killing the whole process) or return a silent non-result; this converts any
// panic into a normal error so the disc summary always settles to a visible
// state, and logs start/done so a stalled scan is diagnosable from the log.
func runISOScan(isoPath string, onNote func(string)) (result *DiscScanResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("ISO scan panicked: %v", r)
			logging.Error(logging.CatDVD, "runISOScan: scanISOViaUDF panicked for %s: %v", isoPath, r)
		}
	}()
	logging.Info(logging.CatDVD, "runISOScan: scanning ISO %s", isoPath)
	result, err = scanISOViaUDF(isoPath, onNote)
	if err != nil {
		logging.Warning(logging.CatDVD, "runISOScan: failed for %s: %v", isoPath, err)
		return nil, err
	}
	logging.Info(logging.CatDVD, "runISOScan: %s: %d titles, type=%s, size=%d, region=%s, std=%s",
		isoPath, len(result.Titles), result.DiscType, result.TotalSize, result.Region, result.VideoStandard)
	return result, nil
}

// langList returns a comma-separated list of unique uppercase language codes.
func langList(tracks []DiscTitleTrack) string {
	return strings.Join(uniqueSubtitleLangs(tracks), ", ")
}

// uniqueSubtitleLangs returns the distinct uppercase language codes in disc
// order, used to build the per-language subtitle checkboxes.
func uniqueSubtitleLangs(tracks []DiscTitleTrack) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range tracks {
		l := strings.ToUpper(t.Language)
		if l != "" && !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	return out
}

// titleSnippet renders one compact scan-as-you-go note for a single title:
// number, duration, chapter/audio/subtitle counts where known.
func titleSnippet(dt DiscTitle) string {
	parts := []string{fmt.Sprintf("T%02d", dt.Number)}
	if dt.Duration > 0 {
		parts = append(parts, FormatDuration(dt.Duration))
	}
	if dt.NumChapters > 1 {
		parts = append(parts, fmt.Sprintf("%d ch", dt.NumChapters))
	}
	if langs := langList(dt.Audio); langs != "" {
		parts = append(parts, fmt.Sprintf("%s audio", langs))
	}
	if langs := langList(dt.Subtitles); langs != "" {
		parts = append(parts, fmt.Sprintf("%s subs", langs))
	}
	return strings.Join(parts, " · ")
}
