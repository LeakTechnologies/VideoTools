package ifo

import (
	"fmt"
	"strings"
)

// DVDCommand is a single 8-byte DVD-Video Virtual Machine instruction.
type DVDCommand [8]byte

// DVDCommandTable groups the three command sections of a PGC.
type DVDCommandTable struct {
	Pre  []DVDCommand // executed when PGC is entered
	Post []DVDCommand // executed when PGC exits
	Cell []DVDCommand // button action commands (indexed by CommandNR in PCI, 1-based)
}

// Empty reports whether the table has no commands at all.
func (t *DVDCommandTable) Empty() bool {
	return len(t.Pre) == 0 && len(t.Post) == 0 && len(t.Cell) == 0
}

// JumpTTCommand returns a VM instruction that jumps to title N (1-based) on the disc.
// Encoding observed in dvdauthor-generated IFO files: 0x30 00 00 00 00 00 TT 00.
func JumpTTCommand(titleN int) DVDCommand {
	return DVDCommand{0x30, 0x00, 0x00, 0x00, 0x00, 0x00, byte(titleN), 0x00}
}

// JumpVMGM_PGCNCommand returns a VM instruction that jumps to PGC N (1-based) in the
// VMGM (Video Manager Menu) domain. Used for inter-menu navigation between the main
// menu, chapter pages, and extras menu.
// Encoding: JumpSS VMGM menu PGC N — opcode 0x30, sub-opcode 0x06.
func JumpVMGM_PGCNCommand(pgcN int) DVDCommand {
	return DVDCommand{0x30, 0x06, 0x00, 0x00, 0x00, 0x00, byte(pgcN), 0x00}
}

// SetHL_BTNNCommand returns a VM instruction that selects button N (1-based) as
// the default highlighted button when the menu PGC is entered.
// Encoding observed in dvdauthor output: 0x36 NN 00 00 00 00 00 00.
func SetHL_BTNNCommand(btnN int) DVDCommand {
	return DVDCommand{0x36, byte(btnN), 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
}

// NOPCommand is a no-operation (placeholder) instruction.
func NOPCommand() DVDCommand {
	return DVDCommand{}
}

// ParseButtonCommand translates a dvdauthor-style command string to a DVDCommand.
//
// Supported forms:
//   - "jump title N;"           → JumpTT(N) — play title N on the disc
//   - "jump title N chapter M;" → JumpTT(N) (chapter ignored; chapter seek not yet implemented)
//   - "jump menu N;"            → JumpVMGM_PGCN(N) — jump to VMGM PGC N (inter-menu navigation)
//   - "jump menu pgc N;"        → JumpVMGM_PGCN(N) — alternate form
//
// Unrecognised commands fall back to NOP.
func ParseButtonCommand(cmd string) DVDCommand {
	cmd = strings.TrimSpace(strings.ToLower(cmd))
	cmd = strings.TrimSuffix(cmd, ";")
	cmd = strings.TrimSpace(cmd)

	var titleN, chapterN int
	if n, _ := fmt.Sscanf(cmd, "jump title %d chapter %d", &titleN, &chapterN); n >= 1 {
		return JumpTTCommand(titleN)
	}
	if n, _ := fmt.Sscanf(cmd, "jump title %d", &titleN); n == 1 {
		return JumpTTCommand(titleN)
	}

	var pgcN int
	if n, _ := fmt.Sscanf(cmd, "jump menu pgc %d", &pgcN); n == 1 {
		return JumpVMGM_PGCNCommand(pgcN)
	}
	if n, _ := fmt.Sscanf(cmd, "jump menu %d", &pgcN); n == 1 {
		return JumpVMGM_PGCNCommand(pgcN)
	}

	return NOPCommand()
}

// SerializeCommandTable serialises a DVDCommandTable into its on-disc binary
// representation and returns the bytes.
//
// Layout:
//
//	[0:2]  nr_of_PreCmd  uint16
//	[2:4]  nr_of_PostCmd uint16
//	[4:6]  nr_of_CellCmd uint16
//	[6:8]  last_byte     uint16  = total_size - 1
//	[8:]   Pre commands  (each 8 bytes)
//	       Post commands
//	       Cell commands
func SerializeCommandTable(t *DVDCommandTable) []byte {
	nPre := len(t.Pre)
	nPost := len(t.Post)
	nCell := len(t.Cell)
	total := 8 + (nPre+nPost+nCell)*8
	buf := make([]byte, total)

	buf[0] = byte(nPre >> 8)
	buf[1] = byte(nPre)
	buf[2] = byte(nPost >> 8)
	buf[3] = byte(nPost)
	buf[4] = byte(nCell >> 8)
	buf[5] = byte(nCell)
	lastByte := uint16(total - 1)
	buf[6] = byte(lastByte >> 8)
	buf[7] = byte(lastByte)

	off := 8
	for _, c := range t.Pre {
		copy(buf[off:], c[:])
		off += 8
	}
	for _, c := range t.Post {
		copy(buf[off:], c[:])
		off += 8
	}
	for _, c := range t.Cell {
		copy(buf[off:], c[:])
		off += 8
	}
	return buf
}
