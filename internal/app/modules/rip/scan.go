package rip

import (
	"fmt"
	"path/filepath"
	"strings"

	"git.leaktechnologies.dev/stu/VideoTools/internal/dvd/ifo"
	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

// ScanDisc reads the VMG IFO and per-VTS IFOs to populate a DiscScanResult.
// It is safe to call from a goroutine; it performs no UI work.
func ScanDisc(videoTSPath string) (*DiscScanResult, error) {
	vmgPath := filepath.Join(videoTSPath, "VIDEO_TS.IFO")
	tsps, err := ifo.ReadTitleList(vmgPath)
	if err != nil {
		return nil, fmt.Errorf("read title list: %w", err)
	}

	// Cache per-VTS IFO reads — multiple titles can share a VTS.
	type vtsKey = int
	vtsCache := map[vtsKey]*ifo.TitleInfo{}

	result := &DiscScanResult{}
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
	logging.Info(logging.CatDVD, "ScanDisc: %d titles in VIDEO_TS", len(result.Titles))
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
