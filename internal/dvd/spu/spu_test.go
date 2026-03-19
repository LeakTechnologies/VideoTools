package spu

import (
	"encoding/binary"
	"image"
	"image/color"
	"testing"
)

// makePaletted returns a small solid-color indexed image.
func makePaletted(w, h int, idx uint8) *image.Paletted {
	pal := color.Palette{
		color.RGBA{0, 0, 0, 0},
		color.RGBA{255, 255, 255, 255},
		color.RGBA{200, 0, 0, 255},
		color.RGBA{0, 0, 200, 255},
	}
	img := image.NewPaletted(image.Rect(0, 0, w, h), pal)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetColorIndex(x, y, idx)
		}
	}
	return img
}

func TestEncode_PacketHeader(t *testing.T) {
	enc := NewEncoder(4, 4)
	img := makePaletted(4, 4, 1)
	data, err := enc.Encode(img, DefaultSPUOptions())
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if len(data) < 4 {
		t.Fatalf("packet too short: %d bytes", len(data))
	}

	// SP_DCSQ_SZ must equal total packet length.
	spSize := int(binary.BigEndian.Uint16(data[0:2]))
	if spSize != len(data) {
		t.Errorf("SP_DCSQ_SZ %d != packet len %d", spSize, len(data))
	}

	// SP_DCSQTA must point past the pixel data (>= 4).
	dcsqTA := int(binary.BigEndian.Uint16(data[2:4]))
	if dcsqTA < 4 {
		t.Errorf("SP_DCSQTA %d too small (< 4)", dcsqTA)
	}
	if dcsqTA >= len(data) {
		t.Errorf("SP_DCSQTA %d >= packet len %d", dcsqTA, len(data))
	}
}

func TestEncode_DCSQ0Structure(t *testing.T) {
	enc := NewEncoder(8, 8)
	img := makePaletted(8, 8, 0)
	opts := DefaultSPUOptions()
	data, err := enc.Encode(img, opts)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	dcsqTA := int(binary.BigEndian.Uint16(data[2:4]))
	if dcsqTA+4 > len(data) {
		t.Fatalf("not enough bytes for DCSQ[0] header")
	}

	// DCSQ[0] delay must be 0.
	delay := binary.BigEndian.Uint16(data[dcsqTA : dcsqTA+2])
	if delay != 0 {
		t.Errorf("DCSQ[0] delay = %d, want 0", delay)
	}

	// DCSQ[0] next_dcsq must be > dcsqTA (points forward to DCSQ[1]).
	nextDCSQ := int(binary.BigEndian.Uint16(data[dcsqTA+2 : dcsqTA+4]))
	if nextDCSQ <= dcsqTA {
		t.Errorf("DCSQ[0] next_dcsq %d <= dcsqTA %d (must point forward)", nextDCSQ, dcsqTA)
	}
	if nextDCSQ >= len(data) {
		t.Errorf("DCSQ[0] next_dcsq %d out of bounds (packet len %d)", nextDCSQ, len(data))
	}

	// First command byte must be FORCED_START (0x00).
	if data[dcsqTA+4] != DCSQ_FORCED_START {
		t.Errorf("DCSQ[0] first cmd = 0x%02X, want FORCED_START 0x00", data[dcsqTA+4])
	}
}

func TestEncode_DCSQ0Commands(t *testing.T) {
	enc := NewEncoder(8, 8)
	img := makePaletted(8, 8, 0)
	opts := SPUOptions{
		PaletteIndices: [4]uint8{0, 5, 10, 15},
		AlphaValues:    [4]uint8{0, 15, 12, 4},
	}
	data, err := enc.Encode(img, opts)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	dcsqTA := int(binary.BigEndian.Uint16(data[2:4]))
	cmds := data[dcsqTA+4:] // skip DCSQ[0] header

	// Expected command sequence:
	// [0] FORCED_START (0x00)
	// [1] SET_COLOR    (0x03)
	// [2] color high byte
	// [3] color low byte
	// [4] SET_CONTR    (0x04)
	// [5] contr high byte
	// [6] contr low byte
	// [7] SET_AREA     (0x05)
	// [8..13] 6 bytes area
	// [14] SET_ADDRESS  (0x06)
	// [15..18] 4 bytes
	// [19] END          (0xFF)
	if len(cmds) < 20 {
		t.Fatalf("DCSQ[0] commands too short (%d bytes)", len(cmds))
	}

	if cmds[0] != DCSQ_FORCED_START {
		t.Errorf("cmd[0] = 0x%02X, want FORCED_START", cmds[0])
	}
	if cmds[1] != DCSQ_SET_COLOR {
		t.Errorf("cmd[1] = 0x%02X, want SET_COLOR 0x03", cmds[1])
	}
	// SET_COLOR word: [15-12]=15, [11-8]=10, [7-4]=5, [3-0]=0 => 0xFA50
	colorWord := uint16(cmds[2])<<8 | uint16(cmds[3])
	wantColor := uint16(opts.PaletteIndices[3])<<12 |
		uint16(opts.PaletteIndices[2])<<8 |
		uint16(opts.PaletteIndices[1])<<4 |
		uint16(opts.PaletteIndices[0])
	if colorWord != wantColor {
		t.Errorf("SET_COLOR word = 0x%04X, want 0x%04X", colorWord, wantColor)
	}

	if cmds[4] != DCSQ_SET_CONTR {
		t.Errorf("cmd[4] = 0x%02X, want SET_CONTR 0x04", cmds[4])
	}
	contWord := uint16(cmds[5])<<8 | uint16(cmds[6])
	wantContr := uint16(opts.AlphaValues[3])<<12 |
		uint16(opts.AlphaValues[2])<<8 |
		uint16(opts.AlphaValues[1])<<4 |
		uint16(opts.AlphaValues[0])
	if contWord != wantContr {
		t.Errorf("SET_CONTR word = 0x%04X, want 0x%04X", contWord, wantContr)
	}

	if cmds[7] != DCSQ_SET_AREA {
		t.Errorf("cmd[7] = 0x%02X, want SET_AREA 0x05", cmds[7])
	}
	if cmds[14] != DCSQ_SET_ADDRESS {
		t.Errorf("cmd[14] = 0x%02X, want SET_ADDRESS 0x06", cmds[14])
	}
}

