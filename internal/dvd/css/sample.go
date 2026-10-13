package css

import (
	"context"
	"fmt"
)

// DecryptSample decrypts a bounded, sector-aligned preview in place. Key
// recovery inspects only these bytes and never scans the source disc. Failure
// leaves scrambled bytes unpresented; callers must discard the sample.
func DecryptSample(ctx context.Context, data []byte) error {
	key, encrypted, err := RecoverSampleKey(ctx, data)
	if err != nil {
		return err
	}
	if !encrypted {
		return nil
	}
	return DecryptSampleWithKey(ctx, data, key)
}

// RecoverSampleKey performs bounded key recovery on an already-read sample.
// The boolean reports whether the sample contained encrypted payload sectors.
func RecoverSampleKey(ctx context.Context, data []byte) ([5]byte, bool, error) {
	if len(data)%SectorSize != 0 {
		return [5]byte{}, false, fmt.Errorf("CSS sample is not sector aligned")
	}
	var key [5]byte
	found, encrypted := false, false
	attempts := 0
	for offset := 0; offset < len(data); offset += SectorSize {
		if err := ctx.Err(); err != nil {
			return [5]byte{}, false, err
		}
		sector := data[offset : offset+SectorSize]
		if !IsScrambledSector(sector) || sector[0x11] == 0xbb || sector[0x11] == 0xbe || sector[0x11] == 0xbf {
			continue
		}
		encrypted = true
		attempts++
		if key, found = attackPattern(sector); found {
			break
		}
		if attempts >= 128 {
			break
		}
	}
	if !encrypted {
		return [5]byte{}, false, nil
	}
	if !found {
		return [5]byte{}, true, ErrTitleKeyNotFound
	}
	return key, true, nil
}

func DecryptSampleWithKey(ctx context.Context, data []byte, key [5]byte) error {
	if len(data)%SectorSize != 0 {
		return fmt.Errorf("CSS sample is not sector aligned")
	}
	d := NewDecryptor(key)
	for offset := 0; offset < len(data); offset += SectorSize {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := d.DecryptSector(data[offset : offset+SectorSize]); err != nil {
			return err
		}
	}
	return nil
}
