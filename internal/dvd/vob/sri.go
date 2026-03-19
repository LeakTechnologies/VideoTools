package vob

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"git.leaktechnologies.dev/stu/VideoTools/internal/logging"
)

// NAVPCKInfo holds the sector address and presentation timestamp of one NAV_PCK.
type NAVPCKInfo struct {
	Sector uint32 // 0-indexed sector offset within the VOB file
	PTM    uint32 // VOBU start presentation timestamp in 90kHz ticks (from PCI)
}

// sriByteOffset is the byte offset of the VOBU_SRI table within a 2048-byte sector.
//
//	Pack Header (14) + System Header (24) + PCI PES header (6) + PCI payload offset 12
//	= 56 — but VOBU_SRI is in the DSI, not the PCI.
//
//	DSI PES starts at: 14 + 24 + (6 + 980) = 1024; DSI payload at 1024 + 6 = 1030.
//	dsiOffVOBUSRI = 196 (within the DSI payload).
//	→ VOBU_SRI in sector: 1030 + 196 = 1226.
const sriByteOffset = 1226

// pciPTMByteOffset is the byte offset of vobu_s_ptm in a 2048-byte sector.
//
//	Pack Header (14) + System Header (24) + PCI PES header (6) + pciOffVOBUSPTM (12) = 56.
const pciPTMByteOffset = 56

// sriTimeOffsets holds the 30 VOBU_SRI time offsets in 90kHz ticks.
// The table is split into 20 forward entries [0..19] and 10 backward entries [20..29].
// Time values follow the DVD-Video spec (ECMA-267 §7.6.2), using a coarse
// doubling schedule that covers from 0.5 s to ~512 s in each direction.
//
// Forward [0..19]: 0.5 s, 1, 2, 4, 8, 16, 32, 64, 128, 256, 512 s (then zeros for the rest)
// Backward [20..29]: same intervals negated
var sriTimeOffsets = func() [dsiVOBUSRICount]int64 {
	seconds := []float64{0.5, 1, 2, 4, 8, 16, 32, 64, 128, 256, 512}
	var t [dsiVOBUSRICount]int64
	for i, s := range seconds {
		if i < 20 {
			t[i] = int64(s * 90000)
		}
	}
	for i, s := range seconds {
		if 20+i < dsiVOBUSRICount {
			t[20+i] = -int64(s * 90000)
		}
	}
	return t
}()

// ScanVOBNAVPCKs reads a VOB file and returns timing information for every
// NAV_PCK: its sector number and the VOBU start PTM from the PCI payload.
func ScanVOBNAVPCKs(path string) ([]NAVPCKInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open vob: %w", err)
	}
	defer f.Close()

	buf := make([]byte, PackSize)
	var navs []NAVPCKInfo
	sector := uint32(0)

	for {
		_, err := io.ReadFull(f, buf)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read sector %d: %w", sector, err)
		}
		if binary.BigEndian.Uint32(buf[0:4]) == PackStartCode &&
			binary.BigEndian.Uint32(buf[14:18]) == SystemHeaderCode {
			ptm := binary.BigEndian.Uint32(buf[pciPTMByteOffset:])
			navs = append(navs, NAVPCKInfo{Sector: sector, PTM: ptm})
		}
		sector++
	}
	logging.Debug(logging.CatDVD, "ScanVOBNAVPCKs: %d NAV_PCKs in %d sectors (%s)",
		len(navs), sector, path)
	return navs, nil
}

// PatchVOBUSRI reads a VOB file, computes the VOBU_SRI relative-seek table for
// every NAV_PCK from the scanned timing data, and writes the patched entries back
// to the file in-place.
//
// VOBU_SRI entries are signed relative sector offsets (from the current NAV_PCK's
// sector to the target NAV_PCK's sector) stored as uint32 with the top bit clear
// for within-cell entries. SRIEndOfCell is written when no VOBU exists at the
// requested time offset.
//
// This enables hardware players to perform smooth trick-play (fast-forward, rewind)
// without relying solely on the VOBU_ADMAP for absolute seek.
func PatchVOBUSRI(path string, navs []NAVPCKInfo) error {
	if len(navs) < 2 {
		return nil // nothing to patch; SRIEndOfCell is already correct
	}

	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open vob for patching: %w", err)
	}
	defer f.Close()

	sriEntry := make([]byte, 4)

	for i, nav := range navs {
		var sri [dsiVOBUSRICount]uint32

		for k, dtTicks := range sriTimeOffsets {
			targetPTM := int64(nav.PTM) + dtTicks
			sri[k] = SRIEndOfCell

			if dtTicks >= 0 {
				// Forward search: first NAV_PCK with PTM >= targetPTM
				for j := i + 1; j < len(navs); j++ {
					if int64(navs[j].PTM) >= targetPTM {
						sri[k] = navs[j].Sector - nav.Sector
						break
					}
				}
			} else {
				// Backward search: last NAV_PCK with PTM <= targetPTM
				for j := i - 1; j >= 0; j-- {
					if int64(navs[j].PTM) <= targetPTM {
						// Backward entry: relative offset is negative but stored
						// as a positive value with bit 31 set per the DVD spec.
						diff := nav.Sector - navs[j].Sector
						sri[k] = 0x80000000 | diff
						break
					}
				}
			}
		}

		// Seek to the VOBU_SRI region in this sector and patch it.
		offset := int64(nav.Sector)*PackSize + sriByteOffset
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return fmt.Errorf("seek to sector %d sri: %w", nav.Sector, err)
		}
		for k := 0; k < dsiVOBUSRICount; k++ {
			binary.BigEndian.PutUint32(sriEntry, sri[k])
			if _, err := f.Write(sriEntry); err != nil {
				return fmt.Errorf("write sri entry %d in sector %d: %w", k, nav.Sector, err)
			}
		}
	}

	logging.Info(logging.CatDVD, "PatchVOBUSRI: patched %d NAV_PCKs in %s", len(navs), path)
	return nil
}
