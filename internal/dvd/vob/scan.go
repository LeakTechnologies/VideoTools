package vob

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"git.leaktechnologies.dev/leak_technologies/VideoTools/internal/logging"
)

// ScanVOBForNAVPCKs reads a DVD VOB file and returns the sector offset
// (0-indexed from the start of the file) of every NAV_PCK it contains.
//
// A NAV_PCK is identified by the two-code sequence at the start of a sector:
//
//	Offset  0–3:  Pack Start Code  0x000001BA
//	Offset 14–17: System Header Code 0x000001BB
//
// This signature is unique to Navigation Packs in the DVD-Video specification.
// Regular video/audio/padding packs never carry a system header.
func ScanVOBForNAVPCKs(path string) ([]uint32, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open vob: %w", err)
	}
	defer f.Close()

	buf := make([]byte, PackSize)
	var sectors []uint32
	sector := uint32(0)

	for {
		_, err := io.ReadFull(f, buf)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read vob sector %d: %w", sector, err)
		}

		if binary.BigEndian.Uint32(buf[0:4]) == PackStartCode &&
			binary.BigEndian.Uint32(buf[14:18]) == SystemHeaderCode {
			sectors = append(sectors, sector)
		}
		sector++
	}

	logging.Debug(logging.CatDVD, "ScanVOBForNAVPCKs: found %d NAV_PCKs in %d sectors (%s)",
		len(sectors), sector, path)
	return sectors, nil
}
