package css

import (
	"os"
	"path/filepath"
	"testing"
)

// cssTab1Inv is the inverse permutation of cssTab1. CSS sector decryption is
// P = cssTab1[S] ⊕ ks (see unscrambleSector), so producing a *scrambled*
// sector from plaintext requires S = cssTab1⁻¹[P ⊕ ks]. cssTab1 must be a
// permutation for CSS to be invertible; we require that here.
var cssTab1Inv = func() [256]byte {
	var inv [256]byte
	for i := 0; i < 256; i++ {
		inv[cssTab1[i]] = byte(i)
	}
	return inv
}()

// scrambleSector encrypts a plaintext sector in-place using the CSS title key.
// It mirrors the keystream loop of unscrambleSector (bytes 0x80..0x800) but
// writes S = cssTab1⁻¹[P ⊕ ks] instead of the decrypt-side
// P = cssTab1[S] ⊕ ks. This is the opposite direction of the real cipher and
// is neither symmetric nor self-inverse.
func scrambleSector(titleKey [5]byte, sec []byte) {
	if len(sec) < 0x800 {
		return
	}

	t1 := uint32(titleKey[0]^sec[0x54]) | 0x100
	t2 := uint32(titleKey[1] ^ sec[0x55])
	t3 := (uint32(titleKey[2]) ^ uint32(sec[0x56])) |
		((uint32(titleKey[3]) ^ uint32(sec[0x57])) << 8) |
		((uint32(titleKey[4]) ^ uint32(sec[0x58])) << 16)
	t4 := t3 & 7
	t3 = t3*2 + 8 - t4

	var t5 uint32
	for i := 0x80; i < 0x800; i++ {
		t4 = uint32(cssTab2[t2]) ^ uint32(cssTab3[t1])
		t2 = t1 >> 1
		t1 = ((t1 & 1) << 8) ^ t4
		t6lfsr1 := uint32(cssTab5[t4])

		t6 := ((((((t3 >> 3) ^ t3) >> 1) ^ t3) >> 8) ^ t3) >> 5 & 0xff
		t3 = (t3 << 8) | t6
		t6lfsr2 := uint32(cssTab4[t6])

		t5 += t6lfsr2 + t6lfsr1

		sec[i] = cssTab1Inv[sec[i]^uint8(t5)]
		t5 >>= 8
	}
}

// buildScrambledSector constructs a genuinely CSS-encrypted 2048-byte sector
// from a known title key. The real scramble is S = cssTab1⁻¹[P ⊕ ks], which is
// the inverse of the decrypt operation and is deliberately implemented by
// scrambleSector rather than by re-using unscrambleSector.
//
// The plaintext is a 4-byte cycle {0x00,0x00,0x01,0xba} that fills the whole
// sector. A pack start (00 00 01 ba) lands at offset 0; the seed at 0x54-0x58
// is kept part of the unbroken cycle (as padding on a real disc is), so the
// repeating pattern just before the encrypted region at 0x80 is exactly what
// the known-plaintext attack exploits.
func buildScrambledSector(titleKey [5]byte) []byte {
	sec := make([]byte, SectorSize)
	for i := range sec {
		sec[i] = [4]byte{0x00, 0x00, 0x01, 0xba}[i%4]
	}
	sec[0x14] |= 0x30 // PES_scrambling_control bits

	scrambleSector(titleKey, sec)
	return sec
}

func TestCrackTitleKeyRecoversKnownKey(t *testing.T) {
	titleKey := [5]byte{0x11, 0x22, 0x33, 0x44, 0x55}

	dir := t.TempDir()
	vobPath := filepath.Join(dir, "VTS_01_1.VOB")
	sec := buildScrambledSector(titleKey)

	// Sector-content verification first: the scrambled form must actually be
	// encrypted (its bytes at 0x80 differ from the plaintext pattern).
	if sec[0x80] == 0x00 || sec[0x14]&0x30 == 0 {
		t.Fatal("test sector is not actually scrambled")
	}

	// A clean 2048*N sector file, no partial tail.
	if err := os.WriteFile(vobPath, sec, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := CrackTitleKeyFiles([]string{vobPath})
	if err != nil {
		t.Fatalf("CrackTitleKeyFiles: %v", err)
	}
	if got != titleKey {
		t.Fatalf("recovered key %#v, want %#v", got, titleKey)
	}

	// Round-trip: a Decryptor built from the cracked key must restore the
	// original plaintext sector (the 4-byte cycle).
	back := make([]byte, len(sec))
	copy(back, sec)
	if err := NewDecryptor(got).DecryptSector(back); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 0x800; i++ {
		if got := [4]byte{0x00, 0x00, 0x01, 0xba}[i%4]; back[i] != got {
			t.Fatalf("sector reframed wrong at offset 0x%x: %#x, want %#x", i, back[i], got)
		}
	}
}

func TestCrackTitleKeyMultiFile(t *testing.T) {
	dir := t.TempDir()
	titleKey := [5]byte{0xde, 0xad, 0xbe, 0xef, 0x01}

	// First file is unencrypted (empty), the key lives in the second file.
	clear := filepath.Join(dir, "VTS_01_1.VOB")
	if err := os.WriteFile(clear, make([]byte, SectorSize), 0o600); err != nil {
		t.Fatal(err)
	}
	scrambled := filepath.Join(dir, "VTS_01_2.VOB")
	if err := os.WriteFile(scrambled, buildScrambledSector(titleKey), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := CrackTitleKeyFiles([]string{clear, scrambled})
	if err != nil {
		t.Fatalf("CrackTitleKeyFiles: %v", err)
	}
	if got != titleKey {
		t.Fatalf("recovered key %#v, want %#v", got, titleKey)
	}
}

func TestCrackTitleKeyNoEncryption(t *testing.T) {
	dir := t.TempDir()
	clear := filepath.Join(dir, "VTS_01_1.VOB")
	sec := make([]byte, SectorSize)
	for i := range sec {
		sec[i] = [4]byte{0x00, 0x00, 0x01, 0xba}[i%4]
	}
	if err := os.WriteFile(clear, sec, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := CrackTitleKeyFiles([]string{clear}); err == nil {
		t.Fatal("expected ErrTitleKeyNotFound for an unencrypted file")
	}
}