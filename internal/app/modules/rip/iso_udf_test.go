package rip

import (
	"os"
	"testing"
)

// TestScanRealISO exercises the scan path (scanISOViaUDF) against a real
// ISO 9660-only DVD image whose broken UDF bridge makes the UDF reader fail.
// Set VT_REAL_ISO to the image path; skipped when unset.
func TestScanRealISO(t *testing.T) {
	path := os.Getenv("VT_REAL_ISO")
	if path == "" {
		t.Skip("VT_REAL_ISO not set")
	}
	result, err := scanISOViaUDF(path, func(note string) {
		t.Logf("note: %s", note)
	})
	if err != nil {
		t.Fatalf("scanISOViaUDF fell back to ISO 9660: %v", err)
	}
	if result == nil {
		t.Fatal("nil scan result")
	}
	if len(result.Titles) == 0 {
		t.Fatalf("expected titles, got none (DiscType=%s, region=%s)", result.DiscType, result.Region)
	}
	for _, tt := range result.Titles {
		t.Logf("title %d (VTS %d): %d chapters, %d audio, %d subs, %.1fs",
			tt.Number, tt.VTSNumber, tt.NumChapters, len(tt.Audio), len(tt.Subtitles), tt.Duration)
	}
}