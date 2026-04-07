// Package css implements DVD Content Scramble System (CSS) decryption
// for archival purposes. Supports decrypting protected VOB/IFO data.
package css

import (
	"encoding/binary"
	"errors"
	"io"
)

var (
	ErrNotEncrypted  = errors.New("content is not CSS encrypted")
	ErrNoTitleKey    = errors.New("no title key available")
	ErrInvalidSector = errors.New("invalid sector for decryption")
)

const (
	SectorSize      = 2048
	ScrambledSector = 0x40000
	TitleKeySize    = 5
	DiscKeySize     = 5
	PlayerKeySize   = 5
)

// Decryptor handles CSS decryption for DVD sectors.
type Decryptor struct {
	titleKey  [TitleKeySize]byte
	discKey   [DiscKeySize]byte
	encrypted bool
}

// NewDecryptor creates a CSS decryptor from a title key.
// The title key is extracted from the disc's key block.
func NewDecryptor(titleKey [TitleKeySize]byte) *Decryptor {
	d := &Decryptor{titleKey: titleKey}
	return d
}

// NewDecryptorFromDiscKey creates a decryptor by deriving title key from disc key.
func NewDecryptorFromDiscKey(discKey [DiscKeySize]byte, encryptedTitleKey []byte) (*Decryptor, error) {
	if len(encryptedTitleKey) < TitleKeySize {
		return nil, ErrNoTitleKey
	}
	d := &Decryptor{discKey: discKey}
	d.decryptTitleKey(encryptedTitleKey)
	return d, nil
}

// decryptTitleKey decrypts an encrypted title key using the disc key.
func (d *Decryptor) decryptTitleKey(encrypted []byte) {
	var key [TitleKeySize]byte
	for i := 0; i < TitleKeySize; i++ {
		key[i] = encrypted[i] ^ d.discKey[i]
	}
	d.titleKey = key
}

// IsEncrypted returns true if the sector appears to be CSS encrypted.
func IsEncrypted(sector []byte) bool {
	if len(sector) < SectorSize {
		return false
	}
	// CSS encrypted sectors have specific byte patterns
	// Check for scrambled sector marker
	return (binary.BigEndian.Uint32(sector[0:4]) & 0xFFFFFF00) != 0x00000100
}

// DecryptSector decrypts a single DVD sector in place.
// Returns ErrNotEncrypted if the sector doesn't appear encrypted.
func (d *Decryptor) DecryptSector(sector []byte) error {
	if len(sector) < SectorSize {
		return ErrInvalidSector
	}

	if !IsEncrypted(sector) {
		return ErrNotEncrypted
	}

	// CSS stream cipher decryption
	d.decryptCSS(sector)
	return nil
}

// decryptCSS applies the CSS stream cipher to decrypt a sector.
func (d *Decryptor) decryptCSS(sector []byte) {
	var lfsr1, lfsr2 uint32

	// Initialize LFSRs from title key
	lfsr1 = uint32(d.titleKey[0])<<9 | uint32(d.titleKey[1])<<1 | uint32(d.titleKey[2]>>7)
	lfsr2 = uint32(d.titleKey[2])<<5 | uint32(d.titleKey[3])>>3 | uint32(d.titleKey[4])<<5

	for i := 0; i < SectorSize; i++ {
		// Clock LFSR1
		bit1 := ((lfsr1 >> 8) ^ (lfsr1 >> 4) ^ (lfsr1 >> 2) ^ lfsr1) & 1
		lfsr1 = (lfsr1 >> 1) | (bit1 << 15)

		// Clock LFSR2
		bit2 := ((lfsr2 >> 1) ^ (lfsr2 >> 2) ^ (lfsr2 >> 4) ^ lfsr2) & 1
		lfsr2 = (lfsr2 >> 1) | (bit2 << 15)

		// XOR with sector data
		sector[i] ^= byte(((lfsr1 ^ lfsr2) & 0xFF))
	}
}

