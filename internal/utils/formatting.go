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

// formatPercent renders a percentage with no trailing zeros after decimal.
func formatPercent(val float64) string {
	val = math.Round(val*10) / 10 // one decimal
	if val == math.Trunc(val) {
		return fmt.Sprintf("%d%%", int(val))
	}
	return fmt.Sprintf("%.1f%%", val)
}
