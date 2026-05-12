package css

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
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
		logging.Warning(logging.CatDVD, "CSS detect: cannot open %s: %v", vobPath, err)
		return false, err
	}
	defer f.Close()

	scrambledCount := 0
	sector := make([]byte, SectorSize)
	for i := 0; i < 20; i++ {
		if _, err := io.ReadFull(f, sector); err != nil {
			logging.Debug(logging.CatDVD, "CSS detect: %s sector %d read error: %v", vobPath, i, err)
			break
		}
		if IsScrambledSector(sector) {
			scrambledCount++
			if scrambledCount >= 2 {
				logging.Info(logging.CatDVD, "CSS encryption confirmed in %s (%d scrambled sectors detected)", vobPath, scrambledCount)
				return true, nil
			}
		}
	}
	if scrambledCount > 0 {
		logging.Debug(logging.CatDVD, "CSS detection inconclusive in %s: only %d/20 scrambled sectors", vobPath, scrambledCount)
	} else {
		logging.Debug(logging.CatDVD, "CSS not detected in %s: all sectors clear", vobPath)
	}
	return false, nil
}

// IsCSSEncrypted checks whether any VOB in the VIDEO_TS directory for the given
// IFO file contains CSS-scrambled sectors.
func IsCSSEncrypted(ifoPath string) (bool, error) {
	dir := filepath.Dir(ifoPath)
	logging.Debug(logging.CatDVD, "CSS encryption check in directory: %s", dir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		logging.Warning(logging.CatDVD, "CSS check: cannot read directory %s: %v", dir, err)
		return false, fmt.Errorf("read dir %s: %w", dir, err)
	}

	vobCount := 0
	vobsScanned := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".vob") {
			continue
		}
		vobCount++
		encrypted, err := DetectEncryption(filepath.Join(dir, entry.Name()))
		if err != nil {
			logging.Debug(logging.CatDVD, "CSS check: skipping %s due to error: %v", entry.Name(), err)
			continue
		}
		vobsScanned++
		if encrypted {
			logging.Info(logging.CatDVD, "CSS encryption detected in %s (scanned %d/%d VOBs)", dir, vobsScanned, vobCount)
			return true, nil
		}
	}
	logging.Debug(logging.CatDVD, "CSS check complete: %d VOBs scanned out of %d found, no encryption detected", vobsScanned, vobCount)
	return false, nil
}