// DecryptReadStream decrypts CSS-encrypted VOB data using stream cipher.
// This is the common case for encrypted movie content.
func (d *Decryptor) DecryptReadStream(data []byte, sectorOffset int64) {
	var lfsr1, lfsr2 uint32

	// Initialize LFSRs from title key
	lfsr1 = uint32(d.titleKey[0])<<9 | uint32(d.titleKey[1])<<1 | uint32(d.titleKey[2]>>7)
	lfsr2 = uint32(d.titleKey[2])<<5 | uint32(d.titleKey[3])>>3 | uint32(d.titleKey[4])<<5

	// Skip to sector-aligned position
	offset := int(sectorOffset % SectorSize)

	for i := 0; i < len(data); i++ {
		// Clock LFSRs on sector boundaries
		if (i+offset)%SectorSize == 0 {
			lfsr1 = uint32(d.titleKey[0])<<9 | uint32(d.titleKey[1])<<1 | uint32(d.titleKey[2]>>7)
			lfsr2 = uint32(d.titleKey[2])<<5 | uint32(d.titleKey[3])>>3 | uint32(d.titleKey[4])<<5
		}

		bit1 := ((lfsr1 >> 8) ^ (lfsr1 >> 4) ^ (lfsr1 >> 2) ^ lfsr1) & 1
		lfsr1 = (lfsr1 >> 1) | (bit1 << 15)

		bit2 := ((lfsr2 >> 1) ^ (lfsr2 >> 2) ^ (lfsr2 >> 4) ^ lfsr2) & 1
		lfsr2 = (lfsr2 >> 1) | (bit2 << 15)

		data[i] ^= byte(((lfsr1 ^ lfsr2) & 0xFF))
	}
}

// DecryptReader wraps an io.Reader to decrypt CSS-encrypted VOB data on the fly.
type DecryptReader struct {
	reader      io.Reader
	decryptor   *Decryptor
	sectorBuf   [SectorSize]byte
	bufPos      int
	bufLen      int
	sectorCount int64
}

// NewDecryptReader creates a reader that decrypts data from the underlying source.
func NewDecryptReader(r io.Reader, d *Decryptor) *DecryptReader {
	return &DecryptReader{reader: r, decryptor: d}
}

func (dr *DecryptReader) Read(p []byte) (n int, err error) {
	total := 0

	for total < len(p) {
		// Refill buffer if empty
		if dr.bufPos >= dr.bufLen {
			dr.bufLen, err = io.ReadFull(dr.reader, dr.sectorBuf[:])
			if err != nil {
				if total > 0 {
					return total, nil
				}
				return total, err
			}

			// Decrypt the sector
			if IsEncrypted(dr.sectorBuf[:]) {
				dr.decryptor.DecryptSector(dr.sectorBuf[:])
			}
			dr.bufPos = 0
			dr.sectorCount++
		}

		// Copy from buffer to output
		copied := copy(p[total:], dr.sectorBuf[dr.bufPos:dr.bufLen])
		total += copied
		dr.bufPos += copied
	}

	return total, nil
}

// TitleKeys represents the decrypted title keys for a DVD.
type TitleKeys struct {
	keys [][TitleKeySize]byte
}

// ExtractTitleKeys attempts to extract title keys from a VIDEO_TS.IFO file.
// This requires the disc key which is typically obtained via authentication.
func ExtractTitleKeys(vmgi io.ReadSeeker, discKey [DiscKeySize]byte) (*TitleKeys, error) {
	// Read VMGI to find title key locations
	// Title keys are stored in the disc key block
	var keys [][TitleKeySize]byte

	// Seek to key block location in IFO
	_, err := vmgi.Seek(128, io.SeekStart) // Typical location for key block
	if err != nil {
		return nil, err
	}

	// Read encrypted title keys
	encKeys := make([]byte, TitleKeySize)
	for {
		_, err := io.ReadFull(vmgi, encKeys)
		if err != nil {
			break
		}

		// Decrypt title key with disc key
		var tk [TitleKeySize]byte
		for i := 0; i < TitleKeySize; i++ {
			tk[i] = encKeys[i] ^ discKey[i%DiscKeySize]
		}
		keys = append(keys, tk)
	}

	if len(keys) == 0 {
		return nil, ErrNoTitleKey
	}

	return &TitleKeys{keys: keys}, nil
}

// Get returns the title key for title index i.
func (tk *TitleKeys) Get(i int) ([TitleKeySize]byte, bool) {
	if i < 0 || i >= len(tk.keys) {
		return [TitleKeySize]byte{}, false
	}
	return tk.keys[i], true
}

// Count returns the number of title keys.
func (tk *TitleKeys) Count() int {
	return len(tk.keys)
}
