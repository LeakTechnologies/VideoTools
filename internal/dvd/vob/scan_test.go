package vob

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// buildTestVOB creates a synthetic VOB in a temp file containing the given
// sectors. navPositions lists which sector indices (0-based) should be NAV_PCKs;
// all other sectors are regular video packs.
func buildTestVOB(t *testing.T, totalSectors int, navPositions []int) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.vob")

	navSet := make(map[int]bool, len(navPositions))
	for _, n := range navPositions {
		navSet[n] = true
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create test vob: %v", err)
	}
	defer f.Close()

	sector := make([]byte, PackSize)
	for i := 0; i < totalSectors; i++ {
		// Zero-fill
		for j := range sector {
			sector[j] = 0
		}
		if navSet[i] {
			// NAV_PCK: pack start code at 0, system header code at 14
			binary.BigEndian.PutUint32(sector[0:4], PackStartCode)
			binary.BigEndian.PutUint32(sector[14:18], SystemHeaderCode)
		} else {
			// Regular video pack: pack start code only, no system header
			binary.BigEndian.PutUint32(sector[0:4], PackStartCode)
			binary.BigEndian.PutUint32(sector[14:18], uint32(VideoStream0))
		}
		if _, err := f.Write(sector); err != nil {
			t.Fatalf("write sector %d: %v", i, err)
		}
	}
	return path
}

func TestScanVOBForNAVPCKs_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.vob")
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	sectors, err := ScanVOBForNAVPCKs(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sectors) != 0 {
		t.Errorf("expected 0 NAV_PCKs, got %d", len(sectors))
	}
}

func TestScanVOBForNAVPCKs_NoNAVPCKs(t *testing.T) {
	path := buildTestVOB(t, 10, nil)
	sectors, err := ScanVOBForNAVPCKs(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sectors) != 0 {
		t.Errorf("expected 0 NAV_PCKs, got %d: %v", len(sectors), sectors)
	}
}

func TestScanVOBForNAVPCKs_SingleNAVPCK(t *testing.T) {
	path := buildTestVOB(t, 5, []int{2})
	got, err := ScanVOBForNAVPCKs(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != 2 {
		t.Errorf("expected [2], got %v", got)
	}
}

func TestScanVOBForNAVPCKs_MultipleNAVPCKs(t *testing.T) {
	navAt := []int{0, 5, 10, 15}
	path := buildTestVOB(t, 20, navAt)
	got, err := ScanVOBForNAVPCKs(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(navAt) {
		t.Fatalf("expected %d NAV_PCKs, got %d: %v", len(navAt), len(got), got)
	}
	for i, want := range navAt {
		if got[i] != uint32(want) {
			t.Errorf("NAV_PCK[%d]: got sector %d, want %d", i, got[i], want)
		}
	}
}

func TestScanVOBForNAVPCKs_PartialLastSector(t *testing.T) {
	// A VOB with a partial trailing sector should be handled gracefully.
	dir := t.TempDir()
	path := filepath.Join(dir, "partial.vob")

	var buf bytes.Buffer
	// Write 2 complete NAV_PCK sectors
	for i := 0; i < 2; i++ {
		sector := make([]byte, PackSize)
		binary.BigEndian.PutUint32(sector[0:4], PackStartCode)
		binary.BigEndian.PutUint32(sector[14:18], SystemHeaderCode)
		buf.Write(sector)
	}
	// Append a partial sector (should be ignored)
	buf.Write(make([]byte, 100))

	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := ScanVOBForNAVPCKs(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 NAV_PCKs, got %d: %v", len(got), got)
	}
}
