package utils

import "math"

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
	return formatFileSize(newBytes) + " (" + formatPercent(reduction) + " reduction)"
}

// formatPercent renders a percentage with no trailing zeros after decimal.
func formatPercent(val float64) string {
	val = math.Round(val*10) / 10 // one decimal
	if val == math.Trunc(val) {
		return formatInt(int(val)) + "%"
	}
	return formatFloat(val) + "%"
}
