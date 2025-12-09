package utils

import (
	"fmt"
	"math"
)

// ReductionText returns a string like "965 MB (24% reduction)" given original bytes and new bytes.
func ReductionText(origBytes, newBytes int64) string {
	if origBytes <= 0 || newBytes <= 0 {
		return ""
	}
	if newBytes >= origBytes {
		return ""
	}
	reduction := 100.0 * (1.0 - float64(newBytes)/float64(origBytes))
	if reduction <= 0 {
		return ""
	}
	return formatBytes(newBytes) + " (" + formatPercent(reduction) + " reduction)"
}

func formatBytes(b int64) string {
	if b <= 0 {
		return "0 B"
	}
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(MB))
	default:
		return fmt.Sprintf("%.2f KB", float64(b)/float64(KB))
	}
}

// DeltaBytes renders size plus delta vs reference.
func DeltaBytes(newBytes, refBytes int64) string {
	if newBytes <= 0 {
		return "0 B"
	}
	size := formatBytes(newBytes)
	if refBytes <= 0 || refBytes == newBytes {
		return size
	}
	change := float64(newBytes-refBytes) / float64(refBytes)
	dir := "increase"
	if change < 0 {
		dir = "reduction"
	}
	pct := math.Abs(change) * 100
	return fmt.Sprintf("%s (%.1f%% %s)", size, pct, dir)
}

// DeltaBitrate renders bitrate plus delta vs reference (expects bps).
func DeltaBitrate(newBps, refBps int) string {
	if newBps <= 0 {
		return "--"
	}
	br := formatBitrate(newBps)
	if refBps <= 0 || refBps == newBps {
		return br
	}
	change := float64(newBps-refBps) / float64(refBps)
	dir := "increase"
	if change < 0 {
		dir = "reduction"
	}
	pct := math.Abs(change) * 100
	return fmt.Sprintf("%s (%.1f%% %s)", br, pct, dir)
}

// formatPercent renders a percentage with no trailing zeros after decimal.
func formatPercent(val float64) string {
	val = math.Round(val*10) / 10 // one decimal
	if val == math.Trunc(val) {
		return fmt.Sprintf("%d%%", int(val))
	}
	return fmt.Sprintf("%.1f%%", val)
}
