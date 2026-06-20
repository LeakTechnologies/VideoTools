package vob

import (
	"fmt"
	"os"

	"github.com/LeakTechnologies/VideoTools/internal/logging"
)

// CreateMinimalMenuVOB writes a single-sector VIDEO_TS.VOB containing one
// NAV_PCK. This satisfies strict hardware players that require a menu VOB to
// be present even when no actual menu is authored. The VOB contains zeroed
// PCI/DSI data, which is sufficient for players that just check the file exists.
func CreateMinimalMenuVOB(path string) error {
	logging.Info(logging.CatDVD, "Creating minimal menu VOB: %s", path)

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create menu vob: %w", err)
	}
	defer f.Close()

	m := NewMuxer(f)
	if err := m.WriteNAV_PCK(&PCIPacket{}, &DSIPacket{}); err != nil {
		return fmt.Errorf("write menu nav_pck: %w", err)
	}
	return nil
}
