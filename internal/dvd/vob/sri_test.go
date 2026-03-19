package vob

import (
	"encoding/binary"
	"os"
	"testing"
)

// buildTimedVOB writes a minimal VOB where every sector is a NAV_PCK with PTMs
// spaced ptmStep ticks apart. Returns the file path and the expected NAVPCKInfo slice.
func buildTimedVOB(t *testing.T, count int, ptmStep uint32) (string, []NAVPCKInfo) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "test_*.vob")
	if err != nil {
		t.Fatalf("create temp vob: %v", err)
	}
	defer f.Close()

	sector := make([]byte, PackSize)
	var navs []NAVPCKInfo

	for i := 0; i < count; i++ {
		for j := range sector {
			sector[j] = 0
		}
		binary.BigEndian.PutUint32(sector[0:], PackStartCode)
		binary.BigEndian.PutUint32(sector[14:], SystemHeaderCode)
		ptm := uint32(i) * ptmStep
		binary.BigEndian.PutUint32(sector[pciPTMByteOffset:], ptm)
		for k := 0; k < dsiVOBUSRICount; k++ {
			binary.BigEndian.PutUint32(sector[sriByteOffset+k*4:], SRIEndOfCell)
		}
		if _, err := f.Write(sector); err != nil {
			t.Fatalf("write sector %d: %v", i, err)
		}
		navs = append(navs, NAVPCKInfo{Sector: uint32(i), PTM: ptm})
	}
	return f.Name(), navs
}

// TestScanVOBNAVPCKs_PTMs verifies PTMs are read from the correct byte offset.
func TestScanVOBNAVPCKs_PTMs(t *testing.T) {
	const ptmStep = uint32(90000) // 1 second per VOBU
	path, want := buildTimedVOB(t,5, ptmStep)

	got, err := ScanVOBNAVPCKs(path)
	if err != nil {
		t.Fatalf("ScanVOBNAVPCKs: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i, g := range got {
		if g.Sector != want[i].Sector {
			t.Errorf("[%d] Sector = %d, want %d", i, g.Sector, want[i].Sector)
		}
		if g.PTM != want[i].PTM {
			t.Errorf("[%d] PTM = %d, want %d", i, g.PTM, want[i].PTM)
		}
	}
}

// TestPatchVOBUSRI_ForwardEntry verifies that a forward SRI entry is filled when
// a VOBU exists at the requested time offset.
func TestPatchVOBUSRI_ForwardEntry(t *testing.T) {
	// 10 VOBUs, 1 second apart (ptmStep = 90000 ticks = 1s)
	path, navs := buildTimedVOB(t,10, 90000)

	if err := PatchVOBUSRI(path, navs); err != nil {
		t.Fatalf("PatchVOBUSRI: %v", err)
	}

	// Read back sector 0's VOBU_SRI
	f, _ := os.Open(path)
	defer f.Close()
	buf := make([]byte, PackSize)
	f.Read(buf)

	// Entry [0] = forward 0.5s = 45000 ticks. Nearest VOBU is at 1s (sector 1).
	entry0 := binary.BigEndian.Uint32(buf[sriByteOffset:])
	// Should be non-SRIEndOfCell (a VOBU exists within reach)
	if entry0 == SRIEndOfCell {
		t.Errorf("FWD[0] (0.5s) = SRIEndOfCell, expected a valid sector offset")
	}
	// The relative offset should be positive (forward)
	if entry0&0x80000000 != 0 {
		t.Errorf("FWD[0] has backward bit set: 0x%08X", entry0)
	}
}

// TestPatchVOBUSRI_BackwardEntry verifies backward SRI entries use the high bit.
func TestPatchVOBUSRI_BackwardEntry(t *testing.T) {
	// 5 VOBUs, 2 seconds apart
	path, navs := buildTimedVOB(t,5, 2*90000)

	if err := PatchVOBUSRI(path, navs); err != nil {
		t.Fatalf("PatchVOBUSRI: %v", err)
	}

	// Read back sector 4 (last VOBU) — it should have backward entries filled
	f, _ := os.Open(path)
	defer f.Close()
	buf := make([]byte, PackSize)
	// Seek to sector 4
	f.Seek(int64(4)*PackSize, 0)
	f.Read(buf)

	// BWD entries start at index 20. Entry [20] = backward 0.5s.
	// From VOBU 4 (PTM=8*90000=720000), 0.5s back → PTM=675000.
	// VOBU 3 is at PTM=6*90000=540000 which is within the search.
	bwdEntry := binary.BigEndian.Uint32(buf[sriByteOffset+20*4:])
	if bwdEntry == SRIEndOfCell {
		t.Errorf("BWD[0] from last sector = SRIEndOfCell, expected a valid entry")
	}
	// Backward entries should have bit 31 set
	if bwdEntry&0x80000000 == 0 {
		t.Errorf("BWD[0] missing backward bit: 0x%08X", bwdEntry)
	}
}

// TestPatchVOBUSRI_TooFewVOBUs verifies no error when count < 2.
func TestPatchVOBUSRI_TooFewVOBUs(t *testing.T) {
	path, navs := buildTimedVOB(t,1, 90000)
	if err := PatchVOBUSRI(path, navs); err != nil {
		t.Errorf("PatchVOBUSRI with 1 VOBU: %v", err)
	}
}
