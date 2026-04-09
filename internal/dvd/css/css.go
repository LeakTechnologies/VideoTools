// Package css implements DVD Content Scramble System (CSS) decryption.
package css

import (
	"errors"
	"io"
	"os"
)

var (
	ErrNotEncrypted  = errors.New("content is not CSS encrypted")
	ErrNoTitleKey    = errors.New("no title key available")
	ErrInvalidSector = errors.New("invalid sector for decryption")
)

const (
	SectorSize    = 2048
	TitleKeySize  = 5
	DiscKeySize   = 5
	PlayerKeySize = 5
)

// Decryptor holds a title key and decrypts CSS-scrambled DVD sectors.
type Decryptor struct {
	titleKey [TitleKeySize]byte
}

// NewDecryptor creates a Decryptor from a plaintext title key.
func NewDecryptor(titleKey [TitleKeySize]byte) *Decryptor {
	return &Decryptor{titleKey: titleKey}
}

// NewDecryptorFromDiscKey creates a Decryptor by decrypting an encrypted title key
// with the given disc key.
func NewDecryptorFromDiscKey(discKey [DiscKeySize]byte, encryptedTitleKey [TitleKeySize]byte) *Decryptor {
	return &Decryptor{titleKey: DecryptTitleKey(discKey, encryptedTitleKey)}
}

// DecryptSector decrypts a CSS-scrambled 2048-byte sector in place.
// If the sector's scrambling flag is not set, it is left unchanged.
func (d *Decryptor) DecryptSector(sector []byte) error {
	if len(sector) < SectorSize {
		return ErrInvalidSector
	}
	unscrambleSector(d.titleKey, sector)
	return nil
}

// DecryptReader wraps an io.Reader to transparently decrypt CSS-scrambled sectors.
type DecryptReader struct {
	r      io.Reader
	d      *Decryptor
	buf    [SectorSize]byte
	bufPos int
	bufLen int
}

// NewDecryptReader creates a DecryptReader that decrypts data from r sector by sector.
func NewDecryptReader(r io.Reader, d *Decryptor) *DecryptReader {
	return &DecryptReader{r: r, d: d}
}

func (dr *DecryptReader) Read(p []byte) (int, error) {
	total := 0
	for total < len(p) {
		if dr.bufPos >= dr.bufLen {
			n, err := io.ReadFull(dr.r, dr.buf[:])
			if err != nil {
				if total > 0 {
					return total, nil
				}
				if err == io.ErrUnexpectedEOF {
					// Partial final sector — pass through as-is.
					dr.bufLen = n
					dr.bufPos = 0
					break
				}
				return 0, err
			}
			if IsScrambledSector(dr.buf[:]) {
				_ = dr.d.DecryptSector(dr.buf[:])
			}
			dr.bufPos = 0
			dr.bufLen = n
		}
		copied := copy(p[total:], dr.buf[dr.bufPos:dr.bufLen])
		total += copied
		dr.bufPos += copied
	}
	return total, nil
}

// DecryptVOB decrypts a CSS-scrambled VOB file to outPath, returning bytes written.
func DecryptVOB(vobPath, outPath string, d *Decryptor) (int64, error) {
	in, err := os.Open(vobPath)
	if err != nil {
		return 0, err
	}
	defer in.Close()

	out, err := os.Create(outPath)
	if err != nil {
		return 0, err
	}
	defer out.Close()

	return io.Copy(out, NewDecryptReader(in, d))
}
