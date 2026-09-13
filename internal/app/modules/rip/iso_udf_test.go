package rip

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExtractedVideoTSPath locks down the flat-vs-nested extraction layout the
// resolver must accept. The native readers extract a target directory's
// descendants FLAT into the destination root (tempDir/VIDEO_TS.IFO, no nested
// tempDir/VIDEO_TS folder), and resolveISOWithUDF previously required the
// nested layout — which failed every grey-market ISO 9660-only rip with
// "VIDEO_TS not found in ISO 9660 image".
func TestExtractedVideoTSPath(t *testing.T) {
	t.Run("nested directory wins when present", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "VIDEO_TS"), 0755); err != nil {
			t.Fatal(err)
		}
		got := extractedVideoTSPath(root, "VIDEO_TS")
		if got != filepath.Join(root, "VIDEO_TS") {
			t.Fatalf("got %q, want %q", got, filepath.Join(root, "VIDEO_TS"))
		}
	})

	t.Run("flat layout via VIDEO_TS.IFO marker", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "VIDEO_TS.IFO"), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
		got := extractedVideoTSPath(root, "VIDEO_TS")
		if got != root {
			t.Fatalf("got %q, want %q (flat DVD layout)", got, root)
		}
	})

	t.Run("flat layout via index.bdmv marker", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "index.bdmv"), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
		got := extractedVideoTSPath(root, "BDMV")
		if got != root {
			t.Fatalf("got %q, want %q (flat Blu-ray layout)", got, root)
		}
	})

	t.Run("no markers found", func(t *testing.T) {
		if got := extractedVideoTSPath(t.TempDir(), "VIDEO_TS"); got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})
}

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

	// Rip-path regression: resolveISOWithUDF must find the extracted content
	// (flat layout) and return a resolvable VIDEO_TS directory, mirroring the
	// tester-reported "VIDEO_TS not found in ISO 9660 image" failure.
	tmp := t.TempDir()
	cleanup := func() {}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open ISO: %v", err)
	}
	defer f.Close()
	videoTS, _, err := resolveISOWithUDF(t.Context(), f, path, tmp, cleanup)
	if err != nil {
		t.Fatalf("resolveISOWithUDF: %v", err)
	}
	t.Logf("resolved VIDEO_TS: %s (%d files)", videoTS, extractFileCount(videoTS))
	if len(videoTS) == 0 {
		t.Fatal("empty resolved path")
	}
}
