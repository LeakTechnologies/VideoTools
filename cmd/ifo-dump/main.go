package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: ifo-dump <path-to-ifo-file>")
		fmt.Println("       ifo-dump <path-to-video_ts-folder>")
		os.Exit(1)
	}

	path := os.Args[1]
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if info.IsDir() {
		dumpVIDEO_TS(path)
	} else {
		dumpIFO(path)
	}
}

func dumpVIDEO_TS(dir string) {
	files, err := filepath.Glob(filepath.Join(dir, "*.IFO"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning directory: %v\n", err)
		os.Exit(1)
	}

	for _, f := range files {
		fmt.Printf("\n=== %s ===\n", filepath.Base(f))
		dumpIFO(f)
	}
}

func dumpIFO(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	if len(data) < 2048 {
		fmt.Fprintf(os.Stderr, "File too small: %d bytes\n", len(data))
		os.Exit(1)
	}

	filename := filepath.Base(path)

	if data[0] == 'D' && data[1] == 'V' && data[2] == 'D' {
		if filename == "VIDEO_TS.IFO" {
			dumpVMG_MAT(data)
		} else {
			dumpVTS_MAT(data)
		}
	} else {
		fmt.Println("Not a valid IFO file (missing DVD identifier)")
		os.Exit(1)
	}
}

func dumpVMG_MAT(data []byte) {
	fmt.Println("\n--- VMG_MAT (VIDEO_TS.IFO) ---")
	fmt.Printf("Identifier:        %s\n", string(data[0:12]))
	fmt.Printf("VMG_LastSector:    %d\n", binary.BigEndian.Uint32(data[12:16]))
	fmt.Printf("VMGI_LastByte:     %d (file size - 1)\n", binary.BigEndian.Uint32(data[128:132]))

	fmt.Printf("\nOffsets (all in sectors, multiply by 2048 for bytes):\n")
	fmt.Printf("  0xC4 TT_SRPT:          %d\n", binary.BigEndian.Uint32(data[196:200]))
	fmt.Printf("  0xC8 VMGM_PGCI_UT:     %d\n", binary.BigEndian.Uint32(data[200:204]))
	fmt.Printf("  0xCC VMG_PTL_MAIT:     %d\n", binary.BigEndian.Uint32(data[204:208]))
	fmt.Printf("  0xD0 VMG_VTS_ATRT:     %d\n", binary.BigEndian.Uint32(data[208:212]))
	fmt.Printf("  0xD4 VMG_TXTDT_MG:     %d\n", binary.BigEndian.Uint32(data[212:216]))
	fmt.Printf("  0xD8 VMG_M_C_ADT:      %d\n", binary.BigEndian.Uint32(data[216:220]))
	fmt.Printf("  0xDC VMG_M_VOBU_ADMAP: %d\n", binary.BigEndian.Uint32(data[220:224]))

	fmt.Printf("\nFirst Play PGC:    byte %d (sector %d + 0x%03X)\n",
		binary.BigEndian.Uint32(data[132:136]),
		binary.BigEndian.Uint32(data[132:136])/2048,
		binary.BigEndian.Uint32(data[132:136])%2048)

	dumpHex(data, 0, 256, "header")
}

func dumpVTS_MAT(data []byte) {
	fmt.Println("\n--- VTS_MAT ---")
	fmt.Printf("Identifier:        %s\n", string(data[0:12]))
	fmt.Printf("VTS_LastSector:    %d\n", binary.BigEndian.Uint32(data[12:16]))
	fmt.Printf("VTSI_LastByte:      %d (file size - 1)\n", binary.BigEndian.Uint32(data[128:132]))

	fmt.Printf("\nOffsets (all in sectors, multiply by 2048 for bytes):\n")
	fmt.Printf("  0xC0 VTSM_VOBS:        %d\n", binary.BigEndian.Uint32(data[192:196]))
	fmt.Printf("  0xC4 VTSTT_VOBS:       %d\n", binary.BigEndian.Uint32(data[196:200]))
	fmt.Printf("  0xC8 VTS_PTT_SRPT:     %d\n", binary.BigEndian.Uint32(data[200:204]))
	fmt.Printf("  0xCC VTS_PGCITI:       %d\n", binary.BigEndian.Uint32(data[204:208]))
	fmt.Printf("  0xD0 VTSM_PGCI_UT:     %d\n", binary.BigEndian.Uint32(data[208:212]))
	fmt.Printf("  0xD4 VTS_TMAPTI:       %d\n", binary.BigEndian.Uint32(data[212:216]))
	fmt.Printf("  0xD8 VTSM_C_ADT:       %d\n", binary.BigEndian.Uint32(data[216:220]))
	fmt.Printf("  0xDC VTSM_VOBU_ADMAP:  %d\n", binary.BigEndian.Uint32(data[220:224]))
	fmt.Printf("  0xE0 VTS_C_ADT:        %d\n", binary.BigEndian.Uint32(data[224:228]))
	fmt.Printf("  0xE4 VTS_VOBU_ADMAP:   %d\n", binary.BigEndian.Uint32(data[228:232]))

	// Dump TMAPT if present
	tmaptOff := binary.BigEndian.Uint32(data[212:216])
	if tmaptOff > 0 {
		tmaptByte := tmaptOff * 2048
		if int(tmaptByte) < len(data) {
			dumpTMAPT(data[tmaptByte:])
		}
	}

	dumpHex(data, 192, 256, "offset table")
}

func dumpTMAPT(data []byte) {
	fmt.Println("\n--- VTS_TMAPT ---")
	if len(data) < 16 {
		fmt.Printf("TMAPT data too short: %d bytes\n", len(data))
		return
	}

	nrOfTMAPTs := binary.BigEndian.Uint16(data[0:2])
	endByte := binary.BigEndian.Uint32(data[4:8])
	tmapOff := binary.BigEndian.Uint32(data[8:12])

	fmt.Printf("NrOf_VTS_TMAPTs:  %d\n", nrOfTMAPTs)
	fmt.Printf("EndByte:           %d\n", endByte)
	fmt.Printf("TMAP[0] offset:     %d (byte %d in TMAPT)\n", tmapOff, tmapOff)

	if nrOfTMAPTs > 0 && int(tmapOff)+4 <= len(data) {
		zero1 := data[tmapOff]
		timeUnit := data[tmapOff+1]
		nrOfEntries := binary.BigEndian.Uint16(data[tmapOff+2 : tmapOff+4])

		fmt.Printf("\nTMAP[0] header:\n")
		fmt.Printf("  zero_1:          0x%02X%s\n", zero1, checkZero(zero1))
		fmt.Printf("  Time_Unit:        %d seconds\n", timeUnit)
		fmt.Printf("  NrOf_Entries:     %d\n", nrOfEntries)

		// Dump first few entries
		entryOff := int(tmapOff) + 4
		maxEntries := int(nrOfEntries)
		if maxEntries > 5 {
			maxEntries = 5
		}
		fmt.Printf("\n  First %d sectors:\n", maxEntries)
		for i := 0; i < maxEntries && entryOff+4 <= len(data); i++ {
			sector := binary.BigEndian.Uint32(data[entryOff : entryOff+4])
			ecce := (sector & 0x80000000) != 0
			addr := sector & 0x7FFFFFFF
			fmt.Printf("    Entry %d: sector %d (ECCE=%v)\n", i, addr, ecce)
			entryOff += 4
		}

		// Check for issues
		if nrOfTMAPTs > 1 {
			fmt.Printf("\n  WARNING: NrOf_VTS_TMAPTs = %d, but our code only generates1 TMAP\n", nrOfTMAPTs)
			// Dump subsequent TMAP headers
			for i := uint16(1); i < nrOfTMAPTs; i++ {
				// Each TMAP offset is at position 8 + i*4
				offPos := 8 + i*4
				if int(offPos)+4 > len(data) {
					break
				}
				off := binary.BigEndian.Uint32(data[offPos : offPos+4])
				fmt.Printf("TMAP[%d] offset: %d\n", i, off)
			}
		}
	}

	dumpHex(data, 0, 64, "TMAPT header")
}

func checkZero(val byte) string {
	if val == 0 {
		return " ✓"
	}
	return " ✗ SHOULD BE 0x00!"
}

func dumpHex(data []byte, start, end int, label string) {
	if end > len(data) {
		end = len(data)
	}
	fmt.Printf("\n--- Hex dump: %s (bytes %d-%d) ---\n", label, start, end)
	for i := start; i < end; i += 16 {
		endOff := i + 16
		if endOff > end {
			endOff = end
		}
		line := data[i:endOff]
		fmt.Printf("  %04X: %s ", i, hex.EncodeToString(line))
		for _, b := range line {
			if b >= 32 && b < 127 {
				fmt.Printf("%c", b)
			} else {
				fmt.Printf(".")
			}
		}
		fmt.Println()
	}
}
