package css

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// IsScrambledSector reports whether a DVD sector has the CSS scrambling flag set.
// A sector is CSS-scrambled when bits 4 or 5 of byte 0x14 are non-zero.
func IsScrambledSector(sec []byte) bool {
	return len(sec) > 0x14 && sec[0x14]&0x30 != 0
}

// DetectEncryption checks whether a VOB file contains CSS-scrambled sectors.
// It reads up to the first 20 sectors looking for a scrambling flag.
func DetectEncryption(vobPath string) (bool, error) {
	f, err := os.Open(vobPath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	sector := make([]byte, SectorSize)
	for i := 0; i < 20; i++ {
		if _, err := io.ReadFull(f, sector); err != nil {
			break
		}
		if IsScrambledSector(sector) {
			return true, nil
		}
	}
	return false, nil
}

// IsCSSEncrypted checks whether any VOB in the VIDEO_TS directory for the given
// IFO file contains CSS-scrambled sectors.
func IsCSSEncrypted(ifoPath string) (bool, error) {
	dir := filepath.Dir(ifoPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".vob") {
			continue
		}
		encrypted, err := DetectEncryption(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		if encrypted {
			return true, nil
		}
	}
	return false, nil
}
