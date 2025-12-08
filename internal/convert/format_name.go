package convert

import "strings"

// humanFriendlyFormat normalizes format names to less confusing labels.
func humanFriendlyFormat(format, formatLong string) string {
	f := strings.ToLower(strings.TrimSpace(format))
	fl := strings.ToLower(strings.TrimSpace(formatLong))

	// Treat common QuickTime/MOV wording as MP4 when the extension is typically mp4
	if strings.Contains(f, "mov") || strings.Contains(fl, "quicktime") {
		return "MP4"
	}
	if f != "" {
		return format
	}
	if formatLong != "" {
		return formatLong
	}
	return "Unknown"
}
