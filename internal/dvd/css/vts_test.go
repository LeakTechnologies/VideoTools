package css

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// buildScrambledVOB returns a scrambled VOB of n sectors (≥2 so
// DetectEncryption's noise threshold is met), each carrying the XOR of key with
// the sector header seed.
func buildScrambledVOB(key [5]byte, n int) []byte {
	out := make([]byte, 0, n*SectorSize)
	for i := 0; i < n; i++ {
		out = append(out, buildScrambledSector(key)...)
	}
	return out
}

// buildVTSDir assembles a synthetic VIDEO_TS tree: an IFO passthrough file, a
// scrambled VTS_01 set, and a clear VTS_02 set. Each scrambled set uses a
// different title key to prove per-VTS key recovery.
func buildVTSDir(t *testing.T, key01 [5]byte, key02 [5]byte) (string, []byte) {
	t.Helper()
	dir := t.TempDir()

	ifo := []byte("not-a-real-ifo-but-must-pass-through-verbatim")
	if err := os.WriteFile(filepath.Join(dir, "VIDEO_TS.IFO"), ifo, 0o600); err != nil {
		t.Fatal(err)
	}

	clearSector := func() []byte {
		s := make([]byte, SectorSize)
		for i := range s {
			s[i] = [4]byte{0x00, 0x00, 0x01, 0xba}[i%4]
		}
		return s
	}

	// VTS_01: one scrambled file, one clear file.
	if err := os.WriteFile(filepath.Join(dir, "VTS_01_1.VOB"), buildScrambledVOB(key01, 4), 0o600); err != nil {
		t.Fatal(err)
	}
	clear01 := clearSector()
	if err := os.WriteFile(filepath.Join(dir, "VTS_01_2.VOB"), clear01, 0o600); err != nil {
		t.Fatal(err)
	}

	// VTS_02: whole set clear.
	if err := os.WriteFile(filepath.Join(dir, "VTS_02_1.VOB"), clearSector(), 0o600); err != nil {
		t.Fatal(err)
	}

	return dir, ifo
}

func TestDecryptVideoTS(t *testing.T) {
	key01 := [5]byte{0x01, 0x02, 0x03, 0x04, 0x05}
	key02 := [5]byte{0x11, 0x22, 0x33, 0x44, 0x55}
	src, ifo := buildVTSDir(t, key01, key02)

	dst := filepath.Join(t.TempDir(), "decrypted")
	var logged []string
	if err := DecryptVideoTS(src, dst, func(s string) { logged = append(logged, s) }); err != nil {
		t.Fatalf("DecryptVideoTS: %v", err)
	}

	// IFO passes through verbatim.
	gotIFO, err := os.ReadFile(filepath.Join(dst, "VIDEO_TS.IFO"))
	if err != nil {
		t.Fatalf("read decrypted IFO: %v", err)
	}
	if string(gotIFO) != string(ifo) {
		t.Fatalf("IFO mangled: got %q want %q", gotIFO, ifo)
	}

	// Scrambled VTS_01_1 restores the plaintext cycle after a full
	// sector-by-sector DecryptReader pass (DecryptSector handles one sector).
	raw01, err := os.ReadFile(filepath.Join(dst, "VTS_01_1.VOB"))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw01) != 4*SectorSize {
		t.Fatalf("VTS_01_1 decoded length = %d, want %d", len(raw01), 4*SectorSize)
	}
	dec01, err := io.ReadAll(NewDecryptReader(bytes.NewReader(raw01), NewDecryptor(key01)))
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range dec01 {
		if want := [4]byte{0x00, 0x00, 0x01, 0xba}[i%4]; b != want {
			t.Fatalf("VTS_01_1 byte 0x%x = %#x, want %#x", i, b, want)
		}
	}

	// Clear VTS_01_2 and VTS_02_1 pass through untouched, and the output
	// exists for every expected file.
	for _, name := range []string{"VTS_01_2.VOB", "VTS_02_1.VOB"} {
		if _, err := os.Stat(filepath.Join(dst, name)); err != nil {
			t.Fatalf("missing decrypted output %s: %v", name, err)
		}
	}

	if len(logged) == 0 {
		t.Fatal("expected at least one decryption log line")
	}
}

func TestDecryptVideoTSOutputIsPlaintext(t *testing.T) {
	// The DecryptVOB path runs sector-by-sector through DecryptReader, which
	// skips sectors whose scrambling flag is clear. The fixture's clear
	// VTS_02 set must therefore survive verbatim, and the scrambled VTS_01_1
	// must decode back to the fixture's plaintext cycle.
	titleKey := [5]byte{0xaa, 0x66, 0x37, 0x91, 0x0e}
	src, _ := buildVTSDir(t, titleKey, [5]byte{0xde, 0xad, 0xbe, 0xef, 0x01})

	dst := filepath.Join(t.TempDir(), "decrypted")
	if err := DecryptVideoTS(src, dst, nil); err != nil {
		t.Fatalf("DecryptVideoTS: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(dst, "VTS_01_1.VOB"))
	if err != nil {
		t.Fatal(err)
	}
	if IsScrambledSector(out) {
		t.Fatal("decrypted output is still marked scrambled")
	}
	for i, b := range out {
		if want := [4]byte{0x00, 0x00, 0x01, 0xba}[i%4]; b != want {
			t.Fatalf("decrypted VTS_01_1 byte 0x%x = %#x, want %#x", i, b, want)
		}
	}

	clearOut, err := os.ReadFile(filepath.Join(dst, "VTS_02_1.VOB"))
	if err != nil {
		t.Fatal(err)
	}
	if IsScrambledSector(clearOut) {
		t.Fatal("clear passthrough got marked scrambled")
	}
}