func TestEncode_DCSQ1Terminator(t *testing.T) {
	enc := NewEncoder(4, 4)
	img := makePaletted(4, 4, 0)
	data, err := enc.Encode(img, DefaultSPUOptions())
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	dcsqTA := int(binary.BigEndian.Uint16(data[2:4]))
	dcsq1Offset := int(binary.BigEndian.Uint16(data[dcsqTA+2 : dcsqTA+4]))

	if dcsq1Offset+6 > len(data) {
		t.Fatalf("DCSQ[1] offset %d out of bounds (packet len %d)", dcsq1Offset, len(data))
	}

	// DCSQ[1] next_dcsq must point to itself (self-reference = terminator).
	dcsq1Next := int(binary.BigEndian.Uint16(data[dcsq1Offset+2 : dcsq1Offset+4]))
	if dcsq1Next != dcsq1Offset {
		t.Errorf("DCSQ[1] next_dcsq %d != self %d (not self-referencing)", dcsq1Next, dcsq1Offset)
	}

	// DCSQ[1] must issue STOP then END.
	if data[dcsq1Offset+4] != DCSQ_STOP {
		t.Errorf("DCSQ[1] cmd = 0x%02X, want STOP 0x02", data[dcsq1Offset+4])
	}
	if data[dcsq1Offset+5] != DCSQ_END {
		t.Errorf("DCSQ[1] terminator = 0x%02X, want END 0xFF", data[dcsq1Offset+5])
	}
}

func TestEncode_SetAreaCovers_FullImage(t *testing.T) {
	w, h := 720, 480
	enc := NewEncoder(w, h)
	img := makePaletted(w, h, 0)
	data, err := enc.Encode(img, DefaultSPUOptions())
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	dcsqTA := int(binary.BigEndian.Uint16(data[2:4]))
	cmds := data[dcsqTA+4:]

	// SET_AREA is cmd[7], 6 bytes at cmds[8..13].
	// Bytes 0-2: [x1:12][x2:12] = [0:12][719:12]
	// Bytes 3-5: [y1:12][y2:12] = [0:12][479:12]
	areaBytes := cmds[8:14]

	x1 := uint16(areaBytes[0])<<4 | uint16(areaBytes[1])>>4
	x2 := uint16(areaBytes[1]&0x0F)<<8 | uint16(areaBytes[2])
	y1 := uint16(areaBytes[3])<<4 | uint16(areaBytes[4])>>4
	y2 := uint16(areaBytes[4]&0x0F)<<8 | uint16(areaBytes[5])

	if x1 != 0 || x2 != uint16(w-1) {
		t.Errorf("SET_AREA x = [%d, %d], want [0, %d]", x1, x2, w-1)
	}
	if y1 != 0 || y2 != uint16(h-1) {
		t.Errorf("SET_AREA y = [%d, %d], want [0, %d]", y1, y2, h-1)
	}
}

func TestEncode_SetAddress_ValidOffsets(t *testing.T) {
	enc := NewEncoder(8, 8)
	img := makePaletted(8, 8, 0)
	data, err := enc.Encode(img, DefaultSPUOptions())
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	dcsqTA := int(binary.BigEndian.Uint16(data[2:4]))
	cmds := data[dcsqTA+4:]

	// SET_ADDRESS is cmds[14], top offset at cmds[15:17], bot at cmds[17:19].
	topOff := int(binary.BigEndian.Uint16(cmds[15:17]))
	botOff := int(binary.BigEndian.Uint16(cmds[17:19]))

	// Top field offset must be 4 (immediately after the 4-byte header).
	if topOff != 4 {
		t.Errorf("top field offset = %d, want 4", topOff)
	}
	// Bot field offset must be >= top field offset.
	if botOff <= topOff {
		t.Errorf("bot field offset %d <= top field offset %d", botOff, topOff)
	}
	// Both offsets must be inside the pixel data region (before dcsqTA).
	if topOff >= dcsqTA {
		t.Errorf("top field offset %d >= dcsqTA %d", topOff, dcsqTA)
	}
	if botOff >= dcsqTA {
		t.Errorf("bot field offset %d >= dcsqTA %d", botOff, dcsqTA)
	}
}

func TestPack12Pair(t *testing.T) {
	cases := []struct{ v1, v2 uint16 }{
		{0, 0},
		{0, 719},
		{0, 479},
		{100, 620},
		{0x0FFF, 0x0FFF},
	}
	for _, c := range cases {
		b := pack12pair(c.v1, c.v2)
		got1 := uint16(b[0])<<4 | uint16(b[1])>>4
		got2 := uint16(b[1]&0x0F)<<8 | uint16(b[2])
		if got1 != c.v1 || got2 != c.v2 {
			t.Errorf("pack12pair(%d,%d) roundtrip = (%d,%d)", c.v1, c.v2, got1, got2)
		}
	}
}
