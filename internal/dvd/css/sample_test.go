package css

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestDecryptSampleRecoversKeyAndDecryptsOnlySample(t *testing.T) {
	key := [5]byte{0x11, 0x22, 0x33, 0x44, 0x55}
	data := append(buildScrambledSector(key), buildScrambledSector(key)...)
	want := append([]byte(nil), data...)
	d := NewDecryptor(key)
	for offset := 0; offset < len(want); offset += SectorSize {
		if err := d.DecryptSector(want[offset : offset+SectorSize]); err != nil {
			t.Fatal(err)
		}
	}
	if err := DecryptSample(context.Background(), data); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, want) {
		t.Fatal("bounded sample was not decrypted")
	}
}

func TestDecryptSampleCancelsAndRejectsPartialSector(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := DecryptSample(ctx, make([]byte, SectorSize)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := DecryptSample(context.Background(), make([]byte, SectorSize+1)); err == nil {
		t.Fatal("partial sector accepted")
	}
}
