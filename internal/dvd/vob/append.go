package vob

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/LeakTechnologies/VideoTools/internal/logging"
)

// Byte offsets of the PCI and DSI table data within a 2048-byte NAV_PCK
// sector: pack header (14) + system header (24) + PES header (6) + substream
// ID (1) = 45 for the PCI; + PCI payload (980) + PES header (6) + substream
// ID (1) = 1031 for the DSI.
const (
	navSectorPCIOff = 45
	navSectorDSIOff = 1031
)

// AppendMenuVOB appends a native-muxed menu MPG to w, rebasing every
// NAV_PCK so the result is a valid multi-menu VIDEO_TS.VOB (audit findings
// A9/A10):
//
//   - PCI nv_pck_lbn and DSI nv_pck_lbn are rebased by sectorBase so LBNs are
//     correct within the concatenated VOB (raw concatenation left every menu
//     after the first with LBNs restarting at 0).
//   - DSI vobu_vob_idn / vobu_c_idn are set to vobID / cellID (1-based).
//     Each menu page is its own VOB within VMGM_VOBS; the IDs must match the
//     PGC CellPosition entries and the menu C_ADT.
//
// Non-NAV sectors are copied through unchanged. A short final sector is
// zero-padded to the 2048-byte boundary. Returns the number of sectors
// written.
func AppendMenuVOB(w io.Writer, mpgPath string, sectorBase uint32, vobID uint16, cellID uint8) (uint32, error) {
	f, err := os.Open(mpgPath)
	if err != nil {
		return 0, fmt.Errorf("AppendMenuVOB open: %w", err)
	}
	defer f.Close()

	buf := make([]byte, PackSize)
	var written uint32
	navPatched := 0

	for {
		n, err := io.ReadFull(f, buf)
		if err == io.EOF {
			break
		}
		if err == io.ErrUnexpectedEOF {
			// Zero-pad a short tail to keep the output sector-aligned.
			for i := n; i < PackSize; i++ {
				buf[i] = 0
			}
		} else if err != nil {
			return written, fmt.Errorf("AppendMenuVOB read: %w", err)
		}

		// NAV_PCK signature: pack start code + system header (see ScanVOBForNAVPCKs).
		if binary.BigEndian.Uint32(buf[0:4]) == PackStartCode &&
			binary.BigEndian.Uint32(buf[14:18]) == SystemHeaderCode {
			pci := buf[navSectorPCIOff:]
			dsi := buf[navSectorDSIOff:]

			lbn := binary.BigEndian.Uint32(pci[pciOffNVPCKLBN:]) + sectorBase
			binary.BigEndian.PutUint32(pci[pciOffNVPCKLBN:], lbn)
			binary.BigEndian.PutUint32(dsi[dsiOffNVPCKLBN:], lbn)
			binary.BigEndian.PutUint16(dsi[dsiOffVOBUVOBIDN:], vobID)
			dsi[dsiOffVOBUCIDN] = cellID
			navPatched++
		}

		if _, err := w.Write(buf); err != nil {
			return written, fmt.Errorf("AppendMenuVOB write: %w", err)
		}
		written++

		if err == io.ErrUnexpectedEOF {
			break
		}
	}

	logging.Info(logging.CatDVD, "AppendMenuVOB: %s → %d sectors at base %d (VOB %d, %d NAV_PCKs rebased)",
		mpgPath, written, sectorBase, vobID, navPatched)
	return written, nil
}
