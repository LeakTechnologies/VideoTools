package css

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
)

// Detection errors
var (
	ErrNoDiscKey  = errors.New("no disc key available - cannot decrypt CSS")
	ErrAuthFailed = errors.New("drive authentication failed")
	ErrNoDrive    = errors.New("no DVD drive found")
)

// EncryptedState tracks the encryption state of a DVD.
type EncryptedState int

const (
	NotEncrypted EncryptedState = iota
	EncryptedCSS
	EncryptedUnknown
)

// DetectEncryption checks if VOB data is CSS encrypted.
// CSS encryption affects certain sectors in predictable ways.
func DetectEncryption(vobPath string) (EncryptedState, error) {
	f, err := os.Open(vobPath)
	if err != nil {
		return EncryptedUnknown, err
	}
	defer f.Close()

	// Read first few sectors
	buf := make([]byte, SectorSize*4)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return EncryptedUnknown, err
	}
	buf = buf[:n]

	// CSS encrypted VOBs have specific patterns
	// Check for MPEG pack header - if missing or corrupt, likely encrypted
	for i := 0; i < len(buf)-4; i++ {
		// MPEG pack header: 0x000001BA
		if buf[i] == 0x00 && buf[i+1] == 0x00 && buf[i+2] == 0x01 && buf[i+3] == 0xBA {
			// Found valid pack header - likely not encrypted
			return NotEncrypted, nil
		}
	}

	// No valid MPEG headers found - might be encrypted
	// Check sector headers for CSS patterns
	sectors := len(buf) / SectorSize
	encryptedSectors := 0

	for i := 0; i < sectors; i++ {
		sector := buf[i*SectorSize : (i+1)*SectorSize]
		if isCSSSector(sector) {
			encryptedSectors++
		}
	}

	// If most sectors appear encrypted, report as encrypted
	if encryptedSectors > sectors/2 {
		return EncryptedCSS, nil
	}

	return NotEncrypted, nil
}

// isCSSSector checks if a sector has CSS encryption markers.
func isCSSSector(sector []byte) bool {
	if len(sector) < 12 {
		return false
	}

	// CSS encrypted sectors have scrambled data patterns
	// Check for absence of normal DVD structure markers
	header := binary.BigEndian.Uint32(sector[0:4])

	// Normal sectors start with known patterns:
	// 0x000001BA (MPEG pack)
	// 0x000001BB (MPEG system)
	// 0x000001BE (MPEG private stream)
	// 0x000001E0 (MPEG video)
	// 0x000001BD (MPEG audio/private)
	// Encrypted sectors won't have these

	switch header {
	case 0x000001BA, 0x000001BB, 0x000001BE, 0x000001BD:
		return false // Not encrypted
	}

	// Check for scrambled sector indicators
	// CSS affects specific bit patterns
	scrambleIndicator := sector[0x14:0x16] // Common scramble location
	if scrambleIndicator[0] != 0x00 || scrambleIndicator[1] != 0x00 {
		return true // Likely encrypted
	}

	return false
}

// DecryptVOB decrypts a CSS-encrypted VOB file.
// Output is written to outPath. Returns bytes decrypted.
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

	reader := NewDecryptReader(in, d)

	// Stream decrypt
	sector := make([]byte, SectorSize)
	var total int64

	for {
		n, err := io.ReadFull(reader, sector)
		if err != nil {
			if err == io.ErrUnexpectedEOF || err == io.EOF {
				break
			}
			return total, err
		}

		written, err := out.Write(sector[:n])
		if err != nil {
			return total, err
		}
		total += int64(written)
	}

	return total, nil
}

// DecryptVOBSet decrypts all VOBs in a title set.
// Returns the total bytes decrypted.
func DecryptVOBSet(vobPaths []string, outputDir string, d *Decryptor) (int64, error) {
	var total int64

	for _, vobPath := range vobPaths {
		outPath := outputDir + "/" + vobPath[len(vobPath)-len("VTS_XX_Y.VOB"):]
		n, err := DecryptVOB(vobPath, outPath, d)
		if err != nil {
			return total, err
		}
		total += n
	}

	return total, nil
}

// IsCSSEncrypted checks the VIDEO_TS.IFO for CSS encryption indicators.
func IsCSSEncrypted(ifoPath string) (bool, error) {
	f, err := os.Open(ifoPath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// Read header
	header := make([]byte, 128)
	_, err = io.ReadFull(f, header)
	if err != nil {
		return false, err
	}

	// Check for encryption marker
	// CSS encrypted discs have specific flags in the IFO header
	// Offset 0x40-0x43 contains protection info
	if len(header) >= 0x44 {
		protection := binary.BigEndian.Uint32(header[0x40:0x44])
		// Bit flags indicate encryption type
		return protection&0x01 != 0, nil
	}

	return false, nil
}
